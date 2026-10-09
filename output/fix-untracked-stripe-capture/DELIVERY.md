# PR20 — K3 quantity-bound follow-up

- Branch/worktree: `unit/fix-untracked-stripe-capture`, `.worktrees/fix-untracked-stripe-capture`.
- Base: `bbb8728dbf79672fa2a157d574afbe9d6f2be95a`; tested source: `7499dfc78bfe7a856f9c8ea945d2be5ca93bed26`.
- Codex-4 / GPT-6 family; exact runtime model/effort not exposed. Coordination task `483c5aa5-8f65-43ea-8d90-95cf8a0cc81a`.
- K3 PASSed the preceding batch with one round-one P2: the two nine-digit quantity regexes excluded the inclusive SQL CHECK upper bound. Both capture and close now validate ten-digit syntax, then use a CASE-guarded bigint range `1..1000000000`. Malformed/overflow values fail closed as PT409. No interface, ACL, definer attributes, migration number/count or dependencies changed.
- New REAL_PG `TestStripeUntrackedQuantityBoundary` tests capture and close/replay, one terminal fact, correct order/reservation states, no stock movement or review. Seven invalid quote quantities fail atomically on both paths; a real INSERT accepts `1000000000` and rejects `1000000001` with CHECK 23514.
- Fixture limit: modern untracked Begin enforces max-per-order 999. This test explicitly seeds a **synthetic historical** quantity on a zero-price untracked line, after real Begin, with original quote and order snapshot kept equal and all monetary totals unchanged. Replica mode disables immutable-row triggers for that disclosed fixture only; the actual INSERT CHECK remains enabled. This does not claim a present-day checkout can buy a billion units.

## Current evidence

Exact commands, exit codes and log/source hashes: `qty-bound/results.json`, `qty-bound/source-hashes.json`.

| Command | Exit | Result |
| --- | --- | --- |
| `bash scripts/dev/test-focused.sh '^TestStripeUntrackedQuantityBoundary$' ./tests/foundation` on old SQL | 1 | RED: capture and close both PT409; `qty-bound/red.log` |
| Same focused command on fixed SQL | 0 | 1 PASS /0 FAIL /0 SKIP; `qty-bound/green.log` |
| `LC_FOCUSED_TIMEOUT=1800s bash scripts/dev/test-focused.sh '^TestStripe(SP\|SL\|RF\|Untracked)' ./tests/foundation` | 0 | **46 PASS /0 FAIL /2 SKIP**,623.159s; `qty-bound/stripe-regression.log` |
| `bash scripts/dev/test-node.sh` | 0 | 1148 PASS /0 FAIL; `qty-bound/node.log` |
| `pnpm --filter @live-commerce/admin typecheck` | 0 | `qty-bound/typecheck.log` |
| `bash scripts/dev/check-gates.sh` | 0 | 82 documented modes; `qty-bound/check-gates.log` |

**E3: tested REAL_PG + MOCK environment**, source7499dfc7. Plaintext RED evidence is uncompressed. K3/CI on the new head, SANDBOX credential probes, browser/full foundation and LIVE/production are NOT_RUN here. Commit only; integrator pushes. Return to LC-B2-DEL after this batch; W3-U2 stays awaiting K3. Bounded independent source/log review PASS with no new P0/P1/P2; no independent runtime rerun. Reverting only the two new predicates/comments restores the complete migration byte-for-byte to bbb8728d (`qty-bound/restore-proof.json`). Harnesses completed and removed their task-owned PG fixture; no active runner remains.

## Previous round-one evidence (historical bbb8728d)

- Branch/worktree: `unit/fix-untracked-stripe-capture`, `.worktrees/fix-untracked-stripe-capture`.
- Base: `66f9976122074c74138bdeae19c2b1dd0a61eeb8`; tested source: **`2b8089a9baa27e6d679018cea3e551c837bb2029`**. Final commit adds evidence only; `pr20-r1/source-hashes.json` binds current source and RF12 restore proof.
- Codex-4 / GPT-6 family; exact parent runtime model/effort not exposed. Read-only collaborator checked SQL proof/restore; no second writer. Coordination task `45f87a0d-d0a6-46d0-b6e1-c2f4148664c9`.
- Packet: `output/integrator/triage/pr20-66f99761.md`. Four real review threads fixed in this batch. No push.

## Changes

`migrations/0167_stripe_untracked_reservation.sql:160` now proves **every** unreserved quote SKU from the immutable original quote and that SKU's original RESERVE history, including mixed orders. Shape, scope, quantity mismatch, missing tracked reservations and extra reserved SKUs fail closed. A sibling tracked SKU's legitimate RESERVE does not invalidate an untracked line.

First capture checks current catalogue drift before the general review early return. A deleted/newly tracked unreserved SKU produces CAPTURED evidence, PAID_ALLOCATION_FAILED review and fulfillment state, and REVIEW_REQUIRED work; it never partially allocates tracked siblings or marks READY. PROVIDER_PRESENTMENT_DRIFT remains alongside allocation failure, so the existing refund guard permits only the full remaining amount.

Unpaid close uses original checkout evidence only; current SKU deletion/tracking no longer rolls back CLOSED_UNPAID. Its real capture worker returns success and query job terminates without another provider call. Later reports after an already successful capture do not reclassify the order: `v_new_capture` preserves stripe-refund-v1 §4.4. No contract change needed.

Detailed comment IDs, line anchors and §4.4 explanation: **`pr20-r1/REVIEW-NOTES.md`**. Migration0167 was edited in place as authorized; number/count, owner, SECURITY DEFINER, search_path, ACL and COMMENT are unchanged. No Go runtime/API/migration-count/dependency changes. RF12 restores all three explicitly permitted seams and compares the whole function byte-for-byte with0062; the original0061→0062 guard assertion remains.

## Gates and red→green

Exact commands, exit codes, counts and plaintext-log hashes: **`pr20-r1/results.json`**. All commands ran from this worktree on the frozen source above. Tests changed before their RED run; source implementation changed only afterward.

| Gate | Exit | Result |
| --- | ---: | --- |
| Four new REAL_PG review regressions before fix | 1 | 0 top-level PASS /4 FAIL; `red.log` |
| Untracked + RF12 + SP06 focused green | 0 | 11 PASS /0 FAIL /0 SKIP; `green.log` |
| Later post-capture catalogue/presentment control | 0 | 1 PASS, unchanged order/work state; `post-capture.log` |
| Foundation Stripe SP/SL/RF/Untracked | 0 | **45 PASS /0 FAIL /2 SKIP**,610.334s; `stripe-regression.log` |
| Stripe/payment/refund package gates | 0 | 37 top-level PASS /1 SKIP; `stripe-package-gates.log` |
| R2 release-head upgrade + KC03 + CRP02 | 0 | **3 PASS /0 FAIL /0 SKIP**; `upgrade.log` |
| `bash scripts/dev/test-node.sh` | 0 | 1148 PASS /0 FAIL; `node.log` |
| `pnpm --filter @live-commerce/admin typecheck` | 0 | `typecheck.log` |
| `bash scripts/dev/check-gates.sh` | 0 | 82 modes documented, all tracked tests assigned, headers green; `check-gates.log` |
| Source-only `git diff --check 66f99761..2b8089a9` | 0 | source whitespace clean |

The four new failure tests cover mixed tracking/deletion, durable fulfillment failure, unpaid all-untracked/mixed closure after tracking/deletion, and both reviews plus actual refund HTTP partial422/full201. A fifth control proves a later presentment report after ordinary capture preserves the settled aggregate even after catalogue drift. Existing normal all-untracked/mixed controls and ten original corruption refusals remain intact.

**Evidence: E3, tested REAL_PG + MOCK environment.** No real provider money/refund was sent. Independent source review found no confirmed P0/P1 money/data-integrity gap and independently restored the complete function to0062; that review is E1 and does not replace K3 deep review or runtime acceptance. New red logs are uncompressed. Historical root/k3-r1 results describe earlier source and are superseded by this `pr20-r1/` package.

## NOT_RUN / integrator

- K3 deep review and PR CI on this batch: pending integrator. Commit only; integrator pushes.
- Existing sandbox skips: SL08 restricted-key probe, RF10 refund sandbox, SP16 package sandbox, optional registrar probe. No owner-provided test credentials were loaded. Nested RF03 populated0061-upgrade remains the pre-existing NOT_RUN because the migrator has no partial-apply hook; the three requested upgrade gates pass separately.
- Full foundation/release G07, browser modes, provider SANDBOX/LIVE and production deployment: NOT_RUN. This batch changes SQL and Go tests only.
- Foundation govulncheck failure in the packet is KNOWN and fixed by PR19. This batch does not modify go.mod/go.sum or toolchain pins to duplicate that fix.
- Harnesses stopped and cleaned their owned PG/worker processes. Other agents' processes/caches untouched. No active implementation blocker.
- W3-U2 remains explicitly paused at checkpoint `d5f4fb3e`, tested source115bd202. Resume must merge PR22's new registry before any runner edit, then trunk after PR22 lands; no old if/elif edits.

Evidence packaging: the raw typecheck log ends with a blank EOF line; only that line is trimmed in the committed review copy for git whitespace checks. Raw uncompressed stdout and its hash remain at the canonical path recorded in results.json. All RED logs remain exact and uncompressed. Packaged check-gates rerun exits0.
