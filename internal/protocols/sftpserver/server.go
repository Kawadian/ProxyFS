package sftpserver

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"

	"github.com/gliderlabs/ssh"
	"github.com/lxcfh/lxcfh/internal/auth"
	"github.com/lxcfh/lxcfh/internal/rbac"
	"github.com/lxcfh/lxcfh/internal/vfs"
	"github.com/pkg/sftp"
	gossh "golang.org/x/crypto/ssh"
)

const DefaultPort = 2022

// Config configures the SFTP SSH server.
type Config struct {
	Addr       string
	HostKeyPEM []byte
	AllowGuest bool
}

// Server exposes VirtualFS over SFTP on an SSH listener.
type Server struct {
	cfg     Config
	auth    *auth.Service
	fs      *vfs.VirtualFS
	rbac    *rbac.Engine
	db      *sql.DB
	logger  *slog.Logger
	sshSrv  *ssh.Server
	mu      sync.Mutex
	running bool
}

// New creates an SFTP server.
func New(cfg Config, authSvc *auth.Service, fs *vfs.VirtualFS, checker *rbac.Engine, db *sql.DB, logger *slog.Logger) (*Server, error) {
	if cfg.Addr == "" {
		cfg.Addr = fmt.Sprintf(":%d", DefaultPort)
	}
	if len(cfg.HostKeyPEM) == 0 {
		key, err := generateHostKey()
		if err != nil {
			return nil, fmt.Errorf("sftp host key: %w", err)
		}
		cfg.HostKeyPEM = key
	}
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{
		cfg:    cfg,
		auth:   authSvc,
		fs:     fs,
		rbac:   checker,
		db:     db,
		logger: logger,
	}
	s.sshSrv = &ssh.Server{
		Addr:             cfg.Addr,
		Handler:          s.sessionHandler,
		PasswordHandler:  s.passwordHandler,
		PublicKeyHandler: s.publicKeyHandler,
	}
	signer, err := gossh.ParsePrivateKey(cfg.HostKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("sftp parse host key: %w", err)
	}
	s.sshSrv.AddHostKey(signer)
	return s, nil
}

// Start listens for SSH connections.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return errors.New("sftp: already running")
	}
	ln, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return fmt.Errorf("sftp listen %s: %w", s.cfg.Addr, err)
	}
	s.running = true
	s.logger.Info("sftp server listening", "addr", s.cfg.Addr)
	go func() {
		if err := s.sshSrv.Serve(ln); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			s.logger.Error("sftp serve", "error", err)
		}
	}()
	return nil
}

// Stop shuts down the SSH server.
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return nil
	}
	s.running = false
	return s.sshSrv.Close()
}

// IsRunning reports whether the SFTP listener is active.
func (s *Server) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *Server) sessionHandler(sess ssh.Session) {
	user := sess.Context().Value(ctxUserKey{}).(*auth.User)
	if user == nil {
		_ = sess.Exit(1)
		return
	}
	if !strings.EqualFold(sess.Subsystem(), "sftp") {
		_, _ = io.WriteString(sess.Stderr(), "only sftp subsystem supported\n")
		_ = sess.Exit(1)
		return
	}
	handler := NewHandler(sess.Context(), s.fs, s.rbac, user)
	server := sftp.NewRequestServer(sess, handler.Handlers())
	if err := server.Serve(); err != nil && !errors.Is(err, io.EOF) {
		s.logger.Warn("sftp session", "user", user.Username, "error", err)
	}
	_ = server.Close()
}

type ctxUserKey struct{}

func (s *Server) passwordHandler(ctx ssh.Context, password string) bool {
	remoteIP := clientIP(ctx)
	user, err := s.authenticatePassword(ctx, ctx.User(), password, remoteIP)
	if err != nil {
		s.logger.Debug("sftp password auth failed", "user", ctx.User(), "ip", remoteIP, "error", err)
		return false
	}
	ctx.SetValue(ctxUserKey{}, user)
	return true
}

func (s *Server) publicKeyHandler(ctx ssh.Context, key ssh.PublicKey) bool {
	username := ctx.User()
	marshaled := strings.TrimSpace(string(gossh.MarshalAuthorizedKey(key)))
	user, err := s.authenticatePublicKey(ctx, username, marshaled)
	if err != nil {
		s.logger.Debug("sftp pubkey auth failed", "user", username, "error", err)
		return false
	}
	ctx.SetValue(ctxUserKey{}, user)
	return true
}

func (s *Server) authenticatePassword(ctx context.Context, username, password, remoteIP string) (*auth.User, error) {
	if strings.EqualFold(username, "guest") {
		if !s.cfg.AllowGuest {
			return nil, auth.ErrUnauthorized
		}
		guest, err := s.auth.AuthenticateGuest(remoteIP)
		if err != nil {
			return nil, err
		}
		return guest, nil
	}
	return s.auth.AuthenticatePassword(ctx, username, password)
}

func (s *Server) authenticatePublicKey(ctx context.Context, username, marshaledKey string) (*auth.User, error) {
	var userID string
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM users WHERE username = ? COLLATE NOCASE AND enabled = 1`,
		username,
	).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, auth.ErrInvalidCreds
		}
		return nil, err
	}
	var count int
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM user_ssh_keys WHERE user_id = ? AND public_key = ?`,
		userID, marshaledKey,
	).Scan(&count)
	if err != nil {
		return nil, err
	}
	if count == 0 {
		// also match trimmed lines stored with comments
		err = s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM user_ssh_keys WHERE user_id = ? AND TRIM(public_key) = ?`,
			userID, marshaledKey,
		).Scan(&count)
		if err != nil || count == 0 {
			return nil, auth.ErrInvalidCreds
		}
	}
	return s.auth.GetUser(ctx, userID)
}

func clientIP(ctx ssh.Context) string {
	if ctx.RemoteAddr() != nil {
		host, _, err := net.SplitHostPort(ctx.RemoteAddr().String())
		if err == nil {
			return host
		}
		return ctx.RemoteAddr().String()
	}
	return ""
}

func generateHostKey() ([]byte, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}), nil
}

// SetContextUser attaches an authenticated user to a context (for tests).
func SetContextUser(ctx context.Context, user *auth.User) context.Context {
	return context.WithValue(ctx, ctxUserKey{}, user)
}

// GenerateHostKey creates a new RSA host key PEM block.
func GenerateHostKey() ([]byte, error) {
	return generateHostKey()
}

// ErrNoSubsystem is returned when the client does not request sftp.
var ErrNoSubsystem = errors.New("sftp: subsystem not requested")
