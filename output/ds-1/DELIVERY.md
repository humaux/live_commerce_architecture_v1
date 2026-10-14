<!-- Purpose: delivery record of unit DS-1 (admin design-system foundation, redesign W2): what was built, token table, component states, alias list, gates, NOT_RUN and risks.
Depends on: docs/design/page-inventory/reference-shopline-admin.md and NEEDS.md sections 1 and 3 (read from the page-inventory worktree), packages/ui, apps/admin, scripts/dev/test-local.sh, docs/delivery/GATES.md.
Used by: the integrator (merge decision), the W3 page-migration units (token and component contract). -->
# DS-1 delivery: admin design-system foundation

| Field | Value |
| --- | --- |
| Role | ui_worker (write) |
| Branch / worktree | `unit/ds-1-foundation` / `.worktrees/ds-1` |
| Base | `origin/r3/integration` = `92d736612` |
| Allowed paths used | `packages/ui/**`, `apps/admin/{components,src,app/[locale]/ui-gallery}/**` (new files only), `tests/ui/**`, `tests/admin/ui-gallery-*.mjs`, `scripts/dev/{test-local.sh,test-node.sh,release-gate.sh,check-hex-ratchet.mjs}`, `docs/delivery/GATES.md`, `tests/ci/pr-modes.test.mjs` (one list entry), `tests/admin/shell-registry.test.ts` (one named exemption, see risks) |
| Evidence level | **NOT_RUN for every executable gate.** Nothing was executed (owner rule: tests run only in CI; this environment also refuses to run scripts). Everything below is DESIGN + static review. |

No existing page was migrated. Legacy `--ui-*` values are unchanged, `AppShell`/`Presentation` are untouched, the storefront is untouched.

## 1. Token table (`packages/ui/src/tokens.css`, `--ds-*`)

Reference value = measured SHOPLINE (reference-shopline-admin.md section 6). Where the reference fails WCAG the DS value is darker and the reason is stated; every pair below is pinned by `tests/ui/ds-tokens.test.mjs`.

| Token | DS value | Reference | Note |
| --- | --- | --- | --- |
| `nav-bg` / `nav-active` / `nav-logo-bg` | `#373d4c` / `#494f5d` / `#4c5561` | same | dark sidebar |
| `nav-text` / `nav-text-muted` | `#f4f6f9` / `#c4cad6` | white / light grey | 10:1 / 5:1 on nav |
| `nav-accent` / `nav-accent-text` | `#4492d1` / `#9cc9f5` | (brand) | active bar 3.2:1 on nav; text 4.7:1 on active item |
| `topbar-bg` | `#fdfdfd` | `#FDFDFD` | |
| `page` / `surface` / `surface-muted` | `#f8f8f8` / `#fff` / `#f3f3f3` | same | |
| `brand` | `#4492d1` | `--primary` | decorative only (white text on it is 3.3:1) |
| `primary` / `primary-hover` | `#2a6fb0` / `#225e96` | `#4492D1` / `#4C9EE3` | **darkened**: 5.3:1 with white text; still "blue = main action" |
| `link` / `link-hover` | `#2a6fb0` / `#1f5a91` | `#3E8CD7` | **darkened**: 4.9:1 on `page` |
| `toggle-on`, `segment-on` | `#3f7f32`, `#2a6fb0` | `#74B666`, `#5D86DF` | **darkened**: white 是 on `#74B666` is 2.6:1 |
| `success-text/bg`, `warn-text/bg/soft`, `danger(-text/-bg/-hover)`, `info-text/bg`, `neutral-text/bg` | see file | `#67A98E`, `#FAE2A9` | one text+background pair per tone, each >= 4.5:1 on white and on its own bg |
| `text-1` / `text-2` / `text-3` | `#333` / `#5b5b5b` / `#6b6b6b` | `#333` / `#5B5B5B` / `#777` | `text-3` **darkened** (`#777` is 4.48:1 on white) |
| `border` / `border-input` | `#ddd` / `#8c8c8c` | `#DDD` / `#CCC` | input border **darkened** to hold 3:1 as a control boundary |
| `focus` / `focus-on-dark` | `#1d4ed8` / `#9cc9f5` | n/a | 2 px ring, 2 px offset; 6.7:1 on white, 6.3:1 on `page`; dark ring 6.2:1 on `nav-bg`, 4.7:1 on `nav-active` |
| spacing | `space-1..12` = 4, 8, 12, 16, 20, 24, 32, 40, 48 px | | 4-based |
| radius / shadow | `radius-sm/md/lg/pill` = 2/4/8/999; `shadow-1/2/3`, `scrim` | card radius ~4 px | |
| type scale | `text-xs..3xl` = 12/13/14/16/18/22/28 px | | |
| z-index | `base 0, sticky 20, topbar 25, nav 30, drawer 40, dialog 60, toast 70` | | strictly ordered |
| sizes | `nav-width 260px` (248-270), `nav-item-height 48px`, `topbar-height 50px`, `target 44px`, `control-height 36px` | 270 / 52 / 50 | |

## 2. Alias list (legacy names kept; current pages do not change)

`:root` still defines every `--ui-*` token with its **old value** (frozen by test). Opt-in remap: put `data-ds="2"` on a page wrapper and the legacy names resolve to the new tokens:

`--ui-primary/-brand/-brand-text` -> `--ds-primary`; `--ui-primary-hover` -> `--ds-primary-hover`; `--ui-primary-soft` -> `--ds-primary-soft`; `--ui-link` -> `--ds-link`; `--ui-rail` -> `--ds-nav-bg`; `--ui-rail-selected` -> `--ds-nav-active`; `--ui-rail-active` -> `--ds-nav-accent-text`; `--ui-page` -> `--ds-page`; `--ui-surface` -> `--ds-surface`; `--ui-border` -> `--ds-border`; `--ui-text` -> `--ds-text-1`; `--ui-muted` -> `--ds-text-2`; `--ui-danger/-success/-warning/-info` -> `--ds-<tone>-text`; `--ui-neutral/success/warning/danger/info-soft` -> `--ds-<tone>-bg`; `--ui-focus` -> `--ds-focus`; `--ui-radius` -> `--ds-radius-md`; `--ui-text-body/-helper` -> `--ds-text-md/-xs`; `--ui-rail-width` -> `--ds-nav-width`.
Not remapped (same px already): `--ui-space-*`, `--ui-target`, `--ui-control-compact`, `--ui-heading-*`, `--ui-field-*`.

## 3. Components (`import ... from "@live-commerce/ui/ds"`, separate entry because `PageHeader`/`Badge` exist in the legacy export)

All copy arrives as props (no literal words in `packages/ui/src/ds`, tested). TS strict. No literal colours (tested).

| Component | States / behaviour |
| --- | --- |
| `Button`, `ButtonLink` | primary / secondary / danger / link; sm / md / lg; `loading` (spinner, `aria-busy`, clicks swallowed); **no plain `disabled`**: `disabledReason` is required to disable, sets `aria-disabled`, `title`, `aria-describedby`, and a click reveals the reason in place |
| `SegmentedSwitch` | on / off as the words 是 / 否 (props), one `role=switch` named by `label`, disabled with reason |
| `Tabs`, `TabPanel` | `underline` and `section` variants, roving tabindex, Arrow / Home / End, count badge |
| `Badge`, `StatusPill` | tone + shape glyph (not colour alone); `StatusPill` takes a closed per-domain vocabulary: payment, fulfilment, refund, connection (statuses of another domain do not compile; `unknown` = 待核对 is a warning in all four) |
| `Alert` | info / success / warning / danger; danger = `role=alert`, others `role=status`; optional action and dismiss |
| `Card`, `FormSection` | blue title row, help "?" toggle (`aria-expanded`), meta counter ("0 / 12"), actions slot, flush body |
| `PageHeader`, `Breadcrumb` | breadcrumb (last crumb current), h1, description, actions; `Breadcrumb` also fits the shell slot |
| `ActionBars` | top + bottom on desktop, **sticky bottom only below 768 px**, dirty note, actions rendered twice (no ids inside) |
| `SettingsLayout` | one card, left sub-nav with group titles, blue bar + light-blue current item, stacks on mobile |
| `KpiCard` | with `href` the whole card is one link ("value label"); optional icon and trend |
| `DataTable` | selection with **"this page" vs "all N results"** scope (logic.ts), thumbnail cell (`ThumbCell`), row actions with **at most one primary** (throws in dev, demotes in prod), `MenuButton` for "more", empty / loading / error as rows, caption + scroll frame |
| `FilterBar`, `FilterSelect`, `SearchBox`, `BulkMenu`, `FilterDivider` | select with visible label, search with its own button (Enter or click), bulk menu disabled-with-reason until a selection exists |
| `Pagination`, `withPage`, `pageFromSearch` | real anchors built from the URL query, page 1 has one canonical URL, window with gaps, previous/next disabled at the ends |
| `EmptyState`, `ErrorState`, `PermissionState`, `BlockedState` | reason + next step (Blocked requires both), Error is `role=alert` with retry |
| `Dropzone` | format / size hint always on screen **before** upload; wrong type or size listed with the caller's message; file chooser via a real button |
| `ConfirmDialog` | native modal `<dialog>`, Escape / Cancel call `onCancel`, backdrop does not dismiss, optional typed-confirm phrase (Confirm disabled with reason until it matches) |
| `AppShellV2` | dark 260 px sidebar: brand bar, store card, home link + the nine NEEDS 1.1 groups with collapsible second level (`aria-expanded`, current group open), topbar slots (locale, help, notifications, **进入我的商店**, account), breadcrumb slot, mobile drawer < 768 px (focus trap, Escape, inert background, scroll lock, closes when the window widens) |

Gallery: `/{locale}/ui-gallery` (`apps/admin/app/[locale]/ui-gallery/page.tsx`), 404 unless `LC_UI_GALLERY=1`, shows every component and state inside `AppShellV2`; copy in `apps/admin/src/ui-gallery-copy.ts` (zh-TW, zh-CN, en; key parity enforced by the `GalleryCopy` type).

## 4. Gates (all NOT_RUN here)

| Gate | Where | What it proves | Tier |
| --- | --- | --- | --- |
| `tests/ui/ds-logic.test.ts` | `test-node.sh`, `--browser-ui-gallery` | selection scope, one primary per row, disabled-with-reason, paging/URL, typed confirm, file limits, status families, nav open state | MODEL_ONLY |
| `tests/ui/ds-tokens.test.mjs` | same | contrast of 38 text pairs (4.5:1) and 11 non-text pairs (3:1) incl. focus on both surfaces and the dark nav; 4-based spacing; 12-28 type scale; z-order; nav width; legacy `--ui-*` values frozen; no literal colour / copy in `ds/` | MODEL_ONLY |
| `scripts/dev/check-hex-ratchet.mjs` + `tests/ui/ds-hex-ratchet.test.mjs` + `tests/ui/hex-baseline.json` | `test-node.sh` (T0) | hard-coded hex colours in `apps/admin` may not increase, per file and in total; new files start at 0; `--write` refuses to raise | MODEL_ONLY |
| `--browser-ui-gallery` (`tests/admin/ui-gallery-runner.mjs`, `ui-gallery-clicks.mjs`) | `test-local.sh`, `release-gate.sh`, GATES.md (all three updated) | flag off = 404 x3 locales; flag on: 3 locales x (1440, 390) screenshot + axe serious/critical = 0 + visual-lint blocking = 0; real-click ledger (about 65 rows per run, two runs: zh-TW 1440 and en 390) with reload checks for URL state | MOCK (BROWSER) |

Click ledger (page, control, action, expected, actual, result) is produced by the run at `<run-root>/ui-gallery/click-ledger.{json,md}`; it does not exist yet because nothing was run.

## 5. NOT_RUN / BLOCKED

- NOT_RUN: `node --test` of the three new suites, `check-hex-ratchet`, `shell-registry.test.ts` (edited), `tsc` / `next build`, `--browser-ui-gallery`, `bash scripts/dev/check-gates.sh`, `release-gate.sh`. All written, none executed.
- BLOCKED: `bash output/integrator/tools/prepush.sh <worktree>` returned "requires approval" in this environment and was not run. Substitute: the header check was done by hand with `grep -L` over every new source file (all 14 carry `Purpose:` / `Depends on:` / `Used by:`); no `xgdwm` domain was written.
- NOT_RUN by design: WebKit; any real page on the DS (W3); file drag-and-drop onto the dropzone (the format/size rule is unit-tested, the click-to-chooser path is in the ledger); real SHOPLINE pixel comparison.

## 6. Risks (read before merging)

1. **Hex baseline is hand-counted.** The brief says "today ≈ 260"; the scan of `apps/admin` CSS/TS/TSX finds **386** hex literals in 25 files (`grep -o` over the tree, counted by hand per file; `#facebook` ids excluded; comment occurrences may be included, which only makes the baseline slightly loose). First CI run of `test-node.sh` will say so if a file is off by one: fix with `node scripts/dev/check-hex-ratchet.mjs --write` on a clean tree (it refuses to raise an existing baseline, so run it before anything else touches the CSS).
2. **Nothing was compiled or run.** Expect possible first-run TypeScript or selector fixes in `Data.tsx`, `UiGallery.tsx`, `ui-gallery-clicks.mjs`. The click journey uses accessible names built from the copy file; a changed label breaks it by design.
3. **Visual-lint / axe may flag the gallery** (R2/R3 on the filter bar and table, target size). The assertions are blocking; thresholds were not tuned. Mitigations already in CSS: table `min-width: 760px` so row actions never stack, sticky bars are "pinned" for R6, disabled page arrows stay at 5:1.
4. **Gate edit to an existing test:** `tests/admin/shell-registry.test.ts` now exempts exactly `/ui-gallery` from the page/registry bijection and pins the `LC_UI_GALLERY !== "1"` 404 guard by regex. Without it the bijection test fails on the new route. Integrator to confirm this is acceptable (alternative: register the route, which would put a dev page in the merchant nav registry).
5. **Palette deviations from "贴近 SHOPLINE"** (section 1): primary, link, text-3, input border, toggle-on are darker than the measured values because the measured ones fail WCAG AA. `brand #4492d1` is kept for decorative use. Owner can veto per token; each is one line in `tokens.css` and one pair in the contrast test.
6. **Navigation count:** NEEDS 1.1 lists ten rows (0-9) while its text says nine groups. The shell takes row 0 (工作台) as a top-level link and rows 1-9 as the nine collapsible groups.
7. **Drawer logic is duplicated** from `AppShell` (`useDrawer` in `AppShellV2.tsx`) instead of refactoring the live shell in a unit that must not change pages; fold them together when W3 switches the shell.
8. **`AppShellV2` is a separate component, not a prop on `AppShell`.** The "flag" is which component a page imports; W3 decides. Only the gallery mounts it.
9. **Demo state without a URL form is not persisted** (typed field, dialog result, bulk result): the gallery has no backing store, so "refresh keeps it" is verified only for tabs, switch, settings item, KPI list, table state and page.
10. `process.env.NODE_ENV` in `DataTable` selects strict (throw) vs demote for a second primary action; Next inlines it, so production never throws on styling.

## 7. Round 2 (PR #66 review: PASS with six P2s, all fixed; nothing was run locally)

| # | Fix | Test (all NOT_RUN here, MODEL_ONLY / MOCK) |
| --- | --- | --- |
| 1 | `check-hex-ratchet.mjs`: comments are stripped by a tokenizer (`stripComments`) that respects `'` `"` strings, template literals with nested `${}`, and `//` comments (a `/*` inside any of them does not open a block). Scans `.css .scss .js .jsx .ts .tsx .mjs`. Skips `node_modules`/`.next` anywhere, but `tests`/`public` only as `apps/admin/tests` and `apps/admin/public` (by path). "Never raise the baseline" is unchanged (`--write` still refuses growth). | `tests/ui/ds-hex-ratchet.test.mjs`: one case per bypass (string, template, template expression, `//` comment, CSS string, `url(//)`, SCSS `url(http://)`), comments still ignored, and a temp-tree `scan` test for the new extensions and the path-based skips |
| 2 | `logic.ts`: `selectAllResults(state, pageIds, total, filterKey)` records the key; `bulkTarget` returns `{ scope: "stale_selection" }` when the current key differs. `DataTable` takes `selection.filterKey`; the gallery clears the selection and reports nothing on a stale target. | `tests/ui/ds-logic.test.ts` (different filter, different query, same key, empty key) |
| 3 | Page scope counts only ids on the current page (`selectionCount(state, total, pageIds)`); `bulkTarget(..., pageIds)` limits ids the same way, so the label, the bulk count and the target agree. | `ds-logic.test.ts` and `tests/ui/ds-render.test.mjs` (banner on page 2 does not count page-1 ids) |
| 4 | "More" menu items with only `href` render as `<a>` (`menuItemKind`); disabled ones stay a button with the reason. | `ds-logic.test.ts` (`menuItemKind`). The open menu itself is not rendered in a test and no gallery row has a `more` action with only an href: NOT_RUN in the browser |
| 5 | `AppShellV2`: `close` is `useCallback`, so the drawer effect no longer re-runs on re-render (it restored focus to the menu button and re-focused the first link on every group toggle). | click ledger step "Shell group" now also asserts the group button keeps focus after the click (`ui-gallery-clicks.mjs`, runs in `--browser-ui-gallery`) |
| 6 | `SegmentedSwitch`: `aria-describedby` points at the reason, which is screen-reader-only until the switch is clicked, then visible (`role=status`), like `Button`. | `tests/ui/ds-render.test.mjs` (describedby id matches the reason element; enabled switch has none). The gallery switch has no disabled reason, so there is no browser click row for it |

New file `tests/ui/ds-render.test.mjs` (server render of the real `Controls.tsx`/`Data.tsx`) is registered in `scripts/dev/test-node.sh` next to the other DS suites.

**Hex baseline:** `tests/ui/hex-baseline.json` was not edited. The stricter tokenizer can change per-file counts (a hex in a trailing `//` comment no longer counts; a hex hidden behind a string `/*` now does). Counts can only fall for the first kind; the second could raise a file above its baseline. The baseline will be re-written in CI with `node scripts/dev/check-hex-ratchet.mjs --write` on the clean tree; that command still refuses to raise an existing baseline, so if CI shows a real increase it must be fixed in the CSS/TS, not in the JSON.

NOT_RUN: `node --test` of the touched suites, `tsc`, `--browser-ui-gallery`. Written, not executed. Known limit: the tokenizer does not parse JS regex literals.
