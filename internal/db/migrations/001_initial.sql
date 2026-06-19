-- LXC File Hub initial schema

PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE COLLATE NOCASE,
    password_hash TEXT NOT NULL,
    display_name  TEXT NOT NULL DEFAULT '',
    email         TEXT NOT NULL DEFAULT '',
    role          TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('admin', 'user', 'guest')),
    enabled       INTEGER NOT NULL DEFAULT 1,
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS sessions (
    id         TEXT PRIMARY KEY,
    user_id    TEXT REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    ip_address TEXT NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT '',
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);

CREATE TABLE IF NOT EXISTS guest_ips (
    id         TEXT PRIMARY KEY,
    cidr       TEXT NOT NULL UNIQUE,
    label      TEXT NOT NULL DEFAULT '',
    enabled    INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS nodes (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE COLLATE NOCASE,
    provider    TEXT NOT NULL CHECK (provider IN ('sftp', 'local', 'smb')),
    host        TEXT NOT NULL DEFAULT '',
    port        INTEGER NOT NULL DEFAULT 22,
    root_path   TEXT NOT NULL DEFAULT '/',
    enabled     INTEGER NOT NULL DEFAULT 1,
    config_json TEXT NOT NULL DEFAULT '{}',
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_nodes_provider ON nodes(provider);

CREATE TABLE IF NOT EXISTS credentials (
    id              TEXT PRIMARY KEY,
    node_id         TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    auth_type       TEXT NOT NULL CHECK (auth_type IN ('password', 'private_key', 'agent')),
    username        TEXT NOT NULL DEFAULT '',
    secret_enc      BLOB,
    passphrase_enc  BLOB,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(node_id, name)
);
CREATE INDEX IF NOT EXISTS idx_credentials_node ON credentials(node_id);

CREATE TABLE IF NOT EXISTS keys (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL UNIQUE COLLATE NOCASE,
    key_type      TEXT NOT NULL CHECK (key_type IN ('ssh', 'api', 'generic')),
    material_enc  BLOB NOT NULL,
    fingerprint   TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS key_grants (
    id         TEXT PRIMARY KEY,
    key_id     TEXT NOT NULL REFERENCES keys(id) ON DELETE CASCADE,
    user_id    TEXT REFERENCES users(id) ON DELETE CASCADE,
    node_id    TEXT REFERENCES nodes(id) ON DELETE CASCADE,
    permission TEXT NOT NULL DEFAULT 'read' CHECK (permission IN ('read', 'write', 'admin')),
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(key_id, user_id, node_id)
);

CREATE TABLE IF NOT EXISTS transfers (
    id            TEXT PRIMARY KEY,
    node_id       TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    user_id       TEXT REFERENCES users(id) ON DELETE SET NULL,
    direction     TEXT NOT NULL CHECK (direction IN ('upload', 'download')),
    source_path   TEXT NOT NULL,
    dest_path     TEXT NOT NULL,
    total_bytes   INTEGER NOT NULL DEFAULT 0,
    offset_bytes  INTEGER NOT NULL DEFAULT 0,
    chunk_size    INTEGER NOT NULL DEFAULT 8388608,
    status        TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'paused', 'completed', 'failed', 'cancelled')),
    error_message TEXT NOT NULL DEFAULT '',
    checksum      TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at    TEXT NOT NULL DEFAULT (datetime('now')),
    completed_at  TEXT
);
CREATE INDEX IF NOT EXISTS idx_transfers_status ON transfers(status);
CREATE INDEX IF NOT EXISTS idx_transfers_node ON transfers(node_id);

CREATE TABLE IF NOT EXISTS uploads (
    id            TEXT PRIMARY KEY,
    transfer_id   TEXT REFERENCES transfers(id) ON DELETE SET NULL,
    node_id       TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    user_id       TEXT REFERENCES users(id) ON DELETE SET NULL,
    tus_id        TEXT NOT NULL UNIQUE,
    path          TEXT NOT NULL,
    total_size    INTEGER NOT NULL DEFAULT 0,
    offset_bytes  INTEGER NOT NULL DEFAULT 0,
    metadata_json TEXT NOT NULL DEFAULT '{}',
    status        TEXT NOT NULL DEFAULT 'created' CHECK (status IN ('created', 'uploading', 'completed', 'expired', 'cancelled')),
    storage_path  TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at    TEXT NOT NULL DEFAULT (datetime('now')),
    expires_at    TEXT
);
CREATE INDEX IF NOT EXISTS idx_uploads_tus ON uploads(tus_id);
CREATE INDEX IF NOT EXISTS idx_uploads_status ON uploads(status);

CREATE TABLE IF NOT EXISTS health_checks (
    id           TEXT PRIMARY KEY,
    node_id      TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    status       TEXT NOT NULL CHECK (status IN ('up', 'down', 'unknown')),
    latency_ms   INTEGER NOT NULL DEFAULT 0,
    message      TEXT NOT NULL DEFAULT '',
    checked_at   TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_health_node ON health_checks(node_id, checked_at DESC);

CREATE TABLE IF NOT EXISTS vfs_mounts (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE COLLATE NOCASE,
    node_id    TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    mount_path TEXT NOT NULL,
    read_only  INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS audit_log (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    TEXT,
    action     TEXT NOT NULL,
    resource   TEXT NOT NULL DEFAULT '',
    details    TEXT NOT NULL DEFAULT '',
    ip_address TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_log(created_at DESC);

CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL DEFAULT (datetime('now'))
);
