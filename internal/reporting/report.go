// report.go is the shared plumbing of the four W6-02B reports (contracts/reporting-v2.md): one database call per report through
// identity.read_report_* / identity.export_report, strict decoding, the second permission fence and the CSV writer with the formula guard.
//
// Purpose: keep products.go / channels.go / funnel.go / manual.go to their own row shapes and invariants.
// Depends on: identity.read_report_products / _channels / _funnel / _manual_orders and identity.export_report (migrations/0147_reports.sql, owner
//   commerce_auth, EXECUTE commerce_runtime); platform.RequirePermission; finance.go (ParseRange, mapError, ErrUnavailable and the patterns).
// Used by: internal/httpapi/reports.go; tests/foundation/reports_test.go.
// Invariants: I05 (per currency and environment, integer minor units, no conversion), I01 (scope only from the server Scope), read-only: the only
//   write is the audit row of an export, done inside identity.export_report.
// Status: REAL_PG (MOCK data).

package reporting

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// MaxProductRows is the bound of the product report (identity.read_report_products truncates beyond it and says so).
const MaxProductRows = 1000

// report names the database side of one report: the read definer's SQL and the name identity.export_report dispatches on.
type report struct {
	name  string // products | channels | funnel | manual_orders (export_report's p_report)
	query string // the read definer call; $5 only for the funnel
}

var (
	reportProducts = report{"products", `SELECT identity.read_report_products($1,$2::uuid,$3::date,$4::date)`}
	reportChannels = report{"channels", `SELECT identity.read_report_channels($1,$2::uuid,$3::date,$4::date)`}
	reportManual   = report{"manual_orders", `SELECT identity.read_report_manual_orders($1,$2::uuid,$3::date,$4::date)`}
	reportFunnel   = report{"funnel", `SELECT identity.read_report_funnel($1,$2::uuid,$3::date,$4::date,$5::uuid)`}
)

// fetch runs one report (read: orders:read; export: orders:export + one audit row reports.exported) and returns the raw jsonb. session is only
// meaningful for the funnel ("" = whole store).
func fetch(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, r report, from, to, session string, export bool) ([]byte, error) {
	if tx == nil || !command.ValidID(scope.TenantID) || !command.ValidID(scope.StoreID) || !command.ValidID(scope.PrincipalID) ||
		scope.Revision < 1 || len(token) < 32 || len(token) > 512 || (session != "" && (r.name != "funnel" || !command.ValidID(session))) {
		return nil, command.ErrInvalid
	}
	if _, _, err := ParseRange(from, to); err != nil {
		return nil, err
	}
	var sess any // NULL = whole store
	if session != "" {
		sess = session
	}
	hash := sha256.Sum256([]byte(token))
	permission := "orders:read"
	var args []any
	query := r.query
	if export {
		// identity.export_report: same jsonb as the read definer, orders:export + orders:read, audit row reports.exported.
		permission, query = "orders:export", `SELECT identity.export_report($1,$2::uuid,$3,$4::date,$5::date,$6::uuid)`
		args = []any{hash[:], scope.StoreID, r.name, from, to, sess}
	} else if r.name == "funnel" {
		args = []any{hash[:], scope.StoreID, from, to, sess}
	} else {
		args = []any{hash[:], scope.StoreID, from, to}
	}
	var raw []byte
	if err := tx.QueryRow(ctx, query, args...).Scan(&raw); err != nil {
		return nil, mapError(err)
	}
	// Second fence with the original Go Scope (finance pattern).
	if err := platform.RequirePermission(ctx, tx, scope, token, permission); err != nil {
		return nil, mapError(err)
	}
	return raw, nil
}

// strict decodes exactly one JSON object into out: unknown keys, duplicate-shaped drift and trailing data are all ErrUnavailable.
func strict(raw []byte, out any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(out) != nil {
		return ErrUnavailable
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return ErrUnavailable
	}
	return nil
}

// validHead checks the echoed range and zone of a decoded report against the request.
func validHead(gotFrom, gotTo, zone, from, to string) bool {
	return gotFrom == from && gotTo == to && zone == Timezone
}

func validEnv(e string) bool { return e == "SANDBOX" || e == "LIVE" }

// Money is one environment's money of a bucket (channels and manual orders). Offline money (COD, pay-at-pickup, confirmed bank transfer) is
// never part of captured or net. Amounts are integer minor units (I05).
type Money struct {
	Environment   string `json:"environment"`
	CapturedCount int64  `json:"captured_count"`
	CapturedMinor int64  `json:"captured_minor"`
	RefundedMinor int64  `json:"refunded_minor"`
	NetMinor      int64  `json:"net_minor"`
	OfflineCount  int64  `json:"offline_count"`
	OfflineMinor  int64  `json:"offline_minor"`
}

func validMoney(ms []Money) bool {
	for i, m := range ms {
		if !validEnv(m.Environment) || m.CapturedCount < 0 || m.CapturedMinor < 0 || m.RefundedMinor < 0 || m.OfflineCount < 0 || m.OfflineMinor < 0 ||
			(m.CapturedCount == 0 && m.CapturedMinor != 0) || (m.OfflineCount == 0 && m.OfflineMinor != 0) || m.NetMinor != m.CapturedMinor-m.RefundedMinor ||
			(i > 0 && ms[i-1].Environment >= m.Environment) {
			return false
		}
	}
	return true
}

func i64(v int64) string { return strconv.FormatInt(v, 10) }

// cell guards a merchant-supplied text cell against spreadsheet formulas (first non-space char = + - @, or a first TAB/CR/LF).
func cell(s string) string {
	if t := strings.TrimLeft(s, " \t\r\n"); s != "" && (s[0] == '\t' || s[0] == '\r' || s[0] == '\n' || (t != "" && strings.IndexByte("=+-@", t[0]) >= 0)) {
		return "'" + s
	}
	return s
}

// writeCSV renders a header and rows; every text field passed through cell() by the caller.
func writeCSV(header []string, rows [][]string) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(header); err != nil {
		return nil, err
	}
	for _, r := range rows {
		if err := w.Write(r); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

// moneyRows flattens a bucket's per-environment money into CSV cells; a bucket with no money yields one all-zero row (environment empty).
func moneyRows(ms []Money, emit func(m Money) []string) [][]string {
	if len(ms) == 0 {
		return [][]string{emit(Money{})}
	}
	out := make([][]string, len(ms))
	for i, m := range ms {
		out[i] = emit(m)
	}
	return out
}
