#!/usr/bin/env bash
# Ensure third-party notices and license pins stay present.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${root}"

fail=0

require_grep() {
  local file="$1"
  local pattern="$2"
  local msg="$3"
  if grep -Eq "${pattern}" "${file}"; then
    echo "OK: ${msg}"
  else
    echo "FAIL: ${msg}" >&2
    fail=1
  fi
}

if [[ ! -f THIRD_PARTY_NOTICES.md ]]; then
  echo "FAIL: missing THIRD_PARTY_NOTICES.md" >&2
  exit 1
fi

require_grep THIRD_PARTY_NOTICES.md 'LGPL-3.0' "Snap7 LGPL notice present"
require_grep THIRD_PARTY_NOTICES.md 'python-snap7' "python-snap7 notice present"
require_grep THIRD_PARTY_NOTICES.md 'MIT' "python-snap7 MIT license mentioned"
require_grep THIRD_PARTY_NOTICES.md 'f6ff90317ca5d54250f4dcd29209689a74e26d82' "Snap7 revision pinned in notices"
require_grep THIRD_PARTY_NOTICES.md '3\.2\.1' "python-snap7 version pinned in notices"
require_grep versions.yaml 'sha256:7b140f374b289a7c2befc338f42ebe6441b7ea838a042bbd5acbfca6ec875818' "debian base digest pinned"
require_grep versions.yaml 'sha256:d50fb7611f86d04a3b0471b46d7557818d88983fc3136726336b2a4c657aa30b' "python base digest pinned"
require_grep SECURITY.md 'CRITICAL' "SECURITY.md defines release-blocking severity"

if [[ "${fail}" -ne 0 ]]; then
  exit 1
fi
echo "check-notices passed"
