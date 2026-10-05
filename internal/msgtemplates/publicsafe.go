// Purpose: the §3.5 public-reply content rule (contracts/live-console-v1.md §3.5): the single shared validator that
// both the publish path (LC-B5) and the send path (LC-B4) call before a template body is usable as a public reply or
// a recommend comment. Matching runs on the §3.5-normalized copy — NFKC + case-fold + strip Cf + remove whitespace +
// remove dashes between digits — and the stored text is never altered. It returns a fixed reason (never driver text).
// Depends on: golang.org/x/text (norm.NFKC/norm.NFC, cases.Fold).
// Used by: publish.go (public_safe flag enforcement), LC-B4's send path, and the unit tests.

package msgtemplates

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// Fixed §3.5 refusal reasons (internal to ValidatePublicSafe; the transport code is always
// public_reply_forbidden_content). Never driver text, never a customer value.
const (
	reasonURL         = "url"
	reasonDomain      = "domain"
	reasonLineID      = "line_id"
	reasonHandle      = "handle"
	reasonPhone       = "phone"
	reasonEmail       = "email"
	reasonBuyerVar    = "buyer_variable"
	reasonLinkPlace   = "link_placeholder"
	reasonStoreOrigin = "store_origin"
	reasonLength      = "too_long"
	reasonEmpty       = "empty"
)

var (
	// bareDomain covers t.me/wa.me too (they are domain tokens with the me TLD) and is a substring check on the
	// whitespace-stripped copy, so "example . com" collapses to "example.com" and is caught.
	bareDomain = regexp.MustCompile(`[a-z0-9-]+\.(com|tw|hk|cn|net|org|shop|store|me|io|app|link|ly)`)
	// lineBare matches a bare "line"/"lineid" token bounded by non-alphanumerics (Go's RE2 has no \b). The contiguous
	// "lineid" substring (which loses its boundaries after whitespace removal, e.g. "my line id is …" → "mylineidis")
	// is checked separately below.
	lineBare = regexp.MustCompile(`(^|[^a-z0-9])line(id)?([^a-z0-9]|$)`)
	emailRe  = regexp.MustCompile(`[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}`)
	handleRe = regexp.MustCompile(`@[a-z0-9_.]{1,64}`)
	phoneRun = regexp.MustCompile(`[0-9]{8,}`)
	// dashBetweenDigits collapses the dashes between ASCII/Unicode digits (§3.5 "remove dashes between digits");
	// a repeated pass also collapses multi-dash and alternating-digit runs.
	dashBetweenDigits = regexp.MustCompile(`([0-9])[-‐-―]+([0-9])`)
)

// ValidatePublicSafe runs the §3.5 content rule and returns "" when text is safe, else a fixed reason code.
// storeOrigin is the store's own origin (the publish path passes ""; the send path passes the caller's origin).
// The length bound is 1..300 runes after NFC, exactly as §3.5 states.
func ValidatePublicSafe(text string, storeOrigin string) string {
	if n := utf8.RuneCountInString(norm.NFC.String(text)); n > 300 {
		return reasonLength
	}
	s := normalizeForMatch(text)
	// Empty after normalization (only spaces/zero-width/etc.) is still empty content.
	if s == "" {
		return reasonEmpty
	}
	if strings.Contains(s, "https://") || strings.Contains(s, "http://") {
		return reasonURL
	}
	// email before bare domain/handle: an email embeds a domain and an @, so the more specific reason wins.
	if emailRe.MatchString(s) {
		return reasonEmail
	}
	if strings.Contains(s, "www.") || bareDomain.MatchString(s) {
		return reasonDomain
	}
	if lineBare.MatchString(s) || strings.Contains(s, "lineid") {
		return reasonLineID
	}
	if handleRe.MatchString(s) {
		return reasonHandle
	}
	if phoneRun.MatchString(s) {
		return reasonPhone
	}
	if strings.Contains(s, "{{order.") || strings.Contains(s, "{{buyer.") || strings.Contains(s, "{{link.") {
		return reasonBuyerVar
	}
	if strings.Contains(s, "{{連結}}") || strings.Contains(s, "結帳{{") || strings.Contains(s, "付款連結{{") {
		return reasonLinkPlace
	}
	if storeOrigin != "" && strings.Contains(s, normalizeForMatch(storeOrigin)) {
		return reasonStoreOrigin
	}
	return ""
}

// normalizeForMatch is the §3.5 matching normalization: NFKC, case-fold, strip Cf (zero-width etc.), remove all
// whitespace, then collapse dashes between digits. The whitespace removal is deliberately broader than "between
// digits": it is what makes "l i n e", "w w w . example . com" and "09 12 345 678" fail closed as one token.
func normalizeForMatch(text string) string {
	s := norm.NFKC.String(text)
	s = cases.Fold().String(s)
	s = strings.Map(func(r rune) rune {
		if unicode.In(r, unicode.Cf) || unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
	for {
		next := dashBetweenDigits.ReplaceAllString(s, "${1}${2}")
		if next == s {
			return s
		}
		s = next
	}
}
