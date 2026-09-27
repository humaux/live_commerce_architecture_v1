# Media login custody acceptance — 2026-09-27

Status: **PASS_LOCAL_MOCK_LOGIN_CUSTODY — MLC01–06**.
Frozen contract: [MLC v1](../../contracts/live-media-login-custody-v1.md),
`5db67f5`; independent contract review at `3cbdaea` found no remaining confirmed
P0/P1 after fixing replay's missing revision GUC. Source acceptance is separate.

## Scope and ownership

This binds the existing MOCK media Start to its exact initiating merchant login,
so pre-wire logout prevents Start and post-wire logout retains cleanup authority.
It does not issue browser credentials, grant LIVE access, or implement input
cleanup after Egress terminal. BRI01–07 and overall T08/G06 remain unaccepted.

|Role|Worktree/branch|Allowed writes|
|---|---|---|
|source `/root/media_runtime_source`|`/Volumes/data/worktrees/commerce-meta-inbox-go-20260926`; `commerce/media-login-source-20260927`|`migrations/0039_live_media_login_custody.sql`, `internal/live/media_plan.go`|
|independent tests `/root/media_runtime_tests`|`/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926`; `commerce/media-login-tests-20260927`|New MLC tests; historical-0035 seed helper in `live_media_plan_test.go`; only historical setup call sites in `live_media_execution_faults_test.go` and `live_media_stop_test.go`|
|fixed-source reviewer `/root/livekit_protocol_impl`|Read-only main/candidate commits|None|
|integrator `/root`|Main checkout|Final migration merge, docs, independent regression|

Both writers start at `5db67f5`. Existing idle agents/worktrees are reused;
their current runtime model/effort is not exposed by the tools. No new override
or recursive delegation. Maximum two simultaneous source/test writers.

## Required evidence

Run independent MLC cases against actual isolated PG18/roles. Prefix test names
`TestLiveMediaExecutionMLC` so the existing `--live-media-stop` selector includes
them, avoiding another test runner. Also replay native media-process and Studio
tests for source/replay compatibility. Root must independently rerun fixed code.

Do not replace actual login IDs with only a principal, label a fabricated old
row as an upgraded real identity, or weaken pre-existing queue/authority gates.
Existing permission grants do not automatically increment authz revision;
transient revoke-and-regrant without such an increment is an explicit remaining
permission-management prerequisite, not an accepted revocation guarantee.

Source/test SHAs, measured results, failures, cleanup and accepted scope will be
filled only from executed evidence. No customer service or provider write is
authorized by this record.

## Implementation and dependency map

Source candidate `4640887`, targeted correction `70f57b1`; integrated as
`b515c6b` and `da4b709`. No new package, provider SDK, configuration or queue.
Independent test commits `278792f`, `0a06cc1`, `683f9d3` are integrated as
`23577f8`, `5839fce`, `9e30686`. Root's full PG/race/vet run uses `9e30686`.

- `MediaPlanner.PlanStart` still uses `command.Run` and the existing River
  transaction. It sets the authorization revision before the command receipt
  lookup, then calls `live.assert_media_start_login` even on receipt replay.
- Forward migration `0039` creates the private immutable custody child. Its
  identity-owned locking helper reuses existing identity-writer privileges;
  the media writer does not acquire session UPDATE permission.
- Existing dispatch and lifetime predicates call `live.media_login_eligible`.
  The original attempt, operation and job remain the recovery authority after
  wire reservation. No input admission or input-cleanup state is added here.
- New Go requires migration `0039`: a pre-0039 database must fail closed.
  Never add a missing-function fallback that skips the replay guard. Historical
  upgrade tests seed with a test-only frozen old-schema path, not current Go;
  this is not an old-binary compatibility claim.

## Preserved failed evidence

The first independent actual-PG run is
`/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926/output/mlc-stop-first-20260927.log`
(exit1; SHA256 `593ec58997e0f5cac87257f682019b26f6b6c2e3e781667176e7e99716b52350`).
Root and independent review found the missing finite-expiry database constraint
and helper check in `4640887`; `70f57b1` adds both and restores the authorization
lookup's `FOUND` check before the new login query can overwrite it. The test run
also exposed new-fixture mistakes and historical setup using new Go against old
SQL. Corrections must retain old assertions and the production fail-closed gate.
This log is preserved, not replaced by later passing output.

The second independent PG run, `output/mlc-stop-second-20260927.log` in the
same test worktree, exited1 (SHA256
`da667189fe9a1d3771ef45c939d598780ee2a9091a75a77da54fe51efc4f16af`).
Historical upgrades and MLC02/03/04 passed. Remaining test mistakes were an
invalid expectation that a test helper clears its already-returned callback
value after a COMMIT-ACK loss, and resolving a private function name under a
role without schema USAGE. Keep the commit error, persisted facts, exact replay
identity and real-role EXECUTE denial assertions; do not treat arbitrary SQL
errors as proof of denied EXECUTE.

## Root partial regression at `da4b709`

All three commands below exited0; this is not yet aggregate MLC acceptance.

|Command|Measured result|Log SHA256 under `/Volumes/data/output/`|
|---|---|---|
|`bash scripts/dev/test-local.sh --live-media-runtime`|8 top-level PASS, 0 FAIL/SKIP|`mlc-root-runtime-20260927.log`: `55f0a88be923aecad967fc3c6f582798dd398f678def7a6e4f51adfa35febe80`|
|`bash scripts/dev/test-local.sh --studio-backend`|8 top-level PASS, 0 FAIL/SKIP|`mlc-root-studio-backend-20260927.log`: `02b66950c9e5ad72f224fec1059df4ec89a12bacdb83ea13561051087fe0725c`|
|`bash scripts/dev/test-local.sh --browser-studio-bff`|2 raw-grammar tests and 1 real Chromium/Next/Go/PG chain; 0 FAIL/SKIP|`mlc-root-studio-bff-20260927.log`: `452c9bda482872eb6d64088a13dc30e1edd5ad766886b5642ba3c8872571f288`|

Browser evidence: `output/playwright/studio-bff-20260927T051044.725908000/`.
The fixed-source reviewer rechecked `70f57b1` and found both reported gaps closed,
with no new confirmed P0/P1. This is source review, not PG or Cloud evidence.

## Independent final PG evidence

Fixed test SHA `683f9d3`, source `4640887` + `70f57b1` (test-tree cherry-picks
`1ae84b3` + `4fce6b9`). `bash scripts/dev/test-local.sh --live-media-stop` exited0:
**47 top-level PASS, 0 FAIL, 0 SKIP**, including **5 MLC tests / 23 subcases**;
foundation package time 221.020s. This selector does not include every package
or the browser chain; counts are not added to overlapping Root results.

Log: `/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926/output/mlc-stop-final-20260927.log`;
SHA256 `d455bec00d07b9224fe3424c022e2dfbcd36b8c8e27b6149862ea16244d6c0be`.
Independent `go vet ./tests/foundation` also exited0. The final run preserved
both previous failing logs, used actual PG18 role logins, and removed its owned
container `lc-foundation-test-70889` on exit. No customer/provider resource used.

## Root final acceptance at `9e30686`

`bash scripts/dev/test-local.sh` exited0: **663 top-level PASS, 0 FAIL, 0 SKIP**
across **33 tested packages**, with actual isolated PG18 and race detection.
Foundation package time: **688.760s**. The runner's subsequent `go vet ./...`
also succeeded. No SQL, lease, pacing or individual fault timeout was weakened.

Log: `/Volumes/data/output/mlc-root-full-20260927.log`;
SHA256 `589effa80d8271622db780917454287c6861cddb58a6e32ab5c98b32735c49cf`.
This contains the 5 MLC top-level tests; they are not extra tests to add to663.
The runtime/Studio/BFF runs above used identical production source at `da4b709`;
later commits only added tests and documentation.

|Gate|Result and bounded evidence|
|---|---|
|MLC01|PASS: exact real-login custody, rollback, concurrent same-key replay, real lost COMMIT ACK, body conflict, finite-expiry counterexample|
|MLC02|PASS: same-principal other-login denial, unrelated logout isolation, original logout/expiry/revision/grant loss, explicit revision-after-regrant denial, independent authorized Stop|
|MLC03|PASS: observed binding and exact-login row waits, fresh post-wait logout/expiry/revision checks, rejection both before claim and after claim/load at final reservation|
|MLC04|PASS_LOCAL_TLS_MOCK: reserved logout/expiry/store-read/revision loss and unbound historical-state simulation preserve exact observations and original job/operation for bounded Stop; ambiguous resource stays UNKNOWN|
|MLC05|PASS: private table denial with42501, actual-role function-OID EXECUTE denial, fixed definer owner/path review, cross-store/malformed/unbound guards, unchanged safe BFF projection; populated old-schema upgrades retain checksums/job/state|
|MLC06|PASS_BOUNDED_LOCAL: independent fixed-source review, independent47-test PG/race run, Root663-test PG/race/vet run, runtime8, Studio8 and real browser BFF chain|

The unbound-reserved cleanup case deliberately removes a child as the test
owner; it is not labeled as an old executable. Separate historical upgrade
fixtures genuinely apply0039 over populated old SQL and check unchanged records.
The final root postflight found container `lc-foundation-test-70926` and shell/Go
PIDs70926/70946 absent. Other tasks' containers and original evidence remain.

Humaux review: `dd33a392-b264-4bca-a598-3c4398f804d9`; independent PG receipt:
`5de1f54e-81f1-41f7-b4f6-9557b72968e9`; source repair:
`4fa3b506-28cd-40f6-a949-e567379dc16c`. Go/source tests indexed; SQL is not an
indexed code-graph entity, so this document is its maintained decision pointer.

Still **NOT_RUN**: publisher token admission, input observation/cleanup through
Egress terminal, full browser/SFU product chain, Cloud cached-token revocation,
approved Studio UI and G06. Orders C's MOU03 native restoration gate is unrelated
and remains unresolved. No customer broadcast, payment or production deployment
was touched. Production permission mutation must advance authorization revision;
grant removal/regrant without a revision bump is not a guaranteed detected event.
