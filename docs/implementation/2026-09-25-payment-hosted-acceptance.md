# Order-bound hosted payment preparation and handoff

2026-09-25. Status: PASS_BOUNDED_INTERNAL_HOSTED_PROVIDER_MOCK.
Production code `dac73ca`; final tested tree `cce122d`. HP01–07 passed for the
internal order-bound preparation and one-shot handoff. This is not public buyer
payment, merchant qualification, a provider transaction or deployment acceptance.
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

## Final evidence

All PG runs use the existing disposable, owner-labelled PG18 runner, not a local
developer database. Cryptography and identities are synthetic; no card/customer
data or provider secrets are logged.

| Evidence | Outcome and limit |
| --- | --- |
| `/Volumes/data/output/hosted-sql-baseline-payment.log` | 33 original payment tests PASS, 50.376s; SQL compatibility only |
| `/Volumes/data/output/hosted-core-unit-race.log` | checkout/accounts/platform race and vet PASS at `dac73ca` |
| `/Volumes/data/output/hosted-pg-author-payment-2.log` | Independent author run: ten new hosted cases plus original payment cases PASS, 56.413s |
| `/Volumes/data/output/hosted-root-payment-1.log` | Root rerun: 44 top-level tests PASS, 52.369s, including real lost-COMMIT-ack case |
| `/Volumes/data/output/hosted-pg-author-payment-4.log` | Independent author: 16 hosted cases plus original payment cases PASS, 56.911s; exact blocked-write and trigger-entry evidence |
| `/Volumes/data/output/hosted-root-full-1.log` | First root full run at `07f10ca`: 404 top-level PASS, 0 FAIL/0 SKIP, foundation 167.037s; predates final causal-test strengthening |
| `/Volumes/data/output/hosted-root-full-final.log` | Final root full run at `cce122d`: 404 top-level PASS, 0 FAIL/0 SKIP; actual PG foundation 163.696s, Go race and `go vet ./...`, exit 0 |

Final command: `bash scripts/dev/test-local.sh`. SHA-256:
`508f4491c07f1cb668b9f96605e0181a3e017af307ed23d630daf1916e839be0`.
Author final SHA-256:
`4bd360af52dc3054dfe0d6021e0dd2807b07f7116a8356f73a0521da2438dc7c`.
The existing runner used a disposable owner-labelled PG18 container and removed
it after exit. Root verified `lc-foundation-test-75732` absent; unrelated Humaux
containers were left alone. Failed/earlier logs and isolated worktrees remain.

The commit-loss fixture consumes a real PostgreSQL `CommandComplete(COMMIT)` then
closes the transport without forwarding acknowledgement to pgx. The API returns
an error with zero form. A separate owner connection reads the committed handoff;
normal recovery returns ALREADY_ISSUED and no new payment/stock/job facts. The test
fails if it did not observe the real COMMIT response; a sent COMMIT is not enough.

| Gate | Verified outcome |
| --- | --- |
| HP01 | Preparation commits exactly one original attempt/query job/receipt/stock transition/page; key and storage failures roll everything back. |
| HP02 | Identical replay preserves receipt, form and deadline. Changed order/method/version/locale/config/profile conflicts; another key cannot create another attempt. Three locale mappings are independently decrypted. |
| HP03 | Concurrent takes release one form. Actual successful-COMMIT acknowledgement loss returns an error and zero form; authoritative recovery returns `ALREADY_ISSUED`, without another attempt or stock/job mutation. |
| HP04 | Exact dedicated LOGIN grants, no SET/writer escalation, owner/store/capability and forged-GUC denial, immutable form/deadline and one-way timestamp RLS; ordinary checkout cannot load/save/take. Malformed outer wire fields fail closed. |
| HP05 | Qualification, binding, method and credential drift block preparation/first take. Exact `pg_blocking_pids` proves lock waits before expiry/revocation; nontransactional sequence markers prove delayed INSERT/UPDATE was entered before expected `ErrConflict`. MOCK cannot qualify SANDBOX/LIVE. |
| HP06 | Independent AES-GCM verifier checks frozen merchant/trade/TWD amount, timestamp, language and callbacks. Financial facts/review and local deadline prevent first release. |
| HP07 | Final full Go/actual PG/race/vet passes; independent source and test-causality review found no evidenced unresolved P0/P1/P2 within the stated signer trust boundary. Existing payment, capture, history and grant tests were not weakened. |

Detailed author assertions and weaker/failed baselines are retained in
[the independent PG record](2026-09-25-hosted-pg-author.md). The local 60-second
deadline branch uses an explicitly owner-only time shift in its fixture; it is
not evidence of the provider's undocumented timestamp window.

Root also added callback roundtrip and raw Unicode/angle/quote-path regressions
in `cce122d`. They passed the unchanged production baseline: its existing
`url.Parse`/`RawPath` guard already rejects those paths. The diagnostic log
`/Volumes/data/output/hosted-config-roundtrip-red.log` is historically named for
an expected red stage, but actually passed (1.033s). No URL bug or RED/GREEN
production fix is claimed; only test coverage was added.

## Work ownership and independent review

Root integrator owns the contract, migration0025, dependency documentation,
commit-loss test, integration and independent reruns. Go author
`hosted_go_implementation` used gpt-6-sol/high, base `d6720aa`, worktree
`/Volumes/data/worktrees/commerce-hosted-go-20260925`; seven scoped Go paths,
commits `12571fb` / `3645071`, merged as `1602059` / `dac73ca`.
Test author `hosted_pg_gate` uses gpt-6-sol/high, same frozen base, worktree
`/Volumes/data/worktrees/commerce-hosted-pg-20260925`, only its two new test files
and evidence note. No child recursively delegates or modifies old test thresholds.
Author test commits `7d3107b` / `f0b284a` / `aa0bec5` were merged as `8105e39` /
`07f10ca` / `420ee17`; author note `6f336aa` as `e719cba`. The root separately
owns `hosted_payment_commit_test.go` (`7b29072`) and the callback regression.
Read-only `hosted_contract_security_review` used gpt-6-sol/high and found no
evidenced P0/P1/P2 at `dac73ca` in the stated trust boundary. Its provisional table
ownership concern was withdrawn after inspecting the migration executor; actual
PG now verifies the table owner and writer's denied form UPDATE.
The source review is Humaux `3986e0c9-aa95-4766-ac35-7b468c00a4a3`; author test
delivery is `821746d1-d66b-4ce9-a95f-409aab6bd673`. Separate final review
`24e02274-ab4e-43e1-81a9-4c1b282d0c15` checked `07f10ca..cce122d` and the final root
log; it did not author production code/tests
or claim a second independent provider transaction.
Decision Humaux `0035e8b4-824c-456e-a2e1-4d48baf21201` is linked to BeginHosted,
TakeHosted, BuildPaymentHosted and ValidateHostedPool in the code graph.
The four added test files were incrementally indexed (94 entities, 0 rejected);
lock/trigger evidence helpers link to the author record, and the real COMMIT-loss
test links to the frozen handoff decision. No frontend/design file changed in this
increment; browser payment acceptance remains NOT_RUN, not inferred from Go tests.

## Still not delivered by this increment

Buyer payment options/status projection, public HTTP/BFF and the explicit payment
button/return-page browser journey; production query/capture worker assembly;
notification delivery/ACK integration; real merchant qualification and authorized
provider SANDBOX/LIVE acceptance are all separate gates. Merchant payment enabling
remains closed. No automatic cancellation/retry for an expired or uncertain
handoff; reconcile or review the original attempt, never silently release stock
or generate a second payment. Global SaaS, CVS, identity recovery and deployment
gates remain open. No full-system or real-payment readiness claim.
