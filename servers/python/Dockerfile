# Pure-Python snap-interop server. Must not load native Snap7.
# Build from repository root:
#   docker buildx build --platform=linux/amd64,linux/arm64 \
#     -f servers/python/Dockerfile -t snap-interop-python:local .

ARG PYTHON_DIGEST=sha256:d50fb7611f86d04a3b0471b46d7557818d88983fc3136726336b2a4c657aa30b

FROM python:3.12-slim-bookworm@${PYTHON_DIGEST} AS build

WORKDIR /src
COPY servers/python/requirements.txt servers/python/pyproject.toml servers/python/README.md ./
COPY servers/python/snap_interop_server ./snap_interop_server
COPY servers/python/tests ./tests
COPY fixtures/compiled /fixtures

RUN pip install --no-cache-dir --upgrade pip \
    && pip install --no-cache-dir -r requirements.txt \
    && pip install --no-cache-dir pytest==8.4.1 \
    && pip install --no-cache-dir . \
    && python -m pytest tests/test_no_native_lib.py \
    && python - <<'PY'
import ctypes.util, pathlib, snap7
assert ctypes.util.find_library("snap7") is None
root = pathlib.Path(snap7.__file__).resolve().parent
assert not list(root.rglob("*.so"))
assert not list(root.rglob("*.dylib"))
assert not list(root.rglob("*.dll"))
print("OK: no native Snap7 library")
PY

FROM python:3.12-slim-bookworm@${PYTHON_DIGEST} AS runtime

# Take the Debian security updates the pinned base image does not carry yet.
RUN apt-get update && apt-get upgrade -y --no-install-recommends \
    && apt-get install -y --no-install-recommends \
      bash \
      ca-certificates \
      libcap2-bin \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --system --uid 10001 --create-home --home-dir /home/snap \
         --shell /usr/sbin/nologin snap \
    && mkdir -p /run/snap-interop /fixtures \
    && chown snap:snap /run/snap-interop

WORKDIR /app
COPY --from=build /usr/local/lib/python3.12/site-packages /usr/local/lib/python3.12/site-packages
COPY --from=build /usr/local/bin/snap-interop-python /usr/local/bin/snap-interop-python
COPY servers/python/snap_interop_server /app/snap_interop_server

# The server never installs packages at runtime. Drop pip (the base image's and
# the upgraded one copied from the build stage) so its vendored libraries do not
# ship in the image.
RUN rm -rf /usr/local/lib/python3.12/site-packages/pip \
      /usr/local/lib/python3.12/site-packages/pip-*.dist-info \
      /usr/local/bin/pip /usr/local/bin/pip3 /usr/local/bin/pip3.12

# Allow binding port 102 without a permanent root process when the kernel honors file caps.
RUN setcap 'cap_net_bind_service=+ep' /usr/local/bin/python3.12 || true

ENV SNAP_INTEROP_FIXTURE=/fixtures/basic-db.compiled.json \
    SNAP_INTEROP_LISTEN_ADDRESS=0.0.0.0 \
    SNAP_INTEROP_PORT=102 \
    SNAP_INTEROP_LOG_FORMAT=json \
    SNAP_INTEROP_STATE_PATH=/run/snap-interop/state.json \
    PYTHONUNBUFFERED=1

USER snap
EXPOSE 102

HEALTHCHECK --interval=2s --timeout=2s --start-period=5s --retries=15 \
  CMD bash -c 'exec 3<>/dev/tcp/127.0.0.1/$SNAP_INTEROP_PORT'

ENTRYPOINT ["snap-interop-python"]
