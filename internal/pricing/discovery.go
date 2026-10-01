package pricing

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/command"
	"livecommerce/internal/pagination"
	"livecommerce/internal/platform"
)

// ListMarketsPage exposes only the market configuration needed by an admin.
func ListMarketsPage(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, request pagination.Request) (pagination.Page[Market], error) {
	page := pagination.Page[Market]{Items: make([]Market, 0)}
	if !validScope(tx, scope) || scope.Revision < 1 {
		return page, command.ErrInvalid
	}
	if err := platform.RequirePermission(ctx, tx, scope, token, "pricing:read"); err != nil {
		return page, err
	}
	binding := pagination.Binding{TenantID: scope.TenantID, StoreID: scope.StoreID, Collection: "markets"}
	limit, after, err := pagination.Decode(request, binding, 1)
	if err != nil {
		return page, err
	}
	args := []any{scope.TenantID, scope.StoreID}
	query := `SELECT id::text,code,name,currency,version,active FROM pricing.markets
		WHERE tenant_id=$1 AND store_id=$2`
	if len(after) == 1 {
		query += ` AND id>$3::uuid`
		args = append(args, after[0])
	}
	query += ` ORDER BY id LIMIT $` + strconv.Itoa(len(args)+1)
	args = append(args, limit+1)
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return pagination.Page[Market]{}, mapError(err)
	}
	for rows.Next() {
		var item Market
		if err := rows.Scan(&item.ID, &item.Code, &item.Name, &item.Currency, &item.Version, &item.Active); err != nil {
			rows.Close()
			return pagination.Page[Market]{}, mapError(err)
		}
		page.Items = append(page.Items, item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return pagination.Page[Market]{}, mapError(err)
	}
	if err := platform.RequirePermission(ctx, tx, scope, token, "pricing:read"); err != nil {
		return pagination.Page[Market]{}, err
	}
	if len(page.Items) > limit {
		page.Items = page.Items[:limit]
		page.NextCursor, err = pagination.Encode(binding, []string{page.Items[len(page.Items)-1].ID})
		if err != nil {
			return pagination.Page[Market]{}, err
		}
	}
	return page, nil
}

// GetDeliveryPolicy returns the current policy, including a disabled one.
// The projection deliberately excludes configuration_ref and principal_id.
func GetDeliveryPolicy(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, marketID, country, code string) (Policy, error) {
	if !validScope(tx, scope) || scope.Revision < 1 || !command.ValidID(marketID) || !countryPattern.MatchString(country) {
		return Policy{}, command.ErrInvalid
	}
	method, err := DeliveryMethod(code)
	if err != nil {
		return Policy{}, err
	}
	if err := platform.RequirePermission(ctx, tx, scope, token, "pricing:read"); err != nil {
		return Policy{}, err
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pricing.markets WHERE tenant_id=$1 AND store_id=$2 AND id=$3)`,
		scope.TenantID, scope.StoreID, marketID).Scan(&exists); err != nil {
		return Policy{}, mapError(err)
	}
	if err := platform.RequirePermission(ctx, tx, scope, token, "pricing:read"); err != nil {
		return Policy{}, err
	}
	if !exists {
		return Policy{}, command.ErrNotFound
	}
	var out Policy
	err = tx.QueryRow(ctx, `SELECT v.market_id::text,v.country,v.method,v.currency,v.shipping_mode,v.tax_mode,v.tax_basis,
		v.version,v.shipping_minor,v.tax_rate_bps,v.quote_ttl_seconds,v.enabled,v.free_shipping_threshold_minor
		FROM pricing.policy_heads h JOIN pricing.policy_versions v
		ON (v.tenant_id,v.store_id,v.market_id,v.country,v.method,v.version)=
		(h.tenant_id,h.store_id,h.market_id,h.country,h.method,h.current_version)
		WHERE h.tenant_id=$1 AND h.store_id=$2 AND h.market_id=$3 AND h.country=$4 AND h.method=$5`,
		scope.TenantID, scope.StoreID, marketID, country, method).Scan(
		&out.MarketID, &out.Country, &out.Method, &out.Currency, &out.ShippingMode, &out.TaxMode, &out.TaxBasis,
		&out.Version, &out.ShippingMinor, &out.TaxRateBPS, &out.QuoteTTLSeconds, &out.Enabled, &out.FreeShippingThresholdMinor)
	if err != nil {
		return Policy{}, mapError(err)
	}
	if err := platform.RequirePermission(ctx, tx, scope, token, "pricing:read"); err != nil {
		return Policy{}, err
	}
	return out, nil
}
