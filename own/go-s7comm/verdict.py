#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Verdict of one self-test fixture run of the otfabric image.

Usage: verdict.py <report.json> <known-differences.tsv> <fixture-id>

The probe's first endpoint ("native" in report.json) is the reference image
and must pass every step. The second ("python") is the otfabric image: each
failing step must be listed in known-differences.tsv with the outcome it
returned. Exit 0 pass, 3 pass with known differences, 1 fail.
"""

from __future__ import annotations

import csv
import json
import sys


def load_known(path: str) -> set[tuple[str, int, str]]:
    known: set[tuple[str, int, str]] = set()
    with open(path, encoding="utf-8") as fh:
        rows = (line for line in fh if line.strip() and not line.startswith("#"))
        for row in csv.reader(rows, delimiter="\t"):
            known.add((row[0], int(row[1]), row[2]))
    return known


def main(argv: list[str]) -> int:
    if len(argv) != 4:
        print(__doc__, file=sys.stderr)
        return 2
    try:
        with open(argv[1], encoding="utf-8") as fh:
            report = json.load(fh)
    except (OSError, ValueError) as exc:
        print(f"fail: no report ({exc})")
        return 1
    known = load_known(argv[2])
    fixture = argv[3]

    ref, own = report.get("native", {}), report.get("python", {})
    if ref.get("skipReason") or own.get("skipReason"):
        print("fail: skipped")
        return 1
    ref_steps, own_steps = ref.get("steps") or [], own.get("steps") or []
    if not ref_steps or not own_steps:
        print("fail: no steps ran")
        return 1
    bad_ref = [s for s in ref_steps if not s.get("pass")]
    if bad_ref:
        print(f"fail: reference failed steps {[s.get('index') for s in bad_ref]}")
        return 1
    unknown, accepted = [], []
    for step in own_steps:
        if step.get("pass"):
            continue
        key = (fixture, int(step.get("index", -1)), str(step.get("outcome")))
        (accepted if key in known else unknown).append(key)
    if unknown:
        print(f"fail: otfabric failed {unknown}")
        return 1
    if accepted:
        print(f"known difference {accepted}")
        return 3
    print(f"pass ({len(own_steps)} steps)")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
