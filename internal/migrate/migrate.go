package migrate

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

// Run applies pending *.up.sql migrations in lexical order.
func Run(databaseURL, migrationsPath string) error {
	if strings.TrimSpace(databaseURL) == "" {
		return nil
	}

	dbPath, err := sqlitePath(databaseURL)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil {
		return fmt.Errorf("data dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	if err := ensureSchemaTable(db); err != nil {
		return err
	}

	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read migrations: %w", err)
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".up.sql") {
			files = append(files, name)
		}
	}
	sort.Strings(files)

	for _, name := range files {
		version := strings.TrimSuffix(name, ".up.sql")
		applied, err := isApplied(db, version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		body, err := os.ReadFile(filepath.Join(migrationsPath, name))
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		if _, err := db.Exec(string(body)); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if _, err := db.Exec(`INSERT INTO schema_migrations(version) VALUES (?)`, version); err != nil {
			return fmt.Errorf("record %s: %w", name, err)
		}
	}
	return nil
}

func sqlitePath(databaseURL string) (string, error) {
	switch {
	case strings.HasPrefix(databaseURL, "sqlite://"):
		return strings.TrimPrefix(databaseURL, "sqlite://"), nil
	case strings.HasPrefix(databaseURL, "sqlite3://"):
		return strings.TrimPrefix(databaseURL, "sqlite3://"), nil
	default:
		return "", fmt.Errorf("unsupported database url: %s", databaseURL)
	}
}

func ensureSchemaTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`)
	return err
}

func isApplied(db *sql.DB, version string) (bool, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(1) FROM schema_migrations WHERE version = ?`, version).Scan(&count)
	return count > 0, err
}
