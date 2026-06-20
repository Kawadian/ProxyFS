package fuse

import (
	"context"
	"testing"

	gofuse "github.com/hanwen/go-fuse/v2/fuse"
	"github.com/lxcfh/lxcfh/internal/vfs"
)

type testUIDMapper struct{}

func (testUIDMapper) Lookup(_ context.Context, _ string) (uint32, uint32, bool) {
	return 0, 0, false
}

func (testUIDMapper) Default() (uint32, uint32) {
	return 0, 0
}

func TestFillAttrSuppliesUsableDefaultPermissions(t *testing.T) {
	var dir gofuse.Attr
	fillAttr(&dir, vfs.FileInfo{IsDir: true}, testUIDMapper{})
	if got := dir.Mode & 0o777; got != 0o755 {
		t.Fatalf("directory permissions: got %#o want 0755", got)
	}
	if dir.Ino == 0 || dir.Nlink == 0 {
		t.Fatalf("directory identity is incomplete: inode=%d nlink=%d", dir.Ino, dir.Nlink)
	}

	var file gofuse.Attr
	fillAttr(&file, vfs.FileInfo{}, testUIDMapper{})
	if got := file.Mode & 0o777; got != 0o644 {
		t.Fatalf("file permissions: got %#o want 0644", got)
	}
}
