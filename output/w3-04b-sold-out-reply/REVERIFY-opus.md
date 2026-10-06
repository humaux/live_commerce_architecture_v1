# W3-04B sold-out auto reply: independent re-verification of the fix round (Opus, read-only)

## Scope and evidence

- **Reviewed:** branch `unit/w3-04b-sold-out-reply` HEAD `ff3a496b7da2cb9a0ec984f552f76794dd392041`, fix diff `git diff d29a8a34 ff3a496b`.
- **File hashes (sha256):**
  - `migrations/0151_sold_out_reply.sql` `d6b7c155…6b0921`
  - `internal/integrations/metareply/sold_out.go` `60ddbc41…1a60f6`
  - `tests/foundation/sold_out_reply_test.go` `e94ca13c…eb077`
- **Re-run by the reviewer (DB-free only):**
  - `go build ./...` exit 0
  - `go vet ./internal/integrations/metareply/ ./tests/foundation/` exit 0
  - `go test -count=1 -run SoldOut ./internal/integrations/metareply/` ok
- **Not re-run by the reviewer:** the PG suites (NOT_RUN as instructed). For those I relied on the author's `green-p1.log` (31 PASS, exit 0) and `green-p1b.log` (4 PASS, exit 0), plus reasoning over the code.
- **Trunk drift:** none. `git log d29a8a34..r3/integration -- migrations` is empty.

## Verdict: **MERGE**

Both P1s are fixed. There is no new P0 or P1. The P2 items below can be handled at integration.

---

## P1-1: one text rule in SQL and Go. FIXED

| Claim | Evidence |
|---|---|
| Go rejects Cc only | `sold_out.go:61-71`: `unicode.IsControl` plus 1..400 runes plus valid UTF-8. Go's `IsControl` is exactly Latin-1 Cc: U+0000-001F and U+007F-009F. |
| SQL applies the identical set | `0151:49`: template `!~ '[\x01-\x1f\x7f-\x9f]'`. `0151:146`: name `regexp_replace(... '[\x01-\x1f\x7f-\x9f]',' ','g')`. Postgres text cannot hold NUL, so the sets are equal. The green tests prove the regex works: `\n` in a name becomes a space, and the U+0085 template `so-c1` is refused. |
| U+3000, ZWJ and NBSP are accepted | Neither fence rejects them. Go unit `TestSoldOutTextRuleIsControlCharactersOnly` (sold_out_test.go:105) covers all three. PG `TestSoldOutReplyTextMatchesAdapter/full-width space, ZWJ and newline` sends end to end. |
| Only one adapter path | `replyContent` (routes.go:409-415) and the Check path both go through `parseSoldOut`, which calls `soldOutTextOK` (sold_out.go:52). The `printable`/`IsPrint` helper (routes.go:451) is used only for message and comment ids. |
| `{{product.name}}` at most once | `0151:50`: `(char_length(body)-char_length(replace(body,'{{product.name}}','')))<=16`. Foreign placeholders are still refused at `0151:51`. |
| Name is clipped by characters | `left(…,60)` on the sanitised name (`0151:146`). PG `left` counts code points, and a UTF8 database cannot hold a surrogate, so a byte or surrogate split is impossible. A ZWJ grapheme can be cut visually, which is harmless. Go counts code points too (`RuneCountInString`), so the counts match. |
| Empty text cannot occur | Product name is 1..120 characters (`0002:28`), and offers have a FK to the SKU (`0060:67`). So the name is never empty and the rendered text is at least 1 character. |
| Fallback | `0151:227-230`: an unusable chosen template falls back to fixed `sold-out-reply/v1` with the name. `0151:233-235`: a NULL or over-400-character render falls back to fixed text without the name. `0151:245-248`: a request over 2048 bytes falls back to fixed text. |
| No raise from template or name content | The sold-out branch (`0151:223-248`) has no RAISE. `sold_out_body` is a SQL STABLE select. `claim_sold_out_facts` raises only through `claim_sku_sold_out` on quantity outside 1..999, which `0060:144` makes impossible, or on missing GUCs, which the intake transaction always sets. The global `0151:260` (>2048 → 22023) cannot be reached by the sold-out branch after the fallback. |

### 2048-byte check (jsonb text, max-length ids)

Assumed maxima: `comment_ref` 80, `asset_id`/`app_id` 40, UUIDs 36, bigint generation 19 digits.

| Case | Text | Request bytes |
|---|---|---|
| Fixed template + 60 × 4-byte name | 85 chars / 313 B | **1078** (OK) |
| Merchant template at the bound (280 chars = placeholder + 264 × 4-byte) + 60 × 4-byte name | 324 chars | **2061** (> 2048) |
| Fallback text | | 846 |

The second case is the reachable one. It triggers the `0151:245` fallback, so behaviour is correct: no raise, and the fixed text is sent. The code comment "unreachable with the 280/60/400 bounds" is wrong (see P2-1).

---

## P1-2: no raise on a disagreement between the two fact reads. FIXED

- **`0151:222-223`:** `v_so := sf.sold_out IS TRUE AND sf.reply_enabled IS TRUE`. Only that combination takes the sold-out branch. Every other combination takes the link branch (`0151:249-258`), which is the unchanged 0128 body. That includes a NULL `sf`, which happens when the event has zero rows or no offer.
- **The old `IF NOT sf.reply_enabled THEN RAISE 22023` is gone.** The remaining RAISEs in `plan_claim_reply` are the pre-existing 0128 invariant checks (intake lease, event, source/binding, storefront, River job). None of them depends on stock or the switch.
- **Disagreement matrix.** "plannable" is `claim_reply_plannable`, "plan" is `plan_claim_reply`.

  | What plannable saw | What plan sees | Result |
  |---|---|---|
  | Sold out, OFF | (plan never runs) | Audited skip; plan is never called (poller.go:217-219) |
  | Not sold out, or ON | Sold out, ON | Sold-out branch |
  | Not sold out, or ON | Anything else | Link branch |

  No combination raises, so `classify` (poller.go:250+) can no longer turn the race into a final failure.
- **Accepted consequence:** a sold-out claim with the switch OFF can get a link when it loses the race. Begin stays the stock authority. This is the pre-0151 behaviour and is documented in `contracts/live-keyword-claims-v1.md` (Race rules).

## reply_kind and the KC03 matrix: correct, with one P2

- **Column:** `0151:111` adds the column with `CHECK (reply_kind IS NULL OR (reply_kind='sold_out' AND outcome='ACCEPTED'))`.
- **Set once:**
  - It is set only inside the sold-out branch (`0151:274`, `WHERE … reply_kind IS NULL`).
  - `commerce_integration_writer` gets column UPDATE (`0151:115`) under a policy with `USING reply_kind IS NULL` and `WITH CHECK reply_kind='sold_out'` (`0151:116-118`). Once set, it cannot be changed or cleared.
  - No other role has UPDATE on `claims.events`: grep shows only SELECT and INSERT grants (0060:283, 0064:355/387/457, 0113:42, 0123:444). There are no triggers on the table.
- **KC03:** the iw SELECT(+reply_kind) and UPDATE(reply_kind) rows are added explicitly (`live_claims_schema_test.go:283-284`). That part is honest.
- **The gap (P2-2):**
  - `commerce_runtime` (0060:283) and `commerce_claims_intake` (0064:355) hold table-level INSERT, which automatically covers the new column. They can INSERT an event with `reply_kind='sold_out'` already set.
  - KC03 accepts this silently, because its `cols("claims.events")` helper expands dynamically.
  - Impact: the field is informational only. It can be forged at insert time, but not changed once set.

## Tests: do they encode the P1s?

**P1-1, Go unit.** Valid red: `red-p1.log` shows U+3000, ZWJ and NBSP refused by the old adapter. Reverting `IsControl` back to `!IsPrint` makes it fail again.

**P1-1, PG (`TestSoldOutReplyTextMatchesAdapter`).**
- Subtest 1 has a valid red for the right reason: the old code froze a `\n` into the text. If only the Go fence is reverted, `sendOK` also fails.
- Subtest 3 has a valid red: the old code accepted `so-six`. Its fallback half asserts `template='sold-out-reply/v1'`, which the old code (rendering `so-six`) would fail.
- Subtest 2 (120-character name) passed on the old code too. It is a guard, not a red.
- Nothing exercises the >400-character or >2048-byte fallbacks. They are unreachable or near-unreachable through validated templates (see P2-1).

**P1-2 (`TestSoldOutReplyDisagreementKeepsTheClaim`).**
- **The red is not valid evidence.** Both subtests failed with `FAILED/sqlstate_42501`, including subtest 2, which raises nothing on the old code either. So the 42501 comes from the test rig (most likely the `so_flip` trigger or its environment in that run), not from the P1-2 path. That red proves nothing about the fix.
- **By reasoning, the green subtest 1 does encode P1-2.** With stock 5 and switch OFF, `plannable` returns OK. The trigger fires inside the same transaction on the River insert (poller.go:224, between the plannable call at :212 and the plan call at :233) and sets stock to 0. If the fix is reverted, `plan` sees sold out with the switch OFF, hits the old `RAISE 22023`, and `classify` returns `invalid, final`. The intake becomes FAILED and the assertion at :617 fails.
- Subtest 2 is a guard, not a P1-2 red.

**reply_kind (`TestSoldOutReplyEventMark`).**
- Its red is "column missing", which is the feature being absent; that is acceptable.
- It checks the CHECK vocabulary as owner. It does not exercise the RLS one-way rule as `commerce_integration_writer` (P2-3).

## No regression

- **`plan_claim_reply` vs 0128 (`diff`):** the only changes are the sold-out additions and a one-space re-indent of the link branch into `ELSE`.
- **`check_meta_reply` vs 0128:** only the message-type guard around the link proof.
- **`claim_reply_plannable`:** the patch hunk is unchanged from the reviewed version (needle-guarded in-place patch over 0143).
- **Later migrations:** no migration after 0128 re-declares these functions except 0143 (the plannable patch base) and 0151. The same holds on trunk.
- **Template id CHECK:** `0151:31` = 0144's set `{order-pay-link, offer-recommend, checkout-reminder}` plus `sold-out-reply`. No id is lost.
- **Owner and ACL:** owners are re-asserted (`0151:172, 278, 328`). `claim_sold_out_facts` now has its COMMENT (the old P2-7 is closed).

---

## Remaining items (P2, none blocking)

1. **`0151:245` comment.** It says "unreachable", but the maximum-length merchant template in 4-byte characters plus maximum-length ids gives 2061 bytes. Behaviour is correct (fallback). Fix the comment, or tighten the merchant bound so the case really is unreachable.
2. **`reply_kind` can be forged at INSERT.** `commerce_runtime` and `commerce_claims_intake` can set it on INSERT through their table-level grants, and KC03 accepts that silently via `cols()`. Options:
   - add `reply_kind IS NULL` to the `event_insert` / `event_intake_insert` WITH CHECK (needs a migration ≥ 0151, or put it in 0151 now); or
   - at least list it in KC03 and the contract.
3. **The P1-2 red is invalid (42501 rig failure).** Per the gate rule (inject-fault → red → fix → green), re-capture it: revert only `0151:222-223` to the old RAISE and show `FAILED/invalid` on subtest 1. Then add one case running `UPDATE claims.events SET reply_kind=NULL` as `commerce_integration_writer`, and expect 0 rows or a denial.
4. **Carried over from REVIEW-opus.md, still open:**
   - URL ban or contract note for merchant sold-out templates (P2-1)
   - W3-05B number must be > 0151 (P2-2)
   - scope assert in `claim_sold_out_facts` (P2-6)
   - SO03 ingest ruling (P2-5)
   - DELIVERY items (P2-10): reasoning level and write_paths are now present.
