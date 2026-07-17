# snap-interop

Deterministic, fixture-driven Snap7 interoperability servers for testing S7 clients against independent implementations.

`snap-interop` provides two Dockerized S7 server targets:

- a native server built in-house from the Snap7 C/C++ sources;
- a Python server built in-house on the pure-Python `python-snap7` server;
- one shared JSON fixture format, validated by JSON Schema;
- one black-box interoperability contract applied to both servers.

The primary downstream consumer will be [`otfabric/go-s7comm`](https://github.com/otfabric/go-s7comm), but this repository must remain client-neutral.

## Core design principle

**Fixtures define observable server state. Servers are adapters.**

A fixture describes memory areas, initial bytes, access policy, server settings and expected operations. It must contain no C-specific or Python-specific behavior. Both server adapters consume the same normalized fixture and expose the same externally observable S7 behavior wherever their capabilities overlap.

```text
source fixture.json
       │
       ├── JSON Schema validation
       ├── semantic validation
       └── canonical compilation
                │
                ├── native Snap7 adapter
                └── pure-Python adapter
                         │
                         └── common black-box probes and reports
```

## Why a separate repository?

Keeping the test servers outside `go-s7comm` provides:

- implementation independence from the client under test;
- reusable fixtures across languages and clients;
- reproducible Docker-based test environments;
- explicit comparison of two unrelated server implementations;
- independent release and version pinning;
- a place to document real implementation differences instead of hiding them in client tests.

Snap7 provides a native multi-platform S7 communication suite and server API. Current `python-snap7` releases provide a pure-Python server without requiring the native Snap7 library, making the two implementations suitable independent targets. The exact upstream versions will be pinned in this repository rather than tracking floating branches.

## Initial scope

The first release targets classic S7 communication over ISO-on-TCP:

- TCP listener on container port 102;
- DB, marker, input and output memory;
- deterministic byte initialization;
- reads and writes;
- bit and scalar test values;
- boundary and invalid-access cases;
- multiple simultaneous clients;
- Docker health checks and structured logs;
- machine-readable interoperability reports;
- `linux/amd64` initially, followed by `linux/arm64`.

Timers, counters, negotiated PDU edge cases, CPU state and implementation-specific diagnostics are added only after their behavior has been characterized on both servers.

## Non-goals

This project is not intended to:

- emulate a complete Siemens PLC or scan cycle;
- execute LAD, FBD, STL or SCL programs;
- reproduce all CPU models and firmware quirks;
- support optimized S7-1200/S7-1500 data blocks in the first release;
- serve as a production PLC simulator;
- make `go-s7comm` a dependency of either server;
- force implementation-specific behavior into the fixture format.

## Proposed repository layout

```text
snap-interop/
├── README.md
├── REQUIREMENTS.md
├── PLAN.md
├── LICENSE
├── Makefile
├── compose.yaml
├── versions.yaml
├── fixtures/
│   ├── schema/
│   │   └── fixture.schema.json
│   ├── canonical/
│   ├── invalid/
│   └── compiled/
├── tooling/
│   ├── fixturectl/
│   └── interop-probe/
├── servers/
│   ├── native/
│   │   ├── Dockerfile
│   │   ├── CMakeLists.txt
│   │   └── src/
│   └── python/
│       ├── Dockerfile
│       ├── pyproject.toml
│       └── snap_interop_server/
├── tests/
│   ├── conformance/
│   ├── integration/
│   └── golden/
├── docs/
│   ├── adr/
│   └── compatibility.md
└── .github/workflows/
```

The tooling language is not part of the wire contract. Go is a practical choice for `fixturectl` and `interop-probe`, but the server containers remain C/C++ and Python.

## Fixture example

```json
{
  "$schema": "../schema/fixture.schema.json",
  "schemaVersion": "1.0.0",
  "id": "basic-read-write",
  "description": "Basic DB and marker reads and writes",
  "server": {
    "rack": 0,
    "slot": 2,
    "pduSize": 480
  },
  "areas": [
    {
      "type": "db",
      "number": 1,
      "size": 64,
      "access": "read-write",
      "initial": {
        "hex": "0102030405060708"
      }
    },
    {
      "type": "markers",
      "size": 32,
      "access": "read-write",
      "values": [
        {
          "offset": 0,
          "dataType": "uint16",
          "byteOrder": "big-endian",
          "value": 4660
        },
        {
          "offset": 2,
          "dataType": "float32",
          "byteOrder": "big-endian",
          "value": 12.5
        }
      ]
    }
  ],
  "expectations": [
    {
      "operation": "read",
      "area": { "type": "db", "number": 1 },
      "offset": 0,
      "length": 8,
      "result": { "hex": "0102030405060708" }
    },
    {
      "operation": "write",
      "area": { "type": "db", "number": 1 },
      "offset": 8,
      "data": { "hex": "deadbeef" },
      "result": "accepted"
    }
  ]
}
```

Typed values are compiled once into canonical bytes. The native and Python servers do not independently encode fixture types.

## Container contract

Both images expose the same runtime interface:

```text
SNAP_INTEROP_FIXTURE=/fixtures/basic-read-write.compiled.json
SNAP_INTEROP_LISTEN_ADDRESS=0.0.0.0
SNAP_INTEROP_PORT=102
SNAP_INTEROP_LOG_FORMAT=json
SNAP_INTEROP_STATE_PATH=/run/snap-interop/state.json
```

Both containers must:

- reject missing or invalid fixtures before readiness;
- emit `server_ready` only after accepting connections;
- expose a Docker health check;
- handle `SIGTERM` cleanly;
- write logs to stdout/stderr;
- maintain an atomic diagnostic state snapshot;
- avoid language-specific fixture semantics.

Suggested local ports:

```text
native:  localhost:1102 -> container:102
python:  localhost:2102 -> container:102
```

## Expected developer workflow

```bash
make fixtures-check
make images
make interop
```

Run a specific fixture:

```bash
FIXTURE=fixtures/canonical/basic-read-write.json make interop
```

Expected outputs:

```text
reports/basic-read-write/report.json
reports/basic-read-write/report.md
reports/basic-read-write/junit.xml
```

## Relationship with go-s7comm

`snap-interop` owns:

- deterministic server images;
- fixture validation and compilation;
- shared server behavior;
- cross-server equivalence reporting.

`go-s7comm` owns:

- Go client assertions;
- client API behavior;
- client-specific regression tests;
- the choice of which released images and fixtures gate its CI.

Recommended downstream flow:

```text
go-s7comm CI
 ├── start snap-interop-native:<pinned version>
 ├── run client tests
 ├── start snap-interop-python:<pinned version>
 ├── run identical client tests
 └── retain logs and fixture reports on failure
```

## Versioning

Version independently:

- repository releases and image tags;
- fixture schema;
- fixture compiler;
- individual fixture revisions;
- native Snap7 source revision;
- `python-snap7` package version.

Published image tags must be immutable. Downstream release tests should preferably pin image digests.

## Security and licensing

These servers are test infrastructure and must not be exposed to untrusted networks.

The repository will:

- pin upstream sources and base images;
- generate SBOMs;
- scan dependencies and images;
- preserve required third-party license notices;
- run containers with minimal privileges;
- exclude Siemens binaries, firmware, TIA assets and copyrighted standards text.

See [REQUIREMENTS.md](REQUIREMENTS.md) for normative requirements and [PLAN.md](PLAN.md) for the implementation sequence.
