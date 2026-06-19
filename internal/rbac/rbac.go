package rbac

import (
	"context"
	"errors"
	"path"
	"strings"
	"sync"

	"github.com/lxcfh/lxcfh/internal/hub"
)

// Permission is a filesystem capability checked before protocol operations.
type Permission string

const (
	Read    Permission = "read"
	Write   Permission = "write"
	Delete  Permission = "delete"
	Execute Permission = "execute"
	List    Permission = "list"
)

// Checker authorizes Hub users for VFS paths.
type Checker interface {
	Check(ctx context.Context, user *hub.User, vfsPath string, perm Permission) error
}

// Rule binds a path prefix to allowed roles and permissions.
type Rule struct {
	PathPrefix  string
	Roles       []string
	Permissions []Permission
}

// Engine evaluates path-prefix rules for Hub roles.
type Engine struct {
	mu    sync.RWMutex
	rules []Rule
}

// NewEngine returns an RBAC engine with default permissive rules for admins.
func NewEngine() *Engine {
	return &Engine{
		rules: []Rule{
			{PathPrefix: "/", Roles: []string{"admin"}, Permissions: []Permission{Read, Write, Delete, Execute, List}},
			{PathPrefix: "/", Roles: []string{"user"}, Permissions: []Permission{Read, List}},
			{PathPrefix: "/shared", Roles: []string{"user"}, Permissions: []Permission{Read, Write, List}},
			{PathPrefix: "/", Roles: []string{"guest"}, Permissions: []Permission{Read, List}},
		},
	}
}

// SetRules replaces the active rule set.
func (e *Engine) SetRules(rules []Rule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules = append([]Rule(nil), rules...)
}

// Check implements Checker.
func (e *Engine) Check(ctx context.Context, user *hub.User, vfsPath string, perm Permission) error {
	if user == nil {
		return errors.New("rbac: anonymous user denied")
	}
	clean := cleanPath(vfsPath)
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, rule := range e.rules {
		prefix := cleanPath(rule.PathPrefix)
		if !strings.HasPrefix(clean, prefix) {
			continue
		}
		if !roleMatch(user.Roles, rule.Roles) {
			continue
		}
		for _, p := range rule.Permissions {
			if p == perm {
				return nil
			}
		}
	}
	return errors.New("rbac: permission denied")
}

func cleanPath(p string) string {
	if p == "" {
		return "/"
	}
	c := path.Clean("/" + strings.TrimPrefix(p, "/"))
	if c == "." {
		return "/"
	}
	return c
}

func roleMatch(userRoles, ruleRoles []string) bool {
	if len(ruleRoles) == 0 {
		return true
	}
	roleSet := make(map[string]struct{}, len(userRoles))
	for _, r := range userRoles {
		roleSet[r] = struct{}{}
	}
	for _, r := range ruleRoles {
		if _, ok := roleSet[r]; ok {
			return true
		}
	}
	return false
}
