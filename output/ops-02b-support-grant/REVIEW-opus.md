# OPS-02B support grant: independent security final review (Opus)

- Reviewer role: independent security final reviewer. Read-only: no edits to source, no commit. Model: Claude Opus 5.5.
- Reviewed: worktree `.worktrees/ops-02b-support-grant`, HEAD `e68b73a2` (author `c1358b06` plus trunk merge), `git diff r3/integration...HEAD`.
  `migrations/0153_support_grants.sql` sha256 `c2710a05...b045`; `internal/platform/platform.go` sha256 `371636cc...3c93`.
- Inputs: brief `docs/delivery/units/ops-02b-support-grant.md` (Integrator 裁决 overrides), `DELIVERY.md`, contracts platform-operator-v1 §7, merchant-identity-v1 amendment, AGENTS.md.
- Evidence class of this review: DESIGN/static (DB-free). Nothing was executed against PG. The author's REAL_PG logs predate the trunk merge (see P2-6).
- Integrator ruling respected: support cannot read order pages yet (default-deny through `read_merchant_orders*` and `merchant_access_denied`). That is not raised as a defect.

## Verdict: MERGE-AFTER-FIX

There is no P0. There is one P1: the "designated platform support principal" required by the ruling does not exist, so any active principal can be granted, including a merchant of another tenant. The read-only enforcement, expiry, revocation, suspension, ACLs and audit are sound. The P2 items should land in the same patch where noted.

---

## P0
None.

## P1

### P1-1: there is no designated support principal; any active principal, including a member of another tenant or a removed staffer, can be granted
- **Evidence**
  - `0153:110-114`: `grant_support` only requires `identity.principals.active` and the absence of a `store_grants` row for **this** store. There is no check that the principal is platform support staff.
  - `0153:236-243`: the `resolve_access` support branch checks only the grant row and the absence of `store_grants` for this store. `0153:300-305` (`list_session_stores`) does the same.
  - `tests/foundation/support_grant_test.go:533`: `s.must(s.grant(s.storeB, s.member, 4, nil))`. The test pins a **successful** grant of tenant B's store to an active member and owner of tenant A.
  - `0089:367-369`: `staff_remove` **deletes** the removed staffer's `store_grants`. That makes a removed staffer of the same store eligible again. This contradicts the definer comment at `0153:112` ("never layered on a member or former member") and contract §7 "Default-deny".
  - Brief, Integrator 裁决, line 1: "给**一个指定的平台支持 principal** 开…只读的店铺访问" (open read-only store access for **one designated platform support principal**).
- **Why it matters.** Whether one merchant (or ex-employee) can read another tenant's store depends entirely on an operator typing the right UUID. The grant has no binding to platform staff, so a wrong UUID or a socially-engineered request is not refused.
  - The merchant-visible audit (`support.granted`) shows only a principal UUID, so the merchant cannot tell that the "support" person is a competitor or a fired employee.
  - Once a grant is open, it also survives a later regular grant and revives after `staff_remove`, for up to 72 h (`0153:242-243`). The branch checks `NOT EXISTS store_grants` per request, but nothing closes the support row when regular access appears and then disappears. This happens even though the merchant explicitly removed the person.
- **Fix (small).**
  1. Add a platform support registry, for example `identity.platform_support_principals(principal_id PK, added_at, added_by_db_user, operator, ticket, disabled_at)`. It is written only by two new operator definers (EXECUTE `commerce_platform_operator`), which are audited in `control.operator_audit`.
  2. `grant_support` requires an enabled registry row **and** refuses a principal with any active membership in any tenant (PT409).
  3. The `resolve_access` support branch and the `list_session_stores` support arm also require the registry row (`disabled_at IS NULL`), so disabling a support person ends all of their grants at once.
  4. Optional defence for the revival case: `staff_accept` / `staff_apply_role` close any open `support_grants` row for (store, principal).
  5. Tests: flip `support_grant_test.go:533` to expect a refusal. Add "grant to an unregistered principal → PT409/PT404", "registry disabled → next request not_found", and "staff_remove does not revive support".
  6. Fix the comment at `0153:112`.

## P2

1. **NULL permission fails open, but only in the support branch.**
   - Evidence: `0153:248` `IF p_permission !~ ':read$' OR NOT (p_permission = ANY(v_support_perms))`. With `p_permission IS NULL`, the condition is NULL, the IF is skipped, and the function returns `ok`/-1. The regular branch (`0153:257-261`) returns `forbidden` for NULL.
   - Reachability: `ads.auth` (`0074:380-388`) loops over a caller array whose NULL elements pass its `cardinality`/`array_ndims` checks. All callers found pass literal arrays, so this is not client-reachable today.
   - Fix (one line, same patch): `IF NOT coalesce(p_permission ~ ':read$' AND p_permission = ANY(v_support_perms), false) THEN … 'forbidden'`. Add an SG test calling `resolve_access(hash, store, NULL)` for support and expecting `forbidden`.

2. **Refused or failed support attempts leave no trace.**
   - Evidence: `support.used` is inserted inside the request transaction (`platform.go:679-690`), and any `fn` error rolls it back (`platform.go:695-701`). Out-of-pack permissions return before the insert, because `resolve_access` gives `forbidden`.
   - Why it matters: a support principal probing write routes (25006) or out-of-pack routes is invisible to the merchant and to ops.
   - Fix: log `support_read_only` / `forbidden` denials for scope revision -1 at the Go layer. Optionally write the use row in its own short transaction.

3. **Free-text ticket and operator label are shown to the merchant.**
   - Evidence: `0153:131-133` and `0153:163`. The details carry `operator` and `ticket`. The ticket accepts any printable text, 1..80 characters (`0153:101`, `main.go validTicket`).
   - Why it matters: an operator can paste internal notes or buyer PII into a merchant-visible row.
   - Fix: restrict the support ticket to an ID charset (for example `^[A-Za-z0-9._#-]{1,40}$`) in both the CLI and SQL, or omit the ticket from `ops.audit_events.details`.

4. **The contract over-claims which routes support can reach, and the 403 body code is not delivered.**
   - Many pack-permission handlers have Go validators that refuse `scope.Revision < 1` with `ErrInvalid` (400/422), not 403. Examples:
     - `merchanttools/dashboard.go:78`
     - `live/draft.go:226` (all live routes: studio, console, comments)
     - `claims/claims.go:124`
     - `payments/methods.go:396`
     - `fulfillment/service.go:366`
     - `integrations/accounts/service.go:246`
     - `storefrontadmin/merchant.go:54`
     - `storefrontdomains/service.go:90`
     - `promotions/admin.go:118`
     - `reporting/*.go`
   - Some read definers write and so now fail with 25006: `fulfillment.read_cvs_shipment` "settles a stuck attempt", `cvs.go:663`.
   - Safe (default-deny), but contract §7 says "the read routes built on it". In practice support reaches the store route, the store list, catalog/product/collection/image/inventory/warehouse reads, billing standing and a few settings reads.
   - The brief's `403 support_read_only` body (item 6) is recorded as a follow-up.
   - Fix: list the routes support actually reaches in §7, and map `ErrInvalid` for revision -1 to 403.
   - Forward risk: these `Revision > 0` validators are now a load-bearing second fence. If `live/draft.go:226` is relaxed, `GET …/comments` (`live/stream.go:149-167`, `live:read`) would call Meta Graph with the merchant's Page token and return buyer `author_name`/text. That is PII, which contradicts "pack has no PII". Any unit that relaxes a `Revision > 0` guard must be re-reviewed against support.

5. **Test gaps.**
   - No test that `staff_remove` does not revive support (see P1-1).
   - No test for a NULL permission (P2-1).
   - No HTTP-level test that a write-verb route reachable with a pack `:read` permission is refused. `POST …/settings/payment-methods/{code}/inspect` (`httpapi/settings.go:43`, `integration:read`) takes `FOR UPDATE` locks: today it returns 25006 → 403.
   - SG02's write probe is a direct INSERT. A SECURITY DEFINER write probe would document the "definer is also blocked" claim. (PG `XactReadOnly` is transaction-global, so it is blocked.)

6. **Evidence is not bound to HEAD.**
   - `green.log`, `green3.log` and `green4.log` predate the trunk merge. `green3.log` passed the R2 pin at 74 files. HEAD pins 77 (`r2_integration_upgrade_test.go:63-65`), which was not run locally.
   - DELIVERY.md still says "73 -> 74".
   - CI must run `^TestSupportGrant|^TestR2IntegrationUpgrade|^TestIdentity|^TestPlatformOperator|^TestWAS` plus the brief's browser modes on `e68b73a2`. Until then: E3 for `c1358b06` only, NOT_RUN for HEAD.

7. **Nits.**
   - `0153:66-69` drops `operator_audit` CHECKs by `LIKE` pattern. This is correct today (only the action and scope checks match) but fragile. Naming the constraints explicitly is safer.
   - `commerce_platform_writer` holds column INSERT on `support_grants.granted_at`. The writer is NOLOGIN with no members (`0143:25-32`), and `SET ROLE` is refused (SG08), so only the definer path can set it. The definer always uses `clock_timestamp()`. No action needed.

---

## Attack checklist (what holds, with evidence)

### 1. `resolve_access` diff
- **0003 is the latest prior definition.** A grep of all `migrations/*.sql` and `migrations/post_river/*.sql` for `CREATE|REPLACE|ALTER|DROP … FUNCTION identity.resolve_access` finds only `0003:3`, `0003:43` and `0153:204`. `post_river/0013` and `post_river/0016` only *call* it.
  - A `diff` of `0003:3-42` against `0153:204-265` shows the regular branch is unchanged byte for byte. The only changes are the `v_support_perms` declaration and the block that replaces the `not_found` return.
  - `list_session_stores`: the latest prior definition is `0089:441`. Its body is preserved as the first `UNION ALL` arm.
- **The support branch runs only after the regular lookup finds nothing, and only with no `store_grants` row** (`0153:231`, `0153:242-243`). A regular member, or a principal with a lingering grant, is never routed to it.
- **Expiry and revocation** use `g.revoked_at IS NULL AND g.expires_at > clock_timestamp()` (`0153:241`). The session itself still uses `statement_timestamp()`, unchanged from before.
- **An inactive store or tenant is refused**: joins `s.active` and `t.active` (`0153:238-239`). Granting on a suspended store gives PT409 (`0153:109`).
- **Only granted `:read` permissions pass**: `0153:248`. The table CHECK limits grants to the read pack (`0153:26-27`). See P2-1 for the NULL case.
- **`authz_revision = -1` cannot collide with a real revision.** `memberships.authz_revision` has `CHECK > 0` (`0001:32`), and Go coalesces a non-ok result to 0 (`platform.go:654`).
  - `RequirePermission` and the SQL "fresh/final" fences compare -1 with -1 (consistent). Fences comparing against a membership revision deny support (`0039:63-64`).
  - No caches, ETags or session-revision stores were found: grep of `internal/platform` and `internal/httpapi` for caches found none. No SQL treats a negative or zero revision as "skip".
- **A removed regular member** keeps access only if a support grant exists for them. That gap is P1-1.

### 2. Read-only transaction for revision -1
- **Order of operations**: resolve_access, then scope GUCs, then the `support.used` INSERT, then `set_config('transaction_read_only','on',true)`, then `fn` (`platform.go:679-694`). Nothing runs between the audit write and read-only mode.
- `transaction_read_only` cannot be switched back after the first snapshot, which SG02 tests. It is transaction-global, so it also blocks writes inside SECURITY DEFINER bodies, `nextval`, and River job INSERTs. NOTIFY is still allowed but is harmless without job rows.
- **`support.used` is written before read-only mode** and is rolled back on failure (P2-2).
- **Handlers with their own pool or transaction**:
  - billing (`billing:manage`) and metaconnect Pick/Disconnect/Callback (`integration:manage`, `metaconnect/service.go:218-538`) are outside the pack.
  - `cvs.go:271` uses `integration:manage`.
  - `order_for_buyer.go:550` also requires `inventory:reserve`.
  - `session_stores.go:35` uses the pool directly, but only against the STABLE read definer.
  - `attribution/feed.go` and the buyer routes are not merchant scopes.
- **External side effects after a `:read` scope**: none found on reachable pack routes. Live and comments are blocked by `live/draft.go:226`.

### 3. `list_session_stores` role "support"
- **Display hint only**:
  - `routes.ts:56-60` `canOpen` uses the permission list, or `role === "owner"`.
  - `team-model.ts:96-100`: `team` is shown only for the owner.
  - The BFF (`apps/admin/proxy.ts`) only proxies to Go, and no TS code accesses the DB.
- The two arms cannot overlap: the support arm requires the absence of `store_grants`, the regular arm requires `store:read`.

### 4. Grant, revoke and list definers
- **EXECUTE** is granted only to `commerce_platform_operator` (`0153:189-192`). The owner is `commerce_platform_writer`, which is NOLOGIN with no members. SG08 checks 10 real logins (42501) and `SET ROLE`.
- **Operator identity**: `session_user` is recorded as `db_user` (`0153:127-129`). `--operator` is only a label, as in 0143.
- **The 72 h cap** is enforced by the table CHECK (`0153:34`) and the definer (`0153:99`).
- **One open grant per (store, principal)**: partial unique index (`0153:39`) plus a store `FOR UPDATE` lock (`0153:107`).
- **Audit goes to both tables**: grant (`0153:128-133`), revoke (`0153:162-167`).
- **No PII in audit**: rows hold UUIDs only, apart from the free-text ticket (P2-3).
- **search_path and names**: `search_path=pg_catalog`. All relations are schema-qualified. The only functions used are built-ins.
- **No identity function has PUBLIC EXECUTE**: a script-checked comparison of every `CREATE FUNCTION identity.*` against the `REVOKE … FROM PUBLIC` statements found none missing. The new `USAGE ON SCHEMA identity` for the operator therefore exposes nothing else.

### 5. CLI
- **DSN** comes only from the environment. Usage errors are fixed codes, and `PT409` maps to `platform_admin_conflict` (`main.go:265`). Invalid input never opens the DB (`main_test.go`).
- **`deploy/scripts/ops-admin.sh`** has no `platform-admin` branch on this base (its case list covers only stripe-, meta- and store-admin). `logins.tsv` has no operator login. Nothing is exposed; wiring is a follow-up for the integrator (contract §6).

### 6. Login path
- **Any principal can sign in**: password signup/login (`0070:351`) and OIDC (`0004:86`) issue `audience='merchant'` sessions without needing a membership. Support people use the normal login, as ruled; there is no impersonation.
- **Audience `'support'`** (`0001:48`) still has no issuer or reader.
- **Routes without `resolve_access`**: `list_session_stores` (a projection that now includes the same support predicate), `staff_accept` (invite token plus email match), and `create_initial_store` (the caller's own new tenant). All are pre-existing, and none reaches another tenant's data.

### 7. Tests
- **Red/green**:
  - SG01–SG08: red without the migration (`red.log`: 8 FAIL, exit 1); green with it (`green.log`: 8 PASS).
  - CLI usage matrix: `main_test.go`.
  - The R2 pin change is a count bump only (76 → 77), not a weakened assertion.
- **Missing coverage**: P2-5.
- **Evidence binding**: P2-6.
