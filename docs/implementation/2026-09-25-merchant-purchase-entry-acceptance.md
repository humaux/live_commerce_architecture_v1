# Merchant purchase-entry backend acceptance

Status: ACCEPTED — BACKEND CONFIGURATION PROJECTION ONLY, 2026-09-25.
Source: `621e7d4`, `70ebeb2`, `e8cb64b`, `c26cac9`; frozen contract `07477f7`.
This is the authenticated backend projection, **not** automatic collection,
a deployed buyer page, DNS/TLS proof, a PSP adapter or PE01 end-to-end acceptance.

## Delivered source and authority

- A strict GET purchase-entry endpoint under existing merchant catalog:read.
  Exactly four response keys: product_id, locale, state, url. Three explicit
  locales; no browser-supplied hostname or amount in the link.
- Migration0023 gives only required publication/domain columns to the existing
  non-login auth owner. The VOLATILE SECURITY DEFINER reader checks the token,
  scope GUCs and READ COMMITTED before one bounded eligibility SELECT.
- Zero domains is unavailable; two eligible domains require explicit selection.
  A sole candidate is rechecked against the final database clock. Expiration
  during the query cannot turn two captured candidates into a guessed primary.
- Product and SKU writes and their original command receipts are unchanged.
  URL projection does not create buyer capabilities, orders, stock holds or PSP
  calls; prices remain catalog authority, not embedded link authority.
- The approved B inline buyer composition is recorded separately at `f17d8b3`
  in the [surface brief](2026-09-25-buyer-surface-proposal.md). Approval does not
  assert its implementation or browser acceptance. Its asset audit found no
  required shipping raster: text and controls remain semantic code.

## Actual validation

Commands run from the repository with GOFLAGS=-p=1. The local runner creates a
named, ownership-labeled disposable PostgreSQL18 container with a random secret
and removes only that fixture at exit. No customer database, PSP or live system
is used. Domain proofs, products and all pricing in tests are synthetic.

| Gate | Evidence / current result |
| --- | --- |
| MPE01 | focused-4.log PASS: own product and current-currency active SKU yield exact zh-CN/zh-TW/en URLs; zero price remains data; price update preserves URL; product/SKU save replay preserves byte-identical original receipts. |
| MPE02 | PASS: unknown/foreign/archived/no-SKU/wrong-currency, inactive tenant/store, all non-ACTIVE domain states, future/expired proofs, publication change and multiple domains. Four raw domains include two ineligible leading rows. |
| MPE02 temporal | PASS: two separate real relation-lock cases observe pg_stat_activity waiting, wait for proof expiry on the database clock and release the lock. Sole candidate is unavailable; two candidates remain ambiguous. |
| MPE03 | PASS: owner/volatility/security-definer, FORCE RLS, application-role ACL matrix and no auth-owner writes; bad hash/token/store, absent/malformed/individually forged GUCs, expired/wrong-audience session and non-RC rejection. |
| MPE04 | PASS: real PG-backed admin HTTP, strict query/body/key/method boundaries, exact DTO, no-store and attacker Host ignored. After WithScope succeeds, revoke session/catalog permission/store grant: Go returns unauthorized/forbidden/not-found, and transport classification tests assert401/403/404. Real non-RC maps to unavailable; transport unit gate asserts503. |
| MPE05 | PASS: fourteen purchase/session/inventory/provider/command fact counts unchanged by GET and original save replay; later price edit keeps original SKU receipt and purchase URL. |
| MPE06 | PASS: full real-PG/race/vet exit0:375 top-level PASS,0FAIL,0SKIP; independent source/evidence review has no remaining P0/P1/P2. Dependency map, source index and Humaux records updated. |

Focused real PostgreSQL/HTTP command:
`bash scripts/dev/test-local.sh --purchase-entry` — **exit0**, four top-level
tests plus eight named subtests in `focused-4.log`. This is not twelve separate
top-level business gates; the cases also exercise multiple assertions in loops.

Evidence directory: `/Volumes/data/output/live-commerce-purchase-entry-tests/`.
The first attempt failed compilation because the test helper returns int, not
int64. The second exposed a test-helper error: it sent an empty Idempotency-Key
header on every GET, correctly denied by the endpoint. The helper now omits that
header when absent. Both original logs are preserved, not counted as passes.
Focused runs3/4 pass. The first full run (`full.log`) failed one existing
domain ACL assertion that still required commerce_auth to have zero column
reads. Migration0023 deliberately grants that non-login owner limited reads.
`c26cac9` replaces the stale assertion with an exact per-column SELECT whitelist
and retains no table-level SELECT, no writes, no application direct access and
no auth execution of the buyer resolver. It also asserts the new function's
fixed search_path, non-login/non-superuser/non-bypass owner and no PUBLIC execute.
The repeated complete command is recorded in `full-2.log`; no failing run is
counted as a pass.

Final complete command `GOFLAGS=-p=1 bash scripts/dev/test-local.sh` returned
**exit0**:375 top-level tests passed, no failures or skips. The foundation package
took148.643s under the race detector; all packages and subsequent `go vet ./...`
passed. At exit, the fixture-label Docker query was empty. No customer resource
was stopped or deleted. `git diff --check`,62 relative document-link checks and
the three-sidecar check (only B approved) also passed.

## Independent correction and maintenance

Independent review found the first Go projection guard accepted localhost, IP,
trailing-dot and underscore hostnames, unlike the database's canonical grammar.
Although SQL constraints blocked normal rows, accepting a malformed privileged
projection violated the explicit contract. `e8cb64b` removes the second weaker
parser and reuses the unchanged existing grammar as `domains.ValidOrigin`, with
all five reported bad-host regression cases. Reviewer independently reran the
catalog, httpapi and domains unit packages successfully. Final independent
review at `c26cac9` reports no open P0/P1/P2 and confirms the revised column
whitelist did not weaken the gate. Review memory:
`0555be35-d8ae-4f7a-b5f7-26d2e7949dcc`, title
`Merchant purchase entry c26cac9 final independent source and evidence PASS`.
Full real-PG/race/vet was run by root and its evidence read by the reviewer;
the reviewer did not claim a second complete PG run.

No new package dependency. The [dependency map](dependencies.md) documents
HTTP → scoped catalog → authenticated SQL and the reused canonical grammar.
Incremental Humaux code indexing processed eight files /123 entities from nine
submitted paths, followed by the ACL correction's two files /38 entities;
ReadPurchaseEntry and the observed-lock regression are linked
to the independently frozen contract memory8737f904-f447-424e-b0fd-bf0568db1e9c.

## Remaining release boundaries

- No merchant copy/open control is released before the actual B buyer product
  route and its real browser gate. Backend configured is not network readiness.
- Public page must resolve published origin and active product on each visit;
  no cross-merchant fallback, fictitious price, image or stock claim.
- Cart/destination/Quote/order-specific hosted checkout, provider expiry versus
  stock lifetime, signed payment evidence and real sandbox acceptance are still
  distinct gates. A Stripe plugin link does not qualify an application adapter.
- Customer production data, live broadcasts, existing logistics and payments
  remain untouched. Whole-SaaS release is not claimed by this bounded unit.
