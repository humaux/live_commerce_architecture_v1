//go:build browser

// Purpose: Real-click route/journey acceptance with isolated identity/domain fixtures and actual optional merchant surfaces.
// Depends on: production Next/Go handlers, PG/tcvEnv, comment stream, inbox/msgtemplates, signed MOCK IdP and buyer HTTPS edge.
// Used by: --browser-click-sweep/--browser-visual-lint; empty inbox uses its authorized API, never a missing-route fallback.
package foundation_test

// G-UI8 real-click sweep (owner 2026-10-03; docs/engineering/ui-architecture.md 10.6b). `TestBrowserClickSweep`, prefix `cs`.
// Run through `bash scripts/dev/test-local.sh --browser-click-sweep` (isolated PG 18, production admin + storefront Next builds).
//
// ONE stack, built on the tcvNew harness (taiwan_cvs_env_test.go): the production admin Next build (signed MOCK IdP, the store creator holds the
// owner staff role and every permission of the live catalogue) over the real private Go API with every optional surface mounted (CVS, ECPay MOCK
// fake, bank transfer / cash on delivery settings, promotions, manual orders, Studio + keyword claims, Meta connect over a loopback fake Graph,
// Meta ads over the ads fake Graph, platform billing over the billingtest fake, provider accounts, store domains), and the production storefront
// Next build (started by the runner) over the real buyerhttp handler behind a synthetic TLS edge for https://buyer.example.
//
// Seeded through the real merchant / buyer paths (owner-pool writes, all disclosed): store_staff owner row and the full permission catalogue for the
// store creator, the harness product renamed + its SKU axis, the storefront-shell catalogue (two more products, two collections, photos) created
// through the admin API, a COD order, a bank-transfer order, a CVS buyer_entered bank-transfer order and an ECPay pay-at-pickup order, one promotion,
// one connected MOCK Meta Page (merchant connect start -> callback -> pick against the fake Graph), one Studio draft session.
//
// The runner tests/ui/click-sweep.mjs then drives real Chromium (real Playwright clicks only) over every admin registry route and every storefront
// route at 1586x992 / 390x844 in zh-TW (+ en at desktop), plus four end-to-end journeys, and writes output/ui-click-sweep/ledger.{json,md}. This test
// reads PG afterwards (journey facts, destructive actions changed nothing). Evidence labels: BROWSER, MOCK (no PSP, no carrier, no Meta, no IdP).
// CI (gates.yml) runs this test once per LC_SWEEP_SHARD=i/N slice, each with its own PG and seeded store; tests/ui/sweep-aggregate.mjs proves the slices cover everything.
// Depends-on: tcvEnv/tcvBuyer (taiwan_cvs_env_test.go), mabStartAdmin (browser_meta_ads_test.go), mcn* (meta_connect_test.go), brf* helpers.

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
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/ads"
	"livecommerce/internal/billing"
	"livecommerce/internal/billing/billingtest"
	"livecommerce/internal/buyerhttp"
	"livecommerce/internal/claims"
	"livecommerce/internal/fulfillment"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/identity"
	"livecommerce/internal/identityhttp"
	"livecommerce/internal/inbox"
	"livecommerce/internal/integrations/accounts"
	core "livecommerce/internal/integrations/core"
	metaoauth "livecommerce/internal/integrations/meta/oauth"
	metaads "livecommerce/internal/integrations/meta_ads"
	"livecommerce/internal/integrations/metabridge"
	"livecommerce/internal/live"
	"livecommerce/internal/metaconnect"
	"livecommerce/internal/msgtemplates"
	"livecommerce/internal/oidclogin"
	"livecommerce/internal/platform"
	adsfake "livecommerce/tests/ads/fakegraph"
	"livecommerce/tests/metaconnect/fakegraph"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

const csOrigin = "https://buyer.example"

func csRequire(t *testing.T) {
	t.Helper()
	if os.Getenv("LC_BROWSER_CLICK_SWEEP_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-click-sweep")
	}
}

type csProduct struct {
	ID    string `json:"id"`
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

// csFacts is handed to the runner (a file; never a credential).
type csFacts struct {
	Store       string            `json:"store"`
	StoreName   string            `json:"store_name"`
	StoreOrigin string            `json:"store_origin"`
	Products    []csProduct       `json:"products"`
	Collections []csProduct       `json:"collections"`
	Orders      map[string]string `json:"orders"`
	Customer    string            `json:"customer"`
	Session     string            `json:"session"`
	Promo       string            `json:"promo"`
	MetaPage    string            `json:"meta_page"`
	JourneyTag  string            `json:"journey_tag"`
}

func TestBrowserClickSweep(t *testing.T) {
	csRequire(t)
	ctx, cancel := context.WithTimeout(context.Background(), csSweepBudget)
	defer cancel()
	root, _ := filepath.Abs("../..")
	evidence := brfEvidence(t, root, "click-sweep")
	outDir := filepath.Join(root, "output", "ui-click-sweep")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	e := tcvNew(t, tcvOpts{origin: csOrigin})
	f := e.p.f
	owner := f.owner
	leaseUntil, err := csFixtureLeaseUntil(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	// bhPublish's one-hour synthetic domain lease is shorter than this gate's
	// 85-minute budget. Align only this isolated fixture; production expiry stays enforced.
	result, err := owner.Exec(ctx, `UPDATE control.storefront_domains SET valid_until=$4
		WHERE tenant_id=$1 AND store_id=$2 AND origin=$3`, f.tenantA, f.storeA1, csOrigin, leaseUntil)
	if err != nil || result.RowsAffected() != 1 {
		t.Fatalf("align task-owned domain lease: rows=%d error=%v", result.RowsAffected(), err)
	}
	// The store creator is the merchant the MOCK IdP signs in: the owner staff role and every permission of the live catalogue.
	mustExec(t, owner, `INSERT INTO identity.store_staff(tenant_id,store_id,principal_id,role) VALUES($1,$2,$3,'owner') ON CONFLICT DO NOTHING`, f.tenantA, f.storeA1, f.principalA)
	mustExec(t, owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission)
		SELECT $1,$2,$3,p FROM unnest(identity.staff_permission_catalogue()) p ON CONFLICT DO NOTHING`, f.tenantA, f.storeA1, f.principalA)
	e.topUp()
	facts := csFacts{Store: e.store(), StoreName: "fixture-store-a1", StoreOrigin: csOrigin, Orders: map[string]string{}, JourneyTag: t04Tag()}

	// ---- every optional merchant surface, mounted the way cmd/api mounts it -------------------------------------------------------------
	labels, err := claims.NewLabelKey(randomBytes(32))
	if err != nil {
		t.Fatal(err)
	}
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
	// Meta connect over the loopback fake Graph (mcn* constants/helpers of meta_connect_test.go).
	metaFake := fakegraph.New()
	t.Cleanup(metaFake.Close)
	metaFake.ExpectApp(miApp, miSecret, mcnRedirect)
	graph, err := metaoauth.NewGraph(metaFake.URL(), mcnVersion, nil)
	if err != nil {
		t.Fatal(err)
	}
	seal, _ := mcnPageRing(t)
	metaJobs, err := newInsertOnlyClient(f)
	if err != nil {
		t.Fatal(err)
	}
	metaSvc, err := metaconnect.New(metaconnect.Config{Graph: graph, App: metaoauth.App{ID: miApp, RedirectURI: mcnRedirect, Secret: []byte(miSecret)},
		ConfigID: mcnConfig, GraphVersion: mcnVersion, PageAppID: miApp, IGAppID: miApp, StateKey: metaconnect.StateKeyFor([]byte(miSecret)), Seal: seal}, metaJobs)
	if err != nil {
		t.Fatal(err)
	}
	// Meta ads over the ads fake Graph (newAdsEnv's merchant surface; the page shows its not-connected state, the dialog is external).
	adsGraph := adsfake.New()
	t.Cleanup(adsGraph.Close)
	adsGraph.ExpectApp(adsApp, adsAppSecret, adsRedirect)
	adsPub, _ := adsKeys(t)
	adsSeal, err := metaads.LoadSealKeys(func(k string) string { return adsPub[k] })
	if err != nil {
		t.Fatal(err)
	}
	adsOAuth, err := metaads.NewOAuth(metaads.Config{GraphBaseURL: adsGraph.URL(), GraphVersion: adsVersion, PartnerAgent: adsPartner}, metaads.AppConfig{AppID: adsApp, RedirectURI: adsRedirect, AppSecret: []byte(adsAppSecret)}, adsSeal)
	if err != nil {
		t.Fatal(err)
	}
	adsJobs, err := river.NewClient[pgx.Tx](riverpgxv5.New(f.runtime), &river.Config{Schema: "river"})
	if err != nil {
		t.Fatal(err)
	}
	adsSvc, err := ads.NewService(adsJobs, adsOAuth.Connect, ads.DialogConfig{AppID: adsApp, ConfigID: adsConfigID, RedirectURI: adsRedirect, GraphVersion: adsVersion, StateKey: bytes.Repeat([]byte{7}, 32)})
	if err != nil {
		t.Fatal(err)
	}
	// Platform billing over the billingtest fake (checkout.stripe.com / billing.stripe.com are external: the runner blocks them at the network edge).
	billFake := billingtest.New("acct_ClickSweep0001")
	t.Cleanup(billFake.Close)
	billCfg := billing.Config{SecretKey: "sk_" + "test_" + fmt.Sprintf("%x", randomBytes(12)), WebhookSecret: "whsec_" + fmt.Sprintf("%x", randomBytes(16)),
		ReturnOrigin: "https://admin.example.test", PriceIDs: []string{"price_ClickSweepMonth"}}
	billFake.RequireKey(billCfg.SecretKey)
	billFake.AddPrice("price_ClickSweepMonth", 30000, "twd", "month", "Pro Monthly", true, false)
	billSvc, err := billing.New(ctx, f.runtime, billCfg, billFake.Transport())
	if err != nil || !billSvc.Enabled() {
		t.Fatalf("billing service: enabled=%v err=%v", billSvc != nil && billSvc.Enabled(), err)
	}
	// Match cmd/api's configured stream; absence of a bound source stays not_started, not bridge_disabled.
	bridgeToken := randomBytes(32)
	bridgeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/internal/v1/comment-page" || r.Header.Get("Authorization") != "Bearer "+base64.StdEncoding.EncodeToString(bridgeToken) {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(metabridge.BridgePage{Epoch: 1, Items: []metabridge.BridgeComment{}, Stream: metabridge.BridgeStreamState{
			State: "not_started", PollIntervalMs: 5000, SourcePlatform: "facebook", Reason: "no_source",
		}})
	}))
	t.Cleanup(bridgeServer.Close)
	bridge, err := metabridge.NewBridgeClient(bridgeServer.URL, bridgeToken)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := live.NewCommentStream(bridge, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The /messages route is part of the live UI registry, even when this store has no conversations.
	payloadJSON := fmt.Sprintf(`{"keys":[{"id":"click_sweep_mock","key_base64":%q}]}`, base64.StdEncoding.EncodeToString(randomBytes(32)))
	payloadKeys, err := inbox.LoadKeyring(func(name string) string {
		switch name {
		case "COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID":
			return "click_sweep_mock"
		case "COMMERCE_META_PAYLOAD_KEYS_JSON":
			return payloadJSON
		}
		return ""
	})
	if err != nil {
		t.Fatal("MOCK inbox payload keyring configuration failed")
	}
	inboxService, err := inbox.NewService(payloadKeys)
	if err != nil {
		t.Fatal("MOCK inbox service configuration failed")
	}
	templates := msgtemplates.NewService()
	inboxService.EnableSend(seal, metaJobs, templates)
	options := httpapi.Options{SessionStoreList: true, CVS: e.cvs, Accounts: accountService, Studio: true, CommentStream: stream, ClaimLabels: &labels, RefundJobs: e.jobs,
		Inbox: inboxService, MsgTemplates: templates,
		MetaConnect: metaSvc, Ads: adsSvc, Billing: billSvc, ManualOrders: mtManualOrders(t, e), StoreBaseDomain: "lctest.example",
		PaymentProfile: "PROVIDER_MOCK"} // W4-U1: mounts payments/card so /settings/payments/card renders its real (platform NONE) state instead of "not available"
	api := httpapi.NewHandler(f.runtime, options)
	call := func(method, path, key string, body any, want int, out any) {
		t.Helper()
		var rd io.Reader // nil for a bodyless request (a typed-nil reader would send Content-Length 0 and fail validation)
		if body != nil {
			raw, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			rd = bytes.NewReader(raw)
		}
		req := httptest.NewRequest(method, "/v1/admin/stores/"+e.store()+path, rd)
		req.Header.Set("Authorization", "Bearer "+e.token())
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		w := httptest.NewRecorder()
		api.ServeHTTP(w, req)
		if w.Code != want {
			t.Fatalf("%s %s: %d (want %d) %s", method, path, w.Code, want, w.Body.String())
		}
		if out != nil {
			if err := json.Unmarshal(w.Body.Bytes(), out); err != nil {
				t.Fatalf("%s %s: %v: %s", method, path, err, w.Body.String())
			}
		}
	}

	// The same handler/options are handed to csStartAdmin; prove the empty-list route before browser startup.
	var emptyInbox struct {
		Items []json.RawMessage `json:"items"`
	}
	call(http.MethodGet, "/inbox/conversations?filter=all&limit=50", "", nil, http.StatusOK, &emptyInbox)
	if emptyInbox.Items == nil || len(emptyInbox.Items) != 0 {
		t.Fatal("fresh sweep store must return an authorized empty inbox array")
	}
	unauthenticatedInbox := httptest.NewRecorder()
	api.ServeHTTP(unauthenticatedInbox, httptest.NewRequest("GET", "/v1/admin/stores/"+e.store()+"/inbox/conversations?filter=all&limit=50", nil))
	if unauthenticatedInbox.Code != http.StatusUnauthorized {
		t.Fatal("empty inbox route must still require authentication")
	}

	// ---- storefront-shell catalogue through the admin API (the harness product already exists: rename it, give it a SKU axis) -------------
	mustExec(t, owner, `UPDATE catalog.products SET name='Sweep Cedar Candle',description='Synthetic acceptance fixture' WHERE id=$1`, e.p.stock.product.ID)
	sfiAxisBySKUCode(t, owner, e.p.stock.product.ID)
	var slug string
	if err := owner.QueryRow(ctx, `SELECT slug FROM catalog.products WHERE id=$1`, e.p.stock.product.ID).Scan(&slug); err != nil {
		t.Fatal(err)
	}
	facts.Products = append(facts.Products, csProduct{ID: e.p.stock.product.ID, Slug: slug, Title: "Sweep Cedar Candle"})
	admin := v2Admin{t: t, h: api, token: e.token(), store: e.store()}
	for i, p := range []struct{ slug, title, code string }{{"sweep-wool-scarf", "Sweep Wool Scarf", "SWP-SCF"}, {"sweep-ceramic-mug", "Sweep Ceramic Mug", "SWP-MUG"}} {
		var made struct {
			ID string `json:"id"`
		}
		admin.do("POST", "/products", t04Key("cs-p"), map[string]any{"name": p.title, "slug": p.slug, "description": "Synthetic acceptance fixture " + p.title, "status": "active"}, 200, &made)
		var s v2SKU
		admin.do("POST", "/skus", t04Key("cs-sku"), map[string]any{"product_id": made.ID, "code": p.code, "price_minor": 9800 + int64(i)*3000, "weight_grams": 300, "length_mm": 100, "width_mm": 100, "height_mm": 50}, 200, &s)
		admin.do("POST", "/inventory/adjustments", t04Key("cs-adj"), map[string]any{"warehouse_id": e.p.stock.warehouse.ID, "sku_id": s.ID, "delta": 40, "expected_version": 0, "reason": "click sweep"}, 200, nil)
		data, ctype := v2Multipart(t, v2PNG(t, uint8(30+i)))
		admin.raw("POST", "/products/"+made.ID+"/images", t04Key("cs-img"), data, ctype, 200, nil)
		facts.Products = append(facts.Products, csProduct{ID: made.ID, Slug: p.slug, Title: p.title})
	}
	for _, c := range []struct{ slug, title string }{{"sweep-home", "Sweep home"}, {"sweep-wear", "Sweep wear"}} {
		var made struct {
			ID      string `json:"id"`
			Version int64  `json:"version"`
		}
		admin.do("POST", "/collections", t04Key("cs-col"), map[string]any{"title": c.title, "slug": c.slug, "description": c.title + " picks"}, 200, &made)
		ids := []string{facts.Products[0].ID, facts.Products[1].ID}
		if c.slug == "sweep-wear" {
			ids = []string{facts.Products[1].ID, facts.Products[2].ID}
		}
		admin.do("PUT", "/collections/"+made.ID+"/products", t04Key("cs-set"), map[string]any{"expected_version": made.Version, "product_ids": ids}, 200, nil)
		facts.Collections = append(facts.Collections, csProduct{ID: made.ID, Slug: c.slug, Title: c.title})
	}

	// ---- the storefront design (a published home, a header/footer menu and the merchant page /pages/about) through the design admin API ----------
	design := map[string]any{
		"profile": map[string]any{"name": "Sweep Goods", "tagline": "Synthetic click-sweep storefront", "logo_image_id": nil, "favicon_image_id": nil, "accent_color": "#2f6b5a",
			"announcement": "Free delivery over $200 · synthetic announcement",
			"contact": map[string]any{"email": "hello@sweep.example", "phone": "+886 2 2345 6789", "address": "100 Heping East Road, Taipei",
				"line_url": "https://line.me/R/ti/p/@sweep", "facebook_url": "https://www.facebook.com/sweep.example", "instagram_url": "https://www.instagram.com/sweep.example"}},
		"nav": map[string]any{
			"header": []any{map[string]any{"label": "All products", "kind": "all_products", "target": nil}, map[string]any{"label": "Sweep home", "kind": "collection", "target": "sweep-home"},
				map[string]any{"label": "About us", "kind": "page", "target": "about"}},
			"footer": []any{map[string]any{"label": "Sweep wear", "kind": "collection", "target": "sweep-wear"}}},
		"home": map[string]any{"sections": []any{
			map[string]any{"type": "featured_collection", "collection_slug": "sweep-home", "heading": "Sweep home", "limit": 8},
			map[string]any{"type": "rich_text", "heading": "How we choose", "body": "Every item is **synthetic**.\n\n- honest\n- repairable"},
			map[string]any{"type": "product_grid", "heading": "Just in", "sort": "newest", "limit": 8}}},
		"pages": []any{map[string]any{"slug": "about", "title": "About us", "body": "A small synthetic shop in **Taipei**."}},
	}
	var drafted struct {
		Version int64 `json:"version"`
	}
	call("PUT", "/design/draft", "", map[string]any{"expected_version": 0, "document": design}, 200, &drafted)
	call("POST", "/design/publish", "", map[string]any{"expected_draft_version": drafted.Version}, 200, nil)

	// ---- delivery + payment settings and orders of every kind, through the real merchant and buyer paths -------------------------------------
	e.cvsSettings(tcvAllChains, true, "20000", 500)
	if st, out := e.hcodSettings(0, true, 20000, 50, "black_cat"); st != 200 {
		t.Fatalf("COD settings: %d %v", st, out)
	}
	if st, out := e.cofSettings(0, true, true, 72); st != 200 {
		t.Fatalf("bank transfer settings: %d %v", st, out)
	}
	e.startDispatcher()
	manual, _, _ := e.service("cvs_711", "MANUAL", 0)
	cvsBuyer := e.newBuyer()
	cvsBank, err := e.cofPlaceCVS(cvsBuyer, manual)
	if err != nil {
		t.Fatalf("CVS bank-transfer order: %v", err)
	}
	facts.Orders["cvs_bank_transfer"] = cvsBank.OrderID
	e.updateService(manual, func(in *fulfillment.ServiceInput) { in.Enabled = false })
	e.connect("C2C")
	for _, k := range []struct{ kind, mode string }{{"cvs_711", "API"}, {"cvs_familymart", "API"}, {"cvs_hilife", "MANUAL"}, {"cvs_okmart", "MANUAL"}} {
		code, _, _ := e.service(k.kind, k.mode, 0)
		if k.kind == "cvs_711" {
			e.apiCode = code
		}
	}
	e.startDispatcherWith(nil)
	e.routeNewJobs()
	codBuyer := e.newBuyer()
	cod, err := e.hcodPlace(codBuyer)
	if err != nil {
		t.Fatalf("COD order: %v", err)
	}
	facts.Orders["cash_on_delivery"] = cod.OrderID
	bankBuyer := e.newBuyer()
	bank, err := e.cofPlaceHome(bankBuyer, "sweep.buyer@example.invalid")
	if err != nil {
		t.Fatalf("bank-transfer order: %v", err)
	}
	facts.Orders["bank_transfer"] = bank.OrderID
	// the buyer reports the transfer (last five digits + amount): the order reaches SUBMITTED, where the merchant sees Confirm / Reject
	if res := bankBuyer.cofProof(bank.OrderID, t04Key("cs-proof"), "12345", e.cogTotal(bank.OrderID)); res.status != 200 {
		t.Fatalf("bank transfer proof: %d %s", res.status, res.body)
	}
	papOrder, _ := e.cvsOrder(tcvOrderSpec{kind: "cvs_711", code: e.apiCode, paymentMode: "pay_at_pickup"})
	facts.Orders["cvs_pay_at_pickup"] = papOrder
	// A CVS order whose ECPay label exists (the merchant "request label" action, dispatched to the fake): the label print page needs one.
	labelOrder, _ := e.cvsOrder(tcvOrderSpec{kind: "cvs_711", code: e.apiCode, paymentMode: "pay_at_pickup"})
	if st, _, raw := e.ship(e.token(), labelOrder, 0, "", true); st != 202 {
		t.Fatalf("request the CVS label: %d %s", st, raw)
	}
	e.awaitShip(labelOrder, "CREATED")
	facts.Orders["cvs_label_created"] = labelOrder
	if err := owner.QueryRow(ctx, `SELECT owner_id::text FROM checkout.orders WHERE id=$1`, cod.OrderID).Scan(&facts.Customer); err != nil {
		t.Fatal(err)
	}
	promoID, _ := e.proMust("SWEEP10", nil)
	facts.Promo = promoID

	// ---- one connected MOCK Meta Page (merchant connect: start -> dialog -> callback -> pick) ------------------------------------------------
	page := mcnPage("Sweep Page", true)
	metaFake.AddCode("SWEEP-SYNTH-CODE", fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{page}})
	var started struct {
		DialogURL string `json:"dialog_url"`
		StateID   string `json:"state_id"`
	}
	call("POST", "/meta-connect/start", t04Key("cs-meta"), nil, 201, &started)
	dialog, err := url.Parse(started.DialogURL)
	if err != nil {
		t.Fatal(err)
	}
	var cb struct {
		StateID string `json:"state_id"`
	}
	call("GET", "/meta-connect/callback?code=SWEEP-SYNTH-CODE&state="+url.QueryEscape(dialog.Query().Get("state")), "", nil, 200, &cb)
	call("POST", "/meta-connect/pick", t04Key("cs-pick"), map[string]any{"state_id": cb.StateID, "page_id": page.ID, "include_instagram": true}, 201, nil)
	facts.MetaPage = page.Name

	// ---- one Studio draft session (the Studio page and its claims workspace need a scene) -------------------------------------------------
	var draft live.Draft
	if err := platform.WithScope(ctx, f.runtime, e.token(), e.store(), "live:manage", func(tx pgx.Tx, s platform.Scope) (err error) {
		draft, err = live.CreateDraft(ctx, tx, s, e.token(), t04Key("cs-draft"), live.DraftInput{Title: "Click sweep scene " + t04Tag(), AspectRatio: "9:16"})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	facts.Session = draft.ID

	// ---- the stack: buyer API (storefront Next is started by the runner), admin Next ----------------------------------------------------------
	bffKey := base64.RawURLEncoding.EncodeToString(randomBytes(32))
	buyerHandler, err := buyerhttp.New(ctx, e.p.a.issuer, e.p.a.runtime, e.svc, bffKey, time.Hour)
	if err != nil {
		t.Fatalf("buyer HTTP constructor: %v", err)
	}
	buyerAPI := newHTTPServer(t, buyerHandler)
	// Password login (the /signup and /reset routes, the Team page's BFF) is on next to the MOCK IdP: the same wiring as TestBrowserPasswordAuth
	// (loopback SMTP + HIBP fakes), one private BFF key for both identity handlers.
	pw := newPwa(t, pwaCap(5000))
	stack := csStartAdmin(t, ctx, f, f.principalA, evidence, options, pw)

	// Runner-only control listener (random key): the "nothing changed" fingerprint of the state a destructive action could touch.
	controlKey := randomToken()
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Gate-Key") != controlKey || r.Method != http.MethodGet || r.URL.Path != "/fingerprint" {
			http.Error(w, "forbidden", 403)
			return
		}
		var raw []byte
		if err := owner.QueryRow(r.Context(), csFingerprintSQL, f.tenantA, f.storeA1).Scan(&raw); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
	t.Cleanup(control.Close)

	factsPath := filepath.Join(evidence, "facts.json")
	raw, _ := json.MarshalIndent(facts, "", "  ")
	if err := os.WriteFile(factsPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	before := csFingerprint(t, owner, f.tenantA, f.storeA1)

	// ---- the runner ---------------------------------------------------------------------------------------------------------------------------
	values := map[string]string{"COMMERCE_BUYER_WEB_ENABLED": "1", "COMMERCE_BUYER_API_ORIGIN": buyerAPI.URL, "COMMERCE_BUYER_DEMO_LABEL": "1", "COMMERCE_BUYER_BFF_KEY": bffKey,
		"COMMERCE_BUYER_COOKIE_KEY": base64.RawURLEncoding.EncodeToString(randomBytes(32)), "COMMERCE_BUYER_SESSION_TTL": "3600",
		"LC_SWEEP_ADMIN_ORIGIN": stack.origin, "LC_SWEEP_STORE": e.store(), "LC_SWEEP_EVIDENCE": evidence, "LC_SWEEP_OUT": outDir, "LC_SWEEP_FACTS": factsPath,
		"LC_SWEEP_CONTROL": control.URL, "LC_SWEEP_CONTROL_KEY": controlKey, "LC_SWEEP_ONLY": os.Getenv("LC_SWEEP_ONLY"), "LC_SWEEP_PAGES": os.Getenv("LC_SWEEP_PAGES"),
		"LC_SWEEP_WORKERS": os.Getenv("LC_SWEEP_WORKERS"), "LC_SWEEP_SHARD": os.Getenv("LC_SWEEP_SHARD"),
		"LC_SWEEP_INJECT_FAULT": os.Getenv("LC_SWEEP_INJECT_FAULT")} // SHARD=i/N: CI runs one slice per job (tests/ui/sweep-shard-lib.mjs); INJECT_FAULT: gate calibration only
	// Developer affordance (never set by the gate): LC_SWEEP_HOLD=<seconds> keeps the seeded stack up after writing runner.env, so the runner can be
	// re-run by hand (`env $(cat runner.env) node tests/ui/click-sweep.mjs`) without re-seeding. The file holds only this ephemeral stack's random keys.
	if hold := os.Getenv("LC_SWEEP_HOLD"); hold != "" {
		var lines []byte
		for k, v := range values {
			lines = append(lines, []byte(k+"="+v+"\n")...)
		}
		_ = os.WriteFile(filepath.Join(evidence, "runner.env"), lines, 0o600)
		secs, _ := strconv.Atoi(hold)
		t.Logf("HOLD: stack up for %ds; runner env in %s", secs, filepath.Join(evidence, "runner.env"))
		select {
		case <-time.After(time.Duration(secs) * time.Second):
		case <-ctx.Done():
		}
		return
	}
	cmd := exec.CommandContext(ctx, "node", "tests/ui/click-sweep.mjs")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	defer func() {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}()
	cmd.Dir = root
	cmd.Env = browserEnvironment(values)
	log := browserLog(t, filepath.Join(evidence, "click-sweep.mjs.log"))
	cmd.Stdout, cmd.Stderr = log, log
	runnerErr := cmd.Run()

	// ---- PG facts: the journeys really wrote through the UI; destructive controls changed nothing ----------------------------------------------
	jraw, err := os.ReadFile(filepath.Join(outDir, "journeys.json"))
	var journeys struct {
		ProductTitle string `json:"product_title"`
		CodOrder     string `json:"cod_order"`
		Domain       string `json:"domain"`
	}
	if err == nil && json.Unmarshal(jraw, &journeys) == nil {
		if journeys.ProductTitle != "" {
			if n := countRows(t, owner, `SELECT count(*) FROM catalog.products WHERE store_id=$1 AND name=$2 AND status='active'`, f.storeA1, journeys.ProductTitle); n != 1 {
				t.Errorf("journey 1: %d active products named %q created in the editor, want 1", n, journeys.ProductTitle)
			}
		}
		if journeys.CodOrder != "" {
			if n := countRows(t, owner, `SELECT count(*) FROM checkout.orders WHERE id=$1 AND payment_mode='cash_on_delivery' AND store_id=$2`, journeys.CodOrder, f.storeA1); n != 1 {
				t.Errorf("journey 2: the storefront COD order %s is missing in PG", journeys.CodOrder)
			}
			if n := countRows(t, owner, `SELECT count(*) FROM checkout.orders WHERE id=$1 AND fulfillment_state='MERCHANT_SHIPPED' AND collection_state='COLLECTED'`, journeys.CodOrder); n != 1 {
				t.Errorf("journey 3: order %s is not MERCHANT_SHIPPED + COLLECTED after the admin clicks", journeys.CodOrder)
			}
		}
		if journeys.Domain != "" {
			if n := countRows(t, owner, `SELECT count(*) FROM control.storefront_domains WHERE store_id=$1 AND origin LIKE '%'||$2`, f.storeA1, journeys.Domain); n < 1 {
				t.Errorf("journey 4: no storefront_domains row for %s", journeys.Domain)
			}
		}
	} else {
		t.Errorf("journeys.json missing or unreadable: %v", err)
	}
	after := csFingerprint(t, owner, f.tenantA, f.storeA1)
	for k, v := range after.destructive {
		if before.destructive[k] != v {
			t.Errorf("a destructive control changed state (%s: %q -> %q); the sweep must stop at the confirmation dialog", k, before.destructive[k], v)
		}
	}
	if runnerErr != nil {
		t.Fatalf("click sweep FAILED (exit: %v); the ledger and the failing controls are in %s (ledger.md, DEFECTS in the runner log %s)", runnerErr, outDir, filepath.Join(evidence, "click-sweep.mjs.log"))
	}
	t.Logf("G-UI8 click sweep passed; ledger=%s evidence=%s", outDir, evidence)
}

// csFingerprintSQL: counts per state of everything a destructive control (delete/archive/void/disconnect/refund/cancel/publish) could change.
// Journey writes are excluded by name elsewhere: this is read before and after the whole run and only the keys below are compared.
const csFingerprintSQL = `SELECT jsonb_build_object(
  'products_by_status', (SELECT coalesce(jsonb_object_agg(status,n),'{}') FROM (SELECT status,count(*) n FROM catalog.products WHERE tenant_id=$1 AND store_id=$2 GROUP BY status) x),
  'collections', (SELECT count(*) FROM catalog.collections WHERE tenant_id=$1 AND store_id=$2),
  'orders_by_state', (SELECT coalesce(jsonb_object_agg(k,n),'{}') FROM (SELECT commercial_state||'/'||fulfillment_state||'/'||coalesce(collection_state,'-') k,count(*) n FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2 GROUP BY 1) x),
  'meta_bindings_enabled', (SELECT count(*) FROM integration.bindings WHERE tenant_id=$1 AND store_id=$2 AND enabled),
  'publications', (SELECT coalesce(jsonb_object_agg(published::text,n),'{}') FROM (SELECT published,count(*) n FROM control.storefront_publications WHERE tenant_id=$1 AND store_id=$2 GROUP BY published) x),
  'domains_by_state', (SELECT coalesce(jsonb_object_agg(state,n),'{}') FROM (SELECT state,count(*) n FROM control.storefront_domains WHERE store_id=$2 GROUP BY state) x),
  'staff_by_role', (SELECT coalesce(jsonb_object_agg(role,n),'{}') FROM (SELECT role,count(*) n FROM identity.store_staff WHERE tenant_id=$1 AND store_id=$2 GROUP BY role) x),
  'sessions', (SELECT count(*) FROM live.sessions WHERE tenant_id=$1 AND store_id=$2)
)`

type csPrint struct {
	all         string
	destructive map[string]string
}

func csFingerprint(t *testing.T, pool interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, tenant, store string) csPrint {
	t.Helper()
	var raw []byte
	if err := pool.QueryRow(context.Background(), csFingerprintSQL, tenant, store).Scan(&raw); err != nil {
		t.Fatalf("fingerprint: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	out := csPrint{all: string(raw), destructive: map[string]string{}}
	// products/orders/domains legitimately gain rows through the journeys: compare only the keys a destructive control would
	// decrease or flip when it ran (archived/cancelled/voided/disconnected/unpublished), never the creating ones.
	for _, k := range []string{"meta_bindings_enabled", "publications", "staff_by_role", "collections", "sessions"} {
		out.destructive[k] = string(m[k])
	}
	var products map[string]int
	_ = json.Unmarshal(m["products_by_status"], &products)
	out.destructive["products_archived"] = fmt.Sprint(products["archived"])
	var orders map[string]int
	_ = json.Unmarshal(m["orders_by_state"], &orders)
	cancelled := 0
	for k, n := range orders {
		if bytes.Contains([]byte(k), []byte("CANCEL")) || bytes.Contains([]byte(k), []byte("VOID")) || bytes.Contains([]byte(k), []byte("REFUND")) {
			cancelled += n
		}
	}
	out.destructive["orders_cancelled_or_refunded"] = fmt.Sprint(cancelled)
	return out
}

// csStartAdmin is mabStartAdmin (browser_meta_ads_test.go) for the sweep: the admin Next build runs with merchant password login ON next to the signed
// MOCK IdP (the /signup, /reset and Team routes answer 404 without it), and the private Go API mounts the password handler of `pw` beside the identity
// handler under the one BFF key.
func csStartAdmin(t *testing.T, ctx context.Context, f *testFixture, principal, evidence string, options httpapi.Options, pw *pwaEnv) *brfStack {
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
	origin := "http://" + address
	idp := newBrowserIDP(t, origin+"/api/auth/callback")
	mustExec(t, f.owner, `INSERT INTO identity.external_identities(issuer,subject,principal_id) VALUES($1,'browser-subject',$2)`, idp.server.URL, principal)
	role := "cs_" + strings.ReplaceAll(randomUUID(), "-", "")
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
	now := time.Now()
	leaseUntil, err := csFixtureLeaseUntil(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	// Test-only signed sessions must survive the gate's remaining bounded work.
	// Permission/CSRF/revocation checks and dedicated expired-session negatives are unchanged.
	service, err := identity.New(authority, observedBrowserProvider{Provider: provider, t: t}, identity.Policy{ProviderKey: "browser-click-sweep-v1", SessionTTL: leaseUntil.Sub(now), OnboardingEnabled: true, Currencies: []string{"TWD", "USD"}})
	if err != nil {
		t.Fatal(err)
	}
	private, err := identityhttp.NewHandler(service, pw.bffKey)
	if err != nil {
		t.Fatal(err)
	}
	staffService, err := identity.NewStaff(pw.pool, pw.mailer, origin) // invitations are mailed through the loopback SMTP fake
	if err != nil {
		t.Fatal(err)
	}
	staffHandler, err := identityhttp.NewStaffHandler(staffService, pw.bffKey)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/v1/identity/staff/", staffHandler) // the mount cmd/api's withStaff makes (longest pattern wins)
	mux.Handle("/v1/identity/password/", pw.handler)
	mux.Handle("/v1/identity/", private)
	mux.Handle("/", httpapi.NewHandler(f.runtime, options))
	api := httptest.NewServer(mux)
	t.Cleanup(api.Close)
	nextLog := browserLog(t, filepath.Join(evidence, "next.log"))
	next := exec.CommandContext(ctx, "node", filepath.Join(root, "apps/admin/.next/standalone/apps/admin/server.js"))
	next.Dir = root
	next.Env = browserEnvironment(map[string]string{"HOSTNAME": "127.0.0.1", "PORT": port, "NODE_ENV": "production", "COMMERCE_IDENTITY_ENABLED": "1", "COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS": "1",
		"COMMERCE_PASSWORD_LOGIN_ENABLED": "1", "COMMERCE_PUBLIC_ORIGIN": origin, "COMMERCE_API_ORIGIN": api.URL, "COMMERCE_OIDC_ISSUER": idp.server.URL, "COMMERCE_BFF_KEY": pw.bffKey,
		"COMMERCE_ONBOARDING_ENABLED": "1", "COMMERCE_ONBOARDING_CURRENCIES": "TWD,USD"})
	next.Stdout, next.Stderr = nextLog, nextLog
	next.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := next.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Kill(-next.Process.Pid, syscall.SIGKILL); _, _ = next.Process.Wait() })
	client := &http.Client{Timeout: time.Second}
	for attempt := 0; ; attempt++ {
		if response, err := client.Get(origin + "/api/stores"); err == nil {
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
