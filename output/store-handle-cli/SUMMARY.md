# store-admin handle-set — SUMMARY

Backend-only operator subcommand `store-admin handle-set <store-uuid> <handle>`, backed by a
SECURITY DEFINER `control.operator_set_store_handle` in the unreleased migration 0106 (edited in place).

## What changed

- **migrations/0106_store_domains.sql** — `control.operator_set_store_handle(p_store, p_handle, p_after_publish)`.
  SECURITY DEFINER owned by `commerce_storefront_writer`, `EXECUTE` only `commerce_storefront_registrar`
  (never `commerce_worker`), same shape as `operator_bind_domain`. Validates format/reserved/xn--/uniqueness
  exactly like `assign_store_handle` (reuses `store_handle_valid`/`store_handle_reserved`/`store_handle_taken`).
  Refuses `PT409 store_published` when the store was ever published unless `--after-publish`. In the same
  transaction: `UPDATE control.stores SET handle` (fires `stores_handle_after_update` → old platform origin
  DETACHED) then `ensure_store_platform_domain` → new ACTIVE `https://<handle>.<base>`. Base from the
  `lc.store_base_domain` GUC; unset = `PT409 base domain not configured`. One `operator.handle_set` audit row,
  no secrets. New `GRANT UPDATE(handle)`, new UPDATE RLS policy `storefront_writer_stores_update`, audit-policy
  extension, and `EXECUTE` grants on the two validation helpers to `commerce_storefront_writer`.
- **internal/storefrontadmin/operator.go** — `HandleSet(ctx, Beginner, storeID, handle, afterPublish, baseDomain)`
  → `HandleSetResult`; new `Tx`/`Beginner` interfaces + `ErrStorePublished`. Runs the GUC in its own statement
  then the definer in one transaction.
- **cmd/store-admin** — `handle-set` positional subcommand, `--after-publish` flag, `poolBeginner` adapter,
  `LC_STORE_BASE_DOMAIN` read/lower-case before any connection; new fixed stderr codes wired in `mapFailure`
  (`store_published` → `store_admin_store_published`). Unit tests for the happy path, usage errors, and failure
  reduction.
- **deploy/scripts/ops-admin.sh** allowlist + **deploy/postgres/provision-logins.sh** registrar EXECUTE count 4→5.
- **docs/runbooks/merchant-onboarding.md** §5.1 — documents the command. dependency-map regenerated.

## Findings worth recording

1. **GUC propagation is statement-scoped, not transaction-scoped.** `set_config('lc.store_base_domain', v, true)`
   inside a MATERIALIZED CTE in a single autocommit statement is NOT visible to `current_setting` later in that
   same statement — PostgreSQL applies `set_config` only after the current statement finishes. The working idiom
   (used at 17 codebase sites) is an explicit transaction: `tx.Exec(SELECT set_config(...,true))` first, then
   `tx.QueryRow(definer)`. `HandleSet` uses this.
2. **Operator audit attribution needs an onboarding row.** The definer attributes `operator.handle_set` to
   `identity.initial_stores.principal_id`; direct-INSERT fixtures lack that row and get
   `PT409 store has no owner principal`. `SDW07` seeds `identity.initial_stores` + a warehouse for the fixture store.

## Gates

- `go build ./...`, `go vet ./...`, `gofmt -l` — clean.
- `bash scripts/dev/depmap.sh --check` — up to date.
- `bash scripts/dev/test-focused.sh '^(TestStoreDomains|TestStorefrontPublish|TestT06WorkerAuthorityAndFunctionACL|TestR2IntegrationUpgradeFromReleaseHead)'`
  — **33 PASS, 0 FAIL**, including `TestStoreDomainsSDW07OperatorHandleSet`, `SDW06` (ACL inventory) and `SPW10` (allowlist).
- `bash scripts/dev/check-gates.sh` — fails only on `Cannot find module 'typescript-api'` from
  `scripts/dev/ui-architecture-gate.mjs` (fresh worktree has no `node_modules`). Environmental, unrelated to the
  backend change; the one UI test (`tests/admin/shell-architecture.test.mjs`) fails on that missing module.
