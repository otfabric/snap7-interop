#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Cross-stack self-test: every case of tests/cases.tsv against two servers.
#
# Usage: interop.sh <adapter-a> <adapter-b>     e.g. interop.sh native python
#
# For each fixture both servers start from it and interop-probe runs its
# expectations against each and compares the two. The reports land in
# REPORT_DIR/<fixture>/ (report.json, report.md, junit.xml, server logs) and
# REPORT_DIR/results.json (interop-result-v1). A failing step listed in
# tests/known-differences.tsv for the adapter that failed it is a known difference.
# The run fails on any other failure, on a stale known-differences entry, and when
# nothing passed.
#
# Environment: VERSION (local or a release tag), ADAPTER_IMAGE_<adapter>,
# BUILD_PLATFORM, REPORT_DIR (default reports), FIXTURE (one fixture ID), PROBE.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}"
# shellcheck source=scripts/lib.sh
source scripts/lib.sh

[[ $# -eq 2 ]] || { echo "usage: interop.sh <adapter-a> <adapter-b>  (interop-probe compares two servers)" >&2; exit 2; }
a="$1" b="$2"
REPORT_DIR="${REPORT_DIR:-reports}"
require_tools
trap cleanup EXIT
log "server a: ${a} $(image_of "${a}"), server b: ${b} $(image_of "${b}")"

started="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
ran=()
while read -r id; do
  fixture="fixtures/compiled/${id}.compiled.json"
  [[ -f "${fixture}" ]] || { log "no compiled fixture ${fixture}"; exit 2; }
  out="${REPORT_DIR}/${id}"
  mkdir -p "${out}"
  a_cid=$(start_server "${a}" 1102 "${id}")
  b_cid=$(start_server "${b}" 2102 "${id}")
  t0=${SECONDS}
  set +e
  "${PROBE}" run --fixture "${fixture}" --native 127.0.0.1:1102 --python 127.0.0.1:2102 --out "${out}" >"${out}/probe.log" 2>&1
  rc=$?
  set -e
  ran+=("${id}=$(((SECONDS - t0) * 1000))")
  "${DOCKER}" logs "${a_cid}" >"${out}/${a}.log" 2>&1 || true
  "${DOCKER}" logs "${b_cid}" >"${out}/${b}.log" 2>&1 || true
  stop_server "${a_cid}"
  stop_server "${b_cid}"
  if [[ "${rc}" -eq 0 ]]; then
    echo "[interop] ${a} ${b} ${id}: pass"
  else
    echo "[interop] ${a} ${b} ${id}: not passed (probe exit ${rc}; see ${out}/report.md)"
  fi
done < <(cases)

# results.json decides: fail, infra-error, a stale known-differences entry or no pass fails the run.
python3 scripts/report-to-results.py "${REPORT_DIR}" "${REPORT_DIR}/results.json" "${started}" \
  "VERSION=${VERSION} BUILD_PLATFORM=${BUILD_PLATFORM} FIXTURE=${FIXTURE:-} scripts/interop.sh ${a} ${b}" \
  --servers "${a}=$(image_of "${a}"),${b}=$(image_of "${b}")" --known tests/known-differences.tsv "${ran[@]}"
echo "[interop] ${a} ${b}: passed"
