// A5 live-session flow author smoke (REAL_PG, MOCK picker): proves migrations/0118_live_session_flow.sql
// applies and that the A5-1 results read model and A5-2 one-click copy hold their contract. It is NOT the
// independent gate (test_worker writes those from the contract). Isolation: lcSetup's own principal with
// lcPurgeSessions cleanup; the picker's worker normalization has a DB-free unit test in internal/integrations/metareply.
package foundation_test

import (
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/claims"
	"livecommerce/internal/command"
	"livecommerce/internal/live"
	"livecommerce/internal/platform"
)

// TestLiveSessionCopy proves A5-2: one CopySession transaction creates a brand-new session whose CLOSED
// generation-0 window keeps the source match mode and whose ACTIVE offers keep their live_price_minor;
// the source version CAS refuses a stale expected_version.
func TestLiveSessionCopy(t *testing.T) {
	h := lcSetup(t)
	src := h.draft(t, h.f.storeA1)
	if w := h.open(t, src, claims.MatchExact); w.State != claims.WindowOpen {
		t.Fatalf("open window: %+v", w)
	}
	sku := lcSKUs(t, h.f, h.f.tenantA, h.f.storeA1, "USD", 1)[0]
	price := int64(199)
	o, err := h.createOffer(h.token, h.f.storeA1, t04Key("a5-offer"), src, claims.OfferInput{
		Keyword: "COPYKEYWORD", SKUID: sku, MaxQuantityPerClaim: 2, LivePriceMinor: &price,
	})
	if err != nil {
		t.Fatalf("CreateOffer with live price: %v", err)
	}
	if o.LivePriceMinor == nil || *o.LivePriceMinor != price {
		t.Fatalf("live price not stored: %+v", o)
	}
	var sourceVersion int64
	if err := h.do(h.token, h.f.storeA1, func(tx pgx.Tx, s platform.Scope) error {
		d, e := live.GetDraft(h.ctx, tx, s, h.token, src)
		if e != nil {
			return e
		}
		sourceVersion = d.Version
		return nil
	}); err != nil {
		t.Fatalf("GetDraft source: %v", err)
	}
	at := time.Now().Add(time.Hour).UTC().Truncate(time.Microsecond)
	var out live.CopyResult
	if err := h.do(h.token, h.f.storeA1, func(tx pgx.Tx, s platform.Scope) error {
		var e error
		out, e = live.CopySession(h.ctx, tx, s, h.token, t04Key("a5-copy"), src,
			live.CopyInput{Title: "copy " + t04Tag(), ScheduledAt: &at, ExpectedVersion: sourceVersion})
		return e
	}); err != nil {
		t.Fatalf("CopySession: %v", err)
	}
	if out.Session.ID == src || !command.ValidID(out.Session.ID) || out.Session.ID == "" {
		t.Fatalf("copy session id: %q", out.Session.ID)
	}
	if out.Window.State != claims.WindowClosed || out.Window.Generation != 0 || out.Window.MatchMode != claims.MatchExact {
		t.Fatalf("copy window: %+v", out.Window)
	}
	if out.SourceVersion != sourceVersion {
		t.Fatalf("source version: %d != %d", out.SourceVersion, sourceVersion)
	}
	if len(out.Created) != 1 || out.Conflicts == nil {
		t.Fatalf("created=%+v conflicts=%+v", out.Created, out.Conflicts)
	}
	created := out.Created[0]
	if created.Keyword != "COPYKEYWORD" || created.SKUID != sku || !created.Active ||
		created.LivePriceMinor == nil || *created.LivePriceMinor != price {
		t.Fatalf("copied offer: %+v", created)
	}
	// A stale source version must be refused (the FOR SHARE lock + CAS is the serialization point).
	if err := h.do(h.token, h.f.storeA1, func(tx pgx.Tx, s platform.Scope) error {
		_, e := live.CopySession(h.ctx, tx, s, h.token, t04Key("a5-copy-stale"), src,
			live.CopyInput{Title: "stale", ExpectedVersion: sourceVersion + 1})
		return e
	}); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("stale copy: %v", err)
	}
}

// TestLiveSessionResults proves A5-1: identity.read_live_session_results returns one item per requested
// id in request order with non-negative totals, and a merchant principal without orders:read is refused.
func TestLiveSessionResults(t *testing.T) {
	h := lcSetup(t)
	a := h.draft(t, h.f.storeA1)
	b := h.draft(t, h.f.storeA1)
	_, resultsToken := lcPrincipal(t, h.f, h.f.tenantA, []string{h.f.storeA1}, "store:read", "live:read", "orders:read")
	var out live.SessionResults
	if err := h.do(resultsToken, h.f.storeA1, func(tx pgx.Tx, s platform.Scope) error {
		var e error
		out, e = live.Results(h.ctx, tx, s, resultsToken, []string{b, a})
		return e
	}); err != nil {
		t.Fatalf("Results: %v", err)
	}
	if len(out.Items) != 2 || out.Items[0].SessionID != b || out.Items[1].SessionID != a {
		t.Fatalf("results order: %+v", out.Items)
	}
	for _, item := range out.Items {
		if item.Orders < 0 || item.PaidOrders < 0 || item.MultiSessionOrders < 0 {
			t.Fatalf("negative count: %+v", item)
		}
		for _, m := range item.Money {
			if m.OrderMinor < 0 || m.PaidMinor < 0 || m.SandboxPaidMinor < 0 {
				t.Fatalf("negative amount: %+v", m)
			}
		}
	}
	// The merchant principal has live:read but not orders:read -> 403, totals never leak.
	if err := h.do(h.token, h.f.storeA1, func(tx pgx.Tx, s platform.Scope) error {
		_, e := live.Results(h.ctx, tx, s, h.token, []string{a})
		return e
	}); !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("results without orders:read: %v", err)
	}
	// Duplicate ids are 422 before any database call (the read model is strict about 1..50 distinct).
	var dup live.SessionResults
	if err := h.do(resultsToken, h.f.storeA1, func(tx pgx.Tx, s platform.Scope) error {
		var e error
		dup, e = live.Results(h.ctx, tx, s, resultsToken, []string{a, a})
		return e
	}); !errors.Is(err, command.ErrInvalid) {
		t.Fatalf("duplicate ids: %v %+v", err, dup)
	}
}
