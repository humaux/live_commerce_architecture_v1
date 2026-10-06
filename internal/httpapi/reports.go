// reports.go owns the W6-02B merchant report HTTP adapter (contracts/reporting-v2.md): GET reports/{products|channels|funnel|manual-orders}
// ?from&to[&session_id] (orders:read; funnel also live:read) and the same paths with a .csv suffix (orders:export, audited reports.export.<report>).
// The admin BFF mirrors them under /api/admin/.
//
// Purpose: transport rules only (method, exact query, range, ids) before any database work, then one commerce_runtime READ COMMITTED transaction
// with a 60 s budget per request; every response is private and non-cacheable.
// Depends on: internal/reporting (Products/ChannelsReport/Funnel/ManualOrders and their CSVs), platform.WithScopeBudget via withToolsScope,
//   customersClassify/customerRoute (customers.go) for errors and the transport gate, writeAttachment.
// Used by: NewHandler (handler.go), reports_test.go, tests/foundation/reports_test.go.
// Invariants: I01 (store from the path, authorised by the bearer's grants, never trusted), range 0..91 days (422 invalid_request otherwise).
// Status: REAL_PG (MOCK data).

package httpapi

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
	"livecommerce/internal/reporting"
)

// reportBudget is the whole-transaction budget of one report read (W6-02B brief: 60 s).
const reportBudget = 60 * time.Second

// registerReportRoutes mounts the eight report rows; NewHandler calls it unconditionally (pool only, no worker or provider). environment is the deployment
// payment environment (Options.PaymentEnvironment, SANDBOX by default): the order counts of the reports follow it, money stays split per environment.
func registerReportRoutes(mux *http.ServeMux, pool *pgxpool.Pool, environment string) {
	const base = "/v1/admin/stores/{store_id}/reports/"
	type page struct {
		slug    string
		session bool // only the funnel takes session_id
		read    func(ctx context.Context, tx pgx.Tx, s platform.Scope, token, from, to, session string) (any, error)
		export  func(ctx context.Context, tx pgx.Tx, s platform.Scope, token, from, to, session string) ([]byte, error)
	}
	for _, p := range []page{
		{"products", false,
			func(ctx context.Context, tx pgx.Tx, s platform.Scope, token, from, to, _ string) (any, error) {
				return reporting.Products(ctx, tx, s, token, from, to)
			},
			func(ctx context.Context, tx pgx.Tx, s platform.Scope, token, from, to, _ string) ([]byte, error) {
				return reporting.ProductsCSV(ctx, tx, s, token, from, to)
			}},
		{"channels", false,
			func(ctx context.Context, tx pgx.Tx, s platform.Scope, token, from, to, _ string) (any, error) {
				return reporting.ChannelsReport(ctx, tx, s, token, from, to, environment)
			},
			func(ctx context.Context, tx pgx.Tx, s platform.Scope, token, from, to, _ string) ([]byte, error) {
				return reporting.ChannelsCSV(ctx, tx, s, token, from, to, environment)
			}},
		{"funnel", true,
			func(ctx context.Context, tx pgx.Tx, s platform.Scope, token, from, to, session string) (any, error) {
				return reporting.Funnel(ctx, tx, s, token, from, to, session, environment)
			},
			func(ctx context.Context, tx pgx.Tx, s platform.Scope, token, from, to, session string) ([]byte, error) {
				return reporting.FunnelCSV(ctx, tx, s, token, from, to, session, environment)
			}},
		{"manual-orders", false,
			func(ctx context.Context, tx pgx.Tx, s platform.Scope, token, from, to, _ string) (any, error) {
				return reporting.ManualOrders(ctx, tx, s, token, from, to, environment)
			},
			func(ctx context.Context, tx pgx.Tx, s platform.Scope, token, from, to, _ string) ([]byte, error) {
				return reporting.ManualOrdersCSV(ctx, tx, s, token, from, to, environment)
			}},
	} {
		mux.HandleFunc("GET "+base+p.slug, customerRoute(http.MethodGet, true, func(w http.ResponseWriter, r *http.Request) {
			from, to, session, err := parseReportQuery(r.URL, p.session)
			if err != nil {
				respondError(w, http.StatusUnprocessableEntity, "invalid_request")
				return
			}
			var result any
			if reportRun(w, r, pool, "orders:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope) (err error) {
				result, err = p.read(ctx, tx, s, bearerToken(r), from, to, session)
				return err
			}) {
				respond(w, http.StatusOK, result)
			}
		}))
		mux.HandleFunc("GET "+base+p.slug+".csv", customerRoute(http.MethodGet, true, func(w http.ResponseWriter, r *http.Request) {
			from, to, session, err := parseReportQuery(r.URL, p.session)
			if err != nil {
				respondError(w, http.StatusUnprocessableEntity, "invalid_request")
				return
			}
			var body []byte
			if reportRun(w, r, pool, "orders:export", func(ctx context.Context, tx pgx.Tx, s platform.Scope) (err error) {
				body, err = p.export(ctx, tx, s, bearerToken(r), from, to, session)
				return err
			}) {
				// only after COMMIT was acknowledged (the export audit row is durable)
				writeAttachment(w, "text/csv; charset=utf-8", `attachment; filename="report-`+strings.ReplaceAll(p.slug, "-", "_")+`-`+from+`-`+to+`.csv"`, body)
			}
		}))
	}
}

// reportRun runs fn in one platform.WithScopeBudget transaction; on failure it has already written the classified error.
func reportRun(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool, permission string, fn func(context.Context, pgx.Tx, platform.Scope) error) bool {
	if !canonicalBearer(r) {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return false
	}
	if err := withToolsScope(r, pool, permission, reportBudget, fn); err != nil {
		status, code := customersClassify(err)
		respondError(w, status, code)
		return false
	}
	return true
}

// parseReportQuery requires from and to once each as valid dates within the range bound, plus (funnel only) an optional session_id.
func parseReportQuery(u *url.URL, allowSession bool) (from, to, session string, err error) {
	if u.ForceQuery || len(u.RawQuery) > 512 || u.RawQuery == "" {
		return "", "", "", command.ErrInvalid
	}
	for _, field := range strings.Split(u.RawQuery, "&") {
		if field == "" || !strings.Contains(field, "=") {
			return "", "", "", command.ErrInvalid
		}
	}
	values, perr := url.ParseQuery(u.RawQuery)
	if perr != nil || len(values["from"]) != 1 || len(values["to"]) != 1 {
		return "", "", "", command.ErrInvalid
	}
	want := 2
	if ids, ok := values["session_id"]; ok {
		if !allowSession || len(ids) != 1 || !command.ValidID(ids[0]) {
			return "", "", "", command.ErrInvalid
		}
		session, want = ids[0], 3
	}
	if len(values) != want {
		return "", "", "", command.ErrInvalid
	}
	from, to = values["from"][0], values["to"][0]
	if _, _, rerr := reporting.ParseRange(from, to); rerr != nil {
		return "", "", "", rerr
	}
	return from, to, session, nil
}
