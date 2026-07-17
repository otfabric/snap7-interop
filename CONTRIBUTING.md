# Contributing to otfabric/snap-interop

Thank you for your interest in contributing.

## Before you change code

1. Read [README.md](README.md), [REQUIREMENTS.md](REQUIREMENTS.md),
   [PLAN.md](PLAN.md) and the ADRs under [docs/adr/](docs/adr/).
2. Treat `REQUIREMENTS.md` as normative.
3. Prefer a small complete vertical slice over broad scaffolding.
4. Keep fixture fields implementation-neutral.
5. Encode typed values only in the canonical compiler.
6. Never silently ignore unsupported behavior.
7. Pin every external dependency.
8. Preserve native/Python protocol independence.
9. Do not add `go-s7comm` as a server dependency.

## Development commands

```sh
make help
make fmt
make lint
make test
make ci
```

All CI targets are non-interactive. Run `make ci` before declaring work complete.

## Fixtures

- Source fixtures live under `fixtures/canonical/` and `fixtures/invalid/`.
- Compiled golden outputs live under `fixtures/compiled/`.
- Update golden files only through the explicit `make fixtures-update` target
  (once the compiler exists).
- Invalid fixtures must produce stable diagnostics with stable error codes.

## Pull requests

1. Open an issue or pick an existing one when the change is non-trivial.
2. Keep PRs aligned with the recommended sequence in `PLAN.md`.
3. Add or update tests for every semantic change.
4. Update `PLAN.md` status when a milestone item is completed or blocked.
5. Document upstream limitations in `docs/compatibility.md`.
6. Do not weaken a normative requirement without updating `REQUIREMENTS.md`
   and recording an ADR.

## Security

See [SECURITY.md](SECURITY.md). Do not expose port 102 publicly in examples.
