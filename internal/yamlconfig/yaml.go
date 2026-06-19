package yamlconfig

import (
	"context"
	"fmt"

	"github.com/lxcfh/lxcfh/internal/models"
	"github.com/lxcfh/lxcfh/internal/services"
	"github.com/lxcfh/lxcfh/internal/store"
	"gopkg.in/yaml.v3"
)

type Manager struct {
	svc *services.Services
}

func NewManager(svc *services.Services) *Manager {
	return &Manager{svc: svc}
}

func (m *Manager) Export(ctx context.Context) ([]byte, error) {
	cfg, err := m.svc.ExportConfig(ctx)
	if err != nil {
		return nil, err
	}
	return yaml.Marshal(BuildDocument(cfg))
}

func (m *Manager) Parse(data []byte) (models.ConfigYAMLDocument, error) {
	var doc models.ConfigYAMLDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return models.ConfigYAMLDocument{}, fmt.Errorf("invalid yaml: %w", err)
	}
	return doc, nil
}

func (m *Manager) Validate(ctx context.Context, data []byte) (models.ValidationResult, error) {
	doc, err := m.Parse(data)
	if err != nil {
		return models.ValidationResult{Valid: false, Errors: []string{err.Error()}}, nil
	}
	return m.validateDocument(ctx, doc)
}

func (m *Manager) validateDocument(ctx context.Context, doc models.ConfigYAMLDocument) (models.ValidationResult, error) {
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

	credNames := map[string]bool{}
	for _, c := range doc.Credentials {
		if c.Name == "" {
			result.Valid = false
			result.Errors = append(result.Errors, "credential name is required")
		}
		if credNames[c.Name] {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("duplicate credential name: %s", c.Name))
		}
		credNames[c.Name] = true
	}

	keyNames := map[string]bool{}
	for _, k := range doc.Keys {
		if k.Name == "" {
			result.Valid = false
			result.Errors = append(result.Errors, "key name is required")
		}
		if keyNames[k.Name] {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("duplicate key name: %s", k.Name))
		}
		keyNames[k.Name] = true
	}

	userNames := map[string]bool{}
	for _, u := range doc.Users {
		if u.Username == "" {
			result.Valid = false
			result.Errors = append(result.Errors, "user username is required")
		}
		if userNames[u.Username] {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("duplicate username: %s", u.Username))
		}
		userNames[u.Username] = true
	}

	setup, err := m.svc.IsSetup(ctx)
	if err != nil {
		return models.ValidationResult{}, err
	}
	if setup && len(doc.Users) == 0 {
		result.Warnings = append(result.Warnings, "import will remove all users")
	}

	return result, nil
}

func (m *Manager) Preview(ctx context.Context, data []byte) (models.ConfigPreview, error) {
	doc, err := m.Parse(data)
	if err != nil {
		return models.ConfigPreview{}, err
	}
	currentCfg, err := m.svc.ExportConfig(ctx)
	if err != nil {
		return models.ConfigPreview{}, err
	}
	current := BuildDocument(currentCfg)

	var preview models.ConfigPreview
	preview.Changes = append(preview.Changes, diffResources("settings", boolToInt(current.Settings != nil), boolToInt(doc.Settings != nil))...)
	preview.Changes = append(preview.Changes, diffList("nodes", current.Nodes, doc.Nodes, func(n models.NodeSpec) string { return n.Name })...)
	preview.Changes = append(preview.Changes, diffList("credentials", current.Credentials, doc.Credentials, func(c models.CredentialSpec) string { return c.Name })...)
	preview.Changes = append(preview.Changes, diffList("keys", current.Keys, doc.Keys, func(k models.KeySpec) string { return k.Name })...)
	preview.Changes = append(preview.Changes, diffList("users", current.Users, doc.Users, func(u models.UserSpec) string { return u.Username })...)
	return preview, nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func diffResources(resource string, oldCount, newCount int) []models.ConfigChange {
	if oldCount == newCount {
		return nil
	}
	return []models.ConfigChange{{Resource: resource, Action: "replace", Detail: fmt.Sprintf("%d -> %d", oldCount, newCount)}}
}

func diffList[T any](resource string, old, new []T, nameFn func(T) string) []models.ConfigChange {
	oldMap := map[string]bool{}
	for _, item := range old {
		oldMap[nameFn(item)] = true
	}
	newMap := map[string]bool{}
	for _, item := range new {
		newMap[nameFn(item)] = true
	}
	var changes []models.ConfigChange
	for name := range newMap {
		if !oldMap[name] {
			changes = append(changes, models.ConfigChange{Resource: resource, Action: "add", Detail: name})
		}
	}
	for name := range oldMap {
		if !newMap[name] {
			changes = append(changes, models.ConfigChange{Resource: resource, Action: "remove", Detail: name})
		}
	}
	for name := range newMap {
		if oldMap[name] {
			changes = append(changes, models.ConfigChange{Resource: resource, Action: "update", Detail: name})
		}
	}
	return changes
}

func (m *Manager) Apply(ctx context.Context, data []byte) error {
	doc, err := m.Parse(data)
	if err != nil {
		return err
	}
	result, err := m.validateDocument(ctx, doc)
	if err != nil {
		return err
	}
	if !result.Valid {
		return fmt.Errorf("%w: %v", store.ErrConflict, result.Errors)
	}
	if doc.Settings == nil {
		current, err := m.svc.ExportConfig(ctx)
		if err != nil {
			return err
		}
		settings := current.Settings
		doc.Settings = &settings
	}
	cfg, err := ToConfigDocument(doc)
	if err != nil {
		return fmt.Errorf("%w: %v", store.ErrConflict, err)
	}
	return m.svc.ApplyConfig(ctx, cfg)
}
