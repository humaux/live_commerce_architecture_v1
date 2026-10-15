// hosted.go owns Stripe-only payment admission and read-only historical projections.
// Depends on buyer scope, the hosted PG role and River inserts; called by buyerhttp.
// Removed providers never prepare a form, enqueue a start or release a handoff.
package checkout

import (
	"context"
	"crypto/sha256"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"livecommerce/internal/buyer"
	"livecommerce/internal/command"
	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/platform"
)

// HostedConfig retains the retired constructor input for historical-view callers.
// Neither endpoint is read, hashed, persisted or dispatched.
type HostedConfig struct{ ReturnURL, NotifyURL string }

type HostedInput struct {
	OrderID       string `json:"order_id"`
	MethodCode    string `json:"method_code"`
	MethodVersion int64  `json:"method_version"`
	Locale        string `json:"locale"`
}

type HostedForm struct {
	Action string            `json:"action"`
	Fields map[string]string `json:"fields"`
}

// HostedHandoff carries a Stripe redirect release. A Stripe
// RedirectURL is a live Checkout capability: it is never logged or persisted here.
type HostedHandoff struct {
	OrderID     string      `json:"order_id"`
	Disposition string      `json:"disposition"`
	ExpiresAt   time.Time   `json:"expires_at"`
	Form        *HostedForm `json:"form,omitempty"`
	RedirectURL string      `json:"redirect_url,omitempty"`
}

type HostedPaymentStarter struct {
	starter      PaymentStarter
	stripe       *StripeHostedConfig
	stripeDigest [32]byte
}

// HostedProviders admits only Stripe; an empty value creates a historical read-only service.
type HostedProviders struct {
	PAYUNi *HostedConfig // removed provider — read-only, never dispatched; ignored compatibility input
	Stripe *StripeHostedConfig
}

// NewHostedPaymentStarter is a retired execution entry point, retained for historical views only.
func NewHostedPaymentStarter(ctx context.Context, hostedPool *pgxpool.Pool, jobs *river.Client[pgx.Tx],
	profile string, keys *accounts.Keyring, config HostedConfig) (*HostedPaymentStarter, error) {
	return NewHostedPaymentService(ctx, hostedPool, jobs, profile, keys, HostedProviders{})
}

func NewHostedPaymentService(ctx context.Context, hostedPool *pgxpool.Pool, jobs *river.Client[pgx.Tx],
	profile string, keys *accounts.Keyring, p HostedProviders) (*HostedPaymentStarter, error) {
	if ctx == nil || jobs == nil || !validPaymentProfile(profile) {
		return nil, command.ErrInvalid
	}
	out := &HostedPaymentStarter{starter: PaymentStarter{pool: hostedPool, jobs: jobs, profile: profile}}
	if p.Stripe != nil {
		canonical, digest, err := p.Stripe.CanonicalDigest()
		// stripe-live-enable-v1 §5.2: the Stripe branch admits every valid profile (validPaymentProfile above), LIVE
		// included. The owner's flag+reference pair is enforced by the caller (cmd/api loadBuyerPaymentConfig), and the
		// per-store gate is SQL: hosted_payment_view_v2 / start_stripe_payment need a REAL_LIVE, unrevoked qualification.
		if err != nil {
			return nil, command.ErrInvalid
		}
		out.stripe, out.stripeDigest = &canonical, digest
	}
	if err := platform.ValidateHostedPool(ctx, hostedPool); err != nil {
		return nil, err
	}
	return out, nil
}

// BeginHosted starts only Stripe. Removed method codes fail before any transaction or job insert.
func (s *HostedPaymentStarter) BeginHosted(ctx context.Context, token, storeID, key string, in HostedInput) (PaymentResult, error) {
	if ctx == nil || s == nil || s.starter.pool == nil || s.starter.jobs == nil ||
		!validPaymentProfile(s.starter.profile) || !checkoutKey.MatchString(key) || !validHostedInput(in) {
		return PaymentResult{}, command.ErrInvalid
	}
	return s.beginStripe(ctx, token, storeID, key, in)
}

// TakeHosted releases only a Stripe redirect. Historical attempts are never resumed.
func (s *HostedPaymentStarter) TakeHosted(ctx context.Context, token, storeID, orderID string) (HostedHandoff, error) {
	if ctx == nil || s == nil || s.starter.pool == nil || !validPaymentProfile(s.starter.profile) || !command.ValidID(orderID) {
		return HostedHandoff{}, command.ErrInvalid
	}
	tokenHash := sha256.Sum256([]byte(token))
	var out HostedHandoff
	err := buyer.WithScope(ctx, s.starter.pool, token, storeID, func(callCtx context.Context, tx pgx.Tx, scope buyer.Scope) error {
		if s.stripe != nil {
			stripeOrder, err := s.isStripeOrder(callCtx, tx, tokenHash[:], storeID, orderID)
			if err != nil {
				return err
			}
			if stripeOrder {
				return s.takeStripeTx(callCtx, tx, scope, tokenHash[:], storeID, orderID, &out)
			}
		}
		return command.ErrConflict
	})
	if err != nil {
		return HostedHandoff{}, safeError(ctx, err)
	}
	return out, nil
}

func validHostedInput(in HostedInput) bool {
	return command.ValidID(in.OrderID) && in.MethodCode == stripeMethodCode && in.MethodVersion > 0 &&
		(in.Locale == "zh-CN" || in.Locale == "zh-TW" || in.Locale == "en")
}

func checkHostedDeadline(ctx context.Context, tx pgx.Tx, deadline time.Time) error {
	var current bool
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp() < $1::timestamptz`, deadline).Scan(&current); err != nil {
		return err
	}
	if !current {
		return command.ErrConflict
	}
	return nil
}
