// Purpose: the pure half of the customer CSV import (migration-import-v1 section 3): header folding and the SHOPLINE column aliases,
//   mapping resolution (merchant mapping overrides auto-detection), the strict RFC 4180 reader, and the per-cell rules (Taiwan mobile
//   to E.164, name, email, source-system id). No database, no clock, no randomness.
// Depends on: encoding/csv, unicode, golang.org/x/text/unicode/norm (NFC), internal/csvguard (unguard test).
// Used by: customers.go (CustomersPreview / CustomersCommit); pinned by the DB-free tests in customers_test.go.
// Invariants: imported consent is never read: a consent column is only DETECTED so the preview can say it was ignored; the SHOPLINE
//   notes column is ignored too (CI-OPEN-2); the file content never appears in a returned error.
// Status: MOCK (header aliases are best-guess SHOPLINE/Chinese names until the owner supplies a de-identified header row, CI-OPEN-1).

package migrationimport

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"livecommerce/internal/csvguard"
)

// Canonical customer fields a mapping may name. "consent" is mappable only so a non-default consent header can be flagged ignored.
const (
	fieldExternalID = "external_id"
	fieldName       = "name"
	fieldPhone      = "phone"
	fieldEmail      = "email"
	fieldConsent    = "consent"
)

var mappingFields = []string{fieldExternalID, fieldName, fieldPhone, fieldEmail, fieldConsent}

// Row outcomes (also the values stored in batch results and written to results.csv).
const (
	outcomeCreated = "created"
	outcomeUpdated = "updated"
	outcomeFailed  = "failed"
)

// headerAliases lists, per field, the folded header names auto-detected (see foldHeader). Best-guess SHOPLINE / Taiwan spreadsheet
// names: the owner's real export header decides the final list (CI-OPEN-1); until then a merchant can always override per field.
var headerAliases = map[string][]string{
	fieldExternalID: {"customer_id", "id", "member_id", "customer_number", "顧客編號", "顾客编号", "顧客id", "顾客id", "客戶編號", "客户编号", "會員編號", "会员编号", "顧客代碼"},
	fieldName:       {"name", "full_name", "customer_name", "姓名", "顧客姓名", "顾客姓名", "顧客名稱", "顾客名称", "客戶名稱", "客户名称", "會員姓名", "名稱", "名字"},
	fieldPhone:      {"phone", "mobile", "phone_number", "mobile_phone", "手機", "手机", "手機號碼", "手机号码", "行動電話", "電話", "电话", "聯絡電話", "联系电话"},
	fieldEmail:      {"email", "e_mail", "email_address", "電子郵件", "电子邮件", "電子信箱", "电子邮箱", "電郵", "信箱"},
	fieldConsent:    {"accepts_marketing", "marketing_consent", "marketing", "subscribed", "consent", "同意行銷", "同意行销", "同意行銷訊息", "接受行銷", "接受行销", "同意接收行銷", "訂閱電子報", "订阅电子报"},
}

var (
	mobileRe = regexp.MustCompile(`^9[0-9]{8}$`)
	emailRe  = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
)

// foldHeader makes one header cell comparable: trims, maps full-width ASCII (U+FF01..U+FF5E) and the ideographic space to their
// half-width forms, lower-cases, and turns spaces and hyphens into underscores ("Customer ID" and "customer-id" -> "customer_id").
func foldHeader(s string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(s) {
		switch {
		case r == '　':
			b.WriteRune('_')
		case r >= '！' && r <= '～':
			b.WriteRune(r - 0xfee0)
		default:
			b.WriteRune(r)
		}
	}
	return strings.NewReplacer(" ", "_", "-", "_").Replace(strings.ToLower(b.String()))
}

// unguardCell removes the one apostrophe a formula guard added (csvguard.Cell) so an exported-then-reimported cell round-trips.
func unguardCell(s string) string {
	if len(s) >= 2 && s[0] == '\'' && csvguard.Cell(s[1:]) != s[1:] {
		return s[1:]
	}
	return s
}

// customerRow is one data row after parsing and cell validation. outcome is "failed" with code on any defect, "" while still pending.
type customerRow struct {
	n              int
	externalID     string
	name           string
	phone          string // E.164, "" when the cell was empty
	email          string // lower-case, "" when the cell was empty
	consentIgnored bool
	outcome, code  string
}

// parsedCustomers is a parsed upload: the file's headers, the mapping actually used (field -> header as written in the file) and rows.
type parsedCustomers struct {
	headers []string
	mapping map[string]string
	rows    []customerRow
}

// parseCustomerCSV reads the uploaded CSV (UTF-8, optional BOM, comma separated, RFC 4180 quotes, unknown columns ignored, empty rows
// skipped and not counted) and returns the rows plus a file-level code: encoding_not_utf8, too_many_rows, required (empty file or a
// required column unmapped) or invalid_request (broken header / mapping, malformed record). requested is the merchant's mapping
// (field -> header; "" unmaps a field) and may be nil. An empty code means the file parsed; the rows already carry per-cell verdicts.
func parseCustomerCSV(data []byte, requested map[string]string) (parsedCustomers, string) {
	out := parsedCustomers{}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if !utf8.Valid(data) {
		return out, "encoding_not_utf8"
	}
	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return out, "required"
		}
		return out, "invalid_request"
	}
	out.headers = make([]string, len(header))
	for i, h := range header {
		out.headers[i] = strings.TrimSpace(h)
	}
	cols, mapping, code := resolveMapping(out.headers, requested)
	if code != "" {
		return out, code
	}
	out.mapping = mapping
	records, err := reader.ReadAll()
	if err != nil {
		return out, "invalid_request"
	}
	for _, rec := range records {
		if blankRecord(rec) {
			continue
		}
		if len(out.rows) >= MaxRows {
			return out, "too_many_rows"
		}
		row := customerRow{n: len(out.rows) + 1}
		if len(rec) != len(out.headers) {
			row.outcome, row.code = outcomeFailed, "invalid_request" // field-count mismatch: refuse the row, never index out of range
			out.rows = append(out.rows, row)
			continue
		}
		cell := func(field string) string {
			if i, ok := cols[field]; ok {
				return unguardCell(strings.TrimSpace(rec[i]))
			}
			return ""
		}
		normalizeCustomerRow(&row, cell(fieldExternalID), cell(fieldName), cell(fieldPhone), cell(fieldEmail))
		row.consentIgnored = cell(fieldConsent) != ""
		out.rows = append(out.rows, row)
	}
	markDuplicates(out.rows)
	return out, ""
}

// resolveMapping decides which column feeds which field: an explicit merchant entry wins (its header must exist, exactly once after
// folding; "" unmaps the field), otherwise the first not-yet-claimed column whose folded header is a known alias. external_id and name
// are required. It returns the column index per field and the mapping as header text (stored on the batch, shown in the preview).
func resolveMapping(headers []string, requested map[string]string) (map[string]int, map[string]string, string) {
	byFold := map[string][]int{}
	for i, h := range headers {
		byFold[foldHeader(h)] = append(byFold[foldHeader(h)], i)
	}
	for field := range requested {
		known := false
		for _, f := range mappingFields {
			known = known || f == field
		}
		if !known {
			return nil, nil, "invalid_request"
		}
	}
	cols, mapping, claimed := map[string]int{}, map[string]string{}, map[int]bool{}
	for _, field := range mappingFields {
		if h, ok := requested[field]; ok {
			if h == "" {
				continue
			}
			idx := byFold[foldHeader(h)]
			if len(idx) != 1 || claimed[idx[0]] {
				return nil, nil, "invalid_request" // unknown header, ambiguous header, or one column for two fields
			}
			cols[field], claimed[idx[0]], mapping[field] = idx[0], true, headers[idx[0]]
			continue
		}
		for i, h := range headers {
			if claimed[i] || !containsString(headerAliases[field], foldHeader(h)) {
				continue
			}
			cols[field], claimed[i], mapping[field] = i, true, h
			break
		}
	}
	for _, required := range []string{fieldExternalID, fieldName} {
		if _, ok := cols[required]; !ok {
			return nil, nil, "required"
		}
	}
	return cols, mapping, ""
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func blankRecord(rec []string) bool {
	for _, c := range rec {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

// normalizeCustomerRow validates one row's cells and stores the canonical values; the first defect wins (outcome failed + code).
func normalizeCustomerRow(r *customerRow, externalID, name, phone, email string) {
	r.externalID = truncateRunes(externalID, 64) // a refused over-long cell is echoed truncated, never in full
	switch {
	case externalID == "":
		r.outcome, r.code = outcomeFailed, "required"
		return
	case utf8.RuneCountInString(externalID) > 64 || hasControl(externalID) || looksLikeContact(externalID):
		r.outcome, r.code = outcomeFailed, "invalid_external_id"
		return
	}
	name = norm.NFC.String(name)
	switch {
	case name == "":
		r.outcome, r.code = outcomeFailed, "required"
		return
	case utf8.RuneCountInString(name) > 80:
		r.outcome, r.code = outcomeFailed, "name_too_long"
		return
	case hasControl(name):
		r.outcome, r.code = outcomeFailed, "invalid_name"
		return
	}
	r.name = name
	if phone != "" {
		e164, ok := normalizeTWMobile(phone)
		if !ok {
			r.outcome, r.code = outcomeFailed, "invalid_phone"
			return
		}
		r.phone = e164
	}
	if email != "" {
		email = strings.ToLower(email)
		if len(email) > 254 || hasControl(email) || strings.IndexFunc(email, unicode.IsSpace) >= 0 || !emailRe.MatchString(email) {
			r.outcome, r.code = outcomeFailed, "invalid_email"
			return
		}
		r.email = email
	}
}

// looksLikeContact reports an external id that is really an email or a Taiwan mobile number: a mis-mapped column would otherwise copy
// that PII into external_ids, batch results and results.csv. A numeric source id that is also a valid mobile number must be prefixed.
func looksLikeContact(id string) bool {
	if strings.Contains(id, "@") {
		return true
	}
	_, phone := normalizeTWMobile(id)
	return phone
}

// markDuplicates refuses every still-valid row whose external_id appears on two or more rows (never guess which one is right). A row
// already refused keeps its own more specific code but still counts as a holder of the id.
func markDuplicates(rows []customerRow) {
	count := map[string]int{}
	for _, r := range rows {
		if r.externalID != "" {
			count[r.externalID]++
		}
	}
	for i := range rows {
		if rows[i].outcome == "" && count[rows[i].externalID] > 1 {
			rows[i].outcome, rows[i].code = outcomeFailed, "duplicate_external_id"
		}
	}
}

// normalizeTWMobile turns a Taiwan mobile number written as 0912-345-678, 0912345678, 912345678 (Excel dropped the zero),
// +886 912 345 678, 886912345678, 00886912345678 or +886 0912... into +886912345678. Landlines and anything else are refused.
func normalizeTWMobile(s string) (string, bool) {
	var b strings.Builder
	for i, r := range strings.TrimSpace(s) {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '+' && i == 0:
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '(' || r == ')' || r == '.':
		default:
			return "", false
		}
	}
	d := b.String()
	switch {
	case strings.HasPrefix(d, "+886"):
		d = d[4:]
	case strings.HasPrefix(d, "00886"):
		d = d[5:]
	case strings.HasPrefix(d, "886"):
		d = d[3:]
	case strings.HasPrefix(d, "+"):
		return "", false
	}
	d = strings.TrimPrefix(d, "0")
	if !mobileRe.MatchString(d) {
		return "", false
	}
	return "+886" + d, true
}

// hasControl reports control or invisible format characters (a tab or newline inside a quoted cell, a zero-width space): they would
// let two ids or names look equal, and a newline would break the one-line projections.
func hasControl(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return true
		}
	}
	return false
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}
