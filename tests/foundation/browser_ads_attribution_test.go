//go:build browser

package foundation_test

// AT5 / AT7 / AT9: no browser response interception. Actual production Next
// builds talk to the real Go handlers on disposable PG; only Meta/Stripe are MOCK.
import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"livecommerce/internal/buyerhttp"
	"livecommerce/internal/httpapi"
)

func atBrowserRequire(t *testing.T) {
	t.Helper()
	if os.Getenv("LC_BROWSER_ATTRIBUTION_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Skip("requires formal --browser-ads-attribution disposable PG and production builds")
	}
}

func atBrowserShots(t *testing.T, evidence string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(evidence, "screenshots.json"))
	if err != nil {
		t.Fatal("screenshot manifest missing", err)
	}
	var shots []struct{ File, Sha256, Locale, Viewport string }
	if err = json.Unmarshal(raw, &shots); err != nil || len(shots) < 6 {
		t.Fatal("six hashed screenshot cases required", err)
	}
	cases := map[string]bool{}
	for _, s := range shots {
		if filepath.Base(s.File) != s.File || strings.Contains(s.File, "..") {
			t.Fatal("unsafe screenshot filename")
		}
		pixels, err := os.ReadFile(filepath.Join(evidence, s.File))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(pixels)
		if len(pixels) == 0 || hex.EncodeToString(digest[:]) != s.Sha256 {
			t.Fatal("screenshot byte hash mismatch")
		}
		width := s.Viewport
		if width == "mobile" {
			width = "390"
		}
		if width == "desktop" {
			width = "1586"
		}
		cases[s.Locale+"/"+width] = true
	}
	for _, locale := range []string{"en", "zh-TW", "zh-CN"} {
		for _, width := range []string{"390", "1586"} {
			if !cases[locale+"/"+width] {
				t.Fatal("screenshot case missing", locale, width)
			}
		}
	}
}

func TestBrowserAdsAttributionReport(t *testing.T) {
	atBrowserRequire(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	x := atNewReportEnv(t)
	x.assertReport(t) // strict PG equality before any UI observation
	x.r11BrowserCap(t)
	// Synthetic merchant grants are fixture preparation, not a browser action.
	mustExec(t, x.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'integration:read') ON CONFLICT DO NOTHING`, x.f.tenantA, x.store, x.creator)
	root, _ := filepath.Abs("../..")
	evidence := brfEvidence(t, root, "ads-attribution-report")
	stack := mabStartAdmin(t, ctx, x.f, x.creator, evidence, httpapi.Options{SessionStoreList: true, Ads: x.svc})
	brfPlaywright(t, ctx, stack, []string{"attribution.spec.ts"}, map[string]string{"LC_BROWSER_STORE": x.store, "LC_ATTRIBUTION_FIXTURE": mustJSON(t, x.browserFixture())})
	atBrowserShots(t, evidence)
	x.assertReport(t) // browser reads and refused audience requests cannot alter money/cohort
}

func TestBrowserAdsAttributionCheckout(t *testing.T) {
	atBrowserRequire(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	const origin = "https://buyer.example"
	c := newCapiEnv(t, adsOpts{origin: origin})
	d := c.newDraft(adsDraftIn{})
	// Browser places six distinct real orders (three locales x two sizes).
	// Configuration uses actual merchant settings, and all writes occur before
	// the read-only control server exists. No endpoint fabricates an order.
	for _, permission := range []string{"integration:manage", "orders:read", "fulfillment:write"} {
		mustExec(t, c.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, c.tenant, c.store, c.creator, permission)
	}
	offline := &tcvEnv{t: t, p: c.p.psHarness, merchant: httpapi.NewHandler(c.f.runtime, httpapi.Options{})}
	if st, _ := offline.cofSettings(0, true, false, 72); st != 200 {
		t.Fatalf("bank-transfer settings=%d", st)
	}
	if n := c.count(`SELECT count(*) FROM catalog.products WHERE id=$1 AND status='active'`, c.p.stock.product.ID); n != 1 {
		t.Fatal("browser product must really be published")
	}
	if n := c.count(`SELECT on_hand-reserved-allocated-unavailable FROM inventory.balances WHERE sku_id=$1`, c.p.stock.skus[0].ID); n < 6 {
		t.Fatal("browser fixture has insufficient stock")
	}
	root, _ := filepath.Abs("../..")
	evidence := brfEvidence(t, root, "ads-attribution-checkout")
	controlKey := randomToken()
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Gate-Key") != controlKey {
			http.Error(w, "forbidden", 403)
			return
		}
		if r.Method != "POST" || r.URL.Path != "/order-check" {
			http.NotFound(w, r)
			return
		}
		var in struct {
			OrderID string `json:"order_id"`
			DraftID string `json:"draft_id"`
		}
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&in); err != nil || in.OrderID == "" || in.DraftID != d {
			http.Error(w, "invalid fixture request", 400)
			return
		}
		var order, path, draft string
		// SELECT only, store and draft are fixed by this driver, never by browser scope.
		err := c.f.owner.QueryRow(ctx, `SELECT a.order_id::text,a.path,a.draft_id::text FROM orders.order_attribution a JOIN checkout.orders o ON o.tenant_id=a.tenant_id AND o.store_id=a.store_id AND o.id=a.order_id WHERE a.tenant_id=$1 AND a.store_id=$2 AND a.order_id::text=$3 AND a.draft_id=$4 AND a.path='ad_click' AND o.payment_mode='bank_transfer'`, c.tenant, c.store, in.OrderID, d).Scan(&order, &path, &draft)
		if err != nil {
			http.Error(w, "authoritative attribution absent", 409)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"order_id": order, "path": path, "draft_id": draft})
	}))
	t.Cleanup(control.Close)
	key := base64.RawURLEncoding.EncodeToString(randomBytes(32))
	handler, err := buyerhttp.New(ctx, c.p.a.issuer, c.p.a.runtime, c.p.bcHarness.service, key, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	values := map[string]string{"COMMERCE_BUYER_WEB_ENABLED": "1", "COMMERCE_BUYER_API_ORIGIN": server.URL, "COMMERCE_BUYER_DEMO_LABEL": "1", "COMMERCE_BUYER_BFF_KEY": key, "COMMERCE_BUYER_COOKIE_KEY": base64.RawURLEncoding.EncodeToString(randomBytes(32)), "COMMERCE_BUYER_SESSION_TTL": "3600", "LC_AT_EVIDENCE": evidence, "LC_AT_ORIGIN": origin, "LC_AT_PRODUCT": c.p.stock.product.ID, "LC_AT_DRAFT": d, "LC_AT_CONTROL": control.URL, "LC_AT_CONTROL_KEY": controlKey}
	cmd := exec.CommandContext(ctx, "node", "tests/storefront/ads-attribution.mjs")
	cmd.Dir = root
	cmd.Env = browserEnvironment(values)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	defer func() {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}()
	log := browserLog(t, filepath.Join(evidence, "ads-attribution.mjs.log"))
	cmd.Stdout, cmd.Stderr = log, log
	if err = cmd.Run(); err != nil {
		t.Fatalf("AT5 storefront real-click gate: %v; evidence=%s", err, evidence)
	}
	if n := c.count(`SELECT count(*) FROM orders.order_attribution WHERE tenant_id=$1 AND store_id=$2 AND draft_id=$3 AND path='ad_click'`, c.tenant, c.store, d); n != 6 {
		t.Fatalf("real browser order count=%d want 6", n)
	}
	response := c.api("GET", "/attribution?from="+taipeiDay(0)+"&to="+taipeiDay(0), c.token, nil, nil)
	if response.Status != 200 {
		t.Fatalf("post-browser report status=%d", response.Status)
	}
	found := false
	for _, raw := range response.JSON["drafts"].([]any) {
		row := raw.(map[string]any)
		if row["draft_id"] == d {
			found = true
			paths := row["orders"].([]any)
			if len(paths) != 1 {
				t.Fatal("ad draft must have one ad_click aggregate")
			}
			// Six real origins above are unpaid transfers, not collected performance.
			atNum(t, paths[0].(map[string]any), "orders", 0)
			atNum(t, paths[0].(map[string]any), "pending_orders", 0)
			atNum(t, paths[0].(map[string]any), "net_minor", 0)
			atNum(t, paths[0].(map[string]any), "pending_minor", 0)
		}
	}
	if !found {
		t.Fatal("authoritative browser draft report absent")
	}
	atBrowserShots(t, evidence)
	adminEvidence := brfEvidence(t, root, "ads-attribution-checkout-admin")
	stack := mabStartAdmin(t, ctx, c.f, c.creator, adminEvidence, httpapi.Options{SessionStoreList: true, Ads: c.svc})
	fixture := map[string]any{"from": taipeiDay(0), "to": taipeiDay(0), "draft_id": d, "orders": 0, "net_minor": 0, "pending_orders": 0, "pending_minor": 0, "path": "ad_click"}
	brfPlaywright(t, ctx, stack, []string{"attribution-checkout.spec.ts"}, map[string]string{"LC_BROWSER_STORE": c.store, "LC_ATTRIBUTION_CHECKOUT_FIXTURE": mustJSON(t, fixture)})
	atBrowserShots(t, adminEvidence)
}
