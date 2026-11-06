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
| Architectures | Upstream unix makefiles lack aarch64; in-tree `adapters/native/snap7/aarch64_linux.mk` | `linux/amd64` and `linux/arm64` images |
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
| Pinned release | `python-snap7==3.2.1` | `adapters/python/requirements.txt` |
| Server API | `InteropServer` subclass of `snap7.server.Server` | Aligns OOR / bit / RO with native Snap7 |
| Native library | Absent from wheel; build/test assert no `libsnap7` | Release-blocking if violated |
| Access policy | Stock API ignores RO; adapter rejects with item `0x0A` | Matches native ResourceLess surface |
| Out-of-range | Stock pads/truncates; adapter returns item `0x05` | Matches native RegisterArea |
| Bit writes | Stock overwrites whole byte; adapter masks single bit | Matches native Snap7 |
| Concurrent clients | Thread-per-client | `concurrent-reads` |
| Timers / counters | **deferred** | Not in schema v1 |
| Identify / SZL / CPU state | Upstream-specific | **deferred** |

## Capabilities

The capability tokens and which adapter has them: [CAPABILITIES.md](CAPABILITIES.md).

## Known differences (classified)

| Topic | Native | Python | Interop handling |
| --- | --- | --- | --- |
| Distinct `write-protected` (item `0x03`) | ResourceLess callback fails as not-available (`0x0A`) | Matches native with `0x0A` | Fixture `read-only-reject` expects `address-out-of-range`; follow-up read proves bytes unchanged. Our own `otfabric` image (not a reference) sends `0x03`, reported as `write-protected`: an intentional difference, listed in `tests/known-differences.tsv` ([below](#write-to-a-read-only-area-0x03-and-0x0a)) |
| Identify / SZL / CPU state | Upstream-specific | Upstream-specific | Deferred — not in fixture matrix |
| Timers / counters | Deferred | Deferred | Not in schema v1 until characterized |

### Write to a read-only area: `0x03` and `0x0A`

The `read-only-reject` fixture records what Snap7 and python-snap7 do, not
Siemens conformance. Both references answer a write to a read-only DB with item
code `0x0A`; our `otfabric` image answers `0x03`. That is an intentional
semantic difference, not a bug on either side.

| Source | `0x03` | `0x05` | `0x0A` |
|---|---|---|---|
| Siemens, STEP 7 V20, [Use of FETCH/WRITE (S7-300, S7-400)](https://docs.tia.siemens.cloud/r/en-us/v20/editing-devices-and-networks/configure-networks/communication-via-connections/connection-types-s7-300-s7-400-s7-1500/fetch/write-mode-s7-300-s7-400/use-of-fetch/write-s7-300-s7-400), diagnostic-buffer codes | Object access is not allowed | Invalid address (syntax ID, range, type, bit number) | The object does not exist, or the range limit was exceeded |
| Wireshark [packet-s7comm.c](https://github.com/wireshark/wireshark/blob/master/epan/dissectors/packet-s7comm.c), item return values | Accessing the object not allowed | Invalid address | Object does not exist |
| Apache PLC4X S7 model, [DataTransportErrorCode](https://github.com/apache/plc4x/blob/develop/plc4go/protocols/s7/readwrite/model/DataTransportErrorCode.go) | `ACCESS_DENIED` | `INVALID_ADDRESS` | `NOT_FOUND` |
| python-snap7 3.3.1 (latest release; the python adapter pins 3.2.1) `S7_RETURN_CODES` (client) | Accessing the object not allowed | Invalid address | Object does not exist |
| Snap7 (SCADACS f6ff903) `s7_types.h` | not defined | `Code7AddressOutOfRange` | `Code7ResItemNotAvailable` |

- The Siemens table is published for the FETCH/WRITE diagnostic buffer, not
  explicitly for the S7comm Write Var item byte; its values and meanings agree
  with the other sources.
- Why the references send `0x0A`: Snap7 has no read-only areas. The native
  adapter refuses the write in its ResourceLess callback, and Snap7 turns any
  refused callback into `Code7ResItemNotAvailable` (`0x0A`). The python adapter
  copies that surface on purpose. python-snap7's own `Server` has no read-only
  areas at all.
- `0x0A` also covers "range limit exceeded", so it is not wrong in every case.
  For a valid address in an existing, readable DB that is refused only because
  it is write-protected, `0x03` is the closer match.
- A CPU can also refuse the whole job at header level: error class/code
  `0x81/0x04` or `0x85` when PUT/GET is off, `0x83/0x04` on an S7-300
  ([PLC4X, 2026-08-16](https://www.mail-archive.com/commits@plc4x.apache.org/msg24099.html)).
  Connection-level access control is a different case from per-item write
  protection; the fixtures do not normalise one into the other.
- What a physical Siemens CPU sends per item for a write-protected DB is not
  verified yet. The capture plan and the clarification questions to
  [python-snap7](https://github.com/gijzelaerr/python-snap7/issues/976),
  [Snap7 (SCADACS)](https://github.com/SCADACS/snap7/issues/26) and
  [Apache PLC4X](https://github.com/apache/plc4x/discussions/2850) are tracked in
  [otfabric/snap7-interop#19](https://github.com/otfabric/snap7-interop/issues/19).
