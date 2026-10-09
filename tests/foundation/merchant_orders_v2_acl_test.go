package foundation_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// 0110 is an internal read-contract extension, not an exception to KC03,
// LPC06 or TCV02. Their frozen direct-table privilege assertions stay unchanged.
func TestMerchantOrdersV2DomainReadAuthority(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	for _, helper := range []struct{ signature, owner, result string }{
		{"claims.order_live_sources(uuid,uuid,uuid[])", "commerce_claims_writer", "TABLE(order_id uuid, session_id uuid)"},
		{"live.order_session_labels(uuid,uuid,uuid[])", "commerce_media_writer", "TABLE(session_id uuid, name text, session_created_at timestamp with time zone)"},
		{"fulfillment.order_cvs_tracking(uuid,uuid,uuid[])", "commerce_checkout_writer", "TABLE(order_id uuid, state text, provider_logistics_id text, shipment_no text)"},
	} {
		t.Run(helper.signature, func(t *testing.T) {
			var owner, result string
			var definer, noLogin, noBypass, fixedPath, stable bool
			var principals []string
			err := f.owner.QueryRow(ctx, `SELECT pg_get_userbyid(p.proowner), pg_get_function_result(p.oid),
			 p.prosecdef, NOT r.rolcanlogin, NOT r.rolbypassrls,
			 p.proconfig=ARRAY['search_path=pg_catalog']::text[], p.provolatile='s',
			 ARRAY(SELECT CASE WHEN a.grantee=0 THEN 'PUBLIC' ELSE pg_get_userbyid(a.grantee) END
			 FROM aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a
			 WHERE a.privilege_type='EXECUTE' ORDER BY 1)
			 FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid=$1::regprocedure`, helper.signature).
				Scan(&owner, &result, &definer, &noLogin, &noBypass, &fixedPath, &stable, &principals)
			if err != nil {
				t.Fatal(err)
			}
			if owner != helper.owner || result != helper.result || !definer || !noLogin || !noBypass || !fixedPath || !stable ||
				!slices.Equal(principals, []string{"commerce_auth", helper.owner}) {
				t.Fatalf("domain boundary: owner=%s result=%s definer=%v noLogin=%v noBypass=%v path=%v stable=%v ACL=%v", owner, result, definer, noLogin, noBypass, fixedPath, stable, principals)
			}
			// Enumerate every commerce role, including workers and web logins. An
			// internal helper must not become an unauthenticated runtime endpoint.
			var others int
			if err := f.owner.QueryRow(ctx, `SELECT count(*) FROM pg_roles WHERE rolname LIKE 'commerce\_%'
			 AND rolname NOT IN ('commerce_auth',$2) AND has_function_privilege(oid,$1,'EXECUTE')`, helper.signature, helper.owner).Scan(&others); err != nil || others != 0 {
				t.Fatalf("unexpected helper callers=%d err=%v", others, err)
			}
		})
	}
	var body string
	var config []string
	if err := f.owner.QueryRow(ctx, `SELECT prosrc,coalesce(proconfig,'{}') FROM pg_proc WHERE oid='identity.read_merchant_orders_v2(bytea,uuid,integer,timestamptz,uuid,text,text,text,text,text,uuid,timestamptz,timestamptz)'::regprocedure`).Scan(&body, &config); err != nil {
		t.Fatal(err)
	}
	// The reader's single statement is costed far above PG's JIT thresholds; with jit on every call spent 0.3-0.7 s compiling
	// (the 10k search gate's intermittent >1 s). The function-level setting is the guard, so it must not be dropped.
	if !slices.Equal(config, []string{"search_path=pg_catalog", "jit=off"}) {
		t.Fatalf("v2 reader proconfig=%v, want search_path=pg_catalog and jit=off", config)
	}
	for _, table := range []string{"claims.live_price_uses", "claims.bundles", "live.sessions", "fulfillment.cvs_shipments"} {
		if strings.Contains(body, table) {
			t.Fatalf("v2 reader still reaches domain table %s", table)
		}
	}
	var policies int
	if err := f.owner.QueryRow(ctx, `SELECT count(*) FROM pg_policy WHERE polname IN
	 ('orders_v2_price_use_read','orders_v2_bundle_read','orders_v2_session_read')`).Scan(&policies); err != nil || policies != 0 {
		t.Fatalf("0110 added domain policies=%d err=%v", policies, err)
	}
}

// 0146 (W3-07B) adds seven parcel-group definers with the same shape (owner commerce_checkout_writer, EXECUTE commerce_runtime only),
// 0166 (W3-U4) the eighth (read_open_parcel_groups);
// 0155 (W3-08B) adds the eight returns.* definers and fulfillment.merchant_cancel_order with it.
// 0130 adds two read-only definers for the pick list: the shared reader (EXECUTE commerce_runtime only,
// owner commerce_checkout_writer) and the session->orders resolution helper (claims-owned, EXECUTE
// commerce_checkout_writer only). No worker authority, legacy role, auth login, buyer role or any other
// commerce_* role may call either.
func TestMerchantOrdersV2PickListReadAuthority(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	for _, helper := range []struct {
		signature, owner string
		exec             []string // explicit non-owner grantees (the owner holds the implicit grant)
		stable           bool
	}{
		{"fulfillment.read_pick_list(bytea,uuid,uuid[],uuid,boolean)", "commerce_checkout_writer", []string{"commerce_runtime"}, false},
		{"claims.pick_list_session_orders(uuid,uuid,uuid)", "commerce_claims_writer", []string{"commerce_checkout_writer"}, true},
		{"fulfillment.read_merge_suggestions(bytea,uuid)", "commerce_checkout_writer", []string{"commerce_runtime"}, false},                          // 0146 W3-07B
		{"fulfillment.create_parcel_group(bytea,uuid,text,bytea,uuid[])", "commerce_checkout_writer", []string{"commerce_runtime"}, false},           // 0146 W3-07B
		{"fulfillment.dissolve_parcel_group(bytea,uuid,uuid,bigint)", "commerce_checkout_writer", []string{"commerce_runtime"}, false},               // 0146 W3-07B
		{"fulfillment.begin_parcel_group_shipment(bytea,uuid,uuid)", "commerce_checkout_writer", []string{"commerce_runtime"}, false},                // 0146 W3-07B
		{"fulfillment.mark_parcel_group_shipped(bytea,uuid,uuid)", "commerce_checkout_writer", []string{"commerce_runtime"}, false},                  // 0146 W3-07B
		{"fulfillment.guard_parcel_group_orders(bytea,uuid,uuid[])", "commerce_checkout_writer", []string{"commerce_runtime"}, false},                // 0146 W3-07B
		{"fulfillment.read_parcel_group_ids(bytea,uuid,uuid[])", "commerce_checkout_writer", []string{"commerce_runtime"}, false},                    // 0146 W3-07B
		{"fulfillment.read_open_parcel_groups(bytea,uuid)", "commerce_checkout_writer", []string{"commerce_runtime"}, false},                         // 0166 W3-U4
		{"returns.register_rma(bytea,uuid,uuid,text,bytea,text,jsonb)", "commerce_checkout_writer", []string{"commerce_runtime"}, false},             // 0155 W3-08B
		{"returns.receive_rma(bytea,uuid,uuid,text,bytea,bigint,jsonb)", "commerce_checkout_writer", []string{"commerce_runtime"}, false},            // 0155 W3-08B
		{"returns.inspect_rma(bytea,uuid,uuid,text,bytea,bigint,jsonb)", "commerce_checkout_writer", []string{"commerce_runtime"}, false},            // 0155 W3-08B
		{"returns.cancel_rma(bytea,uuid,uuid,text,bytea,bigint)", "commerce_checkout_writer", []string{"commerce_runtime"}, false},                   // 0155 W3-08B
		{"returns.close_rma(bytea,uuid,uuid,text,bytea,bigint,uuid)", "commerce_checkout_writer", []string{"commerce_runtime"}, false},               // 0155 W3-08B
		{"returns.read_order_returns(bytea,uuid,uuid)", "commerce_checkout_writer", []string{"commerce_runtime"}, false},                             // 0155 W3-08B
		{"returns.list_returns(bytea,uuid,text)", "commerce_checkout_writer", []string{"commerce_runtime"}, false},                                   // 0155 W3-08B
		{"fulfillment.merchant_cancel_order(bytea,uuid,uuid,text,bytea,text,text)", "commerce_checkout_writer", []string{"commerce_runtime"}, false}, // 0155 W3-08B
		{"returns.list_cancel_refund_gaps(bytea,uuid)", "commerce_checkout_writer", []string{"commerce_runtime"}, false},                             // 0155 W3-08B
	} {
		t.Run(helper.signature, func(t *testing.T) {
			var owner string
			var definer, noLogin, noBypass, fixedPath, stable, publicExec bool
			var principals []string
			err := f.owner.QueryRow(ctx, `SELECT pg_get_userbyid(p.proowner),p.prosecdef,NOT r.rolcanlogin,NOT r.rolbypassrls,
			 p.proconfig=ARRAY['search_path=pg_catalog']::text[], p.provolatile='s',
			 EXISTS(SELECT 1 FROM aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a WHERE a.grantee=0 AND a.privilege_type='EXECUTE'),
			 ARRAY(SELECT pg_get_userbyid(a.grantee) FROM aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a
			  WHERE a.privilege_type='EXECUTE' AND a.grantee<>p.proowner ORDER BY 1)
			 FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid=$1::regprocedure`, helper.signature).
				Scan(&owner, &definer, &noLogin, &noBypass, &fixedPath, &stable, &publicExec, &principals)
			if err != nil {
				t.Fatal(err)
			}
			if owner != helper.owner || !definer || !noLogin || !noBypass || !fixedPath || stable != helper.stable || publicExec ||
				!slices.Equal(principals, helper.exec) {
				t.Fatalf("pick-list definer: owner=%s definer=%v noLogin=%v noBypass=%v path=%v stable=%v publicExec=%v ACL=%v",
					owner, definer, noLogin, noBypass, fixedPath, stable, publicExec, principals)
			}
			// Enumerate every commerce role except the owner and its explicit grantees: none may call it.
			var others int
			if err := f.owner.QueryRow(ctx, `SELECT count(*) FROM pg_roles WHERE rolname LIKE 'commerce\_%'
			 AND rolname <> $2 AND NOT rolname = ANY($3::text[]) AND has_function_privilege(oid,$1,'EXECUTE')`,
				helper.signature, helper.owner, helper.exec).Scan(&others); err != nil || others != 0 {
				t.Fatalf("unexpected pick-list caller=%d err=%v", others, err)
			}
		})
	}
}

func TestMerchantOrdersV2DomainReadScope(t *testing.T) {
	e := ltgNew(t)
	f, ctx := e.p.f, context.Background()
	session, _ := e.session("A1", ltgLive, 5)
	_, link := e.claimLink(session, "synthetic-acl", "A1+2")
	b := e.redeemed(link)
	quote, _ := e.line(b, "")
	order, err := e.placeHome(b, quote)
	if err != nil {
		t.Fatal(err)
	}
	// Real local checkout and fake-provider label lifecycle, no planted ledger
	// rows and no real provider account or customer data.
	e.startDispatcher()
	e.grantCreator("fulfillment:write", "integration:manage", "integration:read")
	e.connect("C2C")
	e.cvsSettings(tcvAllChains, true, "20000", 500)
	code, _, _ := e.service("cvs_711", "API", 0)
	cvsOrder, _ := e.cvsOrder(tcvOrderSpec{kind: "cvs_711", code: code, paymentMode: "pay_at_pickup"})
	if status, _, body := e.ship(e.token(), cvsOrder, 0, "", true); status != 202 {
		t.Fatalf("local label request: %d %s", status, body)
	}
	e.awaitShip(cvsOrder, "CREATED")
	var tracking string
	var shipment *string // C2C may have a provider ID before a shipment number.
	if err := f.owner.QueryRow(ctx, `SELECT provider_logistics_id,shipment_no FROM fulfillment.cvs_shipments WHERE order_id=$1 ORDER BY attempt DESC LIMIT 1`, cvsOrder).Scan(&tracking, &shipment); err != nil || tracking == "" {
		t.Fatalf("missing fake shipment witness: %v", err)
	}
	tx, err := f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	must := func(query string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	must(`SELECT set_config('app.tenant_id',$1,true),set_config('app.store_id',$2,true),set_config('app.principal_id',$3,true)`, f.tenantA, e.store(), f.principalA)
	must(`SET LOCAL ROLE commerce_auth`)
	for _, h := range []struct{ function, id, witness string }{
		{"claims.order_live_sources", order.OrderID, session},
		{"live.order_session_labels", session, session},
		{"fulfillment.order_cvs_tracking", cvsOrder, tracking},
	} {
		column := "session_id::text"
		if h.function == "fulfillment.order_cvs_tracking" {
			column = "provider_logistics_id||'/'||coalesce(shipment_no,'<NULL>')||'/'||state"
			if shipment == nil {
				h.witness += "/<NULL>/CREATED"
			} else {
				h.witness += "/" + *shipment + "/CREATED"
			}
		}
		// Repeat on one connection beyond generic-plan promotion. The tenant and
		// store parameters are deliberately wrong while RLS context stays valid:
		// these negatives exercise the explicit predicates, not just RLS.
		for i := 0; i < 9; i++ {
			var got string
			if err := tx.QueryRow(ctx, "SELECT "+column+" FROM "+h.function+"($1,$2,$3)", f.tenantA, e.store(), []string{h.id}).Scan(&got); err != nil || got != h.witness {
				t.Fatalf("%s positive read #%d: %q err=%v", h.function, i, got, err)
			}
		}
		for _, scope := range []struct {
			tenant, store string
			ids           []string
		}{
			{randomUUID(), e.store(), []string{h.id}},
			{f.tenantA, randomUUID(), []string{h.id}},
			{f.tenantA, e.store(), []string{randomUUID()}},
			{f.tenantA, e.store(), []string{}},
			{f.tenantA, e.store(), nil},
		} {
			var count int
			if err := tx.QueryRow(ctx, "SELECT count(*) FROM "+h.function+"($1,$2,$3)", scope.tenant, scope.store, scope.ids).Scan(&count); err != nil || count != 0 {
				t.Fatalf("%s cross-scope/empty read count=%d err=%v", h.function, count, err)
			}
		}
	}
	// Roll back SET ROLE and the read transaction before the public projection.
	if err := tx.Rollback(ctx); err != nil && err != pgx.ErrTxClosed {
		t.Fatal(err)
	}
	rows, err := moReadV2(ctx, f.runtime, e.token(), f.tenantA, e.store(), f.principalA, "", "all", 100)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range rows {
		if row["order_id"] == order.OrderID {
			sources, ok := row["live_sessions"].([]any)
			if !ok || len(sources) != 1 || sources[0].(map[string]any)["id"] != session {
				t.Fatalf("order lost its scoped live provenance: %v", row["live_sessions"])
			}
			found = true
		}
	}
	if !found {
		t.Fatal("live order missing from authenticated v2 projection")
	}
	search, _ := json.Marshal(map[string]string{"q": tracking})
	req := httptest.NewRequest("POST", "/v1/admin/stores/"+e.store()+"/orders/search?view=v2", strings.NewReader(string(search)))
	req.Header.Set("Authorization", "Bearer "+e.token())
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	e.merchant.ServeHTTP(response, req)
	var page struct {
		Items []struct {
			ID string `json:"order_id"`
		} `json:"items"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || response.Code != 200 || page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != cvsOrder {
		t.Fatalf("CVS tracking search: status=%d body=%s err=%v", response.Code, response.Body.String(), err)
	}
	// Queue counts read the same labelled CVS state: the real CREATED label is the store's one consign task and not yet shipped.
	list := httptest.NewRequest("GET", "/v1/admin/stores/"+e.store()+"/orders?view=v2&limit=100", nil)
	list.Header.Set("Authorization", "Bearer "+e.token())
	listed := httptest.NewRecorder()
	e.merchant.ServeHTTP(listed, list)
	var queues struct {
		Counts map[string]int `json:"counts"`
	}
	if err := json.Unmarshal(listed.Body.Bytes(), &queues); err != nil || listed.Code != 200 || queues.Counts["ready_to_consign"] != 1 || queues.Counts["shipped"] != 0 {
		t.Fatalf("CVS queue counts: status=%d body=%s err=%v", listed.Code, listed.Body.String(), err)
	}
}
