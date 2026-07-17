"""Thin internal adapter around the pure-Python python-snap7 Server API."""

from __future__ import annotations

from snap7.server import Server
from snap7.type import Parameter, SrvArea

from snap_interop_server.fixture import AreaBuffer, CompiledFixture


class Snap7ServerAdapter:
    """Isolate upstream python-snap7 calls from the rest of the adapter."""

    def __init__(self) -> None:
        # Disable upstream's own event logger; we emit repository JSON logs.
        self._server = Server(log=False)

    def configure(self, listen_address: str, port: int, pdu_size: int) -> None:
        self._server.set_param(Parameter.LocalPort, port)
        if pdu_size > 0:
            self._server.set_param(Parameter.PDURequest, pdu_size)
        self._listen_address = listen_address
        self._port = port

    def register_fixture_areas(self, fixture: CompiledFixture) -> None:
        for area in fixture.areas:
            self.register_area(area)

    def register_area(self, area: AreaBuffer) -> None:
        srv_area = _area_type(area.type)
        index = 0
        if area.type == "db":
            if area.number is None or area.number < 1 or area.number > 65535:
                raise ValueError("invalid DB number")
            index = area.number
        # python-snap7 keeps this bytearray for the server lifetime.
        rc = self._server.register_area(srv_area, index, area.data)
        if rc != 0:
            raise RuntimeError(f"register_area failed with code {rc}")

    def start(self) -> None:
        rc = self._server.start_to(self._listen_address, self._port)
        if rc != 0:
            raise RuntimeError(f"start_to failed with code {rc}")

    def stop(self) -> None:
        self._server.stop()


def _area_type(area_type: str) -> SrvArea:
    mapping = {
        "db": SrvArea.DB,
        "markers": SrvArea.MK,
        "inputs": SrvArea.PE,
        "outputs": SrvArea.PA,
    }
    try:
        return mapping[area_type]
    except KeyError as exc:
        raise ValueError(f"unsupported area type {area_type!r}") from exc
