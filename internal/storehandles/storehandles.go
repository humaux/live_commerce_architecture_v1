// Package storehandles owns the explicit operator handle grammar and reserved names.
// Automatic onboarding numbers are assigned only by control.assign_store_handle (0106).
// It never writes the database or verifies DNS/TLS. store-admin handle-set retains vanity handles.
package storehandles

import "strings"

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
