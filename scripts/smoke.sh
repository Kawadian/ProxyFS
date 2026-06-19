#!/usr/bin/env bash
set -euo pipefail

HUB_URL="${HUB_URL:-http://127.0.0.1:8080}"
TIMEOUT="${SMOKE_TIMEOUT:-60}"

log() { printf '[smoke] %s\n' "$*"; }

wait_for_hub() {
  local i
  for ((i=1; i<=TIMEOUT; i++)); do
    if curl -fsS "${HUB_URL}/health/live" | grep -q alive; then
      log "hub healthy at ${HUB_URL}"
      return 0
    fi
    sleep 1
  done
  log "hub did not become healthy within ${TIMEOUT}s"
  return 1
}

wait_for_hub
log "smoke tests passed"
