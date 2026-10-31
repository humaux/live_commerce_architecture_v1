// Purpose: exercise FEAT-AUTO-COLLECTIONS through merchant and published buyer HTTP routes.
// Depends on: ccNew/ccAdmin real PostgreSQL fixtures, catalog products and storefront-v2 section A.
// Used by: GitHub Foundation and Gates foundation shards; never run locally for this card.
// Invariants: membership is evaluated from current products, scoped by server auth, with no copied membership.
package foundation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
)

func acRules(match string, days ...int) map[string]any {
	conditions := []any{}
	for _, d := range days {
		conditions = append(conditions, map[string]any{"field": "created_at", "operator": "within_days", "days": d})
	}
	return map[string]any{"match": match, "conditions": conditions}
}

func acCreate(e *ccEnv, rules map[string]any) ccCollection {
	e.t.Helper()
	return e.collection("automatic", map[string]any{"membership_mode": "automatic", "rules": rules, "sort_mode": "newest"})
}

func acMembers(e *ccEnv, c ccCollection) []string {
	e.t.Helper()
	return ccIDs(e.list("q=" + e.tag + "&collection=" + c.Slug))
}

func acWantMembers(t *testing.T, e *ccEnv, c ccCollection, want ...string) {
	t.Helper()
	if got := acMembers(e, c); !reflect.DeepEqual(got, want) {
		t.Fatalf("automatic members = %v, want %v", got, want)
	}
}

// One top-level test leaves the suite's catch-all shard able to discover all new cases without a shard exemption.
func TestCatalogCoreAutomaticCollections(t *testing.T) {
	t.Run("all_any_read_time_sort_and_no_materialization", func(t *testing.T) {
		e := ccNew(t)
		fresh, _ := e.simple("fresh", 900, 0, nil)
		middle, _ := e.simple("middle", 100, 0, nil)
		old, _ := e.simple("old", 500, 0, nil)
		for _, p := range []*ccProduct{&fresh, &middle, &old} {
			e.activate(p)
		}
		mustExec(t, e.h.f.owner, `UPDATE catalog.products SET created_at=clock_timestamp()-interval '3 days' WHERE id=$1`, middle.ID)
		mustExec(t, e.h.f.owner, `UPDATE catalog.products SET created_at=clock_timestamp()-interval '10 days' WHERE id=$1`, old.ID)
		all := acCreate(e, acRules("all", 1, 7))
		any := acCreate(e, acRules("any", 1, 7))
		acWantMembers(t, e, all, fresh.ID)
		acWantMembers(t, e, any, fresh.ID, middle.ID)
		if got := ccIDs(e.list("q=" + e.tag + "&collection=" + any.Slug + "&sort=price_asc")); !reflect.DeepEqual(got, []string{middle.ID, fresh.ID}) {
			t.Fatalf("price sort = %v", got)
		}
		// Moving only the source product timestamp changes membership immediately, without saving rules.
		mustExec(t, e.h.f.owner, `UPDATE catalog.products SET created_at=clock_timestamp()-interval '20 days' WHERE id=$1`, fresh.ID)
		acWantMembers(t, e, any, middle.ID)
		late, _ := e.simple("late", 300, 0, nil)
		e.activate(&late)
		acWantMembers(t, e, any, late.ID, middle.ID)
		var copied int
		if err := e.h.f.owner.QueryRow(context.Background(), `SELECT count(*) FROM catalog.collection_products WHERE collection_id IN ($1,$2)`, all.ID, any.ID).Scan(&copied); err != nil {
			t.Fatal(err)
		}
		if copied != 0 {
			t.Fatalf("automatic evaluation copied %d membership rows", copied)
		}
	})

	t.Run("buyer_visibility_count_detail_and_unpublished_origin", func(t *testing.T) {
		e := ccNew(t)
		visible, _ := e.simple("visible", 100, 0, nil)
		draft, _ := e.simple("draft", 100, 0, nil)
		archived, _ := e.simple("archived", 100, 0, nil)
		e.activate(&visible)
		e.setStatus(&archived, "archived")
		noSKU := e.product("no active SKU", map[string]any{"status": "active"})
		col := acCreate(e, acRules("all", 7))
		acWantMembers(t, e, col, visible.ID)
		for _, p := range []ccProduct{draft, archived, noSKU} {
			if ccHas(e.list("q="+e.tag+"&collection="+col.Slug), p.ID) {
				t.Fatalf("ineligible product leaked: %s", p.ID)
			}
		}
		readCount := func() int {
			t.Helper()
			r := e.buyer("/v1/buyer/catalog/v2/collections")
			var out struct {
				Collections []struct {
					ID    string `json:"id"`
					Count int    `json:"product_count"`
				} `json:"collections"`
			}
			if r.status != 200 || json.Unmarshal(r.body, &out) != nil {
				t.Fatalf("buyer collection list: %d %s", r.status, r.body)
			}
			for _, c := range out.Collections {
				if c.ID == col.ID {
					return c.Count
				}
			}
			t.Fatal("automatic collection missing from buyer list")
			return -1
		}
		before := readCount()
		if before < 1 {
			t.Fatal("automatic count still reads the empty manual list")
		}
		// Activation must update both count and members without a collection write.
		e.activate(&draft)
		if got := readCount(); got != before+1 {
			t.Fatalf("count after activation = %d, want %d", got, before+1)
		}
		r := e.buyer("/v1/buyer/catalog/v2/products/" + visible.ID)
		if r.status != 200 || !bytes.Contains(r.body, []byte(col.Slug)) {
			t.Fatalf("product detail lost automatic collection: %d %s", r.status, r.body)
		}
		e.a.ok("PATCH", "/collections/"+col.ID, e.key("hide"), map[string]any{"expected_version": col.Version, "status": "hidden"}, &col)
		for _, path := range []string{"/v1/buyer/catalog/v2/collections/" + col.Slug, "/v1/buyer/catalog/v2/products?collection=" + col.Slug} {
			if r := e.buyer(path); r.status != 404 {
				t.Fatalf("hidden automatic collection: %s = %d", path, r.status)
			}
		}
		e.a.ok("PATCH", "/collections/"+col.ID, e.key("show"), map[string]any{"expected_version": col.Version, "status": "active"}, &col)
		mustExec(t, e.h.f.owner, `UPDATE control.storefront_publications SET published=false WHERE tenant_id=$1 AND store_id=$2`, e.h.f.tenantA, e.h.f.storeA1)
		t.Cleanup(func() {
			mustExec(t, e.h.f.owner, `UPDATE control.storefront_publications SET published=true WHERE tenant_id=$1 AND store_id=$2`, e.h.f.tenantA, e.h.f.storeA1)
		})
		for _, path := range []string{"/v1/buyer/catalog/v2/collections", "/v1/buyer/catalog/v2/collections/" + col.Slug, "/v1/buyer/catalog/v2/products?collection=" + col.Slug} {
			if r := e.buyer(path); r.status != 404 {
				t.Fatalf("unpublished automatic collection: %s = %d", path, r.status)
			}
		}
	})

	t.Run("mode_switch_preserves_manual_list_and_versions", func(t *testing.T) {
		e := ccNew(t)
		manual, _ := e.simple("manual", 100, 0, nil)
		recent, _ := e.simple("recent", 200, 0, nil)
		e.activate(&manual)
		e.activate(&recent)
		mustExec(t, e.h.f.owner, `UPDATE catalog.products SET created_at=clock_timestamp()-interval '30 days' WHERE id=$1`, manual.ID)
		col := e.collection("switch", nil, manual.ID)
		version := col.Version
		// Merely reading an editor draft cannot change the saved manual membership.
		e.a.ok("GET", "/collections/"+col.ID, "", nil, nil)
		acWantMembers(t, e, col, manual.ID)
		patch := map[string]any{"expected_version": version, "membership_mode": "automatic", "rules": acRules("all", 7), "sort_mode": "newest"}
		key := e.key("switch")
		s1, b1 := e.a.call("PATCH", "/collections/"+col.ID, key, patch)
		s2, b2 := e.a.call("PATCH", "/collections/"+col.ID, key, patch)
		if s1 != 200 || s2 != 200 || !bytes.Equal(b1, b2) {
			t.Fatalf("mode PATCH replay = %d/%d, identical=%v; first=%s", s1, s2, bytes.Equal(b1, b2), b1)
		}
		if err := json.Unmarshal(b1, &col); err != nil {
			t.Fatal(err)
		}
		if col.Version != version+1 {
			t.Fatalf("mode version = %d, want %d", col.Version, version+1)
		}
		acWantMembers(t, e, col, recent.ID)
		e.a.refuse(409, "PATCH", "/collections/"+col.ID, e.key("stale"), patch)
		e.a.refuse(409, "PUT", "/collections/"+col.ID+"/products", e.key("auto-members"), map[string]any{"expected_version": col.Version, "product_ids": []string{recent.ID}})
		e.a.ok("PATCH", "/collections/"+col.ID, e.key("manual"), map[string]any{"expected_version": col.Version, "membership_mode": "manual", "sort_mode": "manual"}, &col)
		acWantMembers(t, e, col, manual.ID)
		if col.Version != version+2 {
			t.Fatalf("manual version = %d", col.Version)
		}
	})

	t.Run("create_retry_scope_and_staff_permissions", func(t *testing.T) {
		e := ccNew(t)
		body := map[string]any{"title": e.name("scoped"), "membership_mode": "automatic", "sort_mode": "newest", "rules": acRules("all", 7), "tenant_id": e.h.f.tenantB, "store_id": e.h.f.storeB}
		key := e.key("create")
		s1, b1 := e.a.call("POST", "/collections", key, body)
		s2, b2 := e.a.call("POST", "/collections", key, body)
		if s1 != 200 || s2 != 200 || !bytes.Equal(b1, b2) {
			t.Fatalf("scoped automatic create/replay = %d/%d, identical=%v; first=%s", s1, s2, bytes.Equal(b1, b2), b1)
		}
		var col ccCollection
		if err := json.Unmarshal(b1, &col); err != nil {
			t.Fatal(err)
		}
		var tenant, store string
		if err := e.h.f.owner.QueryRow(context.Background(), `SELECT tenant_id::text,store_id::text FROM catalog.collections WHERE id=$1`, col.ID).Scan(&tenant, &store); err != nil {
			t.Fatal(err)
		}
		if tenant != e.h.f.tenantA || store != e.h.f.storeA1 {
			t.Fatalf("body selected the collection scope: %s/%s", tenant, store)
		}
		body["rules"] = acRules("all", 3)
		e.a.refuse(409, "POST", "/collections", key, body)
		patch := map[string]any{"expected_version": col.Version, "rules": acRules("any", 1, 7)}
		for _, other := range []ccAdmin{
			{t: t, h: e.srv, token: e.h.f.tokens["b"], store: e.h.f.storeB},
			{t: t, h: e.srv, token: e.h.f.tokens["a2"], store: e.h.f.storeA2},
		} {
			other.refuse(404, "GET", "/collections/"+col.ID, "", nil)
			other.refuse(404, "PATCH", "/collections/"+col.ID, e.key("foreign"), patch)
		}
		_, token := lcPrincipal(t, e.h.f, e.h.f.tenantA, []string{e.h.f.storeA1}, "store:read", "catalog:read")
		reader := ccAdmin{t: t, h: e.srv, token: token, store: e.h.f.storeA1}
		reader.refuse(403, "POST", "/collections", e.key("no-create"), body)
		reader.refuse(403, "PATCH", "/collections/"+col.ID, e.key("no-patch"), patch)
		// Buyer membership cannot use a tenant/store supplied in a header or unpublished origin.
		for _, origin := range []string{"https://unpublished.example.test"} {
			r := e.h.request(t, "GET", "/v1/buyer/catalog/v2/collections/"+col.Slug, "", "", nil, func(r *http.Request) { r.Header.Set("X-Commerce-Storefront-Origin", origin) })
			if r.status != 404 {
				t.Fatalf("foreign unpublished origin: %d", r.status)
			}
		}
	})

	t.Run("validation_is_atomic_and_unknown_rules_are_refused", func(t *testing.T) {
		e := ccNew(t)
		col := acCreate(e, acRules("all", 7))
		invalid := []any{
			acRules("none", 7), acRules("all"), acRules("all", 0), acRules("all", -1), acRules("all", 3651),
			map[string]any{"match": "all", "conditions": []any{map[string]any{"field": "tag", "operator": "equals", "value": "accessory"}}},
			map[string]any{"match": "all", "conditions": []any{map[string]any{"field": "source", "operator": "equals", "value": "live"}}},
			map[string]any{"match": "all", "conditions": []any{map[string]any{"field": "created_at", "operator": "within_days", "days": 1.5}}},
			map[string]any{"match": "all", "conditions": []any{map[string]any{"field": "created_at", "operator": "sql", "days": 7}}},
		}
		for _, rules := range invalid {
			e.a.refuse(422, "PATCH", "/collections/"+col.ID, e.key("invalid"), map[string]any{"expected_version": col.Version, "rules": rules})
		}
		e.a.refuse(422, "PATCH", "/collections/"+col.ID, e.key("mode"), map[string]any{"expected_version": col.Version, "membership_mode": "dynamic"})
		e.a.refuse(422, "POST", "/collections", e.key("missing"), map[string]any{"title": e.name("missing"), "membership_mode": "automatic"})
		var cur map[string]any
		e.a.ok("GET", "/collections/"+col.ID, "", nil, &cur)
		if cur["version"] != float64(col.Version) || cur["membership_mode"] != "automatic" {
			t.Fatalf("invalid rule request changed saved collection: %v", cur)
		}
		if got, _ := json.Marshal(cur["rules"]); string(got) != `{"conditions":[{"days":7,"field":"created_at","operator":"within_days"}],"match":"all"}` {
			t.Fatalf("rules did not round-trip after refusal: %s", got)
		}
	})
}
