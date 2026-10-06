// products.go serves the W6-02B product report: per SKU units, captured, refunded, net and offline money of a range, from
// identity.read_report_products (payment facts spread over the FROZEN order snapshot lines; contracts/reporting-v2.md §2).
//
// Purpose: strict decoding and re-verification of the product rows, and their CSV.
// Depends on: report.go (fetch/strict/CSV), identity.read_report_products / export_report (migrations/0147_reports.sql).
// Used by: internal/httpapi/reports.go.
// Invariants: I05 (net = captured - refunded per row; per currency and environment), at most MaxProductRows rows, truncation is declared.
// Status: REAL_PG (MOCK data).

package reporting

import (
	"context"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// ProductRow is one (SKU, currency, environment). Units are card-paid units (orders with a CAPTURED fact in the range); OfflineUnits and
// OfflineMinor are COD / pay-at-pickup / bank-transfer money, never part of captured or net.
type ProductRow struct {
	SKUID         string `json:"sku_id"`
	ProductID     string `json:"product_id"`
	Code          string `json:"code"`
	Name          string `json:"name"`
	Currency      string `json:"currency"`
	Environment   string `json:"environment"`
	Units         int64  `json:"units"`
	CapturedMinor int64  `json:"captured_minor"`
	RefundedMinor int64  `json:"refunded_minor"`
	NetMinor      int64  `json:"net_minor"`
	OfflineUnits  int64  `json:"offline_units"`
	OfflineMinor  int64  `json:"offline_minor"`
}

// ProductReport lists rows by net descending; Truncated says more than MaxProductRows existed.
type ProductReport struct {
	From      string       `json:"from"`
	To        string       `json:"to"`
	Timezone  string       `json:"timezone"`
	Truncated bool         `json:"truncated"`
	Rows      []ProductRow `json:"rows"`
}

// Products reads the product report (orders:read, no write).
func Products(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, from, to string) (ProductReport, error) {
	return productsFetch(ctx, tx, scope, token, from, to, false)
}

// ProductsCSV is Products as CSV through identity.export_report (orders:export AND orders:read, one audit row reports.exported).
func ProductsCSV(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, from, to string) ([]byte, error) {
	rep, err := productsFetch(ctx, tx, scope, token, from, to, true)
	if err != nil {
		return nil, err
	}
	rows := make([][]string, len(rep.Rows))
	for i, r := range rep.Rows {
		rows[i] = []string{r.SKUID, r.ProductID, cell(r.Code), cell(r.Name), r.Currency, r.Environment, i64(r.Units), i64(r.CapturedMinor),
			i64(r.RefundedMinor), i64(r.NetMinor), i64(r.OfflineUnits), i64(r.OfflineMinor)}
	}
	return writeCSV([]string{"sku_id", "product_id", "code", "name", "currency", "environment", "units", "captured_minor", "refunded_minor",
		"net_minor", "offline_units", "offline_minor"}, rows)
}

func productsFetch(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, from, to string, export bool) (ProductReport, error) {
	raw, err := fetch(ctx, tx, scope, token, reportProducts, from, to, "", export)
	if err != nil {
		return ProductReport{}, err
	}
	return decodeProducts(raw, from, to)
}

func decodeProducts(raw []byte, from, to string) (ProductReport, error) {
	var rep ProductReport
	if strict(raw, &rep) != nil || !validHead(rep.From, rep.To, rep.Timezone, from, to) || len(rep.Rows) > MaxProductRows || (rep.Truncated && len(rep.Rows) != MaxProductRows) {
		return ProductReport{}, ErrUnavailable
	}
	for i, r := range rep.Rows {
		if !command.ValidID(r.SKUID) || !command.ValidID(r.ProductID) || !currencyCode.MatchString(r.Currency) || !validEnv(r.Environment) ||
			r.Units < 0 || r.CapturedMinor < 0 || r.RefundedMinor < 0 || r.OfflineUnits < 0 || r.OfflineMinor < 0 || r.NetMinor != r.CapturedMinor-r.RefundedMinor ||
			(i > 0 && rep.Rows[i-1].NetMinor < r.NetMinor) {
			return ProductReport{}, ErrUnavailable
		}
	}
	if rep.Rows == nil {
		rep.Rows = []ProductRow{}
	}
	return rep, nil
}
