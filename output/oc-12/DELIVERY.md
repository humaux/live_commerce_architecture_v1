# OC-12 delivery — inbox "unread" filter

Role commerce_worker · model Claude Opus 5.5 (subagent, no recursive delegation) · base origin/r3/integration @ 1360be187 ·
worktree .claude/worktrees/oc12 · branch unit/oc12-inbox-unread · PR #227 → r3/integration (no auto-merge).
Self-report = E0; acceptance belongs to the integrator's independent review.

## Unread definition (summary; full text output/oc-12/UNREAD.md)
Unread = `COALESCE(last_inbound_seq,0) > read_seq` on `inbox.conversation_state` (live-console-v1 §3.6, OPEN-9):
store-level (shared by all staff), cleared only by `inbox.mark_read` for `inbox:reply` holders, re-opened when the
`social.messages` trigger raises `last_inbound_seq`. Scope = server GUCs from `platform.WithScope`; no request tenant id.

## Changes (all inside write_paths except the deviation noted)
| File | Change |
|---|---|
| migrations/0204_inbox_unread.sql | `CREATE OR REPLACE social.list_conversations` (same signature/result/owner/EXECUTE): `'unread'` in the PT400 allowlist + predicate. |
| internal/inbox/read.go | filter=unread never appends link-pending bundle rows (they carry no read marker). Header updated. |
| internal/inbox/unread_test.go | UR01 definition · UR02 store-level clear/reset · UR03 cross-tenant/cross-store isolation (real PG, real service path). |
| apps/admin/components/Inbox.tsx | `unread` option in the existing native filter `<select>` (label = existing copy key `unread`: 未讀/未读/Unread). |
| output/oc-12/UNREAD.md, DELIVERY.md, out-of-scope-allowlist.patch | evidence + the approval patch below. |
| tests/foundation/r2_integration_upgrade_test.go:124-127 | **Integrator-granted** (coordinator message 2026-10-27, scope: the R2 migration-count pin and its comment only): comment line for 0204 and count 132 -> 133. Trigger: shard:g04 at a563f284e (run 40466503323, job 122087549703) failed TestR2IntegrationUpgradeFromReleaseHead "R2 migration set = 133 files ..., want 132". #211 (0202) then merged (fb4558c2c); after merging base the pin is 133 -> 134 (coordinator update, same grant). |

Deviation: the card names `tests/inbox/unread_test.go`. A new `tests/inbox` package needs a non-test `doc.go`
(scripts/dev/check-pkgdocs.sh) and a regenerated `docs/engineering/dependency-map.md` (depmap --check), both outside
write_paths. The test lives in `internal/inbox/unread_test.go` (package inbox_test, inside `internal/inbox/**`), runs in
CI shard:unit under `./...`, and needs no CI registration. Moving it is a pure rename + those two files if preferred.

## Red → green
| Step | Head | Run / job | Result |
|---|---|---|---|
| RED (tests only) | 626cea5a3 | run 40463313144 / job 122054896395 (gate shard:unit) | FAIL exactly 3 top-level tests, all `ERROR: invalid_filter (SQLSTATE PT400)`: TestOC12UnreadDefinition (unread_test.go:226), TestOC12UnreadStoreLevelAndReset (:250), TestOC12UnreadCrossTenantIsolation (:280). No other top-level FAIL in the job. |
| implementation | a563f284e | run 40466503323 | shard:g04 FAIL only TestR2IntegrationUpgradeFromReleaseHead (migration-count pin, fixed by the granted edit above). shard:unit job 122087549874 SUCCESS: TestOC12UnreadDefinition / StoreLevelAndReset / CrossTenantIsolation all `--- PASS` (not SKIP). |
| GREEN (before allowlist) | b9889f0e0 | run 40508176316: shard:unit job 122226111091, shard:g04 job 122226110088 | 81 SUCCESS + 1 SKIPPED (deploy). The 3 TestOC12* tests and TestR2IntegrationUpgradeFromReleaseHead show `--- PASS`. |
| GREEN (final, allowlist + UR04 + INU01) | 52f3ae7adaa85b4ef8dd90ffc627352a05d30e12 | run 40538883929: shard:unit job 122316890900, browser-inbox job 122316892730 | Run and both jobs SUCCESS (API readback 2026-10-28); historical OC-12 evidence, not acceptance of #259. |

RED scope clarification (#242 item 4): run 40463313144 / job 122054896395 observed the SQL
`invalid_filter` path, before the SQL unread implementation. It did **not** execute the later UR04/INU01
assertions and is not their observed CI RED. The pre-allowlist HTTP parser rejected `unread` with 400;
the BFF's `filters` omitted `unread` and its route returned 400 `invalid_filter` (the original before/after
source is preserved in `output/oc-12/out-of-scope-allowlist.patch`). Therefore the new UR04/INU01
`status=200` assertions would fail on that pre-fix 400 path. This is source-level counterfactual evidence
only, not an executed RED of those assertions. Their independent CI mutation RED remains NOT_RUN;
the #259 INU01 row-removal RED (run 40628056371 / job 122622864648) covers a different assertion.

## Out-of-scope edits (integrator-approved 2026-10-28, applied from output/oc-12/out-of-scope-allowlist.patch)
1. internal/httpapi/inbox.go:269: `"unread"` in the A8 filter `case`.
2. apps/admin/lib/inbox-bff.ts: `"unread"` in `filters`.
3. apps/admin/app/api/stores/[store]/[...resource]/route.ts:128: `"unread"` in the invalid_filter list.
4. contracts/live-console-v1.md:675: A8 row `?filter=all|unread|unreplied|…`.
5. tests/admin/inbox-ui.spec.ts INU01: selects the `unread` option and asserts that the BFF list request carries
   `filter=unread`, gets 200 (no 400) and lists the unread thread (real Next/BFF/Go/PG chain, --browser-inbox).
Plus, inside write_paths: internal/inbox/unread_test.go UR04 `TestOC12UnreadHTTP`. It runs the real httpapi handler over real
PG: filter=unread 200 with only this store's unread row, filter=bogus 400 `invalid_filter`, and a foreign-tenant token
gets no 200 and no data.

## Risks
- #242 replaces UR02's simulated reset with `tests/foundation/inbox_unread_trigger_test.go`:
  signed synthetic webhook → real consumer → committed `social.messages` INSERT → production trigger.
  Permission split and the store-level clear remain asserted; CI acceptance of this replacement is pending.
- No partial index; same scan cost as `unreplied`.

## Static checks run locally
gofmt -l (clean), go vet ./internal/inbox (ok). tsc NOT_RUN locally (no node_modules in the worktree) → CI typecheck.

## NOT_RUN
Local tests (policy: CI only) ·
real Meta traffic (N/A) · independent review.
