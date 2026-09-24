package fulfillment

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/command"
	"livecommerce/internal/pagination"
	"livecommerce/internal/platform"
)

// ListServicesPage uses the immutable code as its stable key, never sort_order.
func ListServicesPage(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, marketID, country string, request pagination.Request) (pagination.Page[Service], error) {
	page := pagination.Page[Service]{Items: make([]Service, 0)}
	if !command.ValidID(marketID) || !countryPattern.MatchString(country) {
		return page, command.ErrInvalid
	}
	if err := authorize(ctx, tx, scope, token, readPermission); err != nil {
		return page, err
	}
	binding := pagination.Binding{TenantID: scope.TenantID, StoreID: scope.StoreID, Collection: "delivery-services", ParentID: marketID, Filter: country}
	limit, after, err := pagination.Decode(request, binding, 1)
	if err != nil {
		return page, err
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pricing.markets WHERE tenant_id=$1 AND store_id=$2 AND id=$3)`,
		scope.TenantID, scope.StoreID, marketID).Scan(&exists); err != nil {
		return pagination.Page[Service]{}, mapError(err)
	}
	if err := authorize(ctx, tx, scope, token, readPermission); err != nil {
		return pagination.Page[Service]{}, err
	}
	if !exists {
		return pagination.Page[Service]{}, command.ErrNotFound
	}
	args := []any{scope.TenantID, scope.StoreID, marketID, country}
	query := `SELECT v.market_id::text,v.country,v.code,v.version,v.policy_method,v.policy_version,v.currency,
		v.name_hans,v.name_hant,v.name_en,v.delivery_kind,v.mode,v.enabled,v.visible,v.sort_order,
		coalesce(v.binding_id::text,''),coalesce(v.binding_version,0)
		FROM fulfillment.service_heads h JOIN fulfillment.service_versions v
		ON (v.tenant_id,v.store_id,v.market_id,v.country,v.code,v.version)=
		(h.tenant_id,h.store_id,h.market_id,h.country,h.code,h.current_version)
		WHERE h.tenant_id=$1 AND h.store_id=$2 AND h.market_id=$3 AND h.country=$4`
	if len(after) == 1 {
		query += ` AND v.code COLLATE "C">$5::text COLLATE "C"`
		args = append(args, after[0])
	}
	query += ` ORDER BY v.code COLLATE "C" LIMIT $` + strconv.Itoa(len(args)+1)
	args = append(args, limit+1)
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return pagination.Page[Service]{}, mapError(err)
	}
	for rows.Next() {
		var item Service
		if err := rows.Scan(&item.MarketID, &item.Country, &item.Code, &item.Version, &item.PolicyMethod, &item.PolicyVersion,
			&item.Currency, &item.NameHans, &item.NameHant, &item.NameEN, &item.DeliveryKind, &item.Mode,
			&item.Enabled, &item.Visible, &item.SortOrder, &item.BindingID, &item.BindingVersion); err != nil {
			rows.Close()
			return pagination.Page[Service]{}, mapError(err)
		}
		page.Items = append(page.Items, item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return pagination.Page[Service]{}, mapError(err)
	}
	if err := authorize(ctx, tx, scope, token, readPermission); err != nil {
		return pagination.Page[Service]{}, err
	}
	if len(page.Items) > limit {
		page.Items = page.Items[:limit]
		page.NextCursor, err = pagination.Encode(binding, []string{page.Items[len(page.Items)-1].Code})
		if err != nil {
			return pagination.Page[Service]{}, err
		}
	}
	return page, nil
}
