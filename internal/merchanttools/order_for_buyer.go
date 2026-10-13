// Purpose: A15/A16 of live-console-v1 §5 (LC-B6, 幫他建立訂單): the merchant creates the order FOR a buyer who will not click the pushed link,
// then DMs the pay link. A thin wrapper over ManualOrders.Place and its durable server intent: bundle reservation rows (one live order per bundle), the
// fail-closed merchant-attested live price (a single-use, quote-bound grant; the Quote stays the only price), an audit trail and an optional
// order-pay-link/v1 DM. Resumable under the Idempotency-Key: a crash between Place, the reservation record and the DM is completed by a replay
// with one order and one DM operation. GET order-prefill reads the claim lines and the explicitly linked customer; it never uses owner_id.
// Depends on: ManualOrders (manual.go: Place, RegenerateLink, Options, hooks), internal/claims (live_price_grant.go: claims.for_buyer_* and
//   claims.bind_merchant_origin_grant), internal/inbox (PlanOrderPayLink via PayLinkPlanner), internal/customers + internal/merchantorders
//   (the linked customer's newest order, read-only), internal/command (receipts, audit), internal/platform, internal/storefront (ClaimOrigin),
//   ops.order_intents (original owner under RLS), fulfillment.order_intent_begin/order_intent_has_catalog_lines/order_prefill_email/order_pay_link_valid and claims.close_cancelled_for_buyer (0172),
//   catalog.skus/products and control.stores (read-only, commerce_runtime under RLS).
// Used by: internal/httpapi/merchanttools.go (routes A15/A16); cmd/api builds it beside the ManualOrders pipeline and the inbox service.
// Invariants: I05/I08 (no client price: the body has no price key; the live price is decided by claims.live_prices), I03 (stock only through
//   begin_hold in Place), live-console-v1 §5.1 (one live order per bundle, resumable) and §5.3 (fail-closed eligibility, single-use grant),
//   I09 (claims.bundles.owner_id is never read), A1.3 (the DM planner locks on the conversation). Status: MOCK (REAL_PG gate LCN12).

package merchanttools

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/buyer"
	"livecommerce/internal/claims"
	"livecommerce/internal/command"
	"livecommerce/internal/customers"
	"livecommerce/internal/inbox"
	"livecommerce/internal/merchantorders"
	"livecommerce/internal/platform"
	"livecommerce/internal/storefront"
)

const (
	forBuyerOperation = "merchanttools.order.for_buyer"
	maxForBuyerBundle = 5
	// maxGrantAuditBytes keeps one claims.merchant_origin_granted detail under command.AuditDetails' 1024-byte cap.
	maxGrantAuditBytes = 900
	// maxGrantAuditLines is how many lines (full offer id, SKU id, quantity, unit price actually charged) one audit event lists; line_count is the total.
	maxGrantAuditLines = 5
)

// ForBuyerTarget is the body's `for` object: the claim bundles the order is made for (0..5; empty = a plain manual order, never a live price)
// and the conversation the buyer wrote in (null = no thread).
type ForBuyerTarget struct {
	BundleIDs      []string `json:"bundle_ids"`
	ConversationID *string  `json:"conversation_id"`
}

// ForBuyerInput is the exact POST orders/for-buyer body. There is no price, discount or tenant field.
type ForBuyerInput struct {
	Items           []ManualItem   `json:"items"`
	Customer        ManualCustomer `json:"customer"`
	Delivery        ManualDelivery `json:"delivery"`
	PaymentMode     string         `json:"payment_mode"`
	Locale          string         `json:"locale"`
	For             ForBuyerTarget `json:"for"`
	SendPaymentLink bool           `json:"send_payment_link"`
}

// ForBuyerSend is the DM outcome: {operation_id, state:"queued"} when planned, else {state:"not_sent", reason} (the UI offers copy-link).
type ForBuyerSend struct {
	OperationID string `json:"operation_id,omitempty"`
	State       string `json:"state"`
	Reason      string `json:"reason,omitempty"`
}

// forBuyerReceipt is what the for-buyer command receipt keeps: the manual receipt plus the live-price outcome. Never a link or a token.
type forBuyerReceipt struct {
	manualReceipt
	LivePrice       string `json:"live_price"`
	LivePriceReason string `json:"live_price_reason"`
}

// ForBuyerResult is the 201 (or replayed 200) body: the ManualResult of POST orders/manual plus the live-price and DM outcomes.
type ForBuyerResult struct {
	ManualResult
	LivePrice       string       `json:"live_price"`
	LivePriceReason string       `json:"live_price_reason"`
	Send            ForBuyerSend `json:"send"`
}

// PayLinkPlanner plans the pay-link DM (inbox.Service.PlanOrderPayLink). A nil planner (messaging not configured) makes every DM "not_sent".
type PayLinkPlanner interface {
	PlanOrderPayLink(ctx context.Context, tx pgx.Tx, scope platform.Scope, key, conversationID, orderID, link string) (inbox.SendOutput, error)
}

// ForBuyer is the A15/A16 service. FaultAfterPlace is a test seam: a non-nil hook runs right after Place and before the reservation record, and
// an error it returns aborts the request there exactly as a process kill would (the replay gate of LCN12); production leaves it nil.
type ForBuyer struct {
	manual          *ManualOrders
	planner         PayLinkPlanner
	FaultAfterPlace func() error
}

// NewForBuyer wires the service on the manual-order pipeline. planner may be nil.
func NewForBuyer(manual *ManualOrders, planner PayLinkPlanner) (*ForBuyer, error) {
	if manual == nil {
		return nil, command.ErrInvalid
	}
	return &ForBuyer{manual: manual, planner: planner}, nil
}

// validateTarget normalizes the `for` object: ids valid and distinct, at most five, sorted (the lock order of the reservation rows).
func validateTarget(t ForBuyerTarget) (ForBuyerTarget, error) {
	t = normalizeTarget(t)
	bad := &Error{Status: http.StatusUnprocessableEntity, Code: "invalid_request"}
	if len(t.BundleIDs) > maxForBuyerBundle || (t.ConversationID != nil && !command.ValidID(*t.ConversationID)) {
		return t, bad
	}
	for i, id := range t.BundleIDs {
		if !command.ValidID(id) || (i > 0 && t.BundleIDs[i-1] == id) {
			return t, bad
		}
	}
	return t, nil
}

func normalizeTarget(t ForBuyerTarget) ForBuyerTarget {
	ids := slices.Clone(t.BundleIDs)
	if ids == nil {
		ids = []string{}
	}
	sort.Strings(ids)
	t.BundleIDs = ids
	return t
}

// forBuyerRequest is the single normalized full A16 request used by placement, binding and recovery receipts.
type forBuyerRequest struct {
	Manual ManualInput    `json:"manual"`
	For    ForBuyerTarget `json:"for"`
	Send   bool           `json:"send_payment_link"`
}

func canonicalForBuyer(in ForBuyerInput) forBuyerRequest {
	manual := normalizeManual(ManualInput{Items: in.Items, Customer: in.Customer, Delivery: in.Delivery, PaymentMode: in.PaymentMode, Locale: in.Locale})
	return forBuyerRequest{Manual: manual, For: normalizeTarget(in.For), Send: in.SendPaymentLink}
}

// pickOrigins derives a cart-line origin only when exactly one claim line matches its SKU and covers its quantity.
// Multiple claim origins cannot be represented by the cart's one-origin-per-SKU shape, so that aggregate stays catalog priced.
// A line that does not qualify
// stays at the catalog price (fail closed; carryOrigins would drop it anyway). skipped counts requested SKUs that had a claim line but none
// qualified. Pure: the Quote, not this, decides the price.
func pickOrigins(items []ManualItem, lines []claims.ForBuyerLine) (origins map[string]storefront.ClaimOrigin, skipped int) {
	origins = map[string]storefront.ClaimOrigin{}
	for _, it := range items {
		matches := 0
		var candidate claims.ForBuyerLine
		for _, l := range lines {
			if l.SKUID != it.SKUID {
				continue
			}
			matches++
			candidate = l
			if matches > 1 {
				break
			}
		}
		if matches == 1 && candidate.LivePriceMinor != nil && candidate.Quantity >= it.Quantity && candidate.LiveRemaining >= it.Quantity {
			origins[it.SKUID] = storefront.ClaimOrigin{BundleID: candidate.BundleID, OfferID: candidate.OfferID, Quantity: candidate.Quantity}
		} else if matches > 0 {
			skipped++
		}
	}
	return origins, skipped
}

// Place creates (or replays) the order for the buyer and plans the pay-link DM. replayed is true when the receipt of an earlier identical request
// answered. Errors: the orders/manual codes, 409 capability (send_payment_link without inbox:reply), 409 bundle_already_ordered (claims.ForBuyerError
// with the order id), 409 bundle_buyer_mismatch. A DM that cannot be planned never fails the order: send says why (the UI offers copy-link).
func (f *ForBuyer) Place(ctx context.Context, token, storeID, key string, in ForBuyerInput) (ForBuyerResult, bool, error) {
	if f == nil || f.manual == nil {
		return ForBuyerResult{}, false, ErrManualDisabled
	}
	m := f.manual
	request := canonicalForBuyer(in)
	if !attemptHeld(ctx, m, storeID, key) {
		var out ForBuyerResult
		var replayed bool
		err := m.withAttempt(ctx, token, storeID, key, forBuyerOperation, request, false, func(c context.Context) error {
			var inner error
			out, replayed, inner = f.Place(c, token, storeID, key, in)
			return inner
		})
		if err != nil {
			return ForBuyerResult{}, false, mapForBuyerError(err)
		}
		return out, replayed, nil
	}
	manualIn, err := ValidateManual(request.Manual)
	if err != nil {
		return ForBuyerResult{}, false, err
	}
	target, err := validateTarget(request.For)
	if err != nil {
		return ForBuyerResult{}, false, err
	}

	var receipt forBuyerReceipt
	var linkState string
	receiptReplayed := false
	// Probe: authority (inventory:reserve + catalog:read as Place; inbox:reply when a DM is asked, P2-7 b), the storefront origin state and any
	// receipt of this key. A missing inbox:reply is refused here, BEFORE anything is reserved or created.
	err = platform.WithScope(ctx, m.pool, token, storeID, manualPermission, func(tx pgx.Tx, scope platform.Scope) error {
		if inner := requireCatalog(ctx, tx, scope, token); inner != nil {
			return inner
		}
		if in.SendPaymentLink {
			if inner := platform.RequirePermission(ctx, tx, scope, token, "inbox:reply"); inner != nil {
				if errors.Is(inner, platform.ErrForbidden) {
					return &Error{Status: http.StatusConflict, Code: "capability"}
				}
				return inner
			}
		}
		var inner error
		if linkState, _, inner = readOrigin(ctx, tx, token, scope); inner != nil {
			return inner
		}
		inner = command.Run(ctx, tx, scope, forBuyerOperation, key, request, &receipt, func() error { return errNeedWork })
		switch {
		case inner == nil:
			receiptReplayed = true
		case errors.Is(inner, errNeedWork):
			inner = nil
		}
		return inner
	})
	if err != nil {
		return ForBuyerResult{}, false, mapForBuyerError(err)
	}

	out := ForBuyerResult{}
	freshLink := false // Place created here; update its returned BuyerLink after successful atomic DM planning.
	if receiptReplayed {
		receipt.ExpiresAt = receipt.ExpiresAt.UTC()
		out.ManualResult = ManualResult{manualReceipt: receipt.manualReceipt, LinkState: linkState, Source: "merchant_manual"}
	} else {
		var placed ManualResult
		var placedReplayed bool
		if placed, receipt.LivePrice, receipt.LivePriceReason, placedReplayed, err = f.placeOrder(ctx, token, storeID, key, manualIn, target, request); err != nil {
			return ForBuyerResult{}, false, err
		}
		receipt.manualReceipt = placed.manualReceipt
		out.ManualResult = placed
		freshLink = !placedReplayed
	}
	out.LivePrice, out.LivePriceReason = receipt.LivePrice, receipt.LivePriceReason

	out.Send, err = f.sendPayLink(ctx, token, storeID, key, in, target, manualIn.Locale, out.OrderID, out.BuyerLink, freshLink)
	if err != nil {
		return ForBuyerResult{}, false, err
	}
	if out.CommercialState == "CANCELLED" || (out.Send.State == "not_sent" && (out.Send.Reason == "link_unavailable" || out.Send.Reason == "window_closed" || out.Send.Reason == "order_cancelled")) {
		// A competing reminder may have superseded Place's URL before DM planning. Never return that dead token.
		out.BuyerLink = nil
	}
	if out.BuyerLink != nil {
		// The candidate may predate a competing planner. Only a locked exact-token validity proof permits its response.
		link, parseErr := url.Parse(*out.BuyerLink)
		var fragment url.Values
		if parseErr == nil {
			fragment, parseErr = url.ParseQuery(link.Fragment)
		}
		if parseErr != nil || len(fragment) != 2 || fragment.Get("o") != out.OrderID || fragment.Get("t") == "" {
			out.BuyerLink = nil
		} else if valid, proofErr := m.validOrderLink(ctx, token, storeID, out.OrderID, fragment.Get("t")); proofErr != nil || !valid {
			out.BuyerLink = nil
		}
	}
	// The inner A16 receipt predates DM planning. Recovery must not finalize a partially committed pipeline.
	if err = f.complete(ctx, token, storeID, key, request, out.OrderID); err != nil {
		return ForBuyerResult{}, false, mapForBuyerError(err)
	}
	return out, receiptReplayed, nil
}

// placeOrder is steps 3a-3d: reserve + grant (merchant tx), Place with server-only origins/grant binding, then reservation/receipt/audit.
// A known Begin resumes its immutable intent without new pricing/grants; cancellation closes only the original bundle bookkeeping.
// A definitive Place refusal gives bundles back; uncertainty retains the intent even if uncommitted bundle reservations expire.
func (f *ForBuyer) placeOrder(ctx context.Context, token, storeID, key string, manualIn ManualInput, target ForBuyerTarget,
	request any) (ManualResult, string, string, bool, error) {
	m := f.manual
	var zero ManualResult
	original, committed, err := m.committedIntent(ctx, token, storeID, key)
	if err != nil {
		return zero, "", "", false, mapForBuyerError(err)
	}
	cancelled := committed && original.CommercialState == "CANCELLED"
	buyerOwner := ""
	if len(target.BundleIDs) > 0 {
		// The original intent's private owner is immutable across signing-key rotation and buyer capability expiry.
		err := platform.WithScope(ctx, m.pool, token, storeID, manualPermission, func(tx pgx.Tx, scope platform.Scope) error {
			intent, err := readIntent(ctx, tx, scope, key)
			buyerOwner = intent.Owner
			return err
		})
		if err != nil {
			return zero, "", "", false, classifyPipeline(ctx, err)
		}
	}
	keyHash := sha256.Sum256([]byte("order-for-buyer|" + storeID + "|" + key))
	canonical, err := json.Marshal(request)
	if err != nil {
		return zero, "", "", false, &Error{Status: http.StatusUnprocessableEntity, Code: "invalid_request"}
	}
	requestHash := sha256.Sum256(canonical) // the same key with another body (bundle set, items, delivery) is refused in SQL (P2-4)
	var begun claims.ForBuyerBegun
	var lines []claims.ForBuyerLine
	err = platform.WithScope(ctx, m.pool, token, storeID, manualPermission, func(tx pgx.Tx, scope platform.Scope) error {
		var inner error
		if committed {
			if len(target.BundleIDs) == 0 {
				begun.Reason = "no_bundle"
				return nil
			}
			// The cancelled-original closure proves and releases its exact reservations later, even if they already expired/released.
			if cancelled {
				return nil
			}
			// Calls claims.for_buyer_request (0170): an established Begin resumes only original reservations, never renews grants.
			pending, lookupErr := claims.RequestForBuyer(ctx, tx, keyHash[:], requestHash[:], buyerOwner)
			if lookupErr != nil {
				return lookupErr
			}
			if pending == nil {
				return ErrUnavailable
			}
			begun.RequestID = *pending
			if target.ConversationID == nil {
				begun.Reason = "no_conversation"
			}
			return nil
		}
		// Calls claims.for_buyer_begin (0129; live-console-v1 §5.1 3a+3b): reservation rows, fail-closed buyer comparison, grants.
		if begun, inner = claims.BeginForBuyer(ctx, tx, keyHash[:], requestHash[:], target.ConversationID, target.BundleIDs, buyerOwner); inner != nil {
			return inner
		}
		if begun.Reason != "" {
			return nil
		}
		for _, bundle := range target.BundleIDs {
			// Calls claims.for_buyer_lines (0129): the claim lines the origins are derived from (server facts, never the request).
			got, e := claims.ForBuyerLines(ctx, tx, nil, &bundle)
			if e != nil {
				return e
			}
			lines = append(lines, got...)
		}
		return nil
	})
	if err != nil {
		return zero, "", "", false, mapForBuyerError(err)
	}

	hooks := &pipelineHooks{}
	skipped := 0
	if !committed && begun.Reason == "" && len(target.BundleIDs) > 0 {
		hooks.origins, skipped = pickOrigins(manualIn.Items, lines)
		hooks.afterQuote = f.bindGrant
	}
	placed, placedReplayed, err := m.Place(withPipelineHooks(ctx, hooks), token, storeID, key, manualIn)
	if err != nil {
		if definitiveRefusal(err) && len(target.BundleIDs) > 0 {
			f.release(ctx, token, storeID, begun.RequestID)
		}
		return zero, "", "", false, err
	}
	if f.FaultAfterPlace != nil {
		if err = f.FaultAfterPlace(); err != nil {
			return zero, "", "", false, err
		}
	}
	if cancelled {
		if placed.OrderID != original.OrderID {
			return zero, "", "", false, mapForBuyerError(command.ErrConflict)
		}
		// A historical manual receipt may still describe the pre-cancellation snapshot. Never expose its link, even for a queued DM receipt.
		placed.CommercialState = original.CommercialState
		placed.BuyerLink = nil
	}

	var granted []claims.GrantedLine
	var receipt forBuyerReceipt
	err = platform.WithScope(ctx, m.pool, token, storeID, manualPermission, func(tx pgx.Tx, scope platform.Scope) error {
		return command.Run(ctx, tx, scope, forBuyerOperation, key, request, &receipt, func() error {
			if len(target.BundleIDs) > 0 {
				var inner error
				if cancelled {
					granted, inner = closeCancelledForBuyer(ctx, tx, keyHash[:], requestHash[:], buyerOwner, placed.OrderID)
				} else {
					// Calls claims.for_buyer_finish (0129; §5.1 3d): the rows become placed; the ledger rows say whether the live price was applied.
					granted, inner = claims.FinishForBuyer(ctx, tx, begun.RequestID, placed.OrderID)
				}
				if inner != nil {
					return inner
				}
			}
			receipt = forBuyerReceipt{manualReceipt: placed.manualReceipt, LivePrice: "not_applied", LivePriceReason: notAppliedReason(begun.Reason, len(hooks.origins))}
			if len(granted) > 0 {
				receipt.LivePrice, receipt.LivePriceReason = "applied", ""
				catalogLines := skipped > 0
				if committed {
					// Calls fulfillment.order_intent_has_catalog_lines (0172): original private Begin + immutable Quote rules.
					// Current claims may be purged or changed; the consumed ledger and original Quote alone prove mixed pricing.
					hash := sha256.Sum256([]byte(token))
					if err := tx.QueryRow(ctx, `SELECT fulfillment.order_intent_has_catalog_lines($1,$2::uuid,$3)`, hash[:], storeID, key).Scan(&catalogLines); err != nil {
						return err
					}
				}
				if catalogLines {
					receipt.LivePriceReason = "some_lines_catalog"
				}
			}
			if err := command.AuditDetails(ctx, tx, scope, "order.for_buyer_created", map[string]any{
				"order_id": placed.OrderID, "bundles": len(target.BundleIDs), "live_price": receipt.LivePrice}); err != nil {
				return err
			}
			// OPEN-13 condition (2): one audit event per bundle that priced a line at the live price (principal comes from the scope).
			for _, bundle := range target.BundleIDs {
				if detail, ok := grantAudit(bundle, buyerOwner, placed.OrderID, granted); ok {
					if err := command.AuditDetails(ctx, tx, scope, "claims.merchant_origin_granted", detail); err != nil {
						return err
					}
				}
			}
			return nil
		})
	})
	if err != nil {
		return zero, "", "", false, mapForBuyerError(err)
	}
	return placed, receipt.LivePrice, receipt.LivePriceReason, placedReplayed, nil
}

// closeCancelledForBuyer completes only the original cancelled intent's claim bookkeeping; the definer proves its exact owner/Begin.
// It releases original pending reservations, expires unconsumed grants and returns historical consumed prices, never a new price or grant.
func closeCancelledForBuyer(ctx context.Context, tx pgx.Tx, keyHash, requestHash []byte, owner, orderID string) ([]claims.GrantedLine, error) {
	rows, err := tx.Query(ctx, `SELECT bundle_id::text,offer_id::text,sku_id::text,quantity,live_price_minor FROM claims.close_cancelled_for_buyer($1,$2,$3::uuid,$4::uuid)`,
		keyHash, requestHash, owner, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	granted := []claims.GrantedLine{}
	for rows.Next() {
		var line claims.GrantedLine
		var price *int64
		if err := rows.Scan(&line.BundleID, &line.OfferID, &line.SKUID, &line.Quantity, &price); err != nil {
			return nil, err
		}
		if price != nil {
			line.LivePriceMinor = *price
		}
		granted = append(granted, line)
	}
	return granted, rows.Err()
}

// notAppliedReason names why no live price applied when no ledger row exists: the definer's reason when no grant was written, else the grant
// was written but no requested line qualified or the live price vanished before Begin.
func notAppliedReason(reason string, origins int) string {
	switch {
	case reason != "":
		return reason
	case origins == 0:
		return "no_live_line"
	}
	return "live_price_unavailable"
}

// grantAudit builds the claims.merchant_origin_granted detail of one bundle (principal is the audit row's own column): bundle, buyer, order and
// the live-priced lines {offer id, SKU id, quantity, unit price the ledger consumed}. The first maxGrantAuditLines lines are listed to stay under the 1 KiB
// detail cap; line_count is the total and the full facts live in claims.live_price_uses.
func grantAudit(bundle, buyer, order string, granted []claims.GrantedLine) (map[string]any, bool) {
	lines := []map[string]any{}
	total := 0
	for _, g := range granted {
		if g.BundleID != bundle {
			continue
		}
		total++
		if len(lines) < maxGrantAuditLines {
			lines = append(lines, map[string]any{"o": g.OfferID, "k": g.SKUID, "q": g.Quantity, "p": g.LivePriceMinor})
		}
	}
	if total == 0 {
		return nil, false
	}
	return map[string]any{"bundle_id": bundle, "buyer_id": buyer, "order_id": order, "lines": lines, "line_count": total}, true
}

// bindGrant runs right after CreateQuote on the buyer pool: the buyer's grants of the live-priced bundles of the quote bind to that quote
// (claims.bind_merchant_origin_grant, 0129). A quote without a live line binds nothing.
func (f *ForBuyer) bindGrant(ctx context.Context, capability, storeID, quoteID string) error {
	return buyer.WithScope(ctx, f.manual.buyerPool, capability, storeID, func(c context.Context, tx pgx.Tx, _ buyer.Scope) error {
		_, err := claims.BindMerchantOriginGrant(c, tx, quoteID)
		return err
	})
}

// release gives the bundles back after a definitive refusal. Best effort: a failure here only delays the 15-minute expiry of the row.
func (f *ForBuyer) release(ctx context.Context, token, storeID, request string) {
	if request == "" {
		return
	}
	_ = platform.WithScope(ctx, f.manual.pool, token, storeID, manualPermission, func(tx pgx.Tx, scope platform.Scope) error {
		return claims.ReleaseForBuyer(ctx, tx, request)
	})
}

// definitiveRefusal is true when Place refused before it could have created an order: a coded 4xx other than the idempotency conflict (which
// means another body already owns this key and its rows must stay).
func definitiveRefusal(err error) bool {
	var coded *Error
	if errors.As(err, &coded) {
		return coded.Status >= 400 && coded.Status < 500 && coded.Code != "idempotency_conflict"
	}
	return false
}

// mapForBuyerError keeps the for-buyer refusals (409 bundle_*) and maps everything else like the manual order does.
func mapForBuyerError(err error) error {
	var fb *claims.ForBuyerError
	var coded *Error
	switch {
	case errors.As(err, &fb):
		return fb
	case errors.As(err, &coded): // a refusal this file raised itself (409 capability) or a pipeline refusal already coded
		return coded
	case errors.Is(err, command.ErrConflict):
		// Whole-attempt binding now detects this before claims reservation; preserve the A16 domain error contract.
		return &claims.ForBuyerError{Status: http.StatusConflict, Code: "idempotency_conflict"}
	}
	return mapReceiptError(err)
}

// sendPayLink is step 4. It never fails the order for a DM refusal: not_sent says why. An unknown failure is returned (the replay resumes it).
func (f *ForBuyer) sendPayLink(ctx context.Context, token, storeID, key string, in ForBuyerInput, target ForBuyerTarget, locale, orderID string,
	buyerLink *string, freshLink bool) (ForBuyerSend, error) {
	m := f.manual
	if in.SendPaymentLink {
		original, committed, err := m.committedIntent(ctx, token, storeID, key)
		if err != nil {
			return ForBuyerSend{}, mapForBuyerError(err)
		}
		if committed && original.CommercialState == "CANCELLED" {
			// No planner or relink on a cancelled original order. A historical receipt may only report the existing DM.
			out := ForBuyerSend{State: "not_sent", Reason: "order_cancelled"}
			err := platform.WithScope(ctx, m.pool, token, storeID, manualPermission, func(tx pgx.Tx, scope platform.Scope) error {
				var raw []byte
				err := tx.QueryRow(ctx, `SELECT response FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND operation='inbox.dm.order_pay_link' AND idempotency_key=$3`,
					scope.TenantID, scope.StoreID, key).Scan(&raw)
				if errors.Is(err, pgx.ErrNoRows) {
					return nil
				}
				if err != nil {
					return err
				}
				var existing inbox.SendOutput
				if err := json.Unmarshal(raw, &existing); err != nil {
					return err
				}
				out = ForBuyerSend{OperationID: existing.OperationID, State: existing.SendState}
				return nil
			})
			if err != nil {
				return ForBuyerSend{}, mapForBuyerError(err)
			}
			return out, nil
		}
	}
	switch {
	case !in.SendPaymentLink:
		return ForBuyerSend{State: "not_sent", Reason: "not_requested"}, nil
	case target.ConversationID == nil:
		return ForBuyerSend{State: "not_sent", Reason: "no_conversation"}, nil
	case f.planner == nil:
		return ForBuyerSend{State: "not_sent", Reason: "send_unavailable"}, nil
	}
	conversation := *target.ConversationID
	// Probe (no write): a DM of this key is already planned (replay), or the window is closed (do not re-issue a link for nothing).
	var planned inbox.SendOutput
	err := platform.WithScope(ctx, m.pool, token, storeID, manualPermission, func(tx pgx.Tx, scope platform.Scope) error {
		var inner error
		planned, inner = f.planner.PlanOrderPayLink(ctx, tx, scope, key, conversation, orderID, "")
		return inner
	})
	switch {
	case err == nil:
		return ForBuyerSend{OperationID: planned.OperationID, State: planned.SendState}, nil
	case errors.Is(err, inbox.ErrPayLinkNotPlanned):
	default:
		return f.payLinkOutcome(err)
	}
	var issued string
	err = platform.WithScope(ctx, m.pool, token, storeID, manualPermission, func(tx pgx.Tx, scope platform.Scope) error {
		// Re-probe under the order lock: a concurrent same-key request may have finished after our first probe.
		var inner error
		planned, inner = f.planner.PlanOrderPayLink(ctx, tx, scope, key, conversation, orderID, "")
		if inner == nil {
			return nil
		}
		if !errors.Is(inner, inbox.ErrPayLinkNotPlanned) {
			return inner
		}
		// A new private key excludes legacy relink-only receipts from the former two-transaction implementation.
		// Place already committed, so even its fresh link may have been superseded. Reissue and seal under the order lock.
		regenerated, _, inner := m.regenerateLink(ctx, tx, scope, token, stepKey(key, "dm-plan-link"), ManualRegenerateInput{OrderID: orderID, Locale: locale})
		if inner != nil {
			return inner
		}
		issued = regenerated.BuyerLink
		planned, inner = f.planner.PlanOrderPayLink(ctx, tx, scope, key, conversation, orderID, issued)
		return inner
	})
	if err == nil {
		if freshLink && buyerLink != nil && issued != "" {
			*buyerLink = issued
		}
		return ForBuyerSend{OperationID: planned.OperationID, State: planned.SendState}, nil
	}
	var coded *Error
	if errors.As(err, &coded) && (coded == ErrUnavailable || coded.Code == "conflict") {
		return ForBuyerSend{State: "not_sent", Reason: "link_unavailable"}, nil
	}
	return f.payLinkOutcome(err)
}

// payLinkOutcome maps a DM planner refusal to "not_sent" with a fixed reason; anything else is a retryable failure of the request.
func (f *ForBuyer) payLinkOutcome(err error) (ForBuyerSend, error) {
	notSent := func(reason string) (ForBuyerSend, error) { return ForBuyerSend{State: "not_sent", Reason: reason}, nil }
	var sendErr *inbox.SendError
	var pg *pgconn.PgError
	switch {
	case errors.Is(err, inbox.ErrSendUnavailable):
		return notSent("send_unavailable")
	case errors.As(err, &sendErr):
		if sendErr.Code == "conversation_gone" {
			return notSent("no_conversation")
		}
		return notSent(sendErr.Code)
	case errors.As(err, &pg) && (pg.Code == "PT409" || pg.Code == "PT429" || pg.Code == "PT403"):
		switch pg.Message {
		case "window_closed", "capability", "takeover_changed", "duplicate_recent":
			return notSent(pg.Message)
		case "conversation_gone":
			return notSent("no_conversation")
		case "rate_limited":
			return notSent("rate_limited")
		case "forbidden":
			return notSent("capability")
		}
	case errors.Is(err, command.ErrNotFound):
		return notSent("no_conversation")
	}
	return ForBuyerSend{}, mapForBuyerError(err)
}

// ---- A15: GET inbox/order-prefill --------------------------------------------------------------------------------------------------------

// PrefillItem is one claim line of the prefill. LivePriceMinor is null when the offer has no usable live price; LiveQuantityRemaining is the
// claimed quantity minus the units held by non-CANCELLED orders (P2-7 a).
type PrefillItem struct {
	SKUID                 string `json:"sku_id"`
	OfferID               string `json:"offer_id"`
	Keyword               string `json:"keyword"`
	Name                  string `json:"name"`
	Variant               string `json:"variant"`
	Quantity              int64  `json:"quantity"`
	LivePriceMinor        *int64 `json:"live_price_minor"`
	CatalogPriceMinor     int64  `json:"catalog_price_minor"`
	Sellable              bool   `json:"sellable"`
	LiveQuantityRemaining int64  `json:"live_quantity_remaining"`
}

// PrefillDelivery is the linked customer's most recent delivery: a CVS store or a home address, under the option key the options list offers.
type PrefillDelivery struct {
	OptionKey   string                  `json:"option_key"`
	CVS         *ManualCVS              `json:"cvs,omitempty"`
	HomeAddress *storefront.HomeAddress `json:"home_address,omitempty"`
}

// Prefill is the A15 answer. Customer and LastDelivery are filled ONLY from an explicit merchant link (conversation_state.customer_id), never from
// a bundle owner (I09); otherwise they are null and the UI shows empty fields.
type Prefill struct {
	Items              []PrefillItem    `json:"items"`
	Bundles            []string         `json:"bundles"`
	Customer           *ManualCustomer  `json:"customer"`
	LastDelivery       *PrefillDelivery `json:"last_delivery"`
	SuggestedOptionKey *string          `json:"suggested_option_key"`
	LivePriceEligible  bool             `json:"live_price_eligible"`
	LivePriceReason    string           `json:"live_price_reason"`
}

// Prefill reads the claim lines of one bundle or of the bundles linked to one conversation (exactly one non-nil), the live-price eligibility the
// server will apply on A16, and (conversation only) the explicitly linked customer's last contact and delivery. Read-only, orders:read +
// inventory:reserve. Side effects: none.
func (f *ForBuyer) Prefill(ctx context.Context, token, storeID string, conversationID, bundleID *string) (Prefill, error) {
	if f == nil || f.manual == nil {
		return Prefill{}, ErrManualDisabled
	}
	if (conversationID == nil) == (bundleID == nil) || (conversationID != nil && !command.ValidID(*conversationID)) || (bundleID != nil && !command.ValidID(*bundleID)) {
		return Prefill{}, &Error{Status: http.StatusUnprocessableEntity, Code: "invalid_request"}
	}
	out := Prefill{Items: []PrefillItem{}, Bundles: []string{}}
	var customerID *string
	err := platform.WithScope(ctx, f.manual.pool, token, storeID, "orders:read", func(tx pgx.Tx, scope platform.Scope) error {
		if inner := platform.RequirePermission(ctx, tx, scope, token, manualPermission); inner != nil {
			return inner
		}
		lines, inner := claims.ForBuyerLines(ctx, tx, conversationID, bundleID)
		if inner != nil {
			return inner
		}
		if out.Items, inner = prefillItems(ctx, tx, scope, lines); inner != nil {
			return inner
		}
		for _, l := range lines {
			if !slices.Contains(out.Bundles, l.BundleID) {
				out.Bundles = append(out.Bundles, l.BundleID)
			}
		}
		if out.LivePriceReason, inner = f.eligibility(ctx, tx, scope, token, conversationID, out.Bundles); inner != nil {
			return inner
		}
		out.LivePriceEligible = out.LivePriceReason == ""
		if conversationID != nil {
			// social.conversation_meta (0119): the explicit merchant link of A14; null for an unlinked conversation. Its definer re-checks
			// inbox:read, which A15 does not require: a refusal (inside a savepoint, so the transaction survives) simply means "no link known".
			sp, e := tx.Begin(ctx)
			if e != nil {
				return e
			}
			if e = sp.QueryRow(ctx, `SELECT linked_customer_id::text FROM social.conversation_meta($1::uuid)`, *conversationID).Scan(&customerID); e != nil {
				customerID = nil
				_ = sp.Rollback(ctx)
			} else if e = sp.Commit(ctx); e != nil {
				return e
			}
		}
		return nil
	})
	if err != nil {
		return Prefill{}, mapForBuyerError(err)
	}
	if customerID != nil {
		f.fillLinkedCustomer(ctx, token, storeID, *customerID, &out)
	}
	return out, nil
}

// eligibility is the live_price_eligible/reason of the prefill: the same comparison A16 applies (claims.for_buyer_peer_state), plus the live:manage
// permission. A conversation without any linked bundle is "unverified" (P2-7 c), a bare bundle has no conversation to compare with.
func (f *ForBuyer) eligibility(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, conversationID *string, bundles []string) (string, error) {
	if conversationID == nil {
		return "no_conversation", nil
	}
	if len(bundles) == 0 {
		return "bundle_buyer_unverified", nil
	}
	var reason string
	// Calls claims.for_buyer_peer_state (0129): the server compares peers; nothing here trusts a client value.
	if err := tx.QueryRow(ctx, `SELECT claims.for_buyer_peer_state($1::uuid,$2::uuid[])`, *conversationID, bundles).Scan(&reason); err != nil {
		return "", mapForBuyerError(err)
	}
	if reason != "" {
		return reason, nil
	}
	if err := platform.RequirePermission(ctx, tx, scope, token, "live:manage"); err != nil {
		if errors.Is(err, platform.ErrForbidden) {
			return "permission", nil
		}
		return "", err
	}
	return "", nil
}

// prefillItems joins the claim lines with catalog facts (name, SKU code as the variant label, catalog price, availability).
func prefillItems(ctx context.Context, tx pgx.Tx, scope platform.Scope, lines []claims.ForBuyerLine) ([]PrefillItem, error) {
	items := make([]PrefillItem, 0, len(lines))
	if len(lines) == 0 {
		return items, nil
	}
	skus := make([]string, 0, len(lines))
	for _, l := range lines {
		skus = append(skus, l.SKUID)
	}
	type fact struct {
		name, code string
		price      int64
		sellable   bool
	}
	facts := map[string]fact{}
	// catalog.skus / catalog.products / control.stores (commerce_runtime, RLS-scoped): the same availability rule claims.RedeemLink applies.
	rows, err := tx.Query(ctx, `SELECT s.id::text,p.name,s.code,s.price_minor,(s.status='active' AND p.status='active' AND s.currency=st.currency)
		FROM catalog.skus s JOIN catalog.products p ON p.tenant_id=s.tenant_id AND p.store_id=s.store_id AND p.id=s.product_id
		JOIN control.stores st ON st.tenant_id=s.tenant_id AND st.id=s.store_id
		WHERE s.tenant_id=$1 AND s.store_id=$2 AND s.id=ANY($3::uuid[])`, scope.TenantID, scope.StoreID, skus)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var f fact
		if err = rows.Scan(&id, &f.name, &f.code, &f.price, &f.sellable); err != nil {
			return nil, err
		}
		facts[id] = f
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	for _, l := range lines {
		f := facts[l.SKUID]
		items = append(items, PrefillItem{SKUID: l.SKUID, OfferID: l.OfferID, Keyword: l.Keyword, Name: f.name, Variant: f.code, Quantity: l.Quantity,
			LivePriceMinor: l.LivePriceMinor, CatalogPriceMinor: f.price, Sellable: f.sellable, LiveQuantityRemaining: l.LiveRemaining})
	}
	return items, nil
}

// fillLinkedCustomer pre-fills the contact and delivery from the explicitly linked customer's newest authorized order.
// Best effort inside a savepoint: no contact escapes a refused read; both permissions are rechecked before publishing the optional result.
func (f *ForBuyer) fillLinkedCustomer(ctx context.Context, token, storeID, customerID string, out *Prefill) {
	options, err := f.manual.Options(ctx, token, storeID)
	if err != nil {
		options = nil
	}
	var contact Prefill
	err = platform.WithScope(ctx, f.manual.pool, token, storeID, "orders:read", func(tx pgx.Tx, scope platform.Scope) error {
		sp, err := tx.Begin(ctx) // savepoint: a refusal of the optional read must not abort the transaction
		if err != nil {
			return nil
		}
		// customers.Get -> merchantorders.Get: the existing projections (customers:read / orders:read re-verified in SQL), newest order first.
		detail, err := customers.Get(ctx, sp, scope, token, customerID)
		if err != nil || len(detail.Orders) == 0 {
			_ = sp.Rollback(ctx)
			return nil
		}
		newest := detail.Orders[0]
		for _, o := range detail.Orders {
			if o.CreatedAt > newest.CreatedAt {
				newest = o
			}
		}
		order, err := merchantorders.Get(ctx, sp, scope, token, newest.OrderID)
		var email string
		if err == nil {
			// Calls fulfillment.order_prefill_email (0172): only this exact authorized order, gated by orders:read AND customers:read.
			// The narrow definer avoids a direct runtime buyer_email grant or expanding the public order projection.
			hash := sha256.Sum256([]byte(token))
			err = sp.QueryRow(ctx, `SELECT fulfillment.order_prefill_email($1,$2::uuid,$3::uuid)`, hash[:], storeID, newest.OrderID).Scan(&email)
		}
		_ = sp.Rollback(ctx) // read-only: nothing to keep
		if err != nil || (email != "" && (len(email) > 254 || !emailRx.MatchString(email))) {
			return nil
		}
		for _, permission := range []string{"orders:read", "customers:read"} {
			if err := platform.RequirePermission(ctx, tx, scope, token, permission); err != nil {
				return err
			}
		}
		d := order.Destination
		contact.Customer = &ManualCustomer{Name: d.RecipientName, Phone: d.Phone, Email: email}
		i := slices.IndexFunc(options, func(o ManualOption) bool { return o.Country == order.Country && o.DeliveryCode == order.ServiceCode })
		if i < 0 {
			return nil
		}
		delivery := &PrefillDelivery{OptionKey: options[i].OptionKey}
		switch {
		case d.Pickup != nil:
			delivery.CVS = &ManualCVS{StoreCode: d.Pickup.Code, StoreName: d.Pickup.Name, StoreAddress: d.Pickup.Address}
			if order.ServiceCode == "cvs_711" {
				key := options[i].OptionKey
				contact.SuggestedOptionKey = &key // 7-ELEVEN pickup, when the customer's last order used it (§5.1 step 1)
			}
		case d.Kind == "home":
			h := d.HomeAddress
			delivery.HomeAddress = &h
		}
		contact.LastDelivery = delivery
		return nil
	})
	if err == nil {
		out.Customer, out.LastDelivery, out.SuggestedOptionKey = contact.Customer, contact.LastDelivery, contact.SuggestedOptionKey
	}
}
