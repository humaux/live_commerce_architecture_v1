package checkout

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/buyer"
	"livecommerce/internal/command"
	"livecommerce/internal/pagination"
	"livecommerce/internal/pricing"
)

type OptionsRequest struct {
	MarketID string             `json:"market_id"`
	Country  string             `json:"country"`
	Page     pagination.Request `json:"page"`
}

type Option struct {
	MarketID          string `json:"market_id"`
	MarketCode        string `json:"market_code"`
	MarketName        string `json:"market_name"`
	Country           string `json:"country"`
	Currency          string `json:"currency"`
	DeliveryCode      string `json:"delivery_code"`
	Method            string `json:"method"`
	ServiceVersion    int64  `json:"service_version"`
	AllocationVersion int64  `json:"allocation_version"`
	DeliveryKind      string `json:"delivery_kind"`
	Mode              string `json:"mode"`
	NameHans          string `json:"name_hans"`
	NameHant          string `json:"name_hant"`
	NameEN            string `json:"name_en"`
	SortOrder         int    `json:"sort_order"`
}

type optionsCursor struct {
	Version      int    `json:"version"`
	Binding      string `json:"binding"`
	MarketID     string `json:"market_id"`
	Country      string `json:"country"`
	DeliveryCode string `json:"delivery_code"`
}

func validOptionCountry(country string) bool {
	return len(country) == 2 && country[0] >= 'A' && country[0] <= 'Z' && country[1] >= 'A' && country[1] <= 'Z'
}

func optionsBinding(scope buyer.Scope, in OptionsRequest) string {
	raw, _ := json.Marshal(struct {
		Collection string `json:"collection"`
		TenantID   string `json:"tenant_id"`
		StoreID    string `json:"store_id"`
		MarketID   string `json:"market_id"`
		Country    string `json:"country"`
	}{"buyer.checkout-options.v1", scope.TenantID, scope.StoreID, in.MarketID, in.Country})
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func decodeOptionsCursor(encoded, binding string) (optionsCursor, error) {
	if encoded == "" {
		return optionsCursor{}, nil
	}
	if len(encoded) > 1024 {
		return optionsCursor{}, command.ErrInvalid
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil || len(raw) == 0 || len(raw) > 1024 || base64.RawURLEncoding.EncodeToString(raw) != encoded {
		return optionsCursor{}, command.ErrInvalid
	}
	var position optionsCursor
	if err = json.Unmarshal(raw, &position); err != nil {
		return optionsCursor{}, command.ErrInvalid
	}
	canonical, err := json.Marshal(position)
	_, codeErr := pricing.DeliveryMethod(position.DeliveryCode)
	if err != nil || !bytes.Equal(raw, canonical) || position.Version != 1 || position.Binding != binding || !command.ValidID(position.MarketID) || !validOptionCountry(position.Country) || codeErr != nil {
		return optionsCursor{}, command.ErrInvalid
	}
	return position, nil
}

func encodeOptionsCursor(binding string, option Option) string {
	raw, _ := json.Marshal(optionsCursor{1, binding, option.MarketID, option.Country, option.DeliveryCode})
	return base64.RawURLEncoding.EncodeToString(raw)
}

// ListOptions presents current selectable configuration, not availability or a
// reservation. Begin still owns final locked configuration/stock revalidation.
func (s *Service) ListOptions(ctx context.Context, token, storeID string, in OptionsRequest) (pagination.Page[Option], error) {
	page := pagination.Page[Option]{Items: []Option{}}
	if ctx == nil || s == nil || s.pool == nil || in.MarketID != "" && !command.ValidID(in.MarketID) || in.Country != "" && !validOptionCountry(in.Country) || in.Page.Limit < 0 || in.Page.Limit > 100 || len(in.Page.Cursor) > 1024 {
		return page, command.ErrInvalid
	}
	limit := in.Page.Limit
	if limit == 0 {
		limit = 50
	}
	tokenHash := sha256.Sum256([]byte(token))
	err := buyer.WithScope(ctx, s.pool, token, storeID, func(callCtx context.Context, tx pgx.Tx, scope buyer.Scope) error {
		if err := buyer.CheckScope(callCtx, tx, scope); err != nil {
			return err
		}
		binding := optionsBinding(scope, in)
		position, err := decodeOptionsCursor(in.Page.Cursor, binding)
		if err != nil {
			return err
		}
		var marketFilter, countryFilter, afterMarket, afterCountry, afterCode any
		if in.MarketID != "" {
			marketFilter = in.MarketID
		}
		if in.Country != "" {
			countryFilter = in.Country
		}
		if position.MarketID != "" {
			afterMarket, afterCountry, afterCode = position.MarketID, position.Country, position.DeliveryCode
		}
		rows, err := tx.Query(callCtx, `SELECT m.id::text,m.code,m.name,sv.country,sv.currency,sv.code,sv.policy_method,
			sv.version,av.version,sv.delivery_kind,sv.mode,sv.name_hans,sv.name_hant,sv.name_en,sv.sort_order
			FROM pricing.markets m
			JOIN control.stores st ON st.tenant_id=m.tenant_id AND st.id=m.store_id
			JOIN fulfillment.service_heads sh ON sh.tenant_id=m.tenant_id AND sh.store_id=m.store_id AND sh.market_id=m.id
			JOIN fulfillment.service_versions sv ON sv.tenant_id=sh.tenant_id AND sv.store_id=sh.store_id AND sv.market_id=sh.market_id
			 AND sv.country=sh.country AND sv.code=sh.code AND sv.version=sh.current_version
			JOIN pricing.policy_heads ph ON ph.tenant_id=sv.tenant_id AND ph.store_id=sv.store_id AND ph.market_id=sv.market_id
			 AND ph.country=sv.country AND ph.method=sv.policy_method
			JOIN pricing.policy_versions pv ON pv.tenant_id=ph.tenant_id AND pv.store_id=ph.store_id AND pv.market_id=ph.market_id
			 AND pv.country=ph.country AND pv.method=ph.method AND pv.version=ph.current_version
			JOIN fulfillment.allocation_heads ah ON ah.tenant_id=sv.tenant_id AND ah.store_id=sv.store_id AND ah.market_id=sv.market_id
			 AND ah.country=sv.country AND ah.code=sv.code
			JOIN fulfillment.allocation_versions av ON av.tenant_id=ah.tenant_id AND av.store_id=ah.store_id AND av.market_id=ah.market_id
			 AND av.country=ah.country AND av.code=ah.code AND av.version=ah.current_version
			JOIN LATERAL (SELECT count(*) AS child_count,bool_and(w.active) AS all_active
			 FROM fulfillment.allocation_warehouses aw
			 JOIN inventory.warehouses w ON w.tenant_id=aw.tenant_id AND w.store_id=aw.store_id AND w.id=aw.warehouse_id
			 WHERE aw.tenant_id=av.tenant_id AND aw.store_id=av.store_id AND aw.market_id=av.market_id
			 AND aw.country=av.country AND aw.code=av.code AND aw.version=av.version) wh
			 ON wh.child_count=av.warehouse_count AND wh.all_active
			WHERE m.tenant_id=$1 AND m.store_id=$2 AND m.active AND m.currency=st.currency
			 AND sv.currency=m.currency AND pv.currency=sv.currency AND pv.enabled AND pv.version=sv.policy_version
			 AND sv.enabled AND sv.visible AND sv.mode='MANUAL' AND sv.binding_id IS NULL AND sv.binding_version IS NULL
			 AND sv.delivery_kind IN ('home','cvs_711','cvs_familymart') AND (sv.delivery_kind='home' OR sv.country='TW')
			 AND av.warehouse_count BETWEEN 1 AND 16
			 AND ($3::uuid IS NULL OR m.id=$3::uuid) AND ($4::text IS NULL OR sv.country=$4::text)
			 AND ($5::uuid IS NULL OR (m.id,sv.country COLLATE "C",sv.code COLLATE "C") > ($5::uuid,$6::text COLLATE "C",$7::text COLLATE "C"))
			ORDER BY m.id,sv.country COLLATE "C",sv.code COLLATE "C" LIMIT $8`, scope.TenantID, scope.StoreID,
			marketFilter, countryFilter, afterMarket, afterCountry, afterCode, limit+1)
		if err != nil {
			return err
		}
		for rows.Next() {
			var option Option
			if err = rows.Scan(&option.MarketID, &option.MarketCode, &option.MarketName, &option.Country, &option.Currency,
				&option.DeliveryCode, &option.Method, &option.ServiceVersion, &option.AllocationVersion,
				&option.DeliveryKind, &option.Mode, &option.NameHans, &option.NameHant, &option.NameEN, &option.SortOrder); err != nil {
				rows.Close()
				return err
			}
			page.Items = append(page.Items, option)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if err = checkCapability(callCtx, tx, tokenHash[:], storeID, scope); err != nil {
			return err
		}
		if len(page.Items) > limit {
			page.Items = page.Items[:limit]
			page.NextCursor = encodeOptionsCursor(binding, page.Items[limit-1])
		}
		return nil
	})
	if err != nil {
		return pagination.Page[Option]{Items: []Option{}}, safeError(ctx, err)
	}
	return page, nil
}
