// Purpose: the W3-08B HTTP adapter of merchant returns (RMA) and merchant order cancel (contracts/returns-v1.md §6): POST cancel,
//   POST/GET order returns, GET returns list and the four RMA step routes (receive, inspect, close, cancel). Each route is a thin
//   transport gate over one internal/returns or internal/fulfillment command; every rule lives in migration 0155.
// Depends on: returns.Register/Receive/Inspect/Close/CancelRMA/ForOrder/List, fulfillment.CancelOrder, shipments.go helpers
//   (shipmentRoute, shipmentScope, parcelKey), studioDecodeRaw, platform.WithScope (one READ COMMITTED transaction).
// Used by: NewHandler (handler.go registerReturnRoutes). Tests: returns_test.go (DB-free router) and TestReturns*/TestMerchantCancel* (REAL_PG).
// Invariants: Idempotency-Key exactly once on every POST, none on GET; strict JSON bodies (unknown or duplicate keys 400); every 2xx waits
//   for COMMIT; no body, token or reason is logged; a return/cancel never starts a refund (I13).

package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/command"
	"livecommerce/internal/fulfillment"
	"livecommerce/internal/platform"
	"livecommerce/internal/returns"
)

type cancelOrderBody struct {
	ExpectedState string `json:"expected_state"`
	Reason        string `json:"reason"`
}

type registerReturnBody struct {
	Reason string                 `json:"reason"`
	Lines  []returns.RegisterLine `json:"lines"`
}

type receiveReturnBody struct {
	ExpectedVersion int64                 `json:"expected_version"`
	Lines           []returns.ReceiveLine `json:"lines"`
}

type inspectReturnBody struct {
	ExpectedVersion int64                 `json:"expected_version"`
	Lines           []returns.InspectLine `json:"lines"`
}

type closeReturnBody struct {
	ExpectedVersion int64   `json:"expected_version"`
	RefundID        *string `json:"refund_id"`
}

type cancelReturnBody struct {
	ExpectedVersion int64 `json:"expected_version"`
}

// returnsClassify maps the closed refusal set of the returns/cancel commands, then the shipment/claims table (deadlock -> 503
// retry_later, unclassified -> 503 unavailable). No driver message is ever returned.
func returnsClassify(err error) (int, string) {
	var coded *returns.Error
	switch {
	case errors.As(err, &coded):
		return coded.Status, coded.Code
	case errors.Is(err, returns.ErrUnavailable):
		return http.StatusServiceUnavailable, "unavailable"
	}
	return claimsClassify(err)
}

// returnScope is shipmentScope with the returns classifier: it runs fn in one scoped transaction opened with permission and has
// already written the classified error on failure; ok=true means COMMIT was acknowledged.
func returnScope(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool, permission string, fn action) (any, bool) {
	if !canonicalBearer(r) {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return nil, false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var result any
	err := platform.WithScope(ctx, pool, bearerToken(r), r.PathValue("store_id"), permission, func(tx pgx.Tx, s platform.Scope) error {
		var inner error
		result, inner = fn(ctx, tx, s, r)
		return inner
	})
	if err != nil {
		status, code := returnsClassify(err)
		respondError(w, status, code)
		return nil, false
	}
	return result, true
}

// rmaRoute is shipmentRoute plus the canonical rma_id path value.
func rmaRoute(method string, next http.HandlerFunc) http.HandlerFunc {
	return shipmentRoute(method, func(w http.ResponseWriter, r *http.Request) {
		if id := r.PathValue("rma_id"); id != "" && !command.ValidID(id) {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		next(w, r)
	})
}

// registerReturnRoutes mounts the nine W3-08B routes; NewHandler calls it unconditionally.
func registerReturnRoutes(mux *http.ServeMux, pool *pgxpool.Pool) {
	const base = "/v1/admin/stores/{store_id}"
	// POST cancel: {expected_state, reason}; fulfillment:write. A paid card order is refused 409 refund_first until refunds cover the capture.
	mux.HandleFunc("POST "+base+"/orders/{order_id}/cancel", shipmentRoute(http.MethodPost, func(w http.ResponseWriter, r *http.Request) {
		key, ok := parcelKey(w, r)
		if !ok {
			return
		}
		in, _, ok := studioDecodeRaw[cancelOrderBody](w, r, []string{"expected_state", "reason"})
		if !ok {
			return
		}
		result, ok := returnScope(w, r, pool, "fulfillment:write", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return fulfillment.CancelOrder(ctx, tx, s, bearerToken(r), key, r.PathValue("order_id"), in.ExpectedState, in.Reason)
		})
		if ok {
			respond(w, http.StatusOK, result)
		}
	}))
	// GET cancelled card orders whose in-flight refund later failed (orders:read); the literal segment wins over {order_id}.
	mux.HandleFunc("GET "+base+"/orders/cancel-refund-gaps", shipmentRoute(http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
		result, ok := returnScope(w, r, pool, "orders:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			items, err := returns.CancelRefundGaps(ctx, tx, s, bearerToken(r))
			return struct {
				Items []returns.RefundGap `json:"items"`
			}{items}, err
		})
		if ok {
			respond(w, http.StatusOK, result)
		}
	}))
	// POST register: {reason, lines[{sku_id, warehouse_id?, quantity}]}; fulfillment:write; shipped orders only.
	mux.HandleFunc("POST "+base+"/orders/{order_id}/returns", shipmentRoute(http.MethodPost, func(w http.ResponseWriter, r *http.Request) {
		key, ok := parcelKey(w, r)
		if !ok {
			return
		}
		in, _, ok := studioDecodeRaw[registerReturnBody](w, r, []string{"reason", "lines"})
		if !ok {
			return
		}
		result, ok := returnScope(w, r, pool, "fulfillment:write", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return returns.Register(ctx, tx, s, bearerToken(r), key, r.PathValue("order_id"), in.Reason, in.Lines)
		})
		if ok {
			respond(w, http.StatusCreated, result)
		}
	}))
	// GET the RMAs of one order (orders:read).
	mux.HandleFunc("GET "+base+"/orders/{order_id}/returns", shipmentRoute(http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
		result, ok := returnScope(w, r, pool, "orders:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			items, err := returns.ForOrder(ctx, tx, s, bearerToken(r), r.PathValue("order_id"))
			return struct {
				Items []returns.RMA `json:"items"`
			}{items}, err
		})
		if ok {
			respond(w, http.StatusOK, result)
		}
	}))
	// GET the store's 100 newest RMAs, optionally ?state=REGISTERED|RECEIVED|INSPECTED|CLOSED|CANCELLED (orders:read).
	mux.HandleFunc("GET "+base+"/returns", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, private")
		q := r.URL.Query()
		if len(q) > 1 || len(q["state"]) > 1 || (len(q) == 1 && len(q["state"]) != 1) || len(r.Header.Values("Idempotency-Key")) != 0 ||
			r.ContentLength != 0 || len(r.TransferEncoding) != 0 {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		state := q.Get("state")
		result, ok := returnScope(w, r, pool, "orders:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			items, err := returns.List(ctx, tx, s, bearerToken(r), state)
			return struct {
				Items []returns.RMA `json:"items"`
			}{items}, err
		})
		if ok {
			respond(w, http.StatusOK, result)
		}
	})
	// POST receive / inspect / close / cancel of one RMA: {expected_version, ...}; CAS on the RMA version.
	mux.HandleFunc("POST "+base+"/returns/{rma_id}/receive", rmaRoute(http.MethodPost, func(w http.ResponseWriter, r *http.Request) {
		key, ok := parcelKey(w, r)
		if !ok {
			return
		}
		in, _, ok := studioDecodeRaw[receiveReturnBody](w, r, []string{"expected_version", "lines"})
		if !ok {
			return
		}
		result, ok := returnScope(w, r, pool, "fulfillment:write", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return returns.Receive(ctx, tx, s, bearerToken(r), key, r.PathValue("rma_id"), in.ExpectedVersion, in.Lines)
		})
		if ok {
			respond(w, http.StatusOK, result)
		}
	}))
	mux.HandleFunc("POST "+base+"/returns/{rma_id}/inspect", rmaRoute(http.MethodPost, func(w http.ResponseWriter, r *http.Request) {
		key, ok := parcelKey(w, r)
		if !ok {
			return
		}
		in, _, ok := studioDecodeRaw[inspectReturnBody](w, r, []string{"expected_version", "lines"})
		if !ok {
			return
		}
		// The scope opens with inventory:write (the stock-deciding step); returns.Inspect also requires fulfillment:write.
		result, ok := returnScope(w, r, pool, "inventory:write", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return returns.Inspect(ctx, tx, s, bearerToken(r), key, r.PathValue("rma_id"), in.ExpectedVersion, in.Lines)
		})
		if ok {
			respond(w, http.StatusOK, result)
		}
	}))
	mux.HandleFunc("POST "+base+"/returns/{rma_id}/close", rmaRoute(http.MethodPost, func(w http.ResponseWriter, r *http.Request) {
		key, ok := parcelKey(w, r)
		if !ok {
			return
		}
		in, _, ok := studioDecodeRaw[closeReturnBody](w, r, []string{"expected_version", "refund_id"})
		if !ok {
			return
		}
		result, ok := returnScope(w, r, pool, "inventory:write", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return returns.Close(ctx, tx, s, bearerToken(r), key, r.PathValue("rma_id"), in.ExpectedVersion, in.RefundID)
		})
		if ok {
			respond(w, http.StatusOK, result)
		}
	}))
	mux.HandleFunc("POST "+base+"/returns/{rma_id}/cancel", rmaRoute(http.MethodPost, func(w http.ResponseWriter, r *http.Request) {
		key, ok := parcelKey(w, r)
		if !ok {
			return
		}
		in, _, ok := studioDecodeRaw[cancelReturnBody](w, r, []string{"expected_version"})
		if !ok {
			return
		}
		result, ok := returnScope(w, r, pool, "fulfillment:write", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return returns.CancelRMA(ctx, tx, s, bearerToken(r), key, r.PathValue("rma_id"), in.ExpectedVersion)
		})
		if ok {
			respond(w, http.StatusOK, result)
		}
	}))
}
