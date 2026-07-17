# Multi-stage native Snap7 server. Do not publish until release hardening (M6).
# Build from repository root:
#   docker build --platform=linux/amd64 -f servers/native/Dockerfile -t snap-interop-native:local .

FROM debian:bookworm-slim AS snap7-build

ARG SNAP7_REPO=https://github.com/SCADACS/snap7.git
ARG SNAP7_REVISION=f6ff90317ca5d54250f4dcd29209689a74e26d82

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

WORKDIR /src/snap7/build/unix
RUN make -f x86_64_linux.mk all

FROM debian:bookworm-slim AS adapter-build

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
RUN cmake .. \
      -DCMAKE_BUILD_TYPE=Release \
      -DSNAP7_ROOT=/src/snap7 \
      -DSNAP7_LIB_DIR=/src/snap7/build/bin/x86_64-linux \
    && cmake --build . --config Release \
    && cmake --install . --prefix /opt/snap-interop

FROM debian:bookworm-slim AS runtime

RUN apt-get update && apt-get install -y --no-install-recommends \
      bash \
      ca-certificates \
      libcap2-bin \
      libstdc++6 \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --system --uid 10001 --create-home --home-dir /home/snap \
         --shell /usr/sbin/nologin snap

COPY --from=snap7-build /src/snap7/build/bin/x86_64-linux/libsnap7.so /usr/local/lib/libsnap7.so
COPY --from=snap7-build /src/snap7/lgpl-3.0.txt /usr/share/doc/snap7/lgpl-3.0.txt
COPY --from=adapter-build /opt/snap-interop/bin/snap-interop-native /usr/local/bin/snap-interop-native

RUN echo "/usr/local/lib" > /etc/ld.so.conf.d/snap7.conf \
    && ldconfig \
    && setcap 'cap_net_bind_service=+ep' /usr/local/bin/snap-interop-native \
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

ENTRYPOINT ["/usr/local/bin/snap-interop-native"]
