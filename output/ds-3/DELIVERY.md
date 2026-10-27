# DS-3 delivery: ds component layer takes the Figma master values

Role ui_worker · model Claude Opus 5.5 (reasoning default) · worktree `.claude/worktrees/ds3` · branch `unit/ds3-figma-tokens` · base `origin/r3/integration` @ d960cab2a · PR #237 (draft, target r3/integration, no auto-merge).
Tier: MOCK (gallery sample data). Self-report = E0; independent review is the integrator's.

Authority: owner 2026-10-26 "以 Figma 为准", 2026-10-27 "ui全部使用figma的设计". Values: Figma page 4:8829 masters, re-read 2026-10-27 through the fleet local bridge (`use_figma`, read-only `getNodeByIdAsync`, 8 roots in one call; screenshots via `get_screenshot`; no writes). Raw read: `figma/masters.json` (script `figma/read.js`), master screenshots `figma/shots/`, page context `figma/page/` (4:10687 desktop, 4:9104 mobile).

## Gate

- `tests/ui/ds-figma-parity.mjs` + `output/ds-3/figma-expected.json` (37 checks, each with Figma node id and the raw Figma read). Computed background / border / text colour / radius / height / weight (+ label inset, separator, placeholder colour, caret, bar contents) of every master variant in the UI gallery at 1586x900 zh-TW. Comparator self-check (wrong colour / height / missing probe must fail) runs at import.
- Registered in CI through the existing `--browser-ui-gallery` runner (`tests/admin/ui-gallery-runner.mjs`), which every UI-path PR runs (scripts/dev/pr-modes.mjs). No new registry entry needed. Each check is a click-ledger row; the new specimen card's 8 buttons are really clicked (aria-disabled ones with the real mouse) and its 3 inputs filled.
- RED: Gates run 40500056860 @ 972878c1f, job `gate (--browser-ui-gallery)` failure, 26 findings, all DS-3 (every other DS-1/DS-2 row passed): `red-run-40500056860.txt`. Button/Nav/Language/Page Header/Segmented weights failed on values; Tag, Input, Pagination passed (they already matched, as the audit said). 4 rows failed on the gate itself (aria-disabled click wait) and were fixed in the GREEN commit.
- GREEN: see "Runs" below.

## Before / after / Figma

Before = origin/r3/integration ds (audit AUDIT-DS-VS-FIGMA.md §1, confirmed by the RED run). After = this PR. Figma = the read value.

| Master (node) | Property | Before | After | Figma |
|---|---|---|---|---|
| Button Primary (4:12712) | fill / border / text | #2a6fb0 / #2a6fb0 / #fff | #142942 / #142942 / #fff | #142942 / none / #fff |
| Button Secondary (4:12714) | fill / border / text | #fff / #8c8c8c / #333 | #fff / #e5e7eb / #142942 | same as after |
| Button Danger (1394:103516) | fill / border / text | #b42318 / #b42318 / #fff (filled) | #fff / #b42318 / #b42318 (outline) | same as after |
| Button Text (1394:103518) = ds `ghost` | text | #5b5b5b | #0f766e | #0f766e |
| Button all kinds | height / radius / pad / weight | 36 / 4 / 16 / 400 | 44 / 6 / 14 / 500 | 44 / 6 / 14 / Medium |
| Button Secondary/Danger Disabled | fill / border / text | #dae5ef / #dae5ef / #435b70 | #f5f6f8 / #e5e7eb / #566171 | same as after |
| Button Text Disabled | fill / border / text | #dae5ef / #dae5ef / #435b70 | #f5f6f8 / #f5f6f8 / #566171 | #f5f6f8 / none / #566171 |
| Button Primary Disabled (1394:103520) | fill / border / text | #6d8cab / #6d8cab / #fff | unchanged (gate) | #f5f6f8 / #e5e7eb / #566171 — deviation D1 |
| Nav Item Default (4:12733) | fill / text / radius / inset / weight | transparent on #373d4c / #f4f6f9 / 0 (5 from globals) / 55 / 400 | #fff / #6b7280 / 6 / 42 / 500 | #fff / #6b7280 / 6 / 42 / Medium |
| Nav Item Active (4:12735) | fill / text / accent bar | #494f5d / #f4f6f9 / 3px #4492d1 | #eef2f7 / #142942 / none | #eef2f7 / #142942 / none |
| Rail | background | #373d4c | #fff | #fff (light rail) |
| Language switch (1397:105153) | fill / border / text / radius / height / caret | #fff / #8c8c8c / #333 / 4 / 36 / chevron | #fff / #e5e7eb / #142942 / 6 / 44 / none | #fff / #e5e7eb / #142942 / 6 / 44 / none |
| Page Header (1397:105155) | bar bg / height / pad / rule / crumbs in bar | #fdfdfd / 50 / 16 / 1px / no (row below) | #fff / 64 / 24 / none / yes (desktop) | #fff / 64 / 24 / none / yes |
| Crumbs (1397:105157-61) | colour / size / separator / gap | #6b6b6b, current #333 / 13 / chevron icon / 8 | #596579 all / 14, line 21 / "/" / 4 | #596579 all / 14, line 21 / "/" / 4 |
| Segmented selected (1401:38433/37/41/45) | weight | 600 | 500 | Medium |
| Segmented idle (1401:38435/39/47) | fill | transparent (container painted) | #fff / #f3f3f3 own fill | #fff / #f3f3f3 |
| Tabs (1401:38441/45) | radius | 5 (leaked from admin globals.css `button`) | 0 | 0 |
| Tag (1401:38422-30) | all | match | unchanged | match |
| Input (1401:38450/52/54) | all | match | unchanged | match |
| Pagination (1401:57997-58021) | all | match | unchanged | match (Current wght 600) |

## Registered deviations (owner / integrator decide; none silently changed)

- D1 Primary/Disabled colours. Figma #f5f6f8 / #e5e7eb / #566171 conflicts with the DS-1 gate that requires a disabled primary fill >= 3:1 against what is behind it (`tests/admin/ui-gallery-clicks.mjs` readFrame `faint`; `tests/ui/ds-tokens.test.mjs` disabled-primary pairs; it was a DS-1 review defect). #f5f6f8 on #fff is 1.08:1. Gate > owner wording (iron rule 3), so the colour keeps #6d8cab / #fff and geometry follows Figma. One word from the owner flips it (change `--ds-disabled-primary-*` and drop the gate).
- D2 Rail width. Figma rows are 216 at x12 (rail about 240). `ds-tokens.test.mjs` bounds `--ds-nav-width` to 248–270, so it stays 270; rows are inset 12 px like Figma and are therefore 246 wide.
- D3 Language switch width. Figma fixes 64; ds width follows the label (zh-TW 繁中 about 58, English wider). Not asserted.
- D4 Hover / focus. No Figma master has them (UNKNOWN). Nav hover reuses Active (default text on the Active fill is 4.30:1 < AA). Primary has no hover; Secondary / Danger / Text keep the DS-2 #f3f3f3 hover. Focus ring is `--ds-focus` (the dark-rail ring is gone).
- D5 No Figma master (UNKNOWN, kept DS-2 values): Button `sm` 28 and `lg`, `link` variant, icon-only, loading; nav level-2 items (styled with the light palette, indent 42); rail icons (Figma rows have none; icons are caller data, label inset still 42); topbar help / notifications / store / account tools (all now 44 Buttons); phone breadcrumb row (Figma mobile header 4:9104 has no breadcrumb; ds keeps it under the 64 px bar); brand / store text colour (ink, from the 4:10687 page screenshot, not a master).
- D6 Font weights 500 are set in CSS; the rendered face depends on the platform font (system-ui / PingFang / Noto Sans TC). Computed weight is asserted, glyph rendering is not.
- D7 Buttons in a FilterBar on desktop are 36, not 44. Figma Button is 44 but Figma Input / Pagination / Segmented are 36, and the DS-2 same-row gate requires one height per row. Found by GREEN attempt 1 (run 40514175160: BulkMenu 44 beside 36 filter fields, 3 locales at 1440). Scoped CSS in `data.module.css` (`.filters button[data-size="md"]`, >= 768 px only); everywhere else Buttons are 44. Owner may prefer 44 fields in filter rows instead.
- Figma Segmented / Input have no Focus or container variants, so there was no "Figma has, ds lacks" state to add for Segmented / Input / Pagination / Tag (the card's examples are the reverse direction; listed for the Figma lane instead).

## Legacy pages

None change. Changed / removed tokens are `--ds-*` only. Outside packages/ui/src/ds, `--ds-*` is read by `apps/admin/components/UiGallery.module.css` and `UiGallerySections.tsx` (gallery), and `apps/admin/components/product-media.css` (`--ds-on-primary`, unchanged). `globals.css` imports tokens.css but reads no `--ds-*`; the `[data-ds="2"]` remap is applied nowhere (its `--ui-rail-active` target moved from the removed `--ds-nav-accent-text` to `--ds-nav-text-active`). Removed tokens: `--ds-nav-accent`, `--ds-nav-accent-text`, `--ds-focus-on-dark` (0 remaining references).

## Out-of-scope writes and requests (integrator to approve)

- `packages/ui/src/tokens.css` — outside the card's write_paths, but the only file where R1 (`scripts/dev/check-ds-rules.mjs`, `ds-tokens.test.mjs`) allows colour literals; the card asks for token-first changes. Only the `--ds-*` block and the opt-in remap changed; the legacy `--ui-*` block is byte-identical (its gate passes).
- `tests/admin/ui-gallery-runner.mjs` (registers the parity module), `tests/admin/ui-gallery-clicks.mjs` (topbar row 50 → 64 and tools 36 → 44; crumb links excluded from the tool-height check), `tests/ui/ds-tokens.test.mjs` (contrast pairs for the light rail and the new colours; thresholds unchanged, dark-rail pairs replaced). Read as "existing ui-gallery spec files"; flagged in case the integrator reads write_paths more narrowly.
- The parity module is `tests/ui/ds-figma-parity.mjs`, not `ds*.spec.ts`: check-gates requires any `*.spec.*` file to be named in test-local.sh / foundation / test-node.sh (foundation run 40496965866 failed on that). Optional hardening for the integrator: add `test -f tests/ui/ds-figma-parity.mjs` and `test -f output/ds-3/figma-expected.json` to `--browser-ui-gallery` lc_prepare in `scripts/dev/test-local.sh`.
- `docs/design/ds/source-sha256.txt` (DS-2 round-2 byte manifest) is now stale for the changed files; left untouched as a historical record.

## Risks

- The light rail and 64 px bar change every future AppShellV2 page; today only the gallery mounts it.
- Buttons are now 44 next to 36 inputs; the DS-2 same-row height gate passes on the gallery (GREEN run) but a W3 page that puts a Button beside an Input in one flex row will fail it. That is a Figma-level conflict (Button 44 vs Input 36) for the owner.
- The breadcrumb renders twice (bar on desktop, row on phone; CSS shows one). Callers must not give crumb elements ids.

## NOT_RUN

Local tests and servers (CI only, per card). Independent review. WebKit / Safari rendering of the CSS `content: "/" / ""` alt syntax (older Safari reads the slash). Hover / focus parity (no Figma value). zh-CN / en parity rows (the gate runs zh-TW; other locales run the existing DS-1/DS-2 matrix only).

## Runs

(filled in below)
