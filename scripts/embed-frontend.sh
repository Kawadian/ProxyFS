#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FRONTEND_DIR="${ROOT}/frontend"
WEB_DIST="${ROOT}/web/dist"
STATIC_DIR="${ROOT}/cmd/lxcfh/static"

cd "${FRONTEND_DIR}"
if [[ ! -d node_modules ]]; then
  npm install
fi
npm run build

rm -rf "${STATIC_DIR}"
mkdir -p "${STATIC_DIR}"
cp -a "${WEB_DIST}/." "${STATIC_DIR}/"
echo "embedded frontend -> ${STATIC_DIR}"
