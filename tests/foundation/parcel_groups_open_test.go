package foundation_test

// Purpose: W3-U4 amendment of W3-07B over REAL_PG and the real HTTP handler: GET /parcel-groups (migration 0166
//   fulfillment.read_open_parcel_groups) lists exactly the store's OPEN groups with version and masked members, so the orders page
//   can rebuild ship/dissolve panels after a reload. Pins: empty store, newest first, SHIPPED/DISSOLVED absent, other store absent,
//   orders:read authority (no token 401, fulfillment:write-only 403, foreign-store member 404), masking identical to the orders
//   list (incl. leading ideographic space and a blank name), and no name/phone/address/amount in the body. W3-U4 finisher:
//   GET /orders/merge-suggestions (0146 read_merge_suggestions, amended in place by 0166) is a LIST-level read the orders page
//   fires on every load, so it returns recipient_masked (the list mask), never the full recipient name or phone.
// Depends on: tcvEnv (stripe mode, card home orders), pgHome/pgCreate/pgShip helpers of parcel_groups_test.go, e.pgMemberOn
//   (promotions_gate_test.go), the 0166 definer, routes /parcel-groups, /orders (v2 list), DELETE /parcel-groups/{id}.
// Used by: go test ./tests/foundation (-run '^TestParcelGroupOpenRead'); evidence class REAL_PG with MOCK Stripe fakes.

import (
	"strings"
	"testing"
)

// pgoOpen reads GET /parcel-groups as token on store and returns the status, the items and the raw body.
func (e *tcvEnv) pgoOpen(token, store string) (int, []map[string]any, string) {
	e.t.Helper()
	st, out, raw := e.mcall(token, "GET", "/v1/admin/stores/"+store+"/parcel-groups", "", "")
	var items []map[string]any
	if raw, ok := out["items"].([]any); ok {
		for _, it := range raw {
			items = append(items, it.(map[string]any))
		}
	}
	return st, items, string(raw)
}

// pgoMembers returns order_id -> recipient_masked of one listed group, failing on a malformed member.
func pgoMembers(t *testing.T, group map[string]any) map[string]string {
	t.Helper()
	got := map[string]string{}
	for _, raw := range group["members"].([]any) {
		m := raw.(map[string]any)
		id, _ := m["order_id"].(string)
		if num, _ := m["order_number"].(string); num != "LC-"+strings.ToUpper(strings.ReplaceAll(id, "-", "")) || len(m) != 3 {
			t.Fatalf("member %v: order_number must be LC-<id> and the member carry exactly order_id, order_number, recipient_masked", m)
		}
		got[id], _ = m["recipient_masked"].(string)
	}
	return got
}

func TestParcelGroupOpenRead(t *testing.T) {
	e := tcvNew(t, tcvOpts{stripe: true})
	e.r.startWorker(t)
	e.grantCreator("orders:read", "fulfillment:write")
	f := e.p.f

	if st, items, raw := e.pgoOpen(e.token(), e.store()); st != 200 || len(items) != 0 || !strings.Contains(raw, `"items":[]`) {
		t.Fatalf("empty store: %d %s, want 200 with items []", st, raw)
	}

	b, other := e.newBuyer(), e.newBuyer()
	o1, o2 := e.pgHome(b), e.pgHome(b)
	x1, x2 := e.pgHome(other), e.pgHome(other)
	s1, s2 := e.pgHome(b), e.pgHome(b)
	d1, d2 := e.pgHome(b), e.pgHome(b)
	group := func(key string, ids ...string) (id string) {
		st, out, raw := e.pgCreate(key, ids...)
		if st != 201 {
			t.Fatalf("create %s: %d %s", key, st, raw)
		}
		return out["id"].(string)
	}

	g1 := group("pgo-create-0001", o1, o2)
	t.Run("one OPEN group is listed with its version and masked members", func(t *testing.T) {
		st, items, raw := e.pgoOpen(e.token(), e.store())
		if st != 200 || len(items) != 1 || items[0]["group_id"] != g1 || items[0]["version"] != float64(1) {
			t.Fatalf("open groups: %d %s", st, raw)
		}
		if _, ok := items[0]["created_at"].(string); !ok || len(items[0]) != 4 {
			t.Fatalf("group %v: want exactly group_id, version, created_at, members", items[0])
		}
		if m := pgoMembers(t, items[0]); len(m) != 2 || m[o1] == "" || m[o2] == "" {
			t.Fatalf("members %v, want {o1,o2}", m)
		}
		// Display fields only: never the recipient name, phone, address or any amount.
		for _, leak := range []string{hcodName, hcodPhone, "Synthetic", "total_minor", "phone", "address", "recipient_name"} {
			if strings.Contains(raw, leak) {
				t.Fatalf("response leaks %q: %s", leak, raw)
			}
		}
	})

	g2 := group("pgo-create-0002", x1, x2)
	gS := group("pgo-create-0003", s1, s2)
	gD := group("pgo-create-0004", d1, d2)
	if st, _, raw := e.pgShip(gS, "pgo-ship-0001", "TRKOPEN0001"); st != 200 {
		t.Fatalf("ship gS: %d %s", st, raw)
	}
	if st, _, raw := e.mcall(e.token(), "DELETE", e.pgPath("/parcel-groups/"+gD+"?expected_version=1"), "", ""); st != 200 {
		t.Fatalf("dissolve gD: %d %s", st, raw)
	}

	t.Run("newest first; SHIPPED and DISSOLVED groups are absent", func(t *testing.T) {
		if n := e.count(`SELECT count(*) FROM fulfillment.parcel_groups WHERE store_id=$1`, e.store()); n != 4 {
			t.Fatalf("fixture has %d groups, want 4 (2 OPEN + SHIPPED + DISSOLVED)", n)
		}
		st, items, raw := e.pgoOpen(e.token(), e.store())
		if st != 200 || len(items) != 2 || items[0]["group_id"] != g2 || items[1]["group_id"] != g1 {
			t.Fatalf("open groups %d %s, want [g2, g1] (gS shipped %s and gD dissolved %s hidden)", st, raw, gS, gD)
		}
		for _, hidden := range []string{gS, gD} {
			if strings.Contains(raw, hidden) {
				t.Fatalf("group %s is not OPEN yet it is listed", hidden)
			}
		}
		if m := pgoMembers(t, items[0]); len(m) != 2 || m[x1] == "" || m[x2] == "" {
			t.Fatalf("g2 members %v, want {x1,x2}", m)
		}
	})

	t.Run("recipient_masked equals the orders-list mask, incl. leading ideographic space and a blank name", func(t *testing.T) {
		// Disclosed fixtures (owner pool): legacy-looking names on two members; the list and the group read must agree row by row.
		mustExec(t, f.owner, `UPDATE checkout.orders SET snapshot=jsonb_set(snapshot,'{destination,recipient_name}',to_jsonb(E'　　王小明'::text)) WHERE id=$1`, o1)
		mustExec(t, f.owner, `UPDATE checkout.orders SET snapshot=jsonb_set(snapshot,'{destination,recipient_name}','"   "') WHERE id=$1`, o2)
		st, items, raw := e.pgoOpen(e.token(), e.store())
		if st != 200 {
			t.Fatalf("open groups: %d %s", st, raw)
		}
		masked := map[string]string{}
		for _, g := range items {
			for id, m := range pgoMembers(t, g) {
				masked[id] = m
			}
		}
		if masked[o1] != "王***" || masked[o2] != "—" || masked[x1] != "王***" {
			t.Fatalf("masks o1=%q o2=%q x1=%q, want 王*** / — / 王***", masked[o1], masked[o2], masked[x1])
		}
		st, list, rawList := e.mcall(e.token(), "GET", "/v1/admin/stores/"+e.store()+"/orders?view=v2&state=unshipped&limit=100", "", "")
		rows, _ := list["items"].([]any)
		if st != 200 || len(rows) == 0 {
			t.Fatalf("orders list: %d %s", st, rawList)
		}
		seen := 0
		for _, r := range rows {
			row := r.(map[string]any)
			id, _ := row["order_id"].(string)
			if want, ok := masked[id]; ok {
				seen++
				if row["recipient_masked"] != want {
					t.Fatalf("order %s: list mask %v, group mask %q", id, row["recipient_masked"], want)
				}
			}
		}
		if seen != len(masked) {
			t.Fatalf("the orders list showed %d of the %d grouped orders", seen, len(masked))
		}
	})

	t.Run("authority: no token 401, fulfillment:write alone 403, foreign-store member 404, other store sees none", func(t *testing.T) {
		if st, _, _ := e.pgoOpen("", e.store()); st != 401 {
			t.Fatalf("no token: %d, want 401", st)
		}
		writeOnly, _ := e.member("fulfillment:write")
		if st, _, raw := e.pgoOpen(writeOnly, e.store()); st != 403 {
			t.Fatalf("fulfillment:write without orders:read: %d %s, want 403", st, raw)
		}
		readOnly, _ := e.member("orders:read")
		if st, items, raw := e.pgoOpen(readOnly, e.store()); st != 200 || len(items) != 2 {
			t.Fatalf("orders:read member: %d %s, want the 2 OPEN groups", st, raw)
		}
		// A member of ANOTHER store of the same tenant: its own store lists none of store A1's groups, and A1's path is not found.
		storeA2 := randomUUID()
		mustExec(t, f.owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'pgo-store-a2','TWD')`, e.tenant(), storeA2)
		a2 := e.pgMemberOn(e.tenant(), storeA2, "orders:read")
		if st, items, raw := e.pgoOpen(a2, storeA2); st != 200 || len(items) != 0 || !strings.Contains(raw, `"items":[]`) {
			t.Fatalf("store A2 member on A2: %d %s, want an empty list (A1's groups must not leak)", st, raw)
		}
		if st, _, raw := e.pgoOpen(a2, e.store()); st != 404 {
			t.Fatalf("store A2 member on A1: %d %s, want 404", st, raw)
		}
	})

	t.Run("definer ACL: read only (no rows written), EXECUTE for commerce_runtime only", func(t *testing.T) {
		before := e.count(`SELECT count(*) FROM fulfillment.parcel_groups WHERE store_id=$1`, e.store()) +
			e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1`, e.store())
		e.pgoOpen(e.token(), e.store())
		after := e.count(`SELECT count(*) FROM fulfillment.parcel_groups WHERE store_id=$1`, e.store()) +
			e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1`, e.store())
		if before != after {
			t.Fatalf("the read wrote rows: %d -> %d", before, after)
		}
		if n := countRows(t, f.owner, `SELECT count(*) FROM pg_roles WHERE rolname LIKE 'commerce\_%' AND rolname NOT IN ('commerce_checkout_writer','commerce_runtime')
		 AND has_function_privilege(rolname,'fulfillment.read_open_parcel_groups(bytea,uuid)','EXECUTE')`); n != 0 {
			t.Fatalf("%d unexpected roles can execute read_open_parcel_groups", n)
		}
	})
}

// TestParcelGroupMergeSuggestionsMasked pins the P1-A leak fix: the suggestions read is fetched on every orders-page load, so like
// the orders list it may carry only the masked recipient. Three buyers at the shared home address (owner pool fixtures give two of
// them a legacy-looking name) must yield exactly {recipient_masked, order_ids} items whose mask equals the orders-list row mask,
// and the raw body must contain neither the full name nor the phone.
func TestParcelGroupMergeSuggestionsMasked(t *testing.T) {
	e := tcvNew(t, tcvOpts{stripe: true})
	e.r.startWorker(t)
	e.grantCreator("orders:read", "fulfillment:write")
	f := e.p.f

	b, c, d := e.newBuyer(), e.newBuyer(), e.newBuyer()
	b1, b2 := e.pgHome(b), e.pgHome(b)
	c1, c2 := e.pgHome(c), e.pgHome(c)
	d1, d2 := e.pgHome(d), e.pgHome(d)
	for _, id := range []string{c1, c2} { // leading ideographic space: the mask starts at the first non-blank rune
		mustExec(t, f.owner, `UPDATE checkout.orders SET snapshot=jsonb_set(snapshot,'{destination,recipient_name}',to_jsonb(E'　　王小明'::text)) WHERE id=$1`, id)
	}
	for _, id := range []string{d1, d2} { // blank name: the placeholder
		mustExec(t, f.owner, `UPDATE checkout.orders SET snapshot=jsonb_set(snapshot,'{destination,recipient_name}','"   "') WHERE id=$1`, id)
	}

	st, out, raw := e.mcall(e.token(), "GET", e.pgPath("/orders/merge-suggestions"), "", "")
	if st != 200 {
		t.Fatalf("merge-suggestions answered %d: %s", st, raw)
	}
	for _, leak := range []string{hcodName, hcodPhone, "recipient_name", "Synthetic"} {
		if strings.Contains(string(raw), leak) {
			t.Fatalf("merge-suggestions leaks %q to the browser: %s", leak, raw)
		}
	}
	want := map[string]string{pgSorted(b1, b2)[0]: "王***", pgSorted(c1, c2)[0]: "王***", pgSorted(d1, d2)[0]: "—"}
	items, _ := out["items"].([]any)
	if len(items) != 3 {
		t.Fatalf("suggestions %s, want 3 sets (one per buyer)", raw)
	}
	st, list, rawList := e.mcall(e.token(), "GET", "/v1/admin/stores/"+e.store()+"/orders?view=v2&state=unshipped&limit=100", "", "")
	rows, _ := list["items"].([]any)
	if st != 200 || len(rows) == 0 {
		t.Fatalf("orders list: %d %s", st, rawList)
	}
	listMask := map[string]any{}
	for _, r := range rows {
		row := r.(map[string]any)
		listMask[row["order_id"].(string)] = row["recipient_masked"]
	}
	for _, it := range items {
		s := it.(map[string]any)
		ids := pgIDs(s["order_ids"])
		if len(s) != 2 || len(ids) != 2 {
			t.Fatalf("item %v: want exactly recipient_masked and two order_ids", s)
		}
		if w := want[ids[0]]; s["recipient_masked"] != w {
			t.Fatalf("set %v: recipient_masked %v, want %q", ids, s["recipient_masked"], w)
		}
		for _, id := range ids {
			if listMask[id] != s["recipient_masked"] {
				t.Fatalf("order %s: orders-list mask %v differs from the suggestion mask %v", id, listMask[id], s["recipient_masked"])
			}
		}
	}
}

// TestParcelGroupWaybillFieldsPersist: the group waybill carries EVERY control the merchant UI offers (carrier incl. `other` + name,
// tracking, https link, note) onto each member's shipment version, and a non-https link is refused with the group still OPEN
// (Codex P1 on PR #2; the browser spec types the same values, here the backend is proven without a browser).
func TestParcelGroupWaybillFieldsPersist(t *testing.T) {
	e := tcvNew(t, tcvOpts{stripe: true})
	e.r.startWorker(t)
	e.grantCreator("orders:read", "fulfillment:write")
	b := e.newBuyer()
	o1, o2, k1, k2 := e.pgHome(b), e.pgHome(b), e.pgHome(b), e.pgHome(b)
	create := func(key string, ids ...string) string {
		st, out, raw := e.pgCreate(key, ids...)
		if st != 201 {
			t.Fatalf("create %s: %d %s", key, st, raw)
		}
		return out["id"].(string)
	}
	gOther, gKnown := create("pgw-create-0001", o1, o2), create("pgw-create-0002", k1, k2)
	put := func(group, key, body string) (int, map[string]any, []byte) {
		return e.mcall(e.token(), "PUT", e.pgPath("/parcel-groups/"+group+"/shipment"), key, body)
	}
	const url, note = "https://track.example.com/t/SYPARCEL1234567890", "Group waybill: handle with care"
	if st, out, raw := put(gOther, "pgw-ship-bad-url", mfxShipBody(0, "SHIPPED", "other", "Synthetic Courier", "SYPARCEL1234567890", "http://track.example.com/t/x", note, "")); st != 422 && st != 400 {
		t.Fatalf("non-https link: %d %v %s, want a 4xx refusal", st, out, raw)
	}
	if n := e.count(`SELECT count(*) FROM fulfillment.parcel_groups WHERE id=$1 AND state='OPEN'`, gOther); n != 1 {
		t.Fatal("a refused waybill must leave the group OPEN")
	}
	if st, _, raw := put(gOther, "pgw-ship-other-01", mfxShipBody(0, "SHIPPED", "other", "Synthetic Courier", "SYPARCEL1234567890", url, note, "")); st != 200 {
		t.Fatalf("ship other-carrier group: %d %s", st, raw)
	}
	if st, _, raw := put(gKnown, "pgw-ship-known-01", mfxShipBody(0, "SHIPPED", "black_cat", "", "RELOAD1234567890", "https://track.example.org/r/RELOAD1234567890", "Reloaded panel waybill", "")); st != 200 {
		t.Fatalf("ship known-carrier group: %d %s", st, raw)
	}
	rows := func(ids []string, code, name, tracking, link, memo string) int {
		return e.count(`SELECT count(*) FROM fulfillment.manual_shipment_heads h
		 JOIN fulfillment.manual_shipment_versions v ON (v.tenant_id,v.store_id,v.order_id,v.version)=(h.tenant_id,h.store_id,h.order_id,h.current_version)
		 WHERE h.store_id=$1 AND h.order_id=ANY($2::uuid[]) AND v.status='SHIPPED' AND v.carrier_code=$3 AND coalesce(v.carrier_name,'')=$4
		  AND v.tracking_number=$5 AND v.tracking_url=$6 AND v.note=$7`, e.store(), ids, code, name, tracking, link, memo)
	}
	if n := rows([]string{o1, o2}, "other", "Synthetic Courier", "SYPARCEL1234567890", url, note); n != 2 {
		t.Fatalf("other-carrier members with every field persisted: %d, want 2", n)
	}
	if n := rows([]string{k1, k2}, "black_cat", "", "RELOAD1234567890", "https://track.example.org/r/RELOAD1234567890", "Reloaded panel waybill"); n != 2 {
		t.Fatalf("known-carrier members with every field persisted: %d, want 2", n)
	}
}
