# DS-3 delivery: ds component layer takes the Figma master values

Role ui_worker · model Claude Opus 5.5 (reasoning default) · worktree `.claude/worktrees/ds3` · branch `unit/ds3-figma-tokens` · base `origin/r3/integration` @ d960cab2a · PR #237 (target r3/integration, no auto-merge).
Tier: MOCK (gallery sample data). Self-report = E0; independent review is the integrator's.

Authority: owner 2026-10-26 "以 Figma 为准", 2026-10-27 "ui全部使用figma的设计". Values: Figma page 4:8829 masters, re-read 2026-10-27 through the fleet local bridge (`use_figma`, read-only `getNodeByIdAsync`, 8 roots in one call; screenshots via `get_screenshot`; no writes). Raw read: `figma/masters.json` (script `figma/read.js`), master screenshots `figma/shots/`, page context `figma/page/` (4:10687 desktop, 4:9104 mobile).

## Gate

- `tests/ui/ds-figma-parity.mjs` + `output/ds-3/figma-expected.json` (46 checks after review round 1, each with Figma node id and the raw Figma read). Computed background / border / text colour / radius / height / weight / line-height (+ label inset, separator, placeholder colour, caret, bar contents, href / aria-disabled, browser-measured hover contrast, rail hit area) of every variant of the 8 Figma masters in the UI gallery at 1586x900 zh-TW: Button 10/10, Nav Item Default + Active, Language switch, Page Header, Tag 5/5, Segmented 8/8, Input 3/3, Pagination 10/10. Plus derived rows with no Figma master: nav level-1 / level-2 hover (>= 4.5:1) and rail hit area (>= 44). Comparator self-check (wrong colour / height / missing probe must fail) runs at import.
- Registered in CI through the existing `--browser-ui-gallery` runner (`tests/admin/ui-gallery-runner.mjs`), which every UI-path PR runs (scripts/dev/pr-modes.mjs). No new registry entry needed. Each check is a click-ledger row; the new specimen card's 10 buttons are really clicked (aria-disabled ones with the real mouse) and its 3 inputs filled.
- RED: Gates run 40500056860 @ 972878c1f, job `gate (--browser-ui-gallery)` failure, 26 findings, all DS-3 (every other DS-1/DS-2 row passed): `red-run-40500056860.txt`. Button/Nav/Language/Page Header/Segmented weights failed on values; Tag, Input, Pagination passed (they already matched, as the audit said). 4 rows failed on the gate itself (aria-disabled click wait) and were fixed in the GREEN commit.
- GREEN (round 0): Gates run 40533759133 @ 5614d4966, 37/37.
- Review round 1 RED: Gates run 40553401910 @ 550fe9f3c, ui-gallery 14 findings (line-height 18.2 vs 22 on 11 button/language rows, Ghost text, level-2 hover 4.30:1, rail hit area 40): `red-run-40553401910.txt`. The 4 new Pagination rows passed in RED (the code already matched; they closed a coverage gap, not a defect).
- **Final GREEN: Gates run 40590988493 @ 28a63507e (ui-gallery job 122510894456), Foundation run 40590988522 success.** ui-gallery: 46/46 parity rows, 59/59 DS-3 ledger rows, 1218 ledger rows with 0 failed (`gallery/parity.json`, `gallery/ds3-ledger-rows.json`); measured hover contrast 13.1:1 on both nav levels.

## Before / after / Figma

Before = origin/r3/integration ds as measured by the RED runs (`red-run-40500056860.txt`, `red-run-40553401910.txt`); where a RED row passed, before = the expected value. After = this PR. Figma = the read value.

| Master (node) | Property | Before | After | Figma |
|---|---|---|---|---|
| Button Primary (4:12712) | fill / border / text | #2a6fb0 / #2a6fb0 / #fff | #142942 / #142942 / #fff | #142942 / none / #fff |
| Button Secondary (4:12714) | fill / border / text | #fff / #8c8c8c / #333 | #fff / #e5e7eb / #142942 | same as after |
| Button Danger (1394:103516) | fill / border / text | #b42318 / #b42318 / #fff (filled) | #fff / #b42318 / #b42318 (outline) | same as after |
| Button Text (1394:103518) = ds `ghost` | text | #5b5b5b | #0f766e | #0f766e |
| Button Ghost (4:12716 / 1394:103524) = new ds `outline` | fill / border / text; disabled | no variant (specimen rendered secondary: ink text) | #fff / #e5e7eb / #0f766e; disabled #f5f6f8 / #e5e7eb / #566171 | same as after |
| Button all kinds + Language switch | height / radius / pad / weight / line-height | 36 / 4 / 16 / 400 / 18.2 | 44 / 6 / 14 / 500 / 22 | 44 / 6 / 14 / Medium (wght 500) / 22 |
| Button Secondary/Danger Disabled | fill / border / text | #dae5ef / #dae5ef / #435b70 | #f5f6f8 / #e5e7eb / #566171 | same as after |
| Button Text Disabled | fill / border / text | #dae5ef / #dae5ef / #435b70 | #f5f6f8 / #f5f6f8 / #566171 | #f5f6f8 / none / #566171 |
| Button Primary Disabled (1394:103520) | fill / border / text | #6d8cab / #6d8cab / #fff | unchanged (gate) | #f5f6f8 / #e5e7eb / #566171 — deviation D1 |
| Nav Item Default (4:12733) | fill / text / radius / inset / weight | transparent on #373d4c / #f4f6f9 / 0 (5 from globals) / 55 / 400 | #fff / #6b7280 / 6 / 42 / 500 | #fff / #6b7280 / 6 / 42 / Medium |
| Nav Item Active (4:12735) | fill / text / accent bar | #494f5d / #f4f6f9 / 3px #4492d1 | #eef2f7 / #142942 / none | #eef2f7 / #142942 / none |
| Rail | background | #373d4c | #fff | #fff (light rail) |
| Rail hover, level 1 / level 2 (no master) | fill / text / contrast | #edf3f8 from globals.css / #6b7280 on #eef2f7 = 4.30:1 | #eef2f7 / #142942, >= 4.5:1 | UNKNOWN; rule = Active colours |
| Rail level-2 rows (no master) | height | 40 | 44 | Nav Item rows 44 |
| Language switch (1397:105153) | fill / border / text / radius / height / caret | #fff / #8c8c8c / #333 / 4 / 36 / chevron | #fff / #e5e7eb / #142942 / 6 / 44 / none | #fff / #e5e7eb / #142942 / 6 / 44 / none |
| Page Header (1397:105155) | bar bg / height / pad / rule / crumbs in bar | #fdfdfd / 50 / 16 / 1px / no (row below) | #fff / 64 / 24 / none / yes (desktop) | #fff / 64 / 24 / none / yes |
| Crumbs (1397:105157-61) | colour / size / separator / gap | #6b6b6b, current #333 / 13 / chevron icon / 8 | #596579 all / 14, line 21 / "/" / 4 | #596579 all / 14, line 21 / "/" / 4 |
| Segmented selected (1401:38433/37/41/45) | weight | 600 | 500 | Medium |
| Segmented idle (1401:38435/39/47) | fill | transparent (container painted) | #fff / #f3f3f3 own fill | #fff / #f3f3f3 |
| Tabs (1401:38441/45) | radius | 5 (leaked from admin globals.css `button`) | 0 | 0 |
| Tag (1401:38422-30) | all | match | unchanged | match |
| Input (1401:38450/52/54) | all | match | unchanged | match |
| Pagination (1401:57997-58021, 10 variants) | all | match | unchanged | match; Current wght 600 per `figma/weights.json` (fontName Bold + variationSettings wght 600, node fontWeight 600) |

## Registered deviations (owner / integrator decide; none silently changed)

- D1 Primary/Disabled colours. Figma #f5f6f8 / #e5e7eb / #566171 conflicts with the DS-1 gate that requires a disabled primary fill >= 3:1 against what is behind it (`tests/admin/ui-gallery-clicks.mjs` readFrame `faint`; `tests/ui/ds-tokens.test.mjs` disabled-primary pairs; it was a DS-1 review defect). #f5f6f8 on #fff is 1.08:1. Gate > owner wording (iron rule 3), so the colour keeps #6d8cab / #fff and geometry follows Figma. One word from the owner flips it (change `--ds-disabled-primary-*` and drop the gate).
- D2 Rail width. Figma rows are 216 at x12 (rail about 240). `ds-tokens.test.mjs` bounds `--ds-nav-width` to 248–270, so it stays 270; rows are inset 12 px like Figma and are therefore 246 wide.
- D3 Language switch width. Figma fixes 64; ds width follows the label (zh-TW 繁中 about 58, English wider). Not asserted.
- D4 Hover / focus. No Figma master has them (UNKNOWN). Nav hover (both levels) reuses Active (default text on the Active fill is 4.30:1 < AA), asserted in the browser. Primary keeps its fill on hover (declared so the admin `globals.css` `button:hover` #edf3f8 cannot repaint it); Secondary / Danger / Text keep the DS-2 #f3f3f3 hover. Focus ring is `--ds-focus` (the dark-rail ring is gone).
- D5 No Figma master (UNKNOWN, kept DS-2 values): Button `sm` 28 and `lg`, `link` variant, icon-only, loading; nav level-2 items (styled with the light palette, indent 42); rail icons (Figma rows have none; icons are caller data, label inset still 42); topbar help / notifications / store / account tools (all now 44 Buttons); phone breadcrumb row (Figma mobile header 4:9104 has no breadcrumb; ds keeps it under the 64 px bar); brand / store text colour (ink, from the 4:10687 page screenshot, not a master).
- D6 Font weights 500 are set in CSS; the rendered face depends on the platform font (system-ui / PingFang / Noto Sans TC). Computed weight is asserted, glyph rendering is not.
- D8 Figma Button Kind=Ghost is DEPRECATED in Figma (STAGE-6: 0 page instances, kept because the backup page references it). Review round 1 asked for it to exist in code anyway: ds `outline`. ds `ghost` stays Figma Text, so existing ghost callers (topbar tools) keep their look.
- D9 Pre-existing, not fixed here: the admin `globals.css` `button:hover:not(:disabled)` (#edf3f8) still repaints ds Tabs and the SegmentedSwitch on hover (no Figma hover master for either).
- D7 Buttons in a FilterBar on desktop are 36, not 44. Figma Button is 44 but Figma Input / Pagination / Segmented are 36, and the DS-2 same-row gate requires one height per row. Found by GREEN attempt 1 (run 40514175160: BulkMenu 44 beside 36 filter fields, 3 locales at 1440). Scoped CSS in `data.module.css` (`.filters button[data-size="md"]`, >= 768 px only); everywhere else Buttons are 44. Owner may prefer 44 fields in filter rows instead.
- Figma Segmented / Input have no Focus or container variants, so there was no "Figma has, ds lacks" state to add for Segmented / Input / Pagination / Tag (the card's examples are the reverse direction; listed for the Figma lane instead).

## Legacy pages

None change. Changed / removed tokens are `--ds-*` only. Outside packages/ui/src/ds, `--ds-*` is read by `apps/admin/components/UiGallery.module.css` and `UiGallerySections.tsx` (gallery), and `apps/admin/components/product-media.css` (`--ds-on-primary`, unchanged). `globals.css` imports tokens.css but reads no `--ds-*`; the `[data-ds="2"]` remap is applied nowhere (its `--ui-rail-active` target moved from the removed `--ds-nav-accent-text` to `--ds-nav-text-active`). Removed tokens: `--ds-nav-accent`, `--ds-nav-accent-text`, `--ds-focus-on-dark` (0 remaining references).

## Out-of-scope writes and requests (integrator to approve)

- `packages/ui/src/tokens.css` — outside the card's write_paths, but the only file where R1 (`scripts/dev/check-ds-rules.mjs`, `ds-tokens.test.mjs`) allows colour literals; the card asks for token-first changes. Only the `--ds-*` block and the opt-in remap changed; the legacy `--ui-*` block is byte-identical (its gate passes).
- Approved by the integrator in review round 1: `tokens.css` (`--ds-*` block only), `tests/admin/ui-gallery-runner.mjs`, `tests/admin/ui-gallery-clicks.mjs` (topbar row 50 → 64 and tools 36 → 44; crumb links excluded from the tool-height check), `tests/ui/ds-tokens.test.mjs` (pairs only, thresholds unchanged), and `scripts/dev/test-local.sh` `--browser-ui-gallery` lc_prepare `test -f tests/ui/ds-figma-parity.mjs` / `output/ds-3/figma-expected.json` (added).
- The parity module is `tests/ui/ds-figma-parity.mjs`, not `ds*.spec.ts`: check-gates requires any `*.spec.*` file to be named in test-local.sh / foundation / test-node.sh (foundation run 40496965866 failed on that).
- `docs/design/ds/source-sha256.txt` (DS-2 round-2 byte manifest) is now stale for the changed files; left untouched as a historical record.

## Risks

- The light rail and 64 px bar change every future AppShellV2 page; today only the gallery mounts it.
- Buttons are now 44 next to 36 inputs; the DS-2 same-row height gate passes on the gallery (GREEN run) but a W3 page that puts a Button beside an Input in one flex row will fail it. That is a Figma-level conflict (Button 44 vs Input 36) for the owner.
- The breadcrumb renders twice (bar on desktop, row on phone; CSS shows one). Callers must not give crumb elements ids.

## NOT_RUN

Local tests and servers (CI only, per card). Independent review. WebKit / Safari rendering of the CSS `content: "/" / ""` alt syntax (older Safari reads the slash). Hover / focus parity (no Figma value). zh-CN / en parity rows (the gate runs zh-TW; other locales run the existing DS-1/DS-2 matrix only).

## Runs

| Step | Head | Run | Result |
|---|---|---|---|
| RED attempt 0 | 54f99257f | Foundation 40496965866 | failure: check-gates, `tests/ui/ds-figma.spec.mjs` run by no gate (renamed to `ds-figma-parity.mjs`) |
| RED | 972878c1f | Gates 40500056860, ui-gallery job 122186754000 | failure, 26 findings, all DS-3 (`red-run-40500056860.txt`) |
| GREEN attempt 1 | ea29cef98 | Gates 40514175160, job 122264267125 | failure, 4 findings: DS-2 same-row (BulkMenu 44 vs 36 filter fields, 3 locales at 1440) + comparator treated `allCrumbsSameColor` as a colour; fixed in 5614d4966 (D7) |
| GREEN | 5614d4966 | Gates 40533759133, job 122296409784; Foundation 40533759141 | ui-gallery success (37/37, 1207 ledger rows, 0 failed); all 78 gate jobs + receipts + aggregate success; foundation success |
| R1 RED | 550fe9f3c | Gates 40553401910, job 122406035854 | failure, 14 DS-3 findings (`red-run-40553401910.txt`); `--browser-payment` failure in the same run = known SU09 flake #145 (integrator) |
| R1 GREEN attempt 1 | fedb285a3 | Gates 40576831820, job 122457266637 | failure, 1 finding: nav.item.hover rgb(237,243,248): admin globals.css `button:hover:not(:disabled)` out-specified `.groupRow:hover`; fixed in 65c8296f0 (also pinned primary / link hover) |
| R1 GREEN attempt 2 | 65c8296f0 | Foundation 40585473703, job 122475610688 | failure: ds-tokens "tokens only" found a hex quoted in a CSS comment; comment fixed in 28a63507e, no value change |
| **R1 GREEN** | **28a63507e** | **Gates 40590988493 (ui-gallery job 122510894456); Foundation 40590988522** | **ui-gallery success 46/46; all 78 gate jobs + receipts + aggregate success; foundation success** |

Round 0: one targeted fix for the GREEN blocker. Round 1: two targeted fixes after the R1 GREEN commit (hover specificity, comment hex), within the two-fix limit.

## Screenshots

- `gallery/gallery-1586.png`, `gallery/gallery-390.png`: gallery viewport from the final GREEN run 40590988493 (artifact gate-browser-ui-gallery).
- `gallery/specimens-1586.png`, `gallery/specimens-390.png`: the #sec-figma specimen card (10 buttons incl. outline = Figma Ghost, 3 inputs).
- `side-by-side-1586.png`, `side-by-side-390.png`: Figma master screenshots (`figma/shots/`, transparent master frames render black) next to the gallery.

## Verification commands

- Local (static only): `tsc --noEmit -p apps/admin/tsconfig.json` exit 0 (temporary node_modules symlinks, removed); `node scripts/dev/check-ds-rules.mjs` exit 0 (20 files, 0 violations); token contrast pairs recomputed statically, 0 below threshold.
- CI: `scripts/dev/test-local.sh --browser-ui-gallery` (runs `node --test` ds-logic / ds-tokens / ds-hex-ratchet, the flag-enabled build, and `tests/admin/ui-gallery-runner.mjs`, which imports `tests/ui/ds-figma-parity.mjs`).

## Review round 1 responses

| Item | Fix | Evidence |
|---|---|---|
| P1-1 Ghost missing | ds `outline` = Figma Ghost (4:12716 / 1394:103524); specimen + 2 rows + ledger clicks; `ghost` stays Text | RED 40553401910 (Ghost text ink) → GREEN 40590988493 |
| P1-2 level-2 hover 4.30:1 | late `.item:hover` override removed; hover rules prefixed `.nav` so globals.css cannot win; real-hover contrast rows for both levels; ds-tokens comment now true | RED (sub-item #6b7280, contrastAA false) → GREEN 13.1:1 |
| P2-3 4 pager masters | rows for 1401:58003/58006/58015/58018 via real clicks to page 2 / last; disabled arrows assert no href + aria-disabled | 46/46 |
| P2-4 line-height | `--ds-leading-button` 22px; lineHeight asserted on 10 button rows + language switch | RED 18.2px → GREEN |
| P2-5 level-2 40px | `.item` min-height 44 everywhere; `nav.hitArea` minHit over buttons and nested links | RED 40 → GREEN 44 |
| P2-6 weight citation | fresh read `figma/weights.json`: 1401:58002 fontName Bold + wght 600, fontWeight 600 (also confirms Medium = wght 500 on button / nav / segment labels) | file |
| P2-7 stale spec name | DS-2.md and UiGallerySections.tsx now cite `tests/ui/ds-figma-parity.mjs` (the DELIVERY runs table keeps the old name only as the historical RED attempt 0) | diff |
| P2-8 audit citation | "before" now cites the RED run files in this folder | this file |
