package main

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"livecommerce/internal/checkout"
	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/platform"
)

type buyerPaymentConfig struct {
	enabled   bool
	hostedDSN string
	profile   string
	endpoints checkout.HostedConfig
	keys      *accounts.Keyring
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
	c.endpoints = checkout.HostedConfig{ReturnURL: getenv("COMMERCE_PAYMENT_RETURN_URL"),
		NotifyURL: getenv("COMMERCE_PAYMENT_NOTIFY_URL")}
	if strings.TrimSpace(c.hostedDSN) == "" ||
		(c.profile != "PROVIDER_MOCK" && c.profile != "SANDBOX" && c.profile != "LIVE") {
		return buyerPaymentConfig{}, errBuyerConfig
	}
	canonical, _, err := c.endpoints.CanonicalDigest()
	if err != nil {
		return buyerPaymentConfig{}, errBuyerConfig
	}
	c.endpoints = canonical
	c.keys, err = loadAccountKeys(getenv)
	if err != nil {
		return buyerPaymentConfig{}, errBuyerConfig
	}
	c.enabled = true
	return c, nil
}

func buildBuyerPayment(ctx context.Context, c buyerPaymentConfig) (*checkout.HostedPaymentStarter, *pgxpool.Pool, error) {
	if !c.enabled {
		return nil, nil, nil
	}
	if ctx == nil || c.keys == nil {
		return nil, nil, errBuyerConfig
	}
	pool, err := platform.OpenHostedPool(ctx, c.hostedDSN)
	if err != nil {
		return nil, nil, errBuyerConfig
	}
	// This client inserts an existing query job in the payment transaction. It
	// does not start a worker or make a provider request in the API process.
	jobs, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Schema: "river"})
	if err != nil {
		pool.Close()
		return nil, nil, errBuyerConfig
	}
	service, err := checkout.NewHostedPaymentStarter(ctx, pool, jobs, c.profile, c.keys, c.endpoints)
	if err != nil {
		pool.Close()
		return nil, nil, errBuyerConfig
	}
	return service, pool, nil
}
