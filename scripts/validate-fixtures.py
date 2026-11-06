#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Checks that the case lists agree with the fixtures.

    scripts/validate-fixtures.py

tests/cases.tsv must name every canonical fixture once, with the capability
tokens the fixture requires, and a compiled golden must exist for each.
tests/known-bugs.tsv rows must name a listed fixture, an existing step and an
outcome interop-probe reports, with an http(s) evidence URL. The fixtures
themselves are validated by fixturectl (make test, make fixtures-check).
"""

import csv
import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
OUTCOMES = {"success", "accepted", "address-out-of-range", "write-protected", "unsupported",
            "transport-failure", "protocol-failure"}


def rows(path):
    with open(path, encoding="utf-8") as fh:
        lines = [line for line in fh if line.strip() and not line.startswith("#")]
    return list(csv.reader(lines, delimiter="\t"))


def problems(root=ROOT):
    out = []
    fixtures = {}
    for p in sorted((root / "fixtures" / "canonical").glob("*.json")):
        doc = json.loads(p.read_text())
        fixtures[doc["id"]] = doc
    seen = set()
    for row in rows(root / "tests" / "cases.tsv"):
        if len(row) != 3:
            out.append("cases.tsv: %r: want 3 tab-separated columns" % row)
            continue
        fid, requires = row[0], row[1]
        if fid in seen:
            out.append("cases.tsv: %s listed twice" % fid)
        seen.add(fid)
        if fid not in fixtures:
            out.append("cases.tsv: %s is not a canonical fixture" % fid)
            continue
        if requires != ",".join(fixtures[fid].get("requires", [])):
            out.append("cases.tsv: %s requires %r, the fixture %r" % (fid, requires, ",".join(fixtures[fid].get("requires", []))))
        if not (root / "fixtures" / "compiled" / ("%s.compiled.json" % fid)).exists():
            out.append("cases.tsv: %s has no compiled golden" % fid)
    for fid in sorted(set(fixtures) - seen):
        out.append("cases.tsv: canonical fixture %s is not listed" % fid)
    for row in rows(root / "tests" / "known-bugs.tsv"):
        if len(row) != 6:
            out.append("known-bugs.tsv: %r: want 6 tab-separated columns" % row)
            continue
        adapter, fid, step, outcome, evidence, _ = row
        if fid not in seen:
            out.append("known-bugs.tsv: %s: %s is not a case" % (adapter, fid))
        elif not step.isdigit() or int(step) >= len(fixtures[fid].get("expectations", [])):
            out.append("known-bugs.tsv: %s %s: no step %s" % (adapter, fid, step))
        if outcome not in OUTCOMES:
            out.append("known-bugs.tsv: %s %s: unknown outcome %s" % (adapter, fid, outcome))
        if not evidence.startswith(("https://", "http://")):
            out.append("known-bugs.tsv: %s %s: evidence is not a URL" % (adapter, fid))
    return out


def main():
    bad = problems()
    for p in bad:
        print("FAIL " + p, file=sys.stderr)
    if not bad:
        print("ok   tests/cases.tsv and tests/known-bugs.tsv agree with the fixtures")
    return 1 if bad else 0


if __name__ == "__main__":
    sys.exit(main())
