<!-- Purpose: LC-U2a implementation, current-source gate receipts and independent-review handoff.
Depends on: frozen LC-U2a brief, live-console-v1, SOURCE-SHA256.txt and logs in this directory.
Used by: integrator/K3 and the eventual PR; no production or LIVE Meta acceptance claim. -->
# LC-U2a delivery

- Branch: `unit/lc-u2a-comment-stream`.
- Tested implementation: `35795208da5016c6206a101beb16e4eec8987c86`; final delivery commit is evidence-only.
- Assigned base: `a698d44c`; brief `4ecbc161`; merged remote trunk `d3fd1026` in `f2626fe3` before browser gates.
- Implementer: Codex-1 (host does not expose the exact model identifier). Test-only helper: `test_worker`, inherited model/high reasoning, separate `unit/lc-u2a-fixture` at base `4ecbc161`, commits `11353718` and `f948740b`. Targeted read-only privacy review: `security_reviewer`, inherited model/high reasoning. Integrator K3 pre-push review remains pending.
- Scope: UI/BFF plus the user's explicit **Go test-fixture-only** exception. No production Go, SQL/migration, OpenAPI, dependency/lockfile, deployment, real Meta, or production-message changes.

## Implemented

1. Console middle column: A2 visible-only 3-second single-flight polling, capped 3/6/12/24/30-second failure backoff, epoch/reset clearing and signed older-page cursor pagination. Private memory and selected buyer/composer clear on hiding or authority loss.
2. All/keyword/private/unreplied filters; A8 remains server-scoped to `filter=live_comment&session_id`. Right column reuses **BuyerPanel**, unchanged, only via a returned `bundle_id`; no identity guessing.
3. A4/A5 explicit private/public replies and published templates, three locales, rule hints and closed refusal codes. One-private-reply quota, preemption confirmation, public-payment-link refusal, IG-live-public unsupported and capability restrictions remain server authoritative. Restricted-buyer marks show a warning before manual send.
4. Reply uncertainty survives selection/navigation/reload via a coarse **auth/store/session boolean**. It contains no comment reference, PSID, buyer name or reply body. It is armed before sending; queued/UNKNOWN require explicit external verification before further replies. Verification unlocks only, never resends. Storage failure fails closed. SQL uppercase operation marks map to the five visible delivery states; `UNKNOWN` stays fenced without a local receipt too.
5. Exact A2–A5 grammar is wired through **both** the raw Next proxy and catch-all BFF. Closed methods/query/body keys, session/CSRF/Origin, no-store/no-referrer, no diagnostic text reflection. A3 transport exists, but there is no printing UI or create-order drawer.
6. Real Go/PG acceptance fixture with a loopback MOCK Graph edge, 60 comments, actual claim/bundle, six separate one-shot replies, actual bridge epoch replacement, count-only diagnostics and masked screenshots. Existing LC-U1 assertions retained.

## Gate receipts

All final commands below use implementation commit `35795208`; logs are in this directory. Evidence class: **REAL_PG + MOCK Graph/IdP** for LC-U2a browser; existing LC-U1 upstream scenes remain **MOCK**. No LIVE acceptance.

| Command | Exit | Result / evidence |
|---|---:|---|
| `bash scripts/dev/test-node.sh` | 0 | 1062 tests; `node-final2.log` |
| `pnpm --filter admin exec tsc --noEmit` | 0 | `tsc-final2.log` |
| `bash scripts/dev/check-gates.sh` | 0 | G-UI1/3/5, headers, 81 documented modes; `gates-final2.log` |
| `bash scripts/dev/depmap.sh --check` | 0 | `depmap-final.log`; no production Go graph change |
| `gofmt -l tests/foundation/browser_live_console{,_comments_fixture,_diagnostics}_test.go` | 0 | no filenames printed |
| `go test -tags browser ./tests/foundation -run '^TestBrowserLiveConsoleDiagnosticPrivacy$' -count=1` | 0 | `go-diagnostic-final.log` |
| `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-live-console` | 0 | **20/20**; `browser-final.log`, `evidence/console-playwright.log` |
| `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-inbox` | 0 | **13/13** plus Go diagnostic subtest; `inbox-final.log`, `evidence/inbox-playwright.log` |
| `LC_TEST_LOCK_WAIT=14400 LC_CONSOLE_CALIBRATION=retain-on-reset bash scripts/dev/test-local.sh --browser-live-console` | **1 (expected)** | **19 passed / 1 failed**, only `LCU2_RESET`; `calibration.log`, `evidence/calibration-playwright.log` |

No parallel `test-local`/PG modes ran. Each harness was bounded and its owned services terminated on completion. Standalone DB-free Node/type/static checks ran alongside browser execution without source edits.
The clean helper checkout `.worktrees/lc-u2a-fixture` was removed after its scoped commits were incorporated; branch `unit/lc-u2a-fixture` still retains `f948740b` for recovery/review. Shared caches and other worktrees were not cleaned. Failure logs remain; temporary native browser profiles were removed by their owning harnesses.

### Red → green and diagnosis

- Raw Next proxy: `proxy-red.log` → `proxy-green.log` (A2 previously 404 before reaching Go; first browser facts showed **zero A2 requests**).
- Signed bridge cursor: `signed-cursor-red.log` → `signed-cursor-green.log` (real Go returned 50 rows, but `payload.signature` failed a base64-only UI check).
- Hidden/reveal lifetime: `hooks-first.log` → `hooks-green.log` (old cleanup reopened a hidden fence and prevented reveal).
- Remount uncertainty: `receipt-red.log` → `receipt-green.log` (queued/unknown public receipt was lost with the composer).
- SQL UNKNOWN: `sql-mark-regression-red.log` → `sql-mark-green.log` (real uppercase operation mark did not activate the guard). `sql-mark-red.log` is an earlier test-registration/TDZ failure, **not** semantic red evidence.
- `node-red.log` is initial module scaffolding failure; `grammar-dependency-red.log` is missing offline dependencies, **not** a passed grammar gate. Later `grammar-installed.log` passed.
- Initial browser collection failure (`test.use` inside describe) is retained in `browser-first.log`; fixed at top-level. `browser-second.log` retains the 9-new-fail/11-existing-pass proxy diagnosis. No failed assertions were removed or weakened.

## Evidence and review boundaries

- Final original console evidence: `output/playwright/live-console-441866882/` (20 passed). Privacy-safe committed subset: `evidence/console-counts.json`, A2 count-only `comment-reads-*.json`, `console-playwright.log`, six `comments-<locale>-<width>.png` screenshots (1440/390 × zh-TW/zh-CN/en).
- Inbox original evidence: `output/playwright/inbox-ui/20261008T110217.661963000/` (13 passed). BuyerPanel was not modified. Existing inbox evidence policy remains its own gate; this unit does not copy its private-page screenshots.
- Calibration original evidence: `output/playwright/live-console-3256261478/`. Sole failure is `live-console.spec.ts:170`, assertion at line 174: old-epoch row expected **0**, received **1**. The other 19 cases passed. This run enabled only the existing loopback-only calibration switch; normal source/thresholds were unchanged.
- Screenshots intentionally mask the comment-list viewport, selected buyer data and reply textarea. Masking offscreen row children produced false overlay rectangles outside their scroll clip, so the bounded list region is masked instead. Visual assertions also check viewport overflow; visual-lint still belongs to CI.
- Targeted non-author review found the remount-UNKNOWN P1; source closure was checked at `7aee2e9f`. Runtime closure is the final real-Go lost-ACK/select/reload case plus Node tests. This is **not** the independent integrator/K3 pre-push review.

## CI gates / NOT_RUN

- Required CI review gates: `--browser-live-console`, `--browser-inbox`, `--browser-click-sweep`, `--browser-visual-lint`.
- The repository selector (`node scripts/dev/pr-modes.mjs origin/r3/integration HEAD`, exit 0) conservatively selects **51 workflow modes** because the raw proxy/Next configuration changed. The exact complete list is `pr-modes.json` (`deploy:false`); do not narrow it by hand. The first one-argument invocation failed usage validation; the corrected two-argument invocation generated this file.
- NOT_RUN: CI click-sweep/visual-lint, the selector's other broad modes/foundation shards, independent K3/PR checks, LIVE Meta and production. The Node runner also explicitly reports `tests/media/r04-input-runner.test.mjs` NOT_RUN because `COMMERCE_R04_LIVEKIT_BINARY` is unset (unrelated media runtime).
- No push, PR creation, merge into another branch or deployment. Integrator owns push, K3 review and opening the PR.

## Remaining limitations

- A2 has a public-reply count, not an authoritative public-operation terminal-state read. The UI intentionally uses the coarse verification guard rather than inventing successful delivery or silently resending.
- No print workflow, buyer-order creation drawer or general inbound-DM composer is added. Existing inbox remains the DM surface; A8 views here select a returned bundle for BuyerPanel.
- Screenshots use synthetic fixtures; masked content is not a visual acceptance of real buyer data. Final acceptance is bounded to the tested environment (E3), not E4/production.
