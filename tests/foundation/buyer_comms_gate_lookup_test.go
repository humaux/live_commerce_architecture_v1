package foundation_test

// BG07-BG08: independent gate for contracts/storefront-v2.md §E5 (guest order lookup and the VIEW-ONLY session it issues), real buyer HTTP handler
// (internal/buyerhttp) over the real PG definers; written from the contract. Evidence tier REAL_PG (HTTP handler in-process, no BFF, no browser:
// the browser half is TestBrowserBuyerComms).
//   BG07 TestBuyerCommsGateLookup      identical refusal bytes for every mismatch class (unknown / wrong email / wrong phone / another buyer's email /
//                                      other store / erased owner), normalisation of ref + email + phone, 10 per IP / 5 per ref / 200 per store per
//                                      10 min counted BEFORE the lookup (a valid hit is throttled too), 429 + Retry-After, windows reset, mismatch
//                                      timing indistinguishable from unknown (medians, disclosed tolerance), a reused bearer cannot be hijacked
//   BG08 TestBuyerCommsGateViewOnly    the issued session reads its own order / payment status / bank-transfer instructions and NOTHING else: a
//                                      route x method matrix is 403, and the database fingerprint of every commerce schema is unchanged by it
// Disclosed owner-pool fixtures: DELETE FROM checkout.lookup_throttle between throttle scenarios and a 11-minute shift of window_start (window reset
// without sleeping), one extra published store + domain for the "other store" case.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"livecommerce/internal/notify"
)

var bgIPSeq atomic.Int64

// bgIP is a unique client address per call (the per-IP bucket is global in the shared database).
func bgIP() string {
	n := bgIPSeq.Add(1) + time.Now().UnixNano()%200000
	return fmt.Sprintf("100.%d.%d.%d", (n>>16)&255, (n>>8)&255, n&255)
}

func bgToken() string { return randomToken() }

func (g *bgEnv) lookupAt(origin, token, ref, contact, ip string) bhResponse {
	return g.bh.request(g.t, "POST", "/v1/buyer/orders/lookup", token, "", map[string]string{"order_ref": ref, "contact": contact},
		func(r *http.Request) {
			r.Header.Set("X-Commerce-Client-IP", ip)
			if origin != "" {
				r.Header.Set("X-Commerce-Storefront-Origin", origin)
			}
		})
}

func (g *bgEnv) lookup(ref, contact string) bhResponse {
	return g.lookupAt("", bgToken(), ref, contact, bgIP())
}

func (g *bgEnv) phoneOf(order string) string {
	g.t.Helper()
	var phone string
	if err := g.p.f.owner.QueryRow(context.Background(), `SELECT d.phone FROM checkout.orders o JOIN storefront.destination_snapshots d ON d.tenant_id=o.tenant_id AND d.store_id=o.store_id AND d.owner_id=o.owner_id AND d.id=o.destination_id WHERE o.id=$1`, order).Scan(&phone); err != nil {
		g.t.Fatal(err)
	}
	return phone
}

var bgRequestID = regexp.MustCompile(`"request_id":"[0-9a-f]+"`)

// bgRefusal is everything a caller can observe of a refusal except the per-request id (which is random by design).
// bgLookupWindow mirrors internal/buyerhttp lookupWindow (seconds); the limits subtest uses it to bound Retry-After.
const bgLookupWindow = 600

func bgRefusal(r bhResponse) string {
	return fmt.Sprintf("%d|%s|ct=%s|cc=%s|ra=%s|%s", r.status, bgRequestID.ReplaceAllString(strings.TrimSpace(string(r.body)), `"request_id":"-"`), r.header.Get("Content-Type"), r.header.Get("Cache-Control"), r.header.Get("Retry-After"), r.header.Get("Set-Cookie"))
}

// rawRequest is bh.request without its header assertions (the matrix reports a missing header as a finding instead of aborting).
func (g *bgEnv) rawRequest(method, path, token, key string, input any) bhResponse {
	g.t.Helper()
	var data []byte
	if input != nil {
		data, _ = json.Marshal(input)
	}
	r, err := http.NewRequest(method, g.bh.server.URL+path, bytes.NewReader(data))
	if err != nil {
		g.t.Fatal(err)
	}
	r.Header.Set("X-Commerce-Buyer-BFF-Key", g.bh.key)
	r.Header.Set("X-Commerce-Storefront-Origin", g.bh.origin)
	r.Header.Set("X-Commerce-Client-IP", bgIP())
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	if input != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	res, err := (&http.Client{Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Do(r)
	if err != nil {
		g.t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	return bhResponse{res.StatusCode, res.Header, body}
}

func bgMedian(d []time.Duration) time.Duration {
	s := append([]time.Duration(nil), d...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	return s[len(s)/2]
}

func TestBuyerCommsGateLookup(t *testing.T) {
	g := bgBank(t)
	ctx := context.Background()
	f := g.p.f
	tag := t04Tag()
	email := "Guest-" + tag + "@Buyers.Example.Test"
	order, _ := g.bcmPlace(email)
	other, _ := g.bcmPlace("other-" + tag + "@buyers.example.test")
	number, phone := notify.OrderNumber(order), g.phoneOf(order)
	digits := strings.TrimLeft(strings.TrimPrefix(strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, phone), "886"), "0")
	reset := func() { mustExec(t, f.owner, `DELETE FROM checkout.lookup_throttle`) }

	// a second published store, the order of the first store is looked up through it
	store2 := randomUUID()
	mustExec(t, f.owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'second store','TWD')`, g.tenant(), store2)
	origin2 := "https://second-" + tag + ".example"
	bhPublish(t, g.p.bcHarness, origin2, g.tenant(), store2)

	t.Run("a match is normalised (ref case/dashes/spaces, email case/space, phone formats) and returns only the order id", func(t *testing.T) {
		reset()
		for i, tc := range []struct{ ref, contact string }{
			{number, email},
			{strings.ToLower(strings.ReplaceAll(number, "-", " ")), "  " + strings.ToLower(email) + " "},
			{strings.ReplaceAll(number, "-", ""), phone},
			{order, "+886" + digits},
			{order, "0" + digits},
			{order, digits},
		} {
			r := g.lookup(tc.ref, tc.contact)
			if r.status != 200 {
				t.Fatalf("case %d (%q,%q): %d %s", i, tc.ref, tc.contact, r.status, r.body)
			}
			var out map[string]any
			if err := json.Unmarshal(r.body, &out); err != nil || len(out) != 1 || out["order_id"] != order {
				t.Fatalf("case %d: body must be exactly {order_id}: %s", i, r.body)
			}
		}
	})

	t.Run("every mismatch class is the same refusal, byte for byte", func(t *testing.T) {
		reset()
		// erased owner: a third order whose buyer erased through the real privacy route
		erasedOrder, eb := g.bcmPlace("erased-" + tag + "@buyers.example.test")
		g.cofAge(erasedOrder, 80)
		if d, _ := g.cofExpire(erasedOrder); d != "EXPIRED" {
			t.Fatalf("expire: %s", d)
		}
		if r := eb.req("POST", "/v1/buyer/privacy/erasure", t04Key("bg-lk-erase"), map[string]any{"confirm": "ERASE"}, nil); r.status != 200 {
			t.Fatalf("erasure: %d %s", r.status, r.body)
		}
		cases := []struct {
			name   string
			origin string
			ref    string
			cont   string
		}{
			{"unknown ref", "", "FFFF-FFFF-FFFF", email},
			{"unknown full id", "", "ffffffff-ffff-4fff-bfff-ffffffffffff", email},
			{"wrong email", "", number, "nobody@buyers.example.test"},
			{"wrong phone", "", number, "0911111119"},
			{"another buyer's email", "", number, "other-" + tag + "@buyers.example.test"},
			{"other store origin, right credentials", origin2, number, email},
			{"other store origin, full id", origin2, order, email},
			{"erased owner, old email", "", notify.OrderNumber(erasedOrder), "erased-" + tag + "@buyers.example.test"},
			{"erased owner, phone", "", notify.OrderNumber(erasedOrder), g.phoneOf(erasedOrder)},
			{"unknown ref, phone", "", "0000-0000-0000", phone},
		}
		var want string
		for i, c := range cases {
			// one fresh IP and a fresh token per probe, so nothing but the mismatch can differ
			r := g.lookupAt(c.origin, bgToken(), c.ref, c.cont, bgIP())
			got := bgRefusal(r)
			if r.status != 404 {
				t.Errorf("%s: %d %s (want 404)", c.name, r.status, r.body)
			}
			if i == 0 {
				want = got
			}
			if got != want {
				t.Errorf("%s differs from the unknown-order refusal:\n got %s\nwant %s", c.name, got, want)
			}
		}
		if !strings.Contains(want, "not_found") {
			t.Errorf("refusal code: %s", want)
		}
	})

	t.Run("refusals register no session", func(t *testing.T) {
		reset()
		before := g.count(`SELECT count(*) FROM buyer.capability_sessions`)
		for i := 0; i < 4; i++ {
			g.lookup(number, fmt.Sprintf("wrong%d@buyers.example.test", i))
		}
		if after := g.count(`SELECT count(*) FROM buyer.capability_sessions`); after != before {
			t.Errorf("sessions %d -> %d after refusals", before, after)
		}
	})

	t.Run("limits: 5 per order ref, counted before the lookup, 429 with Retry-After and an identical body for real and unknown refs", func(t *testing.T) {
		reset()
		for i := 1; i <= 5; i++ {
			if r := g.lookup(number, fmt.Sprintf("guess%d@buyers.example.test", i)); r.status != 404 {
				t.Fatalf("attempt %d: %d", i, r.status)
			}
		}
		real := g.lookup(number, email) // CORRECT credentials, 6th hit on the ref
		if real.status != 429 || real.header.Get("Retry-After") == "" {
			t.Fatalf("6th attempt with valid credentials: %d retry-after %q (the limit is counted before any lookup)", real.status, real.header.Get("Retry-After"))
		}
		if secs := real.header.Get("Retry-After"); secs == "0" || len(secs) > 3 {
			t.Errorf("Retry-After %q is not a sane delay within a 10 minute window", secs)
		}
		for i := 1; i <= 5; i++ {
			g.lookup("EEEE-EEEE-EEEE", "x@buyers.example.test")
		}
		unknown := g.lookup("EEEE-EEEE-EEEE", "x@buyers.example.test")
		// Retry-After is wall-clock aligned (internal/buyerhttp/lookup.go: lookupWindow - now%lookupWindow), so it never depends on the
		// ref; the two 429s are taken up to a second apart, so their values may differ by one (flake: ra=31 vs ra=32, CI 37475164874).
		// Everything else must be byte-identical.
		noRA := func(r bhResponse) string {
			return regexp.MustCompile(`\|ra=\d+\|`).ReplaceAllString(bgRefusal(r), "|ra=*|")
		}
		raU, _ := strconv.Atoi(unknown.header.Get("Retry-After"))
		raR, _ := strconv.Atoi(real.header.Get("Retry-After"))
		if src, err := os.ReadFile("../../internal/buyerhttp/lookup.go"); err != nil || !regexp.MustCompile(`lookupWindow\s*=\s*`+strconv.Itoa(bgLookupWindow)+`\b`).Match(src) {
			t.Fatalf("bgLookupWindow %d must equal internal/buyerhttp lookupWindow (%v)", bgLookupWindow, err)
		}
		// The window rolls over at most once between the two requests: real=1 -> unknown=window is the same one-second step.
		step := (raR - raU + bgLookupWindow) % bgLookupWindow
		if unknown.status != 429 || noRA(unknown) != noRA(real) || raU < 1 || raR < 1 || raU > bgLookupWindow || raR > bgLookupWindow || (step != 0 && step != 1) {
			t.Errorf("the 429 for an unknown ref must equal the 429 for a real one:\n%s\n%s", bgRefusal(unknown), bgRefusal(real))
		}
		// another ref is untouched by this one
		if r := g.lookup(strings.ToUpper(notify.OrderNumber(other)), "other-"+tag+"@buyers.example.test"); r.status != 200 {
			t.Errorf("an unrelated ref is throttled: %d %s", r.status, r.body)
		}
		// fixed 10-minute window: shifting the window back opens a new one
		mustExec(t, f.owner, `UPDATE checkout.lookup_throttle SET window_start=window_start-interval '11 minutes'`)
		if r := g.lookup(number, email); r.status != 200 {
			t.Errorf("after the window: %d %s", r.status, r.body)
		}
	})

	t.Run("limits: 10 per client IP, counted before the lookup", func(t *testing.T) {
		reset()
		ip := bgIP()
		for i := 0; i < 10; i++ {
			if r := g.lookupAt("", bgToken(), fmt.Sprintf("%04X-0000-0000", i+1), "x@buyers.example.test", ip); r.status != 404 {
				t.Fatalf("hit %d from one IP: %d", i+1, r.status)
			}
		}
		if r := g.lookupAt("", bgToken(), number, email, ip); r.status != 429 {
			t.Errorf("11th hit from one IP with valid credentials: %d %s", r.status, r.body)
		}
		if r := g.lookupAt("", bgToken(), number, email, bgIP()); r.status != 200 {
			t.Errorf("another IP must be unaffected: %d %s", r.status, r.body)
		}
	})

	t.Run("limits: 200 per store in 10 minutes across any number of IPs and refs", func(t *testing.T) {
		reset()
		for i := 0; i < 200; i++ {
			if r := g.lookupAt("", bgToken(), fmt.Sprintf("%04X-%04X-0001", i, i+7), "x@buyers.example.test", bgIP()); r.status != 404 {
				t.Fatalf("hit %d of 200: %d %s", i+1, r.status, r.body)
			}
		}
		r := g.lookupAt("", bgToken(), number, email, bgIP())
		if r.status != 429 {
			t.Fatalf("201st hit on the store with valid credentials: %d %s", r.status, r.body)
		}
		// another store has its own bucket
		if r := g.lookupAt(origin2, bgToken(), "AAAA-AAAA-AAAA", "x@buyers.example.test", bgIP()); r.status != 404 {
			t.Errorf("the second store shares the first store's bucket: %d", r.status)
		}
		reset()
	})

	t.Run("input and auth edges: no bearer 401, missing field 422, GET 405", func(t *testing.T) {
		reset()
		if r := g.bh.request(t, "POST", "/v1/buyer/orders/lookup", "", "", map[string]string{"order_ref": number, "contact": email}, nil); r.status != 401 {
			t.Errorf("no bearer: %d", r.status)
		}
		for _, body := range []map[string]string{{"order_ref": number}, {"contact": email}, {"order_ref": "", "contact": ""}, {"order_ref": "zzzz", "contact": email}} {
			if r := g.bh.request(t, "POST", "/v1/buyer/orders/lookup", bgToken(), "", body, func(r *http.Request) { r.Header.Set("X-Commerce-Client-IP", bgIP()) }); r.status != 422 && r.status != 400 {
				t.Errorf("%v: %d %s", body, r.status, r.body)
			}
		}
		if r := g.bh.request(t, "GET", "/v1/buyer/orders/lookup", bgToken(), "", nil, nil); r.status != 405 {
			t.Errorf("GET: %d", r.status)
		}
	})

	t.Run("timing: unknown order, wrong email and wrong phone cost the same (medians of 60, tolerance max(35%, 2 ms))", func(t *testing.T) {
		reset()
		kinds := []struct {
			name string
			ref  func(i int) string
			cont string
		}{
			{"unknown ref", func(i int) string { return fmt.Sprintf("%04X-%04X-%04X", 0xA000+i, 0xB000+i, 0xC000+i) }, "x@buyers.example.test"},
			{"real ref, wrong email", func(int) string { return number }, "x@buyers.example.test"},
			{"real ref, wrong phone", func(int) string { return number }, "0911111119"},
		}
		samples := make([][]time.Duration, len(kinds))
		for round := 0; round < 60; round++ {
			if round%2 == 0 {
				reset() // fixture: the per-ref bucket would otherwise cap the real-ref probes at 5 (two probes per round)
			}
			for k, kind := range kinds {
				start := time.Now()
				r := g.lookup(kind.ref(round), kind.cont)
				d := time.Since(start)
				if r.status != 404 {
					t.Fatalf("%s: %d", kind.name, r.status)
				}
				if round >= 5 { // warm-up
					samples[k] = append(samples[k], d)
				}
			}
		}
		base := bgMedian(samples[0])
		for k := 1; k < len(kinds); k++ {
			m := bgMedian(samples[k])
			diff := m - base
			if diff < 0 {
				diff = -diff
			}
			tol := base * 35 / 100
			if tol < 2*time.Millisecond {
				tol = 2 * time.Millisecond
			}
			t.Logf("median %s = %v vs unknown ref %v (diff %v, tolerance %v)", kinds[k].name, m, base, diff, tol)
			if diff > tol {
				t.Errorf("%s median %v differs from unknown-ref median %v by more than %v: timing reveals that the order exists", kinds[k].name, m, base, tol)
			}
		}
	})
	_ = ctx
}

// BG07b: a lookup that arrives with a bearer which already belongs to a buyer (a buggy or hostile client: the BFF always mints a fresh one) must not
// rebind that buyer's session and must not answer 5xx.
func TestBuyerCommsGateLookupReusedBearer(t *testing.T) {
	g := bgBank(t)
	tag := t04Tag()
	email := "Guest-" + tag + "@Buyers.Example.Test"
	order, _ := g.bcmPlace(email)
	number := notify.OrderNumber(order)
	victim, vb := g.bcmPlace("victim-" + tag + "@buyers.example.test")
	r := g.lookupAt("", vb.cap.Token, number, email, bgIP())
	if r.status >= 500 {
		t.Errorf("lookup with an existing buyer token answered %d %s", r.status, r.body)
	}
	if own := vb.req("GET", "/v1/buyer/orders/"+victim, "", nil, nil); own.status != 200 {
		t.Errorf("the victim's own capability lost its right to its own order: %d %s", own.status, own.body)
	}
	if list := vb.req("GET", "/v1/buyer/orders", "", nil, nil); list.status != 200 {
		t.Errorf("the victim's own capability lost its right to list its orders: %d %s", list.status, list.body)
	}
	if foreign := vb.req("GET", "/v1/buyer/orders/"+order, "", nil, nil); foreign.status == 200 {
		t.Errorf("the victim's capability was extended to the looked-up order: %d", foreign.status)
	}
}

// fingerprint hashes every row of every commerce table a buyer write could touch (not the lookup throttle, not the session table).
func (g *bgEnv) fingerprint() string {
	g.t.Helper()
	ctx := context.Background()
	rows, err := g.p.f.owner.Query(ctx, `SELECT table_schema||'.'||table_name FROM information_schema.tables
	  WHERE table_type='BASE TABLE' AND table_schema IN ('checkout','customers','storefront','payments','inventory','notify','ops') AND table_name NOT IN ('lookup_throttle')
	  ORDER BY 1`)
	if err != nil {
		g.t.Fatal(err)
	}
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			g.t.Fatal(err)
		}
		names = append(names, n)
	}
	rows.Close()
	var out bytes.Buffer
	for _, n := range names {
		var h string
		if err := g.p.f.owner.QueryRow(ctx, `SELECT coalesce(md5(string_agg(t::text,'' ORDER BY t::text)),'-') FROM `+n+` t`).Scan(&h); err != nil {
			g.t.Fatalf("%s: %v", n, err)
		}
		fmt.Fprintf(&out, "%s=%s\n", n, h)
	}
	return out.String()
}

func TestBuyerCommsGateViewOnly(t *testing.T) {
	g := bgBank(t)
	tag := t04Tag()
	email := "view-" + tag + "@buyers.example.test"
	mine, full := g.bcmPlace(email)
	theirs, theirBuyer := g.bcmPlace("neighbour-" + tag + "@buyers.example.test")
	guest := bgToken()
	if r := g.lookupAt("", guest, notify.OrderNumber(mine), email, bgIP()); r.status != 200 {
		t.Fatalf("lookup: %d %s", r.status, r.body)
	}
	call := func(method, path string, body any) bhResponse {
		key := ""
		if method != "GET" && method != "DELETE" {
			key = t04Key("bg-view")
		}
		if strings.HasSuffix(path, "/handoff") || strings.HasSuffix(path, "/verify") || strings.HasSuffix(path, "/refresh") || strings.HasSuffix(path, "/cancel") {
			key = ""
		}
		return g.rawRequest(method, path, guest, key, body)
	}
	own := "/v1/buyer/orders/" + mine
	foreign := "/v1/buyer/orders/" + theirs

	// allowed: reading its one order, its payment status, its bank-transfer instructions, its session
	for _, p := range []string{"/v1/buyer/session", own, own + "/bank-transfer"} {
		if r := call("GET", p, nil); r.status != 200 {
			t.Errorf("GET %s: %d %s (a view-only session must read this)", p, r.status, r.body)
		}
	}
	if r := call("GET", own+"/payment", nil); r.status == 403 || r.status >= 500 {
		t.Errorf("GET own payment status: %d %s", r.status, r.body)
	}
	if r := call("GET", own, nil); !strings.Contains(string(r.body), mine) {
		t.Errorf("own order body lacks its id: %s", r.body)
	}
	bank := call("GET", own+"/bank-transfer", nil)
	for _, want := range []string{"Taiwan Bank", "123-456-7890"} {
		if !strings.Contains(string(bank.body), want) {
			t.Errorf("bank-transfer instructions lack %q: %s", want, bank.body)
		}
	}

	before := g.fingerprint()
	proof := map[string]any{"last5": "12345", "amount_minor": 100, "paid_at": "2026-10-01T00:00:00Z"}
	// soft: a method that the route does not implement is 405 before the session is even classified; still closed, and it must change nothing
	soft := map[string]bool{"POST " + own: true, "PUT " + own: true, "DELETE " + own: true, "PATCH " + own: true, "GET /v1/buyer/consents": true,
		"POST /v1/buyer/cart": true, "POST /v1/buyer/destination": true}
	closed := []struct {
		method, path string
		body         any
	}{
		// other orders and lists
		{"GET", "/v1/buyer/orders", nil}, {"GET", foreign, nil}, {"GET", foreign + "/payment", nil}, {"GET", foreign + "/bank-transfer", nil},
		{"PUT", foreign + "/bank-transfer/proof", proof},
		// writes on its own order
		{"PUT", own + "/bank-transfer/proof", proof}, {"POST", own + "/payment/prepare", map[string]any{"method_code": "stripe_checkout", "method_version": 1, "locale": "en"}},
		{"POST", own + "/payment/handoff", nil}, {"POST", own + "/payment/refresh", nil}, {"POST", own + "/payment/cancel", nil},
		{"POST", own, nil}, {"PUT", own, map[string]any{}}, {"DELETE", own, nil}, {"PATCH", own, map[string]any{}},
		// privacy, consent
		{"GET", "/v1/buyer/privacy", nil}, {"POST", "/v1/buyer/privacy/export", nil}, {"POST", "/v1/buyer/privacy/erasure", map[string]any{"confirm": "ERASE"}},
		{"PUT", "/v1/buyer/consents", map[string]any{"purpose": "ads_personalization", "channel": "web", "granted": true, "source": "settings"}},
		{"GET", "/v1/buyer/consents", nil},
		// cart, quote, destination, checkout, catalog, options
		{"GET", "/v1/buyer/cart", nil}, {"PUT", "/v1/buyer/cart", map[string]any{"items": []any{}}}, {"POST", "/v1/buyer/cart", map[string]any{}},
		{"POST", "/v1/buyer/quotes", map[string]any{}}, {"POST", "/v1/buyer/destination", map[string]any{}}, {"POST", "/v1/buyer/checkout", map[string]any{}},
		{"GET", "/v1/buyer/catalog", nil}, {"GET", "/v1/buyer/checkout-options", nil},
		// CVS and claims
		{"POST", "/v1/buyer/cvs-selections", map[string]any{}}, {"POST", "/v1/buyer/cvs-stores", map[string]any{}}, {"GET", "/v1/buyer/cvs-selections/" + randomUUID(), nil},
		{"POST", "/v1/buyer/cvs-selections/" + randomUUID() + "/verify", nil}, {"GET", "/v1/buyer/claim-link", nil}, {"POST", "/v1/buyer/claim-link/redeem", map[string]any{}},
	}
	for _, c := range closed {
		r := call(c.method, c.path, c.body)
		if r.status != 403 && !(soft[c.method+" "+c.path] && r.status == 405) {
			t.Errorf("%s %s with a view-only session: %d %s (want 403)", c.method, c.path, r.status, r.body)
		}
		if !strings.Contains(r.header.Get("Cache-Control"), "no-store") || r.header.Get("X-Content-Type-Options") != "nosniff" || len(r.header.Get("X-Request-ID")) != 32 {
			t.Errorf("%s %s: refusal lacks the safe response headers (Cache-Control %q nosniff %q request id %q)", c.method, c.path, r.header.Get("Cache-Control"), r.header.Get("X-Content-Type-Options"), r.header.Get("X-Request-ID"))
		}
	}
	if after := g.fingerprint(); after != before {
		t.Errorf("the refused view-only requests changed commerce data:\nbefore\n%s\nafter\n%s", before, after)
	}
	if n := g.count(`SELECT count(*) FROM customers.privacy_actions WHERE tenant_id=$1`, g.tenant()); n != 0 {
		t.Errorf("privacy actions started by a view-only session: %d", n)
	}

	// a guest session is not an elevation: the order's own checkout capability keeps every right and the neighbour's order stays closed to it
	if r := full.req("GET", "/v1/buyer/orders", "", nil, nil); r.status != 200 {
		t.Errorf("checkout capability lost its order list: %d", r.status)
	}
	if r := full.req("GET", foreign, "", nil, nil); r.status == 200 {
		t.Errorf("checkout capability of one buyer reads another buyer's order: %d", r.status)
	}
	if r := theirBuyer.req("GET", foreign, "", nil, nil); r.status != 200 {
		t.Errorf("the neighbour's own capability is broken: %d", r.status)
	}
	// bootstrap / retire / logout stay open to the guest session (not 403), and after logout the token is dead
	guest2 := bgToken()
	if r := g.lookupAt("", guest2, notify.OrderNumber(mine), email, bgIP()); r.status != 200 {
		t.Fatalf("second lookup: %d %s", r.status, r.body)
	}
	for _, p := range []string{"/v1/buyer/session/bootstrap", "/v1/buyer/session/retire"} {
		if r := g.bh.request(t, "POST", p, guest2, "", nil, nil); r.status == 403 {
			t.Errorf("POST %s is on the view-only allowlist, got 403 %s", p, r.body)
		}
	}
	if r := call("DELETE", "/v1/buyer/session", nil); r.status >= 400 {
		t.Errorf("guest logout: %d %s", r.status, r.body)
	}
	if r := call("GET", own, nil); r.status == 200 {
		t.Errorf("a retired guest session still reads the order: %d", r.status)
	}
}
