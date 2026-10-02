package foundation_test

// S1 REAL_PG regressions use authentic merchant auth/command routes; only the old-image fixture deletes child rows as owner.
import (
	"bytes"
	"context"
	"fmt"
	"image"
	"testing"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/platform"
)

func TestMediaSizesLifecycleAndIsolation(t *testing.T) {
	h, m, product := cmiEnv(t)
	f := h.f
	ctx := context.Background()
	original := cmiJPEG(t, 1440, 1800, 31)
	key := t04Key("sizes-upload")
	first := m.upload(product, key, cmiFile("photo.jpg", "image/jpeg", original))
	if first.Code != 200 {
		t.Fatalf("upload %d %s", first.Code, first.Body.String())
	}
	second := m.upload(product, key, cmiFile("photo.jpg", "image/jpeg", original))
	if second.Code != 200 || !bytes.Equal(first.Body.Bytes(), second.Body.Bytes()) {
		t.Fatal("upload idempotency changed")
	}
	img := m.list(product)[0]
	if n := countRows(t, f.owner, `SELECT count(*) FROM catalog.product_image_sizes WHERE image_id=$1`, img.ID); n != 3 {
		t.Fatalf("sizes=%d", n)
	}
	for _, width := range []int{360, 720, 1080} {
		r := cmiGetMedia(t, h, h.origin, product, fmt.Sprintf("%s?w=%d", img.ID, width), nil)
		cfg, format, err := image.DecodeConfig(bytes.NewReader(r.body))
		if r.status != 200 || err != nil || format != "jpeg" || cfg.Width != width || cfg.Height != width*5/4 {
			t.Fatalf("width %d: status=%d cfg=%+v format=%s err=%v", width, r.status, cfg, format, err)
		}
		if r.header.Get("X-Commerce-Image-Rendition") != "1" || r.header.Get("Cache-Control") != "public, max-age=86400, immutable" {
			t.Fatal("committed rendition cache policy")
		}
	}
	if r := cmiGetMedia(t, h, h.origin, product, img.ID, nil); r.status != 200 || !bytes.Equal(r.body, original) {
		t.Fatal("original changed")
	}
	for _, q := range []string{"w=0", "w=640", "w=360&w=720", "w=360&store_id=x", "w=%33%36%30"} {
		if r := cmiGetMedia(t, h, h.origin, product, img.ID+"?"+q, nil); r.status != 403 {
			t.Fatalf("invalid %s status %d", q, r.status)
		}
	}
	foreign := t04CreateStock(t, f, f.tokens["b"], f.storeB, 2).product.ID
	cmiCleanup(t, f, foreign)
	fm := cmiMerchant{t: t, h: m.h, token: f.tokens["b"], store: f.storeB}
	fi := fm.mustUpload(foreign, cmiPNG(t, 40, 30, 2), "x.png", "image/png")
	small := m.mustUpload(product, v2PNG(t, 3), "small.png", "image/png")
	var metadataCount, pixelWidth int
	if err := h.a.runtime.QueryRow(ctx, `SELECT count(*) FROM catalog.buyer_image_sizes($1,$2::uuid[])`, h.origin, []string{img.ID, small.ID, fi.ID}).Scan(&metadataCount); err != nil || metadataCount != 6 {
		t.Fatalf("scoped metadata includes only own images: count=%d err=%v", metadataCount, err)
	}
	if err := h.a.runtime.QueryRow(ctx, `SELECT pixel_width FROM catalog.buyer_image_sizes($1,$2::uuid[]) WHERE width=720`, h.origin, []string{small.ID}).Scan(&pixelWidth); err != nil || pixelWidth != 3 {
		t.Fatalf("no-upscale actual descriptor=%d err=%v", pixelWidth, err)
	}
	for _, route := range []string{"/v1/buyer/catalog/v2/products", "/v1/buyer/catalog/v2/products/" + product} {
		response := h.request(t, "GET", route, "", "", nil, nil)
		if response.status != 200 || !bytes.Contains(response.body, []byte(`"pixel_width":360`)) {
			t.Fatalf("catalog rendition metadata missing: status=%d body=%s", response.status, response.body)
		}
	}
	for _, width := range []int{360, 720, 1080} {
		for _, pair := range [][2]string{{foreign, fi.ID}, {product, fi.ID}, {foreign, img.ID}} {
			if r := cmiGetMedia(t, h, h.origin, pair[0], fmt.Sprintf("%s?w=%d", pair[1], width), nil); r.status != 404 {
				t.Fatalf("cross-store variant=%d", r.status)
			}
		}
	}
	err := platform.WithScope(ctx, f.runtime, f.tokens["a"], f.storeA1, "catalog:read", func(tx pgx.Tx, _ platform.Scope) error {
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM catalog.product_image_sizes WHERE image_id=$1`, fi.ID).Scan(&n); err != nil {
			return err
		}
		if n != 0 {
			t.Fatal("foreign child exposed by RLS")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"commerce_buyer_runtime", "commerce_catalog_media"} {
		var can bool
		if err := f.owner.QueryRow(ctx, `SELECT has_table_privilege($1,'catalog.product_image_sizes','INSERT')`, role).Scan(&can); err != nil || can {
			t.Fatalf("public role can insert: %s %v", role, err)
		}
	}
	// Old-image fixture: source retained, no automatic public write. Then explicit repeat-safe backfill.
	mustExec(t, f.owner, `DELETE FROM catalog.product_image_sizes WHERE image_id=$1`, img.ID)
	if err := h.a.runtime.QueryRow(ctx, `SELECT count(*) FROM catalog.buyer_image_sizes($1,$2::uuid[])`, h.origin, []string{img.ID}).Scan(&metadataCount); err != nil || metadataCount != 0 {
		t.Fatal("historical fallback must not advertise guessed srcset dimensions")
	}
	fallback := cmiGetMedia(t, h, h.origin, product, img.ID+"?w=360", nil)
	if fallback.status != 200 || !bytes.Equal(fallback.body, original) || fallback.header.Get("Cache-Control") != "no-store" {
		t.Fatal("old-image fallback must be uncached original")
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM catalog.product_image_sizes WHERE image_id=$1`, img.ID); n != 0 {
		t.Fatal("public GET created sizes")
	}
	headers := map[string]string{"Content-Type": "application/json", "Idempotency-Key": t04Key("sizes-backfill")}
	path := m.imagesPath(product) + "/" + img.ID + "/renditions"
	if r := m.raw("POST", path, map[string]string{"Content-Type": "application/json"}, []byte(`{}`)); r.Code != 422 {
		t.Fatalf("missing backfill key %d", r.Code)
	}
	unauth := cmiMerchant{t: t, h: m.h, store: m.store}
	if r := unauth.raw("POST", path, headers, []byte(`{}`)); r.Code != 401 {
		t.Fatalf("unauth backfill %d", r.Code)
	}
	if r := m.raw("POST", m.imagesPath(foreign)+"/"+fi.ID+"/renditions", headers, []byte(`{}`)); r.Code != 404 {
		t.Fatalf("foreign backfill %d", r.Code)
	}
	a := m.raw("POST", path, headers, []byte(`{}`))
	b := m.raw("POST", path, headers, []byte(`{}`))
	if a.Code != 200 || b.Code != 200 || !bytes.Equal(a.Body.Bytes(), b.Body.Bytes()) {
		t.Fatalf("backfill retry %d/%d %s", a.Code, b.Code, a.Body.String())
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM catalog.product_image_sizes WHERE image_id=$1`, img.ID); n != 3 {
		t.Fatalf("backfill sizes %d", n)
	}
	var forced, enabled bool
	if err := f.owner.QueryRow(ctx, `SELECT relforcerowsecurity,relrowsecurity FROM pg_class WHERE oid='catalog.product_image_sizes'::regclass`).Scan(&forced, &enabled); err != nil || !forced || !enabled {
		t.Fatalf("RLS flags %v/%v %v", forced, enabled, err)
	}
	if r := cmiGetMedia(t, h, h.origin, product, img.ID+"?w=360", nil); r.status != 200 || r.header.Get("X-Commerce-Image-Rendition") != "1" {
		t.Fatal("backfill not served")
	}
	for _, state := range []string{"draft", "archived"} {
		mustExec(t, f.owner, `UPDATE catalog.products SET status=$2 WHERE id=$1`, product, state)
		if r := cmiGetMedia(t, h, h.origin, product, img.ID+"?w=360", nil); r.status != 404 {
			t.Fatalf("%s rendition status %d", state, r.status)
		}
	}
	mustExec(t, f.owner, `UPDATE catalog.products SET status='active' WHERE id=$1`, product)
	mustExec(t, f.owner, `UPDATE control.storefront_publications SET published=false WHERE store_id=$1`, f.storeA1)
	closed := cmiGetMedia(t, h, h.origin, product, img.ID+"?w=360", nil)
	mustExec(t, f.owner, `UPDATE control.storefront_publications SET published=true WHERE store_id=$1`, f.storeA1)
	if closed.status != 404 {
		t.Fatalf("unpublished rendition status %d", closed.status)
	}
	if r := m.remove(product, img.ID, t04Key("sizes-delete")); r.Code != 200 {
		t.Fatalf("delete %d", r.Code)
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM catalog.product_image_sizes WHERE image_id=$1`, img.ID); n != 0 {
		t.Fatal("orphaned sizes")
	}
	if r := cmiGetMedia(t, h, h.origin, product, img.ID+"?w=360", nil); r.status != 404 {
		t.Fatal("deleted rendition served")
	}
}
