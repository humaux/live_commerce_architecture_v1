# Private buyer session registration — 2026-09-25

Status: PASS_BOUNDED_PRIVATE_REGISTRATION. This is a private BFF prerequisite,
not public browser bootstrap or an automatically generated payment link.

## Provenance and implementation

- Contract frozen at `b8b305d`; independent draft preflight `76667454-e8e6-4637-9711-188b063f05c7`.
- Root migration and independent PG tests: `ff64e16`, `28a2159`, `d21802b`.
- Go/HTTP author settings_ui_impl, assigned gpt-6-sol/high, isolated worktree
  `/Volumes/data/live-commerce-buyer-http-20260925`, branch
  `commerce/buyer-registration-20260925`. Source `aa080e2` cherry-picked as
  `669e148`, which is the final source/test revision exercised below.
- Root owns integration and real-PG gates; independent reviewer owns source and
  evidence review, not test authorship. Author-scoped unit/race/vet reported PASS
  (memory `4680d54c-3bbd-49cf-9205-57ef675e302a`); root full regression is the
  release evidence, not an assumption based on that report.

`buyer.register_capability` reuses the existing hash uniqueness and issuer/scope
functions. Only its fixed SECURITY DEFINER function is new: no new table, column,
index, runtime grant or package dependency. Registration stores SHA256 only;
same store/token replays preserve the original owner/session/expiry. Concurrent
losers roll back their inserted owner before resolving the winner. Only the exact
token constraint/schema/table is handled; unrelated integrity failures propagate.

`RegisterForTrustedStore` takes a trusted server's existing random token with a
five-second bound. Private POST `/v1/buyer/session/bootstrap` requires the existing
BFF key, canonical Bearer, exact currently published origin and empty JSON object;
returns only authenticated/expiry. Old POST session still mints independent
identities and remains explicitly nonretryable. No automatic token replacement.

## Actual acceptance evidence

Logs: `/Volumes/data/output/live-commerce-buyer-registration-tests/`.

| Run | Actual result | SHA256 |
| --- | --- | --- |
| root-targeted-initial.log | session87534 exit0, 17 top-level PASS / 0 FAIL / 0 SKIP; foundation10.181s | `51104860a3a7c8cb1224db42501a6ddef7a7ff7259ec93103e471296f79b01ea` |
| root-full.log | session86075 actual exit0, 361 top-level PASS / 0 FAIL / 0 SKIP; foundation143.677s; race + vet | `32829d32adbffaec8f9fed35526bc509ec4ba5936a488f7cc471713adeda13a5` |

Commands: `GOFLAGS=-p=1 bash scripts/dev/test-local.sh --buyer-http` and
`GOFLAGS=-p=1 bash scripts/dev/test-local.sh`. Each starts an owned, disposable
real PostgreSQL fixture. Full mode runs `go test -race -count=1` and `go vet ./...`.
Targeted 17 = 12 previous HTTP tests plus five new registration tests, not 17 new
product features. No previous test expectation was relaxed.
Root observed the actual process exit, not just the post-vet PASS marker.
Postflight `docker ps -a --filter label=livecommerce.fixture` returned no fixtures.

The new gates exercise:

- Actual committed success followed by a hijacked/closed TCP connection with no
  response; same-token retry on another handler/pool resolves the same identity.
- Saved cart survives replay across instances; changing configured TTL does not
  refresh expiry; independent token and legacy issuance remain separate owners.
- Eight concurrent HTTP calls produce one owner/session/issued event. A second,
  deterministic test holds a winner uncommitted, observes the loser's transaction
  ID lock, then commits and proves exact one-owner/session/event delta.
- Stale REPEATABLE READ snapshot fails closed after a separate committed winner;
  no internal retry loop or orphan owner. A fixture-only, store-filtered unique
  index on owners proves unrelated SQLSTATE23505 propagates unchanged and is
  removed by its exact-name cleanup.
- Runtime/merchant/identity/checkout EXECUTE denial; issuer direct-table denial;
  SECURITY DEFINER owner, fixed search path, volatility and no PUBLIC EXECUTE;
  invalid SQL and noncanonical Go tokens deny without durable new facts.
- Cross-store/tenant, inactive owner/store/tenant, revoked/expired tokens and
  current domain rebind/unpublish deny. A replay's scope locks block revoke;
  subsequent replay after revoke commits fails. HTTP strictness, safe exact
  response and old issuance/nonretryable semantics remain intact.

## Review and boundaries

Final independent source/evidence review at `669e148`: PASS, no open P0/P1/P2,
Humaux `c06967f5-a087-47be-8067-ca068b13c456`. The reviewer independently inspected
source and root log hashes/counts, but did not independently rerun PG or unit tests.
Root indexed five Go files (98 entities) and linked
registration and its causal concurrency gate to the independent design rationale.
Dependency and private HTTP contracts name all changed call paths and upgrade tests.
Five documentation files have 57 local links, none missing. `check_packet.py`
passes structure only (not a SaaS product gate); `git diff --check` passes.

Still NOT_RUN: public session cookie delivery and initial multi-tab/reset/logout
coordination, CSRF/rate controls, buyer visual comps and three-language pages,
automatic merchant share-link output, trusted CVS selection, order-bound PSP
checkout creation, real sandbox payment, signed capture callbacks and full SaaS
deployment/recovery gates. Two-phase cookies alone do not solve first-visit races.
No customer live stream, production database, real provider settings or funds changed.
