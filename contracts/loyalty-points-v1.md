# Member points v1 — merchant-kept member records and an append-only points ledger

Status: **DESIGN, review-ready 2026-11-05; every LPG gate below is NOT_RUN.** No Go, SQL or TS exists for this
contract. Nothing here authorizes a real buyer erasure, a message to a buyer or any LIVE action.

Authority (read in this order):
- Owner requirement 2026-11-05 (binding, relayed by the integrator card for `unit/loyalty-points-contract`; merchant
  words from a chat): give a manual-order buyer member points; 「我不用算 我手动加」「我们后台自己看」
  「订购人的名字我用客户的名字」; some customers come from LINE with no Meta conversation. Owner decisions: points are
  **only a record**, redeemed offline and deducted by hand, **no discount integration, no gift catalogue, no expiry**.
- It **supersedes** the 2026-10-05 provisional owner ruling 「会员/积分/分润首版不做」 for *member points only*
  (分润 / referral commission and member tiers stay out of scope). Former locations of that ruling:
  `docs/delivery/units/INDEX-w3-w6.md` (owner provisional rulings), `docs/delivery/units/w6-01b-customer-tags-notes.md`.
- 架构 §14.1/§14.2 (identity graph: no merge on name/phone equality), §21.4 (deletion vs retention); invariants
  I01, I02, I04, I09, I11, I14, I23; gates G01, G09.
- [customers-billing-v1](customers-billing-v1.md) CD1/CD2 (customer = `buyer.owners`, no automatic identity edge),
  CD6–CD8 (synchronous privacy actions, tombstone replay); Amendment W6-01B (`customers:write`, tags/notes, the
  0063 `pg_get_constraintdef` re-derive pattern).
- [storefront-v2](storefront-v2.md) §G3 (manual order `POST orders/manual`: multi-transaction pipeline, then ONE
  merchant transaction = command receipt + `fulfillment.mark_order_merchant_manual` + audit `order.manual_created`)
  and its Amendment 2026-11-01 FEAT-MANUAL-DISCOUNT (0220, additive optional body key pattern).
- [merchant-identity-v1](merchant-identity-v1.md) Amendment W6-01B (permission catalogue and owner/admin bundles).

Where those conflict with this file, this file is wrong.

Migration reserved (provisional, integrator confirms the number): **`0223_loyalty_points.sql`** — follows 0222 on
`r3/integration` @ `b8305338b`. Forward-only, no destructive step, no post-River file.

## 0. Decisions

| # | Decision | Why / risk closed |
| --- | --- | --- |
| LP1 | **Member = a merchant-kept record `loyalty.members`, store-scoped**, separate from `buyer.owners`. Fields: `display_name` (required, trimmed NFC, 1..60 chars, no control chars), `contact_note` (optional free text ≤ 120, e.g. LINE id / IG handle / phone, never parsed or validated as an identifier), `notes` (optional ≤ 1000), `balance`, `version`, `created_by`, timestamps, `erased_at`. ≤ 20,000 members per store. | LINE customers have no Meta conversation and no `buyer.owners` row, so a member cannot be a customer (CD1). The merchant's own words: 「我们后台自己看」. |
| LP2 | **No automatic matching, merging or dedupe.** Same name or same contact note = two members if the merchant creates two. No unique index on name/contact. The create form shows a search of existing members first (UI convenience, not enforcement). No merge action in v1. | CD2, 架构 §14.1: name/phone equality is not evidence. A wrong merge would move another person's points. |
| LP3 | **No link between a member and a `buyer.owners` customer in v1** (not even a manual one). | A manual link would make member PII part of the customer erasure/export path (CD7) and of any future consent/audience reader; the owner asked for none of that. Upgrade signal §7. |
| LP4 | **Points ledger `loyalty.point_entries` is append-only.** Entry = member, `delta` (non-zero integer, \|delta\| ≤ 1,000,000), `reason` ∈ {`manual_order_award`, `manual_adjust`, `reversal`}, `order_id` (required for award and reversal, NULL for manual_adjust), `note` (required 1..200 for manual_adjust and reversal, NULL for award), `actor_principal_id` (server, from the bearer), `created_at` (server), `idempotency_key`. A trigger refuses UPDATE and DELETE, with the single LP11 redaction exception. Corrections are new entries. | Owner: points are a record; an editable ledger cannot be audited. |
| LP5 | **Balance is stored on the member row (`balance bigint NOT NULL CHECK (balance >= 0 AND balance <= 1e12)`) and changes only inside the same transaction as the entry insert**, after `SELECT … FOR UPDATE` of that member row. Every ledger writer is one SECURITY DEFINER function that takes the lock first. Invariant: `balance = SUM(delta)` per member (gate LPG01 checks it). | Two staff deducting at once serialize on the row lock: the second sees the first's balance, so no lost write; the CHECK makes a negative balance impossible even if a writer forgot the precheck (I14). One row lock per member, no store-wide lock. |
| LP6 | **Deductions refuse, never clamp:** a write that would make the balance negative fails `409 points_insufficient` with nothing written. | Silent clamping would record a deduction the merchant did not make. |
| LP7 | **Manual-order award = optional body key `member_award:{member_id, points}` on `POST orders/manual`**, `points` integer 1..1,000,000, empty by default, typed by the merchant, never computed. The award entry is written **in the same merchant transaction that writes the command receipt, `mark_order_merchant_manual` and `order.manual_created`** (§G3), not in `begin_hold`. One award per order: `UNIQUE (tenant_id, store_id, order_id) WHERE reason='manual_order_award'`. | §G3's pipeline is several transactions by design; the final merchant transaction is the only one that is the "manual order" commit. Putting the award there keeps award ⇔ source=merchant_manual atomic (I04). See deviation D1. |
| LP8 | **Award idempotency rides on the order's Idempotency-Key:** `member_award` is part of the normalized request hashed by `command.Run`. Same key + same body = same order, same award (no second entry); same key + different `member_award` (including added/removed) = `409 idempotency_conflict`. The entry's own `idempotency_key` = `'order:' \|\| order_id`. | Same rule as the FEAT-MANUAL-DISCOUNT key (I02). |
| LP9 | **Order cancel, expiry and refund never touch points.** The order page offers 「撤回本單積分」: `POST members/orders/{order_id}/reversal` writes `reason='reversal'`, `delta = -points`, where `points` defaults to the order's remaining award (award − earlier reversals) and may be lowered; total reversals of an order can never exceed its award (`409 reversal_exceeds_award`), checked under the member row lock. A reversal also obeys LP6. | Owner: offline redemption means the merchant, not the system, knows whether points were already spent. |
| LP10 | **Manual adjust** (`POST members/{member_id}/adjustments`, `delta` ±, required note) is the only other write. A mistaken manual adjust is corrected by an opposite manual adjust with a note; `reversal` is reserved for order awards. | `reversal` needs a cap anchor (the award). See deviation D2. |
| LP11 | **Member erasure** (merchant-initiated, `customers:privacy`): in one transaction lock the member, set `display_name='[erased]'`, `contact_note=NULL`, `notes=NULL`, `erased_at=now()`, and set every non-NULL ledger `note` of that member to `'[erased]'` (the only UPDATE the ledger trigger allows: `note` → `'[erased]'` while the member's `erased_at` is set; numbers, reason, order id, actor and time are kept). An erased member accepts no writes (`409 member_erased`) and is hidden from the list unless `?include_erased=1`. Erasure is allowed at any balance. Tombstone = `erased_at` on the row; `loyalty.replay_member_erasures(p_members uuid[])` re-applies it after a restore from the external erased-id list (same procedure and same T20 gap as CD8). | Members are merchant-entered PII; ledger notes are free text that may carry names. Numbers stay so the store's records keep adding up; orders stay untouched (CD7 retention). |
| LP12 | **Member export** (`customers:privacy`): synchronous JSON of the member record + full ledger (CD6 style), audit `member.exported`. The buyer-facing privacy export/erasure of a `buyer.owners` customer is **unchanged**, because no member is linked to a customer (LP3). | A LINE customer asks the merchant, not the site; the merchant answers with this export. |
| LP13 | **Rights:** new permission **`customers:points`** = create/edit members, manual adjust, order award, order reversal. Reading members and ledgers = existing `customers:read`. Export/erase a member = existing `customers:privacy`. Default bundles: **owner and admin** get `customers:points` (backfilled for existing owner/admin staff); viewer, live_operator and fulfilment do not. Placing a manual order **with** `member_award` additionally needs `customers:points`, checked before the pipeline starts and re-checked in SQL inside the final transaction. | Same naming (`customers:<action>`) and same bundle rule as W6-01B `customers:write`. Points are value the merchant gives away, so they are not part of `inventory:reserve`. |
| LP14 | **Audit = the actor on every ledger entry** (no duplicate audit row per entry). Member create/edit/export/erase write audit rows `member.created`, `member.updated`, `member.exported`, `member.erased` with ids and counts only — never name, contact note, notes or ledger note. `order.manual_created.details` gains `member_award:{member_id, points}` (ids and numbers only). | Ledger is itself the audit trail; PII stays out of the audit table (I11). |
| LP15 | **Out of every other surface:** no buyer-facing UI, no buyer notification or DM, no ads/CAPI/audience/report use, no CSV import/export of members, no use by any job. Members are not customers for any metric. | Owner: points are only a record. Keeps I07/I22 consent paths unchanged. |
| LP16 | **Manual order only.** `POST orders/for-buyer` (live DM shortcut) does not accept `member_award` in v1 (unknown key = 400), even though it wraps the same `Place` pipeline. | Owner named 手工單. See open question Q3. |
| LP17 | **Order customer name is independent of the member.** The UI may prefill the order's `customer.name` from the chosen member's `display_name` (「订购人的名字我用客户的名字」), editable; the server stores no relation other than the award entry. Member erasure does not alter the order snapshot. | §G3 order snapshot is financial record (CD7). |

### Rejected alternatives

- **Automatic identity match/merge** (by name, phone in contact note, LINE id, Meta actor, claim link): CD2, 架构 §14.1/§14.2; LINE ids are never verified; a wrong merge moves another person's points.
- **Member = `buyer.owners` customer, or a mandatory link:** LINE customers have no owner row (CD1); a manual link is deferred, not rejected (LP3, §7).
- **Points rules / automatic calculation** (per NT$, per order, multipliers, tiers): owner 「我不用算 我手动加」; a rule engine would need refund pro-rating and versioning nobody asked for.
- **Expiry:** owner decision "no expiry"; expiry also needs a job and FIFO lot accounting (I23 growth) for no stated need.
- **Discount redemption at checkout / gift catalogue:** owner decision; it would turn a record into money in the quote (I05/I08) and touch payment/refund allocation.
- **Auto-reversal on order cancel/refund:** the merchant redeems offline; the system cannot know whether points were spent (LP9).
- **Balance computed as `SUM(delta)` on every read without a stored balance:** the list sorts/filters by balance and paginates; a stored balance under the same row lock is cheaper and the CHECK is the non-negative guarantee. `SUM` stays as the gate.
- **Advisory lock per store or SERIALIZABLE isolation:** a per-member row lock is enough (writes to one member are the only conflict) and matches CD4's lock-the-owner-row pattern.
- **UPDATE/DELETE of ledger entries for corrections:** destroys the record (LP4).
- **Clamping a deduction at 0:** records a deduction that did not happen (LP6).
- **Award inside `checkout.begin_hold`:** would put loyalty writes inside the checkout pool and its definer; the order is not yet `merchant_manual` there (deviation D1).
- **Reusing `customers:write`:** tag/note editors would gain the power to give away value; a separate right costs one catalogue entry.
- **Buyer-facing balance page / notification:** owner did not ask; needs consent and a buyer identity that LINE members do not have.
- **Hard delete of a member on erasure:** the ledger rows reference it and the store's totals must stay explainable (LP11).

### Deviations from the integrator design (each justified)

- **D1** "Same transaction as manual-order creation" is refined to *the final §G3 merchant transaction* (receipt + source mark + audit). The order row itself is created earlier by `begin_hold` in another pool; §G3 states the pipeline is not atomic. Consequence: an award never exists without `source=merchant_manual`, and a crash before the final transaction leaves neither (a retry with the same key writes both).
- **D2** `reversal` is limited to order awards (needs `order_id`); manual-adjust mistakes are corrected with an opposite manual adjust. The integrator text "corrections are reversal entries" is kept in spirit (append-only, never edit).
- **D3** Order award data is read from a new `GET members/orders/{order_id}` instead of being added to `GET orders/{order_id}`: the order detail projection is `orders:read`, member names are `customers:read`; a separate read keeps both authorities and does not re-open the order detail definer.
- **D4** Ledger "no UPDATE" has exactly one exception: erasure sets `note` to `'[erased]'` (LP11). Without it, free-text notes with names would survive erasure.
- **D5** `for-buyer` excluded (LP16); `member_award` accepted only on `orders/manual`.
- **D6** 架构.md never contained the literal 「会员/积分 首版不做」; the ruling lived in `docs/delivery/units/INDEX-w3-w6.md`. 架构 §2.1 gets a pointer and the index line is updated.
- **D7** `scripts/check_packet.py` hard-codes the release task count; it is raised 43 → 45 for the two new tasks ("revise explicitly", same as #206).

## 1. Schema sketch (`0223_loyalty_points.sql`, design only)

```sql
CREATE SCHEMA loyalty;
CREATE TABLE loyalty.members (
  tenant_id uuid NOT NULL, store_id uuid NOT NULL, id uuid NOT NULL DEFAULT gen_random_uuid(),
  display_name text NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 60),   -- trimmed, NFC, no control chars (definer validates)
  contact_note text CHECK (char_length(contact_note) <= 120),
  notes text CHECK (char_length(notes) <= 1000),
  balance bigint NOT NULL DEFAULT 0 CHECK (balance >= 0 AND balance <= 1000000000000),
  version integer NOT NULL DEFAULT 1,
  created_by uuid NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
  erased_at timestamptz,
  create_key text NOT NULL,                                 -- Idempotency-Key of the create call
  PRIMARY KEY (tenant_id, store_id, id),
  UNIQUE (tenant_id, store_id, create_key),
  FOREIGN KEY (tenant_id, store_id) REFERENCES control.stores(tenant_id, id)   -- exact store FK per existing pattern
);
CREATE INDEX members_list ON loyalty.members (tenant_id, store_id, created_at DESC, id DESC) WHERE erased_at IS NULL;

CREATE TABLE loyalty.point_entries (
  tenant_id uuid NOT NULL, store_id uuid NOT NULL, id uuid NOT NULL DEFAULT gen_random_uuid(),
  member_id uuid NOT NULL,
  delta integer NOT NULL CHECK (delta <> 0 AND delta BETWEEN -1000000 AND 1000000),
  reason text NOT NULL CHECK (reason IN ('manual_order_award','manual_adjust','reversal')),
  order_id uuid,
  note text,
  actor_principal_id uuid NOT NULL,
  idempotency_key text NOT NULL,
  request_sha256 bytea NOT NULL,                            -- normalized request hash for replay vs conflict
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, store_id, id),
  FOREIGN KEY (tenant_id, store_id, member_id) REFERENCES loyalty.members,
  FOREIGN KEY (tenant_id, store_id, order_id) REFERENCES checkout.orders(tenant_id, store_id, id),  -- exact order FK per existing pattern
  UNIQUE (tenant_id, store_id, idempotency_key),
  CHECK ((reason = 'manual_order_award' AND delta > 0 AND order_id IS NOT NULL AND note IS NULL)
      OR (reason = 'manual_adjust' AND order_id IS NULL AND char_length(note) BETWEEN 1 AND 200)
      OR (reason = 'reversal' AND delta < 0 AND order_id IS NOT NULL AND char_length(note) BETWEEN 1 AND 200)
      OR note = '[erased]')
);
CREATE UNIQUE INDEX point_entries_one_award ON loyalty.point_entries (tenant_id, store_id, order_id) WHERE reason = 'manual_order_award';
CREATE INDEX point_entries_member ON loyalty.point_entries (tenant_id, store_id, member_id, created_at DESC, id DESC);
CREATE INDEX point_entries_order ON loyalty.point_entries (tenant_id, store_id, order_id) WHERE order_id IS NOT NULL;
-- trigger loyalty.point_entries_immutable: BEFORE UPDATE OR DELETE → raise, except UPDATE whose only change is
-- note → '[erased]' while the member's erased_at IS NOT NULL (LP11). TRUNCATE revoked from every role.
```

- Both tables ENABLE + FORCE RLS with the GUC tenant/store policy; **no direct GRANT** to any login role (like `customers.*`); all access through definers.
- Definers: owner `commerce_privacy_writer` (same owner as the customers privacy/tag definers), SECURITY DEFINER, `search_path=pg_catalog`, EXECUTE only `commerce_runtime`; worker roles get nothing (worker-authority-split WAS02 pin).
- `identity.store_grants` permission CHECK re-derived (0063 `pg_get_constraintdef` pattern) with `customers:points`; owner/admin bundle seed + backfill.

| Definer | Permission | Rule |
| --- | --- | --- |
| `loyalty.create_member(hash, store, display_name, contact_note, notes, key)` | `customers:points` | key replay/conflict (I02); ≤ 20,000 members → `limit_reached`; audit `member.created` |
| `loyalty.update_member(hash, store, member, expected_version, fields…)` | `customers:points` | lock row; erased → `member_erased`; version CAS → `conflict`; audit `member.updated` (field names only) |
| `loyalty.adjust_points(hash, store, member, delta, note, key)` | `customers:points` | key lookup first (same hash → stored entry; other → `idempotency_conflict`); lock member FOR UPDATE; erased → `member_erased`; `balance + delta < 0` → `points_insufficient`; insert entry, update balance |
| `loyalty.award_manual_order(hash, store, order, member, points)` | `customers:points` (+ caller already holds `inventory:reserve`) | called only inside the §G3 final transaction after `mark_order_merchant_manual`; order must be `source='merchant_manual'` of this store; lock member; erased/unknown → `member_unavailable`; entry key `order:<order_id>`; existing award for the order with same member+points → no-op (replay), different → `idempotency_conflict` |
| `loyalty.reverse_order_award(hash, store, order, points NULL, note, key)` | `customers:points` | key lookup; find the award (none → `not_found`); lock its member; `points` NULL = remaining; `points > remaining` → `reversal_exceeds_award`; LP6 |
| `loyalty.read_members(hash, store, q, include_erased, cursor, limit)` / `read_member(…, ledger_cursor)` / `read_order_award(hash, store, order)` | `customers:read` | RLS + permission re-check (0063 pattern) |
| `loyalty.export_member(hash, store, member, key)` | `customers:privacy` | returns JSON, audit `member.exported` |
| `loyalty.erase_member(hash, store, member, key)` | `customers:privacy` | LP11; replay by key returns stored summary; audit `member.erased` |
| `loyalty.replay_member_erasures(p_members uuid[])` | ops only | LP11 restore procedure |

SQLSTATE: reuse PT403/PT404/PT409 with the refusal code in the message detail, as the customers definers do; **no new SQLSTATE**.

## 2. HTTP API (prefix `/v1/admin/stores/{store_id}`; admin BFF `/api/stores/{store}/members/**`)

Common: tenant and store come only from the authenticated merchant session and the path store it may access (I01); a body
`tenant_id`, `store_id`, `actor`, `balance`, `created_at` or any unknown key at any depth = `400 invalid_json`. JSON ≤ 16 KiB,
`Content-Type: application/json` else `415 json_required`. Every POST/PATCH requires `Idempotency-Key` (8..128
`[A-Za-z0-9_.:-]`); same key + same normalized body = the first answer with HTTP 200; other body = `409 idempotency_conflict`.
No automatic retry; `503 retry_later` means retry with the same key. Names, contact notes and notes never appear in a URL,
query string, log line or metric label — the search term `q` is the one exception in the query string and must be dropped
from access logs (the route is registered as a redacted-query route; gate LPG09).

| Route | Permission | Request → response |
| --- | --- | --- |
| `GET /members?q=&cursor=&limit=&include_erased=` | `customers:read` | `q` 1..60, case-insensitive substring over `display_name` and `contact_note`; `limit` 1..100 (default 50); keyset cursor on `(created_at DESC, id DESC)`, opaque, bound to `q`/`include_erased` (changed filter = 422). → `{"members":[{"id","display_name","contact_note","balance","updated_at","erased":false}],"next_cursor":string\|null}` |
| `POST /members` | `customers:points` | `{"display_name","contact_note":string\|null,"notes":string\|null}` → 201 `{member}` |
| `GET /members/{member_id}?ledger_cursor=` | `customers:read` | → `{"member":{"id","display_name","contact_note","notes","balance","version","created_at","erased_at"},"entries":[{"id","delta","reason","order_id","note","actor":{"principal_id","display_name"},"created_at"}],"next_ledger_cursor"}` (50 per page, newest first) |
| `PATCH /members/{member_id}` | `customers:points` | `{"expected_version","display_name"?,"contact_note"?,"notes"?}` (explicit null clears optional fields) → 200 `{member}` |
| `POST /members/{member_id}/adjustments` | `customers:points` | `{"delta": int ≠ 0,"note": "1..200"}` → 201 `{"entry","balance"}` |
| `POST /members/{member_id}/export` | `customers:privacy` | → 200 JSON attachment `{"member","entries"}` |
| `POST /members/{member_id}/erasure` | `customers:privacy` | `{}` → 200 `{"member_id","erased_at","entries_redacted"}` |
| `GET /members/orders/{order_id}` | `customers:read` | → `{"award":{"member_id","member_display_name","points","reversed","remaining","created_at"}\|null}` |
| `POST /members/orders/{order_id}/reversal` | `customers:points` | `{"points": int\|null,"note":"1..200"}` → 201 `{"entry","balance","remaining"}` |
| `POST /orders/manual` (existing §G3) | existing + `customers:points` when `member_award` is present | additive optional `"member_award":{"member_id","points"}`; success adds `"member_award":{"member_id","points","entry_id"}` |

Refusal codes (existing style, additive): `401 unauthorized`; `403 forbidden` (missing permission — including
`customers:points` on a manual order with `member_award`, refused **before** the pipeline runs, so no order is placed);
`404 not_found` (unknown/foreign member, order or award — one opaque answer); `409 idempotency_conflict`; `409 conflict`
(version); `409 points_insufficient`; `409 reversal_exceeds_award`; `409 member_erased`; `409 member_unavailable`
(manual order: member erased or unknown when the final transaction runs; see §7 limit L3); `422 invalid_request`;
`422 limit_reached`; `400 invalid_json`; `415 json_required`; `503 retry_later`.

**Integrator follow-ups (not done here):** register the nine `/members**` routes and the `orders/manual` body key in the
Go route table + `scripts/dev/contractdrift/baseline.json` (CI-DRIFT ratchet) and, if the dispatcher closure changes,
`contracts/route-table-dispatchers.json` (independent review required by `docs/engineering/contractdrift.md`); add the
BFF route rows. No `contracts/*openapi.json` currently covers customers or manual-order routes, so **no OpenAPI entry is
required unless the integrator decides to start one (UNKNOWN, Q4)**. Confirm migration number 0223.

## 3. Admin UI (TypeScript strict, zh-TW / zh-CN / en)

- Nav: 「會員」 under 顧客 (exact shell position follows the signed frames / Figma; UNKNOWN until a frame exists, Q5).
- 會員 list: search (name or contact note), balance column, cursor 「載入更多」, 「新增會員」.
- Member detail: header (name, contact note, notes, balance), ledger table (time, ± points, reason label, order link, note, staff), 「加點 / 扣點」 dialog (sign toggle, integer, required 原因), edit, export, erase (confirm dialog that states numbers stay and names are removed).
- Manual-order page (owned by the PARITY-MANUAL-ORDER lane): optional 「會員」 picker (search + 「新增會員」 inline) and 「本單贈送積分」 integer field, both empty by default; choosing a member may prefill 訂購人姓名 (editable, LP17); hidden entirely without `customers:points`.
- Order detail: 「本單積分」 panel (member, awarded, reversed) with 「撤回本單積分」 (one click = remaining, editable number, required note); hidden without `customers:read`; button hidden without `customers:points`.
- No buyer-facing page, notification, ads or report surface. No PII in client logs, URLs (member id only) or analytics.

## 4. Invariants (this contract)

- **LPI1** For every member, `balance = SUM(delta)` and `balance ≥ 0` at every commit (LP5).
- **LPI2** Ledger rows are never deleted; the only permitted update is `note → '[erased]'` for an erased member (LP4/LP11).
- **LPI3** For every order, `SUM(-delta of reversals) ≤ award` and at most one award (LP7/LP9).
- **LPI4** An award entry exists iff the order's final §G3 transaction committed with `member_award` (LP7).
- **LPI5** No code path creates, merges or links members automatically; no member is linked to `buyer.owners` (LP2/LP3).
- **LPI6** Tenant/store always server-derived; foreign scope = 404 (I01).
- **LPI7** No member PII in logs, audit details, URLs (except the dropped `q`), fixtures (synthetic names only), metrics (I11).

## 5. Acceptance gates (each starts RED; each needs a recorded "inject fault → red → fix → green")

Backend (REAL_PG, `tests/foundation/loyalty_points_test.go` + Go unit tests):

| Gate | Test | Inject fault → red → fix → green |
| --- | --- | --- |
| LPG01 | 50 concurrent deductions of 10 on a balance of 100 from two staff sessions: exactly 10 succeed, 40 `points_insufficient`, final balance 0, `balance = SUM(delta)` | remove `FOR UPDATE` and the CHECK → red (negative or lost write) → restore → green |
| LPG02 | Manual order with `member_award`: award + `source=merchant_manual` + receipt commit together; fail the final tx after the award insert → no award, source stays `storefront`; retry same key → both | move the award call into its own transaction → red → fix → green |
| LPG03 | Reversal cap: award 100, reverse 60, reverse 50 → `reversal_exceeds_award`; concurrent reversals of 60 + 60 → one wins; default reversal = remaining | drop the remaining check → red → fix → green |
| LPG04 | Idempotent replay: same key + body on adjust, reversal, create, manual order → one entry, HTTP 200, same answer; changed body → 409 | make the key lookup run after the insert / omit `member_award` from the hash → red → fix → green |
| LPG05 | Rights: viewer, live_operator, fulfilment and a role with only `inventory:reserve`+`catalog:read` get 403 on every write; a manual order with `member_award` without `customers:points` is 403 **and places no order**; foreign store 404 | grant `customers:points` to viewer in the seed / check permission after the pipeline → red → fix → green |
| LPG06 | Erasure: name, contact note, notes and every ledger note redacted, numbers/reason/order/actor kept, writes after erasure `member_erased`, replay by key same summary, `replay_member_erasures` restores redaction | skip ledger note redaction → red → fix → green |
| LPG07 | No auto-merge: two members with identical name and contact note stay two; a manual order whose customer name/phone equals a member's contact never creates or links a member; no FK/column to `buyer.owners` exists (catalog query) | add a "find by name" upsert in create → red → fix → green |
| LPG08 | Ledger immutability: UPDATE delta, DELETE, TRUNCATE from `commerce_runtime` and the definer owner fail; balance column cannot be written outside definers | drop the trigger → red → fix → green |
| LPG09 | PII hygiene: log capture of every route contains no name/contact/note/`q` value; audit `details` contain ids/numbers only; fixtures use synthetic `測試會員` names | log the request body in the handler → red → fix → green |
| LPG10 | ACL pins: function list/EXECUTE grants in `customers_billing_schema_test.go`-style pin, worker roles have no EXECUTE (WAS02), permission CHECK includes `customers:points` | grant EXECUTE to a worker role → red → fix → green |

UI (Vitest + Playwright, real browser, real clicks against the dev stack with a seeded REAL_PG store; 375 px and desktop; three locales):

| Gate | Flow | Inject fault → red → fix → green |
| --- | --- | --- |
| LPU01 | 會員 → 新增會員 → search finds it by contact note → open → 加點 50 with 原因 → balance 50 and a ledger row with staff name | send `delta` without the note / skip refresh → red → fix → green |
| LPU02 | 扣點 more than balance → visible 「積分不足」 error, balance unchanged | swallow the 409 → red → fix → green |
| LPU03 | 手工單: pick member, type 30 points, submit → order created; order page shows 本單積分 30; 撤回本單積分 → remaining 0, balance −30; second click shows cap error | send `member_award` with the field empty → red (400) → fix (omit key when empty) → green |
| LPU04 | Staff without `customers:points`: no 會員 picker/points field on 手工單, no 加點/扣點/撤回 buttons; direct API call 403 | render buttons by role name instead of permission → red → fix → green |
| LPU05 | Erase member → confirm → list hides it, detail shows `[erased]`, ledger notes `[erased]`, numbers unchanged | skip the confirm step → red → fix → green |
| LPU06 | URL and console contain only member ids, never names (assert on `page.url()` and console messages) | put name in the query string → red → fix → green |

Evidence labels: DESIGN until code exists; MOCK / REAL_PG / browser results are reported separately; independent review by
test_worker + security_reviewer; author PASS is not acceptance.

## 6. Unit split

- `LP-1` backend (`docs/delivery/units/loyalty-points-backend.md`), then `LP-2` admin UI
  (`docs/delivery/units/loyalty-points-admin-ui.md`). LP-2 starts after LP-1 is merged and its routes frozen, and after the
  PARITY-MANUAL-ORDER page lane (owner of the 手工單 page) is merged.

## 7. Known limits and upgrade signals

- **L1** Search is a substring scan over ≤ 20,000 members per store (no trigram index). Upgrade: p95 list > 300 ms on REAL_PG or a store near the cap → `pg_trgm` index (first confirm the extension is available on the managed PG).
- **L2** No member merge. Upgrade: merchants report duplicates they want combined → a merchant-initiated merge that writes a transfer pair of entries (never automatic).
- **L3** Race: if a member is erased between the pre-check and the §G3 final transaction, that transaction fails `member_unavailable`; the order is already placed with `source=storefront` and retries with the same key fail the same way. The merchant cancels that order. Millisecond window; upgrade if it is ever observed.
- **L4** No link to `buyer.owners` (LP3). Upgrade: owner asks to see points on a site customer → a manual, revocable link with its own erasure/export coverage and an amendment to CD7.
- **L5** `for-buyer` cannot award (LP16). Upgrade: owner asks for points from the live DM shortcut.
- **L6** Restore replay depends on the external erased-id list (T20), same as CD8.
- **L7** No expiry, tiers, rules, redemption or buyer view — owner decisions, not limits; reopen only on an owner request.

## 8. Open questions (UNKNOWN — not decided here)

- **Q1** Default bundle for a future customer-service role: none exists (W6-01B note); owner/admin only for now.
- **Q2** Legal retention for an erased member's numbers (U08): kept indefinitely in this draft.
- **Q3** Should the live-console 代建訂單 shortcut (`for-buyer`) award points? Owner named only 手工單.
- **Q4** Whether merchant admin routes should start getting OpenAPI entries (none do today).
- **Q5** Exact nav position and frames for 會員 (no Figma frame located for this feature).
- **Q6** Final migration number (0223 provisional; another lane may claim it first).
