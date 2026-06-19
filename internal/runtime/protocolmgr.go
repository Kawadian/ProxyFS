package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/lxcfh/lxcfh/internal/auth"
	"github.com/lxcfh/lxcfh/internal/config"
	"github.com/lxcfh/lxcfh/internal/models"
	fuseproto "github.com/lxcfh/lxcfh/internal/protocols/fuse"
	"github.com/lxcfh/lxcfh/internal/protocols/sftpserver"
	"github.com/lxcfh/lxcfh/internal/protocols/webdav"
	"github.com/lxcfh/lxcfh/internal/samba"
	"github.com/lxcfh/lxcfh/internal/store"
	"github.com/lxcfh/lxcfh/internal/vfs"
)

// ProtocolManager starts and stops protocol services based on Web UI settings.
type ProtocolManager struct {
	cfg    *config.Config
	store  *store.Store
	sftp   *sftpserver.Server
	webdav *webdav.Server
	vfs    *vfs.VirtualFS
	auth   *auth.Service
	logger *slog.Logger

	mu         sync.Mutex
	fuse       *fuseproto.Mounter
	fuseUp     bool
	smbdCmd    *exec.Cmd
	nmbdCmd    *exec.Cmd
	lastSMBErr string
}

// NewProtocolManager wires runtime protocol control.
func NewProtocolManager(
	cfg *config.Config,
	st *store.Store,
	sftpSrv *sftpserver.Server,
	webdavSrv *webdav.Server,
	vfsFS *vfs.VirtualFS,
	authSvc *auth.Service,
	logger *slog.Logger,
) *ProtocolManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &ProtocolManager{
		cfg:    cfg,
		store:  st,
		sftp:   sftpSrv,
		webdav: webdavSrv,
		vfs:    vfsFS,
		auth:   authSvc,
		logger: logger,
	}
}

// Apply reconciles running services with settings.
func (m *ProtocolManager) Apply(ctx context.Context, settings models.Settings) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	proto := settings.Protocols
	if proto == (models.ProtocolSettings{}) {
		proto = models.DefaultProtocolSettings()
	}

	if proto.SFTPEnabled {
		if !m.sftp.IsRunning() {
			if err := m.sftp.Start(); err != nil {
				return fmt.Errorf("start sftp: %w", err)
			}
		}
	} else if m.sftp.IsRunning() {
		if err := m.sftp.Stop(); err != nil {
			return fmt.Errorf("stop sftp: %w", err)
		}
	}

	m.webdav.SetEnabled(proto.WebDAVEnabled)

	if proto.SMBEnabled {
		if err := m.startSMBLocked(ctx); err != nil {
			m.lastSMBErr = err.Error()
			return fmt.Errorf("start smb: %w", err)
		}
		m.lastSMBErr = ""
	} else {
		m.stopSMBLocked()
		m.lastSMBErr = ""
	}

	return nil
}

// Status returns current protocol runtime state.
func (m *ProtocolManager) Status(settings models.Settings) models.ProtocolsOverview {
	m.mu.Lock()
	defer m.mu.Unlock()

	proto := settings.Protocols
	if proto == (models.ProtocolSettings{}) {
		proto = models.DefaultProtocolSettings()
	}

	sftpPort := m.cfg.SFTPPort
	if sftpPort == 0 {
		sftpPort = sftpserver.DefaultPort
	}

	return models.ProtocolsOverview{
		Protocols: []models.ProtocolStatus{
			{
				Name:    "sftp",
				Enabled: proto.SFTPEnabled,
				Running: m.sftp.IsRunning(),
				Port:    sftpPort,
			},
			{
				Name:    "webdav",
				Enabled: proto.WebDAVEnabled,
				Running: m.webdav.IsEnabled(),
				Port:    m.cfg.BindPort,
				Path:    webdav.MountPath,
			},
			{
				Name:    "smb",
				Enabled: proto.SMBEnabled,
				Running: m.smbRunningLocked(),
				Port:    445,
				Path:    m.cfg.FuseMountPath,
				Message: m.smbStatusMessageLocked(proto.SMBEnabled),
			},
		},
	}
}

func (m *ProtocolManager) smbStatusMessageLocked(enabled bool) string {
	if !enabled {
		return ""
	}
	if m.lastSMBErr != "" {
		return m.lastSMBErr
	}
	if !m.fuseCapable() {
		return "FUSE unavailable (/dev/fuse missing); SMB cannot start in this environment"
	}
	if !m.sambaInstalled() {
		return "Samba binaries not installed in this image"
	}
	return ""
}

func (m *ProtocolManager) startSMBLocked(ctx context.Context) error {
	if !m.fuseCapable() {
		return fmt.Errorf("fuse unavailable")
	}
	if !m.sambaInstalled() {
		return fmt.Errorf("samba not installed")
	}

	if m.smbRunningLocked() {
		return nil
	}

	mountPoint := m.cfg.FuseMountPath
	if mountPoint == "" {
		mountPoint = fuseproto.DefaultMountPoint
	}
	if err := os.MkdirAll(mountPoint, 0o755); err != nil {
		return err
	}

	if !m.fuseUp {
		if m.fuse == nil {
			m.fuse = fuseproto.NewMounter(fuseproto.Config{
				MountPoint:  mountPoint,
				AllowOther:  true,
				FSName:      "lxcfh",
				DirectMount: true,
			}, m.vfs, m.auth, nil, m.logger)
		}
		if err := m.fuse.Mount(context.Background()); err != nil {
			return err
		}
		m.fuseUp = true
	}

	if err := m.ensureSMBConfig(mountPoint); err != nil {
		return err
	}

	if err := os.MkdirAll("/run/samba", 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll("/var/log/samba", 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(m.cfg.DataDir, "samba"), 0o700); err != nil {
		return err
	}

	if err := m.syncSMBUsersLocked(ctx); err != nil {
		m.logger.Warn("samba user sync on start", "error", err)
	}

	m.nmbdCmd = exec.CommandContext(ctx, "nmbd", "--foreground", "--no-process-group")
	m.nmbdCmd.Stdout = os.Stdout
	m.nmbdCmd.Stderr = os.Stderr
	if err := m.nmbdCmd.Start(); err != nil {
		m.stopFuseLocked()
		return fmt.Errorf("nmbd: %w", err)
	}

	m.smbdCmd = exec.CommandContext(ctx, "smbd", "--foreground", "--no-process-group")
	m.smbdCmd.Stdout = os.Stdout
	m.smbdCmd.Stderr = os.Stderr
	if err := m.smbdCmd.Start(); err != nil {
		_ = m.stopProcess(m.nmbdCmd)
		m.nmbdCmd = nil
		m.stopFuseLocked()
		return fmt.Errorf("smbd: %w", err)
	}

	m.logger.Info("smb started", "share", mountPoint)
	return nil
}

// SyncSMBUsers applies Hub user changes immediately while SMB is running.
func (m *ProtocolManager) SyncSMBUsers(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.smbRunningLocked() {
		return nil
	}
	return m.syncSMBUsersLocked(ctx)
}

func (m *ProtocolManager) syncSMBUsersLocked(ctx context.Context) error {
	syncer := samba.NewSyncer(m.store.DB(), samba.Config{}, m.logger)
	_, err := syncer.SyncAll(ctx, nil)
	return err
}

func (m *ProtocolManager) stopSMBLocked() {
	if m.smbdCmd != nil {
		_ = m.stopProcess(m.smbdCmd)
		m.smbdCmd = nil
	}
	if m.nmbdCmd != nil {
		_ = m.stopProcess(m.nmbdCmd)
		m.nmbdCmd = nil
	}
	m.stopFuseLocked()
}

func (m *ProtocolManager) stopFuseLocked() {
	if m.fuse != nil {
		_ = m.fuse.Unmount()
		m.fuseUp = false
	}
}

func (m *ProtocolManager) stopProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	_ = cmd.Process.Signal(os.Interrupt)
	_, _ = cmd.Process.Wait()
	return nil
}

func (m *ProtocolManager) smbRunningLocked() bool {
	return m.smbdCmd != nil && m.smbdCmd.Process != nil
}

func (m *ProtocolManager) fuseCapable() bool {
	_, err := os.Stat("/dev/fuse")
	return err == nil
}

func (m *ProtocolManager) sambaInstalled() bool {
	_, err := exec.LookPath("smbd")
	return err == nil
}

func (m *ProtocolManager) ensureSMBConfig(sharePath string) error {
	confPath := "/etc/samba/smb.conf"
	if err := os.MkdirAll(filepath.Dir(confPath), 0o755); err != nil {
		return err
	}
	shareName := os.Getenv("SMB_SHARE_NAME")
	if shareName == "" {
		shareName = "lxcfh"
	}
	content := fmt.Sprintf(`[global]
   workgroup = LXCFH
   server string = LXC File Hub
   security = user
   map to guest = Bad User
   passdb backend = tdbsam:%s
   load printers = no
   printing = bsd
   disable spoolss = yes
   server min protocol = SMB2
   pid directory = /run/samba
   log file = /var/log/samba/log.%%m

[%s]
   comment = LXC File Hub
   path = %s
   browseable = yes
   read only = no
   guest ok = no
`, filepath.Join(m.cfg.DataDir, "samba", "passdb.tdb"), shareName, sharePath)
	return os.WriteFile(confPath, []byte(content), 0o644)
}

// Shutdown stops all managed protocol services.
func (m *ProtocolManager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sftp.IsRunning() {
		_ = m.sftp.Stop()
	}
	m.webdav.SetEnabled(false)
	m.stopSMBLocked()
}
