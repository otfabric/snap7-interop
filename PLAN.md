# snap-interop implementation plan

## Status

Last updated: 2026-07-17

| Milestone | Status | Notes |
| --- | --- | --- |
| M0 — Repository bootstrap | **complete** | `make ci` passes locally; Dockerfiles are non-runnable placeholders |
| M1 — Fixture schema and compiler | **complete** | Schema, validate, compile, digests, goldens; `make ci` green |
| M2 — Minimal native Snap7 server | **complete** | `make smoke-native` passes (DB read + write); ASan/UBSan CI deferred |
| M3 — Minimal pure-Python server | **complete** | `make smoke-python` passes; no native lib; same probe bytes as native |
| M4 — Interoperability runner | **complete** | `make interop` passes all compiled fixtures; JSON/MD/JUnit reports |
| M5 — Fixture matrix expansion | **complete** | 15 fixtures pass dual interop; go-s7comm read/write map covered |
| M6 — CI, security and release hardening | not started | |
| M7 — go-s7comm consumption | not started | Downstream |

### Current focus

Milestone 6 — CI, security and release hardening.

### Completed (M0–M3)

- [x] Bootstrap, schema/compiler, native + Python servers, smoke probes

### Completed (M4)

- [x] Normalized outcomes (`success`, `address-out-of-range`, `write-protected`, …)
- [x] Bounded readiness + fixture expectation execution on both endpoints
- [x] Byte/outcome comparison with actionable diffs
- [x] Reports: `report.json`, `report.md`, `junit.xml` under `reports/<id>/`
- [x] Failure log retention (`native.log` / `python.log`)
- [x] `make interop` (optional `FIXTURE=basic-db`) and CI `interop` job
- [x] Deliberate mismatch unit test for diagnostics

### Completed (M5)

- [x] Fixture matrix for go-s7comm ReadDB/WriteDB/ReadMerkers/ReadInputs/ReadOutputs/ReadArea/WriteArea
- [x] Bit, OOR, RO reject, concurrent-read, reconnect, large-read, typed boundaries
- [x] Probe/runner: bit ops, reconnect, concurrent-read; adapter RO/OOR/bit alignment
- [x] Compatibility notes for Snap7 ResourceLess RO reject (`0x0A`)

### Open items (follow-up)

- [ ] AddressSanitizer / UndefinedBehaviorSanitizer CI jobs for native adapter
- [ ] Timers/counters and Identify/SZL characterization (deferred from M5)

## 1. Delivery strategy

Implement `snap-interop` as a sequence of complete vertical slices.

Do not build two large server implementations first. Freeze the smallest useful fixture contract, compile it into canonical bytes, and prove one DB read against one server. Then add the second server and make equivalence a mandatory gate.

Recommended sequence:

1. repository and architectural contracts;
2. fixture schema and canonical compiler;
3. minimal native server;
4. minimal pure-Python server;
5. black-box equivalence runner;
6. fixture breadth;
7. CI and supply-chain hardening;
8. downstream `go-s7comm` integration.

## 2. Foundational decisions

### 2.1 Fixtures are the product contract

Source fixtures describe intent. Compiled fixtures define exact bytes. Server adapters only register and expose those bytes.

### 2.2 Compare bytes before abstractions

The initial success criterion is not broad PLC emulation. It is identical network-visible bytes and equivalent error behavior for a narrow supported feature set.

### 2.3 Capabilities are explicit

Differences between native Snap7 and the pure-Python server must be published as capabilities and visible skips. Never silently downgrade a fixture.

### 2.4 Keep server implementations independent

Do not share protocol code between the native and Python servers. Do not let the Python image load the native Snap7 library.

### 2.5 Keep go-s7comm downstream

The repository must be useful without `go-s7comm`. Its own probe should not rely exclusively on the client that will later be tested.

## 3. Milestones

## Milestone 0 — Repository bootstrap

Status: **complete** (2026-07-17)

### Goal

Create a reviewable, buildable repository with explicit boundaries.

### Work

- [x] Create `otfabric/snap-interop`.
- [x] Add the three bootstrap documents.
- [x] Add an OTfabric-approved license.
- [x] Add `.editorconfig`, `.gitignore`, `CODEOWNERS`, `SECURITY.md` and contribution guidance.
- [x] Create the proposed directory skeleton.
- [x] Add a root `Makefile` with working `help`, `fmt`, `lint` and `test` targets.
- [x] Add `compose.yaml` with placeholder services.
- [x] Add `versions.yaml` with unresolved pinned-version fields.
- [x] Add initial ADRs:
  - fixture-first architecture;
  - canonical byte compilation;
  - independent server implementations;
  - normalized error categories;
  - independent repository probe.
- [x] Add basic GitHub Actions validation.

### Exit criteria

- [x] Clean checkout passes `make ci`.
- [x] No external images are published.
- [x] Repository boundaries and non-goals are documented.

## Milestone 1 — Fixture schema and compiler

Status: **complete** (2026-07-17)

### Goal

Compile a human-authored DB fixture into deterministic bytes.

### Work

1. [x] Add JSON Schema Draft 2020-12.
2. [x] Implement `fixturectl validate`.
3. [x] Implement semantic validation:
   - duplicate areas;
   - invalid DB numbers;
   - bounds;
   - overlap;
   - type ranges;
   - maximum allocations.
4. [x] Implement canonical typed encoders.
5. [x] Implement complete zero-filled area materialization.
6. [x] Implement stable canonical JSON serialization.
7. [x] Add source and compiled SHA-256 digests.
8. [x] Implement `fixturectl compile`.
9. [x] Add text and JSON diagnostics with stable error codes.
10. [x] Add golden tests.
11. [x] Add initial fixtures:
    - `basic-db`;
    - `typed-values`;
    - `write-cycle`;
    - invalid bounds;
    - invalid overlap;
    - invalid type range.

### Minimal compiled shape

```json
{
  "format": "snap-interop-compiled-fixture",
  "formatVersion": "1.0.0",
  "fixture": {
    "id": "basic-db",
    "revision": 1,
    "sourceSha256": "...",
    "compiledSha256": "..."
  },
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
      "dataBase64": "..."
    }
  ],
  "expectations": []
}
```

### Exit criteria

- [x] Compilation is deterministic.
- [x] Invalid fixtures produce stable diagnostics.
- [x] Typed values are encoded only in the compiler.
- [x] Golden tests require an explicit update command.

## Milestone 2 — Minimal native Snap7 server

Status: **complete** (2026-07-17) — ASan/UBSan CI deferred

### Goal

Expose DB1 from one compiled fixture through native Snap7.

### Work

1. [x] Select and pin an immutable Snap7 revision.
2. [x] Review and record license obligations.
3. [x] Build Snap7 from source in a multi-stage Dockerfile.
4. [x] Implement a narrow C++ adapter:
   - parse compiled JSON;
   - allocate stable backing buffers;
   - register DB areas;
   - configure and start the server;
   - emit lifecycle logs;
   - write state snapshots;
   - handle signals.
5. [x] Add health checking.
6. [x] Add a minimal independent smoke probe.
7. [x] Verify exact DB reads.
8. [x] Verify write mutation.
9. [ ] Add AddressSanitizer and UndefinedBehaviorSanitizer jobs.
10. [x] Document upstream constraints.

### Technical cautions

- Registered buffers must remain allocated and unmoved throughout server lifetime.
- Validate conversions between fixture sizes and upstream integer types.
- Prefer `CAP_NET_BIND_SERVICE`, or an internal high-port option, over running permanently as root.
- Treat native event codes as diagnostics, not cross-server contract values.

### Exit criteria

- [x] Native container becomes healthy.
- [x] Probe reads exact fixture bytes.
- [x] Probe writes and observes exact changed bytes.
- [x] Normal shutdown is clean (`docker stop` / SIGTERM).
- [ ] Sanitizer shutdowns are clean (deferred with item 9).

## Milestone 3 — Minimal pure-Python server

Status: **complete** (2026-07-17)

### Goal

Expose the same compiled DB fixture through an independent pure-Python implementation.

### Work

1. [x] Pin an exact current `python-snap7` release.
2. [x] Verify the preferred pure-Python server API with a short spike.
3. [x] Lock Python dependencies.
4. [x] Implement an internal compatibility adapter around upstream calls.
5. [x] Implement compiled fixture loading and area registration.
6. [x] Implement common lifecycle logs, health and state snapshot behavior.
7. [x] Add `ruff`, static typing and `pytest`.
8. [x] Add a build-time test proving no native Snap7 library is loaded.
9. [x] Run the same read and write smoke probes.

### Exit criteria

- [x] Python container consumes the same compiled fixture.
- [x] Identical probe operations return identical bytes.
- [x] The runtime has no native Snap7 dependency.
- [x] State snapshot follows the same repository contract.

## Milestone 4 — Interoperability runner

Status: **complete** (2026-07-17)

### Goal

Make cross-server comparison a single local command and CI gate.

### Work

1. [x] Implement `interop-probe`.
2. [x] Define normalized outcomes:
   - success;
   - address-out-of-range;
   - write-protected;
   - unsupported;
   - transport-failure;
   - protocol-failure.
3. [x] Add bounded readiness handling.
4. [x] Execute fixture expectations against one endpoint.
5. [x] Execute the same sequence against both endpoints.
6. [x] Compare returned bytes and normalized outcomes.
7. [x] Compare expected final state.
8. [x] Emit JSON, Markdown and JUnit reports.
9. [x] Retain logs and snapshots on failure.
10. [x] Add `make interop` and fixture selection.
11. [x] Add a deliberate mismatch test to prove diagnostics.

### Exit criteria

- [x] One command builds, starts, probes, compares, reports and tears down.
- [x] Byte mismatches show area, offset, expected bytes and actual bytes.
- [x] Unsupported capabilities are visible skips.
- [x] Unexpected divergence fails CI.

## Milestone 5 — Fixture matrix expansion

Status: **complete** (2026-07-17)

### Goal

Cover the S7 operations required by `go-s7comm`.

### Add fixtures in this order

1. [x] multiple DBs (`multiple-dbs`);
2. [x] markers (`markers-io`);
3. [x] inputs and outputs (`markers-io`);
4. [x] integer boundaries (`integer-boundaries`);
5. [x] floating-point values (`floating-point`);
6. [x] bit reads and writes (`bit-access`);
7. [x] unaligned ranges (`unaligned-range`);
8. [x] last-valid-byte access (`last-valid-byte`);
9. [x] out-of-range access (`out-of-range`);
10. [x] read-only rejection (`read-only-reject`);
11. [x] large reads within PDU (`large-read`; multi-PDU chunking remains client-side / M7);
12. [x] concurrent clients (`concurrent-reads`);
13. [x] reconnect and state preservation (`reconnect-state`);
14. [ ] timers and counters after characterization;
15. [ ] CPU state and identity where meaningful.

For every shipped fixture:

- [x] source JSON + compiled golden;
- [x] capabilities declared;
- [x] dual-adapter expectations;
- [x] known differences documented (`docs/compatibility.md`, `docs/go-s7comm-coverage.md`).

### Exit criteria

- [x] At least ten fixtures pass on both implementations (15 pass).
- [x] Every supported `go-s7comm` read/write primitive has a matching fixture.
- [x] Known differences are classified rather than hidden.

## Milestone 6 — CI, security and release hardening

### Goal

Publish trustworthy reusable images.

### Work

- Pin base image digests.
- Build `linux/amd64`.
- Add `linux/arm64`.
- Generate SPDX and CycloneDX SBOMs.
- Scan with Grype or Trivy.
- Add build provenance.
- Sign images if OTfabric release policy supports it.
- Publish immutable GHCR tags.
- Publish image digests, checksums and version manifest.
- Add `THIRD_PARTY_NOTICES.md`.
- Add scheduled upstream candidate testing without changing release pins.
- Verify read-only root filesystem and minimal capabilities.
- Add fixture parser fuzzing.

### Exit criteria

- Release artifacts are reproducible and auditable.
- Both supported architectures pass smoke tests.
- Security policy defines release-blocking severity.
- Published tags never move.

## Milestone 7 — go-s7comm consumption

### Goal

Run the same Go client tests against both released server images.

### Work

1. Pin a released image version or digest in `go-s7comm`.
2. Add a test harness that accepts endpoint and fixture metadata.
3. Start the native target and run the selected client suite.
4. Start the Python target and run the identical suite.
5. Upload server logs and reports on failure.
6. Keep client assertions inside `go-s7comm`.
7. Define a fast pull-request subset and full nightly matrix.

### Recommended PR subset

- basic DB;
- typed values;
- write cycle;
- boundary read;
- concurrent reads.

### Recommended nightly/release suite

- complete fixture matrix;
- reconnect and race cases;
- PDU segmentation;
- both architectures where practical;
- candidate upstream dependency versions.

### Exit criteria

- `go-s7comm` detects regressions against both independent implementations.
- `snap-interop` remains independently buildable and releasable.
- Failure artifacts distinguish client defects, adapter divergence and fixture errors.

## 4. Recommended first pull requests

### PR 1 — Bootstrap

Documents, skeleton, Makefile, basic CI, compose placeholders and ADRs.

### PR 2 — Schema v1 minimum

DB area, raw hex initializer, schema validation and valid/invalid examples.

### PR 3 — Canonical compiler

Zero-fill, patches, canonical output, digests and golden tests.

### PR 4 — Native vertical slice

Pinned source, DB1 registration, Docker image, readiness and one black-box read.

### PR 5 — Python vertical slice

Pinned pure-Python dependency, DB1 registration, same runtime contract and same read.

### PR 6 — Equivalence gate

Common probe, reports, CI comparison and deliberate mismatch coverage.

### PR 7 — Writes and final state

Write expectations, snapshots and read-only rejection.

### PR 8 onward — Coherent fixture groups

Add one behavior family per pull request.

## 5. Up-front technical spikes

Resolve these before freezing schema v1:

1. Which native Snap7 revision builds cleanly on amd64 and arm64?
2. Which exact pure-Python server API should the adapter target?
3. Which area types have equivalent byte semantics on both implementations?
4. How does each server represent out-of-range reads and writes?
5. Can both meaningfully expose equivalent PDU and CPU-state settings?
6. Can port 102 be bound without a root runtime process?
7. How do both servers handle concurrent clients?
8. Are write callbacks and backing-buffer mutations synchronous?
9. How should timers and counters be represented?
10. Which independent client/probe should gate this repository?

Record results in ADRs or `docs/compatibility.md`. Do not encode assumptions into the schema.

## 6. Rules for an AI coding agent

1. Read `README.md`, `REQUIREMENTS.md`, `PLAN.md` and all ADRs first.
2. Treat `REQUIREMENTS.md` as normative.
3. Keep fixture fields implementation-neutral.
4. Encode typed values only in the canonical compiler.
5. Never silently ignore unsupported behavior.
6. Add tests for every semantic change.
7. Update golden files only through the explicit target.
8. Pin every external dependency.
9. Preserve native/Python protocol independence.
10. Do not add `go-s7comm` as a server dependency.
11. Prefer a small complete vertical slice over broad scaffolding.
12. Document upstream limitations in `docs/compatibility.md`.
13. Run `make ci` before declaring a task complete.
14. Include failure evidence: logs, reports and relevant state snapshots.
15. Do not weaken a normative requirement without updating the requirement and recording an ADR.

## 7. Definition of done

A change is complete only when:

- code is formatted and linted;
- unit tests pass;
- relevant golden fixtures pass;
- both affected adapters pass shared fixtures;
- documentation is current;
- dependencies are pinned;
- diagnostics are actionable;
- CI artifacts permit failure analysis;
- no implementation-specific semantics leaked into fixtures.

## 8. Target release sequence

### v0.0.1 — Bootstrap

Repository contracts, schema skeleton and CI scaffolding.

### v0.1.0 — Minimal interoperability

Two images, canonical fixture compiler, DB reads/writes, reports and at least ten fixtures.

### v0.2.0 — Broader memory coverage

Markers, inputs, outputs, boundaries, concurrency and reconnect behavior.

### v0.3.0 — Downstream maturity

Stable `go-s7comm` consumption contract, multi-architecture images and improved diagnostics.

### v1.0.0 — Stable fixture contract

Stable schema, documented compatibility matrix, reproducible releases and proven downstream use.
