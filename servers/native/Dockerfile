# Multi-stage native Snap7 server.
# Build from repository root (linux/amd64 or linux/arm64):
#   docker buildx build --platform=linux/amd64 -f servers/native/Dockerfile -t snap7-interop-native:local .

ARG DEBIAN_DIGEST=sha256:7b140f374b289a7c2befc338f42ebe6441b7ea838a042bbd5acbfca6ec875818

FROM debian:bookworm-slim@${DEBIAN_DIGEST} AS snap7-build

ARG SNAP7_REPO=https://github.com/SCADACS/snap7.git
ARG SNAP7_REVISION=f6ff90317ca5d54250f4dcd29209689a74e26d82
ARG TARGETARCH

RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates \
      git \
      g++ \
      make \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src
RUN git clone "${SNAP7_REPO}" snap7 \
    && cd snap7 \
    && git checkout --detach "${SNAP7_REVISION}" \
    && test "$(git rev-parse HEAD)" = "${SNAP7_REVISION}"

# In-tree aarch64 makefile (upstream ships armv6/v7 only).
COPY servers/native/snap7/aarch64_linux.mk /src/snap7/build/unix/aarch64_linux.mk

WORKDIR /src/snap7/build/unix
RUN set -eux; \
    case "${TARGETARCH}" in \
      amd64) make -f x86_64_linux.mk all ;; \
      arm64) make -f aarch64_linux.mk all ;; \
      *) echo "unsupported TARGETARCH=${TARGETARCH}" >&2; exit 1 ;; \
    esac

FROM debian:bookworm-slim@${DEBIAN_DIGEST} AS adapter-build

ARG TARGETARCH

RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates \
      cmake \
      g++ \
      make \
      nlohmann-json3-dev \
      pkg-config \
    && rm -rf /var/lib/apt/lists/*

COPY --from=snap7-build /src/snap7 /src/snap7
COPY servers/native /src/adapter

WORKDIR /src/adapter/build
RUN set -eux; \
    case "${TARGETARCH}" in \
      amd64) snap7_lib=/src/snap7/build/bin/x86_64-linux ;; \
      arm64) snap7_lib=/src/snap7/build/bin/aarch64-linux ;; \
      *) echo "unsupported TARGETARCH=${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    cmake .. \
      -DCMAKE_BUILD_TYPE=Release \
      -DSNAP7_ROOT=/src/snap7 \
      -DSNAP7_LIB_DIR="${snap7_lib}" \
    && cmake --build . --config Release \
    && cmake --install . --prefix /opt/snap7-interop

FROM debian:bookworm-slim@${DEBIAN_DIGEST} AS runtime

ARG TARGETARCH

# Take the Debian security updates the pinned base image does not carry yet.
RUN apt-get update && apt-get upgrade -y --no-install-recommends \
    && apt-get install -y --no-install-recommends \
      bash \
      ca-certificates \
      libcap2-bin \
      libstdc++6 \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --system --uid 10001 --create-home --home-dir /home/snap \
         --shell /usr/sbin/nologin snap

COPY --from=snap7-build /src/snap7/build/bin/ /opt/snap7-bin/
COPY --from=snap7-build /src/snap7/lgpl-3.0.txt /usr/share/doc/snap7/lgpl-3.0.txt
COPY --from=adapter-build /opt/snap7-interop/bin/snap7-interop-native /usr/local/bin/snap7-interop-native

RUN set -eux; \
    case "${TARGETARCH}" in \
      amd64) src=/opt/snap7-bin/x86_64-linux/libsnap7.so ;; \
      arm64) src=/opt/snap7-bin/aarch64-linux/libsnap7.so ;; \
      *) echo "unsupported TARGETARCH=${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    cp "${src}" /usr/local/lib/libsnap7.so \
    && rm -rf /opt/snap7-bin \
    && echo "/usr/local/lib" > /etc/ld.so.conf.d/snap7.conf \
    && ldconfig \
    && setcap 'cap_net_bind_service=+ep' /usr/local/bin/snap7-interop-native \
    && mkdir -p /run/snap-interop /fixtures \
    && chown snap:snap /run/snap-interop

ENV SNAP_INTEROP_FIXTURE=/fixtures/basic-db.compiled.json \
    SNAP_INTEROP_LISTEN_ADDRESS=0.0.0.0 \
    SNAP_INTEROP_PORT=102 \
    SNAP_INTEROP_LOG_FORMAT=json \
    SNAP_INTEROP_STATE_PATH=/run/snap-interop/state.json \
    LD_LIBRARY_PATH=/usr/local/lib

USER snap
EXPOSE 102

HEALTHCHECK --interval=2s --timeout=2s --start-period=5s --retries=15 \
  CMD bash -c 'exec 3<>/dev/tcp/127.0.0.1/$SNAP_INTEROP_PORT'

ENTRYPOINT ["/usr/local/bin/snap7-interop-native"]
