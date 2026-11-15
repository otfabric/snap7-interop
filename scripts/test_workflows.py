#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""SECURITY.md: every image a workflow pushes is scanned first, with the
release gate (fixable CRITICAL and HIGH fail the job)."""

import glob
import os
import unittest

try:
    import yaml
except ImportError:
    yaml = None

WORKFLOWS = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), ".github", "workflows")


def pushes(step):
    """Whether a step pushes an image to a registry."""
    run = step.get("run") or ""
    if "docker push" in run or "--push" in run:
        return True
    if str(step.get("uses", "")).startswith("docker/build-push-action"):
        w = step.get("with") or {}
        return str(w.get("push", "")).lower() == "true" or "push=true" in str(w.get("outputs", ""))
    return False


def release_gate(step):
    """Whether a step is a Trivy scan that fails on fixable CRITICAL and HIGH findings."""
    if not str(step.get("uses", "")).startswith("aquasecurity/trivy-action"):
        return False
    w = step.get("with") or {}
    severities = {s.strip() for s in str(w.get("severity", "")).split(",")}
    return ({"CRITICAL", "HIGH"} <= severities and str(w.get("ignore-unfixed", "")).lower() == "true"
            and str(w.get("exit-code", "")) == "1")


@unittest.skipIf(yaml is None, "PyYAML is not installed")
class Workflows(unittest.TestCase):
    def test_every_push_follows_a_scan_with_the_release_gate(self):
        pushed = 0
        for path in sorted(glob.glob(os.path.join(WORKFLOWS, "*.yml"))):
            with open(path, encoding="utf-8") as f:
                doc = yaml.safe_load(f)
            for job_id, job in (doc.get("jobs") or {}).items():
                steps = job.get("steps") or []
                for i, step in enumerate(steps):
                    if not pushes(step):
                        continue
                    pushed += 1
                    with self.subTest(workflow=os.path.basename(path), job=job_id, step=step.get("name")):
                        self.assertTrue(any(release_gate(s) for s in steps[:i]),
                                        "pushes an image without a Trivy scan (CRITICAL,HIGH, ignore-unfixed, exit-code 1) before it")
        # release.yml pushes by digest, own-go-s7comm.yml the tested own image.
        self.assertGreaterEqual(pushed, 2)

    def test_the_release_writes_manifest_json_with_every_digest(self):
        with open(os.path.join(WORKFLOWS, "release.yml"), encoding="utf-8") as f:
            steps = yaml.safe_load(f)["jobs"]["publish"]["steps"]
        runs = [s.get("run") or "" for s in steps]
        manifest = [r for r in runs if "scripts/release-manifest.py" in r]
        self.assertEqual(len(manifest), 1, "the publish job runs scripts/release-manifest.py once")
        self.assertIn("dist/release/manifest.json", manifest[0])
        for adapter in ("native", "python"):
            self.assertIn('"%s=${%s_DIGEST}"' % (adapter, adapter.upper()), manifest[0])
        upload = [s for s in steps if str(s.get("uses", "")).startswith("softprops/action-gh-release")]
        self.assertIn("dist/release/**", upload[0]["with"]["files"])


if __name__ == "__main__":
    unittest.main()
