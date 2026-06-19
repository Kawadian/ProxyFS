package vfs

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestStatVirtualRoot(t *testing.T) {
	fs := New("/", time.Minute, time.Minute)

	info, err := fs.Stat(context.Background(), "/")
	if err != nil {
		t.Fatalf("Stat(/): %v", err)
	}
	if !info.IsDir {
		t.Fatal("virtual root must be a directory")
	}
	if info.Mode != os.ModeDir|0o755 {
		t.Fatalf("root mode = %v, want %v", info.Mode, os.ModeDir|0o755)
	}
}

func TestMountChangesInvalidateRootDirectoryCache(t *testing.T) {
	fs := New("/", time.Hour, time.Hour)
	backend := &LocalBackend{Root: t.TempDir()}

	if err := fs.AddMount(Mount{Name: "one", Prefix: "/one", Backend: backend}); err != nil {
		t.Fatal(err)
	}
	entries, err := fs.ReadDir(context.Background(), "/")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name != "one" {
		t.Fatalf("initial root entries = %#v", entries)
	}

	if err := fs.AddMount(Mount{Name: "two", Prefix: "/two", Backend: backend}); err != nil {
		t.Fatal(err)
	}
	entries, err = fs.ReadDir(context.Background(), "/")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("root cache was not invalidated after add: %#v", entries)
	}

	if err := fs.RemoveMount("/one"); err != nil {
		t.Fatal(err)
	}
	entries, err = fs.ReadDir(context.Background(), "/")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name != "two" {
		t.Fatalf("root cache was not invalidated after remove: %#v", entries)
	}
}
