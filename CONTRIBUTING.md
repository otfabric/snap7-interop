# Contributing to otfabric/snap7-interop

Thank you for your interest in contributing.

## Before you change code

1. Read [README.md](README.md) (scope, fixture catalog, out-of-scope, open TODOs)
   and ADRs under [docs/adr/](docs/adr/).
2. Prefer a small complete vertical slice over broad scaffolding.
3. Keep fixture fields implementation-neutral.
4. Encode typed values only in the canonical compiler.
5. Never silently ignore unsupported behavior — use capabilities / skips.
6. Pin every external dependency (`versions.yaml`, Docker digests, Python pins).
7. Preserve native/Python protocol independence.
8. Do not add `go-s7comm` as a server dependency.

## Development commands

```sh
make help
make fmt
make lint
make test
make ci
make images
make interop
make sbom
make scan
```

All CI targets are non-interactive. Run `make ci` (and `make interop` for
server/behavior changes) before declaring work complete.

## Fixtures

- Catalog and scope: [README.md § Fixture catalog](README.md#fixture-catalog).
- Source: `fixtures/canonical/` and `fixtures/invalid/`.
- Compiled goldens: `fixtures/compiled/` — update **only** via
  `make fixtures-update`.
- After adding or changing a canonical fixture:
  1. `make fixtures-update && make fixtures-check`
  2. `FIXTURE=<id> make interop`
  3. Update the README catalog + [docs/go-s7comm-coverage.md](docs/go-s7comm-coverage.md)
  4. Record differences in [docs/compatibility.md](docs/compatibility.md)
- Invalid fixtures must produce stable diagnostics with stable error codes.

## Scope changes

If you expand coverage into a deferred area (see [Open TODOs](README.md#open-todos)):

1. Characterize **both** adapters first.
2. Document findings in `docs/compatibility.md`.
3. Extend schema / capabilities only after semantics agree.
4. Update README “What you can test”, “Out of scope”, and “Open TODOs”.

## Pull requests

1. Open an issue or pick an existing one when the change is non-trivial.
2. Add or update tests for every semantic change.
3. Keep README TODOs / fixture catalog accurate when closing or opening work.

## Publishing images

Maintainers with GHCR `write:packages`:

```sh
echo "$GITHUB_TOKEN" | docker login ghcr.io -u USERNAME --password-stdin
make publish
```

Set package visibility to **Public** for anonymous pulls. Prefer digest pins
downstream. See README “Published images”.

## Releases

1. Move notes from **Unreleased** into a new `## vX.Y.Z` section in
   [RELEASE.md](RELEASE.md) (Summary, Breaking changes, pins if relevant),
   ending with a `---` line. That section, up to its `---`, is the body of the
   GitHub release (`scripts/release-notes.sh vX.Y.Z` prints it).
2. Bump `interop_version` in `versions.yaml` (and README badge / pull examples).
   `make lint` and CI fail when RELEASE.md has no notes for it.
3. Tag `vX.Y.Z` and push — `.github/workflows/release.yml` checks the notes,
   publishes GHCR images and creates the release with those notes, the image
   digests and the release artifacts.

## Security

See [SECURITY.md](SECURITY.md). Do not expose port 102 publicly in examples.
