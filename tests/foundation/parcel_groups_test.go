package foundation_test

// Purpose: W3-07B parcel groups over REAL_PG and the real HTTP handler (unit w3-07b-parcel-merge): PG01 suggestion + create,
//   PG02 refusals (address / owner / COD / CVS), PG03 one order one group, PG04 atomic group shipment (same tracking, per-order
//   audit and mail, payments untouched = I05), PG05 drift rolls the whole group back, PG06 single/bulk shipment refused while
//   grouped, PG07 dissolve then ship alone, PG08 concurrent creators, plus pick-list/export adjacency.
// Depends on: tcvEnv (stripe mode: card home orders paid through the rfx capture path, COD and CVS orders), the 0146 definers,
//   merchant routes /orders/merge-suggestions, /parcel-groups, /orders/{id}/shipment, /shipments/tracking-import/preview.
// Used by: go test ./tests/foundation (-run '^TestParcelGroup'); evidence class REAL_PG with MOCK Stripe/ECPay fakes.

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
)

// pgHome places one card home order of buyer b at the shared home address (hcodRehome pins the same recipient and address for
// every buyer) and pays it through the real capture path.
func (e *tcvEnv) pgHome(b *tcvBuyer) string {
	e.t.Helper()
	e.hcodRehome(b)
	res, err := e.svc.Begin(context.Background(), b.cap.Token, e.store(), t04Key("pg-begin"), b.h.input)
	if err != nil {
		e.t.Fatalf("place card home order: %v", err)
	}
	return e.payHold(res, b).order
}

func (e *tcvEnv) pgPath(suffix string) string { return "/v1/admin/stores/" + e.store() + suffix }

func (e *tcvEnv) pgCreate(key string, ids ...string) (int, map[string]any, []byte) {
	body := `{"order_ids":["` + strings.Join(ids, `","`) + `"]}`
	return e.mcall(e.token(), "POST", e.pgPath("/parcel-groups"), key, body)
}

func (e *tcvEnv) pgShip(group, key, tracking string) (int, map[string]any, []byte) {
	return e.mcall(e.token(), "PUT", e.pgPath("/parcel-groups/"+group+"/shipment"), key, mfxShip(0, "black_cat", tracking))
}

func (e *tcvEnv) pgSuggestions() []map[string]any {
	e.t.Helper()
	st, out, raw := e.mcall(e.token(), "GET", e.pgPath("/orders/merge-suggestions"), "", "")
	if st != 200 {
		e.t.Fatalf("merge-suggestions answered %d: %s", st, raw)
	}
	items, _ := out["items"].([]any)
	var res []map[string]any
	for _, it := range items {
		res = append(res, it.(map[string]any))
	}
	return res
}

// pgIDs returns the order ids of a response body's order_ids sorted.
func pgIDs(v any) []string {
	var ids []string
	for _, x := range v.([]any) {
		ids = append(ids, x.(string))
	}
	sort.Strings(ids)
	return ids
}

func pgSorted(ids ...string) []string {
	out := append([]string(nil), ids...)
	sort.Strings(out)
	return out
}

func pgExpectCode(t *testing.T, what string, st int, out map[string]any, wantStatus int, wantCode string) {
	t.Helper()
	if st != wantStatus || out["code"] != wantCode {
		t.Fatalf("%s: got %d %v, want %d %s", what, st, out, wantStatus, wantCode)
	}
}

func (e *tcvEnv) pgState(order string) string {
	var s string
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT fulfillment_state FROM checkout.orders WHERE id=$1`, order).Scan(&s); err != nil {
		e.t.Fatal(err)
	}
	return s
}

func (e *tcvEnv) pgPaymentFingerprint() [3]int {
	return [3]int{e.count(`SELECT count(*) FROM payments.facts WHERE store_id=$1`, e.store()),
		e.count(`SELECT count(*) FROM checkout.payment_attempts WHERE store_id=$1`, e.store()),
		e.count(`SELECT coalesce(sum(total_minor),0)::int FROM checkout.orders WHERE store_id=$1`, e.store())}
}

func TestParcelGroup(t *testing.T) {
	e := tcvNew(t, tcvOpts{stripe: true})
	e.r.startWorker(t)
	e.grantCreator("orders:read", "orders:export", "fulfillment:write")
	f := e.p.f
	ctx := context.Background()

	b, other := e.newBuyer(), e.newBuyer()
	o1 := e.pgHome(b)
	x := e.pgHome(other) // another buyer, same address: never merged with b's orders
	o2 := e.pgHome(b)
	o3 := e.pgHome(b)

	var g1 string
	t.Run("PG01 suggestion for the same buyer and address, then create", func(t *testing.T) {
		items := e.pgSuggestions()
		if len(items) != 1 || strings.Join(pgIDs(items[0]["order_ids"]), ",") != strings.Join(pgSorted(o1, o2, o3), ",") {
			t.Fatalf("suggestions %v, want one set {o1,o2,o3} (the other buyer's order %s excluded)", items, x)
		}
		st, out, raw := e.pgCreate("pg-create-0001", o3, o1, o2)
		if st != 201 || out["state"] != "OPEN" || out["version"] != float64(1) || len(pgIDs(out["order_ids"])) != 3 {
			t.Fatalf("create: %d %s", st, raw)
		}
		g1 = out["id"].(string)
		if n := e.count(`SELECT count(*) FROM fulfillment.parcel_group_orders WHERE group_id=$1`, g1); n != 3 {
			t.Fatalf("membership rows %d", n)
		}
		if n := e.count(`SELECT count(*) FROM fulfillment.parcel_groups WHERE id=$1 AND state='OPEN' AND octet_length(destination_hash)=32`, g1); n != 1 {
			t.Fatal("group row missing or malformed")
		}
		if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='fulfillment.parcel_group_created'`, e.store()); n != 1 {
			t.Fatalf("created audit rows %d", n)
		}
		// Replay: same key + same set -> same group; same key + different set -> conflict; nothing duplicated.
		st, out, _ = e.pgCreate("pg-create-0001", o1, o2, o3)
		if st != 201 || out["id"] != g1 {
			t.Fatalf("replay: %d %v", st, out)
		}
		st, out, _ = e.pgCreate("pg-create-0001", o1, o2)
		pgExpectCode(t, "same key, other set", st, out, 409, "conflict")
		if n := e.count(`SELECT count(*) FROM fulfillment.parcel_groups WHERE store_id=$1`, e.store()); n != 1 {
			t.Fatalf("groups after replay %d", n)
		}
		if items := e.pgSuggestions(); len(items) != 0 {
			t.Fatalf("grouped orders are still suggested: %v", items)
		}
	})

	o4, o5 := e.pgHome(b), e.pgHome(b)
	t.Run("PG02 refusals: address, owner, COD, CVS, size, unknown", func(t *testing.T) {
		// Disclosed owner fixture: o4 moves to another street (the hash is computed from the snapshot at read time).
		mustExec(t, f.owner, `UPDATE checkout.orders SET snapshot=jsonb_set(snapshot,'{destination,home_address,line1}','"Another street 9"') WHERE id=$1`, o4)
		st, out, _ := e.pgCreate("pg-create-0002", o4, o5)
		pgExpectCode(t, "different address", st, out, 409, "destination_mismatch")
		st, out, _ = e.pgCreate("pg-create-0003", o5, x)
		pgExpectCode(t, "different owner", st, out, 409, "owner_mismatch")

		e.hcodSettings(0, true, 20000, 0, "black_cat")
		e.hcodRehome(b)
		cod, err := e.hcodPlace(b)
		if err != nil {
			t.Fatalf("COD order: %v", err)
		}
		st, out, _ = e.pgCreate("pg-create-0004", o5, cod.OrderID)
		pgExpectCode(t, "COD order", st, out, 409, "cod_not_mergeable")

		e.connect("C2C")
		e.cvsSettings(tcvAllChains, true, "20000", 500)
		code, _, _ := e.service("cvs_711", "API", 0)
		cvs, _ := e.cvsOrder(tcvOrderSpec{kind: "cvs_711", code: code})
		st, out, _ = e.pgCreate("pg-create-0005", o5, cvs)
		pgExpectCode(t, "CVS order", st, out, 409, "cvs_not_mergeable")

		st, out, _ = e.pgCreate("pg-create-0006", o5)
		pgExpectCode(t, "one order", st, out, 422, "invalid_request")
		st, out, _ = e.pgCreate("pg-create-0007", o5, "99999999-9999-4999-8999-999999999999")
		pgExpectCode(t, "unknown order", st, out, 404, "not_found")
		if items := e.pgSuggestions(); len(items) != 0 {
			t.Fatalf("COD, CVS, other-address or other-owner orders were suggested: %v", items)
		}
		if n := e.count(`SELECT count(*) FROM fulfillment.parcel_groups WHERE store_id=$1`, e.store()); n != 1 {
			t.Fatalf("a refused create left a group (%d)", n)
		}
	})

	t.Run("PG03 one order, at most one group", func(t *testing.T) {
		st, out, _ := e.pgCreate("pg-create-0008", o1, o5)
		pgExpectCode(t, "order already grouped", st, out, 409, "already_in_group")
	})

	t.Run("pick list and export put a group side by side with parcel_group_id", func(t *testing.T) {
		// Creation order is o1, x, o2, o3: the reader would list x between o1 and o2; the group must stay together.
		st, _, raw := e.mcall(e.token(), "POST", e.pgPath("/orders/pick-list"), "", plPickBody([]string{o1, x, o2, o3}))
		if st != 200 {
			t.Fatalf("pick list: %d %s", st, raw)
		}
		var list plParcelBody
		if err := json.Unmarshal(raw, &list); err != nil {
			t.Fatal(err)
		}
		var seq []string
		for _, o := range list.Orders {
			seq = append(seq, o.OrderID)
			if (o.OrderID == x) != (o.ParcelGroupID == "") || (o.OrderID != x && o.ParcelGroupID != g1) {
				t.Fatalf("order %s carries parcel_group_id %q (group %s)", o.OrderID, o.ParcelGroupID, g1)
			}
		}
		if len(seq) != 4 || seq[3] != x && seq[0] != x {
			t.Fatalf("group members are not adjacent: %v (x=%s)", seq, x)
		}
		st, _, csvRaw := e.mcall(e.token(), "POST", e.pgPath("/orders/export?template=generic"), "", plPickBody([]string{o1, x, o2, o3}))
		if st != 200 {
			t.Fatalf("export: %d %s", st, csvRaw)
		}
		records, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(csvRaw), "\xEF\xBB\xBF"))).ReadAll()
		if err != nil || len(records) != 5 {
			t.Fatalf("export rows: %v %d", err, len(records))
		}
		col := -1
		for i, h := range records[0] {
			if h == "parcel_group_id" {
				col = i
			}
		}
		oid := -1
		for i, h := range records[0] {
			if h == "order_id" {
				oid = i
			}
		}
		if col < 0 {
			t.Fatalf("export header lacks parcel_group_id: %v", records[0])
		}
		var run []string
		for _, r := range records[1:] {
			run = append(run, r[col])
			if (r[oid] == x) != (r[col] == "") {
				t.Fatalf("export row %v", r)
			}
		}
		if run[0] != run[1] && run[1] != run[2] || run[0] == "" && run[1] == "" {
			t.Fatalf("export group column not adjacent: %v", run)
		}
	})

	// A control order of the other buyer shipped through the single route: the shipped mail of a group member must match it.
	t.Run("PG04 group shipment: one tracking number, per-order audit and mail, no money touched", func(t *testing.T) {
		st, out, raw := e.mcall(e.token(), "PUT", e.pgPath("/orders/"+x+"/shipment"), "pg-single-0001", mfxShip(0, "black_cat", "CTRL0001"))
		if st != 200 {
			t.Fatalf("control shipment: %d %s", st, raw)
		}
		controlMail := e.count(`SELECT count(*) FROM notify.outbox WHERE order_id=$1 AND kind='shipped'`, x)
		t.Logf("control order shipped-mail rows: %d", controlMail)
		auditBefore := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='fulfillment.shipment_recorded'`, e.store())
		money := e.pgPaymentFingerprint()

		st, out, raw = e.pgShip(g1, "pg-ship-0001", "TRK12345678")
		if st != 200 || out["state"] != "SHIPPED" || out["version"] != float64(2) {
			t.Fatalf("group shipment: %d %s", st, raw)
		}
		ships, _ := out["shipments"].([]any)
		if len(ships) != 3 {
			t.Fatalf("shipments %v", ships)
		}
		for _, o := range []string{o1, o2, o3} {
			if e.pgState(o) != "MERCHANT_SHIPPED" {
				t.Fatalf("order %s is %s", o, e.pgState(o))
			}
			if n := e.count(`SELECT count(*) FROM fulfillment.manual_shipment_versions v WHERE v.order_id=$1 AND v.tracking_number='TRK12345678' AND v.status='SHIPPED'`, o); n != 1 {
				t.Fatalf("order %s shipment rows %d", o, n)
			}
			if n := e.count(`SELECT count(*) FROM notify.outbox WHERE order_id=$1 AND kind='shipped'`, o); n != controlMail {
				t.Fatalf("order %s shipped mails %d, control order has %d", o, n, controlMail)
			}
		}
		if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='fulfillment.shipment_recorded'`, e.store()); n != auditBefore+3 {
			t.Fatalf("shipment audits %d, want %d", n, auditBefore+3)
		}
		if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='fulfillment.parcel_group_shipped'`, e.store()); n != 1 {
			t.Fatalf("group shipped audits %d", n)
		}
		if got := e.pgPaymentFingerprint(); got != money {
			t.Fatalf("I05: payments/orders money changed %v -> %v", money, got)
		}
		// Same request again replays every member and changes nothing; a new key on a SHIPPED group is a version conflict.
		st, out, _ = e.pgShip(g1, "pg-ship-0001", "TRK12345678")
		if st != 200 || out["state"] != "SHIPPED" || e.count(`SELECT count(*) FROM fulfillment.manual_shipment_versions WHERE order_id=ANY($1::uuid[])`, []string{o1, o2, o3}) != 3 {
			t.Fatalf("replay: %d %v", st, out)
		}
		st, out, _ = e.pgShip(g1, "pg-ship-0002", "TRK12345678")
		pgExpectCode(t, "re-ship with a new key", st, out, 409, "version_changed")
	})

	o6, o7, o8 := e.pgHome(b), e.pgHome(b), e.pgHome(b)
	var g2 string
	t.Run("PG05 a drifted member rolls the whole group back", func(t *testing.T) {
		st, out, raw := e.pgCreate("pg-create-0009", o6, o7, o8)
		if st != 201 {
			t.Fatalf("create g2: %d %s", st, raw)
		}
		g2 = out["id"].(string)
		ids := pgSorted(o6, o7, o8)
		victim := ids[2] // the last member in id order: the earlier two are already written when it fails
		// Disclosed fixture: a shipment recorded behind the group's back (the definer called directly), i.e. version drift.
		mustExec(t, f.owner, `SELECT fulfillment.record_manual_shipment(sha256(convert_to($1,'UTF8')),$2::uuid,$3::uuid,'drift-key-0001',sha256('drift'::bytea),0,'SHIPPED','black_cat',NULL,'DRIFT0001',NULL,NULL,NULL)`,
			e.token(), e.store(), victim)
		before := e.count(`SELECT count(*) FROM fulfillment.manual_shipment_versions WHERE order_id=ANY($1::uuid[])`, ids)
		st, out, raw = e.pgShip(g2, "pg-ship-0003", "TRK99999999")
		pgExpectCode(t, "drifted member", st, out, 409, "version_changed")
		if n := e.count(`SELECT count(*) FROM fulfillment.manual_shipment_versions WHERE order_id=ANY($1::uuid[])`, ids); n != before {
			t.Fatalf("shipment rows %d -> %d: the failed group shipment was not rolled back", before, n)
		}
		for _, o := range ids[:2] {
			if e.pgState(o) != "MANUAL_UNASSIGNED" {
				t.Fatalf("order %s is %s after the rollback", o, e.pgState(o))
			}
		}
		if n := e.count(`SELECT count(*) FROM fulfillment.parcel_groups WHERE id=$1 AND state='OPEN' AND version=1`, g2); n != 1 {
			t.Fatal("group g2 left OPEN/version 1")
		}
	})

	t.Run("PG06 a grouped order cannot ship alone or through the bulk import", func(t *testing.T) {
		member := pgSorted(o6, o7, o8)[0]
		st, out, _ := e.mcall(e.token(), "PUT", e.pgPath("/orders/"+member+"/shipment"), "pg-single-0002", mfxShip(0, "black_cat", "SOLO0001"))
		pgExpectCode(t, "single shipment", st, out, 409, "in_parcel_group")
		req := httptest.NewRequest("POST", e.pgPath("/shipments/tracking-import/preview"), strings.NewReader("order_number,carrier,tracking_number\n"+member+",black_cat,SOLO0002\n"))
		req.Header.Set("Authorization", "Bearer "+e.token())
		req.Header.Set("Content-Type", "text/csv")
		w := httptest.NewRecorder()
		e.merchant.ServeHTTP(w, req)
		body := w.Body.String()
		if w.Code != http.StatusOK || !strings.Contains(body, `"in_parcel_group"`) || !strings.Contains(body, `"apply_rows":0`) && !strings.Contains(body, `"apply_rows": 0`) {
			t.Fatalf("bulk preview of a grouped order: %d %s", w.Code, body)
		}
		if e.pgState(member) != "MANUAL_UNASSIGNED" {
			t.Fatal("the grouped order shipped")
		}
	})

	t.Run("PG07 dissolve (CAS, OPEN only) then the order ships alone", func(t *testing.T) {
		member := pgSorted(o6, o7, o8)[0]
		del := func(group string, v string) (int, map[string]any, []byte) {
			return e.mcall(e.token(), "DELETE", e.pgPath("/parcel-groups/"+group+"?expected_version="+v), "", "")
		}
		st, out, _ := del(g2, "7")
		pgExpectCode(t, "stale version", st, out, 409, "version_changed")
		st, out, raw := del(g2, "1")
		if st != 200 || out["state"] != "DISSOLVED" || out["version"] != float64(2) {
			t.Fatalf("dissolve: %d %s", st, raw)
		}
		st, out, _ = del(g2, "2")
		pgExpectCode(t, "dissolve twice", st, out, 409, "group_not_open")
		st, out, _ = del(g1, "2")
		pgExpectCode(t, "dissolve a SHIPPED group", st, out, 409, "group_not_open")
		if n := e.count(`SELECT count(*) FROM fulfillment.parcel_group_orders WHERE group_id=$1`, g2); n != 0 {
			t.Fatalf("members left after dissolve: %d", n)
		}
		st, out, raw = e.mcall(e.token(), "PUT", e.pgPath("/orders/"+member+"/shipment"), "pg-single-0003", mfxShip(0, "black_cat", "SOLO0003"))
		if st != 200 {
			t.Fatalf("shipment after dissolve: %d %s", st, raw)
		}
		st, out, _ = e.pgShip(g2, "pg-ship-0004", "TRK55555555")
		pgExpectCode(t, "ship a dissolved group", st, out, 409, "group_not_open")
	})

	t.Run("PG08 two creators racing for the same orders: exactly one wins", func(t *testing.T) {
		p1, p2 := e.pgHome(b), e.pgHome(b)
		var wg sync.WaitGroup
		codes := make([]int, 2)
		for i := range codes {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				codes[i], _, _ = e.pgCreate("pg-race-000"+string(rune('1'+i)), p1, p2)
			}(i)
		}
		wg.Wait()
		sort.Ints(codes)
		if codes[0] != 201 || codes[1] != 409 {
			t.Fatalf("race statuses %v, want one 201 and one 409", codes)
		}
		if n := e.count(`SELECT count(*) FROM fulfillment.parcel_group_orders WHERE order_id=ANY($1::uuid[])`, []string{p1, p2}); n != 2 {
			t.Fatalf("membership rows %d, want 2", n)
		}
	})
	_ = ctx
}

// plParcelBody is the pick-list body with the W3-07B parcel_group_id.
type plParcelBody struct {
	Orders []struct {
		OrderID       string `json:"order_id"`
		ParcelGroupID string `json:"parcel_group_id"`
	} `json:"orders"`
}

// TestParcelGroupACL pins the 0146 surface: FORCE RLS, nothing for PUBLIC, only the definer owner touches the tables (writer: no
// DELETE on groups, UPDATE only on state/version), the INVOKER helpers are callable by nobody else, the migration is recorded.
func TestParcelGroupACL(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	tables := []string{"fulfillment.parcel_groups", "fulfillment.parcel_group_orders"}
	for _, tbl := range tables {
		var forced, public bool
		if err := f.owner.QueryRow(ctx, `SELECT c.relrowsecurity AND c.relforcerowsecurity,
		 EXISTS(SELECT 1 FROM aclexplode(coalesce(c.relacl,acldefault('r',c.relowner))) a WHERE a.grantee=0) FROM pg_class c WHERE c.oid=$1::regclass`, tbl).Scan(&forced, &public); err != nil {
			t.Fatal(err)
		}
		if !forced || public {
			t.Errorf("%s: forced RLS=%v public grant=%v", tbl, forced, public)
		}
		for _, role := range []string{"commerce_runtime", "commerce_auth", "commerce_checkout_runtime", waPayment, waLive, waExpiry, waAds, waClaims, waLegacy} {
			if n := countRows(t, f.owner, `SELECT count(*) FROM (SELECT 1 WHERE has_any_column_privilege($1,$2::regclass,'SELECT,INSERT,UPDATE') OR has_table_privilege($1,$2::regclass,'DELETE')) x`, role, tbl); n != 0 {
				t.Errorf("%s must have no privilege on %s", role, tbl)
			}
		}
	}
	for _, c := range []struct {
		tbl, priv string
		want      bool
	}{{tables[0], "SELECT", true}, {tables[0], "INSERT", true}, {tables[0], "DELETE", false}, {tables[1], "DELETE", true}, {tables[1], "UPDATE", false}} {
		var got bool
		if err := f.owner.QueryRow(ctx, `SELECT has_table_privilege('commerce_checkout_writer',$1::regclass,$2)`, c.tbl, c.priv).Scan(&got); err != nil || got != c.want {
			t.Errorf("writer %s on %s = %v (err %v), want %v", c.priv, c.tbl, got, err, c.want)
		}
	}
	for col, want := range map[string]bool{"state": true, "version": true, "owner_id": false, "destination_hash": false, "created_by": false} {
		var got bool
		if err := f.owner.QueryRow(ctx, `SELECT has_column_privilege('commerce_checkout_writer','fulfillment.parcel_groups',$1,'UPDATE')`, col).Scan(&got); err != nil || got != want {
			t.Errorf("writer UPDATE(%s) = %v (err %v), want %v", col, got, err, want)
		}
	}
	for _, sig := range []string{"fulfillment.parcel_destination_hash(jsonb,text)", "fulfillment.parcel_merge_block(uuid,uuid,uuid)"} {
		var owner string
		var definer bool
		var others int
		if err := f.owner.QueryRow(ctx, `SELECT pg_get_userbyid(proowner),prosecdef FROM pg_proc WHERE oid=$1::regprocedure`, sig).Scan(&owner, &definer); err != nil {
			t.Fatal(err)
		}
		if err := f.owner.QueryRow(ctx, `SELECT count(*) FROM pg_roles WHERE rolname LIKE 'commerce\_%' AND rolname<>'commerce_checkout_writer' AND has_function_privilege(oid,$1::regprocedure,'EXECUTE')`, sig).Scan(&others); err != nil {
			t.Fatal(err)
		}
		if owner != "commerce_checkout_writer" || definer || others != 0 {
			t.Errorf("%s: owner=%s definer=%v other callers=%d (want an owner-only INVOKER helper)", sig, owner, definer, others)
		}
	}
	// Behavioral: the runtime login cannot read the tables even under matching GUCs (only the definers can).
	tcsAs(t, f, "commerce_runtime", map[string]string{"app.tenant_id": f.tenantA, "app.store_id": f.storeA1}, func(ctx context.Context, tx pgx.Tx) {
		for _, tbl := range tables {
			if state, _ := tcsSub(ctx, tx, `SELECT 1 FROM `+tbl); state != "42501" {
				t.Errorf("commerce_runtime SELECT on %s: SQLSTATE %q, want 42501", tbl, state)
			}
		}
	})
	if !tcsBool(t, f, "0146", `SELECT EXISTS(SELECT 1 FROM public.lc_schema_migrations WHERE version=$1 AND checksum<>'')`, "0146_parcel_groups.sql") {
		t.Error("migration 0146_parcel_groups.sql is not recorded")
	}
}
