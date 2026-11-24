package foundation_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/catalog"
	"livecommerce/internal/pagination"
)

func TestCatalogCoreCC12cAdminStockFilter(t *testing.T) {
	e := ccNew(t)
	f := e.h.f
	twoWares, sku := e.simple("both warehouses", 100, 4, nil)
	e.stock(sku.ID, e.wh2, 5)
	e.activate(&twoWares)
	draft, _ := e.simple("draft with stock", 100, 2, nil)
	archived, _ := e.simple("archived with stock", 100, 3, nil)
	e.setStatus(&archived, "archived")
	zero, zeroSKU := e.simple("zero balance", 100, 4, nil)
	e.stock(zeroSKU.ID, e.wh1, -4)
	e.activate(&zero)
	missing, missingSKU := e.simple("no balance", 100, 0, nil)
	e.activate(&missing)

	mixed := e.product("mixed tracking", nil)
	e.a.ok("PATCH", "/products/"+mixed.ID, e.key("options"), map[string]any{"expected_version": mixed.Version, "options": []map[string]any{{"name": "Size", "values": []string{"S", "M"}}}}, &mixed)
	tracked := e.sku(mixed.ID, "MIX-S", 100, []string{"S"})
	e.stock(tracked.ID, e.wh1, 6)
	addUntracked := func(p ccProduct, values []string) {
		e.a.ok("PUT", "/products/"+p.ID+"/document", e.key("untracked"), map[string]any{
			"expected_version": p.Version, "skus": []map[string]any{{
				"price_minor": 100, "option_values": values, "stock": map[string]any{"mode": "untracked", "max_per_order": 3},
			}},
		}, nil)
	}
	addUntracked(mixed, []string{"M"})
	untracked := e.product("all untracked", nil)
	addUntracked(untracked, []string{})
	empty, archivedSKU := e.simple("no active variants", 100, 8, nil)
	e.a.ok("POST", "/skus/"+archivedSKU.ID+"/archive", e.key("archive"), map[string]any{"expected_version": archivedSKU.Version}, nil)
	mustExec(t, f.owner, `UPDATE inventory.warehouses SET active=false WHERE tenant_id=$1 AND store_id=$2 AND id=$3`, f.tenantA, f.storeA1, e.wh2)

	const route = "/catalog-products?"
	q := "q=" + e.tag
	read := func(query string) catalog.ProductListResult {
		t.Helper()
		var page catalog.ProductListResult
		e.a.ok("GET", route+query, "", nil, &page)
		return page
	}
	assert := func(page catalog.ProductListResult, counts catalog.StatusCounts, want ...string) {
		t.Helper()
		if page.Items == nil {
			t.Fatal("items must be an array")
		}
		got := make([]string, 0, len(page.Items))
		for _, row := range page.Items {
			got = append(got, row.ID)
		}
		if counts != page.StatusCounts || page.Total != counts.Draft+counts.Active+counts.Archived || !slices.Equal(got, want) {
			t.Fatalf("ids=%v want=%v total=%d counts=%+v want=%+v", got, want, page.Total, page.StatusCounts, counts)
		}
	}
	available := q + "&stock=available"
	initial := read(available)
	assert(initial, catalog.StatusCounts{Draft: 1, Active: 1, Archived: 1}, archived.ID, draft.ID, twoWares.ID)
	if row := initial.Items[2]; row.Available != 9 || !row.InventoryTracked || row.SKUCount != 1 {
		t.Fatalf("same SKU across two warehouses, including inactive: %+v", row)
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM inventory.balances WHERE (warehouse_id,sku_id)=($1,$2) AND on_hand=5`, e.wh2, sku.ID); n != 1 {
		t.Fatal("inactive warehouse did not retain balance")
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM inventory.balances WHERE (warehouse_id,sku_id)=($1,$2) AND on_hand=0`, e.wh1, zeroSKU.ID); n != 1 {
		t.Fatal("explicit zero balance missing")
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM inventory.balances WHERE (tenant_id,store_id,sku_id)=($1,$2,$3)`, f.tenantA, f.storeA1, missingSKU.ID); n != 0 {
		t.Fatal("missing balance fixture has a row")
	}
	assert(read("q="+url.QueryEscape(strings.ToLower(sku.Code))+"&stock=available"),
		catalog.StatusCounts{Active: 1}, twoWares.ID)
	assert(read(q+"&stock=sold_out"), catalog.StatusCounts{Active: 2}, missing.ID, zero.ID)
	mixedPage := read(q + "&stock=untracked")
	assert(mixedPage, catalog.StatusCounts{Draft: 3}, empty.ID, untracked.ID, mixed.ID)
	if mixedPage.Items[0].Available != 0 || mixedPage.Items[0].SKUCount != 0 || mixedPage.Items[2].Available != 6 || mixedPage.Items[2].InventoryTracked {
		t.Fatalf("archived SKU and mixed tracking: %+v", mixedPage.Items)
	}
	assert(read(q+"&stock=untracked&status=active"), catalog.StatusCounts{Draft: 3})
	assert(read("q=nonmatching-"+e.tag+"&stock=available"), catalog.StatusCounts{})

	newest := []string{twoWares.ID}
	for i := 0; i < 25; i++ {
		if i == 8 || i == 18 {
			noStock, _ := e.simple(fmt.Sprintf("interleaved sold out %d", i), 100, 0, nil)
			e.activate(&noStock)
			e.product(fmt.Sprintf("interleaved untracked %d", i), nil)
		}
		p, _ := e.simple(fmt.Sprintf("in stock %02d", i), 100, 1, nil)
		e.activate(&p)
		newest = append(newest, p.ID)
		if i == 23 {
			same := slices.Clone(newest)
			slices.Reverse(same)
			page := read(available + "&status=active&limit=25")
			assert(page, catalog.StatusCounts{Draft: 1, Active: 25, Archived: 1}, same...)
			if page.NextCursor != "" {
				t.Fatal("25 eligible rows yielded a continuation")
			}
		}
	}
	slices.Reverse(newest)
	query := available + "&status=active&limit=25"
	first := read(query)
	counts := catalog.StatusCounts{Draft: 1, Active: 26, Archived: 1}
	assert(first, counts, newest[:25]...)
	if first.NextCursor == "" {
		t.Fatal("26 eligible rows require a continuation")
	}
	tail := "&cursor=" + url.QueryEscape(first.NextCursor)
	last := read(query + tail)
	assert(last, counts, newest[25])
	if last.NextCursor != "" {
		t.Fatal("last page yielded a continuation")
	}
	e.setStatus(&twoWares, "draft")
	pastEnd := read(query + tail)
	assert(pastEnd, catalog.StatusCounts{Draft: 2, Active: 25, Archived: 1})
	if pastEnd.NextCursor != "" {
		t.Fatal("empty continuation yielded a cursor")
	}
	e.activate(&twoWares)

	// Construct the prior API's cursor with its exact digest and the actual row's key.
	var stamp string
	if err := f.owner.QueryRow(context.Background(), `SELECT to_char(created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"') FROM catalog.products WHERE id=$1`, newest[0]).Scan(&stamp); err != nil {
		t.Fatal(err)
	}
	oldDigest := sha256.Sum256([]byte("active\x00" + e.tag))
	binding := pagination.Binding{TenantID: f.tenantA, StoreID: f.storeA1, Collection: "catalog-products", Filter: hex.EncodeToString(oldDigest[:])}
	legacy, err := pagination.Encode(binding, []string{stamp, newest[0]})
	if err != nil {
		t.Fatal(err)
	}
	oldTail := "&cursor=" + url.QueryEscape(legacy)
	unfiltered := q + "&status=active&limit=2"
	unfilteredCounts := catalog.StatusCounts{Draft: 6, Active: 30, Archived: 1}
	if oldHead := read(unfiltered); len(oldHead.Items) != 2 || oldHead.Items[0].ID != newest[0] || oldHead.Total != 37 || oldHead.StatusCounts != unfilteredCounts {
		t.Fatalf("unfiltered totals or order changed: %+v", oldHead)
	}
	if oldPage := read(unfiltered + oldTail); len(oldPage.Items) != 2 || oldPage.Items[0].ID != newest[1] || oldPage.Total != 37 || oldPage.StatusCounts != unfilteredCounts {
		t.Fatalf("unfiltered legacy cursor could not continue: %+v", oldPage)
	}
	for _, invalid := range []string{
		"stock", "stock=", "stock=Available", "stock=bogus", "stock=available&stock=available",
		"stock=available&%73tock=sold_out", "stock=available&q=%ZZ", "stock=available&q=x&q=y",
		"stock=available&q=" + strings.Repeat("x", 121), "stock=available&q=a%00b", "stock=available&status=hidden",
		"stock=available&limit=0", "stock=available&limit=101", "stock=available&cursor=bogus",
		"stock=available&cursor=" + strings.Repeat("x", 1025), "stock=available&unknown=1",
		query + oldTail, unfiltered + tail, q + "&stock=sold_out&status=active" + tail,
		q + "x&stock=available&status=active" + tail, available + "&status=draft" + tail,
	} {
		code, body := e.a.call("GET", route+invalid, "", nil)
		if code != 422 || !strings.Contains(string(body), `"code":"invalid_request"`) {
			t.Fatalf("query %.100q: want invalid_request 422, got %d %s", invalid, code, body)
		}
	}
	for _, tt := range []struct {
		permissions []string
		status      int
	}{
		{[]string{"catalog:read", "inventory:read"}, 200},
		{[]string{"inventory:read"}, 403},
		{[]string{"catalog:read"}, 403},
	} {
		reader := e.a
		reader.token = mtMember(t, psHarness{f: f}, tt.permissions...)
		reader.refuse(tt.status, "GET", route+query+tail, "", nil)
	}
	for _, foreign := range []struct{ store, token string }{{f.storeA2, f.tokens["a2"]}, {f.storeB, f.tokens["b"]}} {
		reader := e.a
		reader.store, reader.token = foreign.store, foreign.token
		var page catalog.ProductListResult
		reader.ok("GET", route+query, "", nil, &page)
		assert(page, catalog.StatusCounts{})
		reader.refuse(422, "GET", route+query+tail, "", nil)
	}
	t.Log("stock 200, all three buckets, 25/26 keyset, zero/empty/last page tallies, old cursor, invalid_request 422, missing grant 403 and two-store isolation")
}

func TestCatalogCoreCC12dAdminStockSnapshot(t *testing.T) {
	e := ccNew(t)
	p, sku := e.simple("snapshot", 100, 1, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	f := e.h.f
	// Pause the eligibility scan on the old balance, while the adjustment's readback can see zero.
	mustExec(t, f.owner, fmt.Sprintf(`CREATE POLICY r07_stock_read_barrier ON inventory.balances AS RESTRICTIVE FOR SELECT TO commerce_runtime
		USING (CASE WHEN sku_id = '%s'::uuid AND on_hand=1 THEN (SELECT true FROM pg_advisory_xact_lock(71203603)) ELSE true END)`, sku.ID))
	t.Cleanup(func() { mustExec(t, f.owner, `DROP POLICY r07_stock_read_barrier ON inventory.balances`) })
	holder, err := f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(context.Background())
	if _, err := holder.Exec(ctx, `SELECT pg_advisory_xact_lock(71203603)`); err != nil {
		t.Fatal(err)
	}
	request := "/catalog-products?q=" + e.tag + "&stock=available"
	type captured struct {
		status int
		body   []byte
	}
	read := make(chan captured, 1)
	go func() {
		status, body, _ := e.a.raw("GET", request, "", nil, "", func(r *http.Request) { *r = *r.WithContext(ctx) })
		read <- captured{status, body}
	}()
	pid := int32(holder.Conn().PgConn().PID())
	for {
		select {
		case reply := <-read:
			t.Fatalf("read returned before lock observation: %d %s", reply.status, reply.body)
		default:
		}
		var blocked bool
		if err := f.owner.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, pid).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
	}
	e.stock(sku.ID, e.wh1, -1)
	if err := holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case reply := <-read:
		var page catalog.ProductListResult
		if err := json.Unmarshal(reply.body, &page); err != nil || reply.status != 200 || page.Total != 1 ||
			page.StatusCounts != (catalog.StatusCounts{Draft: 1}) || len(page.Items) != 1 || page.Items[0].ID != p.ID || page.Items[0].Available != 1 {
			t.Fatalf("mixed row and count snapshots: %d %s", reply.status, reply.body)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	var now catalog.ProductListResult
	e.a.ok("GET", request, "", nil, &now)
	if now.Items == nil || len(now.Items) != 0 || now.Total != 0 || now.StatusCounts != (catalog.StatusCounts{}) || now.NextCursor != "" {
		t.Fatalf("following request must see the committed bucket change: %+v", now)
	}
	e.a.ok("GET", strings.Replace(request, "available", "sold_out", 1), "", nil, &now)
	if now.Total != 1 || now.StatusCounts.Draft != 1 || len(now.Items) != 1 || now.Items[0].Available != 0 {
		t.Fatalf("committed sold_out page: %+v", now)
	}
	t.Log("verified read blocked in PG after snapshot acquisition; concurrent stock crossed available -> sold_out; old page and counts coherent, next read sees new bucket")
}
