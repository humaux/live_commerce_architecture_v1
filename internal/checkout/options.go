package checkout

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"slices"

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
	// CVS rows (taiwan-cvs-logistics-v1 §5.1, §16.1, §16.5). PickupSelection is how the buyer names the store: "ecpay_map" when the
	// store has an enabled qualified ECPay profile, else "buyer_entered" (mode is decided in SQL only, X9). PaymentModes lists
	// "card" (only while this process can take card payment, OP1) and, when the store enabled it, "pay_at_pickup"; a row with no mode
	// is not offered. StoreSearchURL is the chain's official search page (buyer_entered only).
	// Available is present (false) only on a row the store configured but the buyer cannot use yet: Reason coming_soon (OK mart /
	// Hi-Life not verified) or temporarily_unavailable (CVS_ECPAY_ENABLED off with an enabled profile).
	PickupSelection string   `json:"pickup_selection,omitempty"`
	PaymentModes    []string `json:"payment_modes,omitempty"`
	StoreSearchURL  string   `json:"store_search_url,omitempty"`
	Available       *bool    `json:"available,omitempty"`
	Reason          string   `json:"reason,omitempty"`
	// TransferWindowHours (storefront-v2 §C) is present only when "bank_transfer" is in PaymentModes: how long the stock stays reserved
	// for the transfer. A home row carries PaymentModes only when the store enabled bank transfer (absent = card only, as before).
	TransferWindowHours int `json:"transfer_window_hours,omitempty"`
	// FreeShippingThresholdMinor (storefront-v2 §A/§C) is the row's delivery policy threshold: the merchandise subtotal at or above
	// which the quote charges shipping 0 (pricing.Calculate applies it; this field only lets the storefront word a hint). nil when the
	// policy has none, and also when it is 0 ("always free" needs no progress hint; the buyer UI accepts positive integers or null).
	// The quote stays the only authority on the amount charged (I05): a stale hint can mislead copy, never money.
	FreeShippingThresholdMinor *int64 `json:"free_shipping_threshold_minor"`
	// CodSurchargeMinor (home-cod R5, migration 0107) is present only on a home row whose PaymentModes includes "cash_on_delivery":
	// the store's whole-TWD COD surcharge in minor units the buyer pays on delivery on top of the order total (0 = no surcharge).
	// The quote stays the only authority on the order total (I05); this field only lets the storefront show the surcharge.
	CodSurchargeMinor int64 `json:"cod_surcharge_minor,omitempty"`
}

// transferOffer is checkout.read_transfer_offer: the store's bank-transfer switch, whether CVS destinations may use it, and the window.
// It never carries bank details (those are shown on the buyer's own order only).
type transferOffer struct {
	enabled, allowCVS bool
	windowHours       int
}

// codOffer is checkout.read_cod_offer (home-cod R5, migration 0107): the store's cash-on-delivery switch, the whole-TWD surcharge,
// the carrier label (manual fulfilment, never a carrier API) and the per-order cap. Home rows only; it never carries money details
// beyond the public offer.
type codOffer struct {
	enabled      bool
	surchargeTWD int
	carrier      string
	maxTWD       int
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

// This digest binds a cursor position to scope and filters; the capability and
// database RLS authenticate the read. It is deliberately not a signature.
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
		// Allocation.service_version is historical write-CAS provenance, not an
		// eligibility equality against the current (possibly renamed) service.
		// The count/all_active gate checks every assigned warehouse, not SKU stock.
		rows, err := tx.Query(callCtx, `SELECT m.id::text,m.code,m.name,sv.country,sv.currency,sv.code,sv.policy_method,
			sv.version,av.version,sv.delivery_kind,sv.mode,sv.name_hans,sv.name_hant,sv.name_en,sv.sort_order,
			pv.free_shipping_threshold_minor
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
			 AND sv.enabled AND sv.visible
			 AND ((sv.mode='MANUAL' AND sv.binding_id IS NULL AND sv.binding_version IS NULL)
			  OR (sv.mode='API' AND sv.binding_id IS NOT NULL AND sv.binding_version IS NOT NULL AND sv.delivery_kind LIKE 'cvs\_%'))
			 AND sv.delivery_kind IN ('home','cvs_711','cvs_familymart','cvs_hilife','cvs_okmart') AND (sv.delivery_kind='home' OR sv.country='TW')
			 AND av.warehouse_count BETWEEN 1 AND 16
			 AND ($3::uuid IS NULL OR m.id=$3::uuid) AND ($4::text IS NULL OR sv.country=$4::text)
			 AND ($5::uuid IS NULL OR (m.id,sv.country COLLATE "C",sv.code COLLATE "C") > ($5::uuid,$6::text COLLATE "C",$7::text COLLATE "C"))
			ORDER BY m.id,sv.country COLLATE "C",sv.code COLLATE "C" LIMIT $8`, scope.TenantID, scope.StoreID,
			marketFilter, countryFilter, afterMarket, afterCountry, afterCode, limit+1)
		if err != nil {
			return err
		}
		var fetched []Option
		for rows.Next() {
			var option Option
			if err = rows.Scan(&option.MarketID, &option.MarketCode, &option.MarketName, &option.Country, &option.Currency,
				&option.DeliveryCode, &option.Method, &option.ServiceVersion, &option.AllocationVersion,
				&option.DeliveryKind, &option.Mode, &option.NameHans, &option.NameHant, &option.NameEN, &option.SortOrder,
				&option.FreeShippingThresholdMinor); err != nil {
				rows.Close()
				return err
			}
			if option.FreeShippingThresholdMinor != nil && *option.FreeShippingThresholdMinor <= 0 {
				option.FreeShippingThresholdMinor = nil // 0 = always free: no hint to show (see the field comment)
			}
			fetched = append(fetched, option)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		// The cursor follows the FETCHED rows (position in the SQL order), so filtering CVS rows below never skips or repeats one.
		more := len(fetched) > limit
		if more {
			fetched = fetched[:limit]
		}
		var offer transferOffer
		if err = tx.QueryRow(callCtx, `SELECT enabled,allow_cvs,window_hours FROM checkout.read_transfer_offer($1,$2::uuid)`,
			tokenHash[:], storeID).Scan(&offer.enabled, &offer.allowCVS, &offer.windowHours); err != nil {
			return err
		}
		var cod codOffer
		if err = tx.QueryRow(callCtx, `SELECT enabled,surcharge_twd,carrier,max_twd FROM checkout.read_cod_offer($1,$2::uuid)`,
			tokenHash[:], storeID).Scan(&cod.enabled, &cod.surchargeTWD, &cod.carrier, &cod.maxTWD); err != nil {
			return err
		}
		items, err := s.decorateCVS(callCtx, tx, tokenHash[:], storeID, fetched, offer, cod)
		if err != nil {
			return err
		}
		if err = checkCapability(callCtx, tx, tokenHash[:], storeID, scope); err != nil {
			return err
		}
		page.Items = items
		if more && len(fetched) > 0 {
			page.NextCursor = encodeOptionsCursor(binding, fetched[len(fetched)-1])
		}
		return nil
	})
	if err != nil {
		return pagination.Page[Option]{Items: []Option{}}, safeError(ctx, err)
	}
	return page, nil
}

// Official chain store-search pages shown next to the code field of a buyer_entered store (§16.1, retrieved 2026-09-30). They are
// links the buyer opens in a new tab: this service never fetches, scrapes or calls any of them (AGENTS.md: no chain private endpoint).
// FamilyMart uses the store-number page that states the 6-digit format (https://www.family.com.tw/Marketing/zh/Map returned HTTP 403 to
// our fetcher on 2026-09-30).
var cvsSearchURLs = map[string]string{
	"cvs_711":        "https://emap.pcsc.com.tw/",
	"cvs_familymart": "https://family.map.com.tw/famiport/storeNumberFreeze.aspx",
	"cvs_hilife":     "https://www.hilife.com.tw/storeInquiry_street.aspx",
	"cvs_okmart":     "https://www.okmart.com.tw/convenient_shopSearch",
}

// decorateCVS adds the §5.1/§16.5 CVS fields to the fetched rows and drops the rows the store did not configure. Store mode and
// settings are read only through fulfillment.read_cvs_offer (no table access, buyer scope from the capability); non-CVS rows pass
// unchanged. CVS_ECPAY_ENABLED is the process kill switch: an ecpay_map store is then temporarily_unavailable, never buyer_entered.
// cod is the home cash-on-delivery offer (home-cod R5): a home row lists "cash_on_delivery" while the store enables it.
func (s *Service) decorateCVS(ctx context.Context, tx pgx.Tx, tokenHash []byte, storeID string, rows []Option, transfer transferOffer, cod codOffer) ([]Option, error) {
	items := make([]Option, 0, len(rows))
	hasCVS := false
	for _, option := range rows {
		hasCVS = hasCVS || option.DeliveryKind != "home"
	}
	if !hasCVS && !transfer.enabled && !cod.enabled {
		if s.noCard {
			return items, nil // home rows are card-only (OP1): nothing payable, nothing offered
		}
		return append(items, rows...), nil
	}
	var offer struct {
		selection string
		chains    []string
		pap       bool
		papMax    *int32
		ok, hl    bool
	}
	if hasCVS {
		if err := tx.QueryRow(ctx, `SELECT pickup_selection,enabled_chains,pay_at_pickup_enabled,pay_at_pickup_max_twd,ok_verified,hilife_verified
			FROM fulfillment.read_cvs_offer($1,$2::uuid)`, tokenHash, storeID).Scan(&offer.selection, &offer.chains, &offer.pap,
			&offer.papMax, &offer.ok, &offer.hl); err != nil {
			return nil, err
		}
		if offer.selection != "ecpay_map" && offer.selection != "buyer_entered" {
			return nil, command.ErrConflict
		}
	}
	ecpayOn := s.cvs != nil && s.cvs.cfg.ECPay.Enabled
	for _, option := range rows {
		if option.DeliveryKind == "home" {
			// Home rows are card-only unless the store enabled bank transfer or cash on delivery (home-cod R5); then they list their
			// modes like a CVS row does.
			if transfer.enabled || cod.enabled {
				option.PaymentModes = paymentModes(!s.noCard, false, transfer.enabled, cod.enabled)
			}
			if transfer.enabled {
				option.TransferWindowHours = transfer.windowHours
			}
			if cod.enabled {
				option.CodSurchargeMinor = int64(cod.surchargeTWD) * 100
			}
			if !s.noCard || transfer.enabled || cod.enabled {
				items = append(items, option)
			}
			continue
		}
		if !slices.Contains(offer.chains, option.DeliveryKind) {
			continue // the store did not configure this chain (C4)
		}
		// An API row is only meaningful in ecpay_map mode; a store that lost its profile falls back to its MANUAL rows only.
		if option.Mode == "API" && offer.selection != "ecpay_map" {
			continue
		}
		option.PickupSelection = offer.selection
		bank := transfer.enabled && transfer.allowCVS
		option.PaymentModes = paymentModes(!s.noCard, offer.pap && offer.papMax != nil, bank, false) // cash on delivery is home-only
		if len(option.PaymentModes) == 0 {
			continue // OP1: no payment mode can be completed, so the buyer is not offered this chain
		}
		if bank {
			option.TransferWindowHours = transfer.windowHours
		}
		if offer.selection == "buyer_entered" {
			option.StoreSearchURL = cvsSearchURLs[option.DeliveryKind]
		} else if reason := unavailableReason(option.DeliveryKind, offer.ok, offer.hl, ecpayOn); reason != "" {
			no := false
			option.Available, option.Reason = &no, reason
		}
		items = append(items, option)
	}
	return items, nil
}

// paymentModes lists what a buyer can complete on a row: card while the process can take it, pay_at_pickup / bank_transfer /
// cash_on_delivery when the store enabled them (the caller decides destination eligibility).
func paymentModes(card, payAtPickup, bankTransfer, cashOnDelivery bool) []string {
	modes := make([]string, 0, 4)
	if card {
		modes = append(modes, "card")
	}
	if payAtPickup {
		modes = append(modes, "pay_at_pickup")
	}
	if bankTransfer {
		modes = append(modes, "bank_transfer")
	}
	if cashOnDelivery {
		modes = append(modes, "cash_on_delivery")
	}
	return modes
}

// unavailableReason: OK mart needs ok_verified and Hi-Life hilife_verified (TD6, F17/F18); a disabled process kill switch makes every
// ecpay_map row temporarily unavailable.
func unavailableReason(kind string, okVerified, hilifeVerified, ecpayEnabled bool) string {
	switch {
	case !ecpayEnabled:
		return "temporarily_unavailable"
	case kind == "cvs_okmart" && !okVerified, kind == "cvs_hilife" && !hilifeVerified:
		return "coming_soon"
	}
	return ""
}
