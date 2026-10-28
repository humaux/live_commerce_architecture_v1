# INBOX-V2-IMPL — implementation / CI handoff

## Authority and ownership

- Card: owner-signed simplified 訊息 implementation; supplied by the Claude integrator. One card / one PR into `r3/integration`; never merge or enable auto-merge.
- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/inbox-v2-ui`; branch `unit/inbox-v2-ui`.
- Base: `9a5a2f765c29fc0ab59fbb71e68ba468b31041e8`. Fetched and merged `origin/r3/integration` before adding a browser spec; already up to date.
- Role: dispatched UI implementation worker. Runtime model/effort not exposed: UNKNOWN. No delegation.
- UI authority: `/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/DELIVERY.md`, current 2026-10-29 polish + retained simplify baseline. Owner sign-off is supplied in this implementation card; the design handoff's older UNKNOWN sign-off is historical.
- Read: `AGENTS.md`; `架构.md` §§0/1/2/10/26/27; `contracts/live-console-v1.md` §§3.3/3.6/4.4/11; current A8/A9/A13/A14 browser DTOs and their consumers. The existing inbox is the implementation authority for privacy, cancellation, CAS and send eligibility.
- Relay update 2026-10-29: Humaux became available after initial discovery. Search/get and rejected representative enumeration ran (5/9, coverage 0.5556; not the complete set). Dispatch ownership remains with the integrator; no additional claim/delegation. Code-index/navigation tools unavailable; used rg symbols/imports and direct callers. Shared-store receipt and own sub-element updates are recorded separately.

## Exact module write paths (before code)

Located using `rg -l 'Inbox|inbox' apps/`, component imports and browser harness references.

|Path|Responsibility|
|---|---|
|`apps/admin/components/Inbox.tsx`|Signed list, fixed All/Unread/Assigned-in-store chip order, existing server data and responsive selection.|
|`apps/admin/components/InboxThread.tsx`|Signed thread, progressively disclosed facts, compact composer and distinct send/recovery states; existing guards unchanged.|
|`apps/admin/components/BuyerPanel.tsx`|Inbox-specific compact customer facts; preserve standalone console behavior.|
|`apps/admin/src/features/messages/Inbox.module.css`|Module layout, full-width rows, 44px targets, folded detail and responsive visibility.|
|`apps/admin/src/features/messages/copy.ts`|Bridge signed copy into the existing locale/error resolver; preserve other consumers' legacy keys.|
|`packages/i18n/src/inbox.ts`|Typed zh-TW / zh-CN / en signed inbox copy, dynamic placeholders instead of demo facts.|
|`packages/i18n/src/index.ts`|Export the module's locale entries.|
|`tests/admin/inbox-v2-ui.spec.ts`|New real-click acceptance for the signed main path, simplified states and three locales.|
|`tests/admin/inbox-copy.test.ts`|ADD signed-copy assertions to the existing registered Node suite; fail by assertion before implementation.|
|`tests/admin/inbox-ui.spec.ts`|Adapt existing controls to the signed UI after RED while retaining every behavior assertion.|
|`tests/admin/inbox-customer-ui.spec.ts`|Open the disclosure before existing customer-control interactions, preserving all CAS assertions.|
|`tests/admin/inbox-bundle-ui.spec.ts`|Only if needed: adapt disclosure interactions, preserve credential and clipboard assertions.|
|`tests/admin/inbox-review-host.test.ts`|Update the import adapter if changed component imports require it; inspect every importing harness.|
|`tests/admin/inbox-discovery.test.ts`|ADD discovery assertions for the new spec; retain the existing 13 legacy cases and all calibration assertions.|
|`output/inbox-v2-impl/**`|Plan, delivery, click ledger and SHA-bound RED/CI handoff evidence.|

The page `apps/admin/app/[locale]/messages/page.tsx` was inspected: its authentication/locale/server-scope contract needs no change. No API, DTO, contract, migration, pin, dependency or lockfile change.

Necessary test-registration/generated paths outside the module list (list in PR body): `playwright.config.ts` (ADD the new spec to the existing inbox suite); `output/ci-select-backend-browser/browser-ownership.json` (regenerate using the required script). `docs/engineering/dependency-map.md` may only be generated with `scripts/dev/depmap.sh` if CI requires a change; no Go dependency change is planned.

## Implementation sequence and gate

1. One tests-only RED commit covering the whole card; browser registration + generated ownership in the same push. Existing production UI remains unchanged. RED must reach new assertions, not collection/import/compile errors.
2. Create one PR to `r3/integration`, push once, record the Gates/Foundation run IDs, inspect CI for at most about 10 minutes. If queued/running, hand off WAIT_RUN and stop for the integrator's completion notification.
3. Only after causal RED: record exact run/job/SHA in `red.log`, implement the whole signed card, update all affected harnesses in the same implementation push, regenerate ownership after any test import change.
4. GREEN is the PR's own exact-head CI. No evidence-only push after GREEN; comment final receipts. Independent acceptance belongs to the integrator.

Locally: static checks only (`node --check`, `tsc`, allowed packet check where appropriate). No local tests, browser, dev server, PG or containers. Figma is read-only through Fleet Bridge; never use `use_figma`. Known sample names, timestamps, counts, takeover actors and totals must be replaced by current server facts; no fake saved draft persistence or invented identity/total. Existing privacy contract clears private drafts on hide/unmount.

## Coverage matrix

|Signed unit|CI acceptance|
|---|---|
|Fixed All → Unread → Assigned-in-store; Instagram; full-width list|Native clicks + visible row filtering and fixed chip position. Counts use server facts, never Figma sample numbers.|
|Desktop + 390px mobile; zh-TW/zh-CN/en main screen|Real locale select, native list/thread/back/disclosure controls, geometry read only.|
|Customer details default collapsed; manual link and unavailable order facts|Real disclosure click; visible facts, no invented customer match or spend.|
|Clear original draft, send once, queued → query → sent|Native fill/click; MOCK transport uses existing response shapes, no real send; outbound once, no repeated draft; reload observes persisted fixture state. Existing real BFF/PG send gate stays intact.|
|Rejected / blocked / unknown / still unconfirmed|Distinct visible result; edit only after known rejection; unknown queries without a second POST and remains locked after reload.|
|Window/connection/permission restriction|Native opening and visible reason, disabled sending; no synthetic capability inferred in product.|
|Privacy + read state + customer linking|Retain all existing real-chain tests and their assertions.|

## Current evidence

Causal copy RED: E3 GitHub Foundation 40772119429 / 123111778889, tests-only head 2b63813f, tested synthetic merge dfebcc0daa0d38fcd845ced00a11db6c79e449d8. Implementation written; author E0, local syntax checks E1 only. Implementation GREEN/browser/density/independent acceptance PENDING.

Additional direct-consumer test paths needed during implementation (outside the original pre-code list; listed in PR body): `tests/admin/inbox-unknown.test.ts` adapts signed warning copy and ADDS stricter queued/blocked and new-inbound counterexamples; `tests/admin/inbox-window.test.ts` updates two closed-window copy literals to the signed wording. All timeout, boundary, no-POST, CAS, permission and privacy assertions remain. No Go graph change; depmap regeneration unnecessary.

## CI registration repair — integration synchronization (attempt 1)

Source implementation commit `c7cb646d7f1b19b073fea7bef256c30277a28f30` was pushed once, but no workflow/check suite registered. GitHub reported CONFLICTING/DIRTY. `git ls-remote` proved r3/integration had advanced to `dcecd6220094234b4760315c676d9737ed713c40`, while the PR API still returned the original base. Fetch/merge in this assigned worktree found exactly one conflict: the four header-comment lines in `packages/i18n/src/index.ts`. Keep the integration header, stateRecoveryCopy export and inboxMessages export; update its dependency line to name both tables. No integration product source was manually edited. Regenerate browser ownership from the merged source and refresh source hashes. This is the first targeted CI-registration repair; no test/threshold/fixture/baseline change and no local runtime execution. The merge synchronization is necessary to allow this PR's own CI to start; it is not an evidence-only push.


## Relay 2 — formatter architecture repair (attempt 1 for this blocker)

Current failed head: `7a10e999ceec83bc1c69b5c04f32170d15715a61`; Foundation run `40777677585`, job `123131069803`, exit 1. Raw filtered receipt: `relay2-foundation-failure.log`. The existing G-UI3/G-UI5 gate rejects `InboxThread.tsx: format 1 > 0`: the signed compact timestamp introduced a component-local `Intl.DateTimeFormat`. This is an implementation defect, not another causal tests-only RED.

Fix: reuse existing `@live-commerce/format.instantToTaipei` and display its canonical HH:mm suffix; store-timezone conversion remains owned by the shared formatter. No shared-package source or API changes. The generic hook host re-exports the actual formatter for the new import; its existing time/money test seams remain. All three direct InboxThread harness importers were inspected; unknown/window consumers use that shared host. The native-click `open` helper now asserts the fixture's `06:32Z` is visibly `14:32` in every INV2 locale. Existing architecture assertions, exemptions, thresholds, retries and fixtures stay unchanged. Regenerated browser ownership after the test/harness edits; no spec/page/route addition or rename in this repair, so another integration merge is unnecessary.

Local E1 only: `node --check tests/admin/inbox-review-host.test.ts`, `node --check tests/admin/inbox-v2-ui.spec.ts`, `node scripts/dev/derive-browser-ownership.mjs`, and `git diff --check` all exit 0. Local tests, servers, browser, PG and containers NOT_RUN. PR-head GREEN and independent acceptance PENDING; author E0. Source hashes refreshed. No new outside-card path; the already-disclosed generated browser-ownership file changes. Depmap is Go-only and no Go edges changed.

Humaux tools are unavailable in this relay; store/canvas/code-index/link NOT_RUN, no shared-state write claimed. Integrator retains ownership and wake-up responsibility after the next WAIT_RUN handoff. No local background processes or fixtures were started.

## Relay 3 — inbox registration assertion repair (attempt 1 for this blocker)

Foundation run `40778689341`, job `123134516917`, head `6195fce85afac8b8c97f1e834169c2fca4b8d50d` failed at `scripts/dev/test-node.sh:88`: `inbox authority suite missing`. This is an implementation-registration defect, not the original causal RED. The preceding real Playwright discovery assertion passed and the inbox suite reported 185 tests / 185 pass / 0 fail. Raw filtered failure: `relay3-foundation-failure.log`.

The card appended `inbox-v2-ui.spec.ts` to the existing Playwright inbox array but missed its literal authority assertion. Append that same fourth spec to this inbox-specific expected array: every original filename and its order stays, all other assertions stay. No general CI policy or next-startup assertion changes. This direct inbox registration consumer is outside the initial write-path list; list `scripts/dev/test-node.sh` in the PR body for integrator review under the card's module-root-cause scope provision. Regenerate browser ownership because its inputs hash this script. No spec/page/route addition or rename in this repair; no further integration merge required.

Static-only verification: `bash -n scripts/dev/test-node.sh`, required ownership generation and `git diff --check` exit 0 (E1). No local test, discovery, browser, server, PG or container ran. Product GREEN and independent acceptance remain PENDING; author E0. Humaux tools are available in this relay; store and own sub-element canvas receipts follow under lock. Parent task ownership stays with the integrator; no new claim or delegation.
