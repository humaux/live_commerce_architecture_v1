package foundation_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/catalog"
	"livecommerce/internal/integrations/shipping/ecpay/ecpaytest"
	"livecommerce/internal/platform"

	"github.com/jackc/pgx/v5"
)

func r03ShipmentBody(shipmentVersion, orderRevision int64, carrier, tracking string) string {
	return strings.TrimSuffix(mfxShip(shipmentVersion, carrier, tracking), "}") + fmt.Sprintf(`,"order_revision":%d}`, orderRevision)
}

func TestHomeCODAmendmentProviderHold(t *testing.T) {
	e := tcvNew(t)
	e.grantCreator("orders:read", "orders:amend", "fulfillment:write", "integration:manage", "integration:read")
	e.connect("C2C")
	e.cvsSettings(tcvAllChains, true, "20000", 500)
	code, _, _ := e.service("cvs_711", "API", 0)
	e.startDispatcher()
	for _, tc := range []struct {
		mode ecpaytest.CreateMode
		want string
	}{{ecpaytest.Create403, "UNKNOWN"}, {ecpaytest.CreateOK, "CREATED"}} {
		t.Run(tc.want, func(t *testing.T) {
			e.fake.SetCreateMode(tc.mode, "")
			order, _ := e.cvsOrder(tcvOrderSpec{kind: "cvs_711", code: code, paymentMode: "pay_at_pickup"})
			if status, _, raw := e.ship(e.token(), order, 0, "", true); status != 202 {
				t.Fatalf("synthetic provider request: %d %s", status, raw)
			}
			e.awaitShip(order, tc.want)
			trade := e.tradeNo(order)
			prior := e.fake.CreateCalls(trade)
			path := strings.TrimSuffix(e.hcrShipPath(order), "/shipment")
			if status, resp, _ := e.mcall(e.token(), "POST", path+"/amendment/review", "",
				`{"expected_revision":1,"recipient":{"line1":"Out of scope"}}`); status != 409 || cofCode(resp) != "state_changed" {
				t.Fatalf("provider hold: %d %v", status, resp)
			}
			if status, resp, _ := e.mcall(e.token(), "POST", path+"/shipment/document", "", `{"expected_revision":1}`); status != 422 ||
				cofCode(resp) != "not_shippable" {
				t.Fatalf("provider handover cannot use an R03 document: %d %v", status, resp)
			}
			if current, _, _ := e.shipState(order); current != tc.want || e.fake.CreateCalls(trade) != prior ||
				e.count(`SELECT count(*) FROM checkout.order_revisions WHERE order_id=$1`, order) != 0 {
				t.Fatalf("R03 effected provider state %s or wrote a revision", current)
			}
		})
	}
}

func TestHomeCODAmendmentPrepaidHold(t *testing.T) {
	e := rfxNew(t)
	e.startWorker(t)
	paid := e.payMore(t, e.storeFor(t))
	e.grant(t, paid, "orders:amend")
	path := "/v1/admin/stores/" + paid.store() + "/orders/" + paid.order
	status, raw, _ := e.call("POST", path+"/amendment/review", paid.token(),
		map[string]string{"Content-Type": "application/json"},
		`{"expected_revision":1,"recipient":{"line1":"Synthetic rejected change"}}`)
	if status != 409 || cofCode(sraJSON(raw)) != "state_changed" {
		t.Fatalf("prepaid refusal: %d %s", status, raw)
	}
	var revision, revisions, paymentAttempts int
	if err := e.f.owner.QueryRow(context.Background(), `SELECT o.order_revision,
		(SELECT count(*) FROM checkout.order_revisions r WHERE r.order_id=o.id),
		(SELECT count(*) FROM checkout.payment_attempts p WHERE p.order_id=o.id)
		FROM checkout.orders o WHERE o.id=$1`, paid.order).Scan(&revision, &revisions, &paymentAttempts); err != nil {
		t.Fatal(err)
	}
	if revision != 1 || revisions != 0 || paymentAttempts != 1 {
		t.Fatalf("prepaid refusal changed facts: revision=%d history=%d attempts=%d", revision, revisions, paymentAttempts)
	}
}

func TestHomeCODAmendment(t *testing.T) {
	e := tcvNew(t)
	e.grantCreator("orders:read", "orders:amend", "fulfillment:write")
	if status, data := e.hcodSettings(0, true, 20000, 50, "black_cat"); status != http.StatusOK {
		t.Fatalf("enable synthetic COD: %d %v", status, data)
	}
	placed, err := e.hcodPlace(e.newBuyer())
	if err != nil {
		t.Fatal(err)
	}
	path := "/v1/admin/stores/" + e.store() + "/orders/" + placed.OrderID
	address := `{"expected_revision":1,"recipient":{"line1":"Synthetic updated address"}}`
	sku := e.p.stock.skus[0].ID
	initialBalance := e.tppBalance(sku)
	t.Run("review through original order resource", func(t *testing.T) {
		status, _, raw := e.mcall(e.token(), http.MethodPost, path+"/amendment/review", "", address)
		if status != http.StatusOK {
			t.Fatalf("eligible address review: want 200, got %d %s", status, raw)
		}
	})
	t.Run("original dispatch requires reviewed order revision", func(t *testing.T) {
		status, out, _ := e.mcall(e.token(), http.MethodPut, path+"/shipment", t04Key("r03-legacy"), mfxShip(0, "black_cat", "SYNTHETIC123"))
		if status != http.StatusConflict || cofCode(out) != "version_changed" {
			t.Fatalf("first dispatch with only shipment CAS: want 409 version_changed, got %d %v", status, out)
		}
		foreign, _, _ := e.mcall(e.token(), http.MethodPost, strings.Replace(path, e.store(), randomUUID(), 1)+"/amendment/review", "", address)
		if foreign != 404 {
			t.Fatalf("foreign store resolved: %d", foreign)
		}
		csv := tiFile(tiRow(placed.OrderID, "black_cat", "SYNTHETIC123"))
		bulk := func(route string) (int, map[string]any) {
			req := httptest.NewRequest("POST", "/v1/admin/stores/"+e.store()+"/shipments/tracking-import/"+route, strings.NewReader(csv))
			req.Header.Set("Authorization", "Bearer "+e.token())
			req.Header.Set("Content-Type", "text/csv")
			w := httptest.NewRecorder()
			e.merchant.ServeHTTP(w, req)
			return w.Code, sraJSON(w.Body.Bytes())
		}
		status, out = bulk("preview")
		if status != 200 || out["apply_rows"] != float64(0) || out["failed_rows"] != float64(1) {
			t.Fatalf("bulk preview: %d %v", status, out)
		}
		status, out = bulk("commit?expected_apply_rows=1")
		if status != 409 || out["apply_rows"] != float64(0) || out["failed_rows"] != float64(1) ||
			e.count(`SELECT count(*) FROM fulfillment.manual_shipment_versions WHERE order_id=$1`, placed.OrderID) != 0 {
			t.Fatalf("legacy bulk commit: %d %v", status, out)
		}
	})
	t.Run("address only, replay, stale paper and add at current price", func(t *testing.T) {
		call := func(method, resource, key, body string, expected int) map[string]any {
			t.Helper()
			status, out, raw := e.mcall(e.token(), method, path+resource, key, body)
			if status != expected {
				t.Fatalf("%s %s: got %d want %d %s", method, resource, status, expected, raw)
			}
			return out
		}
		old := call("POST", "/shipment/document", "", `{"expected_revision":1}`, 200)
		quote := call("POST", "/amendment/review", "", address, 200)
		save := strings.TrimSuffix(address, "}") + fmt.Sprintf(`,"proposal":%q}`, quote["proposal"])
		key := t04Key("r03-address")
		v2 := call("PUT", "/amendment", key, save, 200)
		replayed := call("PUT", "/amendment", key, save, 200)
		if v2["order_revision"] != float64(2) || replayed["order_revision"] != v2["order_revision"] ||
			v2["cod_collect_minor"] != old["cod_collect_minor"] || initialBalance != e.tppBalance(sku) {
			t.Fatalf("address/replay v2=%v replay=%v balance=%+v", v2, replayed, e.tppBalance(sku))
		}
		if changed := call("PUT", "/amendment", key, strings.Replace(save, "updated", "different", 1), 409); cofCode(changed) != "idempotency_conflict" {
			t.Fatalf("changed key: %v", changed)
		}
		call("POST", "/shipment/document", "", `{"expected_revision":1}`, 409)
		call("POST", "/shipment/document", "", `{"expected_revision":2}`, 200)
		oldConfirm := r03ShipmentBody(0, 1, "black_cat", "R03OLD123")
		call("PUT", "/shipment", t04Key("r03-stale"), oldConfirm, 409)
		add := fmt.Sprintf(`{"expected_revision":2,"additions":[{"sku_id":%q,"quantity":1}]}`, sku)
		evenQuantity := strings.Replace(add, `"quantity":1`, `"quantity":2`, 1)
		obsolete := call("POST", "/amendment/review", "", evenQuantity, 200)
		if err := platform.WithScope(context.Background(), e.p.f.runtime, e.token(), e.store(), "pricing:write", func(tx pgx.Tx, s platform.Scope) error {
			_, err := catalog.SetSKUPrice(context.Background(), tx, s, t04Key("r03-price"), sku,
				catalog.PriceInput{PriceMinor: 1300, ExpectedVersion: e.p.stock.skus[0].Version})
			return err
		}); err != nil {
			t.Fatal(err)
		}
		obsoleteBody := strings.TrimSuffix(evenQuantity, "}") + fmt.Sprintf(`,"proposal":%q}`, obsolete["proposal"])
		if got := call("PUT", "/amendment", t04Key("r03-price-stale"), obsoleteBody, 409); cofCode(got) != "version_changed" {
			t.Fatalf("old offer accepted: %v", got)
		}
		tooMany := fmt.Sprintf(`{"expected_revision":2,"additions":[{"sku_id":%q,"quantity":1000}]}`, sku)
		tooManyOffer := call("POST", "/amendment/review", "", tooMany, 200)
		tooManyBody := strings.TrimSuffix(tooMany, "}") + fmt.Sprintf(`,"proposal":%q}`, tooManyOffer["proposal"])
		if got := call("PUT", "/amendment", t04Key("r03-no-stock"), tooManyBody, 409); cofCode(got) != "insufficient_inventory" ||
			initialBalance != e.tppBalance(sku) {
			t.Fatalf("outside stock bounds: %v", got)
		}
		offer := call("POST", "/amendment/review", "", add, 200)
		addBody := strings.TrimSuffix(add, "}") + fmt.Sprintf(`,"proposal":%q}`, offer["proposal"])
		updated := call("PUT", "/amendment", t04Key("r03-add"), addBody, 200)
		newDoc := call("POST", "/shipment/document", "", `{"expected_revision":3}`, 200)
		items := newDoc["items"].([]any)
		if updated["cod_collect_minor"] != float64(8800) || e.tppBalance(sku).allocated != initialBalance.allocated+1 ||
			items[0].(map[string]any)["unit_price_minor"] != float64(1250) || items[1].(map[string]any)["unit_price_minor"] != float64(1300) {
			t.Fatalf("updated=%v items=%v balance=%+v", updated, items, e.tppBalance(sku))
		}
		if despiteV3 := call("PUT", "/amendment", key, save, 200); despiteV3["order_revision"] != float64(2) {
			t.Fatalf("prior receipt moved with current revision: %v", despiteV3)
		}
		if n := e.count(`SELECT count(*) FROM checkout.order_revisions WHERE order_id=$1`, placed.OrderID); n != 3 {
			t.Fatalf("revision history count=%d", n)
		}
		if n := e.count(`SELECT count(*) FROM inventory.ledger WHERE reservation_id=$1 AND operation='checkout.order_amendment'`, placed.OrderID); n != 1 {
			t.Fatalf("amendment ledger count=%d", n)
		}
		call("POST", "/cancel", t04Key("r03-cancel"), `{"expected_state":"AWAITING_COLLECTION","reason":"Synthetic cancellation"}`, 200)
		if e.tppBalance(sku).allocated != initialBalance.allocated-2 {
			t.Fatalf("cancel did not release the effective reservation: %+v", e.tppBalance(sku))
		}
	})
	t.Run("shipment head and order revision are independent; exact replay survives", func(t *testing.T) {
		order, _ := e.hcrPlace()
		path := e.hcrShipPath(order)
		body := r03ShipmentBody(0, 1, "black_cat", "SYNTHETIC-REPLAY")
		if status, out, _ := e.mcall(e.token(), "PUT", path, t04Key("r03-cas"), r03ShipmentBody(1, 1, "black_cat", "SYNTHETIC-REPLAY")); status != 409 ||
			cofCode(out) != "version_changed" {
			t.Fatalf("correct order revision and stale head: %d %v", status, out)
		}
		key := t04Key("r03-ship-replay")
		status, out, raw := e.mcall(e.token(), "PUT", path, key, body)
		again, _, againRaw := e.mcall(e.token(), "PUT", path, key, body)
		if status != 200 || again != 200 || out["order_revision"] != float64(1) || string(raw) != string(againRaw) {
			t.Fatalf("shipment replay: %d %d %s / %s", status, again, raw, againRaw)
		}
		if n := e.count(`SELECT count(*) FROM fulfillment.manual_shipment_versions WHERE order_id=$1`, order); n != 1 {
			t.Fatalf("shipment rows=%d", n)
		}
		if status, out, _ := e.mcall(e.token(), "POST", strings.TrimSuffix(path, "/shipment")+"/amendment/review", "", address); status != 409 ||
			cofCode(out) != "state_changed" {
			t.Fatalf("amendment after recorded dispatch: %d %v", status, out)
		}
		if status, _, raw := e.record(e.token(), order, t04Key("r03-collected"), "PENDING", "collected"); status != 200 {
			t.Fatalf("record synthetic collection: %d %s", status, raw)
		}
		if status, out, _ := e.mcall(e.token(), "POST", strings.TrimSuffix(path, "/shipment")+"/amendment/review", "", address); status != 409 ||
			cofCode(out) != "state_changed" {
			t.Fatalf("amendment after collected COD: %d %v", status, out)
		}
		if n := e.count(`SELECT count(*) FROM checkout.order_revisions WHERE order_id=$1`, order); n != 0 {
			t.Fatalf("dispatch/collection refusal wrote %d revisions", n)
		}
	})
}

func TestHomeCODAmendmentSerialization(t *testing.T) {
	e := tcvNew(t)
	e.grantCreator("orders:read", "orders:amend", "fulfillment:write")
	e.hcrEnable(50)
	readOnly, _ := e.member("orders:read")
	staff, principal := e.member("orders:read", "orders:amend")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	const draft = `{"expected_revision":1,"recipient":{"line1":"Synthetic amended address"}}`
	const change = `{"line1":"Synthetic amended address"}`
	mainHash := sha256.Sum256([]byte(e.token()))
	staffHash := sha256.Sum256([]byte(staff))
	amend := func(tx pgx.Tx, hash [32]byte, order, key, proposal string) error {
		var receipt []byte
		return tx.QueryRow(ctx, `SELECT identity.amend_order($1,$2,$3,1,$4::jsonb,NULL,$5,$6)`,
			hash[:], e.store(), order, change, key, proposal).Scan(&receipt)
	}
	ship := func(tx pgx.Tx, order, key string) error {
		var receipt []byte
		return tx.QueryRow(ctx, `SELECT fulfillment.record_manual_shipment($1,$2,$3,$4,$5,0,'SHIPPED','black_cat',NULL,'MOCK123',NULL,NULL,NULL,1)`,
			mainHash[:], e.store(), order, key, mainHash[:]).Scan(&receipt)
	}
	proposal := func(t *testing.T, order, token string) string {
		t.Helper()
		url := "/v1/admin/stores/" + e.store() + "/orders/" + order + "/amendment"
		status, out, raw := e.mcall(token, "POST", url+"/review", "", draft)
		if status != 200 {
			t.Fatalf("review=%d %s", status, raw)
		}
		return out["proposal"].(string)
	}
	begin := func(t *testing.T) (pgx.Tx, int) {
		t.Helper()
		tx, err := e.p.f.runtime.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
		var pid int
		if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
			t.Fatal(err)
		}
		return tx, pid
	}
	for _, first := range []string{"amend", "ship", "other edit", "revoke"} {
		t.Run(first, func(t *testing.T) {
			order, _ := e.hcrPlace()
			path := "/v1/admin/stores/" + e.store() + "/orders/" + order
			if status, _, _ := e.mcall(readOnly, "POST", path+"/amendment/review", "", draft); status != 403 {
				t.Fatalf("read-only review status=%d", status)
			}
			mainProposal := proposal(t, order, e.token())
			staffProposal := proposal(t, order, staff)
			winningKey, losingKey := t04Key("r03-winner"), t04Key("r03-loser")
			holder, holderPID := begin(t)
			waiter, waiterPID := begin(t)
			var err error
			switch first {
			case "amend", "other edit":
				err = amend(holder, mainHash, order, winningKey, mainProposal)
			case "ship":
				err = ship(holder, order, winningKey)
			case "revoke":
				_ = holder.Rollback(ctx)
				holder, err = e.p.f.owner.Begin(ctx)
				if err == nil {
					defer holder.Rollback(context.Background())
					err = holder.QueryRow(ctx, `SELECT pg_backend_pid() FROM checkout.orders WHERE id=$1 FOR UPDATE`, order).Scan(&holderPID)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			finished := make(chan error, 1)
			go func() {
				if first == "amend" {
					finished <- ship(waiter, order, losingKey)
				} else {
					finished <- amend(waiter, staffHash, order, losingKey, staffProposal)
				}
			}()
			lmaObserveBlock(t, e.p.f.owner, waiterPID, holderPID, false)
			if first == "revoke" {
				mustExec(t, e.p.f.owner, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='orders:amend'`,
					e.tenant(), e.store(), principal)
			}
			if err := holder.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			err = <-finished
			expected := "PT409"
			if first == "revoke" {
				expected = "PT403"
			}
			if sqlState(err) != expected {
				t.Fatalf("loser expected %s, got %v", expected, err)
			}
			var revision, shipments, payments, receipts int
			var collection string
			if err := e.p.f.owner.QueryRow(ctx, `SELECT o.order_revision,o.collection_state,
				(SELECT count(*) FROM fulfillment.manual_shipment_versions v WHERE v.order_id=o.id),
				(SELECT count(*) FROM checkout.payment_attempts p WHERE p.order_id=o.id),
				(SELECT count(*) FROM ops.command_results c WHERE c.store_id=o.store_id AND c.idempotency_key IN ($2,$3))
				FROM checkout.orders o WHERE o.id=$1`, order, winningKey, losingKey).Scan(&revision, &collection, &shipments, &payments, &receipts); err != nil {
				t.Fatal(err)
			}
			wantRev, wantShip, wantReceipt := 2, 0, 1
			switch first {
			case "ship":
				wantRev, wantShip = 1, 1
			case "revoke":
				wantRev, wantReceipt = 1, 0
			}
			if revision != wantRev || shipments != wantShip || collection != "PENDING" || payments != 0 || receipts != wantReceipt {
				t.Fatalf("facts: revision=%d shipment=%d collection=%s payments=%d receipts=%d", revision, shipments, collection, payments, receipts)
			}
		})
	}
}
