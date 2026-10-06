// Purpose: the operator steps of platform Stripe (contracts/stripe-platform-account-v1.md §3.3, §7, owner amendment AD-PF2):
// designate the platform connection, open/close new enrollments, allowlist a store, and block/unblock one store (kill switch).
// Each is one call to a registry SQL definer scoped to the PLATFORM store; none takes a secret and none calls Stripe.
// Depends on: SQL payments.designate_stripe_platform, set_stripe_platform_open, allow_platform_stripe, block_platform_stripe
//   (0137; owner registry writer, EXECUTE commerce_payment_registrar only).
// Used by: cmd/stripe-admin platform-designate | platform-open | platform-close | platform-allow | platform-disallow |
//   platform-block | platform-unblock.
// Invariants: I16 (the kill switches stop new starts only); errors stay ErrConfig/ErrDatabase/ErrRejected.
// Status: MOCK + REAL_PG.

package stripeadmin

import (
	"context"
	"regexp"
	"time"

	"livecommerce/internal/command"
)

var (
	displayNamePattern = regexp.MustCompile(`^[^\x00-\x1f\x7f<>]{2,60}$`)
	descriptorPattern  = regexp.MustCompile(`^[A-Za-z0-9 .*-]{5,22}$`)
	termsPattern       = regexp.MustCompile(`^[a-z0-9.-]{3,40}$`)
	operatorPattern    = regexp.MustCompile(`^[A-Za-z0-9._:@-]{2,64}$`)
)

// PlatformDesignateInput marks one PRIMARY Stripe connection of the scope store as the environment platform connection.
type PlatformDesignateInput struct {
	ConnectionID, DisplayName, DescriptorDisplay, TermsVersion string
	ExpectedVersion                                            int64 // 0 on first designation
}

// PlatformDesignate calls payments.designate_stripe_platform and returns the new row version.
func (r *Registrar) PlatformDesignate(ctx context.Context, s Scope, in PlatformDesignateInput) (int64, error) {
	if r == nil || r.db == nil || ctx == nil || !validScope(s) || !command.ValidID(in.ConnectionID) || in.ExpectedVersion < 0 {
		return 0, ErrConfig
	}
	if !displayNamePattern.MatchString(in.DisplayName) || !descriptorPattern.MatchString(in.DescriptorDisplay) ||
		!termsPattern.MatchString(in.TermsVersion) {
		return 0, ErrRejected // the definer answers 22023 for the same grammar
	}
	var v int64
	// payments.designate_stripe_platform: primary connection only; refuses a move while enrollments exist (PT409).
	if err := r.scan(ctx, &v, `SELECT payments.designate_stripe_platform($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::text,$6::text,$7::text,$8::bigint)`,
		s.TenantID, s.StoreID, s.PrincipalID, in.ConnectionID, in.DisplayName, in.DescriptorDisplay, in.TermsVersion, in.ExpectedVersion); err != nil {
		return 0, err
	}
	return v, nil
}

// PlatformOpen opens or closes NEW enrollments (existing stores keep selling). feeBPS < 0 leaves the fee unchanged.
// LIVE open needs the verified canary and the four platform attestation codes (checked in SQL).
func (r *Registrar) PlatformOpen(ctx context.Context, s Scope, environment string, open bool, feeBPS int, expected int64) (int64, error) {
	if r == nil || r.db == nil || ctx == nil || !validScope(s) || expected < 1 || feeBPS > 3000 ||
		(environment != envSandbox && environment != envLive) {
		return 0, ErrConfig
	}
	var fee any
	if feeBPS >= 0 {
		fee = int32(feeBPS)
	}
	var v int64
	// payments.set_stripe_platform_open: CAS on the platform row version.
	if err := r.scan(ctx, &v, `SELECT payments.set_stripe_platform_open($1::uuid,$2::uuid,$3::uuid,$4::text,$5::boolean,$6::integer,$7::bigint)`,
		s.TenantID, s.StoreID, s.PrincipalID, environment, open, fee, expected); err != nil {
		return 0, err
	}
	return v, nil
}

// PlatformBlock is the per-store kill switch (blocked=true) or its release (blocked=false). ref is the operator ticket.
// It never needs the LIVE flag+reference pair: stopping a store must not depend on deploy env. Block revokes the target's
// derived qualification and disables its card method; unblock clears the flag only (the merchant then enables again).
func (r *Registrar) PlatformBlock(ctx context.Context, s Scope, targetTenant, targetStore, environment string, blocked bool,
	operator, ref string) (time.Time, error) {
	if r == nil || r.db == nil || ctx == nil || !validScope(s) || !command.ValidID(targetTenant) || !command.ValidID(targetStore) ||
		(environment != envSandbox && environment != envLive) {
		return time.Time{}, ErrConfig
	}
	if !operatorPattern.MatchString(operator) || (blocked && !refPattern.MatchString(ref)) {
		return time.Time{}, ErrRejected
	}
	var refArg any
	if ref != "" {
		refArg = ref
	}
	var at time.Time
	// payments.block_platform_stripe: target-scope row writes, audit in the platform scope with the target ids in details.
	if err := r.scan(ctx, &at, `SELECT payments.block_platform_stripe($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::uuid,$6::text,$7::boolean,$8::text,$9::text)`,
		s.TenantID, s.StoreID, s.PrincipalID, targetTenant, targetStore, environment, blocked, operator, refArg); err != nil {
		return time.Time{}, err
	}
	return at, nil
}

// PlatformAllow allowlists (allowed=true) or withdraws a store for platform-Stripe self-enable (AD-PF2: only the platform
// owner's own stores; third-party merchants stay refused). Withdrawing does not stop sales: use PlatformBlock.
func (r *Registrar) PlatformAllow(ctx context.Context, s Scope, targetTenant, targetStore, environment string, allowed bool,
	operator, ref string) (int64, error) {
	if r == nil || r.db == nil || ctx == nil || !validScope(s) || !command.ValidID(targetTenant) || !command.ValidID(targetStore) ||
		(environment != envSandbox && environment != envLive) {
		return 0, ErrConfig
	}
	if !operatorPattern.MatchString(operator) || !refPattern.MatchString(ref) {
		return 0, ErrRejected
	}
	var v int64
	// payments.allow_platform_stripe: audited allowlist row in the target scope.
	if err := r.scan(ctx, &v, `SELECT payments.allow_platform_stripe($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::uuid,$6::text,$7::boolean,$8::text,$9::text)`,
		s.TenantID, s.StoreID, s.PrincipalID, targetTenant, targetStore, environment, allowed, operator, ref); err != nil {
		return 0, err
	}
	return v, nil
}
