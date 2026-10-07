//go:build browser

package foundation_test

// W4-U1 (docs/delivery/units/w4-u1-payment-activation-ui.md; contracts/stripe-platform-account-v1.md §0.3, §2, §3.3, §5, §6.4): real browsers.
//
// Stack: an isolated PG 18 container (rfx harness), the real payment worker and the independent Stripe fake (merchant A and the platform store
// pay through the real capture path; evidence class MOCK, no Stripe, no key, test mode only), the real platform Stripe state (designated, OPEN,
// allowlisted through the operator definers of stripeadmin), three weekly settlement statements produced only through the operator paths
// (sync -> close -> payout record: w4u1SeedStatements), the real private Go API (identity + admin routes incl. payments/card and settlements),
// the production admin Next build with a signed mock IdP and, for the buyer half, the real buyerhttp handler behind the production storefront Next build.
//
// Admin half: tests/admin/card-payments.spec.ts and tests/admin/settlements.spec.ts (Playwright; en + zh-TW + zh-CN, desktop 1586x992 + 390 px,
// screenshots hashed). The runner-only control listener changes the platform and the store through the real operator definers (platform close/open,
// store block/unblock, a competing enrollment change); Node never receives a database credential. Buyer half: tests/storefront/collector-buyer.mjs
// (the §5 disclosure above the pay button, on the paid order page, and its absence for a primary connection). Evidence: output/playwright/card-payments/<timestamp>/.

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/buyerhttp"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/payments/platformstripe"
	"livecommerce/internal/platform"
)

func cpbrRequire(t *testing.T) {
	t.Helper()
	if os.Getenv("LC_BROWSER_CARD_PAYMENTS_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-card-payments")
	}
}

// cpbrControl performs the runner-only state changes through the SAME operator definers the CLI uses (never the UI under test). Its handler runs
// on the http server goroutine, so it reports errors as 500 instead of failing the test from there.
type cpbrControl struct {
	e  *pslEnv
	mu sync.Mutex
}

func (c *cpbrControl) platformOpen(ctx context.Context, open bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, err := c.e.reg.PlatformOpen(ctx, c.e.op, "SANDBOX", open, -1, c.e.version)
	if err == nil {
		c.e.version = v
	}
	return err
}

func (c *cpbrControl) block(ctx context.Context, blocked bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.e.reg.PlatformBlock(ctx, c.e.op, c.e.a.f.tenantA, c.e.a.f.storeA1, "SANDBOX", blocked, "op@test", "tk-"+t04Tag())
	return err
}

// bump is another merchant session changing the enrollment (a real enable with a new suffix): the browser's page now holds a stale version.
func (c *cpbrControl) bump(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	token, store := c.e.a.f.tokens["a"], c.e.a.f.storeA1
	var current platformstripe.Summary
	if err := platform.WithScope(ctx, c.e.f.runtime, token, store, "integration:read", func(tx pgx.Tx, s platform.Scope) error {
		var err error
		current, err = platformstripe.Read(ctx, tx, s, token, "PROVIDER_MOCK")
		return err
	}); err != nil {
		return err
	}
	suffix := "BUMPED"
	in := platformstripe.Input{Enabled: true, TermsVersion: pfTerms, DescriptorSuffix: &suffix, ExpectedVersion: current.Version}
	return platform.WithScope(ctx, c.e.f.runtime, token, store, "billing:manage", func(tx pgx.Tx, s platform.Scope) error {
		_, err := platformstripe.Set(ctx, tx, s, token, "PROVIDER_MOCK", in)
		return err
	})
}

// cpbrBuyerNode starts the real buyerhttp handler for the store of p and runs tests/storefront/collector-buyer.mjs against the production
// storefront Next build for one order of that store (the buyer capability token stays inside Go and Node's env).
func cpbrBuyerNode(t *testing.T, ctx context.Context, e *pslEnv, p psHarness, order, expect, phase, storefrontOrigin, evidence string, extra map[string]string) {
	t.Helper()
	root, _ := filepath.Abs("../..")
	if countRows(t, e.f.owner, `SELECT count(*) FROM control.storefront_publications WHERE tenant_id=$1 AND store_id=$2`, p.f.tenantA, p.f.storeA1) == 0 {
		bhPublish(t, p.bcHarness, storefrontOrigin, p.f.tenantA, p.f.storeA1)
	}
	bffKey := base64.RawURLEncoding.EncodeToString(randomBytes(32))
	handler, err := buyerhttp.New(ctx, p.a.issuer, p.a.runtime, p.bcHarness.service, bffKey, time.Hour, e.svc)
	if err != nil {
		t.Fatalf("buyer HTTP constructor: %v", err)
	}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	mustExec(t, e.f.owner, `UPDATE catalog.products SET name='Synthetic browser collector product',description='Synthetic acceptance fixture' WHERE id=$1`, p.stock.product.ID)
	values := map[string]string{"COMMERCE_BUYER_WEB_ENABLED": "1", "COMMERCE_BUYER_API_ORIGIN": server.URL, "COMMERCE_BUYER_DEMO_LABEL": "1", "COMMERCE_BUYER_BFF_KEY": bffKey,
		"COMMERCE_BUYER_COOKIE_KEY": base64.RawURLEncoding.EncodeToString(randomBytes(32)), "COMMERCE_BUYER_SESSION_TTL": "3600",
		"LC_CD_EVIDENCE": evidence, "LC_CD_ORIGIN": storefrontOrigin, "LC_CD_ORDER": order, "LC_CD_BUYER_TOKEN": p.cap.Token, "LC_CD_EXPECT": expect, "LC_CD_PHASE": phase}
	for k, v := range extra {
		values[k] = v
	}
	cmd := exec.CommandContext(ctx, "node", "tests/storefront/collector-buyer.mjs")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	defer func() {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}()
	cmd.Dir = root
	cmd.Env = browserEnvironment(values)
	log := browserLog(t, filepath.Join(evidence, "collector-buyer-"+phase+".log"))
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Run(); err != nil {
		t.Fatalf("storefront browser gate (collector %s) failed: %v; evidence=%s", phase, err, evidence)
	}
}

func TestBrowserCardPayments(t *testing.T) {
	cpbrRequire(t)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Minute)
	defer cancel()
	// platform OPEN and designated; merchants A, B, C allowlisted and enabled; A, the platform store (P) and others paid through the real capture path
	e := pslNew(t)
	seeded := w4u1SeedStatements(t, e)
	store, tenant, principal := e.storeA, e.a.f.tenantA, e.a.f.principalA
	count := func(q string, args ...any) int { return countRows(t, e.f.owner, q, args...) }
	statementsBefore := count(`SELECT count(*) FROM payments.settlement_statements WHERE store_id=$1`, store)
	if statementsBefore != 3 {
		t.Fatalf("fixture: %d statements for store A, want 3", statementsBefore)
	}

	ctl := &cpbrControl{e: e}
	controlKey := randomToken()
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Gate-Key") != controlKey || r.Method != http.MethodPost {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var err error
		switch r.URL.Path {
		case "/platform/close":
			err = ctl.platformOpen(r.Context(), false)
		case "/platform/open":
			err = ctl.platformOpen(r.Context(), true)
		case "/store/block":
			err = ctl.block(r.Context(), true)
		case "/store/unblock":
			err = ctl.block(r.Context(), false)
		case "/store/bump":
			err = ctl.bump(r.Context())
		default:
			http.NotFound(w, r)
			return
		}
		if err != nil {
			t.Errorf("control %s: %v", r.URL.Path, err)
			http.Error(w, "control failed", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(control.Close)

	root, _ := filepath.Abs("../..")
	evidence := brfEvidence(t, root, "card-payments")
	stack := cbbrStartAdmin(t, ctx, e.rfxEnv, principal, evidence, func(string) httpapi.Options {
		return httpapi.Options{SessionStoreList: true, PaymentProfile: "PROVIDER_MOCK"}
	})
	env := map[string]string{
		"LC_BROWSER_STORE": store, "LC_BROWSER_CONTROL": control.URL, "LC_BROWSER_CONTROL_KEY": controlKey,
		"LC_BROWSER_STMT_PAID": seeded.Paid, "LC_BROWSER_STMT_CARRIED": seeded.Carried, "LC_BROWSER_STMT_PENDING": seeded.Pending, "LC_BROWSER_PAYOUT_REF": seeded.PayoutRef,
	}
	brfPlaywright(t, ctx, stack, []string{"card-payments.spec.ts", "settlements.spec.ts"}, env)

	// ---- PG facts after the admin half: the UI drove real commands, once each, and wrote nothing it must not ----
	if n := count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND principal_id=$2 AND action='stripe.platform.disable'`, store, principal); n != 2 {
		t.Errorf("CPU: %d disable audits for store A, want exactly 2 (CPU2 and the CPU4 retry; the stale PUT and the BLOCKED disable write none); evidence=%s", n, evidence)
	}
	if n := count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND principal_id=$2 AND action='stripe.platform.enable'`, store, principal); n != 4 {
		t.Errorf("CPU: %d enable audits for store A, want 4 (the fixture enable, CPU3, the bump, CPU7); evidence=%s", n, evidence)
	}
	var termsVersion string
	var suffix *string
	var enrolledBy string
	if err := e.f.owner.QueryRow(ctx, `SELECT terms_version,descriptor_suffix,enrolled_by::text FROM payments.platform_stripe_enrollments WHERE tenant_id=$1 AND store_id=$2`, tenant, store).Scan(&termsVersion, &suffix, &enrolledBy); err != nil {
		t.Fatalf("CPU: enrollment of store A: %v", err)
	}
	if termsVersion != pfTerms || suffix != nil || enrolledBy != principal {
		t.Errorf("CPU: enrollment terms=%q suffix=%v enrolled_by=%s, want the accepted terms, no suffix (CPU7 cleared it) and the merchant principal", termsVersion, suffix, enrolledBy)
	}
	if n := count(`SELECT count(*) FROM payments.method_heads hd JOIN payments.method_versions mv ON mv.tenant_id=hd.tenant_id AND mv.store_id=hd.store_id AND mv.market_id=hd.market_id
		AND mv.country=hd.country AND mv.code=hd.code AND mv.version=hd.current_version WHERE hd.tenant_id=$1 AND hd.store_id=$2 AND hd.code='stripe_checkout' AND mv.enabled`, tenant, store); n != 1 {
		t.Errorf("CPU: the card method of store A is not enabled after CPU7 (%d rows)", n)
	}
	if n := count(`SELECT count(*) FROM payments.settlement_statements WHERE store_id=$1`, store); n != statementsBefore {
		t.Errorf("STL: the read-only settlements page changed the ledger (%d statements, want %d)", n, statementsBefore)
	}
	if n := count(`SELECT count(*) FROM payments.settlement_statements WHERE store_id=$1 AND payout_ref IS NOT NULL`, store); n != 1 {
		t.Errorf("STL: %d paid statements, want exactly the one recorded by the operator path", n)
	}
	cbbrShots(t, evidence, 14, "en", "zh-TW")
	t.Logf("W4-U1 BROWSER (MOCK Stripe): admin half passed (card-payments + settlements); screenshots hashed; evidence=%s", evidence)

	// ---- buyer half: the §5 disclosure on the real order payment page ----
	summary := e.read(t, e.a)
	if summary.DisplayName == nil || summary.DescriptorPreview == nil {
		t.Fatalf("buyer half: no collector facts in the card summary: %+v", summary)
	}
	derived := map[string]string{"LC_CD_DISPLAY_NAME": *summary.DisplayName, "LC_CD_PREVIEW": *summary.DescriptorPreview}
	e.ensureStock(t, e.oa1)
	unpaid := sstMoreHold(t, e.oa1.s.p) // a payable order: Stripe offered, no attempt yet
	cpbrBuyerNode(t, ctx, e, unpaid, unpaid.hold.OrderID, "unpaid", "unpaid", "https://buyer-a.example", evidence, derived)
	cpbrBuyerNode(t, ctx, e, e.oa1.s.p, e.oa1.order, "paid", "paid", "https://buyer-a.example", evidence, derived)
	cpbrBuyerNode(t, ctx, e, e.op1.s.p, e.op1.order, "primary", "primary", "https://buyer-p.example", evidence, nil)
	cbbrShots(t, evidence, 24, "en", "zh-TW")
	t.Logf("W4-U1 BROWSER buyer half passed (disclosure present above the pay button and on the paid page, absent for a primary connection); evidence=%s", evidence)
}
