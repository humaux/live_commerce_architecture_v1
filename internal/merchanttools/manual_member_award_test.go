// Purpose: DB-free gates of the optional member_award key of POST orders/manual (loyalty-points-v1 LP7/LP8/LP16): input bounds,
//   the key is part of the hashed request (and absent keeps old hashes), the answer carries the award outcome.
// Depends on: ValidateManual, ManualInput / ManualResult JSON shapes, internal/loyalty (Award, AwardOutcome).
// Used by: go test ./internal/merchanttools. Database behaviour: tests/foundation/loyalty_points_test.go.

package merchanttools

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"livecommerce/internal/loyalty"
	"livecommerce/internal/storefront"
)

func lpManualInput(award *loyalty.Award) ManualInput {
	return ManualInput{
		Items:       []ManualItem{{SKUID: "22222222-2222-4222-8222-222222222222", Quantity: 1}},
		Customer:    ManualCustomer{Name: "測試會員", Phone: "0900000000"},
		Delivery:    ManualDelivery{OptionKey: "22222222-2222-4222-8222-222222222222|TW|home", HomeAddress: &storefront.HomeAddress{City: "測試市", Line1: "測試路1號"}},
		PaymentMode: "bank_transfer", Locale: "zh-TW", MemberAward: award,
	}
}

func TestManualMemberAwardBounds(t *testing.T) {
	const member = "33333333-3333-4333-8333-333333333333"
	for _, c := range []struct {
		name  string
		award loyalty.Award
		ok    bool
	}{
		{"valid", loyalty.Award{MemberID: member, Points: 30}, true},
		{"one point", loyalty.Award{MemberID: member, Points: 1}, true},
		{"the 1,000,000 cap", loyalty.Award{MemberID: member, Points: 1_000_000}, true},
		{"zero points", loyalty.Award{MemberID: member, Points: 0}, false},
		{"negative points", loyalty.Award{MemberID: member, Points: -5}, false},
		{"over the cap", loyalty.Award{MemberID: member, Points: 1_000_001}, false},
		{"not a uuid", loyalty.Award{MemberID: "member-1", Points: 5}, false},
		{"upper-case uuid", loyalty.Award{MemberID: strings.ToUpper(strings.Replace(member, "33333333", "aaaaaaaa", 1)), Points: 5}, false},
	} {
		award := c.award
		_, err := ValidateManual(lpManualInput(&award))
		var coded *Error
		switch {
		case c.ok && err != nil:
			t.Errorf("%s: refused: %v", c.name, err)
		case !c.ok && (err == nil || !errors.As(err, &coded) || coded.Status != 422 || coded.Code != "invalid_request"):
			t.Errorf("%s: %v, want 422 invalid_request", c.name, err)
		}
	}
	if _, err := ValidateManual(lpManualInput(nil)); err != nil {
		t.Errorf("no award: %v", err)
	}
}

// The award is inside the request command.Run hashes: present and absent bodies differ, and an absent key leaves the old JSON (and
// so every stored pre-LP receipt hash) byte-identical.
func TestManualMemberAwardIsHashed(t *testing.T) {
	without, _ := json.Marshal(lpManualInput(nil))
	if strings.Contains(string(without), "member_award") {
		t.Fatalf("absent award must not appear in the hashed request: %s", without)
	}
	with, _ := json.Marshal(lpManualInput(&loyalty.Award{MemberID: "33333333-3333-4333-8333-333333333333", Points: 30}))
	if !strings.Contains(string(with), `"member_award":{"member_id":"33333333-3333-4333-8333-333333333333","points":30}`) {
		t.Fatalf("award missing from the hashed request: %s", with)
	}
	out, _ := json.Marshal(ManualResult{manualReceipt: manualReceipt{OrderID: "o", MemberAward: &loyalty.AwardOutcome{MemberID: "m", Points: 3, State: "not_awarded", Reason: "member_erased"}}})
	if !strings.Contains(string(out), `"member_award":{"member_id":"m","points":3,"state":"not_awarded","reason":"member_erased"}`) {
		t.Fatalf("answer lacks the award outcome: %s", out)
	}
}
