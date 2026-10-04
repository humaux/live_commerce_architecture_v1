//go:build browser

package foundation_test

// ops-polish independent browser gates (docs/delivery/units/ops-polish.md OP1 buyer half, OP2, OP3 UI half, OP4), written from the brief.
// Run through `bash scripts/dev/test-local.sh --browser-ops-polish` (isolated PG 18, production storefront + admin Next builds). Prefix `bop`.
//   - TestBrowserOpsPolishStorefront: real Chromium, production storefront Next -> private buyerhttp serving checkout.Service.WithoutCardPayment()
//     (what cmd/api builds when COMMERCE_BUYER_PAYMENT_ENABLED is off) -> PG. Script tests/storefront/cvs-pap-only.mjs: zh-TW + en, desktop + 390px.
//     MOCK: no PSP exists in this stack at all (no hosted payment service); ECPay has no profile (buyer_entered).
//   - TestBrowserOpsPolishAdmin: production admin Next + private Go API + signed MOCK IdP; spec tests/admin/ops-polish.spec.ts (OP2 poll on Playwright's
//     fake clock, OP3 finance page/CSV link, OP4 Studio subtitle + nav in three locales). New orders arrive through the real buyer path on demand.
// Owner-pool writes (disclosed fixtures): identity grants (tcvEnv), product name for screenshots.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"livecommerce/internal/buyerhttp"
)

func bopRequire(t *testing.T) {
	t.Helper()
	if os.Getenv("LC_BROWSER_OPS_POLISH_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-ops-polish")
	}
}

func TestBrowserOpsPolishStorefront(t *testing.T) {
	bopRequire(t)
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	root, _ := filepath.Abs("../..")
	evidence := brfEvidence(t, root, "ops-polish-storefront")
	const origin = "https://buyer.example"
	e := tcvNew(t, tcvOpts{origin: origin})
	e.grantCreator("orders:read", "fulfillment:write")
	e.cvsSettings(tcvAllChains, true, "20000", 500)
	e.service("cvs_711", "MANUAL", 0) // the other chains stay unconfigured: only 7-ELEVEN is offered
	mustExec(t, e.p.f.owner, `UPDATE catalog.products SET name='Synthetic ops-polish product',description='Synthetic acceptance fixture' WHERE id=$1`, e.p.stock.product.ID)

	bffKey := base64.RawURLEncoding.EncodeToString(randomBytes(32))
	// the production wiring of cmd/api with COMMERCE_BUYER_PAYMENT_ENABLED off (proven separately by TestOpsPolishOP1APIAssembly)
	handler, err := buyerhttp.New(ctx, e.p.a.issuer, e.p.a.runtime, e.svc.WithoutCardPayment(), bffKey, time.Hour)
	if err != nil {
		t.Fatalf("buyer HTTP constructor: %v", err)
	}
	buyerAPI := newHTTPServer(t, handler)
	values := map[string]string{"COMMERCE_BUYER_WEB_ENABLED": "1", "COMMERCE_BUYER_API_ORIGIN": buyerAPI.URL, "COMMERCE_BUYER_DEMO_LABEL": "1", "COMMERCE_BUYER_BFF_KEY": bffKey,
		"COMMERCE_BUYER_COOKIE_KEY": base64.RawURLEncoding.EncodeToString(randomBytes(32)), "COMMERCE_BUYER_SESSION_TTL": "3600",
		"LC_OPP_EVIDENCE": evidence, "LC_OPP_ORIGIN": origin, "LC_OPP_PRODUCT": e.p.stock.product.ID}
	// the shared harness already holds one card order of its own; only a change during the browser run counts
	cardBefore := e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1 AND payment_mode='card'`, e.store())
	attemptsBefore := e.count(`SELECT count(*) FROM checkout.payment_attempts a JOIN checkout.orders o ON o.id=a.order_id WHERE o.store_id=$1`, e.store())
	cmd := exec.CommandContext(ctx, "node", "tests/storefront/cvs-pap-only.mjs")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	defer func() {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}()
	cmd.Dir = root
	cmd.Env = browserEnvironment(values)
	log := browserLog(t, filepath.Join(evidence, "cvs-pap-only.mjs.log"))
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Run(); err != nil {
		t.Fatalf("storefront browser gate failed: %v; evidence=%s", err, evidence)
	}
	// hashed screenshots: before and after the order for zh-TW + en, desktop + 390px (brfShots wants zh-CN too, which this gate does not cover)
	rawShots, err := os.ReadFile(filepath.Join(evidence, "screenshots.json"))
	if err != nil {
		t.Fatalf("screenshot hash manifest missing: %v; evidence=%s", err, evidence)
	}
	var shots []struct{ File, Sha256, Locale, Viewport string }
	if err := json.Unmarshal(rawShots, &shots); err != nil || len(shots) != 8 {
		t.Fatalf("screenshot manifest has %d entries (want 8): %v", len(shots), err)
	}
	for _, s := range shots {
		if len(s.Sha256) != 64 {
			t.Fatalf("screenshot %s is not hashed", s.File)
		}
	}
	// PG facts: four UI runs = four pay-at-pickup orders; no card order and no payment attempt could have been created
	if n := e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1 AND payment_mode='pay_at_pickup' AND collection_state='PENDING' AND commercial_state='CONFIRMED'`, e.store()); n != 4 {
		t.Errorf("%d CONFIRMED pay-at-pickup orders placed through the UI, want 4 (zh-TW + en, desktop + 390px)", n)
	}
	if n := e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1 AND payment_mode='card'`, e.store()); n != cardBefore {
		t.Errorf("%d card orders were created by the browser run although the store could not take a card payment", n-cardBefore)
	}
	if n := e.count(`SELECT count(*) FROM checkout.payment_attempts a JOIN checkout.orders o ON o.id=a.order_id WHERE o.store_id=$1`, e.store()); n != attemptsBefore {
		t.Errorf("%d payment attempts were created for a store with no payment service", n-attemptsBefore)
	}
	t.Logf("OP1 buyer browser (MOCK, no PSP) passed; evidence=%s", evidence)
}

func TestBrowserOpsPolishAdmin(t *testing.T) {
	bopRequire(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	root, _ := filepath.Abs("../..")
	evidence := brfEvidence(t, root, "ops-polish-admin")
	e := tcvNew(t, tcvOpts{origin: "https://ops.example"})
	// OP4 opens Studio; the shell and real endpoint both require live:read.
	e.grantCreator("orders:read", "orders:export", "fulfillment:write", "integration:manage", "integration:read", "live:read")
	e.cvsSettings(tcvAllChains, true, "20000", 500)
	manual, _, _ := e.service("cvs_711", "MANUAL", 0)

	place := func() string {
		b := e.newBuyer()
		res, err := e.tppPlace(b, manual, e.tppEntered(b, manual), tppName, tppPhone)
		if err != nil {
			t.Errorf("place order: %v", err)
			return ""
		}
		return res.OrderID
	}
	seed := struct {
		Pending      []string `json:"pending"`
		Collected    []string `json:"collected"`
		CollectedMin int64    `json:"collectedMinor"`
	}{}
	seed.Pending = []string{place(), place()}
	for i := 0; i < 2; i++ { // collected today: no cvs_shipments row, so LIVE; counted in today's Taipei finance day
		id := place()
		if st, _, raw := e.mcall(e.token(), "PUT", "/v1/admin/stores/"+e.store()+"/orders/"+id+"/shipment", t04Key("bop-ms"), mfxShip(0, "seven_eleven_cvs", fmt.Sprintf("00%08d", 4000+i))); st != 200 {
			t.Fatalf("manual shipment: %d %s", st, raw)
		}
		if st, _, raw := e.record(e.token(), id, t04Key("bop-collect"), "PENDING", "collected"); st != 200 {
			t.Fatalf("collected: %d %s", st, raw)
		}
		var total int64
		if err := e.p.f.owner.QueryRow(ctx, `SELECT total_minor FROM checkout.orders WHERE id=$1`, id).Scan(&total); err != nil {
			t.Fatal(err)
		}
		seed.Collected, seed.CollectedMin = append(seed.Collected, id), seed.CollectedMin+total
	}
	// a late order is placed by the browser spec through this loopback hook (real buyer path); a failed placement answers 500, never a fake id
	hook := newHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		done := make(chan string, 1)
		go func() {
			id := ""
			defer func() { done <- id }() // runs on t.Fatal's Goexit too
			id = place()
		}()
		id := <-done
		if id == "" {
			http.Error(w, "placement failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"order_id": id})
	}))
	stack := brcStartAdmin(t, ctx, e, evidence)
	raw, _ := json.Marshal(seed)
	brfPlaywright(t, ctx, stack, []string{"ops-polish.spec.ts"}, map[string]string{"LC_BROWSER_STORE": e.store(), "LC_OPP_PLACE_URL": hook.URL, "LC_OPP_SEED": string(raw)})
	t.Logf("OP2/OP3/OP4 admin browser gates (MOCK IdP, real Go API + PG) passed; evidence=%s", evidence)
}
