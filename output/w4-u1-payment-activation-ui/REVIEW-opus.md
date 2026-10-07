# W4-U1 independent money + security review (Claude Opus 5.5, read-only)

- Subject: `unit/w4-u1-payment-activation-ui` HEAD `5a8a9855288852a0603d6ad2ece5f902ec8f8a43`, diff `8464eb96...HEAD -- apps tests scripts docs`
  (the ads/refund-bff hunks in that range come from the `r3/integration` merge a52f3266, not from this unit).
- Binding inputs: brief `docs/delivery/units/w4-u1-payment-activation-ui.md` (Integrator 裁决), contract `contracts/stripe-platform-account-v1.md`
  (§0.3 AD-PF2, §2, §3.3, §5, §6), integrator rulings in the review card (ENABLED on a not-OPEN platform keeps disable; limits come from the server).
- Reviewer did not edit or commit code. Only this file was written.

## Verdict: **MERGE-AFTER-FIX**

There is no P0. There are two P1s. Both are display or copy fixes: neither touches money logic, authority or the wire. The CI browser gate
`--browser-card-payments` has never run, so it must also pass once before merge, as DELIVERY already says.

## What I ran (this HEAD, DB-free)

| Check | Result |
| --- | --- |
| `node --test --experimental-strip-types tests/admin/card-payments-{model,request,copy,wire}.test.ts apps/storefront/tests/payment-collector.test.mjs` | exit 0, 46/46 |
| `cd apps/admin && pnpm exec tsc --noEmit -p .` / same in `apps/storefront` | exit 0 / exit 0 |
| `go vet ./tests/foundation` / `go vet -tags browser ./tests/foundation` (go1.27.1) | exit 0 / exit 0 (so there are no helper-name clashes under `//go:build browser`) |
| `bash scripts/dev/check-gates.sh` | exit 0 (74 modes, headers OK; G-UI5 warnings only) |
| Contract §5 copy diff (python, extracting the quoted strings from the contract file) | zh-TW terms, plus the zh-TW, zh-CN and en storefront templates, are byte-equal to the contract |
| JavaScriptCore (`jsc`) `Date.parse` of the PG forms `…33.774048+00:00` and `…33+00:00` | finite (the WebKit parsing risk is cleared) |
| Probe script (scratchpad) against the real wire fixtures | evidence for P1-1, P1-2, P2-2, P2-3 and P2-5 below |

NOT_RUN: `TestW4U1WireShapes`, `TestW4U1StateSequence` (PG), the `--browser-card-payments` gate, builds, SANDBOX and LIVE.

## P0

None.

## P1

### P1-1 Settlement amounts are shown with two opposite sign conventions and no legend

- **Evidence.**
  - `apps/admin/components/Settlements.tsx:138-144` (list) and `:175-199` (detail) print each server field verbatim with `money()`.
  - The labels at `apps/admin/lib/card-payments-settlements-copy.ts:24-29` (`Refunded`, `Disputes`, `Stripe fee`, `Platform fee`, `Carried in`)
    carry no direction.
  - The server convention (migrations/0150 CHECK, `net = captured − refunded − dispute + stripe_fee − platform_fee + carried_in`) mixes two senses:
    - `refunded`, `dispute` and `platform_fee`: positive means deducted;
    - `stripe_fee` and `carried_in`: negative means deducted.
  - The probe, rendering the real `settlements-list.json` rows as the page does:
    - `2026-09-07`: `NT$25 | NT$8 | NT$25 | -NT$60 | NT$0 | NT$0 | -NT$68`. Refunds and disputes look positive, the fee looks negative, and both reduce the net.
    - `2026-09-14`: `NT$25 | NT$0 | -NT$25 | NT$58 | NT$0 | -NT$68 | NT$40`. A dispute **reversal (a credit)** reads `-NT$25` under "Disputes", and a
      **returned fee (a credit)** reads `NT$58` under "Stripe fee".
  - In the detail view, the same refund is `-NT$8` in the lines table (`Settlements.tsx:239`, `store_minor`) but `NT$8` in the totals.
- **Why.** This is the merchant's reconciliation document against a bank transfer. In exactly the sign cases the brief calls out
  (negative fees, refunds, disputes and carry-forward), a natural reading gives the wrong direction for 2 of the 6 component columns. The visible
  numbers cannot be summed to the server's net. The numbers are faithful to the server, but the presentation is not correct.
- **Fix (no recomputation).** Keep the server values verbatim and make the direction explicit:
  1. Use direction-carrying labels in all three languages. For example, 「退款（自應撥額扣除）」 and 「Stripe 手續費（負數＝扣除）」, or the more
     compact 「退款 −」 / 「平台費 −」 / 「Stripe 手續費 ±」 / 「上期結轉 ±」.
  2. Add one formula line under the table and the detail, quoting the 0150 identity:
     `應撥淨額 = 收款 − 退款 − 爭議款 + Stripe 手續費 − 平台費 + 上期結轉`.
  3. Add a literal-string test, not one that goes through `money()`, for the `2026-09-14` row in STL1. A node copy test should pin the legend.

  The alternative is to show every column as its signed contribution to net, negating refunded, dispute and platform_fee for display only. That
  departs from the CSV export (§6.5), so labels plus the legend are the smaller change.

### P1-2 A store that is not allowlisted, on an OPEN platform, does not show 「信用卡收款尚未開放」, and the badge says the platform is open

- **Evidence.**
  - `apps/admin/lib/card-payments-model.ts:138-139`: `notOpen: !open`, `notAllowed: open && !summary.allowed`.
  - `apps/admin/components/CardPayments.tsx:164-175` renders the platform badge 「已開放」 plus `c.notAllowed` =
    「此商店不在平台信用卡收款允許名單內，請聯絡客服。」 (`apps/admin/lib/card-payments-copy.ts:163`, en `:30`).
  - Probe: `cardView({platform_state:"OPEN", allowed:false, store_state:"NONE"})` → `{"notOpen":false,"notAllowed":true,...}`.
  - `tests/admin/card-payments-model.test.ts:350-352` pins this behaviour, and no browser spec covers the not-allowlisted state.
- **Why.**
  - The review card states the expected display: "not allowlisted or platform not OPEN → 「信用卡收款尚未開放」 with no enable control".
  - Under AD-PF2 (§0.3), every third-party store is legally ineligible. These are most stores. Today every one of them is told that the platform
    is 「已開放」 and that it should contact support to get onto an allowlist that it cannot legally join. That is a support and legal-messaging
    problem.
  - The safety half is correct: there is no enable control (`canEnable` requires `allowed`).
- **Fix.**
  1. In `cardView`, set `notOpen = !open || (!summary.allowed && (state === "NONE" || state === "DISABLED"))` and drop the `notAllowed` text.
     A withdrawn-but-ENABLED store keeps its disable button, as in the CLOSED ruling.
  2. Do not show the 「已開放」 platform badge to a store that is not allowlisted. Either hide it, or show the not-open label.
  3. Update the node test.
  4. Add a CPU case through a `w4uControl` `store/disallow` route that uses the real `platform-disallow` definer.

  If the integrator prefers to keep a distinct not-allowlisted message, this item drops to P2. The badge contradiction still needs the fix.

## P2

1. **Two ruled states have no browser assertion.** The ENABLED store on a CLOSED platform (disable kept: integrator ruling) and the
   not-allowlisted state are both pinned only by node tests (`card-payments-model.test.ts:339-341`). CPU5 (`tests/admin/card-payments.spec.ts:241-258`)
   closes the platform only while the store is DISABLED. Fix: in CPU5, close the platform once while the store is ENABLED and assert that
   `card-disable-open` is visible and that `card-enable-open` has count 0.
2. **The live suffix-length check does not apply the server's rule.**
   - UI: `suffixBudget` = `min(10, 22 − len(descriptor_display) − 2)` (`card-payments-model.ts:103-106`).
   - Server: `L = 10` in SANDBOX, or `PrefixLength` in LIVE (`migrations/0137_platform_stripe.sql:638-648`).
   - Probe: for a 15-character display such as `HK BOWL TRADING`, the UI caps the suffix at 5 while SANDBOX accepts 10. With LIVE
     `PrefixLength > len(display)`, the UI under-refuses and the server answers 422.
   - The server stays the authority, so this is not a safety issue. It is a functional over-refusal that does not match the brief's
     "against the server's rule".
   - Fix: have the read return the server's `suffix_max` (contract change), or let the UI enforce only the charset and ≤ 10 and leave the
     length to the server's `descriptor_suffix_too_long`.
3. **The `"* "` split is ambiguous.** `descriptorBase` and `descriptorSuffixOf` (`card-payments-model.ts:114-127`) assume that the display can
   never contain `"* "`, but designate admits `*` (`0137:723`, `^[A-Za-z0-9 .*-]{5,22}$`). Probe: `descriptorSuffixOf("LC* SHOP")` → `"SHOP"`, so a
   NONE store gets a bogus suffix pre-filled. Fix it server-side: Stripe forbids `*` in descriptors, so drop `*` from the designate regex, or return
   the display and the suffix separately.
4. **A failed post-write refresh is ignored.** `CardPayments.tsx:124-149` discards `refresh()`'s boolean, so "Saved." or "The page reloaded"
   can sit next to a stale badge. This is safe because the CAS rejects a stale retry, but it is misleading. Fix: `if (!(await refresh())) reload();`.
5. **The detail parser accepts a body with missing lines.** It accepts a detail with `line_count > 0` and no `lines` key
   (`card-payments-settlements-model.ts:158-174`). Probe: carried detail with `lines` removed → accepted, `lines=null`, and an empty lines table is
   shown. Fix: in `parseSettlementDetail`, require `lines` when `line_count > 0`. A quiet-week statement has `line_count = 0` and Go omits the
   empty array, so that shape stays valid.
6. **The Settlements page cannot be reached from the UI.** No link points to `/settings/settlements` (grep), and the route is `nav:false`.
   The page also fetches only the first 52 statements; the grammar supports `before=`, but the page never pages. Fix: link to it from the card
   page and the billing page for `billing:manage` holders. Paging can wait until a year of statements exists.
7. **The leak scans are partial.**
   - `tests/admin/settlements.spec.ts:67-73` scans the DOM and storage, but not network bodies (the card spec does).
   - Neither spec has a negative control showing that the scanner can fail.
   - Approval ids are UUIDs, so no regex can see them. The strict parsers, which refuse unknown keys, are the real guard there.
   - Fix: reuse `watch()` in the settlements spec, and add one stubbed body containing `acct_1X…` that must turn the scan red.
8. **The copy test does not read the contract.** `tests/admin/card-payments-copy.test.ts:32-47` hard-codes the contract sentence instead of
   extracting it from `contracts/stripe-platform-account-v1.md`, so the two can drift unseen. The extraction is about 5 lines (as done in this
   review).
9. **The disclosure's store name is merchant-editable.** The storefront's `{store_name}` is the design profile name
   (`apps/storefront/app/[locale]/layout.tsx:51,67`), which the merchant can edit. It falls back to 「商店」/"Store" when the design can't be read.
   For a legal collector disclosure, the server-side `collector` projection should carry the store's name. This is a server follow-up, not a
   UI defect.
10. **The fixture writes outside the definers.** The buyer half of the harness writes directly to the owner pool, outside the operator definers:
    `DELETE FROM payments.method_heads … payuni_credit` and `UPDATE catalog.products SET name`
    (`tests/foundation/browser_card_payments_test.go:56,173`). The writes are disclosed in comments and happen after the admin assertions.
    Acceptable as fixture shaping, but the claim "seeding only through real definers" covers the platform and ledger state only.
11. **Writes outside the brief's write paths** (they are needed; the integrator should acknowledge them):
    - `apps/storefront/lib/payment-contract.ts`: without it, the strict validator would refuse every derived-connection view;
    - `apps/storefront/app/[locale]/layout.tsx` and the new `components/ShopName.tsx`;
    - `tests/foundation/browser_click_sweep_test.go:193-194`.
12. **The 收款 step still offers PAYUNi** (outside this unit). The new entry link (`SettingsWizard.tsx:1759-1767`) sits in a step that still offers
    PAYUNi with HashKey/HashIV password inputs (`:1708`). PAYUNi is cancelled by §0.1, so this belongs to the `pay-remove-payuni` follow-up.

## The six checks, item by item

1. **Credentials and identifiers: PASS.**
   - The card page has exactly one checkbox and one suffix text field (`CardPayments.tsx:277-298`), with no key, secret or account field.
   - Nothing renders `display_name`, an account id, a key or an approval id.
   - There is no `localStorage`, `sessionStorage` or `document.cookie` use and no `console.*` in the new code (grep over the unit's files).
   - URLs carry only the store UUID and the statement UUID (`card-payments-client.ts:17-26`).
   - BFF:
     - the PUT body is exactly `{enabled, terms_version, descriptor_suffix, expected_version}` (`card-payments-request.ts:63-89`, wired at
       `route.ts:340`);
     - the PUT is keyless (`route.ts:299`, so no Idempotency-Key is forwarded);
     - GETs carry no body and no key (`card-payments-request.ts:51-58`, `route.ts:148`);
     - only `before` and `limit` are accepted, once each, on the list (`:33-48`);
     - responses pass through (`route.ts:460`), but Go strict-decodes into closed structs (`platformstripe/service.go:93,162`,
       `settlement/service.go`) and the admin parsers refuse unknown keys.
   - A credential-shaped `payout_ref` is refused (`settlements-model:51,151`). This is defence in depth: the 0150 regex already excludes `_`.
   - Gap: P2-7.
2. **State machine: PASS, except P1-2.**
   - Not OPEN shows 「信用卡收款尚未開放」 and no enable control (`cardView`; node test `:331-338`; CPU5).
   - Allowlisted and OPEN: the dialog quotes the §5 terms byte-for-byte, with `terms_version`, and the preview is built from the server's
     `descriptor_preview` (`CardPayments.tsx:268-312`).
   - BLOCKED shows 「已被平台暫停，請聯絡客服」 and offers disable only (CPU6, node `:357-363`).
   - ENABLED on a not-OPEN platform keeps disable (`card-payments-model.ts:142`; node `:339-341`).
   - Nothing is optimistic:
     - the badge only ever comes from a server read;
     - 409, uncertain and closed answers reload (`CardPayments.tsx:131-149`);
     - CAS is enforced by `expected_version`;
     - `canConfirm` only gates the button, and the server re-decides allowlist, OPEN, block, terms and suffix.
3. **Money display: PASS on correctness and FAIL on presentation (P1-1).**
   - Amounts are integer minor units formatted by the shared `money()` (`NT$25`, `-NT$68`, `NT$0.50`; checked identical in V8 and JSC).
   - Nothing is summed for display. The parsers cross-check 0150's identities and refuse anything inconsistent rather than "fixing" it
     (`settlements-model:139-173`).
   - Offsets `±HH:MM` with up to 9 fractional digits are accepted (`:11-14`).
   - `red-wire.log` (Kimi parsers, 3/3 real bodies refused) → green on the same fixtures (`card-payments-wire.test.ts`).
4. **Storefront disclosure: PASS.**
   - The three templates match the contract byte-for-byte.
   - `collectorDisclosure` returns null unless `collector` is present (`payment-copy.ts:137-147`).
   - The line renders inside the view block, before both `pay-order` buttons (`OrderPayment.tsx:434-440`, 501/536; source-order test).
   - The validator admits only `{display_name, descriptor_preview}` (`payment-contract.ts:241-249`).
   - The server emits `collector` only for derived connections (`migrations/post_river/0022_platform_stripe.sql:254-268`).
   - The buyer script covers unpaid in 3 languages, paid, and primary-absent. Its DOM-order check uses `compareDocumentPosition`. Store name:
     P2-9.
5. **Shell and permissions: PASS.**
   - Both routes are `nav:false`, pinned in `card-payments-model.test.ts:411-425`.
   - The card page requires `integration:read`; the server GET checks it in Go (`payment_card.go:34`) and in SQL (`0137:700`).
   - The toggle UI requires owner or `billing:manage` (`CardPayments.tsx:112`); the server PUT requires `billing:manage` (`payment_card.go:44`,
     SQL §3.3).
   - Settlements require `billing:manage` in the route, in Go (`settlements.go:111`) and in SQL.
6. **Tests: PASS with gaps.**
   - The node suites encode the parsers, the BFF grammar, the copy, the state machine and nav:false.
   - The harness seeds the platform and ledger through the real definers: designate, allow and set (`pslNew`); `PlatformOpen`; `PlatformBlock`;
     `platformstripe.Set` for the bump; sync → close → payout.
   - It is test-mode only: SANDBOX environment, `PROVIDER_MOCK`, and a `LC_BROWSER_CARD_PAYMENTS_ACCEPTANCE=1` plus `LC_TEST_DATABASE_ALLOWED=1`
     guard.
   - The helper names (`cpbr*`, `w4u*`) are unique; `go vet -tags browser` is clean.
   - Gaps: P2-1, P2-7, P2-8, P2-10. The browser gate has never run, which DELIVERY discloses.

## Evidence level

- E3 for the node, typecheck, vet and gate checks I ran at `5a8a9855`.
- The PG and browser halves were not independently re-run (NOT_RUN above).
