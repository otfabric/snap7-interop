#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Tests of scripts/report-to-results.py that need no Docker."""

import importlib.util
import json
import os
import tempfile
import unittest
from pathlib import Path

import interop_result

HERE = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("report_to_results", os.path.join(HERE, "report-to-results.py"))
rr = importlib.util.module_from_spec(spec)
spec.loader.exec_module(rr)


def report(status, **kw):
    r = {"fixtureId": "basic-db", "status": status, "native": {"name": "native", "steps": []}, "python": {"name": "python", "steps": []}}
    r.update(kw)
    return r


def convert(*reports):
    with tempfile.TemporaryDirectory() as d:
        for rep in reports:
            path = Path(d) / rep["fixtureId"]
            path.mkdir()
            (path / "report.json").write_text(json.dumps(rep))
        fixtures = [(r["fixtureId"], 5) for r in reports]
        doc = rr.convert(d, "2026-10-30T00:00:00Z", "2026-10-30T00:00:01Z", "make interop", fixtures)
    doc["summary"] = interop_result.summarize(doc["results"])
    return doc


class Convert(unittest.TestCase):
    def test_a_pass_is_valid_and_passes(self):
        doc = convert(report("pass"))
        self.assertEqual(interop_result.schema_errors(doc), [])
        self.assertEqual(interop_result.problems(doc), [])
        self.assertEqual(doc["results"][0]["id"], "fixture.basic-db")

    def test_mapping(self):
        for status, outcome in (("fail", "fail"), ("invalid", "fail"), ("infrastructure-error", "infra-error"), ("skip-unsupported", "unsupported")):
            doc = convert(report("pass"), report(status, fixtureId="other"))
            self.assertEqual(doc["results"][1]["outcome"], outcome)
            self.assertEqual(interop_result.schema_errors(doc), [])

    def test_all_skipped_fails(self):
        doc = convert(report("skip-unsupported", native={"skipReason": "no such area", "steps": []}))
        self.assertEqual(doc["results"][0]["reason"], "no such area")
        self.assertTrue(interop_result.problems(doc))

    def test_fail_gives_the_first_diff(self):
        doc = convert(report("fail", diffs=[{"message": "bytes differ"}]))
        self.assertEqual(doc["results"][0]["reason"], "bytes differ")
        self.assertTrue(interop_result.problems(doc))

    def test_fail_without_diff_names_the_step(self):
        step = {"index": 2, "operation": "read", "outcome": "timeout", "pass": False}
        doc = convert(report("fail", python={"steps": [step]}))
        self.assertIn("step 2 read", doc["results"][0]["reason"])

    def test_no_report_is_an_infra_error(self):
        res = rr.result_of("basic-db", 5, None)
        self.assertEqual(res["outcome"], "infra-error")

    def test_a_status_never_seen_is_refused(self):
        with self.assertRaises(KeyError):
            rr.result_of("basic-db", 5, report("skip"))

    def test_no_fixture_fails(self):
        doc = rr.convert("/nonexistent", "2026-10-30T00:00:00Z", "2026-10-30T00:00:01Z", "make interop", [])
        doc["summary"] = interop_result.summarize(doc["results"])
        self.assertTrue(interop_result.problems(doc))


if __name__ == "__main__":
    unittest.main()
