# Container contract

Contract version **1.0** (`contract_version` in [versions.yaml](../versions.yaml)). It records what both server images do today: `ghcr.io/otfabric/snap7-interop-native` (Snap7 C++ server) and `ghcr.io/otfabric/snap7-interop-python` (python-snap7 server). Unlike the other `*-interop` harnesses these images are servers only: there is no `client <op>` or `print-capabilities` command, and clients are the systems under test or `tooling/interop-probe`.

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

| Code | Native | Python |
|---|---|---|
| 0 | clean stop | clean stop |
| 1 | fixture or startup failure | fixture or startup failure, and invalid configuration |
| 2 | missing fixture variable or invalid port | not used |

The organisation exit codes in `otfabric/engineering` `standards/interoperability.md` do not apply to this server-only contract as it stands; converging is a contract change.

## Fixtures and capabilities

Fixtures follow [`fixtures/schema/fixture.schema.json`](../fixtures/schema/fixture.schema.json). A fixture's `requires` lists capability tokens; `internal/interop` skips a fixture with a missing capability explicitly (`skip-unsupported`), never as a pass. Differences between the two servers are in [compatibility.md](compatibility.md).
