# snap-interop requirements

## 1. Purpose

This document defines the normative requirements for `otfabric/snap-interop`.

The keywords **MUST**, **MUST NOT**, **SHOULD**, **SHOULD NOT** and **MAY** are normative.

## 2. Product goals

The repository MUST:

1. provide Dockerized native Snap7 and pure-Python S7 servers;
2. build both images from repository-controlled Dockerfiles and adapter code;
3. initialize both servers from one implementation-neutral fixture contract;
4. validate fixtures with a committed JSON Schema;
5. expose equivalent externally observable behavior for shared capabilities;
6. support deterministic local and CI execution;
7. remain independent of `go-s7comm`;
8. generate machine-readable conformance reports;
9. pin and expose all important upstream versions.

## 3. Architectural requirements

### 3.1 Fixture authority

The fixture MUST be the authoritative definition of server state and expected behavior.

Server adapters MUST NOT:

- introduce C- or Python-specific fixture fields;
- independently interpret typed values;
- silently ignore mandatory fixture settings;
- execute code or expressions embedded in fixtures.

### 3.2 Canonical compilation

Source fixtures MUST be compiled into a canonical representation before server startup.

The compiler MUST:

1. validate the JSON document against JSON Schema;
2. perform semantic validation;
3. normalize defaults;
4. encode typed values into bytes;
5. materialize complete memory-area byte arrays;
6. calculate deterministic source and output digests;
7. emit byte-stable canonical JSON.

Identical source input, schema and compiler versions MUST produce byte-identical compiled output.

### 3.3 Independent servers

The native implementation MUST use the native Snap7 server API.

The Python implementation MUST use the pure-Python server implementation and MUST NOT load or wrap the native Snap7 shared library.

The adapters SHOULD share only repository-defined fixture and reporting contracts, not protocol implementation code.

### 3.4 Black-box verification

Network-visible S7 behavior MUST be the primary basis for pass/fail decisions.

Out-of-band state snapshots MAY aid diagnosis but MUST NOT replace protocol probes.

The repository probe SHOULD be independent from `go-s7comm`. `go-s7comm` MAY later run an additional downstream test matrix.

## 4. Fixture schema

### 4.1 Format

Fixtures MUST:

- use UTF-8 JSON;
- validate against JSON Schema Draft 2020-12;
- reject unknown fields by default;
- include `schemaVersion`, `id`, `description`, `server` and `areas`;
- use stable kebab-case fixture IDs.

Fixture IDs MUST match:

```regex
^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$
```

A fixture SHOULD also support:

- `revision`;
- `tags`;
- `requires`;
- `references`;
- `notes`.

### 4.2 Server settings

The schema MUST support:

- rack;
- slot;
- requested or advertised PDU size where available;
- CPU state where available;
- server identity fields where available.

Listener address and port MUST be runtime-overridable.

Unsupported mandatory settings MUST fail validation or startup. Unsupported optional settings MUST produce an explicit `skip-unsupported` result.

### 4.3 Memory areas

Schema version 1 MUST define these logical area types:

- `db`;
- `markers`;
- `inputs`;
- `outputs`.

The design MUST reserve future support for:

- `timers`;
- `counters`.

Every area MUST define:

- type;
- size in bytes;
- access policy;
- initial content or implicit zero initialization.

DB areas MUST define a DB number. Non-DB areas MUST NOT define one.

The validator MUST reject:

- negative sizes and offsets;
- duplicate logical areas;
- missing DB numbers;
- DB numbers on non-DB areas;
- writes beyond the declared area;
- overlapping initializers unless explicit replacement is supported;
- values outside the declared type range;
- allocations above configured safety limits.

### 4.4 Area initialization

The fixture format MUST support:

- implicit zero initialization;
- hexadecimal bytes;
- base64 bytes;
- byte arrays;
- typed values.

Initialization order MUST be deterministic:

1. zero-fill the complete area;
2. apply at most one bulk initializer;
3. apply byte or typed patches in declaration order;
4. reject overlaps unless replacement was explicitly requested.

Hex strings MUST contain an even number of valid hexadecimal characters.

### 4.5 Typed values

Schema version 1 SHOULD support:

- bit/boolean;
- signed and unsigned 8-, 16-, 32- and 64-bit integers;
- IEEE-754 `float32` and `float64`;
- fixed-length bytes;
- fixed-length ASCII strings.

Multi-byte values MUST declare byte order. S7-oriented fixtures SHOULD normally use big-endian byte order.

Bits MUST declare byte offset and bit index.

Large 64-bit integer source values SHOULD be represented as JSON strings to prevent parser precision loss.

Siemens-specific string encodings MUST NOT be added until their exact byte layout is separately specified and covered by golden tests.

### 4.6 Expectations

Fixtures SHOULD define expected operations:

- read;
- write;
- rejected read;
- rejected write;
- final state;
- connection/reconnection.

Each operation MUST identify:

- logical area;
- DB number where applicable;
- byte or bit offset;
- length or payload;
- expected success or normalized error category.

Expected bytes are authoritative. Typed annotations are optional readability aids.

### 4.7 Capabilities

Each adapter MUST publish a machine-readable capability manifest.

Fixtures MAY declare requirements such as:

```json
{
  "requires": [
    "area.db",
    "operation.read",
    "operation.write"
  ]
}
```

The runner MUST classify every fixture as one of:

- `pass`;
- `fail`;
- `skip-unsupported`;
- `invalid`;
- `infrastructure-error`.

A missing capability MUST NOT be reported as a pass.

## 5. Compiled fixture contract

A compiled fixture MUST contain:

- format and format version;
- source fixture ID and revision;
- schema and compiler versions;
- source digest;
- compiled digest;
- normalized server settings;
- complete byte content for every area;
- normalized expectations;
- required capabilities.

The compiled form MUST contain no executable hooks and no language-specific objects.

Both server adapters MUST consume the same compiled artifact.

## 6. Native server

The native image MUST:

- build Snap7 from a pinned immutable source revision;
- compile repository-owned C or C++ adapter code;
- use a multi-stage Docker build;
- include only required runtime artifacts;
- keep registered backing buffers valid for the server lifetime;
- validate all sizes before passing them to upstream APIs;
- emit structured lifecycle and access logs;
- write an atomic state snapshot;
- handle startup failure and `SIGTERM` cleanly;
- provide sanitizer-enabled CI builds.

The adapter MUST document any Snap7 limitation that prevents exact fixture behavior.

## 7. Python server

The Python image MUST:

- install a pinned `python-snap7` release and locked dependencies;
- use the pure-Python server implementation;
- contain repository-owned adapter code;
- isolate upstream API usage behind a small internal adapter;
- emit the same repository-defined lifecycle events;
- write the same state snapshot structure;
- handle startup failure and `SIGTERM` cleanly;
- pass linting, static typing and unit tests;
- verify that no native Snap7 shared library is loaded.

## 8. Container contract

Both images MUST accept:

```text
SNAP_INTEROP_FIXTURE
SNAP_INTEROP_LISTEN_ADDRESS
SNAP_INTEROP_PORT
SNAP_INTEROP_LOG_FORMAT
SNAP_INTEROP_STATE_PATH
```

Both images MUST:

- expose container TCP port 102;
- support host port remapping;
- validate fixtures before readiness;
- become healthy only after accepting S7 connections;
- log to stdout/stderr;
- terminate within a bounded grace period;
- write only to declared runtime locations;
- run with a read-only root filesystem where practical;
- drop unnecessary Linux capabilities;
- avoid a permanently privileged process where practical.

`compose.yaml` MUST allow both servers to run concurrently.

## 9. Logging and state

### 9.1 Logs

JSON logs MUST include:

- timestamp;
- level;
- implementation;
- event;
- fixture ID;
- adapter version;
- relevant structured fields.

Required lifecycle events:

- `fixture_loaded`;
- `server_starting`;
- `server_ready`;
- `client_connected`;
- `client_disconnected`;
- `server_stopping`;
- `server_stopped`;
- `fatal_error`.

Access events SHOULD include operation, area, DB number, offset, length and outcome.

### 9.2 State snapshot

The atomic state snapshot MUST contain:

- fixture identity and digest;
- implementation identity;
- server status;
- current bytes for every area;
- accepted write count;
- rejected access count;
- last update time.

This state is diagnostic instrumentation and MUST NOT affect S7 behavior.

## 10. Probe and report requirements

The interoperability runner MUST:

1. start or connect to both servers;
2. wait for bounded readiness;
3. execute fixture operations;
4. capture raw and normalized results;
5. compare returned bytes;
6. compare accepted/rejected semantics;
7. inspect expected final state;
8. generate JSON, Markdown and JUnit reports;
9. retain server logs and snapshots on failure;
10. tear down all test resources.

Error comparison MUST distinguish:

- strict byte equality;
- normalized semantic equality;
- implementation-specific informational differences.

The runner MUST use bounded connection, request and suite timeouts.

Retries MAY be used for startup connection establishment. Assertion failures MUST NOT be hidden by retries.

## 11. Required tests

### 11.1 Fixture tooling

Tests MUST cover:

- schema validation;
- semantic validation;
- bounds and overlap detection;
- typed encoding;
- canonical serialization;
- digest stability;
- capability matching;
- invalid input diagnostics.

### 11.2 Golden tests

Every canonical source fixture MUST have an expected compiled output.

Golden files MUST be updated only by an explicit command.

### 11.3 Adapter tests

Each adapter MUST prove:

- fixture loading;
- area registration;
- initial byte correctness;
- write mutation;
- invalid input rejection;
- readiness and health behavior;
- graceful termination.

### 11.4 Initial interop fixture matrix

The first release MUST include at least:

1. zero-filled DB;
2. non-zero byte pattern;
3. multiple DBs;
4. markers, inputs and outputs;
5. signed and unsigned integer boundaries;
6. floating-point values;
7. bit reads and writes;
8. unaligned reads;
9. last-valid-byte access;
10. out-of-range access;
11. accepted write mutation;
12. rejected write to read-only memory;
13. PDU segmentation candidate;
14. concurrent reads;
15. reconnect with preserved server state.

Timers and counters MAY be deferred until equivalent semantics are confirmed.

## 12. CI and releases

Pull-request CI MUST run:

- formatting and linting;
- schema checks;
- fixture tooling tests;
- golden tests;
- native build and sanitizer tests;
- Python lint, type checks and tests;
- Docker builds;
- health checks;
- shared interop fixtures;
- invalid-fixture tests;
- SBOM generation;
- vulnerability scanning;
- third-party notice checks.

Release CI MUST:

- publish immutable images to GHCR;
- publish exact image digests;
- attach SBOMs, checksums and provenance;
- include a dependency/version manifest;
- support `linux/amd64` initially;
- add `linux/arm64` before `v1.0.0`.

Suggested images:

```text
ghcr.io/otfabric/snap-interop-native
ghcr.io/otfabric/snap-interop-python
```

## 13. Required developer commands

The repository SHOULD provide:

```text
make help
make fmt
make lint
make test
make fixtures
make fixtures-check
make fixtures-update
make native
make python
make images
make up
make down
make probe
make interop
make report
make sbom
make scan
make ci
```

All CI targets MUST be non-interactive.

## 14. Version manifest

The repository MUST maintain a machine-readable manifest similar to:

```yaml
snap_interop: 0.1.0
fixture_schema: 1.0.0
fixture_compiler: 0.1.0
native_snap7:
  repository: SCADACS/snap7
  revision: <immutable-commit>
python_snap7:
  package: python-snap7
  version: <exact-version>
```

Any adapter or upstream version change MUST run the complete canonical fixture matrix.

## 15. Security and licensing

The project MUST:

- document that the servers are test infrastructure only;
- forbid public exposure of port 102 in examples;
- avoid secrets and credentials in fixtures;
- reject fixture path traversal;
- bound file size, nesting and memory allocation;
- pin all external dependencies;
- generate SBOMs;
- scan source dependencies and images;
- preserve Snap7 and `python-snap7` notices;
- provide `THIRD_PARTY_NOTICES.md` before the first release;
- exclude proprietary Siemens software and standards content.

Fuzzing SHOULD target fixture parsing, canonical compilation and protocol response parsing.

## 16. Documentation requirements

Before `v0.1.0`, the repository MUST document:

- fixture authoring;
- schema evolution;
- local Docker use;
- adding a fixture;
- adding or updating an adapter;
- interpreting reports;
- upstream version updates;
- downstream consumption;
- known incompatibilities.

## 17. v0.1.0 acceptance criteria

`v0.1.0` is complete when:

1. both images build from a clean checkout;
2. both consume the same compiled fixture format;
3. at least ten shared fixtures pass;
4. invalid fixtures fail deterministically;
5. reads and writes match byte-for-byte for shared capabilities;
6. JSON, Markdown and JUnit reports are generated;
7. CI executes the full required suite;
8. immutable images are published to GHCR;
9. SBOMs and third-party notices are present;
10. `go-s7comm` can consume the images without importing repository code.
