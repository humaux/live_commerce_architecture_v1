# Platform operator console (suspend / resume) — backend contract v1 (OPS-01B, migration 0143)

Status: implementation contract of the CLI-only platform console (owner ruling: no web console until more than ~10 merchants or non-technical operators). Evidence class REAL_PG (MOCK Graph/Stripe). Integrator freezes.

## 1. Surface
`cmd/platform-admin` (ops one-shot, run through `deploy/scripts/ops-admin.sh platform-admin ...`, integrator wiring), login role `commerce_platform_operator` (INHERIT TRUE / SET FALSE, one login, never a service), env `COMMERCE_PLATFORM_OPERATOR_DATABASE_URL`. No HTTP route.

| Subcommand | Effect |
|---|---|
| `store-suspend --store <uuid> --operator <n> --ticket <ref> --reason <code>` / `store-resume` | flips `control.stores.active` |
| `tenant-suspend --tenant <uuid> ...` / `tenant-resume` | flips `control.tenants.active`; per-store flags untouched (resume restores the exact previous per-store state) |
| `status --store <uuid> \| --tenant <uuid>` | read-only: tenant_active, per store active / serving (store AND tenant) |
| `audit [--since RFC3339] [--limit 1..500]` | newest first, `control.read_operator_audit` |

`--operator` is `[a-z0-9._-]{1,40}`, `--ticket` 1..80 printable, `--reason` one of `fraud|non_payment|legal|owner_request|other` (suspend only). Re-applying the target state prints `"result":"unchanged"`, exits 0 and is still audited. stdout is one JSON line, stderr one fixed code (`platform_admin_usage|config|database|not_found|denied|failed`); a DSN, driver text or flag value is never printed. All input is validated before any connection opens and again in SQL; the 30 s run deadline bounds connect and query.

## 2. Audit
`control.operator_audit` is append-only (UPDATE/DELETE/TRUNCATE rejected by trigger for every role, the migration owner included), written only by the two `set_*_active` definers in the same transaction as the flag, read only through `read_operator_audit`. It records `operator` (self-asserted), `db_user` (= `session_user`, not a parameter), action, tenant, store, ticket, reason, `detail {changed, was_active}`. No FK (survives erasure). OPS-02B (support grants) widens the action CHECK and reuses it.

Known limits (v1, accepted): `--operator` is a self-asserted label over one shared login (the DB login is the only authenticated identity; per-person logins are a follow-up); refused attempts (validation, unknown target, denied login) are not audited.

## 3. Semantics: what stops, what does not
Suspension never deletes data, never unbinds a PSP/Meta binding, refunds nothing and force-closes no live claim window.

Stops immediately (the existing `active` joins, untouched): merchant API (`identity.resolve_access`, existing refusal code — **not** a new `store_suspended` code), buyer capabilities (`buyer.resolve_scope`, `issue_capability`), the published-store resolver, every stored-principal Check (`identity.principal_holds`: claim link replies `claims.check_meta_reply` → `principal_revoked`, CAPI, ads).

Added by 0143 (all through `control.store_serving(tenant, store)`): no automatic claim reply is planned (`integration.claim_reply_plannable` skip `store_suspended`; the claim is still recorded and the bundle is flagged `link_pending_manual` so the merchant is prompted after resume); `inbox.check_send` refuses every origin (auto included) → operation `BLOCKED_POLICY` / `store_suspended`, zero HTTP; `notify.claim_batch` claims no buyer/merchant mail for the store **except `paid` and `refunded` buyer confirmations** (money already in flight is confirmed); `notify.claim_merchant_alerts` claims nothing; `live.comment_poll_sources` omits the store's sources (no Graph polling with the merchant token). Blocked mail rows stay PENDING (the existing 24 h stale rule applies).

Still runs while suspended (money in flight, not gated): PSP notifies, payment query/reconcile, Stripe webhook receipt and capture/refund settlement, refund worker, expiry/stock release, `notify.record_result`. Pinned by `TestK3W401BDisabledStoreStillRecordsAndWakes` (PAYUNi) and `TestPlatformOperatorOP04b*` (Stripe webhook capture, refund settlement, definer-source pin).

## 4. Refund runbook (ruling: v1 = runbook, no code exemption)
The merchant refund API is a merchant action and is refused while the store is suspended. To refund a buyer of a suspended store: the operator issues the refund in the Stripe dashboard (test/live per environment) with the order's PaymentIntent; the Stripe refund webhook is received and recorded by the unchanged ingest/settlement definers (they do not read the suspension flags). For a refund already requested before the suspension, the worker settles it normally. If the refund must go through the product's own refund job, resume the store first, refund, then suspend again.

## 5. Other documented boundaries (no code)
- Queued CVS label and live-media operations created before the suspension are in-flight and still complete.
- Denial codes surface through the existing checks (e.g. `principal_revoked` for a claim link reply), not as a new code.
- CAPI events that would have been sent during the suspension are lost (no replay).
- Ads drafts mid-publish stay BLOCKED while suspended.
- The buyer storefront for a suspended store answers the generic not-published page, order lookup included; a 「暫停營業」 page is a UI follow-up (needs a resolver signal).
- Tenant resume does not resume individually suspended stores (by design).

## 6. Deploy wiring (integrator)
Login row `lc_platform_operator → commerce_platform_operator (inherit_noset, core, platform-admin:COMMERCE_PLATFORM_OPERATOR_DATABASE_URL)` in `deploy/postgres/logins.tsv`; provision-logins EXECUTE check (4 definers); secrets manifest `pw_/dsn_lc_platform_operator`; compose service `platform-admin` (profile ops, mounts only that DSN); `ops-admin.sh platform-admin` branch; smoke "mounts a registrar DSN" list.
