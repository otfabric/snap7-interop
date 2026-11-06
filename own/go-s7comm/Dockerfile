# SPDX-License-Identifier: MIT
#
# snap7-interop-otfabric: OT Fabric's own S7 server (go-s7comm/server) behind
# the snap7-interop container contract. Not an independent implementation:
# see README.md.
#
# Build from the repository root:
#   make -f own/go-s7comm/Makefile image
#
# The build takes the go-s7comm release as UPSTREAM_VERSION (the released git
# tag, which is the Go module version) and its commit as UPSTREAM_SHA. There is
# no pin in this repository: the workflow passes the latest release. A build
# never uses main. go.mod and go.sum here are only a starting point for local
# work.

ARG BASE_REGISTRY=docker.io/library
# golang:1.27.2-trixie and debian:trixie-slim, multi-arch indexes
# (linux/amd64 and linux/arm64).
ARG GOLANG_DIGEST=sha256:e58d6f83b3416618d8bcac2b3dde1b7f7e3c4a77d25e88637f8bbae81536c48d
ARG DEBIAN_DIGEST=sha256:f6b08b09a05725f9431502d3b5714f5c4576e82c1a84fa1debe0c24a7e0c3ec4

FROM ${BASE_REGISTRY}/golang:1.27.2-trixie@${GOLANG_DIGEST} AS build

ARG UPSTREAM_VERSION
ARG UPSTREAM_SHA
ARG ADAPTER_VERSION=dev
ARG GOPROXY=https://proxy.golang.org

ENV CGO_ENABLED=0 GOPROXY=${GOPROXY} GOSUMDB=sum.golang.org GOTOOLCHAIN=local GOWORK=off
WORKDIR /src
COPY own/go-s7comm/go.mod own/go-s7comm/go.sum own/go-s7comm/*.go ./

RUN set -eu; \
    test -n "${UPSTREAM_VERSION}" || { echo "UPSTREAM_VERSION is required" >&2; exit 1; }; \
    echo "${UPSTREAM_SHA}" | grep -Eq '^[0-9a-f]{40}$'; \
    go get "github.com/otfabric/go-s7comm@${UPSTREAM_VERSION}"; \
    go mod tidy; \
    built="$(go list -m -f '{{.Version}}' github.com/otfabric/go-s7comm)"; \
    test "${built}" = "${UPSTREAM_VERSION}" || { echo "go.mod has go-s7comm ${built}, want ${UPSTREAM_VERSION}" >&2; exit 1; }; \
    go mod download && go mod verify; \
    origin="$(go mod download -json "github.com/otfabric/go-s7comm@${UPSTREAM_VERSION}" \
        | sed -nE 's/^[[:space:]]*"Hash": "([0-9a-f]{40})".*/\1/p')"; \
    if [ -n "${origin}" ] && [ "${origin}" != "${UPSTREAM_SHA}" ]; then \
        echo "go-s7comm ${UPSTREAM_VERSION} is commit ${origin}, not ${UPSTREAM_SHA}" >&2; exit 1; \
    fi

RUN set -eu; \
    test -z "$(gofmt -l .)"; \
    go vet ./...; \
    go test ./...; \
    go build -trimpath -ldflags "-s -w \
        -X main.adapterVersion=${ADAPTER_VERSION} \
        -X main.upstreamVersion=${UPSTREAM_VERSION} \
        -X main.upstreamRevision=${UPSTREAM_SHA}" \
        -o /out/snap7-interop-otfabric .; \
    mkdir -p /out/licenses/go; \
    cp "$(go env GOROOT)/LICENSE" /out/licenses/go/LICENSE; \
    go version -m /out/snap7-interop-otfabric | awk '$1 == "dep" { print $2, $3 }' > /out/licenses/BUNDLED-MODULES.txt; \
    test -s /out/licenses/BUNDLED-MODULES.txt; \
    while read -r module modversion; do \
        dir="$(go list -m -f '{{.Dir}}' "${module}")"; \
        mkdir -p "/out/licenses/${module}"; \
        for file in "${dir}"/LICENSE* "${dir}"/LICENCE* "${dir}"/COPYING* "${dir}"/NOTICE*; do \
            if [ -f "${file}" ]; then cp "${file}" "/out/licenses/${module}/"; fi; \
        done; \
        test -n "$(ls "/out/licenses/${module}")" || { echo "no licence file in ${module} ${modversion}" >&2; exit 1; }; \
    done < /out/licenses/BUNDLED-MODULES.txt

FROM ${BASE_REGISTRY}/debian:trixie-slim@${DEBIAN_DIGEST} AS runtime

ARG ADAPTER_VERSION=dev
ARG UPSTREAM_VERSION

RUN useradd --system --uid 10001 --no-create-home --shell /usr/sbin/nologin snap \
    && mkdir -p /run/snap-interop \
    && chown snap /run/snap-interop

COPY --from=build /out/snap7-interop-otfabric /usr/local/bin/snap7-interop-otfabric
COPY --from=build /out/licenses /licenses
COPY LICENSE /licenses/snap7-interop/LICENSE
COPY fixtures/compiled /fixtures

LABEL org.opencontainers.image.title="snap7-interop-otfabric" \
      org.opencontainers.image.description="OT Fabric go-s7comm server behind the snap7-interop container contract, for testing. Not an independent implementation." \
      org.opencontainers.image.source="https://github.com/otfabric/snap7-interop" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${UPSTREAM_VERSION}"

ENV SNAP_INTEROP_FIXTURE=/fixtures/basic-db.compiled.json \
    SNAP_INTEROP_LISTEN_ADDRESS=0.0.0.0 \
    SNAP_INTEROP_PORT=102 \
    SNAP_INTEROP_LOG_FORMAT=json \
    SNAP_INTEROP_STATE_PATH=/run/snap-interop/state.json

USER snap
EXPOSE 102

HEALTHCHECK --interval=2s --timeout=2s --start-period=5s --retries=15 \
  CMD bash -c 'exec 3<>/dev/tcp/127.0.0.1/$SNAP_INTEROP_PORT'

ENTRYPOINT ["/usr/local/bin/snap7-interop-otfabric"]
