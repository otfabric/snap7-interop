"""Prove the Python runtime does not load a native Snap7 shared library."""

from __future__ import annotations

import ctypes.util
import importlib
import pathlib

import snap7


def test_find_library_snap7_is_none() -> None:
    assert ctypes.util.find_library("snap7") is None


def test_package_contains_no_native_extension() -> None:
    root = pathlib.Path(snap7.__file__).resolve().parent
    native = list(root.rglob("*.so")) + list(root.rglob("*.dylib")) + list(root.rglob("*.dll"))
    assert native == [], f"unexpected native libs in python-snap7 package: {native}"


def test_server_is_pure_python_module() -> None:
    mod = importlib.import_module("snap7.server")
    source = pathlib.Path(mod.__file__).read_text(encoding="utf-8")
    assert "Pure Python" in source or "pure Python" in source
    assert "ctypes.CDLL" not in source
    assert "cdll.LoadLibrary" not in source
