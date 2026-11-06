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

`selftest.sh` runs `interop-probe` on every compiled fixture with a reference
image in the first endpoint and this image in the second. The reference must pass
every step. A failing step of this image passes only when
`known-differences.tsv` lists the fixture, the step and the outcome it returned
(`verdict.py`).

| Fixture | Step | otfabric | References | Why |
|---|---|---|---|---|
| `read-only-reject` | 1 (write to a read-only DB) | `write-protected` (item code `0x03`) | `address-out-of-range` (`0x0A`) | go-s7comm uses the Siemens code, "accessing the object not allowed"; the references' `0x0A` comes from Snap7's ResourceLess callback ([compatibility.md](../../docs/compatibility.md)) |

## Local use

From the repository root, with Docker:

```sh
make -f own/go-s7comm/Makefile image smoke interop
```

`image` builds the latest go-s7comm release (`UPSTREAM_VERSION=vX.Y.Z` to choose);
`interop` tests against the reference images of the latest harness tag
(`VERSION=vX.Y.Z` to choose). Reports land in `reports/own/`.

Last local run, 2026-11-06, linux/arm64, go-s7comm v0.10.0 against v0.4.2:
smoke passed; 28 fixture runs passed, 2 known differences (`read-only-reject`
against each reference), 0 failed.
