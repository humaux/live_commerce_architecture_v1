// dashboard.go assembles the admin landing read model (contract G1) in the caller's one platform.WithScope(orders:read) transaction:
// placed-order counts (identity.dashboard_orders), GMV by payment mode (internal/reporting.Finance re-cut to today / 7 days, so there is
// no second ledger), the to-do counters (identity.dashboard_todos plus one scoped low-stock read), and the ten latest orders
// (internal/merchantorders.List).
//
// Non-goals: no write of any kind, no cache, no currency conversion or cross-environment sum (a GMV entry is one currency and one
// environment, I05), no new permission. Every block is bounded by the existing indexes (merchant_orders_history, orders_unshipped,
// bank_transfers_open), so the whole read stays well under the 300 ms budget on 10k orders (measured in the PG smoke).
// Tables read under RLS as commerce_runtime: catalog.skus/products, inventory.balances (low stock only); the rest are definers.

package merchanttools

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/command"
	"livecommerce/internal/merchantorders"
	"livecommerce/internal/pagination"
	"livecommerce/internal/platform"
	"livecommerce/internal/reporting"
)

// lowStockThreshold: available units at or below this count a SKU as low (the buyer "low" band is 1..5, plus out of stock).
const lowStockThreshold = 5

// ModeTotals is GMV minor units by payment mode for one window.
type ModeTotals struct {
	CardMinor         int64 `json:"card_minor"`
	BankTransferMinor int64 `json:"bank_transfer_minor"`
	PayAtPickupMinor  int64 `json:"pay_at_pickup_minor"`
}

// GMV is one (currency, environment) entry; never summed with another.
type GMV struct {
	Currency    string     `json:"currency"`
	Environment string     `json:"environment"`
	Today       ModeTotals `json:"today"`
	Last7Days   ModeTotals `json:"last_7_days"`
}

type OrderCounts struct {
	Today     int64 `json:"today"`
	Last7Days int64 `json:"last_7_days"`
}

type Todos struct {
	AwaitingTransferConfirmation int64 `json:"awaiting_transfer_confirmation"`
	ToShip                       int64 `json:"to_ship"`
	CVSAwaitingLabel             int64 `json:"cvs_awaiting_label"`
	LowStockSKUs                 int64 `json:"low_stock_skus"`
	OpenRefunds                  int64 `json:"open_refunds"`
}

// DashboardData is the GET dashboard body.
type DashboardData struct {
	GeneratedAt  string                   `json:"generated_at"`
	Timezone     string                   `json:"timezone"`
	Orders       OrderCounts              `json:"orders"`
	GMV          []GMV                    `json:"gmv"`
	Todos        Todos                    `json:"todos"`
	LatestOrders []merchantorders.Summary `json:"latest_orders"`
}

// Dashboard runs the five blocks. token is the merchant bearer the definers re-verify (second authority fence).
func Dashboard(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string) (DashboardData, error) {
	out := DashboardData{Timezone: reporting.Timezone, GMV: []GMV{}, LatestOrders: []merchantorders.Summary{}}
	if tx == nil || !command.ValidID(scope.TenantID) || !command.ValidID(scope.StoreID) || !command.ValidID(scope.PrincipalID) ||
		scope.Revision < 1 || len(token) < 32 || len(token) > 512 {
		return out, command.ErrInvalid
	}
	hash := sha256.Sum256([]byte(token))
	var today string
	var now time.Time
	if err := tx.QueryRow(ctx, `SELECT to_char((clock_timestamp() AT TIME ZONE 'Asia/Taipei')::date,'YYYY-MM-DD'),clock_timestamp()`).Scan(&today, &now); err != nil {
		return out, mapDashboardError(err)
	}
	out.GeneratedAt = now.UTC().Format(time.RFC3339)

	// identity.dashboard_orders (0094): placed-order counts, orders:read re-verified inside.
	if err := definerJSON(ctx, tx, `SELECT identity.dashboard_orders($1,$2::uuid)`, hash[:], scope.StoreID, &out.Orders); err != nil {
		return out, err
	}
	// reporting.Finance -> identity.read_finance_summary (0078/0085/0088): the only money source; re-cut here, never recomputed.
	first, err := time.Parse("2006-01-02", today)
	if err != nil {
		return out, ErrUnavailable
	}
	summary, err := reporting.Finance(ctx, tx, scope, token, first.AddDate(0, 0, -6).Format("2006-01-02"), today)
	if err != nil {
		return out, err
	}
	out.GMV = cutGMV(summary.Rows, today)
	var todos struct {
		AwaitingTransferConfirmation int64 `json:"awaiting_transfer_confirmation"`
		ToShip                       int64 `json:"to_ship"`
		CVSAwaitingLabel             int64 `json:"cvs_awaiting_label"`
		OpenRefunds                  int64 `json:"open_refunds"`
	}
	// identity.dashboard_todos (0094): bank transfers SUBMITTED, the unshipped predicate, CVS without a live label, open Stripe refunds.
	if err = definerJSON(ctx, tx, `SELECT identity.dashboard_todos($1,$2::uuid)`, hash[:], scope.StoreID, &todos); err != nil {
		return out, err
	}
	out.Todos = Todos{todos.AwaitingTransferConfirmation, todos.ToShip, todos.CVSAwaitingLabel, 0, todos.OpenRefunds}
	// Low stock: a plain scoped read (catalog.skus, catalog.products, inventory.balances are readable by commerce_runtime under RLS);
	// a SKU without any balance row counts as available 0.
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM (
		SELECT s.id FROM catalog.skus s
		JOIN catalog.products p ON p.tenant_id=s.tenant_id AND p.store_id=s.store_id AND p.id=s.product_id
		LEFT JOIN inventory.balances b ON b.tenant_id=s.tenant_id AND b.store_id=s.store_id AND b.sku_id=s.id
		WHERE s.tenant_id=$1 AND s.store_id=$2 AND s.status='active' AND p.status='active'
		GROUP BY s.id HAVING coalesce(sum(b.on_hand-b.reserved-b.allocated-b.unavailable),0)<=$3) low`,
		scope.TenantID, scope.StoreID, lowStockThreshold).Scan(&out.Todos.LowStockSKUs); err != nil {
		return out, mapDashboardError(err)
	}
	// merchantorders.List -> identity.read_merchant_orders: the same projection the Orders page shows, newest first.
	page, err := merchantorders.List(ctx, tx, scope, token, merchantorders.ListRequest{Page: pagination.Request{Limit: 10}, State: "all"})
	if err != nil {
		return out, err
	}
	out.LatestOrders = page.Items
	return out, nil
}

// cutGMV folds finance day rows into one entry per (currency, environment): the row of `today` feeds both windows, every other row of
// the seven-day range feeds the 7-day window only. card = captured, bank = confirmed transfers, pickup = collected.
func cutGMV(rows []reporting.FinanceRow, today string) []GMV {
	index := map[[2]string]int{}
	out := []GMV{}
	for _, r := range rows {
		key := [2]string{r.Currency, r.Environment}
		i, ok := index[key]
		if !ok {
			i = len(out)
			index[key] = i
			out = append(out, GMV{Currency: r.Currency, Environment: r.Environment})
		}
		add := func(m *ModeTotals) {
			m.CardMinor += r.CapturedMinor
			m.BankTransferMinor += r.BankTransferConfirmedMinor
			m.PayAtPickupMinor += r.PickupCollectedMinor
		}
		add(&out[i].Last7Days)
		if r.Day == today {
			add(&out[i].Today)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Currency != out[j].Currency {
			return out[i].Currency < out[j].Currency
		}
		return out[i].Environment < out[j].Environment
	})
	return out
}

// definerJSON runs one dashboard definer and strictly decodes its object (unknown keys are drift, not data).
func definerJSON(ctx context.Context, tx pgx.Tx, query string, hash []byte, store string, into any) error {
	var raw []byte
	if err := tx.QueryRow(ctx, query, hash, store).Scan(&raw); err != nil {
		return mapDashboardError(err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		return ErrUnavailable
	}
	return nil
}

// mapDashboardError turns the definers' PT401/PT403/PT404 into the platform sentinels the HTTP layer already classifies; anything else
// is an unavailable read (no driver text leaves).
func mapDashboardError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "PT401":
			return platform.ErrUnauthorized
		case "PT403":
			return platform.ErrForbidden
		case "PT404":
			return platform.ErrScopeNotFound
		}
	}
	if errors.Is(err, platform.ErrUnauthorized) || errors.Is(err, platform.ErrForbidden) || errors.Is(err, platform.ErrScopeNotFound) {
		return err
	}
	return abort(err)
}
