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
Add the three 0153 definers (`identity.grant_support/revoke_support/list_support_grants`) to the provision-logins EXECUTE check and `support-grant|support-revoke|support-list` to the `ops-admin.sh platform-admin` allowlist. Login row `lc_platform_operator → commerce_platform_operator (inherit_noset, core, platform-admin:COMMERCE_PLATFORM_OPERATOR_DATABASE_URL)` in `deploy/postgres/logins.tsv`; provision-logins EXECUTE check (4 definers); secrets manifest `pw_/dsn_lc_platform_operator`; compose service `platform-admin` (profile ops, mounts only that DSN); `ops-admin.sh platform-admin` branch; smoke "mounts a registrar DSN" list.

## 7. Support grants (OPS-02B, migration 0153)
An operator gives ONE named principal limited-time, read-only access to ONE store: `platform-admin support-grant --store <uuid> --principal <uuid> [--hours 1..72, default 4] [--perm <subset>] --operator <name> --ticket <ref>`, `support-revoke --grant <uuid> | --store <uuid> --principal <uuid> --operator --ticket`, `support-list --store <uuid>`. The support person signs in with the ordinary merchant login; there is no impersonation and no new login method.

- **Pack.** `store:read, orders:read, catalog:read, inventory:read, live:read, integration:read` (a subset including `store:read` may be requested). Never `customers:*`, `inbox:*`, `payments:*`, any write; enforced by the table CHECK, the definer, the CLI and, in `identity.resolve_access`, by requiring a granted `:read` permission. PII access for support is an owner decision (SG-OPEN-1), not part of v1.
- **Default-deny.** The support branch of `identity.resolve_access` applies only when the principal holds **no** `store_grants` row for that store (a member, or a former member with lingering grants, never gets support access on top); the grant must be unrevoked and unexpired by `clock_timestamp()`, the store and tenant active (OPS-01B suspension also stops support; granting on a suspended store is refused, `PT409`). Expiry and revocation are evaluated on every request; there is no sweeper. One open grant per (store, principal); re-granting after expiry closes the expired row at its expiry.
- **Read-only twice more.** A support scope is recognised by `authz_revision = -1` (regular revisions are > 0); `internal/platform` then makes the transaction `transaction_read_only` (any write, in any definer, fails `25006` and is reported as `ErrSupportReadOnly`, which wraps `ErrForbidden`: HTTP 403 with the route's usual `forbidden` body; a distinct `support_read_only` body code is a per-handler follow-up). Definers that re-join an active membership inline instead of calling `identity.resolve_access` (the merchant order list/detail `identity.read_merchant_orders*`, the 0063/0073 `merchant_access_denied` fence, customers, CVS) refuse support principals by construction: **in v1 a support principal reaches only `resolve_access`-gated routes (store, session store list, and the read routes built on it); the order pages are NOT among them** (known limit, pinned by `TestSupportGrantSG01*`; serving them needs a conscious patch of those fences, tracked as a follow-up).
- **Audit.** Grant and revoke append `control.operator_audit` (`support_grant`/`support_revoke`, `detail` = grant id, principal, expiry, permissions) and the merchant-visible `ops.audit_events` (`support.granted` / `support.revoked`, `details` = grant id, expiry, operator, ticket). Use is audited by `internal/platform`: one `support.used` row per principal+store per 15 minutes (written before the transaction turns read-only; a request that fails rolls it back with the rest). `ops.audit_events.principal_id` references `identity.memberships`, so `grant_support` inserts an **inactive** placeholder membership when the principal has none in that tenant (`active=false` never satisfies a regular join).
- **Authority.** `identity.grant_support / revoke_support / list_support_grants`: SECURITY DEFINER, owner `commerce_platform_writer`, EXECUTE `commerce_platform_operator` only. The operator identity is the database login (`session_user` recorded as `db_user`), `--operator` stays a label (same known limit as §2). `--hours`/`--perm` are validated in the CLI and again in SQL; stderr adds the fixed code `platform_admin_conflict` (`PT409`).
- **Not in v1.** Merchant consent before a grant (SG-OPEN-2), support access to customer data (SG-OPEN-1), per-request merchant-visible use rows, a distinct HTTP body code, `authz_revision` bump on revoke (not needed: nothing is cached).
