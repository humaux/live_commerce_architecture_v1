# Member points v1 — merchant-kept member records and an append-only points ledger

Status: **DESIGN, review round 3 2026-11-05; every LPG/LPU gate below is NOT_RUN.** No Go, SQL or TS exists for this
contract. Nothing here authorizes a real buyer erasure, a message to a buyer or any LIVE action.
Review history: round 1 `output/issue-107/review-549-55fc66b3f-fable.md` (REQUEST_CHANGES, 3 P1 + 9 P2) closed in place,
ledger §9 (round 1 and round 2 tables).

Authority (read in this order):
- Owner requirement 2026-11-05 (binding, relayed by the integrator card for `unit/loyalty-points-contract`; merchant
  words from a chat): give a manual-order buyer member points; 「我不用算 我手动加」「我们后台自己看」
  「订购人的名字我用客户的名字」; some customers come from LINE with no Meta conversation. Owner decisions: points are
  **only a record**, redeemed offline and deducted by hand, **no discount integration, no gift catalogue, no expiry**.
- It **supersedes** the 2026-10-05 provisional owner ruling 「会员/积分/分润首版不做」 for *member points only*
  (分润 / referral commission and member tiers stay out of scope). Former locations of that ruling:
  `docs/delivery/units/INDEX-w3-w6.md` (owner provisional rulings), `docs/delivery/units/w6-01b-customer-tags-notes.md`.
- Integrator ruling 2026-11-05 on review P2-7 (accepted, supersedes the integrator's first design): no separate reversal
  semantics; deductions are manual adjusts, optionally referencing an order.
- 架构 §14.1/§14.2 (identity graph: no merge on name/phone equality), §21.4 (deletion vs retention); invariants
  I01, I02, I04, I09, I11, I14, I23; gates G01, G09.
- [customers-billing-v1](customers-billing-v1.md) CD1/CD2 (customer = `buyer.owners`, no automatic identity edge),
  CD6–CD8 (synchronous privacy actions, tombstone replay); Amendment W6-01B (`customers:write`, the 0063
  `pg_get_constraintdef` re-derive pattern).
- [storefront-v2](storefront-v2.md) §G3 (manual order `POST orders/manual`: multi-transaction pipeline, then ONE
  merchant transaction = command receipt + `fulfillment.mark_order_merchant_manual` + audit `order.manual_created`)
  and its Amendment 2026-11-01 FEAT-MANUAL-DISCOUNT (0220, additive optional body key pattern).
- `internal/command` receipts: `ops.command_results` keyed by (tenant, store, operation, idempotency_key) with
  `request_hash` and stored `response`, written in the caller's transaction (`internal/command/command.go:32-66`).
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
| LP4 | **Points ledger `loyalty.point_entries` is append-only.** Entry = member, `delta` (non-zero integer, \|delta\| ≤ 1,000,000), `reason` ∈ {`manual_order_award`, `manual_adjust`}, `order_id` (required for award; optional reference for manual_adjust), `note` (NULL for award; required 1..200 for manual_adjust), `actor_principal_id` (server, from the bearer), `created_at` (server). A trigger refuses UPDATE and DELETE, with the single LP11 redaction exception. Corrections are new entries. No per-member entry cap (round-2 cap withdrawn, delta review P2-C). | Owner: points are a record; an editable ledger cannot be audited. Entries are hand-typed, so growth is bounded by staff effort; the export is bounded instead (LP12). |
| LP5 | **Balance is stored on the member row (`balance bigint NOT NULL CHECK (balance >= 0 AND balance <= 1e12)`) and changes only inside the same transaction as the entry insert**, after `SELECT … FOR UPDATE` of that member row. Every ledger writer is one SECURITY DEFINER function that takes the lock first and pre-checks both bounds, so neither CHECK is ever reached in normal operation (they are the backstop). Invariant: `balance = SUM(delta)` per member (gate LPG01). | Two staff deducting at once serialize on the row lock: the second sees the first's balance, so it is refused with a precise code instead of a CHECK violation; the CHECK makes a negative balance impossible even if a writer forgot the precheck (I14). |
| LP6 | **Bounds refuse, never clamp:** a write that would make the balance negative fails `409 points_insufficient`; a write that would exceed 1e12 fails `422 limit_reached`; nothing is written. | Silent clamping would record a deduction the merchant did not make; an unmapped CHECK would be a 500. |
| LP7 | **Manual-order award = optional body key `member_award:{member_id, points}` on `POST orders/manual`**, `points` integer 1..1,000,000, empty by default, typed by the merchant, never computed. **Before any pipeline write** (before the buyer capability, cart, quote or `begin_hold`), Go runs `loyalty.precheck_award` in its own read transaction: `customers:points` held, member exists in this store, not erased, `balance + points ≤ 1e12`. The pre-check runs **only when the probe transaction found no receipt for the key** (`!replayed` in `Place`, `internal/merchanttools/manual.go:363-379`), so a same-key replay always returns the stored receipt even if the member was erased since. Failure = the matching refusal (`403 forbidden`, `404 not_found`, `409 member_erased`, `422 limit_reached`) and **no order is placed**. The award entry itself is written in the **final §G3 merchant transaction** (receipt + `mark_order_merchant_manual` + `order.manual_created`) by `loyalty.try_award_manual_order`, which **never raises on an award problem** (LP18). One award per order: `UNIQUE (tenant_id, store_id, order_id) WHERE reason='manual_order_award'`. | Validation errors (wrong id, foreign id, already erased member, missing right) are ordinary input errors and must stop before stock is touched (review P1-1). The final transaction stays the only "manual order" commit (I04). |
| LP8 | **Award idempotency rides on the order's command receipt:** `member_award` is part of the request hashed by `command.Run` (`merchanttools.order.manual`). Same key + same body = the stored receipt, including the award outcome and `entry_id`; same key + different `member_award` (including added/removed) = `409 idempotency_conflict`. The award entry has no key of its own. | Same rule as the FEAT-MANUAL-DISCOUNT key (I02); one key layer (review P1-3). |
| LP9 | **WITHDRAWN 2026-11-05 (review P2-7, integrator ruling).** Was: separate `reversal` reason with an award cap, `reverse_order_award`, `reversal_exceeds_award`, `GET members/orders/{order_id}`. Replaced by LP10 + LP19. | Owner: deductions are by hand. |
| LP10 | **Manual adjust is the only write besides the award:** `POST members/{member_id}/adjustments {delta ±, note, order_id?}`. `order_id`, when given, must be an order of this store (else `404 not_found`); it is a reference only — no cap, no link to the award amount, no check that the member was awarded on that order. A mistaken adjust is corrected by an opposite adjust with a note. LP5/LP6 apply. | One write semantics instead of two (review P2-7). |
| LP11 | **Member erasure** (merchant-initiated, `customers:privacy`): in one transaction lock the member, set `display_name='[erased]'`, `contact_note=NULL`, `notes=NULL`, `erased_at=now()`, and set every non-NULL ledger `note` of that member to `'[erased]'` (the only UPDATE the ledger trigger allows: `note` from a non-NULL value to `'[erased]'`, only while the member's `erased_at` is set, every other column unchanged). Numbers, reason, order id, actor and time are kept. **Orders are not changed:** a name the merchant copied into an order's 訂購人 (LP17) stays in that order's snapshot (financial record, CD7); the confirm dialog says so. Stored command receipts hold no member PII (§1), so erasure has nothing to scrub there. An erased member accepts no writes (`409 member_erased`) and is hidden from search unless `include_erased:true`. Erasure is allowed at any balance. Tombstone = `erased_at` on the row; `loyalty.replay_member_erasures(p_members uuid[])` re-applies it after a restore from the external erased-id list (same procedure and same T20 gap as CD8). | Members are merchant-entered PII; ledger notes are free text that may carry names. Numbers stay so the store's records keep adding up; orders stay untouched (CD7 retention). |
| LP12 | **Member export** (`customers:privacy`): synchronous JSON of the member record + full ledger, CD6 style, with the customers export bound: a body over **1 MiB** is refused `409 export_too_large` and the transaction rolls back with no audit row (precedent `internal/buyerhttp/privacy.go:179`, customers-billing-v1 export ≤ 1 MiB; ≈ 1,400 entries even if every note is 200 CJK characters (≤ 600 bytes + ~150 bytes of fields), far beyond a hand-kept ledger); audit `member.exported`. Export entries carry `actor_principal_id` only, **never staff display names**. The buyer-facing privacy export/erasure of a `buyer.owners` customer is unchanged, because no member is linked to a customer (LP3). | A LINE customer asks the merchant, not the site; the export may leave the store, so it carries no staff PII (review P2-6/P2-9; delta P2-C). |
| LP13 | **Rights:** new permission **`customers:points`** = create/edit members, manual adjust, order award. Reading members and ledgers = existing `customers:read`. Export/erase a member = existing `customers:privacy`. Default bundles: **owner and admin** get `customers:points` (backfilled for existing owner/admin staff); viewer, live_operator and fulfilment do not. A manual order with `member_award` additionally needs `customers:points`, checked by LP7's pre-check and re-checked in the final transaction (LP18). | Same naming (`customers:<action>`) and same bundle rule as W6-01B `customers:write`. Points are value the merchant gives away, so they are not part of `inventory:reserve`. |
| LP14 | **Audit = the actor on every ledger entry** (no duplicate audit row per entry). Member create/edit/export/erase write audit rows `member.created`, `member.updated`, `member.exported`, `member.erased` with ids and counts only — never name, contact note, notes or ledger note. `order.manual_created.details` gains `member_award:{member_id, points, state}` (ids, numbers, state only). | Ledger is itself the audit trail; PII stays out of the audit table (I11). |
| LP15 | **Out of every other surface:** no buyer-facing UI, no buyer notification or DM, no ads/CAPI/audience/report use, no CSV import/export of members, no use by any job. Members are not customers for any metric. | Owner: points are only a record. Keeps I07/I22 consent paths unchanged. |
| LP16 | **Manual order only.** `POST orders/for-buyer` (live DM shortcut) refuses `member_award` (`400 invalid_json`, its own body allow-list in `order_for_buyer.go`) in v1, even though it wraps the same `Place` pipeline. Gate LPG11. | Owner named 手工單. See open question Q3. |
| LP17 | **Order customer name is independent of the member.** The UI may prefill the order's `customer.name` from the chosen member's `display_name` (「订购人的名字我用客户的名字」), editable; the server stores no relation other than the award entry. Member erasure does not alter the order snapshot. | §G3 order snapshot is financial record (CD7). |
| LP18 | **Mid-flow race (member erased, `customers:points` revoked, or a bound newly crossed between the pre-check and the final transaction): the order still completes; the award is recorded as not given.** `loyalty.try_award_manual_order` locks the member row, re-checks right / member / bounds and returns `{state:"awarded", entry_id}` or `{state:"not_awarded", reason:"member_erased"\|"member_unavailable"\|"forbidden"\|"limit_reached"}` **without raising**, so the final transaction always commits receipt + source mark + audit. The 201 answer and every replay carry `member_award.state`; the UI shows 「積分未贈送：<reason>」 and the merchant may add points by a manual adjust referencing the order. Erasure and the award serialize on the member row lock: an erasure that commits first makes the award `not_awarded`; an award that commits first is then redacted like any entry. Revoking `inventory:reserve` mid-flow is the existing §G3 behaviour, unchanged. | An award problem can never leave a placed order stuck as `source=storefront` (review P1-1, P2-1): the only failure left in the final transaction is the pre-existing §G3 one. |
| LP19 | **Order page 「扣回本單積分」 = a prefilled manual deduct**: the order page reads the ledger entries referencing that order (`GET members/entries?order_id=`), and the button opens the LP10 adjust dialog with member = the award's member, `delta = -award points` (editable), `order_id` = this order, note required. No cap logic; LP6 still refuses going below 0. | The UI needs the award to prefill; a generic ledger filter is the smallest read (integrator: keep a read only if the UI needs it). |

### Rejected alternatives

- **Automatic identity match/merge** (by name, phone in contact note, LINE id, Meta actor, claim link): CD2, 架构 §14.1/§14.2; LINE ids are never verified; a wrong merge moves another person's points.
- **Member = `buyer.owners` customer, or a mandatory link:** LINE customers have no owner row (CD1); a manual link is deferred, not rejected (LP3, §7).
- **Points rules / automatic calculation** (per NT$, per order, multipliers, tiers): owner 「我不用算 我手动加」; a rule engine would need refund pro-rating and versioning nobody asked for.
- **Expiry:** owner decision "no expiry"; expiry also needs a job and FIFO lot accounting (I23 growth) for no stated need.
- **Discount redemption at checkout / gift catalogue:** owner decision; it would turn a record into money in the quote (I05/I08) and touch payment/refund allocation.
- **Auto-reversal on order cancel/refund:** the merchant redeems offline; the system cannot know whether points were spent.
- **Separate `reversal` reason with an award cap (round-1 LP9/D2/D3, withdrawn):** a second write semantics, one definer, one route, one refusal code and one gate for something the owner does by hand; a referenced manual deduct (LP10/LP19) keeps the record.
- **Validating the member only inside the final transaction (round-1 LP7):** a wrong or erased member placed an order with stock reserved and then failed forever on retry (review P1-1).
- **Raising inside the final transaction on a race:** aborts receipt + source mark, leaving a stuck `storefront` order; LP18 records `not_awarded` instead.
- **Per-table idempotency keys (`create_key`, `idempotency_key`, `request_sha256`) next to command receipts (round-1):** two key layers drift apart and the table keys could not detect a changed body (review P1-3); `command.Run` is the one layer.
- **GET with `q` in the query string (round-1):** the search term (a name) reaches the BFF log, proxies and browser history (review P2-6); search is a POST body.
- **Balance computed as `SUM(delta)` on every read without a stored balance:** the list shows balance and paginates; a stored balance under the same row lock is cheaper and the CHECK is the non-negative guarantee. `SUM` stays as the gate.
- **Advisory lock per store or SERIALIZABLE isolation:** a per-member row lock is enough and matches CD4's lock-the-owner-row pattern.
- **UPDATE/DELETE of ledger entries for corrections:** destroys the record (LP4).
- **Clamping a deduction at 0:** records a deduction that did not happen (LP6).
- **Award inside `checkout.begin_hold`:** would put loyalty writes inside the checkout pool and its definer; the order is not yet `merchant_manual` there.
- **Reusing `customers:write`:** tag/note editors would gain the power to give away value; a separate right costs one catalogue entry.
- **Buyer-facing balance page / notification:** owner did not ask; needs consent and a buyer identity that LINE members do not have.
- **Hard delete of a member on erasure:** the ledger rows reference it and the store's totals must stay explainable (LP11).

### Deviations from the integrator design (each justified)

- **D1** "Same transaction as manual-order creation" is refined to *the final §G3 merchant transaction* (receipt + source mark + audit). The order row itself is created earlier by `begin_hold` in another pool; §G3 states the pipeline is not atomic. A crash after `begin_hold` and before the final transaction leaves a placed order with `source=storefront`, reserved (or, for pay-at-pickup/COD, committed) stock, no receipt and no award — the existing §G3 state; a retry with the same key replays the earlier steps and runs the final transaction, writing source mark, receipt and award together. An award never exists without `source=merchant_manual`.
- **D2** WITHDRAWN (round 1: reversal limited to order awards). Reversal semantics removed by the integrator's P2-7 ruling.
- **D3** WITHDRAWN (round 1: separate `GET members/orders/{order_id}`). Replaced by the generic ledger filter of LP19, needed by the UI.
- **D4** Ledger "no UPDATE" has exactly one exception: erasure sets a non-NULL `note` to `'[erased]'` (LP11). Without it, free-text notes with names would survive erasure.
- **D5** `for-buyer` excluded (LP16); `member_award` accepted only on `orders/manual`.
- **D6** 架构.md never contained the literal 「会员/积分 首版不做」; the ruling lived in `docs/delivery/units/INDEX-w3-w6.md`. 架构 §2.1 gets a pointer and the index line is updated.
- **D7** `scripts/check_packet.py` hard-codes the release task count; it is raised 43 → 45 for the two new tasks ("revise explicitly", same as #206).
- **D8** An award race does not abort the order (LP18): the integrator's "same transaction" stays true for the award *outcome* (recorded in the same receipt), not for the award *entry*, which is absent when `state=not_awarded`.

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
  PRIMARY KEY (tenant_id, store_id, id),
  FOREIGN KEY (tenant_id, store_id) REFERENCES control.stores(tenant_id, id)   -- exact store FK per existing pattern
);
CREATE INDEX members_list ON loyalty.members (tenant_id, store_id, created_at DESC, id DESC) WHERE erased_at IS NULL;

CREATE TABLE loyalty.point_entries (
  tenant_id uuid NOT NULL, store_id uuid NOT NULL, id uuid NOT NULL DEFAULT gen_random_uuid(),
  member_id uuid NOT NULL,
  delta integer NOT NULL CHECK (delta <> 0 AND delta BETWEEN -1000000 AND 1000000),
  reason text NOT NULL CHECK (reason IN ('manual_order_award','manual_adjust')),
  order_id uuid,
  note text,
  actor_principal_id uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, store_id, id),
  FOREIGN KEY (tenant_id, store_id, member_id) REFERENCES loyalty.members,
  FOREIGN KEY (tenant_id, store_id, order_id) REFERENCES checkout.orders(tenant_id, store_id, id),  -- exact order FK per existing pattern
  CHECK ((reason = 'manual_order_award' AND delta > 0 AND order_id IS NOT NULL AND note IS NULL)
      OR (reason = 'manual_adjust' AND char_length(note) BETWEEN 1 AND 200))   -- '[erased]' (8 chars) satisfies the adjust branch; no other escape
);
CREATE UNIQUE INDEX point_entries_one_award ON loyalty.point_entries (tenant_id, store_id, order_id) WHERE reason = 'manual_order_award';
CREATE INDEX point_entries_member ON loyalty.point_entries (tenant_id, store_id, member_id, created_at DESC, id DESC);
CREATE INDEX point_entries_order ON loyalty.point_entries (tenant_id, store_id, order_id) WHERE order_id IS NOT NULL;
-- trigger loyalty.point_entries_immutable: BEFORE UPDATE OR DELETE → raise, except an UPDATE where every column
-- but note is unchanged, OLD.note IS NOT NULL, NEW.note = '[erased]' and the member's erased_at IS NOT NULL (LP11).
-- TRUNCATE revoked from every role.
```

- Both tables ENABLE + FORCE RLS with the GUC tenant/store policy; **no direct GRANT** to any login role (like `customers.*`); all access through definers.
- Definers: owner `commerce_privacy_writer` (same owner as the customers privacy/tag definers), SECURITY DEFINER, `search_path=pg_catalog`, EXECUTE only `commerce_runtime`; worker roles get nothing (worker-authority-split WAS02 pin).
- `identity.store_grants` permission CHECK re-derived (0063 `pg_get_constraintdef` pattern) with `customers:points`; owner/admin bundle seed + backfill.
- **Idempotency (one layer):** every write route runs inside `command.Run` in the same transaction as its definer call; the receipt `ops.command_results(tenant, store, operation, idempotency_key, request_hash, response)` is the only key store. Operations: `loyalty.member.create`, `loyalty.member.update`, `loyalty.points.adjust`, `loyalty.member.export`, `loyalty.member.erase`, plus the existing `merchanttools.order.manual`. The operation is part of the key, so keys of different operations never collide (review P2-3). Same key + same normalized request = the stored `response` (HTTP 200); same key + different request = `409 idempotency_conflict` (I02). Concurrent same-key requests (double click) serialize on `pg_advisory_xact_lock(scope|operation|key)` taken **before** the receipt lookup and held to commit (`internal/command/command.go:45-57`); the second then finds the committed receipt and replays (`:64-76`), or a rolled-back first leaves no receipt and the second runs once. **Stored responses carry ids, numbers, versions and states only** — never `display_name`, `contact_note`, `notes` or a ledger `note` (W6-01B note-receipt precedent): create/update store `{member_id, version}`, adjust `{entry_id, balance}`, erase `{member_id, erased_at, entries_redacted}`, export `{member_id, entries_exported, exported_at}`, manual order `member_award{member_id, points, state, entry_id or reason}`. The HTTP body is built from a read of the current rows in the same transaction, on the first call and on replay alike, so a replay after erasure shows `[erased]`. `request_hash` (sha256 of the request) is retained like every other receipt (§G3/W6-01B precedent). Search writes no receipt.

| Definer | Permission | Rule |
| --- | --- | --- |
| `loyalty.create_member(hash, store, display_name, contact_note, notes)` | `customers:points` | ≤ 20,000 members → `limit_reached`; audit `member.created` |
| `loyalty.update_member(hash, store, member, expected_version, fields…)` | `customers:points` | lock row; erased → `member_erased`; version CAS → `conflict`; audit `member.updated` (field names only) |
| `loyalty.adjust_points(hash, store, member, delta, note, order_id)` | `customers:points` | lock member FOR UPDATE; erased → `member_erased`; foreign/unknown order → `not_found`; `balance + delta < 0` → `points_insufficient`; `> 1e12` → `limit_reached`; insert entry, update balance |
| `loyalty.precheck_award(hash, store, member, points)` | `customers:points` | read-only, LP7; raises the matching refusal |
| `loyalty.try_award_manual_order(hash, store, order, member, points) → jsonb` | re-checks `customers:points` | only inside the §G3 final transaction after `mark_order_merchant_manual`; order must be `source='merchant_manual'` of this store; lock member; returns LP18 state, never raises on right/member/bounds; existing award for the order (resume) → returns it |
| `loyalty.read_members(hash, store, q, include_erased, cursor, limit)` / `read_member(…, ledger_cursor)` / `read_order_entries(hash, store, order)` | `customers:read` | RLS + permission re-check (0063 pattern) |
| `loyalty.export_member(hash, store, member)` | `customers:privacy` | returns JSON (LP12); over 1 MiB → `export_too_large`; audit `member.exported` |
| `loyalty.erase_member(hash, store, member) → jsonb` | `customers:privacy` | LP11; returns `{member_id, erased_at, entries_redacted}`, stored as the receipt response; audit `member.erased` |
| `loyalty.replay_member_erasures(p_members uuid[])` | ops only | LP11 restore procedure |

SQLSTATE: reuse the existing PT4xx conventions (PT403/PT404/PT409 and the existing invalid-input mapping) with the refusal code in the message detail, as the customers definers do; **no new SQLSTATE**. A raw 23514 from a backstop CHECK is a bug (gate LPG01b).

## 2. HTTP API (prefix `/v1/admin/stores/{store_id}`; admin BFF `/api/stores/{store}/members/**`)

Common: tenant and store come only from the authenticated merchant session and the path store it may access (I01); a body
`tenant_id`, `store_id`, `actor`, `balance`, `created_at` or any unknown key at any depth = `400 invalid_json`. JSON ≤ 16 KiB,
`Content-Type: application/json` else `415 json_required`. Every write (POST/PATCH except `members/search`) requires
`Idempotency-Key` (8..128 `[A-Za-z0-9_.:-]`) handled by `command.Run` (§1). No automatic retry; `503 retry_later` means
retry with the same key. **Names, contact notes, notes and search terms never appear in a URL, query string, log line or
metric label**; URLs carry only ids and opaque cursors (gates LPG09, LPU06).

| Route | Permission | Request → response |
| --- | --- | --- |
| `POST /members/search` (read-only, writes nothing, no Idempotency-Key) | `customers:read` | `{"q":string\|null (1..60),"include_erased":bool,"cursor":string\|null,"limit":1..100}`; case-insensitive substring over `display_name` and `contact_note`; keyset cursor on `(created_at DESC, id DESC)`, opaque, bound to `q`/`include_erased` (changed filter = 422). → `{"members":[{"id","display_name","contact_note","balance","updated_at","erased":false}],"next_cursor":string\|null}` |
| `POST /members` | `customers:points` | `{"display_name","contact_note":string\|null,"notes":string\|null}` → 201 `{member}` |
| `GET /members/{member_id}?ledger_cursor=` | `customers:read` | → `{"member":{"id","display_name","contact_note","notes","balance","version","created_at","erased_at"},"entries":[{"id","delta","reason","order_id","note","actor":{"principal_id","display_name"},"created_at"}],"next_ledger_cursor"}` (50 per page, newest first; staff display names are for the admin screen only) |
| `PATCH /members/{member_id}` | `customers:points` | `{"expected_version","display_name"?,"contact_note"?,"notes"?}` (explicit null clears optional fields) → 200 `{member}` |
| `POST /members/{member_id}/adjustments` | `customers:points` | `{"delta": int ≠ 0,"note": "1..200","order_id": uuid\|null}` → 201 `{"entry","balance"}` |
| `GET /members/entries?order_id={order_id}` | `customers:read` | → `{"entries":[{"id","member_id","member_display_name","delta","reason","note","created_at"}]}` (entries referencing that order, oldest first, at most 100; above that the first 100 with `"truncated":true`) |
| `POST /members/{member_id}/export` | `customers:privacy` | → 200 JSON attachment `{"member","entries":[{"id","delta","reason","order_id","note","actor_principal_id","created_at"}]}` (no staff names, LP12); 409 `export_too_large` over 1 MiB; same-key replay returns a fresh export of the same member and writes no second audit row (§1: bodies are always built from current rows) |
| `POST /members/{member_id}/erasure` | `customers:privacy` | `{}` → 200 `{"member_id","erased_at","entries_redacted"}`; replay = the stored response |
| `POST /orders/manual` (existing §G3) | existing + `customers:points` when `member_award` is present | additive optional `"member_award":{"member_id","points"}`; success adds `"member_award":{"member_id","points","state":"awarded","entry_id"}` or `{"member_id","points","state":"not_awarded","reason"}` (LP18) |

Refusal codes (existing style, additive): `401 unauthorized`; `403 forbidden` (missing permission — including
`customers:points` on a manual order with `member_award`, refused by the pre-check before any pipeline write);
`404 not_found` (unknown/foreign member or order — one opaque answer); `409 idempotency_conflict`; `409 conflict`
(version); `409 points_insufficient`; `409 member_erased`; `409 export_too_large`; `422 invalid_request`; `422 limit_reached`; `400 invalid_json`;
`415 json_required`; `503 retry_later`. `member_unavailable` exists only as an LP18 `reason`, never as an HTTP refusal.

**Integrator follow-ups (not done here):** register the eight `/members**` routes and the `orders/manual` body key in the
Go route table + `scripts/dev/contractdrift/baseline.json` (CI-DRIFT ratchet) and, if the dispatcher closure changes,
`contracts/route-table-dispatchers.json` (independent review required by `docs/engineering/contractdrift.md`); add the
BFF route rows. No `contracts/*openapi.json` currently covers customers or manual-order routes, so **no OpenAPI entry is
required unless the integrator decides to start one (UNKNOWN, Q4)**. Confirm migration number 0223.

## 3. Admin UI (TypeScript strict, zh-TW / zh-CN / en)

- Nav: 「會員」 under 顧客 (exact shell position follows the signed frames / Figma; UNKNOWN until a frame exists, Q5).
- 會員 list: search box (sent as a POST body, never in the URL), balance column, cursor 「載入更多」, 「新增會員」.
- Member detail: header (name, contact note, notes, balance), ledger table (time, ± points, reason label, order link, note, staff), 「加點 / 扣點」 dialog (sign toggle, integer, required 原因, optional order reference), edit, export, erase (confirm dialog copy: 「點數紀錄保留，會員姓名、聯絡備註與備註會移除；已建立訂單上的訂購人姓名不會變更。」 and the zh-CN/en equivalents).
- Manual-order page (owned by the PARITY-MANUAL-ORDER lane): optional 「會員」 picker (search + 「新增會員」 inline) and 「本單贈送積分」 integer field, both empty by default; choosing a member may prefill 訂購人姓名 (editable, LP17); hidden entirely without `customers:points`. A `not_awarded` answer shows 「積分未贈送」 with the reason next to the created order.
- Order detail: 「本單積分」 panel from `GET members/entries?order_id=` (member, entries) with 「扣回本單積分」 (LP19: prefilled manual deduct, editable amount, required note); panel hidden without `customers:read`, button hidden without `customers:points`.
- No buyer-facing page, notification, ads or report surface. No PII in client logs, URLs (ids only) or analytics.

## 4. Invariants (this contract)

- **LPI1** For every member, `balance = SUM(delta)`, and `0 ≤ balance ≤ 1e12` at every commit (LP5).
- **LPI2** Ledger rows are never deleted; the only permitted update is a non-NULL `note → '[erased]'` for an erased member (LP4/LP11).
- **LPI3** At most one award per order (LP7).
- **LPI4** An award entry exists iff the order's final §G3 transaction committed with `member_award.state='awarded'`; a committed manual order with `member_award` always has a recorded state (LP7/LP18).
- **LPI5** No code path creates, merges or links members automatically; no member is linked to `buyer.owners` (LP2/LP3).
- **LPI6** Tenant/store always server-derived; foreign scope = 404 (I01).
- **LPI7** No member PII in logs (Go, BFF, proxy), audit details, URLs, fixtures (synthetic names only), metrics or export staff fields (I11).
- **LPI8** One idempotency layer: every write is a `command.Run` receipt in the write's own transaction (I02).

## 5. Acceptance gates (each starts RED; each needs a recorded "inject fault → red → fix → green")

Backend (REAL_PG, `tests/foundation/loyalty_points_test.go` + Go unit tests). Every failed call asserts its exact refusal code.

| Gate | Test | Inject fault → red → fix → green |
| --- | --- | --- |
| LPG01a | 50 concurrent deductions of 10 on a balance of 100 from two staff sessions: exactly 10 succeed, 40 `409 points_insufficient`, final balance 0, `balance = SUM(delta)` | remove the `balance + delta < 0` pre-check (keep lock + CHECK) → red (23514/500 instead of 409) → restore → green |
| LPG01b | Same load; and an adjust that would pass 1e12 → `422 limit_reached`; no 23514 ever reaches HTTP | injection 1: remove `FOR UPDATE`, compute the new balance in Go (read, then write the absolute value) and drop the CHECK → red (negative balance or lost write) → restore → green; injection 2: remove the 1e12 pre-check (CHECK kept) → red (500 instead of 422) → restore → green |
| LPG02 | Manual order with `member_award`: award + `source=merchant_manual` + receipt commit together; fail the final tx after the award insert → no award, source stays `storefront`; retry same key → both | move the award call into its own transaction → red → fix → green |
| LPG03 | Pre-check before writes: unknown member, foreign-store member, erased member, missing `customers:points`, `balance + points > 1e12` → 404 / 404 / 409 `member_erased` / 403 / 422, and **zero** rows in buyer capability, cart, quote, hold and orders for that key | move the pre-check after `begin_hold` → red (order rows exist) → fix → green |
| LPG04 | Idempotent replay through receipts: same key + body on create, update, adjust, erase, export, manual order → one effect, HTTP 200, same response (manual order: same `entry_id` and `state` after a lost response); changed body → 409; the same key string used for `loyalty.points.adjust` and `loyalty.member.create` → two independent receipts; **two concurrent adjusts with one key** (double click) → exactly one entry, both answers 200/201 with the same `entry_id`; **manual order awarded, member erased, same key replayed** → 200 stored receipt with `state=awarded` (pre-check skipped on replay, P2-B) | injection 1: hash only part of the request (omit `member_award` / `order_id`) → red → fix → green; injection 2: run `precheck_award` before the receipt probe → red (409 `member_erased` on replay) → fix → green; injection 3: do the receipt lookup outside `command.Run` (no advisory lock) → red (two entries or 23505) → fix → green |
| LPG05 | Rights: viewer, live_operator, fulfilment and a role with only `inventory:reserve`+`catalog:read` get 403 on every write; read-only roles without `customers:read` get 403 on search/detail/entries; foreign store 404 | grant `customers:points` to viewer in the seed → red → fix → green |
| LPG06 | Erasure: name, contact note, notes and every non-NULL ledger note redacted, numbers/reason/order/actor kept, writes after erasure `member_erased`, replay by key returns the stored summary, `replay_member_erasures` restores redaction; **no `ops.command_results.response` of any `loyalty.*` operation or of `merchanttools.order.manual` contains the member's name, contact note, notes or a ledger note** (scan all receipts of the store for the fixture strings), and a replayed create/update/adjust after erasure shows `[erased]`; the member's past order snapshot is unchanged | injection 1: skip ledger note redaction → red → fix → green; injection 2: store the full `{member}` as the create receipt response → red (fixture name found in receipts) → fix → green |
| LPG06r | Race (LP18): (a) erase commits while a manual order sits between pre-check and final tx → order commits, `state=not_awarded`, `reason=member_erased`, no entry; (b) `customers:points` revoked in the same window → `not_awarded`, `forbidden`; (c) award commits first, erase second → entry exists with numbers, member redacted; in all three the order ends `source=merchant_manual` with a receipt | make `try_award_manual_order` raise instead of returning → red (order stuck `storefront`) → fix → green |
| LPG07 | No auto-merge: two members with identical name and contact note stay two; a manual order whose customer name/phone equals a member's contact never creates or links a member; no FK/column to `buyer.owners` exists (catalog query) | add a "find by name" upsert in create → red → fix → green |
| LPG08 | Ledger immutability and shape: from `commerce_runtime` and the definer owner, `UPDATE delta`, `DELETE`, `TRUNCATE` fail; `UPDATE note='[erased]'` on a **non-erased** member's row fails; `UPDATE delta` on an **erased** member's row fails; `INSERT reason='manual_order_award', delta=-500, order_id=NULL, note='[erased]'` fails the CHECK; balance not writable outside definers | re-add `OR note = '[erased]'` to the CHECK / loosen the trigger to "member erased" only → red → fix → green |
| LPG09 | PII hygiene: log capture of the Go API **and the admin BFF server** for every route contains no name/contact/note/search term; audit `details` contain ids/numbers/state only; export entries have no staff display name; fixtures use synthetic `測試會員` names | log the request body in the handler / put `q` in the query string → red → fix → green |
| LPG10 | ACL pins: function list/EXECUTE grants in `customers_billing_schema_test.go`-style pin, worker roles have no EXECUTE (WAS02), permission CHECK includes `customers:points` | grant EXECUTE to a worker role → red → fix → green |
| LPG11 | `POST orders/for-buyer` with `member_award` → `400 invalid_json`, no order, no entry (LP16) | add `member_award` to `forBuyerFields` (`internal/httpapi/merchanttools.go:39`, the pre-decode allow-list used by `studioUniqueJSON`) **and** to `ForBuyerInput` → red (order placed) → remove → green |

UI (Playwright real clicks per AGENTS.md「UI 验收」, real browser against the dev stack with a seeded REAL_PG store; 375 px and desktop; three locales; results persist after reload; click ledger delivered):

| Gate | Flow | Inject fault → red → fix → green |
| --- | --- | --- |
| LPU01 | 會員 → 新增會員 → search finds it by contact note → open → 加點 50 with 原因 → balance 50 and a ledger row with staff name; reload keeps it | send `delta` without the note / skip refresh → red → fix → green |
| LPU02 | 扣點 more than balance → visible 「積分不足」 error, balance unchanged | swallow the 409 → red → fix → green |
| LPU03 | 手工單: pick member, type 30 points, submit → order created; order page 本單積分 shows +30; 扣回本單積分 opens prefilled −30 with this order, submit with note → balance 0 (Δ −30), ledger row links the order; a second 扣回 beyond balance shows 「積分不足」 | send `member_award` with the field empty → red (400) → fix (omit key when empty) → green |
| LPU04 | Staff without `customers:points`: no 會員 picker/points field on 手工單, no 加點/扣點/扣回 buttons; direct API call 403 | render buttons by role name instead of permission → red → fix → green |
| LPU05 | Erase member → confirm → search hides it, detail shows `[erased]`, ledger notes `[erased]`, numbers unchanged; the confirm dialog shows the 訂購人 sentence; the member's past order still shows its 訂購人 name | skip the confirm step → red → fix → green |
| LPU06 | Every page URL (`page.url()`, history) and console contain only ids and opaque cursors, never names or search terms | put the search term in the URL → red → fix → green |
| LPU07 | `not_awarded` answer (stubbed only at the BFF boundary, documented as fixture prep) renders 「積分未贈送」 with the reason beside the created order | ignore `state` → red → fix → green |

Evidence labels: DESIGN until code exists; MOCK / REAL_PG / browser results are reported separately; independent review by
test_worker + security_reviewer; author PASS is not acceptance.

## 6. Unit split

- `LP-1` backend (`docs/delivery/units/loyalty-points-backend.md`), then `LP-2` admin UI
  (`docs/delivery/units/loyalty-points-admin-ui.md`). LP-2 starts after LP-1 is merged and its routes frozen, and after the
  PARITY-MANUAL-ORDER page lane (owner of the 手工單 page) is merged.

## 7. Known limits and upgrade signals

- **L1** Search is a substring scan over ≤ 20,000 members per store (no trigram index). Upgrade: p95 search > 300 ms on REAL_PG or a store near the cap → `pg_trgm` index (first confirm the extension is available on the managed PG).
- **L2** No member merge. Upgrade: merchants report duplicates they want combined → a merchant-initiated merge that writes a transfer pair of entries (never automatic).
- **L3** Mid-flow race (LP18) gives `not_awarded`, not a retry; the merchant re-adds points by a referenced adjust. Upgrade: if `not_awarded` is ever observed in production logs (counted by state, no PII).
- **L4** No link to `buyer.owners` (LP3). Upgrade: owner asks to see points on a site customer → a manual, revocable link with its own erasure/export coverage and an amendment to CD7.
- **L5** `for-buyer` cannot award (LP16). Upgrade: owner asks for points from the live DM shortcut.
- **L6** Restore replay depends on the external erased-id list (T20), same as CD8.
- **L7** Member export over 1 MiB is refused `export_too_large`. Upgrade: the first such refusal → paged export (several files by entry range).
- **L8** No expiry, tiers, rules, redemption, reversal cap or buyer view — owner decisions, not limits; reopen only on an owner request.

## 8. Open questions (UNKNOWN — not decided here)

- **Q1** Default bundle for a future customer-service role: none exists (W6-01B note); owner/admin only for now.
- **Q2** Legal retention for an erased member's numbers (U08): kept indefinitely in this draft.
- **Q3** Should the live-console 代建訂單 shortcut (`for-buyer`) award points? Owner named only 手工單.
- **Q4** Whether merchant admin routes should start getting OpenAPI entries (none do today).
- **Q5** Exact nav position and frames for 會員 (no Figma frame located for this feature).
- **Q6** Final migration number (0223 provisional; another lane may claim it first).
- **Q7** Command receipt retention for the `loyalty.*` operations: this draft assumes the existing `ops.command_results` lifetime; if receipts are purged, a later replay of an old key becomes a new request (same as every other merchant command).

## 9. Review ledger (round 1, Fable 5.1 @ 55fc66b3f)

| Finding | Resolution |
| --- | --- |
| P1-1 member validated after order placed | LP7 pre-check before any pipeline write; LP18 race records `not_awarded` without raising; gates LPG03, LPG06r |
| P1-2 CHECK escape hatch | `OR note='[erased]'` removed; trigger allows only non-NULL note → `[erased]` on an erased member; LPG08 pins |
| P1-3 idempotency storage vs I02 | per-table keys removed; all writes through `command.Run` receipts with request hash + stored response (§1, LPI8) |
| P2-1 rights revoked mid-flow | LP18 `forbidden` → `not_awarded`; LPG06r(b) |
| P2-2 1e12 CHECK unmapped | pre-check → `422 limit_reached` in adjust, pre-check and award (LP6); LPG01b |
| P2-3 key namespace collision | receipts keyed by operation; award has no own key (LP8); LPG04 |
| P2-4 LPG01 bundled injections | split into LPG01a/LPG01b with code assertions |
| P2-5 missing gates | LPG11 (for-buyer 400), LPG06r (erase race), LPG08 (note on non-erased, delta on erased), LPG04 (`entry_id` replay) |
| P2-6 `q` in URL; staff names in export | `POST /members/search`; BFF log capture in LPG09, URL/history in LPU06; export has `actor_principal_id` only (LP12) |
| P2-7 over-built reversal | ACCEPTED: LP9/D2/D3 withdrawn; LP10 optional order reference; LP19 prefilled deduct; LPG03 (old) replaced |
| P2-8 D1 "leaves neither" | D1 restated: placed `storefront` order with reserved/committed stock, no receipt, no award |
| P2-9 unbounded export | export bound (LP12, L7); round-2 entry cap withdrawn by delta P2-C |

### Round 2 (Fable 5.1 delta @ e0d3e9686, APPROVE) and supplementary bench reviews @ 55fc66b3f

| Finding | Resolution |
| --- | --- |
| Fable P2-A LPG11 fault cannot go RED | fault renamed: add the key to `forBuyerFields` (pre-decode allow-list) and `ForBuyerInput` |
| Fable P2-B pre-check vs receipt probe | LP7: pre-check only when the probe finds no receipt; LPG04 replay-after-erase + injection 2 |
| Fable P2-C entry cap over-built | ACCEPTED: `entry_count` and the 10,000 cap removed; export refused over 1 MiB (`export_too_large`, customers precedent) |
| Fable P2-D export replay vs stored response | §1: all receipts store ids/numbers only and every body is built from current rows, so export is no longer an exception |
| Fable P2-E entries "all" and "≤ 100" | first 100 + `truncated:true` |
| hy4 P0-1 PII in receipts after erasure | still applied (round 2 had no rule): receipts carry ids/numbers/states only (§1); erasure has nothing to scrub; LPG06 scans receipts. The tombstone/CD8 part already matches CD8's ids path and T20 gap (LP11) |
| hy4 P1-4 concurrent same-key race | covered by `command.Run`: advisory lock on scope/operation/key before the lookup, held to commit (`internal/command/command.go:45-76`); per-table keys no longer exist; LPG04 double-click case + injection 3 |
| hy4 P2-1 LPU03 negative balance | corrected to balance 0 (Δ −30) |
| GLM P2-7 erase copy vs order snapshots | LP11 + dialog copy state that 訂購人 names in past orders are unchanged; LPU05 asserts it |
| GLM P1-2 superseded ruling in W6-01B card | `docs/delivery/units/w6-01b-customer-tags-notes.md` ruling line struck through with a pointer here |
