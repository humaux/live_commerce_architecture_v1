# W3-U2 integrator rulings — 2026-10-09 (answers READINESS.md)

Dependency: base on `origin/unit/lc-u2a-comment-stream` (PR #18, now published at 89499013). Merge it in now; merge it again after #18's round-2 privacy fix lands. Do not wait for #18 to merge.

1. **Automatic reminders.** Ship the manual parts only: trigger, report, copy. Do NOT render an automatic switch or delay that cannot save. The page has no placeholder control for it; one line of helper text is allowed ("自動提醒尚未開放"). The backend unit for automatic reminder settings goes to FOLLOWUPS (integrator adds it).
2. **Sold-out switch.** Authorized: a thin Go HTTP adapter over the existing `GetSoldOutReply` / `SetSoldOutReply`.
   - `GET` and `PUT live-settings/sold-out-reply`, store-scoped.
   - Permissions: `live:read` to read, `live:manage` to write.
   - PUT needs an `Idempotency-Key`, and a CAS `version` that returns 409 on mismatch.
   - Tenant and store come from auth only.
   - The template must be a merchant-published template ID/version that the API returned. Fixed IDs stay immutable.
   - Allowed write paths are the adapter, its registration and its Go tests, plus `contracts/openapi` changes. The OpenAPI file merges via integrator only, so commit it in its own commit.
   - Red test first: an unauthorised PUT gets 403, a stale version gets 409, and a cross-store request gets 404.
3. **Blocklist rows.** No display name. Show platform, note, source bundle and created_at. Never derive identity from names or IDs. Restricted rows hide copy-link (W3-05B follow-up).
4. **Order-drawer warning.** Out of scope. LC-U3 owns the drawer and will reuse `GET claims/blocklist/check?bundle_id` (integrator adds the follow-up).
5. **Browser gate.** Authorized to add:
   - the route page;
   - the `--browser-live-settings` mode registration in `scripts/dev/test-local.sh`, its GATES/README entry, and the `pr-modes` mapping;
   - the Node real-seam tests.
   `check-gates.sh` must stay green, including the G-UI1/G-UI8 route inventory.

Acceptance:
- red→green per item;
- `test-node.sh`, admin tsc and `check-gates.sh` green;
- `--browser-live-settings` green;
- the focused Go regex for the adapter green under REAL_PG;
- a DELIVERY.md with exit codes.
Commit, then stop. Do not push: the integrator runs K3 pre-review, then pushes.
