# Payment query and authenticated observations

2026-09-24. Status: PASS_BOUNDED_INTERNAL_QUERY_AND_AUTHENTICATED_REPORTS.
Code: `3029998`; full regression and independent final review passed.
Contract: [payment-query-v1](../../contracts/payment-query-v1.md).
This increment does not release payment forms or confirm captured funds.

## Implemented boundary

- 0017 permits only a leased buyer-query worker to load the attempt's exact
  credential version. Scope, account, binding target, profile, generation, token
  and database-clock lease are checked. Ordinary roles cannot read ciphertext;
  disabled bindings and rotated keys do not erase historical payment facts.
- The reader reuses AES-GCM AAD/keyring and exact minor-to-TWD conversion. Missing
  keys do not fall back. Failed materialization exposes only already authenticated
  age, allowing the frozen retry horizon to stop it.
- `payuni.NewQuery` has no dummy callbacks and cannot build a form or verify
  notifications. Its one-shot internal transport disables idle keepalives; hosted
  behavior is unchanged. MOCK workers require an explicit mock transport; real
  profiles reject it. Tests run actual wire authentication, not fake Observations.
- The River worker reuses Claim/Complete and the original job. Claim and metadata
  read commit before one query. Parent cancellation propagates. Report insert
  and UNKNOWN completion are atomic; duplicate reports deduplicate, and provider
  references cannot move between attempts of the same account.
- Failure/budget completion also checks profile, reference and clock after the
  final event write. Expired age/generation is persisted as
  UNKNOWN/payment_query_budget_exhausted before job cancellation. Duplicate jobs
  cannot implicitly reset that budget. No query result releases inventory.

## Why query success is not capture

Official inspection distinguished query Status=SUCCESS, payment TradeStatus=1,
card one-time type AuthType=1, capture CloseStatus=2 and merchant payout.
DataSource B is incomplete; this worker snoozes at least ten minutes. Full capture
also needs adequate CloseAmt, which the current ten-field projection does not
include. Therefore reports cannot confirm an order or commit its reservation.

Sources: [transaction query](https://docs.payuni.com.tw/web/#/7/164),
[card status diagram](https://docs.payuni.com.tw/web/#/7/153),
[payout query](https://docs.payuni.com.tw/web/#/7/219).
Humaux research: `4a543f3d-bf28-4d6a-a32b-f2744cbf3b51`.
No real account, sandbox/live transaction or customer data was used.

## Tests and retained failures

Logs: `/Volumes/data/output/live-commerce-payment-query-tests/`.

| Run | Result |
|---|---|
|schema-preflight.log|Initial 0017 plus prior nine start PG tests: PASS, 11.007s|
|payment-first.log|Root test compile failure: PlanResult.OperationID, not ID; test field corrected|
|payment-second.log|Wire failures reached expected outcomes; test incorrectly scanned nullable River errors into string. NULL now means no errors; leak assertion retained|
|payment-third.log|Red reproduction: expired age plus missing key. Second-order fixture also conflicted by reusing a cart without its version; changed to an independent buyer in the same store/account|
|payment-final.log|20 top-level PASS, 0 FAIL/SKIP, 50.537s: 11 new query plus nine start tests|
|full-final.log|Exit 1, 142.670s: only old T06 function ACL inventory failed after adding query entrypoints|
|full-corrected.log|Exit 0: 265 top-level PASS, 0 FAIL/SKIP, foundation135.051s plus vet|
|full-accepted.log|Exact final source, including checkout-writer ACL denial: exit 0, 265 top-level PASS, 0 FAIL/SKIP, foundation167.629s plus vet|

The age/key defect was in product flow: decryption failure discarded authenticated
age and delayed the 24-hour stop until generation exhaustion. The reader now
returns age-only failure metadata, which survives worker rollback and is checked
before the material error. SQL/token/profile errors still return zero material.
The real-PG red case is green in payment-final.log.

The old ACL test assumed two functions and worker EXECUTE on every integration
function. Its replacement enumerates six exact signatures, five worker APIs and
one private guard. Unknown functions/overloads, wrong owner/search_path, PUBLIC,
merchant, buyer or checkout runtime/writer execution fail. Product GRANTs were
not relaxed. The independent reviewer requested the final checkout-writer denial,
so the exact final source received a separate full run, not a cached earlier pass.

Final full log SHA256:
`d0ec2887e13b59f9aa13fb60de483e69d09ac2270e0d6af1672f74bf85c69a5c`.
Prior full RED SHA256:
`d25a32314f8df30c882c3ea9d550e9bf082c330a42f32e53f71720f6ea0e9137`.
Final query subset SHA256:
`f8ba62776934451e8f4757acfe8276dbaf5dfbeca4c64f9eeac13a2280621b1f`.
The task-owned fixture container listing was empty after completion. Failed logs
are retained; no developer database or shared cache was removed.

Independent pre-merge review also corrected author draft minor-unit conversion
and duplicate Complete after SQL record had already cleared the lease. Final code
converts once and completes once. An initial reviewer Claim warning was withdrawn
after checking the 0016 replacement; it was not a product defect.

New PG cases cover signed historical-v2 requests after v3 rotation/disable, ACLs,
bad profile/token/amount/merchant/signature, missing key/AAD, timeout/panic,
reference uniqueness, report dedup/concurrent single winner, post-write lease
waits, durable age/generation budgets and cancelled I/O. Payment/stock remain
pending. These are local/mock gates, not real payment acceptance.

## Work division and limits

Root owns contract, 0017 SQL and independent PG tests. Author payment_query_impl
(gpt-6-sol/high, commerce_worker) used isolated worktree
`/Volumes/data/live-commerce-worktrees/payment-query-20260924`, branch
codex/payment-query-20260924, base7d24544, commits7822367 and95edd7c. Six Go paths;
package race/vet passed. Root independently reran the real-PG gates and committed
the combined Go/SQL/tests as `3029998` only after final independent review.
Reviewer payment_query_review (gpt-6-sol/high) is read-only, independently ran
three-package race/vet and verified the exact full-PG log/hash. Verdict: no
unresolved P0/P1 within this query-only scope, Humaux
`a90c8824-8680-4e50-91b2-9a91b1ce5d72`. Research roles were read-only and no agent
recursively delegated. Root fix record: `3fc6820a-21c5-4985-8992-5a3824fdd25c`;
worker and budget test entities are linked to it. Nine files submitted to code
index; eight Go files / 241 entities processed. SQL is retained in the migration.

Documentation checks: packet structure PASS (not a product gate); eight touched
Markdown documents, 49 local links, zero missing targets; `git diff --check` clean.
README, task DAG, service-settings coverage and dependency/upgrade notes point to
this increment without upgrading the global production gates.

No new dependency, actual PSP call, production migration, customer toggle or UI
change. Browser payment acceptance is NOT_RUN. Next: capture/refund field
projection, financial facts and notification deduplication, atomic allocation
and fulfillment intent, real qualification, hosted forms, public checkout and
three-language settings. Full SaaS and global production gates remain open.
