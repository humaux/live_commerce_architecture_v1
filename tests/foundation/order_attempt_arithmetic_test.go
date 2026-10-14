// Purpose: reject a corrupted frozen calculation before C2 can materialize money.
// Depends on: real C1 admission, protected execution fence, plan AEAD and the pricing DTO.
// Used by: OA1 CI; tests exercise the same plan verifier required by the executor.
// Invariants: I08; every per-origin and aggregate amount is verified without repricing.
package foundation_test

import (
	"encoding/json"
	"testing"

	"livecommerce/internal/orderattempt"
)

func oaVerifyCalculation(t *testing.T, plan orderattempt.FrozenPlan) error {
	t.Helper()
	verifier, ok := any(plan).(interface{ VerifyCalculation() error })
	if !ok {
		t.Fatal("C2 plan has no frozen calculation verifier")
	}
	return verifier.VerifyCalculation()
}

func TestOrderAttemptExecutionArithmetic(t *testing.T) {
	e := lbuNew(t)
	a := oaQuoteAdmit(t, e, true)
	_, plain := oaFenceDomainTx(t, e, a, true)
	var original orderattempt.FrozenPlan
	if err := json.Unmarshal(plain, &original); err != nil {
		t.Fatal(err)
	}
	if len(original.Calculation.Lines) < 2 || original.Calculation.Lines[0] == original.Calculation.Lines[1] {
		t.Fatal("positive sentinel needs distinct amounts for two origins of the same SKU")
	}
	if err := oaVerifyCalculation(t, original); err != nil {
		t.Fatalf("real frozen C1 calculation rejected: %v", err)
	}
	mutations := map[string]func(*orderattempt.FrozenPlan){
		"subtotal":      func(p *orderattempt.FrozenPlan) { p.Calculation.SubtotalMinor++ },
		"discount":      func(p *orderattempt.FrozenPlan) { p.Calculation.DiscountMinor++ },
		"shipping":      func(p *orderattempt.FrozenPlan) { p.Calculation.ShippingMinor++ },
		"shipping-tax":  func(p *orderattempt.FrozenPlan) { p.Calculation.ShippingTaxMinor++ },
		"tax":           func(p *orderattempt.FrozenPlan) { p.Calculation.TaxMinor++ },
		"total":         func(p *orderattempt.FrozenPlan) { p.Calculation.TotalMinor++ },
		"line-subtotal": func(p *orderattempt.FrozenPlan) { p.Calculation.Lines[0].SubtotalMinor++ },
		"line-discount": func(p *orderattempt.FrozenPlan) { p.Calculation.Lines[0].DiscountMinor++ },
		"line-tax":      func(p *orderattempt.FrozenPlan) { p.Calculation.Lines[0].TaxMinor++ },
		"line-total":    func(p *orderattempt.FrozenPlan) { p.Calculation.Lines[0].TotalMinor++ },
		"origins-swapped": func(p *orderattempt.FrozenPlan) {
			p.Calculation.Lines[0], p.Calculation.Lines[1] = p.Calculation.Lines[1], p.Calculation.Lines[0]
		},
		"origin-dropped": func(p *orderattempt.FrozenPlan) { p.Calculation.Lines = p.Calculation.Lines[:1] },
		"origin-added": func(p *orderattempt.FrozenPlan) {
			p.Calculation.Lines = append(p.Calculation.Lines, p.Calculation.Lines[0])
		},
		"facts-invalid": func(p *orderattempt.FrozenPlan) { p.Facts = json.RawMessage(`{}`) },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			var candidate orderattempt.FrozenPlan
			if err := json.Unmarshal(plain, &candidate); err != nil {
				t.Fatal(err)
			}
			mutate(&candidate)
			if err := oaVerifyCalculation(t, candidate); err == nil {
				t.Fatal("corrupt frozen calculation accepted for C2")
			}
		})
	}
	if err := oaVerifyCalculation(t, original); err != nil {
		t.Fatal("verification mutated the original frozen plan")
	}
}
