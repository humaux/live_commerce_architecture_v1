# CA-ADMIN-STATE delivery

## Relay3 current correction (2026-10-29)

- Assigned FIX-RED #268, same worktree/branch; executor Codex (runtime model/effort not exposed). Base head: 5db5ec3d72f57c44be4c9cdcbf5998f0007331f7. No recursive delegation or separate queue claim.
- `git fetch origin && git merge origin/r3/integration`: exit 0, already up to date. The approved fix3 loading relocation, including customer/order page guards before Suspense, is already implemented. The pre-existing untracked patch remains untouched.
- Read current `gh pr checks 268` and the only failed job directly: Foundation40789708532/job123171584335. Gates40789708692 build was still pending. No older-head success is acceptance.
- Root cause: the new AD-10 contextual SSR test uses `<p[^>]*>` and accidentally starts at the state icon's SVG `<path>`. Its captured HTML includes the title and paragraph opener, so adding caller denial breaks a substring that was never just guidance. Product copy is present.
- Fix: match an exact paragraph opening tag, retaining all denial/guidance, locale, permission and dashboard-link assertions. No product code, thresholds, fixtures, ACL logic or gate exemptions changed. This is a harness correction, not a browser acceptance claim.
- Raw failure excerpt: relay3-foundation.log; run/job recorded in red.log. The existing current-head failure is the directed RED; no extra tests-only push. Targeted attempt 1 for this matcher blocker.
- Static `node --check tests/admin/code-alignment-admin-state.test.mjs`: exit 0. Regenerated browser ownership using the existing generator: exit 0, 53 modes. No import changed; existing compilation harnesses are untouched. No Go dependency change requires depmap regeneration.
- Evidence: E0 self-review; new exact-head Foundation/Gates GREEN PENDING. No local tests/servers, browser, PG, SANDBOX, LIVE or production. Independent review NOT_RUN. Integrator relays a fresh session on registered CI completion; no local background process.
- Scope: test is original card scope; generated ownership and output/admin-state evidence are pre-approved. No new outside-module edits.
- Rollback: revert the relay3 correction commit. No merge, auto-merge, force-push, commit status or deployment.

### BOURDIEU

Q1–Q6 unchanged/UNKNOWN; this relay corrects test parsing only. Q6 aesthetics remains owner-only.

### PROJECT UNDERSTANDING

Read AGENTS.md and 架构.md sections 0/1/2/26/27, the current test, real DS state renderer and both page guards. Source/runtime evidence stays distinct; TS/React baseline and server-owned scopes remain; author review is E0. Owner choice OC-Q7 (contracts/owner-choices-20261026-v1.md) keeps neutral denial without inferred roles. The current card authorizes restoring caller copy and loading placement, already implemented.

### MOBBIN → EAGLE

No new visual pattern; no reference import or query required for an SSR parser correction. Prior design provenance not re-audited.

### FIGMA OFFICIAL GUIDANCE

No Figma write or new design; guidance application, copy lint, density and mobile parity NOT_RUN in this relay. Existing UI click ledger remains pending CI; parser evidence cannot substitute for real clicks.

Evidence: E0 implementation self-review only. Runtime/browser/product acceptance is NOT_RUN locally and must come from PR #268 CI. No LIVE, SANDBOX or production claim.

## Task identity

- Role: ui_worker (single writer; no recursive delegation)
- Actual model: Kimi K3.1 Pro; reasoning effort not exposed by this runtime
- Base SHA: 997480141a7959ad6256a29e43bb1af31b97a404
- RED SHA: b02a185628b9b1c6bc07eb538df635c75f8a761f
- Worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/admin-state
- Branch: unit/ca-admin-state; PR #268 into r3/integration
- Allowed writes: the CA-ADMIN-STATE card JSON paths and output/admin-state/** only

## Implementation

- Added DS-backed AdminStateBlock for empty/error/loading and neutral AdminForbidden for the three approved locales.
- Added localized error/not-found/loading boundaries and an independent global-error html/body layout.
- Wired Customers and MerchantOrders display branches without changing their read clients, ACL checks, filters, pagination or retry functions.
- Added empty-state guidance in zh-TW, zh-CN and en. Empty result sets have no new action buttons.
- New CSS uses --ds-* tokens and no hex colours; no error internals or unsaved-draft safety claim is rendered.
- Test-only infrastructure correction after RED: mapped the real DS ./logic module in the SSR loader. No assertion was removed or relaxed.

## Verification ledger

| Check | Result | Evidence |
|---|---|---|
| tests-only RED push | CI Foundation failed; Gates succeeded | run 40645302665 / 40645302418; red.log |
| node --check tests/admin/code-alignment-admin-state.test.mjs | NOT_RUN: sandbox approval blocked execution | no PASS claimed |
| pnpm --filter @live-commerce/admin exec tsc --noEmit | NOT_RUN: sandbox approval blocked execution | no PASS claimed |
| git diff --check | exit 0 | local static whitespace check |
| GitHub implementation CI | WAIT_RUN to be supplied by integrator after local commit/push | not yet available at writing |

## UI click ledger

No local browser or server was started. Existing CI browser scenarios remain the authority; no screenshot or source-only check is presented as click acceptance.

| Page | Control | Operation | Expected | Actual | Status |
|---|---|---|---|---|---|
| Customers | search, submit, tag filter, refresh, previous/next | Real Playwright click/fill/selectOption in existing customers-billing suite | Existing behavior and empty-search guidance remain visible | NOT_RUN locally; await CI | NOT_RUN |
| Orders | filters, tabs, refresh, previous/next, retry | Real Playwright clicks in existing orders-ui suite | Existing controls remain; shared states render after real read results | NOT_RUN locally; await CI | NOT_RUN |
| Admin error boundary | Try again / dashboard link | Real click when a route-error CI scenario is available | Segment reset or locale dashboard navigation | No route-error browser scenario added by this card | NOT_RUN |

## Risks / scope notes

- tests/admin/orders-ui.spec.ts:906 still asserts the former order-specific forbidden sentence. The card freezes the new neutral Figma sentence and does not list that spec in write_paths, so it was not edited or weakened. Integrator must reconcile this stale assertion through an approved scope update if CI selects it.
- Failed Foundation job IDs/log excerpts were not readable because gh required sandbox approval; only relayed run conclusions are recorded.
- No dependency-map regeneration was forced or run before the implementation push.

## NOT_RUN / BLOCKED

- All local tests, browser tests, servers, builds and typecheck: NOT_RUN under the CI-only rule; attempted static node/tsc commands were blocked by sandbox approval.
- Independent review: NOT_RUN; author self-review is E0 only.
- Real ACL, production authorization, production deployment and live customer/buyer data: not touched and NOT_RUN.

## Directed AD-09 fix, attempt 1 (2026-10-28)

- Executor: Codex; base SHA 36f5b1e39147ed3aac80296eaabcd07028407ccd; same assigned worktree/branch and PR #268. No delegation.
- Authority: integrator's directed fix card and the failed Foundation receipt: run 40690045203, job 122822227182, head 36f5b1e39147ed3aac80296eaabcd07028407ccd. AD-09 fails its own document-prefix assertion; the suite reports 8 passed / 1 failed.
- Root cause: `global-error.tsx` already contains only html/body JSX. React DOM is pinned to 19.3.0 in apps/admin/package.json and pnpm-lock.yaml. Its server renderer creates an implicit head when rendering html, then closes it before body. Removing a head from the component cannot fix this because there is no head in that component.
- Primary source (read 2026-10-28): [React v19.3.0 ReactFizzConfigDOM.js](https://github.com/facebook/react/blob/v19.3.0/packages/react-dom-bindings/src/server/ReactFizzConfigDOM.js#L5329-L5343), `writePreambleStart`; `writePreambleEnd` at lines 5412-5428 closes the implicit head before emitting body.
- Fix: AD-09 now requires the exact prefix `<html lang="en"><head></head><body>`. No optional arbitrary head, attributes, contents or wildcard is permitted. Every other AD-09 assertion is unchanged; product components and imports are unchanged.
- Local verification: `node --check tests/admin/code-alignment-admin-state.test.mjs` exited 0; `git diff --check` exited 0. All local tests/servers remain NOT_RUN. Current-head GitHub verification is PENDING; this fix's self-review is E0 until CI supplies evidence.
- Humaux tools are unavailable in this runtime; no shared-memory store, canvas update or coord claim is claimed. This delivery and the PR handoff preserve the directed-fix evidence.

### Component harness audit (static only)

Search: `rg -n 'Customers\.tsx|MerchantOrders\.tsx|global-error|AdminStateBlock|AdminForbidden' tests`, plus component-name searches across tests and loader/harness files.

| Component | Harness / use | Import/global coverage |
|---|---|---|
| Customers | customer-tags-scope.test.mjs | Full-module VM loader explicitly resolves both ./AdminStateBlock and ./AdminForbidden; existing imports remain mapped. |
| Customers | import-integration.test.ts | Extracted Customers function receives both AdminStateBlock and AdminForbidden globals and its existing hook/copy/frame globals. |
| Customers | w6-integration.test.ts | Compiles only url and CustomerRow fragments; neither fragment references either new state component. |
| MerchantOrders | parcels-scope.test.ts | AST-extracted parcel callbacks only; does not load or render the full component or new state branches. |
| global-error | code-alignment-admin-state.test.mjs | Route loader maps CSS, next/navigation and AdminStateBlock; locale helpers resolve through requireApp. |
| AdminStateBlock / AdminForbidden | code-alignment-admin-state.test.mjs | Both full-module loaders map CSS and the real DS exports; Controls/States dependencies map Icon and logic. Locale imports are type-only. |

Other component-name hits are browser scenarios, fixtures/models, static source checks or legacy metadata, with no custom component import/global allowlist to update. No additional harness edits were indicated by this static audit. Runtime confirmation remains CI-only.

## Directed shared HTTP 404 investigation, fix 2b (2026-10-29)

- Executor: Codex; base SHA a1673cff8fa909866d01051d759a74bf3cf941f5; assigned worktree/branch and PR #268 unchanged. No delegation or local tests/servers.
- RED receipt: Gates run 40705999120, bundle:01 job 122933199700. Read the completed job's raw log directly; disabled /zh-TW/ui-gallery returned 200 instead of 404 at ui-gallery-runner.mjs:109. Synthetic merge receipt SHA: 0f71a089741768940fcca9da8f60a1fca614cb43.
- Static root-cause finding (E0 pending CI): gallery/page.tsx still calls notFound() before its first await. The new locale-wide loading.tsx inserts a Suspense fallback around that async page and every descendant. Flushing that fallback commits HTTP 200 before Next handles the rejected notFound promise. Rendering a different not-found display cannot change already-sent headers.
- Official authority: Next.js loading.js, Status Codes, read 2026-10-29: https://nextjs.org/docs/app/api-reference/file-conventions/loading . A real 404 requires checking existence before the response starts streaming.
- Original-card authority read directly: output/figma-ui-completion-20261025/CODE-ALIGNMENT-BACKLOG.md, CA-ADMIN-STATE lines 698–728, git blob fb520c93a9962d0abb67348ba39491862b47b960. The allowed route paths are error.tsx, not-found.tsx and loading.tsx; proxy.ts, the locale layout and individual page guards are outside the card.
- The previous session's proxy/pathname proposal is superseded as a recommendation: it only addresses selected paths and would duplicate route validation. The common regression is the new ancestor loading boundary; Customers and MerchantOrders already render the shared loading variant inside their validated page components.
- Gate conflict: tests/admin/code-alignment-admin-state.test.mjs:51–68 explicitly requires app/[locale]/loading.tsx to exist, and its localized-state test renders that exact module. Removing the ancestor boundary would restore the previous non-streaming guard placement, but cannot pass this frozen existence assertion. No assertion, route guard, fixture or exemption was changed. Moving every guard above the boundary would require out-of-card paths and is not claimed as implemented.
- Per-card STOP condition was reached independently: bundle:08's uploaded Playwright artifact shows a different failure in tests/admin/staff-team.spec.ts:175, outside this card's write_paths. The observed error is request.headerValue: Target page, context or browser has been closed, from the asynchronous request listener. It is not a recorded HTTP 404 assertion failure. Any causal relationship to the streaming change remains unproven; the raw failure must not be relabeled as the 404 family.
- No implementation, commit or push in this session. GREEN and WAIT_RUN are unavailable because implementation is blocked before push. Source diagnosis is E0; the existing CI failures are raw receipts for the old head only, not acceptance of a fix.
- Required verification: unchanged --browser-ui-gallery status assertions in all three locales, explicit unknown-route HTTP status and shared state UI checks, enabled-gallery rendering, request-header spoofing counterexample, and every affected proxy/layout harness. Runtime verification is GitHub CI only.
- Component harness inventory (static): code-alignment-admin-state.test.mjs loads the route/state modules; customer-tags-scope.test.mjs loads Customers; import-integration.test.ts injects Customers globals; w6-integration.test.ts and parcels-scope.test.ts extract limited fragments. No new import has been introduced, and no harness change is claimed.
- Humaux search/store/canvas/coord tools are unavailable. Shared-state updates are NOT_RUN; this local evidence is not represented as a Humaux store.

### Direct CI receipt readback

All four requested `gh run view 40705999120 --log-failed --job <id>` commands were attempted first. Each reported that the overall run was still in progress. Completed job logs were then read through `gh api repos/humaux/live_commerce_architecture_v1/actions/jobs/<id>/logs`; this is a read fallback, not a test rerun.

| Bundle | Completed job | Direct observation |
|---|---|---|
| 01 | 122933199700 | ui-gallery-runner.mjs:109: disabled /zh-TW/ui-gallery returned 200, expected 404. |
| 09 | 122933201564 | shell-browser.mjs:414: unknown store returned 200, expected 404. TestBrowserMerchantOrdersUIRealChain also failed; detailed artifact 12839256540 was NOT_READ because the sequential download stopped at the card-payments timeout. |
| 08 | 122933201753 | TestBrowserStaffTeam failed. Artifact 12839086450, staff-team-20261028T175124.723373186/playwright.log:14–23: request.headerValue rejected after page/context/browser closure at staff-team.spec.ts:175; 1 failed, 3 did not run, 2 passed. |
| 06 | 122933200181 | TestBrowserCardPayments failed. Artifact 12834974786 download timed out after 150 seconds (reader exit 1); its assertion-level cause remains UNKNOWN. No retry or 404-family attribution is claimed. |

No local test, build, server or browser was started. No dependency-map regeneration is needed because no source dependency changed. No delegation, migration, pin, baseline or gate-adapter edit occurred.

Handoff: unblock the loading-file contract without weakening HTTP 404 expectations, and assign/authorize the out-of-card staff-team listener failure before resuming implementation. Current head remains a1673cff8fa909866d01051d759a74bf3cf941f5. Only the two existing output/admin-state evidence files changed; they remain uncommitted. `git diff --check` exited 0. The artifact reader ended, left no downloaded files, and started no local runtime. Humaux store/canvas/coord and independent review remain NOT_RUN.

## Fix 3 ruling and concrete scope proposal (2026-10-29)

- Authority: the integrator's current fix 3 instruction permits relocating the PR's loading artifact and changing the AD-09 path contract; it supersedes fix 2b's frozen-assertion blocker and proxy proposal. Existing HTTP 404 gates and rendered-loading assertions remain mandatory.
- Repository authority read: AGENTS.md blob f0a6fd9bccdff7f2c14d2193437070a423e70d1a; 架构.md sections 0/1/2/26/27, blob 119fd82e3434c8b12650d0c6e305e805e52dd358; original CA-ADMIN-STATE card, blob fb520c93a9962d0abb67348ba39491862b47b960 lines 698–728.
- Structural finding: this checkout has no validating workspace layout. LocaleLayout only checks locale. OrdersPage validates query/store itself; CustomersPage awaits loadPage, then validates its customer query. Gallery flag validation is in UiGalleryPage itself. A loading.tsx beside any of these pages still wraps that page's own guards and cannot meet the ruling's required ordering.
- Official primary source read 2026-10-29: https://nextjs.org/docs/app/api-reference/file-conventions/loading (page updated 2026-09-23, live docs v16.4.3; repo pins Next 16.3.5). The same-segment loading file wraps page.tsx; response status is fixed once a fallback streams. This is source guidance, not current-head runtime evidence.
- Reviewable proposal: `output/admin-state/fix3-proposed.patch` (NOT_APPLIED). Move the identical localized loading presentation to components/AdminLoading.tsx, wrap only the returned Customers/MerchantOrders content in explicit Suspense after all server guards, change the AD-09 path, and add the assertion that locale-wide loading.tsx is absent. The role=status, 正在加载 and aria-hidden assertions stay verbatim. Gallery and other routes regain their prior absence of ancestor Suspense without duplicating any guard.
- Required scope decision: the two page.tsx files are outside the original write_paths. A single asynchronous clarification is pending; no answer or elapsed time is treated as approval. No production source or test file was changed. No commit or push was made; WAIT_RUN and GREEN do not exist for fix 3 yet.
- Harness audit for the proposal: no tests compile either page.tsx by path or function name; the only broad OrdersPage text match is the unrelated parseHistoricalOrdersPage symbol. The AD-09 existing loader already handles the relocated module's unchanged next/navigation, i18n and AdminStateBlock imports. No component harness edit is proposed.
- Staff-team readback: tests/admin/staff-team.spec.ts:175 starts an unawaited headerValue promise without rejection handling; the test closes the owner/invitee contexts in finally at lines 407–410. The spec has no diff against the PR merge base. This establishes a pre-existing race risk, not that the observed failure is definitely a pre-existing flake. No causal link to streaming is established; no spec edit is proposed.
- Existing RED independently re-read through the jobs API: run 40705999120 / job 122933199700 (gallery 200 vs 404) and job 122933201564 (unknown-store 200 vs 404). These are receipts for a1673cff8fa909866d01051d759a74bf3cf941f5 / synthetic merge 0f71a089741768940fcca9da8f60a1fca614cb43 only.
- Evidence: E0 proposal; `git diff --check` exit 0 for current evidence edits. All tests, servers, runtime validation, independent review and Humaux store/canvas/coord are NOT_RUN. No background command remains running. This is a blocked scope handoff, not completion.

## FIX-RED #268 approved fix3 implementation (2026-10-29)

This section supersedes the earlier fix2b STOP and fix3 scope-proposal status. The integrator explicitly approved the proposed loading relocation and both customers/orders page edits in the current instruction.

- Executor: Codex (GPT-6 per runtime; exact model/effort not exposed), single writer, no delegation. Worktree `/Volumes/data/live_commerce_architecture_v1/.worktrees/admin-state`, branch `unit/ca-admin-state`, PR #268 targeting `r3/integration`. Entry head `a1673cff8fa909866d01051d759a74bf3cf941f5`; fetched and merged integration `bf161bac96252c35337e07713c08a275d48e3e17` as `7ecf791c4141451dfc25db972e6aedc00a5e0199` before editing.
- Rollback: entry head plus the pre-existing uncommitted DELIVERY.md/red.log and fix3-proposed.patch, all preserved; no reset/clean or other-worktree write.
- Authority: current integrator instruction approving fix3; AGENTS.md blob `f0a6fd9bccdff7f2c14d2193437070a423e70d1a`; 架构.md blob `119fd82e3434c8b12650d0c6e305e805e52dd358` sections 0,1,2,26,27; original CA-ADMIN-STATE card read from blob `fb520c93a9962d0abb67348ba39491862b47b960` lines 698–728. Current source confirms the customer guards in `customers/page-data.ts` + `customers/page.tsx` and order guards in `orders/page.tsx`.
- Official source re-read 2026-10-29: [Next.js loading.js](https://nextjs.org/docs/app/api-reference/file-conventions/loading), current docs v16.4.3 (repo Next 16.3.5). Same-segment loading wraps page/descendants in Suspense; the first fallback fixes response headers. This guidance supports the diagnosis; runtime acceptance remains CI-only.
- Root-cause repair: remove the newly introduced locale-wide loading.tsx, preserve its localized/accessibility presentation as `components/AdminLoading.tsx`, and place explicit Suspense only around the customers/orders return content after every locale/query/store guard. Gallery and other route guards regain their previous non-streaming response behavior. No proxy/guard duplication or ACL change. Existing client loading states remain.
- AD-09 changes only the approved loading module path plus an assertion forbidding the regressing locale-wide boundary. Existing role=status, Simplified loading copy and aria-hidden assertions remain verbatim. No tests/ci/next-startup.test.mjs assertion, fixture, threshold, baseline, route-table dispatcher or exemption is edited.
- Harness inventory: `rg -l 'AdminLoading|LocaleLoading|loading\.tsx|customers/page\.tsx|orders/page\.tsx|CustomersPage|OrdersPage' tests` finds only AD-09 and the unrelated parseHistoricalOrdersPage test. AD-09's loader already resolves next/navigation, i18n and /AdminStateBlock for the relocated component; no page-compiling custom harness exists in this inventory. Prior Customers/MerchantOrders harness mappings remain intact.
- Generated ownership: after merging integration, ran `node scripts/dev/derive-browser-ownership.mjs`, exit 0, 53 modes. It updates the hashes of this PR's existing node/browser tests and test-node registration. Go imports/packages are unchanged, so dependency-map regeneration is not required.

### Current-head failures read directly

All five failed jobs from `gh pr checks 268` were read directly via jobs API after each requested `gh run view 40705999120 --log-failed --job <id>` reported the overall run unfinished. Head `a1673cff...`; run 40705999120 attempt 1; synthetic merge `0f71a089741768940fcca9da8f60a1fca614cb43`. Raw receipt pointers and the exact assertions are in red.log.

| Job | Failure seen in raw job log | Attribution before new CI |
|---|---|---|
| 122933199700 / bundle:01 | disabled gallery HTTP 200, expected 404 | locale loading regression; repaired as approved |
| 122933201582 / bundle:04 | StoreDesign browser exit 1, mobile zh-TW history empty | detailed browser assertion unavailable after bounded artifact reads; UNKNOWN |
| 122933200181 / bundle:06 | CardPayments browser exit 1 | detailed browser assertion unavailable after bounded artifact reads; UNKNOWN |
| 122933201753 / bundle:08 | StaffTeam browser exit 1; downstream chain/audit/canary counts incomplete | wrapper log alone does not prove HTTP-404 cause |
| 122933201564 / bundle:09 | parcel-merge browser exit 1; unknown-store HTTP 200, expected 404 | shell 404 repaired; parcel-merge detail unavailable after bounded artifact reads |

### Static checks and verification limits

- `node --check tests/admin/code-alignment-admin-state.test.mjs`: exit 0. `git diff --check`: exit 0. These are static checks, not runtime tests.
- `pnpm --filter @live-commerce/admin exec tsc --noEmit --incremental false`: exit 254, `Command "tsc" not found`. No dependencies installed; typecheck NOT_RUN.
- No local tests, builds, servers, PG, Playwright, browser or Figma operations. New-head GREEN is PENDING; author self-review E0 only. The integrator resumes this card after the new Gates/Foundation runs conclude; no merge, auto-merge or production action.
- Humaux search/grep/get/store/canvas/coord/code-index tools unavailable in this runtime; those operations are NOT_RUN. Repository evidence and PR handoff are an equivalent trail, not a claimed Humaux write.
- Source paths outside the original card (approved in this instruction): `apps/admin/components/AdminLoading.tsx`, `apps/admin/app/[locale]/customers/page.tsx`, `apps/admin/app/[locale]/orders/page.tsx`; generated `output/ci-select-backend-browser/browser-ownership.json`. Prior PR scope extensions remain in the PR body.
- This repair preserves existing UI copy, fields, tokens and interactions in zh-TW/zh-CN/en. No new visual design or Figma hand-in is claimed; the original visual/click acceptance is not upgraded by source inspection. Independent review, new-head runtime verification and production acceptance remain NOT_RUN/PENDING.

Artifact transport limits for this session: gh ZIP reads for 12834974786, 12839256540, 12839086450 and 12841600388 each timed out at 90 seconds; the direct-reader fallback hung and was terminated (own PID 49912, exit 143); final cache-busted gh ZIP reads for 12834974786, 12839256540, 12839086450 and the correct StoreDesign artifact 12841765213 each timed out at 60 seconds. No archive/fixture file was left on disk. Job logs were successfully read for every failed job; archive-level causes remain UNKNOWN where stated. No fix for those unconfirmed causes is claimed.

## RELAY 2: current denial-copy root cause (2026-10-29)

- Executor: Codex, ui_worker responsibility, exact runtime model/effort not exposed. No delegation. Entry PR head8a671a9a004ee3d4a96763ef80e7f5c7db03cec0; branch unit/ca-admin-state; PR268 into r3/integration. First fetched/merged integration2bc6d482606a2b0583af15789086fdad727b27ec; final local merge242144915dcb1a965b68ea0522941ad24291510a (Co-Authored-By trailer added before any push).
- Authority: this relay's human FIX-RED instruction; repository AGENTS.md; 架构.md sections0,1,2,26,27; CA-ADMIN-STATE card original blobfb520c93a9962d0abb67348ba39491862b47b960 lines698–728; actual components and unmodified parcel-merge assertions. Humaux development rule7bdb220b-0798-4b1d-aff5-16ee94cae75f read; rejected enumeration coverage9/9. Parent-dispatched child; no separate claim or recursive delegation.
- The approved fix3 already exists. The sole failing current-head job is Gates40774523865/job123156443677 bundle09. All other completed checks, including Foundation40774523875/job123120190295 and admin-shell, passed at readback. In-progress checks are not PASS.
- Raw job API fallback and artifact12860707314 prove parcel-merge.spec.ts:462 failed because MerchantOrders no longer showed the existing order denial. The held-poll timeout occurs while that copy assertion is stalled. The prior note treating the order-specific assertion as stale is superseded: it exposes a real omission in the shared display.
- Repair: optional localized description in AdminForbidden preserves the caller's existing denial, followed by the approved neutral guidance. Both Customers and MerchantOrders pass c.forbidden from existing en/zh-CN/zh-TW copy tables. No transport, ACL, private data, required-role mapping or status purge changes. No new imports; existing component VM allowlists/globals stay valid.
- Added six SSR cases (two callers × three locales), retaining all prior assertions. Existing browser suggestions/groups401/403/404 purge, held-completion, reload and real-click assertions remain verbatim. No new test claims to have run locally.
- Static node --check and git diff --check exited0. Browser ownership regenerated deterministically after integration merge (53modes); only this test's source hash changes. No Go dependency changes; depmap generation unnecessary. tsc NOT_RUN: no installed compiler; no dependency install attempted.
- RED evidence: red.log + relay2-playwright-parcel.log. New-head GREEN PENDING; self-review E0. Rollback: revert this repair commit; retain approved post-validation Suspense fix. Before-edit merge242144915dcb1a965b68ea0522941ad24291510a.
- New changes outside original write_paths: generated output/ci-select-backend-browser/browser-ownership.json and output/admin-state evidence only. No new module scope expansion. Prior approved AdminLoading/customers-page/orders-page edits and prior harness/browser scope extensions remain listed in PR body. Upstream integration changes are merge content, not authored here.
- Humaux fix stored as d7b6ff19-c178-47b6-baf2-e515d738cfcd; own child canvas codex-admin-state-fixred-sub-relay2 updated. No production action, merge/auto-merge of this PR, local tests/servers/PG, or background application process.

### Relay2 click ledger

| Page/control | CI operation | Expected | Actual before repair | New head |
|---|---|---|---|---|
| Orders / parcel retry | real retry click with one503 then403; held stale completions | Order-specific denial plus cleared private UI | Denial omitted; generic state only; artifact12860707314 | PENDING CI |
| Orders / remaining privacy states | existing401/403/404 real-click/read barriers unchanged | Cleared rows/detail/bulk/groups persist after stale completions | CI stops at first403 omission | PENDING CI |
| Permission / dashboard | existing locale dashboard link | Return to locale dashboard | href preserved by existing SSR assertions | PENDING CI |

### Relay2 three-language copy parity

Only existing localized copy is restored; no new translation keys.

| Context | zh-TW | zh-CN | en |
|---|---|---|---|
| Orders denial | 此帳號沒有讀取訂單的權限。 | 当前账号没有读取订单的权限。 | This account does not have permission to read orders. |
| Customers denial | 此帳號無權查看客戶。 | 此账号无权查看客户。 | This account does not have permission to view customers. |

### BOURDIEU

Q1–Q6: UNKNOWN; no new audience, expertise, hierarchy or aesthetic decisions in this corrective relay. Existing merchant-facing denial copy is restored. Q6 remains owner-only; no aesthetic acceptance claimed.

### PROJECT UNDERSTANDING

架构0 distinguishes source/model/runtime evidence;1 requires TS strict and server-owned business rules;2 limits this to merchant UI;26 requires assigned worktree and independent acceptance;27 requires failure-path evidence. CA-ADMIN-STATE display-only contract forbids ACL changes. Existing server refusal is displayed without resource identifiers or inferred roles.

### MOBBIN → EAGLE

No new layout/pattern/reference creation in this relay. Existing permission-state structure retained. No Mobbin query or Eagle import required for restoring the already-localized denial. Prior design-reference provenance not re-audited here.

### FIGMA OFFICIAL GUIDANCE

No Figma design or write in this relay; no new auto-layout/components/variables/prototype claim. output/ux-audit-ref/FIGMA-GUIDE.md was not available/read for this repair. Figma parity/copy-lint/density acceptance NOT_RUN, not implied by SSR or source inspection.

Evidence formatting: staged diff check initially caught two trailing spaces from the downloaded Playwright log; trimmed only those line endings. Assertion text and original artifact12860707314 remain unchanged. Final staged diff check must pass before commit. Own downloaded archive/full wrapper log removed after preserving selected failure evidence; pre-existing fix3-proposed.patch retained.
