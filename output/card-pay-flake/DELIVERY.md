<!-- Purpose: track CARD-PAY-FLAKE investigation, exact CI evidence and remaining acceptance. -->
<!-- Depends on: failed PR32/PR49 card-payments artifacts, the real Go/Next/PG browser harness and Gates workflow. -->
<!-- Used by: PR reviewers and the integrator; author record, not independent acceptance. -->
# CARD-PAY-FLAKE delivery — investigation in progress

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
- Diagnostic changes log only route classes/status/elapsed/capacity and canceled-acquire counts, never tokens, headers, identifiers, query strings or response bodies. One owner-pool connection observes capacity outside the HTTP response path; it is held until the phase drains, and its contribution is included in the measured capacity.
- `LC_CARD_PAY_CONCURRENT=1` enables eight independent primary buyer contexts, one attempt each, preserving every original disclosure/absence assertion and timeout. No retries. Screenshot manifest appends are serialized.

## Reproduced failure and candidate lifetime fix

- [Diagnostic collector](evidence/diagnostic/collector-primary.log), [capacity receipt](evidence/diagnostic/capacity.log), [Next startup](evidence/diagnostic/next-primary.log): exact diagnostic SHA `aa9177af162f96dc979d2c25b2afc1be2c0b8898`, run 38460385850, job 115513839541, artifact 11845358883 (digest `59d861a0a89cb4db39064e364132239b8fdca308f6b182cbe7364c663471b495`). All eight primary contexts fail: seven cannot load the order list; one loads the list then cannot load detail. Admin, unpaid and paid phases pass; primary Next is ready in 93 ms.
- The failing list/detail requests return 503 in 13–41 ms while the observer sees 57–60 client connections, against a normal-role limit of 57 out of 60. Both canceled-acquire counters remain zero. This reproduces both historical order-history symptoms under concurrency. SQLSTATE was not captured; the historical runs still lack HTTP/PG telemetry, so their exact cause remains unproven.
- Candidate fix: after the admin assertions, close 24 fixture-only pool objects: each of A/B/C/P/X's identity, seed payment-worker and seed expiry pools; also B/C/X buyer runtime, issuer and checkout pools. Keep A/P's active buyer pools, the real payment worker, hosted service, ingress, registrar and shared pools. Pointer deduplication handles order-copy aliases; existing cleanup remains idempotent. No product code, max_connections, deadlines or assertions change. The same eight-context case must go green before claiming the reproduced mechanism is fixed.
- Read-only delegate reviewed the actual cleanup diff and downstream dependencies with no P0/P1/P2 findings (E1; Humaux research `6bf5cf18-6c3a-4cc4-9615-0478c0f8b418`). Runtime GREEN remains pending.

## CI gates

1. Diagnostic candidate `aa9177af162f96dc979d2c25b2afc1be2c0b8898`: `gh workflow run gates.yml --ref unit/card-pay-flake -f modes='["--browser-card-payments"]' -f extra_env='LC_CARD_PAY_CONCURRENT=1'` exits 0. [Run 38460385850](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38460385850) starts at 2026-10-13 20:08:02 UTC; exact head verified, plan succeeds. Gate job 115513839541 runs 20:08:21–20:12:21 UTC and FAILS in the primary order-history flow. Workflow completes FAILURE at 20:12:28 UTC. `gh run download` and job-log download exit 0. This is actual browser RED, not a setup failure.
2. After a proven mechanism and minimal fix: same controlled case plus normal `--browser-card-payments`, required PR Gates/Foundation and independent review.

## NOT_RUN / pending

- All local tests, builds, Node/Go/pnpm/Playwright, servers and PG: NOT_RUN, owner CI-only directive.
- Fixed eight-context GREEN, normal card gate, required PR checks and independent final acceptance: NOT_RUN/pending. No completion claim.
- Stripe SANDBOX/LIVE, real buyer data, production deployment: NOT_RUN and outside this unit.
