# OC-12 — definition of "unread" (written before build)

Card: contracts/tasks.json OC-12 · owner choice: contracts/owner-choices-20261026-v1.md OC-Q12 (Option B) ·
base origin/r3/integration @ 1360be187.

## Authority
The definition already exists and is frozen: contracts/live-console-v1.md §3.6 ("Unread = `last_inbound_seq > read_seq`;
store-level, not per staff") and OPEN-9 (RESOLVED, reviewer counter adopted). Migration 0119 implements it
(`inbox.conversation_state.read_seq` / `last_inbound_seq`, `inbox.mark_read`, `social.unread_conversation_count`), and
`social.list_conversations` (0165) already returns the per-row `unread` flag. OC-12 adds no new state; it only adds the
filter value `unread` to the existing A8 read.

## Definition
| Question | Answer |
|---|---|
| Read marker | `inbox.conversation_state.read_seq` (bigint, default 0): the highest inbound `server_seq` acknowledged. |
| Unread | `COALESCE(last_inbound_seq, 0) > read_seq`. A conversation with no buyer message (`last_inbound_seq` NULL) is never unread. |
| Whose read | **Store-level**, one marker per (tenant, store, conversation), shared by all staff. No per-staff state. |
| Who clears it | `POST …/conversations/{cid}/read` → `inbox.mark_read`: `read_seq = LEAST(GREATEST(read_seq, requested), last_inbound_seq)`, honoured **only** for principals holding `inbox:reply`. An `inbox:read`-only principal gets the unchanged value back and the thread stays unread for the assignee. Opening a thread (A9) does not clear it. |
| New message resets it | The AFTER INSERT trigger `inbox.advance_conversation_state` on `social.messages` raises `last_inbound_seq` to the new `server_seq`, so `last_inbound_seq > read_seq` again: the conversation is unread again without touching `read_seq`. Outbound sends, echoes, reactions, read receipts never advance `last_inbound_seq` (§3.6). |
| Scope | Tenant and store come only from the GUCs `platform.WithScope` sets from the authenticated session + `identity.resolve_access`; the request carries no tenant id and a store id the principal holds no grant on is refused before any read. The definer filters `c.tenant_id = app.tenant_id AND c.store_id = app.store_id` and requires `inbox:read`. |
| Bundle-only rows | Live-comment / link-pending bundle rows have no read marker and are always `unread=false`; filter `unread` therefore lists conversations only (no bundle rows appended). |
| `unread_total` | Unchanged: `social.unread_conversation_count()` over the same predicate, same scope. |

## Why this minimal option
- The data model already stores exactly this state; a per-staff read table would contradict OPEN-9 and the frozen §3.6.
- One migration (`0204_inbox_unread.sql`, number assigned by the integrator) re-creates `social.list_conversations`
  with the same signature, result type, owner (`commerce_meta_writer`) and EXECUTE grant (`commerce_runtime`), adding
  `'unread'` to the filter allowlist and the predicate above. No table, column, index, role or permission change.
- A migration is needed because the filter allowlist lives in the SECURITY DEFINER body (`invalid_filter` PT400).

## Known limits
- Index: the predicate rides the existing `(tenant_id, store_id)` scan used by every other filter; no partial index is
  added (same cost profile as `unreplied`). Upgrade signal: A8 p95 regression on stores with > 10k conversations.
