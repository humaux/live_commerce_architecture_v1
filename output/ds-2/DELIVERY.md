<!-- Purpose: DS-2 implementation handoff, audit mapping and evidence limits. Depends on: task card and scoped source commits. Used by: Claude integrator/independent reviewer/owner. -->
# DS-2 delivery — implementation awaiting CI and visual acceptance

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
