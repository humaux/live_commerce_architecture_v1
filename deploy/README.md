<!--
File: deploy/README.md
Purpose: index of the deployment package, quick start, status matrix, deploy-only dependency
  ledger, verification evidence summary, deviations from the design, and open blockers.
Runs as/in: documentation (read by operators, integrator, reviewers).
Reads env / secrets: none.
Used by: docs/runbooks/*.md link here; integrator copies the dependency ledger into
  docs/implementation/dependencies.md (I5).
Status: DESIGN. Nothing here has been deployed to production (owner approval required).
Change rules: update the ledger + status matrix with every digest bump or new file.
-->
# live-commerce deploy package (T22)

Single-region deployment on **one Linux host running Docker Compose**, with a documented 2-host
variant (DB on its own host). PostgreSQL 18 is the only transactional source of truth, and River
queues live inside it. There is no Kafka, Redis or service mesh (AGENTS.md).

> **Not HA.** 架构.md §4.2 describes Compose as a dev/repeatable-acceptance tool. Running
> production on it needs owner ADR **O1** (blocker B8). **No production deploy has been run.**
> A real deploy needs explicit owner approval.

Runbooks (Chinese): [deploy](../docs/runbooks/deploy.md) ·
[backup/restore](../docs/runbooks/backup-restore.md) · [incident](../docs/runbooks/incident.md)

## Quick start (single host, as root)

```sh
deploy/scripts/host-setup.sh                      # group 10500, dirs/modes, templates -> /etc/live-commerce
vi /etc/live-commerce/compose.env /etc/live-commerce/env/*.env   # hosts, flags, OIDC client id
deploy/scripts/build-images.sh                    # prints IMAGE_TAG (git sha12); put it in compose.env
deploy/scripts/secrets-init.sh                    # creates missing secrets only, never prints values
# owner secrets: replace __UNSET__ in secrets/commerce_oidc_client_secret (and meta apps if used)
deploy/scripts/preflight.sh --online              # P01-P16, names only
deploy/scripts/deploy.sh first                    # postgres -> migrate -> provision -> up -> checks
cp deploy/host/crontab.example /etc/cron.d/live-commerce   # backups + watchdog (edit paths)
```
Upgrade: `deploy.sh upgrade <tag>`. It takes a backup, opens a 503 window, runs migrate and provision, then starts the stack.
Rollback: `deploy.sh app-rollback <tag>` works only when the migration ledger is unchanged. Otherwise the answer is forward-fix.

## Topology

```
Internet ─80/443(+udp)─► edge-netns (pause) ── shared 127.0.0.1 ──┬ caddy :80/:443 (TLS, 4 hosts)
                                                                 ├ api :8080 (must be loopback)
                                                                 ├ admin :3100 (Next standalone)
                                                                 └ storefront :3200 (next start)
backend (internal) : postgres :5432 ◄── api, expiry-worker, meta-worker, payment-worker-* (+egress)
pgsocket volume    : postgres ◄── migrate (network none), provision-logins, pg-ops (network none)
```
Profiles: `db` (postgres, migrate, provision-logins), `app` (edge-netns, caddy, api, admin,
storefront, expiry-worker), `payments-sandbox`, `payments-live` (REAL MONEY), `meta`, `ops` (pg-ops).
The media worker is **not deployed**, because it is MOCK-only (`worker_env.go:174`).

## Files

| Path | What it is |
|---|---|
| `compose.yml` / `compose.two-host-db.yml` | Whole stack; 2-host override (NOT_RUN) |
| `docker/{go,admin,storefront,caddy}.Dockerfile` | `lc-go` (all Go binaries + migrate + lcentry), `lc-admin`, `lc-storefront`, `lc-caddy` |
| `tools/lcentry/` | Stdlib-only launcher: `*_FILE` secret files → env, then `execve`; loopback HTTP probe |
| `caddy/Caddyfile` | Edge routing/TLS: admin, shop, api (default-deny, `/healthz` only), hooks (`/v1/meta/webhooks/*`) |
| `postgres/postgresql.conf`, `pg_hba.conf` | Server settings + WAL archiving; superuser socket-only, services via scram |
| `postgres/logins.tsv` | Service LOGIN → one authority → grant shape → consumer (single source) |
| `postgres/provision-logins.sh` | Idempotent logins + verification matrix + TCP auth + readiness gates |
| `postgres/ops/*` | backup, basebackup, restore-dump, restore-pitr, verify.sql (run inside `pg-ops`) |
| `secrets.manifest.tsv` | Every secret file: kind, generator, consumers, rotation |
| `env/*.env.example` | `compose.env` (cross-service values) + per-service knob templates |
| `scripts/lib.sh` | Shared helpers (`lc_compose`, `lc_psql`, `lc_secret_scan`, env loader) |
| `scripts/host-setup.sh`, `secrets-init.sh`, `preflight.sh` | Host prep, secret generation, config validation |
| `scripts/build-images.sh`, `check-pins.sh` | Image build (sha12 tags, OCI labels); digest-pin guard |
| `scripts/deploy.sh`, `pg-ops.sh` | first / upgrade / app-rollback; DB operations wrapper |
| `scripts/watchdog.sh`, `collect-diagnostics.sh` | Cron health checks W1–W9; incident bundle (secret-scanned) |
| `scripts/smoke.sh`, `smoke-browser.mjs` | Acceptance `static` (S01–S06) / `full` (S07–S39) with evidence |
| `host/crontab.example` | Backup + watchdog schedule |

## Status matrix (2026-09-28)

| Area | Status | Evidence |
|---|---|---|
| Static package: syntax, shellcheck, pins, compose config (4 profile sets × 2 files), lcentry tests (96.7 %), Caddyfile validate/fmt, ignore files | **PASS** | `smoke.sh static` (S01–S06) |
| `smoke.sh full` | **BLOCKED** at S07: `cmd/migrate` missing (I1) | exits 3, evidence under `.evidence/` |
| DB layer + edge with **scratch** images: postgres (non-root, read-only, checksums, archiving), migrate ×2 over the socket, provision-logins ×2 (12 logins, matrix, TCP auth, readiness), superuser-over-TCP rejected, api/caddy healthy, worker ready tokens, Caddy internal-CA TLS, default-deny, webhook routing, 308 redirect, unknown Host not proxied, 503 + Retry-After window, hardening of 7 containers, no secret in inspect/logs, backup + restore into a new DB, WAL archiving, basebackup + `pg_verifybackup`, PITR drill (2 s), watchdog W2–W9, diagnostics bundle, graceful stop, clean teardown | **VERIFIED_LOCAL (scratch)** | Scratch `lc-go` built on the host from this tree plus the §7 `cmd/migrate` proposal (outside the worktree, not committed); real `lc-caddy` from `docker/caddy.Dockerfile`. This is not product acceptance |
| `lc-admin` / `lc-storefront` image builds, S20/S21/S34 | **NOT_RUN** | Build containers have no egress in the verification sandbox (pnpm/corepack need the registry) |
| `deploy.sh` first/upgrade/app-rollback end-to-end (S37–S39) | **NOT_RUN** | Needs all 4 images (blocked by I1 + Node builds) |
| Real ACME/DNS, OIDC login, PAYUNi sandbox/live, Meta webhooks, media, 2-host TLS, K8s, load, CVE scan | **NOT_RUN** | Need owner inputs (O2, O3, O11, B3–B6) |

## Deploy-only dependency ledger (integrator: copy into docs/implementation/dependencies.md, I5)

| Dependency | Version / pin | Where used | Why |
|---|---|---|---|
| Docker Engine | ≥ 25 (verified 29.3.1) | host | BuildKit cache mounts, `init`, sysctls |
| Docker Compose plugin | ≥ 2.24 (verified v5.1.1) | host | `depends_on.required: false`, profiles |
| bash, coreutils, openssl, curl, python3 | distro | `deploy/scripts/*` | Key generation, JSON/format checks, secret scan |
| golang | `1.27.1-trixie@sha256:4337…5183` | build stages | go.mod `go 1.27.1` |
| distroless static | `debian13:nonroot@sha256:e2e9…55e3` | `lc-go` runtime | CA certs + tzdata, UID 65532, no shell |
| node | `24.15.0-trixie-slim@sha256:291b…0eb7` | `lc-admin`, `lc-storefront` | Ledger-tested Node (O6: 24.21.0 bump candidate) |
| pnpm | 10.33.0 via corepack | Node builds | root `packageManager` |
| caddy | `2.11.4-alpine@sha256:6aed…cb2b` | `lc-caddy` | Edge TLS; `setcap -r` for non-root |
| pause | `registry.k8s.io/pause:3.10.1@sha256:278f…d24c` | `edge-netns` | Shared netns owner |
| postgres | `@sha256:4ef4…2280` (18.6) | postgres, provision-logins, pg-ops | Same pin as `scripts/dev/test-local.sh` |
| shellcheck (optional) | 0.11.0 | `smoke.sh` S01 | Script lint; NOT_RUN when absent |
| Playwright (repo pin 1.63.0) + Chromium | repo | `smoke-browser.mjs` (S34, optional) | Browser through the edge |

No new Go modules and no new npm packages. `lcentry` uses only the Go standard library.

## Deviations from `deploy-design.md` (all found by local verification or code facts)

1. **Media readiness gate informational by default** (`LC_REQUIRE_MEDIA_GATE=0`). `live.media_plan_ready()`
   pins `md5(pg_get_constraintdef())`, and PostgreSQL flattens the nested `AND` from `BETWEEN` on re-parse.
   So every DB restored from `pg_dump` reports the gate **false** (VERIFIED_LOCAL: `media_stop_budget`,
   `sessions_title_check`, `media_authorization_destinations_external_asset_id_check`). Payment, expiry and
   meta gates stay mandatory in provision-logins, verify.sql and watchdog W8. The migration fix is REQUIRES_INTEGRATOR.
2. PITR scratch server reads `max_connections` etc. from the backup's `pg_control`. Hot standby aborts below
   the primary's values.
3. `archive_command` succeeds on an identical, already-archived segment (`cmp`). This keeps the archiver
   from wedging after a crash.
4. `pitr-scratch` is mounted at `/var/lib/postgresql` **inside pg-ops only**. It inherits UID 999 ownership
   from the image. pg-ops never mounts pgdata.
5. Preflight rejects `LC_ACME_CA=` (empty). Caddy applies `{$VAR:default}` only when the variable is unset.
6. Helper functions are used because `--profile X` **replaces** `COMPOSE_PROFILES`: `lc_compose_with_ops`
   and `lc_compose_all`.
7. `producer | grep -q` is banned under `pipefail`, because SIGPIPE caused false negatives.
8. `lcentry` also rejects a secret that is empty after newline stripping.
9. `host-setup.sh` installs the env templates when they are missing and never overwrites them.

## Blockers and hand-offs

- **REQUIRES_INTEGRATOR** (outside `deploy/**`):
  - I1 `cmd/migrate`: blocks S07+ and any deploy.
  - I2 `/healthz` route in both Next apps.
  - I3 storefront `output:"standalone"`.
  - I4 `ErrLedgerAhead` + configurable Apply timeout.
  - I5 dependency ledger.
  - I6 evidence copy.
  - I7 postgres digest refresh.
  - I8 restore-stable `live.media_plan_ready()`.
- **Owner decisions:**
  - O1 Compose/non-HA ADR.
  - O2 IdP.
  - O3 off-host encrypted backups.
  - O4 host.
  - O5 feature phases.
  - O6 Node bump.
  - O7 HSTS preload.
  - O8 log/IP retention.
  - O9 WAF/rate limiting.
  - O10 off-host logs.
  - O11 CVE scanner.
- **Launch blockers:** B1–B8 (see `docs/runbooks/deploy.md` §0).
