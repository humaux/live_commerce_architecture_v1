// funnel.go serves the W6-02B funnel: claimed -> link sent -> ordered -> paid bundle counts of a range, whole store or one live session, from
// identity.read_report_funnel (contracts/reporting-v2.md §4).
//
// Purpose: strict decoding and re-verification of the four nested counts, and their CSV.
// Depends on: report.go (fetch/strict/CSV), identity.read_report_funnel / export_report (migrations/0147_reports.sql).
// Used by: internal/httpapi/reports.go.
// Invariants: stages are nested (claimed >= link_sent >= ordered >= paid), so a funnel never grows toward the right; no actor key, no buyer data.
// Status: REAL_PG (MOCK data).

package reporting

import (
	"context"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// FunnelReport counts bundles. OrderedWithoutLink is the ordered bundles whose link the system did not send (merchant-created or hand-delivered).
type FunnelReport struct {
	From               string  `json:"from"`
	To                 string  `json:"to"`
	Timezone           string  `json:"timezone"`
	SessionID          *string `json:"session_id"`
	Claimed            int64   `json:"claimed"`
	LinkSent           int64   `json:"link_sent"`
	Ordered            int64   `json:"ordered"`
	Paid               int64   `json:"paid"`
	OrderedWithoutLink int64   `json:"ordered_without_link"`
}

// Funnel reads the funnel (orders:read AND live:read, no write); session "" = whole store.
func Funnel(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, from, to, session string) (FunnelReport, error) {
	return funnelFetch(ctx, tx, scope, token, from, to, session, false)
}

// FunnelCSV is Funnel as a one-row CSV through identity.export_report (orders:export AND orders:read AND live:read, one audit row reports.exported).
func FunnelCSV(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, from, to, session string) ([]byte, error) {
	rep, err := funnelFetch(ctx, tx, scope, token, from, to, session, true)
	if err != nil {
		return nil, err
	}
	return writeCSV([]string{"session_id", "claimed", "link_sent", "ordered", "paid", "ordered_without_link"},
		[][]string{{deref(rep.SessionID), i64(rep.Claimed), i64(rep.LinkSent), i64(rep.Ordered), i64(rep.Paid), i64(rep.OrderedWithoutLink)}})
}

func funnelFetch(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, from, to, session string, export bool) (FunnelReport, error) {
	raw, err := fetch(ctx, tx, scope, token, reportFunnel, from, to, session, export)
	if err != nil {
		return FunnelReport{}, err
	}
	var rep FunnelReport
	if strict(raw, &rep) != nil || !validHead(rep.From, rep.To, rep.Timezone, from, to) ||
		(session == "") != (rep.SessionID == nil) || (rep.SessionID != nil && (*rep.SessionID != session || !command.ValidID(session))) ||
		rep.Paid < 0 || rep.Paid > rep.Ordered || rep.Ordered > rep.LinkSent || rep.LinkSent > rep.Claimed ||
		rep.OrderedWithoutLink < 0 || rep.LinkSent+rep.OrderedWithoutLink > rep.Claimed {
		return FunnelReport{}, ErrUnavailable
	}
	return rep, nil
}
