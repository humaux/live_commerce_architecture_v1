// Purpose: the settlement subcommands of stripe-admin (contracts/stripe-platform-account-v1.md §6.5, W4-S2; §6.6, S2-OPEN-1):
//   settlement-sync    --environment --connection --expected-version --from --to   (Stripe GET balance transactions, STORED key)
//   settlement-close   --environment --period-start [--target-store]               (SQL only; prints the period's operator_notes; a refusal for
//                      unresolved unmapped_source rows prints their blocking balance_txn_ids, up to 20, §6.6)
//   settlement-export  --statement --out                                           (SQL read + a new 0600 CSV file)
//   settlement-payout  --statement --payout-ref --amount --paid-at                 (SQL only: RECORDS a payout, never pays)
//   settlement-resolve --environment --balance-txn --resolution [--target-tenant --target-store] --note
//                      (SQL only: appends one §6.6 resolution row to an unmapped_source row so close can proceed; never moves money)
//   settlement-payout-return --statement --return-ref --amount --returned-at   (SQL only: RECORDS that a payout bounced, §6.7; the next close
//                      carries it into the store's next statement; never calls a bank)
//   settlement-payables --environment [--limit 1..200, default 50]              (SQL only, read only: statements still owed money, §6.9)
// Depends on: internal/payments/stripeadmin (SettlementSync/Close/Payout/PayoutReturn/Payables/Statement/Resolve), internal/payments/settlement (CSV);
//   env COMMERCE_STRIPE_REGISTRAR_DATABASE_URL, the API keyring (sync only), STRIPE_SANDBOX=1 (SANDBOX sync opt-in) and the owner's LIVE
//   flag+reference pair (LIVE sync only).
// Used by: cmd/stripe-admin main.go dispatch; deploy/scripts/ops-admin.sh allowlist.
// Invariants: none takes a secret; --tenant/--store/--principal name the PLATFORM store (the target store of close is --target-store,
//   because --store is already the scope flag; the assigned store of resolve is --target-tenant + --target-store); --operator and --ticket
//   are required on every subcommand; the ticket goes into the audit row's details and is echoed in the one JSON result line.
//   settlement-sync also names --connection, which SQL asserts is the designated platform connection, and refuses --to later than now minus
//   15 minutes, prints the run id and the stored per-currency reconciliation, and exits non-zero (settlement_unreconciled) when any currency
//   has a non-zero difference (the rows are already stored; the exit only tells the operator that close will refuse, §6.8). Close, export,
//   payout, payout-return, payables and resolve never call Stripe and never read the LIVE pair.
// Status: MOCK + REAL_PG; SANDBOX sync needs the owner's test key (NOT_RUN here).

package main

import (
	"context"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"

	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/integrations/psp/stripe"
	"livecommerce/internal/payments/settlement"
)

// errUnreconciled is the fixed exit text of a settlement-sync whose stored comparison has a non-zero difference (§6.8).
var errUnreconciled = errors.New("settlement_unreconciled")

var (
	ticketPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{8,128}$`)
	operatorName  = regexp.MustCompile(`^[A-Za-z0-9._:@-]{2,64}$`)
)

func runSettlement(ctx context.Context, name string, args []string, getenv func(string) string, stdout io.Writer) error {
	c := newCommand(name)
	var environment, connection, fromStr, toStr, periodStart, targetStore, statement, out, payoutRef, paidAtStr, operator, ticket string
	var balanceTxn, resolution, targetTenant, note, returnRef, returnedAtStr string
	var expected, amount int64
	limit := 50
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
	case "settlement-resolve":
		c.fs.StringVar(&environment, "environment", "", "")
		c.fs.StringVar(&balanceTxn, "balance-txn", "", "")
		c.fs.StringVar(&resolution, "resolution", "", "")
		c.fs.StringVar(&targetTenant, "target-tenant", "", "")
		c.fs.StringVar(&targetStore, "target-store", "", "")
		c.fs.StringVar(&note, "note", "", "")
	case "settlement-export":
		c.fs.StringVar(&statement, "statement", "", "")
		c.fs.StringVar(&out, "out", "", "")
	case "settlement-payout":
		c.fs.StringVar(&statement, "statement", "", "")
		c.fs.StringVar(&payoutRef, "payout-ref", "", "")
		c.fs.Int64Var(&amount, "amount", 0, "")
		c.fs.StringVar(&paidAtStr, "paid-at", "", "")
	case "settlement-payout-return":
		c.fs.StringVar(&statement, "statement", "", "")
		c.fs.StringVar(&returnRef, "return-ref", "", "")
		c.fs.Int64Var(&amount, "amount", 0, "")
		c.fs.StringVar(&returnedAtStr, "returned-at", "", "")
	case "settlement-payables":
		c.fs.StringVar(&environment, "environment", "", "")
		c.fs.IntVar(&limit, "limit", 50, "")
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
	if (name == "settlement-sync" || name == "settlement-close" || name == "settlement-resolve" || name == "settlement-payables") &&
		environment != "SANDBOX" && environment != "LIVE" {
		return errUsage
	}
	if name == "settlement-payables" && (limit < 1 || limit > 200) {
		return errUsage
	}
	var from, to, paidAt, returnedAt time.Time
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
	case "settlement-payout-return":
		if returnedAt, err = time.Parse(time.RFC3339, returnedAtStr); err != nil {
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
		if err := emit(stdout, map[string]any{"environment": environment, "ticket": ticket, "from": from.UTC().Format(time.RFC3339),
			"to": to.UTC().Format(time.RFC3339), "sync_run_id": rep.SyncRunID, "reconciliation": rep.Reconciliation, "report": rep}); err != nil {
			return err
		}
		if !rep.Reconciled() {
			return errUnreconciled // §6.8: the line above is already stored; this only tells the operator that close will refuse
		}
		return nil
	case "settlement-close":
		res, err := reg.SettlementClose(ctx, c.scope, environment, periodStart, operator, targetStore, ticket)
		if err != nil {
			return err
		}
		return emit(stdout, map[string]any{"environment": environment, "ticket": ticket, "period_start": periodStart,
			"statements": res.Statements, "operator_notes": res.OperatorNotes})
	case "settlement-resolve":
		res, err := reg.SettlementResolve(ctx, c.scope, environment, balanceTxn, resolution, targetTenant, targetStore, note, operator, ticket)
		if err != nil {
			return err
		}
		return emit(stdout, map[string]any{"environment": environment, "ticket": ticket, "balance_txn": res.BalanceTxnID,
			"resolution": res.Resolution, "target_tenant": res.TargetTenantID, "target_store": res.TargetStoreID,
			"resolved_at": res.ResolvedAt, "replayed": res.Replayed})
	case "settlement-export":
		st, err := reg.SettlementStatement(ctx, c.scope, statement)
		if err != nil {
			return err
		}
		if err := settlement.WriteFile(out, st); err != nil {
			return err
		}
		return emit(stdout, map[string]any{"statement_id": statement, "ticket": ticket, "line_count": st.LineCount, "written": true})
	case "settlement-payout-return":
		at, err := reg.SettlementPayoutReturn(ctx, c.scope, statement, returnRef, amount, returnedAt, operator, ticket)
		if err != nil {
			return err
		}
		return emit(stdout, map[string]any{"statement_id": statement, "ticket": ticket, "recorded_at": at.UTC().Format(time.RFC3339Nano)})
	case "settlement-payables":
		res, err := reg.SettlementPayables(ctx, c.scope, environment, limit)
		if err != nil {
			return err
		}
		return emit(stdout, map[string]any{"environment": environment, "ticket": ticket, "payables": res.Payables, "truncated": res.Truncated})
	default: // settlement-payout
		at, err := reg.SettlementPayout(ctx, c.scope, statement, payoutRef, amount, paidAt, operator, ticket)
		if err != nil {
			return err
		}
		return emit(stdout, map[string]any{"statement_id": statement, "ticket": ticket, "paid_at": at.UTC().Format(time.RFC3339)})
	}
}
