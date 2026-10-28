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
