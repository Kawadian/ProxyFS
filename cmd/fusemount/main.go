package main

import (
	"os"

	"github.com/lxcfh/lxcfh/internal/config"
	"github.com/lxcfh/lxcfh/internal/fusemount"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	if !cfg.SMBEnabled {
		os.Exit(0)
	}
	if err := fusemount.Mount(cfg.FuseMountPath, cfg.FuseBackendPath); err != nil {
		panic(err)
	}
}
