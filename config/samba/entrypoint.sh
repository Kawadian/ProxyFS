#!/usr/bin/env bash
set -euo pipefail

SMB_USER="${SMB_USER:-lxcfh}"
SMB_PASSWORD="${SMB_PASSWORD:-changeme}"
SMB_SHARE_NAME="${SMB_SHARE_NAME:-lxcfh}"
SMB_SHARE_PATH="${SMB_SHARE_PATH:-/fuse-share}"

log() {
  printf '[samba-entrypoint] %s\n' "$*"
}

mkdir -p "${SMB_SHARE_PATH}" /var/lib/samba/private /var/log/samba /run/samba
chown -R lxcfh:lxcfh "${SMB_SHARE_PATH}" /var/log/samba /run/samba || true

if ! pdbedit -L 2>/dev/null | grep -q "^${SMB_USER}:"; then
  log "creating Samba user ${SMB_USER}"
  (echo "${SMB_PASSWORD}"; echo "${SMB_PASSWORD}") | smbpasswd -a -s "${SMB_USER}"
else
  log "updating password for ${SMB_USER}"
  (echo "${SMB_PASSWORD}"; echo "${SMB_PASSWORD}") | smbpasswd -s "${SMB_USER}"
fi
smbpasswd -e "${SMB_USER}"

sed -i "s|^\\[lxcfh\\]|[${SMB_SHARE_NAME}]|" /etc/samba/smb.conf
sed -i "s|^   path = /fuse-share|   path = ${SMB_SHARE_PATH}|" /etc/samba/smb.conf

log "starting smbd and nmbd"
smbd --foreground --no-process-group &
SMBD_PID=$!
nmbd --foreground --no-process-group &
NMBD_PID=$!

term_handler() {
  kill -TERM "${SMBD_PID}" "${NMBD_PID}" 2>/dev/null || true
  wait "${SMBD_PID}" "${NMBD_PID}" 2>/dev/null || true
  exit 0
}
trap term_handler INT TERM

wait -n "${SMBD_PID}" "${NMBD_PID}"
