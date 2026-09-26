# T08 LMA01–05: prepared media authority registry

Status: **PASS_INDEPENDENT_LOCAL_PG_AND_ROOT_FULL_REGRESSION** at main `6722915`.
This is MOCK-only authorization persistence, not an executable media controller.
No customer database, provider account, stream, credential or production service
was changed. Full MLA01–08, LIVE intake, Cloud/media, studio browser, G06 and
deployment remain **NOT_RUN**.

## Implementation and dependencies

[Frozen contract](../../contracts/live-media-authorization-v1.md): `a00fa25`,
mixed-role admission addendum `67ae261`, catalog-lookup correction `a542e5c`.
Migration `0034_live_media_authorization.sql` creates typed immutable authority,
ordered destination and append-only revocation tables. It reuses existing draft
sessions/programs, scoped integration bindings and PostgreSQL RLS/row locks.
Two fixed private-writer functions admit a separately provisioned registrar;
ordinary merchant/worker/Meta/payment roles get no secret-table access.

Registration validates closed JSONB values, scope, active bindings and exact
versions, DRAFT layout, deadline/caps and envelope shape. It locks bindings in
UUID order before tenant/store, session/program and authority. Exact normalized
replay is idempotent; changed data cannot overwrite the record. Revocation cannot
be undone by replay and remains available after references are disabled/expired.
No operations, jobs, HTTP endpoint, worker or provider side effects are created.

The existing `platform.validatePoolAuthority` also rejects mixed registrar/writer
membership and direct/PUBLIC/SET-reachable EXECUTE on the two fixed functions.
Catalog OIDs are resolved by exact namespace/name/signature without requiring an
ordinary worker to acquire access to the `live` schema. No new dependency, SDK,
queue or generic authorization framework was introduced (ponytail reuse).

SQL verifies the envelope's shape, **not** its authentication or destination
ownership. LKM must still open it against trusted scope at the future dispatch
gate. An evidence hash is not real Meta eligibility, and MOCK cannot become LIVE
by changing a runtime flag.

## Ownership and revisions

|Role|Actual model/effort|Base, commits and exclusive write paths|
|---|---|---|
|Source integration worker|gpt-6-sol / medium|base `a00fa25`; addendum cherry-pick `bf4764f`; `9754bde`, repair1 `bba6f3a`, repair2 `607f710`; only migration0034 and `internal/platform/platform.go`|
|Independent test worker|gpt-6-sol / high|base `a00fa25`, addendum `36fc2d5`; test commits `a504dc3`, fixture correction `c79d6a1`, causal additions `79796e5`; only `tests/foundation/live_media_authorization_test.go`; final test worktree `59adbb4` includes source|
|Independent reviewer|Inherited model/effort not exposed|Read-only contract/source/repair and gate coverage review; no code/test edits|
|Root integrator|Inherited root session|main source `19a85d5`, `25b0ede`, `3bece73`; tests `f3fd28e`, `4599efc`, `6722915`; runner/contracts/docs and independent full regression|

Reused worktrees: `/Volumes/data/worktrees/commerce-meta-inbox-go-20260926`
(`commerce/media-authority-sql-20260927`) and
`/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926`
(`commerce/media-authority-tests-20260927`). No recursive delegation.

## Gates and retained failures

- LMA01: exact persisted scope/order/envelope/provenance, one/two destinations,
  canonical integral-decimal replay, rollback and no partial invalid children.
- LMA02: actual separate LOGIN roles, schema/function/table/column privileges,
  private RLS, mixed-role and direct/PUBLIC/SET-only authority negatives. Clean
  runtime/worker startup before hostile grants and after revocation prevents a
  vacuous negative gate.
- LMA03: invalid environment/evidence/fields/types/scope/bindings/versions/layout/
  time/caps/envelope, static errors without input echo and zero writes.
- LMA04: exact concurrent replay, changed-envelope rejection, observed native
  row waits with stale binding/deadline rejection. A fixture-only trigger holds
  the final child write on an advisory lock; after observing the real wait and
  DB deadline expiry, release leads to rejection and zero header/child rows.
- LMA05: scoped append-only/idempotent revoke, mismatches and changed reasons
  rejected, replay cannot revive, expired/disabled references remain revocable.

First independent run exposed legal `1.0` JSONB values rejected by later raw-text
integer casts. Source now casts the already validated numeric value; tests retain
integral-decimal normalization. Review also required the final deadline check
after child work, proven by the observed advisory-wait gate.

The first LMA02 fixture assertion counted superuser as MEMBER of every role.
Only that fixture assertion was corrected to exclude privileged owner; ordinary
role negatives were retained. Repair1 then exposed a **source** error: clean
workers were rejected because `to_regprocedure('live...')` itself requires schema
USAGE. Repair2 uses exact catalog OIDs, with no new grants or relaxed assertion.
Both red logs remain available. No test was deleted or success threshold lowered.

|Executor/command|Result|Log under `/Volumes/data/output/`|
|---|---|---|
|Author final `bash scripts/dev/test-local.sh --live-planning`|0; migration + prior LSP smoke, not LMA behavior|`lma-author-repair2-live-planning-20260927.log`|
|Author `go test ./internal/platform`; `go vet ./internal/platform`|0 / 0|Author tool transcript; no separate stdout artifact claimed|
|Independent `bash scripts/dev/test-local.sh --live-authority`, original|1; numeric source failure + fixture assertion|`live-media-authorization-independent-20260927-first.log`|
|Independent same command, repair1|1; clean-worker startup regression caught|`live-media-authorization-independent-20260927-repair1.log`|
|Independent same command, repair2|0; LMA01–05 + LSP01–05, 10 top-level PASS, 0 FAIL/SKIP|`live-media-authorization-independent-20260927-repair2.log`|
|Independent `GOTOOLCHAIN=go1.27.1 go vet ./tests/foundation`|0|Independent tool transcript|
|Root `bash scripts/dev/test-local.sh` at `6722915`|0; isolated PG18, 603 top-level PASS, 0 FAIL/SKIP in 31 test packages; full Go race and vet pass; foundation 435.225s|`live-media-authorization-root-full-20260927.log`|

Independent focused foundation duration: 15.624s. Root terminal session `23678`
returned exit 0; the runner's final PASS follows `go vet ./...`. The temporary
fixture is absent from Docker after exit. No existing service was stopped.

SHA-256 of immutable acceptance logs:

- Independent repair2: `63a673b9d22408165719fe397bc5605cce1d854144a798985ab1b75078ed08bc`.
- Root full: `7cda80d6e364bda7490d4eb1a2b00b2db9fbfe8e6cccdcf881d92f2b3f6d9502`.

These are backend gates; browser-tagged tests, provider qualification and media
quality gates are not included in the 603 count and are not claimed as passed.

## Remaining boundary

The next controller increment needs the MEDIA_ATTEMPT actor/composite FK,
ordinary-worker operation/event isolation, fenced material/observation functions,
exact same-transaction River lane and Go lifecycle. Bounded same-ID Stop recovery
is reviewed design only. Existing client already uses `StartEgress`; an earlier
API-migration blocker was incorrect and superseded in Humaux `992c3a9c`.

Source receipt `292cbc61-9d22-460c-9bca-9c41a42dbdbe`; independent contract
preflight `a6261a22-8cba-4d59-9772-e056a09a4530`; mixed registrar fix direction
`74a3047a-1cdb-44a8-badd-dffabdc04716`; independent test receipt
`2fe666d0-cbb3-4848-b2e8-d53ebe3210bd`; final independent source review
`f97a7749-3a07-4fe7-82fc-98479a8f6233` (no remaining P0/P1 within this slice,
reviewed before root full finished). Task-owned fixtures were removed by their
runners; worktrees and failure evidence are retained. Full T08 remains IN_PROGRESS.
