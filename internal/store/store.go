package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lxcfh/lxcfh/internal/models"
	_ "modernc.org/sqlite"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
	ErrConflict      = errors.New("conflict")
)

type Store struct {
	db *sql.DB
	mu sync.RWMutex
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS meta (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS users (
	id TEXT PRIMARY KEY,
	username TEXT NOT NULL UNIQUE,
	display_name TEXT,
	email TEXT,
	password_hash TEXT NOT NULL,
	role TEXT NOT NULL,
	enabled INTEGER NOT NULL DEFAULT 1,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	last_login_at TEXT
);
CREATE TABLE IF NOT EXISTS user_ssh_keys (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	name TEXT NOT NULL,
	fingerprint TEXT NOT NULL,
	public_key TEXT NOT NULL,
	created_at TEXT NOT NULL,
	FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS sessions (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	csrf_token TEXT NOT NULL,
	expires_at TEXT NOT NULL,
	created_at TEXT NOT NULL,
	FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS nodes (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL UNIQUE,
	host TEXT NOT NULL,
	port INTEGER NOT NULL,
	username TEXT NOT NULL,
	credential_id TEXT,
	key_id TEXT,
	labels TEXT,
	enabled INTEGER NOT NULL DEFAULT 1,
	host_key_status TEXT NOT NULL DEFAULT 'unknown',
	host_key_fingerprint TEXT,
	last_ping_at TEXT,
	last_ping_status TEXT,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS credentials (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL UNIQUE,
	type TEXT NOT NULL,
	username TEXT,
	secret TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS ssh_keys (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL UNIQUE,
	fingerprint TEXT NOT NULL,
	public_key TEXT NOT NULL,
	private_key TEXT NOT NULL,
	comment TEXT,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS transfers (
	id TEXT PRIMARY KEY,
	node_id TEXT NOT NULL,
	source_path TEXT NOT NULL,
	dest_path TEXT NOT NULL,
	direction TEXT NOT NULL,
	status TEXT NOT NULL,
	bytes_total INTEGER NOT NULL DEFAULT 0,
	bytes_done INTEGER NOT NULL DEFAULT 0,
	error TEXT,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	completed_at TEXT,
	FOREIGN KEY(node_id) REFERENCES nodes(id)
);
CREATE TABLE IF NOT EXISTS uploads (
	id TEXT PRIMARY KEY,
	node_id TEXT NOT NULL,
	path TEXT NOT NULL,
	size INTEGER NOT NULL DEFAULT 0,
	offset INTEGER NOT NULL DEFAULT 0,
	status TEXT NOT NULL,
	data BLOB,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS settings (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	data TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS guest_ips (
	id TEXT PRIMARY KEY,
	cidr TEXT NOT NULL,
	label TEXT,
	enabled INTEGER NOT NULL DEFAULT 1
);
CREATE TABLE IF NOT EXISTS webdav_locks (
	token TEXT PRIMARY KEY,
	path TEXT NOT NULL,
	owner TEXT,
	depth TEXT,
	timeout_secs INTEGER,
	exclusive INTEGER NOT NULL DEFAULT 1,
	created_at TEXT NOT NULL,
	expires_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS health_checks (
	id TEXT PRIMARY KEY,
	node_id TEXT NOT NULL,
	status TEXT NOT NULL,
	latency_ms INTEGER,
	message TEXT,
	checked_at TEXT NOT NULL
);
`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	if err := s.runMigrations(); err != nil {
		return err
	}
	return s.ensureDefaultSettings()
}

func (s *Store) runMigrations() error {
	alters := []string{
		`ALTER TABLE nodes ADD COLUMN slug TEXT`,
		`ALTER TABLE nodes ADD COLUMN display_name TEXT`,
		`ALTER TABLE nodes ADD COLUMN root_path TEXT DEFAULT '/'`,
		`ALTER TABLE nodes ADD COLUMN provider TEXT DEFAULT 'sftp'`,
		`ALTER TABLE nodes ADD COLUMN read_only INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN unix_uid INTEGER`,
	}
	for _, stmt := range alters {
		_, _ = s.db.Exec(stmt)
	}
	return nil
}

func (s *Store) ensureDefaultSettings() error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM settings`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	def := models.Settings{
		SiteName:            "LXC File Hub",
		SessionTimeoutMin:   720,
		MaxUploadSizeMB:     1024,
		RateLimitPerMinute:  120,
		RequireReauth:       false,
		AllowRegistration:   false,
		DefaultNodePort:     22,
		BackupRetentionDays: 7,
	}
	data, err := json.Marshal(def)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO settings (id, data) VALUES (1, ?)`, string(data))
	return err
}

func (s *Store) IsSetup(ctx context.Context) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	return count > 0, err
}

func (s *Store) GetSettings(ctx context.Context) (models.Settings, error) {
	var data string
	err := s.db.QueryRowContext(ctx, `SELECT data FROM settings WHERE id = 1`).Scan(&data)
	if err != nil {
		return models.Settings{}, err
	}
	var settings models.Settings
	if err := json.Unmarshal([]byte(data), &settings); err != nil {
		return models.Settings{}, err
	}
	return settings, nil
}

func (s *Store) UpdateSettings(ctx context.Context, settings models.Settings) error {
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE settings SET data = ? WHERE id = 1`, string(data))
	return err
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func parseTimePtr(s sql.NullString) *time.Time {
	if !s.Valid || s.String == "" {
		return nil
	}
	t := parseTime(s.String)
	return &t
}

func labelsToJSON(labels map[string]string) string {
	if labels == nil {
		return "{}"
	}
	b, _ := json.Marshal(labels)
	return string(b)
}

func labelsFromJSON(s string) map[string]string {
	out := map[string]string{}
	_ = json.Unmarshal([]byte(s), &out)
	return out
}

func newID() string {
	return uuid.NewString()
}

func (s *Store) CreateUser(ctx context.Context, username, displayName, email, passwordHash string, role models.Role) (models.User, error) {
	id := newID()
	ts := now()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users (id, username, display_name, email, password_hash, role, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?)`,
		id, username, displayName, email, passwordHash, string(role), ts, ts)
	if err != nil {
		return models.User{}, err
	}
	return models.User{
		ID: id, Username: username, DisplayName: displayName, Email: email,
		Role: role, Enabled: true, CreatedAt: parseTime(ts), UpdatedAt: parseTime(ts),
	}, nil
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (models.User, string, error) {
	var u models.User
	var role string
	var enabled int
	var pwHash string
	var createdAt, updatedAt string
	var lastLogin sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, username, display_name, email, password_hash, role, enabled, created_at, updated_at, last_login_at
		FROM users WHERE username = ?`, username).Scan(
		&u.ID, &u.Username, &u.DisplayName, &u.Email, &pwHash, &role, &enabled,
		&createdAt, &updatedAt, &lastLogin)
	if err == sql.ErrNoRows {
		return models.User{}, "", ErrNotFound
	}
	if err != nil {
		return models.User{}, "", err
	}
	u.Role = models.Role(role)
	u.Enabled = enabled == 1
	u.CreatedAt = parseTime(createdAt)
	u.UpdatedAt = parseTime(updatedAt)
	u.LastLoginAt = parseTimePtr(lastLogin)
	return u, pwHash, nil
}

func (s *Store) GetUser(ctx context.Context, id string) (models.User, error) {
	var u models.User
	var role string
	var enabled int
	var createdAt, updatedAt string
	var lastLogin sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, username, display_name, email, role, enabled, created_at, updated_at, last_login_at
		FROM users WHERE id = ?`, id).Scan(
		&u.ID, &u.Username, &u.DisplayName, &u.Email, &role, &enabled,
		&createdAt, &updatedAt, &lastLogin)
	if err == sql.ErrNoRows {
		return models.User{}, ErrNotFound
	}
	if err != nil {
		return models.User{}, err
	}
	u.Role = models.Role(role)
	u.Enabled = enabled == 1
	u.CreatedAt = parseTime(createdAt)
	u.UpdatedAt = parseTime(updatedAt)
	u.LastLoginAt = parseTimePtr(lastLogin)
	return u, nil
}

func (s *Store) ListUsers(ctx context.Context) ([]models.User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, username, display_name, email, role, enabled, created_at, updated_at, last_login_at
		FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []models.User
	for rows.Next() {
		var u models.User
		var role string
		var enabled int
		var createdAt, updatedAt string
		var lastLogin sql.NullString
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &role, &enabled,
			&createdAt, &updatedAt, &lastLogin); err != nil {
			return nil, err
		}
		u.Role = models.Role(role)
		u.Enabled = enabled == 1
		u.CreatedAt = parseTime(createdAt)
		u.UpdatedAt = parseTime(updatedAt)
		u.LastLoginAt = parseTimePtr(lastLogin)
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) UpdateUser(ctx context.Context, id string, displayName, email *string, role *models.Role, enabled *bool) (models.User, error) {
	u, err := s.GetUser(ctx, id)
	if err != nil {
		return models.User{}, err
	}
	if displayName != nil {
		u.DisplayName = *displayName
	}
	if email != nil {
		u.Email = *email
	}
	if role != nil {
		u.Role = *role
	}
	if enabled != nil {
		u.Enabled = *enabled
	}
	ts := now()
	_, err = s.db.ExecContext(ctx, `
		UPDATE users SET display_name=?, email=?, role=?, enabled=?, updated_at=? WHERE id=?`,
		u.DisplayName, u.Email, string(u.Role), boolToInt(u.Enabled), ts, id)
	if err != nil {
		return models.User{}, err
	}
	u.UpdatedAt = parseTime(ts)
	return u, nil
}

func (s *Store) DeleteUser(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) UpdateUserPassword(ctx context.Context, id, passwordHash string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE users SET password_hash=?, updated_at=? WHERE id=?`, passwordHash, now(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) RecordLogin(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET last_login_at=?, updated_at=? WHERE id=?`, now(), now(), userID)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *Store) CreateSession(ctx context.Context, userID, csrf string, expires time.Time) (models.Session, error) {
	id := newID()
	ts := now()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions (id, user_id, csrf_token, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?)`, id, userID, csrf, expires.UTC().Format(time.RFC3339), ts)
	if err != nil {
		return models.Session{}, err
	}
	return models.Session{ID: id, UserID: userID, CSRFToken: csrf, ExpiresAt: expires, CreatedAt: parseTime(ts)}, nil
}

func (s *Store) GetSession(ctx context.Context, id string) (models.Session, error) {
	var sess models.Session
	var expires, created string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, csrf_token, expires_at, created_at FROM sessions WHERE id = ?`, id).Scan(
		&sess.ID, &sess.UserID, &sess.CSRFToken, &expires, &created)
	if err == sql.ErrNoRows {
		return models.Session{}, ErrNotFound
	}
	if err != nil {
		return models.Session{}, err
	}
	sess.ExpiresAt = parseTime(expires)
	sess.CreatedAt = parseTime(created)
	if time.Now().After(sess.ExpiresAt) {
		_ = s.DeleteSession(ctx, id)
		return models.Session{}, ErrNotFound
	}
	return sess, nil
}

func (s *Store) DeleteSession(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	return err
}

func (s *Store) ListUserSSHKeys(ctx context.Context, userID string) ([]models.UserSSHKey, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, name, fingerprint, public_key, created_at
		FROM user_ssh_keys WHERE user_id = ? ORDER BY name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []models.UserSSHKey
	for rows.Next() {
		var k models.UserSSHKey
		var createdAt string
		if err := rows.Scan(&k.ID, &k.UserID, &k.Name, &k.Fingerprint, &k.PublicKey, &createdAt); err != nil {
			return nil, err
		}
		k.CreatedAt = parseTime(createdAt)
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (s *Store) AddUserSSHKey(ctx context.Context, userID, name, fingerprint, publicKey string) (models.UserSSHKey, error) {
	id := newID()
	ts := now()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_ssh_keys (id, user_id, name, fingerprint, public_key, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`, id, userID, name, fingerprint, publicKey, ts)
	if err != nil {
		return models.UserSSHKey{}, err
	}
	return models.UserSSHKey{ID: id, UserID: userID, Name: name, Fingerprint: fingerprint, PublicKey: publicKey, CreatedAt: parseTime(ts)}, nil
}

func (s *Store) DeleteUserSSHKey(ctx context.Context, userID, keyID string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM user_ssh_keys WHERE id = ? AND user_id = ?`, keyID, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CreateNode(ctx context.Context, n models.Node) (models.Node, error) {
	id := newID()
	ts := now()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO nodes (id, name, host, port, username, credential_id, key_id, labels, enabled,
			host_key_status, host_key_fingerprint, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, n.Name, n.Host, n.Port, n.Username, nullStr(n.CredentialID), nullStr(n.KeyID),
		labelsToJSON(n.Labels), boolToInt(n.Enabled), n.HostKeyStatus, nullStr(n.HostKeyFingerprint), ts, ts)
	if err != nil {
		return models.Node{}, err
	}
	n.ID = id
	n.CreatedAt = parseTime(ts)
	n.UpdatedAt = parseTime(ts)
	return n, nil
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func (s *Store) scanNode(row interface{ Scan(...any) error }) (models.Node, error) {
	var n models.Node
	var labels string
	var enabled int
	var credID, keyID, hostFP sql.NullString
	var lastPing sql.NullString
	var lastStatus sql.NullString
	var createdAt, updatedAt string
	err := row.Scan(&n.ID, &n.Name, &n.Host, &n.Port, &n.Username, &credID, &keyID, &labels, &enabled,
		&n.HostKeyStatus, &hostFP, &lastPing, &lastStatus, &createdAt, &updatedAt)
	if err != nil {
		return models.Node{}, err
	}
	n.CreatedAt = parseTime(createdAt)
	n.UpdatedAt = parseTime(updatedAt)
	n.CredentialID = credID.String
	n.KeyID = keyID.String
	n.Labels = labelsFromJSON(labels)
	n.Enabled = enabled == 1
	n.HostKeyFingerprint = hostFP.String
	n.LastPingAt = parseTimePtr(lastPing)
	n.LastPingStatus = lastStatus.String
	return n, nil
}

const nodeSelect = `SELECT id, name, host, port, username, credential_id, key_id, labels, enabled,
	host_key_status, host_key_fingerprint, last_ping_at, last_ping_status, created_at, updated_at FROM nodes`

func (s *Store) GetNode(ctx context.Context, id string) (models.Node, error) {
	row := s.db.QueryRowContext(ctx, nodeSelect+` WHERE id = ?`, id)
	n, err := s.scanNode(row)
	if err == sql.ErrNoRows {
		return models.Node{}, ErrNotFound
	}
	return n, err
}

func (s *Store) ListNodes(ctx context.Context) ([]models.Node, error) {
	rows, err := s.db.QueryContext(ctx, nodeSelect+` ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var nodes []models.Node
	for rows.Next() {
		n, err := s.scanNode(rows)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

func (s *Store) UpdateNode(ctx context.Context, id string, n models.Node) (models.Node, error) {
	ts := now()
	res, err := s.db.ExecContext(ctx, `
		UPDATE nodes SET name=?, host=?, port=?, username=?, credential_id=?, key_id=?, labels=?,
			enabled=?, host_key_status=?, host_key_fingerprint=?, updated_at=? WHERE id=?`,
		n.Name, n.Host, n.Port, n.Username, nullStr(n.CredentialID), nullStr(n.KeyID),
		labelsToJSON(n.Labels), boolToInt(n.Enabled), n.HostKeyStatus, nullStr(n.HostKeyFingerprint), ts, id)
	if err != nil {
		return models.Node{}, err
	}
	aff, _ := res.RowsAffected()
	if aff == 0 {
		return models.Node{}, ErrNotFound
	}
	return s.GetNode(ctx, id)
}

func (s *Store) UpdateNodePing(ctx context.Context, id, status string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE nodes SET last_ping_at=?, last_ping_status=?, updated_at=? WHERE id=?`,
		now(), status, now(), id)
	return err
}

func (s *Store) DeleteNode(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM nodes WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CreateCredential(ctx context.Context, name, credType, username, secret string) (models.Credential, error) {
	id := newID()
	ts := now()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO credentials (id, name, type, username, secret, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, id, name, credType, username, secret, ts, ts)
	if err != nil {
		return models.Credential{}, err
	}
	return models.Credential{ID: id, Name: name, Type: credType, Username: username, CreatedAt: parseTime(ts), UpdatedAt: parseTime(ts)}, nil
}

func (s *Store) GetCredential(ctx context.Context, id string) (models.Credential, string, error) {
	var c models.Credential
	var secret string
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, type, username, secret, created_at, updated_at FROM credentials WHERE id = ?`, id).Scan(
		&c.ID, &c.Name, &c.Type, &c.Username, &secret, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return models.Credential{}, "", ErrNotFound
	}
	if err != nil {
		return models.Credential{}, "", err
	}
	c.CreatedAt = parseTime(createdAt)
	c.UpdatedAt = parseTime(updatedAt)
	return c, secret, nil
}

func (s *Store) ListCredentials(ctx context.Context) ([]models.Credential, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, type, username, created_at, updated_at FROM credentials ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var creds []models.Credential
	for rows.Next() {
		var c models.Credential
		var createdAt, updatedAt string
		if err := rows.Scan(&c.ID, &c.Name, &c.Type, &c.Username, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		c.CreatedAt = parseTime(createdAt)
		c.UpdatedAt = parseTime(updatedAt)
		creds = append(creds, c)
	}
	return creds, rows.Err()
}

func (s *Store) UpdateCredential(ctx context.Context, id, name, credType, username, secret string) (models.Credential, error) {
	ts := now()
	res, err := s.db.ExecContext(ctx, `
		UPDATE credentials SET name=?, type=?, username=?, secret=?, updated_at=? WHERE id=?`,
		name, credType, username, secret, ts, id)
	if err != nil {
		return models.Credential{}, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return models.Credential{}, ErrNotFound
	}
	c, _, err := s.GetCredential(ctx, id)
	return c, err
}

func (s *Store) DeleteCredential(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM credentials WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CreateSSHKey(ctx context.Context, name, fingerprint, publicKey, privateKey, comment string) (models.SSHKey, error) {
	id := newID()
	ts := now()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO ssh_keys (id, name, fingerprint, public_key, private_key, comment, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, id, name, fingerprint, publicKey, privateKey, comment, ts, ts)
	if err != nil {
		return models.SSHKey{}, err
	}
	return models.SSHKey{ID: id, Name: name, Fingerprint: fingerprint, PublicKey: publicKey, Comment: comment, CreatedAt: parseTime(ts), UpdatedAt: parseTime(ts)}, nil
}

func (s *Store) GetSSHKey(ctx context.Context, id string) (models.SSHKey, string, error) {
	var k models.SSHKey
	var priv string
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, fingerprint, public_key, private_key, comment, created_at, updated_at
		FROM ssh_keys WHERE id = ?`, id).Scan(
		&k.ID, &k.Name, &k.Fingerprint, &k.PublicKey, &priv, &k.Comment, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return models.SSHKey{}, "", ErrNotFound
	}
	if err != nil {
		return models.SSHKey{}, "", err
	}
	k.CreatedAt = parseTime(createdAt)
	k.UpdatedAt = parseTime(updatedAt)
	return k, priv, nil
}

func (s *Store) ListSSHKeys(ctx context.Context) ([]models.SSHKey, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, fingerprint, public_key, comment, created_at, updated_at FROM ssh_keys ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []models.SSHKey
	for rows.Next() {
		var k models.SSHKey
		var createdAt, updatedAt string
		if err := rows.Scan(&k.ID, &k.Name, &k.Fingerprint, &k.PublicKey, &k.Comment, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		k.CreatedAt = parseTime(createdAt)
		k.UpdatedAt = parseTime(updatedAt)
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (s *Store) DeleteSSHKey(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM ssh_keys WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CreateTransfer(ctx context.Context, t models.Transfer) (models.Transfer, error) {
	id := newID()
	ts := now()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO transfers (id, node_id, source_path, dest_path, direction, status, bytes_total, bytes_done, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		id, t.NodeID, t.SourcePath, t.DestPath, t.Direction, string(t.Status), t.BytesTotal, ts, ts)
	if err != nil {
		return models.Transfer{}, err
	}
	t.ID = id
	t.Status = models.TransferPending
	t.BytesDone = 0
	t.CreatedAt = parseTime(ts)
	t.UpdatedAt = parseTime(ts)
	return t, nil
}

func (s *Store) GetTransfer(ctx context.Context, id string) (models.Transfer, error) {
	var t models.Transfer
	var status string
	var completed sql.NullString
	var errMsg sql.NullString
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, node_id, source_path, dest_path, direction, status, bytes_total, bytes_done, error, created_at, updated_at, completed_at
		FROM transfers WHERE id = ?`, id).Scan(
		&t.ID, &t.NodeID, &t.SourcePath, &t.DestPath, &t.Direction, &status, &t.BytesTotal, &t.BytesDone,
		&errMsg, &createdAt, &updatedAt, &completed)
	if err == sql.ErrNoRows {
		return models.Transfer{}, ErrNotFound
	}
	if err != nil {
		return models.Transfer{}, err
	}
	t.Status = models.TransferStatus(status)
	t.Error = errMsg.String
	t.CreatedAt = parseTime(createdAt)
	t.UpdatedAt = parseTime(updatedAt)
	t.CompletedAt = parseTimePtr(completed)
	return t, nil
}

func (s *Store) ListTransfers(ctx context.Context) ([]models.Transfer, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, node_id, source_path, dest_path, direction, status, bytes_total, bytes_done, error, created_at, updated_at, completed_at
		FROM transfers ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var transfers []models.Transfer
	for rows.Next() {
		var t models.Transfer
		var status string
		var completed sql.NullString
		var errMsg sql.NullString
		var createdAt, updatedAt string
		if err := rows.Scan(&t.ID, &t.NodeID, &t.SourcePath, &t.DestPath, &t.Direction, &status,
			&t.BytesTotal, &t.BytesDone, &errMsg, &createdAt, &updatedAt, &completed); err != nil {
			return nil, err
		}
		t.Status = models.TransferStatus(status)
		t.Error = errMsg.String
		t.CreatedAt = parseTime(createdAt)
		t.UpdatedAt = parseTime(updatedAt)
		t.CompletedAt = parseTimePtr(completed)
		transfers = append(transfers, t)
	}
	return transfers, rows.Err()
}

func (s *Store) UpdateTransferStatus(ctx context.Context, id string, status models.TransferStatus, errMsg string) (models.Transfer, error) {
	ts := now()
	var completed interface{}
	if status == models.TransferCompleted || status == models.TransferFailed || status == models.TransferCancelled {
		completed = ts
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE transfers SET status=?, error=?, updated_at=?, completed_at=COALESCE(?, completed_at) WHERE id=?`,
		string(status), nullStr(errMsg), ts, completed, id)
	if err != nil {
		return models.Transfer{}, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return models.Transfer{}, ErrNotFound
	}
	return s.GetTransfer(ctx, id)
}

func (s *Store) DeleteTransfer(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM transfers WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CreateUpload(ctx context.Context, nodeID, path string, size int64) (models.Upload, error) {
	id := newID()
	ts := now()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO uploads (id, node_id, path, size, offset, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, 0, 'created', ?, ?)`, id, nodeID, path, size, ts, ts)
	if err != nil {
		return models.Upload{}, err
	}
	return models.Upload{ID: id, NodeID: nodeID, Path: path, Size: size, Status: "created", CreatedAt: parseTime(ts), UpdatedAt: parseTime(ts)}, nil
}

func (s *Store) GetUpload(ctx context.Context, id string) (models.Upload, error) {
	var u models.Upload
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, node_id, path, size, offset, status, created_at, updated_at FROM uploads WHERE id = ?`, id).Scan(
		&u.ID, &u.NodeID, &u.Path, &u.Size, &u.Offset, &u.Status, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return models.Upload{}, ErrNotFound
	}
	if err != nil {
		return models.Upload{}, err
	}
	u.CreatedAt = parseTime(createdAt)
	u.UpdatedAt = parseTime(updatedAt)
	return u, nil
}

func (s *Store) PatchUpload(ctx context.Context, id string, offset int64, data []byte) (models.Upload, error) {
	u, err := s.GetUpload(ctx, id)
	if err != nil {
		return models.Upload{}, err
	}
	if offset != u.Offset {
		return models.Upload{}, fmt.Errorf("%w: offset mismatch", ErrConflict)
	}
	newOffset := offset + int64(len(data))
	ts := now()
	_, err = s.db.ExecContext(ctx, `
		UPDATE uploads SET offset=?, data=COALESCE(data, '') || ?, status='uploading', updated_at=? WHERE id=?`,
		newOffset, data, ts, id)
	if err != nil {
		return models.Upload{}, err
	}
	u.Offset = newOffset
	u.Status = "uploading"
	u.UpdatedAt = parseTime(ts)
	if u.Offset >= u.Size {
		u.Status = "completed"
		_, _ = s.db.ExecContext(ctx, `UPDATE uploads SET status='completed' WHERE id=?`, id)
	}
	return u, nil
}

func (s *Store) DeleteUpload(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM uploads WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CountActiveTransfers(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM transfers WHERE status IN ('pending', 'running', 'paused')`).Scan(&count)
	return count, err
}

func (s *Store) ReplaceAllConfig(ctx context.Context, doc models.ConfigDocument) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tables := []string{"uploads", "transfers", "nodes", "credentials", "ssh_keys", "user_ssh_keys", "sessions", "users"}
	for _, t := range tables {
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+t); err != nil {
			return err
		}
	}

	data, err := json.Marshal(doc.Settings)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE settings SET data = ? WHERE id = 1`, string(data)); err != nil {
		return err
	}

	for _, u := range doc.Users {
		ts := now()
		_, err := tx.ExecContext(ctx, `
			INSERT INTO users (id, username, display_name, email, password_hash, role, enabled, created_at, updated_at)
			VALUES (?, ?, ?, ?, '', ?, ?, ?, ?)`,
			u.ID, u.Username, u.DisplayName, u.Email, string(u.Role), boolToInt(u.Enabled), ts, ts)
		if err != nil {
			return err
		}
	}
	for _, n := range doc.Nodes {
		ts := now()
		_, err := tx.ExecContext(ctx, `
			INSERT INTO nodes (id, name, host, port, username, credential_id, key_id, labels, enabled,
				host_key_status, host_key_fingerprint, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			n.ID, n.Name, n.Host, n.Port, n.Username, nullStr(n.CredentialID), nullStr(n.KeyID),
			labelsToJSON(n.Labels), boolToInt(n.Enabled), n.HostKeyStatus, nullStr(n.HostKeyFingerprint), ts, ts)
		if err != nil {
			return err
		}
	}
	for _, c := range doc.Credentials {
		ts := now()
		_, err := tx.ExecContext(ctx, `
			INSERT INTO credentials (id, name, type, username, secret, created_at, updated_at)
			VALUES (?, ?, ?, ?, '', ?, ?)`, c.ID, c.Name, c.Type, c.Username, ts, ts)
		if err != nil {
			return err
		}
	}
	for _, k := range doc.Keys {
		ts := now()
		_, err := tx.ExecContext(ctx, `
			INSERT INTO ssh_keys (id, name, fingerprint, public_key, private_key, comment, created_at, updated_at)
			VALUES (?, ?, ?, ?, '', ?, ?, ?)`, k.ID, k.Name, k.Fingerprint, k.PublicKey, k.Comment, ts, ts)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) ExportConfig(ctx context.Context) (models.ConfigDocument, error) {
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return models.ConfigDocument{}, err
	}
	nodes, err := s.ListNodes(ctx)
	if err != nil {
		return models.ConfigDocument{}, err
	}
	creds, err := s.ListCredentials(ctx)
	if err != nil {
		return models.ConfigDocument{}, err
	}
	keys, err := s.ListSSHKeys(ctx)
	if err != nil {
		return models.ConfigDocument{}, err
	}
	users, err := s.ListUsers(ctx)
	if err != nil {
		return models.ConfigDocument{}, err
	}
	return models.ConfigDocument{
		Version: 1, Settings: settings, Nodes: nodes, Credentials: creds, Keys: keys, Users: users,
	}, nil
}

func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}
