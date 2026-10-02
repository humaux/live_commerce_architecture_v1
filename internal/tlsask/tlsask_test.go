// tlsask_test.go: pure-logic unit tests for the edge TLS ask decision (Decision 4). The definer
// (control.resolve_storefront_ask / control.admitted_storefront_hosts) is faked; P1-3 removed the shared rate-limit
// bucket and the TTL cache, so these tests prove the two paths — the 30-second in-memory set (production: Query
// surface) and the per-ask fallback (QueryRow-only fakes). The SQL/grants are proven against real PG by the gate suite.

package tlsask

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeRow struct {
	allowed bool
	err     error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*(dest[0].(*bool)) = r.allowed
	return nil
}

// fakeQ is a QueryRow-only querier: the per-ask fallback (no set, no bucket, no cache).
type fakeQ struct {
	row   fakeRow
	calls int
	args  []any
}

func (f *fakeQ) QueryRow(_ context.Context, _ string, args ...any) pgx.Row {
	f.calls++
	f.args = args
	return f.row
}

// fakeSetQ is a Query+QueryRow querier: New() detects the set surface and serves the in-memory refresh.
type fakeSetQ struct {
	hosts         []string
	queryCalls    int
	queryErr      error
	queryRowCalls int
}

func (f *fakeSetQ) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	f.queryRowCalls++
	return fakeRow{err: errors.New("unexpected QueryRow in set path")}
}

func (f *fakeSetQ) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	f.queryCalls++
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	rows := make([][]any, 0, len(f.hosts))
	for _, h := range f.hosts {
		rows = append(rows, []any{h})
	}
	return &fakeRows{rows: rows}, nil
}

// fakeRows is a minimal pgx.Rows over string values (the admitted-host set read).
type fakeRows struct {
	rows [][]any
	idx  int
	err  error
}

func (r *fakeRows) Close()                                       {}
func (r *fakeRows) Err() error                                   { return r.err }
func (r *fakeRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }

func (r *fakeRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *fakeRows) Scan(dest ...any) error {
	vals := r.rows[r.idx-1]
	for i, d := range dest {
		if i >= len(vals) {
			return errors.New("scan: too many destinations")
		}
		switch p := d.(type) {
		case *string:
			*p, _ = vals[i].(string)
		default:
			return errors.New("scan: unsupported destination")
		}
	}
	return nil
}

func (r *fakeRows) Values() ([]any, error) { return nil, nil }
func (r *fakeRows) RawValues() [][]byte    { return nil }
func (r *fakeRows) Conn() *pgx.Conn        { return nil }
func (r *fakeRows) TypeMap() *pgtype.Map   { return nil }

func statusFor(s *Service, target string) int {
	r := httptest.NewRequest(http.MethodGet, target, nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w.Code
}

func TestNewValidates(t *testing.T) {
	if _, err := New(nil, 120, time.Minute, time.Minute); err == nil {
		t.Fatal("nil querier accepted")
	}
	if _, err := New(&fakeQ{}, 0, time.Minute, time.Minute); err == nil {
		t.Fatal("zero rate limit accepted")
	}
	if _, err := New(&fakeQ{}, 120, time.Millisecond, time.Minute); err == nil {
		t.Fatal("sub-second deny TTL accepted")
	}
	if _, err := New(&fakeQ{}, 120, time.Minute, time.Millisecond); err == nil {
		t.Fatal("sub-second allow TTL accepted")
	}
	if _, err := New(&fakeQ{}, 120, time.Minute, time.Minute); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
}

func TestAllowBadHostsFailClosedWithoutDB(t *testing.T) {
	q := &fakeQ{}
	s, _ := New(q, 120, time.Minute, time.Minute)
	for _, host := range []string{"", "  ", "bad host", "example.com:443", "localhost", "shop_example.com"} {
		allowed, err := s.Allow(context.Background(), host)
		if err != nil || allowed {
			t.Errorf("Allow(%q) = %v, %v; want false, nil", host, allowed, err)
		}
	}
	if q.calls != 0 {
		t.Fatalf("bad hosts reached the database")
	}
}

func TestAllowNormalizesWithoutCache(t *testing.T) {
	q := &fakeQ{row: fakeRow{allowed: true}}
	s, _ := New(q, 120, time.Minute, time.Minute)
	allowed, err := s.Allow(context.Background(), "Shop.Example.com")
	if err != nil || !allowed {
		t.Fatalf("Allow = %v, %v", allowed, err)
	}
	if q.args[0] != "shop.example.com" {
		t.Fatalf("host not normalized: %v", q.args[0])
	}
	// P1-3 removed the positive cache: a second ask re-queries the definer.
	q.row = fakeRow{allowed: false}
	allowed, err = s.Allow(context.Background(), "shop.example.com")
	if err != nil || allowed {
		t.Fatalf("expected re-query to observe new admission state, got %v, %v", allowed, err)
	}
	if q.calls != 2 {
		t.Fatalf("expected 2 per-ask calls, got %d", q.calls)
	}
}

func TestAllowDeniesWithoutNegativeCache(t *testing.T) {
	q := &fakeQ{row: fakeRow{allowed: false}}
	s, _ := New(q, 120, time.Minute, time.Minute)
	allowed, err := s.Allow(context.Background(), "shop.example.com")
	if err != nil || allowed {
		t.Fatalf("Allow = %v, %v; want false, nil", allowed, err)
	}
	// P1-3 removed the negative cache (P2-6): a freshly-verified host is admitted on the next ask.
	q.row = fakeRow{allowed: true}
	allowed, err = s.Allow(context.Background(), "shop.example.com")
	if err != nil || !allowed {
		t.Fatalf("freshly-verified host still denied: %v, %v", allowed, err)
	}
	if q.calls != 2 {
		t.Fatalf("expected 2 per-ask calls, got %d", q.calls)
	}
}

func TestAllowNoBucketStarvation(t *testing.T) {
	q := &fakeQ{row: fakeRow{allowed: false}}
	s, _ := New(q, 1, time.Minute, time.Minute) // bucket size gone (P1-3): no global starvation
	for _, host := range []string{"scan1.example", "scan2.example", "scan3.example"} {
		if allowed, err := s.Allow(context.Background(), host); err != nil || allowed {
			t.Fatalf("unknown host %s = %v, %v; want a clean deny", host, allowed, err)
		}
	}
	q.row = fakeRow{allowed: true}
	if allowed, err := s.Allow(context.Background(), "legit.example.com"); err != nil || !allowed {
		t.Fatalf("legitimate host starved by scanners: %v, %v", allowed, err)
	}
	if q.calls != 4 {
		t.Fatalf("expected 4 per-ask calls, got %d", q.calls)
	}
}

func TestAllowDBError(t *testing.T) {
	q := &fakeQ{row: fakeRow{err: errors.New("connection refused")}}
	s, _ := New(q, 120, time.Minute, time.Minute)
	if _, err := s.Allow(context.Background(), "shop.example.com"); err == nil {
		t.Fatal("db error swallowed")
	}
}

// TestAllowSetPath proves the production path: New() detects the Query surface, serves asks from the in-memory
// admitted set, refreshes once per interval, and never issues a per-ask query.
func TestAllowSetPath(t *testing.T) {
	q := &fakeSetQ{hosts: []string{"shop.example.com", "pending.example.com"}}
	s, err := New(q, 120, time.Minute, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if s.loader == nil {
		t.Fatal("set loader not detected")
	}

	if allowed, err := s.Allow(context.Background(), "Shop.Example.com"); err != nil || !allowed {
		t.Fatalf("admitted host (normalized) = %v, %v; want true, nil", allowed, err)
	}
	if allowed, err := s.Allow(context.Background(), "pending.example.com"); err != nil || !allowed {
		t.Fatalf("TLS_PENDING host = %v, %v; want true, nil", allowed, err)
	}
	if allowed, err := s.Allow(context.Background(), "unknown.example.com"); err != nil || allowed {
		t.Fatalf("unknown host = %v, %v; want false, nil", allowed, err)
	}
	if q.queryCalls != 1 {
		t.Fatalf("expected one refresh query to serve all asks, got %d", q.queryCalls)
	}
	if q.queryRowCalls != 0 {
		t.Fatalf("set path issued %d per-ask queries", q.queryRowCalls)
	}
}

// TestAllowSetRefreshError proves a refresh failure fails closed (503), not a stale admission.
func TestAllowSetRefreshError(t *testing.T) {
	q := &fakeSetQ{hosts: []string{"shop.example.com"}, queryErr: errors.New("connection refused")}
	s, err := New(q, 120, time.Minute, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Allow(context.Background(), "shop.example.com"); err == nil {
		t.Fatal("refresh error swallowed")
	}
}

func TestHandlerStatusCodes(t *testing.T) {
	allow := &fakeQ{row: fakeRow{allowed: true}}
	s, _ := New(allow, 120, time.Minute, time.Minute)

	if code := statusFor(s, "/internal/tls-ask?domain=shop.example.com"); code != http.StatusOK {
		t.Errorf("allowed = %d", code)
	}
	if code := statusFor(s, "/internal/tls-ask"); code != http.StatusUnprocessableEntity {
		t.Errorf("no query = %d", code)
	}
	if code := statusFor(s, "/internal/tls-ask?domain=shop.example.com&extra=1"); code != http.StatusUnprocessableEntity {
		t.Errorf("extra key = %d", code)
	}

	r := httptest.NewRequest(http.MethodPost, "/internal/tls-ask?domain=shop.example.com", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST = %d", w.Code)
	}

	deny, _ := New(&fakeQ{row: fakeRow{allowed: false}}, 120, time.Minute, time.Minute)
	if code := statusFor(deny, "/internal/tls-ask?domain=shop.example.com"); code != http.StatusNotFound {
		t.Errorf("denied = %d", code)
	}

	errs, _ := New(&fakeQ{row: fakeRow{err: errors.New("boom")}}, 120, time.Minute, time.Minute)
	if code := statusFor(errs, "/internal/tls-ask?domain=shop.example.com"); code != http.StatusServiceUnavailable {
		t.Errorf("db error = %d", code)
	}
}
