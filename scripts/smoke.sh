#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Container-contract smoke test for one adapter image.
#
# Usage: smoke.sh <adapter>          e.g. smoke.sh native
#
# Checks the parts of docs/CONTAINER_CONTRACT.md that need no second server:
# a missing fixture is refused before readiness; with basic-db the server
# logs server_ready, answers a read with the fixture's bytes, accepts a write
# that a second read sees, and stops cleanly on SIGTERM.
#
# Environment: VERSION (local or a release tag), ADAPTER_IMAGE_<adapter>,
# BUILD_PLATFORM, HOST_PORT (default 1102), PROBE.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}"
# shellcheck source=scripts/lib.sh
source scripts/lib.sh

adapter="${1:?usage: smoke.sh <adapter>}"
port="${HOST_PORT:-1102}"
require_tools
trap cleanup EXIT
log "${adapter}: $(image_of "${adapter}")"

# A fixture that does not exist is refused, with a non-zero exit, before readiness.
cid=$(start_server "${adapter}" "${port}" does-not-exist)
code=$("${DOCKER}" wait "${cid}" 2>/dev/null || echo "?")
if [[ "${code}" == "0" ]]; then
  log "FAIL: ${adapter} started with a missing fixture"
  exit 1
fi
log "ok: missing fixture refused (exit ${code})"

cid=$(start_server "${adapter}" "${port}" basic-db)
ok=0
for _ in $(seq 1 "${READY_TIMEOUT}"); do
  if "${PROBE}" -addr "127.0.0.1:${port}" -db 1 -offset 0 -length 8 -expect-hex 0102030405060708 >/dev/null 2>&1; then
    ok=1
    break
  fi
  sleep 1
done
if [[ "${ok}" -ne 1 ]]; then
  log "FAIL: ${adapter} read of basic-db"
  "${DOCKER}" logs "${cid}" >&2 || true
  exit 1
fi
log "ok: read DB1 0..7"
"${DOCKER}" logs "${cid}" 2>&1 | grep -q '"event": *"server_ready"' || { log "FAIL: no server_ready event"; exit 1; }
log "ok: server_ready logged"
"${PROBE}" -addr "127.0.0.1:${port}" -db 1 -offset 8 -length 4 -write-hex deadbeef >&2
log "ok: write DB1 8..11, read back"

"${DOCKER}" kill --signal TERM "${cid}" >/dev/null
code=$("${DOCKER}" wait "${cid}" 2>/dev/null || echo "?")
if [[ "${code}" != "0" ]]; then
  log "FAIL: ${adapter} exit ${code} on SIGTERM, want 0"
  exit 1
fi
log "ok: clean stop on SIGTERM"
echo "smoke-${adapter} passed"
