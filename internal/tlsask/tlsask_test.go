// tlsask_test.go: pure-logic unit tests for the edge TLS ask decision (Decision 4). The definer
// (control.resolve_storefront_ask) is a one-row fake; the rate limiter and cache are driven through the real Service
// methods. The SQL/grants are proven against real PG by the gate suite.

package tlsask

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
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

func TestAllowNormalizesAndCaches(t *testing.T) {
	q := &fakeQ{row: fakeRow{allowed: true}}
	s, _ := New(q, 120, time.Minute, time.Minute)
	allowed, err := s.Allow(context.Background(), "Shop.Example.com")
	if err != nil || !allowed {
		t.Fatalf("Allow = %v, %v", allowed, err)
	}
	if q.args[0] != "shop.example.com" {
		t.Fatalf("host not normalized: %v", q.args[0])
	}
	// a second ask for the same host is served from the positive cache, not the database
	q.row = fakeRow{allowed: false}
	allowed, err = s.Allow(context.Background(), "shop.example.com")
	if err != nil || !allowed {
		t.Fatalf("cached allow = %v, %v", allowed, err)
	}
	if q.calls != 1 {
		t.Fatalf("cache miss: %d calls", q.calls)
	}
}

func TestAllowDeniesAndCachesNegative(t *testing.T) {
	q := &fakeQ{row: fakeRow{allowed: false}}
	s, _ := New(q, 120, time.Minute, time.Minute)
	allowed, err := s.Allow(context.Background(), "shop.example.com")
	if err != nil || allowed {
		t.Fatalf("Allow = %v, %v; want false, nil", allowed, err)
	}
	q.row = fakeRow{allowed: true}
	allowed, _ = s.Allow(context.Background(), "shop.example.com")
	if allowed {
		t.Fatal("cached negative turned into allow")
	}
	if q.calls != 1 {
		t.Fatalf("cache miss: %d calls", q.calls)
	}
}

func TestAllowRateLimit(t *testing.T) {
	q := &fakeQ{row: fakeRow{allowed: true}}
	s, _ := New(q, 1, time.Minute, time.Minute) // one ask per minute
	if allowed, _ := s.Allow(context.Background(), "a.example.com"); !allowed {
		t.Fatal("first ask should pass")
	}
	calls := q.calls
	allowed, err := s.Allow(context.Background(), "b.example.com")
	if err != nil || allowed {
		t.Fatalf("rate-limited ask = %v, %v; want false, nil", allowed, err)
	}
	if q.calls != calls {
		t.Fatal("rate-limited ask reached the database")
	}
}

func TestAllowDBError(t *testing.T) {
	q := &fakeQ{row: fakeRow{err: errors.New("connection refused")}}
	s, _ := New(q, 120, time.Minute, time.Minute)
	if _, err := s.Allow(context.Background(), "shop.example.com"); err == nil {
		t.Fatal("db error swallowed")
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
