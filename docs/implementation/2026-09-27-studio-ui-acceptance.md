# Studio B UI acceptance — 2026-09-27

Status: **SOURCE_CANDIDATE_UNMERGED; LATEST_CORE_CALENDAR_FAILED; NATIVE_CONCEAL_TWO_PASS; VISUAL_SCORED_FIXES_SHIP**.

Latest no-account native preflight (12:21 UTC): actual exit 0 using a separately
launched task-owned browser with public `noDefaults:true` and its existing
default context. Both real tab switching and window minimization produced
trusted hidden → visible events. This establishes a usable device, not a
product PASS. The two prior failed preflights remain below. The subsequent
actual product run passed both native-conceal cases but failed the calendar
value check; see the retained ninth-run result below.

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

## Sixth run: visual-batch regression and valid recaptures

Source `fc3cfef` integrates into the independent test tree as `3766bf4`;
test-only `52a32af` asserts locale-neutral English entry and rejects
`2030-02-30T00:00` with the exact error and zero POSTs. Valid UTC entry still
passes create/edit/reopen and mobile save. No production behavior or existing
security, history, session-swap, lost-ACK or native requirement was weakened.

At fixed test HEAD `52a32af`, `--browser-studio-ui` exited **1**, test 45.67s /
foundation 46.709s. Browser **3 PASS / 2 FAIL**; every actual OIDC/Next/Go/PG/
local MOCK worker counter and persistence check passed. The same two real
native-conceal prerequisites failed because `visibilityState` remained
`visible`; native lifecycle stays **NOT_RUN**, overall STU04 **NOT_PASS**.

Root independently verified the logs and opened all nine new locale captures:

- `output/studio-ui-sixth-20260927.log`, SHA256
  `68753f17de1b46fb6e8d316c8b216f849e4e1ce874906c0fb4119fc179da9baf`.
- `output/playwright/studio-ui-20260927T083110.972823000/playwright.log`, SHA256
  `7037b18a583f61cc955109c72fb7c5851091f364a25f021c3d4d1eaaa67305d2`.
- That directory contains each locale's 1586×992 desktop, 390×844 phone and
  full-height phone capture. These exact new paths are authoritative for the
  fix-verdict pass; fifth-run captures remain immutable historical evidence.

Paths above are relative to the independent test worktree in Scope. Receipt
`34af68a1-f1a5-414f-b0a7-60668bca69e4` records the test result and task-owned
PG/worker/browser cleanup. Same-reviewer visual verdict is pending. A visual
pass cannot waive native conceal or the root integration/full-regression gate.

## First fix verdict and final bounded correction

The same independent reviewer returned **fix**, receipt
`0c2f687a-2f95-4d01-9d45-196c4eac7bea`. Facebook and English-language display
were resolved. State-panel geometry was partial: the desktop button remained
about 160px below the panel instead of forming the comp's close group. The
English text-only date control introduced a usability regression by removing
the date-picker entry point. The second and final bounded correction targets
only these two findings; no broader redesign or new defect hunt is authorized.
Documentation of the built system follows the last correction, not this
intermediate layout. If findings remain after the second verdict, escalate
instead of another automatic polishing round.

## Final visual correction and seventh-run tester failure

Source `4ff7972` groups the observed-state inset and rehearsal action at the
bottom of the desktop status rail, with 18px between the card and action;
mobile remains in normal document order. English retains its explicit-format
text alternative and adds a labelled native `datetime-local` calendar entry
with focus and disabled treatment. No API, UTC conversion, custody, provider
or security behavior changed. Root read the entire three-file source diff.

The independent tree integrated it as `1a7a978`; test `bb47512` incorrectly
clicked the disabled calendar during a pending lost-ACK write. The seventh
run exited **1**, browser **1 PASS / 4 FAIL**: that click consumed the main
test's 240s deadline, so worker/read-only follow-on expectations could not pass.
This is a confirmed tester placement defect, not evidence of a product defect.
Its earlier nine captures were valid, opened by root and retained for the final
visual verdict. Historical evidence remains immutable:

- `output/studio-ui-seventh-20260927.log`, SHA256
  `f91063f8dcfce4050a23bf7250fb2780474891edaceac65a04c0552e1b60c3e1`.
- `output/playwright/studio-ui-20260927T085335.127525000/playwright.log`, SHA256
  `9bda885a7de4f7508d48192440783dcba9e040824cb6bf263ecb88b05ba1502e`.

## Eighth run and final bounded visual verdict

Test-only `c275c45` moves the trusted calendar interaction before fault
injection, on a saved, enabled DRAFT; UI source is unchanged. Typecheck and
five-case discovery passed. One actual eighth run exited **1**, test 47.20s /
foundation 48.590s, browser **2 PASS / 3 FAIL**. All non-calendar core checks
continued, and every actual Go/PG/MOCK readback passed, including exactly one
Start and Stop, terminal proof, original-key replay, fresh signed login
isolation, UTC schedule preservation and secret checks.

Two failures remain the native-conceal prerequisite: the actual document stayed
visible. The third is **TEST_OBSERVATION_LIMITATION**: an enabled native
calendar input received a trusted click and visible focus, but the test searched
for a page-DOM `dialog`. A browser-owned native popup is not guaranteed to be
exposed there. Neither that result nor the page screenshot proves that the
calendar failed to open. Native date selection followed by save/reopen remains
**NOT_RUN**, not a product-defect verdict. No ninth run was performed.

- `output/studio-ui-eighth-20260927.log`, SHA256
  `bec871743f5b0725e92936d5246eec833bd4cce7c733e0dedb610cefb6430b38`.
- `output/playwright/studio-ui-20260927T090538.982490000/playwright.log`, SHA256
  `576fb5849c5128ea6dc8d32406eff5169999b0b8d6443eb5c4b30c5ef4d9dcda`.
- That directory retains all nine locale captures and
  `en-native-calendar-open.png`, SHA256
  `7ab06ae6d5b17991818fb29cdd57f16f9f85d7c4b1ef2b83880a8de619cf0a42`.

Root independently read the test-only diff, complete failure signatures and
Go postflight checks, verified both run-log hashes, and viewed the native-click
capture. Agent receipts `d04bca71-1b59-4c75-87e1-eaa3bdd58234` and
`0a6dde17-2512-49e4-8506-507df594836d` record result, corrected observation
boundary and task-owned PG/browser cleanup. Source/test remain unmerged.

The previous reviewer handle was unavailable, so a fresh replacement independent
`gpt-6-sol/high` reviewer used the full prior findings and verdict, not a new
defect hunt. It opened all nine seventh-run captures of the unchanged final UI
and returned **ship for the scored visual fixes only**, receipt
`f3c813b9-9b31-47e2-a69a-803dd5a7bf09`. Panel/action grouping, Facebook mark,
English date display and calendar affordance are resolved, with no new visible
batch regression. Native popup behavior is explicitly outside this verdict.
Built-design documentation follows as an additive incumbent-world merge.

That documentation is now recorded in candidate-branch commit `03928d5`,
receipt `d9215ba9-ada0-4f1a-a57d-7a4f5ecf042d`, by a separate scoped
`gpt-6-luna/medium` documenter. Only `DESIGN.md` and `.impeccable/design.json`
changed: incumbent tokens and five existing examples remain, with one Studio
state/action example and the built desktop/mobile ordering added. Root read the
complete diff, verified clean candidate status, JSON schemaVersion 2 with six
examples and `git diff --check`. Native calendar behavior is expressly not
canonized as tested. These files remain with the unmerged UI candidate, not a
claim that main or production ships this surface.

STU04 overall is still **NOT_PASS**. Missing native lifecycle proof, a valid
native-date selection observation method and root fixed-tree full acceptance
remain release gates. Visual ship cannot waive any of them.

## Bounded native-device preflight evidence

Both probes used the same headed Playwright **1.63.0** / Chromium
**153.0.8010.12**, with no user account, network target, database or product
change. The first actual tab-switch probe exited **1**: trusted click/keyboard
input changed the datetime-local value, but no hidden visibility event occurred.
Evidence `/Volumes/data/output/playwright/studio-native-preflight-20260927.json`,
SHA-256 `0a7f073440d630a996ef5086849d3be17638cd3eca3214e4c06337c94c9ab15e`.

The second probe exited **1**. Browser window bounds independently confirmed
normal → minimized → normal, while document visibility remained visible and
the visibilitychange event list stayed empty. It restored the window and closed
the task-owned browser in finally. Evidence
`/Volumes/data/output/playwright/studio-native-minimize-20260927.json`, SHA-256
`039c941ed233e5f2021bb4dd6676a11bb98293815307f7231ad8f18aa9680011`;
script SHA-256 `22a6cdd3eb1c1269758775f75c618890929fa600a979f9025eeff24af451485f`.
No fake visibility values/events, product workaround or ninth full run was used.
Independent causal review `df8ad7c3-1319-4ea3-a372-8afd9a34d06e` then identified
the session mismatch. [Chromium's EmulationHandler](https://chromium.googlesource.com/chromium/src/+/main/content/browser/devtools/protocol/emulation_handler.cc)
owns the focus-emulation capture handle per protocol handler. Sending false on
a fresh handler does not release the original Playwright handler's capture.
Installed Playwright source agrees with its
[public noDefaults documentation](https://playwright.dev/docs/api/class-browsertype#browser-type-connect-over-cdp-option-no-defaults):
this option skips default focus emulation on the existing default context only;
new browser contexts are not covered.

Root's single reviewed causal preflight used the same browser executable and
version, an empty task-owned profile and loopback CDP, never a user's browser.
Real tab switching produced trusted hidden → visible, and actual window bounds
normal → minimized → normal produced a second trusted pair. Actual exit **0**;
`/Volumes/data/output/playwright/studio-native-nodefaults-20260927.json`, SHA-256
`63f1c9cff4778b1a55a5424424f4a75d5f25a1af45aa3ae6303ca81530f342ca`;
script SHA-256 `1b300ed6f59e46a01f3e696c3b95ff8156f16cff24d550401aea92f59dc8875a`.
The window was restored, but raw Chromium lingered after Browser.close/SIGTERM.
Root verified the exact task profile and PID 79040 before killing only that
process; subsequent PID/profile checks found no remaining process. The JSON's
null childExit is a pre-cleanup snapshot, not a successful shutdown assertion.
The unique profile remains retained with evidence. CDP attachment has lower
fidelity than Playwright's normal connection and raw launch arguments differ;
do not claim exact environment equivalence. The test-only fixture must include
bounded task-specific cleanup. Native device proof does not close product
native save/reopen or conceal gates; source remains unmerged.

## Ninth actual run — native conceal passed, calendar primary failure

Independent review `0ac93821-e946-4911-9f2c-ea33c505747b` approved test-only
`07e2fdc` for one actual run. Its admin source is byte-identical to `03928d5`.
The existing `--browser-studio-ui` command exited **1**, with browser **3 PASS /
2 FAIL**. Both native-conceal tests passed, including trusted hidden/visible
events, concealed form, returned dirty draft and retained uncertain request.

The primary failure is at `studio-ui.spec.ts:222`: after a trusted click,
screenshot and ArrowRight/Enter, the native picker value remained
`2030-01-01T00:00`. The core test stopped there. The read-only test's expected
absence of Start then failed because the earlier core chain never started the
rehearsal. Go postflight truthfully reported Start=0 / Stop=0 and incomplete
lost-ACK effects. This is not an accepted core or complete Studio chain. The
past core successes remain historical; they cannot replace this failed run.

- Raw log `output/studio-native-07e2fdc-20260927.log`, SHA-256
  `678e7555ff1b81dc1a9436b289922154c4537effddedaa6bc208e477343bf08d`.
- Evidence directory
  `output/playwright/studio-ui-20260927T123516.533039000/`; Playwright log SHA-256
  `fd3faa57320c0414192862390118ab0f507549257b44ad2eded671948d9baade`.
- Paths above are under the independent test worktree
  `/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926`.

Root read the targeted JSX/CSS, failure log and native-click screenshot.
The screenshot shows focus but cannot establish a native popup. Intervening
screenshot focus effects versus the compact transparent input/default click
action remain causal hypotheses, not established product diagnoses. No blind
second run or softened assertion is authorized by this result. The test worker
reported exact owned-profile/descendant and PG cleanup, retaining traces and
failed screenshots; the protected upgrade container was untouched.

## Calendar differential and scoped test correction

The independently adjudicated single 2×2 no-account comparison used the same
Playwright/Chromium versions: wide plain versus faithful compact transparent
input, each with or without a screenshot between click and ArrowRight/Enter.
Both no-screenshot cells changed January 1 to January 2 with a trusted input
event. Both screenshot cells retained January 1 and received the keys on the
underlying input. All four showPicker calls returned. This demonstrates a
same-host screenshot-interposition effect; it does not directly observe popup
open/close, establish a React defect, or accept the product flow.

Comparison JSON `/Volumes/data/output/playwright/studio-picker-matrix-20260927.json`,
SHA-256 `e509e68e8ae80627046f22ca8ff0e2181c6da35dfe4f8aecb4178aefb9d93b3f`;
script SHA-256 `a19a2a5789b7c1556389dfa477b3b61e0570f75be038435f8b4e6b82f15b144b`.
The isolated comparison completed with exit 0 and closed its browser; that is
not a Studio gate result. Code indexing accepts the probe file but extracts
zero entities from `.mjs`; no probe-symbol link is claimed.

Independent causal review `1ab47944-ac3d-4bc1-a6b2-7863e8ab91da` approved only
moving screenshot capture after native value selection/readback. Test author
`7b24381` makes that exact 3-add/3-delete diff, renames the image to
`en-native-calendar-selected.png`, and retains all hard checks and save/version/
reload/restore steps. Root independently read the complete diff, passed
`git show --check`, and verified admin source still byte-identical to `03928d5`.
One real gate rerun is authorized on this frozen test; its result is pending.

## Remaining boundary

This slice does not qualify real browser publishing, Cloud Egress, public social
live output, comment-desk/manual-takeover features or a production deployment.
Customer broadcasts, orders and services have not been touched.
