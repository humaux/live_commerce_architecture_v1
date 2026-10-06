// Purpose: DB-free unit tests of the pure parts of the order-for-a-buyer service (LC-B6): the `for` object normalisation, the server-side choice of
// cart-line origins (a line that does not qualify stays at the catalog price), the grant audit detail size cap and the DM refusal mapping.
// Depends on: order_for_buyer.go, internal/claims (ForBuyerLine/GrantedLine), internal/inbox (SendError), pgconn.
// Used by: go test ./internal/merchanttools. Database behaviour is gate LCN12 in tests/foundation.

package merchanttools

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/claims"
	"livecommerce/internal/inbox"
)

func TestValidateTargetSortsAndRefusesBadIds(t *testing.T) {
	a, b := "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"
	got, err := validateTarget(ForBuyerTarget{BundleIDs: []string{b, a}})
	if err != nil || got.BundleIDs[0] != a || got.BundleIDs[1] != b {
		t.Fatalf("sorted: %+v %v", got, err)
	}
	if got, err = validateTarget(ForBuyerTarget{}); err != nil || got.BundleIDs == nil || len(got.BundleIDs) != 0 {
		t.Fatalf("no bundles is a plain order with an empty (non-nil) list: %+v %v", got, err)
	}
	bad := "not-a-uuid"
	six := []string{a, b, "33333333-3333-4333-8333-333333333333", "44444444-4444-4444-8444-444444444444", "55555555-5555-4555-8555-555555555555", "66666666-6666-4666-8666-666666666666"}
	for name, in := range map[string]ForBuyerTarget{
		"duplicate": {BundleIDs: []string{a, a}}, "bad id": {BundleIDs: []string{bad}}, "six": {BundleIDs: six}, "bad conversation": {ConversationID: &bad},
	} {
		if _, err := validateTarget(in); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func price(v int64) *int64 { return &v }

func TestPickOriginsFailsClosedPerLine(t *testing.T) {
	lines := []claims.ForBuyerLine{
		{BundleID: "b1", OfferID: "o1", SKUID: "s1", Quantity: 2, LivePriceMinor: price(100), LiveRemaining: 2},
		{BundleID: "b1", OfferID: "o2", SKUID: "s2", Quantity: 3, LivePriceMinor: nil, LiveRemaining: 3},       // inactive / no live price
		{BundleID: "b1", OfferID: "o3", SKUID: "s3", Quantity: 3, LivePriceMinor: price(90), LiveRemaining: 1}, // 2 already held
	}
	origins, skipped := pickOrigins([]ManualItem{{SKUID: "s1", Quantity: 2}, {SKUID: "s2", Quantity: 1}, {SKUID: "s3", Quantity: 2}, {SKUID: "s4", Quantity: 1}}, lines)
	if len(origins) != 1 || origins["s1"].BundleID != "b1" || origins["s1"].OfferID != "o1" || origins["s1"].Quantity != 2 {
		t.Fatalf("origins: %+v", origins)
	}
	if skipped != 2 { // s2 (no live price) and s3 (remaining below the quantity); s4 has no claim line at all
		t.Fatalf("skipped %d, want 2", skipped)
	}
	// quantity above the claimed quantity: no origin (the carry rule would drop it anyway)
	if origins, _ = pickOrigins([]ManualItem{{SKUID: "s1", Quantity: 3}}, lines); len(origins) != 0 {
		t.Fatalf("above the claim: %+v", origins)
	}
}

func TestGrantAuditStaysUnderTheDetailCap(t *testing.T) {
	var granted []claims.GrantedLine
	for i := 0; i < 50; i++ {
		granted = append(granted, claims.GrantedLine{BundleID: "11111111-1111-4111-8111-111111111111", OfferID: "22222222-2222-4222-8222-222222222222", SKUID: "s", Quantity: 999, LivePriceMinor: 1000000000000})
	}
	detail, ok := grantAudit("11111111-1111-4111-8111-111111111111", "33333333-3333-4333-8333-333333333333", "44444444-4444-4444-8444-444444444444", granted)
	raw, _ := json.Marshal(detail)
	if !ok || len(raw) > maxGrantAuditBytes || detail["line_count"] != 50 {
		t.Fatalf("detail %d bytes ok=%v: %s", len(raw), ok, raw)
	}
	if _, ok := grantAudit("another-bundle", "x", "y", granted); ok {
		t.Fatal("an audit for a bundle that priced no line")
	}
}

func TestPayLinkRefusalsBecomeNotSentNeverFailTheOrder(t *testing.T) {
	f := &ForBuyer{}
	for name, c := range map[string]struct {
		err    error
		reason string
	}{
		"closed window (service)":  {&inbox.SendError{Status: 409, Code: "window_closed"}, "window_closed"},
		"closed window (planner)":  {&pgconn.PgError{Code: "PT409", Message: "window_closed"}, "window_closed"},
		"severed connection":       {&pgconn.PgError{Code: "PT409", Message: "capability"}, "capability"},
		"thread gone":              {&pgconn.PgError{Code: "PT409", Message: "conversation_gone"}, "no_conversation"},
		"rate":                     {&pgconn.PgError{Code: "PT429", Message: "rate_limited"}, "rate_limited"},
		"messaging not configured": {inbox.ErrSendUnavailable, "send_unavailable"},
	} {
		got, err := f.payLinkOutcome(c.err)
		if err != nil || got.State != "not_sent" || got.Reason != c.reason {
			t.Errorf("%s: %+v %v", name, got, err)
		}
	}
	// An unknown failure is returned as the retryable coded refusal (the HTTP layer writes only the code); the replay resumes the DM.
	var coded *Error
	if _, err := f.payLinkOutcome(errors.New("connection reset")); !errors.As(err, &coded) || coded.Status != 503 || coded.Code != "retry_later" {
		t.Errorf("an unknown failure must be the retryable coded error: %v", err)
	}
}
