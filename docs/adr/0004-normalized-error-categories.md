# ADR 0004: Normalized error categories

## Status

Accepted

## Date

2026-07-17

## Context

Native Snap7 and pure-Python servers expose different event codes and error
surfaces. Comparing raw upstream codes would couple the interop gate to one
implementation and hide semantic agreement.

## Decision

Black-box comparison uses normalized outcome categories:

- `success`
- `address-out-of-range`
- `write-protected`
- `unsupported`
- `transport-failure`
- `protocol-failure`

Native event codes and Python exceptions are diagnostics only. They may appear
in logs and reports as implementation-specific detail, but pass/fail decisions
use normalized categories plus returned bytes.

## Consequences

- Fixture expectations declare normalized results, not Snap7 event numbers.
- Mapping tables live in adapters/probe code and must be tested.
- Informational differences that do not affect bytes or category are not
  failures.
- Some upstream surfaces cannot emit every category distinctly (for example
  Snap7 ResourceLess read-only rejects as item `0x0A`, normalized today as
  `address-out-of-range`). See `docs/compatibility.md`.
