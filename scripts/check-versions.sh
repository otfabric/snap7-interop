#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Checks that the pins in the adapters equal those in versions.yaml: the Snap7
# revision, the python-snap7 version and the base image digests. Nothing
# floats (AGENTS.md rule 1). Also that the Snap7 version is its short revision
# and that a reference is published: true exactly when the release publishes
# it (PUBLISHED_ADAPTERS of the Makefile) and images: names its image.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${root}"

fail=0
check() { # description want got [where got is from]
  if [[ "$2" == "$3" ]]; then
    echo "ok   $1"
  else
    echo "FAIL $1: versions.yaml has '$2', ${4:-the adapter} '$3'" >&2
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

snap7_revision="$(sed -nE 's/^ARG SNAP7_REVISION=(.+)$/\1/p' adapters/native/Dockerfile)"
check "Snap7 version (the fork has no releases: the short revision)" \
  "$(yaml '/^native_snap7:/,/^[a-z]/ s/^  version: "([^"]+)".*/\1/p')" \
  "${snap7_revision:0:12}"

published_adapters=" $(sed -nE 's/^PUBLISHED_ADAPTERS \?= *(.*)$/\1/p' Makefile) "
for pair in native:native_snap7 python:python_snap7; do
  adapter="${pair%%:*}" section="${pair#*:}"
  published="$(yaml "/^${section}:/,/^[a-z]/ s/^  published: *([a-z]+).*/\1/p")"
  in_makefile=false
  [[ "${published_adapters}" == *" ${adapter} "* ]] && in_makefile=true
  check "${adapter} published" "${published}" "${in_makefile}" "PUBLISHED_ADAPTERS of the Makefile"
  named=false
  [[ -n "$(yaml "/^images:/,/^[a-z]/ s/^  ${adapter}: \"([^\"]+)\".*/\1/p")" ]] && named=true
  check "${adapter} image" "${published}" "${named}" "images: (has an entry)"
done
exit "${fail}"
