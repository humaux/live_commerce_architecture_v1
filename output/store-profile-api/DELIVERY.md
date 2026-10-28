# STORE-PROFILE-API — RELAY 3 drift repair awaiting CI

Assigned implementer: Codex, dispatched by Claude integrator; no recursive delegation.
Exact runtime model identifier/reasoning setting are not exposed by this session.
Branch `unit/store-profile-api`; worktree `.worktrees/store-profile-api`; PR #287 into
`r3/integration`. Main checkout and all other worktrees remain read-only.

## Delivered behavior

1. Contract amendment in `contracts/merchant-settings-wizard-v1.md` freezes signed
   GET/PUT profile DTOs, required name/email, optional phone, expected version,
   idempotency and server-only scope. Backend and BFF implement that contract.
2. `internal/httpapi/profile.go` registers through the existing handler. Restricted
   migration **0209_store_profile.sql** keeps `control.stores.name` authoritative,
   adds contacts/version, and rechecks auth after a real row-lock wait. Existing
   command receipts, audits and the profile update commit in one transaction.
   Replayed commands return their original result and recheck current permission.
3. **#254 / OC-05 is merged** in base `313c2c36f4ac064975c1c7c170d710cb63fa457f`;
   the existing quickstart implementation/0205 is reused without modification.
4. The contract maps home/cvs_711/cvs_familymart/cvs_hilife/cvs_okmart to the selected
   store/market's unique existing current **MANUAL** service and its persisted
   `code`, `policy_method`, `policy_version` and pricing facts. Missing/ambiguous
   service or policy remains unavailable. No invented fee/tax or carrier enabling.

The R2 pin is **136**: fetched integration base has **135** files in the exact R2
set; this card adds 0209. #262 and #265 remain OPEN at the readback. Integrator must
sequence the merge after those cards and settlement 0208 and refresh the pin to
the actual merged set. This lane never merges or pushes integration.

## Evidence and remaining acceptance

- RELAY 3: implementation head `216d2979312f1a0f6e78654646562df40abfd223`
  failed Gates run **40765780013**, plan job **123096837291**, with five
  contract-drift findings before any Gates test job. Raw filtered receipt:
  `relay3-drift.log`. Root causes: methods outside Markdown code spans and
  shorthand PUT path were not explicit contract routes; destructured BFF store
  was outside the scanner's supported leaf grammar. Repair attempt **1/2**
  uses full method/path spans and the existing `p.store` forwarding pattern.
  No scanner, baseline, exemption, dispatcher, test or fixture was changed.
- Fetched integration again in RELAY 3: still `313c2c36f4ac064975c1c7c170d710cb63fa457f`;
  R2 pin remains 136. This relay changes no route identity, import, browser spec
  or ownership discovery input; no generated ownership refresh is needed.
- RELAY 3 static: `node --check apps/admin/app/api/stores/[store]/profile/route.ts`
  and `git diff --check` exit 0. Runtime tests and contract-drift execution are
  reserved for GitHub CI; E1 syntax only, author assessment E0.
- Causal RED: Foundation **40762550914 / 123078018073**, PR head
  `24cac517c734c0d09cd017f523c98d31738c8a24`, tested synthetic merge
  `e431a572b1a5b24eee6a5ffaef99b8714d2ad11f` into base `313c2c36f`.
  Four new Node assertions fail: profile positive GET (404 vs 200), absent-session
  route (404 vs 401), conflict propagation (404 vs 409), missing contract amendment.
  Raw filtered job receipt is in `red.log`; this proves absent BFF/contract behavior,
  not independent execution of the downstream PG tests (Node failure stops the job).
- The earlier **40760270986 / 123069991728** failure was browser-ownership drift,
  not candidate RED. Its one targeted reachability repair remains recorded.
- E1 static commands and individual exit codes: `static.log`; Go build/vet,
  Node syntax checks and architecture packet check. No local test or server ran.
- Existing RED assertions remain intact. Profile tests now explicitly grant the
  contract's profile permissions within those tests instead of inheriting unrelated
  live-only permissions; no global fixture or expected denial was weakened.
  Added negative read/write grant tests, malformed BFF commands, mismatched receipt
  rejection, real middleware admission and restricted SQL-writer capability checks.
- The owner-authorized static generators refreshed browser ownership (53 modes)
  and dependency map. The dependency map has no diff. No dispatcher pin, drift
  baseline, next-startup assertion, lockfile, OpenAPI or unassigned migration edited.
- GREEN **PENDING** until implementation-head PR CI. Independent review **NOT_RUN**.
  Runtime PostgreSQL, local tests/servers/browser, provider SANDBOX/LIVE, production
  and tsc are **NOT_RUN** (no installed worktree TypeScript dependencies).
  Author assessment is **E0**; no completion, production acceptance or PASS claim.

## Ownership, rollback and handoff

Card paths: owning contract; profile Go/handler; migration0209; exact admin profile
route plus existing `live-settings-model.ts`/`live-settings-proxy.ts` helpers;
foundation profile/R2 tests; existing BFF tests; `output/store-profile-api/**`.
Generated extra path: `output/ci-select-backend-browser/browser-ownership.json`
(explicitly authorized by card/RELAY1). The two BFF helper files are same-app scope
under the card's pre-approval; list them in the PR body for integrator review.

Rollback reference: pre-implementation head `24cac517`; no destructive reset,
force push, production migration, merge or deployment performed. A subsequent
authorized revert can remove the implementation without rewriting shared history.
Migration0209 has not been applied locally or to production.

Humaux tools are unavailable in this relay, so no store/coord/canvas mutation is
claimed. Integrator must mirror the receipt with the shared-state lock protocol.
The current PR handoff comment supplies the implementation head and Gates/Foundation
workflow run IDs after push. Integrator owns the completion wakeup via a fresh relay;
no background poller or other task process remains. Gates may test a synthetic merge;
read its checkout/head/base receipt before final acceptance.
