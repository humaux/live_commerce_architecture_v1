// Purpose: Merchant catalog ledger read: one row per SKU with stock columns and the product cover (main image position 0).
// Depends on: catalog.skus/products/product_images, inventory.balances (role commerce_runtime, RLS scope).
// Used by: internal/httpapi ledger routes; apps/admin Ledger.

package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/command"
	"livecommerce/internal/pagination"
	"livecommerce/internal/platform"
)

type LedgerRow struct {
	ProductID          string `json:"product_id"`
	ProductName        string `json:"product_name"`
	ProductDescription string `json:"product_description"`
	SKUID              string `json:"sku_id"`
	Code               string `json:"code"`
	Status             string `json:"status"`
	Currency           string `json:"currency"`
	PriceMinor         int64  `json:"price_minor"`
	SKUVersion         int64  `json:"sku_version"`
	WarehouseID        string `json:"warehouse_id"`
	OnHand             int64  `json:"on_hand"`
	Reserved           int64  `json:"reserved"`
	Allocated          int64  `json:"allocated"`
	Unavailable        int64  `json:"unavailable"`
	Available          int64  `json:"available"`
	BalanceVersion     int64  `json:"balance_version"`
	// catalog-media: the product-level facts the merchant UI needs for rename/archive (expected_version) and the
	// thumbnail. ProductStatus is the product's own status (Status above also turns archived with its SKU).
	// CoverImageID is the id of the product's position-0 photo, nil when it has none.
	ProductVersion int64   `json:"product_version"`
	ProductStatus  string  `json:"product_status"`
	CoverImageID   *string `json:"cover_image_id"`
}
type LedgerRequest struct {
	Page        pagination.Request `json:"page"`
	WarehouseID string             `json:"warehouse_id"`
	Query       string             `json:"query"`
	Status      string             `json:"status"`
}

func ListLedger(ctx context.Context, tx pgx.Tx, scope platform.Scope, in LedgerRequest) (pagination.Page[LedgerRow], error) {
	page := pagination.Page[LedgerRow]{Items: make([]LedgerRow, 0)}
	if !validScope(tx, scope) || !command.ValidID(in.WarehouseID) {
		return page, command.ErrInvalid
	}
	query, status, ok := ledgerFilter(in.Query, in.Status)
	if !ok {
		return page, command.ErrInvalid
	}
	var warehouse string
	if err := tx.QueryRow(ctx, `SELECT id::text FROM inventory.warehouses WHERE tenant_id=$1 AND store_id=$2 AND id=$3 AND active`, scope.TenantID, scope.StoreID, in.WarehouseID).Scan(&warehouse); err != nil {
		return page, mapError(err)
	}
	hash := sha256.Sum256([]byte(query + "\x00" + status))
	binding := pagination.Binding{TenantID: scope.TenantID, StoreID: scope.StoreID, Collection: "catalog-ledger", ParentID: warehouse, Filter: hex.EncodeToString(hash[:])}
	limit, after, err := pagination.Decode(in.Page, binding, 1)
	if err != nil {
		return page, err
	}
	args := []any{scope.TenantID, scope.StoreID, warehouse}
	filters := ""
	if query != "" {
		filters += ` AND (p.name ILIKE '%' || $4 || '%' ESCAPE '!' OR s.code ILIKE '%' || $4 || '%' ESCAPE '!')`
		args = append(args, escapeLike(query))
	}
	if status != "all" {
		filters += ` AND CASE WHEN p.status='archived' OR s.status='archived' THEN 'archived' ELSE 'active' END=$` + itoa(len(args)+1)
		args = append(args, status)
	}
	if len(after) == 1 {
		filters += ` AND s.id>$` + itoa(len(args)+1) + `::uuid`
		args = append(args, after[0])
	}
	sql := `SELECT p.id::text,p.name,p.description,s.id::text,s.code,CASE WHEN p.status='archived' OR s.status='archived' THEN 'archived' ELSE 'active' END,s.currency,s.price_minor,s.version,$3::text,COALESCE(b.on_hand,0),COALESCE(b.reserved,0),COALESCE(b.allocated,0),COALESCE(b.unavailable,0),COALESCE(b.on_hand,0)-COALESCE(b.reserved,0)-COALESCE(b.allocated,0)-COALESCE(b.unavailable,0),COALESCE(b.version,0),p.version,p.status,(SELECT i.id::text FROM catalog.product_images i WHERE i.tenant_id=p.tenant_id AND i.store_id=p.store_id AND i.product_id=p.id AND i.role='main' ORDER BY i.position LIMIT 1) FROM catalog.skus s JOIN catalog.products p ON p.tenant_id=s.tenant_id AND p.store_id=s.store_id AND p.id=s.product_id LEFT JOIN inventory.balances b ON b.tenant_id=s.tenant_id AND b.store_id=s.store_id AND b.warehouse_id=$3::uuid AND b.sku_id=s.id WHERE s.tenant_id=$1 AND s.store_id=$2` + filters + ` ORDER BY s.id LIMIT $` + itoa(len(args)+1)
	args = append(args, limit+1)
	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return page, mapError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var r LedgerRow
		if err := rows.Scan(&r.ProductID, &r.ProductName, &r.ProductDescription, &r.SKUID, &r.Code, &r.Status, &r.Currency, &r.PriceMinor, &r.SKUVersion, &r.WarehouseID, &r.OnHand, &r.Reserved, &r.Allocated, &r.Unavailable, &r.Available, &r.BalanceVersion, &r.ProductVersion, &r.ProductStatus, &r.CoverImageID); err != nil {
			return page, err
		}
		page.Items = append(page.Items, r)
	}
	if err := rows.Err(); err != nil {
		return page, mapError(err)
	}
	if len(page.Items) <= limit {
		return page, nil
	}
	page.Items = page.Items[:limit]
	page.NextCursor, err = pagination.Encode(binding, []string{page.Items[len(page.Items)-1].SKUID})
	return page, err
}
func ledgerFilter(q, status string) (string, string, bool) {
	for _, r := range q {
		if unicode.IsControl(r) {
			return "", "", false
		}
	}
	q = strings.TrimSpace(q)
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		status = "all"
	}
	if utf8.RuneCountInString(q) > 120 {
		return "", "", false
	}
	return q, status, status == "all" || status == "active" || status == "archived"
}
func escapeLike(s string) string {
	// Explicit single-character escape avoids standard_conforming_strings drift.
	return strings.NewReplacer(`!`, `!!`, `%`, `!%`, `_`, `!_`).Replace(s)
}
func itoa(n int) string { return strconv.Itoa(n) }
