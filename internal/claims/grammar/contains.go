// Purpose: kwc-v1 restricted contains grammar (ParseContains) and the mode-independent ingest parse entry (ParseForIngest: kw-v1 first, kwc-v1 fallback).
// Depends on: pure Go (no I/O); grammar.go width map and §2.2 head rules; frozen negation/question word lists in this file.
// Used by: internal/claims (ingest.go, manual.go), internal/integrations/meta/claim_intake.go (qualifyClaim); tests/claims/kwc-v1-vectors.json.
// contains.go owns kwc-v1 (VersionContains), the one restricted contains grammar (§2.5):
// ParseContains finds a single maximal [A-Z0-9+] fragment of a comment and applies the
// §2.2 head rules to it; ParseForIngest is the mode-independent parse entry used by
// ingest (exact kw-v1 first, contains fallback only when exact produced NO_MATCH).
//
// The negation and question tables below are frozen data (§2.5); changing them is a new
// grammar version (kwc-v2), never an in-place edit.

package grammar

import "strings"

// negationFragments is the frozen §2.5 negation table (conservative: "H1+1 不錯" is also
// refused so a seller can re-ask or record manually; the miss direction is safe).
var negationFragments = []string{"不", "沒", "没", "別", "别", "勿", "莫", "取消", "算了", "退"}

// questionFragments is the frozen §2.5 question table. '?' covers both the ASCII form and
// the full-width ？ (already width-mapped to '?' by §2.1 step 1).
var questionFragments = []string{"?", "嗎", "吗", "呢", "嘛", "幾", "几", "多少", "怎", "哪", "什麼", "什么", "啥", "是否", "可否"}

// ParseContains applies kwc-v1 (§2.5) to one comment, in order: §2.1 steps 0–3 on the
// whole string; negation table; question table; exactly one maximal [A-Z0-9+] fragment;
// the §2.2 head/quantity rules on that fragment. It never fails: anything else is
// NO_MATCH, and an out-of-range "+N" on a keyword-shaped head is INVALID_QUANTITY.
// A MATCH/INVALID_QUANTITY result carries VersionContains. Pure; concurrent-safe.
func ParseContains(text string) Result {
	s, ok := canonical(text)
	if !ok {
		return Result{Version: VersionContains, Kind: NoMatch}
	}
	if containsAny(s, negationFragments) || containsAny(s, questionFragments) {
		return Result{Version: VersionContains, Kind: NoMatch}
	}
	fragments := keywordFragments(s)
	if len(fragments) != 1 {
		return Result{Version: VersionContains, Kind: NoMatch}
	}
	head, tail, plus := cutPlus(fragments[0])
	if !isKeyword(head) {
		return Result{Version: VersionContains, Kind: NoMatch}
	}
	if !plus {
		return Result{Version: VersionContains, Kind: Match, Keyword: head, Quantity: 1}
	}
	if tail == "" || !allDigits(tail) {
		return Result{Version: VersionContains, Kind: NoMatch}
	}
	if len(tail) > maxDigitsInQty || tail[0] == '0' {
		return Result{Version: VersionContains, Kind: InvalidQuantity, Keyword: head}
	}
	var quantity int64
	for i := 0; i < len(tail); i++ {
		quantity = quantity*10 + int64(tail[i]-'0')
	}
	return Result{Version: VersionContains, Kind: Match, Keyword: head, Quantity: quantity, Explicit: true}
}

// ParseForIngest is the single mode-independent entry (§2.5 property): exact kw-v1 wins;
// only a kw-v1 NO_MATCH falls through to kwc-v1, and only a non-NO_MATCH contains result
// is returned (so a kwc-v1 miss stays the kw-v1 NO_MATCH). Deterministic and identical for
// every window mode; package claims decides whether a kwc-v1 result applies via effective.
func ParseForIngest(text string) Result {
	p := Parse(text)
	if p.Kind == NoMatch {
		if c := ParseContains(text); c.Kind != NoMatch {
			return c
		}
	}
	return p
}

// containsAny reports whether s contains any of subs. All fragments are ≤2 runes, and s is
// already canonical, so a byte search is correct (no width map is applied to the tables).
func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// keywordFragments splits s into maximal contiguous runs of [A-Z0-9+]. Any other byte is a
// boundary; a multi-byte CJK/emoji/trap rune is therefore a boundary byte for byte, which
// is all a fragment scan needs (it never has to decode them).
func keywordFragments(s string) []string {
	var out []string
	start := -1
	for i := 0; i < len(s); i++ {
		if isFragmentByte(s[i]) {
			if start < 0 {
				start = i
			}
		} else if start >= 0 {
			out = append(out, s[start:i])
			start = -1
		}
	}
	if start >= 0 {
		out = append(out, s[start:])
	}
	return out
}

func isFragmentByte(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '+'
}
