# fix-webkit-order-gate delivery

- Branch: `unit/fix-webkit-order-gate`; base: `424f17ccd3fd8db00923d91d3d23ca8bfafc249f` (`origin/r3/integration` at start). Commit: the commit containing this delivery; code hashes in `verified-source-hashes.json`.
- Owner: Codex-3; worktree: `.worktrees/fix-webkit-order-gate`. Primary model/effort inherited from this session (exact identifier unavailable); read-only explorer: `gpt-6.1-sol`, high. No delegated code writers.
- Write paths: `apps/storefront/components/OrderFlow.tsx`, `tests/storefront/order-gate.mjs`, `tests/foundation/{browser_order_chain,buyer_order_evidence}_test.go`, this evidence directory. Contract/API changes: none.

## Root causes and fixes

1. **Product layout race, visible to WebKit buyers.** When the real destination-head request completes, `OrderFlow` removed its loading paragraph. A diagnostic run on the original product naturally stalled a locale switch: trusted pointerdown/mousedown targeted the anchor, the document and scroll position shrank by 39 px, and pointerup/click targeted a DIV; no navigation followed. This is not a cancelled handler or a missing `waitForURL` listener. The current hit-test still reported the anchor, so the evidence supports native WebKit retargeting during layout/scroll anchoring, not a claim that the pointer physically left the link. Retain the notice line's space after loading; preserve all status/error text and roles, hide only the empty spacer from accessibility.
2. **PR #7 audit lifecycle bug, independent of engine.** On `origin/unit/tz-audit` at `08a54057862d14688d6d8bffea48410a9eb63699`, the UTC history context is intentionally closed but remains in `contexts[]`. BO06 later calls `cookies()` on it. Capture its cookie flags and page storage before intentional closure, then include its snapshot in the final audit after every context has supplied its secret canaries. Preserve all attempted-write/URL/console/page-error checks. The mainline regression closes an existing finished buyer context; no new order fixture is needed.
3. The Go harness now requires the exact 23 existing observations plus both new layout checks. It rejects unknown, duplicate and missing observations. PR #7's two renamed history observations are explicit aliases; its UTC observation is the only optional extra. Original exact order/hold/job/receipt/reserve counts remain unchanged.

## Red evidence and non-vacuity

Evidence subpaths below are preserved inside `evidence.tar.gz` (extract with `tar -xzf evidence.tar.gz`); its logs are gzip-compressed without changing their contents. `MANIFEST.json` verifies every archived member and standalone summary. Full logs/screenshots remain under the main checkout's `output/fix-webkit-order-gate/`.

- `red-evidence/baseline-01`: original trunk WebKit order passed once; the defect is intermittent.
- `red-evidence/diagnostic-02`: original product, repeated existing locale loop and read-only native event instrumentation, failed with the original 30 s `switchLocale` timeout. `last-native-events.json` and full compressed trace retain the anchor-down / DIV-click evidence. No injected click or artificial event cancellation.
- `red-evidence/layout-red`: hold/release the **real** `GET /api/buyer/destination` response; footer document top changed from `2639.28125` to `2600.484375`. The new stability assertion failed before the product fix. Shipping regression runs this causal transition at desktop and 390×844 mobile sizes, then uses a real click/native WebKit tap to switch language.
- `red-evidence/diagnostic-01`: deliberately close the existing finished buyer context before BO06; original audit failed with `browserContext.cookies: Target page, context or browser has been closed`, after BO02 passed. The closure remains in the shipping regression.
- `red-evidence/privacy-mutation-red`: temporarily write a canary through a direct localStorage property (bypassing the `setItem` observer), assert the observer did not see it, close the context, and only then register the canary as a secret. The final saved-storage assertion failed with `PII/bearer leaked into persistent storage`. Mutation removed before all acceptance runs.
- `evidence-shape/exact-cases-red.log.gz`: unknown/duplicate replacement of a core observation passed the initial count-only checker, so the new negative tests failed. `exact-cases-green.log.gz`: the exact-set checker passes; `test-focused: top-level PASS=1 FAIL=0 SKIP=0 exit=0`.
- A fixed-product diagnostic stress run completed 150 locale switches and all 25 browser observations, but the old Go count guard rejected the two added cases. It is retained in the full evidence as exit 1 and is **not counted** as a green gate.

## Acceptance

Five final WebKit runs (07–11), final Chromium and PR #7 compatibility WebKit (26 observations, same exact seven-order database facts) exited 0; focused Go, Node (618 tests), both typechecks, check-gates and browser-tag vet exited 0. Final run results are recorded in `verified-series.json` and per-command status/log files. All primary-worktree acceptance runs pin source digest `cba5fb94b7d531ab9f3760a455a4a7f2d93f24830d5fcdc50d1f34932d5675f1` and verify it did not change during execution. The PR #7 copy has its own recorded source digest. No timeout increase, automatic retry or assertion removal was made.

- Five sequential runs: `LC_BROWSER_ENGINE=webkit LC_WEBKIT_STEPS=order bash scripts/dev/test-local.sh --browser-webkit` (order scenario only).
- Chromium: `LC_BROWSER_ENGINE=chromium bash scripts/dev/test-local.sh --browser-order`.
- PR #7 compatibility: apply the patch to a disposable detached copy of `08a54057`, retaining the Los Angeles/UTC contexts, date assertions, and explicit UTC close; run the same WebKit order command. Patch/source hashes and logs are archived under `compatibility/`.
- Focused Go: `bash scripts/dev/test-focused.sh '^TestBuyerOrderEvidenceCompleteness$'`.
- `bash scripts/dev/test-node.sh`; `pnpm exec tsc --noEmit -p .` in both apps; `bash scripts/dev/check-gates.sh`; `go vet -tags browser ./tests/foundation`.

## CI gates / evidence boundary

- Directly affected modes: `--browser-order` and `--browser-webkit` (order step). The local five-run proof explicitly narrows `LC_WEBKIT_STEPS=order`; it does not certify all seven WebKit scenarios.
- The checked-in planner at this base requires two CLI refs, so the equivalent pre-commit command is `node scripts/dev/pr-modes.mjs --stdin` with the four changed source paths. It selects 48 modes (including foundation shards); the exact list is `pr-modes.json`. The full matrix is NOT_RUN locally; the integrator runs CI after pushing. This unit follows the user's explicit local five-WebKit-plus-Chromium proof scope rather than running the full matrix on the shared Mac.
- Evidence: **E3, BROWSER + REAL_PG, MOCK external services**. Production Next/Go routes, task-owned PostgreSQL, synthetic TLS/buyer data. Physical iPhone/system Safari, provider SANDBOX/LIVE, full foundation/race and G07: NOT_RUN. Repeated successful runs reduce the observed flake; they are not proof of every possible WebKit layout interaction.
- No backend/product API changes, no fixtures weakened, no new dependencies. Local task-owned browser/server/PG processes and the disposable PR #7 comparison checkout are cleaned up; evidence is retained. Humaux fix records and own canvas updated; the index accepted three of four files (the `.mjs` symbols were not exposed), with rationale linked to `OrderFlow` and the Go evidence checker. No push or GitHub thread reply. Integrator opens/reviews/pushes the PR.
