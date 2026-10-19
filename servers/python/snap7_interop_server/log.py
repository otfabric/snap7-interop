"""JSON lifecycle logging matching the repository contract."""

from __future__ import annotations

import json
import sys
from datetime import UTC, datetime
from typing import Any

ADAPTER_VERSION = "0.1.0"


def log_event(
    level: str,
    event: str,
    fixture_id: str,
    **fields: Any,
) -> None:
    payload: dict[str, Any] = {
        "timestamp": datetime.now(UTC).strftime("%Y-%m-%dT%H:%M:%S.%f")[:-3] + "Z",
        "level": level,
        "implementation": "python",
        "event": event,
        "fixture_id": fixture_id,
        "adapter_version": ADAPTER_VERSION,
    }
    payload.update(fields)
    sys.stdout.write(json.dumps(payload, separators=(",", ": ")) + "\n")
    sys.stdout.flush()
