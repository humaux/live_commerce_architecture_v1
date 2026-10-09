// Purpose: the W3-07B HTTP adapter of parcel groups (manual-fulfilment-v1 Amendment W3-07B): GET merge suggestions, GET open groups
//   (W3-U4), POST create group, DELETE dissolve group and PUT group shipment. Each route is a thin transport gate over one internal/merchantorders
//   command; every rule (owner, destination, COD/CVS exclusion, CAS, atomic shipment) lives in migration 0146 and parcels.go.
// Depends on: merchantorders.MergeSuggestions/OpenParcelGroups/CreateParcelGroup/DissolveParcelGroup/ShipParcelGroup, shipments.go helpers
//   (shipmentRoute, shipmentScope, shipmentClassify, decodeShipmentBody), platform.WithScope (one READ COMMITTED transaction).
// Used by: NewHandler (handler.go registerParcelRoutes). Tests: parcels_test.go (DB-free router) and TestParcelGroup* (REAL_PG).
// Invariants: Idempotency-Key exactly once on POST/PUT, none on GET/DELETE; no query except ?expected_version on DELETE; every
//   2xx waits for COMMIT; no body, token or recipient is logged; I05 (merging parcels never touches money).

package httpapi

import (
	"context"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/command"
	"livecommerce/internal/merchantorders"
	"livecommerce/internal/platform"
)

// parcelCreateBody is the POST body: exactly one key, order_ids (2..20 ids, validated by the command and SQL).
type parcelCreateBody struct {
	OrderIDs []string `json:"order_ids"`
}

// registerParcelRoutes mounts the five parcel routes (four W3-07B + the W3-U4 open-groups read); NewHandler calls it unconditionally.
func registerParcelRoutes(mux *http.ServeMux, pool *pgxpool.Pool) {
	const base = "/v1/admin/stores/{store_id}"
	// GET suggestions: same buyer + same address, read only (orders:read).
	mux.HandleFunc("GET "+base+"/orders/merge-suggestions", shipmentRoute(http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
		result, ok := shipmentScope(w, r, pool, "orders:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			items, err := merchantorders.MergeSuggestions(ctx, tx, s, bearerToken(r))
			return struct {
				Items []merchantorders.MergeSuggestion `json:"items"`
			}{items}, err
		})
		if ok {
			respond(w, http.StatusOK, result)
		}
	}))
	// GET open groups: the store's OPEN groups with members (masked recipients), so the UI can rebuild ship/dissolve panels after a
	// reload (orders:read, no query). Calls fulfillment.read_open_parcel_groups via merchantorders.OpenParcelGroups (migration 0166).
	mux.HandleFunc("GET "+base+"/parcel-groups", shipmentRoute(http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
		result, ok := shipmentScope(w, r, pool, "orders:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			items, err := merchantorders.OpenParcelGroups(ctx, tx, s, bearerToken(r))
			return struct {
				Items []merchantorders.OpenParcelGroup `json:"items"`
			}{items}, err
		})
		if ok {
			respond(w, http.StatusOK, result)
		}
	}))
	// POST create: {order_ids}; the server re-derives owner, destination and eligibility (fulfillment:write).
	mux.HandleFunc("POST "+base+"/parcel-groups", shipmentRoute(http.MethodPost, func(w http.ResponseWriter, r *http.Request) {
		key, ok := parcelKey(w, r)
		if !ok {
			return
		}
		in, _, ok := studioDecodeRaw[parcelCreateBody](w, r, []string{"order_ids"})
		if !ok {
			return
		}
		if in.OrderIDs == nil {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		result, ok := shipmentScope(w, r, pool, "fulfillment:write", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return merchantorders.CreateParcelGroup(ctx, tx, s, bearerToken(r), key, in.OrderIDs)
		})
		if ok {
			respond(w, http.StatusCreated, result)
		}
	}))
	// DELETE dissolve: ?expected_version=N, OPEN groups only (fulfillment:write).
	mux.HandleFunc("DELETE "+base+"/parcel-groups/{group_id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, private")
		q := r.URL.Query()
		version, err := strconv.ParseInt(q.Get("expected_version"), 10, 64)
		if len(q) != 1 || len(q["expected_version"]) != 1 || err != nil || version < 1 || !command.ValidID(r.PathValue("group_id")) ||
			len(r.Header.Values("Idempotency-Key")) != 0 || r.ContentLength != 0 || len(r.TransferEncoding) != 0 {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		result, ok := shipmentScope(w, r, pool, "fulfillment:write", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return merchantorders.DissolveParcelGroup(ctx, tx, s, bearerToken(r), r.PathValue("group_id"), version)
		})
		if ok {
			respond(w, http.StatusOK, result)
		}
	})
	// PUT shipment: one ShipmentInput (record, expected_version 0) shipped on every member in one transaction.
	mux.HandleFunc("PUT "+base+"/parcel-groups/{group_id}/shipment", shipmentRoute(http.MethodPut, func(w http.ResponseWriter, r *http.Request) {
		key, ok := parcelKey(w, r)
		if !ok {
			return
		}
		if !command.ValidID(r.PathValue("group_id")) {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		in, ok := decodeShipmentBody(w, r)
		if !ok {
			return
		}
		result, ok := shipmentScope(w, r, pool, "fulfillment:write", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return merchantorders.ShipParcelGroup(ctx, tx, s, bearerToken(r), key, r.PathValue("group_id"), in)
		})
		if ok {
			respond(w, http.StatusOK, result)
		}
	}))
}

// parcelKey returns the single canonical Idempotency-Key or answers 422 invalid_request.
func parcelKey(w http.ResponseWriter, r *http.Request) (string, bool) {
	keys := r.Header.Values("Idempotency-Key")
	if len(keys) != 1 || !claimsKey.MatchString(keys[0]) {
		respondError(w, http.StatusUnprocessableEntity, "invalid_request")
		return "", false
	}
	return keys[0], true
}
