# Compatibility notes

Upstream constraints and classified differences between the native Snap7 adapter
and the pure-Python adapter.

Do not encode unresolved assumptions into the fixture schema. Characterize
behavior first, then document it here and in fixture `requires` / capabilities.

Human-facing scope summary: [README.md](../README.md).

## Status

Both adapters consume the same compiled fixtures. Black-box equivalence is gated
by `make interop`. The canonical matrix (15 fixtures) passes on pinned
`linux/amd64` and `linux/arm64` images.

## Native Snap7

| Topic | Finding | Impact |
| --- | --- | --- |
| Pinned revision | `f6ff90317ca5d54250f4dcd29209689a74e26d82` (SCADACS/snap7) | Native Dockerfile + `versions.yaml` |
| License | LGPL-3.0-or-later | Dynamic link; notices in `THIRD_PARTY_NOTICES.md` and image `/usr/share/doc/snap7/` |
| Architectures | Upstream unix makefiles lack aarch64; in-tree `servers/native/snap7/aarch64_linux.mk` | `linux/amd64` and `linux/arm64` images |
| Port 102 without root | `CAP_NET_BIND_SERVICE` via `setcap` / `cap_add` | Avoid permanent root |
| Area registration | `Srv_RegisterArea` keeps caller-owned buffers | Do not reallocate registered vectors |
| Access policy | RegisterArea ignores `access`; RO fixtures use ResourceLess RW callback | Rejected RO writes → item code `0x0A` → normalized `address-out-of-range` |
| Out-of-range | RegisterArea path → item code `0x05` | Normalized `address-out-of-range` |
| Concurrent clients | Supported | `concurrent-reads` |
| Timers / counters | **deferred** | Not in schema v1 |
| Identify / SZL / CPU state | Upstream-specific | **deferred** characterization |
| Event codes | Implementation diagnostics only | Not part of interop pass/fail |

## Pure-Python (`python-snap7`)

| Topic | Finding | Impact |
| --- | --- | --- |
| Pinned release | `python-snap7==3.0.0` | `servers/python/requirements.txt` |
| Server API | `InteropServer` subclass of `snap7.server.Server` | Aligns OOR / bit / RO with native Snap7 |
| Native library | Absent from wheel; build/test assert no `libsnap7` | Release-blocking if violated |
| Access policy | Stock API ignores RO; adapter rejects with item `0x0A` | Matches native ResourceLess surface |
| Out-of-range | Stock pads/truncates; adapter returns item `0x05` | Matches native RegisterArea |
| Bit writes | Stock overwrites whole byte; adapter masks single bit | Matches native Snap7 |
| Concurrent clients | Thread-per-client | `concurrent-reads` |
| Timers / counters | **deferred** | Not in schema v1 |
| Identify / SZL / CPU state | Upstream-specific | **deferred** |

## Shared capability matrix

| Capability | Native | Python | Notes |
| --- | --- | --- | --- |
| `area.db` | yes | yes | |
| `area.markers` | yes | yes | |
| `area.inputs` | yes | yes | |
| `area.outputs` | yes | yes | |
| `operation.read` | yes | yes | |
| `operation.write` | yes | yes | |
| `operation.bit` | yes | yes | |
| `operation.concurrent-read` | yes | yes | |
| `operation.reconnect` | yes | yes | Client reconnect; server memory preserved |
| `access.read-only-enforce` | yes | yes | Reject normalized as `address-out-of-range` |

## Known differences (classified)

| Topic | Native | Python | Interop handling |
| --- | --- | --- | --- |
| Distinct `write-protected` (item `0x03`) | ResourceLess callback fails as not-available (`0x0A`) | Matches native with `0x0A` | Fixture `read-only-reject` expects `address-out-of-range`; follow-up read proves bytes unchanged |
| Identify / SZL / CPU state | Upstream-specific | Upstream-specific | Deferred — not in fixture matrix |
| Timers / counters | Deferred | Deferred | Not in schema v1 until characterized |
