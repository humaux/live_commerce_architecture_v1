package storefront

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strconv"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/buyer"
	"livecommerce/internal/command"
	"livecommerce/internal/pagination"
)

type CatalogRequest struct {
	ProductID string             `json:"product_id"`
	Page      pagination.Request `json:"page"`
}

type CatalogItem struct {
	ProductID   string `json:"product_id"`
	SKUID       string `json:"sku_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	SKUCode     string `json:"sku_code"`
	Currency    string `json:"currency"`
	PriceMinor  int64  `json:"price_minor"`
}

type catalogCursor struct {
	Version int    `json:"version"`
	Binding string `json:"binding"`
	After   string `json:"after"`
}

// The digest binds a position to the authenticated collection and filter;
// it is not an authenticator. The buyer capability and RLS remain authoritative.
func catalogBinding(s buyer.Scope, productID string) string {
	bound, _ := json.Marshal(struct {
		Collection string `json:"collection"`
		TenantID   string `json:"tenant_id"`
		StoreID    string `json:"store_id"`
		ProductID  string `json:"product_id"`
	}{"buyer.catalog.v1", s.TenantID, s.StoreID, productID})
	digest := sha256.Sum256(bound)
	return hex.EncodeToString(digest[:])
}

func decodeCatalogCursor(encoded, binding string) (string, error) {
	if encoded == "" {
		return "", nil
	}
	if len(encoded) > 1024 {
		return "", command.ErrInvalid
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil || len(raw) == 0 || len(raw) > 1024 || base64.RawURLEncoding.EncodeToString(raw) != encoded {
		return "", command.ErrInvalid
	}
	var cursor catalogCursor
	if err = json.Unmarshal(raw, &cursor); err != nil {
		return "", command.ErrInvalid
	}
	canonical, err := json.Marshal(cursor)
	if err != nil || !bytes.Equal(raw, canonical) || cursor.Version != 1 || cursor.Binding != binding || !command.ValidID(cursor.After) {
		return "", command.ErrInvalid
	}
	return cursor.After, nil
}

func encodeCatalogCursor(binding, after string) string {
	raw, _ := json.Marshal(catalogCursor{Version: 1, Binding: binding, After: after})
	return base64.RawURLEncoding.EncodeToString(raw)
}

// ListCatalog reads only the current, active buyer-visible SKU projection.
// It must run inside buyer.WithScope, which owns this transaction's lifetime.
func ListCatalog(ctx context.Context, tx pgx.Tx, s buyer.Scope, request CatalogRequest) (pagination.Page[CatalogItem], error) {
	page := pagination.Page[CatalogItem]{Items: []CatalogItem{}}
	if request.ProductID != "" && !command.ValidID(request.ProductID) || request.Page.Limit < 0 || request.Page.Limit > 100 {
		return page, command.ErrInvalid
	}
	if err := buyer.CheckScope(ctx, tx, s); err != nil {
		return page, err
	}
	limit := request.Page.Limit
	if limit == 0 {
		limit = 50
	}
	binding := catalogBinding(s, request.ProductID)
	after, err := decodeCatalogCursor(request.Page.Cursor, binding)
	if err != nil {
		return page, err
	}
	args := []any{s.TenantID, s.StoreID}
	query := `SELECT p.id::text,s.id::text,p.name,p.description,s.code,s.currency,s.price_minor
		FROM catalog.skus s
		JOIN catalog.products p ON p.tenant_id=s.tenant_id AND p.store_id=s.store_id AND p.id=s.product_id
		JOIN control.stores st ON st.tenant_id=s.tenant_id AND st.id=s.store_id
		WHERE s.tenant_id=$1 AND s.store_id=$2 AND p.status='active' AND s.status='active' AND s.currency=st.currency`
	if request.ProductID != "" {
		args = append(args, request.ProductID)
		query += ` AND p.id=$` + strconv.Itoa(len(args)) + `::uuid`
	}
	if after != "" {
		args = append(args, after)
		query += ` AND s.id>$` + strconv.Itoa(len(args)) + `::uuid`
	}
	args = append(args, limit+1)
	query += ` ORDER BY s.id LIMIT $` + strconv.Itoa(len(args))
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return page, err
	}
	for rows.Next() {
		var item CatalogItem
		if err = rows.Scan(&item.ProductID, &item.SKUID, &item.Name, &item.Description, &item.SKUCode, &item.Currency, &item.PriceMinor); err != nil {
			rows.Close()
			return page, err
		}
		page.Items = append(page.Items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return page, err
	}
	if err = buyer.CheckScope(ctx, tx, s); err != nil {
		return page, err
	}
	if len(page.Items) > limit {
		page.Items = page.Items[:limit]
		page.NextCursor = encodeCatalogCursor(binding, page.Items[limit-1].SKUID)
	}
	return page, nil
}
