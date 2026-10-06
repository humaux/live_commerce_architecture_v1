// channels.go serves the W6-02B channel report: orders and money per sales channel (facebook_live, instagram_live, storefront, manual) and
// currency, from identity.read_report_channels (contracts/reporting-v2.md §3: one order, exactly one channel).
//
// Purpose: strict decoding and re-verification of the channel buckets, and their CSV.
// Depends on: report.go (fetch/strict/Money/CSV), identity.read_report_channels / export_report (migrations/0147_reports.sql).
// Used by: internal/httpapi/reports.go.
// Invariants: I05 (money per currency and environment; offline money never in captured/net); sums over channels equal read_finance_summary.
// Status: REAL_PG (MOCK data).

package reporting

import (
	"context"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/platform"
)

// Channels is the closed channel vocabulary, in report order.
var Channels = []string{"facebook_live", "instagram_live", "storefront", "manual"}

// ChannelRow is one (channel, currency): non-cancelled orders created in the range, cancelled ones, and the money of facts dated in the range.
type ChannelRow struct {
	Channel         string  `json:"channel"`
	Currency        string  `json:"currency"`
	Orders          int64   `json:"orders"`
	CancelledOrders int64   `json:"cancelled_orders"`
	Money           []Money `json:"money"`
}

// ChannelReport is the whole channel answer for a range.
type ChannelReport struct {
	From     string       `json:"from"`
	To       string       `json:"to"`
	Timezone string       `json:"timezone"`
	Rows     []ChannelRow `json:"rows"`
}

// ChannelsReport reads the channel report (orders:read, no write).
func ChannelsReport(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, from, to string) (ChannelReport, error) {
	return channelsFetch(ctx, tx, scope, token, from, to, false)
}

// ChannelsCSV is ChannelsReport as CSV through identity.export_report (orders:export AND orders:read, one audit row reports.exported); one CSV row
// per (channel, currency, environment).
func ChannelsCSV(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, from, to string) ([]byte, error) {
	rep, err := channelsFetch(ctx, tx, scope, token, from, to, true)
	if err != nil {
		return nil, err
	}
	var rows [][]string
	for _, r := range rep.Rows {
		rows = append(rows, moneyRows(r.Money, func(m Money) []string {
			return []string{r.Channel, r.Currency, i64(r.Orders), i64(r.CancelledOrders), m.Environment, i64(m.CapturedCount), i64(m.CapturedMinor),
				i64(m.RefundedMinor), i64(m.NetMinor), i64(m.OfflineCount), i64(m.OfflineMinor)}
		})...)
	}
	return writeCSV([]string{"channel", "currency", "orders", "cancelled_orders", "environment", "captured_count", "captured_minor",
		"refunded_minor", "net_minor", "offline_count", "offline_minor"}, rows)
}

func channelsFetch(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, from, to string, export bool) (ChannelReport, error) {
	raw, err := fetch(ctx, tx, scope, token, reportChannels, from, to, "", export)
	if err != nil {
		return ChannelReport{}, err
	}
	var rep ChannelReport
	if strict(raw, &rep) != nil || !validHead(rep.From, rep.To, rep.Timezone, from, to) {
		return ChannelReport{}, ErrUnavailable
	}
	last := -1
	for _, r := range rep.Rows {
		at := indexOf(Channels, r.Channel)
		if at < 0 || at < last || !currencyCode.MatchString(r.Currency) || r.Orders < 0 || r.CancelledOrders < 0 || !validMoney(r.Money) {
			return ChannelReport{}, ErrUnavailable
		}
		last = at
	}
	if rep.Rows == nil {
		rep.Rows = []ChannelRow{}
	}
	return rep, nil
}

func indexOf(list []string, v string) int {
	for i, x := range list {
		if x == v {
			return i
		}
	}
	return -1
}
