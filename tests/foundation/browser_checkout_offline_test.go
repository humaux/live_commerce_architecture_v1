//go:build browser

package foundation_test

// COB (contracts/storefront-v2.md §C, unit checkout-offline; R4 independent browser gate): `TestBrowserCheckoutOffline`, prefix `bco`. Run through
// `bash scripts/dev/test-local.sh --browser-checkout-offline` (isolated PG 18, production storefront + admin Next builds).
//
// One real chain, two real browsers' worth of pages, one clock the test controls:
//   1. merchant (admin Next, zh-TW desktop) enables bank transfer in the settings wizard, sets bank details, the 6 hour window and a flat fee with a
//      free-shipping threshold (service re-saved), reads the values back after a reload
//   2. five buyers on the production storefront (zh-TW/en x desktop/390px) check out HOME delivery at the threshold with bank transfer: free shipping,
//      the shop's bank details and a countdown, last-5 + amount + time submitted -> SUBMITTED; the fifth does not pay
//   3. merchant rejects A (reason), confirms B, C, D in the order page; buyers see the reason / the shop's confirmation; A resubmits, merchant confirms
//   4. order E's window is aged out and the expiry function runs (nothing waits hours): CANCELLED, stock released, bank details withdrawn
// Evidence labels: BROWSER, MOCK (no PSP exists on this path). PG facts are asserted around every phase. Owner-pool writes (disclosed fixtures):
// aging order E's expires_at (cofAge), the settings of nothing else, and SELECTs.

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

func bcoRequire(t *testing.T) {
	t.Helper()
	if os.Getenv("LC_BROWSER_CHECKOUT_OFFLINE_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-checkout-offline")
	}
}

func TestBrowserCheckoutOffline(t *testing.T) {
	bcoRequire(t)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()
	root, _ := filepath.Abs("../..")
	evidence := brfEvidence(t, root, "checkout-offline")
	const origin = "https://buyer.example"
	const unit, fee, orderQty = int64(1250), int64(6000), int64(2)

	e := tcvNew(t, tcvOpts{origin: origin})
	e.grantCreator("orders:read", "payments:refund", "fulfillment:write", "integration:manage", "integration:read")
	e.topUp()
	mustExec(t, e.p.f.owner, `UPDATE catalog.products SET name='Synthetic bank transfer product',description='Synthetic acceptance fixture' WHERE id=$1`, e.p.stock.product.ID)
	sku := e.p.stock.skus[0].ID

	bffKey := base64.RawURLEncoding.EncodeToString(randomBytes(32))
	handler, err := buyerhttp.New(ctx, e.p.a.issuer, e.p.a.runtime, e.svc, bffKey, time.Hour)
	if err != nil {
		t.Fatalf("buyer HTTP constructor: %v", err)
	}
	buyerAPI := newHTTPServer(t, handler)
	stack := bcoStartAdmin(t, ctx, e, evidence)
	adminEnv := func(phase string, extra map[string]string) map[string]string {
		env := map[string]string{"LC_BROWSER_STORE": e.store(), "LC_OFF_PHASE": phase, "LC_BROWSER_PHASE": phase, "LC_OFF_BANK": cogBank, "LC_OFF_ACCOUNT": cogAcct,
			"LC_OFF_THRESHOLD": "2500", "LC_OFF_FEE": "6000", "LC_OFF_MARKET": e.p.market.ID, "LC_OFF_SERVICE": e.p.delivery.Code, "LC_OFF_REASON": "amount does not match the transfer"}
		for k, v := range extra {
			env[k] = v
		}
		return env
	}

	// ---- phase 1: the merchant configures everything through the UI ----------------------------------------------------------------
	if n := e.count(`SELECT count(*) FROM checkout.bank_transfer_settings WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()); n != 0 {
		t.Fatalf("setup: %d settings rows before the merchant configured anything", n)
	}
	brfPlaywright(t, ctx, stack, []string{"checkout-offline.spec.ts"}, adminEnv("settings", nil))
	if n := e.count(`SELECT count(*) FROM checkout.bank_transfer_settings WHERE tenant_id=$1 AND store_id=$2 AND enabled AND bank_name=$3 AND account_number=$4 AND window_hours=6 AND NOT allow_cvs`, e.tenant(), e.store(), cogBank, cogAcct); n != 1 {
		t.Fatal("the settings wizard did not store the bank details and the 6 hour window")
	}
	var shipping, threshold int64
	var enabled bool
	if err := e.p.f.owner.QueryRow(ctx, `SELECT v.shipping_minor,coalesce(v.free_shipping_threshold_minor,-1),v.enabled FROM pricing.policy_versions v JOIN pricing.policy_heads h ON h.tenant_id=v.tenant_id AND h.store_id=v.store_id
	  AND h.method=v.method AND h.current_version=v.version WHERE v.tenant_id=$1 AND v.store_id=$2 AND v.method=$3`, e.tenant(), e.store(), "delivery:"+e.p.delivery.Code).Scan(&shipping, &threshold, &enabled); err != nil {
		t.Fatal(err)
	}
	if shipping != fee || threshold != 2500 || !enabled {
		t.Fatalf("the policy the wizard saved: shipping %d threshold %d enabled %v, want %d/2500/true", shipping, threshold, enabled, fee)
	}
	if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='checkout.bank_transfer_settings_changed'`, e.store()); n < 1 {
		t.Error("the settings change was not audited")
	}

	// ---- phases 2-4: one long-lived buyer script, handed over through files ----------------------------------------------------------
	values := map[string]string{"COMMERCE_BUYER_WEB_ENABLED": "1", "COMMERCE_BUYER_API_ORIGIN": buyerAPI.URL, "COMMERCE_BUYER_DEMO_LABEL": "1", "COMMERCE_BUYER_BFF_KEY": bffKey,
		"COMMERCE_BUYER_COOKIE_KEY": base64.RawURLEncoding.EncodeToString(randomBytes(32)), "COMMERCE_BUYER_SESSION_TTL": "3600",
		"LC_OFF_EVIDENCE": evidence, "LC_OFF_ORIGIN": origin, "LC_OFF_PRODUCT": e.p.stock.product.ID, "LC_OFF_ACCOUNT": cogAcct, "LC_OFF_BANK": cogBank,
		"LC_OFF_PRICE": "1250", "LC_OFF_FEE": "6000", "LC_OFF_REASON": "amount does not match the transfer"}
	cmd := exec.CommandContext(ctx, "node", "tests/storefront/offline-buyer.mjs")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	defer func() {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}()
	cmd.Dir = root
	cmd.Env = browserEnvironment(values)
	log := browserLog(t, filepath.Join(evidence, "offline-buyer.mjs.log"))
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
				// The script writes its marker and then closes its browsers and exits: the exit can win this select inside one poll interval.
				// The marker is the truth; hand the exit result back for the final `<-exited`.
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
	raw, err := os.ReadFile(filepath.Join(evidence, "orders.json"))
	if err != nil {
		t.Fatal(err)
	}
	var orders map[string]struct {
		ID     string `json:"id"`
		Locale string `json:"locale"`
		Mobile bool   `json:"mobile"`
	}
	if err := json.Unmarshal(raw, &orders); err != nil || len(orders) != 5 {
		t.Fatalf("orders.json: %v %s", err, raw)
	}
	ordersEnv, _ := json.Marshal(orders)
	// PG facts after the buyer placed five orders through the UI: free shipping at the boundary, snapshot rows, stock only RESERVED
	total := unit * orderQty
	for k, o := range orders {
		var commercial, mode, last5 string
		var amount int64
		var email *string
		if err := e.p.f.owner.QueryRow(ctx, `SELECT o.commercial_state,o.payment_mode,o.total_minor,coalesce(t.proof_last5,''),o.buyer_email FROM checkout.orders o JOIN checkout.bank_transfers t ON t.order_id=o.id WHERE o.id=$1`, o.ID).Scan(&commercial, &mode, &amount, &last5, &email); err != nil {
			t.Fatalf("order %s: %v", k, err)
		}
		wantLast5 := "12345"
		if k == "E" {
			wantLast5 = ""
		}
		if commercial != "AWAITING_TRANSFER" || mode != "bank_transfer" || amount != total || last5 != wantLast5 {
			t.Errorf("order %s after placement: %s/%s total %d last5 %q (want AWAITING_TRANSFER/bank_transfer/%d/%q): the shipping at the boundary must be 0", k, commercial, mode, amount, last5, total, wantLast5)
		}
		if (k == "A") != (email != nil && *email == "buyer.a@example.com") {
			t.Errorf("order %s buyer_email %v", k, email)
		}
		if n := e.count(`SELECT count(*) FROM inventory.ledger WHERE reservation_id=$1 AND kind IN ('ALLOCATE','RELEASE')`, o.ID); n != 0 {
			t.Errorf("order %s has %d ledger rows before any merchant decision or expiry", k, n)
		}
		if n := e.count(`SELECT count(*) FROM checkout.payment_attempts WHERE order_id=$1`, o.ID); n != 0 {
			t.Errorf("order %s has a payment attempt", k)
		}
	}
	_, reservedHeld, allocated0 := e.cofBalance(sku)
	brfPlaywright(t, ctx, stack, []string{"checkout-offline.spec.ts"}, adminEnv("review", map[string]string{"LC_OFF_ORDERS": string(ordersEnv)}))
	for k, want := range map[string]string{"A": "REJECTED", "B": "CONFIRMED", "C": "CONFIRMED", "D": "CONFIRMED", "E": "AWAITING"} {
		if n := e.count(`SELECT count(*) FROM checkout.bank_transfers WHERE order_id=$1 AND state=$2`, orders[k].ID, want); n != 1 {
			t.Errorf("after the merchant's first pass order %s is not %s", k, want)
		}
	}
	if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='checkout.bank_transfer_confirmed'`, e.store()); n != 3 {
		t.Errorf("%d confirm audit rows after confirming B, C, D", n)
	}
	if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='checkout.bank_transfer_rejected'`, e.store()); n != 1 {
		t.Errorf("%d reject audit rows", n)
	}
	if _, reserved, allocated := e.cofBalance(sku); reserved != reservedHeld-3*orderQty || allocated != allocated0+3*orderQty {
		t.Errorf("stock after three confirmations: reserved %d -> %d, allocated %d -> %d (want -6/+6)", reservedHeld, reserved, allocated0, allocated)
	}
	release("1")
	wait("after1")
	brfPlaywright(t, ctx, stack, []string{"checkout-offline.spec.ts"}, adminEnv("confirm-a", map[string]string{"LC_OFF_ORDERS": string(ordersEnv)}))
	if n := e.count(`SELECT count(*) FROM checkout.bank_transfers WHERE order_id=$1 AND state='CONFIRMED' AND proof_last5='54321' AND proof_count=2`, orders["A"].ID); n != 1 {
		t.Error("A was not confirmed on its resubmitted details")
	}
	release("2")
	wait("after2")

	// the clock is ours: age E's whole window out and run the expiry function exactly as the River worker does
	_, reservedBeforeExpiry, allocatedBeforeExpiry := e.cofBalance(sku)
	if d, _ := e.cofExpire(orders["E"].ID); d != "NOT_DUE" {
		t.Errorf("E's window is still open: expire_held answered %s", d)
	}
	e.cofAge(orders["E"].ID, 7)
	if d, _ := e.cofExpire(orders["E"].ID); d != "EXPIRED" {
		t.Fatalf("expire_held on the aged order: %s", d)
	}
	if _, reserved, allocated := e.cofBalance(sku); reserved != reservedBeforeExpiry-orderQty || allocated != allocatedBeforeExpiry {
		t.Errorf("stock after the expiry: reserved %d -> %d (want -%d), allocated %d -> %d (want unchanged)", reservedBeforeExpiry, reserved, orderQty, allocatedBeforeExpiry, allocated)
	}
	if o, err := e.cogRow(orders["E"].ID).cogOutcome(); err != nil || o != "expired" {
		t.Errorf("E end state %q %v", o, err)
	}
	release("3")
	wait("done")
	brfPlaywright(t, ctx, stack, []string{"checkout-offline.spec.ts"}, adminEnv("final", map[string]string{"LC_OFF_ORDERS": string(ordersEnv)}))
	if err := <-exited; err != nil {
		t.Fatalf("buyer browser gate failed: %v; evidence=%s", err, evidence)
	}
	bcoShots(t, evidence, 12)
	if n := e.count(`SELECT count(*) FROM checkout.bank_transfers WHERE order_id=ANY($1) AND state='CONFIRMED' AND confirmed_amount_minor=$2`, []string{orders["A"].ID, orders["B"].ID, orders["C"].ID, orders["D"].ID}, total); n != 4 {
		t.Errorf("%d of the four orders are CONFIRMED at the server total %d", n, total)
	}
	t.Logf("COB BROWSER (MOCK) merchant settings -> buyer checkout/proof -> merchant confirm/reject -> buyer sees result -> expiry released stock; evidence=%s", evidence)
}

// bcoStartAdmin is brcStartAdmin plus the merchant account service (the settings wizard reads provider-accounts, which answers 503 without it): the private Go API (identity + admin routes incl. the CVS routes) over the isolated database,
// and the production admin Next build against it. The store creator is the principal the mock IdP subject maps to.
func bcoStartAdmin(t *testing.T, ctx context.Context, e *tcvEnv, evidence string) *brfStack {
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
	origin := browserFront(t, address) // https TLS front under LC_BROWSER_ENGINE=webkit, else http://address
	f := e.p.f
	idp := newBrowserIDP(t, origin+"/api/auth/callback")
	mustExec(t, f.owner, `INSERT INTO identity.external_identities(issuer,subject,principal_id) VALUES($1,'browser-subject',$2)`, idp.server.URL, f.principalA)
	role := "bco_" + strings.ReplaceAll(randomUUID(), "-", "")
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
	service, err := identity.New(authority, observedBrowserProvider{Provider: provider, t: t}, identity.Policy{ProviderKey: "browser-checkout-offline-v1", SessionTTL: time.Hour, OnboardingEnabled: true, Currencies: []string{"TWD", "USD"}})
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

// bcoShots verifies the hashed screenshot manifest: at least min shots, every one hashed, exactly the locales zh-TW and en on desktop and 390px.
func bcoShots(t *testing.T, evidence string, min int) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(evidence, "screenshots.json"))
	if err != nil {
		t.Fatalf("screenshot hash manifest missing: %v; evidence=%s", err, evidence)
	}
	var shots []struct{ File, Sha256, Locale, Viewport string }
	if err := json.Unmarshal(raw, &shots); err != nil || len(shots) < min {
		t.Fatalf("screenshot manifest has %d entries (want >= %d): %v", len(shots), min, err)
	}
	seen := map[string]bool{}
	for _, s := range shots {
		if len(s.Sha256) != 64 {
			t.Fatalf("screenshot %s is not hashed", s.File)
		}
		seen[s.Locale+"/"+s.Viewport] = true
	}
	for _, want := range []string{"zh-TW/desktop", "zh-TW/mobile", "en/desktop", "en/mobile"} {
		if !seen[want] {
			t.Errorf("no screenshot for %s", want)
		}
	}
}
