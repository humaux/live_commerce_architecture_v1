// Package httperror owns transport-safe error envelopes, never domain policy.
package httperror

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
)

type Envelope struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"request_id"`
	Retryable bool           `json:"retryable"`
	Details   map[string]any `json:"details"`
}

type requestKey struct{}

// Middleware assigns a server-owned correlation ID. Nested platform handlers
// reuse the context value, never an untrusted inbound X-Request-ID header.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := r.Context().Value(requestKey{}).(string)
		if id == "" {
			var value [16]byte
			_, _ = rand.Read(value[:]) // Go's crypto/rand terminates on entropy failure.
			id = hex.EncodeToString(value[:])
			r = r.WithContext(context.WithValue(r.Context(), requestKey{}, id))
		}
		w.Header().Set("X-Request-ID", id)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(&errorWriter{ResponseWriter: w}, r)
	})
}

func Write(w http.ResponseWriter, status int, code string) {
	write(w, status, code, status == http.StatusServiceUnavailable || status == http.StatusTooManyRequests)
}

// WriteNonRetryable is for non-idempotent operations such as issuing a new
// anonymous owner. A 503 does not prove that the database failed to commit.
func WriteNonRetryable(w http.ResponseWriter, status int, code string) {
	write(w, status, code, false)
}

// WriteDetails is Write with a bounded details object (e.g. {"max":1000} for invalid_text); callers pass fixed keys and numbers or
// fixed reason codes only, never a request value.
func WriteDetails(w http.ResponseWriter, status int, code string, details map[string]any) {
	writeDetails(w, status, code, status == http.StatusServiceUnavailable || status == http.StatusTooManyRequests, details)
}

func write(w http.ResponseWriter, status int, code string, retryable bool) {
	writeDetails(w, status, code, retryable, map[string]any{})
}

func writeDetails(w http.ResponseWriter, status int, code string, retryable bool, details map[string]any) {
	messages := map[string]string{
		"unauthorized": "Sign-in required.", "forbidden": "Operation not permitted.",
		"not_found": "Resource not found.", "method_not_allowed": "Method not allowed.",
		"invalid_request": "Request validation failed.", "invalid_json": "Malformed JSON body.",
		"json_required": "JSON content type required.", "conflict": "Request conflicts with current state.",
		"rate_limited":           "Too many requests.",
		"insufficient_inventory": "Insufficient available inventory.",
		"retry_later":            "Temporarily unavailable.", "unavailable": "Temporarily unavailable.",
		"internal": "Request could not be completed.",
		// meta-ads-v1 §7 / internal/ads frozenStatus: every frozen ads refusal code must be listed here or the
		// merchant sees "internal" (found by MA04; internal/ads TestFrozenCodesSurviveHTTPError guards the drift).
		"state_mismatch":           "This Meta connection attempt does not belong to this session.",
		"state_expired":            "This Meta connection attempt expired.",
		"meta_connect_failed":      "Meta could not complete the connection.",
		"not_in_pick_list":         "That ad account or dataset was not offered by Meta for this login.",
		"client_business_changed":  "This ad account now belongs to a different Meta business.",
		"revision_changed":         "The draft changed since it was loaded.",
		"draft_approved":           "An approved draft cannot be edited.",
		"over_allowance":           "The budget exceeds the store's ads allowance.",
		"attempt_changed":          "The publish attempt changed since it was loaded.",
		"prior_attempt_not_paused": "An earlier attempt is not confirmed paused.",
		"budget_below_minimum":     "The budget is below the minimum.",
		"not_whole_unit":           "The budget must be a whole currency unit.",
		"currency_mismatch":        "The currency does not match the ad account.",
		"starts_too_soon":          "The start time is too soon.",
		"binding_disabled":         "The Meta ads connection is not enabled.",
		"source_not_owned":         "That post does not belong to this store's connection.",
		"product_not_published":    "The product is not published.",
		// meta-connect (merchant Facebook Page / Instagram connect, internal/metaconnect frozenStatus): codes not already listed above;
		// internal/metaconnect TestFrozenCodesSurviveHTTPError guards the drift.
		"state_used":         "That Meta connection attempt was already completed.",
		"missing_permission": "A required Facebook permission or Page access is missing.",
		"cap_exceeded":       "This store has reached the limit of 10 connected Facebook Pages.",
		"page_taken":         "That Facebook Page is already connected to another store.",
		"recheck_too_soon":   "A re-check was already requested in the last minute.",
		// stripe-refund-v1 §7.1 (ruling 15: unknown codes were rewritten to "internal").
		"refundable_changed":    "Refundable amount changed since it was loaded.",
		"exceeds_refundable":    "Amount exceeds the refundable amount.",
		"amount_step":           "Amount is not a valid step for this currency.",
		"not_refundable":        "Order cannot be refunded.",
		"refund_blocked_review": "Refund is blocked by an open payment review.",
		"refund_limit":          "Refund limit reached for this payment.",
		// manual-fulfilment-v1 §5.1.
		"version_changed":       "This item changed since it was loaded.",
		"not_shippable":         "Order cannot be shipped in its current state.",
		"invalid_carrier":       "Carrier is not valid.",
		"invalid_tracking":      "Tracking number is not valid.",
		"invalid_url":           "Tracking URL is not valid.",
		"void_requires_shipped": "Only a shipped record can be voided.",
		"invalid_void":          "Void request is not valid.",
		// manual-fulfilment-v1 Amendment "M-7 revoked" (bulk tracking import, unit w3-01b). The file-level
		// codes refuse the whole CSV (422); preview_stale (409) carries the fresh preview in its body.
		"preview_stale":     "The preview changed since it was loaded.",
		"nothing_to_apply":  "Nothing to apply.",
		"encoding_not_utf8": "The file must be UTF-8 encoded.",
		"too_many_rows":     "Too many rows.",
		"required":          "A required column is missing.",
		// manual-fulfilment-v1 Amendment W3-02B (pick list, unit w3-02b-picklist): > 500 order_ids.
		"too_many": "Too many orders.",
		// taiwan-cvs-logistics-v1 §8 / §5.2 / §16 (unit cvs-core). Ruling 15: an unknown code would be rewritten to "internal".
		"ecpay_probe_failed": "ECPay rejected the keys or could not be reached.", "invalid_sender": "Sender name or mobile number is not valid.",
		"ecpay_environment_not_allowed": "This ECPay environment is not allowed on this deployment.",
		"not_qualified":                 "The connection has not passed its check with the current keys.",
		"another_profile_enabled":       "Another ECPay connection of this store is enabled.", "merchant_id_changed": "The ECPay merchant id cannot change on rotation.",
		"connection_unavailable": "No usable ECPay connection for this store.", "no_cvs_destination": "The order has no ECPay-verified pickup store.",
		"cvs_recipient_rejected":   "The recipient name or mobile number does not meet the ECPay rules.",
		"cvs_environment_mismatch": "The pickup store or connection belongs to another ECPay environment.",
		"cvs_amount_exceeds":       "The amount is outside the ECPay convenience-store limits.", "cvs_source_mismatch": "This store source cannot be used with this delivery service.",
		"print_unsupported": "This label cannot be printed here.", "not_created": "No label has been created for this order yet.",
		"ecpay_shows_movement": "ECPay shows the parcel is not unmoved; it cannot be abandoned.", "ecpay_trade_found": "ECPay has this shipment; it was recorded as created.",
		"not_lapsed": "The ECPay order has not lapsed yet.", "reconcile_in_progress": "The shipment is still being reconciled with ECPay.",
		"acknowledgement_required": "Confirm that you checked the ECPay back office.", "attempt_in_flight": "A label request is still in progress.",
		"cvs_attempt_in_flight": "A label request may already be at ECPay.", "not_abandonable": "This shipment can no longer be abandoned.",
		"invalid_settings": "The convenience-store settings are not valid.", "not_pay_at_pickup": "The order is not a pay-at-pickup order.",
		"not_shipped": "The order has not been shipped.", "collection_state_changed": "The collection state changed since it was loaded.",
		"parcel_not_returned": "The parcel has not been returned yet.", "parcel_not_picked_up": "The parcel has not been picked up yet.", "not_cancellable": "The order can no longer be cancelled.",
		"idempotency_conflict": "This request key was already used for a different request.",
		"bad_return_path":      "The return page is not allowed.", "bad_return_origin": "The storefront origin is not allowed.",
		"service_unavailable": "This delivery service is not available.", "selection_replay_new_key": "Start the store selection again.",
		"bad_store_code": "The store number is not valid for this chain.", "bad_store_name": "The store name is not valid.",
		"bad_store_address": "The store address is not valid.", "pay_at_pickup_unavailable": "Pay at pickup is not available for this order.",
		"pay_at_pickup_amount_exceeds": "The amount is outside the pay-at-pickup limit.", "card_unavailable": "Card payment is not available for this store.", "pay_at_pickup_limit": "Too many pay-at-pickup orders are open.",
		// payment-methods-v1 Amendment W4-02B (PAYUNi self-serve activation); not_qualified is shared, declared with the CVS codes above.
		"platform_disabled": "Card payments are not enabled on this platform yet.", "profile_not_allowed": "This step is not available in this deployment.",
		"payuni_probe_failed": "PAYUNi did not confirm the keys, or could not be reached.",
		// storefront-v2 §C (unit checkout-offline): bank_transfer placement, proof, merchant confirm/reject/refund. Coded 422/409 of the 0088 definers.
		"bank_transfer_unavailable": "Bank transfer is not available for this order.", "not_bank_transfer": "The order is not a bank-transfer order.",
		"transfer_not_open": "This transfer is no longer open.", "transfer_window_closed": "The transfer window has ended.",
		"invalid_proof": "The transfer details are not valid.", "invalid_reason": "The rejection reason is not valid.", "transfer_not_submitted": "The buyer has not submitted transfer details.",
		"already_confirmed": "The transfer was already confirmed.", "already_refunded": "The transfer was already refunded.",
		"transfer_not_confirmed": "The transfer has not been confirmed.",
		// 0099 K3-02: the offline-refund restock choice.
		"already_shipped": "The order was already handed over; its stock cannot be restocked here.", "restock_unavailable": "The reserved stock of this order cannot be released.",
		// home-cod R5 (migration 0107): cash_on_delivery placement refusals of the begin_hold COD branch.
		"cash_on_delivery_unavailable": "Cash on delivery is not available for this order.", "cash_on_delivery_amount_exceeds": "The amount is outside the cash-on-delivery limit.",
		"cash_on_delivery_limit": "Too many cash-on-delivery orders are open.",
		// storefront-v2 §F (unit promotions): discount codes. The buyer codes are the 422s of the quote request and of BeginCheckout.
		"promo_invalid": "This discount code is not valid.", "promo_not_started": "This discount code is not active yet.",
		"promo_expired": "This discount code has expired.", "promo_min_subtotal": "The order is below the minimum amount for this discount code.",
		"promo_used_up": "This discount code has been fully used.", "promo_buyer_limit": "You have already used this discount code the maximum number of times.",
		"promo_changed": "The discount code changed; refresh the quote and try again.",
		"promo_exists":  "A discount code with this text already exists.", "invalid_promotion": "The discount code settings are not valid.",
		// meta-claims-intake-v1 §2 / claim-source unit: comment source binding (version_changed above is shared).
		"input_invalid":      "The pasted link or id is not a supported Facebook or Instagram post.",
		"input_unresolvable": "This link cannot be resolved without Meta; paste the numeric post or media id.",
		"binding_missing":    "No enabled Meta connection is ready for this post.",
		"binding_ambiguous":  "Several Meta connections are enabled; the post cannot be assigned to one.",
		"source_conflict":    "This post already feeds another live session.",
		"page_token_missing": "Private replies need a registered Page token for this connection.",
		// customers-billing-v1 §5/§6 (customers-core): consent, export and erasure.
		// idempotency_conflict: shared, declared with the CVS codes above.
		"erasure_blocked":  "Erasure is blocked while a hold, payment or refund is in progress.",
		"erased":           "This data has been erased.",
		"export_too_large": "The export is too large to generate.",
		// customers-billing-v1 Amendment W6-01B (tags and notes); version_changed is shared, declared above.
		"tag_exists":    "A tag with this name already exists.",
		"limit_reached": "The limit for tags or notes has been reached.",
		// customers-billing-v1 §5 / billing-core B2, B12: platform billing.
		"billing_unavailable": "Billing is temporarily unavailable.",
		"billing_restricted":  "New claim windows are paused until billing is up to date.",
		"subscription_exists": "This store already has a subscription.",
		"no_billing_customer": "No billing account exists for this store yet.",
		// storefront-v2 section G (unit merchant-tools): manual (merchant-created) orders and the product CSV export. Placement refusals reuse the
		// 0088/CVS codes above (bank_transfer_unavailable, pay_at_pickup_unavailable, insufficient_inventory, idempotency_conflict, export_too_large).
		"manual_order_unavailable": "Creating orders from the admin is not available on this deployment.",
		"cvs_entry_unavailable":    "This convenience-store service needs the store to be chosen from the map.",
		// product-editor §f (unit product-core, internal/catalog coded.go): the A6 document command refusals. Every catalog
		// code must be listed here or the merchant sees "internal" (catalogClassify checks *catalog.Error first).
		"amount_not_whole_twd": "For TWD the price must be a whole dollar.",
		"keyword_taken":        "That keyword is already used by another SKU.",
		"live_window_open":     "This product cannot be unlisted while its live window is open.",
		// live-console-v1 §3.3-3.6 / §11 (units LC-B3/LC-B4): inbox, takeover and manual sends. Every code must be listed or the merchant
		// sees "internal" (ruling 15); internal/httpapi inbox_send_test.go guards the drift.
		"takeover_changed":               "Another staff member took over this conversation; reload before sending.",
		"version_conflict":               "This item changed since it was loaded.",
		"no_source":                      "This live session has no comment source.",
		"window_closed":                  "The 24-hour messaging window with this buyer is closed.",
		"capability":                     "This connection cannot send this kind of message right now.",
		"conversation_gone":              "This conversation is no longer available.",
		"duplicate_recent":               "The same message was just sent.",
		"invalid_text":                   "The message text is not valid.",
		"used":                           "This comment's one private reply was already used.",
		"auto_pending":                   "The automatic reply to this comment is still pending.",
		"auto_pending_confirm":           "The automatic reply may be about to send; confirm to send a manual one instead.",
		"expired_7d":                     "The private-reply window for this comment has passed.",
		"ig_live_ended":                  "The Instagram live private-reply window has ended.",
		"page_comment":                   "This comment was written by the Page itself.",
		"reply_comment_unsupported":      "Replies to replies cannot receive a private reply.",
		"comment_unknown":                "This comment was not found for this live session.",
		"comment_facts_unavailable":      "The time and author of this comment cannot be confirmed.",
		"public_reply_forbidden_content": "Public replies cannot contain links, contact details or payment links.",
		"ig_live_unsupported":            "This action is not available for Instagram live.",
		"offer_unavailable":              "This offer is not available.",
		"stream_unavailable":             "The comment stream is temporarily unavailable.",
		"invalid_cursor":                 "The page cursor is not valid.",
		"invalid_ref":                    "The comment reference is not valid.",
		"invalid_filter":                 "The filter is not valid.",
		// live-console-v1 §5 / §11 A16 (unit LC-B6): order made for a buyer from the inbox.
		"bundle_already_ordered": "These claims already have an order.",
		"bundle_buyer_mismatch":  "These claims belong to a different buyer than this conversation.",
	}
	message, ok := messages[code]
	if !ok {
		code, message = "internal", messages["internal"]
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Envelope{Code: code, Message: message,
		RequestID: w.Header().Get("X-Request-ID"), Retryable: retryable,
		Details: details})
}

// ServeMux generates plain-text 404/405 responses. Translate only non-JSON
// errors at the boundary; never buffer application responses or expose text.
type errorWriter struct {
	http.ResponseWriter
	wrote, suppressed bool
}

func (w *errorWriter) WriteHeader(status int) {
	if w.wrote {
		return
	}
	w.wrote = true
	if status >= 400 && !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
		w.suppressed = true
		code := "internal"
		if status == 404 {
			code = "not_found"
		}
		if status == 405 {
			code = "method_not_allowed"
		}
		w.Header().Del("Content-Length")
		Write(w.ResponseWriter, status, code)
		return
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *errorWriter) Write(p []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	if w.suppressed {
		return len(p), nil
	}
	return w.ResponseWriter.Write(p)
}

func (w *errorWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
