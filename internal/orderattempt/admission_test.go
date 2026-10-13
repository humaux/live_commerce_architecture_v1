// Purpose: independent money and origin counterexamples for C1's pure assembly.
// Depends: frozen admission facts DTO, existing pricing calculator; no DB fixture.
// Used: OA1 CI before wiring the admission service into any process.
// Invariants: duplicate SKU origins stay separate; exact live-price bindings and
// one shipping/COD fee application; no silent fallback on malformed domain facts.
package orderattempt

import (
	"encoding/json"
	"testing"
)

func admissionMoneyFacts(t *testing.T) map[string]any {
	t.Helper()
	var facts map[string]any
	err := json.Unmarshal([]byte(`{"catalog":{"policy":{"market_id":"11111111-1111-1111-1111-111111111111","country":"TW","method":"delivery:home","currency":"TWD","shipping_mode":"flat","tax_mode":"none","tax_basis":"goods","version":1,"shipping_minor":1000,"tax_rate_bps":0,"quote_ttl_seconds":900,"enabled":true},"payment":{"mode":"bank_transfer"},"lines":[{"ordinal":1,"sku_id":"22222222-2222-2222-2222-222222222222","origin_id":"33333333-3333-3333-3333-333333333333","bundle_id":"44444444-4444-4444-4444-444444444444","offer_id":"55555555-5555-5555-5555-555555555555","quantity":2,"unit_price_minor":30000,"currency":"TWD"},{"ordinal":2,"sku_id":"22222222-2222-2222-2222-222222222222","origin_id":"66666666-6666-6666-6666-666666666666","quantity":3,"unit_price_minor":30000,"currency":"TWD"}]},"claims":{"lines":[{"ordinal":1,"sku_id":"22222222-2222-2222-2222-222222222222","origin_id":"33333333-3333-3333-3333-333333333333","bundle_id":"44444444-4444-4444-4444-444444444444","offer_id":"55555555-5555-5555-5555-555555555555","quantity":2,"live_price_minor":25000,"reason":""}]}}`), &facts)
	if err != nil { t.Fatal(err) }
	return facts
}

func TestAdmissionCalculationPreservesOriginsAndFees(t *testing.T) {
	facts := admissionMoneyFacts(t)
	raw, _ := json.Marshal(facts)
	got, err := admissionCalculation(raw)
	if err != nil || len(got.Lines) != 2 || got.Lines[0].SubtotalMinor != 50000 || got.Lines[1].SubtotalMinor != 90000 || got.ShippingMinor != 1000 || got.TotalMinor != 141000 {
		t.Fatalf("per-origin money: %+v %v", got, err)
	}
	catalog := facts["catalog"].(map[string]any)
	catalog["payment"] = map[string]any{"mode":"cash_on_delivery", "surcharge_minor":5000, "max_minor":146000}
	raw, _ = json.Marshal(facts)
	got, err = admissionCalculation(raw)
	if err != nil || got.TotalMinor != 141000 { t.Fatalf("COD surcharge folded into quote or cap not honored: %+v %v", got, err) }
	catalog["payment"].(map[string]any)["max_minor"] = 145900
	raw, _ = json.Marshal(facts)
	if _, err = admissionCalculation(raw); err == nil { t.Fatal("COD collection cap ignored") }
}

func TestAdmissionCalculationRejectsMismatchedClaimAndNeverMerges(t *testing.T) {
	for _, field := range []string{"ordinal","sku_id","origin_id","bundle_id","offer_id","quantity","live_price_minor"} {
		t.Run(field, func(t *testing.T) {
			facts := admissionMoneyFacts(t)
			line := facts["claims"].(map[string]any)["lines"].([]any)[0].(map[string]any)
			switch field {
			case "ordinal", "quantity": line[field] = 3
			case "live_price_minor": line[field] = -1
			default: line[field] = "77777777-7777-7777-7777-777777777777"
			}
			raw, _ := json.Marshal(facts)
			if _, err := admissionCalculation(raw); err == nil { t.Fatal("malformed claim became a valid price") }
		})
	}
	facts := admissionMoneyFacts(t)
	line := facts["claims"].(map[string]any)["lines"].([]any)[0].(map[string]any)
	line["live_price_minor"] = nil
	line["reason"] = "no_live_price"
	raw, _ := json.Marshal(facts)
	got, err := admissionCalculation(raw)
	if err != nil || len(got.Lines) != 2 || got.TotalMinor != 151000 { t.Fatalf("explicit catalog fallback: %+v %v", got, err) }
}
