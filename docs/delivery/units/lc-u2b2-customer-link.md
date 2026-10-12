# Unit LC-U2b2: inbox customer link (A14) and the full BuyerPanel data (follow-up to LC-U2b)

Status: FROZEN (integrator, 2026-10-08). Base: `r3/integration`, after #8 (LC-U2b) and #12 (LC-B3b) merged.
Branch: `unit/lc-u2b2-customer-link`. Worktree: `.worktrees/lc-u2b2-customer-link`.
Read first, in order: `docs/delivery/AGENT-PREAMBLE.md` → `AGENTS.md` → `docs/delivery/PROCESS.md` → this file → `contracts/live-console-v1.md` §3.7, §6, §11 rows A9, A13 and A14, §12, and the "LC-B3b" amendment notes → the current code: `apps/admin/components/BuyerPanel.tsx`, `apps/admin/components/InboxThread.tsx`, `apps/admin/lib/inbox-*.ts`, and the BFF catch-all `apps/admin/app/api/stores/[store]/[...resource]/route.ts`.

## Why
LC-U2b shipped the inbox and the BuyerPanel, but its A14 control stayed disabled with a "missing version" reason. The A13 claims and orders were also empty until LC-B3b landed.
LC-B3b now exposes:
- A9 `link_version` and `binding_id`;
- A13 `claims`, `claim_total_minor`, `orders` (only with orders:read), `purchase_ordinal` and `auto_reply`.

A merchant must be able to link a conversation to an existing customer, and to see the buyer's claims and orders next to the conversation.

## Scope (UI and BFF only; no Go, migration or OpenAPI changes)
1. **A14 link and unlink in the BuyerPanel.**
   - Search or pick an existing customer: use the customers list read the admin already has (customers:read). No new endpoint.
   - POST A14 with `{customer_id | null, expected_version: link_version}`.
   - Show 「已手動連結」 and the customer name. Unlinking is allowed.
   - On 409 `version_conflict`: refresh the thread header and keep the merchant's choice visible for an explicit retry. Never retry automatically.
   - On 404: show not-found and tear down the panel's private data.
   - Gate the control on inbox:reply **and** customers:read. Without both, show a disabled control with the reason (§6 capability rules).
2. **The full A13 view.**
   - Render the claims (keyword, quantity, session) and `claim_total_minor` (TWD).
   - Render `orders` only when the key is present (it is omitted without orders:read). Never fabricate it, and keep the distinct "server withheld" copy (FOLLOWUPS item from the PR #8 review).
   - Render 第 N 次購買 from `purchase_ordinal`, and the `auto_reply` send state.
3. **`binding_id`.** Match the conversation's capability row by `binding_id` exactly, replacing the "multiple bindings → advisory only" heuristic. With `binding_id: null`, treat replies as unavailable, with the reason shown.
4. **BFF.** Add only the exact A14 path, method and body keys to the allowlist, with the same CSRF and origin rules. Leave every other route unchanged.

## Never
- No identity guess from names or `owner_id` (I09). The only links are `bundle_peers` and an explicit A14.
- No PII in logs, URLs or browser storage. Responses stay no-store.
- Do not widen any other BFF route.

## Gates (red → green)
- Node: tests for the A14 model and BFF seam:
  - exact path, method and body keys;
  - CAS 409 handling;
  - 404 teardown;
  - permission gating;
  - an absent `orders` key.
- Browser: extend `--browser-inbox`, at zh-TW/zh-CN/en and 1440/390:
  - link → reload → 「已手動連結」 persists;
  - a stale version → 409 → refresh → explicit retry succeeds;
  - unlink;
  - claims, orders and ordinal render from a real Go/PG seed;
  - a session without customers:read sees a disabled control with the reason.
- The INU05 calibration (`LC_INBOX_CALIBRATION=retain-thread`) must still fail exactly INU05.
- Also run `test-node.sh`, `check-gates.sh` and typecheck. Click sweep and visual lint run in CI.

## Roles
| Role | Who |
|---|---|
| Implementer | Codex-2 |
| Pre-push and independent review | Kimi K3 (privacy, deep) |
| PR, push and merge | integrator |
