// Purpose: the operator steps of the per-store settlement ledger (contracts/stripe-platform-account-v1.md §6, W4-S2; §6.6, S2-OPEN-1):
//   sync the platform account's balance transactions into the ledger, close a weekly statement, record a payout (RECORD ONLY), read a
//   statement for the CSV export, and resolve an unmapped_source row (append-only, §6.6) so close can proceed. Sync is the only step that
//   calls Stripe (GET balance transactions with the STORED platform key); close, payout, read and resolve are SQL only. Nothing here calls
//   a bank, a Stripe payout or a transfer API, and none takes a secret as input; resolve moves no money (a v1 assignment is a note).
// Depends on: SQL payments.record_settlement_lines, close_settlement, record_settlement_payout, read_settlement_statement (0150) and
//   record_settlement_resolution (0163); owner registry writer, EXECUTE commerce_payment_registrar only; payments.stripe_registrar_credential
//   via storedCredential; Stripe GET /v1/balance_transactions through stripe.Client.ListBalanceTransactions; internal/payments/settlement (types).
// Used by: cmd/stripe-admin settlement-sync | settlement-close | settlement-export | settlement-payout | settlement-resolve.
// Invariants: I02/I06/I20 (idempotent by Stripe balance transaction id: an identical replay returns the stored row, a different payload is
//   PT409), I19 (SQL is the only arithmetic), a refused SQL step returns one of our own coded tokens (never a driver message). Errors stay
//   ErrConfig/ErrDatabase/ErrRejected/ErrProvider (+ the token).
// Status: MOCK + REAL_PG; SANDBOX needs the owner's test key (NOT_RUN here); never LIVE.

package stripeadmin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/command"
	"livecommerce/internal/integrations/psp/stripe"
	"livecommerce/internal/payments/settlement"
)

const (
	settlementBudget = 10 * time.Minute // up to 50 Stripe pages of 10 s each is the wire bound; SQL is bounded separately
	settlementChunk  = 500              // record_settlement_lines accepts at most 500 lines per call
	maxSettleWindow  = 8 * 24 * time.Hour
	// settleMargin: a sync window may not end later than now minus this (P1-2). A window that claims time that has not settled would let close
	// treat balance transactions that did not exist yet as already read. SQL enforces the same margin (record_settlement_lines).
	settleMargin = 15 * time.Minute
)

var (
	datePattern       = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
	payoutRefPat      = regexp.MustCompile(`^[A-Za-z0-9._:/-]{4,80}$`)
	tokenPattern      = regexp.MustCompile(`^[a-z][a-z0-9_]{2,59}$`)
	balanceTxnPattern = regexp.MustCompile(`^txn_[A-Za-z0-9]{1,255}$`) // the ledger's own id grammar (0150/0163 CHECK)
)

// balanceLister is the slice of *stripe.Client the sync needs. It is asserted on the provider so the other registrar operations keep
// their narrow provider interface (and their test fakes).
type balanceLister interface {
	ListBalanceTransactions(ctx context.Context, createdGTE, createdLT int64) ([]stripe.BalanceTransaction, stripe.CallMeta, error)
}

// SyncReport is what settlement-sync prints: counts, and for each settlement currency Stripe's own net of the window next to the net of
// the rows now persisted for the same window. Difference is the persisted net minus Stripe's (0 = reconciled); counts and amounts only.
type SyncReport struct {
	Fetched      int              `json:"fetched"`
	Inserted     int              `json:"inserted"`
	Duplicate    int              `json:"duplicate"`
	Unattributed int              `json:"unattributed"`
	Mismatch     int              `json:"mismatch"`
	Rechecked    int              `json:"rechecked"` // lines whose mismatch (or fee share) was re-evaluated because a late fact arrived (P1-1)
	StripeNet    map[string]int64 `json:"stripe_net"`
	RecordedNet  map[string]int64 `json:"recorded_net"`
	Difference   map[string]int64 `json:"difference"`
}

// ClosedStatement is one statement id from close_settlement with its net and whether it was a replay.
type ClosedStatement struct {
	StatementID     string `json:"statement_id"`
	StoreID         string `json:"store_id"`
	NetPayableMinor int64  `json:"net_payable_minor"`
	LineCount       int    `json:"line_count"`
	Replayed        bool   `json:"replayed"`
}

// OperatorNote is one assigned_to_store resolution (§6.6) printed by close: v1 moves NO money, so the owner pays the
// target store out of band against this note. The unattributed row carries the amounts.
type OperatorNote struct {
	BalanceTxnID   string `json:"balance_txn_id"`
	TargetTenantID string `json:"target_tenant_id"`
	TargetStoreID  string `json:"target_store_id"`
	Operator       string `json:"operator"`
	Ticket         string `json:"ticket"`
	Note           string `json:"note"`
}

// CloseResult is the close_settlement output: the closed (or replayed) statements plus the window's operator notes.
type CloseResult struct {
	Statements    []ClosedStatement `json:"statements"`
	OperatorNotes []OperatorNote    `json:"operator_notes"`
}

// Resolution values of SettlementResolve (§6.6). A resolution is a recorded operator decision, never a money movement.
const (
	ResolveNotStoreRevenue = "not_store_revenue"
	ResolveAssignedToStore = "assigned_to_store"
)

// ResolvedUnattributed is the stored resolution row record_settlement_resolution returns. ResolvedAt stays the SQL
// text (house pattern: settlement.Statement.ClosedAt is a string for the same reason).
type ResolvedUnattributed struct {
	BalanceTxnID   string `json:"balance_txn_id"`
	Resolution     string `json:"resolution"`
	TargetTenantID string `json:"target_tenant_id"`
	TargetStoreID  string `json:"target_store_id"`
	Operator       string `json:"operator"`
	Ticket         string `json:"ticket"`
	Note           string `json:"note"`
	ResolvedAt     string `json:"resolved_at"`
	Replayed       bool   `json:"replayed"`
}

// SettlementSync reads the platform account's balance transactions created in [from, to) (at most 8 days) with the STORED credential of
// connectionID at expectedVersion, then records them (RecordSettlementLines). It fetches ALL pages first: on ErrUncertain (more than
// 50 pages, a malformed body, a transport failure) nothing is written. A read only: a failed run is simply rerun. The window is cut to whole
// seconds (the wire lists unix seconds) and may not end inside the last 15 minutes (ErrRejected). connectionID is also sent to SQL, which
// refuses it unless it is the designated platform connection; ticket (optional) goes into the audit row.
func (r *Registrar) SettlementSync(ctx context.Context, s Scope, connectionID string, expectedVersion int64, from, to time.Time, ticket string) (SyncReport, error) {
	if r == nil || r.db == nil || ctx == nil || r.apiKeys == nil || !validScope(s) || !command.ValidID(connectionID) || expectedVersion < 1 ||
		from.IsZero() || !to.After(from) || to.Sub(from) > maxSettleWindow {
		return SyncReport{}, ErrConfig
	}
	from, to = from.Truncate(time.Second), to.Truncate(time.Second)
	if !settledWindow(to) || (ticket != "" && !refPattern.MatchString(ticket)) {
		return SyncReport{}, ErrRejected
	}
	bounded, cancel := context.WithTimeout(ctx, settlementBudget)
	defer cancel()
	account, secret, err := r.storedCredential(bounded, s, connectionID, expectedVersion)
	if err != nil {
		return SyncReport{}, err
	}
	client, err := r.providerClient(secret, account)
	if err != nil {
		return SyncReport{}, err
	}
	lister, ok := client.(balanceLister)
	if !ok {
		return SyncReport{}, ErrConfig
	}
	// GET /v1/balance_transactions with expand[]=data.source (stripe-platform-account-v1 §6.5); the read is bounded to 50 pages.
	txns, _, err := lister.ListBalanceTransactions(bounded, from.Unix(), to.Unix())
	switch {
	case errors.Is(err, stripe.ErrAuthentication):
		return SyncReport{}, ErrRejected
	case err != nil:
		return SyncReport{}, ErrProvider
	}
	return r.RecordSettlementLines(bounded, s, connectionID, ticket, txns, from, to)
}

// settledWindow reports whether a window ending at to claims only time that has settled (to <= now - 15 minutes).
func settledWindow(to time.Time) bool { return !to.After(time.Now().Add(-settleMargin)) }

// RecordSettlementLines hands the projected transactions to payments.record_settlement_lines in chunks of at most 500 (attribution is
// done in SQL from Stripe ids). The LAST chunk carries the window, which records the sync coverage that close requires and returns the
// persisted net of the window for the reconciliation. Replaying a window inserts nothing new (duplicates are counted), but a line that still
// has no statement and carries a mismatch is re-checked (rechecked). connection (optional) is asserted in SQL to be the designated platform
// connection; ticket (optional) goes into the audit row. SQL applies the same window rules as SettlementSync (whole seconds, 15-minute margin).
func (r *Registrar) RecordSettlementLines(ctx context.Context, s Scope, connection, ticket string, txns []stripe.BalanceTransaction, from, to time.Time) (SyncReport, error) {
	if r == nil || r.db == nil || ctx == nil || !validScope(s) || from.IsZero() || !to.After(from) || to.Sub(from) > maxSettleWindow ||
		(connection != "" && !command.ValidID(connection)) {
		return SyncReport{}, ErrConfig
	}
	from, to = from.Truncate(time.Second), to.Truncate(time.Second)
	if !settledWindow(to) || (ticket != "" && !refPattern.MatchString(ticket)) {
		return SyncReport{}, ErrRejected
	}
	var connArg, ticketArg any
	if connection != "" {
		connArg = connection
	}
	if ticket != "" {
		ticketArg = ticket
	}
	rep := SyncReport{Fetched: len(txns), StripeNet: map[string]int64{}, RecordedNet: map[string]int64{}, Difference: map[string]int64{}}
	for _, t := range txns {
		rep.StripeNet[t.Currency] += t.Net
	}
	for start := 0; start == 0 || start < len(txns); start += settlementChunk {
		end := min(start+settlementChunk, len(txns))
		last := end >= len(txns)
		payload, err := json.Marshal(append([]stripe.BalanceTransaction{}, txns[start:end]...)) // never JSON null: an empty run still records its window
		if err != nil {
			return SyncReport{}, ErrConfig
		}
		var out string
		var winFrom, winTo any // only the LAST chunk carries the window: coverage exists only after a complete record
		if last {
			winFrom, winTo = from.UTC(), to.UTC()
		}
		// payments.record_settlement_lines: platform-scope operator definer; attribution, cross-checks and the unattributed rows are SQL.
		err = r.settlementScan(ctx, &out, `SELECT payments.record_settlement_lines($1::uuid,$2::uuid,$3::uuid,$4::text,$5::jsonb,
			$6::timestamptz,$7::timestamptz,$8::uuid,$9::text)::text`, s.TenantID, s.StoreID, s.PrincipalID, r.environment(), string(payload),
			winFrom, winTo, connArg, ticketArg)
		if err != nil {
			return SyncReport{}, err
		}
		var res struct {
			Inserted, Duplicate, Unattributed, Mismatch, Rechecked int
			WindowNet                                              map[string]int64 `json:"window_net"`
		}
		if json.Unmarshal([]byte(out), &res) != nil {
			return SyncReport{}, ErrDatabase
		}
		rep.Inserted += res.Inserted
		rep.Duplicate += res.Duplicate
		rep.Unattributed += res.Unattributed
		rep.Mismatch += res.Mismatch
		rep.Rechecked += res.Rechecked
		if last {
			rep.RecordedNet = res.WindowNet
			if rep.RecordedNet == nil {
				rep.RecordedNet = map[string]int64{}
			}
		}
	}
	for cur, net := range rep.StripeNet {
		rep.Difference[cur] = rep.RecordedNet[cur] - net
	}
	for cur, net := range rep.RecordedNet {
		if _, ok := rep.StripeNet[cur]; !ok {
			rep.Difference[cur] = net
		}
	}
	return rep, nil
}

// SettlementClose closes the weekly statement of the Monday periodStart (YYYY-MM-DD, Asia/Taipei) for one store, or for every store with
// unassigned lines when targetStore is "". SQL applies the §6.2 rules and returns the statements plus the window's operator notes (§6.6:
// the assigned_to_store resolutions; v1 moves no money). A replay returns the stored ones.
func (r *Registrar) SettlementClose(ctx context.Context, s Scope, environment, periodStart, operator, targetStore, ticket string) (CloseResult, error) {
	if r == nil || r.db == nil || ctx == nil || !validScope(s) || (environment != envSandbox && environment != envLive) ||
		(targetStore != "" && !command.ValidID(targetStore)) {
		return CloseResult{}, ErrConfig
	}
	if !datePattern.MatchString(periodStart) || !operatorPattern.MatchString(operator) || (ticket != "" && !refPattern.MatchString(ticket)) {
		return CloseResult{}, ErrRejected
	}
	var target, ticketArg any
	if targetStore != "" {
		target = targetStore
	}
	if ticket != "" {
		ticketArg = ticket
	}
	var out string
	// payments.close_settlement: platform-scope definer; sets statement_id on the lines once, one audit row per statement.
	if err := r.settlementScan(ctx, &out, `SELECT payments.close_settlement($1::uuid,$2::uuid,$3::uuid,$4::text,$5::date,$6::text,$7::uuid,$8::text)::text`,
		s.TenantID, s.StoreID, s.PrincipalID, environment, periodStart, operator, target, ticketArg); err != nil {
		return CloseResult{}, err
	}
	var res CloseResult
	if json.Unmarshal([]byte(out), &res) != nil {
		return CloseResult{}, ErrDatabase
	}
	if res.OperatorNotes == nil {
		res.OperatorNotes = []OperatorNote{}
	}
	return res, nil
}

// SettlementResolve appends ONE resolution row (§6.6) for an unmapped_source row of settlement_unattributed so the environment's close can
// proceed; the unattributed row itself is never touched (append-only). not_store_revenue takes no target; assigned_to_store requires
// targetTenant+targetStore of a store that has used the platform account, and in v1 moves NO money: close prints the assignment as an
// operator note and the owner pays out of band. ticket is REQUIRED: this is an escalation step and must stay traceable. An identical
// replay returns the stored row (Replayed); a different payload for the same txn is PT409 resolution_conflict. SQL only: never Stripe.
func (r *Registrar) SettlementResolve(ctx context.Context, s Scope, environment, balanceTxn, resolution, targetTenant, targetStore,
	note, operator, ticket string) (ResolvedUnattributed, error) {
	if r == nil || r.db == nil || ctx == nil || !validScope(s) || (environment != envSandbox && environment != envLive) ||
		((targetTenant != "" || targetStore != "") && (!command.ValidID(targetTenant) || !command.ValidID(targetStore))) {
		return ResolvedUnattributed{}, ErrConfig
	}
	if !balanceTxnPattern.MatchString(balanceTxn) || (resolution != ResolveNotStoreRevenue && resolution != ResolveAssignedToStore) ||
		(resolution == ResolveAssignedToStore) != (targetTenant != "") || !operatorPattern.MatchString(operator) ||
		!refPattern.MatchString(ticket) || utf8.RuneCountInString(note) < 1 || utf8.RuneCountInString(note) > 500 {
		return ResolvedUnattributed{}, ErrRejected
	}
	var targetT, targetS any
	if targetTenant != "" {
		targetT, targetS = targetTenant, targetStore
	}
	var out string
	// payments.record_settlement_resolution: platform-scope definer; one append-only row + one audit row, the same tx.
	if err := r.settlementScan(ctx, &out, `SELECT payments.record_settlement_resolution($1::uuid,$2::uuid,$3::uuid,$4::text,$5::text,
		$6::text,$7::text,$8::text,$9::text,$10::uuid,$11::uuid)::text`, s.TenantID, s.StoreID, s.PrincipalID, environment, balanceTxn,
		resolution, operator, ticket, note, targetT, targetS); err != nil {
		return ResolvedUnattributed{}, err
	}
	var res ResolvedUnattributed
	if json.Unmarshal([]byte(out), &res) != nil {
		return ResolvedUnattributed{}, ErrDatabase
	}
	return res, nil
}

// SettlementPayout RECORDS that the operator paid a statement off-Stripe (bank reference, amount, time). It never moves money. SQL sets
// the payout quadruple once; an identical replay returns the stored time, anything else is refused.
func (r *Registrar) SettlementPayout(ctx context.Context, s Scope, statement, payoutRef string, amountMinor int64, paidAt time.Time,
	operator, ticket string) (time.Time, error) {
	if r == nil || r.db == nil || ctx == nil || !validScope(s) || !command.ValidID(statement) {
		return time.Time{}, ErrConfig
	}
	if !payoutRefPat.MatchString(payoutRef) || amountMinor < 1 || paidAt.IsZero() || !operatorPattern.MatchString(operator) ||
		(ticket != "" && !refPattern.MatchString(ticket)) {
		return time.Time{}, ErrRejected
	}
	var ticketArg any
	if ticket != "" {
		ticketArg = ticket
	}
	var at time.Time
	// payments.record_settlement_payout: net_payable > 0 and amount = net_payable, paid_at not in the future, set once.
	if err := r.settlementScan(ctx, &at, `SELECT payments.record_settlement_payout($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::text,$6::bigint,
		$7::timestamptz,$8::text,$9::text)`, s.TenantID, s.StoreID, s.PrincipalID, statement, payoutRef, amountMinor, paidAt.UTC(), operator, ticketArg); err != nil {
		return time.Time{}, err
	}
	return at, nil
}

// SettlementStatement reads one statement with its lines for settlement-export (payments.read_settlement_statement, platform scope).
func (r *Registrar) SettlementStatement(ctx context.Context, s Scope, statement string) (settlement.Statement, error) {
	if r == nil || r.db == nil || ctx == nil || !validScope(s) || !command.ValidID(statement) {
		return settlement.Statement{}, ErrConfig
	}
	var out string
	if err := r.settlementScan(ctx, &out, `SELECT payments.read_settlement_statement($1::uuid,$2::uuid,$3::uuid,$4::uuid)::text`,
		s.TenantID, s.StoreID, s.PrincipalID, statement); err != nil {
		return settlement.Statement{}, err
	}
	st, err := settlement.DecodeStatement([]byte(out))
	if err != nil {
		return settlement.Statement{}, ErrDatabase
	}
	return st, nil
}

// settlementScan is scan with a longer budget and an error that keeps OUR coded refusal token (PT409 message such as
// settlement_mismatch, sync_required): the operator needs to know which rule refused. Only a message of the fixed token grammar is
// kept; any other driver message collapses to the plain sentinel.
func (r *Registrar) settlementScan(ctx context.Context, dest any, sql string, args ...any) error {
	bounded, cancel := context.WithTimeout(ctx, settlementBudget)
	defer cancel()
	err := r.db.QueryRow(bounded, sql, args...).Scan(dest)
	if err == nil {
		return nil
	}
	collapsed := sqlError(err)
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "PT409" && tokenPattern.MatchString(pg.Message) {
		return fmt.Errorf("%w: %s", collapsed, pg.Message)
	}
	return collapsed
}
