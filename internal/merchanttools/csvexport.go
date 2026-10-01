// csvexport.go serves the product CSV export (contract G2): one row per active variant, UTF-8 with a BOM so Excel opens it as UTF-8,
// CRLF rows, spreadsheet-safe text cells, stock on hand per warehouse, collection slugs and the photo count.
//
// Non-goals: no write and no audit row (it is a catalog read, not a customer-data export), no archived product or variant, no price
// history, no file storage (generated per request, nothing kept), no image bytes.
// Tables read under RLS as commerce_runtime inside the caller's platform.WithScope transaction: catalog.products, catalog.skus,
// catalog.collections, catalog.collection_products, catalog.product_images, inventory.balances, control.stores (one currency read).

package merchanttools

import (
	"bytes"
	"context"
	"encoding/csv"
	"strconv"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/command"
	"livecommerce/internal/inventory"
	"livecommerce/internal/platform"
)

// storeCurrency reads the store's currency (control.stores, commerce_runtime under RLS), the same read catalog.storeCurrency does for a SKU.
func storeCurrency(ctx context.Context, tx pgx.Tx, scope platform.Scope) (string, error) {
	var currency string
	err := tx.QueryRow(ctx, `SELECT currency FROM control.stores WHERE tenant_id=$1 AND id=$2`, scope.TenantID, scope.StoreID).Scan(&currency)
	return currency, err
}

// ExportProducts returns the whole catalog as CSV bytes (BOM included).
func ExportProducts(ctx context.Context, tx pgx.Tx, scope platform.Scope) ([]byte, error) {
	if tx == nil || !command.ValidID(scope.TenantID) || !command.ValidID(scope.StoreID) {
		return nil, command.ErrInvalid
	}
	currency, err := storeCurrency(ctx, tx, scope)
	if err != nil {
		return nil, err
	}
	exp := CurrencyExponent(currency)
	warehouses, err := inventory.ListWarehouses(ctx, tx, scope)
	if err != nil {
		return nil, err
	}
	onHand := map[string]int64{} // "sku|warehouse" -> on_hand
	balances, err := tx.Query(ctx, `SELECT sku_id::text,warehouse_id::text,on_hand FROM inventory.balances WHERE tenant_id=$1 AND store_id=$2`, scope.TenantID, scope.StoreID)
	if err != nil {
		return nil, err
	}
	for balances.Next() {
		var sku, wh string
		var qty int64
		if err = balances.Scan(&sku, &wh, &qty); err != nil {
			balances.Close()
			return nil, err
		}
		onHand[sku+"|"+wh] = qty
	}
	balances.Close()
	if err = balances.Err(); err != nil {
		return nil, err
	}
	collections := map[string][]string{}
	members, err := tx.Query(ctx, `SELECT cp.product_id::text,c.slug FROM catalog.collection_products cp
		JOIN catalog.collections c ON c.tenant_id=cp.tenant_id AND c.store_id=cp.store_id AND c.id=cp.collection_id
		WHERE cp.tenant_id=$1 AND cp.store_id=$2 ORDER BY cp.product_id,c.slug`, scope.TenantID, scope.StoreID)
	if err != nil {
		return nil, err
	}
	for members.Next() {
		var product, slug string
		if err = members.Scan(&product, &slug); err != nil {
			members.Close()
			return nil, err
		}
		collections[product] = append(collections[product], slug)
	}
	members.Close()
	if err = members.Err(); err != nil {
		return nil, err
	}
	images := map[string]int64{}
	counts, err := tx.Query(ctx, `SELECT product_id::text,count(*) FROM catalog.product_images WHERE tenant_id=$1 AND store_id=$2 GROUP BY 1`, scope.TenantID, scope.StoreID)
	if err != nil {
		return nil, err
	}
	for counts.Next() {
		var product string
		var n int64
		if err = counts.Scan(&product, &n); err != nil {
			counts.Close()
			return nil, err
		}
		images[product] = n
	}
	counts.Close()
	if err = counts.Err(); err != nil {
		return nil, err
	}

	var out bytes.Buffer
	out.Write([]byte{0xEF, 0xBB, 0xBF})
	w := csv.NewWriter(&out)
	w.UseCRLF = true
	header := append([]string{}, fixedColumns...)
	for _, wh := range warehouses {
		header = append(header, stockPrefix+wh.Name)
	}
	header = append(header, tailColumns...)
	if err = w.Write(header); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT p.id::text,p.slug,p.name,p.description,p.status,p.options,
			coalesce(s.id::text,''),coalesce(s.code,''),s.price_minor,s.compare_at_minor,s.option_values
		FROM catalog.products p
		LEFT JOIN catalog.skus s ON s.tenant_id=p.tenant_id AND s.store_id=p.store_id AND s.product_id=p.id AND s.status='active'
		WHERE p.tenant_id=$1 AND p.store_id=$2 AND p.status<>'archived'
		ORDER BY p.slug,s.created_at,s.id LIMIT $3`, scope.TenantID, scope.StoreID, MaxExportRows+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var product, slug, name, description, status, skuID, code string
		var axes []struct {
			Name string `json:"name"`
		}
		var price, compare *int64
		var values []string
		if err = rows.Scan(&product, &slug, &name, &description, &status, &axes, &skuID, &code, &price, &compare, &values); err != nil {
			return nil, err
		}
		if n++; n > MaxExportRows {
			return nil, ErrExportTooLarge
		}
		rec := []string{guardCell(slug), guardCell(name), guardCell(description), status}
		for k := 0; k < 3; k++ {
			axisName, axisValue := "", ""
			if k < len(axes) {
				axisName = guardCell(axes[k].Name)
			}
			if k < len(values) {
				axisValue = guardCell(values[k])
			}
			rec = append(rec, axisName, axisValue)
		}
		rec = append(rec, guardCell(code), optMajor(price, exp), optMajor(compare, exp))
		for _, wh := range warehouses {
			cell := ""
			if qty, ok := onHand[skuID+"|"+wh.ID]; ok && skuID != "" {
				cell = strconv.FormatInt(qty, 10)
			}
			rec = append(rec, cell)
		}
		list := ""
		for i, slug := range collections[product] {
			if i > 0 {
				list += "|"
			}
			list += slug
		}
		rec = append(rec, list, strconv.FormatInt(images[product], 10))
		if err = w.Write(rec); err != nil {
			return nil, err
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	w.Flush()
	return out.Bytes(), w.Error()
}

func optMajor(minor *int64, exp int) string {
	if minor == nil {
		return ""
	}
	return formatMajor(*minor, exp)
}
