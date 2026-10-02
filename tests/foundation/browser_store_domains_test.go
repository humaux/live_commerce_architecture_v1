//go:build browser

package foundation_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/buyerhttp"
	"livecommerce/internal/command"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/identity"
	"livecommerce/internal/identityhttp"
	"livecommerce/internal/oidclogin"
	"livecommerce/internal/storefrontdomains"
)

// R5 store-domains KEY acceptance gate (BROWSER; MOCK: signed MOCK IdP, synthetic TLS/CONNECT edge for the buyer
// hosts, a scripted DNS resolver + TLS prober standing in for public DNS and certificate issuance).
//
// Production shape: NO owner-seeded store, publication or domain row exists for the merchant who signs in (a fresh
// principal with an external identity and zero memberships; asserted before the browsers start). The merchant runs
// the real onboarding wizard with an English store name (en, desktop): the handle is previewed live, the receipt
// shows https://<handle>.<base> and PG holds the ACTIVE platform-subdomain row. The merchant publishes from the
// Settings card (zh-TW, 390px) and an anonymous buyer browser opens the store at the platform origin through the
// test edge. The merchant then requests a custom domain (zh-CN, desktop), sees the DNS instructions, the runner's
// scripted DNS answers the TXT + CNAME proof, one VerifyPending sweep turns the row ACTIVE, the buyer is served on
// the custom host, and the platform subdomain must answer 301 to the custom origin (contract: 架构 §7 / Decision 3;
// RED while internal/buyerhttp's primary-origin endpoint has no storefront caller). Suspend, detach and unpublish
// close it again; every state change is read back from PG, including the audit trail.
func TestBrowserStoreDomains(t *testing.T) {
	if os.Getenv("LC_BROWSER_STORE_DOMAINS_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-store-domains")
	}
	// bhSetup minus bhPublish: the buyer HTTP service is production code; admission facts are NOT seeded.
	b := bcSetup(t)
	h := bhHarness{bcHarness: b, key: brToken(), origin: "https://gate.lctest.example"}
	handler, err := buyerhttp.New(context.Background(), b.a.issuer, b.a.runtime, b.service, h.key, time.Hour)
	if err != nil {
		t.Fatal("buyer HTTP constructor failed")
	}
	h.server = httptest.NewServer(handler)
	t.Cleanup(h.server.Close)
	owner := h.f.owner
	if n := countRows(t, owner, `SELECT (SELECT count(*) FROM control.storefront_publications)+(SELECT count(*) FROM control.storefront_domains)`); n != 0 {
		t.Fatalf("%d storefront publication/domain rows exist before the gate: the production-shape gate must start with none", n)
	}
	const baseDomain = "lctest.example"
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := os.MkdirTemp(filepath.Join(root, "output/playwright"), "store-domains-")
	if err != nil {
		t.Fatal(err)
	}
	// The verifier login the DNS/TLS sweep runs on (lc_store_domain_verify grant shape, inherit_noset) — the same role
	// the production worker (P0-2: still unwired, see REVIEW-store-domains.md) would use.
	verifier := miPool(t, h.f, "commerce_storefront_verifier")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	adminOrigin := browserFront(t, listener.Addr().String())
	_, adminPort, _ := net.SplitHostPort(listener.Addr().String())
	_ = listener.Close()
	idp := newBrowserIDP(t, adminOrigin+"/api/auth/callback")
	_, _, authority := identityFixture(t)
	provider, err := oidclogin.New(ctx, oidclogin.Config{Issuer: idp.server.URL, ClientID: browserClientID, RedirectURL: idp.redirect, AllowLoopbackForTests: true})
	if err != nil {
		t.Fatal(err)
	}
	service, err := identity.New(authority, observedBrowserProvider{Provider: provider, t: t}, identity.Policy{
		ProviderKey:       "browser-store-domains-mock-v1",
		SessionTTL:        time.Hour,
		OnboardingEnabled: true,
		Currencies:        []string{"USD", "TWD"},
		StoreBaseDomain:   baseDomain,
	})
	if err != nil {
		t.Fatal(err)
	}
	// The merchant: a real principal the MOCK IdP subject resolves to, with NO membership and NO store grant — the
	// wizard must create everything. Nothing else about this principal is seeded.
	principal := randomUUID()
	mustExec(t, owner, `INSERT INTO identity.principals(id) VALUES($1)`, principal)
	mustExec(t, owner, `INSERT INTO identity.external_identities(issuer,subject,principal_id) VALUES($1,'browser-subject',$2)`, idp.server.URL, principal)
	adminKey := randomToken()
	private, err := identityhttp.NewHandler(service, adminKey)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/v1/identity/", private)
	mux.Handle("/", httpapi.NewHandler(h.f.runtime, httpapi.Options{SessionStoreList: true, StoreBaseDomain: baseDomain}))
	api := httptest.NewServer(mux)
	t.Cleanup(api.Close)

	// Runner-only controls (random key, ephemeral loopback): read-only DB facts, the scripted public-DNS answers, and
	// one sweep of the production VerifyPending worker library with the scripted resolver/prober.
	dns := &sdBrowserDNS{txt: map[string][]string{}, cname: map[string]string{}, addr: map[string][]string{}}
	probe := sdBrowserProber{notAfter: time.Now().Add(90 * 24 * time.Hour).UTC()}
	type sdFacts struct {
		Handle       string           `json:"handle"`
		Publications []map[string]any `json:"publications"`
		Domains      []map[string]any `json:"domains"`
		Audits       []string         `json:"audits"`
	}
	facts := func(store string) sdFacts {
		out := sdFacts{Publications: []map[string]any{}, Domains: []map[string]any{}, Audits: []string{}}
		if err := owner.QueryRow(ctx, `SELECT handle FROM control.stores WHERE id=$1`, store).Scan(&out.Handle); err != nil {
			t.Fatal(err)
		}
		prow, err := owner.Query(ctx, `SELECT published,version FROM control.storefront_publications WHERE store_id=$1`, store)
		if err != nil {
			t.Fatal(err)
		}
		for prow.Next() {
			var p bool
			var v int64
			_ = prow.Scan(&p, &v)
			out.Publications = append(out.Publications, map[string]any{"published": p, "version": v})
		}
		prow.Close()
		drow, err := owner.Query(ctx, `SELECT origin,state,version FROM control.storefront_domains WHERE store_id=$1 ORDER BY origin`, store)
		if err != nil {
			t.Fatal(err)
		}
		for drow.Next() {
			var o, s string
			var v int64
			_ = drow.Scan(&o, &s, &v)
			out.Domains = append(out.Domains, map[string]any{"origin": o, "state": s, "version": v})
		}
		drow.Close()
		arow, err := owner.Query(ctx, `SELECT action FROM ops.audit_events WHERE store_id=$1 AND action LIKE ANY(ARRAY['merchant.store_created','merchant.storefront%','merchant.domain%']) ORDER BY created_at,id`, store)
		if err != nil {
			t.Fatal(err)
		}
		out.Audits, err = pgx.CollectRows(arow, pgx.RowTo[string])
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	controlKey := randomToken()
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Gate-Key") != controlKey {
			http.Error(w, "forbidden", 403)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "GET" && r.URL.Path == "/db":
			store := r.URL.Query().Get("store")
			if !command.ValidID(store) {
				http.Error(w, "bad request", 400)
				return
			}
			_ = json.NewEncoder(w).Encode(facts(store))
		case r.Method == "POST" && r.URL.Path == "/dns":
			var in struct {
				Host        string `json:"host"`
				TXTName     string `json:"txt_name"`
				TXTValue    string `json:"txt_value"`
				CNAMETarget string `json:"cname_target"`
			}
			if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<14)).Decode(&in) != nil || in.Host == "" || in.TXTName == "" || in.TXTValue == "" || in.CNAMETarget == "" {
				http.Error(w, "bad request", 400)
				return
			}
			dns.answer(in.Host, in.TXTName, in.TXTValue, in.CNAMETarget)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		case r.Method == "POST" && r.URL.Path == "/verify":
			// One sweep of the production worker library (internal/storefrontdomains.VerifyPending) on the verifier
			// login: REQUESTED -> OWNERSHIP_PENDING -> TLS_PENDING via the scripted public DNS, then the scripted TLS
			// probe completes TLS_PENDING -> ACTIVE. Public DNS and certificate issuance are the MOCK seam.
			attempts, completed, renewed, err := storefrontdomains.VerifyPending(r.Context(), verifier, dns, probe, time.Now(), baseDomain)
			if err != nil {
				http.Error(w, "verify sweep failed", 502)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"dns_attempts": attempts, "tls_completed": completed, "tls_renewed": renewed})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(control.Close)

	cmd := exec.CommandContext(ctx, "node", "tests/storefront/store-domains-gate.mjs")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	defer func() {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}()
	cmd.Dir = root
	cmd.Env = browserEnvironment(map[string]string{
		"COMMERCE_IDENTITY_ENABLED": "1", "COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS": "1",
		"COMMERCE_PUBLIC_ORIGIN": adminOrigin, "COMMERCE_API_ORIGIN": api.URL,
		"COMMERCE_OIDC_ISSUER": idp.server.URL, "COMMERCE_BFF_KEY": adminKey,
		"COMMERCE_ONBOARDING_ENABLED": "1", "COMMERCE_ONBOARDING_CURRENCIES": "USD,TWD",
		"COMMERCE_BUYER_WEB_ENABLED": "1", "COMMERCE_BUYER_DEMO_LABEL": "1",
		"COMMERCE_BUYER_API_ORIGIN": h.server.URL, "COMMERCE_BUYER_BFF_KEY": h.key,
		"COMMERCE_BUYER_COOKIE_KEY": brToken(), "COMMERCE_BUYER_SESSION_TTL": "3600",
		"LC_JOINT_EVIDENCE": evidence, "LC_JOINT_ADMIN_PORT": adminPort, "LC_JOINT_BASE": baseDomain,
		"LC_JOINT_CONTROL": control.URL, "LC_JOINT_CONTROL_KEY": controlKey,
	})
	log := browserLog(t, filepath.Join(evidence, "browser.log"))
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Run(); err != nil {
		t.Fatalf("store-domains browser gate failed; evidence=%s", evidence)
	}
	data, err := os.ReadFile(filepath.Join(evidence, "result.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Cases        int      `json:"cases"`
		Locales      []string `json:"locales"`
		Viewports    []string `json:"viewports"`
		Audit        []string `json:"audit_expected"`
		Handle       string   `json:"handle"`
		Origin       string   `json:"origin"`
		CustomOrigin string   `json:"custom_origin"`
		StoreID      string   `json:"store_id"`
		TenantID     string   `json:"tenant_id"`
	}
	if json.Unmarshal(data, &result) != nil || result.Cases < 15 ||
		!reflect.DeepEqual(result.Locales, []string{"en", "zh-TW", "zh-CN"}) ||
		!reflect.DeepEqual(result.Viewports, []string{"desktop", "390px"}) ||
		!regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{1,28}[a-z0-9])$`).MatchString(result.Handle) ||
		result.Origin != "https://"+result.Handle+"."+baseDomain || !command.ValidID(result.StoreID) || !command.ValidID(result.TenantID) {
		t.Fatalf("missing exact browser gate results: %s", data)
	}
	t.Cleanup(func() { // the onboarding-created tenant's rows, so the serial harness stays row-free
		for _, q := range []string{`DELETE FROM control.storefront_domains WHERE tenant_id=$1`, `DELETE FROM control.storefront_publications WHERE tenant_id=$1`,
			`DELETE FROM ops.audit_events WHERE tenant_id=$1 AND action LIKE ANY(ARRAY['merchant.store_created','merchant.storefront%','merchant.domain%'])`} {
			_, _ = owner.Exec(context.Background(), q, result.TenantID)
		}
	})
	// Independent PostgreSQL readback of what the browsers and the sweep did.
	after := facts(result.StoreID)
	if fmt.Sprint(after.Audits) != fmt.Sprint(result.Audit) {
		t.Fatalf("audit trail in PG differs from the actions the gate drove:\n got %v\nwant %v", after.Audits, result.Audit)
	}
	if after.Handle != result.Handle {
		t.Fatalf("stores.handle = %q, the wizard receipt and preview said %q", after.Handle, result.Handle)
	}
	if len(after.Publications) != 1 || after.Publications[0]["published"] != false || after.Publications[0]["version"].(int64) != 2 {
		t.Fatalf("final publication row (one publish + one unpublish): %v", after.Publications)
	}
	// Final domain rows: the platform subdomain ACTIVE (untouched by the merchant moves), the custom domain DETACHED,
	// its one-time token long cleared (tls_complete clears it), evidence markers intact.
	if len(after.Domains) != 2 {
		t.Fatalf("final domain rows: %v", after.Domains)
	}
	stateBy := map[string]string{}
	for _, d := range after.Domains {
		stateBy[d["origin"].(string)] = d["state"].(string)
	}
	if stateBy[result.Origin] != "ACTIVE" || stateBy[result.CustomOrigin] != "DETACHED" {
		t.Fatalf("final domain states: %v (platform %q, custom %q)", after.Domains, result.Origin, result.CustomOrigin)
	}
	var evidenceRef string
	var token *string
	if err := owner.QueryRow(ctx, `SELECT evidence_ref,verification_token FROM control.storefront_domains WHERE origin=$1`, result.Origin).Scan(&evidenceRef, &token); err != nil || evidenceRef != "platform-subdomain" || token != nil {
		t.Fatalf("platform origin row readback: evidence=%q token=%v err=%v", evidenceRef, token, err)
	}
	if err := owner.QueryRow(ctx, `SELECT evidence_ref,verification_token FROM control.storefront_domains WHERE origin=$1`, result.CustomOrigin).Scan(&evidenceRef, &token); err != nil || evidenceRef != "merchant-tls" || token != nil {
		t.Fatalf("custom origin row readback: evidence=%q token=%v err=%v", evidenceRef, token, err)
	}
	// Every row came from the writers: each state change has its audit row, attributed to the creating principal.
	if n := countRows(t, owner, `SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND principal_id=$2 AND action LIKE ANY(ARRAY['merchant.store_created','merchant.storefront%','merchant.domain%'])`, result.StoreID, principal); n != len(result.Audit) {
		t.Fatalf("audit rows attributed to the creating principal = %d, want %d", n, len(result.Audit))
	}
	// The merchant holds exactly one tenant, created by the wizard, and a real session signed by the MOCK IdP.
	if n := countRows(t, owner, `SELECT count(*) FROM identity.memberships WHERE principal_id=$1`, principal); n != 1 {
		t.Fatalf("wizard-created memberships = %d, want 1", n)
	}
	if countRows(t, owner, `SELECT count(*) FROM identity.sessions WHERE principal_id=$1 AND audience='merchant'`, principal) < 1 {
		t.Fatal("no real merchant session")
	}
	idp.mu.Lock()
	exchanges := idp.exchanges
	idp.mu.Unlock()
	if exchanges < 1 {
		t.Fatal("expected a real signed MOCK IdP exchange")
	}
	t.Logf("PASS: onboarding wizard (en) -> handle + ACTIVE platform subdomain -> publish (zh-TW 390px) -> buyer on https://<handle>.%s -> custom domain (zh-CN) -> scripted DNS + VerifyPending sweep -> ACTIVE -> 301 contract -> suspend/detach/unpublish; cases=%d; evidence=%s", baseDomain, result.Cases, evidence)
}

// sdBrowserDNS is the gate's scripted public DNS: the runner posts the TXT/CNAME answers the merchant was told to
// create; lookups with no scripted answer NXDOMAIN, exactly like a record the merchant has not created yet.
type sdBrowserDNS struct {
	mu    sync.Mutex
	txt   map[string][]string
	cname map[string]string
	addr  map[string][]string
}

func (d *sdBrowserDNS) answer(host, txtName, txtValue, cnameTarget string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.txt[txtName] = []string{txtValue}
	d.cname[host] = cnameTarget
}

func (d *sdBrowserDNS) LookupTXT(_ context.Context, name string) ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if v, ok := d.txt[name]; ok {
		return v, nil
	}
	return nil, &net.DNSError{IsNotFound: true, Name: name}
}

func (d *sdBrowserDNS) LookupCNAME(_ context.Context, host string) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if v, ok := d.cname[host]; ok {
		return v, nil
	}
	return "", &net.DNSError{IsNotFound: true, Name: host}
}

func (d *sdBrowserDNS) LookupAddr(_ context.Context, host string) ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if v, ok := d.addr[host]; ok {
		return v, nil
	}
	return nil, &net.DNSError{IsNotFound: true, Name: host}
}

// sdBrowserProber is the scripted certificate-issuance seam: every TLS_PENDING host presents a certificate valid
// for 90 days (the gate's own edge cert), so the sweep's TLS half completes what the DNS half proved.
type sdBrowserProber struct{ notAfter time.Time }

func (p sdBrowserProber) NotAfter(_ context.Context, _ string) (time.Time, error) {
	return p.notAfter, nil
}
