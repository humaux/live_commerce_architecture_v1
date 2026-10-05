package foundation_test

// W3-02B pick list / carrier export / CVS batch (unit w3-02b-picklist). Author smoke of the three
// backend features, built on the existing rfx (Stripe capture) and tcv (ECPay fake) harnesses. Every
// order is placed through the real product path (buyer checkout -> Stripe capture -> CONFIRMED +
// MANUAL_UNASSIGNED); the only owner-pool fixtures are the disclosed clones for PL08 (immutable
// synthetic read rows, not payment facts) and the formula-injection recipient name in PL04.

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/catalog"
	integration "livecommerce/internal/integrations/core"
	"livecommerce/internal/integrations/shipping/ecpay/ecpaytest"
	"livecommerce/internal/inventory"
	"livecommerce/internal/platform"
	"livecommerce/internal/storefront"
)

// ---- decode shapes -----------------------------------------------------------------------------

type plLine struct {
	SKUID       string  `json:"sku_id"`
	SKUCode     string  `json:"sku_code"`
	Title       string  `json:"title"`
	OptionLabel *string `json:"option_label"`
	Qty         int64   `json:"qty"`
}

type plOrder struct {
	OrderID     string   `json:"order_id"`
	OrderNumber string   `json:"order_number"`
	Lines       []plLine `json:"lines"`
}

type plSkipped struct {
	OrderID string `json:"order_id"`
	Code    string `json:"code"`
}

type plBody struct {
	GeneratedAt string      `json:"generated_at"`
	Orders      []plOrder   `json:"orders"`
	Totals      []plLine    `json:"totals"`
	Skipped     []plSkipped `json:"skipped"`
}

type plErr struct {
	Code string `json:"code"`
}

// ---- shared helpers ----------------------------------------------------------------------------

// plSKU creates one catalog SKU under the harness product with the given price/stock and returns its id.
func plSKU(t *testing.T, p psHarness, code string, priceMinor, stock int64) string {
	t.Helper()
	ctx := context.Background()
	sku, err := t04Scoped(ctx, p.f, p.f.tokens["a"], p.f.storeA1, "catalog:write", func(tx pgx.Tx, s platform.Scope) (catalog.SKU, error) {
		return catalog.CreateSKU(ctx, tx, s, t04Key("pl-sku"), catalog.SKUInput{ProductID: p.stock.product.ID, Code: code, PriceMinor: priceMinor,
			WeightGrams: 100, LengthMM: 10, WidthMM: 20, HeightMM: 30, OriginCountry: "TW", CustomsName: "test item", HSCandidate: "851840"})
	})
	if err != nil {
		t.Fatalf("create sku %s: %v", code, err)
	}
	if _, err = t04Scoped(ctx, p.f, p.f.tokens["a"], p.f.storeA1, "inventory:write", func(tx pgx.Tx, s platform.Scope) (inventory.Balance, error) {
		return inventory.AdjustOnHand(ctx, tx, s, t04Key("pl-stock"), inventory.Adjustment{WarehouseID: p.stock.warehouse.ID, SKUID: sku.ID, Delta: stock, ExpectedVersion: 0, Reason: "pl fixture stock"})
	}); err != nil {
		t.Fatalf("stock %s: %v", code, err)
	}
	return sku.ID
}

// plHold adds another buyer/hold to the store with the exact items (mirrors sstMoreHold).
func plHold(t *testing.T, p psHarness, items []storefront.Item) psHarness {
	t.Helper()
	q := p
	q.bcHarness.prepare(t, mustIssue(t, p.cqHarness.service, p.f.storeA1), items)
	hold, err := q.bcHarness.begin(t04Key("pl-hold"))
	if err != nil {
		t.Fatal(err)
	}
	q.hold = hold
	return q
}

// plPaidOrder places and pays one more order of the store with custom items (same account/endpoint).
func plPaidOrder(t *testing.T, e *rfxEnv, base rfxOrder, items []storefront.Item) rfxOrder {
	t.Helper()
	s := base.s
	s.p = plHold(t, base.s.p, items)
	return e.pay(t, s, base.endpoint, base.secret)
}

func plPickBody(ids []string) string {
	raw, _ := json.Marshal(map[string][]string{"order_ids": ids})
	return string(raw)
}

func (e *rfxEnv) plPickList(t *testing.T, o rfxOrder, token, body string) (int, plBody, []byte) {
	t.Helper()
	status, raw, _ := e.call("POST", "/v1/admin/stores/"+o.store()+"/orders/pick-list", token, nil, body)
	var out plBody
	if status == 200 {
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("pick-list body: %v (%s)", err, raw)
		}
	}
	return status, out, raw
}

func (e *rfxEnv) plExport(t *testing.T, o rfxOrder, token, template string, ids []string) (int, []byte, http.Header) {
	t.Helper()
	return e.call("POST", "/v1/admin/stores/"+o.store()+"/orders/export?template="+template, token, nil, plPickBody(ids))
}

// totalsBySKU folds the totals into sku_id -> qty.
func totalsBySKU(lines []plLine) map[string]int64 {
	out := map[string]int64{}
	for _, l := range lines {
		out[l.SKUID] += l.Qty
	}
	return out
}

// TestPickList covers PL01 (totals merge), PL02 (skipped codes), PL03 (501 -> 422 too_many) and
// PL05 (permission split on one shared rfx store).
func TestPickList(t *testing.T) {
	e := rfxNew(t)
	e.startWorker(t)
	base := e.payInStore(t, e.storeFor(t))

	a := plSKU(t, base.s.p, "PL-A-"+t04Tag(), 2000, 200)
	b := plSKU(t, base.s.p, "PL-B-"+t04Tag(), 3000, 200)
	c := plSKU(t, base.s.p, "PL-C-"+t04Tag(), 4000, 200)

	o1 := plPaidOrder(t, e, base, []storefront.Item{{SKUID: a, Quantity: 2}, {SKUID: b, Quantity: 1}})
	o2 := plPaidOrder(t, e, base, []storefront.Item{{SKUID: a, Quantity: 3}})
	o3 := plPaidOrder(t, e, base, []storefront.Item{{SKUID: b, Quantity: 2}, {SKUID: c, Quantity: 1}})

	t.Run("PL01 totals merge and per-order lines", func(t *testing.T) {
		status, body, raw := e.plPickList(t, base, base.token(), plPickBody([]string{o1.order, o2.order, o3.order}))
		if status != 200 {
			t.Fatalf("pick-list answered %d: %s", status, raw)
		}
		if body.GeneratedAt == "" || len(body.Orders) != 3 || len(body.Skipped) != 0 || len(body.Totals) != 3 {
			t.Fatalf("shape: orders=%d totals=%d skipped=%d", len(body.Orders), len(body.Totals), len(body.Skipped))
		}
		totals := totalsBySKU(body.Totals)
		if totals[a] != 5 || totals[b] != 3 || totals[c] != 1 {
			t.Fatalf("totals by sku: %v", totals)
		}
		// totals are sorted by sku_code ascending (PL-A < PL-B < PL-C).
		for i := 1; i < len(body.Totals); i++ {
			if body.Totals[i-1].SKUCode >= body.Totals[i].SKUCode {
				t.Fatalf("totals not sorted by sku_code: %v", body.Totals)
			}
		}
		byOrder := map[string][]plLine{}
		for _, o := range body.Orders {
			if !strings.HasPrefix(o.OrderNumber, "LC-") || len(o.OrderNumber) != 35 {
				t.Fatalf("order_number %q", o.OrderNumber)
			}
			for _, l := range o.Lines {
				if l.OptionLabel != nil || l.Title == "" || l.Qty < 1 {
					t.Fatalf("bad line %+v", l)
				}
			}
			byOrder[o.OrderID] = o.Lines
		}
		want := map[string]map[string]int64{
			o1.order: {a: 2, b: 1},
			o2.order: {a: 3},
			o3.order: {b: 2, c: 1},
		}
		for id, lines := range want {
			got := map[string]int64{}
			for _, l := range byOrder[id] {
				got[l.SKUID] += l.Qty
			}
			if len(got) != len(lines) {
				t.Fatalf("order %s lines %v", id, got)
			}
			for sku, qty := range lines {
				if got[sku] != qty {
					t.Fatalf("order %s sku %s qty %d want %d", id, sku, got[sku], qty)
				}
			}
		}
	})

	t.Run("PL02 shipped/cancelled/foreign skipped codes", func(t *testing.T) {
		e.mustShip(t, base, 0, "sf_express", "SF1") // base -> not_pickable
		cancelled := e.payMore(t, base)
		mustExec(t, e.f.owner, `UPDATE checkout.orders SET commercial_state='CANCELLED' WHERE id=$1`, cancelled.order)
		foreign := e.payInStore(t, e.storeFor(t)) // another store: order_not_found (indistinct from random)
		ghost := randomUUID()

		status, body, raw := e.plPickList(t, base, base.token(), plPickBody([]string{o1.order, base.order, cancelled.order, foreign.order, ghost}))
		if status != 200 {
			t.Fatalf("pick-list answered %d: %s", status, raw)
		}
		if len(body.Orders) != 1 || body.Orders[0].OrderID != o1.order {
			t.Fatalf("orders: %v", body.Orders)
		}
		skipped := map[string]string{}
		for _, s := range body.Skipped {
			skipped[s.OrderID] = s.Code
		}
		if skipped[base.order] != "not_pickable" || skipped[cancelled.order] != "not_pickable" ||
			skipped[foreign.order] != "order_not_found" || skipped[ghost] != "order_not_found" {
			t.Fatalf("skipped: %v", skipped)
		}
		if len(body.Skipped) != 4 {
			t.Fatalf("skipped len=%d", len(body.Skipped))
		}
	})

	t.Run("PL03 501 ids -> 422 too_many", func(t *testing.T) {
		ids := make([]string, 501)
		for i := range ids {
			ids[i] = randomUUID()
		}
		status, _, raw := e.plPickList(t, base, base.token(), plPickBody(ids))
		if status != http.StatusUnprocessableEntity {
			t.Fatalf("501 ids answered %d: %s", status, raw)
		}
		var env plErr
		if err := json.Unmarshal(raw, &env); err != nil || env.Code != "too_many" {
			t.Fatalf("501 ids code=%q err=%v", env.Code, err)
		}
	})

	t.Run("PL05 permission split", func(t *testing.T) {
		viewer, _ := e.member(t, base, "orders:read")
		exporter, _ := e.member(t, base, "orders:export")
		if status, _, raw := e.plPickList(t, base, viewer, plPickBody([]string{o1.order})); status != 200 {
			t.Fatalf("viewer pick-list answered %d: %s", status, raw)
		}
		if status, _, _ := e.plExport(t, base, viewer, "black_cat", []string{o1.order}); status != http.StatusForbidden {
			t.Fatalf("viewer export answered %d, want 403", status)
		}
		// orders:export alone cannot pass the reader's orders:read fence.
		if status, _, _ := e.plExport(t, base, exporter, "black_cat", []string{o1.order}); status != http.StatusForbidden {
			t.Fatalf("exporter-only export answered %d, want 403", status)
		}
	})
}

// TestPickList500 covers PL08: 500 pickable rows answered inside 10 s. The 500 rows are immutable
// synthetic read fixtures cloned from one real paid order (unique id/job_id/cart_version), not
// payment facts.
func TestPickList500(t *testing.T) {
	e := rfxNew(t)
	e.startWorker(t)
	base := e.payInStore(t, e.storeFor(t))
	// Cloning a CONFIRMED order also has to clone its stock reservation: order_reservation_fk is the
	// composite (tenant,store,id,owner_id,creator_session_id) -> inventory.reservations and reservation.id
	// equals the order id, so a new order id without a matching reservation violates it. One data-modifying
	// CTE inserts the 500 reservations and the 500 orders in the same statement (the two deferred FKs are
	// checked at commit). The generated ids are materialized once so both inserts agree.
	mustExec(t, e.f.owner, `WITH gen AS MATERIALIZED (
	  SELECT n, gen_random_uuid() AS id, 7000000000::bigint + n AS job FROM generate_series(1,500) AS n
	), src AS MATERIALIZED (
	  SELECT o.tenant_id,o.store_id,o.owner_id,o.creator_session_id,r.state AS r_state,r.generation AS r_generation
	  FROM checkout.orders o JOIN inventory.reservations r
	   ON r.tenant_id=o.tenant_id AND r.store_id=o.store_id AND r.id=o.id
	  WHERE o.id=$1
	), ins_res AS (
	  INSERT INTO inventory.reservations(tenant_id,store_id,id,state,expires_at,created_at,checkout_id,buyer_owner_id,buyer_session_id,generation)
	  SELECT s.tenant_id,s.store_id,g.id,s.r_state,clock_timestamp(),clock_timestamp(),g.id,s.owner_id,s.creator_session_id,s.r_generation
	  FROM src s CROSS JOIN gen g
	), ins_ord AS (
	  INSERT INTO checkout.orders
	  SELECT (jsonb_populate_record(NULL::checkout.orders, to_jsonb(o)||jsonb_build_object(
	   'id',g.id,'job_id',g.job,'cart_version',1000000::bigint+g.n,
	   'commercial_state','CONFIRMED','fulfillment_state','MANUAL_UNASSIGNED',
	   'created_at','2026-01-01T00:00:00Z','updated_at','2026-01-01T00:00:00Z','expires_at','2026-01-01T00:15:00Z'
	  ))).* FROM checkout.orders o CROSS JOIN gen g WHERE o.id=$1
	)
	SELECT count(*) FROM gen`, base.order)

	ids := make([]string, 0, 500)
	rows, err := e.f.owner.Query(context.Background(), `SELECT id::text FROM checkout.orders WHERE job_id BETWEEN 7000000000 AND 7000000500 ORDER BY job_id`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if len(ids) != 500 {
		t.Fatalf("cloned %d rows, want 500", len(ids))
	}

	start := time.Now()
	status, body, raw := e.plPickList(t, base, base.token(), plPickBody(ids))
	elapsed := time.Since(start)
	if status != 200 {
		t.Fatalf("500 pick-list answered %d: %s", status, raw)
	}
	if len(body.Orders) != 500 || len(body.Skipped) != 0 || len(body.Totals) == 0 {
		t.Fatalf("500 shape: orders=%d skipped=%d totals=%d", len(body.Orders), len(body.Skipped), len(body.Totals))
	}
	if elapsed >= 10*time.Second {
		t.Fatalf("500 pick-list took %v, want < 10s", elapsed)
	}
	t.Logf("PL08 500-order pick list: %v", elapsed)
}

// TestCarrierExport covers PL04: each template's exact column list, formula-injection protection,
// and the collect/total cells equal to the frozen order amount.
func TestCarrierExport(t *testing.T) {
	e := rfxNew(t)
	e.startWorker(t)
	base := e.payInStore(t, e.storeFor(t))
	// Disclosed owner fixture: plant a spreadsheet formula in the recipient name.
	mustExec(t, e.f.owner, `UPDATE checkout.orders SET snapshot=jsonb_set(snapshot,'{destination,recipient_name}','"=1+1"') WHERE id=$1`, base.order)
	var total int64
	if err := e.f.owner.QueryRow(context.Background(), `SELECT total_minor FROM checkout.orders WHERE id=$1`, base.order).Scan(&total); err != nil {
		t.Fatal(err)
	}

	templates := []struct {
		name, header         string
		hasCollect, hasTotal bool
	}{
		{"black_cat", "order_id,recipient_name,phone,region,city,line1,line2,items,collect_minor", true, false},
		{"hsinchu", "order_id,order_number,recipient_name,phone,region,city,line1,line2,items,collect_minor", true, false},
		{"chunghwa_post", "order_id,recipient_name,phone,country,region,city,postal_code,line1,line2,items,total_minor", false, true},
		{"generic", "order_id,order_number,created_at_utc,destination_kind,recipient_name,phone,region,city,line1,line2,pickup_code,items,total_minor,collect_minor", true, true},
	}
	for _, tc := range templates {
		t.Run(tc.name, func(t *testing.T) {
			status, raw, hdr := e.plExport(t, base, base.token(), tc.name, []string{base.order})
			if status != 200 {
				t.Fatalf("export answered %d: %s", status, raw)
			}
			if ct := hdr.Get("Content-Type"); ct != "text/csv; charset=utf-8" {
				t.Fatalf("content-type %q", ct)
			}
			if !strings.HasPrefix(string(raw), "\xEF\xBB\xBF") {
				t.Fatalf("missing UTF-8 BOM")
			}
			r := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(raw), "\xEF\xBB\xBF")))
			records, err := r.ReadAll()
			if err != nil {
				t.Fatal(err)
			}
			if len(records) != 2 || strings.Join(records[0], ",") != tc.header {
				t.Fatalf("header=%v want %q", records[0], tc.header)
			}
			idx := map[string]int{}
			for i, h := range records[0] {
				idx[h] = i
			}
			if got := records[1][idx["recipient_name"]]; got != "'=1+1" {
				t.Fatalf("recipient cell %q, want formula-guarded '=1+1", got)
			}
			want := strconv.FormatInt(total, 10)
			if tc.hasCollect && records[1][idx["collect_minor"]] != want {
				t.Fatalf("collect cell %q want %s", records[1][idx["collect_minor"]], want)
			}
			if tc.hasTotal && records[1][idx["total_minor"]] != want {
				t.Fatalf("total cell %q want %s", records[1][idx["total_minor"]], want)
			}
		})
	}
	if n := e.mfxAudit(t, base, "orders.carrier_export"); n != 4 {
		t.Fatalf("carrier_export audit rows=%d, want 4", n)
	}
}

// TestCVSBatch covers PL06: 3 CVS orders queue exactly 3 existing River jobs, a same-key replay adds
// none, and a home-delivery order is refused failed:not_cvs (not order_not_found).
func TestCVSBatch(t *testing.T) {
	e := tcvNew(t, tcvOpts{stripe: true})
	e.r.startWorker(t)
	// Pay the store's DRAFT home hold FIRST (before cvsOrder advances e.p.hold) so the batch sees a
	// CONFIRMED card-paid home order in the same store.
	home := e.r.payInStore(t, e.ro)
	e.grantCreator("orders:read", "fulfillment:write", "integration:manage", "integration:read")
	e.connect("C2C")
	e.cvsSettings(tcvAllChains, true, "20000", 500)
	code, _, _ := e.service("cvs_711", "API", 0)
	var cvsIDs []string
	for i := 0; i < 3; i++ {
		id, _ := e.cvsOrder(tcvOrderSpec{kind: "cvs_711", code: code, paymentMode: "pay_at_pickup"})
		cvsIDs = append(cvsIDs, id)
	}

	key := t04Key("pl-batch")
	all := append(append([]string{}, cvsIDs...), home.order)
	body, _ := json.Marshal(map[string][]string{"order_ids": all})
	path := "/v1/admin/stores/" + e.store() + "/shipments/cvs-batch"
	status, out, raw := e.mcall(e.token(), "POST", path, key, string(body))
	if status != 200 {
		t.Fatalf("batch answered %d: %s", status, raw)
	}
	results, _ := out["results"].([]any)
	if len(results) != 4 {
		t.Fatalf("batch results=%v", results)
	}
	outcomes := map[string]map[string]any{}
	for _, r := range results {
		m := r.(map[string]any)
		outcomes[m["order_id"].(string)] = m
	}
	for _, id := range cvsIDs {
		if outcomes[id]["outcome"] != "queued" || outcomes[id]["code"] != nil {
			t.Fatalf("cvs order %s outcome %v", id, outcomes[id])
		}
		if e.count(`SELECT count(*) FROM fulfillment.cvs_shipments WHERE order_id=$1 AND state='REQUESTED'`, id) != 1 {
			t.Fatalf("cvs order %s has no REQUESTED shipment", id)
		}
	}
	if outcomes[home.order]["outcome"] != "failed" || outcomes[home.order]["code"] != "not_cvs" {
		t.Fatalf("home order outcome %v", outcomes[home.order])
	}
	jobsBefore := e.count(`SELECT count(*) FROM river.river_job WHERE kind='external_operation_v1' AND args->>'operation_id' IN
	 (SELECT id::text FROM integration.operations WHERE tenant_id=$1 AND provider='ecpay_logistics')`, e.tenant())

	// Same key + same body replays the stored result without any new job.
	status, out2, raw2 := e.mcall(e.token(), "POST", path, key, string(body))
	if status != 200 {
		t.Fatalf("replay answered %d: %s", status, raw2)
	}
	if len(out2["results"].([]any)) != 4 {
		t.Fatalf("replay results=%v", out2["results"])
	}
	jobsAfter := e.count(`SELECT count(*) FROM river.river_job WHERE kind='external_operation_v1' AND args->>'operation_id' IN
	 (SELECT id::text FROM integration.operations WHERE tenant_id=$1 AND provider='ecpay_logistics')`, e.tenant())
	if jobsAfter != jobsBefore {
		t.Fatalf("replay changed job count %d -> %d", jobsBefore, jobsAfter)
	}
}

// TestCVSBatchUnknown covers PL07: one CVS order queued, the existing dispatcher turns the fake ECPay
// timeout into UNKNOWN, and the batch does not re-queue it (a single attempt, a single job).
func TestCVSBatchUnknown(t *testing.T) {
	e := tcvNew(t, tcvOpts{stripe: true})
	e.r.startWorker(t)
	e.startDispatcherWith(func(o *integration.DispatcherOptions) { o.CallTimeout = time.Second })
	e.routeNewJobs() // the batch result has no operation_id; route the new job as it appears.
	e.grantCreator("orders:read", "fulfillment:write", "integration:manage", "integration:read")
	e.connect("C2C")
	e.cvsSettings(tcvAllChains, true, "20000", 500)
	code, _, _ := e.service("cvs_711", "API", 0)
	order, _ := e.cvsOrder(tcvOrderSpec{kind: "cvs_711", code: code, paymentMode: "pay_at_pickup"})
	e.fake.SetCreateMode(ecpaytest.CreateTimeoutNothing, "")

	key := t04Key("pl-batch-unknown")
	body, _ := json.Marshal(map[string][]string{"order_ids": []string{order}})
	status, out, raw := e.mcall(e.token(), "POST", "/v1/admin/stores/"+e.store()+"/shipments/cvs-batch", key, string(body))
	if status != 200 {
		t.Fatalf("batch answered %d: %s", status, raw)
	}
	results, _ := out["results"].([]any)
	if len(results) != 1 || results[0].(map[string]any)["outcome"] != "queued" {
		t.Fatalf("batch results=%v", results)
	}
	e.awaitShip(order, "UNKNOWN")
	if state, attempt, _ := e.shipState(order); state != "UNKNOWN" || attempt != 1 {
		t.Fatalf("shipment state=%s attempt=%d, want UNKNOWN/1", state, attempt)
	}
	if n := e.count(`SELECT count(*) FROM fulfillment.cvs_shipments WHERE order_id=$1`, order); n != 1 {
		t.Fatalf("shipment rows=%d, want 1 (no requeue)", n)
	}
}
