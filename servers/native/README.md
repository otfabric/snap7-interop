# snap-interop native server

C++ adapter around pinned SCADACS/snap7 (`versions.yaml` → `native_snap7.revision`).

Published image: `ghcr.io/otfabric/snap-interop-native`.

Scope, fixture catalog, and out-of-scope topics:
[repository README](../../README.md). Compatibility notes:
[docs/compatibility.md](../../docs/compatibility.md).

## Contract

Same runtime environment variables as the Python image:

```text
SNAP_INTEROP_FIXTURE
SNAP_INTEROP_LISTEN_ADDRESS
SNAP_INTEROP_PORT
SNAP_INTEROP_LOG_FORMAT
SNAP_INTEROP_STATE_PATH
```

Read-only fixtures use Snap7 ResourceLess RW callbacks (see compatibility notes).
Multi-arch builds use `servers/native/snap7/aarch64_linux.mk` for `linux/arm64`.

## Build

From repository root:

```bash
make native
BUILD_PLATFORM=linux/arm64 make native
make smoke-native
```
