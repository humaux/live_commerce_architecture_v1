// Purpose: the pick list (contract amendment W3-02B §1) — one read-only projection over
//   fulfillment.read_pick_list that summarises SKU quantities and per-order lines for browser printing.
//   It never writes, never changes state and never logs recipient data.
// Depends on: fulfillment.read_pick_list + claims.pick_list_session_orders (0130), fulfillment.read_parcel_group_ids (0146, parcel group
//   annotation via parcels.go), identity.resolve_access
//   (orders:read), platform.WithScope, command (id validation), the shared row decoder also used by carrier_export.go.
// Used by: internal/httpapi/picklist.go (pick-list route). Tests: TestPickList/TestPickList500.
// Invariants: collection rule CONFIRMED|AWAITING_COLLECTION AND MANUAL_UNASSIGNED (0130); skipped codes
//   not_pickable|order_not_found; <=500 ids (501 -> 422 too_many); rows sorted by sku_code.

package merchantorders

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

const pickListMaxOrders = 500

// ErrPickListTooMany is the 422 too_many refusal (PL03): the SQL reader raises PT422 for the same
// bound and Go checks it first so a 501-array never reaches the database.
var ErrPickListTooMany = errors.New("too many pick list orders")

var orderNumberRE = regexp.MustCompile(`^LC-[0-9A-F]{32}$`)

// PickListSelection is exactly one of {order_ids} (<= 500) or {session_id}.
type PickListSelection struct {
	OrderIDs  []string
	SessionID string
}

// PickListLine is one SKU line; OptionLabel is always null (the catalog has no variant model).
type PickListLine struct {
	SKUID       string  `json:"sku_id"`
	SKUCode     string  `json:"sku_code"`
	Title       string  `json:"title"`
	OptionLabel *string `json:"option_label"`
	Qty         int64   `json:"qty"`
}

// PickListOrder is one order of the list; ParcelGroupID is set (and the group's orders are adjacent) when the order sits in a
// W3-07B parcel group, so the picker packs them into one parcel.
type PickListOrder struct {
	OrderID       string         `json:"order_id"`
	OrderNumber   string         `json:"order_number"`
	ParcelGroupID string         `json:"parcel_group_id,omitempty"`
	Lines         []PickListLine `json:"lines"`
}

type PickListSkipped struct {
	OrderID string `json:"order_id"`
	Code    string `json:"code"`
}

// PickListResult is the 200 body; generated_at is stamped in Go, not by the SQL reader.
type PickListResult struct {
	GeneratedAt string            `json:"generated_at"`
	Orders      []PickListOrder   `json:"orders"`
	Totals      []PickListLine    `json:"totals"`
	Skipped     []PickListSkipped `json:"skipped"`
}

// pickListRow is one fulfillment.read_pick_list order: the pick-list lines plus the frozen
// destination/PII/money fields the carrier CSV reuses (carrier_export.go).
type pickListRow struct {
	OrderID         string         `json:"order_id"`
	OrderNumber     string         `json:"order_number"`
	CreatedAtUTC    string         `json:"created_at_utc"`
	ServiceCode     string         `json:"service_code"`
	DestinationKind string         `json:"destination_kind"`
	RecipientName   string         `json:"recipient_name"`
	Phone           string         `json:"phone"`
	Country         string         `json:"country"`
	Region          string         `json:"region"`
	City            string         `json:"city"`
	PostalCode      string         `json:"postal_code"`
	Line1           string         `json:"line1"`
	Line2           string         `json:"line2"`
	PickupNamespace string         `json:"pickup_namespace"`
	PickupCode      string         `json:"pickup_code"`
	PickupName      string         `json:"pickup_name"`
	PickupAddress   string         `json:"pickup_address"`
	PickupSource    string         `json:"pickup_source"`
	PaymentMode     string         `json:"payment_mode"`
	TotalMinor      int64          `json:"total_minor"`
	Currency        string         `json:"currency"`
	CollectMinor    int64          `json:"collect_minor"`
	Lines           []PickListLine `json:"lines"`
	ParcelGroupID   string         `json:"-"` // W3-07B: filled by groupAdjacent, never by the SQL reader
}

type pickListJSON struct {
	Orders  []pickListRow     `json:"orders"`
	Totals  []PickListLine    `json:"totals"`
	Skipped []PickListSkipped `json:"skipped"`
}

// validSelection bounds the request before any SQL: exactly one selector, canonical ids, <= 500 ids.
func validSelection(in PickListSelection) error {
	hasOrders := len(in.OrderIDs) > 0
	hasSession := in.SessionID != ""
	if hasOrders == hasSession {
		return command.ErrInvalid
	}
	if hasSession {
		if !command.ValidID(in.SessionID) {
			return command.ErrInvalid
		}
		return nil
	}
	if len(in.OrderIDs) > pickListMaxOrders {
		return ErrPickListTooMany
	}
	for _, id := range in.OrderIDs {
		if !command.ValidID(id) {
			return command.ErrInvalid
		}
	}
	return nil
}

// PickList returns the pick list for the selection (orders:read).
func PickList(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, in PickListSelection) (PickListResult, error) {
	if tx == nil || !validAuthorityInput(scope, token) {
		return PickListResult{}, command.ErrInvalid
	}
	if err := validSelection(in); err != nil {
		return PickListResult{}, err
	}
	raw, err := readPickListJSON(ctx, tx, scope, token, in, false)
	if err != nil {
		return PickListResult{}, err
	}
	out := PickListResult{GeneratedAt: time.Now().UTC().Format(timestampLayout),
		Orders: make([]PickListOrder, 0, len(raw.Orders)),
		Totals: raw.Totals, Skipped: raw.Skipped}
	if out.Totals == nil {
		out.Totals = []PickListLine{}
	}
	if out.Skipped == nil {
		out.Skipped = []PickListSkipped{}
	}
	for _, row := range raw.Orders {
		out.Orders = append(out.Orders, PickListOrder{OrderID: row.OrderID, OrderNumber: row.OrderNumber, ParcelGroupID: row.ParcelGroupID, Lines: row.Lines})
	}
	return out, nil
}

// readPickListJSON runs the shared reader, re-checks orders:read in Go, and decodes the strict JSON.
// export selects the carrier-export eligibility (manual_shipment_eligible + non-CVS destination) instead
// of the pick-list eligibility (order_money_shippable); it is the reader's fifth argument.
func readPickListJSON(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, in PickListSelection, export bool) (pickListJSON, error) {
	hash := sha256.Sum256([]byte(token))
	var orders, session any
	if len(in.OrderIDs) > 0 {
		orders = in.OrderIDs
	} else {
		session = in.SessionID
	}
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT fulfillment.read_pick_list($1,$2::uuid,$3::uuid[],$4::uuid,$5)`,
		hash[:], scope.StoreID, orders, session, export).Scan(&raw); err != nil {
		return pickListJSON{}, mapPickListError(err)
	}
	if err := platform.RequirePermission(ctx, tx, scope, token, "orders:read"); err != nil {
		return pickListJSON{}, mapPickListError(err)
	}
	if len(raw) == 0 || len(raw) > 8<<20 {
		return pickListJSON{}, ErrUnavailable
	}
	var out pickListJSON
	if err := json.Unmarshal(raw, &out); err != nil || !validPickListJSON(out) {
		return pickListJSON{}, ErrUnavailable
	}
	// W3-07B: members of one parcel group are listed side by side and carry parcel_group_id (fulfillment.read_parcel_group_ids).
	ids := make([]string, len(out.Orders))
	for i, row := range out.Orders {
		ids[i] = row.OrderID
	}
	groups, err := parcelGroupIDs(ctx, tx, scope, token, ids)
	if err != nil {
		return pickListJSON{}, err
	}
	out.Orders = groupAdjacent(out.Orders, groups)
	return out, nil
}

func validPickListJSON(v pickListJSON) bool {
	if len(v.Orders) > pickListMaxOrders || len(v.Totals) > pickListMaxOrders*50 || len(v.Skipped) > pickListMaxOrders {
		return false
	}
	for _, row := range v.Orders {
		if !command.ValidID(row.OrderID) || !orderNumberRE.MatchString(row.OrderNumber) ||
			!country(row.Country) || !serviceCode.MatchString(row.ServiceCode) || !currency(row.Currency) ||
			!money(row.TotalMinor) || !money(row.CollectMinor) || !validPaymentMode(row.PaymentMode) ||
			(row.PickupSource != "" && row.PickupSource != "ecpay_directory" && row.PickupSource != "buyer_entered" &&
				row.PickupSource != "merchant_attested") || !validPickListLines(row.Lines) {
			return false
		}
	}
	for _, line := range v.Totals {
		if !validPickListLine(line) {
			return false
		}
	}
	for _, skip := range v.Skipped {
		if !command.ValidID(skip.OrderID) || (skip.Code != "not_pickable" && skip.Code != "order_not_found") {
			return false
		}
	}
	return true
}

func validPickListLines(lines []PickListLine) bool {
	if len(lines) == 0 || len(lines) > 50 {
		return false
	}
	for _, line := range lines {
		if !validPickListLine(line) {
			return false
		}
	}
	return true
}

func validPickListLine(line PickListLine) bool {
	return command.ValidID(line.SKUID) && skuCode.MatchString(line.SKUCode) && textValue(line.Title, 120, true) &&
		line.OptionLabel == nil && line.Qty >= 1 && line.Qty <= command.MaxQuantity
}

// mapPickListError turns the reader's SQLSTATEs into stable classes; PT422 too_many is the only coded refusal.
func mapPickListError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "PT400":
			return command.ErrInvalid
		case "PT401":
			return platform.ErrUnauthorized
		case "PT403":
			return platform.ErrForbidden
		case "PT404":
			return platform.ErrScopeNotFound
		case "PT422":
			if pg.Message == "too_many" {
				return ErrPickListTooMany
			}
			return command.ErrInvalid
		case "PT503":
			return ErrUnavailable
		}
	}
	if errors.Is(err, platform.ErrUnauthorized) || errors.Is(err, platform.ErrForbidden) || errors.Is(err, platform.ErrScopeNotFound) {
		return err
	}
	return ErrUnavailable
}
