"""Load compiled snap-interop fixtures."""

from __future__ import annotations

import base64
import json
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any


@dataclass
class AreaBuffer:
    type: str
    size: int
    access: str
    data: bytearray
    number: int | None = None


@dataclass
class CompiledFixture:
    id: str
    revision: int
    source_sha256: str
    compiled_sha256: str
    rack: int
    slot: int
    pdu_size: int
    areas: list[AreaBuffer] = field(default_factory=list)


def load_compiled_fixture(path: str | Path) -> CompiledFixture:
    raw = Path(path).read_text(encoding="utf-8")
    doc: dict[str, Any] = json.loads(raw)

    if doc.get("format") != "snap-interop-compiled-fixture":
        raise ValueError("unsupported compiled fixture format")
    if doc.get("formatVersion") != "1.0.0":
        raise ValueError("unsupported compiled fixture formatVersion")

    meta = doc["fixture"]
    server = doc["server"]
    areas: list[AreaBuffer] = []
    for area in doc["areas"]:
        size = int(area["size"])
        if size <= 0 or size > 1024 * 1024:
            raise ValueError("area size out of allowed range")
        data = bytearray(base64.b64decode(area["dataBase64"]))
        if len(data) != size:
            raise ValueError(f"area dataBase64 length mismatch for type={area['type']}")
        areas.append(
            AreaBuffer(
                type=str(area["type"]),
                number=int(area["number"]) if area["type"] == "db" else None,
                size=size,
                access=str(area.get("access", "read-write")),
                data=data,
            )
        )

    if not areas:
        raise ValueError("compiled fixture has no areas")

    return CompiledFixture(
        id=str(meta["id"]),
        revision=int(meta.get("revision", 1)),
        source_sha256=str(meta.get("sourceSha256", "")),
        compiled_sha256=str(meta.get("compiledSha256", "")),
        rack=int(server.get("rack", 0)),
        slot=int(server.get("slot", 2)),
        pdu_size=int(server.get("pduSize", 480)),
        areas=areas,
    )
