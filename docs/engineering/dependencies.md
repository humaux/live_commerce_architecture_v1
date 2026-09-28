# Third-party dependency register

Every direct third-party dependency, why it was admitted, who imports it, and what was
rejected instead (PROCESS.md §5). Adding a module or npm package without a row here is a
review blocker. Transitive dependencies are governed by `go.sum` / `pnpm-lock.yaml`; the
generated `dependency-map.md` shows which packages import what.

Baseline rule (`AGENTS.md`, 架构 §3): Go stdlib + PostgreSQL first. No second transaction
engine, no Kafka, no Redis business queue, no service mesh, no vendor SDK where a documented
HTTP API and ~200 lines do the job (e.g. no `stripe-go`: `internal/integrations/psp/stripe`
speaks the REST API directly so every parameter is reviewed and golden-tested).

## Go modules (`go.mod`)

| Module | Version | Why | Imported by | Rejected alternatives |
| --- | --- | --- | --- | --- |
| `github.com/jackc/pgx/v5` | v5.11.0 | PostgreSQL driver + pool; native types, `COPY`, per-tx GUCs needed for RLS scope | nearly every `internal/*` package and `cmd/*` (see dependency-map) | `database/sql` + lib/pq (no pool control, maintenance mode); ORMs (hide SQL the contracts freeze) |
| `github.com/riverqueue/river` (+ `riverdriver/riverpgxv5`) | v0.40.0 | Durable jobs in the same PG transaction as the business write (outbox without a second system) | `internal/jobqueue`, `internal/payments`, `internal/checkout`, `internal/live`, `internal/integrations/{core,meta}`, `cmd/api` | Kafka/Redis queues (forbidden by ADR baseline); hand-rolled `SKIP LOCKED` table (reinventing retries/leases) |
| `github.com/coreos/go-oidc/v3` | v3.21.0 | OIDC ID-token verification for merchant login (JWKS, issuer, audience) | `internal/oidclogin` | Hand-written JWT verification (security risk); a hosted auth SDK (vendor lock-in) |
| `golang.org/x/oauth2` | v0.37.0 | Authorization-code + PKCE exchange for OIDC | `internal/oidclogin` | Hand-written token exchange |

## npm packages (`package.json`, `apps/*/package.json`)

| Package | Scope | Why | Used by |
| --- | --- | --- | --- |
| `next` 16.3.5 | admin, storefront | SSR/BFF for merchant admin and buyer storefront; BFF routes keep bearer tokens server-side | `apps/admin`, `apps/storefront` |
| `react`, `react-dom` 19.3.0 | admin, storefront | UI runtime required by Next | both apps |
| `@live-commerce/i18n` (workspace) | admin, storefront | Shared zh-CN / zh-TW / en message catalogs and locale routing | both apps |
| `@playwright/test` (dev) | root | Real-browser gates (Chromium) for every page | `tests/**`, `apps/*/tests` |
| `typescript` (dev) | root | `strict` typecheck of both apps | CI |
| `prettier` (dev) | root | Formatting only | dev tooling |
| `livekit-client` (dev) | root | Browser-side LiveKit publisher used only by the local R04 input probe | `scripts/dev/r04-local-input.mjs` |
| `@types/*` (dev) | apps | Type definitions | typecheck |

## External services (runtime, not packages)

| Service | Host | Called from | Contract |
| --- | --- | --- | --- |
| Stripe API | `api.stripe.com` | `internal/integrations/psp/stripe` only (SP20 source guard) | `contracts/stripe-psp-v1.md` |
| PAYUNi | per `contracts/payuni-wire-v1.md` | `internal/integrations/psp/payuni` only | `payuni-wire-v1.md` |
| Meta Graph API / webhooks | `graph.facebook.com`, inbound `/v1/meta/webhooks/*` | `internal/integrations/meta` | `meta-*-v1.md` |
| LiveKit (Egress / ingress) | configured per deployment | `internal/integrations/livekit`, `cmd/media-worker` | `livekit-*-v1.md` |
| OIDC identity provider | owner choice (deploy blocker B2) | `internal/oidclogin` | `merchant-identity-v1.md` |
