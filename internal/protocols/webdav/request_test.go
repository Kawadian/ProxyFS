package webdav

import (
	"net/http"
	"testing"
)

func TestIsWebDAVRequest(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "http://example/", nil)
	if IsWebDAVRequest(req) {
		t.Fatal("GET should not be treated as WebDAV")
	}

	req, _ = http.NewRequest("PROPFIND", "http://example/", nil)
	if !IsWebDAVRequest(req) {
		t.Fatal("PROPFIND should be WebDAV")
	}

	req, _ = http.NewRequest(http.MethodOptions, "http://example/", nil)
	req.Header.Set("DAV", "1")
	if !IsWebDAVRequest(req) {
		t.Fatal("OPTIONS with DAV header should be WebDAV")
	}
}

func TestNormalizeRequestPath(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"/", "/"},
		{"/node/file", "/node/file"},
		{"/dav/", "/"},
		{"/dav/node/file", "/node/file"},
	}
	for _, tt := range tests {
		if got := NormalizeRequestPath(tt.in); got != tt.want {
			t.Fatalf("%q: got %q want %q", tt.in, got, tt.want)
		}
	}
}
