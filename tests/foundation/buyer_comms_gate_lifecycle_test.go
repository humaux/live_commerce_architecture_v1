package foundation_test

// BG04: independent gate for contracts/storefront-v2.md §E1 + §E4: every mail kind is triggered by the REAL order lifecycle (real buyer Begin, real
// merchant routes, the real ECPay status hook, the real Stripe capture and refund path against the independent stripetest fake), delivered through
// the real SMTP adapter to the loopback fake, and its content is checked from the contract: store name, order number XXXX-XXXX-XXXX, order link on
// the store's own published origin, the order's own bank snapshot + deadline, carrier/tracking or CVS store, HTML escaping, no remote image, no other
// buyer's PII. Evidence tier REAL_PG + MOCK Stripe + MOCK ECPay + SMTP FAKE.
//   placed (bank_transfer)   place via Begin                     paid (transfer)  proof + merchant confirm      shipped (manual)  PUT shipment
//   placed (pay_at_pickup)   CVS API order                       shipped (CVS)    ECPay status 2030 then 2073   cancelled         expiry
//   paid (card capture)      Stripe MOCK                         refunded          offline refund + 2 Stripe partial refunds
// Disclosed fixtures: the store name is set by the owner pool to a string with HTML metacharacters (escaping check); order ageing for expiry
// (cofAge, as the checkout-offline gate does); the owner address (bcmOwner).

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"

	"livecommerce/internal/checkout"
	"livecommerce/internal/mail/mailtest"
	"livecommerce/internal/notify"
)

const bgStoreName = `Tom & Jerry's <b>Shop</b>`

// bgBegin is tcvBuyer.begin with a buyer e-mail.
func (b *tcvBuyer) bgBegin(dest string, quoteID string, serviceVersion int64, mode, email string) (checkout.Result, error) {
	in := checkout.Input{QuoteID: quoteID, DestinationID: dest, CartVersion: b.cartVersion(), ServiceVersion: serviceVersion, AllocationVersion: 1, PaymentMode: mode, BuyerEmail: email}
	return b.e.svc.Begin(context.Background(), b.cap.Token, b.e.store(), t04Key("bg-begin"), in)
}

// bgCVS places an API-service CVS order (verified ECPay pickup) with an e-mail; mode "" = card (DRAFT hold), "pay_at_pickup" = CONFIRMED.
func (g *bgEnv) bgCVS(code, mode, email string) (checkout.Result, *tcvBuyer) {
	g.t.Helper()
	b := g.newBuyer()
	_, pickup := g.verifiedPickup(b, code)
	dest, err := b.destination("cvs_711", pickup, tppName, tppPhone)
	if err != nil {
		g.t.Fatalf("destination: %v", err)
	}
	q, err := b.quote(code)
	if err != nil {
		g.t.Fatalf("quote: %v", err)
	}
	res, err := b.bgBegin(dest.ID, q.ID, g.svcVer[code], mode, email)
	if err != nil {
		g.t.Fatalf("begin %q: %v", mode, err)
	}
	return res, b
}

var bgURL = regexp.MustCompile(`https?://[^\s"'<>)]+`)

// bgNext drains the worker and returns the single new mail to addr (the test fails on 0 or more than 1).
type bgMailbox struct {
	g    *bgEnv
	w    *notify.Worker
	seen map[string]int
}

func (m *bgMailbox) next(addr string) mailtest.Received {
	m.g.t.Helper()
	m.g.drain(m.w)
	all := m.g.mailsTo(addr)
	if len(all)-m.seen[addr] != 1 {
		m.g.t.Fatalf("%d new mails to %s, want exactly 1 (total %d, seen %d)", len(all)-m.seen[addr], addr, len(all), m.seen[addr])
	}
	m.seen[addr] = len(all)
	return all[len(all)-1]
}

func (m *bgMailbox) none(addr string) {
	m.g.t.Helper()
	m.g.drain(m.w)
	if got := len(m.g.mailsTo(addr)); got != m.seen[addr] {
		m.g.t.Fatalf("%d unexpected new mails to %s", got-m.seen[addr], addr)
	}
}

func hasCJK(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func TestBuyerCommsGateLifecycle(t *testing.T) {
	g := bgNew(t, tcvOpts{stripe: true})
	f := g.p.f
	g.r.startWorker(t)
	g.startDispatcher()
	g.connect("C2C")
	g.cvsSettings(tcvAllChains, true, "20000", 500)
	api, _, _ := g.service("cvs_711", "API", 0)
	g.cofEnsureSettings(0, true, false, 72)
	mustExec(t, f.owner, `UPDATE control.stores SET name=$2 WHERE id=$1`, g.store(), bgStoreName)
	endpoint := g.endpointID()
	tag := t04Tag()
	w := g.worker(100000)
	box := &bgMailbox{g: g, w: w, seen: map[string]int{}}
	em := func(n string) string { return fmt.Sprintf("%s-%s@buyers.example.test", n, tag) }
	emails := map[string]string{} // order -> buyer address
	number := func(o string) string { return notify.OrderNumber(o) }
	var allBuyers []string

	// every mail: store name, order number, link, locale, escaping, no remote asset, no foreign URL; returns it for kind-specific checks
	common := func(t *testing.T, m mailtest.Received, order, addr string, extraHosts ...string) {
		t.Helper()
		link := g.origin + "/zh-TW/orders/" + order
		if !strings.Contains(m.Subject, bgStoreName) && !strings.Contains(m.Text, bgStoreName) {
			t.Errorf("mail lacks the store name: %q", m.Subject)
		}
		if !strings.Contains(m.Text, number(order)) || !strings.Contains(m.Text, link) {
			t.Errorf("plain text lacks order number %s or link %s:\n%s", number(order), link, m.Text)
		}
		if m.HTML == "" || !strings.Contains(m.HTML, number(order)) || !strings.Contains(m.HTML, link) {
			t.Errorf("html alternative lacks order number or link")
		}
		if !hasCJK(m.Subject) {
			t.Errorf("default locale is zh-TW: subject %q", m.Subject)
		}
		if strings.Contains(m.HTML, "<b>Shop") || strings.Contains(m.HTML, bgStoreName) || (strings.Contains(m.HTML, "Tom") && !strings.Contains(m.HTML, "Tom &amp; Jerry")) {
			t.Errorf("a store name with HTML metacharacters must never reach the html part unescaped")
		}
		low := strings.ToLower(m.HTML)
		for _, bad := range []string{"<img", "<script", "<iframe", "<link", "src=", "background:url", "url(", "@import", "<style src"} {
			if strings.Contains(low, bad) {
				t.Errorf("html part contains %q (no remote image / tracking pixel allowed)", bad)
			}
		}
		for _, u := range bgURL.FindAllString(m.Text+" "+m.HTML, -1) {
			ok := strings.HasPrefix(u, g.origin+"/")
			for _, h := range extraHosts {
				ok = ok || strings.HasPrefix(strings.ReplaceAll(u, "&amp;", "&"), h)
			}
			if !ok {
				t.Errorf("mail links to %q: only the store's own origin (and a merchant tracking URL) may appear", u)
			}
		}
		if !strings.EqualFold(m.To, addr) {
			t.Errorf("recipient %s, want %s", m.To, addr)
		}
	}
	deadline := func(order string) string {
		var hhmm string
		if err := f.owner.QueryRow(context.Background(), `SELECT to_char(expires_at AT TIME ZONE 'Asia/Taipei','HH24:MI') FROM checkout.orders WHERE id=$1`, order).Scan(&hhmm); err != nil {
			t.Fatal(err)
		}
		return hhmm
	}
	place := func(name string) (string, *tcvBuyer) {
		o, b := g.bcmPlace(em(name))
		emails[o] = em(name)
		allBuyers = append(allBuyers, em(name))
		return o, b
	}

	// ---- A: bank transfer placed -> paid -> shipped (manual) ---------------------------------------------------------------------------
	oA, bA := place("alice")
	var placedA mailtest.Received
	t.Run("placed (bank_transfer) carries store, number, link, the order's bank snapshot and the deadline", func(t *testing.T) {
		placedA = box.next(em("alice"))
		common(t, placedA, oA, em("alice"))
		for _, want := range []string{"Taiwan Bank", "Shop Ltd", "123-456-7890", deadline(oA)} {
			if !strings.Contains(placedA.Text, want) {
				t.Errorf("placed mail lacks %q:\n%s", want, placedA.Text)
			}
		}
		if !strings.Contains(placedA.HTML, "123-456-7890") {
			t.Error("html part lacks the account number")
		}
	})
	var merchantMail mailtest.Received
	t.Run("merchant new-order mail: counts orders, names no buyer, carries no bank detail", func(t *testing.T) {
		got := g.mailsTo(g.owner)
		if len(got) != 1 {
			t.Fatalf("merchant mails: %d, want 1", len(got))
		}
		merchantMail = got[0]
		box.seen[g.owner] = 1
		if !strings.Contains(merchantMail.Text, number(oA)) {
			t.Errorf("merchant mail lacks the order number:\n%s", merchantMail.Text)
		}
		for _, bad := range []string{em("alice"), "123-456-7890", "Taiwan Bank", "buyers.example.test"} {
			if strings.Contains(merchantMail.Text+merchantMail.HTML+merchantMail.Subject, bad) {
				t.Errorf("merchant mail leaks %q", bad)
			}
		}
	})
	t.Run("paid (transfer confirmed): proof by the buyer, confirm by the merchant, one mail", func(t *testing.T) {
		if res := bA.cofProof(oA, t04Key("bgl-proof"), "12345", totalOf(t, g.tcvEnv, oA)); res.status != 200 {
			t.Fatalf("proof %d %s", res.status, res.body)
		}
		box.none(em("alice"))
		if st, out := g.cofDecide(g.token(), oA, "confirm", t04Key("bgl-confirm"), `{}`); st != 200 {
			t.Fatalf("confirm %d %v", st, out)
		}
		m := box.next(em("alice"))
		common(t, m, oA, em("alice"))
		if m.Subject == placedA.Subject {
			t.Errorf("paid and placed share a subject %q", m.Subject)
		}
		if strings.Contains(m.Text, "123-456-7890") {
			t.Errorf("the paid mail must not repeat the bank account")
		}
	})
	t.Run("shipped (manual): carrier, tracking number and the https tracking URL, escaped in html", func(t *testing.T) {
		trackURL := "https://track.carrier.example/t?no=SF" + tag + "&lang=zh"
		st, _, raw := g.mcall(g.token(), "PUT", "/v1/admin/stores/"+g.store()+"/orders/"+oA+"/shipment", t04Key("bgl-ship"), mfxShipBody(0, "SHIPPED", "sf_express", "", "SF"+tag, trackURL, "", ""))
		if st != 200 {
			t.Fatalf("manual shipment: %d %s", st, raw)
		}
		m := box.next(em("alice"))
		common(t, m, oA, em("alice"), "https://track.carrier.example/")
		for _, want := range []string{"SF" + tag, trackURL} {
			if !strings.Contains(m.Text, want) {
				t.Errorf("shipped mail text lacks %q:\n%s", want, m.Text)
			}
		}
		if !strings.Contains(m.HTML, "SF"+tag) || !strings.Contains(m.HTML, "lang=zh") || strings.Contains(m.HTML, "no=SF"+tag+"&lang") {
			t.Errorf("tracking URL must be HTML-escaped (&amp;) in the html part:\n%s", m.HTML)
		}
		// editing the shipment later must not mail again (exactly once per (order, kind))
		st, _, raw = g.mcall(g.token(), "PUT", "/v1/admin/stores/"+g.store()+"/orders/"+oA+"/shipment", t04Key("bgl-ship2"), mfxShipBody(1, "SHIPPED", "sf_express", "", "SF"+tag+"X", trackURL, "", ""))
		_ = raw
		box.none(em("alice"))
		_ = st
	})

	// ---- F: merchant-typed bank text with HTML metacharacters ------------------------------------------------------------------------------
	t.Run("placed: merchant-typed bank fields are HTML-escaped in the html part and literal in the text part", func(t *testing.T) {
		var version int64
		if err := f.owner.QueryRow(context.Background(), `SELECT version FROM checkout.bank_transfer_settings WHERE store_id=$1`, g.store()).Scan(&version); err != nil {
			t.Fatal(err)
		}
		body := fmt.Sprintf(`{"expected_version":%d,"enabled":true,"allow_cvs":false,"bank_name":"A&B <i>Bank</i>","branch":"Taipei","account_name":"Shop \"Q\" Ltd","account_number":"123-456-7890","window_hours":72}`, version)
		if st, out, raw := g.mcall(g.token(), "PUT", "/v1/admin/stores/"+g.store()+"/bank-transfer-settings", t04Key("bgl-esc"), body); st != 200 {
			t.Skipf("the settings route refuses markup in the bank name (%d %v %s): escaping of merchant text cannot be driven through the product", st, out, raw)
		}
		oF, _ := place("frank")
		m := box.next(em("frank"))
		common(t, m, oF, em("frank"))
		if !strings.Contains(m.Text, "A&B <i>Bank</i>") {
			t.Errorf("text part must carry the bank name literally:\n%s", m.Text)
		}
		if strings.Contains(m.HTML, "<i>Bank</i>") || !strings.Contains(m.HTML, "A&amp;B &lt;i&gt;Bank&lt;/i&gt;") {
			t.Errorf("html part must escape the bank name:\n%s", m.HTML)
		}
	})

	// ---- B: offline refund ---------------------------------------------------------------------------------------------------------------
	oB, bB := place("bob")
	t.Run("refunded (offline) after a confirmed transfer", func(t *testing.T) {
		box.next(em("bob")) // placed
		if res := bB.cofProof(oB, t04Key("bgl-proofb"), "54321", totalOf(t, g.tcvEnv, oB)); res.status != 200 {
			t.Fatalf("proof %d", res.status)
		}
		if st, out := g.cofDecide(g.token(), oB, "confirm", t04Key("bgl-confb"), `{}`); st != 200 {
			t.Fatalf("confirm %d %v", st, out)
		}
		box.next(em("bob")) // paid
		if st, out := g.cofDecide(g.token(), oB, "refund-offline", t04Key("bgl-refund"), `{}`); st != 200 {
			t.Fatalf("refund-offline %d %v", st, out)
		}
		m := box.next(em("bob"))
		common(t, m, oB, em("bob"))
		if st := g.states(oB); st["refunded"] != "SENT" || st["placed"] != "SENT" || st["paid"] != "SENT" {
			t.Errorf("rows: %v", st)
		}
	})

	// ---- C: expiry ---------------------------------------------------------------------------------------------------------------------------
	oC, _ := place("carol")
	t.Run("cancelled (expiry of an unpaid transfer hold)", func(t *testing.T) {
		box.next(em("carol")) // placed
		g.cofAge(oC, 80)
		if d, _ := g.cofExpire(oC); d != "EXPIRED" {
			t.Fatalf("expire: %s", d)
		}
		m := box.next(em("carol"))
		common(t, m, oC, em("carol"))
	})

	// ---- D: pay-at-pickup CVS (placed with pickup, shipped via ECPay status) ------------------------------------------------------------
	t.Run("placed (pay_at_pickup) names the pickup store; shipped (CVS) once at AT_DC and not again at AT_STORE", func(t *testing.T) {
		res, _ := g.bgCVS(api, "pay_at_pickup", em("dave"))
		oD := res.OrderID
		emails[oD] = em("dave")
		allBuyers = append(allBuyers, em("dave"))
		placed := box.next(em("dave"))
		common(t, placed, oD, em("dave"))
		if !strings.Contains(placed.Text, "Stage 7-ELEVEN") && !strings.Contains(placed.Text, "131386") {
			t.Errorf("pay_at_pickup placed mail lacks the pickup store:\n%s", placed.Text)
		}
		if strings.Contains(placed.Text, "123-456-7890") {
			t.Errorf("a pay_at_pickup order has no bank details")
		}
		if st, _, raw := g.ship(g.token(), oD, 0, "", true); st != 202 {
			t.Fatalf("request shipment: %d %s", st, raw)
		}
		g.awaitShip(oD, "CREATED")
		box.none(em("dave")) // a created label is not a shipment yet
		g.tppStatuses(endpoint, oD, "2030")
		m := box.next(em("dave"))
		common(t, m, oD, em("dave"))
		if !strings.Contains(m.Text, "Stage 7-ELEVEN") && !strings.Contains(m.Text, "131386") {
			t.Errorf("CVS shipped mail lacks the pickup store:\n%s", m.Text)
		}
		var shippingNo string
		_ = f.owner.QueryRow(context.Background(), `SELECT coalesce((SELECT value FROM jsonb_each_text(to_jsonb(s)) WHERE key ILIKE '%shipping%no%' OR key ILIKE '%shipping_number%' LIMIT 1),'') FROM fulfillment.cvs_shipments s WHERE order_id=$1 ORDER BY attempt DESC LIMIT 1`, oD).Scan(&shippingNo)
		if shippingNo != "" && !strings.Contains(m.Text, shippingNo) {
			t.Errorf("CVS shipped mail lacks the shipping number %q:\n%s", shippingNo, m.Text)
		}
		g.tppStatuses(endpoint, oD, "2073", "2067")
		box.none(em("dave"))
		if c := g.count(`SELECT count(*) FROM notify.outbox WHERE order_id=$1 AND kind='shipped'`, oD); c != 1 {
			t.Errorf("%d shipped rows for one CVS shipment", c)
		}
	})

	// ---- E: card capture (Stripe MOCK) and two partial Stripe refunds -------------------------------------------------------------
	t.Run("paid (card capture, Stripe MOCK) sends no 'placed'; two partial Stripe refunds send one 'refunded'", func(t *testing.T) {
		// an abandoned card hold sends nothing at all, not even a cancellation
		hold, _ := g.bgCVS(api, "", em("ghost"))
		// card holds last at most 15 minutes (orders_expiry_window): age the hold inside that window
		mustExec(t, f.owner, `UPDATE checkout.orders SET created_at=clock_timestamp()-interval '1 hour',expires_at=clock_timestamp()-interval '50 minutes' WHERE id=$1`, hold.OrderID)
		mustExec(t, f.owner, `UPDATE inventory.reservations SET created_at=clock_timestamp()-interval '1 hour',expires_at=clock_timestamp()-interval '50 minutes' WHERE id=$1`, hold.OrderID)
		if d, _ := g.cofExpire(hold.OrderID); d != "EXPIRED" {
			t.Fatalf("expire abandoned card hold: %s", d)
		}
		if c := g.count(`SELECT count(*) FROM notify.outbox WHERE order_id=$1`, hold.OrderID); c != 0 {
			t.Errorf("an abandoned card hold left %d outbox rows", c)
		}
		res, b := g.bgCVS(api, "", em("erin"))
		oE := res.OrderID
		emails[oE] = em("erin")
		allBuyers = append(allBuyers, em("erin"))
		if c := g.count(`SELECT count(*) FROM notify.outbox WHERE order_id=$1`, oE); c != 0 {
			t.Fatalf("an unpaid card hold must not enqueue anything: %d rows", c)
		}
		o := g.payHold(res, b)
		m := box.next(em("erin"))
		common(t, m, oE, em("erin"))
		if st := g.states(oE); st["paid"] != "SENT" || st["placed"] != "" {
			t.Errorf("rows after capture: %v (want paid only)", st)
		}
		r1 := g.r.mustRefund(t, o, 1000, "requested_by_customer")
		g.r.awaitRefundFact(t, r1, o.attempt, "SUCCEEDED")
		m = box.next(em("erin"))
		common(t, m, oE, em("erin"))
		r2 := g.r.mustRefund(t, o, 500, "requested_by_customer")
		g.r.awaitRefundFact(t, r2, o.attempt, "SUCCEEDED")
		box.none(em("erin"))
		if c := g.count(`SELECT count(*) FROM notify.outbox WHERE order_id=$1 AND kind='refunded'`, oE); c != 1 {
			t.Errorf("%d refunded rows after two partial refunds", c)
		}
	})

	if dir := os.Getenv("LC_BG_MAIL_DIR"); dir != "" { // evidence: every captured message exactly as the SMTP fake received it
		_ = os.MkdirAll(dir, 0o700)
		for i, m := range g.srv.Messages() {
			_ = os.WriteFile(filepath.Join(dir, fmt.Sprintf("%02d.eml", i+1)), m.Raw, 0o600)
		}
	}
	// ---- no other buyer's PII in any mail ---------------------------------------------------------------------------------------------------
	t.Run("no mail carries another buyer's address, order number or order id", func(t *testing.T) {
		for _, m := range g.srv.Messages() {
			blob := m.Subject + "\n" + m.Text + "\n" + m.HTML + "\n" + string(m.Raw)
			if strings.EqualFold(m.To, g.owner) {
				for o := range emails {
					_ = o
				}
				for _, a := range allBuyers {
					if strings.Contains(strings.ToLower(blob), strings.ToLower(a)) {
						t.Errorf("merchant mail contains buyer address %s", a)
					}
				}
				continue
			}
			var mine string
			for o, a := range emails {
				if strings.EqualFold(a, m.To) {
					mine = o
				}
			}
			if mine == "" {
				t.Errorf("mail to an unknown recipient %s", m.To)
				continue
			}
			for o, a := range emails {
				if o == mine {
					continue
				}
				for _, secret := range []string{a, number(o), o, strings.ReplaceAll(number(o), "-", "")} {
					if strings.Contains(strings.ToLower(blob), strings.ToLower(secret)) {
						t.Errorf("mail to %s contains %q belonging to another buyer's order %s", m.To, secret, o)
					}
				}
			}
		}
		// the buyers' own phone number and delivery name never appear (not part of any contract field)
		for _, m := range g.srv.Messages() {
			if strings.Contains(m.Text+m.HTML, tppPhone) || strings.Contains(m.Text+m.HTML, tppName) {
				t.Errorf("mail to %s contains the recipient phone or name", m.To)
			}
		}
	})
}
