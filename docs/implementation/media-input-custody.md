# Media input custody: dependencies and diagnosis

Scope: the local `LOCAL_SFU_MOCK_EGRESS` persistence kernel. It does not expose
publisher credentials, run an input-queue consumer, or prove Cloud cleanup.
Acceptance receipts are in [the dated record](2026-09-27-browser-input-runtime.md).
The [frozen contract](../../contracts/live-browser-input-runtime-v1.md) remains
the authority for admission, ownership and completion semantics.

## Call path and ownership

|Caller / source|Dependency|Responsibility|
|---|---|---|
|Trusted registrar|`live.register_media_input_profile`|Attach the one fixed local profile to an unused MOCK authorization while serialized against planning. No merchant-provided provider identity.|
|`MediaPlanner.PlanInputStart`, `internal/live/media_plan.go`|Existing `command.Run`, River `InsertTx`, `live.plan_media_input_start`|One caller transaction creates the original job, operation, attempt, exact-login MLC custody and input child. Never a second Stop ledger.|
|`MediaPlanner.ReserveInput`, `internal/live/media_input.go`|Existing authorization, command receipt, `live.reserve_media_input`|Reserve fixed nonsecret grant material. Recheck current admission and compare the same spec after both fresh execution and receipt replay. No JWT signing.|
|Media executor|`claim_media_input_operation`, `load_media_input_custody`, `close_media_input_admission`|Use the original operation generation, lease and key. A claim is not permission to dispatch Egress. Merchant logout denies new grants but does not remove executor cleanup custody.|
|Existing observation/uncertain-finish functions|Private `close_media_input_custody` and joint projection|Terminal Egress alone cannot complete an operation that still owns issued input. Both executor observation wrappers and uncertain-finish paths are tested.|
|Native River lifecycle / startup|Post-River guards and `media_input_plan_ready`; `internal/platform/media_runtime.go` role checks|Reject finalizing/removing open-liability jobs and detect incorrect linkage, grants, guards or function definitions.|

Storage dependencies are forward migration `0040_live_media_input_custody.sql`
and, after native River schema installation, `post_river/0009_live_media_input_custody.sql`.
The two children (`prepared_media_input_profiles`, `media_input_custody`) are
private, FORCE-RLS tables. Application roles use the fixed functions, not direct
table access. The runtime pool remains separate from worker/executor authority.

## Queue and state meaning

|Profile|Original native queue|Consumer in this increment|
|---|---|---|
|Historical `PROVIDER_MOCK`|`media_mock_v1`|Existing unchanged media worker|
|`LOCAL_SFU_MOCK_EGRESS`|`media_input_mock_v1`|None; token delivery also disabled|

Both queues use the existing `live_media_operation_v1` kind in `river_media`.
Their exact profile/queue pairing is enforced in SQL; the old worker and old
planner must not silently accept the input profile.

- `await_admission`: unissued eligible input; no new generation or active lease.
  A future consumer must snooze the same job, not acknowledge completion.
- `claimed` / `reconcile`: fenced cleanup/observation custody, not Egress dispatch.
- `held`: automation has no permitted next action. Original liability stays
  `UNKNOWN`; do not finalize, delete or re-create its job to clear the display.
- `terminal`: combined completion is proven. An unissued grant can close without
  provider proof; a reserved local grant cannot become `CLOSED` in this kernel.

Waiting consumes the immutable attempt-anchored lifetime. Reservation fixes the
publisher identity and bounded grant timestamps once. Replays never extend them.
The 4096-generation / 24-hour automation budgets do not erase unresolved input.

## First diagnosis checks

Use the actual intended role and scoped test environment when reproducing a
failure. `SELECT live.media_input_plan_ready()` is the bounded readiness check;
`true` is not token-delivery or provider readiness.

|Symptom|Check first|Do not do|
|Readiness false / pool rejected|Both migration phases, role/function ACLs, exact native job linkage and guard definitions|Widen grants or skip startup validation|
|Reservation replay rejected|Exact initiating login, revision, session version, admission closure and database wall-clock expiry|Mint a new identity or extend the old grant|
|Stop acknowledged but operation UNKNOWN|Issued input liability and Egress state are separate; local strict revocation is unqualified|Display broadcast/resource cleanup success from the Stop response alone|
|Observation rejected|Source kind, original generation/lease, exact room/Egress association and coherent timestamps|Relax timestamp/fencing validation to fit a fixture|
|Old binary rejects new executor functions|The strict allowlist deliberately detects incompatible expanded capabilities|Claim a transparent rolling migration|

Do not log raw bearer tokens, JWTs, key material or caller-supplied errors while
diagnosing. The eight-field reservation spec is not itself a browser credential.
Production diagnosis stays read-only until a concrete backup, compatibility
procedure and impact scope are approved. Customer broadcasts must not be stopped
to make migration convenient.

## Reproducible gates and next boundary

`bash scripts/dev/test-local.sh --live-media-input` is the fast isolated PG18/race
feedback subset. It refuses missing test files/symbols; it does not replace
`--live-media-stop`, `--live-media-runtime`, `--studio-backend`,
`--browser-studio-bff`, or the final full `test-local.sh` race/vet gate.
Owner-seeded future-wire fixtures prove SQL invariants only, not provider calls.

The next separately reviewed BRW unit must add the original-job consumer,
provider camera-plus-microphone observation, final Start gate, bounded cleanup
and authenticated post-commit credential delivery. Local browser/SFU decoding,
qualified Cloud token revocation, real output visibility, Studio UI acceptance
and production deployment remain separate evidence requirements.
