package buyerhttp

// media.go serves GET /v1/buyer/media/p/{product_id}/{image_id}: the bytes of one merchant product photo
// (docs/delivery/units/catalog-media.md CM4, contracts/buyer-catalog-discovery-v1.md). Public data, so it needs no
// buyer bearer: the storefront Next route apps/storefront/app/media/p/[productID]/[imageID]/route.ts derives the
// origin from the verified Host and calls this with the BFF key + X-Commerce-Storefront-Origin only (the same trust
// shape as the Meta feed, internal/attribution/feed.go). Authorization, cookies and query strings are refused.
//
// SQL touched: catalog.buyer_media_image (migrations/0082; EXECUTE commerce_buyer_runtime) which resolves the
// store from the origin itself (buyer.resolve_published_store) and returns bytes only for an active product of a
// published store. The id changes whenever the content changes, so the response is cache-immutable.

import (
	"context"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/command"
)

func (h *handler) mediaGet(ctx context.Context, w http.ResponseWriter, r *http.Request, selected route) error {
	if err := noBody(r); err != nil {
		return err
	}
	if !command.ValidID(selected.id) || !command.ValidID(selected.image) {
		return responseError{http.StatusUnprocessableEntity, "invalid_request"}
	}
	tx, err := h.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted, AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	var contentType string
	var data, digest []byte
	err = tx.QueryRow(ctx, `SELECT content_type,bytes,sha256 FROM catalog.buyer_media_image($1::text,$2::uuid,$3::uuid)`,
		r.Header.Get("X-Commerce-Storefront-Origin"), selected.id, selected.image).Scan(&contentType, &data, &digest)
	if err != nil {
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
