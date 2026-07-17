#!/usr/bin/env bash
# Validate Milestone 0 repository bootstrap contracts.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${root}"

failures=0

fail() {
  echo "FAIL: $*" >&2
  failures=$((failures + 1))
}

ok() {
  echo "OK: $*"
}

require_file() {
  if [[ -f "$1" ]]; then
    ok "file $1"
  else
    fail "missing file $1"
  fi
}

require_dir() {
  if [[ -d "$1" ]]; then
    ok "dir $1"
  else
    fail "missing dir $1"
  fi
}

echo "==> checking required documents and meta files"
require_file README.md
require_file REQUIREMENTS.md
require_file PLAN.md
require_file LICENSE
require_file SECURITY.md
require_file CONTRIBUTING.md
require_file CODEOWNERS
require_file .editorconfig
require_file .gitignore
require_file Makefile
require_file compose.yaml
require_file versions.yaml

echo "==> checking directory skeleton"
require_dir fixtures/schema
require_dir fixtures/canonical
require_dir fixtures/invalid
require_dir fixtures/compiled
require_dir tooling/fixturectl
require_dir tooling/interop-probe
require_dir servers/native/src
require_dir servers/python/snap_interop_server
require_dir tests/conformance
require_dir tests/integration
require_dir tests/golden
require_dir docs/adr
require_dir .github/workflows
require_dir internal/fixture
require_file fixtures/schema/fixture.schema.json
require_file go.mod
require_file tooling/fixturectl/main.go
require_file tooling/interop-probe/main.go
require_file servers/native/src/main.cpp
require_file servers/native/CMakeLists.txt
require_file servers/python/snap_interop_server/main.py
require_file servers/python/requirements.txt
require_file servers/python/tests/test_no_native_lib.py

echo "==> checking ADRs"
for adr in \
  docs/adr/0001-fixture-first-architecture.md \
  docs/adr/0002-canonical-byte-compilation.md \
  docs/adr/0003-independent-server-implementations.md \
  docs/adr/0004-normalized-error-categories.md \
  docs/adr/0005-independent-repository-probe.md
do
  require_file "${adr}"
done
require_file docs/compatibility.md

echo "==> checking versions.yaml upstream pins"
if grep -Eq 'revision: "f6ff90317ca5d54250f4dcd29209689a74e26d82"' versions.yaml; then
  ok "native_snap7 revision pinned"
else
  fail "native_snap7.revision must remain the pinned immutable SHA"
fi
if grep -Eq 'version: "3.0.0"' versions.yaml; then
  ok "python_snap7 version pinned"
else
  fail "python_snap7.version must remain the pinned release"
fi

echo "==> parsing YAML manifests"
parse_yaml() {
  local file="$1"
  if python3 -c 'import yaml' >/dev/null 2>&1; then
    python3 -c "import yaml,sys; yaml.safe_load(open(sys.argv[1])); print('parsed', sys.argv[1])" "${file}"
    return 0
  fi
  if command -v ruby >/dev/null 2>&1; then
    ruby -ryaml -e "YAML.load_file(ARGV[0]); puts \"parsed #{ARGV[0]}\"" "${file}"
    return 0
  fi
  return 1
}

if parse_yaml versions.yaml && parse_yaml compose.yaml; then
  ok "versions.yaml and compose.yaml parse"
else
  fail "need PyYAML (python3) or Ruby stdlib YAML to parse versions.yaml/compose.yaml"
fi

echo "==> checking placeholder Dockerfiles exist"
require_file servers/native/Dockerfile
require_file servers/python/Dockerfile

echo "==> checking compose service names"
if grep -Eq '^[[:space:]]+native:' compose.yaml && grep -Eq '^[[:space:]]+python:' compose.yaml; then
  ok "compose.yaml defines native and python services"
else
  fail "compose.yaml must define native and python services"
fi

echo "==> checking PLAN.md status section"
if grep -Eq '^## Status' PLAN.md; then
  ok "PLAN.md has Status section"
else
  fail "PLAN.md must track implementation status"
fi

if [[ "${failures}" -ne 0 ]]; then
  echo "bootstrap-check failed with ${failures} error(s)" >&2
  exit 1
fi

echo "bootstrap-check passed"
