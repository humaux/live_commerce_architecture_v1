# Unit LC-U3: 幫他建立訂單 create-order drawer (W2-U3), opened from the console and the inbox BuyerPanel

Status: FROZEN (integrator, 2026-10-08). Base: the LC-U2a branch head. LC-U2a is K3 PASS, and only its P2s are pending. When LC-U2a merges, merge r3/integration in.
Branch: `unit/lc-u3-create-order-drawer`. Worktree: `.worktrees/lc-u3-create-order-drawer`.
Read first, in order: `docs/delivery/AGENT-PREAMBLE.md` → `AGENTS.md` → `docs/delivery/PROCESS.md` → this file → `contracts/live-console-v1.md`. Sections to read:
- §5, the whole of it: 5.1 flow, 5.2 reused vs new, 5.3 card and live price, and the integrator OPEN-13 money-path ruling;
- §6 capability states;
- §11 rows A13, A15 and A16;
- §12 privacy;
- §13.2 row LCN12.
Then read the current code: `apps/admin/components/ManualOrder.tsx` (the existing manual-order form), `BuyerPanel.tsx`, `CommentStream.tsx`, `LiveConsole.tsx`, the BFF catch-all, and the `orders/manual` client.

## Why
During a live sale, the seller builds an order for a buyer straight from the console or the inbox. Claimed SKUs and quantities are prefilled, the seller picks delivery (7-ELEVEN suggested only from an explicitly linked customer's last order) and payment, and can send the payment link by DM. This is SHOPLINE parity and the last big console piece. The backend (LC-B6: A15 order-prefill, A16 orders/for-buyer) is merged.

## Scope (UI and BFF only; no product Go, migration or OpenAPI changes)
1. **Extract the shared form.** Pull the item lines, delivery and payment form out of `ManualOrder.tsx` into a shared component, without changing ManualOrder's behaviour: `--browser-manual-order` stays green, and so do the existing manual-order tests.
2. **`CreateOrderDrawer.tsx`.** It opens from the BuyerPanel (inbox and console) and from a console comment/bundle via 「幫他建立訂單」.
   - **Prefill:** call A15 (`?bundle_id=` or `?conversation_id=`).
   - Customer and delivery come ONLY from an explicit A14 link, shown as 「已手動連結」; otherwise the fields stay empty. Never fill them from `owner_id` or names (I09).
   - Quantities ± per line, where 0 removes the line. Delivery and payment come from the reused `orders/manual/options`.
3. **Live price is fail-closed.** When the prefill says `live_price_eligible=false`, show 「直播價不適用代建訂單，請改傳認領連結」 BEFORE confirming, and show the catalog price. Never display a live price that the server will not apply. The Quote is the only price.
4. **Submit (A16).**
   - Exact body keys, and an `Idempotency-Key` generated once per drawer attempt and reused on retry. On UNKNOWN or network failure, retry only with the same key, never a new one.
   - `409 bundle_already_ordered` → show 「查看訂單」 linking to the order, and never offer a second order.
   - `409 bundle_buyer_mismatch` → coded copy; nothing was created.
   - **Result:**
     - `live_price` applied or not applied, with the reason;
     - `send` state, or not_sent with the reason window_closed or no_conversation;
     - when not sent, offer copy-link (clipboard only; the URL never goes into the DOM, logs or storage).
5. **Permissions.** inventory:reserve is required, and the live price additionally needs live:manage. Without them, show disabled controls with the reasons (§6).
6. **BFF.** Add only the exact A15 and A16 paths, methods, query keys and body keys, plus the Idempotency-Key header passthrough. Keep CSRF and origin rules.

## Never
- No buyer PII in logs, URLs or storage. Responses stay no-store.
- No identity guessing.
- No price computed on the client.
- No auto-retry with a new key.
- Do not touch label print (LC-U4) or live settings (W3-U2).

## Gates (red → green)
- **Node:**
  - prefill mapping;
  - an unlinked conversation gives empty customer and delivery;
  - eligibility copy;
  - Idempotency-Key reuse;
  - every coded error;
  - BFF seam tests for exact keys and the header.
- **Browser:** extend `--browser-manual-order` (or the console mode) over a real Go/PG seed, zh-TW/zh-CN/en at 1440/390:
  1. open from the BuyerPanel with a linked customer → prefilled delivery → create → order exists, send not_sent(window_closed) → copy-link works;
  2. an unlinked conversation → empty fields, and the live-price-not-applied copy shows before confirming;
  3. a second attempt on the same bundle → 409 → 「查看訂單」;
  4. a network failure on submit → retry with the same key → exactly one order (verify in PG);
  5. ManualOrder unchanged.
- **Calibration:** an `LC_DRAWER_CALIBRATION=new-key-on-retry` fault must fail exactly the single-order case.
- Also `test-node.sh`, `check-gates.sh` and typecheck. Click sweep and visual lint run in CI.

## Roles
| Role | Who |
|---|---|
| Implementer | Codex-3 |
| Pre-push and independent review | Kimi K3 (money and privacy, deep) |
| PR, push and merge | integrator, after LC-U2a merges |
