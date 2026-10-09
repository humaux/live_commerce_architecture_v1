// Purpose: W3-U2 store-scoped GET/PUT sold-out reply settings; a thin merchant HTTP adapter with strict JSON and private responses.
// Depends on: claims.GetSoldOutReply/SetSoldOutReply (migration 0151), msgtemplates.Resolve (0121), command.Run and platform.WithScope.
// Used by: registerClaimRoutes when cmd/api enables Claims; the admin live-settings BFF and foundation HTTP gates.
// Invariants: I01 authenticated scope; I02 durable replay; I04 atomic setting/audit/receipt; I14 CAS; I15 private no-store.
package httpapi

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/claims"
	"livecommerce/internal/command"
	"livecommerce/internal/msgtemplates"
	"livecommerce/internal/platform"
)

// soldOutSettingsInput is the exact PUT wire and receipt digest; absent/null fields are rejected before scope open.
type soldOutSettingsInput struct {
	Enabled         bool   `json:"enabled"`
	TemplateID      string `json:"template_id"`
	TemplateVersion int64  `json:"template_version"`
	ExpectedVersion int64  `json:"expected_version"`
}

// registerSoldOutSettingsRoutes runs inside registerClaimRoutes' Claims-enabled boundary; it performs no external send.
func registerSoldOutSettingsRoutes(mux *http.ServeMux, pool *pgxpool.Pool) {
	const base = "/v1/admin/stores/{store_id}/live-settings/sold-out-reply"
	templates := msgtemplates.NewService()
	mux.HandleFunc("GET "+base, claimsRoute(http.MethodGet, false, claimsScoped(pool, "live:read",
		func(ctx context.Context, tx pgx.Tx, _ platform.Scope, _ *http.Request) (any, error) {
			// claims.get_sold_out_reply (0151): returns the store setting or its read-only fixed default.
			return claims.GetSoldOutReply(ctx, tx)
		})))
	mux.HandleFunc("PUT "+base, claimsRoute(http.MethodPut, false, func(w http.ResponseWriter, r *http.Request) {
		in, ok := claimsBody[soldOutSettingsInput](w, r, []string{"enabled", "template_id", "template_version", "expected_version"}, nil)
		if !ok {
			return
		}
		if in.TemplateID == "" || in.TemplateVersion < 1 || in.ExpectedVersion < 0 {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		claimsScoped(pool, "live:manage", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			var out claims.SoldOutReply
			// I02/I04: the enclosing WithScope transaction commits setting, SQL audit and receipt together.
			// Resolve/CAS are inside the callback: an original successful request replays even after its CAS becomes stale.
			err := command.Run(ctx, tx, s, "claims.sold_out_reply.set", r.Header.Get("Idempotency-Key"), in, &out, func() error {
				// msgtemplates.resolve (0121): tenant/store and publish version are verified by the existing definer.
				resolved, err := templates.Resolve(ctx, tx, in.TemplateID, in.TemplateVersion)
				if err != nil {
					return err
				}
				if resolved.Fixed {
					return command.ErrInvalid
				} // W3-U2 ruling: fixed defaults may be read, never selected by PUT.
				// claims.set_sold_out_reply (0151): owns kind/body/placeholder validation, version CAS and audit.
				out, err = claims.SetSoldOutReply(ctx, tx, in.Enabled, in.TemplateID, in.TemplateVersion, in.ExpectedVersion)
				return err
			})
			return out, err
		})(w, r)
	}))
	mux.HandleFunc(base, studioRoute("", false, nil))
}
