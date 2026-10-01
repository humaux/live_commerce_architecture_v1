package foundation_test

// catalog_core_extra_test.go: CC04b / CC10b / CC12a of unit catalog-core (contracts/storefront-v2.md section A, "Implemented shapes and
// acceptance list"): SKU PATCH with option values, the publication gate on every v2 read, and the merchant product list the admin page reads.

import (
	"net/url"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// CC04b: SKU PATCH accepts option_values (contract A): aligned, unique among active SKUs, absent keeps, derived title follows.
func TestCatalogCoreCC04bSKUPatchOptionValues(t *testing.T) {
	e := ccNew(t)
	p := e.product("Patchy", nil)
	e.a.ok("PATCH", "/products/"+p.ID, e.key("ax"), map[string]any{"expected_version": p.Version, "options": []map[string]any{
		{"name": "Color", "values": []string{"Red", "Blue"}}, {"name": "Size", "values": []string{"S", "M"}}}}, &p)
	red := e.sku(p.ID, "RS", 1000, []string{"Red", "S"})
	blue := e.sku(p.ID, "BS", 1000, []string{"Blue", "S"})
	patch := func(want int, s ccSKU, values any) ccSKU {
		t.Helper()
		body := map[string]any{"product_id": p.ID, "code": s.Code, "price_minor": s.PriceMinor, "expected_version": s.Version}
		if values != nil {
			body["option_values"] = values
		}
		var out ccSKU
		if want == 200 {
			e.a.ok("PATCH", "/skus/"+s.ID, e.key("up"), body, &out)
		} else {
			e.a.refuse(want, "PATCH", "/skus/"+s.ID, e.key("up"), body)
		}
		return out
	}
	patch(409, red, []string{"Blue", "S"})  // the combination of another active variant
	patch(422, red, []string{"Blue"})       // wrong arity
	patch(422, red, []string{"Green", "S"}) // not on the axis
	patch(422, red, []string{"S", "Blue"})  // right values, wrong axes
	kept := patch(200, red, nil)            // absent keeps
	if !reflect.DeepEqual(kept.OptionValues, []string{"Red", "S"}) || kept.Title != "Red / S" {
		t.Fatalf("an absent option_values must keep the stored ones: %+v", kept)
	}
	moved := patch(200, kept, []string{"Red", "M"}) // a free combination
	if !reflect.DeepEqual(moved.OptionValues, []string{"Red", "M"}) || moved.Title != "Red / M" {
		t.Fatalf("the derived title follows the values: %+v", moved)
	}
	// the freed combination can be taken again; the moved one is taken now
	e.sku(p.ID, "RS2", 1100, []string{"Red", "S"})
	e.a.refuse(409, "POST", "/skus", e.key("sku"), map[string]any{"product_id": p.ID, "code": strings.ToUpper(e.tag) + "-RM2", "price_minor": 1, "option_values": []string{"Red", "M"}})
	_ = blue
	// a stale version is 409
	e.a.refuse(409, "PATCH", "/skus/"+red.ID, e.key("up"), map[string]any{"product_id": p.ID, "code": red.Code, "price_minor": red.PriceMinor, "expected_version": red.Version})
	// the buyer sees the moved values
	e.stock(red.ID, e.wh1, 3)
	e.activate(&p)
	d, _ := e.detail(p.ID)
	titles := []string{}
	for _, v := range d.Variants {
		titles = append(titles, v.Title)
	}
	slices.Sort(titles)
	if !slices.Equal(titles, []string{"Blue / S", "Red / M", "Red / S"}) {
		t.Fatalf("buyer variants after the patches: %v", titles)
	}
}

// CC10b: an unpublished store serves nothing through any v2 read (the store comes from the verified origin, never from a parameter).
func TestCatalogCoreCC10bUnpublishedStore(t *testing.T) {
	e := ccNew(t)
	p, _ := e.simple("Shown", 500, 5, nil)
	e.activate(&p)
	col := e.collection("Shown", nil, p.ID)
	if !ccHas(e.list("q="+e.tag), p.ID) {
		t.Fatal("precondition: the product is listed while the store is published")
	}
	mustExec(t, e.h.f.owner, `UPDATE control.storefront_publications SET published=false WHERE tenant_id=$1 AND store_id=$2`, e.h.f.tenantA, e.h.f.storeA1)
	for _, path := range []string{
		"/v1/buyer/catalog/v2/products?q=" + e.tag,
		"/v1/buyer/catalog/v2/products/" + p.ID,
		"/v1/buyer/catalog/v2/products/" + p.Slug,
		"/v1/buyer/catalog/v2/collections",
		"/v1/buyer/catalog/v2/collections/" + col.Slug,
	} {
		if r := e.buyer(path); r.status != 404 {
			t.Fatalf("%s on an unpublished store must be 404, got %d %s", path, r.status, r.body)
		}
	}
	// the merchant still works on it (publication gates shoppers, not the merchant)
	var cur ccProduct
	e.a.ok("GET", "/products/"+p.ID, "", nil, &cur)
	if cur.ID != p.ID {
		t.Fatal("merchant read of an unpublished store's product")
	}
	mustExec(t, e.h.f.owner, `UPDATE control.storefront_publications SET published=true WHERE tenant_id=$1 AND store_id=$2`, e.h.f.tenantA, e.h.f.storeA1)
	if !ccHas(e.list("q="+e.tag), p.ID) {
		t.Fatal("re-published: the product is back")
	}
}

// CC12a: the merchant product list behind the admin page: cards with cover, price range, variant count and available stock; status and
// text filters; a cursor that walks every row exactly once.
func TestCatalogCoreCC12aAdminList(t *testing.T) {
	e := ccNew(t)
	multi := e.product("Zed multi", nil)
	e.a.ok("PATCH", "/products/"+multi.ID, e.key("ax"), map[string]any{"expected_version": multi.Version, "options": []map[string]any{{"name": "Size", "values": []string{"S", "M"}}}}, &multi)
	s1 := e.sku(multi.ID, "ZS", 700, []string{"S"})
	s2 := e.sku(multi.ID, "ZM", 900, []string{"M"})
	e.stock(s1.ID, e.wh1, 4)
	e.stock(s2.ID, e.wh2, 5)
	e.setStatus(&multi, "active")
	photo, photoType := ccMultipart(ccPNG(3), "z.png")
	status, body, _ := e.a.raw("POST", "/products/"+multi.ID+"/images", e.key("img"), photo, photoType, nil)
	if status != 200 {
		t.Fatalf("photo: %d %s", status, body)
	}
	draft := e.product("Alpha draft", nil)
	arch, _ := e.simple("Beta archived", 100, 0, nil)
	e.setStatus(&arch, "archived")
	empty := e.product("Gamma nosku", nil)

	type card struct {
		ID       string  `json:"id"`
		Slug     string  `json:"slug"`
		Name     string  `json:"name"`
		Status   string  `json:"status"`
		Cover    *string `json:"cover_image_id"`
		Min      *int64  `json:"price_min_minor"`
		Max      *int64  `json:"price_max_minor"`
		SKUCount int     `json:"sku_count"`
		Avail    int64   `json:"available"`
	}
	type page struct {
		Items []card `json:"items"`
		Next  string `json:"next_cursor"`
	}
	read := func(q string) page {
		t.Helper()
		var pg page
		e.a.ok("GET", "/catalog-products?"+q, "", nil, &pg)
		return pg
	}
	all := read("q=" + url.QueryEscape(e.tag))
	by := map[string]card{}
	for _, c := range all.Items {
		by[c.ID] = c
	}
	if len(all.Items) != 4 {
		t.Fatalf("the merchant list shows every status (4 products): %d", len(all.Items))
	}
	m := by[multi.ID]
	if m.Status != "active" || m.Cover == nil || m.Min == nil || *m.Min != 700 || m.Max == nil || *m.Max != 900 || m.SKUCount != 2 || m.Avail != 9 || m.Slug != multi.Slug || m.Name != multi.Name {
		t.Fatalf("multi-variant card: %+v", m)
	}
	if c := by[empty.ID]; c.Min != nil || c.Max != nil || c.SKUCount != 0 || c.Avail != 0 || c.Status != "draft" {
		t.Fatalf("a product without variants has no price range: %+v", c)
	}
	// filters
	only := func(pg page, ids ...string) {
		t.Helper()
		got := []string{}
		for _, c := range pg.Items {
			got = append(got, c.ID)
		}
		slices.Sort(got)
		w := slices.Clone(ids)
		slices.Sort(w)
		if !slices.Equal(got, w) {
			t.Fatalf("got %v want %v", got, w)
		}
	}
	only(read("status=draft&q="+e.tag), draft.ID, empty.ID)
	only(read("status=active&q="+e.tag), multi.ID)
	only(read("status=archived&q="+e.tag), arch.ID)
	only(read("q="+url.QueryEscape(strings.ToUpper(e.tag)+" ZED")), multi.ID) // name, case-insensitive
	only(read("q="+url.QueryEscape(strings.ToLower(s2.Code))), multi.ID)      // SKU code
	only(read("q="+url.QueryEscape(multi.Slug)), multi.ID)                    // handle (the admin search box promises name, handle or SKU code)
	if st, _ := e.a.call("GET", "/catalog-products?status=bogus", "", nil); st != 422 {
		t.Fatalf("unknown status filter: %d", st)
	}
	// a cursor walks every row exactly once, newest first or by name (whatever the order, no gaps and no repeats)
	seen := map[string]int{}
	pg := read("q=" + url.QueryEscape(e.tag) + "&limit=1")
	for i := 0; i < 10; i++ {
		for _, c := range pg.Items {
			seen[c.ID]++
		}
		if pg.Next == "" {
			break
		}
		pg = read("q=" + url.QueryEscape(e.tag) + "&limit=1&cursor=" + url.QueryEscape(pg.Next))
	}
	if len(seen) != 4 {
		t.Fatalf("cursor walk saw %d distinct products, want 4: %v", len(seen), seen)
	}
	for id, n := range seen {
		if n != 1 {
			t.Fatalf("product %s seen %d times across pages", id, n)
		}
	}
	// the detail the editor reads: product + skus[] with derived title, option values and available stock
	var detail struct {
		ccProduct
		SKUs []struct {
			ccSKU
			Available int64 `json:"available"`
		} `json:"skus"`
	}
	e.a.ok("GET", "/products/"+multi.ID, "", nil, &detail)
	if detail.ID != multi.ID || len(detail.SKUs) != 2 || len(detail.Options) != 1 {
		t.Fatalf("editor detail: %+v", detail)
	}
	avail := map[string]int64{}
	for _, s := range detail.SKUs {
		avail[s.Title] = s.Available
	}
	if avail["S"] != 4 || avail["M"] != 5 {
		t.Fatalf("per-variant available over all warehouses: %v", avail)
	}
}

// CC12b: the merchant search box is a literal search too: % and _ are not wildcards (same rule as the buyer q), a backslash is not an escape.
func TestCatalogCoreCC12bAdminSearchLiteral(t *testing.T) {
	e := ccNew(t)
	pct := e.product("100% pure", nil)
	other := e.product("100x pure", nil)
	us := e.product("a_b set", nil)
	wild := e.product("aXb set", nil)
	bs := e.product(`back\slash`, nil)
	type pg struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	ids := func(q string) []string {
		t.Helper()
		var out pg
		e.a.ok("GET", "/catalog-products?q="+url.QueryEscape(q), "", nil, &out)
		got := []string{}
		for _, it := range out.Items {
			got = append(got, it.ID)
		}
		slices.Sort(got)
		return got
	}
	want := func(q string, products ...ccProduct) {
		t.Helper()
		w := []string{}
		for _, p := range products {
			w = append(w, p.ID)
		}
		slices.Sort(w)
		if got := ids(q); !slices.Equal(got, w) {
			t.Fatalf("admin search %q: got %v want %v", q, got, w)
		}
	}
	want(e.tag+" 100%", pct)
	want(e.tag+" a_b", us)
	want(e.tag+` back\`, bs)
	want(e.tag + " %")
	want(e.tag + " _")
	_, _ = other, wild
}
