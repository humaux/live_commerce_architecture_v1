//go:build browser

package foundation_test

// Live tools (R4) browser gate, driven by tests/e2e/live-tools.spec.ts. Independent test author (not the implementer), from the contract
// (contracts/live-keyword-claims-v1.md amendment "Live tools (R4)") and the unit brief. Run through `bash scripts/dev/test-local.sh --browser-e2e`
// (it runs this test after the T12 deal loop).
//
// Journey per matrix cell (Chromium; desktop 1440 and 390 px; zh-TW and en; four cells, one scene/post/SKU each, in ONE stack):
//   merchant (production admin Next, signed MOCK IdP) adds a library keyword in Studio -> creates a NEW scene -> "import all library keywords"
//   -> sets the live price in the offers table -> opens the window, binds the Facebook post -> a SIGNED Meta comment "<KW>+2" over HTTP into the
//   real ingress -> consumer -> intake poller -> claim bundle -> one private reply with the claim link (fake Graph) -> the buyer opens the link on
//   the new storefront shell (https://buyer.example through the synthetic TLS edge), sees the LIVE price, adds it to the cart, checks out with pay at
//   pickup (buyer-entered 7-ELEVEN store) and the order page shows the live price; a second buyer who never saw the link buys the SAME SKU from the
//   product page and pays the normal price. Go is the oracle for every PostgreSQL fact (orders, snapshot price_rule, payment mode, no card attempt).
//
// Evidence labels: BROWSER, Meta = MOCK (signed synthetic webhook, fake Graph), IdP = signed MOCK, no PSP (pay at pickup), no provider/deployment
// acceptance. Disclosed fixtures: catalog/stock/pricing/delivery/CVS settings through the store creator's own domain calls (no Studio UI for them
// in R4), the Meta binding + route rows (owner registrar path, as the MCI and T12 gates), the storefront publication/domain rows.
// LC_LTG_WORKAROUND_D1=1 applies the disclosed checkout-grant fixture of defect D1 (see DEFECTS.md); without it the order step is red.

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/buyerhttp"
	"livecommerce/internal/catalog"
	"livecommerce/internal/claims"
	"livecommerce/internal/claimsintake"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/identity"
	"livecommerce/internal/identityhttp"
	"livecommerce/internal/integrations/meta"
	"livecommerce/internal/integrations/metareply"
	"livecommerce/internal/inventory"
	"livecommerce/internal/oidclogin"
	"livecommerce/internal/platform"
)

type ltRun struct {
	t        *testing.T
	ctx      context.Context
	e        *ltgEnv
	root     string
	evidence string
	actions  chan e2eAction

	idp          *browserIDP
	adminOrigin  string
	adminAPI     *httptest.Server
	buyerAPI     *httptest.Server
	ingress      *httptest.Server
	adminKey     string
	buyerKey     string
	graph        *mciGraph
	mci          *mciEnv
	pageAsset    string
	post         string
	firstComment string
	cvsCode      string
	secrets      map[string]string
	tokens       []string
	sends        int
}

var ltLink = regexp.MustCompile(`https://buyer\.example/(zh-TW|en)/claim#t=([A-Za-z0-9_-]{43})`)

func TestBrowserLiveTools(t *testing.T) {
	e2eRequire(t)
	ctx, cancel := context.WithTimeout(context.Background(), 28*time.Minute)
	defer cancel()
	x := &ltRun{t: t, ctx: ctx, actions: make(chan e2eAction), secrets: map[string]string{}}
	x.root, _ = filepath.Abs("../..")
	x.evidence = brfEvidence(t, x.root, "live-tools")
	x.e = ltgNew(t, tcvOpts{origin: sbOrigin})
	f := x.e.p.f
	x.e.grantCreator("store:read", "catalog:read", "catalog:write", "inventory:read", "inventory:write", "orders:read")
	// pay at pickup with a buyer-entered store (the store has no ECPay profile): the merchant-side settings the checkout UI needs.
	x.e.cvsSettings(tcvAllChains, true, "20000", 500)
	x.cvsCode, _, _ = x.e.service("cvs_711", "MANUAL", 0)

	x.startAdminAPI(t)
	x.startBuyerAPI(t)
	x.provisionMeta(t)
	control, controlKey := x.startControl(t)
	x.startAdminNext(t)
	storefrontPort := x.startStorefrontNext(t)
	target, err := url.Parse("http://127.0.0.1:" + storefrontPort)
	if err != nil {
		t.Fatal(err)
	}
	edge := httptest.NewTLSServer(httputil.NewSingleHostReverseProxy(target))
	t.Cleanup(edge.Close)
	proxy := connectProxy(t, "buyer.example:443", edge.Listener.Addr().String())

	config := filepath.Join(x.evidence, "playwright.livetools.config.ts")
	body := fmt.Sprintf(`import { defineConfig } from "@playwright/test";
export default defineConfig({
  testDir: %q, testMatch: ["live-tools.spec.ts"],
  fullyParallel: false, workers: 1, retries: 0, timeout: 900000,
  expect: { timeout: 15000 },
  reporter: [["list"]], outputDir: %q,
  use: { headless: true, trace: "off", screenshot: "only-on-failure", actionTimeout: 20000, navigationTimeout: 30000 },
});
`, filepath.Join(x.root, "tests/e2e"), filepath.Join(x.evidence, "results"))
	if err := os.WriteFile(config, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	spec := exec.CommandContext(ctx, "pnpm", "exec", "playwright", "test", "tests/e2e/live-tools.spec.ts", "--config", config, "--reporter=list")
	spec.Dir = x.root
	spec.Env = browserEnvironment(map[string]string{"LC_LT_ADMIN_ORIGIN": x.adminOrigin, "LC_LT_BUYER_ORIGIN": sbOrigin, "LC_LT_PROXY": proxy,
		"LC_LT_EVIDENCE": x.evidence, "LC_LT_CONTROL": control, "LC_LT_CONTROL_KEY": controlKey, "LC_LT_STORE": f.storeA1, "LC_LT_CVS_CODE": x.cvsCode})
	log := browserLog(t, filepath.Join(x.evidence, "playwright.log"))
	spec.Stdout, spec.Stderr = log, log
	if err := spec.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- spec.Wait() }()
	for running := true; running; {
		select {
		case a := <-x.actions:
			status, out := x.dispatch(t, a)
			a.reply <- e2eReply{status, out}
		case err := <-done:
			running = false
			if err != nil {
				x.finish(t)
				t.Fatalf("live-tools browser chain failed: %v; evidence=%s", err, x.evidence)
			}
		case <-ctx.Done():
			t.Fatalf("live-tools deadline; evidence=%s", x.evidence)
		}
	}
	x.finish(t)
	t.Logf("LIVE-TOOLS BROWSER (MOCK Meta, pay at pickup): chromium desktop + 390 px x zh-TW + en passed; evidence=%s", x.evidence)
}

// ---------------------------------------------------------------------------------------------------------------------------------------
// processes
// ---------------------------------------------------------------------------------------------------------------------------------------

// startAdminAPI is brcStartAdmin with Studio + claims mounted: the private Go API (identity + admin routes) over the isolated database; the signed
// MOCK IdP subject maps to the store creator.
func (x *ltRun) startAdminAPI(t *testing.T) {
	ctx := x.ctx
	f := x.e.p.f
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	x.adminOrigin = "http://" + listener.Addr().String()
	_ = listener.Close()
	x.idp = newBrowserIDP(t, x.adminOrigin+"/api/auth/callback")
	mustExec(t, f.owner, `INSERT INTO identity.external_identities(issuer,subject,principal_id) VALUES($1,'browser-subject',$2)`, x.idp.server.URL, f.principalA)
	role := "lt_" + strings.ReplaceAll(randomUUID(), "-", "")
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
	provider, err := oidclogin.New(ctx, oidclogin.Config{Issuer: x.idp.server.URL, ClientID: browserClientID, RedirectURL: x.idp.redirect, AllowLoopbackForTests: true})
	if err != nil {
		t.Fatal(err)
	}
	service, err := identity.New(authority, observedBrowserProvider{Provider: provider, t: t}, identity.Policy{ProviderKey: "browser-live-tools-v1", SessionTTL: time.Hour, OnboardingEnabled: true, Currencies: []string{"TWD", "USD"}})
	if err != nil {
		t.Fatal(err)
	}
	x.adminKey = randomToken()
	x.secrets["admin BFF key"] = x.adminKey
	private, err := identityhttp.NewHandler(service, x.adminKey)
	if err != nil {
		t.Fatal(err)
	}
	labels, err := claims.NewLabelKey(randomBytes(32))
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/v1/identity/", private)
	// R1 deploy shape (ruling G2): planning-only Studio + claims, no media planner.
	mux.Handle("/", httpapi.NewHandler(f.runtime, httpapi.Options{SessionStoreList: true, Studio: true, ClaimLabels: &labels, CVS: x.e.cvs}))
	x.adminAPI = httptest.NewServer(mux)
	t.Cleanup(x.adminAPI.Close)
}

func (x *ltRun) startBuyerAPI(t *testing.T) {
	e := x.e
	x.buyerKey = base64.RawURLEncoding.EncodeToString(randomBytes(32))
	x.secrets["buyer BFF key"] = x.buyerKey
	handler, err := buyerhttp.New(x.ctx, e.p.a.issuer, e.p.a.runtime, e.svc, x.buyerKey, time.Hour)
	if err != nil {
		t.Fatalf("buyer HTTP constructor: %v", err)
	}
	x.buyerAPI = httptest.NewServer(handler)
	t.Cleanup(x.buyerAPI.Close)
}

func (x *ltRun) startAdminNext(t *testing.T) {
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(x.adminOrigin, "http://"))
	logFile := browserLog(t, filepath.Join(x.evidence, "admin-next.log"))
	cmd := exec.CommandContext(x.ctx, "node", filepath.Join(x.root, "apps/admin/.next/standalone/apps/admin/server.js"))
	cmd.Dir = x.root
	cmd.Env = browserEnvironment(map[string]string{"HOSTNAME": "127.0.0.1", "PORT": port, "NODE_ENV": "production", "NEXT_TELEMETRY_DISABLED": "1",
		"COMMERCE_IDENTITY_ENABLED": "1", "COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS": "1", "COMMERCE_PUBLIC_ORIGIN": x.adminOrigin,
		"COMMERCE_API_ORIGIN": x.adminAPI.URL, "COMMERCE_OIDC_ISSUER": x.idp.server.URL, "COMMERCE_BFF_KEY": x.adminKey,
		"COMMERCE_ONBOARDING_ENABLED": "1", "COMMERCE_ONBOARDING_CURRENCIES": "TWD,USD"})
	cmd.Stdout, cmd.Stderr = logFile, logFile
	x.spawn(t, cmd)
	waitReady(x.ctx, t, x.adminOrigin+"/api/stores", "", http.StatusUnauthorized, x.evidence)
}

func (x *ltRun) startStorefrontNext(t *testing.T) string {
	port := freeLoopbackPort(t)
	logFile := browserLog(t, filepath.Join(x.evidence, "storefront-next.log"))
	cookieKey := brToken()
	x.secrets["buyer cookie key"] = cookieKey
	cmd := exec.CommandContext(x.ctx, "node", filepath.Join(x.root, "apps/storefront/node_modules/next/dist/bin/next"), "start", "--hostname", "127.0.0.1", "--port", port)
	cmd.Dir = filepath.Join(x.root, "apps/storefront")
	cmd.Env = browserEnvironment(map[string]string{"NODE_ENV": "production", "NEXT_TELEMETRY_DISABLED": "1", "COMMERCE_BUYER_WEB_ENABLED": "1",
		"COMMERCE_BUYER_DEMO_LABEL": "1", "COMMERCE_BUYER_API_ORIGIN": x.buyerAPI.URL, "COMMERCE_BUYER_BFF_KEY": x.buyerKey,
		"COMMERCE_BUYER_COOKIE_KEY": cookieKey, "COMMERCE_BUYER_SESSION_TTL": "3600"})
	cmd.Stdout, cmd.Stderr = logFile, logFile
	x.spawn(t, cmd)
	waitReady(x.ctx, t, "http://127.0.0.1:"+port+"/api/buyer/session", "buyer.example", http.StatusOK, x.evidence)
	return port
}

func (x *ltRun) spawn(t *testing.T, cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); _ = cmd.Wait() })
}

// provisionMeta is e2eRun.provisionMeta for this store (Facebook Page binding + route, Page token for private replies, the claims-aware
// consumer, the intake poller, the metareply dispatcher over a fake Graph, the real ingress handler).
func (x *ltRun) provisionMeta(t *testing.T) {
	ctx := x.ctx
	f := x.e.p.f
	m := e2eMetaSetup(t, f)
	e := &mciEnv{t: t, h: x.e.h, page: m, consumerWorkers: 1, stopConsumer: mciNoop}
	var err error
	e.actorRaw, e.linkRaw, e.pageKeyRaw = randomBytes(32), randomBytes(32), randomBytes(32)
	if e.actor, err = meta.NewClaimsActorKey(e.actorRaw); err != nil {
		t.Fatal(err)
	}
	if e.link, err = claims.NewReplyLinkKey(e.linkRaw); err != nil {
		t.Fatal(err)
	}
	if e.pageKeys, err = metareply.NewPageTokenKeyring("pt_key_1", map[string][]byte{"pt_key_1": e.pageKeyRaw}); err != nil {
		t.Fatal(err)
	}
	e.pageAsset = miAsset()
	e.pageBinding = miBinding(t, m, e.pageAsset, "facebook", f.tenantA, f.storeA1, f.principalA)
	miRoute(t, m, e.pageAsset, f.tenantA, f.storeA1, e.pageBinding)
	e.pageToken = "SENTINEL-EAAG-PAGE-" + t04Tag() + t04Tag()
	x.secrets["page token"] = e.pageToken
	e.registerToken(t, "facebook", e.pageBinding, e.pageAsset, []string{"pages_messaging"}, e.pageToken)
	x.pageAsset = e.pageAsset
	x.mci = e
	e.intakeLogin = miRole(t, f, "commerce_claims_intake")
	if e.intakePool, err = platform.OpenClaimsIntakePool(ctx, e.intakeLogin); err != nil {
		t.Fatalf("claims intake login: %v", err)
	}
	t.Cleanup(e.intakePool.Close)
	if e.poller, err = claimsintake.New(ctx, e.intakePool, e.link, claimsintake.Config{Workers: 1, IdleSleep: 200 * time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	pollCtx, stopPoll := context.WithCancel(ctx)
	pollDone := make(chan struct{})
	go func() { _ = e.poller.Run(pollCtx); close(pollDone) }()
	t.Cleanup(func() { stopPoll(); <-pollDone })
	e.startConsumer(t)
	t.Cleanup(func() { e.stopConsumer() })
	x.graph = newMciGraph(t)
	pool := x.e.p.worker // the commerce_worker login of the checkout harness: the one worker authority of this process
	routes, err := metareply.Routes(pool, e.link, e.pageKeys, metareply.Config{GraphBaseURL: x.graph.srv.URL, GraphVersion: "v99.0", HTTPClient: x.graph.srv.Client()})
	if err != nil {
		t.Fatalf("metareply.Routes: %v", err)
	}
	t06StartDispatcher(t, pool, "default", routes, mciDispatchOptions())
	x.ingress = httptest.NewServer(m.handler)
	t.Cleanup(x.ingress.Close)
	m.registrar.Close()
	m.curator.Close()
}

func (x *ltRun) startControl(t *testing.T) (origin, key string) {
	key = randomToken()
	x.secrets["control key"] = key
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Gate-Key") != key || r.Method != http.MethodPost || r.URL.Path != "/act" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<16))
		reply := make(chan e2eReply, 1)
		select {
		case x.actions <- e2eAction{name: r.URL.Query().Get("name"), body: raw, reply: reply}:
		case <-x.ctx.Done():
			http.Error(w, "closed", http.StatusServiceUnavailable)
			return
		}
		var out e2eReply
		select {
		case out = <-reply:
		case <-x.ctx.Done():
			http.Error(w, "closed", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(out.status)
		_ = json.NewEncoder(w).Encode(out.body)
	}))
	t.Cleanup(server.Close)
	return server.URL, key
}

// ---------------------------------------------------------------------------------------------------------------------------------------
// actions (run on the test goroutine, so a failed assertion is an ordinary t.Fatalf)
// ---------------------------------------------------------------------------------------------------------------------------------------

func (x *ltRun) count(q string, args ...any) int { return countRows(x.t, x.e.p.f.owner, q, args...) }

func (x *ltRun) dispatch(t *testing.T, a e2eAction) (int, any) {
	t.Helper()
	var in map[string]any
	if len(a.body) > 0 {
		if err := json.Unmarshal(a.body, &in); err != nil {
			return http.StatusBadRequest, map[string]string{"error": "bad json"}
		}
	}
	str := func(k string) string { s, _ := in[k].(string); return s }
	var out any
	switch a.name {
	case "provision-run":
		out = x.provisionRun(t, int(in["n"].(float64)))
	case "new-post":
		x.post = mciDigits(15)
		out = map[string]any{"post_url": "https://www.facebook.com/" + x.pageAsset + "/posts/" + x.post, "object": x.pageAsset + "_" + x.post}
	case "comment":
		out = x.comment(t, str("text"))
	case "await-reply":
		out = x.awaitReply(t)
	case "end-run":
		out = x.endRun(t, str("scene"))
	case "check":
		out = x.check(t, str("name"), in)
	default:
		return http.StatusNotFound, map[string]string{"error": "unknown action"}
	}
	t.Logf("LIVE-TOOLS step %s ok", a.name)
	return http.StatusOK, out
}

// provisionRun creates the run's own product with one SKU (catalog 30000 minor = TWD 300.00, 50 units) through the store creator's domain calls.
func (x *ltRun) provisionRun(t *testing.T, n int) map[string]any {
	ctx := x.ctx
	e := x.e
	f := e.p.f
	tag := strings.ToUpper(t04Tag()[:5])
	name := fmt.Sprintf("Live tools product %d %s", n, tag)
	product, err := t04Scoped(ctx, f, e.token(), f.storeA1, "catalog:write", func(tx pgx.Tx, s platform.Scope) (catalog.Product, error) {
		return catalog.CreateProduct(ctx, tx, s, t04Key("lt-product"), catalog.ProductInput{Name: name, Description: "Synthetic live tools fixture", Status: catalog.StatusActive})
	})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	sku, err := t04Scoped(ctx, f, e.token(), f.storeA1, "catalog:write", func(tx pgx.Tx, s platform.Scope) (catalog.SKU, error) {
		return catalog.CreateSKU(ctx, tx, s, t04Key("lt-sku"), catalog.SKUInput{ProductID: product.ID, Code: fmt.Sprintf("LT%d-%s", n, tag), PriceMinor: ltgCatalog,
			WeightGrams: 100, LengthMM: 10, WidthMM: 20, HeightMM: 30, OriginCountry: "TW", CustomsName: "test item", HSCandidate: "851840"})
	})
	if err != nil {
		t.Fatalf("create SKU: %v", err)
	}
	if _, err = t04Scoped(ctx, f, e.token(), f.storeA1, "inventory:write", func(tx pgx.Tx, s platform.Scope) (inventory.Balance, error) {
		return inventory.AdjustOnHand(ctx, tx, s, t04Key("lt-stock"), inventory.Adjustment{WarehouseID: e.p.stock.warehouse.ID, SKUID: sku.ID, Delta: 50, ExpectedVersion: 0, Reason: "live tools browser stock"})
	}); err != nil {
		t.Fatalf("stock: %v", err)
	}
	return map[string]any{"product_id": product.ID, "product_name": name, "sku_id": sku.ID, "sku_code": sku.Code, "keyword": fmt.Sprintf("LT%d", n),
		"catalog_minor": ltgCatalog, "live_minor": ltgLive}
}

// comment posts one signed comment on the current post and waits for the consumer's `processed` fact (e2eRun.comment for this stack).
func (x *ltRun) comment(t *testing.T, text string) map[string]any {
	t.Helper()
	comment := mciDigits(15) + "_" + mciDigits(10)
	from := "100" + mciDigits(12)
	at := time.Now().Add(3 * time.Second)
	raw := mciFBBody(x.pageAsset, x.pageAsset+"_"+x.post, comment, from, "Amy Synthetic", text, &at, nil)
	batch, err := x.mci.page.verifier.Verify(raw, miSignature(raw))
	if err != nil || len(batch.Events) != 1 || batch.Events[0].QuarantineReason != "" {
		t.Fatalf("the synthetic comment is not one clean event: %v", err)
	}
	req, err := http.NewRequestWithContext(x.ctx, http.MethodPost, x.ingress.URL+"/meta-webhook", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", miSignature(raw))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("webhook POST: %v", err)
	}
	body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
	_ = res.Body.Close()
	if res.StatusCode != 200 || string(body) != "EVENT_RECEIVED" {
		t.Fatalf("real ingress answered %d %q", res.StatusCode, body)
	}
	ev := mcEvent{key: batch.Events[0].Key, app: miApp, object: batch.Object, asset: x.pageAsset}
	if err := x.e.p.f.owner.QueryRow(x.ctx, `SELECT id::text,route_id::text,route_epoch,job_id FROM meta_inbox.events WHERE app_id=$1 AND object=$2 AND event_key=$3 AND is_primary`,
		miApp, ev.object, ev.key).Scan(&ev.id, &ev.route, &ev.epoch, &ev.job); err != nil {
		t.Fatalf("the ingress admitted no primary event: %v", err)
	}
	mcAwait(t, x.mci.page, ev)
	x.firstComment = comment
	return map[string]any{"comment_id": comment, "ingress_status": res.StatusCode}
}

// awaitReply waits for exactly one private reply for the current comment and returns its claim link.
func (x *ltRun) awaitReply(t *testing.T) map[string]any {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for x.graph.posts(x.firstComment) == 0 {
		if time.Now().After(deadline) {
			t.Fatalf("no private reply reached the fake Graph within 60s (operations: %d)", x.count(`SELECT count(*) FROM integration.operations WHERE action='meta.private_reply'`))
		}
		time.Sleep(200 * time.Millisecond)
	}
	time.Sleep(1500 * time.Millisecond) // a second, wrong send would land inside this window
	x.sends++
	var mine []mciGraphReq
	for _, r := range x.graph.all() {
		if r.comment == x.firstComment {
			mine = append(mine, r)
		}
	}
	if len(mine) != 1 || mine[0].method != http.MethodPost || len(x.graph.all()) != x.sends {
		t.Fatalf("the fake Graph saw %d request(s) for this comment and %d in total, want exactly one POST each", len(mine), len(x.graph.all()))
	}
	match := ltLink.FindStringSubmatch(mine[0].text)
	if match == nil {
		t.Fatalf("the private reply carries no claim link on the published origin")
	}
	x.secrets["claim link token "+match[2][:6]] = match[2]
	x.tokens = append(x.tokens, match[2])
	return map[string]any{"link": match[0], "locale": match[1], "sends": len(mine)}
}

func (x *ltRun) endRun(t *testing.T, scene string) map[string]any {
	t.Helper()
	x.e.h.closeWindow(t, scene)
	return map[string]any{"closed": scene}
}

// ordersOf is the single order of an owner-agnostic id lookup with its snapshot.
func (x *ltRun) order(t *testing.T, id string) (total int64, state, mode string, collection *string, lines []map[string]any, attempts int) {
	t.Helper()
	var snapshot []byte
	if err := x.e.p.f.owner.QueryRow(x.ctx, `SELECT total_minor,commercial_state,payment_mode,collection_state,snapshot FROM checkout.orders WHERE id=$1`, id).
		Scan(&total, &state, &mode, &collection, &snapshot); err != nil {
		t.Fatalf("order %s: %v", id, err)
	}
	lines, _ = snapQuote(t, snapshot)
	attempts = x.count(`SELECT count(*) FROM checkout.payment_attempts WHERE order_id=$1`, id)
	return
}

func (x *ltRun) need(t *testing.T, ok bool, format string, args ...any) {
	t.Helper()
	if !ok {
		t.Fatalf("live-tools checkpoint failed: "+format+"; evidence="+x.evidence, args...)
	}
}

// check asserts PostgreSQL facts at one step of the chain.
func (x *ltRun) check(t *testing.T, name string, in map[string]any) map[string]any {
	t.Helper()
	owner := x.e.p.f.owner
	f := x.e.p.f
	str := func(k string) string { s, _ := in[k].(string); return s }
	facts := map[string]any{"checkpoint": name}
	switch name {
	case "library": // the Studio library panel wrote the row
		var kw string
		x.need(t, owner.QueryRow(x.ctx, `SELECT keyword FROM live.keyword_library WHERE tenant_id=$1 AND store_id=$2 AND sku_id=$3`, f.tenantA, f.storeA1, str("sku_id")).Scan(&kw) == nil && kw == str("keyword"),
			"library row for %s keyword %q (got %q)", str("sku_id"), str("keyword"), kw)
		x.need(t, x.count(`SELECT count(*) FROM live.offers WHERE tenant_id=$1 AND store_id=$2 AND sku_id=$3`, f.tenantA, f.storeA1, str("sku_id")) == 0, "a library row must not create an offer")
	case "offer": // the import created the offer in the NEW scene and the live price was set in the UI
		var kw string
		var price *int64
		var max int64
		x.need(t, owner.QueryRow(x.ctx, `SELECT keyword,live_price_minor,max_quantity_per_claim FROM live.offers WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3 AND sku_id=$4 AND active`,
			f.tenantA, f.storeA1, str("scene"), str("sku_id")).Scan(&kw, &price, &max) == nil && kw == str("keyword") && price != nil && *price == ltgLive && max >= 2,
			"offer in the new scene: keyword=%q price=%v max=%d", kw, price, max)
	case "claimed":
		var qty int
		var owned bool
		x.need(t, owner.QueryRow(x.ctx, `SELECT l.quantity,b.owner_id IS NOT NULL FROM claims.lines l JOIN claims.bundles b ON b.tenant_id=l.tenant_id AND b.store_id=l.store_id AND b.id=l.bundle_id
			WHERE l.tenant_id=$1 AND l.store_id=$2 AND l.sku_id=$3 AND b.session_id=$4`, f.tenantA, f.storeA1, str("sku_id"), str("scene")).Scan(&qty, &owned) == nil && qty == 2 && !owned,
			"claim line qty=%d bound=%v", qty, owned)
	case "cart": // the claim page's add-to-cart wrote the claim origin
		var qty, claimQty int
		x.need(t, owner.QueryRow(x.ctx, `SELECT c.quantity,c.claim_quantity FROM storefront.cart_lines c JOIN claims.bundles b ON b.tenant_id=c.tenant_id AND b.store_id=c.store_id AND b.owner_id=c.owner_id AND b.id=c.claim_bundle_id
			WHERE c.tenant_id=$1 AND c.store_id=$2 AND c.sku_id=$3 AND b.session_id=$4`, f.tenantA, f.storeA1, str("sku_id"), str("scene")).Scan(&qty, &claimQty) == nil && qty == 2 && claimQty == 2, "cart line qty=%d claim_quantity=%d", qty, claimQty)
	case "order-live":
		total, state, mode, collection, lines, attempts := x.order(t, str("order"))
		x.need(t, total == 2*ltgLive && state == "CONFIRMED" && mode == "pay_at_pickup" && collection != nil && *collection == "PENDING" && attempts == 0,
			"live order: total=%d state=%s mode=%s attempts=%d", total, state, mode, attempts)
		x.need(t, len(lines) == 1 && lines[0]["price_rule"] == "live_claim" && int64(lines[0]["unit_price_minor"].(float64)) == ltgLive && int64(lines[0]["catalog_unit_price_minor"].(float64)) == ltgCatalog,
			"snapshot line %v must record the live rule", lines)
		var bundle string
		x.need(t, owner.QueryRow(x.ctx, `SELECT b.id::text FROM claims.bundles b JOIN claims.lines l ON l.tenant_id=b.tenant_id AND l.store_id=b.store_id AND l.bundle_id=b.id WHERE b.session_id=$1 AND l.sku_id=$2`, str("scene"), str("sku_id")).Scan(&bundle) == nil &&
			lines[0]["claim_bundle_id"] == bundle, "snapshot claim_bundle_id %v is not the claimed bundle %s", lines[0]["claim_bundle_id"], bundle)
		facts["total_minor"] = total
	case "order-direct":
		total, state, mode, collection, lines, attempts := x.order(t, str("order"))
		x.need(t, total == ltgCatalog*int64(in["quantity"].(float64)) && state == "CONFIRMED" && mode == "pay_at_pickup" && collection != nil && attempts == 0, "direct order: total=%d state=%s mode=%s", total, state, mode)
		x.need(t, len(lines) == 1 && lines[0]["price_rule"] == nil && lines[0]["claim_bundle_id"] == nil && int64(lines[0]["unit_price_minor"].(float64)) == ltgCatalog, "the direct order must be priced at catalog with no live evidence: %v", lines)
		facts["total_minor"] = total
	default:
		t.Fatalf("unknown checkpoint %q", name)
	}
	return facts
}

func (x *ltRun) finish(t *testing.T) {
	t.Helper()
	needles := map[string]string{}
	for label, value := range x.secrets {
		needles[label] = value
	}
	scratch := t.TempDir()
	if err := os.WriteFile(filepath.Join(scratch, "clean.log"), []byte("nothing sensitive here"), 0o600); err != nil {
		t.Fatal(err)
	}
	if l, _ := e2eScan(scratch, needles); l != "" {
		t.Fatalf("scanner self-test: a clean file matched %q", l)
	}
	if len(x.tokens) > 0 {
		if err := os.WriteFile(filepath.Join(scratch, "leak.log"), []byte("x "+x.tokens[0]+" y"), 0o600); err != nil {
			t.Fatal(err)
		}
		if l, f := e2eScan(scratch, needles); l == "" || f != "leak.log" {
			t.Fatalf("scanner self-test: a planted claim token was not found (%q in %q)", l, f)
		}
	}
	if l, f := e2eScan(x.evidence, needles); l != "" {
		t.Fatalf("SECRET LEAK: %q found in %s", l, f)
	}
	t.Log("live-tools evidence logs scanned clean")
}
