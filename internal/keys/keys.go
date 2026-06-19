package keys

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/lxcfh/lxcfh/internal/crypto"
)

var (
	ErrKeyNotFound = errors.New("key not found")
	ErrKeyExists   = errors.New("key already exists")
)

// Key represents an encrypted key in the vault.
type Key struct {
	ID          string
	Name        string
	KeyType     string
	Fingerprint string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Vault stores and retrieves encrypted key material.
type Vault struct {
	db        *sql.DB
	masterKey []byte
}

// NewVault creates a key vault using the master encryption key.
func NewVault(db *sql.DB, masterKey []byte) *Vault {
	return &Vault{db: db, masterKey: masterKey}
}

// Store encrypts and persists key material.
func (v *Vault) Store(ctx context.Context, name, keyType string, material []byte) (*Key, error) {
	enc, err := crypto.Encrypt(v.masterKey, material)
	if err != nil {
		return nil, err
	}
	fp := fingerprint(material)
	id := fingerprint([]byte(name + keyType + fp))[:32]
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = v.db.ExecContext(ctx,
		`INSERT INTO keys (id, name, key_type, material_enc, fingerprint, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, name, keyType, enc, fp, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("store key: %w", err)
	}
	return &Key{
		ID:          id,
		Name:        name,
		KeyType:     keyType,
		Fingerprint: fp,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}, nil
}

// Get returns key metadata by name.
func (v *Vault) Get(ctx context.Context, name string) (*Key, error) {
	var k Key
	var created, updated string
	err := v.db.QueryRowContext(ctx,
		`SELECT id, name, key_type, fingerprint, created_at, updated_at FROM keys WHERE name = ? COLLATE NOCASE`,
		name,
	).Scan(&k.ID, &k.Name, &k.KeyType, &k.Fingerprint, &created, &updated)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrKeyNotFound
		}
		return nil, err
	}
	k.CreatedAt, _ = time.Parse(time.RFC3339, created)
	k.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
	return &k, nil
}

// Material decrypts and returns key material by ID.
func (v *Vault) Material(ctx context.Context, id string) ([]byte, error) {
	var enc []byte
	err := v.db.QueryRowContext(ctx, `SELECT material_enc FROM keys WHERE id = ?`, id).Scan(&enc)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrKeyNotFound
		}
		return nil, err
	}
	return crypto.Decrypt(v.masterKey, enc)
}

// MaterialByName decrypts key material by name.
func (v *Vault) MaterialByName(ctx context.Context, name string) ([]byte, error) {
	k, err := v.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	return v.Material(ctx, k.ID)
}

// Delete removes a key from the vault.
func (v *Vault) Delete(ctx context.Context, name string) error {
	res, err := v.db.ExecContext(ctx, `DELETE FROM keys WHERE name = ? COLLATE NOCASE`, name)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrKeyNotFound
	}
	return nil
}

// List returns all key metadata entries.
func (v *Vault) List(ctx context.Context) ([]Key, error) {
	rows, err := v.db.QueryContext(ctx,
		`SELECT id, name, key_type, fingerprint, created_at, updated_at FROM keys ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Key
	for rows.Next() {
		var k Key
		var created, updated string
		if err := rows.Scan(&k.ID, &k.Name, &k.KeyType, &k.Fingerprint, &created, &updated); err != nil {
			return nil, err
		}
		k.CreatedAt, _ = time.Parse(time.RFC3339, created)
		k.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
		out = append(out, k)
	}
	return out, rows.Err()
}

// Grant assigns a key to a user or node with a permission level.
func (v *Vault) Grant(ctx context.Context, keyID, userID, nodeID, permission string) error {
	id := fingerprint([]byte(keyID + userID + nodeID + permission))[:32]
	_, err := v.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO key_grants (id, key_id, user_id, node_id, permission) VALUES (?, ?, ?, ?, ?)`,
		id, keyID, nullIfEmpty(userID), nullIfEmpty(nodeID), permission,
	)
	return err
}

func fingerprint(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
