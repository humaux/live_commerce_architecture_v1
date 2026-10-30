// Purpose: FEAT-MANUAL-DISCOUNT validation and canonical replay inputs before database work.
// Depends on: the public ManualInput JSON boundary and ValidateManual; synthetic fixtures only.
// Used by: CI unit gate; no provider, database or local test server.
// Invariants: only a named positive integer discount is accepted; no item price override.
package merchanttools

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestManualDiscountValidation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		discount any
		valid    bool
	}{
		{"trimmed", map[string]any{"name": "  直播優惠  ", "amount_minor": 100}, true},
		{"thirty unicode characters", map[string]any{"name": strings.Repeat("惠", 30), "amount_minor": 1}, true},
		{"empty", map[string]any{"name": "", "amount_minor": 100}, false},
		{"whitespace", map[string]any{"name": " \t ", "amount_minor": 100}, false},
		{"over thirty", map[string]any{"name": strings.Repeat("惠", 31), "amount_minor": 100}, false},
		{"control", map[string]any{"name": "直播\n優惠", "amount_minor": 100}, false},
		{"zero", map[string]any{"name": "直播優惠", "amount_minor": 0}, false},
		{"negative", map[string]any{"name": "直播優惠", "amount_minor": -100}, false},
		{"fraction", map[string]any{"name": "直播優惠", "amount_minor": 1.5}, false},
		{"string amount", map[string]any{"name": "直播優惠", "amount_minor": "100"}, false},
		{"money overflow", map[string]any{"name": "直播優惠", "amount_minor": int64(1_000_000_000_001)}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(validManual())
			if err != nil {
				t.Fatal(err)
			}
			var body map[string]any
			if err = json.Unmarshal(raw, &body); err != nil {
				t.Fatal(err)
			}
			body["discount"] = tc.discount
			raw, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			var in ManualInput
			if err = json.Unmarshal(raw, &in); err != nil {
				if tc.valid {
					t.Fatal(err)
				}
				return // A fractional/string integer is refused at the JSON trust boundary.
			}
			got, err := ValidateManual(in)
			if !tc.valid {
				var coded *Error
				if !errors.As(err, &coded) || coded.Status != 422 || coded.Code != "invalid_request" {
					t.Fatalf("discount must be refused as 422 invalid_request, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			raw, err = json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			var normalized struct {
				Discount *struct {
					Name   string `json:"name"`
					Amount int64  `json:"amount_minor"`
				} `json:"discount"`
			}
			if err = json.Unmarshal(raw, &normalized); err != nil {
				t.Fatal(err)
			}
			want := tc.discount.(map[string]any)
			if normalized.Discount == nil || normalized.Discount.Name != strings.TrimSpace(want["name"].(string)) || normalized.Discount.Amount != int64(want["amount_minor"].(int)) {
				t.Fatalf("normalized input lost its named discount: %s", raw)
			}
		})
	}
}
