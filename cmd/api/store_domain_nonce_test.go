// store_domain_nonce_test.go: unit tests for the per-row TLS nonce endpoint (R5 store-domains P1-2). The pool path is
// faked through nonceQuerier, so the tests never touch PG; the definer control.tls_pending_nonce is proven by the PG
// gate suite.

package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
)

type nonceRow struct {
	ok  bool
	err error
}

func (r *nonceRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if b, ok := dest[0].(*bool); ok {
		*b = r.ok
		return nil
	}
	return fmt.Errorf("unexpected scan dest %T", dest[0])
}

type nonceQ struct {
	ok    bool
	err   error
	host  string
	nonce string
}

func (q *nonceQ) QueryRow(_ context.Context, _ string, args ...any) pgx.Row {
	q.host, _ = args[0].(string)
	q.nonce, _ = args[1].(string)
	return &nonceRow{ok: q.ok, err: q.err}
}

func nonceGET(path, host string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, path, nil)
	r.Host = host
	return r
}

func TestNonceFromPath(t *testing.T) {
	for _, tc := range []struct {
		in, want string
	}{
		{"/.well-known/lc-domain-check/abc123", "abc123"},
		{"/.well-known/lc-domain-check/", ""},    // empty nonce
		{"/.well-known/lc-domain-check", ""},     // prefix only
		{"/.well-known/lc-domain-check/a/b", ""}, // extra segment
		{"/other", ""},                           // wrong prefix
	} {
		if got := nonceFromPath(tc.in); got != tc.want {
			t.Errorf("nonceFromPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRequestHostname(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"Shop.Example.com", "shop.example.com"},
		{"shop.example.com:443", "shop.example.com"},
		{"  SHOP.Example.COM  ", "shop.example.com"},
		{"", ""},
	} {
		if got := requestHostname(tc.in); got != tc.want {
			t.Errorf("requestHostname(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNonceHandlerMethod(t *testing.T) {
	h := buildStoreDomainNonceHandlerOn(&nonceQ{})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/.well-known/lc-domain-check/x", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST = %d, want 405", rr.Code)
	}
}

func TestNonceHandlerFailClosed(t *testing.T) {
	for name, tc := range map[string]struct {
		q    *nonceQ
		req  *http.Request
		want int
	}{
		"empty nonce": {&nonceQ{}, nonceGET("/", "shop.example.com"), http.StatusNotFound},
		"empty host":  {&nonceQ{}, nonceGET("/.well-known/lc-domain-check/x", ""), http.StatusNotFound},
		"query error": {&nonceQ{err: errors.New("down")}, nonceGET("/.well-known/lc-domain-check/x", "shop.example.com"), http.StatusServiceUnavailable},
		"not pending": {&nonceQ{ok: false}, nonceGET("/.well-known/lc-domain-check/x", "shop.example.com"), http.StatusNotFound},
	} {
		h := buildStoreDomainNonceHandlerOn(tc.q)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, tc.req)
		if rr.Code != tc.want {
			t.Errorf("%s = %d, want %d", name, rr.Code, tc.want)
		}
	}
}

func TestNonceHandlerOK(t *testing.T) {
	q := &nonceQ{ok: true}
	h := buildStoreDomainNonceHandlerOn(q)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, nonceGET("/.well-known/lc-domain-check/abc123", "Shop.Example.com:443"))
	if rr.Code != http.StatusOK {
		t.Fatalf("= %d, want 200", rr.Code)
	}
	if rr.Body.String() != "abc123" {
		t.Errorf("body = %q, want the nonce", rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	// The definer receives the lower-cased host and the exact nonce.
	if q.host != "shop.example.com" || q.nonce != "abc123" {
		t.Errorf("definer args = %q, %q", q.host, q.nonce)
	}
}

func TestMountStoreDomainNonce(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	mux := mountStoreDomainNonce(next, buildStoreDomainNonceHandlerOn(&nonceQ{ok: true}))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, nonceGET("/.well-known/lc-domain-check/x", "shop.example.com"))
	if rr.Code != http.StatusOK {
		t.Errorf("nonce route = %d, want 200", rr.Code)
	}
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/anything", nil))
	if rr.Code != http.StatusTeapot {
		t.Errorf("fallthrough = %d, want 418", rr.Code)
	}
	// Non-canonical paths must reach next with the ORIGINAL path: a ServeMux here would answer 307 to the cleaned path and the
	// namespace reservations inside (mountMeta, mountStripe, ...) could never refuse the alias (release gate r5 MetaRuntime alias).
	for _, alias := range []string{"/v1//meta/webhooks/1/page", "/x/../v1/meta/webhooks/1/page", "/.well-known//lc-domain-check/x"} {
		rr = httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "http://example.test/", nil)
		req.URL.Path = alias
		mux.ServeHTTP(rr, req)
		if rr.Code != http.StatusTeapot || rr.Header().Get("Location") != "" {
			t.Errorf("alias %q = %d location %q, want an untouched pass-through (418, no redirect)", alias, rr.Code, rr.Header().Get("Location"))
		}
	}
}
