#!/usr/bin/env bash
# Build multi-arch server images and push them to GHCR for public consumption.
#
# Prerequisites:
#   echo "$GITHUB_TOKEN" | docker login ghcr.io -u USERNAME --password-stdin
#   (token needs write:packages; package visibility can be set to Public in GHCR)
#
# Usage:
#   make publish
#   IMAGE_TAG=0.1.0 make publish
#   PUBLISH_PLATFORMS=linux/amd64 make publish
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${root}"

DOCKER="${DOCKER:-docker}"
GHCR_OWNER="${GHCR_OWNER:-otfabric}"
NATIVE_REPO="${GHCR_NATIVE:-ghcr.io/${GHCR_OWNER}/snap-interop-native}"
PYTHON_REPO="${GHCR_PYTHON:-ghcr.io/${GHCR_OWNER}/snap-interop-python}"
PUBLISH_PLATFORMS="${PUBLISH_PLATFORMS:-linux/amd64,linux/arm64}"

if [[ -z "${IMAGE_TAG:-}" ]]; then
  if python3 -c 'import yaml' >/dev/null 2>&1; then
    IMAGE_TAG="$(python3 - <<'PY'
import yaml
with open("versions.yaml", encoding="utf-8") as f:
    print(yaml.safe_load(f)["snap_interop"])
PY
)"
  else
    IMAGE_TAG="$(sed -n 's/^snap_interop:[[:space:]]*"\([^"]*\)"/\1/p' versions.yaml | head -1)"
  fi
fi
if [[ -z "${IMAGE_TAG}" ]]; then
  echo "FAIL: could not determine IMAGE_TAG (set IMAGE_TAG=... or fix versions.yaml)" >&2
  exit 1
fi

# Normalize: accept "0.1.0" or "v0.1.0"
VERSION_TAG="${IMAGE_TAG#v}"
V_TAG="v${VERSION_TAG}"

GIT_SHA="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"

echo "Publishing snap-interop images to GHCR"
echo "  native:  ${NATIVE_REPO}:{${V_TAG},${VERSION_TAG}}"
echo "  python:  ${PYTHON_REPO}:{${V_TAG},${VERSION_TAG}}"
echo "  platforms: ${PUBLISH_PLATFORMS}"
echo "  git_sha: ${GIT_SHA}"

if ! "${DOCKER}" buildx version >/dev/null 2>&1; then
  echo "FAIL: docker buildx is required for multi-arch publish" >&2
  exit 1
fi

# Ensure a builder that can push multi-platform manifests.
if ! "${DOCKER}" buildx inspect snap-interop-publisher >/dev/null 2>&1; then
  "${DOCKER}" buildx create --name snap-interop-publisher --use >/dev/null
else
  "${DOCKER}" buildx use snap-interop-publisher >/dev/null
fi

SOURCE_URL="${IMAGE_SOURCE_URL:-https://github.com/${GHCR_OWNER}/snap7-interop}"
common_labels=(
  --label "org.opencontainers.image.source=${SOURCE_URL}"
  --label "org.opencontainers.image.revision=$(git rev-parse HEAD 2>/dev/null || true)"
  --label "org.opencontainers.image.version=${VERSION_TAG}"
  --label "org.opencontainers.image.vendor=otfabric"
)

publish_one() {
  local dockerfile="$1"
  local repo="$2"
  echo "==> ${repo}"
  "${DOCKER}" buildx build \
    --platform "${PUBLISH_PLATFORMS}" \
    -f "${dockerfile}" \
    -t "${repo}:${V_TAG}" \
    -t "${repo}:${VERSION_TAG}" \
    "${common_labels[@]}" \
    --provenance=true \
    --sbom=true \
    --push \
    .
}

publish_one servers/native/Dockerfile "${NATIVE_REPO}"
publish_one servers/python/Dockerfile "${PYTHON_REPO}"

mkdir -p dist/release
{
  echo "snap_interop: ${VERSION_TAG}"
  echo "git_sha: ${GIT_SHA}"
  echo "native_image: ${NATIVE_REPO}"
  echo "python_image: ${PYTHON_REPO}"
  echo "tags: ${V_TAG} ${VERSION_TAG}"
  echo "platforms: ${PUBLISH_PLATFORMS}"
} | tee dist/release/publish.txt

echo
echo "Published:"
echo "  docker pull ${NATIVE_REPO}:${V_TAG}"
echo "  docker pull ${PYTHON_REPO}:${V_TAG}"
echo "Pin digests in go-s7comm for immutable CI (docker buildx imagetools inspect <ref>)."
