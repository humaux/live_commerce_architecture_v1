// merchanttools.go owns the merchant HTTP adapter of contracts/storefront-v2.md section G: GET dashboard, GET products/export.csv,
// POST products/import/{preview,commit} and GET orders/manual/options, POST orders/manual. Each handler is the Go endpoint; the admin BFF
// (apps/admin/app/api/stores/[store]/tools/[...resource]/route.ts) mirrors them under /api/stores/{store}/tools/.
//
// Non-goals: no rule of its own (internal/merchanttools decides; the SQL definers of migration 0094 and internal/catalog / inventory /
// checkout decide below it), no tenant or store from the request (platform.WithScope resolves both from the bearer), no query string, no
// body on a read, no driver text in an error (a coded refusal returns only its contract code), no file kept (the CSV is read once into
// memory, applied or rolled back, and dropped).
// Budgets: the import runs in ONE transaction with a 60 s budget (platform.WithScopeBudget); the handler extends the connection's read and
// write deadlines for that request only (http.ResponseController), because cmd/api's server-wide WriteTimeout is 15 s.

package httpapi

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/merchanttools"
	"livecommerce/internal/platform"
)

var manualOrderFields = []string{"items", "customer", "delivery", "payment_mode", "locale"}
var manualRegenerateFields = []string{"order_id", "locale"}

const importBudget = 60 * time.Second

func registerMerchantToolsRoutes(mux *http.ServeMux, pool *pgxpool.Pool, manual *merchanttools.ManualOrders) {
	const base = "/v1/admin/stores/{store_id}"
	mux.HandleFunc("GET "+base+"/dashboard", cvsRoute(http.MethodGet, false, func(w http.ResponseWriter, r *http.Request) {
		toolsServe(w, r, pool, "orders:read", 0, http.StatusOK, func(ctx context.Context, tx pgx.Tx, s platform.Scope) (any, error) {
			return merchanttools.Dashboard(ctx, tx, s, bearerToken(r))
		})
	}))
	mux.HandleFunc("GET "+base+"/products/export.csv", cvsRoute(http.MethodGet, false, func(w http.ResponseWriter, r *http.Request) {
		var csv []byte
		ok := toolsRun(w, r, pool, "catalog:read", 0, func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
			if err := platform.RequirePermission(ctx, tx, s, bearerToken(r), "inventory:read"); err != nil {
				return err
			}
			var err error
			csv, err = merchanttools.ExportProducts(ctx, tx, s)
			return err
		})
		if ok {
			writeAttachment(w, "text/csv; charset=utf-8", `attachment; filename="products-`+time.Now().UTC().Format("2006-01-02")+`.csv"`, csv)
		}
	}))
	for _, mode := range []struct {
		path   string
		commit bool
	}{{"/products/import/preview", false}, {"/products/import/commit", true}} {
		mux.HandleFunc("POST "+base+mode.path, cvsRoute(http.MethodPost, false, func(w http.ResponseWriter, r *http.Request) {
			media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || media != "text/csv" {
				respondError(w, http.StatusUnsupportedMediaType, "invalid_request")
				return
			}
			// Per-request deadlines: the 2 MiB body may be slow, the all-or-nothing apply may take most of importBudget.
			control := http.NewResponseController(w)
			_ = control.SetReadDeadline(time.Now().Add(30 * time.Second))
			_ = control.SetWriteDeadline(time.Now().Add(importBudget + 15*time.Second))
			r.Body = http.MaxBytesReader(w, r.Body, merchanttools.MaxCSVBytes)
			data, err := io.ReadAll(r.Body)
			if err != nil || len(data) == 0 {
				respondError(w, http.StatusRequestEntityTooLarge, "invalid_request")
				return
			}
			var result merchanttools.ImportResult
			err = withToolsScope(r, pool, "catalog:write", importBudget, func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
				if err := platform.RequirePermission(ctx, tx, s, bearerToken(r), "inventory:write"); err != nil {
					return err
				}
				var inner error
				result, inner = merchanttools.ImportProducts(ctx, tx, s, data, mode.commit)
				return inner
			})
			switch {
			case err == nil || errors.Is(err, merchanttools.ErrPreviewRolledBack):
				respond(w, http.StatusOK, result) // a preview answers 200 even with row errors; its transaction rolled back
			case errors.Is(err, merchanttools.ErrImportHasErrors):
				respond(w, http.StatusUnprocessableEntity, result) // commit refused, nothing written; the body lists every row error
			default:
				status, code := toolsClassify(err)
				respondError(w, status, code)
			}
		}))
	}
	mux.HandleFunc("GET "+base+"/orders/manual/options", cvsRoute(http.MethodGet, false, func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cancel()
		options, err := manual.Options(ctx, bearerToken(r), r.PathValue("store_id"))
		if err != nil {
			status, code := toolsClassify(err)
			respondError(w, status, code)
			return
		}
		respond(w, http.StatusOK, map[string]any{"options": options})
	}))
	mux.HandleFunc("POST "+base+"/orders/manual", cvsRoute(http.MethodPost, true, func(w http.ResponseWriter, r *http.Request) {
		in, ok := cvsStrictBody[merchanttools.ManualInput](w, r, manualOrderFields, nil)
		if !ok {
			return
		}
		// The pipeline is several transactions on three pools (each bounded to 5 s); the whole request gets 14 s inside the 15 s server cap.
		ctx, cancel := context.WithTimeout(r.Context(), 14*time.Second)
		defer cancel()
		result, replayed, err := manual.Place(ctx, bearerToken(r), r.PathValue("store_id"), r.Header.Get("Idempotency-Key"), in)
		if err != nil {
			status, code := toolsClassify(err)
			respondError(w, status, code)
			return
		}
		status := http.StatusCreated
		if replayed {
			status = http.StatusOK
		}
		respond(w, status, result)
	}))
	mux.HandleFunc("POST "+base+"/orders/manual/regenerate-link", cvsRoute(http.MethodPost, true, func(w http.ResponseWriter, r *http.Request) {
		in, ok := cvsStrictBody[merchanttools.ManualRegenerateInput](w, r, manualRegenerateFields, nil)
		if !ok {
			return
		}
		// One merchant transaction (regenerate is a single SQL definer call + audit); the whole request gets 14 s inside the 15 s server cap.
		ctx, cancel := context.WithTimeout(r.Context(), 14*time.Second)
		defer cancel()
		result, replayed, err := manual.RegenerateLink(ctx, bearerToken(r), r.PathValue("store_id"), r.Header.Get("Idempotency-Key"), in)
		if err != nil {
			status, code := toolsClassify(err)
			respondError(w, status, code)
			return
		}
		status := http.StatusCreated
		if replayed {
			status = http.StatusOK
		}
		respond(w, status, result)
	}))
}

// toolsServe runs fn in one platform.WithScope transaction (budget 0 = the default 5 s) and answers its result or the classified refusal.
func toolsServe(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool, permission string, budget time.Duration, success int, fn func(context.Context, pgx.Tx, platform.Scope) (any, error)) {
	var result any
	if toolsRun(w, r, pool, permission, budget, func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
		var err error
		result, err = fn(ctx, tx, s)
		return err
	}) {
		respond(w, success, result)
	}
}

func toolsRun(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool, permission string, budget time.Duration, fn func(context.Context, pgx.Tx, platform.Scope) error) bool {
	if err := withToolsScope(r, pool, permission, budget, fn); err != nil {
		status, code := toolsClassify(err)
		respondError(w, status, code)
		return false
	}
	return true
}

// withToolsScope is platform.WithScope (or WithScopeBudget for the import) with the request's bearer and store.
func withToolsScope(r *http.Request, pool *pgxpool.Pool, permission string, budget time.Duration, fn func(context.Context, pgx.Tx, platform.Scope) error) error {
	timeout := 6 * time.Second
	if budget > 0 {
		timeout = budget + 5*time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	run := func(tx pgx.Tx, s platform.Scope) error { return fn(ctx, tx, s) }
	if budget > 0 {
		return platform.WithScopeBudget(ctx, pool, bearerToken(r), r.PathValue("store_id"), permission, budget, run)
	}
	return platform.WithScope(ctx, pool, bearerToken(r), r.PathValue("store_id"), permission, run)
}

// toolsClassify maps a merchanttools coded refusal first, then the shared claims table (authority, not found, conflict, deadline).
func toolsClassify(err error) (int, string) {
	var coded *merchanttools.Error
	if errors.As(err, &coded) {
		return coded.Status, coded.Code
	}
	return claimsClassify(err)
}
