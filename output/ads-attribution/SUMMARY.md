# ads-attribution — Amendment 1 checkpoint (NOT ACCEPTED)

## Scope and release stop line

- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/ads-attribution`; branch `unit/ads-attribution`.
- Runtime source through `e530bfdc`, browser fixtures through `bac64356`. Full G07 is an earlier compiled checkpoint `3dab4b9f`, not the final source. Authorized merge `ea131b56` contains `fbbe22cc` and `b7afdf1d`; ancestor checks exit 0.
- Migration **0113** belongs to this unit. Migration **0112** refusal fields are unchanged from `fbbe22cc` (empty diff).
- **BLOCKED — do not merge/release:** `orders.freeze_attribution` rejects legitimate Begin transactions with `PT404 / attribution order unavailable`. This affects ordinary checkout, not only reports.
- Two attempted creation guards failed in REAL_PG: top-level transaction xmin, then the creating backend's transaction locks. The latter remains in source. The suspected cause is the `begin_hold` exception subtransaction, but exact transaction lineage is not proved. Per PROCESS, no third fix or relaxed historical-order assertion without integrator ruling.
- Proposed explicit creation-transaction marker was sent for a decision. **Not approved or implemented.** It must prevent historical-order attribution and leave financial snapshots unchanged.
- No push, deployment, Meta mutation, production request, sandbox event, real buyer data or credentials access. All exercised Graph traffic is local MOCK.
- Old initial checkpoint `2ca36a7c` remains in Git history; its cookie/design questions are superseded by Amendment 1, not current blockers.

## Implementation / ruling ledger

| Ruling / area | Implementation and evidence | State |
|---|---|---|
| R1 baseline | Authorized merge; 0112 untouched | PASS |
| R2 storage | Narrow scoped orders.order_attribution; Begin transaction; financial snapshot unchanged | IMPLEMENTED, **FAIL at guard** |
| R3 erasure / IP | Erasure nulls pseudonyms; terminal CAPI trigger + bounded purge; final lease/consent guarded CAPI context | IMPLEMENTED, checkout-dependent PG blocked |
| R4 precedence | Valid click wins; simultaneous boosts yield draft NULL, never split | IMPLEMENTED, PG FAIL before assertions |
| R5 cookies | Independent rolling90d fbp; fbc replaced only by fbclid; no ordinary-visitor cookies; no fbclid-only touch; Host-only Secure/HttpOnly/Lax | UNIT PASS9; browser NOT_RUN |
| R6 comment identity | Server-owned claim line version, exact claim→intake→post; no session fan-out; independent of discounted price | IMPLEMENTED, PG blocked |
| R7 time | timezone_name persisted; absolute hourly time; daily account-day label retained | PG/MOCK storage PASS; browser NOT_RUN |
| R8 audience | GET-only Page-token route, explicit read command, insufficient/unknown not zero, view-time buckets, no buyer join | REAL_PG/MOCK read chain PASS; LIVE NOT_RUN |
| Own / Meta reports | Separate figures; own net minus refunds; pending COD separate; exact county allowlist excludes address/name free text | IMPLEMENTED; populated report PG/browser blocked |
| Session report | Timeline, funnel, ambiguity, buyer aggregates, Meta panels; snapshot ordering by request not completion | Read chain PASS; populated browser NOT_RUN |

R7 currently fetches account timezone once per independently queued draft/day read operation and reuses it for all five breakdown requests; not a global cache across operations. No daily relabelling as Taipei.

## Logical commits

All implementation/test commits carry `Co-Authored-By: Codex <noreply@openai.com>`.

- `ea131b56`: authorized integration baseline merge.
- `dc4186fe`: bounded Meta breakdown reads and attributed URL.
- `033f3e6a`, `b8fd65f4`: runtime cookies/Begin/CAPI, 0113, exact claim origins and reports.
- `911318eb`: live SQL, nullable claim CHECK, **current blocked creation guard**.
- `947255dc`, `b54a5b55`, `66027dc1`: three-locale report UI, shell recovery, explicit audience read/fence.
- `f15f5e3b`, `fc5d3f97`, `197572c5`: scoped audience plan/Page custody/GET-only dispatcher/policy.
- `3f66c274`: county PII protection, request-ordered snapshots and bounded buckets.
- `2371cd77`, `185ce287`, `d15d52b5`: independent core PG, real-click AT5 driver, audience PG tests.
- `db911b57`, `1c861cdb`: scoped aggregate grants, frozen privacy-version grammar, typed AD403/AD422.
- `3dab4b9f`: formal browser mode, contract and empty-report PG test.
- `10c46962`: exact populated report PG and actual checkout/admin browser runners (no fixture bypass).
- `bac64356`: exact audience/hourly/privacy assertions and draft-only six-order browser report.
- `e530bfdc`: remove unused direct customers consent authority from checkout; frozen CB02 ACL tests unchanged.

## Commands actually run

Evidence paths below are relative to this directory unless absolute. Earlier red logs are retained. Empty output does not constitute a test count.

| Command | Exit | Evidence / count |
|---|---:|---|
| R5 cookie test before amendment implementation | 1 | red-r5-amendment.log; initial red-fbp-lifetime.log retained |
| `node --test --experimental-strip-types apps/storefront/tests/ad-touch.test.mjs` after R5 | 0 | green-r5-amendment.log: 9 PASS, 0 FAIL, 0 SKIP |
| `go build ./...` | 0 | Root tool transcript; final-source rerun required after blocker |
| `go vet ./...` | 0 | Root tool transcript |
| `gofmt -l` changed Go packages/tests; `git diff --check` | 0 | No files/whitespace reported |
| `go test -race ./internal/integrations/meta_ads ./internal/integrations/metareply ./internal/attribution/... ./internal/checkout ./internal/httpapi` | 0 | go-pure-runtime.log: six packages PASS |
| `bash scripts/dev/test-node.sh` | 0 | node-runtime-4.log: 359 PASS, 0 FAIL; optional R04 binary suite NOT_RUN |
| `pnpm --filter admin exec tsc --noEmit` | 0 | admin-tsc-runtime.log |
| `pnpm --filter storefront exec tsc --noEmit` | 0 | storefront-tsc-runtime.log |
| Final fixture source: normal and browser-tag `go test ./tests/foundation -run '^$'`, browser-tag `go vet ./tests/foundation` | 0 each | report-fixture-compile.log, browser-fixture-compile.log, browser-fixture-vet.log; compile only, zero tests run |
| Strict targeted TypeScript compile of attribution.spec.ts and attribution-checkout.spec.ts | 0 | browser-facts-tsc.log; exact command in author output/ads-attribution-ui/SUMMARY.md |
| Fixture-source test-node, admin/storefront tsc, check-gates | 0 each | node-fixtures.log (359 PASS), admin-tsc-fixtures.log, storefront-tsc-fixtures.log, check-gates-fixtures.log |
| `bash scripts/dev/check-gates.sh` | 0 | check-gates-runtime.log:62 documented modes; >500-line warnings retained |
| `bash scripts/dev/release-gate.sh --list` | 0 | Formal B-browser-ads-attribution present, not an alias |
| `LC_RELEASE_GATE_OUT=output/ads-attribution/g04-checkpoint bash scripts/dev/release-gate.sh --strict --only G04` | 0 | g04-checkpoint/results.tsv; selected secret gate only, not a release verdict |
| `bash scripts/dev/test-focused.sh '^(TestAdsAttributionSessionAudience\|TestAdsAttributionEmptyProjection\|TestAdsAttributionAT8BreakdownDispatch\|TestLiveClaimsKC03Schema)'` | 0 | live-read-pg.log:top-level PASS7 FAIL0 SKIP0, REAL_PG + MOCK |
| `bash scripts/dev/test-focused.sh '^TestAdsAttributionEmptyProjection$'` | 0 | empty-report-pg.log:PASS1 FAIL0 SKIP0 |
| `bash scripts/dev/test-focused.sh '^TestAdsAttributionReport$'` | 1 | report-pg-2.log:PASS0 FAIL1, Begin PT404 |
| Independent core PG (author; exact command in log) | 1 | /Volumes/data/live_commerce_architecture_v1/output/ads-attribution-tests/core-pg-fourth.log:PASS1 FAIL10; root reproduces blocker |
| `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-focused.sh '^TestCustomersBillingCB02Schema$'` after e530bfdc | 0 | consent-acl-pg.log:PASS1 FAIL0 SKIP0; original frozen customers ACL assertions unchanged |
| `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/release-gate.sh --strict --only G07` on 3dab4b9f | RUNNING | /Volumes/data/live_commerce_architecture_v1/output/release-gate/20261003T215312Z-3dab4b9ff854/G07.log; PT404 reproduced; started load~3, sampled~5.4 |

Full G07 exit/count pending. No locks deleted or foreign processes stopped.

## AT1–AT9 verdict / NOT_RUN

| Gate | Verdict |
|---|---|
| AT1 | PARTIAL UNIT PASS / REAL_PG FAIL. Cookie cases green; Begin and browser blocked. |
| AT2 | PARTIAL pure policy PASS; consent/terminal IP/erasure PG not accepted because Begin fails. |
| AT3 | FAIL. Exact comment/ambiguity tests fail before expected assertions. No negative assertion relaxed. |
| AT4 | NOT_ACCEPTED. Exact populated refund/COD/report fixture compiled; no green runtime result. |
| AT5 | NOT_RUN. Real-click driver/runner/spec compiled; no six-matrix screenshots claimed. |
| AT6 | NOT_RUN accepted prerequisite: owner dataset ID, test_event_code and Events Manager sandbox evidence. |
| AT7 | NOT_ACCEPTED. Full G07 running/red; click sweep NOT_RUN pending working Begin. |
| AT8 | PARTIAL REAL_PG/MOCK PASS for dimensions/replacement/isolation; browser display NOT_RUN. |
| AT9 | PARTIAL REAL_PG/MOCK PASS for Page plan/dispatch/persistence/report, denials, exact lease and overlapping completions. Populated funnel/timeline/browser NOT_RUN; LIVE NOT_RUN pending read_insights. |

Browser acceptance requires actual clicks at 390/1586 in zh-TW/zh-CN/en. No patched DOM, fabricated network responses or pre-seeded replacement for the six AT5 orders.

## Independent evidence / team

- Red lifetime evidence → green-r5-amendment.log. Red read-chain logs → root live-read-pg.log. Root/author failure logs retained, never counted as current greens.
- Scoped security reviewer identified raw county free-text leakage and stale completion ordering; 3f66c274 fixes both. Source review cleared within that scope, **not PG/browser/release acceptance**. County synthetic-private-string PG negative remains blocked at Begin.
- Full G07 found extra checkout-writer customers consent grants. e530bfdc removes the unused grants rather than changing frozen CB02 expectations; consent-acl-pg.log independently turns the original assertion green.
- Full G07 also found historical pre-0113 upgrade fixtures using the current cart writer without claim_line_version. This is separate from PT404; investigation pending, not dismissed as an environment issue.
- Root codex-ads-attribution task:6d5e18a9-696a-4671-b9bf-6c154f066042; sole root worktree writer/integrator. No model override.
- Independent worktrees: ads-attribution-insights (Go adapter/audience tests), ads-attribution-ui (UI/TS browser tests), ads-attribution-tests (PG/Go browser fixtures). Children do not edit this worktree.
- ps_security_final: read-only scoped review; no production write ownership. Author is not sole reviewer.
- Humaux records/canvas hold progress; unit not marked complete. Skills: frontend-architect (capture/authority/report boundaries), impeccable (audit existing report layout), playwright (real-click acceptance).
- All 55 changed Go/TS/TSX/MJS/SQL files submitted to incremental code indexing; AudienceRoutes linked to independently verified read-chain memory.

## Next steps

1. Obtain integrator decision for Begin creation-transaction guard; preserve historical denial and receipt replay semantics.
2. Implement approved design, rerun core and populated report PG including ordinary checkout.
3. Integrate/compile browser fixtures; run formal browser-ads-attribution six matrices and real-click ledger.
4. Re-run final-source build/vet/gofmt/node/tsc/check-gates, full G07 on quiet machine and click sweep. Record exit/count/screenshots; retain red history.
5. Keep AT6 SANDBOX and AT9 LIVE as explicit NOT_RUN until prerequisites exist. Update evidence/Humaux/canvas and obtain independent release review. No push/deploy.
