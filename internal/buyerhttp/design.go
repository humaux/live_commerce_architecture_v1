package buyerhttp

// design.go serves the storefront design document to the storefront Next app (unit store-design, contracts/storefront-v2.md
// section B). Three public reads, none takes a buyer bearer (the BFF key + X-Commerce-Storefront-Origin only, the same
// trust shape as media.go; Authorization, cookies and query strings are refused by ServeHTTP):
//
//	GET /v1/buyer/design/published        -> {"version": n, "document": {...}}   (version 0 = no published version: default document)
//	GET /v1/buyer/design/preview          -> same shape, the DRAFT; token in header X-Commerce-Design-Preview
//	GET /v1/buyer/media/s/{image_id}      -> image bytes of one store-media row
//
// SQL touched (migrations/0087, EXECUTE commerce_buyer_runtime, each resolves the store from the origin itself via
// buyer.resolve_published_store): design.buyer_published, design.buyer_preview, design.buyer_media. Preview answers are
// no-store; a token that is malformed, unknown, expired, for another store or stale (draft saved since) is the same 404 as
// an unpublished store, so nothing is learned from a miss. Media is cache-immutable like product photos (the id changes
// with the content).

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/command"
	"livecommerce/internal/design"
)

const (
	designPublishedPath = "/v1/buyer/design/published"
	designPreviewPath   = "/v1/buyer/design/preview"
	// previewTokenHeader carries the preview token from the storefront BFF; never a query string (logs, Referer).
	previewTokenHeader = "X-Commerce-Design-Preview"
)

func isDesignRoute(kind routeKind) bool {
	return kind == designPublishedRoute || kind == designPreviewRoute || kind == storeMediaRoute
}

func (h *handler) designGet(ctx context.Context, w http.ResponseWriter, r *http.Request, selected route) error {
	if err := noBody(r); err != nil {
		return err
	}
	origin := r.Header.Get("X-Commerce-Storefront-Origin")
	var query string
	var args []any
	switch selected.kind {
	case designPublishedRoute:
		if len(r.Header.Values(previewTokenHeader)) != 0 {
			return responseError{http.StatusUnprocessableEntity, "invalid_request"}
		}
		query, args = `SELECT version,document,store_name FROM design.buyer_published($1::text)`, []any{origin}
	case designPreviewRoute:
		values := r.Header.Values(previewTokenHeader)
		var hash []byte
		ok := len(values) == 1
		if ok {
			hash, ok = design.TokenHash(values[0])
		}
		if !ok {
			return responseError{http.StatusNotFound, "not_found"} // identical to an unknown token
		}
		query, args = `SELECT version,document,store_name FROM design.buyer_preview($1::text,$2::bytea)`, []any{origin, hash}
	default:
		if !command.ValidID(selected.image) {
			return responseError{http.StatusUnprocessableEntity, "invalid_request"}
		}
		return h.storeMedia(ctx, w, origin, selected.image)
	}
	tx, err := h.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted, AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	var version int64
	var document []byte
	var storeName string
	if err = tx.QueryRow(ctx, query, args...).Scan(&version, &document, &storeName); err != nil {
		return designPGError(err)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if document == nil { // no published version yet: the contract default derived from the store name
		document = design.DefaultDocument(storeName)
	}
	writeOK(w, struct {
		Version  int64           `json:"version"`
		Document json.RawMessage `json:"document"`
	}{version, document})
	return nil
}

func (h *handler) storeMedia(ctx context.Context, w http.ResponseWriter, origin, image string) error {
	tx, err := h.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted, AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	var contentType string
	var data, digest []byte
	if err = tx.QueryRow(ctx, `SELECT content_type,bytes,sha256 FROM design.buyer_media($1::text,$2::uuid)`, origin, image).Scan(&contentType, &data, &digest); err != nil {
		return designPGError(err)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	// ponytail: whole file in memory (<= 2 MiB by CHECK); stream from object storage when images leave PG.
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("ETag", `"`+hex.EncodeToString(digest)+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
	return nil
}

// designPGError maps the definers' PT400 (invalid origin) / PT404 (not published, no such row) codes.
func designPGError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "PT400":
			return responseError{http.StatusUnprocessableEntity, "invalid_request"}
		case "PT404":
			return responseError{http.StatusNotFound, "not_found"}
		}
	}
	return err
}
