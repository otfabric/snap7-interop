# ADR 0001: Fixture-first architecture

## Status

Accepted

## Date

2026-07-17

## Context

`snap7-interop` must drive two independent S7 server implementations from one
contract and later serve as a reusable test target for clients such as
`go-s7comm`. If adapters invent their own initialization or typed encoding, the
repository cannot prove equivalence.

## Decision

Fixtures are the product contract.

- Source fixtures describe intent in implementation-neutral JSON.
- Server adapters only register and expose compiled bytes and declared settings.
- Fixtures must not contain C-specific or Python-specific fields.
- Adapters must not silently ignore mandatory fixture settings.
- Unsupported optional settings produce explicit `skip-unsupported` results.

## Consequences

- Schema and compiler work precedes broad server scaffolding.
- Behavioral changes require fixture and/or expectation updates.
- Capability manifests become part of the public contract.
