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

## Post-push handoff (local evidence update; no evidence-only commit)

Diagnostic head: `3433eeea14387b05b56e78b89686bf9ea5d0f5b1`, pushed once to the existing #365 branch.
Spec SHA-256: `1ee3d7c70f9c6e70d7608dd14f4317b4f1aacb750c3633ca1bb70b6d6472c84c`.
Gates run `41278031103`: in progress, Console test job not yet allocated at readback.
Foundation run `41278031111`, job `124764095697`: in progress. Both run headSha values match this head.
New diagnostic RED/GREEN conclusions remain PENDING; the old failed assertion is unchanged.
WAIT_RUN: `41278031103`, `41278031111`. Wake/relay owner: integrator, on workflow completion.
The committed report points to the PR comment; this post-push evidence update remains local to avoid another CI fleet.

STATUS: DONE diagnostics pushed; root cause UNKNOWN, product fix and CI acceptance pending.

## Relay readback — 2026-11-01, diagnostic head 3433eeea1

Authority: current dispatched CONSOLE relay; GitHub current PR head and raw run/job/artifact,
not the old output snapshot. Humaux tools are available in this session; earlier unavailability
statements above describe the previous session. Read current development rules memory
7bdb220b-0798-4b1d-aff5-16ee94cae75f and code repo map. Rejected sample coverage 3/9.
This delegated lane does not claim a separate task or delegate again.

Read-only CI commands: `gh pr checks 365`; `gh run view 41278031103 --job 124764728535 --log-failed`
(filtered); `gh run download 41278031103 -n gate-browser-live-console`; artifact ID 13073236725.
All read commands exited 0 except `gh pr checks`, whose failure is expected for failing required checks.
No local tests or servers; no rerun, status publication, or PR merge/automerge.

| Current-head evidence | Result |
| --- | --- |
| Gates 41278031103 / Console 124764728535 | FAIL: workspace 26 passed / 1 failed; labels passed. |
| Exact failure | zh-CN-390, spec :455: first stock Save expects 17, receives 12. |
| Previously reported en-390 +3/v4 | Request observed, status 200; native submit reaches React; pending/busy become true after dispatch. |
| Other instrumented cases | en-1586, zh-TW-1586/390, zh-CN-1586 all send +3/v4, status 200. |
| Failed zh-CN-390 diagnostic | Not produced: first save :453-455 precedes observer installation :476. |
| Foundation 41278031111 / 124764095697 | SUCCESS for 3433eeea1. |
| receipts 124773758786 / required 124774147562 | Derived failures; whole PR is not GREEN. |

Kept only allowlisted stock event/network JSON, failure source context and summary under
`console-evidence-41278031103/`; full downloaded artifact was removed. en-390 stock diagnostics
SHA-256: 839870a361d14f60a00abfab30bd568ba853c8d88df73152cda37a4f83ff58e6.
The observer is evidence, not a new assertion: this failure is NOT a RED on a newly added assertion.
Original C4 causal RED remains 41052979614 / 124055728603; no new implementation GREEN exists.

Own-file impact was rechecked: SessionTasks/SessionProducts/claim-fields/StudioClaims do not mount
on Console; prompt select changes local state; CSS module classes do not match Console;
LiveWorkspace differs only in its Sessions anchor destination/header, not Console mounting;
shared hook is identical to integration. No own-file command-dedupe change swallows dispatch.
These findings do not prove the latest Save event's cause or rule out a base lifecycle race.

The remaining root cause is UNKNOWN. The next missing evidence is at first Save :453-455,
before existing diagnostics. Candidate investigation sites are LiveConsole.tsx :29 (refresh),
:129-147 (OfferRow draft/submit), and use-live-workspace.ts :22/:125-131 (read clearing/command guards).
No exact defective line is proved. LiveConsole is explicitly reserved to #355, so no speculative
product edit is made. The explicitly authorized diagnostic fallback is extended to first Save using
the existing observer in a try/finally before reload. All original assertions/actions/timeouts remain
verbatim and in order. This emits `<locale>-<width>-initial-stock-dispatch.json` even on the current failure.
No imports, helpers, product source, fixture, gate or thresholds change.

Required integration merge completed locally: integration 4b8c04c72, local merge 3d94f84bc.
One batched tests-only diagnostic push follows, with no evidence-only follow-up push.
`node --check tests/admin/live-console.spec.ts`, ownership generation then `--check`, dependency-map
generation, and `git diff --check` all exit 0. Only ownership source hash changes; dependency map unchanged.
Preexisting RULING-FIX.md, DIAG.md and other lane changes remain untouched.
Evidence level E3 for the recorded automated MOCK CI observations only; author judgment E0;
no completed repair, whole-PR GREEN or E4 is asserted. Shared memory/canvas updated under scoped lock.

NOT_RUN: local tests/servers (prohibited), implementation fix, new CI conclusion, independent review,
provider SANDBOX/LIVE, production acceptance. New push head/run IDs follow in PR comment.
Integrator owns CI completion notification and fresh-session relay; no local background work.
Scope: Console spec + generated ownership + CONSOLE.md/red.log; no additional source path.

STATUS: DONE diagnostics extended to first Save; root cause UNKNOWN, product repair/CI acceptance pending.
