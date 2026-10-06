// Purpose: the settlement subcommands of stripe-admin (contracts/stripe-platform-account-v1.md §6.5, W4-S2):
//   settlement-sync   --environment --connection --expected-version --from --to    (Stripe GET balance transactions, STORED key)
//   settlement-close  --environment --period-start [--target-store]                (SQL only)
//   settlement-export --statement --out                                            (SQL read + a new 0600 CSV file)
//   settlement-payout --statement --payout-ref --amount --paid-at                  (SQL only: RECORDS a payout, never pays)
// Depends on: internal/payments/stripeadmin (SettlementSync/Close/Payout/Statement), internal/payments/settlement (CSV);
//   env COMMERCE_STRIPE_REGISTRAR_DATABASE_URL, the API keyring (sync only), STRIPE_SANDBOX=1 (SANDBOX sync opt-in) and the owner's LIVE
//   flag+reference pair (LIVE sync only).
// Used by: cmd/stripe-admin main.go dispatch; deploy/scripts/ops-admin.sh allowlist.
// Invariants: none takes a secret; --tenant/--store/--principal name the PLATFORM store (the target store of close is --target-store,
//   because --store is already the scope flag); --operator and --ticket are required on every subcommand; the ticket goes into the audit
//   row's details and is echoed in the one JSON result line. settlement-sync also names --connection, which SQL asserts is the designated
//   platform connection, and refuses --to later than now minus 15 minutes. Close, export and payout never call Stripe and never read the LIVE pair.
// Status: MOCK + REAL_PG; SANDBOX sync needs the owner's test key (NOT_RUN here).

package main

import (
	"context"
	"io"
	"regexp"
	"strings"
	"time"

	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/integrations/psp/stripe"
	"livecommerce/internal/payments/settlement"
)

var (
	ticketPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{8,128}$`)
	operatorName  = regexp.MustCompile(`^[A-Za-z0-9._:@-]{2,64}$`)
)

func runSettlement(ctx context.Context, name string, args []string, getenv func(string) string, stdout io.Writer) error {
	c := newCommand(name)
	var environment, connection, fromStr, toStr, periodStart, targetStore, statement, out, payoutRef, paidAtStr, operator, ticket string
	var expected, amount int64
	switch name {
	case "settlement-sync":
		c.fs.StringVar(&environment, "environment", "", "")
		c.fs.StringVar(&connection, "connection", "", "")
		c.fs.Int64Var(&expected, "expected-version", 0, "")
		c.fs.StringVar(&fromStr, "from", "", "")
		c.fs.StringVar(&toStr, "to", "", "")
	case "settlement-close":
		c.fs.StringVar(&environment, "environment", "", "")
		c.fs.StringVar(&periodStart, "period-start", "", "")
		c.fs.StringVar(&targetStore, "target-store", "", "")
	case "settlement-export":
		c.fs.StringVar(&statement, "statement", "", "")
		c.fs.StringVar(&out, "out", "", "")
	case "settlement-payout":
		c.fs.StringVar(&statement, "statement", "", "")
		c.fs.StringVar(&payoutRef, "payout-ref", "", "")
		c.fs.Int64Var(&amount, "amount", 0, "")
		c.fs.StringVar(&paidAtStr, "paid-at", "", "")
	default:
		return errUsage
	}
	c.fs.StringVar(&operator, "operator", "", "")
	c.fs.StringVar(&ticket, "ticket", "", "")
	if err := c.parse(args); err != nil {
		return err
	}
	if !operatorName.MatchString(operator) || !ticketPattern.MatchString(ticket) {
		return errUsage
	}
	if (name == "settlement-sync" || name == "settlement-close") && environment != "SANDBOX" && environment != "LIVE" {
		return errUsage
	}
	var from, to, paidAt time.Time
	var err error
	switch name {
	case "settlement-sync":
		if from, err = time.Parse(time.RFC3339, fromStr); err != nil {
			return errUsage // the parse error would echo the value; the fixed code does not
		}
		if to, err = time.Parse(time.RFC3339, toStr); err != nil {
			return errUsage
		}
		// P1-2: a window may not claim time that has not settled (--to later than now minus 15 minutes); SQL enforces it again
		if to.After(time.Now().Add(-15 * time.Minute)) {
			return errUsage
		}
	case "settlement-payout":
		if paidAt, err = time.Parse(time.RFC3339, paidAtStr); err != nil {
			return errUsage
		}
	case "settlement-export":
		if out == "" {
			return errUsage
		}
	}
	dsn := getenv("COMMERCE_STRIPE_REGISTRAR_DATABASE_URL")
	if strings.TrimSpace(dsn) == "" {
		return errConfig
	}
	// Only a LIVE sync reads the owner's pair and the API keyring (it unseals the stored platform key like live-approve).
	var live stripe.LiveApproval
	var apiKeys *accounts.Keyring
	if name == "settlement-sync" {
		if environment == "LIVE" {
			if live, err = requireLivePair(getenv); err != nil {
				return err
			}
		} else if getenv("STRIPE_SANDBOX") != "1" { // a SANDBOX sync calls the real Stripe API (psp/stripe client) with the stored test key: explicit opt-in
			return errConfig
		}
		if apiKeys, err = apiKeyring(getenv); err != nil {
			return err
		}
	}
	reg, err := openRegistrar(ctx, dsn, apiKeys, nil, live)
	if err != nil {
		return err
	}
	defer reg.Close()
	switch name {
	case "settlement-sync":
		rep, err := reg.SettlementSync(ctx, c.scope, connection, expected, from, to, ticket)
		if err != nil {
			return err
		}
		return emit(stdout, map[string]any{"environment": environment, "ticket": ticket, "from": from.UTC().Format(time.RFC3339),
			"to": to.UTC().Format(time.RFC3339), "report": rep})
	case "settlement-close":
		statements, err := reg.SettlementClose(ctx, c.scope, environment, periodStart, operator, targetStore, ticket)
		if err != nil {
			return err
		}
		return emit(stdout, map[string]any{"environment": environment, "ticket": ticket, "period_start": periodStart, "statements": statements})
	case "settlement-export":
		st, err := reg.SettlementStatement(ctx, c.scope, statement)
		if err != nil {
			return err
		}
		if err := settlement.WriteFile(out, st); err != nil {
			return err
		}
		return emit(stdout, map[string]any{"statement_id": statement, "ticket": ticket, "line_count": st.LineCount, "written": true})
	default: // settlement-payout
		at, err := reg.SettlementPayout(ctx, c.scope, statement, payoutRef, amount, paidAt, operator, ticket)
		if err != nil {
			return err
		}
		return emit(stdout, map[string]any{"statement_id": statement, "ticket": ticket, "paid_at": at.UTC().Format(time.RFC3339)})
	}
}
