# T08 live planning: LSP01–05 local acceptance

Scope: a merchant-scoped, durable **DRAFT** session/program pair; not live media.
Contract: [live-planning-v1](../../contracts/live-planning-v1.md). No HTTP route,
studio UI, LiveKit/Meta network request, stream key, destination, MediaAttempt,
River job or public deployment is introduced. G06 and complete T08 remain open.

## Source and independent roles

Base `f4b3c00`; frozen contract `fe98daf`; migration `c369195`; focused runner
`6356fec`; provider precheck `5f54a84`; Go author `4bbe5e1` integrated as `b4ee6cf`;
test author `6aa1f95`/`8e1d067` integrated as `41ca817`/`fa528a2`.

|Role|Model/effort|Worktree and write boundary|
|---|---|---|
|Integrator|inherited root session|main repository: migration 0033, contract, runner and documentation|
|Go author|gpt-6-sol/high|`/Volumes/data/worktrees/commerce-meta-inbox-go-20260926`, branch `commerce/live-draft-go-20260927`, `internal/live/draft.go` only|
|Independent test author|gpt-6-sol/high|`/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926`, branch `commerce/live-draft-tests-20260927`, `tests/foundation/live_planning_test.go` only|
|Independent reviewer|gpt-6-sol/high|read-only contract, migration, final source/tests and evidence|

The root reread source and test deltas, then reran the focused PG gate separately.
No recursive delegation or new dependency. See the
[dependency/call trace](dependencies.md#t08-场次节目草稿调用链).

## Deterministic gates

|Gate|Verified local behavior|Evidence|
|---|---|---|
|LSP01|Atomic pair/audit/receipt, canonical UTC microseconds, joined read, stable IDs and rollback of create+update|`TestLivePlanningLSP01AtomicPairAndSchedule`|
|LSP02|Concurrent same-key dedup, changed payload/principal conflict, frozen historical replay, competing-version CAS|`TestLivePlanningLSP02ReplayAndCAS`|
|LSP03|Read/manage separation, denied grants, revoked/expired tokens, stale revision, GUC mismatch, valid-GUC repeatable-read rejection, cross-store/tenant isolation and read-only counts|`TestLivePlanningLSP03AuthorityIsolationAndReadOnly`|
|LSP04|Native observed row/advisory lock waits; DB clock expiry before release; rejected update/replay with unchanged title/aspect/state/version/update time and fact counts|`TestLivePlanningLSP04ObservedWaitExpiry`|
|LSP05|Title/UUID/date/aspect bounds, SQL constraints, immutable runtime columns, DRAFT-only state, no delete/public/buyer/worker privilege|`TestLivePlanningLSP05BoundariesConstraintsAndACL`|

Before acceptance, the original test candidate was strengthened: valid GUCs make
the isolation denial causal; expiry checks compare the five selected mutable fields, not just row
counts; separate read-only/manage-only positive and negative controls prevent
permission conflation. No application guard, production timeout or capacity was
relaxed. Initial and amended logs remain available below.

## Runs and evidence

- Independent exact test candidate `8e1d067`:
  `bash scripts/dev/test-local.sh --live-planning`, exit **0**, **5** top-level
  PASS, zero FAIL/SKIP, real isolated PG18 and Go race detector.
  `/Volumes/data/output/live-planning-test-worker-final-20260927.log`;
  SHA256 `a44d3b7d5d51143ff28d049ed311989ffe8a3a3c0ff48c5cf7c64517f0c047f2`.
- Root `fa528a2`: same command, exit **0**, **5** top-level PASS, zero FAIL/SKIP;
  foundation **6.292s**.
  `/Volumes/data/output/live-planning-root-focused-20260927.log`;
  SHA256 `ce053271ea96338d217d2d75c79158d5b7f32c09c47d7f34134f600d3a61aae3`.
- Root `go vet ./internal/live ./tests/foundation`: exit **0**.
- Original candidate and amended intermediate logs:
  `/Volumes/data/output/live-planning-test-worker-20260927.log` and
  `/Volumes/data/output/live-planning-test-worker-amended-20260927.log`.
- First existing full regression on `b4ee6cf` (before adding the new test file):
  exit **1**, **566 PASS / 2 FAIL**. The failures were historical migration-ledger
  tests treating latest as 0032: a global count of 37 and an exclusion list that
  incorrectly included legitimate new 0033 among historical rows. Original log:
  `/Volumes/data/output/live-planning-root-full-20260927.log`;
  SHA256 `2a5f76b5fb574749c04636e5aa3a5115d5ac9655f9c3c483987df4cb448f6488`.
- Test-only fixes `13e7631`/`bde39ae`, integrated as `39c996e`/`734f320`, preserve
  per-version historical checksum/row equality, explicitly require 0033 once,
  and check the full ledger is unchanged by a second Apply. Exact `post_river/`
  prefix matching avoids SQL LIKE's underscore wildcard. No application source,
  database capacity, deadline or business assertion changed.
- The first retry (`full2`) was intentionally interrupted before foundation while
  the exact-prefix correction arrived; its log is retained, **not acceptance**:
  `/Volumes/data/output/live-planning-root-full2-20260927.log`;
  SHA256 `651c89d921e0d572808c5e5957ff89fce5d4128a6fe94962164d5deb6b1b724f`.
- Final full regression on frozen `734f320`, now including LSP01–05 and both
  repaired historical gates: `bash scripts/dev/test-local.sh`, exit **0**,
  **573 top-level PASS / 0 FAIL / 0 SKIP**, 30 tested packages, foundation
  **451.166s**. `go test -p 1 -race -count=1 -timeout=600s -v ./...` and
  subsequent `go vet ./...` both exited 0; the existing envelope is unchanged.
  `/Volumes/data/output/live-planning-root-full3-20260927.log`;
  SHA256 `1f3c4c9955ed6b6d9ae6b673b9aac17aa2755d67d4b03db2ed71ae5a2962a332`.

Exact source hashes:

|File|SHA256|
|---|---|
|`internal/live/draft.go`|`14b1ef8b6afe291754add1d85e7f1250191adb936bf949877344aa872b64bcaf`|
|`migrations/0033_live_planning.sql`|`3a2e56c33d2ec6cacac0ed7e68bd8c81d203a090f65660e10c6f6c7d93432144`|
|`tests/foundation/live_planning_test.go`|`d8be1cc2deb3eaf2ce5629d0c0faa356751ad5b8ef11847d3733467a00889396`|
|`scripts/dev/test-local.sh`|`63ddc343e770d50f0e105b76e4ebe266d7ca5d3b456929f576bc26acdafdf6a0`|

## Boundary and next work

Only LOCAL draft planning is accepted. Live permissions are a vocabulary addition,
not automatic grants or a completed merchant authorization UI. No browser gate is
claimed because this increment adds no browser surface. Existing buyer B layout
and pending merchant-orders visual choice are unchanged.

Next T08 slice must specify destination ownership, media attempts and durable
start/stop/reconciliation before registering LiveKit routes. The current official
API precheck is retained in the contract; no real provider qualification is inferred
from local tests. Global G06, replay/recording/timeline, multi-destination monitoring,
customer production and full SaaS deployment remain **NOT_RUN / incomplete**.

Final independent verdict: **PASS for bounded LOCAL LSP01–05**, no open P0/P1/P2.
Reviewer independently checked final source/tests and the focused/full log hashes,
counts and seven named new/repaired gates. Humaux review:
`915001cd-3edc-4e9d-9c80-639da3d74335`, title
`T08 LSP01-05 final independent source and PG gate review at 734f320`.
Task-owned fixture cleanup was confirmed after the final script exited: no `livecommerce.fixture` containers
remain. No customer service or existing stream was stopped. Evidence logs and
committed worktrees are deliberately retained; the unrelated pending merchant
visual-choice server is not a disposable resource of this increment.
