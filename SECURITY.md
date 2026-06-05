# Security Policy

## Supported Versions

| Version | Supported |
| --- | --- |
| Latest release (`v1.x.x`) | Yes |
| `edge` (develop) | Best effort |

## Reporting a Vulnerability

If you discover a security vulnerability in this project, please report it responsibly.

**Do not open a public GitHub issue for security vulnerabilities.**

Instead, please email: **support@ictsolutions.net**

Include:

- Description of the vulnerability
- Steps to reproduce
- Affected versions
- Potential impact

We will acknowledge your report within **48 hours** and aim to provide a fix or mitigation within **7 days** for critical issues.

## Security Measures

This project implements the following security practices:

- **Non-root container** — runs as user `35505:35505` on a minimal Alpine base image
- **Static binary** — built with `CGO_ENABLED=0`; no build toolchain in the final image
- **No static secrets in the image** — configuration is generated at startup from environment variables, with Docker / Swarm secret (`__FILE`) support
- **Vulnerability scanning** — [Trivy](https://github.com/aquasecurity/trivy) on every build, results in the GitHub Security tab
- **SBOM generation** — SPDX format for supply chain transparency
- **Build provenance** — attested via [GitHub Attestations](https://github.com/actions/attest-build-provenance)
- **Dependency updates** — Dependabot monitors Go modules, the Docker base image, and GitHub Actions

## Deployment Hardening

- The bridge has no authentication of its own. Expose it only on a trusted internal network, or behind a reverse
  proxy with an IP allow-list (see [`examples/docker-compose.yml`](examples/docker-compose.yml)).
- Keep `config.yaml` (which contains your Signal number and group IDs) out of public locations and version control.

## Disclosure Policy

We follow [coordinated vulnerability disclosure](https://en.wikipedia.org/wiki/Coordinated_vulnerability_disclosure).
We ask that you give us reasonable time to address the issue before public disclosure.
