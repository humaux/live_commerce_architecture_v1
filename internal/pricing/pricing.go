// Package pricing owns merchant market policy writes (markets and their currency) and the pure quote
// money calculation.
//
// It never reads a client-supplied amount, never touches stock or payment state, and never rounds
// money outside Calculate.
package pricing

import (
	"context"
	"errors"
	"math/bits"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

var (
	marketCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,39}$`)
	countryPattern    = regexp.MustCompile(`^[A-Z]{2}$`)
	currencyPattern   = regexp.MustCompile(`^[A-Z]{3}$`)
)

type Market struct {
	ID       string `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	Currency string `json:"currency"`
	Version  int64  `json:"version"`
	Active   bool   `json:"active"`
}

type MarketInput struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Currency string `json:"currency"`
}

type Policy struct {
	MarketID        string `json:"market_id"`
	Country         string `json:"country"`
	Method          string `json:"method"`
	Currency        string `json:"currency"`
	ShippingMode    string `json:"shipping_mode"`
	TaxMode         string `json:"tax_mode"`
	TaxBasis        string `json:"tax_basis"`
	Version         int64  `json:"version"`
	ShippingMinor   int64  `json:"shipping_minor"`
	TaxRateBPS      int64  `json:"tax_rate_bps"`
	QuoteTTLSeconds int64  `json:"quote_ttl_seconds"`
	Enabled         bool   `json:"enabled"`
	// FreeShippingThresholdMinor (storefront-v2 §C): merchandise subtotal at or above which Calculate charges shipping 0; nil = never.
	// It is part of the quote snapshot, so a quote keeps the outcome it was priced with. omitempty: a nil threshold leaves the key out, so a
	// quote snapshot stored before 0088 still round-trips byte-equal through checkout.begin_hold's "quote changed" comparison.
	FreeShippingThresholdMinor *int64 `json:"free_shipping_threshold_minor,omitempty"`
}

type PolicyInput struct {
	MarketID         string `json:"market_id"`
	Country          string `json:"country"`
	Method           string `json:"method"`
	Currency         string `json:"currency"`
	ShippingMode     string `json:"shipping_mode"`
	TaxMode          string `json:"tax_mode"`
	TaxBasis         string `json:"tax_basis"`
	ExpectedVersion  int64  `json:"expected_version"`
	ShippingMinor    *int64 `json:"shipping_minor"`
	TaxRateBPS       *int64 `json:"tax_rate_bps"`
	QuoteTTLSeconds  int64  `json:"quote_ttl_seconds"`
	Enabled          bool   `json:"enabled"`
	ConfigurationRef string `json:"configuration_ref"`
	// FreeShippingThresholdMinor: optional (absent or null = no threshold). omitempty keeps the request digest of a pre-0088 body unchanged.
	FreeShippingThresholdMinor *int64 `json:"free_shipping_threshold_minor,omitempty"`
}

type AmountLine struct {
	UnitPriceMinor int64 `json:"unit_price_minor"`
	Quantity       int64 `json:"quantity"`
}

type LineAmount struct {
	SubtotalMinor int64 `json:"subtotal_minor"`
	DiscountMinor int64 `json:"discount_minor"`
	TaxMinor      int64 `json:"tax_minor"`
	TotalMinor    int64 `json:"total_minor"`
}

type Calculation struct {
	SubtotalMinor    int64        `json:"subtotal_minor"`
	DiscountMinor    int64        `json:"discount_minor"`
	ShippingMinor    int64        `json:"shipping_minor"`
	ShippingTaxMinor int64        `json:"shipping_tax_minor"`
	TaxMinor         int64        `json:"tax_minor"`
	TotalMinor       int64        `json:"total_minor"`
	Lines            []LineAmount `json:"lines"`
}

func CreateMarket(ctx context.Context, tx pgx.Tx, scope platform.Scope, key string, in MarketInput) (out Market, err error) {
	if !validScope(tx, scope) || !marketCodePattern.MatchString(in.Code) || !printable(in.Name, 120) || !currencyPattern.MatchString(in.Currency) {
		return out, command.ErrInvalid
	}
	request := struct {
		PrincipalID string `json:"principal_id"`
		MarketInput
	}{scope.PrincipalID, in}
	err = command.Run(ctx, tx, scope, "pricing.market.create", key, request, &out, func() error {
		var storeCurrency string
		if err := tx.QueryRow(ctx, `SELECT currency FROM control.stores WHERE tenant_id=$1 AND id=$2`, scope.TenantID, scope.StoreID).Scan(&storeCurrency); err != nil {
			return mapError(err)
		}
		if storeCurrency != in.Currency {
			return command.ErrInvalid
		}
		err := tx.QueryRow(ctx, `INSERT INTO pricing.markets(tenant_id,store_id,code,name,currency,principal_id)
			VALUES($1,$2,$3,$4,$5,$6) RETURNING id::text,code,name,currency,version,active`,
			scope.TenantID, scope.StoreID, in.Code, in.Name, in.Currency, scope.PrincipalID).
			Scan(&out.ID, &out.Code, &out.Name, &out.Currency, &out.Version, &out.Active)
		if err != nil {
			return mapError(err)
		}
		return command.Audit(ctx, tx, scope, "pricing.market.created")
	})
	return out, mapError(err)
}

func SetMarketActive(ctx context.Context, tx pgx.Tx, scope platform.Scope, key, id string, expectedVersion int64, active bool) (out Market, err error) {
	if !validScope(tx, scope) || !command.ValidID(id) || expectedVersion < 1 {
		return out, command.ErrInvalid
	}
	request := struct {
		PrincipalID     string `json:"principal_id"`
		ID              string `json:"id"`
		ExpectedVersion int64  `json:"expected_version"`
		Active          bool   `json:"active"`
	}{scope.PrincipalID, id, expectedVersion, active}
	err = command.Run(ctx, tx, scope, "pricing.market.active", key, request, &out, func() error {
		var current Market
		err := tx.QueryRow(ctx, `SELECT id::text,code,name,currency,version,active FROM pricing.markets
			WHERE tenant_id=$1 AND store_id=$2 AND id=$3 FOR UPDATE`, scope.TenantID, scope.StoreID, id).
			Scan(&current.ID, &current.Code, &current.Name, &current.Currency, &current.Version, &current.Active)
		if err != nil {
			return mapError(err)
		}
		if current.Version != expectedVersion {
			return command.ErrConflict
		}
		err = tx.QueryRow(ctx, `UPDATE pricing.markets SET active=$4,version=version+1
			WHERE tenant_id=$1 AND store_id=$2 AND id=$3 AND version=$5
			RETURNING id::text,code,name,currency,version,active`, scope.TenantID, scope.StoreID, id, active, expectedVersion).
			Scan(&out.ID, &out.Code, &out.Name, &out.Currency, &out.Version, &out.Active)
		if err != nil {
			return mapVersionError(err)
		}
		return command.Audit(ctx, tx, scope, "pricing.market.active_changed")
	})
	return out, mapError(err)
}

func LockMarket(ctx context.Context, tx pgx.Tx, tenantID, storeID, id string) (out Market, err error) {
	if tx == nil || !command.ValidID(tenantID) || !command.ValidID(storeID) || !command.ValidID(id) {
		return out, command.ErrInvalid
	}
	err = tx.QueryRow(ctx, `SELECT id::text,code,name,currency,version,active FROM pricing.markets
		WHERE tenant_id=$1 AND store_id=$2 AND id=$3 FOR SHARE`, tenantID, storeID, id).
		Scan(&out.ID, &out.Code, &out.Name, &out.Currency, &out.Version, &out.Active)
	return out, mapError(err)
}

func SetPolicy(ctx context.Context, tx pgx.Tx, scope platform.Scope, key string, in PolicyInput) (out Policy, err error) {
	if !validScope(tx, scope) || !validPolicyInput(in) {
		return out, command.ErrInvalid
	}
	request := struct {
		PrincipalID string `json:"principal_id"`
		PolicyInput
	}{scope.PrincipalID, in}
	err = command.Run(ctx, tx, scope, "pricing.policy.set", key, request, &out, func() error {
		market, err := LockMarket(ctx, tx, scope.TenantID, scope.StoreID, in.MarketID)
		if err != nil {
			return err
		}
		if market.Currency != in.Currency {
			return command.ErrInvalid
		}
		if err = advisoryLock(ctx, tx, "pricing.policy|"+scope.TenantID+"|"+scope.StoreID+"|"+in.MarketID+"|"+in.Country+"|"+in.Method); err != nil {
			return err
		}
		var current int64
		err = tx.QueryRow(ctx, `SELECT current_version FROM pricing.policy_heads
			WHERE tenant_id=$1 AND store_id=$2 AND market_id=$3 AND country=$4 AND method=$5 FOR UPDATE`,
			scope.TenantID, scope.StoreID, in.MarketID, in.Country, in.Method).Scan(&current)
		switch {
		case errors.Is(err, pgx.ErrNoRows) && in.ExpectedVersion == 0:
			current = 0
		case err != nil:
			return mapError(err)
		case current != in.ExpectedVersion:
			return command.ErrConflict
		}
		out = Policy{MarketID: in.MarketID, Country: in.Country, Method: in.Method, Currency: in.Currency,
			ShippingMode: in.ShippingMode, TaxMode: in.TaxMode, TaxBasis: in.TaxBasis, Version: current + 1,
			ShippingMinor: *in.ShippingMinor, TaxRateBPS: *in.TaxRateBPS, QuoteTTLSeconds: in.QuoteTTLSeconds, Enabled: in.Enabled,
			FreeShippingThresholdMinor: in.FreeShippingThresholdMinor}
		_, err = tx.Exec(ctx, `INSERT INTO pricing.policy_versions(tenant_id,store_id,market_id,country,method,version,currency,
			shipping_mode,shipping_minor,tax_mode,tax_basis,tax_rate_bps,quote_ttl_seconds,enabled,configuration_ref,principal_id,
			free_shipping_threshold_minor)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`, scope.TenantID, scope.StoreID,
			out.MarketID, out.Country, out.Method, out.Version, out.Currency, out.ShippingMode, out.ShippingMinor,
			out.TaxMode, out.TaxBasis, out.TaxRateBPS, out.QuoteTTLSeconds, out.Enabled, in.ConfigurationRef, scope.PrincipalID,
			in.FreeShippingThresholdMinor)
		if err != nil {
			return mapError(err)
		}
		if current == 0 {
			_, err = tx.Exec(ctx, `INSERT INTO pricing.policy_heads(tenant_id,store_id,market_id,country,method,current_version)
				VALUES($1,$2,$3,$4,$5,$6)`, scope.TenantID, scope.StoreID, in.MarketID, in.Country, in.Method, out.Version)
		} else {
			var tag pgconn.CommandTag
			tag, err = tx.Exec(ctx, `UPDATE pricing.policy_heads SET current_version=$6
				WHERE tenant_id=$1 AND store_id=$2 AND market_id=$3 AND country=$4 AND method=$5 AND current_version=$7`,
				scope.TenantID, scope.StoreID, in.MarketID, in.Country, in.Method, out.Version, current)
			if err == nil && tag.RowsAffected() != 1 {
				return command.ErrConflict
			}
		}
		if err != nil {
			return mapError(err)
		}
		return command.Audit(ctx, tx, scope, "pricing.policy.set")
	})
	return out, mapError(err)
}

func LockCurrent(ctx context.Context, tx pgx.Tx, tenantID, storeID, marketID, country, method string) (out Policy, err error) {
	if tx == nil || !command.ValidID(tenantID) || !command.ValidID(storeID) || !command.ValidID(marketID) ||
		!countryPattern.MatchString(country) || !ValidMethod(method) {
		return out, command.ErrInvalid
	}
	var currentVersion int64
	err = tx.QueryRow(ctx, `SELECT current_version FROM pricing.policy_heads
		WHERE tenant_id=$1 AND store_id=$2 AND market_id=$3 AND country=$4 AND method=$5 FOR SHARE`,
		tenantID, storeID, marketID, country, method).Scan(&currentVersion)
	if err != nil {
		return out, mapError(err)
	}
	err = tx.QueryRow(ctx, `SELECT market_id::text,country,method,currency,shipping_mode,tax_mode,tax_basis,
		version,shipping_minor,tax_rate_bps,quote_ttl_seconds,enabled,free_shipping_threshold_minor FROM pricing.policy_versions
		WHERE tenant_id=$1 AND store_id=$2 AND market_id=$3 AND country=$4 AND method=$5 AND version=$6`,
		tenantID, storeID, marketID, country, method, currentVersion).Scan(
		&out.MarketID, &out.Country, &out.Method, &out.Currency, &out.ShippingMode, &out.TaxMode, &out.TaxBasis,
		&out.Version, &out.ShippingMinor, &out.TaxRateBPS, &out.QuoteTTLSeconds, &out.Enabled, &out.FreeShippingThresholdMinor)
	if err != nil {
		return out, mapError(err)
	}
	if !out.Enabled {
		return Policy{}, command.ErrNotFound
	}
	return out, nil
}

// Promo is the frozen effect of one discount code (storefront-v2 section F). ID/Code/Version identify the merchant's code row at pricing
// time (BeginCheckout compares Version under a lock, so an edit or pause after the quote is a typed refusal); Kind/Percent/FixedMinor are
// all Calculate reads. omitempty on the numbers keeps a snapshot of the other kind byte-stable through checkout.begin_hold's comparison.
type Promo struct {
	ID         string `json:"id"`
	Code       string `json:"code"`
	Version    int64  `json:"version"`
	Kind       string `json:"kind"`
	Percent    int64  `json:"percent,omitempty"`
	FixedMinor int64  `json:"fixed_minor,omitempty"`
}

// wholeStep is the smallest chargeable step of a currency in minor units. TWD is charged in whole dollars (stripe-psp-v1 D15 and the section 4
// table: TWD 2500..99999900 step 100; PAYUNi amount%100=0), so a discount must not leave a fractional-dollar total that payment start would
// refuse after the order was already placed. Every other currency of the closed allowlist charges at its minor unit.
func wholeStep(currency string) int64 {
	if currency == "TWD" {
		return 100
	}
	return 1
}

// discountOn is the ONE place a code becomes money. I05: integer minor units only, floor rounding for percent, rounded DOWN to the currency's
// chargeable step (whole dollars for TWD), and the result never exceeds the merchandise subtotal, so the goods charge cannot go below zero.
// Shipping is not an input: a code never discounts shipping.
func (p Promo) discountOn(subtotal, step int64) (int64, error) {
	if subtotal < 0 || subtotal > command.MaxMoney || step < 1 {
		return 0, command.ErrInvalid
	}
	var discount int64
	switch p.Kind {
	case "percent":
		if p.Percent < 1 || p.Percent > 90 || p.FixedMinor != 0 {
			return 0, command.ErrInvalid
		}
		discount = subtotal * p.Percent / 100 // subtotal <= 1e12 and percent <= 90: no overflow
	case "fixed":
		if p.FixedMinor < 1 || p.FixedMinor > command.MaxMoney || p.Percent != 0 {
			return 0, command.ErrInvalid
		}
		discount = min(p.FixedMinor, subtotal)
	default:
		return 0, command.ErrInvalid
	}
	return discount - discount%step, nil
}

// allocateDiscount splits discount over the line subtotals proportionally (floor, then the leftover units one by one to the largest
// fractional remainders, ties to the lower index) so the line discounts sum to exactly discount and none exceeds its line. 128-bit
// product via math/bits: discount <= total <= 1e12 makes discount*line overflow int64.
func allocateDiscount(discount int64, subtotals []int64, total int64) []int64 {
	out := make([]int64, len(subtotals))
	if discount == 0 || total == 0 {
		return out
	}
	rems := make([]uint64, len(subtotals))
	var given int64
	for i, sub := range subtotals {
		hi, lo := bits.Mul64(uint64(discount), uint64(sub))
		q, r := bits.Div64(hi, lo, uint64(total)) // hi < total because discount <= total
		out[i], rems[i] = int64(q), r
		given += int64(q)
	}
	for left := discount - given; left > 0; left-- {
		best := -1
		for i := range rems {
			if out[i] < subtotals[i] && (best < 0 || rems[i] > rems[best]) {
				best = i
			}
		}
		out[best]++
		rems[best] = 0
	}
	return out
}

// Calculate prices a cart without a discount code; see CalculateWith.
func Calculate(policy Policy, lines []AmountLine) (Calculation, error) {
	return CalculateWith(policy, lines, nil)
}

// CalculateWith is the pure quote calculator. promo == nil is byte-identical to the pre-0091 calculation (discount 0).
func CalculateWith(policy Policy, lines []AmountLine, promo *Promo) (Calculation, error) {
	result := Calculation{ShippingMinor: policy.ShippingMinor, Lines: make([]LineAmount, 0, len(lines))}
	if !validPolicy(policy) || len(lines) == 0 || len(lines) > 50 {
		return Calculation{}, command.ErrInvalid
	}
	subtotals := make([]int64, len(lines))
	for i, line := range lines {
		subtotal, err := command.CheckMoney(line.UnitPriceMinor, line.Quantity)
		if err != nil {
			return Calculation{}, err
		}
		subtotals[i] = subtotal
		if result.SubtotalMinor, err = addMoney(result.SubtotalMinor, subtotal); err != nil {
			return Calculation{}, err
		}
	}
	if promo != nil {
		var err error
		if result.DiscountMinor, err = promo.discountOn(result.SubtotalMinor, wholeStep(policy.Currency)); err != nil {
			return Calculation{}, err
		}
	}
	shares := allocateDiscount(result.DiscountMinor, subtotals, result.SubtotalMinor)
	for i, subtotal := range subtotals {
		net := subtotal - shares[i] // >= 0: a share never exceeds its line
		tax := taxMinor(net, policy.TaxRateBPS, policy.TaxMode)
		lineTotal := net
		var err error
		if policy.TaxMode == "exclusive" {
			if lineTotal, err = addMoney(net, tax); err != nil {
				return Calculation{}, err
			}
		}
		if result.TaxMinor, err = addMoney(result.TaxMinor, tax); err != nil {
			return Calculation{}, err
		}
		result.Lines = append(result.Lines, LineAmount{SubtotalMinor: subtotal, DiscountMinor: shares[i], TaxMinor: tax, TotalMinor: lineTotal})
	}
	shipping := policy.ShippingMinor
	// I05 / storefront-v2 §C: free shipping is decided here, from the server-side merchandise subtotal (BEFORE any code discount, section F)
	// and the policy's threshold, never from a client value. Shipping 0 also zeroes the shipping tax below because both read this variable.
	if policy.FreeShippingThresholdMinor != nil && result.SubtotalMinor >= *policy.FreeShippingThresholdMinor {
		shipping = 0
	}
	result.ShippingMinor = shipping
	if policy.TaxBasis == "goods_and_shipping" {
		result.ShippingTaxMinor = taxMinor(shipping, policy.TaxRateBPS, policy.TaxMode)
	}
	var err error
	result.TaxMinor, err = addMoney(result.TaxMinor, result.ShippingTaxMinor)
	if err != nil {
		return Calculation{}, err
	}
	// I05: total = goods after discount + shipping (+ tax when exclusive); the discount is subtracted from goods only.
	total, err := addMoney(result.SubtotalMinor-result.DiscountMinor, shipping)
	if err != nil {
		return Calculation{}, err
	}
	if policy.TaxMode == "exclusive" {
		if total, err = addMoney(total, result.TaxMinor); err != nil {
			return Calculation{}, err
		}
	}
	result.TotalMinor = total
	return result, nil
}

func validPolicyInput(in PolicyInput) bool {
	return command.ValidID(in.MarketID) && in.ExpectedVersion >= 0 && in.ShippingMinor != nil && in.TaxRateBPS != nil &&
		currencyPattern.MatchString(in.Currency) && countryPattern.MatchString(in.Country) && ValidMethod(in.Method) &&
		in.ShippingMode == "country_flat" && validTax(in.TaxMode, in.TaxBasis, *in.TaxRateBPS) &&
		*in.ShippingMinor >= 0 && *in.ShippingMinor <= command.MaxMoney && in.QuoteTTLSeconds >= 60 && in.QuoteTTLSeconds <= 1800 &&
		printable(in.ConfigurationRef, 240) &&
		(in.FreeShippingThresholdMinor == nil || (*in.FreeShippingThresholdMinor >= 0 && *in.FreeShippingThresholdMinor <= command.MaxMoney))
}

func validPolicy(p Policy) bool {
	return command.ValidID(p.MarketID) && p.Version > 0 && p.Enabled && currencyPattern.MatchString(p.Currency) &&
		countryPattern.MatchString(p.Country) && ValidMethod(p.Method) && p.ShippingMode == "country_flat" &&
		p.ShippingMinor >= 0 && p.ShippingMinor <= command.MaxMoney && validTax(p.TaxMode, p.TaxBasis, p.TaxRateBPS) &&
		p.QuoteTTLSeconds >= 60 && p.QuoteTTLSeconds <= 1800 &&
		(p.FreeShippingThresholdMinor == nil || (*p.FreeShippingThresholdMinor >= 0 && *p.FreeShippingThresholdMinor <= command.MaxMoney))
}

func validTax(mode, basis string, bps int64) bool {
	return (basis == "goods" || basis == "goods_and_shipping") && bps >= 0 && bps <= 10000 &&
		(mode == "exclusive" || mode == "inclusive" || mode == "none") && (mode != "none" || bps == 0)
}

// ValidMethod reports whether method is a supported legacy or delivery-service key.
func ValidMethod(method string) bool {
	if method == "home" || method == "cvs_711" || method == "cvs_familymart" {
		return true
	}
	code, ok := strings.CutPrefix(method, "delivery:")
	return ok && marketCodePattern.MatchString(code)
}

// DeliveryMethod returns the reserved pricing key for one delivery service.
func DeliveryMethod(code string) (string, error) {
	if !marketCodePattern.MatchString(code) {
		return "", command.ErrInvalid
	}
	return "delivery:" + code, nil
}

func printable(value string, max int) bool {
	length := utf8.RuneCountInString(value)
	if length < 1 || length > max || !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

func validScope(tx pgx.Tx, scope platform.Scope) bool {
	return tx != nil && command.ValidID(scope.TenantID) && command.ValidID(scope.StoreID) && command.ValidID(scope.PrincipalID)
}

func advisoryLock(ctx context.Context, tx pgx.Tx, key string) error {
	var lockTimeout string
	if err := tx.QueryRow(ctx, `SHOW lock_timeout`).Scan(&lockTimeout); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('lock_timeout','0',true)`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, key); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `SELECT set_config('lock_timeout',$1,true)`, lockTimeout)
	return err
}

func taxMinor(base, bps int64, mode string) int64 {
	if mode == "none" || bps == 0 {
		return 0
	}
	if mode == "inclusive" {
		net := (base*10000 + (10000+bps)/2) / (10000 + bps)
		return base - net
	}
	return (base*bps + 5000) / 10000
}

func addMoney(a, b int64) (int64, error) {
	if a < 0 || b < 0 || a > command.MaxMoney-b {
		return 0, command.ErrInvalid
	}
	return a + b, nil
}

func mapVersionError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return command.ErrConflict
	}
	return mapError(err)
}

func mapError(err error) error {
	if err == nil || errors.Is(err, command.ErrInvalid) || errors.Is(err, command.ErrConflict) || errors.Is(err, command.ErrNotFound) {
		return err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return command.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505", "40001":
			return command.ErrConflict
		case "23503", "23514", "22P02":
			return command.ErrInvalid
		}
	}
	return err
}
