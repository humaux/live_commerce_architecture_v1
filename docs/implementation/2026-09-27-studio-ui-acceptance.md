# Studio B UI acceptance — 2026-09-27

Status: **SOURCE_CANDIDATE_UNMERGED; STU04_CORE_PG_PASS; NATIVE_NOT_RUN; VISUAL_REVIEW_PENDING**.

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
run failed as recorded below. Add subsequent actual logs, exit codes,
screenshots and hashes here only after execution. Do not replace failures.

## First real browser run — retained failure

Independent fixed source/test `4e66c47` ran the actual gate and exited 1,
foundation 25.885s. Evidence in the test worktree:

- `output/studio-ui-first-20260927.log`, SHA256
  `6de4cd570628e0407f89a4d62ebe6904ba1bcc121251d755ee9e9ff809a444ae`.
- `output/playwright/studio-ui-20260927T072456.942396000/playwright.log`, SHA256
  `a3ca3c44af6f14402c3a4375d6108c1873363a05ec93253e26c5733e4b2dfd4f`.

The signed list/detail reads returned HTTP 200, exact frozen keys and correct
no-store headers. Actual Go/PG timestamps serialized RFC3339 `+08:00`; the new
Studio parser required terminal `Z`, so it rejected valid replies and rendered
unavailable. This is a product UI/backend time-contract mismatch. A distinct
targeted repair must accept valid offsets without weakening malformed-value or
shape validation. It must also fix the raw `scheduled_at.slice(0,16)` consumer
of the UTC-labelled input, otherwise a title edit could shift the saved instant.
Do not force the fixture/database timezone to hide the mismatch.

The read-only fixture separately lacked its paired CSRF cookie; the client
correctly refused its session boundary. Correct the task-owned fixture cookie,
not the product authorization. Native history/uncertain-write recovery has not
yet reached its runtime assertions despite the static review clearance.

## Date repair and native browser environment

Source `fd27c87` accepts valid RFC3339 offsets with finite/calendar checks,
uses a single UTC-minute display conversion and preserves the validated original
offset/seconds/microseconds for a title-only edit. Independent fixed review
`58db348a-537a-4dde-9c34-4509d1098608` found no confirmed P0/P1 in that change.

Test/source `7ebf141` reran STU04 and exited 1, foundation 17.197s.
Parser and read-only/expired cases passed; the main chain passed signed login,
prepared authority, real create/edit/reopen and two-tab version conflict. It
then failed the genuine visibility assertion: headless Chromium kept the first
tab visible after another tab was brought forward. No synthetic event was used.
Log `output/studio-ui-second-20260927.log` in the test worktree, SHA256
`9f871638c09d47a55fbf4ecaa13cebb958b243082fe41a53d896ecf3ca6be575`;
screenshots/trace in `output/playwright/studio-ui-20260927T073308.409334000/`.

Test-only `3e399b0` switched to headed Chromium without changing assertions or
deadlines. Its actual third run also exited 1 at the same visibility assertion:
the first document stayed visible after the other tab came forward. Log
`output/studio-ui-third-20260927.log`, SHA256
`7cb33f66e2ade96a46018f4003cfd4ade4af4033688633741c22ddb6ed8db3d0`;
evidence `output/playwright/studio-ui-20260927T073424.105278000/`.
This repeats the existing Orders MOU03 host limitation; no more environment
retries or synthetic-event substitution are authorized for this pass.

Integrator decision: keep native visibility as an explicit failing/NOT_RUN
acceptance case and run the remaining real chain independently. The Go gate
must still execute its PG/counter readbacks and then remain nonzero while the
native criterion is missing. This prevents an early environment failure from
masking product defects; it does not waive the native criterion or turn STU04
green. Actual history recovery, worker controls and visual finish remain pending
until their separate runtime evidence is captured.

## Fourth run: independent fixture defects and evidence validity

Fixed `ffafbc9` exited 1 (test 301.72s; foundation 303.120s), log
`output/studio-ui-fourth-20260927.log` in the test worktree, SHA256
`c10d9fe57cace1952cb429fc311a57abb364f0db2efe9b537c220047b0eba617`.
Real create/edit/reopen, version conflict, history recovery, mobile edits and
same-key lost-ACK retry reached their assertions before Start observation failed.
Counters were 147 calls, 0 wrong authority, 2 lost ACKs, 1 Start, 0 Stops; the
counter Fatal masked later PG readbacks, so no PG acceptance is claimed.

The TLS fake provider built its reply from `h.plan.RoomName`, which this test
left empty until post-run cleanup. The real worker correctly rejected the
mismatched room. The UI trace independently showed successful detail GETs at
five-second intervals, not a missing poll. A read-only assertion also expected
a Start button on an already READY scene, and the dirty native case reused its
disabled input. The uncertain native case reached the actual visibility check
and reproduced the known host limitation. These failures remain retained.

Root viewed all six first-viewport captures under
`output/playwright/studio-ui-20260927T074704.654385000/`. Both Chinese phone
captures caught a partially translated navigation rail over the content; they
are not valid finish-review evidence. Settled viewport and full mobile-flow
captures are required, not CSS that hides the symptom for the screenshot.

Test-only fixes `4ae0bff`, `eebe437`, `7e34fbb` bind the room before worker startup,
keep read-only assertions tied to actual controls, use an independent dirty
New-scene form, aggregate every PG/counter check, and settle the rail before
capture. A native-only report requires all non-native/readback checks to pass;
the aggregate gate remains nonzero. Root reviewed this fixed diff before run 5.

Source `aa2bc39` separately fixes two confirmed P2 text errors: distinguish a
non-DRAFT edit lock from missing management permission, and label `updated_at`
as status-update time instead of observation time in all three locales. Root
reviewed its two-file diff; no permission, polling or history logic changed.
Source/test memory: `325df2c9-bb5a-4781-b2a8-a56f571b0e1f`,
`6c09b8f8-b939-4d7d-a794-0909aecdc327`. Candidate remains unmerged.

## Fifth run: core and database pass, native conceal still missing

Fixed test `7e34fbb` (UI source `aa2bc39`, cherry `24302df`) exited 1,
test 43.24s / foundation 44.239s. Playwright reported 3 PASS / 2 FAIL;
the two failures were native conceal because actual `visibilityState` stayed
`visible`. No synthetic event was substituted and the aggregate remains red.

The main signed OIDC → production Next → Go/PG → local TLS MOCK worker chain
passed create/edit/reopen, competing version edits, native Back/Forward and
shell/locale guards, mobile saves in three locales, actual committed/lost ACK
and original-key retry after unmount, and persisted Start/Stop to TERMINAL.
A fresh signed login for the same principal/store invalidated the old pending
request in place without reload: retry sent zero new POSTs. Expiry, read-only,
CSRF and foreign/unlisted-store checks also passed. Every independent PG and
worker counter readback passed before the final native-NOT_RUN error.

Evidence in `/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926`:

- `output/studio-ui-fifth-20260927.log`, SHA256
  `2e3c562629c5d426e1b55d300ef1d043e3593f260a8a075c50d8926fa11eb888`.
- `output/playwright/studio-ui-20260927T080641.578556000/playwright.log`, SHA256
  `29e52bd98317404d4131a93de30f2215b8c4a9fedd27b0e58c5ccaed57a73189`.
- In that capture directory, all nine locale desktop/phone/phone-full images
  were viewed by root and show settled navigation and loaded content. These,
  not the fourth-run partial-rail captures, are the finish-review input.

Humaux acceptance receipt `7ba64840-bd28-448f-8195-20df966f35e4`; test-only
repair receipt `414e553d-bfc3-48ee-ad92-c2fc8777bf06`. Task-owned PG, worker and
Playwright processes were stopped. Independent visual review is pending;
neither this partial runtime pass nor a later visual verdict can waive native
conceal or root integration/full-regression requirements.

## Independent visual review — first bounded batch

Fresh `gpt-6-sol/high` reviewer returned **fix**, memory
`0f6cd083-17b4-4453-894e-e68f17d1833c`. It inspected all nine required captures
and the approved comp independently of the build conversation. Shell, split
proportions, actions, editor and mobile order matched or had grounded product
adaptations. Required corrections are: distinct truthful current-state panel
and low-anchored desktop action; vector mark for the actual Facebook destination;
English or locale-neutral date entry instead of OS-Chinese affordances.
No fabricated Instagram row or live-output proof may be added. A standalone
QUALITY BAR card is absent for this incumbent world, explicitly disclosed.

One visual-fix batch `fc3cfef` in the existing source worktree restores the
status panel, anchors the desktop action without changing mobile order, and
adds only the returned Facebook destination's vector mark. English uses a
locale-neutral `YYYY-MM-DDTHH:mm` text input with the existing strict UTC
validator; other locales retain their native control. Original unchanged
schedule preservation and all recovery/security logic are untouched. Author
typecheck/build/diff checks passed; root read the complete three-file diff.
Humaux `8ba7d708-9a1c-44d9-9692-30bf4ec56f05` records the source batch.
Independent browser recaptures, an English invalid-calendar/no-write check and
same-reviewer verdict remain pending. This does not waive native conceal.

## Remaining boundary

This slice does not qualify real browser publishing, Cloud Egress, public social
live output, comment-desk/manual-takeover features or a production deployment.
Customer broadcasts, orders and services have not been touched.
