package catalog

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// ErrPurchaseEntryUnavailable is a safe transport boundary for a malformed or
// unavailable privileged origin projection; driver details must not reach HTTP.
var ErrPurchaseEntryUnavailable = errors.New("purchase entry unavailable")

type PurchaseEntry struct {
	ProductID string `json:"product_id"`
	Locale    string `json:"locale"`
	State     string `json:"state"`
	URL       string `json:"url"`
}

// ReadPurchaseEntry uses the existing merchant-scoped transaction. The second
// token check inside resolve_storefront_origin is intentional: Scope/GUC alone
// cannot authorize a privileged publication read after a session is revoked.
func ReadPurchaseEntry(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, productID, locale string) (PurchaseEntry, error) {
	var out PurchaseEntry
	if !validScope(tx, scope) || !command.ValidID(productID) || !validPurchaseLocale(locale) || token == "" {
		return out, command.ErrInvalid
	}
	hash := sha256.Sum256([]byte(token))
	var state, origin string
	if err := tx.QueryRow(ctx, `SELECT state,origin FROM identity.resolve_storefront_origin($1::bytea,$2::uuid)`, hash[:], scope.StoreID).Scan(&state, &origin); err != nil {
		return out, purchaseEntryDBError(ctx, err)
	}
	if !validPurchaseOrigin(state, origin) {
		return out, ErrPurchaseEntryUnavailable
	}

	var productStatus string
	var hasSKU bool
	if err := tx.QueryRow(ctx, `SELECT product.status, EXISTS (
		SELECT 1 FROM catalog.skus AS sku
		JOIN control.stores AS store ON store.tenant_id=sku.tenant_id AND store.id=sku.store_id
		WHERE sku.tenant_id=product.tenant_id AND sku.store_id=product.store_id
		AND sku.product_id=product.id AND sku.status='active' AND sku.currency=store.currency
	) FROM catalog.products AS product
	WHERE product.tenant_id=$1::uuid AND product.store_id=$2::uuid AND product.id=$3::uuid`, scope.TenantID, scope.StoreID, productID).Scan(&productStatus, &hasSKU); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return out, command.ErrNotFound
		}
		return out, purchaseEntryDBError(ctx, err)
	}
	out = PurchaseEntry{ProductID: productID, Locale: locale}
	switch productStatus {
	case "archived":
		out.State = "product_inactive"
		return out, nil
	case "active":
	default:
		return PurchaseEntry{}, ErrPurchaseEntryUnavailable
	}

	if !hasSKU {
		out.State = "no_active_sku"
		return out, nil
	}
	out.State = state
	if state == "configured" {
		out.URL = origin + "/" + locale + "/products/" + productID
	}
	return out, nil
}

func validPurchaseLocale(locale string) bool {
	return locale == "zh-CN" || locale == "zh-TW" || locale == "en"
}

func validPurchaseOrigin(state, origin string) bool {
	if state == "storefront_unavailable" || state == "domain_selection_required" {
		return origin == ""
	}
	if state != "configured" || origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "https" || u.Opaque != "" || u.Host == "" || u.User != nil || u.Path != "" || u.RawPath != "" || u.RawQuery != "" || u.Fragment != "" || u.ForceQuery {
		return false
	}
	return u.Host == u.Hostname() && u.Host == strings.ToLower(u.Host) && u.String() == origin
}

func purchaseEntryDBError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "PT400":
			return command.ErrInvalid
		case "PT401":
			return platform.ErrUnauthorized
		case "PT403":
			return platform.ErrForbidden
		case "PT404":
			return platform.ErrScopeNotFound
		case "PT503":
			return ErrPurchaseEntryUnavailable
		}
	}
	return ErrPurchaseEntryUnavailable
}
