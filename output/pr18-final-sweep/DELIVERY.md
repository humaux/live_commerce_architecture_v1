<!-- Purpose: deliver PR18 FINAL sweep, approved sequence/scan DTOs and fixed-source acceptance evidence.
Depends on: FINAL packet sections0–4, registry305695a7 and authorisations for seq/scan_exhausted; tested source0e5ba787.
Used by: integrator independent review and PR18 CI; author E3 does not substitute for independent merge acceptance. -->
# PR18 FINAL batch

- Branch `unit/lc-u2a-comment-stream`; tested source **0e5ba787a4f9dff1842f6a713d15ab0fcacd439d**. Evidence-only commit does not change this source.
- Trunk merged as **21c27683**, from origin/r3/integration305695a7 (#22 plus #16/#20/#21). The only conflict was test-local.sh; kept the complete single-mode registry and changed only its live-console PASS description. No legacy if/elif dispatch restored. `--list` exit0:83 entries; check-gates82 documented modes; post-merge Node1245/0. Upstream archived evidence whitespace from that merge was preserved, not rewritten.
- Parent Codex-1 owns UI/reconciliation. Integration worker (inherited model, high reasoning) used separate worktrees `pr18-seq-dto-only` and `pr18-ig-scan-dto`, owning Go/contract tests only. Exact runtime model identifier unavailable. Read-only source reviewer found two introduced P1s; both received causal red→green fixes, then no further introducedP0/P1 in the client diff. Source review is E1, not independent runtime/K3 acceptance.
- No push, deploy, real Meta mutation, credentials, SQL/migration/GRANT changes.

## Changes and isolated commits

1. **Public unresolved send guard** (1d00ae16, retained): PUBLIC store/session/ref key survives same-tab re-login; coarse session fence still follows CSRF boundary. Only terminal/verified clears it. No reply text/name stored.
2. **Older cursor availability** (1d00ae16, retained): older invalid_cursor drops only pagination cursor, preserving live buffer/selection/continuation. Incremental invalid_cursor still resets.
3. **IG earliest-window P1** (34b01cab, retained): ordinary cursorless IG HEAD always merges. Existing parsed source_platform proves FB newest-by-arrival-seq; no invented authority flag. The100-row hook/composer regression retains51–100, selected75, draft, next100 and zero sends over3 head cycles.
4. **Approved nullable seq DTO** — **680aa304** is isolated Go/TS-parser/contract commit. FB buffer's actual cc.seq; IG env.Seq; direct Graph history null. Go *int64 json:seq, no omitempty. No per-item seq fabricated from endpoint/time. TS accepts only explicit null or safe nonnegative integers.
5. **Additional approved scan EOF DTO** — **b8c286f8** is isolated Go/contract commit. A2 scan_exhausted is a required boolean. IG raw SQL row count, not accepted item length, proves EOF; next.seq advances to actual scanned env.Seq before decrypt/media filtering. FB false means no EOF proof, not necessarily more rows.
6. **Reconciliation/rebuild client** — **0e5ba787**. FB absence deletion only inside actual returned numeric [min(seq),max(seq)] and same epoch. Null history/out-of-range rows stay; created_at is display/cap ordering only. Explicit internal initialized state survives command cursorless reads, including epoch0. Null history reserves capacity at1000 and retains provenance on numeric overlap.
7. Live rebuild attempts every4 visible minutes, with60s total scan deadline/20-request budget and one privacy ticket. IG re-reads the published numeric range (manual reset starts at0), follows fresh scanned continuation and publishes once after coverage or trusted EOF. This removes a deleted finalseq100 even when current data ends99. Ordinary short/empty IG pages never prove EOF. Failures/stalls/budget exhaustion retain prior buffer/selection/draft and expose unavailable/backoff; successful readable refreshes complete within5min. A failed/unavailable upstream does not establish deletion; no blanket five-minute guarantee under failure.
8. Background rebuild pins Graph history and its pagination state while refreshing numeric live rows. The user's refresh explicitly resets history; command/onSent mark-refresh does not. Selection/composer clears only when the committed fresh window omits it or a real epoch/privacy boundary changes. Hidden/revoked scopes abort staging. Historical lookup may be lost through explicit manual reset, never automatic absence inference.

OpenAPI: structured scan of all6 existing JSON specs found A2 matches0; no OpenAPI changed. Isolated interface commits include contracts/live-console-v1.md §2.6 and semantics. Go tests prove FB/IG numericalseq, Graph null, all-filtered raw page false with cursor progress,100→99 trusted EOF, emptyraw EOF and FB explicitfalse.

## Red→green evidence

| Scenario | Red exit/count | Green | Evidence |
|---|---|---|---|
| Public re-login / expired older cursor | 1,2 intended failures | 0,2 | red-client-final.log / green-client.log |
| IG earliest50 / unknown authority | 1,2 | 0; cohort9 | red-ig-p1.log / green-ig-p1.log |
| Seq Go wire/service FB/IG/history | 1 unit +1 REAL_PG | package48 / PG7, exit0 | seq-dto-evidence/ |
| Strict seq parser | 1,1 | 0,1 | seq-parser-red.log / seq-parser-integrated-green.log |
| Numeric coverage/out-of-range/null/epoch0 | 1,4 | 0,4 + legacy cohort | seq-coverage-red-final.log / seq-coverage-green.log |
| Rebuild/delete-behind60/history/manual/IG75 | 1,4; history-live case1 | 0,7 | window-refresh-red.log / window-refresh-final-green.log |
| No rebuild mutation negative controls | 1,2 | 0,2 | window-refresh-negative-red.log / window-refresh-negative-green.log |
| Review P1 cursorless epoch/history capacity975+25 | 1,3 | 0,3 | client-review-red.log / client-review-green.log |
| Last IG seq100 deleted without EOF | 1,1 | 0 after trusted EOF | ig-eof-required-red.log / ig-eof-consumer-green.log |
| Raw scan/exhaustion Go service | 1 unit +1 REAL_PG | package8 / PG2, exit0 | scan-dto-evidence/ |
| Strict EOF parser | 1,1 | 0,2 incl seq | scan-parser-red.log / scan-parser-green.log |

Exploration failures are preserved: strip-only parameter-property import in red-client.log; worker timeout suffix/dependency errors; first100-webhook fixture allocated too many pools (53300), corrected to reuse one real pool with all100 signed-webhook assertions unchanged. These are not counted as successful acceptance. Existing frozen behavior/assertions remain; legacy created_at-floor expectations were updated to the explicitly approved numeric-coverage contract, preserving strict covered-ref deletion and adding edge/null protections.

## Final-source gates

| Command | Exit / count | Evidence |
|---|---|---|
| GOTOOLCHAIN=go1.27.2 go build ./... | 0 | go-build-final.log |
| GOTOOLCHAIN=go1.27.2 go vet ./... | 0 | go-vet-final.log |
| gofmt -l on7 changed Go source/test files + assert empty | 0, empty | gofmt-final.log |
| GOTOOLCHAIN=go1.27.2 go test -race -count=1 -v ./internal/integrations/metabridge ./internal/integrations/metareply ./internal/live | 0,49 top-levelPASS | go-packages-final.log |
| LC_TEST_LOCK_WAIT=14400 LC_FOCUSED_TIMEOUT=900s bash scripts/dev/test-focused.sh '^TestLiveConsoleLCN(01BufferCapAgeAndCursor\|02\|04NoCommentTextPersisted\|05PrintAndMarks)' | 0,8PASS/0FAIL/0SKIP | pg-final.log |
| bash scripts/dev/test-node.sh | 0,1262/0FAIL | node-final.log |
| pnpm --filter admin exec tsc --noEmit | 0 | tsc-final.log |
| bash scripts/dev/check-gates.sh | 0,82 modes/1251 top-levelGo inventory | gates-final.log |
| LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-live-console | 0,23/23; Go322.28s | browser-final.log / browser-final-results.txt |
| LC_TEST_LOCK_WAIT=14400 LC_CONSOLE_CALIBRATION=retain-on-reset LC_BROWSER_CONSOLE_GREP=LCU2_RESET bash scripts/dev/test-local.sh --browser-live-console | 1,expected single RESET failure | calibration-final.log / calibration-failure.txt |

All PG/browser modes strictly serial on heartbeat lock; source stays fixed at0e5ba787. Go package tests have no PG fixture. SHA256 binds the final changed source subset in source-final.sha256. Generated evidence retained; other agents' untracked output and existing historical tracked evidence preserved.

Browser artifact: output/playwright/live-console-1575136830/. Final native-hidden evidence: hidden starts0, trusted hidden event, server31→31. Calibration artifact: output/playwright/live-console-413473935/; its only failure is live-console.spec.ts:254 LCU2_RESET, exact reason **LCU2_RESET old epoch retained**, expected0/received1. This is intentional E3 sensitivity evidence, not a passing runtime gate; the normal23/23 run includes the same reset case green. No source/threshold change during calibration. All owned commands/servers/fixtures exited.

Final three UI files were code-indexed (40 entities); useCommentStream was linked to the raw-scan fix memory3da42695-8067-4a4a-b159-9032dcaa2fb0. Go worker indexed/link-recorded its owned files independently. Own helper worktrees' KEEP evidence was collected; no other task directory was removed.

## Limits / CI gates

MOCK + REAL_PG author E3 within the tested environment, not LIVE or independent E4. Backend deletion-eviction PR24 is not merged in this base; upstream user-deletion detection remains its responsibility. Client tests establish cleanup after server no longer returns a ref, not real Meta deletion behavior. IG100 selection/draft/rebuild checks use actual production-source Node hooks with synthetic DTOs; real PG covers actual IG DTO/scan projection. New IG-specific full-browser seed remains NOT_RUN (not needed to fake upstream deletion).

Independent K3/new required PR CI, full foundation/G07 (no new migrations/grants/checkout write path), physical devices and Meta SANDBOX/LIVE/production are NOT_RUN here. Integrator-declared unrelated5e3ce199 payment/promotions/storefront/g10 failures were untouched. R04 binary absence remains the existing runner's explicit skip.

CI gates: --browser-live-console plus repository-selected required PR modes. Integrator owns review replies/push/merge; author commits and stops. No open authority question remains for the implemented DTOs.
