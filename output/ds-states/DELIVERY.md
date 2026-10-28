# PR #274 — FIX-RED implementation handoff

claim_type: fix · project: live_commerce_architecture_v1 · module: ds-states · date: 2026-10-29

Authority: owner relay in this session; repository AGENTS.md; 架构.md §§0,1,2,26,27;
docs/design/ds/DS-2.md; docs/delivery/GATES.md gallery gate; original CI receipts in red.log.
Assigned branch/worktree: unit/ds-states-v2, /Volumes/data/live_commerce_architecture_v1/.worktrees/ds-states.
Incoming RED head: 855152b662b99d004881ef255ac1c79d42e077aa. Executor: Codex; runtime model/effort UNKNOWN. No delegation.
Merged integration bf161bac96252c35337e07713c08a275d48e3e17 via bcaf5543f1d66797578e97361574797e7619b5ed before implementation. Humaux became available in this session; targeted search, rejected enumeration, development-rules lookup and code-map read performed. The map had no useful focused DS symbols; local symbol/caller reads supplied the impact inspection. Rejected lookup covered 5/9 representative entries (coverage 0.5556), not the complete project history.

Status: implementation restored; own acceptance E0 / GREEN PENDING. The integrator relays a fresh session when the new WAIT_RUN workflows finish; run/job receipts go in the PR comment and local WAIT-RUN.md, without an evidence-only push.

## Root causes and corrections

1. Unconditional RED return made the later InlineError action guard unreachable; TypeScript could not narrow StateAction | undefined. Remove unreachable implementation from both RED stubs. Keep exported legacy renderers, types and assertions; after assertion-level RED restore the implementation with `git apply --unidiff-zero output/ds-states/implementation.patch` on this exact RED head.
2. Component-level fetch violates G-UI3/G-UI5. Use existing admin lib/customers-client readJSON. Preserve fixed-operation GET and response validation, adding the helper's no-store response header to the intercepted HTTP fixture. Update the only SSR harness compiling UiGalleryRecovery.tsx with an explicit no-network import binding; real browser clicks still exercise the actual helper.

Both preparation blockers above were corrected on the incoming RED head. This implementation restores StatePanel's variant, note/status slots and polite transient region; InlineError's alert/action; and GalleryForbidden's safe three-locale body. No assertions, imports, control flows or synthetic payloads were changed in this implementation batch.

## Verification and rollback

E3 RED: Foundation run 40752816753 / job 123043721507 at 855152b662b99d004881ef255ac1c79d42e077aa reached ERR_ASSERTION at ds-rework.test.mjs:55 (raw diagnostic leak), :142 (missing variant), :170 (missing InlineError alert). Original filtered excerpts are in red.log. This proves the negative control only.
E1: node --check on tests/ui/ds-rework.test.mjs, tests/admin/ds-states-clicks.mjs and tests/admin/ui-gallery-clicks.mjs returned 0. git diff --check returned 0. Browser ownership generator returned 0 (53 modes), with no new artifact difference after merging integration. No local tests or servers, dependency installation or builds. The skill context script was not run because the card permits only the named local static checks; existing source/contract inspection supplied context.
TypeScript check NOT_RUN: tsc is absent in this worktree. CI must validate types and real clicks.
Rollback reference: incoming RED SHA above; initial dirty output/ds-states-v2 files and untracked output/ux-states-ref preserved and excluded from commits. Only this assigned branch is pushed; no production application change. Humaux records use an acquired advisory lock.

## UI process and evidence

No new design, screen, route, string or control. Existing three-locale copy table, required-fact parity table, Q1–Q6, project rules, Mobbin→Eagle inherited receipts and Figma guidance limitations remain in output/ds-states-v2/DELIVERY.md. No new live design/reference verification claimed. Existing click ledger contract covers each StatePanel/InlineError action at all three locales and both widths, visible result, 44px targets, same-operation read, no write/replay, refresh persistence. Actual results PENDING CI.

### BOURDIEU
Q1 recover from a read failure or inaccessible page; Q2 recent experience with this task defines novice/expert; Q3 clear cause plus one next action; Q4 experienced users use that same action; Q5 no class/device capability assumptions; Q6 aesthetics UNKNOWN pending owner. Preserves the approved component layout.

### PROJECT UNDERSTANDING
架构.md §§0/1/2 require honest evidence labels, TypeScript strict, Go/PG business truth and no invented merchant facts; §§26/27 require isolated paths, independent acceptance and slice gates. DS-2 owns token/component composition; GATES.md owns three-locale, two-width gallery clicks. UNKNOWN re-reads the same operation; denial copy cannot disclose identifiers. No task DAG edge or business contract changed.

### MOBBIN → EAGLE
No new pattern. Inherited reference receipts: Copilot MVOVLMVSA15MW (one first action), Bevel MVOVLO8WKRBD6 (recovery within task), Toggl MVRSAPZL4DT5D (permission exit), as documented in the existing delivery. No new live reference observation claimed; no Mobbin query needed for restoring these existing renderers.

### FIGMA OFFICIAL GUIDANCE
No Figma write or new design hand-in. Preserves existing auto layout/action composition and variable styles for StatePanel 1429:34298 / InlineError 1429:34299. output/ux-audit-ref/FIGMA-GUIDE.md is absent in this worktree; no official-guide verification claimed. Design-copy lint/density probe NOT_RUN; restoration is checked by the PR's browser gate.

## Scope and handoff

This implementation commit changes packages/ui/src/ds/States.tsx, apps/admin/components/UiGalleryRecovery.tsx and output/ds-states/{DELIVERY.md,red.log,PR-BODY.md}. Gallery caller and evidence paths use explicit in-module/evidence scope approval. The ownership artifact was regenerated with no new diff. Integration-only changes are inherited by the required merge, not authored here. No tests, assertions, gates, baselines, exemptions, migration numbers or R2 pin changed. No Go dependency change; depmap regeneration unnecessary.

NOT_RUN/PENDING: GREEN and current-head real-click/browser acceptance; local tests/server/browser/build; tsc; independent review; production/LIVE/provider/PG/WebKit/Figma acceptance. Integrator relay explicitly supplies wake-up after WAIT_RUN completion. No app process or fixture started. Own PASS remains E0; full card completion is not claimed.
