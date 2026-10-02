// Package storehandles owns the store handle grammar (R5 unit store-domains, Decision 1): the platform-wide,
// lower-case ASCII slug under which a store is addressed at https://<handle>.<LC_STORE_BASE_DOMAIN>. It mirrors,
// character-for-character, the SQL helpers in migrations/0106 (control.store_handle_valid / slug_store_handle /
// store_handle_reserved) and control.stores.stores_handle_format, so the Go onboarding preview and the DB assignment
// never disagree.
//
// Owns: handle format, the reserved-word list, the slug and the store-<id8> fallback.
// Never: writes the database (assignment/suffix live in control.assign_store_handle; availability in
//   control.suggest_store_handle), verifies DNS/TLS, or resolves origins (internal/domains).
// Depends on: nothing.
// Used by: internal/identity (onboarding preview), internal/storefrontdomains (hostname/base validation).
package storehandles

import (
	"strings"
	"unicode"
)

// maxLen is the longest handle the DB CHECK admits (first + {1,28} middle + last).
const maxLen = 30

// reservedWords is Decision 1's reserved list (always compared lower-case) plus the punycode prefix.
var reservedWords = map[string]bool{
	"www": true, "admin": true, "api": true, "hooks": true, "shop": true, "mail": true,
	"static": true, "cdn": true, "assets": true, "app": true, "help": true, "support": true,
	"status": true, "stores": true,
}

// Reserved reports whether a lower-cased handle is a reserved platform word or a punycode (xn--) name.
func Reserved(handle string) bool {
	if reservedWords[handle] {
		return true
	}
	return strings.HasPrefix(handle, "xn--")
}

// Valid reports whether handle matches the store handle grammar exactly: 3..30 lower-case ASCII,
// letters/digits and hyphens only, no leading or trailing hyphen, not reserved. Consecutive hyphens are
// admitted (the DB regex [a-z0-9-]{1,28} allows them), so Go and SQL stay in parity.
func Valid(handle string) bool {
	n := len(handle)
	if n < 3 || n > maxLen {
		return false
	}
	for i := 0; i < n; i++ {
		if c := handle[i]; !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	if !alnum(handle[0]) || !alnum(handle[n-1]) {
		return false
	}
	return !Reserved(handle)
}

func alnum(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= '0' && c <= '9'
}

// Slug derives the handle slug from a display name: lower-case, every run of non-alphanumerics becomes one
// hyphen, trimmed. A name with no ASCII alphanumerics (e.g. a Chinese store name) yields the empty string and
// the caller falls back to Fallback. It never emits a leading or trailing hyphen.
func Slug(name string) string {
	var b strings.Builder
	lastHyphen := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastHyphen = false
		case b.Len() > 0 && !lastHyphen:
			b.WriteByte('-')
			lastHyphen = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// Fallback is the store-<first 8 hex of id> handle used when a slug is empty, too short or reserved.
// id is a canonical 32-hex uuid; any other shape returns "" (callers treat that as "no suggestion").
func Fallback(id string) string {
	h := strings.ReplaceAll(strings.ToLower(id), "-", "")
	for _, c := range h {
		if !unicode.Is(unicode.Hex_Digit, c) {
			return ""
		}
	}
	if len(h) < 8 {
		return ""
	}
	return "store-" + h[:8]
}

// Suggest is the onboarding preview: the slug, or the store-<id8> fallback when the slug is not a valid
// handle. It truncates to the DB length and re-trims, exactly as control.suggest_store_handle does.
func Suggest(name, id string) string {
	s := Slug(name)
	if !Valid(s) {
		s = Fallback(id)
		if s == "" {
			return ""
		}
	}
	if len(s) > maxLen {
		s = strings.Trim(s[:maxLen], "-")
	}
	if !Valid(s) {
		return Fallback(id)
	}
	return s
}
