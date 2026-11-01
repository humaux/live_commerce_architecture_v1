# PARITY-LIVE-SESSIONS — CONSOLE

Date: 2026-11-01. Card: delegated CONSOLE diagnosis/repair for PR #365 into `r3/integration`.
Role: Codex implementation lane; actual model/effort identifiers are not exposed by the runtime. No delegation.
Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/parity-live-sessions`.
Base/head before this card: `26826d389edcb2f96f97bf72ea91850bc1b2add4`.
Fetched integration: `fde11c1310fce20b576fc98023bd3003c66c205b`; required merge returned already up to date.

## Authority and outcome

Read the dispatched CONSOLE card, AGENTS.md, 架构.md §§0/1/2/26/27, `contracts/invariants.json`,
`contracts/tasks.json`, `contracts/live-console-v1.md` §§7.1–7.2, and the requested MAIN read-only
`output/integrator/diag/live-console-20261101.md`. Current source and original GitHub job logs take precedence over that diagnosis snapshot.
Applied the installed ponytail skill. Humaux/coord/canvas tools are unavailable in this session;
no claim, shared-state write, memory store, canvas update, or E4 acceptance is asserted.

**Root cause remains UNKNOWN.** Use the card's explicit tests-only diagnostic fallback. No product fix is claimed.
No application source changed; in particular `LiveConsole.tsx` and the shared command hook remain untouched.

## First question: what from #365 mounts or shares state on Console?

- `LiveWorkspace.tsx:46` mounts only `LiveConsole` for the selected scene. #365 changes its Sessions link at :43,
  not its mount key or conditional rendering. `SessionTasks` belongs to Studio, `claim-fields` to StudioClaims,
  and `SessionProducts` to StudioClaims' products task; none mounts on the Console route.
- `SessionTasks.tsx:37` and `SessionProducts.tsx:22` use separate `useLiveCommand` instances. They can share the
  durable `live-workspace-command:<store>:<scene>` sessionStorage fence across route visits, but no simultaneous
  hook instance is shared with Console. A retained journal is a possibility to measure, not a demonstrated cause.
- `StudioClaims.tsx:242–263` uses its own `perform`/pending ref. Prompt keyword/language changes at :537/:540
  are local select state, with no live-workspace command dispatch. No causal prompt-save leak is established.
- `session-tasks.module.css` selectors are rooted in CSS-module classes; Console does not use those classes.
  This excludes a direct matching C4 style rule, not every possible browser hit-testing/timing defect.
- `use-live-workspace.ts` has no #365 diff. Its run early-return remains boundary/active/pending/fenced at :125.
  No changed shared dedupe dispatch path was found. No speculative changes to journal safety or stock drafts.

## Corrected CI premise

The card says run 41273619268 attempts 1 and 2 both fail Console. Live GitHub inspection contradicts that:

| Attempt | Head / executed merge | Console job | Observed result |
| --- | --- | --- | --- |
| 1 | `26826d389` / `c990cdfc755af0590816a5e2caed948dbf5bea07` | `124751957935` | Recorded failure: en-390 Stock 20 rather than 23; no +3/v4 receipt (card and requested diagnosis). |
| 2 | same head / same merge | `124760549136` | Raw log: workspace PASS (343.20 s), labels PASS (25.51 s), receipt PASS, 38 tests / 37 specs. |

`gh run view 41273619268 --attempt 2 --job 124760549136 --log` filtered for test outcomes exits 0.
The run as a whole still concludes failure; attempt 2's failed jobs are receipts `124762130667` and required `124762201865`.
This is E3 evidence of that Console execution, not a whole-PR GREEN, causal repair, or proof of flakiness/determinism.

## Tests-only evidence added

`tests/admin/live-console.spec.ts` adds `observeStockDispatch` around the unchanged stock rebase/confirm/save
sequence for en/zh-TW/zh-CN at 1586/390. No assertion, fixture, action, timeout, threshold or exemption changed.
The `finally` writes `<LC_BROWSER_EVIDENCE>/<locale>-<width>-stock-dispatch.json` even if the stock assertion fails.

- Browser inventory POST pathname, status, numeric delta and expected_version; absent POST remains an empty network list.
- Real pointerdown/pointerup/click and native submit capture/after-React observations, defaultPrevented,
  numeric draft/baseline/server versions and quantities, Save disabled/hit-test booleans, confirm presence,
  host-node mount identities, and busy/fenced/pending/active/blocked/boundary-present booleans.
- Read-only committed React fiber inspection finds the source-bound command hook shape; missing shape/host gives
  null/false, never fabricated state. Native submit plus after-React defaultPrevented helps distinguish reaching
  the form handler from failing its guards; it does not claim a direct trace inside `command.run`.
- Artifact fields are an explicit primitive allowlist. No cookies, token/header values, command keys, journal
  contents, raw bodies, buyer strings, errors, DOM snapshots or React objects are serialized. Existing private
  evidence guards and trace/screenshot/video settings remain unchanged. No DOM/state mutation or synthetic action.

Risk: diagnostics can perturb timing and React-private state inspection may become unavailable on source/runtime drift.
The relay must use `hook_shape_found` and actual event/network evidence before selecting a product repair.
No new assertion was added solely to manufacture RED; the existing Stock 23 and +3/version 4 assertions remain the oracle.

## Static verification and handoff

- `node --check tests/admin/live-console.spec.ts`: exit 0, E1 syntax only.
- `python3 scripts/check_packet.py`: exit 0, PASS_PACKET_STRUCTURE_ONLY. Its generated timestamp receipt is restored;
  it is not product validation and is outside this card's write paths.
- `node scripts/dev/derive-browser-ownership.mjs` then `--check`: exit 0; generated ownership hash committed.
- `bash scripts/dev/depmap.sh`: exit 0, generated map unchanged.
- `git diff --check`: exit 0; no test/server execution locally.
- New diagnostic push's head/run/job IDs are recorded in the PR handoff comment, without an evidence-only push.
  Integrator owns the CI completion notification and fresh-session relay; no unattended local process remains.

Scope: the authorized Console spec, generated browser ownership, this report and existing card `red.log` only.
No additional application, contract, migration, lockfile, R2 pin, baseline or route-dispatcher changes.
Pre-existing dirty `RULING-FIX.md` and untracked `DIAG.md` are preserved and excluded from this commit.

NOT_RUN: local tests/servers (prohibited), tsc (no local executable/workspace dependencies), new CI conclusion until
the queued run finishes, product repair, independent review, provider SANDBOX/LIVE and production acceptance;
Humaux store/canvas/code-index (tools unavailable). Author self-assessment is E0; static evidence is E1 only.

STATUS: READY diagnostics-only commit; push/run handoff follows in the PR comment.
