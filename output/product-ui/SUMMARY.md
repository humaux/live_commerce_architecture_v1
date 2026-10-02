# product-ui — PARTIAL / BLOCKED, DO NOT MERGE

Date: 2026-10-03 (Asia/Shanghai). Scope is this worktree only; no Go/SQL, production, secrets, push or merge.

## Commits

| SHA | Change |
| --- | --- |
| `54b72548` | Product document/bulk/copy BFF routes, strict response models, media helper, model tests |
| `428889a7` | One staged product form, matrix, list workflows, three-language copy and scoped styles; old ProductVariants writer removed |
| `5a43bb2b` | Additive `--browser-product-editor` click mode; frozen catalog-core assertions unchanged |
| `c91f8cad` | Catalogue receipt fence across reload/navigation/session change; committed-copy/lost-response browser case |

All implementation commits include `Co-Authored-By: Codex <noreply@openai.com>`. Base is `e66560e7646ff405fef785fc183af6099b075bd4`, which contains product-core `39bb8ec`. The last source revision tested is `c91f8cad`; this evidence-only summary is committed separately.

## Requested work

| Item | Status and evidence |
| --- | --- |
| Unified create/edit layout; section navigation/readiness; fixed actions | PARTIAL: shared component and create workflow implemented. Existing edit is explicitly read-only because the backend cannot safely preserve hidden fields. Not a completed replacement. |
| Multi-image upload, cover, sorting, max 12 | Implemented for creation; browser clicked 3 uploads and reorder, then one publication. Existing-product media edits and all 12/13 edge interactions are NOT_RUN/incomplete. Native img retained. |
| Basics/keyword, major-unit price, tracking/∞ and max | Implemented for creation. Browser confirms USD60, untracked max3, exact keyed writes; pure tests confirm TWD60->6000 and reject fractional TWD. No claim of browser TWD acceptance. |
| Up to three axes / client matrix / batch fill | Implemented. Browser creates 3×4 locally, fills only-empty prices, fills quantity, selects warehouse, saves and reloads 12 rows. Escape returns focus. Some drag/all-fill cases remain NOT_RUN. |
| Collections, shipping and SEO | Controls implemented; exact stock warehouse selection executed. Collection assignment and full shipping/SEO persisted-click coverage remain NOT_RUN; collections currently load first 100. |
| Stage images after draft and before publish | Implemented; one document + three uploads + one order command verified, then authoritative image check + keyed bulk publication. Duplicate click does not create a second product. |
| List tabs/counts, smart filters, selection, copy/bulk results | PARTIAL: controls use returned counts; filters/sort explicitly page-local. Copy and three-product unpublish persisted. Exact live-window refusal, archive and collection-assignment flows NOT_RUN. Global smart-filter/search semantics need backend/ruling. |
| Inline price and inventory edits | BLOCKED and not enabled. Do not use aggregate `available` as warehouse on-hand or send a destructive full replacement. |
| Retire duplicate writers | Implemented: old editor/PATCH/variants UI flow removed. Backend legacy routes are not removed (other clients still use them). Existing-media/legacy browser regressions must be resolved before merge. |
| Publish requires image (UI only) | Implemented in create and list publication. No Go enforcement added. |
| Three locales and requested viewport screenshots | 23 captures plus SHA manifest. List/editor: zh-TW, zh-CN, en × 1366×768 / 1586×992 / 375×812. Additional matrix, batch result, two inventory and mobile final-field captures. |
| UNKNOWN/scope safety | Same command/key retry while mounted; receipt UUID fence after page loss blocks fresh duplicates pending reconciliation. Browser fault injection committed a copy then discarded the response; reload prevented a second command. Independent scope review passed its bounded audit. Separate tabs do not share sessionStorage. |

## Gates

All commands run from this worktree. `test-node` includes 291 existing tests + 6 product model tests. Pinned legacy R04 binary probe is NOT_RUN when the optional binary is unset, as the script reports. Browser test data is isolated; identity/provider transport is MOCK and PostgreSQL is real local test storage, **not LIVE production acceptance**.

| Command | Exit | Result / evidence |
| --- | ---: | --- |
| `bash scripts/dev/test-node.sh` | 0 | 297 pass, 0 fail — `gate-test-node.log` |
| `pnpm --filter admin exec tsc --noEmit` | 0 | `gate-tsc.log` (empty output, successful exit) |
| `bash scripts/dev/check-gates.sh` | 0 | `gate-check-gates.log`; also rerun after staging new tests |
| `bash scripts/dev/test-local.sh --browser-product-editor` | 1 | Final `c91f8cad` run reaches required PE14 editable-save assertion and fails. Working subsets and receipt fault test precede it; frozen CC12 cases then skip due existing serial mode. `gate-product-editor-final-c91f8cad.log` |
| `bash scripts/dev/test-local.sh --browser-catalog-core` | 1 | Final `c91f8cad`: old name-only creation/redirect waits its unchanged 420-second timeout; 1 failure, second serial case not run. `gate-catalog-core-final-c91f8cad.log`, `red/catalog-core-final.log` |
| `bash scripts/dev/test-local.sh --browser-catalog-media` | 1 | Final `c91f8cad`: required `photo-manager` is absent; existing-product media edits not restored. `gate-catalog-media-final-c91f8cad.log`, `red/catalog-media-final.log` |
| `bash scripts/dev/test-local.sh --browser-admin-legacy` | 0 | Final `c91f8cad`: ledger, production fail-closed, identity mock and entry mock suites pass. `gate-admin-legacy-final-c91f8cad.log` |
| `bash scripts/dev/test-local.sh --browser-merchant-buyer` | 1 | Final `c91f8cad`: helper still waits for old new-product `/products` response; timeout 30 seconds unchanged. `gate-merchant-buyer-final-c91f8cad.log`, `red/merchant-buyer-final.log` |

Additional checks: `git diff --check` exit 0; Impeccable detector exit 0 (`design-detector.json`: `[]`). Model tests do not constitute PE14 edit acceptance.

## Stop line and handoff

1. **Backend:** detail does not expose complete editable per-SKU keywords/membership; global logistics overwrite heterogeneous SKU data. Full replacement contradicts PE14 partial-change intent. `target_qty=0` is skipped. Correct the contract before enabling existing-product writes. Evidence and exact symbols are in `REVIEW.md`; no Go patch was made.
2. **UI/test follow-up:** restore supported existing-product media workflows and adapt old flow drivers without removing their substantive assertions. Merchant-buyer also has Go-level old-route fault injection, outside this unit's authority. Do not merely rename a locator to claim functionality passed.
3. **Acceptance:** full PE12 (including TWD buyer UI/category), PE14/15, axe/keyboard-only, every-control G-UI8, broader data/permission/error coverage are not complete. Full NOT_RUN ledger is in `REVIEW.md`. Existing failing assertions and thresholds remain unchanged.
4. **Reconciliation:** receipt fences prevent unsafe same-tab replay; after a page loss an administrator must reconcile the command, because no automatic command receipt endpoint is available. Do not silently clear the fence or claim full recovery.

Evidence retained in the user-requested **worktree** output directory, overriding PROCESS.md's generic main-checkout placement. No main-checkout files were written. `screenshots.json`, `click-ledger.json`, `write-metadata.json`, all logs, `REVIEW.md` and screenshots accompany this handoff. Humaux research/fix records and task canvas were updated; code graph indexed and the safe edit guard linked to the backend evidence memory.

Final gate count: **4 exit 0, 4 exit 1**. There is no release/merge approval. All 23 screenshot hashes match their manifest; the click ledger contains 14 passing scoped records and 1 blocked editing record, not 15 fully covered features. Failed-run images/logs are preserved under `red/` and must not be used as passing visual proof. The test harness also produced `output/admin-ui-fixes/`; it is retained as generated regression evidence, not an unrelated code edit.

Final browser fixture directories: product-editor `catalog-core/20261002T164226.445950000`; legacy core `catalog-core/20261002T164257.592486000`; media `catalog-media/20261002T165013.677851000`; merchant-buyer `merchant-buyer-real-3357591713` under this worktree's `output/playwright/`. Source contents stayed at `c91f8cad` throughout these final runs. Test harnesses exited and cleaned their owned local runtimes; no customer process was stopped.
