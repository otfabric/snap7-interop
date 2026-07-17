# Compatibility notes

This document records upstream constraints and known behavioral differences
between the native Snap7 adapter and the pure-Python adapter.

Do not encode unresolved assumptions into the fixture schema. Characterize
behavior first, then document it here and classify differences in fixture
`requires` / capability manifests.

## Status

Native adapter (Milestone 2) is in progress against pinned SCADACS/snap7.
Python adapter is not started.

## Native Snap7

| Topic | Finding | Impact |
| --- | --- | --- |
| Pinned revision | `f6ff90317ca5d54250f4dcd29209689a74e26d82` (SCADACS/snap7) | Used by native Dockerfile |
| License | LGPL-3.0-or-later (`lgpl-3.0.txt` in upstream tree) | Dynamic link; preserve notices before release |
| Architectures | Upstream makefiles: x86_64, i386, armv6, armv7 | `linux/amd64` first; aarch64 needs custom makefile |
| Port 102 without root | Prefer `CAP_NET_BIND_SERVICE` via `setcap` on the adapter binary | Avoid permanent root in runtime image |
| Area registration | `Srv_RegisterArea` keeps caller-owned buffers for server lifetime | Adapter must not move/reallocate registered vectors |
| Access policy | Stock RegisterArea does not enforce read-only | Read-only rejection requires RW-area callback (later) |
| Out-of-range semantics | unresolved beyond stock Snap7 behavior | Normalize in probe/runner |
| Concurrent clients | Snap7 server supports multiple clients | Characterize under load in M5 |
| Timers / counters | deferred | Not in schema v1 until characterized |
| Event codes | Implementation-specific diagnostics only | Not part of interop pass/fail |

## Pure-Python (`python-snap7`)

| Topic | Finding | Impact |
| --- | --- | --- |
| Pinned release | unresolved | Blocks Milestone 3 |
| Preferred server API | unresolved | Internal adapter boundary |
| Native library loading | must remain absent | Build-time check required |
| Out-of-range semantics | unresolved | Normalized error categories |
| Concurrent clients | unresolved | Fixture matrix |
| Timers / counters | deferred | Not in schema v1 until characterized |

## Shared capability matrix

| Capability | Native | Python | Notes |
| --- | --- | --- | --- |
| `area.db` | yes (M2) | pending | First vertical slice |
| `area.markers` | yes (register) | pending | Smoke focuses on DB |
| `area.inputs` | yes (register) | pending | |
| `area.outputs` | yes (register) | pending | |
| `operation.read` | yes | pending | |
| `operation.write` | yes | pending | No read-only enforcement yet |
