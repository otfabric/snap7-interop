# AGENTS.md — snap7-interop

## What this is

Deterministic, fixture-driven S7 servers in two independent implementations (the Snap7 C++ server and the python-snap7 server), used as black-box oracles by S7 clients such as go-s7comm. Profile: **interoperability**.

## Organisation standards

This repository follows **otfabric/engineering policy 1.1**, profile **interoperability** (`.otfabric/policy.yaml`, mode `enforce`). Where this file and the organisation standards disagree, this file wins here — say so in your report.

## Where the truth lives

| Question | Source |
|---|---|
| Container environment, lifecycle, exit codes | docs/CONTAINER_CONTRACT.md |
| Pins (Snap7 revision, python-snap7, base image digests) | versions.yaml |
| Settled design decisions | docs/adr/ |
| Fixtures | fixtures/canonical, fixtures/schema/fixture.schema.json |
| Upstream constraints and classified differences | docs/compatibility.md |
| Client API to fixture mapping | docs/go-s7comm-coverage.md |

## Hard rules

1. Nothing floats: the Snap7 revision, the python-snap7 version and base images by digest. `make notices` must pass.
2. Skips are never counted as passes; a capability a server lacks is an explicit `skip-unsupported`.
3. The python image must not load the native Snap7 library.
4. Never lower a gate, skip or delete tests to make CI pass.
5. The release tag equals `interop_version`.

## Commands

| When you change… | Run |
|---|---|
| anything | `make ci` |
| fixtures | `make fixtures-check` |
| a server | `make images smoke-native smoke-python` |
| behaviour across the two servers | `make interop` (Docker) |
| third-party components | `make notices` |

## Git and releases

Branches and PRs. Do not push to `main`, push tags or publish images unless the maintainer asks in the session.
