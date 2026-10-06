<!-- Purpose: W3-U1b current CI repair handoff and evidence limits.
Depends on: r3/integration71235, CI37470015577, focused MOU07 and main evidence logs.
Used by: integrator re-review and GitHub rerun; no production acceptance. -->
# W3-U1b CI repair delivery

- Branch `unit/w3-u1b-picklist-ui`, trunk71235fc4 merged before checks. Final commit: containing this file (actual SHA in handoff/commit-receipt).
- Supersedes author4a228b24/int f3f97fcf CI37470015577. Base `71235fc40db20189583c08057f8e21fd8dcbd10e`. Parent Codex GPT-6, independent read-only review by existing explorer; no backend runtime/schema/dependency edits.
- Source binding `0ba2e641d024ee10db05db4952cb46c0722c0e6a7c636e59c3316685d0def1ea` in `ci-37470015577/source-manifest.json`.

MOU07's first row was y982>=844 because the always-expanded pick/export/CVS controls consumed the phone viewport. Batch controls now use a compact native disclosure, expand when a selection or valid live-session scope exists, and preserve scope/cap500/print/command behavior. The selected count remains visible while closed.

Click-sweep r00208 placed page selection inside a table header clipped to1x1px on mobile. The same checked/indeterminate/cap500/handler control now sits in a visible list-tools row outside the header. The original viewport threshold stays unchanged; MOU07 additionally checks/unchecks the visible checkbox with real clicks. Native-device merge preserves current Linux --no-sandbox plus owned-child FD diagnostics/cleanup.

## Evidence and exits

All paths below are main `output/w3-u1b-picklist-ui/ci-37470015577/`.

| Check | Exit | Evidence |
|---|---:|---|
| CI old UI red |1| run37470015577 artifacts: MOU07 y982, gate-4/ui-click-sweep/ledger rowr00208/screenshot; original tests intact |
| focused Node red→green |1→0| `node-red.log`→`node-final.log`,12PASS/0FAIL |
| admin TypeScript |0| `types-first.log` |
| strict MOU/native spec TypeScript |0| `browser-types.log` |
| admin production build |0| `build-green.log`; first setup lacked synthetic company email (`build-red.log`,1), configured build0 |
| `LC_TEST_LOCK_WAIT=600 LC_BROWSER_MERCHANT_ORDERS_UI_ACCEPTANCE=1 LC_FOCUSED_TAGS=browser LC_FOCUSED_TIMEOUT=700s bash scripts/dev/test-focused.sh '^TestBrowserMerchantOrdersUIFocused$'` |0| `focused-green.log`; actual Playwright MOU07 passed and PG readonly facts passed |
| `bash scripts/dev/check-gates.sh` |0| `gates.log`,72modes/header ratchet |
| `git diff --check` |0| clean |

Local focused MOU07 covers3locales×1586/390, unchanged first-row threshold and added real page-checkbox clicks. Recorded first-row y: zh-TW752.492, zh-CN752.492, en749.492 (<844). Durable full runtime tree and orders-v2 screenshots/ledger under `local-focused/`. Local pre-fix red request never ran (sharedPG queue; only verified ownPID18823 stopped, exit143/NOT_RUN); existing CI red supplies the reproduced defect evidence.

The focused Go entry is separate. Original `TestBrowserMerchantOrdersUIRealChain` still runs every spec/assertion and native proof. Only the focused entry selectsMOU07, retains PG read-only checks, and explicitly reports unrelated MOU/native cases NOT_RUN. No full-gate thresholds, assertion or fixture weakened. Independent source review found no proven regression/gate weakening. E3 here = focused real browser + realPG with MOCK IdP; not complete W3 acceptance.

## CI gates / NOT_RUN

Integrator pushes/runs:

- `bash scripts/dev/test-local.sh --browser-merchant-orders-ui`
- `bash scripts/dev/test-local.sh --browser-picklist`
- `bash scripts/dev/test-local.sh --browser-cvs`
- `bash scripts/dev/test-local.sh --browser-click-sweep`
- `bash scripts/dev/test-local.sh --browser-visual-lint`

All full modes/native proof/complete click sweep/visual lint are NOT_RUN locally this repair turn. LC-U1 findings are outside this unit and were not edited. No release merge/push/provider action. Earlier feature delivery and historical evidence remain in main output; current CI repair supersedes prior author SHA.
