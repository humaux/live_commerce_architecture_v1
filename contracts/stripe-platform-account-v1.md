# Stripe platform account v1: one platform Stripe account collects for every store

Status: **DRAFT, FROZEN-ready, 2026-10-06 (DESIGN; every PF gate NOT_RUN)**. Integrator review and freeze are pending.
Evidence label for this file: DESIGN. Nothing here moves real money. The only LIVE charge is still the
owner's single canary on the platform store (stripe-live-enable §10). It is done **once**, not once per store.

Amends:
- [stripe-psp-v1](stripe-psp-v1.md): §0.2 "Account identity", §0.2 "Per-account webhook routing", §5.4 key set, §9.1 mapping.
- [stripe-live-enable-v1](stripe-live-enable-v1.md): LD2, LD5, LD7, LQ1 and §13 "one Stripe account per store".
- [stripe-refund-v1](stripe-refund-v1.md): RD1 "balance transactions out".
- Architecture §2.2 (deviation AD-PF1, §0.2 below).

Each amended file carries a one-paragraph pointer to this file. Every rule in those contracts that this file does not
name stays in force. That includes the amount table, minor units, idempotency keys, the lease-fenced loaders, refund
capacity RD3, the kill-switch layers, W11 and the SANDBOX/LIVE split. **No Connect, no new engine, no new queue, no
new dependency, no new process.**

## 0. Owner decision, facts and the conflicts it resolves

### 0.1 Owner decision (2026-10-06, verbatim)

「不使用PAYUNi，直接允许商家使用平台的stripe进行收款」

In English: do not use PAYUNi. Let merchants collect card payments directly through the platform's Stripe account.

| # | Fact | Source | Status |
| --- | --- | --- | --- |
| PF-F1 | Platform account `acct_1UJDadRzKpmj4jFL`, HK, default currency HKD, `charges_enabled=true`, `payouts_enabled=false`. | Integrator brief 2026-10-06 (Stripe MCP read by the integrator) | LOCAL. `live-approve` still re-reads it (LD8), and `payouts_enabled=false` fails readiness until the owner finishes payout setup. |
| PF-F2 | Taiwan is not a Stripe-supported account country. So Taiwan merchants cannot have connected accounts, and HK→TW Connect transfers are unavailable. | Integrator brief, consistent with stripe.com/global | LOCAL, not re-fetched this session. |
| PF-F3 | Without `on_behalf_of` or Connect, the account that creates the charge is the business of record. Its statement descriptor and business name are what the buyer sees. | [separate charges and transfers](https://docs.stripe.com/connect/separate-charges-and-transfers) ("If `on_behalf_of` is omitted, the platform is the business of record") | VERIFIED 2026-10-06 (Stripe docs MCP) |
| PF-F4 | A per-charge suffix is set through `payment_intent_data[statement_descriptor_suffix]` and joined to the card prefix. The full descriptor is at most 22 characters. | stripe-live-enable L7 | VERIFIED (2026-09-29) |
| PF-F5 | Balance transactions carry `id, type, amount, fee, net, currency, exchange_rate, created, source, reporting_category`. They can be listed by `created` range and expanded with `expand[]=data.source`. Amounts are in the **settlement** currency (HKD here). | [balance transactions API](https://docs.stripe.com/api/balance_transactions) | **UNVERIFIED this session.** W4-S2 must WebFetch the object page and record the exact fields before it writes code. A missing field fails closed (§6.3). |
| PF-F6 | Stripe positions multi-party businesses (marketplaces, platforms) on Connect. Whether the Stripe Services Agreement lets a non-Connect HK account collect for third-party Taiwan stores is **not established**. | Stripe docs (Connect overview), no explicit permission found | **UNKNOWN → owner question OQ-1 (blocking LIVE enrollment).** |

### 0.2 Conflicts with frozen documents and how this file resolves them

| Frozen text | Conflict | Resolution |
| --- | --- | --- |
| Architecture §2.2: 「不能先把全部商家货款…收成一个平台资金池」, and the merchant should be the MoR | The platform account becomes the business of record and collects every store's card money. That is a pooled-funds model. | **Deviation AD-PF1.** This is the owner's explicit business decision, recorded here because the repository has no ADR folder. Architecture §2.2 gets a pointer line. Mitigations: per-store attribution of every money fact (§4) and a per-store ledger with statements (§6); buyer disclosure (§5); kill switches (§7); **LIVE enrollment opens only after OQ-1, OQ-4 and OQ-5 are answered** (§8 checklist codes). |
| stripe-psp §0.2: "One provider account cannot silently become the common collection account of unrelated stores", enforced by unique index `stripe_account_identity_unique (environment,account_id)` | Every store must use the same account. | The account may be shared only **explicitly**. A store's connection is a *derived* row that points at the one designated platform connection (§3.2). The unique index keeps applying to primary rows. Unrelated stores can never share a primary registration silently. |
| stripe-psp §0.2: one webhook endpoint per store connection | Stripe allows at most 16 endpoints per account (stripe-live-enable L4), so per-store endpoints cannot scale. | One endpoint per platform connection and profile serves every derived store. Mapping is server-side (§4.2). |
| stripe-live-enable LD2/LD5/LQ1/§13: approval, canary and per-order max per store; "other merchants need their own account" | Owner wants「直接允许」: no per-store credential and no per-store operator approval. | Approval, readiness and canary are done **once** on the platform connection. A store enables itself (§3.3) and inherits the platform approval's currency and caps, including LQ3. LQ1's default is superseded. |
| stripe-live-enable LD7: "the per-store approval is the owner's approval for merchant refunds" | No per-store approval exists any more. | The platform approval plus the store's accepted terms version (§3.1) authorize that store's merchant refunds. RD3 capacity is unchanged (§4.3). |
| stripe-psp §5.4: exact create key set | The store tag and descriptor suffix are new keys. | Three keys are added (§4.1). SP03 vectors change. |
| stripe-refund RD1: balance transactions are out of scope | The ledger needs fees and disputes. | Balance transactions are **read** for the ledger only (§6). They are never refund or payment authority. |
| W4-01B/02B/03B (PAYUNi), INDEX D3 "Stripe only for the owner's HK store" | Superseded by §0.1. | W4-02B and W4-03B are cancelled. The merged W4-01B receiver stays disabled and is removed by `pay-remove-payuni` (forward migration). |

Invariants are unchanged and upheld as follows:
- **I01**: the store comes from the authenticated token or from the server-side attempt row, never from metadata.
- **I05**: a fact still requires the attempt's connection account, currency, amount and order to match. The derived connection's `account_id` is the platform account.
- **I06/I20**: idempotency keys are unchanged. Ledger rows are keyed by Stripe balance transaction id.
- **I16**: the platform switch and kill switches only stop new starts. Reconcile and refunds keep running.
- **I17**: the capability is labelled `LIVE (owner canary)` until a non-developer Taiwan merchant completes a LIVE order and refund. Only then can it be `production_supported`.
- **I19**: PostgreSQL stays the single source of truth.

### 0.3 Amendment AD-PF2 (owner 2026-10-06, after legal research): the platform account collects for the platform owner's own stores only

Legal research found that the platform Stripe account collecting card money for **third-party** merchants conflicts with Stripe's
Services Agreement §3.4(b) and with Taiwan third-party-payment rules. Therefore, until **plan A (a merchant-owned Taiwan PSP)**:
**third-party merchants: not allowed.** The platform account is used only for stores of the same legal entity as the Stripe
account (the platform owner's own store(s)).

- **Allowlist.** A store may self-enable card payments (§3.3) only if an operator allowlisted it:
  `ops-admin.sh stripe-admin platform-allow|platform-disallow --target-tenant --target-store --environment --operator --ticket`
  (SQL `payments.allow_platform_stripe`, table `payments.platform_stripe_allowlist`, audit `stripe.platform.allow|disallow` in the
  platform scope with the target ids in `details`). `ops.audit_events` plus the table's own `allowed_by/allowed_ref` are the audit
  trail; OPS-01B's `control.operator_audit` is not on this base, so the integrator may re-point the audit when it merges.
- **Gate.** Without an active allowlist row `set_platform_stripe(enabled=true)` is refused with `platform_stripe_not_allowed`
  (403), before any platform-state check. Disable is never gated. Withdrawing the allowlist does **not** stop sales of an enrolled
  store: use `platform-block` (§7).
- `read_platform_stripe` returns `allowed` (boolean) so the admin UI can hide the toggle.
- Everything else in this file (attribution §4, kill switches §7, per-order cap, refunds limited to the store's own captures, the
  settlement ledger) is unchanged. Sections that say "every store" / "merchant" read as "every allowlisted store".

## 1. Model in one paragraph

The owner registers the platform Stripe account **once** with the existing operator flow. That flow covers register,
webhook, live-approve, qualify, method and canary on the **platform store**, which is the owner's own store with a TWD
market. The operator then *designates* that connection as the platform connection for its environment. When the platform
is OPEN, a merchant with `billing:manage` turns on card payments for their store in admin (self-serve). One SQL
definer then creates that store's *derived* connection, the credential reference, the derived qualification and the
method. The existing start, worker, webhook and refund paths run unchanged against those per-store rows, so every
attempt, fact, refund and ledger line stays in the store's own tenant scope. Money settles into the platform's Stripe
balance (HKD). The platform pays each store off-Stripe from a per-store statement (§6).

## 2. Platform state (derived, never stored as a status)

| State | Derived from | Merchant enrollment | Buyer starts on enrolled stores |
| --- | --- | --- | --- |
| NONE | no `payments.stripe_platform` row for the deployment environment | refused `platform_stripe_unavailable` | none |
| DESIGNATED | row exists, and for LIVE: no active approval, or canary not verified | refused | none |
| OPEN | LIVE: active approval with `canary_verified_at` set, a valid REAL_LIVE platform qualification and `enrollment_open=true`. SANDBOX/MOCK: a valid platform qualification and `enrollment_open=true` | allowed | yes, up to `max_minor` (LQ3 TWD 2,000,000) |
| CLOSED | OPEN except `enrollment_open=false` | refused `platform_stripe_closed` (re-enable is also refused) | **yes**: closing stops new enrollments, not existing stores |
| REVOKED | platform approval revoked (`live-revoke`) | refused | none. Every derived qualification is revoked in the same transaction (§3.4). |

The deployment-wide switches are unchanged: the flag+ref pair and `LC_STRIPE_CHECKOUT_ENABLED=0`
(stripe-live-enable LD6).

## 3. Persistence: `migrations/0137_platform_stripe.sql` and post-River `0022_platform_stripe.sql` (placeholders)

### 3.1 New tables (0137; FORCE RLS; PUBLIC revoked; no DELETE grant to anyone)

```
payments.stripe_platform                     -- one row per environment; owner payments/stripeadmin
 environment text PRIMARY KEY CHECK(environment IN ('SANDBOX','LIVE')),
 tenant_id, store_id, connection_id uuid NOT NULL,          -- the platform store's primary Stripe connection
 account_id text NOT NULL,                                   -- = merchant_accounts.account_id (FK on the tuple)
 display_name text NOT NULL CHECK(char_length(display_name) BETWEEN 2 AND 60 AND display_name !~ '[[:cntrl:]<>]'),
 descriptor_display text NOT NULL CHECK(descriptor_display ~ '^[A-Za-z0-9 .*-]{5,22}$'), -- operator-attested text of the card prefix shown to buyers
 enrollment_open boolean NOT NULL DEFAULT false,
 terms_version text NOT NULL CHECK(terms_version ~ '^[a-z0-9.-]{3,40}$'),       -- current merchant terms
 platform_fee_bps integer NOT NULL DEFAULT 0 CHECK(platform_fee_bps BETWEEN 0 AND 3000),
 version bigint NOT NULL DEFAULT 1, updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 FOREIGN KEY(tenant_id,store_id,connection_id,provider,environment,account_id)  -- provider generated 'stripe'
   REFERENCES integration.merchant_accounts(tenant_id,store_id,id,provider,environment,account_id),
 UNIQUE(connection_id)

payments.platform_stripe_enrollments         -- one row per (store, environment)
 tenant_id, store_id uuid NOT NULL, environment text NOT NULL,
 connection_id uuid NOT NULL UNIQUE,         -- the derived merchant_accounts row
 enrolled_by uuid NOT NULL,                  -- FK identity.memberships
 terms_version text NOT NULL, accepted_at timestamptz NOT NULL,
 descriptor_suffix text CHECK(descriptor_suffix ~ '^[A-Za-z0-9][A-Za-z0-9 .-]{1,9}$' AND descriptor_suffix ~ '[A-Za-z]'),
 blocked_at timestamptz, blocked_ref text CHECK(blocked_ref ~ '^[A-Za-z0-9._:-]{8,128}$'), blocked_by text,  -- operator name
 version bigint NOT NULL DEFAULT 1, created_at, updated_at,
 PRIMARY KEY(tenant_id,store_id,environment),
 FOREIGN KEY(tenant_id,store_id) REFERENCES control.stores(tenant_id,id),
 CHECK((blocked_at IS NULL)=(blocked_ref IS NULL) AND (blocked_at IS NULL)=(blocked_by IS NULL))
```

Suffix length rule: let `L = PrefixLength` from the approval's `account_readiness`, or `min(DescriptorLength,10)` when
`PrefixLength=0`. Then `L + 2 + char_length(suffix) ≤ 22` (PF-F4). The enable definer checks it and fails with
`descriptor_suffix_too_long`. SANDBOX has no approval, so the check uses `L=10`.

### 3.2 Widened objects (0137)

| Object | Change |
| --- | --- |
| `integration.merchant_accounts` | Add `platform_connection_id uuid NULL REFERENCES integration.merchant_accounts(id)`, with `CHECK(platform_connection_id IS NULL OR provider='stripe')`. Recreate `stripe_account_identity_unique` as `(environment,account_id) WHERE provider='stripe' AND platform_connection_id IS NULL`. `stripe_one_account_per_store_environment` is **unchanged**, so a store holds either its own primary connection or one derived connection per environment. Trigger `guard_derived_stripe_account`: a derived row's `environment` and `account_id` must equal the referenced row's. The referenced row must be the `stripe_platform.connection_id` of that environment. `platform_connection_id` is immutable. Derived rows may be inserted only by the enable definer's owner. |
| `integration.account_credentials` | Add `sealed_version bigint NULL`. It is set exactly on rows of derived connections. Trigger `guard_derived_credential`: `key_id`, `nonce` and `ciphertext` must be byte-equal to the platform connection's credential row at `sealed_version`. The envelope is **copied, never re-encrypted**: SQL never sees plaintext, `cmd/api` holds no API keyring, and the AAD stays bound to the platform scope. The frozen "no `ENV_PLATFORM`, no nullable ciphertext" rule still holds. |
| `payments.account_qualifications` | Add `platform_qualification_id uuid NULL REFERENCES payments.account_qualifications(id)`. Replace the 0077 REAL_LIVE CHECK with: `code<>'stripe_checkout' OR proof_class<>'REAL_LIVE' OR live_approval_id IS NOT NULL OR platform_qualification_id IS NOT NULL`. Add `CHECK(NOT (live_approval_id IS NOT NULL AND platform_qualification_id IS NOT NULL))`. Extend `account_qualification_revoke_only` to rows with `platform_qualification_id IS NOT NULL`. |
| `payments.stripe_webhook_endpoints` | No column change. `set_stripe_webhook_endpoint` refuses a derived connection (`stripe_platform_derived`). |

### 3.3 SQL entry points (0137 unless marked)

All functions below are SECURITY DEFINER with `search_path=pg_catalog`, PUBLIC revoked and one `ops.audit_events` row
each.
- **Merchant functions** are owned by `commerce_payment_registry_writer` and EXECUTE `commerce_runtime` only. They
  resolve scope with `identity.resolve_access(token, store, <perm>)`, then set `app.tenant_id`/`app.store_id` from the
  resolved scope. Registry-writer RLS policies (0061 pattern) apply.
- **Operator functions** are EXECUTE `commerce_payment_registrar` only and use `require_stripe_registrar_scope` on the
  **platform** store.

| Function | Contract |
| --- | --- |
| `payments.set_platform_stripe(p_token bytea, p_store uuid, p_profile text, p_enabled boolean, p_terms_version text, p_descriptor_suffix text, p_expected_version bigint) RETURNS jsonb` | Merchant function. Requires `billing:manage`. Enabling also requires the tenant and store to be active (OPS-01B flags), platform state OPEN (§2), `p_terms_version = stripe_platform.terms_version`, the store's market currency = approval currency (LIVE) or `stripe_min_minor(currency) IS NOT NULL` (SANDBOX), no block, and the store **not** holding a primary Stripe connection in this environment (`stripe_store_has_own_account`). On first enable, one transaction creates: binding, derived `merchant_accounts` row (`principal_id` = caller), derived credential version 1 (`sealed_version` = platform head), derived qualification (`proof_class`, `environment` and `expires_at` copied from the platform's current valid qualification, `platform_qualification_id` set, `evidence_ref='platform:'||<platform qualification id>`), enrollment row, method version and head (`enabled`, `visible`, `min = stripe_min_minor(currency)`, `max = approval.max_minor` (LIVE) or the SANDBOX `stripe_amount_ok` max, default names 信用卡 / 信用卡 / Card). Re-enable reuses the rows. If the platform state moved since the derived rows were made, it refreshes them as §3.4 does. Disable (`p_enabled=false`) is **always allowed**, including when the store is blocked or suspended or the platform is CLOSED or REVOKED. It sets the method disabled (the stripe-live-enable §3.4 rule) and keeps every row. CAS on `enrollments.version` → PT409. Replaying identical input returns the stored result. Audit `stripe.platform.enable` / `.disable`. Returns `{state, version, max_minor, currency, descriptor_preview}`. |
| `payments.read_platform_stripe(p_token bytea, p_store uuid, p_profile text) RETURNS jsonb` | Merchant function. Requires `integration:read`. Returns `{platform_state, store_state: NONE\|ENABLED\|DISABLED\|BLOCKED, terms_version, accepted_terms_version, display_name, descriptor_preview, currency, min_minor, max_minor, version}`. No account id, key or approval data. |
| `payments.designate_stripe_platform(tenant, store, principal, connection uuid, display_name text, descriptor_display text, terms_version text, expected_version bigint) RETURNS bigint` | Operator function. The connection must be a **primary** Stripe connection of that store. The environment is the account's. An existing row may be updated only while no enrollment exists, or only in `display_name`/`descriptor_display`/`terms_version` (moving the connection would orphan derived rows → PT409 `platform_has_enrollments`). Audit `stripe.platform.designate`. |
| `payments.set_stripe_platform_open(tenant, store, principal, environment text, open boolean, platform_fee_bps integer, expected_version bigint) RETURNS bigint` | Operator function. `open=true` requires the OPEN preconditions minus the flag itself. `platform_fee_bps` changes apply only to statements closed afterwards (§6). Audit `stripe.platform.open` / `.close`. |
| `payments.block_platform_stripe(tenant, store, principal, target_tenant uuid, target_store uuid, environment text, blocked boolean, operator text, ref text) RETURNS timestamptz` | Operator function: the per-store kill switch (§7). Block sets the enrollment block triple, revokes the target's derived qualification (start → PT409) and disables its method. Unblock clears the triple only. The merchant then re-enables, which re-derives. The audit row is written in the platform scope, with the target ids in the details. |
| `payments.platform_stripe_fanout(p_environment text, p_reason text) RETURNS integer` | **Internal.** No EXECUTE grant. Called only from the re-created registrar definers in §3.4. It loops over enrollments of the environment that are not blocked. For each it sets the scoped GUCs, refreshes the derived rows and returns the count. Bound: **≤ 2000** enrollments per call, otherwise PT409 `platform_fanout_too_large`. Above 2000 enrollments, fan-out would move to a River job. |

### 3.4 Re-created definers (same signatures, owners and grants; body deltas only)

- **`integration.rotate_stripe_key`** (latest body): when the connection is a `stripe_platform.connection_id`, call
  `platform_stripe_fanout(env,'rotate')` in the same transaction. For each store it appends derived credential version
  n+1 (`sealed_version` = new platform head), inserts a fresh derived qualification **only if** the platform has a
  valid qualification for the new head (otherwise none, which blocks starts as the existing rule does), and CASes the
  method head to it. When called on a derived connection it fails with PT409 `stripe_platform_derived`.
- **`payments.qualify_stripe_method`**: on the platform connection, fan out `'qualify'`, which gives new derived
  qualifications with the new `expires_at` and CASes the method heads. On a derived connection it is refused.
- **`payments.revoke_stripe_live`**: on the platform approval, fan out `'revoke'`, which revokes every derived
  qualification in the same transaction. The lock order stays account → binding → approval → qualification →
  method head, taken per store inside the loop after the platform rows.
- **`payments.set_stripe_method`**: on a derived connection only `p_enabled=false` is admitted. That is the operator
  path of the per-store kill switch, and it needs a principal holding registrar scope on the target store. In
  practice the operator uses `block_platform_stripe`.
- **`integration.register_stripe_account`, `payments.approve_stripe_live`, `payments.record_stripe_live_canary`,
  `payments.set_stripe_webhook_endpoint`**: refuse a derived connection.
- **Credential loaders: `integration.load_stripe_credential`, `integration.load_stripe_refund` and
  `payments.stripe_registrar_credential`.** Each gets trailing return columns `aad_tenant_id uuid, aad_store_id uuid,
  aad_connection_id uuid, aad_version bigint` (DROP + CREATE in one transaction, then grants restored). For primary
  rows these equal the row's own scope and version. For derived rows they name the platform connection and
  `sealed_version`. Go (`accounts/stripe_crypto.go`) opens the envelope with the `aad_*` scope. Every other check is
  unchanged: the attempt's own scope, the lease fence, `VerifyAccount` against `account_id`, and the refund
  current-head rule RD13 applied to the *derived* head.
- **post-River `checkout.start_stripe_payment`**: no admission change, because it already validates the derived
  qualification, credential head and method. It adds the §4.1 keys to `create_params`.
- **post-River `payments.stripe_webhook_prepare`** and the refund-event prepare branch (stripe-refund §7.3): mapping
  delta in §4.2.

### 3.5 Grants and ACL pins

- `commerce_runtime` gets EXECUTE on `set_platform_stripe` and `read_platform_stripe` only.
- `commerce_payment_registrar` gets EXECUTE on `designate_stripe_platform`, `set_stripe_platform_open` and
  `block_platform_stripe`.
- `platform_stripe_fanout` has no EXECUTE grant.
- `commerce_payment_registry_writer` gets SELECT/INSERT and narrow UPDATEs on the two new tables, plus INSERT on the
  derived rows of bindings, merchant_accounts, account_credentials and account_qualifications. It already holds most of
  these.
- Workers, ingress, `commerce_checkout_writer` and `commerce_integration_writer`: no privilege on
  `stripe_platform` or `platform_stripe_enrollments`. Runtime admission reads only the derived rows. The ingress prepare
  definer (integration_writer) reads `merchant_accounts.platform_connection_id`, for which it already holds SELECT.
- **Accepted deviations from "no privilege" (integrator ruling, review of W4-S1):**
  - `commerce_checkout_writer` may SELECT `stripe_platform(environment, connection_id, display_name, descriptor_display)` and
    `platform_stripe_enrollments(tenant_id, store_id, environment, connection_id, descriptor_suffix)` (scope-bound policy). The start
    definer needs the suffix and the hosted view needs the disclosure. Never an account id. Pinned in the SL02 upgrade delta and
    `TestStripeAuthorityPlatformPolicies`.
  - `commerce_payment_registry_writer` reads (`platform_*_read`) and only locks (`platform_*_lock`, WITH CHECK false) the platform's own
    merchant_accounts / account_credentials / account_qualifications / stripe_live_approvals rows across tenants. Locks are needed
    because `SELECT ... FOR SHARE` also applies the UPDATE policy. It also reads `pricing.policy_versions(country)` and executes
    `identity.resolve_access`. Pinned in `TestStripeAuthorityPlatformPolicies`.
  - `commerce_runtime` INSERT on `merchant_accounts` is a column-list grant that excludes `platform_connection_id`.
- **Lock order (P1-1).** Enable takes: allowlist FOR SHARE -> platform account FOR SHARE -> active approval FOR SHARE (LIVE) -> platform
  qualification FOR SHARE -> the store's own rows. `revoke_stripe_live` locks the approval FOR UPDATE, so an enable and a revoke
  serialise in either order. A derived REAL_LIVE qualification cannot survive the platform revoke.
- **LIVE re-arm (P1-2).** The `qualify` fan-out on LIVE mirrors nothing unless the platform is OPEN or CLOSED (approval active and
  canary verified). After revoke -> new approval -> qualify the platform stays DESIGNATED, so no store is re-armed with the old cap
  before the new canary; a merchant replay of the enable re-derives afterwards under the new approval. Fan-out `qualify` mirrors the
  proof class of each store's current head only.
- **Rotation and blocked stores.** A rotation copies the new credential head to blocked stores too (their refunds need the current key,
  §4.3); only qualification and card heads skip them. `stripe_registrar_credential` refuses a derived connection.
- **`platform-disallow` on an enrolled store** also runs the block path (new sales stop, refunds keep working); re-allow does not
  unblock (use `platform-unblock`).
- **Dispatch-time kill switch (frozen behaviour, ruling 9).** A kill switch (block, disable, revoke, close) stops NEW starts. A Checkout
  Session already created is not force-expired and the worker keeps reconciling it (I16). Only an attempt not yet created at dispatch
  re-reads the head at start.
- **Deploy order (ruling 10).** Run `cmd/migrate` first, then roll ALL stripe-ingress pods together. `payments.stripe_webhook_prepare`
  changed from 18 to 19 arguments, so an old pod that restarts after the migration fails the exact-signature authority check until it is
  replaced by the new build.
- Pins to update: `tests/foundation/stripe_authority_test.go`, `stripe_live_schema_test.go` (SL02 column-privilege
  matrix), `worker_authority_split_test.go` (WAS02: five worker logins have no EXECUTE on the new functions) and
  `merchant_orders_v2_acl_test.go` (runtime EXECUTE list).

## 4. Attribution: every money fact belongs to exactly one tenant, store and order

### 4.1 Create parameters (stripe-psp §5.4 key set +3)

```
metadata[lc_store]=<store uuid>                       payment_intent_data[metadata][lc_store]=<store uuid>
payment_intent_data[statement_descriptor_suffix]=<enrollment suffix>   -- only when the attempt's connection is derived AND a suffix is set
```

`lc_store` is sent for **every** Stripe attempt, primary or derived, so there is one key set. These keys are for Dashboard
readability and for cross-checks. **They are never authority.** `params.go` allowed-key set and SP03 vectors are
updated. Each new key gets an `ErrInvalid` vector in SP03.

### 4.2 Webhook mapping (prepare)

The endpoint resolves `(environment, account, profile)` exactly as today. The endpoint's connection is `E`.
- **Candidate attempts.** Today a candidate must be in the endpoint's tenant and store. Now it may be any Stripe attempt
  `a` whose connection is `E`, **or** a derived connection with `platform_connection_id = E`. The mapping keys are
  unchanged: pinned `stripe_sessions.session_id`; `client_reference_id = metadata.lc_attempt`; refund id via
  `payments.stripe_refunds`.
- **Scope.** The receipt's and signal's `tenant_id`/`store_id` are **`a`'s**, read from the attempt row. The definer sets
  the scope GUC from `a` after mapping. RLS policies of the definer owner on receipts and signals admit that scope.
  Metadata never selects a row.
- **Cross-check.** If `metadata.lc_store` is present and ≠ `a.store_id`, the result is `QUARANTINED reference_mismatch`.
  A missing `lc_store` (pre-amendment sessions) is accepted.
- **Dedupe.** Unchanged: `(endpoint_id,event_id)`, so one platform endpoint deduplicates across stores.

Because attempts are always created in the store's own scope (start), an event can only ever reach the store that
created the session.

### 4.3 Refunds (stripe-refund unchanged except as named)

- **Who.** The merchant with `payments:refund` on the store (RD10) refunds that store's attempts only. The request path
  is already store-scoped (`request_stripe_refund` locks the order in the caller's store).
- **Capacity.** RD3 per attempt: `held + amount ≤ captured`. So a store can never refund more than it captured, on any
  order. There is **no** cross-store netting: a store's refund is not limited by, and never draws on, another store's
  captures.
- **When.** Refunds keep working after disable, block, suspend, platform CLOSED or REVOKED (LD7).
- **Store balance.** A refund after a payout makes that store's next statement negative (carried forward, §6.2). The
  platform's Stripe balance funds the refund. That is the platform's risk, listed in §9.

### 4.4 Settlement attribution (W4-S2, §6)

- **Charge** balance transaction → expanded `source` charge → `payment_intent` → `payments.stripe_sessions.payment_intent_id`
  → attempt → (tenant, store, order).
- **Refund** balance transaction → `source` refund id → `payments.stripe_refunds` → attempt.
- **Dispute** balance transaction → `source` dispute → `payment_intent` → attempt.
- **Only attempts whose connection is the platform connection or derived from it are eligible.**
- Anything unmapped goes to `payments.settlement_unattributed`. It is never guessed and never assigned by metadata.

## 5. Buyer disclosure, descriptor, privacy

- **Hosted view.** `checkout.hosted_payment_view_v2` (Stripe branch) returns `collector:{display_name,
  descriptor_preview}` for derived connections, and `null` for primary ones. `descriptor_preview` =
  `descriptor_display` + (`'* '` + suffix when set).
- **Storefront copy** (W4-U1, Codex, three languages). It is shown above the Stripe button and on the order page:
  - zh-TW: 「本筆信用卡款項由 {display_name} 代 {store_name} 收取，信用卡帳單顯示「{descriptor_preview}」。」
  - zh-CN: 「本笔信用卡款项由 {display_name} 代 {store_name} 收取，信用卡账单显示「{descriptor_preview}」。」
  - en: "Card payment collected by {display_name} on behalf of {store_name}. Your card statement shows "{descriptor_preview}"."
  - The Stripe hosted page itself shows the platform's business name (PF-F3).
- **Merchant terms.** Enabling shows `terms_version` and the plain statement: 「款項由平台代收，按結算週期以銀行轉帳撥付；
  退款與爭議款會從你的結算中扣除」. The legal text is owner-supplied (OQ-4).
- **Privacy is unchanged.** No `customer`/`customer_email` is sent. Metadata holds UUIDs only. Ledger rows and CSV
  hold order numbers, ids, amounts and dates only: no buyer name, phone, email or address, no card data, no session URL
  (I11).

## 6. Per-store settlement ledger (W4-S2, `migrations/0138_platform_settlement.sql`)

### 6.1 Tables (FORCE RLS, PUBLIC revoked, owner `commerce_payment_registry_writer`, append-only except set-once columns)

```
payments.settlement_lines
 balance_txn_id text PRIMARY KEY CHECK(balance_txn_id ~ '^txn_[A-Za-z0-9]{1,255}$'),
 tenant_id, store_id uuid NOT NULL, environment text NOT NULL, attempt_id uuid NOT NULL,
 refund_id uuid NULL, dispute_id text NULL CHECK(dispute_id ~ '^dp_[A-Za-z0-9]{1,255}$'),
 kind text NOT NULL CHECK(kind IN ('CHARGE','REFUND','REFUND_FAILURE','DISPUTE','DISPUTE_REVERSAL')),
 store_currency text NOT NULL, store_minor bigint NOT NULL,        -- signed, presentment = store currency (TWD)
 settle_currency text NOT NULL, settle_amount bigint NOT NULL, settle_fee bigint NOT NULL, settle_net bigint NOT NULL,
 fee_store_minor bigint NOT NULL,                                   -- §6.3 conversion, signed (fee is a cost: ≤ 0 normally)
 txn_created_at timestamptz NOT NULL, synced_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 mismatch text NULL CHECK(mismatch IN ('amount','currency','no_fact')), -- cross-check vs payments.facts / refund_facts
 statement_id uuid NULL,                                            -- set once at close
 FOREIGN KEY(tenant_id,store_id,attempt_id) REFERENCES integration.operations(tenant_id,store_id,id),
 CHECK(settle_amount - settle_fee = settle_net)                     -- Stripe's own identity; violation = reject row

payments.settlement_unattributed
 balance_txn_id text PRIMARY KEY, environment, type text, settle_currency, settle_amount, settle_fee,
 txn_created_at, reason text CHECK(reason IN ('unmapped_source','foreign_connection','unsupported_type')), synced_at

payments.settlement_unattributed_resolutions                        -- 0163 (S2-OPEN-1); append-only: no UPDATE/DELETE grant to anyone
 balance_txn_id text PRIMARY KEY REFERENCES payments.settlement_unattributed(balance_txn_id),
 tenant_id, store_id uuid NOT NULL,                                  -- the platform store's scope; FORCE RLS like §6.1
 environment text NOT NULL,
 resolution text NOT NULL CHECK(resolution IN ('not_store_revenue','assigned_to_store')),
 target_tenant_id uuid NULL, target_store_id uuid NULL,              -- both set iff resolution='assigned_to_store', else both NULL
 CHECK((resolution='assigned_to_store')=(target_tenant_id IS NOT NULL AND target_store_id IS NOT NULL)),
 CHECK((target_tenant_id IS NULL)=(target_store_id IS NULL)),
 FOREIGN KEY(target_tenant_id,target_store_id) REFERENCES control.stores(tenant_id,id),
 operator text NOT NULL CHECK(operator ~ '^[A-Za-z0-9._:@-]{2,64}$'),
 ticket text NOT NULL CHECK(ticket ~ '^[A-Za-z0-9._:-]{8,128}$'),
 note text NOT NULL CHECK(char_length(note) BETWEEN 1 AND 500 AND btrim(note)<>'' AND note !~ '[[:cntrl:]]'),
 resolved_at timestamptz NOT NULL DEFAULT clock_timestamp()

payments.settlement_statements
 id uuid PRIMARY KEY, tenant_id, store_id, environment, period_start date, period_end date,  -- [start,end) Asia/Taipei
 currency text, captured_minor, refunded_minor, dispute_minor, stripe_fee_minor, platform_fee_bps integer,
 platform_fee_minor, carried_in_minor, net_payable_minor bigint NOT NULL, line_count integer, lines_sha256 bytea,
 closed_at timestamptz NOT NULL, closed_by text NOT NULL,                          -- operator name
 payout_ref text NULL CHECK(payout_ref ~ '^[A-Za-z0-9._:/-]{4,80}$'), payout_minor bigint NULL, paid_at timestamptz NULL,
 paid_recorded_by text NULL,                                                         -- payout triple set once
 UNIQUE(tenant_id,store_id,environment,period_start),
 CHECK(net_payable_minor = captured_minor - refunded_minor - dispute_minor + stripe_fee_minor - platform_fee_minor + carried_in_minor),
 CHECK((payout_ref IS NULL)=(paid_at IS NULL) AND (payout_ref IS NULL)=(payout_minor IS NULL)),
 CHECK(payout_minor IS NULL OR (net_payable_minor > 0 AND payout_minor = net_payable_minor))
```

### 6.2 Rules

- **Money facts come only from Stripe API reads.** These are the balance-transaction list read by the sync CLI and the
  existing worker facts. Never the client, never webhook payloads, never metadata.
- **Idempotent.** Re-syncing a window inserts nothing new. The same `balance_txn_id` with different content raises
  PT409 and opens a `settlement_unattributed` row with reason `unsupported_type`. Close is idempotent per
  `(store, period)`: a replay returns the stored statement, and different totals raise PT409.
- **Period.** Default `WEEKLY`, Monday 00:00 to the next Monday 00:00 `Asia/Taipei` (OQ-2). A statement for a period
  closes only when:
  - `period_end + 72 h ≤ now`, which covers late Stripe balance transactions;
  - a sync covered `[period_start − 7 d, period_end)`;
  - the store's previous period is closed, or that store has no earlier line.
- **What a statement contains.** Every line of the store with `statement_id IS NULL AND txn_created_at < period_end`.
  Lines that arrive late roll into the next statement, never into a closed one.
- **Totals**, all in the store currency:
  - `captured` = Σ CHARGE.
  - `refunded` = −Σ REFUND − Σ REFUND_FAILURE (a failure re-credits).
  - `dispute` = −Σ (DISPUTE + DISPUTE_REVERSAL).
  - `stripe_fee` = Σ `fee_store_minor`. This is negative: Stripe fees are passed through by default (OQ-3).
  - `platform_fee` = `round_half_up((captured − refunded) × bps / 10000)` to the currency step (TWD 100). It is 0 while
    billing is off.
  - `carried_in` = the previous statement's `net_payable` if negative, else 0.
- **Close refuses** with `settlement_mismatch` while any line in scope has `mismatch IS NOT NULL`. The operator resolves
  it outside this contract (escalation, NOT_IMPLEMENTED). It refuses the **whole** environment's close while any
  `settlement_unattributed` row in the window has reason `unmapped_source` **and no line and no §6.6 resolution row**, because an
  unmapped charge might belong to any store. The only clearing path is the §6.6 resolve step (0163); deleting or
  editing the unattributed row stays a red-line action.
- **Reconcilable.** Each sync prints, for the window, Σ `settle_net` of lines + Σ unattributed net, next to Stripe's own
  window total of the same list. The operator compares them with the payout report in the Dashboard. A difference is
  printed as counts and ids only.
- **No cross-store leakage.** Statements and lines are RLS-scoped by tenant/store. The merchant reader takes the store
  from the token. CSV export is per statement.

### 6.3 Fee conversion (settlement HKD → store TWD)

For CHARGE:
`fee_store_minor = −step × round_half_up(settle_fee × store_minor / settle_amount / step)`, with `step = 100` for TWD
(whole NT$). This uses the charge's own conversion ratio.

DISPUTE and DISPUTE_REVERSAL use the **original charge's** ratio (`store_minor/settle_amount` of the CHARGE line of the
same attempt). If that line is missing, the row has `mismatch='no_fact'`.

REFUND fee is normally 0, and it is converted the same way if not.

A null or absent `exchange_rate`/`fee`/`source` fails closed: the row goes to `settlement_unattributed` with reason
`unsupported_type`. FX gain or loss between the charge's ratio and the platform's actual bank conversion stays with the
platform (OQ-3).

### 6.4 SQL entry points (0138)

| Function | Caller | Contract |
| --- | --- | --- |
| `payments.record_settlement_lines(tenant, store, principal, environment text, lines jsonb) RETURNS jsonb` | registrar (operator CLI, scoped to the platform store) | `lines` is an array (≤ 500) of the exact projection keys in §6.5. Attribution happens in SQL (§4.4), never by input tenant or store. A row with a §6.6 resolution is final: counted as a duplicate, never attributed (0163). The function sets the target scope per line, inserts lines or unattributed rows and cross-checks against `payments.facts` (CAPTURED amount/currency) and `refund_facts` (SUCCEEDED). Returns counts `{inserted, duplicate, unattributed, mismatch}`. Audit `stripe.settlement.sync` (platform scope, counts only). |
| `payments.close_settlement(tenant, store, principal, environment, period_start date, operator text, target_store uuid NULL) RETURNS jsonb` | registrar | Closes one store, or every store with unassigned lines when `target_store` is NULL, applying the §6.2 rules. Sets `statement_id` on the lines. Returns statement ids and totals, plus `operator_notes` (0163): one entry per `assigned_to_store` resolution (§6.6) whose transaction falls in the closed period `[period_start, period_end)` (never reprinted by a later period's close), each with `period_start`, `type` and the SIGNED `settle_amount`/`settle_net`/`settle_currency`, printed by the CLI. The `settlement_unattributed` refusal carries up to 20 blocking `balance_txn_id`s (oldest first) as its DETAIL; the CLI prints them. Audit `stripe.settlement.close` per statement (platform scope; the target store is in the details). |
| `payments.record_settlement_resolution(tenant, store, principal, environment text, balance_txn text, resolution text, operator text, ticket text, note text, target_tenant uuid NULL, target_store uuid NULL) RETURNS jsonb` (0163) | registrar | §6.6. Appends one resolution row for one `unmapped_source` row that has no settlement line (`already_attributed` otherwise); never touches the unattributed row itself. Idempotent: an identical replay returns the stored row (`replayed:true`); a different payload for the same `balance_txn_id` raises PT409. Audit `stripe.settlement.resolve` (platform scope; ticket in the details). |
| `payments.record_settlement_payout(tenant, store, principal, statement uuid, payout_ref text, payout_minor bigint, paid_at timestamptz, operator text) RETURNS timestamptz` | registrar | Sets the payout triple once. It requires `net_payable > 0` and `payout_minor = net_payable`. `paid_at ≤ now`. An identical replay returns the stored time; anything else raises PT409. Audit `stripe.settlement.payout`. |
| `payments.read_store_settlements(p_token bytea, p_store uuid, p_limit int, p_before date) RETURNS jsonb` | `commerce_runtime` | Requires `billing:manage`. Returns statements (≤ 52) with totals and payout status. It also returns, per statement, lines (order number, kind, `store_minor`, `fee_store_minor`, date). Never returned: settlement-currency amounts, txn ids of other stores, the platform's unattributed rows. |

### 6.5 Wire and CLI (W4-S2)

- **Wire call.** `stripe.Client.ListBalanceTransactions(ctx, createdGTE, createdLT, startingAfter)` is
  `GET /v1/balance_transactions?created[gte]&created[lt]&limit=100&expand[]=data.source`.
  - Projection, exact keys: `ID, Type, Amount, Fee, Net, Currency, ExchangeRate?, Created, SourceID, SourceObject,
    ChargePaymentIntent?, RefundID?, DisputeID?, DisputePaymentIntent?, DisputeAmount?, DisputeCurrency?`.
  - Bounds: ≤ 50 pages per run, otherwise `ErrUncertain` and nothing is written. The window is ≤ 8 days.
  - The expanded source is parsed for the listed ids only. No billing details, no email, no card fields.
- **RAK permission.** The live RAK needs *Balance transactions read*. The SL08 list is extended, and the owner adds it
  in the Dashboard.
- **CLI.** `cmd/stripe-admin` gains the following subcommands, all through `ops-admin.sh stripe-admin …` with the
  allowlist extended. None takes a secret input. Each uses the **stored** platform credential via
  `stripe_registrar_credential`, as `live-approve` does.
  - `settlement-sync --environment --from --to`
  - `settlement-close --environment --period-start [--store]`
  - `settlement-export --statement --out <path>`: CSV, UTF-8 BOM, `guardFormula` reused from
    `merchantorders/export.go`, columns `period, order_number, kind, amount, stripe_fee, date` plus a totals block,
    file mode 0600.
  - `settlement-payout --statement --payout-ref --amount --paid-at`
  - `settlement-resolve --environment --balance-txn --resolution [--target-tenant --target-store] --note` (0163): the
    §6.6 step. SQL-only — it never calls Stripe and never reads the LIVE key pair (like `settlement-close`); `--ticket`
    is required. One JSON result line on success.
  - `--operator` and `--ticket` on every subcommand.
- **Merchant route.** `GET /v1/admin/stores/{store_id}/settlements[?before=]`. The admin BFF mirrors it, and the
  read-only UI is in W4-U1.

### 6.6 Resolving unmapped rows (0163, S2-OPEN-1)

A charge the system never created lands in `payments.settlement_unattributed` with reason `unmapped_source` and blocks
the whole environment's close (§6.2). Before 0163 the only remedy was a red-line superuser DELETE. The resolve step
replaces it, **append-only**:

- **One row per transaction.** `settlement_unattributed_resolutions` (§6.1) keys on the unattributed `balance_txn_id`
  (FK + UNIQUE). Nothing ever UPDATEs or DELETEs an unattributed row or a resolution row; no role holds those grants,
  and a `BEFORE UPDATE OR DELETE OR TRUNCATE` trigger refuses it even for the table owner (PT409
  `settlement_resolution_immutable`; the FK keeps the resolved unattributed row from being deleted as well). A wrong
  resolution is never edited: it escalates to the owner (§9).
- **Two resolutions.** `not_store_revenue` (Stripe fee/adjustment/unknown money that is not any store's sale) needs
  nothing else. `assigned_to_store` says the money belongs to a target store; the target pair is required, the store
  must exist, and it must have used the platform account (an enrollment for the environment, or the platform store
  itself).
- **v1 does NOT move money (binding).** An `assigned_to_store` resolution creates no settlement line and changes no
  statement total. It is recorded and printed by `settlement-close` as an `operator_notes` entry (§6.4) so the owner
  settles the **signed** amount with the store out of band: the note carries the period, the transaction type and the
  signed settlement-currency amount and net (a negative row is money that left the platform account: the owner recovers
  it from the store, not pays it). A close prints only the notes of its own period. Moving the money into a later
  statement needs an amendment.
- **`not_store_revenue` on a negative row.** A negative (dispute-shaped) row resolved as `not_store_revenue` means the
  platform absorbs it. If a store should bear it, use `assigned_to_store`: the owner recovers it from that store using
  the signed amount in the note.
- **Refusals.** PT409 when the row does not exist in the caller's environment (`unattributed_unavailable`), its reason is
  `foreign_connection`/`unsupported_type` (`unresolvable_reason`: those never block close and are not attribution
  questions), the row's weekly period (§6.2, Asia/Taipei) is already closed for **any** store, or, for `assigned_to_store` only, any statement exists for a LATER period (`period_already_closed`: a close prints only its own period's notes, so the note of such a late row would be printed by no close; `not_store_revenue` carries no payout and stays resolvable),
  the row already has a settlement line (`already_attributed`: 0150 keeps the unattributed row when a later sync attributes it, and
  resolving it would credit the store's statement AND tell the owner to pay again), or, for `assigned_to_store`, the target store is
  unknown, foreign or never enrolled (`unknown_target_store`). Structural input errors (bad
  txn id, resolution value, operator/ticket shape, a blank, control-character or longer-than-500 note, target pairing)
  raise 22023 before any read.
- **Idempotent (I02/I06).** Same `balance_txn_id` + identical payload → the stored row is returned with
  `replayed:true`, no second row, no second audit entry. Different payload → PT409 `resolution_conflict`.
- **Same lock, same scope.** The function is `SECURITY DEFINER`, owner `commerce_payment_registry_writer`, EXECUTE
  to `commerce_payment_registrar` only, `search_path=pg_catalog`, takes the same per-environment advisory lock as
  sync/close, and requires the platform-store registrar scope (`integration.require_stripe_registrar_scope`). The
  0150 close check is patched **in place** (0163) to skip **resolved** rows only — a resolved row no longer refuses close,
  an unresolved one still does, and totals are unchanged because resolutions are not lines.
- **A resolution is final (sync guard).** `record_settlement_lines` retries an unattributed row on every identical
  re-sync (§4.4: a session or refund may have been recorded since). 0163 patches it in place so a row that has a
  resolution is counted as a duplicate and **never attributed afterwards**. Without it an `assigned_to_store` note
  would make the owner pay the store out of band **and** the new line would pay it again from the statement, and a
  `not_store_revenue` row would leak into a store's totals. Rows without a resolution behave exactly as in 0150.
- **CLI.** `settlement-resolve` (§6.5), allowlisted in `ops-admin.sh`; it never calls Stripe and never reads the LIVE
  key pair. Audit action `stripe.settlement.resolve` in the same transaction.

## 7. Kill switches

| Scope | Command | Effect | Never affected |
| --- | --- | --- | --- |
| One store, merchant | admin toggle → `set_platform_stripe(enabled=false)` | method disabled; new starts PT409 | reconcile, refunds, ledger |
| One store, operator | `ops-admin.sh stripe-admin platform-block --store --ref` (no flag+ref pair needed) | derived qualification revoked plus method disabled. The merchant cannot re-enable until `platform-unblock`. | same |
| One store or tenant, suspended | OPS-01B `store-suspend` | buyer orders already refused by the OPS-01B guards. Enable is refused while inactive. Disable is still allowed. Resume does **not** re-enable: the enrollment state is whatever it was before. | same |
| New enrollments | `stripe-admin platform-close` (`enrollment_open=false`) | no new or re-enabled stores. Existing ones keep selling. | same |
| All stores, hard | `live-revoke` on the platform approval | fan-out revokes every derived qualification (§3.4). Re-opening needs a new approval, qualify and canary on the platform store. | same |
| All stores, deploy | `LC_STRIPE_CHECKOUT_ENABLED=0` (unchanged) | Stripe is removed from the hosted service | same |
| Key compromise | stripe-live-enable §7 row on the **platform** connection. `rotate` fans out new derived credential versions; until re-qualify, starts are blocked. | | |

## 8. Owner checklist and LIVE opening (added to stripe-live-enable §9 for the platform approval)

`live-approve` on the platform connection additionally requires these attestation codes:

| Code | Item |
| --- | --- |
| `platform_stripe_terms` | The owner has confirmed with Stripe (support ticket id recorded in the `approval_ref` message) that the HK account may collect card payments as business of record for third-party Taiwan stores without Connect (OQ-1). |
| `merchant_terms` | Merchant terms text `terms_version` exists and is published (OQ-4). |
| `tax_invoice` | The owner has decided who issues the Taiwan e-invoice (統一發票) for platform-collected sales, and how (OQ-5). |
| `payout_ops` | `payouts_enabled=true` (PF-F1; already enforced by LD8 readiness), and the owner has a bank route to pay TWD to Taiwan merchants (OQ-6). |

The canary (stripe-live-enable §10) runs on the platform store in **TWD**. `set_stripe_platform_open(true)` is refused
until the canary is verified. SANDBOX staging opens without those codes, since there is no approval in SANDBOX.

**LIVE settlement runbook step (0163).** Before the first LIVE weekly close — and afterwards whenever a close refuses
with `settlement_unattributed` — the operator runs `ops-admin.sh stripe-admin settlement-resolve` (§6.6) for every
`unmapped_source` row that the close refusal names and that has no line (the refusal lists up to 20 ids; rerun the close for
more), recording operator name and support ticket in each resolution. A replayed or per-store close reprints that period's notes by design: settle each `balance_txn_id` exactly once, regardless of reprints.
**No buyer PII in the note** (no name, e-mail, phone,
address, card data): it is printed in CLI output and close notes, and the details belong in the ticket. Deleting or editing
the row by hand stays a red-line action requiring owner approval; the resolve step is the sanctioned path.

## 9. Known limits / NOT_RUN

- **Evidence:** DESIGN. PF01–PF14 are NOT_RUN. LIVE is owner-run only. Agents and CI never touch LIVE.
- **One currency per platform approval:** TWD (one active approval per connection, 0077). An HKD or multi-currency
  platform needs an amendment.
- **Disputes are not ingested as webhooks** (event allowlist unchanged). They appear in the ledger at the next sync. The
  owner, as business of record, answers them in the Stripe Dashboard and must collect evidence from the merchant
  (OQ-7).
- **Platform credit risk:** refunds, disputes and negative store balances are funded from the platform Stripe balance.
  There is no reserve or holdback in v1 (OQ-3).
- **Fan-out** is bounded to 2000 enrollments per transaction.
- **Settlement is CLI-only.** There is no automated payout and no bank API. The payout reference is operator-entered,
  and the bank transfer is a red-line action done by the owner.
- **`assigned_to_store` does not move money in v1** (§6.6): the resolution is recorded and printed as an operator
  note; the owner settles the signed amount with the store out of band. A ledger-visible assignment needs an amendment.
- **A resolution (and its note) is never edited or erased by this tool.** Erasing or correcting one is a red-line owner
  action (SQL with a ticket), including a note that was written with personal data by mistake.
- **A late `unmapped_source` row in an already-closed period cannot be resolved** (§6.6 `period_already_closed`):
  it fails closed, keeps refusing every later close for the environment, and escalates to the owner as a red-line
  SQL fix with a ticket. This is accepted for v1 (rows normally surface within the +72 h close delay).
  The same refusal applies to a late `assigned_to_store` row whose week precedes any closed statement.
  *Follow-up amendment (open):* closed-week rows resolvable as `not_store_revenue`, and a late `assigned_to_store` row for a
  closed week.
- **Balance-transaction field names are UNVERIFIED** until W4-S2's WebFetch (PF-F5).
- **Card statement text is UNKNOWN** until the canary reads `calculated_statement_descriptor` (L7).

## 10. Gates (tiers as stripe-psp §14)

| Gate | Unit | Test | Tier | Proves |
| --- | --- | --- | --- | --- |
| PF01 | S1 | `TestPlatformStripePF01Schema` | REAL_PG | 0137 fresh and populated-latest; derived account FK/trigger (wrong account, wrong env, not the designated connection, immutable pointer); credential byte-equality trigger; qualification CHECK both ways; unique-index split (two primary rows on one account still rejected); FORCE RLS; ACL matrix |
| PF02 | S1 | `TestPlatformStripePF02Enable` | REAL_PG | each refusal: no `billing:manage`, NONE, DESIGNATED, CLOSED, REVOKED, blocked, suspended, own primary connection, currency, stale terms, suffix too long, CAS; first enable creates exactly the five row kinds; replay; disable always allowed |
| PF03 | S1 | `TestPlatformStripePF03Fanout` | REAL_PG | rotate, qualify and revoke on the platform each update every enrolled store in one transaction; a blocked store is skipped; the 2001st enrollment is refused; a derived connection refuses rotate, qualify, approve and webhook |
| PF04 | S1 | `TestPlatformStripePF04Start` | MOCK + REAL_PG | two stores on one platform account: each attempt carries its own store and derived connection; `create_params` has `lc_store` plus a suffix only for derived; the worker opens the copied envelope with the `aad_*` scope; a primary-store attempt is unchanged |
| PF05 | S1 | `TestPlatformStripePF05Webhook` | HTTP_PG | one platform endpoint: store A's session → receipt in A's scope, store B's → B's; `lc_store` forged to B on A's session → `reference_mismatch`; a session of an unrelated primary account on the same endpoint → `unknown_session`; dedupe across stores; refund event mapped to the refund's store |
| PF06 | S1 | `TestPlatformStripePF06Refund` | MOCK + REAL_PG | store A cannot refund B's order (not found); RD3 cap per attempt; refund allowed after disable, block, platform close and revoke |
| PF07 | S1 | `TestPlatformStripePF07Kill` | REAL_PG + process | each §7 row: new start PT409 while an in-flight attempt reconciles; block then merchant re-enable refused; unblock then re-enable re-derives |
| PF08 | S1 | `TestPlatformStripePF08Routes` | HTTP (DB-free router) + HTTP_PG | `GET/PUT /v1/admin/stores/{store_id}/payments/card`; buyer hosted view `collector` field; no account id in any response |
| PF09 | S2 | `TestPlatformSettlementPF09Schema` | REAL_PG | 0138 objects, checks (net identity, payout rule), RLS, ACL |
| PF10 | S2 | `TestPlatformSettlementPF10Sync` | MOCK + REAL_PG | fixture balance list (charge, refund, refund_failure, dispute, reversal, payout, stripe_fee, unknown source, other connection) → correct store lines and unattributed rows; replay inserts 0; changed content PT409; fact mismatch flagged; missing `exchange_rate`/`source` fail closed; the page cap raises `ErrUncertain` and writes nothing |
| PF11 | S2 | `TestPlatformSettlementPF11Close` | REAL_PG | totals and §6.3 rounding vectors (TWD step 100, half-up, negative fee); late line rolls into the next period; mismatch or unmapped blocks; carry-forward of a negative balance; platform fee bps; idempotent close |
| PF12 | S2 | `TestPlatformSettlementPF12Payout` | REAL_PG | payout set-once; amount ≠ net refused; non-positive net refused; replay |
| PF13 | S2 | `TestPlatformSettlementPF13Read` | HTTP_PG | merchant sees only their store; `billing:manage` required; no settlement-currency or other-store data; CSV has no PII columns and formula guards hold |
| PF14 | S1+S2 | regression | CI | SP01–SP21, RF01–RF12 and SL01–SL09 unchanged and green, SP03 with the new key vectors, plus `--stripe-browser` and `--browser-payment` |
| PF15 | S2-OPEN-1 | `TestPlatformSettlementPF15Resolve` | REAL_PG + MOCK | 0163 §6.6: `not_store_revenue` resolve clears the close refusal with totals unchanged; identical replay returns the stored row; different payload PT409; `foreign_connection`/`unsupported_type` refused; `assigned_to_store` without target / unknown / foreign store refused; already-closed period refused; blank note refused; runtime and merchant roles hold no privilege on the resolutions table and not even the owner can UPDATE/DELETE/TRUNCATE a resolution; a resolved row is never attributed by a later sync (an unresolved control row still is); CLI e2e (one JSON line + audit row); PF11 clears its blocking row through the resolve path, no owner-pool DELETE |
| PF-SBX | S1 | SANDBOX | owner test keys | two SANDBOX stores enrolled on the platform's `sk_test`/`rk_test` account; one paid order and one refund each; ledger sync shows two attributed charges. NOT_RUN without keys. |
| PF-LIVE | — | owner | LIVE | after the platform canary: the first non-developer Taiwan merchant's real order and refund (I17). NOT_RUN. |

## 11. Owner questions (each has a default; the ones marked blocking stop LIVE enrollment, not development)

| # | Question | Default |
| --- | --- | --- |
| OQ-1 (blocking) | Does Stripe allow this HK account to collect for third-party Taiwan stores without Connect (PF-F6)? | Owner asks Stripe support before LIVE opening; the ticket id goes into the approval message (`platform_stripe_terms`). |
| OQ-2 | Settlement cycle. | Weekly, Monday to Monday Asia/Taipei, closed ≥ 72 h after period end, paid within 5 business days. |
| OQ-3 | Platform fee %, and who bears Stripe fees, cross-border/FX costs, disputes and transfer fees. | Platform fee 0% while billing is off. Stripe fees are passed through at the charge's own conversion ratio. FX difference and bank transfer fees are borne by the platform. Disputes are deducted from the store. No reserve. |
| OQ-4 (blocking) | Merchant terms (代收款 agreement) text and version. | Owner supplies the text. The version string is `pf-2026-10`. |
| OQ-5 (blocking) | Taiwan e-invoice (統一發票) and tax responsibility when the HK platform is business of record. | Merchant issues invoices for its own sales; platform invoices only its own fee. Owner to confirm with an accountant. |
| OQ-6 | Payout setup: when payouts are enabled on the HK account, and which bank route pays TWD to Taiwan merchants. | Owner completes Stripe payout setup (required for LD8 readiness) and names the bank route before the first statement. |
| OQ-7 | Dispute handling as business of record. | Owner responds in Stripe; the merchant must supply evidence within 5 days; the loss is deducted from the store. |
| OQ-8 | `display_name` and `descriptor_display` shown to buyers. | Platform brand on `xgdwm.com`; descriptor = the Stripe card prefix (LQ6). |
