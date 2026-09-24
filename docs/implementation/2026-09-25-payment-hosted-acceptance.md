# Order-bound hosted payment preparation and handoff

2026-09-25. Status: IN_PROGRESS_INTERNAL_GATES.
Production code `dac73ca`; first independent PG tests `8105e39`; actual commit
acknowledgement-loss test `7b29072`. Full regression and final test expansion pending.
Contract: [payment-hosted-v1](../../contracts/payment-hosted-v1.md).

## Implemented scope

- `BeginHosted` reuses the original payment-start transaction, immutable attempt,
  command receipt, inventory transition and River query job. It builds and saves
  one order-bound form in that same transaction. Failure rolls everything back.
- The amount, merchant account, credential version and trade reference come from
  the frozen order/attempt, never the buyer. The keyring alone decrypts historical
  credentials and constructs the existing PAYUNi wire request; no raw keys escape.
- Locale is bound into the request digest. Both Chinese locales map to provider
  `zh-tw`; English maps to `en`. Return/notify URLs are fixed server configuration.
- The local release window is at most 60 seconds from `attempt.created_at`, bounded
  by qualification expiry. Replay cannot regenerate the form or extend the window.
- `TakeHosted` marks the form handed out in a committed transaction before any
  return to its caller. Repeat calls return `ALREADY_ISSUED` without form bytes.
  This does not prove that the browser received it or that the provider was paid.
- Separate hosted LOGIN authority inherits only the checkout family. Provision
  hosted membership with `INHERIT TRUE, SET FALSE`. Generic checkout/buyer/merchant/
  worker logins cannot execute the material/save/take functions or select forms.
  Database row scope derives from the owned immutable attempt. The migration owner
  retains table ownership; the private writer gets only the handoff timestamp UPDATE.
- SQL authenticates outer fields and scope, not encrypted inner content. The
  privileged hosted Go service plus keyring is the signing trust boundary. No
  caller-facing API accepts raw form JSON. No second DB signing key or engine.
- No new third-party dependency, provider request, production migration, live
  charge or customer toggle. Existing generic checkout signatures/digests unchanged.

## Evidence so far

All PG runs use the existing disposable, owner-labelled PG18 runner, not a local
developer database. Cryptography and identities are synthetic; no card/customer
data or provider secrets are logged.

| Evidence | Outcome and limit |
| --- | --- |
| `/Volumes/data/output/hosted-sql-baseline-payment.log` | 33 original payment tests PASS, 50.376s; SQL compatibility only |
| `/Volumes/data/output/hosted-core-unit-race.log` | checkout/accounts/platform race and vet PASS at `dac73ca` |
| `/Volumes/data/output/hosted-pg-author-payment-2.log` | Independent author run: ten new hosted cases plus original payment cases PASS, 56.413s |
| `/Volumes/data/output/hosted-root-payment-1.log` | Root rerun: 44 top-level tests PASS, 52.369s, including real lost-COMMIT-ack case |

The commit-loss fixture consumes a real PostgreSQL `CommandComplete(COMMIT)` then
closes the transport without forwarding acknowledgement to pgx. The API returns
an error with zero form. A separate owner connection reads the committed handoff;
normal recovery returns ALREADY_ISSUED and no new payment/stock/job facts. The test
fails if it did not observe the real COMMIT response; a sent COMMIT is not enough.

HP01–06 are being expanded for the complete locale/late-wait matrix. HP07 full
Go/PG/race/vet remains pending; the subset is not substituted for that gate.

## Work ownership and independent review

Root integrator owns the contract, migration0025, dependency documentation,
commit-loss test, integration and independent reruns. Go author
`hosted_go_implementation` used gpt-6-sol/high, base `d6720aa`, worktree
`/Volumes/data/worktrees/commerce-hosted-go-20260925`; seven scoped Go paths,
commits `12571fb` / `3645071`, merged as `1602059` / `dac73ca`.
Test author `hosted_pg_gate` uses gpt-6-sol/high, same frozen base, worktree
`/Volumes/data/worktrees/commerce-hosted-pg-20260925`, only its two new test files
and evidence note. No child recursively delegates or modifies old test thresholds.
Read-only `hosted_contract_security_review` used gpt-6-sol/high and found no
evidenced P0/P1/P2 at `dac73ca` in the stated trust boundary. Its provisional table
ownership concern was withdrawn after inspecting the migration executor; actual
PG now verifies the table owner and writer's denied form UPDATE.
Decision Humaux `0035e8b4-824c-456e-a2e1-4d48baf21201` is linked to BeginHosted,
TakeHosted, BuildPaymentHosted and ValidateHostedPool in the code graph.

## Still not delivered by this increment

Buyer payment options/status projection, public HTTP/BFF and the explicit payment
button/return-page browser journey; production query/capture worker assembly;
notification delivery/ACK integration; real merchant qualification and authorized
provider SANDBOX/LIVE acceptance are all separate gates. Merchant payment enabling
remains closed. No automatic cancellation/retry for an expired or uncertain
handoff; reconcile or review the original attempt, never silently release stock
or generate a second payment. Global SaaS, CVS, identity recovery and deployment
gates remain open. No full-system or real-payment readiness claim.
