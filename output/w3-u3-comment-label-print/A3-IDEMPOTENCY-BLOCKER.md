<!-- Purpose: transfer the real A3 replay defect without widening the UI unit.
Depends on: live-console-v1 §7.4, real signed Next/Go/PG print acceptance and existing command receipts.
Used by: integrator-assigned backend task f4c26fcb and W3-U3 final gate. -->
# A3 print facts — backend handoff (P1 / I02)

Owner ruling: “交给后端单元修复，保留 UI 范围”.
Backend coordination task: `f4c26fcb-ab2d-4a1b-9e29-0908e02406d3` (submitted for assignment, not claimed by this UI author).

## Reproduction (E3, source 7877c4ef; first seen on 26d665f2)

`LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-live-console` → exit **1**.
Workspace 23/23; labels 8/9. Only failed case: “W3U3 lost A3 acknowledgement retries the same fact key”.
The browser sends a real A3 through the signed BFF/Go/PG path. Playwright `route.fetch()` lets it commit,
then drops only the ACK. A real second click resends the **same** key/body. Key equality passes; count equality fails:
first **7**, replay **8**, expected **7**. There is no synthetic response replacing the write.

- Final run: `resume/browser-live-console-final.log`.
- Final error: `resume/idempotency-final-error.txt`.
- Final full artifact: `output/playwright/live-console-2056379755/`.
- Regression remains in `tests/admin/comment-label-print.spec.ts`; no assertion removed or weakened.

## Root cause and exact boundary

`internal/httpapi/live_stream.go:53–62` uses `liveStreamScoped` and invokes `PrintComment` directly.
`internal/live/stream.go:176–185` accepts no command key and calls `live.comment_print`.
`migrations/0123_live_console_comments.sql:449–479` says “no idempotency replay beyond the row itself” and increments
the count on every call. Row upsert is not command-key replay. This conflicts with §7.4 “idempotent per key”.

Backend owner chooses the scoped command-receipt implementation. Preserve body exactly `{}`, rechecked
`live:manage`, server tenant/store scope and no label content persistence. Prove same-key replay does not increment,
new-key print does increment, and wrong scope/authority cannot replay another store's receipt.

The UI now checks current management authority across awaits, expires on A3 scoped 404, and retains an UNKNOWN key
for an explicit retry. It does **not** automatically retry. Browser native printing remains available after a
non-authority network failure, without claiming a new confirmed badge.

No product Go/SQL/DTO/contract changes were made in this UI unit. No push or deployment.
