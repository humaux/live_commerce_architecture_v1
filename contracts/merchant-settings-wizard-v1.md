# Merchant logistics/payments wizard v1

> Amended 2026-10-26 by `owner-choices-20261026-v1.md` (OC-AD03); ADR-20261026.

2026-09-24. Frozen after independent preflight; extends existing account/method HTTP contracts.
User-approved composition: `.impeccable/merchant-settings-brief.md`, comp A.
Operate mode; no new provider, schema, dependency or production change.

## Readiness gap and reuse

Fresh onboarding creates no market or delivery pricing policy. A wizard cannot
pretend configuration worked using a fixture market. Expose the existing pricing
commands and safe discovery; keep all existing payment/API-logistics enable gates.
No universal provider abstraction, new transaction engine or background worker.

## Frozen transport additions

All paths below follow `/v1/admin/stores/{store_id}` and the existing scoped
transaction, safe error, no-store/request ID, 5s deadline and strict 64KiB JSON.
Mutations require command key and exact path/body target equality. BFF is actual
identity-only for these new routes, with existing Origin/CSRF and store authority.

|Method/path|Input and output|Permission|
|---|---|---|
|GET `/markets`|existing `limit/cursor`; `Page[pricing.Market]`|`pricing:read`|
|POST `/markets`|`pricing.MarketInput`; `pricing.Market`|`pricing:write`|
|GET `/markets/{market}/countries/{country}/delivery-services`|`limit/cursor`; `Page[fulfillment.Service]`|`integration:read`|
|GET `/markets/{market}/countries/TW/payment-methods`|no query; `{items: payments.Method[], next_cursor:""}`; at most the existing five codes|`integration:read`|
|GET `/markets/{market}/countries/{country}/delivery-services/{code}/policy`|no query; current `pricing.Policy`, including disabled configuration|`pricing:read`|
|PUT same `/policy`|`pricing.PolicyInput`; current `pricing.Policy`|`pricing:write`|

Policy method must equal `delivery:` + path code, market and country must match.
Policy GET must not use `LockCurrent`, which hides disabled policies; read the
explicit current-head safe columns, never `configuration_ref` or principal data.
Existing `CreateMarket` enforces store currency; never infer currency from locale.
No public market activation endpoint is required for this wizard; inactive markets
are visible as inactive, not silently reactivated. No market/policy auto-seeding.

Each new read and pricing write checks permission before and after work, including
after rows.Close/lock waits and command replay, before exposing DTO/committing.
Only safe projections, explicit tenant/store filters plus existing RLS. Lists and
policy read verify the target market belongs to the scope; absent market => 404.
No read creates receipts, versions, audits, jobs or provider calls.

Market list uses existing UUID keyset with collection `markets`. Delivery list
orders by code using COLLATE "C", not mutable sort_order; code is the stable key.
Reuse the cursor envelope with collection `delivery-services`, parent=market UUID,
filter=two-letter country. Only this collection accepts one lowercase code key
(`^[a-z][a-z0-9_-]{0,39}$`); all old collections retain exact UUID validation.
Scope/parent/country/collection cursor crossover rejects. Both lists default 50,
max100, fetch limit+1; arrays never null. Payment list is bounded by its five-code
domain and sorted by code; all queries including trailing `?` reject there.

## Actual UI

Route `/{locale}/settings`, locales zh-CN/zh-TW/en. Settings navigation is real.
Reuse incumbent shell controls/CSS/Icon and authenticated store list; no inventory
permission prerequisite to view settings. SSR provides only selected authorized
store metadata, never account secrets. Explicit foreign store query fails closed.
Four steps: choose platform, connect account, configure methods, inspect status. **Superseded by the "Amendment ADR-20261026 AD-03" section at the end of this file for the merchant-facing quick start.**
PAYUNi uses the existing credential schema. Merchant-arranged manual delivery is
a separate honest choice, never described as the screenshot's provider自主模式.
Other providers are not invented as working integrations.

- Account step: choose existing account or create; environment, MerID, blank
  HashKey/HashIV. Rotation uses current credential version and exact same account.
  Saving remains CONFIGURED_UNVERIFIED; no verification, payment or shipping call.
- Methods step: select/create market using explicit code/name and store currency;
  country explicit (TW for PAYUNi). Load existing settings before edit. Configure
  all five existing PAYUNi method codes, three localized display names, visibility,
  sort order, min/max minor units, selected account/binding version. Enabled stays
  false with a clear reason; UI does not imply visibility makes payment available.
- Manual delivery: choose/create stable code and kind home/cvs_711/cvs_familymart;
  country explicit, CVS only TW. Save explicit country_flat shipping/tax policy
  (no guessed tax), then service referencing returned policy version. Two commands
  are clearly sequential, not advertised as atomic; a failed second save retains
  policy. Only an uncertain result permits retrying the identical service command;
  definite 409/422 requires reread, correction and a fresh command key.
  The market must be active and the saved policy explicitly enabled before normal
  service save (even a disabled service). Show this prerequisite; do not enable
  policies or reactivate markets silently. Policy enable, service enable and
  visibility are separate controls with explicit state readback.
  Every policy version requires an operator-entered, nonsecret configuration
  reference/reason (1–240 printable characters); never fabricate tax validation.
  Safe GET omits the prior reference, so editing asks for a new reference explicitly.
  No automatic carrier account, label purchase, tracking or guaranteed CVS delivery.
- Status step: reload server metadata; payment inspection remains diagnostic and
  uses current method/environment/currency/amount. Label configuration, provider
  qualification, enabled, visibility and runtime availability separately. Read
  zero/error/pending states truthfully; no generic green connected badge.
- Desktop follows A at 1586x992; 390px form precedes status, no horizontal overflow.
  One current primary action, proper labels/focus, error/request-ID and recovery.

## Session, drafts and uncertainty

Never keep credentials in URLs, storage, SSR props, telemetry or retry journals.
Clear secret inputs immediately after submission and on step/locale/store/session
change. A request may hold its secret body only for the in-flight fetch; no automatic
secret retries. Secret input re-entry is required for uncertain create/rotation.
Journal only nonsecret command identity/target/expected version for credential
commands; re-entry retries the original key and exact nonsecret context. A metadata
match alone is not proof that an unknown command committed. No fresh-key blind retry.

Nonsecret market/policy/method commands may journal exact bytes/key and retry them.
All journals are scoped to selected store and SHA256 of the issued CSRF session
boundary, never a token. Before every write reread the cookie; a changed/expired
session clears old form/journal and sends zero writes under the new session.
Use existing native storage/locks patterns; storage failure blocks mutation safely.
Locale changes preserve nonsecret drafts and pending identity; no currency change.
Stale requests cannot populate another store/session. Reads paginate explicitly.
No unbounded automatic paging or request retry loop.

## Acceptance

- W01: fresh actual signed-MOCK-IdP browser session creates real market/account,
  configures a payment draft, refreshes/reopens and reads it; inspect unavailable.
  Manual policy then service save and reload also use actual Go/isolated PG.
- W02: all three languages, 1586 desktop + 390 mobile, keyboard/focus, pending,
  validation, empty/error and preserved nonsecret language switch. Comp comparison,
  bounded visual QA and independent finish review required.
- W03: secrets cleared/absent storage & SSR; unknown secret requires re-entry same
  key, other-tab changed session produces zero writes, no stale read repaint.
  Unknown nonsecret command retries identical bytes/key; stale CAS stays a conflict.
- W04: real PG collection pagination/scope/country separation, disabled policy read,
  invalid input, no read effects, strict pricing permission and proven after-lock
  revocation rollback; current full race/vet and existing browser gates stay green.
- W05: author-independent review, dependency notes and evidence with explicit
  real provider/IdP/production NOT_RUN. This UI does not certify the full SaaS.

Ownership after preflight: Go author owns new domain discovery files, pagination
extension and HTTP registration; UI author owns settings component/copy/state/page
and shared frame extraction. Root owns this contract, BFF and independent tests.

## Amendment ADR-20261026 AD-03 (owner 2026-10-26: 商家設定要簡單，快速開店)

- The merchant-facing quick start (快速上手) has **three** steps: 店名 → 配送 → 第一個商品. No provider, key, merchant-id or payment step (payment is platform-collected, see stripe-platform-account AD-PF3). The four-step provider wizard above is not shown to merchants.
- After login the merchant lands on the dashboard (看板). The quick start is shown only for a newly created store; existing stores are never forced through it.
- "Newly created" = the store has no completed quick-start marker (server-side, per store, set when the 3 steps are done or dismissed). Evidence DESIGN; gate NOT_RUN.

### OC-05 server marker (0205, assigned 2026-10-28)

| Method/path | Input and output | Permission |
|---|---|---|
| GET /v1/admin/stores/{store_id}/quickstart | No query; `{completed, completed_at, steps:{store_name, shipping, first_product}}` | `store:read` |
| POST /v1/admin/stores/{store_id}/quickstart | Strict empty JSON object `{}`, required `Idempotency-Key`, no query; same state DTO | `integration:manage` |

This POST is an idempotent completion/dismissal, an exception to the older keyed
command receipts above: the store primary key preserves the first completion time,
even across different keys. Call it after the steps are done or on explicit dismissal.
It never asserts that unfinished steps were performed. GET makes no writes.
`completed_at` is nullable; `completed` means the persistent marker exists. Only the
database chooses the timestamp. The server derives `store_name` from nonblank
`control.stores.name`, `shipping` from at least one enabled current delivery-service
version, and `first_product` from at least one product (including archived products).
Derived steps remain truthful after dismissal; they are never client-supplied flags.
0205 atomically backfills every existing store as completed while serializing store
creation. New stores have no marker until POST. Runtime can only insert scoped
tenant/store columns; no timestamp overwrite, marker deletion or reset is exposed.
The normal authenticated scope and RLS apply, with permission rechecks before/after
reads and writes. No new definer or privileged role is needed for these scoped reads
and the append-only marker. CI tests: `tests/quickstart/quickstart_test.go`; independent
review and runtime evidence remain NOT_RUN until the PR checks are read.

## STORE-PROFILE-API — signed settings v2 (2026-10-29)

Authority: owner's STORE-PROFILE-API card and the signed settings-v2 request;
this amendment owns the profile transport and simplified delivery mapping.

### Store profile

| Method/path | Input/output | Permission |
|---|---|---|
| `GET /v1/admin/stores/{store_id}/profile` | `{name, contact_email, contact_phone, version}` | `store:read` |
| `PUT /v1/admin/stores/{store_id}/profile` | `{name, contact_email, contact_phone?, expected_version}`; same output | `integration:manage` |

Admin BFF: GET/PUT `/api/stores/{store}/profile`, actual signed session only,
Origin/CSRF protection on writes, no fixture identity. Tenant, principal and store
authority come exclusively from authentication; the selected path store must be
authorized. Body `tenant_id`, `store_id`, actor fields and any other unknown fields
are rejected. Host/forwarded-host/authority headers never select a tenant.

`name` is required, nonblank, 1–120 printable Unicode characters. `contact_email`
is required, a plain mailbox address up to 254 UTF-8 bytes, without a display name,
whitespace, quotes, angle brackets, backslashes or controls. `contact_phone` is
optional, up to 32 printable characters; omitted means clear to `""` on this
replacement PUT. `expected_version` is required and non-null, an integer from 0
through 9007199254740990. No trimming or silent rewriting of the submitted fields.
The returned version remains within JavaScript's safe-integer range.

PUT requires `Idempotency-Key` (existing 8–128 character command-key grammar).
An identical command by the same principal returns its original durable result,
even after newer versions exist. Changed input or another principal reusing the
key returns 409. A fresh-key stale `expected_version` returns 409; reread before
editing. After an uncertain result retry only the original body/key. One accepted
command atomically updates the profile, increments version, inserts one audit
without contact values and saves its original response in the same transaction.
Same-key concurrent retries consume one version; competing editors have one winner.

Migration **0209_store_profile.sql**, reserved by this card, adds contacts and
`profile_version` to `control.stores`; `control.stores.name` remains the single
authoritative name used by the store list and quick start. Legacy stores read as
version 0 with blank contacts; GET performs no writes or backfill. A restricted
NOLOGIN/NOBYPASSRLS writer exposes only authenticated profile CAS; runtime receives
no direct store UPDATE grant. Reauthorize after row/advisory lock waits and replay,
before exposing a result or committing. Revocation rolls back receipt/audit/profile.
No provider calls, jobs, email sends, account qualification or secret configuration.

Existing private no-store/generated request-ID/safe-error behavior applies.
Go uses strict JSON, a 64 KiB body limit and a five-second scope deadline. The BFF
uses its existing stricter 8 KiB command limit. Queries including bare `?` reject
with 422, unknown subpaths with 404 and unsupported methods with 405. Missing auth
is 401; forbidden permission is 403; inaccessible stores are non-disclosing 404.

### Five simplified delivery switches

Owner rule: 「不要配置太复杂」. **First launch ships MANUAL delivery only.**
Each switch resolves within the authenticated store's explicitly selected existing
market, country `TW`, and current service versions. It selects the one existing
MANUAL service with that `delivery_kind`; never treat the kind as a guessed code.

| Switch | Existing TW delivery_kind | Existing service and default policy |
|---|---|---|
| 宅配 | home | Unique current MANUAL home service; preserve its `code`, `policy_method`, `policy_version` and existing pricing facts |
| 7-ELEVEN | cvs_711 | Unique current MANUAL cvs_711 service; preserve its `code`, `policy_method`, `policy_version` and existing pricing facts |
| 全家 | cvs_familymart | Unique current MANUAL cvs_familymart service; preserve its `code`, `policy_method`, `policy_version` and existing pricing facts |
| 萊爾富 | cvs_hilife | Unique current MANUAL cvs_hilife service; preserve its `code`, `policy_method`, `policy_version` and existing pricing facts |
| OK 超商 | cvs_okmart | Unique current MANUAL cvs_okmart service; preserve its `code`, `policy_method`, `policy_version` and existing pricing facts |

Read using the existing delivery-services discovery and per-code service/policy
routes; the actual persisted `code` chooses the PUT path. A switch changes only
`enabled` via the existing full `ServiceInput` and current `expected_version`;
preserve mode MANUAL, names, visibility, order, COD choice, currency and policy
reference. Existing default policy means the service's already-persisted pricing
policy, not a newly invented flat fee/tax/free-shipping rule. The normal service
command also ensures its existing default warehouse allocation atomically.

No match, multiple matches, absent policy or an unmet existing activation gate
leaves the switch unavailable; report the prerequisite rather than auto-seeding a
market, changing policy, choosing an arbitrary duplicate, or enabling API mode.
Never fabricate carrier configuration, buy a label or infer a shipping/tax amount.
This amendment freezes mapping for the settings lane; it adds no second delivery
API or automatic migration of existing services.

### Quick start and acceptance

Reuse **#254 (OC-05)**, merged into this card's base (`313c2c36f`), and its existing
GET/POST `/quickstart` marker and derived steps. Profile writes update the same
name that OC-05 reads; they do not complete or dismiss the marker implicitly.
No replacement marker, duplicate backfill or alternate onboarding flag.

Gate: existing Foundation PG tests in `tests/foundation/store_profile_http_test.go`
cover persistence, original replay/CAS/concurrency, validation, auth-only scope,
no read effects and real post-lock revocation rollback. Existing Node BFF tests
cover signed sessions, scope, CSRF/Origin, exact routing and contract mapping.
CI RED run 40762550914/job 123078018073 at head `24cac517` reached four new BFF/
contract assertion failures (404 for absent profile route; missing amendment).
GREEN requires this PR's own implementation-head CI and independent review;
no local runtime or production acceptance is claimed by this amendment.
