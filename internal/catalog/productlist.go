package catalog

// productlist.go owns the merchant product list and product detail reads behind the admin Products pages
// (docs/delivery/units/catalog-core.md): one row per product with search, status filter, cover thumbnail, active
// price range and stock summed over all warehouses; and one product with its SKUs and per-SKU stock. Read-only; the
// wide per-warehouse inventory view stays in ledger.go. Tables: catalog.products, catalog.skus,
// catalog.product_images, inventory.balances (commerce_runtime, scope_access). It never writes stock.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/command"
	"livecommerce/internal/pagination"
	"livecommerce/internal/platform"
)

// ProductSummary is one product-list row. PriceMin/PriceMax are over active SKUs (nil without one); Available is the
// sum over all warehouses of on_hand - reserved - allocated - unavailable of the active SKUs (the ledger's formula).
// Keyword is the keyword of the product's keyworded active SKU with the smallest id (empty when none; the full per-SKU
// keywords are in GetProductDetail). InventoryTracked is true only when every active SKU is inventory_tracked (A6);
// a product with no active SKU or a mix of tracked and untracked SKUs reads false. UpdatedAt is products.updated_at
// in the UTC microsecond form.
type ProductSummary struct {
	ID               string  `json:"id"`
	Slug             string  `json:"slug"`
	Name             string  `json:"name"`
	Status           string  `json:"status"`
	Version          int64   `json:"version"`
	CoverImageID     *string `json:"cover_image_id"`
	PriceMinMinor    *int64  `json:"price_min_minor"`
	PriceMaxMinor    *int64  `json:"price_max_minor"`
	Currency         string  `json:"currency"`
	SKUCount         int     `json:"sku_count"`
	Available        int64   `json:"available"`
	Keyword          string  `json:"keyword"`
	InventoryTracked bool    `json:"inventory_tracked"`
	UpdatedAt        string  `json:"updated_at"`
}

// StatusCounts is the per-status product tally that backs the list tabs (draft/active/archived), matching the current
// search but not the status filter.
type StatusCounts struct {
	Draft    int `json:"draft"`
	Active   int `json:"active"`
	Archived int `json:"archived"`
}

// ProductListResult is the product-list response: the page plus the store-wide tallies (product-editor §f list additions).
type ProductListResult struct {
	Items        []ProductSummary `json:"items"`
	NextCursor   string           `json:"next_cursor"`
	Total        int              `json:"total"`
	StatusCounts StatusCounts     `json:"status_counts"`
}

// ProductListRequest filters the list: Query is a literal case-insensitive match on name, slug or SKU code; Status is
// all (default), draft, active or archived.
type ProductListRequest struct {
	Page   pagination.Request
	Query  string
	Status string
}

// ListProductSummaries lists products newest first (keyset on created_at, id) and returns the store-wide tallies
// (Total, per-status counts) for the tabs. The tallies match the search but not the status filter, so the tabs stay
// stable while a tab is selected.
func ListProductSummaries(ctx context.Context, tx pgx.Tx, scope platform.Scope, in ProductListRequest) (ProductListResult, error) {
	out := ProductListResult{Items: make([]ProductSummary, 0)}
	status := strings.ToLower(strings.TrimSpace(in.Status))
	if status == "" {
		status = "all"
	}
	q := strings.TrimSpace(in.Query)
	if !validScope(tx, scope) || utf8Count(q) > 120 || strings.ContainsFunc(q, func(r rune) bool { return r < 0x20 || r == 0x7f }) ||
		(status != "all" && status != StatusDraft && status != StatusActive && status != StatusArchived) {
		return out, command.ErrInvalid
	}
	digest := sha256.Sum256([]byte(status + "\x00" + q))
	binding := pagination.Binding{TenantID: scope.TenantID, StoreID: scope.StoreID, Collection: "catalog-products", Filter: hex.EncodeToString(digest[:])}
	limit, after, err := pagination.Decode(in.Page, binding, 2)
	if err != nil {
		return out, err
	}
	// search is the shared name/slug/SKU-code predicate. The like argument is always $3 (right after tenant/store) in
	// both the tally and the page query, so the fragment is safe to splice into either.
	search := ""
	if q != "" {
		search = ` AND (p.name ILIKE '%' || $3 || '%' ESCAPE '!' OR p.slug ILIKE '%' || $3 || '%' ESCAPE '!'
			OR EXISTS (SELECT 1 FROM catalog.skus x WHERE x.tenant_id=p.tenant_id AND x.store_id=p.store_id AND x.product_id=p.id AND x.code ILIKE '%' || $3 || '%' ESCAPE '!'))`
	}
	tallyArgs := []any{scope.TenantID, scope.StoreID}
	if q != "" {
		tallyArgs = append(tallyArgs, escapeLike(q))
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FILTER (WHERE p.status='draft')::int,
			count(*) FILTER (WHERE p.status='active')::int,
			count(*) FILTER (WHERE p.status='archived')::int,
			count(*)::int
		FROM catalog.products p WHERE p.tenant_id=$1 AND p.store_id=$2`+search, tallyArgs...).
		Scan(&out.StatusCounts.Draft, &out.StatusCounts.Active, &out.StatusCounts.Archived, &out.Total); err != nil {
		return out, mapError(err)
	}
	args := []any{scope.TenantID, scope.StoreID}
	filters := ""
	if q != "" {
		args = append(args, escapeLike(q))
		filters += search
	}
	if status != "all" {
		args = append(args, status)
		filters += ` AND p.status=$` + placeholder(len(args))
	}
	if len(after) == 2 {
		args = append(args, after[0], after[1])
		filters += ` AND (p.created_at,p.id) < ($` + placeholder(len(args)-1) + `::timestamptz,$` + placeholder(len(args)) + `::uuid)`
	}
	args = append(args, limit+1)
	rows, err := tx.Query(ctx, `SELECT p.id::text,p.slug,p.name,p.status,p.version,
		(SELECT i.id::text FROM catalog.product_images i WHERE i.tenant_id=p.tenant_id AND i.store_id=p.store_id AND i.product_id=p.id ORDER BY i.position LIMIT 1),
		a.pmin,a.pmax,coalesce(a.currency,''),coalesce(a.n,0),coalesce(a.avail,0),
		coalesce(k.keyword,''),coalesce(a.tracked,false),
		to_char(p.updated_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
		to_char(p.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"')
		FROM catalog.products p
		LEFT JOIN LATERAL (SELECT min(s.price_minor) pmin,max(s.price_minor) pmax,min(s.currency) currency,count(*)::int n,
				coalesce(sum(b.avail),0)::bigint avail,bool_and(s.inventory_tracked) tracked
			FROM catalog.skus s
			LEFT JOIN LATERAL (SELECT sum(x.on_hand-x.reserved-x.allocated-x.unavailable) avail FROM inventory.balances x
				WHERE x.tenant_id=s.tenant_id AND x.store_id=s.store_id AND x.sku_id=s.id) b ON true
			WHERE s.tenant_id=p.tenant_id AND s.store_id=p.store_id AND s.product_id=p.id AND s.status='active') a ON true
		LEFT JOIN LATERAL (SELECT l.keyword FROM catalog.skus s
			JOIN live.keyword_library l ON l.tenant_id=s.tenant_id AND l.store_id=s.store_id AND l.sku_id=s.id
			WHERE s.tenant_id=p.tenant_id AND s.store_id=p.store_id AND s.product_id=p.id AND s.status='active'
			ORDER BY s.id LIMIT 1) k ON true
		WHERE p.tenant_id=$1 AND p.store_id=$2`+filters+` ORDER BY p.created_at DESC,p.id DESC LIMIT $`+placeholder(len(args)), args...)
	if err != nil {
		return out, mapError(err)
	}
	defer rows.Close()
	stamps := make([]string, 0, limit+1)
	for rows.Next() {
		var s ProductSummary
		var stamp string
		if err := rows.Scan(&s.ID, &s.Slug, &s.Name, &s.Status, &s.Version, &s.CoverImageID, &s.PriceMinMinor, &s.PriceMaxMinor, &s.Currency, &s.SKUCount, &s.Available, &s.Keyword, &s.InventoryTracked, &s.UpdatedAt, &stamp); err != nil {
			return out, err
		}
		out.Items = append(out.Items, s)
		stamps = append(stamps, stamp)
	}
	if err := rows.Err(); err != nil {
		return out, mapError(err)
	}
	if len(out.Items) <= limit {
		return out, nil
	}
	out.Items = out.Items[:limit]
	out.NextCursor, err = pagination.Encode(binding, []string{stamps[limit-1], out.Items[limit-1].ID})
	return out, err
}

// ProductDetail is the editor's read: the product, its photos' ids come from ListImages, and every non-archived SKU
// with its stock summed over all warehouses.
type ProductDetail struct {
	Product
	SKUs []SKUStock `json:"skus"`
}

// SKUStock is a SKU plus its all-warehouse available quantity.
type SKUStock struct {
	SKU
	Available int64 `json:"available"`
}

// GetProductDetail reads a product (any status) with its active SKUs (<= 100, maxActiveSKUsPerProduct).
func GetProductDetail(ctx context.Context, tx pgx.Tx, scope platform.Scope, id string) (ProductDetail, error) {
	d := ProductDetail{SKUs: []SKUStock{}}
	var err error
	if d.Product, err = GetProduct(ctx, tx, scope, id); err != nil {
		return d, err
	}
	rows, err := tx.Query(ctx, `SELECT `+prefixed("s", skuColumns)+`,coalesce(b.avail,0)::bigint FROM catalog.skus s
		LEFT JOIN LATERAL (SELECT sum(x.on_hand-x.reserved-x.allocated-x.unavailable) avail FROM inventory.balances x
			WHERE x.tenant_id=s.tenant_id AND x.store_id=s.store_id AND x.sku_id=s.id) b ON true
		WHERE s.tenant_id=$1 AND s.store_id=$2 AND s.product_id=$3 AND s.status='active' ORDER BY s.created_at,s.id LIMIT 200`, scope.TenantID, scope.StoreID, id)
	if err != nil {
		return d, mapError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var s SKUStock
		if err := rows.Scan(append(skuFields(&s.SKU), &s.Available)...); err != nil {
			return d, err
		}
		finishSKU(&s.SKU)
		d.SKUs = append(d.SKUs, s)
	}
	return d, mapError(rows.Err())
}

// prefixed qualifies each column of a comma list with a table alias (skuColumns uses no function-call commas).
func prefixed(alias, cols string) string {
	parts := strings.Split(cols, ",")
	for i, c := range parts {
		parts[i] = alias + "." + c
	}
	return strings.Join(parts, ",")
}
