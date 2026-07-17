# Security

These servers are **test infrastructure only**. They are not production PLC
simulators and must not be exposed to untrusted networks. Do not publish host
port 102 in examples or deployments.

Supported test surface and deferred topics are summarized in
[README.md](README.md) (“What you can test” / “Out of scope”).

## Reporting a vulnerability

If you believe you have found a security vulnerability in this project, please
report it responsibly.

- **Do not** open a public GitHub issue for security-sensitive bugs.
- Open a **private security advisory** on GitHub: **Security** → **Advisories** →
  **Report a vulnerability**.

Include a clear description, steps to reproduce, affected versions, and the
impact. We will acknowledge your report and work with you on a fix and
disclosure timeline.

## Scope notes

- Fixtures must not contain secrets or credentials.
- Fixture paths are validated to reject path traversal.
- Containers should run with a read-only root filesystem and minimal
  capabilities where practical (`cap_drop: ALL`, `cap_add: NET_BIND_SERVICE`,
  `no-new-privileges`).
- Upstream Snap7 and `python-snap7` license obligations are tracked in
  `THIRD_PARTY_NOTICES.md`.

## Release-blocking vulnerability policy

Image and dependency scans (`make scan`, CI `scan` / release workflows) use
[Trivy](https://github.com/aquasecurity/trivy). Severity gates:

| Context | Blocking severities | Action |
| --- | --- | --- |
| Pull request / image `scan` job | **CRITICAL** (fixable) | Fail the job; fix or document an accepted exception before merge |
| Release / published GHCR images | **CRITICAL** and **HIGH** (fixable) | Fail the release; do not publish |

Scans use Trivy `--ignore-unfixed`: base-image CVEs with no fixed package in the
distro are reported for awareness but do not block. Accepted exceptions for
*fixable* findings MUST be recorded in an ADR or release notes with owner,
expiry, and rationale.

Floating tags (`latest`) are not used for published images; release tags and
digests are immutable.
