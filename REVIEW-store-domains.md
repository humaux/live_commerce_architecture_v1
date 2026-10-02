# Adversarial review — unit store-domains (R5)

Reviewer: independent test author / security reviewer (not the implementer). Base: `ec769a2` on
`unit/store-domains` (worktree `store-domains-tests`). Scope: `migrations/0106_store_domains.sql`,
`internal/storehandles`, `internal/storefrontdomains`, `internal/tlsask`, `internal/httpapi/storefront.go`,
`internal/identity` onboarding + `internal/identityhttp` handle-suggest, `apps/admin` StorefrontSettings /
Entry / onboarding BFF, `apps/storefront` origin handling, `deploy/caddy/Caddyfile`,
`deploy/scripts/preflight.sh` (P19), `cmd/store-admin`, the implementer's own tests
(`tests/foundation/store_domains_test.go` SDW01–SDW06, `internal/*_test.go`).

Method: read the contract (`docs/delivery/units/store-domains.md` Decisions 1–6 + Acceptance gate,
`架构.md` §7) first, then tried to break the implementation along: domain takeover, TLS ask, DNS
verification, TLS probe, handle rules, cookies/isolation, fail-closed resolution, deploy wiring.

**The focused gate command is RED at baseline** (`bash scripts/dev/test-focused.sh
'^(TestStoreDomains|TestR2IntegrationUpgradeFromReleaseHead|TestPublishedStorefront|TestStorefrontPublish|TestT06WorkerAuthorityAndFunctionACL)'`
→ exit 1, PASS=21 FAIL=2, evidence `output/store-domains-tests/baseline-focused.log`), **and so is the R3 browser
gate** (`bash scripts/dev/test-local.sh --browser-storefront-publish` → exit 1, evidence
`output/store-domains-tests/browser-storefront-publish.log` + `browser-r3-baseline-red/`): the frozen R3 driver
asserts the Settings card "has no input a merchant could bind a domain with"
(`tests/storefront/storefront-publish-gate.mjs:240`), and the R5 domain-request form this unit added to the same
card (`apps/admin/components/StorefrontSettings.tsx:283-299`) breaks that assertion before the driver publishes
anything. All of these failures are caused by this unit and block merge on their own.

---

## P0 — block merge

### P0-1 The unit breaks the R3 upgrade gate: 0106 cannot apply from the 0080 head
`tests/foundation/storefront_publish_test.go:297` (`TestStorefrontPublishSPW02UpgradeAfter0080`) fails:
`apply 0106_store_domains.sql: ERROR: role "commerce_storefront_writer" does not exist (SQLSTATE 42704)`.
SPW02 pre-marks 0081 in the ledger and applies every other migration to prove the chain applies cleanly
from the 0080 release head; 0106's `ALTER FUNCTION … OWNER TO commerce_storefront_writer` /
`GRANT … TO commerce_storefront_writer|registrar` (0106:226-230, 305-307, 436-438, 458-460, 472-474,
483-485, 501-503, 525-527) references roles 0081 creates, so 0106 hard-fails in that scenario. A second,
independent baseline failure is `TestStorefrontPublishSPW10DeployWiring`
(storefront_publish_test.go:1174): the unit rewrote `docs/runbooks/merchant-onboarding.md` and dropped
the `--valid-until` / `openssl` / `store-admin status` operator block the frozen R3 gate asserts.
Impact: the release's own focused command exits 1; nothing about this unit is mergeable until the
implementer/integrator restores both (a migration that applies in the SPW02 scenario, or an integrator
ruling amending SPW02; and a runbook that keeps the operator CLI reference). A third baseline failure is
the R3 browser gate above: the merchant domain form shares the frozen `storefront-card` section, tripping
the R3 driver's "no domain input on the card" assertion; either the R5 form moves to its own testid-scoped
section or the integrator amends the R3 driver to scope its count to the publication half of the card.

### P0-2 The DNS/TLS verification sweep has no production runner — merchant domains can never activate
`internal/storefrontdomains/verify.go:141` (`VerifyPending`) has **zero callers outside tests**
(repo-wide grep: only its own definition). `cmd/store-admin/main.go:69-107` has no `domain-verify`
subcommand even though verify.go:12's package comment names it ("the operator one-shot sweep,
cmd/store-admin domain-verify"), and `deploy/scripts/ops-admin.sh` — whose allowlist SPW10 freezes to
exactly `domain-bind|domain-suspend|domain-detach|status` — cannot run it either. No cron, no River job,
no worker service. Consequence: a merchant who adds a custom domain sits in REQUESTED for 72 h and
expires; Decision 3's lifecycle (`REQUESTED → OWNERSHIP_PENDING → TLS_PENDING → ACTIVE`) is dead after
the first step in every real deployment. The unit shipped the library and the tests but not the worker.

### P0-3 "301 to the primary origin" is never served
Decision 3: "Every non-primary ACTIVE origin answers 301 to the primary for GET (SEO canonical)" and the
brief's browser gate: "the old subdomain 301s to it". The Go side exists
(`internal/buyerhttp/primary.go:18`, `GET /v1/buyer/storefront/primary-origin`, PG
`control.resolve_primary_origin`) but **nothing calls it**: `grep -rn "primary-origin|primary_origin"
apps/` returns nothing. The storefront Next keeps answering 200 with full content on
`https://<handle>.<base>` after the merchant domain is ACTIVE. The endpoint is dead code; the acceptance
step cannot pass; SEO-canonical behaviour (the stated reason for the 301) is absent.

### P0-4 The Caddy catch-all never issues a certificate (missing `tls { on_demand }`)
`deploy/caddy/Caddyfile:42-46` puts `on_demand_tls { ask … }` in global options — per Caddy's docs this
only *configures* on-demand TLS. The catch-all site `https://` (Caddyfile:155-161) has **no
`tls { on_demand }` block**, which is the per-site switch that actually enables on-demand issuance.
With a hostless site address and no on-demand flag, Caddy has no automation policy for
`<handle>.<base>` or any merchant domain: every such TLS handshake fails at the edge. `caddy validate`
(smoke S05) passes because the config is syntactically valid — the behaviour is simply never enabled, so
none of the existing gates notices. The whole unit is dead at the deployed edge. (NOT_RUN here: no caddy
binary in this sandbox; verified against the Caddyfile docs semantics, and a static gate test added as
SDW12.)

---

## P1 — must fix before release

### P1-1 The DNS verifier accepts ANY A/AAAA record as "pointed at us" (and no proxied-CNAME refusal)
`internal/storefrontdomains/verify.go:88-91`:
```go
if addrs, err := r.LookupAddr(ctx, host); err == nil && len(addrs) > 0 { out.AddrFound = true }
out.Matched = out.TXTFound && (out.CNAMEMatch || out.AddrFound)
```
Decision 3 allows an A record **to the edge IP** for apex domains; the brief (Security, P18) requires the
TXT/CNAME check to **refuse a proxied CNAME target**. As written, any address passes. Exploit A: a
merchant sets the TXT token, points `shop.theirdomain` at their own VPS (A 192.0.2.1); verification
passes, and (with P1-2) the row goes ACTIVE while traffic never touches the platform — the store escapes
the §7.2 controlled-template sandbox under the platform's ACTIVE blessing, and once P0-3 is fixed the
platform subdomain would 301 buyers to that server. Exploit B (P18 bypass): a Cloudflare-proxied
(orange-cloud) CNAME returns no CNAME in the answer and Cloudflare anycast A records — `CNAMEMatch=false`
but `AddrFound=true`, so a proxied domain verifies, exactly what P18 forbids (the edge then sees proxy
IPs, not buyer IPs). Red-first test: SDW08.

### P1-2 The TLS proof is "some certificate exists", not Decision 3's per-row nonce through the edge
Decision 3: "A TLS probe then moves it to ACTIVE: **HTTPS GET of a per-row nonce path served by the
storefront through the edge**." `verify.go:42-57` (`SystemProber`) instead dials `host:443` wherever the
merchant's DNS points and accepts any valid leaf's `NotAfter`. A domain owner can always obtain a
certificate for their own domain from any CA, so the probe proves nothing about the platform serving the
host; combined with P1-1 a domain reaches ACTIVE with zero platform contact. The same dial is
SSRF-adjacent: a platform worker initiates outbound TLS to merchant-controlled addresses (impact bounded
to a handshake — but the nonce design in the decision exists precisely to avoid dialing arbitrary
answers and to bind activation to the edge). No test can encode the nonce against the current API shape;
flagged for the implementer to redesign with the nonce probe.

### P1-3 The tls-ask rate limit is one global bucket — unauthenticated internet DoS on certificate issuance
`internal/tlsask/tlsask.go:141-154`: a single fixed window of 120 asks/minute shared by all hostnames.
Caddy calls the ask for every ClientHello that presents an uncertified SNI — i.e. driven directly by the
public internet through the catch-all site. A scanner replaying random SNIs exhausts the bucket in
seconds; every subsequent ask — including the legitimate first handshake of a merchant whose domain just
became TLS_PENDING — gets 404 ("not eligible") and Caddy issues nothing. The negative cache does not help
(the attacker varies names). Denies for unknown hosts must not share the allow path's budget (per-host
keys, or count only DB-miss asks, or make the deny path effectively unbounded and rate-limit at the Caddy
`interval/burst` layer that already exists). Red-first test: SDW09.

### P1-4 Handle change strands the old origin ACTIVE; a re-issued handle binds the wrong store
Decision 1: "Changing it detaches the old platform origin (no silent redirect chains)." Nothing detaches
or suspends the old `https://<handle>.<base>` row when `control.stores.handle` changes (no trigger, no
definer; runbook §5.1 says post-publish changes are operator-only but gives no detach step). Chain of
failure: (1) operator changes store A's handle; A's old origin row stays ACTIVE and keeps serving A.
(2) The freed handle is assigned to a new store B at onboarding (`assign_store_handle` sees no conflict).
(3) B's `ensure_store_platform_domain` hits `ON CONFLICT (origin) DO NOTHING` (0106:223) and returns a
success-shaped `{handle, origin}` — onboarding tells B "your address is https://oldhandle.base" while
that address keeps resolving to **store A**. Cross-store address takeover via a naming race, created by
an approved operator action. Red-first test: SDW11.

---

## P2 — 8 findings

1. **`resolve_storefront_ask` ignores `valid_until`** (0106:494-500): an ACTIVE row past its validity is
   still admitted (cert renewed for a host the resolver already refuses to serve — `read_store_domains`'s
   `serving` and the 0081 resolver both check `valid_until`). Fail-closed should include expiry. Test SDW07.
2. **Dead privilege**: `GRANT EXECUTE ON control.ensure_store_platform_domain TO commerce_runtime`
   (0106:229) has no caller anywhere in Go (the migration comment promises a "lazy admin Settings" path
   that does not exist) — an unauthenticated-by-design definer (no p_hash/scope check inside) on the
   busiest login. Revoke or implement the caller.
3. **Merchant-suspended domain is unrecoverable self-service**: re-request of own SUSPENDED row →
   PT409 `domain_suspended` (0106:285-287); there is no un-suspend. One misclick in the Settings card
   permanently kills the domain pending an operator ticket (the operator CLI *can* re-bind after suspend,
   so the asymmetry is self-service-only).
4. **DETACHED host is un-bindable by every merchant forever**: owner re-request → `domain_detached`,
   any other store → `domain_owned_elsewhere` (0106:284-286), even with fresh DNS proof. 架构 §7.1
   requires re-binding to be possible *with re-proof*; self-service offers the proof but is refused.
   Only the operator CLI (`rebind_from_detached`) can. Either document as final-tombstone (then
   `domain_owned_elsewhere` leaks that the host was once bound — minor info leak) or allow re-request
   with fresh proof.
5. **`POST /v1/identity/handle-suggest` is unauthenticated and unrate-limited**
   (`internal/identityhttp/handler.go:96`, behind only the BFF key): anyone who can reach the admin BFF
   can enumerate which handles exist (`available: false`) before those stores publish. Handles become
   public addresses at publish, so the value is low, but the probe is free and unbounded.
6. **Ask deny-cache delays legitimate first issuance**: a host probed (by an SNI scan or a curious
   client) minutes before it flips to TLS_PENDING stays denied for the 5-minute deny TTL
   (`DefaultDenyTTL`, tlsask.go:37) — Caddy's own retry cadence is faster, so first-visit latency for a
   freshly-verified domain is bounded by the negative cache, not by issuance.
7. **Documentation promises what the code does not have**: verify.go:12 names a `domain-verify` CLI that
   doesn't exist (hid P0-2); `docs/delivery/GATES.md:71` describes the browser gate asserting the 301
   (hid P0-3) and zh-CN coverage; the runbook §5.1 describes self-service activation as if it completes.
   Gates and runbooks must not describe unshipped behaviour — this is how two P0s passed review.
8. **Merchant self-service can suspend/detach the store's own platform subdomain**
   (0106:338-396, `suspend_merchant_domain` / `detach_merchant_domain`): neither definer exempts the
   `evidence_ref='platform-subdomain'` row, and the Settings card renders suspend/detach buttons on every
   non-DETACHED row including the platform one (StorefrontSettings.tsx:262-274, `moveable`). One click in
   Settings permanently removes the store's own platform address — and there is no self-service way back,
   because `request_merchant_domain` refuses the whole base zone (0106:275-276). Decision 2 binds that
   address at onboarding as the store's permanent platform identity; only the platform/operator should be
   able to unbind it. Test SDW15 (red).

## Verified OK (no finding)

- Host/origin derivation: `candidateOrigin` (apps/storefront/lib/public-upstream.ts) rejects ports, IPs,
  userinfo, trailing dots, case; Go `domains.ValidOrigin` parity; the resolver (0081, existing gate)
  admits only ACTIVE in-window origins; unknown Host fails closed (404, no default store).
- Takeover via case / trailing dot / punycode merchant host: lower-cased before the unique origin check;
  regex refuses empty last label; `xn--` merchant hosts are allowed only with real DNS proof (they must
  control the zone) — no takeover.
- Under-base refusal (`shop.<base>`, `<base>` itself) in Go (`underBase`) and SQL (0106:275-276);
  reserved handle list + `xn--` in `store_handle_reserved`, enforced by the BEFORE INSERT trigger;
  unique partial index is the concurrency backstop (raw duplicate → 23505).
- Definers keep the `p_hash + resolve_access(permission) + WithScope` re-check pattern
  (`integration:manage`/`integration:read`); tenant/store come from server auth only; the verification
  token is never written to audit or logs (policy admits nine fixed actions; SDW06 inventory passes).
- tls-ask transport: 404 for every denial class (no oracle between unknown/rate-limited/suspended), 422
  on malformed, 405 on non-GET, 503 on DB error; endpoint mounted only on the internal api listener
  (`{$LC_API_HOST}` exposes only `/healthz`; Caddy asks over `http://127.0.0.1:8080`).
- Cookies: this unit adds no cookie; admin session cookies stay host-only (unchanged code path).
- Preflight P19 (wildcard + `stores.<base>` DNS-only to this host) is correctly modelled on P17/P18.
- ACL inventory (SDW06): all 18 functions SECURITY DEFINER with `search_path=pg_catalog`, PUBLIC revoked,
  retired `commerce_worker` holds nothing.

## Tests added by this review (all in `tests/foundation/store_domains_adversarial_test.go`)

| Test | Finding | Red on current code? |
| --- | --- | --- |
| SDW07 ask admit/deny incl. expired `valid_until` + trailing-dot/upper-case host | P2-1 | **RED** (expired ACTIVE admitted) |
| SDW08 DNS verifier refuses non-edge A records / proxied-CNAME shape | P1-1 | **RED** |
| SDW09 ask rate limit must not starve a legitimate allow | P1-3 | **RED** |
| SDW10 ask HTTP matrix fail-closed (method/query/unknown/DB-down) + no-leak | hardening | green |
| SDW11 handle change must detach the old origin; re-issued handle must not bind the wrong store | P1-4 | **RED** |
| SDW12 Caddyfile catch-all must enable `tls { on_demand }` | P0-4 | **RED** |
| SDW13 a production runner for the verify sweep must exist (`domain-verify` wired) | P0-2 | **RED** |
| SDW14 reserved-name / under-base / duplicate-handle refusals (mutation targets) | hardening | green |
| SDW15 merchant suspend/detach must refuse the store's own platform-subdomain origin | P2-8 | **RED** |

Browser gate: `tests/foundation/browser_store_domains_test.go` + `tests/storefront/store-domains-gate.mjs`
(the `--browser-store-domains` mode was registered but had no test files). The gate encodes the contract
including the platform-subdomain 301 (P0-3) — it is RED at that step on current code; every step before it
(onboarding → handle → publish → served at `https://<handle>.<base>` → custom domain → DNS instructions →
MOCK DNS/TLS sweep → ACTIVE at the custom host) is driven end-to-end.
