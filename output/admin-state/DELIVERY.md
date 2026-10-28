# CA-ADMIN-STATE delivery

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
