package catalog

// document.go owns the product-editor document save command (docs/delivery/units/product-editor.md §f; the A6 amendment of
// contracts/catalog-inventory-v1.md): one idempotent command that writes a product, its option axes, SKUs (with
// inventory_tracked/max_per_order), stock opening/target, keyword and collection membership in a single transaction, plus
// the bulk status and copy commands.
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
)

// ProductDocumentInput is the create body and the full-replace edit body. ExpectedVersion 0 means create (ID empty); a
// positive version means edit (ID is the product being replaced). WarehouseID is optional: empty resolves the store's single
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

// DocumentSKUInput is one SKU of the document. ID is set only on edit (the SKU to update in place); an entry without ID
// is created. OptionValues is the full combination; Code is generated from the product slug when empty. Stock carries the
// tracked/untracked mode and the opening (create) or target (edit) on-hand quantity. Keyword is the store keyword (cleared
// when empty). Active is nil or true -> active; false -> archived.
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

// SaveProductDocument writes a product document (create product.save / edit product.save:<id>) in one command.Run. It is a
// full replace: every SKU in the document is created or updated in place, and an existing active SKU that is not referenced
// is archived (an axis change archives any SKU the new document omits, so order history stays intact and the 0086 partial
// combination index frees the slot). Any failure — a conflicting expected_version, a taken keyword or slug, a bad amount —
// rolls the whole transaction back.
func SaveProductDocument(ctx context.Context, tx pgx.Tx, scope platform.Scope, key string, in ProductDocumentInput) (out ProductDocument, err error) {
	updating := in.ExpectedVersion >= 1
	if !validDocumentInput(in, updating) {
		return out, command.ErrInvalid
	}
	productID := in.ID
	status := in.Status
	if status == "" {
		status = StatusDraft
	}
	operation := documentCreateOperation
	if updating {
		operation = documentEditOperation + strings.ReplaceAll(productID, "-", "")
	}
	// Canonical request: the validated input with defaults the hash must see (DB-derived values — generated codes, the
	// resolved warehouse — stay out, so a replay with the same bytes still replays the first result).
	request := canonicalDocumentRequest(in, status)
	err = command.Run(ctx, tx, scope, operation, key, request, &out, func() error {
		currency, err := storeCurrency(ctx, tx, scope)
		if err != nil {
			return err
		}
		axes := in.Options
		if axes == nil {
			axes = []OptionAxis{}
		}
		var prod Product
		if updating {
			prod, err = applyProductEdit(ctx, tx, scope, productID, in, status, axes)
			if err != nil {
				return err
			}
		} else {
			slugPtr := &in.Slug
			if in.Slug == "" {
				slugPtr, err = freeSlug(ctx, tx, scope, "catalog.products", Slugify(in.Name))
				if err != nil {
					return err
				}
			}
			err = tx.QueryRow(ctx, `INSERT INTO catalog.products(tenant_id,store_id,name,description,status,slug,seo_title,seo_description,options)
				VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING `+productColumns,
				scope.TenantID, scope.StoreID, in.Name, in.Description, status, slugPtr, in.SEOTitle, in.SEODescription, axes).Scan(productFields(&prod)...)
			if err != nil {
				return err
			}
			finishProduct(&prod)
		}
		out.Product = prod
		productID = prod.ID
		existing, err := loadActiveSKUs(ctx, tx, scope, productID)
		if err != nil {
			return err
		}
		referenced := map[string]bool{}
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
			if entry.ID != "" {
				referenced[entry.ID] = true
			}
			sku, created, err := writeSKU(ctx, tx, scope, productID, prod.Slug, currency, entry, values, stock, in.WeightGrams, in.LengthMM, in.WidthMM, in.HeightMM)
			if err != nil {
				return err
			}
			if stock.tracked && stock.qty != nil && *stock.qty != 0 {
				if warehouse == "" {
					if warehouse, err = resolveWarehouse(ctx, tx, scope, ""); err != nil {
						return err
					}
				}
				if created {
					if err := writeOpeningStock(ctx, tx, scope, warehouse, sku.ID, *stock.qty, operation, key); err != nil {
						return err
					}
				} else if err := writeTargetStock(ctx, tx, scope, warehouse, sku.ID, *stock.qty, operation, key); err != nil {
					return err
				}
			}
			if err := writeKeyword(ctx, tx, scope, sku.ID, entry.Keyword); err != nil {
				return err
			}
		}
		// Full replace: every existing active SKU not referenced by the document is archived (this is also how an axis
		// change retires the SKUs the new document omits). The product lock serializes against concurrent SKU mutation.
		for id, sku := range existing {
			if referenced[id] {
				continue
			}
			if err := archiveSKURow(ctx, tx, scope, id, sku.Version); err != nil {
				return err
			}
		}
		if err := setProductCollections(ctx, tx, scope, productID, in.CollectionIDs); err != nil {
			return err
		}
		out.SKUs, err = loadActiveSKUsOrdered(ctx, tx, scope, productID)
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

// validDocumentInput is the create/edit grammar. Callers already canonicalize status.
func validDocumentInput(in ProductDocumentInput, updating bool) bool {
	if !validName(in.Name) || !validDescription(in.Description) ||
		(in.Status != "" && in.Status != StatusDraft && in.Status != StatusActive) ||
		(in.Slug != "" && !validSlug(in.Slug)) || !validSEO(in.SEOTitle, in.SEODescription) ||
		!validOptions(in.Options) || (!updating && in.ExpectedVersion != 0) || (updating && in.ExpectedVersion < 1) ||
		(updating && !command.ValidID(in.ID)) || (!updating && in.ID != "") ||
		len(in.SKUs) < 1 || len(in.SKUs) > maxActiveSKUsPerProduct ||
		len(in.CollectionIDs) > maxCollectionProducts || (in.WarehouseID != "" && !command.ValidID(in.WarehouseID)) ||
		in.WeightGrams < 0 || in.WeightGrams > command.MaxQuantity ||
		in.LengthMM < 0 || in.LengthMM > 1_000_000 || in.WidthMM < 0 || in.WidthMM > 1_000_000 ||
		in.HeightMM < 0 || in.HeightMM > 1_000_000 {
		return false
	}
	seenIDs := map[string]bool{}
	seenValues := map[string]bool{}
	for _, e := range in.SKUs {
		if (e.ID != "" && (!command.ValidID(e.ID) || seenIDs[e.ID])) ||
			(e.Code != "" && !skuCodePattern.MatchString(e.Code)) ||
			e.PriceMinor < 0 || e.PriceMinor > command.MaxMoney ||
			(e.CompareAtMinor != nil && (*e.CompareAtMinor <= e.PriceMinor || *e.CompareAtMinor > command.MaxMoney)) ||
			(e.OriginCountry != "" && !countryPattern.MatchString(e.OriginCountry)) ||
			utf8Count(e.CustomsName) > 240 || (e.HSCandidate != "" && !hsPattern.MatchString(e.HSCandidate)) ||
			len(e.OptionValues) > 3 {
			return false
		}
		if e.ID != "" {
			seenIDs[e.ID] = true
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
			if !updating && e.Stock.TargetQty != nil {
				return false // create writes opening_qty only
			}
			if updating && e.Stock.OpeningQty != nil {
				return false // edit writes target_qty only
			}
			if e.Stock.OpeningQty != nil && *e.Stock.OpeningQty < 0 {
				return false
			}
			if e.Stock.TargetQty != nil && *e.Stock.TargetQty < 0 {
				return false
			}
		}
	}
	return true
}

// applyProductEdit locks the product, verifies the optimistic version, and writes the scalar fields and axes. An omitted
// slug keeps the current one (full-replace but the slug is part of the product's public identity).
func applyProductEdit(ctx context.Context, tx pgx.Tx, scope platform.Scope, id string, in ProductDocumentInput, status string, axes []OptionAxis) (Product, error) {
	var out Product
	if err := lockProduct(ctx, tx, scope, id); err != nil {
		return out, err
	}
	var cur Product
	if err := tx.QueryRow(ctx, `SELECT `+productColumns+` FROM catalog.products WHERE tenant_id=$1 AND store_id=$2 AND id=$3`,
		scope.TenantID, scope.StoreID, id).Scan(productFields(&cur)...); err != nil {
		return out, mapError(err)
	}
	if cur.Version != in.ExpectedVersion {
		return out, command.ErrConflict
	}
	if cur.Status == StatusArchived {
		return out, command.ErrConflict
	}
	slug := in.Slug
	if slug == "" {
		slug = cur.Slug
	}
	err := tx.QueryRow(ctx, `UPDATE catalog.products SET name=$3,description=$4,status=$5,slug=$6,seo_title=$7,seo_description=$8,options=$9,version=version+1,updated_at=clock_timestamp()
		WHERE tenant_id=$1 AND store_id=$2 AND id=$10 AND version=$11 RETURNING `+productColumns,
		scope.TenantID, scope.StoreID, in.Name, in.Description, status, slug, in.SEOTitle, in.SEODescription, axes, id, in.ExpectedVersion).Scan(productFields(&out)...)
	if err != nil {
		return out, mapError(err)
	}
	finishProduct(&out)
	return out, nil
}

// loadActiveSKUs returns the product's active SKUs keyed by id (for the full-replace archive decision).
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

// freeSKUCode returns a store-unique SKU code derived from the product slug (slug, slug-2, ...), like freeSlug.
func freeSKUCode(ctx context.Context, tx pgx.Tx, scope platform.Scope, base string) (string, error) {
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
		var open bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM live.claim_windows w
			JOIN live.offers o ON o.tenant_id=w.tenant_id AND o.store_id=w.store_id AND o.session_id=w.session_id
			JOIN catalog.skus s ON s.tenant_id=o.tenant_id AND s.store_id=o.store_id AND s.id=o.sku_id
			WHERE w.tenant_id=$1 AND w.store_id=$2 AND w.state='OPEN' AND o.active AND s.product_id=$3)`,
			scope.TenantID, scope.StoreID, id).Scan(&open); err != nil {
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
		name := src.Name + "（复制）"
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
