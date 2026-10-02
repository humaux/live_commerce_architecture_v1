// Package catalog owns the merchant-scoped catalog transaction slice: products (draft, active, archived), their slugs,
// SEO fields and option axes, SKUs (option values, compare-at price, price history), the wide product/SKU ledger read
// projection (contracts/admin-ledger-v1.md), the merchant product list/detail reads (productlist.go), merchant
// collections with their ordered membership and image (collections.go, migrations/0086), the purchase-entry read, and the
// merchant product photos (images.go, migrations/0082).
//
// It never writes stock (internal/inventory owns balances and the ledger), never opens a transaction (callers pass one from
// platform.WithScope), never trusts a caller-supplied tenant or store, and never serves buyers: the buyer reads are the
// catalog.buyer_* definers behind internal/buyerhttp and internal/storefront.
package catalog

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/command"
	"livecommerce/internal/pagination"
	"livecommerce/internal/platform"
)

var (
	skuCodePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)
	countryPattern = regexp.MustCompile(`^[A-Z]{2}$`)
	hsPattern      = regexp.MustCompile(`^[0-9]{6,12}$`)
)

// maxActiveSKUsPerProduct bounds the option matrix (3 axes of up to 50 values would be 125000 SKUs): the admin
// product editor and the buyer detail read both return a product's SKUs in one page.
const maxActiveSKUsPerProduct = 100

// Product statuses (migrations/0086): draft is merchant-only, active is buyer-visible, archived is retired.
const (
	StatusDraft    = "draft"
	StatusActive   = "active"
	StatusArchived = "archived"
)

type Product struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Status         string `json:"status"`
	Version        int64  `json:"version"`
	Slug           string `json:"slug"`
	SEOTitle       string `json:"seo_title"`
	SEODescription string `json:"seo_description"`
	// Options are the 0..3 option axes (always non-nil after a read).
	Options []OptionAxis `json:"options"`
}

// ProductInput is the create body and the legacy full-replace update. Status "" means draft (contract section A).
type ProductInput struct {
	Name            string       `json:"name"`
	Description     string       `json:"description"`
	ExpectedVersion int64        `json:"expected_version"`
	Status          string       `json:"status,omitempty"`
	Slug            string       `json:"slug,omitempty"`
	SEOTitle        string       `json:"seo_title,omitempty"`
	SEODescription  string       `json:"seo_description,omitempty"`
	Options         []OptionAxis `json:"options,omitempty"`
}

type SKU struct {
	ID            string `json:"id"`
	ProductID     string `json:"product_id"`
	Code          string `json:"code"`
	Status        string `json:"status"`
	Currency      string `json:"currency"`
	PriceMinor    int64  `json:"price_minor"`
	Version       int64  `json:"version"`
	WeightGrams   int64  `json:"weight_grams"`
	LengthMM      int64  `json:"length_mm"`
	WidthMM       int64  `json:"width_mm"`
	HeightMM      int64  `json:"height_mm"`
	OriginCountry string `json:"origin_country"`
	CustomsName   string `json:"customs_name"`
	HSCandidate   string `json:"hs_candidate"`
	// OptionValues align to the product's axes; Title is derived from them (never stored). CompareAtMinor is the
	// optional strike-through price, display only.
	OptionValues   []string `json:"option_values"`
	Title          string   `json:"title"`
	CompareAtMinor *int64   `json:"compare_at_minor"`
	// InventoryTracked is the A6 flag (migration 0109): true = tracked (checkout Begin locks and deducts stock, no
	// per-order cap); false = untracked (checkout skips the lock/deduct and enforces MaxPerOrder). MaxPerOrder is the
	// per-order cap, set only when untracked (NULL for tracked).
	InventoryTracked bool   `json:"inventory_tracked"`
	MaxPerOrder      *int64 `json:"max_per_order,omitempty"`
}

type SKUInput struct {
	ProductID     string `json:"product_id"`
	Code          string `json:"code"`
	PriceMinor    int64  `json:"price_minor"`
	WeightGrams   int64  `json:"weight_grams"`
	LengthMM      int64  `json:"length_mm"`
	WidthMM       int64  `json:"width_mm"`
	HeightMM      int64  `json:"height_mm"`
	OriginCountry string `json:"origin_country"`
	CustomsName   string `json:"customs_name"`
	HSCandidate   string `json:"hs_candidate"`
	// OptionValues: on create nil means axis-less; on update nil leaves the stored values untouched (an explicit
	// [] sets none). CompareAtMinor: update is a full replace like every other field, so absent clears it.
	OptionValues    []string `json:"option_values,omitempty"`
	CompareAtMinor  *int64   `json:"compare_at_minor,omitempty"`
	ExpectedVersion int64    `json:"expected_version"`
}

// PriceInput changes the price and, optionally, the compare-at price in the same version bump (so the strike-through
// can never be left below the price). CompareAt: absent = unchanged, null = clear, number = set.
type PriceInput struct {
	PriceMinor      int64      `json:"price_minor"`
	ExpectedVersion int64      `json:"expected_version"`
	CompareAt       Opt[int64] `json:"compare_at_minor,omitzero"`
}

// productColumns is the one product projection; productFields is its Scan target list.
const productColumns = `id::text,name,description,status,version,slug,seo_title,seo_description,options`

func productFields(p *Product) []any {
	return []any{&p.ID, &p.Name, &p.Description, &p.Status, &p.Version, &p.Slug, &p.SEOTitle, &p.SEODescription, &p.Options}
}

func finishProduct(p *Product) {
	if p.Options == nil {
		p.Options = []OptionAxis{}
	}
}

// CreateProduct inserts a product, draft unless the caller says active (contract storefront-v2 section A). The slug
// is the caller's, or generated from the name; a generated slug that is taken gets a numeric suffix, and a name with
// no ASCII letters or digits falls back to the id prefix (trigger catalog.products_default_slug). An explicit slug
// that is taken is ErrConflict.
func CreateProduct(ctx context.Context, tx pgx.Tx, scope platform.Scope, key string, in ProductInput) (out Product, err error) {
	if !validProductInput(in, false) {
		return out, command.ErrInvalid
	}
	status := in.Status
	if status == "" {
		status = StatusDraft
	}
	options := in.Options
	if options == nil {
		options = []OptionAxis{}
	}
	err = command.Run(ctx, tx, scope, "catalog.product.create", key, in, &out, func() error {
		slug := &in.Slug
		if in.Slug == "" {
			var err error
			if slug, err = freeSlug(ctx, tx, scope, "catalog.products", Slugify(in.Name)); err != nil {
				return err
			}
		}
		err := tx.QueryRow(ctx, `INSERT INTO catalog.products(tenant_id,store_id,name,description,status,slug,seo_title,seo_description,options)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING `+productColumns,
			scope.TenantID, scope.StoreID, in.Name, in.Description, status, slug, in.SEOTitle, in.SEODescription, options).Scan(productFields(&out)...)
		if err != nil {
			return err
		}
		finishProduct(&out)
		return command.Audit(ctx, tx, scope, "catalog.product.created")
	})
	// A replayed answer stored before 0086 has no options / option_values: keep the JSON shape non-null.
	finishProduct(&out)
	return out, mapError(err)
}

// UpdateProduct is the legacy full replace of name and description (kept for existing callers); PatchProduct is the
// general partial update behind the PATCH route.
func UpdateProduct(ctx context.Context, tx pgx.Tx, scope platform.Scope, key, id string, in ProductInput) (Product, error) {
	return PatchProduct(ctx, tx, scope, key, id, ProductPatch{Name: &in.Name, Description: &in.Description, ExpectedVersion: in.ExpectedVersion})
}

// ProductPatch is a partial product update: a nil field is left as it is. ExpectedVersion is mandatory.
type ProductPatch struct {
	Name            *string       `json:"name,omitempty"`
	Description     *string       `json:"description,omitempty"`
	Status          *string       `json:"status,omitempty"`
	Slug            *string       `json:"slug,omitempty"`
	SEOTitle        *string       `json:"seo_title,omitempty"`
	SEODescription  *string       `json:"seo_description,omitempty"`
	Options         *[]OptionAxis `json:"options,omitempty"`
	ExpectedVersion int64         `json:"expected_version"`
}

func (p ProductPatch) empty() bool {
	return p.Name == nil && p.Description == nil && p.Status == nil && p.Slug == nil && p.SEOTitle == nil && p.SEODescription == nil && p.Options == nil
}

// PatchProduct applies a partial update under the product row lock and the optimistic version. Changing the option
// axes is refused (ErrConflict) while an active SKU would no longer align with them: the merchant must archive or
// re-key those SKUs first, so a buyer can never see a variant whose values are not in the axes.
func PatchProduct(ctx context.Context, tx pgx.Tx, scope platform.Scope, key, id string, in ProductPatch) (out Product, err error) {
	if !command.ValidID(id) || in.ExpectedVersion < 1 || in.empty() || !validPatch(in) {
		return out, command.ErrInvalid
	}
	request := struct {
		ID string `json:"id"`
		ProductPatch
	}{id, in}
	err = command.Run(ctx, tx, scope, "catalog.product.update", key, request, &out, func() error {
		if err := lockProduct(ctx, tx, scope, id); err != nil {
			return err
		}
		var cur Product
		if err := tx.QueryRow(ctx, `SELECT `+productColumns+` FROM catalog.products WHERE tenant_id=$1 AND store_id=$2 AND id=$3`,
			scope.TenantID, scope.StoreID, id).Scan(productFields(&cur)...); err != nil {
			return err
		}
		if cur.Version != in.ExpectedVersion {
			return command.ErrConflict
		}
		if in.Options != nil {
			if err := optionsFitSKUs(ctx, tx, scope, id, *in.Options); err != nil {
				return err
			}
			cur.Options = *in.Options
		}
		for _, f := range []struct {
			set *string
			dst *string
		}{{in.Name, &cur.Name}, {in.Description, &cur.Description}, {in.Status, &cur.Status}, {in.Slug, &cur.Slug}, {in.SEOTitle, &cur.SEOTitle}, {in.SEODescription, &cur.SEODescription}} {
			if f.set != nil {
				*f.dst = *f.set
			}
		}
		if cur.Options == nil {
			cur.Options = []OptionAxis{}
		}
		err := tx.QueryRow(ctx, `UPDATE catalog.products SET name=$3,description=$4,status=$5,slug=$6,seo_title=$7,seo_description=$8,options=$9,version=version+1,updated_at=clock_timestamp()
			WHERE tenant_id=$1 AND store_id=$2 AND id=$10 AND version=$11 RETURNING `+productColumns,
			scope.TenantID, scope.StoreID, cur.Name, cur.Description, cur.Status, cur.Slug, cur.SEOTitle, cur.SEODescription, cur.Options, id, in.ExpectedVersion).Scan(productFields(&out)...)
		if err != nil {
			return err
		}
		finishProduct(&out)
		return command.Audit(ctx, tx, scope, "catalog.product.updated")
	})
	// A replayed answer stored before 0086 has no options / option_values: keep the JSON shape non-null.
	finishProduct(&out)
	return out, mapVersionError(err)
}

func ArchiveProduct(ctx context.Context, tx pgx.Tx, scope platform.Scope, key, id string, expectedVersion int64) (out Product, err error) {
	if !command.ValidID(id) || expectedVersion < 1 {
		return out, command.ErrInvalid
	}
	request := struct {
		ID              string `json:"id"`
		ExpectedVersion int64  `json:"expected_version"`
	}{id, expectedVersion}
	err = command.Run(ctx, tx, scope, "catalog.product.archive", key, request, &out, func() error {
		// LOCK: catalog archive and inventory readers acquire product before SKU.
		if err := lockProduct(ctx, tx, scope, id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT id FROM catalog.skus WHERE tenant_id=$1 AND store_id=$2 AND product_id=$3 ORDER BY id FOR UPDATE`, scope.TenantID, scope.StoreID, id); err != nil {
			return err
		}
		err := tx.QueryRow(ctx, `UPDATE catalog.products SET status='archived',version=version+1,updated_at=clock_timestamp()
			WHERE tenant_id=$1 AND store_id=$2 AND id=$3 AND version=$4
			RETURNING `+productColumns, scope.TenantID, scope.StoreID, id, expectedVersion).Scan(productFields(&out)...)
		if err != nil {
			return err
		}
		finishProduct(&out)
		return command.Audit(ctx, tx, scope, "catalog.product.archived")
	})
	// A replayed answer stored before 0086 has no options / option_values: keep the JSON shape non-null.
	finishProduct(&out)
	return out, mapVersionError(err)
}

// GetProduct reads one product (merchant view, any status). A product of another store is ErrNotFound.
func GetProduct(ctx context.Context, tx pgx.Tx, scope platform.Scope, id string) (Product, error) {
	var p Product
	if !validScope(tx, scope) || !command.ValidID(id) {
		return p, command.ErrInvalid
	}
	err := tx.QueryRow(ctx, `SELECT `+productColumns+` FROM catalog.products WHERE tenant_id=$1 AND store_id=$2 AND id=$3`,
		scope.TenantID, scope.StoreID, id).Scan(productFields(&p)...)
	finishProduct(&p)
	return p, mapError(err)
}

func ListProducts(ctx context.Context, tx pgx.Tx, scope platform.Scope) ([]Product, error) {
	page, err := ListProductsPage(ctx, tx, scope, pagination.Request{Limit: 100})
	return page.Items, err
}

func ListProductsPage(ctx context.Context, tx pgx.Tx, scope platform.Scope, request pagination.Request) (pagination.Page[Product], error) {
	page := pagination.Page[Product]{Items: make([]Product, 0)}
	if !validScope(tx, scope) {
		return page, command.ErrInvalid
	}
	binding := pagination.Binding{TenantID: scope.TenantID, StoreID: scope.StoreID, Collection: "products"}
	limit, after, err := pagination.Decode(request, binding, 1)
	if err != nil {
		return page, err
	}
	args := []any{scope.TenantID, scope.StoreID}
	if len(after) == 1 {
		args = append(args, after[0])
	}
	query := `SELECT ` + productColumns + ` FROM catalog.products WHERE tenant_id=$1 AND store_id=$2` + keysetID(after, 3) + ` ORDER BY id LIMIT $` + strconv.Itoa(len(args)+1)
	args = append(args, limit+1)
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return page, mapError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var p Product
		if err := rows.Scan(productFields(&p)...); err != nil {
			return page, err
		}
		finishProduct(&p)
		page.Items = append(page.Items, p)
	}
	if err := rows.Err(); err != nil {
		return page, mapError(err)
	}
	if len(page.Items) <= limit {
		return page, nil
	}
	page.Items = page.Items[:limit]
	page.NextCursor, err = pagination.Encode(binding, []string{page.Items[len(page.Items)-1].ID})
	return page, err
}

// skuColumns is the one SKU projection; skuFields is its Scan target list (same order).
const skuColumns = `id::text,product_id::text,code,status,currency,price_minor,version,weight_grams,length_mm,width_mm,height_mm,origin_country,customs_name,hs_candidate,option_values,compare_at_minor,inventory_tracked,max_per_order`

func CreateSKU(ctx context.Context, tx pgx.Tx, scope platform.Scope, key string, in SKUInput) (out SKU, err error) {
	if !validSKUInput(in, false) {
		return out, command.ErrInvalid
	}
	values := in.OptionValues
	if values == nil {
		values = []string{}
	}
	err = command.Run(ctx, tx, scope, "catalog.sku.create", key, in, &out, func() error {
		axes, err := editableProduct(ctx, tx, scope, in.ProductID)
		if err != nil {
			return err
		}
		if !valuesFit(axes, values) {
			return command.ErrInvalid
		}
		var active int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM catalog.skus WHERE tenant_id=$1 AND store_id=$2 AND product_id=$3 AND status='active'`,
			scope.TenantID, scope.StoreID, in.ProductID).Scan(&active); err != nil {
			return err
		}
		if active >= maxActiveSKUsPerProduct {
			return command.ErrConflict
		}
		currency, err := storeCurrency(ctx, tx, scope)
		if err != nil {
			return err
		}
		if err := checkWholeTWD(currency, in.PriceMinor, in.CompareAtMinor); err != nil {
			return err
		}
		err = tx.QueryRow(ctx, `INSERT INTO catalog.skus(tenant_id,store_id,product_id,code,currency,price_minor,weight_grams,length_mm,width_mm,height_mm,origin_country,customs_name,hs_candidate,option_values,compare_at_minor)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
			RETURNING `+skuColumns,
			scope.TenantID, scope.StoreID, in.ProductID, in.Code, currency, in.PriceMinor, in.WeightGrams, in.LengthMM, in.WidthMM, in.HeightMM, in.OriginCountry, in.CustomsName, in.HSCandidate, values, in.CompareAtMinor).Scan(skuFields(&out)...)
		if err != nil {
			return err
		}
		finishSKU(&out)
		if err := appendPriceHistory(ctx, tx, scope, out); err != nil {
			return err
		}
		return command.Audit(ctx, tx, scope, "catalog.sku.created")
	})
	// A replayed answer stored before 0086 has no options / option_values: keep the JSON shape non-null.
	finishSKU(&out)
	return out, mapError(err)
}

func UpdateSKU(ctx context.Context, tx pgx.Tx, scope platform.Scope, key, id string, in SKUInput) (out SKU, err error) {
	if !command.ValidID(id) || !validSKUInput(in, true) {
		return out, command.ErrInvalid
	}
	request := struct {
		ID string `json:"id"`
		SKUInput
	}{id, in}
	err = command.Run(ctx, tx, scope, "catalog.sku.update", key, request, &out, func() error {
		current, err := lockSKU(ctx, tx, scope, id)
		if err != nil {
			return err
		}
		if current.ProductID != in.ProductID || current.PriceMinor != in.PriceMinor || current.Version != in.ExpectedVersion {
			return command.ErrConflict
		}
		if current.Status != "active" {
			return command.ErrConflict
		}
		if in.OptionValues != nil {
			axes, err := productAxes(ctx, tx, scope, in.ProductID)
			if err != nil {
				return err
			}
			if !valuesFit(axes, in.OptionValues) {
				return command.ErrInvalid
			}
		}
		err = tx.QueryRow(ctx, `UPDATE catalog.skus SET code=$3,weight_grams=$4,length_mm=$5,width_mm=$6,height_mm=$7,origin_country=$8,customs_name=$9,hs_candidate=$10,option_values=coalesce($13::text[],option_values),compare_at_minor=$14,version=version+1,updated_at=clock_timestamp()
			WHERE tenant_id=$1 AND store_id=$2 AND id=$11 AND version=$12
			RETURNING `+skuColumns,
			scope.TenantID, scope.StoreID, in.Code, in.WeightGrams, in.LengthMM, in.WidthMM, in.HeightMM, in.OriginCountry, in.CustomsName, in.HSCandidate, id, in.ExpectedVersion, in.OptionValues, in.CompareAtMinor).Scan(skuFields(&out)...)
		if err != nil {
			return err
		}
		finishSKU(&out)
		return command.Audit(ctx, tx, scope, "catalog.sku.updated")
	})
	// A replayed answer stored before 0086 has no options / option_values: keep the JSON shape non-null.
	finishSKU(&out)
	return out, mapVersionError(err)
}

func SetSKUPrice(ctx context.Context, tx pgx.Tx, scope platform.Scope, key, id string, in PriceInput) (out SKU, err error) {
	if !command.ValidID(id) || in.PriceMinor < 0 || in.PriceMinor > command.MaxMoney || in.ExpectedVersion < 1 ||
		(in.CompareAt.Val != nil && (*in.CompareAt.Val <= in.PriceMinor || *in.CompareAt.Val > command.MaxMoney)) {
		return out, command.ErrInvalid
	}
	request := struct {
		ID string `json:"id"`
		PriceInput
	}{id, in}
	err = command.Run(ctx, tx, scope, "catalog.sku.set_price", key, request, &out, func() error {
		current, err := lockSKU(ctx, tx, scope, id)
		if err != nil {
			return err
		}
		if current.Version != in.ExpectedVersion || current.Status != "active" {
			return command.ErrConflict
		}
		compare := current.CompareAtMinor
		if in.CompareAt.Set {
			compare = in.CompareAt.Val
		}
		// compare-at must stay strictly above the new price (SQL CHECK repeats it): a price rise past the strike-through
		// is a merchant error to fix by sending compare_at_minor (or null) in the same request.
		if compare != nil && *compare <= in.PriceMinor {
			return command.ErrInvalid
		}
		if err := checkWholeTWD(current.Currency, in.PriceMinor, compare); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `UPDATE catalog.skus SET price_minor=$3,compare_at_minor=$6,version=version+1,updated_at=clock_timestamp()
			WHERE tenant_id=$1 AND store_id=$2 AND id=$4 AND version=$5
			RETURNING `+skuColumns, scope.TenantID, scope.StoreID, in.PriceMinor, id, in.ExpectedVersion, compare).Scan(skuFields(&out)...); err != nil {
			return err
		}
		finishSKU(&out)
		if err := appendPriceHistory(ctx, tx, scope, out); err != nil {
			return err
		}
		return command.Audit(ctx, tx, scope, "catalog.sku.price_changed")
	})
	// A replayed answer stored before 0086 has no options / option_values: keep the JSON shape non-null.
	finishSKU(&out)
	return out, mapVersionError(err)
}

func ArchiveSKU(ctx context.Context, tx pgx.Tx, scope platform.Scope, key, id string, expectedVersion int64) (out SKU, err error) {
	if !command.ValidID(id) || expectedVersion < 1 {
		return out, command.ErrInvalid
	}
	request := struct {
		ID              string `json:"id"`
		ExpectedVersion int64  `json:"expected_version"`
	}{id, expectedVersion}
	err = command.Run(ctx, tx, scope, "catalog.sku.archive", key, request, &out, func() error {
		current, err := lockSKU(ctx, tx, scope, id)
		if err != nil {
			return err
		}
		if current.Version != expectedVersion {
			return command.ErrConflict
		}
		err = tx.QueryRow(ctx, `UPDATE catalog.skus SET status='archived',version=version+1,updated_at=clock_timestamp()
			WHERE tenant_id=$1 AND store_id=$2 AND id=$3 AND version=$4
			RETURNING `+skuColumns, scope.TenantID, scope.StoreID, id, expectedVersion).Scan(skuFields(&out)...)
		if err != nil {
			return err
		}
		finishSKU(&out)
		return command.Audit(ctx, tx, scope, "catalog.sku.archived")
	})
	// A replayed answer stored before 0086 has no options / option_values: keep the JSON shape non-null.
	finishSKU(&out)
	return out, mapVersionError(err)
}

func ListSKUs(ctx context.Context, tx pgx.Tx, scope platform.Scope, productID string) ([]SKU, error) {
	page, err := ListSKUsPage(ctx, tx, scope, productID, pagination.Request{Limit: 100})
	return page.Items, err
}

func ListSKUsPage(ctx context.Context, tx pgx.Tx, scope platform.Scope, productID string, request pagination.Request) (pagination.Page[SKU], error) {
	page := pagination.Page[SKU]{Items: make([]SKU, 0)}
	if !validScope(tx, scope) || !command.ValidID(productID) {
		return page, command.ErrInvalid
	}
	// A read-only parent visibility check must not hold a row write lock until
	// the caller's transaction ends. Mutation helpers retain their own locks.
	var visible bool
	if err := tx.QueryRow(ctx, `SELECT true FROM catalog.products WHERE tenant_id=$1 AND store_id=$2 AND id=$3`, scope.TenantID, scope.StoreID, productID).Scan(&visible); err != nil {
		return page, mapError(err)
	}
	binding := pagination.Binding{TenantID: scope.TenantID, StoreID: scope.StoreID, Collection: "skus", ParentID: productID}
	limit, after, err := pagination.Decode(request, binding, 1)
	if err != nil {
		return page, err
	}
	query := `SELECT ` + skuColumns + ` FROM catalog.skus WHERE tenant_id=$1 AND store_id=$2 AND product_id=$3` + keysetID(after, 4) + ` ORDER BY id LIMIT $` + placeholder(4+len(after))
	args := []any{scope.TenantID, scope.StoreID, productID}
	if len(after) == 1 {
		args = append(args, after[0])
	}
	args = append(args, limit+1)
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return page, mapError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var sku SKU
		if err := rows.Scan(skuFields(&sku)...); err != nil {
			return page, err
		}
		finishSKU(&sku)
		page.Items = append(page.Items, sku)
	}
	if err := rows.Err(); err != nil {
		return page, mapError(err)
	}
	if len(page.Items) <= limit {
		return page, nil
	}
	page.Items = page.Items[:limit]
	page.NextCursor, err = pagination.Encode(binding, []string{page.Items[len(page.Items)-1].ID})
	return page, err
}

func keysetID(after []string, position int) string {
	if len(after) == 1 {
		return ` AND id>$` + strconv.Itoa(position) + `::uuid`
	}
	return ``
}
func placeholder(number int) string { return strconv.Itoa(number) }

func lockProduct(ctx context.Context, tx pgx.Tx, scope platform.Scope, id string) error {
	var ignored string
	err := tx.QueryRow(ctx, `SELECT id::text FROM catalog.products WHERE tenant_id=$1 AND store_id=$2 AND id=$3 FOR UPDATE`, scope.TenantID, scope.StoreID, id).Scan(&ignored)
	return mapError(err)
}

// editableProduct locks the product row and returns its option axes. Draft and active products are editable (a
// merchant builds SKUs and photos on a draft before publishing); an archived product is ErrConflict.
func editableProduct(ctx context.Context, tx pgx.Tx, scope platform.Scope, id string) ([]OptionAxis, error) {
	var status string
	var axes []OptionAxis
	err := tx.QueryRow(ctx, `SELECT status,options FROM catalog.products WHERE tenant_id=$1 AND store_id=$2 AND id=$3 FOR UPDATE`, scope.TenantID, scope.StoreID, id).Scan(&status, &axes)
	if err != nil {
		return nil, mapError(err)
	}
	if status == StatusArchived {
		return nil, command.ErrConflict
	}
	return axes, nil
}

// productAxes reads the option axes of a product the caller already holds the lock on.
func productAxes(ctx context.Context, tx pgx.Tx, scope platform.Scope, id string) ([]OptionAxis, error) {
	var axes []OptionAxis
	err := tx.QueryRow(ctx, `SELECT options FROM catalog.products WHERE tenant_id=$1 AND store_id=$2 AND id=$3`, scope.TenantID, scope.StoreID, id).Scan(&axes)
	return axes, mapError(err)
}

// optionsFitSKUs refuses new axes that an active SKU of the product would not align with.
func optionsFitSKUs(ctx context.Context, tx pgx.Tx, scope platform.Scope, productID string, axes []OptionAxis) error {
	rows, err := tx.Query(ctx, `SELECT option_values FROM catalog.skus WHERE tenant_id=$1 AND store_id=$2 AND product_id=$3 AND status='active'`, scope.TenantID, scope.StoreID, productID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var values []string
		if err := rows.Scan(&values); err != nil {
			return err
		}
		if !valuesFit(axes, values) {
			return command.ErrConflict
		}
	}
	return rows.Err()
}

func lockSKU(ctx context.Context, tx pgx.Tx, scope platform.Scope, id string) (SKU, error) {
	var sku SKU
	// LOCK: product first prevents an archive racing a SKU mutation or inventory read.
	var productID string
	err := tx.QueryRow(ctx, `SELECT product_id::text FROM catalog.skus WHERE tenant_id=$1 AND store_id=$2 AND id=$3`, scope.TenantID, scope.StoreID, id).Scan(&productID)
	if err != nil {
		return sku, mapError(err)
	}
	if err = lockProduct(ctx, tx, scope, productID); err != nil {
		return sku, err
	}
	err = tx.QueryRow(ctx, `SELECT `+skuColumns+` FROM catalog.skus WHERE tenant_id=$1 AND store_id=$2 AND id=$3 FOR UPDATE`, scope.TenantID, scope.StoreID, id).Scan(skuFields(&sku)...)
	finishSKU(&sku)
	return sku, mapError(err)
}

func storeCurrency(ctx context.Context, tx pgx.Tx, scope platform.Scope) (string, error) {
	var c string
	err := tx.QueryRow(ctx, `SELECT currency FROM control.stores WHERE tenant_id=$1 AND id=$2`, scope.TenantID, scope.StoreID).Scan(&c)
	return c, mapError(err)
}

// checkWholeTWD refuses a TWD price or compare-at that is not a whole dollar (product-editor §f ruling 6). A currency
// other than TWD is never checked (minor units may be cents there); compare may be nil. This is a Go check, not a DB
// CHECK, so legacy non-whole TWD rows do not block migration 0109.
func checkWholeTWD(currency string, priceMinor int64, compare *int64) error {
	if currency != "TWD" {
		return nil
	}
	if priceMinor%100 != 0 || (compare != nil && *compare%100 != 0) {
		return ErrAmountNotWholeTWD
	}
	return nil
}

// appendPriceHistory keeps the create price (version 1) and later price changes
// in the same append-only stream, before the command result/audit can commit.
func appendPriceHistory(ctx context.Context, tx pgx.Tx, scope platform.Scope, sku SKU) error {
	_, err := tx.Exec(ctx, `INSERT INTO catalog.price_history(tenant_id,store_id,sku_id,version,price_minor,currency,principal_id)
		VALUES($1,$2,$3,$4,$5,$6,$7)`, scope.TenantID, scope.StoreID, sku.ID, sku.Version, sku.PriceMinor, sku.Currency, scope.PrincipalID)
	return err
}
func skuFields(s *SKU) []any {
	return []any{&s.ID, &s.ProductID, &s.Code, &s.Status, &s.Currency, &s.PriceMinor, &s.Version, &s.WeightGrams, &s.LengthMM, &s.WidthMM, &s.HeightMM, &s.OriginCountry, &s.CustomsName, &s.HSCandidate, &s.OptionValues, &s.CompareAtMinor, &s.InventoryTracked, &s.MaxPerOrder}
}

// finishSKU derives Title (never stored) and keeps OptionValues non-nil for JSON.
func finishSKU(s *SKU) {
	if s.OptionValues == nil {
		s.OptionValues = []string{}
	}
	s.Title = skuTitle(s.OptionValues)
}
func validScope(tx pgx.Tx, s platform.Scope) bool {
	return tx != nil && command.ValidID(s.TenantID) && command.ValidID(s.StoreID) && command.ValidID(s.PrincipalID)
}
func validProductInput(in ProductInput, updating bool) bool {
	return validName(in.Name) && validDescription(in.Description) && (!updating || in.ExpectedVersion >= 1) &&
		(in.Status == "" || in.Status == StatusDraft || in.Status == StatusActive) &&
		(in.Slug == "" || validSlug(in.Slug)) && validSEO(in.SEOTitle, in.SEODescription) && validOptions(in.Options)
}
func validPatch(in ProductPatch) bool {
	return (in.Name == nil || validName(*in.Name)) && (in.Description == nil || validDescription(*in.Description)) &&
		(in.Status == nil || *in.Status == StatusDraft || *in.Status == StatusActive || *in.Status == StatusArchived) &&
		(in.Slug == nil || validSlug(*in.Slug)) && (in.SEOTitle == nil || utf8.RuneCountInString(*in.SEOTitle) <= 70) &&
		(in.SEODescription == nil || utf8.RuneCountInString(*in.SEODescription) <= 160) && (in.Options == nil || validOptions(*in.Options))
}
func validName(n string) bool {
	return utf8.RuneCountInString(n) >= 1 && utf8.RuneCountInString(n) <= 120
}
func validDescription(d string) bool { return utf8.RuneCountInString(d) <= 8000 }
func validSEO(title, description string) bool {
	return utf8.RuneCountInString(title) <= 70 && utf8.RuneCountInString(description) <= 160
}
func validSKUInput(in SKUInput, updating bool) bool {
	return command.ValidID(in.ProductID) && skuCodePattern.MatchString(in.Code) && in.PriceMinor >= 0 && in.PriceMinor <= command.MaxMoney && in.WeightGrams >= 0 && in.WeightGrams <= command.MaxQuantity && in.LengthMM >= 0 && in.LengthMM <= 1_000_000 && in.WidthMM >= 0 && in.WidthMM <= 1_000_000 && in.HeightMM >= 0 && in.HeightMM <= 1_000_000 && (in.OriginCountry == "" || countryPattern.MatchString(in.OriginCountry)) && utf8.RuneCountInString(in.CustomsName) <= 240 && (in.HSCandidate == "" || hsPattern.MatchString(in.HSCandidate)) && (!updating || in.ExpectedVersion >= 1) && validCompareAt(in) && len(in.OptionValues) <= 3
}
func validCompareAt(in SKUInput) bool {
	return in.CompareAtMinor == nil || (*in.CompareAtMinor > in.PriceMinor && *in.CompareAtMinor <= command.MaxMoney)
}
func mapVersionError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return command.ErrConflict
	}
	return mapError(err)
}
func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return command.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return command.ErrConflict
		case "23503":
			return command.ErrNotFound
		case "22001", "22007", "22008", "22023", "22P02", "23514":
			return command.ErrInvalid
		}
	}
	return err
}
