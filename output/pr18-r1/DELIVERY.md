<!-- Purpose: PR18 round-one corrective batch, source-bound gate ledger and review handoff.
Depends on: pr18-d5c865a5 triage packet, frozen live-console contract and the real local/CI evidence below.
Used by: integrator K3 pre-review and PR18 CI; not production or merge approval. -->
# PR #18 round 1

- Branch `unit/lc-u2a-comment-stream`; base `d5c865a5`. Fetched/merged own remote and origin/r3/integration; both already up to date.
- Final runtime/test source **`673c0be59c2cf34e06676476253392e27cd5f5d1`**. `9aa4a07a` fixes review findings; `673c0be5` fixes scroll readiness in the attribution test. Final delivery commit changes evidence only.
- Codex-1 authored UI/BFF/tests. One read-only explorer independently inspected CSS and the CI scroll trace (inherited model identifier unavailable, high reasoning, no code writes/browser/PG). Integrator still owns independent batch review, push and merge.

## All review findings

| Comment | Root fix / regression |
|---|---|
| 4223328013 P1 | A normal `queued` acknowledgement clears the coarse session receipt; per-comment send state stays visible. Actual `unknown` and lost ACK remain fenced. Actual-component tests cover both states after comment remount and storage contents. The old queued-block assertion was corrected by the integrator's contract ruling, not silently removed. |
| 4223328020 | `422 invalid_ref` is preserved by A3 through the real Request/BFF seam, non-retryable. |
| 4223328029 | A8 failures wait 10/20/30 seconds (then cap), respecting larger Retry-After; healthy reads remain 10 seconds. Both private/unreplied fake-clock tests pin every boundary. |
| 4223328037 | A8 state joins A2's synchronous privacy clear callback. A counterexample inspects actual state slots immediately after hide, before render/passive effects. |
| 4223328043 | Shared malformed-cursor classifier feeds Next proxy and the catch-all: negative/non-integer, unpaired, duplicate cursor keys and mixed before/after →400 `invalid_cursor`. Unknown/private keys remain422. Duplicate handling follows this packet's explicit ruling; product Go was not changed. |
| 4223328047 | Head refresh merges into the loaded A8 set and retains its history cursor; pagination merges current state by conversation/bundle identity and blocks overlapping pagination calls. Fake-clock test loads a tail, refreshes head, and proves the tail and exhausted cursor survive. |
| 4223328055 | GET A2 alone uses8MiB. Its admitted limit is100, not only the UI's50:100 × (8,000 text +255 name) runes ×6 bytes (worst Go JSON HTML escape)=4,953,000 bytes, plus bounded marks/ref/envelope. Real BFF tests cover long CJK and escaped HTML-sensitive pages. Other A8–A14/command caps stay1MiB and still reject those large bodies. |
| 4223424434 | A2 still polls its incremental cursor; at most once per10s it additionally rereads the recent50-row window for async marks, preserving both incremental and historical cursors. Tests prove claim-null→claim-completed without a new sequence, unchanged selection, keyword visibility and the reused BuyerPanel. Older than that recent window still requires an explicit refresh; no new Go endpoint/contract. |
| 4223424448 | At1000 rows, history loading stops without a request/cursor advance or row discard. The button is disabled with three-language guidance. A partially filled history merge keeps the returned older page rather than dropping it from the front. Full-buffer regression retains the original rows and both cursors. |

## Ads-attribution CI failure: measured cause, not a CSS patch

The CI red is **local table `scrollLeft` after ArrowLeft**, not document horizontal overflow. The exact-zero assertion remains. No attribution CSS, shared tokens, shell or table component differs from trunk; LC-U2a's CSS additions are `.comment-*` / `.live-console-*` scoped.

CI run37831840615 / job113499043533 trace (SHA256 `d9ce9c382fd59d345c1e85ebd4a52edeea9c19c2c441af0d33ea65678c8f4854`): right key15032.659–15048.319ms; frame scrollLeft38 at15180.111; left key15194.555–15211.022; frames40 at15200.121/15209.066/15220.838, and still40 at20176.823. The test used the first positive intermediate position as readiness; it had no scroll-settlement boundary. The trace does not identify the exact internal browser keydown/animation ordering more precisely.

The untouched local `d5c865a5` full ads gate passed (`ads-before.log`, exit0), consistent with a timing-sensitive test. `tableLayout` now arms a read-only native `scrollend` observer, performs the real keypress, awaits settlement, then reverses. Focus, positive displacement and exact return to zero are retained; no fixed sleep, timeout increase, retry, scroll assignment, style injection or overflow hiding. The observer has a5s failure timeout, shorter than the original10s assertion.

## Red → green and gates

`node-red.log` contains the intended old-source receipt/A8/marks/cap/history failures. It also contains one unrelated loader failure because that initial combined command used strip-types for a test requiring transform-types; that failure is **not** counted as a product red. `proxy-red.log` separately proves the proxy400 regression with the correct transform-types mode and cursor guard removed/restored. `node-focused-green.log` runs all four real-source test files with transform-types: **49/49, exit0**.

| Command | Exit | Evidence |
|---|---:|---|
| `bash scripts/dev/test-node.sh` | 0 | `node-final.log`,1176 tests,0 failures |
| `pnpm --filter admin exec tsc --noEmit` | 0 | `tsc-final.log` |
| `bash scripts/dev/check-gates.sh` | 0 | `gates-final.log`,82 modes/all documented, headers/architecture |
| `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-live-console` | 0 | `browser-live-console.log`,20/20 browser cases |
| `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-inbox` | 0 | `browser-inbox.log`,13/13 browser cases |
| `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-ads-attribution` | 0 | `browser-ads-attribution.log`, report + checkout Go tests both PASS; report6/6 browser cases |
| `LC_CONSOLE_CALIBRATION=retain-on-reset LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-live-console` | **1 expected** | `calibration-retain-on-reset.log`: **19 PASS / 1 FAIL**, only `live-console.spec.ts:170` (`LCU2_RESET`); calibrated defect detection, not a normal PASS |

All browser/PG runs are strictly serial; runtime/test source remains frozen while running. Evidence class: actual-source Node MOCK plus real Next/Go/PG browser fixtures with MOCK Meta. No external customer messages or provider mutations.

Safe committed excerpts live under `evidence/`: console20, inbox13, ads report6, ads checkout, the exact reset calibration coordinates, CI red and native-scroll timing. Original browser artifacts are in this worktree's `output/playwright/live-console-2902780133`, `live-console-963366430` (calibration), `inbox-ui/20261008T204712.529477000`, `ads-attribution-report/20261008T204824.390631000`, and `ads-attribution-checkout-admin/20261008T204920.226170000`. `SOURCE-SHA256.txt` binds every changed runtime/test file to the tested source.

**Local requested batch complete at E3.** All owned commands/harness services have exited; no background test remains. This is not independent merge approval.

## NOT_RUN / handoff

- Required PR CI and independent K3 batch review on the new head; full foundation/G07, global click sweep/visual lint (not requested locally in this packet).
- LIVE Meta, production and deployment. Unrelated R04 input-runner is explicitly NOT_RUN in test-node because `COMMERCE_R04_LIVEKIT_BINARY` is unset.
- CI download (including its trace/network archive) stays local and is excluded from the evidence commit; committed diagnostic excerpts contain no credential headers or private payloads.
- No push/deploy/review-thread mutation. W3-U3 remains separately paused at22808172 pending its Node-host-only scope ruling; PR16 was already delivered asf313e418; settings-followups remains queued after W3-U3.
