#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Checks that the pins in the adapters equal those in versions.yaml: the Snap7
# revision, the python-snap7 version and the base image digests. Nothing
# floats (AGENTS.md rule 1).
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${root}"

fail=0
check() { # description want got
  if [[ "$2" == "$3" ]]; then
    echo "ok   $1"
  else
    echo "FAIL $1: versions.yaml has '$2', the adapter '$3'" >&2
    fail=1
  fi
}
yaml() { sed -nE "$1" versions.yaml | head -1; }

check "Snap7 revision" \
  "$(yaml '/^native_snap7:/,/^[a-z]/ s/^  revision: "([^"]+)".*/\1/p')" \
  "$(sed -nE 's/^ARG SNAP7_REVISION=(.+)$/\1/p' adapters/native/Dockerfile)"
check "python-snap7 version" \
  "$(yaml '/^python_snap7:/,/^[a-z]/ s/^  version: "([^"]+)".*/\1/p')" \
  "$(sed -nE 's/^python-snap7==(.+)$/\1/p' adapters/python/requirements.txt)"
check "native base image digest" \
  "$(yaml 's/^  native_runtime: ".*@(sha256:[0-9a-f]+)".*/\1/p')" \
  "$(sed -nE 's/^ARG DEBIAN_DIGEST=(sha256:[0-9a-f]+).*/\1/p' adapters/native/Dockerfile)"
check "python base image digest" \
  "$(yaml 's/^  python_runtime: ".*@(sha256:[0-9a-f]+)".*/\1/p')" \
  "$(sed -nE 's/^ARG PYTHON_DIGEST=(sha256:[0-9a-f]+).*/\1/p' adapters/python/Dockerfile)"
check "otfabric build image digest" \
  "$(sed -nE 's/^      build: ".*@(sha256:[0-9a-f]+)".*/\1/p' versions.yaml | head -1)" \
  "$(sed -nE 's/^ARG GOLANG_DIGEST=(sha256:[0-9a-f]+).*/\1/p' own/go-s7comm/Dockerfile)"
check "otfabric runtime image digest" \
  "$(sed -nE 's/^      runtime: ".*@(sha256:[0-9a-f]+)".*/\1/p' versions.yaml | head -1)" \
  "$(sed -nE 's/^ARG DEBIAN_DIGEST=(sha256:[0-9a-f]+).*/\1/p' own/go-s7comm/Dockerfile)"
exit "${fail}"
