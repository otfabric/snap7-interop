#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Compares the upstream pin of one server with the latest upstream.

    scripts/latest-upstream.py python-snap7|snap7

Prints key=value lines (the format of $GITHUB_OUTPUT):

    pin=<pin in versions.yaml>
    latest=<latest upstream>
    latest_sha=<its commit, for a component that is pinned by commit>
    update=yes|no
    source=<where the latest was looked up>

python-snap7 (servers/python) is compared with the latest stable PyPI release;
pre-releases and yanked releases are never "latest". Snap7 (servers/native)
is pinned by commit of the SCADACS fork, which has no release tags, and is
compared with the HEAD of its default branch. Nothing is modified.
"""

import json
import re
import subprocess
import sys
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
AGENT = "snap7-interop latest-upstream (https://github.com/otfabric/snap7-interop)"
SNAP7_REPO = "https://github.com/SCADACS/snap7.git"


def fetch(url):
    request = urllib.request.Request(url, headers={"User-Agent": AGENT, "Accept": "application/json, */*"})
    with urllib.request.urlopen(request, timeout=30) as response:
        return response.read().decode()


def key(version):
    """Sort key of a stable version; None for anything else (pre-releases)."""
    match = re.fullmatch(r"v?(\d+(?:\.\d+)*)", version)
    return tuple(int(part) for part in match.group(1).split(".")) if match else None


def manifest(section, field):
    """versions.yaml <section>.<field> (a two-level lookup, no YAML parser needed)."""
    text = (ROOT / "versions.yaml").read_text()
    block = re.search(rf"^{section}:\n((?:[ \t]+.*\n|\n)*)", text, re.M)
    value = block and re.search(rf"^\s+{field}:\s*[\"']?([^\"'\n]+)", block.group(1), re.M)
    if not value:
        raise SystemExit(f"versions.yaml has no {section}.{field}")
    return value.group(1).strip()


def same_pin(pin, path, pattern):
    """The pin of versions.yaml must be the one that is built."""
    match = re.search(pattern, (ROOT / path).read_text(), re.M)
    if not match or match.group(1).strip() != pin:
        raise SystemExit(f"{path} does not pin {pin} (versions.yaml)")


def python_snap7():
    pin = manifest("python_snap7", "version")
    same_pin(pin, "servers/python/requirements.txt", r"^python-snap7==(\S+)")
    releases = json.loads(fetch("https://pypi.org/pypi/python-snap7/json"))["releases"]
    live = [v for v, files in releases.items() if files and not all(f.get("yanked") for f in files)]
    stable = [v for v in live if key(v) is not None]
    if not stable:
        raise SystemExit("no stable release found upstream")
    latest = max(stable, key=key)
    return pin, latest, "", "yes" if key(latest) != key(pin) else "no", "PyPI"


def snap7():
    pin = manifest("native_snap7", "revision")
    same_pin(pin, "servers/native/Dockerfile", r"^ARG SNAP7_REVISION=(\S+)")
    head = subprocess.check_output(["git", "ls-remote", SNAP7_REPO, "HEAD"], text=True).split()[0]
    return pin, head, head, "yes" if head != pin else "no", "HEAD of SCADACS/snap7"


RESOLVERS = {"python-snap7": python_snap7, "snap7": snap7}


def main():
    if len(sys.argv) != 2 or sys.argv[1] not in RESOLVERS:
        raise SystemExit(f"usage: latest-upstream.py {'|'.join(RESOLVERS)}")
    pin, latest, latest_sha, update, source = RESOLVERS[sys.argv[1]]()
    print(f"pin={pin}")
    print(f"latest={latest}")
    print(f"latest_sha={latest_sha}")
    print(f"update={update}")
    print(f"source={source}")


if __name__ == "__main__":
    main()
