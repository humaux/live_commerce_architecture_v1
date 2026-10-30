# PARITY-SETTINGS-FIX1 — directed batch awaiting CI

Evidence date: 2026-10-30. PR #354, branch `unit/parity-settings`.
Failing head: `e9c1fe9ca90abe6b5b6d42ffde2a92853c1b6f44`.
Latest merged base: `2f61bf866` (origin/r3/integration fetched this session).
`git merge --no-edit origin/r3/integration`: exit 0; merge performed before fixes.
Working tree had existing untracked relay evidence; it was preserved.

Authority: owner FIX1 card; AGENTS.md; 架构.md §§0/1/2/26/27;
`contracts/merchant-settings-wizard-v1.md` including verbatim AD-04;
current settings source and the original GitHub logs/artifacts below.
Installed ponytail skill applied to root-cause tracing. Humaux/code-index tools
are unavailable in this session: no store/canvas/coord receipt is claimed.

## Evidence binding and paths

RED run: https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41062911958
PR head above; tested synthetic merge: `4b5021d8da38a92f9a1835720c54a8df800e4859`
(raw matrix-receipts lines in job logs). Failure evidence is E3 for this historical
run only; no GREEN or independent acceptance is claimed.

All local evidence paths below are relative to `fix1-evidence/` beside this file.
`artifacts.tsv` records original artifact IDs/names; `job-<id>.log` contains raw
`gh run view 41062911958 --job <id> --log-failed` output. Downloaded artifacts
retain the original screenshot, trace.zip, error-context and browser logs.
Line numbers refer to those raw browser logs, not agent summaries.

Class (a) = superseded navigation/copy, preserve every downstream assertion and
use real clicks. Class (b) = regression requiring a root fix, not assertion or
fixture weakening. Class (c) requires an identical failure on base. UNKNOWN is
explicitly retained where the evidence does not establish any of those classes.

## Every failing job → test → raw evidence → class

|Job / mode|Failing test|Evidence path and assertion|Class / disposition|
|---|---|---|---|
|124100558313 / store-domains|TestBrowserStoreDomains → openCard|`store-domains/playwright/store-domains-2307918659/browser.log:8-23`: Advanced/card locator absent, `store-domains-gate.mjs:219`; `failure-0-1793368882790.png` alongside log|a: signed overview replaces Advanced with Edit settings; existing domain, publication, width and persistence oracles must remain.|
|124100559012 / ads-attribution|TestBrowserAdsAttributionReport → audienceStateClicks|`ads-attribution/playwright/run.eIGySCn2/ads-attribution-report/20261030T141706.731125642/playwright.log:16-29`: Advanced click timeout, `attribution.spec.ts:522`|a: replace signed editor entry; also re-enter after reload because full-editor view is intentionally nonpersistent.|
|124100559012 / admin-legacy|TestBrowserAdminLedgerFixtureChain → approved ledger reproduction and mobile table remain usable|`admin-legacy/playwright/run.emMtK4cd/admin-ledger-20261030T142059.395387660/playwright.log:27-51`: 30000ms test deadline, `ledger.spec.ts:25` radio check; screenshot and trace in `results/ledger-approved-ledger-rep-0bd0b--mobile-table-remain-usable/`|UNKNOWN / BLOCKED: trace shows navigation exhausted the deadline before inventory selection; no settings setup in this path. Cannot mark c without identical base-run failure. Do not change inventory, startup gates, waits or timeouts.|
|124100560368 / storefront-publish|TestBrowserStorefrontPublish → openCard|`storefront-publish/playwright/storefront-publish-1285242642/browser.log:8-23`: Advanced/card locator absent, `storefront-publish-gate.mjs:209`; failure PNG alongside log|a: enter via Edit settings; retain publication, reload, origin, CSRF, buyer and overflow coverage.|
|124100560297 / card-payments|TestBrowserCardPayments → CPU1 enabled store: badges, limits from the server, no credential input, entry link from Settings|`card-payments/playwright/run.le6greHt/card-payments/20261030T142754.893281559/playwright.log:35-49`: Advanced click timeout, `card-payments.spec.ts:130`|a: signed Edit settings entry; retain real click on payment link and all server-limit/safety assertions.|
|124100560297 / home-cod|TestBrowserHomeCod → settings: the merchant enables cash on delivery with a cap, surcharge and carrier|`home-cod/playwright/run.9vZr0rGL/home-cod/20261030T143123.207531798/playwright-settings.log:13-25`: waiting for 進階, `home-cod.spec.ts:90`|a: signed entry and explicit re-entry after refresh; retain cap, surcharge rejection, carrier and persistence assertions.|
|124100560297 / meta-health-ui|TestBrowserMetaHealthUI → named FB/IG capabilities keep review-required restricted (zh-TW, zh-CN, en)|`meta-health-ui/playwright/run.S7RjISE0/meta-health-ui/20261030T143534.219276447/playwright.log:19-39,59-79,99-119`: old Advanced/card union absent, `enterAdvanced:21`, caller `:62`|a: update the existing shared entry helper once; keep all capability and restriction oracles.|
|124100560297 / meta-health-ui|TestBrowserMetaHealthUI → desktop: warning is neutral, none and failed reads hide advice without blocking the workspace|Same log `:139-159`, caller `meta-health-ui.spec.ts:77`|a: same shared entry helper.|
|124100560297 / meta-health-ui|TestBrowserMetaHealthUI → explicit recheck schedules, then reports the real 429 without another schedule|Same log `:179-199`, caller `meta-health-ui.spec.ts:87`|a: same shared entry helper; do not change scheduling/rate-limit assertions.|
|124100560297 / meta-health-ui|TestBrowserMetaHealthUI → viewer sees owner advice and capability state but no managed actions|Same log `:219-239`, caller `meta-health-ui.spec.ts:103`|b plus a: overview made the full read-only connection card unreachable. Restore a localized View settings entry for non-managers; real-click/reload checks now cover all three locales and retain every no-action/disabled/next_due assertion.|
|124100560297 / meta-health-ui|TestBrowserMetaHealthUI → reconnect uses the existing OAuth picker and restores the same Page|Same log `:259-277`: `metaconnect-token` missing after reload at `meta-health-ui.spec.ts:131`; corresponding error-context shows overview|a: re-enter full editor after refresh, then keep token `data-state=active` assertion verbatim. OAuth picker/row assertions before refresh passed.|
|124100559375 / identity|TestBrowserSettingsWizardRealChain → REAL_PG A wizard has no PAYUNi, creates unseeded configuration and preserves safe uncertainty|`identity/playwright/settings-real-20261030T142144.191754062/playwright.log:9-34`: 宅配 expected unchecked, received checked, `settings-v2.spec.ts:693`|b (test setup regression): the new zero-market journey followed `deliverySwitches` in the same tab and inherited its saved draft. Isolate the new MOCK scenario in a fresh tab of the same authenticated context/store; all assertions and same-tab uncertain-command recovery remain unchanged. No storage clearing, altered responses or reduced fixtures.|
|124100559711 / checkout-offline|TestBrowserCheckoutOffline → settings: merchant-configured offline checkout|`checkout-offline/playwright/run.QNyhEvl0/checkout-offline/20261030T143220.647063128/playwright-settings.log:14-26`: 進階 click did not complete, `checkout-offline.spec.ts:96`|a: signed entry, including the English entry at :240 and every reload/navigation caller; preserve configured values and buyer flow.|
|124100559711 / meta-connect|TestBrowserMetaConnectGate → zh-TW desktop: before any connection the Studio claim source points at the Settings card, and the link works|`meta-connect/playwright/run.opIDDKmQ/meta-connect-gate/20261030T142759.256755617/playwright.log:23-35`: 進階 click timeout, `meta-connect-gate.spec.ts:127`|a: click signed Edit settings after the real Studio link; update existing openSettings helper, retain connection/refusal/state/no-secret oracles.|
|124100559711 / meta-connect|TestBrowserMetaConnect → not connected: the card offers Connect; a forged return state is refused and nothing is connected|`meta-connect/playwright/run.opIDDKmQ/meta-connect/20261030T143202.058241783/playwright.log:14-34`: Advanced/card union absent, `openSettings:55`; error-context lists Start quick setup + Edit settings|a: update existing openSettings helper; retain forged-state and no-connection assertions.|
|124123765553 / receipts|Every planned mode has one fresh PASS receipt with tests|`job-124123765553.log:26-35`: ten browser modes have status FAIL|Derived failure of rows above; no receipt/status edits.|
|124136164958 / required|required aggregation|`job-124136164958.log` last two lines: `gate=failure ... receipts=failure`, exit 1|Derived failure; no gate changes.|

The six failing bundles contain **11 failing Go browser test entry points**;
the meta-health entry contains seven failing Playwright cases. Tests not reached
in serial suites remain NOT_RUN, not independently proven passing or failing.

## Unresolved ledger failure — no product or budget change

Original `trace.zip` inspection (read-only zip/JSON extraction, no browser execution):

```text
test navigation pw:api@43 startTime=1384.516
test deadline/After Hooks startTime=31285.847
navigation pw:api@43 endTime=34218.113
navigation elapsed=32833.597ms > unchanged 30000ms test deadline
radio selection pw:api@52: visible/enabled/stable, click action done,
  scheduled navigations finished, then test timeout during context teardown
```

The artifact's `dev-next.log:13-15` additionally records:

```text
Compiling /[locale]/inventory ...
GET /zh-CN/inventory 200 in 18.2s (next.js: 15.7s, proxy.ts: 10ms, application-code: 2.5s)
Finished writing to filesystem cache in 20.6s
```

This path does not open settings. `git diff --quiet origin/r3/integration...HEAD`
for the ledger spec, its identity fixture, Go wrapper and catalog feature paths
exits 0. This establishes unchanged direct sources, **not** a reproduced base
failure or proof that PR effects are impossible.

Base run `41050344892` is success at `3ea359f796d36153bb4b8aa7259e4e0c27676b86`,
but its original jobs inventory (`base-run.json`) contains backend shards and
**no browser-admin-legacy/browser bundle job**; build and aggregate were skipped.
Thus “base is GREEN” does not supply the identical base browser failure line
required for class (c). Do not label this pre-existing, flaky or fixed from these
receipts alone.

The ledger failure remains UNKNOWN, not class (c). No ledger, inventory, startup,
fixture, timeout, retry or gate change is included. The settings fixes are independent
of this unexplained failure; the next PR CI must establish their actual result.
If this same test fails again, hand off its exact assertion/trace as BLOCKED rather
than trying a third approach. An identical failing base receipt is still absent.

## Implemented batch and static evidence

- Superseded full-editor entry names changed in the existing helpers/callers for
  domains, publication, ads attribution, card payments, COD, offline checkout,
  Meta connect/gate and health. After refresh/locale navigation, tests explicitly
  re-enter; OAuth-return helpers accept an already-visible card. All downstream
  behavior, permission, data, width, persistence and no-secret assertions remain.
- Read-only members now have `檢視設定 | 查看设置 | View settings` navigation.
  Quick setup/edit actions remain unavailable; the existing full editor's server
  authorization and disabled managed actions are unchanged. Viewer main-screen
  real clicks and refresh assertions cover zh-TW, zh-CN and en.
- Zero-market AD-04 runs in a fresh tab with the same signed session/store. It
  retains exact zero-market responses, zero writes before clicks, unchecked initial
  choices, all five independent/select-all checks, exact key/body replay after a
  lost policy acknowledgement, policy-before-service order and persisted count.
  Original tab/draft is preserved, and the temporary tab closes in finally.
- E1: TypeScript strict noEmit (all admin sources + both settings specs + all seven
  changed admin specs), exit 0. `fix1-tsc.log` is empty. Dependencies read from main
  checkout; all source aliases resolve to this worktree. Initial static invocation
  lacked react-dom and Next image type paths; the check-only config now resolves
  the existing declarations without changing application compiler options.
- E1: both changed storefront drivers `node --check`, exit 0; browser ownership
  generation and `--check`, exit 0; depmap regenerated (no diff); packet structure
  check exit 0; diff whitespace check exit 0. Packet's timestamp-only generated
  change was restored, keeping it outside this patch. UI static detector: `[]`.
- No runtime import added/renamed in SettingsV2; tests/ search found only UI legacy
  registry entries for that component, no compilation harness to update.

## Handoff, bounds and NOT_RUN

One FIX1 implementation push only; no local tests/servers. New-head GREEN and
independent review are PENDING. Historical Foundation `41062911937` /
`124086863404` passed only on the failing pre-fix head and is not acceptance.
CI run IDs are recorded in the PR comment/final relay, avoiding an evidence-only
commit. Integrator owns the completion notification; queue >3h or cancellation
requires integrator relay. No background local process remains.

Outside original narrow settings paths, listed for integrator review: the existing
admin specs for attribution/card-payments/checkout-offline/home-cod/meta-connect/
meta-connect-gate/meta-health-ui; the two storefront settings-entry drivers; shared
three-locale settings copy in packages/i18n/src/index.ts; generated ownership;
FIX1/DELIVERY/red evidence. These are settings navigation callers/copy, not changes
to storefront app/backend behavior. No migration/contract/OpenAPI/go.mod/lockfile,
baseline, dispatcher exemption, next-startup adapter assertion or timeout changes.

NOT_RUN: local tests/servers (owner instruction); new-head browser/runtime/PG CI
until receipt; independent/visual acceptance; provider SANDBOX/LIVE/production.
Humaux store/canvas/coord/code-memory tools unavailable; no such receipt claimed.
Original downloaded failure evidence retained untracked; no unrelated files cleaned.
Author assessment E0; static checks E1; historical failing CI assertions E3 only.
