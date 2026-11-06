"""Interop-tuned python-snap7 Server subclass.

Stock python-snap7 pads out-of-range reads and overwrites whole bytes on bit
writes. This adapter aligns those surfaces with native Snap7 for fixture gates.
"""

from __future__ import annotations

import logging
import struct
import time
from typing import Any

from snap7.datatypes import S7Area, S7WordLen
from snap7.s7protocol import S7Function, S7PDUType
from snap7.server import Server
from snap7.type import SrvArea, SrvEvent

logger = logging.getLogger(__name__)

# Item return codes (data section), matching Snap7 / Siemens.
_RC_SUCCESS = 0xFF
_RC_OUT_OF_RANGE = 0x05
_RC_NOT_AVAILABLE = 0x0A

_SRV_TO_S7 = {
    SrvArea.PE: S7Area.PE,
    SrvArea.PA: S7Area.PA,
    SrvArea.MK: S7Area.MK,
    SrvArea.DB: S7Area.DB,
    SrvArea.CT: S7Area.CT,
    SrvArea.TM: S7Area.TM,
}


class InteropServer(Server):
    """Server with Snap7-aligned OOR, bit, and read-only behavior."""

    def __init__(self, log: bool = True) -> None:
        super().__init__(log=log)
        # (S7Area, db_number) -> "read-only" | "read-write"
        self.area_access: dict[tuple[S7Area, int], str] = {}

    def register_area_with_access(
        self, area: SrvArea, index: int, data: bytearray, access: str
    ) -> int:
        rc = self.register_area(area, index, data)
        if rc == 0:
            s7_area = _SRV_TO_S7[area]
            self.area_access[(s7_area, index)] = access
        return rc

    def _read_from_memory_area(
        self, area: S7Area, db_number: int, start: int, count: int
    ) -> tuple[int, bytearray]:
        """Return the item return code and the data, as python-snap7 3.2 does."""
        area_key = (area, db_number)
        if area_key not in self.memory_areas:
            return _RC_NOT_AVAILABLE, bytearray()
        with self.area_locks[area_key]:
            area_data = self.memory_areas[area_key]
            if start < 0 or count < 0 or start + count > len(area_data):
                return _RC_OUT_OF_RANGE, bytearray()
            return _RC_SUCCESS, bytearray(area_data[start : start + count])

    def _write_to_memory_area(
        self, area: S7Area, db_number: int, start: int, write_data: bytearray
    ) -> bool:
        area_key = (area, db_number)
        if area_key not in self.memory_areas:
            return False
        if self.area_access.get(area_key, "read-write") == "read-only":
            return False
        with self.area_locks[area_key]:
            area_data = self.memory_areas[area_key]
            if start < 0 or start + len(write_data) > len(area_data):
                return False
            area_data[start : start + len(write_data)] = write_data
            return True

    def _handle_read_area(self, request: dict[str, Any], client_address: tuple[str, int]) -> bytes:
        try:
            addr_info = self._parse_read_address(request)
            if not addr_info:
                return self._item_error_read_response(request, _RC_OUT_OF_RANGE)

            area, db_number, start, count = addr_info
            params = request.get("parameters") or {}
            addr_spec = params.get("address_spec") or {}
            word_len = addr_spec.get("word_len", S7WordLen.BYTE)

            if word_len == S7WordLen.BIT:
                return self._handle_bit_read(request, area, db_number, addr_spec)

            code, read_data = self._read_from_memory_area(area, db_number, start, count)
            if code != _RC_SUCCESS:
                return self._item_error_read_response(request, code)

            data_len = 4 + len(read_data)
            header = struct.pack(
                ">BBHHHHBB",
                0x32,
                S7PDUType.ACK_DATA,
                0x0000,
                request["sequence"],
                0x0002,
                data_len,
                0x00,
                0x00,
            )
            parameters = struct.pack(">BB", S7Function.READ_AREA, 0x01)
            data_section = (
                struct.pack(">BBH", _RC_SUCCESS, 0x04, len(read_data) * 8) + read_data
            )
            if self.read_callback:
                event = SrvEvent()
                event.EvtTime = int(time.time())
                event.EvtSender = 0
                event.EvtCode = 0x00004000
                event.EvtRetCode = 0
                event.EvtParam1 = 1
                event.EvtParam2 = 0
                event.EvtParam3 = len(read_data)
                event.EvtParam4 = 0
                try:
                    self.read_callback(event)
                except Exception as exc:  # noqa: BLE001
                    logger.error("Error in read callback: %s", exc)
            return header + parameters + data_section
        except Exception as exc:  # noqa: BLE001
            logger.error("Error handling read request: %s", exc)
            return self._build_error_response(request, 0x8000)

    def _handle_bit_read(
        self,
        request: dict[str, Any],
        area: S7Area,
        db_number: int,
        addr_spec: dict[str, Any],
    ) -> bytes:
        # Address in Any-pointer is bit offset for BIT transport.
        bit_addr = int(addr_spec.get("start", 0)) * 8  # start already byte-converted upstream
        # Re-parse raw bit address from request parameters.
        raw = request.get("raw_parameters") or b""
        if len(raw) >= 14:
            address = struct.unpack(">I", b"\x00" + raw[11:14])[0]
            bit_addr = address
        byte_off = bit_addr // 8
        bit_idx = bit_addr & 0x07
        area_key = (area, db_number)
        if area_key not in self.memory_areas:
            return self._item_error_read_response(request, _RC_NOT_AVAILABLE)
        with self.area_locks[area_key]:
            data = self.memory_areas[area_key]
            if byte_off >= len(data):
                return self._item_error_read_response(request, _RC_OUT_OF_RANGE)
            bit_val = 1 if (data[byte_off] >> bit_idx) & 1 else 0
        payload = bytes([bit_val])
        header = struct.pack(
            ">BBHHHHBB",
            0x32,
            S7PDUType.ACK_DATA,
            0x0000,
            request["sequence"],
            0x0002,
            4 + len(payload),
            0x00,
            0x00,
        )
        parameters = struct.pack(">BB", S7Function.READ_AREA, 0x01)
        data_section = struct.pack(">BBH", _RC_SUCCESS, 0x03, 1) + payload
        return header + parameters + data_section

    def _handle_write_area(self, request: dict[str, Any], client_address: tuple[str, int]) -> bytes:
        try:
            addr_info = self._parse_write_address(request)
            if not addr_info:
                return self._item_error_write_response(request, _RC_OUT_OF_RANGE)

            area, db_number, start, count, write_data = addr_info
            params = request.get("parameters") or {}
            addr_spec = params.get("address_spec") or {}
            word_len = addr_spec.get("word_len", S7WordLen.BYTE)
            area_key = (area, db_number)

            if area_key not in self.memory_areas:
                return self._item_error_write_response(request, _RC_NOT_AVAILABLE)

            if self.area_access.get(area_key, "read-write") == "read-only":
                # Match Snap7 ResourceLess reject surface (0x0A).
                return self._item_error_write_response(request, _RC_NOT_AVAILABLE)

            if word_len == S7WordLen.BIT:
                return self._handle_bit_write(request, area, db_number, write_data)

            if start < 0 or start + len(write_data) > len(self.memory_areas[area_key]):
                return self._item_error_write_response(request, _RC_OUT_OF_RANGE)

            if not self._write_to_memory_area(area, db_number, start, write_data):
                return self._item_error_write_response(request, _RC_OUT_OF_RANGE)

            return self._item_success_write_response(request)
        except Exception as exc:  # noqa: BLE001
            logger.error("Error handling write request: %s", exc)
            return self._build_error_response(request, 0x8000)

    def _handle_bit_write(
        self,
        request: dict[str, Any],
        area: S7Area,
        db_number: int,
        write_data: bytearray,
    ) -> bytes:
        raw = request.get("raw_parameters") or b""
        if len(raw) < 14:
            return self._item_error_write_response(request, _RC_OUT_OF_RANGE)
        bit_addr = struct.unpack(">I", b"\x00" + raw[11:14])[0]
        byte_off = bit_addr // 8
        bit_idx = bit_addr & 0x07
        area_key = (area, db_number)
        with self.area_locks[area_key]:
            data = self.memory_areas[area_key]
            if byte_off >= len(data):
                return self._item_error_write_response(request, _RC_OUT_OF_RANGE)
            set_bit = bool(write_data) and (write_data[0] & 0x01) != 0
            if set_bit:
                data[byte_off] = data[byte_off] | (1 << bit_idx)
            else:
                data[byte_off] = data[byte_off] & ~(1 << bit_idx)
        return self._item_success_write_response(request)

    def _item_success_write_response(self, request: dict[str, Any]) -> bytes:
        header = struct.pack(
            ">BBHHHHBB",
            0x32,
            S7PDUType.ACK_DATA,
            0x0000,
            request["sequence"],
            0x0002,
            0x0001,
            0x00,
            0x00,
        )
        parameters = struct.pack(">BB", S7Function.WRITE_AREA, 0x01)
        return header + parameters + bytes([_RC_SUCCESS])

    def _item_error_write_response(self, request: dict[str, Any], code: int) -> bytes:
        header = struct.pack(
            ">BBHHHHBB",
            0x32,
            S7PDUType.ACK_DATA,
            0x0000,
            request["sequence"],
            0x0002,
            0x0001,
            0x00,
            0x00,
        )
        parameters = struct.pack(">BB", S7Function.WRITE_AREA, 0x01)
        return header + parameters + bytes([code & 0xFF])

    def _item_error_read_response(self, request: dict[str, Any], code: int) -> bytes:
        header = struct.pack(
            ">BBHHHHBB",
            0x32,
            S7PDUType.ACK_DATA,
            0x0000,
            request["sequence"],
            0x0002,
            0x0004,
            0x00,
            0x00,
        )
        parameters = struct.pack(">BB", S7Function.READ_AREA, 0x01)
        data_section = struct.pack(">BBH", code & 0xFF, 0x00, 0x0004)
        return header + parameters + data_section

    def _parse_data_section(self, data_section: bytes) -> dict[str, Any]:
        """Parse write/read data; fix bit-length flooring in upstream (1 bit → 0 bytes)."""
        if len(data_section) == 1:
            return {
                "return_code": data_section[0],
                "transport_size": 0,
                "data_length": 0,
                "data": b"",
            }
        if len(data_section) >= 4:
            return_code = data_section[0]
            transport_size = data_section[1]
            data_length = struct.unpack(">H", data_section[2:4])[0]
            if transport_size in (0x00, 0x09):
                actual_data = data_section[4 : 4 + data_length]
            else:
                nbytes = (data_length + 7) // 8
                actual_data = data_section[4 : 4 + nbytes]
            return {
                "return_code": return_code,
                "transport_size": transport_size,
                "data_length": data_length,
                "data": actual_data,
            }
        return {"raw_data": data_section}
