// Purpose: the merchant restricted-buyer list routes (W3-05B): list, add, remove and a per-bundle "is restricted" check, under the claims route family of a live session.
// Depends on: internal/claims (BlockActor/UnblockActor/ListBlockedActors/BlockedForBundle; SQL definers of migration 0154), claims.go helpers (claimsRoute, claimsBody, claimsScoped), studioPage, platform.WithScope via scopedAs.
// Used by: registerClaimRoutes (claims.go), mounted only when the claims routes are (COMMERCE_CLAIMS_ENABLED); the admin BFF forwards them unchanged.
// Invariants: the request never carries an actor_key (a body with any key other than comment_ref / conversation_id / bundle_id / note is 400 invalid_json); exactly one reference per add (422);
//   live:manage for add/remove, live:read for list/check; no note, actor key or bearer in a log or an error body.
//
// Routes (base = /v1/admin/stores/{store_id}/live-sessions/{session_id}/claims):
//   GET    base/blocklist?limit&cursor      list newest first            -> {items:[{id,platform,note,source_bundle_id,created_at}], next_cursor}
//   POST   base/blocklist                   add by {comment_ref|conversation_id|bundle_id, note?} -> {id,platform,created_at,created}
//   DELETE base/blocklist/entries/{id}      remove one entry             -> {removed:true}
//   GET    base/blocklist/check?bundle_id=  -> {restricted:bool}         (never the note)

package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/claims"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// blocklistBodyFields: exactly one of the three references is required; note is the only optional key.
var blocklistRefFields = []string{"comment_ref", "conversation_id", "bundle_id"}

// registerBlocklistRoutes mounts the four blocklist routes on the claims base path. Called once by registerClaimRoutes.
func registerBlocklistRoutes(mux *http.ServeMux, pool *pgxpool.Pool, base string) {
	list := base + "/blocklist"
	check := base + "/blocklist/check"
	entry := base + "/blocklist/entries/{entry_id}"
	mux.HandleFunc("GET "+list, claimsRoute(http.MethodGet, true, func(w http.ResponseWriter, r *http.Request) {
		page, err := studioPage(r.URL)
		if err != nil {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		scopedAs(pool, "live:read", blocklistClassify, func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return claims.ListBlockedActors(ctx, tx, s, bearerToken(r), page)
		})(w, r)
	}))
	mux.HandleFunc("POST "+list, claimsRoute(http.MethodPost, false, func(w http.ResponseWriter, r *http.Request) {
		in, ok := claimsBody[claims.BlockInput](w, r, nil, blocklistRefFields, "note")
		if !ok {
			return
		}
		scopedAs(pool, "live:manage", blocklistClassify, func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return claims.BlockActor(ctx, tx, s, bearerToken(r), r.Header.Get("Idempotency-Key"), r.PathValue("session_id"), in)
		})(w, r)
	}))
	mux.HandleFunc("DELETE "+entry, claimsRoute(http.MethodDelete, false, func(w http.ResponseWriter, r *http.Request) {
		if !command.ValidID(r.PathValue("entry_id")) {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		scopedAs(pool, "live:manage", blocklistClassify, func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return claims.UnblockActor(ctx, tx, s, bearerToken(r), r.Header.Get("Idempotency-Key"), r.PathValue("entry_id"))
		})(w, r)
	}))
	mux.HandleFunc("GET "+check, claimsRoute(http.MethodGet, true, func(w http.ResponseWriter, r *http.Request) {
		bundle, ok := blocklistCheckQuery(r.URL)
		if !ok {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		scopedAs(pool, "live:read", blocklistClassify, func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			restricted, err := claims.BlockedForBundle(ctx, tx, s, bearerToken(r), bundle)
			return map[string]bool{"restricted": restricted}, err
		})(w, r)
	}))
	for _, path := range []string{list, check, entry} { // methodless fallbacks keep 405 inside the private boundary
		mux.HandleFunc(path, studioRoute("", false, nil))
	}
}

// blocklistCheckQuery accepts exactly one query parameter, bundle_id, holding a canonical UUID.
func blocklistCheckQuery(u *url.URL) (string, bool) {
	if u.ForceQuery {
		return "", false
	}
	values, err := url.ParseQuery(u.RawQuery)
	if err != nil || len(values) != 1 || len(values["bundle_id"]) != 1 || !command.ValidID(values["bundle_id"][0]) {
		return "", false
	}
	return values["bundle_id"][0], true
}

// blocklistClassify adds the two blocklist conflicts (full list, ambiguous conversation) to the claims error table.
func blocklistClassify(err error) (int, string) {
	switch {
	case errors.Is(err, claims.ErrBlocklistFull):
		return http.StatusConflict, "limit_reached"
	case errors.Is(err, claims.ErrAmbiguousActor):
		return http.StatusConflict, "ambiguous_actor"
	}
	return claimsClassify(err)
}
