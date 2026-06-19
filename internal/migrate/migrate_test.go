package migrate_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lxcfh/lxcfh/internal/migrate"
)

func TestRunAppliesMigrations(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	migrations := filepath.Join("..", "..", "migrations")

	if err := migrate.Run("sqlite://"+dbPath, migrations); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if err := migrate.Run("sqlite://"+dbPath, migrations); err != nil {
		t.Fatalf("Run second time: %v", err)
	}

	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("stat db: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("expected non-empty database")
	}
}

func TestRunMissingMigrationsDir(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	if err := migrate.Run("sqlite://"+dbPath, filepath.Join(dir, "missing")); err != nil {
		t.Fatalf("Run: %v", err)
	}
}
