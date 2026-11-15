# M7 Loyalty Member Recent-Auth RED Regression Test Delivery (Correction-1)

## Metadata
- **Task ID**: `task_9743d9f46d63` (continuation of `task_71314bb6a4b7`)
- **Dispatch ID**: `ctx_442f3cee24a9`
- **Role**: `test_worker` (dispatched worker on `orca_session_id:5e3e4f82-33bc-43ba-b1b3-e88109c8cd5c`)
- **Coordinator**: `orca_session_id:codex_759cd980_5a90_4d23_88cc_66f1621da8d8`
- **Worktree**: `/Volumes/data/live_commerce_architecture_v1/fb-m7-member-reauth-red-1115`
- **Branch**: `luogangan7-lgtm/fb-m7-member-reauth-red-1115`
- **Base Commit**: `f31999ddc098743a82adc6d308043e5c801af513`
- **Prior Test Commit**: `6da65c26b533299444ccc2f0d8d8de869ddcd920`
- **Evidence Class**: `E1` (DESIGN / TESTS_AUTHORED / NOT_RUN — Expected RED only)
- **Phase Status**: `READY_FOR_ROOT_PUBLICATION` (Tests Only — No Production/Migration Fix)

---

## Authority & Confirmed Gap
1. **Contract Invariant** (`contracts/merchant-password-auth-v1.md` §16:616–618):
   - Fixed 10-minute recent-auth requirement inside every guarded SQL definer in the same transaction.
   - Refusal must be HTTP 403 / SQL `PT403` with code `reauth_required` (never 401 or logout; session preserved).
   - Guarded operations include every export (customer privacy export) and erasure. Contract §16 covers every erasure; LP12 / §1:173 fixes stored response after authorization and gives no authorization waiver.
   - Real re-authentication updates `authenticated_at` via `identity.complete_oidc_reauth` or password challenge.
2. **Database Gap** (`migrations/0223_loyalty_points.sql` lines 232–253):
   - `loyalty.export_member(h bytea, s uuid, i uuid)` and `loyalty.erase_member(h bytea, s uuid, i uuid)` call `customers.tn_authority(h, s, 'customers:privacy')` and `customers.tn_fence`, but omit `identity.require_recent_auth(h, interval '10 minutes')`.
   - Shared `0139_customers_authority.sql` `tn_authority` only checks permission/tenant/principal, not `authenticated_at` freshness.
3. **Transport & Go Pipeline**:
   - `internal/loyalty/members.go` maps `PT403` to `platform.PT403(pg.Message, platform.ErrForbidden)`.
   - `internal/httpapi/members.go` scopes privacy routes via `memberScope`, which relies on database / loyalty errors.

---

## Bounded Corrections Applied in this Commit

1. **Stale Same-Key Erase Rejection (`TestLoyaltyLPG06Erasure`)**:
   - Corrected previous acceptance of 200 on stale same-key erase: an expired session (valid session with `authenticated_at` > 10 min) calling erasure with an existing idempotency key MUST be refused with 403 `reauth_required`.
   - Confirmed no extra audit row or command receipt is emitted during the refusal.
   - Verified that subsequent real `sa.reauth()` allows same-key replay to return the exact stored erasure summary (`replayed=true`) with audit count remaining 1.
2. **Removed Ordering Bias in Direct Definer (`TestLoyaltyLPG05Rights`)**:
   - Removed the newly added null-hash and unknown-hash subtests that forced `reauth_required` before `tn_authority`.
   - Preserved all valid-stale session refusals (`"11 minutes"`, `"2 hours"`, `"-2 minutes"`, `"infinity"`, `"-infinity"`), direct ordinary runtime pool calls, unknown-member `PT404`, and freshness controls.
3. **Immediate Effect Assertions & Clean Diagnostics**:
   - Added immediate assertion that `member.exported` audit count remains 1 and no command receipt is stored for `expKey2` immediately after stale new-key export refusal.
   - Replaced body content strings in diagnostic error messages with length/status/code assertions to avoid dumping export JSON or body content.
4. **BFF Test Bytes Kept Unchanged**:
   - Kept `tests/admin/members-bff.test.ts` bytes unchanged from commit `6da65c26b533299444ccc2f0d8d8de869ddcd920`.

---

## Existing Automatic CI Execution Path

- **Foundation Shard CI Path**:
  - `foundation-shards` workflow runs `shard:g10 catch_all` (`scripts/dev/shard-plan.json` line 1423).
  - The `catch_all: true` group executes all tests not explicitly partitioned into groups `g01`–`g09`, automatically covering the three unchanged top-level test functions:
    - `TestLoyaltyLPG05Rights`
    - `TestLoyaltyLPG06Erasure`
    - `TestLoyaltyLPG09HTTPRoutesLogNoPII`
  - (`scripts/dev/test-focused.sh` is a manual local developer helper, not the CI path).
- **Admin BFF CI Path**:
  - `scripts/dev/test-node.sh` line 32:
    ```sh
    node --test --experimental-strip-types tests/admin/members-bff.test.ts
    ```
- **Handoff to Root**:
  - Root derives browser-ownership hashes for both changed test sources.
  - Root publishes Draft PR and triggers CI to collect the original native RED.

---

## Evidence & Verification Boundaries (E1 / NOT_RUN)

- **Evidence Class**: `E1` (Tests Authored, NOT_RUN).
- **Expected Outcome**: Native RED on CI because `migrations/0223_loyalty_points.sql` currently lacks `identity.require_recent_auth`.
- **Claims Boundary**:
  - No claim that RED has already been established or observed on CI.
  - No claim of stale exploit observed LIVE.
  - No claim of native/browser session retention proved (browser acceptance is NOT_RUN).
- **Local Tooling Boundary**:
  - Zero local test/build/lint/typecheck/vet/PG/Docker commands executed.
  - Formatted strictly via standard `gofmt` and inspected via static diff.
  - No production Go/SQL/UI edits, no migrations allocated.
