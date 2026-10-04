# REVIEW — meta-multi-page backend (independent, adversarial)

Reviewer: test_worker + security_reviewer (Kimi K3), not the implementer.
Base: branch `unit/meta-multi-page` @ 452556d (as worktree `unit/meta-multi-page-tests`).
Scope: migrations/0108, internal/metaconnect, internal/integrations/metareply (Unsubscriber),
claims-intake routing (internal/claims/claim_source.go), internal/httpapi/meta_connect.go,
contracts/meta-claims-intake-v1.md ("Merchant connect (R4)" + §7), docs/delivery/units/meta-multi-page.md (Decisions 1–5).

Verdict: **no P0, one P1 (cross-unit caller left behind in apps/, documented D5 hand-off), 4 P2.**
Backend security invariants hold; evidence = the focused suite plus the new independent tests
(`tests/foundation/meta_multi_page_independent_test.go`, MPG06–MPG09) and three mutation reds
(output/meta-multi-page-tests/).

## Verified (security invariants, all hold)

1. **Page-token custody unchanged (D4 / contract §7).** 0108 adds no token column; `meta_connect_finish`
   receives only sealed HPKE bytes (0108:70-148); the API seals to the PUBLIC ring and can never open
   (`internal/metaconnect/service.go:444`, `Seal.Seal`); only the claims-worker opens (metareply
   `openPageToken`, `unsubscribe.go:111`). `meta_connect_status` (0108:156-176) returns ids/names/status/
   permissions/timestamps — no token, nonce or ciphertext. `ops.audit_events` has no payload column
   (0001:54-64, action CHECK `^[a-z][a-z0-9_.:]{0,79}$`), so connect audits are fixed strings. The stored
   pick list (`meta_connect_states.pages`) is built from `pageEntry` which has no Token field
   (graph.go:34-47); the user token is process-memory only (service.go:67-116, zeroed on pick/expiry).
   Covered by MPG09 (token absent from status body, pick-list rows, command receipts; ciphertext absent
   from the status DTO) and the existing TestMetaConnectAPIHoldsNoPagePrivateKey / MCI10 env guard.
2. **A page_id can never feed two stores.** `page_id text ... UNIQUE` (0095:44) is untouched by the re-key
   (0108:22-23); `connect_activate` serializes per asset (`pg_advisory_xact_lock('meta-owner',object,asset)`,
   0095:103) and raises PT409 → `page_taken` before the INSERT is ever reached, so the cross-store race
   loser never hits a bare 23505. `asset_owners` survives disconnect, so a disconnected Page cannot
   silently move stores either. Covered by MCG03 (3-store race) and MPG07 (2-store race + reconnect after
   winner's disconnect).
3. **Cap 10 under the store lock.** `meta_connect_finish` keeps the 0100 per-store advisory lock
   (0108:90) and checks `count>=10 AND NOT EXISTS(same page)` inside it (0108:106-108); the prepare-time
   check (0108:53-54) is only a pre-Graph precision refusal, finish is authoritative. Concurrent
   different-Page connects at 9/10 collapse to exactly one winner (MPG06); the 11th sequential pick is
   `409 cap_exceeded` and a refresh at the cap still works (MPG02).
4. **Per-Page disconnect revokes only that Page.** 0108:188-209 scopes the unsubscribe job (one row, head
   of `c.fb_binding` only), credential deletes, route disables and the row delete to `p_page`; the caller
   disables exactly the two returned bindings (service.go:487-505). Peer credentials survive byte-identical
   (MPG08), peer keeps routing (MPG08), exactly one PENDING job for the disconnected Page (MPG03).
5. **Re-key + backfill.** 0108:22-23 re-keys on populated data without touching the row; MPG01 proves the
   seeded row survives, page_id stays UNIQUE, a second Page inserts, re-Apply is idempotent.
6. **Intake routing by page_id.** `resolveBinding` pins a numeric Page id to that asset
   (claim_source.go:261-276), routes stay keyed by (object, asset_id); a comment on Page A routes to A's
   session, Page B to B's (MPG05), a comment on a disconnected Page's post stages nothing (MPG08).
7. **Tenant/store from server auth only.** Every definer authenticates via `meta_connect_auth(p_hash,...)`;
   the service derives scope from `platform.WithScope(token)`; no client-supplied tenant/store is read.
8. **Removed `already_connected` / changed disconnect body — Go callers.** None left behind in Go: the only
   caller of `meta_connect_disconnect` is the 3-arg call in service.go:481; cmd/claims-worker consumes the
   jobs table (unchanged since 0100); `already_connected` is gone from `frozenStatus` (errors.go:32-39) and
   no live definer raises it (0095/0100 bodies are superseded by 0108). See P1-1 for the apps/ caller.

## Findings

### P1-1 — apps/ callers left behind by the changed disconnect body and status shape (cross-unit break on merge)
`apps/admin/lib/meta-connect-client.ts:36` still POSTs `meta-connect/disconnect` with `"{}"`; the Go
handler now requires `page_id` (internal/httpapi/meta_connect.go:71-78, `metaConnectDisconnectFields`), so
the admin card's Disconnect button gets `422 invalid_request`. `parseStatus`/the card consume the old
`{connected, page:{…}}` shape while `meta_connect_status` now returns `{connected,count,cap,pages:[…]}`
(0108:175), and stale `already_connected` copy remains in `apps/admin/lib/meta-connect-copy.ts:36,76,115`
and `tests/admin/meta-connect-model.test.ts:90`. Failure scenario: merge backend alone → Settings >
Facebook card shows "not connected" for connected stores and disconnect always fails; both
`--browser-meta-connect` gates (browser_meta_connect_test.go, browser_meta_connect_gate_test.go) go red in
CI. The implementer's SUMMARY documents this as the D5 hand-off to the UI owner (Codex); it must land in
the same release cut. No Go caller is affected.

### P2-1 — disconnect takes no per-store lock: disconnect racing a same-Page reconnect can unsubscribe the live Page
`meta_connect_disconnect` (0108:188-209) does not take the (tenant,store) advisory lock that
`meta_connect_finish` takes (0108:90), and finish's supersede of PENDING jobs (0108:135-136) runs before its
INSERT. Interleaving: finish(T1) supersedes (no job yet) → disconnect(T2) enqueues the job and commits →
T1 inserts the fresh row and commits. The PENDING job then makes the worker DELETE
`/{page}/subscribed_apps` for a Page the DB shows as connected (Meta unsubscribed until the next
reconnect). Adjacent to the contract's known limit (reconnect vs leased job, R4 §4); same-store,
self-inflicted, recoverable by reconnecting. Suggested fix (product owner decision): disconnect also takes
the store advisory lock.

### P2-2 — meta_connect_gate_test.go:494 still accepts the removed `already_connected` code
The MCG03 loser-code assertion tolerates `already_connected`, which no definer raises any more. Harmless
dead tolerance, but it would mask a regression that re-introduced the old refusal. (Test-only; not a
product defect.)

### P2-3 — browser gate not re-run for the multi-page flow
The unit Gate requires `--browser-meta-connect` extended (connect A and B → both listed → picker offers
both → disconnect B leaves A working); it is blocked by P1-1 and NOT_RUN here (backend-only worktree).
Tracked for the UI task, not a backend defect.

### P2-4 — gofmt drift in internal/metaconnect/errors.go
`gofmt -l` (this machine) flags internal/metaconnect/errors.go:36 — map-key alignment went stale when the
`already_connected` line was removed (the implementer's run reported clean; likely a gofmt version skew).
Cosmetic; left untouched per "do not fix product code".

## Mutation evidence (each key assertion proven falsifiable)

| Mutation (reverted) | Assertion | Red evidence |
| --- | --- | --- |
| 0108 + `DROP CONSTRAINT meta_connections_page_id_key` | page_id UNIQUE platform-wide | TestMetaConnectMultiPageUpgrade FAIL (output/meta-multi-page-tests/mutation_unique_red.log) |
| 0108 finish: cap check commented out | cap 10 under the store lock | TestMetaConnectIndCapConcurrent FAIL (mutation_cap_red.log) |
| 0108 disconnect: unsubscribe job enqueued for ALL heads of the store | per-Page unsubscribe | TestMetaConnectGatePerPageDisconnect FAIL (mutation_disconnect_red.log) |

Green after revert: output/meta-multi-page-tests/final_run.log. Product diff empty at the end.
