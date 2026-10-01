package checkout

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/command"
	"livecommerce/internal/fulfillment"
)

// storefront-v2 §C: the third payment mode and the optional buyer email on the Begin input.
func TestBeginInputBankTransferAndEmail(t *testing.T) {
	in := Input{QuoteID: testID, DestinationID: testID, CartVersion: 1, ServiceVersion: 1, AllocationVersion: 1, PaymentMode: "bank_transfer"}
	if !validInput(in) {
		t.Fatal("bank_transfer rejected")
	}
	if commercialAtPlacement("bank_transfer") != "AWAITING_TRANSFER" || commercialAtPlacement("card") != "DRAFT" || commercialAtPlacement("pay_at_pickup") != "CONFIRMED" {
		t.Fatal("placement state: a transfer order waits for the merchant, it is never CONFIRMED at placement")
	}
	for _, ok := range []string{"", "a@b.co", "buyer+tag@example.com", "x.y@sub.example.org"} {
		in.BuyerEmail = ok
		if !validInput(in) {
			t.Errorf("email %q rejected", ok)
		}
	}
	for _, bad := range []string{"plain", "a@", "@b.co", "a b@c.co", "Name <a@b.co>", "a@b.co ", " a@b.co", "a@b.co,c@d.co", "a@@b.co", "a\n@b.co",
		"(c)a@b.co", `"q"@b.co`, strings.Repeat("a", 250) + "@b.co"} {
		in.BuyerEmail = bad
		if validInput(in) {
			t.Errorf("email %q accepted", bad)
		}
	}
	// omitempty: a request without an email digests exactly as before the migration, so old replays still match.
	in.BuyerEmail, in.PaymentMode = "", ""
	raw, _ := json.Marshal(in)
	if strings.Contains(string(raw), "buyer_email") || strings.Contains(string(raw), "payment_mode") {
		t.Fatalf("empty optional fields changed the request digest: %s", raw)
	}
}

// begin_hold answers a transfer hold of 6..168 hours; card and pay_at_pickup keep the 15-minute bound.
func TestHoldWithinBounds(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		mode string
		hold time.Duration
		ok   bool
	}{
		{"card", 15 * time.Minute, true}, {"card", 72 * time.Hour, false}, {"pay_at_pickup", 15 * time.Minute, true},
		{"bank_transfer", 72 * time.Hour, true}, {"bank_transfer", 6 * time.Hour, true}, {"bank_transfer", 168 * time.Hour, true},
		{"bank_transfer", 15 * time.Minute, false}, {"bank_transfer", 169 * time.Hour, false},
	}
	for _, c := range cases {
		if got := holdWithinBounds(c.mode, now, now.Add(c.hold)); got != c.ok {
			t.Errorf("%s %v: got %v want %v", c.mode, c.hold, got, c.ok)
		}
	}
}

func TestValidProof(t *testing.T) {
	good := ProofInput{Last5: "01234", AmountMinor: 150000, PaidAt: time.Now()}
	if !validProof(good) {
		t.Fatal("good proof rejected")
	}
	for name, edit := range map[string]func(*ProofInput){
		"short digits":  func(p *ProofInput) { p.Last5 = "1234" },
		"letters":       func(p *ProofInput) { p.Last5 = "12a45" },
		"six digits":    func(p *ProofInput) { p.Last5 = "123456" },
		"zero amount":   func(p *ProofInput) { p.AmountMinor = 0 },
		"huge amount":   func(p *ProofInput) { p.AmountMinor = maxProofAmount + 1 },
		"no paid_at":    func(p *ProofInput) { p.PaidAt = time.Time{} },
		"negative sign": func(p *ProofInput) { p.AmountMinor = -5 },
	} {
		p := good
		edit(&p)
		if validProof(p) {
			t.Errorf("%s accepted", name)
		}
	}
	// Refused before any SQL: a nil pool service must answer ErrInvalid, never touch the database.
	svc := &Service{}
	if _, err := svc.SubmitTransferProof(context.Background(), "tok", testID, "short", testID, good); !errors.Is(err, command.ErrInvalid) {
		t.Fatalf("pool-less service: %v", err)
	}
}

func TestValidTransferView(t *testing.T) {
	now := time.Now()
	base := func() TransferView {
		return TransferView{OrderID: testID, State: "AWAITING", WindowHours: 72, DeadlineAt: now, Currency: "TWD", AmountMinor: 90000,
			Bank: &TransferBank{BankName: "B", AccountNumber: "123456"}}
	}
	if !validTransferView(base(), testID) {
		t.Fatal("awaiting view rejected")
	}
	reason := "amount mismatch"
	proof := &TransferProof{Last5: "12345", AmountMinor: 90000, PaidAt: now, SubmittedAt: now, Count: 1}
	for name, edit := range map[string]func(*TransferView){
		"another order":              func(v *TransferView) { v.OrderID = "00000000-0000-0000-0000-000000000009" },
		"unknown state":              func(v *TransferView) { v.State = "PAID" },
		"window out of range":        func(v *TransferView) { v.WindowHours = 5 },
		"expired keeps bank details": func(v *TransferView) { v.State = "EXPIRED" },
		"awaiting hides bank":        func(v *TransferView) { v.Bank = nil },
		"submitted without proof":    func(v *TransferView) { v.State = "SUBMITTED" },
		"rejected without reason":    func(v *TransferView) { v.State = "REJECTED"; v.Proof = proof },
		"reason on awaiting":         func(v *TransferView) { v.RejectReason = &reason },
		"confirmed without time":     func(v *TransferView) { v.State = "CONFIRMED" },
		"time on awaiting":           func(v *TransferView) { v.ConfirmedAt = &now },
		"bad proof digits":           func(v *TransferView) { p := *proof; p.Last5 = "x"; v.Proof = &p },
	} {
		v := base()
		edit(&v)
		if validTransferView(v, testID) {
			t.Errorf("%s accepted", name)
		}
	}
	rejected := base()
	rejected.State, rejected.RejectReason, rejected.Proof = "REJECTED", &reason, proof
	if !validTransferView(rejected, testID) {
		t.Fatal("rejected view with reason rejected")
	}
	expired := base()
	expired.State, expired.Bank = "EXPIRED", nil
	if !validTransferView(expired, testID) {
		t.Fatal("expired view without bank details rejected")
	}
}

// The coded refusals of the 0088 buyer definers reach the HTTP layer as 422 codes, never as a generic failure.
func TestTransferRefusalsAreCoded(t *testing.T) {
	for _, code := range []string{"transfer_not_open", "transfer_window_closed", "invalid_proof", "not_bank_transfer", "bank_transfer_unavailable"} {
		err := safeError(context.Background(), &pgconn.PgError{Code: "PT422", Message: code})
		var refusal *fulfillment.CVSError
		if !errors.As(err, &refusal) || refusal.Status != 422 || refusal.Code != code {
			t.Errorf("%s: %v", code, err)
		}
	}
}
