// Purpose: reject unexplained live-price fallback in frozen C1 money assembly.
// Depends on: admissionCalculation and the existing independent money fixture.
// Used by: OA1 Go-area review CI before product changes.
// Invariants: a null live price requires an explicit authoritative fallback reason.

package orderattempt

import (
	"encoding/json"
	"testing"
)

func TestAreaGoAdmissionNullLivePriceRequiresReason(t *testing.T) {
	for _, reason := range []string{"", "no_live_price"} {
		t.Run("reason="+reason, func(t *testing.T) {
			facts := admissionMoneyFacts(t)
			line := facts["claims"].(map[string]any)["lines"].([]any)[0].(map[string]any)
			line["live_price_minor"], line["reason"] = nil, reason
			raw, err := json.Marshal(facts)
			if err != nil {
				t.Fatal(err)
			}
			got, err := admissionCalculation(raw)
			if reason == "" {
				if err == nil {
					t.Fatal("null live price silently became a catalog charge without a reason")
				}
			} else if err != nil || got.TotalMinor != 151000 {
				t.Fatal("explicit catalog fallback must retain the independently expected total", err)
			}
		})
	}
}
