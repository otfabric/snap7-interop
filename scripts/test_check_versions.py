#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Tests of scripts/check-versions.sh, on a copy of the files it reads."""

import os
import re
import shutil
import subprocess
import tempfile
import unittest

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
FILES = ("versions.yaml", "Makefile", "scripts/check-versions.sh", "adapters/native/Dockerfile",
         "adapters/python/Dockerfile", "adapters/python/requirements.txt", "own/go-s7comm/Dockerfile")


def read(path):
    with open(os.path.join(ROOT, path), encoding="utf-8") as f:
        return f.read().replace("\r\n", "\n")


class CheckVersions(unittest.TestCase):
    def run_check(self, changes=None):
        """check-versions.sh on a copy in which, for each path: (old, new) of changes, old is replaced once."""
        with tempfile.TemporaryDirectory() as tmp:
            for path in FILES:
                os.makedirs(os.path.dirname(os.path.join(tmp, path)) or tmp, exist_ok=True)
                shutil.copy(os.path.join(ROOT, path), os.path.join(tmp, path))
            for name, (old, new) in (changes or {}).items():
                path = os.path.join(tmp, name)
                with open(path, encoding="utf-8") as f:
                    text = f.read().replace("\r\n", "\n")
                self.assertEqual(text.count(old), 1, "%s: %r" % (name, old))
                with open(path, "w", encoding="utf-8") as f:
                    f.write(text.replace(old, new))
            p = subprocess.run(["bash", os.path.join(tmp, "scripts", "check-versions.sh")],
                               stdout=subprocess.PIPE, stderr=subprocess.STDOUT, universal_newlines=True)
        return p.returncode, p.stdout

    def published_line(self, section):
        block = re.search(r"^%s:\n((?:  .*\n)*)" % section, read("versions.yaml"), re.M).group(1)
        return re.search(r"^  published: .*$", block, re.M).group(0)

    def test_the_repository_passes(self):
        code, out = self.run_check()
        self.assertEqual(code, 0, out)
        self.assertNotIn("FAIL", out)

    def test_a_snap7_version_other_than_the_short_revision_fails(self):
        version = re.search(r'^native_snap7:\n(?:  .*\n)*?  version: "([^"]+)"', read("versions.yaml"), re.M).group(1)
        code, out = self.run_check({"versions.yaml": ('  version: "%s"' % version, '  version: "1.4.0"')})
        self.assertEqual(code, 1, out)
        self.assertIn("FAIL Snap7 version", out)

    def test_a_reference_the_release_publishes_but_versions_yaml_does_not_fails(self):
        line = self.published_line("native_snap7")
        code, out = self.run_check({"versions.yaml": (line, "  published: false")})
        self.assertEqual(code, 1, out)
        self.assertIn("FAIL native published", out)

    def test_a_published_reference_the_release_does_not_publish_fails(self):
        line = re.search(r"^PUBLISHED_ADAPTERS \?=.*$", read("Makefile"), re.M).group(0)
        code, out = self.run_check({"Makefile": (line, "PUBLISHED_ADAPTERS ?= native")})
        self.assertEqual(code, 1, out)
        self.assertIn("FAIL python published", out)

    def test_a_published_reference_without_an_image_fails(self):
        line = re.search(r'^  python: "ghcr\.io/[^"]+"\n', read("versions.yaml"), re.M).group(0)
        code, out = self.run_check({"versions.yaml": (line, "")})
        self.assertEqual(code, 1, out)
        self.assertIn("FAIL python image", out)


if __name__ == "__main__":
    unittest.main()
