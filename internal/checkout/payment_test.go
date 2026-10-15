package checkout

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"livecommerce/internal/command"
)

func TestPaymentInputAndProfileBoundary(t *testing.T) {
	s := &PaymentStarter{pool: new(pgxpool.Pool), jobs: new(river.Client[pgx.Tx]), profile: "SANDBOX"}
	for _, code := range []string{"payuni_credit", "payuni_installment", "payuni_atm", "payuni_cvs", "payuni_linepay", "stripe_checkout"} {
		out, err := s.StartPayment(context.Background(), "token", testID, "validkey1", PaymentInput{OrderID: testID, MethodCode: code, MethodVersion: 1})
		if !errors.Is(err, command.ErrInvalid) || out != (PaymentResult{}) {
			t.Fatalf("retired non-hosted start admitted %s: %+v %v", code, out, err)
		}
	}
	for _, profile := range []string{"PROVIDER_MOCK", "SANDBOX", "LIVE"} {
		if !validPaymentProfile(profile) {
			t.Fatalf("valid profile %s rejected", profile)
		}
	}
	for _, profile := range []string{"", "mock", "SANDBOX ", "PRODUCTION"} {
		if validPaymentProfile(profile) {
			t.Fatalf("invalid profile %q accepted", profile)
		}
		if _, err := NewPaymentStarter(context.Background(), nil, nil, profile); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("invalid profile %q reached pool validation: %v", profile, err)
		}
	}
}

func TestPaymentResultAndJobBoundary(t *testing.T) {
	args := paymentQueryArgs{OperationID: testID, Version: 1}
	if args.Kind() != "payment_query_v1" {
		t.Fatal("query job kind drifted")
	}
	body, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"operation_id":"00000000-0000-0000-0000-000000000001","version":1}` {
		t.Fatalf("query job args drifted: %s", body)
	}
}
