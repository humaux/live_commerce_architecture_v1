package buyerhttp

// catalogv2.go serves the catalog-v2 buyer reads of contracts/storefront-v2.md section A: GET
// /v1/buyer/catalog/v2/products (list), .../products/{slug_or_id}, .../collections, .../collections/{slug}, and the
// public collection image bytes GET /v1/buyer/media/c/{collection_id}/{image_id} (unit catalog-core).
// They are public store data, so like the media route they take NO buyer bearer: the storefront BFF sends only its
// key and the verified origin (X-Commerce-Storefront-Origin); Authorization, cookies and unknown query keys are
// refused. SQL touched: the catalog.buyer_v2_* and catalog.buyer_collection_image definers (migrations/0086; owner
// commerce_catalog_media, EXECUTE commerce_buyer_runtime), each resolving the store from the origin itself
// (buyer.resolve_published_store: PT400 invalid, PT404 not published or no such row). The definers return jsonb;
// this file decodes it into closed structs (explicit projection) so a database drift can never widen the response.
// Caching is the BFF's decision: every answer here is Cache-Control no-store. It never reads a table directly.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/command"
)

const (
	v2Prefix        = "/v1/buyer/catalog/v2/"
	v2DefaultLimit  = 24
	v2MaxLimit      = 48
	v2MaxQuery      = 60
	v2MaxOffset     = 100000
	v2MaxMinorMoney = 1_000_000_000_000
)

var v2Slug = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// matchCatalogV2 maps the part after /v1/buyer/catalog/v2/ to a route; anything else is not found.
func matchCatalogV2(rest string) route {
	switch {
	case rest == "products":
		return route{kind: catalogV2ProductsRoute}
	case rest == "collections":
		return route{kind: catalogV2CollectionsRoute}
	}
	if key, ok := strings.CutPrefix(rest, "products/"); ok && (v2Slug.MatchString(key) && len(key) <= 80 || command.ValidID(key)) {
		return route{kind: catalogV2ProductRoute, slug: key}
	}
	if key, ok := strings.CutPrefix(rest, "collections/"); ok && v2Slug.MatchString(key) && len(key) <= 80 && !command.ValidID(key) { // UUID-shaped slugs cannot exist (SQL CHECK)
		return route{kind: catalogV2CollectionRoute, slug: key}
	}
	return route{}
}

func isCatalogV2(kind routeKind) bool {
	return kind == catalogV2ProductsRoute || kind == catalogV2ProductRoute || kind == catalogV2CollectionsRoute || kind == catalogV2CollectionRoute
}

// isPublicRoute: routes answered without a buyer capability (origin + BFF key only).
func isPublicRoute(kind routeKind) bool {
	return kind == mediaRoute || kind == collectionMediaRoute || isCatalogV2(kind) || kind == primaryOriginRoute
}

type v2Store struct {
	Name     string `json:"name"`
	Currency string `json:"currency"`
}
type v2Card struct {
	ID           string  `json:"id"`
	Slug         string  `json:"slug"`
	Title        string  `json:"title"`
	PriceMin     int64   `json:"price_min_minor"`
	PriceMax     int64   `json:"price_max_minor"`
	CompareAtMin *int64  `json:"compare_at_min_minor"`
	CoverImageID *string `json:"cover_image_id"`
	InStock      bool    `json:"in_stock"`
}
type v2List struct {
	Store    v2Store  `json:"store"`
	Products []v2Card `json:"products"`
	Next     *string  `json:"next"`
}
type v2Image struct {
	ID     string `json:"id"`
	Width  *int   `json:"width"`
	Height *int   `json:"height"`
}
type v2Axis struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}
type v2Variant struct {
	SKUID        string   `json:"sku_id"`
	Title        string   `json:"title"`
	OptionValues []string `json:"option_values"`
	Price        int64    `json:"price_minor"`
	CompareAt    *int64   `json:"compare_at_minor"`
	Stock        string   `json:"stock"`
}
type v2CollectionRef struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
}
type v2Detail struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description"`
	SEO         struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	} `json:"seo"`
	Images      []v2Image         `json:"images"`
	Options     []v2Axis          `json:"options"`
	Variants    []v2Variant       `json:"variants"`
	Collections []v2CollectionRef `json:"collections"`
}
type v2CollectionCard struct {
	ID           string  `json:"id"` // storefront-v2 §A: builds /media/c/{id}/{image_id}
	Slug         string  `json:"slug"`
	Title        string  `json:"title"`
	ImageID      *string `json:"image_id"`
	ProductCount int     `json:"product_count"`
}
type v2Collections struct {
	Collections []v2CollectionCard `json:"collections"`
}
type v2CollectionOne struct {
	ID          string  `json:"id"`
	Slug        string  `json:"slug"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	ImageID     *string `json:"image_id"`
}

// v2Query is the parsed, canonical list query.
type v2Query struct {
	collection, q, sort string
	min, max            *int64
	offset, limit       int
}

func (q v2Query) digest() string {
	sum := sha256.Sum256([]byte(strings.Join([]string{q.collection, q.q, q.sort, optInt(q.min), optInt(q.max), strconv.Itoa(q.limit)}, "\x00")))
	return hex.EncodeToString(sum[:])
}

func optInt(v *int64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatInt(*v, 10)
}

type v2Cursor struct {
	Version int    `json:"v"`
	Filter  string `json:"f"`
	Offset  int    `json:"o"`
}

func encodeV2Cursor(filter string, offset int) string {
	raw, _ := json.Marshal(v2Cursor{1, filter, offset})
	return base64.RawURLEncoding.EncodeToString(raw)
}

// decodeV2Cursor accepts only a canonical cursor of this exact filter; the cursor carries a position, not authority.
func decodeV2Cursor(encoded, filter string) (int, error) {
	raw, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil || len(encoded) > 256 || len(raw) == 0 {
		return 0, command.ErrInvalid
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var c v2Cursor
	if err = dec.Decode(&c); err != nil || c.Version != 1 || c.Filter != filter || c.Offset < 1 || c.Offset > v2MaxOffset {
		return 0, command.ErrInvalid
	}
	if canonical, _ := json.Marshal(c); !bytes.Equal(canonical, raw) {
		return 0, command.ErrInvalid
	}
	return c.Offset, nil
}

// parseV2Query is strict like catalogRequest: one value per key, no empty value, known keys only.
func parseV2Query(raw string) (q v2Query, after string, err error) {
	q.limit = v2DefaultLimit
	if len(raw) > 2048 || strings.Contains(raw, ";") || strings.HasPrefix(raw, "&") || strings.HasSuffix(raw, "&") || strings.Contains(raw, "&&") {
		return q, "", command.ErrInvalid
	}
	values, perr := url.ParseQuery(raw)
	if perr != nil {
		return q, "", command.ErrInvalid
	}
	for name, vals := range values {
		if len(vals) != 1 || vals[0] == "" {
			return q, "", command.ErrInvalid
		}
		v := vals[0]
		switch name {
		case "collection":
			if len(v) > 80 || !v2Slug.MatchString(v) {
				return q, "", command.ErrInvalid
			}
			q.collection = v
		case "q":
			if utf8.RuneCountInString(v) > v2MaxQuery || !utf8.ValidString(v) || strings.ContainsFunc(v, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
				return q, "", command.ErrInvalid
			}
			q.q = v
		case "sort":
			if v != "newest" && v != "price_asc" && v != "price_desc" && v != "title" {
				return q, "", command.ErrInvalid
			}
			q.sort = v
		case "min", "max":
			n, nerr := parseMinor(v)
			if nerr != nil {
				return q, "", nerr
			}
			if name == "min" {
				q.min = &n
			} else {
				q.max = &n
			}
		case "limit":
			n, nerr := strconv.Atoi(v)
			if nerr != nil || len(v) > 2 || strings.Trim(v, "0123456789") != "" || n < 1 || n > v2MaxLimit {
				return q, "", command.ErrInvalid
			}
			q.limit = n
		case "after":
			after = v
		default:
			return q, "", command.ErrInvalid
		}
	}
	if q.min != nil && q.max != nil && *q.min > *q.max {
		return q, "", command.ErrInvalid
	}
	if after != "" {
		if q.offset, err = decodeV2Cursor(after, q.digest()); err != nil {
			return q, "", err
		}
	}
	return q, after, nil
}

func parseMinor(v string) (int64, error) {
	if len(v) > 13 || strings.Trim(v, "0123456789") != "" {
		return 0, command.ErrInvalid
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < 0 || n > v2MaxMinorMoney {
		return 0, command.ErrInvalid
	}
	return n, nil
}

// catalogV2Get answers every catalog/v2 read (GET only, no body, no credential).
func (h *handler) catalogV2Get(ctx context.Context, w http.ResponseWriter, r *http.Request, selected route) error {
	if err := noBody(r); err != nil {
		return err
	}
	origin := r.Header.Get("X-Commerce-Storefront-Origin")
	var query v2Query
	if selected.kind == catalogV2ProductsRoute {
		var err error
		if query, _, err = parseV2Query(r.URL.RawQuery); err != nil {
			return responseError{http.StatusUnprocessableEntity, "invalid_request"}
		}
	} else if r.URL.RawQuery != "" {
		return responseError{http.StatusForbidden, "forbidden"}
	}
	tx, err := h.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted, AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	var raw []byte
	switch selected.kind {
	case catalogV2ProductsRoute:
		err = tx.QueryRow(ctx, `SELECT catalog.buyer_v2_products($1::text,$2::text,$3::text,$4::text,$5::bigint,$6::bigint,$7::int,$8::int)`,
			origin, query.collection, query.q, query.sort, query.min, query.max, query.offset, query.limit).Scan(&raw)
	case catalogV2ProductRoute:
		err = tx.QueryRow(ctx, `SELECT catalog.buyer_v2_product($1::text,$2::text)`, origin, selected.slug).Scan(&raw)
	case catalogV2CollectionsRoute:
		err = tx.QueryRow(ctx, `SELECT catalog.buyer_v2_collections($1::text)`, origin).Scan(&raw)
	default:
		err = tx.QueryRow(ctx, `SELECT catalog.buyer_v2_collection($1::text,$2::text)`, origin, selected.slug).Scan(&raw)
	}
	if err != nil {
		return mapPublicCatalogError(err)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	out, err := projectV2(selected.kind, raw, query)
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "no-store")
	writeOK(w, out)
	return nil
}

// mapPublicCatalogError turns the definers' PT codes into the public 422/404; everything else is returned as is
// (classify makes it a generic 503, never a database message).
func mapPublicCatalogError(err error) error {
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

// strictDecode decodes definer jsonb into a closed struct: an unknown key means the database drifted from this
// contract, which is an operational error (503), never silently widened output.
func strictDecode(raw []byte, into any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	return dec.Decode(into)
}

func projectV2(kind routeKind, raw []byte, query v2Query) (any, error) {
	switch kind {
	case catalogV2ProductsRoute:
		var wire struct {
			Store      v2Store  `json:"store"`
			Products   []v2Card `json:"products"`
			NextOffset *int     `json:"next_offset"`
		}
		if err := strictDecode(raw, &wire); err != nil {
			return nil, err
		}
		out := v2List{Store: wire.Store, Products: wire.Products}
		if out.Products == nil {
			out.Products = []v2Card{}
		}
		if wire.NextOffset != nil {
			next := encodeV2Cursor(query.digest(), *wire.NextOffset)
			out.Next = &next
		}
		return out, nil
	case catalogV2ProductRoute:
		var d v2Detail
		if err := strictDecode(raw, &d); err != nil {
			return nil, err
		}
		if d.Images == nil {
			d.Images = []v2Image{}
		}
		if d.Options == nil {
			d.Options = []v2Axis{}
		}
		if d.Variants == nil {
			d.Variants = []v2Variant{}
		}
		if d.Collections == nil {
			d.Collections = []v2CollectionRef{}
		}
		for i := range d.Variants {
			if d.Variants[i].OptionValues == nil {
				d.Variants[i].OptionValues = []string{}
			}
		}
		return d, nil
	case catalogV2CollectionsRoute:
		var c v2Collections
		if err := strictDecode(raw, &c); err != nil {
			return nil, err
		}
		if c.Collections == nil {
			c.Collections = []v2CollectionCard{}
		}
		return c, nil
	}
	var c v2CollectionOne
	if err := strictDecode(raw, &c); err != nil {
		return nil, err
	}
	return c, nil
}

// collectionMediaGet serves GET /v1/buyer/media/c/{collection_id}/{image_id}: same trust shape, headers and PT
// mapping as mediaGet (media.go), over catalog.buyer_collection_image.
func (h *handler) collectionMediaGet(ctx context.Context, w http.ResponseWriter, r *http.Request, selected route) error {
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
	if err = tx.QueryRow(ctx, `SELECT content_type,bytes,sha256 FROM catalog.buyer_collection_image($1::text,$2::uuid,$3::uuid)`,
		r.Header.Get("X-Commerce-Storefront-Origin"), selected.id, selected.image).Scan(&contentType, &data, &digest); err != nil {
		return mapPublicCatalogError(err)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	writeImage(w, contentType, data, digest)
	return nil
}
