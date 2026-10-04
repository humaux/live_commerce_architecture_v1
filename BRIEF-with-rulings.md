# Unit store-domains — every store gets an address automatically; merchants can bring their own domain (R5)

Owner (2026-10-02): "自动给入驻的商家分配网址，商家可以使用自己的域名，如果商家没有自己的域名的话，就使用商家的英文名作为店铺ID".
架构.md §7.1 already specifies it: one Store resolved by a platform subdomain and by merchant domains, with the lifecycle
REQUESTED → OWNERSHIP_PENDING → TLS_PENDING → ACTIVE → SUSPENDED → DETACHED; only ACTIVE resolves; an unknown Host fails closed;
binding is verified independently of certificate issuance.
Today (156b1ca): `control.storefront_domains` has that lifecycle, but the only writer is the operator CLI
(`ops-admin store-admin domain-bind`, migration 0081). The admin shows "awaiting platform domain"
(apps/admin/components/StorefrontSettings.tsx:8), so a new merchant cannot reach their shop without an operator.

Read: 架构.md §7, contracts/storefront-v2.md (publication + resolver), migrations/0020 and 0081, internal/domains, internal/storefrontadmin,
apps/admin/components/StorefrontSettings.tsx and the onboarding wizard (apps/admin/components/Entry.tsx and create_initial_store),
deploy/caddy/Caddyfile, deploy/scripts/preflight.sh (P17/P18), docs/runbooks/merchant-onboarding.md.

## Decisions (binding unless the contract-freeze review rejects one with evidence)
1. **Store handle.**
   - Add `control.stores.handle`: lower-case ASCII, `^[a-z0-9]([a-z0-9-]{1,28}[a-z0-9])$`, UNIQUE across the platform.
   - Refuse a reserved list: www admin api hooks shop mail static cdn assets app help support status stores, plus anything `xn--`.
   - Onboarding asks for the store's English name / store ID, suggests a slug of it, checks availability live, and suffixes `-2`, `-3` on collision.
   - Existing stores are backfilled from their name when it slugs to a valid handle, otherwise `store-<first 8 hex of id>`.
   - The handle can change only while the store has never been published. After that only the operator can change it. Changing it detaches the old platform origin (no silent redirect chains).
2. **Platform address.**
   - Creating a store (and the backfill) writes an ACTIVE `https://<handle>.<LC_STORE_BASE_DOMAIN>` domain row.
   - The platform owns that zone, so ownership is implied; evidence is recorded as `platform-subdomain`.
   - A merchant is reachable the moment they publish, with no operator step.
3. **Merchant domain (self-service).**
   - In Settings > Storefront the owner/admin enters a hostname. This writes REQUESTED with a random verification token, then shows the DNS records to add: `CNAME <host> → stores.<LC_STORE_BASE_DOMAIN>` (A record to the edge IP for an apex domain) and `TXT _lc-verify.<host> = <token>`.
   - A worker job verifies DNS with backoff, bounded to 72 h; on failure the row stays visible and is not ACTIVE. Once TXT and CNAME/A match → OWNERSHIP_PENDING → TLS_PENDING.
   - A TLS probe then moves it to ACTIVE: HTTPS GET of a per-row nonce path served by the storefront through the edge.
   - Suspend and detach are owner actions as well as operator actions.
   - One primary origin per store: the merchant domain when it is ACTIVE, otherwise the platform subdomain. Every non-primary ACTIVE origin answers 301 to the primary for GET (SEO canonical).
4. **Edge TLS.**
   - Caddy `on_demand_tls { ask http://api:<port>/internal/tls-ask }` plus a catch-all `https://` site to the storefront.
   - The ask endpoint (Go, internal network only) answers 200 only for a hostname that is an ACTIVE platform subdomain or a merchant domain in TLS_PENDING or ACTIVE; it is rate-limited and caches negatives briefly.
   - Explicit hosts (admin, api, hooks, shop) keep their own blocks. Unknown hosts get no certificate (fail closed).
   - Owner DNS action: a DNS-only wildcard `*.<base>` to the edge IP, plus `stores.<base>`.
   - **Done 2026-10-02** through the Cloudflare API (owner authorised it): zone xgdwm.com, `*.xgdwm.com` and `stores.xgdwm.com` as A records to 35.212.187.34, DNS-only, TTL 300. Verified via DoH. `LC_STORE_BASE_DOMAIN=xgdwm.com`.
5. **Security** (架构.md §7.1):
   - Cookies stay host-only. No admin session on store hosts.
   - The resolver trusts only the edge-set Host.
   - P18 (DNS-only) also applies to merchant domains: the TXT/CNAME check refuses a proxied CNAME target.
   - Domains are never PII, but verification tokens are never logged.
6. Forward migration(s) only. No grant to the retired commerce_worker. Merchant definers follow the existing `p_hash + resolve_access` pattern with permission `integration:manage`.

## Acceptance gate (red first, then green)
- **PG:** handle format, reserved names, uniqueness, collision suffix, backfill of existing stores, platform row ACTIVE at create, merchant domain lifecycle transitions and refusals (taken host, invalid host, platform zone, reserved), owner-only permissions, ACL inventory.
- **Go:** the ask endpoint (allow/deny matrix, rate limit), DNS verifier with a fake resolver (match, mismatch, NXDOMAIN, timeout, 72 h expiry), TLS probe, 301 to primary.
- **Browser** (new mode `--browser-store-domains`):
  - wizard with English name → handle shown → publish → the storefront is served at `https://<handle>.<base>` through the test edge (Host mapping);
  - the merchant adds a custom domain, sees the DNS instructions, the fake DNS turns green → ACTIVE → the old subdomain 301s to it.
  - zh-TW / zh-CN / en, desktop + 390px.
- **Deploy:** Caddyfile validates; preflight check for the wildcard DNS; runbook §6.x for the owner DNS records; `docs/runbooks/merchant-onboarding.md` updated (no more operator `domain-bind` for normal onboarding).

## Review rulings (integrator, 2026-10-02) — K3 review `REVIEW-store-domains.md` at 798a136 (4 P0, 4 P1, 8 P2)
All K3 red tests (SDW07–SDW15, `--browser-store-domains`) stay as written; the fix makes them green. Rulings:
- **P0-1:**
  - **SPW02 test design.** "Every migration except 0081" stopped meaning "the 0080 head" once later migrations existed.
    - Amend SPW02 so the head is really the 0080 head: ledger-mark every migration ≥0081 with its checksum, apply, then delete those marks and apply the whole chain 0081→latest twice. This is stricter: it proves the full upgrade chain from 0080.
    - 0106 keeps its grants.
  - **SPW10.** Restore the runbook's operator CLI block (`--valid-until`, openssl, `store-admin status`) as the documented fallback.
  - **R3 browser driver.** It stays frozen. The merchant domain form moves out of `storefront-card` into its own Settings section (`data-testid="storefront-domains-card"`).
- **P0-2 (DNS verification sweep).**
  - Run `VerifyPending` as a River periodic job in an existing worker. Follow the `internal/retention/job.go` pattern, with backoff and the 72 h expiry.
  - Do not add a `store-admin domain-verify` CLI. Remove the doc and comment claims that it exists.
- **P0-3 (301 to the primary origin).**
  - The storefront answers 301 to the primary origin for GET/HEAD on every non-primary ACTIVE origin, preserving path and query. The target comes only from the DB (no open redirect).
  - Prefer returning `primary_origin` from the existing host resolution over adding a second call per request.
- **P0-4 (Caddy on-demand TLS).**
  - The catch-all site gets `tls { on_demand }`.
  - The gate runs real Caddy (docker `caddy adapt`) and asserts that the catch-all automation policy has `on_demand: true`; a static grep alone is not enough.
- **P1-1 (DNS verifier).** The CNAME must equal `stores.<base>`, or A/AAAA must equal the edge address set. Any other address, including a proxied CNAME's anycast A records, is refused.
- **P1-2 (TLS probe).** Implement Decision 3 as written:
  - a per-row nonce served by the storefront at `/.well-known/lc-domain-check/<nonce>`, for TLS_PENDING hosts and that path only;
  - the worker dials the edge address with SNI = the host (never the merchant's DNS answer), verifies the certificate and compares the body with the nonce.
- **P1-3 (tls-ask rate limit).**
  - The ask answers from an in-memory set of admitted hosts (ACTIVE in-window platform subdomains, plus merchant domains in TLS_PENDING/ACTIVE in-window), refreshed every 30 s. There is no DB query per ask and no global bucket.
  - This also fixes P2-1 (valid_until) and P2-6 (deny cache).
- **P1-4 (handle change).**
  - Changing a handle detaches the old platform origin in the same transaction.
  - `assign_store_handle` treats a handle as unavailable while any domain row for `<handle>.<base>` belongs to another store.
  - `ensure_store_platform_domain` raises (it does not return success) when the origin belongs to another store.
- **P2s to fix in this pass:**
  - **P2-2:** revoke the dead `commerce_runtime` grant.
  - **P2-3:** the owner can re-request their own SUSPENDED domain; it returns to REQUESTED with a fresh token.
  - **P2-4:** a DETACHED host can be re-requested with fresh DNS proof (架构 §7.1).
  - **P2-7:** docs and GATES must describe only shipped behaviour.
  - **P2-8:** merchants cannot suspend or detach the platform-subdomain row, and the card shows no such buttons.
- **P2-5 (handle-suggest is unauthenticated and unrate-limited):** accepted as low risk; logged. Revisit when the public signup is opened.
