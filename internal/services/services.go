package services

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lxcfh/lxcfh/internal/crypto"
	"github.com/lxcfh/lxcfh/internal/hub"
	"github.com/lxcfh/lxcfh/internal/models"
	"github.com/lxcfh/lxcfh/internal/nodesbackup"
	"github.com/lxcfh/lxcfh/internal/store"
	"github.com/lxcfh/lxcfh/internal/vfs"
	"golang.org/x/crypto/ssh"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrInvalidInput = errors.New("invalid input")
)

type Services struct {
	Store             *store.Store
	BackupDir         string
	DataDir           string
	VFS               *vfs.VirtualFS
	VFSManager        *hub.VFSManager
	MasterKey         []byte
	OnNodesChanged    func(ctx context.Context)
	OnSettingsChanged func(ctx context.Context, settings models.Settings) error
}

func New(st *store.Store, dataDir string) *Services {
	return &Services{
		Store:     st,
		BackupDir: filepath.Join(dataDir, "backups"),
		DataDir:   dataDir,
	}
}

func (s *Services) Ready(ctx context.Context) error {
	return s.Store.Ping(ctx)
}

func (s *Services) IsSetup(ctx context.Context) (bool, error) {
	return s.Store.IsSetup(ctx)
}

func (s *Services) Setup(ctx context.Context, username, password, displayName string) (models.User, models.Session, error) {
	setup, err := s.Store.IsSetup(ctx)
	if err != nil {
		return models.User{}, models.Session{}, err
	}
	if setup {
		return models.User{}, models.Session{}, fmt.Errorf("%w: already configured", store.ErrConflict)
	}
	if username == "" || password == "" {
		return models.User{}, models.Session{}, ErrInvalidInput
	}
	hash, err := crypto.HashPassword(password)
	if err != nil {
		return models.User{}, models.Session{}, err
	}
	user, err := s.Store.CreateUser(ctx, username, displayName, "", hash, models.RoleAdmin)
	if err != nil {
		return models.User{}, models.Session{}, err
	}
	sess, err := s.createSession(ctx, user.ID)
	if err != nil {
		return models.User{}, models.Session{}, err
	}
	_ = s.requestSambaSync(ctx, username, password)
	return user, sess, nil
}

func (s *Services) Login(ctx context.Context, username, password string) (models.User, models.Session, error) {
	user, hash, err := s.Store.GetUserByUsername(ctx, username)
	if err != nil {
		return models.User{}, models.Session{}, ErrUnauthorized
	}
	if !user.Enabled {
		return models.User{}, models.Session{}, ErrForbidden
	}
	ok, err := crypto.VerifyPassword(password, hash)
	if err != nil || !ok {
		return models.User{}, models.Session{}, ErrUnauthorized
	}
	_ = s.Store.RecordLogin(ctx, user.ID)
	sess, err := s.createSession(ctx, user.ID)
	return user, sess, err
}

func (s *Services) createSession(ctx context.Context, userID string) (models.Session, error) {
	settings, err := s.Store.GetSettings(ctx)
	if err != nil {
		return models.Session{}, err
	}
	timeout := time.Duration(settings.SessionTimeoutMin) * time.Minute
	if timeout <= 0 {
		timeout = time.Hour
	}
	csrf := uuid.NewString()
	return s.Store.CreateSession(ctx, userID, csrf, time.Now().Add(timeout))
}

func (s *Services) Logout(ctx context.Context, sessionID string) error {
	return s.Store.DeleteSession(ctx, sessionID)
}

func (s *Services) GetSession(ctx context.Context, sessionID string) (models.Session, models.User, error) {
	sess, err := s.Store.GetSession(ctx, sessionID)
	if err != nil {
		return models.Session{}, models.User{}, err
	}
	user, err := s.Store.GetUser(ctx, sess.UserID)
	if err != nil {
		return models.Session{}, models.User{}, err
	}
	return sess, user, nil
}

func (s *Services) Reauth(ctx context.Context, sessionID, password string) error {
	sess, user, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	_, hash, err := s.Store.GetUserByUsername(ctx, user.Username)
	if err != nil {
		return ErrUnauthorized
	}
	ok, err := crypto.VerifyPassword(password, hash)
	if err != nil || !ok {
		return ErrUnauthorized
	}
	settings, _ := s.Store.GetSettings(ctx)
	timeout := time.Duration(settings.SessionTimeoutMin) * time.Minute
	newSess, err := s.Store.CreateSession(ctx, sess.UserID, uuid.NewString(), time.Now().Add(timeout))
	if err != nil {
		return err
	}
	_ = s.Store.DeleteSession(ctx, sessionID)
	_ = newSess
	return nil
}

func (s *Services) ListUsers(ctx context.Context) ([]models.User, error) {
	return s.Store.ListUsers(ctx)
}

func (s *Services) CreateUser(ctx context.Context, username, password, displayName, email string, role models.Role) (models.User, error) {
	if username == "" || password == "" {
		return models.User{}, ErrInvalidInput
	}
	hash, err := crypto.HashPassword(password)
	if err != nil {
		return models.User{}, err
	}
	user, err := s.Store.CreateUser(ctx, username, displayName, email, hash, role)
	if err != nil {
		return models.User{}, err
	}
	_ = s.requestSambaSync(ctx, username, password)
	return user, nil
}

func (s *Services) GetUser(ctx context.Context, id string) (models.User, error) {
	return s.Store.GetUser(ctx, id)
}

func (s *Services) UpdateUser(ctx context.Context, id string, displayName, email *string, role *models.Role, enabled *bool) (models.User, error) {
	user, err := s.Store.UpdateUser(ctx, id, displayName, email, role, enabled)
	if err != nil {
		return models.User{}, err
	}
	_ = s.requestSambaSync(ctx, user.Username, "")
	return user, nil
}

func (s *Services) DeleteUser(ctx context.Context, id string) error {
	user, err := s.Store.GetUser(ctx, id)
	if err != nil {
		return err
	}
	if err := s.Store.DeleteUser(ctx, id); err != nil {
		return err
	}
	_ = s.requestSambaSync(ctx, user.Username, "")
	return nil
}

func (s *Services) ChangePassword(ctx context.Context, id, password string) error {
	if password == "" {
		return ErrInvalidInput
	}
	user, err := s.Store.GetUser(ctx, id)
	if err != nil {
		return err
	}
	hash, err := crypto.HashPassword(password)
	if err != nil {
		return err
	}
	if err := s.Store.UpdateUserPassword(ctx, id, hash); err != nil {
		return err
	}
	_ = s.requestSambaSync(ctx, user.Username, password)
	return nil
}

func (s *Services) requestSambaSync(ctx context.Context, username, password string) error {
	if username != "" && password != "" {
		_ = s.Store.SetSambaPendingPassword(ctx, username, password)
	}
	settings, err := s.Store.GetSettings(ctx)
	if err != nil {
		return err
	}
	if !settings.Protocols.SMBEnabled {
		return nil
	}
	return s.Store.BumpSambaSyncNonce(ctx)
}

func (s *Services) ListUserSSHKeys(ctx context.Context, userID string) ([]models.UserSSHKey, error) {
	return s.Store.ListUserSSHKeys(ctx, userID)
}

func (s *Services) AddUserSSHKey(ctx context.Context, userID, name, publicKey string) (models.UserSSHKey, error) {
	fp, err := fingerprintPublicKey(publicKey)
	if err != nil {
		return models.UserSSHKey{}, err
	}
	return s.Store.AddUserSSHKey(ctx, userID, name, fp, strings.TrimSpace(publicKey))
}

func (s *Services) DeleteUserSSHKey(ctx context.Context, userID, keyID string) error {
	return s.Store.DeleteUserSSHKey(ctx, userID, keyID)
}

func (s *Services) ListNodes(ctx context.Context) ([]models.Node, error) {
	return s.Store.ListNodes(ctx)
}

func (s *Services) CreateNode(ctx context.Context, n models.Node) (models.Node, error) {
	if n.Name == "" || n.Host == "" {
		return models.Node{}, ErrInvalidInput
	}
	if n.Port == 0 {
		settings, _ := s.Store.GetSettings(ctx)
		n.Port = settings.DefaultNodePort
	}
	if n.HostKeyStatus == "" {
		n.HostKeyStatus = "unknown"
	}
	if n.Slug == "" {
		n.Slug = n.Name
	}
	if n.Provider == "" {
		n.Provider = "sftp"
	}
	if n.RootPath == "" {
		n.RootPath = "/"
	}
	node, err := s.Store.CreateNode(ctx, n)
	if err != nil {
		return models.Node{}, err
	}
	s.notifyNodesChanged(ctx)
	return node, nil
}

func (s *Services) GetNode(ctx context.Context, id string) (models.Node, error) {
	return s.Store.GetNode(ctx, id)
}

func (s *Services) UpdateNode(ctx context.Context, id string, n models.Node) (models.Node, error) {
	existing, err := s.Store.GetNode(ctx, id)
	if err != nil {
		return models.Node{}, err
	}
	if n.Name != "" {
		existing.Name = n.Name
	}
	if n.Host != "" {
		existing.Host = n.Host
	}
	if n.Port != 0 {
		existing.Port = n.Port
	}
	if n.Username != "" {
		existing.Username = n.Username
	}
	if n.CredentialID != "" {
		existing.CredentialID = n.CredentialID
	}
	if n.KeyID != "" {
		existing.KeyID = n.KeyID
	}
	if n.Labels != nil {
		existing.Labels = n.Labels
	}
	existing.Enabled = n.Enabled
	node, err := s.Store.UpdateNode(ctx, id, existing)
	if err != nil {
		return models.Node{}, err
	}
	s.notifyNodesChanged(ctx)
	return node, nil
}

func (s *Services) DeleteNode(ctx context.Context, id string) error {
	if err := s.Store.DeleteNode(ctx, id); err != nil {
		return err
	}
	s.notifyNodesChanged(ctx)
	return nil
}

func (s *Services) PingNode(ctx context.Context, id string) (models.PingResult, error) {
	node, err := s.Store.GetNode(ctx, id)
	if err != nil {
		return models.PingResult{}, err
	}
	start := time.Now()
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", node.Host, node.Port), 5*time.Second)
	latency := time.Since(start).Milliseconds()
	result := models.PingResult{NodeID: id, LatencyMs: latency}
	if err != nil {
		result.Reachable = false
		result.Message = err.Error()
		_ = s.Store.UpdateNodePing(ctx, id, "unreachable")
		return result, nil
	}
	_ = conn.Close()
	result.Reachable = true
	result.Message = "ok"
	_ = s.Store.UpdateNodePing(ctx, id, "reachable")
	return result, nil
}

func (s *Services) TestNode(ctx context.Context, id string) (models.TestResult, error) {
	node, err := s.Store.GetNode(ctx, id)
	if err != nil {
		return models.TestResult{}, err
	}
	ping, err := s.PingNode(ctx, id)
	if err != nil {
		return models.TestResult{}, err
	}
	if !ping.Reachable {
		return models.TestResult{Success: false, Message: "host unreachable"}, nil
	}
	if node.CredentialID == "" && node.KeyID == "" {
		return models.TestResult{Success: true, Message: "connectivity ok (no credentials configured)"}, nil
	}
	return models.TestResult{Success: true, Message: fmt.Sprintf("connectivity ok to %s@%s:%d", node.Username, node.Host, node.Port)}, nil
}

func (s *Services) AcceptHostKey(ctx context.Context, id, fingerprint string) (models.Node, error) {
	node, err := s.Store.GetNode(ctx, id)
	if err != nil {
		return models.Node{}, err
	}
	node.HostKeyStatus = "accepted"
	node.HostKeyFingerprint = fingerprint
	return s.Store.UpdateNode(ctx, id, node)
}

func (s *Services) ListCredentials(ctx context.Context) ([]models.Credential, error) {
	return s.Store.ListCredentials(ctx)
}

func (s *Services) CreateCredential(ctx context.Context, name, credType, username, secret string) (models.Credential, error) {
	if name == "" || credType == "" || secret == "" {
		return models.Credential{}, ErrInvalidInput
	}
	return s.Store.CreateCredential(ctx, name, credType, username, secret)
}

func (s *Services) GetCredential(ctx context.Context, id string) (models.Credential, error) {
	c, _, err := s.Store.GetCredential(ctx, id)
	return c, err
}

func (s *Services) UpdateCredential(ctx context.Context, id, name, credType, username, secret string) (models.Credential, error) {
	existing, oldSecret, err := s.Store.GetCredential(ctx, id)
	if err != nil {
		return models.Credential{}, err
	}
	if name != "" {
		existing.Name = name
	}
	if credType != "" {
		existing.Type = credType
	}
	if username != "" {
		existing.Username = username
	}
	if secret == "" {
		secret = oldSecret
	}
	return s.Store.UpdateCredential(ctx, id, existing.Name, existing.Type, existing.Username, secret)
}

func (s *Services) DeleteCredential(ctx context.Context, id string) error {
	return s.Store.DeleteCredential(ctx, id)
}

func (s *Services) TestCredential(ctx context.Context, id string) (models.TestResult, error) {
	c, secret, err := s.Store.GetCredential(ctx, id)
	if err != nil {
		return models.TestResult{}, err
	}
	if secret == "" {
		return models.TestResult{Success: false, Message: "empty secret"}, nil
	}
	return models.TestResult{Success: true, Message: fmt.Sprintf("credential %s (%s) is valid", c.Name, c.Type)}, nil
}

func (s *Services) ListSSHKeys(ctx context.Context) ([]models.SSHKey, error) {
	return s.Store.ListSSHKeys(ctx)
}

func (s *Services) UploadSSHKey(ctx context.Context, name, privateKeyPEM, comment string) (models.SSHKey, error) {
	signer, err := ssh.ParsePrivateKey([]byte(privateKeyPEM))
	if err != nil {
		return models.SSHKey{}, fmt.Errorf("%w: invalid private key", ErrInvalidInput)
	}
	pub := ssh.MarshalAuthorizedKey(signer.PublicKey())
	fp := ssh.FingerprintSHA256(signer.PublicKey())
	return s.Store.CreateSSHKey(ctx, name, fp, string(pub), privateKeyPEM, comment)
}

func (s *Services) GenerateSSHKey(ctx context.Context, name, comment string) (models.SSHKey, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return models.SSHKey{}, err
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		return models.SSHKey{}, err
	}
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		return models.SSHKey{}, err
	}
	privPEM := string(pem.EncodeToMemory(block))
	pub := ssh.MarshalAuthorizedKey(signer.PublicKey())
	fp := ssh.FingerprintSHA256(signer.PublicKey())
	return s.Store.CreateSSHKey(ctx, name, fp, string(pub), privPEM, comment)
}

func (s *Services) GetSSHKeyPrivate(ctx context.Context, id string) (models.SSHKey, string, error) {
	return s.Store.GetSSHKey(ctx, id)
}

func (s *Services) DeleteSSHKey(ctx context.Context, id string) error {
	return s.Store.DeleteSSHKey(ctx, id)
}

func (s *Services) RotateSSHKey(ctx context.Context, id, name string) (models.SSHKey, error) {
	old, _, err := s.Store.GetSSHKey(ctx, id)
	if err != nil {
		return models.SSHKey{}, err
	}
	if name == "" {
		name = old.Name + "-rotated"
	}
	newKey, err := s.GenerateSSHKey(ctx, name, old.Comment)
	if err != nil {
		return models.SSHKey{}, err
	}
	_ = s.Store.DeleteSSHKey(ctx, id)
	return newKey, nil
}

func fingerprintPublicKey(pub string) (string, error) {
	pk, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pub))
	if err != nil {
		return "", fmt.Errorf("%w: invalid public key", ErrInvalidInput)
	}
	return ssh.FingerprintSHA256(pk), nil
}

func (s *Services) nodeVirtualPath(node models.Node, relPath string) string {
	slug := node.Slug
	if slug == "" {
		slug = node.Name
	}
	relPath = strings.TrimPrefix(relPath, "/")
	if relPath == "" {
		return "/" + slug
	}
	return "/" + slug + "/" + relPath
}

func (s *Services) ListDir(ctx context.Context, nodeID, path string) ([]models.FileEntry, error) {
	node, err := s.Store.GetNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	if s.VFS != nil {
		vpath := s.nodeVirtualPath(node, path)
		if path == "" || path == "/" {
			vpath = "/" + node.Slug
			if node.Slug == "" {
				vpath = "/" + node.Name
			}
		}
		infos, err := s.VFS.ReadDir(ctx, vpath)
		if err != nil {
			return nil, err
		}
		var out []models.FileEntry
		for _, info := range infos {
			p := info.Name
			if path != "" && path != "/" {
				p = strings.TrimPrefix(path, "/") + "/" + info.Name
			}
			out = append(out, models.FileEntry{
				Name: info.Name, Path: p, IsDir: info.IsDir, Size: info.Size,
				Mode: info.Mode.String(), ModTime: info.ModTime.UTC().Format(time.RFC3339),
			})
		}
		return out, nil
	}
	base := s.localNodePath(nodeID)
	full := filepath.Join(base, filepath.Clean("/"+path))
	entries, err := os.ReadDir(full)
	if err != nil {
		return nil, err
	}
	var out []models.FileEntry
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		p := filepath.Join(path, e.Name())
		out = append(out, models.FileEntry{
			Name: e.Name(), Path: p, IsDir: e.IsDir(), Size: info.Size(),
			Mode: info.Mode().String(), ModTime: info.ModTime().UTC().Format(time.RFC3339),
		})
	}
	return out, nil
}

func (s *Services) StatPath(ctx context.Context, nodeID, path string) (models.FileStat, error) {
	node, err := s.Store.GetNode(ctx, nodeID)
	if err != nil {
		return models.FileStat{}, err
	}
	if s.VFS != nil {
		vpath := s.nodeVirtualPath(node, path)
		info, err := s.VFS.Stat(ctx, vpath)
		if err != nil {
			return models.FileStat{}, err
		}
		return models.FileStat{
			Path: path, IsDir: info.IsDir, Size: info.Size,
			Mode: info.Mode.String(), ModTime: info.ModTime.UTC().Format(time.RFC3339),
		}, nil
	}
	full := filepath.Join(s.localNodePath(nodeID), filepath.Clean("/"+path))
	info, err := os.Stat(full)
	if err != nil {
		return models.FileStat{}, err
	}
	return models.FileStat{
		Path: path, IsDir: info.IsDir(), Size: info.Size(),
		Mode: info.Mode().String(), ModTime: info.ModTime().UTC().Format(time.RFC3339),
	}, nil
}

func (s *Services) DownloadFile(ctx context.Context, nodeID, path string) (io.ReadCloser, models.FileStat, error) {
	node, err := s.Store.GetNode(ctx, nodeID)
	if err != nil {
		return nil, models.FileStat{}, err
	}
	stat, err := s.StatPath(ctx, nodeID, path)
	if err != nil {
		return nil, models.FileStat{}, err
	}
	if stat.IsDir {
		return nil, models.FileStat{}, fmt.Errorf("%w: is a directory", ErrInvalidInput)
	}
	if s.VFS != nil {
		rc, err := s.VFS.Open(ctx, s.nodeVirtualPath(node, path))
		return rc, stat, err
	}
	full := filepath.Join(s.localNodePath(nodeID), filepath.Clean("/"+path))
	f, err := os.Open(full)
	return f, stat, err
}

func (s *Services) Mkdir(ctx context.Context, nodeID, path string) error {
	node, err := s.Store.GetNode(ctx, nodeID)
	if err != nil {
		return err
	}
	if s.VFS != nil {
		return s.VFS.Mkdir(ctx, s.nodeVirtualPath(node, path))
	}
	full := filepath.Join(s.localNodePath(nodeID), filepath.Clean("/"+path))
	return os.MkdirAll(full, 0o755)
}

func (s *Services) Rename(ctx context.Context, nodeID, from, to string) error {
	node, err := s.Store.GetNode(ctx, nodeID)
	if err != nil {
		return err
	}
	if s.VFS != nil {
		src := s.nodeVirtualPath(node, from)
		dst := s.nodeVirtualPath(node, to)
		return s.VFS.Rename(ctx, src, dst)
	}
	base := s.localNodePath(nodeID)
	src := filepath.Join(base, filepath.Clean("/"+from))
	dst := filepath.Join(base, filepath.Clean("/"+to))
	return os.Rename(src, dst)
}

func (s *Services) CopyMovePath(ctx context.Context, sourceNodeID, sourcePath, destNodeID, destPath, mode string) error {
	if sourceNodeID == "" || sourcePath == "" || destNodeID == "" || destPath == "" {
		return ErrInvalidInput
	}
	if mode != "copy" && mode != "move" {
		return ErrInvalidInput
	}
	if mode == "move" && sourceNodeID == destNodeID {
		return s.Rename(ctx, sourceNodeID, sourcePath, destPath)
	}
	srcNode, err := s.Store.GetNode(ctx, sourceNodeID)
	if err != nil {
		return err
	}
	dstNode, err := s.Store.GetNode(ctx, destNodeID)
	if err != nil {
		return err
	}
	if s.VFS != nil {
		src := s.nodeVirtualPath(srcNode, sourcePath)
		dst := s.nodeVirtualPath(dstNode, destPath)
		if err := s.copyVirtualPath(ctx, src, dst); err != nil {
			return err
		}
		if mode == "move" {
			return s.removeVirtualRecursive(ctx, src)
		}
		return nil
	}
	src := filepath.Join(s.localNodePath(sourceNodeID), filepath.Clean("/"+sourcePath))
	dst := filepath.Join(s.localNodePath(destNodeID), filepath.Clean("/"+destPath))
	if err := copyLocalPath(src, dst); err != nil {
		return err
	}
	if mode == "move" {
		return os.RemoveAll(src)
	}
	return nil
}

func (s *Services) copyVirtualPath(ctx context.Context, src, dst string) error {
	info, err := s.VFS.Stat(ctx, src)
	if err != nil {
		return err
	}
	if info.IsDir {
		if err := s.VFS.Mkdir(ctx, dst); err != nil {
			return err
		}
		entries, err := s.VFS.ReadDir(ctx, src)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := s.copyVirtualPath(ctx, entry.Path, filepath.Join(dst, entry.Name)); err != nil {
				return err
			}
		}
		return nil
	}
	rc, err := s.VFS.Open(ctx, src)
	if err != nil {
		return err
	}
	defer rc.Close()
	_, err = s.VFS.Write(ctx, dst, 0, rc)
	return err
}

func (s *Services) removeVirtualRecursive(ctx context.Context, path string) error {
	info, err := s.VFS.Stat(ctx, path)
	if err != nil {
		return err
	}
	if info.IsDir {
		entries, err := s.VFS.ReadDir(ctx, path)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := s.removeVirtualRecursive(ctx, entry.Path); err != nil {
				return err
			}
		}
	}
	return s.VFS.Remove(ctx, path)
}

func copyLocalPath(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if err := os.MkdirAll(dst, info.Mode()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := copyLocalPath(filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func (s *Services) DeletePath(ctx context.Context, nodeID, path string) error {
	node, err := s.Store.GetNode(ctx, nodeID)
	if err != nil {
		return err
	}
	if s.VFS != nil {
		return s.VFS.Remove(ctx, s.nodeVirtualPath(node, path))
	}
	full := filepath.Join(s.localNodePath(nodeID), filepath.Clean("/"+path))
	return os.RemoveAll(full)
}

func (s *Services) ReadText(ctx context.Context, nodeID, path string) (string, error) {
	if _, err := s.Store.GetNode(ctx, nodeID); err != nil {
		return "", err
	}
	rc, stat, err := s.DownloadFile(ctx, nodeID, path)
	if err != nil {
		return "", err
	}
	defer rc.Close()
	if stat.Size > 1<<20 {
		return "", fmt.Errorf("%w: file too large for text edit", ErrInvalidInput)
	}
	data, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *Services) WriteText(ctx context.Context, nodeID, path, content string) error {
	node, err := s.Store.GetNode(ctx, nodeID)
	if err != nil {
		return err
	}
	if s.VFS != nil {
		_, err := s.VFS.Write(ctx, s.nodeVirtualPath(node, path), 0, strings.NewReader(content))
		return err
	}
	full := filepath.Join(s.localNodePath(nodeID), filepath.Clean("/"+path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, []byte(content), 0o644)
}

func (s *Services) notifyNodesChanged(ctx context.Context) {
	if s.OnNodesChanged != nil {
		s.OnNodesChanged(ctx)
	}
}

func (s *Services) localNodePath(nodeID string) string {
	p := filepath.Join(s.DataDir, "nodes", nodeID)
	_ = os.MkdirAll(p, 0o755)
	return p
}

func (s *Services) ListTransfers(ctx context.Context) ([]models.Transfer, error) {
	return s.Store.ListTransfers(ctx)
}

func (s *Services) CreateTransfer(ctx context.Context, t models.Transfer) (models.Transfer, error) {
	if t.NodeID == "" || t.SourcePath == "" || t.DestPath == "" {
		return models.Transfer{}, ErrInvalidInput
	}
	if t.Direction == "" {
		t.Direction = "upload"
	}
	t.Status = models.TransferPending
	return s.Store.CreateTransfer(ctx, t)
}

func (s *Services) GetTransfer(ctx context.Context, id string) (models.Transfer, error) {
	return s.Store.GetTransfer(ctx, id)
}

func (s *Services) DeleteTransfer(ctx context.Context, id string) error {
	return s.Store.DeleteTransfer(ctx, id)
}

func (s *Services) PauseTransfer(ctx context.Context, id string) (models.Transfer, error) {
	return s.Store.UpdateTransferStatus(ctx, id, models.TransferPaused, "")
}

func (s *Services) ResumeTransfer(ctx context.Context, id string) (models.Transfer, error) {
	return s.Store.UpdateTransferStatus(ctx, id, models.TransferRunning, "")
}

func (s *Services) CancelTransfer(ctx context.Context, id string) (models.Transfer, error) {
	return s.Store.UpdateTransferStatus(ctx, id, models.TransferCancelled, "")
}

func (s *Services) RetryTransfer(ctx context.Context, id string) (models.Transfer, error) {
	if _, err := s.Store.GetTransfer(ctx, id); err != nil {
		return models.Transfer{}, err
	}
	return s.Store.UpdateTransferStatus(ctx, id, models.TransferPending, "")
}

func (s *Services) CreateUpload(ctx context.Context, nodeID, path string, size int64) (models.Upload, error) {
	return s.Store.CreateUpload(ctx, nodeID, path, size)
}

func (s *Services) GetUpload(ctx context.Context, id string) (models.Upload, error) {
	return s.Store.GetUpload(ctx, id)
}

func (s *Services) PatchUpload(ctx context.Context, id string, offset int64, data []byte) (models.Upload, error) {
	return s.Store.PatchUpload(ctx, id, offset, data)
}

func (s *Services) DeleteUpload(ctx context.Context, id string) error {
	return s.Store.DeleteUpload(ctx, id)
}

func (s *Services) GetSettings(ctx context.Context) (models.Settings, error) {
	return s.Store.GetSettings(ctx)
}

func (s *Services) UpdateSettings(ctx context.Context, patch models.Settings) (models.Settings, error) {
	current, err := s.Store.GetSettings(ctx)
	if err != nil {
		return models.Settings{}, err
	}
	if patch.SiteName != "" {
		current.SiteName = patch.SiteName
	}
	if patch.SessionTimeoutMin > 0 {
		current.SessionTimeoutMin = patch.SessionTimeoutMin
	}
	if patch.MaxUploadSizeMB > 0 {
		current.MaxUploadSizeMB = patch.MaxUploadSizeMB
	}
	if patch.RateLimitPerMinute > 0 {
		current.RateLimitPerMinute = patch.RateLimitPerMinute
	}
	current.RequireReauth = patch.RequireReauth
	current.AllowRegistration = patch.AllowRegistration
	if patch.DefaultNodePort > 0 {
		current.DefaultNodePort = patch.DefaultNodePort
	}
	if patch.BackupRetentionDays > 0 {
		current.BackupRetentionDays = patch.BackupRetentionDays
	}
	if patch.Protocols != (models.ProtocolSettings{}) {
		current.Protocols = patch.Protocols
	}
	if err := s.Store.UpdateSettings(ctx, current); err != nil {
		return models.Settings{}, err
	}
	if s.OnSettingsChanged != nil {
		if err := s.OnSettingsChanged(ctx, current); err != nil {
			return models.Settings{}, err
		}
	}
	return current, nil
}

func (s *Services) ExportConfig(ctx context.Context) (models.ConfigDocument, error) {
	return s.Store.ExportConfig(ctx)
}

func (s *Services) ApplyConfig(ctx context.Context, doc models.ConfigDocument) error {
	return s.Store.ReplaceAllConfig(ctx, doc)
}

func (s *Services) RestoreNodes(ctx context.Context, specs []models.NodeSpec) error {
	return s.restoreNodesBackup(ctx, specs)
}

func (s *Services) Dashboard(ctx context.Context) (models.Dashboard, error) {
	nodes, err := s.Store.ListNodes(ctx)
	if err != nil {
		return models.Dashboard{}, err
	}
	users, err := s.Store.ListUsers(ctx)
	if err != nil {
		return models.Dashboard{}, err
	}
	active, err := s.Store.CountActiveTransfers(ctx)
	if err != nil {
		return models.Dashboard{}, err
	}
	return models.Dashboard{
		NodeCount: len(nodes), TotalUsers: len(users), ActiveTransfers: active,
	}, nil
}

func (s *Services) Backup(ctx context.Context) (models.BackupResult, error) {
	if err := os.MkdirAll(s.BackupDir, 0o755); err != nil {
		return models.BackupResult{}, err
	}
	nodes, err := s.ListNodes(ctx)
	if err != nil {
		return models.BackupResult{}, err
	}
	creds, err := s.ListCredentials(ctx)
	if err != nil {
		return models.BackupResult{}, err
	}
	keys, err := s.ListSSHKeys(ctx)
	if err != nil {
		return models.BackupResult{}, err
	}
	settings, err := s.Store.GetSettings(ctx)
	if err != nil {
		return models.BackupResult{}, err
	}
	doc := nodesbackup.BuildDocument(nodes, creds, keys, settings.DefaultNodePort)
	data, err := nodesbackup.Marshal(doc)
	if err != nil {
		return models.BackupResult{}, err
	}
	name := fmt.Sprintf("backup-%s.yaml", time.Now().UTC().Format("20060102-150405"))
	path := filepath.Join(s.BackupDir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return models.BackupResult{}, err
	}
	info, _ := os.Stat(path)
	return models.BackupResult{Path: path, Size: info.Size(), CreatedAt: time.Now().UTC()}, nil
}

func (s *Services) Restore(ctx context.Context, data []byte) error {
	doc, err := nodesbackup.Parse(data)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	result := nodesbackup.Validate(doc)
	if !result.Valid {
		return fmt.Errorf("%w: %v", store.ErrConflict, result.Errors)
	}
	return s.restoreNodesBackup(ctx, doc.Nodes)
}

func (s *Services) restoreNodesBackup(ctx context.Context, specs []models.NodeSpec) error {
	creds, err := s.ListCredentials(ctx)
	if err != nil {
		return err
	}
	keys, err := s.ListSSHKeys(ctx)
	if err != nil {
		return err
	}
	credByName := make(map[string]string, len(creds))
	for _, c := range creds {
		credByName[c.Name] = c.ID
	}
	keyByName := make(map[string]string, len(keys))
	for _, k := range keys {
		keyByName[k.Name] = k.ID
	}

	settings, err := s.Store.GetSettings(ctx)
	if err != nil {
		return err
	}

	existing, err := s.ListNodes(ctx)
	if err != nil {
		return err
	}
	existingByName := make(map[string]models.Node, len(existing))
	for _, n := range existing {
		existingByName[n.Name] = n
	}

	seen := make(map[string]bool, len(specs))
	for _, spec := range specs {
		seen[spec.Name] = true

		port := spec.Port
		if port == 0 {
			port = settings.DefaultNodePort
		}
		enabled := true
		if spec.Enabled != nil {
			enabled = *spec.Enabled
		}

		credID := ""
		if spec.Credential != "" {
			id, ok := credByName[spec.Credential]
			if !ok {
				return fmt.Errorf("%w: credential not found: %s", ErrInvalidInput, spec.Credential)
			}
			credID = id
		}
		keyID := ""
		if spec.Key != "" {
			id, ok := keyByName[spec.Key]
			if !ok {
				return fmt.Errorf("%w: key not found: %s", ErrInvalidInput, spec.Key)
			}
			keyID = id
		}

		if current, ok := existingByName[spec.Name]; ok {
			if spec.Enabled == nil {
				enabled = current.Enabled
			}
			current.Host = spec.Host
			current.Port = port
			current.Username = spec.Username
			current.CredentialID = credID
			current.KeyID = keyID
			current.Labels = spec.Labels
			current.Enabled = enabled
			if _, err := s.Store.UpdateNode(ctx, current.ID, current); err != nil {
				return err
			}
			continue
		}

		if _, err := s.CreateNode(ctx, models.Node{
			Name:         spec.Name,
			Host:         spec.Host,
			Port:         port,
			Username:     spec.Username,
			CredentialID: credID,
			KeyID:        keyID,
			Labels:       spec.Labels,
			Enabled:      enabled,
		}); err != nil {
			return err
		}
	}

	for name, node := range existingByName {
		if seen[name] {
			continue
		}
		if err := s.DeleteNode(ctx, node.ID); err != nil {
			return err
		}
	}

	return nil
}

func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func GenerateToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}
