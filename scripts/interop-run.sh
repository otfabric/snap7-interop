#!/usr/bin/env bash
# Build both images, run selected compiled fixtures against both servers, write reports.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${root}"

export GOWORK="${GOWORK:-off}"
DOCKER="${DOCKER:-docker}"
PROBE="${PROBE:-bin/interop-probe}"
NATIVE_IMAGE="${NATIVE_IMAGE:-snap7-interop-native:local}"
PYTHON_IMAGE="${PYTHON_IMAGE:-snap7-interop-python:local}"
# Same default as the Makefile; set BUILD_PLATFORM=linux/arm64 on an arm64 host.
BUILD_PLATFORM="${BUILD_PLATFORM:-linux/amd64}"
export BUILD_PLATFORM

FIXTURE="${FIXTURE:-}"
if [[ -n "${FIXTURE}" ]]; then
  # Accept path or bare id.
  if [[ -f "${FIXTURE}" ]]; then
    fixtures=("${FIXTURE}")
  elif [[ -f "fixtures/compiled/${FIXTURE}.compiled.json" ]]; then
    fixtures=("fixtures/compiled/${FIXTURE}.compiled.json")
  elif [[ -f "fixtures/compiled/${FIXTURE}" ]]; then
    fixtures=("fixtures/compiled/${FIXTURE}")
  else
    echo "unknown FIXTURE=${FIXTURE}" >&2
    exit 2
  fi
else
  fixtures=(fixtures/compiled/*.compiled.json)
fi

make images probe-bin

failed=0
for fixture in "${fixtures[@]}"; do
  id="$(basename "${fixture}" .compiled.json)"
  echo "==> interop fixture ${id}"
  report_dir="reports/${id}"
  mkdir -p "${report_dir}"

  native_cid="$("${DOCKER}" run -d --rm --platform="${BUILD_PLATFORM}" \
    -p 1102:102 \
    -v "${root}/fixtures/compiled:/fixtures:ro" \
    --read-only \
    --tmpfs /tmp \
    --tmpfs /run/snap-interop:uid=10001,gid=10001,mode=0755 \
    --cap-drop ALL --cap-add NET_BIND_SERVICE \
    -e "SNAP_INTEROP_FIXTURE=/fixtures/${id}.compiled.json" \
    "${NATIVE_IMAGE}")"
  python_cid="$("${DOCKER}" run -d --rm --platform="${BUILD_PLATFORM}" \
    -p 2102:102 \
    -v "${root}/fixtures/compiled:/fixtures:ro" \
    --read-only \
    --tmpfs /tmp \
    --tmpfs /run/snap-interop:uid=10001,gid=10001,mode=0755 \
    --cap-drop ALL --cap-add NET_BIND_SERVICE \
    -e "SNAP_INTEROP_FIXTURE=/fixtures/${id}.compiled.json" \
    "${PYTHON_IMAGE}")"

  set +e
  "${PROBE}" run \
    --fixture "${fixture}" \
    --native 127.0.0.1:1102 \
    --python 127.0.0.1:2102 \
    --out "${report_dir}"
  rc=$?
  set -e

  if [[ "${rc}" -ne 0 ]]; then
    echo "FAIL fixture ${id}" >&2
    "${DOCKER}" logs "${native_cid}" >"${report_dir}/native.log" 2>&1 || true
    "${DOCKER}" logs "${python_cid}" >"${report_dir}/python.log" 2>&1 || true
    failed=1
  else
    echo "OK  fixture ${id} -> ${report_dir}"
  fi

  "${DOCKER}" stop "${native_cid}" >/dev/null 2>&1 || true
  "${DOCKER}" stop "${python_cid}" >/dev/null 2>&1 || true
done

if [[ "${failed}" -ne 0 ]]; then
  echo "interop failed; see reports/*/report.md" >&2
  exit 1
fi
echo "interop passed"
