// Purpose: the pure half of the historical-order CSV import (W5-03B, migration-import-v1 section 7): the SHOPLINE order column aliases,
//   the per-line cell rules (amount, date, status, quantity, city) and the aggregation of one-line-per-item files into one unit per
//   order. No database, no clock, no randomness.
// Depends on: customers_csv.go (resolveMapping, foldHeader, unguardCell, hasControl, looksLikeContact, blankRecord, truncateRunes,
//   outcome constants), encoding/csv, golang.org/x/text/unicode/norm (NFC).
// Used by: orders.go (OrdersPreview / OrdersCommit); pinned by orders_test.go (DB-free).
// Invariants: only the mapped columns are ever read, so an address, phone, email, payment or note column in the file is never stored;
//   the city cell is refused when it contains a digit or is over 20 characters (a mis-mapped full address cannot be archived); the file
//   content never appears in a returned error.
// Status: MOCK (header aliases are best-guess SHOPLINE / Taiwan spreadsheet names until the owner supplies a de-identified header row).

package migrationimport

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Canonical order fields a mapping may name. Every other column of the file is ignored and never read.
const (
	fieldOrderID    = "order_id"
	fieldCustomerID = "customer_id"
	fieldOrderedAt  = "ordered_at"
	fieldStatus     = "status"
	fieldTotal      = "total"
	fieldItemName   = "item_name"
	fieldItemQty    = "item_qty"
	fieldCity       = "city"
)

var orderFields = []string{fieldOrderID, fieldCustomerID, fieldOrderedAt, fieldStatus, fieldTotal, fieldItemName, fieldItemQty, fieldCity}
var orderRequired = []string{fieldOrderID, fieldCustomerID, fieldOrderedAt, fieldStatus, fieldTotal}

// orderAliases lists, per field, the folded header names auto-detected (see foldHeader). Best-guess names; a merchant can override.
var orderAliases = map[string][]string{
	fieldOrderID:    {"order_id", "order_number", "order_no", "訂單編號", "订单编号", "訂單號碼", "订单号码", "訂單號", "订单号"},
	fieldCustomerID: {"customer_id", "member_id", "customer_number", "顧客編號", "顾客编号", "顧客id", "顾客id", "客戶編號", "客户编号", "會員編號", "会员编号", "顧客代碼"},
	fieldOrderedAt:  {"ordered_at", "order_date", "order_time", "created_at", "訂單日期", "订单日期", "訂單時間", "订单时间", "下單時間", "下单时间", "下單日期"},
	fieldStatus:     {"status", "order_status", "訂單狀態", "订单状态", "狀態", "状态"},
	fieldTotal:      {"total", "order_total", "total_amount", "grand_total", "訂單總金額", "订单总金额", "訂單金額", "订单金额", "總金額", "总金额", "訂單總額", "合計"},
	fieldItemName:   {"item_name", "product_name", "product", "商品名稱", "商品名称", "商品", "品項", "品名"},
	fieldItemQty:    {"item_qty", "quantity", "qty", "數量", "数量", "商品數量", "商品数量"},
	fieldCity:       {"city", "shipping_city", "delivery_city", "城市", "縣市", "县市", "配送城市", "收件城市", "收件縣市"},
}

// Limits of the archive row (the table CHECKs are the backstop).
const (
	maxStatusRunes   = 40
	maxCityRunes     = 20
	maxItemsRunes    = 500
	maxOrderIDRunes  = 64
	maxItemQty       = 9999
	maxTotalDollars  = 10_000_000_000 // total_minor = dollars x 100 stays below the 10^12 CHECK
	orderMinYear     = 2000
	orderMaxYear     = 2100
	taipeiOffsetSecs = 8 * 3600
)

// taipei is a fixed UTC+8 zone: Taiwan has no daylight saving, and a container may lack tzdata.
var taipei = time.FixedZone("Asia/Taipei", taipeiOffsetSecs)

// orderLayouts are the date spellings accepted; a layout without a zone is read as Asia/Taipei.
var orderLayouts = []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02 15:04", "2006/01/02 15:04:05", "2006/01/02 15:04", "2006-01-02", "2006/01/02",
	time.RFC3339, "2006-01-02T15:04:05Z0700", "2006-01-02 15:04:05 -0700"}

// orderUnit is one archive row after aggregation: all CSV lines of one order, or a single unusable line. n is the data-row number of
// the first line (the row number shown in previews and results). outcome is "failed" with code on any defect, "" while pending.
type orderUnit struct {
	n             int
	orderID       string
	customerID    string
	orderedAt     time.Time
	status        string
	totalMinor    int64
	items         string
	city          string
	outcome, code string
}

// parsedOrders is a parsed upload: headers, the mapping actually used (field -> header as written) and one unit per order.
type parsedOrders struct {
	headers []string
	mapping map[string]string
	units   []orderUnit
}

// orderLine is one data line before grouping (cells already trimmed and unguarded).
type orderLine struct {
	n                                       int
	orderID, customerID, at, status, total string
	item, qty, city                         string
}

// parseOrderCSV reads the uploaded CSV (UTF-8, optional BOM, comma separated, RFC 4180, blank rows skipped and not counted, unknown
// columns ignored) and returns the aggregated units plus a file-level code: encoding_not_utf8, too_many_rows (more than MaxRows data
// lines), required (empty file or a required column unmapped) or invalid_request (broken header / mapping / record).
func parseOrderCSV(data []byte, requested map[string]string) (parsedOrders, string) {
	out := parsedOrders{}
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
	cols, mapping, code := resolveMapping(out.headers, requested, orderFields, orderAliases, orderRequired)
	if code != "" {
		return out, code
	}
	out.mapping = mapping
	records, err := reader.ReadAll()
	if err != nil {
		return out, "invalid_request"
	}
	var lines []orderLine
	var broken []int // data-row numbers of lines whose field count does not match the header
	rowNo := 0
	for _, rec := range records {
		if blankRecord(rec) {
			continue
		}
		if rowNo >= MaxRows {
			return out, "too_many_rows"
		}
		rowNo++
		if len(rec) != len(out.headers) {
			broken = append(broken, rowNo)
			lines = append(lines, orderLine{n: rowNo})
			continue
		}
		cell := func(field string) string {
			if i, ok := cols[field]; ok {
				return unguardCell(strings.TrimSpace(rec[i]))
			}
			return ""
		}
		lines = append(lines, orderLine{n: rowNo, orderID: cell(fieldOrderID), customerID: cell(fieldCustomerID), at: cell(fieldOrderedAt),
			status: cell(fieldStatus), total: cell(fieldTotal), item: cell(fieldItemName), qty: cell(fieldItemQty), city: cell(fieldCity)})
	}
	out.units = groupOrders(lines, broken)
	return out, ""
}

// groupOrders turns lines into units in order of first appearance. A line that cannot be attributed to an order (field-count mismatch,
// no or invalid order id) is its own failed unit; all other lines of one order id merge, and any disagreement between non-empty cells
// of one order fails the whole order (inconsistent_order): a total, customer, date, status or city is never guessed.
func groupOrders(lines []orderLine, broken []int) []orderUnit {
	isBroken := map[int]bool{}
	for _, n := range broken {
		isBroken[n] = true
	}
	var units []orderUnit
	index := map[string]int{} // order id -> units index
	groups := map[int][]orderLine{}
	for _, l := range lines {
		switch {
		case isBroken[l.n]:
			units = append(units, orderUnit{n: l.n, outcome: outcomeFailed, code: "invalid_request"})
		case l.orderID == "":
			units = append(units, orderUnit{n: l.n, outcome: outcomeFailed, code: "required"})
		case utf8.RuneCountInString(l.orderID) > maxOrderIDRunes || hasControl(l.orderID) || looksLikeContact(l.orderID):
			units = append(units, orderUnit{n: l.n, outcome: outcomeFailed, code: "invalid_order_id"})
		default:
			i, seen := index[l.orderID]
			if !seen {
				i = len(units)
				index[l.orderID] = i
				units = append(units, orderUnit{n: l.n, orderID: l.orderID})
			}
			groups[i] = append(groups[i], l)
		}
	}
	for i, ls := range groups {
		buildUnit(&units[i], ls)
	}
	return units
}

// buildUnit merges the lines of one order into u, or fails it with the first defect found.
func buildUnit(u *orderUnit, ls []orderLine) {
	fail := func(code string) { u.outcome, u.code = outcomeFailed, code }
	var customer, status, city string
	var at time.Time
	var total int64
	haveAt, haveTotal := false, false
	var items []string
	for _, l := range ls {
		if l.customerID != "" {
			if utf8.RuneCountInString(l.customerID) > 64 || hasControl(l.customerID) || looksLikeContact(l.customerID) {
				fail("invalid_external_id")
				return
			}
			if customer != "" && customer != l.customerID {
				fail("inconsistent_order")
				return
			}
			customer = l.customerID
		}
		if l.at != "" {
			t, ok := parseOrderTime(l.at)
			if !ok {
				fail("invalid_date")
				return
			}
			if haveAt && !at.Equal(t) {
				fail("inconsistent_order")
				return
			}
			at, haveAt = t, true
		}
		if l.status != "" {
			s := norm.NFC.String(l.status)
			if utf8.RuneCountInString(s) > maxStatusRunes || hasControl(s) {
				fail("invalid_status")
				return
			}
			if status != "" && status != s {
				fail("inconsistent_order")
				return
			}
			status = s
		}
		if l.total != "" {
			m, ok := parseTWDMinor(l.total)
			if !ok {
				fail("invalid_amount")
				return
			}
			if haveTotal && total != m {
				fail("inconsistent_order")
				return
			}
			total, haveTotal = m, true
		}
		if l.city != "" {
			c := norm.NFC.String(l.city)
			// Taiwanese city / county names carry no digit: a digit or a long cell is a street address in the wrong column.
			if utf8.RuneCountInString(c) > maxCityRunes || hasControl(c) || strings.IndexFunc(c, unicode.IsDigit) >= 0 {
				fail("invalid_city")
				return
			}
			if city != "" && city != c {
				fail("inconsistent_order")
				return
			}
			city = c
		}
		if name := cleanItemName(l.item); name != "" {
			qty := 1
			if l.qty != "" {
				q, err := strconv.Atoi(l.qty)
				if err != nil || q < 1 || q > maxItemQty {
					fail("invalid_quantity")
					return
				}
				qty = q
			}
			items = append(items, name+"×"+strconv.Itoa(qty))
		}
	}
	if customer == "" || !haveAt || status == "" || !haveTotal {
		fail("required")
		return
	}
	u.customerID, u.orderedAt, u.status, u.totalMinor, u.city = customer, at.UTC(), status, total, city
	u.items = truncateRunes(strings.Join(items, "、"), maxItemsRunes)
}

// cleanItemName drops control and format characters from a product name and collapses surrounding space (a name is display text only).
func cleanItemName(s string) string {
	s = norm.NFC.String(s)
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, s))
}

// parseOrderTime reads one date cell; a cell without a zone is Asia/Taipei. Years outside 2000..2100 are refused.
func parseOrderTime(s string) (time.Time, bool) {
	for _, layout := range orderLayouts {
		if t, err := time.ParseInLocation(layout, s, taipei); err == nil && t.Year() >= orderMinYear && t.Year() <= orderMaxYear {
			return t, true
		}
	}
	return time.Time{}, false
}

// parseTWDMinor turns an amount cell ("1280", "1,280", "NT$1,280", "$1280.00", "1280元") into TWD minor units (whole NT$ x 100). A
// non-zero fraction, a sign, an exponent or anything else is refused: the archive never rounds a display total.
func parseTWDMinor(s string) (int64, bool) {
	s = strings.TrimSpace(s)
	for _, prefix := range []string{"NT$", "NTD", "TWD", "$", "＄"} {
		s = strings.TrimSpace(strings.TrimPrefix(s, prefix))
	}
	s = strings.TrimSpace(strings.TrimSuffix(s, "元"))
	s = strings.ReplaceAll(s, ",", "")
	whole, frac, hasFrac := strings.Cut(s, ".")
	if whole == "" || len(whole) > 11 || (hasFrac && strings.Trim(frac, "0") != "") {
		return 0, false
	}
	for _, r := range whole {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	dollars, err := strconv.ParseInt(whole, 10, 64)
	if err != nil || dollars > maxTotalDollars {
		return 0, false
	}
	return dollars * 100, true
}
