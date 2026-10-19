// hosted_test.go asserts removal before PG/queue access; Stripe validators remain in stripe_test.go.
package checkout

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"livecommerce/internal/command"
	"testing"
)

func TestRemovedHostedMethodsFailBeforePGOrQueue(t *testing.T) {
	// A syntactically valid old request with structurally present dependencies must be refused
	// without touching the deliberately uninitialized pool or queue (either access would panic).
	s := &HostedPaymentStarter{starter: PaymentStarter{pool: new(pgxpool.Pool), jobs: new(river.Client[pgx.Tx]), profile: "SANDBOX"}}
	for _, code := range []string{"payuni_credit", "payuni_installment", "payuni_atm", "payuni_cvs", "payuni_linepay"} {
		in := HostedInput{OrderID: testID, MethodCode: code, MethodVersion: 1, Locale: "zh-TW"}
		if validHostedInput(in) {
			t.Fatalf("removed method admitted: %s", code)
		}
		out, err := s.BeginHosted(context.Background(), "token", testID, "validkey1", in)
		if !errors.Is(err, command.ErrInvalid) || out != (PaymentResult{}) {
			t.Fatalf("removed start returned material: %+v %v", out, err)
		}
	}
}

func TestRemovedPaymentProjectionPreservesHistory(t *testing.T) {
	for _, state := range []string{"PENDING", "AUTHORIZED", "CAPTURED", "REVIEW_REQUIRED"} {
		view := OrderPayment{OrderID: testID, Currency: "TWD", TotalMinor: 2500, CommercialState: "AWAITING_PAYMENT", PaymentState: state, HandoffState: "UNAVAILABLE", Methods: []PaymentMethodOption{{Code: "payuni_credit", Version: 1}}}
		view.Methods = withoutRemovedMethods(view.Methods)
		if len(view.Methods) != 0 || view.PaymentState != state || !validPaymentView(view, testID) {
			t.Fatalf("history was hidden or changed: %+v", view)
		}
	}
	for _, code := range []string{"payuni_credit", "payuni_installment", "payuni_atm", "payuni_cvs", "payuni_linepay"} {
		input := []PaymentMethodOption{{Code: code}, {Code: "stripe_checkout", Version: 1}, {Code: "unknown"}}
		got := withoutRemovedMethods(input)
		if len(got) != 2 || got[0].Code != "stripe_checkout" || got[1].Code != "unknown" || input[0].Code != code {
			t.Fatalf("retired projection filter changed another provider: %+v", got)
		}
	}
	if withoutRemovedMethods(nil) != nil {
		t.Fatal("malformed nil methods must remain invalid")
	}
}
