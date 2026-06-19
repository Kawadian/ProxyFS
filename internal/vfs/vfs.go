package vfs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotFound     = errors.New("file not found")
	ErrPermission   = errors.New("permission denied")
	ErrIsDirectory  = errors.New("is a directory")
	ErrNotDirectory = errors.New("not a directory")
)

// FileInfo describes a virtual file or directory entry.
type FileInfo struct {
	Name    string
	Path    string
	Size    int64
	Mode    os.FileMode
	ModTime time.Time
	IsDir   bool
}

// NodeBackend abstracts remote or local storage for a mounted node.
type NodeBackend interface {
	Stat(ctx context.Context, path string) (FileInfo, error)
	ReadDir(ctx context.Context, path string) ([]FileInfo, error)
	Open(ctx context.Context, path string) (io.ReadCloser, error)
	Write(ctx context.Context, path string, offset int64, r io.Reader) (int64, error)
	Remove(ctx context.Context, path string) error
	Mkdir(ctx context.Context, path string) error
	Rename(ctx context.Context, oldPath, newPath string) error
}

// Mount maps a virtual path prefix to a node backend.
type Mount struct {
	Name     string
	Prefix   string
	Backend  NodeBackend
	ReadOnly bool
}

// VirtualFS aggregates multiple node backends under a unified namespace.
type VirtualFS struct {
	mu        sync.RWMutex
	root      string
	mounts    []Mount
	dirTTL    time.Duration
	statTTL   time.Duration
	dirCache  map[string]cachedDir
	statCache map[string]cachedStat
}

type cachedDir struct {
	entries []FileInfo
	expires time.Time
}

type cachedStat struct {
	info    FileInfo
	expires time.Time
}

// New creates a VirtualFS rooted at root with the given cache TTLs.
func New(root string, dirTTL, statTTL time.Duration) *VirtualFS {
	return &VirtualFS{
		root:      root,
		dirTTL:    dirTTL,
		statTTL:   statTTL,
		dirCache:  make(map[string]cachedDir),
		statCache: make(map[string]cachedStat),
	}
}

// AddMount registers a backend at a virtual prefix like "/node-name".
func (v *VirtualFS) AddMount(m Mount) error {
	m.Prefix = CleanVirtualPath(m.Prefix)
	if m.Prefix == "/" || m.Prefix == "" {
		return errors.New("mount prefix must not be root")
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	for _, existing := range v.mounts {
		if existing.Prefix == m.Prefix {
			return errors.New("mount already exists")
		}
	}
	v.mounts = append(v.mounts, m)
	delete(v.dirCache, "/")
	delete(v.statCache, "/")
	return nil
}

// Mounts returns a copy of registered mounts.
func (v *VirtualFS) Mounts() []Mount {
	v.mu.RLock()
	defer v.mu.RUnlock()
	out := make([]Mount, len(v.mounts))
	copy(out, v.mounts)
	return out
}

// MountPrefixes returns all registered mount prefix paths.
func (v *VirtualFS) MountPrefixes() map[string]struct{} {
	v.mu.RLock()
	defer v.mu.RUnlock()
	out := make(map[string]struct{}, len(v.mounts))
	for _, m := range v.mounts {
		out[m.Prefix] = struct{}{}
	}
	return out
}

// RemoveMount removes a mount by prefix.
func (v *VirtualFS) RemoveMount(prefix string) error {
	prefix = CleanVirtualPath(prefix)
	v.mu.Lock()
	defer v.mu.Unlock()
	for i, m := range v.mounts {
		if m.Prefix == prefix {
			v.mounts = append(v.mounts[:i], v.mounts[i+1:]...)
			delete(v.dirCache, prefix)
			delete(v.statCache, prefix)
			delete(v.dirCache, "/")
			delete(v.statCache, "/")
			return nil
		}
	}
	return nil
}

func (v *VirtualFS) resolve(virtualPath string) (Mount, string, error) {
	clean, err := ResolveVirtualPath(virtualPath)
	if err != nil {
		return Mount{}, "", err
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	var best Mount
	bestLen := -1
	for _, m := range v.mounts {
		if clean == m.Prefix || strings.HasPrefix(clean+"/", m.Prefix+"/") {
			if len(m.Prefix) > bestLen {
				best = m
				bestLen = len(m.Prefix)
			}
		}
	}
	if bestLen < 0 {
		return Mount{}, "", fs.ErrNotExist
	}
	rel := strings.TrimPrefix(clean, best.Prefix)
	if rel == "" {
		rel = "/"
	}
	nodePath, err := ResolveNodePath(best.Backend, rel)
	if err != nil {
		return Mount{}, "", err
	}
	return best, nodePath, nil
}

// Stat returns metadata for a virtual path.
func (v *VirtualFS) Stat(ctx context.Context, virtualPath string) (FileInfo, error) {
	clean, err := ResolveVirtualPath(virtualPath)
	if err != nil {
		return FileInfo{}, err
	}
	if clean == "/" {
		return FileInfo{
			Name:    "/",
			Path:    "/",
			Mode:    os.ModeDir | 0o755,
			ModTime: time.Now(),
			IsDir:   true,
		}, nil
	}
	v.mu.RLock()
	if cached, ok := v.statCache[clean]; ok && time.Now().Before(cached.expires) {
		v.mu.RUnlock()
		return cached.info, nil
	}
	v.mu.RUnlock()

	mount, nodePath, err := v.resolve(clean)
	if err != nil {
		return FileInfo{}, err
	}
	info, err := mount.Backend.Stat(ctx, nodePath)
	if err != nil {
		return FileInfo{}, err
	}
	info.Path = clean
	v.mu.Lock()
	v.statCache[clean] = cachedStat{info: info, expires: time.Now().Add(v.statTTL)}
	v.mu.Unlock()
	return info, nil
}

// ReadDir lists a virtual directory.
func (v *VirtualFS) ReadDir(ctx context.Context, virtualPath string) ([]FileInfo, error) {
	clean, err := ResolveVirtualPath(virtualPath)
	if err != nil {
		return nil, err
	}
	v.mu.RLock()
	if cached, ok := v.dirCache[clean]; ok && time.Now().Before(cached.expires) {
		v.mu.RUnlock()
		return cached.entries, nil
	}
	v.mu.RUnlock()

	mount, nodePath, err := v.resolve(clean)
	if err != nil {
		// Root listing: expose mount points
		if errors.Is(err, fs.ErrNotExist) && clean == "/" {
			return v.listMounts(), nil
		}
		return nil, err
	}
	entries, err := mount.Backend.ReadDir(ctx, nodePath)
	if err != nil {
		return nil, err
	}
	for i := range entries {
		entries[i].Path = filepath.Join(clean, entries[i].Name)
	}
	v.mu.Lock()
	v.dirCache[clean] = cachedDir{entries: entries, expires: time.Now().Add(v.dirTTL)}
	v.mu.Unlock()
	return entries, nil
}

func (v *VirtualFS) listMounts() []FileInfo {
	v.mu.RLock()
	defer v.mu.RUnlock()
	out := make([]FileInfo, 0, len(v.mounts))
	for _, m := range v.mounts {
		out = append(out, FileInfo{
			Name:    filepath.Base(m.Prefix),
			Path:    m.Prefix,
			Mode:    os.ModeDir | 0o755,
			ModTime: time.Now(),
			IsDir:   true,
		})
	}
	return out
}

// Open opens a file for reading.
func (v *VirtualFS) Open(ctx context.Context, virtualPath string) (io.ReadCloser, error) {
	mount, nodePath, err := v.resolve(virtualPath)
	if err != nil {
		return nil, err
	}
	return mount.Backend.Open(ctx, nodePath)
}

// Write writes data at an offset; write operations fail on read-only mounts.
func (v *VirtualFS) Write(ctx context.Context, virtualPath string, offset int64, r io.Reader) (int64, error) {
	mount, nodePath, err := v.resolve(virtualPath)
	if err != nil {
		return 0, err
	}
	if mount.ReadOnly {
		return 0, ErrPermission
	}
	v.invalidate(virtualPath)
	return mount.Backend.Write(ctx, nodePath, offset, r)
}

// Remove deletes a file or empty directory.
func (v *VirtualFS) Remove(ctx context.Context, virtualPath string) error {
	mount, nodePath, err := v.resolve(virtualPath)
	if err != nil {
		return err
	}
	if mount.ReadOnly {
		return ErrPermission
	}
	v.invalidate(virtualPath)
	return mount.Backend.Remove(ctx, nodePath)
}

// Mkdir creates a directory.
func (v *VirtualFS) Mkdir(ctx context.Context, virtualPath string) error {
	mount, nodePath, err := v.resolve(virtualPath)
	if err != nil {
		return err
	}
	if mount.ReadOnly {
		return ErrPermission
	}
	v.invalidate(virtualPath)
	return mount.Backend.Mkdir(ctx, nodePath)
}

// Rename moves a file or directory within the same mount.
func (v *VirtualFS) Rename(ctx context.Context, oldPath, newPath string) error {
	oldMount, oldNodePath, err := v.resolve(oldPath)
	if err != nil {
		return err
	}
	newMount, newNodePath, err := v.resolve(newPath)
	if err != nil {
		return err
	}
	if oldMount.Prefix != newMount.Prefix {
		return fmt.Errorf("cross-mount rename not supported")
	}
	if oldMount.ReadOnly {
		return ErrPermission
	}
	v.invalidate(oldPath)
	v.invalidate(newPath)
	return oldMount.Backend.Rename(ctx, oldNodePath, newNodePath)
}

func (v *VirtualFS) invalidate(virtualPath string) {
	clean, err := ResolveVirtualPath(virtualPath)
	if err != nil {
		return
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.statCache, clean)
	delete(v.dirCache, clean)
	parent := filepath.Dir(clean)
	delete(v.dirCache, parent)
}

// LocalBackend implements NodeBackend on the local filesystem.
type LocalBackend struct {
	Root string
}

func (l *LocalBackend) Stat(ctx context.Context, path string) (FileInfo, error) {
	full, err := secureJoin(l.Root, path)
	if err != nil {
		return FileInfo{}, err
	}
	fi, err := os.Stat(full)
	if err != nil {
		if os.IsNotExist(err) {
			return FileInfo{}, ErrNotFound
		}
		return FileInfo{}, err
	}
	return fileInfoFromOS(path, fi), nil
}

func (l *LocalBackend) ReadDir(ctx context.Context, path string) ([]FileInfo, error) {
	full, err := secureJoin(l.Root, path)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(full)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	out := make([]FileInfo, 0, len(entries))
	for _, e := range entries {
		fi, err := e.Info()
		if err != nil {
			return nil, err
		}
		out = append(out, fileInfoFromOS(filepath.Join(path, e.Name()), fi))
	}
	return out, nil
}

func (l *LocalBackend) Open(ctx context.Context, path string) (io.ReadCloser, error) {
	full, err := secureJoin(l.Root, path)
	if err != nil {
		return nil, err
	}
	fi, err := os.Stat(full)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if fi.IsDir() {
		return nil, ErrIsDirectory
	}
	return os.Open(full)
}

func (l *LocalBackend) Write(ctx context.Context, path string, offset int64, r io.Reader) (int64, error) {
	full, err := secureJoin(l.Root, path)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return 0, err
	}
	f, err := os.OpenFile(full, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return 0, err
		}
	}
	return io.Copy(f, r)
}

func (l *LocalBackend) Remove(ctx context.Context, path string) error {
	full, err := secureJoin(l.Root, path)
	if err != nil {
		return err
	}
	return os.Remove(full)
}

func (l *LocalBackend) Mkdir(ctx context.Context, path string) error {
	full, err := secureJoin(l.Root, path)
	if err != nil {
		return err
	}
	return os.MkdirAll(full, 0o755)
}

func (l *LocalBackend) Rename(ctx context.Context, oldPath, newPath string) error {
	oldFull, err := secureJoin(l.Root, oldPath)
	if err != nil {
		return err
	}
	newFull, err := secureJoin(l.Root, newPath)
	if err != nil {
		return err
	}
	return os.Rename(oldFull, newFull)
}

func fileInfoFromOS(path string, fi os.FileInfo) FileInfo {
	return FileInfo{
		Name:    fi.Name(),
		Path:    path,
		Size:    fi.Size(),
		Mode:    fi.Mode(),
		ModTime: fi.ModTime(),
		IsDir:   fi.IsDir(),
	}
}

func secureJoin(root, path string) (string, error) {
	clean := filepath.Clean("/" + path)
	full := filepath.Join(root, clean)
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	absFull, err := filepath.Abs(full)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(absFull, absRoot+string(os.PathSeparator)) && absFull != absRoot {
		return "", ErrPermission
	}
	return absFull, nil
}
