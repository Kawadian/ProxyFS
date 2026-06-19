package sftpserver

import (
	"context"
	"errors"
	"io"
	"os"
	"path"
	"sync"
	"syscall"
	"time"

	"github.com/lxcfh/lxcfh/internal/auth"
	"github.com/lxcfh/lxcfh/internal/hub"
	"github.com/lxcfh/lxcfh/internal/rbac"
	"github.com/lxcfh/lxcfh/internal/vfs"
	"github.com/pkg/sftp"
)

// Handler serves SFTP requests against VirtualFS with RBAC enforcement.
type Handler struct {
	fs   *vfs.VirtualFS
	rbac rbac.Checker
	user *auth.User
	hub  *hub.User
	ctx  context.Context
}

// NewHandler creates an SFTP handler bound to an authenticated user.
func NewHandler(ctx context.Context, fs *vfs.VirtualFS, checker rbac.Checker, user *auth.User) *Handler {
	roles := []string{string(user.Role)}
	h := hub.FromRoles(user.ID, user.Username, roles, user.Role == auth.RoleGuest)
	return &Handler{
		fs:   fs,
		rbac: checker,
		user: user,
		hub:  h,
		ctx:  ctx,
	}
}

// Handlers returns pkg/sftp handlers for the request server.
func (h *Handler) Handlers() sftp.Handlers {
	return sftp.Handlers{
		FileGet:  h,
		FilePut:  h,
		FileCmd:  h,
		FileList: h,
	}
}

func (h *Handler) vfsPath(reqPath string) (string, error) {
	return vfs.ResolveVirtualPath(reqPath)
}

func (h *Handler) authorize(vfsPath string, perm rbac.Permission) error {
	return h.rbac.Check(h.ctx, h.hub, vfsPath, perm)
}

// Fileread opens a file for reading.
func (h *Handler) Fileread(r *sftp.Request) (io.ReaderAt, error) {
	if r.Method != "Read" {
		return nil, os.ErrInvalid
	}
	p, err := h.vfsPath(r.Filepath)
	if err != nil {
		return nil, syscall.ENOENT
	}
	if err := h.authorize(p, rbac.Read); err != nil {
		return nil, syscall.EACCES
	}
	rc, err := h.fs.Open(h.ctx, p)
	if err != nil {
		return nil, mapVFSError(err)
	}
	data, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		return nil, err
	}
	return readerAt{data: data}, nil
}

// Filewrite opens a file for writing.
func (h *Handler) Filewrite(r *sftp.Request) (io.WriterAt, error) {
	p, err := h.vfsPath(r.Filepath)
	if err != nil {
		return nil, syscall.ENOENT
	}
	if err := h.authorize(p, rbac.Write); err != nil {
		return nil, syscall.EACCES
	}
	return &vfsWriterAt{
		ctx:  h.ctx,
		fs:   h.fs,
		path: p,
	}, nil
}

// Filecmd handles mkdir, remove, rename, etc.
func (h *Handler) Filecmd(r *sftp.Request) error {
	switch r.Method {
	case "Setstat":
		p, err := h.vfsPath(r.Filepath)
		if err != nil {
			return syscall.ENOENT
		}
		return h.authorize(p, rbac.Write)
	case "Rename":
		src, err := h.vfsPath(r.Filepath)
		if err != nil {
			return syscall.ENOENT
		}
		dst, err := h.vfsPath(r.Target)
		if err != nil {
			return syscall.ENOENT
		}
		if err := h.authorize(src, rbac.Write); err != nil {
			return syscall.EACCES
		}
		if err := h.authorize(dst, rbac.Write); err != nil {
			return syscall.EACCES
		}
		return mapVFSError(h.rename(src, dst))
	case "Rmdir":
		p, err := h.vfsPath(r.Filepath)
		if err != nil {
			return syscall.ENOENT
		}
		if err := h.authorize(p, rbac.Delete); err != nil {
			return syscall.EACCES
		}
		return mapVFSError(h.fs.Remove(h.ctx, p))
	case "Mkdir":
		p, err := h.vfsPath(r.Filepath)
		if err != nil {
			return syscall.ENOENT
		}
		if err := h.authorize(p, rbac.Write); err != nil {
			return syscall.EACCES
		}
		return mapVFSError(h.fs.Mkdir(h.ctx, p))
	case "Remove":
		p, err := h.vfsPath(r.Filepath)
		if err != nil {
			return syscall.ENOENT
		}
		if err := h.authorize(p, rbac.Delete); err != nil {
			return syscall.EACCES
		}
		return mapVFSError(h.fs.Remove(h.ctx, p))
	default:
		return os.ErrInvalid
	}
}

func (h *Handler) rename(src, dst string) error {
	if err := h.fs.Remove(h.ctx, dst); err != nil && !errors.Is(err, vfs.ErrNotFound) {
		// destination may not exist
	}
	data, err := h.fs.Open(h.ctx, src)
	if err != nil {
		return err
	}
	content, err := io.ReadAll(data)
	_ = data.Close()
	if err != nil {
		return err
	}
	if _, err := h.fs.Write(h.ctx, dst, 0, &byteReader{b: content}); err != nil {
		return err
	}
	return h.fs.Remove(h.ctx, src)
}

// Filestat returns file metadata.
func (h *Handler) Filestat(r *sftp.Request) (sftp.ListerAt, error) {
	p, err := h.vfsPath(r.Filepath)
	if err != nil {
		return nil, syscall.ENOENT
	}
	if err := h.authorize(p, rbac.Read); err != nil {
		return nil, syscall.EACCES
	}
	info, err := h.fs.Stat(h.ctx, p)
	if err != nil {
		return nil, mapVFSError(err)
	}
	return listerAt{entries: []os.FileInfo{fileInfo{info}}}, nil
}

// Filelist lists directory contents.
func (h *Handler) Filelist(r *sftp.Request) (sftp.ListerAt, error) {
	p, err := h.vfsPath(r.Filepath)
	if err != nil {
		return nil, syscall.ENOENT
	}
	if err := h.authorize(p, rbac.List); err != nil {
		return nil, syscall.EACCES
	}
	switch r.Method {
	case "List":
		entries, err := h.fs.ReadDir(h.ctx, p)
		if err != nil {
			return nil, mapVFSError(err)
		}
		infos := make([]os.FileInfo, len(entries))
		for i, e := range entries {
			infos[i] = fileInfo{e}
		}
		return listerAt{entries: infos}, nil
	case "Stat":
		info, err := h.fs.Stat(h.ctx, p)
		if err != nil {
			return nil, mapVFSError(err)
		}
		return listerAt{entries: []os.FileInfo{fileInfo{info}}}, nil
	default:
		return nil, os.ErrInvalid
	}
}

type readerAt struct {
	data []byte
}

func (r readerAt) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(r.data)) {
		return 0, io.EOF
	}
	n := copy(p, r.data[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

type byteReader struct {
	b []byte
	i int
}

func (r *byteReader) Read(p []byte) (int, error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}

type vfsWriterAt struct {
	ctx  context.Context
	fs   *vfs.VirtualFS
	path string
	mu   sync.Mutex
}

func (w *vfsWriterAt) WriteAt(p []byte, off int64) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, err := w.fs.Write(w.ctx, w.path, off, &byteReader{b: p})
	if err != nil {
		return 0, mapVFSError(err)
	}
	return len(p), nil
}

type listerAt struct {
	entries []os.FileInfo
}

func (l listerAt) ListAt(f []os.FileInfo, offset int64) (int, error) {
	if offset >= int64(len(l.entries)) {
		return 0, io.EOF
	}
	n := copy(f, l.entries[offset:])
	if int(offset)+n < len(l.entries) {
		return n, nil
	}
	return n, io.EOF
}

type fileInfo struct {
	vfs.FileInfo
}

func (f fileInfo) Name() string       { return path.Base(f.Path) }
func (f fileInfo) Size() int64        { return f.FileInfo.Size }
func (f fileInfo) Mode() os.FileMode  { return f.FileInfo.Mode }
func (f fileInfo) ModTime() time.Time { return f.FileInfo.ModTime }
func (f fileInfo) IsDir() bool        { return f.FileInfo.IsDir }
func (f fileInfo) Sys() interface{}   { return nil }

func mapVFSError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, vfs.ErrNotFound):
		return syscall.ENOENT
	case errors.Is(err, vfs.ErrPermission):
		return syscall.EACCES
	case errors.Is(err, vfs.ErrIsDirectory):
		return syscall.EISDIR
	case errors.Is(err, vfs.ErrNotDirectory):
		return syscall.ENOTDIR
	default:
		return err
	}
}
