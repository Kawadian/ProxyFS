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

	admin, err := svc.CreateUser(ctx, "admin", "secret", "Admin", "", models.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.CreateNode(ctx, models.Node{
		Name: "old-node", Host: "10.0.0.1", Port: 22, Username: "root", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	yaml := `version: 1
nodes:
  - name: new-node
    host: 10.0.0.2
    port: 22
    username: deploy
`
	if err := mgr.Apply(ctx, []byte(yaml)); err != nil {
		t.Fatal(err)
	}

	users, err := svc.ListUsers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].Username != admin.Username {
		t.Fatalf("users changed: %+v", users)
	}

	nodes, err := svc.ListNodes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].Name != "new-node" {
		t.Fatalf("nodes = %+v", nodes)
	}
}
