package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lxcfh/lxcfh/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("SMB_ENABLED", "")
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "master.key")
	if err := os.WriteFile(keyPath, []byte("01234567890123456789012345678901"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LXCFH_MASTER_KEY_PATH", keyPath)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BindPort != 8080 {
		t.Fatalf("port: got %d", cfg.BindPort)
	}
	if cfg.SMBEnabled {
		t.Fatal("expected SMB disabled by default")
	}
	if cfg.WebAddr() != "0.0.0.0:8080" {
		t.Fatalf("addr: %s", cfg.WebAddr())
	}
}

func TestLoadSMBEnabled(t *testing.T) {
	t.Setenv("SMB_ENABLED", "true")
	t.Setenv("LXCFH_FUSE_MOUNT", "/fuse-mount")
	t.Setenv("LXCFH_FUSE_BACKEND", "/fuse-share")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.SMBEnabled {
		t.Fatal("expected SMB enabled")
	}
}

func TestLoadInvalidPort(t *testing.T) {
	t.Setenv("LXCFH_BIND_PORT", "not-a-port")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid port")
	}
}

func TestParseBoolEnv(t *testing.T) {
	for _, tc := range []struct {
		val  string
		want bool
	}{
		{"true", true},
		{"1", true},
		{"false", false},
		{"", false},
	} {
		t.Setenv("SMB_ENABLED", tc.val)
		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("Load(%q): %v", tc.val, err)
		}
		if cfg.SMBEnabled != tc.want {
			t.Fatalf("SMB_ENABLED=%q: got %v want %v", tc.val, cfg.SMBEnabled, tc.want)
		}
	}
	os.Unsetenv("SMB_ENABLED")
}
