# Unit LC-U2a: live console comment stream, reply modes and rule hints (W2-U2 middle column; right column reuses BuyerPanel)

Status: FROZEN (integrator, 2026-10-08). Base: `r3/integration` a698d44c, after LC-U1 #5, LC-U2b #8 and LC-B3b #12 merged.
Branch: `unit/lc-u2a-comment-stream`. Worktree: `.worktrees/lc-u2a-comment-stream`.
Read first, in order: `docs/delivery/AGENT-PREAMBLE.md` → `AGENTS.md` → `docs/delivery/PROCESS.md` → this file → `contracts/live-console-v1.md`. Sections to read:
- §2.6 (comments HTTP, epoch/reset, cadence 3 s, pause when hidden, back-off);
- §3.3 (send rules) and §3.5 (public-reply content rule);
- §4.2 (one private reply per comment) and §4.4 (visible send states);
- §6 (capability states) and §7.1 (console read model);
- §11 rows A1–A5, A8, A9 and A13;
- §12 (privacy), plus Amendment A1.1 (`link_pending_manual`).

Then read the current code: `apps/admin/components/{LiveWorkspace.tsx,LiveConsole.tsx,BuyerPanel.tsx,InboxThread.tsx}`, `apps/admin/lib/inbox-*.ts`, `apps/admin/src/features/live/**`, and the BFF catch-all.

## Why
LC-U1 shipped the console shell: left column, status bar, offers and stock. LC-U2b shipped the inbox and BuyerPanel. What's missing is the console's middle column: one stream of live comments with keyword marks, and replying from the console. A seller works the whole live sale from this column. It is SHOPLINE parity, and it is required by LC-U3 (create-order drawer), LC-U4/W3-U3 (label print) and W3-U2 (live settings).

## Scope (UI and BFF only; no Go, migration or OpenAPI changes)
1. **`CommentStream.tsx`, the middle column.**
   - A2 polling every 3 s while visible, paused when hidden, with back-off 3 s → 30 s on 5xx.
   - Cursor by `{epoch, seq}`: on `reset=true`, clear and re-read. `older_cursor` loads earlier comments.
   - Filters: 全部 / 關鍵字 / 私訊 / 待回覆. Use A2 `marks` for keyword comments, and A8 `filter=live_comment&session_id=` for the private-message and unreplied views.
   - Show the comment text, author (may be null), time and an attachment flag. Never put comment text into the URL or into storage.
2. **Selecting a comment or bundle** opens the right column with **`BuyerPanel` reused as-is** (A13 by `bundle_id`). Do not fork it. Add props only if needed, and if you do, keep `--browser-inbox` green.
3. **Reply from the console.** Two modes, private reply (A4) and public reply (A5), plus templates.
   - **Rule hints shown before sending** (§3.3, §3.5):
     - one private reply per comment, and its state when it is already used;
     - after the buyer messages, the 24 h window;
     - never put a payment link in a public reply. The server returns 422 `public_reply_forbidden_content`; show it as coded copy.
   - Instagram live public replies are unsupported (409 `ig_live_unsupported`).
   - Send states follow §4.4. UNKNOWN keeps a "verify in Messenger" guard, the same way the inbox does.
   - On 409 `used`/`auto_pending`, show the coded reason and never resend automatically.
4. **Capability** states (§6) disable the controls, with reasons shown.
5. **BFF:** add only the exact A2–A5 paths, methods and query/body keys, with the same CSRF and origin rules.

## Never
- No comment text, PSIDs or buyer PII in logs, URLs, storage or artifacts. Responses stay no-store and no-referrer.
- No identity guessing (I09).
- No automatic retry of an UNKNOWN or 409 send.
- Do not build the create-order drawer (LC-U3) or print (LC-U4). Leave a disabled placeholder entry point only if the layout needs one.

## Gates (red → green)
- **Node model tests:**
  - cursor and epoch reset;
  - filter mapping;
  - cadence, pause and back-off (fake timers);
  - reply-mode rule hints;
  - every coded error;
  - BFF seam tests (exact path, method and keys).
- **Browser `--browser-live-console`,** extended over a real Go/PG seed, zh-TW/zh-CN/en at 1440/390:
  - the stream renders and paginates older comments;
  - an epoch reset clears the stream;
  - filters;
  - selecting a comment shows the BuyerPanel with claims;
  - a private reply sends once and a second attempt shows `used`;
  - a public reply containing a payment link is refused with the coded copy;
  - a hidden tab stops polling, which you prove with request counts.
- **Calibration:** add an `LC_CONSOLE_CALIBRATION=retain-on-reset` fault that keeps old comments after `reset=true`. It must fail exactly the reset case.
- Also: `--browser-inbox` stays green, plus G-UI1, `test-node.sh`, `check-gates.sh` and typecheck. Click sweep and visual lint run in CI.

## Roles
| Role | Who |
|---|---|
| Implementer | Codex-1 |
| Pre-push and independent review | Kimi K3 (privacy and send rules, deep) |
| PR, push and merge | integrator |
| Independent acceptance | LC-T1 (K3), after this unit merges |
