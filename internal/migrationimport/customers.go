// Purpose: the orchestration half of the customer CSV import (W5-02B, migration-import-v1): the preview (always rolled back), the
//   idempotent commit and the row-by-row apply through the migrationimport.import_customers definer.
// Depends on: customers_csv.go (parse, mapping, cell rules), batch.go (recordBatch, errors), internal/command (Run), internal/platform
//   (Scope), pgx/v5; SQL migrationimport.import_customers / record_batch (migration 0152, customers:privacy).
// Used by: internal/httpapi/imports.go (the preview / commit routes); pinned by customers_test.go (DB-free) and
//   tests/foundation/customer_import_test.go (REAL_PG).
// Invariants: imported consent is unknown (nothing here writes customers.consent_events); the commit applies the file only when the
//   applicable set still equals the preview's count; the same bytes commit once (file hash = idempotency key); the uploaded content
//   and every cell stay out of logs, errors, audit rows and ops.command_results.
// Status: REAL_PG + MOCK (synthetic data).
//
// The same apply code serves the preview (its transaction is rolled back by the caller) and the commit (kept only when the count
// matches), so what the preview promised is what the commit does. Rows that fail validation are never sent to the database; the
// database call is all-or-nothing, so a database failure aborts the whole import (503 retry_later) instead of leaving a partial file.

package migrationimport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// kindCustomers is the migrationimport.batches.kind of this import.
const kindCustomers = "customers"

// PreviewRow is one data row's verdict in the preview response (no name, phone or email: only enough to correlate).
type PreviewRow struct {
	Row            int    `json:"row"`
	ExternalID     string `json:"external_id"`
	Outcome        string `json:"outcome"` // created | updated | failed
	Code           string `json:"code,omitempty"`
	ConsentIgnored bool   `json:"consent_ignored,omitempty"`
}

// CustomerPreview is the preview answer (also attached to a 409 preview_stale). Headers and Mapping let the UI show and edit the
// column mapping; ApplyRows = NewRows + UpdateRows is the count the commit must echo as expected_apply_rows.
type CustomerPreview struct {
	FileSHA256 string            `json:"file_sha256"`
	Headers    []string          `json:"headers"`
	Mapping    map[string]string `json:"mapping"`
	RowsTotal  int               `json:"rows_total"`
	NewRows    int               `json:"new_rows"`
	UpdateRows int               `json:"update_rows"`
	ApplyRows  int               `json:"apply_rows"`
	FailedRows int               `json:"failed_rows"`
	// ErasedRows counts failed rows whose source id carries an erasure tombstone (code erased); they never show their id.
	ErasedRows         int          `json:"erased_rows"`
	ConsentIgnoredRows int          `json:"consent_ignored_rows"`
	Rows               []PreviewRow `json:"rows"`
}

// CustomerCommitResult is the commit answer (replayed:true on an idempotent re-submit of the same file and count).
type CustomerCommitResult struct {
	BatchID  string `json:"batch_id"`
	Created  int    `json:"created"`
	Updated  int    `json:"updated"`
	Failed   int    `json:"failed"`
	Replayed bool   `json:"replayed"`
}

// PreviewStaleError is the commit refusal when the applicable set drifted after the preview: the handler answers 409 preview_stale
// with the fresh preview so the merchant re-confirms the new count.
type PreviewStaleError struct {
	Preview CustomerPreview
}

func (e *PreviewStaleError) Error() string { return "customer import preview stale" }

// importWireRow is one element of the p_rows array of migrationimport.import_customers.
type importWireRow struct {
	Row        int    `json:"row"`
	ExternalID string `json:"external_id"`
	Name       string `json:"name"`
	// Phone and Email are pointers so an UNMAPPED column is absent from the JSON (the definer keeps the stored value) while a mapped
	// column with an empty cell is "" (the definer clears it).
	Phone *string `json:"phone,omitempty"`
	Email *string `json:"email,omitempty"`
}

// ParseMapping decodes the ?mapping= query value: a JSON object field -> header (external_id, name, phone, email, consent), at most
// 2 KiB, each header 1..100 characters or "" to unmap a field. An empty string means no mapping (auto-detect everything).
func ParseMapping(raw string) (map[string]string, error) {
	if raw == "" {
		return nil, nil
	}
	if len(raw) > 2048 {
		return nil, fileError("invalid_request")
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil || m == nil {
		return nil, fileError("invalid_request")
	}
	for _, h := range m {
		if len([]rune(h)) > 100 {
			return nil, fileError("invalid_request")
		}
	}
	return m, nil
}

// CustomersPreview parses and dry-runs the file; it always returns ErrPreviewRolledBack with the result (the caller serves it and
// rolls the transaction back). A file-level defect is a coded *Error. Authority customers:privacy.
func CustomersPreview(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, data []byte, mapping map[string]string) (CustomerPreview, error) {
	if err := validateImport(tx, scope, token, data); err != nil {
		return CustomerPreview{}, err
	}
	digest, _ := digestOf(data)
	parsed, err := runCustomers(ctx, tx, scope, token, data, mapping)
	if err != nil {
		return CustomerPreview{}, err
	}
	return buildPreview(digest, parsed), ErrPreviewRolledBack
}

// CustomersCommit applies the file once. Idempotent per file hash + mapping + expected_apply_rows: a re-run with the same triple
// replays the first summary; the same bytes with a different mapping or count is 409 idempotency_conflict. A commit whose actual
// applicable set differs from the preview's count commits nothing (409 preview_stale with a fresh preview); zero applicable rows is
// 422 nothing_to_apply. Writes buyer.owners + customers.import_profiles + migrationimport.external_ids + one batch + one audit row.
func CustomersCommit(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, data []byte, mapping map[string]string, expectedApplyRows int64) (CustomerCommitResult, error) {
	empty := CustomerCommitResult{}
	if err := validateImport(tx, scope, token, data); err != nil {
		return empty, err
	}
	if expectedApplyRows < 0 || expectedApplyRows > MaxRows {
		return empty, command.ErrInvalid
	}
	digest, sum := digestOf(data)
	var res, attempt CustomerCommitResult
	executed := false
	var applyErr error
	err := command.Run(ctx, tx, scope, "migrationimport.customers_commit", "cimp-"+digest[:32], struct {
		SHA               string            `json:"sha256"`
		Mapping           map[string]string `json:"mapping"`
		ExpectedApplyRows int64             `json:"expected_apply_rows"`
	}{digest, mapping, expectedApplyRows}, &res, func() error {
		executed = true
		attempt, applyErr = applyCustomers(ctx, tx, scope, token, data, mapping, digest, sum, expectedApplyRows)
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

func validateImport(tx pgx.Tx, scope platform.Scope, token string, data []byte) error {
	if !validAuthority(tx, scope, token) {
		return command.ErrInvalid
	}
	if len(data) == 0 || len(data) > MaxCSVBytes {
		return &Error{Status: http.StatusRequestEntityTooLarge, Code: "invalid_request"}
	}
	return nil
}

// applyCustomers runs the shared loop for the commit and enforces the preview/commit contract: the actual applicable count must equal
// the preview's, otherwise nothing is written (the batch insert only happens on the nil-return path inside the command transaction).
func applyCustomers(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, data []byte, mapping map[string]string,
	digest string, sum [32]byte, expectedApplyRows int64) (CustomerCommitResult, error) {
	parsed, err := runCustomers(ctx, tx, scope, token, data, mapping)
	if err != nil {
		return CustomerCommitResult{}, err
	}
	created, updated, failed := tally(parsed.rows)
	if int64(created+updated) != expectedApplyRows {
		return CustomerCommitResult{}, &PreviewStaleError{Preview: buildPreview(digest, parsed)}
	}
	if created+updated == 0 {
		return CustomerCommitResult{}, NothingToApply
	}
	results := make([]batchResult, 0, len(parsed.rows))
	for _, r := range parsed.rows {
		results = append(results, batchResult{Row: r.n, Outcome: r.outcome, Code: r.code, ExternalID: r.externalID})
	}
	id, err := recordBatch(ctx, tx, scope, token, kindCustomers, sum, parsed.mapping, len(parsed.rows), created, updated, failed, results)
	if err != nil {
		return CustomerCommitResult{}, err
	}
	return CustomerCommitResult{BatchID: id, Created: created, Updated: updated, Failed: failed}, nil
}

// runCustomers parses the file and sends every valid row to the database in chunks; on return each row has its final outcome
// (created | updated | failed). A file-level defect refuses the whole file (nothing sent).
func runCustomers(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, data []byte, mapping map[string]string) (parsedCustomers, error) {
	parsed, code := parseCustomerCSV(data, mapping)
	if code != "" {
		return parsed, fileError(code)
	}
	var wire []importWireRow
	var index []int // wire position -> parsed.rows index
	for i, r := range parsed.rows {
		if r.outcome == "" {
			w := importWireRow{Row: r.n, ExternalID: r.externalID, Name: r.name}
			if _, mapped := parsed.mapping[fieldPhone]; mapped {
				w.Phone = &parsed.rows[i].phone
			}
			if _, mapped := parsed.mapping[fieldEmail]; mapped {
				w.Email = &parsed.rows[i].email
			}
			wire = append(wire, w)
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
		// Calls migrationimport.import_customers (customers:privacy): owner + profile + external id per row, no consent write.
		if err := tx.QueryRow(ctx, `SELECT migrationimport.import_customers($1,$2::uuid,$3::jsonb)`, hash[:], scope.StoreID, string(body)).Scan(&raw); err != nil {
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
			erased := o.Outcome == outcomeFailed && o.Code == "erased"
			if o.Row != parsed.rows[i].n || (o.Outcome != outcomeCreated && o.Outcome != outcomeUpdated && !erased) {
				return parsed, ErrUnavailable
			}
			parsed.rows[i].outcome = o.Outcome
			if erased {
				// The row keeps its number and code only: the erased source id must not reappear in a preview, batch or results.csv.
				parsed.rows[i].code, parsed.rows[i].externalID = "erased", ""
			}
		}
	}
	return parsed, nil
}

func tally(rows []customerRow) (created, updated, failed int) {
	for _, r := range rows {
		switch r.outcome {
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

func buildPreview(digest string, p parsedCustomers) CustomerPreview {
	created, updated, failed := tally(p.rows)
	out := CustomerPreview{FileSHA256: digest, Headers: p.headers, Mapping: p.mapping, RowsTotal: len(p.rows),
		NewRows: created, UpdateRows: updated, ApplyRows: created + updated, FailedRows: failed, Rows: make([]PreviewRow, 0, len(p.rows))}
	if out.Headers == nil {
		out.Headers = []string{}
	}
	if out.Mapping == nil {
		out.Mapping = map[string]string{}
	}
	for _, r := range p.rows {
		if r.consentIgnored {
			out.ConsentIgnoredRows++
		}
		if r.code == "erased" {
			out.ErasedRows++
		}
		out.Rows = append(out.Rows, PreviewRow{Row: r.n, ExternalID: r.externalID, Outcome: r.outcome, Code: r.code, ConsentIgnored: r.consentIgnored})
	}
	return out
}
