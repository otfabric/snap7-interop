# snap7-interop-otfabric

`ghcr.io/otfabric/snap7-interop-otfabric` runs OT Fabric's own S7 server, the
`server` package of [go-s7comm](https://github.com/otfabric/go-s7comm), behind the
snap7-interop [container contract](../../docs/CONTAINER_CONTRACT.md), with the adapter
name `otfabric`. Anyone testing an S7 client can run it with the same compiled
fixtures, environment and ports as the reference images.

**It is not an independent implementation.** A result against it is at most
*tested*, never evidence for go-s7comm. The images it is compared against,
`snap7-interop-native` (Snap7) and `snap7-interop-python` (python-snap7), are.
It reports `independentEngine: false` (`version`, the state snapshot and the
`server_ready` event). It is for testing only; do not expose it to untrusted
networks. Flaws in go-s7comm releases are listed in that repository's
security advisories, and old tags stay available.

## How it is built and published (ADR-0012)

| | |
|---|---|
| Source | `main.go` here: loads a compiled fixture, registers its areas with `server.RegisterDB`/`RegisterArea`, serves it, writes the state snapshot |
| Library | the latest go-s7comm release when the job runs, never `main`; no pin in this repository |
| Workflow | **Own stack (go-s7comm)** (`.github/workflows/own-go-s7comm.yml`), manual only |
| Tests | `go test` in the build; the contract smoke test; every fixture against both reference images of the latest harness release |
| Tags | `:<go-s7comm version>` and `:latest`, from `main` only; a rebuild replaces its tag |
| Release | not in a harness release or its `images.txt` |

Nothing is committed between this repository and go-s7comm, so no release loop
can form: go-s7comm's manual **Self-interop** check uses `:latest` and logs the
digest.

## Self-test

The harness's own scripts test this image like any adapter, through
`ADAPTER_IMAGE_otfabric`: `scripts/smoke.sh otfabric` (the container-contract
smoke test) and `scripts/interop.sh <reference> otfabric` for each reference,
which runs every case of `tests/cases.tsv` with interop-probe against the
reference and this image. The reference must pass every step. A failing step of
this image passes only as a known difference listed in `tests/known-differences.tsv`,
recorded with a waiver in `results.json`.

| Fixture | Step | otfabric | References | Why |
|---|---|---|---|---|
| `read-only-reject` | 1 (write to a read-only DB) | `write-protected` (item code `0x03`) | `address-out-of-range` (`0x0A`) | Intentional: go-s7comm uses the Siemens code, "object access is not allowed"; the references' `0x0A` comes from Snap7's ResourceLess callback ([COMPATIBILITY.md](../../docs/COMPATIBILITY.md#write-to-a-read-only-area-0x03-and-0x0a)) |

## Local use

From the repository root, with Docker:

```sh
make -f own/go-s7comm/Makefile image smoke interop
```

`image` builds the latest go-s7comm release (`UPSTREAM_VERSION=vX.Y.Z` to choose);
`interop` tests against the reference images of the latest harness tag
(`VERSION=vX.Y.Z` to choose). Reports land in `reports/own/<reference>/`.

Last local run, 2026-11-06, linux/arm64, go-s7comm v0.10.0 against v0.5.0:
smoke passed; against each reference 14 pass, 1 known difference
(`read-only-reject`), 0 fail.
