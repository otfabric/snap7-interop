#!/usr/bin/env bash
# Vulnerability scan for local images (and optional filesystem) via Trivy.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${root}"

OUT="${SCAN_OUT:-dist/scan}"
mkdir -p "${OUT}"

TRIVY_IMAGE="${TRIVY_IMAGE:-aquasec/trivy:0.62.1}"
# Comma-separated severities that fail the script (default: release gate).
SEVERITY="${SCAN_SEVERITY:-CRITICAL,HIGH}"
EXIT_CODE="${SCAN_EXIT_CODE:-1}"

scan_image() {
  local image="$1"
  local name="$2"
  if ! docker image inspect "${image}" >/dev/null 2>&1; then
    echo "skip ${name}: image ${image} not present"
    return 0
  fi
  echo "==> scanning ${image} (fail on ${SEVERITY})"
  # --ignore-unfixed: Debian/base CVEs without a fixed package version must not
  # block CI; release still fails on fixable CRITICAL/HIGH (see SECURITY.md).
  docker run --rm \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v "${root}/${OUT}:/out" \
    "${TRIVY_IMAGE}" image \
    --severity "${SEVERITY}" \
    --ignore-unfixed \
    --exit-code "${EXIT_CODE}" \
    --format table \
    --output "/out/${name}.txt" \
    "${image}"
  docker run --rm \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v "${root}/${OUT}:/out" \
    "${TRIVY_IMAGE}" image \
    --severity "${SEVERITY}" \
    --ignore-unfixed \
    --exit-code 0 \
    --format json \
    --output "/out/${name}.json" \
    "${image}" || true
}

scan_image "${NATIVE_IMAGE:-snap-interop-native:local}" native
scan_image "${PYTHON_IMAGE:-snap-interop-python:local}" python

echo "Scan reports under ${OUT}/"
ls -la "${OUT}"
