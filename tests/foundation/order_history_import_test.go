package foundation_test

// OH01-OH12 (contracts/migration-import-v1.md section 7; unit w5-03b-order-history-import; tier REAL_PG over httpapi.NewHandler, MOCK data:
// synthetic ids, names and amounts). Helper prefix `oh`. What it proves:
//   - OH01 a 3-order / 7-line file previews (rolled back) and commits as 3 archive rows with aggregated items, audit customers.orders_imported;
//   - OH02 an order of a customer that was never imported fails customer_not_imported and creates no customer; nothing_to_apply when all fail;
//   - OH03 I05: the archive changes no money, order, inventory or finance row, and no payment / inventory / finance / report function
//     body mentions it (static grep over pg_proc, view bodies and foreign keys);
//   - OH04 a full address column is never stored: the whole database holds no copy of it (only the city survives);
//   - OH05 the same order again is updated, never duplicated; another customer's order number is order_owner_conflict;
//   - OH06 erasure deletes the customer's archive, the stale file then fails erased for that customer and nothing else;
//   - OH07 customers:privacy to import, customers:read to read, another store's customer is 404, no login role holds a table grant;
//   - OH08 more than 5000 data lines is too_many_rows (the 0152 batch cap), over 2 MiB is 413, results carry no id;
//   - OH09 the keyset read pages newest first with a total; OH10 the merchant privacy export contains the archive;
//   - OH13 only the 22 Taiwan cities are archived (台/臺 normalised); any other city cell is dropped to NULL with a counted warning, never stored, and a table CHECK backs it.
//   - OH11 2000 archive rows per customer: the next order fails order_limit; OH12 the batch / results.csv hold row numbers and codes only.
// Disclosed owner-pool fixtures: the final purge of the archive rows this test created, and the 2000-row filler of OH11.

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
)

type ohEnv struct {
	*ciEnv
	finTok string // orders:read: the finance summary
}

func ohSetup(t *testing.T) *ohEnv {
	t.Helper()
	c := &ohEnv{ciEnv: ciSetup(t)}
	_, c.finTok = lcPrincipal(t, c.f, c.f.tenantA, []string{c.f.storeA1}, "store:read", "orders:read")
	t.Cleanup(func() {
		mustExec(t, c.f.owner, `DELETE FROM customers.historical_orders WHERE tenant_id=ANY($1)`, []string{c.f.tenantA, c.f.tenantB})
	})
	return c
}

func (c *ohEnv) opreview(token, body, query string) (int, map[string]any) {
	st, raw, _ := c.call("POST", c.imports(c.store)+"/orders/preview"+query, token, body)
	return st, sraJSON(raw)
}

func (c *ohEnv) ocommit(token, body string, expected int, extra string) (int, map[string]any) {
	st, raw, _ := c.call("POST", fmt.Sprintf("%s/orders/commit?expected_apply_rows=%d%s", c.imports(c.store), expected, extra), token, body)
	return st, sraJSON(raw)
}

func (c *ohEnv) archive(owner string, query string) (int, map[string]any) {
	st, raw, _ := c.call("GET", "/v1/admin/stores/"+c.store+"/customers/"+owner+"/historical-orders"+query, c.rtoken, "")
	return st, sraJSON(raw)
}

func ohFixture(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("testdata/shopline_orders_synthetic.csv")
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// moneyTables lists every base table of the money / order / inventory schemas that exist; their row counts must not move.
func (c *ohEnv) moneySnapshot() map[string]int {
	c.t.Helper()
	rows, err := c.f.owner.Query(context.Background(), `SELECT quote_ident(table_schema)||'.'||quote_ident(table_name) FROM information_schema.tables
	 WHERE table_type='BASE TABLE' AND table_schema IN ('checkout','payments','catalog','inventory','billing','reporting','storefront','ads')`)
	if err != nil {
		c.t.Fatal(err)
	}
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			c.t.Fatal(err)
		}
		names = append(names, n)
	}
	rows.Close()
	out := map[string]int{}
	for _, n := range names {
		out[n] = c.n(`SELECT count(*) FROM ` + n)
	}
	return out
}

func TestOrderHistoryImport(t *testing.T) {
	c := ohSetup(t)
	f := c.f
	custFile := ciFixture(t)
	file := ohFixture(t)
	const addrSecret = "SECRET-ADDR"
	var firstBatch, owner1, owner2 string

	t.Run("OH02 an order of a never-imported customer fails and creates no customer", func(t *testing.T) {
		if st, cm := c.commit(c.adminTok, custFile, 3, ""); st != 200 || cm["created"] != float64(3) {
			t.Fatalf("customer commit: %d %v", st, cm)
		}
		owner1, owner2 = c.ownerOf("SL-0001"), c.ownerOf("SL-0002")
		ownersBefore := c.n(`SELECT count(*) FROM buyer.owners WHERE tenant_id=$1`, f.tenantA)
		only := "訂單號碼,顧客編號,訂單日期,訂單狀態,訂單總金額\nOH-9001,NOBODY-1,2026-03-05,已完成,100\n"
		st, pv := c.opreview(c.adminTok, only, "")
		if st != 200 || pv["failed_rows"] != float64(1) || pv["apply_rows"] != float64(0) {
			t.Fatalf("preview: %d %v", st, pv)
		}
		if row := pv["rows"].([]any)[0].(map[string]any); row["code"] != "customer_not_imported" || row["row"] != float64(1) {
			t.Fatalf("row: %v", row)
		}
		if st, cm := c.ocommit(c.adminTok, only, 0, ""); st != 422 || cm["code"] != "nothing_to_apply" {
			t.Fatalf("all-failed commit: %d %v", st, cm)
		}
		if c.n(`SELECT count(*) FROM buyer.owners WHERE tenant_id=$1`, f.tenantA) != ownersBefore || c.n(`SELECT count(*) FROM customers.historical_orders WHERE tenant_id=$1`, f.tenantA) != 0 {
			t.Fatal("a failed order created an owner or an archive row")
		}
	})

	t.Run("OH01 preview rolls back; commit archives 3 orders from 7 lines", func(t *testing.T) {
		st, pv := c.opreview(c.adminTok, file, "")
		if st != 200 || pv["rows_total"] != float64(3) || pv["new_rows"] != float64(3) || pv["apply_rows"] != float64(3) || pv["failed_rows"] != float64(0) {
			t.Fatalf("preview: %d %v", st, pv)
		}
		m := pv["mapping"].(map[string]any)
		if _, ok := m["address"]; ok || m["order_id"] != "訂單號碼" || m["total"] != "訂單總金額" || m["city"] != "城市" {
			t.Fatalf("mapping: %v", m)
		}
		if c.n(`SELECT count(*) FROM customers.historical_orders WHERE tenant_id=$1`, f.tenantA) != 0 {
			t.Fatal("preview wrote rows")
		}
		st, cm := c.ocommit(c.adminTok, file, 3, "")
		firstBatch, _ = cm["batch_id"].(string)
		if st != 200 || firstBatch == "" || cm["created"] != float64(3) || cm["updated"] != float64(0) || cm["failed"] != float64(0) || cm["replayed"] != false {
			t.Fatalf("commit: %d %v", st, cm)
		}
		if c.n(`SELECT count(*) FROM customers.historical_orders WHERE tenant_id=$1 AND store_id=$2`, f.tenantA, c.store) != 3 {
			t.Fatal("want 3 archive rows")
		}
		var items, city, status, currency string
		var total int64
		if err := f.owner.QueryRow(context.Background(), `SELECT items_summary,city,status,total_minor,currency FROM customers.historical_orders WHERE external_order_id='OH-1002'`).
			Scan(&items, &city, &status, &total, &currency); err != nil || items != "貼紙×3、膠帶×1、卡片×2" || city != "臺北市" || status != "已出貨" || total != 30000 || currency != "TWD" {
			t.Fatalf("OH-1002: %q %q %q %d %q %v", items, city, status, total, currency, err)
		}
		if c.n(`SELECT count(*) FROM customers.historical_orders WHERE external_order_id='OH-1001' AND total_minor=128000 AND owner_id=$1 AND ordered_at='2026-03-05T06:30:00Z'`, owner1) != 1 {
			t.Fatal("OH-1001 amount / owner / Asia/Taipei time")
		}
		if c.n(`SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='customers.orders_imported'`, f.tenantA, c.store) != 1 ||
			c.n(`SELECT count(*) FROM migrationimport.batches WHERE tenant_id=$1 AND kind='orders' AND rows_total=3 AND applied=3`, f.tenantA) != 1 {
			t.Fatal("audit row / batch row")
		}
		// Replay of the same file and count: same batch, nothing twice.
		if st, again := c.ocommit(c.adminTok, file, 3, ""); st != 200 || again["replayed"] != true || again["batch_id"] != firstBatch {
			t.Fatalf("replay: %d %v", st, again)
		}
		if st, bad := c.ocommit(c.adminTok, file, 2, ""); st != 409 {
			t.Fatalf("same bytes another count: %d %v", st, bad)
		}
	})

	t.Run("OH03 I05 the archive touches no money, order or inventory row and no such function reads it", func(t *testing.T) {
		before := c.moneySnapshot()
		_, rawBefore, _ := c.call("GET", "/v1/admin/stores/"+c.store+"/finance/summary?from=2026-03-01&to=2026-04-30", c.finTok, "")
		changed := strings.Replace(file, ",已完成,500,", ",已完成,600,", 2)
		st, cm := c.ocommit(c.adminTok, changed, 3, "")
		if st != 200 || cm["updated"] != float64(3) {
			t.Fatalf("re-import for the I05 check: %d %v", st, cm)
		}
		after := c.moneySnapshot()
		for table, n := range before {
			if after[table] != n {
				t.Errorf("%s changed from %d to %d rows", table, n, after[table])
			}
		}
		_, rawAfter, _ := c.call("GET", "/v1/admin/stores/"+c.store+"/finance/summary?from=2026-03-01&to=2026-04-30", c.finTok, "")
		if string(rawBefore) != string(rawAfter) || len(rawBefore) == 0 {
			t.Errorf("finance summary moved: %s -> %s", rawBefore, rawAfter)
		}
		// Static: only the importer, the reader, the erasure hook and the export reader may name the table.
		rows, err := f.owner.Query(context.Background(), `SELECT n.nspname||'.'||p.proname FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
		 WHERE p.prosrc LIKE '%historical_orders%' ORDER BY 1`)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for rows.Next() {
			var n string
			_ = rows.Scan(&n)
			names = append(names, n)
		}
		rows.Close()
		if got := strings.Join(names, ","); got != "customers.erase_import_profile,customers.export_import_profile,customers.read_historical_orders,migrationimport.import_orders" {
			t.Fatalf("functions that name customers.historical_orders: %s", got)
		}
		if c.n(`SELECT count(*) FROM pg_views WHERE definition LIKE '%historical_orders%'`)+c.n(`SELECT count(*) FROM pg_matviews WHERE definition LIKE '%historical_orders%'`) != 0 {
			t.Error("a view names the archive")
		}
		if c.n(`SELECT count(*) FROM pg_constraint WHERE contype='f' AND confrelid='customers.historical_orders'::regclass`) != 0 ||
			c.n(`SELECT count(*) FROM pg_constraint WHERE contype='f' AND conrelid='customers.historical_orders'::regclass AND confrelid<>'buyer.owners'::regclass`) != 0 {
			t.Error("the archive must reference buyer.owners only and be referenced by nothing")
		}
		// Restore the first version for the later subtests.
		if st, cm := c.ocommit(c.adminTok, file, 3, ""); st != 200 || cm["replayed"] != true {
			t.Fatalf("original file replay: %d %v", st, cm)
		}
		mustExec(t, f.owner, `UPDATE customers.historical_orders SET total_minor=50000 WHERE external_order_id='OH-1003'`)
	})

	t.Run("OH04 the full address never reaches the database; only the city does", func(t *testing.T) {
		rows, err := f.owner.Query(context.Background(), `SELECT quote_ident(table_schema)||'.'||quote_ident(table_name) FROM information_schema.tables
		 WHERE table_type='BASE TABLE' AND table_schema NOT IN ('pg_catalog','information_schema')`)
		if err != nil {
			t.Fatal(err)
		}
		var tables []string
		for rows.Next() {
			var n string
			_ = rows.Scan(&n)
			tables = append(tables, n)
		}
		rows.Close()
		for _, table := range tables {
			if got := c.n(`SELECT count(*) FROM ` + table + ` t WHERE t::text LIKE '%` + addrSecret + `%' OR t::text LIKE '%中山路%' OR t::text LIKE '%文化路%'`); got != 0 {
				t.Errorf("%s holds the address text (%d rows)", table, got)
			}
		}
		if c.n(`SELECT count(*) FROM information_schema.columns WHERE table_schema='customers' AND table_name='historical_orders' AND column_name ~ '(addr|street|line1|line2|phone|email|card|bank|payment|account)'`) != 0 {
			t.Error("the archive table has an address / contact / payment column")
		}
	})

	t.Run("OH05 the same order again is updated; another customer's order number is refused", func(t *testing.T) {
		changed := strings.ReplaceAll(file, ",已完成,\"1,280\"", ",已退貨,\"1,280\"")
		changed = strings.ReplaceAll(changed, "OH-1003,SL-0002", "OH-1003,SL-0003") // OH-1003 belongs to SL-0002: conflict
		st, pv := c.opreview(c.adminTok, changed, "")
		if st != 200 || pv["update_rows"] != float64(2) || pv["failed_rows"] != float64(1) || pv["new_rows"] != float64(0) {
			t.Fatalf("preview: %d %v", st, pv)
		}
		if row := pv["rows"].([]any)[2].(map[string]any); row["code"] != "order_owner_conflict" || row["external_id"] != nil && row["external_id"] != "" {
			t.Fatalf("conflict row: %v", row)
		}
		st, cm := c.ocommit(c.adminTok, changed, 2, "")
		if st != 200 || cm["updated"] != float64(2) || cm["created"] != float64(0) || cm["failed"] != float64(1) {
			t.Fatalf("commit: %d %v", st, cm)
		}
		if c.n(`SELECT count(*) FROM customers.historical_orders WHERE tenant_id=$1 AND store_id=$2`, f.tenantA, c.store) != 3 ||
			c.n(`SELECT count(*) FROM customers.historical_orders WHERE external_order_id='OH-1001' AND status='已退貨'`) != 1 ||
			c.n(`SELECT count(*) FROM customers.historical_orders WHERE external_order_id='OH-1003' AND owner_id=$1`, owner2) != 1 {
			t.Fatal("update changed the row count, missed the status or moved the order to another customer")
		}
	})

	t.Run("OH09 the keyset read pages newest first with a total", func(t *testing.T) {
		st, p1 := c.archive(owner1, "?limit=1")
		items, _ := p1["items"].([]any)
		if st != 200 || p1["total"] != float64(2) || len(items) != 1 || p1["next_cursor"] == "" {
			t.Fatalf("page 1: %d %v", st, p1)
		}
		first := items[0].(map[string]any)
		if first["order_id"] != "OH-1002" || first["currency"] != "TWD" || first["city"] != "臺北市" || first["items_summary"] != "貼紙×3、膠帶×1、卡片×2" || first["total_minor"] != float64(30000) {
			t.Fatalf("newest first: %v", first)
		}
		if len(first) != 7 {
			t.Fatalf("item keys must be exactly the 7 display fields: %v", first)
		}
		st, p2 := c.archive(owner1, "?limit=1&after="+p1["next_cursor"].(string))
		items2, _ := p2["items"].([]any)
		if st != 200 || len(items2) != 1 || items2[0].(map[string]any)["order_id"] != "OH-1001" || p2["next_cursor"] != "" {
			t.Fatalf("page 2: %d %v", st, p2)
		}
		if st, _ := c.archive(owner1, "?after=not-a-cursor"); st != 422 {
			t.Fatalf("bad cursor: %d", st)
		}
		// A customer without an archive is an empty page; a plain id is 404.
		if st, p := c.archive(c.ownerOf("SL-0003"), ""); st != 200 || p["total"] != float64(0) || len(p["items"].([]any)) != 0 {
			t.Fatalf("empty archive: %d %v", st, p)
		}
		if st, _ := c.archive("00000000-0000-4000-8000-000000000000", ""); st != 404 {
			t.Fatalf("unknown customer: %d", st)
		}
	})

	t.Run("OH07 permissions, store scope and table grants", func(t *testing.T) {
		for name, token := range map[string]string{"customers:write only": c.authorATok, "read only": c.rtoken} {
			if st, _ := c.opreview(token, file, ""); st != 403 {
				t.Fatalf("%s preview: %d", name, st)
			}
			if st, _ := c.ocommit(token, file, 3, ""); st != 403 {
				t.Fatalf("%s commit: %d", name, st)
			}
		}
		if st, _, _ := c.call("GET", "/v1/admin/stores/"+c.store+"/customers/"+owner1+"/historical-orders", c.authorATok, ""); st != 200 {
			t.Fatalf("customers:read holder (write-only principal also has read): %d", st)
		}
		if st, _, _ := c.call("GET", "/v1/admin/stores/"+c.store+"/customers/"+owner1+"/historical-orders", "", ""); st != 401 {
			t.Fatalf("no token: %d", st)
		}
		// Tenant B: the customer id of tenant A on its own store is 404, on A's store 403/404.
		if st, _, _ := c.call("GET", "/v1/admin/stores/"+f.storeB+"/customers/"+owner1+"/historical-orders", c.otok, ""); st != 404 {
			t.Fatalf("cross-store customer: %d", st)
		}
		if st, _, _ := c.call("GET", "/v1/admin/stores/"+c.store+"/customers/"+owner1+"/historical-orders", c.otok, ""); st != 403 && st != 404 {
			t.Fatalf("other tenant on A1: %d", st)
		}
		// Tenant B imports orders for ids it has not imported: customer_not_imported even though tenant A has SL-0001.
		st, raw, _ := c.call("POST", c.imports(f.storeB)+"/orders/preview", c.otok, file)
		if pv := sraJSON(raw); st != 200 || pv["failed_rows"] != float64(3) || pv["apply_rows"] != float64(0) {
			t.Fatalf("tenant B preview: %d %s", st, raw)
		}
		ctx := context.Background()
		if _, err := f.runtime.Exec(ctx, `SELECT 1 FROM customers.historical_orders`); err == nil || !strings.Contains(err.Error(), "42501") {
			t.Fatalf("runtime login must be denied the archive table: %v", err)
		}
		if got := c.n(`SELECT count(*) FROM information_schema.table_privileges WHERE table_schema='customers' AND table_name='historical_orders'
		  AND grantee NOT IN ('commerce_privacy_writer') AND grantee<>(SELECT pg_get_userbyid(relowner) FROM pg_class WHERE oid='customers.historical_orders'::regclass)`); got != 0 {
			t.Fatalf("%d unexpected grants on the archive table", got)
		}
	})

	t.Run("OH10 the merchant privacy export contains the archive", func(t *testing.T) {
		st, raw, _ := c.call("POST", "/v1/admin/stores/"+c.store+"/customers/"+owner1+"/exports", c.adminTok, "", "Content-Type", "application/json", "Idempotency-Key", t04Key("oh-export"))
		if st != 200 {
			t.Fatalf("export: %d %s", st, raw)
		}
		prof, _ := sraJSON(raw)["import_profile"].(map[string]any)
		orders, _ := prof["historical_orders"].([]any)
		if len(orders) != 2 || orders[0].(map[string]any)["order_id"] != "OH-1002" || orders[1].(map[string]any)["order_id"] != "OH-1001" {
			t.Fatalf("export historical_orders: %v", prof["historical_orders"])
		}
		for label, q := range map[string]string{
			"receipts": `SELECT count(*) FROM ops.command_results r WHERE r.tenant_id=$1 AND r.response::text LIKE '%OH-1001%'`,
			"audit":    `SELECT count(*) FROM ops.audit_events a WHERE a.tenant_id=$1 AND row_to_json(a)::text LIKE '%OH-1001%'`,
			"batches":  `SELECT count(*) FROM migrationimport.batches b WHERE b.tenant_id=$1 AND b.results::text LIKE '%OH-100%'`,
		} {
			if got := c.n(q, f.tenantA); got != 0 {
				t.Errorf("%s names an order number: %d", label, got)
			}
		}
	})

	t.Run("OH12 the batch and results.csv carry row numbers and codes only", func(t *testing.T) {
		body := c.resultsCSV(firstBatch)
		if !strings.Contains(body, "row,external_id,outcome,code") || strings.Contains(body, "OH-") || strings.Contains(body, "SL-") || strings.Count(body, "created") != 3 {
			t.Fatalf("results.csv: %q", body)
		}
	})

	t.Run("OH06 erasure deletes the archive and the stale file fails erased", func(t *testing.T) {
		st, raw, _ := c.call("POST", "/v1/admin/stores/"+c.store+"/customers/"+owner1+"/erasure", c.adminTok, `{"confirm":"ERASE"}`,
			"Content-Type", "application/json", "Idempotency-Key", t04Key("oh-erase"))
		if st != 200 {
			t.Fatalf("erasure: %d %s", st, raw)
		}
		if c.n(`SELECT count(*) FROM customers.historical_orders WHERE owner_id=$1`, owner1) != 0 || c.n(`SELECT count(*) FROM customers.historical_orders WHERE owner_id=$1`, owner2) != 1 {
			t.Fatal("erasure must delete that customer's archive and no other")
		}
		if st, _ := c.archive(owner1, ""); st != 404 {
			t.Fatalf("erased customer's archive: %d", st)
		}
		stale := strings.Replace(file, ",已完成,500,", ",已完成,700,", 2)
		st, pv := c.opreview(c.adminTok, stale, "")
		if st != 200 || pv["erased_rows"] != float64(2) || pv["update_rows"] != float64(1) || pv["failed_rows"] != float64(2) || pv["apply_rows"] != float64(1) {
			t.Fatalf("preview after erasure: %d %v", st, pv)
		}
		if strings.Contains(fmt.Sprint(pv), "SL-0001") || strings.Contains(fmt.Sprint(pv["rows"]), "OH-1001") {
			t.Fatalf("an erased customer's ids are echoed: %v", pv)
		}
		st, cm := c.ocommit(c.adminTok, stale, 1, "")
		if st != 200 || cm["updated"] != float64(1) || cm["failed"] != float64(2) || cm["created"] != float64(0) {
			t.Fatalf("commit after erasure: %d %v", st, cm)
		}
		if c.n(`SELECT count(*) FROM customers.historical_orders WHERE owner_id=$1`, owner1) != 0 {
			t.Fatal("an erased customer's order was archived again")
		}
		if body := c.resultsCSV(cm["batch_id"].(string)); strings.Count(body, "erased") != 2 || strings.Contains(body, "OH-") || strings.Contains(body, "SL-0001") {
			t.Fatalf("results.csv after erasure: %q", body)
		}
		// restore replay (CD8): an archive row that reappears is deleted again.
		mustExec(t, f.owner, `INSERT INTO customers.historical_orders(tenant_id,store_id,owner_id,external_order_id,ordered_at,status,total_minor,currency,items_summary)
		 VALUES($1,$2,$3,'OH-RESTORED',now(),'x',100,'TWD','')`, f.tenantA, c.store, owner1)
		mustExec(t, f.owner, `SELECT customers.replay_erasures(ARRAY[$1::uuid])`, owner1)
		if c.n(`SELECT count(*) FROM customers.historical_orders WHERE owner_id=$1`, owner1) != 0 {
			t.Fatal("replay_erasures left an archive row")
		}
	})

	t.Run("OH13 only the 22 cities are archived as a city; anything else is dropped, counted and never stored", func(t *testing.T) {
		const samples = "臺北市中正區重慶南路一段一二二號|王小明|amy@mail.tw|台北市大安區忠孝東路四段"
		var b strings.Builder
		b.WriteString("訂單號碼,顧客編號,訂單日期,訂單狀態,訂單總金額,城市\n")
		for i, cell := range append(strings.Split(samples, "|"), "台中市") {
			fmt.Fprintf(&b, "OH-91%02d,SL-0002,2026-05-0%d,已完成,100,%s\n", i, i+1, cell)
		}
		st, pv := c.opreview(c.adminTok, b.String(), "")
		if st != 200 || pv["new_rows"] != float64(5) || pv["failed_rows"] != float64(0) || pv["city_dropped_rows"] != float64(4) {
			t.Fatalf("preview: %d %v", st, pv)
		}
		if row := pv["rows"].([]any)[0].(map[string]any); row["warning"] != "city_dropped" || row["outcome"] != "created" {
			t.Fatalf("warning row: %v", row)
		}
		st, cm := c.ocommit(c.adminTok, b.String(), 5, "")
		if st != 200 || cm["created"] != float64(5) {
			t.Fatalf("commit: %d %v", st, cm)
		}
		if c.n(`SELECT count(*) FROM customers.historical_orders WHERE external_order_id LIKE 'OH-91%' AND city IS NULL`) != 4 ||
			c.n(`SELECT count(*) FROM customers.historical_orders WHERE external_order_id='OH-9104' AND city='臺中市'`) != 1 {
			t.Fatal("a refused city must be stored as NULL and 台中市 as 臺中市")
		}
		for _, cell := range strings.Split(samples, "|") {
			rows, err := f.owner.Query(context.Background(), `SELECT quote_ident(table_schema)||'.'||quote_ident(table_name) FROM information_schema.tables
			 WHERE table_type='BASE TABLE' AND table_schema NOT IN ('pg_catalog','information_schema')`)
			if err != nil {
				t.Fatal(err)
			}
			var tables []string
			for rows.Next() {
				var n string
				_ = rows.Scan(&n)
				tables = append(tables, n)
			}
			rows.Close()
			for _, table := range tables {
				if got := c.n(`SELECT count(*) FROM `+table+` t WHERE t::text LIKE '%'||$1||'%'`, cell); got != 0 {
					t.Errorf("%s holds the refused city cell %q", table, cell)
				}
			}
		}
		// Backstop: the table itself refuses anything outside the allowlist, whoever writes it.
		for _, bad := range []string{"王小明", "台北市", "臺北市中正區", "amy@mail.tw"} {
			_, err := f.owner.Exec(context.Background(), `INSERT INTO customers.historical_orders(tenant_id,store_id,owner_id,external_order_id,ordered_at,status,total_minor,currency,items_summary,city)
			 VALUES($1,$2,$3,'OH-CHK',now(),'x',1,'TWD','',$4)`, f.tenantA, c.store, owner2, bad)
			if err == nil || !strings.Contains(err.Error(), "23514") {
				t.Errorf("city %q must violate the CHECK: %v", bad, err)
			}
		}
		mustExec(t, f.owner, `DELETE FROM customers.historical_orders WHERE external_order_id LIKE 'OH-91%'`)
	})

	t.Run("OH08 limits", func(t *testing.T) {
		var b strings.Builder
		b.WriteString("訂單號碼,顧客編號,訂單日期,訂單狀態,訂單總金額\n")
		for i := 0; i < 5001; i++ {
			fmt.Fprintf(&b, "OH-L%d,SL-0003,2026-03-05,已完成,100\n", i)
		}
		if st, pv := c.opreview(c.adminTok, b.String(), ""); st != 422 || pv["code"] != "too_many_rows" {
			t.Fatalf("5001 lines: %d %v", st, pv)
		}
		if st, _, _ := c.call("POST", c.imports(c.store)+"/orders/preview", c.adminTok, strings.Repeat("a", 2<<20+1)); st != http.StatusRequestEntityTooLarge {
			t.Fatalf("2 MiB + 1: %d", st)
		}
		if st, pv := c.opreview(c.adminTok, "訂單號碼,顧客編號\nO1,SL-0003\n", ""); st != 422 || pv["code"] != "required" {
			t.Fatalf("missing required columns: %d %v", st, pv)
		}
	})

	t.Run("OH11 2000 archive rows per customer, then order_limit and a bounded export", func(t *testing.T) {
		owner3 := c.ownerOf("SL-0003")
		mustExec(t, f.owner, `INSERT INTO customers.historical_orders(tenant_id,store_id,owner_id,external_order_id,ordered_at,status,total_minor,currency,items_summary)
		 SELECT $1,$2,$3,'FILL-'||g,'2026-01-01T00:00:00Z'::timestamptz+g*interval '1 second','x',100,'TWD',repeat('商',500) FROM generate_series(1,2000) g`, f.tenantA, c.store, owner3)
		one := "訂單號碼,顧客編號,訂單日期,訂單狀態,訂單總金額\nOH-LIMIT,SL-0003,2026-03-05,已完成,100\n"
		st, pv := c.opreview(c.adminTok, one, "")
		if st != 200 || pv["failed_rows"] != float64(1) || pv["rows"].([]any)[0].(map[string]any)["code"] != "order_limit" {
			t.Fatalf("2001st order: %d %v", st, pv)
		}
		if st, p := c.archive(owner3, "?limit=100"); st != 200 || p["total"] != float64(2000) || len(p["items"].([]any)) != 100 {
			t.Fatalf("archive of 2000: %d total=%v", st, p["total"])
		}
		// P2-4: 2000 rows of 500 characters would be ~3 MB; the export keeps the newest 100 and states the total.
		st, raw, _ := c.call("POST", "/v1/admin/stores/"+c.store+"/customers/"+owner3+"/exports", c.adminTok, "", "Content-Type", "application/json", "Idempotency-Key", t04Key("oh-export-cap"))
		if st != 200 || len(raw) > 1<<20 {
			t.Fatalf("export of a 2000-row archive: %d (%d bytes)", st, len(raw))
		}
		prof, _ := sraJSON(raw)["import_profile"].(map[string]any)
		kept, _ := prof["historical_orders"].([]any)
		if len(kept) != 100 || prof["historical_orders_total"] != float64(2000) || kept[0].(map[string]any)["order_id"] != "FILL-2000" {
			t.Fatalf("export archive section: kept=%d total=%v", len(kept), prof["historical_orders_total"])
		}
	})
}
