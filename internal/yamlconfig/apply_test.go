package yamlconfig_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/lxcfh/lxcfh/internal/models"
	"github.com/lxcfh/lxcfh/internal/services"
	"github.com/lxcfh/lxcfh/internal/store"
	"github.com/lxcfh/lxcfh/internal/yamlconfig"
)

func TestApplyRestoresNodesOnly(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := services.New(st, t.TempDir())
	mgr := yamlconfig.NewManager(svc)

	admin, _, err := svc.Setup(ctx, "admin", "secret", "")
	if err != nil {
		t.Fatal(err)
	}

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
  - name: server-b
    host: 10.0.0.3
    port: 22
    username: deploy
users:
  - username: ghost
    role: admin
keys:
  - name: orphan
    public_key: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIHRlc3Q="
`
	if err := mgr.Apply(ctx, []byte(yaml)); err != nil {
		t.Fatal(err)
	}

	nodes, err := svc.ListNodes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Name != "server-b" {
		t.Fatalf("node name = %q", nodes[0].Name)
	}

	users, err := svc.ListUsers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].ID != admin.ID {
		t.Fatalf("expected admin user preserved, got %+v", users)
	}

	keys, err := svc.ListSSHKeys(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 0 {
		t.Fatalf("expected no keys imported, got %d", len(keys))
	}
}

func TestPreviewShowsNodesOnly(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := services.New(st, t.TempDir())
	mgr := yamlconfig.NewManager(svc)

	if _, _, err := svc.Setup(ctx, "admin", "secret", ""); err != nil {
		t.Fatal(err)
	}

	yaml := `version: 1
nodes:
  - name: server-a
    host: 10.0.0.1
    port: 22
    username: root
users:
  - username: ghost
    role: admin
`
	preview, err := mgr.Preview(ctx, []byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range preview.Changes {
		if change.Resource != "nodes" {
			t.Fatalf("unexpected preview resource %q", change.Resource)
		}
	}
}
