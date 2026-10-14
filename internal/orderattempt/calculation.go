// Purpose: verify C2's frozen money against the exact C1 facts before any write.
// Depends on: admissionCalculation and the existing pricing.Calculation algorithm.
// Used by: the Go execution transaction after authenticated plan decryption.
// Invariants: I08; preserve each origin/ordinal, honor frozen price/fee versions,
// and reject changed aggregates or lines without consulting current catalog data.

package orderattempt

import "slices"

// VerifyCalculation checks all frozen amounts without changing the plan or
// reading a database. Decryption establishes provenance; this separate check
// prevents the executor from materializing an internally inconsistent snapshot.
func (p FrozenPlan) VerifyCalculation() error {
	computed, err := admissionCalculation(p.Facts)
	if err != nil {
		return ErrPlanInvalid
	}
	stored := p.Calculation
	if computed.SubtotalMinor != stored.SubtotalMinor || computed.DiscountMinor != stored.DiscountMinor ||
		computed.ShippingMinor != stored.ShippingMinor || computed.ShippingTaxMinor != stored.ShippingTaxMinor ||
		computed.TaxMinor != stored.TaxMinor || computed.TotalMinor != stored.TotalMinor ||
		!slices.Equal(computed.Lines, stored.Lines) {
		return ErrPlanInvalid
	}
	return nil
}
