# Published storefront resolver v1

Status: PASS_BOUNDED_INTERNAL_PUBLISHED_ORIGIN_RESOLVER, code `cbffd8a`.
See [317-test acceptance](../docs/implementation/2026-09-25-published-storefront-resolver-acceptance.md).
Local prerequisite only, not public checkout or domain provisioning.
Writer (R3, migration 0081, unit storefront-publish): IMPLEMENTED, evidence MOCK (Go unit tests) + NOT_RUN (real-PG gates are
the independent test phase's); see "Writer (R3)" at the end. The "no mutation API ships here" statements above describe the
resolver slice only.
Baseline: `1baa0d2`. Contributes to T03; full G01/G02/G11 remain NOT_RUN.

## Decision and scope

`control.stores.active` is operational eligibility, not consent to publish a
website. No existing store is implicitly published. Introduce one small DB
resolver before the public buyer transport, reusing the checked buyer-issuer
authority and its existing non-login SECURITY DEFINER owner. No second identity
service, DNS client, cache, production domain registration or HTTP route here.

The resolver consumes **already provisioned** control-plane facts. This slice
does not implement their writer or prove DNS/TLS ownership. Local tests seed
synthetic facts as the isolated migration owner; that is not a provider proof.
Production remains disabled until an audited ownership/TLS/publication writer
and the separate cookie/CSRF/rate-limit/HTTP/browser gates are implemented.

## Durable facts (forward migration 0020)

- `control.storefront_publications`: `(tenant_id,store_id)` composite primary
  key/FK to `control.stores`, `published boolean NOT NULL DEFAULT false`,
  `version bigint > 0 DEFAULT 1`. No backfill from active stores.
- `control.storefront_domains`: generated UUID `id` primary key; tenant/store
  composite FK; globally unique canonical `origin`; `version bigint > 0 DEFAULT
  1`; `state` in REQUESTED, OWNERSHIP_PENDING, TLS_PENDING, ACTIVE, SUSPENDED,
  DETACHED (default REQUESTED); optional ownership_verified_at, tls_verified_at,
  valid_until and evidence_ref. ACTIVE requires all three timestamps and a
  evidence_ref of at most 240 characters containing a non-whitespace character
  (PG POSIX space class) and no control characters, all supplied timestamps finite,
  with valid_until later than
  both verification timestamps. Check origin shape/length in both Go and SQL.
- Both tables FORCE RLS. Ordinary runtime, merchant identity, buyer runtime,
  buyer issuer, checkout and worker logins receive no direct table privileges.
  Existing `commerce_buyer_writer` receives SELECT only via an explicit read
  policy. No new membership or public schema access. No credentials or buyer PII
  in either table or returned tuple.
- Rows describe current control-plane facts only; this is NOT an implemented
  domain lifecycle transition/audit writer. Future changes must be authorized,
  versioned, audited, and invalidate resolution; DETACHED cannot be rebound by a
  mere update without renewed ownership proof. No mutation API ships here.

## Exact lookup and Go contract

- Origin is `https://` followed by an ASCII lowercase DNS hostname, at most 253
  hostname characters, at least two labels, labels 1..63 characters; letters,
  digits and interior hyphens only; final label begins with a letter. No IP,
  localhost or `*.localhost`, wildcard, path (including `/`), port (including `:443`), userinfo,
  query, fragment, trailing dot, Unicode or whitespace. Already encoded `xn--`
  DNS labels are allowed; no implicit normalization or DNS lookup. A future
  configuration form may normalize deliberately before this trust boundary.
- `buyer.resolve_published_store(p_origin text)` returns zero or one row:
  `(domain_id uuid, store_id uuid, domain_version bigint,
  publication_version bigint, origin text)`. Fixed SECURITY DEFINER owner
  `commerce_buyer_writer`, `search_path=pg_catalog`, fully qualified references,
  PUBLIC EXECUTE revoked; EXECUTE granted only to `commerce_buyer_issuer`.
  Invalid syntax raises sanitized PT400. Valid but unknown/ineligible yields no
  rows. It must require active tenant AND active store, published=true, exact
  origin, ACTIVE domain, both verification times <= final DB clock and
  valid_until > that clock. SQL independently checks input syntax.
- `platform.ValidateBuyerIssuerPool(ctx,pool) error` reuses the existing startup
  authority check without closing a caller-owned pool. Nil context/pool fail.
- `internal/domains.New(ctx,issuerPool) (*Resolver,error)` validates that exact
  authority; caller retains pool ownership. `(*Resolver).Resolve(ctx,origin)
  (Route,error)` uses a bounded five-second DB request, validates input before
  querying, calls only the fixed function, and checks the returned shape/exact
  origin. `Route` fields: DomainID, StoreID, DomainVersion, PublicationVersion,
  Origin. `domains.ErrInvalid`, `domains.ErrUnavailable` (unknown/ineligible),
  and `domains.ErrDatabase` are stable; preserve context cancellation/deadline,
  never return driver/SQL/DSN details. Nil resolver/context fail closed.

Resolution is an admission snapshot, not a lock or a permanent permission.
There is no cache. After suspension/unpublish/expiry commits, a new resolve must
deny; a request admitted before the commit can finish within its bounded
deadline. Future transport resolves on **every** request, derives store only
from that result, then separately authenticates its buyer capability for that
store. A route does not authorize a buyer, a cart or an order. Rebinding cannot
make an old-store capability valid for a new store. No fallback/default store.
Do not accept a caller's store ID or arbitrary forwarded host as a substitute.

## Acceptance gate for this prerequisite

| Gate | Required evidence |
| --- | --- |
| PR01 | Go and direct SQL origin grammar parity, positive HTTPS domain and negatives including suffix tricks, ports, wildcard, Unicode, whitespace and control bytes |
| PR02 | Real isolated PG: active store without publication/domain denies; only exact published+verified ACTIVE mapping resolves; independent tenant/store/publication/domain denial and future/expired proof times |
| PR03 | Every non-issuer application role denied EXECUTE; issuer has no direct table read/write; owner/mixed-role constructor rejected; qualified tenant/store FK and global-origin uniqueness enforced |
| PR04 | Same resolver sees committed suspension, unpublish and renewal without restart; wrong-store capabilities still denied; no buyer/session/member issuance from lookup |
| PR05 | Context/error sanitization, closed-pool failure, no tenant/proof/PII fields in returned tuple; existing role checks and full real-PG/race/vet regression remain green |
| PR06 | Independent P0/P1 review, dependency/acceptance record, code index and Humaux decision/canvas updates |

Not covered: verified domain issuance, lifecycle write API/UI, rate limiting,
public session issuance/CSRF/cookies, catalogue browsing, cart/quote/destination/
checkout HTTP or three-locale buyer UI, real payments/shipping. The public order
response must later explicitly project buyer fields: never serialize raw
`checkout.Order` (job/reservation/allocation/warehouse/binding internals).

## Why not alternatives / upgrade signal

- Active-store-only or arbitrary request store ID: would publish without consent
  and bypass domain binding; rejected.
- Process-local host map: cannot observe committed suspension consistently across
  instances; use the existing PG truth, not a second routing cache.
- Full DNS/CDN automation now: needs actual provider credentials and controlled
  ownership verification. Add the separately reviewed writer/adapter next;
  these unpopulated tables cannot confer real provider readiness by themselves.

## Writer (R3)

Unit `storefront-publish`, migration `0081_storefront_publish.sql`. The two facts the resolver consumes now have an audited
production writer in two separate consents (rulings SP1-SP7):

- **Publication is the merchant's act.** `control.read_storefront(p_hash,p_store)` (integration:read) and
  `control.set_storefront_published(p_hash,p_store,p_published,p_expected)` (integration:manage); SECURITY DEFINER, owner
  `commerce_storefront_writer`, `search_path=pg_catalog`, PUBLIC revoked, EXECUTE `commerce_runtime`. They verify the bearer
  through `identity.resolve_access` and the WithScope GUCs (0078 pattern), compare-and-set on `version` (0 = no row; stale =
  PT409 `version_conflict`), bump `version` and write one `ops.audit_events` row (`merchant.storefront_published` /
  `merchant.storefront_unpublished`) per real change. Re-asserting the current state is a no-op. HTTP:
  `GET /v1/admin/stores/{store_id}/storefront`, `POST .../storefront/publication {published, expected_version}`
  (internal/httpapi/storefront.go; BFF allowlist GET `storefront`, POST `storefront/publication`; admin Settings card).
  Publishing alone serves nothing: the resolver also needs an ACTIVE domain.
- **Domain binding is the platform operator's act.** `control.operator_bind_domain(store,origin,evidence,valid_until)`,
  `operator_suspend_domain(origin)`, `operator_detach_domain(origin)`, `operator_storefront_status(store)`; same owner;
  EXECUTE `commerce_storefront_registrar` only (login `lc_store_registrar`, `cmd/store-admin` through `ops-admin.sh`, compose
  profile `ops`). No merchant HTTP route exists. Bind creates the origin directly ACTIVE or advances/renews a non-DETACHED row of
  the same store, stamping ownership/TLS verification with the database clock; `valid_until` is the TLS certificate notAfter
  (future, at most 400 days). The origin grammar is the 0020 one (SQL regex = internal/domains.ValidOrigin, PR01 parity).
  Lifecycle moves: bind -> ACTIVE (from none/REQUESTED/OWNERSHIP_PENDING/TLS_PENDING/SUSPENDED/ACTIVE), suspend -> SUSPENDED,
  detach -> DETACHED; each real move bumps `version` and audits `operator.domain_bound|suspended|detached`. A DETACHED origin is
  never re-bound (PT409 `domain_detached`); an origin of another store is PT409 `domain_owned_elsewhere`.
- **Cache.** There is none: every write bumps `version` and the next `buyer.resolve_published_store` sees it (PR04), so nothing
  needs invalidating.
- **Audit attribution.** `ops.audit_events.principal_id` is NOT NULL and an operator has no principal, so operator actions are
  attributed to the store's creating principal (`identity.initial_stores`; PT409 `store has no owner principal` when absent) with
  the `operator.*` action prefix. Not a claim that the merchant acted.
- **Pilot ceiling.** Caddy serves one store host per deployment, so one ACTIVE origin; per-store hosts need `on_demand_tls` with
  an `ask` endpoint backed by the resolver. Non-goals: custom merchant domains, DNS automation, wildcard TLS, per-store legal
  pages, unpublish side effects on open carts.
