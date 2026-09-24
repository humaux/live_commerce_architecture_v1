# Buyer private HTTP — bounded local acceptance

Status: PASS_BOUNDED_PRIVATE_BUYER_HTTP. Date: 2026-09-25 Asia/Shanghai.
Final tested code: `7c69fec`; frozen contract: `f84727d`.

## Delivered boundary

[The frozen contract](../../contracts/buyer-http-v1.md) is implemented by
`internal/buyerhttp`, mounted by `cmd/api/buyer.go` only when explicitly enabled.
The existing buyer, domain resolver, storefront and checkout services retain
their SQL authority and transaction ownership. No new dependency or migration.

This is an internal loopback-only BFF-to-Go transport, NOT public browser
checkout. Each request has a separate BFF credential and exact published origin;
all routes except session creation also require the buyer capability. Cookie,
Origin, query and client-supplied tenant/store headers are denied. Resolved scope
is never taken from Host or forwarded headers. The outer mount intercepts cleaned
path aliases without redirecting them; the inner router rejects the original
noncanonical path. Disabled mode does not read buyer secrets or open buyer pools.

The routes cover session issue/read/revoke, cart read/update, quote create/read,
destination update/read, checkout and owned-order read. Concrete allowlisted
response types omit internal warehouse allocations, actor/evidence fields,
binding/job identifiers and credential data. Existing workflow IDs and version
inputs remain because the private BFF needs them. This does not supply catalog,
policy or service/allocation-version discovery.

Session issuance is not idempotent. All its HTTP errors are nonretryable and a
lost response must not be automatically retried. Startup validates three distinct
database authorities and closes every opened pool on partial failure. Revocation
and resource isolation remain database checks, not just HTTP validation.

## Actual gates

| Gate | Result |
| --- | --- |
| `GOFLAGS=-p=1 bash scripts/dev/test-local.sh --buyer-http` | actual exit0; 4 top-level PASS, 0 FAIL/SKIP; foundation6.794s |
| `GOFLAGS=-p=1 bash scripts/dev/test-local.sh` | actual exit0; 339 top-level PASS, 0 FAIL/SKIP; foundation182.550s; same race, count1, 240s envelope and final `go vet ./...` |
| Independent final-code unit race replay | buyerhttp1.430s, httperror1.304s, cmd/api1.576s; no concrete static findings remaining before final PG review |
| Final independent review | bounded PASS at `7c69fec`, no open P0/P1/P2; independently counted/hashed both accepted PG logs, did not rerun PG |
| Packet/format checks | `python3 scripts/check_packet.py` PASS_PACKET_STRUCTURE_ONLY; `bash -n scripts/dev/test-local.sh` and `git diff --check` exit0 |
| Owned fixture postflight | no containers returned by `docker ps -a --filter label=livecommerce.fixture` |

`-p=1` serializes Go package execution; it does not remove tests, disable race
checking, increase deadlines or weaken expected results.

Evidence directory: `/Volumes/data/output/live-commerce-buyer-http-tests/`.

- `targeted-delta-fix.log`: SHA256
  `990fdfaddf6ce1c2973d4a362012fa84d10c4cdb97a16f21d2521ca7f8744568`.
- `full-delta-fix.log`: SHA256
  `3c47806341133141697a6367eda68a28441bf183a485fa6c12728c605e69ede0`.
- Earlier RED logs remain: `targeted-initial.log`, `targeted-cleanup-fix.log`,
  `full-initial.log`. Full RED SHA256
  `b4ffef1b2acb069531d8e920ee30073154f5d1bfb8bca10601d3bc08dea8061c`.

Real HTTP requests against isolated real PG prove the fresh anonymous-owner
workflow, exact JSON projections, checkout replay producing no extra facts,
conflicting replay, foreign owner/store denial, merchant-token rejection,
domain remapping/unpublication, revocation/expiry, database failure sanitization
and nonretryable failed issuance. The assembly test invokes actual tagged
`cmd/api` tests with distinct roles: partial failures leave no connections and
the successful assembly first proves all three pools opened before closing them.

## Failures repaired, not hidden

1. Shared fixture stores made a publication insert collide after the first test.
   Tests now clean up their exact domain/publication rows; no UPSERT hides it.
2. Changing pgx parsed runtime parameters did not serialize application names into
   the original DSN. Test DSNs now use `net/url`, with a positive three-connection
   assertion before testing cleanup, so an empty query cannot pass vacuously.
3. One checkout fact is a global River-job count. An absolute value of one passed
   alone but failed after prior tests. `7c69fec` reuses the existing before/after
   pattern and requires exactly one new fact for all six counters, then no changes
   after replay/conflict. No product behavior or gate was relaxed.
4. Prior macOS executions stalled before Go initialization; one sampled process
   stayed in `_dyld_start`. Those runs failed their actual runner envelopes and
   were retained. The final serialized run completed normally. This is not a
   claim that OS security controls were disabled or their root cause repaired.

Input tests include null at any JSON depth, malformed/trailing/unknown fields,
64KiB limit, wrong methods/routes, duplicate or malformed credentials and scope
headers. Request-body timeout and context cancellation retain unavailable
classification. `ResponseController` unwrapping has a regression test; the
existing unwrap method was not missing and was not replaced.

## Ownership and maintenance

- DTO author: `buyer_entry_inventory`; author `013cc9f`, merged `2604d71`.
- HTTP author: `settings_ui_impl`; author `befde83`, merged `9e3d4af`.
- Root: shared validators/error helper `1d0dd03`, assembly/integration `fe45c26`.
- Test delta repair: `buyer_entry_inventory`, `7c69fec`; independently reviewed.
- Final independent reviewer: `published_resolver_review`; actual full-PG
  evidence reviewed separately from author unit results; Humaux
  `0d50e4f9-05fb-4f94-8d88-cd37d1715141`.
- Thirteen final Go files / 267 entities were incrementally indexed in the
  `live-commerce` code graph. The checkout delta test links to Humaux fix
  `d4fd210d-29ee-4c7b-8535-a52286ec89f0`.

Dependency rationale and upgrade gates: [dependencies](dependencies.md).
No UI code changed; prior browser runs are not relabeled as buyer-browser proof.

## Still required

Public BFF with HttpOnly cookies, CSRF/origin validation, scoped rate limits,
bootstrap concurrency and discovery; three-locale buyer visual design and browser
flows; trusted domain/publication management; production worker assembly; actual
PSP/fulfillment qualification and event reconciliation; remaining full-SaaS gates.
The separate Stripe plugin sandbox link is not a server-side Stripe integration
and did not exercise these buyer orders. No customer production service, stream,
order, payment or configuration was modified by this local acceptance.
