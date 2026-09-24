# Merchant logistics/payments wizard v1

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
Four steps: choose platform, connect account, configure methods, inspect status.
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
