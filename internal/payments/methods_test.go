package payments

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

const testMarketID = "11111111-1111-4111-8111-111111111111"
const testConnectionID = "22222222-2222-4222-8222-222222222222"

func validInput() MethodInput {
	return MethodInput{MarketID: testMarketID, Country: "TW", Code: "payuni_credit",
		Environment: "SANDBOX", NameHans: "信用卡", NameHant: "信用卡", NameEN: "Credit card",
		Visible: true, MinAmountMinor: 1, MaxAmountMinor: command.MaxMoney}
}

func TestMethodInputValidation(t *testing.T) {
	base := validInput()
	for _, code := range []string{"payuni_credit", "payuni_installment", "payuni_atm", "payuni_cvs", "payuni_linepay"} {
		in := base
		in.Code = code
		if !validMethodInput(in) {
			t.Fatalf("valid code %q rejected", code)
		}
	}
	linked := base
	linked.ConnectionID, linked.BindingVersion = testConnectionID, 1
	if !validMethodInput(linked) {
		t.Fatal("valid connection pair rejected")
	}
	for name, mutate := range map[string]func(*MethodInput){
		"unknown code":      func(in *MethodInput) { in.Code = "cash_on_delivery" },
		"country":           func(in *MethodInput) { in.Country = "US" },
		"market id":         func(in *MethodInput) { in.MarketID = "invalid" },
		"environment":       func(in *MethodInput) { in.Environment = "sandbox" },
		"empty label":       func(in *MethodInput) { in.NameEN = " \t" },
		"control label":     func(in *MethodInput) { in.NameEN = "bad\x00" },
		"overlong label":    func(in *MethodInput) { in.NameEN = string(makeRunes(121)) },
		"sort":              func(in *MethodInput) { in.SortOrder = 1001 },
		"minimum":           func(in *MethodInput) { in.MinAmountMinor = 0 },
		"maximum":           func(in *MethodInput) { in.MaxAmountMinor = command.MaxMoney + 1 },
		"inverted range":    func(in *MethodInput) { in.MinAmountMinor = 2; in.MaxAmountMinor = 1 },
		"negative version":  func(in *MethodInput) { in.ExpectedVersion = -1 },
		"overflow version":  func(in *MethodInput) { in.ExpectedVersion = math.MaxInt64 },
		"orphan connection": func(in *MethodInput) { in.ConnectionID = testConnectionID },
		"orphan binding":    func(in *MethodInput) { in.BindingVersion = 1 },
	} {
		t.Run(name, func(t *testing.T) {
			in := base
			mutate(&in)
			if validMethodInput(in) {
				t.Fatalf("invalid input accepted: %+v", in)
			}
		})
	}
}

func TestEnabledDraftRejected(t *testing.T) {
	in := validInput()
	in.Enabled = true
	_, err := SetMethod(context.Background(), nil, platform.Scope{}, "", "", in)
	if !errors.Is(err, command.ErrConflict) {
		t.Fatalf("enabled draft error = %v, want conflict", err)
	}
}

func makeRunes(n int) []rune {
	out := make([]rune, n)
	for i := range out {
		out[i] = '字'
	}
	return out
}

func TestCheckInputValidation(t *testing.T) {
	in := CheckInput{MarketID: testMarketID, Country: "TW", Code: "payuni_credit",
		ExpectedVersion: 1, Environment: "SANDBOX", Currency: "USD", AmountMinor: 1}
	if !validCheckInput(in) {
		t.Fatal("valid mismatch diagnostic input rejected")
	}
	in.Currency = "usd"
	if validCheckInput(in) {
		t.Fatal("lowercase currency accepted")
	}
	in.Currency = "TWD"
	in.ExpectedVersion = 0
	if validCheckInput(in) {
		t.Fatal("zero expected version accepted")
	}
	in.ExpectedVersion = 1
	in.AmountMinor = command.MaxMoney + 1
	if validCheckInput(in) {
		t.Fatal("out-of-domain amount accepted")
	}
}

func TestDiagnosticReasonsOrderAndFailClosed(t *testing.T) {
	method := Method{Version: 2, Environment: "LIVE", Currency: "TWD",
		ConnectionID: testConnectionID, BindingVersion: 3,
		MinAmountMinor: 100, MaxAmountMinor: 200}
	check := CheckInput{ExpectedVersion: 1, Environment: "SANDBOX", Currency: "USD", AmountMinor: 50}
	got := diagnose(method, false, check, 4, 5, false)
	want := []string{"METHOD_VERSION_CHANGED", "METHOD_DISABLED", "METHOD_HIDDEN",
		"MARKET_INACTIVE", "ENVIRONMENT_MISMATCH", "CURRENCY_MISMATCH", "AMOUNT_OUT_OF_RANGE",
		"BINDING_VERSION_CHANGED", "BINDING_DISABLED", "CREDENTIALS_UNVERIFIED", "ADAPTER_UNAVAILABLE"}
	if got.Available || got.MethodVersion != 2 || got.BindingVersion != 4 ||
		got.CredentialVersion != 5 || !reflect.DeepEqual(got.Reasons, want) {
		t.Fatalf("diagnosis = %+v, want reasons %v", got, want)
	}
	method.Enabled, method.Visible = true, true
	method.ConnectionID, method.BindingVersion = "", 0
	check.ExpectedVersion, check.Environment, check.Currency, check.AmountMinor = 2, "LIVE", "TWD", 150
	got = diagnose(method, true, check, 0, 0, false)
	if got.Available || !reflect.DeepEqual(got.Reasons, []string{"CONNECTION_MISSING", "ADAPTER_UNAVAILABLE"}) {
		t.Fatalf("unlinked diagnosis = %+v", got)
	}
	method.ConnectionID, method.BindingVersion = testConnectionID, 3
	got = diagnose(method, true, check, 3, 8, true)
	if got.Available || !reflect.DeepEqual(got.Reasons, []string{"CREDENTIALS_UNVERIFIED", "ADAPTER_UNAVAILABLE"}) ||
		got.CredentialVersion != 8 {
		t.Fatalf("linked diagnosis = %+v", got)
	}
}
