package fuse

import (
	"context"
	"io"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/lxcfh/lxcfh/internal/auth"
	"github.com/lxcfh/lxcfh/internal/vfs"
)

// Root is the FUSE filesystem root backed by VirtualFS.
type Root struct {
	fs.Inode
	vfs    *vfs.VirtualFS
	auth   *auth.Service
	uidMap UIDMapper
}

var (
	_ fs.NodeLookuper  = (*Root)(nil)
	_ fs.NodeReaddirer = (*Root)(nil)
	_ fs.NodeGetattrer = (*Root)(nil)
	_ fs.NodeCreater   = (*Root)(nil)
	_ fs.NodeMkdirer   = (*Root)(nil)
	_ fs.NodeUnlinker  = (*Root)(nil)
	_ fs.NodeRmdirer   = (*Root)(nil)
	_ fs.NodeRenamer   = (*Root)(nil)
)

func (r *Root) vpath(name string) string {
	if name == "" || name == "." {
		return "/"
	}
	if name[0] == '/' {
		return name
	}
	return "/" + name
}

func (r *Root) actingUser() *auth.User {
	return &auth.User{ID: "fuse", Username: "fuse", Role: auth.RoleAdmin, Enabled: true}
}

// Lookup finds a child node.
func (r *Root) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	path := r.vpath(name)
	info, err := r.vfs.Stat(ctx, path)
	if err != nil {
		return nil, syscall.ENOENT
	}
	child := r.newNode(path, info)
	out.NodeId = child.StableAttr().Ino
	r.fillAttr(out, info)
	return child, 0
}

// Readdir lists root children.
func (r *Root) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	return r.readDirStream(ctx, "/")
}

// Getattr returns root attributes.
func (r *Root) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	info, err := r.vfs.Stat(ctx, "/")
	if err != nil {
		return syscall.ENOENT
	}
	r.fillAttrOut(out, info)
	return 0
}

// Create creates a new file under root.
func (r *Root) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	path := r.vpath(name)
	if _, err := r.vfs.Write(ctx, path, 0, &emptyReader{}); err != nil {
		return nil, nil, 0, mapErr(err)
	}
	info, err := r.vfs.Stat(ctx, path)
	if err != nil {
		return nil, nil, 0, syscall.EIO
	}
	child := r.newNode(path, info)
	r.fillAttr(out, info)
	fh := &fileHandle{vfs: r.vfs, path: path, user: r.actingUser()}
	return child, fh, 0, 0
}

// Mkdir creates a directory under root.
func (r *Root) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	path := r.vpath(name)
	if err := r.vfs.Mkdir(ctx, path); err != nil {
		return nil, mapErr(err)
	}
	info, err := r.vfs.Stat(ctx, path)
	if err != nil {
		return nil, syscall.EIO
	}
	child := r.newNode(path, info)
	r.fillAttr(out, info)
	return child, 0
}

// Unlink removes a file under root.
func (r *Root) Unlink(ctx context.Context, name string) syscall.Errno {
	return mapErr(r.vfs.Remove(ctx, r.vpath(name)))
}

// Rmdir removes a directory under root.
func (r *Root) Rmdir(ctx context.Context, name string) syscall.Errno {
	return mapErr(r.vfs.Remove(ctx, r.vpath(name)))
}

// Rename renames a child of root.
func (r *Root) Rename(ctx context.Context, name string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	src := r.vpath(name)
	dstParent, ok := newParent.(*Root)
	if !ok {
		node, ok := newParent.(*VNode)
		if !ok {
			return syscall.EXDEV
		}
		dst := node.vpath(newName)
		return mapErr(renameVFS(ctx, r.vfs, src, dst))
	}
	dst := dstParent.vpath(newName)
	return mapErr(renameVFS(ctx, r.vfs, src, dst))
}

func (r *Root) newNode(path string, info vfs.FileInfo) *fs.Inode {
	n := &VNode{vfs: r.vfs, auth: r.auth, uidMap: r.uidMap, path: path}
	if info.IsDir {
		return r.NewInode(context.Background(), n, fs.StableAttr{Mode: fuse.S_IFDIR})
	}
	return r.NewInode(context.Background(), n, fs.StableAttr{Mode: fuse.S_IFREG})
}

func (r *Root) readDirStream(ctx context.Context, dirPath string) (fs.DirStream, syscall.Errno) {
	entries, err := r.vfs.ReadDir(ctx, dirPath)
	if err != nil {
		return nil, mapErr(err)
	}
	var items []fuse.DirEntry
	for _, e := range entries {
		mode := uint32(fuse.S_IFREG)
		if e.IsDir {
			mode = fuse.S_IFDIR
		}
		items = append(items, fuse.DirEntry{Name: e.Name, Mode: mode})
	}
	return fs.NewListDirStream(items), 0
}

func (r *Root) fillAttr(out *fuse.EntryOut, info vfs.FileInfo) {
	fillAttr(&out.Attr, info, r.uidMap)
}

func (r *Root) fillAttrOut(out *fuse.AttrOut, info vfs.FileInfo) {
	fillAttr(&out.Attr, info, r.uidMap)
}

func fillAttr(out *fuse.Attr, info vfs.FileInfo, uidMap UIDMapper) {
	uid, gid := uidMap.Default()
	if uidMap != nil {
		if u, g, ok := uidMap.Lookup(context.Background(), info.Name); ok {
			uid, gid = u, g
		}
	}
	mode := uint32(info.Mode.Perm())
	if info.IsDir {
		mode |= fuse.S_IFDIR
	} else {
		mode |= fuse.S_IFREG
	}
	out.Mode = mode
	out.Size = uint64(info.Size)
	out.Mtime = uint64(info.ModTime.Unix())
	out.Mtimensec = uint32(info.ModTime.Nanosecond())
	out.Uid = uid
	out.Gid = gid
	out.Atime = out.Mtime
	out.Ctime = out.Mtime
}

// VNode represents a virtual filesystem node below root.
type VNode struct {
	fs.Inode
	vfs    *vfs.VirtualFS
	auth   *auth.Service
	uidMap UIDMapper
	path   string
}

var (
	_ fs.NodeLookuper  = (*VNode)(nil)
	_ fs.NodeReaddirer = (*VNode)(nil)
	_ fs.NodeGetattrer = (*VNode)(nil)
	_ fs.NodeOpener    = (*VNode)(nil)
	_ fs.NodeCreater   = (*VNode)(nil)
	_ fs.NodeMkdirer   = (*VNode)(nil)
	_ fs.NodeUnlinker  = (*VNode)(nil)
	_ fs.NodeRmdirer   = (*VNode)(nil)
	_ fs.NodeRenamer   = (*VNode)(nil)
)

func (n *VNode) vpath(name string) string {
	if name == "" {
		return n.path
	}
	if name[0] == '/' {
		return name
	}
	if n.path == "/" {
		return "/" + name
	}
	return n.path + "/" + name
}

func (n *VNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	path := n.vpath(name)
	info, err := n.vfs.Stat(ctx, path)
	if err != nil {
		return nil, syscall.ENOENT
	}
	child := n.newChild(path, info)
	root := &Root{vfs: n.vfs, auth: n.auth, uidMap: n.uidMap}
	root.fillAttr(out, info)
	out.NodeId = child.StableAttr().Ino
	return child, 0
}

func (n *VNode) newChild(path string, info vfs.FileInfo) *fs.Inode {
	v := &VNode{vfs: n.vfs, auth: n.auth, uidMap: n.uidMap, path: path}
	if info.IsDir {
		return n.NewInode(context.Background(), v, fs.StableAttr{Mode: fuse.S_IFDIR})
	}
	return n.NewInode(context.Background(), v, fs.StableAttr{Mode: fuse.S_IFREG})
}

func (n *VNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	entries, err := n.vfs.ReadDir(ctx, n.path)
	if err != nil {
		return nil, mapErr(err)
	}
	var items []fuse.DirEntry
	for _, e := range entries {
		mode := uint32(fuse.S_IFREG)
		if e.IsDir {
			mode = fuse.S_IFDIR
		}
		items = append(items, fuse.DirEntry{Name: e.Name, Mode: mode})
	}
	return fs.NewListDirStream(items), 0
}

func (n *VNode) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	info, err := n.vfs.Stat(ctx, n.path)
	if err != nil {
		return syscall.ENOENT
	}
	root := &Root{vfs: n.vfs, auth: n.auth, uidMap: n.uidMap}
	root.fillAttrOut(out, info)
	return 0
}

func (n *VNode) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	return &fileHandle{vfs: n.vfs, path: n.path, user: n.actingUser()}, 0, 0
}

func (n *VNode) actingUser() *auth.User {
	return &auth.User{ID: "fuse", Username: "fuse", Role: auth.RoleAdmin, Enabled: true}
}

func (n *VNode) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	path := n.vpath(name)
	if _, err := n.vfs.Write(ctx, path, 0, &emptyReader{}); err != nil {
		return nil, nil, 0, mapErr(err)
	}
	info, err := n.vfs.Stat(ctx, path)
	if err != nil {
		return nil, nil, 0, syscall.EIO
	}
	child := n.newChild(path, info)
	root := &Root{vfs: n.vfs, auth: n.auth, uidMap: n.uidMap}
	root.fillAttr(out, info)
	return child, &fileHandle{vfs: n.vfs, path: path, user: n.actingUser()}, 0, 0
}

func (n *VNode) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	path := n.vpath(name)
	if err := n.vfs.Mkdir(ctx, path); err != nil {
		return nil, mapErr(err)
	}
	info, err := n.vfs.Stat(ctx, path)
	if err != nil {
		return nil, syscall.EIO
	}
	child := n.newChild(path, info)
	root := &Root{vfs: n.vfs, auth: n.auth, uidMap: n.uidMap}
	root.fillAttr(out, info)
	return child, 0
}

func (n *VNode) Unlink(ctx context.Context, name string) syscall.Errno {
	return mapErr(n.vfs.Remove(ctx, n.vpath(name)))
}

func (n *VNode) Rmdir(ctx context.Context, name string) syscall.Errno {
	return mapErr(n.vfs.Remove(ctx, n.vpath(name)))
}

func (n *VNode) Rename(ctx context.Context, name string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	src := n.vpath(name)
	var dst string
	switch p := newParent.(type) {
	case *VNode:
		dst = p.vpath(newName)
	case *Root:
		dst = p.vpath(newName)
	default:
		return syscall.EXDEV
	}
	return mapErr(renameVFS(ctx, n.vfs, src, dst))
}

type fileHandle struct {
	vfs  *vfs.VirtualFS
	path string
	user *auth.User
}

var (
	_ fs.FileReader = (*fileHandle)(nil)
	_ fs.FileWriter = (*fileHandle)(nil)
)

func (f *fileHandle) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	rc, err := f.vfs.Open(ctx, f.path)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, syscall.EIO
	}
	if off >= int64(len(data)) {
		return fuse.ReadResultData(nil), 0
	}
	n := copy(dest, data[off:])
	return fuse.ReadResultData(dest[:n]), 0
}

func (f *fileHandle) Write(ctx context.Context, data []byte, off int64) (uint32, syscall.Errno) {
	_, err := f.vfs.Write(ctx, f.path, off, &byteSliceReader{b: data})
	if err != nil {
		return 0, mapErr(err)
	}
	return uint32(len(data)), 0
}

func (f *fileHandle) Flush(ctx context.Context) syscall.Errno   { return 0 }
func (f *fileHandle) Release(ctx context.Context) syscall.Errno { return 0 }

type byteSliceReader struct {
	b []byte
	i int
}

func (r *byteSliceReader) Read(p []byte) (int, error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}

type emptyReader struct{}

func (e *emptyReader) Read(p []byte) (int, error) { return 0, io.EOF }

func renameVFS(ctx context.Context, vfsFS *vfs.VirtualFS, src, dst string) error {
	rc, err := vfsFS.Open(ctx, src)
	if err != nil {
		return err
	}
	data, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		return err
	}
	if _, err := vfsFS.Write(ctx, dst, 0, &byteSliceReader{b: data}); err != nil {
		return err
	}
	return vfsFS.Remove(ctx, src)
}

func mapErr(err error) syscall.Errno {
	if err == nil {
		return 0
	}
	switch err {
	case vfs.ErrNotFound:
		return syscall.ENOENT
	case vfs.ErrPermission:
		return syscall.EACCES
	case vfs.ErrIsDirectory:
		return syscall.EISDIR
	case vfs.ErrNotDirectory:
		return syscall.ENOTDIR
	default:
		return syscall.EIO
	}
}
