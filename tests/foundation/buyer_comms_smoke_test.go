package foundation_test

// BCM01-BCM04 (contracts/storefront-v2.md §E, unit buyer-comms, migration 0090): author REAL_PG smoke through the real checkout, merchant and
// buyer HTTP code. It is NOT the independent gate (the tester writes that from the contract); it proves the unit's own claims once. The mail
// side is MOCK: internal/notify.Worker drives a fake Mailer (no SMTP, no network); the SMTP wire itself is internal/mail's own gate.
//   BCM01 TestBuyerCommsOutbox       triggers enqueue exactly once per (order, kind) in the order transaction (placed, paid, refunded, cancelled),
//                                    the worker sends them with the order's own bank snapshot, merchant batch <= 1 per 5 min, UNKNOWN never re-sent,
//                                    definite refusal retried with backoff then FAILED, dead SENDING -> UNKNOWN, caps hold, no recipient / stale are
//                                    SKIPPED, erasure clears the recipient hash and skips pending rows, the log has no body or address column
//   BCM02 TestGuestOrderLookup       order number + email or phone issues a working capability for the order's owner; every mismatch is the same
//                                    404; ip / ref throttles answer 429 with Retry-After; the same work is done for a hit and a miss
//   BCM03 TestNotifySettingsRoute    GET/PUT notification-settings (permissions, strict body) and the opt-out SKIPs merchant mail
//   BCM04 TestBuyerCommsCardCaptureAndStripeRefundTriggers  card capture (paid + merchant_new), unpaid expiry sends nothing, Stripe refund facts -> one refunded row
// Owner-pool writes (disclosed fixtures): one password credential + store_staff owner row so the merchant has an address (the OIDC fixture mints
// creators without one), ageing checkout.orders / inventory.reservations / notify.outbox timestamps, and one direct notify.enqueue call to prove
// the unique key.

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/mail"
	"livecommerce/internal/notify"
)

type bcmMailer struct {
	mu   sync.Mutex
	sent []mail.Message
	errs []error
}

func (m *bcmMailer) Send(_ context.Context, msg mail.Message) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var err error
	if len(m.errs) > 0 {
		err, m.errs = m.errs[0], m.errs[1:]
	}
	if err == nil {
		m.sent = append(m.sent, msg)
	}
	return "250 ok", err
}

func (m *bcmMailer) take() []mail.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.sent
	m.sent = nil
	return out
}

// bcmOwner gives the store creator a verified password email and an owner row (disclosed owner-pool fixture) and returns the address.
func (e *tcvEnv) bcmOwner() string {
	e.t.Helper()
	f := e.p.f
	addr := "owner-" + t04Tag() + "@merchant.example.test"
	hash := "$argon2id$v=19$m=4096,t=1,p=1$" + strings.Repeat("A", 22) + "$" + strings.Repeat("A", 43)
	mustExec(e.t, f.owner, `INSERT INTO identity.password_credentials(principal_id,email,password_hash,email_verified_at) VALUES($1,$2,$3,clock_timestamp())`, f.principalA, addr, hash)
	mustExec(e.t, f.owner, `INSERT INTO identity.store_staff(tenant_id,store_id,principal_id,role) VALUES($1,$2,$3,'owner') ON CONFLICT DO NOTHING`, f.tenantA, f.storeA1, f.principalA)
	return addr
}

func (e *tcvEnv) bcmWorker(m *bcmMailer, cap int) *notify.Worker {
	e.t.Helper()
	w, err := notify.NewWorker(e.p.worker, m, cap)
	if err != nil {
		e.t.Fatal(err)
	}
	return w
}

// bcmRow reads one outbox row: state, attempts, skip reason, recipient hash presence.
func (e *tcvEnv) bcmRow(order, kind string) (state string, attempts int, skip string, hasHash bool) {
	e.t.Helper()
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT state,attempts,coalesce(skip_reason,''),recipient_hash IS NOT NULL FROM notify.outbox WHERE order_id=$1 AND kind=$2`, order, kind).
		Scan(&state, &attempts, &skip, &hasHash); err != nil {
		e.t.Fatalf("outbox row %s/%s: %v", order, kind, err)
	}
	return
}

func (e *tcvEnv) bcmPlace(email string) (order string, b *tcvBuyer) {
	e.t.Helper()
	b = e.newBuyer()
	res, err := e.cofPlaceHome(b, email)
	if err != nil {
		e.t.Fatalf("place order: %v", err)
	}
	return res.OrderID, b
}

func (e *tcvEnv) bcmSetup() {
	e.t.Helper()
	e.grantCreator("orders:read", "payments:refund", "fulfillment:write")
	e.service("cvs_711", "MANUAL", 0)
	e.cofEnsureSettings(0, true, false, 72)
	// Disclosed owner-pool fixture, same isolation the gate file documents: the database is shared across tests and a
	// gate test can leave rows behind (TestBuyerCommsGateStoreCapDoesNotStarveOtherStores leaves 25 PENDING rows of its
	// capped store BY DESIGN), and claim_batch is global — these tests' claim-count assertions assume an empty outbox.
	mustExec(e.t, e.p.f.owner, `DELETE FROM notify.outbox`)
	mustExec(e.t, e.p.f.owner, `DELETE FROM checkout.lookup_throttle`)
}

func TestBuyerCommsOutbox(t *testing.T) {
	e := tcvNew(t)
	f := e.p.f
	ctx := context.Background()
	e.bcmSetup()
	ownerAddr := e.bcmOwner()
	m := &bcmMailer{}
	w := e.bcmWorker(m, 200)

	t.Run("placement enqueues placed + merchant_new in the order transaction; one buyer mail carries the bank snapshot", func(t *testing.T) {
		order, b := e.bcmPlace("Buyer@Example.test")
		if s, _, _, _ := e.bcmRow(order, "placed"); s != "PENDING" {
			t.Fatalf("placed row: %s", s)
		}
		if s, _, _, _ := e.bcmRow(order, "merchant_new"); s != "PENDING" {
			t.Fatalf("merchant_new row: %s", s)
		}
		if n := e.count(`SELECT count(*) FROM notify.outbox WHERE order_id=$1`, order); n != 2 {
			t.Fatalf("exactly placed + merchant_new, got %d rows", n)
		}
		if n, err := w.Once(ctx); err != nil || n != 2 {
			t.Fatalf("claimed %d err %v", n, err)
		}
		sent := m.take()
		if len(sent) != 2 {
			t.Fatalf("expected a buyer mail and a merchant mail, got %d", len(sent))
		}
		var buyerMail, merchantMail *mail.Message
		for i := range sent {
			switch sent[i].To {
			case "Buyer@Example.test":
				buyerMail = &sent[i]
			case ownerAddr:
				merchantMail = &sent[i]
			}
		}
		if buyerMail == nil || merchantMail == nil {
			t.Fatalf("recipients: %+v", sent)
		}
		no := notify.OrderNumber(order)
		for _, want := range []string{no, "123-456-7890", "Taiwan Bank", "https://" /* origin link */, "/zh-TW/orders/" + order, "UTC+8"} {
			if !strings.Contains(buyerMail.Text, want) {
				t.Errorf("buyer mail lacks %q:\n%s", want, buyerMail.Text)
			}
		}
		if strings.Contains(merchantMail.Text, "Buyer@") || strings.Contains(merchantMail.Text, "buyer@") || !strings.Contains(merchantMail.Text, no) {
			t.Errorf("merchant mail names the order, never the buyer:\n%s", merchantMail.Text)
		}
		for _, kind := range []string{"placed", "merchant_new"} {
			s, attempts, _, hasHash := e.bcmRow(order, kind)
			if s != "SENT" || attempts != 1 || !hasHash {
				t.Errorf("%s after send: %s attempts %d hash %v", kind, s, attempts, hasHash)
			}
		}
		var hash []byte
		if err := f.owner.QueryRow(ctx, `SELECT recipient_hash FROM notify.outbox WHERE order_id=$1 AND kind='placed'`, order).Scan(&hash); err != nil {
			t.Fatal(err)
		}
		if want := sha256.Sum256([]byte(order + ":buyer@example.test")); string(hash) != string(want[:]) {
			t.Error("recipient_hash is sha256(order id : lowercase address)")
		}
		if n, _ := w.Once(ctx); n != 0 || len(m.take()) != 0 {
			t.Fatal("a SENT row must never be claimed again")
		}
		_ = b
	})

	t.Run("the log has no body and no address column; direct enqueue of an existing (order, kind) is a no-op", func(t *testing.T) {
		var bad int
		if err := f.owner.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema='notify' AND table_name='outbox'
		 AND (column_name ~ '(body|text|html|subject|address|email|to_)' )`).Scan(&bad); err != nil || bad != 0 {
			t.Fatalf("outbox must keep no body or address column: %d %v", bad, err)
		}
		var id string
		if err := f.owner.QueryRow(ctx, `SELECT order_id::text FROM notify.outbox WHERE kind='placed' ORDER BY created_at LIMIT 1`).Scan(&id); err != nil {
			t.Fatal(err)
		}
		before := e.count(`SELECT count(*) FROM notify.outbox WHERE order_id=$1`, id)
		mustExec(t, f.owner, `SELECT notify.enqueue($1,$2,$3,'placed')`, e.tenant(), e.store(), id)
		if after := e.count(`SELECT count(*) FROM notify.outbox WHERE order_id=$1`, id); after != before {
			t.Fatalf("duplicate enqueue created a row: %d -> %d", before, after)
		}
	})

	t.Run("confirm -> paid mail; offline refund -> refunded mail; each exactly once", func(t *testing.T) {
		order, b := e.bcmPlace("pay@example.test")
		w.Once(ctx)
		m.take()
		if res := b.cofProof(order, t04Key("bcm-proof"), "12345", 0+int64(totalOf(t, e, order))); res.status != 200 {
			t.Fatalf("proof: %d %s", res.status, res.body)
		}
		if st, out := e.cofDecide(e.token(), order, "confirm", t04Key("bcm-confirm"), `{}`); st != 200 {
			t.Fatalf("confirm: %d %v", st, out)
		}
		if s, _, _, _ := e.bcmRow(order, "paid"); s != "PENDING" {
			t.Fatalf("paid row after confirm: %s", s)
		}
		if n := e.count(`SELECT count(*) FROM notify.outbox WHERE order_id=$1 AND kind='merchant_new'`, order); n != 1 {
			t.Fatalf("a confirmed transfer must not announce the order to the merchant again: %d", n)
		}
		w.Once(ctx)
		sent := m.take()
		if len(sent) != 1 || sent[0].To != "pay@example.test" || !strings.Contains(sent[0].Subject, "付款") {
			t.Fatalf("paid mail: %+v", sent)
		}
		if st, out := e.cofDecide(e.token(), order, "refund-offline", t04Key("bcm-refund"), `{}`); st != 200 {
			t.Fatalf("refund-offline: %d %v", st, out)
		}
		if s, _, _, _ := e.bcmRow(order, "refunded"); s != "PENDING" {
			t.Fatalf("refunded row: %s", s)
		}
		w.Once(ctx)
		if sent = m.take(); len(sent) != 1 || !strings.Contains(sent[0].Subject, "退款") {
			t.Fatalf("refunded mail: %+v", sent)
		}
	})

	t.Run("expiry -> cancelled mail", func(t *testing.T) {
		order, _ := e.bcmPlace("cancel@example.test")
		w.Once(ctx)
		m.take()
		e.cofAge(order, 80)
		if d, _ := e.cofExpire(order); d != "EXPIRED" {
			t.Fatalf("expire_held: %s", d)
		}
		if s, _, _, _ := e.bcmRow(order, "cancelled"); s != "PENDING" {
			t.Fatalf("cancelled row: %s", s)
		}
		w.Once(ctx)
		if sent := m.take(); len(sent) != 1 || sent[0].To != "cancel@example.test" || !strings.Contains(sent[0].Subject, "取消") {
			t.Fatalf("cancelled mail: %+v", sent)
		}
	})

	t.Run("merchant batch: at most one mail per store per 5 minutes, covering every pending order", func(t *testing.T) {
		// the previous subtests' merchant mails were just sent: the throttle is armed
		o1, _ := e.bcmPlace("m1@example.test")
		o2, _ := e.bcmPlace("m2@example.test")
		w.Once(ctx)
		for _, s := range m.take() {
			if s.To == ownerAddr {
				t.Fatalf("merchant mail inside the 5 minute window: %q", s.Subject)
			}
		}
		for _, o := range []string{o1, o2} {
			if s, _, _, _ := e.bcmRow(o, "merchant_new"); s != "PENDING" {
				t.Fatalf("merchant_new of %s must wait: %s", o, s)
			}
		}
		pendingMerchant := e.count(`SELECT count(*) FROM notify.outbox WHERE store_id=$1 AND kind='merchant_new' AND state='PENDING'`, e.store())
		mustExec(t, f.owner, `UPDATE notify.outbox SET claimed_at=clock_timestamp()-interval '6 minutes' WHERE kind='merchant_new' AND state='SENT'`)
		w.Once(ctx)
		var batch []mail.Message
		for _, s := range m.take() {
			if s.To == ownerAddr {
				batch = append(batch, s)
			}
		}
		if len(batch) != 1 || !strings.Contains(batch[0].Subject, "("+strconv.Itoa(pendingMerchant)+")") || !strings.Contains(batch[0].Text, notify.OrderNumber(o1)) ||
			!strings.Contains(batch[0].Text, notify.OrderNumber(o2)) || !strings.Contains(batch[0].Subject, "【") {
			t.Fatalf("one batch for the %d pending orders: %+v", pendingMerchant, batch)
		}
		if n := e.count(`SELECT count(*) FROM notify.outbox WHERE store_id=$1 AND kind='merchant_new' AND state='PENDING'`, e.store()); n != 0 {
			t.Fatalf("every pending order is in the batch: %d left", n)
		}
	})

	t.Run("UNKNOWN is final; a definite refusal retries with backoff and ends FAILED; a dead SENDING becomes UNKNOWN", func(t *testing.T) {
		order, _ := e.bcmPlace("flaky@example.test")
		mustExec(t, f.owner, `UPDATE notify.outbox SET state='SKIPPED',skip_reason='opted_out' WHERE order_id=$1 AND kind='merchant_new'`, order)
		m.errs = []error{mail.ErrUnknown}
		w.Once(ctx)
		if s, a, _, h := e.bcmRow(order, "placed"); s != "UNKNOWN" || a != 1 || !h {
			t.Fatalf("UNKNOWN row: %s %d %v", s, a, h)
		}
		if n, _ := w.Once(ctx); n != 0 || len(m.take()) != 0 {
			t.Fatal("an UNKNOWN row must never be re-sent")
		}

		order2, _ := e.bcmPlace("refused@example.test")
		mustExec(t, f.owner, `UPDATE notify.outbox SET state='SKIPPED',skip_reason='opted_out' WHERE order_id=$1 AND kind='merchant_new'`, order2)
		for attempt := 1; attempt <= 3; attempt++ {
			m.errs = []error{mail.ErrFailed}
			if n, _ := w.Once(ctx); n != 1 {
				t.Fatalf("attempt %d claimed %d", attempt, n)
			}
			s, a, _, _ := e.bcmRow(order2, "placed")
			if a != attempt || (attempt < 3 && s != "PENDING") || (attempt == 3 && s != "FAILED") {
				t.Fatalf("after refusal %d: %s attempts %d", attempt, s, a)
			}
			if attempt < 3 {
				if n, _ := w.Once(ctx); n != 0 {
					t.Fatal("backoff: the row is not due yet")
				}
				mustExec(t, f.owner, `UPDATE notify.outbox SET next_attempt_at=clock_timestamp() WHERE order_id=$1 AND kind='placed'`, order2)
			}
		}

		order3, _ := e.bcmPlace("dead@example.test")
		mustExec(t, f.owner, `UPDATE notify.outbox SET state='SKIPPED',skip_reason='opted_out' WHERE order_id=$1 AND kind='merchant_new'`, order3)
		mustExec(t, f.owner, `UPDATE notify.outbox SET state='SENDING',attempts=1,batch_id=gen_random_uuid(),claimed_at=clock_timestamp()-interval '16 minutes' WHERE order_id=$1 AND kind='placed'`, order3)
		w.Once(ctx)
		if s, _, _, _ := e.bcmRow(order3, "placed"); s != "UNKNOWN" || len(m.take()) != 0 {
			t.Fatalf("a dead SENDING row is UNKNOWN, never re-sent: %s", s)
		}
	})

	t.Run("no recipient and stale rows are SKIPPED, never sent", func(t *testing.T) {
		noMail, _ := e.bcmPlace("")
		stale, _ := e.bcmPlace("stale@example.test")
		for _, o := range []string{noMail, stale} {
			mustExec(t, f.owner, `UPDATE notify.outbox SET state='SKIPPED',skip_reason='opted_out' WHERE order_id=$1 AND kind='merchant_new'`, o)
		}
		mustExec(t, f.owner, `UPDATE notify.outbox SET created_at=clock_timestamp()-interval '25 hours' WHERE order_id=$1 AND kind='placed'`, stale)
		w.Once(ctx)
		if s, _, r, _ := e.bcmRow(noMail, "placed"); s != "SKIPPED" || r != "no_recipient" {
			t.Fatalf("no email: %s/%s", s, r)
		}
		if s, _, r, _ := e.bcmRow(stale, "placed"); s != "SKIPPED" || r != "stale" {
			t.Fatalf("stale: %s/%s", s, r)
		}
		if len(m.take()) != 0 {
			t.Fatal("nothing may be sent for either")
		}
	})

	t.Run("caps: per-store hourly and the daily budget leave rows PENDING", func(t *testing.T) {
		var ids []string
		for i := 0; i < 3; i++ {
			o, _ := e.bcmPlace("cap" + string(rune('a'+i)) + "@example.test")
			mustExec(t, f.owner, `UPDATE notify.outbox SET state='SKIPPED',skip_reason='opted_out' WHERE order_id=$1 AND kind='merchant_new'`, o)
			ids = append(ids, o)
		}
		claim := func(daily, hourly int) int {
			var raw []byte
			if err := e.p.worker.QueryRow(ctx, `SELECT notify.claim_batch(10,$1,$2)`, daily, hourly).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			var out []json.RawMessage
			if err := json.Unmarshal(raw, &out); err != nil {
				t.Fatal(err)
			}
			return len(out)
		}
		// rows claimed in the last hour already count: ask for one more than they use
		used := e.count(`SELECT count(*) FROM notify.outbox WHERE store_id=$1 AND kind<>'merchant_new' AND state IN ('SENDING','SENT','UNKNOWN') AND claimed_at>=clock_timestamp()-interval '1 hour'`, e.store())
		if n := claim(1000, used+1); n != 1 {
			t.Fatalf("hourly cap used+1 admits exactly one more: %d", n)
		}
		var daily int
		if err := f.owner.QueryRow(ctx, `SELECT count(DISTINCT coalesce(batch_id::text,order_id::text||kind)) FROM notify.outbox WHERE state IN ('SENDING','SENT','UNKNOWN')
		 AND claimed_at>=to_timestamp((floor((extract(epoch FROM clock_timestamp())+28800)/86400)*86400-28800)::double precision)`).Scan(&daily); err != nil {
			t.Fatal(err)
		}
		if n := claim(daily, 1000); n != 0 {
			t.Fatalf("a spent daily budget claims nothing: %d", n)
		}
		if n := claim(daily+5, 1000); n != 2 {
			t.Fatalf("room for the remaining two: %d", n)
		}
		_ = ids
	})

	t.Run("erasure clears the recipient hash and skips pending rows", func(t *testing.T) {
		order, _ := e.bcmPlace("erase@example.test")
		mustExec(t, f.owner, `UPDATE notify.outbox SET state='SKIPPED',skip_reason='opted_out' WHERE order_id=$1 AND kind='merchant_new'`, order)
		w.Once(ctx)
		m.take()
		if _, _, _, h := e.bcmRow(order, "placed"); !h {
			t.Fatal("sent row keeps a hash before erasure")
		}
		// a second, still-pending mail of the same order
		mustExec(t, f.owner, `SELECT notify.enqueue($1,$2,$3,'cancelled')`, e.tenant(), e.store(), order)
		var owner string
		if err := f.owner.QueryRow(ctx, `SELECT owner_id::text FROM checkout.orders WHERE id=$1`, order).Scan(&owner); err != nil {
			t.Fatal(err)
		}
		// checkout.clear_buyer_email is what customers.apply_erasure calls (0088)
		if err := f.owner.QueryRow(ctx, `SELECT checkout.clear_buyer_email($1,$2,$3)`, e.tenant(), e.store(), owner).Scan(new(int)); err != nil {
			t.Fatal(err)
		}
		if s, _, _, h := e.bcmRow(order, "placed"); s != "SENT" || h {
			t.Fatalf("erasure keeps the state but drops the hash: %s %v", s, h)
		}
		if s, _, r, _ := e.bcmRow(order, "cancelled"); s != "SKIPPED" || r != "erased" {
			t.Fatalf("pending row after erasure: %s/%s", s, r)
		}
		w.Once(ctx)
		if len(m.take()) != 0 {
			t.Fatal("nothing may be sent after erasure")
		}
	})
}

// totalOf reads the order total (the proof's amount hint).
func totalOf(t *testing.T, e *tcvEnv, order string) int64 {
	t.Helper()
	var total int64
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT total_minor FROM checkout.orders WHERE id=$1`, order).Scan(&total); err != nil {
		t.Fatal(err)
	}
	return total
}

func TestGuestOrderLookup(t *testing.T) {
	e := tcvNew(t)
	f := e.p.f
	ctx := context.Background()
	e.bcmSetup()
	order, _ := e.bcmPlace("Guest@Example.test")
	var phone string
	if err := f.owner.QueryRow(ctx, `SELECT d.phone FROM checkout.orders o JOIN storefront.destination_snapshots d ON d.tenant_id=o.tenant_id AND d.store_id=o.store_id AND d.owner_id=o.owner_id AND d.id=o.destination_id WHERE o.id=$1`, order).Scan(&phone); err != nil {
		t.Fatal(err)
	}
	number := notify.OrderNumber(order)
	ip := func(v string) func(*http.Request) {
		return func(r *http.Request) { r.Header.Set("X-Commerce-Client-IP", v) }
	}
	lookup := func(token, ref, contact, addr string) bhResponse {
		return e.bh.request(t, "POST", "/v1/buyer/orders/lookup", token, "", map[string]string{"order_ref": ref, "contact": contact}, ip(addr))
	}
	freshToken := func() string {
		return strings.TrimRight(strings.ReplaceAll(strings.ReplaceAll(randomToken(), "+", "-"), "/", "_"), "=")
	}

	t.Run("a match issues a working capability for the order's owner (email, then phone, then full id)", func(t *testing.T) {
		for i, tc := range []struct{ ref, contact string }{
			{number, "guest@example.test"},
			{strings.ToLower(strings.ReplaceAll(number, "-", " ")), phone},
			{order, " GUEST@example.test "},
		} {
			token := freshToken()
			res := lookup(token, tc.ref, tc.contact, "203.0.113."+string(rune('1'+i)))
			if res.status != 200 {
				t.Fatalf("case %d: %d %s", i, res.status, res.body)
			}
			var out struct {
				OrderID string `json:"order_id"`
			}
			if err := json.Unmarshal(res.body, &out); err != nil || out.OrderID != order {
				t.Fatalf("case %d body %s", i, res.body)
			}
			// the new token is a real capability: it reads the order (GET /v1/buyer/orders/{id}) and lists it
			if got := e.bh.request(t, "GET", "/v1/buyer/orders/"+order, token, "", nil, nil); got.status != 200 {
				t.Fatalf("case %d: the issued capability cannot read the order: %d %s", i, got.status, got.body)
			}
		}
		if n := e.count(`SELECT count(*) FROM buyer.capability_sessions s JOIN checkout.orders o ON o.owner_id=s.owner_id WHERE o.id=$1`, order); n < 4 {
			t.Fatalf("one original + three lookup sessions for the same owner, got %d", n)
		}
	})

	t.Run("every mismatch is the same 404", func(t *testing.T) {
		var bodies []string
		for i, tc := range []struct{ ref, contact string }{
			{number, "other@example.test"},           // wrong email
			{number, "0999000111"},                   // wrong phone
			{"FFFF-FFFF-FFFF", "guest@example.test"}, // unknown order
			{"0000-0000-0000", "0999000111"},         // unknown order, phone
		} {
			res := lookup(freshToken(), tc.ref, tc.contact, "198.51.100."+string(rune('1'+i)))
			if res.status != 404 {
				t.Fatalf("case %d: %d %s", i, res.status, res.body)
			}
			var env struct{ Code, Message string }
			_ = json.Unmarshal(res.body, &env)
			bodies = append(bodies, env.Code+"|"+env.Message)
		}
		for _, b := range bodies[1:] {
			if b != bodies[0] || !strings.HasPrefix(b, "not_found") {
				t.Fatalf("mismatch answers differ: %v", bodies)
			}
		}
		if n := e.count(`SELECT count(*) FROM buyer.capability_sessions WHERE created_at>clock_timestamp()-interval '1 minute'`); n < 1 {
			t.Fatal("sanity: earlier sessions exist")
		}
	})

	t.Run("a draft or other store's order never matches; malformed input is 422", func(t *testing.T) {
		for _, tc := range []struct{ ref, contact string }{{"nothex", "a@b.c"}, {number, ""}, {number, "not a contact"}, {number[:4], "a@b.c"}} {
			if res := lookup(freshToken(), tc.ref, tc.contact, "198.51.100.77"); res.status != 422 {
				t.Fatalf("%q/%q: %d %s", tc.ref, tc.contact, res.status, res.body)
			}
		}
		if res := e.bh.request(t, "POST", "/v1/buyer/orders/lookup", "", "", map[string]string{"order_ref": number, "contact": "a@b.c"}, nil); res.status != 401 {
			t.Fatalf("no bearer: %d", res.status)
		}
		if res := e.bh.request(t, "GET", "/v1/buyer/orders/lookup", freshToken(), "", nil, nil); res.status != 405 {
			t.Fatalf("GET: %d", res.status)
		}
	})

	t.Run("throttles: per order ref 5, per client IP 10, per 10 minutes, with Retry-After", func(t *testing.T) {
		ref := "ABCD-EF01-2345"
		for i := 1; i <= 5; i++ {
			if res := lookup(freshToken(), ref, "x@example.test", "192.0.2."+string(rune('0'+i))); res.status != 404 {
				t.Fatalf("ref attempt %d: %d", i, res.status)
			}
		}
		res := lookup(freshToken(), ref, "x@example.test", "192.0.2.99")
		if res.status != 429 || res.header.Get("Retry-After") == "" {
			t.Fatalf("6th attempt on one ref: %d retry-after %q", res.status, res.header.Get("Retry-After"))
		}
		// the real order's ref is not blocked by someone else hammering another ref
		if res := lookup(freshToken(), number, "guest@example.test", "192.0.2.98"); res.status != 200 {
			t.Fatalf("an unrelated ref must still work: %d %s", res.status, res.body)
		}
		same := "192.0.2.200"
		for i := 0; i < 10; i++ {
			lookup(freshToken(), "0000-0000-00"+string(rune('0'+i))+"0", "x@example.test", same)
		}
		if res := lookup(freshToken(), "0000-0000-0099", "x@example.test", same); res.status != 429 {
			t.Fatalf("11th attempt from one IP: %d", res.status)
		}
	})

	t.Run("the guest session is the buyer's own: lookup never reaches another owner's order", func(t *testing.T) {
		// a fresh order: the first order's ref bucket (5 per 10 minutes) was used up by the subtests above
		mine, _ := e.bcmPlace("mine@example.test")
		other, _ := e.bcmPlace("someone-else@example.test")
		token := freshToken()
		if res := lookup(token, notify.OrderNumber(mine), "mine@example.test", "192.0.2.150"); res.status != 200 {
			t.Fatalf("lookup: %d %s", res.status, res.body)
		}
		if got := e.bh.request(t, "GET", "/v1/buyer/orders/"+mine, token, "", nil, nil); got.status != 200 {
			t.Fatalf("own order: %d", got.status)
		}
		if got := e.bh.request(t, "GET", "/v1/buyer/orders/"+other, token, "", nil, nil); got.status != 403 {
			t.Fatalf("another owner's order through a guest session: %d %s", got.status, got.body)
		}
	})
}

// BCM05: a guest-lookup session is VIEW-ONLY (integrator ruling): it reads its one order and nothing else, on every other buyer route it is 403
// from the shared gate; the checkout-issued capability keeps every right.
func TestGuestLookupSessionIsViewOnly(t *testing.T) {
	e := tcvNew(t)
	e.bcmSetup()
	mine, full := e.bcmPlace("viewonly@example.test")
	other, _ := e.bcmPlace("neighbour@example.test")
	guest := strings.TrimRight(strings.ReplaceAll(strings.ReplaceAll(randomToken(), "+", "-"), "/", "_"), "=")
	res := e.bh.request(t, "POST", "/v1/buyer/orders/lookup", guest, "", map[string]string{"order_ref": notify.OrderNumber(mine), "contact": "viewonly@example.test"},
		func(r *http.Request) { r.Header.Set("X-Commerce-Client-IP", "198.18.0.1") })
	if res.status != 200 {
		t.Fatalf("lookup: %d %s", res.status, res.body)
	}
	var view *string
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT view_order_id::text FROM buyer.capability_sessions WHERE token_hash=sha256($1::bytea)`, []byte(guest)).Scan(&view); err != nil || view == nil || *view != mine {
		t.Fatalf("the lookup session row names its order: %v %v", view, err)
	}
	g := func(method, path, key string, input any) bhResponse {
		return e.bh.request(t, method, path, guest, key, input, nil)
	}
	// reads of its own order (status, payment status, bank-transfer instructions) are allowed; the payment / transfer GET may answer any non-403 of its own
	for _, ok := range []string{"/v1/buyer/session", "/v1/buyer/orders/" + mine} {
		if r := g("GET", ok, "", nil); r.status != 200 {
			t.Errorf("GET %s with the guest session: %d %s", ok, r.status, r.body)
		}
	}
	for _, tc := range []struct {
		method, path string
		body         any
	}{
		{"GET", "/v1/buyer/orders", nil},
		{"GET", "/v1/buyer/orders/" + other, nil},
		{"GET", "/v1/buyer/privacy", nil},
		{"POST", "/v1/buyer/privacy/export", nil},
		{"POST", "/v1/buyer/privacy/erasure", map[string]string{"confirm": "ERASE"}},
		{"PUT", "/v1/buyer/consents", map[string]any{"purpose": "ads_personalization", "channel": "web", "granted": true, "source": "settings"}},
		{"GET", "/v1/buyer/orders/" + other + "/payment", nil},
		{"GET", "/v1/buyer/orders/" + other + "/bank-transfer", nil},
		{"POST", "/v1/buyer/orders/" + mine + "/payment/handoff", nil},
		{"PUT", "/v1/buyer/orders/" + mine + "/bank-transfer/proof", map[string]any{"last5": "12345", "amount_minor": 100, "paid_at": "2026-10-01T00:00:00Z"}},
		{"POST", "/v1/buyer/orders/" + mine + "/payment/prepare", map[string]any{"method_code": "stripe_checkout", "method_version": 1, "locale": "en"}},
		{"GET", "/v1/buyer/cart", nil},
		{"GET", "/v1/buyer/catalog", nil},
	} {
		key := ""
		if tc.method != "GET" && !strings.HasSuffix(tc.path, "/handoff") { // handoff is keyless (ServeHTTP refuses a key before the view gate)
			key = t04Key("bcm-view")
		}
		if r := g(tc.method, tc.path, key, tc.body); r.status != 403 {
			t.Errorf("%s %s with the guest session: %d %s (want 403)", tc.method, tc.path, r.status, r.body)
		}
	}
	// own order: GET payment status and bank-transfer instructions pass the gate (the bank_transfer order has no hosted payment, so that GET may be
	// 404/409 from the route itself, never 403); the instructions are a 200
	if r := g("GET", "/v1/buyer/orders/"+mine+"/payment", "", nil); r.status == 403 {
		t.Errorf("GET payment of its own order must pass the view-only gate: %d %s", r.status, r.body)
	}
	if r := g("GET", "/v1/buyer/orders/"+mine+"/bank-transfer", "", nil); r.status != 200 {
		t.Errorf("GET bank-transfer instructions of its own order: %d %s", r.status, r.body)
	}
	if n := e.count(`SELECT count(*) FROM customers.privacy_actions WHERE kind='ERASURE'`); n != 0 {
		t.Errorf("no erasure may have started: %d", n)
	}
	// the checkout-issued capability is unchanged
	for _, path := range []string{"/v1/buyer/session", "/v1/buyer/orders", "/v1/buyer/orders/" + mine, "/v1/buyer/privacy"} {
		if r := full.req("GET", path, "", nil, nil); r.status != 200 {
			t.Errorf("checkout session GET %s: %d %s", path, r.status, r.body)
		}
	}
	if r := full.req("POST", "/v1/buyer/privacy/export", t04Key("bcm-export"), nil, nil); r.status != 200 {
		t.Errorf("checkout session privacy export: %d %s", r.status, r.body)
	}
}

func TestNotifySettingsRoute(t *testing.T) {
	e := tcvNew(t)
	ctx := context.Background()
	e.bcmSetup()
	e.bcmOwner()
	url := "/v1/admin/stores/" + e.store() + "/notification-settings"
	if st, out, raw := e.mcall(e.token(), "GET", url, "", ""); st != 200 || out["merchant_new_order_email"] != true {
		t.Fatalf("default is on: %d %s", st, raw)
	}
	reader, _ := e.member("orders:read", "integration:read")
	if st, _, _ := e.mcall(reader, "GET", url, "", ""); st != 200 {
		t.Fatalf("integration:read reads: %d", st)
	}
	if st, out, _ := e.mcall(reader, "PUT", url, t04Key("bcm-set"), `{"merchant_new_order_email":false}`); st != 403 {
		t.Fatalf("integration:read must not write: %d %v", st, out)
	}
	if st, _, _ := e.mcall("", "GET", url, "", ""); st != 401 {
		t.Fatalf("no bearer: %d", st)
	}
	for _, bad := range []string{`{}`, `{"merchant_new_order_email":null}`, `{"merchant_new_order_email":"no"}`, `{"merchant_new_order_email":false,"x":1}`} {
		if st, _, _ := e.mcall(e.token(), "PUT", url, t04Key("bcm-bad"), bad); st != 422 && st != 400 {
			t.Fatalf("%s: %d", bad, st)
		}
	}
	if st, _, _ := e.mcall(e.token(), "PUT", url, "", `{"merchant_new_order_email":false}`); st != 422 {
		t.Fatalf("a PUT needs an Idempotency-Key: %d", st)
	}
	for i := 0; i < 2; i++ { // a retry of the same set is harmless
		if st, out, raw := e.mcall(e.token(), "PUT", url, t04Key("bcm-off"), `{"merchant_new_order_email":false}`); st != 200 || out["merchant_new_order_email"] != false {
			t.Fatalf("set off: %d %s", st, raw)
		}
	}
	if st, out, _ := e.mcall(e.token(), "GET", url, "", ""); st != 200 || out["merchant_new_order_email"] != false {
		t.Fatalf("read back off: %d %v", st, out)
	}
	m := &bcmMailer{}
	w := e.bcmWorker(m, 200)
	order, _ := e.bcmPlace("optout@example.test")
	w.Once(ctx)
	if s, _, r, _ := e.bcmRow(order, "merchant_new"); s != "SKIPPED" || r != "opted_out" {
		t.Fatalf("opted-out store: %s/%s", s, r)
	}
	for _, s := range m.take() {
		if strings.Contains(s.To, "merchant.example.test") {
			t.Fatal("no merchant mail for an opted-out store")
		}
	}
	if st, _, _ := e.mcall(e.token(), "PUT", url, t04Key("bcm-on"), `{"merchant_new_order_email":true}`); st != 200 {
		t.Fatal("set on")
	}
	if n := e.count(`SELECT count(*) FROM notify.store_settings WHERE store_id=$1`, e.store()); n != 1 {
		t.Fatalf("one settings row: %d", n)
	}
}

// BCM04: the card and Stripe-refund triggers. The Stripe MOCK harness (srfNew) cannot open its registrar login in a focused run, so these two
// transitions are driven inside ROLLED-BACK owner-pool transactions (disclosed fixture): a DRAFT card order moved to CONFIRMED (what apply_capture
// does), and two refund facts inserted with FK triggers skipped (session_replication_role=replica) while ONLY the notify trigger is ENABLE ALWAYS. The
// trigger functions still run as their SECURITY DEFINER owner, so the policies and GUC scoping they depend on are the real ones.
func TestBuyerCommsCardCaptureAndStripeRefundTriggers(t *testing.T) {
	e := tcvNew(t)
	f := e.p.f
	ctx := context.Background()
	e.bcmSetup()
	b := e.newBuyer()
	in := b.h.input
	in.PaymentMode, in.BuyerEmail = "card", "card@example.test"
	res, err := e.svc.Begin(ctx, b.cap.Token, e.store(), t04Key("bcm-card"), in)
	if err != nil {
		t.Fatalf("card Begin: %v", err)
	}
	order := res.OrderID
	rows := func(tx pgx.Tx, kind string) int {
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM notify.outbox WHERE order_id=$1 AND kind=$2`, order, kind).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	t.Run("DRAFT card order -> CONFIRMED enqueues paid + merchant_new, never placed", func(t *testing.T) {
		tx, err := f.owner.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if rows(tx, "paid")+rows(tx, "merchant_new")+rows(tx, "placed") != 0 {
			t.Fatal("a DRAFT card order announces nothing")
		}
		if _, err := tx.Exec(ctx, `UPDATE checkout.orders SET commercial_state='CONFIRMED' WHERE id=$1`, order); err != nil {
			t.Fatalf("confirm: %v", err)
		}
		if rows(tx, "paid") != 1 || rows(tx, "merchant_new") != 1 || rows(tx, "placed") != 0 {
			t.Fatalf("paid=%d merchant_new=%d placed=%d", rows(tx, "paid"), rows(tx, "merchant_new"), rows(tx, "placed"))
		}
		if _, err := tx.Exec(ctx, `UPDATE checkout.orders SET commercial_state='CANCELLED' WHERE id=$1`, order); err != nil {
			t.Fatalf("cancel: %v", err)
		}
		if rows(tx, "cancelled") != 1 {
			t.Fatal("CONFIRMED -> CANCELLED enqueues cancelled")
		}
	})
	t.Run("a DRAFT order that expires unpaid sends nothing", func(t *testing.T) {
		tx, err := f.owner.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if _, err := tx.Exec(ctx, `UPDATE checkout.orders SET commercial_state='CANCELLED' WHERE id=$1`, order); err != nil {
			t.Fatal(err)
		}
		if rows(tx, "cancelled") != 0 {
			t.Fatal("an abandoned DRAFT checkout was never announced, so it is not cancelled either")
		}
	})
	t.Run("Stripe refund facts enqueue one refunded row per order however many partial refunds", func(t *testing.T) {
		tx, err := f.owner.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		must := func(q string, args ...any) {
			t.Helper()
			if _, err := tx.Exec(ctx, q, args...); err != nil {
				t.Fatalf("%v: %s", err, q)
			}
		}
		must(`ALTER TABLE payments.refund_facts ENABLE ALWAYS TRIGGER notify_stripe_refund`)
		must(`SET LOCAL session_replication_role=replica`)
		var owner string
		if err := tx.QueryRow(ctx, `SELECT owner_id::text FROM checkout.orders WHERE id=$1`, order).Scan(&owner); err != nil {
			t.Fatal(err)
		}
		attempt := randomUUID()
		for i, amount := range []int{1000, 1500} {
			refund := randomUUID()
			must(`INSERT INTO payments.stripe_refunds(tenant_id,store_id,id,attempt_id,order_id,owner_id,principal_id,environment,account_id,credential_version,payment_intent_id,
			  currency,amount_minor,reason,create_params,requested_at,resend_until) VALUES($1,$2,$3,$4,$5,$6,$7,'SANDBOX','acct_TestBcm0001',1,'pi_test_bcm','TWD',$8,'requested_by_customer','{}',
			  clock_timestamp(),clock_timestamp()+interval '20 hours')`, e.tenant(), e.store(), refund, attempt, order, owner, f.principalA, amount)
			must(`INSERT INTO payments.refund_facts(tenant_id,store_id,refund_id,attempt_id,kind,amount_minor,currency,stripe_refund_id,source_report_hash)
			  VALUES($1,$2,$3,$4,'SUCCEEDED',$5,'TWD',$6,sha256($7::bytea))`, e.tenant(), e.store(), refund, attempt, amount, "re_test_bcm_"+string(rune('a'+i)), []byte(refund))
			if rows(tx, "refunded") != 1 {
				t.Fatalf("after refund fact %d: %d refunded rows", i+1, rows(tx, "refunded"))
			}
		}
	})
}
