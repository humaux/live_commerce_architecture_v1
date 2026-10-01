// csvimport.go applies an uploaded product CSV (contract G2) to the caller's store inside the caller's ONE platform.WithScopeBudget
// transaction. The same apply code serves the preview (always rolled back) and the commit (kept only when there is no row error), so what
// the preview promised is what the commit does. Each product and each variant runs in its own savepoint: a failure is recorded as a
// RowError and the loop continues, which is how the preview lists every error with its row number.
//
// It writes ONLY through internal/catalog (CreateProduct, PatchProduct, CreateSKU, UpdateSKU, SetSKUPrice, SetCollectionProducts) and
// internal/inventory (AdjustOnHand with the reason "csv import <sha256 prefix>"): journaled commands with keys derived from the file hash
// and the row, optimistic versions read in the same transaction, the catalog's own validation. It never deletes or archives a row (a row
// missing from the file leaves its product and SKU alone), never writes the stock ledger itself, never imports images and never creates a
// collection or a warehouse.
// Tables read directly under RLS (commerce_runtime): catalog.products, catalog.skus, catalog.collections, catalog.collection_products,
// inventory.balances, inventory.warehouses (through inventory.ListWarehouses).
//
// Idempotency (MT08): the commit is one command (operation merchanttools.csv.import, key csv-<sha256 prefix>); the same bytes uploaded
// again replay the first summary and change nothing.

package merchanttools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sort"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/catalog"
	"livecommerce/internal/command"
	"livecommerce/internal/inventory"
	"livecommerce/internal/platform"
)

// ImportResult is the preview and commit answer (contract G2).
type ImportResult struct {
	FileSHA256       string     `json:"file_sha256"`
	Rows             int        `json:"rows"`
	CreatedProducts  int        `json:"created_products"`
	UpdatedProducts  int        `json:"updated_products"`
	CreatedSKUs      int        `json:"created_skus"`
	UpdatedSKUs      int        `json:"updated_skus"`
	StockAdjustments int        `json:"stock_adjustments"`
	UnchangedRows    int        `json:"unchanged_rows"`
	Errors           []RowError `json:"errors"`
	ErrorsTruncated  bool       `json:"errors_truncated"`
	Committed        bool       `json:"committed"`
	Replayed         bool       `json:"replayed"`
}

// ImportProducts parses and applies data. commit=false always returns ErrPreviewRolledBack with the computed result (the caller serves it
// and the transaction rolls back); commit=true returns the result with a nil error only when every row applied, ErrImportHasErrors (with the
// result) otherwise, and a replay of an earlier commit of the same bytes with Replayed set.
func ImportProducts(ctx context.Context, tx pgx.Tx, scope platform.Scope, data []byte, commit bool) (ImportResult, error) {
	empty := ImportResult{Errors: []RowError{}}
	if tx == nil || !command.ValidID(scope.TenantID) || !command.ValidID(scope.StoreID) || !command.ValidID(scope.PrincipalID) {
		return empty, command.ErrInvalid
	}
	if len(data) == 0 || len(data) > MaxCSVBytes {
		return empty, &Error{Status: http.StatusRequestEntityTooLarge, Code: "invalid_request"}
	}
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	empty.FileSHA256 = digest
	currency, err := storeCurrency(ctx, tx, scope)
	if err != nil {
		return empty, abort(err)
	}
	if !commit {
		res, err := applyFile(ctx, tx, scope, data, CurrencyExponent(currency), digest)
		if err != nil {
			return res, err
		}
		return res, ErrPreviewRolledBack
	}
	var res, attempt ImportResult
	executed := false
	var applyErr error
	err = command.Run(ctx, tx, scope, "merchanttools.csv.import", "csv-"+digest[:32], struct {
		SHA string `json:"sha256"`
	}{digest}, &res, func() error {
		executed = true
		attempt, applyErr = applyFile(ctx, tx, scope, data, CurrencyExponent(currency), digest)
		if applyErr != nil {
			return applyErr
		}
		if len(attempt.Errors) > 0 {
			return ErrImportHasErrors
		}
		attempt.Committed = true
		res = attempt
		return command.Audit(ctx, tx, scope, "merchanttools.csv_imported")
	})
	switch {
	case err == nil:
		res.Replayed = !executed
		return res, nil
	case errors.Is(err, ErrImportHasErrors):
		return attempt, ErrImportHasErrors
	case errors.Is(err, command.ErrConflict) && !executed:
		return empty, &Error{Status: http.StatusConflict, Code: "idempotency_conflict"}
	}
	return empty, err
}

// maxWorkUnits caps the lock-taking commands of ONE import: product create/patch 1, variant create/option update/price 1 each, a collection
// membership 1, a stock adjustment 2 (its command lock and its balance lock). Every unit is a transaction-scoped advisory lock held until the end of
// the single all-or-nothing transaction, and PostgreSQL's shared lock table is max_locks_per_transaction x (max_connections + prepared) = 9,600
// slots by default (deploy/postgres/postgresql.conf keeps the default), shared with every other session; 5,000 rows measured out of shared memory.
// 6,000 leaves headroom. Rows that change nothing cost nothing, so re-importing a mostly unchanged export is not penalised.
const maxWorkUnits = 6000

type importRun struct {
	ctx        context.Context
	tx         pgx.Tx
	scope      platform.Scope
	sha12      string
	res        *ImportResult
	warehouses map[string]string // header name -> warehouse id (only unambiguous names)
	hasCompare bool              // the file has a compare_at_price column (see File.HasCompare)
	units      int               // lock-taking commands issued so far (maxWorkUnits)
	stop       bool              // the work cap was hit: no further row is attempted
	fatal      error
}

// charge accounts n lock-taking commands before they are issued; false means the work cap is exceeded (the caller aborts its row with the
// limit code and the whole import stops, so a refused file is reported at the row where it became too large).
func (r *importRun) charge(n int) bool {
	if r.units+n > maxWorkUnits {
		r.stop = true
		return false
	}
	r.units += n
	return true
}

func (r *importRun) fail(row int, column, code string) {
	r.res.Errors = append(r.res.Errors, RowError{Row: row, Column: column, Code: code})
}

// savepoint runs fn in a nested transaction. A refusal the domain classifies becomes a RowError (the savepoint rolls back); anything else
// (deadline, connection, unknown SQL error) aborts the whole import as unavailable, never as a misleading row error.
func (r *importRun) savepoint(row int, fn func(pgx.Tx) (column, code string, err error)) bool {
	if r.fatal != nil {
		return false
	}
	sp, err := r.tx.Begin(r.ctx)
	if err != nil {
		r.fatal = abort(err)
		return false
	}
	column, code, err := fn(sp)
	if err == nil {
		if err = sp.Commit(r.ctx); err != nil {
			r.fatal = abort(err)
			return false
		}
		return true
	}
	_ = sp.Rollback(r.ctx)
	switch {
	case code != "":
		r.fail(row, column, code)
	case errors.Is(err, command.ErrInvalid):
		r.fail(row, column, codeInvalidValue)
	case errors.Is(err, command.ErrConflict), errors.Is(err, command.ErrInsufficient), errors.Is(err, command.ErrNotFound):
		r.fail(row, column, codeConflict)
	default:
		r.fatal = abort(err)
	}
	return false
}

// abort is ErrUnavailable that remembers why (errors.Is / As still see the *Error; classification answers only its code, so no driver text
// reaches a client, but a test or a log can print the cause: out of shared memory, a deadline, a lost connection).
func abort(cause error) error { return fmt.Errorf("%w: %v", ErrUnavailable, cause) }

func applyFile(ctx context.Context, tx pgx.Tx, scope platform.Scope, data []byte, exp int, digest string) (ImportResult, error) {
	res := ImportResult{FileSHA256: digest, Errors: []RowError{}}
	file := ParseFile(data, exp)
	res.Rows = len(file.Rows)
	res.Errors = append(res.Errors, file.Errors...)
	groups, groupErrs := GroupRows(file.Rows)
	res.Errors = append(res.Errors, groupErrs...)
	run := &importRun{ctx: ctx, tx: tx, scope: scope, sha12: digest[:12], res: &res, warehouses: map[string]string{}, hasCompare: file.HasCompare}

	// Warehouses named by stock: columns must exist and be unambiguous.
	if len(file.Warehouses) > 0 {
		list, err := inventory.ListWarehouses(ctx, tx, scope)
		if err != nil {
			return res, abort(err)
		}
		byName := map[string][]string{}
		for _, w := range list {
			byName[w.Name] = append(byName[w.Name], w.ID)
		}
		for _, name := range file.Warehouses {
			switch ids := byName[name]; len(ids) {
			case 1:
				run.warehouses[name] = ids[0]
			case 0:
				run.fail(0, stockPrefix+name, codeUnknownWh)
			default:
				run.fail(0, stockPrefix+name, codeInvalidValue)
			}
		}
	}
	// A product with any parse or cross-row error is skipped by the DB pass (its findings are already listed).
	bad := map[int]bool{}
	for _, e := range res.Errors {
		if e.Row > 0 {
			bad[e.Row] = true
		}
	}
	fileLevel := false
	for _, e := range res.Errors {
		fileLevel = fileLevel || e.Row == 0
	}
	if !fileLevel {
		for _, g := range groups {
			skip := bad[g.First]
			for _, row := range g.Rows {
				skip = skip || bad[row.N]
			}
			if !skip {
				run.group(g, file.Warehouses)
			}
			if run.fatal != nil {
				return res, run.fatal
			}
			if run.stop {
				break
			}
		}
	}
	sort.SliceStable(res.Errors, func(i, j int) bool {
		if res.Errors[i].Row != res.Errors[j].Row {
			return res.Errors[i].Row < res.Errors[j].Row
		}
		return res.Errors[i].Column < res.Errors[j].Column
	})
	if len(res.Errors) > maxRowErrors {
		res.Errors, res.ErrorsTruncated = res.Errors[:maxRowErrors], true
	}
	return res, nil
}

type productRow struct {
	ID          string
	Name        string
	Description string
	Status      string
	Version     int64
	Options     []catalog.OptionAxis
}

// axes derives the axes the file asks for: one per named option column, values in first-seen order across the variant rows.
func axesOf(g Group) []catalog.OptionAxis {
	var axes []catalog.OptionAxis
	for k := 0; k < 3 && g.OptName[k] != ""; k++ {
		axis := catalog.OptionAxis{Name: g.OptName[k], Values: []string{}}
		for _, r := range g.Rows {
			if !slices.Contains(axis.Values, r.OptValue[k]) {
				axis.Values = append(axis.Values, r.OptValue[k])
			}
		}
		axes = append(axes, axis)
	}
	return axes
}

// mergeAxes keeps an existing product's axes when the file names the same axes (new values are appended); a different axis set replaces
// them (catalog.PatchProduct refuses that while an active variant would not fit). No axis in the file leaves the product's axes alone.
func mergeAxes(existing, want []catalog.OptionAxis) []catalog.OptionAxis {
	if len(want) == 0 {
		return existing
	}
	if len(existing) != len(want) {
		return want
	}
	merged := make([]catalog.OptionAxis, len(want))
	for i := range want {
		if existing[i].Name != want[i].Name {
			return want
		}
		merged[i] = catalog.OptionAxis{Name: want[i].Name, Values: slices.Clone(existing[i].Values)}
		for _, v := range want[i].Values {
			if !slices.Contains(merged[i].Values, v) {
				merged[i].Values = append(merged[i].Values, v)
			}
		}
	}
	return merged
}

func sameAxes(a, b []catalog.OptionAxis) bool {
	return len(a) == len(b) && slices.EqualFunc(a, b, func(x, y catalog.OptionAxis) bool {
		return x.Name == y.Name && slices.Equal(x.Values, y.Values)
	})
}

func (r *importRun) key(row int, suffix string) string {
	return fmt.Sprintf("csv-%s-r%d-%s", r.sha12, row, suffix)
}

// group applies one product: the product row, then each variant row, then its collections.
func (r *importRun) group(g Group, header []string) {
	var product productRow
	created, changed := false, false
	want := axesOf(g)
	if len(want) == 0 && g.OptName[0] != "" {
		r.fail(g.First, "option1_name", codeOptionMismatch) // an axis with no variant row has no values
		return
	}
	ok := r.savepoint(g.First, func(tx pgx.Tx) (string, string, error) {
		err := tx.QueryRow(r.ctx, `SELECT id::text,name,description,status,version,options FROM catalog.products
			WHERE tenant_id=$1 AND store_id=$2 AND slug=$3`, r.scope.TenantID, r.scope.StoreID, g.Handle).
			Scan(&product.ID, &product.Name, &product.Description, &product.Status, &product.Version, &product.Options)
		if errors.Is(err, pgx.ErrNoRows) {
			status := g.Status
			if status == "" {
				status = catalog.StatusDraft
			}
			if status == catalog.StatusArchived {
				return "status", codeInvalidValue, command.ErrInvalid
			}
			if !r.charge(1) {
				return "", codeLimit, command.ErrInvalid
			}
			p, err := catalog.CreateProduct(r.ctx, tx, r.scope, r.key(g.First, "p"), catalog.ProductInput{Name: g.Title, Description: g.Description,
				Status: status, Slug: g.Handle, Options: want})
			if err != nil {
				if errors.Is(err, command.ErrConflict) {
					return "handle", codeSlugTaken, err
				}
				return "", "", err
			}
			product = productRow{ID: p.ID, Version: p.Version, Options: p.Options}
			created = true
			return "", "", nil
		}
		if err != nil {
			return "", "", err
		}
		patch := catalog.ProductPatch{ExpectedVersion: product.Version}
		if g.Title != "" && g.Title != product.Name {
			patch.Name = &g.Title
		}
		if g.Description != "" && g.Description != product.Description {
			patch.Description = &g.Description
		}
		if g.Status != "" && g.Status != product.Status {
			patch.Status = &g.Status
		}
		if merged := mergeAxes(product.Options, want); !sameAxes(merged, product.Options) {
			patch.Options = &merged
		}
		if patch.Name == nil && patch.Description == nil && patch.Status == nil && patch.Options == nil {
			return "", "", nil
		}
		if !r.charge(1) {
			return "", codeLimit, command.ErrInvalid
		}
		p, err := catalog.PatchProduct(r.ctx, tx, r.scope, r.key(g.First, "p"), product.ID, patch)
		if err != nil {
			if patch.Options != nil && errors.Is(err, command.ErrConflict) {
				return "option1_name", codeOptionMismatch, err
			}
			return "", "", err
		}
		product.Version, product.Options = p.Version, p.Options
		changed = true
		return "", "", nil
	})
	if !ok {
		return
	}
	if created {
		r.res.CreatedProducts++
	}
	touched := created || changed
	for _, row := range g.Rows {
		if r.fatal != nil || r.stop {
			return
		}
		r.variant(g, row, product.ID, len(want) > 0, header, &touched)
	}
	if len(g.Collections) > 0 {
		r.savepoint(g.First, func(tx pgx.Tx) (string, string, error) {
			for _, slug := range g.Collections {
				added, column, code, err := r.addToCollection(tx, slug, product.ID, g.First)
				if err != nil {
					return column, code, err
				}
				changed = changed || added
			}
			return "", "", nil
		})
	}
	if changed && !created {
		r.res.UpdatedProducts++ // a product-level change: patched fields, axes or a new collection membership
	}
	if len(g.Rows) == 0 && !created && !changed && !touched {
		r.res.UnchangedRows++
	}
}

// addToCollection appends the product to the end of the collection's manual order unless it is already a member (never removes).
func (r *importRun) addToCollection(tx pgx.Tx, slug, productID string, row int) (bool, string, string, error) {
	var id string
	var version int64
	// catalog.collections: commerce_runtime under RLS; membership is read here to append, then written by SetCollectionProducts.
	err := tx.QueryRow(r.ctx, `SELECT id::text,version FROM catalog.collections WHERE tenant_id=$1 AND store_id=$2 AND slug=$3`,
		r.scope.TenantID, r.scope.StoreID, slug).Scan(&id, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, "collections", codeUnknownColl, command.ErrNotFound
	}
	if err != nil {
		return false, "", "", err
	}
	rows, err := tx.Query(r.ctx, `SELECT product_id::text FROM catalog.collection_products WHERE tenant_id=$1 AND store_id=$2 AND collection_id=$3 ORDER BY position`,
		r.scope.TenantID, r.scope.StoreID, id)
	if err != nil {
		return false, "", "", err
	}
	ids := []string{}
	for rows.Next() {
		var p string
		if err = rows.Scan(&p); err != nil {
			rows.Close()
			return false, "", "", err
		}
		ids = append(ids, p)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return false, "", "", err
	}
	if slices.Contains(ids, productID) {
		return false, "", "", nil
	}
	if !r.charge(1) {
		return false, "", codeLimit, command.ErrInvalid
	}
	_, err = catalog.SetCollectionProducts(r.ctx, tx, r.scope, r.key(row, "c-"+slug), id, catalog.CollectionProductsInput{ProductIDs: append(ids, productID), ExpectedVersion: version})
	return err == nil, "collections", "", err
}

// variant applies one SKU row and its stock cells.
func (r *importRun) variant(g Group, row Row, productID string, withAxes bool, header []string, touched *bool) {
	created, updated, adjusted := false, false, 0
	values := []string{}
	for k := 0; withAxes && k < 3 && g.OptName[k] != ""; k++ {
		values = append(values, row.OptValue[k])
	}
	ok := r.savepoint(row.N, func(tx pgx.Tx) (string, string, error) {
		var skuID, owner, status string
		// catalog.skus: code is unique per store across statuses, so one lookup tells "new", "mine", "someone else's" and "archived".
		err := tx.QueryRow(r.ctx, `SELECT id::text,product_id::text,status FROM catalog.skus WHERE tenant_id=$1 AND store_id=$2 AND code=$3`,
			r.scope.TenantID, r.scope.StoreID, row.SKU).Scan(&skuID, &owner, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			if row.Price == nil {
				return "price", codeRequired, command.ErrInvalid
			}
			if !r.charge(1) {
				return "", codeLimit, command.ErrInvalid
			}
			sku, err := catalog.CreateSKU(r.ctx, tx, r.scope, r.key(row.N, "s"), catalog.SKUInput{ProductID: productID, Code: row.SKU,
				PriceMinor: *row.Price, OptionValues: values, CompareAtMinor: row.Compare})
			if err != nil {
				if errors.Is(err, command.ErrConflict) {
					return "sku", codeConflict, err
				}
				return "", "", err
			}
			skuID, created = sku.ID, true
		} else if err != nil {
			return "", "", err
		} else {
			if owner != productID {
				return "sku", codeSKUOtherProduct, command.ErrConflict
			}
			if status != "active" {
				return "sku", codeSKUArchived, command.ErrConflict
			}
			current, err := currentSKU(r.ctx, tx, r.scope, productID, skuID)
			if err != nil {
				return "", "", err
			}
			version := current.Version
			if withAxes && !slices.Equal(current.OptionValues, values) {
				// UpdateSKU is a full replace: carry every stored field, change only the option values (price must equal the stored one).
				if !r.charge(1) {
					return "", codeLimit, command.ErrInvalid
				}
				sku, err := catalog.UpdateSKU(r.ctx, tx, r.scope, r.key(row.N, "o"), skuID, catalog.SKUInput{ProductID: productID, Code: current.Code,
					PriceMinor: current.PriceMinor, WeightGrams: current.WeightGrams, LengthMM: current.LengthMM, WidthMM: current.WidthMM,
					HeightMM: current.HeightMM, OriginCountry: current.OriginCountry, CustomsName: current.CustomsName, HSCandidate: current.HSCandidate,
					OptionValues: values, CompareAtMinor: current.CompareAtMinor, ExpectedVersion: version})
				if err != nil {
					if errors.Is(err, command.ErrConflict) || errors.Is(err, command.ErrInvalid) {
						return "option1_value", codeOptionMismatch, err
					}
					return "", "", err
				}
				version, updated = sku.Version, true
			}
			// A filled price cell makes the price authoritative; with a compare_at_price column its cell is authoritative too (empty clears it),
			// without the column the stored compare-at stays. An empty price cell leaves both.
			compare := current.CompareAtMinor
			if r.hasCompare {
				compare = row.Compare
			}
			if row.Price != nil && (*row.Price != current.PriceMinor || !sameInt(compare, current.CompareAtMinor)) {
				if !r.charge(1) {
					return "", codeLimit, command.ErrInvalid
				}
				if _, err := catalog.SetSKUPrice(r.ctx, tx, r.scope, r.key(row.N, "r"), skuID,
					catalog.PriceInput{PriceMinor: *row.Price, ExpectedVersion: version, CompareAt: catalog.Opt[int64]{Set: true, Val: compare}}); err != nil {
					return "price", "", err
				}
				updated = true
			}
		}
		for i, name := range header {
			target, set := row.Stock[name]
			warehouse, known := r.warehouses[name]
			if !set || !known {
				continue
			}
			var onHand, balanceVersion int64
			// inventory.balances: commerce_runtime under RLS; the write is inventory.AdjustOnHand (single stock writer), this read only
			// turns the target on-hand into a delta and supplies the optimistic version.
			err := tx.QueryRow(r.ctx, `SELECT on_hand,version FROM inventory.balances WHERE tenant_id=$1 AND store_id=$2 AND warehouse_id=$3 AND sku_id=$4`,
				r.scope.TenantID, r.scope.StoreID, warehouse, skuID).Scan(&onHand, &balanceVersion)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return "", "", err
			}
			if target == onHand {
				continue
			}
			if !r.charge(2) {
				return "", codeLimit, command.ErrInvalid
			}
			if _, err = inventory.AdjustOnHand(r.ctx, tx, r.scope, r.key(row.N, fmt.Sprintf("w%d", i)), inventory.Adjustment{WarehouseID: warehouse, SKUID: skuID,
				Delta: target - onHand, ExpectedVersion: balanceVersion, Reason: "csv import " + r.sha12}); err != nil {
				return stockPrefix + name, "", err
			}
			adjusted++
		}
		return "", "", nil
	})
	if !ok {
		return
	}
	r.res.StockAdjustments += adjusted
	switch {
	case created:
		r.res.CreatedSKUs++
	case updated:
		r.res.UpdatedSKUs++
	case adjusted == 0:
		r.res.UnchangedRows++
	}
	*touched = *touched || created || updated || adjusted > 0
}

func sameInt(a, b *int64) bool { return (a == nil) == (b == nil) && (a == nil || *a == *b) }

// currentSKU reads one variant with every field catalog.UpdateSKU must carry (catalog.ListSKUs is the existing read; a product has at
// most 100 active variants).
func currentSKU(ctx context.Context, tx pgx.Tx, scope platform.Scope, productID, skuID string) (catalog.SKU, error) {
	list, err := catalog.ListSKUs(ctx, tx, scope, productID)
	if err != nil {
		return catalog.SKU{}, err
	}
	for _, s := range list {
		if s.ID == skuID {
			return s, nil
		}
	}
	return catalog.SKU{}, command.ErrNotFound
}
