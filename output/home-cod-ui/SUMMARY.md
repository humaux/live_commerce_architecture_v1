# home-cod-ui — UI delivery / backend handoff

2026-10-02. Worktree `.worktrees/home-cod-ui`, branch `unit/home-cod-ui`.
Actual starting SHA **0c6d965c**, which includes the requested 75931ee plus the collected_at correction. No main/other worktree edits, Go/SQL edits, push, merge, deployment or credential access.

## Commits

| SHA | Scope |
| --- | --- |
| 2c3c320c | Merchant strict COD projection, collect amounts, NT$ display and Taipei order time |
| bfdf91b2 | Buyer cap/fee fence, error passthrough, immutable collect total, localized checkout/order states |
| 49e2cdd0 | Post-collection shipment safety; browser counterexamples and localized responsive evidence |
| 69f829ed | Visual review fixes: primary collect amount, explicit order-total labels, complete mobile footer digits |

All commits carry `Co-Authored-By: Codex <noreply@openai.com>`. The final evidence commit contains this report; find its SHA with `git log -1 -- output/home-cod-ui/SUMMARY.md`.

## Requested items

| Item | Status / evidence |
| --- | --- |
| Blocking orders-model parser | DONE. Exact new cod_collect_minor key; COD collect = safe total + fee; non-COD null. Does not relax closed DTO parsing. Red then green recorded below. |
| 1 Buyer detail, order link, lookup | DONE. Immutable collect total + included fee; PENDING says 到貨需付 NT$75（含貨到付款手續費 NT$50） in synthetic fixture. Closed collection states show original recorded amount, not another payment demand. Direct order URL in all 3 languages and anonymous phone lookup browser-verified. |
| 2 Merchant list/detail/dialog | DONE. All display server cod_collect_minor. Dialog also includes fee. Not an online payment or optimistic collection claim. |
| 3 Expected fee / 409 | DONE. POST carries expected_cod_surcharge_minor including explicit zero. Definite cod_surcharge_changed clears confirmation, reloads options, requires buyer confirmation; never auto-replays. Browser negative + pure/BFF tests. |
| 4 Cap | DONE. Hide COD if quoted total INCLUDING delivery/tax plus COD fee exceeds cod_max_minor; exact boundary allowed. Fractional TWD is distinguished from cap-exceeded. Server remains authoritative. Browser cap counterexample plus pure boundary tests. |
| 5 BFF errors | DONE. cash_on_delivery_unavailable/amount_exceeds/limit and cod_surcharge_changed passed through. COD limit 429 is non-retryable; ordinary rate limits still retryable. Expected fee body survives BFF unchanged. |
| 6 Carrier/manual shipment | PARTIAL / BACKEND BLOCKED. 黑貓 / 新竹 shown in current checkout offer and COD settings; manual shipment explicitly labeled. Historical order carrier DTO and Go manual carrier enum need backend correction: see BACKEND-BLOCKERS.md. No unsupported codes sent. |
| 7 Buyer headline | DONE. Tracks PENDING / COLLECTED / RETURNED / REFUNDED_OFFLINE / CANCELLED / RESTOCKED; browser verifies PENDING and COLLECTED. No misleading confirmed/paid-online state introduced. |
| 8 Money/time/disabled states | DONE. Settings use major whole TWD; authoritative minor-unit money displayed as integer NT$ when whole. Non-COD fractional price precision is preserved, never rounded away. Taipei time explicitly labeled. CVS shows disabled COD with home-only reason. Pre-shipment collection disabled with explanation; after collection shipment void disabled but tracking correction remains available. |
| 9 Locales/responsiveness | DONE for scoped controls. zh-TW / zh-CN / en copy; 390/1366/1586 no document overflow checks. COD settings input/select/save >=44px measured. Existing shells retained. Screenshots use exact CSS pixels; rail resize animation is fast-forwarded, plus input occlusion assertion. |

## Audit coverage

- CodSettings / SettingsWizard: existing whole-dollar parser and settings write contract retained; clearer inclusive-cap instructions and 44px controls. No parallel configuration system.
- MerchantOrders / OrderCodCollection / OrderShipment: collect total, state preconditions and manual fulfillment inspected and repaired.
- Finance / Dashboard: already separate COD from captured/net; retained existing functions and reused the corrected money/time formatters. Ops-polish and COD finance checks passed.
- CodOrderStatus / OrderFlow / purchase / buyer BFF: immutable projection, explicit fee acceptance, cap, coded refusals, scope and recovery boundaries inspected.
- No authentication, inventory, carrier integration, order amount or transaction authority moved into the browser.

## Required gates — actual runs

| Command | Exit | Evidence |
| --- | --- | --- |
| `bash scripts/dev/test-node.sh` | 0 | test-node-final.log — 271 tests passed |
| `pnpm --filter admin exec tsc --noEmit` | 0 | admin-tsc-final.log (empty on success) |
| `pnpm --filter storefront exec tsc --noEmit` | 0 | storefront-tsc-final.log (empty on success) |
| `bash scripts/dev/check-gates.sh` | 0 | check-gates-final.log — 56 modes |
| `bash scripts/dev/test-local.sh --browser-home-cod` | 0 | browser-home-cod-final.log; fixture 20261002T124031.220115000 |
| `bash scripts/dev/test-local.sh --browser-checkout-offline` | 0 | browser-checkout-offline-final.log; fixture 20261002T124120.386194000 |
| `bash scripts/dev/test-local.sh --browser-cvs` | 0 | browser-cvs-final.log; MOCK 20261002T124211.137384000 + WebKit 20261002T124239.191267000 |
| `bash scripts/dev/test-local.sh --browser-ops-polish` | 0 | browser-ops-polish-final.log; storefront 20261002T124327.558576000 + admin 20261002T124335.614451000 |

Tests ran sequentially under the machine PG/browser coordination lock. Browser gates use isolated real Go/PG and Next with MOCK providers/identity. They are not SANDBOX/LIVE certification.
All eight were rerun after the final visual repair with no app/test edits during execution. Earlier passing logs are retained without the `-final` suffix. Terminal carriage returns and trailing whitespace in saved logs are normalized; no output/diagnostic lines removed. Source `git diff --check` exited 0; staged evidence initially flagged terminal whitespace only, then passed after this normalization.

### Red/green and retained assertions

- Baseline browser-home-cod: exit **1**, strict order projection unavailable, `browser-home-cod-red.log`, fixture 20261002T115322.208639000.
- New focused parser counterexample: exit **1** before repair, `parser-red.log`.
- `node --test --experimental-strip-types tests/admin/orders-model.test.ts tests/admin/customers-model.test.ts tests/admin/customers-bff.test.ts tests/admin/merchant-tools-model.test.ts`: exit **0**, `parser-green.log`.
- No existing assertion deleted or weakened. Fixture additions supply the backend's required null field. Money tests only update the required TWD prefix from `TWD ` to `NT$`, retaining exact numeric precision. Definite-refusal expectation adds the new COD limit without removing the old limit.
- Additional browser tests caught a 38px save button (fixed to 44px) and screenshot transition artifact (animations disabled + occlusion checked). Failure traces remain under `output/playwright/home-cod/`.
- Negative provider response tests use the exact BFF headers/error schema; malformed JSON/unknown-outcome safety stays intact.
- Finish-review regression assertions measure complete footer number bounds and primary collection-amount hierarchy, rather than merely testing document width.

## Screenshots

This directory contains 30 `admin-settings-*`, `buyer-checkout-*`, `buyer-confirm-*`, and `buyer-order-pending-*` PNGs. `390` = 390×844; `1366` = 1366×992; `1586` = 1586×992. All PNG dimensions independently checked, exit 0. Required zh-TW/en included. Settings and order detail include additional zh-CN captures. `buyer-confirm` and admin settings are deliberately scrolled feature captures; `buyer-checkout` and order detail capture document top. Synthetic data is explicitly labeled.

## Independent review / traceability

- Read-only contract/security review: no introduced P0/P1 confirmed; one P2 precision-vs-cap wording finding fixed. Humaux title: `home-cod-ui scoped independent review: one P2 false cap hint, no confirmed introduced P0/P1`.
- Visual finish review: **ship** after fixing the two scored findings (amount hierarchy and clipped English 390px footer); all 12 required captures independently rechecked. See VISUAL-REVIEW.md. Impeccable audit/finish workflow influenced the explicit primary collect amount and responsive numeric-bounds tests; incumbent design retained. Humaux verdict 42293e7b-e2d9-45aa-99e0-655c701f7a5b.
- Initial architecture/surface audit: SURFACE.md. Backend continuation: BACKEND-BLOCKERS.md.
- 31 changed source/test files submitted to incremental code_index; seven final repair files resubmitted, index status done. `parseOrderSummary` linked to memory b946dc5d-8ddd-4424-b74b-c27a19a2bdec; `offeredPaymentModes` linked to a72a99bc-5fe9-4c66-8142-10f995031da7; `CodAmount` linked to the final visual verdict.
- Coordination task: 5e810ce5-6e3c-4f2d-9498-6f9cd6fe5ea2, agent codex-home-cod-ui. Root sole writer; read-only assistants had no write_paths. Base for all audits 0c6d965c.
- Roles/model records: root Codex UI implementer (host model/effort not exposed); r5_gate_paths and r5_image_audit read-only explorers (gpt-6-luna, medium); domains_security_review independent read-only review (gpt-6-sol, high); domains_visual_review read-only visual review (gpt-6-luna, medium). No recursive delegation or concurrent writers.

## NOT_RUN / unresolved

- Historical carrier projection and Go carrier allowlist: BLOCKED, backend-only changes excluded from this unit.
- Live carrier collection, payment/PSP operations, production deployment: NOT_RUN and not authorized.
- CVS SANDBOX: NOT_RUN — sandbox credentials/flags not used; MOCK and WebKit passed.
- `tests/media/r04-input-runner.test.mjs`: NOT_RUN — COMMERCE_R04_LIVEKIT_BINARY unset (reported by test-node, unrelated to COD).
- Remaining collection states use reviewed localized mappings; no claim that all six were driven through live carrier events.
- The older backend SUMMARY's finance updated_at deviation is stale at this actual base, already corrected by 0c6d965c.

Original untracked BRIEF-with-rulings.md and output/home-cod-fix/ were preserved, not staged. Owned test services exited through the harness; no production/customer service was stopped.
