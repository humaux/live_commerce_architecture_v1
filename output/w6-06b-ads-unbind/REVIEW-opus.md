# REVIEW — W6-06B Meta ad-account unbind + catalog feed URL (independent, read-only)

- Reviewer: Claude Opus 5.5 (independent; author = Aliyun Qwen qwen3.8-max). Date 2026-10-07.
- Reviewed HEAD `84ac7f1f` (author `4251c24e` + trunk merge), diff `git diff r3/integration...HEAD` (12 files, +1343/−13).
- Nothing edited or committed except this file.

## Verdict: **MERGE-AFTER-FIX**

Two P1s, both mechanical and both make CI red today. The design is sound. No P0: a spending campaign cannot be
orphaned (see §1). After the two P1 fixes and a focused re-run on the new SHA, this unit can merge.

## Re-run evidence (this review, HEAD 84ac7f1f)

| Command | Result |
| --- | --- |
| `go build ./... && go vet ./...` | ok |
| `gofmt -l internal cmd tests` | clean |
| `go test ./internal/ads/...` (DB-free) | **FAIL**: `TestFrozenCodesSurviveHTTPError`: `binding_in_use` and `operations_in_flight` are "rewritten to "internal" by httperror" |
| `go test ./internal/httpapi/` | ok |
| `bash scripts/dev/check-gates.sh` | `check-gates: ok (72 modes…)`, `check-headers: OK` |
| `python3 scripts/check_packet.py` | exit 0 |
| `test-focused.sh 'TestT06WorkerAuthorityAndFunctionACL\|TestMetaAdsMA02Schema\|TestR2IntegrationUpgradeFromReleaseHead'` (REAL_PG) | **FAIL=1**: `external_operation_authority_test.go:331: fixed function ACL: count=90 safe=false`. MA02 PASS. R2 upgrade PASS (the pin of 84 is correct). |
| `test-focused.sh 'TestAds' ./internal/ads` (REAL_PG) | PASS=5 FAIL=0 (the unit's own tests re-run independently: green) |

---

## P1 (must fix before merge)

### P1-1 New frozen codes never reach the wire: merchants see `internal`, and an existing package test is red
- **Evidence.** `internal/ads/errors.go:60` adds `operations_in_flight` and `binding_in_use` to `frozenStatus`.
  `internal/httperror/error.go` has no message entry for either code (grep finds nothing). `writeDetails` rewrites
  unknown codes (`error.go:267-270`: `if !ok { code, message = "internal", … }`).
  The drift guard `internal/ads/validate_test.go:111-120` (`TestFrozenCodesSurviveHTTPError`) now fails. The
  author ran only `-run 'TestAds…'`, so the test was never executed (DELIVERY "Tests & exit codes").
- **Why it matters.** Both 409s ship with `code:"internal"` and the message "Request could not be completed."
  - `binding_in_use` is the R2-ADS-PAUSE-1 refusal. The UI cannot show the required "pause first, then disconnect"
    copy, which tasks.json r2_close says ships with this route.
  - The `operations_in_flight` details still arrive, but under the wrong code.
  - The unit package is red in CI.
  - No test checks the wire envelope: the transport tests only check admission, and the service tests check the Go
    `*Refusal`. The tests therefore do not cover the "409 code" acceptance claim.
- **Fix.**
  1. Add the two codes to the `messages` map in `internal/httperror/error.go`, under a "meta-ads-v1 Amendment
     W6-06B" comment. For example:
     - `"operations_in_flight": "Meta ads operations are still in progress; wait for them to finish."`
     - `"binding_in_use": "Pause the running Meta ads campaigns of this ad account before disconnecting it."`
  2. Add one assertion of the HTTP envelope: `adsScope` returns `Refusal{409,operations_in_flight,Details}`, and the
     body must have `code=operations_in_flight`, a non-empty `details.operations`, and the same check for
     `binding_in_use`.
  3. Add a `Service.Unbind`-level `binding_in_use` case. Today it is proven only with a test-local copy of the
     UPDATE (`unbind_pg_test.go:303-309`), so the `mapError` PT409 branch (`errors.go:104`) has no test.
  4. Red evidence: the current failure output above.

### P1-2 Integration-schema function ACL pin not updated (the "no ACL delta" claim is wrong)
- **Evidence.** `tests/foundation/external_operation_authority_test.go:315-332`
  (`TestT06WorkerAuthorityAndFunctionACL`) pins every `integration.*` function to an approved row and checks
  `functions != 89`. Migration 0160 adds `integration.meta_ads_unbind` without a row. Measured result:
  `count=90 safe=false` (REAL_PG run above). 0160 adds exactly one integration function; the list has 89 rows
  (the trunk count). The failure is therefore this unit's.
- **Why it matters.** It breaks the foundation suite. The DELIVERY and the contract amendment claim that no ACL pin
  needs updating; that is incorrect.
- **Fix.**
  1. Add the row `('integration.meta_ads_unbind(bytea,uuid,text)'::regprocedure::oid,ARRAY[]::text[],'commerce_integration_writer',false,true,false,false)`.
     It has the same shape as `meta_connect_disconnect` at `:271`.
  2. Change 89 to 90 and add a one-line comment.
  3. Re-run the focused test.

---

## 1. Unbind behaviour

### Money and spend safety: **covered by the frozen guard (no P0)**
Unbinding never leaves a possibly spending campaign without a pause path:
- **The guard refuses the detach.** The Go CAS `UPDATE … enabled=false` (`unbind.go:103-107`) fires
  `bindings_ads_disable_guard` (`0074:510-512`, `WHEN OLD.provider='meta_ads' AND OLD.enabled AND NOT NEW.enabled`).
  The guard refuses PT409 `binding_in_use` while any draft on the binding satisfies `ads.draft_counts`
  (`0074:464-489`).
- **What counts.** A draft counts when any activate op is READY, DISPATCHING, UNKNOWN, ACKNOWLEDGED, SUCCEEDED,
  FAILED_FINAL or STALE_BINDING, unless one of these holds:
  - a pause SUCCEEDED was claimed after every activate left READY/DISPATCHING;
  - the time is past `ends_at + 1 day` (Meta's own end_time has stopped delivery by then);
  - every activate is `BLOCKED_POLICY`, which is set before any provider call.
- **Atomic rollback.** The trigger runs in the same transaction as the token deletion. `platform.WithScope` rolls back
  every returned error (`internal/command/command.go:1-3`), so a refusal also restores the credentials. The
  REAL_PG case proves this at the SQL level (`unbind_pg_test.go:291-323`: creds/heads 1/1, binding still enabled).
- **In-flight check.** DISPATCHING, UNKNOWN and ACKNOWLEDGED ops refuse before anything is written
  (`0160:65-76`).
- **Races.** An activate planned concurrently can slip past the trigger's statement snapshot. It is inserted READY
  on a binding that is disabled at commit. `claim_operation` then marks it `STALE_BINDING` (`0096:145-152`, MERCHANT
  plus `NOT b.enabled`), so it is never dispatched.
- **Remaining gap: UX only.** Today the refusal surfaces as `internal` (P1-1) and lists no drafts.
- **Proposed improvement (P2-1 below).** Have the refusal carry the counting draft ids, so the UI can say "pause
  these N drafts". That needs an ads-side read, because `ads.draft_counts` is not executable by the integration
  owner. Do not "pause automatically, then unbind": a pause is an external op that can end UNKNOWN, and the unbind
  would then strand it. Refusing is the correct choice.

### Sweeper, guard and lock order
- **Lock order with `claim_operation` is safe.**
  - `claim_operation` takes the binding `FOR SHARE`, then the op `FOR UPDATE` (`0096:129-130`).
  - The unbind takes the binding `FOR UPDATE` (`0160:53-58`), then reads ops without locks. It never locks an op.
  - Result: no deadlock. A claim cannot enter DISPATCHING mid-unbind.
  - Every statement after the lock gets a new snapshot (READ COMMITTED is enforced by `ads.auth`, and the function
    is VOLATILE), so a claim committed just before the lock is seen by the in-flight count.
- **Lock order with `register_meta_ads_token` is the same.** It takes the binding `FOR SHARE`, then the head
  `FOR UPDATE` (`0074:720-741`). A concurrent re-bind of the same asset then gets `binding_disabled` after the unbind
  commits.
- **Sweeper side effects are noisy but not money-relevant.** They are newly reachable by merchants; see P2-2.

### Sealed-token destruction: **correct, follows 0108**
- It deletes the heads first, then all versions, per binding, scoped to tenant and store (`0160:79-87`), the same as
  `meta_connect_disconnect` (`0108`).
- After bind, the only ciphertext copy is in the credentials tables: the oauth-state pending copy is wiped by
  `finish_bind` (`0074:640-653`).
- Nothing token-shaped is returned or logged. `unbind.go` has no logging, and the refusal details carry only
  `operation_id`, `action` and `state`.
- One scope limit, by design: see P2-3 (the dataset binding keeps the same BISU token).

### History is preserved: **yes**
Only `meta_page_heads` and `meta_page_credentials` are deleted. The following are proven to survive
(`unbind_pg_test.go:359-371` and the re-bind case `:579-582`):
- `ads.connections` (the re-bind case shows connections=2);
- drafts;
- oauth states;
- the operations ledger;
- the audit row.

Idempotency and re-bind are also correct:
- A replay with the same key returns the stored receipt, compared byte-for-byte.
- A new key returns the 200 no-op.
- A refused attempt leaves no receipt.
- Re-binding through the connect flow creates a new binding, because `bindOne` selects enabled bindings only.

## 2. Feed URL: **correct**
- **Path.** `/feeds/meta.csv` exists on the storefront host (`apps/storefront/app/feeds/meta.csv/route.ts`). It
  proxies to `GET /v1/buyer/feeds/meta.csv` (`cmd/api/buyer.go:174`). `ads.feed_rows` resolves the store via
  `buyer.resolve_published_store` (0080:339-340).
- **Origin.** Origins are stored as `https://host` with no trailing slash (CHECK at `0020:17-20`), so plain
  concatenation is right. The query keeps only ACTIVE domains of a published store (`0160:109-117`). Reads use the
  existing 0074 GAP-2 column grants and policies (`0074:343-346`); every column referenced is granted.
- **Scope.** Tenant and store come from `ads.auth` with GUC equality. The cross-store leak test passes.
- **Permission.** `ads:read` is correct, because the feed is public and unsigned and no secret is involved.
- **Known limit (documented).** `valid_until`, `stores.active` and `tenants.active` are not re-checked, so a listed
  URL can 404 at fetch time.

## 3. SECURITY DEFINER hygiene, grants, RLS, pins
- **Both definers.**
  - `SECURITY DEFINER SET search_path=pg_catalog`.
  - Every relation and function is schema-qualified.
  - `REVOKE ALL FROM PUBLIC`, `EXECUTE commerce_runtime` only.
  - Correct owners (`0160:91-93`, `121-123`), and `COMMENT ON` is present.
- **No table, column-grant or policy delta: verified.**
  - The integration writer already holds:
    - SELECT, `UPDATE(id)` and the `worker_binding_lock` policy on bindings (`0008:107-118`);
    - `DELETE` on the credential tables (`0095:73`);
    - `USAGE ads` and `EXECUTE ads.auth` (`0113:610-611`).
  - The runtime already holds `UPDATE(enabled,semantic_version,updated_at)` (`0008:115`).
- **The claim "no ACL pin delta" is false:** see P1-2.
- **Pins.** MA02 passes. The R2 pin of 84 is correct on this branch and its upgrade test passes.

## 4. Tests

### Covered vs. missing

| Acceptance | Covered | Gap |
| --- | --- | --- |
| detach | ✓ (svc `:496-512`) | — |
| new plans blocked | partial: only `CreateDraft` → `binding_disabled` (`:372-380`) | contract §C claims approve/publish too |
| history kept | ✓ | — |
| 409 in-flight | ✓, DISPATCHING only | UNKNOWN and ACKNOWLEDGED untested: dropping either from the IN-list survives |
| idempotent | ✓ replay + no-op + no receipt on refusal | — |
| cross-store | ✓ for the feed. Unbind no-op: the "store1 binding untouched" assertion is conditional (`:412 if f.adBinding != ""`) | it is skipped in the brief's own focused regex |
| permission | ✓ (scope layer + definer AD403, ads:manage without integration:manage) | — |
| feed scope | ✓ own ACTIVE domain, cross-store | non-ACTIVE domain, unpublished → `null` untested (mutation survivors) |
| router | ✓ 11 transport cases | no 409 envelope (P1-1) |

- **Real definers, not mocks: yes.** REAL_PG uses the real migrations; only `ConnectFunc` (Meta) is faked.
  Two cases (`:291-323`, `:325-380`) drive the definer and then a test-local UPDATE instead of `Service.Unbind`.
  The service test covers the happy path but not PT409.
- **Red evidence: real but weak.**
  - Commit `6c537d9c` precedes GREEN `be44d6fe`. `red.log` shows 42883 "function does not exist" and 404s.
  - This is absence-red only: it proves the tests run, not that the assertions catch a wrong implementation.
  - No mutation red exists, for example for removing the in-flight check, the token DELETE, or `state='ACTIVE'`.

## 5. Code quality
- No `apps/` edits. No hallucinated APIs: every referenced function, grant, helper and line pointer exists and
  compiles. Existing patterns are reused correctly:
  - `command.Run` and the receipt;
  - `queryJSON` and `mapError`;
  - `respondErrorDetails` and `httperror.WriteDetails` (empty `{}` details give a byte-identical envelope for the
    other routes).
- The hand-written CAS UPDATE duplicates `core.SetBindingEnabled`. This is acceptable: that helper requires
  `integration:manage` plus its own receipt and audit, while the brief requires `ads:manage` only.
- Headers: see P2-5.

## P2 (follow-up; do not block)
1. **P2-1 `binding_in_use` lists nothing.** Add `details.drafts` with the counting draft ids (≤50), so the merchant
   knows what to pause. This needs an ads-owned read definer, or the integrator's ruling on granting
   `ads.draft_counts`.
2. **P2-2 Sweeper noise after unbind.** The disabled-binding paths in 0074 are now reachable by merchants:
   - **Pause retries.** An unbound, non-counting draft with a pinned campaign reaches `ends_at`. Then
     `advance_decide` plans a pause (`0074:1471`). Pause is allowed on a disabled binding (`:1140`). The claim marks
     it `STALE_BINDING`, and then `pause_retry` repeats every 15 minutes up to seq 50 (`:1480`). Each retry logs
     `slog.Error("ads pause retry planned")` (`sweepers.go:183`): up to 49 false "pause failing" alerts over about
     12 hours.
   - **Insights.** `insights_candidates` (`:1606`) has no enabled filter, so `insights_plan` raises 22023 (`:1659`)
     once a day. The result is a warn at `sweepers.go:206`.
   - **Mid-chain drafts.** `plan_op` denies drafts in the middle of the create chain, which gives a warn per sweep
     (`sweepers.go:150`).
   - **Fix.** In 0160, `CREATE OR REPLACE` `advance_decide`/`advance_candidates`/`insights_candidates` to skip drafts
     whose ad binding is disabled and `NOT ads.draft_counts`. Record this in the amendment. Alternatively, record the
     noise as accepted and silence the alert.
3. **P2-3 The dataset binding keeps the same BISU token.** When CAPI was bound in the same connect, the
   `meta_dataset` copy still holds the same system-user token, which carries `ads_management`. The ad-account copy is
   forgotten, but the grant is not. This is a non-goal by brief. State it explicitly in the amendment §A and the UI
   copy ("CAPI remains connected; revoke in Facebook settings").
4. **P2-4 Test gaps.** These are listed in §4: the UNKNOWN/ACKNOWLEDGED refusal, the non-ACTIVE and unpublished
   feed, approve and publish blocked, the unconditional cross-store assertion, and at least one mutation red per
   guard.
5. **P2-5 Docs, comments and dead code.**
   - Wrong counts and claims:
     - The migration header, brief, contract §C and DELIVERY say "R2 count 81→82"; the actual change is 83→84.
     - The amendment says the "Admin BFF mirrors it unchanged", but the BFF allowlist (`tests/admin/ads-request.test.ts`) has neither route yet.
   - Stale comments and regex:
     - `unbind.go:95` says "nothing written", but the receipt is written.
     - The brief's regex `TestAdsCatalogFeed` matches no test.
   - Dead code: the fields `unbindFx.principal3`/`principal4` (`unbind_pg_test.go:32`) are never set or read, and
     `unbindDefinerResult.AlreadyUnbound`/`CredentialsDestroyed` are decoded but unused.
   - PREAMBLE §3 gaps:
     - No one-line callee comment at the definer calls `unbind.go:81` and `:130`.
     - The `unbind_pg_test.go` header is not in Purpose/Depends/Used-by form.
     - The file is 615 lines, over the ~600 guideline.
6. **P2-6 Audit and evidence provenance.**
   - `ads.account_unbound` has no details. Use `command.AuditDetails` with `{ad_account_id, bindings:n}`; neither is PII.
   - `green.log` has no command or SHA headers.
   - DELIVERY lacks the reasoning level and allowed write_paths (AGENTS.md).
   - Commit trailers say `Co-Authored-By: Claude Code` although the implementer was Qwen.

## Calibration note: Qwen versus our standards
- **Design and SQL: close to our bar.**
  - It followed the repo's idioms:
    - fixed-search_path definers;
    - the 0108 token-destruction order;
    - command receipts;
    - the GAP-2 grants.
  - It reasoned correctly about lock order against `claim_operation` and about atomic rollback through the frozen
    guard.
  - It invented no API.
- **Gate discipline: well below our bar.**
  - It verified only with name-filtered runs (`-run TestAds…`). That missed:
    - a red drift test in its own package;
    - the integration-schema ACL pin.
  - It then asserted "no ACL pin delta" and "all green". That is an E0 claim presented as E3.
- **Tests are happy-path heavy.**
  - Red is absence-only, and several assertions are conditional or easy for a mutation to survive.
  - Docs drift (counts, BFF claim) and the comments are verbose.
- **Calibration verdict.** Trust Qwen for pattern-following implementation of small units. Always require from the
  integrator an unfiltered package `go test` plus the schema-pin foundation tests. Do not accept its "no
  privilege/pin change" statements without a run.
