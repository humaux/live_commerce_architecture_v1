// Purpose: private merchant member HTTP adapter; loyalty and its SQL definers own policy.
// Depends on: loyalty, platform scoped transactions, studio strict JSON and customer attachments.
// Used by: registerMerchantToolsRoutes; member transport and REAL_PG acceptance tests.
// Invariants: no PII in URLs/logs; emit success only after transaction commit.
package httpapi

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/command"
	"livecommerce/internal/loyalty"
	"livecommerce/internal/platform"
	"net/http"
	"net/url"
	"time"
)

const memberBase = "/v1/admin/stores/{store_id}/members"

func memberRoute(method string, read bool, query string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, private")
		if r.Method != method {
			respondError(w, 405, "method_not_allowed")
			return
		}
		id := r.PathValue("member_id")
		if id != "" && !command.ValidID(id) {
			respondError(w, 422, "invalid_request")
			return
		}
		q, err := parseMemberQuery(r, query)
		if err != nil {
			respondError(w, 422, "invalid_request")
			return
		}
		_ = q
		keys := r.Header.Values("Idempotency-Key")
		if (read && len(keys) != 0) || (!read && (len(keys) != 1 || !claimsKey.MatchString(keys[0]))) || (method == "GET" && hasBody(r)) {
			respondError(w, 422, "invalid_request")
			return
		}
		next(w, r)
	}
}
func parseMemberQuery(r *http.Request, allowed string) (string, error) {
	if r.URL.ForceQuery || len(r.URL.RawQuery) > 2048 {
		return "", command.ErrInvalid
	}
	q, err := r.URL.Query(), error(nil)
	if r.URL.RawQuery != "" {
		q, err = url.ParseQuery(r.URL.RawQuery)
	}
	if err != nil {
		return "", err
	}
	for k, v := range q {
		if allowed == "" || k != allowed || len(v) != 1 || v[0] == "" || len(v[0]) > 1024 {
			return "", command.ErrInvalid
		}
	}
	if allowed == "order_id" && !command.ValidID(q.Get(allowed)) {
		return "", command.ErrInvalid
	}
	return q.Get(allowed), nil
}
func memberBody[T any](w http.ResponseWriter, r *http.Request, fields []string) (T, map[string]json.RawMessage, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	in, raw, ok := studioDecodeRaw[T](w, r, fields)
	var present map[string]json.RawMessage
	if ok {
		ok = json.Unmarshal(raw, &present) == nil
	}
	return in, present, ok
}
func memberScope(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool, permission string, fn action) (any, bool) {
	if pool == nil || !canonicalBearer(r) {
		respondError(w, 401, "unauthorized")
		return nil, false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var result any
	err := platform.WithScope(ctx, pool, bearerToken(r), r.PathValue("store_id"), permission, func(tx pgx.Tx, s platform.Scope) error { var e error; result, e = fn(ctx, tx, s, r); return e })
	if err != nil {
		status, code, ok := loyalty.Classify(err)
		if !ok {
			status, code = claimsClassify(err)
		}
		respondError(w, status, code)
		return nil, false
	}
	return result, true
}
func registerMemberRoutes(mux *http.ServeMux, pool *pgxpool.Pool) {
	mux.HandleFunc("POST "+memberBase+"/search", memberRoute("POST", true, "", func(w http.ResponseWriter, r *http.Request) {
		in, _, ok := memberBody[loyalty.SearchInput](w, r, []string{"q", "include_erased", "cursor", "limit"})
		if !ok {
			return
		}
		if out, ok := memberScope(w, r, pool, "customers:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, _ *http.Request) (any, error) {
			return loyalty.Search(ctx, tx, s, bearerToken(r), in)
		}); ok {
			respond(w, 200, out)
		}
	}))
	mux.HandleFunc("POST "+memberBase, memberRoute("POST", false, "", func(w http.ResponseWriter, r *http.Request) {
		in, _, ok := memberBody[loyalty.MemberInput](w, r, []string{"display_name", "contact_note", "notes"})
		if !ok {
			return
		}
		if in.DisplayName == "" {
			respondError(w, 422, "invalid_request")
			return
		}
		status := 201
		if out, ok := memberScope(w, r, pool, "customers:points", func(ctx context.Context, tx pgx.Tx, s platform.Scope, _ *http.Request) (any, error) {
			m, replay, e := loyalty.CreateMember(ctx, tx, s, bearerToken(r), r.Header.Get("Idempotency-Key"), in)
			if replay {
				status = 200
			}
			return map[string]any{"member": m}, e
		}); ok {
			respond(w, status, out)
		}
	}))
	mux.HandleFunc("GET "+memberBase+"/{member_id}", memberRoute("GET", true, "ledger_cursor", func(w http.ResponseWriter, r *http.Request) {
		if out, ok := memberScope(w, r, pool, "customers:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, _ *http.Request) (any, error) {
			return loyalty.Get(ctx, tx, s, bearerToken(r), r.PathValue("member_id"), r.URL.Query().Get("ledger_cursor"))
		}); ok {
			respond(w, 200, out)
		}
	}))
	mux.HandleFunc("PATCH "+memberBase+"/{member_id}", memberRoute("PATCH", false, "", func(w http.ResponseWriter, r *http.Request) {
		in, fields, ok := memberBody[loyalty.MemberPatch](w, r, []string{"expected_version", "display_name", "contact_note", "notes"})
		if !ok {
			return
		}
		for _, k := range []string{"display_name", "contact_note", "notes"} {
			if _, yes := fields[k]; yes {
				in.Fields = append(in.Fields, k)
			}
		}
		if in.ExpectedVersion < 1 || len(in.Fields) == 0 || (fields["display_name"] != nil && in.DisplayName == nil) {
			respondError(w, 422, "invalid_request")
			return
		}
		if out, ok := memberScope(w, r, pool, "customers:points", func(ctx context.Context, tx pgx.Tx, s platform.Scope, _ *http.Request) (any, error) {
			m, _, e := loyalty.UpdateMember(ctx, tx, s, bearerToken(r), r.Header.Get("Idempotency-Key"), r.PathValue("member_id"), in)
			return map[string]any{"member": m}, e
		}); ok {
			respond(w, 200, out)
		}
	}))
	mux.HandleFunc("POST "+memberBase+"/{member_id}/adjustments", memberRoute("POST", false, "", func(w http.ResponseWriter, r *http.Request) {
		in, _, ok := memberBody[loyalty.AdjustInput](w, r, []string{"delta", "note", "order_id"})
		if !ok {
			return
		}
		if in.Delta == 0 || in.Note == "" {
			respondError(w, 422, "invalid_request")
			return
		}
		status := 201
		if out, ok := memberScope(w, r, pool, "customers:points", func(ctx context.Context, tx pgx.Tx, s platform.Scope, _ *http.Request) (any, error) {
			v, replay, e := loyalty.Adjust(ctx, tx, s, bearerToken(r), r.Header.Get("Idempotency-Key"), r.PathValue("member_id"), in)
			if replay {
				status = 200
			}
			return v, e
		}); ok {
			respond(w, status, out)
		}
	}))
	mux.HandleFunc("GET "+memberBase+"/entries", memberRoute("GET", true, "order_id", func(w http.ResponseWriter, r *http.Request) {
		if out, ok := memberScope(w, r, pool, "customers:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, _ *http.Request) (any, error) {
			return loyalty.EntriesForOrder(ctx, tx, s, bearerToken(r), r.URL.Query().Get("order_id"))
		}); ok {
			respond(w, 200, out)
		}
	}))
	for _, mode := range []string{"export", "erasure"} {
		mux.HandleFunc("POST "+memberBase+"/{member_id}/"+mode, memberRoute("POST", false, "", func(w http.ResponseWriter, r *http.Request) {
			_, _, ok := memberBody[struct{}](w, r, nil)
			if !ok {
				return
			}
			out, ok := memberScope(w, r, pool, "customers:privacy", func(ctx context.Context, tx pgx.Tx, s platform.Scope, _ *http.Request) (any, error) {
				if mode == "export" {
					v, _, e := loyalty.Export(ctx, tx, s, bearerToken(r), r.Header.Get("Idempotency-Key"), r.PathValue("member_id"))
					return v, e
				}
				v, _, e := loyalty.Erase(ctx, tx, s, bearerToken(r), r.Header.Get("Idempotency-Key"), r.PathValue("member_id"))
				return v, e
			})
			if !ok {
				return
			}
			if mode == "export" {
				writeAttachment(w, "application/json", `attachment; filename="member-`+r.PathValue("member_id")+`.json"`, out.([]byte))
			} else {
				respond(w, 200, out)
			}
		}))
	}
}
