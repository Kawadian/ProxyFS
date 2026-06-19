package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/lxcfh/lxcfh/internal/crypto"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrInvalidCreds = errors.New("invalid credentials")
)

// User represents an authenticated principal.
type User struct {
	ID          string
	Username    string
	DisplayName string
	Role        Role
	Enabled     bool
}

// Session holds an active login session.
type Session struct {
	ID        string
	UserID    string
	Token     string
	IPAddress string
	UserAgent string
	ExpiresAt time.Time
}

// Service manages authentication, sessions, and guest IP access.
type Service struct {
	db         *sql.DB
	sessionTTL time.Duration
	mu         sync.RWMutex
	guestCIDRs []guestEntry
}

type guestEntry struct {
	cidr  *net.IPNet
	label string
}

// NewService creates an auth service backed by the database.
func NewService(db *sql.DB, sessionTTL time.Duration) *Service {
	return &Service{
		db:         db,
		sessionTTL: sessionTTL,
	}
}

// LoadGuestIPs refreshes the in-memory guest IP allowlist from the database.
func (s *Service) LoadGuestIPs(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT cidr, label FROM guest_ips WHERE enabled = 1`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var entries []guestEntry
	for rows.Next() {
		var cidrStr, label string
		if err := rows.Scan(&cidrStr, &label); err != nil {
			return err
		}
		_, network, err := net.ParseCIDR(cidrStr)
		if err != nil {
			ip := net.ParseIP(cidrStr)
			if ip == nil {
				continue
			}
			if ip.To4() != nil {
				_, network, _ = net.ParseCIDR(cidrStr + "/32")
			} else {
				_, network, _ = net.ParseCIDR(cidrStr + "/128")
			}
		}
		entries = append(entries, guestEntry{cidr: network, label: label})
	}
	s.mu.Lock()
	s.guestCIDRs = entries
	s.mu.Unlock()
	return rows.Err()
}

// AuthenticatePassword validates username/password and returns the user.
func (s *Service) AuthenticatePassword(ctx context.Context, username, password string) (*User, error) {
	var u User
	var hash string
	var enabled int
	var role string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, display_name, role, enabled, password_hash FROM users WHERE username = ? COLLATE NOCASE`,
		username,
	).Scan(&u.ID, &u.Username, &u.DisplayName, &role, &enabled, &hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInvalidCreds
		}
		return nil, err
	}
	if enabled == 0 {
		return nil, ErrInvalidCreds
	}
	ok, err := crypto.VerifyPassword(password, hash)
	if err != nil || !ok {
		return nil, ErrInvalidCreds
	}
	u.Enabled = enabled == 1
	u.Role = Role(role)
	return &u, nil
}

// CreateSession issues a new session token for a user.
func (s *Service) CreateSession(ctx context.Context, userID, ip, userAgent string) (*Session, error) {
	token, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	id, err := randomToken(16)
	if err != nil {
		return nil, err
	}
	expires := time.Now().Add(s.sessionTTL)
	hash := hashToken(token)
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO sessions (id, user_id, token_hash, ip_address, user_agent, expires_at) VALUES (?, ?, ?, ?, ?, ?)`,
		id, userID, hash, ip, userAgent, expires.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	return &Session{
		ID:        id,
		UserID:    userID,
		Token:     token,
		IPAddress: ip,
		UserAgent: userAgent,
		ExpiresAt: expires,
	}, nil
}

// ValidateSession looks up a bearer token and returns the associated user.
func (s *Service) ValidateSession(ctx context.Context, token string) (*User, error) {
	hash := hashToken(token)
	var userID string
	var expiresStr string
	err := s.db.QueryRowContext(ctx,
		`SELECT user_id, expires_at FROM sessions WHERE token_hash = ?`,
		hash,
	).Scan(&userID, &expiresStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}
	expires, err := time.Parse(time.RFC3339, expiresStr)
	if err != nil {
		return nil, err
	}
	if time.Now().After(expires) {
		_, _ = s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, hash)
		return nil, ErrUnauthorized
	}
	return s.GetUser(ctx, userID)
}

// AuthenticateGuest checks whether an IP address is allowed guest access.
func (s *Service) AuthenticateGuest(ipStr string) (*User, error) {
	ip := net.ParseIP(strings.TrimSpace(ipStr))
	if ip == nil {
		return nil, ErrUnauthorized
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, entry := range s.guestCIDRs {
		if entry.cidr.Contains(ip) {
			return &User{
				ID:       "guest:" + entry.cidr.String(),
				Username: "guest",
				Role:     RoleGuest,
				Enabled:  true,
			}, nil
		}
	}
	return nil, ErrUnauthorized
}

// AuthenticateRequest resolves a user from session token or guest IP.
func (s *Service) AuthenticateRequest(ctx context.Context, token, ip string) (*User, error) {
	if token != "" {
		return s.ValidateSession(ctx, token)
	}
	return s.AuthenticateGuest(ip)
}

// GetUser loads a user by ID.
func (s *Service) GetUser(ctx context.Context, id string) (*User, error) {
	if strings.HasPrefix(id, "guest:") {
		return &User{ID: id, Username: "guest", Role: RoleGuest, Enabled: true}, nil
	}
	var u User
	var enabled int
	var role string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, display_name, role, enabled FROM users WHERE id = ?`,
		id,
	).Scan(&u.ID, &u.Username, &u.DisplayName, &role, &enabled)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}
	u.Role = Role(role)
	u.Enabled = enabled == 1
	return &u, nil
}

// RevokeSession invalidates a session token.
func (s *Service) RevokeSession(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, hashToken(token))
	return err
}

// PurgeExpiredSessions removes expired sessions from the database.
func (s *Service) PurgeExpiredSessions(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < datetime('now')`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// CreateUser inserts a new user with a hashed password.
func (s *Service) CreateUser(ctx context.Context, username, password, displayName string, role Role) (*User, error) {
	hash, err := crypto.HashPassword(password)
	if err != nil {
		return nil, err
	}
	id, err := randomToken(16)
	if err != nil {
		return nil, err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO users (id, username, password_hash, display_name, role) VALUES (?, ?, ?, ?, ?)`,
		id, username, hash, displayName, string(role),
	)
	if err != nil {
		return nil, err
	}
	return &User{ID: id, Username: username, DisplayName: displayName, Role: role, Enabled: true}, nil
}

// Authorize checks RBAC permission for a user on a resource action.
func (s *Service) Authorize(user *User, action Action, resource Resource) error {
	if user == nil {
		return ErrUnauthorized
	}
	if !user.Enabled {
		return ErrForbidden
	}
	if HasPermission(user.Role, action) {
		return nil
	}
	return fmt.Errorf("%w: role %s cannot %s on %s", ErrForbidden, user.Role, action, resource)
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
