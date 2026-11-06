# Capabilities

A fixture's `requires` lists capability tokens (`fixtures/schema/fixture.schema.json`).
interop-probe runs every server with the same set, printed by `make capabilities`
(`internal/interop/capabilities.go`); a fixture that requires a token outside it is
reported as `skip-unsupported`, never as a pass. The images have no
`print-capabilities` command: this contract is server-only
([CONTAINER_CONTRACT.md](CONTAINER_CONTRACT.md)).

| Capability | native | python | otfabric | Notes |
| --- | --- | --- | --- | --- |
| `area.db` | yes | yes | yes |  |
| `area.markers` | yes | yes | yes |  |
| `area.inputs` | yes | yes | yes |  |
| `area.outputs` | yes | yes | yes |  |
| `operation.read` | yes | yes | yes |  |
| `operation.write` | yes | yes | yes |  |
| `operation.bit` | yes | yes | yes |  |
| `operation.concurrent-read` | yes | yes | yes |  |
| `operation.reconnect` | yes | yes | yes | Client reconnect; server memory preserved |
| `access.read-only-enforce` | yes | yes | yes | Reject normalised as `address-out-of-range` for native and python (item `0x0A`); otfabric sends `0x03`, `write-protected` (tests/known-bugs.tsv) |

otfabric is our own go-s7comm server (`own/go-s7comm`), not a reference. The
deferred areas and services (timers, counters, SZL, blocks) have no token: see
[COMPATIBILITY.md](COMPATIBILITY.md).
