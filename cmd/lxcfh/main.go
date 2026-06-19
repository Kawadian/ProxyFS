package main

import (
	"context"
	"log"
	"os"

	"github.com/lxcfh/lxcfh/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	app, err := newRuntimeApp(cfg)
	if err != nil {
		log.Fatal(err)
	}

	if err := app.run(context.Background()); err != nil {
		log.Fatal(err)
	}
	os.Exit(0)
}
