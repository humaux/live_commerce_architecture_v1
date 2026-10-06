# OPS-02B support grant: independent security re-verification (Opus)

- **Reviewer:** independent security re-verifier, Claude Opus 5.5. Read-only: no source edits, no commit. This file is the only write.
- **What was reviewed:**
  - Fix commit `9a844c00`. Its code equals `77dfb44a`: `git diff 77dfb44a 9a844c00` touches only `output/` (DELIVERY.md, check-gates.log, green-fix.log).
  - The fix diff, `git diff e68b73a2 9a844c00`.
- **File hashes:**
  - `migrations/0153_support_grants.sql` sha256 `925911e0…74c3`
  - `tests/foundation/support_grant_test.go` sha256 `87ca248f…51f1`
- **Evidence class:** DESIGN/static, DB-free. Nothing was executed against PG here.
- **Author's evidence:** `green-fix.log` is bound to `77dfb44a` (130 top-level PASS, exit 0). It includes SupportGrant x11, `TestR2IntegrationUpgradeFromReleaseHead`, Identity x12, Platform x12, Staff x20 and WAS01-06. It is not the full suite, so CI must still run PG on `9a844c00`.
- Line numbers below refer to `migrations/0153_support_grants.sql` at `9a844c00` unless another file is named.

## Verdict: MERGE

- **P1-1 is FIXED.**
- **The trigger is sound.** It is not redundant: it is the only thing that stops a grant reviving after `staff_remove`. Keep it.
- **No new P0 or P1.**
- **Three new P2s**, all small. They can land in the same integration batch or as follow-ups: N1 (re-enrol race), N2 (stale contract §6 wiring), N3 (the HTTP write-verb assertion proves the Revision fence, not read-only).

---

## 1. P1-1: FIXED

| Requirement | Evidence |
|---|---|
| Registry exists, operator-managed | `identity.platform_support_principals` (29-36), FORCE RLS (37-38), `REVOKE ALL FROM PUBLIC` (39). The writer has only SELECT/INSERT plus a column UPDATE (40-41). `commerce_auth` reads two columns (42). SG08 pins the table and column ACLs (test diff, lines 341-348). |
| Add and revoke: EXECUTE `commerce_platform_operator` only | Owner is `commerce_platform_writer` (314-315); REVOKE PUBLIC and GRANT to the operator (321-324). SG08 extends the 10-role denial matrix to both functions. Internal helpers (`support_principal_ok`, `support_close_grants`, the trigger function) are unreachable by any `commerce_*` role except owner and writer (SG08, test 349-356). |
| Audited | `support_principal_add`: 158-159. `support_principal_revoke`: 193-195. Both write to `control.operator_audit` with tenant and store NULL. The new named CHECKs are at 106-111. SG09 counts 2 rows per principal, including the no-op call. |
| Identity from the server | `db_user = session_user` (154, 159, 194). `--operator` is only a regex-checked label (145, 183). The CLI validates its input before opening the DB (`cmd/platform-admin/main.go` support-principal case; `main_test.go` usage matrix). |
| `grant_support` requires an enabled registry row and a merchant-free principal | 238-239 call `support_principal_ok`, defined at 132-137: an enabled registry row, no active membership anywhere, no `store_grants` anywhere. |
| `resolve_access` support branch requires the same | 389: `IF NOT FOUND OR NOT identity.support_principal_ok(v_principal)` gives `not_found`. |
| `list_session_stores` support arm requires the same | 450. |
| Registration also refuses merchants | 150-152 (PT409). |
| Registry revoke cuts access immediately | `revoke_support_principal` (179-197) sets `revoked_at` and calls `support_close_grants` (165-175). The resolve-time registry check is a second fence. SG09: access is not_found on both stores, the list drops to 0, open grants drop to 0, and there are 2 merchant `support.revoked` rows. |
| The cross-tenant owner test now asserts refusal | `support_grant_test.go` SG07: `grant(storeB, member)` must be PT409, and memberships are unchanged (test diff, lines 115-121). |

**Why the trigger is needed (no-revival case):**
- 0089 `staff_remove` (0089:362-376) deletes the `store_grants` rows and sets `memberships.active = EXISTS(grants)`, which is false for a single-store staffer.
- After that, `support_principal_ok` would be true again.
- Without the trigger, an open grant from before the person joined as staff would come back to life. Section 2 covers this.

## 2. The new trigger

**Definition.**
- Trigger function: `identity.support_regular_access_trigger()` (202-214).
- Trigger `support_regular_access_grants`: `AFTER INSERT ON identity.store_grants` (325-326).
- Trigger `support_regular_access_membership`: `AFTER INSERT OR UPDATE OF active ON identity.memberships WHEN (NEW.active)` (327-328).

**Definer hygiene: OK.**
- `SECURITY DEFINER SET search_path=pg_catalog` (203). Every relation is schema-qualified, and the only built-in it uses is `clock_timestamp()`.
- Owner is `commerce_platform_writer` (317), which is NOLOGIN with no members.
- EXECUTE is revoked from PUBLIC (319). PG checks EXECUTE on a trigger function only at CREATE TRIGGER, not at fire time, so merchant roles need nothing extra.
- The owner holds what the body needs:
  - UPDATE on the registry `revoked_at` column, plus SELECT and UPDATE policies (41, 43, 45).
  - Implicit EXECUTE on the same-owner `support_close_grants`.
  - Column UPDATE on `support_grants`, plus a policy (80, 84).
  - INSERT on `operator_audit` (0143:58,60) and the `ops.audit_events` INSERT policy (121-122).
- The trigger audit row satisfies every CHECK:
  - operator `'system'` matches `^[a-z0-9._-]{1,40}$`;
  - ticket `'regular-access'` is valid;
  - tenant and store are NULL, as the tenant/store scope CHECKs require for `support_principal_revoke`.

**Abuse ("merchant invites the support principal to knock out support"): not unilaterally possible, fails closed, and is audited. Acceptable.**
- Only three things fire the trigger:
  - `staff_accept`. It needs the support principal's own merchant session plus a matching invite email (0089:384-410).
  - `create_initial_store`. It needs the principal's own session.
  - `staff_apply_role`. It is internal (EXECUTE `commerce_staff_writer`) and runs only from `staff_accept` / `staff_set_role` on an existing member.
- No merchant-only path inserts `store_grants` for someone else's principal.
- The worst case is that the support person disables themselves: a DoS of support, not an access gain.
- Each disenrolment writes:
  - one `control.operator_audit` row `support_principal_revoke` with `detail.automatic=true` (209-211);
  - one merchant-visible `support.revoked` per closed grant, with reason `regular_access` (172).
- Minor: the merchant whose grant was closed learns that the support person gained merchant access somewhere, but not where. Negligible.

**Privileges and lock order inside merchant transactions: low risk.**
- For a non-registry principal, the trigger is one PK-probe UPDATE that matches no row: no row lock and no extra writes. It fires for every owner bootstrap and staff accept (0004/0065/0079 `create_initial_store`, 0089 `staff_accept`/`staff_apply_role`) and for `cmd/admin-fixture`.
- **Deadlock (P3).** One theoretical cycle exists:
  - Preconditions: an operator re-grants the same (store, principal) after expiry, and at the same moment that principal accepts a staff invite in the same tenant.
  - `grant_support` holds the expired `support_grants` row (`FOR UPDATE`, 242-243) and then waits on the placeholder membership `ON CONFLICT` (250).
  - `staff_accept` holds that membership row, and its trigger waits on the same `support_grants` row (169).
  - PG's deadlock detector aborts one transaction. Nothing becomes inconsistent. Accept.
- Read-only support transactions never fire it, because they cannot insert.

**Existing flows and tests: not broken.**
- `staff_accept` uses `INSERT … ON CONFLICT DO UPDATE SET active=true` (0089:404-405), so it activates an existing inactive placeholder membership and fires the trigger as intended.
- `staff_set_role` only updates `authz_revision`, so the `UPDATE OF active` trigger does not fire. `staff_remove` fires it only while `active` stays true.
- `green-fix.log`: Staff x20, Identity x12, Owner, NewInitialStore, Merchant x7, Customers, Live x4 and WAS all PASS on `77dfb44a`.
- Not proven: the full suite (many fixtures insert `store_grants`). This is CI.

**Is it needed? Yes. It is not redundant, so keep it.**
- The resolve-time check covers only the period while regular access exists (SG09 "trigger off" block).
- After `staff_remove` the principal is merchant-free again, and only the trigger has already closed the grants and disenrolled them.
- Optional simplification: drop the `memberships` trigger and keep only the `store_grants` one. Every real activation path (`staff_accept`, `create_initial_store`) also inserts `store_grants`. The membership trigger is cheap, so this is optional and not a finding.

## 3. `resolve_access` body

- Against 0003:3-42 (the latest prior definition; a grep finds no other CREATE/ALTER/DROP of it, only GRANT EXECUTE additions), `diff` shows exactly three changes:
  - `CREATE` → `CREATE OR REPLACE`;
  - a `v_support_perms` declaration;
  - the `not_found` return replaced by the support branch (378-399).
- The regular branch is byte-for-byte unchanged.
- Compared with the first round, the only changes are:
  - the per-store `NOT EXISTS store_grants` became `support_principal_ok(v_principal)`, which is strictly stronger;
  - the NULL-safe permission guard.
- Signature, owner and EXECUTE are unchanged. `CREATE OR REPLACE` keeps the owner `commerce_auth` (0003:43) and every later GRANT EXECUTE.
- `support_principal_ok` is owned by `commerce_auth` (318), so `resolve_access` calls it as its owner.
- `list_session_stores` differs from the first round only at 450.
- Fresh-vs-upgrade catalog parity: `TestR2IntegrationUpgradeFromReleaseHead` PASS in `green-fix.log:408-409` at 77 files on `77dfb44a`. 0153 is a plain forward migration that is identical in both paths.

## 4. P2s from the first review

| Item | Status | Evidence |
|---|---|---|
| NULL permission denied | Fixed | 393: `IF NOT coalesce(p_permission ~ ':read$' AND p_permission = ANY(v_support_perms), false)` → forbidden. `TestSupportGrantNullPermissionDenied` covers support and member, plus `orders:write`, `customers:read` and `pricing:read`. |
| Merchant-visible audit is minimal | Fixed | Grant: `{label:'平台支援', grant_id, expires_at}` (258). Operator revoke: `{label, grant_id, reason:'operator'}` (288). Registry or trigger revoke: `{label, grant_id, reason}` (172). Ticket and operator stay in `control.operator_audit` (253-255). SG01 asserts the label is present and the ticket and operator are absent. |
| Contract §7 truthful | Mostly | §7 now says support reaches the store route, the store list and "the few read routes whose Go validators accept the support scope", and names the refused families and the second fence. It is still not a per-route list (P3). See N2 and N3 below. |
| `Revision > 0` fences not relaxed | Confirmed | `git diff --stat e68b73a2 9a844c00 -- internal/` is empty. The only `Revision` lines in the diff are contract and docs text. |
| Write-verb routes refused | Proven at the Go level | `TestSupportGrantWriteVerbRouteWithReadPermissionRefused`: `SELECT … FOR UPDATE` under an `integration:read` support scope gives `ErrSupportReadOnly`. The HTTP half is weaker; see N3. |

## 5. Evidence honesty

- **`red-fix.log` is a setup failure, not an assertion failure.** All 11 tests fail at the fixture's `add_support_principal` call (42883). It therefore proves nothing about the individual assertions.
- **Would each new assertion fail if its specific fix were reverted?** Reasoned per assertion:

| Assertion | Reverted fix | Red? |
|---|---|---|
| SG07: grant to tenant A's member on tenant B must be PT409 | Registry/merchant-free check in `grant_support` (back to the per-store NOT EXISTS) | Yes. The member has no `store_grants` on storeB, so the grant succeeds. |
| SG09: unregistered principal must be PT409 | Registry clause | Yes |
| SG09: removed staffer not grantable | Registry clause | Yes |
| SG09: no open grants / registry disabled after regular access | The `store_grants` trigger | Yes |
| SG09: "support revived after staff_remove" | The trigger | Yes. Without it the registry stays enabled and the principal becomes merchant-free again. |
| SG09 "trigger off": regular grant must refuse support on `store` | The `store_grants` clause of `support_principal_ok` in `resolve_access` | Yes. This matches DELIVERY's mutation note. |
| `support_principal_ok`'s **membership** clause at resolve time | That clause | **No.** The trigger-off block inserts only a grant. The clause is never exercised alone (P3 test gap). |
| `resolve_access` **registry** clause at resolve time | That clause | **No.** Registry revoke also closes every grant, so access ends either way. The clause matters only in the N1 race (P3 test gap). |
| `list_session_stores` `support_principal_ok` | That call | **No**, same reason. The arm is display-only; routes stay gated by `resolve_access`. |
| NULL permission → forbidden | The `coalesce` | Yes. The old `!~` gives NULL and returns ok. |
| Merchant audit has no ticket or operator | The details change | Yes |
| HTTP POST inspect refused (`rec.Code >= 400`) | Read-only transaction | **No** (N3) |
| Go `FOR UPDATE` → `ErrSupportReadOnly` | Read-only transaction | Yes |

- **Net:** every security property of P1-1 has at least one assertion that turns red when its fix is reverted. The gaps are only in the second-fence checks.

---

## New findings (no P0, no P1)

### N1 (P2): a registry revoke or trigger racing a concurrent `grant_support` can leave an open grant that revives on re-enrolment
- **The race.** `grant_support` reads the registry through `support_principal_ok` without a row lock (238). Under READ COMMITTED:
  - a concurrent `revoke_support_principal` (186 `FOR UPDATE`, then 192), or the trigger (206, 208),
  - runs `support_close_grants`, whose UPDATE cannot see the grant row that is inserted but not yet committed (251).
  - Both transactions commit. Result: registry revoked, grant open.
- **Today it is inert.** The resolve-time registry check refuses it (389), and that check is exactly why it is worth keeping.
- **The problem.** `add_support_principal` re-enables the row (155) without closing stale grants. A grant that the operator audit did not list as closed comes back for the rest of its window, at most 72 h.
- **Fix (pick one, one line each):**
  - in `grant_support`, lock the registry row `FOR SHARE` before the eligibility check, which serialises it with the revoke's `FOR UPDATE` and the trigger's UPDATE; or
  - have `add_support_principal` call `support_close_grants(p_principal,'re_enrolled')` when it re-enables a row.

### N2 (P2, documentation and wiring): contract §6 is stale for the integrator
- `contracts/platform-operator-v1.md:43` still says "Add the **three** 0153 definers…", "(4 definers)" and allows only `support-grant|support-revoke|support-list`.
- `add_support_principal`/`revoke_support_principal` and `support-principal-add|support-principal-revoke` are missing.
- If the integrator follows §6 literally, nobody can be enrolled, so no grant is possible. That fails closed, but the feature would be dead.
- Fix: update §6 to five definers and the five commands.

### N3 (P2, evidence labelling): the HTTP write-verb check does not exercise read-only
- `POST …/payment-methods/{code}/inspect` reaches `payments.InspectMethod`. Its `authorize` (`internal/payments/methods.go:394-397`) returns `ErrInvalid` for `scope.Revision < 1` before any lock. The test also uses a random market id.
- The `rec.Code < 400` assertion therefore passes because of the Revision fence (or a 404) and would stay green without the read-only transaction.
- The Go-level `FOR UPDATE` probe is the real proof.
- Fix: in contract §7 ("Pinned by tests…") and the test comment, say the HTTP refusal is the Revision fence, or assert the specific status and code.

### P3 (no action required)
- **Test gaps:**
  - the membership clause of `support_principal_ok` at resolve time;
  - the registry clause at resolve time and in `list_session_stores` (each needs a direct-SQL setup with a revoked registry row and an open grant).
- **Deadlock window** described in section 2. The PG detector resolves it.
- **Seq scans.** `support_principal_ok` and `add_support_principal` look up `memberships`/`store_grants` by `principal_id` alone, and neither table has an index leading with `principal_id`. Only support requests and operator calls pay this. Add an index if those tables grow.
- **Former merchant staff can be enrolled.** The registry does not refuse principals with past, inactive memberships. That is an operator decision, and it is audited.
- **Route list.** §7 is still not a per-route list of what support reaches.
- **First-review items still open:** P2-2 (denied or failed support attempts are not audited) is DEFERRED and documented in §7. The `support_read_only` 403 body code is still a follow-up.

## Gates still owed (CI, PG)
- `^TestSupportGrant|^TestR2IntegrationUpgrade|^TestIdentity|^TestPlatformOperator|^TestWAS|^TestStaff` plus the full foundation suite on `9a844c00`. Many fixtures insert `store_grants` and now fire the trigger.
- The brief's browser modes.
- Until then: E3 for `77dfb44a` (focused 130 tests), NOT_RUN for the full suite.
