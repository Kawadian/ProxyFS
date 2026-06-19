package vfs

import (
	"testing"
)

func TestResolveVirtualPath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{"empty", "", "/", nil},
		{"root", "/", "/", nil},
		{"simple", "/foo/bar", "/foo/bar", nil},
		{"dot segments", "/foo/./bar", "/foo/bar", nil},
		{"traversal", "/foo/../etc/passwd", "", ErrTraversal},
		{"encoded traversal", "/foo/bar/../../../secret", "", ErrTraversal},
		{"backslash", `\foo\bar`, "/foo/bar", nil},
		{"null byte", "/foo\x00/bar", "", ErrInvalidPath},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveVirtualPath(tt.input)
			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Fatalf("expected %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestJoinVirtual(t *testing.T) {
	got, err := JoinVirtual("/mount", "subdir", "file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/mount/subdir/file.txt" {
		t.Fatalf("got %q", got)
	}
	_, err = JoinVirtual("/mount", "..", "secret")
	if err != ErrTraversal {
		t.Fatalf("expected traversal error, got %v", err)
	}
}

func TestValidateMountPrefix(t *testing.T) {
	if err := ValidateMountPrefix("/node-a"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMountPrefix("/"); err == nil {
		t.Fatal("expected error for root mount")
	}
	if err := ValidateMountPrefix("/node/../escape"); err != ErrTraversal {
		t.Fatalf("expected traversal error, got %v", err)
	}
}

func TestRelVirtual(t *testing.T) {
	rel, err := RelVirtual("/mount", "/mount/foo/bar")
	if err != nil {
		t.Fatal(err)
	}
	if rel != "/foo/bar" {
		t.Fatalf("got %q", rel)
	}
	_, err = RelVirtual("/mount", "/other/file")
	if err == nil {
		t.Fatal("expected error for unrelated paths")
	}
}

func TestCleanVirtualPath(t *testing.T) {
	if got := CleanVirtualPath("foo"); got != "/foo" {
		t.Fatalf("got %q", got)
	}
}
