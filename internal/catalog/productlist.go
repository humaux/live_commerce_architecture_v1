package catalog

// productlist.go owns the merchant product list and product detail reads behind the admin Products pages
// (docs/delivery/units/catalog-core.md): one row per product with search, status filter, cover thumbnail, active
// price range and stock summed over all warehouses; and one product with its SKUs, per-SKU stock/keyword, collection ids
// and the single-warehouse on_hand/committed (product-editor §g.4). Read-only; the wide per-warehouse inventory view stays
// in ledger.go. Tables: catalog.products, catalog.skus, catalog.collection_products, catalog.product_images,
// inventory.balances, live.keyword_library (commerce_runtime, scope_access). It never writes stock.

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
	CoverImageURL    *string `json:"cover_image_url"`
	PriceMinMinor    *int64  `json:"price_min_minor"`
	PriceMaxMinor    *int64  `json:"price_max_minor"`
	Currency         string  `json:"currency"`
	SKUCount         int     `json:"sku_count"`
	Available        int64   `json:"available"`
	Keyword          string  `json:"keyword"`
	InventoryTracked bool    `json:"inventory_tracked"`
	UpdatedAt        string  `json:"updated_at"`
}

// StatusCounts backs the list tabs for the current search and stock filters across all statuses.
type StatusCounts struct {
	Draft    int `json:"draft"`
	Active   int `json:"active"`
	Archived int `json:"archived"`
}

// ProductListResult is the page and its filter-matched tallies.
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
	Stock  string
}

const productSKUStock = `LEFT JOIN LATERAL (
	SELECT min(s.price_minor) pmin,max(s.price_minor) pmax,min(s.currency) currency,count(*)::int n,
		coalesce(sum(b.avail),0)::bigint avail,coalesce(bool_and(s.inventory_tracked),false) tracked
	FROM catalog.skus s
	LEFT JOIN LATERAL (
		SELECT sum(x.on_hand-x.reserved-x.allocated-x.unavailable) avail FROM inventory.balances x
		WHERE x.tenant_id=s.tenant_id AND x.store_id=s.store_id AND x.sku_id=s.id
	) b ON true
	WHERE s.tenant_id=p.tenant_id AND s.store_id=p.store_id AND s.product_id=p.id AND s.status='active'
) a ON true`

// ListProductSummaries reads the page and all status tallies in one PostgreSQL statement snapshot.
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
	filterKey := status + "\x00" + q
	stockPredicate := ""
	switch in.Stock {
	case "":
	case "available":
		stockPredicate = " AND a.tracked AND a.avail>0"
	case "sold_out":
		stockPredicate = " AND a.tracked AND a.avail=0"
	case "untracked":
		stockPredicate = " AND NOT a.tracked"
	default:
		return out, command.ErrInvalid
	}
	if in.Stock != "" {
		filterKey += "\x00stock\x00" + in.Stock
	}
	digest := sha256.Sum256([]byte(filterKey))
	binding := pagination.Binding{TenantID: scope.TenantID, StoreID: scope.StoreID, Collection: "catalog-products", Filter: hex.EncodeToString(digest[:])}
	limit, after, err := pagination.Decode(in.Page, binding, 2)
	if err != nil {
		return out, err
	}
	args := []any{scope.TenantID, scope.StoreID}
	search := ""
	if q != "" {
		args = append(args, escapeLike(q))
		search = ` AND (p.name ILIKE '%' || $3 || '%' ESCAPE '!' OR p.slug ILIKE '%' || $3 || '%' ESCAPE '!'
			OR EXISTS (SELECT 1 FROM catalog.skus x WHERE x.tenant_id=p.tenant_id AND x.store_id=p.store_id AND x.product_id=p.id AND x.code ILIKE '%' || $3 || '%' ESCAPE '!'))`
	}
	// Keep the existing keyset index path without stock, and share the stock eligibility work when filtering.
	materialized, stockJoin := "NOT MATERIALIZED", ""
	if stockPredicate != "" {
		materialized, stockJoin = "MATERIALIZED", productSKUStock
	}
	filters := ""
	if status != "all" {
		args = append(args, status)
		filters += ` AND p.status=$` + placeholder(len(args))
	}
	if len(after) == 2 {
		args = append(args, after[0], after[1])
		filters += ` AND (p.created_at,p.id) < ($` + placeholder(len(args)-1) + `::timestamptz,$` + placeholder(len(args)) + `::uuid)`
	}
	args = append(args, limit+1)
	var page []struct {
		ProductSummary
		CreatedAt string `json:"created_at"`
	}
	if err := tx.QueryRow(ctx, `WITH eligible AS `+materialized+` (
		SELECT p.id,p.status,p.created_at FROM catalog.products p `+stockJoin+`
		WHERE p.tenant_id=$1 AND p.store_id=$2`+search+stockPredicate+`
	), page AS (
		SELECT p.id FROM eligible p WHERE true`+filters+`
		ORDER BY p.created_at DESC,p.id DESC LIMIT $`+placeholder(len(args))+`
	), summaries AS (
		SELECT p.id,p.slug,p.name,p.status,p.version,
			(SELECT i.id FROM catalog.product_images i WHERE i.tenant_id=p.tenant_id AND i.store_id=p.store_id AND i.product_id=p.id AND i.role='main' ORDER BY i.position LIMIT 1) AS cover_image_id,
			a.pmin AS price_min_minor,a.pmax AS price_max_minor,coalesce(a.currency,'') AS currency,
			a.n AS sku_count,a.avail AS available,coalesce(k.keyword,'') AS keyword,a.tracked AS inventory_tracked,
			to_char(p.updated_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"') AS updated_at,
			to_char(p.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"') AS created_at
		FROM page JOIN catalog.products p ON p.tenant_id=$1 AND p.store_id=$2 AND p.id=page.id
		`+productSKUStock+`
		LEFT JOIN LATERAL (SELECT l.keyword FROM catalog.skus s
			JOIN live.keyword_library l ON l.tenant_id=s.tenant_id AND l.store_id=s.store_id AND l.sku_id=s.id
			WHERE s.tenant_id=p.tenant_id AND s.store_id=p.store_id AND s.product_id=p.id AND s.status='active'
			ORDER BY s.id LIMIT 1) k ON true
	)
	SELECT count(*) FILTER (WHERE status='draft')::int,
		count(*) FILTER (WHERE status='active')::int,
		count(*) FILTER (WHERE status='archived')::int,
		count(*)::int,
		(SELECT coalesce(jsonb_agg(s ORDER BY s.created_at COLLATE "C" DESC,s.id DESC),'[]'::jsonb) FROM summaries s)
	FROM eligible`, args...).Scan(&out.StatusCounts.Draft, &out.StatusCounts.Active, &out.StatusCounts.Archived, &out.Total, &page); err != nil {
		return out, mapError(err)
	}
	if len(page) > limit {
		page = page[:limit]
		last := page[limit-1]
		out.NextCursor, err = pagination.Encode(binding, []string{last.CreatedAt, last.ID})
	}
	for _, row := range page {
		s := row.ProductSummary
		if s.CoverImageID != nil {
			url := "/api/stores/" + scope.StoreID + "/products/" + s.ID + "/images/" + *s.CoverImageID + "?w=360"
			s.CoverImageURL = &url
		}
		out.Items = append(out.Items, s)
	}
	return out, err
}

// ProductDetail is the editor's read (product-editor §g.4): the product, every collection id it belongs to (all of them,
// no pagination), the store's single active warehouse (null when 0 or >1), and every active SKU with its all-warehouse
// available, its keyword ("" when none) and — only when warehouse_id is non-null — its on_hand and committed
// (reserved+allocated+unavailable) taken from that warehouse (null when no balance row).
type ProductDetail struct {
	Product
	CollectionIDs []string   `json:"collection_ids"`
	WarehouseID   *string    `json:"warehouse_id"`
	SKUs          []SKUStock `json:"skus"`
}

// SKUStock is a SKU plus its all-warehouse available quantity, its keyword, and its single-warehouse on_hand/committed.
type SKUStock struct {
	SKU
	Available int64  `json:"available"`
	Keyword   string `json:"keyword"`
	OnHand    *int64 `json:"on_hand"`
	Committed *int64 `json:"committed"`
}

// GetProductDetail reads a product (any status) with its active SKUs (<= 100, maxActiveSKUsPerProduct).
func GetProductDetail(ctx context.Context, tx pgx.Tx, scope platform.Scope, id string) (ProductDetail, error) {
	d := ProductDetail{SKUs: []SKUStock{}, CollectionIDs: []string{}}
	var err error
	if d.Product, err = GetProduct(ctx, tx, scope, id); err != nil {
		return d, err
	}
	crows, err := tx.Query(ctx, `SELECT collection_id::text FROM catalog.collection_products WHERE tenant_id=$1 AND store_id=$2 AND product_id=$3 ORDER BY position,collection_id`,
		scope.TenantID, scope.StoreID, id)
	if err != nil {
		return d, mapError(err)
	}
	for crows.Next() {
		var cid string
		if err := crows.Scan(&cid); err != nil {
			crows.Close()
			return d, err
		}
		d.CollectionIDs = append(d.CollectionIDs, cid)
	}
	if err := crows.Err(); err != nil {
		crows.Close()
		return d, mapError(err)
	}
	crows.Close()

	// warehouse_id: the single active warehouse, else null (0 or >1 active warehouses).
	var warehouseID *string
	whs, err := tx.Query(ctx, `SELECT id::text FROM inventory.warehouses WHERE tenant_id=$1 AND store_id=$2 AND active ORDER BY id`,
		scope.TenantID, scope.StoreID)
	if err != nil {
		return d, mapError(err)
	}
	whList := []string{}
	for whs.Next() {
		var w string
		if err := whs.Scan(&w); err != nil {
			whs.Close()
			return d, err
		}
		whList = append(whList, w)
	}
	if err := whs.Err(); err != nil {
		whs.Close()
		return d, mapError(err)
	}
	whs.Close()
	if len(whList) == 1 {
		warehouseID = &whList[0]
	}
	d.WarehouseID = warehouseID

	// on_hand/committed come from the single warehouse (null when 0 or >1 active warehouses); available is summed over all
	// warehouses and keyword is the SKU's library keyword ("" when none).
	var rows pgx.Rows
	if warehouseID != nil {
		rows, err = tx.Query(ctx, `SELECT `+prefixed("s", skuColumns)+`,
				coalesce(b.avail,0)::bigint,coalesce(k.keyword,''),bal.on_hand,(bal.reserved+bal.allocated+bal.unavailable)
			FROM catalog.skus s
			LEFT JOIN LATERAL (SELECT sum(x.on_hand-x.reserved-x.allocated-x.unavailable) avail FROM inventory.balances x
				WHERE x.tenant_id=s.tenant_id AND x.store_id=s.store_id AND x.sku_id=s.id) b ON true
			LEFT JOIN LATERAL (SELECT l.keyword FROM live.keyword_library l
				WHERE l.tenant_id=s.tenant_id AND l.store_id=s.store_id AND l.sku_id=s.id) k ON true
			LEFT JOIN inventory.balances bal ON bal.tenant_id=s.tenant_id AND bal.store_id=s.store_id AND bal.sku_id=s.id
				AND bal.warehouse_id=$3
			WHERE s.tenant_id=$1 AND s.store_id=$2 AND s.product_id=$4 AND s.status='active' ORDER BY s.created_at,s.id LIMIT 200`,
			scope.TenantID, scope.StoreID, *warehouseID, id)
	} else {
		rows, err = tx.Query(ctx, `SELECT `+prefixed("s", skuColumns)+`,
				coalesce(b.avail,0)::bigint,coalesce(k.keyword,''),NULL::bigint,NULL::bigint
			FROM catalog.skus s
			LEFT JOIN LATERAL (SELECT sum(x.on_hand-x.reserved-x.allocated-x.unavailable) avail FROM inventory.balances x
				WHERE x.tenant_id=s.tenant_id AND x.store_id=s.store_id AND x.sku_id=s.id) b ON true
			LEFT JOIN LATERAL (SELECT l.keyword FROM live.keyword_library l
				WHERE l.tenant_id=s.tenant_id AND l.store_id=s.store_id AND l.sku_id=s.id) k ON true
			WHERE s.tenant_id=$1 AND s.store_id=$2 AND s.product_id=$3 AND s.status='active' ORDER BY s.created_at,s.id LIMIT 200`,
			scope.TenantID, scope.StoreID, id)
	}
	if err != nil {
		return d, mapError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var s SKUStock
		if err := rows.Scan(append(skuFields(&s.SKU), &s.Available, &s.Keyword, &s.OnHand, &s.Committed)...); err != nil {
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
