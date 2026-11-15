#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Tests of scripts/release-manifest.py, the manifest.json of a release."""

import importlib.util
import json
import os
import re
import subprocess
import sys
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("release_manifest", os.path.join(HERE, "release-manifest.py"))
rm = importlib.util.module_from_spec(spec)
spec.loader.exec_module(rm)

with open(os.path.join(HERE, "..", "versions.yaml"), encoding="utf-8") as f:
    TEXT = f.read().replace("\r\n", "\n")
VERSION = "v" + re.search(r'^interop_version: "([^"]+)"', TEXT, re.M).group(1)
COMMIT = "c" * 40
DIGESTS = [("native", "sha256:" + "1" * 64), ("python", "sha256:" + "2" * 64)]


class Manifest(unittest.TestCase):
    def test_every_published_reference_with_its_digest_and_upstream(self):
        doc = rm.manifest(VERSION, COMMIT, DIGESTS, TEXT)
        self.assertEqual((doc["version"], doc["commit"]), (VERSION, COMMIT))
        self.assertEqual(doc["platforms"], ["linux/amd64", "linux/arm64"])
        self.assertEqual(doc["images"], {"native": "ghcr.io/otfabric/snap7-interop-native@" + DIGESTS[0][1],
                                         "python": "ghcr.io/otfabric/snap7-interop-python@" + DIGESTS[1][1]})
        self.assertEqual(doc["upstream"]["native"]["revision"][:12], doc["upstream"]["native"]["version"])
        self.assertEqual(doc["upstream"]["python"]["license"], "MIT")
        self.assertEqual(doc["upstream"]["python"]["roles"], ["server"])
        for key in ("contract", "fixtureSchema"):
            self.assertTrue(doc[key])

    def test_the_licence_comes_from_versions_yaml(self):
        text = TEXT.replace('  license: "MIT"', '  license: "BSD-3-Clause"', 1)
        self.assertEqual(rm.manifest(VERSION, COMMIT, DIGESTS, text)["upstream"]["python"]["license"], "BSD-3-Clause")

    def test_a_missing_repeated_or_unknown_digest_is_refused(self):
        for digests in (DIGESTS[:1], DIGESTS + DIGESTS[1:], DIGESTS + [("otfabric", "sha256:" + "3" * 64)], []):
            with self.subTest(digests=digests), self.assertRaises(ValueError):
                rm.manifest(VERSION, COMMIT, digests, TEXT)

    def test_a_value_that_is_not_a_digest_is_refused(self):
        for value in ("", "sha256:", "sha256:" + "1" * 63, "1" * 64, "sha256:" + "A" * 64):
            with self.subTest(value=value), self.assertRaises(ValueError):
                rm.manifest(VERSION, COMMIT, [("native", value), DIGESTS[1]], TEXT)

    def test_a_version_other_than_interop_version_is_refused(self):
        for version in ("v0.0.0", VERSION[1:], VERSION + "-rc.1"):
            with self.subTest(version=version), self.assertRaises(ValueError):
                rm.manifest(version, COMMIT, DIGESTS, TEXT)

    def test_an_unpublished_reference_needs_no_digest(self):
        text = TEXT.replace("  published: true               # images.python", "  published: false", 1)
        doc = rm.manifest(VERSION, COMMIT, DIGESTS[:1], text)
        self.assertEqual(list(doc["images"]), ["native"])


class Command(unittest.TestCase):
    def run_manifest(self, *args):
        with tempfile.TemporaryDirectory() as tmp:
            out = os.path.join(tmp, "manifest.json")
            p = subprocess.run([sys.executable, os.path.join(HERE, "release-manifest.py"), VERSION, COMMIT, out] + list(args),
                               stdout=subprocess.PIPE, stderr=subprocess.PIPE, universal_newlines=True)
            doc = None
            if os.path.exists(out):
                with open(out) as f:
                    doc = json.load(f)
        return p.returncode, doc

    def test_writes_the_manifest(self):
        code, doc = self.run_manifest(*("%s=%s" % d for d in DIGESTS))
        self.assertEqual(code, 0)
        self.assertEqual(sorted(doc["images"]), ["native", "python"])

    def test_writes_nothing_without_every_digest(self):
        self.assertEqual(self.run_manifest("native=" + DIGESTS[0][1]), (1, None))
        self.assertEqual(self.run_manifest("native=" + DIGESTS[0][1], "python="), (1, None))
        self.assertEqual(self.run_manifest()[0], 2)


if __name__ == "__main__":
    unittest.main()
