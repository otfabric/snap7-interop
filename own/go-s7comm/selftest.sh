#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
#
# Cross-stack self-test of the otfabric image: for every compiled fixture and
# every reference image, run interop-probe with the reference in the first
# endpoint and OWN_IMAGE in the second. The reference must pass every step;
# a failing step of OWN_IMAGE passes only when known-differences.tsv lists it.
#
# Environment: OWN_IMAGE, REFERENCE_VERSION (a harness release tag),
# REFERENCES ("native python"), BUILD_PLATFORM, REPORT_DIR (default
# reports/own), PROBE (default bin/interop-probe), FIXTURE (one id, optional).
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${root}"

: "${OWN_IMAGE:?OWN_IMAGE is required}"
: "${REFERENCE_VERSION:?REFERENCE_VERSION is required}"
REFERENCES="${REFERENCES:-native python}"
BUILD_PLATFORM="${BUILD_PLATFORM:-linux/amd64}"
REPORT_DIR="${REPORT_DIR:-reports/own}"
PROBE="${PROBE:-bin/interop-probe}"
DOCKER="${DOCKER:-docker}"
known="own/go-s7comm/known-differences.tsv"

if [[ -n "${FIXTURE:-}" ]]; then
  fixtures=("fixtures/compiled/${FIXTURE}.compiled.json")
else
  fixtures=(fixtures/compiled/*.compiled.json)
fi

start() { # image host-port fixture-id
  "${DOCKER}" run -d --rm --platform="${BUILD_PLATFORM}" -p "$2:102" \
    -v "${root}/fixtures/compiled:/fixtures:ro" --read-only --tmpfs /tmp \
    --tmpfs /run/snap-interop:uid=10001,gid=10001,mode=0755 \
    --cap-drop ALL --cap-add NET_BIND_SERVICE \
    -e "SNAP_INTEROP_FIXTURE=/fixtures/$3.compiled.json" "$1"
}

passed=0 knowns=0 failed=0
for ref in ${REFERENCES}; do
  ref_image="ghcr.io/otfabric/snap7-interop-${ref}:${REFERENCE_VERSION}"
  for fixture in "${fixtures[@]}"; do
    id="$(basename "${fixture}" .compiled.json)"
    out="${REPORT_DIR}/${ref}/${id}"
    mkdir -p "${out}"
    ref_cid="$(start "${ref_image}" 1102 "${id}")"
    own_cid="$(start "${OWN_IMAGE}" 2102 "${id}")"
    set +e
    "${PROBE}" run --fixture "${fixture}" --native 127.0.0.1:1102 --python 127.0.0.1:2102 \
      --out "${out}" >"${out}/probe.log" 2>&1
    set -e
    "${DOCKER}" logs "${ref_cid}" >"${out}/${ref}.log" 2>&1 || true
    "${DOCKER}" logs "${own_cid}" >"${out}/otfabric.log" 2>&1 || true
    "${DOCKER}" stop "${ref_cid}" "${own_cid}" >/dev/null 2>&1 || true
    set +e
    verdict="$(python3 own/go-s7comm/verdict.py "${out}/report.json" "${known}" "${id}")"
    rc=$?
    set -e
    echo "[interop] ${ref} vs otfabric ${id}: ${verdict}"
    case "${rc}" in
      0) passed=$((passed + 1)) ;;
      3) knowns=$((knowns + 1)) ;;
      *) failed=$((failed + 1)); cat "${out}/probe.log" >&2 ;;
    esac
  done
done

echo "[interop] summary: ${passed} passed, ${knowns} known differences, ${failed} failed"
[[ "${failed}" -eq 0 && "${passed}" -gt 0 ]]
