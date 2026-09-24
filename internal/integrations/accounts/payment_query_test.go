package accounts

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

type queryTransport func(*http.Request) (*http.Response, error)

func (f queryTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type paymentQueryRowFunc func(...any) error

func (f paymentQueryRowFunc) Scan(dest ...any) error { return f(dest...) }

type paymentQueryTx struct {
	pgx.Tx
	row pgx.Row
}

func (tx paymentQueryTx) QueryRow(context.Context, string, ...any) pgx.Row { return tx.row }

func TestPaymentQueryProfileFences(t *testing.T) {
	for _, tc := range []struct {
		profile, environment string
		valid                bool
	}{
		{"PROVIDER_MOCK", "SANDBOX", true}, {"SANDBOX", "SANDBOX", true},
		{"LIVE", "LIVE", true}, {"PROVIDER_MOCK", "LIVE", false},
		{"LIVE", "SANDBOX", false}, {"SANDBOX", "LIVE", false},
	} {
		if got := validQueryEnvironment(tc.profile, tc.environment); got != tc.valid {
			t.Errorf("%s/%s: got %t", tc.profile, tc.environment, got)
		}
	}
	if validQueryProfile("MOCK") || validQueryProfile("") {
		t.Fatal("unknown execution profile accepted")
	}
}

func TestLoadPaymentQueryRejectsUnscopedInputs(t *testing.T) {
	k := testKeyring(t)
	id := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	token := make([]byte, 32)
	transport := queryTransport(func(*http.Request) (*http.Response, error) { t.Fatal("unexpected wire call"); return nil, nil })
	for _, tc := range []struct {
		id, profile string
		generation  int64
		token       []byte
		mock        []http.RoundTripper
	}{
		{"invalid", "SANDBOX", 1, token, nil},
		{id, "SANDBOX", 0, token, nil},
		{id, "SANDBOX", 1, token[:31], nil},
		{id, "LIVE", 1, token, []http.RoundTripper{transport}},
		{id, "PROVIDER_MOCK", 1, token, []http.RoundTripper{nil}},
		{id, "PROVIDER_MOCK", 1, token, []http.RoundTripper{transport, transport}},
	} {
		if _, err := k.LoadPaymentQuery(context.Background(), nil, tc.id, tc.generation,
			tc.token, tc.profile, tc.mock...); !errors.Is(err, ErrPaymentQueryMaterial) {
			t.Fatalf("invalid material input returned %v", err)
		}
	}
}

func TestLoadPaymentQueryPreservesOnlyAuthenticatedAgeOnMissingKey(t *testing.T) {
	k := testKeyring(t)
	id := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	token := make([]byte, 32)
	row := func(environment string, scanErr error) pgx.Row {
		return paymentQueryRowFunc(func(dest ...any) error {
			if scanErr != nil {
				return scanErr
			}
			*dest[0].(*string) = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
			*dest[1].(*string) = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
			*dest[2].(*string) = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
			*dest[3].(*string) = "payuni"
			*dest[4].(*string) = environment
			*dest[5].(*string) = "Mer_ID-1"
			*dest[6].(*int64) = 1
			*dest[7].(*string) = "missing_historical_key"
			*dest[8].(*[]byte) = make([]byte, 12)
			*dest[9].(*[]byte) = make([]byte, 32)
			*dest[10].(*string) = "ORDER_1"
			*dest[11].(*string) = "TWD"
			*dest[12].(*int64) = 2500
			*dest[13].(*string) = "payuni_credit"
			*dest[14].(*string) = ""
			*dest[15].(*float64) = (25 * time.Hour).Seconds()
			return nil
		})
	}
	material, err := k.LoadPaymentQuery(context.Background(), paymentQueryTx{row: row("SANDBOX", nil)},
		id, 2, token, "SANDBOX")
	if !errors.Is(err, ErrPaymentQueryMaterial) || material.Age != 25*time.Hour ||
		material.Client != nil || material.Expected.MerTradeNo != "" {
		t.Fatalf("authenticated age-only failure: age=%s client=%v err=%v", material.Age, material.Client, err)
	}
	for _, tc := range []struct {
		name        string
		environment string
		scanErr     error
	}{
		{"SQL denied", "SANDBOX", errors.New("private SQL denied")},
		{"wrong profile", "LIVE", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			material, err := k.LoadPaymentQuery(context.Background(), paymentQueryTx{row: row(tc.environment, tc.scanErr)},
				id, 2, token, "SANDBOX")
			if !errors.Is(err, ErrPaymentQueryMaterial) || material.Age != 0 || material.Client != nil {
				t.Fatalf("untrusted age escaped: age=%s client=%v err=%v", material.Age, material.Client, err)
			}
		})
	}
}
