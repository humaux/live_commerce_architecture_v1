package foundation_test

// R5 unit store-domains, INDEPENDENT adversarial gate (test author != implementer). Written from
// docs/delivery/units/store-domains.md (Decisions 1-6) and 架构.md §7, against the implementation.
// SDW07-SDW09, SDW11-SDW13, SDW15 encode contract behaviour the current code violates (they are RED until the
// implementer/integrator fixes the finding; see REVIEW-store-domains.md P0-2, P0-4, P1-1, P1-3, P1-4, P2-1, P2-8).
// SDW10 and SDW14 pass on current code and are the mutation targets for the red/green proof.
//
//	SDW07 TLS ask admit/deny incl. expired valid_until      SDW08 DNS verifier refuses foreign A / proxied shape
//	SDW09 ask rate limit must not starve a legit allow      SDW10 ask HTTP matrix fail-closed, no echo
//	SDW11 handle change detaches old origin; no re-issue    SDW12 Caddy catch-all enables tls on_demand
//	SDW13 the verify sweep has a production runner          SDW14 reserved/under-base/duplicate refusals
//	SDW15 merchant self-service cannot move the platform origin

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/command"
	"livecommerce/internal/storefrontdomains"
	"livecommerce/internal/tlsask"
)

// ---------------------------------------------------------------- ask matrix (PG)

func TestStoreDomainsSDW07AskAdmitDenyMatrix(t *testing.T) {
	s := sdSetup(t)
	ctx := context.Background()
	o := s.b.owner
	ask := func(host string) bool {
		var allowed bool
		if err := s.b.runtime.QueryRow(ctx, `SELECT control.resolve_storefront_ask($1)`, host).Scan(&allowed); err != nil {
			t.Fatal(err)
		}
		return allowed
	}
	bind := func(state, validUntil string) string {
		host := sdHost()
		mustExec(t, o, `INSERT INTO control.storefront_domains(tenant_id,store_id,origin,state,ownership_verified_at,tls_verified_at,valid_until,evidence_ref)
			VALUES($1,$2,$3,$4,clock_timestamp()-interval '2 hour',clock_timestamp()-interval '1 hour',clock_timestamp()+`+validUntil+`,'sdw07 synthetic')`,
			s.f.tenant, s.f.store, "https://"+host, state)
		return host
	}

	// Denied classes (green on current code): unknown, REQUESTED, SUSPENDED, DETACHED hosts get no certificate.
	if ask(sdHost()) {
		t.Error("ask admitted an unknown host")
	}
	for _, state := range []string{"REQUESTED", "SUSPENDED", "DETACHED"} {
		if host := bind(state, `interval '30 days'`); ask(host) {
			t.Errorf("ask admitted a %s host %s", state, host)
		}
	}
	// Admitted classes (green): TLS_PENDING (issuance in flight) and ACTIVE inside its validity.
	if host := bind("TLS_PENDING", `interval '30 days'`); !ask(host) {
		t.Errorf("ask denied a TLS_PENDING host %s", host)
	}
	active := bind("ACTIVE", `interval '30 days'`)
	if !ask(active) {
		t.Errorf("ask denied an ACTIVE host %s", active)
	}
	// Host normalization: the Go endpoint lower-cases before calling (internal/tlsask.Allow); the raw SQL
	// must still fail closed on a non-canonical host (upper case, trailing dot) rather than admit it.
	if ask(strings.ToUpper(active)) {
		t.Errorf("raw SQL admitted the upper-case form of an ACTIVE host %s", active)
	}
	if ask(active + ".") {
		t.Errorf("ask admitted the trailing-dot form of %s", active)
	}

	// P2-1 (REVIEW): an ACTIVE row past its valid_until is no longer served by the resolver — the ask must
	// fail closed too, or Caddy keeps renewing a certificate for a host the platform refuses to serve.
	expired := bind("ACTIVE", `interval '-30 minutes'`) // past, but still after both verified_at (0020 CHECK)
	if ask(expired) {
		t.Errorf("P2-1: ask admitted %s whose ACTIVE row expired (valid_until in the past)", expired)
	}
}

// ---------------------------------------------------------------- DNS verifier (Go)

// sdFakeResolver is the DNS seam (MOCK): scripted TXT/CNAME/Addr answers per name.
type sdFakeResolver struct {
	txt    map[string][]string
	txtErr map[string]error
	cname  map[string]string
	addrs  map[string][]string
}

func (r sdFakeResolver) LookupTXT(_ context.Context, name string) ([]string, error) {
	if err := r.txtErr[name]; err != nil {
		return nil, err
	}
	v, ok := r.txt[name]
	if !ok {
		return nil, errors.New("NXDOMAIN")
	}
	return v, nil
}

func (r sdFakeResolver) LookupCNAME(_ context.Context, host string) (string, error) {
	if v, ok := r.cname[host]; ok {
		return v, nil
	}
	return host + ".", nil // no CNAME in the answer: the name is canonical for itself
}

func (r sdFakeResolver) LookupAddr(_ context.Context, host string) ([]string, error) {
	return r.addrs[host], nil
}

func TestStoreDomainsSDW08DNSVerifierRefusesForeignAddr(t *testing.T) {
	const base = "example.com"
	token := strings.Repeat("A", 43)                 // 32 zero bytes, base64url-strict valid (the _lc-verify TXT value shape)
	edge := []string{"203.0.113.10", "203.0.113.11"} // the addresses stores.<base> (the platform edge) answers with
	host := "shop.merchant.example"
	verify := func(r sdFakeResolver) storefrontdomains.DNSResult {
		out, err := storefrontdomains.VerifyDNS(context.Background(), r, host, token, base)
		if err != nil {
			t.Fatalf("VerifyDNS: %v", err)
		}
		return out
	}

	// Green controls: the CNAME proof and no-proof refusals.
	if out := verify(sdFakeResolver{
		txt:   map[string][]string{"_lc-verify." + host: {token}},
		cname: map[string]string{host: "stores." + base + "."},
	}); !out.Matched || !out.TXTFound || !out.CNAMEMatch {
		t.Errorf("CNAME-to-stores proof must match: %+v", out)
	}
	if out := verify(sdFakeResolver{
		txt:   map[string][]string{"_lc-verify." + host: {"someone-elses-token"}},
		cname: map[string]string{host: "stores." + base + "."},
	}); out.Matched {
		t.Errorf("a foreign TXT token must never match: %+v", out)
	}
	if out := verify(sdFakeResolver{
		txt:   map[string][]string{"_lc-verify." + host: {token}},
		cname: map[string]string{host: "stores.evil.example."},
	}); out.Matched {
		t.Errorf("a CNAME to an attacker zone must never match: %+v", out)
	}

	// P1-1a (REVIEW): Decision 3 admits an apex A record only when it points AT THE PLATFORM EDGE
	// (the same addresses stores.<base> serves). Any other address is the merchant pointing the
	// "verified" host at their own server.
	if out := verify(sdFakeResolver{
		txt:   map[string][]string{"_lc-verify." + host: {token}},
		addrs: map[string][]string{host: edge, "stores." + base: edge},
	}); !out.Matched {
		t.Errorf("apex A record at the edge addresses must match: %+v", out)
	}
	if out := verify(sdFakeResolver{
		txt:   map[string][]string{"_lc-verify." + host: {token}},
		addrs: map[string][]string{host: {"192.0.2.99"}, "stores." + base: edge}, // the merchant's own VPS
	}); out.Matched {
		t.Errorf("P1-1: an A record that is not the platform edge must not verify: %+v", out)
	}

	// P1-1b (REVIEW, brief P18): a proxied CNAME (Cloudflare orange-cloud) answers with no CNAME and the
	// proxy's anycast addresses; the check must refuse it, not read it as "pointed at us".
	if out := verify(sdFakeResolver{
		txt:   map[string][]string{"_lc-verify." + host: {token}},
		addrs: map[string][]string{host: {"104.16.10.10", "104.16.11.10"}, "stores." + base: edge},
	}); out.Matched {
		t.Errorf("P1-1/P18: a proxied (anycast A, no CNAME) host must be refused: %+v", out)
	}
}

// ---------------------------------------------------------------- ask rate limit (Go)

// sdAskQuerier fakes control.resolve_storefront_ask: admit only names on the allow list.
type sdAskQuerier struct {
	admit map[string]bool
	calls *int
}

type sdAskRow struct {
	host string
	q    sdAskQuerier
}

func (r sdAskRow) Scan(dest ...any) error {
	(*r.q.calls)++
	if p, ok := dest[0].(*bool); ok {
		*p = r.q.admit[r.host]
		return nil
	}
	return errors.New("unexpected scan target")
}

func (q sdAskQuerier) QueryRow(_ context.Context, _ string, args ...any) pgx.Row {
	return sdAskRow{host: args[0].(string), q: q}
}

func TestStoreDomainsSDW09AskRateLimitFairness(t *testing.T) {
	calls := new(int)
	q := sdAskQuerier{admit: map[string]bool{"legit.merchant.example": true}}
	q.calls = calls
	svc, err := tlsask.New(q, 3, time.Minute, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	// A scanner burns the whole per-minute budget on random unknown hosts (what a random-SNI flood
	// through the public catch-all does to Caddy's ask calls).
	for _, host := range []string{"scan1.example", "scan2.example", "scan3.example"} {
		if ok, err := svc.Allow(ctx, host); err != nil || ok {
			t.Fatalf("unknown host %s: ok=%v err=%v, want a clean deny", host, ok, err)
		}
	}
	// P1-3 (REVIEW): the denied flood must not starve a certificate-eligible host — the edge's asks for a
	// real merchant domain arriving in the same minute are 404 ("not eligible") and Caddy issues nothing.
	ok, err := svc.Allow(ctx, "legit.merchant.example")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf("P1-3: a certificate-eligible host was denied after %d unknown-host asks exhausted the shared budget (db calls=%d)", 3, *calls)
	}
}

// ---------------------------------------------------------------- ask HTTP matrix (Go)

func TestStoreDomainsSDW10AskHTTPMatrix(t *testing.T) {
	calls := new(int)
	q := sdAskQuerier{admit: map[string]bool{"live.merchant.example": true}}
	q.calls = calls
	svc, err := tlsask.New(q, 60, time.Minute, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(svc.Handler())
	t.Cleanup(server.Close)
	client := server.Client()
	get := func(path string) (int, string) {
		res, err := client.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<16))
		return res.StatusCode, string(body)
	}
	// The one admitted host answers 200; everything else fails closed.
	if code, _ := get("/internal/tls-ask?domain=live.merchant.example"); code != http.StatusOK {
		t.Errorf("admitted host: %d, want 200", code)
	}
	denied := map[string]int{
		"/internal/tls-ask?domain=unknown.example":              http.StatusNotFound,
		"/internal/tls-ask?domain=live.merchant.example.":       http.StatusNotFound, // trailing dot
		"/internal/tls-ask?domain=..%2fetc":                     http.StatusNotFound,
		"/internal/tls-ask":                                     http.StatusUnprocessableEntity,
		"/internal/tls-ask?foo=bar":                             http.StatusUnprocessableEntity,
		"/internal/tls-ask?domain=a.example&domain=b.example":   http.StatusUnprocessableEntity,
		"/internal/tls-ask?domain=a.example&extra=1":            http.StatusUnprocessableEntity,
		"/internal/tls-ask?domain=" + strings.Repeat("a", 2100): http.StatusUnprocessableEntity,
		"/internal/tls-ask?domain=%ZZ":                          http.StatusUnprocessableEntity,
	}
	for path, want := range denied {
		if code, _ := get(path); code != want {
			t.Errorf("GET %s: %d, want %d", path, code, want)
		}
	}
	// No information leak: a denied answer never echoes the probed name (no "known/unknown" oracle in the body).
	code, body := get("/internal/tls-ask?domain=secret-probe.merchant.example")
	if code != http.StatusNotFound || strings.Contains(body, "secret-probe") {
		t.Errorf("denied answer leaks the probed host: %d %q", code, body)
	}
	// Non-GET is refused.
	res, err := client.Post(server.URL+"/internal/tls-ask?domain=live.merchant.example", "text/plain", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST: %d, want 405", res.StatusCode)
	}
}

// ---------------------------------------------------------------- handle change (PG)

func TestStoreDomainsSDW11HandleChangeDetachesOldOrigin(t *testing.T) {
	s := sdSetup(t)
	o := s.b.owner
	name := "Handle Move " + randomUUID()[:8]
	slug := strings.ToLower(strings.ReplaceAll(name, " ", "-"))

	storeA := randomUUID()
	mustExec(t, o, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,$3,'USD')`, s.f.tenant, storeA, name)
	if h := handleOf(t, o, storeA); h != slug {
		t.Fatalf("store A handle = %q, want %q", h, slug)
	}
	_, originA := platformOrigin(t, o, storeA, s.base) // ACTIVE https://<slug>.example.com

	// The operator changes the handle (Decision 1: post-publish changes are operator-only).
	mustExec(t, o, `UPDATE control.stores SET handle=$1 WHERE id=$2`, slug+"-x", storeA)

	// P1-4a (REVIEW): "Changing it detaches the old platform origin (no silent redirect chains)" — the old
	// ACTIVE row must not keep resolving the store under its previous address.
	if state, _ := domainState(t, o, originA); state == "ACTIVE" {
		t.Errorf("P1-4: the old platform origin %s is still ACTIVE after the handle change", originA)
	}

	// A new store B takes the freed handle at onboarding.
	storeB := randomUUID()
	mustExec(t, o, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,$3,'USD')`, s.f.tenant, storeB, name)
	if h := handleOf(t, o, storeB); h != slug {
		t.Fatalf("store B handle = %q, want the freed %q", h, slug)
	}
	// P1-4b (REVIEW): B's platform ensure must not report success while the origin row still belongs to A.
	_, originB := platformOrigin(t, o, storeB, s.base)
	var rowStore string
	if err := o.QueryRow(context.Background(), `SELECT store_id::text FROM control.storefront_domains WHERE origin=$1`, originB).Scan(&rowStore); err != nil {
		t.Fatal(err)
	}
	if rowStore != storeB {
		t.Errorf("P1-4: %s was reported to store B as its address but the ACTIVE row belongs to store %s", originB, rowStore)
	}
}

// ---------------------------------------------------------------- deploy wiring (static)

// caddyBlock returns the body of the first top-level block whose opener line equals opener (brace-matched).
func caddyBlock(t *testing.T, text, opener string) string {
	t.Helper()
	start := strings.Index(text, opener)
	if start < 0 {
		t.Fatalf("Caddyfile has no %q block", opener)
	}
	depth := 0
	for i := start; i < len(text); i++ {
		switch text[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[start:i]
			}
		}
	}
	t.Fatalf("unbalanced braces after %q", opener)
	return ""
}

func TestStoreDomainsSDW12CaddyOnDemandEnabled(t *testing.T) {
	raw, err := os.ReadFile("../../deploy/caddy/Caddyfile")
	if err != nil {
		t.Fatal(err)
	}
	// P0-4 (REVIEW): the global on_demand_tls block only CONFIGURES on-demand TLS; the catch-all site that
	// serves <handle>.<base> and merchant domains must ENABLE it in its own tls directive, or Caddy has no
	// automation policy for those hosts and every handshake fails at the edge.
	catchAll := caddyBlock(t, string(raw), "\nhttps:// {")
	if !regexp.MustCompile(`(?m)^\s*tls\s*\{[^}]*\bon_demand\b`).MatchString(catchAll) {
		t.Errorf("P0-4: the catch-all https:// site does not enable on-demand TLS (no `tls { on_demand }`); no certificate is ever issued for platform subdomains or merchant domains")
	}
	// The ask wiring stays as Decision 4 describes (this half is green on current code).
	global := caddyBlock(t, string(raw), "on_demand_tls {")
	if !strings.Contains(global, "ask http://127.0.0.1:8080/internal/tls-ask?domain={host}") {
		t.Errorf("the on_demand_tls ask must target the internal ask endpoint, got:\n%s", global)
	}
}

func TestStoreDomainsSDW13VerifySweepHasProductionRunner(t *testing.T) {
	// P0-2 (REVIEW, ruled): Decision 3's "worker job verifies DNS with backoff ... a TLS probe then moves it to
	// ACTIVE" needs a runner in the deployed system — a merchant domain must be able to leave REQUESTED in
	// production. The binding ruling fixes this as a River periodic job in the existing claims-worker, NOT a
	// `store-admin domain-verify` CLI (that CLI is explicitly forbidden). Assert the real runner is wired.
	main, err := os.ReadFile("../../cmd/claims-worker/main.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(main), "storefrontdomains.NewWorker") ||
		!strings.Contains(string(main), "storefrontdomains.PeriodicJob()") {
		t.Errorf(`P0-2: cmd/claims-worker does not register the store-domain verify sweep (NewWorker + PeriodicJob); VerifyPending has no production runner`)
	}
	// The forbidden operator one-shot must not exist (the fix is the worker, not a store-admin subcommand).
	admin, err := os.ReadFile("../../cmd/store-admin/main.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(admin), `"domain-verify"`) {
		t.Errorf(`P0-2: cmd/store-admin must not claim a "domain-verify" subcommand; the sweep runs as the claims-worker River job`)
	}
}

// ---------------------------------------------------------------- refusals (mutation targets, green)

func TestStoreDomainsSDW14RefusalsHold(t *testing.T) {
	s := sdSetup(t)
	ctx := context.Background()
	o := s.b.owner

	// A reserved platform word can never become a handle, even via a direct insert.
	if _, err := o.Exec(ctx, `INSERT INTO control.stores(tenant_id,id,name,currency,handle) VALUES($1,$2,'sdw14','USD','admin')`, s.f.tenant, randomUUID()); pgCode(err) != "PT400" {
		t.Errorf("reserved handle insert: err=%v, want PT400", err)
	}
	// The unique index is the concurrency backstop for handle assignment.
	handle := "sdw14-" + randomUUID()[:8]
	mustExec(t, o, `INSERT INTO control.stores(tenant_id,id,name,currency,handle) VALUES($1,$2,'sdw14','USD',$3)`, s.f.tenant, randomUUID(), handle)
	if _, err := o.Exec(ctx, `INSERT INTO control.stores(tenant_id,id,name,currency,handle) VALUES($1,$2,'sdw14','USD',$3)`, s.f.tenant, randomUUID(), handle); pgCode(err) != "23505" {
		t.Errorf("duplicate handle insert: err=%v, want 23505", err)
	}
	// A merchant can never self-bind a host under the platform base zone.
	if _, err := s.request(s.f.token, s.f.store, "api."+s.base); !errors.Is(err, storefrontdomains.ErrReservedHostname) {
		t.Errorf("under-base host request: %v, want ErrReservedHostname", err)
	}
	if _, err := s.request(s.f.token, s.f.store, s.base); !errors.Is(err, storefrontdomains.ErrReservedHostname) {
		t.Errorf("bare base-zone request: %v, want ErrReservedHostname", err)
	}
	// The ask fails closed for a host nobody bound.
	var allowed bool
	if err := s.b.runtime.QueryRow(ctx, `SELECT control.resolve_storefront_ask($1)`, sdHost()).Scan(&allowed); err != nil || allowed {
		t.Errorf("ask for an unbound host = %v %v, want a clean deny", allowed, err)
	}
	// A merchant cannot bind a host another store of the same tenant already holds.
	host := sdHost()
	if _, err := s.request(s.f.token, s.f.store, host); err != nil {
		t.Fatal(err)
	}
	// The harness shares one database: a live REQUESTED row (verify_deadline set by the definer) would join
	// the next test's worker sweep, so this one is removed when the refusal checks are done.
	t.Cleanup(func() { mustExec(t, o, `DELETE FROM control.storefront_domains WHERE origin=$1`, "https://"+host) })
	if _, err := s.request(s.f.token, s.f.otherStore, host); !errors.Is(err, storefrontdomains.ErrDomainOwnedElsewhere) {
		t.Errorf("sibling store re-request: %v, want ErrDomainOwnedElsewhere", err)
	}
	// An invalid hostname is refused before any SQL.
	if _, err := s.request(s.f.token, s.f.store, "not a host"); !errors.Is(err, command.ErrInvalid) {
		t.Errorf("invalid hostname: %v, want ErrInvalid", err)
	}
}

// ---------------------------------------------------------------- platform origin vs self-service (PG)

func TestStoreDomainsSDW15PlatformOriginSurvivesSelfService(t *testing.T) {
	s := sdSetup(t)
	_, origin := platformOrigin(t, s.b.owner, s.f.store, s.base) // Decision 2: the platform-bound ACTIVE subdomain
	// P2-8 (REVIEW): the merchant self-service moves must refuse the platform-bound origin. Detaching it has no
	// self-service way back (request_merchant_domain refuses the whole base zone), so one click permanently
	// removes the store's own platform address; the suspend half takes the store off its platform name just the
	// same. The card renders both buttons on the platform row (StorefrontSettings.tsx moveable), so the definer
	// is the only wall.
	if _, err := s.suspend(s.f.token, s.f.store, origin); err == nil {
		t.Errorf("P2-8: merchant suspend of the platform origin %s succeeded; the platform-bound address must not be merchant-moveable", origin)
	}
	if _, err := s.detach(s.f.token, s.f.store, origin); err == nil {
		t.Errorf("P2-8: merchant detach of the platform origin %s succeeded; only the platform may unbind its own subdomain", origin)
	}
	if state, _ := domainState(t, s.b.owner, origin); state != "ACTIVE" {
		t.Errorf("P2-8: the platform origin is %s after merchant self-service moves, want ACTIVE", state)
	}
}
