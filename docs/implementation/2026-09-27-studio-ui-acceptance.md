# Studio B UI acceptance — 2026-09-27

Status: **SOURCE_CANDIDATE_UNMERGED; REPAIR_2_STATIC_CLEAR; STU04_RUNNING**.

User approved **B 场次列表＋双区工作台**, seed `6373bb3f`, in `fb760ff`.
The approved comp is `.impeccable/mocks/decision/studio-split.png` (1586×992);
the authoritative surface brief is
`apps/admin/.impeccable/surfaces/app-locale-studio.md`.
This decision does not change the separately approved Orders C composition.

## Scope and ownership

Reuse the existing workspace shell, locale routes, Studio BFF and Go/PG domain.
Do not introduce a second mutation backend or generic frontend state framework.
List/editor/status share one joined work surface. All displayed preparation and
attempt facts come from the frozen `contracts/studio-v1.md` DTO. Simulation is
labelled MOCK; platform output is unverified. No camera/chat/viewer-count facade.

|Role|Worktree / branch|Allowed writes|
|---|---|---|
|UI author|`/Volumes/data/live-commerce-buyer-http-20260925`; `commerce/studio-b-ui-20260927`|Studio route/component/model/client/copy, scoped CSS, routing-only WorkspaceFrame|
|Independent tests|`/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926`; `commerce/studio-ui-tests-20260927`|`tests/admin/studio-ui.spec.ts`, `tests/foundation/browser_studio_ui_test.go`|
|Integrator|Main checkout; task `e1e40a57-5a48-47bd-97be-de60f96018d2`|Reviewed merge, runner registration, contract metadata and this receipt|

UI author uses `gpt-6-sol/high` as recorded at assignment. Reused test/reviewer
agents have no exposed runtime model override; none is claimed. No recursive
delegation. Shared source stays frozen during actual acceptance runs.

## Candidate and same-P1 repair boundary

Initial candidate `158a5cc` passed author typecheck/build but failed independent
static review: conceal and browser history discarded unsaved input and the
original idempotency key after an uncertain write; shell navigation bypassed
dirty-draft confirmation. These are user-data/duplicate-write risks, not polish.

Repair 1 `e1506e5` retains same-session temporary-conceal state and guards shell
navigation. It passed typecheck/build, but native Back/Forward still called the
destructive clear path. Independent review kept the same P1 open. Repair 2 is
the final targeted attempt before a design escalation, not permission for an
unbounded patch loop. Fixed repair 2 `32dfd1e` uses one bounded, volatile per-tab
recovery slot, exact cookie/session revalidation and fresh reads before showing
it, no automatic writes, and explicit consent before document exit. Other scene
routes cannot silently overwrite that slot; logout/session-change invalidates
it. Independent fixed-source review found the original P1 closed in source and
no new confirmed P0/P1 in the bounded scope. Actual browser proof remains
required; static review alone does not close STU04.

Humaux source/review receipts: `21f33470-6f00-4622-b4f1-a6daf7ecdb22`,
`19ca2e73-7284-4fd5-951c-8156f19d1892`,
`44005821-c380-440a-b378-f90d6268ccfc`,
`16595991-144c-424f-a32c-63357043da26`,
`3b01e53e-2963-4ea2-b377-436ff309f84c`.

## Required independent acceptance

`bash scripts/dev/test-local.sh --browser-studio-ui` is registered by root
`27b8c57` / `2695b57` and fails early if its tests are missing. It must run the
real signed local OIDC → packaged Next → Go/PG chain and actual MOCK worker.
Test-only setup/fault endpoints are confined to the task-owned fixture.

- Real create/edit/reopen; two-tab stale-version conflict cannot overwrite.
- Dirty draft survives temporary conceal and actual Back/Forward, or leaves
  only after explicit discard consent. Locale and shell navigation agree.
- Drop ACK only after an actual committed write; retry uses the original key
  after route unmount/re-entry, and PG contains exactly one draft.
- A different signed login for the same principal/store cannot inherit old
  pending request or form. Expiry, read-only, CSRF and cross-store denial hold.
- Prepared/no-prepared and persisted Start/Stop states remain truthful.
- zh-CN / zh-TW / en at desktop 1586×992 and mobile 390×844; loaded first viewport,
  no clipping or persistent bearer/media-payload cache.
- Independent visual finish review against approved B, then bounded corrections
  and built-surface documentation. Build/typecheck alone is not visual acceptance.
- Root repeats the real browser gate and clean full PG/race/vet on fixed source.

Initial test-only series: `a0e2a72`, `f5f3da9`, `489665c`, `0e6242b`, `6244f0e`.
Complements `43e22f0`, `f7910e5`, `c39b3f2` add persistent-storage inspection,
native history/session swap, mobile save/reopen and locale overflow checks.
Static compilation/vet/typecheck/discovery passed. Fixed repair 2 is integrated
as `4e66c47` in the independent test checkout, where the first actual browser/PG
run is underway. Add actual logs, exit codes,
screenshots and hashes here only after execution. Do not replace failures.

## Remaining boundary

This slice does not qualify real browser publishing, Cloud Egress, public social
live output, comment-desk/manual-takeover features or a production deployment.
Customer broadcasts, orders and services have not been touched.
