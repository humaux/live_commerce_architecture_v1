# SCOPE-404 — mid-request grant revocation must answer the LCN03 404, not 403

Branch: `unit/scope-revoke-404` (base `origin/r3/integration`). Evidence class: REAL_PG (Go gates),
BROWSER gate BLOCKED (see NOT_RUN/BLOCKED). No contract change — the contract is right; the code was wrong.

## Symptom

`tests/admin/live-console.spec.ts:219` LCU2_404 ("real store grant revoked clears all/private view and
selected buyer") intermittently saw **403 forbidden** instead of the scope-loss **404** from scoped GETs.
LCN03 (`contracts/live-console-v1.md` §12, line ~709) freezes: "Cross-tenant/store reads return 404 for
A1–A16". The BFF patch on the other branch (Codex-1 `58510545`) re-read `/v1/admin/stores` after a GET 403
and rewrote it to 404 — a symptom patch (inbox branch only; a second, lag-prone opinion that can misreport
a genuine 403 as 404; changes BuyerPanel revocation semantics). This unit fixes the root cause in Go; the
BFF patch is unnecessary.

## Root cause (proven, deterministic)

The browser harness fault `grant_revoke` (`tests/foundation/browser_live_console_test.go:449-462`) deletes
only the principal's **`store:read`** grant on another connection:

```sql
DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='store:read'
```

The 403 comes from **path (c)** — none of the two hypothesised paths:

- (a) `RequirePermission`'s revision-mismatch branch: not involved. The fault bumps no `authz_revision`,
  and a second-fence `resolve_access` after the delete already returns `not_found` → 404 (pinned by
  `TestRequirePermissionFullRevokeBecomesScopeNotFound`).
- (b) `resolve_access` returning `forbidden`: impossible here. Its regular branch *requires* the
  `store:read` grant in the visibility join; with it deleted the answer is `not_found`, never `forbidden`.
- (c) **the actual path**: a scoped READ COMMITTED request whose scope opened *before* the delete commits
  runs its definer statement on a *fresh* snapshot that already sees the revoke. The A13 buyer panel
  definer `inbox.buyer_panel` (migration 0165:463-465) guards with **`identity.principal_holds`**
  (migration 0064:54-74), which — unlike `inbox.principal_holds` (0119) — requires the `store:read`
  grant. The guard fails and raises **PT403 'forbidden'**; `withScopeContext` passed that raw error
  through and `inboxClassify`/`templatesClassify` map PT403 → 403. The console UI treats 403 as a plain
  denial (stops the scope-loss handling), so the watched 404 never arrives and the test times out.
  The same guard shape exists in `inbox.read_outbound`, `inbox.link_pending_bundles`,
  `inbox.link_pending_for` (0128) — every scoped family whose definer guards on
  `identity.principal_holds` had the same latent race.

Deterministic RED proof (`tests/foundation/scope_revoke_race_test.go`,
`TestScopeRevokeMidTransactionBecomesScopeNotFound`): open a scope with `inbox:read`, delete the
`store:read` grant via the owner connection *inside* the open transaction (exactly the harness fault),
call `inbox.buyer_panel`. Before the fix:

```
--- FAIL: TestScopeRevokeMidTransactionBecomesScopeNotFound (1.41s)
    scope_revoke_race_test.go:39: mid-request store:read revoke error = ERROR: forbidden (SQLSTATE PT403)
        (sqlstate "PT403"), want ErrScopeNotFound (LCN03 404)
```

## Fix (one shared place; all scoped families inherit it)

`internal/platform/platform.go` only:

1. **`withScopeContext`** — when `fn` returns a denial (`platform.ErrForbidden` or a definer `PT403`),
   re-resolve once with `identity.resolve_access` on a **fresh pooled statement** (the scope transaction
   is already aborted by the error, so the re-check cannot run inside it):
   - `not_found` → return `ErrScopeNotFound` (the LCN03 non-disclosing 404: the principal provably no
     longer sees the store);
   - anything else (`forbidden`, `ok`, `unauthorized`, or a failed re-check) → return the **original**
     error unchanged. Fail closed: a re-check never upgrades a denial, and a genuine permission denial
     (store still visible, permission missing) keeps its 403.

2. **`RequirePermission`** — the `ok`-but-mismatched (tenant/principal/revision) branch no longer
   blanket-returns `ErrForbidden`; it re-resolves once more on a fresh statement of the still-healthy
   transaction (`reResolveScopeLoss`): `not_found` → `ErrScopeNotFound`; every other outcome →
   `ErrForbidden` (fail closed, never success). The `forbidden` branch is unchanged: that answer already
   came from the current statement's fresh snapshot and means "store visible, permission missing" — a
   genuine 403.

This cannot misreport a genuine 403 as 404 the way the BFF patch could: the re-check is the *same*
authoritative function (`resolve_access`) over the *same* store and the *same* route permission, and it
only ever converts a denial into 404 when store visibility itself is gone (the `store:read` grant /
membership / active store+tenant no longer resolve). Two-view lag is impossible because there is one view.

### Caller audit (grep: every `RequirePermission` / `ErrForbidden` consumer)

- `merchantorders.Actions`, `live.studio` CanManage, `merchanttools/order_for_buyer` capability probes:
  map `ErrForbidden` → "button hidden"/`capability`/"permission". Genuine denials still produce
  `ErrForbidden` (unchanged). A mid-request scope loss now surfaces `ErrScopeNotFound` → 404, which is
  exactly LCN03's required answer for those routes.
- `internal/httpapi/live_console.go:55` probe: its `ErrForbidden` comes from the scope-open
  `resolve_access` (permission missing, store visible) — path unchanged.
- `merchanttools/checkout_reminders.go` refusalCode: maps `ErrForbidden`/PT403 → `forbidden`;
  `ErrScopeNotFound` ends the pass and propagates → 404. Correct.
- Existing test that pins the fail-closed branch: `TestLivePlanningLSP03AuthorityIsolationAndReadOnly`
  ("stale revision accepted" → `ErrForbidden` with grants intact) — still green.

## Tests (RED → GREEN)

New `tests/foundation/scope_revoke_race_test.go` (REAL_PG, synthetic principals only):

| test | scenario | want |
|---|---|---|
| TestScopeRevokeMidTransactionBecomesScopeNotFound | store:read deleted mid-scope (harness fault), definer PT403 | `ErrScopeNotFound` (was PT403 → RED) |
| TestScopePermissionLossMidTransactionStaysForbidden | only `inbox:read` deleted mid-scope | raw PT403 preserved (403) |
| TestRequirePermissionRevisionBumpFailsClosed | `authz_revision` bumped, grants intact | `ErrForbidden` (fail closed, never upgrade) |
| TestRequirePermissionFullRevokeBecomesScopeNotFound | all grants deleted + revision bumped (0089 staff_remove shape) | `ErrScopeNotFound` |
| TestScopeOpenWithoutAnyGrantIsScopeNotFound | fresh request, principal with no grants | `ErrScopeNotFound` |

### Commands and exit codes

- RED (before fix): `bash scripts/dev/test-focused.sh 'TestScopeRevokeMidTransactionBecomesScopeNotFound|…'`
  → exit 1, `PASS=4 FAIL=1` (the race test failed with SQLSTATE PT403, quoted above).
- GREEN (after fix): same command + `TestScopeIsolationAndSessionRevocation` → **exit 0, PASS=6 FAIL=0**.
- Scope suites: `bash scripts/dev/test-focused.sh 'TestScope|TestRequirePermission|TestHTTPBearerScope|
  TestErrorPanicCancel|TestAuditIsScoped|TestAuditAndRiverJob|TestMigrationIsIdempotent|
  TestMigrationBusyFailsFast|TestOpenPoolRejects|TestSupportGrant|TestLiveConsoleBuyerPanel|
  TestLiveConsoleInbox|TestInboxUnknownAuthority|TestLiveConsoleTemplates|
  TestLiveConsoleMerchantOriginGrantBranch|TestLiveConsoleOrderForBuyerGrantsDoNotOutliveRequest'`
  → **exit 0, PASS=51 FAIL=0**.
- Identity/live read/planning/order-for-buyer suites: `bash scripts/dev/test-focused.sh 'TestIdentity|
  TestPrivateIdentityHTTPWithRealDatabase|TestLiveConsoleRead|TestLiveConsoleLCN0|TestLiveStudio|
  TestLiveDraft|TestLivePlanning|TestLiveConsoleOrderForBuyer|TestT04MigrationScope'`
  → **exit 0, PASS=46 FAIL=0** (includes `TestLivePlanningLSP03…` stale-revision → ErrForbidden pin).
- Media/draft RequirePermission-heavy suites: `bash scripts/dev/test-focused.sh 'TestLiveMediaPlan|
  TestLiveMediaStop|TestLiveMediaExecution|TestLiveMediaInput|TestLiveMediaRecovery|TestLiveConsoleRead|
  TestPickupRevisionsRevocationAndScope'` → **exit 0, PASS=65 FAIL=0** (595.8 s).
- `go vet ./...` → exit 0. `go vet -tags browser ./tests/foundation` → exit 0 (check-gates' browser-tag
  leg run standalone). `gofmt -l internal/platform tests/foundation/scope_revoke_race_test.go` → empty.
- `go test ./internal/platform/ ./internal/inbox/ ./internal/live/ ./internal/httpapi/
  ./internal/identity/ ./internal/msgtemplates/` → all ok.

## NOT_RUN / BLOCKED

- `bash scripts/dev/check-gates.sh` — **BLOCKED**: this fresh worktree has no `node_modules`; the gate's
  `check-browser-evidence.mjs` needs the workspace `typescript-api` package, and
  `pnpm install --frozen-lockfile` requires a permission approval this session declined. Static portions
  verified manually instead: `go vet ./...` clean, `go vet -tags browser ./tests/foundation` clean
  (the gate's second vet leg), gofmt clean, shard-plan catch-all
  covers the new test file by construction (`scripts/dev/shard-plan.mjs` documents "a test in no list
  lands in the catch-all"), header ratchet satisfied (Purpose/Depends on/Used by present).
- `bash scripts/dev/test-local.sh --browser-live-console` ×2 — **BLOCKED**: same missing `node_modules`
  (admin `next build` fails: `next: command not found`). LCU2_404 end-to-end on this branch is therefore
  NOT_RUN here; the deterministic REAL_PG race test above reproduces the exact fault the spec injects
  (identical DELETE) and proves the 404. Ask the owner/session to approve
  `pnpm install --frozen-lockfile` and the two browser runs can proceed unchanged.
- No LIVE/SANDBOX anything: all evidence REAL_PG + MOCK.

## Contract note

No contract change. LCN03 already freezes 404 for cross-tenant/store reads; a mid-request revocation puts
the store outside the principal's visibility, so 404 is the frozen answer and 403 was the defect. The
review's objection to the BFF patch is honoured: BuyerPanel revocation semantics are unchanged in SQL
(the definer still raises PT403 for a genuine permission loss — preserved as 403), and no second,
differently-lagged opinion is consulted.

## Round 1 (Qwen READONLY review PASS; three P2s; Kimi quota-blocked, so the integrator made these changes)

- **Browser runs are no longer BLOCKED.** The NOT_RUN statement above is superseded. The integrator ran `bash scripts/dev/test-local.sh --browser-live-console` twice on cac97d3f with no BFF patch. `TestBrowserLiveConsoleRealChain` passed both times (335.28 s and 329.54 s), including LCU2_404 all/private (`browser-live-console-1.log`, `browser-live-console-2.log`). The integrator also reproduced red by putting trunk's `platform.go` back: PT403, exit 1 (`red-integrator.log`).
- **Pool pressure.** `withScopeContext` now rolls back the aborted scope transaction before the re-check, which releases its connection; the deferred rollback becomes a no-op on the closed tx. The re-check is bounded by `scopeRecheckBudget = 500ms`, and a timeout or error keeps the original denial.
- **Fail-closed tests.**
  - `internal/platform/scope_recheck_test.go` `TestScopeRecheckFailureKeepsDenial`: a re-check whose row scan fails never reports scope loss. Mutation red: `err != nil || …` fails the test; green after reverting.
  - `TestScopeSupportGrantRevokedMidTransactionBecomesScopeNotFound` (REAL_PG): a 0153 support grant revoked inside the scope, followed by a denial, gives `ErrScopeNotFound`, the same as the between-request answer. Red on trunk `platform.go` (`forbidden`, `r1-red-support-revoke.log`); green on the fix (`r1-green-support-revoke.log`).
