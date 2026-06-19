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
Usage: ./stop.sh [-r]

  Stop the LXC File Hub stack.

  -r, --reset   Stop and remove containers, local images, and named volumes.
EOF
      exit 0
      ;;
    *)
      echo "Unknown option: $arg" >&2
      exit 1
      ;;
  esac
done

log() { printf '[stop] %s\n' "$*"; }

ENV_FILE=".env"
if [[ ! -f "$ENV_FILE" ]]; then
  ENV_FILE=".env.example"
fi

if [[ "$RESET" == true ]]; then
  log "stopping and removing containers, images, and volumes"
  docker compose --env-file "$ENV_FILE" down -v --remove-orphans --rmi local
  docker volume rm lxcfh_lxcfh-data 2>/dev/null || true
  docker volume rm lxcfh_lxcfh-fuse 2>/dev/null || true
  docker volume rm lxcfh_test-node-data 2>/dev/null || true
  docker volume rm lxcfh_test-node-uploads 2>/dev/null || true
else
  log "stopping containers"
  docker compose --env-file "$ENV_FILE" down --remove-orphans
fi

log "done"
