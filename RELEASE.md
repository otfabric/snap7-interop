# snap7-interop Releases

## How a release is made

1. Rename **Unreleased** below to `## vX.Y.Z` (with `**Date:**` and `**Previous release:**`), and set `snap7_interop` in `versions.yaml` to the same version, in one change. `make lint` (and so `make ci` and CI) fails when RELEASE.md has no notes for `snap7_interop`.
2. `make ci` on the commit to release.
3. Run the `Release` workflow on `main` from the Actions tab and pick the bump (`patch` by default, `minor` or `major`) that gives `vX.Y.Z`, or push the tag `vX.Y.Z` yourself. The release workflow checks the notes of the tag first, then runs `make ci`, builds each image on a native amd64 and arm64 runner, scans it, pushes it by digest, joins the two digests into one signed multi-arch manifest list under `vX.Y.Z` and `X.Y.Z`, and creates the GitHub release with `images.txt`, `versions.yaml`, `THIRD_PARTY_NOTICES.md`, a source SBOM and `SHA256SUMS`.
4. Consumers pin the digests from the release body or `images.txt`.

The notes are the body of the GitHub release, followed by the image digests:
everything after the heading `## vX.Y.Z` up to the next line that is exactly
`---` (`scripts/release-notes.sh vX.Y.Z` prints it). This is the format of
every otfabric repository and the rule of the shared release workflow, so:
one section per version, newest first, each ending with a `---` line, and no
`---` line inside a section.

Tags are immutable. A fix is a new version.

---

## Unreleased

### Documentation

- `docs/COMPATIBILITY.md`, *Write to a read-only area*, and the README: a real CPU 1214C DC/DC/DC (S7-1200, firmware V4.2) answers a write to a write-protected DB with item code `0x03` and no header error. The capture was posted on [python-snap7#976](https://github.com/gijzelaerr/python-snap7/issues/976#issuecomment-6800464989) and is kept in go-s7comm's `testdata/captures`. The `otfabric` image's `0x03` is the code this CPU sends. An S7-300 or S7-1500 is still open (#19).

---

## v0.6.1

**Date:** 2026-11-06
**Previous release:** v0.6.0

### Summary

The read-only write difference of the otfabric image (item code `0x03` where both references send `0x0A`) is reclassified as an intentional semantic difference and documented against the Siemens codes. Images, the container contract and the fixtures are unchanged.

### Changed

- `tests/known-bugs.tsv` is now `tests/known-differences.tsv`, the name the other `*-interop` harnesses use for classified differences of a pin. Its one entry gives the Siemens STEP 7 V20 *Use of FETCH/WRITE* page as evidence and says why the references send `0x0A`. `scripts/interop.sh`, `scripts/validate-fixtures.py` and the otfabric self-test read the new name; the waiver in `results.json` is unchanged.

### Documentation

- `docs/COMPATIBILITY.md`, section *Write to a read-only area*: the codes `0x03`, `0x05` and `0x0A` in Siemens, Wireshark, PLC4X, python-snap7 and Snap7; why Snap7 sends `0x0A`; that `0x0A` also means "range limit exceeded"; and that header-level refusals (`0x81/0x04`, `0x85`, `0x83/0x04` on an S7-300) are a different case from per-item write protection.
- `README.md`, `docs/CAPABILITIES.md`, `docs/CONTAINER_CONTRACT.md`, `docs/FIXTURES.md` and `own/go-s7comm/README.md` link to it. The README's open item is now the real-CPU capture (#19), not "only if Snap7 can emit `0x03`".

### Open

- Real-CPU evidence and the clarification questions to python-snap7, Snap7 (SCADACS) and Apache PLC4X: #19.

---

## v0.6.0

**Date:** 2026-11-06
**Previous release:** v0.5.0

### Summary

The repository layout of every otfabric `*-interop` harness (`engineering/docs/own-stack-rollout.md`). Image names, the container contract and the fixtures are unchanged.

### Changed

- `servers/` is now `adapters/` (`adapters/native`, `adapters/python`); `docs/compatibility.md` is `docs/COMPATIBILITY.md`.
- `scripts/lib.sh`, `scripts/smoke.sh <adapter>` and `scripts/interop.sh <adapter-a> <adapter-b>` replace `scripts/interop-run.sh` and the Makefile's `_smoke-run`. `image_of` takes `ADAPTER_IMAGE_<adapter>`, and `VERSION=vX.Y.Z` tests the images of a release. The smoke test now also checks that a missing fixture is refused before readiness, that `server_ready` is logged, and that SIGTERM stops the server with exit code 0.
- The results writer takes each server's adapter and image. The references' independence comes from `versions.yaml`, and any other image's from what it reports (`version`). A failing step listed in `tests/known-bugs.tsv` is a `known-difference` with a waiver; an entry that no longer reproduces fails the run.
- The otfabric self-test uses these scripts; `own/go-s7comm/selftest.sh`, `verdict.py` and `known-differences.tsv` are gone. Its one known difference (read-only write, `0x03` against `0x0A`) is in `tests/known-bugs.tsv`.

### Added

- `tests/cases.tsv` (the fixtures `scripts/interop.sh` runs) and `tests/known-bugs.tsv`, checked against the fixtures by `scripts/validate-fixtures.py`.
- `scripts/check-versions.sh`: the pins of the Dockerfiles and `requirements.txt` against `versions.yaml`. Also `scripts/retry.sh`.
- Makefile: `ADAPTERS`, `PUBLISHED_ADAPTERS`, `all`, `validate`, `documents`, `smoke`, `capabilities` (`interop-probe capabilities`), `print-adapters`, `print-published-adapters`, `buildx-setup`, `buildx`, `publish-arch`, `publish-manifest`, `digests`, `clean`. The old target names still work.
- `REQUIREMENTS.md` (R1 isolation, with `own/` as the one exception), `docs/FIXTURES.md` (the fixture catalog, moved from README), `docs/CAPABILITIES.md` (the capability matrix, moved from COMPATIBILITY, with otfabric), `licenses/` (LGPL-3.0 and GPL-3.0 texts for Snap7), `.dockerignore`, and the `policy.yml` workflow.

### Breaking changes

- None for consumers. A local script or CI job that calls `scripts/interop-run.sh` or paths under `servers/` must use `scripts/interop.sh native python` and `adapters/`.

---

## v0.5.0

**Date:** 2026-11-06
**Previous release:** v0.4.2

### Summary

- **Our own stack behind the contract** (ADR-0012 in otfabric/engineering): `own/go-s7comm/` builds `ghcr.io/otfabric/snap7-interop-otfabric`, the go-s7comm `server` package behind the container contract, adapter name `otfabric`. It is not an independent implementation and not part of this release: the manual workflow **Own stack (go-s7comm)** builds it from the latest go-s7comm release, runs the smoke test and every fixture against the reference images of the latest harness release (`own/go-s7comm/selftest.sh`), and publishes `:<go-s7comm version>` and `:latest` from `main`. Known difference: it answers a write to a read-only area with `0x03` where the references send `0x0A` (`own/go-s7comm/known-differences.tsv`).
- **Contract 1.1**: adds the otfabric image, its `version` command and `independentEngine: false` in what it reports. The references are unchanged.
- **CI**: checks that nothing outside `own/go-s7comm/` depends on go-s7comm.

- **Go 1.25**: `golang.org/x/text` moves to v0.41.0 (govulncheck), which requires it; `versions.yaml` and CI follow.
- **CI and release run on native runners**: each architecture builds on its own GitHub runner (`ubuntu-latest` for amd64, `ubuntu-24.04-arm` for arm64) in parallel, with no QEMU emulation. CI now also smoke-tests, scans and runs interop on arm64. The release pushes each image and architecture by digest, then joins them into one signed multi-arch manifest list under the release tags. Both architectures are scanned for fixable CRITICAL and HIGH findings before anything is tagged.
- **Release notes come from RELEASE.md**, as in every otfabric repository: the section of the tag, up to its `---` line, is the body of the GitHub release, followed by the image digests and pin snippets. `scripts/release-notes.sh` extracts it; `make lint`, CI and the release workflow fail when the notes of the version are missing.
- **Harness fixes**: `interop-probe` applies `--timeout` per request, as documented, rather than once to the whole session. A step after a failed reconnect reports a transport failure instead of panicking. `junit.xml` counts an `invalid` fixture as a failure. `fixturectl` rejects a JSON number of exactly 2^63 (int64) or 2^64 (uint64); use the string form. `fixturectl` reports an overlap for a `bool` patch of a bit already set, or over a one-byte `uint8` patch or initializer, unless it has `replace: true`; only different bits of one byte may share it.

### Breaking changes

- None.

---

## v0.4.2

**Date:** 2026-11-01
**Previous release:** v0.4.1

### Changed

- Policy 1.2: build output in `bin/` and `release/`, and shared CI at v3.3.0.

---

## v0.4.1

**Date:** 2026-10-31
**Previous release:** v0.4.0

### Changed

- Policy 1.2: build output in `bin/` and `release/`, and shared CI at v3.3.0.

---

## v0.4.0

**Date:** 2026-10-30

**Previous release:** v0.3.0

## Summary

Interop results as the organisation's interop-result-v1.

## Changes

### Added

- **`reports/results.json`** ([interop-result-v1](schemas/interop-result-v1.schema.json)): `make interop` derives it from the fixture reports, one result per fixture with the ID `fixture.<fixture-id>`. Only `pass` counts as passing; a run with no pass fails. Existing reports keep their shape.

---

## v0.3.0

**Date:** 2026-10-19
**Previous release:** v0.2.0

## Summary

The project's artifacts now carry its name, snap7-interop. The servers, the
fixtures and the interop results are unchanged.

### Breaking changes

- **Images**: published as `ghcr.io/otfabric/snap7-interop-native` and
  `ghcr.io/otfabric/snap7-interop-python`. v0.2.0 and earlier stay available
  under the old `snap-interop-native` and `snap-interop-python` names; no new
  tags are pushed there.
- **Go module**: `github.com/otfabric/snap7-interop` (was
  `github.com/otfabric/snap-interop`). The module only holds internal packages
  and tooling, so no importer is affected.
- **Server binaries and package**: `snap7-interop-native`, `snap7-interop-python`,
  and the Python package `snap7_interop_server` (distribution
  `snap7-interop-server`). Both images keep their entrypoints, so `docker run`
  invocations need only the new image name.
- **versions.yaml**: the version key is `snap7_interop` (was `snap_interop`).

### Unchanged

- The container contract: the `SNAP_INTEROP_*` environment variables, the
  `/run/snap-interop` state directory, ports, log events and exit codes.
- The compiled fixture format string `snap-interop-compiled-fixture` and the
  fixture contents, so vendored fixtures in consumers stay valid.

## Pins

| Component | Version / pin |
| --- | --- |
| `snap7_interop` | `0.3.0` |
| Fixture schema | `1.0.0` |
| Fixture compiler | `0.1.0` |
| Native Snap7 | `f6ff90317ca5d54250f4dcd29209689a74e26d82` |
| `python-snap7` | `3.2.1` |
| Tooling | Go 1.23, Python 3.12 |

## Pull

```bash
docker pull ghcr.io/otfabric/snap7-interop-native:v0.3.0
docker pull ghcr.io/otfabric/snap7-interop-python:v0.3.0
```

---

## v0.2.0

**Date:** 2026-10-19
**Previous release:** v0.1.0

## Summary

Dependency and security refresh. The fixtures, the container contract and the
interop results are unchanged.

- **Pure-Python server**: `python-snap7` bumped from `3.0.0` to `3.2.1`. The adapter's `_read_from_memory_area` override now returns the item return code with the data, as 3.2 expects. All 15 canonical fixtures give the same results as on `3.0.0`.
- **Image security**: both runtime images apply Debian security updates at build time, which clears the fixable critical `perl-base` CVEs (CVE-2026-13221, CVE-2026-42496, CVE-2026-8376) the image scan reported. The Python image no longer ships `pip`, whose vendored `urllib3`, `msgpack` and `setuptools` carried fixable high CVEs; the server never installs packages at runtime.
- **Docs**: the project is named snap7-interop throughout, and go-s7comm links point at its `main` branch.

## Pins

| Component | Version / pin |
| --- | --- |
| `snap_interop` | `0.2.0` |
| Fixture schema | `1.0.0` |
| Fixture compiler | `0.1.0` |
| Native Snap7 | `f6ff90317ca5d54250f4dcd29209689a74e26d82` |
| `python-snap7` | `3.2.1` |
| Tooling | Go 1.23, Python 3.12 |

## Pull

```bash
docker pull ghcr.io/otfabric/snap-interop-native:v0.2.0
docker pull ghcr.io/otfabric/snap-interop-python:v0.2.0
```

Prefer digest pins downstream (`docker buildx imagetools inspect …`).

## Breaking changes

- None.

---

## v0.1.0

**Date:** 2026-07-17
**Previous release:** none

## Summary

First public release of fixture-driven dual Snap7 interoperability servers for
black-box S7 client testing. Both adapters consume the same compiled fixtures;
equivalence is gated by `make interop`. Images publish to GHCR for
`linux/amd64` and `linux/arm64` with immutable tags (no floating `latest`).

- **Native server**: Snap7 ResourceLess adapter (`ghcr.io/otfabric/snap-interop-native`), pinned SCADACS/snap7 revision, in-tree `aarch64_linux.mk` for arm64.
- **Pure-Python server**: `python-snap7==3.0.0` `InteropServer` adapter (`ghcr.io/otfabric/snap-interop-python`) with no `libsnap7`; OOR / bit / RO surface aligned to native.
- **Fixture toolchain**: Draft 2020-12 schema, semantic validation, canonical compiler (`fixturectl`), golden `fixtures/compiled/`, and invalid fixtures with stable diagnostics.
- **Canonical matrix (15)**: `basic-db`, `write-cycle`, `multiple-dbs`, `markers-io`, `typed-values`, `integer-boundaries`, `floating-point`, `bit-access`, `unaligned-range`, `last-valid-byte`, `out-of-range`, `read-only-reject`, `large-read`, `concurrent-reads`, `reconnect-state`.
- **Interop runner**: Black-box probe + reports under `reports/<fixture-id>/` (`report.json`, `report.md`, `junit.xml`).
- **Container contract**: `SNAP_INTEROP_*` env, `server_ready` readiness, health check, SIGTERM, atomic state snapshot, read-only rootfs + `cap_drop ALL` + `NET_BIND_SERVICE`.
- **Release hardening**: Digest-pinned base images, `THIRD_PARTY_NOTICES.md`, SBOM (`make sbom`), Trivy scan gates (`make scan` / release workflow), tag-triggered GHCR publish with provenance artifacts.
- **Docs**: README scope/catalog/TODOs, `docs/compatibility.md`, `docs/go-s7comm-coverage.md`, ADRs, `SECURITY.md`.

## Pins

| Component | Version / pin |
| --- | --- |
| `snap_interop` | `0.1.0` |
| Fixture schema | `1.0.0` |
| Fixture compiler | `0.1.0` |
| Native Snap7 | `f6ff90317ca5d54250f4dcd29209689a74e26d82` |
| `python-snap7` | `3.0.0` |
| Tooling | Go 1.23, Python 3.12 |

## Pull

```bash
docker pull ghcr.io/otfabric/snap-interop-native:v0.1.0
docker pull ghcr.io/otfabric/snap-interop-python:v0.1.0
```

Prefer digest pins downstream (`docker buildx imagetools inspect …`).

## Breaking changes

- None (initial release).

## Known limitations

- Read-only write rejects normalize to `address-out-of-range` (Snap7 item `0x0A`), not a distinct `write-protected` (`0x03`) — see [docs/COMPATIBILITY.md](docs/COMPATIBILITY.md) (then `docs/compatibility.md`).
- Timers / counters, Identify / SZL / CPU state / diagnostics, and block upload/download are deferred.
- Optimized S7-1200/1500 DB addressing is out of v1 scope.
- Test infrastructure only — do not expose to untrusted networks.

---
