# Requirements

What this repository must do and must not do.
[docs/CONTAINER_CONTRACT.md](docs/CONTAINER_CONTRACT.md) and
[docs/FIXTURES.md](docs/FIXTURES.md) are the specifications that follow from it;
[docs/adr/](docs/adr/) records how.

## Purpose

Deterministic, fixture-driven S7 servers from independent implementations (the
Snap7 C++ server and the python-snap7 server), packaged as containers that behave
the same way, so that an S7 client can be tested against more than one
implementation without owning any of them.

It is not an S7 implementation, not a PLC simulator and not a general
protocol-testing framework ([README.md](README.md#out-of-scope-and-why)).

## R1 - Repository isolation

- R1.1 This repository does not import, build, link against or depend on any
  library under test, and contains no test logic or expectation specific to one.
  The one exception is `own/` with its workflow: images of OT Fabric's own stacks,
  published for others to test against (`own/go-s7comm/README.md`). The rest of
  the repository knows them only as an adapter name in `tests/known-differences.tsv` and
  through `ADAPTER_IMAGE_<adapter>`, and they are not part of a release. CI checks
  that nothing outside `own/` uses go-s7comm.
- R1.2 A consumer depends on published images and the documented contract only.
  It does not need a checkout of this repository to run its tests.
- R1.3 No third-party code from here is copied into a consumer, and the contract
  does not shape a consumer's public API.
- R1.4 The harness's own client, `tooling/interop-probe` with `internal/s7client`,
  is independent of every client under test ([ADR 0005](docs/adr/0005-independent-repository-probe.md)).

| Responsibility | Here | Consumer |
|----------------|:----:|:--------:|
| Reference servers, adapters, Dockerfiles | yes | no |
| Fixtures, fixture compiler and validation | yes | no |
| Container contract, capability tokens | yes | no |
| Image builds and publishing, licence compliance | yes | no |
| Self-tests of the reference servers (`make smoke interop`) | yes | no |
| Client API assertions about a library | no | yes |
| Compatibility claims and release qualification of a library | no | yes |

## R2 - Reference servers

- R2.1 Each reference is an established third-party implementation, pinned
  (`versions.yaml`: Snap7 revision, python-snap7 version, base images by digest)
  and rebuilt reproducibly for linux/amd64 and linux/arm64.
- R2.2 The python image never loads the native Snap7 library.
- R2.3 Where the references differ, the difference is characterised and
  recorded in [docs/COMPATIBILITY.md](docs/COMPATIBILITY.md), never hidden in a
  fixture.

## R3 - Evidence

- R3.1 Every run writes interop-result-v1 (`reports/results.json`); only `pass`
  counts as passing, a run with no pass fails, and a known difference carries a
  waiver that fails the run once it no longer reproduces.
- R3.2 A capability a server lacks is an explicit `skip-unsupported`, never a pass.
- R3.3 Releases are immutable: the tag equals `interop_version`, and consumers
  pin image digests.
