package nodesbackup

import (
	"strings"
	"testing"

	"github.com/lxcfh/lxcfh/internal/models"
)

func TestBuildDocumentOmitsIDs(t *testing.T) {
	data, err := Marshal(BuildDocument(
		[]models.Node{{
			ID:            "should-not-appear",
			Name:          "server-a",
			Host:          "10.0.0.1",
			Port:          22,
			Username:      "root",
			CredentialID:  "cred-id",
			Enabled:       true,
			HostKeyStatus: "accepted",
		}},
		[]models.Credential{{ID: "cred-id", Name: "main-cred"}},
		nil,
		22,
	))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, "should-not-appear") || strings.Contains(text, "cred-id") || strings.Contains(text, "host_key") {
		t.Fatalf("unexpected fields in export:\n%s", text)
	}
	if strings.Contains(text, "\n  enabled:") || strings.Contains(text, "\n    enabled:") {
		t.Fatalf("enabled should be omitted when true:\n%s", text)
	}
	if strings.Contains(text, "port:") {
		t.Fatalf("port should be omitted when default:\n%s", text)
	}
	if !strings.Contains(text, "main-cred") {
		t.Fatalf("expected credential name in export:\n%s", text)
	}
}

func TestParseAndValidate(t *testing.T) {
	doc, err := Parse([]byte(`version: 1
nodes:
  - name: a
    host: 1.1.1.1
`))
	if err != nil {
		t.Fatal(err)
	}
	if !Validate(doc).Valid {
		t.Fatal("expected valid document")
	}
}

func TestValidateRejectsDuplicateNames(t *testing.T) {
	result := Validate(models.NodesBackupDocument{
		Version: 1,
		Nodes: []models.NodeSpec{
			{Name: "dup", Host: "1.1.1.1"},
			{Name: "dup", Host: "2.2.2.2"},
		},
	})
	if result.Valid {
		t.Fatal("expected invalid result")
	}
}
