# snap7-interop

[![CI](https://github.com/otfabric/snap7-interop/actions/workflows/ci.yml/badge.svg)](https://github.com/otfabric/snap7-interop/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Version](https://img.shields.io/badge/version-0.3.0-blue.svg)](versions.yaml)
[![Platforms](https://img.shields.io/badge/platforms-linux%2Famd64%20%7C%20linux%2Farm64-informational)](#published-images-ghcr)
[![Go](https://img.shields.io/badge/Go-1.23-00ADD8?logo=go&logoColor=white)](versions.yaml)
[![Python](https://img.shields.io/badge/Python-3.12-3776AB?logo=python&logoColor=white)](versions.yaml)

Deterministic, fixture-driven Snap7 interoperability servers for testing S7
clients against two independent implementations.

**Status:** Ready for use. Both images build for `linux/amd64` and `linux/arm64`,
the full canonical fixture matrix passes dual interop (`make interop`), and
images publish to GHCR. Remaining work is listed under [Open TODOs](#open-todos).

| Image | Registry |
| --- | --- |
| Native Snap7 server | `ghcr.io/otfabric/snap7-interop-native` |
| Pure-Python server | `ghcr.io/otfabric/snap7-interop-python` |

Releases up to v0.2.0 were published as `ghcr.io/otfabric/snap-interop-native`
and `ghcr.io/otfabric/snap-interop-python`; those tags stay available.

This repository stays client-neutral: it does not depend on `go-s7comm`.

## Table of contents

- [What you can test today](#what-you-can-test-today)
- [Out of scope (and why)](#out-of-scope-and-why)
- [Open TODOs](#open-todos)
- [Core design](#core-design)
- [Fixture catalog](#fixture-catalog)
  - [Layout](#layout)
  - [Canonical fixtures (interop matrix)](#canonical-fixtures-interop-matrix)
  - [Invalid fixtures (compiler / schema gate)](#invalid-fixtures-compiler--schema-gate)
  - [Adding a fixture](#adding-a-fixture)
- [Published images (GHCR)](#published-images-ghcr)
- [Container contract](#container-contract)
- [Developer workflow](#developer-workflow)
- [Fixture example](#fixture-example)
- [Relationship with go-s7comm](#relationship-with-go-s7comm)
- [Repository layout](#repository-layout)
- [Documentation index](#documentation-index)
- [Versioning](#versioning)
- [Security and licensing](#security-and-licensing)

## What you can test today

Both servers expose classic S7 communication over ISO-on-TCP (container port
102) and are gated by the same compiled fixtures:

| Area | Operations | How it is proven |
| --- | --- | --- |
| Data blocks (DB) | byte read/write, bit read/write | `basic-db`, `write-cycle`, `multiple-dbs`, `bit-access`, … |
| Markers (Merkers) | byte read/write | `markers-io`, `typed-values` |
| Inputs / outputs | byte read (and marker write path) | `markers-io` |
| Typed scalars | int/uint/float as compiled big-endian bytes | `integer-boundaries`, `floating-point`, `typed-values` |
| Boundaries | unaligned spans, last valid byte, out-of-range reject | `unaligned-range`, `last-valid-byte`, `out-of-range` |
| Access policy | read-only write reject (normalized outcome) | `read-only-reject` |
| Session / load | concurrent readers, reconnect with preserved memory | `concurrent-reads`, `reconnect-state` |
| PDU | large single-request read within negotiated PDU | `large-read` |

Normalized probe outcomes used in expectations:

- `success` / `accepted`
- `address-out-of-range`
- `write-protected` (schema-ready; RO reject currently maps to `address-out-of-range` — see [docs/compatibility.md](docs/compatibility.md))
- `unsupported`, `transport-failure`, `protocol-failure`

Run the full black-box matrix locally:

```bash
make fixtures-check
make interop                 # all canonical fixtures
FIXTURE=basic-db make interop
```

Reports land under `reports/<fixture-id>/` (`report.json`, `report.md`, `junit.xml`).

Client API mapping for go-s7comm is in
[docs/go-s7comm-coverage.md](docs/go-s7comm-coverage.md).

## Out of scope (and why)

| Topic | Status | Reason |
| --- | --- | --- |
| Full PLC / scan-cycle emulation | **out of scope** | Fixtures define static memory, not a CPU program |
| LAD / FBD / STL / SCL execution | **out of scope** | Not a runtime; servers only expose registered areas |
| Per-CPU / firmware quirk fidelity | **out of scope** | Goal is shared, reproducible interop—not Siemens parity |
| Optimized S7-1200/1500 DBs | **out of scope (v1)** | Classic DB addressing only until characterized |
| Production PLC simulator | **out of scope** | Test infrastructure only; see [SECURITY.md](SECURITY.md) |
| Timers / counters areas | **deferred** | Need equivalent semantics on both adapters before schema freeze |
| Identify / SZL / CPU state / diagnostics | **deferred** | Upstream-specific; characterize both servers first |
| Block list / upload / download | **deferred** | Outside memory interop contract |
| Client-only APIs (`ProbeReadableRanges`, `CompareRead`, discovery, rack/slot probe) | **not fixture-gated here** | Orchestration belongs in the client; servers covered by OOR/span/memory fixtures |
| Multi-PDU client chunking beyond negotiated PDU | **client-side** | `large-read` covers a large *single* request; go-s7comm chunks larger spans |
| Distinct wire code for “write-protected” (`0x03`) | **classified difference** | Stock Snap7 ResourceLess rejects as `0x0A`; both adapters match that surface |

Do not encode unresolved assumptions into the fixture schema. Document
differences in [docs/compatibility.md](docs/compatibility.md).

## Open TODOs

Potential follow-ups (not blockers for current GHCR consumption):

| Item | Notes |
| --- | --- |
| Wire `go-s7comm` CI to pinned GHCR digests | Downstream: pull both images, run the client suite, retain logs on failure |
| AddressSanitizer / UndefinedBehaviorSanitizer CI for the native adapter | Optional hardening for `servers/native` |
| Timers / counters memory areas | Characterize both adapters, then extend schema + fixtures |
| Identify / SZL / CPU state / diagnostics | Characterize both adapters before freezing expectations |
| Block list / upload / download | Outside current memory interop contract |
| Distinct `write-protected` item code (`0x03`) | Only if Snap7 can emit it for RO; today RO reject is `0x0A` → `address-out-of-range` |
| Optimized S7-1200/1500 DB addressing | Out of v1 scope until characterized |

When closing a TODO, update this table and any related fixture/docs.

## Core design

**Fixtures define observable server state. Servers are adapters.**

```text
source fixture (fixtures/canonical/*.json)
       │
       ├── JSON Schema + semantic validation
       └── canonical compilation → fixtures/compiled/*.compiled.json
                │
                ├── ghcr.io/otfabric/snap7-interop-native
                └── ghcr.io/otfabric/snap7-interop-python
                         │
                         └── black-box probe / make interop reports
```

- Source fixtures are implementation-neutral JSON.
- Typed values are encoded only by the compiler.
- Adapters register compiled bytes; they do not re-interpret types.
- Unsupported capabilities must surface as explicit skips, never silent downgrades.

## Fixture catalog

### Layout

| Path | Role |
| --- | --- |
| `fixtures/schema/fixture.schema.json` | Draft 2020-12 schema for source fixtures |
| `fixtures/canonical/` | Human-authored fixtures that must pass on both servers |
| `fixtures/compiled/` | Golden compiled artifacts (update only via `make fixtures-update`) |
| `fixtures/invalid/` | Fixtures that must fail validation with stable diagnostics |

### Canonical fixtures (interop matrix)

All of the following pass dual interop on the pinned native and Python images.

| ID | Description |
| --- | --- |
| `basic-db` | Smoke DB1 with a short hex prefix; primary health/smoke fixture |
| `write-cycle` | Accepted write mutates DB1 and is visible on subsequent read |
| `multiple-dbs` | Two DBs with distinct patterns (ReadDB/WriteDB) |
| `markers-io` | Markers, inputs, and outputs (ReadMerkers/ReadInputs/ReadOutputs) |
| `typed-values` | DB + markers with typed scalar patches |
| `integer-boundaries` | Signed/unsigned integer boundary bytes (big-endian) |
| `floating-point` | IEEE-754 float32/float64 patterns (big-endian) |
| `bit-access` | Bit reads and writes within a DB byte |
| `unaligned-range` | Unaligned multi-byte reads across odd offsets |
| `last-valid-byte` | Read at the last valid byte of a DB |
| `out-of-range` | Out-of-range read/write rejected as `address-out-of-range` |
| `read-only-reject` | Write to read-only DB rejected; memory unchanged on follow-up read |
| `large-read` | Large single-request DB read within negotiated PDU |
| `concurrent-reads` | Multiple simultaneous clients read identical bytes |
| `reconnect-state` | Write, reconnect client, observe preserved server memory |

Source: `fixtures/canonical/<id>.json`  
Compiled: `fixtures/compiled/<id>.compiled.json`

### Invalid fixtures (compiler / schema gate)

These must **fail** `fixturectl validate` (used by `make test`):

| File | Intent |
| --- | --- |
| `invalid-bounds.json` | Hex initializer exceeds declared area size |
| `invalid-duplicate-area.json` | Duplicate DB1 areas rejected |
| `invalid-overlap.json` | Typed patches overlap without explicit replace |
| `invalid-type-range.json` | `uint8` value outside 0..255 |

### Adding a fixture

1. Author `fixtures/canonical/<id>.json` against the schema.
2. Declare `requires` capabilities the servers must advertise.
3. Add expectations with normalized outcomes or hex payloads.
4. Run `make fixtures-update` then `make fixtures-check`.
5. Run `FIXTURE=<id> make interop` and keep both servers green.
6. Document differences in `docs/compatibility.md` if needed.
7. Update this catalog and [docs/go-s7comm-coverage.md](docs/go-s7comm-coverage.md).

## Published images (GHCR)

Tags are immutable (no floating `latest`): `v<version>` and `<version>` from
`versions.yaml` (`interop_version`, currently `0.3.0`). Changelog: [RELEASE.md](RELEASE.md).

```bash
docker pull ghcr.io/otfabric/snap7-interop-native:v0.3.0
docker pull ghcr.io/otfabric/snap7-interop-python:v0.3.0

docker run --rm -p 1102:102 --read-only \
  --tmpfs /tmp --tmpfs /run/snap-interop:uid=10001,gid=10001,mode=0755 \
  --cap-drop ALL --cap-add NET_BIND_SERVICE \
  -v "$PWD/fixtures/compiled:/fixtures:ro" \
  -e SNAP_INTEROP_FIXTURE=/fixtures/basic-db.compiled.json \
  ghcr.io/otfabric/snap7-interop-native:v0.3.0
```

Publish multi-arch images (requires `docker login ghcr.io` with `write:packages`;
set package visibility to **Public** for anonymous pulls):

```bash
make publish
# IMAGE_TAG=0.3.0 PUBLISH_PLATFORMS=linux/amd64,linux/arm64 make publish
```

Prefer digest pins in go-s7comm CI:

```bash
docker buildx imagetools inspect ghcr.io/otfabric/snap7-interop-native:v0.3.0
```

## Container contract

```text
SNAP_INTEROP_FIXTURE=/fixtures/basic-db.compiled.json
SNAP_INTEROP_LISTEN_ADDRESS=0.0.0.0
SNAP_INTEROP_PORT=102
SNAP_INTEROP_LOG_FORMAT=json
SNAP_INTEROP_STATE_PATH=/run/snap-interop/state.json
```

Both containers:

- reject missing/invalid fixtures before readiness;
- emit `server_ready` only after accepting connections;
- expose a Docker health check;
- handle `SIGTERM` cleanly;
- log to stdout/stderr;
- keep an atomic diagnostic state snapshot under `SNAP_INTEROP_STATE_PATH`;
- run with read-only rootfs + `cap_drop ALL` + `NET_BIND_SERVICE` in compose/smoke.

Suggested host ports (do not publish privileged host port 102):

```text
native:  localhost:1102 → container:102
python:  localhost:2102 → container:102
```

## Developer workflow

```bash
make help
make ci                 # lint, tests, fixture goldens, Python checks
make images             # build both local images
make smoke-native       # native read/write smoke
make smoke-python
make interop            # full dual-server matrix + reports/
make candidate-python CANDIDATE_VERSION=<v>  # smoke a newer python-snap7 (pins untouched)
make candidate-native CANDIDATE_SHA=<commit>  # smoke another Snap7 commit
make publish            # multi-arch push to GHCR
```

The daily Upstream workflow (`.github/workflows/upstream.yml`) compares the pins with
the latest upstream (`scripts/latest-upstream.py`), runs these candidates for pins
that are behind and keeps the issue "Upstream releases: pins behind the latest" up to date.

Architecture override:

```bash
BUILD_PLATFORM=linux/arm64 make smoke-native
```

## Fixture example

Minimal source shape (see `fixtures/canonical/basic-db.json` for a real fixture):

```json
{
  "$schema": "../schema/fixture.schema.json",
  "schemaVersion": "1.0.0",
  "id": "basic-db",
  "description": "Single zero-filled DB1 with a short hex prefix for smoke reads",
  "requires": ["area.db", "operation.read"],
  "server": { "rack": 0, "slot": 2, "pduSize": 480 },
  "areas": [
    {
      "type": "db",
      "number": 1,
      "size": 64,
      "access": "read-write",
      "initial": { "hex": "0102030405060708" }
    }
  ],
  "expectations": [
    {
      "operation": "read",
      "area": { "type": "db", "number": 1 },
      "offset": 0,
      "length": 8,
      "result": { "hex": "0102030405060708" }
    }
  ]
}
```

Typed values are compiled once into canonical bytes. Adapters do not encode types themselves.

## Relationship with go-s7comm

| Owns | Component |
| --- | --- |
| `snap7-interop` | Images, fixtures, compiler, dual-server equivalence |
| `go-s7comm` | Client API assertions, client CI matrix, digest pins |

```text
go-s7comm CI
├── pull ghcr.io/otfabric/snap7-interop-native@sha256:…
├── run client tests
├── pull ghcr.io/otfabric/snap7-interop-python@sha256:…
├── run identical client tests
└── retain logs / fixture reports on failure
```

## Repository layout

```text
snap7-interop/
├── README.md / RELEASE.md / SECURITY.md / THIRD_PARTY_NOTICES.md / CONTRIBUTING.md
├── Makefile / compose.yaml / versions.yaml
├── fixtures/{schema,canonical,invalid,compiled}/
├── tooling/{fixturectl,interop-probe}/
├── servers/{native,python}/
├── internal/{fixture,interop,s7client}/
├── docs/{adr,compatibility.md,go-s7comm-coverage.md}/
└── .github/workflows/
```

## Documentation index

| Document | Purpose |
| --- | --- |
| [README.md](README.md) | Scope, fixtures, GHCR, open TODOs (this file) |
| [RELEASE.md](RELEASE.md) | Version changelog and release notes |
| [docs/compatibility.md](docs/compatibility.md) | Upstream constraints and classified differences |
| [docs/go-s7comm-coverage.md](docs/go-s7comm-coverage.md) | Client API ↔ fixture map |
| [docs/adr/](docs/adr/) | Architecture decisions |
| [SECURITY.md](SECURITY.md) | Reporting + release-blocking scan policy |
| [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) | Snap7 / python-snap7 / base image notices |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Contribution workflow |

## Versioning

Version independently:

- repository / image tags (`versions.yaml` → `interop_version`);
- fixture schema and compiler;
- individual fixture revisions;
- native Snap7 git revision;
- `python-snap7` package version.

Published tags must not move. Prefer digest pins downstream.

## Security and licensing

These servers are **test infrastructure only**. Do not expose them to untrusted
networks. Do not bind host port 102 in examples.

The repository pins base images by digest, generates SBOMs (`make sbom`), scans
images (`make scan`), and excludes Siemens proprietary assets. See
[SECURITY.md](SECURITY.md) and [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
