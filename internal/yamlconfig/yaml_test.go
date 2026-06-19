package yamlconfig_test

import (
	"strings"
	"testing"
	"time"

	"github.com/lxcfh/lxcfh/internal/models"
	"github.com/lxcfh/lxcfh/internal/yamlconfig"
	"gopkg.in/yaml.v3"
)

func TestBuildDocumentOmitsAutoFields(t *testing.T) {
	now := time.Now()
	settings := models.Settings{
		SiteName:          "LXC File Hub",
		SessionTimeoutMin: 720,
		DefaultNodePort:   22,
		Protocols:         models.DefaultProtocolSettings(),
	}
	doc := yamlconfig.BuildDocument(models.ConfigDocument{
		Version:  1,
		Settings: settings,
		Nodes: []models.Node{{
			ID: "node-id", Name: "server-a", Host: "10.0.0.1", Port: 22, Username: "root",
			CredentialID: "cred-id", Enabled: true, CreatedAt: now, UpdatedAt: now,
			HostKeyStatus: "accepted", HostKeyFingerprint: "fp",
		}},
		Credentials: []models.Credential{{
			ID: "cred-id", Name: "main-cred", Type: "password", Username: "root",
			CreatedAt: now, UpdatedAt: now,
		}},
		Keys: []models.SSHKey{{
			ID: "key-id", Name: "deploy", Fingerprint: "SHA256:abc", PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIHRlc3Q=",
			CreatedAt: now, UpdatedAt: now,
		}},
		Users: []models.User{{
			ID: "user-id", Username: "admin", Role: models.RoleAdmin, Enabled: true,
			CreatedAt: now, UpdatedAt: now,
		}},
	})

	data, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, forbidden := range []string{
		"node-id", "cred-id", "key-id", "user-id",
		"created_at", "updated_at", "createdat", "updatedat",
		"fingerprint", "host_key", "credential_id", "key_id",
		"users:", "keys:", "settings:", "credentials:",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("unexpected field %q in export:\n%s", forbidden, text)
		}
	}
	if !strings.Contains(text, "main-cred") {
		t.Fatalf("expected credential name in export:\n%s", text)
	}
	if strings.Contains(text, "  enabled:") {
		t.Fatalf("enabled should be omitted when true:\n%s", text)
	}
}
