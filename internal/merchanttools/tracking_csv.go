// Purpose: the pure half of the bulk tracking import (manual-fulfilment-v1 Amendment "M-7 revoked"):
//   the column set with its Chinese aliases, the case/full-width-insensitive header fold, the carrier
//   alias table, the order-reference normalization (LC-… or uuid) and the strict RFC 4180 parser. No
//   database, no clock, no randomness.
// Depends on: encoding/csv, unicode/utf8, livecommerce/internal/command (ValidID),
//   internal/merchanttools csvfile.go (unguardCell).
// Used by: internal/merchanttools/tracking_import.go (runTrackingImport); pinned by the DB-free unit
//   tests in internal/merchanttools/tracking_import_test.go.
//
// tracking_csv.go is the pure half of the bulk tracking import (manual-fulfilment-v1 Amendment "M-7
// revoked"): the column set with its Chinese aliases, the case/full-width/whitespace-insensitive header
// fold, the carrier-code alias table, the order-reference normalization (LC-… or uuid), and the strict
// CSV parser that turns the uploaded bytes into raw rows plus a file-level code. No database, no clock,
// no randomness, so the parse rules are pinned by tracking_import_test.go.
//
// Non-goals: it decides nothing about the store (whether an order exists, is a CVS order or is already
// shipped is tracking_import.go through the precheck and merchantorders.RecordShipment), and it never
// reads a file the caller did not already bound to 2 MiB. It does NOT ship anything.

package merchanttools

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"livecommerce/internal/command"
)

// Bulk tracking-import limits (Amendment). MaxCSVBytes is shared with the product import (2 MiB); the
// 500-row cap is the Amendment's bound: at the notify quota of 30 buyer mails per store per rolling hour
// (internal/notify/worker.go storeHourly) a full file mails out in ~17 h, under the 24 h SKIP threshold.
// MaxTrackingImportRows is the Amendment's row bound (shared with the HTTP commit gate).
const MaxTrackingImportRows = 500

// Column canonical keys.
const (
	tcolOrderNumber    = "order_number"
	tcolCarrier        = "carrier"
	tcolTrackingNumber = "tracking_number"
	tcolCarrierName    = "carrier_name"
	tcolTrackingURL    = "tracking_url"
)

// trackingImportColumns maps a folded header cell to its canonical column key. The brief lists the
// traditional and simplified Chinese aliases; the fold makes matching case / full-width insensitive.
var trackingImportColumns = map[string]string{
	"order_number":    tcolOrderNumber,
	"order_id":        tcolOrderNumber,
	"訂單編號":            tcolOrderNumber,
	"订单编号":            tcolOrderNumber,
	"carrier":         tcolCarrier,
	"物流商":             tcolCarrier,
	"承運商":             tcolCarrier,
	"承运商":             tcolCarrier,
	"tracking_number": tcolTrackingNumber,
	"運單號碼":            tcolTrackingNumber,
	"运单号":             tcolTrackingNumber,
	"託運單號":            tcolTrackingNumber,
	"carrier_name":    tcolCarrierName,
	"物流商名稱":           tcolCarrierName,
	"tracking_url":    tcolTrackingURL,
	"追蹤網址":            tcolTrackingURL,
}

// trackingCarrierAliases maps a folded carrier cell to its carrier_code. Only the four home-delivery
// carriers the Amendment opens are accepted (ruling T-OPEN-3): black_cat, hsinchu, chunghwa_post, other.
// sf_express and the CVS chains are deliberately absent — CVS orders are refused by the precheck, and
// "other" plus a carrier_name covers every not-yet-aliased courier.
var trackingCarrierAliases = map[string]string{
	"black_cat":     "black_cat",
	"黑貓":            "black_cat",
	"黑猫":            "black_cat",
	"宅急便":           "black_cat",
	"統一速達":          "black_cat",
	"hsinchu":       "hsinchu",
	"新竹":            "hsinchu",
	"新竹物流":          "hsinchu",
	"chunghwa_post": "chunghwa_post",
	"郵局":            "chunghwa_post",
	"邮局":            "chunghwa_post",
	"中華郵政":          "chunghwa_post",
	"other":         "other",
	"其他":            "other",
}

// orderLC matches an LC-… order number: the LC- prefix followed by the 32 hex digits of a uuid with the
// hyphens removed (contracts/merchant-orders-v2.md: order_number = 'LC-' || upper(replace(id,'-',”))).
var orderLC = regexp.MustCompile(`^lc-([0-9a-f]{32})$`)

// foldTrackingHeader makes one header cell comparable: trims, maps full-width ASCII (U+FF01..U+FF5E)
// and the ideographic space to their half-width forms, and lower-cases. Chinese alias cells are left
// untouched (they carry no width variants), so both 訂單編號 and 订单编号 fold to themselves.
func foldTrackingHeader(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '　':
			b.WriteRune(' ')
		case r >= '！' && r <= '～':
			b.WriteRune(r - 0xfee0)
		default:
			b.WriteRune(r)
		}
	}
	return strings.ToLower(b.String())
}

// normalizeTrackingOrderRef returns the canonical lowercase uuid for an LC-… order number or a plain
// uuid; ok=false means the cell is neither (invalid_order_ref).
func normalizeTrackingOrderRef(s string) (string, bool) {
	v := strings.ToLower(strings.TrimSpace(s))
	if m := orderLC.FindStringSubmatch(v); m != nil {
		h := m[1]
		return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32], true
	}
	if command.ValidID(v) {
		return v, true
	}
	return "", false
}

// trackingCSVRow is one data row after parsing; the cells are unguarded and trimmed. code is set only
// for a structural defect (an RFC 4180 field-count mismatch) and is empty otherwise.
type trackingCSVRow struct {
	n              int
	orderRef       string
	carrier        string
	trackingNumber string
	carrierName    string
	trackingURL    string
	code           string
}

// parseTrackingCSV reads the uploaded CSV (UTF-8, optional BOM, comma separated, RFC 4180 quotes,
// unknown columns ignored, empty rows skipped) and returns the data rows plus a file-level code.
// File-level codes: encoding_not_utf8, too_many_rows, required (missing required column / empty file),
// invalid_request (broken header, duplicate known column, malformed record). An empty code means the
// file parsed and the caller must still validate every cell.
func parseTrackingCSV(data []byte) ([]trackingCSVRow, string) {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if !utf8.Valid(data) {
		return nil, "encoding_not_utf8"
	}
	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, "required"
		}
		return nil, "invalid_request"
	}
	cols := make([]string, len(header))
	seen := map[string]bool{}
	for i, h := range header {
		key, known := trackingImportColumns[foldTrackingHeader(h)]
		if !known {
			cols[i] = "" // unknown column ignored, so an unshipped.csv with two extra columns is accepted
			continue
		}
		if seen[key] {
			return nil, "invalid_request" // a duplicate known column is ambiguous, never a silent pick
		}
		seen[key] = true
		cols[i] = key
	}
	for _, required := range []string{tcolOrderNumber, tcolCarrier, tcolTrackingNumber} {
		if !seen[required] {
			return nil, "required"
		}
	}
	records, err := reader.ReadAll()
	if err != nil {
		return nil, "invalid_request"
	}
	rows := make([]trackingCSVRow, 0, len(records))
	for _, rec := range records {
		if blankTrackingRow(rec) {
			continue // empty rows are skipped and not counted
		}
		if len(rows) >= MaxTrackingImportRows {
			return nil, "too_many_rows"
		}
		row := trackingCSVRow{n: len(rows) + 1}
		if len(rec) != len(cols) {
			row.code = "invalid_request" // RFC 4180 field-count mismatch: refuse the row, never index out of range
			rows = append(rows, row)
			continue
		}
		for i, col := range cols {
			cell := unguardCell(strings.TrimSpace(rec[i]))
			switch col {
			case tcolOrderNumber:
				row.orderRef = cell
			case tcolCarrier:
				row.carrier = cell
			case tcolTrackingNumber:
				row.trackingNumber = cell
			case tcolCarrierName:
				row.carrierName = cell
			case tcolTrackingURL:
				row.trackingURL = cell
			}
		}
		rows = append(rows, row)
	}
	return rows, ""
}

func blankTrackingRow(rec []string) bool {
	for _, c := range rec {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}
