// manual.go serves the W6-02B manual-order report: merchant-created (代建单) orders of a range per creating staff principal and live session,
// with order counts and money, from identity.read_report_manual_orders (contracts/reporting-v2.md §5).
//
// Purpose: strict decoding and re-verification of the manual-order buckets, and their CSV. It carries principal and session ids only: the
// admin UI resolves them to a staff name and a session title; no buyer data and no names are read here.
// Depends on: report.go (fetch/strict/Money/CSV), identity.read_report_manual_orders / export_report (migrations/0147_reports.sql).
// Used by: internal/httpapi/reports.go.
// Invariants: I05 (money per currency and environment; offline money never in captured/net).
// Status: REAL_PG (MOCK data).

package reporting

import (
	"context"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// ManualRow is one (creator, session, currency). PrincipalID is nil when the creation receipt is missing; SessionID is nil when the order did
// not come from a live session.
type ManualRow struct {
	PrincipalID     *string `json:"principal_id"`
	SessionID       *string `json:"session_id"`
	Currency        string  `json:"currency"`
	Orders          int64   `json:"orders"`
	CancelledOrders int64   `json:"cancelled_orders"`
	Money           []Money `json:"money"`
}

// ManualReport is the whole manual-order answer for a range.
type ManualReport struct {
	From     string      `json:"from"`
	To       string      `json:"to"`
	Timezone string      `json:"timezone"`
	Rows     []ManualRow `json:"rows"`
}

// ManualOrders reads the manual-order report (orders:read, no write); environment is the deployment payment environment (SANDBOX or LIVE) the order counts follow.
func ManualOrders(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, from, to, environment string) (ManualReport, error) {
	return manualFetch(ctx, tx, scope, token, from, to, environment, false)
}

// ManualOrdersCSV is ManualOrders as CSV through identity.export_report (orders:export AND orders:read, one audit row reports.export.<report>).
func ManualOrdersCSV(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, from, to, environment string) ([]byte, error) {
	rep, err := manualFetch(ctx, tx, scope, token, from, to, environment, true)
	if err != nil {
		return nil, err
	}
	var rows [][]string
	for _, r := range rep.Rows {
		rows = append(rows, moneyRows(r.Money, func(m Money) []string {
			return []string{deref(r.PrincipalID), deref(r.SessionID), r.Currency, i64(r.Orders), i64(r.CancelledOrders), m.Environment,
				i64(m.CapturedCount), i64(m.CapturedMinor), i64(m.RefundedMinor), i64(m.NetMinor), i64(m.OfflineCount), i64(m.OfflineMinor)}
		})...)
	}
	return writeCSV([]string{"principal_id", "session_id", "currency", "orders", "cancelled_orders", "environment", "captured_count",
		"captured_minor", "refunded_minor", "net_minor", "offline_count", "offline_minor"}, rows)
}

func manualFetch(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, from, to, environment string, export bool) (ManualReport, error) {
	raw, err := fetch(ctx, tx, scope, token, reportManual, from, to, "", environment, export)
	if err != nil {
		return ManualReport{}, err
	}
	var rep ManualReport
	if strict(raw, &rep) != nil || !validHead(rep.From, rep.To, rep.Timezone, from, to) {
		return ManualReport{}, ErrUnavailable
	}
	for _, r := range rep.Rows {
		if (r.PrincipalID != nil && !command.ValidID(*r.PrincipalID)) || (r.SessionID != nil && !command.ValidID(*r.SessionID)) ||
			!currencyCode.MatchString(r.Currency) || r.Orders < 0 || r.CancelledOrders < 0 || !validMoney(r.Money) {
			return ManualReport{}, ErrUnavailable
		}
	}
	if rep.Rows == nil {
		rep.Rows = []ManualRow{}
	}
	return rep, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
