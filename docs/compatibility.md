# Compatibility notes

This document records upstream constraints and known behavioral differences
between the native Snap7 adapter and the pure-Python adapter.

Do not encode unresolved assumptions into the fixture schema. Characterize
behavior first, then document it here and classify differences in fixture
`requires` / capability manifests.

## Status

Both adapters consume the same compiled fixtures. Black-box equivalence is gated
by `make interop` (M4+). M5 expands the fixture matrix for go-s7comm read/write
primitives.

## Native Snap7

| Topic | Finding | Impact |
| --- | --- | --- |
| Pinned revision | `f6ff90317ca5d54250f4dcd29209689a74e26d82` (SCADACS/snap7) | Used by native Dockerfile |
| License | LGPL-3.0-or-later (`lgpl-3.0.txt` in upstream tree) | Dynamic link; preserve notices before release |
| Architectures | Upstream makefiles: x86_64, i386, armv6, armv7 | `linux/amd64` first; aarch64 needs custom makefile |
| Port 102 without root | Prefer `CAP_NET_BIND_SERVICE` via `setcap` / `cap_add` | Avoid permanent root in runtime image |
| Area registration | `Srv_RegisterArea` keeps caller-owned buffers for server lifetime | Adapter must not move/reallocate registered vectors |
| Access policy | RegisterArea ignores `access`; RO fixtures use ResourceLess RW callback | Rejected RO writes surface as item code `0x0A` (normalized `address-out-of-range`) |
| Out-of-range semantics | RegisterArea path returns item code `0x05` | Normalized `address-out-of-range` |
| Concurrent clients | Snap7 server supports multiple clients | Covered by `concurrent-reads` |
| Timers / counters | deferred | Not in schema v1 until characterized |
| Event codes | Implementation-specific diagnostics only | Not part of interop pass/fail |

## Pure-Python (`python-snap7`)

| Topic | Finding | Impact |
| --- | --- | --- |
| Pinned release | `python-snap7==3.0.0` | Locked in `servers/python/requirements.txt` |
| Preferred server API | `snap7.server.Server` subclass `InteropServer` | Aligns OOR/bit/RO with native Snap7 |
| Native library loading | Absent from wheel; build/test assert `find_library("snap7") is None` | Release-blocking if violated |
| `start_to(ip, port)` | When `port == 102`, upstream uses `self.port` | Adapter sets `Parameter.LocalPort` then starts |
| Area registration | `register_area` stores the provided `bytearray` | Keep references for state snapshots |
| Access policy | Stock API ignores RO; adapter rejects with item code `0x0A` | Matches native ResourceLess surface |
| Out-of-range semantics | Stock pads/truncates; adapter returns item code `0x05` | Matches native RegisterArea |
| Bit writes | Stock overwrites whole byte; adapter masks single bit | Matches native Snap7 |
| Concurrent clients | Thread-per-client in upstream server | Covered by `concurrent-reads` |
| Timers / counters | deferred | Not in schema v1 until characterized |

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
| `access.read-only-enforce` | yes | yes | Reject code normalized as `address-out-of-range` (Snap7 ResourceLess `0x0A`) |

## Known differences (classified)

| Topic | Native | Python | Interop handling |
| --- | --- | --- | --- |
| Distinct `write-protected` (item `0x03`) | ResourceLess callback can only fail as not-available (`0x0A`) | Can emit `0x03`, but matches native with `0x0A` | Fixture `read-only-reject` expects `address-out-of-range`; follow-up read proves bytes unchanged |
| Identify / SZL / CPU state | Upstream-specific | Upstream-specific | Deferred characterization |
| Timers / counters | Deferred | Deferred | Not in schema v1 |
