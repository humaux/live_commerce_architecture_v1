# Unit LC-B3b — inbox read gaps: A13 buyer panel, A8 session/live_comment, A14 link version, A9 binding id (backend)

Status: FROZEN (integrator, 2026-10-07). Base `r3/integration` 35abffca. Migration **0165** (assigned by the integrator; 0163 settlement-resolve and
0164 W3-U4 are in flight; the R2 upgrade pin moves +1 at merge, and the integrator takes the union). Branch `unit/lc-b3b-buyer-panel`, worktree `.worktrees/lc-b3b-buyer-panel`.
Read first: `docs/delivery/AGENT-PREAMBLE.md` → `AGENTS.md` → `docs/delivery/PROCESS.md` → this file → `contracts/live-console-v1.md` §3.2, §3.6, §3.7, §11 rows A8, A9, A13 and A14, §12 → current code
`internal/inbox/read.go` (BuyerPanel ~L262 returns empty claims/orders "until LC-B4"; LC-B4 never filled them), `internal/httpapi/inbox.go`, migrations 0119, 0128 and 0060.

## Why
LC-U2b (the inbox page and BuyerPanel) found the frozen A13/A8/A9 still stubbed. A live seller needs to see a buyer's claims and orders next to the conversation. That is SHOPLINE parity and the
core of the 直播控台 right column (LC-U2a reuses BuyerPanel).

## Integrator rulings (these override the prose)
- **Identity link is ONLY `inbox.bundle_peers`** (written when a private reply succeeds, §3.7) plus the explicit A14 `linked_customer_id`. `claims.bundles.owner_id` never feeds the panel,
  a suggestion or a prefill (I09). No name matching, ever.
- **A13 fields, `?conversation_id=`:** the bundles = the store's bundles whose `bundle_peers.peer_key` equals this conversation's peer (same app/object/asset). `?bundle_id=`: that bundle only.
  - `claims`: `[{session_id, offer_id, keyword, quantity}]` of those bundles' accepted claims, newest session first, at most 50.
  - `claim_total_minor`: the sum of quantity × the unit price the claim recorded (no recomputation from the current catalogue).
  - `orders` (only with `orders:read`; otherwise omit the key entirely): orders created from those bundles (claim checkout or the A16 for-buyer order), newest first, at most 20,
    `{order_id, number, state, total_minor, created_at}`. `number` uses the existing `LC-…` rule.
  - `purchase_ordinal`: the count of those orders that are not CANCELLED and have reached CONFIRMED (paid or COD-confirmed). 0 when none. The UI shows 第 N 次購買.
  - `display_name`: only from data the conversation already holds (the existing encrypted display copy, if 0119 has one). Otherwise omit it. Never from a bundle or an order.
  - `auto_reply`: `{send_state}` of the newest automated private-reply operation for those bundles, or omitted.
- **A8:**
  - `session_id` must filter (conversations with a bundle_peers link to a bundle of that session).
  - `filter=live_comment` must return the bundle-only rows of live comments in scope (`bundle_id`, no `conversation_id`), as §11 already promises.
  - Add `link_version` to each item.
- **A9:** add `link_version` (for A14's `expected_version`) and `binding_id` (so the UI matches the capability row exactly). Both are additive fields.
- Additive only: no rename or removal of existing fields, and no change to permissions. Cross-tenant/store → 404 (LCN03). Responses stay no-store.

## Implementation shape
- One SECURITY DEFINER read function per need, or an extension of the existing definers via CREATE OR REPLACE with every attribute repeated (`SECURITY DEFINER SET search_path=pg_catalog`,
  OWNER/REVOKE/GRANT/COMMENT exactly as before). Access is checked at the start (`identity.resolve_access`), and every query is filtered by tenant and store. Bounded results.
- Go: `internal/inbox/read.go` fills the DTO. `internal/httpapi/inbox.go` keeps the route shapes, and passes the orders:read check to decide whether `orders` is included.
- Contract: a §11 A13 note recording the semantics above (amendment "LC-B3b"), plus the A8 and A9 additive fields.

## Gates (red → green; REAL_PG via `bash scripts/dev/test-focused.sh '<regex>'`)
- New `tests/foundation/live_console_buyer_panel_test.go`:
  - peer-linked claims and orders appear; an unlinked conversation shows none;
  - a bundle whose `owner_id` matches but has no bundle_peers row shows NONE (I09);
  - cross-store and cross-tenant give 404;
  - orders are omitted without orders:read; purchase_ordinal excludes CANCELLED and unconfirmed orders;
  - bounds hold (51 claims → 50);
  - A8 session_id filter and live_comment rows; A9 link_version / binding_id; an A14 CAS with the exposed link_version succeeds and a stale one gets 409;
  - leak scan: sentinel names and PSIDs are absent from every response field not specified above.
- The existing inbox, live_console_* and LCN tests stay green. The R2 upgrade pin +1. Full G07 (`release-gate.sh --strict --only G07`) is mandatory (migration + definer).
- No browser mode (backend only). LC-U2b's follow-up enables the UI.

## Evidence / roles
REAL_PG + MOCK. Implementer: Qwen (backend lane), committing after every green step. Finisher/verifier: Sonnet. Independent review: Opus (privacy). PR into r3/integration, merged by the
integrator after the required check and `review/independent` are green.
