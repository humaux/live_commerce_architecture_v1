//go:build buyerintegration

// Purpose: exercise buyer assembly and startup preflight against isolated PostgreSQL roles.
// Depends on: platform pool admission and the foundation buyer HTTP fixture.
// Used by: TestBuyerHTTPAPIAssemblyRealPoolCleanup through the buyerintegration gate.
// Invariants: fail closed on wrong authority; temporary pools close; reference pool survives.
package main

import (
	"context"
	"encoding/base64"
	"errors"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/platform"
)

// Invoked only by the isolated foundation fixture after its roles/migrations
// exist. No production DSN is accepted from the regular API environment.
func TestBuyerPoolAssemblyRealPG(t *testing.T) {
	if os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" || os.Getenv("LC_BUYER_ASSEMBLY_GATE") != "1" {
		t.Fatal("use the isolated foundation buyer HTTP gate")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	observer, err := pgxpool.New(ctx, os.Getenv("LC_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal("fixture observer unavailable")
	}
	defer observer.Close()
	base := buyerConfig{enabled: true, issuerDSN: os.Getenv("LC_BUYER_TEST_ISSUER_DSN"), buyerDSN: os.Getenv("LC_BUYER_TEST_RUNTIME_DSN"), checkoutDSN: os.Getenv("LC_BUYER_TEST_CHECKOUT_DSN"), bffKey: base64.RawURLEncoding.EncodeToString(make([]byte, 32)), ttl: time.Hour}
	for _, stage := range []string{"runtime", "checkout", "handler", "success"} {
		t.Run(stage, func(t *testing.T) {
			c := base
			if stage == "runtime" {
				c.buyerDSN = c.issuerDSN
			}
			if stage == "checkout" {
				c.checkoutDSN = c.buyerDSN
			}
			if stage == "handler" {
				c.ttl = 0
			}
			label := "buyer-assembly-" + stage
			for _, dsn := range []*string{&c.issuerDSN, &c.buyerDSN, &c.checkoutDSN} {
				parsed, err := url.Parse(*dsn)
				if err != nil || parsed.Scheme != "postgres" {
					t.Fatal("invalid fixture DSN")
				}
				query := parsed.Query()
				query.Set("application_name", label)
				parsed.RawQuery = query.Encode()
				*dsn = parsed.String()
			}
			h, closePools, err := buildBuyerHandler(ctx, c)
			if stage == "success" {
				if err != nil || h == nil || closePools == nil {
					t.Fatal("valid assembly failed")
				}
				t.Cleanup(closePools)
				var count int
				if observer.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE application_name=$1`, label).Scan(&count) != nil || count < 3 {
					t.Fatal("three independent pools not opened")
				}
				closePools()
			} else if err == nil || h != nil || closePools != nil {
				t.Fatal("partial startup did not fail closed")
			}
			// Backend exit is asynchronous; observe actual PG connections rather
			// than treating a call to Close as proof of released resources.
			deadline := time.Now().Add(time.Second)
			for {
				var count int
				if observer.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE application_name=$1`, label).Scan(&count) != nil {
					t.Fatal("connection readback failed")
				}
				if count == 0 {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("assembly leaked a fixture connection")
				}
				time.Sleep(10 * time.Millisecond)
			}
		})
	}
	// These sentinels establish working fixture authorities on the actual DB,
	// before a refusal or an empty pg_stat_activity result is accepted as proof.
	var database string
	var observerPID int
	if observer.QueryRow(ctx, `SELECT current_database(), pg_backend_pid()`).Scan(&database, &observerPID) != nil {
		t.Fatal("preflight reference database unavailable")
	}
	for _, probe := range []struct {
		dsn, role string
		open      func(context.Context, string) (*pgxpool.Pool, error)
	}{
		{base.issuerDSN, "commerce_buyer_issuer", platform.OpenBuyerIssuerPool},
		{base.buyerDSN, "commerce_buyer_runtime", platform.OpenBuyerPool},
		{base.checkoutDSN, "commerce_checkout_runtime", platform.OpenCheckoutPool},
	} {
		pool, err := probe.open(ctx, probe.dsn)
		if err != nil {
			t.Fatal("preflight authority sentinel unavailable")
		}
		var actualDatabase string
		var probePID int
		var member, sameLogin bool
		err = pool.QueryRow(ctx, `SELECT current_database(), pg_backend_pid(), pg_has_role(session_user, $1::name, 'MEMBER'), session_user=current_user`, probe.role).Scan(&actualDatabase, &probePID, &member, &sameLogin)
		pool.Close()
		if err != nil || actualDatabase != database || !member || !sameLogin {
			t.Fatal("preflight authority/database sentinel failed")
		}
		deadline := time.Now().Add(time.Second)
		for {
			var count int
			if observer.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE pid=$1`, probePID).Scan(&count) != nil {
				t.Fatal("preflight sentinel connection readback failed")
			}
			if count == 0 {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("preflight sentinel connection did not disappear")
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	for _, stage := range []string{"issuer", "runtime", "checkout", "nil-context", "canceled", "success"} {
		t.Run("preflight-"+stage, func(t *testing.T) {
			c := base
			switch stage {
			case "issuer":
				c.issuerDSN = c.buyerDSN
			case "runtime":
				c.buyerDSN = c.issuerDSN
			case "checkout":
				c.checkoutDSN = c.buyerDSN
			}
			label := "buyer-preflight-" + stage
			for _, dsn := range []*string{&c.issuerDSN, &c.buyerDSN, &c.checkoutDSN} {
				parsed, err := url.Parse(*dsn)
				if err != nil || parsed.Scheme != "postgres" {
					t.Fatal("invalid preflight fixture DSN")
				}
				query := parsed.Query()
				query.Set("application_name", label)
				parsed.RawQuery = query.Encode()
				*dsn = parsed.String()
			}
			preflightCtx := ctx
			if stage == "nil-context" {
				preflightCtx = nil
			}
			if stage == "canceled" {
				canceledCtx, stop := context.WithCancel(ctx)
				stop()
				preflightCtx = canceledCtx
			}
			err := buyerDatabasePreflight(preflightCtx, c, observer)
			if stage == "success" {
				if err != nil {
					t.Fatal("valid database preflight failed")
				}
			} else if !errors.Is(err, errBuyerConfig) || err.Error() != errBuyerConfig.Error() {
				t.Fatal("preflight did not reject wrong authority with a redacted error")
			}
			deadline := time.Now().Add(time.Second)
			for {
				var count int
				if observer.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE application_name=$1`, label).Scan(&count) != nil {
					t.Fatal("preflight connection readback failed")
				}
				if count == 0 {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("preflight leaked a fixture connection")
				}
				time.Sleep(10 * time.Millisecond)
			}
			var actualPID int
			if observer.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&actualPID) != nil || actualPID != observerPID {
				t.Fatal("preflight closed or replaced the reference connection")
			}
		})
	}
}
