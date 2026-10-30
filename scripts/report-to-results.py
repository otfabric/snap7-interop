#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Turn the fixture reports of a run into the organisation's interop-result-v1.

    scripts/report-to-results.py <reports dir> <results.json> <started> <reproduce command> <fixture>=<ms>...

report.json, report.md and junit.xml of each fixture stay what interop-probe
writes; results.json is derived from them. The fixtures of this run are named
on the command line, so a report left by an earlier run does not count, and a
fixture of this run without a report is an infrastructure error. The exit
status is 1 when the run failed: a fail or infra-error, or no pass at all
(scripts/interop_result.py).
"""

import json
import re
import sys
from pathlib import Path

import interop_result

ROOT = Path(__file__).resolve().parent.parent
OUTCOME = {"pass": "pass", "fail": "fail", "invalid": "fail",
           "skip-unsupported": "unsupported", "infrastructure-error": "infra-error"}
ROLES = {"probe": "interop-probe", "server-a": "snap7", "server-b": "python-snap7"}


def reason_of(report, outcome):
    if outcome == "unsupported":
        return report["native"].get("skipReason") or report["python"].get("skipReason") or report["status"]
    diffs = report.get("diffs") or []
    if diffs:
        return diffs[0]["message"]
    for side in ("native", "python"):
        for step in report[side].get("steps") or []:
            if not step.get("pass"):
                return "%s: step %d %s: %s" % (side, step["index"], step["operation"], step.get("error") or step["outcome"])
    return "fixture status %s" % report["status"]


def result_of(fixture, ms, report):
    res = {"id": "fixture." + fixture, "suite": "fixture", "feature": fixture, "roles": ROLES, "durationMs": ms}
    if report is None:
        res.update(outcome="infra-error", reason="no report.json: the probe did not finish this fixture")
        return res
    res["outcome"] = OUTCOME[report["status"]]
    if res["outcome"] != "pass":
        res["reason"] = reason_of(report, res["outcome"])
    res["evidence"] = ["reports/%s/report.json" % fixture]
    return res


def implementations(text):
    native = re.search(r'^native_snap7:\n(?:  .*\n)*?  revision: "([^"]+)"', text, re.M).group(1)
    python = re.search(r'^python_snap7:\n(?:  .*\n)*?  version: "([^"]+)"', text, re.M).group(1)
    return [{"name": "snap7", "upstream": "https://github.com/SCADACS/snap7", "version": native[:12], "revision": native,
             "license": "LGPL-3.0-or-later", "independent": True},
            {"name": "python-snap7", "version": python, "license": "MIT", "independent": True},
            {"name": "interop-probe", "version": interop_result.version_field(str(ROOT), "interop_version"), "independent": False}]


def convert(reports, started, finished, reproduce, fixtures):
    results = []
    for fixture, ms in fixtures:
        path = Path(reports) / fixture / "report.json"
        results.append(result_of(fixture, ms, json.loads(path.read_text()) if path.exists() else None))
    return {"schemaVersion": "1.0", "repository": "otfabric/snap7-interop",
            "interopVersion": interop_result.version_field(str(ROOT), "interop_version"),
            "contractVersion": interop_result.version_field(str(ROOT), "contract_version"),
            "gitSha": interop_result.git_sha(str(ROOT)), "startedAt": started, "finishedAt": finished,
            "host": interop_result.host(), "implementations": implementations((ROOT / "versions.yaml").read_text()),
            "reproduce": reproduce, "results": results}


def main(argv):
    if len(argv) < 4:
        print(__doc__, file=sys.stderr)
        return 2
    reports, out, started, reproduce = argv[:4]
    fixtures = [(a.split("=")[0], int(a.split("=")[1])) for a in argv[4:]]
    doc = convert(reports, started, interop_result.now(), reproduce, fixtures)
    interop_result.write(out, doc)
    bad = interop_result.schema_errors(doc) + interop_result.problems(doc)
    print("%s: %s" % (out, ", ".join("%d %s" % (doc["summary"][o], o) for o in interop_result.OUTCOMES)))
    for p in bad:
        print("FAIL " + p, file=sys.stderr)
    return 1 if bad else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
