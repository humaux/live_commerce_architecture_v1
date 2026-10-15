**Verdict: APPROVE**

Review of commit f02adcb3b ("bind identity login to the isolated fixture in merchant-orders browser gates"), worktree .worktrees/pay-rm2-p2a (PR #70). This was a read-only review. I did not run any Go tests, which is policy. Evidence level: E1 (static trace + vet/gofmt). The real-chain browser gates were NOT_RUN by me. CI on f02adcb3b is the acceptance signal.

## 1. Diagnosis: confirmed
- `browser_merchant_orders_{bff,ui}_test.go` get `q := rm2StripePending(t)` (bff:37, ui:57). That calls `rm2StripePendingOn(t, pwIsolatedFixture(t))` (payment_fixture_helpers_test.go:9-11), which starts a separate PG container (payment_runtime_test.go:40).
- The harness inserts `identity.external_identities(idp issuer,'browser-subject',principalA)` on `q.f.owner` (bff:92, ui:250) and serves `httpapi.NewHandler(q.f.runtime, …)` on the isolated cluster.
- Before the fix, `identityFixture(t)` opened the identity pool on the shared `fixture(t)` DB. `identity.issue_merchant_session` (migrations/0004_merchant_identity.sql:80-85) looks up `external_identities` there and finds no `browser-subject`. Because it auto-creates a new principal with an external identity, the login "succeeds" but the session lands in the shared DB, under a principal that has no store membership.
- So the Next cookie carries a token unknown to `q.f.runtime`. That gives the BFF 401 and the UI stuck on sign-in (no `nav-orders`). This matches both CI symptoms exactly. As a side effect, the old code also leaked a stray principal and session into the shared DB on every run. The fix removes that too.

## 2. identityFixtureOn on an isolated fixture: correct
- Role `commerce_identity` is created by migration 0004:2 (cluster-scoped). `pwIsolatedFixture` runs `migrations.Apply` twice on a fresh container, so the role, the identity schema and the identity functions are all present. The owner is the container's `postgres` superuser, so `CREATE ROLE … IN ROLE commerce_identity` is permitted. `bcRole(t, f, "commerce_runtime")` already uses the same pattern on the isolated fixture.
- `f.owner` and `f.databaseURL` are both set by `pwIsolatedFixture` (payment_runtime_test.go, `&testFixture{owner: owner, databaseURL: u.String(), …}`).
- Cleanup order (t.Cleanup runs in LIFO order):
  - `pwIsolatedFixture` registers the container `docker rm` first, then `owner.Close`.
  - `identityFixtureOn` registers DROP OWNED/DROP ROLE later, then `pool.Close` last.
  - Teardown therefore runs: pool.Close → DROP OWNED/ROLE (owner still open, container alive) → … → owner.Close → container rm. This is safe. On an isolated container the DROP is redundant but harmless.

## 3. No behavior change for existing callers
`identityFixture(t)` is now `return identityFixtureOn(t, fixture(t))` (identity_integration_test.go:39-42). The body moved verbatim: same role creation, same cleanup, same pool, same identity.Policy. There are 27 files with `identityFixture(t)` call sites, and all of them are unaffected.

## 4. Other harnesses with the same bug: none found
- I scanned every function in tests/foundation that obtains an isolated fixture: `pwIsolatedFixture`, `rm2StripePending`, `sflNew`, `ewFixture`, `mrFixture`, `pqSetupWorker`, and `sucOrder` with `isolated`. I checked each one for `identityFixture(t)`, `fixture(t)`, `pqSetup(t)`, `pqSetupSession` or `pqSetupItems`.
- Hits after manual review:
  - merchant_orders_test.go:532 `TestMerchantOrdersActualPaymentAndExpiryStates`: uses `pqSetup(t)` only for the "authorized" mode, and every access goes through that same `q.f`. Consistent, not a bug.
  - stripe_untracked_capture_test.go:19 `sucOrder`: falls back to `fixture(t)` only when no isolated fixture is passed. Consistent.
  - password_schema_test.go:497: the isolated sub-test uses only `iso`. Consistent.
  - `pwIsolatedFixture` itself calls `fixture(t)` only as a permission gate.
- The two merchant-orders browser harnesses were the only mixed cases.

## 5. Assertions and hygiene
- No assertion was changed, removed or relaxed. The diff only touches fixture binding and one comment.
- The comment fix at payment_fixture_helpers_test.go:53-55 is accurate:
  - `set_stripe_webhook_endpoint` raises PT409 "already exists" only for the same `endpoint_id` (migrations/0137_platform_stripe.sql:951-953).
  - A second endpoint for the same connection/profile violates `UNIQUE(tenant_id,store_id,connection_id,execution_profile)` (0061_stripe_psp.sql:358), which returns 23505.
  - The registrar maps 23505 to `ErrRejected` (internal/payments/stripeadmin/registrar_test.go:319).
  - Nit, not blocking: it is a UNIQUE constraint backed by an index, not a named index.

## Commands
- `git show f02adcb3b`: exit 0
- `gofmt -l` on the 4 changed files: no output, exit 0
- `go vet ./tests/foundation/`: exit 0
- `go vet -tags browser ./tests/foundation/`: exit 0
- Go tests: NOT_RUN (forbidden for this reviewer)

## Residual risk
- The browser real-chain gates have not been re-run by me. The fix is accepted only once CI on f02adcb3b shows `TestBrowserMerchantOrdersUIRealChain` and `TestBrowserMerchantOrdersBFFRealChain` green.
