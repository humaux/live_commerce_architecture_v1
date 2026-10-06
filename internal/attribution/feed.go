// Purpose: GET /v1/buyer/feeds/meta.csv: the public Meta Commerce Manager product feed of the verified storefront host (image links per product-media-v2).
// Depends on: ads.feed_rows, catalog.buyer_feed_images, catalog.buyer_feed_variant_images (migrations 0080/0082/0149); buyer.resolve_published_store.
// Used by: cmd/api (mountFeed) via the storefront BFF app/feeds/meta.csv; tests/foundation/product_media_v2_test.go.

package attribution

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/httperror"
)

// feed.go is GET /v1/buyer/feeds/meta.csv (contract 7, AD10, C6): the public Meta Commerce Manager product feed of the
// verified storefront host. The store is never taken from a query, path or store parameter: only the BFF-set
// X-Commerce-Storefront-Origin header, which ads.feed_rows resolves through buyer.resolve_published_store (0020). The
// feed lists published data only (name, description, price, availability, link), so it needs no signature and carries
// no buyer data. Backend: apps/storefront/app/feeds/meta.csv/route.ts proxies it on the storefront host.
// SQL touched: ads.feed_rows (EXECUTE commerce_buyer_runtime).

// feedColumns are the Meta catalog feed columns (F17). Source: Meta Commerce Manager scheduled data feeds
// https://www.facebook.com/business/help/2284463181837648 (contract F17, retrieved 2026-09-29 as a search excerpt only).
// Contract F17 says "exact columns: re-verify at implementation"; this unit had no network, so the re-verification is
// NOT_RUN and the set below is the long-standing required list (id, title, description, availability, condition, price,
// link, image_link, brand). image_link is the storefront origin + /media/p/<product>/<first image> (catalog-media CM4,
// migrations/0082) and stays empty for a product without photos; Meta reports such items as incomplete.
var feedColumns = []string{"id", "title", "description", "availability", "condition", "price", "link", "image_link", "additional_image_link", "brand"}

const originHeader = "X-Commerce-Storefront-Origin"

// feedRow is one ads.feed_rows row.
type feedRow struct {
	ID, Title, Description, Availability string
	PriceMinor                           int64
	Currency, Link, Brand                string
	ImageLink                            string // absolute public URL: the variant's option-value image, else the first main photo; "" when none
	AdditionalImageLink                  string // comma-joined absolute URLs of the other main photos (product-media-v2); "" when none
}

// feedPrice formats price_minor as Meta's "<amount> <ISO>" (I05: exact decimal of minor units / 100, never a float).
func feedPrice(minor int64, currency string) (string, bool) {
	if minor < 0 || !isoPattern.MatchString(currency) {
		return "", false
	}
	return fmt.Sprintf("%d.%02d %s", minor/100, minor%100, currency), true
}

var isoPattern = regexp.MustCompile(`^[A-Z]{3}$`)

// writeFeed renders the CSV with encoding/csv, so commas, quotes and newlines in a merchant-controlled title are quoted
// per RFC 4180. condition is always "new" (the platform sells new goods only). A row with an unusable price is skipped,
// never emitted with a guessed price.
func writeFeed(w *csv.Writer, rows []feedRow) error {
	if err := w.Write(feedColumns); err != nil {
		return err
	}
	for _, r := range rows {
		price, ok := feedPrice(r.PriceMinor, r.Currency)
		if !ok {
			continue
		}
		if err := w.Write([]string{r.ID, r.Title, r.Description, r.Availability, "new", price, r.Link, r.ImageLink, r.AdditionalImageLink, r.Brand}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

// FeedHandler serves the feed from the buyer runtime pool. Only GET/HEAD, no query string, exactly one origin header.
// Errors use the shared httperror envelope: 422 invalid_request (bad origin), 404 not_found (host not published or
// unknown), 503 unavailable. Success is cacheable for 15 minutes (cache key = host, the CDN keys on the Host header).
func FeedHandler(pool *pgxpool.Pool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			httperror.Write(w, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}
		origins := r.Header.Values(originHeader)
		if r.URL.RawQuery != "" || len(origins) != 1 || origins[0] == "" || strings.ContainsAny(origins[0], " ,\r\n") {
			httperror.Write(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		rows, err := loadFeed(ctx, pool, origins[0])
		if err != nil {
			status, code := feedError(err)
			httperror.Write(w, status, code)
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=900")
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodGet {
			_ = writeFeed(csv.NewWriter(w), rows) // a write error is a closed client; the status line is already sent
		}
	})
}

func loadFeed(ctx context.Context, pool *pgxpool.Pool, origin string) ([]feedRow, error) {
	if pool == nil {
		return nil, errors.New("attribution: no pool")
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	// ads.feed_rows: resolves the store from the verified origin itself (buyer.resolve_published_store) and returns
	// active SKUs of active products only; PT400 invalid origin, PT404 not published.
	rs, err := tx.Query(ctx, `SELECT id,title,description,availability,price_minor,currency,link,brand FROM ads.feed_rows($1::text)`, origin)
	if err != nil {
		return nil, err
	}
	defer rs.Close() // closed again explicitly below; Close is idempotent
	var out []feedRow
	for rs.Next() {
		var r feedRow
		if err = rs.Scan(&r.ID, &r.Title, &r.Description, &r.Availability, &r.PriceMinor, &r.Currency, &r.Link, &r.Brand); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err = rs.Err(); err != nil {
		return nil, err
	}
	rs.Close() // the connection must be idle before the second statement
	return out, addImageLinks(ctx, tx, origin, out)
}

// addImageLinks sets ImageLink and AdditionalImageLink (product-media-v2). catalog.buyer_feed_images (migrations/0082, replaced by 0149; owner
// commerce_catalog_media, EXECUTE commerce_buyer_runtime) lists every MAIN image id of each active product in display order and
// catalog.buyer_feed_variant_images (0149) the option-value image of each SKU that has one; no bytes, no detail images. The feed row's
// link is origin + "/products/" + product id (ads.feed_rows) and its id is the SKU id: the join keys back to the product and the SKU.
func addImageLinks(ctx context.Context, tx pgx.Tx, origin string, rows []feedRow) error {
	if len(rows) == 0 {
		return nil
	}
	rs, err := tx.Query(ctx, `SELECT product_id::text,image_id::text FROM catalog.buyer_feed_images($1::text)`, origin)
	if err != nil {
		return err
	}
	defer rs.Close()
	main := map[string][]string{}
	for rs.Next() {
		var product, image string
		if err = rs.Scan(&product, &image); err != nil {
			return err
		}
		main[product] = append(main[product], image)
	}
	if err = rs.Err(); err != nil {
		return err
	}
	rs.Close() // the connection must be idle before the second statement
	vs, err := tx.Query(ctx, `SELECT sku_id::text,image_id::text FROM catalog.buyer_feed_variant_images($1::text)`, origin)
	if err != nil {
		return err
	}
	defer vs.Close()
	variant := map[string]string{}
	for vs.Next() {
		var sku, image string
		if err = vs.Scan(&sku, &image); err != nil {
			return err
		}
		variant[sku] = image
	}
	if err = vs.Err(); err != nil {
		return err
	}
	for i := range rows {
		rows[i].ImageLink, rows[i].AdditionalImageLink = imageLinks(origin, rows[i].Link, variant[rows[i].ID], main)
	}
	return nil
}

// imageLinks maps a feed row link (origin/products/<product>), the SKU's option-value image id ("" when none) and the main-image map to
// (image_link, additional_image_link): the variant image else main[0] first, then main[1..3] (or main[0..3] after a variant image).
// Both are "" when the link is not of that shape or there is no image.
func imageLinks(origin, link, variantImage string, main map[string][]string) (string, string) {
	product, ok := strings.CutPrefix(link, origin+"/products/")
	if !ok {
		return "", ""
	}
	url := func(id string) string { return origin + "/media/p/" + product + "/" + id }
	ids := main[product]
	if variantImage != "" {
		ids = append([]string{variantImage}, ids...)
	}
	if len(ids) == 0 {
		return "", ""
	}
	if len(ids) > 5 { // a variant image plus the four main images is the most a row can show
		ids = ids[:5]
	}
	extra := make([]string, 0, len(ids)-1)
	for _, id := range ids[1:] {
		extra = append(extra, url(id))
	}
	return url(ids[0]), strings.Join(extra, ",")
}

// feedError maps the definer's SQLSTATEs to HTTP without ever returning a driver message.
func feedError(err error) (int, string) {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "PT400":
			return http.StatusUnprocessableEntity, "invalid_request"
		case "PT404":
			return http.StatusNotFound, "not_found"
		}
	}
	return http.StatusServiceUnavailable, "unavailable"
}
