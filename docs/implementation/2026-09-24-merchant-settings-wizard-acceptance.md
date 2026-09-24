# Merchant setup wizard — A composition

Contract: [merchant-settings-wizard-v1](../../contracts/merchant-settings-wizard-v1.md).
Approved direction: horizontal steps plus right status column; mobile form first.
This record separates internal acceptance from actual provider qualification.

## Ownership and dependencies

|Unit|Owner / model / base|Scope|
|---|---|---|
|Go discovery|integration_worker, gpt-6-sol/high, f5c38b4; author 8afca25, integrated 2d8b240|`internal/{pricing,fulfillment,payments}/discovery.go`, pagination and HTTP registration|
|A UI|ui_worker, gpt-6-sol/high, f5c38b4; author de642ca/a323220/3bd4803/4060716, integrated 66dd82c/7e1205e/cdd10f0/64e7fa6|Settings wizard and shared incumbent shell; no BFF or schema ownership|
|Independent transport tests|root; 90fff3d|BFF allowlist and real PG tests|
|Independent review|settings_wizard_preflight, gpt-6-sol/high, read-only|Bounded Go/BFF/test review; no P0/P1/P2 found|
|Independent UI safety|settings_ui_safety_review, read-only; final root 1a5c64b|Four original P1 issues plus binding and explicit-discard gaps closed; no remaining P0/P1 at scope|

No new dependency, migration, provider worker or production setting was added.
The Go transport reuses `pricing.CreateMarket/SetPolicy`, scoped runtime role,
command receipts and current immutable heads. The UI calls the Next BFF; the BFF
uses the actual HttpOnly session and authorized store list, never browser-supplied
tenant, bearer or price authority. New settings pages do not require inventory access.

Market discovery uses UUID keyset paging. Delivery discovery uses stable service
code with `COLLATE "C"` and a collection-specific cursor validator; old collections
still require UUID keys. TW payment methods have a fixed five-code domain.
Policy reads deliberately include disabled configuration and exclude private
`configuration_ref` and principal data. Every edited version requires a new
explicit operator reference; it is not a certificate of tax/provider validity.

## Verified transport evidence

- `bash scripts/dev/test-local.sh`: actual exit 0, **308 top-level PASS / 0 FAIL /
  0 SKIP**, race and vet; foundation 148.480 s.
- Evidence: `/Volumes/data/output/live-commerce-account-intake-tests/wizard-pg-initial.log`.
  SHA256 `504f42680f35839da5801a9e34a14503b365e9b7d19fc9b5d3e87be9aeb90774`.
- Five independent `TestMerchantWizard*` tests cover persisted market commands,
  permanent replay/CAS, scope and country paging, disabled policy projection,
  invalid inputs, strict query handling, five payment methods and no read effects.
- Six observed PostgreSQL lock-wait revocations cover market and policy writes,
  permanent replay and reads. `pg_blocking_pids` establishes ordering before grant
  revocation; the response must be 403 and persisted state unchanged.
- Browser BFF **transport MOCK**: 1 PASS, exit 0; evidence
  `/Volumes/data/output/live-commerce-account-intake-tests/wizard-bff-mock-initial.log`.
  Forged authority, missing CSRF, cross-store access, pagination forwarding and
  exact-resource query rejection verified. This mock proves no business persistence.
- Initial production Next build, TypeScript and tagged foundation vet passed.

## UI and fresh setup gate

Status: **PASS_BOUNDED_LOCAL_WIZARD**. This is not production/provider acceptance.
`TestBrowserSettingsWizardRealChain` uses the same signed MOCK IdP / actual Next /
actual Go / isolated PG harness as the historical identity test, but explicitly
disables its legacy market seed. `settings-real.spec.ts` must create the market
through the displayed UI, not injected forms or successful-response stubs.

The gate checks account response loss followed by credential re-entry and exact
command replay, nonsecret response loss/exact retry, three locale routes, mobile
overflow, policy then manual service, reload/readback and old-session zero writes.
Separate PG postflight counts the account credentials, payment drafts, market
command, policy and service versions, with zero qualifications/provider operations.
Screenshots are taken only with empty/cleared credential fields; trace is off.

Final functional evidence at root `1a5c64b` plus an exact-viewport screenshot and
computed-contrast logging addition (no runtime change):

- `bash scripts/dev/test-local.sh --browser-identity`: exit 0, **3 PASS / 0 FAIL /
  0 SKIP**, foundation 13.822 s. Includes actual API process restart, historical
  BFF identity transport and the new real settings UI. External identity is a
  signed MOCK; database and application stack are real isolated instances.
- `/Volumes/data/output/live-commerce-account-intake-tests/wizard-browser-release.log`,
  SHA256 `6b919949b429ba911a965745548549fd1a5799017d09c67a50e9f69740344b33`.
- UI evidence: `output/playwright/settings-real-20260924T155737.743633000`;
  final stronger-binding run before the capture correction is also retained at
  `settings-real-20260924T155548.616028000`.
- Account command actually commits, then the page reloads **before receiving any
  response**. A wrong-secret retry returns 409 without losing the original command;
  original-secret retry uses the same key and creates no duplicate credential.
  No secret values appear in storage or HTML; tracing is deliberately off.
- Payment draft commits with its response lost, then retries byte-for-byte with
  the same key. A second editor creates version 2 and binds a non-first account.
  Dirty locale-restored forms cannot borrow that version. Explicit discard
  reloads the saved name **and account binding**. A fresh tab independently
  hydrates the non-first bound account, not the catalog default.
- The browser creates an unseeded market, explicitly saves pricing, then saves
  and reloads manual delivery. A changed session cookie produces zero writes.
- PostgreSQL postflight: two accounts, two credential versions total (each v1),
  one payment head/two immutable versions with `Remote saved name` at v2,
  one market/create command, one policy and one service. No enabled payment
  method, qualification or provider operation. Logout revokes the one session.
- Existing `ledger.spec.ts` + `production.spec.ts`: **8 PASS / 0 FAIL / 0 SKIP**
  at shared-shell baseline `462413f`. Fixture uses the intended development-only
  adapter at 3100; disabled standalone production preview at 3101. Earlier wrong
  fixture runner mode failed and is not counted as product acceptance.
  Evidence: `/Volumes/data/live-commerce-worktrees/settings-ledger-regression-20260924/output/playwright/settings-ledger-regression-success.txt`.
- Root production Next build, admin typecheck, i18n typecheck and 4 i18n tests pass.
- Independent safety review memory: `e3d50cc4-7b91-49e9-bf27-741bf148de02`.
  It reviewed code and root gate evidence, not a separate browser rerun.

## Visual closure

The generic fresh reviewer substitutes for the unavailable named Impeccable agent.
The initial review required three changes: purposeful environment choices,
human-readable safety/next-step copy with semantic icon, and local text contrast.
These are implemented without changing backend states. Scoped muted ink is
`#5f7083`; actual computed 14px text on `#f5f7fa` measures **4.73555:1**.
Detector reports are empty; only static source targets were scanned, not runtime
accessibility. See `wizard-ui-detector.json` and `wizard-ui-detector-rest.json` in
the same external evidence directory. There was no post-fix second detector.

Durable screenshots: `.impeccable/review/merchant-settings/` contains desktop,
mobile, zh-CN, zh-TW, and `hero-repro.png`. The first hero was full-page 1586×1018,
so the reviewer requested recapture; the final hero is a genuine **1586×992**
viewport capture (`fullPage:false`), not a crop.

Final visual verdict: **ship for the three scored fixes**, all resolved; no
presentation-batch regression observed. Memory `333388a4-8f34-4859-96aa-b55c7bea2968`.
The reviewer reported its memory-store pre-lock acknowledgement was unproven;
root retained that procedural caveat and recorded the evidence under its own
confirmed lock. Do not rewrite this as a fully audited lock trail.
Documentation merge is limited to the existing system and this surface brief;
it must not replace the incumbent visual world or promote this surface into a
global completion claim.

## Explicit limits

- Saving PAYUNi credentials leaves `CONFIGURED_UNVERIFIED`; payment methods remain
  disabled drafts. Runtime inspection is diagnostic, not activation.
- Manual delivery is an explicit merchant-arranged branch, not the previous
  platform's provider “自主模式”. No labels, carrier booking or CVS delivery promise.
- A service requires an active market and explicitly enabled policy even when
  the service itself is saved disabled. Policy and service saves are separate.
- Real merchant IdP, production HTTPS, PSP sandbox/live qualification, actual
  payments, carrier callbacks and customer migration: **NOT_RUN**.
- No client store, live stream, production database, payment or message was changed.
- Passing this slice does not certify the entire SaaS for production.
