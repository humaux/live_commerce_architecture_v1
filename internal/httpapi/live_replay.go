// Purpose: private merchant replay GET/PUT with closed bodies and coded publication refusals.
// Depends on: live.GetReplay/SetReplay, claimsRoute, scopedAs and lifecycleClassify.
// Used by: registerLiveFlowRoutes when Studio/Live is enabled.
// Invariants: live:read/live:manage; authenticated store scope; no-store; unknown authority keys refused.
package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/live"
	"livecommerce/internal/platform"
)

func registerLiveReplayRoutes(mux *http.ServeMux, pool *pgxpool.Pool) {
	const path = "/v1/admin/stores/{store_id}/live-sessions/{session_id}/replay"
	mux.HandleFunc("GET "+path, replayReadRoute(claimsScoped(pool, "live:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
		return live.GetReplay(ctx, tx, s, bearerToken(r), r.PathValue("session_id"))
	})))
	mux.HandleFunc("PUT "+path, claimsRoute(http.MethodPut, false, func(w http.ResponseWriter, r *http.Request) {
		in, ok := claimsBody[live.ReplayInput](w, r, []string{"public", "expected_version"}, nil)
		if !ok {
			return
		}
		scopedAs(pool, "live:manage", replayClassify, func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return live.SetReplay(ctx, tx, s, bearerToken(r), r.Header.Get("Idempotency-Key"), r.PathValue("session_id"), in)
		})(w, r)
	}))
	mux.HandleFunc(path, studioRoute("", false, nil))
}

// replayReadRoute accepts an empty reader as an absent body, including in-process
// callers. Prove EOF with a bounded read; never infer emptiness from ContentLength.
// The existing claims guard still owns query, framing, key and UUID validation.
func replayReadRoute(next http.HandlerFunc) http.HandlerFunc {
	read := claimsRoute(http.MethodGet, false, next)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, no-store")
		if r.ContentLength == 0 && len(r.TransferEncoding) == 0 && r.Body != nil && r.Body != http.NoBody {
			body, err := io.ReadAll(io.LimitReader(r.Body, 1))
			if err != nil || len(body) != 0 {
				respondError(w, http.StatusUnprocessableEntity, "invalid_request")
				return
			}
			_ = r.Body.Close()
			r.Body = http.NoBody
		}
		read(w, r)
	}
}

func replayClassify(err error) (int, string) {
	if errors.Is(err, live.ErrNoOfficialVideo) {
		return http.StatusConflict, "no_official_video"
	}
	return lifecycleClassify(err)
}
