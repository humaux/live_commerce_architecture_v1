package catalog

// document.go owns the product-editor document save command (docs/delivery/units/product-editor.md §f create and §g
// merge-patch edit; the A6 amendment of contracts/catalog-inventory-v1.md): two idempotent commands that write a product,
// its option axes, SKUs (with inventory_tracked/max_per_order), stock opening/target, keyword and collection membership in
// a single transaction, plus the bulk status and copy commands. Create (SaveProductDocument) takes the whole document;
// edit (SaveProductEdit) is a presence-aware merge patch — an absent field keeps its value, an absent SKU is untouched,
// and a SKU is archived only by an explicit active:false.
//
// Layering and cross-domain writes: the command runs under the merchant runtime (commerce_runtime) and writes catalog
// tables, live.keyword_library (the claims keyword vocabulary, via the shared pure grammar package; the DB CHECK is the
// authority and NormalizeKeyword its canonical form) and inventory.ledger (the sole balance writer via its trigger; this
// package never touches inventory.balances directly). Every write is parameterized and store-scoped by the GUCs set by
// platform.WithScope. The ledger UNIQUE (operation,command_key,warehouse,sku,kind) and the keyword UNIQUE(tenant,store,keyword)
// are the race backstops; the pre-checks below turn the common single-writer cases into the contract error codes.
//
// Operation naming: create is `product.save`; edit is `product.save:<id>` with the UUID hyphens stripped, because
// command.Run's operation grammar (`^[a-z][a-z0-9_.:]{0,79}$`, also the ops.command_results CHECK) admits no hyphen, so the
// id is embedded in its 32-hex form. The dedup key stays per-product and the ledger row still names the exact command.

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/claims/grammar"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

const (
	documentCreateOperation = "product.save"  // create: fixed operation
	documentEditOperation   = "product.save:" // edit: operation = product.save:<32-hex id>
	reasonOpeningStock      = "期初库存"          // create: initial on-hand
	reasonTargetStock       = "后台编辑"          // edit: server-computed target delta (ruling §f)
	auditDocumentSaved      = "catalog.product.saved"
	copyNameSuffix          = "（复制）" // copy: name suffix (§f ruling 5), 4 runes
)

// ProductDocumentInput is the create body (POST /products/document). ExpectedVersion must be 0 and ID empty; edits use
// ProductDocumentPatch (SaveProductEdit), never this type. WarehouseID is optional: empty resolves the store's single
// active warehouse when stock is written (required when the store has 0 or >1 active warehouses).
type ProductDocumentInput struct {
	ID             string             `json:"id,omitempty"`
	Name           string             `json:"name"`
	Description    string             `json:"description"`
	Status         string             `json:"status,omitempty"` // "" or "draft"; "active" publishes
	Slug           string             `json:"slug,omitempty"`
	SEOTitle       string             `json:"seo_title,omitempty"`
	SEODescription string             `json:"seo_description,omitempty"`
	Options        []OptionAxis       `json:"options,omitempty"`
	SKUs           []DocumentSKUInput `json:"skus"`
	CollectionIDs  []string           `json:"collection_ids,omitempty"`
	WarehouseID    string             `json:"warehouse_id,omitempty"`
	// WeightGrams and Length/Width/HeightMM are the logistics defaults (c6 "weight/dims"), applied to every SKU of the
	// document; 0 means unset. The editor's logistics section is folded and optional.
	WeightGrams     int64 `json:"weight_grams,omitempty"`
	LengthMM        int64 `json:"length_mm,omitempty"`
	WidthMM         int64 `json:"width_mm,omitempty"`
	HeightMM        int64 `json:"height_mm,omitempty"`
	ExpectedVersion int64 `json:"expected_version"`
}

// DocumentSKUInput is one SKU of the create document: a whole SKU. OptionValues is the full combination; Code is generated
// from the product slug when empty. Stock carries the tracked/untracked mode and the opening on-hand quantity. Keyword is
// the store keyword (cleared when empty). Active is nil or true -> active; false -> archived.
type DocumentSKUInput struct {
	ID             string         `json:"id,omitempty"`
	OptionValues   []string       `json:"option_values,omitempty"`
	Code           string         `json:"code,omitempty"`
	PriceMinor     int64          `json:"price_minor"`
	CompareAtMinor *int64         `json:"compare_at_minor,omitempty"`
	OriginCountry  string         `json:"origin_country,omitempty"`
	CustomsName    string         `json:"customs_name,omitempty"`
	HSCandidate    string         `json:"hs_candidate,omitempty"`
	Stock          *DocumentStock `json:"stock,omitempty"`
	Keyword        string         `json:"keyword,omitempty"`
	Active         *bool          `json:"active,omitempty"`
}

// DocumentStock is a SKU's stock block. Mode "" or "tracked" tracks the SKU (checkout locks/deducts); "untracked" skips
// the lock and requires MaxPerOrder 1..999. OpeningQty is the create initial on-hand; TargetQty is the edit final on-hand
// (the server computes the delta against the current balance, reason 后台编辑). An untracked SKU never carries a quantity.
type DocumentStock struct {
	Mode        string `json:"mode"`
	OpeningQty  *int64 `json:"opening_qty,omitempty"`
	TargetQty   *int64 `json:"target_qty,omitempty"`
	MaxPerOrder *int64 `json:"max_per_order,omitempty"`
}

// ProductDocument is the save result: the product plus its now-active SKUs in creation order.
type ProductDocument struct {
	Product
	SKUs []SKU `json:"skus"`
}

// BulkStatusInput sets the status of up to 100 products.
type BulkStatusInput struct {
	IDs    []string `json:"ids"`
	Status string   `json:"status"`
}

// BulkStatusItem is one product's outcome; Err is the refusal code (live_window_open / not_found) or empty on success.
type BulkStatusItem struct {
	ID     string `json:"id"`
	Status string `json:"status,omitempty"`
	Err    string `json:"error,omitempty"`
}

// CopyInput is the copy body (ExpectedVersion guards the source product).
type CopyInput struct {
	ExpectedVersion int64 `json:"expected_version"`
}

// ProductDocumentPatch is the merge-patch edit body (PUT /products/{id}/document, product-editor §g). Top-level fields are
// presence-aware: nil leaves the stored value, a pointer sets it (Slug "" keeps the current slug). CollectionIDs nil leaves
// membership alone; [] clears it. SKUs nil leaves every SKU untouched; present patches the id-carrying entries and creates
// the rest (an unmentioned active SKU is left alone — archiving is only ever an explicit active:false). WeightGrams and
// Length/Width/HeightMM are create-only (§g.3): any of them present in an edit is ErrInvalid, so they stay fields here to
// reach the documented 422 instead of a bare unknown-field 400.
type ProductDocumentPatch struct {
	ID              string              `json:"id,omitempty"`
	Name            *string             `json:"name,omitempty"`
	Description     *string             `json:"description,omitempty"`
	Status          *string             `json:"status,omitempty"` // nil keeps; "draft"/"active" sets (never "archived")
	Slug            *string             `json:"slug,omitempty"`
	SEOTitle        *string             `json:"seo_title,omitempty"`
	SEODescription  *string             `json:"seo_description,omitempty"`
	Options         *[]OptionAxis       `json:"options,omitempty"`
	SKUs            *[]DocumentSKUPatch `json:"skus,omitempty"`
	CollectionIDs   *[]string           `json:"collection_ids,omitempty"`
	WeightGrams     *int64              `json:"weight_grams,omitempty"` // create-only; present in an edit -> ErrInvalid (§g.3)
	LengthMM        *int64              `json:"length_mm,omitempty"`    // create-only; present in an edit -> ErrInvalid (§g.3)
	WidthMM         *int64              `json:"width_mm,omitempty"`     // create-only; present in an edit -> ErrInvalid (§g.3)
	HeightMM        *int64              `json:"height_mm,omitempty"`    // create-only; present in an edit -> ErrInvalid (§g.3)
	ExpectedVersion int64               `json:"expected_version"`
}

// DocumentSKUPatch is one entry of the edit's skus array. ID present patches that SKU in place (only the fields present;
// active:false archives and releases its keyword); ID absent creates a new SKU (validated exactly like a create SKU and
// using opening_qty, not target_qty). Stock reuses DocumentStock: on an existing SKU only target_qty is meaningful (present,
// including 0, sets the final on-hand), on a new SKU only opening_qty. Keyword is Opt: absent keeps, null or "" clears, a
// value sets. CompareAtMinor is Opt: absent keeps, null clears, a value sets. PriceMinor is required on a new SKU and, when
// present on an existing one, must stay below CompareAtMinor.
type DocumentSKUPatch struct {
	ID             string         `json:"id,omitempty"`
	OptionValues   *[]string      `json:"option_values,omitempty"`
	PriceMinor     *int64         `json:"price_minor,omitempty"`
	CompareAtMinor Opt[int64]     `json:"compare_at_minor,omitzero"`
	OriginCountry  *string        `json:"origin_country,omitempty"`
	CustomsName    *string        `json:"customs_name,omitempty"`
	HSCandidate    *string        `json:"hs_candidate,omitempty"`
	Stock          *DocumentStock `json:"stock,omitempty"`
	Keyword        Opt[string]    `json:"keyword,omitzero"`
	Active         *bool          `json:"active,omitempty"`
	WeightGrams    *int64         `json:"weight_grams,omitempty"` // per-SKU logistics (§g.3)
	LengthMM       *int64         `json:"length_mm,omitempty"`
	WidthMM        *int64         `json:"width_mm,omitempty"`
	HeightMM       *int64         `json:"height_mm,omitempty"`
}

// SaveProductDocument writes a product document (create product.save) in one command.Run. It is a whole document: every SKU
// in the body is created and any failure — a taken keyword or slug, a bad amount — rolls the whole transaction back. Edit is
// SaveProductEdit (merge patch), not this function.
func SaveProductDocument(ctx context.Context, tx pgx.Tx, scope platform.Scope, key string, in ProductDocumentInput) (out ProductDocument, err error) {
	if in.ExpectedVersion != 0 || in.ID != "" {
		return out, command.ErrInvalid
	}
	if !validDocumentInput(in) {
		return out, command.ErrInvalid
	}
	status := in.Status
	if status == "" {
		status = StatusDraft
	}
	// Canonical request: the validated input with defaults the hash must see (DB-derived values — generated codes, the
	// resolved warehouse — stay out, so a replay with the same bytes still replays the first result).
	request := canonicalDocumentRequest(in, status)
	err = command.Run(ctx, tx, scope, documentCreateOperation, key, request, &out, func() error {
		currency, err := storeCurrency(ctx, tx, scope)
		if err != nil {
			return err
		}
		axes := in.Options
		if axes == nil {
			axes = []OptionAxis{}
		}
		slugPtr := &in.Slug
		if in.Slug == "" {
			slugPtr, err = freeSlug(ctx, tx, scope, "catalog.products", Slugify(in.Name))
			if err != nil {
				return err
			}
		}
		var prod Product
		err = tx.QueryRow(ctx, `INSERT INTO catalog.products(tenant_id,store_id,name,description,status,slug,seo_title,seo_description,options)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING `+productColumns,
			scope.TenantID, scope.StoreID, in.Name, in.Description, status, slugPtr, in.SEOTitle, in.SEODescription, axes).Scan(productFields(&prod)...)
		if err != nil {
			return err
		}
		finishProduct(&prod)
		out.Product = prod
		warehouse := in.WarehouseID
		for _, entry := range in.SKUs {
			stock := normalizeStock(entry.Stock)
			values := entry.OptionValues
			if values == nil {
				values = []string{}
			}
			if !valuesFit(axes, values) {
				return command.ErrInvalid
			}
			if err := checkWholeTWD(currency, entry.PriceMinor, entry.CompareAtMinor); err != nil {
				return err
			}
			sku, created, err := writeSKU(ctx, tx, scope, prod.ID, prod.Slug, currency, entry, values, stock, in.WeightGrams, in.LengthMM, in.WidthMM, in.HeightMM)
			if err != nil {
				return err
			}
			// Create writes opening_qty only; a nonzero opening writes the initial balance, an opening of 0 writes nothing.
			if created && stock.tracked && stock.qty != nil && *stock.qty != 0 {
				if warehouse == "" {
					if warehouse, err = resolveWarehouse(ctx, tx, scope, ""); err != nil {
						return err
					}
				}
				if err := writeOpeningStock(ctx, tx, scope, warehouse, sku.ID, *stock.qty, documentCreateOperation, key); err != nil {
					return err
				}
			}
			if err := writeKeyword(ctx, tx, scope, sku.ID, entry.Keyword); err != nil {
				return err
			}
		}
		if err := setProductCollections(ctx, tx, scope, prod.ID, in.CollectionIDs); err != nil {
			return err
		}
		out.SKUs, err = loadActiveSKUsOrdered(ctx, tx, scope, prod.ID)
		if err != nil {
			return err
		}
		return command.Audit(ctx, tx, scope, auditDocumentSaved)
	})
	// A replayed answer stored before the document command had no options / option_values: keep the JSON shape non-null.
	finishProduct(&out.Product)
	return out, mapError(err)
}

// resolvedStock is the stock block resolved to tracked/max-per-order and the one quantity (opening on create, target on edit).
type resolvedStock struct {
	tracked     bool
	maxPerOrder *int64
	qty         *int64
}

func normalizeStock(s *DocumentStock) resolvedStock {
	if s == nil {
		return resolvedStock{tracked: true}
	}
	rs := resolvedStock{tracked: s.Mode != "untracked", maxPerOrder: s.MaxPerOrder}
	if s.OpeningQty != nil {
		rs.qty = s.OpeningQty
	} else if s.TargetQty != nil {
		rs.qty = s.TargetQty
	}
	return rs
}

func canonicalDocumentRequest(in ProductDocumentInput, status string) ProductDocumentInput {
	in.Status = status
	if in.Options == nil {
		in.Options = []OptionAxis{}
	}
	if in.CollectionIDs == nil {
		in.CollectionIDs = []string{}
	}
	skus := make([]DocumentSKUInput, len(in.SKUs))
	for i, e := range in.SKUs {
		if e.OptionValues == nil {
			e.OptionValues = []string{}
		}
		if e.Stock != nil {
			sc := *e.Stock
			if sc.Mode == "" {
				sc.Mode = "tracked"
			}
			e.Stock = &sc
		}
		skus[i] = e
	}
	in.SKUs = skus
	return in
}

// validDocumentInput is the create grammar. Callers already canonicalize status.
func validDocumentInput(in ProductDocumentInput) bool {
	if !validName(in.Name) || !validDescription(in.Description) ||
		(in.Status != "" && in.Status != StatusDraft && in.Status != StatusActive) ||
		(in.Slug != "" && !validSlug(in.Slug)) || !validSEO(in.SEOTitle, in.SEODescription) ||
		!validOptions(in.Options) || in.ExpectedVersion != 0 || in.ID != "" ||
		len(in.SKUs) < 1 || len(in.SKUs) > maxActiveSKUsPerProduct ||
		len(in.CollectionIDs) > maxCollectionProducts || (in.WarehouseID != "" && !command.ValidID(in.WarehouseID)) ||
		in.WeightGrams < 0 || in.WeightGrams > command.MaxQuantity ||
		in.LengthMM < 0 || in.LengthMM > 1_000_000 || in.WidthMM < 0 || in.WidthMM > 1_000_000 ||
		in.HeightMM < 0 || in.HeightMM > 1_000_000 {
		return false
	}
	seenValues := map[string]bool{}
	for _, e := range in.SKUs {
		if (e.ID != "" && !command.ValidID(e.ID)) ||
			(e.Code != "" && !skuCodePattern.MatchString(e.Code)) ||
			e.PriceMinor < 0 || e.PriceMinor > command.MaxMoney ||
			(e.CompareAtMinor != nil && (*e.CompareAtMinor <= e.PriceMinor || *e.CompareAtMinor > command.MaxMoney)) ||
			(e.OriginCountry != "" && !countryPattern.MatchString(e.OriginCountry)) ||
			utf8Count(e.CustomsName) > 240 || (e.HSCandidate != "" && !hsPattern.MatchString(e.HSCandidate)) ||
			len(e.OptionValues) > 3 {
			return false
		}
		if len(e.OptionValues) > 0 {
			k := strings.Join(e.OptionValues, "\x00")
			if seenValues[k] {
				return false // two document SKUs with the same option combination (the 0086 partial index also forbids it)
			}
			seenValues[k] = true
		}
		if e.Stock != nil {
			if e.Stock.Mode != "" && e.Stock.Mode != "tracked" && e.Stock.Mode != "untracked" {
				return false
			}
			untracked := e.Stock.Mode == "untracked"
			if untracked {
				if e.Stock.MaxPerOrder == nil || *e.Stock.MaxPerOrder < 1 || *e.Stock.MaxPerOrder > 999 {
					return false
				}
				if e.Stock.OpeningQty != nil || e.Stock.TargetQty != nil {
					return false
				}
			} else if e.Stock.MaxPerOrder != nil {
				return false // tracked SKUs carry no per-order cap
			}
			if e.Stock.TargetQty != nil {
				return false // create writes opening_qty only
			}
			if e.Stock.OpeningQty != nil && *e.Stock.OpeningQty < 0 {
				return false
			}
		}
	}
	return true
}

// SaveProductEdit applies a merge-patch edit (product.save:<id>, PUT products/{id}/document, product-editor §g) in one
// command.Run. Top-level fields are presence-aware (absent keeps the value; collection_ids: [] clears); skus absent leaves
// every SKU untouched, skus present patches the id-carrying entries and creates the rest, and an unmentioned SKU stays as
// it is. Archiving is only ever an explicit active:false. Any failure — a stale expected_version, an axis change that
// strands an active SKU, an edit to an open live window — rolls the whole transaction back.
func SaveProductEdit(ctx context.Context, tx pgx.Tx, scope platform.Scope, key, id string, in ProductDocumentPatch) (out ProductDocument, err error) {
	in.ID = id
	if !validDocumentPatch(in) {
		return out, command.ErrInvalid
	}
	operation := documentEditOperation + strings.ReplaceAll(id, "-", "")
	err = command.Run(ctx, tx, scope, operation, key, in, &out, func() error {
		currency, err := storeCurrency(ctx, tx, scope)
		if err != nil {
			return err
		}
		if err := lockProduct(ctx, tx, scope, id); err != nil {
			return err
		}
		var cur Product
		if err := tx.QueryRow(ctx, `SELECT `+productColumns+` FROM catalog.products WHERE tenant_id=$1 AND store_id=$2 AND id=$3`,
			scope.TenantID, scope.StoreID, id).Scan(productFields(&cur)...); err != nil {
			return mapError(err)
		}
		if cur.Version != in.ExpectedVersion {
			return command.ErrConflict
		}
		if cur.Status == StatusArchived {
			return command.ErrConflict
		}
		finishProduct(&cur)

		name, description, status, slug, seoTitle, seoDesc := cur.Name, cur.Description, cur.Status, cur.Slug, cur.SEOTitle, cur.SEODescription
		axes := cur.Options
		changed := []string{}
		if in.Name != nil {
			name, changed = *in.Name, append(changed, "catalog.product.edited.name")
		}
		if in.Description != nil {
			description, changed = *in.Description, append(changed, "catalog.product.edited.description")
		}
		if in.Status != nil {
			status, changed = *in.Status, append(changed, "catalog.product.edited.status")
		}
		if in.Slug != nil && *in.Slug != "" {
			slug, changed = *in.Slug, append(changed, "catalog.product.edited.slug")
		}
		if in.SEOTitle != nil {
			seoTitle, changed = *in.SEOTitle, append(changed, "catalog.product.edited.seo_title")
		}
		if in.SEODescription != nil {
			seoDesc, changed = *in.SEODescription, append(changed, "catalog.product.edited.seo_description")
		}
		if in.Options != nil {
			axes, changed = *in.Options, append(changed, "catalog.product.edited.options")
		}

		// §g.5: unlisting a product (status != active) while an OPEN claim window has an active offer on one of its SKUs
		// is refused live_window_open; listing to active is always allowed.
		if in.Status != nil && status != StatusActive {
			open, err := liveWindowOpenForProduct(ctx, tx, scope, id)
			if err != nil {
				return err
			}
			if open {
				return ErrLiveWindowOpen
			}
		}

		err = tx.QueryRow(ctx, `UPDATE catalog.products SET name=$3,description=$4,status=$5,slug=$6,seo_title=$7,seo_description=$8,options=$9,version=version+1,updated_at=clock_timestamp()
			WHERE tenant_id=$1 AND store_id=$2 AND id=$10 AND version=$11 RETURNING `+productColumns,
			scope.TenantID, scope.StoreID, name, description, status, slug, seoTitle, seoDesc, axes, id, in.ExpectedVersion).Scan(productFields(&out.Product)...)
		if err != nil {
			return mapError(err)
		}
		finishProduct(&out.Product)

		if in.Options != nil && in.SKUs == nil {
			// §g.1: an options change must not strand an active SKU — the fit check runs even when the patch
			// carries no skus array (applySKUPatch covers the in.SKUs != nil case).
			active, err := loadActiveSKUs(ctx, tx, scope, id)
			if err != nil {
				return err
			}
			for _, s := range active {
				if !valuesFit(axes, s.OptionValues) {
					return command.ErrInvalid
				}
			}
		}
		if in.SKUs != nil {
			skuChanged, err := applySKUPatch(ctx, tx, scope, id, out.Product.Slug, currency, axes, *in.SKUs, operation, key)
			if err != nil {
				return err
			}
			changed = append(changed, skuChanged...)
		}
		if in.CollectionIDs != nil {
			if err := setProductCollections(ctx, tx, scope, id, *in.CollectionIDs); err != nil {
				return err
			}
			changed = append(changed, "catalog.product.edited.collection_ids")
		}
		out.SKUs, err = loadActiveSKUsOrdered(ctx, tx, scope, id)
		if err != nil {
			return err
		}
		return auditDocumentEdit(ctx, tx, scope, changed)
	})
	finishProduct(&out.Product)
	return out, mapError(err)
}

// auditDocumentEdit writes one ops.audit_events row per changed field name (never a value): catalog.product.edited.<field>
// for top-level fields and catalog.sku.edited.<field> / catalog.sku.created for SKU changes. Replay never re-audits.
func auditDocumentEdit(ctx context.Context, tx pgx.Tx, scope platform.Scope, actions []string) error {
	sort.Strings(actions)
	for _, a := range actions {
		if err := command.Audit(ctx, tx, scope, a); err != nil {
			return err
		}
	}
	return nil
}

// liveWindowOpenForProduct reports whether an OPEN claim window has an active offer on one of the product's SKUs
// (product-editor §f ruling 3 / §g.5): the reason an unlist is refused live_window_open.
func liveWindowOpenForProduct(ctx context.Context, tx pgx.Tx, scope platform.Scope, productID string) (bool, error) {
	var open bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM live.claim_windows w
		JOIN live.offers o ON o.tenant_id=w.tenant_id AND o.store_id=w.store_id AND o.session_id=w.session_id
		JOIN catalog.skus s ON s.tenant_id=o.tenant_id AND s.store_id=o.store_id AND s.id=o.sku_id
		WHERE w.tenant_id=$1 AND w.store_id=$2 AND w.state='OPEN' AND o.active AND s.product_id=$3)`,
		scope.TenantID, scope.StoreID, productID).Scan(&open)
	return open, err
}

// liveWindowOpenForSKU reports whether an OPEN claim window has an active offer on skuID.
func liveWindowOpenForSKU(ctx context.Context, tx pgx.Tx, scope platform.Scope, skuID string) (bool, error) {
	var open bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM live.claim_windows w
		JOIN live.offers o ON o.tenant_id=w.tenant_id AND o.store_id=w.store_id AND o.session_id=w.session_id
		WHERE w.tenant_id=$1 AND w.store_id=$2 AND w.state='OPEN' AND o.active AND o.sku_id=$3)`,
		scope.TenantID, scope.StoreID, skuID).Scan(&open)
	return open, err
}

// validDocumentPatch is the edit grammar (§g): presence-aware top-level fields and per-SKU patches. Top-level logistics are
// create-only (§g.3); an id-carrying entry has a valid unique id with its present fields checked in isolation (cross-field
// checks that need the current value run at write time), and a new entry is validated by validateNewSKUPatch.
func validDocumentPatch(in ProductDocumentPatch) bool {
	if in.ExpectedVersion < 1 || !command.ValidID(in.ID) {
		return false
	}
	if in.WeightGrams != nil || in.LengthMM != nil || in.WidthMM != nil || in.HeightMM != nil {
		return false // §g.3: top-level logistics are create-only
	}
	if in.Name != nil && !validName(*in.Name) {
		return false
	}
	if in.Description != nil && !validDescription(*in.Description) {
		return false
	}
	if in.Status != nil && *in.Status != StatusDraft && *in.Status != StatusActive {
		return false
	}
	if in.Slug != nil && *in.Slug != "" && !validSlug(*in.Slug) {
		return false
	}
	if in.SEOTitle != nil && utf8Count(*in.SEOTitle) > 70 {
		return false
	}
	if in.SEODescription != nil && utf8Count(*in.SEODescription) > 160 {
		return false
	}
	if in.Options != nil && !validOptions(*in.Options) {
		return false
	}
	if in.CollectionIDs != nil && len(*in.CollectionIDs) > maxCollectionProducts {
		return false
	}
	if in.SKUs != nil && len(*in.SKUs) > maxActiveSKUsPerProduct {
		return false
	}
	seenIDs := map[string]bool{}
	if in.SKUs != nil {
		for i := range *in.SKUs {
			e := &(*in.SKUs)[i]
			if e.ID == "" {
				continue // a new SKU is validated by validateNewSKUPatch
			}
			if !command.ValidID(e.ID) || seenIDs[e.ID] || !validPatchSKUFields(e) {
				return false
			}
			seenIDs[e.ID] = true
		}
	}
	return true
}

// validPatchSKUFields checks the present fields of an id-carrying patch entry in isolation. Cross-field checks that need
// the current value (compare_at > price, whole-TWD, tracked/untracked cap rules) run at write time in patchExistingSKU.
func validPatchSKUFields(e *DocumentSKUPatch) bool {
	// An archive entry carries only id + active:false: any other field would be silently dropped, so it is refused.
	if e.Active != nil && !*e.Active && (e.OptionValues != nil || e.PriceMinor != nil || e.CompareAtMinor.Set ||
		e.OriginCountry != nil || e.CustomsName != nil || e.HSCandidate != nil || e.Stock != nil || e.Keyword.Set ||
		e.WeightGrams != nil || e.LengthMM != nil || e.WidthMM != nil || e.HeightMM != nil) {
		return false
	}
	if e.PriceMinor != nil && (*e.PriceMinor < 0 || *e.PriceMinor > command.MaxMoney) {
		return false
	}
	if e.CompareAtMinor.Set && e.CompareAtMinor.Val != nil && *e.CompareAtMinor.Val > command.MaxMoney {
		return false
	}
	if e.OriginCountry != nil && *e.OriginCountry != "" && !countryPattern.MatchString(*e.OriginCountry) {
		return false
	}
	if e.CustomsName != nil && utf8Count(*e.CustomsName) > 240 {
		return false
	}
	if e.HSCandidate != nil && *e.HSCandidate != "" && !hsPattern.MatchString(*e.HSCandidate) {
		return false
	}
	if e.OptionValues != nil && len(*e.OptionValues) > 3 {
		return false
	}
	if e.WeightGrams != nil && (*e.WeightGrams < 0 || *e.WeightGrams > command.MaxQuantity) {
		return false
	}
	for _, p := range []*int64{e.LengthMM, e.WidthMM, e.HeightMM} {
		if p != nil && (*p < 0 || *p > 1_000_000) {
			return false
		}
	}
	if e.Stock != nil {
		s := e.Stock
		if s.Mode != "" && s.Mode != "tracked" && s.Mode != "untracked" {
			return false
		}
		if s.OpeningQty != nil {
			return false // edit writes target_qty only (§g.2)
		}
		if s.TargetQty != nil && *s.TargetQty < 0 {
			return false
		}
		if s.MaxPerOrder != nil && (*s.MaxPerOrder < 1 || *s.MaxPerOrder > 999) {
			return false
		}
	}
	return true
}

// applySKUPatch applies the patch's SKU entries: id-carrying entries patch only the fields that are present (unmentioned
// fields keep their stored values; active:false archives and releases the keyword), entries without id are new SKUs with
// the create validation. After an options change every still-active SKU must fit the new axes and no two may share a
// combination, else the whole command is ErrInvalid. Returns the audit action names of the SKU changes.
func applySKUPatch(ctx context.Context, tx pgx.Tx, scope platform.Scope, productID, slug, currency string, axes []OptionAxis, entries []DocumentSKUPatch, operation, key string) ([]string, error) {
	existing, err := loadActiveSKUs(ctx, tx, scope, productID)
	if err != nil {
		return nil, err
	}
	// Resolve the final active option-value set for the fit + duplicate checks: existing SKUs kept active (with their patch
	// values) plus the new active SKUs. Archived SKUs are dropped before the check.
	final := map[string][]string{}
	for id, s := range existing {
		final[id] = s.OptionValues
	}
	newValues := [][]string{}
	for _, e := range entries {
		if e.ID == "" {
			if err := validateNewSKUPatch(e); err != nil {
				return nil, err
			}
			if e.Active == nil || *e.Active {
				values := []string{}
				if e.OptionValues != nil {
					values = *e.OptionValues
				}
				newValues = append(newValues, values)
			}
			continue
		}
		_, ok := existing[e.ID]
		if !ok {
			return nil, command.ErrNotFound
		}
		if e.Active != nil && !*e.Active {
			delete(final, e.ID)
		} else if e.OptionValues != nil {
			final[e.ID] = *e.OptionValues
		}
	}
	// The edit may not leave more active SKUs than create allows (ListSKUs and the detail read assume the cap).
	if len(final)+len(newValues) > maxActiveSKUsPerProduct {
		return nil, command.ErrInvalid
	}
	// Combination uniqueness applies to non-empty combinations only, as on create and in the 0086 partial index: an
	// axis-less product may hold several SKUs with no option values.
	seen := map[string]bool{}
	for _, values := range final {
		if !valuesFit(axes, values) {
			return nil, command.ErrInvalid // §g.1: an active SKU stranded by the new axes
		}
		if len(values) > 0 {
			k := strings.Join(values, "\x00")
			if seen[k] {
				return nil, command.ErrInvalid // two active SKUs with the same combination (§g.1; the 0086 index backstops)
			}
			seen[k] = true
		}
	}
	for _, values := range newValues {
		if !valuesFit(axes, values) {
			return nil, command.ErrInvalid
		}
		k := strings.Join(values, "\x00")
		if len(values) > 0 && seen[k] {
			return nil, command.ErrInvalid // two active SKUs with the same combination
		}
		seen[k] = true
	}

	// §g.5: archiving a SKU or touching the keyword of a SKU inside an OPEN window is refused per item, whole rollback.
	for _, e := range entries {
		if e.ID == "" {
			continue
		}
		if (e.Active != nil && !*e.Active) || e.Keyword.Set {
			open, err := liveWindowOpenForSKU(ctx, tx, scope, e.ID)
			if err != nil {
				return nil, err
			}
			if open {
				return nil, ErrLiveWindowOpen
			}
		}
	}

	warehouse := ""
	actions := []string{}
	for _, e := range entries {
		if e.ID == "" {
			values := []string{}
			if e.OptionValues != nil {
				values = *e.OptionValues
			}
			if !valuesFit(axes, values) {
				return nil, command.ErrInvalid // matches create: every SKU, active or archived, must fit the axes
			}
			price := int64(0)
			if e.PriceMinor != nil {
				price = *e.PriceMinor
			}
			var compare *int64
			if e.CompareAtMinor.Set {
				compare = e.CompareAtMinor.Val
			}
			if err := checkWholeTWD(currency, price, compare); err != nil {
				return nil, err
			}
			stock := (*DocumentStock)(nil)
			if e.Stock != nil {
				s := *e.Stock
				stock = &s
			}
			origin, customs, hs := "", "", ""
			if e.OriginCountry != nil {
				origin = *e.OriginCountry
			}
			if e.CustomsName != nil {
				customs = *e.CustomsName
			}
			if e.HSCandidate != nil {
				hs = *e.HSCandidate
			}
			weight, length, width, height := int64(0), int64(0), int64(0), int64(0)
			if e.WeightGrams != nil {
				weight = *e.WeightGrams
			}
			if e.LengthMM != nil {
				length = *e.LengthMM
			}
			if e.WidthMM != nil {
				width = *e.WidthMM
			}
			if e.HeightMM != nil {
				height = *e.HeightMM
			}
			entry := DocumentSKUInput{
				PriceMinor: price, CompareAtMinor: compare, OriginCountry: origin, CustomsName: customs, HSCandidate: hs,
				Stock: stock, Active: e.Active,
			}
			rs := normalizeStock(stock)
			sku, created, err := writeSKU(ctx, tx, scope, productID, slug, currency, entry, values, rs, weight, length, width, height)
			if err != nil {
				return nil, err
			}
			if created && rs.tracked && rs.qty != nil && *rs.qty != 0 {
				if warehouse == "" {
					if warehouse, err = resolveWarehouse(ctx, tx, scope, ""); err != nil {
						return nil, err
					}
				}
				if err := writeOpeningStock(ctx, tx, scope, warehouse, sku.ID, *rs.qty, operation, key); err != nil {
					return nil, err
				}
			}
			if e.Keyword.Set {
				kw := ""
				if e.Keyword.Val != nil {
					kw = *e.Keyword.Val
				}
				if err := writeKeyword(ctx, tx, scope, sku.ID, kw); err != nil {
					return nil, err
				}
			}
			actions = append(actions, "catalog.sku.created")
			continue
		}
		_, changed, err := patchExistingSKU(ctx, tx, scope, productID, currency, e, &warehouse, operation, key)
		if err != nil {
			return nil, err
		}
		actions = append(actions, changed...)
	}
	return actions, nil
}

// validateNewSKUPatch applies the create grammar to a patch entry without an id (§g.1: a new SKU is validated exactly like
// a create SKU): price_minor required, option_values <= 3, and the create stock rules (opening_qty only, untracked requires
// max_per_order 1..999 and never carries a quantity).
func validateNewSKUPatch(e DocumentSKUPatch) error {
	if e.PriceMinor == nil || *e.PriceMinor < 0 || *e.PriceMinor > command.MaxMoney {
		return command.ErrInvalid
	}
	if e.CompareAtMinor.Set && e.CompareAtMinor.Val != nil && (*e.CompareAtMinor.Val <= *e.PriceMinor || *e.CompareAtMinor.Val > command.MaxMoney) {
		return command.ErrInvalid
	}
	if e.OriginCountry != nil && *e.OriginCountry != "" && !countryPattern.MatchString(*e.OriginCountry) {
		return command.ErrInvalid
	}
	if e.CustomsName != nil && utf8Count(*e.CustomsName) > 240 {
		return command.ErrInvalid
	}
	if e.HSCandidate != nil && *e.HSCandidate != "" && !hsPattern.MatchString(*e.HSCandidate) {
		return command.ErrInvalid
	}
	if e.OptionValues != nil && len(*e.OptionValues) > 3 {
		return command.ErrInvalid
	}
	if e.WeightGrams != nil && (*e.WeightGrams < 0 || *e.WeightGrams > command.MaxQuantity) {
		return command.ErrInvalid
	}
	for _, p := range []*int64{e.LengthMM, e.WidthMM, e.HeightMM} {
		if p != nil && (*p < 0 || *p > 1_000_000) {
			return command.ErrInvalid
		}
	}
	if e.Stock != nil {
		s := e.Stock
		if s.Mode != "" && s.Mode != "tracked" && s.Mode != "untracked" {
			return command.ErrInvalid
		}
		untracked := s.Mode == "untracked"
		if untracked {
			if s.MaxPerOrder == nil || *s.MaxPerOrder < 1 || *s.MaxPerOrder > 999 {
				return command.ErrInvalid
			}
			if s.OpeningQty != nil || s.TargetQty != nil {
				return command.ErrInvalid
			}
		} else if s.MaxPerOrder != nil {
			return command.ErrInvalid // a tracked SKU carries no per-order cap
		}
		if s.TargetQty != nil {
			return command.ErrInvalid // a new SKU writes opening_qty only
		}
		if s.OpeningQty != nil && (*s.OpeningQty < 0 || *s.OpeningQty > command.MaxQuantity) {
			return command.ErrInvalid
		}
	}
	return nil
}

// patchExistingSKU locks one id-carrying entry's SKU and applies only the fields present in the entry; an active:false
// archives it and releases its keyword. The target_qty write (§g.2) resolves the store's single active warehouse through the
// caller's warehouse accumulator. Returns the audit action names of the SKU's changed fields.
func patchExistingSKU(ctx context.Context, tx pgx.Tx, scope platform.Scope, productID, currency string, e DocumentSKUPatch, warehouse *string, operation, key string) (SKU, []string, error) {
	var cur SKU
	err := tx.QueryRow(ctx, `SELECT `+skuColumns+` FROM catalog.skus WHERE tenant_id=$1 AND store_id=$2 AND id=$3 AND product_id=$4 AND status='active' FOR UPDATE`,
		scope.TenantID, scope.StoreID, e.ID, productID).Scan(skuFields(&cur)...)
	if err != nil {
		return cur, nil, mapError(err)
	}
	finishSKU(&cur)

	if e.Active != nil && !*e.Active {
		if err := archiveSKURow(ctx, tx, scope, cur.ID, cur.Version); err != nil {
			return cur, nil, err
		}
		cur.Status = StatusArchived
		cur.Version++
		if err := writeKeyword(ctx, tx, scope, cur.ID, ""); err != nil {
			return cur, nil, err
		}
		return cur, []string{"catalog.sku.edited.active"}, nil
	}

	changes := map[string]bool{}
	mark := func(field string) { changes["catalog.sku.edited."+field] = true }

	price := cur.PriceMinor
	if e.PriceMinor != nil {
		price = *e.PriceMinor
		mark("price_minor")
	}
	compare := cur.CompareAtMinor
	if e.CompareAtMinor.Set {
		compare = e.CompareAtMinor.Val
		mark("compare_at_minor")
	}
	values := cur.OptionValues
	if e.OptionValues != nil {
		values = *e.OptionValues
		mark("option_values")
	}
	weight := cur.WeightGrams
	if e.WeightGrams != nil {
		weight = *e.WeightGrams
		mark("weight_grams")
	}
	length := cur.LengthMM
	if e.LengthMM != nil {
		length = *e.LengthMM
		mark("length_mm")
	}
	width := cur.WidthMM
	if e.WidthMM != nil {
		width = *e.WidthMM
		mark("width_mm")
	}
	height := cur.HeightMM
	if e.HeightMM != nil {
		height = *e.HeightMM
		mark("height_mm")
	}
	origin := cur.OriginCountry
	if e.OriginCountry != nil {
		origin = *e.OriginCountry
		mark("origin_country")
	}
	customs := cur.CustomsName
	if e.CustomsName != nil {
		customs = *e.CustomsName
		mark("customs_name")
	}
	hs := cur.HSCandidate
	if e.HSCandidate != nil {
		hs = *e.HSCandidate
		mark("hs_candidate")
	}

	tracked := cur.InventoryTracked
	maxPerOrder := cur.MaxPerOrder
	var targetQty *int64
	if e.Stock != nil {
		s := e.Stock
		if s.Mode == "tracked" {
			tracked, maxPerOrder = true, nil
			mark("stock")
		} else if s.Mode == "untracked" {
			tracked = false
			mark("stock")
		}
		if s.MaxPerOrder != nil {
			maxPerOrder = s.MaxPerOrder
			mark("stock")
		}
		if s.TargetQty != nil {
			targetQty = s.TargetQty
			mark("stock")
		}
	}

	// resolved-value validation (only the present fields; the untouched values were already valid at create time).
	if price < 0 || price > command.MaxMoney || (compare != nil && (*compare <= price || *compare > command.MaxMoney)) {
		return cur, nil, command.ErrInvalid
	}
	if err := checkWholeTWD(currency, price, compare); err != nil {
		return cur, nil, err
	}
	if origin != "" && !countryPattern.MatchString(origin) {
		return cur, nil, command.ErrInvalid
	}
	if utf8Count(customs) > 240 {
		return cur, nil, command.ErrInvalid
	}
	if hs != "" && !hsPattern.MatchString(hs) {
		return cur, nil, command.ErrInvalid
	}
	if len(values) > 3 {
		return cur, nil, command.ErrInvalid
	}
	if weight < 0 || weight > command.MaxQuantity || length < 0 || length > 1_000_000 || width < 0 || width > 1_000_000 || height < 0 || height > 1_000_000 {
		return cur, nil, command.ErrInvalid
	}
	if !tracked {
		if maxPerOrder == nil || *maxPerOrder < 1 || *maxPerOrder > 999 {
			return cur, nil, command.ErrInvalid
		}
		if targetQty != nil {
			return cur, nil, command.ErrInvalid // an untracked SKU never carries a quantity
		}
	} else if maxPerOrder != nil {
		return cur, nil, command.ErrInvalid // a tracked SKU carries no per-order cap
	}
	if targetQty != nil && (*targetQty < 0 || *targetQty > command.MaxQuantity) {
		return cur, nil, command.ErrInvalid
	}

	var out SKU
	err = tx.QueryRow(ctx, `UPDATE catalog.skus SET price_minor=$3,compare_at_minor=$4,weight_grams=$5,length_mm=$6,width_mm=$7,height_mm=$8,origin_country=$9,customs_name=$10,hs_candidate=$11,option_values=$12,inventory_tracked=$13,max_per_order=$14,version=version+1,updated_at=clock_timestamp()
		WHERE tenant_id=$1 AND store_id=$2 AND id=$15 AND version=$16 RETURNING `+skuColumns,
		scope.TenantID, scope.StoreID, price, compare, weight, length, width, height, origin, customs, hs, values, tracked, maxPerOrder, cur.ID, cur.Version).Scan(skuFields(&out)...)
	if err != nil {
		return cur, nil, mapError(err)
	}
	finishSKU(&out)
	if cur.PriceMinor != price {
		if err := appendPriceHistory(ctx, tx, scope, out); err != nil {
			return cur, nil, err
		}
	}
	if e.Keyword.Set {
		mark("keyword")
		kw := ""
		if e.Keyword.Val != nil {
			kw = *e.Keyword.Val
		}
		if err := writeKeyword(ctx, tx, scope, out.ID, kw); err != nil {
			return cur, nil, err
		}
	}
	// §g.2: target_qty present, including 0, sets the final on-hand; absent leaves it alone.
	if targetQty != nil {
		if *warehouse == "" {
			var err error
			if *warehouse, err = resolveWarehouse(ctx, tx, scope, ""); err != nil {
				return cur, nil, err
			}
		}
		if err := writeTargetStock(ctx, tx, scope, *warehouse, out.ID, *targetQty, operation, key); err != nil {
			return cur, nil, err
		}
	}

	actions := make([]string, 0, len(changes))
	for a := range changes {
		actions = append(actions, a)
	}
	sort.Strings(actions)
	return out, actions, nil
}

// loadActiveSKUs returns the product's active SKUs keyed by id (for the merge-patch resolve + archive decision).
func loadActiveSKUs(ctx context.Context, tx pgx.Tx, scope platform.Scope, productID string) (map[string]SKU, error) {
	rows, err := tx.Query(ctx, `SELECT `+skuColumns+` FROM catalog.skus WHERE tenant_id=$1 AND store_id=$2 AND product_id=$3 AND status='active'`,
		scope.TenantID, scope.StoreID, productID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	out := map[string]SKU{}
	for rows.Next() {
		var s SKU
		if err := rows.Scan(skuFields(&s)...); err != nil {
			return nil, err
		}
		finishSKU(&s)
		out[s.ID] = s
	}
	return out, mapError(rows.Err())
}

func loadActiveSKUsOrdered(ctx context.Context, tx pgx.Tx, scope platform.Scope, productID string) ([]SKU, error) {
	rows, err := tx.Query(ctx, `SELECT `+skuColumns+` FROM catalog.skus WHERE tenant_id=$1 AND store_id=$2 AND product_id=$3 AND status='active' ORDER BY created_at,id LIMIT 200`,
		scope.TenantID, scope.StoreID, productID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	out := []SKU{}
	for rows.Next() {
		var s SKU
		if err := rows.Scan(skuFields(&s)...); err != nil {
			return nil, err
		}
		finishSKU(&s)
		out = append(out, s)
	}
	return out, mapError(rows.Err())
}

func archiveSKURow(ctx context.Context, tx pgx.Tx, scope platform.Scope, id string, version int64) error {
	_, err := tx.Exec(ctx, `UPDATE catalog.skus SET status='archived',version=version+1,updated_at=clock_timestamp()
		WHERE tenant_id=$1 AND store_id=$2 AND id=$3 AND version=$4`, scope.TenantID, scope.StoreID, id, version)
	return mapError(err)
}

// writeSKU inserts a new SKU (created=true) or updates an existing active SKU in place (created=false). Price history is
// appended on create and whenever price_minor actually changes (append-only, matching SetSKUPrice).
func writeSKU(ctx context.Context, tx pgx.Tx, scope platform.Scope, productID, slug, currency string, e DocumentSKUInput, values []string, stock resolvedStock, weight, length, width, height int64) (SKU, bool, error) {
	var out SKU
	active := e.Active == nil || *e.Active
	status := StatusActive
	if !active {
		status = StatusArchived
	}
	if e.ID != "" {
		var cur SKU
		err := tx.QueryRow(ctx, `SELECT `+skuColumns+` FROM catalog.skus WHERE tenant_id=$1 AND store_id=$2 AND id=$3 AND product_id=$4 AND status='active' FOR UPDATE`,
			scope.TenantID, scope.StoreID, e.ID, productID).Scan(skuFields(&cur)...)
		if err != nil {
			return out, false, mapError(err) // foreign/archived/unknown id -> ErrNotFound
		}
		finishSKU(&cur)
		if !active {
			if err := archiveSKURow(ctx, tx, scope, cur.ID, cur.Version); err != nil {
				return out, false, err
			}
			out = cur
			out.Status = StatusArchived
			out.Version++
			return out, false, nil
		}
		code := cur.Code
		if e.Code != "" {
			code = e.Code
		}
		err = tx.QueryRow(ctx, `UPDATE catalog.skus SET code=$3,price_minor=$4,compare_at_minor=$5,weight_grams=$6,length_mm=$7,width_mm=$8,height_mm=$9,origin_country=$10,customs_name=$11,hs_candidate=$12,option_values=$13,inventory_tracked=$14,max_per_order=$15,version=version+1,updated_at=clock_timestamp()
			WHERE tenant_id=$1 AND store_id=$2 AND id=$16 AND version=$17 RETURNING `+skuColumns,
			scope.TenantID, scope.StoreID, code, e.PriceMinor, e.CompareAtMinor, weight, length, width, height, e.OriginCountry, e.CustomsName, e.HSCandidate, values, stock.tracked, stock.maxPerOrder, cur.ID, cur.Version).Scan(skuFields(&out)...)
		if err != nil {
			return out, false, mapError(err)
		}
		finishSKU(&out)
		if cur.PriceMinor != e.PriceMinor {
			if err := appendPriceHistory(ctx, tx, scope, out); err != nil {
				return out, false, err
			}
		}
		return out, false, nil
	}
	code := e.Code
	if code == "" {
		var err error
		if code, err = freeSKUCode(ctx, tx, scope, slug); err != nil {
			return out, false, err
		}
	}
	err := tx.QueryRow(ctx, `INSERT INTO catalog.skus(tenant_id,store_id,product_id,code,status,currency,price_minor,weight_grams,length_mm,width_mm,height_mm,origin_country,customs_name,hs_candidate,option_values,compare_at_minor,inventory_tracked,max_per_order)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18) RETURNING `+skuColumns,
		scope.TenantID, scope.StoreID, productID, code, status, currency, e.PriceMinor, weight, length, width, height, e.OriginCountry, e.CustomsName, e.HSCandidate, values, e.CompareAtMinor, stock.tracked, stock.maxPerOrder).Scan(skuFields(&out)...)
	if err != nil {
		return out, false, mapError(err)
	}
	finishSKU(&out)
	if err := appendPriceHistory(ctx, tx, scope, out); err != nil {
		return out, false, err
	}
	return out, true, nil
}

// freeSKUCode returns a store-unique SKU code derived from the product slug (slug, slug-2, ...), like freeSlug. The SKU
// code CHECK allows 64 chars while the slug allows 80, so the base is clamped first; without it a long product name
// (slug > 64) would generate an 80-char code and fail 23514 -> a bare 422 on create/copy.
func freeSKUCode(ctx context.Context, tx pgx.Tx, scope platform.Scope, base string) (string, error) {
	if len(base) > 64 {
		base = strings.TrimRight(base[:64], "-")
	}
	for n := 1; n <= 50; n++ {
		candidate := base
		if n > 1 {
			suffix := "-" + strconv.Itoa(n)
			if len(base)+len(suffix) > 64 {
				candidate = strings.TrimRight(base[:64-len(suffix)], "-") + suffix
			} else {
				candidate = base + suffix
			}
		}
		var taken bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM catalog.skus WHERE tenant_id=$1 AND store_id=$2 AND code=$3)`,
			scope.TenantID, scope.StoreID, candidate).Scan(&taken); err != nil {
			return "", err
		}
		if !taken {
			return candidate, nil
		}
	}
	return "", command.ErrConflict
}

func resolveWarehouse(ctx context.Context, tx pgx.Tx, scope platform.Scope, explicit string) (string, error) {
	if explicit != "" {
		var active bool
		if err := tx.QueryRow(ctx, `SELECT active FROM inventory.warehouses WHERE tenant_id=$1 AND store_id=$2 AND id=$3`,
			scope.TenantID, scope.StoreID, explicit).Scan(&active); err != nil {
			return "", mapError(err)
		}
		if !active {
			return "", command.ErrConflict
		}
		return explicit, nil
	}
	rows, err := tx.Query(ctx, `SELECT id::text FROM inventory.warehouses WHERE tenant_id=$1 AND store_id=$2 AND active ORDER BY id`,
		scope.TenantID, scope.StoreID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if len(ids) != 1 {
		return "", command.ErrInvalid
	}
	return ids[0], nil
}

func writeOpeningStock(ctx context.Context, tx pgx.Tx, scope platform.Scope, warehouseID, skuID string, qty int64, operation, key string) error {
	if qty < 0 || qty > command.MaxQuantity {
		return command.ErrInvalid
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`,
		"balance|"+scope.TenantID+"|"+scope.StoreID+"|"+warehouseID+"|"+skuID); err != nil {
		return err
	}
	return insertLedgerAdjust(ctx, tx, scope, warehouseID, skuID, qty, operation, key, reasonOpeningStock)
}

func writeTargetStock(ctx context.Context, tx pgx.Tx, scope platform.Scope, warehouseID, skuID string, target int64, operation, key string) error {
	if target < 0 || target > command.MaxQuantity {
		return command.ErrInvalid
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`,
		"balance|"+scope.TenantID+"|"+scope.StoreID+"|"+warehouseID+"|"+skuID); err != nil {
		return err
	}
	var onHand, reserved, allocated, unavailable int64
	err := tx.QueryRow(ctx, `SELECT on_hand,reserved,allocated,unavailable FROM inventory.lock_balance($1::uuid,$2::uuid)`, warehouseID, skuID).
		Scan(&onHand, &reserved, &allocated, &unavailable)
	if errors.Is(err, pgx.ErrNoRows) {
		onHand, reserved, allocated, unavailable = 0, 0, 0, 0
	} else if err != nil {
		return mapError(err)
	}
	delta := target - onHand
	if delta == 0 {
		return nil
	}
	if delta < 0 && -(onHand-reserved-allocated-unavailable) > delta {
		return command.ErrInsufficient
	}
	return insertLedgerAdjust(ctx, tx, scope, warehouseID, skuID, delta, operation, key, reasonTargetStock)
}

// insertLedgerAdjust appends one ADJUST ledger row (the trigger is the sole balance writer). It mirrors
// inventory.insertLedger but omits reservation_id (ADJUST requires NULL).
func insertLedgerAdjust(ctx context.Context, tx pgx.Tx, scope platform.Scope, warehouseID, skuID string, delta int64, operation, key, reason string) error {
	_, err := tx.Exec(ctx, `INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_on_hand,delta_reserved,delta_allocated,delta_unavailable,operation,command_key,reason,principal_id)
		VALUES($1,$2,$3,$4,'ADJUST',$5,0,0,0,$6,$7,$8,$9)`,
		scope.TenantID, scope.StoreID, warehouseID, skuID, delta, operation, key, reason, scope.PrincipalID)
	return err
}

// writeKeyword sets (non-empty) or clears (empty) the SKU's store keyword. The keyword is normalized with the shared pure
// grammar and the taken check returns ErrKeywordTaken (409 keyword_taken) before the DB UNIQUE backstop.
func writeKeyword(ctx context.Context, tx pgx.Tx, scope platform.Scope, skuID, keyword string) error {
	if keyword == "" {
		_, err := tx.Exec(ctx, `DELETE FROM live.keyword_library WHERE tenant_id=$1 AND store_id=$2 AND sku_id=$3`,
			scope.TenantID, scope.StoreID, skuID)
		return err
	}
	kw, ok := grammar.NormalizeKeyword(keyword)
	if !ok {
		return command.ErrInvalid
	}
	var taken string
	err := tx.QueryRow(ctx, `SELECT sku_id::text FROM live.keyword_library WHERE tenant_id=$1 AND store_id=$2 AND keyword=$3 AND sku_id<>$4`,
		scope.TenantID, scope.StoreID, kw, skuID).Scan(&taken)
	if err == nil {
		return ErrKeywordTaken
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO live.keyword_library(tenant_id,store_id,sku_id,keyword,principal_id) VALUES($1,$2,$3,$4,$5)
		ON CONFLICT (tenant_id,store_id,sku_id) DO UPDATE SET keyword=EXCLUDED.keyword,version=live.keyword_library.version+1,principal_id=EXCLUDED.principal_id,updated_at=clock_timestamp()`,
		scope.TenantID, scope.StoreID, skuID, kw, scope.PrincipalID)
	return err
}

// setProductCollections replaces the product's collection membership: remove it everywhere, append it to each requested
// collection (locking each collection row so the append position is race-free), and bump the version of every collection
// whose membership changed.
func setProductCollections(ctx context.Context, tx pgx.Tx, scope platform.Scope, productID string, collectionIDs []string) error {
	ids := append([]string(nil), collectionIDs...)
	sort.Strings(ids)
	for _, id := range ids {
		if _, err := lockCollection(ctx, tx, scope, id); err != nil {
			return err
		}
	}
	old := map[string]bool{}
	rows, err := tx.Query(ctx, `SELECT collection_id::text FROM catalog.collection_products WHERE tenant_id=$1 AND store_id=$2 AND product_id=$3`,
		scope.TenantID, scope.StoreID, productID)
	if err != nil {
		return mapError(err)
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		old[id] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if _, err := tx.Exec(ctx, `DELETE FROM catalog.collection_products WHERE tenant_id=$1 AND store_id=$2 AND product_id=$3`,
		scope.TenantID, scope.StoreID, productID); err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := tx.Exec(ctx, `INSERT INTO catalog.collection_products(tenant_id,store_id,collection_id,product_id,position)
			SELECT $1,$2,$3,$4,coalesce(max(position)+1,0)::integer FROM catalog.collection_products WHERE tenant_id=$1 AND store_id=$2 AND collection_id=$3`,
			scope.TenantID, scope.StoreID, id, productID); err != nil {
			return err
		}
		old[id] = true // mark for version bump
	}
	for id := range old {
		if _, err := tx.Exec(ctx, `UPDATE catalog.collections SET version=version+1,updated_at=clock_timestamp() WHERE tenant_id=$1 AND store_id=$2 AND id=$3`,
			scope.TenantID, scope.StoreID, id); err != nil {
			return err
		}
	}
	return nil
}

// BulkSetProductStatus sets one status on up to 100 products, per-item. A product whose live window is open is refused
// live_window_open per item; other products proceed independently (a per-item refusal does not abort the rest).
func BulkSetProductStatus(ctx context.Context, tx pgx.Tx, scope platform.Scope, key string, in BulkStatusInput) ([]BulkStatusItem, error) {
	status := in.Status
	if status != StatusDraft && status != StatusActive && status != StatusArchived {
		return nil, command.ErrInvalid
	}
	if len(in.IDs) < 1 || len(in.IDs) > 100 {
		return nil, command.ErrInvalid
	}
	seen := map[string]bool{}
	for _, id := range in.IDs {
		if !command.ValidID(id) || seen[id] {
			return nil, command.ErrInvalid
		}
		seen[id] = true
	}
	request := struct {
		IDs    []string `json:"ids"`
		Status string   `json:"status"`
	}{in.IDs, status}
	out := []BulkStatusItem{}
	err := command.Run(ctx, tx, scope, "catalog.product.bulk_status", key, request, &out, func() error {
		for _, id := range in.IDs {
			item := BulkStatusItem{ID: id}
			if err := setOneProductStatus(ctx, tx, scope, id, status); err != nil {
				var coded *Error
				if errors.As(err, &coded) {
					item.Err = coded.Code
				} else if errors.Is(err, command.ErrNotFound) {
					item.Err = "not_found"
				} else {
					return err // whole-command failure, not a per-item refusal
				}
			} else {
				item.Status = status
			}
			out = append(out, item)
		}
		return command.Audit(ctx, tx, scope, "catalog.product.bulk_status")
	})
	return out, mapError(err)
}

// setOneProductStatus applies one status change to one product, refusing live_window_open while an OPEN claim window has
// an active offer on one of its SKUs (unlist = draft or archived; listing to active is always allowed).
func setOneProductStatus(ctx context.Context, tx pgx.Tx, scope platform.Scope, id, status string) error {
	if err := lockProduct(ctx, tx, scope, id); err != nil {
		return err
	}
	if status != StatusActive {
		open, err := liveWindowOpenForProduct(ctx, tx, scope, id)
		if err != nil {
			return err
		}
		if open {
			return ErrLiveWindowOpen
		}
	}
	_, err := tx.Exec(ctx, `UPDATE catalog.products SET status=$3,version=version+1,updated_at=clock_timestamp()
		WHERE tenant_id=$1 AND store_id=$2 AND id=$4`, scope.TenantID, scope.StoreID, status, id)
	return mapError(err)
}

// CopyProduct duplicates a product as a draft: name +「（复制）」, a fresh slug, regenerated SKU codes, the same options and
// prices, and no images or keywords. The source product must exist (ExpectedVersion guards it).
func CopyProduct(ctx context.Context, tx pgx.Tx, scope platform.Scope, key, id string, in CopyInput) (Product, error) {
	if !command.ValidID(id) || in.ExpectedVersion < 1 {
		return Product{}, command.ErrInvalid
	}
	request := struct {
		ID              string `json:"id"`
		ExpectedVersion int64  `json:"expected_version"`
	}{id, in.ExpectedVersion}
	var out Product
	err := command.Run(ctx, tx, scope, "catalog.product.copy", key, request, &out, func() error {
		var src Product
		if err := tx.QueryRow(ctx, `SELECT `+productColumns+` FROM catalog.products WHERE tenant_id=$1 AND store_id=$2 AND id=$3 FOR SHARE`,
			scope.TenantID, scope.StoreID, id).Scan(productFields(&src)...); err != nil {
			return mapError(err)
		}
		finishProduct(&src)
		if src.Version != in.ExpectedVersion {
			return command.ErrConflict
		}
		// Clamp the source name so name + suffix stays within the 120-rune name CHECK (0002 catalog.products.name): a
		// 117–120-rune source would otherwise make the INSERT fail 23514 -> a bare 422. src.Name is always <=120 runes.
		name := src.Name
		if max := 120 - utf8Count(copyNameSuffix); utf8Count(name) > max {
			name = string([]rune(name)[:max])
		}
		name += copyNameSuffix
		slug, err := freeSlug(ctx, tx, scope, "catalog.products", Slugify(name))
		if err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `INSERT INTO catalog.products(tenant_id,store_id,name,description,status,slug,seo_title,seo_description,options)
			VALUES($1,$2,$3,$4,'draft',$5,$6,$7,$8) RETURNING `+productColumns,
			scope.TenantID, scope.StoreID, name, src.Description, slug, src.SEOTitle, src.SEODescription, src.Options).Scan(productFields(&out)...); err != nil {
			return err
		}
		finishProduct(&out)
		skus, err := loadActiveSKUsOrdered(ctx, tx, scope, id)
		if err != nil {
			return err
		}
		for _, s := range skus {
			code, err := freeSKUCode(ctx, tx, scope, out.Slug)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO catalog.skus(tenant_id,store_id,product_id,code,status,currency,price_minor,weight_grams,length_mm,width_mm,height_mm,origin_country,customs_name,hs_candidate,option_values,compare_at_minor,inventory_tracked,max_per_order)
				VALUES($1,$2,$3,$4,'active',$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
				scope.TenantID, scope.StoreID, out.ID, code, s.Currency, s.PriceMinor, s.WeightGrams, s.LengthMM, s.WidthMM, s.HeightMM, s.OriginCountry, s.CustomsName, s.HSCandidate, s.OptionValues, s.CompareAtMinor, s.InventoryTracked, s.MaxPerOrder); err != nil {
				return err
			}
			// No price_history copy: the copy has its own SKU ids and starts its own (empty) history.
		}
		return command.Audit(ctx, tx, scope, "catalog.product.copied")
	})
	finishProduct(&out)
	return out, mapError(err)
}
