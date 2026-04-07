# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in dns-entree, please report it
privately. Do **not** open a public GitHub issue.

**Contact:** security@spoofcanary.com

Include:
- A description of the issue
- Steps to reproduce
- Affected version(s)
- Any proof-of-concept code or output

## Response SLA

- **Acknowledgement:** within 72 hours of receipt
- **Initial assessment:** within 7 days
- **Fix for high-severity issues:** within 30 days of confirmation
- **Public disclosure:** coordinated with the reporter after a fix ships

## Supported Versions

During the v0.x line, only the latest minor release receives security fixes.
Once v1.0.0 ships, the previous minor will also receive backports for one
release cycle.

| Version       | Supported          |
| ------------- | ------------------ |
| 0.1.x-alpha   | Yes (latest only)  |
| < 0.1.0       | No                 |

## Scope

In scope:
- The `dns-entree` library and `entree` CLI
- Provider integrations under `providers/`
- Domain Connect handling under `domainconnect/`

Out of scope:
- Vulnerabilities in upstream provider APIs (report to the provider directly)
- Issues in example code under `_examples/`
