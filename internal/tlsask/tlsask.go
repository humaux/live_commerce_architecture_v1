// Package tlsask owns the edge TLS ask endpoint (R5 unit store-domains, Decision 4): Caddy's on_demand_tls
// `ask http://api:<port>/internal/tls-ask` calls it before issuing a certificate, and it answers 200 only for a
// hostname that maps to an ACTIVE in-window origin or a merchant origin in TLS_PENDING — every other host fails
// closed.
//
// P1-3 (integrator ruling): the ask answers from an in-memory set of admitted hosts, refreshed every 30 s from
// control.admitted_storefront_hosts() — there is no per-ask database query and no shared global bucket, so a
// random-SNI scanner cannot starve a certificate-eligible host. This also fixes P2-1 (the set excludes ACTIVE rows
// past valid_until) and P2-6 (no negative cache delays a freshly-verified host).
//
// Owns: the ask decision's transport. The decision itself is control.admitted_storefront_hosts /
// control.resolve_storefront_ask (0106, EXECUTE commerce_runtime); this package never reads a domain table directly
// and never answers "allow" for a host the definer did not admit.
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

// Querier is the one call the ask needs for a per-ask fallback; *pgxpool.Pool and pgx.Tx satisfy it, tests fake it.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// setLoader is the optional surface the 30-second in-memory refresh needs; *pgxpool.Pool satisfies it. A QueryRow-only
// querier (the pure-logic tests and the adversarial gate) does not, so those take the per-ask fallback below.
type setLoader interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Defaults (kept for the frozen constructor shape; the shared rate-limit bucket and TTL cache these named are
// gone — P1-3 replaced them with the 30 s in-memory set). The parameters are validated and otherwise unused.
const (
	DefaultMaxPerMinute = 120
	DefaultDenyTTL      = 5 * time.Minute
	DefaultAllowTTL     = time.Minute
)

// refreshInterval is how often the admitted set reloads from control.admitted_storefront_hosts (P1-3 ruling).
const refreshInterval = 30 * time.Second

// Service answers the ask for a hostname; the admitted set is safe for concurrent use.
type Service struct {
	q        Querier
	loader   setLoader // nil when q is QueryRow-only (per-ask fallback, no bucket)
	mu       sync.Mutex
	admitted map[string]bool
	next     time.Time
}

// New validates the limits (frozen signature) and returns a Service. q must run on a login with EXECUTE on
// control.admitted_storefront_hosts (and control.resolve_storefront_ask for the QueryRow-only fallback), i.e.
// commerce_runtime. maxPerMinute/denyTTL/allowTTL are validated and otherwise unused: the global bucket and TTL
// cache were removed by P1-3.
func New(q Querier, maxPerMinute int, denyTTL, allowTTL time.Duration) (*Service, error) {
	if q == nil || maxPerMinute < 1 || denyTTL < time.Second || allowTTL < time.Second {
		return nil, errInvalid
	}
	s := &Service{q: q, admitted: make(map[string]bool)}
	if loader, ok := q.(setLoader); ok {
		s.loader = loader
	}
	return s, nil
}

// Allow is the ask decision for one hostname: false when the host is not cert-eligible, true only when the definer
// admits it. A database error is returned for the caller to map to 503. There is no rate-limit starvation: a flood
// of unknown hosts never consumes a budget a legitimate host would need (P1-3).
func (s *Service) Allow(ctx context.Context, host string) (bool, error) {
	host = strings.ToLower(strings.TrimSpace(host))
	if !validHost(host) {
		return false, nil // malformed hosts fail closed, not an error
	}
	if s.loader == nil {
		// QueryRow-only querier (pure-logic tests and the adversarial gate): per-ask admission, no cache, no bucket.
		var allowed bool
		if err := s.q.QueryRow(ctx, `SELECT control.resolve_storefront_ask($1)`, host).Scan(&allowed); err != nil {
			return false, err
		}
		return allowed, nil
	}
	return s.setLookup(ctx, host)
}

// setLookup serves the ask from the in-memory set, refreshing it every refreshInterval. A refresh error is returned
// (503) rather than a stale admission.
func (s *Service) setLookup(ctx context.Context, host string) (bool, error) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.next.IsZero() || !now.Before(s.next) {
		if err := s.refreshLocked(ctx, now); err != nil {
			return false, err
		}
	}
	return s.admitted[host], nil
}

func (s *Service) refreshLocked(ctx context.Context, now time.Time) error {
	rows, err := s.loader.Query(ctx, `SELECT * FROM control.admitted_storefront_hosts()`)
	if err != nil {
		return err
	}
	defer rows.Close()
	admitted := make(map[string]bool)
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			return err
		}
		admitted[strings.ToLower(h)] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	s.admitted = admitted
	s.next = now.Add(refreshInterval)
	return nil
}

// Handler serves GET /internal/tls-ask?domain=<host>: 200 when allowed, 404 when denied or unknown (fail closed —
// no oracle between unknown/not-eligible), 503 on a database error. The only query key is `domain` (Caddy's
// on_demand_tls format); anything else is 422.
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
