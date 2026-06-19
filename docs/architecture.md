# Architecture

## Overview

LXC File Hub exposes a unified virtual filesystem backed by SFTP connections to registered nodes.

```
SFTP client ───────┐
WebDAV client ─────┼──> Hub VirtualFS ──> SFTPNodeProvider ──> remote node
Web UI ────────────┘
                             │
                             └──> FUSE /mnt/lxcfh ──> Samba ──> SMB client
```

## Components

| Package | Responsibility |
|---------|----------------|
| `cmd/lxcfh` | Application entry, HTTP routing |
| `internal/vfs` | Virtual filesystem with mount points per node |
| `internal/nodes/sftp` | SFTP node provider with connection pool |
| `internal/protocols/sftpserver` | Hub SFTP server (port 2022) |
| `internal/protocols/webdav` | WebDAV at `/dav/` |
| `internal/protocols/fuse` | FUSE export for Samba |
| `internal/transfer` | Persistent async copy/move jobs |
| `internal/upload` | tus-compatible resumable uploads |
| `internal/api` | REST API and Web UI backend |
| `frontend` | React SPA |

## Data

SQLite (WAL mode) stores users, nodes, credentials, transfers, sessions, and settings. Private keys and passwords are encrypted with AES-256-GCM.

## Authentication

| Protocol | Password | SSH key | IP guest |
|----------|----------|---------|----------|
| Web UI   | Yes      | No      | Yes      |
| WebDAV   | Yes      | No      | Yes      |
| SFTP     | Yes      | Yes     | Yes      |
| SMB      | Yes      | No      | Yes      |

Roles: `admin`, `editor`, `viewer`. Viewer write operations are rejected at the Hub boundary.

## Path resolution

Virtual path: `/<node-slug>/<relative-path>`

- `/` lists enabled nodes
- Each node maps to its configured `root_path` on the remote SFTP server
- Path traversal (`..`, injection) is rejected

## Caching

- No persistent file content cache
- Directory/stat metadata: 2s TTL (configurable via env)
- Health results: 1h TTL
