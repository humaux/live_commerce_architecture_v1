//go:build buyerintegration

package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestBuyerPaymentPoolAssemblyRealPG runs only inside the disposable foundation
// fixture. The observer is independent of every pool owned by the API builder.
func TestBuyerPaymentPoolAssemblyRealPG(t *testing.T) {
	if os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" || os.Getenv("LC_BUYER_PAYMENT_ASSEMBLY_GATE") != "1" {
		t.Fatal("use the isolated foundation buyer payment assembly gate")
	}
	for _, name := range []string{"LC_TEST_DATABASE_URL", "LC_BUYER_TEST_ISSUER_DSN", "LC_BUYER_TEST_RUNTIME_DSN",
		"LC_BUYER_TEST_CHECKOUT_DSN", "LC_BUYER_TEST_HOSTED_DSN"} {
		if os.Getenv(name) == "" {
			t.Fatalf("missing fixture setting %s", name)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	observer, err := pgxpool.New(ctx, os.Getenv("LC_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal("fixture observer unavailable")
	}
	defer observer.Close()

	for _, stage := range []string{"success", "wrong-hosted-role", "invalid-profile", "invalid-key", "handler"} {
		t.Run(stage, func(t *testing.T) {
			values := buyerPaymentTestEnv() // synthetic keys only; no provider request
			values["COMMERCE_BUYER_ISSUER_DATABASE_URL"] = os.Getenv("LC_BUYER_TEST_ISSUER_DSN")
			values["COMMERCE_BUYER_DATABASE_URL"] = os.Getenv("LC_BUYER_TEST_RUNTIME_DSN")
			values["COMMERCE_CHECKOUT_DATABASE_URL"] = os.Getenv("LC_BUYER_TEST_CHECKOUT_DSN")
			values["COMMERCE_HOSTED_DATABASE_URL"] = os.Getenv("LC_BUYER_TEST_HOSTED_DSN")
			config, err := loadBuyerConfig(func(name string) string { return values[name] }, "127.0.0.1:8080")
			if err != nil || !config.payment.enabled {
				t.Fatal("valid fixture payment configuration rejected")
			}
			label := fmt.Sprintf("bph05-%d-%s", os.Getpid(), stage)
			for _, dsn := range []*string{&config.issuerDSN, &config.buyerDSN, &config.checkoutDSN, &config.payment.hostedDSN} {
				*dsn = paymentAssemblyLabeledDSN(t, *dsn, label)
			}
			switch stage {
			case "wrong-hosted-role":
				config.payment.hostedDSN = config.checkoutDSN
			case "invalid-profile":
				config.payment.profile = "INVALID"
			case "invalid-key":
				config.payment.keys = nil
			case "handler":
				config.ttl = 0
			}
			h, closePools, err := buildBuyerHandler(ctx, config)
			if stage == "success" {
				if err != nil || h == nil || closePools == nil {
					t.Fatal("valid hosted buyer assembly failed")
				}
				closed := false
				defer func() {
					if !closed {
						closePools()
					}
				}()
				var count, roles int
				if err := observer.QueryRow(ctx, `SELECT count(*),count(DISTINCT usename) FROM pg_stat_activity WHERE application_name=$1`, label).
					Scan(&count, &roles); err != nil || count < 4 || roles != 4 {
					t.Fatal("four distinct buyer/checkout/hosted roles were not admitted")
				}
				closePools()
				closed = true
			} else if !errors.Is(err, errBuyerConfig) || h != nil || closePools != nil {
				t.Fatal("invalid hosted startup did not fail closed")
			}
			// Close may precede backend exit. Verify actual connection removal,
			// including the pool whose role or signer validation failed.
			until := time.Now().Add(2 * time.Second)
			for {
				var count int
				if err := observer.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE application_name=$1`, label).Scan(&count); err != nil {
					t.Fatal("fixture connection readback failed")
				}
				if count == 0 {
					break
				}
				if time.Now().After(until) {
					t.Fatal("hosted buyer assembly leaked a fixture connection")
				}
				time.Sleep(10 * time.Millisecond)
			}
		})
	}
}

func paymentAssemblyLabeledDSN(t *testing.T, dsn, label string) string {
	t.Helper()
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.Scheme != "postgres" {
		t.Fatal("invalid fixture DSN")
	}
	query := parsed.Query()
	query.Set("application_name", label)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
