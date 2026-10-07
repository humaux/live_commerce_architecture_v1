package foundation_test

// W4-U1 (docs/delivery/units/w4-u1-payment-activation-ui.md): the real-ledger fixture shared by the card-payments / settlements
// browser gate (browser_card_payments_test.go) and the wire-shape check below. Tier: MOCK balance transactions (never Stripe) + REAL_PG:
// merchant A of pslNew pays through the real capture path, then three weekly statements are produced ONLY through the operator paths
// (stripeadmin sync -> close -> payout record). Nothing is inserted into the settlement tables directly.
//
// Why a wire test: the admin parsers (apps/admin/lib/card-payments-settlements-model.ts) are strict. TestW4U1WireShapes asserts the real
// HTTP bodies carry every sign case the parsers must accept (a fee is a cost <= 0, a dispute reversal makes dispute_minor negative, a
// carried-in debt, an unpaid net <= 0) and, with LC_W4U1_WIRE_OUT=<dir>, writes the bodies verbatim so tests/admin/fixtures/card-payments-wire
// can be regenerated and re-parsed by tests/admin/card-payments-wire.test.ts (Go PG -> JSON -> TS parser, E3).

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/payments/platformstripe"
	"livecommerce/internal/payments/settlement"
	"livecommerce/internal/platform"
)

// w4u1Statements names the three statements of merchant A: Paid (net > 0, payout recorded), Carried (net <= 0: refunds and a dispute
// outran the week, never paid) and Pending (net > 0 after the carry, dispute reversal makes dispute_minor negative, not yet paid).
type w4u1Statements struct {
	Paid, Carried, Pending string
	PaidNet                int64
	PayoutRef              string
}

const w4u1PayoutRef = "BANK-REF-W4U1-0831"

// w4u1SeedStatements closes weeks 08-31, 09-07 and 09-14 (Asia/Taipei) of store A and records the payout of the first.
func w4u1SeedStatements(t *testing.T, e *pslEnv) w4u1Statements {
	t.Helper()
	w0, w1, w2, w3 := pslDay(2026, 8, 31), pslDay(2026, 9, 7), pslDay(2026, 9, 14), pslDay(2026, 9, 21)
	d := func(w time.Time, n int) time.Time { return w.Add(time.Duration(n)*24*time.Hour + 3*time.Hour) }
	settleOf := func(minor int64) int64 { return minor * 2564 / 10000 }
	capOf := func(o rfxOrder) int64 { return o.captured }
	// coverage: close needs the union of sync windows to span [period_start - 7d, period_end)
	e.sync(t, w0.Add(-7*24*time.Hour), w0)
	e.sync(t, w0, w1, pslCharge("txn_W4U1ChargeA2", e.oa2.pi, capOf(e.oa2), settleOf(capOf(e.oa2)), 33, d(w0, 1)))
	e.sync(t, w1, w2,
		pslCharge("txn_W4U1ChargeA1", e.oa1.pi, capOf(e.oa1), settleOf(capOf(e.oa1)), 31, d(w1, 1)),
		pslRefund("txn_W4U1RefundA1", e.refundA1, 800, settleOf(800), 0, d(w1, 2)),
		pslDispute("txn_W4U1DisputeA2", "dp_W4U1A2", e.oa2.pi, capOf(e.oa2), settleOf(capOf(e.oa2)), 1500, false, d(w1, 3)))
	e.sync(t, w2, w3,
		pslCharge("txn_W4U1ChargeA3", e.oa3.pi, capOf(e.oa3), settleOf(capOf(e.oa3)), 35, d(w2, 1)),
		pslDispute("txn_W4U1ReversalA2", "dp_W4U1A2", e.oa2.pi, capOf(e.oa2), settleOf(capOf(e.oa2)), 1500, true, d(w2, 2)))
	closeOne := func(week time.Time) (string, int64) {
		got, err := e.closeWeek(week, e.storeA)
		if err != nil || len(got) != 1 {
			t.Fatalf("close %s: %+v %v", week.Format("2006-01-02"), got, err)
		}
		return got[0].StatementID, got[0].NetPayableMinor
	}
	var out w4u1Statements
	out.Paid, out.PaidNet = closeOne(w0)
	out.Carried, _ = closeOne(w1)
	out.Pending, _ = closeOne(w2)
	if out.PaidNet <= 0 {
		t.Fatalf("fixture: the first statement must be payable, net=%d", out.PaidNet)
	}
	paidAt := time.Now().Add(-time.Hour).Truncate(time.Second)
	if _, err := e.reg.SettlementPayout(t.Context(), e.op, out.Paid, w4u1PayoutRef, out.PaidNet, paidAt, "op@test", pslTicket); err != nil {
		t.Fatalf("payout record: %v", err)
	}
	out.PayoutRef = w4u1PayoutRef
	return out
}

// w4u1Get calls the real merchant HTTP handler (card routes + settlement routes) with the store creator's bearer.
func w4u1Get(t *testing.T, h http.Handler, token, path string) (int, []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w.Code, w.Body.Bytes()
}

func TestW4U1WireShapes(t *testing.T) {
	e := pslNew(t)
	st := w4u1SeedStatements(t, e)
	token, store := e.a.f.tokens["a"], e.storeA
	base := "/v1/admin/stores/" + store

	status, raw := w4u1Get(t, e.card, token, base+"/settlements")
	var list settlement.List
	if status != http.StatusOK || json.Unmarshal(raw, &list) != nil || len(list.Statements) != 3 {
		t.Fatalf("settlement list: %d %s", status, raw)
	}
	byID := map[string]settlement.Statement{}
	for _, s := range list.Statements {
		byID[s.StatementID] = s
		if len(s.Lines) != 0 {
			t.Fatalf("the list must not carry lines: %+v", s)
		}
	}
	details := map[string][]byte{}
	for id := range byID {
		status, raw := w4u1Get(t, e.card, token, base+"/settlements/"+id)
		var detail settlement.Detail
		if status != http.StatusOK || json.Unmarshal(raw, &detail) != nil || len(detail.Statement.Lines) != detail.Statement.LineCount {
			t.Fatalf("detail %s: %d %s", id, status, raw)
		}
		details[id] = raw
	}
	paid, carried, pending := byID[st.Paid], byID[st.Carried], byID[st.Pending]
	// the sign cases the admin parsers must accept
	if !paid.Paid || paid.PayoutRef == nil || *paid.PayoutRef != st.PayoutRef || paid.PayoutMinor == nil || *paid.PayoutMinor != paid.NetPayableMinor || paid.StripeFeeMinor >= 0 {
		t.Errorf("paid statement: %+v", paid)
	}
	if carried.Paid || carried.NetPayableMinor > 0 || carried.RefundedMinor <= 0 || carried.DisputeMinor <= 0 || carried.StripeFeeMinor >= 0 {
		t.Errorf("carried statement (net <= 0, refund and dispute positive, fee a cost): %+v", carried)
	}
	if pending.Paid || pending.NetPayableMinor <= 0 || pending.CarriedInMinor >= 0 || pending.DisputeMinor >= 0 {
		t.Errorf("pending statement (positive net after a carried debt, dispute reversal negative): %+v", pending)
	}
	if dir := os.Getenv("LC_W4U1_WIRE_OUT"); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		write := func(name string, body []byte) {
			if err := os.WriteFile(filepath.Join(dir, name), append(body, '\n'), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		write("settlements-list.json", raw)
		write("settlement-paid.json", details[st.Paid])
		write("settlement-carried.json", details[st.Carried])
		write("settlement-pending.json", details[st.Pending])
		for name, path := range map[string]string{"card-enabled.json": base + "/payments/card"} {
			status, body := w4u1Get(t, e.card, token, path)
			if status != http.StatusOK {
				t.Fatalf("%s: %d %s", path, status, body)
			}
			write(name, body)
		}
		// a store with no statement answers an empty list; the card read of a never-enabled, allowlisted store is the NONE shape
		status, body := w4u1Get(t, e.card, e.b.f.tokens["a"], "/v1/admin/stores/"+e.storeB+"/settlements")
		if status != http.StatusOK {
			t.Fatalf("empty list: %d %s", status, body)
		}
		write("settlements-empty.json", body)
	}
}

// w4uControl performs the runner-only state changes through the SAME operator definers the CLI uses (never the UI under test). Its handler runs
// on the http server goroutine, so it reports errors as 500 instead of failing the test from there.
type w4uControl struct {
	e  *pslEnv
	mu sync.Mutex
}

func (c *w4uControl) platformOpen(ctx context.Context, open bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, err := c.e.reg.PlatformOpen(ctx, c.e.op, "SANDBOX", open, -1, c.e.version)
	if err == nil {
		c.e.version = v
	}
	return err
}

func (c *w4uControl) block(ctx context.Context, blocked bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.e.reg.PlatformBlock(ctx, c.e.op, c.e.a.f.tenantA, c.e.a.f.storeA1, "SANDBOX", blocked, "op@test", "tk-"+t04Tag())
	return err
}

// allow is the operator's platform-allow / platform-disallow (the real definer). Withdrawing the allowlist of an enrolled, unblocked store also
// BLOCKS it (migrations/0137 P2-4); re-allowing does not unblock.
func (c *w4uControl) allow(ctx context.Context, allowed bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.e.reg.PlatformAllow(ctx, c.e.op, c.e.a.f.tenantA, c.e.a.f.storeA1, "SANDBOX", allowed, "op@test", "tk-"+t04Tag())
	return err
}

// bump is another merchant session changing the enrollment (a real enable with a new suffix): the browser's page now holds a stale version.
func (c *w4uControl) bump(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	token, store := c.e.a.f.tokens["a"], c.e.a.f.storeA1
	var current platformstripe.Summary
	if err := platform.WithScope(ctx, c.e.f.runtime, token, store, "integration:read", func(tx pgx.Tx, s platform.Scope) error {
		var err error
		current, err = platformstripe.Read(ctx, tx, s, token, "PROVIDER_MOCK")
		return err
	}); err != nil {
		return err
	}
	suffix := "BUMPED"
	in := platformstripe.Input{Enabled: true, TermsVersion: pfTerms, DescriptorSuffix: &suffix, ExpectedVersion: current.Version}
	return platform.WithScope(ctx, c.e.f.runtime, token, store, "billing:manage", func(tx pgx.Tx, s platform.Scope) error {
		_, err := platformstripe.Set(ctx, tx, s, token, "PROVIDER_MOCK", in)
		return err
	})
}

// w4uCall is one merchant request against the real handler (card + settlement routes), as the admin BFF sends it: bearer only, JSON body for the PUT,
// never an Idempotency-Key (the PUT is CAS-guarded).
func w4uCall(t *testing.T, h http.Handler, method, token, path, body string) (int, string) {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Authorization", "Bearer "+token)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w.Code, w.Body.String()
}

// TestW4U1StateSequence replays, at the HTTP level and in the same order, every server interaction of tests/admin/card-payments.spec.ts (the browser
// gate cannot run on the dev Mac): the same GETs, the exact PUT bodies the page builds (buildCardInput), the competing change, platform close/open and
// block/unblock through the operator definers. It pins what the spec asserts about the server (states, 409/403 codes, descriptor previews, audit counts).
func TestW4U1StateSequence(t *testing.T) {
	e := pslNew(t)
	ctl := &w4uControl{e: e}
	ctx := context.Background()
	token, store, principal := e.a.f.tokens["a"], e.storeA, e.a.f.principalA
	path := "/v1/admin/stores/" + store + "/payments/card"
	read := func() platformstripe.Summary {
		t.Helper()
		status, body := w4uCall(t, e.card, http.MethodGet, token, path, "")
		var out platformstripe.Summary
		if status != http.StatusOK || json.Unmarshal([]byte(body), &out) != nil {
			t.Fatalf("GET payments/card: %d %s", status, body)
		}
		return out
	}
	put := func(enabled bool, terms string, suffix *string, version int64) (int, string) {
		t.Helper()
		sfx := "null"
		if suffix != nil {
			sfx = fmt.Sprintf("%q", *suffix)
		}
		return w4uCall(t, e.card, http.MethodPut, token, path, fmt.Sprintf(`{"enabled":%v,"terms_version":%q,"descriptor_suffix":%s,"expected_version":%d}`, enabled, terms, sfx, version))
	}
	want := func(what string, status int, body string, wantStatus int, contains string) {
		t.Helper()
		if status != wantStatus || !strings.Contains(body, contains) {
			t.Fatalf("%s: %d %s (want %d containing %q)", what, status, body, wantStatus, contains)
		}
	}
	str := func(p *string) string {
		if p == nil {
			return "<nil>"
		}
		return *p
	}
	shop := "SHOP"

	// the buyer half's scenarios, at the service level (the browser gate cannot run on the dev Mac): a payable order of the enrolled store offers
	// Stripe and carries the collector; the platform store's own (primary connection) order carries none
	// Single-method UI (as browser_stripe_test.go, Q3): the fixture store also offers the PAYUNi mock method, and the storefront's Stripe plan needs
	// Stripe to be the only offered method. Owner-pool fixture, disclosed; the platform state under test is untouched.
	mustExec(t, e.f.owner, `DELETE FROM payments.method_heads WHERE tenant_id=$1 AND store_id=$2 AND code='payuni_credit'`, e.a.f.tenantA, e.a.f.storeA1)
	e.ensureStock(t, e.oa1)
	unpaid := e.oa1.s
	unpaid.p = sstMoreHold(t, e.oa1.s.p)
	v := e.view(t, unpaid)
	if v.PaymentState != "NOT_STARTED" || v.CommercialState != "DRAFT" || len(v.Methods) != 1 || v.Methods[0].Code != "stripe_checkout" ||
		v.Collector == nil || v.Collector.DisplayName != "Platform Test" || v.Collector.DescriptorPreview != "LCPLATFORM" {
		t.Fatalf("payable order of an enrolled store: %+v collector=%+v", v, v.Collector)
	}
	if paid := e.view(t, e.oa1.s); paid.Collector == nil || paid.Collector.DescriptorPreview != "LCPLATFORM" {
		t.Fatalf("captured order of an enrolled store: collector=%+v", paid.Collector)
	}
	if primary := e.view(t, e.plat); primary.Collector != nil {
		t.Fatalf("a primary connection must carry no collector: %+v", primary.Collector)
	}

	// CPU1: the seeded store is ENABLED on an OPEN platform; the limits are the server's (TWD min 2500)
	s := read()
	if s.PlatformState != "OPEN" || s.StoreState != "ENABLED" || !s.Allowed || str(s.AcceptedTermsVersion) != pfTerms || str(s.Currency) != "TWD" || s.MinMinor == nil || *s.MinMinor != 2500 {
		t.Fatalf("CPU1 summary: %+v", s)
	}
	// CPU2: disable quotes the accepted terms and carries the version read
	status, body := put(false, str(s.AcceptedTermsVersion), nil, s.Version)
	want("CPU2 disable", status, body, 200, `"state":"DISABLED"`)
	// CPU3: enable with the suffix; the preview is base + "* " + suffix
	s = read()
	if s.StoreState != "DISABLED" || str(s.DescriptorPreview) != "LCPLATFORM" {
		t.Fatalf("CPU3 before: %+v", s)
	}
	status, body = put(true, str(s.TermsVersion), &shop, s.Version)
	want("CPU3 enable", status, body, 200, `"descriptor_preview":"LCPLATFORM* SHOP"`)
	// CPU4: a competing session changes the enrollment; the page's version is stale, so its disable is a 409 that changes nothing
	stale := read()
	if err := ctl.bump(ctx); err != nil {
		t.Fatalf("bump: %v", err)
	}
	status, body = put(false, str(stale.AcceptedTermsVersion), nil, stale.Version)
	want("CPU4 stale disable", status, body, 409, "version_changed")
	fresh := read()
	if fresh.StoreState != "ENABLED" || fresh.Version <= stale.Version || !strings.HasSuffix(str(fresh.DescriptorPreview), "* BUMPED") {
		t.Fatalf("CPU4 after the 409 the store must still be ENABLED with the other session's suffix: %+v", fresh)
	}
	status, body = put(false, str(fresh.AcceptedTermsVersion), nil, fresh.Version)
	want("CPU4 retry", status, body, 200, `"state":"DISABLED"`)
	// CPU5: platform CLOSED: not OPEN for the page, an enable is refused (the page offers none), then OPEN again
	if err := ctl.platformOpen(ctx, false); err != nil {
		t.Fatalf("platform close: %v", err)
	}
	s = read()
	if s.PlatformState != "CLOSED" || s.StoreState != "DISABLED" {
		t.Fatalf("CPU5 closed: %+v", s)
	}
	status, body = put(true, str(s.TermsVersion), nil, s.Version)
	want("CPU5 enable on a CLOSED platform", status, body, 409, "platform_stripe_closed")
	if err := ctl.platformOpen(ctx, true); err != nil {
		t.Fatalf("platform open: %v", err)
	}
	if s = read(); s.PlatformState != "OPEN" {
		t.Fatalf("CPU5 reopened: %+v", s)
	}
	// CPU6: BLOCKED: enable is a 403, disable is still a 200 and the store stays BLOCKED; unblock returns it to DISABLED
	if err := ctl.block(ctx, true); err != nil {
		t.Fatalf("block: %v", err)
	}
	s = read()
	if s.StoreState != "BLOCKED" {
		t.Fatalf("CPU6 blocked: %+v", s)
	}
	status, body = put(true, str(s.TermsVersion), nil, s.Version)
	want("CPU6 enable while BLOCKED", status, body, 403, "platform_stripe_blocked")
	status, body = put(false, str(s.AcceptedTermsVersion), nil, s.Version)
	want("CPU6 disable while BLOCKED", status, body, 200, `"state":"BLOCKED"`)
	if err := ctl.block(ctx, false); err != nil {
		t.Fatalf("unblock: %v", err)
	}
	// CPU6b: not allowlisted. Withdrawing the allowlist of the enrolled store blocks it; once unblocked it is DISABLED and still not allowlisted: the
	// page reads that as "not open" (no enable offered) and a PUT would be refused platform_stripe_not_allowed; re-allowing restores the toggle.
	if err := ctl.allow(ctx, false); err != nil {
		t.Fatalf("disallow: %v", err)
	}
	if s = read(); s.StoreState != "BLOCKED" || s.Allowed {
		t.Fatalf("CPU6b disallowed: %+v", s)
	}
	if err := ctl.block(ctx, false); err != nil {
		t.Fatalf("unblock after disallow: %v", err)
	}
	s = read()
	if s.StoreState != "DISABLED" || s.Allowed || s.PlatformState != "OPEN" {
		t.Fatalf("CPU6b unblocked but not allowlisted: %+v", s)
	}
	status, body = put(true, str(s.TermsVersion), nil, s.Version)
	want("CPU6b enable while not allowlisted", status, body, 403, "platform_stripe_not_allowed")
	if err := ctl.allow(ctx, true); err != nil {
		t.Fatalf("allow: %v", err)
	}
	if s = read(); s.StoreState != "DISABLED" || !s.Allowed {
		t.Fatalf("CPU6b re-allowed: %+v", s)
	}
	// CPU7: the dialog pre-fills the retained suffix; the merchant clears it and enables again: no suffix remains
	s = read()
	if s.StoreState != "DISABLED" || !strings.HasSuffix(str(s.DescriptorPreview), "* BUMPED") {
		t.Fatalf("CPU7 before: %+v", s)
	}
	status, body = put(true, str(s.TermsVersion), nil, s.Version)
	want("CPU7 enable", status, body, 200, `"descriptor_preview":"LCPLATFORM"`)
	// CPU7b: the platform closes while the store is ENABLED: the page says "not open", offers no enable, keeps disable (the server allows it)
	if err := ctl.platformOpen(ctx, false); err != nil {
		t.Fatalf("platform close (enabled store): %v", err)
	}
	if s = read(); s.PlatformState != "CLOSED" || s.StoreState != "ENABLED" {
		t.Fatalf("CPU7b: %+v", s)
	}
	if err := ctl.platformOpen(ctx, true); err != nil {
		t.Fatalf("platform reopen: %v", err)
	}
	// the audit facts the browser harness asserts: every UI write is one definer call; refused or no-op requests write none
	n := func(action string) int {
		return countRows(t, e.f.owner, `SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND principal_id=$2 AND action=$3`, store, principal, action)
	}
	if n("stripe.platform.disable") != 2 || n("stripe.platform.enable") != 4 {
		t.Fatalf("audit rows: %d disable (want 2), %d enable (want 4: the fixture enable, CPU3, the bump, CPU7)", n("stripe.platform.disable"), n("stripe.platform.enable"))
	}
	// the wire never carries an account id, key or approval id
	for _, text := range []string{body, fmt.Sprint(s)} {
		pfSecretFree(t, "card wire", text)
	}
	// owner-pool assertion of what the browser harness checks last: the card method is enabled again and the enrollment keeps the accepted terms
	var enabled bool
	var terms string
	if err := e.f.owner.QueryRow(ctx, `SELECT mv.enabled,en.terms_version FROM payments.method_heads hd JOIN payments.method_versions mv ON mv.tenant_id=hd.tenant_id AND mv.store_id=hd.store_id
		AND mv.market_id=hd.market_id AND mv.country=hd.country AND mv.code=hd.code AND mv.version=hd.current_version
		JOIN payments.platform_stripe_enrollments en ON en.tenant_id=hd.tenant_id AND en.store_id=hd.store_id
		WHERE hd.tenant_id=$1 AND hd.store_id=$2 AND hd.code='stripe_checkout'`, e.a.f.tenantA, store).Scan(&enabled, &terms); err != nil || !enabled || terms != pfTerms {
		t.Fatalf("method enabled=%v terms=%q err=%v", enabled, terms, err)
	}
}
