# syntax=docker/dockerfile:1

# ---- Build stage ----
FROM golang:1.25-alpine AS build

# Build a fully static binary from the vendored dependencies (no module downloads).
ENV CGO_ENABLED=0 \
    GOFLAGS=-mod=vendor

WORKDIR /src

COPY . .

ARG APP_VERSION=dev
RUN go build -trimpath -ldflags="-s -w" -o /out/alertmanager-webhook-signal .

# ---- Runtime stage ----
FROM alpine:3.21

LABEL org.opencontainers.image.source="https://github.com/ict-solutions-dev/alertmanager-webhook-signal" \
      org.opencontainers.image.description="Webhook bridge translating Alertmanager and Grafana alerts to signal-cli-rest-api." \
      org.opencontainers.image.title="Alertmanager Webhook Signal" \
      org.opencontainers.image.authors="Jozef Rebjak <jozef.rebjak@ictsolutions.net>" \
      org.opencontainers.image.vendor="ICT Solutions" \
      org.opencontainers.image.licenses="MIT"

# bash for the entrypoint, ca-certificates for outbound HTTPS (e.g. Grafana images).
# hadolint ignore=DL3018
RUN apk add --no-cache bash ca-certificates && \
    addgroup -g 35505 -S app && \
    adduser -u 35505 -S app -G app

COPY --from=build /out/alertmanager-webhook-signal /usr/local/bin/alertmanager-webhook-signal
COPY assets/01-init.sh /01-init.sh
RUN chmod +x /01-init.sh

USER 35505:35505

EXPOSE 10000

ENTRYPOINT ["/01-init.sh"]
