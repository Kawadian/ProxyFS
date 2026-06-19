package vfs

import (
	"context"
	"errors"
	"io/fs"
	"testing"
)

func TestStatRootWithoutMounts(t *testing.T) {
	v := New("/", 0, 0)
	info, err := v.Stat(context.Background(), "/")
	if err != nil {
		t.Fatalf("Stat(/): %v", err)
	}
	if !info.IsDir {
		t.Fatal("root should be a directory")
	}
	if info.Path != "/" {
		t.Fatalf("path: %q", info.Path)
	}
}

func TestReadDirRootWithoutMounts(t *testing.T) {
	v := New("/", 0, 0)
	entries, err := v.ReadDir(context.Background(), "/")
	if err != nil {
		t.Fatalf("ReadDir(/): %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty root, got %d entries", len(entries))
	}
}

func TestStatMissingPath(t *testing.T) {
	v := New("/", 0, 0)
	_, err := v.Stat(context.Background(), "/missing")
	if err == nil {
		t.Fatal("expected error for missing path")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected ErrNotExist, got %v", err)
	}
}
