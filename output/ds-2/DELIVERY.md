<!-- Purpose: DS-2 implementation handoff, audit mapping and evidence limits. Depends on: task card and scoped source commits. Used by: Claude integrator/independent reviewer/owner. -->
# DS-2 delivery — implementation awaiting CI and visual acceptance

## Round 3 — 2026-10-15

**READY_FOR_CI — E1: TypeScript, DS rules and Prettier exit 0; browser acceptance NOT_RUN.**

Base `60732a530c87042a495042b310a66cc0883d963d`; implementation/static-check head `a1caef739f71c348638ed7dd47ea5f85f8e9ce10`. This delivery is a subsequent evidence-only commit. Branch `unit/ds-2-rework`, assigned `.worktrees/ds-2`, PR #69 remains integrator-owned/draft. Executor: Codex, ui_worker responsibility; runtime model/effort identifiers not exposed; no delegated agents. No push, PR creation, merge or deployment.

### Historical RED and evidence limits

Read the original brief and card-r2 from the **main worktree** `output/` (they are absent from this worktree), DESIGN-SPEC and Round 2 above. CI `38711545381`, `--browser-ui-gallery`, is RED at the supplied base, not acceptance of these changes. Artifact root in the main worktree:
`output/ds-2/ci-gallery-38711545381/playwright/run.gPZ4pfpI/`.

Ledger has **891 rows / 13 failures: F1×4, F2×6, F3×1, F4×1, F5×1**. F1 is three nested mobile cases plus the legacy mobile journey after the matrix; F3/F4 are en desktop, F5 en mobile. These actual counts differ from the dispatch's F1×6. Zero-based failure indices and original input hashes are preserved in [ci-red-summary.json](round3-evidence/ci-red-summary.json).

The runner (`tests/admin/ui-gallery-runner.mjs:77`) keeps only the first exception line: old actual/expected operands and stacks are unavailable. F2's failed assertion can still be identified: **all six `settings-notices` screenshots were captured, with axe/lint empty and errors=0**, after the URL and checkbox assertions. The only assertion after capture is the returned profile name. F3's expanded-vs-collapsed failing arm cannot be recovered from these artifacts; both arms remain covered with distinct diagnostic messages. No runtime trace or unrecorded geometry is invented here.

### F1–F5: causes, fixes and B1–B11 preservation

| Failure | Root cause and evidence | Separate fix commit / resulting source | Preservation |
| --- | --- | --- | --- |
| F1 account menu | Base `tests/admin/ui-gallery-clicks.mjs:527` queries the entire page for a profile **link** on mobile. `UiGalleryFrame.tsx:34` supplies the account profile link, while `UiGallerySections.tsx:239` independently renders the same label in SettingsLayout. Sidebar also legitimately uses that navigation name (`ui-gallery-nav.ts:52`). `shell2.module.css` uses `display:none` for inactive responsive chrome; `Data.tsx:130` mounts a disclosure only when open. Thus this is a **locator scope defect**, not evidence that desktop/mobile account copies are both accessible. Ledger indices 262/512/762/887 explicitly report two links. | `30bf3945f`: current `tests/admin/ui-gallery-clicks.mjs:1070` follows the unique account trigger's `aria-controls`, then asserts exactly one profile item inside that disclosure before clicking and checking the same URL. Adds unique accessible trigger assertion. | No UI/CSS change, so B1's 50px phone row, moved utilities and keyboard coverage remain; no other visual output changes. Global navigation names need not be unique across independently named navigation contexts. The action in the account disclosure must be unique. |
| F2 settings | **Final store-name inputValue after returning to Profile** fails, not `setting=notices` or the reloaded checkbox: six notices captures prove the earlier assertions passed (en1440 shot visibly shows checked notifications). At base `UiGalleryTemplates.tsx:234–238`, server `Demo store` initializes both saved/data, `ui-gallery-state.ts:9` restores storage in an effect, then another effect copies saved into data. A newly loaded field is observable with the seed before restoration. Read-only does not prevent `inputValue`. Native `?setting=profile` navigation remounts it. | `049a8c2cb`: current `UiGalleryTemplates.tsx:1035–1062` derives data from restored saved state until the first edit and renders a loading status until ready. Removes the second effect; Cancel clears the draft. Save already persists synchronously inside its simulated completion before publishing saved state. Added assertion-specific messages; original URL/checkbox/profile assertions unchanged. | Restored settings render the same SettingsLayout, full-width Field/grid and responsive bars. B12 is also retained. No tokens, field sizes, chrome, tabs, badges or table changes. |
| F3 sidebar | Ledger index 638: en1440 post-reload aria-expanded equality. Base `AppShellV2.tsx:83–92` renders active-group defaults before the preference effect, exposing a collapsed Marketing button on a reload even when saved expanded. Additionally `:106–113` writes sessionStorage inside a React state updater, coupling persistence to updater execution. No locale branch exists in this path. | `c5f1a8bf7`: current `AppShellV2.tsx:122–143` exposes groups after storage restoration; `:159–170` calculates from a ref and writes sessionStorage **in the user action**, before setExpanded. No effect/updater write can be beaten by reload. Both expanded/collapsed assertions remain (`clicks.mjs:1116/1124`). | Same rail, groups, current markers, SVG slots and chevron CSS after restoration. B4 scroll isolation and B6 chevron geometry unchanged. Brief loading phase no longer presents incorrect preferences. |
| F4 note | Ledger index 663: en1440 equality. At base `UiGalleryTemplates.tsx:120–122`, SSR note starts empty, then storage effect and note-copy effect restore it. The returned textarea exists before the stored value does. The distinct `noteContent` Field is structurally unique (detail screenshot shows one); Timeline unconditionally renders three ISO times. These facts isolate the reload value window; no locale-specific note path or extra timeline exists. | `218b7db05`: current `UiGalleryTemplates.tsx:488–496` uses a nullable draft over restored saved note and exposes the editor only when ready. The existing synchronous save remains. Journey `:1730` waits for the visible restored editor before its initial uniqueness count; the exact count, saved value and three ISO-time assertions remain, with diagnostic messages. | B7 distinct card/field labels and B8 title-row statuses remain. Detail composition, badges, timeline and typography after restore are unchanged. The visibility wait addresses the real loading UI; no reload value wait, timeout increase or assertion relaxation. |
| F5 sticky release | Ledger index 804: en390 fails **footer releases below the surface** after form bottom was reached. Base `layout.module.css:80` applies bottom:0 unconditionally; `Layout.tsx:169–175` puts footer after surface with a normal-flow footprint. Native bottom-sticky remains displaced until **surface bottom + footer height** fits the viewport; the requirement releases at **surface bottom**. The en390 form shot shows the taller wrapped content/phone footer that can expose the interval. This CSS condition is wrong for any locale; artifacts do not record the exact failing scroll offset. | `d94fa5ad3`: current `Layout.tsx:194–220` measures surface edge on layout, scrolling, resizing and content resize, cleans up listeners/observer; `layout.module.css:332` sets bottom:auto when the edge is on-screen. Position remains sticky and footer keeps its outside normal-flow footprint. Existing geometry/flow/switch/reload checks remain; adds `form-released` screenshot and one-line box diagnostics (`clicks.mjs:1993`). | B3 natural page flow, no clipped inner scroller, top desktop bar / phone bottom-only bar and outside footprint retained. Only sticky inset changes at the specified release boundary; no sidebar, tokens, buttons, breadcrumb, list, table or detail style changes. |

F3/F4's en-only occurrence is consistent with an unguarded restoration race, not an en-only implementation: locale copy changes the work/layout before immediate reads, but the truncated receipts cannot prove the scheduler's exact ordering. F5's interval depends on content/footer height and scroll position; en copy wraps differently in the supplied shot. **Exact timing/scroll attribution remains a CI diagnostic limitation**, not a claimed measured result. All three fixes apply to all locales, and the full six-case matrix remains mandatory.

F1 and F4 are the only locator/readiness adjustments: F1 targets the control actually under test and adds uniqueness; F4 waits for the page's restored editor before counting it. No assertion removed, skipped, force-clicked, weakened or given a longer timeout. B1–B11's existing interaction and geometry assertions remain. `a1caef739` is a separate Prettier-only commit on the five touched source/test/style files; its larger textual diff is formatting, following the five small functional commits.

### Commands, evidence and handoff

All successful checks bind to `a1caef739` and [source-sha256.txt](round3-evidence/source-sha256.txt); [static-receipt.json](round3-evidence/static-receipt.json) records commands, exits, setup recoveries and NOT_RUN. No source changed after those checks.

| Command | Exit / evidence |
| --- | --- |
| `pnpm install --offline --frozen-lockfile --ignore-scripts` | 0, `round3-evidence/dependencies.log`; restores this worktree's existing frozen dependencies, no lockfile/package edits or install scripts. UI workspace link verified to this worktree. |
| `pnpm -C apps/admin exec tsc --noEmit` | 0, `round3-evidence/typecheck.log` (empty on success), TypeScript 7.0.2. Initially exit254 without tsc; PATH-only discovery exit1 without workspace dependencies. Both setup issues resolved by the offline install. |
| `node scripts/dev/check-ds-rules.mjs` | 0, `round3-evidence/ds-rules.log`: 20 files, 0 violations. File headers retained. |
| `pnpm exec prettier --write` then `--check` on the five touched files | 0 final, `round3-evidence/prettier-*.log`, Prettier 3.9.8. First check exit1 on chained-evaluate formatting; second write/check exit0. No semantic adjustment. |
| `git diff --check` | 0. |
| ESLint | NOT_RUN: no installed ESLint/config; Prettier check used. No new tooling dependency. |

**NOT_RUN:** all local tests, browser/server runs and builds (prohibited); new-head CI gallery clicks/axe/visual-lint/screenshot matrix; unit/token/hex-ratchet runtime gates; fault-injection RED→GREEN; independent Jev/Antigravity/integrator review; owner screenshot sign-off; SANDBOX/LIVE. Historical RED is supplied CI evidence only. No E3 completion or current visual PASS claimed.

**Risks:** storage-disabled browsers retain the documented in-memory fallback and cannot promise reload persistence. Brief localized loading states replace stale server defaults for settings/note/sidebar. Sticky boundary observer/scroll behavior, en-only timing and all B1–B11 visual geometry need current-head CI and independent review. New released-footer captures must be inspected. No live route adopts DS-2; no production/data/permission changes. No task test/browser/server/process/container/fixture was started. Dependency installation is worktree-local and ignored; no temporary fixture/script remains. No blocker remained after two failed repair attempts; recovered setup/formatting failures are recorded above.

Integrator: push this branch when ready, run the **full** `--browser-ui-gallery` matrix on the exact new head, retain the current-head receipts, then perform the existing independent visual gates and owner screenshot approval. Do not treat E1 as release approval.

## Round 2 — 2026-10-15 (structure ready; E1, acceptance NOT_RUN)

Base/head supplied: `6b7b766048cd0370a1cc3489b047600348199ae3`, PR #69 draft. Same worktree/branch; executor Codex (runtime model identifier unavailable), no delegated agents. Four requested commit groups: A, B1–B2, B3–B5, B6–B13. No push/PR/test/browser/server authorized; only `node scripts/dev/check-ds-rules.mjs` may execute locally.

### A — CI function diagnosis and repair

Historical RED evidence: CI run `38700369043` at `6b7b7660`; supplied `ui-gallery/click-ledger.json` has 830 rows, 42 fail: each of the seven interactions fails in all 3 locales × 2 viewports. Artifact root: `/private/tmp/claude-501/-Volumes-data-live-commerce-architecture-v1/d13e4b5b-6b4d-4249-8530-ca9ae1ef506c/scratchpad/g69art/playwright/run.JDBl4nHD/ui-gallery/`. Review: main worktree `output/visual-review/gallery-38700369043/{integrator-audit.md,jev.txt}`. These are old-head RED evidence, never a current-head acceptance result.

| Item | Cause from ledger + source | Change / evidence pointer |
| --- | --- | --- |
| A1 | `detail: Internal note` reports strict-mode resolution to two elements. `Card` labels its section with the heading, while the textarea Field used the identical `t.note`. | `apps/admin/components/UiGalleryTemplates.tsx:132`: distinct localized `t.noteContent` field label. `apps/admin/src/ui-gallery-template-copy.ts:8`: three locales. Test queries follow the new visible label, add uniqueness, retain fill/save/reload/time assertions. |
| A2 | All 18 form failures occur at required Name or Price fill. `packages/ui/src/ds/Fields.tsx:18` placed the literal required star inside `<label>`: exact label text was `商品名稱 *` / `商品名称 *` / `Product name *` (and price equivalents), while the journey asks for the bare copy with `exact:true`. aria-hidden hides the star from assistive naming, but does not remove it from Playwright label text. General-section screenshots show the name/description controls rendered; name fails first, before description is reached. Pricing is selected by the preceding keyboard assertion. | `packages/ui/src/ds/Fields.tsx:18`, `packages/ui/src/ds/fields.module.css:7`: required mark is a sibling of the exact label; native required/error/help/counter unchanged. No readiness/editability guard removed. `ds-rework.test.mjs`: exact raw label + required sibling + native required regression assertions. `ui-gallery-clicks.mjs`: name uniqueness before original interactions. |
| A3 | All 18 settings failures target required Store name, including final `inputValue` after returning from Notifications. Same label-star mismatch, not editability: inputValue does not wait for editable state. Supplied profile screenshot shows the input present; the readOnly guard is only saving/initial hydration. | Shared Field fix covers store name. Added exact-label uniqueness; retained every original settings value/save/cancel/sub-nav/reload assertion. |

Diagnosis is source/ledger analysis; no local browser reproduction is claimed. Browser GREEN and injected-fault RED→GREEN remain NOT_RUN and belong to integrator CI on the final SHA.

### Visual repair groups

| Item | Cause | Change (paths below are relative to the DS/gallery source directories) |
| --- | --- | --- |
| B1 | Explicit phone flex-wrap and all desktop tools retained; no store-name slot. | `packages/ui/src/ds/AppShellV2.tsx:178`, `shell2.module.css:86`: single fixed 50px row, phone store name; desktop-only tools hidden and mobile account slot. `apps/admin/components/UiGalleryFrame.tsx:33`: profile, all 3 locales, help and external store link in mobile account. Existing help/store/profile/locale assertions adapted to their new locations; added all-item keyboard cycle, focus restore, 50px/44px geometry. |
| B2 | Min-height alone plus legacy button padding yielded intrinsic sizes; section tabs 40px, help 24px, topbar mixed sm/ghost. | `packages/ui/src/ds/Controls.tsx:65`, `controls.module.css:17`: explicit 28/36/44 border-box heights; iconOnly square; phone 44. `Data.tsx`: MenuButton uses same sizes and optional icon-only/ghost, menu items 36/44. `data.module.css`: search/paging same scale. Shell right cluster 36 desktop/44 phone. `layout.module.css`: help icon 28. Sidebar nav keeps specification-mandated 52px rows (navigation, not action buttons). |
| B3 | Native sticky bottom moved the footer up from its reserved position inside the white card, leaving a white hole and overlaying the lower controls in full-page screenshots. No inner-scroll CSS exists in the old TabPanel; the screenshot alone did not establish an actual inner scroller. | `packages/ui/src/ds/Layout.tsx:139` / `layout.module.css:132`: natural-height, overflow-visible content, separate white form surface and normal-flow sticky footer within one connected frame. The displaced footer footprint is outside the white card. Added real keyboard page-scroll docking/release assertions, no inner overflow, switch click and section persistence; original two-bar/sticky assertions retained. CI screenshot verification NOT_RUN. |
| B4 | Whole-nav overflow plus shrinking flex children let the last group sit below the viewport; no dedicated list scroll region. | `packages/ui/src/ds/shell2.module.css:12`: 100dvh sticky rail with non-shrinking brand/store; groups flex:1/min-height:0/overflow-y:auto. Added real Settings click plus list-scroll/viewport/page-scroll isolation checks. |
| B5 | Frame used `template` route slug as crumb text and always used online-store parent. | `apps/admin/components/UiGalleryFrame.tsx:18`: typed TemplateName and existing localized nav groups; orders→list→detail, products→new product, settings→store settings. Tests assert all localized crumbs/current item and parent click/reload for every template in all six cases. |
| B6 | SVG already had 16px attributes but its layout box had no explicit flex basis; review flagged the small chevron. | `packages/ui/src/ds/AppShellV2.tsx:152`, `shell2.module.css:57`: fixed 16px slot with the 16px Icon inside; slot rotates 90° on expand. `ui-gallery-clicks.mjs:424`: slot dimensions/flex basis and actual expanded/collapsed rotation, alongside original mouse toggles. |
| B7 | Same cause as A1: repeated note heading/label. | `apps/admin/components/UiGalleryTemplates.tsx:132`: card remains Internal note; field becomes Note content / 備註內容 / 备注内容. |
| B8 | Status/date were rendered by DetailTemplate below the entire PageHeader. | `packages/ui/src/ds/Layout.tsx:107`, `layout.module.css:61`, `UiGalleryTemplates.tsx:53`: PageHeader.status in the h1 row; date in description. `ui-gallery-clicks.mjs:796`: header status containment and desktop baseline overlap. |
| B9 | Bulk command and preview-state selector were children of FilterBar. | `UiGalleryTemplates.tsx:89` / `:109`, `Data.tsx:299`: preview toolbar outside card, filter/search only inside; `selection.actions` displays bulk menu on selection bar only. Added placement assertions and selection-bar action check; original selection/confirmation/reload intent retained. |
| B10 | Row used a more SVG as MenuButton label, and MenuButton always appended a caret. | `packages/ui/src/ds/Data.tsx:278`: icon-only more mode, one SVG, explicit aria-label and scale-matched square trigger. Browser assertions count SVG/name/icon mode before the existing real click; SSR icon-menu assertion also added. |
| B11 | Table had horizontal overflow but no visible mobile cue or keyboard focus stop. | `Data.tsx:302`, `data.module.css:103`: localized persistent chevron/text cue, focusable native horizontal scroller. Both gallery tables pass translated hints. `docs/design/ds/DS-2.md` documents horizontal scrolling as the single rule. Browser ArrowRight/reload/no-page-overflow checks added; row action real clicks remain. |
| B12 | Store-name Field had a width cap; currency/timezone filled their cells. | `UiGalleryTemplates.tsx:258`: full width relative to each form row/grid column; no store-name cap. Browser checks full rule/equal grid cells and full-width phone rows before original edit/save flow. |
| B13 | Five UNCERTAIN items require separate dispositions. | See explicit table below; fixes and source arguments, not a claimed visual PASS. |

### B13 — Jev UNCERTAIN dispositions

| Finding | Disposition and pointer |
| --- | --- |
| icons_text_glyphs 0.42 | **Argue existing SVG rule + remove extra glyph-like clutter.** `packages/ui/src/ds/Icon.tsx:34` is the sole SVG owner (24 viewBox/1.75 stroke). All chrome uses Icon; badges stay text-only. Data punctuation and separators express content, not icons. Row more now has one SVG; B6 keeps a 16px optical slot. Existing icon SSR tests remain. R4/R5 static result below; rendered review NOT_RUN. |
| topbar_mixed_styles 0.45 | **Fix.** `UiGalleryFrame.tsx:29`: shared secondary locale/store controls, shared ghost help/bell/account icons, common 36px desktop /44px phone scale. Mobile utilities are in the account disclosure. |
| form_input_arbitrary 0.48 | **Fix.** `fields.module.css:13`: shared explicit 36/44 heights, full width relative to grid cells; Textarea is resizable and unit input has the same outside height. `UiGalleryTemplates.tsx:182` and `:258`: pricing/settings row widths unified to full. No new arbitrary input width. |
| card_padding_inconsistent 0.47 | **Fix nested settings inset; retain content-driven cards.** `layout.module.css:99`: no double SettingsLayout+FormActionBar padding or nested border. Card header/body/footer retain token spacing 16×24/24/16; the phone form inset remains the existing 16px responsive variant. Different content heights are intentional; cards have no minimum height. `DS-2.md` records the rule. |
| disabled_primary_faint 0.44 | **Argue token pair, add computed-browser assertion.** `tokens.css:161` defines disabled text/background, `controls.module.css:48` applies them at opacity 1. Existing token gate requires ≥4.5:1 (unchanged, stronger than ≥3:1). `ui-gallery-clicks.mjs:39` adds computed browser contrast ≥3:1 and opacity=1 before original disabled-click/keyboard assertions. Token/browser execution NOT_RUN; no measured-browser PASS claimed. |

### Tests authored and evidence limits

- Existing assertion intent is retained throughout. Note queries follow the newly distinct visible field label. Mobile help/profile/locale/store queries follow the required move into Account. The FormActionBar SSR assertion names the same content plus its new data attribute; no assertion deleted, weakened or bypassed.
- New/extended browser assertions: exact-label uniqueness; mobile menu keyboard coverage; topbar 50px and control 36/44 geometry; last sidebar group/list scroll isolation; chevron box/rotation; localized breadcrumb parent click/reload; form page-scroll dock/release + switch click; status title row; list controls/selection bar; more single icon/name; native table horizontal scroll/cue/reload; settings width consistency; computed disabled contrast.
- New/extended SSR assertions in `tests/ui/ds-rework.test.mjs:60`: exact required-label text and sibling mark/native required; connected form surface; icon-only menu naming/one SVG; status title-row placement; localized scroll hint/focusable native group.
- Existing historical RED is retained in CI38700369043 and its supplied click ledger. All new assertion/mutation RED→GREEN runs are **NOT_RUN**; authored tests are not a proven gate until CI executes them on the new SHA.

| Check | Exit / result | Evidence |
| --- | --- | --- |
| `node scripts/dev/check-ds-rules.mjs` | 0; 20 files in scope, 0 violations | E1 static only; `docs/design/ds/round2-static.txt`, bound to `source-sha256.txt` file hashes |
| TypeScript / lint / unit / token / hex ratchet / build tests | NOT_RUN | Local tests prohibited; no dependencies installed or lockfile changed |
| Browser gallery clicks / axe / visual-lint / screenshot matrix | NOT_RUN | Integrator CI on new SHA; no local browser or server |
| New assertion fault injection RED→GREEN | NOT_RUN | Integrator must prove each new gate |
| Independent Jev / integrator visual review / owner sign-off | NOT_RUN | Required before acceptance or live adoption |

### Risks / handoff

- This round delivers source and authored tests at **E1**, not E3 or visual acceptance. The old 42-failure CI receipt is explicitly old-head RED evidence. No fix is declared browser-green from source inspection.
- Sticky footer behavior, sidebar scrolling, full-page screenshot positions, font rendering, longer en labels, computed contrast and asynchronous save/hydration timings need current-head CI. The B3 footer keeps a normal-flow footprint outside the white surface; reviewers must inspect both docked and released screenshots.
- New public DS options are additive: Button.iconOnly; MenuButton.icon/variant/lg and external item target; AppShellV2.topbar.mobileAccount; PageHeader.status; DataTable.scrollHint and selection.actions. No new package, transaction engine, live route or backend change.
- No P1 is waived. P2/UNCERTAIN arguments above require independent review. No blocker was retried twice; none hidden. No test/browser/server/process/fixture/container was started; no temporary artifacts remain to clean up.
- The integrator retains PR69 push/CI/review ownership. No push, PR creation, merge or deployment occurred here. Preserve all original ledger assertions and run the full three-locale × two-viewport gallery gate, then independent visual review and owner screenshot approval.

### Round 2 commit groups

- A: `2a1f9f10d`
- B1–B2: `2055d9bc2`
- B3–B5: `6a618b5cf`
- B6–B13: the commit containing this Round 2 handoff, source manifest and static receipt (see branch log).

---

## Round 1 — historical handoff (superseded by Round 2 above)

Role: ui_worker (write); executor: Codex, task requested reasoning high (exact runtime model identifier is not exposed by this session). No delegated agents. Worktree `/Volumes/data/live_commerce_architecture_v1/.worktrees/ds-2`; branch `unit/ds-2-rework`; base `e4dad98cc322d35709b475de25063efc92e5d866` on `r3/integration`.

Implementation/test source SHA: `7ee79c1cd5e652c5d33a8bff77435c6c99b2da80`. This delivery and source-hash manifest are committed afterward with no implementation changes. File hashes: `docs/design/ds/source-sha256.txt`. Status is E1 **JavaScript syntax/source structure only**, not TypeScript compilation, E3 acceptance or visual sign-off. Browser tier: MOCK/synthetic. All runtime gates below are NOT_RUN.

Write scope stayed in the assigned DS/gallery/tests/design paths. This requested `output/ds-2/DELIVERY.md` is the task card's explicit deliverable exception. No live page adoption, dependency/lockfile, Go or migration changes. No push, PR or local test/server/browser was run.

## Commits

| Commit | Component group |
| --- | --- |
| `1ea66bc6a` | Authored SVG icons, controls, SHOPLINE shell and tokens |
| `bab2c32e0` | Fields, form bars, upload/detail/state patterns |
| `204b91434` | Shared localized frame and four gallery-only templates |
| `6ffb25afa` | Expansion/collapse persistence, form saving guards, menu positioning and component spacing |
| `7ee79c1cd` | Preserved legacy assertions; six-case template/component CI matrix and render contracts |

## Fourteen audit items → implementation pointers

All entries describe source changes; visual/function conclusions require the pending gates.

| # | Change and source pointer |
| --- | --- |
| 1 | Single authored SVG `Icon` covers every requested symbol; 24 viewBox/1.75 currentColor, 16/20/24. `packages/ui/src/ds/Icon.tsx:1`; icons applied in `AppShellV2.tsx:119`, `Controls.tsx:70`, `Data.tsx:125`, `States.tsx:34`, and `apps/admin/src/ui-gallery-nav.ts`. No vendor paths copied, therefore no Lucide notice required. |
| 2 | 270/52 tokens, fixed 24 icon slot, rotating 16 chevron, child hover; validated tab-local expanded AND collapsed preferences. `packages/ui/src/tokens.css:164`, `packages/ui/src/ds/shell2.module.css:33`, `packages/ui/src/ds/AppShellV2.tsx:79`. |
| 3 | Single language disclosure, ghost help icon, consistent right-cluster controls, secondary store link/external SVG; 50 px desktop bar. `apps/admin/components/UiGalleryFrame.tsx:11`, `packages/ui/src/ds/AppShellV2.tsx`, `packages/ui/src/ds/shell2.module.css:66`. Mobile wraps deliberately to retain every action and touch target. |
| 4 | Underline=page, boxed=section; padded bordered panel and neutral count pill. `packages/ui/src/ds/Controls.tsx:157`, `controls.module.css:78`, `controls.module.css:142`; usage rule in `docs/design/ds/DS-2.md`. |
| 5 | Badges uniformly text-only; semantic domain mapping remains unchanged in `logic.ts`. `packages/ui/src/ds/Controls.tsx:226`. |
| 6 | SVG icon/title alignment, top-aligned SVG dismiss, light tint/neutral thin outline, optional right action. `packages/ui/src/ds/Controls.tsx:247`, `controls.module.css:119`. No heavy left bar. |
| 7 | Default compact (~200 px) and full modes, 48 px soft SVG circle and aligned next-step text. `packages/ui/src/ds/States.tsx:12`, `states.module.css:4`, `states.module.css:47`. Gallery keeps one main action. |
| 8 | Field owns labels/help/errors/counter/maxLength; native select/textarea/unit/checkbox/radio and segmented switch; 160/320/480/full widths and two→one-column grid. `packages/ui/src/ds/Fields.tsx:7`, `fields.module.css:1`, `tokens.css:151`; template `UiGalleryTemplates.tsx:133`. |
| 9 | One connected FormActionBar, desktop top+sticky bottom and phone bottom only; dirty/saving/saved, guard edits while saving. Readable disabled pair is added to the 4.5:1 gate. `packages/ui/src/ds/Layout.tsx:136`, `layout.module.css:81`, `tokens.css:161`, `tests/ui/ds-tokens.test.mjs:38`. Computed contrast gate NOT_RUN. |
| 10 | Tight Dropzone, visible format/size hint, controlled thumbnail/progress/error/retry/remove grid and simulator. `packages/ui/src/ds/States.tsx:132`, `States.tsx:231`, `states.module.css:28`, `apps/admin/components/UiGalleryTemplates.tsx:184`. |
| 11 | Fixed number/label/delta rows reserve optional delta; entire KPI remains a link. `packages/ui/src/ds/Layout.tsx:215`, `layout.module.css:132`, `apps/admin/components/UiGallerySections.tsx:250`. |
| 12 | Card header/body/footer spacing tokens, no card min-height, removed upload/button blank result spacing. `packages/ui/src/tokens.css:157`, `packages/ui/src/ds/Layout.tsx:30`, `layout.module.css:25`, `apps/admin/components/UiGallery.module.css`. |
| 13 | 28/36/44 desktop heights/padding, SVG spinner, consistent icon gap, one primary per action row; phones retain 44 px targets. `packages/ui/src/ds/controls.module.css:17`, `tokens.css:154`, `apps/admin/components/UiGallerySections.tsx:62`. |
| 14 | TC system stack plus locale-aware SC stack, existing type scale plus line-height tokens, single blue card-title rule. `packages/ui/src/tokens.css:147`, `packages/ui/src/ds/shell2.module.css:93`, `layout.module.css:26`. Existing darker accessible blue is retained rather than copying the reference blue below text contrast. |

## Components and templates

Detailed props/usage rules: `docs/design/ds/DS-2.md` (“New and extended APIs”). New APIs: Icon; Field with render-prop control associations; native TextInput/Select/Textarea/NumberInput/Checkbox/RadioGroup; FormGrid; FormActionBar (ActionBars alias); UploadGrid/UploadItem; Timeline/TimelineItem; DetailLayout; Stack/Inline. Extended APIs: Card.footer, states.mode, AppShellV2.persistenceKey and IconName group icons, Button ghost. No new npm dependency.

| Template | Source entry | Key behavior |
| --- | --- | --- |
| List | `apps/admin/components/UiGalleryTemplates.tsx:67` | Synthetic orders, filters, private memory-only search, export/menu, select-page vs select-all, confirm bulk, row actions, pagination, loading/empty/error+recovery, confirmed demo draft creation |
| Detail | `UiGalleryTemplates.tsx:112` | Header/status/actions, main+summary columns, ISO timeline, saved mock note |
| Form | `UiGalleryTemplates.tsx:133` | Section tabs, native field grid and validation, save/dirty/discard, upload/error/retry/remove |
| Settings | `UiGalleryTemplates.tsx:227` | In-card sub-nav, store name/currency/timezone, notification preference, save bar |

Routes: `/{zh-TW,zh-CN,en}/ui-gallery/templates/{list,detail,form,settings}`. Guard is first in `apps/admin/app/[locale]/ui-gallery/templates/[name]/page.tsx:16`; unknown locale/template returns 404. All use `UiGalleryFrame` and AppShellV2. Three-locale copy is typed in `apps/admin/src/ui-gallery-template-copy.ts`; no page-specific template CSS.

## Assertions added (authored, NOT_RUN)

- `tests/ui/ds-rework.test.mjs:35`: icon geometry/stroke/sizes/names and decorative/labelled semantics; text-only badges and SVG spinner/alerts; Field id/error/help/counter/maxLength positive+negative associations; connected FormActionBar and single status announcer; upload progress/error/retry/remove names and no retry on non-error rows; explicit compact/full states and ISO timeline. `ds-tokens.test.mjs:9` imports these through both existing explicit CI entries. Existing SSR loader uses real Icon; no legacy assertion deleted or weakened.
- `tests/ui/ds-tokens.test.mjs:38`: disabled text/background in the existing 4.5:1 matrix. Legacy `--ui-*` checks and hex ratchet unchanged.
- `tests/admin/ui-gallery-clicks.mjs:465`: expansion and collapse survive reload, dialog focus containment/Esc/restoration, dropdown language; existing component search now asserts no q/pq URL parameter.
- `ui-gallery-clicks.mjs:509`: three locales × 1440/390, five baseline pages, all-impact axe=0 (stricter than the original runner), blocking visual-lint=0, no Next error overlay, screenshot SHA-256. Expected complete DS-2 manifest: 30 baseline +36 variant captures. Original six screenshot cases and original two click journeys remain.
- `ui-gallery-clicks.mjs:563`: menu keyboard command, filter→URL/reload, page/all-results confirmation, row select/clear, private search checks on current/linked URLs, pagination/reload, row actions, list loading/error/empty persist/recover, order links, demo draft persistence; detail note and actions reload; form disabled→enabled, all field types, error association/discard/Esc, tab keyboard+numeric persistence, real filechooser/error/retry/progress/save/reload/remove; settings fields/cancel/sub-nav/save/reload; template navigation links.
- Real click/fill/check/selectOption/key input drives product state. evaluate is read/measurement except one explicitly labelled reset of `ds2-` sessionStorage keys before each synthetic matrix fixture. No forced click, dispatchEvent, DOM mutation or direct API state transition substitutes for a user action.

## Commands and evidence

| Command/check | Exit / result | Evidence level |
| --- | --- | --- |
| `git diff --check` | 0 | E1 whitespace/source structure only |
| `node --check tests/admin/ui-gallery-clicks.mjs` | 0 | E1 JavaScript parse only; no imports/tests executed |
| `node --check tests/ui/ds-rework.test.mjs` | 0 | E1 JavaScript parse only |
| `node --check tests/ui/ds-render.test.mjs` | 0 | E1 JavaScript parse only |
| `node --check tests/ui/ds-tokens.test.mjs` | 0 | E1 JavaScript parse only |
| TypeScript compilation / build / unit tests / hex ratchet | NOT_RUN | CI only per task; dependencies not installed locally |
| `bash scripts/dev/test-local.sh --browser-ui-gallery` | NOT_RUN | Integrator CI at actual pushed SHA; no local browser run |
| All click journeys, axe, visual-lint, screenshots | NOT_RUN | Planned evidence: existing ui-gallery evidence directory: `ds2-matrix.json`, `click-ledger.{json,md}`, `shots/ds2-*.png`; existing workflow uploads browser evidence tree |
| New-gate injected-fault red→green | NOT_RUN | Required before treating the authored assertions as proved gates |
| Independent Kimi vision review + integrator audit | NOT_RUN | Integrator uses `output/integrator/tools/visual-review-prompt.md`; PASS requires no P1 |
| Owner screenshot sign-off | NOT_RUN | Required before any live-page adoption |

Local commands above do not provide E3. No runtime evidence files or screenshots are fabricated. No local processes, containers, ports or test fixtures were started, so none remain to clean up. Source file hashes bind these authored changes; they do not prove behavior.

## Risks, limitations and handoff

- TypeScript, layout, keyboard behavior, contrast measurements, CI timing and screenshot quality are unverified. Independent review must identify and repair any failures before acceptance.
- Gallery persistence is sessionStorage/mock-only; blocked storage falls back to memory. Upload stores metadata, not bytes: after reload/remount thumbnail uses SVG fallback. No real upload/export/receipt/archive/notification operation occurs.
- Browser matrix extends the permitted click module because runner/workflow are outside write_paths. It runs sequentially on the same Page; the legacy final console banner still says DS-1/six cases. Acceptance must inspect `ds2-matrix.json` and click ledger, not that banner alone. Reused browser context has its original browser locale; route locale/HTML lang/copy still vary for all six cases.
- The wrapper captures all-impact axe including minor/moderate; any such finding intentionally fails the gate. Incomplete/aborted captures are not PASS. Existing runner timeouts and artifact retention remain integrator-owned.
- No blocker was attempted three times; no BLOCKED item was suppressed. Unavailable local runtime checks are intentionally NOT_RUN per authorization.
- Integrator owns pushing branch/PR/CI, current-SHA receipts, independent visual review and owner screenshot review. DS-1 is not signed off; DS-2 remains gallery-only until those gates and owner approval pass.
