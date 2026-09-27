# Browser input restart observation — BRW07

Status: DESIGN_CANDIDATE; implementation and real-process gates NOT_RUN.
Baseline: `9a5636e`. Owner: integrator, task `e8665b18-b402-4c67-b976-281bc85582e4`.
Extends `live-media-recovery-observer-v1.md` and
`live-browser-input-worker-v1.md`; does not replace either cleanup contract.

## Scope and reuse

After abnormal restart, within the original parent's 90-second monotonic
window, obtain committed fresh observations for every admitted liability OR
retain UNKNOWN responsibility, persist a sticky timeout, and emit the existing
redacted actionable alert. Observation does not mean cleanup, token revocation,
decoded AV, native River rescue, or completed Stop. Actual alert delivery and
deployment binding remain separate release gates.

Reuse one parent episode/t0, one total capacity (1..32), original operation/job,
original generation/token/30-second reconcile lease and existing timeout path.
No new queue, Stop ledger, cleanup ordinal, global River rescue tuning, aged
timestamps, or provider mutation. Keep the legacy seven public SQL signatures.
Extend by a forward migration; do not rewrite historical migrations.

## Admission and immutable masks

Persist `include_browser_input boolean NOT NULL DEFAULT false` on the existing
episode scope. The new Begin sets true and admits a union of:

- Existing eligible `PROVIDER_MOCK` originals, with mask `(false,true)`.
- `LOCAL_SFU_MOCK_EGRESS`, immutable runtime marker 1, with issued non-CLOSED
  input (`grant_iat IS NOT NULL`) and/or reserved nonterminal Egress. Include
  held input and escalated Egress. Include READY/gen0 issued input even before
  its first worker claim, using LEFT JOIN for missing execution state.

Exclude marker 0, UNISSUED/pre-grant-only attempts and unrelated profiles.
Capture unique operation IDs AND both liability bits in the initial ordered
LIMIT capacity+1 snapshot. Overflow is fail-whole, not the first 32 successes.
Acquire established business locks in deterministic operation order before
writing scope. Under lock, validate exact attempt/custody/original job identity;
create a missing execution projection from immutable issued-input identity.
OR initial bits with current liabilities for each already selected operation.
Never drop a selected member or clear a bit because it became terminal while
waiting. Contradictory identity must fail closed, not create a covered empty
scope. A terminalized/unclaimable required side may time out; it cannot vanish.

Persist profile, original job, baseline generation and nonempty mask on the
existing admitted operation event. Add explicit checked columns rather than
reason-code JSON. Scope, membership and active projection commit atomically.
At SQL function entry capture one clock_timestamp BEFORE any lock wait; derive
the diagnostic deadline from that entry timestamp plus max(0,90000-elapsed)ms.
Never derive it from a later post-lock timestamp. Replays keep the original
deadline. Parent postcommit monotonic readback remains the final success clock.
Do not add new candidates after the initial snapshot. Coverage describes that
snapshot, not resources created later. Prior unfinished episodes, deadlines,
capacity, generation ceiling and native-job eligibility retain MRR semantics.

Same episode ID replay checks scope kind before returning history. Legacy
Begin/claim/record/finish/read/witness reject mixed scopes with ME409, INCLUDING
already-witnessed replay. Legacy Begin still excludes INPUT. Shared timeout is
the only intentionally cross-kind legacy entry. Private implementation helpers
may be reused; never route mixed legacy members through a public legacy API.

## Evidence and qualification

Add append-only `live.media_input_recovery_observations`: UUID id, unique
(episode_id,operation_id), original job and attempt, generation, exact
project/version/endpoint/room/publisher, source/result, participant SID/state
and observed_at. Foreign keys and checked correlation bind these fields to the
admitted attempt and original job. FORCE RLS; SECDEF writer access only, no raw
table access for recovery/runtime/worker roles. No cleanup responsibility or
retry authority lives here.

Valid input evidence is exactly one of:

| Source/result | Payload and meaning |
| --- | --- |
| PARTICIPANT/PRESENT | Exact GetParticipant target, PA_ SID and adapter-validated JOINING/JOINED/ACTIVE/DISCONNECTED state. Presence is an observation, not usable media. |
| ROOM/ABSENT | Strict exact-room ListRooms empty response; NULL SID/state. A transient absent room does not revoke a cached token or close custody. |

GetParticipant failure alone is not ABSENT. Optional exact-room fallback may
qualify only ABSENT; room PRESENT with unknown participant remains unqualified.
Malformed responses, errors and unrelated rooms never qualify. Input insert and
`input_qualified` operation event are one transaction. Do not reuse pre-Start
`input_observed_at` or cleanup READ ordinals. Event `observation_id` is the
input receipt only for `input_qualified`; Egress `qualified` retains its existing
media-observation ID and execution-state projection.

Egress uses the current private `project_media_observation`, extended episode
qualification only when its immutable mask requires Egress. Keep exact wire,
project, room/EG ID, generation and reconcile fences. A normal worker observation
may qualify only if it meets the same episode/mask/deadline checks. No provider
webhook, pre-admission observation or cleanup ordinal substitutes for proof.

Input must record BEFORE Egress because the Egress projector clears the lease.
An error on one side must not suppress a safe observation on the other side.
Required terminal/held sides are not silently cleared. When the existing Egress
projector cannot accept terminal/escalated state, leave that bit unproven and
timeout truthfully rather than changing cleanup semantics.

First committed proof for each side is immutable. If input G commits and Egress
fails, a later claim with fresh token/generation G+1 attempts ONLY missing Egress.
Composite proof accepts different generations, both above admission baseline,
same episode/op/job/target. Never overwrite, copy to G+1, or reuse G's token.
After either record ACK loss, read durable evidence before another claim; if
both IDs exist, witness without repeating provider I/O. SQL record entrypoints
reject existing proof; only readback recovers its ID. An input-only observation
releases its own lease through Finish; Egress remains the last projector call.

## Private SQL ABI

All new entrypoints: READ COMMITTED, owner commerce_media_writer, SECURITY
DEFINER, search_path=pg_catalog; exact EXECUTE grant to commerce_media_recovery
only. No PUBLIC grants or overload aliases. Invalid shape ME400; wrong scope,
membership, generation/token/lease/target ME409. Tokens are exactly 32 bytes.
Arguments/columns below are ordered. Absent IDs are SQL NULL, not empty UUIDs.

`begin_media_recovery_episode_with_input(uuid,bigint,integer,boolean)` returns
the nine legacy Begin columns followed by
`execution_profile text,input_required boolean,egress_required boolean`.
Scope-only rows use NULL profile and false/false masks. Legacy dispositions
remain; no `checked` admission shortcut.

`claim_media_recovery_observation_with_input(uuid,uuid,bigint,bytea)` returns
`disposition text,generation bigint,project_id text,credential_version bigint,
endpoint_identity text,room_name text,publisher_identity text,egress_id text,
execution_profile text,input_required boolean,egress_required boolean,
input_observation_id uuid,egress_observation_id uuid`.
Accept both admitted profiles, not INPUT alone. Require native eligibility,
unexpired episode and NULL/expired original lease, not current resource absence.
Only `claimed` exposes targets; other rows return NULL target/proof fields and
false masks. Reuse dispositions claimed/busy/already_observed/terminal_at_lock/
native_ineligible/ceiling/overdue. `already_observed` requires every masked side;
held/terminal state on one side cannot prevent observing another missing side.
Generation increments once, same 30-second lease. A side with existing receipt
is skipped. No remaining safely recordable side returns terminal_at_lock, not
already_observed. Do not spend dispatch or cleanup budgets.

`record_browser_input_recovery_observation(uuid,uuid,bigint,bytea,text,text,text,text)`
arguments are episode,operation,generation,token,source,result,SID,state;
returns `disposition text,input_observation_id uuid`, disposition `checked`.
Lock original operation, execution state and custody in established order;
require runtime1, admitted input bit, exact identities, unresolved input and
current lease. Receipt timestamp must be at or before the scope deadline.
It never closes input, changes holds or finalizes the native job.

`record_media_recovery_observation_with_input(uuid,uuid,bigint,bytea,text,text,text,text,bigint,bigint,bigint)`
has legacy record arguments and `(disposition text,observation_id uuid)` output;
accepts either admitted profile with Egress bit. Uses the same private projector
and `checked|terminal` results. It cannot accept input-only members or missing
wire. Both receipt qualification paths require timestamps within this episode.

`finish_media_recovery_observation_with_input(uuid,uuid,bigint,bytea,text)`
returns text `released`; same five legacy reason codes only. Releases only its
own current reconcile lease, even for input-only or independently held Egress.
No required-terminal predicate that would prevent input-only lease release.
Preserve original UNKNOWN responsibility, holds, ordinals and native job.
An already-released/stale lease cannot release a new owner's lease.

`read_media_recovery_episode_with_input(uuid)` returns the 14 legacy Read
columns (observation_* still Egress only) followed by
`execution_profile text,input_required boolean,egress_required boolean,
input_observation_id uuid,input_observation_source text,
input_observation_result text,input_observation_generation bigint,job_id bigint`.
Read immutable admitted masks/job and exactly correlated receipt events, not
caller masks or only current custody state. A partial receipt is not a witness.
For new scope `checked` means all required evidence exists, independent of
cleanup_required. Preserve historical readback after projection moves on.

`witness_media_recovery_episode_with_input(uuid,uuid,uuid,uuid,bigint)`
arguments are episode,operation,input_ID,Egress_ID,parent_elapsed_ms;
returns witnessed/already_witnessed/timeout_wins/unqualified. Require exact IDs
for required bits and NULL for non-required bits, nonempty mask, immutable
profile/job/target, each generation > baseline, no sticky timeout, and both
qualified receipt timestamps within the episode deadline. SQL observed_at is
not a commit timestamp: only the parent's successful postcommit readback sampled
within 90 seconds attests timely visibility. Required generations
need not be equal. Same IDs plus same elapsed is the only idempotent replay.
Check scope kind BEFORE replay. Timeout-first cannot later become witnessed.

Reuse `timeout_media_recovery_episode(uuid,bigint)` only after tests prove it
handles every new member, including newly created gen0 projections. Same sorted
operation-lock-first order, no scope-to-operation inversion. Unknown coverage,
overflow and prior unfinished scope still produce sticky scope timeout. Known
complete empty remains NO_WORK, never a fresh-observation PASS.

`media_browser_input_recovery_ready() RETURNS boolean` checks all seven new
signatures/return shapes, owner/ACL/search_path/SECDEF, scope/mask/receipt
constraints, FORCE RLS, qualifier/immutability/native guards AND legacy ready.
Old seven-function readiness count stays seven. Extend exact platform recovery
allowlist and registrar negative audit, not broad name-prefix grants.

## Go consumer and time

Extend RecoveryMember/RecoveryReadback with profile/masks/input evidence/job;
existing ObservationID/Source/Generation remain Egress. New diagnostic constructor
`NewMediaRecoveryLedgerWithBrowserInput(pool)` selects mixed scope without I/O.
`NewMediaRecoveryObserverWithBrowserInput(ctx,pool,projects,runtime)` validates
both immutable maps (project/version/endpoint), new readiness and exact role.
Legacy constructors remain legacy. A diagnostic ledger never enables I/O.
Begin/Read/Observe dispatch only according to the constructed scope mode.
Add `WitnessWithInput(ctx,episode,operation,inputID,egressID,elapsedMS)`;
legacy Witness remains legacy and must never bypass mixed SQL validation.

Supervisor stores one pending composite attestation of both IDs and elapsed,
created ONLY after committed readback with every required proof and full
membership. Reject an entire malformed batch: wrong cardinality, duplicate or
foreign operation, or row episode/job/profile/mask/baseline not equal to the
admitted member. Validate coverage/count and scope fields consistently across
all rows before honoring `finished`; a row status alone never resolves a mixed
episode. Scope-only empty is exactly one row with no member and known coverage.
Negative elapsed is invalid. Reuse original parent t0 before config/DB/child release. Elapsed is
sampled after Read returns; a first read after 90000ms cannot create proof.
A timely cached attestation may be retried after deadline, as in MRR, but loses
to an already committed timeout. No new provider I/O after deadline.

SQL calls <=5s. For each claim use one provider budget <=10s shared across
participant/fallback/Egress, not 10s per call; entire turn <=25s and <=episode
remaining time. Best-effort fenced Finish has <=5s and cannot extend the parent
deadline. Capacity is one bounded concurrent wave, not serial per-member waits.
Persisted proof readback after ACK loss is mandatory before retry.

## Process capability binding (release dependency)

Current main/native child has no compiled input configuration loader. Do not
claim or enable process BRW07 with just a test-injected parent pointer. Existing
BrowserInputRuntime is a validated typed map, not a transport-safety proof.
Use ONE reviewed loader/config mapping in BOTH supervisor and exec child;
independently reconstruct identical project/version/endpoint/local-SFU mapping.
Do not invent a second permissive parser beside LoadWorkerProjects. Its exact
configuration format and process wiring require a follow-on frozen amendment
before activating the mixed entrypoint. Legacy wrappers stay disabled by default.
This dependency blocks real-process gate acceptance, not SQL/readback coding.

Requested input capability with absent/invalid loader must use mixed diagnostic
admission with unknown coverage, fail provider readiness and retain sticky
timeout/alert; it must not scan only legacy and report empty. Parent retains its
independent recovery-role DB path even when native config cannot load. Child
never gets recovery DSN/tokens. All pools must prove same physical DB and role
readiness. No production input activation is authorized by this contract.

## Acceptance and remaining limits

Independent test_worker authors actual PG tests against the ABI before root
integration. Exact gate plan, not a claim that tests exist:

1. READY/gen0 issued input without execution state; input-only/both/legacy mixed
   members; held-input/active-Egress, held-Egress/active-input, terminal-Egress/
   unresolved-input; immutable bits across admission lock wait; unique cap33.
2. First proof commits, other fails, new lease/generation fills only missing
   proof; ACK loss for each record; partial/stale/wrong-role/job/target/token/
   generation/marker0/pregrant negatives; old witness/read/claim cannot bypass.
3. Real unaged SIGKILL/parent+exec-child restart from t0 before configuration,
   old live lease wait, repeated child crash, config/DB/provider failure and
   provider result at deadline; exact postcommit readback and timeout races.
4. <=90 seconds composite committed/readback witness OR sticky unresolved
   timeout and configured redacted alert. Zero observer Start/Stop/Remove/Delete,
   zero cleanup ordinal expenditure; original operation/job and independent
   liabilities retained. Native orphan-job progress is a SEPARATE BRW03 gate.
5. Legacy MRR, BRW03/04, owner-approved LMR05, platform privilege matrix, full
   PG/race/vet and actual Studio browser gates. External alert delivery and
   deployment wiring are still required; local mock cannot prove Cloud/G06.

Do not widen runtime activation or production claims until those gates pass.
This extends evidence of state inspection, not the bounded cleanup engine.
