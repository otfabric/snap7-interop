# ADR 0002: Canonical byte compilation

## Status

Accepted

## Date

2026-07-17

## Context

Typed values, hex/base64 initializers and patches can be encoded in more than
one way. If each server encodes types independently, byte-level comparison is
meaningless.

## Decision

Source fixtures are compiled into a canonical representation before server
startup.

The compiler must:

1. validate against JSON Schema Draft 2020-12;
2. perform semantic validation;
3. normalize defaults;
4. encode typed values into bytes;
5. materialize complete zero-filled memory areas;
6. compute deterministic source and compiled digests;
7. emit byte-stable canonical JSON.

Identical source input, schema and compiler versions must produce byte-identical
compiled output. Golden tests update only through an explicit command.

## Consequences

- Servers consume compiled fixtures, not source fixtures.
- Typed encoding lives only in the compiler.
- Digest mismatches are release-blocking for fixture/tooling changes.
