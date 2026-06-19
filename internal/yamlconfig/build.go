package yamlconfig

import (
	"github.com/lxcfh/lxcfh/internal/models"
	"github.com/lxcfh/lxcfh/internal/nodesbackup"
)

func BuildDocument(cfg models.ConfigDocument) models.ConfigYAMLDocument {
	settings := cfg.Settings
	return models.ConfigYAMLDocument{
		Version:     cfg.Version,
		Settings:    &settings,
		Nodes:       nodesbackup.BuildDocument(cfg.Nodes, cfg.Credentials, cfg.Keys, cfg.Settings.DefaultNodePort).Nodes,
		Credentials: buildCredentialSpecs(cfg.Credentials),
		Keys:        buildKeySpecs(cfg.Keys),
		Users:       buildUserSpecs(cfg.Users),
	}
}

func buildCredentialSpecs(creds []models.Credential) []models.CredentialSpec {
	specs := make([]models.CredentialSpec, 0, len(creds))
	for _, c := range creds {
		specs = append(specs, models.CredentialSpec{
			Name:     c.Name,
			Type:     c.Type,
			Username: c.Username,
		})
	}
	return specs
}

func buildKeySpecs(keys []models.SSHKey) []models.KeySpec {
	specs := make([]models.KeySpec, 0, len(keys))
	for _, k := range keys {
		specs = append(specs, models.KeySpec{
			Name:      k.Name,
			PublicKey: k.PublicKey,
			Comment:   k.Comment,
		})
	}
	return specs
}

func buildUserSpecs(users []models.User) []models.UserSpec {
	specs := make([]models.UserSpec, 0, len(users))
	for _, u := range users {
		spec := models.UserSpec{
			Username:    u.Username,
			DisplayName: u.DisplayName,
			Email:       u.Email,
			Role:        u.Role,
		}
		if !u.Enabled {
			enabled := false
			spec.Enabled = &enabled
		}
		specs = append(specs, spec)
	}
	return specs
}
