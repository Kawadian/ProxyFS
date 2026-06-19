package store

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"
)

const (
	sambaSyncNonceKey = "samba_sync_nonce"
)

func (s *Store) ensureSambaTables() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS samba_accounts (
			user_id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			nt_hash TEXT NOT NULL,
			uid INTEGER NOT NULL,
			gid INTEGER NOT NULL,
			synced_at TEXT NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS samba_sync_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL,
			action TEXT NOT NULL,
			status TEXT NOT NULL,
			detail TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS samba_pending_passwords (
			username TEXT PRIMARY KEY,
			password TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) BumpSambaSyncNonce(ctx context.Context) error {
	val := strconv.FormatInt(time.Now().UnixNano(), 10)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO meta (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		sambaSyncNonceKey, val,
	)
	return err
}

func (s *Store) GetSambaSyncNonce(ctx context.Context) (string, error) {
	var val string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM meta WHERE key = ?`, sambaSyncNonceKey).Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return val, nil
}

func (s *Store) SetSambaPendingPassword(ctx context.Context, username, password string) error {
	if username == "" || password == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO samba_pending_passwords (username, password, created_at)
		VALUES (?, ?, ?)
		ON CONFLICT(username) DO UPDATE SET password = excluded.password, created_at = excluded.created_at`,
		username, password, now(),
	)
	return err
}

func (s *Store) ListSambaPendingPasswords(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT username, password FROM samba_pending_passwords`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]string)
	for rows.Next() {
		var username, password string
		if err := rows.Scan(&username, &password); err != nil {
			return nil, err
		}
		out[username] = password
	}
	return out, rows.Err()
}

func (s *Store) ClearSambaPendingPassword(ctx context.Context, username string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM samba_pending_passwords WHERE username = ?`, username)
	return err
}
