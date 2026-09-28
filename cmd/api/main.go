package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"livecommerce/internal/httpapi"
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
	buyerHandler, closeBuyer, err := buildBuyerHandler(startup, buyerConfig)
	if err != nil {
		return err
	}
	defer closeBuyer()
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
	accountService, err := buildAccountsService(pool, accountConfig)
	if err != nil {
		return err
	}
	studioPlanner, err := buildStudioPlanner(startup, pool, studioConfig)
	if err != nil {
		return err
	}
	handler := httpapi.NewHandler(pool, httpapi.Options{SessionStoreList: identityConfig.enabled, Accounts: accountService, Live: studioPlanner,
		ClaimLabels: claimsConfig.labels})
	if identityHandler != nil {
		mux := http.NewServeMux()
		mux.Handle("/v1/identity/", identityHandler)
		mux.Handle("/", handler)
		handler = mux
	}
	handler = mountBuyer(handler, buyerHandler)
	handler = mountMeta(handler, metaHandler)
	handler = mountStripe(handler, stripeHandler)
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
