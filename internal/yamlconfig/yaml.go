package yamlconfig

import (
	"context"
	"fmt"

	"github.com/lxcfh/lxcfh/internal/models"
	"github.com/lxcfh/lxcfh/internal/nodesbackup"
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
	return nodesbackup.Parse(data)
}

func (m *Manager) Validate(ctx context.Context, data []byte) (models.ValidationResult, error) {
	doc, err := m.Parse(data)
	if err != nil {
		return models.ValidationResult{Valid: false, Errors: []string{err.Error()}}, nil
	}
	return nodesbackup.Validate(doc), nil
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
	preview.Changes = append(preview.Changes, diffList("nodes", current.Nodes, doc.Nodes, func(n models.NodeSpec) string { return n.Name })...)
	return preview, nil
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
	result := nodesbackup.Validate(doc)
	if !result.Valid {
		return fmt.Errorf("%w: %v", store.ErrConflict, result.Errors)
	}
	return m.svc.RestoreNodes(ctx, doc.Nodes)
}
