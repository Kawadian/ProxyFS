-- Protocol support: SSH user keys, WebDAV locks, Samba sync state

CREATE TABLE IF NOT EXISTS user_ssh_keys (
    id            TEXT PRIMARY KEY,
    user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name          TEXT NOT NULL DEFAULT '',
    fingerprint   TEXT NOT NULL DEFAULT '',
    public_key    TEXT NOT NULL,
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(user_id, fingerprint)
);
CREATE INDEX IF NOT EXISTS idx_user_ssh_keys_user ON user_ssh_keys(user_id);

CREATE TABLE IF NOT EXISTS webdav_locks (
    token         TEXT PRIMARY KEY,
    path          TEXT NOT NULL,
    owner         TEXT NOT NULL DEFAULT '',
    depth         INTEGER NOT NULL DEFAULT 0,
    timeout_secs  INTEGER NOT NULL DEFAULT 3600,
    exclusive     INTEGER NOT NULL DEFAULT 1,
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    expires_at    TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_webdav_locks_path ON webdav_locks(path);
CREATE INDEX IF NOT EXISTS idx_webdav_locks_expires ON webdav_locks(expires_at);

CREATE TABLE IF NOT EXISTS samba_accounts (
    user_id       TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    username      TEXT NOT NULL UNIQUE COLLATE NOCASE,
    nt_hash       TEXT NOT NULL,
    uid           INTEGER NOT NULL,
    gid           INTEGER NOT NULL,
    synced_at     TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS samba_sync_log (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT NOT NULL,
    action        TEXT NOT NULL,
    status        TEXT NOT NULL,
    detail        TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);
