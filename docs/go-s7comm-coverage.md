# go-s7comm API coverage map

Target client API:
[`otfabric/go-s7comm` API.md](https://github.com/otfabric/go-s7comm/blob/refactor/cotp-tp0-service/API.md)
(branch `refactor/cotp-tp0-service`).

This maps public client primitives to `snap-interop` fixtures. Fixtures gate
black-box behavior of the two servers. Wiring pinned GHCR images into
`go-s7comm` CI is listed under [README open TODOs](../README.md#open-todos).

Fixture catalog: [README.md § Fixture catalog](../README.md#fixture-catalog).

## Covered read/write primitives

| Client API | Area | Fixture(s) | Status |
| --- | --- | --- | --- |
| `ReadDB` / `WriteDB` | DB | `basic-db`, `write-cycle`, `multiple-dbs` | covered |
| `ReadArea` / `WriteArea` | any | all area fixtures | covered |
| `ReadMerkers` | markers | `markers-io` | covered |
| `ReadInputs` | inputs | `markers-io` | covered |
| `ReadOutputs` | outputs | `markers-io` | covered |
| Typed encode/decode (client-side) | DB/markers | `integer-boundaries`, `floating-point`, `typed-values` | covered via compiled bytes |
| Bit-oriented access | DB | `bit-access` | covered |
| Unaligned byte ranges | DB | `unaligned-range` | covered |
| Last valid byte | DB | `last-valid-byte` | covered |
| Out-of-range read/write | DB | `out-of-range` | covered |
| Rejected write (read-only) | DB | `read-only-reject` | covered (normalized `address-out-of-range`; see [compatibility.md](compatibility.md)) |
| Large read within PDU | DB | `large-read` | covered |
| Concurrent readers | DB | `concurrent-reads` | covered |
| Reconnect + preserved state | DB | `reconnect-state` | covered |

All rows above pass `make interop` on both pinned server images.

## Not fixture-gated here (client or deferred)

| Client API / topic | Status | Reason |
| --- | --- | --- |
| `ProbeReadableRanges` | client-side | Scan over `ReadArea`; servers covered by OOR + span fixtures |
| `CompareRead` | client-side | Multi-session orchestration in the client |
| `Discover` / `ProbeRackSlots` | out of server memory scope | Network topology, not fixture memory contract |
| `Identify` / `GetCPUState` / SZL / diag | **deferred** | Characterize both servers before freezing expectations |
| `ListBlocks` / `UploadBlock` | **deferred** | Outside initial memory interop scope |
| Timers / counters (`AreaTimer` / `AreaCounter`) | **deferred** | Need equivalent semantics on both adapters |
| Multi-PDU chunking of spans larger than PDU | client-side (M7) | `large-read` is a large *single* request; chunking is go-s7comm `ReadArea` |

## Recommended go-s7comm subsets

**PR / fast gate**

- `basic-db`, `typed-values`, `write-cycle`, `last-valid-byte`, `concurrent-reads`

**Nightly / release**

- Full canonical matrix under `fixtures/canonical/`
- Prefer image digest pins from GHCR over moving tags
