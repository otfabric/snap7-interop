# Third-party notices

This file lists third-party software incorporated into `snap7-interop` server
images and tooling. It must remain present before any published release.

## Snap7 (native server image)

| Field | Value |
| --- | --- |
| Component | Snap7 (SCADACS fork) |
| Source | https://github.com/SCADACS/snap7 |
| Pin | `f6ff90317ca5d54250f4dcd29209689a74e26d82` (see `versions.yaml`) |
| License | LGPL-3.0-or-later |
| Used by | `ghcr.io/otfabric/snap-interop-native` (dynamically linked `libsnap7.so`) |

The LGPL-3.0 text is copied into the native runtime image at
`/usr/share/doc/snap7/lgpl-3.0.txt`. Corresponding upstream notices (`lgpl-3.0.txt`,
`gpl.txt`) ship in the pinned Snap7 source tree.

Snap7 is free software: you can redistribute it and/or modify it under the terms
of the GNU Lesser General Public License as published by the Free Software
Foundation, either version 3 of the License, or (at your option) any later version.

## python-snap7 (Python server image)

| Field | Value |
| --- | --- |
| Component | python-snap7 |
| Source | https://github.com/gijzelaerr/python-snap7 |
| Pin | `3.0.0` (see `versions.yaml`) |
| License | MIT |
| Used by | `ghcr.io/otfabric/snap-interop-python` (pure-Python; no native `libsnap7`) |

Copyright (c) 2013 Gijs Molenaar, Stephan Preeker

Permission is hereby granted, free of charge, to any person obtaining a copy of
this software and associated documentation files (the "Software"), to deal in
the Software without restriction, including without limitation the rights to
use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
the Software, and to permit persons to whom the Software is furnished to do so,
subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

## Base container images

Pinned by digest in `versions.yaml` and Dockerfiles:

- `debian:bookworm-slim` — Debian Free Software Guidelines; see
  https://www.debian.org/legal/licenses/
- `python:3.12-slim-bookworm` — Python Software Foundation License / Debian base

## Go module dependencies

Go tooling (`fixturectl`, `interop-probe`) vendors dependency license metadata
via generated SBOMs under `dist/sbom/` (`make sbom`). Notable direct dependency:

| Module | License |
| --- | --- |
| `github.com/santhosh-tekuri/jsonschema/v6` | Apache-2.0 |

## Exclusions

This repository does **not** redistribute Siemens proprietary software, firmware,
TIA Portal assets, or copyrighted standards text.
