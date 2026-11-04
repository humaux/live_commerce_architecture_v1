// Purpose: named fixed merchant adjustments and exact, bounded line allocations (cart-quote-v1 amendment).
// Depends on: command money bounds and standard-library Unicode/128-bit integer arithmetic.
// Used by: merchanttools input validation and storefront quote creation/revalidation.
// Invariants: goods only; shares conserve the discount and never exceed a line's subtotal; no item price override.

package pricing

import (
	"math/bits"
	"strings"
	"unicode"
	"unicode/utf8"

	"livecommerce/internal/command"
)

type ManualDiscount struct {
	Name        string `json:"name"`
	AmountMinor int64  `json:"amount_minor"`
}

type Adjustment struct {
	Kind     string `json:"kind"`
	Currency string `json:"currency"`
	ManualDiscount
}

type DiscountError struct{ Code string }

func (e *DiscountError) Error() string { return "pricing: " + e.Code }

func ValidManualDiscount(d ManualDiscount) bool {
	if !utf8.ValidString(d.Name) || strings.TrimSpace(d.Name) != d.Name || utf8.RuneCountInString(d.Name) < 1 || utf8.RuneCountInString(d.Name) > 30 || d.AmountMinor < 1 || d.AmountMinor > command.MaxMoney {
		return false
	}
	for _, r := range d.Name {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

// Floor each proportional share with a 128-bit product, then round into the last
// line. If it cannot absorb the remainder, spill backwards without overdrawing it.
func allocateManualDiscount(discount int64, subtotals []int64, total int64) []int64 {
	out := make([]int64, len(subtotals))
	if discount == 0 || total == 0 {
		return out
	}
	var given int64
	for i, sub := range subtotals {
		hi, lo := bits.Mul64(uint64(discount), uint64(sub))
		q, _ := bits.Div64(hi, lo, uint64(total))
		out[i] = int64(q)
		given += out[i]
	}
	left := discount - given
	for i := len(out) - 1; i >= 0 && left > 0; i-- {
		extra := min(left, subtotals[i]-out[i])
		out[i] += extra
		left -= extra
	}
	return out
}
