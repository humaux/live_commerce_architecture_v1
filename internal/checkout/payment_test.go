package checkout

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"livecommerce/internal/command"
)

func TestPaymentInputAndProfileBoundary(t *testing.T) {
	in := PaymentInput{OrderID: testID, MethodCode: "payuni_credit", MethodVersion: 1}
	if !validPaymentInput(in) {
		t.Fatal("valid input rejected")
	}
	body, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"order_id":"00000000-0000-0000-0000-000000000001","method_code":"payuni_credit","method_version":1}` {
		t.Fatalf("payment input shape drifted: %s", body)
	}
	for _, changed := range []PaymentInput{
		{OrderID: "bad", MethodCode: "payuni_credit", MethodVersion: 1},
		{OrderID: testID, MethodCode: "payuni_installment", MethodVersion: 1},
		{OrderID: testID, MethodCode: "payuni_credit", MethodVersion: 0},
	} {
		if validPaymentInput(changed) {
			t.Fatalf("invalid payment input accepted: %+v", changed)
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
	out := PaymentResult{OrderID: testID, AttemptID: testID, OperationID: testID, JobID: 1,
		Generation: 2, MerchantTradeNo: "M123", Currency: "TWD", AmountMinor: 100, State: "PAYMENT_PENDING"}
	if !validPaymentResult(out, testID) {
		t.Fatal("valid payment result rejected")
	}
	for _, mutate := range []func(*PaymentResult){
		func(v *PaymentResult) { v.OperationID = "bad" },
		func(v *PaymentResult) { v.Generation = 1 },
		func(v *PaymentResult) { v.AmountMinor = 101 },
		func(v *PaymentResult) { v.AmountMinor = 20000000 },
		func(v *PaymentResult) { v.State = "PAID" },
	} {
		changed := out
		mutate(&changed)
		if validPaymentResult(changed, testID) {
			t.Fatalf("invalid payment result accepted: %+v", changed)
		}
	}
	if _, err := (&PaymentStarter{}).StartPayment(context.Background(), "", testID, "validkey1", PaymentInput{OrderID: testID, MethodCode: "payuni_credit", MethodVersion: 1}); !errors.Is(err, command.ErrInvalid) {
		t.Fatalf("uninitialized starter accepted: %v", err)
	}
}
