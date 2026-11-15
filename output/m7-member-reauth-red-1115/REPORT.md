# M7 Loyalty Member Recent-Auth RED Regression Test Delivery

## Metadata
- **Task ID**: `task_71314bb6a4b7`
- **Dispatch ID**: `ctx_7a7039240339`
- **Role**: `test_worker` (dispatched worker on `orca_session_id:5e3e4f82-33bc-43ba-b1b3-e88109c8cd5c`)
- **Coordinator**: `orca_session_id:codex_759cd980_5a90_4d23_88cc_66f1621da8d8`
- **Worktree**: `/Volumes/data/live_commerce_architecture_v1/fb-m7-member-reauth-red-1115`
- **Branch**: `luogangan7-lgtm/fb-m7-member-reauth-red-1115`
- **Base Commit**: `f31999ddc098743a82adc6d308043e5c801af513`
- **Base Tree**: `9b2a2fca247fba0403841a607661439542c033bd`
- **Phase Status**: `READY_FOR_ROOT_PUBLICATION` (Tests Only — No Production/Migration Fix)

---

## Authority & Confirmed Gap
1. **Contract Invariant** (`contracts/merchant-password-auth-v1.md` §16:616–618):
   - Fixed 10-minute recent-auth requirement inside every guarded SQL definer in the same transaction.
   - Refusal must be HTTP 403 / SQL `PT403` with code `reauth_required` (never 401 or logout; session preserved).
   - Guarded operations include every export (customer privacy export) and erasure.
   - Real re-authentication updates `authenticated_at` via `identity.complete_oidc_reauth` or password challenge.
2. **Database Gap** (`migrations/0223_loyalty_points.sql` lines 232–253):
   - `loyalty.export_member(h bytea, s uuid, i uuid)` and `loyalty.erase_member(h bytea, s uuid, i uuid)` call `customers.tn_authority(h, s, 'customers:privacy')` and `customers.tn_fence`, but omit `identity.require_recent_auth(h, interval '10 minutes')`.
   - Shared `0139_customers_authority.sql` `tn_authority` only checks permission/tenant/principal, not `authenticated_at` freshness.
3. **Transport & Go Pipeline**:
   - `internal/loyalty/members.go` maps `PT403` to `platform.PT403(pg.Message, platform.ErrForbidden)`.
   - `internal/httpapi/members.go` scopes privacy routes via `memberScope`, which relies on database / loyalty errors.
   - Stale sessions currently bypass recent-auth checks on export and erasure.

---

## Changes Made (Test-Only Ownership)

### 1. `tests/foundation/loyalty_points_test.go`
- **Imports**: Added `"io"` and `"time"`.
- **Error Classifier (`lpClassify`)**: Updated to map `platform.ErrReauthRequired` to `(403, "reauth_required")` and `platform.ErrOwnerRoleDisabled` to `(403, "owner_role_disabled")` prior to checking `platform.ErrForbidden`.
- **Session Re-Auth Fixture Helper (`newSessionAuth`)**:
  - Implemented `(s *lpStore) newSessionAuth(permissions ...string) *lpSessionAuth`.
  - Creates dedicated synthetic merchant principal and session token.
  - Binds dedicated synthetic OIDC identity in `identity.external_identities`.
  - Cleans up external identities and restores original session `authenticated_at` via `t.Cleanup`.
  - Provides `age(interval)` to update session `authenticated_at = clock_timestamp() - interval::interval` using owner pool.
  - Provides `reauth()` to call the real definer `identity.complete_oidc_reauth(hash, issuer, subject, clock_timestamp())`, verifying outcome `"REAUTHENTICATED"`.
- **Direct SQL Definers Regression Subtest in `TestLoyaltyLPG05Rights`**:
  - Subtest: `"direct SQL definers require recent auth within ten minutes"`.
  - Asserts 9-minute fresh session succeeds for direct `loyalty.export_member`.
  - Asserts fail-closed intervals (`"11 minutes"`, `"2 hours"`, `"-2 minutes"`, `"infinity"`, `"-infinity"`) refuse with SQL `PT403` message `reauth_required` for both `loyalty.export_member` and `loyalty.erase_member`, releasing zero bytes.
  - Asserts null and unknown hash fail closed with `PT403 reauth_required`.
  - Asserts unknown member still returns `PT404 not_found`.
  - Asserts real re-auth restores freshness for both export and erasure.
- **Idempotency Replay & Redaction Subtest in `TestLoyaltyLPG06Erasure`**:
  - Subtest: `"export and erasure recent auth across aging replay and new keys"`.
  - Exercises fresh export with `key1` (succeeds, emits 1 `member.exported` audit event).
  - Ages session to 11 minutes (stale):
    - Same-key replay of export is refused with `403 reauth_required` (because fresh export body generation runs inside definer).
    - New-key export is refused with `403 reauth_required`.
    - Erase on separate member fixture with new key is refused with `403 reauth_required`.
    - Asserts member is NOT erased (`erased_at IS NULL`), no `member.erased` audit row is written, and no command receipt is stored.
    - Asserts session row remains live in `identity.sessions`.
  - Re-authenticates via `sa.reauth()`:
    - Same-key export replay succeeds, `replayed=true`, content matches, audit count remains 1 (audit-once invariant).
    - New-key export succeeds, `replayed=false`, audit count becomes 2.
    - Erase succeeds, `replayed=false`, member is erased, entries redacted, 1 `member.erased` audit event emitted.
  - Ages session to 2 hours:
    - Replay of erase with existing key returns stored receipt without executing definer (LP12 stored response).
    - New key on already-erased member runs definer and refuses with `403 reauth_required` (recent-auth precedes member-erased state).
- **HTTP Privacy Wire Route Subtest in `TestLoyaltyLPG09HTTPRoutesLogNoPII`**:
  - Subtest: `"privacy routes refuse stale auth with 403 reauth_required on the wire"`.
  - Tests HTTP POST `/v1/admin/stores/{store_id}/members/{member_id}/export` and `.../erasure`.
  - Fresh 9-minute session returns HTTP 200 with `attachment;` header.
  - Stale sessions (11 minutes, 2 hours) return HTTP 403 with JSON body `{"code": "reauth_required"}`.
  - Asserts absence of `Content-Disposition` header and absence of member PII in refused responses.
  - Asserts database state remains untouched (member not erased, no audit row).
  - Re-authentication restores freshness: post-reauth erasure returns HTTP 200 and records erasure in database.

### 2. `tests/admin/members-bff.test.ts`
- Added exact two export and erasure 403 envelope preservation assertions:
  - Subtest: `'export and erasure preserve upstream 403 reauth_required envelope without attachment or retry'`.
  - Calls `[member]/export` and `[member]/erasure` when mock upstream returns 403 `reauth_required`.
  - Asserts response status is 403, body code is `reauth_required`, `content-disposition` header is absent, upstream diagnostics are not leaked, and request is not retried.

---

## Native CI Selectors & Expected RED

### Discovered Exact Native CI Commands
1. **Foundation Loyalty Suite (Go Native)**:
   ```sh
   bash scripts/dev/test-focused.sh '^TestLoyalty'
   ```
   Or direct Go test invocation on the catch-all shard:
   ```sh
   GOTOOLCHAIN=go1.27.4 go test -race -v -run '^TestLoyalty(LPG05Rights|LPG06Erasure|LPG09HTTPRoutesLogNoPII)$' ./tests/foundation
   ```
2. **Admin Members BFF Suite (Node Native)**:
   ```sh
   node --test tests/admin/members-bff.test.ts
   ```

### Why Expected NATIVE RED
- The current production database definers `loyalty.export_member` and `loyalty.erase_member` (`migrations/0223_loyalty_points.sql`) do not execute `PERFORM identity.require_recent_auth(h, interval '10 minutes')`.
- When invoked with sessions aged > 10 minutes (e.g. 11 minutes, 2 hours, -2 minutes, infinity), they currently execute successfully (returning 200 / SQL success) rather than raising `PT403 reauth_required`.
- The new tests expect `PT403` / HTTP 403 `reauth_required` and will fail with unexpected 200 / missing refusal on un-patched database definers.
- This establishes the genuine native RED baseline required by the contract prior to minimal production repair.

---

## Boundaries & NOT_RUN Checklist
- **Local Test Execution**: `NOT_RUN` (strict constraint: no local `go test`, `node --test`, `docker`, `pgx`, `devserver`).
- **Production Go & SQL**: `NOT_RUN` / `DEFERRED` (no edits to `migrations/`, `internal/loyalty/`, `internal/httpapi/`, or `apps/`).
- **UI / Playwright Harness**: `NOT_RUN` (browser tests untouched; UI author works in separate lane).
- **Credentials / Migrations**: Zero live credentials, zero migrations allocated this phase.
- **Diff Quality**: Formatted with standard `gofmt` and static inspection; zero warnings.
