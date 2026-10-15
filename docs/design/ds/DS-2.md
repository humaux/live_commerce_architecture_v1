<!-- Purpose: DS-2 component contracts and review rules. Depends on: SHOPLINE reference, NEEDS and DS sources. Used by: integrator and later page units after owner sign-off. -->
# DS-2 component and composition contracts

Status: implementation for gallery review only. Browser, compilation, independent visual review and owner acceptance are NOT_RUN. No production page should adopt this package until those gates pass.

The measured target is [reference-shopline-admin.md](../page-inventory/reference-shopline-admin.md). The nine IA groups and twelve cross-page patterns remain governed by [NEEDS.md](../page-inventory/NEEDS.md). No package, transaction engine or business API was added.

## Visual rules

- Shell: 270 px dark rail, 52 px first-level rows, 24 px icon slot, rotating SVG chevron, plain indented child links; desktop topbar 50 px. Phones use the existing modal drawer and a wrapping topbar.
- Type: system/TC stack, SC variant under `html[lang=zh-CN]`; body 14/1.5 and the existing 12/13/14/16/18/22/28 scale. Card titles use the blue link token; page and field headings use neutral text.
- Buttons: 28/36/44 px desktop scale; phone targets at least 44 px. Primary action at most once in each action row. Disabled buttons remain focusable and explain the reason. The disabled foreground/background pair is included in the existing 4.5:1 gate (stricter than the task's 3:1).
- Cards: header 16 px block/24 px inline, body 24 px, footer 16 px block. Content determines height. KPI cards reserve number/label/delta rows and keep the entire card as a link.
- Tabs: underline for page views; section/boxed for form sections. Panels own 24 px padding and a border. Counts are neutral pills. Arrow keys, Home and End follow the existing automatic-activation pattern.
- Badges are uniformly text-only; semantic payment/shipping/refund/connection mappings stay in `logic.ts`. Alerts use tint plus a neutral thin outline, no heavy accent bar; SVG icon and dismiss control align with the title row.
- State blocks default to compact (~200 px); `mode="full"` is for page states. Each has a 48 px soft icon circle. Callers supply one main action and aligned reason/next-step copy.

## New and extended APIs

| Component | Contract |
| --- | --- |
| `Icon` | `name: IconName`, `size?: 16 \| 20 \| 24`, optional `title`, standard SVG props. Authored paths, 24 viewBox, 1.75 currentColor stroke. Decorative by default; a title makes it an accessible image. |
| `Field` | `label`, `required?`, `help?`, `error?`, `width?: sm \| md \| lg \| full`, `counter?: {value,max,format}`, render-prop `children(control)`. Spread `control` on the actual input/select/textarea. Label, error/help/counter ids and maxLength have one owner. |
| `TextInput`, `Select`, `Textarea` | Native HTML input/select/textarea props. Width belongs to Field. |
| `NumberInput` | Native input props except type, plus `unit`; input is numeric. Include the currency/unit in the Field label for assistive technology. |
| `Checkbox` | Native checkbox props plus required `label`. |
| `RadioGroup` | `label`, `options: {value,label}[]`, `value`, `onChange`, optional `help` and `disabled`. Native fieldset/legend and same-name radios. |
| `FormGrid` | `children`; two equal desktop columns, one phone column. Width tokens are 160/320/480 px/full. |
| `FormActionBar` | `label`, `actions`, `children`, optional `state: clean \| dirty \| saving \| saved`, `dirty`, and localized `note/dirtyNote/savingNote/savedNote`. One form frame; desktop top and sticky bottom, phone bottom only. Actions render twice: no explicit duplicate ids. Existing `ActionBars` is an alias. Caller owns persistence and validation. |
| `UploadGrid` | Controlled `items: UploadItem[]`, `label`, localized `removeLabel/retryLabel/progressLabel/doneLabel`, `onRemove`, `onRetry`, optional `disabledReason`. Item: `id,name,src?,status,progress,error?`. Shows native progress, explicit failure, retry and removal. No transport is implied. |
| `Timeline` | `label`, `items: {id,title,time,dateTime,detail?,tone?}[]`. Display time is localized by caller; `dateTime` is ISO. |
| `DetailLayout` | `main`, `summary`; 2:1 desktop columns and one phone column. |
| `Stack`, `Inline` | `children`; shared vertical rhythm or wrapping action row. |
| `Card` | Existing props plus `footer`; token padding, no component min-height. Metadata must have a clear meaning. |
| State components | Existing props plus `mode?: compact \| full`; caller owns next-step copy/action. |
| `AppShellV2` | Existing props plus optional `persistenceKey`; group icons use `IconName`. Tab-local expanded/collapsed preferences validate nav ids; first visit expands current group. |
| `Button`, `MenuButton` | Button adds `ghost`; loading uses SVG. Disclosure menu supports ArrowUp/Down, Home/End, Escape, outside click and focus exit. Fixed menu measures/clamps its position to avoid table clipping. |

## Gallery templates

Every route is guarded by `LC_UI_GALLERY=1`, has three locale copies and uses `UiGalleryFrame`/`AppShellV2` plus DS components. Template code adds composition and synthetic state, not independent page styling.

| Route suffix under `/{locale}/ui-gallery/templates/` | Review content |
| --- | --- |
| `list` | Order-style title/action group, filters, export, page/all-results bulk scope, thumbnail/status/action table, pagination, loading/error/empty selectors and recovery. Create demo draft is a confirmed local mock action. |
| `detail` | Status header, main cards/side summary, ISO timeline and persistent synthetic note. Receipt/archive only announce a labelled mock outcome. |
| `form` | FormActionBar, section tabs, labelled grid/select/textarea/unit/checkbox/radio/switch, validation, dirty/discard/save states and upload error/retry/remove queue. |
| `settings` | In-card left navigation, profile fields and notification preference with shared save bars. |

Only allowlisted non-sensitive filter/page/section/nav state enters URL parameters. Search is memory-only: neither `q` nor renamed `pq` is serialized. The helper stores allowlisted synthetic primitive records in sessionStorage, never real store/customer data. This is refresh persistence in one browser tab, not backend durability. Selected rows remain transient; all-results operations require confirmation and use the filter fingerprint contract.

Upload is an explicit simulator: 12 files at most, extension/5 MB validation, local objectURL previews and queued progress/failure. Save persists metadata only. A reload or tab remount uses the SVG placeholder because image bytes are not persisted. ObjectURLs and timers are disposed. Later real pages must provide their own authorized upload/persistence contracts.

## CI and acceptance

`tests/admin/ui-gallery-clicks.mjs` extends the existing allowed entry rather than modifying the out-of-scope runner. After the original zh-TW desktop journey it runs the whole component journey in each of six locale/viewport cases, then the four template journeys in each case. The same Page preserves correct click-ledger route attribution. The explicit sessionStorage reset is fixture setup before a case; persistence checks always use real interaction then reload.

The extension writes `ds2-matrix.json` and `shots/ds2-*.png` under the existing ui-gallery browser-evidence directory, including SHA-256, exact route/viewport, all-impact axe violations, blocking visual-lint and error-overlay count. There are 30 baseline captures plus 36 variant captures when all journeys succeed. Existing CI uploads the browser evidence tree. An aborted/incomplete manifest is not acceptance.

`ds-tokens.test.mjs` imports the new render-contract tests so both the explicit test-node list and `--browser-ui-gallery` execute them. Existing assertions remain. This import is necessary because the runner does not glob new test files and changing the runner is outside this task's write paths.

Integrator must run CI at the actual pushed SHA, verify full matrix/ledger completeness, retain screenshots and zero axe/blocking-lint findings, independently review with `output/integrator/tools/visual-review-prompt.md` (no P1), audit the fourteen items, and obtain owner screenshot sign-off. New regression assertions are authored but their injected-fault red→green execution is NOT_RUN. Do not substitute this document or source inspection for those gates.
