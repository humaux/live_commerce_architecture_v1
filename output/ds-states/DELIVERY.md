# PR #274 continuation — corrected RED preparation

claim_type: fix · project: live_commerce_architecture_v1 · module: ds-states · date: 2026-10-29

Authority: owner relay in this session; repository AGENTS.md; 架构.md §§0,1,2,26,27;
docs/design/ds/DS-2.md; docs/delivery/GATES.md gallery gate; original CI receipts in red.log.
Assigned branch/worktree: unit/ds-states-v2, /Volumes/data/live_commerce_architecture_v1/.worktrees/ds-states.
Base: ce10fcee79e897f6c14798dc97703cc59a67a6b6. Executor: Codex; model effort UNKNOWN. No delegation.
Humaux search/store/canvas/coord and code-index tools unavailable. This is a file handoff, not shared-store completion.

## Root causes and corrections

1. Unconditional RED return made the later InlineError action guard unreachable; TypeScript could not narrow StateAction | undefined. Remove unreachable implementation from both RED stubs. Keep exported legacy renderers, types and assertions; after assertion-level RED restore the implementation with `git apply --unidiff-zero output/ds-states/implementation.patch` on this exact RED head.
2. Component-level fetch violates G-UI3/G-UI5. Use existing admin lib/customers-client readJSON. Preserve fixed-operation GET and response validation, adding the helper's no-store response header to the intercepted HTTP fixture. Update the only SSR harness compiling UiGalleryRecovery.tsx with an explicit no-network import binding; real browser clicks still exercise the actual helper.

## Verification and rollback

E1: node --check on tests/ui/ds-rework.test.mjs, tests/admin/ds-states-clicks.mjs and tests/admin/ui-gallery-clicks.mjs returned 0. git diff --check returned 0. Browser ownership generator returned 0 (53 modes). No local tests or servers, dependency installation or builds.
TypeScript check NOT_RUN: tsc is absent in this worktree. CI must validate types and reach the new assertions.
Rollback reference: base SHA above, with initial dirty output/ds-states-v2 files and untracked output/ux-states-ref preserved and excluded from commits.
No production or shared-state mutation; only this assigned branch is pushed.

## UI process and evidence

No new design, screen, route, string or control. Existing three-locale copy table, required-fact parity table, Q1–Q6, project rules, Mobbin→Eagle inherited receipts and Figma guidance limitations remain in output/ds-states-v2/DELIVERY.md. No new live design/reference verification claimed. Existing click ledger contract covers each StatePanel/InlineError action at all three locales and both widths, visible result, 44px targets, same-operation read, no write/replay, refresh persistence. Actual results PENDING CI.

## Scope and handoff

Changes: packages/ui/src/ds/States.tsx; tests/ui/ds-rework.test.mjs; approved additions apps/admin/components/UiGalleryRecovery.tsx, tests/admin/ds-states-clicks.mjs, generated output/ci-select-backend-browser/browser-ownership.json, output/ds-states/**. No other files staged. No assertions, gates, baselines, exemptions, migration numbers or R2 pin changed. No Go dependency change; depmap regeneration unnecessary.

NOT_RUN: assertion-level RED for corrected batch; GREEN; local tests/server/browser/build; tsc; independent review; production/LIVE/provider/PG/WebKit/Figma acceptance. Integrator relay explicitly supplies wake-up after WAIT_RUN completion. No process or fixture started.
