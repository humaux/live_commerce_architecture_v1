# 商品编辑 audit: functional, design and fix plan

The owner is right on both counts. The upload failure sends merchants to a button that isn't on screen. The status select can show 上架中 for a product that isn't listed. The 规格图 select looks clickable but does nothing. The layout has three separate scroll areas and a right gutter that grows with screen width.

**Evidence level:** all findings are E1 (screenshots) or E2 (source reading). I ran no browser, made no clicks and did not produce a click ledger, so live behaviour is NOT_RUN. I edited no files.

## A. Functional findings

### P0 — blocks a merchant task

**A1. The upload failure is unexplained and the recovery it names isn't on screen.** Seen in `owner-production-editor.png`.
- `lib/product-media-client.ts:71-76` does not map HTTP 413 to a "too large" code. `lib/catalog-v2-client.ts:97` already does this for collections. Caddy's 413 body isn't JSON, so the error code is lost and `ProductMediaManager.tsx:195-202` falls back to the generic `c.failed`.
- Symptom: a merchant with a 1–2 MB photo gets 「图片未能保存，请重新读取清单后再试。」. The copy implies the image list is stale, when the real cause is file size.
- The copy (`product-media-copy.ts:209`) points to a 清单 action, but:
  - The 「重新读取图片」 button (`:202`) only renders when `!list` (`ProductMediaManager.tsx:390-394`). After a 413 the list re-reads fine, so no button appears.
  - The copy says 清单 but the button says 图片.
  - `:204` clears `pending`, so the retry panel (`:397-411`) doesn't show either.
- So the merchant has zero buttons to act on. Retrying the same file fails the same way forever. The only real fix is a smaller file, and nothing says so.

**A2. Batch uploads drop files silently.** `ProductMediaManager.tsx:302-316` stops at the first failure. If a merchant picks 5 photos and #2 fails, #3–5 are never sent. The single message doesn't say how many were saved or which file failed.

### P1 — wrong or misleading behaviour

**A3. No file limits are shown before upload.**
- `mainHelp` and `detailHelp` (`product-media-copy.ts:180-182`) mention 800px and the 6× aspect ratio. They say nothing about JPEG/PNG/WebP or the size cap.
- The real cap is `MAX_PHOTO_BYTES = 2 MiB` (`image-list-contract.ts:8`). Its message (`:217`) only appears after the photo has been processed. Before PR #63, the edge limit (1 MB) was lower than this client limit (2 MB). Nothing asserts that the two stay consistent.

**A4. Errors appear far from the control that caused them.**
- The message is a plain `<p role="status">` at the top of the media manager (`:396`).
- In the owner's screenshot it sits at y≈207, while the 详情图 「新增图片」 button is at y≈775, about 570 px away inside a scroll pane. The message can be scrolled out of view.
- It has no error colour or icon, and `role=status` is polite, not `alert`. It looks like body text.
- 「图片已保存」 (`:256`) is never cleared either.

**A5. The 规格图 select looks enabled but is disabled.**
- `ProductMediaManager.tsx:439` disables it when `!axes.length`.
- `product-media.css:98-107` forces ink text on a white background and has no `:disabled` rule, so it looks identical to the enabled state (owner's screenshot; also `editor-zh-CN-1586-media.png`). Clicking it does nothing — a defect under the click-acceptance rule.
- The note 「请先新增并保存商品规格」 (`:452-453`) has no link to the 规格 section.
- `axes` comes from the **saved** detail (`ProductDocumentForm.tsx:346`). A merchant who just added specs in 规格 but hasn't saved still sees "please add specs".
- The block makes no sense for single-variant products and should be hidden or collapsed.

**A6. The status select shows the target status, not the current one.**
- `ProductDocumentForm.tsx:371-393` initialises the select from `detail.status` and then tracks local edits. Nothing distinguishes saved from pending.
- The hint 「上架中：店铺发布后，买家可以看到并购买。」 describes a state the product may not be in.
- In the owner's screenshot: 上架中 is selected, there are 0 main images, 售价 is 待填写, yet the `activeMainMissing` banner (`:335`) is absent. That banner only fires when the **saved** status is active. Likely inference (E2): the product is still a draft and the select merely shows an unsaved choice.
- With 上架中 selected, 「保存修改」 publishes (`:265`), so a "save" button changes buyer visibility.
- An active product's footer shows 「保存修改」, 「下架转草稿」 **and** 「上架」 (`:764-789`). That is three status paths (select plus two buttons), and 「上架」 is offered for a product that is already listed.

**A7. The completion list doesn't match what is enforced, and clicking an item doesn't reach its field.**
- 图片 is listed under 必填 (`:203`), but draft saves never require it. The client only blocks publishing when the **saved** status is active (`mainMissing`, `:228`, `:785`). Clicking 上架 on a draft with 0 images is sent to the server anyway.
- 「还缺必填项目: 2」 is not tied to whether 上架 is enabled.
- Clicking an item calls `focus(section)`, which focuses the **first** control in that section (`useProductEditorLayout.ts:170-172`). 商品名称 lands on the 显示状态 select (visible in `edit-zh-CN-1440-basics.png`: the select has the focus ring while 商品名称 is highlighted). 售价 and 库存 both go to the same place.
- Risk (E2): on Windows/Linux Chrome, arrow keys on a focused closed `<select>` change its value. Keyboard users could switch the status by accident.

**A8. One image-list failure freezes the whole editor and blocks leaving it.**
- `mediaBusy` is set from `busy||pending||lost||!ready||!list` (`ProductMediaManager.tsx:146`). It feeds `disabled` (`ProductDocumentForm.tsx:75-83`), which disables every fieldset, the nav and the save buttons.
- So if image loading fails or is uncertain (e.g. a 20 s upload timeout returns `uncertain`), name, price and SEO all become uneditable.
- It also sets `locked` (`:161`). `use-product-leave-guard.ts:13` then returns `false` with **no message**, so 「全部商品」 and the breadcrumb links silently do nothing.

**A9. The section nav is disabled during every upload.** `ProductDocumentForm.tsx:274` sets `disabled={disabled}` on each nav button.

### P2 — polish

**A10. The nav highlight never reaches the last section on scroll.** `useProductEditorLayout.ts:131-135` highlights the first visible section. In `edit-zh-CN-1440-scroll-end.png`, the SEO card is fully visible but 物流 stays highlighted. Clicking SEO directly does work (`:186`).

**A11. Two or three items highlight at once.** `ProductReadiness.tsx:28` gives `aria-current` to every checklist item whose key matches the active section. In the owner's screenshot, 商品图片, 图片 and 至少 3 张图片 are all highlighted.

**A12. The unsaved-changes guard works** (`use-product-leave-guard.ts`: `beforeunload` plus link capture). The only gap is the silent lock in A8.

**A13. Keyboard focus.** Nav clicks move focus into the pane (good). The focus ring is blue (`--focus`) against a green brand, and on selects it renders 3–4 px thick (`edit-zh-CN-1440-basics.png`). I did not verify roving arrow-key behaviour on the tab strip (NOT_RUN).

## B. Design findings (measured on the screenshots)

### P1

**B1. Three nested scroll areas.**
- `.pe-index` scrolls on its own (`ProductDocument.css:32`), `.pe-fields` scrolls on its own (`:117`), and the page scrolls too. The pane height is computed in JS (`useProductEditorLayout.ts:54`).
- At 1440×900 the left column is clipped at y≈815 behind the save bar; 「已完成 库存」 is cut in half (`edit-zh-CN-1440*.png`).
- The owner's screenshot shows the inner scrollbar at x≈1293, inside the content area.

**B2. The right gutter grows with the screen.**
- `.pe-document { max-width: 1280px }` (`:20`) is left-aligned while the page header spans the full width.
- 1586 px (`editor-zh-CN-1586-media.png`): the cards end at x=1523 but 「全部商品」 ends at 1562, leaving a 39 px misalignment. In the owner's screenshot the gutter is ≈190 px (cards end at 1287 in a 1487 px area).
- Inside cards, inputs cap at 640 px. At 1440, the basics card is 962 px wide (453→1415) with inputs at 473→1113, so ≈30% of every card is empty on the right.

**B3. The media card has no title, and the heading levels are flat.**
- The media section (`ProductDocumentForm.tsx:339-351`) has no 「商品图片」 heading, even though the nav uses that name. 主图, 规格图 and 详情图 are all `<h2>` (`ProductMediaManager.tsx:368`, `:431`), the same level as 「基本信息」.
- The owner's screenshot reads as three peer cards squeezed into one box, with the error text as its first line.

**B4. Emphasis is reversed in the completion card.**
- 「待填写」 is 12 px muted grey; 「已完成」 is green bold (`ProductDocument.css:90-103`). The items that need action are the least visible.
- 「还缺必填项目: 2」 is in the success green (`.pe-index p { color: accent }`, `:104-107`).

**B5. On mobile, 「向左滚动 · 向右滚动」 looks like links but isn't.** It's a non-interactive `<p>` (`packages/ui/src/Presentation.tsx:215`), turned brand green by the same `.pe-index p` rule (`edit-zh-CN-390-media.png`).

### P2

**B6. Inconsistent button styles.**
- 「新增图片」 is dashed, weight 600 (`product-media.css:135`).
- 「添加规格（颜色、尺寸…）」 is a solid border (`edit-zh-CN-1440-variants.png`).
- 物流 and 搜索引擎优化 are `+`/`−` `<details>` toggles while every other section is always open.
- These are three different ways to show an "add or expand" action.

**B7. Three control widths and inconsistent text weights.**
- The media select is 380 px (`product-media.css:95`), form inputs are 640 px, and the shipping dimensions use a full-width two-column grid. In `edit-zh-CN-1440-scroll-end.png` the 高 input ends at x≈926 while SEO inputs end at x≈1113.
- The blocking note 「请先新增并保存商品规格」 is 14 px ink, louder than the 12 px muted hints around it, while the real error (A4) is the same plain style.

**B8. Too many borders.** Page → card → `.pm-option` card → `.pm-photo` tile → `.pm-recovery` box is up to four bordered layers. The left column is two more separate cards (nav, then completion, 18 px gap) next to one long stack.

**B9. Too much helper text.** Each media sub-block has a 12 px hint, a count, an empty-state line and a button, about 4 lines of chrome per empty block. The empty media card is 640 px tall in the owner's screenshot.

**B10. Mobile (390, `edit-zh-CN-390-media.png`).**
- About 430 px of header, tabs and completion toggle, plus a ≈96 px save bar, leave ≈315 px (37% of 844) for the editing pane.
- Tile buttons (← → ×) are 40 px wide.
- 「全部商品」 sits on its own row.

## C. Fix plan (3 PR-sized units)

**PR-1: Media failure and recovery** (`ProductMediaManager.tsx`, `product-media-client.ts`, `product-media-copy.ts`, `product-media.css`)
- Map 413 to `too_large` and give it its own copy, e.g. 「照片超过 2 MB，请压缩或换一张」. Rewrite `failed` to name the real button.
- Always show 「重新读取图片」 and 「重试」 after a definite failure.
- Show errors inline under the picker that triggered them, with `role=alert` and error styling.
- Report batches as "已保存 n/m，<file> 失败".
- Show "JPEG/PNG/WebP，单张 ≤2 MB" before upload.
- Stop image-list failures from disabling non-media fields and nav. When navigation is locked, show a message instead of silently blocking.
- Acceptance (Playwright, real clicks; the route stub only prepares the fixture and is commented as such):
  - Stub the upload route to 413, then use the real file chooser with `setInputFiles` on 详情图.
  - Assert the alert is within 120 px of that picker and contains the size copy.
  - Click 「重新读取图片」 and assert the list reloads.
  - Fill 商品名称 during the failure, save, reload, and assert the name persisted.
  - Click 「全部商品」 while locked and assert a visible message.

**PR-2: Status, completion and 规格图 semantics** (`ProductDocumentForm.tsx`, `ProductReadiness.tsx`, `useProductEditorLayout.ts`)
- Show the saved status as a chip, plus a "将改为…（未保存）" marker when the select differs.
- Hide 「上架」 when the product is already active; disable it until the required items are complete, and list what's missing.
- Make each completion item focus its own field (name → name input, price → price input). Only the nav gets `aria-current`.
- Give the 规格图 select a visible disabled style, or hide the block when there are no specs, and add a 「去新增规格」 button that focuses 规格.
- Make the nav highlight the last section at scroll end.
- Acceptance:
  - Change the select on a draft to 上架中 and assert the "未保存" marker; reload and assert the select shows 草稿.
  - Click every completion item and assert `document.activeElement` is that field.
  - On a product with no specs, assert `getComputedStyle(select).opacity < 1` or the block is absent; click 「去新增规格」 and assert the 规格 section has focus.
  - Scroll to the end and assert the SEO nav item has `aria-current`.

**PR-3: Layout and visual system** (`ProductDocument.css`, `product-media.css`, `Presentation.tsx` hint)
- Use a single document scroll with a sticky left column and sticky save bar, and remove the JS pane height.
- Remove or align the 1280 px cap so the card's right edge matches the header.
- Add 「商品图片」 as the card's h2, with h3 sub-blocks.
- Use one secondary-button style for every add action, one input width, and one card style without nesting.
- Make pending completion items the emphasised state.
- Render the mobile scroll hint as real buttons or drop it.
- Acceptance:
  - At 1440, 1586 and 1920: `|card.right − header.right| ≤ 1px`.
  - `.pe-fields` and `.pe-index` computed `overflow-y` is not `auto`.
  - Clicking each nav item scrolls its section to within the viewport and sets `aria-current`.
  - At 390, every element styled as a link is a `<button>` or `<a>`, and clicking it changes the visible tab.
  - Re-run the zh-CN/zh-TW/en screenshot matrix and attach the click ledger.