// Purpose: the merchant tag and note HTTP adapter of the W6-01B amendment to contracts/customers-billing-v1.md, under
//   /v1/admin/stores/{store_id}/customers: GET|POST /tags, PATCH|DELETE /tags/{tag_id}, PUT /{customer_id}/tags,
//   GET|POST /{customer_id}/notes, PATCH|DELETE /{customer_id}/notes/{note_id}. It decides no rule: internal/customers and the
//   0139 SECURITY DEFINER functions do. Writes need customers:write and exactly one Idempotency-Key; reads need customers:read.
// Depends on: internal/customers (CreateTag, RenameTag, DeleteTag, ListTags, SetOwnerTags, AddNote, EditNote, DeleteNote,
//   ListNotes), customers.go helpers (customerRoute, customersScope, customersClassify), claimsBody (strict JSON decoder).
// Used by: customers.go registerCustomerRoutes (mounted unconditionally, like the five privacy/read rows).
// Invariants: never log or echo a note body outside the success response; responses are private and non-cacheable.

package httpapi

import (
	"context"
	"net/http"
	"net/url"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/command"
	"livecommerce/internal/customers"
	"livecommerce/internal/pagination"
	"livecommerce/internal/platform"
)

// tagWrite builds one customers:write row: the customerRoute transport gate first (method, ids, exactly one Idempotency-Key),
// then the strict JSON body (or no body at all when no field is declared), then the scoped transaction. status is the
// success code sent only after COMMIT is acknowledged.
func tagWrite[T any](pool *pgxpool.Pool, method string, status int, required, optional []string,
	fn func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, in T) (any, error)) http.HandlerFunc {
	return customerRoute(method, false, func(w http.ResponseWriter, r *http.Request) {
		var in T
		if len(required)+len(optional) == 0 {
			if hasBody(r) {
				respondError(w, http.StatusUnprocessableEntity, "invalid_request")
				return
			}
		} else {
			var ok bool
			if in, ok = claimsBody[T](w, r, required, nil, optional...); !ok {
				return
			}
		}
		if result, ok := customersScope(w, r, pool, "customers:write", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return fn(ctx, tx, s, r, in)
		}); ok {
			respond(w, status, result)
		}
	})
}

// registerCustomerTagRoutes mounts the nine W6-01B rows. Go 1.22 mux specificity keeps the literal "tags" segment apart from
// the {customer_id} wildcard; the DB-free full-router test (customer_tags_test.go) fails at registration on any conflict.
func registerCustomerTagRoutes(mux *http.ServeMux, pool *pgxpool.Pool) {
	key := func(r *http.Request) string { return r.Header.Get("Idempotency-Key") }

	mux.HandleFunc("GET "+customerBase+"/tags", customerRoute(http.MethodGet, false, func(w http.ResponseWriter, r *http.Request) {
		if result, ok := customersScope(w, r, pool, "customers:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			tags, err := customers.ListTags(ctx, tx, s, bearerToken(r))
			return map[string]any{"items": tags}, err
		}); ok {
			respond(w, http.StatusOK, result)
		}
	}))
	mux.HandleFunc("POST "+customerBase+"/tags", tagWrite(pool, http.MethodPost, http.StatusCreated, []string{"name", "color"}, nil,
		func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, in customers.TagInput) (any, error) {
			return customers.CreateTag(ctx, tx, s, bearerToken(r), key(r), in)
		}))
	mux.HandleFunc("PATCH "+customerBase+"/tags/{tag_id}", tagWrite(pool, http.MethodPatch, http.StatusOK, nil, []string{"name", "color"},
		func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, in customers.TagPatch) (any, error) {
			return customers.RenameTag(ctx, tx, s, bearerToken(r), key(r), r.PathValue("tag_id"), in)
		}))
	mux.HandleFunc("DELETE "+customerBase+"/tags/{tag_id}", tagWrite(pool, http.MethodDelete, http.StatusOK, nil, nil,
		func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, _ struct{}) (any, error) {
			return customers.DeleteTag(ctx, tx, s, bearerToken(r), key(r), r.PathValue("tag_id"))
		}))
	mux.HandleFunc("PUT "+customerBase+"/{customer_id}/tags", tagWrite(pool, http.MethodPut, http.StatusOK, []string{"tag_ids", "revision"}, nil,
		func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, in customers.SetTagsInput) (any, error) {
			return customers.SetOwnerTags(ctx, tx, s, bearerToken(r), key(r), r.PathValue("customer_id"), in)
		}))

	mux.HandleFunc("GET "+customerBase+"/{customer_id}/notes", customerRoute(http.MethodGet, true, func(w http.ResponseWriter, r *http.Request) {
		page, err := parseNotesQuery(r.URL)
		if err != nil {
			respondError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		if result, ok := customersScope(w, r, pool, "customers:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return customers.ListNotes(ctx, tx, s, bearerToken(r), r.PathValue("customer_id"), page)
		}); ok {
			respond(w, http.StatusOK, result)
		}
	}))
	mux.HandleFunc("POST "+customerBase+"/{customer_id}/notes", tagWrite(pool, http.MethodPost, http.StatusCreated, []string{"body"}, nil,
		func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, in customers.NoteInput) (any, error) {
			return customers.AddNote(ctx, tx, s, bearerToken(r), key(r), r.PathValue("customer_id"), in)
		}))
	mux.HandleFunc("PATCH "+customerBase+"/{customer_id}/notes/{note_id}", tagWrite(pool, http.MethodPatch, http.StatusOK, []string{"body", "version"}, nil,
		func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, in customers.EditNoteInput) (any, error) {
			return customers.EditNote(ctx, tx, s, bearerToken(r), key(r), r.PathValue("customer_id"), r.PathValue("note_id"), in)
		}))
	mux.HandleFunc("DELETE "+customerBase+"/{customer_id}/notes/{note_id}", tagWrite(pool, http.MethodDelete, http.StatusOK, nil, nil,
		func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, _ struct{}) (any, error) {
			return customers.DeleteNote(ctx, tx, s, bearerToken(r), key(r), r.PathValue("customer_id"), r.PathValue("note_id"))
		}))
}

// parseNotesQuery accepts only limit and after (the list-query grammar of the customers route, minus q and tag).
func parseNotesQuery(u *url.URL) (pagination.Request, error) {
	in, err := parseCustomersQuery(u)
	if err != nil || in.Q != "" || in.Tag != "" {
		return pagination.Request{}, command.ErrInvalid
	}
	return in.Page, nil
}
