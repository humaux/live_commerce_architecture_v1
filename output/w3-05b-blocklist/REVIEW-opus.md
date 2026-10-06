# W3-05B buyer blocklist — independent review (Opus)

- Reviewer: Claude Opus 5.5 (read-only, independent of the author). Date 2026-10-06.
- Target: worktree `.worktrees/w3-05b-blocklist`, HEAD `ea80f56e` (author `9f04153b` + trunk merge `161d5d34`; the merge touches only `tests/admin/auth-real.spec.ts` and `buyer_comms_gate_lookup_test.go`).
- Diff: `git diff r3/integration...HEAD` (18 files).
- Local checks I ran (DB-free, at ea80f56e):
  - `go build ./...` passed.
  - `go vet` on claims, httpapi, pagination and tests/foundation passed.
  - `go test ./internal/httpapi ./internal/pagination ./internal/claims ./internal/claimsintake` passed.
  - `bash scripts/dev/check-headers.sh r3/integration` returned OK.
  - `bash scripts/dev/check-gates.sh` exited 0.
  - `python3 scripts/check_packet.py` exited 0.
- NOT_RUN: every REAL_PG test (CI only). For PG behaviour, the author's `green.log` and `red.log` are E0 for this review.

## Verdict: **MERGE-AFTER-FIX** (2 × P1, no P0)

The core design is sound. The restricted decision sits in `claim_reply_plannable`, which never raises on this path. `plan_claim_reply` is untouched. The anchors are unique and their guards are real. The table is closed to login roles, and the client can never send an actor_key. Two paths still break the unit's own promises:
- a claim link and DM can reach a restricted buyer through checkout reminders;
- erasure fails in the steady state where the blocklist row is the only record left of the actor.

---

## P1

### P1-1 Checkout reminders issue a fresh claim link and DM to a restricted actor
- **Evidence:**
  - `migrations/0144_checkout_reminders.sql:216-284`. `inbox.checkout_reminder_candidates` loops over every social bundle of the session. It never consults `claims.blocked_actors`. `grep blocked_actors` finds no consumer outside 0154.
  - When the verdict is `send`, `internal/merchanttools/checkout_reminders.go:142-144` calls `claims.IssueLink` (`claims.issue_link`, 0060:595). This **creates a new `claims.links` row** and plans a `meta.dm_send` with that link (template `checkout-reminder/v1`).
- **Reachability:** the bundle needs a peer. Peers are written by `inbox.finish_send` after any successful private reply (`0128:723-737`). Two ordinary cases provide one:
  - (a) the merchant uses the one manual reply that BL03 deliberately keeps (`blocklist_test.go:155`);
  - (b) the actor's bundle was created before the block.
  If the buyer then wrote within 24 h, the merchant's bulk 「提醒未付款」 (`p_bundle IS NULL`) re-issues a claim link and DMs it to the restricted buyer.
- **Why it matters:**
  - The integrator ruling says 「名单内买家…不签发链接」.
  - The Amendment says "the system issues no claim link".
  - The Limits list covers only links issued *before* the block, not new links issued after it.
  - A bulk action is not a deliberate per-buyer choice, unlike LC-B6, which the brief explicitly downgrades to a warning.
- **Fix:**
  - In `checkout_reminder_candidates`, give a bundle whose `(tenant, store, actor_key)` is listed a followup verdict `restricted`. Extend the `reason` CHECK at 0144:53 and record it like the other followups, or simply `CONTINUE`. This covers both the batch and the single-bundle path. The single-bundle path should return `409 not_remindable`.
  - The function is owned by `commerce_integration_writer`, and its `blocked_reply_read` policy only admits `claims.intake_scope()`. You therefore need either a merchant-scope policy (`inbox.lcn_in_scope(tenant_id, store_id)`, as 0151 uses for `sold_out_settings`) or a boolean helper owned by `commerce_claims_writer`.
  - Add a REAL_PG test: block → manual reply (peer recorded) → inbound DM → batch reminder. Expect 0 new links and 0 `dm_send`.
  - Alternatively, if the integrator accepts this behaviour, write it into the Amendment's Limits. As it stands, the contract contradicts the code.

### P1-2 Erasure fails (PT404 rollback) when the blocklist row is the only remaining record of the actor
- **Evidence:**
  - The 0154 patch (`0154:156-169`) adds `DELETE FROM claims.blocked_actors … WHERE actor_key=p_actor_key`, but it does not add the deleted count to the returned counts. Those counts are built at `0127:321-323`: bundles, lines, links, intake, comment_events, operations, messages, conversations.
  - `claims.erase_actor` sums the counts and raises `PT404 'erasure target not found'` when the sum is 0 (`0071:571-574`). That rolls back the whole transaction, including the blocklist DELETE.
- **Why it matters:**
  - By design, blocklist rows outlive age retention (`0154:22`, Amendment "Erasure and retention").
  - Age retention de-identifies bundles after `claims_days` (default 90, `0071:254-273`) and deletes intake after `intake_days`.
  - So the normal long-term state of a blocked actor is "only the blocklist row still carries the actor_key, plus a free-text note that may contain PII". An erasure request for that actor (selector (a) actor key; selectors (b) and (c) cannot even resolve once intake and bundles are gone) then fails and the row survives. Requirement 3e ("erasure removes entries") does not hold in its most likely case.
  - `TestBlocklistErasureRemovesEntry` (`blocklist_test.go:427-448`) calls the helper `apply_actor_erasure` directly, while live bundles and intake rows still exist. It cannot see this.
- **Fix:**
  - Count the deleted rows and return them. One option is to also patch the `RETURN jsonb_build_object('bundles',…` anchor and add a `'blocked_actors'` key. Check CRP02 pins on the counts shape first.
  - Alternatively, fold the count into an existing counter and say so.
  - Add a REAL_PG test through `claims.erase_actor(p_actor_key=…)`: purge or de-identify the actor's intake and bundles first, leaving only the blocklist row. Expect success and 0 rows.

---

## P2

1. **Restricted mark missing when the source has `private_reply=false`.**
   - **Evidence:** `internal/claimsintake/poller.go:186-197` calls `planReply` (and therefore `claim_reply_plannable`) only when the claim is `ACCEPTED && BundleCreated && private_reply`.
   - **Effect:** with auto-reply off, a restricted actor's claim gets no `reply_kind='restricted'` and the console does not show 「已限制」, against the brief ("claim 标「已限制」"). No link leaks.
   - **Fix:** document it in the Limits, or mark it in the ingest path.
2. **Merchant manual link paths have no guard and no documented warning.** These are `claims.issue_link` (0060, the "issue/rotate link" route) and the LC-B4 "send the link by hand" DM. Only LC-B6 is named as warn-only.
   - **Fix:** add one Amendment line saying manual link issue and manual DM to a restricted actor are allowed but the UI warns via `/blocklist/check`, so W3-U2 wires it.
3. **The race description is inaccurate.** See `0154:18-20`, the Amendment "Race" bullet and DELIVERY item 5.
   - A claim that meets an in-flight block *waits* on the advisory key. It only ends with 55P03 when the block holds the key longer than the poller's `lock_timeout` of 1 s (`poller.go:44,134`).
   - Normally it proceeds and, because each statement of a VOLATILE plpgsql function takes a fresh snapshot under READ COMMITTED, sees the committed block. The race test holds the key indefinitely, so it only exercises the timeout branch.
   - 55P03 is non-final in `classify` (`poller.go:244-256`), so the claim retries. It can only be lost after 10 consecutive timeouts (fail_meta_intake cap), which is not realistic for a millisecond-long block transaction. **Retry semantics are correct.**
   - **Fix:** reword the description.
4. **Lock mode.** `claim_actor_restricted` takes the per-actor key *exclusively* (`0154:104`), so two claims of the same actor in the same store serialise on it, for example in two concurrent sessions. The reader only needs `pg_advisory_xact_lock_shared`; `block_actor` keeps the exclusive lock.
   - The order (claim: intake → lcn-mpr → binding → source → actor key; block: store key → actor key, advisory keys only, no row locks) has no cycle. Shared mode is the only improvement.
5. **No ACL pin for the new `commerce_integration_writer` grants.**
   - The new grants are `SELECT(tenant_id, store_id, actor_key)` on `claims.blocked_actors`, `SELECT(actor_key)` on `claims.meta_intake` and the policy `claim_reply_restricted_audit` (`0154:55-72`).
   - KC03, T06 and CRP02 pin the claims_writer, function and retention sides. MCI02 is a 0064-only delta, and WAS06 only inventories.
   - **Fix:** add an exact-set assertion so that widening any of these fails a gate.
6. **Weak red.** `red.log` is "migration absent" (42883/42P01).
   - **Fix:** add a behavioural red, such as 0154 applied without the plannable patch so BL01 fails with a link and operation issued, to show the test catches the real regression.
   - Also add an interaction test: restricted + sold-out (switch on and switch off) → `restricted`, 0 operations. The analysis below says restricted wins, but nothing pins it.
7. **Session not used by some routes.** List, check and DELETE are mounted under `/live-sessions/{session_id}/claims/…` but ignore the session (store-level data); a non-existent session id still answers. This is harmless but surprising. Either validate the session or say so in the contract.
8. **Pre-existing LiveConsoleInbox failures.** I confirmed they are not caused by this unit: `nobl-order.log:289-297` and `green.log:545-553`.
   - **LCN10:** `live_console_inbox_test.go:435` reads `list_conversations('all',NULL,NULL,50)` and finds no row.
   - **CrossStore:** line 534 shows exactly 50 ids without convA1.
   - Both are a LIMIT-50 page in a shared fixture store that earlier tests (Ads/CheckoutReminder/…) fill with newer conversations. 0154 does not touch `social.conversations` or `list_conversations`, and the failure reproduces without the blocklist tests.
   - This test-isolation bug should be tracked separately.

---

## Questions answered (with evidence)

1. **Restricted behaviour.**
   - `0154:120-133`: restricted is the first branch of the skip chain. It sets `reply_kind='restricted'` (no `link_pending_manual`) and audits `claim_reply_skipped:restricted`.
   - The poller returns early on a code other than OK (`poller.go:216-218`): no River job, no operation, no `claims.links`, so the `mpr:` budget is unspent.
   - `console_marks` reason is `restricted` (`0154:143-149`).
   - Paths that reach a restricted actor anyway:
     - sold-out reply: no (it is only reachable through plan_claim_reply after OK);
     - **reminders: yes (P1-1)**;
     - manual issue_link / LC-B4 manual send: merchant-deliberate (P2-2);
     - direct checkout: only redeems an existing link, so it is covered by the "links issued before the block" Limit.
2. **Restricted vs sold-out.** Restricted wins.
   - The 0151 sold-out branches are `sold_out_off` (later in the plannable chain, `0151:164`) and the sold-out reply in `plan_claim_reply` (only after OK).
   - No disagreement or RAISE is possible: plannable holds the actor key until commit, so a block cannot commit between plannable and plan_claim_reply, and plan_claim_reply has no restricted logic.
3. **Privacy.**
   - The actor_key never leaves the database: definers return id, platform, note and bundle only; unblock is by id.
   - The runtime role has no table grant, and this is tested (`blocklist_test.go:285-291`).
   - The client cannot send an actor_key: the strict decoder returns 400 and the definer returns 22023.
   - Scoping is per store: PK and RLS on tenant and store, and every resolver filters `v[1]/v[2]`.
   - The note never reaches the audit, the receipt (only the request hash is stored, `command.go:37-44`) or Meta.
   - Erasure: **P1-2**.
   - The 5000 cap holds: it is counted under the store advisory key, and the only insert path is the definer.
   - Global erasure by actor_key matches the other 0127 deletions, since the key embeds object and asset.
4. **Concurrency.** Correct; see P2-3 and P2-4. `fail_meta_intake` retries 55P03, and nothing is half-applied because the whole apply transaction rolls back.
5. **SECURITY DEFINER hygiene.**
   - All five new functions use `search_path=pg_catalog`, `REVOKE PUBLIC` and explicit owners.
   - `claim_actor_restricted` has no EXECUTE grant (T06 pin).
   - The definers re-check `live:manage` / `live:read`, and the routes check them through `scopedAs`.
   - Tenant and store come from `platform.WithScope` and `claims.for_buyer_scope()`.
   - FORCE RLS is on the table.
   - The anchors are unique and the assertion is exact. `(len - len(replace))=len(needle)` means exactly one occurrence. v_n1 and v_n2 occur once after 0143/0151, and v_n2 stays unique after the patch. The console and erasure anchors are unique in 0123 and 0127.
6. **Pins.** R2 76→77, KC03 11→12 tables and 38→42 owned objects, T06 88→89 and CRP02 matrix and hold-back are additions only and match the objects created. I saw no weakened assertion.
7. **Headers and docs.** Headers are present and check-headers is OK. The Limits are in the Amendment, but they miss the reminder path (P1-1), manual link and DM (P2-2) and the private_reply-off mark (P2-1), and the race text is inaccurate (P2-3).
