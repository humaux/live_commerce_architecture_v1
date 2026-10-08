# fix-untracked-stripe-capture — K3 round 1 delivery

- Branch: `unit/fix-untracked-stripe-capture`; initial base `e705f70a`; this round starts at `852b515f`.
- Current executable source: `fb238b1ed540ee2de38889c4bda0650a1ad90c5d`. The final delivery commit contains evidence only. No push.
- Author: Codex-4 / GPT-6 family (exact runtime model/effort not exposed). Same dedicated worktree `.worktrees/fix-untracked-stripe-capture`; SQL + foundation tests only.

## K3 P2 fixed

A valid empty-plan order whose SKU becomes tracked or disappears before paid reconciliation now keeps the authenticated CAPTURED fact and records the existing `PAID_ALLOCATION_FAILED` review case plus a `REVIEW_REQUIRED` work item. It returns before order confirmation, reservation commit, inventory movement or settled-order events. This is evidence of real money received, not permission to fulfill or automatically settle the order.

The capture-side proof in unmerged migration 0167 first rejects malformed/empty quote data and any historical RESERVE evidence. If the current scoped catalogue cannot justify the empty plan, durable review is allowed only when the order quote exactly matches its original immutable `storefront.quotes` row (tenant/store/owner/quote ID scoped). Forged/cross-store snapshots and missing tracked reservations remain PT409; all original ten refusal assertions are unchanged. The close-side guard is unchanged. No new table, function, role, grant or reason code; owner/SECURITY DEFINER/search_path/COMMENT stay exact.

The old statement that catalogue drift remains an unresolved paid-recording limitation is superseded by this round. It stays fail-closed to settlement, with durable operator evidence. Current code mapped PT409 to River JobCancel rather than a retry from that individual reconcile job; absent terminal evidence the query path could keep producing observations. The new review path commits successfully, and its CAPTURED flag makes the existing query worker finish `stripe_terminal_observed` before making another HTTP request.

## Red → green evidence

New `TestStripeUntrackedDriftReview` uses a real catalogue edit and a disclosed owner-only deleted-SKU fixture, real Begin/Stripe initiation, actual CaptureWorker.Work, and the production payment-worker assembly over MOCK transport. Before the fix both cases fail with `JobCancelError: payment_capture_invalid_job`. Afterward:

- Exactly one authenticated CAPTURED fact (correct amount/currency/report hash), one review case and one review work item survive commit and replay.
- Order remains AWAITING_PAYMENT; reservation remains PAYMENT_PENDING; no ALLOCATE/RELEASE or settled-order event is written.
- PaymentView reports REVIEW_REQUIRED. Replaying after changing the SKU back to untracked cannot settle.
- The remaining query job completes as `stripe_terminal_observed`, with zero additional provider HTTP requests.
- Existing normal all-untracked/mixed capture+close tests and all ten corruption refusals still pass.

All current gate commands, exit codes, PASS/SKIP counts and log hashes are in **`k3-r1/results.json`**. Current six source hashes are in `source-hashes.json` and `k3-r1/source-hashes.json`. Root `results.json` and compressed logs remain historical evidence from the first batch, not this round.

| Gate | Exit | Result / plaintext log |
|---|---:|---|
| Focused new drift test before fix | 1 | 2 expected failing cases; `k3-r1/red.log` |
| Untracked + RF12 + SP06 | 0 | 7 top-level PASS, 0 FAIL/SKIP; `k3-r1/green.log` |
| Foundation Stripe SP/SL/RF/Untracked | 0 | 40 top-level PASS, 0 FAIL, 2 top-level SKIP; 574.823s; `k3-r1/stripe-regression.log` |
| Stripe/payment/refund package gates | 0 | 37 top-level PASS, 1 SKIP; `k3-r1/stripe-package-gates.log` |
| R2 release-head upgrade + KC03 + CRP02 | 0 | 3 top-level PASS, 0 FAIL/SKIP; `k3-r1/upgrade.log` |
| test-node.sh | 0 | 1137 PASS, 0 FAIL; `k3-r1/node.log` |
| admin typecheck | 0 | `k3-r1/typecheck.log` |
| check-gates.sh | 0 | 82 modes documented, every tracked test assigned, headers PASS; `k3-r1/check-gates.log` |

**Evidence class: E3 in tested REAL_PG + MOCK environment.** All logs above are uncompressed text. The committed typecheck view trims only its final blank line for the Git whitespace check; raw stdout stays at the canonical path, and both hashes are recorded. Earlier first-batch `red-valid.log` is also now directly readable in this worktree; `red.log` retains the earlier diagnostic fixture error and is not the valid first-batch red proof. Gzip copies are supplementary, never required by the reviewer.

Canonical evidence directory: `/Volumes/data/live_commerce_architecture_v1/output/fix-untracked-stripe-capture/`. Same text logs/manifests are committed under this unit's worktree output directory.

## Review and remaining CI

Read-only review of fb238b1e found no P0/P1/P2 in the authorized scope and independently verified that restoring the two marked proof blocks produces the exact 0062 function body. It also verified the unchanged original refusal tests and the new worker RED log. This is E1 source review, not an independent runtime rerun (Humaux `3ca254d3-b7f0-401e-b52b-99d313c632f9`).

- CI gates: foundation-shards and full `bash scripts/dev/release-gate.sh --strict --only G07`; integrator's final money review. Full G07 remains NOT_RUN locally under the Mac RAM rule.
- SANDBOX NOT_RUN: SL08, RF10, SP16 and nested SP21 real probe; no owner test credentials/opt-in used. LIVE/provider capture/refund/deployment NOT_RUN.
- Existing RF03 nested populated-upgrade-from-0061 case remains NOT_RUN (no partial-apply hook); the three listed upgrade gates actually ran.
- Package regex matched no tests in settlement/stripeadmin/stripewebhook packages; those empty selections are not counted as test coverage. Foundation registrar/webhook gates ran separately.
- Migration count remains 92; R2/CRP pins do not change again because 0167 is the same unmerged migration. No merged migration edited.

All author-started test processes finished and the existing scripts cleaned their disposable PG fixtures. No shared cache or other task artifacts removed. Code committed; integrator pushes; no production action.
