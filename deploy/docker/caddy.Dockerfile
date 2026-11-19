# File: deploy/docker/caddy.Dockerfile
# Purpose: build "lc-caddy", the TLS edge, from the upstream stock modules with
#   patched dependencies. The binary has no file capabilities and runs as UID 10001.
#   Binding :80/:443 unprivileged works because the edge netns sets
#   net.ipv4.ip_unprivileged_port_start=0 (VERIFIED_LOCAL during design, deploy-design §22).
# Build: docker build -f deploy/docker/caddy.Dockerfile -t lc-caddy:<sha12> deploy/docker
#   (only the Caddy module lock is read; the operational Caddyfile is bind-mounted read-only).
# Runs as/in: service "caddy" in deploy/compose.yml, UID/GID 10001, read-only rootfs.
# Reads env (runtime): LC_ADMIN_HOST, LC_STORE_HOST, LC_API_HOST, LC_HOOKS_HOST (site names,
#   from compose.env), ACME_EMAIL and optional LC_ACME_CA (from env/caddy.env).
# Reads secrets: none (ACME account keys/certs live in the caddy-data volume at /data).
# Used by: deploy/compose.yml caddy; deploy/scripts/smoke.sh S05 (caddy validate/fmt), S25.
# Depends on: Caddy v2.11.9, Go 1.27.4, Alpine 3.24.4 (BusyBox wget for the healthcheck).
# Status: DESIGN (steps VERIFIED_LOCAL in design probe); verified by smoke S07, S08, S25.
# Change rules: digest bump -> smoke full; any plugin (rate limit/WAF) is a new dependency (O9).

ARG CADDY_IMAGE=caddy:2.11.9-alpine@sha256:003a97808e781aa9f7e75cc2f43d34d461099031faf5e20faefb9f61126836f0
ARG GO_IMAGE=golang:1.27.4-trixie@sha256:b54b2639de8c1bfe538ec5ffc45f29b1932235ac5f6d63c059853646677112d9
ARG ALPINE_IMAGE=alpine:3.24.4@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf
FROM ${CADDY_IMAGE} AS stock

FROM ${GO_IMAGE} AS build
ENV CGO_ENABLED=0 GOTOOLCHAIN=local
WORKDIR /src
COPY caddy-build/go.mod caddy-build/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    go mod download && go mod verify && \
    go build -mod=readonly -trimpath -buildvcs=false -o /out/caddy github.com/caddyserver/caddy/v2/cmd/caddy

FROM ${ALPINE_IMAGE}
# The fresh runtime contains neither the old Caddy binary nor the tools used to download it.
RUN apk add --no-cache ca-certificates=20261020-r0 mailcap=2.1.54-r0 \
       libcrypto3=3.5.9-r1 libssl3=3.5.9-r1 zlib=1.4-r0 \
 && addgroup -S -g 10001 lccaddy && adduser -S -D -H -u 10001 -G lccaddy lccaddy \
 && mkdir -p /data/caddy /config/caddy /run/caddy && chown -R 10001:10001 /data /config /run/caddy
COPY --from=stock /etc/caddy/Caddyfile /etc/caddy/Caddyfile
COPY --from=stock /usr/share/caddy/index.html /usr/share/caddy/index.html
COPY --from=build /out/caddy /usr/bin/caddy
ENV XDG_CONFIG_HOME=/config XDG_DATA_HOME=/data
EXPOSE 80 443 443/udp 2019
WORKDIR /srv
USER 10001:10001
CMD ["caddy", "run", "--config", "/etc/caddy/Caddyfile", "--adapter", "caddyfile"]
# Probes the loopback-only :2080 health site defined in deploy/caddy/Caddyfile.
HEALTHCHECK --interval=15s --timeout=3s --retries=4 CMD wget -q -O /dev/null http://127.0.0.1:2080/healthz || exit 1
ARG GIT_SHA=unknown
LABEL org.opencontainers.image.title="lc-caddy" \
      org.opencontainers.image.version="v2.11.9" \
      org.opencontainers.image.revision=$GIT_SHA \
      org.opencontainers.image.source="live_commerce_architecture_v1" \
      org.opencontainers.image.description="live-commerce TLS edge (Caddy, non-root, no file caps)"
