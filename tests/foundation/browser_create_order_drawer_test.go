//go:build browser

// Purpose: independently verify LC-U3 over real Next/BFF/Go/PG without changing the manual-order gate's facts.
// Depends on: lbuEnv, real inbox A14, signed MOCK OIDC, real comment poller fixture and the Node drawer driver.
// Used by: TestBrowserManualOrderLink after its original ten cases and audit/link assertions.
// Invariants: I02/I03/I06/I09/I11; no bearer response, key, contact or address is emitted to evidence.
package foundation_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"livecommerce/internal/httpapi"
	"livecommerce/internal/identity"
	"livecommerce/internal/identityhttp"
	"livecommerce/internal/inbox"
	"livecommerce/internal/metaconnect"
	"livecommerce/internal/oidclogin"
)

type drawerFixture struct{ Linked, Unlinked, Bundle string }

func runCreateOrderDrawerBrowser(t *testing.T) {
	t.Helper()
	calibration := os.Getenv("LC_DRAWER_CALIBRATION")
	if calibration != "" && calibration != "new-key-on-retry" {
		t.Fatal("unsupported LC_DRAWER_CALIBRATION")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	e := lbuNew(t)
	e.service("cvs_711", "MANUAL", 0)
	e.cvsSettings(tcvAllChains, true, "20000", 20)
	var cvsKey string
	options, err := e.manual.Options(ctx, e.token(), e.store())
	if err != nil {
		t.Fatal(err)
	}
	for _, option := range options {
		if option.DeliveryKind == "cvs_711" {
			cvsKey = option.OptionKey
		}
	}
	if cvsKey == "" {
		t.Fatal("drawer requires a manual 7-ELEVEN option")
	}

	var skuCode string
	if err := e.p.f.owner.QueryRow(ctx, `SELECT code FROM catalog.skus WHERE id=$1`, e.money).Scan(&skuCode); err != nil {
		t.Fatal(err)
	}
	mustExec(t, e.p.f.owner, `UPDATE catalog.products SET name='LC-U3 synthetic catalog' WHERE id=(SELECT product_id FROM catalog.skus WHERE id=$1)`, e.money)
	// Synthetic capability evidence enables only the exact conversation binding, through the real B1 read model.
	pageID, bindingID := mciDigits(15), randomUUID()
	mustExec(t, e.p.f.owner, `INSERT INTO integration.bindings(tenant_id,store_id,id,principal_id,provider,external_asset_id) VALUES($1,$2,$3,$4,'facebook',$5)`, e.tenant(), e.store(), bindingID, e.p.f.principalA, pageID)
	mustExec(t, e.p.f.owner, `INSERT INTO integration.meta_connections(tenant_id,store_id,page_id,page_name,fb_binding,scopes,status,connected_by,route_expires_at) VALUES($1,$2,$3,'LC-U3 MOCK Page',$4,ARRAY['pages_messaging'],'active',$5,clock_timestamp()+interval '1 day')`, e.tenant(), e.store(), pageID, bindingID, e.p.f.principalA)
	mustExec(t, e.p.f.owner, `INSERT INTO integration.binding_capabilities(tenant_id,store_id,page_id,binding_id,provider,capability,state,reason,evidence,checked_at) VALUES($1,$2,$3,$4,'facebook','dm_session','ok','fixture_ok','MOCK',clock_timestamp())`, e.tenant(), e.store(), pageID, bindingID)
	e.plan.closed = true // Synthetic closed-DM boundary only; all order/quote/hold writes use real Go/PG.
	kr, err := inbox.LoadKeyring(func(n string) string {
		switch n {
		case "COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID":
			return miKeyID
		case "COMMERCE_META_PAYLOAD_KEYS_JSON":
			return `{"keys":[{"id":"` + miKeyID + `","key_base64":"` + base64.StdEncoding.EncodeToString(randomBytes(32)) + `"}]}`
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	svc, err := inbox.NewService(kr)
	if err != nil {
		t.Fatal(err)
	}
	e.handler = httpapi.NewHandler(e.p.f.runtime, httpapi.Options{SessionStoreList: true, Studio: true, ClaimLabels: &e.h.labels, CVS: e.cvs, ManualOrders: e.manual, ForBuyer: e.fb, Inbox: svc, MetaHealth: &metaconnect.Health{Reader: metaconnect.TableReader{Fallback: metaconnect.SnapshotReader{}}}})
	// Seed the customer's actual contact and most recent delivery via the ordinary real Place pipeline.
	st, seed := e.post(e.token(), t04Key("drawer-history"), e.body(1, nil, "", false))
	if st != 201 {
		t.Fatalf("drawer history fixture status=%d", st)
	}
	var customer string
	if err := e.p.f.owner.QueryRow(ctx, `SELECT owner_id::text FROM checkout.orders WHERE id=$1`, lbuStr(seed, "order_id")).Scan(&customer); err != nil {
		t.Fatal(err)
	}
	fixtures := []drawerFixture{}
	for i := 0; i < 6; i++ {
		_, _, bundle, conv := e.scenario()
		mustExec(t, e.p.f.owner, `UPDATE social.conversations SET asset_id=$2 WHERE id=$1`, conv, pageID)
		mustExec(t, e.p.f.owner, `UPDATE inbox.bundle_peers SET asset_id=$2 WHERE bundle_id=$1`, bundle, pageID)
		unlinked := lcConversation(t, e.p.f, e.tenant(), e.store(), "page")
		mustExec(t, e.p.f.owner, `UPDATE inbox.conversation_state SET last_inbound_at=clock_timestamp()-interval '25 hours' WHERE conversation_id IN ($1,$2)`, conv, unlinked)
		// Calls real A14 CAS; no direct customer_id update and no owner guessing.
		b, _ := json.Marshal(map[string]any{"customer_id": customer, "expected_version": 0})
		reply := e.admin(e.token()).call("POST", "/inbox/conversations/"+conv+"/customer-link", t04Key("drawer-link"), "application/json", b)
		if reply.Code != 200 {
			t.Fatalf("drawer A14 fixture status=%d", reply.Code)
		}
		fixtures = append(fixtures, drawerFixture{conv, unlinked, bundle})
	}
	// The existing signed-ingress/real-poller fixture proves the direct CommentStream trigger over real A2.
	comments := newConsoleCommentsFixture(t)
	cf := comments.e.h.f
	mustExec(t, cf.owner, `INSERT INTO identity.memberships(tenant_id,principal_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, cf.tenantA, e.p.f.principalA)
	for _, perm := range []string{"store:read", "live:read", "live:manage", "inbox:read", "orders:read", "inventory:reserve", "catalog:read"} {
		mustExec(t, cf.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, cf.tenantA, cf.storeA1, e.p.f.principalA, perm)
	}
	commentHandler := httpapi.NewHandler(cf.runtime, httpapi.Options{SessionStoreList: true, Studio: true, ClaimLabels: &comments.e.h.labels, CommentStream: comments.stream, Inbox: comments.e.svc, ForBuyer: e.fb, ManualOrders: e.manual,
		MetaHealth: &metaconnect.Health{Reader: metaconnect.TableReader{Fallback: metaconnect.SnapshotReader{}}}})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	_, port, _ := net.SplitHostPort(addr)
	_ = listener.Close()
	origin := browserFront(t, addr)
	idp := newBrowserIDP(t, origin+"/api/auth/callback")
	mustExec(t, e.p.f.owner, `INSERT INTO identity.external_identities(issuer,subject,principal_id) VALUES($1,'browser-subject',$2)`, idp.server.URL, e.p.f.principalA)
	_, _, authority := identityFixture(t)
	provider, err := oidclogin.New(ctx, oidclogin.Config{Issuer: idp.server.URL, ClientID: browserClientID, RedirectURL: idp.redirect, AllowLoopbackForTests: true})
	if err != nil {
		t.Fatal(err)
	}
	service, err := identity.New(authority, observedBrowserProvider{Provider: provider, t: t}, identity.Policy{ProviderKey: "browser-create-order-drawer-v1", SessionTTL: time.Hour})
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
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && (r.URL.Path == "/__test/drawer-request-facts" || r.URL.Path == "/__test/manual-request-facts") {
			operation := "merchanttools.order.for_buyer"
			if r.URL.Path == "/__test/manual-request-facts" {
				operation = "merchanttools.order.manual"
			}
			var input struct {
				Keys []string `json:"keys"`
			}
			dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
			dec.DisallowUnknownFields()
			if dec.Decode(&input) != nil || len(input.Keys) < 1 || len(input.Keys) > 2 {
				http.Error(w, "invalid observation", 400)
				return
			}
			for _, key := range input.Keys {
				if !regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`).MatchString(key) {
					http.Error(w, "invalid observation", 400)
					return
				}
			}
			var orders, receipts int
			if err := e.p.f.owner.QueryRow(r.Context(), `SELECT count(DISTINCT o.id),count(*) FROM ops.command_results c
     JOIN checkout.orders o ON o.id=(c.response->>'order_id')::uuid AND o.tenant_id=c.tenant_id AND o.store_id=c.store_id
     WHERE c.tenant_id=$1 AND c.store_id=$2 AND c.principal_id=$3 AND c.operation=$5 AND c.idempotency_key=ANY($4::text[])`, e.tenant(), e.store(), e.p.f.principalA, input.Keys, operation).Scan(&orders, &receipts); err != nil {
				http.Error(w, "observation failed", 500)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			_ = json.NewEncoder(w).Encode(map[string]int{"orders": orders, "receipts": receipts})
			return
		}
		if r.Method == "GET" && r.URL.Path == "/__test/drawer-facts" {
			var orders int
			if err := e.p.f.owner.QueryRow(r.Context(), `SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2 AND source='merchant_manual'`, e.tenant(), e.store()).Scan(&orders); err != nil {
				http.Error(w, "fixture observation failed", 500)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			_ = json.NewEncoder(w).Encode(map[string]int{"orders": orders})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/admin/stores/"+cf.storeA1+"/") {
			commentHandler.ServeHTTP(w, r)
			return
		}
		e.handler.ServeHTTP(w, r)
	}))
	api := httptest.NewServer(mux)
	t.Cleanup(api.Close)
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	evidence := filepath.Join(root, "output/lc-u3-create-order-drawer/browser", time.Now().UTC().Format("20060102T150405.000000000"))
	if err := os.MkdirAll(evidence, 0700); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(fixtures)
	cmd := exec.CommandContext(ctx, "node", "tests/admin/create-order-drawer-gate.mjs")
	cmd.Dir = root
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	defer func() {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}()
	cmd.Env = browserEnvironment(map[string]string{
		"COMMERCE_IDENTITY_ENABLED": "1", "COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS": "1", "COMMERCE_PUBLIC_ORIGIN": origin, "COMMERCE_API_ORIGIN": api.URL, "COMMERCE_OIDC_ISSUER": idp.server.URL, "COMMERCE_BFF_KEY": bffKey,
		"LC_DRAWER_EVIDENCE": evidence, "LC_DRAWER_PORT": port, "LC_DRAWER_STORE": e.store(), "LC_DRAWER_FIXTURES": string(raw), "LC_DRAWER_SKU": e.money, "LC_DRAWER_SKU_CODE": skuCode, "LC_DRAWER_HOME": e.homeKey, "LC_DRAWER_CVS": cvsKey,
		"LC_DRAWER_COMMENT_STORE": cf.storeA1, "LC_DRAWER_COMMENT_SCENE": comments.e.session, "LC_DRAWER_COMMENT_REF": fmt.Sprint(comments.ids["claim_ref"]), "LC_DRAWER_CALIBRATION": calibration,
	})
	log := browserLog(t, filepath.Join(evidence, "browser.log"))
	cmd.Stdout, cmd.Stderr = log, log
	runErr := cmd.Run()
	// Perform independent final PG readback even when the calibration intentionally failed in Node.
	var orders int
	if err := e.p.f.owner.QueryRow(ctx, `SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2 AND source='merchant_manual'`, e.tenant(), e.store()).Scan(&orders); err != nil {
		t.Fatal(err)
	}
	// One history + twelve drawer orders + one manual UNKNOWN/edit/replay order, each additionally bound to its request receipts.
	pg, _ := json.Marshal(map[string]any{"orders": orders, "expected": 14, "single_order": orders == 14})
	if err := os.WriteFile(filepath.Join(evidence, "pg-readback.json"), pg, 0600); err != nil {
		t.Fatal(err)
	}
	if runErr != nil {
		t.Fatalf("drawer browser gate failed: %v; PG orders=%d expected=14; evidence=%s", runErr, orders, evidence)
	}
	if orders != 14 {
		t.Fatalf("drawer single-order PG readback orders=%d expected=14; evidence=%s", orders, evidence)
	}
	for _, fx := range fixtures {
		if n := e.count(`SELECT count(*) FROM inbox.order_for_buyer WHERE store_id=$1 AND bundle_id=$2 AND state='placed'`, e.store(), fx.Bundle); n != 1 {
			t.Fatalf("drawer bundle reservation count=%d", n)
		}
	}
	data, err := os.ReadFile(filepath.Join(evidence, "result.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Cases  int      `json:"cases"`
		Failed []string `json:"failed"`
	}
	if json.Unmarshal(data, &result) != nil || result.Cases != 25 || len(result.Failed) != 0 {
		t.Fatal("drawer exact 25-case matrix result missing")
	}
	t.Logf("PASS LC-U3 real browser/Go/PG matrix cases=%d orders=%d; evidence=%s", result.Cases, orders, evidence)
}
