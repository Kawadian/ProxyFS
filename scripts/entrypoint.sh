#!/usr/bin/env bash
set -euo pipefail

log() {
  printf '[entrypoint] %s\n' "$*"
}

LXCFH_DATA_DIR="${LXCFH_DATA_DIR:-/var/lib/lxcfh}"
LXCFH_MASTER_KEY_PATH="${LXCFH_MASTER_KEY_PATH:-/run/secrets/master.key}"
LXCFH_FUSE_MOUNT="${LXCFH_FUSE_MOUNT:-/fuse-mount}"

mkdir -p "${LXCFH_DATA_DIR}" "${LXCFH_FUSE_MOUNT}"

if [[ -f "${LXCFH_MASTER_KEY_PATH}" ]]; then
  chmod 600 "${LXCFH_MASTER_KEY_PATH}" 2>/dev/null || true
else
  log "master key not found at ${LXCFH_MASTER_KEY_PATH}; hub will use dev fallback key"
fi

log "starting lxcfh hub (protocol services controlled via Web UI)"
if [[ $# -gt 0 ]]; then
  exec "$@"
fi
exec lxcfh
