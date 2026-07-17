# Compatibility notes

This document records upstream constraints and known behavioral differences
between the native Snap7 adapter and the pure-Python adapter.

Do not encode unresolved assumptions into the fixture schema. Characterize
behavior first, then document it here and classify differences in fixture
`requires` / capability manifests.

## Status

Native adapter (M2) and Python adapter (M3) consume the same compiled fixtures.
Cross-server equivalence gating lands in M4.

## Native Snap7

| Topic | Finding | Impact |
| --- | --- | --- |
| Pinned revision | `f6ff90317ca5d54250f4dcd29209689a74e26d82` (SCADACS/snap7) | Used by native Dockerfile |
| License | LGPL-3.0-or-later (`lgpl-3.0.txt` in upstream tree) | Dynamic link; preserve notices before release |
| Architectures | Upstream makefiles: x86_64, i386, armv6, armv7 | `linux/amd64` first; aarch64 needs custom makefile |
| Port 102 without root | Prefer `CAP_NET_BIND_SERVICE` via `setcap` / `cap_add` | Avoid permanent root in runtime image |
| Area registration | `Srv_RegisterArea` keeps caller-owned buffers for server lifetime | Adapter must not move/reallocate registered vectors |
| Access policy | Stock RegisterArea does not enforce read-only | Read-only rejection requires RW-area callback (later) |
| Out-of-range semantics | unresolved beyond stock Snap7 behavior | Normalize in probe/runner |
| Concurrent clients | Snap7 server supports multiple clients | Characterize under load in M5 |
| Timers / counters | deferred | Not in schema v1 until characterized |
| Event codes | Implementation-specific diagnostics only | Not part of interop pass/fail |

## Pure-Python (`python-snap7`)

| Topic | Finding | Impact |
| --- | --- | --- |
| Pinned release | `python-snap7==3.0.0` | Locked in `servers/python/requirements.txt` |
| Preferred server API | `snap7.server.Server` (pure Python) | Wrapped by `Snap7ServerAdapter` |
| Native library loading | Absent from wheel; build/test assert `find_library("snap7") is None` | Release-blocking if violated |
| `start_to(ip, port)` | When `port == 102`, upstream uses `self.port` | Adapter sets `Parameter.LocalPort` then starts |
| Area registration | `register_area` stores the provided `bytearray` | Keep references for state snapshots |
| Access policy | No read-only enforcement in stock API | Same deferred work as native |
| Out-of-range semantics | unresolved beyond upstream behavior | Normalize in probe/runner |
| Concurrent clients | Thread-per-client in upstream server | Characterize under load in M5 |
| Timers / counters | deferred | Not in schema v1 until characterized |

## Shared capability matrix

| Capability | Native | Python | Notes |
| --- | --- | --- | --- |
| `area.db` | yes | yes | Smoke fixture `basic-db` |
| `area.markers` | yes (register) | yes (register) | Smoke focuses on DB |
| `area.inputs` | yes (register) | yes (register) | |
| `area.outputs` | yes (register) | yes (register) | |
| `operation.read` | yes | yes | |
| `operation.write` | yes | yes | No read-only enforcement yet |
