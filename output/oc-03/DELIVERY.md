# OC-03 re-authentication SQL guards: delivery

- Role: commerce_worker (Claude Sonnet 5.5, subagent). Branch `unit/oc03-reauth-guards`, worktree `.claude/worktrees/oc03`, base `origin/r3/integration` 33a03bbb8 (OC-02 merged). PR #217 (base r3/integration), no auto-merge, not merged.
- Authority: contracts/merchant-password-auth-v1.md §16; contracts/tasks.json OC-03 (integrator ruling 2026-10-27: migration number 0203). Issue #215 items 1 and 2.
- Evidence class: real-PG gates are CI-only (shard:unit). No local go test, no local PG. Self-reported PASS = E0; independent review is NOT_RUN.
- RED: run 40436212776, job 121967146167 (gate shard:unit), head 4a4e25b83, tests only. OG01, OG03-OG06 fail on assertions; OG02 and RA01-RA07 pass (details in `red.log`).
- GREEN and head SHA: a commit cannot contain its own SHA or the run that tests it; both are in the PR body and the worker's report (`gh pr view 217`).

## What changed
- `migrations/0203_recent_auth_guards.sql`: one DO block, pg_get_functiondef + exactly-one-anchor replace (0200 idiom), owner/ACL/signature unchanged.
  - `identity.require_recent_auth(<hash>, interval '10 minutes')` inserted in 16 definers right after their own authorization and before any lock/write (full list, source migrations and owners: `DEFINERS.md`). Dual-mode privacy definers (`record_export`, `erase_owner`): merchant branch only.
  - Owner role disabled: `staff_invite` and `staff_set_role` refuse `owner`, `staff_accept` refuses an owner invitation, all PT403 `owner_role_disabled`; nothing is rewritten, existing owners unchanged.
  - EXECUTE on `identity.require_recent_auth` granted to the owner role of each guarded definer (read from pg_proc), nobody else.
  - #215 item 1: `complete_oidc_reauth` sets `GREATEST(s.authenticated_at, LEAST(p_auth_time, clock_timestamp()))`.
  - #215 item 2: `require_recent_auth` treats `authenticated_at > clock_timestamp() + 1 minute` (and infinity) as not recent.
- `migrations/post_river/0028_recent_auth_refund_guard.sql`: the guard of `payments.request_stripe_refund`. That definer is created and last replaced in the post-River phase (post_river/0013, 0016), which runs after every numbered migration, so 0203 cannot patch it. Same idiom, same owner (commerce_checkout_writer).
- `tests/identity/reauth_guard_sql_test.go`: OG01 stale session refused by each of 17 direct definer calls (decide_bank_transfer twice: confirm and refund_offline); OG02 fresh session passes the guard (staff trio and unshipped-orders export succeed outright); OG03 owner role disabled incl. old owner invitation, viewer invitation still accepted; OG04 guard present in every definer + EXECUTE grantee set exact; OG05 "password then OIDC, time does not go back"; OG06 future/infinite not recent.

## Pin edits (forced by the new migrations)
- `tests/foundation/r2_integration_upgrade_test.go`: R2 count 130 -> 132 (0203 and post_river/0028).
- `tests/foundation/meta_ads_schema_test.go` (MA02): one listed exception, `func:identity.require_recent_auth(p_hash bytea, p_window interval):EXECUTE` for commerce_ads_writer (exact string, no wildcard). Found by CI run 40440719124 (shard g07) at head ce05bed68.
- `tests/foundation/stripe_live_schema_test.go` (SL02 upgrade fixture): 0203 and post_river/0028 are held back with 0137 (0203 patches set_platform_stripe); ledger delta 7 -> 9; `require_recent_auth` ACL compared without the seven owner roles' EXECUTE (helper `slsDropRecentAuthOwners`); the privilege delta accepts exactly `exec identity.require_recent_auth` for those seven roles. Not yet seen red or green in CI.
- `tests/foundation/staff_team_gate_test.go`, `tests/foundation/staff_team_smoke_test.go`: fixtures that needed a second owner through invite/accept or set_role now seed the owner rows directly through the owner pool (new helper `seedOwner`; `join("owner")`, `restorePair`, smoke last-owner race). Assertions are unchanged; the product path can no longer create an owner.

## NOT_RUN / not done
- Local go test and PG (by rule). Independent review.
- Success paths of the payment, privacy, billing and ads families under a fresh session are not in the new test (they need a paid order, a customer, a billing customer, a draft); they are covered by the existing foundation suites, which use fresh sessions and must stay green.
- HTTP mapping of `reauth_required` / `owner_role_disabled` to 403 codes, BFF allowlists and UI (OC-04, OC-15).
- Not guarded (no merchant session-hash SQL entry): see the end of `DEFINERS.md` (settlement CSV is an operator CLI; carrier/pick-list CSV is formatted in Go from a shared read; refund-refresh and refresh_subscription are deliberate).
- Other #215 items (3-9).

## Risks
- The SQL window is fixed at 10 minutes; `COMMERCE_REAUTH_WINDOW_MINUTES` (1-60) cannot reach SQL. A longer Go window would be ineffective, a shorter one only makes the UI ask earlier.
- Until OC-15 (re-auth routes) and OC-04 (UI) land, every guarded operation by a session older than 10 minutes answers PT403 `reauth_required` with no way to refresh it. Merging OC-03 alone on a deployed branch locks merchants out of these operations after 10 minutes. Integrator: order the merges.
- `set_platform_stripe` guards the disable path as well (the contract lists the operation without exception).
- `billing.refresh_subscription` is deliberately unguarded (refresh-on-read mirror); `pin_customer` and `record_checkout_session` are guarded, so a billing checkout started with a session older than 10 minutes asks for re-auth.
- If the guard is satisfied by `authenticated_at = infinity` written before 0203, OIDC re-auth keeps GREATEST(infinity, ...) = infinity; no writer produces it and the guard rejects it, but only the password re-auth path (which sets clock_timestamp()) would repair it.
- Existing long-lived test sessions (older than 10 minutes inside a single test) would now be refused by the guarded definers; none known, CI decides.
