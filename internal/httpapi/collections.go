package httpapi

// collections.go mounts the catalog-v2 merchant routes of unit catalog-core (contracts/storefront-v2.md section A):
// the product list/detail reads behind the admin Products pages, and collections (CRUD, ordered membership, one
// image). BFF: apps/admin/app/api/stores/[store]/[...resource]/route.ts (allowlisted exactly these paths) -> here.
// Domain: internal/catalog collections.go, productlist.go (all SQL and validation). This file owns transport only:
// query grammar, body bounds (bodyRoute / uploadRoute) and permissions. Scope comes from server auth (scoped); no
// tenant or store id is read from the request. Buyers never reach these routes (internal/buyerhttp catalogv2.go).

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/catalog"
	"livecommerce/internal/command"
	"livecommerce/internal/pagination"
	"livecommerce/internal/platform"
)

func registerCatalogV2Routes(mux *http.ServeMux, pool *pgxpool.Pool) {
	const base = "/v1/admin/stores/{store_id}"
	// Product list: search + status filter + stock summary, so it needs both read permissions like the ledger.
	mux.HandleFunc("GET "+base+"/catalog-products", scoped(pool, "catalog:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
		if err := platform.RequirePermission(ctx, tx, s, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), "inventory:read"); err != nil {
			return nil, err
		}
		values, err := singleValueQuery(r.URL.RawQuery)
		if err != nil {
			return nil, err
		}
		in := catalog.ProductListRequest{Query: values.Get("q"), Status: values.Get("status")}
		values.Del("q")
		values.Del("status")
		if in.Page, err = parsePage(values.Encode()); err != nil {
			return nil, err
		}
		return catalog.ListProductSummaries(ctx, tx, s, in)
	}))
	mux.HandleFunc("GET "+base+"/products/{product_id}", scoped(pool, "catalog:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
		if err := platform.RequirePermission(ctx, tx, s, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), "inventory:read"); err != nil {
			return nil, err
		}
		return catalog.GetProductDetail(ctx, tx, s, r.PathValue("product_id"))
	}))

	const collections = base + "/collections"
	mux.HandleFunc("GET "+collections, listRoute(pool, "catalog:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, page pagination.Request) (any, error) {
		return catalog.ListCollectionsPage(ctx, tx, s, page)
	}))
	mux.HandleFunc("POST "+collections, bodyRoute(pool, "catalog:write", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, in catalog.CollectionInput) (any, error) {
		return catalog.CreateCollection(ctx, tx, s, r.Header.Get("Idempotency-Key"), in)
	}))
	mux.HandleFunc("GET "+collections+"/{collection_id}", scoped(pool, "catalog:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
		return catalog.GetCollection(ctx, tx, s, r.PathValue("collection_id"))
	}))
	mux.HandleFunc("PATCH "+collections+"/{collection_id}", bodyRoute(pool, "catalog:write", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, in catalog.CollectionPatch) (any, error) {
		return catalog.PatchCollection(ctx, tx, s, r.Header.Get("Idempotency-Key"), r.PathValue("collection_id"), in)
	}))
	mux.HandleFunc("POST "+collections+"/{collection_id}/delete", bodyRoute(pool, "catalog:write", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, in versionInput) (any, error) {
		return catalog.DeleteCollection(ctx, tx, s, r.Header.Get("Idempotency-Key"), r.PathValue("collection_id"), in.ExpectedVersion)
	}))
	mux.HandleFunc("PUT "+collections+"/{collection_id}/products", bodyRoute(pool, "catalog:write", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, in catalog.CollectionProductsInput) (any, error) {
		return catalog.SetCollectionProducts(ctx, tx, s, r.Header.Get("Idempotency-Key"), r.PathValue("collection_id"), in)
	}))
	mux.HandleFunc("POST "+collections+"/{collection_id}/image", uploadRoute(pool, func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, data []byte) (any, error) {
		return catalog.UploadCollectionImage(ctx, tx, s, r.Header.Get("Idempotency-Key"), r.PathValue("collection_id"), data)
	}))
	mux.HandleFunc("GET "+collections+"/{collection_id}/image", scoped(pool, "catalog:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
		img, err := catalog.GetCollectionImage(ctx, tx, s, r.PathValue("collection_id"))
		if err != nil {
			return nil, err
		}
		return rawResponse{contentType: img.ContentType, body: img.Bytes}, nil
	}))
	mux.HandleFunc("POST "+collections+"/{collection_id}/image/delete", bodyRoute(pool, "catalog:write", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, _ struct{}) (any, error) {
		return catalog.DeleteCollectionImage(ctx, tx, s, r.Header.Get("Idempotency-Key"), r.PathValue("collection_id"))
	}))
}

// singleValueQuery parses a query string where every key may appear once (4 KiB cap), as the ledger route does.
func singleValueQuery(raw string) (url.Values, error) {
	if len(raw) > 4096 {
		return nil, command.ErrInvalid
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return nil, command.ErrInvalid
	}
	for _, v := range values {
		if len(v) != 1 {
			return nil, command.ErrInvalid
		}
	}
	return values, nil
}
