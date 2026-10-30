# PR #294 IMPL3 relay1 — independent mock outbound identity

2026-10-30. Assigned inbox UI worker, branch `unit/inbox-v2-ui`, base `c5988f6fd7ce7e8984fc6a77a89137075fea5b9b`. Runtime Codex; exact model variant/effort unavailable. No delegation or new claim. Work remains in the assigned worktree. Both fast-forward pull and integration merge reported already up to date.

## Root cause, repair and gate

Read AGENTS.md, architecture §§0/1/2/26/27, `docs/delivery/units/meta-inbox-v2.md`, live-console-v1 A9/A12 and delivery-guard clauses, actual InboxThread and fixtures, and raw failed Foundation job. Authority is the integrator's IMPL3 ruling and current repository contracts, not older reports.

Foundation run **40978644799**, job **123812830250**, head **c5988f6fd7ce7e8984fc6a77a89137075fea5b9b**: Inbox Node suite 253 passed / 6 failed. All six failures assert that a different sent row cannot reconcile an observed queued/blocked operation. The unrelated mock row spread `projection("sent")`, inheriting the observed row's outbound_id. Add the unrelated row's own distinct UUID075, preserving observed/recovered UUID074. This is attempt 1 for this fixture-identity blocker. No assertions, text, time, state, exemptions or safety policy changed. UI implementation/DTO and seam commits remain untouched.

Filtered raw receipt: `red.log`; local syntax/ownership receipts: `static.log`. This failed implementation run is E3 failure evidence, not a tests-only RED. Earlier tests-only RED remains run40943430245/job123692452595 at head5d66cd1b3; see `../impl3-294/red.log`. Acceptance requires new-head PR Foundation/Gates, including inbox native-click tests in all three locales, plus independent integrator acceptance. GREEN PENDING; author's runtime claim E0. No local tests, servers or browsers. tsc NOT_RUN: compiler not installed. No install attempted.

## BOURDIEU

Q1 experience means recent task experience; Q2 existing recipient/reply/status sequence retained; Q3 independent staff sends remain independent; Q4 IDs establish identity; Q5 existing three-locale behavior retained; Q6 aesthetics UNKNOWN until owner. No visual or copy changes.

## PROJECT UNDERSTANDING

Architecture §§0/1/2 require evidence boundaries/server authority; §§26/27 require isolated writes/independent acceptance. A9/A12 correlate outbound_id; missing identity fails closed per IMPL3. Only synthetic fixture identity changes.

## MOBBIN → EAGLE

No new visual pattern or reference used. Needed Mobbin query: none. No access/import claimed.

## FIGMA OFFICIAL GUIDANCE

No Figma/design changes; auto layout/components/variables/prototyping unchanged. No fresh guide access or copy-lint claimed. Existing design-process delivery: `../impl3-294/DELIVERY.md`.

## Scope, rollback and handoff

This relay changes only `tests/admin/inbox-unknown.test.ts`, this evidence directory and mandatory generated `output/ci-select-backend-browser/browser-ownership.json`. Generated ownership is outside initial card paths, authorized by the preamble and disclosed in PR body/comment. No Go/contracts/migrations/locks/imports/routes changed; no dependency edge change requires depmap regeneration. Preserve all pre-existing dirty/untracked handoffs. Rollback reference: parent c5988f6f; no destructive restore performed. No local process/fixture/port created. Integrator pipe relays the recorded WAIT_RUN workflows; stop after the one push. No PR merge/automerge/status/deploy.

P2 for issue: existing timestamp display ordering, already noted in PR comment6489297640; unchanged.

NOT_RUN: local tests/build/tsc/server/browser/PG/containers/LIVE; new-head CI acceptance; native click/persistence/density acceptance; independent review. Final head and WAIT_RUN IDs belong in the PR comment; no evidence-only commit after GREEN.
