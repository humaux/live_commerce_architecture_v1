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

## CI gates

1. Diagnostic candidate: `gh workflow run gates.yml --ref unit/card-pay-flake -f modes='["--browser-card-payments"]' -f extra_env='LC_CARD_PAY_CONCURRENT=1'`. Run ID and verdict pending. A passing probe alone will not establish the historical root cause or final acceptance.
2. After a proven mechanism and minimal fix: same controlled case plus normal `--browser-card-payments`, required PR Gates/Foundation and independent review.

## NOT_RUN / pending

- All local tests, builds, Node/Go/pnpm/Playwright, servers and PG: NOT_RUN, owner CI-only directive.
- New diagnostic CI, deterministic RED and fixed GREEN: not yet run; no root-cause or completion claim.
- Stripe SANDBOX/LIVE, real buyer data, production deployment: NOT_RUN and outside this unit.
