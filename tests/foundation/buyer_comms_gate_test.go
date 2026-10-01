package foundation_test

// BG01-BG03, BG06, BG09: independent gate for contracts/storefront-v2.md §E1-E3, §E6, §E7 (unit buyer-comms, migration 0090). Written from the
// contract by the test author, not from the implementation; it shares only the foundation harness (tcvEnv, real order paths) with the author smoke.
// Evidence tier: REAL_PG + a loopback SMTP FAKE (internal/mail/mailtest, wire-level: the real *mail.SMTP adapter talks TLS to it). No real mailbox.
//   BG01 TestBuyerCommsGateExactlyOnce   racing workers / raw claims / concurrent triggers: one mail per (order, kind); SENT is final
//   BG02 TestBuyerCommsGateUnknownRetry  SMTP drop after the dot = UNKNOWN, never re-sent; definite refusals back off 2 min / 10 min, FAILED after 3
//   BG03 TestBuyerCommsGateCaps          daily share <= 60% of the cap (login-code ledger untouched), 30 buyer mails per store per rolling hour
//   BG06 TestBuyerCommsGateMerchantBatch <= 1 merchant mail per store per 5 minutes, owners only, opt-out through the real route
//   BG09 TestBuyerCommsGateErasure       the real buyer erasure clears the recipient and skips pending rows; no address or body is ever stored
// Disclosed owner-pool fixtures: DELETE FROM notify.outbox / checkout.lookup_throttle at the start (isolates the shared database from earlier tests),
// one password credential + owner row so the merchant has an address, ageing of timestamps (claimed_at / next_attempt_at / created_at / orders),
// direct notify.enqueue to build more than one kind on one real order for the hourly cap. Everything else goes through the real routes.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"livecommerce/internal/mail"
	"livecommerce/internal/mail/mailtest"
	"livecommerce/internal/notify"
)

type bgEnv struct {
	*tcvEnv
	srv   *mailtest.Server
	smtp  *mail.SMTP
	owner string // the merchant owner's verified address
}

// bgNew builds the env, a loopback SMTP fake behind the REAL adapter, the merchant owner address and an empty outbox.
func bgNew(t *testing.T, opts ...tcvOpts) *bgEnv {
	t.Helper()
	e := tcvNew(t, opts...)
	e.grantCreator("orders:read", "payments:refund", "fulfillment:write", "integration:manage", "integration:read")
	srv := mailtest.New(t)
	sm, err := mail.NewSMTP(srv.Config(srv.Username))
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, e.p.f.owner, `DELETE FROM notify.outbox`)
	mustExec(t, e.p.f.owner, `DELETE FROM checkout.lookup_throttle`)
	g := &bgEnv{tcvEnv: e, srv: srv, smtp: sm}
	g.owner = e.bcmOwner()
	return g
}

// bgBank is bgNew plus the bank-transfer settings (72 h window) of the harness store.
func bgBank(t *testing.T) *bgEnv {
	t.Helper()
	g := bgNew(t)
	g.cofEnsureSettings(0, true, false, 72)
	return g
}

func (g *bgEnv) worker(dailyCap int) *notify.Worker {
	g.t.Helper()
	w, err := notify.NewWorker(g.p.worker, g.smtp, dailyCap)
	if err != nil {
		g.t.Fatal(err)
	}
	return w
}

// drain runs claims until one comes back empty.
func (g *bgEnv) drain(w *notify.Worker) {
	g.t.Helper()
	for i := 0; i < 40; i++ {
		n, err := w.Once(context.Background())
		if err != nil {
			g.t.Fatalf("claim: %v", err)
		}
		if n == 0 {
			return
		}
	}
	g.t.Fatal("outbox did not drain in 40 claims")
}

func (g *bgEnv) mailsTo(addr string) []mailtest.Received {
	var out []mailtest.Received
	for _, m := range g.srv.Messages() {
		if strings.EqualFold(m.To, addr) {
			out = append(out, m)
		}
	}
	return out
}

func (g *bgEnv) setMerchantMail(on bool) {
	g.t.Helper()
	st, out, raw := g.mcall(g.token(), "PUT", "/v1/admin/stores/"+g.store()+"/notification-settings", t04Key("bg-optout"), fmt.Sprintf(`{"merchant_new_order_email":%v}`, on))
	if st != 200 {
		g.t.Fatalf("notification-settings: %d %v %s", st, out, raw)
	}
}

// outboxRows lists (kind,state) of one order.
func (g *bgEnv) states(order string) map[string]string {
	g.t.Helper()
	rows, err := g.p.f.owner.Query(context.Background(), `SELECT kind,state FROM notify.outbox WHERE order_id=$1`, order)
	if err != nil {
		g.t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, s string
		if err := rows.Scan(&k, &s); err != nil {
			g.t.Fatal(err)
		}
		out[k] = s
	}
	return out
}

func (g *bgEnv) due(order, kind string) {
	g.t.Helper()
	mustExec(g.t, g.p.f.owner, `UPDATE notify.outbox SET next_attempt_at=clock_timestamp()-interval '1 second' WHERE order_id=$1 AND kind=$2`, order, kind)
}

func (g *bgEnv) nextIn(order, kind string) float64 {
	g.t.Helper()
	var s float64
	if err := g.p.f.owner.QueryRow(context.Background(), `SELECT extract(epoch FROM next_attempt_at-clock_timestamp()) FROM notify.outbox WHERE order_id=$1 AND kind=$2`, order, kind).Scan(&s); err != nil {
		g.t.Fatal(err)
	}
	return s
}

func TestBuyerCommsGateExactlyOnce(t *testing.T) {
	g := bgBank(t)
	ctx := context.Background()
	tag := t04Tag()

	t.Run("six workers racing over one outbox: one SMTP mail per (order, kind), one merchant batch", func(t *testing.T) {
		const n = 9
		emails := map[string]string{}
		for i := 0; i < n; i++ {
			email := fmt.Sprintf("race%d-%s@buyers.example.test", i, tag)
			o, _ := g.bcmPlace(email)
			emails[o] = email
		}
		var wg sync.WaitGroup
		for i := 0; i < 6; i++ {
			w := g.worker(1000)
			wg.Add(1)
			go func() {
				defer wg.Done()
				for k := 0; k < 4; k++ {
					if _, err := w.Once(ctx); err != nil {
						t.Error(err)
					}
				}
			}()
		}
		wg.Wait()
		g.drain(g.worker(1000))
		for o, email := range emails {
			got := g.mailsTo(email)
			if len(got) != 1 || !strings.Contains(got[0].Text, notify.OrderNumber(o)) {
				t.Errorf("order %s: %d mails to %s (want exactly 1 carrying the order number)", o, len(got), email)
			}
			if s := g.states(o); s["placed"] != "SENT" || s["merchant_new"] != "SENT" {
				t.Errorf("order %s rows: %v", o, s)
			}
		}
		ownerMails := g.mailsTo(g.owner)
		if len(ownerMails) != 1 {
			t.Fatalf("the merchant owner got %d mails, want 1 batch", len(ownerMails))
		}
		for o := range emails {
			if !strings.Contains(ownerMails[0].Text, notify.OrderNumber(o)) {
				t.Errorf("merchant batch lacks order %s", notify.OrderNumber(o))
			}
		}
		if got := g.srv.DataCount(); got != n+1 {
			t.Errorf("SMTP DATA commands: %d, want %d buyer mails + 1 merchant batch", got, n+1)
		}
		if c := g.count(`SELECT count(*) FROM notify.outbox WHERE attempts<>1 OR state<>'SENT'`); c != 0 {
			t.Errorf("%d rows are not SENT after exactly one attempt", c)
		}
	})

	t.Run("fourteen raw claim_batch callers never receive the same (order, kind) twice", func(t *testing.T) {
		const n = 12
		want := map[string]bool{}
		for i := 0; i < n; i++ {
			o, _ := g.bcmPlace(fmt.Sprintf("claim%d-%s@buyers.example.test", i, tag))
			want[o] = true
		}
		results := make(chan []notify.Payload, 14)
		var wg sync.WaitGroup
		for i := 0; i < 14; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				var raw []byte
				if err := g.p.worker.QueryRow(ctx, `SELECT notify.claim_batch(3,100000,1000)`).Scan(&raw); err != nil {
					t.Error(err)
					return
				}
				var ps []notify.Payload
				if err := json.Unmarshal(raw, &ps); err != nil {
					t.Error(err)
					return
				}
				results <- ps
			}()
		}
		wg.Wait()
		close(results)
		seen := map[string]int{}
		var batches []string
		for ps := range results {
			for _, p := range ps {
				if p.Kind == notify.KindMerchantNew {
					t.Errorf("merchant batch claimed inside the 5 minute window opened by the previous subtest")
					continue
				}
				seen[p.OrderID+"/"+p.Kind]++
				batches = append(batches, p.BatchID)
			}
		}
		if len(seen) != n {
			t.Errorf("claimed %d distinct (order, kind), want %d", len(seen), n)
		}
		for k, c := range seen {
			if c != 1 {
				t.Errorf("%s was handed to %d claimers", k, c)
			}
			if !want[strings.Split(k, "/")[0]] {
				t.Errorf("claimed a foreign row %s", k)
			}
		}
		// SENT is final: a late or replayed result can neither revive nor flip it
		for _, b := range batches {
			mustExec(t, g.p.worker, `SELECT notify.record_result($1::uuid,'SENT',NULL)`, b)
		}
		for _, b := range batches {
			for _, late := range []string{"FAILED", "UNKNOWN", "SENT"} {
				var n int
				if err := g.p.worker.QueryRow(ctx, `SELECT notify.record_result($1::uuid,$2,NULL)`, b, late).Scan(&n); err != nil {
					t.Fatalf("record_result: %v", err)
				}
				if n != 0 {
					t.Errorf("a late %s result touched %d rows of a SENT batch", late, n)
				}
			}
		}
		if c := g.count(`SELECT count(*) FROM notify.outbox WHERE order_id = ANY($1) AND kind='placed' AND state='SENT'`, bgKeys(want)); c != n {
			t.Errorf("SENT rows %d, want %d", c, n)
		}
	})

	t.Run("concurrent confirm, concurrent expiry and concurrent enqueue each leave one row", func(t *testing.T) {
		w := g.worker(1000)
		order, b := g.bcmPlace("confirm-" + tag + "@buyers.example.test")
		g.drain(w)
		if res := b.cofProof(order, t04Key("bg-proof"), "12345", totalOf(t, g.tcvEnv, order)); res.status != 200 {
			t.Fatalf("proof: %d %s", res.status, res.body)
		}
		var wg sync.WaitGroup
		var ok int
		var mu sync.Mutex
		for i := 0; i < 6; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				st, _, _ := g.mcall(g.token(), "POST", "/v1/admin/stores/"+g.store()+"/orders/"+order+"/bank-transfer/confirm", t04Key(fmt.Sprintf("bg-conf-%d", i)), `{}`)
				if st == 200 {
					mu.Lock()
					ok++
					mu.Unlock()
				}
			}(i)
		}
		wg.Wait()
		if ok < 1 {
			t.Fatal("no confirm succeeded")
		}
		if c := g.count(`SELECT count(*) FROM notify.outbox WHERE order_id=$1 AND kind='paid'`, order); c != 1 {
			t.Fatalf("%d paid rows after %d concurrent confirms", c, 6)
		}
		g.drain(w)
		g.drain(w)
		if got := g.mailsTo("confirm-" + tag + "@buyers.example.test"); len(got) != 2 {
			t.Errorf("placed + paid expected, got %d mails", len(got))
		}

		o2, _ := g.bcmPlace("expire-" + tag + "@buyers.example.test")
		g.drain(w)
		g.cofAge(o2, 80)
		for i := 0; i < 6; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); _, _ = g.p.worker.Exec(ctx, `SELECT checkout.expire_held($1,1)`, o2) }()
		}
		wg.Wait()
		if c := g.count(`SELECT count(*) FROM notify.outbox WHERE order_id=$1 AND kind='cancelled'`, o2); c != 1 {
			t.Fatalf("%d cancelled rows after concurrent expiry", c)
		}
		g.drain(w)
		if got := g.mailsTo("expire-" + tag + "@buyers.example.test"); len(got) != 2 {
			t.Errorf("placed + cancelled expected, got %d mails", len(got))
		}

		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := g.p.f.owner.Exec(ctx, `SELECT notify.enqueue($1,$2,$3,'shipped')`, g.tenant(), g.store(), o2); err != nil {
					t.Error(err)
				}
			}()
		}
		wg.Wait()
		if c := g.count(`SELECT count(*) FROM notify.outbox WHERE order_id=$1 AND kind='shipped'`, o2); c != 1 {
			t.Fatalf("%d shipped rows after 8 concurrent enqueues", c)
		}
	})
}

func bgKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestBuyerCommsGateUnknownRetry(t *testing.T) {
	g := bgBank(t)
	g.setMerchantMail(false)
	ctx := context.Background()
	tag := t04Tag()
	w := g.worker(1000)

	t.Run("an SMTP connection dropped after the dot is UNKNOWN and is never sent again", func(t *testing.T) {
		email := "unknown-" + tag + "@buyers.example.test"
		order, _ := g.bcmPlace(email)
		g.srv.SetNextFault(mailtest.FaultDropAfterDot)
		g.drain(w)
		if s, a, _, h := g.bcmRow(order, "placed"); s != "UNKNOWN" || a != 1 || !h {
			t.Fatalf("after the drop: %s attempts %d hash %v", s, a, h)
		}
		before := g.srv.DataCount()
		for i := 0; i < 5; i++ {
			g.due(order, "placed")
			g.drain(w)
		}
		// even a day later (the stale rule only retires PENDING rows)
		mustExec(t, g.p.f.owner, `UPDATE notify.outbox SET created_at=clock_timestamp()-interval '30 hours',next_attempt_at=clock_timestamp()-interval '30 hours' WHERE order_id=$1`, order)
		g.drain(w)
		if s, a, _, _ := g.bcmRow(order, "placed"); s != "UNKNOWN" || a != 1 {
			t.Errorf("UNKNOWN must stay UNKNOWN with one attempt: %s %d", s, a)
		}
		if g.srv.DataCount() != before {
			t.Errorf("SMTP DATA count moved from %d to %d: an UNKNOWN mail was re-sent", before, g.srv.DataCount())
		}
	})

	t.Run("a dead SENDING row (worker killed mid-send) becomes UNKNOWN, not a second send", func(t *testing.T) {
		order, _ := g.bcmPlace("dead-" + tag + "@buyers.example.test")
		mustExec(t, g.p.f.owner, `UPDATE notify.outbox SET state='SENDING',attempts=1,batch_id=gen_random_uuid(),claimed_at=clock_timestamp()-interval '20 minutes' WHERE order_id=$1 AND kind='placed'`, order)
		before := g.srv.DataCount()
		g.drain(w)
		if s, _, _, _ := g.bcmRow(order, "placed"); s != "UNKNOWN" {
			t.Errorf("dead SENDING row is %s", s)
		}
		if g.srv.DataCount() != before {
			t.Error("a dead SENDING row was sent again")
		}
	})

	t.Run("definite refusals back off 2 min then 10 min and end FAILED after the third attempt", func(t *testing.T) {
		email := "refused-" + tag + "@buyers.example.test"
		order, _ := g.bcmPlace(email)
		wantBackoff := []struct{ lo, hi float64 }{{90, 150}, {540, 660}}
		for attempt := 1; attempt <= 3; attempt++ {
			g.srv.SetNextFault(mailtest.FaultRcpt)
			g.drain(w)
			s, a, _, _ := g.bcmRow(order, "placed")
			if a != attempt {
				t.Fatalf("attempt counter %d after refusal %d", a, attempt)
			}
			if attempt < 3 {
				if s != "PENDING" {
					t.Fatalf("after refusal %d the row is %s, want PENDING", attempt, s)
				}
				if in := g.nextIn(order, "placed"); in < wantBackoff[attempt-1].lo || in > wantBackoff[attempt-1].hi {
					t.Errorf("backoff after refusal %d is %.0fs, want %v", attempt, in, wantBackoff[attempt-1])
				}
				// not due yet: an immediate pass must not even dial
				g.srv.SetNextFault(mailtest.FaultRcpt)
				g.drain(w)
				if _, a2, _, _ := g.bcmRow(order, "placed"); a2 != attempt {
					t.Fatalf("a row inside its backoff was claimed again (attempts %d)", a2)
				}
				g.srv.SetNextFault(mailtest.FaultNone)
				g.due(order, "placed")
			} else if s != "FAILED" {
				t.Fatalf("after the third refusal the row is %s, want FAILED", s)
			}
		}
		g.srv.SetNextFault(mailtest.FaultNone)
		g.due(order, "placed")
		g.drain(w)
		if s, a, _, _ := g.bcmRow(order, "placed"); s != "FAILED" || a != 3 {
			t.Errorf("FAILED is final: %s attempts %d", s, a)
		}
		if got := g.mailsTo(email); len(got) != 0 {
			t.Errorf("a refused mail was delivered %d times", len(got))
		}
	})

	t.Run("a refusal followed by success delivers exactly once", func(t *testing.T) {
		email := "dial-" + tag + "@buyers.example.test"
		order, _ := g.bcmPlace(email)
		g.srv.SetNextFault(mailtest.FaultDial)
		g.drain(w)
		if s, a, _, _ := g.bcmRow(order, "placed"); s != "PENDING" || a != 1 {
			t.Fatalf("after a failed dial: %s %d", s, a)
		}
		g.due(order, "placed")
		g.drain(w)
		g.due(order, "placed")
		g.drain(w)
		if got := g.mailsTo(email); len(got) != 1 {
			t.Errorf("%d mails delivered, want 1", len(got))
		}
		if s, a, _, _ := g.bcmRow(order, "placed"); s != "SENT" || a != 2 {
			t.Errorf("final row: %s attempts %d", s, a)
		}
	})
	_ = ctx
}

// bgDailyBudget returns a NewWorker daily cap whose 60% share is exactly want mails.
func bgDailyCapFor(want int) int {
	for c := 20; c < 100000; c++ {
		if c*60/100 == want {
			return c
		}
	}
	return 0
}

func (g *bgEnv) sentToday() int {
	g.t.Helper()
	return g.count(`SELECT count(DISTINCT coalesce(batch_id::text,order_id::text||kind)) FROM notify.outbox WHERE state IN ('SENDING','SENT','UNKNOWN')
	  AND claimed_at>=to_timestamp((floor((extract(epoch FROM clock_timestamp())+28800)/86400)*86400-28800)::double precision)`)
}

func TestBuyerCommsGateCaps(t *testing.T) {
	g := bgBank(t)
	ctx := context.Background()
	tag := t04Tag()
	g.setMerchantMail(false)

	t.Run("daily share: buyer mail never exceeds 60% of COMMERCE_MAIL_DAILY_CAP, the rest stays PENDING for login codes", func(t *testing.T) {
		const dailyCap = 20 // 60% = 12
		budget := dailyCap * 60 / 100
		loginBefore := g.count(`SELECT count(*) FROM identity.auth_throttle`)
		var orders []string
		for i := 0; i < budget+3; i++ {
			o, _ := g.bcmPlace(fmt.Sprintf("daily%d-%s@buyers.example.test", i, tag))
			orders = append(orders, o)
		}
		w := g.worker(dailyCap)
		for pass := 0; pass < 4; pass++ {
			g.drain(w)
		}
		if got := g.sentToday(); got != budget {
			t.Fatalf("mails reaching SENDING today: %d, want exactly the budget %d", got, budget)
		}
		if got := len(g.srv.Messages()); got != budget {
			t.Errorf("SMTP received %d mails, want %d", got, budget)
		}
		pending := g.count(`SELECT count(*) FROM notify.outbox WHERE order_id = ANY($1) AND kind='placed' AND state='PENDING' AND skip_reason IS NULL`, orders)
		if pending != 3 {
			t.Errorf("%d rows left PENDING over the cap, want 3 (not SKIPPED, not FAILED)", pending)
		}
		if g.sentToday() > dailyCap*60/100 || dailyCap-g.sentToday() < dailyCap*40/100 {
			t.Errorf("at least 40%% of the daily cap must stay free for merchant login codes")
		}
		if after := g.count(`SELECT count(*) FROM identity.auth_throttle`); after != loginBefore {
			t.Errorf("identity login-code ledger changed by buyer mail: %d -> %d", loginBefore, after)
		}
		// the next UTC+8 day: the held rows go out. Move today's claims to yesterday (fixture) and the rows are due.
		mustExec(t, g.p.f.owner, `UPDATE notify.outbox SET claimed_at=clock_timestamp()-interval '30 hours' WHERE state='SENT'`)
		g.drain(w)
		if c := g.count(`SELECT count(*) FROM notify.outbox WHERE order_id = ANY($1) AND kind='placed' AND state='SENT'`, orders); c != budget+3 {
			t.Errorf("after the day rolled over %d of %d rows are SENT", c, budget+3)
		}
	})

	t.Run("daily share counts merchant batches too (buyer + merchant together stay under the share)", func(t *testing.T) {
		g.setMerchantMail(true)
		mustExec(t, g.p.f.owner, `DELETE FROM notify.outbox`)
		g.srv.SetNextFault(mailtest.FaultNone)
		before := len(g.srv.Messages())
		const dailyCap = 20
		var orders []string
		for i := 0; i < 14; i++ {
			o, _ := g.bcmPlace(fmt.Sprintf("mix%d-%s@buyers.example.test", i, tag))
			orders = append(orders, o)
		}
		w := g.worker(dailyCap)
		for pass := 0; pass < 4; pass++ {
			g.drain(w)
		}
		if got := g.sentToday(); got > dailyCap*60/100 {
			t.Errorf("buyer + merchant sends today %d exceed the share %d", got, dailyCap*60/100)
		}
		if got := len(g.srv.Messages()) - before; got > dailyCap*60/100 {
			t.Errorf("SMTP got %d mails in this subtest, share is %d", got, dailyCap*60/100)
		}
		_ = orders
	})

	t.Run("per store: at most 30 buyer mails in a rolling hour", func(t *testing.T) {
		g.setMerchantMail(false)
		mustExec(t, g.p.f.owner, `DELETE FROM notify.outbox`)
		const orders = 7
		var ids []string
		for i := 0; i < orders; i++ {
			o, _ := g.bcmPlace(fmt.Sprintf("hour%d-%s@buyers.example.test", i, tag))
			ids = append(ids, o)
		}
		// fixture: a real order cannot reach every kind, so the missing kinds are enqueued through the same internal helper the triggers call
		for i := 0; i < 6; i++ {
			for _, k := range []string{"paid", "shipped", "cancelled", "refunded"} {
				mustExec(t, g.p.f.owner, `SELECT notify.enqueue($1,$2,$3,$4)`, g.tenant(), g.store(), ids[i], k)
			}
		}
		// 6 orders x 5 kinds + the 7th order's placed row = 31 buyer rows
		if n := g.count(`SELECT count(*) FROM notify.outbox WHERE store_id=$1 AND kind<>'merchant_new' AND state='PENDING'`, g.store()); n != 31 {
			t.Fatalf("fixture: %d buyer rows pending, want 31", n)
		}
		w := g.worker(100000)
		for pass := 0; pass < 4; pass++ {
			g.drain(w)
		}
		if n := g.count(`SELECT count(*) FROM notify.outbox WHERE store_id=$1 AND kind<>'merchant_new' AND state='SENT'`, g.store()); n != 30 {
			t.Errorf("%d buyer mails left in one hour, want exactly 30", n)
		}
		if n := g.count(`SELECT count(*) FROM notify.outbox WHERE store_id=$1 AND kind<>'merchant_new' AND state='PENDING' AND skip_reason IS NULL`, g.store()); n != 1 {
			t.Errorf("the 31st row must wait PENDING, %d do", n)
		}
		// an hour later it goes
		mustExec(t, g.p.f.owner, `UPDATE notify.outbox SET claimed_at=clock_timestamp()-interval '61 minutes' WHERE state='SENT'`)
		g.drain(w)
		if n := g.count(`SELECT count(*) FROM notify.outbox WHERE store_id=$1 AND kind<>'merchant_new' AND state='SENT'`, g.store()); n != 31 {
			t.Errorf("after the hour %d rows are SENT, want 31", n)
		}
		_ = ctx
	})
}

func TestBuyerCommsGateMerchantBatch(t *testing.T) {
	g := bgBank(t)
	tag := t04Tag()
	f := g.p.f
	w := g.worker(100000)
	hash := "$argon2id$v=19$m=4096,t=1,p=1$" + strings.Repeat("A", 22) + "$" + strings.Repeat("A", 43)
	// recipients per §E6: owners with a verified password e-mail (the schema cannot hold an unverified one); an admin must not be mailed.
	second := randomUUID()
	mustExec(t, f.owner, `INSERT INTO identity.principals(id) VALUES($1)`, second)
	mustExec(t, f.owner, `INSERT INTO identity.memberships(tenant_id,principal_id) VALUES($1,$2)`, g.tenant(), second)
	secondAddr := "owner2-" + tag + "@merchant.example.test"
	mustExec(t, f.owner, `INSERT INTO identity.password_credentials(principal_id,email,password_hash,email_verified_at) VALUES($1,$2,$3,clock_timestamp())`, second, secondAddr, hash)
	mustExec(t, f.owner, `INSERT INTO identity.store_staff(tenant_id,store_id,principal_id,role) VALUES($1,$2,$3,'owner')`, g.tenant(), g.store(), second)
	fourth := randomUUID()
	adminAddr := "admin-" + tag + "@merchant.example.test"
	mustExec(t, f.owner, `INSERT INTO identity.principals(id) VALUES($1)`, fourth)
	mustExec(t, f.owner, `INSERT INTO identity.memberships(tenant_id,principal_id) VALUES($1,$2)`, g.tenant(), fourth)
	mustExec(t, f.owner, `INSERT INTO identity.password_credentials(principal_id,email,password_hash,email_verified_at) VALUES($1,$2,$3,clock_timestamp())`, fourth, adminAddr, hash)
	mustExec(t, f.owner, `INSERT INTO identity.store_staff(tenant_id,store_id,principal_id,role) VALUES($1,$2,$3,'admin')`, g.tenant(), g.store(), fourth)

	buyerEmail := func(i int) string { return fmt.Sprintf("m%d-%s@buyers.example.test", i, tag) }
	var orders []string
	place := func(i int) string { o, _ := g.bcmPlace(buyerEmail(i)); orders = append(orders, o); return o }
	merchantMails := func() int { return len(g.mailsTo(g.owner)) }

	o1, o2, o3 := place(1), place(2), place(3)
	// two workers at once: still one mail per owner
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); g.drain(w) }()
	}
	wg.Wait()
	if merchantMails() != 1 || len(g.mailsTo(secondAddr)) != 1 {
		t.Fatalf("first batch: owner mails %d, second owner mails %d, want 1 and 1", merchantMails(), len(g.mailsTo(secondAddr)))
	}
	if len(g.mailsTo(adminAddr)) != 0 {
		t.Errorf("an admin (not an owner) was mailed")
	}
	body := g.mailsTo(g.owner)[0]
	for _, o := range []string{o1, o2, o3} {
		if !strings.Contains(body.Text, notify.OrderNumber(o)) {
			t.Errorf("batch lacks %s", notify.OrderNumber(o))
		}
	}
	for i := 1; i <= 3; i++ {
		if strings.Contains(body.Text+body.HTML+body.Subject, buyerEmail(i)) {
			t.Errorf("merchant mail leaks buyer email %s", buyerEmail(i))
		}
	}
	if !strings.Contains(body.Subject+body.Text, "3") {
		t.Errorf("batch does not say how many orders: %q", body.Subject)
	}

	t.Run("orders inside the 5 minute window wait, then go out as one batch", func(t *testing.T) {
		o4, o5 := place(4), place(5)
		for i := 0; i < 3; i++ {
			g.drain(w)
		}
		if merchantMails() != 1 {
			t.Fatalf("a second merchant mail inside 5 minutes (%d)", merchantMails())
		}
		if s := g.states(o4)["merchant_new"]; s != "PENDING" {
			t.Errorf("held row is %s", s)
		}
		mustExec(t, f.owner, `UPDATE notify.outbox SET claimed_at=clock_timestamp()-interval '301 seconds' WHERE kind='merchant_new' AND state='SENT'`)
		g.drain(w)
		if merchantMails() != 2 {
			t.Fatalf("after the window %d merchant mails, want 2", merchantMails())
		}
		last := g.mailsTo(g.owner)[1]
		if !strings.Contains(last.Text, notify.OrderNumber(o4)) || !strings.Contains(last.Text, notify.OrderNumber(o5)) || strings.Contains(last.Text, notify.OrderNumber(o1)) {
			t.Errorf("second batch must hold exactly the new orders:\n%s", last.Text)
		}
	})

	t.Run("a confirmed transfer does not announce the same order to the merchant again", func(t *testing.T) {
		order, b := g.bcmPlace(buyerEmail(6))
		if c := g.count(`SELECT count(*) FROM notify.outbox WHERE order_id=$1 AND kind='merchant_new'`, order); c != 1 {
			t.Fatalf("merchant_new rows at placement: %d", c)
		}
		if res := b.cofProof(order, t04Key("bgm-proof"), "12345", totalOf(t, g.tcvEnv, order)); res.status != 200 {
			t.Fatalf("proof %d", res.status)
		}
		if st, out := g.cofDecide(g.token(), order, "confirm", t04Key("bgm-confirm"), `{}`); st != 200 {
			t.Fatalf("confirm %d %v", st, out)
		}
		if c := g.count(`SELECT count(*) FROM notify.outbox WHERE order_id=$1 AND kind='merchant_new'`, order); c != 1 {
			t.Errorf("merchant_new rows after confirm: %d, want still 1", c)
		}
	})

	t.Run("opt-out through the real route skips merchant mail; turning it on again does not resurrect skipped rows", func(t *testing.T) {
		mustExec(t, f.owner, `UPDATE notify.outbox SET claimed_at=clock_timestamp()-interval '10 minutes' WHERE kind='merchant_new' AND state='SENT'`)
		g.drain(w)
		before := merchantMails()
		g.setMerchantMail(false)
		oOff, _ := g.bcmPlace(buyerEmail(7))
		for i := 0; i < 3; i++ {
			g.drain(w)
		}
		if s, _, r, _ := g.bcmRow(oOff, "merchant_new"); s != "SKIPPED" || r != "opted_out" {
			t.Fatalf("opted-out row: %s/%s", s, r)
		}
		if merchantMails() != before {
			t.Errorf("merchant mailed while opted out")
		}
		if len(g.mailsTo(buyerEmail(7))) != 1 {
			t.Errorf("the buyer mail must be unaffected by the merchant opt-out")
		}
		g.setMerchantMail(true)
		mustExec(t, f.owner, `UPDATE notify.outbox SET claimed_at=clock_timestamp()-interval '10 minutes' WHERE kind='merchant_new' AND state='SENT'`)
		g.drain(w)
		if merchantMails() != before {
			t.Errorf("skipped rows were mailed after the opt-in")
		}
		oOn, _ := g.bcmPlace(buyerEmail(8))
		g.drain(w)
		if merchantMails() != before+1 || !strings.Contains(g.mailsTo(g.owner)[before].Text, notify.OrderNumber(oOn)) {
			t.Errorf("after opting in again the next order must be mailed (mails %d, want %d)", merchantMails(), before+1)
		}
	})
}

func TestBuyerCommsGateErasure(t *testing.T) {
	g := bgBank(t)
	g.setMerchantMail(false)
	tag := t04Tag()
	ctx := context.Background()
	w := g.worker(100000)
	gone, keep := "erase-"+tag+"@buyers.example.test", "keep-"+tag+"@buyers.example.test"
	oGone, bGone := g.bcmPlace(gone)
	oKeep, _ := g.bcmPlace(keep)
	g.drain(w)
	for _, o := range []string{oGone, oKeep} {
		if _, _, _, h := g.bcmRow(o, "placed"); !h {
			t.Fatalf("a sent row keeps its recipient hash until erasure")
		}
	}
	// the hold ends (expiry enqueues a cancelled mail that has not been sent yet), then the buyer erases through the real privacy route
	g.cofAge(oGone, 80)
	if d, _ := g.cofExpire(oGone); d != "EXPIRED" {
		t.Fatalf("expire: %s", d)
	}
	g.cofAge(oKeep, 80)
	if d, _ := g.cofExpire(oKeep); d != "EXPIRED" {
		t.Fatalf("expire: %s", d)
	}
	if s := g.states(oGone)["cancelled"]; s != "PENDING" {
		t.Fatalf("cancelled before erasure: %s", s)
	}
	er := bGone.req("POST", "/v1/buyer/privacy/erasure", t04Key("bg-erase"), map[string]any{"confirm": "ERASE"}, nil)
	if er.status != 200 {
		t.Fatalf("erasure: %d %s", er.status, er.body)
	}
	if s, _, _, h := g.bcmRow(oGone, "placed"); s != "SENT" || h {
		t.Errorf("erased order's sent row: state %s hash present %v (want SENT, no hash)", s, h)
	}
	if s, _, r, h := g.bcmRow(oGone, "cancelled"); s != "SKIPPED" || r != "erased" || h {
		t.Errorf("erased order's pending row: %s/%s hash %v", s, r, h)
	}
	if s, _, _, h := g.bcmRow(oKeep, "placed"); s != "SENT" || !h {
		t.Errorf("another buyer's row must be untouched: %s %v", s, h)
	}
	before := len(g.srv.Messages())
	g.drain(w)
	got := g.srv.Messages()[before:]
	if len(got) != 1 || !strings.EqualFold(got[0].To, keep) {
		t.Errorf("after erasure exactly the other buyer's cancelled mail goes out, got %d (%v)", len(got), got)
	}
	// nothing in the notify schema can identify the address or hold a body (recipient is hashed, never stored)
	rows, err := g.p.f.owner.Query(ctx, `SELECT row_to_json(o)::text FROM notify.outbox o`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var j string
		if err := rows.Scan(&j); err != nil {
			t.Fatal(err)
		}
		for _, bad := range []string{gone, keep, "@", "Taiwan Bank", "123-456-7890", "<html", "Subject"} {
			if strings.Contains(strings.ToLower(j), strings.ToLower(bad)) {
				t.Errorf("outbox row stores %q: %s", bad, j)
			}
		}
	}
	// the buyer lookup cannot reach an erased owner either (E5: erased owner = the same refusal)
	token := bgToken()
	r := g.bh.request(t, "POST", "/v1/buyer/orders/lookup", token, "", map[string]string{"order_ref": notify.OrderNumber(oGone), "contact": gone},
		func(r *http.Request) { r.Header.Set("X-Commerce-Client-IP", "203.0.113.77") })
	if r.status != 404 {
		t.Errorf("lookup of an erased owner's order: %d %s", r.status, r.body)
	}
	_ = time.Second
}

// BG03d: one store over its hourly cap must not hold up the mail of other stores (§E3: the cap exists so one store cannot hurt the others).
// Store A has 55 buyer rows pending (11 real orders x 5 kinds, the extra kinds through the same internal helper the triggers call): 30 may go in the
// hour, 25 stay PENDING at the head of the queue. Store B then places one real order; its placed mail must still leave in the same hour.
func TestBuyerCommsGateStoreCapDoesNotStarveOtherStores(t *testing.T) {
	a := bgBank(t)
	b := bgBank(t) // a second store (own tenant) in the same database and the same outbox
	a.setMerchantMail(false)
	b.setMerchantMail(false)
	tag := t04Tag()
	w := a.worker(100000) // one worker pool for the whole outbox, as production
	for i := 0; i < 11; i++ {
		o, _ := a.bcmPlace(fmt.Sprintf("flash%d-%s@buyers.example.test", i, tag))
		for _, k := range []string{"paid", "shipped", "cancelled", "refunded"} {
			mustExec(t, a.p.f.owner, `SELECT notify.enqueue($1,$2,$3,$4)`, a.tenant(), a.store(), o, k)
		}
	}
	for pass := 0; pass < 6; pass++ {
		a.drain(w)
	}
	if n := a.count(`SELECT count(*) FROM notify.outbox WHERE store_id=$1 AND kind<>'merchant_new' AND state='SENT'`, a.store()); n != 30 {
		t.Fatalf("fixture: store A sent %d in the hour, want 30", n)
	}
	if n := a.count(`SELECT count(*) FROM notify.outbox WHERE store_id=$1 AND kind<>'merchant_new' AND state='PENDING'`, a.store()); n != 25 {
		t.Fatalf("fixture: store A has %d rows stuck over the cap, want 25", n)
	}
	email := "late-" + tag + "@buyers.example.test"
	order, _ := b.bcmPlace(email)
	for pass := 0; pass < 6; pass++ {
		a.drain(w)
	}
	if s := b.states(order)["placed"]; s != "SENT" {
		t.Errorf("store B's placed mail is %s while store A is over its hourly cap: a capped store starves the others (claim_batch takes the oldest 20 PENDING rows, all of them store A's)", s)
	}
	if got := a.mailsTo(email); len(got) != 1 {
		t.Errorf("store B's buyer got %d mails, want 1", len(got))
	}
}
