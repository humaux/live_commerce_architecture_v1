# FIX-LATENT-BUNDLES-FIX1 — PR #478

Date: 2026-11-01. Status: implementation proposed; own assessment E0, static syntax E1; PR CI pending.
Branch: `unit/fix-latent-bundles`; worktree: `.worktrees/fix-latent-bundles`.
Base: `4b8c04c7213025c173e260db6f9c81cb5b818c61`.
RED head: `9a718122fb71dc969107dc5efa7b54f02090e943`.
Role: dispatched Codex implementation lane; exact runtime model/reasoning are not exposed.
Authority: owner FIX1 card, AGENTS.md, architecture §§0/1/2/26/27,
`contracts/buyer-order-ui-v1.md` (BO04–BO07), current source and original GitHub evidence.
Humaux tools are absent in this session: memory/coord/canvas/index/link operations are unavailable;
no shared-state write or independent review is claimed. Existing local `FIX.md` predates this card.

## Integration and RED evidence

Fetched `origin/r3/integration`, then `git merge --no-edit origin/r3/integration`: exit 0,
Already up to date. No other branch/worktree was written.
Existing tests-only trigger `9a718122` is the RED commit; it changes header comments and selection hashes,
not product or test behaviour. Gates `41282493913` executed merge SHA
`31e73f6309e915fd30e47a836c9c97df764f279c`.
The added MODEL_ONLY regression below has not had a separate isolated RED run;
the already-failed real BO05 assertion is the red-first evidence for this directed, single-push fix.

| Failing job | Failing test / exact evidence | Class and action |
| --- | --- | --- |
| `124778237011`, `gate (--browser-webkit)` | `TestBrowserBuyerOrderUI`, BO05 locator-storage failure recovery. `order-gate.mjs:627` calls `created`, whose `:153` assertion fails: `getByTestId('order-section')`, expected visible, element not found, 5000ms. Artifact browser log and `failure-6.txt` show the page still displays Checking order status / Check order status. CVS in this same job passed (`pass=2 fail=0`). | **(b), interaction/helper repair candidate**, not a stale signed-frame assertion. Synchronize the shared real recovery click with page idle and native focus before pointer activation. The exact original failure's focus/compositor cause remains a hypothesis until CI validates the repair; no product-regression attribution is proven by the comment-only trigger. |
| `124786249686`, `receipts` | `matrix-receipts: RED (1 problem(s))`; `FAIL: --browser-webkit: status FAIL`. The WebKit receipt has `test_count=139`; this is not a missing browser-spec sentinel. | Consequence of (b) above; do not change receipts or its thresholds. |
| `124786325493`, `required` | `plan=success build=success ... gate=failure aggregate=success ... receipts=failure`. | Consequence of (b) above; do not change required/aggregate. Aggregate itself passed. |

No class (a) UI rewrite is justified. No class (c) identical-base failure is claimed:
the supplied green base run `41277975588` did not select WebKit or browser bundles.
It therefore cannot establish whether BO05 would pass on the base. Original current-PR target
CVS/WebKit (`124778237038`) and inbox (`124778236979`) both passed; no speculative repair to those cases.
The known inbox helper issue and evidence retention exemptions are not edited.

## Evidence pointer (no raw artifacts committed)

- Run: https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41282493913
- WebKit job: https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41282493913/job/124778237011
- Artifact `gate-browser-webkit`, id `13074304456`, archive SHA256
  `a1620635e39940cd0df9bd92d6895e184d835aea3e0a3f255e9bf22fa521811d`.
- Inner evidence: `playwright/buyer-order-1467609188/browser.log`,
  `playwright/buyer-order-1467609188/failure-6.txt`,
  `playwright/run.VSmJFjn2/webkit/31e73f6309e9-order.jsonl`.
  Archives were inspected in memory only; no raw logs/screens/traces/downloads are retained.
- Prior Foundation: run `41282493728`, job `124777082408`, success for the RED head only.

## Bounded repair and preserved gates

`recoverPurchase` in the existing `tests/storefront/shop-helpers.mjs` waits for `.purchase-main`
to finish loading/working, then focuses the real recovery button, asserts focus, and clicks once.
All eight order-gate recovery sites and the COD locale/viewport recovery site use this shared helper.
The order suite already uses this focus-before-click sequence for BO03 quote-less reconfirmation
and BH history activation. The new helper does not create/replay API requests, change browser state
through evaluate, dispatch events, force clicks, retry, or extend timeouts.

The new MODEL_ONLY regression executes the actual BO05 call site and actual helper with the existing
wrong-target focus-scroll mutation. It checks idle-before-focus-before-one-click and both callers'
use of the shared path. Only GitHub real browsers can establish that this addresses the observed failure.
Every existing gate-adapter assertion remains verbatim and in order. BO05 still requires the owned DRAFT,
exactly two checkout POSTs with the original idempotency key, single hold/job/receipt, and recovery persistence.
No thresholds, fixtures, exception lists, gate logic, locales, Figma frames or product code are changed.
This is directed attempt 1 for the newly authorized WebKit order blocker; prior relays made no repair.
If the same assertion remains red, hand off BLOCKED with its exact current assertion/evidence; no third approach.

## Static checks and handoff

- `node --check` on the four edited `.mjs` files: exit 0 (syntax only).
- `node scripts/dev/derive-browser-ownership.mjs`, then `--check`: exit 0, 55 modes.
- `git diff --check`: exit 0.
- `node scripts/dev/check-evidence-retention.mjs --base origin/r3/integration`: exit 0;
  rerun on the committed diff before the single push.
- Acceptance: this PR's own Gates/Foundation, including real Chromium/WebKit order and existing
  affected modes. New MODEL_ONLY checks are supplementary, not UI acceptance.
- NOT_RUN: all local tests/servers (prohibited); new PR CI pending; provider SANDBOX/LIVE;
  independent source/browser review. No background local processes remain.

## Scope accounting

Additional owning-storefront paths beyond the original CVS/inbox write paths, under the card's
in-module pre-approval: `tests/storefront/shop-helpers.mjs`, `tests/storefront/order-gate.mjs`,
`tests/storefront/checkout-v2.mjs`, `apps/storefront/tests/order-gate-timeout.test.mjs`.
Generated ownership and `output/fix-latent-bundles/{FIX1.md,red.log}` are card/generated paths.
No component imports were changed, so component compiler harness updates are not required.
No Go imports were changed, so dependency-map regeneration is not required.
No migrations/contracts/OpenAPI/R2 pin/go.mod/lockfiles or another app/backend module were changed.
Integrator should review the additional owning-storefront paths listed here. No merge/auto-merge/deploy.

Final run/job IDs will be posted in the PR comment after this single push; no evidence-only follow-up commit.
The integrator relay is the wakeup path if CI remains queued at session end.

🤖 Generated with Codex (dispatched by Claude Code integrator).
