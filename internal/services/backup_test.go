package services_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/lxcfh/lxcfh/internal/models"
	"github.com/lxcfh/lxcfh/internal/services"
	"github.com/lxcfh/lxcfh/internal/store"
)

func TestRestoreNodesBackupWithoutIDs(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := services.New(st, t.TempDir())

	_, err = svc.CreateNode(ctx, models.Node{
		Name:     "server-a",
		Host:     "10.0.0.1",
		Port:     22,
		Username: "root",
		Enabled:  true,
	})
	if err != nil {
		t.Fatal(err)
	}

	yaml := `version: 1
nodes:
  - name: server-a
    host: 10.0.0.2
    port: 2222
    username: admin
    enabled: false
  - name: server-b
    host: 10.0.0.3
    port: 22
    username: deploy
    enabled: true
`
	if err := svc.Restore(ctx, []byte(yaml)); err != nil {
		t.Fatal(err)
	}

	nodes, err := svc.ListNodes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}

	byName := map[string]models.Node{}
	for _, n := range nodes {
		byName[n.Name] = n
	}
	if byName["server-a"].Host != "10.0.0.2" {
		t.Fatalf("server-a host = %q", byName["server-a"].Host)
	}
	if byName["server-a"].Enabled {
		t.Fatal("server-a should be disabled")
	}
	if byName["server-a"].ID == "" {
		t.Fatal("server-a should have generated id")
	}
}
