// Purpose: the orchestration half of the bulk tracking import (manual-fulfilment-v1 Amendment "M-7
//   revoked"): the preview (always rolled back), the idempotent commit and the result.csv read. Every
//   applied row goes through merchantorders.RecordShipment unchanged; the commit is one command.Run keyed
//   by the file hash + expected apply count.
// Depends on: internal/merchanttools (tracking_csv.go parse/fold/normalize, csvfile.go guardCell,
//   csvimport.go abort), internal/command (Run/Audit/ValidID), internal/merchantorders
//   (RecordShipment/NormalizeShipment), internal/platform (Scope), pgx/v5; SQL
//   fulfillment.tracking_import_batches and fulfillment.tracking_import_precheck (migration 0124).
// Used by: internal/httpapi/merchanttools.go (the three tracking-import routes); pinned by the DB-free
//   unit tests in internal/merchanttools/tracking_import_test.go and the smoke test
//   tests/foundation/tracking_import_test.go.
//
// tracking_import.go applies an uploaded home-delivery tracking CSV (manual-fulfilment-v1 Amendment
// "M-7 revoked") inside the caller's ONE platform.WithScopeBudget (60 s) READ COMMITTED transaction.
// The same apply code serves the preview (always rolled back) and the commit (kept only when the
// applicable set still matches the preview's count), so what the preview promised is what the commit
// does. Every applied row goes through merchantorders.RecordShipment unchanged — same auth, order lock,
// expected_version CAS, MD6 eligibility, audit and shipped-mail outbox as the single PUT.
//
// Per-row decisions: CSV parse/normalize (tracking_csv.go) -> in-file duplicate -> lock-free precheck
// (fulfillment.tracking_import_precheck) -> unchanged / already_shipped / cvs_order / order_not_found /
// apply -> RecordShipment in a savepoint. A failed row is skipped and reported; a file-level defect
// refuses the whole file. The commit is one command (operation fulfillment.tracking_import, key
// trk-<sha256[:32]>) so the same bytes with the same expected_apply_rows replay the first batch and
// never re-ship or re-mail.
//
// It writes ONLY through merchantorders.RecordShipment and this package's batch table (direct
// INSERT/DELETE under RLS); it never writes the ledger, never touches notify, never logs tracking
// numbers and never reads recipient PII.

package merchanttools

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

	"livecommerce/internal/command"
	"livecommerce/internal/merchantorders"
	"livecommerce/internal/platform"
)

// Row outcome values (also stored in the batch results jsonb and the result.csv).
const (
	trackingOutcomeApply     = "apply"
	trackingOutcomeUnchanged = "unchanged"
	trackingOutcomeFailed    = "failed"
)

// TrackingImportRow is one data row's verdict in the preview response (no carrier_name/tracking_url:
// the preview shows enough to correlate, never the full shipment detail).
type TrackingImportRow struct {
	Row            int    `json:"row"`
	OrderNumber    string `json:"order_number"` // canonical uuid; raw cell when the ref did not parse
	CarrierCode    string `json:"carrier_code"`
	TrackingNumber string `json:"tracking_number"`
	Outcome        string `json:"outcome"`
	Code           string `json:"code,omitempty"`
}

// TrackingImportResult is the preview answer (also attached to a 409 preview_stale).
type TrackingImportResult struct {
	FileSHA256    string              `json:"file_sha256"`
	RowsTotal     int                 `json:"rows_total"`
	ApplyRows     int                 `json:"apply_rows"`
	UnchangedRows int                 `json:"unchanged_rows"`
	FailedRows    int                 `json:"failed_rows"`
	Rows          []TrackingImportRow `json:"rows"`
	MailETAHours  int                 `json:"mail_eta_hours"`
}

// TrackingImportCommitResult is the commit answer (replayed:true on an idempotent re-submit).
type TrackingImportCommitResult struct {
	BatchID   string `json:"batch_id"`
	Applied   int    `json:"applied"`
	Unchanged int    `json:"unchanged"`
	Failed    int    `json:"failed"`
	Replayed  bool   `json:"replayed"`
}

// trackingImportBatchResult is one element of the batch's results jsonb (order_id, never buyer PII).
type trackingImportBatchResult struct {
	Row            int     `json:"row"`
	OrderID        *string `json:"order_id,omitempty"`
	CarrierCode    string  `json:"carrier_code"`
	CarrierName    *string `json:"carrier_name,omitempty"`
	TrackingNumber string  `json:"tracking_number"`
	Outcome        string  `json:"outcome"`
	Code           string  `json:"code,omitempty"`
}

// trackingPrecheckEntry is one order of fulfillment.tracking_import_precheck's jsonb array.
type trackingPrecheckEntry struct {
	OrderID        string  `json:"order_id"`
	Found          bool    `json:"found"`
	CVS            bool    `json:"cvs"`
	HeadVersion    int64   `json:"head_version"`
	HeadStatus     *string `json:"head_status"`
	CarrierCode    *string `json:"carrier_code"`
	CarrierName    *string `json:"carrier_name"`
	TrackingNumber *string `json:"tracking_number"`
	TrackingURL    *string `json:"tracking_url"`
}

// trackingRowResult is one row's internal verdict, before the public projection.
type trackingRowResult struct {
	row            int
	orderID        string // canonical uuid ("" when the cell did not parse)
	orderRef       string // raw cell, for the preview order_number fallback
	carrierCode    string
	carrierName    *string
	trackingNumber string
	trackingURL    *string
	outcome        string // "" = pending, then apply | unchanged | failed
	code           string
	version        int64 // expected_version for apply rows (head version or 0)
}

type trackingOutcome struct {
	apply     int
	unchanged int
	failed    int
	rows      []trackingRowResult
}

// PreviewStaleError is the commit refusal when the applicable set drifted after the preview: the
// handler returns 409 preview_stale with the fresh preview so the merchant re-confirms the new count.
type PreviewStaleError struct {
	Preview TrackingImportResult
}

func (e *PreviewStaleError) Error() string { return "tracking import preview stale" }

// NothingToApply is the commit refusal when nothing would apply (422 nothing_to_apply, no batch).
var NothingToApply = &Error{Status: http.StatusUnprocessableEntity, Code: "nothing_to_apply"}

// TrackingImportPreview parses and dry-runs the file; it always returns ErrPreviewRolledBack with the
// result (the caller serves it and rolls the transaction back). A file-level defect is a coded *Error.
func TrackingImportPreview(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, data []byte) (TrackingImportResult, error) {
	if err := validateTrackingImport(tx, scope, token, data); err != nil {
		return TrackingImportResult{}, err
	}
	digest, _ := trackingDigest(data)
	out, err := runTrackingImport(ctx, tx, scope, token, data, digest)
	if err != nil {
		return TrackingImportResult{}, err
	}
	return buildTrackingPreview(digest, out), ErrPreviewRolledBack
}

// TrackingImportCommit applies the file once. Idempotent per file hash + expected_apply_rows: a re-run
// with the same pair replays the first summary; the same file with a different count is a 409
// idempotency_conflict. A commit whose actual applicable set differs from the preview's count commits
// nothing (409 preview_stale with a fresh preview); zero applicable rows is 422 nothing_to_apply.
func TrackingImportCommit(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, data []byte, expectedApplyRows int64) (TrackingImportCommitResult, error) {
	empty := TrackingImportCommitResult{}
	if err := validateTrackingImport(tx, scope, token, data); err != nil {
		return empty, err
	}
	if expectedApplyRows < 0 || expectedApplyRows > MaxTrackingImportRows {
		return empty, command.ErrInvalid
	}
	digest, sum := trackingDigest(data)
	var res, attempt TrackingImportCommitResult
	executed := false
	var applyErr error
	err := command.Run(ctx, tx, scope, "fulfillment.tracking_import", "trk-"+digest[:32], struct {
		SHA               string `json:"sha256"`
		ExpectedApplyRows int64  `json:"expected_apply_rows"`
	}{digest, expectedApplyRows}, &res, func() error {
		executed = true
		attempt, applyErr = applyTrackingImport(ctx, tx, scope, token, data, digest, sum, expectedApplyRows)
		if applyErr != nil {
			return applyErr
		}
		res = attempt
		return command.Audit(ctx, tx, scope, "fulfillment.tracking_imported")
	})
	switch {
	case err == nil:
		res.Replayed = !executed
		return res, nil
	case errors.Is(err, command.ErrConflict) && !executed:
		return empty, &Error{Status: http.StatusConflict, Code: "idempotency_conflict"}
	}
	if applyErr != nil {
		return attempt, applyErr
	}
	return empty, err
}

// TrackingResultCSV reads one committed batch and renders its result.csv: UTF-8 BOM (Excel), CRLF via
// encoding/csv, guardCell on every cell, columns row,order_number,carrier,tracking_number,outcome,code.
// onlyFailed keeps just the failed rows. There is no recipient, phone or address anywhere in the batch.
func TrackingResultCSV(ctx context.Context, tx pgx.Tx, scope platform.Scope, batchID string, onlyFailed bool) ([]byte, error) {
	if tx == nil || !command.ValidID(scope.TenantID) || !command.ValidID(scope.StoreID) || !command.ValidID(batchID) {
		return nil, command.ErrInvalid
	}
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT results FROM fulfillment.tracking_import_batches
		WHERE tenant_id=$1 AND store_id=$2 AND id=$3`, scope.TenantID, scope.StoreID, batchID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, command.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var results []trackingImportBatchResult
	if err := json.Unmarshal(raw, &results); err != nil || len(results) > MaxTrackingImportRows {
		return nil, ErrUnavailable
	}
	var b bytes.Buffer
	b.WriteString("\xEF\xBB\xBF") // UTF-8 BOM so Taiwan spreadsheet tools open it as text
	w := csv.NewWriter(&b)
	_ = w.Write([]string{"row", "order_number", "carrier", "tracking_number", "outcome", "code"})
	for _, r := range results {
		if onlyFailed && r.Outcome != trackingOutcomeFailed {
			continue
		}
		orderNumber := ""
		if r.OrderID != nil {
			orderNumber = *r.OrderID
		}
		_ = w.Write([]string{strconv.Itoa(r.Row), guardCell(orderNumber), guardCell(r.CarrierCode),
			guardCell(r.TrackingNumber), guardCell(r.Outcome), guardCell(r.Code)})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func trackingDigest(data []byte) (string, [32]byte) {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), sum
}

func validateTrackingImport(tx pgx.Tx, scope platform.Scope, token string, data []byte) error {
	if tx == nil || !command.ValidID(scope.TenantID) || !command.ValidID(scope.StoreID) || !command.ValidID(scope.PrincipalID) ||
		scope.Revision <= 0 || len(token) < 32 || len(token) > 512 {
		return command.ErrInvalid
	}
	if len(data) == 0 || len(data) > MaxCSVBytes {
		return &Error{Status: http.StatusRequestEntityTooLarge, Code: "invalid_request"}
	}
	return nil
}

// applyTrackingImport runs the shared row loop for the commit and enforces the preview/commit contract:
// the actual applicable count must equal the preview's, otherwise nothing is written (the batch insert
// only happens on the nil-return path, inside the caller's command.Run transaction).
func applyTrackingImport(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, data []byte, digest string, sum [32]byte, expectedApplyRows int64) (TrackingImportCommitResult, error) {
	out, err := runTrackingImport(ctx, tx, scope, token, data, digest)
	if err != nil {
		return TrackingImportCommitResult{}, err
	}
	if int64(out.apply) != expectedApplyRows {
		return TrackingImportCommitResult{}, &PreviewStaleError{Preview: buildTrackingPreview(digest, out)}
	}
	if out.apply == 0 {
		return TrackingImportCommitResult{}, NothingToApply
	}
	batchID, err := insertTrackingBatch(ctx, tx, scope, sum, out)
	if err != nil {
		return TrackingImportCommitResult{}, err
	}
	return TrackingImportCommitResult{BatchID: batchID, Applied: out.apply, Unchanged: out.unchanged, Failed: out.failed}, nil
}

func runTrackingImport(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, data []byte, digest string) (trackingOutcome, error) {
	out := trackingOutcome{rows: []trackingRowResult{}}
	rawRows, fileCode := parseTrackingCSV(data)
	if fileCode != "" {
		return out, trackingFileError(fileCode)
	}
	rows := make([]trackingRowResult, 0, len(rawRows))
	for _, raw := range rawRows {
		rows = append(rows, normalizeTrackingRow(raw))
	}
	// In-file duplicate: the same order in two still-valid rows refuses both rows (never guess which one is
	// right). A row already refused keeps its own more specific code and does not take part.
	byOrder := map[string][]int{}
	for i := range rows {
		if rows[i].outcome == "" && rows[i].orderID != "" {
			byOrder[rows[i].orderID] = append(byOrder[rows[i].orderID], i)
		}
	}
	for _, idxs := range byOrder {
		if len(idxs) < 2 {
			continue
		}
		for _, i := range idxs {
			rows[i].outcome = trackingOutcomeFailed
			rows[i].code = "duplicate_order"
		}
	}
	// Lock-free precheck for every still-pending unique order.
	pre := map[string]trackingPrecheckEntry{}
	seen := map[string]bool{}
	var orderIDs []string
	for i := range rows {
		if rows[i].outcome == "" && rows[i].orderID != "" && !seen[rows[i].orderID] {
			seen[rows[i].orderID] = true
			orderIDs = append(orderIDs, rows[i].orderID)
		}
	}
	if len(orderIDs) > 0 {
		entries, err := trackingPrecheck(ctx, tx, scope, token, orderIDs)
		if err != nil {
			return out, err
		}
		for i := range entries {
			pre[entries[i].OrderID] = entries[i]
		}
	}
	for i := range rows {
		r := &rows[i]
		if r.outcome != "" {
			continue
		}
		entry := pre[r.orderID]
		switch {
		case !entry.Found:
			// Missing and other-store are indistinguishable (I01), exactly like the single PUT's 404.
			r.outcome, r.code = trackingOutcomeFailed, "order_not_found"
		case entry.CVS:
			r.outcome, r.code = trackingOutcomeFailed, "cvs_order"
		case entry.HeadStatus != nil && *entry.HeadStatus == "SHIPPED":
			if sameShipment(entry, *r) {
				r.outcome = trackingOutcomeUnchanged
			} else {
				// Never overwrite a shipped head from bulk import; a change goes through the single PUT.
				r.outcome, r.code = trackingOutcomeFailed, "already_shipped"
			}
		default:
			r.version = entry.HeadVersion
			r.outcome = trackingOutcomeApply
		}
	}
	// Apply: the same command and audit as the single PUT, one savepoint per row. A domain refusal is a
	// row code; anything else (deadline, lost connection, unknown SQL) aborts the whole import.
	for i := range rows {
		r := &rows[i]
		if r.outcome != trackingOutcomeApply {
			continue
		}
		in := merchantorders.ShipmentInput{
			ExpectedVersion: r.version,
			Status:          "SHIPPED",
			CarrierCode:     &r.carrierCode,
			CarrierName:     r.carrierName,
			TrackingNumber:  &r.trackingNumber,
			TrackingURL:     r.trackingURL,
		}
		err := trackingSavepoint(ctx, tx, func(sp pgx.Tx) error {
			_, err := merchantorders.RecordShipment(ctx, sp, scope, token, trackingRowKey(digest, r.row), r.orderID, in)
			return err
		})
		if err != nil {
			if code := trackingShipmentCode(err); code != "" {
				r.outcome, r.code = trackingOutcomeFailed, code
				continue
			}
			return out, abort(err)
		}
	}
	for _, r := range rows {
		switch r.outcome {
		case trackingOutcomeApply:
			out.apply++
		case trackingOutcomeUnchanged:
			out.unchanged++
		case trackingOutcomeFailed:
			out.failed++
		}
	}
	out.rows = rows
	return out, nil
}

// normalizeTrackingRow validates one parsed row's cells with the same rules as the single PUT
// (merchantorders.NormalizeShipment) and returns its canonical values. outcome is "failed" with code on
// any cell defect, "" otherwise (still pending the precheck).
func normalizeTrackingRow(raw trackingCSVRow) trackingRowResult {
	r := trackingRowResult{row: raw.n, orderRef: raw.orderRef}
	if raw.code != "" {
		r.outcome, r.code = trackingOutcomeFailed, raw.code
		return r
	}
	if raw.orderRef == "" {
		r.outcome, r.code = trackingOutcomeFailed, "required"
		return r
	}
	orderID, ok := normalizeTrackingOrderRef(raw.orderRef)
	if !ok {
		r.outcome, r.code = trackingOutcomeFailed, "invalid_order_ref"
		return r
	}
	r.orderID = orderID
	if raw.carrier == "" {
		r.outcome, r.code = trackingOutcomeFailed, "required"
		return r
	}
	carrierCode, ok := trackingCarrierAliases[foldTrackingHeader(raw.carrier)]
	if !ok {
		r.outcome, r.code = trackingOutcomeFailed, "invalid_carrier"
		return r
	}
	r.carrierCode = carrierCode
	if raw.trackingNumber == "" {
		r.outcome, r.code = trackingOutcomeFailed, "required"
		return r
	}
	var name *string
	if raw.carrierName != "" {
		name = &raw.carrierName
	}
	if carrierCode == "other" && name == nil {
		r.outcome, r.code = trackingOutcomeFailed, "carrier_name_required"
		return r
	}
	var link *string
	if raw.trackingURL != "" {
		link = &raw.trackingURL
	}
	norm, err := merchantorders.NormalizeShipment(merchantorders.ShipmentInput{
		Status:         "SHIPPED",
		CarrierCode:    &carrierCode,
		CarrierName:    name,
		TrackingNumber: &raw.trackingNumber,
		TrackingURL:    link,
	})
	if err != nil {
		r.outcome, r.code = trackingOutcomeFailed, trackingNormalizeCode(err)
		return r
	}
	r.carrierName = norm.CarrierName
	r.trackingNumber = *norm.TrackingNumber
	r.trackingURL = norm.TrackingURL
	return r
}

func trackingNormalizeCode(err error) string {
	switch {
	case errors.Is(err, merchantorders.ErrInvalidCarrier):
		return "invalid_carrier"
	case errors.Is(err, merchantorders.ErrInvalidTracking):
		return "invalid_tracking"
	case errors.Is(err, merchantorders.ErrInvalidURL):
		return "invalid_url"
	default:
		return "invalid_request"
	}
}

// sameShipment is rule 5's unchanged test: the shipped head carries exactly this row's carrier,
// tracking, carrier_name and tracking_url.
func sameShipment(entry trackingPrecheckEntry, r trackingRowResult) bool {
	if entry.CarrierCode == nil || entry.TrackingNumber == nil ||
		*entry.CarrierCode != r.carrierCode || *entry.TrackingNumber != r.trackingNumber {
		return false
	}
	return sameNullableStr(entry.CarrierName, r.carrierName) && sameNullableStr(entry.TrackingURL, r.trackingURL)
}

func sameNullableStr(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func trackingPrecheck(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, orderIDs []string) ([]trackingPrecheckEntry, error) {
	auth := sha256.Sum256([]byte(token))
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT fulfillment.tracking_import_precheck($1,$2::uuid,$3::uuid[])`,
		auth[:], scope.StoreID, orderIDs).Scan(&raw); err != nil {
		return nil, err
	}
	var entries []trackingPrecheckEntry
	if err := json.Unmarshal(raw, &entries); err != nil || len(entries) != len(orderIDs) {
		return nil, ErrUnavailable
	}
	return entries, nil
}

func trackingSavepoint(ctx context.Context, tx pgx.Tx, fn func(pgx.Tx) error) error {
	sp, err := tx.Begin(ctx)
	if err != nil {
		return err
	}
	if err = fn(sp); err != nil {
		_ = sp.Rollback(ctx)
		return err
	}
	return sp.Commit(ctx)
}

func trackingRowKey(digest string, row int) string {
	return fmt.Sprintf("trk-%s-r%d", digest[:24], row)
}

func trackingShipmentCode(err error) string {
	switch {
	case errors.Is(err, merchantorders.ErrVersionChanged):
		return "version_changed"
	case errors.Is(err, merchantorders.ErrNotShippable):
		return "not_shippable"
	case errors.Is(err, merchantorders.ErrInvalidCarrier):
		return "invalid_carrier"
	case errors.Is(err, merchantorders.ErrInvalidTracking):
		return "invalid_tracking"
	case errors.Is(err, merchantorders.ErrInvalidURL):
		return "invalid_url"
	case errors.Is(err, merchantorders.ErrVoidRequiresShipped):
		return "void_requires_shipped"
	case errors.Is(err, merchantorders.ErrInvalidVoid):
		return "invalid_void"
	case errors.Is(err, command.ErrConflict):
		return "conflict"
	case errors.Is(err, command.ErrInvalid):
		return "invalid_request"
	default:
		return ""
	}
}

func trackingFileError(code string) error {
	return &Error{Status: http.StatusUnprocessableEntity, Code: code}
}

func buildTrackingPreview(digest string, out trackingOutcome) TrackingImportResult {
	res := TrackingImportResult{
		FileSHA256:    digest,
		RowsTotal:     len(out.rows),
		ApplyRows:     out.apply,
		UnchangedRows: out.unchanged,
		FailedRows:    out.failed,
		Rows:          make([]TrackingImportRow, 0, len(out.rows)),
		MailETAHours:  mailETAHours(out.apply),
	}
	for _, r := range out.rows {
		orderNumber := r.orderID
		if orderNumber == "" {
			orderNumber = r.orderRef
		}
		res.Rows = append(res.Rows, TrackingImportRow{
			Row:            r.row,
			OrderNumber:    orderNumber,
			CarrierCode:    r.carrierCode,
			TrackingNumber: r.trackingNumber,
			Outcome:        r.outcome,
			Code:           r.code,
		})
	}
	return res
}

// mailETAHours is ceil(apply_rows/30): the notify worker mails at most 30 buyer mails per store per
// rolling hour (internal/notify/worker.go storeHourly), so the hint is a lower bound on delivery.
func mailETAHours(applied int) int {
	if applied <= 0 {
		return 0
	}
	return (applied + 29) / 30
}

// insertTrackingBatch writes the one batch row and lazily prunes this store's batches older than 30
// days (at most 50, I23) in the same transaction.
func insertTrackingBatch(ctx context.Context, tx pgx.Tx, scope platform.Scope, sum [32]byte, out trackingOutcome) (string, error) {
	results := make([]trackingImportBatchResult, 0, len(out.rows))
	for _, r := range out.rows {
		var orderID *string
		if r.orderID != "" {
			orderID = &r.orderID
		}
		results = append(results, trackingImportBatchResult{
			Row:            r.row,
			OrderID:        orderID,
			CarrierCode:    r.carrierCode,
			CarrierName:    r.carrierName,
			TrackingNumber: r.trackingNumber,
			Outcome:        r.outcome,
			Code:           r.code,
		})
	}
	raw, err := json.Marshal(results)
	if err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM fulfillment.tracking_import_batches WHERE tenant_id=$1 AND store_id=$2
		AND id IN (SELECT id FROM fulfillment.tracking_import_batches WHERE tenant_id=$1 AND store_id=$2
			AND created_at < clock_timestamp() - interval '30 days' LIMIT 50)`, scope.TenantID, scope.StoreID); err != nil {
		return "", err
	}
	var batchID string
	err = tx.QueryRow(ctx, `INSERT INTO fulfillment.tracking_import_batches
		(tenant_id,store_id,id,file_sha256,principal_id,rows_total,applied,unchanged,failed,results,created_at)
		VALUES($1,$2,gen_random_uuid(),$3,$4,$5::smallint,$6::smallint,$7::smallint,$8::smallint,$9::jsonb,clock_timestamp())
		RETURNING id::text`, scope.TenantID, scope.StoreID, sum[:], scope.PrincipalID,
		len(out.rows), out.apply, out.unchanged, out.failed, string(raw)).Scan(&batchID)
	if err != nil {
		return "", err
	}
	return batchID, nil
}
