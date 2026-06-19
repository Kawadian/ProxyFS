#!/usr/bin/env bash
# Sync Hub users to Samba with transactional rollback on failure.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

DB_PATH="${LXCFH_DB_PATH:-/var/lib/lxcfh/lxcfh.db}"
DRY_RUN="${SAMBA_SYNC_DRY_RUN:-false}"
SMBPASSWD="${SMBPASSWD_PATH:-smbpasswd}"

log() {
  echo "[samba-sync] $*"
}

if [[ ! -f "${DB_PATH}" ]]; then
  log "database not found: ${DB_PATH}"
  exit 1
fi

cd "${ROOT_DIR}"

ARGS=()
if [[ "${DRY_RUN}" == "true" ]]; then
  ARGS+=("-dry-run")
fi

log "syncing users from ${DB_PATH}"

if ! command -v go >/dev/null 2>&1; then
  log "go not found; building sync binary"
  exit 1
fi

# Run sync via a small Go entrypoint compiled on the fly.
go run "${ROOT_DIR}/internal/samba/cmd/sync/main.go" \
  -db "${DB_PATH}" \
  -smbpasswd "${SMBPASSWD}" \
  ${ARGS[@]+"${ARGS[@]}"}

log "samba sync complete"
