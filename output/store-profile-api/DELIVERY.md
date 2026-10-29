# STORE-PROFILE-API — CONT #287 identity-disabled guard, CI pending

## CONT #287 current handoff (2026-10-29)

Authority: integrator's CONT #287 message; base/head before repair
`027c75a0590f4eec90701db0d21943f36909ad9f`; merchant-password-auth-v1 §16
and merchant-settings-wizard-v1 STORE-PROFILE-API. Historical sections below are
prior receipts, not current acceptance or remaining-work claims.

- Root cause: `authConfig` is evaluated once at module import. Removing
  `COMMERCE_IDENTITY_ENABLED` afterwards leaves that snapshot enabled. The shared
  `liveSettingsProxy` therefore reaches `authenticatedStores` and `scopeRefusal`,
  which returns 403 for the test's unrelated live-only grants. No middleware,
  route wrapper or recent-auth guard produced this failure.
- Repair attempt 1/2: the shared proxy's first denial also requires the current
  identity switch to equal `1`, before session, Origin/CSRF, membership or upstream
  work. This hides all its leaves when identity is disabled; startup configuration
  must still be present. No fixture fallback or authorization grant was added.
- Regression RED (E3 failure evidence): Foundation **40818415986**, job
  **123264887369**, head above; unchanged `live-settings-bff.test.ts:654`
  assertion failed **403 !== 404**. Raw filtered CI receipt appended to `red.log`.
  Historical original-card tests-only RED remains recorded below.
- Static E1: `node --check apps/admin/lib/live-settings-proxy.ts` exit 0;
  `bash scripts/dev/depmap.sh` exit 0, no generated diff. No imports, route identity,
  browser spec, test, fixture, migration, pin, gate or shared contract changed.
- This continuation touches only the existing card helper and its evidence files.
  New outside-card paths: none. GREEN pending PR CI; author assessment E0.
- NOT_RUN: all local tests/servers/builds/PG/browser, provider SANDBOX/LIVE,
  production and independent review. No background process started. Integrator
  relays the workflow completion in a fresh session; exact WAIT_RUNs are in the
  final handoff and PR comment. No merge, auto-merge or deployment authorized.
- Humaux search/store/canvas/coord tools are not exposed in this session. No shared
  state write claimed; integrator must mirror this handoff under its lock protocol.

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

## Current rebase-pin relay (2026-10-29)

Authority: current integrator card #287 rebase-pin. Base is
`0b831887a6ece4381a2bc01f39a5a22b92a0f049` (#286); 0205–0208 are now on base.
The exact static R2 inventory is **138 base + unchanged assigned 0209 = 139**.
This supersedes earlier count/OPEN-state statements in this historical delivery.
Only R2 pin and generated ownership conflicted; the base ledger lines are retained,
and ownership was regenerated with the existing native generator (53 modes).
Dependency map regeneration produced no diff. Migration0209 content is unchanged.
No local tests, servers, PG or browser ran. Syntax/build checks are not runtime acceptance.

Known blocking raw CI at previous head `4bd7c4b8626a148b575ebf0df2cc6da90d494e08`:
- Foundation40768505402/job123099180175: profile.go lacks an explicit backend
  coverage classification. Fresh base still has no declaration. Correct registry
  changes live in global scripts/dev/pr-modes.mjs or scripts/dev/test-local.sh,
  outside this card's module. STOP: no registry/exemption edit or inference of
  BACKEND_ONLY from missing browser evidence. Integrator owns this blocker.
- Gates40768505365/job123102334069: five profile tests fail their shared header
  helper's exact no-store comparison; actual API header is private, no-store.
- Same Gates/job123102334627: profile fixture-identity refusal expected404, actual403.
  These historical failures remain unresolved; this push claims only merge/pin work.

GREEN pending new-head CI; author assessment E0; no completion or independent review.
Exact workflow WAIT_RUN identifiers will be posted in a PR comment after push.
Humaux is available this relay: problem stored under the acquired child-resource lock;
child canvas updated. Integrator relays CI completion; no background process remains.

## Rebase-pin follow-up — 2026-10-29

Current relay receipt: `rebase-pin-relay.md`; raw Foundation40811803504/
job123243599576 failure in `rebase-pin-relay-foundation.log`. Merged fresh
integration61951609f; unchanged0209 and exact base138+1 pin139. Native browser
ownership and depmap generation exit0; no depmap diff. Shared CI classification
remains outside scope, so STOP without gate/exemption/test changes. New exact-head
WAIT_RUNs go in the PR comment; GREEN PENDING, author E0. Humaux is unavailable
in this follow-up; no new store/canvas/coord action claimed. No local tests or servers.

## Relay5 — current-head CI diagnosis and card-local header assertions

Authority: card relay, contracts/merchant-settings-wizard-v1.md STORE-PROFILE-API
(private no-store), and source at 5f82635fd81dabb73a8fe8694a94193c0a28017e.
The identity-disabled BFF assertion is GREEN at Foundation40821040430 /
job123273320136, tested merge c214b634fdd3b9adfbfa53990e0aa66f89371fc8
(head5f82635f + base61951609f). No middleware or re-auth regression remains there.

Current RED is Gates40821040503 / job123274609828, TestStoreProfileRevocationDuringWriteWait:
store_profile_http_test.go:278 calls assertAdminError, which requires exact no-store.
PUT correctly routes through claimsRoute/studioRoute, which sets private, no-store.
This same helper mismatch also prevents PUT success receipts from being inspected.
Fix attempt1: use card-local exact PUT private,no-store header assertions, retaining
all status, envelope, server request-ID, nosniff, leak, CAS, revocation and atomic
rollback assertions. GET still requires exact no-store. No permissive alternatives,
response normalization, fixture changes, generic helper changes or production changes.
The error helper preserves every assertAdminError check; PUT success adds nosniff.

Static only: gofmt, go vet ./tests/foundation, git diff --check and native depmap
regeneration exit0; no dependency-map diff. No local tests or servers. E1 structural
checks; author assessment E0; GREEN PENDING new-head PR CI and independent review
NOT_RUN. No new Test name, browser spec/page/route, migration, pin or contract change.
Changed paths: tests/foundation/store_profile_http_test.go and output/store-profile-api/**;
new outside-card paths: none. Rollback reference: 5f82635f (ordinary revert only).

Humaux is available in relay5. Child-only canvas update under acquired resource
store-profile-api:relay5; no new task claim because this is integrator-delegated work.
Integrator owns CI completion wakeup via fresh relay, maximum10-minute queue read;
final workflow/job receipts are posted in the PR comment, without an evidence-only push.
