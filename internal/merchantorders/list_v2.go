// Purpose: the merchant orders v2 list read (GET /orders?view=v2 and POST /orders/search): filter normalization, cursor binding and the
//   strict decode of each row (order_number, masked recipient, delivery kind, live sessions); normalizeRecipientMask is the single
//   recipient-mask rule, shared with the open parcel-group read.
// Depends on: identity.read_merchant_orders_v2 (migration 0110, READ COMMITTED scope transaction), pagination, command, platform.
// Used by: internal/httpapi/orders.go (list + search routes), internal/merchantorders/parcels.go (normalizeRecipientMask).
// Invariants: read only; a malformed recipient mask degrades that one row to the placeholder (hides, never reveals) and never
//   fails the whole list.

package merchantorders

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/command"
	"livecommerce/internal/pagination"
	"livecommerce/internal/platform"
)

// ListV2Request is read-only. Dates are inclusive Taipei calendar days, not the
// browser's local zone; every normalized filter is part of the cursor binding.
type ListV2Request struct {
	Page                                                       pagination.Request
	State, Bucket, Query, Payment, Delivery, Session, From, To string
}
type OrderSession struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type SummaryV2 struct {
	Summary
	OrderNumber     string         `json:"order_number"`
	RecipientMasked string         `json:"recipient_masked"`
	DeliveryKind    string         `json:"delivery_kind"`
	LiveSessions    []OrderSession `json:"live_sessions"`
	ItemSummary     ItemSummary    `json:"item_summary"`
}

// ItemSummary is a bounded projection of immutable checkout merchandise, never recipient data.
type ItemSummary struct {
	Items          []SummaryItem `json:"items"`
	TotalItemCount int64         `json:"total_item_count"`
}
type SummaryItem struct {
	ProductName  string  `json:"product_name"`
	VariantLabel *string `json:"variant_label"`
	Quantity     int64   `json:"quantity"`
}

type PageV2 struct {
	Items      []SummaryV2      `json:"items"`
	NextCursor string           `json:"next_cursor"`
	Total      int64            `json:"total"`
	Counts     map[string]int64 `json:"counts"`
	Sessions   []OrderSession   `json:"sessions"`
}

var orderBuckets = []string{"all", "unpaid", "transfer_review", "ready_to_ship", "ready_to_consign", "shipped", "completed", "cancelled"}
var orderDeliveries = []string{"home", "cvs_711", "cvs_familymart", "cvs_hilife", "cvs_okmart"}

func (in *ListV2Request) Normalize() error {
	if in.State == "" {
		in.State = "active"
	}
	if in.Bucket == "" {
		in.Bucket = "all"
	}
	if (in.State != "active" && !validState(in.State) && in.State != "cvs_pending") || !slices.Contains(orderBuckets, in.Bucket) ||
		!textValue(in.Query, 80, false) || strings.TrimSpace(in.Query) != in.Query ||
		(in.Payment != "" && !slices.Contains([]string{"card", "bank_transfer", "pay_at_pickup", "cash_on_delivery"}, in.Payment)) ||
		(in.Delivery != "" && !slices.Contains(orderDeliveries, in.Delivery)) || (in.Session != "" && !command.ValidID(in.Session)) {
		return command.ErrInvalid
	}
	a, err := orderDay(in.From)
	if err != nil {
		return err
	}
	b, err := orderDay(in.To)
	if err != nil {
		return err
	}
	if a != nil && b != nil && a.After(*b) {
		return command.ErrInvalid
	}
	return nil
}
func orderDay(day string) (*time.Time, error) {
	if day == "" {
		return nil, nil
	}
	t, err := time.ParseInLocation("2006-01-02", day, time.FixedZone("Asia/Taipei", 8*60*60))
	if err != nil || t.Format("2006-01-02") != day || t.Year() < 2000 || t.Year() > 2199 {
		return nil, command.ErrInvalid
	}
	return &t, nil
}

func ListV2(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, in ListV2Request) (PageV2, error) {
	empty := PageV2{Items: []SummaryV2{}, Counts: map[string]int64{}, Sessions: []OrderSession{}}
	if tx == nil || !validAuthorityInput(scope, token) {
		return empty, command.ErrInvalid
	}
	if err := in.Normalize(); err != nil {
		return empty, err
	}
	filters, _ := json.Marshal([]string{in.State, in.Bucket, in.Query, in.Payment, in.Delivery, in.Session, in.From, in.To})
	digest := sha256.Sum256(filters)
	binding := pagination.Binding{TenantID: scope.TenantID, StoreID: scope.StoreID, Collection: "merchant-orders-v2", Filter: hex.EncodeToString(digest[:])}
	limit, keys, err := pagination.Decode(in.Page, binding, 2)
	if err != nil {
		return empty, err
	}
	var afterTime, afterID, session any
	if len(keys) == 2 {
		afterTime, afterID = keys[0], keys[1]
	}
	if in.Session != "" {
		session = in.Session
	}
	from, _ := orderDay(in.From)
	until, _ := orderDay(in.To)
	if until != nil {
		end := until.AddDate(0, 0, 1)
		until = &end
	}
	hash := sha256.Sum256([]byte(token))
	var raw []byte
	// identity.read_merchant_orders_v2 owns the single-snapshot page+counts and final
	// fresh auth fence; no direct table access is granted to this runtime.
	err = tx.QueryRow(ctx, `SELECT identity.read_merchant_orders_v2($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		hash[:], scope.StoreID, limit+1, afterTime, afterID, in.State, in.Bucket, in.Query, in.Payment, in.Delivery, session, from, until).Scan(&raw)
	if err != nil {
		return empty, mapError(err)
	}
	if err = platform.RequirePermission(ctx, tx, scope, token, "orders:read"); err != nil {
		return empty, err
	}
	if len(raw) > 128<<10 {
		return empty, ErrUnavailable
	}
	fields, err := exact(raw, "items", "total", "counts", "sessions")
	if err != nil {
		return empty, err
	}
	if json.Unmarshal(fields["total"], &empty.Total) != nil || !money(empty.Total) ||
		json.Unmarshal(fields["counts"], &empty.Counts) != nil || len(empty.Counts) != len(orderBuckets) ||
		json.Unmarshal(fields["sessions"], &empty.Sessions) != nil || !validOrderSessions(empty.Sessions, 100) {
		return PageV2{}, ErrUnavailable
	}
	for _, key := range orderBuckets {
		n, ok := empty.Counts[key]
		if !ok || !money(n) {
			return PageV2{}, ErrUnavailable
		}
	}
	if empty.Counts[in.Bucket] != empty.Total {
		return PageV2{}, ErrUnavailable
	}
	objects, err := decodeArray(fields["items"], limit+1)
	if err != nil {
		return PageV2{}, err
	}
	seen := map[string]bool{}
	for _, object := range objects {
		var parts map[string]json.RawMessage
		if json.Unmarshal(object, &parts) != nil {
			return PageV2{}, ErrUnavailable
		}
		var row SummaryV2
		for key, target := range map[string]any{"order_number": &row.OrderNumber, "recipient_masked": &row.RecipientMasked, "delivery_kind": &row.DeliveryKind, "live_sessions": &row.LiveSessions, "source": &row.Source} {
			if json.Unmarshal(parts[key], target) != nil {
				return PageV2{}, ErrUnavailable
			}
			delete(parts, key)
		}
		src := row.Source
		rest, _ := json.Marshal(parts)
		row.Summary, err = decodeSummary(rest)
		row.Source = src
		// A mask that is not exactly one printable rune + "***" (an unprintable first rune in a legacy name, or a projection
		// bug) degrades THIS row to the placeholder: it hides, never reveals, and never makes the whole store list 503.
		row.RecipientMasked = normalizeRecipientMask(row.RecipientMasked)
		if err != nil || seen[row.OrderID] || (src != "storefront" && src != "merchant_manual") ||
			row.OrderNumber != "LC-"+strings.ToUpper(strings.ReplaceAll(row.OrderID, "-", "")) ||
			(row.DeliveryKind != "unknown" && !slices.Contains(orderDeliveries, row.DeliveryKind)) || !validOrderSessions(row.LiveSessions, 100) {
			return PageV2{}, ErrUnavailable
		}
		seen[row.OrderID] = true
		empty.Items = append(empty.Items, row)
	}
	if int64(len(empty.Items)) > empty.Total {
		return PageV2{}, ErrUnavailable
	}
	if len(empty.Items) > limit {
		empty.Items = empty.Items[:limit]
		last := empty.Items[len(empty.Items)-1]
		empty.NextCursor, err = pagination.Encode(binding, []string{last.CreatedAt, last.OrderID})
		if err != nil {
			return PageV2{}, ErrUnavailable
		}
	}
	if err := attachItemSummaries(ctx, tx, scope, hash[:], empty.Items); err != nil {
		return PageV2{}, err
	}
	if err := platform.RequirePermission(ctx, tx, scope, token, "orders:read"); err != nil {
		return PageV2{}, err
	}
	return empty, nil
}

// attachItemSummaries uses one bounded query and the existing scoped reader. Only
// merchandise is projected in SQL: destination/phone/address never leave PG.
// Historical snapshots have no variant label; NULL is deliberate, never a live
// catalog lookup or a guess based on SKU code. Each nested reader rechecks auth.
func attachItemSummaries(ctx context.Context, tx pgx.Tx, scope platform.Scope, hash []byte, items []SummaryV2) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]string, len(items))
	for i := range items {
		ids[i] = items[i].OrderID
	}
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT jsonb_object_agg(order_id, summary) FROM (
 SELECT id AS order_id, jsonb_build_object(
  'items', (SELECT jsonb_agg(jsonb_build_object(
    'product_name', line->'name', 'variant_label', NULL, 'quantity', line->'quantity') ORDER BY position)
    FROM jsonb_array_elements(detail->'items') WITH ORDINALITY AS lines(line, position) WHERE position<=3),
  'total_item_count', (SELECT sum((line->>'quantity')::bigint) FROM jsonb_array_elements(detail->'items') AS lines(line))
 ) AS summary
 FROM unnest($3::uuid[]) AS orders(id)
 CROSS JOIN LATERAL jsonb_array_elements(identity.read_merchant_orders($1,$2::uuid,id,1,NULL,NULL,'all')) AS details(detail)
) AS summaries`, hash, scope.StoreID, ids).Scan(&raw)
	if err != nil {
		return mapError(err)
	}
	var summaries map[string]ItemSummary
	if len(raw) > 128<<10 || json.Unmarshal(raw, &summaries) != nil || len(summaries) != len(items) {
		return ErrUnavailable
	}
	for i := range items {
		summary, ok := summaries[items[i].OrderID]
		if !ok || len(summary.Items) == 0 || len(summary.Items) > 3 || summary.TotalItemCount < 1 || !money(summary.TotalItemCount) {
			return ErrUnavailable
		}
		var shown int64
		for _, item := range summary.Items {
			if !textValue(item.ProductName, 200, true) || item.VariantLabel != nil || item.Quantity < 1 || !money(item.Quantity) {
				return ErrUnavailable
			}
			var sumErr error
			shown, sumErr = add(shown, item.Quantity)
			if sumErr != nil {
				return ErrUnavailable
			}
		}
		if shown > summary.TotalItemCount {
			return ErrUnavailable
		}
		items[i].ItemSummary = summary
	}
	return nil
}

func validOrderSessions(items []OrderSession, max int) bool {
	if items == nil || len(items) > max {
		return false
	}
	seen := map[string]bool{}
	for _, item := range items {
		if !command.ValidID(item.ID) || !textValue(item.Name, 200, true) || seen[item.ID] {
			return false
		}
		seen[item.ID] = true
	}
	return true
}

// normalizeRecipientMask keeps a recipient_masked projection only when it is "—" or exactly one printable rune + "***"; anything else
// (an unprintable first rune in a legacy name, or a projection bug) degrades to the placeholder: it hides, never reveals. Shared by the
// orders list and the open parcel-group read so both show the same mask.
func normalizeRecipientMask(mask string) string {
	if mask != "—" && (!textValue(mask, 4, true) || len([]rune(mask)) != 4 || !strings.HasSuffix(mask, "***")) {
		return "—"
	}
	return mask
}
