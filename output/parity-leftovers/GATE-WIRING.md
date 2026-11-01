# PR473 runnable gate wiring handoff

- task_id: `53ea9f21-3f3b-4626-87ab-42dc853507b9-sub-leftovers-gate`
- parent_coord: `53ea9f21-3f3b-4626-87ab-42dc853507b9`
- role / dispatch model / reasoning: `test_worker / gpt-6.1-sol / high`; no delegated agents or external model calls.
- base_commit: `33d5749aa597d3ff587225dc580339a80e55f5e5` (original PR473).
- integration: clean merge of `origin/r3/integration` at `c52b65283a5ac738d0c527a464d0cccc9523b1f4`; merge checkpoint `6304373dc842ae0e29401dec5a71d6bf51401ee7`.
- branch / worktree: `codex/pr473-gate` / `.worktrees/codex-pr473-gate`.
- evidence level: **E1, wiring structure only**. Browser/product acceptance, Go compile, real PG runtime, independent review, providers and LIVE are **NOT_RUN** in this subtask.

Changed paths (this wiring commit only):

1. `tests/foundation/browser_parity_leftovers_test.go`
2. `scripts/dev/test-local.sh`
3. `docs/delivery/GATES.md`
4. `output/ci-select-backend-browser/browser-ownership.json`
5. `output/parity-leftovers/GATE-WIRING.md` (existing tracked directory spelling; the assigned uppercase path resolves here on this case-insensitive volume)

The new Go test reuses the signed mock OIDC + production Next + real identity/authorization stack (`brfStartAdmin`) and generated direct `testMatch` (`brfPlaywright`). It registers **the original twelve** desktop/mobile zh-TW checks; `tests/admin/parity-leftovers.spec.ts` is unchanged (SHA256 `c03599eccc37476d114e6afb2a528a9f6f3c92e1bc9ef9861cbba2a5a9b1befe`). There is no root Playwright configuration edit or fabricated UI. The existing live-settings mode retains its tests, env, builds, browsers and 780-second Go timeout.

GET-only MOCK projections cover the exact order/product IDs used by the spec, current valid order/list/actions/history, product+variant, empty valid image list, collections/warehouses and six signed template metadata rows. Each intercepted read uses `platform.WithScope` against the real isolated fixture; other methods/routes fall through to the existing Go handler. The fixture emits no template bodies, provider sends or writes. Before browser execution the new test runs current production DTO parsers with Node and obtains authenticated 200 GET receipts, to identify fixture failures separately from product assertions.

## Actual static commands and exit codes

| Command | Exit | Observed result |
|---|---:|---|
| `git worktree add .worktrees/codex-pr473-gate -b codex/pr473-gate 33d5749aa597d3ff587225dc580339a80e55f5e5` | 0 | isolated worktree created; old parity-leftovers worktree untouched |
| `git merge --no-edit origin/r3/integration` | 0 | clean merge at checkpoint above |
| `gofmt -w tests/foundation/browser_parity_leftovers_test.go` | 0 | Go syntax/format; does not typecheck |
| `node --check --input-type=module` (stdin: extracted generated fixture JS, Go `%q`/`%s` placeholders replaced with syntax-only strings) | 0 | generated fixture JS syntax valid; no fixture logic executed |
| `node scripts/dev/derive-browser-ownership.mjs` | 0 | wrote ownership for 55 modes |
| `node scripts/dev/derive-browser-ownership.mjs --check` | 0 | no ownership drift; existing mode owns original spec |
| `bash -n scripts/dev/test-local.sh` | 0 | shell syntax valid |
| `git diff --check` | 0 | no whitespace errors |
| `python3 scripts/check_packet.py` | 0 | `PASS_PACKET_STRUCTURE_ONLY`, `delivery_baseline_checked=false`; generated `experiments/results/packet-check.json` restored to checkpoint bytes afterward |
| `shasum -a 256 tests/admin/parity-leftovers.spec.ts tests/foundation/browser_parity_leftovers_test.go` | 0 | original spec hash above; new Go source `db79f54b2ab901c291711bcc41a724f4b6f083de6d47b5b3f57caf25b4713d11` |

No local tests, browsers, servers or containers were started. No production implementation, assertions, thresholds, dependency locks, contracts or CI definitions changed. No dependency-map regeneration is needed: the new file is a browser-tagged test only and the current depmap reads non-test package imports.

## GitHub gate to run after parent review/push to existing PR473

`bash scripts/dev/test-local.sh --browser-live-settings`

Its native runner is:

`LC_BROWSER_LIVE_SETTINGS_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.3 go test -race -tags browser -count=1 -timeout=780s -run '^(TestBrowserLiveSettingsUIRealChain|TestBrowserParityLeftovers)$' -v ./tests/foundation`

The shell mode provides the existing PG/build/browser preparation. Do not claim a product RED from gate registration, Go compile, login, fixture parser, API fixture, browser launch or timeout failures. Parent must read actual GitHub Playwright enumeration of all twelve cases, successful signed login and fixture logs before classifying assertion failures. No new/duplicate PR or remote push was made by this worker.

Expected run-owned artifacts: `${LC_BROWSER_EVIDENCE_ROOT:-output/playwright}/parity-leftovers/<UTC>/fixture-contract.log`, `mock-projections.json`, `mock-get-receipts.json`, `next.log`, `playwright.log`, generated Playwright config, failure screenshots/traces and original `parity/clicks-*.json`. These paths are **NOT_RUN**, not existing runtime evidence.

## Scenario classification and unresolved limits

| Original scenario | Fixture readiness / authority boundary |
|---|---|
| 訂單缺少收件資料 (2 cases) | **BLOCKED data/route contract alignment**: current frozen order DTO requires a complete destination and represents an existing order. The neutral valid DRAFT fixture does not model an uncreated pre-order with missing phone/CVS information; failures here are not sufficient product RED. Root authority must define that data/route contract before implementation. |
| 訂單匯出說明 (2) | Current order GET/actions ready as MOCK; missing export help route/copy can be classified only after actual CI login/fixture receipts. |
| 媒体上传 (2) | **BLOCKED media candidate contract alignment**: current image list is valid empty and has no signed candidate name/label field or shared candidate library route. Do not call missing named candidates product RED, invent fields, or add fixture HTML. Product media controls/copy still execute unchanged; classify actual failures from CI. |
| 批量填充 (2) | Valid current product+variant MOCK GET; actual controls/copy assertions remain original and await CI. |
| 消息模板 (2) | Six valid metadata records carry the exact signed names/versions/kinds/dates/public flags; routes/copy await CI. Existing repeated exact-text selectors remain unchanged; classify ambiguity as an original-test issue if observed. |
| 新增自訂訊息範本 (2) | GET metadata available; route/form is real product code. No publish/send fixture action is implemented; original visible-copy assertions await CI. |

The first actual GitHub execution reached the parser preflight and failed before authenticated GET preflight or browser execution; see the current repair section below. The revised parser preflight and authenticated GET preflight await GitHub execution. The original 780-second combined budget is unchanged; budget sufficiency is UNKNOWN until observed. Parent owns independent review, push to existing PR473, CI monitoring and any narrowly authorized fixture repair (maximum two directed attempts). No product implementation is authorized by this handoff.

## Directed FIX1 after root independent review

Root found that the initial MOCK amount 1250 represented fractional TWD despite the frozen whole-dollar rule (`contracts/storefront-v2.md` section F; `apps/admin/lib/catalog-v2-model.ts:272`). All summary/item/totals/SKU amounts now consistently use 1200 minor units (12 TWD); quantity remains one. No assertion, product code or money contract changed. The parser-only preflight did not encode this domain rule; valid parser shape alone is not domain acceptance. Original twelve cases remain unchanged.

FIX1 static commands: `gofmt -w tests/foundation/browser_parity_leftovers_test.go`, extracted generated JS `node --check --input-type=module`, and `git diff --check` exit 0. After invite agent released the global ownership lock, acquired it, ran `node scripts/dev/derive-browser-ownership.mjs` and `node scripts/dev/derive-browser-ownership.mjs --check` (both exit 0, 55 modes), then released it. Only the current Go source hash changed in the generated artifact; modes/inputs remain unchanged. Go/PG/browser tests remain NOT_RUN; no push.


## Registry guard alignment after Foundation RED

Foundation [41296883416, job 124819492325](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41296883416/job/124819492325) failed `tests/admin/live-settings-model.test.ts:143`: the old exact selector required only `TestBrowserLiveSettingsUIRealChain`, while the existing runner selects that test and `TestBrowserParityLeftovers` in one anchored alternation. Root authorized extending the guard to the exact current two-test selector, with positive checks for both names and negative checks for similar unrelated names. The single registry entry, admin build, PG fixture, race, 780-second timeout, original test selection and all other assertions remain required. Runner, fixture, original twelve specs and product are unchanged.

This is a guard-only candidate. GitHub GREEN, twelve browser cases, fixture preflight, independent acceptance, SANDBOX and LIVE are NOT_RUN. Existing order and media DTO limitations remain BLOCKED.


## Current handoff: first runtime preflight repair (2026-11-01)

- task_id: `53ea9f21-3f3b-4626-87ab-42dc853507b9-sub-leftovers-gate`; directed subtask `PR473-PREFLIGHT-FIX1`.
- current base_commit: `320f6eb370f45eb5f42dc3286e71b1464c357e7b`; clean status and exact HEAD checked before edits; same branch/worktree and test_worker / gpt-6.1-sol / high dispatch.
- Parent's intervening registry-guard changes remain untouched. No merge or push. One implementation batch only.
- Actual generated path is `output/ci-select-backend-browser/browser-ownership.json` (generator line 202), not `docs/generated/browser-ownership.json`.
- E1 only for this candidate. The historical runtime RED is a **fixture** RED, not a product RED.

Primary CI evidence, read by root and supplied with Humaux `ededffe6-360a-44f7-9cbb-ba32cfdbe1c7`: [Gates 41298127737 / job 124824185169](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41298127737/job/124824185169), merge `fea769e47252a46e4a0f86d1ff91104a8f969f8c`, artifact `13080821100`, `fixture-contract.log`: `Error: unavailable`, stack `orders-model.ts:222` -> `parseSourcedOrderSummary:200` -> `parseOrderDetail:310`; Go reports preflight exit 1. The raw error names no field. The worker did not download or rerun CI; the following field diagnosis comes from current repository predicates.

### Two deterministic defects, fixed together

| Field | Before -> after | Exact authority at base 320f6eb3 | Static conclusion |
|---|---|---|---|
| `summary.created_at` (copied into detail and v2 list row) | `2026-10-25T00:00:00Z` -> `2026-10-25T00:00:00.000000Z` | `orders-model.ts:154`: timestamp regex ends `\.\d{6}Z`; `:182-185` requires that regex and a real calendar instant; `:210` applies `date(created_at)` | Before necessarily false. After preserves the same real instant and six digits. |
| `summary.updated_at` (same consumers) | identical timestamp normalization | Same date predicate at `orders-model.ts:210`; `:211` additionally requires `updated_at >= created_at` | Before necessarily false; after remains equal to created time. |
| `summary.test_mode` | `true` -> `false` | `orders-model.ts:277`: rejects `NOT_STARTED && (test_mode || work_state != NONE)`; Go `internal/merchantorders/orders.go:403` enforces the same; `contracts/merchant-orders-v1.md:98-100`: test_mode is false if no attempt, and false with NOT_STARTED does not mean live payment is enabled | Independent deterministic defect that would remain after timestamps. `payment_state=NOT_STARTED` and `work_state=NONE` are preserved; MOCK classification is unchanged. |

No other data value, amount, count, status, template record or original assertion changed. Amounts remain 1200 minor units (whole 12 TWD), quantity one. The new comment points to the two existing constraints; it does not add or weaken an acceptance predicate.

### Complete active DTO predicate audit (static, not an executed PASS)

All references below are repository source at `320f6eb370f45eb5f42dc3286e71b1464c357e7b`. “Consistent” means the literal fixture satisfies the inspected predicates by static review; runtime acceptance remains NOT_RUN. Predicates under empty-list or inactive enum branches are explicitly distinguished from executed checks.

| Projection / fields | Every active predicate group and concrete fixture assessment |
|---|---|
| Shared order object shape, `source` | `orders-model.ts:156-169,198-209`: exact closed summary keys with no extras; mandatory source present and in `storefront|merchant_manual`. Fixture has exactly the summary keys plus `source=storefront`; detail and v2 list construct copies of this same map. Consistent. |
| `order_id`, `currency`, `total_minor` | `orders-model.ts:152,174-175,210-211`: lowercase canonical UUID, uppercase 3-character currency, safe integer money 0..10^12. Fixture ID suffix500, TWD, 1200. Consistent. |
| `created_at`, `updated_at` | `orders-model.ts:154,182-185,210-211`: six fractional digits, real calendar instant and nondecreasing time. Both fixed above; same Oct25 midnight instant. |
| `commercial_state`, `fulfillment_state`, `payment_state`, `work_state`, `test_mode` | `orders-model.ts:10,23-32,212-215,274-295`: enum membership; card cannot be AWAITING_TRANSFER/COLLECTION; NOT_STARTED requires false/NONE; CANCELLED, allocation-failure, READY, REVIEW_REQUIRED, shipped and CONFIRMED implications checked. Fixture DRAFT/MANUAL_UNASSIGNED/NOT_STARTED/NONE/false satisfies all active implications; other state branches are inactive, not runtime-proven. |
| `refunded_minor`, `refund_pending_minor` | `orders-model.ts:174-175,216`; Go `orders.go:379-389`: bounded safe money, combined not above total and NOT_STARTED requires zero refunds. Both zero, total1200. Consistent. |
| `pickup_source`, `payment_mode`, `collection_state`, `cod_surcharge_minor`, `cod_collect_minor` | `orders-model.ts:34-38,217-235,274-276`: allowed null/enums; offline mode iff collection state present; COD iff surcharge and collect amounts present; COD collect sum if active; buyer-entered pickup cannot have provider label. Fixture card plus four nulls is consistent. pay-at-pickup/bank-transfer/COD branch predicates `:236-272` are inactive. |
| Order detail top-level shape, `country`, `service_code`, `items` | `orders-model.ts:309-313`: exact summary+source+detail keys; requested ID matches; uppercase2 country; lowercase service regex; items array 1..50. Fixture TW/home and one item. Consistent. |
| Detail `totals.subtotal_minor`, `discount_minor`, `shipping_minor`, `shipping_tax_minor`, `tax_minor`, `total_minor` | `orders-model.ts:314-316,347-358`: exact six-key object; bounded money; total equals summary; item subtotal/discount/tax sums; safe addition; inclusive or exclusive total equation. Fixture subtotal1200, all discounts/taxes/shipping zero, total1200: both equations equal1200 without overflow. Consistent. |
| `destination.kind`, `country`, `recipient_name`, `phone`, `home_address`, `pickup` | `orders-model.ts:177-180,191-195,317-325`: exact destination/address key sets; kind/home-country agreement; bounded text excludes control/separator/surrogates; phone trimmed allowed characters with 6..20 digits and length6..32; home requires city/line1 and null pickup/source. Fixture home/TW, Synthetic Buyer, +886900000001 (12 digits), Synthetic city/address; optional region/postal/line2 empty. Consistent. CVS-only predicates `:326-334` inactive. |
| Detail item `sku_id`, `code`, `name`, `quantity`, `unit_price_minor`, `amount` | `orders-model.ts:338-350`: exact item/amount keys; UUID/code pattern; required bounded safe name; quantity safe1..10^9; bounded money; multiplication safe; subtotal=unit*quantity; discount<=subtotal. Fixture suffix601, MOCK-LEFTOVER, Synthetic item, qty1, unit1200, subtotal/total1200 and zero discount/tax. Consistent. |
| Detail `shipment` and `fulfillment_state` | `orders-model.ts:359-361`: non-null shipment iff MERCHANT_SHIPPED. Fixture null + MANUAL_UNASSIGNED. Consistent; shipment record-field parsers are not entered. |
| V2 list `items`, `next_cursor`, `total`, `counts`, `sessions` | `orders-v2.ts:50-69,79`: exact five top-level keys; <=10 rows; cursor string empty or canonical; counts exact eight buckets with safe nonnegative values; row count<=total; unique order IDs. One row, empty cursor/sessions, total1, all/unpaid1 and remaining0. `orders-client.ts` additionally compares total to selected bucket; default all has1. Consistent. |
| V2 row `order_number`, `recipient_masked`, `delivery_kind`, `live_sessions` | `orders-v2.ts:54-77`: source summary re-used; order number exact LC- + uppercase UUID without dashes; mask emdash or four codepoints ending***; delivery enum or unknown; session list<=100, exact id/name fields, UUID/safe text, unique IDs if nonempty. Fixture computed number, S***, home, empty live_sessions and top-level sessions. Consistent; individual session branches inactive. |
| V2 `item_summary.items`, `total_item_count`; each `product_name`, `variant_label`, `quantity` | `orders-v2.ts:83-96`: exact shapes; 1..3 items; total count safe>=1; required safe text<=200; label null or same text rule; quantity safe>=1; summed shown quantity safe and <=total. One Synthetic item/null/1 with total1. Consistent. |
| `order-actions.refund`, `fulfillment_write`, `orders_export` | `orders-model.ts:463-466`: exact3 fields, each boolean. false/false/true. Consistent. |
| `orders/{id}/shipment/history.items` | `orders-model.ts:402-406`: exact items object; array<=1000; mapped row parser and unique versions. Empty array valid; no record-specific shipment predicate entered. |
| Product `id`, `slug`, `name`, `description`, `status`, `version`, `seo_title`, `seo_description` | `catalog-v2-model.ts:97-102,110-135,172-176`: object; UUID; slug1..80/name1..120/description<=8000 strings; draft/active/archived; integer version1..MAX_SAFE_INTEGER; SEO string limits70/160. Fixture suffix600/mock-leftover/Synthetic item/MOCK catalog read/draft/v1/empty SEO. Consistent. |
| Product `options`, `skus`, `collection_ids`, `warehouse_id` | `catalog-v2-model.ts:177-179`: arrays with max3 axes/200 SKUs/10000 collection UUIDs; warehouse null or UUID. Empty axes/collection IDs, one SKU, null warehouse. Consistent; per-axis name/value validation is inactive on empty options. |
| SKU `id`, `code`, `title`, `price_minor`, `compare_at_minor`, `option_values`, `version`, `currency` | `catalog-v2-model.ts:160-165`: object UUID, code1..64/title1..200, integer price0..10^12, nullable positive compare-at, <=3 option strings1..40 if present, version1..MAX_SAFE_INTEGER, currency exactly3 chars. Suffix601/MOCK-LEFTOVER/Standard/1200/null/[]/1/TWD. Consistent. Whole-TWD domain rule also retained from sectionF/model272:1200%100=0. |
| SKU `available`, `inventory_tracked`, `max_per_order`, `keyword`, `on_hand`, `committed` | `catalog-v2-model.ts:165-167`: integer available -10^12..10^12, bool tracked, null or integer max1..999, keyword string<=16, nullable integer stock0..10^12. Fixture10/true/null/empty/10/0; stock arithmetic also10-0=10. Consistent. |
| SKU `weight_grams`, `length_mm`, `width_mm`, `height_mm`, `origin_country`, `customs_name`, `hs_candidate` | `catalog-v2-model.ts:168-169`: integer weight0..10^9; dimensions0..10^6; string max lengths2/240/12. Fixture100/10/10/10/TW/Synthetic item/empty. Consistent. |
| Image list `items`, `image_axis`, `option_images` | `image-list-contract.ts:50-67`: exact3 keys; arrays; axis null or bounded1..30 text; option count<=50; images valid, sorted role/position and contiguous when present. Fixture []/null/[] valid empty. No per-image or per-option record predicate is entered; named-media candidates remain BLOCKED and no synthetic fields are added. |
| `collections.items`, `next_cursor`; `warehouses.items` | `catalog-v2-model.ts:191-195,237-241`: objects, arrays<=100; collection cursor string empty or canonical; mapped collection/warehouse validation if nonempty. Both arrays empty; collection cursor empty. Consistent; individual record fields inactive. These projections are browser-consumed where applicable, not separately called in the existing preflight. |
| Template container and each `template_id`, `version`, `kinds`, `public_safe`, `name`, `created_at` | `live-settings-model.ts:118-149,257-277`: object/items array; ID string<=64; integer version>=1; kinds array<=4 of strings<=32; boolean public_safe; name string<=120; timestamp string<=64 accepted by Date.parse. Six records, IDs parity-leftovers/1..6; versions2 then1; kinds dm/private_reply/public_reply/recommend (legal vocabulary), max2 per row; last public-safe; signed names; valid Oct20..25 UTC strings. These dates do not require microseconds, so no change. Consistent. |

No further deterministic parser violation was found among the above literal values. This is not a claim that subsequent signed login, BFF requests, product routes or original UI assertions pass.

### Candidate hashes and actual commands

- Revised Go fixture SHA256: `db79f54b2ab901c291711bcc41a724f4b6f083de6d47b5b3f57caf25b4713d11`.
- Original twelve-case spec remains `c03599eccc37476d114e6afb2a528a9f6f3c92e1bc9ef9861cbba2a5a9b1befe`.
- Exact parser authority hashes: orders-model `6a39abd0b199732781ddff7a8c1519ff9321f4c7d7c8d93d6cea34e9051e7c31`; orders-v2 `841ef9c279eca4819dd8171931cf6cf54c876048e56cb6f0aa5bc49009eb5fe4`; catalog-v2-model `ceac5083c1b8b3b5a5cc100304ae3708d7c2be77de5eafe87a8416ecb85935c8`; image-list-contract `cf56ff76d61042e5b23793333317ea3f067fea4df910286ac2414d3112902446`; live-settings-model `47ed0e45aa20ea402827465c63666521d6a3e988ea20d01fb7a18939a3ee3442`.

| Actual command | Exit | Evidence / limit |
|---|---:|---|
| `git status --short` / `git rev-parse HEAD` | 0 | initially clean at exact320f6eb3; no other's edits reverted |
| `gofmt -w tests/foundation/browser_parity_leftovers_test.go` | 0 | source syntax/format only |
| extracted generated JS `node --check --input-type=module` through stdin | 0 | `%q`/`%s` replaced with syntax-only placeholders; parser logic NOT_RUN locally |
| `node scripts/dev/derive-browser-ownership.mjs` | 0 | 55 modes; held/released global generated-artifact lock |
| `node scripts/dev/derive-browser-ownership.mjs --check` | 0 | deterministic artifact matches |
| `git diff --check` | 0 | no whitespace error |
| `shasum -a 256` on fixture, original spec and parser authority files | 0 | hashes above |

Only3 paths changed: fixture source, actual generated ownership path and this report. All runtime tests, Go execution, Node parser execution, PG, Next, browser and GitHub rerun for this candidate are NOT_RUN; no local runtime or process residue. The existing root guard fix and all twelve original UI assertions, runner selection, 780-second timeout and business semantics remain unchanged. Signed login/GET receipts/12-case enumeration still need GitHub readback. Missing-recipient pre-order and named-media data/route gaps remain BLOCKED; unrelated WebKit failure remains separately UNKNOWN and outside this card.
