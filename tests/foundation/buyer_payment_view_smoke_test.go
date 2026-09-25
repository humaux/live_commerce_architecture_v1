package foundation_test

import (
	"context"
	"testing"

	"livecommerce/internal/checkout"
)

// Root integration smoke exercises the SQL body, not just CREATE FUNCTION. The
// independent payment-view/HTTP matrix remains a separate test-worker gate.
func TestBuyerPaymentViewSQLSmoke(t *testing.T) {
	h := hpSetup(t)
	api := h.api.(*checkout.HostedPaymentStarter)
	before := h.counts(t)
	view, err := api.PaymentView(context.Background(), h.cap.Token, h.f.storeA1, h.hold.OrderID)
	if err != nil || view.PaymentState != "NOT_STARTED" || !view.TestMode ||
		view.HandoffState != "NONE" || len(view.Methods) != 1 || view.Methods[0].Code != "payuni_credit" {
		t.Fatalf("owned DRAFT projection failed: state=%s methods=%d err=%v", view.PaymentState, len(view.Methods), err)
	}
	if before != h.counts(t) {
		t.Fatal("read-only payment projection changed payment or inventory facts")
	}
}
