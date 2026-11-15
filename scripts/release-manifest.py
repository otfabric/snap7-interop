#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Writes the manifest.json of a release: the digest of every published image
and the upstream it was built from.

    scripts/release-manifest.py <vX.Y.Z> <commit> <manifest.json> <adapter>=<sha256:...>...

The release workflow passes one digest for every reference that versions.yaml
marks published: true; a missing, repeated or unknown adapter, a value that is
not a digest, or a version other than interop_version fails, and nothing is
written. The rest comes from versions.yaml (scripts/references.py).
"""

import json
import re
import sys
from pathlib import Path

import references

ROOT = Path(__file__).resolve().parent.parent


def top(text, key):
    m = re.search(r'^%s: "([^"]+)"' % re.escape(key), text, re.M)
    if not m:
        raise ValueError("versions.yaml has no %s" % key)
    return m.group(1)


def platforms(text):
    m = re.search(r"^platforms:\n((?:[ \t]+-.*\n)*)", text, re.M)
    found = re.findall(r'^[ \t]+- "([^"]+)"', m.group(1), re.M) if m else []
    if not found:
        raise ValueError("versions.yaml has no platforms")
    return found


def manifest(version, commit, digests, text):
    """The manifest document, or ValueError when the arguments do not fit versions.yaml."""
    if not re.fullmatch(r"v\d+\.\d+\.\d+", version):
        raise ValueError("%r is not vX.Y.Z" % version)
    if top(text, "interop_version") != version[1:]:
        raise ValueError("versions.yaml interop_version is not %s" % version[1:])
    refs = references.load(text)
    published = [a for a, r in refs.items() if r["published"]]
    names = [a for a, _ in digests]
    missing = [a for a in published if a not in names]
    repeated = sorted({a for a in names if names.count(a) > 1})
    unknown = [a for a in names if a not in published]
    if missing or repeated or unknown:
        raise ValueError("one digest for every published reference (%s): missing %s, repeated %s, not published %s"
                         % (" ".join(published), missing, repeated, unknown))
    images, upstream = {}, {}
    for adapter, digest in digests:
        if not re.fullmatch(r"sha256:[0-9a-f]{64}", digest):
            raise ValueError("%s: %r is not a digest" % (adapter, digest))
        ref = refs[adapter]
        if "image" not in ref:
            raise ValueError("versions.yaml images has no %s" % adapter)
        images[adapter] = "%s@%s" % (ref["image"], digest)
        upstream[adapter] = {k: ref[k] for k in ("name", "upstream", "version", "revision", "license", "roles") if k in ref}
    return {"version": version, "commit": commit, "contract": top(text, "contract_version"),
            "fixtureSchema": top(text, "fixture_schema"), "platforms": platforms(text),
            "upstream": upstream, "images": images}


def main(argv):
    if len(argv) < 4 or any("=" not in a for a in argv[3:]):
        print(__doc__, file=sys.stderr)
        return 2
    version, commit, out = argv[:3]
    digests = [tuple(a.split("=", 1)) for a in argv[3:]]
    try:
        doc = manifest(version, commit, digests, (ROOT / "versions.yaml").read_text())
    except ValueError as e:
        print("release-manifest: %s" % e, file=sys.stderr)
        return 1
    Path(out).write_text(json.dumps(doc, indent=2) + "\n")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
