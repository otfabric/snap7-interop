#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Turn the fixture reports of a run into the organisation's interop-result-v1.

    scripts/report-to-results.py <reports dir> <results.json> <started> <reproduce command>
        [--servers <adapter-a>=<image>,<adapter-b>=<image>] [--known tests/known-differences.tsv]
        <fixture>=<ms>...

interop-probe compares two servers: its first endpoint ("native" in
report.json) is adapter a, its second ("python") adapter b. Without
--servers they are native and python, the two references.

report.json, report.md and junit.xml of each fixture stay what interop-probe
writes; results.json is derived from them. The fixtures of this run are named
on the command line, so a report left by an earlier run does not count, and a
fixture of this run without a report is an infrastructure error.

A fixture whose failing steps are all listed in the known-differences file, for the
adapter that failed them, is a known difference with a waiver. A listed step
that no longer fails is a stale waiver, which fails the run. The exit status
is 1 when the run failed: a fail or infra-error, a stale waiver, or no pass
at all (scripts/interop_result.py).

Each implementation's independence comes from what it says of itself: the
references from versions.yaml, any other image from its `version` command
(docs/CONTAINER_CONTRACT.md, Our own stack).
"""

import csv
import json
import re
import subprocess
import sys
from pathlib import Path

import interop_result

ROOT = Path(__file__).resolve().parent.parent
OUTCOME = {"pass": "pass", "fail": "fail", "invalid": "fail",
           "skip-unsupported": "unsupported", "infrastructure-error": "infra-error"}
SIDES = ("native", "python")  # the probe's two endpoints in report.json
DEFAULT_SERVERS = (("native", None), ("python", None))


def references(text):
    """The reference implementations, from versions.yaml."""
    native = re.search(r'^native_snap7:\n(?:  .*\n)*?  revision: "([^"]+)"', text, re.M).group(1)
    python = re.search(r'^python_snap7:\n(?:  .*\n)*?  version: "([^"]+)"', text, re.M).group(1)
    return {"native": {"name": "snap7", "upstream": "https://github.com/SCADACS/snap7", "version": native[:12],
                       "revision": native, "license": "LGPL-3.0-or-later", "independent": True},
            "python": {"name": "python-snap7", "version": python, "license": "MIT", "independent": True}}


def self_report(image, run=subprocess.run):
    """What an image other than the references says of itself (`version`)."""
    p = run(["docker", "run", "--rm", image, "version"], capture_output=True, text=True)
    if p.returncode != 0:
        raise ValueError("%s: `version` failed; an image outside the references must report itself" % image)
    v = json.loads(p.stdout)
    impl = {"name": v["upstream"], "version": v["upstreamVersion"], "independent": bool(v["independentEngine"])}
    if v.get("upstreamRevision"):
        impl["revision"] = v["upstreamRevision"]
    return impl


def implementation(adapter, image, refs, run=subprocess.run):
    impl = dict(refs[adapter]) if adapter in refs else self_report(image, run)
    if image:
        digest = interop_result.repo_digest(image)
        if digest and "@sha256:" in digest:
            impl["image"] = digest
    return impl


def load_known(path):
    """Rows of the known-differences file: adapter, fixture, step, outcome, evidence, reason."""
    if not path or not Path(path).exists():
        return []
    with open(path, encoding="utf-8") as fh:
        lines = [line for line in fh if line.strip() and not line.startswith("#")]
    rows = []
    for row in csv.reader(lines, delimiter="\t"):
        adapter, fixture, step, outcome, evidence = row[:5]
        rows.append({"adapter": adapter, "fixture": fixture, "step": int(step), "outcome": outcome,
                     "evidence": evidence, "reason": row[5] if len(row) > 5 else "",
                     "id": "%s.%s.%s" % (adapter, fixture, step), "reproduced": False})
    return rows


def failing_steps(report):
    for side in SIDES:
        for step in report.get(side, {}).get("steps") or []:
            if not step.get("pass"):
                yield side, step


def reason_of(report, outcome):
    if outcome == "unsupported":
        return report["native"].get("skipReason") or report["python"].get("skipReason") or report["status"]
    diffs = report.get("diffs") or []
    if diffs:
        return diffs[0]["message"]
    for side, step in failing_steps(report):
        return "%s: step %d %s: %s" % (side, step["index"], step["operation"], step.get("error") or step["outcome"])
    return "fixture status %s" % report["status"]


def known_difference(fixture, report, adapters, known):
    """The known-differences rows that explain every failing step, or None."""
    failing = list(failing_steps(report))
    if not failing:
        return None
    matched = []
    for side, step in failing:
        adapter = adapters[SIDES.index(side)]
        row = next((r for r in known if r["adapter"] == adapter and r["fixture"] == fixture
                    and r["step"] == step["index"] and r["outcome"] == step["outcome"]), None)
        if row is None:
            return None
        matched.append(row)
    return matched


def result_of(fixture, ms, report, roles=None, adapters=("native", "python"), known=(), reports="reports"):
    res = {"id": "fixture." + fixture, "suite": "fixture", "feature": fixture,
           "roles": roles or {"probe": "interop-probe", "server-a": "snap7", "server-b": "python-snap7"}, "durationMs": ms}
    if report is None:
        res.update(outcome="infra-error", reason="no report.json: the probe did not finish this fixture")
        return res
    res["outcome"] = OUTCOME[report["status"]]
    res["evidence"] = ["%s/%s/report.json" % (reports, fixture)]
    if res["outcome"] == "fail":
        rows = known_difference(fixture, report, adapters, known)
        if rows:
            for r in rows:
                r["reproduced"] = True
            res.update(outcome="known-difference", waiver=rows[0]["id"],
                       reason="; ".join("%s step %d: %s" % (r["adapter"], r["step"], r["reason"] or r["outcome"]) for r in rows))
            return res
    if res["outcome"] != "pass":
        res["reason"] = reason_of(report, res["outcome"])
    return res


def convert(reports, started, finished, reproduce, fixtures, servers=DEFAULT_SERVERS, known_path=None, run=subprocess.run):
    refs = references((ROOT / "versions.yaml").read_text())
    adapters = tuple(a for a, _ in servers)
    impls = [implementation(a, img, refs, run) for a, img in servers]
    roles = {"probe": "interop-probe", "server-a": impls[0]["name"], "server-b": impls[1]["name"]}
    names = {f for f, _ in fixtures}
    known = [r for r in load_known(known_path) if r["adapter"] in adapters and r["fixture"] in names]
    results = []
    for fixture, ms in fixtures:
        path = Path(reports) / fixture / "report.json"
        results.append(result_of(fixture, ms, json.loads(path.read_text()) if path.exists() else None, roles, adapters, known, reports))
    probe = {"name": "interop-probe", "version": interop_result.version_field(str(ROOT), "interop_version"), "independent": False}
    doc = {"schemaVersion": "1.0", "repository": "otfabric/snap7-interop",
           "interopVersion": interop_result.version_field(str(ROOT), "interop_version"),
           "contractVersion": interop_result.version_field(str(ROOT), "contract_version"),
           "gitSha": interop_result.git_sha(str(ROOT)), "startedAt": started, "finishedAt": finished,
           "host": interop_result.host(), "implementations": impls + [probe],
           "reproduce": reproduce, "results": results}
    if known:
        doc["waivers"] = [{"id": r["id"], "scope": "server@%s" % r["adapter"], "evidence": r["evidence"],
                           "reproduced": r["reproduced"]} for r in known]
    return doc


def parse_servers(spec):
    servers = []
    for part in spec.split(","):
        adapter, _, image = part.partition("=")
        servers.append((adapter, image or None))
    if len(servers) != 2:
        raise ValueError("--servers names two adapters, a and b")
    return tuple(servers)


def main(argv):
    if len(argv) < 4:
        print(__doc__, file=sys.stderr)
        return 2
    reports, out, started, reproduce = argv[:4]
    rest = argv[4:]
    servers, known_path = DEFAULT_SERVERS, None
    while rest and rest[0].startswith("--"):
        if rest[0] == "--servers":
            servers = parse_servers(rest[1])
        elif rest[0] == "--known":
            known_path = rest[1]
        else:
            print("unknown option %s" % rest[0], file=sys.stderr)
            return 2
        rest = rest[2:]
    fixtures = [(a.split("=")[0], int(a.split("=")[1])) for a in rest]
    doc = convert(reports, started, interop_result.now(), reproduce, fixtures, servers, known_path)
    interop_result.write(out, doc)
    bad = interop_result.schema_errors(doc) + interop_result.problems(doc)
    print("%s: %s" % (out, ", ".join("%d %s" % (doc["summary"][o], o) for o in interop_result.OUTCOMES)))
    for p in bad:
        print("FAIL " + p, file=sys.stderr)
    return 1 if bad else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
