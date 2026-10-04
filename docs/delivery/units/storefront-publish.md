# Unit storefront-publish — production writer for storefront publication + domain binding (R3 P0)

Role: commerce_worker (mid tier). Base `896bf24`. Worktree `.worktrees/storefront-publish`, branch
`unit/storefront-publish`. No delegation, no new dependency. Migration number **0081** (integrator-assigned).

**Why (R3 readiness audit, output/r3-readiness/REPORT.md gap 1, P0):** `control.storefront_publications`
and `control.storefront_domains` (migrations/0020, contracts/published-storefront-resolver-v1.md) have no
production writer; only tests seed them as the DB owner. In the deployed pilot every buyer path is dead:
storefront 404, purchase-entry, claim links (`StudioClaims.tsx` "no-origin"), Meta private-reply cart link.
The resolver contract says production stays disabled "until an audited ownership/TLS/publication writer"
exists. This unit is that writer, in the smallest form that is safe for the pilot.

## Decisions (integrator rulings, binding)
- SP1 Two separate consents. **Publication** (store visible to buyers) is the merchant's act: an
  authenticated merchant with `store:write`-equivalent permission (reuse the narrowest existing
  permission that settings writes already require; grep the settings wizard routes) toggles it from
  admin Settings. **Domain binding** is the platform operator's act (ownership/TLS proof is the
  platform's, not the merchant's): operator CLI only, never a merchant HTTP route.
- SP2 Domain lifecycle is the contract's: REQUESTED → OWNERSHIP_PENDING → TLS_PENDING → ACTIVE →
  SUSPENDED → DETACHED. The operator CLI offers exactly: `domain-bind --store <uuid> --origin
  https://host --evidence <ref>` (creates or advances to ACTIVE in one audited call, stamping the three
  timestamps; `valid_until` = TLS cert notAfter supplied by `--valid-until` RFC3339, required),
  `domain-suspend`, `domain-detach` (by origin). A DETACHED origin is never re-bound by update: a new
  row with renewed proof (contract rule). Origin validated in Go and SQL exactly as the contract's
  origin grammar (reuse the existing Go validator in internal/storefront or buyer resolver — grep
  `resolve_published_store` callers — do not write a second one).
- SP3 Pilot fact: Caddy serves exactly one store host (`LC_STORE_HOST`), so one ACTIVE origin per
  deployment today. Do not build wildcard/on-demand TLS. Leave a `ponytail:` comment at the CLI naming
  the ceiling (one store host per deployment; per-store hosts need Caddy on_demand_tls + an ask
  endpoint backed by the resolver).
- SP4 All writes go through new SECURITY DEFINER functions in 0081 (owner = the existing control-plane
  owner role pattern used by 0065 owner provisioning; `search_path=pg_catalog`, fully qualified,
  PUBLIC EXECUTE revoked), each bumping `version`, each inserting one `ops.audit_events` row
  (`merchant.storefront_published|unpublished`, `operator.domain_bound|suspended|detached`), and each
  invalidating whatever the resolver caches (grep `revalidate.go` in internal/storefront). EXECUTE:
  merchant toggle → the merchant runtime login that serves settings routes; domain functions → a
  registrar-style login used only by the ops one-shot container (follow how `meta-admin` gets its
  registrar login in deploy/compose.yml + provision-logins; add the new login to
  deploy/secrets.manifest.tsv and secrets-init the same way).
- SP5 Operator CLI: add subcommands to a new `cmd/store-admin` (same shape as cmd/meta-admin: doc.go
  header, JSON result line with IDs/versions only, exit codes). Wire `ops-admin.sh store-admin
  domain-bind|domain-suspend|domain-detach|status` (allowlist + usage header), compose service
  `store-admin` in profile `ops`, `status` prints store id, published, ACTIVE origin(s) (no PII).
- SP6 Admin UI: one card in Settings ("網店發佈 / 网店发布 / Storefront") showing publication state,
  bound ACTIVE origin (read-only, or "awaiting platform domain" when none), a publish/unpublish button
  with confirm. Three locales (zh-CN/zh-TW/en) via the existing copy pattern. BFF: extend the admin
  `[...resource]/route.ts` allowlist for `GET storefront` and `POST storefront/publication` only.
  Go routes `GET /v1/admin/stores/{store_id}/storefront`, `POST .../storefront/publication`
  `{published: bool, expected_version}` with optimistic concurrency (409 on stale version), scope from
  server auth only (AGENTS.md).
- SP7 Runbook: docs/runbooks/merchant-onboarding.md gets the exact two-step go-live (merchant publishes
  in Settings; operator runs `ops-admin.sh store-admin domain-bind ... --valid-until "$(cert notAfter)"`,
  with the one-liner to read notAfter via openssl). deploy smoke: add one case that runs
  `store-admin status` in the ops container (exit 0, JSON shape) — follow how S44 exercises meta-admin.

## Write paths
`migrations/0081_storefront_publish.sql`, `cmd/store-admin/**`, `internal/storefront/**` (only if the
writer Go code belongs there; otherwise a new `internal/storefrontadmin` with package doc),
`internal/httpapi/**` (route registration + handler for the two routes only), `apps/admin/components/
StorefrontSettings.tsx` (+ its CSS in settings.css), `apps/admin/app/[locale]/settings/page.tsx` (mount
only), `apps/admin/app/api/stores/[store]/[...resource]/route.ts` (allowlist only), `apps/admin/lib/**`
copy, `deploy/compose.yml`, `deploy/scripts/ops-admin.sh`, `deploy/scripts/smoke.sh`,
`deploy/secrets.manifest.tsv`, `deploy/scripts/secrets-init.sh` (only if the manifest needs code),
`docs/runbooks/merchant-onboarding.md`, `contracts/published-storefront-resolver-v1.md` (append a
"Writer (R3)" section, status line), `docs/engineering/dependencies.md` + regenerated depmap.

## Done when
`go build ./... && go vet ./...`, gofmt, `bash scripts/dev/check-pkgdocs.sh`, `bash scripts/dev/depmap.sh
--check`, admin typecheck, `python3 scripts/check_packet.py` all exit 0; unit tests for pure Go logic;
author smoke of the SQL functions through `bash scripts/dev/test-focused.sh '<your regex>'` (one PG run at
a time machine-wide). Comment standard PROCESS.md §5 (owns / never / Depends on / Used by; every SQL
call names table, role and why). Evidence → `/Volumes/data/live_commerce_architecture_v1/output/
storefront-publish/`. Commit on your branch; do not merge.

## Non-goals
Custom merchant domains, DNS automation, wildcard TLS, per-store legal pages, unpublish side effects on
open carts (existing resolver behaviour applies).
