//go:build browser

// Purpose: verify a revised COD order with two prices for one SKU through the buyer return UI.
// Depends on: isolated PG, synthetic shipment and collection commands, and the production storefront browser driver.
// Used by: --browser-order and the order step of --browser-webkit.

package foundation_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/catalog"
	"livecommerce/internal/platform"
)

func TestBrowserR03BuyerReturnLine(t *testing.T) {
	if os.Getenv("LC_BROWSER_ORDER_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-order")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	evidence := brfEvidence(t, root, "r03-buyer-return-line")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	const origin = "https://buyer.example"
	e := tcvNew(t, tcvOpts{origin: origin})
	e.grantCreator("orders:read", "orders:amend", "fulfillment:write")
	if status, out := e.hcodSettings(0, true, 20000, 50, "black_cat"); status != 200 {
		t.Fatalf("synthetic COD settings: %d %v", status, out)
	}
	sku, wh := e.rtSKU()
	mustExec(t, e.p.f.owner, `UPDATE catalog.products SET name='日常帆布提袋' WHERE id=$1`, e.p.stock.product.ID)
	mustExec(t, e.p.f.owner, `UPDATE catalog.skus SET code='TOTE-OLV-M' WHERE id=$1`, sku)
	placed, err := e.hcodPlace(e.newBuyer())
	if err != nil {
		t.Fatal(err)
	}
	another, err := e.hcodPlace(e.newBuyer())
	if err != nil {
		t.Fatal(err)
	}
	if err := platform.WithScope(ctx, e.p.f.runtime, e.token(), e.store(), "pricing:write", func(tx pgx.Tx, scope platform.Scope) error {
		_, err := catalog.SetSKUPrice(ctx, tx, scope, t04Key("r03-buyer-price"), sku,
			catalog.PriceInput{PriceMinor: 1300, ExpectedVersion: e.p.stock.skus[0].Version})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	orderPath := e.rtPath("/orders/" + placed.OrderID)
	draft := fmt.Sprintf(`{"expected_revision":1,"additions":[{"sku_id":%q,"quantity":1}]}`, sku)
	status, review, raw := e.mcall(e.token(), "POST", orderPath+"/amendment/review", "", draft)
	if status != 200 || review["proposal"] == nil {
		t.Fatalf("amend review: %d %s", status, raw)
	}
	body := strings.TrimSuffix(draft, "}") + fmt.Sprintf(`,"proposal":%q}`, review["proposal"])
	status, amended, raw := e.mcall(e.token(), "PUT", orderPath+"/amendment", t04Key("r03-buyer-amend"), body)
	if status != 200 || amended["order_revision"] != float64(2) || amended["cod_collect_minor"] != float64(8800) {
		t.Fatalf("amend: %d %s", status, raw)
	}
	status, _, raw = e.mcall(e.token(), "PUT", orderPath+"/shipment", t04Key("r03-buyer-ship"), r03ShipmentBody(0, 2, "black_cat", "R03-BUYER-SYNTHETIC"))
	if status != 200 {
		t.Fatalf("manual mock shipment: %d %s", status, raw)
	}
	issue := func(order string) string {
		token := randomToken()
		mustExec(t, e.p.f.owner, `INSERT INTO checkout.order_links(token_hash,tenant_id,store_id,owner_id,order_id,expires_at)
      SELECT sha256($2::bytea),tenant_id,store_id,owner_id,id,now()+interval '1 hour' FROM checkout.orders WHERE id=$1`, order, []byte(token))
		return token
	}
	var contact string
	if err := e.p.f.owner.QueryRow(ctx, `SELECT d.phone FROM checkout.orders o JOIN storefront.destination_snapshots d
    ON (d.tenant_id,d.store_id,d.owner_id,d.id)=(o.tenant_id,o.store_id,o.owner_id,o.destination_id) WHERE o.id=$1`, placed.OrderID).Scan(&contact); err != nil {
		t.Fatal(err)
	}
	fixture := map[string]string{"order": placed.OrderID, "other_order": another.OrderID, "contact": contact, "sku_id": sku, "warehouse_id": wh,
		"link": issue(placed.OrderID), "other_link": issue(another.OrderID), "fresh_link": issue(placed.OrderID)}
	readFacts := func(ctx context.Context) ([]byte, error) {
		var out []byte
		err := e.p.f.owner.QueryRow(ctx, `SELECT jsonb_build_object(
      'revision',o.order_revision,'shipment_revision',(SELECT order_revision FROM fulfillment.manual_shipment_versions WHERE order_id=o.id AND status='SHIPPED'),
      'collection',o.collection_state,'total_minor',o.total_minor,'surcharge_minor',o.cod_surcharge_minor,'snapshot_hash',md5(o.snapshot::text),
      'quote_lines',o.snapshot->'quote'->'lines',
      'reservation_lines',(SELECT jsonb_agg(jsonb_build_object('sku_id',l.sku_id,'warehouse_id',l.warehouse_id,'quantity',l.quantity) ORDER BY l.warehouse_id,l.sku_id)
        FROM inventory.reservation_lines l WHERE l.reservation_id=o.id),
      'ledger',(SELECT count(*) FROM inventory.ledger WHERE store_id=o.store_id),
      'payment_attempts',(SELECT count(*) FROM checkout.payment_attempts WHERE store_id=o.store_id),
      'refunds',(SELECT count(*) FROM payments.stripe_refunds WHERE store_id=o.store_id),
      'external_operations',(SELECT count(*) FROM integration.operations WHERE store_id=o.store_id),
      'rmas',(SELECT count(*) FROM returns.rmas WHERE store_id=o.store_id),
      'receipts',(SELECT count(*) FROM returns.buyer_request_receipts WHERE store_id=o.store_id),
      'requests',coalesce((SELECT jsonb_agg(jsonb_build_object('id',r.id,'status',r.status,'version',r.version,
        'lines',(SELECT jsonb_agg(jsonb_build_object('sku_id',l.sku_id,'warehouse_id',l.warehouse_id,'quantity',l.quantity) ORDER BY l.warehouse_id,l.sku_id)
          FROM returns.buyer_request_lines l WHERE l.request_id=r.id)) ORDER BY r.id)
        FROM returns.buyer_requests r WHERE r.store_id=o.store_id),'[]'::jsonb))
      FROM checkout.orders o WHERE o.id=$1 AND o.store_id=$2`, placed.OrderID, e.store()).Scan(&out)
		return out, err
	}
	controlKey := randomToken()
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Gate-Key") != controlKey {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		switch {
		case r.Method == "POST" && r.URL.Path == "/r03/record-synthetic-collection":
			status, _, raw := e.record(e.token(), placed.OrderID, t04Key("r03-buyer-collect"), "PENDING", "collected")
			if status != http.StatusOK {
				t.Errorf("fixture-only COD collection: %d %s", status, raw)
				http.Error(w, "fixture collection failed", 500)
				return
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
		case r.Method == "GET" && r.URL.Path == "/r03/fixture":
			_ = json.NewEncoder(w).Encode(fixture)
		case r.Method == "GET" && r.URL.Path == "/r03/facts":
			out, err := readFacts(r.Context())
			if err != nil {
				t.Error(err)
				http.Error(w, "fixture facts failed", 500)
				return
			}
			_, _ = w.Write(out)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(control.Close)
	child := exec.CommandContext(ctx, "node", "tests/storefront/order-gate.mjs", "r03-return-line")
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	defer func() {
		if child.Process != nil {
			_ = syscall.Kill(-child.Process.Pid, syscall.SIGKILL)
		}
	}()
	child.Dir = root
	child.Env = browserEnvironment(map[string]string{
		"COMMERCE_BUYER_WEB_ENABLED": "1", "COMMERCE_BUYER_API_ORIGIN": e.bh.server.URL, "COMMERCE_BUYER_DEMO_LABEL": "1",
		"COMMERCE_BUYER_BFF_KEY": e.bh.key, "COMMERCE_BUYER_COOKIE_KEY": brToken(), "COMMERCE_BUYER_SESSION_TTL": "3600",
		"LC_ORDER_EVIDENCE": evidence, "LC_ORDER_CONTROL": control.URL, "LC_ORDER_CONTROL_KEY": controlKey,
	})
	log := browserLog(t, filepath.Join(evidence, "browser.log"))
	child.Stdout, child.Stderr = log, log
	runErr := child.Run()
	final, err := readFacts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evidence, "final-pg.json"), final, 0o600); err != nil {
		t.Fatal(err)
	}
	if runErr != nil {
		t.Fatalf("buyer browser run: %v; evidence=%s", runErr, evidence)
	}
	t.Logf("R03 buyer line browser + independent PG readback: %s", evidence)
}
