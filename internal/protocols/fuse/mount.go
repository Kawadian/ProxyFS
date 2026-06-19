package fuse

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/lxcfh/lxcfh/internal/auth"
	"github.com/lxcfh/lxcfh/internal/hub"
	"github.com/lxcfh/lxcfh/internal/vfs"
)

const DefaultMountPoint = "/mnt/lxcfh"

// Config configures the FUSE mount.
type Config struct {
	MountPoint  string
	AllowOther  bool
	FSName      string
	DirectMount bool
}

// UIDMapper resolves Hub users to POSIX UID/GID.
type UIDMapper interface {
	Lookup(ctx context.Context, username string) (uid, gid uint32, ok bool)
	Default() (uid, gid uint32)
}

// StaticUIDMapper derives UID/GID from Hub usernames.
type StaticUIDMapper struct {
	Auth *auth.Service
}

// NewHubUIDMapper creates a mapper backed by the auth service.
func NewHubUIDMapper(authSvc *auth.Service) *StaticUIDMapper {
	return &StaticUIDMapper{Auth: authSvc}
}

// Lookup implements UIDMapper.
func (m *StaticUIDMapper) Lookup(ctx context.Context, username string) (uint32, uint32, bool) {
	switch username {
	case "", "root":
		return 0, 0, false
	case "nobody", "guest":
		uid, gid := hub.GuestUIDGID()
		return uid, gid, true
	}
	uid, gid := hub.UIDGID(username)
	return uid, gid, true
}

// Default implements UIDMapper.
func (m *StaticUIDMapper) Default() (uint32, uint32) {
	return hub.GuestUIDGID()
}

// Mounter exposes VirtualFS via FUSE.
type Mounter struct {
	cfg    Config
	vfs    *vfs.VirtualFS
	auth   *auth.Service
	uidMap UIDMapper
	logger *slog.Logger
	server *fuse.Server
	mu     sync.Mutex
}

// NewMounter creates a FUSE mounter.
func NewMounter(cfg Config, vfsFS *vfs.VirtualFS, authSvc *auth.Service, mapper UIDMapper, logger *slog.Logger) *Mounter {
	if cfg.MountPoint == "" {
		cfg.MountPoint = DefaultMountPoint
	}
	if cfg.FSName == "" {
		cfg.FSName = "lxcfh"
	}
	if mapper == nil {
		mapper = NewHubUIDMapper(authSvc)
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Mounter{
		cfg:    cfg,
		vfs:    vfsFS,
		auth:   authSvc,
		uidMap: mapper,
		logger: logger,
	}
}

// Mount mounts VirtualFS at the configured mount point.
func (m *Mounter) Mount(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.server != nil {
		return fmt.Errorf("fuse: already mounted")
	}
	if err := os.MkdirAll(m.cfg.MountPoint, 0o755); err != nil {
		return fmt.Errorf("fuse: mount point: %w", err)
	}
	defUID, defGID := m.uidMap.Default()
	root := &Root{
		vfs:    m.vfs,
		auth:   m.auth,
		uidMap: m.uidMap,
	}
	opts := &fs.Options{
		MountOptions: fuse.MountOptions{
			AllowOther:  m.cfg.AllowOther,
			Name:        m.cfg.FSName,
			FsName:      m.cfg.FSName,
			DirectMount: m.cfg.DirectMount,
		},
		UID: defUID,
		GID: defGID,
	}
	server, err := fs.Mount(m.cfg.MountPoint, root, opts)
	if err != nil {
		return fmt.Errorf("fuse: mount: %w", err)
	}
	m.server = server
	m.logger.Info("fuse mounted", "path", m.cfg.MountPoint)
	go func() {
		<-ctx.Done()
		_ = m.Unmount()
	}()
	return nil
}

// Unmount tears down the FUSE mount.
func (m *Mounter) Unmount() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.server == nil {
		return nil
	}
	err := m.server.Unmount()
	m.server = nil
	return err
}

// Wait blocks until the FUSE server exits.
func (m *Mounter) Wait() {
	m.mu.Lock()
	srv := m.server
	m.mu.Unlock()
	if srv != nil {
		srv.Wait()
	}
}

// MountVirtualFS mounts and blocks until ctx is cancelled.
func MountVirtualFS(ctx context.Context, mountPoint string, vfsFS *vfs.VirtualFS, authSvc *auth.Service, logger *slog.Logger) error {
	m := NewMounter(Config{MountPoint: mountPoint, AllowOther: true}, vfsFS, authSvc, nil, logger)
	if err := m.Mount(ctx); err != nil {
		return err
	}
	m.Wait()
	return nil
}
