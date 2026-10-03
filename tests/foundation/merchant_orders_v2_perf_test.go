package foundation_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/httpapi"
	"livecommerce/internal/merchantorders"
	"livecommerce/internal/platform"
	"livecommerce/internal/storefront"
)

// Ten thousand explicitly synthetic archived read fixtures, NOT payment facts or
// transition acceptance evidence. Clone immutable real checkout references, use
// unique order/job IDs and cancelled carts so all original FK/checks stay on.
func TestMerchantOrdersV2TenThousandScopedSearch(t *testing.T) {
	q := pqSetup(t)
	moGrant(t, q.f, q.f.tenantA, q.f.storeA1, q.f.principalA)
	ctx := context.Background()
	mustExec(t, q.f.owner, `WITH inserted AS (INSERT INTO checkout.orders
 SELECT (jsonb_populate_record(NULL::checkout.orders, to_jsonb(o)||jsonb_build_object(
  'id',gen_random_uuid(),'job_id',9000000000::bigint+n,'commercial_state','CANCELLED','fulfillment_state','CANCELLED',
  'created_at','2026-01-01T00:00:00Z','updated_at','2026-01-01T00:00:00Z','expires_at','2026-01-01T00:15:00Z',
  'snapshot',jsonb_set(jsonb_set(o.snapshot,'{destination,phone}',to_jsonb('+88690000'||lpad(n::text,4,'0'))),'{destination,recipient_name}',to_jsonb('Synthetic recipient '||n))
 ))).* FROM checkout.orders o CROSS JOIN generate_series(0,9999) n WHERE o.id=$1
 RETURNING tenant_id,store_id,id,owner_id,creator_session_id,generation,created_at,expires_at)
 INSERT INTO inventory.reservations(tenant_id,store_id,id,state,expires_at,created_at,checkout_id,buyer_owner_id,buyer_session_id,generation)
 SELECT tenant_id,store_id,id,'RELEASED',expires_at,created_at,id,owner_id,creator_session_id,generation FROM inserted`, q.hold.OrderID)
	var target, owner string
	if err := q.f.owner.QueryRow(ctx, `SELECT id,owner_id FROM checkout.orders WHERE store_id=$1 AND snapshot#>>'{destination,phone}'='+886900007654'`, q.f.storeA1).Scan(&target, &owner); err != nil {
		t.Fatal(err)
	}
	mustExec(t, q.f.owner, `ANALYZE checkout.orders`)
	// Tracking is a separate real captured-and-manually-shipped order, not an
	// impossible shipment attached to a cancelled synthetic order.
	hash := pcRecord(t, q, pcFull(q))
	if err := pcApply(q.worker, q.result.AttemptID, hash); err != nil {
		t.Fatal(err)
	}
	mustExec(t, q.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'fulfillment:write') ON CONFLICT DO NOTHING`, q.f.tenantA, q.f.storeA1, q.f.principalA)
	carrier, tracking := "black_cat", "SYNTHETIC-TRACK-7654"
	if _, err := t04Scoped(ctx, q.f, q.f.tokens["a"], q.f.storeA1, "fulfillment:write", func(tx pgx.Tx, scope platform.Scope) (merchantorders.ShipmentVersion, error) {
		return merchantorders.RecordShipment(ctx, tx, scope, q.f.tokens["a"], t04Key("mo-v2-track"), q.hold.OrderID, merchantorders.ShipmentInput{Status: "SHIPPED", CarrierCode: &carrier, TrackingNumber: &tracking})
	}); err != nil {
		t.Fatal(err)
	}
	// One real paid, unshipped order makes the ready-to-ship SQL count non-zero.
	ready := q
	ready.cap = mustIssue(t, q.cqHarness.service, q.f.storeA1)
	ready.bcHarness.prepare(t, ready.cap, []storefront.Item{{SKUID: q.stock.skus[0].ID, Quantity: 2}}) // mock SKU NT$12.50 x2 meets provider whole-TWD rule
	readyOrder, e := ready.bcHarness.begin(t04Key("mo-v2-ready"))
	if e != nil {
		t.Fatal(e)
	}
	ready.hold = readyOrder
	ready.input.OrderID = readyOrder.OrderID
	ready.result, e = ready.start(t04Key("mo-v2-ready-start"))
	if e != nil {
		t.Fatal(e)
	}
	if e = pcApply(ready.worker, ready.result.AttemptID, pcRecord(t, ready, pcFull(ready))); e != nil {
		t.Fatal(e)
	}
	otherStore, otherID := moBeginInOtherStore(t, q.f, true)
	foreignStore, foreignID := moBeginInOtherStore(t, q.f, false)
	handler := httpapi.NewHandler(q.f.runtime)
	base := "/v1/admin/stores/" + q.f.storeA1 + "/orders?view=v2&limit=10"
	type page struct {
		Items  []map[string]any `json:"items"`
		Total  int              `json:"total"`
		Counts map[string]int   `json:"counts"`
		Cursor string           `json:"next_cursor"`
	}
	read := func(path string, want int) page {
		t.Helper()
		r := httptest.NewRequest("GET", path, nil)
		parsed, _ := url.Parse(path)
		params := parsed.Query()
		if words, ok := params["q"]; ok && len(words) == 1 && words[0] != "" {
			params.Del("q")
			body, _ := json.Marshal(map[string]string{"q": words[0]})
			parsed.Path += "/search"
			parsed.RawQuery = params.Encode()
			r = httptest.NewRequest("POST", parsed.String(), strings.NewReader(string(body)))
			r.Header.Set("Content-Type", "application/json")
		}
		r.Header.Set("Authorization", "Bearer "+q.f.tokens["a"])
		r.Host = "foreign.invalid"
		r.Header.Set("X-Forwarded-Host", "foreign.invalid")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("read status=%d want=%d body=%s", w.Code, want, w.Body.String())
		}
		if w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), "+88690000") || strings.Contains(w.Body.String(), "SQLSTATE") || strings.Contains(w.Body.String(), otherID) || strings.Contains(w.Body.String(), foreignID) {
			t.Fatal("read disclosed private or cross-store data")
		}
		var p page
		if want == 200 {
			if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
				t.Fatal(err)
			}
		}
		return p
	}
	before := map[string]int{}
	for _, table := range []string{"checkout.orders", "checkout.payment_attempts", "payments.facts", "inventory.ledger", "checkout.command_results", "checkout.events", "river.river_job"} {
		before[table] = countRows(t, q.f.owner, "SELECT count(*) FROM "+table)
	}
	all := read(base, 200)
	if all.Total != 10002 || all.Counts["all"] != 10002 || all.Counts["cancelled"] != 10000 || all.Counts["unpaid"] != 0 || all.Counts["shipped"] != 1 || all.Counts["ready_to_ship"] != 1 || len(all.Items) != 10 || all.Cursor == "" {
		t.Fatalf("SQL counts/page mismatch: %+v", all)
	}
	second := read(base+"&cursor="+all.Cursor, 200)
	if second.Total != all.Total || second.Counts["all"] != all.Counts["all"] {
		t.Fatal("cursor changed filtered totals")
	}
	read(base+"&cursor="+all.Cursor+"&q=7654", 422) // cursor cannot move across filters
	cancelled := read(base+"&bucket=cancelled", 200)
	if cancelled.Total != 10000 || cancelled.Counts["unpaid"] != 0 || cancelled.Counts["shipped"] != 1 {
		t.Fatal("bucket changed shared queue counts")
	}
	var expected int
	if err := q.f.owner.QueryRow(ctx, `SELECT count(*) FROM checkout.orders o WHERE tenant_id=$1 AND store_id=$2 AND commercial_state IN ('CONFIRMED','AWAITING_COLLECTION') AND fulfillment_state='MANUAL_UNASSIGNED' AND fulfillment.manual_shipment_eligible(tenant_id,store_id,id)`, q.f.tenantA, q.f.storeA1).Scan(&expected); err != nil {
		t.Fatal(err)
	}
	if all.Counts["ready_to_ship"] != expected {
		t.Fatal("ready-to-ship count disagrees with authoritative SQL")
	}
	for _, query := range []string{"7654", "SYNTHETIC-TRACK-7654"} {
		wantID := target
		if query == tracking {
			wantID = q.hold.OrderID
		}
		var samples []time.Duration
		for i := 0; i < 10; i++ {
			start := time.Now()
			got := read(strings.Replace(base, "limit=10", "limit=100", 1)+"&q="+url.QueryEscape(query), 200)
			elapsed := time.Since(start)
			samples = append(samples, elapsed)
			found := false
			for _, row := range got.Items {
				found = found || row["order_id"] == wantID
				if row["recipient_masked"] == nil || row["phone"] != nil || row["snapshot"] != nil {
					t.Fatal("unsafe list shape")
				}
			}
			if !found {
				t.Fatalf("query %q missed synthetic target", query)
			}
			if elapsed >= time.Second {
				t.Fatalf("10k search %s took %s (must <1s)", query, elapsed)
			}
		}
		sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
		t.Logf("10k %s: n=10 p50=%s max=%s", query, samples[4], samples[9])
	}
	for _, query := range []string{otherID, foreignID} {
		if read(base+"&q="+query, 200).Total != 0 {
			t.Fatal("foreign search matched")
		}
	}
	read("/v1/admin/stores/"+otherStore+"/orders?view=v2&q=7654", 404) // nonexistent and ungranted stores are indistinguishable
	read("/v1/admin/stores/"+foreignStore+"/orders?view=v2&q=7654", 404)
	for _, query := range []string{"&q=", "&q=one&q=two", "&tenant_id=" + q.f.tenantA, "&from=2026-02-30", "&from=2026-01-02&to=2026-01-01", "&q=%00", "&session_id=oops"} {
		read(base+query, 422)
	}
	// UTC midnight = Taipei 08:00 on Jan 1. The entire inclusive Taipei Jan 1 is used.
	if got := read(base+"&from=2026-01-01&to=2026-01-01", 200); got.Total != 10000 {
		t.Fatalf("Taipei date boundary count=%d", got.Total)
	}
	if got := read(base+"&from=2025-12-31&to=2025-12-31", 200); got.Total != 0 {
		t.Fatal("Taipei prior day leaked")
	}
	if got := read(base+"&payment_mode=cash_on_delivery", 200); got.Total != 0 {
		t.Fatal("payment filter mismatch")
	}
	for table, n := range before {
		if after := countRows(t, q.f.owner, "SELECT count(*) FROM "+table); after != n {
			t.Fatal(fmt.Sprintf("read mutated %s: %d -> %d", table, n, after))
		}
	}
}
