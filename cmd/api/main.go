// Purpose: API process assembly — load each feature's config (identity, accounts, buyer/hosted payment, Meta/Stripe webhooks, Studio, claims, refunds, CVS, live-console inbox read side, message templates), open the scoped pools, build the handlers and mount them on one listener. Holds no business rule and never starts a worker or dispatches a provider call.
// Depends on: platform.OpenPool, httpapi.NewHandler, the cmd/api feature builders (accounts/buyer/meta/stripe/studio/claims/merchant_refund/merchant_ads/merchant_meta_connect/cvs/tlsask/store_domain_nonce), payments.ProfileEnvironment.
// Used by: the deployed API binary (LISTEN_ADDR), cmd/api *_test.go.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"livecommerce/internal/httpapi"
	"livecommerce/internal/msgtemplates"
	"livecommerce/internal/payments"
	"livecommerce/internal/platform"
)

func main() {
	if err := run(); err != nil {
		// Do not emit DSNs or driver errors; startup diagnostics belong to the
		// controlled deployment environment rather than application logs.
		slog.Error("api stopped")
		os.Exit(1)
	}
}

func run() error {
	identityConfig, err := loadIdentityConfig(os.Getenv)
	if err != nil {
		return err
	}
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	if identityConfig.enabled && !privateIdentityAddress(addr) {
		return errors.New("identity requires a literal loopback listener")
	}
	accountConfig, err := loadAccountConfig(os.Getenv, identityConfig.enabled, addr)
	if err != nil {
		return err
	}
	buyerConfig, err := loadBuyerConfig(os.Getenv, addr)
	if err != nil {
		return err
	}
	metaConfig, err := loadMetaConfig(os.Getenv, addr)
	if err != nil {
		return err
	}
	stripeConfig, err := loadStripeWebhookConfig(os.Getenv, addr)
	if err != nil {
		return err
	}
	payuniNotifyConfig, err := loadPayuniNotifyConfig(os.Getenv, addr)
	if err != nil {
		return err
	}
	studioConfig, err := loadStudioConfig(os.Getenv, identityConfig.enabled, addr)
	if err != nil {
		return err
	}
	claimsConfig, err := loadClaimsConfig(os.Getenv, studioConfig.enabled)
	if err != nil {
		return err
	}
	startup, stopStartup := context.WithTimeout(context.Background(), 10*time.Second)
	defer stopStartup()
	dsn := os.Getenv("DATABASE_URL")
	pool, err := platform.OpenPool(startup, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()

	identityHandler, closeIdentity, err := buildIdentityHandler(startup, identityConfig)
	if err != nil {
		return err
	}
	defer closeIdentity()
	buyerHandler, cvs, closeBuyer, err := buildBuyerWithCVS(startup, buyerConfig, pool, os.Getenv)
	if err != nil {
		return err
	}
	defer closeBuyer()
	if !buyerConfig.enabled {
		// Merchant-side CVS (settings, collection, release, MANUAL stores) does not need the buyer surface; ECPay itself needs the
		// payment profile, so with buyer payment off CVS_ECPAY_ENABLED=1 is a startup error (unit default C9).
		if cvs, err = buildCVS(startup, os.Getenv, pool, nil, ""); err != nil {
			return err
		}
	}
	metaHandler, closeMeta, err := buildMetaHandler(startup, pool, metaConfig)
	if err != nil {
		return err
	}
	defer closeMeta()
	stripeHandler, closeStripe, err := buildStripeWebhookHandler(startup, pool, stripeConfig)
	if err != nil {
		return err
	}
	defer closeStripe()
	payuniNotifyHandler, closePayuniNotify, err := buildPayuniNotifyHandler(startup, pool, payuniNotifyConfig)
	if err != nil {
		return err
	}
	defer closePayuniNotify()
	// billing-core (customers-billing-v1 T17): nil service + nil webhook while LC_BILLING_ENABLED is unset.
	billingService, billingWebhook, closeBilling, err := buildPlatformBilling(startup, pool, os.Getenv)
	if err != nil {
		return err
	}
	defer closeBilling()
	accountService, err := buildAccountsService(pool, accountConfig)
	if err != nil {
		return err
	}
	studioPlanner, err := buildStudioPlanner(startup, pool, studioConfig)
	if err != nil {
		return err
	}
	liveFlowJobs, err := buildLiveFlowJobs(pool, studioConfig.enabled)
	if err != nil {
		return err
	}
	refundJobs, err := newMerchantRefundJobs(pool)
	if err != nil {
		return err
	}
	// ads-graph: nil when COMMERCE_META_ADS_APP_ID is unset (surface off). Needs ads-core's Options.Ads.
	adsService, err := newMerchantAds(pool, os.Getenv)
	if err != nil {
		return err
	}
	// meta-connect: nil when COMMERCE_META_LOGIN_CONFIG_ID is unset (merchant Page/Instagram connect off).
	metaConnect, err := newMetaConnect(pool, os.Getenv)
	if err != nil {
		return err
	}
	// live-console comment read-through (LC-B2): nil when COMMERCE_CLAIMS_CONSOLE_BASE_URL is unset.
	commentStream, err := buildCommentStream(os.Getenv)
	if err != nil {
		return err
	}
	// meta connection-health banner (contract meta-connection-health-v1 §9): the §7.4 TableReader swap, nil off the same flag.
	metaHealth, err := newMetaHealth(os.Getenv)
	if err != nil {
		return err
	}
	// inbox read side (live-console-v1 §11 A8-A11/A13/A14): nil when the payload keyring is absent (surface off), so the
	// routes stay unmounted. The API process opens sealed message bodies only; it never holds the private page-token ring.
	inboxService, err := newInbox(os.Getenv)
	if err != nil {
		return err
	}
	// manual sends (LC-B4: A4/A5/A6/A12): needs the Page HPKE public ring (merchant connect's) and a River client.
	if err := enableInboxSend(inboxService, pool, os.Getenv); err != nil {
		return err
	}
	// order for a buyer (LC-B6: A15/A16): the manual-order pipeline + the inbox pay-link planner; nil without the buyer surface.
	forBuyer, err := buildForBuyer(cvs.Manual, inboxService)
	if err != nil {
		return err
	}
	// stripe-live-enable-v1 §5.2: the refund routes need the deployment's payment environment. An unset profile keeps
	// the pre-LIVE SANDBOX behavior (payment-free deployments); a set but unknown profile is refused at start.
	paymentEnvironment := ""
	if profile := os.Getenv("COMMERCE_PAYMENT_PROFILE"); profile != "" {
		env, ok := payments.ProfileEnvironment(profile)
		if !ok {
			return errors.New("payment profile is not supported")
		}
		paymentEnvironment = env
	}
	handler := httpapi.NewHandler(pool, httpapi.Options{SessionStoreList: identityConfig.enabled, Accounts: accountService, Studio: studioConfig.enabled, Live: studioPlanner,
		ClaimLabels: claimsConfig.labels, RefundJobs: refundJobs, Ads: adsService, MetaConnect: metaConnect, MetaHealth: metaHealth, Inbox: inboxService, MsgTemplates: msgtemplates.NewService(), Billing: billingService, CVS: cvs.Merchant, PaymentEnvironment: paymentEnvironment, PaymentProfile: os.Getenv("COMMERCE_PAYMENT_PROFILE"), ManualOrders: cvs.Manual, ForBuyer: forBuyer,
		LiveFlowJobs:    liveFlowJobs,
		CommentStream:   commentStream,
		StoreBaseDomain: strings.ToLower(strings.TrimSpace(os.Getenv("LC_STORE_BASE_DOMAIN")))})
	tlsAskHandler, err := buildTLSAskHandler(pool)
	if err != nil {
		return err
	}
	storeDomainNonceHandler := buildStoreDomainNonceHandler(pool)
	if identityHandler != nil {
		mux := http.NewServeMux()
		mux.Handle("/v1/identity/", identityHandler)
		mux.Handle("/", handler)
		handler = mux
	}
	if cvs.Hooks != nil {
		// Public ECPay callbacks (map return, status): reachable only through Caddy's two hooks-host routes (deploy/caddy/Caddyfile).
		mux := http.NewServeMux()
		mux.Handle("/v1/cvs/ecpay/", cvs.Hooks)
		mux.Handle("/", handler)
		handler = mux
	}
	handler = mountBuyer(handler, buyerHandler)
	handler = mountMeta(handler, metaHandler)
	handler = mountStripe(handler, stripeHandler)
	handler = mountPayuniNotify(handler, payuniNotifyHandler)
	handler = mountPlatformBilling(handler, billingWebhook)
	handler = mountTLSAsk(handler, tlsAskHandler)
	handler = mountStoreDomainNonce(handler, storeDomainNonceHandler)
	stopStartup()
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errs := make(chan error, 1)
	go func() { errs <- server.ListenAndServe() }()

	select {
	case err := <-errs:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	}
}
