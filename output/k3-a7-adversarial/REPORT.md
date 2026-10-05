# k3-a7-adversarial — independent adversarial review of kwc-v1 (KEYWORD_QTY_CONTAINS)

Target: `internal/claims/grammar/contains.go` (ParseContains, ParseForIngest) and the mode
gate `effective`/`offerReason` in `internal/claims/ingest.go`, against contract
`contracts/live-keyword-claims-v1.md` §2.5 and brief
`docs/delivery/units/live-a7-contains-match.md`. Written without reading the implementation
diff first (expectations derived from the frozen §2.1/§2.2 rules and the §2.5 tables; the
implementation was probed only to confirm outcomes).

## Cases added

- `tests/claims/kwc-v1-adversarial.json` — 160 cases in 10 categories (hit 30, negation 23,
  question 21, multi-fragment 26, glued 14, numbers 12, no-keyword 10, head-length 8,
  url 8, safe-outcome 14), each with grammar-level expected kind/keyword/quantity/explicit
  and a `safety` flag (`frozen` = spec-derived; `safe_outcome` = brief silent, safe
  direction asserted).
- `tests/claims/kwc_adversarial_test.go` — strict-decoding corpus runner over the exported
  API + ParseForIngest entry invariants (kw-v1 wins; corpus NO_MATCH never becomes an order
  candidate at the single ingest entry).
- `internal/claims/grammar/contains_adversarial_test.go` — raw-byte boundary tests
  (251/254/256 bytes MATCH; 257/260 NO_MATCH; bound is pre-width-map, pre-trim), invalid
  UTF-8, control/bidi/joiner/soft-hyphen boundary bytes, spec-vector cross-check through
  ParseForIngest, and `FuzzParseContainsAdversarial`.

## Result summary

- **146/146 frozen cases PASS** — no violation of the frozen §2.5 rules found. Glued heads
  (`2A01+1`, `A01A02+1`) are never carved into a real keyword; prices/phones/URLs/order
  numbers/multi-keyword comments are NO_MATCH; quantity bounds (1..999, no leading zero,
  length checked before conversion) hold; `A1+2 A1+3`, `+1` alone, `A01-2`, `A01*2` are
  NO_MATCH; `A01x2`→head `A01X2` and `#A01`/`1000元`/`0912345678`→implicit MATCH are
  grammar-level only and land as QUANTITY_REQUIRED (no order) at ingest by §2.3.
- **Fuzz**: `go test -run='^$' -fuzz='^FuzzParseContainsAdversarial$' -fuzztime=60s
  ./internal/claims/grammar` → PASS, 2,614,284 execs, 136 corpus entries, 0 failures
  (output/k3-a7-adversarial/fuzz.log). No panic, deterministic, keyword ⊆ normalized input,
  single-char table fragments unevadable, kw-v1 precedence intact.
- **Mode gate** (`effective`, ingest.go:331; `offerReason`, ingest.go:340): verified by
  inspection only — both are unexported and DB-free tests cannot reach them from my allowed
  write paths. Logic matches the brief: kwc-v1 survives only in CONTAINS; implicit MATCH →
  QUANTITY_REQUIRED in CONTAINS/QTY_ONLY. Executable coverage remains with the author's
  KCC02 REAL_PG tests (NOT_RUN here).

## Failures found (14; all reproduce; each is input → expected safe outcome → actual)

Red evidence: `output/k3-a7-adversarial/red.log` (`go test ./internal/claims/... ./tests/claims/...` exit 1).

### F1 — question-table gaps turn common questions into orders (5 cases; highest risk)
The frozen question table has no `問`/`請問`/`如何`/`價格` fragment, so:
- `請問A01+2` → safe NO_MATCH → **MATCH A01 ×2**
- `請問一下A01+2` → **MATCH A01 ×2**
- `A01+2如何` → **MATCH A01 ×2**
- `請問A01+2還有貨` → **MATCH A01 ×2**
- `A01+2價格` → **MATCH A01 ×2**

In a CONTAINS window with offer A01 these create real orders from price/availability
questions — the exact class the owner ordered to manual review (「問句…不命中，轉人工」).
`請問` is the most common Taiwan question opener; expect this to fire many times in a
5-hour live. Note the conservative table already accepts false negatives (`A01+1不錯`); the
miss direction here is the unsafe one.

### F2 — two-character table entries are evadable by interposed characters (7 cases)
The negation/question check is a plain substring search over the canonical text, so any
2-char entry breaks when a space/emoji/punctuation is inserted:
- `取 消A01+2`, `取😍消A01+2` (取消), `算 了A01+2` (算了) → **MATCH A01 ×2** — cancel intent
  becomes an order
- `多 少A01+2` (多少), `可 否A01+2` (可否), `什 麼A01+2` (什麼), `是 否A01+2` (是否) →
  **MATCH A01 ×2**

Single-char entries (`不 沒 没 別 别 勿 莫 退 ? 嗎 …`) are proven unevadable (corpus +
fuzz property). Buyers do insert spaces/emoji into words casually in live chat, so this is
a realistic, if lower-frequency, hole; the miss direction is unsafe (cancel → order).

### F3 — lookalike-letter carve produces an order for a DIFFERENT keyword (2 cases)
A non-ASCII lookalike letter is a boundary rune, so the digits after it become their own
keyword-shaped fragment:
- `А01+2` (Cyrillic А, visually identical to A01) → **MATCH keyword "01" ×2**
- `Α1+2` (Greek Α) → **MATCH keyword "1" ×2**

If the session has a numeric offer keyword (`01`, `1` — plausible), the buyer's claim lands
on the wrong product instead of NO_MATCH. kwc-v1 correctly never carves ASCII-glued heads
(`2A01+1`→`2A01`), but boundary-carving after a lookalike letter has the same shape and
the unsafe direction.

## Verdict — is kwc-v1 safe for a 5-hour live?

**Core grammar: yes.** Everything the contract froze behaves exactly as specified; the
single-fragment rule plus §2.2 head rules block the high-volume garbage (prices, phones,
URLs, multi-item, glued keywords, `+1` alone); bounds and idempotence invariants hold under
fuzz; the mode gate confines kwc-v1 to CONTAINS windows and implicit matches can never
create orders.

**CONTAINS mode as frozen: no, not without a table fix.** F1 alone (請問/如何 questions
becoming orders) is a per-live-certainty in Taiwan chat, and F2/F3 add lower-frequency
unsafe misses. The tables are frozen data — any fix is **kwc-v2**, never an in-place edit
(§2.5). Recommended before enabling KEYWORD_QTY_CONTAINS for a real 5-hour live:
1. kwc-v2 question table += `問` (covers 請問), `如何`, `價錢`/`價格`; consider negation
   segmentation that tolerates one interposed boundary rune for 2-char entries (F2);
   consider rejecting fragments produced by lookalike-letter carving (F3) or simply
   disallowing all-digit heads below some length in CONTAINS sessions.
2. Until then, keep the EXACT default and treat CONTAINS as opt-in with the host prompt
   warning; monitor the 「看不懂」 count — but note F1/F2/F3 are false *positives*, which
   that counter cannot see.

EXACT and KEYWORD_QTY_ONLY windows are unaffected by all 14 findings (kwc-v1 results are
downgraded there by `effective`).
