package httpapi

// images.go mounts the merchant product-photo routes (docs/delivery/units/catalog-media.md CM3, contract
// contracts/catalog-inventory-openapi.json): multipart upload, metadata list, bytes preview, delete and reorder.
// BFF: apps/admin/app/api/stores/[store]/[...resource]/route.ts (allowlisted exactly these paths) -> here.
// Domain: internal/catalog images.go (validation by magic bytes, SQL on catalog.product_images). This file owns only
// transport: body bounds, the single `file` part, the raw-bytes response and error mapping. Scope comes from server
// auth (scoped); no tenant or store id is read from the request. Buyers never reach these routes (they use
// internal/buyerhttp mediaGet).

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/catalog"
	"livecommerce/internal/platform"
)

// maxUploadBody is the file cap plus multipart framing; the file itself is capped again at catalog.MaxImageBytes.
const maxUploadBody = catalog.MaxImageBytes + 64<<10

// rawResponse is what an action returns when the answer is bytes, not JSON (respond writes it verbatim).
type rawResponse struct {
	contentType string
	body        []byte
}

func registerImageRoutes(mux *http.ServeMux, pool *pgxpool.Pool) {
	const images = "/v1/admin/stores/{store_id}/products/{product_id}/images"
	mux.HandleFunc("POST "+images, uploadImageRoute(pool))
	mux.HandleFunc("GET "+images, scoped(pool, "catalog:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
		return catalog.ListImages(ctx, tx, s, r.PathValue("product_id"))
	}))
	mux.HandleFunc("GET "+images+"/{image_id}", scoped(pool, "catalog:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
		img, err := catalog.GetImage(ctx, tx, s, r.PathValue("product_id"), r.PathValue("image_id"))
		if err != nil {
			return nil, err
		}
		return rawResponse{contentType: img.ContentType, body: img.Bytes}, nil
	}))
	mux.HandleFunc("POST "+images+"/{image_id}/delete", bodyRoute(pool, "catalog:write", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, _ struct{}) (any, error) {
		return catalog.DeleteImage(ctx, tx, s, r.Header.Get("Idempotency-Key"), r.PathValue("product_id"), r.PathValue("image_id"))
	}))
	mux.HandleFunc("POST "+images+"/order", bodyRoute(pool, "catalog:write", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, in catalog.ReorderInput) (any, error) {
		return catalog.ReorderImages(ctx, tx, s, r.Header.Get("Idempotency-Key"), r.PathValue("product_id"), in)
	}))
}

// writeRaw answers an image preview: private cache only (the merchant session is the authority), nosniff comes from
// httperror.Middleware, and a locked-down CSP so the bytes can never run as a document even if a type were wrong.
func writeRaw(w http.ResponseWriter, raw rawResponse) {
	w.Header().Set("Content-Type", raw.contentType)
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw.body)
}

// uploadImageRoute reads the single multipart `file` part before any transaction opens (a slow client must not hold
// a database transaction), but only after the Authorization header is at least well-formed; scoped then
// authenticates for real. The part's filename and Content-Type are ignored: catalog.SniffImage decides the type.
func uploadImageRoute(pool *pgxpool.Pool) http.HandlerFunc {
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
		scoped(pool, "catalog:write", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return catalog.UploadImage(ctx, tx, s, r.Header.Get("Idempotency-Key"), r.PathValue("product_id"), data)
		})(w, r)
	}
}

// readSingleFile returns the bytes of the one `file` part, or the HTTP status to refuse with (0 = ok): 413 when the
// body or the file is over the cap, 422 for any other malformation (no part, another field name, a second part).
func readSingleFile(r *http.Request) ([]byte, int) {
	reader, err := r.MultipartReader()
	if err != nil {
		return nil, http.StatusUnprocessableEntity
	}
	part, err := reader.NextPart()
	if err != nil || part.FormName() != "file" {
		return nil, refuseUpload(err)
	}
	data, err := io.ReadAll(io.LimitReader(part, catalog.MaxImageBytes+1))
	if err != nil {
		return nil, refuseUpload(err)
	}
	if len(data) > catalog.MaxImageBytes {
		return nil, http.StatusRequestEntityTooLarge
	}
	if _, err = reader.NextPart(); !errors.Is(err, io.EOF) {
		return nil, refuseUpload(err)
	}
	return data, 0
}

func refuseUpload(err error) int {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusUnprocessableEntity
}
