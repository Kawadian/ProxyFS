package hub

import (
	"context"
	"fmt"
	"log/slog"
	"path"
	"sync"

	"github.com/lxcfh/lxcfh/internal/models"
	"github.com/lxcfh/lxcfh/internal/nodes"
	sftpprovider "github.com/lxcfh/lxcfh/internal/nodes/sftp"
	"github.com/lxcfh/lxcfh/internal/store"
	"github.com/lxcfh/lxcfh/internal/vfs"
)

// VFSManager keeps VirtualFS mounts in sync with registered SFTP nodes.
type VFSManager struct {
	mu       sync.RWMutex
	fs       *vfs.VirtualFS
	store    *store.Store
	factory  *sftpprovider.Factory
	masterKey []byte
	logger   *slog.Logger
	providers map[string]nodes.NodeProvider
}

// NewVFSManager creates a manager for node-backed VFS mounts.
func NewVFSManager(fs *vfs.VirtualFS, st *store.Store, masterKey []byte, logger *slog.Logger) *VFSManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &VFSManager{
		fs:        fs,
		store:     st,
		factory:   &sftpprovider.Factory{PoolSize: 4},
		masterKey: masterKey,
		logger:    logger,
		providers: make(map[string]nodes.NodeProvider),
	}
}

// Refresh rebuilds all enabled node mounts from the database.
func (m *VFSManager) Refresh(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, p := range m.providers {
		_ = p.Close()
		delete(m.providers, id)
	}

	nodeList, err := m.store.ListNodes(ctx)
	if err != nil {
		return err
	}

	active := make(map[string]struct{}, len(nodeList))
	for _, node := range nodeList {
		if !node.Enabled {
			continue
		}
		slug := node.Slug
		if slug == "" {
			slug = node.Name
		}
		active[node.ID] = struct{}{}

		info := nodes.NodeInfo{
			ID:       node.ID,
			Name:     slug,
			Provider: node.Provider,
			Host:     node.Host,
			Port:     node.Port,
			RootPath: node.RootPath,
			Enabled:  node.Enabled,
		}
		if info.Provider == "" {
			info.Provider = "sftp"
		}
		if info.RootPath == "" {
			info.RootPath = "/"
		}
		if info.Port == 0 {
			info.Port = 22
		}

		cred, err := m.resolveCredential(ctx, node)
		if err != nil {
			m.logger.Warn("node credential resolve failed", "node", slug, "error", err)
			continue
		}

		provider, err := m.factory.Create(ctx, info, cred)
		if err != nil {
			m.logger.Warn("node provider create failed", "node", slug, "error", err)
			continue
		}
		m.providers[node.ID] = provider

		prefix := "/" + slug
		_ = m.fs.RemoveMount(prefix)
		if err := m.fs.AddMount(vfs.Mount{
			Name:     slug,
			Prefix:   prefix,
			Backend:  provider.Backend(),
			ReadOnly: node.ReadOnly,
		}); err != nil {
			m.logger.Warn("vfs mount failed", "node", slug, "error", err)
		}
	}

	for prefix := range m.fs.MountPrefixes() {
		slug := path.Base(prefix)
		found := false
		for _, node := range nodeList {
			ns := node.Slug
			if ns == "" {
				ns = node.Name
			}
			if ns == slug && node.Enabled {
				found = true
				break
			}
		}
		if !found {
			_ = m.fs.RemoveMount(prefix)
		}
	}

	return nil
}

func (m *VFSManager) resolveCredential(ctx context.Context, node models.Node) (nodes.Credential, error) {
	username := node.Username
	var secret []byte
	var passphrase []byte

	if node.CredentialID != "" {
		cred, encSecret, err := m.store.GetCredential(ctx, node.CredentialID)
		if err != nil {
			return nodes.Credential{}, err
		}
		if cred.Username != "" {
			username = cred.Username
		}
		secret = []byte(encSecret)
		_ = cred
	}

	if node.KeyID != "" {
		_, privPEM, err := m.store.GetSSHKey(ctx, node.KeyID)
		if err != nil {
			return nodes.Credential{}, err
		}
		secret = []byte(privPEM)
	}

	if username == "" {
		return nodes.Credential{}, fmt.Errorf("node %s: no username configured", node.Name)
	}

	authType := "password"
	if node.KeyID != "" || (node.CredentialID != "" && len(secret) > 0 && secret[0] == '-') {
		authType = "private_key"
	}

	return nodes.Credential{
		ID:       node.CredentialID,
		NodeID:   node.ID,
		AuthType: authType,
		Username: username,
		Secret:   secret,
		Passphrase: passphrase,
	}, nil
}

// Close releases all node providers.
func (m *VFSManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.providers {
		_ = p.Close()
	}
	m.providers = make(map[string]nodes.NodeProvider)
}
