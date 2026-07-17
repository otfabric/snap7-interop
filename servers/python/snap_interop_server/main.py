"""Entry point for the pure-Python snap-interop server."""

from __future__ import annotations

import os
import signal
import sys
import threading

from snap_interop_server.adapter import Snap7ServerAdapter
from snap_interop_server.fixture import load_compiled_fixture
from snap_interop_server.log import log_event
from snap_interop_server.state import RuntimeCounters, write_state_snapshot


def _env(name: str, default: str | None = None) -> str:
    value = os.environ.get(name, default)
    if value is None or value == "":
        raise SystemExit(f"{name} is required")
    return value


def main() -> None:
    fixture_path = _env("SNAP_INTEROP_FIXTURE")
    listen_address = os.environ.get("SNAP_INTEROP_LISTEN_ADDRESS", "0.0.0.0")
    port_s = os.environ.get("SNAP_INTEROP_PORT", "102")
    state_path = os.environ.get("SNAP_INTEROP_STATE_PATH", "/run/snap-interop/state.json")

    try:
        port = int(port_s)
    except ValueError as exc:
        raise SystemExit("invalid SNAP_INTEROP_PORT") from exc
    if port <= 0 or port > 65535:
        raise SystemExit("SNAP_INTEROP_PORT out of range")

    try:
        fixture = load_compiled_fixture(fixture_path)
    except Exception as exc:  # noqa: BLE001 - startup must report any load failure
        log_event("error", "fatal_error", "unknown", error=str(exc))
        raise SystemExit(1) from exc

    log_event(
        "info",
        "fixture_loaded",
        fixture.id,
        compiled_sha256=fixture.compiled_sha256,
    )

    counters = RuntimeCounters()
    try:
        write_state_snapshot(state_path, fixture, "starting", counters)
    except Exception as exc:  # noqa: BLE001
        log_event("error", "fatal_error", fixture.id, error=str(exc))
        raise SystemExit(1) from exc

    adapter = Snap7ServerAdapter()
    stop = threading.Event()

    def _handle_signal(signum: int, frame: object) -> None:
        del signum, frame
        stop.set()

    signal.signal(signal.SIGTERM, _handle_signal)
    signal.signal(signal.SIGINT, _handle_signal)

    try:
        adapter.configure(listen_address, port, fixture.pdu_size)
        adapter.register_fixture_areas(fixture)
        log_event(
            "info",
            "server_starting",
            fixture.id,
            listen_address=listen_address,
            port=port,
        )
        adapter.start()
        write_state_snapshot(state_path, fixture, "ready", counters)
        log_event("info", "server_ready", fixture.id, port=port)
    except Exception as exc:  # noqa: BLE001
        log_event("error", "fatal_error", fixture.id, error=str(exc))
        try:
            adapter.stop()
        except Exception:  # noqa: BLE001
            pass
        raise SystemExit(1) from exc

    while not stop.is_set():
        try:
            write_state_snapshot(state_path, fixture, "ready", counters)
        except Exception:  # noqa: BLE001 - diagnostics must not kill the server
            pass
        stop.wait(0.5)

    log_event("info", "server_stopping", fixture.id)
    adapter.stop()
    try:
        write_state_snapshot(state_path, fixture, "stopped", counters)
    except Exception:  # noqa: BLE001
        pass
    log_event("info", "server_stopped", fixture.id)


if __name__ == "__main__":
    main()
    sys.exit(0)
