//go:build browser

package foundation_test

// home-cod R5 (unit home-cod, migration 0107): the independent browser gate `TestBrowserHomeCod`, prefix `hcod`. Run through
// `bash scripts/dev/test-local.sh --browser-home-cod` (isolated PG 18, production storefront + admin Next builds).
//
// One real chain, two real browsers' worth of pages:
//   1. merchant (admin Next, zh-TW desktop) enables cash on delivery in the settings wizard: switch, per-order cap, whole-TWD surcharge,
//      carrier label; the values read back after a reload
//   2. four buyers on the production storefront (zh-TW/en x desktop/390px) check out HOME delivery with cash on delivery: the mode row
//      names the total plus the surcharge, the order is placed AWAITING_COLLECTION with collection PENDING and no payment step
//   3. the merchant ships order A (manual record, sf_express) and records the cash collected; finance gains one COD row
//   4. order A's buyer sees COLLECTED (still AWAITING_COLLECTION); B, C, D stay PENDING
// Evidence labels: BROWSER, MOCK (no PSP, no carrier API on this path — the carrier collects the cash and the merchant records it).
// PG facts are asserted around every phase. Owner-pool writes (disclosed fixtures): none beyond the harness stores.
// Depends-on: tcvEnv/tcvBuyer harness (taiwan_cvs_env_test.go), the shared browser helpers (browser_refund_fulfilment_test.go,
// browser_identity_chain_test.go), bcoShots (browser_checkout_offline_test.go). Used-by: nothing yet.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"livecommerce/internal/buyerhttp"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/identity"
	"livecommerce/internal/identityhttp"
	"livecommerce/internal/integrations/accounts"
	core "livecommerce/internal/integrations/core"
	"livecommerce/internal/oidclogin"
	"livecommerce/internal/platform"
)

func hcodRequire(t *testing.T) {
	t.Helper()
	if os.Getenv("LC_BROWSER_HOME_COD_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-home-cod")
	}
}

func TestBrowserHomeCod(t *testing.T) {
	hcodRequire(t)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()
	root, _ := filepath.Abs("../..")
	evidence := brfEvidence(t, root, "home-cod")
	const origin = "https://buyer.example"
	const unit, orderQty, surchargeMinor = int64(1250), int64(2), int64(5000)

	e := tcvNew(t, tcvOpts{origin: origin})
	e.grantCreator("orders:read", "fulfillment:write", "integration:manage", "integration:read")
	e.topUp()
	mustExec(t, e.p.f.owner, `UPDATE catalog.products SET name='Synthetic cash on delivery product',description='Synthetic acceptance fixture' WHERE id=$1`, e.p.stock.product.ID)
	total := unit * orderQty // 2500 minor, a whole TWD with the zero-fee home policy

	bffKey := base64.RawURLEncoding.EncodeToString(randomBytes(32))
	handler, err := buyerhttp.New(ctx, e.p.a.issuer, e.p.a.runtime, e.svc, bffKey, time.Hour)
	if err != nil {
		t.Fatalf("buyer HTTP constructor: %v", err)
	}
	buyerAPI := newHTTPServer(t, handler)
	stack := hcodStartAdmin(t, ctx, e, evidence)
	adminEnv := func(phase string, extra map[string]string) map[string]string {
		env := map[string]string{"LC_BROWSER_STORE": e.store(), "LC_HCOD_PHASE": phase, "LC_BROWSER_PHASE": phase}
		for k, v := range extra {
			env[k] = v
		}
		return env
	}

	// ---- phase 1: the merchant enables cash on delivery through the settings wizard ---------------------------------------------------
	if n := e.count(`SELECT count(*) FROM checkout.cash_on_delivery_settings WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()); n != 0 {
		t.Fatalf("setup: %d COD settings rows before the merchant configured anything", n)
	}
	brfPlaywright(t, ctx, stack, []string{"home-cod.spec.ts"}, adminEnv("settings", nil))
	st, out, raw := e.mcall(e.token(), "GET", "/v1/admin/stores/"+e.store()+"/cash-on-delivery-settings", "", "")
	if st != 200 || out["enabled"] != true || out["max_twd"] != float64(20000) || out["surcharge_twd"] != float64(50) || out["carrier"] != "black_cat" {
		t.Fatalf("the settings wizard did not store the COD settings: %d %s", st, raw)
	}
	if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='checkout.cash_on_delivery_settings_changed'`, e.store()); n < 1 {
		t.Error("the COD settings change was not audited")
	}

	// ---- phases 2-4: one long-lived buyer script, handed over through files -----------------------------------------------------------
	values := map[string]string{"COMMERCE_BUYER_WEB_ENABLED": "1", "COMMERCE_BUYER_API_ORIGIN": buyerAPI.URL, "COMMERCE_BUYER_DEMO_LABEL": "1", "COMMERCE_BUYER_BFF_KEY": bffKey,
		"COMMERCE_BUYER_COOKIE_KEY": base64.RawURLEncoding.EncodeToString(randomBytes(32)), "COMMERCE_BUYER_SESSION_TTL": "3600",
		"LC_HCOD_EVIDENCE": evidence, "LC_HCOD_ORIGIN": origin, "LC_HCOD_PRODUCT": e.p.stock.product.ID,
		"LC_HCOD_PRICE": "1250", "LC_HCOD_SURCHARGE": "5000"}
	cmd := exec.CommandContext(ctx, "node", "tests/storefront/home-cod-buyer.mjs")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	defer func() {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}()
	cmd.Dir = root
	cmd.Env = browserEnvironment(values)
	log := browserLog(t, filepath.Join(evidence, "home-cod-buyer.mjs.log"))
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	wait := func(step string) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Minute)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(filepath.Join(evidence, "ready-"+step)); err == nil {
				return
			}
			select {
			case err := <-exited:
				if _, statErr := os.Stat(filepath.Join(evidence, "ready-"+step)); statErr == nil {
					exited <- err
					return
				}
				t.Fatalf("the buyer script ended before %q: %v; evidence=%s", step, err, evidence)
			case <-time.After(150 * time.Millisecond):
			}
		}
		t.Fatalf("timed out waiting for %q; evidence=%s", step, evidence)
	}
	release := func(step string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(evidence, "go-"+step), []byte("1"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	wait("placed")
	raw, err = os.ReadFile(filepath.Join(evidence, "orders.json"))
	if err != nil {
		t.Fatal(err)
	}
	var orders map[string]struct {
		ID     string `json:"id"`
		Locale string `json:"locale"`
		Mobile bool   `json:"mobile"`
	}
	if err := json.Unmarshal(raw, &orders); err != nil || len(orders) != 4 {
		t.Fatalf("orders.json: %v %s", err, raw)
	}
	// PG facts after the buyer placed four COD orders through the UI: AWAITING_COLLECTION, committed stock, surcharge snapshot, no payment.
	for k, o := range orders {
		commercial, fulfilment, mode, collection, tot, surcharge := e.hcodRow(o.ID)
		if commercial != "AWAITING_COLLECTION" || fulfilment != "MANUAL_UNASSIGNED" || mode != "cash_on_delivery" ||
			collection == nil || *collection != "PENDING" || tot != total || surcharge != surchargeMinor {
			t.Errorf("order %s after placement: %s/%s/%s/%v total=%d surcharge=%d (want AWAITING_COLLECTION/MANUAL_UNASSIGNED/cash_on_delivery/PENDING/%d/%d)", k, commercial, fulfilment, mode, collection, tot, surcharge, total, surchargeMinor)
		}
		if n := e.count(`SELECT count(*) FROM checkout.payment_attempts WHERE order_id=$1`, o.ID); n != 0 {
			t.Errorf("order %s has a payment attempt", k)
		}
	}
	if n := e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1 AND payment_mode='cash_on_delivery' AND commercial_state='AWAITING_COLLECTION' AND collection_state='PENDING'`, e.store()); n != 4 {
		t.Errorf("%d AWAITING_COLLECTION/PENDING COD orders after placement, want 4", n)
	}

	// ---- phase 3: the merchant ships order A and records the collected cash (manual record, no carrier API) ---------------------------
	brfPlaywright(t, ctx, stack, []string{"home-cod.spec.ts"}, adminEnv("ship-collect", map[string]string{"LC_HCOD_ORDER": orders["A"].ID}))
	if got := e.collectionState(orders["A"].ID); got != "COLLECTED" {
		t.Errorf("order A is %s, want COLLECTED", got)
	}
	if n := e.count(`SELECT count(*) FROM checkout.orders WHERE id=$1 AND fulfillment_state='MERCHANT_SHIPPED'`, orders["A"].ID); n != 1 {
		t.Error("order A was not MERCHANT_SHIPPED by the manual record")
	}
	for _, k := range []string{"B", "C", "D"} {
		if got := e.collectionState(orders[k].ID); got != "PENDING" {
			t.Errorf("order %s is %s, want PENDING", k, got)
		}
	}
	if e.audit("fulfillment.collection_recorded") < 1 {
		t.Error("collection audit missing")
	}

	// ---- phase 4: finance carries the COD columns (order total + surcharge, LIVE, the collection day) --------------------------------
	tpe := time.FixedZone("TPE", 8*3600)
	today := time.Now().In(tpe)
	from, to := today.AddDate(0, 0, -1).Format("2006-01-02"), today.AddDate(0, 0, 1).Format("2006-01-02")
	st, out, raw = e.mcall(e.token(), "GET", "/v1/admin/stores/"+e.store()+"/finance/summary?from="+from+"&to="+to, "", "")
	if st != 200 {
		t.Fatalf("finance: %d %s", st, raw)
	}
	var codCount, codMinor float64
	for _, r := range func() []any { r, _ := out["totals"].([]any); return r }() {
		m, _ := r.(map[string]any)
		if m["currency"] == "TWD" && m["environment"] == "LIVE" {
			codCount += m["cod_collected_count"].(float64)
			codMinor += m["cod_collected_minor"].(float64)
		}
	}
	if codCount != 1 || codMinor != float64(total+surchargeMinor) {
		t.Errorf("finance cod columns %v/%v, want 1/%d (%s)", codCount, codMinor, total+surchargeMinor, raw)
	}
	brfPlaywright(t, ctx, stack, []string{"home-cod.spec.ts"}, adminEnv("finance", map[string]string{"LC_HCOD_TODAY": today.Format("2006-01-02")}))

	// ---- phase 5: order A's buyer sees COLLECTED (still AWAITING_COLLECTION); B, C, D stay PENDING -------------------------------
	release("1")
	wait("done")
	if err := <-exited; err != nil {
		t.Fatalf("buyer browser gate failed: %v; evidence=%s", err, evidence)
	}
	bcoShots(t, evidence, 8)
	if n := e.count(`SELECT count(*) FROM checkout.orders WHERE id=$1 AND commercial_state='AWAITING_COLLECTION' AND collection_state='COLLECTED'`, orders["A"].ID); n != 1 {
		t.Error("order A must stay AWAITING_COLLECTION after collection (it is never CONFIRMED)")
	}
	t.Logf("home-cod BROWSER (MOCK) merchant settings -> buyer checkout -> merchant ship/collect -> buyer sees COLLECTED -> finance COD columns; evidence=%s", evidence)
}

// hcodStartAdmin is bcoStartAdmin for the home-cod gate: the private Go API (identity + admin routes incl. the CVS collection/release routes
// that cash_on_delivery shares) over the isolated database, and the production admin Next build against it. The merchant account service is
// started because the settings wizard reads provider-accounts (which answers 503 without it). The store creator is the principal the mock IdP
// subject maps to.
func hcodStartAdmin(t *testing.T, ctx context.Context, e *tcvEnv, evidence string) *brfStack {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	_, port, _ := net.SplitHostPort(address)
	origin := browserFront(t, address)
	f := e.p.f
	idp := newBrowserIDP(t, origin+"/api/auth/callback")
	mustExec(t, f.owner, `INSERT INTO identity.external_identities(issuer,subject,principal_id) VALUES($1,'browser-subject',$2)`, idp.server.URL, f.principalA)
	role := "hcod_" + strings.ReplaceAll(randomUUID(), "-", "")
	password := randomToken()
	mustExec(t, f.owner, `CREATE ROLE `+pgx.Identifier{role}.Sanitize()+` LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE IN ROLE commerce_identity PASSWORD '`+password+`'`)
	t.Cleanup(func() { mustExec(t, f.owner, `DROP ROLE `+pgx.Identifier{role}.Sanitize()) })
	u, err := url.Parse(f.databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.UserPassword(role, password)
	authority, err := platform.OpenIdentityPool(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(authority.Close)
	provider, err := oidclogin.New(ctx, oidclogin.Config{Issuer: idp.server.URL, ClientID: browserClientID, RedirectURL: idp.redirect, AllowLoopbackForTests: true})
	if err != nil {
		t.Fatal(err)
	}
	service, err := identity.New(authority, observedBrowserProvider{Provider: provider, t: t}, identity.Policy{ProviderKey: "browser-home-cod-v1", SessionTTL: time.Hour, OnboardingEnabled: true, Currencies: []string{"TWD", "USD"}})
	if err != nil {
		t.Fatal(err)
	}
	bffKey := randomToken()
	private, err := identityhttp.NewHandler(service, bffKey)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/v1/identity/", private)
	// Insert-only dependency, no worker/provider is started; the random fixture keys stay in Go memory.
	jobs, err := river.NewClient(riverpgxv5.New(f.runtime), &river.Config{Schema: "river"})
	if err != nil {
		t.Fatal(err)
	}
	bindings, err := core.New(jobs)
	if err != nil {
		t.Fatal(err)
	}
	accountKeys, err := accounts.NewKeyring("browser_fixture", map[string][]byte{"browser_fixture": randomBytes(32)}, randomBytes(32))
	if err != nil {
		t.Fatal(err)
	}
	accountService, err := accounts.New(accountKeys, bindings)
	if err != nil {
		t.Fatal(err)
	}
	mux.Handle("/", httpapi.NewHandler(f.runtime, httpapi.Options{SessionStoreList: true, CVS: e.cvs, Accounts: accountService}))
	api := httptest.NewServer(mux)
	t.Cleanup(api.Close)
	nextLog := browserLog(t, filepath.Join(evidence, "admin-next.log"))
	next := exec.CommandContext(ctx, "node", filepath.Join(root, "apps/admin/.next/standalone/apps/admin/server.js"))
	next.Dir = root
	next.Env = browserEnvironment(map[string]string{"HOSTNAME": "127.0.0.1", "PORT": port, "NODE_ENV": "production", "COMMERCE_IDENTITY_ENABLED": "1", "COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS": "1",
		"COMMERCE_PUBLIC_ORIGIN": origin, "COMMERCE_API_ORIGIN": api.URL, "COMMERCE_OIDC_ISSUER": idp.server.URL, "COMMERCE_BFF_KEY": bffKey, "COMMERCE_ONBOARDING_ENABLED": "1", "COMMERCE_ONBOARDING_CURRENCIES": "TWD,USD"})
	next.Stdout, next.Stderr = nextLog, nextLog
	next.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := next.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Kill(-next.Process.Pid, syscall.SIGKILL); _, _ = next.Process.Wait() })
	client := &http.Client{Timeout: time.Second}
	for attempt := 0; ; attempt++ {
		if response, err := client.Get("http://" + address + "/api/stores"); err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusUnauthorized {
				break
			}
		}
		if attempt > 100 {
			t.Fatalf("admin Next readiness failed; evidence=%s", evidence)
		}
		time.Sleep(100 * time.Millisecond)
	}
	return &brfStack{root: root, evidence: evidence, origin: origin, api: api}
}
