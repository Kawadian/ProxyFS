package vfs

import (
	"errors"
	"fmt"
	"path"
	"strings"
)

var (
	ErrInvalidPath   = errors.New("invalid path")
	ErrTraversal     = errors.New("path traversal detected")
	ErrAbsolutePath  = errors.New("absolute paths not allowed in node context")
)

// CleanVirtualPath normalizes a virtual filesystem path to POSIX style.
func CleanVirtualPath(p string) string {
	if p == "" {
		return "/"
	}
	p = strings.ReplaceAll(p, "\\", "/")
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	clean := path.Clean(p)
	if clean == "." {
		return "/"
	}
	return clean
}

// ResolveVirtualPath validates and cleans a user-supplied virtual path.
func ResolveVirtualPath(p string) (string, error) {
	if p == "" {
		return "/", nil
	}
	if strings.Contains(p, "\x00") {
		return "", ErrInvalidPath
	}
	normalized := strings.ReplaceAll(p, "\\", "/")
	if containsDotDot(normalized) {
		return "", ErrTraversal
	}
	clean := CleanVirtualPath(p)
	return clean, nil
}

// ResolveNodePath maps a relative path within a mount to a node-local path.
func ResolveNodePath(_ NodeBackend, rel string) (string, error) {
	if rel == "" {
		rel = "/"
	}
	normalized := strings.ReplaceAll(rel, "\\", "/")
	if containsDotDot(normalized) {
		return "", ErrTraversal
	}
	return CleanVirtualPath(rel), nil
}

// JoinVirtual joins path elements under a virtual root.
func JoinVirtual(base string, elems ...string) (string, error) {
	for _, elem := range elems {
		if containsDotDot(elem) {
			return "", ErrTraversal
		}
	}
	combined := path.Join(append([]string{CleanVirtualPath(base)}, elems...)...)
	return ResolveVirtualPath(combined)
}

// RelVirtual returns the path of target relative to base within the virtual tree.
func RelVirtual(base, target string) (string, error) {
	b, err := ResolveVirtualPath(base)
	if err != nil {
		return "", err
	}
	t, err := ResolveVirtualPath(target)
	if err != nil {
		return "", err
	}
	if b == "/" {
		return t, nil
	}
	if !strings.HasPrefix(t, b+"/") && t != b {
		return "", fmt.Errorf("target %q is not under base %q", target, base)
	}
	rel := strings.TrimPrefix(t, b)
	if rel == "" {
		return "/", nil
	}
	return rel, nil
}

func containsDotDot(p string) bool {
	parts := strings.Split(p, "/")
	for _, part := range parts {
		if part == ".." {
			return true
		}
	}
	return false
}

// ValidateMountPrefix ensures a mount prefix is safe to register.
func ValidateMountPrefix(prefix string) error {
	normalized := strings.ReplaceAll(prefix, "\\", "/")
	if containsDotDot(normalized) {
		return ErrTraversal
	}
	clean := CleanVirtualPath(prefix)
	if clean == "/" {
		return errors.New("mount prefix cannot be root")
	}
	if strings.HasSuffix(clean, "/") {
		return errors.New("mount prefix must not end with slash")
	}
	return nil
}
