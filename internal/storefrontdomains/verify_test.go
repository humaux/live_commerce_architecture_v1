// verify_test.go: pure-logic unit tests for the DNS/TLS worker sweep (Decision 3). The resolver and prober are faked
// so the package never dials DNS/TLS in a test; the batch definers (next_store_domain_dns_check /
// next_store_domain_tls_probe) are faked as one Query each. The SQL transitions are proven against real PG by the gate
// suite.

package storefrontdomains

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeResolver struct {
	txt      []string
	txtErr   error
	cname    string
	cnameErr error
	addrs    []string
	addrsErr error
}

func (r fakeResolver) LookupTXT(_ context.Context, _ string) ([]string, error) {
	return r.txt, r.txtErr
}
func (r fakeResolver) LookupCNAME(_ context.Context, _ string) (string, error) {
	return r.cname, r.cnameErr
}
func (r fakeResolver) LookupAddr(_ context.Context, _ string) ([]string, error) {
	return r.addrs, r.addrsErr
}

type fakeProber struct {
	notAfter time.Time
	err      error
}

func (p fakeProber) NotAfter(_ context.Context, _ string) (time.Time, error) {
	return p.notAfter, p.err
}

func TestVerifyDNS(t *testing.T) {
	host, base := "shop.example.com", "example.com"
	token := strings.Repeat("A", 43)
	for name, tc := range map[string]struct {
		r    fakeResolver
		want DNSResult
	}{
		"txt+cname match": {
			fakeResolver{txt: []string{token}, cname: "stores.example.com."},
			DNSResult{TXTFound: true, CNAMEMatch: true, Matched: true},
		},
		"txt+a record (apex)": {
			// P1-1: the apex A record proves "pointed at us" only when it is a non-empty subset of the edge
			// addresses stores.<base> serves (the fake resolver returns the same addrs for the edge lookup).
			fakeResolver{txt: []string{token}, cnameErr: errors.New("nxdomain"), addrs: []string{"203.0.113.1"}},
			DNSResult{TXTFound: true, AddrFound: true, AddrMatch: true, Matched: true},
		},
		"txt only (not pointed)": {
			fakeResolver{txt: []string{token}, cnameErr: errors.New("nxdomain"), addrsErr: errors.New("no addrs")},
			DNSResult{TXTFound: true, Matched: false},
		},
		"missing txt": {
			fakeResolver{txt: []string{"other"}, cname: "stores.example.com."},
			DNSResult{CNAMEMatch: true, Matched: false},
		},
	} {
		got, err := VerifyDNS(context.Background(), tc.r, host, token, base)
		if err != nil || got != tc.want {
			t.Errorf("%s: VerifyDNS = %+v, %v; want %+v", name, got, err, tc.want)
		}
	}
	// TXT lookup errors propagate (NXDOMAIN/timeout are a transient outage, not a proof failure)
	if _, err := VerifyDNS(context.Background(), fakeResolver{txtErr: errors.New("timeout")}, host, token, base); err == nil {
		t.Error("TXT lookup error swallowed")
	}
	// invalid inputs are rejected before any lookup
	for name, tc := range map[string]struct {
		r           Resolver
		host, token string
		base        string
	}{
		"nil resolver": {nil, host, token, base},
		"bad host":     {fakeResolver{}, "Shop.Example.com", token, base},
		"bad token":    {fakeResolver{}, host, "short", base},
		"bad base":     {fakeResolver{}, host, token, "localhost"},
	} {
		if _, err := VerifyDNS(context.Background(), tc.r, tc.host, tc.token, tc.base); err == nil {
			t.Errorf("%s: accepted invalid input", name)
		}
	}
}

func TestBackoffDelay(t *testing.T) {
	for _, tc := range []struct {
		failures int
		want     time.Duration
	}{
		{-1, time.Minute},
		{0, time.Minute},
		{1, 2 * time.Minute},
		{2, 4 * time.Minute},
		{3, 8 * time.Minute},
		{4, 16 * time.Minute},
		{5, 30 * time.Minute},
		{6, 30 * time.Minute},
		{100, 30 * time.Minute},
	} {
		if got := BackoffDelay(tc.failures); got != tc.want {
			t.Errorf("BackoffDelay(%d) = %v, want %v", tc.failures, got, tc.want)
		}
	}
}

func TestWithinDeadline(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	if !WithinDeadline(now.Add(time.Hour), now) {
		t.Error("future deadline should be open")
	}
	if WithinDeadline(now, now) {
		t.Error("deadline == now should be closed")
	}
	if WithinDeadline(now.Add(-time.Hour), now) {
		t.Error("past deadline should be closed")
	}
	if WithinDeadline(time.Time{}, now) {
		t.Error("zero deadline should be closed")
	}
}

func dnsCheckRow(deadline time.Time, lastChecked *time.Time, failures int) []any {
	return []any{testDomain, "https://shop.example.com", "shop.example.com", strings.Repeat("A", 43), "REQUESTED", deadline, lastChecked, failures}
}

func TestVerifyPendingDNSMatchAdvancesToTLS(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	q := &fakeTx{
		rowsSeq: []*fakeRows{
			{rows: [][]any{dnsCheckRow(now.Add(24*time.Hour), nil, 0)}},
			{rows: [][]any{}}, // no TLS_PENDING rows
		},
		rowSeq: []fakeRow{
			{raw: []byte(`{"domain_id":"` + testDomain + `","state":"OWNERSHIP_PENDING","expired":false,"dns_failures":0}`)},
			{raw: []byte(`{"domain_id":"` + testDomain + `","state":"TLS_PENDING","expired":false,"dns_failures":0}`)},
		},
	}
	r := fakeResolver{txt: []string{strings.Repeat("A", 43)}, cname: "stores.example.com."}
	dns, tls, err := VerifyPending(context.Background(), q, r, fakeProber{}, now, "example.com")
	if err != nil || dns != 1 || tls != 0 {
		t.Fatalf("VerifyPending = %d, %d, %v; want 1, 0, nil", dns, tls, err)
	}
	if q.calls != 2 {
		t.Fatalf("dnsAdvance called %d times, want 2", q.calls)
	}
	for _, a := range q.callArgs {
		if a[1] != true {
			t.Fatalf("advance matched arg = %v", a[1])
		}
	}
}

func TestVerifyPendingSkipsExpiredAndBackedOff(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-time.Minute)
	q := &fakeTx{
		rowsSeq: []*fakeRows{
			{rows: [][]any{
				dnsCheckRow(now.Add(-time.Hour), nil, 0),       // expired (deadline past)
				dnsCheckRow(now.Add(24*time.Hour), &recent, 2), // backed off (1m elapsed < 4m backoff)
			}},
			{rows: [][]any{}},
		},
	}
	dns, tls, err := VerifyPending(context.Background(), q, fakeResolver{}, fakeProber{}, now, "example.com")
	if err != nil || dns != 0 || tls != 0 {
		t.Fatalf("VerifyPending = %d, %d, %v; want 0, 0, nil", dns, tls, err)
	}
	if q.calls != 0 {
		t.Fatalf("no row should reach an advance, got %d", q.calls)
	}
}

func TestVerifyPendingTLSCompletes(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	notAfter := now.Add(90 * 24 * time.Hour)
	q := &fakeTx{
		rowsSeq: []*fakeRows{
			{rows: [][]any{}}, // no DNS checks
			{rows: [][]any{{testDomain, "https://shop.example.com", ""}}}, // one TLS_PENDING row (id, origin, nonce)
		},
		rowSeq: []fakeRow{
			{raw: []byte(`{"domain_id":"` + testDomain + `","state":"ACTIVE","version":5}`)},
		},
	}
	dns, tls, err := VerifyPending(context.Background(), q, fakeResolver{}, fakeProber{notAfter: notAfter}, now, "example.com")
	if err != nil || dns != 0 || tls != 1 {
		t.Fatalf("VerifyPending = %d, %d, %v; want 0, 1, nil", dns, tls, err)
	}
	if q.calls != 1 {
		t.Fatalf("tlsComplete called %d times, want 1", q.calls)
	}
	if got := q.args[1].(time.Time); !got.Equal(notAfter.UTC()) {
		t.Fatalf("tls notAfter arg = %v", got)
	}
}

func TestVerifyPendingAbortsOnFirstError(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	q := &fakeTx{} // no rows configured -> the DNS batch read errors
	dns, tls, err := VerifyPending(context.Background(), q, fakeResolver{}, fakeProber{}, now, "example.com")
	if err == nil || dns != 0 || tls != 0 {
		t.Fatalf("VerifyPending = %d, %d, %v; want 0, 0, error", dns, tls, err)
	}
}

func TestVerifyPendingInvalidInputs(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	if _, _, err := VerifyPending(context.Background(), nil, fakeResolver{}, fakeProber{}, now, "example.com"); err == nil {
		t.Error("nil querier accepted")
	}
	if _, _, err := VerifyPending(context.Background(), &fakeTx{}, nil, fakeProber{}, now, "example.com"); err == nil {
		t.Error("nil resolver accepted")
	}
	if _, _, err := VerifyPending(context.Background(), &fakeTx{}, fakeResolver{}, nil, now, "example.com"); err == nil {
		t.Error("nil prober accepted")
	}
	if _, _, err := VerifyPending(context.Background(), &fakeTx{}, fakeResolver{}, fakeProber{}, now, "localhost"); err == nil {
		t.Error("bad base domain accepted")
	}
	if _, _, err := VerifyPending(nil, &fakeTx{}, fakeResolver{}, fakeProber{}, now, "example.com"); err == nil {
		t.Error("nil context accepted")
	}
}
