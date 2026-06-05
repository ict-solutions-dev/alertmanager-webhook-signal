# Alertmanager Webhook Signal

[![CI](https://github.com/ict-solutions-dev/alertmanager-webhook-signal/actions/workflows/ci.yml/badge.svg)](https://github.com/ict-solutions-dev/alertmanager-webhook-signal/actions/workflows/ci.yml)
[![Docker Image Publish](https://github.com/ict-solutions-dev/alertmanager-webhook-signal/actions/workflows/docker.yml/badge.svg)](https://github.com/ict-solutions-dev/alertmanager-webhook-signal/actions/workflows/docker.yml)
[![GitHub release](https://img.shields.io/github/v/release/ict-solutions-dev/alertmanager-webhook-signal)](https://github.com/ict-solutions-dev/alertmanager-webhook-signal/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A small Go service that exposes an HTTP webhook endpoint for [Prometheus Alertmanager](https://prometheus.io/docs/alerting/latest/configuration/#webhook_config)
(and [Grafana](https://grafana.com/)) and forwards each alert to the [signal-cli-rest-api](https://github.com/bbernhard/signal-cli-rest-api)
`/v2/send` endpoint — delivering your alerts straight into Signal chats and groups.

We use it to replace Slack notifications in Alertmanager, routing alerts to Signal groups through our existing `signal-api` container.

## Fork Notice

> This is a **fork** of [`schlauerlauer/alertmanager-webhook-signal`](https://github.com/schlauerlauer/alertmanager-webhook-signal)
> by Janek Lauer, maintained by [ICT Solutions](https://github.com/ict-solutions-dev) and adapted for our deployment conventions.
>
> The application logic is upstream's. Our changes are operational: a multi-stage Go Dockerfile producing a static
> non-root image, an entrypoint that generates the configuration from environment variables (with Docker / Swarm
> secret support), a GHCR publishing pipeline (hadolint → build → Trivy → SBOM → provenance attestation), and
> deployment examples. The original MIT license and copyright are retained — see [LICENSE](LICENSE).

## How It Works

The bridge listens for webhook POSTs, renders each alert through a Go template, and sends one Signal message per recipient.

**Routing is driven by the `recipients` label on each alert — not by Alertmanager receivers.** This is the key difference
from a Slack-style setup: instead of one Alertmanager receiver per channel, you use a **single** `signal` receiver and let
the bridge fan out based on labels:

1. An alert carries a label such as `recipients: proxmox` (or a comma-separated list `recipients: "proxmox,critical"`).
2. The bridge looks each name up in the recipients map (see [Configuration](#configuration)) and resolves it to a Signal recipient.
3. Alerts **without** a `recipients` label go to the default `signal.recipients` group.

Signal groups are addressed as the recipient string `group.<ID>` (the same group IDs used elsewhere, e.g. LibreNMS).

## Quick Start

The container builds its configuration **from environment variables** at startup (no config file needed):

```bash
docker run -d \
  --name alertmanager-webhook-signal \
  -p 10000:10000 \
  -e SIGNAL_NUMBER="+421918533760" \
  -e SIGNAL_SEND="http://signal-api:8080/v2/send" \
  -e SIGNAL_RECIPIENTS="group.DEFAULT_ID" \
  -e RECIPIENT_PROXMOX="group.PROXMOX_ID" \
  -e RECIPIENT_CRITICAL="group.CRITICAL_ID" \
  ghcr.io/ict-solutions-dev/alertmanager-webhook-signal:edge
```

## Configuration

There are two ways to configure the bridge, handled by the [entrypoint](assets/01-init.sh):

1. **Environment variables (default).** The entrypoint generates `config.yaml` from the variables below at startup.
   Every variable also supports a `<NAME>__FILE` suffix that reads the value from a file — for **Docker / Swarm secrets**.
2. **Static file (optional).** Mount a ready config file at `/config.yaml` (read-only) and it is used as-is, skipping
   generation. See [`examples/config.yaml`](examples/config.yaml) for the full schema.

If `/config.yaml` is present it always wins; otherwise the environment variables are used.

### Environment Variables

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `SIGNAL_NUMBER` | ✅ | — | Signal number messages are sent from (registered in signal-api) |
| `SIGNAL_SEND` | ✅ | — | signal-cli-rest-api `/v2/send` endpoint (e.g. `http://signal-api:8080/v2/send`) |
| `SIGNAL_RECIPIENTS` | ✅ | — | Default recipient(s) for alerts without a `recipients` label (comma-separated) |
| `SIGNAL_TEXTMODE_NORMAL` | | `false` | `false` = `text_mode: styled` (markdown), `true` = `normal` |
| `RECIPIENT_<NAME>` | | — | Maps a `recipients` label value to a Signal recipient, e.g. `RECIPIENT_PROXMOX=group.<ID>` → `recipients: proxmox`. The name is lowercased. |
| `SERVER_PORT` | | `10000` | Port the service listens on |
| `SERVER_INTERFACE` | | `0.0.0.0` | Bind interface |
| `SERVER_DEBUG` | | `false` | Verbose payload logging |
| `ALERT_IGNORE_LABELS` | | `alertname,recipients` | Labels stripped from the rendered message (comma-separated) |
| `ALERT_IGNORE_ANNOTATIONS` | | — | Annotations stripped from the rendered message (comma-separated) |
| `ALERT_GENERATOR_URL` | | `true` | Include the generator URL (link to the firing rule) |
| `TEMPLATE_GRAFANA` / `TEMPLATE_ALERTMANAGER` | | built-in | Optional raw Go message templates |

Booleans accept `true/false`, `1/0`, `yes/no`, `on/off`. Signal groups are addressed as `group.<ID>`.

### Docker / Swarm Secrets

Append `__FILE` to any variable to read its value from a file (e.g. a Swarm secret mounted under `/run/secrets/`):

```yaml
environment:
  SIGNAL_NUMBER__FILE: /run/secrets/signal_number
  RECIPIENT_CRITICAL__FILE: /run/secrets/critical_group
secrets:
  - signal_number
  - critical_group
```

Setting both `VAR` and `VAR__FILE` is an error.

### Finding Signal group IDs

```bash
docker exec signal-api curl -s \
  "http://signal-api:8080/v1/groups/+421918533760" | jq '.[] | {name, id}'
```

## Docker Compose

The service must share a network with your `signal-api` container so it can reach it internally. A full example
with environment-based config, Swarm secrets, and [Traefik](https://traefik.io/) labels + IP allow-list lives in
[`examples/docker-compose.yml`](examples/docker-compose.yml).

```yaml
services:
  alertmanager-webhook-signal:
    image: ghcr.io/ict-solutions-dev/alertmanager-webhook-signal:edge
    container_name: alertmanager-webhook-signal
    restart: unless-stopped
    environment:
      SIGNAL_NUMBER: "+421918533760"
      SIGNAL_SEND: "http://signal-api:8080/v2/send"
      SIGNAL_RECIPIENTS: "group.DEFAULT_ID"
      RECIPIENT_PROXMOX: "group.PROXMOX_ID"
      RECIPIENT_CRITICAL: "group.CRITICAL_ID"
    networks:
      - signal
    # Exposed via Traefik with the same IP allow-list as signal-api — see examples/.

networks:
  signal:
    external: true
```

## Endpoints

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/alertmanager`, `/api/v3/alertmanager` | Alertmanager webhook receiver |
| `POST` | `/grafana`, `/api/v2/alert/grafana`, `/api/v3/grafana` | Grafana webhook receiver |
| `GET` | `/ping` | Liveness check (returns `pong`) |
| `GET` | `/version` | Returns the application version |

## Testing a Running Instance

Replace `BRIDGE_URL` with where the service is reachable (e.g. `http://localhost:10000` internally, or
`https://signal-alertmanager.example.com` through your reverse proxy). If it sits behind Basic Auth, add `-u USER:PASS`.

### Health checks

```bash
curl BRIDGE_URL/ping        # -> pong
curl BRIDGE_URL/version     # -> e.g. v0.1.0 (confirms which build is running)
```

Behind a reverse proxy, a `401` means Basic Auth failed and `403` means the source IP is not in the allow-list.

### Send a test alert

This posts a minimal Alertmanager payload and should produce a real Signal message:

```bash
curl -X POST BRIDGE_URL/alertmanager \
  -H 'Content-Type: application/json' \
  -d '{"alerts":[{"status":"firing","labels":{"alertname":"SignalBridgeTest","severity":"critical"},"annotations":{"message":"Test alert – if you see this in Signal, the bridge works."},"startsAt":"2024-01-01T00:00:00Z"}]}'
```

- No `recipients` label → delivered to the default group (`SIGNAL_RECIPIENTS`).
- Add `"recipients":"proxmox"` (or `critical`) to `labels` to target a mapped group.
- Set `"status":"resolved"` to test the resolved (✅) message.

A successful call returns **HTTP 200 with an empty body** — the confirmation is the message arriving in Signal.

### Send a test Grafana alert

The `/grafana` endpoint accepts Grafana's classic webhook format. Grafana alerts always go to the default group
(`SIGNAL_RECIPIENTS`) — the `recipients` label routing applies to Alertmanager only.

```bash
curl -X POST BRIDGE_URL/grafana \
  -H 'Content-Type: application/json' \
  -d '{"title":"Grafana test","ruleName":"SignalBridgeGrafanaTest","state":"alerting","message":"Test from Grafana – if you see this in Signal, it works.","ruleUrl":"https://grafana.example.com/d/abc"}'
```

`"state":"alerting"` renders ❗, any other state renders ✅. An `imageUrl` field, if present, is downloaded and attached.

### If no message arrives

1. Check the container logs — the bridge logs `WARN error sending signal message` with the status code returned by
   signal-cli-rest-api:
   ```bash
   docker logs <container>            # or: docker service logs <stack>_alertmanager-webhook-signal
   ```
   Run with `SERVER_DEBUG=true` to also log the outgoing payload.
2. Common causes: wrong group ID in `SIGNAL_RECIPIENTS`/`RECIPIENT_*`, the `signal.number` not registered in
   signal-cli-rest-api, or the bridge not sharing a network with `signal-api`.
3. Verify the group IDs against signal-cli-rest-api:
   ```bash
   docker exec <signal-api> curl -s "http://signal-api:8080/v1/groups/<NUMBER>" | jq '.[] | {name, id}'
   ```

## Alertmanager & Prometheus

Configure a **single** webhook receiver and route everything to it; the bridge handles per-group delivery via the
`recipients` label on your alerting rules. Examples are in [`examples/`](examples/):

```yaml
# alertmanager.yml
route:
  receiver: "signal"
receivers:
  - name: "signal"
    webhook_configs:
      - url: "http://alertmanager-webhook-signal:10000/alertmanager"
        send_resolved: true
```

```yaml
# prometheus rule — routes to the Signal "proxmox" group
- alert: "ProxmoxNodeDown"
  labels:
    recipients: "proxmox"
  expr: "..."
```

## Image Tags

Images are published to the [GitHub Container Registry](https://github.com/ict-solutions-dev/alertmanager-webhook-signal/pkgs/container/alertmanager-webhook-signal).

| Tag | Source | Example |
| --- | --- | --- |
| `edge` | `develop` branch | `ghcr.io/ict-solutions-dev/alertmanager-webhook-signal:edge` |
| `{version}` | Git tag (`v*`) | `ghcr.io/ict-solutions-dev/alertmanager-webhook-signal:1.1.1` |

Built for `linux/amd64` and `linux/arm64` as a static (CGO-free) binary on a minimal Alpine base.

## Versioning & Releases

Releases follow [Conventional Commits](https://www.conventionalcommits.org/) via
[Release Drafter](https://github.com/release-drafter/release-drafter):

- PR titles are auto-labelled (`feat`, `fix`, `chore`, `refactor`, `docs`, `test`, `breaking`) and each merge into
  `develop` updates a **draft release** with the next version (`breaking` → major, `feat` → minor, else → patch).
- To cut a release, publish the draft in the GitHub UI — that creates the `vX.Y.Z` tag, which triggers the Docker
  pipeline to publish `ghcr.io/...:X.Y.Z`.

The version reported by `/version` is **injected at build time** (`-ldflags -X main.appVersion`), so nothing is
committed back to the repo and `develop` stays strictly protected:

- **Release** (tag `vX.Y.Z`) → image `...:X.Y.Z`, version `vX.Y.Z`.
- **Edge** (push to `develop`) → single moving `...:edge` image; the version inside reflects the commit via
  `git describe` (e.g. `v1.1.1-4-gabc1234-edge`) and is also exposed as the `org.opencontainers.image.version` label.
- **Local build** (`go build`/`go run`, no ldflags) → falls back to the VCS revision Go embeds automatically
  (e.g. `dev-1a2b3c4d5e6f`, with `-dirty` for uncommitted changes).

## CI/CD Pipeline

The [GitHub Actions workflow](.github/workflows/docker.yml) runs on push to `develop`, version tags (`v*`), pull
requests (build-only), and manual dispatch. The pipeline:

1. **Lints** the Dockerfile with [hadolint](https://github.com/hadolint/hadolint)
2. **Builds** multi-arch images (`linux/amd64`, `linux/arm64`) using Docker Buildx
3. **Pushes** to GHCR (skipped for pull requests)
4. **Scans** for vulnerabilities with [Trivy](https://github.com/aquasecurity/trivy) (results in the Security tab)
5. **Generates** an SBOM in SPDX format via [Anchore](https://github.com/anchore/sbom-action)
6. **Attests** build provenance with [GitHub Attestations](https://github.com/actions/attest-build-provenance)

## Development

Dependencies are vendored, so a network-free build works out of the box:

```bash
go build -mod=vendor ./...
go vet -mod=vendor ./...
go test -mod=vendor ./...

# Lint (https://golangci-lint.run)
golangci-lint run ./...

# Run locally against a config file
CONFIG_PATH=./config.yaml go run -mod=vendor .
```

CI ([`.github/workflows/ci.yml`](.github/workflows/ci.yml)) runs `gofmt`, `go vet`, `go test -race`,
[golangci-lint](https://golangci-lint.run), and [ShellCheck](https://www.shellcheck.net) on every push and pull request.

## Security

- Runs as a **non-root** user (UID/GID `35505`) on a minimal Alpine base; static CGO-free binary
- Trivy vulnerability scanning on every build
- SBOM generation for supply-chain transparency
- Build provenance attestation via Sigstore
- Dependabot enabled for Go modules, the Docker base image, and GitHub Actions

See [SECURITY.md](SECURITY.md) for the vulnerability disclosure policy.

## Contributing

This repository enforces [Conventional Commits](https://www.conventionalcommits.org/) for PR titles. See
[CONTRIBUTING.md](CONTRIBUTING.md) for the workflow and [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) for community guidelines.

## License

[MIT](LICENSE) — Copyright (c) 2020 Janek Lauer, Copyright (c) 2026 ICT Solutions s.r.o.

Original project: [`schlauerlauer/alertmanager-webhook-signal`](https://github.com/schlauerlauer/alertmanager-webhook-signal).
