<!-- Purpose: transfer the real A3 replay defect without widening the UI unit.
Depends on: live-console-v1 §7.4, real signed Next/Go/PG print acceptance and existing command receipts.
Used by: integrator-assigned backend task f4c26fcb and W3-U3 final gate. -->
# A3 print facts — backend handoff (P1 / I02)

**RESOLVED after PR #30:** backend `b1bfbeb3` merged into W3-U3 `042758c5`; original real-click replay now returns **[7,7]** with the same key. Current proof: `pr30-ready/DELIVERY.md`. Keep the historical red below; this is no longer the active blocker. Full-wrapper response-wait failures are owned by W3-U3; the former task8f1bb58f handoff is canceled as 误交接 under the integrator's 2026-10-10 correction. The unchanged trunk b1bfbeb3 control passed the full mode.

**Final update:** the independent full-wrapper blocker is also resolved by the authorised BFF read-scope repair58510545. Frozen source d8744793 full live-console exit0 (23+9), inbox exit0 (13), same-key [7,7] unchanged. See `correction-reruns/DELIVERY.md`; neither blocker is active.

Owner ruling: “交给后端单元修复，保留 UI 范围”.
Backend coordination task: `f4c26fcb-ab2d-4a1b-9e29-0908e02406d3`, claimed by `qwen-aliyun-backend`; branch `unit/lc-a3-print-idempotency`. This UI author does not implement the backend fix.
UI handoff accepts only this case as **BLOCKED(backend f4c26fcb)**; the existing test still fails normally and is not skipped.

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

Integrator assigned `command.Run` receipt replay keyed by `Idempotency-Key`, with Go red tests. Preserve body exactly `{}`, rechecked
`live:manage`, server tenant/store scope and no label content persistence. Prove same-key replay does not increment,
new-key print does increment, and wrong scope/authority cannot replay another store's receipt.

The UI now checks current management authority across awaits, expires on A3 scoped 404, and retains an UNKNOWN key
for an explicit retry. It does **not** automatically retry. Browser native printing remains available after a
non-authority network failure, without claiming a new confirmed badge.

After the backend PR merges, rerun the **unchanged** real-click lost-ACK case via the full `--browser-live-console` mode and append green evidence. Until then retain the red counterexample.
No product Go/SQL/DTO/contract changes were made in this UI unit. No push or deployment.
