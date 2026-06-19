#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export LXCFH_MASTER_KEY_PATH="${LXCFH_MASTER_KEY_PATH:-$ROOT/secrets/dev/master.key}"
export LXCFH_DATA_DIR="${E2E_DATA_DIR:-/tmp/lxcfh-e2e-playwright}"
export LXCFH_DB_PATH="$LXCFH_DATA_DIR/lxcfh.db"
export LXCFH_BIND_PORT="${LXCFH_BIND_PORT:-18080}"

if [[ ! -f "$LXCFH_MASTER_KEY_PATH" ]]; then
  mkdir -p "$(dirname "$LXCFH_MASTER_KEY_PATH")"
  openssl rand -hex 32 > "$LXCFH_MASTER_KEY_PATH"
  chmod 600 "$LXCFH_MASTER_KEY_PATH"
fi

mkdir -p "$LXCFH_DATA_DIR"
rm -f "$LXCFH_DB_PATH" "${LXCFH_DB_PATH}-wal" "${LXCFH_DB_PATH}-shm" 2>/dev/null || true

HUB_BIN="${HUB_BIN:-$ROOT/bin/lxcfh}"
if [[ ! -x "$HUB_BIN" ]]; then
  echo "hub binary not found at $HUB_BIN; run make build first" >&2
  exit 1
fi

exec "$HUB_BIN"
