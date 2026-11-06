# snap7-interop

[![CI](https://github.com/otfabric/snap7-interop/actions/workflows/ci.yml/badge.svg)](https://github.com/otfabric/snap7-interop/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Version](https://img.shields.io/badge/version-0.3.0-blue.svg)](versions.yaml)
[![Platforms](https://img.shields.io/badge/platforms-linux%2Famd64%20%7C%20linux%2Farm64-informational)](#published-images-ghcr)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](versions.yaml)
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

This repository stays client-neutral: nothing outside `own/go-s7comm/` depends on `go-s7comm` (CI checks it). See [Our own stack](#our-own-stack).

## Table of contents

- [What you can test today](#what-you-can-test-today)
- [Out of scope (and why)](#out-of-scope-and-why)
- [Open TODOs](#open-todos)
- [Core design](#core-design)
- [Fixtures](#fixtures)
- [Published images (GHCR)](#published-images-ghcr)
- [Container contract](#container-contract)
- [Developer workflow](#developer-workflow)
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
- `write-protected` (item `0x03`; the references send `0x0A` for a read-only write, so `read-only-reject` expects `address-out-of-range` — see [docs/COMPATIBILITY.md](docs/COMPATIBILITY.md#write-to-a-read-only-area-0x03-and-0x0a))
- `unsupported`, `transport-failure`, `protocol-failure`

Run the full black-box matrix locally:

```bash
make fixtures-check
make interop                 # all canonical fixtures
FIXTURE=basic-db make interop
```

Reports land under `reports/<fixture-id>/` (`report.json`, `report.md`, `junit.xml`).
`reports/results.json` is the organisation's
[interop-result-v1](schemas/interop-result-v1.schema.json), derived from them: one
result per fixture, with the stable ID `fixture.<fixture-id>`. Only `pass` counts as
passing; a run with no pass fails.

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
| Distinct wire code for “write-protected” (`0x03`) | **classified difference** | Stock Snap7 ResourceLess rejects as `0x0A`; both adapters match that surface. The otfabric image sends the Siemens `0x03` ([COMPATIBILITY.md](docs/COMPATIBILITY.md#write-to-a-read-only-area-0x03-and-0x0a)) |

Do not encode unresolved assumptions into the fixture schema. Document
differences in [docs/COMPATIBILITY.md](docs/COMPATIBILITY.md).

## Open TODOs

Potential follow-ups (not blockers for current GHCR consumption):

| Item | Notes |
| --- | --- |
| Wire `go-s7comm` CI to pinned GHCR digests | Downstream: pull both images, run the client suite, retain logs on failure |
| AddressSanitizer / UndefinedBehaviorSanitizer CI for the native adapter | Optional hardening for `adapters/native` |
| Timers / counters memory areas | Characterize both adapters, then extend schema + fixtures |
| Identify / SZL / CPU state / diagnostics | Characterize both adapters before freezing expectations |
| Block list / upload / download | Outside current memory interop contract |
| Real-CPU read-only write capture | What a physical S7-1200/S7-300/S7-1500 sends per item for a write-protected DB ([#19](https://github.com/otfabric/snap7-interop/issues/19)) |
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

## Fixtures

Fifteen canonical fixtures (DB, markers, inputs and outputs; bits, typed values,
boundaries, out-of-range and read-only rejects, concurrent readers, reconnect,
a large read) define what both servers serve and what the probe expects. The
catalog, the layout of `fixtures/`, the invalid fixtures and how to add one:
[docs/FIXTURES.md](docs/FIXTURES.md). Capability tokens:
[docs/CAPABILITIES.md](docs/CAPABILITIES.md).

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

## Our own stack

`ghcr.io/otfabric/snap7-interop-otfabric` is OT Fabric's own S7 server, the `server` package of [go-s7comm](https://github.com/otfabric/go-s7comm), behind the same container contract (adapter name `otfabric`), so you can test an S7 client against our stack with the same fixtures and commands as the references:

```bash
docker run --rm -p 3102:102 --read-only \
  --tmpfs /tmp --tmpfs /run/snap-interop:uid=10001,gid=10001,mode=0755 \
  --cap-drop ALL --cap-add NET_BIND_SERVICE \
  -e SNAP_INTEROP_FIXTURE=/fixtures/basic-db.compiled.json \
  ghcr.io/otfabric/snap7-interop-otfabric:latest
```

It is **not an independent implementation** and not part of a harness release. The manual workflow **Own stack (go-s7comm)** builds it from the latest go-s7comm release, runs the smoke test and every fixture against the reference images of the latest harness release, and publishes `:<go-s7comm version>` and `:latest` from `main`. It answers a write to a read-only area with the Siemens code `0x03` where the references send `0x0A` (an intentional, classified difference). Details: [own/go-s7comm/README.md](own/go-s7comm/README.md).

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
make ci                 # lint, tests, fixture goldens, case lists, version pins (no Docker)
make images             # build every adapter image (ADAPTERS, default: native python)
make smoke              # container-contract smoke test of each (scripts/smoke.sh <adapter>)
make interop            # every case of tests/cases.tsv on both servers (scripts/interop.sh native python)
make documents          # check reports/results.json against the schema and the run rules
make capabilities       # the capability tokens every server is run with
make candidate-python CANDIDATE_VERSION=<v>  # smoke a newer python-snap7 (pins untouched)
make candidate-native CANDIDATE_SHA=<commit>  # smoke another Snap7 commit
make publish            # multi-arch push to GHCR (publish-arch, publish-manifest, digests per architecture)
```

`scripts/` takes any image through `ADAPTER_IMAGE_<adapter>`, and the images of a
release with `VERSION=vX.Y.Z`:

```bash
VERSION=v0.6.0 scripts/smoke.sh native
VERSION=v0.6.0 ADAPTER_IMAGE_otfabric=ghcr.io/otfabric/snap7-interop-otfabric:latest scripts/interop.sh native otfabric
```

The daily Upstream workflow (`.github/workflows/upstream.yml`) compares the pins with
the latest upstream (`scripts/latest-upstream.py`), runs these candidates for pins
that are behind and keeps the issue "Upstream releases: pins behind the latest" up to date.

Architecture override:

```bash
BUILD_PLATFORM=linux/arm64 make smoke interop
```

## Relationship with go-s7comm

| Owns | Component |
| --- | --- |
| `snap7-interop` | Images, fixtures, compiler, dual-server equivalence |
| `go-s7comm` | Client API assertions, client CI matrix, digest pins; its manual Self-interop check runs its client against `snap7-interop-otfabric:latest` |

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
├── README.md / REQUIREMENTS.md / RELEASE.md / SECURITY.md / THIRD_PARTY_NOTICES.md / CONTRIBUTING.md
├── Makefile / compose.yaml / versions.yaml / licenses/
├── adapters/{native,python}/    # the reference servers, one image each
├── own/go-s7comm/               # our own stack (otfabric image), not a reference
├── fixtures/{schema,canonical,invalid,compiled}/
├── schemas/                     # interop-result-v1
├── scripts/                     # lib.sh, smoke.sh, interop.sh, results writer, checks
├── tests/{cases,known-differences}.tsv
├── tooling/{fixturectl,interop-probe}/  # fixture compiler and the harness's own S7 client (Go)
├── internal/{fixture,interop,s7client}/
├── docs/{CONTAINER_CONTRACT,FIXTURES,CAPABILITIES,COMPATIBILITY}.md, docs/adr/
├── reports/                     # generated
└── .github/workflows/           # ci, policy, release, upstream, own-go-s7comm
```

## Documentation index

| Document | Purpose |
| --- | --- |
| [README.md](README.md) | Scope, GHCR, open TODOs (this file) |
| [REQUIREMENTS.md](REQUIREMENTS.md) | What the repository must and must not do (R1 isolation) |
| [RELEASE.md](RELEASE.md) | Version changelog and release notes |
| [docs/CONTAINER_CONTRACT.md](docs/CONTAINER_CONTRACT.md) | Environment, lifecycle, log, exit codes |
| [docs/FIXTURES.md](docs/FIXTURES.md) | Fixture catalog and format, case lists |
| [docs/CAPABILITIES.md](docs/CAPABILITIES.md) | Capability tokens per adapter |
| [docs/COMPATIBILITY.md](docs/COMPATIBILITY.md) | Upstream constraints and classified differences |
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
