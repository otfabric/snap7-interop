#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Shared helpers for scripts/smoke.sh and scripts/interop.sh. Source, do not execute.
#
# Each server runs as a container under the container contract
# (docs/CONTAINER_CONTRACT.md) with its port published on the loopback
# interface; interop-probe is the client.

REGISTRY="${REGISTRY:-ghcr.io/otfabric}"
# "local" means the images `make images` builds; a release tag (vX.Y.Z) means
# the published images of that release.
VERSION="${VERSION:-local}"
BUILD_PLATFORM="${BUILD_PLATFORM:-linux/amd64}"
PROBE="${PROBE:-bin/interop-probe}"
DOCKER="${DOCKER:-docker}"
READY_TIMEOUT="${READY_TIMEOUT:-30}"

log() { echo "[$(basename "$0" .sh)] $*" >&2; }

# image_of <adapter>: ADAPTER_IMAGE_<adapter> names an image outside the release
# set, which this repository's scripts know nothing about.
image_of() {
  local override="ADAPTER_IMAGE_$1"
  if [[ -n "${!override:-}" ]]; then
    echo "${!override}"
  elif [[ "${VERSION}" == "local" ]]; then
    echo "snap7-interop-$1:local"
  else
    echo "${REGISTRY}/snap7-interop-$1:${VERSION}"
  fi
}

require_tools() {
  local t
  for t in "${DOCKER}" python3; do
    command -v "$t" >/dev/null || { log "missing required tool: $t"; exit 2; }
  done
  [[ -x "${PROBE}" ]] || { log "missing ${PROBE}: run make probe-bin"; exit 2; }
}

# start_server <adapter> <host port> <fixture id> [docker run args...]: runs the
# image as the contract asks (read-only rootfs, no capability but
# NET_BIND_SERVICE) and prints the container ID.
start_server() {
  local adapter="$1" port="$2" fixture="$3"
  shift 3
  "${DOCKER}" run -d --platform="${BUILD_PLATFORM}" \
    -p "127.0.0.1:${port}:102" \
    -v "${ROOT}/fixtures/compiled:/fixtures:ro" \
    --read-only --tmpfs /tmp \
    --tmpfs /run/snap-interop:uid=10001,gid=10001,mode=0755 \
    --cap-drop ALL --cap-add NET_BIND_SERVICE \
    --label "snap7-interop.run=$$" \
    -e "SNAP_INTEROP_FIXTURE=/fixtures/${fixture}.compiled.json" \
    "$@" "$(image_of "${adapter}")"
}

stop_server() { "${DOCKER}" rm -f "$1" >/dev/null 2>&1 || true; }

cleanup() {
  "${DOCKER}" ps -aq --filter "label=snap7-interop.run=$$" | xargs -r "${DOCKER}" rm -f >/dev/null 2>&1 || true
}

# cases: the fixture IDs of tests/cases.tsv, or FIXTURE when set.
cases() {
  if [[ -n "${FIXTURE:-}" ]]; then
    echo "${FIXTURE}"
  else
    grep -vE '^[[:space:]]*(#|$)' "${ROOT}/tests/cases.tsv" | cut -f1
  fi
}
