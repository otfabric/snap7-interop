# ADR 0005: Independent repository probe

## Status

Accepted

## Date

2026-07-17

## Context

`go-s7comm` will later consume released images, but if this repository's own
gate depends exclusively on that client, server regressions and client
regressions become entangled and the repository is not independently useful.

## Decision

`snap-interop` owns an independent probe (`interop-probe`) that:

- starts or connects to both servers;
- executes fixture expectations;
- compares bytes and normalized outcomes;
- emits JSON, Markdown and JUnit reports;
- does not import or depend on `go-s7comm`.

`go-s7comm` may run an additional downstream matrix against pinned images.

## Consequences

- Tooling language choice (likely Go) is independent of the wire contract.
- Smoke and interop gates remain available without the downstream client.
- Downstream failures must distinguish client defects, adapter divergence and
  fixture errors.
