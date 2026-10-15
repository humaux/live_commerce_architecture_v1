// buyer_payment.go owns the API-side configuration and assembly of the hosted buyer payment
// service (Stripe Checkout when COMMERCE_STRIPE_CHECKOUT_ENABLED=1). It never
// reads STRIPE_* secrets, starts a worker, or contacts a provider from the API process.

package main

import (
	"context"
	"net"
	"net/url"
	"path"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"livecommerce/internal/checkout"
	"livecommerce/internal/platform"
)

type buyerPaymentConfig struct {
	enabled   bool
	hostedDSN string
	profile   string
	stripe    *checkout.StripeHostedConfig // nil unless COMMERCE_STRIPE_CHECKOUT_ENABLED=1
}

// A disabled nested feature reads only its flag. The root buyer loader calls
// this only after the buyer listener and its own configuration are admitted.
func loadBuyerPaymentConfig(getenv func(string) string, addr string) (buyerPaymentConfig, error) {
	var c buyerPaymentConfig
	enabled, err := flag(getenv("COMMERCE_BUYER_PAYMENT_ENABLED"))
	if err != nil {
		return c, errBuyerConfig
	}
	if !enabled {
		return c, nil
	}
	if !privateIdentityAddress(addr) {
		return c, errBuyerConfig
	}
	c.hostedDSN = getenv("COMMERCE_HOSTED_DATABASE_URL")
	c.profile = getenv("COMMERCE_PAYMENT_PROFILE")
	if strings.TrimSpace(c.hostedDSN) == "" ||
		(c.profile != "PROVIDER_MOCK" && c.profile != "SANDBOX" && c.profile != "LIVE") {
		return buyerPaymentConfig{}, errBuyerConfig
	}
	// Stripe keys remain in PG under the operator registrar; this API only needs a return URL.
	// LD6: this flag is the platform-wide kill switch (LC_STRIPE_CHECKOUT_ENABLED in compose). On LIVE it also
	// needs the owner's flag+reference pair (stripe-live-enable-v1 §5.2); the pair is read only in that case.
	stripeEnabled, err := flag(getenv("COMMERCE_STRIPE_CHECKOUT_ENABLED"))
	if err != nil {
		return buyerPaymentConfig{}, errBuyerConfig
	}
	if stripeEnabled && c.profile == "LIVE" {
		if _, err := loadStripeLiveApproval(getenv); err != nil {
			return buyerPaymentConfig{}, errBuyerConfig
		}
	}
	if stripeEnabled {
		returnURL, ok := canonicalBuyerPaymentReturnURL(getenv("COMMERCE_PAYMENT_RETURN_URL"))
		if !ok {
			return buyerPaymentConfig{}, errBuyerConfig
		}
		stripeConfig := checkout.StripeHostedConfig{ReturnURL: returnURL}
		if _, _, err := stripeConfig.CanonicalDigest(); err != nil {
			return buyerPaymentConfig{}, errBuyerConfig
		}
		c.stripe = &stripeConfig
	}
	c.enabled = true
	return c, nil
}

func buildBuyerPayment(ctx context.Context, c buyerPaymentConfig) (*checkout.HostedPaymentStarter, *pgxpool.Pool, error) {
	if !c.enabled {
		return nil, nil, nil
	}
	if ctx == nil || strings.TrimSpace(c.hostedDSN) == "" || !validBuyerPaymentProfile(c.profile) {
		return nil, nil, errBuyerConfig
	}
	pool, err := platform.OpenHostedPool(ctx, c.hostedDSN)
	if err != nil {
		return nil, nil, errBuyerConfig
	}
	// This client inserts payment query/signal jobs in the payment transaction. It
	// does not start a worker or make a provider request in the API process.
	// Place it in the family schema maintained only by payment workers.
	jobs, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Schema: "river_payment"})
	if err != nil {
		pool.Close()
		return nil, nil, errBuyerConfig
	}
	service, err := checkout.NewHostedPaymentService(ctx, pool, jobs, c.profile, nil,
		checkout.HostedProviders{Stripe: c.stripe})
	if err != nil {
		pool.Close()
		return nil, nil, errBuyerConfig
	}
	return service, pool, nil
}

func validBuyerPaymentProfile(profile string) bool {
	return profile == "PROVIDER_MOCK" || profile == "SANDBOX" || profile == "LIVE"
}

// canonicalBuyerPaymentReturnURL keeps the established Stripe return-page normalization.
func canonicalBuyerPaymentReturnURL(raw string) (string, bool) {
	if len(raw) == 0 || len(raw) > 2048 || strings.ContainsAny(raw, " \r\n\t\\%#") {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Opaque != "" || u.Host == "" || u.User != nil ||
		u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawFragment != "" || u.RawPath != "" ||
		u.Path == "" || !strings.HasPrefix(u.Path, "/") || path.Clean(u.Path) != u.Path {
		return "", false
	}
	host := strings.ToLower(u.Hostname())
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") ||
		net.ParseIP(host) != nil || !strings.Contains(host, ".") ||
		(strings.ToLower(u.Host) != host && strings.ToLower(u.Host) != host+":443") {
		return "", false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", false
		}
		for _, ch := range label {
			if ch != '-' && (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') {
				return "", false
			}
		}
	}
	last := host[strings.LastIndexByte(host, '.')+1:]
	if len(last) < 2 {
		return "", false
	}
	for _, ch := range last {
		if ch < 'a' || ch > 'z' {
			return "", false
		}
	}
	u.Host = host
	return u.String(), true
}
