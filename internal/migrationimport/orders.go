// Purpose: the orchestration half of the historical-order CSV import (W5-03B, migration-import-v1 section 7): the preview (always
//   rolled back), the idempotent commit and the chunked apply through the migrationimport.import_orders definer. The archive is
//   read-only: nothing here touches checkout.orders, payments, inventory or any report.
// Depends on: orders_csv.go (parse, aggregate), customers.go (ParseMapping, validateImport, PreviewStaleError, result types), batch.go
//   (recordBatch, errors, tokenHash), internal/command (Run), internal/platform (Scope), pgx/v5; SQL migrationimport.import_orders /
//   record_batch (migration 0156, customers:privacy).
// Used by: internal/httpapi/imports.go (the orders preview / commit routes); pinned by orders_test.go (DB-free) and
//   tests/foundation/order_history_import_test.go (REAL_PG).
// Invariants: orders attach only to already-imported customers (a missing one fails the row customer_not_imported, never creates
//   one); the stored batch results and results.csv carry row numbers, outcomes and codes only (no order id, no customer id); a failed
//   row echoes no id into a preview; the same bytes commit once (file hash = idempotency key).
// Status: REAL_PG + MOCK (synthetic data).
//
// Row accounting: one unit = one order (all its CSV lines) or one unusable line; rows_total, created, updated and failed count units.
// The MaxRows cap applies to CSV data lines (the 0152 batch CHECKs cap rows_total at 5000, and a unit is never more than one line).

package migrationimport

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// kindOrders is the migrationimport.batches.kind of this import.
const kindOrders = "orders"

// OrderPreviewRow is one unit's verdict in the preview response. ExternalID is the order number of a created / updated unit only.
type OrderPreviewRow struct {
	Row        int    `json:"row"`
	ExternalID string `json:"external_id"`
	Outcome    string `json:"outcome"` // created | updated | failed
	Code       string `json:"code,omitempty"`
	// Warning is city_dropped on a created / updated unit whose city cell was not one of the 22 Taiwan cities (the order imports, the city is not stored).
	Warning string `json:"warning,omitempty"`
}

// OrderPreview is the preview answer (also attached to a 409 preview_stale). ApplyRows = NewRows + UpdateRows is the count the commit
// must echo as expected_apply_rows; ErasedRows counts failed units whose customer id carries an erasure tombstone.
type OrderPreview struct {
	FileSHA256 string            `json:"file_sha256"`
	Headers    []string          `json:"headers"`
	Mapping    map[string]string `json:"mapping"`
	RowsTotal  int               `json:"rows_total"`
	NewRows    int               `json:"new_rows"`
	UpdateRows int               `json:"update_rows"`
	ApplyRows  int               `json:"apply_rows"`
	FailedRows int               `json:"failed_rows"`
	ErasedRows int               `json:"erased_rows"`
	// CityDroppedRows counts applicable units whose city cell was refused and stored as empty (a warning, never a failure).
	CityDroppedRows int               `json:"city_dropped_rows"`
	Rows            []OrderPreviewRow `json:"rows"`
}

// OrderCommitResult is the commit answer (replayed:true on an idempotent re-submit of the same file and count).
type OrderCommitResult = CustomerCommitResult

// orderWireRow is one element of the p_rows array of migrationimport.import_orders.
type orderWireRow struct {
	Row        int    `json:"row"`
	OrderID    string `json:"order_id"`
	CustomerID string `json:"customer_id"`
	OrderedAt  string `json:"ordered_at"` // RFC 3339, UTC
	Status     string `json:"status"`
	TotalMinor int64  `json:"total_minor"`
	Items      string `json:"items"`
	City       string `json:"city"`
}

// OrdersPreview parses and dry-runs the file; it always returns ErrPreviewRolledBack with the result (the caller serves it and rolls the
// transaction back). A file-level defect is a coded *Error. Authority customers:privacy.
func OrdersPreview(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, data []byte, mapping map[string]string) (OrderPreview, error) {
	if err := validateImport(tx, scope, token, data); err != nil {
		return OrderPreview{}, err
	}
	digest, _ := digestOf(data)
	parsed, err := runOrders(ctx, tx, scope, token, data, mapping)
	if err != nil {
		return OrderPreview{}, err
	}
	return buildOrderPreview(digest, parsed), ErrPreviewRolledBack
}

// OrdersCommit applies the file once. Idempotent per file hash + mapping + expected_apply_rows (same contract as CustomersCommit): a
// different applicable count commits nothing (409 preview_stale with a fresh preview), zero applicable units is 422 nothing_to_apply.
// Writes customers.historical_orders + one batch + one audit row customers.orders_imported.
func OrdersCommit(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, data []byte, mapping map[string]string, expectedApplyRows int64) (OrderCommitResult, error) {
	empty := OrderCommitResult{}
	if err := validateImport(tx, scope, token, data); err != nil {
		return empty, err
	}
	if expectedApplyRows < 0 || expectedApplyRows > MaxRows {
		return empty, command.ErrInvalid
	}
	digest, sum := digestOf(data)
	var res, attempt OrderCommitResult
	executed := false
	var applyErr error
	err := command.Run(ctx, tx, scope, "migrationimport.orders_commit", "oimp-"+digest[:32], struct {
		SHA               string            `json:"sha256"`
		Mapping           map[string]string `json:"mapping"`
		ExpectedApplyRows int64             `json:"expected_apply_rows"`
	}{digest, mapping, expectedApplyRows}, &res, func() error {
		executed = true
		attempt, applyErr = applyOrders(ctx, tx, scope, token, data, mapping, digest, sum, expectedApplyRows)
		if applyErr != nil {
			return applyErr
		}
		res = attempt
		return nil
	})
	switch {
	case err == nil:
		res.Replayed = !executed
		return res, nil
	case errors.Is(err, command.ErrConflict) && !executed:
		return empty, errIdempotency
	}
	if applyErr != nil {
		return empty, applyErr
	}
	return empty, err
}

// applyOrders runs the shared loop for the commit and enforces the preview/commit contract (see applyCustomers).
func applyOrders(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, data []byte, mapping map[string]string,
	digest string, sum [32]byte, expectedApplyRows int64) (OrderCommitResult, error) {
	parsed, err := runOrders(ctx, tx, scope, token, data, mapping)
	if err != nil {
		return OrderCommitResult{}, err
	}
	created, updated, failed := tallyOrders(parsed.units)
	if int64(created+updated) != expectedApplyRows {
		return OrderCommitResult{}, &PreviewStaleError{Preview: buildOrderPreview(digest, parsed)}
	}
	if created+updated == 0 {
		return OrderCommitResult{}, NothingToApply
	}
	// Results carry row, outcome and code only: an order number or customer id in a retained batch would outlive an erasure.
	results := make([]batchResult, 0, len(parsed.units))
	for _, u := range parsed.units {
		results = append(results, batchResult{Row: u.n, Outcome: u.outcome, Code: u.code + unitWarning(u)})
	}
	id, err := recordBatch(ctx, tx, scope, token, kindOrders, sum, parsed.mapping, len(parsed.units), created, updated, failed, results)
	if err != nil {
		return OrderCommitResult{}, err
	}
	return OrderCommitResult{BatchID: id, Created: created, Updated: updated, Failed: failed}, nil
}

// runOrders parses the file and sends every valid unit to the database in chunks; on return each unit has its final outcome
// (created | updated | failed). A file-level defect refuses the whole file (nothing sent).
func runOrders(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, data []byte, mapping map[string]string) (parsedOrders, error) {
	parsed, code := parseOrderCSV(data, mapping)
	if code != "" {
		return parsed, fileError(code)
	}
	var wire []orderWireRow
	var index []int // wire position -> parsed.units index
	for i, u := range parsed.units {
		if u.outcome == "" {
			wire = append(wire, orderWireRow{Row: u.n, OrderID: u.orderID, CustomerID: u.customerID, OrderedAt: u.orderedAt.Format("2006-01-02T15:04:05Z"),
				Status: u.status, TotalMinor: u.totalMinor, Items: u.items, City: u.city})
			index = append(index, i)
		}
	}
	hash := tokenHash(token)
	for start := 0; start < len(wire); start += chunkRows {
		end := min(start+chunkRows, len(wire))
		body, err := json.Marshal(wire[start:end])
		if err != nil {
			return parsed, err
		}
		var raw []byte
		// Calls migrationimport.import_orders (customers:privacy): one archive row per order, no payment / inventory / finance write.
		if err := tx.QueryRow(ctx, `SELECT migrationimport.import_orders($1,$2::uuid,$3::jsonb)`, hash[:], scope.StoreID, string(body)).Scan(&raw); err != nil {
			return parsed, mapPG(err)
		}
		var outcomes []struct {
			Row     int    `json:"row"`
			Outcome string `json:"outcome"`
			Code    string `json:"code"`
		}
		if json.Unmarshal(raw, &outcomes) != nil || len(outcomes) != end-start {
			return parsed, ErrUnavailable
		}
		for k, o := range outcomes {
			i := index[start+k]
			if o.Row != parsed.units[i].n || (o.Outcome != outcomeCreated && o.Outcome != outcomeUpdated && o.Outcome != outcomeFailed) ||
				(o.Outcome == outcomeFailed && o.Code == "") {
				return parsed, ErrUnavailable
			}
			parsed.units[i].outcome, parsed.units[i].code = o.Outcome, o.Code
		}
	}
	return parsed, nil
}

// unitWarning is "city_dropped" for an applicable (not failed) unit whose city cell was refused, else "". Batch results carry it in the
// code column of a created / updated row; a failed row keeps its failure code only.
func unitWarning(u orderUnit) string {
	if u.cityDropped && u.outcome != outcomeFailed {
		return "city_dropped"
	}
	return ""
}

func tallyOrders(units []orderUnit) (created, updated, failed int) {
	for _, u := range units {
		switch u.outcome {
		case outcomeCreated:
			created++
		case outcomeUpdated:
			updated++
		case outcomeFailed:
			failed++
		}
	}
	return
}

func buildOrderPreview(digest string, p parsedOrders) OrderPreview {
	created, updated, failed := tallyOrders(p.units)
	out := OrderPreview{FileSHA256: digest, Headers: p.headers, Mapping: p.mapping, RowsTotal: len(p.units), NewRows: created, UpdateRows: updated,
		ApplyRows: created + updated, FailedRows: failed, Rows: make([]OrderPreviewRow, 0, len(p.units))}
	if out.Headers == nil {
		out.Headers = []string{}
	}
	if out.Mapping == nil {
		out.Mapping = map[string]string{}
	}
	for _, u := range p.units {
		if u.code == "erased" {
			out.ErasedRows++
		}
		id := u.orderID
		if u.outcome == outcomeFailed {
			id = "" // a failed unit keeps its row number and code only
		}
		if w := unitWarning(u); w != "" {
			out.CityDroppedRows++
			out.Rows = append(out.Rows, OrderPreviewRow{Row: u.n, ExternalID: id, Outcome: u.outcome, Warning: w})
			continue
		}
		out.Rows = append(out.Rows, OrderPreviewRow{Row: u.n, ExternalID: id, Outcome: u.outcome, Code: u.code})
	}
	return out
}
