// Purpose: the merchant message-template HTTP adapter (contracts/live-console-v1.md §11 /message-templates, unit
// W2-05B) under /v1/admin/stores/{store_id}/message-templates: POST publish (live:manage, idempotent) and GET list
// (inbox:reply). It decides no rule (internal/msgtemplates and the 0121 SECURITY DEFINER functions do), never returns
// a driver message, never logs a body, and keeps the private no-store response boundary.
// Depends on: livecommerce/internal/msgtemplates, livecommerce/internal/platform (WithScope), and the 0121 definers.
// Used by: internal/httpapi/handler.go (registerTemplateRoutes, gated on Options.MsgTemplates); cmd/api builds the service.

package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/msgtemplates"
	"livecommerce/internal/platform"
)

// registerTemplateRoutes mounts POST/GET /message-templates when svc is non-nil; nil leaves them unmounted, exactly
// like every other nil-able service in Options.
func registerTemplateRoutes(mux *http.ServeMux, pool *pgxpool.Pool, svc *msgtemplates.Service) {
	if svc == nil {
		return
	}
	const base = "/v1/admin/stores/{store_id}/message-templates"

	// Publish (live:manage): idempotent, appends the next version. The five keys are all required; null is rejected
	// (claimsBody), so public_safe never decodes from an absent/null boolean.
	mux.HandleFunc("POST "+base, templateRoute(http.MethodPost, true, func(w http.ResponseWriter, r *http.Request) {
		in, ok := claimsBody[msgtemplates.PublishInput](w, r, []string{"template_id", "name", "kinds", "public_safe", "body"}, nil)
		if !ok {
			return
		}
		templatesScoped(pool, "live:manage", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return svc.Publish(ctx, tx, s, r.Header.Get("Idempotency-Key"), in)
		})(w, r)
	}))

	// List (inbox:reply): the latest published version of each template_id, scoped by the definer.
	mux.HandleFunc("GET "+base, templateRoute(http.MethodGet, false, func(w http.ResponseWriter, r *http.Request) {
		templatesScoped(pool, "inbox:reply", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return svc.List(ctx, tx)
		})(w, r)
	}))

	// Methodless fallback keeps 405 inside the same private no-store boundary.
	mux.HandleFunc(base, studioRoute("", false, nil))
}

// templateRoute is studioRoute (private no-store, method, query and GET-body rules) plus the idempotency receipt rule:
// Idempotency-Key exactly once on the write and forbidden (even empty) on the read.
func templateRoute(method string, keyed bool, next http.HandlerFunc) http.HandlerFunc {
	return studioRoute(method, false, func(w http.ResponseWriter, r *http.Request) {
		keys := r.Header.Values("Idempotency-Key")
		if (keyed && (len(keys) != 1 || !claimsKey.MatchString(keys[0]))) || (!keyed && len(keys) != 0) {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		next(w, r)
	})
}

func templatesScoped(pool *pgxpool.Pool, permission string, fn action) http.HandlerFunc {
	return scopedAs(pool, permission, templatesClassify, fn)
}

// templatesClassify maps the fixed definer codes (PT403 forbidden, PT409 template_fixed for the fixed ids) and the
// §3.5 coded refusal to their transport codes, then falls back to the shared table.
func templatesClassify(err error) (int, string) {
	var coded *msgtemplates.Error
	if errors.As(err, &coded) {
		return coded.Status, coded.Code
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "PT403":
			return http.StatusForbidden, "forbidden"
		case "PT404":
			return http.StatusNotFound, "not_found"
		case "PT409":
			return http.StatusConflict, "template_fixed"
		case "PT422":
			return http.StatusUnprocessableEntity, "invalid_request"
		}
	}
	if msgtemplates.IsNotFound(err) {
		return http.StatusNotFound, "not_found"
	}
	return classify(err)
}
