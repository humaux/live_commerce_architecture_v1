package httpapi

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/merchantorders"
	"livecommerce/internal/platform"
)

func orderAmendmentRoute(pool *pgxpool.Pool, method string) http.HandlerFunc {
	return shipmentRoute(method, func(w http.ResponseWriter, r *http.Request) {
		fields := []string{"expected_revision", "recipient", "additions"}
		keys := r.Header.Values("Idempotency-Key")
		var key string
		if method == http.MethodPut {
			if len(keys) != 1 || !claimsKey.MatchString(keys[0]) {
				respondError(w, http.StatusUnprocessableEntity, "invalid_request")
				return
			}
			key = keys[0]
			fields = append(fields, "proposal")
		} else if len(keys) != 0 {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		in, _, ok := studioDecodeRaw[merchantorders.AmendmentInput](w, r, fields)
		if !ok {
			return
		}
		result, ok := shipmentScope(w, r, pool, "orders:read", func(ctx context.Context, tx pgx.Tx, scope platform.Scope, r *http.Request) (any, error) {
			return merchantorders.AmendHomeCOD(ctx, tx, scope, bearerToken(r), r.PathValue("order_id"), key, in)
		})
		if ok {
			respond(w, http.StatusOK, result)
		}
	})
}

func orderDocumentRoute(pool *pgxpool.Pool) http.HandlerFunc {
	return shipmentRoute(http.MethodPost, func(w http.ResponseWriter, r *http.Request) {
		if len(r.Header.Values("Idempotency-Key")) != 0 {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		in, _, ok := studioDecodeRaw[struct {
			Revision int64 `json:"expected_revision"`
		}](w, r, []string{"expected_revision"})
		if !ok {
			return
		}
		result, ok := shipmentScope(w, r, pool, "orders:read", func(ctx context.Context, tx pgx.Tx, scope platform.Scope, r *http.Request) (any, error) {
			return merchantorders.ReadOrderDocument(ctx, tx, scope, bearerToken(r), r.PathValue("order_id"), in.Revision)
		})
		if ok {
			respond(w, http.StatusOK, result)
		}
	})
}
