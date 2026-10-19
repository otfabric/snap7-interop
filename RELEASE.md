# snap7-interop Releases

## Unreleased

### Summary

- (none yet)

### Breaking changes

- None.

---

## v0.3.0

**Date:** 2026-10-19
**Previous release:** v0.2.0

## Summary

The project's artifacts now carry its name, snap7-interop. The servers, the
fixtures and the interop results are unchanged.

### Breaking changes

- **Images**: published as `ghcr.io/otfabric/snap7-interop-native` and
  `ghcr.io/otfabric/snap7-interop-python`. v0.2.0 and earlier stay available
  under the old `snap-interop-native` and `snap-interop-python` names; no new
  tags are pushed there.
- **Go module**: `github.com/otfabric/snap7-interop` (was
  `github.com/otfabric/snap-interop`). The module only holds internal packages
  and tooling, so no importer is affected.
- **Server binaries and package**: `snap7-interop-native`, `snap7-interop-python`,
  and the Python package `snap7_interop_server` (distribution
  `snap7-interop-server`). Both images keep their entrypoints, so `docker run`
  invocations need only the new image name.
- **versions.yaml**: the version key is `snap7_interop` (was `snap_interop`).

### Unchanged

- The container contract: the `SNAP_INTEROP_*` environment variables, the
  `/run/snap-interop` state directory, ports, log events and exit codes.
- The compiled fixture format string `snap-interop-compiled-fixture` and the
  fixture contents, so vendored fixtures in consumers stay valid.

## Pins

| Component | Version / pin |
| --- | --- |
| `snap7_interop` | `0.3.0` |
| Fixture schema | `1.0.0` |
| Fixture compiler | `0.1.0` |
| Native Snap7 | `f6ff90317ca5d54250f4dcd29209689a74e26d82` |
| `python-snap7` | `3.2.1` |
| Tooling | Go 1.23, Python 3.12 |

## Pull

```bash
docker pull ghcr.io/otfabric/snap7-interop-native:v0.3.0
docker pull ghcr.io/otfabric/snap7-interop-python:v0.3.0
```

---

## v0.2.0

**Date:** 2026-10-19
**Previous release:** v0.1.0

## Summary

Dependency and security refresh. The fixtures, the container contract and the
interop results are unchanged.

- **Pure-Python server**: `python-snap7` bumped from `3.0.0` to `3.2.1`. The adapter's `_read_from_memory_area` override now returns the item return code with the data, as 3.2 expects. All 15 canonical fixtures give the same results as on `3.0.0`.
- **Image security**: both runtime images apply Debian security updates at build time, which clears the fixable critical `perl-base` CVEs (CVE-2026-13221, CVE-2026-42496, CVE-2026-8376) the image scan reported. The Python image no longer ships `pip`, whose vendored `urllib3`, `msgpack` and `setuptools` carried fixable high CVEs; the server never installs packages at runtime.
- **Docs**: the project is named snap7-interop throughout, and go-s7comm links point at its `main` branch.

## Pins

| Component | Version / pin |
| --- | --- |
| `snap_interop` | `0.2.0` |
| Fixture schema | `1.0.0` |
| Fixture compiler | `0.1.0` |
| Native Snap7 | `f6ff90317ca5d54250f4dcd29209689a74e26d82` |
| `python-snap7` | `3.2.1` |
| Tooling | Go 1.23, Python 3.12 |

## Pull

```bash
docker pull ghcr.io/otfabric/snap-interop-native:v0.2.0
docker pull ghcr.io/otfabric/snap-interop-python:v0.2.0
```

Prefer digest pins downstream (`docker buildx imagetools inspect …`).

## Breaking changes

- None.

---

## v0.1.0

**Date:** 2026-07-17
**Previous release:** none

## Summary

First public release of fixture-driven dual Snap7 interoperability servers for
black-box S7 client testing. Both adapters consume the same compiled fixtures;
equivalence is gated by `make interop`. Images publish to GHCR for
`linux/amd64` and `linux/arm64` with immutable tags (no floating `latest`).

- **Native server**: Snap7 ResourceLess adapter (`ghcr.io/otfabric/snap-interop-native`), pinned SCADACS/snap7 revision, in-tree `aarch64_linux.mk` for arm64.
- **Pure-Python server**: `python-snap7==3.0.0` `InteropServer` adapter (`ghcr.io/otfabric/snap-interop-python`) with no `libsnap7`; OOR / bit / RO surface aligned to native.
- **Fixture toolchain**: Draft 2020-12 schema, semantic validation, canonical compiler (`fixturectl`), golden `fixtures/compiled/`, and invalid fixtures with stable diagnostics.
- **Canonical matrix (15)**: `basic-db`, `write-cycle`, `multiple-dbs`, `markers-io`, `typed-values`, `integer-boundaries`, `floating-point`, `bit-access`, `unaligned-range`, `last-valid-byte`, `out-of-range`, `read-only-reject`, `large-read`, `concurrent-reads`, `reconnect-state`.
- **Interop runner**: Black-box probe + reports under `reports/<fixture-id>/` (`report.json`, `report.md`, `junit.xml`).
- **Container contract**: `SNAP_INTEROP_*` env, `server_ready` readiness, health check, SIGTERM, atomic state snapshot, read-only rootfs + `cap_drop ALL` + `NET_BIND_SERVICE`.
- **Release hardening**: Digest-pinned base images, `THIRD_PARTY_NOTICES.md`, SBOM (`make sbom`), Trivy scan gates (`make scan` / release workflow), tag-triggered GHCR publish with provenance artifacts.
- **Docs**: README scope/catalog/TODOs, `docs/compatibility.md`, `docs/go-s7comm-coverage.md`, ADRs, `SECURITY.md`.

## Pins

| Component | Version / pin |
| --- | --- |
| `snap_interop` | `0.1.0` |
| Fixture schema | `1.0.0` |
| Fixture compiler | `0.1.0` |
| Native Snap7 | `f6ff90317ca5d54250f4dcd29209689a74e26d82` |
| `python-snap7` | `3.0.0` |
| Tooling | Go 1.23, Python 3.12 |

## Pull

```bash
docker pull ghcr.io/otfabric/snap-interop-native:v0.1.0
docker pull ghcr.io/otfabric/snap-interop-python:v0.1.0
```

Prefer digest pins downstream (`docker buildx imagetools inspect …`).

## Breaking changes

- None (initial release).

## Known limitations

- Read-only write rejects normalize to `address-out-of-range` (Snap7 item `0x0A`), not a distinct `write-protected` (`0x03`) — see [docs/compatibility.md](docs/compatibility.md).
- Timers / counters, Identify / SZL / CPU state / diagnostics, and block upload/download are deferred.
- Optimized S7-1200/1500 DB addressing is out of v1 scope.
- Test infrastructure only — do not expose to untrusted networks.

---
