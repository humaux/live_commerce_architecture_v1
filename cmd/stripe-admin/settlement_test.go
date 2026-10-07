// settlement_test.go: MOCK-tier tests of the settlement-* subcommands (contracts/stripe-platform-account-v1.md §6.5/§6.6): fixed usage
// errors that echo nothing, environment gating before any connection (SANDBOX sync needs STRIPE_SANDBOX=1, LIVE sync the owner's pair),
// and that close/export/payout/resolve read no Stripe or LIVE variable. Non-goal: the SQL (tests/foundation/platform_settlement_test.go,
// tests/foundation/settlement_resolve_test.go).
// Depends on: main.go run, main_test.go env()/do()/ids. Callers: go test ./cmd/stripe-admin.
package main

import (
	"errors"
	"strings"
	"testing"
)

const (
	slOp     = " --operator op@test --ticket TICKET-2026-1001"
	slStmt   = "5a5a5a5a-5a5a-4a5a-8a5a-5a5a5a5a5a01"
	slWindow = " --from 2026-09-14T00:00:00+08:00 --to 2026-09-21T00:00:00+08:00 --connection 44444444-4444-4444-8444-444444444444 --expected-version 1"
)

func TestSettlementUsageErrorsAreFixedAndPrintNothing(t *testing.T) {
	for _, line := range []string{
		"settlement-bogus " + ids,
		"settlement-close " + ids + " --environment SANDBOX --period-start 2026-09-14",                                   // no operator/ticket
		"settlement-close " + ids + " --environment PROD --period-start 2026-09-14" + slOp,                               // bad environment
		"settlement-close " + ids + " --environment SANDBOX --period-start 2026-09-14 --operator o --ticket TICKET-1234", // operator too short
		"settlement-close " + ids + " --environment SANDBOX --period-start 2026-09-14 --operator op@test --ticket short",
		"settlement-sync " + ids + " --environment SANDBOX --from notatime --to 2026-09-21T00:00:00Z" + slOp,
		"settlement-sync " + ids + " --environment SANDBOX --from 2098-12-28T00:00:00Z --to 2099-01-01T00:00:00Z --connection 44444444-4444-4444-8444-444444444444 --expected-version 1" + slOp, // P1-2: a future window
		"settlement-payout " + ids + " --statement " + slStmt + " --payout-ref BANK-1 --amount 100 --paid-at nope" + slOp,
		"settlement-export " + ids + " --statement " + slStmt + slOp, // no --out
		"settlement-export " + ids + " --statement " + slStmt + " --out /tmp/x.csv --extra=SECRETLEAK" + slOp,
		"settlement-resolve " + ids + " --environment SANDBOX --balance-txn txn_R1 --resolution not_store_revenue --note Stripe-fee",     // no operator/ticket
		"settlement-resolve " + ids + " --environment PROD --balance-txn txn_R1 --resolution not_store_revenue --note Stripe-fee" + slOp, // bad environment
		"settlement-resolve " + ids + " --environment SANDBOX --balance-txn txn_R1 --resolution not_store_revenue --note Stripe-fee --operator op@test --ticket short",
		"settlement-resolve " + ids + " --environment SANDBOX --balance-txn txn_R1 --resolution not_store_revenue --note n --extra=SECRETLEAK" + slOp,
	} {
		out, err := do(t, env(), line)
		if !errors.Is(err, errUsage) || out != "" || strings.Contains(err.Error(), "SECRETLEAK") || strings.Contains(err.Error(), "nope") {
			t.Fatalf("%q -> %q %v", line, out, err)
		}
	}
}

func TestSettlementEnvironmentGatesBeforeAnyConnection(t *testing.T) {
	sync := "settlement-sync " + ids + " --environment %s" + slWindow + slOp
	for _, c := range []struct {
		name, line string
		mutate     func(map[string]string)
		want       error
	}{
		{"no database url", "settlement-close " + ids + " --environment SANDBOX --period-start 2026-09-14" + slOp,
			func(m map[string]string) { delete(m, "COMMERCE_STRIPE_REGISTRAR_DATABASE_URL") }, errConfig},
		{"SANDBOX sync needs the explicit opt-in", strings.Replace(sync, "%s", "SANDBOX", 1), func(m map[string]string) {}, errConfig},
		{"LIVE sync needs the owner's pair", strings.Replace(sync, "%s", "LIVE", 1), func(m map[string]string) { m["STRIPE_SANDBOX"] = "1" }, errConfig},
		{"LIVE sync with a half pair", strings.Replace(sync, "%s", "LIVE", 1), func(m map[string]string) { m["COMMERCE_STRIPE_LIVE_ENABLED"] = "1" }, errConfig},
		{"sync needs the API keyring", strings.Replace(sync, "%s", "SANDBOX", 1), func(m map[string]string) {
			m["STRIPE_SANDBOX"] = "1"
			delete(m, "COMMERCE_ACCOUNT_KEYS_JSON")
		}, errConfig},
	} {
		values := env()
		c.mutate(values)
		if out, err := do(t, values, c.line); !errors.Is(err, c.want) || out != "" {
			t.Errorf("%s: %q %v", c.name, out, err)
		}
	}
	// With everything valid the CLI reaches the (unreachable) database and fails masked, never echoing the DSN password.
	values := env()
	values["STRIPE_SANDBOX"] = "1"
	if out, err := do(t, values, strings.Replace(sync, "%s", "SANDBOX", 1)); err == nil || out != "" || strings.Contains(err.Error(), dsnSentinel1) {
		t.Fatalf("valid sync: %q %v", out, err)
	}
}

func TestSettlementCloseExportPayoutReadNoStripeOrLiveVariable(t *testing.T) {
	// They must not need a keyring, STRIPE_* or the LIVE pair (kill-switch-grade independence): an empty environment except the DSN reaches the database step.
	values := map[string]string{"COMMERCE_STRIPE_REGISTRAR_DATABASE_URL": env()["COMMERCE_STRIPE_REGISTRAR_DATABASE_URL"]}
	for _, line := range []string{
		"settlement-close " + ids + " --environment LIVE --period-start 2026-09-14" + slOp,
		"settlement-export " + ids + " --statement " + slStmt + " --out /tmp/stripe-admin-never-written.csv" + slOp,
		"settlement-payout " + ids + " --statement " + slStmt + " --payout-ref BANK-REF-1 --amount 100 --paid-at 2026-10-01T10:00:00Z" + slOp,
		// S2-OPEN-1: resolve is SQL-only like close; even --environment LIVE must not touch the pair or a keyring
		"settlement-resolve " + ids + " --environment LIVE --balance-txn txn_R1 --resolution not_store_revenue --note Stripe-fee" + slOp,
	} {
		_, err := do(t, values, line)
		if err == nil || errors.Is(err, errUsage) || errors.Is(err, errConfig) || strings.Contains(err.Error(), dsnSentinel1) {
			t.Errorf("%q: %v (want a masked database failure, not a usage/config refusal)", line, err)
		}
	}
}
