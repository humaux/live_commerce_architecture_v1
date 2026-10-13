<!-- Purpose: track CARD-PAY-FLAKE investigation, exact CI evidence and remaining acceptance. -->
<!-- Depends on: failed PR32/PR49 card-payments artifacts, the real Go/Next/PG browser harness and Gates workflow. -->
<!-- Used by: PR reviewers and the integrator; author record, not independent acceptance. -->
# CARD-PAY-FLAKE delivery — controlled fix verified, PR acceptance pending

## Charter and provenance

- Author: UI/integration, gpt-6.1-sol, ultra; branch `unit/card-pay-flake`, worktree `/Volumes/data/live_commerce_architecture_v1/.worktrees/card-pay-flake`, base `228a1c877f63b2be0955060d03ed621a05f4bdcf`.
- Read-only delegate `/root/card_harness_source`: explorer, gpt-6.1-sol/high, same base/worktree; no write paths, Git changes or runtime execution. Source trace only (E1).
- Write paths: `tests/foundation/browser_card_payments_test.go`, `tests/storefront/collector-buyer.mjs`, this delivery and scoped evidence under `output/card-pay-flake/`.
- Product code, payment assertions, browser deadlines and PostgreSQL limits remain unchanged. No vendor keys or production operations.
- Before the diagnostic commit: `git fetch origin` exits 0; own remote ref is absent (`git show-ref --verify refs/remotes/origin/unit/card-pay-flake` exits 128). `git merge --no-edit origin/r3/integration` exits 0, fast-forwarding to `256e9740c` (PR51 documentation only).

## Historical failures and downloads

| Evidence | Source | Observation |
| --- | --- | --- |
| [PR32 collector log](evidence/pr32/collector-primary.log), [Next log](evidence/pr32/next-primary.log) | [run 38396909805](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38396909805), job 115298159245, head `eb5f7e002ae3e337acbdad3ed9aebb2a7bef41e9`, artifact 11814201567 | Primary order is listed, but detail is absent; generic history error and 5,000 ms assertion failure. Next starts successfully. |
| [PR49 collector log](evidence/pr49/collector-primary.log), [Next log](evidence/pr49/next-primary.log) | [run 38442137263 attempt 1](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38442137263/attempts/1), job 115451390406, head `c26697a1425359c8826e14a3f78ea95e1a09b442`, artifact 11836252831 | Primary order row is absent; click times out after 30,000 ms. Next starts successfully. |

Both `gh run download <run> --name gate-browser-card-payments --dir output/card-pay-flake/raw/<case>` commands exit 0. PR32 downloaded the original failed evidence. PR49's same-named artifact was replaced by successful rerun artifact 11838724205 (digest `77ab3c99400f656e9266e5c9d5b1e5d656e3ef91d21042e4b211ae24d6a5ed3a`); its original failed logs above were preserved from the earlier PR49 investigation. Raw downloads stay untracked scratch; the four cited logs are portable repository evidence. Their headers record original byte hashes.

## What is proven, and what is not

- Existing artifacts do not record HTTP statuses or database errors at the failed read. They do not prove a startup, seed, cancellation, parsing or payment defect.
- The isolated PG has `max_connections=60` (`payment_runtime_test.go`, `pwIsolatedFixture`). Five payment fixture families retain six pools apiece, plus seven shared/admin pools; platform pools each allow eight connections. Pools grow lazily: 37 pool objects are not 37×8 observed connections.
- A successful order GET is followed by a separate session GET (`purchase.ts`, `readPurchase`). Either read failing can produce the same history error. This is a discriminator, not a root-cause conclusion.
- The historical diagnostic commits log only route classes/status/elapsed/capacity and canceled-acquire counts, never tokens, headers, identifiers, query strings or response bodies. Their one owner-pool observer is included in measured capacity. The final source removes that observer and the per-request diagnostics.
- The historical controlled runs set `LC_CARD_PAY_CONCURRENT=1` for eight independent primary buyer contexts, one attempt each. The final source removes this workflow opt-in: the normal card gate now always runs both the original single-context case and the eight-context case. Original assertions, deadlines and serialized screenshot manifest appends remain intact; no retries.

## Reproduced failure and candidate lifetime fix

- [Diagnostic collector](evidence/diagnostic/collector-primary.log), [capacity receipt](evidence/diagnostic/capacity.log), [Next startup](evidence/diagnostic/next-primary.log): exact diagnostic SHA `aa9177af162f96dc979d2c25b2afc1be2c0b8898`, run 38460385850, job 115513839541, artifact 11845358883 (digest `59d861a0a89cb4db39064e364132239b8fdca308f6b182cbe7364c663471b495`). All eight primary contexts fail: seven cannot load the order list; one loads the list then cannot load detail. Admin, unpaid and paid phases pass; primary Next is ready in 93 ms.
- The failing list/detail requests return 503 in 13–41 ms while the observer sees 57–60 client connections, against a normal-role limit of 57 out of 60. Both canceled-acquire counters remain zero. This reproduces both historical order-history symptoms under concurrency. SQLSTATE was not captured; the historical runs still lack HTTP/PG telemetry, so their exact cause remains unproven.
- Candidate fix: after the admin assertions, close 24 fixture-only pool objects: each of A/B/C/P/X's identity, seed payment-worker and seed expiry pools; also B/C/X buyer runtime, issuer and checkout pools. Keep A/P's active buyer pools, the real payment worker, hosted service, ingress, registrar and shared pools. Pointer deduplication handles order-copy aliases; existing cleanup remains idempotent. No product code, max_connections, deadlines or assertions change. The same eight-context case must go green before claiming the reproduced mechanism is fixed.
- Read-only delegate reviewed the actual cleanup diff and downstream dependencies with no P0/P1/P2 findings (E1; Humaux research `6bf5cf18-6c3a-4cc4-9615-0478c0f8b418`).

## Controlled GREEN on the fix

- Exact fix SHA `9f1179f0464d8df6b1b6817778d8c135c6ad2ec3`, [Gates 38462736944](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38462736944), job 115521966196, identical `LC_CARD_PAY_CONCURRENT=1`: SUCCESS, 20:26:12–20:29:58 UTC. Artifact 11846133440 has digest `4aafc3d1ef497bead10d46035a1b72d23a6f5905834b90a3d8cad4fbb7c69a6c`.
- [Collector](evidence/controlled-green/collector-primary.log) records each buyer 0–7 receiving both order-list and order-detail 200; all session/payment reads are 200, and the original primary assertions finish PASS. [Gate receipt](evidence/controlled-green/gate.log) also passes admin, unpaid and paid phases. [Next receipt](evidence/controlled-green/next-primary.log) preserves startup evidence. Original byte hashes are in each file header; raw downloads are excluded.
- [Capacity receipt](evidence/controlled-green/capacity.log) records `released_setup_pools=24 before_buyer=true`. Primary samples range from 26 to 54 client backends, below the unchanged normal-role limit 57 and max 60; canceled-acquire counts remain zero. The same eight-context workload went from eight failures to success by ending unused fixture pool lifetimes. Samples do not prove the unsampled peak, and do not identify the exact cause of the historical uninstrumented runs.
- Read-only independent receipt audit (`6bfc417c-a19e-451c-897f-e046eda0fd41`) verifies eight independent contexts, each one attempt, eight successful screenshots and no hidden failures: list 8×200, detail 8×200, payment 16×200, session 112×200. All contexts use the same synthetic buyer capability; they are not eight distinct buyer identities. Both source files retain the original assertions and deadlines with no P0/P1/P2 source findings.
- This is E3 for the reproduced CI workload with MOCK Stripe and real Go/Next/PG, not current PR-head or production acceptance. The full independent review at `b63b8a4e41e01cb925e8b08a5a049c3bd16b4752` passes the root-cause and closure-safety review; it requires the round-1 cleanup and final normal-mode CI below.

## PR52 round-1 P2 batch

- P2-1: remove `cpbrResponse`, `cpbrRoute`, the capacity connection, mutex/wait group and `CARD_PAY_TRACE` sampler. `cpbrBuyerNode` directly serves the real handler and has one deferred server close. Fixture pool release remains unchanged.
- P2-2: remove `trace()` and diagnostic failure screenshots. Keep the original single primary call; add an unconditional `primary-concurrent` call with eight independent browser contexts. `scripts/dev/test-local.sh` already registers `--browser-card-payments`; its `lc_run()` runs `^TestBrowserCardPayments$`, so every normal invocation executes both cases without an extra workflow setting. Distinct phase logs and numbered success screenshots retain the eight outcomes. Every context runs the original assertions once; any rejection still fails the aggregate. PASS receipts are emitted after assertions and success capture, and overall PASS also requires cleanup to succeed.
- P2-3: normal `--browser-card-payments` on the final PR head remains pending until GitHub reports it green. The historical sampled margin is not a final-source capacity guarantee. A/P's unused worker/expiry pools are already closed before buyers (zero connections); reducing their `MaxConns` cannot widen buyer-phase headroom. No shared pool configuration is changed. Removing the diagnostic observer also removes its held connection, but no new peak is claimed without measurements.
- P2-4: no change requested. Existing idempotent pool cleanup and downstream active services are preserved. Other browser harnesses with this pattern remain outside this unit, for integrator follow-up.
- Read-only actual-draft review by `/root/card_harness_source`, explorer/gpt-6.1-sol/high: no P0/P1/P2 source findings (E1, Humaux `219947c5-9772-4c7f-b751-b5a83b8b2431`; closed-pool configuration analysis `cb867f4c-2c42-4358-9756-f7901c6e023b`). Compilation and final normal-mode runtime evidence are pending, not inferred from that review.

## CI gates

1. Diagnostic candidate `aa9177af162f96dc979d2c25b2afc1be2c0b8898`: `gh workflow run gates.yml --ref unit/card-pay-flake -f modes='["--browser-card-payments"]' -f extra_env='LC_CARD_PAY_CONCURRENT=1'` exits 0. [Run 38460385850](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38460385850) starts at 2026-10-13 20:08:02 UTC; exact head verified, plan succeeds. Gate job 115513839541 runs 20:08:21–20:12:21 UTC and FAILS in the primary order-history flow. Workflow completes FAILURE at 20:12:28 UTC. `gh run download` and job-log download exit 0. This is actual browser RED, not a setup failure.
2. Lifetime candidate `9f1179f0464d8df6b1b6817778d8c135c6ad2ec3`: fresh fetch/own merge, scoped commit and push each exit 0; controlled run 38462736944 dispatch exits 0 with the identical mode and `LC_CARD_PAY_CONCURRENT=1`. Created 20:25:48 UTC, exact head verified; gate job 115521966196 runs 20:26:12–20:29:58 UTC, SUCCESS. Workflow completes SUCCESS at 20:30:05 UTC. `gh run download` and job-log download exit 0; portable receipts are linked above.
3. PR52 first head `b63b8a4e41e01cb925e8b08a5a049c3bd16b4752`: automatic [Foundation 38465078742](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38465078742) succeeds; [Gates 38465078657](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38465078657) was still running when the owner requested this P2 batch. These are ancestor checks, not acceptance of the diagnostic-free final source.
4. Next: automatic required PR Gates/Foundation on the P2 commit, including NORMAL `--browser-card-payments` with its original single-context case and permanent eight-context regression. No manual duplicate is dispatched while PR checks are planned/running. Final SHA/run/job IDs must be read from GitHub before READY.

## NOT_RUN / pending

- All local tests, builds, Node/Go/pnpm/Playwright, servers and PG: NOT_RUN, owner CI-only directive.
- Final-source normal card gate and required PR checks: NOT_RUN/pending. The independent full review passes `b63b8a4e`; the post-cleanup source review is E1 only. Controlled eight-context GREEN is limited to the exact fix SHA above; no final completion claim.
- Stripe SANDBOX/LIVE, real buyer data, production deployment: NOT_RUN and outside this unit.
