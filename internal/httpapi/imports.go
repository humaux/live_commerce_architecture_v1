// Purpose: the merchant import HTTP adapter of contracts/migration-import-v1.md (unit W5-02B), under
//   /v1/admin/stores/{store_id}/imports: POST customers/preview, POST customers/commit?expected_apply_rows=N and
//   GET {batch_id}/results.csv[?only=failed], plus (W5-03B) POST orders/preview and POST orders/commit for the historical-order archive.
//   It decides no rule: internal/migrationimport and the migration 0152 / 0156 definers do.
// Depends on: internal/migrationimport (CustomersPreview, CustomersCommit, OrdersPreview, OrdersCommit, ResultsCSV, ParseMapping), merchanttools.go helpers
//   (trackingImportRoute, withToolsScope, toolsClassify, importBudget), customers.go writeAttachment.
// Used by: handler.go NewHandler (mounted unconditionally, like the customer rows); pinned by imports_test.go (DB-free full router).
// Invariants: authority is customers:privacy (imported rows are PII); the tenant and store come from the bearer; no Idempotency-Key
//   (the file hash is the idempotency key); the uploaded file is read once into memory and dropped; no cell, name, phone or email in
//   any response error, log line or audit row. Every response is private and non-cacheable.

package httpapi

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/migrationimport"
	"livecommerce/internal/platform"
)

// registerImportRoutes mounts the three customer-import rows and the two order-import rows. The literal "customers" segment and the {batch_id} wildcard never
// overlap (different method and second literal); the DB-free full-router test fails at registration on any conflict.
func registerImportRoutes(mux *http.ServeMux, pool *pgxpool.Pool) {
	const base = "/v1/admin/stores/{store_id}/imports"
	// POST preview: dry-run one CSV (optional ?mapping=JSON); the transaction always rolls back, nothing is written.
	mux.HandleFunc("POST "+base+"/customers/preview", trackingImportRoute(http.MethodPost, true, func(w http.ResponseWriter, r *http.Request) {
		q, ok := importQuery(r, "mapping")
		if !ok {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		mapping, err := migrationimport.ParseMapping(q.Get("mapping"))
		if err != nil {
			status, code := importClassify(err)
			respondError(w, status, code)
			return
		}
		importCSV(w, r, pool, func(ctx context.Context, tx pgx.Tx, s platform.Scope, data []byte) (any, error) {
			return migrationimport.CustomersPreview(ctx, tx, s, bearerToken(r), data, mapping)
		})
	}))
	// POST commit?expected_apply_rows=N[&mapping=JSON]: apply the previewed file once (idempotent per file hash).
	mux.HandleFunc("POST "+base+"/customers/commit", trackingImportRoute(http.MethodPost, true, func(w http.ResponseWriter, r *http.Request) {
		q, ok := importQuery(r, "expected_apply_rows", "mapping")
		expected, perr := strconv.ParseInt(q.Get("expected_apply_rows"), 10, 64)
		if !ok || q.Get("expected_apply_rows") == "" || perr != nil || expected < 0 || expected > migrationimport.MaxRows {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		mapping, err := migrationimport.ParseMapping(q.Get("mapping"))
		if err != nil {
			status, code := importClassify(err)
			respondError(w, status, code)
			return
		}
		importCSV(w, r, pool, func(ctx context.Context, tx pgx.Tx, s platform.Scope, data []byte) (any, error) {
			return migrationimport.CustomersCommit(ctx, tx, s, bearerToken(r), data, mapping, expected)
		})
	}))
	// POST orders/preview and orders/commit (W5-03B): the same two rows for the historical-order CSV (a read-only archive attached to
	// already-imported customers); same query grammar, same file-hash idempotency, same 60 s budget.
	mux.HandleFunc("POST "+base+"/orders/preview", trackingImportRoute(http.MethodPost, true, func(w http.ResponseWriter, r *http.Request) {
		q, ok := importQuery(r, "mapping")
		if !ok {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		mapping, err := migrationimport.ParseMapping(q.Get("mapping"))
		if err != nil {
			status, code := importClassify(err)
			respondError(w, status, code)
			return
		}
		importCSV(w, r, pool, func(ctx context.Context, tx pgx.Tx, s platform.Scope, data []byte) (any, error) {
			return migrationimport.OrdersPreview(ctx, tx, s, bearerToken(r), data, mapping)
		})
	}))
	mux.HandleFunc("POST "+base+"/orders/commit", trackingImportRoute(http.MethodPost, true, func(w http.ResponseWriter, r *http.Request) {
		q, ok := importQuery(r, "expected_apply_rows", "mapping")
		expected, perr := strconv.ParseInt(q.Get("expected_apply_rows"), 10, 64)
		if !ok || q.Get("expected_apply_rows") == "" || perr != nil || expected < 0 || expected > migrationimport.MaxRows {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		mapping, err := migrationimport.ParseMapping(q.Get("mapping"))
		if err != nil {
			status, code := importClassify(err)
			respondError(w, status, code)
			return
		}
		importCSV(w, r, pool, func(ctx context.Context, tx pgx.Tx, s platform.Scope, data []byte) (any, error) {
			return migrationimport.OrdersCommit(ctx, tx, s, bearerToken(r), data, mapping, expected)
		})
	}))
	// GET results.csv[?only=failed]: the downloadable per-row result of one committed batch (no name, phone or email in it).
	mux.HandleFunc("GET "+base+"/{batch_id}/results.csv", trackingImportRoute(http.MethodGet, true, func(w http.ResponseWriter, r *http.Request) {
		q, ok := importQuery(r, "only")
		if v := q.Get("only"); !ok || (len(q) == 1 && v != "failed") {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		var csv []byte
		err := withToolsScope(r, pool, "customers:privacy", 0, func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
			var inner error
			csv, inner = migrationimport.ResultsCSV(ctx, tx, s, bearerToken(r), r.PathValue("batch_id"), q.Get("only") == "failed")
			return inner
		})
		if err != nil {
			status, code := importClassify(err)
			respondError(w, status, code)
			return
		}
		writeAttachment(w, "text/csv; charset=utf-8", `attachment; filename="customer-import-results.csv"`, csv)
	}))
}

// importQuery parses the query strictly: only the allowed keys, each at most once with one non-empty value. No query at all is fine
// (an empty result). ok=false means an unknown, repeated, empty or malformed key.
func importQuery(r *http.Request, allowed ...string) (url.Values, bool) {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || len(r.URL.RawQuery) > 4096 {
		return nil, false
	}
	for name, got := range values {
		known := false
		for _, a := range allowed {
			known = known || a == name
		}
		if !known || len(got) != 1 || got[0] == "" {
			return nil, false
		}
	}
	return values, true
}

// importCSV reads one 2 MiB text/csv body and runs fn in one 60 s WithScopeBudget transaction (customers:privacy): the preview always
// rolls back, the commit keeps only a matching preview. A stale commit answers 409 preview_stale with the fresh preview.
func importCSV(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool, fn func(context.Context, pgx.Tx, platform.Scope, []byte) (any, error)) {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "text/csv" {
		respondError(w, http.StatusUnsupportedMediaType, "invalid_request")
		return
	}
	// Per-request deadlines: the 2 MiB body may be slow, the all-or-nothing apply may take most of importBudget.
	control := http.NewResponseController(w)
	_ = control.SetReadDeadline(time.Now().Add(30 * time.Second))
	_ = control.SetWriteDeadline(time.Now().Add(importBudget + 15*time.Second))
	r.Body = http.MaxBytesReader(w, r.Body, migrationimport.MaxCSVBytes)
	data, err := io.ReadAll(r.Body)
	if err != nil || len(data) == 0 {
		respondError(w, http.StatusRequestEntityTooLarge, "invalid_request")
		return
	}
	var result any
	err = withToolsScope(r, pool, "customers:privacy", importBudget, func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
		var inner error
		result, inner = fn(ctx, tx, s, data)
		return inner
	})
	switch {
	case err == nil || errors.Is(err, migrationimport.ErrPreviewRolledBack):
		respond(w, http.StatusOK, result) // a preview answers 200 even with row failures; its transaction rolled back
	default:
		var stale *migrationimport.PreviewStaleError
		if errors.As(err, &stale) {
			respond(w, http.StatusConflict, stale.Preview) // 409 preview_stale, body = the fresh preview
			return
		}
		status, code := importClassify(err)
		respondError(w, status, code)
	}
}

// importClassify maps a migrationimport coded refusal first, then the shared merchant-tools / claims table.
func importClassify(err error) (int, string) {
	var coded *migrationimport.Error
	if errors.As(err, &coded) {
		return coded.Status, coded.Code
	}
	return toolsClassify(err)
}
