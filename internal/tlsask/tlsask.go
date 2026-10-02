// Package tlsask owns the edge TLS ask endpoint (R5 unit store-domains, Decision 4): Caddy's on_demand_tls
// `ask http://api:<port>/internal/tls-ask` calls it before issuing a certificate, and it answers 200 only for a
// hostname that maps to an ACTIVE origin or a merchant origin in TLS_PENDING — every other host fails closed.
// It adds two bounds the SQL cannot: a per-service rate limit (a scanner probing many hosts) and a short negative
// cache (a repeated probe of one unknown host stays cheap).
//
// Owns: the ask decision's transport + rate limit + cache. The decision itself is control.resolve_storefront_ask
// (0106, EXECUTE commerce_runtime); this package never reads a domain table directly and never answers "allow"
// for a host the definer did not admit.
// Never: issues certificates (Caddy does), resolves a store (internal/domains), or serves merchant/buyer traffic.
// Depends on: internal/domains (hostname grammar). Used by: cmd/api (mounts /internal/tls-ask).
package tlsask

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/domains"
	"livecommerce/internal/httperror"
)

// Querier is the one call the ask needs; *pgxpool.Pool and pgx.Tx satisfy it, tests fake it.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Defaults (bounded, documented): the ask is internal-network only, so the rate limit is generous enough for a
// fleet edge issuing certificates in bursts but not for a host-scan.
const (
	DefaultMaxPerMinute = 120
	DefaultDenyTTL      = 5 * time.Minute
	DefaultAllowTTL     = time.Minute
)

// Service answers the ask for a hostname. The cache is safe for concurrent use.
type Service struct {
	q        Querier
	limiter  *limiter
	denyTTL  time.Duration
	allowTTL time.Duration
}

// New validates the limits and returns a Service. q must run on a login with EXECUTE on
// control.resolve_storefront_ask (commerce_runtime).
func New(q Querier, maxPerMinute int, denyTTL, allowTTL time.Duration) (*Service, error) {
	if q == nil || maxPerMinute < 1 || denyTTL < time.Second || allowTTL < time.Second {
		return nil, errInvalid
	}
	return &Service{q: q, limiter: newLimiter(maxPerMinute, time.Minute), denyTTL: denyTTL, allowTTL: allowTTL}, nil
}

// Allow is the ask decision for one hostname: false when the host is not cert-eligible (or the rate limit is
// exhausted), true only when the definer admits it. A database error is returned for the caller to map to 503.
func (s *Service) Allow(ctx context.Context, host string) (bool, error) {
	host = strings.ToLower(strings.TrimSpace(host))
	if !validHost(host) {
		return false, nil // malformed hosts fail closed, not an error
	}
	now := time.Now()
	if !s.limiter.allow(now) {
		return false, nil
	}
	if allowed, ok := s.limiter.cached(host, now); ok {
		return allowed, nil
	}
	var allowed bool
	if err := s.q.QueryRow(ctx, `SELECT control.resolve_storefront_ask($1)`, host).Scan(&allowed); err != nil {
		return false, err
	}
	s.limiter.record(host, allowed, now, s.allowTTL, s.denyTTL)
	return allowed, nil
}

// Handler serves GET /internal/tls-ask?domain=<host>: 200 when allowed, 404 when denied, unknown or rate-limited
// (fail closed — a burst above the per-service limit reads as "not eligible", never as a scan signal), 503 on a
// database error. The only query key is `domain` (Caddy's on_demand_tls format); anything else is 422.
func (s *Service) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			httperror.Write(w, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}
		if r.URL.RawQuery == "" || len(r.URL.RawQuery) > 2048 {
			httperror.Write(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		values, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil || len(values) != 1 || len(values["domain"]) != 1 {
			httperror.Write(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		allowed, err := s.Allow(r.Context(), values["domain"][0])
		if err != nil {
			httperror.Write(w, http.StatusServiceUnavailable, "unavailable")
			return
		}
		if !allowed {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

func validHost(host string) bool {
	return domains.ValidOrigin("https://" + host)
}

var errInvalid = &serviceError{"invalid tls ask configuration"}

type serviceError struct{ msg string }

func (e *serviceError) Error() string { return e.msg }

// limiter is a fixed-window per-service rate limit plus a small host cache (positive and negative). All time
// flows through its method parameters so tests can drive it deterministically; it is safe for concurrent use.
type limiter struct {
	mu      sync.Mutex
	max     int
	window  time.Duration
	windowN int64 // the window serial this counter belongs to
	count   int
	cache   map[string]cacheEntry
}

type cacheEntry struct {
	allowed   bool
	expiresAt time.Time
}

func newLimiter(max int, window time.Duration) *limiter {
	return &limiter{max: max, window: window, cache: make(map[string]cacheEntry)}
}

func (l *limiter) allow(now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	serial := now.UnixNano() / int64(l.window)
	if serial != l.windowN {
		l.windowN = serial
		l.count = 0
	}
	if l.count >= l.max {
		return false
	}
	l.count++
	return true
}

func (l *limiter) cached(host string, now time.Time) (bool, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.cache[host]
	if !ok || e.expiresAt.Before(now) {
		return false, false
	}
	return e.allowed, true
}

func (l *limiter) record(host string, allowed bool, now time.Time, allowTTL, denyTTL time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	ttl := denyTTL
	if allowed {
		ttl = allowTTL
	}
	l.cache[host] = cacheEntry{allowed: allowed, expiresAt: now.Add(ttl)}
}
