# go-s7comm API coverage map

Target client API: [`otfabric/go-s7comm` API.md](https://github.com/otfabric/go-s7comm/blob/refactor/cotp-tp0-service/API.md)
(branch `refactor/cotp-tp0-service`).

This maps public client primitives to `snap-interop` fixtures. Fixtures gate
black-box behavior of the two servers; `go-s7comm` remains a downstream consumer (M7).

## Read/write primitives (M5 focus)

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
| Rejected write (read-only) | DB | `read-only-reject` | covered (reject as `address-out-of-range`; see compatibility.md) |
| Large read / PDU chunking | DB | `large-read` | covered (within negotiated PDU) |
| Concurrent readers | DB | `concurrent-reads` | covered |
| Reconnect + preserved state | DB | `reconnect-state` | covered |

## Deferred / not fixture-gated in M5

| Client API | Reason |
| --- | --- |
| `ProbeReadableRanges` | Client-side scan over ReadArea; servers covered by OOR + span fixtures |
| `CompareRead` | Client multi-session orchestration |
| `Discover` / `ProbeRackSlots` | Network topology; not server memory contract |
| `Identify` / `GetCPUState` / SZL / diag | Characterize both servers before freezing (PLAN §5) |
| `ListBlocks` / `UploadBlock` | Outside initial memory interop scope |
| Timers / counters (`AreaTimer` / `AreaCounter`) | Deferred until equivalent semantics confirmed |

## Minimum acceptance

M5 exit: ≥10 fixtures pass on both implementations, and every row in the
read/write primitives table above has a matching fixture.
