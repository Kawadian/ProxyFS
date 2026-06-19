# LXC File Hub

LXC File Hub (`lxcfh`) is a self-hosted file access hub that unifies many LXC containers and Linux nodes behind a single virtual root. Access the same files via Web UI, SFTP, SMB, and WebDAV.

## Quick start

```bash
cp .env.example .env
openssl rand -hex 32 > secrets/dev/master.key && chmod 600 secrets/dev/master.key
docker compose up -d --build
```

Open http://localhost:8080 and complete initial setup.

## Ports

| Service | Port | Path |
|---------|------|------|
| Web UI / API / WebDAV | 8080 | `/dav/` for WebDAV |
| SFTP | 2022 | — |
| SMB | 445 | share `LXCFileHub` (with Samba profile) |

## Architecture

- **hub** — Go backend: REST API, SFTP server, WebDAV, FUSE provider, transfer worker
- **samba** — Samba 4 sidecar mounting the FUSE export (profile `smb` or `full`)
- **test-node** — OpenSSH SFTP test node (profile `test`)

All protocols share a common Virtual File System (VFS). Each registered node appears as a top-level folder:

```
/
├── web-01/
├── web-02/
└── db-01/
```

## SMB / FUSE mode

```bash
SMB_ENABLED=true docker compose --profile smb up -d --build
```

Hub requires `/dev/fuse` and `SYS_ADMIN` capability (no `privileged: true`). Set `SMB_ENABLED=false` to run Web/SFTP/WebDAV only.

## Test SFTP node

```bash
docker compose --profile test up -d test-node
sftp -P 2222 sftpuser@127.0.0.1
```

## Persistence

Data persists in Docker volumes across `docker compose down && docker compose up -d`:

- `lxcfh-data` — SQLite DB, encrypted secrets, key vault
- `lxcfh-fuse` — shared FUSE mount for Samba
- `lxcfh-samba` — Samba credentials

## Master key

Production requires a 32+ byte master key at `/run/secrets/lxcfh_master_key` (or `LXCFH_MASTER_KEY_FILE`). Used for AES-256-GCM encryption of secrets and private keys.

## Development

```bash
make dev          # run hub locally
make build        # build with embedded frontend
make test         # unit + integration tests
make test-e2e     # Playwright tests
make smoke        # HTTP health checks
```

Frontend development:

```bash
cd frontend && npm install && npm run dev
```

## Client connections

Hub access (Web UI, SFTP, WebDAV, SMB) uses accounts created in the Web UI at `/setup` or **Users** (admin). Register SSH public keys under **Profile** (self-service) or **Users → SSH keys** (admin) for passwordless SFTP.

### Web UI
Browse to http://localhost:8080

### SFTP
```
Host: hub-host
Port: 2022
User: your-hub-username
Auth: password or registered SSH public key
```

### WebDAV (iOS Files, etc.)
```
URL: http://hub-host:8080/dav/
Auth: Basic (use HTTPS behind reverse proxy in production)
User: your-hub-username
```

### SMB (Windows Explorer)
```
\\hub-host\LXCFileHub
User: your-hub-username (same password as Web UI)
```

## Security

- Argon2id password hashing
- AES-256-GCM encrypted secrets at rest
- CSRF protection on Web API
- SSRF network policy for node connections
- Path traversal prevention
- Role-based access: admin / editor / viewer

See [docs/security.md](docs/security.md) for details.

## License

MIT — see [LICENSE](LICENSE). Dependency licenses: run `make sbom`.
