// Purpose: kwc-v2 restricted contains grammar (ParseContainsV2): kwc-v1 plus the K3 adversarial fixes F1 (question words), F2 (boundary-tolerant negation/question matching) and F3 (lookalike-letter carve rejection).
// Depends on: pure Go (no I/O); contains.go frozen kwc-v1 tables and fragment scan; grammar.go canonical/isKeyword/cutPlus/allDigits; stdlib unicode.
// Used by: ParseForIngest (contains.go) for every NEW parse; tests/claims kwc adversarial corpus; internal/claims via grammar.IsContainsVersion.
// Invariants: I02 (deterministic per stored grammar version; kwc-v1 stays untouched for replay); §2 "Parsing is mode-independent"; safe-miss direction (any doubt is NO_MATCH, never an order).
//
// kwc-v2 is a NEW grammar version, not an edit of kwc-v1 (§2.5: tables are frozen data). It
// only ever turns a kwc-v1 MATCH/INVALID_QUANTITY into NO_MATCH, never the reverse, so the
// "Parse != NO_MATCH => ParseContains agrees" property still holds (an exact kw-v1 comment
// is pure ASCII [A-Z0-9+], none of the new rules can fire on it).

package grammar

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// questionFragmentsV2 is the kwc-v2 question table: every kwc-v1 entry plus 問 (covers 請問),
// 如何 and 價格/價錢 (and their simplified forms). Built from the frozen v1 slice so v1 can
// never be dropped by accident.
var questionFragmentsV2 = append(append([]string{}, questionFragments...),
	"問", "问", "如何", "價格", "价格", "價錢", "价钱")

// ParseContainsV2 applies kwc-v2 to one comment. Same steps as ParseContains, plus:
//  1. negation/question entries are also searched in the "skeleton" of the text (letters and
//     numbers only), so a space, emoji, punctuation or zero-width rune between the characters
//     of a two-character entry (取 消, 取😍消) cannot evade the table;
//  2. a fragment directly preceded or followed by a non-CJK non-ASCII letter/number/mark
//     (Cyrillic А, Greek Α, ...) is NO_MATCH: such a rune is a lookalike of a keyword
//     character, and carving the remaining digits into another keyword would order the
//     wrong product;
//  3. an all-digit head with an explicit "+N" is NO_MATCH unless the whole comment is ASCII
//     (numeric keywords are the carve target of (2) when the lookalike is a symbol such as Ⓐ).
//
// A MATCH/INVALID_QUANTITY result carries VersionContainsV2. Pure; concurrent-safe.
func ParseContainsV2(text string) Result {
	noMatch := Result{Version: VersionContainsV2, Kind: NoMatch}
	s, ok := canonical(text)
	if !ok {
		return noMatch
	}
	skeleton := skeletonOf(s)
	for _, hay := range [2]string{s, skeleton} {
		if containsAny(hay, negationFragments) || containsAny(hay, questionFragmentsV2) {
			return noMatch
		}
	}
	spans := keywordSpans(s)
	if len(spans) != 1 || lookalikeAdjacent(s, spans[0]) {
		return noMatch
	}
	head, tail, plus := cutPlus(s[spans[0][0]:spans[0][1]])
	if !isKeyword(head) {
		return noMatch
	}
	// Defence in depth for (3): symbols that are neither letters nor numbers (Ⓐ, 🅰) can still
	// carve "01+2" out of a lookalike keyword. Only an explicit "+N" is an order candidate
	// (an implicit match is QUANTITY_REQUIRED at ingest, K3A-D01 "1000元" stays a grammar MATCH).
	if plus && allDigits(head) && hasNonASCII(s) {
		return noMatch
	}
	if !plus {
		return Result{Version: VersionContainsV2, Kind: Match, Keyword: head, Quantity: 1}
	}
	if tail == "" || !allDigits(tail) {
		return noMatch
	}
	if len(tail) > maxDigitsInQty || tail[0] == '0' {
		return Result{Version: VersionContainsV2, Kind: InvalidQuantity, Keyword: head}
	}
	var quantity int64
	for i := 0; i < len(tail); i++ {
		quantity = quantity*10 + int64(tail[i]-'0')
	}
	return Result{Version: VersionContainsV2, Kind: Match, Keyword: head, Quantity: quantity, Explicit: true}
}

// skeletonOf keeps only letters and numbers of s (any script), dropping spaces, emoji,
// punctuation, marks and zero-width format runes. The ASCII '?' is checked on s itself.
func skeletonOf(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// lookalikeAdjacent reports whether the rune right before or right after the fragment span
// is a non-ASCII keyword lookalike (see isLookalike).
func lookalikeAdjacent(s string, span [2]int) bool {
	before, _ := utf8.DecodeLastRuneInString(s[:span[0]]) // RuneError (not a lookalike) at the start
	after, _ := utf8.DecodeRuneInString(s[span[1]:])
	return isLookalike(before) || isLookalike(after)
}

// isLookalike is true for a non-ASCII letter, number or combining mark outside the CJK
// scripts: those are the runes (Cyrillic, Greek, Latin extensions, other digits...) that can
// pass for an ASCII keyword character. CJK text around a keyword ("我要A01+2") is normal.
func isLookalike(r rune) bool {
	if r < 0x80 || !(unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r)) {
		return false
	}
	return !unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul, unicode.Bopomofo)
}

// hasNonASCII reports whether s contains any byte >= 0x80.
func hasNonASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return true
		}
	}
	return false
}
