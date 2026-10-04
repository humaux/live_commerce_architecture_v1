package httpapi

// design.go mounts the merchant storefront-design routes (unit store-design, contracts/storefront-v2.md section B):
//
//	GET  /v1/admin/stores/{store_id}/design/draft               integration:read    -> design.ReadDraft
//	PUT  .../design/draft {expected_version, document}          integration:manage  -> design.SaveDraft (CAS)
//	POST .../design/publish {expected_draft_version}            integration:manage  -> design.Publish
//	GET  .../design/versions                                    integration:read    -> design.ListVersions
//	POST .../design/rollback {version}                          integration:manage  -> design.Rollback
//	POST .../design/preview-token {}                            integration:manage  -> design.IssuePreviewToken
//	GET/POST .../design/media, GET .../design/media/{image_id}, POST .../design/media/{image_id}/delete
//
// Permissions: the narrowest ones the Settings storefront card already uses (integration:read / integration:manage), so
// publishing the design needs the same authority as publishing the storefront. BFF: apps/admin/app/api/stores/[store]/
// [...resource]/route.ts (lib/design-request.ts allowlist) -> here; UI apps/admin/components/Design.tsx. Scope comes from
// server auth (platform.WithScope); no tenant or store id is read from the request. A document that fails validation is
// 422 invalid_request with details {path, reason} (the one place a request value is reported, and only its path).

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/design"
	"livecommerce/internal/httperror"
	"livecommerce/internal/platform"
)

// maxDesignBody is the document cap plus the PUT envelope; design.Normalize re-checks the document itself.
const maxDesignBody = design.MaxDocumentBytes + 4<<10

func registerDesignRoutes(mux *http.ServeMux, pool *pgxpool.Pool) {
	const base = "/v1/admin/stores/{store_id}/design"
	mux.HandleFunc("GET "+base+"/draft", designScoped(pool, "integration:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, _ *http.Request) (any, error) {
		return design.ReadDraft(ctx, tx, s)
	}))
	mux.HandleFunc("PUT "+base+"/draft", designBody(pool, "integration:manage", func(ctx context.Context, tx pgx.Tx, s platform.Scope, in design.SaveInput) (any, error) {
		return design.SaveDraft(ctx, tx, s, in)
	}))
	mux.HandleFunc("POST "+base+"/publish", designBody(pool, "integration:manage", func(ctx context.Context, tx pgx.Tx, s platform.Scope, in design.PublishInput) (any, error) {
		return design.Publish(ctx, tx, s, in)
	}))
	mux.HandleFunc("POST "+base+"/rollback", designBody(pool, "integration:manage", func(ctx context.Context, tx pgx.Tx, s platform.Scope, in design.RollbackInput) (any, error) {
		return design.Rollback(ctx, tx, s, in)
	}))
	mux.HandleFunc("POST "+base+"/preview-token", designBody(pool, "integration:manage", func(ctx context.Context, tx pgx.Tx, s platform.Scope, _ struct{}) (any, error) {
		return design.IssuePreviewToken(ctx, tx, s)
	}))
	mux.HandleFunc("GET "+base+"/versions", designScoped(pool, "integration:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, _ *http.Request) (any, error) {
		return design.ListVersions(ctx, tx, s)
	}))
	mux.HandleFunc("GET "+base+"/media", designScoped(pool, "integration:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, _ *http.Request) (any, error) {
		return design.ListMedia(ctx, tx, s)
	}))
	mux.HandleFunc("POST "+base+"/media", uploadDesignMedia(pool))
	mux.HandleFunc("GET "+base+"/media/{image_id}", designScoped(pool, "integration:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
		img, err := design.GetMedia(ctx, tx, s, r.PathValue("image_id"))
		if err != nil {
			return nil, err
		}
		return rawResponse{contentType: img.ContentType, body: img.Bytes}, nil
	}))
	mux.HandleFunc("POST "+base+"/media/{image_id}/delete", designBodyWith(pool, "integration:manage", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, _ struct{}) (any, error) {
		return design.DeleteMedia(ctx, tx, s, r.PathValue("image_id"))
	}))
}

// designScoped is scopedAs with one difference: a design.ValidationError answers 422 with details {path, reason}. The
// shared httperror writer always sends empty details, and an editor needs to know which field to fix.
func designScoped(pool *pgxpool.Pool, permission string, fn action) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") || strings.ContainsAny(strings.TrimPrefix(header, "Bearer "), " \t\r\n") {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		var result any
		err := platform.WithScope(ctx, pool, strings.TrimPrefix(header, "Bearer "), r.PathValue("store_id"), permission, func(tx pgx.Tx, s platform.Scope) error {
			var inner error
			result, inner = fn(ctx, tx, s, r)
			return inner
		})
		if err != nil {
			var invalid *design.ValidationError
			if errors.As(err, &invalid) {
				writeValidation(w, invalid)
				return
			}
			status, code := classify(err)
			respondError(w, status, code)
			return
		}
		respond(w, http.StatusOK, result)
	}
}

func writeValidation(w http.ResponseWriter, e *design.ValidationError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnprocessableEntity)
	_ = json.NewEncoder(w).Encode(httperror.Envelope{Code: "invalid_request", Message: "Request validation failed.",
		RequestID: w.Header().Get("X-Request-ID"), Details: map[string]any{"path": e.Path, "reason": e.Reason}})
}

func designBody[T any](pool *pgxpool.Pool, permission string, fn func(context.Context, pgx.Tx, platform.Scope, T) (any, error)) http.HandlerFunc {
	return designBodyWith(pool, permission, func(ctx context.Context, tx pgx.Tx, s platform.Scope, _ *http.Request, in T) (any, error) {
		return fn(ctx, tx, s, in)
	})
}

// designBodyWith is bodyRoute with the larger document cap: unknown fields and trailing values are refused before any
// transaction opens, and the bytes are never logged.
func designBodyWith[T any](pool *pgxpool.Pool, permission string, fn func(context.Context, pgx.Tx, platform.Scope, *http.Request, T) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || media != "application/json" {
			respondError(w, http.StatusUnsupportedMediaType, "json_required")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxDesignBody)
		defer r.Body.Close()
		var in T
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err = dec.Decode(&in); err != nil {
			respondError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		var extra any
		if err = dec.Decode(&extra); err != io.EOF {
			respondError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		designScoped(pool, permission, func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return fn(ctx, tx, s, r, in)
		})(w, r)
	}
}

// uploadDesignMedia is uploadImageRoute for store media: same single-`file`-part reader and caps, same sniffing
// (design.UploadMedia -> catalog.SniffImage decides the type, the filename and Content-Type are ignored).
func uploadDesignMedia(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") || strings.ContainsAny(strings.TrimPrefix(header, "Bearer "), " \t\r\n") {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || media != "multipart/form-data" {
			respondError(w, http.StatusUnsupportedMediaType, "invalid_request")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxUploadBody)
		defer r.Body.Close()
		data, status := readSingleFile(r)
		if status != 0 {
			respondError(w, status, "invalid_request")
			return
		}
		designScoped(pool, "integration:manage", func(ctx context.Context, tx pgx.Tx, s platform.Scope, _ *http.Request) (any, error) {
			return design.UploadMedia(ctx, tx, s, data)
		})(w, r)
	}
}
