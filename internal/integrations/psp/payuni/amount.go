package payuni

// AmountTWDFromMinor converts the order's two-decimal TWD minor units to PAYUNi's
// integer-yuan TradeAmt. Never round here: that would change the frozen contract
// with the buyer. Method-specific limits and account eligibility are separate.
func AmountTWDFromMinor(currency string, amountMinor int64) (int64, error) {
	if currency != "TWD" || amountMinor <= 0 || amountMinor%100 != 0 {
		return 0, ErrInvalid
	}
	return amountMinor / 100, nil
}
