"""Fixture loading tests for the Python adapter."""

from __future__ import annotations

from pathlib import Path

from snap_interop_server.fixture import load_compiled_fixture

REPO_ROOT = Path(__file__).resolve().parents[3]


def test_load_basic_db_compiled_fixture() -> None:
    path = REPO_ROOT / "fixtures" / "compiled" / "basic-db.compiled.json"
    fx = load_compiled_fixture(path)
    assert fx.id == "basic-db"
    assert len(fx.areas) == 1
    area = fx.areas[0]
    assert area.type == "db"
    assert area.number == 1
    assert area.size == 64
    assert bytes(area.data[:8]) == bytes.fromhex("0102030405060708")
    assert all(b == 0 for b in area.data[8:])
