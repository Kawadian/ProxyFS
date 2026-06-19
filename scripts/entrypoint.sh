#!/usr/bin/env bash
set -euo pipefail

log() {
  printf '[entrypoint] %s\n' "$*"
}

die() {
  printf '[entrypoint] ERROR: %s\n' "$*" >&2
  exit 1
}

SMB_ENABLED="${SMB_ENABLED:-false}"
LXCFH_FUSE_MOUNT="${LXCFH_FUSE_MOUNT:-/fuse-mount}"
LXCFH_FUSE_BACKEND="${LXCFH_FUSE_BACKEND:-/fuse-share}"
LXCFH_DATA_DIR="${LXCFH_DATA_DIR:-/var/lib/lxcfh}"
LXCFH_MASTER_KEY_PATH="${LXCFH_MASTER_KEY_PATH:-/run/secrets/master.key}"
FUSE_PIDFILE="/run/lxcfh-fuse.pid"

mkdir -p "${LXCFH_DATA_DIR}" "${LXCFH_FUSE_BACKEND}" "${LXCFH_FUSE_MOUNT}"

if [[ -f "${LXCFH_MASTER_KEY_PATH}" ]]; then
  chmod 600 "${LXCFH_MASTER_KEY_PATH}" 2>/dev/null || true
else
  log "master key not found at ${LXCFH_MASTER_KEY_PATH}; hub will use dev fallback key"
fi

start_fuse() {
  if [[ "${SMB_ENABLED}" != "true" && "${SMB_ENABLED}" != "1" ]]; then
    log "SMB_ENABLED=${SMB_ENABLED}; FUSE mount skipped"
    return 0
  fi

  if [[ ! -c /dev/fuse ]]; then
    die "SMB_ENABLED=true but /dev/fuse is unavailable"
  fi

  if mountpoint -q "${LXCFH_FUSE_MOUNT}" 2>/dev/null; then
    log "FUSE already mounted at ${LXCFH_FUSE_MOUNT}"
    return 0
  fi

  log "starting FUSE loopback ${LXCFH_FUSE_MOUNT} -> ${LXCFH_FUSE_BACKEND}"
  lxcfh-fuse &
  echo $! > "${FUSE_PIDFILE}"

  for _ in $(seq 1 30); do
    if mountpoint -q "${LXCFH_FUSE_MOUNT}" 2>/dev/null; then
      log "FUSE mount ready"
      return 0
    fi
    sleep 0.5
  done

  die "FUSE mount did not become ready at ${LXCFH_FUSE_MOUNT}"
}

stop_fuse() {
  if [[ -f "${FUSE_PIDFILE}" ]]; then
    local pid
    pid="$(cat "${FUSE_PIDFILE}")"
    if kill -0 "${pid}" 2>/dev/null; then
      log "stopping FUSE (pid ${pid})"
      kill -TERM "${pid}" 2>/dev/null || true
      wait "${pid}" 2>/dev/null || true
    fi
    rm -f "${FUSE_PIDFILE}"
  fi

  if mountpoint -q "${LXCFH_FUSE_MOUNT}" 2>/dev/null; then
    fusermount3 -u "${LXCFH_FUSE_MOUNT}" 2>/dev/null \
      || fusermount -u "${LXCFH_FUSE_MOUNT}" 2>/dev/null \
      || true
  fi
}

trap stop_fuse EXIT INT TERM

start_fuse

log "starting lxcfh hub"
if [[ $# -gt 0 ]]; then
  exec "$@"
fi
exec lxcfh
