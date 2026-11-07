# PR #440 C7 outstanding-frame triage (2026-11-04)

Writer: opus-orchestrator-orca-sub-440 (ui_worker). Base: merge of origin/r3/integration `3696e21e3` into `80cd1d4db` = `b5735c224`.
Scope: the 41 outstanding frames of #488 plus a re-check of the 4 DEFERRED (#375) and 4 BLOCKED (live room) frames.

Figma source: file `iJ9lDdFIXgTlKkt5Tvrc0X`, read 2026-11-04 through the Fleet Bridge local CLI
(`output/integrator/tools/figma-local/call.mjs`: `get_screenshot` for the six b-home frames, one read-only `use_figma` text dump of all 49 frames).
The official Figma MCP (`get_design_context` / `get_screenshot` / `get_metadata`) failed 4 times in a row ("transport dropped mid-call",
"upstream ... reset reason: overflow"; `whoami` OK), so the bridge was used as the equivalent read. Screenshots and the text dump are
cached under the git-ignored `output/parity-buyer-browse/figma/`. Figma file version id was not read (UNKNOWN); the read time is the version pin.

Evidence level: E1 (source reading + frame text). No app or test was run locally (forbidden); reachability is from fixture/handler source.

Fixture shorthand: `SFR` = `tests/foundation/browser_storefront_test.go` (the real-stack storefront fixture behind `tests/storefront/shop-real-gate.mjs`).

## Verdict counts

| Group | Outstanding | REACHABLE | BLOCKED |
|---|---:|---:|---:|
| b-home | 6 | 4 | 2 |
| b-product | 17 | 13 | 2 (+2 already asserted, see note) |
| b-cart | 18 | 12 | 6 |
| **Total** | **41** | **29** | **10** (+2 already asserted) |

Note b-product: `782:4249` (mobile sort sheet) and `51:1380` (spec bottom sheet) were NOT_COVERED at 5973eb440 but are `plan: "assert"` in the
current spec (`tests/storefront/buyer-browse-parity.spec.ts:210` for 782:4249, `:188` for 51:1380, at `80cd1d4db` before this round) after the -fix/-cursor rounds. They need no new work in this
triage; their COVERED status still comes only from the CI ledger. `51:1381` is a partial (description panel only) and stays in the list.

## b-home (6)

| Figma node | Frame | Route | Data needed | MOCK/fixture reaches it? | Write-path owner | Verdict |
|---|---|---|---|---|---|---|
| 4:9192 | Buyer Desktop · 歡迎來到大夢甄選 · SHOPLINE | `/[locale]` | store name (welcome title), hero heading/subheading/CTA, published collections (chips), product grid | Yes: published design with `profile.name` + hero + `product_grid` (SFR:304, :313, :316), two collections (SFR:128-129). Demo copy (大夢甄選, 把喜歡的…, 托特包 placeholder) is replaced by tenant data / generic UI copy. | C7: `app/[locale]/page.tsx`, `components/HomeSections*`, new `components/browse/**` | REACHABLE |
| 4:9296 | Buyer Mobile · 歡迎來到大夢甄選 · SHOPLINE | `/[locale]` | same | same | same | REACHABLE |
| 6:19933 | Buyer Desktop · 日常隨行系列 · SHOPLINE | `/[locale]/collections/home-fragrance` | collection title, product count, product cards, chips | Yes: `home-fragrance` (2 active) / `tableware` (11 active) collections with members (SFR:128-129, :204-216). Count is exact on a single-page collection. | C7: `app/[locale]/collections/**`, `components/ListBody*`, new `components/browse/**` | REACHABLE |
| 6:19984 | Buyer Mobile · 日常隨行系列 · SHOPLINE | same | same | same | same | REACHABLE |
| 1253:17135 | DESIGN · Buyer Desktop · 店鋪暫停營業 · 狀態 | `/[locale]` (closed) | a "paused by owner" discriminator + store name for the header/footer | Closed state IS reachable (control `/unpublish`, SFR:348; used at `tests/storefront/shop-real-gate.mjs:301`). But the frame copy 「店主暫停了營業」 asserts a fact the storefront cannot know: unpublished, never published, unbound and suspended all collapse to one 404 (`contracts/storefront-v2.md:53`; `apps/storefront/lib/shop-upstream.ts:100`, `:111` → `{state:"closed"}`), and the store name is not readable while closed (layout falls back to `defaultDesign`, `app/[locale]/layout.tsx:51`). Two other modules' gates pin the generic closed copy for unbound/unpublished stores (`tests/storefront/store-domains-gate.mjs:45`, `tests/storefront/storefront-publish-gate.mjs:41`). The render lives in `app/[locale]/not-found.tsx`, which is not a C7 write path. | not-found.tsx: unassigned (not C7, not STATES) | BLOCKED (contract gap: no paused discriminator / no closed-store name; needs owner ruling: (a) contract adds a non-PII `closed_reason`, or (b) owner accepts the paused copy for every closed host and the other two gates' copy is re-pinned by their owners) |
| 1253:17159 | DESIGN · Buyer Mobile · 店鋪暫停營業 · 狀態 | same | same | same | same | BLOCKED (same) |

## b-product (17)

| Figma node | Frame | Route | Data needed | MOCK/fixture reaches it? | Write-path owner | Verdict |
|---|---|---|---|---|---|---|
| 102:2163 / 102:2232 | 日常斜背小包 D/M | `/[locale]/products/{slug}` | single-SKU product page: back link, title, price, one default-variant chip (標準款), qty 1, note, 「加入購物車 · price」, 商品說明 | Yes: single-SKU active products `dripper` (SFR:152) and `item01..24` (SFR:156). Demo titles/prices are replaced by merchant data; the six frames are one layout. | C7: `app/[locale]/products/**`, `components/ProductBuy*` via new `components/product/**` | REACHABLE |
| 104:2209 / 104:2278 | 輕量帆布托特包 D/M | same | same single-SKU shape | same (`dripper`, `item*`) | same | REACHABLE |
| 104:2553 / 104:2622 | 迷你收納袋 D/M | same | same | same | same | REACHABLE |
| 104:3072 / 104:3141 | 簡約提袋 D/M | same | same | same | same | REACHABLE |
| 104:3416 / 104:3485 | 米白托特包 D/M | same | one-axis product with a selected value chip (米白) | Yes: one-axis products `candle` (Size, SFR:143) and `scarf` (Color, SFR:146) | same | REACHABLE |
| 104:3758 / 104:3827 | 輕便收納包 D/M | same | single-SKU shape | same (`dripper`, `item*`) | same | REACHABLE |
| 51:1381 | DESIGN · Buyer Mobile · 商品說明與配送 | `/[locale]/products/{slug}` | merchant description + generic delivery note (home delivery / CVS pickup, fee at checkout) | Description: yes (every SFR product has one). The description task is partially built (`components/product/MobileProductDescription.tsx`); the delivery paragraph is static UI copy. Whether a store's enabled methods may be stated before checkout is not re-verified (UNKNOWN) → copy must stay generic. | C7 | REACHABLE (partial today) |
| 66:1752 | Buyer Desktop · 部分商品可購買 | frame copy is the live-claim result (「本次可買」「仍保留在此連結」) → `/[locale]/claim` | claim preview with one sold-out line | Yes: claim sold-out line in MOCK (`tests/storefront/claim-checkout.mjs:234-238`; `tests/foundation/claim_direct_checkout_test.go:29-35`); UI states exist in `components/ClaimLink.tsx:197`, `:331-333`. | C8 PARITY-BUYER-AFTER (`/claim`, `ClaimLink*`; `FIGMA-INVENTORY.md` line 87) | BLOCKED (write-path transfer: the route and component are C8's; integrator must move 66:1748–1753 to C8 or grant C7 `ClaimLink*`) |
| 66:1753 | Buyer Mobile · 部分商品可購買 | same | same | same | same | BLOCKED (same as 66:1752) |

## b-cart (18)

| Figma node | Frame | Route | Data needed | MOCK/fixture reaches it? | Write-path owner | Verdict |
|---|---|---|---|---|---|---|
| 102:2267 / 102:2324 | 購物車 · 日常斜背小包入口 D/M | `/[locale]/cart` (entered from a product) | the just-added line (spec, qty, subtotal, remove), shipping note, 前往結帳, back to 商品詳情, then the cart list | Yes: real add-to-cart of `dripper` / `item*` then cart (cart CAS + `X-Buyer-Context` already exercised in the spec). Entry context (which product) is UI state, no new field. | C7 `components/CartView*` + new `components/cart/**`; `cart/page.tsx` state branches are STATES' (merged) | REACHABLE |
| 104:2313 / 104:2371 | 購物車 · 輕量帆布托特包入口 D/M | same | same | same | same | REACHABLE |
| 104:2657 / 104:2715 | 購物車 · 迷你收納袋入口 D/M | same | same | same | same | REACHABLE |
| 104:3176 / 104:3234 | 購物車 · 簡約提袋入口 D/M | same | same | same | same | REACHABLE |
| 104:3520 / 104:3578 | 購物車 · 米白托特包入口 D/M | same | one-axis line (規格 米白) | Yes: `candle` / `scarf` variants | same | REACHABLE |
| 104:3862 / 104:3920 | 購物車 · 輕便收納包入口 D/M | same | single-SKU line | same | same | REACHABLE |
| 66:1748 / 66:1749 | 已在購物車中 D/M | frame copy 「本次認領」 = claim result → `/[locale]/claim` | claim merge into an existing cart | Yes (`tests/storefront/claim-checkout.mjs:162`) | C8 (`ClaimLink*`) | BLOCKED (write-path transfer, same as 66:1752) |
| 66:1750 / 66:1751 | 目前無法購買 D/M | same | claim line sold out (所需數量已售完) | Yes (`claim-checkout.mjs:234-238`) | C8 | BLOCKED (write-path transfer) |
| 782:3904 / 782:4087 | 結帳 · 共用購物車脈絡 D/M | `/[locale]/checkout` | checkout page in the shared cart context | Checkout reachable in MOCK, but the frame is the checkout screen (CheckoutFlow, `components/checkout/**`) | PARITY-CHECKOUT (`FIGMA-INVENTORY.md` line 74 running-card write paths) | BLOCKED (owner: #488 asks the integrator to assign; not a C7 write path) |

b-cart: 12 REACHABLE, 6 BLOCKED.

## Restated totals

| Group | REACHABLE | BLOCKED | Already asserted (no new work) |
|---|---:|---:|---:|
| b-home (6) | 4 | 2 | 0 |
| b-product (17) | 13 | 2 | 2 |
| b-cart (18) | 12 | 6 | 0 |
| **41** | **29** | **10** | **2** |

## Re-check of DEFERRED and BLOCKED verdicts on the new base

| Frames | Previous | Re-check on `b5735c224` | Still holds? |
|---|---|---|---|
| 26:850, 26:1117, 29:641, 29:670 (聯絡商店 / 訊息已送出) | DEFERRED → #375 | PR #375 is still OPEN (`gh pr view 375`: "feat(storefront): persist encrypted contact messages"); it is backend-only; no storefront contact route exists (`app/[locale]/pages/[slug]` serves merchant pages only). | Yes |
| 6:21646, 6:21711, 6:21771, 6:21825 (直播回放 / 秋日選物直播) | BLOCKED (no route) | No live/replay route under `apps/storefront/app/[locale]/` after the merge; frame copy itself says 「原型，未真實寫入」「回放播放尚在規劃中」. | Yes |

## Plan for this round (b-home)

RED then GREEN for 4:9192, 4:9296, 6:19933, 6:19984 on the real routes, three locales × 1440×900 / 390×844, existing money/stock/privacy checks kept.
1253:17135 / 1253:17159 stay BLOCKED with the evidence above.

## RED witness (b-home)

| Head | Run / job | Result |
|---|---|---|
| `b02cb6de4` (RED, tests only) | Gates 41834320978, gate (bundle:02) job 126591642589 | INVALID RED: `TestBrowserStorefront` killed at `browser_storefront_test.go:393` (`signal: killed`, 723.08s); missing controls waited out Playwright's default 30s per action. No manifest written. First directed fix → `bba5a337c`. |
| `bba5a337c` (RED, tests only) | Gates 41839460034, gate (bundle:02) job 126609241540, artifact `gate-browser-storefront` 13329527479 | Causal RED: `TestBrowserStorefront` FAIL 341.47s, `exit status 1` from the parity assertion. Manifest: `COVERED 26 · NOT_COVERED 37 · DEFERRED 4 · BLOCKED 6 of 73 frames; 275/287 ledger operations passed`. The only 12 FAIL rows are the new anchors: `4:9192`/`4:9296` "home search field" and `6:19933`/`6:19984` "collection search field" × zh-TW/zh-CN/en (`expect(locator('main').getByRole('searchbox', { name: '搜尋商品' })).toBeVisible()` timeout). 18 dependent subcases NOT_RUN by design. 1253:17135/17159 BLOCKED. |
| `bba5a337c` | same run, gate (bundle:07) job 126609240011 | `TestBrowserSettingsWizardRealChain` FAIL (25.64s). Not in this diff (tests-only storefront spec change); the same bundle passed at `b02cb6de4`. Cause UNKNOWN, not touched (other module). |

## Integrator rulings 2026-11-04 (after b-home GREEN 3b9dc194c)

- 1253:17135 / 1253:17159 paused store: DEFERRED to #545; current closed copy kept, no fabricated paused state.
- 66:1748–66:1753 claim-link result states: DEFERRED, "C8 /claim ownership, integrator transfer"; no /claim file edits.
- 782:3904 / 782:4087 checkout in cart context: DEFERRED, owned by PARITY-CHECKOUT.
- `apps/storefront/components/CollectionChips.tsx` approved as a C7 write path.
- Accepted deltas: shared shell header/announcement; collection breadcrumb, description and sort/price controls; chip colour = merchant accent.
  P2 to the Figma sweep (not changed): chip shape (pill vs rectangle); legacy 「查看全部」 link on the merchant featured rail.
- Contact (26:850, 26:1117, 29:641, 29:670): #375 merged at 2faa925d0 (merged here as e706aa737). Go `POST /v1/buyer/contact-messages`
  and BFF `POST /api/buyer/contact-messages` exist (contracts/storefront-v2.md:559, apps/storefront/lib/contact-contract.ts); no storefront
  form UI exists yet → now REACHABLE, planned as round (c) after b-product and b-cart. Still DEFERRED in the spec until that round.

Round order: (a) b-product 13 reachable, (b) b-cart 12 reachable, (c) contact 4.

## Round (a) b-product (13 reachable frames)

Writer: ui_worker (Claude Opus 5.5) after the b-home handover. Base: `520b02972` (origin/r3/integration unchanged since `e706aa737`).
Figma read: the cached bridge screenshots of 102:2163 / 102:2232 / 104:3416 / 51:1381 and the frame text dump (`figma/texts.json`, git-ignored);
the six single-SKU pairs share one layout (text dump), 104:3416/3485 differ only by the selected value chip.

### Frame -> fixture mapping (demo products are a data gap)

| Frames (D / M) | Figma demo product | Fixture product (SFR) | Shape | Route |
|---|---|---|---|---|
| 102:2163 / 102:2232 | 日常斜背小包 NT$980 | `dripper` Ceramic Dripper Set (SFR:152) | single SKU, in stock | `/[locale]/products/ceramic-dripper-set` |
| 104:2209 / 104:2278 | 輕量帆布托特包 NT$1,280 | `item01` Everyday Item 01 (SFR:156) | single SKU, in stock | `/[locale]/products/item-01` |
| 104:2553 / 104:2622 | 迷你收納袋 | `item02` Everyday Item 02 | single SKU, in stock | `/[locale]/products/item-02` |
| 104:3072 / 104:3141 | 簡約提袋 | `item03` Everyday Item 03 | single SKU, in stock | `/[locale]/products/item-03` |
| 104:3758 / 104:3827 | 輕便收納包 | `item04` Everyday Item 04 | single SKU, in stock | `/[locale]/products/item-04` |
| 104:3416 / 104:3485 | 米白托特包 (chip 米白) | `scarf` Wool Knit Scarf, Color = Oat preselected (SFR:146) | one axis | `/[locale]/products/wool-knit-scarf` |
| 51:1381 | 焦糖托特包 description + delivery | `candle` (the existing 4:9532 / 51:1380 product) | one axis | `/[locale]/products/cedar-fig-candle` |

Each mapped product page is reached by a real click on its search-result card, in zh-TW / zh-CN / en at 1440x900 and 390x844.

### Adopted (integrator ruling 2026-11-04: one buy card, add-to-cart AND buy-now on every product)

Default-variant tag 「標準款」 (products without option axes), quantity, the note 「商品規格與金額以本頁選擇為準；收件資料在結帳時填寫。」,
the description card, the desktop back link 「‹ 所有商品」 (replaces the breadcrumb trail), the phone header bar (back + title), the phone
bottom action (single SKU: 「加入購物車」 adds directly), and the 51:1381 delivery section in the description task.

### P2 for #546 (frame detail not implemented; the master frame or a fact rule wins)

- Price inside the add-to-cart label (「加入購物車 · NT$980」, desktop single-SKU frames and 104:3416): not adopted. 4:9441/4:9532 assert the exact
  label 「加入購物車」 (`add-to-cart + buy-now buttons` step, `toHaveText(c.addToCart)`).
- Buy-now absent in the single-SKU frames: not adopted. Integrator ruling: one buy card; 4:9441 asserts 「立即購買」.
- 104:3485 (one axis) phone bottom action 「加入購物車」: kept 「選擇規格」. The 51:1380 `spec dock entry` step asserts the options dock on the
  one-axis master product.
- Phone bottom bar: the frames show the button alone; the dock keeps the price beside it (one dock layout with 4:9532).
- Phone note text: 102:2232 and the other mobile frames say 「加入購物車後，可在結帳時填寫收件資料並選擇配送方式。」. The ruling names the desktop
  note, so one note is shown on every viewport.
- Phone single-SKU pages keep the 4:9532 entries 「選擇規格」 and 「商品說明與配送」 (not drawn in the single-SKU frames).
- Quantity is the 4:9441 stepper (frames draw a static 「數量 1」).
- Desktop frames also draw a page-level title above the grid and the buy side as a bordered card: not in the ruling's list, not built.
- 51:1381 delivery copy is kept generic: 「可選宅配或門市取貨。」 is replaced by the existing 「可用方式依收貨地點而定，請以結帳時確認的選項為準。」, because
  a store's methods are store-dependent (`contracts/storefront-v2.md:123`, "CVS pickup when the merchant enables it") and the page does
  not know them before checkout. The pickup sentence reuses the accepted 51:1380 copy 「門市取貨：結帳時選擇超商品牌，填寫門市號碼（店名、地址可不填）。」.
- The phone shell header stays above the product header bar (frames put the bar at y=0). This is the accepted shell-header delta from b-home.

### RED witness (b-product)

| Head | Run / job | Result |
|---|---|---|
| `b1f59ad8d` (RED, tests only) | Gates 41850176970, gate (bundle:02) job 126647604139, artifact `gate-browser-storefront` 13336252595 | Causal RED: `TestBrowserStorefront` FAIL 331.85s, `exit status 1` from the parity assertion (no kill). Manifest: `COVERED 30 · NOT_COVERED 25 · DEFERRED 14 · BLOCKED 4 of 73`; ledger 388/397 PASS. The only 9 FAIL rows are new: `102:2163` (1440x900) and `102:2232` (390x844) "back link" × zh-TW/zh-CN/en (`getByTestId('product-back').getByRole('link', { name: '所有商品' / 'All products' })` not visible, 10s), and `51:1381` "description delivery section" × zh-TW/zh-CN/en. The 10 other mapped frames are NOT_RUN by design (first-product anchor failed); 4:9441/4:9532 rows all PASS. |

## Round (b) b-cart entry (12 reachable frames)

Writer: ui_worker (Claude Opus 5.5), base `b79f61c4e`. Figma read: bridge `get_screenshot` of 102:2267 / 102:2324 (cached, git-ignored) and the
frame text dump. The screenshots show only the cart list (the master 4:9668 / 4:9746 layout); the entry layer (‹ 商品詳情, 購物車 · {product},
規格 / 數量 / 商品小計 / 移除商品, 運費依配送方式於結帳時計算。, 前往結帳) is in the text dump, so it is built as a panel above the list.

### Frame -> fixture mapping (same products as round (a))

| Frames (D / M) | Figma demo product | Fixture product (SFR) |
|---|---|---|
| 102:2267 / 102:2324 | 日常斜背小包 | `dripper` |
| 104:2313 / 104:2371 | 輕量帆布托特包 | `item01` |
| 104:2657 / 104:2715 | 迷你收納袋 | `item02` |
| 104:3176 / 104:3234 | 簡約提袋 | `item03` |
| 104:3520 / 104:3578 | 米白托特包 (規格 米白) | `scarf` (Oat) |
| 104:3862 / 104:3920 | 輕便收納包 | `item04` |

Flow per locale × viewport, in its own browser context (fresh cart; the 4:9668 / 4:9746 one-line baseline is untouched): product page →
real add-to-cart (phone single-SKU: the bottom dock) → drawer 「查看購物車」 → entry panel asserts → 「‹ 商品詳情」 back click. The last product
clicks 「移除商品」 and reloads (line stays removed). The entry panel is the bounded anchor; a missing panel marks the rest NOT_RUN.

### RED witness (b-cart)

| Head | Run / job | Result |
|---|---|---|
| `33fbf5cd6` (RED, tests only) | Gates 41868361592, gate (bundle:02) job 126711128890, artifact `gate-browser-storefront` 13344638359 | Causal RED: `TestBrowserStorefront` FAIL 392.39s, `exit status 1` from the parity assertion (no kill). Manifest `COVERED 43 · NOT_COVERED 12 · DEFERRED 14 · BLOCKED 4 of 73`; ledger 673/679 PASS. The only 6 FAIL rows are the new anchor `102:2267` (1440x900) / `102:2324` (390x844) "cart entry panel" × zh-TW/zh-CN/en (`getByTestId('cart-entry').getByRole('heading', { name: '購物車 · Ceramic Dripper Set' })` not found, 10s). The add-to-cart + 「查看購物車」 steps PASS; the other 10 frames NOT_RUN by design. Other jobs green (receipts / required fail downstream of bundle:02); Foundation 41868362527 success. |

### P2 for #546 (b-cart)

- The frames' screenshots draw no entry panel (only the text layer has it); the panel is placed above the master list, which keeps 4:9668 / 4:9746 unchanged.
- Phone 「繼續購物」 inside the entry panel: not added; the master list already has 「繼續選購」 (4:9746).
- The entry panel sits outside the `cart-page` host so host-scoped cart gates (order-gate, 51:1380) still see exactly the list.
- Entry context is tab-local `sessionStorage` (one SKU id written by ProductBuy after a successful add); a plain visit to /cart in the same tab
  after an add also shows it. No URL or API field.

## C7 contact continuation 2026-11-07 — integrator v2 adoption

Integrator `output/fleet-control-20261107/ui/C7-DESIGN-ADOPTION.json` SHA256 `631373d3ec3e7a4c0b01dd316edc8c3837844415245a605e32de222a05a86587` adopts historical 26:850 → v2 1468:100491, 26:1117 → 1468:100543, 29:641 → 1468:100679, 29:670 → 1468:100717. Root inspected the four current exports with respective SHA256 `dde5c0e729d83e031ee9ac2565bf6882bb2791d1f502964e9bc465e87407f7a3`, `b383b22ec0e5cc8928a8a7581dfe67ee752ad80891c9982b1184f1b36ed1f191`, `58e136c16a5a795731368b8c931848305282bee59a155bb090ec2329de71185e`, `e8e41dbdbdf002412e68f8aaabb1601ffe2cc09ba5a831ae1460e5c363f66438`. Version UNKNOWN, image inspection E2 only, not acceptance. Original SHOPLINE frames and v2 confirmation/error/throttle/pending frames were also read and viewed; full local hashes and evidence are in `fleet-20261107/CONTACT-EVIDENCE.md`.

Pending copy delta: 1468:101014 text `I1468:137428;1429:34287` / 1468:101061 text `I1468:137442;1429:34287` (“請先查詢結果，暫勿再次送出。”) bind fill `VariableID:13:307`; action text `I1468:101059;3:52` / `I1468:101098;3:52` (“查詢結果”) bind fill `VariableID:13:311`. Integrator directs “確認送出結果” and “使用原訊息確認結果，請勿另建訊息重送。” and corresponding zh-CN/en, because the real operation is explicit same-key/same-body POST, not a read-only lookup. This is a design-sync candidate, not a Figma write. After refresh the original body is unavailable by the no-PII-storage rule; uncertainty remains visible and no recovery POST is issued without it.
