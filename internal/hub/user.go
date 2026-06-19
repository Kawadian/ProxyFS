package hub

import (
	"context"
	"sync"
)

// User represents a Hub identity with POSIX mapping for FUSE and Samba.
type User struct {
	ID       string
	Username string
	Password string // bcrypt hash; empty for pubkey-only or guest
	PubKeys  []string
	UID      uint32
	GID      uint32
	Roles    []string
	IsGuest  bool
}

// UserStore provides Hub user lookup.
type UserStore interface {
	GetByUsername(ctx context.Context, username string) (*User, error)
	GetByID(ctx context.Context, id string) (*User, error)
	List(ctx context.Context) ([]*User, error)
}

// MemoryStore is an in-memory UserStore for development and tests.
type MemoryStore struct {
	mu    sync.RWMutex
	users map[string]*User
	byID  map[string]*User
}

// NewMemoryStore returns an empty in-memory user store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		users: make(map[string]*User),
		byID:  make(map[string]*User),
	}
}

// Add registers a user in the memory store.
func (s *MemoryStore) Add(u *User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[u.Username] = u
	s.byID[u.ID] = u
}

// GetByUsername implements UserStore.
func (s *MemoryStore) GetByUsername(ctx context.Context, username string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[username]
	if !ok {
		return nil, ErrUserNotFound
	}
	return u, nil
}

// GetByID implements UserStore.
func (s *MemoryStore) GetByID(ctx context.Context, id string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.byID[id]
	if !ok {
		return nil, ErrUserNotFound
	}
	return u, nil
}

// List implements UserStore.
func (s *MemoryStore) List(ctx context.Context) ([]*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*User, 0, len(s.users))
	for _, u := range s.users {
		out = append(out, u)
	}
	return out, nil
}
