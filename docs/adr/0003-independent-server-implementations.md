# ADR 0003: Independent server implementations

## Status

Accepted

## Date

2026-07-17

## Context

The repository exists to compare two unrelated S7 server stacks. Sharing
protocol code or loading the native Snap7 library from the Python image would
collapse that independence.

## Decision

Keep server implementations independent.

- The native image builds Snap7 from a pinned immutable source revision and
  repository-owned C/C++ adapter code.
- The Python image uses a pinned pure-Python `python-snap7` server and
  repository-owned adapter code.
- The Python runtime must not load or wrap the native Snap7 shared library.
- Adapters may share only repository-defined fixture, logging, state snapshot
  and reporting contracts.

## Consequences

- Duplicate lifecycle wiring is acceptable; shared protocol libraries are not.
- Build-time checks must prove the Python image has no native Snap7 dependency.
- Divergence is documented in `docs/compatibility.md`, never hidden.
