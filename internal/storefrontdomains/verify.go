package storefrontdomains

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"strings"
	"time"
)

// verify.go is the DNS + TLS verification the "worker" (the operator one-shot sweep, cmd/store-admin domain-verify)
// runs against the commerce_storefront_registrar pool. Every external call is a seam: Resolver and TLSProber are
// faked in tests (match/mismatch/NXDOMAIN/timeout), so this package never dials DNS or TLS on its own in a test.

// verifyWindow bounds DNS verification: requested_at + 72h (the SQL verify_deadline). Past it the row stops being
// returned by control.next_store_domain_dns_check and the merchant re-requests for a fresh token.
const verifyWindow = 72 * time.Hour

// maxBackoff caps the exponential DNS backoff; the sweep is bounded to one run per 30 minutes per host after 6 failures.
const maxBackoff = 30 * time.Minute

// Resolver is the DNS seam: TXT for the token, CNAME and A/AAAA for the "pointed at us" half of the proof.
type Resolver interface {
	LookupTXT(ctx context.Context, name string) ([]string, error)
	LookupCNAME(ctx context.Context, host string) (string, error)
	LookupAddr(ctx context.Context, host string) ([]string, error)
}

// TLSProber reports the certificate notAfter for a hostname (the TLS proof); the real one dials TLS 1.2+ with
// SNI and verification and returns the leaf's NotAfter.
type TLSProber interface {
	NotAfter(ctx context.Context, host string) (time.Time, error)
}

// SystemProber is the production prober; it verifies the chain (tls.Dial's default verification) and reports
// the leaf certificate's NotAfter.
type SystemProber struct{}

// NotAfter dials host:443 and returns the leaf certificate's expiry.
func (SystemProber) NotAfter(ctx context.Context, host string) (time.Time, error) {
	d := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := tls.DialWithDialer(d, "tcp", net.JoinHostPort(host, "443"), &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: host,
	})
	if err != nil {
		return time.Time{}, err
	}
	defer conn.Close()
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return time.Time{}, errors.New("no peer certificate")
	}
	return certs[0].NotAfter, nil
}

// DNSResult is one verification attempt; Matched is true only when the TXT token matches AND the host points at us
// (CNAME to stores.<base> or any A/AAAA record, per Decision 3).
type DNSResult struct {
	TXTFound   bool
	CNAMEMatch bool
	AddrFound  bool
	Matched    bool
}

// VerifyDNS runs one attempt. TXT lookup errors (NXDOMAIN, timeout) propagate so the caller can distinguish them;
// CNAME/A errors are treated as "not pointed yet" (a transient DNS outage must not be read as a proof failure).
func VerifyDNS(ctx context.Context, r Resolver, host, token, baseDomain string) (DNSResult, error) {
	if ctx == nil || r == nil || !validHostname(host) || !validToken(token) || !validHostname(baseDomain) {
		return DNSResult{}, errors.New("invalid dns verification request")
	}
	var out DNSResult
	txts, err := r.LookupTXT(ctx, "_lc-verify."+host)
	if err != nil {
		return DNSResult{}, err
	}
	for _, t := range txts {
		if t == token {
			out.TXTFound = true
			break
		}
	}
	if cname, err := r.LookupCNAME(ctx, host); err == nil {
		out.CNAMEMatch = strings.EqualFold(strings.TrimSuffix(cname, "."), "stores."+baseDomain)
	}
	if addrs, err := r.LookupAddr(ctx, host); err == nil && len(addrs) > 0 {
		out.AddrFound = true
	}
	out.Matched = out.TXTFound && (out.CNAMEMatch || out.AddrFound)
	return out, nil
}

// WithinDeadline reports whether the request is still inside its 72h verification window.
func WithinDeadline(requestedAt, now time.Time) bool {
	return !requestedAt.IsZero() && !now.Before(requestedAt) && now.Sub(requestedAt) <= verifyWindow
}

// BackoffDelay is the DNS retry delay: 1 minute, doubling per consecutive failure, capped at 30 minutes.
func BackoffDelay(failures int) time.Duration {
	if failures < 0 {
		failures = 0
	}
	d := time.Minute
	for i := 0; i < failures && d < maxBackoff; i++ {
		d *= 2
		if d > maxBackoff {
			d = maxBackoff
		}
	}
	return d
}

// DNSCheck is one pending row for the worker sweep.
type DNSCheck struct {
	DomainID       string
	Origin         string
	Hostname       string
	Token          string
	State          string
	VerifyDeadline time.Time
	LastCheckedAt  *time.Time
	DNSFailures    int
}

// AdvanceResult is the worker transition outcome (state after the move, plus expiry).
type AdvanceResult struct {
	DomainID    string
	State       string
	Expired     bool
	DNSFailures int
}

// VerifyPending sweeps both queues once: REQUESTED rows still inside their window get a DNS attempt (respecting the
// backoff), and TLS_PENDING rows get a TLS probe that completes them to ACTIVE with the certificate's notAfter.
// The q here is the commerce_storefront_registrar pool (EXECUTE on the worker definers); baseDomain is the platform
// base zone (LC_STORE_BASE_DOMAIN) used to build the CNAME target. It returns counts of DNS attempts and TLS
// completions; the first hard error aborts.
func VerifyPending(ctx context.Context, q Querier, r Resolver, p TLSProber, now time.Time, baseDomain string) (dnsAttempts, tlsCompleted int, err error) {
	if ctx == nil || q == nil || r == nil || p == nil || !validHostname(strings.ToLower(baseDomain)) {
		return 0, 0, errVerifyInvalid
	}
	dnsAttempts, err = sweepDNS(ctx, q, r, now, baseDomain)
	if err != nil {
		return dnsAttempts, 0, err
	}
	tlsCompleted, err = sweepTLS(ctx, q, p, now)
	return dnsAttempts, tlsCompleted, err
}

func sweepDNS(ctx context.Context, q Querier, r Resolver, now time.Time, baseDomain string) (int, error) {
	rows, err := queryDNSChecks(ctx, q)
	if err != nil {
		return 0, err
	}
	attempts := 0
	for _, row := range rows {
		if !WithinDeadline(row.VerifyDeadline, now) {
			continue // expired: no more attempts, the merchant re-requests
		}
		if row.LastCheckedAt != nil && now.Sub(*row.LastCheckedAt) < BackoffDelay(row.DNSFailures) {
			continue // backoff not yet elapsed
		}
		result, err := VerifyDNS(ctx, r, row.Hostname, row.Token, baseDomain)
		attempts++
		if err != nil {
			// A DNS outage is recorded as a failure but never advances state; NXDOMAIN/timeout keep the row REQUESTED.
			_, _ = dnsAdvance(ctx, q, row.DomainID, false)
			continue
		}
		if !result.Matched {
			_, _ = dnsAdvance(ctx, q, row.DomainID, false)
			continue
		}
		adv, err := dnsAdvance(ctx, q, row.DomainID, true) // REQUESTED -> OWNERSHIP_PENDING
		if err != nil {
			return attempts, err
		}
		if adv.State == "OWNERSHIP_PENDING" {
			_, err = dnsAdvance(ctx, q, row.DomainID, true) // OWNERSHIP_PENDING -> TLS_PENDING
			if err != nil {
				return attempts, err
			}
		}
	}
	return attempts, nil
}

func sweepTLS(ctx context.Context, q Querier, p TLSProber, now time.Time) (int, error) {
	rows, err := queryTLSProbes(ctx, q)
	if err != nil {
		return 0, err
	}
	completed := 0
	for _, row := range rows {
		notAfter, err := p.NotAfter(ctx, row.Hostname)
		if err != nil || notAfter.IsZero() || !notAfter.After(now) || notAfter.After(now.Add(MaxProofLifetime)) {
			continue // certificate not yet issued or out of the proof window
		}
		if _, err := tlsComplete(ctx, q, row.DomainID, notAfter); err != nil {
			return completed, err
		}
		completed++
	}
	return completed, nil
}

// MaxProofLifetime caps the TLS valid_until exactly like the operator bind path (a public CA certificate lives at
// most 398 days, rounded up to 400).
const MaxProofLifetime = 400 * 24 * time.Hour

func dnsAdvance(ctx context.Context, q Querier, domainID string, matched bool) (AdvanceResult, error) {
	var raw []byte
	// control.store_domain_dns_advance (0106): EXECUTE commerce_storefront_registrar.
	if err := q.QueryRow(ctx, `SELECT control.store_domain_dns_advance($1::uuid,$2)`, domainID, matched).Scan(&raw); err != nil {
		return AdvanceResult{}, mapError(err)
	}
	var out AdvanceResult
	if !strictDecode(raw, &out) || out.DomainID != domainID {
		return AdvanceResult{}, ErrUnavailable
	}
	return out, nil
}

// tlsResult is the jsonb control.store_domain_tls_complete returns.
type tlsResult struct {
	DomainID string `json:"domain_id"`
	State    string `json:"state"`
	Version  int64  `json:"version"`
}

func tlsComplete(ctx context.Context, q Querier, domainID string, notAfter time.Time) (string, error) {
	var raw []byte
	// control.store_domain_tls_complete (0106): EXECUTE commerce_storefront_registrar; TLS_PENDING -> ACTIVE.
	if err := q.QueryRow(ctx, `SELECT control.store_domain_tls_complete($1::uuid,$2)`, domainID, notAfter.UTC()).Scan(&raw); err != nil {
		return "", mapError(err)
	}
	var out tlsResult
	if !strictDecode(raw, &out) || out.DomainID != domainID || out.State != "ACTIVE" || out.Version < 1 {
		return "", ErrUnavailable
	}
	return out.State, nil
}

func queryDNSChecks(ctx context.Context, q Querier) ([]DNSCheck, error) {
	rows, err := q.Query(ctx, `SELECT domain_id,origin,hostname,token,state,verify_deadline,last_checked_at,dns_failures FROM control.next_store_domain_dns_check()`)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	var out []DNSCheck
	for rows.Next() {
		var d DNSCheck
		if err := rows.Scan(&d.DomainID, &d.Origin, &d.Hostname, &d.Token, &d.State, &d.VerifyDeadline, &d.LastCheckedAt, &d.DNSFailures); err != nil {
			return nil, mapError(err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// tlsProbeRow is one TLS_PENDING row (domain id + hostname derived from origin).
type tlsProbeRow struct {
	DomainID string
	Hostname string
}

func queryTLSProbes(ctx context.Context, q Querier) ([]tlsProbeRow, error) {
	rows, err := q.Query(ctx, `SELECT domain_id,origin FROM control.next_store_domain_tls_probe()`)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	var out []tlsProbeRow
	for rows.Next() {
		var id, origin string
		if err := rows.Scan(&id, &origin); err != nil {
			return nil, mapError(err)
		}
		out = append(out, tlsProbeRow{DomainID: id, Hostname: strings.TrimPrefix(origin, "https://")})
	}
	return out, rows.Err()
}

var errVerifyInvalid = errors.New("invalid verify request")
