# RVFIX #303 — acknowledged SKU / failed follow-up read, implementation (2026-10-30)

Authority: current integrator RVFIX dispatch and the final REQUEST_CHANGES in
`output/integrator/tools/logs/cxf-rv-303.log`, reviewing head
3eb560907b3a6dc7efa230dbda024a79df42d99c. Read AGENTS.md; architecture
0/1/2/26/27; merchant-settings-wizard-v1 session/draft and quickstart amendments;
merchant-settings-http-v1; catalog-inventory-v1; current SettingsV2 load/save,
settings-v2 execute/draftOf and their real-click caller. Assigned UI implementer,
GPT-6-based Codex (exact runtime model/effort selector unavailable), no delegation.
Worktree settings-v2-ui / branch unit/settings-v2-ui only. Integration
17328372dea91220050e9583259312e2db2e5d97 merged first as
3e9713df86dd5eeca1d9e1a51a10385a223b0c9e. Main checkout read-only.

## P1 / reproduction / acceptance gate

An acknowledged SKU clears its journal before the next product GET. If that GET
fails, there is no pending SKU or inventory command. Reload discards the cached
unsent stock and replaces 10 with server balance 0. This differs from the already
covered lost SKU response. Current causal RED is confirmed, and this relay adds
the implementation (first targeted repair of this distinct P1).

Tests-only HEAD `69651530ee66337af1b6b83dce17cbd791bbbbd4`: Gates
**40927153608 / bundle:07 job 123637810804**, synthetic merge
`e673bf0aa234be9c50ba41dac12eebca412fa682`. Fresh `gh run download` artifact
**12922147689 / gate-browser-identity** proves **settings-v2.spec.ts:288**:
expected stock `10`, received `0`. The previous line-202 lost-response assertion
passed. Raw `ack-read-red-playwright.log`, SHA256
`b50ec8134a173a2c32e3205f490a6373adf58fd6b416c322f89bd95ed0f8aa34`.
Foundation **40927153538 / 123635591001** passed only that tests-only head;
receipts **123648300682** and required **123648788897** propagate the identity
failure. No runner install/network failure established, so no speculative rerun.

Persist an independent, store/session-scoped initial-stock intent **before** SKU
dispatch. Bind it to product id/code, currency, price, warehouse and quantity.
Reload validates it against the scoped cached draft and fresh server responses,
preserving only product fields while taking profile/delivery baselines from the
server. Invalid/missing required draft or mismatched intent fails closed without
overwriting evidence. A successful SKU still clears its own command journal;
only explicit Save submits stock through the existing keyed/CAS adjustment.
Successful stock handling retires the intent. No read-side write or retry added.

Added native-click regression inside the existing one-session/store harness:
the original exact SKU replay receives its real 200 PG acknowledgement, then
only the immediately following product GET gets one explicit MOCK 503. Assert
the successful receipt, absent command journal and zero inventory commands;
Reload, refresh and reopen all three setup steps; require product/price/stock
10 retained without any writes. Explicit Save then enters the unchanged inventory
loss/replay checks and the final refresh-persisted stock-10 checks. This covers
the acknowledged-to-inventory transition without a second fixture or weakening
the original lost-response witness. Implementation adds eight corrupt-intent
counterexamples (product, code, warehouse, currency, price and three quantities)
using fixture-only storage writes, real refresh/Reload, unavailable-state and
zero-command assertions; final acceptance also checks intent retirement. All
original assertions remain verbatim/in order; no thresholds, retries, timeouts,
fixtures or exemptions relaxed.

Gate: the PR's own --browser-identity isolated PG / signed MOCK IdP chain,
Foundation and required Gates, bound to the new head. Current causal RED is E3;
implementation GREEN and independent E4 NOT_RUN. Author judgment E0. E1 only:
node --check tests/admin/settings-v2.spec.ts and git diff --check exit 0;
mandatory browser-ownership regeneration runs last before commit. Local tsc is
NOT_RUN (compiler/dependencies absent); no local runtime verification. No source
imports/Go graph change; no depmap change required. P2 profile CAS remains
deferred in verified PR comment 6499957731 (P2 for issue).

|Page / control|Operation|Expected|Actual|
|---|---|---|---|
|Setup review / Save|real click; actual SKU 200 then one read 503|unavailable; no inventory command|RED CI reached the intended boundary|
|Unavailable / Reload; browser refresh; setup Next controls|real clicks/reload|stock 10; zero writes|RED: stock 0; implementation PENDING CI|
|Unavailable / Reload with eight corrupt intents|real clicks/reload; storage preparation only|fail closed; retain evidence; zero writes|PENDING CI|
|Setup review / explicit Save|real click|one initial adjustment with original quantity; unchanged exact recovery|PENDING CI|
|Overview / refresh / edit / review|real clicks/reload|persisted stock 10|PENDING CI|

## UI PROCESS — BOURDIEU

Q1 preserve the seller's entered quantity; Q2 recovery recognizes recent task
experience; Q3 read failure must not silently change the task; Q4 explicit Save
retains control over writes; Q5 these verdicts do not classify class/device;
Q6 aesthetics remains UNKNOWN pending owner. No visible UI changes in this repair.

## UI PROCESS — PROJECT UNDERSTANDING

Architecture 0/1/2: distinguish evidence and keep PG as inventory authority.
26/27: assigned worktree, bounded fixes and independent CI acceptance. Wizard
session/draft contract: store/session-scoped nonsecret recovery, no read-side
writes or automatic retries. Catalog contract: keyed initial adjustment, version
0 and server-owned balance. Existing three-language and desktop/mobile fact
parity tables below remain; this phase changes recovery logic and tests only.

## UI PROCESS — MOBBIN → EAGLE

No new design or references; existing module references/principles below apply.
No Mobbin/Eagle access/import claimed for this correctness regression.

## UI PROCESS — FIGMA OFFICIAL GUIDANCE

No Figma/layout/component/prototype changes. Prior guidance citations and copy
tables below retained; no new Figma lint/density result claimed.

Authored paths: apps/admin/components/SettingsV2.tsx,
tests/admin/settings-v2.spec.ts, this delivery, red.log, raw RED log and generated
output/ci-select-backend-browser/browser-ownership.json. No new
outside-card file. Preserve pre-existing untracked evidence. No local tests,
servers, builds, containers or background processes started. NOT_RUN: local
runtime verification (prohibited), local tsc (unavailable), implementation GREEN until CI runs,
independent E4, provider/SANDBOX/LIVE/production. Rollback: revert the forthcoming
implementation commit on the assigned branch. Integrator relays CI completion;
handoff records workflow run IDs, never plan-job IDs; no long CI watch.
Humaux recovered after one transient transport error: own subtask canvas updated
and current causal RED problem stored. Own subtask canvas updated. No separate
coord claim; parent retains task ownership.
Fresh fetch/merge reports Already up to date with integration `17328372d`.
Card command mentions #294; that is Inbox (`unit/inbox-v2-ui`), not this branch.
Read its four failed jobs: Foundation 40929100216 / 123642440286 (18 Inbox test
failures), Gates 40929100369 / bundle:06 123645010286 (6 Inbox browser failures),
receipts 123659542487 and required 123659727521. No Inbox edits or rerun.
This record supersedes historical progress descriptions below for this phase.

---

# Historical FIX #303 — pending SKU negative-case synchronization, attempt 1 (2026-10-30)

Authority: dispatched relay/card; AGENTS.md; architecture 0/1/2/26/27;
merchant-settings-wizard-v1.md Session, drafts and uncertainty / STORE-PROFILE-API;
SettingsV2.tsx load/render and the current PR's original GitHub job/artifact.
Assigned UI implementer; no delegation. Runtime identifies GPT-6-based Codex;
exact model/effort selector unavailable. Base e977991eb89f823cd56e5756e318cdce864faeee,
branch unit/settings-v2-ui, assigned settings-v2-ui worktree only. Fresh fetch and
merge of origin/r3/integration a050a17e9aecfe5b3322bfdffd14de297c3e73c2 reports
Already up to date. Authored paths: existing settings browser helper, this delivery,
red.log and mandatory generated browser ownership. No app imports/source changed.

## Current failure / root repair / acceptance gate

Gates 40915958297 / bundle:07 job 123598450349 FAILED for e977991e;
synthetic merge 6e3b24e25360afde48014d06ddd8f5b036441af2. Fresh artifact
12916383922 gate-browser-identity shows a NEW failure at settings-v2.spec.ts:244:
Reload locator timeout after restoring the original cache. Snapshot shows overview.
It reached this line after the stock-10 assertions and negative-case loop; the
old expected-10/received-0 failure is not the current failure. This is an
implementation-phase harness regression, never relabeled as tests-only RED.
Raw extracted relay6-settings-playwright.log SHA256:
fa94fedbd0094c26506c48647a2b5663d18c47de5044bf03fd8493dac28c318f.

load() leaves the unavailable h1 mounted during async reads, while loading hides
its controls. The heading assertion could pass before the last negative Reload
finished; restoring the valid cache then let that in-flight read reach overview,
removing Reload. Add an enabled-Reload assertion after each negative Reload to
wait for the controls to return before checking or replacing that fixture. All
original assertions remain verbatim/in order. No removed assertions, changed
fixture values, timeout increase, retries, weakened gates or production changes.
This is attempt 1 for the synchronization blocker; pending-SKU product repair
remains e977991e. P2 CAS-conflict recovery remains deferred.

Gate: the PR's own --browser-identity chain must complete the unchanged original
SKU exact-key/body replay, initial inventory command/replay and refresh-persisted
stock 10, plus every invalid-cache fail-closed assertion after a completed read.
Historical causal RED E3 remains tests-only 5d85294c1a7eda43ec70bbf11f0c9186c7f034c7,
Gates 40908912713/job 123573702372, stock expected 10 / received 0 at line 202.
Foundation 40915957832/job 123596111488 SUCCESS applies to e977991e only.
Current push GREEN/E3 and independent E4 PENDING. Author judgment E0.

|Page / control|Operation|Expected|Actual evidence|
|---|---|---|---|
|Pending SKU / Reload, refresh, setup navigation|real clicks|stock 10 preserved, no writes|Reached past assertions in e977991e CI|
|Invalid cache / Reload|real click, await enabled Reload|completed read; unavailable; original cache/journal; no writes|New synchronization PENDING CI|
|Restored cache / Reload / setup / Save|real clicks|exact SKU replay then unchanged inventory recovery; persisted 10|Prior CI timed out at Reload; PENDING CI|

Local verification: node --check tests/admin/settings-v2.spec.ts exit 0;
git diff --check exit 0 (E1 syntax/whitespace only). Mandatory static browser
ownership generator runs last. No source import or Go graph change needs depmap.
NOT_RUN: local tests/servers (prohibited), new-head CI GREEN, independent E4,
provider/SANDBOX/LIVE/production. No current completion or acceptance claimed.
Humaux/coord/canvas tools unavailable; no store/canvas write claimed. Preserve all
pre-existing untracked evidence. No local servers/background tests were started.

## UI PROCESS — BOURDIEU

Existing Q1–Q6 verdicts below remain: experience means recent task experience,
never class/device; Q6 aesthetics UNKNOWN to owner. This test-only change adds
no merchant decision, visible string or layout.

## UI PROCESS — PROJECT UNDERSTANDING

Architecture 0/1/2 separates evidence, stock authority and product boundary;
26/27 requires assigned worktree, bounded repair, CI and independent acceptance.
Wizard session/draft contract requires original scoped uncertainty identity;
STORE-PROFILE-API keeps server scope and exact versioned replay. Current
SettingsV2 load/render establishes the read-completion boundary used here.

## UI PROCESS — MOBBIN → EAGLE

No design/reference change; prior module references and structural principles
below remain. No Mobbin/Eagle access or new import claimed for this test repair.

## UI PROCESS — FIGMA OFFICIAL GUIDANCE

No canvas/component/layout/prototype writes; existing official-guidance citations,
fact parity and three-column locale copy below remain unchanged. Figma lint and
density remeasurement are NOT_RUN because no visual artifact changed.

---

# FIX #303 — pending SKU recovery, implementation attempt 1 (2026-10-30)

Authority: the dispatched FIX #303 card; AGENTS.md; architecture sections
0/1/2/26/27; contracts/merchant-settings-wizard-v1.md (Session, drafts and
uncertainty / STORE-PROFILE-API); current SettingsV2/settings-v2 source.
Fresh integration a050a17e9aecfe5b3322bfdffd14de297c3e73c2 merged first as
84ccc9939. Assigned worktree / unit/settings-v2-ui only; no child delegation.
Role: dispatched UI implementer; actual model/effort identifier not exposed by
this runtime. Authored scope: SettingsV2.tsx recovery, existing browser helper,
delivery/red evidence and generated browser ownership. No new imports: the
SettingsV2.tsx harness search found none; existing harness imports stay unchanged.

## Root repair / acceptance gate

load(true) used to discard the cached draft before the initial inventory command
existed, replacing stock 10 with the new SKU's read balance 0. Pending SKU recovery
now requires the exact store/session cache, validates its shape/profile version,
product identity/status, command method/product/code/price, any existing SKU and
nonblank nonnegative safe-integer quantity. Reload preserves only those product
fields while refreshing other settings. Missing or mismatched recovery data stops
visibly in the existing unavailable state before cache writeback. Price and stock
remain locked during an uncertain SKU command, including before SKU commit.
Read recovery sends no mutations; explicit Save retains the original key/body.
P2 profile CAS conflict recovery stays deferred (PR comment 6499957731).

Causal RED E3: tests-only 5d85294c1a7eda43ec70bbf11f0c9186c7f034c7;
Gates 40908912713 / bundle:07 job 123573702372; tested synthetic merge
55be18bc3ead3d165b7a1bd33fa4ff80753c61fc. Fresh artifact 12913317531,
settings-v2.spec.ts:202 expected stock 10, received 0 after Reload + refresh.
Retained original fix303-sku-red-playwright.log SHA256
17e5f22abade53e9dd36a6dd28e8b459165e34a8f772b6832c43ef360d9d1a51.
Foundation 40908912520 / 123571192419 succeeds only for that tests-only head.
Implementation GREEN/E3 and independent E4 are PENDING; author judgment E0.

|Page / control|Operation|Expected|Current evidence|
|---|---|---|---|
|Quick setup / Save|click; SKU commits, reply lost|unavailable; no inventory write|Reached in RED|
|Unavailable / Reload, refresh, setup steps|real clicks|read only; stock 10 retained|RED: actual 0; repair PENDING CI|
|Unavailable / Reload|real clicks with storage fault fixtures|missing/version/product/price/quantity mismatch blocks; cache + journal unchanged|Supplemental CI assertions PENDING|
|Restored original draft / Reload / setup|real clicks|stock 10; no automatic replay|PENDING CI|
|Review / Save / inventory recovery / refresh|unchanged real clicks|exact SKU/inventory replay; persisted 10|NOT_REACHED at RED; PENDING CI|

All original assertions remain verbatim and in order. Supplemental negative
cases use page.evaluate only for explicitly annotated nonsecret storage fixture
preparation/restoration or readback, never DOM/UI simulation or API mutations.
Local node --check of the browser helper and git diff --check exit 0 (E1 syntax).
Targeted tsc exits 1: local Next/CSS/server-only declarations missing and unchanged
WorkspaceFrame.tsx:72 nullability; no errors name SettingsV2.tsx or the modified
browser helper. Full CI typecheck remains required. No dependency installation,
local tests or servers. Browser ownership is regenerated last; no Go graph/import
change requiring depmap. No protected gate/contract/migration/dependency changes.

## UI PROCESS — BOURDIEU

Q1–Q5 retain the signed task-experience verdicts below; recovery does not classify
merchants by device/class or add decisions. Q6 aesthetics stays UNKNOWN to owner.

## UI PROCESS — PROJECT UNDERSTANDING

Architecture 0/1/2 distinguish evidence/PG truth/product scope; 26/27 require
isolated ownership and independent acceptance. The wizard contract binds drafts
to store/session and uncertain retries to original bytes/key; those are the screen
rules repaired here. The current FIX card requires visible fail-closed recovery.

## UI PROCESS — MOBBIN → EAGLE

No new visual pattern/reference used; preserve the existing signed module and
its reference record below. No new Mobbin import request or Eagle write.

## UI PROCESS — FIGMA OFFICIAL GUIDANCE

No Figma/design change; existing frame/component/auto-layout guidance record and
desktop/mobile fact parity / three-language copy tables below remain applicable.
The existing main-screen browser checks still cover zh-TW, zh-CN and en. No new
visible strings, configuration, frame or layout; no fresh Figma lint claim.

Humaux/coord/canvas/code-index tools unavailable: no shared store/update or task
completion claimed. Raw CI failure retained; only this session's duplicate
download removed; pre-existing untracked integrator evidence preserved. No local
background process or fixture server remains. Rollback: revert the implementation
commit on this assigned branch. No integration push/merge/auto-merge/deployment.
Integrator owns completion relay; post-push handoff records Gates/Foundation
WAIT_RUN, never plan jobs. No evidence-only push after GREEN.
NOT_RUN: local tests/servers (prohibited), current implementation CI acceptance,
independent E4, provider/SANDBOX/LIVE/production. No acceptance claimed.

---

# Historical RVFIX #303 — SKU-response-loss RED phase (2026-10-30)

Authority: current dispatched RVFIX instruction and the final P1 finding in
`output/integrator/tools/logs/cxf-rv-303.log` (main checkout, read only), checked
against `SettingsV2.tsx` and `settings-v2.ts` at d6982f50450006656d564bed16c218bb587c9858.
Read AGENTS.md, architecture sections 0/1/2/26/27, existing task write paths and
merchant-settings-wizard-v1.md. The repository code/contract remains the source
of command behavior. Review reports are defect leads, not runtime acceptance.
Fetched and merged origin/r3/integration a250dec75961364de166cdd50190f21d07ddccd0
first, in merge 8583df4d2. Only the assigned worktree/branch is written.

Root path: Reload calls load(true), replaces the cached setup stock with the
new SKU's available 0, and only restores inventory/adjustments journals. A pending
skus command therefore loses the quantity of its not-yet-executed stock command.
The exact SKU command remains journaled; its eventual replay can succeed while
stock initialization is skipped. This is static analysis (E0), not a local run.

One tests-only push adds a preceding SKU fault to the existing single-session
quick-setup journey: forward its real UI POST to PG, assert a successful matching
receipt, then abort the browser response. All original assertions remain, in order.
Reload and refresh must perform no writes and preserve stock 10. Explicit Save
must replay the identical SKU key/body exactly once; the existing inventory-abort,
explicit replay, saved-state and persisted-10 assertions then run unchanged.
No implementation change before an own-assertion CI RED witness is read.

|Page / control|Operation|Expected|Actual|
|---|---|---|---|
|Quick setup / Save|real click; commit SKU then lose reply|uncertain state; no stock write|PENDING CI|
|Unavailable / Reload|real click, then refresh|reads only; pending stock 10 survives|PENDING CI|
|Quick setup / Next controls / Review|real clicks|product, price and stock 10 retained|PENDING CI|
|Review / Save|real click|same SKU key/body replay; inventory attempted|PENDING CI|
|Existing inventory recovery / Save / refresh|existing real clicks|one committed initial quantity; persisted 10|PENDING CI|

No visible UI/copy/locale/design changes: existing BOURDIEU, PROJECT UNDERSTANDING,
MOBBIN → EAGLE, FIGMA OFFICIAL GUIDANCE sections, parity/copy tables and three-locale
main-screen coverage remain below. No new imports/harness bindings or test timeout,
fixture, threshold, exemption, baseline, dispatcher or gate-adapter changes.
Browser ownership is regenerated last using the assigned static generator.
Authored paths in this phase are the existing settings-v2 spec, this delivery,
red.log and generated ownership; none outside the card. Integration merge inputs
are upstream changes, not new work on other modules.

RED/GREEN for the new SKU-loss regression: PENDING. Prior inventory RED and prior
head GREEN do not accept it. P2 for issue: existing CAS recovery remains deferred
in PR comment 6499957731; no P2 implementation. Author judgment E0; syntax only
E1 after static checks. NOT_RUN: all local tests/servers (prohibited), new-head
CI acceptance, implementation, independent review, providers/SANDBOX/LIVE/production.
No background process or local fixture created. Integrator owns CI-completion
relay; handoff records actual Gates/Foundation workflow ids after push, never a
plan job as WAIT_RUN. Humaux/coord/canvas tools unavailable; no store/canvas update
or shared-memory completion claimed. Preserve all pre-existing untracked files.
Rollback point: revert this phase's tests-only commit on unit/settings-v2-ui.

---

# Historical RVFIX #303 — P1 implementation attempt 1 (2026-10-29)

Authority: dispatched RVFIX card, REVIEW-303.md (hash below), current source at
bc38a7bfa67eb73cd0b99cd36bd7a5a4f7de4c6e, and merchant-settings-wizard-v1.md
OC-05/STORE-PROFILE-API. Fresh origin/r3/integration d0e7331525fec046fbe9361906fa66367ae583c6
was fetched and merged first: already up to date. Assigned branch/worktree only.

## Root repairs and acceptance

- Inventory: Reload previously replaced pending delta 10 with current balance 0.
  Load now restores quantity from the durable command after checking SKU,
  warehouse, nonnegative integer, initial version and reason. Other snapshot
  fields still come from the server. The incumbent explicit Save builds the same
  original body, and execute enforces byte/key identity; no automatic replay or
  unchanged-balance reconciliation is introduced. Stock stays locked against edits.
- Completion: OC-05 guarantees append-only per-store completion independent of
  command key. Reconcile the strict empty POST against a valid completed GET
  receipt, including its server timestamp. The hidden setup prompt no longer
  strands the journal after a committed Skip/finish response is lost.
- P2 remains deferred as directed: existing PR comment 6499957731 (P2 for issue).
  No stale-CAS recovery change included.

Causal RED E3: tests-only head bc38a7bf; Gates 40895321677 / bundle:07 job
123525211139; synthetic merge 3097e3bc1599e5d807150c32eae7dae95a35f27f;
artifact gate-browser-identity 12908705432. settings-v2.spec.ts:185 expected
stock "10", received "0" after the real clicked product/SKU creation and MOCK
inventory transport abort before commit. Original extracted playwright.log
SHA256 5b8258250defaa67058aea82b211be0cc7d1f19638f4a9d383003c1dedb4ca17.
This is the new assertion's real defect, not a fixture/compile error. Serial
execution stopped there; marker assertion NOT_REACHED, not separately RED.
Both regressions remain in the single batched tests-only commit unchanged.
Foundation 40895321668 / 123522717118 passed only the tests-only head.
GREEN/E3 and independent E4 acceptance for this repair are PENDING.

|Page / control|Operation|Expected|Observed at RED / repair|
|---|---|---|---|
|Quick setup / Save|click stock 10|product/SKU accepted; inventory failure visible|Reached, RED CI|
|Unavailable / Reload, setup navigation|click|no write; stock remains 10|RED failed: stock 0; repair PENDING CI|
|Review / Save|click|original key/body once; stock 10 after refresh|NOT_REACHED at RED; PENDING CI|
|Quick setup / Skip|click, lose committed reply|reload reads completed marker|NOT_REACHED at RED; PENDING CI|
|Profile / Review / Save|edit phone, click, refresh|save persists without storage clearing|NOT_REACHED at RED; PENDING CI|

Static commands: node --check apps/admin/lib/settings-v2.ts and
node --check tests/admin/settings-v2.spec.ts exit 0 (E1 syntax only).
Targeted tsc --noEmit --project output/settings-v2-impl/tsconfig.static.json
exit 1: missing next/navigation, next/link, CSS/server-only type declarations,
and unchanged WorkspaceFrame.tsx:72 nullability. No errors name the changed
SettingsV2/settings-v2 files; no local typecheck PASS or gate bypass claimed.
No dependency install or local tests/servers. Browser ownership is regenerated
last before push. No imports, route/spec registrations or Go graph changes;
no component harness/depmap update needed. Gate-adapter and protected baselines
are unchanged. No authored path outside this card's write paths in this repair.

UI PROCESS sections below apply: BOURDIEU task experience, PROJECT UNDERSTANDING
(architecture 0/1/2/26/27 and marker/replay contracts), MOBBIN → EAGLE and FIGMA
OFFICIAL GUIDANCE. This recovery fix reuses existing states/controls with no
new layout, copy or configuration. All three locales and existing click tests
are retained. No fresh design-reference, Figma or copy-lint claim.

Humaux/coord/canvas unavailable: no shared store/update/completion claimed.
Original failure evidence retained; own large temporary artifact transports
removed after extracting the original log. Prior untracked relay files preserved.
Rollback: revert this implementation commit on unit/settings-v2-ui.
No merge into integration, auto-merge, status posting or production action.
Integrator relays new Gates/Foundation completion; no local background process.
NOT_RUN: local tests/servers (prohibited), implementation CI GREEN, independent
review of repair, provider/SANDBOX/LIVE/production. Author judgment E0 only.

---

# RVFIX #303 — independent review recovery regressions, tests-only RED (2026-10-29)

Current authority: dispatched RVFIX #303 and
`/Volumes/data/live_commerce_architecture_v1/.worktrees/rv-303/REVIEW-303.md`,
SHA-256 `8ceea5bd32bc10b1ca428aebd19b63cbc288184d43d048d0ec74c4740399d50b`.
Review head and starting SHA: `be463ea8ba7468eb8467b8874d7f2673cd804a0b`.
Freshly fetched and merged `origin/r3/integration` at
`d0e7331525fec046fbe9361906fa66367ae583c6`: already up to date.
Assigned worktree/branch only; role: dispatched UI implementer, Codex;
runtime model/effort not exposed. No recursive delegation.

## RED plan and root causes

- P1 inventory: `load(true)` overwrites the intended stock with a zero server
  balance while the original adjustment journal survives. Reconstructing the
  adjustment from that replacement fails the exact-body guard. Extend the real
  product/SKU creation journey with a MOCK inventory transport abort before
  commit, real Reload and wizard navigation, retained stock 10, explicit Save
  and byte/key-identical replay exactly once. Existing product receipt and
  refresh/review stock assertions remain. No local fixture mutation/storage clear.
- P1 completion marker: a real Skip POST commits in PG, then a MOCK lost response
  leaves its journal. The completed marker hides setup without reconciling that
  command, so ordinary profile writes conflict forever. Forward the clicked UI
  command with `route.fetch`, assert its original 200/completed receipt, drop only
  the response, reload, and save/refresh a profile edit using visible controls.
- P2 profile CAS conflict: report `P2 for issue` in the PR as directed; no scope
  expansion to implement that finding.

One batched tests-only push covers both P1 findings. Source implementation stays
unchanged until CI supplies a real failing assertion; serial acceptance can stop
at the first regression. CI-only gate: the PR's own Gates/Foundation, including
`--browser-identity` settings-real suite. E0 author analysis only; CI RED and
current-head E3 GREEN are PENDING. Historical green at the starting SHA does not
accept these new recovery cases. Run/job receipts will be written to `red.log`
when available. Integrator workflow-completion relay is the wake-up mechanism;
no CI polling beyond ten minutes and no local tests/servers.

## UI PROCESS — BOURDIEU

Q1: retain the seller's intended quantity after a failed operation; comprehension
UNKNOWN pending observation. Q2: recent task experts can explicitly replay from
existing setup/review controls. Q3: review retains quantity and contact facts.
Q4: no new seller choice or configuration. Q5: mobile and desktop share the flow.
Q6: aesthetics separate, UNKNOWN until owner. Task experience defines novice/expert.

## UI PROCESS — PROJECT UNDERSTANDING

Read architecture §§0/1/2/26/27: evidence labels, TS strict, PG truth, isolated
worktree ownership and independent acceptance. Read merchant-settings-wizard-v1
AD-03/OC-05: completion is an append-only per-store marker and GET can establish
completion independently of the lost POST response. STORE-PROFILE-API: uncertain
commands retain their original key/body and no client tenant authority. Read
SettingsV2, settings-v2 model, their direct browser harness and catalog parsers.
Humaux search/rejected/coord/canvas/code-index tools are unavailable in this
runtime; no shared-memory store, task claim or canvas update is asserted.

## UI PROCESS — MOBBIN → EAGLE

This tests-only recovery card introduces no visual pattern. Existing design
reference record below is historical, not fresh inspection. No import or query
is needed for the existing Reload/setup/review controls.

## UI PROCESS — FIGMA OFFICIAL GUIDANCE

No design/frame/copy changes. Existing design handoff below references the guide;
`output/ux-audit-ref/FIGMA-GUIDE.md` is absent here, so no fresh direct guide-read
claim. No Figma write or copy-lint acceptance is claimed.

## Click ledger — authored, execution pending CI

|Page / control|Operation|Expected visible / persistent result|Actual|
|---|---|---|---|
|Quick setup / Save|click after stock 10|product/SKU accepted; inventory transport fault shows unavailable|PENDING|
|Unavailable / Reload|click, then start/next/next|stock remains 10; no automatic inventory write|PENDING|
|Review / Save|click|original key/body replay once; saved; refresh/review shows stock 10|PENDING|
|Quick setup / Skip|click, commit real command, lose reply|unavailable; reload hides completed setup prompt|PENDING|
|Editor / Review / Save|fill phone, click twice|saved and phone remains after refresh, without storage clearing|PENDING|

Scope: `tests/admin/settings-v2.spec.ts`, task evidence/docs and generated browser
ownership only. No source imports changed; no component harness/dependency-map
change required. Existing three-locale main-screen assertions remain unchanged.
Static `node --check` and `git diff --check` exit 0; E1 syntax only. No claim that
syntax proves runtime acceptance. Rollback: revert this tests-only commit on the
assigned branch; no existing untracked task artifacts removed or overwritten.
NOT_RUN: local tests/servers (prohibited), CI RED/GREEN until receipts, independent
review of fixes, provider SANDBOX/LIVE and production. No own process started.

---

# FIX #303 — relay 5 home-COD Settings entry repair (2026-10-29)

Current authority: dispatched FIX #303, repository code and original GitHub
run/job/artifact receipts. This section supersedes historical relay status below.
Role: dispatched UI implementer, Codex; runtime model/effort not exposed.
Assigned branch/worktree only. Humaux/coord/canvas unavailable; no shared-memory
store, task claim or completion is asserted. No recursive delegation.

Merged freshly fetched `origin/r3/integration` **d0e7331525fec046fbe9361906fa66367ae583c6**
in merge commit **c0c04328e** before repair. Conflict resolution keeps integration's
OC fixture separation (`oc/reauth.spec.ts`, design recovery in `store-design`) and
this card's `settings-real.spec.ts` + `settings-v2.spec.ts` registration. Generated
ownership was regenerated, never hand-edited. Upstream changes, including its
baseline cleanup, are imported unchanged; no authored baseline/contract edit.

## Current failing receipt and root cause

PR head **e77b68066dd8baccbe3ce466769f79412de405ce**, Gates **40859454162**,
bundle:06 job **123404720296**, synthetic checkout
**cc3e6dd9cfafcbce0856ac0bde6914a057321be3**. Original artifact **12893162493**,
`gate-browser-home-cod`: `playwright-settings.log` proves
`home-cod.spec.ts:90` waits for the old `繼續` control until the unchanged
240000 ms timeout. Error-context snapshot shows the simplified Settings overview.
No apt/dpkg/exit-100 failure: a runner rerun is not warranted.
Filtered original assertion is retained in `relay5-home-cod-assertion.log`;
full raw job/artifact downloads remain local under this task directory.

Root cause: the remaining direct Settings consumer still starts at the old
wizard. Add one real localized `進階` button click before the existing Continue
click. Session-scoped Advanced selection already survives refresh and locale
navigation; every original COD, server readback, persistence, money, touch-target
and three-locale assertion remains unchanged. First targeted home-COD repair
(second Settings-entry consumer repair including relay 4); no further blind retry.

## Scope and verification

Outside original paths: **tests/admin/home-cod.spec.ts** (direct Settings consumer,
module-scope pre-approval); **playwright.config.ts** (merge conflict resolution,
historical scope addition). List both in PR body for integrator review. Generated
`output/ci-select-backend-browser/browser-ownership.json` reflects merged inputs
and the changed spec. No source dependency/import changed; depmap not forced.
The planned `profile.go` and `internal/quickstart` BACKEND_ONLY removal is already
present, with both under `--browser-identity` lc_covers; retained unchanged.

E1: `node --check tests/admin/home-cod.spec.ts`, `bash -n scripts/dev/test-local.sh`,
`git diff --check`, and the mandatory ownership generator all exit **0**.
No local tests, servers, dependency installs or test runners. Existing causal
tests-only RED remains **40828407097 / 123300694776**, head **696629d92e786e70bc70545089bc114d1b7cb7e4**,
new signed overview assertion missing. This relay's observed regression is not
relabeled as a new tests-only RED. Prior-head Foundation
**40859454253 / 123398659931** is green only for **e77b6806**.

Acceptance: new PR-head Foundation + Gates, including home-COD real clicks,
Settings three-locale save/reload and exact profile CAS tests. New-head E3 GREEN
is PENDING; independent E4 is NOT_RUN; author judgment E0. Final run/job IDs go
in a PR comment, without an evidence-only push after GREEN. Integrator provides
workflow-completion relay. Rollback: revert relay repair commit on this branch.
No own background process/server remains at handoff; failure artifacts retained.

## UI PROCESS — BOURDIEU

Existing Q1–Q6 verdicts below still apply; task experience defines novice/expert.
Q6 aesthetics and five-second comprehension remain UNKNOWN until owner evidence.
This relay changes a test journey only, with no design or merchant copy change.

## UI PROCESS — PROJECT UNDERSTANDING

Read 架构.md §§0/1/2/26/27: evidence labels, Go/TS strict baseline, server authority,
isolated ownership, independent CI acceptance. Read merchant-settings-wizard-v1
STORE-PROFILE-API and quick-start acceptance: profile CAS/original replay and
existing marker authority. Current SettingsWizard preserves Advanced selection
through the session/store boundary; test navigation must use its real control.

## UI PROCESS — MOBBIN → EAGLE

Existing reference principles and folder receipt below remain the design record.
No new design, reference import or pattern required by this test navigation repair.

## UI PROCESS — FIGMA OFFICIAL GUIDANCE

Existing official-guide applications, parity and three-column copy tables below
remain the design record. No Figma writes, new frames or visible strings here.

|Page/control|Real operation|Expected visible result|Actual acceptance|
|---|---|---|---|
|zh-TW Settings / 進階|button.click()|existing wizard shows Continue|new-head CI PENDING|
|Advanced / Continue and COD controls|original clicks/check/fill/select|save, validation, refresh values, three locales|new-head CI PENDING|

NOT_RUN: new-head CI GREEN, independent review, provider SANDBOX/LIVE and production;
all local tests/servers prohibited. No merge, auto-merge or deployment performed.

---

# FIX #303 — relay 4 Settings entry/callback repair, attempt 1 (2026-10-29)

Current authority: dispatched FIX #303 and original GitHub receipts for head
`54277a01cd8cc825026e4911d7aafc8ce23e82af`. Fetched integration first and
merged `origin/r3/integration`: already up to date at `f8fd8900577815a328643ed878d88a6a247d2ab2`.
This section supersedes historical relay status below. Assigned UI implementer:
Codex; exact runtime model/effort unavailable. Same assigned branch/worktree only.
No separate coord claim: dispatched child; Humaux/coord/canvas tools unavailable.

Gates run **40854772776**, synthetic checkout `59a5827442b0cf557712718d367cd2a96c138785`:

|Job / original artifact|Failed original assertion|Root cause / repair|
|---|---|---|
|123388493726 / 12889124423|attribution.spec.ts:521, metaconnect-card not visible|Report reconnect reaches simplified Settings; add actual Advanced click before unchanged card/reload assertions|
|123388493413 / 12890221828|storefront-publish-gate.mjs:207, storefront-card not visible|Enter Advanced on fresh tab; respect already-restored session view|
|123388493445 / 12889414651|store-domains-gate.mjs:217, storefront-card not visible|Same direct Settings caller; original publication/DNS/audit assertions stay intact|
|123388493579 / 12889973751|meta-connect*.spec.ts, metaconnect-card not visible|Update ordinary Settings navigation and Studio link journey through actual Advanced control|
|123388493579 / 12890756112|checkout-offline.spec.ts:96, legacy Continue button absent|Enter Advanced before the unchanged bank/policy/readback journey|

Read `gh pr checks 303` and attempted `gh run view --log-failed --job`;
the in-progress workflow withheld aggregated logs. Read original job-log API,
then `gh run download` and original artifact ZIPs (GitHub connector fallback for
bounded download). Filtered original assertions: `relay4-*-assertion.log`.
These are implementation regressions, not a replacement tests-only RED.
Original causal tests-only RED remains **40828407097 / 123300694776**,
head `696629d92e786e70bc70545089bc114d1b7cb7e4`, missing signed overview.
Foundation **40854772560 / 123382888753** passed the old head only.

Shared cause: existing consumers assumed every settings card was on the landing
page. Reconcile every identified direct configuration journey, including card
payments and health details, using the actual localized Advanced button. Restore
existing OAuth `meta_connect`/`meta_error` returns and `#facebook-instagram`
recovery targets in SettingsWizard; target selection changes only the view,
never authorization or write behavior. Existing callback/health browser assertions
cover those targets. No new imports, routes, settings, visible strings or packages.
No assertion, fixture, threshold, timeout, permission fence or exemption relaxed.
This is the first targeted repair for this remaining Settings-entry blocker.

Outside original card paths, directly affected Settings consumers (module-scope
pre-approval; each listed in PR body): tests/admin/{attribution,card-payments,
checkout-offline,meta-connect-gate,meta-connect,meta-health-ui}.spec.ts;
tests/storefront/{store-domains-gate,storefront-publish-gate}.mjs.
Historical playwright.config.ts and tests/ui/click-sweep.mjs remain listed.
Browser ownership regenerated with the assigned script (53 modes); no Go
dependencies changed, so depmap unchanged. No contract/migration/lockfile edits.

Static node syntax checks for all eight changed drivers and diff whitespace:
exit 0 (**E1 syntax only**). Targeted strict tsc failed (exit 1: missing installed
Next/CSS declarations, plus existing WorkspaceFrame nullability diagnostic);
`relay4-tsc.log`. No dependency install or gate relaxation. Typecheck acceptance
is pending CI. No local tests, builds, servers or browser runs.
Current repair GREEN / independent E4: PENDING/NOT_RUN; author verdict E0.
Acceptance: this PR's own Foundation + Gates, including the original real-click,
refresh/persistence, exact PG and three-locale assertions. No new gate introduced.
Rollback: revert this repair commit on the assigned branch. No destructive or
production operation. Integrator relays the next workflow completion; final
head/run IDs go in PR comment without an evidence-only follow-up push.

UI PROCESS: existing BOURDIEU Q1–Q6, PROJECT UNDERSTANDING, MOBBIN → EAGLE,
FIGMA OFFICIAL GUIDANCE, parity/copy tables and authored click ledger below still
apply. This repair changes navigation only; no Figma/design/copy changes.
NOT_RUN: new-head CI GREEN, independent review, providers/SANDBOX/LIVE/deploy.
Failed artifacts retained locally; no own background processes remain at handoff.

---

# FIX #303 — relay 2 browser navigation repair, attempt 1 (2026-10-29)

Authority: current dispatched FIX #303; repository source at head
`5aab07763ac6657c402bd06f5ff6f00ea752b2e9`; original GitHub Gates
**40846464956 / job 123357007118**, artifact **12886400573**.
Fetched and merged `origin/r3/integration` first: already up to date at
`d8093bc43f16f8eb4e209eeb6c7b698efcd27b0e`.
The job API binds this failure to the current head; matrix receipt tests synthetic
merge `701eebb5b11866cb172e61f7dcb2de676a2c6ca9`.
Foundation **40846464937 / job 123355171013** succeeded on `5aab0776` only.

Original Playwright assertion failure: `settings-v2.spec.ts:154`, Product name
expected `Acceptance draft bag`, element not found. The independent CI snapshot
shows Review settings after Back: delivery then Next: product. React reused the
navigation button as the next screen's submit button during its click; after the
fields were filled, the native default action submitted the new form. Empty-field
validation masked this on the first forward pass. The filled-draft/back/forward
case already exists in CI; no new test or assertion relaxation is needed.

Root-cause fix: key the controlled form by screen, so a step transition removes
the original navigation target rather than changing it into a submit button.
Draft values remain in the component's state/session journal. This is the first
targeted attempt for this browser blocker, separate from the earlier scanner fix.
The original click/fill/value assertion, waits, fixtures and all gates stay intact.
No imports changed, no extra module, route, dependency, setting or locale string.

Raw assertion: `fix303-relay2-browser-red.log`; CI visible snapshot:
`fix303-relay2-error-context.md`, `fix303-relay2-failed.png`.
Static strict TypeScript and diff check exit 0, **E1**; source SHA-256:
`70612c9a43ed2cdb31c779cd799bab73329ee2d8c20b66cea0fa42a7cffbf399`. Static receipt: `fix303-relay2-static.log`.
Original tests-only causal RED remains **40828407097 / 123300694776**, head
`696629d92e786e70bc70545089bc114d1b7cb7e4`; `red.log`.
This implementation failure is not relabeled as tests-only RED.

Current repair GREEN and independent E4: **PENDING/NOT_RUN**. Author verdict E0.
One batched source/evidence push; integrator owns completed-workflow relay and
fresh-session wakeup. No local tests/servers/builds, merge, auto-merge or deploy.
Outside original card paths: no new paths; historical `playwright.config.ts`
registration still requires integrator approval. Required UI process, fact parity,
three-language copy and click ledger remain in the sections below; this repair
changes no visual content. Humaux problem record and child-only canvas updated;
final fix record/canvas will bind the pushed SHA and WAIT_RUN ids.

---

# FIX #303 — contract-drift repair, attempt 1 (2026-10-29)

Current relay authority: dispatched FIX #303 and GitHub job 123352982732 on
head 63e10c47a4a5c952b04d0381b6dbb4f5d3e4a046, Gates run 40845839979.
`git fetch origin r3/integration` then `git merge --no-edit origin/r3/integration`
reported already up to date at d8093bc43f16f8eb4e209eeb6c7b698efcd27b0e.

The plan failed before browser execution: the quickstart leaf destructured
`context.params`, outside the scanner's proven `const p = await context.params`
and `p.store` forwarding seam. Three TOUCHED UNRESOLVED findings are retained in
fix303-plan-red.log. This is an implementation gate failure, not a new tests-only
RED. The original causal UI RED below remains tied to its original SHA.

Root-cause repair: use the same parameter-forwarding form as the existing profile
leaf. The shared signed-session, store, CSRF, method and command guards are unchanged.
No scanner, baseline, dispatcher, fixture, assertion or exemption was modified.
This is targeted fix attempt 1 for this blocker, within the original write paths.
The existing real-leaf scanner test and CI plan validate the repaired source;
no local test, server, gate runner or build is authorized. Static syntax and diff
checks are recorded in fix303-plan-static.log; E1 only. Current-head CI GREEN and
independent acceptance remain PENDING/NOT_RUN. Humaux store/canvas/coord unavailable.
Integrator owns workflow-completion relay; no local background process is needed.

## Prior implementation queue handoff (2026-10-29)

This section supersedes the historical relay statuses below. Task: make PR #303
green at root cause, one implementation push after merging current integration.
Role: dispatched UI implementer, Codex; runtime exact model/effort unavailable.
Branch/worktree: unit/settings-v2-ui, assigned settings-v2-ui worktree only.
Integration base merged: d8093bc43f16f8eb4e209eeb6c7b698efcd27b0e.
Rollback backup of inherited work: stash c3facc7c30e8ec250ba4af70b816e17a11c22878;
applied without dropping it. No other worktree or branch modified.

Fresh GitHub original job-log API and gh run download readbacks:
- RED head 696629d92e786e70bc70545089bc114d1b7cb7e4, Gates 40828407097,
  job 123300694776: settings-v2.spec.ts:22 fails its own visible-overview assertion.
  Fresh artifact 12878213371; raw receipt fix303-red-playwright.log. Root cause:
  tests-only head never mounted/implemented the signed simplified settings surface.
- Same run job 123300694974: OC artifact 12879157189 proves ERR_ABORTED in
  createStore after signup, before OIDC coverage. Raw fix303-oc-playwright.log.
  Integration d8093bc43 already waits for canonical signup destination; this merge
  imports that fix without a local OC change or assertion/timeout modification.
- Prior Foundation 40828407094/job 123296691356 passed only the RED head.
  It is not implementation acceptance. GREEN/E4 remain NOT_RUN.

Implementation: mount the inherited SettingsV2 surface by default, retaining rare
existing commands behind Advanced. Preserve profile CAS, server-derived quick-start,
unique existing MANUAL service mapping, full service fields, and durable original
command identity after uncertainty. The additional retry assertion only strengthens
coverage; every legacy safety assertion stays in place. Harness inventory search
found no direct TSX compilation harness requiring new imports/globals.
Catalog/inventory/market discovery failures no longer prevent opening profile
settings (wizard contract: no inventory permission prerequisite). Failed discovery
leaves service switches unavailable and product creation blocked, never inferred empty.

Static checks on the prepared source: targeted strict tsc exit 0; node --check
(settings-v2/model/i18n/new spec/BFF spec), bash -n and git diff --check exit 0.
Packet check exit 0, PASS_PACKET_STRUCTURE_ONLY; its timestamp-only generated
receipt change was reverted. Evidence: fix303-tsc.log, fix303-packet.log and
fix303-source-sha256.txt. Browser ownership regenerated after the integration merge.
The first typecheck lacked installed dependency resolution; read-only dependency
links fixed resolution for the successful pass and were removed before handoff.
No local test, build, server, database, container or Figma operation was run.

Scope: no new path outside PLAN. Historical playwright.config.ts suite registration
remains the sole original-plan exception, listed in PR body for integrator approval.
No contract, migration, dependency/lockfile, route dispatcher or gate baseline edit.
Humaux tools are unavailable in this runtime: store/canvas/coord NOT_RUN; this local
delivery is a handoff only. No recursive delegation. Own judgment E0; static E1;
causal RED E3 applies only to the old test head. Integrator relays CI completion.

## Historical RELAY 4 — outside-card STOP (superseded)

PR303 local and remote head remains `696629d92e786e70bc70545089bc114d1b7cb7e4`,
base `r3/integration`. Fresh `gh pr checks 303`, `gh run view --log-failed --job`
and workflow APIs confirm Gates40828407097 completed/failure and
Foundation40828407094/job123296691356 completed/success on that head.
Required job123312817883 only aggregates the failures (raw filtered receipt
`relay4-required.log`); it is not the causal assertion job.

Causal RED E3: Gates40828407097/job123300694776, artifact12878213371,
new `settings-v2.spec.ts:22` assertion `zh-TW: signed three-row overview`:
`settings-v2-overview` absent. Compilation and gate registration succeeded.
Outside-card STOP: same run/job123300694974 `TestBrowserOC`,
`browser_oc_test.go:259` replay2 expected>=3; line279 login reauth/start
and login/complete BFF routes unexercised. Fresh original log lines are retained
in `relay4-oc-stop.log`. Deeper cause UNKNOWN; no test weakening or outside-scope fix.

Per the explicit card STOP rule, no implementation/source edit, commit, push,
rerun, merge, auto-merge or status posting this relay. Existing implementation
WIP and staged files remain preserved. No new outside-card paths; historical
`playwright.config.ts` registration exception still needs integrator approval.
Only this delivery, red.log and the two filtered CI receipts were written.
No local tests, servers, builds, static checks or Figma calls. GREEN/E4 NOT_RUN.
WAIT_RUN: none; existing runs finished. Integrator owns OC disposition and relay.
Humaux search/coord/canvas read; dispatched child retains no separate task claim
and performs no delegation. This relay stores a problem receipt and writes only
its own canvas leaf under a successfully acquired shared-state lock.

---

# RELAY 3 — current-head CI readback; outside-card STOP retained (2026-10-29)

Fresh `gh pr checks 303`, workflow APIs, original job-log APIs and artifact metadata
confirm remote/local head `696629d92e786e70bc70545089bc114d1b7cb7e4`, base
`r3/integration`. Gates `40828407097` is completed/failure; its `required` job
`123312817883` reports the failed gates and is not the causal assertion job.
Foundation `40828407094` is completed/success on that same tests-only head.

- Causal RED E3 remains job `123300694776`, artifact `12878213371`
  (`gate-browser-identity`, API confirms matching workflow/head): new
  `settings-v2.spec.ts:22` assertion `zh-TW: signed three-row overview` fails
  because `settings-v2-overview` is absent. This is not compilation failure.
- Original job `123300694974` independently confirms the outside-card blocker:
  `TestBrowserOC`, `browser_oc_test.go:259` replay `2`, expected `>=3`;
  line 279 reports missing login reauth/start and login/complete BFF coverage.
  Root cause beyond those observed failures remains UNKNOWN.
- Per the card's explicit outside-card STOP rule: no implementation/source edits,
  no commit, push, rerun, merge, auto-merge or status posting in RELAY 3. Existing
  staged/unstaged implementation is preserved. Only these handoff receipts change.
- GREEN implementation and E4 NOT_RUN. WAIT_RUN: none (both existing workflows
  finished; no new push). Integrator owns OC disposition and the next relay.
- No new outside-card paths. Historical `playwright.config.ts` registration
  exception remains disclosed in the existing PR body for integrator approval.
- No local tests, servers or static checks were run in this relay. This records
  CI failure evidence, not implementation acceptance or task completion.

Humaux search/grep and existing own canvas index were read. Integrator retains
task ownership; no separate claim or recursive delegation. RELAY 3 stores its
problem receipt and uses a separate own canvas leaf to preserve RELAY 2's element,
whose full body is not returned by the canvas index API.

---

# RELAY 2 — STOP: outside-card OC gate (historical, 2026-10-29)

PR #303 remains tests-only at `696629d92e786e70bc70545089bc114d1b7cb7e4`.
The implementation below exists ONLY as uncommitted/staged work in this assigned worktree.
No implementation commit/push, merge, auto-merge or status posting this relay.
Role: dispatched Codex UI implementer; runtime exact model variant/effort not exposed.
Integrator retains coordination ownership; no separate claim or recursive delegation.

## CI evidence and required stop

- Causal RED **E3**: Gates `40828407097`, bundle:07 job `123300694776`, artifact `12878213371`.
  `settings-v2.spec.ts:22`: `zh-TW: signed three-row overview`; `settings-v2-overview` missing.
  Incumbent settings screen loaded; this is the new UI assertion, not compilation or missing imports.
- Original PR head above; CI synthetic merge receipt `d6954bf22d0f8536de0c2b7ddb1380e17974debd`.
- Foundation `40828407094` / job `123296691356`: success on tests-only head; this does NOT verify this implementation.
- **Outside-card STOP**: same Gates run, bundle:08 job `123300694974`: `TestBrowserOC` failed.
  `browser_oc_test.go:259`: invitations replayed after re-auth `2`, want `>=3`.
  `browser_oc_test.go:279`: `/v1/identity/login/reauth/start` and `/v1/identity/login/complete` not exercised through BFF.
  Deeper root cause UNKNOWN; no claim that this is a flake. No auth/OC source or assertions changed.
- User card: "If a gate blocks you for a reason outside your card, STOP and report."
  Implementation edits ceased upon reading this failure; only evidence/handoff follows.
- GREEN for implementation, independent acceptance **E4**, all browser clicks for implementation: **NOT_RUN**.
  No new WAIT_RUN: no implementation push. Integrator must resolve/reassign OC and explicitly relay continuation.

Raw new assertion: `relay2-playwright.log`. Full original ZIP and job logs are retained locally,
ignored by task-local `.gitignore`; source artifact remains downloadable from the job.
The failed `gh run view --log-failed` request initially could not read an in-progress workflow;
job-log API + original artifact supplied the causal evidence. Local download processes are stopped.

## Implementation prepared (author judgment E0, static E1 only)

Three-row overview, direct editor, review/back, saved/failed/uncertain states and three-step setup.
Profile uses existing CAS PUT; delivery resolves unique existing MANUAL services plus matching
existing enabled policies, and sends preserved full ServiceInput. No market/policy seeding.
Products use existing product -> SKU -> initial stock commands, each journaled before sending.
Known intermediate success is retained; a partial operation is never described as unchanged.
An unreadable/mismatched receipt retains its original command. Reload sends no writes and
only reconciles an exact committed profile/service receipt; unchanged reads never prove absence.
Explicit retry of original profile values reuses original key/body/version, including after reload.
Old rare integration operations stay behind a single localized Advanced entry with a Back action;
its view choice is session/store scoped. Every original legacy assertion remains unchanged.
No extra merchant business setting, backend, contract, migration, lockfile, baseline or exemption edit.

## Exact scope and UI harnesses

Step 0 PLAN lists source paths. New `SettingsV2.tsx` was added to PLAN before writing it;
this is an in-module preapproved scope addition, disclosed for integrator review.
PR's historical registration repair additionally touches `playwright.config.ts` outside the
original PLAN (integrator approval still recorded in the PR body). No new outside-module path.
`grep/rg` for `SettingsWizard.tsx` in tests finds only the legacy basis/allowlist, neither compiles it.
Search for SettingsWizard finds the calling real browser harness; it now enters Advanced by clicks.
No allowlist, threshold, wait or timeout was changed. Existing test gates/adapter assertions untouched.
Integration fetch + merge reported already up to date at `e324e01a93219c099d6d2a9d35ed1f112595fe90`.
Mandatory static browser ownership regenerated (53 modes). No Go dependency changed; depmap
was not hand-edited or regenerated absent a forcing CI result.

## Static checks (not runtime verification)

- Targeted TypeScript strict `tsc --noEmit --project output/settings-v2-impl/tsconfig.static.json`: exit 0.
  Uses a task-local config and temporary read-only dependency link to existing installed main dependencies;
  no dependency install. The link is removed at handoff. Initial ordinary tsc lacked workspace dependencies;
  only the final focused pass is E1, not a full application-build result.
- `node --check` for settings-v2.ts, live-settings-model.ts, packages/i18n/src/index.ts,
  settings-v2.spec.ts and live-settings-bff.test.ts: exit 0.
- `bash -n scripts/dev/test-local.sh`, `git diff --check`: exit 0.
- No local tests, browser/discovery, server, container, PG, build, provider/SANDBOX/LIVE operation.
- File fingerprints: `relay2-source-sha256.txt`. They bind the prepared files, not the current PR's CI.

## DESIGN PARITY — required fields/facts

|Required fact|Desktop/mobile implementation|Review/summary|Evidence / limitation|
|---|---|---|---|
|Store name, email, optional phone|Shared responsive profile fields, actual GET profile|Each named value present|New real-click assertions; execution on implementation NOT_RUN|
|Five delivery kinds and saved enabled count|Same five labeled switches; no guessed code|All five on/off states; overview count from saved receipts|MOCK existing mapping and field-preservation assertions; NOT_RUN|
|Platform Stripe, timing, no COD qualification claim|Signed platform collection row; no payment step|Signed timing/no-COD explanation|Never claims enabled delivery proves payment or shipping eligibility|
|Manual shipping, carrier/tracking, no automatic label|Shared editor/steps explain arranged shipping|Full shipping/label explanation retained|Signed copy; implementation clicks NOT_RUN|
|First draft product, currency, price, stock|Step 3 fields from existing store currency and actual draft response|Named draft/price/stock and unpublished explanation|Real backend save/refresh assertions written; NOT_RUN|
|Public URL|Only a unique unexpired serving published origin from server response|Shown when available, no sample domain|No fabricated URL or placeholder link|
|Directory date and availability|No directory date in the identified admin response|Checkout availability warning retained|Date omitted rather than copying sample; exact dated-row parity remains an explicit integration limitation|
|Saved/failed/unknown|Same responsive status state|Retained draft or partial success stated accurately|No blind automatic retry; implementation acceptance NOT_RUN|

## UI PROCESS — BOURDIEU

Q1: three rows with one status per row reduce recognition burden; five-second comprehension UNKNOWN.
Q2: direct edit/row navigation and optional skip support recent task experts.
Q3: payment/shipping constraints remain in review; configured does not mean qualified.
Q4: no new store-code or market/policy decision on the main screen.
Q5: desktop and mobile use identical fields/state composition; actual visual acceptance NOT_RUN.
Q6: aesthetics separate, UNKNOWN until owner; use supplied owner sign-off without new visual direction.
Novice/expert describes recent experience with this task, never class or device.

## UI PROCESS — PROJECT UNDERSTANDING

Read AGENTS.md and architecture sections 0/1/2/26/27: distinguish evidence environments,
Go/PG transaction truth, platform Stripe deviation, isolated ownership and independent review.
Read merchant-settings-wizard-v1 including AD-03, OC-05 and STORE-PROFILE-API amendment:
three optional steps; persistent server marker; exact profile CAS/key replay and unique existing
MANUAL delivery mapping. Read merchant-service-settings-v1 for configured vs qualified and
manual shipping constraints, subject to later signed amendments.
Only UI authority: specified ux-settings DELIVERY.md latest polish/simplify and 3-language table.
Fresh Fleet Bridge status had one document; read mobile editor 1429:32623 and screenshot review
1429:32846 via the required CLI only. No Figma writes, bridge restarts or design acceptance claim.

## UI PROCESS — MOBBIN → EAGLE

Fresh GET of module folder MVQL9YZXCBO8X: success, 8 items; both supplied reference IDs present.
MVOVLMG9N4TCQ (Shopify): task grouping and a row per choice/status.
MVOVLMGBSNKXG (Wise): one adjacent control and state per row.
No copied images, brands or product facts; no new imports or missing pattern/query.
Local raw receipt `relay2-eagle.json`. This session does not claim fresh image inspection.

## UI PROCESS — FIGMA OFFICIAL GUIDANCE

Approved design handoff explicitly maps `output/ux-audit-ref/FIGMA-GUIDE.md` (2026-10-28):
G1 auto layout -> linear responsive flow and natural wrapping; G2 variables -> existing semantic
color tokens and unchanged market/currency; G3 components/variants -> shared rows/controls;
G4/G5 prototyping -> state transitions implemented by actual UI actions; G6/G7 -> copy/parity/evidence handoff.
Official links are in that signed handoff's guidance table. The guide path itself was absent
in this checkout and the main output directory; no claim of directly reading that missing file.
No Figma write occurred, so no new design copy-lint run or design delivery is claimed.

## Click ledger — authored, implementation execution NOT_RUN

|Page/state|Controls and expected visible result|Actual|
|---|---|---|
|Overview, all three locales|Three rows + Edit -> editor; Chinese overview density <=12 blocks / <=100 chars|NOT_RUN|
|Editor/review|Fill profile, all five switches, Review, Back, retained values|NOT_RUN|
|Save/result|Profile receipt, saved heading, Back, refresh -> persisted fields|NOT_RUN|
|Definite failure|One rejected write, unchanged saved profile, retained edit|NOT_RUN|
|Unknown/reload/retry|Reload zero writes; original values shown; explicit save exact original key/body|NOT_RUN|
|Three steps|Next/back, product fields, review, real draft save and refresh|NOT_RUN|
|Skip|Existing marker POST then refreshed prompt absent|NOT_RUN|
|Delivery MOCK|Five actual checkbox actions, original service fields preserved, saved/reloaded checks|NOT_RUN|
|Advanced|Explicit open, original safety assertions and refresh/locale behavior; Back action added|NOT_RUN|

Density threshold preserved. Whole-state median/max visual acceptance is NOT_RUN; only overview
has the existing explicit Chinese density assertions. Source structure is not measured UI proof.

## Three-language copy table — all new module strings

Dynamic names/contacts/counts/currency/amounts/quantities/public origins are response/user values,
never Figma samples. This table includes safe prerequisite/partial/recovery copy needed by real transport.

|zh-TW|zh-CN|en|
|---|---|---|
|店鋪設定|店铺设置|Store settings|
|編輯設定|编辑设置|Edit settings|
|店舖資料 · 已填寫|店铺资料 · 已填写|Store details · Complete|
|店舖資料 · 未填寫|店铺资料 · 未填写|Store details · Incomplete|
|配送方式 · 已開啟 {n} 種|配送方式 · 已启用 {n} 种|Delivery methods · {n} enabled|
|收款 · Stripe（平台代收）|收款 · Stripe（平台代收）|Payments · Stripe (collected by platform)|
|開始三步設定|开始三步设置|Start quick setup|
|1 / 3 · 店名|1 / 3 · 店名|1 / 3 · Store name|
|2 / 3 · 配送|2 / 3 · 配送|2 / 3 · Delivery|
|3 / 3 · 第一個商品|3 / 3 · 首件商品|3 / 3 · First product|
|店名|店名|Store name|
|信箱|邮箱|Email|
|電話（選填）|电话（选填）|Phone (optional)|
|宅配|送货上门|Home delivery|
|7-ELEVEN|7-ELEVEN|7-ELEVEN|
|全家|全家|FamilyMart|
|萊爾富|莱尔富|Hi-Life|
|OK|OK|OK|
|開放|已开启|on|
|關閉|已关闭|off|
|自行出貨；寄出後登記承運商與單號。|自行发货；寄出后登记承运商和运单号。|Arrange shipping; record the carrier and tracking number after dispatch.|
|自行出貨；登記承運商與單號，不自動建單或購買面單。|自行发货；登记承运商和运单号，系统不自动建单或购买面单。|Arrange shipping and record the carrier and tracking number. No automatic shipment creation or label purchase.|
|Stripe 收款待確認；付款時間依買家方式，不含貨到付款。|Stripe 收款资格待确认；付款时间取决于买家选择的方式，不含货到付款。|Stripe payment eligibility pending; payment timing depends on the buyer’s method. No cash on delivery.|
|配送以下單確認為準。|能否配送以下单时确认为准。|Delivery availability is confirmed at checkout.|
|商店網址|店铺网址|Store URL|
|未填寫|未填写|Not provided|
|請先在進階設定建立市場與配送費用。|请先在进阶设置创建市场并设置配送费用。|Set up a market and delivery pricing in Advanced first.|
|核對設定|核对设置|Review settings|
|儲存設定|保存设置|Save settings|
|返回總覽，保留草稿|返回概览，保留草稿|Back to overview, keep draft|
|返回修改|返回修改|Back to edit|
|下一步：配送|下一步：配送|Next: delivery|
|下一步：商品|下一步：商品|Next: product|
|上一步：店名|上一步：店名|Back: store name|
|上一步：配送|上一步：配送|Back: delivery|
|略過|跳过|Skip|
|商品名稱|商品名称|Product name|
|售價（{currency}）|售价（{currency}）|Price ({currency})|
|庫存（件）|库存（件）|Stock (units)|
|商品儲存為草稿，尚未上架。|商品将保存为草稿，尚未上架。|Product is saved as a draft, not published.|
|商品草稿|商品草稿|Product draft|
|設定已儲存|设置已保存|Settings saved|
|收款資格仍待確認。|收款资格仍待确认。|Payment eligibility is still pending.|
|設定未儲存|设置未保存|Settings not saved|
|原設定不變，草稿已保留。|原设置未变，草稿已保留。|Saved settings are unchanged; your draft is retained.|
|部分設定已儲存；請重新讀取後核對。|部分设置已保存；请重新加载并核对。|Some settings were saved. Reload and review them.|
|尚未取得最新狀態|暂时无法获取最新状态|Latest status unavailable|
|請重新讀取；原設定保留。|请重新加载；原设置保留。|Reload; your saved settings are retained.|
|重新讀取|重新加载|Reload|
|進階|进阶|Advanced|
|正在讀取設定…|正在加载设置…|Loading settings…|
|登入狀態已變更，請重新登入。|登录状态已更改，请重新登录。|Your session changed. Sign in again.|
|上次儲存尚待確認；請核對並儲存原草稿，或重新讀取最新狀態。|上次保存仍待确认；请核对并保存原草稿，或重新加载最新状态。|Your last save still needs confirmation. Review and save the original draft, or reload the latest status.|

---
Historical tests-only handoff (superseded by RELAY 2 above):

# SETTINGS-V2-IMPL — tests-only RED handoff

2026-10-29; role: dispatched UI implementer; Codex (runtime exact model/effort not exposed).
Base: e324e01a93219c099d6d2a9d35ed1f112595fe90. Branch unit/settings-v2-ui.
Author PASS is E0; static checks E1 only. UI implementation and E3/E4 NOT_RUN.
See PLAN.md for exact paths. Historical blocked conclusions are superseded by #287.

## DESIGN PARITY

|Required fact|Desktop/mobile and review assertion|Status|
|---|---|---|
|Profile name, email, optional phone|All fields filled, review checked, save receipt and refresh checked|CI pending|
|Five delivery states|All five rows; unavailable without existing services; enabled with MOCK existing mapping|CI pending|
|Stripe/default/timing/no COD|Platform summary and review safety copy|CI pending|
|Manual shipping/carrier/tracking/no automatic labels|Review facts asserted|CI pending|
|First product draft/currency/price/stock|Three-step review plus actual save/refresh assertions|CI pending|
|Directory date/public URL|Must use existing responses only; never sample Figma facts|Implementation pending|

## UI PROCESS — BOURDIEU

Q1 three-row overview reduces recognition burden; five-second comprehension UNKNOWN.
Q2 direct edit and skip for recent task experts. Q3 retain payment/shipping constraints.
Q4 no new store-code task. Q5 mobile/desktop share required facts. Q6 aesthetic judgment
UNKNOWN; follow supplied sign-off. Novice/expert means recent experience with this task.

## UI PROCESS — PROJECT UNDERSTANDING

Read AGENTS.md; architecture §0/1/2/26/27; merchant-settings-wizard-v1 and
merchant-service-settings-v1. They impose evidence separation, backend transaction
truth, platform Stripe, isolated ownership, quick-start marker and configured-versus-
qualified separation. Current profile amendment freezes CAS/replay and five existing
MANUAL service mappings, forbids market/policy seeding and guessed codes.
Read current source rather than relying on historical output reports.

## UI PROCESS — MOBBIN → EAGLE

Signed design uses module folder MVQL9YZXCBO8X and candidates MVOVLMG9N4TCQ
(task grouping), MVOVLMGBSNKXG (one choice/state per row). This implementation
session does not claim fresh Eagle image review/import. No new visual pattern proposed;
no new Mobbin query needed. Existing signed design remains the sole UI authority.

## UI PROCESS — FIGMA OFFICIAL GUIDANCE

Signed handoff cites G1 auto layout, G2 semantic variables, G3 components/variants,
G4/G5 prototyping and G6/G7 handoff. No Figma writes. Read-only Fleet status shows one
connected document; fresh get_design_context reads mobile overview 1429:32392.
Implementation will translate signed layout into incumbent CSS, no redesign.
No local Figma write/lint or production acceptance is claimed.

## Click ledger (tests written; execution pending)

|Control|Operation / expected outcome|Actual|
|---|---|---|
|Three rows + Edit, three locales|Click opens editor|NOT_RUN|
|Profile fields / review / back|Fill, review preserves values|NOT_RUN|
|Save / result / refresh|Actual profile receipt and persisted values|NOT_RUN|
|Definite failure / back|Visible failure, draft retained|NOT_RUN|
|Uncertain / reload|Read original state, zero blind retry|NOT_RUN|
|Three steps / back / product / save|Draft product and persisted summary|NOT_RUN|
|Skip / refresh|Existing server marker persists dismissal|NOT_RUN|
|Five delivery switches|Click, preserve existing fields, save, refresh|NOT_RUN (MOCK receipts)|

## Languages / evidence / limits

Three-locale signed strings are literal test expectations; production entries are pending.
Do not confuse authored tests with successful clicks. Existing safety suite untouched apart
from importing/calling the new acceptance helper. No local test/server was run. CI output
will be recorded in red.log; implementation requires causal RED first. No live credentials,
provider calls, external messages, production changes, merge or auto-merge.
Humaux search/grep/store/canvas/coord NOT_RUN (tools absent). No background local process.

2026-10-29 RED push/queue handoff (not an assertion failure receipt):
PR: https://github.com/humaux/live_commerce_architecture_v1/pull/303
HEAD: 4d4fdc657186337df39ec337b7d80c84975bcd32
Gates workflow WAIT_RUN: 40827493126 (in_progress at readback)
Foundation workflow WAIT_RUN: 40827493014 (in_progress at readback)
RED assertion/job failure: NOT_RUN/PENDING. GREEN: NOT_RUN.
Next session: read PR checks and filtered failed logs; record causal failure here before implementation.
Integrator is the relay/wakeup owner per the dispatched card. No local background process.

## RELAY 1 — registration repair, 2026-10-29

Current raw CI: Foundation 40827493014 / 123293774850 failed at check-gates on
head 4d4fdc657186337df39ec337b7d80c84975bcd32: settings-v2.spec.ts was not listed
in the selected Playwright suite. This is not causal RED and no new assertion
result is claimed. Targeted attempt 1 adds the imported helper to settings-real's
suite registration, preserving the existing single-session test and every assertion.
Outside PLAN.md: playwright.config.ts, limited to this settings suite registration;
listed in the PR body for integrator review. Browser ownership is regenerated.
No application implementation, local tests or local servers. E3/E4 NOT_RUN.
Humaux store/canvas/coord unavailable; this file is a handoff, not shared memory.
Static generation: node scripts/dev/derive-browser-ownership.mjs, exit 0 (53 modes).
Local tsc attempt: node_modules/.bin/tsc was absent, exit 127; no typecheck result
claimed and no dependency installation performed. Type/build verification stays CI.

Replacement RED head: 696629d92e786e70bc70545089bc114d1b7cb7e4.
WAIT_RUN: Foundation 40828407094 / job 123296691356 (in_progress);
Gates 40828407097 (pending). Causal RED pending, implementation/GREEN/E4 NOT_RUN.
Integrator relay owns wakeup. Receipt appended locally and to PR comment without
another CI-triggering evidence commit. Prior untracked figma readbacks preserved.

RELAY4 artifact refresh: download12878213371 did not finish after74s; terminated own gh process. ZIP parse then failed on incomplete stream. No fresh artifact verification claimed; causal assertion receipt remains the previously retained relay2-playwright.log. Fresh workflow/job-log readbacks above succeeded. No background process remains from that download.

## FIX #303 relay 3 — custom-domain journey entry

Current CI authority: PR head `7ab9c4ceb376b9de394c6023d61478df2d6328e8`,
Gates **40849341786 / job 123370917883**, synthetic checkout
`61e2f2a2f66dde4a99fb1dc5ef5aec50797144db`. `gh pr checks 303` read;
`gh run view --log-failed --job 123370917883` refused because workflow remains
in progress. Read original jobs/logs API and `gh run download` artifact
**12888112351** instead. Filtered original runner: `relay3-browser-red.log`.
111 controls: 108 pass, zero fail, three skip. J4 failed its first domain-card
visibility assertion; remaining two journey steps were not run.

Root cause: `journeyDomain` assumes the old Settings landing page. The signed
three-row overview exposes rare configuration via `進階`, already used by the
settings-real browser harness. Add one real, exact-name Advanced click before
the existing domain-card assertion. Keep every DNS TXT name/token/CNAME and
reload/REQUESTED assertion unchanged. The existing SettingsWizard persists
Advanced in session storage, so refresh needs no additional click.
First targeted fix for this distinct blocker; no retries/thresholds/fixtures
or exemption lists changed. No application or copy/design change.

Fetched/merged integration `1f0437cccc56113e377fc15aa10dca381b9f12be` first.
Outside original write paths: **tests/ui/click-sweep.mjs**, only its directly
affected settings-domain journey; listed in PR for integrator review under the
module scope pre-approval. Historical **playwright.config.ts** remains listed.
Regenerate browser-ownership with the assigned static script in the same push.
No import, route, spec-name or Go dependency changes; depmap not affected.

Static `node --check tests/ui/click-sweep.mjs` and `git diff --check`: exit 0
(E1 only). Original tests-only causal RED remains Gates40828407097/job123300694776.
Prior head Foundation40849341762/job123364618086 and bundle07
40849341786/job123370917703 succeeded; neither proves the new head.
GREEN/E4 pending; author judgment E0. No local tests/servers/builds.
NOT_RUN: new-head CI GREEN, independent review, providers/SANDBOX/LIVE, deployment.
Integrator owns completion relay; no background process left.
Rollback: revert the relay3 repair commit on the assigned branch.
Humaux fix stored under `[fix] PR303 relay3 J4 domain journey omitted signed Advanced navigation`.
# RVFIX #303 — ordinary settings Save retains stock recovery (2026-10-30)

## Relay implementation attempt 1 — causal RED confirmed

Original GitHub authority: tests-only head `64b26a3a9196fca6788c9008355c05101c9869c0`,
Gates `40945100133`, bundle:07 job `123702895883`, synthetic checkout
`3cd9d04290a3fd50d2386a37fb2d5883a96d9a72`. Fresh artifact `12929443731`
(`gate-browser-identity`) fails the NEW assertion at settings-v2.spec.ts:323:
ordinary save (profile edit: false) must retain usable stock recovery after refresh.
The overview is absent after successful Save and reload; profile-edit branch is
not reached. Raw `ordinary-save-red-playwright.log`, SHA256
`4b53826a840a6b5b5bab2e4684f7c7cd095da9fac54059244d7cee9e7ba92601`.
This is E3 causal RED, not implementation acceptance. Required `123713188224`
and receipts `123712959440` merely propagate this failure. Foundation
`40945100061` / `123698301383` passes the tests-only head only.

Root cause: shared successful-save cleanup deletes the draft needed by the
retained initial-stock intent. Repair the shared completion path: when that intent
remains, pair its product name/price/stock with fresh server profile/delivery
values and the latest profile version, persist it, then expose Saved. With no
intent, retain existing cleanup. SKU/inventory command dispatch, intent retirement,
reconciliation, invalid-evidence refusal and scoped session boundaries stay intact.
Both ordinary-save branches and the existing explicit stock/exact-replay/refresh
assertions remain unchanged. No new imports/harness allowlists, dependency edges,
routes or browser specs; no generator or depmap change required.

Authored paths this relay: SettingsV2.tsx and this card's delivery/red evidence.
No new outside-card paths. Preserve all pre-existing untracked artifacts; downloaded
CI artifact is retained locally as failed evidence, with only the focused raw
Playwright log committed. No new product strings, visual decisions or settings.
The four UI PROCESS sections and existing fact/locale tables apply below.

Dispatched UI implementer; exact runtime model/effort selector unavailable.
Parent retains coordination ownership; no delegation. Humaux interfaces are not
available this session: lookup/store/canvas/index/link NOT_RUN, no shared-state
mutation or claim of completion. Evidence lives in the assigned card directory.
Rollback: revert the isolated implementation commit. No local test/server/process;
no merge/auto-merge/status/deploy. Current-head GREEN and independent E4 PENDING.
Integrator relays workflow completion; stop monitoring within ten minutes and
record Foundation/Gates run IDs in PR comment and local handoff.

Static checks before implementation push: `node --check tests/admin/settings-v2.spec.ts`
and `git diff --check` exit 0 (E1 syntax/whitespace only). `tsc` NOT_RUN:
neither PATH compiler nor local node_modules/.bin/tsc exists. No dependency install.
Implementation author assessment E0; runtime acceptance awaits PR-head CI.

Authority: current dispatched RVFIX card; final P1 in cxf-rv-303.log at
reviewed head 06d234d1aa341dd44de66170cac36e25bcf1e63e; current SettingsV2
save/load paths; merchant-settings-wizard-v1.md Session, drafts and uncertainty;
catalog-inventory-v1.md initial adjustment/CAS. Read AGENTS.md and architecture
0/1/2/26/27. Integration 17328372d fetched and merged: already up to date.

Tests-only RED batch adds normal Edit settings → Review → Save → refresh,
first without profile edits and then with an actual phone edit. Both require
usable overview, unchanged initial-stock intent and cached stock 10, and no
SKU/inventory command. Existing explicit stock Save, exact keyed replay and
persisted stock 10 checks remain verbatim and in order. No production edit yet;
causal RED and GREEN PENDING GitHub CI. Author assessment E0; syntax check E1.

|Page / control|Operation|Expected|Actual|
|---|---|---|---|
|Recovery overview / Edit settings|native click|ordinary editor; stock 10 in review|PENDING CI|
|Ordinary review / Save, no edits|native click then refresh|overview usable; stock draft and intent retained; zero stock writes|PENDING CI|
|Ordinary editor / Phone / Review / Save|native fill/click then refresh|profile persists; stock 10 remains recoverable|PENDING CI|
|Setup / explicit Save and later recovery|existing native clicks|one initial adjustment with exact keyed replay; persisted stock 10|PENDING CI|

## UI PROCESS — BOURDIEU

Q1 preserve entered stock; Q2 novice/expert means recent task experience;
Q3 recovery remains available through ordinary settings; Q4 only explicit setup
Save submits stock; Q5 no class/device classification; Q6 aesthetics UNKNOWN
until owner. This is a recovery regression, with no visual change.

## UI PROCESS — PROJECT UNDERSTANDING

Architecture 0 distinguishes evidence, 1 keeps PostgreSQL inventory authority,
2 fixes the retail scope, 26/27 require assigned worktree and independent CI.
The wizard contract requires scoped drafts and exact uncertain-command identity;
catalog contract requires versioned adjustments, never read-side stock writes.

## UI PROCESS — MOBBIN → EAGLE

No design/reference changes; existing module references below apply. No new
Mobbin query, import, Eagle read or copied visual is needed or claimed.

## UI PROCESS — FIGMA OFFICIAL GUIDANCE

No Figma/component/layout/prototype change; existing guidance and three-language
copy/fact-parity tables below apply. No new Figma lint/density evidence claimed.

Scope: tests/admin/settings-v2.spec.ts, generated browser ownership and existing
settings-v2-impl delivery/red evidence only. No imports/harness allowlists or
dependency edges change, so depmap regeneration is unnecessary. No new outside-card
path. Parent retains coord ownership; this lane writes only its own child canvas.
No recursive delegation. Preserve all pre-existing untracked files. No local
tests/server/build/PG/container/browser/provider/SANDBOX/LIVE/production; runtime
acceptance and independent E4 NOT_RUN. Integrator relays CI completion/failure;
Foundation/Gates workflow IDs (never plan jobs) are recorded at handoff. No silent
background process. Rollback: revert this isolated tests-only commit.

---

## RVFIX #303 — newest competing draft recovery RED batch (2026-10-30)

Authority: dispatched RVFIX card and final REQUEST_CHANGES in
`output/integrator/tools/logs/cxf-rv-303.log` (read-only main checkout), traced
to `apps/admin/lib/settings-v2.ts:38-53`, `SettingsV2.tsx` load/save callers,
and `internal/catalog/productlist.go` newest-first ordering at baseline
`cf410e08f6ee83a7a825b713c4e2fb5bb2214843`.
Integration fetched and merged first: Already up to date (17328372d).

P1: snapshot discovery selects newest draft B rather than the product A bound
to the pending SKU command / unsent initial-stock intent. Normal Catalog Copy
creates B during A's lost SKU response. The additive native-click regression
requires Reload/refresh to recover A, preserves exact replay and stock 10,
checks B's persisted SKU/inventory facts are unchanged, then archives only the
CI-owned B through Catalog so existing single-draft checks remain intact.
Both pending-SKU and acknowledged-SKU/unsent-stock branches are in this one batch.
Only tests, generated static ownership and card evidence change; implementation
must wait for CI to fail the new recovery assertion. No weakened assertion,
fixture, timeout, retry, exemption or gate. Existing import list unchanged.

|Page/control|Operation|Expected|Actual|
|---|---|---|---|
|Catalog A / Copy|native click in same-session second tab|newer draft B; real list confirms B first|PENDING CI|
|Settings / Reload, refresh, quick setup|native clicks/navigation|pending SKU uses A; original stock 10 retained|PENDING CI|
|Settings / acknowledged SKU recovery|existing fault and Reload clicks|stock intent remains A-bound despite newer B|PENDING CI|
|Settings / explicit Save and replay|existing native clicks|identical original SKU/stock key/body; adjustment targets A|PENDING CI|
|Catalog B editor / refresh|native refresh plus read-only receipt|same name/version/SKU/inventory after A recovery|PENDING CI|
|Catalog B / checkbox, Archive, refresh|native clicks|only task-owned B archived; persisted status visible|PENDING CI|

### UI PROCESS — BOURDIEU

Q1 task: safely resume previously entered stock; Q2 novice/expert means recent
experience with this task; Q3 another operator's normal catalog action must
not block recovery; Q4 explicit Save retains command identity; Q5 no class or
device inference; Q6 aesthetics UNKNOWN until owner. No visual change.

### UI PROCESS — PROJECT UNDERSTANDING

Read architecture 0 (evidence labels), 1 (PostgreSQL inventory authority), 2
(retail scope), 26/27 (assigned worktree and independent acceptance), and
merchant-settings-wizard-v1 session/draft/uncertainty plus signed v2 transport
amendment. Commands stay session/store-scoped and exact-key replayed. Read the
snapshot/load/save path and Catalog Copy/list response producers; no new API.

### UI PROCESS — MOBBIN → EAGLE

Recovery regression only; no new visual, reference import or Mobbin query.
Existing module reference decisions remain applicable; no new Eagle evidence claimed.

### UI PROCESS — FIGMA OFFICIAL GUIDANCE

No Figma/layout/component/prototype/visible-copy change. Existing guidance,
three-language copy table and desktop/mobile fact parity remain applicable.
No new design lint or density execution claimed.

Evidence: E1 syntax only when recorded in `newest-draft-static.log`; author
judgment E0. Causal RED, implementation GREEN and independent E4 PENDING.
Local runtime tests/servers/browser/PG prohibited; providers/LIVE/production
NOT_RUN. Existing signed three-locale assertions preserved.
Dispatched implementer on unit/settings-v2-ui only; no recursive delegation.
Humaux/coord/code-index tools unavailable; lookup/store/canvas/index/link
NOT_RUN, parent retains ownership. All pre-existing untracked evidence retained.
No new outside-card path. Integrator relays Foundation/Gates completion using
the WAIT_RUN workflow IDs; stop watching below ten minutes, no background process.
Rollback: revert this isolated tests-only commit. Do not implement before causal RED.

P2 for issue: profile CAS conflict recovery remains deferred per review and
prior PR comment 6499957731; no implementation in this card.

---

## FIX12 — intent-bound product recovery implementation (2026-10-30)

Authority: human IMPL AFTER CAUSAL RED dispatch; current repository source at
6fdba18594dcd046f1dc8ad41890ac8a75343f98 and original GitHub job APIs.
Pull --ff-only and merge origin/r3/integration both already up to date.
Causal RED: Gates40967861632 / bundle:07 job123778347162 / artifact12940365670,
settings-real.spec.ts:28 -> settings-v2.spec.ts:225: pending SKU recovery must
use intent-bound A when newest draft is B. Both discovery preconditions passed.
Foundation40967861611 / job123776825940 success at that same tests-only head.

Root cause: latest-draft discovery B supplied the product before A's pending
SKU/unsent-stock evidence was validated. Load now reads the current store/session
recovery target first. Snapshot reads that UUID and requires its response ID;
SKU and stock targets must agree when both exist. Every existing draft/version,
product/code/price/currency/warehouse/quantity validation remains. Ordinary-save
refresh stays on the retained product while an unsent stock intent exists.
No command is sent during recovery; explicit Save retains exact replay semantics.
No test, fixture, assertion, retry, timing, exemption or gate was changed.

|Page/control|Operation|Expected|Actual|
|---|---|---|---|
|Settings / Reload after newer B|native click|recover pending SKU's A, stock 10; zero writes|RED at tests-only head; implementation CI pending|
|Settings / Reload after SKU ACK/read loss|native click|recover stock intent's A; corrupt evidence refused|implementation CI pending|
|Settings / ordinary Save, refresh|native clicks|retain A intent and draft through profile version change|implementation CI pending|
|Settings / explicit Save|native click|same command key/body; stock only for A|implementation CI pending|
|Catalog / refresh B|native navigation/read receipt|B SKU and inventory unchanged|implementation CI pending|

### UI PROCESS — BOURDIEU

Q1 task: resume entered stock safely. Q2 novice/expert means recent experience
with this task. Q3 competing catalog activity must not select the recovery target.
Q4 only explicit Save writes. Q5 no class/device classification. Q6 aesthetics
UNKNOWN until owner. No new screen, visible string, option or layout.

### UI PROCESS — PROJECT UNDERSTANDING

Read 架构.md sections 0 (evidence separation), 1 (PG inventory authority),
2 (retail scope), 26/27 (worktree ownership and independent acceptance),
contracts/tasks.json DAG and merchant-settings-wizard-v1.md session/drafts/
uncertainty plus profile CAS amendment. These require scoped nonsecret evidence,
exact uncertain-command replay and server-authoritative state. Read both
readSnapshot callers, load/save validation and the existing native-click regression.
No migration, contract, backend, dependency, R2 pin or API shape change.

### UI PROCESS — MOBBIN → EAGLE

Recovery logic only. Existing signed module references remain applicable;
no new reference used, image copied, Eagle read/import or Mobbin query required.

### UI PROCESS — FIGMA OFFICIAL GUIDANCE

No Figma, component layout, variable or prototype change. Existing guide,
three-language copy and desktop/mobile fact-parity tables remain applicable;
no new visual lint/density or Figma execution claimed. No locale string changed.

Scope: SettingsV2.tsx, settings-v2.ts and this card's delivery/red evidence.
No new import, renamed spec, route or page; no harness allowlist/dependency edge
change. Browser ownership is regenerated last and checked, never hand-edited.
No new outside-card path. P2 for issue: existing profile CAS recovery deferral.
Local node --check settings-v2.ts and git diff --check exit 0 (E1 syntax only).
tsc NOT_RUN: no compiler/dependencies in this worktree; no install attempted.
Own-head GREEN and independent E4 pending; author judgment E0.
NOT_RUN: local tests/servers/browsers/PG (prohibited), provider SANDBOX/LIVE,
production/deploy. No local process or fixture started; all pre-existing evidence
retained. Rollback: revert this implementation commit on the assigned branch.
Assigned Codex UI implementer; runtime exact model/effort selector unavailable.
No delegation/separate claim; integrator retains ownership. Humaux fix stored
and own child canvas updated; code map/search transport errors, rg used instead.
Integrator completion/failure relay owns wakeup after exact-head WAIT_RUN handoff.
No merge/auto-merge/status/deploy; no evidence-only commit after GREEN.

## FIX12 RELAY 1 — restore the B observer page (2026-10-30)

Base: 20093dcdffed6adb79c8d74e169f9a8a3e660830. Current-head original
GitHub receipts: Foundation40978806864/job123813377549 success;
Gates40978806851/bundle:07 job123814651346 failure. Artifact12943429503
shows settings-v2.spec.ts:421 expected B's product-form after a reload of
Settings. The native discovery observation changed that tab's URL to Settings.
All preceding A-bound recovery, corruption rejection and exact-replay assertions
were reached successfully in this failing run; whole-PR GREEN remains PENDING.

Only add navigation back to B after observing native discovery. Keep the later
native reload and every assertion, fixture, fault, wait, threshold and timeout
verbatim. No product implementation, import, route or locale change. This is the
second targeted FIX12 push after the initial implementation; if still blocked,
stop and relay rather than repeatedly retry. Original causal RED remains
40967861632/job123778347162/artifact12940365670 at tests-only6fdba1859.

|Page/control|Operation|Expected|Actual|
|---|---|---|---|
|Settings / pending SKU and stock Reload|native clicks|recover A despite latest B; refuse corrupt evidence|preceding assertions reached in current-head CI; whole run failed later|
|Settings / explicit Save|native clicks|original SKU/inventory bytes and keys target A|preceding assertions reached in current-head CI; whole run failed later|
|Catalog B / refresh|native navigation then reload|B name/version/SKU/inventory unchanged|observer navigation corrected; fresh CI pending|

### UI PROCESS — BOURDIEU

Q1 safely resume stock; Q2 novice/expert means recent task experience;
Q3 competing catalog activity must not change intent ownership; Q4 explicit
Save alone writes; Q5 no class/device inference; Q6 aesthetics UNKNOWN.

### UI PROCESS — PROJECT UNDERSTANDING

Architecture sections 0/1/2/26/27 and merchant-settings-wizard-v1 session/drafts/
uncertainty require truthful evidence, PG authority, scoped exact replay and
independent acceptance. Current source plus native GitHub failure is authority.
This correction only restores the observer's URL before its persistence check.

### UI PROCESS — MOBBIN → EAGLE

No design change or new reference; existing signed module references unchanged.
No new Mobbin query or Eagle import needed for a test navigation correction.

### UI PROCESS — FIGMA OFFICIAL GUIDANCE

No canvas, layout, component, variable or prototype change. Existing official
guide application, three-language copy and field-parity tables remain applicable.
No visual/density/Figma acceptance claimed.

No new outside-card path. Static syntax and generated ownership checks only;
current-head GREEN and E4 remain pending. Author judgment E0. NOT_RUN: local
tests/servers/browsers/PG (prohibited), provider SANDBOX/LIVE/production/deploy.
No process or fixture started. Downloaded CI failure evidence retained locally.
Rollback: revert this isolated test-navigation commit. Integrator owns CI relay.
Static verification: node --check tests/admin/settings-v2.spec.ts exit 0;
git diff --check exit 0; derive-browser-ownership.mjs and --check exit 0
(53 modes). Only generated source hash changed. No import/harness/dependency
edge change, so no depmap regeneration required. E1 only; CI PENDING.

## RVFIX product ACK / detail-read failure — tests-only RED (2026-10-30)

Authority: final REQUEST_CHANGES in `output/integrator/tools/logs/cxf-rv-303.log`
at aa1594c499772a8d146aa2a3bea494894f164be4. One P1: retain acknowledged product
identity and unsent price/stock before its fallible detail read. No P0/additional
P2. This push changes no product source. Integration a09262050 was merged first
as 48fb74490; generated ownership conflict resolved with the repo generator.

New native-click witness: actual product POST receipt, first detail GET MOCK 503,
zero SKU/stock commands; another tab uses Catalog Copy on the SKU-less product,
and observed Settings discovery proves that competitor is now newest. Reload
and refresh must preserve the original product/1280/10 without writes. Explicit
Save must resume its original ID with no duplicate product. Existing SKU/stock
fault, exact-replay and corruption assertions remain in order. Final native
refresh checks both competitors' persisted name/version/SKU facts unchanged;
native archive retires only CI-created competitors. No budgets/gates weakened.

|Page/control|Operation|Expected|Actual|
|---|---|---|---|
|Setup / Save|native click; product ACK then detail 503|unavailable, zero SKU/stock writes|PENDING CI RED|
|Catalog / Copy; Settings discovery|native control and read observation|competitor is newest|PENDING CI RED|
|Reload / refresh / setup Next|native clicks|original name/1280/10, no writes|PENDING CI RED|
|Explicit Save / Catalog refresh|native controls and readback|only original completed; competitors unchanged|PENDING implementation CI|

### UI PROCESS — BOURDIEU

Q1 resume setup without losing unsent fields; Q2 novice/expert means recent task
experience; Q3 concurrent catalog work cannot redirect intent; Q4 explicit Save
alone writes; Q5 no class/device inference; Q6 aesthetics UNKNOWN.

### UI PROCESS — PROJECT UNDERSTANDING

Read architecture 0/1/2/26/27: truthful evidence, PG authority, isolated ownership,
independent acceptance. Merchant-settings-wizard-v1 Session/drafts/uncertainty
requires scoped durable nonsecret intent and no read-side writes; AD-03 keeps
three steps. Merchant-settings-http-v1 preserves scope/CAS; catalog-inventory-v1
preserves explicit commands and ledger stock. Current product-ACK/read boundary
is the sole target. No new setting, visible string, locale or layout change.

### UI PROCESS — MOBBIN → EAGLE

No new visual reference/query/import needed for this recovery regression;
existing module references unchanged.

### UI PROCESS — FIGMA OFFICIAL GUIDANCE

No canvas/component/variable/prototype changes; existing guide application,
three-language and desktop/mobile parity tables remain applicable. No new
visual acceptance claimed.

E1: node --check and git diff --check exit 0. Ownership regeneration last;
no imports/dependency edges changed, so depmap regeneration unnecessary. RED
and current-head GREEN PENDING; author E0. No new outside-card path. NOT_RUN:
local tests/servers/browser/PG (prohibited), E4, provider SANDBOX/LIVE/production.
No local process/fixture started; pre-existing evidence untouched. Rollback:
revert isolated regression commit. Integrator owns CI completion/failure/timeout
relay: Foundation 90m, Gates bundle 360m/member 150m. Assigned Codex UI author,
exact runtime model/effort selector unavailable. No delegation/separate claim;
parent untouched. Humaux problem stored; child canvas updated at
integrator-sub-settings-v2-postcreate303. Implementation awaits causal CI RED.

## FIX13 — repair Catalog Copy setup only (2026-10-30)

Authority: integrator's FIX13 ruling in the current dispatch; current
ProductList.tsx selection/Advanced/Copy controls. Prior head fee5d8ff0; pull
--ff-only was already current. Fetched origin/r3/integration is already an
ancestor (HEAD...origin/r3/integration = 31/0), so no merge was needed.
Rollback: revert this isolated tests-only setup commit.

Gates 40984910325 / gate (bundle:07) job 123834779971 failed at the removed
row-level Copy control, per FIX13 relay; GitHub confirms failure at fee5d8ff0.
That run is a setup failure, not causal RED. Both the new post-create scenario
and earlier SKU recovery scenario now click only the target row's checkbox,
open native Advanced and click products-copy-selected. Their existing POST
predicates, both discovery preconditions and every behavioural assertion stay
verbatim and in order. No fixture, implementation, import or dependency edge
changed; depmap regeneration is unnecessary. Browser ownership is regenerated
last. Outside-card paths: none; generated ownership is expressly authorized.

|Page/control|Operation|Expected|Actual|
|---|---|---|---|
|Catalog / acknowledged product row|checkbox click|only that row selected|PENDING CI|
|Catalog / Advanced then Copy|native clicks|same product copy POST receipt|PENDING CI|
|Catalog / earlier SKU recovery row|same native selection/Copy path|real newer draft B|PENDING CI|

### UI PROCESS — BOURDIEU

Existing Q1-Q6 verdicts remain applicable; task experience only, aesthetics
UNKNOWN. This change restores the real merchant action used by the test.

### UI PROCESS — PROJECT UNDERSTANDING

Read AGENTS.md and architecture 0/1/2/26/27: PG remains authoritative, UI writes
use real clicks, acceptance stays CI-only and independent. FIX13 limits this
phase to setup repair; product recovery implementation awaits causal CI RED.

### UI PROCESS — MOBBIN → EAGLE

No visual change or new pattern; no reference import/query needed.

### UI PROCESS — FIGMA OFFICIAL GUIDANCE

No design/component/prototype write. Existing guide, required-field parity and
three-language copy tables remain applicable; no new visible product string.

Current evidence: E0 authored repair; E1 syntax/ownership results recorded in
PR handoff after static checks. CI RED/GREEN and E4 remain PENDING. NOT_RUN:
local tests/servers/browser/PG (prohibited), provider SANDBOX/LIVE/deploy.
No process/fixture started; pre-existing untracked evidence retained. Humaux
tools are unavailable in this session: no store/canvas write claimed. Assigned
worktree/branch only, no delegation. Integrator relays CI conclusion; existing
Foundation 90m / Gates bundle 360m / member 150m limits remain unchanged.
## FIX13 relay — native observer editor setup (2026-10-30)

Authority: current integrator relay for PR #303 and original GitHub receipts,
not historical delivery claims. Pulled unit/settings-v2-ui --ff-only: current at
eb9d94fd43eb4dab975f7f94218b8c98ab554741. Fresh origin/r3/integration
a09262050d750895347e7333b8228194fec00cb2 is already an ancestor (32/0), so no
merge is necessary. Assigned worktree only; no delegation or separate claim.
Humaux tools are unavailable: no shared memory/canvas write claimed.

### Current receipt and minimal change

Gates 40988280532 / bundle:07 job 123845389234 fails at settings-v2.spec.ts:219,
waiting for product-form, after the native selection/Copy succeeds. GitHub job
API binds the failure to eb9d94fd; fresh artifact 12949125368 confirms it.
Foundation 40988280632 / job 123844307626 succeeded at that head only.
This is another setup failure, NOT the new recovery assertion's causal RED.
ProductEditor.tsx renders CatalogSimplifiedEditor by default; its existing
catalog-advanced-toggle selects ProductDocumentForm and persists the choice
in that tab's sessionStorage. Each competitor observer now clicks that native
control once. No product code, assertion, fixture, timeout, gate or exemption
changes. Both discovery preconditions remain verbatim/in order. Recovery
implementation remains blocked on causal RED. Outside-card paths: none.

|Page/control|Operation|Expected|Actual|
|---|---|---|---|
|Post-create competing product / Advanced|native click|existing product-form becomes visible|PENDING CI|
|Earlier SKU-recovery competitor / Advanced|native click|same persisted editor for baseline and refresh|PENDING CI|
|Settings / Reload and refresh|unchanged native actions/assertions|original identity, price and stock; no implicit writes|PENDING causal RED|

### UI PROCESS — BOURDIEU

Q1 recover unsent setup safely; Q2 experience means recent task experience;
Q3 concurrent catalog work cannot redirect intent; Q4 explicit Save alone
writes; Q5 no class/device inference; Q6 aesthetics UNKNOWN.

### UI PROCESS — PROJECT UNDERSTANDING

Read architecture 0/1/2/26/27, AGENTS.md and merchant-settings-wizard-v1
Session/drafts/uncertainty: PG authority, scoped nonsecret recovery, native
clicks and independent CI acceptance. Current ProductEditor.tsx and the raw
GitHub receipt explain this setup correction; no new seller setting is added.

### UI PROCESS — MOBBIN → EAGLE

No visual design or new pattern. Existing module references remain applicable;
no new Mobbin query or reference import is required for observer navigation.

### UI PROCESS — FIGMA OFFICIAL GUIDANCE

No Figma/layout/component/variable/prototype changes. Existing guidance,
required-field parity and three-language tables remain applicable; this patch
adds no visible product strings and claims no visual acceptance.

E1: node --check tests/admin/settings-v2.spec.ts and git diff --check exit 0.
Browser ownership regenerated last and checked before commit; no imports or
dependency edges change, so depmap regeneration is unnecessary. Author judgment
E0; new-head CI, causal RED, GREEN and independent E4 remain PENDING. NOT_RUN:
local tests/servers/browsers/PG (prohibited), provider SANDBOX/LIVE/deploy.
Rollback: revert this isolated tests-only commit. No local process or fixture
started. Existing untracked evidence retained. Integrator relays CI completion;
Foundation 90m / Gates bundle 360m / member 150m remain workflow timeouts.
WAIT_RUN IDs will be posted in the PR comment without an evidence-only push.

## FIX13 relay 2 — persist recovery before the first product read (2026-10-30)

Authority: integrator FIX13 ruling in the current task and raw GitHub run
40990412840/job 123852623131 at head a91ae866715ed75a848fb2212dc66277f4b7db07.
Fresh browser artifact 12949522944 proves settings-v2.spec.ts:239 fails the new
identity assertion: expected Acceptance draft bag, received its newer Copy.
Both discovery/setup preconditions passed. This is causal RED (E3 failure
evidence), not completion. Full receipt/hash and provenance are in red.log.

The existing execute() validates the product receipt and clears its command
journal. Previously the first detail GET could fail before initial-stock was
persisted. SettingsV2.save now persists the same scoped initial-stock record
and unsent draft at the current profile version immediately after the receipt,
before that GET. Existing load validation then pins the original product and
retains price/stock through Reload/refresh; explicit Save alone resumes writes.
The SKU path reuses this persistence function. No new storage format, import,
dependency, setting, visible copy or behavioural assertion. Tests remain exactly
at the causal-RED version; no fixture/wait/threshold/gate changes.

|Page/control|Operation|Expected|Actual|
|---|---|---|---|
|Catalog competitor creation|native row checkbox, Advanced, Copy; native editor choice|newer draft leads discovery|passed before CI assertion at a91ae866|
|Settings Reload + refresh + quick setup|existing real clicks and navigation|original product, price 1280, stock 10; no writes|RED at a91ae866; implementation CI PENDING|
|Settings Review + Save|existing real clicks|resume original product, no duplicate create; exact SKU/stock replay|PENDING implementation CI|
|Both competitor editors|native refresh and archive|unchanged baseline before fixture retirement|PENDING implementation CI|

### UI PROCESS — BOURDIEU

Q1 safely resume unsent setup; Q2 novice/expert means recent task experience;
Q3 another operator's newer draft cannot redirect intent; Q4 explicit Save alone
writes; Q5 no class/device inference; Q6 aesthetics separate and UNKNOWN.

### UI PROCESS — PROJECT UNDERSTANDING

Read AGENTS.md, architecture 0/1/2/26/27 and merchant-settings-wizard-v1
Session/drafts/uncertainty plus Quick start/acceptance. They require scoped
nonsecret journals, PG authority, no automatic writes/retries, native clicks,
and independent CI acceptance. SettingsV2 load/save and settings-v2
execute/readSnapshot are the direct implementation authority. Humaux tools
are unavailable: no rejected/lesson lookup, store, code index/link or child
canvas update is claimed. No separate claim or delegation from this lane.

### UI PROCESS — MOBBIN → EAGLE

No visual redesign or new interaction pattern; existing module references
remain applicable. No reference import or additional Mobbin query required.

### UI PROCESS — FIGMA OFFICIAL GUIDANCE

No Figma/layout/components/variables/prototype write. Existing guide, desktop/
mobile required-field parity and three-locale tables remain applicable; no
new visible strings. No new visual acceptance is claimed.

Static checks: node --check tests/admin/settings-v2.spec.ts and git diff
--check exit 0. Local tsc NOT_RUN: this worktree has no installed node_modules;
no dependency install performed. Browser ownership is regenerated/checked last
before commit. No dependency/import edges changed, so depmap is unchanged and
no component harness allowlist update is needed. Author assessment E0;
implementation GREEN/E4 PENDING. NOT_RUN: all local runtime tests/servers/
browsers/PG, independent final review, provider SANDBOX/LIVE/production/deploy.
Outside-card paths in this relay: none. Earlier PR-wide exceptions remain in
the PR body for integrator approval. Rollback: revert the isolated recovery
fix commit. No local process/fixture started. Pre-existing evidence retained;
downloaded failure evidence retained for review. Integrator owns CI completion
relay with existing Foundation 90m / bundle 360m / member 150m workflow limits;
fresh WAIT_RUN IDs go in the PR comment without an evidence-only push.
