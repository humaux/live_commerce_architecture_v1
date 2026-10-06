// Purpose: the merchant read of one customer's historical-order archive (W5-03B): GET customers/{id}/historical-orders, a keyset-paged
//   list of the SHOPLINE orders the merchant imported for that customer. The archive is display-only data: it is not an order, carries
//   no payment detail and is never summed into any finance or report figure (I05).
// Depends on: SQL customers.read_historical_orders (customers:read, migration 0156); internal/pagination (collection
//   "customer-historical-orders"), internal/command, internal/platform.
// Used by: internal/httpapi/customer_historical.go (the route); the privacy export embeds the same item shape (privacy.go).
// Invariants: customers-billing-v1 / migration-import-v1 section 7: an unknown, erased or other-store customer is the not-found class;
//   currency is always TWD; the response never carries a street address, phone, email or payment method (the table has no such column).

package customers

import (
	"context"
	"crypto/sha256"
	"strings"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/command"
	"livecommerce/internal/pagination"
	"livecommerce/internal/platform"
)

// HistoricalOrder is one archived order: display facts only. TotalMinor is TWD minor units (NT$ x 100) and is NOT revenue.
type HistoricalOrder struct {
	OrderID      string  `json:"order_id"`
	OrderedAt    string  `json:"ordered_at"`
	Status       string  `json:"status"`
	TotalMinor   int64   `json:"total_minor"`
	Currency     string  `json:"currency"`
	ItemsSummary string  `json:"items_summary"`
	City         *string `json:"city"`
}

// HistoricalOrdersPage is the list answer: the page, the next cursor ("" at the end) and the customer's total archive size.
type HistoricalOrdersPage struct {
	Items      []HistoricalOrder `json:"items"`
	NextCursor string            `json:"next_cursor"`
	Total      int64             `json:"total"`
}

// historicalRow is the definer's item: the public shape plus the row id the keyset needs.
type historicalRow struct {
	HistoricalOrder
	ID string `json:"id"`
}

func validHistoricalOrder(h HistoricalOrder) bool {
	if _, err := canonicalTime(h.OrderedAt); err != nil {
		return false
	}
	return h.OrderID != "" && len([]rune(h.OrderID)) <= 64 && h.Status != "" && len([]rune(h.Status)) <= 40 && h.TotalMinor >= 0 &&
		h.Currency == "TWD" && len([]rune(h.ItemsSummary)) <= 500 && (h.City == nil || (*h.City != "" && len([]rune(*h.City)) <= 20 && !strings.ContainsAny(*h.City, "0123456789")))
}

// ListHistoricalOrders pages one customer's archive newest first (customers:read; DB: customers.read_historical_orders). The cursor is
// bound to tenant, store and customer; an unknown, erased or other-store customer is the not-found class (command.ErrNotFound).
func ListHistoricalOrders(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, customerID string, req pagination.Request) (HistoricalOrdersPage, error) {
	empty := HistoricalOrdersPage{Items: []HistoricalOrder{}}
	if tx == nil || !validAuthorityInput(scope, token) || !command.ValidID(customerID) {
		return empty, command.ErrInvalid
	}
	binding := pagination.Binding{TenantID: scope.TenantID, StoreID: scope.StoreID, Collection: "customer-historical-orders", ParentID: customerID}
	limit, keys, err := pagination.Decode(req, binding, 2)
	if err != nil {
		return empty, err
	}
	var afterTime, afterID any
	if len(keys) == 2 {
		afterTime, afterID = keys[0], keys[1]
	}
	hash := sha256.Sum256([]byte(token))
	var raw []byte
	// customers.read_historical_orders: customers:read, customer must be visible to the merchant list, authority fenced after the read.
	if err = tx.QueryRow(ctx, `SELECT customers.read_historical_orders($1,$2::uuid,$3::uuid,$4,$5::timestamptz,$6::uuid)`,
		hash[:], scope.StoreID, customerID, limit+1, afterTime, afterID).Scan(&raw); err != nil {
		return empty, mapMerchantError(err)
	}
	if err = platform.RequirePermission(ctx, tx, scope, token, "customers:read"); err != nil {
		return empty, mapMerchantError(err)
	}
	var doc struct {
		Total int64           `json:"total"`
		Items []historicalRow `json:"items"`
	}
	if exactKeys(raw, "total", "items") != nil || decodeInto(raw, &doc) != nil || doc.Items == nil || len(doc.Items) > limit+1 || doc.Total < int64(len(doc.Items)) {
		return empty, ErrUnavailable
	}
	for _, r := range doc.Items {
		if !command.ValidID(r.ID) || !validHistoricalOrder(r.HistoricalOrder) {
			return empty, ErrUnavailable
		}
	}
	page := HistoricalOrdersPage{Items: make([]HistoricalOrder, 0, len(doc.Items)), Total: doc.Total}
	for i, r := range doc.Items {
		if i == limit {
			last := doc.Items[limit-1]
			if page.NextCursor, err = pagination.Encode(binding, []string{last.OrderedAt, last.ID}); err != nil {
				return empty, ErrUnavailable
			}
			break
		}
		page.Items = append(page.Items, r.HistoricalOrder)
	}
	return page, nil
}
