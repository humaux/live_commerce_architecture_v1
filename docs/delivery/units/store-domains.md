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
