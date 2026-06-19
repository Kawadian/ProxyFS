//go:build integration

package integration_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lxcfh/lxcfh/internal/config"
)

func TestConfigAndHealthHandler(t *testing.T) {
	t.Setenv("SMB_ENABLED", "false")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	if cfg.WebAddr() == "" {
		t.Fatal("empty web addr")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	res, err := client.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("healthz: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", res.StatusCode)
	}
}
