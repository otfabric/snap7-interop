# Container contract

Contract version **1.1** (`contract_version` in [versions.yaml](../versions.yaml)). It records what the server images do today: the independent references `ghcr.io/otfabric/snap7-interop-native` (Snap7 C++ server) and `ghcr.io/otfabric/snap7-interop-python` (python-snap7 server), and our own `ghcr.io/otfabric/snap7-interop-otfabric` (go-s7comm server, see [Our own stack](#our-own-stack-otfabric)). Unlike the other `*-interop` harnesses these images are servers only: there is no `client <op>` or `print-capabilities` command, and clients are the systems under test or `tooling/interop-probe`.

## Environment

| Variable | Default in the image | Meaning |
|---|---|---|
| `SNAP_INTEROP_FIXTURE` | `/fixtures/basic-db.compiled.json` | Compiled fixture to serve. Required by the servers. |
| `SNAP_INTEROP_LISTEN_ADDRESS` | `0.0.0.0` | Listen address. |
| `SNAP_INTEROP_PORT` | `102` | Listen port, 1..65535. |
| `SNAP_INTEROP_LOG_FORMAT` | `json` | Log format. |
| `SNAP_INTEROP_STATE_PATH` | `/run/snap-interop/state.json` | Atomic diagnostic state snapshot. |

Publish a non-privileged host port (suggested: `1102` for native, `2102` for python).

## Lifecycle

- A missing or invalid fixture is rejected before readiness.
- `server_ready` is logged only after the server accepts connections.
- `SIGTERM` stops the server cleanly (`server_stopping`, `server_stopped` in the python server).
- Both images have a Docker health check on a TCP connect to `SNAP_INTEROP_PORT`.
- Compose and the smoke tests run them with a read-only root filesystem, `cap_drop: ALL` and `NET_BIND_SERVICE`.

## Log

One JSON object per line with `timestamp`, `level`, `implementation`, `event`, `fixture_id`, `adapter_version` and event fields. Events include `server_starting`, `server_ready` and `fatal_error`.

## Exit codes

| Code | Native | Python | otfabric |
|---|---|---|---|
| 0 | clean stop | clean stop | clean stop |
| 1 | fixture or startup failure | fixture or startup failure, and invalid configuration | fixture or startup failure |
| 2 | missing fixture variable or invalid port | not used | missing fixture variable, invalid port, unknown command |

The organisation exit codes in `otfabric/engineering` `standards/interoperability.md` do not apply to this server-only contract as it stands; converging is a contract change.

## Fixtures and capabilities

Fixtures follow [`fixtures/schema/fixture.schema.json`](../fixtures/schema/fixture.schema.json). A fixture's `requires` lists capability tokens; `internal/interop` skips a fixture with a missing capability explicitly (`skip-unsupported`), never as a pass. Differences between the two servers are in [COMPATIBILITY.md](COMPATIBILITY.md).

## Our own stack (otfabric)

`ghcr.io/otfabric/snap7-interop-otfabric` runs OT Fabric's go-s7comm `server` package behind this contract, so S7 clients can test against our stack the way they test against the references. Adapter name `otfabric`. It is **not an independent implementation** of S7comm: results against it are never evidence for go-s7comm, and it is not part of a harness release ([own/go-s7comm/README.md](../own/go-s7comm/README.md)).

Added in 1.1, for this image only (the references are unchanged):

- `version` prints what the image reports about itself, one JSON object: `name` (`otfabric`), `adapterVersion`, `upstream` (`go-s7comm`), `upstreamVersion`, `upstreamRevision` and `independentEngine` (`false`).
- The state snapshot's `implementation` is that same object; the `server_ready` event carries `upstream`, `upstream_version` and `independent_engine`.
- The image holds the compiled fixtures under `/fixtures`, so it starts without a mount.

Its answers use the Siemens item return codes. One differs from the references, which send `0x0A` for a write to a read-only area: otfabric sends `0x03` (accessing the object not allowed), which interop-probe reports as `write-protected` where the `read-only-reject` fixture expects `address-out-of-range` ([COMPATIBILITY.md](COMPATIBILITY.md#write-to-a-read-only-area-0x03-and-0x0a)). That is an intentional difference, listed in `tests/known-differences.tsv`; the fixture is unchanged.

