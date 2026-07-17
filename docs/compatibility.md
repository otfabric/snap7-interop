# Compatibility notes

This document records upstream constraints and known behavioral differences
between the native Snap7 adapter and the pure-Python adapter.

Do not encode unresolved assumptions into the fixture schema. Characterize
behavior first, then document it here and classify differences in fixture
`requires` / capability manifests.

## Status

No adapter implementations exist yet. Spikes from `PLAN.md` §5 should fill this
document before schema fields that depend on their answers are frozen.

## Native Snap7

| Topic | Finding | Impact |
| --- | --- | --- |
| Pinned revision | unresolved | Blocks Milestone 2 |
| License obligations | unresolved | Record in `THIRD_PARTY_NOTICES.md` before release |
| Port 102 without root | unresolved | Prefer `CAP_NET_BIND_SERVICE` or high-port option |
| Out-of-range semantics | unresolved | Normalized error categories |
| Concurrent clients | unresolved | Fixture matrix |
| Timers / counters | deferred | Not in schema v1 until characterized |

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
| `area.db` | pending | pending | First vertical slice |
| `area.markers` | pending | pending | |
| `area.inputs` | pending | pending | |
| `area.outputs` | pending | pending | |
| `operation.read` | pending | pending | |
| `operation.write` | pending | pending | |
