package webdav

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// LockDepth indicates whether a lock applies to a resource tree.
type LockDepth int

const (
	DepthZero LockDepth = 0
	DepthInf  LockDepth = -1
)

// Lock represents an active WebDAV lock.
type Lock struct {
	Token       string
	Path        string
	Owner       string
	Depth       LockDepth
	TimeoutSecs int
	Exclusive   bool
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

// LockStore persists WebDAV locks in the database.
type LockStore struct {
	db *sql.DB
}

// NewLockStore creates a lock store backed by SQLite.
func NewLockStore(db *sql.DB) *LockStore {
	return &LockStore{db: db}
}

// Create acquires a new lock on path.
func (s *LockStore) Create(ctx context.Context, path, owner string, depth LockDepth, timeoutSecs int, exclusive bool) (*Lock, error) {
	path = cleanLockPath(path)
	if path == "" {
		return nil, errors.New("webdav: empty lock path")
	}
	if timeoutSecs <= 0 {
		timeoutSecs = 3600
	}
	if err := s.purgeExpired(ctx); err != nil {
		return nil, err
	}
	if err := s.checkConflict(ctx, path, depth); err != nil {
		return nil, err
	}
	token := "opaquelocktoken:" + uuid.NewString()
	now := time.Now().UTC()
	expires := now.Add(time.Duration(timeoutSecs) * time.Second)
	excl := 0
	if exclusive {
		excl = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO webdav_locks (token, path, owner, depth, timeout_secs, exclusive, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		token, path, owner, int(depth), timeoutSecs, excl,
		now.Format(time.RFC3339), expires.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("webdav: create lock: %w", err)
	}
	return &Lock{
		Token:       token,
		Path:        path,
		Owner:       owner,
		Depth:       depth,
		TimeoutSecs: timeoutSecs,
		Exclusive:   exclusive,
		CreatedAt:   now,
		ExpiresAt:   expires,
	}, nil
}

// Refresh extends an existing lock timeout.
func (s *LockStore) Refresh(ctx context.Context, token string, timeoutSecs int) (*Lock, error) {
	if timeoutSecs <= 0 {
		timeoutSecs = 3600
	}
	lock, err := s.Get(ctx, token)
	if err != nil {
		return nil, err
	}
	expires := time.Now().UTC().Add(time.Duration(timeoutSecs) * time.Second)
	res, err := s.db.ExecContext(ctx,
		`UPDATE webdav_locks SET timeout_secs = ?, expires_at = ? WHERE token = ?`,
		timeoutSecs, expires.Format(time.RFC3339), token,
	)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, ErrLockNotFound
	}
	lock.TimeoutSecs = timeoutSecs
	lock.ExpiresAt = expires
	return lock, nil
}

// Release removes a lock by token.
func (s *LockStore) Release(ctx context.Context, token string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM webdav_locks WHERE token = ?`, token)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrLockNotFound
	}
	return nil
}

// Get returns a lock by token.
func (s *LockStore) Get(ctx context.Context, token string) (*Lock, error) {
	var lock Lock
	var depth, timeout, exclusive int
	var created, expires string
	err := s.db.QueryRowContext(ctx, `
		SELECT token, path, owner, depth, timeout_secs, exclusive, created_at, expires_at
		FROM webdav_locks WHERE token = ?`, token).Scan(
		&lock.Token, &lock.Path, &lock.Owner, &depth, &timeout, &exclusive, &created, &expires,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrLockNotFound
		}
		return nil, err
	}
	lock.Depth = LockDepth(depth)
	lock.TimeoutSecs = timeout
	lock.Exclusive = exclusive == 1
	lock.CreatedAt, _ = time.Parse(time.RFC3339, created)
	lock.ExpiresAt, _ = time.Parse(time.RFC3339, expires)
	if time.Now().After(lock.ExpiresAt) {
		_, _ = s.db.ExecContext(ctx, `DELETE FROM webdav_locks WHERE token = ?`, token)
		return nil, ErrLockNotFound
	}
	return &lock, nil
}

// IsLocked reports whether path is locked by another holder.
func (s *LockStore) IsLocked(ctx context.Context, path string) (bool, *Lock, error) {
	if err := s.purgeExpired(ctx); err != nil {
		return false, nil, err
	}
	path = cleanLockPath(path)
	rows, err := s.db.QueryContext(ctx, `
		SELECT token, path, owner, depth, timeout_secs, exclusive, created_at, expires_at
		FROM webdav_locks`)
	if err != nil {
		return false, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		lock, err := scanLock(rows)
		if err != nil {
			return false, nil, err
		}
		if lockApplies(lock, path) {
			return true, lock, nil
		}
	}
	return false, nil, rows.Err()
}

// AssertUnlock verifies the If header token unlocks path for mutation.
func (s *LockStore) AssertUnlock(ctx context.Context, path, token string) error {
	if token == "" {
		locked, lock, err := s.IsLocked(ctx, path)
		if err != nil {
			return err
		}
		if locked {
			return fmt.Errorf("%w: %s held by %s", ErrLocked, lock.Path, lock.Owner)
		}
		return nil
	}
	lock, err := s.Get(ctx, token)
	if err != nil {
		return err
	}
	if !lockApplies(lock, cleanLockPath(path)) && lock.Path != cleanLockPath(path) {
		return ErrLockTokenMismatch
	}
	return nil
}

func (s *LockStore) checkConflict(ctx context.Context, path string, depth LockDepth) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT token, path, owner, depth, timeout_secs, exclusive, created_at, expires_at
		FROM webdav_locks`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		existing, err := scanLock(rows)
		if err != nil {
			return err
		}
		if pathsConflict(existing, path, depth) || pathsConflict(&Lock{Path: path, Depth: depth}, existing.Path, existing.Depth) {
			return fmt.Errorf("%w: %s", ErrLocked, existing.Path)
		}
	}
	return rows.Err()
}

func (s *LockStore) purgeExpired(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM webdav_locks WHERE expires_at < datetime('now')`)
	return err
}

func scanLock(scanner interface {
	Scan(dest ...any) error
}) (*Lock, error) {
	var lock Lock
	var depth, timeout, exclusive int
	var created, expires string
	if err := scanner.Scan(
		&lock.Token, &lock.Path, &lock.Owner, &depth, &timeout, &exclusive, &created, &expires,
	); err != nil {
		return nil, err
	}
	lock.Depth = LockDepth(depth)
	lock.TimeoutSecs = timeout
	lock.Exclusive = exclusive == 1
	lock.CreatedAt, _ = time.Parse(time.RFC3339, created)
	lock.ExpiresAt, _ = time.Parse(time.RFC3339, expires)
	return &lock, nil
}

func cleanLockPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	if len(p) > 1 && strings.HasSuffix(p, "/") {
		p = strings.TrimSuffix(p, "/")
	}
	return p
}

func lockApplies(lock *Lock, path string) bool {
	path = cleanLockPath(path)
	lockPath := cleanLockPath(lock.Path)
	if path == lockPath {
		return true
	}
	if lock.Depth == DepthInf && strings.HasPrefix(path, lockPath+"/") {
		return true
	}
	return false
}

func pathsConflict(existing *Lock, newPath string, newDepth LockDepth) bool {
	if lockApplies(existing, newPath) {
		return true
	}
	if newDepth == DepthInf && strings.HasPrefix(existing.Path, cleanLockPath(newPath)+"/") {
		return true
	}
	return false
}

var (
	ErrLockNotFound      = errors.New("webdav: lock not found")
	ErrLocked            = errors.New("webdav: resource locked")
	ErrLockTokenMismatch = errors.New("webdav: lock token mismatch")
)
