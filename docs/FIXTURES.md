# Fixtures

What the servers serve and what interop-probe expects of them. Schema:
[fixtures/schema/fixture.schema.json](../fixtures/schema/fixture.schema.json);
design: [ADR 0001](adr/0001-fixture-first-architecture.md) and
[ADR 0002](adr/0002-canonical-byte-compilation.md).

## Layout

| Path | Role |
| --- | --- |
| `fixtures/schema/fixture.schema.json` | Draft 2020-12 schema for source fixtures |
| `fixtures/canonical/` | Human-authored fixtures that must pass on both servers |
| `fixtures/compiled/` | Golden compiled artifacts (update only via `make fixtures-update`) |
| `fixtures/invalid/` | Fixtures that must fail validation with stable diagnostics |

## Canonical fixtures (interop matrix)

All of the following pass dual interop on the pinned native and Python images.

| ID | Description |
| --- | --- |
| `basic-db` | Smoke DB1 with a short hex prefix; primary health/smoke fixture |
| `write-cycle` | Accepted write mutates DB1 and is visible on subsequent read |
| `multiple-dbs` | Two DBs with distinct patterns (ReadDB/WriteDB) |
| `markers-io` | Markers, inputs, and outputs (ReadMerkers/ReadInputs/ReadOutputs) |
| `typed-values` | DB + markers with typed scalar patches |
| `integer-boundaries` | Signed/unsigned integer boundary bytes (big-endian) |
| `floating-point` | IEEE-754 float32/float64 patterns (big-endian) |
| `bit-access` | Bit reads and writes within a DB byte |
| `unaligned-range` | Unaligned multi-byte reads across odd offsets |
| `last-valid-byte` | Read at the last valid byte of a DB |
| `out-of-range` | Out-of-range read/write rejected as `address-out-of-range` |
| `read-only-reject` | Write to read-only DB rejected; memory unchanged on follow-up read |
| `large-read` | Large single-request DB read within negotiated PDU |
| `concurrent-reads` | Multiple simultaneous clients read identical bytes |
| `reconnect-state` | Write, reconnect client, observe preserved server memory |

Source: `fixtures/canonical/<id>.json`  
Compiled: `fixtures/compiled/<id>.compiled.json`

## Invalid fixtures (compiler / schema gate)

These must **fail** `fixturectl validate` (used by `make test`):

| File | Intent |
| --- | --- |
| `invalid-bounds.json` | Hex initializer exceeds declared area size |
| `invalid-duplicate-area.json` | Duplicate DB1 areas rejected |
| `invalid-overlap.json` | Typed patches overlap without explicit replace |
| `invalid-type-range.json` | `uint8` value outside 0..255 |

## Adding a fixture

1. Author `fixtures/canonical/<id>.json` against the schema.
2. Declare `requires` capabilities the servers must advertise.
3. Add expectations with normalized outcomes or hex payloads.
4. Run `make fixtures-update` then `make fixtures-check`.
5. Run `FIXTURE=<id> make interop` and keep both servers green.
6. Document differences in [COMPATIBILITY.md](COMPATIBILITY.md) if needed.
7. Update this catalog, [tests/cases.tsv](../tests/cases.tsv) (`make validate` checks it) and [go-s7comm-coverage.md](go-s7comm-coverage.md).

## Cases and known differences

`scripts/interop.sh <a> <b>` runs every fixture of [tests/cases.tsv](../tests/cases.tsv)
against two servers and writes `reports/<fixture>/` and `reports/results.json`
(interop-result-v1, one result `fixture.<id>` per fixture). A failing step that
[tests/known-bugs.tsv](../tests/known-bugs.tsv) lists for the adapter that failed
it is a `known-difference` with a waiver; an entry that no longer reproduces
fails the run. The references have no entry.

## Fixture example

Minimal source shape (see [fixtures/canonical/basic-db.json](../fixtures/canonical/basic-db.json) for a real fixture):

```json
{
  "$schema": "../schema/fixture.schema.json",
  "schemaVersion": "1.0.0",
  "id": "basic-db",
  "description": "Single zero-filled DB1 with a short hex prefix for smoke reads",
  "requires": ["area.db", "operation.read"],
  "server": { "rack": 0, "slot": 2, "pduSize": 480 },
  "areas": [
    {
      "type": "db",
      "number": 1,
      "size": 64,
      "access": "read-write",
      "initial": { "hex": "0102030405060708" }
    }
  ],
  "expectations": [
    {
      "operation": "read",
      "area": { "type": "db", "number": 1 },
      "offset": 0,
      "length": 8,
      "result": { "hex": "0102030405060708" }
    }
  ]
}
```

Typed values are compiled once into canonical bytes. Adapters do not encode types themselves.

