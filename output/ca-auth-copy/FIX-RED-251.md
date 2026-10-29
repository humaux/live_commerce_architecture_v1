# FIX-RED #251 — recovery fault-injection lifecycle

Authority: dispatched FIX-RED card; AGENTS.md; 架构.md §§0/1/2/26/27;
contracts/merchant-password-auth-v1.md §7.3 and PA11; current PR source and official CI logs.
Role: dispatched implementation worker, Codex; runtime model/effort not exposed.
Worktree: `.worktrees/ca-auth-copy`; branch: `unit/ca-auth-copy`; base PR head:
`e0c9f10723f5b0108cd34c7544e990b5a334e054`.
First repository mutation merged `origin/r3/integration` at
`bf161bac96252c35337e07713c08a275d48e3e17` as instructed. No other worktree was changed.

## Current-head RED and static diagnosis

Read `gh pr checks 251` and ALL failed jobs with `gh run view --log-failed --job`:
Gates `40660830574`, WebKit `122779975258`, receipts `122873118983`, required `122909226199`.
The WebKit job reports only password-auth failing (39 seconds); receipts rejects that failed receipt,
and required propagates gate/receipt failure. Tested synthetic merge:
`253d9ef77e5c8901d6d4623898d853d7884c5ec0`. These are negative CI evidence (E3), not GREEN.
The original tests-only assertion-failing RED is `40612180005` / `122576061200`, retained in red.log.

The current fault injector aborts the first request of EVERY distinct chunk URL for the entire test.
It thus also aborts a chunk first discovered after the native reload, when the scenario requires a
recovered network. Conversely, duplicate requests before reload are allowed despite the outage.
This lifecycle defect is visible directly in the current source; whether it explains the complete
current WebKit failure must be established by the renewed CI. The job summary does not expose the
browser assertion. Artifact transport investigation is recorded in the handoff; no historical
output snapshot is treated as current evidence.

## Change and acceptance

Use an explicit network recovery phase. Before the recovery click, every matching chunk request
(including retries) is aborted. Restore the network immediately before the SAME real native link
click; continue every subsequent chunk, including newly discovered dependencies. Keep the route
installed throughout both navigations. Count continued requests only when their exact chunk URL
was actually aborted, preserving the existing re-request assertion's meaning.

All assertions, locale/engine coverage, timeouts, native click/load and URL checks remain unchanged.
No product UI or dependency import changed. No gate adapter, threshold, baseline or exemption changed.
The generated browser ownership artifact is regenerated because discovery-input hashes include the
edited spec. Its only changed entry is the spec hash; mode/input ownership is unchanged.

Acceptance: this PR's own complete CI at the pushed head, including the three-locale password-auth
recovery cases on Chromium and WebKit, Foundation and aggregate/receipt/required checks, followed
by independent review. Author assessment is E0; no functional completion or GREEN claim.

## Static checks and scope

- `node --check tests/admin/password-auth.spec.ts`: exit 0 (E1 syntax only).
- `git diff --check`: exit 0.
- `node scripts/dev/derive-browser-ownership.mjs`: deterministic generation only, no tests/server.
- Changed implementation: `tests/admin/password-auth.spec.ts`; evidence: `output/ca-auth-copy/**`.
- Outside original card paths: `output/ci-select-backend-browser/browser-ownership.json`, generated
  and pre-authorized by the FIX-RED preamble. Integration merge changes belong to upstream.
- No contracts, migration, OpenAPI, Go module, lockfile, other app/backend or Figma edits.
- NOT_RUN: local tests/servers/browser/PG/containers; renewed CI acceptance, independent review, LIVE.
- Existing uncommitted delivery/handoff records are preserved; no local test process was started.
- Humaux/coord/canvas/code-index tools were not exposed. No such writes or completion are claimed;
  this file and the PR comment supply integrator recording evidence.

One correction push is planned, with Gates/Foundation run IDs recorded in the PR comment and local
handoff afterward. Integrator owns CI completion notification and fresh-session resume; workflows
provide CI timeouts. No merge, auto-merge or evidence-only follow-up push. Rollback: revert the fix.
