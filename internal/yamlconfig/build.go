package yamlconfig

import (
	"github.com/lxcfh/lxcfh/internal/models"
	"github.com/lxcfh/lxcfh/internal/nodesbackup"
)

func BuildDocument(cfg models.ConfigDocument) models.ConfigYAMLDocument {
	return nodesbackup.BuildDocument(cfg.Nodes, cfg.Credentials, cfg.Keys, cfg.Settings.DefaultNodePort)
}
