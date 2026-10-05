# Adversarial re-review of contracts/live-console-v1.md (K3, round 2)

Reviewer: Kimi K3 (independent re-review after the Sonnet round, `output/live-console-review-k3/REVIEW.md` at 4bcb1adc).
Date 2026-10-05. Worktree `.worktrees/k3-lcn-review` (r3/integration, contract FROZEN; LC-B3 and A5 merged against it).
Read-only except this file. Evidence label: DESIGN review; code facts re-checked by grep at this worktree (cited inline). Nothing was run.

## Verdict: FREEZE_AFTER_FIXES

The Sonnet round was thorough and its 14 P1 fixes are correctly applied in the text. I attacked the amended clauses
themselves and found **4 new P1** the first round missed — three of them are *in* the fixes (the `link_pending_manual`
remediation cannot be executed, the `comment-facts` bridge route is FB-shaped only and breaks the OPEN-4 IG fallback
it was created for, and the adopted `duplicate_recent` rule is a read-then-insert race), plus one retention hole
(C4 never redacts UNKNOWN operations, so buyer ids in frozen requests live forever — contradicting §10's own claim).
Counts: **P0 = 0, P1 = 4, P2 = 10**. Per the freeze header, each finding reopens only the clause it names.

Re-verified OK (attacked again, held): HPKE public ring is mounted in `api`, private ring claims-worker only
(`deploy/secrets.manifest.tsv:99-101`) so the §3.4 seal/open split is deliverable; `takeover_generation:0` literal at
`migrations/0064_meta_claims_intake.sql:871` matches §0.2; `live_claim_window_one_open` at `0060:91`; ingest filters
`session_id` (`internal/claims/ingest.go:150`); retention C4's `LIKE 'mpr-%'` WITH CHECK (claims-retention-purge-v1 §4
line 184) is consistent because the rename target is `mpr-purged:…`; no price field accepted in the §5.1 body; the
`:m1` class is Meta-compliant (BLOCKED_POLICY/STALE_BINDING = provably zero HTTP, so Meta's one-reply budget is
unspent); §13.3 S3 correctly refuses to probe Meta's window enforcement by violating it.

---

## P1 findings

### P1-1  The `link_pending_manual` remediation is impossible — the fix creates an undeliverable-link state (§4.2, §14.1 clause 2)

The adopted fix for the manual-first race sets `claims.bundles.link_pending_manual = true` and shows 「此買家的認領連結
尚未送出，請在私訊中補發」. But after a **manual** private reply won the `mpr:` key: (a) the one private reply per
comment is spent (`used`); (b) a private reply does **not** open a 24 h DM window — "Further messages only after the
person responds" (F1, meta-claims-intake §0) — so A12 to that conversation returns `409 window_closed`; (c) public
replies may not carry links (§3.5). The merchant is told to do something Meta policy makes impossible until the buyer
happens to write first. The race window is also wider than the lock: the merchant can reply to a keyword comment in the
first seconds after it appears, **before the webhook stages the `claims.meta_intake` row** — there is no row to lock
FOR SHARE, so the yield rule cannot fire.

Replace the §4.2 second bullet's UI sentence and add a planner gate:

> …and sets `claims.bundles.link_pending_manual = true`; the console shows 「此買家的認領連結無法送出：這則留言的私訊
> 額度已用。買家回覆私訊後 24 小時內可用一般私訊補發認領連結」 and the bundle appears under A8 `unreplied` with
> `link_pending_manual: true`. No automatic send of any kind is planned for it. When the buyer next sends an inbound
> message the thread's window opens and the merchant pastes the link (A12; DMs may carry links, §12); the flag clears
> when a claim link is issued for the bundle. A13 exposes `link_pending_manual` so the buyer panel explains the state.
>
> Planner-side prevention: `inbox.plan_manual_private_reply` refuses with `409 auto_pending_confirm` when the frozen
> `comment_created_at` is younger than **120 s** and the session's claim window is OPEN (the intake row may not be
> staged yet, so the FOR SHARE yield cannot fire), unless the request carries `confirm_preempt_auto: true`; the UI
> warns 「系統可能正要自動傳送認領連結。手動私訊會用掉這則留言唯一一次私訊機會，確定繼續？」.

Gate LCN07 adds: manual reply inside the 120 s window without the flag → `auto_pending_confirm`; the full race (manual
wins, claim ACCEPTED) leaves a bundle whose A13 shows `link_pending_manual`, A12 → `window_closed` before any inbound
message, and a DM carrying the link succeeds after one; the flag then clears.

### P1-2  `comment-facts` is FB-shaped only; on the OPEN-4 IG fallback there is no facts source at all (§2.3, §3.3)

The bridge route specifies one Graph read: `GET /{comment_id}?fields=created_time,from{id},parent{id}` — FB comment
field names. IG comments expose `timestamp`, not `created_time`, and a different `from`/`parent` shape (F4). Worse:
when LC-U3 fails, OPEN-4 serves IG live comments from the **webhook copy** (`social.comment_events`); those comments
may not be Graph-readable at all, the ring buffer has no IG poller entries for them, and the route then answers
`found=false` → `409 comment_unknown` — manual private reply on IG live comments becomes impossible on exactly the path
OPEN-4 exists to serve, while §3.3 promises it (IG live: window OPEN + 15 min). The webhook copy already holds every
fact the rules need (`created_time`/`occurred_at`, `parent_id`, `from.id`, F4).

Replace the §2.3 `comment-facts` paragraph's lookup sentence with:

> Lookup order: the ring buffer; else, for an IG source in the OPEN-4 fallback (LC-U3 failed), the newest
> `social.comment_events` copy of that comment id through `social.read_comment_facts(p_session uuid, p_comment_ref
> text)` (STABLE SECURITY DEFINER, owner `commerce_meta_writer`, EXECUTE `commerce_claims_worker`; returns the same
> envelope + AAD columns as `social.read_comment_events`; `found=false` when no copy); else one Graph read within the
> shared rate budget, platform-branched: FB `GET /{comment_id}?fields=created_time,from{id},parent{id}`, IG
> `GET /{ig_comment_id}?fields=timestamp,parent_id,username` (exact IG field set recorded at probe R3 as LC-U12).
> `is_page` = the comment author's id equals the source's `asset_id`; `is_reply` = `parent_id`/`parent{id}` present
> (→ `409 reply_comment_unsupported` until LC-U10). `created_at` = `created_time`/`timestamp`/the copy's `occurred_at`.

LCN02/LCN07 add the IG-fallback facts cases (found/page/reply/unknown from the webhook copy).

### P1-3  The adopted `duplicate_recent` rule is a read-then-insert race — concurrent sends with different keys both go out (§3.3 rate, §4.1 recommend rate, §5.1 step 4)

The semantic keys deliberately include the Idempotency-Key (`mdm:`, `mpub:`, `mrec:`), so the unique index does **not**
serialize two different keys. Two tabs / two staff sending the same DM text to the same conversation concurrently both
pass the 30-s `body_hmac` check and both insert → **two identical DMs to the buyer**; the same race posts duplicate
public replies and breaks the ≤ 1 recommend per offer per 10 min rule. The first round proposed the check but no
serialization; a backend agent must now guess the mechanism.

Append to §3.3's rate bullet and §4.1's recommend bullet:

> The `duplicate_recent` checks are serialized inside the planning transaction: before the 30-s check the planner
> takes `pg_advisory_xact_lock(hashtextextended(store_id ‖ target ‖ body_hmac, 0))` (target = `conversation_id` for
> A12 and the §5.1 step-4 DM, `comment_ref` for A4/A5); the recommend 10-min check takes
> `pg_advisory_xact_lock(hashtextextended(store_id ‖ offer_id ‖ 'mrec', 0))`. The second planner blocks until the
> first commits, then sees the row and returns `409 duplicate_recent`. The 60 sends/min store cap is approximate and
> needs no lock.

LCN06/07/08 add: two concurrent same-body requests with different Idempotency-Keys → exactly one operation and one
`409 duplicate_recent`; two concurrent recommends → one `meta.offer_recommend`.

### P1-4  C4 never redacts UNKNOWN operations — buyer ids in frozen requests persist forever, contradicting §10 (§10 C4 extended, §14.7; claims-retention-purge-v1 §1 C4)

C4 eligibility is "terminal five" (existing) and the §10 extension repeats "terminal and created_at < now() −
intake_days". UNKNOWN is **not** terminal and never becomes terminal (external-operation-v1: "No automatic
UNKNOWN→READY"); an UNKNOWN `meta.dm_send`/`meta.public_reply`/`meta.private_reply` therefore keeps `comment_ref`,
`conversation_id` and `peer_key` in its frozen request **forever**, and the send secret is wiped but the ids are not.
§10's C5 row justifies the by-age delete with "the frozen request is redacted by C4" — false for exactly the UNKNOWN
case that motivated the by-age rule. These are the same identifiers C4 exists to redact (peer_key is an unkeyed hash of
an enumerable id, §3.7).

Replace the §10 C4-extended row's "Eligible" and "Action" with:

> `state ∈ (SUCCEEDED, FAILED_FINAL, CANCELLED, BLOCKED_POLICY, STALE_BINDING)` **or** `state = UNKNOWN AND
> updated_at < now() − intake_days`; and `created_at < now() − intake_days`. (UNKNOWN is never redispatched and
> Reconcile is query-only, so the ids have no further use — same argument as the §3.4 secret wipe.) Action: redact
> `comment_ref`, `peer_key`, `conversation_id` as RD-C4 **and rename the semantic key** of every covered action
> (`mpr:` incl. the `:m1` class → `mpr-purged:` ‖ id as today; `mdm:` → `dm-purged:` ‖ id; `mpub:` → `pub-purged:` ‖
> id; `mrec:` → `rec-purged:` ‖ id), because `mpub:` is a deterministic function of (object, asset_id, comment_ref,
> Idempotency-Key) and merchant-chosen keys are not entropy. The base C4 clause of claims-retention-purge-v1 gains the
> same UNKNOWN condition; its §4 RLS rows are updated by the implementing unit's migration (§14.7 mechanism).

LCN13 adds: an UNKNOWN operation older than `intake_days` has all three ids redacted and its key renamed; the §2.5
marks join tolerates renamed rows (old comments simply show no reply mark).

---

## P2 findings (each with the text change)

1. **A8 `display_name` has no source** (§3.2, §11 A8). `social.list_conversations` returns "conversation metadata only
   (no ciphertext)", yet A8 returns `display_name?`. State: `list_conversations` also returns the envelope + AAD columns
   of the newest inbound message per conversation (same shape as `social.read_thread`, one row per conversation) and the
   API derives the display name from it; or `display_name` is always null in v1. Pick one; the first matches the v5 UI.
2. **A8 bundle-only items are shapeless and dead-end** (§4.2, §11 A8). §4.2 puts `link_pending_manual` bundles "under A8
   `unreplied`" but A8's columns (`unread`, `mode`, `assignee`, `window_open_until`) are `conversation_state` fields
   that do not exist without a conversation, and no route acts on such an item (A9/A12 need a conversation; A4 needs the
   comment_ref, which A8 does not return). Add: bundle-only items carry `{bundle_id, conversation_id: null,
   link_pending_manual: true, unread: false, unreplied: true, last_at: bundle created_at, session_id}`; the UI opens the
   buyer panel (A13) and offers 「複製認領連結」 via the reused bundles-link endpoint — never a send button.
3. **Takeover-expiry generation is nondeterministic for Check** (§3.6). Expiry is "auto again with generation+1 —
   applied lazily … and treated as expired by `dm_window` readers even before it is written". An `origin=auto` op frozen
   at generation N sees either N (lazy write pending → send proceeds) or N+1 (write ran → `takeover_changed`) depending
   on timing. Fix in one line: "`dm_window` returns the **effective** generation (stored + 1 when `human_until` has
   passed and the lazy write has not run), so Check's comparison is deterministic"; decide deliberately whether a
   post-expiry auto send is allowed (recommend: it is — the thread is auto again — so expiry alone must not change the
   effective generation; only real takeover/release/writes bump it).
4. **Producer definer signatures are unspecified** (§4.1). MCI pins `plan_claim_reply` to the parameter list; the four
   new producers (`inbox.plan_dm`, `inbox.plan_manual_private_reply`, `inbox.plan_public_reply`,
   `live.plan_offer_recommend`) have names only. Give each its parameter list, return type, deny-code set and lock
   order in §4.1 (one line each), or a backend agent invents four different conventions.
5. **`inbox.bundle_peers` multiplicity and platform matching** (§3.7, §3.6 `dm_window_for_bundle`). State
   `UNIQUE (tenant_id, store_id, bundle_id, peer_key)` (insert is `ON CONFLICT DO NOTHING`; both an auto and a later
   `:m1` manual success insert the same key), and that `dm_window_for_bundle` matches the operation's platform
   (`app_id, object, asset_id` of the frozen request) when a bundle has peers on both platforms.
6. **The `social.messages` trigger must upsert** (§3.6). Two messages of a brand-new conversation committed
   concurrently both fire the AFTER INSERT trigger → unique violation on `conversation_state.conversation_id` rolls one
   consumer transaction back. Add: the trigger is `INSERT … ON CONFLICT (conversation_id) DO UPDATE SET last_inbound_*`
   under the classifier predicate.
7. **A15/A16 small money holes** (§5.1, §11). (a) Prefill shows per-line `live_price_minor` but not the **remaining**
   live quantity (claimed − consumed in `claims.live_price_uses`); a line claimed ×2 with 1 consumed quotes 1 live +
   1 catalog with no warning. Add per-item `live_quantity_remaining` and show it. (b) A16 with `send_payment_link: true`
   from a principal lacking `inbox:reply`: state `409 capability` (consistent with §6's planner rule), never a silent
   skip. (c) A15/A13 by `conversation_id` when LC-U6 is false (no `bundle_peers` row): state the response — `items: []`,
   `live_price_eligible: false, live_price_reason: "bundle_buyer_unverified"`, not 404 — so the UI offers the bundle
   path instead.
8. **LC-B6 misses its dependency on LC-B5** (§16). §5.1 step 4 plans template `order-pay-link/v1`, owned by LC-B5
   (`--msg-templates`), but LC-B6's Depends lists only LC-B3, LC-B4. Add LC-B5; otherwise LC-B6 starts against an
   unfrozen template id.
9. **Session re-`start` weakens the IG-live broadcast proxy** (§9, §3.3). IG private replies are gated on "source
   window OPEN" as a proxy for "broadcast running" (MCI §14 known limit). §9's reopen (`start` on an ended session) can
   open a window with no broadcast; local checks pass and Meta refuses → UNKNOWN until LC-U9. Acceptable (conservative,
   recorded), but say so in §9 and add the probe question to LC-U9 evidence.
10. **§0.2 code facts are stale after the LC-B3 merge** (§0.2). `internal/metaconnect/graph.go:205` now subscribes
    `feed,messages`; the "feed only (graph.go:198)" fact and deviation A18 describe base SHA `80b79487`. Harmless (the
    SHA is named), but annotate "fixed by LC-B3" so later readers do not re-report it.

---

## OPEN-n table (all were RESOLVED at freeze; my answer on each)

| OPEN | Answer | Condition / counter-text |
| --- | --- | --- |
| 1 polling | Agree | p95 ≤ 5 s / 5 tabs gate is already in LCN01. |
| 2 internal bridge | Agree | single replica + 421 + Bearer-only transport held; add nothing. |
| 3 FB poll object | Agree | LC-U1 decides; losing path deleted after R2. |
| 4 IG live comments | Agree **with P1-2** | the fallback is incomplete without a facts source; adopt the `social.read_comment_facts` branch or manual private reply is dead on it. |
| 5 streaming endpoint | Agree, rejected | token in URL; reopen only via LC-U5. |
| 6 ring buffer | Agree | deletion eviction via batched id re-read is a clean implementation of the first round's P2-6. |
| 7 permissions | Agree | role-bundle definition + backfill + explicit `viewer` exclusion, tested in LCN03. |
| 8 takeover trigger | Agree | implicit-on-DM-only + 6 h expiry is right; resolve the generation nondeterminism (P2-3). |
| 9 read state | Agree | `inbox:reply`-only clearing protects the assignee's unread. |
| 10 echoes | Agree | banner + 24 h hint is honest. |
| 11 quota priority | **Counter in part (P1-1)** | first-writer-wins and the yield locks are right; the `link_pending_manual` *remediation* as written is impossible — adopt the corrected hint, the A12-until-inbound rule and the 120 s confirm gate. |
| 12 public content | Agree | NFKC + Cf strip + the LCN08 evasion corpus covers the realistic forms. |
| 13 live price | Agree with the ruling | fail-closed, single-use, quote-bound, `live:manage`, one live order per bundle, 0105 as the only ceiling; adopt P2-7 (remaining quantity, capability code, LC-U6-false response). |
| 14 PSID custody | Agree | wipe at UNKNOWN is correct; extend the same argument to the frozen-request ids (P1-4). |
| 15 cap 5 | Agree | plus fleet/tenant poller caps; §8 grep-gate names its callers. |
| 16 comments after end | Agree | shown and counted, never claimed. |
| 17 `inventory:live_adjust` | Agree | the bounds (forced reason, ±1000, never below reserved+allocated, audited) keep I03. |
| 18 recommend | Agree | one comment per offer per 10 min — but serialize the check (P1-3) or two clicks post two comments. |
| 19 outbound retention | Agree 30 d by age | pair it with P1-4 so the frozen request of the same UNKNOWN operation is also redacted; otherwise the by-age rule's stated justification is false. |
| 20 read audit | Agree | coalesced `inbox.thread_opened`, no content. |

## Attack checklist result (for the record)

| Area | Result |
| --- | --- |
| Meta 24 h, one reply per comment, no tags | Rules held; the one newly found policy-adjacent defect is P1-1 (a state where policy forbids the remediation the UI instructs). `:m1` verified compliant (zero HTTP before it). |
| Public reply content | Held; NFKC corpus in LCN08. |
| Privacy / storage of text, names, PSIDs | Design held (memory-only comments, sealed copies, no clear PSID); gaps are P1-2 (facts path), P1-4 (UNKNOWN redaction), P2-1/2/5. |
| Money (OPEN-13) | Ruling sound: fail-closed, quote-bound single-use grant, 0105 cap, one live order per bundle; no double-charge or double-reserve path found beyond what LCN12 already pins. P2-7 leftovers only. |
| Concurrency / idempotency | `mpr:` first-writer-wins + intake row locks held under `-race` reasoning; new race found in the *adopted* `duplicate_recent`/recommend rules (P1-3); takeover-expiry generation nondeterminism (P2-3). |
| Tenant isolation | Bridge re-check, definer-side store checks, cursor HMAC scoping, LCN03 coverage all held. No new cross-tenant path found. |
| Underspecified for a backend agent | P1-2, P1-3, P2-1, P2-4, P2-6 would each force a guess; P1-1 would ship a dead-end UI instruction. |
