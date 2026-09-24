package payments

import (
	"context"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/command"
	"livecommerce/internal/pagination"
	"livecommerce/internal/platform"
)

// ListMethods returns the closed TW method vocabulary in code order.
func ListMethods(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, marketID, country string) (pagination.Page[Method], error) {
	page := pagination.Page[Method]{Items: make([]Method, 0)}
	if !command.ValidID(marketID) || country != "TW" {
		return page, command.ErrInvalid
	}
	if err := authorize(ctx, tx, scope, token, "integration:read"); err != nil {
		return page, err
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pricing.markets WHERE tenant_id=$1 AND store_id=$2 AND id=$3)`,
		scope.TenantID, scope.StoreID, marketID).Scan(&exists); err != nil {
		return pagination.Page[Method]{}, mapError(err)
	}
	if err := authorize(ctx, tx, scope, token, "integration:read"); err != nil {
		return pagination.Page[Method]{}, err
	}
	if !exists {
		return pagination.Page[Method]{}, command.ErrNotFound
	}
	rows, err := tx.Query(ctx, `SELECT v.market_id::text,v.country,v.code,v.version,v.provider,v.environment,
		coalesce(v.connection_id::text,''),coalesce(v.binding_version,0),v.currency,
		v.name_hans,v.name_hant,v.name_en,v.enabled,v.visible,v.sort_order,v.min_amount_minor,v.max_amount_minor
		FROM payments.method_heads h JOIN payments.method_versions v
		ON (v.tenant_id,v.store_id,v.market_id,v.country,v.code,v.version)=
		(h.tenant_id,h.store_id,h.market_id,h.country,h.code,h.current_version)
		WHERE h.tenant_id=$1 AND h.store_id=$2 AND h.market_id=$3 AND h.country=$4
		ORDER BY v.code COLLATE "C" LIMIT 6`, scope.TenantID, scope.StoreID, marketID, country)
	if err != nil {
		return pagination.Page[Method]{}, mapError(err)
	}
	for rows.Next() {
		var item Method
		if err := rows.Scan(&item.MarketID, &item.Country, &item.Code, &item.Version, &item.Provider, &item.Environment,
			&item.ConnectionID, &item.BindingVersion, &item.Currency, &item.NameHans, &item.NameHant, &item.NameEN,
			&item.Enabled, &item.Visible, &item.SortOrder, &item.MinAmountMinor, &item.MaxAmountMinor); err != nil {
			rows.Close()
			return pagination.Page[Method]{}, mapError(err)
		}
		page.Items = append(page.Items, item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return pagination.Page[Method]{}, mapError(err)
	}
	if len(page.Items) > 5 {
		return pagination.Page[Method]{}, command.ErrConflict
	}
	if err := authorize(ctx, tx, scope, token, "integration:read"); err != nil {
		return pagination.Page[Method]{}, err
	}
	return page, nil
}
