# snap-interop Python server

Pure-Python S7 server adapter built on pinned `python-snap7`.

## Contract

Same runtime environment variables as the native image:

```text
SNAP_INTEROP_FIXTURE
SNAP_INTEROP_LISTEN_ADDRESS
SNAP_INTEROP_PORT
SNAP_INTEROP_LOG_FORMAT
SNAP_INTEROP_STATE_PATH
```

Upstream API usage is isolated in `snap_interop_server.adapter.Snap7ServerAdapter`.
The image must not load the native Snap7 shared library (enforced at build and in tests).

## Local checks

```bash
python -m venv .venv
source .venv/bin/activate
pip install -r requirements-dev.txt
pip install -e .
pytest
ruff check .
mypy
```
