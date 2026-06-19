#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

RESET=false
for arg in "$@"; do
  case "$arg" in
    -r|--reset) RESET=true ;;
    -h|--help)
      cat <<'EOF'
Usage: ./start.sh [-r]

  Start the LXC File Hub stack with Docker Compose.

  -r, --reset   Remove existing containers, images, and volumes before build/start.
                Use this when the UI looks stale or after pulling new changes.
EOF
      exit 0
      ;;
    *)
      echo "Unknown option: $arg" >&2
      exit 1
      ;;
  esac
done

log() { printf '[start] %s\n' "$*"; }

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required but not installed" >&2
  exit 1
fi

if [[ ! -f .env ]]; then
  log "creating .env from .env.example"
  cp .env.example .env
fi

mkdir -p secrets/dev
if [[ ! -f secrets/dev/master.key ]]; then
  log "generating secrets/dev/master.key"
  openssl rand -hex 32 > secrets/dev/master.key
  chmod 600 secrets/dev/master.key
fi

if [[ "$RESET" == true ]]; then
  log "resetting stack (containers, images, volumes)"
  docker compose --env-file .env down -v --remove-orphans --rmi local 2>/dev/null || true
  docker volume rm lxcfh_lxcfh-data 2>/dev/null || true
  docker volume rm lxcfh_lxcfh-fuse 2>/dev/null || true
  docker volume rm lxcfh_test-node-data 2>/dev/null || true
  docker volume rm lxcfh_test-node-uploads 2>/dev/null || true
  log "building images without cache"
  docker compose --env-file .env build --no-cache hub
else
  log "building images"
  docker compose --env-file .env build hub
fi

log "starting hub"
docker compose --env-file .env up -d hub

PORT="${LXCFH_HOST_PORT:-8080}"
if grep -q '^LXCFH_HOST_PORT=' .env 2>/dev/null; then
  PORT="$(grep '^LXCFH_HOST_PORT=' .env | tail -1 | cut -d= -f2-)"
fi

log "waiting for http://127.0.0.1:${PORT}/health/live"
for _ in $(seq 1 60); do
  if curl -fsS "http://127.0.0.1:${PORT}/health/live" | grep -q alive; then
    log "hub is ready at http://127.0.0.1:${PORT}"
    log "admin setup: http://127.0.0.1:${PORT}/setup"
    log "protocols page (admin): http://127.0.0.1:${PORT}/protocols"
    exit 0
  fi
  sleep 2
done

log "hub did not become healthy in time"
docker compose --env-file .env logs hub | tail -50
exit 1
