//go:build buyerintegration

// Purpose: probe actual unchanged factory pools, including split physical databases.
// Depends on: captured fixture DSNs, API factory and PostgreSQL connection readback.
// Used by: TestBuyerKeptPoolPreflightActualFactory isolated foundation child.
// Invariants: held participants match main DB; all cleanup preserves main connection.
package main

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/platform"
)

func TestBuyerKeptDatabasePreflightRealPG(t *testing.T) {
	if os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" || os.Getenv("LC_BUYER_KEPT_GATE") != "1" {
		t.Fatal("use the isolated retained-pool foundation gate")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	main, err := platform.OpenPool(ctx, os.Getenv("LC_BUYER_KEPT_MAIN_DSN"))
	if err != nil {
		t.Fatal("retained main authority unavailable")
	}
	defer main.Close()
	var mainPID int
	if main.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&mainPID) != nil {
		t.Fatal("main connection sentinel unavailable")
	}
	var observers []*pgxpool.Pool
	for _, name := range []string{"LC_BUYER_KEPT_OBSERVER_DSN", "LC_BUYER_KEPT_CLONE_OBSERVER_DSN"} {
		p, err := pgxpool.New(ctx, os.Getenv(name))
		if err != nil || p.Ping(ctx) != nil {
			t.Fatal("fixture observer unavailable")
		}
		defer p.Close()
		observers = append(observers, p)
	}
	base := buyerConfig{enabled: true, issuerDSN: os.Getenv("LC_BUYER_KEPT_ISSUER_DSN"), buyerDSN: os.Getenv("LC_BUYER_KEPT_RUNTIME_DSN"), checkoutDSN: os.Getenv("LC_BUYER_KEPT_CHECKOUT_DSN"), bffKey: strings.Repeat("A", 43), ttl: time.Hour}
	if buyerDatabasePreflight(ctx, base, main) != nil {
		t.Fatal("captured preflight positive control failed")
	}
	for _, stage := range []string{"success", "issuer", "runtime", "checkout"} {
		t.Run(stage, func(t *testing.T) {
			c := base
			switch stage {
			case "issuer":
				c.issuerDSN = os.Getenv("LC_BUYER_KEPT_CLONE_ISSUER_DSN")
			case "runtime":
				c.buyerDSN = os.Getenv("LC_BUYER_KEPT_CLONE_RUNTIME_DSN")
			case "checkout":
				c.checkoutDSN = os.Getenv("LC_BUYER_KEPT_CLONE_CHECKOUT_DSN")
			}
			label := "buyer-kept-" + stage
			for _, dsn := range []*string{&c.issuerDSN, &c.buyerDSN, &c.checkoutDSN} {
				parsed, err := url.Parse(*dsn)
				if err != nil || parsed.Scheme != "postgres" {
					t.Fatal("invalid isolated fixture DSN")
				}
				q := parsed.Query()
				q.Set("application_name", label)
				parsed.RawQuery, *dsn = q.Encode(), ""
				*dsn = parsed.String()
			}
			handler, parts, closePools, err := buildBuyerWithCVS(ctx, c, main, func(string) string { return "" })
			if err != nil || handler == nil || parts.Manual == nil || closePools == nil {
				t.Fatal("actual factory did not retain all valid login authorities")
			}
			t.Cleanup(closePools)
			var total int
			for _, observer := range observers {
				var count int
				if observer.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE application_name=$1`, label).Scan(&count) != nil {
					t.Fatal("retained connection sentinel unavailable")
				}
				total += count
			}
			if total < 3 {
				t.Fatal("actual factory did not hold three live participant connections")
			}
			err = buyerKeptDatabasePreflight(ctx, c, parts)
			if stage == "success" {
				if err != nil {
					t.Fatal("same physical retained pools refused")
				}
			} else if !errors.Is(err, errBuyerConfig) || err.Error() != errBuyerConfig.Error() {
				t.Fatal("actual retained split pool admitted or error disclosed details")
			}
			closePools()
			deadline := time.Now().Add(time.Second)
			for _, observer := range observers {
				for {
					var count int
					if observer.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE application_name=$1`, label).Scan(&count) != nil {
						t.Fatal("retained connection cleanup readback failed")
					}
					if count == 0 {
						break
					}
					if time.Now().After(deadline) {
						t.Fatal("actual retained pool cleanup leaked a connection")
					}
					time.Sleep(10 * time.Millisecond)
				}
			}
			var remainingPID int
			if main.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&remainingPID) != nil || remainingPID != mainPID {
				t.Fatal("retained preflight closed or replaced main connection")
			}
		})
	}
}
