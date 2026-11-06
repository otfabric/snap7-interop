"""Atomic diagnostic state snapshots."""

from __future__ import annotations

import base64
import json
import os
from dataclasses import dataclass
from datetime import UTC, datetime
from pathlib import Path

from snap7_interop_server import __version__
from snap7_interop_server.fixture import CompiledFixture


@dataclass
class RuntimeCounters:
    accepted_writes: int = 0
    rejected_accesses: int = 0


def write_state_snapshot(
    path: str | Path,
    fixture: CompiledFixture,
    status: str,
    counters: RuntimeCounters,
) -> None:
    areas = []
    for area in fixture.areas:
        item: dict[str, object] = {
            "type": area.type,
            "size": area.size,
            "access": area.access,
            "dataBase64": base64.b64encode(bytes(area.data)).decode("ascii"),
        }
        if area.type == "db" and area.number is not None:
            item["number"] = area.number
        areas.append(item)

    doc = {
        "fixture": {
            "id": fixture.id,
            "revision": fixture.revision,
            "compiledSha256": fixture.compiled_sha256,
        },
        "implementation": {
            "name": "python",
            "adapterVersion": __version__,
        },
        "serverStatus": status,
        "acceptedWriteCount": counters.accepted_writes,
        "rejectedAccessCount": counters.rejected_accesses,
        "lastUpdateTime": datetime.now(UTC).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "areas": areas,
    }

    target = Path(path)
    target.parent.mkdir(parents=True, exist_ok=True)
    tmp = target.with_suffix(target.suffix + ".tmp")
    tmp.write_text(json.dumps(doc, indent=2) + "\n", encoding="utf-8")
    os.replace(tmp, target)
