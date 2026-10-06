// Purpose: Go side of the merchant-attested live price of an order made FOR a buyer (live-console-v1 §5.3, OPEN-13): the seven-line view of the
// claims definers that reserve the bundles, evaluate the fail-closed buyer comparison, write the single-use grants, bind a grant to its quote and
// report the live-priced lines of a placed order. It decides nothing: every rule (peer comparison, permission, 15-minute expiry, single use,
// quantity ceiling) lives in SQL (migration 0129 + claims.live_prices / consume_live_prices); a Go caller can only ask.
// Depends on: SQL claims.for_buyer_lines / for_buyer_begin / for_buyer_finish / for_buyer_release (merchant transaction on commerce_runtime) and
//   claims.bind_merchant_origin_grant (buyer transaction on commerce_buyer_runtime), internal/command, internal/platform, pgx.
// Used by: internal/merchanttools/order_for_buyer.go (A15 prefill, A16 for-buyer order).
// Invariants: I05/I08 (no price crosses this file: a live price still comes only from claims.live_prices into the Quote), live-console-v1 §5.1
//   one live order per bundle, §5.3 fail-closed eligibility. Status: MOCK (REAL_PG gates LCN12/LPC01-06).

package claims

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ForBuyerError is a refusal of the for-buyer definers with its transport status and contract code (409 bundle_already_ordered, 409
// bundle_buyer_mismatch). OrderID is set only for bundle_already_ordered when an order already holds the bundle.
type ForBuyerError struct {
	Status  int
	Code    string
	OrderID string
}

func (e *ForBuyerError) Error() string { return "claims: " + e.Code }

// ErrorDetails adds the existing order to the error body (httpapi respondErrorDetails): the UI offers 「查看訂單」 instead of a second order.
func (e *ForBuyerError) ErrorDetails() map[string]any {
	if e.OrderID == "" {
		return map[string]any{"order_id": nil}
	}
	return map[string]any{"order_id": e.OrderID}
}

// forBuyerError maps a definer refusal: PT409 messages bundle_already_ordered / bundle_buyer_mismatch become a *ForBuyerError, the rest follow
// mapError (authority, not found, invalid, conflict).
func forBuyerError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "PT409" {
		switch pg.Message {
		case "bundle_already_ordered":
			// 409 = HTTP Conflict as a literal: importing net/http here would trip MCI10 (claims does no network I/O).
			return &ForBuyerError{Status: 409, Code: pg.Message, OrderID: pg.Detail}
		case "bundle_buyer_mismatch", "idempotency_conflict": // the same key with another request body
			return &ForBuyerError{Status: 409, Code: pg.Message}
		}
	}
	return mapError(err)
}

// ForBuyerLine is one claim line of A15: the live price is nil when the offer is inactive or has none; LiveRemaining is the claimed quantity
// minus the units held by non-CANCELLED orders (never negative).
type ForBuyerLine struct {
	BundleID, OfferID, SKUID, Keyword string
	Quantity                          int64
	LivePriceMinor                    *int64
	LiveRemaining                     int64
}

// ForBuyerLines reads the claim lines of one bundle or of the bundles linked to one conversation (exactly one of the two is non-nil), capped at
// 50. Calls claims.for_buyer_lines (0129) in the merchant transaction; orders:read + inventory:reserve are re-verified in SQL. No write.
func ForBuyerLines(ctx context.Context, tx pgx.Tx, conversation, bundle *string) ([]ForBuyerLine, error) {
	rows, err := tx.Query(ctx, `SELECT bundle_id::text,offer_id::text,sku_id::text,keyword,quantity,live_price_minor,live_remaining
		FROM claims.for_buyer_lines($1::uuid,$2::uuid)`, conversation, bundle)
	if err != nil {
		return nil, forBuyerError(err)
	}
	defer rows.Close()
	out := []ForBuyerLine{}
	for rows.Next() {
		var l ForBuyerLine
		if err = rows.Scan(&l.BundleID, &l.OfferID, &l.SKUID, &l.Keyword, &l.Quantity, &l.LivePriceMinor, &l.LiveRemaining); err != nil {
			return nil, forBuyerError(err)
		}
		out = append(out, l)
	}
	if err = rows.Err(); err != nil {
		return nil, forBuyerError(err)
	}
	return out, nil
}

// ForBuyerBegun is the answer of BeginForBuyer: the request id (stable under one Idempotency-Key) and Reason, the empty string when a grant was
// written per bundle, else why the order is priced at the catalog: no_bundle | no_conversation | bundle_buyer_unverified | permission.
type ForBuyerBegun struct {
	RequestID string
	Reason    string
}

// BeginForBuyer is §5.1 steps 3a+3b: it reserves the bundles (409 bundle_already_ordered) and evaluates the fail-closed buyer comparison (409
// bundle_buyer_mismatch, nothing created; 409 idempotency_conflict when keyHash already started another requestHash) and writes the single-use grants for buyer (the capability owner of the order). Calls
// claims.for_buyer_begin (0129) in the merchant transaction; a resume under the same key hash finds its own rows. bundles must be sorted.
func BeginForBuyer(ctx context.Context, tx pgx.Tx, keyHash, requestHash []byte, conversation *string, bundles []string, buyer string) (ForBuyerBegun, error) {
	var out ForBuyerBegun
	var buyerArg *string
	if buyer != "" {
		buyerArg = &buyer
	}
	if bundles == nil {
		bundles = []string{}
	}
	err := tx.QueryRow(ctx, `SELECT o_request::text,o_reason FROM claims.for_buyer_begin($1::bytea,$2::bytea,$3::uuid,$4::uuid[],$5::uuid)`,
		keyHash, requestHash, conversation, bundles, buyerArg).Scan(&out.RequestID, &out.Reason)
	if err != nil {
		return ForBuyerBegun{}, forBuyerError(err)
	}
	return out, nil
}

// GrantedLine is one live-priced line of a placed order as the ledger recorded it (audit input); LivePriceMinor is the unit price the line was
// CONSUMED at (claims.live_price_uses.unit_price_minor, from the order's own quote), not the offer's current price.
type GrantedLine struct {
	BundleID, OfferID, SKUID string
	Quantity, LivePriceMinor int64
}

// FinishForBuyer is §5.1 step 3d (the reservation part): the request's rows become placed with the order id (409 conflict when a row of the request was lost), its unconsumed grants expire; it returns the order's live-priced
// lines (empty = catalog price). Calls claims.for_buyer_finish (0129) in the merchant transaction.
func FinishForBuyer(ctx context.Context, tx pgx.Tx, request, order string) ([]GrantedLine, error) {
	rows, err := tx.Query(ctx, `SELECT bundle_id::text,offer_id::text,sku_id::text,quantity,live_price_minor FROM claims.for_buyer_finish($1::uuid,$2::uuid)`, request, order)
	if err != nil {
		return nil, forBuyerError(err)
	}
	defer rows.Close()
	out := []GrantedLine{}
	for rows.Next() {
		var g GrantedLine
		var price *int64
		if err = rows.Scan(&g.BundleID, &g.OfferID, &g.SKUID, &g.Quantity, &price); err != nil {
			return nil, forBuyerError(err)
		}
		if price != nil {
			g.LivePriceMinor = *price
		}
		out = append(out, g)
	}
	if err = rows.Err(); err != nil {
		return nil, forBuyerError(err)
	}
	return out, nil
}

// ReleaseForBuyer gives the bundles of a refused placement back and expires its unconsumed grants. Calls claims.for_buyer_release (0129) in the
// merchant transaction. A placed row is never touched.
func ReleaseForBuyer(ctx context.Context, tx pgx.Tx, request string) error {
	var n int
	if err := tx.QueryRow(ctx, `SELECT claims.for_buyer_release($1::uuid)`, request).Scan(&n); err != nil {
		return forBuyerError(err)
	}
	return nil
}

// BindMerchantOriginGrant binds the buyer's unconsumed grants of every live-priced bundle of quote to that quote and returns how many are bound.
// Run it in a buyer transaction (buyer.WithScope) right after CreateQuote: CreateQuote cannot name the quote it is about to insert. Calls
// claims.bind_merchant_origin_grant (0129, EXECUTE commerce_buyer_runtime).
func BindMerchantOriginGrant(ctx context.Context, tx pgx.Tx, quote string) (int, error) {
	var n int
	if err := tx.QueryRow(ctx, `SELECT claims.bind_merchant_origin_grant($1::uuid)`, quote).Scan(&n); err != nil {
		return 0, forBuyerError(err)
	}
	return n, nil
}
