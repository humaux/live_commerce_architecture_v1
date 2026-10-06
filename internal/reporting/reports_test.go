// reports_test.go covers the DB-free half of the W6-02B reports: strict decoding of every report shape, drift refusal, the CSV formula guard,
// and the input refusals that happen before any database call. The money rules are REAL_PG (tests/foundation/reports_test.go).
//
// Purpose: red/green evidence for the Go side of contracts/reporting-v2.md.
// Depends on: products.go, channels.go, manual.go, funnel.go, report.go.
// Used by: go test ./internal/reporting.

package reporting

import (
	"context"
	"strings"
	"testing"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

const (
	rSKU  = "11111111-1111-4111-8111-111111111111"
	rProd = "22222222-2222-4222-8222-222222222222"
	rSess = "33333333-3333-4333-8333-333333333333"
)

func TestDecodeProductsStrict(t *testing.T) {
	ok := `{"from":"2026-09-01","to":"2026-09-30","timezone":"Asia/Taipei","truncated":false,"rows":[
	 {"sku_id":"` + rSKU + `","product_id":"` + rProd + `","code":"A","name":"=cmd","currency":"TWD","environment":"LIVE","units":2,"captured_minor":900,"refunded_minor":100,"net_minor":800,"offline_units":0,"offline_minor":0},
	 {"sku_id":"` + rProd + `","product_id":"` + rProd + `","code":"B","name":"b","currency":"TWD","environment":"LIVE","units":1,"captured_minor":100,"refunded_minor":0,"net_minor":100,"offline_units":1,"offline_minor":500}]}`
	rep, err := decodeProducts([]byte(ok), "2026-09-01", "2026-09-30")
	if err != nil || len(rep.Rows) != 2 || rep.Rows[0].NetMinor != 800 {
		t.Fatalf("valid report refused: %v %+v", err, rep)
	}
	for name, bad := range map[string]string{
		"unknown key":   strings.Replace(ok, `"truncated":false`, `"truncated":false,"extra":1`, 1),
		"net drift":     strings.Replace(ok, `"net_minor":800`, `"net_minor":801`, 1),
		"not sorted":    strings.Replace(ok, `"net_minor":100`, `"net_minor":900`, 1),
		"other range":   strings.Replace(ok, `"to":"2026-09-30"`, `"to":"2026-09-29"`, 1),
		"bad env":       strings.Replace(ok, `"environment":"LIVE"`, `"environment":"TEST"`, 1),
		"bad sku":       strings.Replace(ok, rSKU, "not-a-uuid", 1),
		"trailing data": ok + `{}`,
		"truncated lie": strings.Replace(ok, `"truncated":false`, `"truncated":true`, 1),
		"negative":      strings.Replace(ok, `"units":2`, `"units":-2`, 1),
	} {
		if _, err := decodeProducts([]byte(bad), "2026-09-01", "2026-09-30"); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestCSVFormulaGuardAndShapes(t *testing.T) {
	for in, want := range map[string]string{"=1+1": "'=1+1", " +x": "' +x", "@a": "'@a", "-1": "'-1", "\tx": "'\tx", "plain": "plain", "": ""} {
		if got := cell(in); got != want {
			t.Errorf("cell(%q)=%q want %q", in, got, want)
		}
	}
	out, err := writeCSV([]string{"a", "b"}, [][]string{{"1", cell("=HYPERLINK()")}})
	if err != nil || string(out) != "a,b\n1,'=HYPERLINK()\n" {
		t.Fatalf("csv: %q %v", out, err)
	}
	if rows := moneyRows(nil, func(m Money) []string { return []string{m.Environment, i64(m.NetMinor)} }); len(rows) != 1 || rows[0][1] != "0" {
		t.Fatalf("empty bucket must give one zero row: %v", rows)
	}
}

func TestValidMoney(t *testing.T) {
	good := []Money{{Environment: "LIVE", CapturedCount: 1, CapturedMinor: 500, RefundedMinor: 100, NetMinor: 400, OfflineCount: 1, OfflineMinor: 700},
		{Environment: "SANDBOX"}}
	if !validMoney(good) || validMoney([]Money{good[1], good[0]}) { // the database orders LIVE before SANDBOX
		t.Fatal("environment order not enforced")
	}
	for name, m := range map[string]Money{
		"net drift":        {Environment: "LIVE", CapturedCount: 1, CapturedMinor: 5, NetMinor: 4},
		"amount no count":  {Environment: "LIVE", CapturedMinor: 5, NetMinor: 5},
		"offline no count": {Environment: "LIVE", OfflineMinor: 5},
		"negative":         {Environment: "LIVE", CapturedCount: 1, CapturedMinor: -1, NetMinor: -1},
	} {
		if validMoney([]Money{m}) {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestFunnelAndBucketDecode(t *testing.T) {
	// Nested counts: a funnel that grows to the right is drift.
	head := `"from":"2026-09-01","to":"2026-09-30","timezone":"Asia/Taipei"`
	for name, tc := range map[string]struct {
		body string
		ok   bool
	}{
		"nested":        {`{` + head + `,"session_id":null,"claimed":5,"link_sent":4,"ordered":3,"paid":2,"ordered_without_link":1}`, true},
		"grows":         {`{` + head + `,"session_id":null,"claimed":5,"link_sent":4,"ordered":5,"paid":2,"ordered_without_link":0}`, false},
		"paid>ordered":  {`{` + head + `,"session_id":null,"claimed":5,"link_sent":4,"ordered":3,"paid":4,"ordered_without_link":0}`, false},
		"unlinked>rest": {`{` + head + `,"session_id":null,"claimed":5,"link_sent":4,"ordered":3,"paid":2,"ordered_without_link":2}`, false},
	} {
		var rep FunnelReport
		err := strict([]byte(tc.body), &rep)
		valid := err == nil && validHead(rep.From, rep.To, rep.Timezone, "2026-09-01", "2026-09-30") &&
			rep.Paid <= rep.Ordered && rep.Ordered <= rep.LinkSent && rep.LinkSent <= rep.Claimed && rep.LinkSent+rep.OrderedWithoutLink <= rep.Claimed
		if valid != tc.ok {
			t.Errorf("%s: valid=%v want %v", name, valid, tc.ok)
		}
	}
	if indexOf(Channels, "manual") != 3 || indexOf(Channels, "x") != -1 {
		t.Fatal("channel vocabulary")
	}
}

func TestReportsRefuseBadInputBeforeDatabase(t *testing.T) {
	ctx := context.Background()
	good := platform.Scope{TenantID: rSKU, StoreID: rProd, PrincipalID: rSess, Revision: 1}
	tok := strings.Repeat("a", 43)
	for name, fn := range map[string]func() error{
		"nil tx":      func() error { _, err := Products(ctx, nil, good, tok, "2026-09-01", "2026-09-02"); return err },
		"93 days":     func() error { _, err := ManualOrders(ctx, nil, good, tok, "2026-01-01", "2026-04-03"); return err },
		"reversed":    func() error { _, err := ChannelsReport(ctx, nil, good, tok, "2026-09-02", "2026-09-01"); return err },
		"short token": func() error { _, err := Funnel(ctx, nil, good, "x", "2026-09-01", "2026-09-02", ""); return err },
		"bad session": func() error { _, err := Funnel(ctx, nil, good, tok, "2026-09-01", "2026-09-02", "nope"); return err },
		"nil tx csv":  func() error { _, err := ProductsCSV(ctx, nil, good, tok, "2026-09-01", "2026-09-02"); return err },
	} {
		if err := fn(); err != command.ErrInvalid {
			t.Errorf("%s: %v want ErrInvalid", name, err)
		}
	}
}
