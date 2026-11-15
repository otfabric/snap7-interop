# SPDX-License-Identifier: MIT
"""The two reference implementations as versions.yaml records them.

report-to-results.py names them in results.json, release-manifest.py in the
release's manifest.json. A two-level lookup of versions.yaml, without a YAML
parser: each reference is a top-level section of "  key: value" lines.
"""

import re

# adapter -> its section of versions.yaml, and the field that holds its name.
SECTIONS = {"native": ("native_snap7", "name"), "python": ("python_snap7", "package")}
REQUIRED = ("repository", "version", "license", "roles", "published")


def _block(text, section):
    m = re.search(r"^%s:\n((?:[ \t]+.*\n|[ \t]*#.*\n)*)" % re.escape(section), text, re.M)
    if not m:
        raise ValueError("versions.yaml has no %s" % section)
    return m.group(1)


def _field(block, key):
    """The value of "  key: value" (quotes and a trailing comment removed), or None."""
    m = re.search(r"^  %s:[ \t]*(.*)$" % re.escape(key), block, re.M)
    if not m:
        return None
    value = m.group(1).strip()
    if value.startswith('"'):
        end = value.find('"', 1)
        return value[1:end] if end > 0 else None
    return value.split(" #")[0].strip()


def _images(text):
    """The images: map of versions.yaml, adapter -> image."""
    m = re.search(r"^images:\n((?:[ \t]+.*\n)*)", text, re.M)
    return dict(re.findall(r'^  (\w+): "([^"]+)"', m.group(1), re.M)) if m else {}


def load(text):
    """{adapter: reference} for both references of versions.yaml.

    A reference has name, upstream, version, revision (when recorded),
    license, roles, published and, when published, image. A missing field
    raises ValueError.
    """
    text = text.replace("\r\n", "\n")
    images = _images(text)
    out = {}
    for adapter, (section, name_key) in SECTIONS.items():
        block = _block(text, section)
        values = {key: _field(block, key) for key in (name_key, "revision") + REQUIRED}
        missing = [key for key in (name_key,) + REQUIRED if not values[key]]
        if missing:
            raise ValueError("versions.yaml: %s has no %s" % (section, ", ".join(missing)))
        roles = re.fullmatch(r"\[([^\]]*)\]", values["roles"])
        if not roles or not roles.group(1).strip():
            raise ValueError("versions.yaml: %s roles is not a list like [server]" % section)
        if values["published"] not in ("true", "false"):
            raise ValueError("versions.yaml: %s published is %r, not true or false" % (section, values["published"]))
        ref = {"name": values[name_key], "upstream": values["repository"], "version": values["version"],
               "license": values["license"], "roles": [r.strip() for r in roles.group(1).split(",")],
               "published": values["published"] == "true"}
        if values["revision"]:
            ref["revision"] = values["revision"]
        if adapter in images:
            ref["image"] = images[adapter]
        out[adapter] = ref
    return out
