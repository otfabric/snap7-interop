#!/usr/bin/env bash
# Generate SPDX + CycloneDX SBOMs for source and (optionally) local images.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${root}"

OUT="${SBOM_OUT:-dist/sbom}"
mkdir -p "${OUT}"

SYFT_IMAGE="${SYFT_IMAGE:-anchore/syft:v1.22.0}"

run_syft() {
  local target="$1"
  local spdx="$2"
  local cdx="$3"
  docker run --rm \
    -v "${root}:/work:ro" \
    -v "${root}/${OUT}:/out" \
    -w /work \
    "${SYFT_IMAGE}" \
    "${target}" \
    -o "spdx-json=/out/$(basename "${spdx}")" \
    -o "cyclonedx-json=/out/$(basename "${cdx}")"
}

echo "==> source SBOM (repository tree)"
run_syft dir:. "${OUT}/source.spdx.json" "${OUT}/source.cdx.json"

if docker image inspect "${NATIVE_IMAGE:-snap-interop-native:local}" >/dev/null 2>&1; then
  echo "==> native image SBOM"
  # syft needs access to the host docker daemon for local images
  docker run --rm \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v "${root}/${OUT}:/out" \
    "${SYFT_IMAGE}" \
    "docker:${NATIVE_IMAGE:-snap-interop-native:local}" \
    -o "spdx-json=/out/native.spdx.json" \
    -o "cyclonedx-json=/out/native.cdx.json"
else
  echo "skip native image SBOM (image not present; build with make native)"
fi

if docker image inspect "${PYTHON_IMAGE:-snap-interop-python:local}" >/dev/null 2>&1; then
  echo "==> python image SBOM"
  docker run --rm \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v "${root}/${OUT}:/out" \
    "${SYFT_IMAGE}" \
    "docker:${PYTHON_IMAGE:-snap-interop-python:local}" \
    -o "spdx-json=/out/python.spdx.json" \
    -o "cyclonedx-json=/out/python.cdx.json"
else
  echo "skip python image SBOM (image not present; build with make python)"
fi

echo "==> copying version manifest"
cp versions.yaml "${OUT}/versions.yaml"

echo "SBOMs written under ${OUT}/"
ls -la "${OUT}"
