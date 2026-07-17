# Security

These servers are **test infrastructure only**. Do not expose them to untrusted
networks. Do not publish host port 102 in examples or production deployments.

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
  capabilities where practical.
- Upstream Snap7 and `python-snap7` license obligations are tracked in
  `THIRD_PARTY_NOTICES.md` before the first release.
