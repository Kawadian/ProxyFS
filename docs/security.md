# Security

## Authentication

- Passwords hashed with Argon2id
- Web sessions: HttpOnly cookies, CSRF tokens, 12h default TTL
- SFTP public key auth from `user_ssh_keys` table
- Guest access via CIDR allowlist (disabled by default)

## Encryption at rest

- Master key: `/run/secrets/lxcfh_master_key` (32+ bytes)
- Private keys and passwords: AES-256-GCM
- Samba credentials stored in separate volume, not readable from Hub API

## Network

- SSRF protection: node connection allow/deny CIDR lists
- DNS rebinding: resolve and validate all IPs before dial
- Trusted proxy CIDRs required for `X-Forwarded-For`

## Filesystem

- Path traversal prevention on virtual paths
- Viewer role cannot write via any protocol
- Node credentials are shared (Hub acts as proxy); per-user node accounts are not created

## Operations

- Secrets never logged (passwords, keys, tokens, cookies)
- Login rate limiting
- YAML import: alias bomb protection, schema validation, full-replace confirmation

## Production recommendations

- Terminate TLS at reverse proxy (Caddy, Nginx, Traefik)
- Use HTTPS for WebDAV Basic auth on WAN
- Set explicit master key; do not rely on dev fallback
- Restrict SFTP/SMB ports to LAN/VPN
