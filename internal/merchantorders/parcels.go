// Purpose: W3-07B parcel groups (contracts/manual-fulfilment-v1.md Amendment W3-07B): several orders of ONE buyer going to ONE
//   home address ship as ONE parcel. This file wraps the SQL definers that suggest, create and dissolve a group, ships a group
//   (one RecordShipment per member, same tracking number, one transaction) and exposes the guard that keeps a grouped order out
//   of the single/bulk shipment commands. A group merges PARCELS only: it never reads or writes payments, totals or refunds (I05).
// Depends on: fulfillment.read_merge_suggestions / create_parcel_group / dissolve_parcel_group / begin_parcel_group_shipment /
//   mark_parcel_group_shipped / guard_parcel_group_orders / read_parcel_group_ids (migration 0146), fulfillment.read_open_parcel_groups
//   and the masked read_merge_suggestions replacement (migration 0166, W3-U4), normalizeRecipientMask (list_v2.go), RecordShipment (shipments.go,
//   fulfillment.record_manual_shipment 0107), platform.RequirePermission (second authority fence), command.
// Used by: internal/httpapi/parcels.go (routes), internal/httpapi/shipments.go (single PUT guard), internal/merchanttools/
//   tracking_import.go (bulk guard), picklist.go / carrier_export.go (group annotation + adjacency).
// Invariants: group = 2..20 orders, one owner, one destination hash (SQL decides); COD / pay-at-pickup / CVS never mergeable;
//   a group ships atomically (any member failing rolls the whole transaction back); the group shipment only RECORDS (expected
//   version 0, status SHIPPED), correction/void stay per order; refusal codes are the closed set in parcelCodes.
// Status: REAL_PG (no external system).

package merchantorders

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// ParcelError is a coded 409 refusal of the parcel-group commands (cod_not_mergeable, cvs_not_mergeable, not_mergeable,
// already_in_group, owner_mismatch, destination_mismatch, group_not_open, group_incomplete, in_parcel_group).
type ParcelError struct{ Code string }

func (e *ParcelError) Error() string { return "parcel group: " + e.Code }

// ErrInParcelGroup refuses a single/bulk shipment of an order that sits in an OPEN group (409 in_parcel_group).
var ErrInParcelGroup = &ParcelError{Code: "in_parcel_group"}

// parcelCodes is the closed set of PT409 messages the SQL definers raise; anything else is ErrUnavailable.
var parcelCodes = map[string]bool{"cod_not_mergeable": true, "cvs_not_mergeable": true, "not_mergeable": true, "already_in_group": true,
	"owner_mismatch": true, "destination_mismatch": true, "group_not_open": true, "group_incomplete": true}

// ParcelGroup is the group projection: id, state OPEN|SHIPPED|DISSOLVED, CAS version and (create/ship) the member order ids.
type ParcelGroup struct {
	ID       string   `json:"id"`
	State    string   `json:"state"`
	Version  int64    `json:"version"`
	OrderIDs []string `json:"order_ids,omitempty"`
}

// MergeSuggestion is one "same buyer, same address" candidate set (2..20 orders) for the merchant to confirm. The recipient is
// MASKED like the orders list (0166): the orders page fetches suggestions on every load, so this list-level read never carries the
// full name.
type MergeSuggestion struct {
	RecipientMasked string   `json:"recipient_masked"`
	OrderIDs        []string `json:"order_ids"`
}

// OpenParcelMember is one member order of an OPEN group, display fields only: the recipient is masked like the orders list.
type OpenParcelMember struct {
	OrderID         string `json:"order_id"`
	OrderNumber     string `json:"order_number"`
	RecipientMasked string `json:"recipient_masked"`
}

// OpenParcelGroup is one OPEN group with its CAS version, for the ship/dissolve panels after a reload.
type OpenParcelGroup struct {
	GroupID   string             `json:"group_id"`
	Version   int64              `json:"version"`
	CreatedAt time.Time          `json:"created_at"`
	Members   []OpenParcelMember `json:"members"`
}

// ParcelShipment is the per-member result of a group shipment.
type ParcelShipment struct {
	OrderID  string          `json:"order_id"`
	Shipment ShipmentVersion `json:"shipment"`
}

// ParcelGroupShipment is the PUT shipment response: the SHIPPED group and one shipment version per member (id order).
type ParcelGroupShipment struct {
	ID        string           `json:"id"`
	State     string           `json:"state"`
	Version   int64            `json:"version"`
	Shipments []ParcelShipment `json:"shipments"`
}

// mapParcelError maps the fixed PT messages of the 0146 definers; authority codes delegate to mapError.
func mapParcelError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "PT409":
			switch {
			case pg.Message == "version_changed":
				return ErrVersionChanged
			case parcelCodes[pg.Message]:
				return &ParcelError{Code: pg.Message}
			}
			return command.ErrConflict // idempotency_conflict
		case "PT422":
			if pg.Message == "not_shippable" {
				return ErrNotShippable
			}
			return command.ErrInvalid
		case "23505":
			return command.ErrConflict // the definers catch their own unique violations; anything left is a generic conflict
		}
	}
	return mapError(err)
}

// parcelCall runs one 0146 definer returning jsonb into out, then re-checks the permission in Go (second fence, like
// RecordShipment). args are bound after the leading token hash and store id.
func parcelCall(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, permission, fn string, out any, args ...any) error {
	auth := sha256.Sum256([]byte(token))
	var raw []byte
	call := append([]any{auth[:], scope.StoreID}, args...)
	// Calls fulfillment.<fn> (migration 0146, manual-fulfilment-v1 Amendment W3-07B); placeholders are bound, never concatenated.
	if err := tx.QueryRow(ctx, parcelSQL[fn], call...).Scan(&raw); err != nil {
		return mapParcelError(err)
	}
	if err := platform.RequirePermission(ctx, tx, scope, token, permission); err != nil {
		return mapError(err)
	}
	if len(raw) == 0 || len(raw) > 1<<20 || json.Unmarshal(raw, out) != nil {
		return ErrUnavailable
	}
	return nil
}

// parcelSQL holds one fixed statement per definer (no dynamic SQL).
var parcelSQL = map[string]string{
	"suggest":  `SELECT fulfillment.read_merge_suggestions($1,$2::uuid)`,
	"create":   `SELECT fulfillment.create_parcel_group($1,$2::uuid,$3,$4,$5::uuid[])`,
	"dissolve": `SELECT fulfillment.dissolve_parcel_group($1,$2::uuid,$3::uuid,$4)`,
	"begin":    `SELECT fulfillment.begin_parcel_group_shipment($1,$2::uuid,$3::uuid)`,
	"mark":     `SELECT fulfillment.mark_parcel_group_shipped($1,$2::uuid,$3::uuid)`,
	"guard":    `SELECT fulfillment.guard_parcel_group_orders($1,$2::uuid,$3::uuid[])`,
	"ids":      `SELECT fulfillment.read_parcel_group_ids($1,$2::uuid,$3::uuid[])`,
	"open":     `SELECT fulfillment.read_open_parcel_groups($1,$2::uuid)`,
}

// MergeSuggestions lists the mergeable same-buyer same-address order sets (orders:read). Read only; the recipient is masked in SQL
// (fulfillment.read_merge_suggestions, 0166) and re-validated here with normalizeRecipientMask, so a bad mask degrades to "—".
func MergeSuggestions(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string) ([]MergeSuggestion, error) {
	if tx == nil || !validAuthorityInput(scope, token) {
		return nil, command.ErrInvalid
	}
	var out []MergeSuggestion
	if err := parcelCall(ctx, tx, scope, token, "orders:read", "suggest", &out); err != nil {
		return nil, err
	}
	for i := range out {
		s := &out[i]
		if len(s.OrderIDs) < 2 || len(s.OrderIDs) > 20 || !validIDs(s.OrderIDs) {
			return nil, ErrUnavailable
		}
		s.RecipientMasked = normalizeRecipientMask(s.RecipientMasked)
	}
	if out == nil {
		out = []MergeSuggestion{}
	}
	return out, nil
}

// OpenParcelGroups lists the store's OPEN parcel groups, newest first, at most 200 (orders:read). Read only: SHIPPED and DISSOLVED
// groups never appear, members carry masked recipients only. Calls fulfillment.read_open_parcel_groups (0166, W3-U4 amendment).
func OpenParcelGroups(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string) ([]OpenParcelGroup, error) {
	if tx == nil || !validAuthorityInput(scope, token) {
		return nil, command.ErrInvalid
	}
	var out []OpenParcelGroup
	if err := parcelCall(ctx, tx, scope, token, "orders:read", "open", &out); err != nil {
		return nil, err
	}
	if len(out) > 200 {
		return nil, ErrUnavailable
	}
	seen := map[string]bool{}
	for i := range out {
		g := &out[i]
		if !command.ValidID(g.GroupID) || g.Version < 1 || len(g.Members) < 2 || len(g.Members) > 20 || seen[g.GroupID] {
			return nil, ErrUnavailable
		}
		seen[g.GroupID] = true
		for j := range g.Members {
			m := &g.Members[j]
			if !command.ValidID(m.OrderID) || m.OrderNumber != "LC-"+strings.ToUpper(strings.ReplaceAll(m.OrderID, "-", "")) {
				return nil, ErrUnavailable
			}
			m.RecipientMasked = normalizeRecipientMask(m.RecipientMasked)
		}
	}
	if out == nil {
		out = []OpenParcelGroup{}
	}
	return out, nil
}

// CreateParcelGroup groups 2..20 orders (fulfillment:write, Idempotency-Key). SQL re-derives owner, destination and
// eligibility; this only bounds the request. Writes parcel_groups/_orders, one audit row and the receipt; never a payment row.
func CreateParcelGroup(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key string, orderIDs []string) (ParcelGroup, error) {
	if tx == nil || !validAuthorityInput(scope, token) || !shipmentKey.MatchString(key) || len(orderIDs) < 2 || len(orderIDs) > 20 || !validIDs(orderIDs) {
		return ParcelGroup{}, command.ErrInvalid
	}
	ids := append([]string(nil), orderIDs...)
	sort.Strings(ids)
	digest := sha256.Sum256([]byte("fulfillment.parcel_group.create.v1\n" + joinIDs(ids)))
	var out ParcelGroup
	if err := parcelCall(ctx, tx, scope, token, "fulfillment:write", "create", &out, key, digest[:], ids); err != nil {
		return ParcelGroup{}, err
	}
	if !command.ValidID(out.ID) || out.State != "OPEN" || out.Version != 1 || len(out.OrderIDs) != len(ids) {
		return ParcelGroup{}, ErrUnavailable
	}
	return out, nil
}

// DissolveParcelGroup dissolves an OPEN group at expectedVersion (fulfillment:write); its orders may regroup or ship alone.
func DissolveParcelGroup(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, groupID string, expectedVersion int64) (ParcelGroup, error) {
	if tx == nil || !validAuthorityInput(scope, token) || !command.ValidID(groupID) || expectedVersion < 1 {
		return ParcelGroup{}, command.ErrInvalid
	}
	var out ParcelGroup
	if err := parcelCall(ctx, tx, scope, token, "fulfillment:write", "dissolve", &out, groupID, expectedVersion); err != nil {
		return ParcelGroup{}, err
	}
	if out.State != "DISSOLVED" || out.ID != groupID {
		return ParcelGroup{}, ErrUnavailable
	}
	return out, nil
}

// ShipParcelGroup records ONE shipment (same carrier + tracking number) on every member in id order, inside the caller's
// transaction: any member refusing (version drift, not_shippable, ...) returns the error and the caller's rollback leaves 0
// orders shipped (PG05). Each member goes through RecordShipment, so it gets its own audit row, its own buyer notification and its
// own head version; payments are untouched (I05). The per-order idempotency key derives from the request key, so replaying the
// same request on a SHIPPED group replays each member and changes nothing. Only a first record is supported (expected_version 0,
// status SHIPPED): correction and void stay per order.
func ShipParcelGroup(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, groupID string, in ShipmentInput) (ParcelGroupShipment, error) {
	if tx == nil || !validAuthorityInput(scope, token) || !shipmentKey.MatchString(key) || !command.ValidID(groupID) ||
		in.ExpectedVersion != 0 || in.Status != "SHIPPED" || in.VoidReason != nil {
		return ParcelGroupShipment{}, command.ErrInvalid
	}
	var begin ParcelGroup
	if err := parcelCall(ctx, tx, scope, token, "fulfillment:write", "begin", &begin, groupID); err != nil {
		return ParcelGroupShipment{}, err
	}
	if len(begin.OrderIDs) < 2 || len(begin.OrderIDs) > 20 || !validIDs(begin.OrderIDs) {
		return ParcelGroupShipment{}, ErrUnavailable
	}
	out := ParcelGroupShipment{Shipments: make([]ParcelShipment, 0, len(begin.OrderIDs))}
	for _, orderID := range begin.OrderIDs { // id order (SQL sorted): the same lock order as create and the single-shipment guard
		sum := sha256.Sum256([]byte(key + "\n" + orderID))
		// Calls fulfillment.record_manual_shipment through RecordShipment (manual-fulfilment-v1 §5.1); key = one per member.
		v, err := RecordShipment(ctx, tx, scope, token, "pg-"+hex.EncodeToString(sum[:20]), orderID, in)
		if err != nil {
			return ParcelGroupShipment{}, err
		}
		out.Shipments = append(out.Shipments, ParcelShipment{OrderID: orderID, Shipment: v})
	}
	var done ParcelGroup
	if err := parcelCall(ctx, tx, scope, token, "fulfillment:write", "mark", &done, groupID); err != nil {
		return ParcelGroupShipment{}, err
	}
	if done.ID != groupID || done.State != "SHIPPED" {
		return ParcelGroupShipment{}, ErrUnavailable
	}
	out.ID, out.State, out.Version = done.ID, done.State, done.Version
	return out, nil
}

// GuardNotGrouped locks the named orders (id order) and returns order_id -> group_id for those in an OPEN group
// (fulfillment:write). The single PUT refuses them with ErrInParcelGroup and the bulk import fails their rows with the same
// code, so a grouped order is never shipped alone; locking before the shipment closes the window in which a group could be
// created between this check and the shipment write. Writes nothing.
func GuardNotGrouped(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, orderIDs []string) (map[string]string, error) {
	if tx == nil || !validAuthorityInput(scope, token) || len(orderIDs) > 500 || !validIDs(orderIDs) {
		return nil, command.ErrInvalid
	}
	return parcelMembership(ctx, tx, scope, token, "fulfillment:write", "guard", orderIDs)
}

// parcelGroupIDs returns order_id -> group_id for every group member among orderIDs (orders:read, no lock): the pick list and
// carrier export annotate and group their rows with it.
func parcelGroupIDs(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, orderIDs []string) (map[string]string, error) {
	if len(orderIDs) == 0 {
		return map[string]string{}, nil
	}
	return parcelMembership(ctx, tx, scope, token, "orders:read", "ids", orderIDs)
}

func parcelMembership(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, permission, fn string, orderIDs []string) (map[string]string, error) {
	var rows []struct {
		OrderID string `json:"order_id"`
		GroupID string `json:"group_id"`
	}
	if len(orderIDs) == 0 {
		return map[string]string{}, nil
	}
	if err := parcelCall(ctx, tx, scope, token, permission, fn, &rows, orderIDs); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		if !command.ValidID(r.OrderID) || !command.ValidID(r.GroupID) {
			return nil, ErrUnavailable
		}
		out[r.OrderID] = r.GroupID
	}
	return out, nil
}

// groupAdjacent annotates each row with its parcel group and reorders the rows so the members of one group are adjacent: a
// group sits where its first member was, members keep their relative order. Rows without a group keep their place.
func groupAdjacent(rows []pickListRow, groups map[string]string) []pickListRow {
	if len(groups) == 0 {
		return rows
	}
	out := make([]pickListRow, 0, len(rows))
	emitted := map[string]bool{}
	for i := range rows {
		g := groups[rows[i].OrderID]
		if g == "" {
			out = append(out, rows[i])
			continue
		}
		if emitted[g] {
			continue
		}
		emitted[g] = true
		for j := i; j < len(rows); j++ {
			if groups[rows[j].OrderID] == g {
				rows[j].ParcelGroupID = g
				out = append(out, rows[j])
			}
		}
	}
	return out
}

func validIDs(ids []string) bool {
	for _, id := range ids {
		if !command.ValidID(id) {
			return false
		}
	}
	return true
}

func joinIDs(ids []string) string {
	b := make([]byte, 0, len(ids)*37)
	for _, id := range ids {
		b = append(append(b, id...), '\n')
	}
	return string(b)
}
