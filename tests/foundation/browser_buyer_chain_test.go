//go:build browser

package foundation_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// Real Chromium -> test-owned TLS edge -> production Next -> private Go -> PG.
// The edge certificate/domain evidence and recipient are synthetic; this is not
// deployment DNS/TLS, carrier, UI-design or payment-provider acceptance.
func TestBrowserBuyerRealChain(t *testing.T) {
	if os.Getenv("LC_BROWSER_BUYER_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-buyer")
	}
	h := bhSetup(t)
	// Visual acceptance uses declared synthetic catalog content through the real
	// database/API. No public fixture route or fabricated production claims.
	mustExec(t, h.f.owner, `UPDATE catalog.products SET name='帆布收納袋（兩入組）',description='一組兩入，方便分類收納日常小物。' WHERE id=$1`, h.stock.product.ID)
	for i, code := range []string{"NAVY-01", "SAND-01"} {
		if i < len(h.stock.skus) {
			mustExec(t, h.f.owner, `UPDATE catalog.skus SET code=$2,price_minor=39000 WHERE id=$1`, h.stock.skus[i].ID, code)
		}
	}
	sfiAxisBySKUCode(t, h.f.owner, h.stock.product.ID) // the storefront shell sells variants: one chip per SKU code
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := os.MkdirTemp(filepath.Join(root, "output/playwright"), "buyer-real-")
	if err != nil {
		t.Fatal(err)
	}
	before := countRows(t, h.f.owner, `SELECT count(*) FROM checkout.orders WHERE store_id=$1`, h.f.storeA1)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "tests/storefront/browser-gate.mjs")
	// A timeout must not orphan the Node runner's two Next children. This fresh
	// process group belongs solely to this fixture, never a name/port-wide kill.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	defer func() {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}()
	cmd.Dir = root
	cmd.Env = browserEnvironment(map[string]string{
		"COMMERCE_BUYER_WEB_ENABLED": "1", "COMMERCE_BUYER_API_ORIGIN": h.server.URL,
		"COMMERCE_BUYER_DEMO_LABEL": "1",
		"COMMERCE_BUYER_BFF_KEY":    h.key, "COMMERCE_BUYER_COOKIE_KEY": brToken(), "COMMERCE_BUYER_SESSION_TTL": "3600",
		"LC_BUYER_EVIDENCE": evidence, "LC_BUYER_SKU": h.stock.skus[0].ID, "LC_BUYER_MARKET": h.market.ID, "LC_BUYER_METHOD": "delivery:" + h.delivery.Code,
	})
	log := browserLog(t, filepath.Join(evidence, "browser.log"))
	cmd.Stdout, cmd.Stderr = log, log
	if err = cmd.Run(); err != nil {
		t.Fatalf("buyer browser gate failed; evidence=%s", evidence)
	}
	data, err := os.ReadFile(filepath.Join(evidence, "result.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		OrderID string `json:"order_id"`
		Cases   int    `json:"cases"`
	}
	if json.Unmarshal(data, &result) != nil || result.OrderID == "" || result.Cases < 8 {
		t.Fatal("missing browser result")
	}
	if countRows(t, h.f.owner, `SELECT count(*) FROM checkout.orders WHERE store_id=$1`, h.f.storeA1)-before != 1 {
		t.Fatal("browser did not create exactly one order")
	}
	var state, store, owner string
	if err = h.f.owner.QueryRow(context.Background(), `SELECT commercial_state,store_id::text,owner_id::text FROM checkout.orders WHERE id=$1`, result.OrderID).Scan(&state, &store, &owner); err != nil {
		t.Fatal(err)
	}
	if state != "DRAFT" || store != h.f.storeA1 {
		t.Fatal("persisted browser order scope/state mismatch")
	}
	if countRows(t, h.f.owner, `SELECT count(*) FROM inventory.reservations WHERE buyer_owner_id=$1`, owner) != 1 {
		t.Fatal("checkout replay duplicated/missed inventory hold")
	}
	t.Logf("PASS: real browser/Next/Go/PG; cases=%d; one order/hold; evidence=%s", result.Cases, evidence)
}
