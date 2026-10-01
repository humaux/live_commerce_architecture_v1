//go:build browser

package foundation_test

// Browser gate of the manual-order buyer link (contracts/storefront-v2.md G3; mode --browser-manual-order): production admin and storefront Next builds,
// signed MOCK IdP, the real Go API (identity + admin routes incl. manual orders) and buyer handler over one isolated PG. The merchant UI creates the
// order and copies the link, a FRESH browser opens it, sees the order and bank details and submits the transfer proof; the same link in another fresh
// browser is refused. Independent PG readback afterwards. Evidence label: MOCK (IdP, local TLS/CONNECT edge); no provider or deployment acceptance.

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"livecommerce/internal/httpapi"
	"livecommerce/internal/identity"
	"livecommerce/internal/identityhttp"
	"livecommerce/internal/oidclogin"
)

func TestBrowserManualOrderLink(t *testing.T) {
	if os.Getenv("LC_BROWSER_MANUAL_ORDER_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-manual-order")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 240*time.Second)
	defer cancel()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := os.MkdirTemp(filepath.Join(root, "output/playwright"), "manual-order-link-")
	if err != nil {
		t.Fatal(err)
	}
	// The published origin of the store IS the buyer origin the script's TLS/CONNECT edge serves, so the link the merchant UI shows is openable.
	e, _, _ := mtOrderEnv(t, tcvOpts{origin: "https://buyer.example"})
	mustExec(t, e.p.f.owner, `UPDATE catalog.products SET name='Synthetic manual order product' WHERE id=$1`, e.p.stock.product.ID)

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
	service, err := identity.New(authority, observedBrowserProvider{Provider: provider, t: t}, identity.Policy{ProviderKey: "browser-manual-order-v1", SessionTTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	// The fixture principal (store creator: catalog, inventory incl. reserve, orders:read) is what the mock IdP subject maps to.
	mustExec(t, e.p.f.owner, `INSERT INTO identity.external_identities(issuer,subject,principal_id) VALUES($1,'browser-subject',$2)`, idp.server.URL, e.p.f.principalA)
	adminKey := randomToken()
	private, err := identityhttp.NewHandler(service, adminKey)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/v1/identity/", private)
	mux.Handle("/", httpapi.NewHandler(e.p.f.runtime, httpapi.Options{SessionStoreList: true, CVS: e.cvs, ManualOrders: mtManualOrders(t, e)}))
	api := httptest.NewServer(mux)
	t.Cleanup(api.Close)

	cmd := exec.CommandContext(ctx, "node", "tests/storefront/manual-order-link-gate.mjs")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	defer func() {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}()
	cmd.Dir = root
	cmd.Env = browserEnvironment(map[string]string{
		"COMMERCE_IDENTITY_ENABLED": "1", "COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS": "1",
		"COMMERCE_PUBLIC_ORIGIN": adminOrigin, "COMMERCE_API_ORIGIN": api.URL, "COMMERCE_OIDC_ISSUER": idp.server.URL, "COMMERCE_BFF_KEY": adminKey,
		"COMMERCE_BUYER_WEB_ENABLED": "1", "COMMERCE_BUYER_DEMO_LABEL": "1", "COMMERCE_BUYER_API_ORIGIN": e.bh.server.URL, "COMMERCE_BUYER_BFF_KEY": e.bh.key,
		"COMMERCE_BUYER_COOKIE_KEY": brToken(), "COMMERCE_BUYER_SESSION_TTL": "3600",
		"LC_LINK_EVIDENCE": evidence, "LC_LINK_ADMIN_PORT": adminPort, "LC_LINK_STORE": e.store(), "LC_LINK_PRODUCT_NAME": "Synthetic manual order",
		"LC_LINK_ACCOUNT": "123-456-7890", "LC_LINK_AMOUNT": "12.50",
	})
	log := browserLog(t, filepath.Join(evidence, "browser.log"))
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Run(); err != nil {
		t.Fatalf("manual-order link browser gate failed: %v; evidence=%s", err, evidence)
	}
	data, err := os.ReadFile(filepath.Join(evidence, "result.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		OrderID string `json:"order_id"`
		Cases   int    `json:"cases"`
	}
	if json.Unmarshal(data, &result) != nil || result.Cases != 7 || result.OrderID == "" {
		t.Fatalf("missing exact browser gate results: %s", data)
	}
	// Independent PostgreSQL readback of what the browsers did.
	var source, state string
	if err := e.p.f.owner.QueryRow(ctx, `SELECT source,commercial_state FROM checkout.orders WHERE id=$1 AND store_id=$2`, result.OrderID, e.store()).Scan(&source, &state); err != nil || source != "merchant_manual" || state != "AWAITING_TRANSFER" {
		t.Fatalf("order readback: source=%q state=%q err=%v", source, state, err)
	}
	var last5 string
	var tstate string
	if err := e.p.f.owner.QueryRow(ctx, `SELECT state,proof_last5 FROM checkout.bank_transfers WHERE order_id=$1`, result.OrderID).Scan(&tstate, &last5); err != nil || tstate != "SUBMITTED" || last5 != "12345" {
		t.Fatalf("the buyer's transfer proof was not recorded: state=%q last5=%q err=%v", tstate, last5, err)
	}
	if n := e.count(`SELECT count(*) FROM checkout.order_links WHERE order_id=$1 AND redeemed_at IS NOT NULL`, result.OrderID); n != 1 {
		t.Fatalf("the link must be marked used exactly once, got %d", n)
	}
	if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='order.manual_created'`, e.store()); n != 1 {
		t.Fatalf("manual-order audit rows: %d", n)
	}
	t.Logf("PASS: merchant UI manual order -> link -> fresh browser exchange -> bank details -> transfer proof; single use; cases=%d; evidence=%s", result.Cases, evidence)
}
