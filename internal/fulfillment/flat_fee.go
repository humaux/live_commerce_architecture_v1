// Purpose: Publish a merchant flat-fee change and its delivery service binding in one transaction.
// Depends on: pricing.SetFlatFee, SetService, platform grants and the scoped command journal.
// Used by: the settings flat-fee PUT. Readers see either complete version pair.
package fulfillment

import (
	"context"
	"crypto/rand"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
	"livecommerce/internal/pricing"
)

type FlatFeeInput struct {
	ShippingMinor          *int64 `json:"shipping_minor"`
	ExpectedPolicyVersion  int64  `json:"expected_policy_version"`
	ExpectedServiceVersion int64  `json:"expected_service_version"`
}
type FlatFee struct {
	MarketID       string `json:"market_id"`
	Country        string `json:"country"`
	Code           string `json:"code"`
	ShippingMinor  int64  `json:"shipping_minor"`
	PolicyVersion  int64  `json:"policy_version"`
	ServiceVersion int64  `json:"service_version"`
}

func SetFlatFee(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, marketID, country, code string, in FlatFeeInput) (out FlatFee, err error) {
	if !command.ValidID(marketID) || !codePattern.MatchString(code) || !countryPattern.MatchString(country) || in.ExpectedPolicyVersion < 1 || in.ExpectedServiceVersion < 1 || in.ShippingMinor == nil || *in.ShippingMinor < 0 || *in.ShippingMinor > command.MaxMoney {
		return out, command.ErrInvalid
	}
	check := func() error {
		if e := authorize(ctx, tx, scope, token, managePermission); e != nil {
			return e
		}
		return platform.RequirePermission(ctx, tx, scope, token, "pricing:write")
	}
	if err = check(); err != nil {
		return FlatFee{}, err
	}
	request := struct {
		Principal, MarketID, Country, Code string
		FlatFeeInput
	}{scope.PrincipalID, marketID, country, code, in}
	err = command.Run(ctx, tx, scope, "fulfillment.service.flat_fee", key, request, &out, func() error {
		s, exists, e := readCurrentService(ctx, tx, scope, marketID, country, code)
		if e != nil {
			return e
		}
		if !exists {
			return command.ErrNotFound
		}
		if s.Version != in.ExpectedServiceVersion || s.PolicyVersion != in.ExpectedPolicyVersion {
			return command.ErrConflict
		}
		// Child commands have private keys. Only the outer operation can replay the pair.
		p, e := pricing.SetFlatFee(ctx, tx, scope, rand.Text(), marketID, country, code, in.ExpectedPolicyVersion, *in.ShippingMinor)
		if e != nil {
			return e
		}
		s, e = SetService(ctx, tx, scope, token, rand.Text(), ServiceInput{MarketID: marketID, Country: country, Code: code, ExpectedVersion: in.ExpectedServiceVersion,
			PolicyVersion: p.Version, NameHans: s.NameHans, NameHant: s.NameHant, NameEN: s.NameEN, DeliveryKind: s.DeliveryKind, Mode: s.Mode, Enabled: s.Enabled, Visible: s.Visible, SortOrder: s.SortOrder, BindingID: s.BindingID, BindingVersion: s.BindingVersion})
		if e != nil {
			return e
		}
		out = FlatFee{marketID, country, code, p.ShippingMinor, p.Version, s.Version}
		return nil
	})
	if err != nil {
		return FlatFee{}, err
	}
	if err = check(); err != nil {
		return FlatFee{}, err
	}
	return out, nil
}
