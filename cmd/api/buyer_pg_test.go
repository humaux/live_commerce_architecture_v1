//go:build buyerintegration

package main

import (
	"context"
	"encoding/base64"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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
}
