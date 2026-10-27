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

Deviation: the card names `tests/inbox/unread_test.go`. A new `tests/inbox` package needs a non-test `doc.go`
(scripts/dev/check-pkgdocs.sh) and a regenerated `docs/engineering/dependency-map.md` (depmap --check), both outside
write_paths. The test lives in `internal/inbox/unread_test.go` (package inbox_test, inside `internal/inbox/**`), runs in
CI shard:unit under `./...`, and needs no CI registration. Moving it is a pure rename + those two files if preferred.

## Red → green
| Step | Head | Run / job | Result |
|---|---|---|---|
| RED (tests only) | 626cea5a3 | run 40463313144 / job 122054896395 (gate shard:unit) | FAIL exactly 3 top-level tests, all `ERROR: invalid_filter (SQLSTATE PT400)`: TestOC12UnreadDefinition (unread_test.go:226), TestOC12UnreadStoreLevelAndReset (:250), TestOC12UnreadCrossTenantIsolation (:280). No other top-level FAIL in the job. |
| GREEN | see PR head | (filled after CI) | |

## Out-of-scope approval request (NOT applied; patch in output/oc-12/out-of-scope-allowlist.patch, 7 lines)
Without these the UI option reaches the API as 400 invalid_filter; the Go/SQL read is complete without them.
1. internal/httpapi/inbox.go:269 — add `"unread"` to the A8 filter `case`.
2. apps/admin/lib/inbox-bff.ts:35-41 — add `"unread"` to `filters`.
3. apps/admin/app/api/stores/[store]/[...resource]/route.ts:128 — add `"unread"` to the invalid_filter list.
4. contracts/live-console-v1.md:675 — A8 row `?filter=all|unread|unreplied|…`.
Recommended: approve all four in this PR (merge blocker until then), plus a tests/admin/inbox-ui.spec.ts case selecting `unread` (out of scope, not written).

## Risks
- Merging without the four allowlist lines ships a visible option that errors (P1 for the UI; no data exposure).
- `last_inbound_seq` reset is simulated in UR02 by updating the column the trigger writes (MOCK of the trigger); the
  trigger itself is covered by the existing 0119 foundation tests.
- No partial index; same scan cost as `unreplied`.

## Static checks run locally
gofmt -l (clean), go vet ./internal/inbox (ok). tsc NOT_RUN locally (no node_modules in the worktree) → CI typecheck.

## NOT_RUN
Local tests (policy: CI only) · browser test of the unread option · HTTP-level filter=unread (blocked on the allowlist) ·
real Meta traffic (N/A) · independent review.
