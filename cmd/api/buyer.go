package main

import (
	"context"
	"errors"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"livecommerce/internal/buyerhttp"
	"livecommerce/internal/checkout"
	"livecommerce/internal/identityhttp"
	"livecommerce/internal/platform"
)

var errBuyerConfig = errors.New("invalid buyer configuration")

// Intercept buyer paths and their cleaning aliases before ServeMux. The original
// path is passed unchanged, so the strict buyer router rejects aliases instead
// of redirecting a credential-bearing request to another URL.
func mountBuyer(fallback, buyerHandler http.Handler) http.Handler {
	if buyerHandler == nil {
		return fallback
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/buyer") || strings.HasPrefix(path.Clean(r.URL.Path), "/v1/buyer") {
			buyerHandler.ServeHTTP(w, r)
			return
		}
		fallback.ServeHTTP(w, r)
	})
}

type buyerConfig struct {
	enabled                          bool
	issuerDSN, buyerDSN, checkoutDSN string
	bffKey                           string
	ttl                              time.Duration
	payment                          buyerPaymentConfig
}

// The buyer capability is distinct from a merchant login. Disabled mode must
// not read its DSNs or credentials, and enabling it never exposes the Go port.
func loadBuyerConfig(getenv func(string) string, addr string) (buyerConfig, error) {
	var c buyerConfig
	enabled, err := flag(getenv("COMMERCE_BUYER_ENABLED"))
	if err != nil {
		return c, errBuyerConfig
	}
	if !enabled {
		return c, nil
	}
	if !privateIdentityAddress(addr) {
		return c, errBuyerConfig
	}
	c.issuerDSN = getenv("COMMERCE_BUYER_ISSUER_DATABASE_URL")
	c.buyerDSN = getenv("COMMERCE_BUYER_DATABASE_URL")
	c.checkoutDSN = getenv("COMMERCE_CHECKOUT_DATABASE_URL")
	c.bffKey = getenv("COMMERCE_BUYER_BFF_KEY")
	c.ttl, err = time.ParseDuration(getenv("COMMERCE_BUYER_SESSION_TTL"))
	if err != nil || c.ttl < time.Minute || c.ttl > 30*24*time.Hour || c.ttl%time.Second != 0 ||
		strings.TrimSpace(c.issuerDSN) == "" || strings.TrimSpace(c.buyerDSN) == "" || strings.TrimSpace(c.checkoutDSN) == "" ||
		!identityhttp.ValidSecret(c.bffKey) || c.bffKey == getenv("COMMERCE_BFF_KEY") {
		return buyerConfig{}, errBuyerConfig
	}
	c.enabled = true
	c.payment, err = loadBuyerPaymentConfig(getenv, addr)
	if err != nil {
		return buyerConfig{}, errBuyerConfig
	}
	return c, nil
}

func buildBuyerHandler(ctx context.Context, c buyerConfig) (http.Handler, func(), error) {
	if !c.enabled {
		return nil, func() {}, nil
	}
	issuer, err := platform.OpenBuyerIssuerPool(ctx, c.issuerDSN)
	if err != nil {
		return nil, nil, errBuyerConfig
	}
	runtime, err := platform.OpenBuyerPool(ctx, c.buyerDSN)
	if err != nil {
		issuer.Close()
		return nil, nil, errBuyerConfig
	}
	checkoutPool, err := platform.OpenCheckoutPool(ctx, c.checkoutDSN)
	if err != nil {
		runtime.Close()
		issuer.Close()
		return nil, nil, errBuyerConfig
	}
	var hostedPool *pgxpool.Pool
	closePools := func() {
		if hostedPool != nil {
			hostedPool.Close()
		}
		checkoutPool.Close()
		runtime.Close()
		issuer.Close()
	}
	// River is used only to insert the expiry task in Begin's transaction. API
	// startup does not start workers or gain provider dispatch authority. The
	// expiry schema keeps River maintenance separate from payment and external jobs.
	jobs, err := river.NewClient(riverpgxv5.New(checkoutPool), &river.Config{Schema: "river_expiry"})
	if err != nil {
		closePools()
		return nil, nil, errBuyerConfig
	}
	service, err := checkout.New(ctx, checkoutPool, jobs)
	if err != nil {
		closePools()
		return nil, nil, errBuyerConfig
	}
	payment, openedHostedPool, err := buildBuyerPayment(ctx, c.payment)
	if err != nil {
		closePools()
		return nil, nil, errBuyerConfig
	}
	hostedPool = openedHostedPool
	h, err := buyerhttp.New(ctx, issuer, runtime, service, c.bffKey, c.ttl, payment)
	if err != nil {
		closePools()
		return nil, nil, errBuyerConfig
	}
	return h, closePools, nil
}
