#!/usr/bin/env bash
set -euo pipefail

LXCFH_DB_PATH="${LXCFH_DB_PATH:-/var/lib/lxcfh/lxcfh.db}"
SMB_SHARE_NAME="${SMB_SHARE_NAME:-lxcfh}"
SMB_SHARE_PATH="${SMB_SHARE_PATH:-/fuse-share}"
SAMBA_SYNC_INTERVAL="${SAMBA_SYNC_INTERVAL:-5}"

log() {
  printf '[samba-entrypoint] %s\n' "$*"
}

mkdir -p "${SMB_SHARE_PATH}" /var/lib/samba/private /var/log/samba /run/samba
chown -R lxcfh:lxcfh "${SMB_SHARE_PATH}" /var/log/samba /run/samba || true

sed -i "s|^\\[lxcfh\\]|[${SMB_SHARE_NAME}]|" /etc/samba/smb.conf
sed -i "s|^   path = /fuse-share|   path = ${SMB_SHARE_PATH}|" /etc/samba/smb.conf

run_sync() {
  if [[ ! -f "${LXCFH_DB_PATH}" ]]; then
    log "waiting for hub database at ${LXCFH_DB_PATH}"
    return 1
  fi
  log "syncing Samba users from hub database"
  if lxcfh-samba-sync -db "${LXCFH_DB_PATH}"; then
    log "samba user sync complete"
    return 0
  fi
  log "samba user sync failed"
  return 1
}

wait_for_db() {
  local attempts=0
  while [[ ! -f "${LXCFH_DB_PATH}" && ${attempts} -lt 60 ]]; do
    sleep 2
    attempts=$((attempts + 1))
  done
}

watch_sync() {
  local last_nonce=""
  while true; do
    if [[ -f "${LXCFH_DB_PATH}" ]]; then
      nonce="$(sqlite3 "${LXCFH_DB_PATH}" "SELECT value FROM meta WHERE key = 'samba_sync_nonce' LIMIT 1;" 2>/dev/null || true)"
      if [[ -n "${nonce}" && "${nonce}" != "${last_nonce}" ]]; then
        run_sync || true
        last_nonce="${nonce}"
      fi
    fi
    sleep "${SAMBA_SYNC_INTERVAL}"
  done
}

wait_for_db
until run_sync; do
  sleep 5
done

watch_sync &
WATCHER_PID=$!

log "starting smbd and nmbd"
smbd --foreground --no-process-group &
SMBD_PID=$!
nmbd --foreground --no-process-group &
NMBD_PID=$!

term_handler() {
  kill -TERM "${WATCHER_PID}" "${SMBD_PID}" "${NMBD_PID}" 2>/dev/null || true
  wait "${WATCHER_PID}" "${SMBD_PID}" "${NMBD_PID}" 2>/dev/null || true
  exit 0
}
trap term_handler INT TERM

wait -n "${SMBD_PID}" "${NMBD_PID}"
