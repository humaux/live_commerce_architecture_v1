// Purpose: the W3-02B HTTP adapter — the pick-list projection, the carrier-template CSV export and the CVS
//   batch label request. Each route is a thin transport gate over one internal/merchantorders or
//   internal/fulfillment method; every collection rule, permission and refusal stays in the SQL definers.
// Depends on: merchantorders.PickList/Export, fulfillment.CVS.Batch, platform.WithScope/WithScopeBudget,
//   identity.resolve_access (orders:read / orders:export / fulfillment:write via the SQL definers).
// Used by: cmd/api route registration (internal/httpapi/handler.go). Tests: TestPickList*/TestCarrierExport/TestCVSBatch.
// Invariants: no business rule, no provider call, no recipient field in a response or log, no carrier API
//   (the CSV is for the merchant to upload), no body/token/filename stored; 501 ids -> 422 too_many (PL03).

package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/fulfillment"
	"livecommerce/internal/merchantorders"
	"livecommerce/internal/platform"
)

// pickListBody is the shared {order_ids} XOR {session_id} selection; exactly one key must be present.
type pickListBody struct {
	OrderIDs  []string `json:"order_ids"`
	SessionID *string  `json:"session_id"`
}

// registerPickListRoutes mounts the three routes. NewHandler calls it unconditionally; the cvs-batch
// route is mounted only when a *fulfillment.CVS exists (like registerCVSRoutes).
func registerPickListRoutes(mux *http.ServeMux, pool *pgxpool.Pool, cvs *fulfillment.CVS) {
	const base = "/v1/admin/stores/{store_id}"

	mux.HandleFunc("POST "+base+"/orders/pick-list", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, private")
		if !pickListReadGate(w, r) {
			return
		}
		sel, ok := pickListSelection(w, r)
		if !ok {
			return
		}
		result, ok := pickListScoped(w, r, pool, "orders:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return merchantorders.PickList(ctx, tx, s, bearerToken(r), sel)
		})
		if ok {
			respond(w, http.StatusOK, result)
		}
	})

	mux.HandleFunc("POST "+base+"/orders/export", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, private")
		if !pickListReadGate(w, r) {
			return
		}
		template, ok := exportTemplate(r.URL)
		if !ok {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		sel, ok := pickListSelection(w, r)
		if !ok {
			return
		}
		result, ok := pickListScoped(w, r, pool, "orders:export", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return merchantorders.CarrierExport(ctx, tx, s, bearerToken(r), template, sel)
		})
		if !ok {
			return
		}
		writeCarrierExport(w, r.PathValue("store_id"), template, result.(merchantorders.CarrierExportFile), time.Now())
	})

	if cvs == nil {
		return
	}
	// cvsRoute: exact method, no query, Idempotency-Key exactly once, canonical bearer. cvsServe maps the
	// *fulfillment.CVSError / deadlock table and answers 200 only after COMMIT.
	mux.HandleFunc("POST "+base+"/shipments/cvs-batch", cvsRoute(http.MethodPost, true, func(w http.ResponseWriter, r *http.Request) {
		in, ok := cvsStrictBody[fulfillment.BatchInput](w, r, []string{"order_ids"}, nil)
		if !ok {
			return
		}
		cvsServe(w, r, 30*time.Second, http.StatusOK, func(ctx context.Context) (any, error) {
			return cvs.Batch(ctx, bearerToken(r), r.PathValue("store_id"), r.Header.Get("Idempotency-Key"), in)
		})
	}))
}

// pickListReadGate is the transport gate shared by the two read POSTs: POST only, no Idempotency-Key.
func pickListReadGate(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return false
	}
	if len(r.Header.Values("Idempotency-Key")) != 0 {
		respondError(w, http.StatusUnprocessableEntity, "invalid_request")
		return false
	}
	return true
}

// pickListSelection decodes exactly one of order_ids / session_id (both absent, both present, empty
// array or empty string are invalid). Unknown and duplicate keys are rejected by studioDecodeRaw.
func pickListSelection(w http.ResponseWriter, r *http.Request) (merchantorders.PickListSelection, bool) {
	var sel merchantorders.PickListSelection
	in, _, ok := studioDecodeRaw[pickListBody](w, r, []string{"order_ids", "session_id"})
	if !ok {
		return sel, false
	}
	hasOrders, hasSession := in.OrderIDs != nil, in.SessionID != nil
	if hasOrders == hasSession {
		respondError(w, http.StatusUnprocessableEntity, "invalid_request")
		return sel, false
	}
	if hasOrders {
		if len(in.OrderIDs) == 0 {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return sel, false
		}
		sel.OrderIDs = in.OrderIDs
		return sel, true
	}
	if *in.SessionID == "" {
		respondError(w, http.StatusUnprocessableEntity, "invalid_request")
		return sel, false
	}
	sel.SessionID = *in.SessionID
	return sel, true
}

// exportTemplate accepts exactly ?template=<black_cat|hsinchu|chunghwa_post|generic> and nothing else.
func exportTemplate(u *url.URL) (string, bool) {
	if u.ForceQuery || u.RawQuery == "" || len(u.RawQuery) > 64 {
		return "", false
	}
	values, err := url.ParseQuery(u.RawQuery)
	if err != nil || len(values) != 1 || len(values["template"]) != 1 {
		return "", false
	}
	template := values["template"][0]
	if !merchantorders.ValidCarrierTemplate(template) {
		return "", false
	}
	return template, true
}

// pickListScoped runs fn in one platform.WithScope transaction; on failure it has already written the
// classified error. ok=true means COMMIT was acknowledged.
func pickListScoped(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool, permission string, fn action) (any, bool) {
	if !canonicalBearer(r) {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return nil, false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	var result any
	err := platform.WithScope(ctx, pool, bearerToken(r), r.PathValue("store_id"), permission, func(tx pgx.Tx, s platform.Scope) error {
		var inner error
		result, inner = fn(ctx, tx, s, r)
		return inner
	})
	if err != nil {
		status, code := picklistClassify(err)
		respondError(w, status, code)
		return nil, false
	}
	return result, true
}

// picklistClassify maps the pick-list refusal (422 too_many) then the shared table (deadlock -> 503,
// unclassified -> 503 unavailable). No driver message is ever returned.
func picklistClassify(err error) (int, string) {
	if errors.Is(err, merchantorders.ErrPickListTooMany) {
		return http.StatusUnprocessableEntity, "too_many"
	}
	return claimsClassify(err)
}

// writeCarrierExport sends the CSV: attachment, non-cacheable, UTF-8 BOM already in Body. Nothing is
// stored, logged or cached (MD9).
func writeCarrierExport(w http.ResponseWriter, store, template string, export merchantorders.CarrierExportFile, now time.Time) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+template+`-`+store[:8]+`-`+now.UTC().Format("200601021504")+`.csv"`)
	w.Header().Set("Cache-Control", "no-store, private")
	w.Header().Set("Content-Length", strconv.Itoa(len(export.Body)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(export.Body)
}
