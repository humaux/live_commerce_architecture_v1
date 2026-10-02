# home-cod-ui — final closeout

2026-10-02. **PASS: scoped UI closeout and all eight requested local gates.** Not production/provider acceptance.

- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/home-cod-ui`; branch `unit/home-cod-ui`.
- Closeout base: `1735195d`, verified to contain backend `e3ab330`. Final tested source: **`c5f47771`**.
- Original delivery at `69f829ed` / `76efda33` and its evidence remain in history. Backend gaps are now closed; see BACKEND-BLOCKERS.md for the explicitly historical findings.
- No Go/SQL/deploy changes, other-worktree writes, push, merge, credentials, or production activity.
- Evidence-only commit contains this report; resolve it with `git log -1 -- output/home-cod-ui/SUMMARY.md`.

## Closeout commits

| SHA | Change |
| --- | --- |
| e8bfeaab | Validated order-time carrier; manual shipment codes and labels; COD-specific sidebar state |
| cb017877 | Carrier readback on buyer entry routes; corrected capture timing and extra state evidence |
| f6bf3c95 | Distinguish checkout carrier from actual shipment; differing-carrier counterexample |
| c5f47771 | Exact CSS-pixel screenshot sizes and horizontal/vertical page-top assertions |

Each commit ends with `Co-Authored-By: Codex <noreply@openai.com>`.

## Latest six requests

| Request | Result |
| --- | --- |
| 1. Immutable buyer carrier | PASS. Shared CodOrderStatus reads validated `order.cod_carrier`, never current settings. Order page, direct URL in three locales, and lookup assert the snapshot. Labels explicitly say “下單時的物流商 / 下单时的物流商 / Carrier at checkout” and manual shipping. COD requires black_cat/hsinchu; non-COD permits null/legacy absence, not a forged carrier. |
| 2. Merchant shipment | PASS. Both black_cat and hsinchu accepted in admin/buyer parsers and localized options. Actual isolated-PG COD fixture ships black_cat and buyer reads back shipment.carrier_code and cod_carrier as black_cat. No carrier API implied. |
| 3. D2 collection wording | PASS. COD sidebar uses COD state copy: 等待貨到付款 / 已貨到收款. Pickup orders retain their existing copy. Browser checks pending and collected in both the COD panel and sidebar. |
| 4. D1 filename language | PASS. Existing current files were already correctly named on inspection; no destructive filename swap. All checkout/confirm files regenerated. Capture asserts HTML lang and route locale match filename locale; independent visual checks confirmed. |
| 5. Supplemental captures | PASS. Disabled gray CVS COD with home-only reason (four captures); RETURNED / REFUNDED_OFFLINE / CANCELLED titles (12 explicitly MOCK captures); zh-CN order from top; valid saved settings without red validation. The old settings error screenshot was taken before saving corrected values, not a valid-save product bug. Capture now follows save success and absence of error. |
| 6. P3 Taipei time | PASS. Existing list header already says Taipei UTC+8 in all three locales at this base. Added browser assertion on the orders table; no duplicate time formatter. |

## Original delivery requirements retained

| Item | Final status |
| --- | --- |
| Blocking strict orders parser | PASS. cod_collect_minor remains mandatory in merchant DTO; no closed-parser relaxation. |
| 1 Buyer amount on all entry routes | PASS. Immutable collection total with included COD fee; closed orders show original amount rather than requesting payment again. |
| 2 Merchant list/detail/dialog amount | PASS. Server collection total displayed, never client-authoritative money. |
| 3 Expected fee / 409 | PASS. Explicit expected fee sent; cod_surcharge_changed forces renewed buyer confirmation, no blind replay. |
| 4 Collection cap | PASS. COD hidden above inclusive cap; whole-TWD rules preserved. |
| 5 BFF errors | PASS. cash_on_delivery_* codes retained; COD limit not retryable. |
| **6 Carrier / manual shipment** | **PASS. Backend e3ab330 closed both original gaps; order snapshot and supported shipment codes wired and browser-verified.** |
| 7 Buyer title | PASS. PENDING/COLLECTED from real local workflow; three extra terminal titles from explicitly MOCK projections. |
| 8 Money/time/disabled controls | PASS. Integer NT$ COD, Taipei labels, home-only COD, pre-shipment collect disabled. |
| 9 Three locales / responsive controls | PASS in scoped surfaces: 390/1366/1586 checks, settings controls >=44px; incumbent K3-approved design retained. |

## Gates on final source c5f47771

| Command | Exit | Log / evidence |
| --- | --- | --- |
| `bash scripts/dev/test-node.sh` | 0 | final-test-node.log; 271 passed, 0 failed |
| `pnpm --filter admin exec tsc --noEmit` | 0 | final-admin-tsc.log (empty on success) |
| `pnpm --filter storefront exec tsc --noEmit` | 0 | final-storefront-tsc.log (empty on success) |
| `bash scripts/dev/check-gates.sh` | 0 | final-check-gates.log; 56 modes |
| `bash scripts/dev/test-local.sh --browser-home-cod` | 0 | final-browser-home-cod.log; home-cod/20261002T142733.701501000 |
| `bash scripts/dev/test-local.sh --browser-checkout-offline` | 0 | final-browser-checkout-offline.log; checkout-offline/20261002T142814.332618000 |
| `bash scripts/dev/test-local.sh --browser-cvs` | 0 | final-browser-cvs.log; taiwan-cvs/20261002T142855.625114000 (MOCK), 20261002T142923.709644000 (WebKit) |
| `bash scripts/dev/test-local.sh --browser-ops-polish` | 0 | final-browser-ops-polish.log; ops-polish-storefront/20261002T143016.875043000 and admin path recorded in log |

Fixture directories are under `output/playwright/` in this worktree. Runs were serialized under the machine PG/browser coordination lock and harness locks, on unchanged product/test source. Later edits are evidence only.

Additional focused command: `node --test --experimental-strip-types apps/storefront/tests/purchase.test.mjs tests/admin/refund-bff.test.ts`, exit 0, 20/20 (closeout-focused-green.log). First run exit 1 found an older refund fixture missing mandatory nullable COD amounts; only those null fields were supplied, preserving strict parsing and every negative assertion. Initial red is retained in closeout-focused.log. No existing assertions removed or loosened.

## Captures and independent review

- **46 PNGs** in this directory, with hashes, byte sizes, exact dimensions and evidence scope in CAPTURE-MANIFEST.json. Every size verified: 390×844 / 1366×992 / 1586×992, no mismatch.
- checkout/confirm: zh-TW/en at all three widths. Order pending/settings: also zh-CN. Pending capture asserts scrollX=0, scrollY=0 and no horizontal overflow.
- buyer-cvs-cod-disabled-*: zh-TW/en, 390/1586. Gray disabled radio and “僅限宅配 / Home delivery only”; CSS-pixel dimensions checked in the browser script.
- buyer-returned-mock-*, buyer-refunded_offline-mock-*, buyer-cancelled-mock-*: zh-TW/en, 390/1586. **MOCK responses only**, no real order cancellation, refund or returned transition. Snapshot Hsinchu differs from recorded Black Cat shipment to prove the history label is not current shipping data.
- Admin settings captures have valid saved values and no red validation. Existing full-flow merchant screenshots and screenshot hashes are in the home-cod fixture directory.
- Independent read-only contract/security review: no confirmed new P0/P1. P2 ambiguous carrier label fixed by f6bf3c95 and re-reviewed; Humaux `e5a558e3-8076-4bbb-9686-643745e7c9d8`.
- Independent visual review: localized labels/terminal titles readable; initial suspected header/scroll problem retracted after exact-file readback and deterministic scroll/width evidence. Corrected Humaux record `7cb1f898-2dd3-454a-b094-b8178e97d2c4`. No new confirmed P1/P2.
- Original owner K3 report (P1=0, P2=0) preserved unmodified. Frontend-architect and Impeccable audit-first/harden guidance kept this as a contract/copy/evidence patch, not a redesign; Playwright supplied repeatable evidence.

## Traceability and scope

Root was the sole writer. Read-only assistants: r5_gate_paths (test/capture investigation), domains_security_review (independent review), domains_visual_review (visual check). No recursive delegation. Root and reused assistants' exact runtime model/effort were not newly exposed, so recorded as UNKNOWN rather than inferred.

Coordination task `5e810ce5-6e3c-4f2d-9498-6f9cd6fe5ea2`, agent `codex-home-cod-ui`. Twelve changed source/test files and final two capture-script updates submitted to incremental code_index. CodOrderStatus and validCodAmount linked to the carrier-history review memory. Final result stored in Humaux and own canvas updated at closeout.

## NOT_RUN / boundaries

- Production deployment, customer operations, LIVE carriers/PSPs, real collections/refunds: NOT_RUN, outside authorization.
- CVS external SANDBOX: NOT_RUN (no owner keys/flags used); MOCK and WebKit actually passed.
- `tests/media/r04-input-runner.test.mjs`: NOT_RUN because COMMERCE_R04_LIVEKIT_BINARY unset; script reports it explicitly.
- Real backend RETURNED/REFUNDED_OFFLINE/CANCELLED lifecycle transitions were not added to this UI closeout. Their new screenshots prove rendering only.
- No full-release/security/load acceptance claimed. No unresolved backend blocker remains for the two carrier gaps.
- Untracked integrator BRIEF-with-rulings.md, REPORT-home-cod-ui-vqa.md and output/home-cod-fix/ preserved and not staged. Test harnesses tear down only their owned services/fixtures; no customer service stopped.
