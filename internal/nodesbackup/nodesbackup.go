package nodesbackup

import (
	"fmt"

	"github.com/lxcfh/lxcfh/internal/models"
	"gopkg.in/yaml.v3"
)

func Marshal(doc models.NodesBackupDocument) ([]byte, error) {
	return yaml.Marshal(doc)
}

func Parse(data []byte) (models.NodesBackupDocument, error) {
	var doc models.NodesBackupDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return models.NodesBackupDocument{}, fmt.Errorf("invalid yaml: %w", err)
	}
	return doc, nil
}

func Validate(doc models.NodesBackupDocument) models.ValidationResult {
	var result models.ValidationResult
	result.Valid = true

	if doc.Version <= 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "version must be positive")
	}

	names := map[string]bool{}
	for _, n := range doc.Nodes {
		if n.Name == "" {
			result.Valid = false
			result.Errors = append(result.Errors, "node name is required")
			continue
		}
		if names[n.Name] {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("duplicate node name: %s", n.Name))
		}
		names[n.Name] = true
		if n.Host == "" {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("node %s: host is required", n.Name))
		}
	}

	return result
}

func BuildDocument(nodes []models.Node, creds []models.Credential, keys []models.SSHKey, defaultNodePort int) models.NodesBackupDocument {
	credByID := make(map[string]string, len(creds))
	for _, c := range creds {
		credByID[c.ID] = c.Name
	}
	keyByID := make(map[string]string, len(keys))
	for _, k := range keys {
		keyByID[k.ID] = k.Name
	}

	specs := make([]models.NodeSpec, 0, len(nodes))
	for _, n := range nodes {
		spec := models.NodeSpec{
			Name:     n.Name,
			Host:     n.Host,
			Username: n.Username,
			Labels:   n.Labels,
		}
		if defaultNodePort > 0 && n.Port != defaultNodePort {
			spec.Port = n.Port
		} else if defaultNodePort == 0 && n.Port != 0 {
			spec.Port = n.Port
		}
		if n.CredentialID != "" {
			spec.Credential = credByID[n.CredentialID]
		}
		if n.KeyID != "" {
			spec.Key = keyByID[n.KeyID]
		}
		if !n.Enabled {
			enabled := false
			spec.Enabled = &enabled
		}
		specs = append(specs, spec)
	}

	return models.NodesBackupDocument{Version: 1, Nodes: specs}
}
