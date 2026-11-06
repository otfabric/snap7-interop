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


def convert(*reports, servers=rr.DEFAULT_SERVERS, known="", run=None):
    with tempfile.TemporaryDirectory() as d:
        for rep in reports:
            path = Path(d) / rep["fixtureId"]
            path.mkdir()
            (path / "report.json").write_text(json.dumps(rep))
        known_path = None
        if known:
            known_path = Path(d) / "known-differences.tsv"
            known_path.write_text(known)
        fixtures = [(r["fixtureId"], 5) for r in reports]
        doc = rr.convert(d, "2026-10-30T00:00:00Z", "2026-10-30T00:00:01Z", "make interop", fixtures,
                         servers, known_path, run or fake_version)
    doc["summary"] = interop_result.summarize(doc["results"])
    return doc


class FakeProc:
    def __init__(self, code, out):
        self.returncode, self.stdout = code, out


def fake_version(cmd, **kw):
    """docker run <image> version of an own image."""
    return FakeProc(0, json.dumps({"name": "otfabric", "adapterVersion": "v1.0.0", "upstream": "go-lib",
                                   "upstreamVersion": "v1.0.0", "upstreamRevision": "a" * 40, "independentEngine": False}))


RO_STEP = {"index": 1, "operation": "rejected-write", "outcome": "write-protected", "pass": False}
KNOWN = "# comment\notfabric\tread-only-reject\t1\twrite-protected\thttps://example.org/pr/16\t0x03 vs 0x0A\n"


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

    def test_an_own_image_reports_itself_as_not_independent(self):
        doc = convert(report("pass"), servers=(("native", None), ("otfabric", "own:dev")))
        own = doc["implementations"][1]
        self.assertEqual((own["name"], own["version"], own["independent"]), ("go-lib", "v1.0.0", False))
        self.assertEqual(doc["results"][0]["roles"]["server-b"], "go-lib")
        self.assertEqual(interop_result.schema_errors(doc), [])

    def test_an_image_without_version_is_refused(self):
        with self.assertRaises(ValueError):
            convert(report("pass"), servers=(("native", None), ("other", "x:dev")), run=lambda c, **k: FakeProc(2, ""))

    def test_a_listed_step_is_a_known_difference(self):
        rep = report("fail", fixtureId="read-only-reject", python={"steps": [RO_STEP]}, diffs=[{"message": "outcomes differ"}])
        doc = convert(report("pass"), rep, servers=(("native", None), ("otfabric", "own:dev")), known=KNOWN)
        res = doc["results"][1]
        self.assertEqual((res["outcome"], res["waiver"]), ("known-difference", "otfabric.read-only-reject.1"))
        self.assertEqual(doc["waivers"][0]["reproduced"], True)
        self.assertEqual(interop_result.schema_errors(doc), [])
        self.assertEqual(interop_result.problems(doc), [])

    def test_a_listed_step_of_another_adapter_is_a_failure(self):
        rep = report("fail", fixtureId="read-only-reject", native={"steps": [RO_STEP]})
        doc = convert(report("pass"), rep, servers=(("native", None), ("otfabric", "own:dev")), known=KNOWN)
        self.assertEqual(doc["results"][1]["outcome"], "fail")
        self.assertTrue(interop_result.problems(doc))

    def test_a_waiver_that_no_longer_reproduces_fails(self):
        doc = convert(report("pass"), report("pass", fixtureId="read-only-reject"),
                      servers=(("native", None), ("otfabric", "own:dev")), known=KNOWN)
        self.assertEqual(doc["waivers"][0]["reproduced"], False)
        self.assertTrue(any("stale waiver" in p for p in interop_result.problems(doc)))

    def test_known_differences_of_adapters_not_in_the_run_are_ignored(self):
        doc = convert(report("pass"), known=KNOWN)
        self.assertNotIn("waivers", doc)

    def test_parse_servers(self):
        self.assertEqual(rr.parse_servers("native=a:1,otfabric=b:2"), (("native", "a:1"), ("otfabric", "b:2")))
        with self.assertRaises(ValueError):
            rr.parse_servers("native")

    def test_no_fixture_fails(self):
        doc = rr.convert("/nonexistent", "2026-10-30T00:00:00Z", "2026-10-30T00:00:01Z", "make interop", [])
        doc["summary"] = interop_result.summarize(doc["results"])
        self.assertTrue(interop_result.problems(doc))


if __name__ == "__main__":
    unittest.main()
