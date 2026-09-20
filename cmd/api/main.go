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
	dsn := os.Getenv("DATABASE_URL")
	pool, err := platform.OpenPool(context.Background(), dsn)
	if err != nil {
		return err
	}
	defer pool.Close()

	identityHandler, closeIdentity, err := buildIdentityHandler(context.Background(), identityConfig)
	if err != nil {
		return err
	}
	defer closeIdentity()
	handler := httpapi.NewHandler(pool)
	if identityHandler != nil {
		mux := http.NewServeMux()
		mux.Handle("/v1/identity/", identityHandler)
		mux.Handle("/", handler)
		handler = mux
	}
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
