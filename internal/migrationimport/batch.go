// Purpose: the shared plumbing of the merchant CSV import framework (W5-01B minimal, folded into W5-02B): the coded refusals
//   the HTTP layer maps, the file digest, the PG error mapping that never carries uploaded content, and the batch record / results
//   readers over the migrationimport definers. Customer-specific parsing and the apply loop live in customers.go / customers_csv.go.
// Depends on: pgx/v5 (pgconn codes), internal/command (ValidID, ErrInvalid, ErrNotFound), internal/platform (Scope, ErrForbidden,
//   ErrUnauthorized), internal/csvguard (Cell); SQL migrationimport.record_batch / read_batch_results (migration 0152,
//   customers:privacy; owner commerce_privacy_writer, EXECUTE commerce_runtime).
// Used by: customers.go (preview/commit), internal/httpapi/imports.go (routes, error classification).
// Invariants: the uploaded bytes never reach a log or an error message (only a PG error code does); results rows carry
//   {row, outcome, code, external_id} and never a name, phone or email (migration-import-v1 section 4).
// Status: REAL_PG + MOCK (synthetic data).

package migrationimport

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/command"
	"livecommerce/internal/csvguard"
	"livecommerce/internal/platform"
)

// Import limits (migration-import-v1 section 3): the file is bounded before parsing, the rows before the first database call.
const (
	MaxCSVBytes = 2 << 20
	MaxRows     = 5000
	// chunkRows is the row count of one migrationimport.import_customers call (the definer refuses more).
	chunkRows = 500
)

// Error is a refusal that carries its HTTP status and its contract code (internal/httperror knows every code used here).
type Error struct {
	Status int
	Code   string
}

func (e *Error) Error() string { return "migrationimport: " + e.Code }

// Sentinels the HTTP layer classifies. ErrPreviewRolledBack is returned by a dry run after it computed its answer: the transaction
// must roll back (nothing is written) and the caller still serves the result.
var (
	ErrPreviewRolledBack = errors.New("import preview rolled back")
	ErrUnavailable       = &Error{Status: http.StatusServiceUnavailable, Code: "retry_later"}
	// NothingToApply is the commit refusal when no row would create or update anything (422, no batch is written).
	NothingToApply = &Error{Status: http.StatusUnprocessableEntity, Code: "nothing_to_apply"}
	errIdempotency = &Error{Status: http.StatusConflict, Code: "idempotency_conflict"}
)

// fileError is a file-level defect: the whole file is refused (422) and nothing is written.
func fileError(code string) error { return &Error{Status: http.StatusUnprocessableEntity, Code: code} }

// abort wraps a database failure into a retryable refusal. Only the PG error code is kept: a driver message can quote the failing row
// (a name, phone or email), and the file content must never reach a log or an error text.
func abort(cause error) error {
	var pg *pgconn.PgError
	if errors.As(cause, &pg) {
		return fmt.Errorf("%w (pg %s)", ErrUnavailable, pg.Code)
	}
	return fmt.Errorf("%w", ErrUnavailable)
}

// mapPG turns the definers' coded exceptions into the sentinels the HTTP classifier knows; any other database error is abort()ed.
func mapPG(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "PT400":
			return command.ErrInvalid
		case "PT401":
			return platform.ErrUnauthorized
		case "PT403":
			return platform.ErrForbidden
		case "PT404":
			return command.ErrNotFound
		case "40001", "40P01", "55P03", "57014":
			return err // lock waits and deadlocks: the HTTP layer maps them to retry_later
		}
		return abort(err)
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) ||
		errors.Is(err, platform.ErrForbidden) || errors.Is(err, platform.ErrUnauthorized) {
		return err
	}
	return abort(err)
}

func digestOf(data []byte) (string, [32]byte) {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), sum
}

func tokenHash(token string) [32]byte { return sha256.Sum256([]byte(token)) }

func validAuthority(tx pgx.Tx, scope platform.Scope, token string) bool {
	return tx != nil && command.ValidID(scope.TenantID) && command.ValidID(scope.StoreID) && command.ValidID(scope.PrincipalID) &&
		scope.Revision > 0 && len(token) >= 32 && len(token) <= 512
}

// batchResult is one element of the batch results jsonb and of results.csv. ExternalID is the merchant's source-system id (or the
// leading 64 characters of a refused cell); there is deliberately no field for a name, phone or email.
type batchResult struct {
	Row        int    `json:"row"`
	Outcome    string `json:"outcome"`
	Code       string `json:"code,omitempty"`
	ExternalID string `json:"external_id,omitempty"`
}

// recordBatch writes the committed batch through migrationimport.record_batch (customers:privacy; UNIQUE per file hash; lazy
// 90-day prune; audit customers.imported) and returns the batch id.
func recordBatch(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, kind string, sum [32]byte, mapping map[string]string,
	total, created, updated, failed int, results []batchResult) (string, error) {
	mappingJSON, err := json.Marshal(mapping)
	if err != nil {
		return "", err
	}
	resultsJSON, err := json.Marshal(results)
	if err != nil {
		return "", err
	}
	hash := tokenHash(token)
	var id string
	// Calls migrationimport.record_batch (migration-import-v1 section 4); idempotency = UNIQUE(tenant,store,kind,file_sha256).
	err = tx.QueryRow(ctx, `SELECT migrationimport.record_batch($1,$2::uuid,$3,$4,$5::jsonb,$6,$7,$8,$9,$10::jsonb)::text`,
		hash[:], scope.StoreID, kind, sum[:], string(mappingJSON), total, created, updated, failed, string(resultsJSON)).Scan(&id)
	if err != nil {
		return "", mapPG(err)
	}
	return id, nil
}

// ResultsCSV renders one committed batch as results.csv: UTF-8 BOM (Excel), CRLF via encoding/csv, csvguard on every text cell,
// columns row,external_id,outcome,code. onlyFailed keeps just the failed rows. Authority is customers:privacy (the same permission
// that wrote the batch); another store's or an unknown batch id is command.ErrNotFound. No name, phone or email exists in a batch.
func ResultsCSV(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, batchID string, onlyFailed bool) ([]byte, error) {
	if !validAuthority(tx, scope, token) || !command.ValidID(batchID) {
		return nil, command.ErrInvalid
	}
	hash := tokenHash(token)
	var raw []byte
	// Calls migrationimport.read_batch_results (customers:privacy); PT404 for a batch of another store.
	if err := tx.QueryRow(ctx, `SELECT migrationimport.read_batch_results($1,$2::uuid,$3::uuid)`, hash[:], scope.StoreID, batchID).Scan(&raw); err != nil {
		return nil, mapPG(err)
	}
	var results []batchResult
	if err := json.Unmarshal(raw, &results); err != nil || len(results) > MaxRows {
		return nil, ErrUnavailable
	}
	var b bytes.Buffer
	b.WriteString("\xEF\xBB\xBF") // UTF-8 BOM so Taiwan spreadsheet tools open it as text
	w := csv.NewWriter(&b)
	_ = w.Write([]string{"row", "external_id", "outcome", "code"})
	for _, r := range results {
		if onlyFailed && r.Outcome != outcomeFailed {
			continue
		}
		_ = w.Write([]string{strconv.Itoa(r.Row), csvguard.Cell(r.ExternalID), csvguard.Cell(r.Outcome), csvguard.Cell(r.Code)})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
