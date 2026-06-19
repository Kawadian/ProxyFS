package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/lxcfh/lxcfh/internal/samba"
	_ "modernc.org/sqlite"
)

func main() {
	dbPath := flag.String("db", "/var/lib/lxcfh/lxcfh.db", "path to SQLite database")
	smbpasswd := flag.String("smbpasswd", "smbpasswd", "path to smbpasswd binary")
	dryRun := flag.Bool("dry-run", false, "log actions without modifying Samba")
	flag.Parse()

	db, err := sql.Open("sqlite", *dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	syncer := samba.NewSyncer(db, samba.Config{
		SmbpasswdPath: *smbpasswd,
		DryRun:        *dryRun,
	}, nil)

	result, err := syncer.SyncAll(context.Background(), nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sync failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("created=%d updated=%d removed=%d errors=%d\n",
		result.Created, result.Updated, result.Removed, len(result.Errors))
}
