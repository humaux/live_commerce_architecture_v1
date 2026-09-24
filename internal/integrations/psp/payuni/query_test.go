package payuni

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func signedQueryRow(t *testing.T, row url.Values, expected ExpectedTrade) (Observation, error) {
	t.Helper()
	c := testClient(t)
	row.Del("Status")
	row.Set("DataSource", "A")
	body := queryEnvelope(t, c, row)
	calls := 0
	c.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != "https://sandbox-api.payuni.com.tw/api/trade/query" {
			t.Fatal("unexpected query destination")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)),
			Header: make(http.Header)}, nil
	})
	out, err := c.Query(context.Background(), expected, 1760000000)
	if calls != 1 {
		t.Fatalf("wire calls = %d, want 1", calls)
	}
	return out, err
}

func TestSignedCreditQueryCaptureAndRefundProjection(t *testing.T) {
	row := baseObservation()
	missing, err := signedQueryRow(t, row, testExpected("payuni_credit"))
	if err != nil || missing.CloseAmountTWD != nil || missing.CardRefundAmountTWD != nil ||
		missing.CardRemainAmountTWD != nil {
		t.Fatalf("missing optional amounts lost: %+v %v", missing, err)
	}
	row.Set("CloseStatus", "2")
	row.Set("CloseAmt", "0")
	row.Set("RefundType", "2")
	row.Set("RefundStatus", "8")
	row.Set("RefundAmount", "0")
	row.Set("RefundDay", "2026-02-28 23:59:59")
	row.Set("RemainAmount", "0")
	row.Set("CardNo", "4111111111111111")
	got, err := signedQueryRow(t, row, testExpected("payuni_credit"))
	if err != nil || got.CloseAmountTWD == nil || *got.CloseAmountTWD != 0 ||
		got.CardRefundAmountTWD == nil || *got.CardRefundAmountTWD != 0 ||
		got.CardRemainAmountTWD == nil || *got.CardRemainAmountTWD != 0 ||
		got.CardRefundType != "2" || got.CardRefundStatus != "8" ||
		got.CardRefundDay != "2026-02-28 23:59:59" {
		t.Fatalf("signed zero projection invalid: %+v %v", got, err)
	}
	encoded, err := json.Marshal(got)
	if err != nil || !strings.Contains(string(encoded), `"CloseAmountTWD":0`) ||
		!strings.Contains(string(encoded), `"CardRefundAmountTWD":0`) ||
		strings.Contains(string(encoded), "4111111111111111") || strings.Contains(string(encoded), "CardNo") {
		t.Fatalf("projection leaked or lost zero: %s %v", encoded, err)
	}
	row.Set("CloseAmt", "100")
	row.Set("RefundAmount", "30")
	row.Set("RemainAmount", "70")
	got, err = signedQueryRow(t, row, testExpected("payuni_credit"))
	if err != nil || *got.CloseAmountTWD != 100 || *got.CardRefundAmountTWD != 30 ||
		*got.CardRemainAmountTWD != 70 {
		t.Fatalf("signed amount projection invalid: %+v %v", got, err)
	}
}

func TestSignedInstallmentAndNoncardIsolation(t *testing.T) {
	installment := baseObservation()
	installment.Set("AuthType", "2")
	installment.Set("CardInst", "3")
	installment.Set("CloseAmt", "100")
	installment.Set("RefundType", "3")
	installment.Set("RefundStatus", "2")
	installment.Set("RefundAmount", "10")
	installment.Set("RemainAmount", "90")
	got, err := signedQueryRow(t, installment, ExpectedTrade{MerTradeNo: "ORDER_1",
		AmountTWD: 100, Currency: "TWD", Method: "payuni_installment", Installments: []int{3}})
	if err != nil || got.CardInst != 3 || got.CloseAmountTWD == nil || *got.CloseAmountTWD != 100 ||
		got.CardRefundType != "3" || got.CardRefundStatus != "2" {
		t.Fatalf("installment projection invalid: %+v %v", got, err)
	}
	atm := baseObservation()
	atm.Set("PaymentType", "2")
	atm.Del("AuthType")
	atm.Set("RefundStatus", "9") // ATM status has a different vocabulary.
	atm.Set("RefundType", "not-a-card-refund")
	atm.Set("RefundAmount", "oops")
	atm.Set("CardNo", "4111111111111111")
	got, err = signedQueryRow(t, atm, testExpected("payuni_atm"))
	if err != nil || got.CardRefundStatus != "" || got.CardRefundAmountTWD != nil ||
		got.CloseAmountTWD != nil {
		t.Fatalf("noncard query inherited card semantics: %+v %v", got, err)
	}
	encoded, err := json.Marshal(got)
	if err != nil || strings.Contains(string(encoded), "4111111111111111") || strings.Contains(string(encoded), "CardRefund") {
		t.Fatalf("noncard projection leaked card data: %s %v", encoded, err)
	}
}

func TestSignedCreditQueryRejectsMalformedOptionalFacts(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"CloseAmt", "-1"}, {"CloseAmt", "+1"}, {"CloseAmt", "00"}, {"CloseAmt", "01"},
		{"CloseAmt", "1.0"}, {"CloseAmt", "200000"},
		{"RefundAmount", "999999999999999999999"}, {"RemainAmount", " 1"},
		{"RefundType", "1"}, {"RefundStatus", "9"},
		{"RefundDay", "2025-02-29 12:00:00"}, {"RefundDay", "0000-01-01 12:00:00"},
		{"RefundDay", "2026-01-01T12:00:00"},
	} {
		t.Run(tc.key+"="+tc.value, func(t *testing.T) {
			row := baseObservation()
			row.Set(tc.key, tc.value)
			if _, err := signedQueryRow(t, row, testExpected("payuni_credit")); !errors.Is(err, ErrUncertain) {
				t.Fatalf("malformed signed field accepted: %v", err)
			}
		})
	}
}
