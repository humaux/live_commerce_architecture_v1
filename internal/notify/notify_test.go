package notify

// Unit tests of the pure parts: order number, money, the six renderings in three locales (escaping, no remote image, link rules) and the
// worker's recording logic against a fake queue and a fake mailer. The database side is covered by tests/foundation/buyer_comms_smoke_test.go.

import (
	"context"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/mail"
)

const oid = "0123abcd-4567-4def-8123-456789abcdef"

func ptr[T any](v T) *T { return &v }

func TestOrderNumberAndMoney(t *testing.T) {
	if got := OrderNumber(oid); got != "0123-ABCD-4567" {
		t.Fatalf("order number %q", got)
	}
	for _, bad := range []string{"", "xyz", "0123abcd-4567-4def-8123-456789abcdeg"} {
		if OrderNumber(bad) != "" {
			t.Fatalf("OrderNumber(%q) must be empty", bad)
		}
	}
	for in, want := range map[[2]any]string{{int64(123456), "TWD"}: "TWD 1,234.56", {int64(5), "TWD"}: "TWD 0.05", {int64(1234567), "JPY"}: "JPY 1,234,567", {int64(0), "TWD"}: "TWD 0.00"} {
		if got := Money(in[0].(int64), in[1].(string)); got != want {
			t.Fatalf("Money%v = %q want %q", in, got, want)
		}
	}
}

func base(kind string) Payload {
	return Payload{BatchID: "b", Kind: kind, OrderID: oid, To: []string{"buyer@example.test"}, StoreName: "Shop <b>&</b>", Origin: ptr("https://shop.example.test"),
		TotalMinor: 150000, Currency: "TWD", PaymentMode: "bank_transfer", ExpiresAt: time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC)}
}

func TestRenderAllKindsAllLocales(t *testing.T) {
	for _, loc := range []string{"zh-TW", "zh-CN", "en"} {
		for _, kind := range []string{KindPlaced, KindPaid, KindShipped, KindCancelled, KindRefunded} {
			p := base(kind)
			p.Locale = ptr(loc)
			p.Bank = &Bank{BankName: "Bank", Branch: "Main", AccountName: "Shop Ltd", AccountNumber: "123-456-789"}
			p.Shipment = &Shipment{CarrierCode: "sf_express", TrackingNumber: "SF123", TrackingURL: ptr("https://track.example.test/SF123")}
			msgs := Render(p)
			if len(msgs) != 1 {
				t.Fatalf("%s/%s: %d messages", loc, kind, len(msgs))
			}
			m := msgs[0]
			if m.To != "buyer@example.test" || !strings.Contains(m.Subject, "Shop") || strings.ContainsAny(m.Subject, "\r\n") {
				t.Fatalf("%s/%s header %+v", loc, kind, m)
			}
			// §E4: store name + order number + link; HTML escapes dynamic text; nothing remote is loaded
			for _, part := range []string{m.Text, m.HTML} {
				if !strings.Contains(part, "0123-ABCD-4567") || !strings.Contains(part, "https://shop.example.test/"+loc+"/orders/"+oid) {
					t.Fatalf("%s/%s lacks order number or link:\n%s", loc, kind, part)
				}
			}
			if strings.Contains(m.HTML, "<b>&") || strings.Contains(m.HTML, "<img") || strings.Contains(m.HTML, "src=") {
				t.Fatalf("%s/%s HTML unsafe: %s", loc, kind, m.HTML)
			}
			if (kind == KindPlaced) != strings.Contains(m.Text, "123-456-789") {
				t.Fatalf("%s/%s bank details only in placed", loc, kind)
			}
			if (kind == KindShipped) != strings.Contains(m.Text, "SF123") {
				t.Fatalf("%s/%s tracking only in shipped", loc, kind)
			}
		}
	}
}

func TestRenderRules(t *testing.T) {
	p := base(KindPlaced)
	p.Origin = nil
	if m := Render(p)[0]; strings.Contains(m.Text, "http") {
		t.Fatalf("no origin must mean no link: %s", m.Text)
	}
	p = base(KindShipped)
	p.Shipment = &Shipment{CarrierCode: "other", CarrierName: ptr("Local Express"), TrackingNumber: "T1", TrackingURL: ptr("http://insecure.example.test")}
	if m := Render(p)[0]; strings.Contains(m.Text, "insecure") || !strings.Contains(m.Text, "Local Express") {
		t.Fatalf("http tracking url must be dropped, carrier name kept: %s", m.Text)
	}
	p = base(KindPaid)
	p.StoreName = "Evil\r\nBcc: x@example.test"
	if m := Render(p)[0]; strings.ContainsAny(m.Subject, "\r\n") {
		t.Fatalf("subject must be one line: %q", m.Subject)
	}
	if Render(Payload{Kind: "nope", OrderID: oid, To: []string{"a@b.c"}}) != nil || Render(Payload{Kind: KindPaid, OrderID: "bad", To: []string{"a@b.c"}}) != nil {
		t.Fatal("unknown kind / bad id must not render")
	}
	p = base(KindPaid)
	p.Locale = ptr("fr")
	if m := Render(p)[0]; !strings.Contains(m.Subject, "已確認收到付款") {
		t.Fatalf("unknown locale falls back to zh-TW: %q", m.Subject)
	}
}

func TestRenderMerchantBatch(t *testing.T) {
	p := Payload{Kind: KindMerchantNew, StoreName: "Shop", To: []string{"a@example.test", "b@example.test"}, Count: 3, OrderIDs: []string{oid, "bad"}}
	msgs := Render(p)
	if len(msgs) != 2 || !strings.Contains(msgs[0].Subject, "(3)") || !strings.Contains(msgs[0].Text, "0123-ABCD-4567") || strings.Contains(msgs[0].Text, "@") {
		t.Fatalf("merchant batch: %+v", msgs)
	}
}

type fakeQ struct {
	claim   []Payload
	state   map[string]string
	hash    map[string][]byte
	claimed int
}

func (f *fakeQ) Claim(context.Context, int) ([]Payload, error) {
	f.claimed++
	out := f.claim
	f.claim = nil
	return out, nil
}
func (f *fakeQ) Record(_ context.Context, b, s string, h []byte) error {
	f.state[b], f.hash[b] = s, h
	return nil
}

type fakeM struct {
	errs []error
	sent []mail.Message
}

func (m *fakeM) Send(_ context.Context, msg mail.Message) (string, error) {
	var err error
	if len(m.errs) > 0 {
		err, m.errs = m.errs[0], m.errs[1:]
	}
	if err == nil {
		m.sent = append(m.sent, msg)
	}
	return "250", err
}

func TestWorkerRecording(t *testing.T) {
	two := Payload{BatchID: "m", Kind: KindMerchantNew, StoreName: "S", To: []string{"b@example.test", "a@example.test"}, Count: 1, OrderIDs: []string{oid}}
	for name, tc := range map[string]struct {
		errs []error
		to   Payload
		want string
		sent int
	}{
		"sent":            {nil, base(KindPaid), "SENT", 1},
		"failed":          {[]error{mail.ErrFailed}, base(KindPaid), "FAILED", 0},
		"unknown":         {[]error{mail.ErrUnknown}, base(KindPaid), "UNKNOWN", 0},
		"one of two sent": {[]error{nil, mail.ErrFailed}, two, "SENT", 1},
		"unknown wins":    {[]error{nil, mail.ErrUnknown}, two, "UNKNOWN", 1},
		"unrenderable":    {nil, Payload{BatchID: "x", Kind: "nope"}, "FAILED", 0},
	} {
		tc.to.BatchID = name
		q, m := &fakeQ{claim: []Payload{tc.to}, state: map[string]string{}, hash: map[string][]byte{}}, &fakeM{errs: tc.errs}
		w := &Worker{q: q, m: m}
		if n, err := w.Once(context.Background()); err != nil || n != 1 {
			t.Fatalf("%s: n=%d err=%v", name, n, err)
		}
		if q.state[name] != tc.want || len(m.sent) != tc.sent {
			t.Fatalf("%s: state %q want %q, sent %d want %d", name, q.state[name], tc.want, len(m.sent), tc.sent)
		}
	}
}

func TestWorkerShutdownBeforeSendIsFailedNotUnknown(t *testing.T) {
	q, m := &fakeQ{claim: []Payload{base(KindPaid)}, state: map[string]string{}, hash: map[string][]byte{}}, &fakeM{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (&Worker{q: q, m: m}).Once(ctx); err != nil || q.state["b"] != "FAILED" || len(m.sent) != 0 {
		t.Fatalf("state %q sent %d err %v", q.state["b"], len(m.sent), err)
	}
}

func TestRecipientHash(t *testing.T) {
	a, b := recipientHash(base(KindPaid)), recipientHash(Payload{Kind: KindPaid, OrderID: oid, To: []string{"BUYER@example.test"}})
	if len(a) != 32 || string(a) != string(b) {
		t.Fatal("hash must be sha256 and case-insensitive on the address")
	}
	m1 := recipientHash(Payload{Kind: KindMerchantNew, To: []string{"a@x.y", "b@x.y"}})
	m2 := recipientHash(Payload{Kind: KindMerchantNew, To: []string{"B@x.y", "a@x.y"}})
	if string(m1) != string(m2) || string(m1) == string(a) {
		t.Fatal("merchant hash is order-independent and distinct")
	}
	if recipientHash(Payload{Kind: KindPaid}) != nil {
		t.Fatal("no recipient, no hash")
	}
}
