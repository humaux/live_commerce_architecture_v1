# Live planning v1 — LSP local slice of T08

Base: `f4b3c00`. This contract delivers durable merchant **drafts**, not a broadcast
controller. R04/R19/G06 remain incomplete. No LiveKit/Meta calls, jobs, stream keys,
media attempts, destinations, READY/LIVE transitions, HTTP routes or UI in this slice.

## Aggregate and authority

`live.sessions` owns commercial title and optional schedule; `live.programs` owns
the initial aspect ratio. Exactly one program is created with each session in the
same transaction. Both are scoped by tenant and store with composite foreign keys,
forced RLS, and no PUBLIC, buyer or worker privileges. The program state is fixed
to `DRAFT` in this migration: configuration is not platform eligibility.

Explicit new permissions `live:read` and `live:manage` extend the grant vocabulary
only. Existing memberships and onboarding receive no automatic new grants. An
authorized provisioning flow must grant access before future merchant routes ship.
No integration permission implies live permission.

The internal package is `livecommerce/internal/live`. It uses caller-owned
`pgx.Tx` inside `platform.WithScope`, `platform.RequirePermission`, and
`command.Run/Audit`. No pool, transaction engine, queue or new dependency.
Require canonical UUIDs, valid revision, READ COMMITTED isolation, and exact
transaction tenant/store/principal settings matching the resolved Scope. Recheck
token+permission+revision after lock waits and before a successful return, including
receipt replay. A revoked/expired session may not replay an old success.

## Frozen Go API

`DraftInput { Title string; ScheduledAt *time.Time; AspectRatio string }`
(JSON `title`, `scheduled_at`, `aspect_ratio`). Title must be valid UTF-8, 1–200
Unicode code points, already trimmed, without Unicode control characters. Aspect
ratio is exactly `16:9` or `9:16`. Optional schedule is metadata (past allowed),
year 2000–2199 UTC, canonicalized to UTC microseconds before hashing and storage.

`Draft { ID string; ProgramID string; Title string; ScheduledAt *time.Time;
AspectRatio string; State string; Version int64; CreatedAt time.Time;
UpdatedAt time.Time }`, JSON `session_id`, `program_id`, then snake_case. State
must always be `DRAFT`; version starts at 1. No principal, token or secret in DTO.

- `CreateDraft(ctx, tx, scope, token, key, in) (Draft,error)` requires live:manage;
  atomically creates session+program+one audit `live.draft.created`+command receipt
  operation `live.draft.create`. Canonical receipt input includes principal ID.
- `UpdateDraft(ctx, tx, scope, token, key, id, expectedVersion, in) (Draft,error)`
  requires live:manage, locks session before program, revalidates permission after
  waits, checks expected session version, updates both and increments version once;
  one audit `live.draft.updated`, receipt operation `live.draft.update` binding
  principal, session ID, expected version and full canonical input.
- `GetDraft(ctx, tx, scope, token, id) (Draft,error)` requires live:read, reads the
  pair with one joined statement; no side effects or command receipt.

Same scoped key+canonical input returns the frozen receipt (not the current newer
draft). Changed input/principal conflicts. Different keys with the same expected
version yield exactly one success. Separate keys may create separate deliberate
sessions; title is not a uniqueness key. No deletion or start method is supplied.
Known input/version/missing errors map to command.ErrInvalid/ErrConflict/ErrNotFound;
authorization errors remain platform errors. Caller rolls back any error. Never
return a partial Draft alongside an error.

## Acceptance gates (initially NOT_RUN)

- LSP01: isolated PG18 Create/Get/Update, canonical schedule, atomic pair+audit+
  receipt, exact returned DRAFT/IDs/version, rollback leaves all counts unchanged.
- LSP02: concurrent same-key creation gives one pair/receipt/audit; changed input
  and different principal conflict; identical update replay returns original result
  even after a newer edit; competing versions produce one success, one conflict.
- LSP03: missing permission, revoked/expired token, stale revision, forged GUC,
  cross-store and cross-tenant IDs, repeatable-read transactions fail closed;
  successful Get changes no business/receipt/audit/job counts. Migration never
  backfills grants; no PUBLIC/buyer/worker live-table access.
- LSP04: observed real PG row/advisory lock wait followed by expiry rejects update
  or receipt replay and commits no draft/receipt/audit change. Use DB clock and
  pg_stat_activity evidence, not a sleep-only assertion.
- LSP05: title/UUID/aspect/date boundaries, SQL constraints, immutable IDs/creator/
  creation time, no deletion or non-DRAFT state through runtime. Independent tests
  + root race/PG/vet replay and source review. No skipped positive cases.

## Next boundary

Destinations, media attempts, start/stop and reconciliation must be added as a
separate frozen contract before wiring provider I/O. Reuse T06 durable operation
intent and query-only UNKNOWN handling; never infer platform audience LIVE from
transport success. No start action can bypass this stop line.
