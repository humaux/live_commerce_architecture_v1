// csvfile.go is the pure half of the product CSV (storefront-v2 section G2): the column set, the spreadsheet-safe cell rules, the
// decimal price conversion, and the strict parser that turns an uploaded file into rows plus every row error. No database, no
// clock, no randomness, so csvfile_test.go pins the format.
//
// Non-goals: it decides nothing about the store (whether a slug is taken, a warehouse exists or a price is allowed to change is
// csvimport.go through internal/catalog), and it never reads a file the caller did not already bound to 2 MiB.

package merchanttools

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"livecommerce/internal/command"
)

// Import limits (contract G2). The HTTP layer enforces MaxCSVBytes before reading; MaxCSVRows is enforced here. MaxCSVRows was measured, not
// chosen: the whole file is ONE transaction and every catalog / inventory command takes a transaction advisory lock, so 5,000 rows exhausted
// PostgreSQL's lock table ("out of shared memory") and cost 11-18 ms per product+variant+stock row on an idle machine (a loaded one measured 4x
// that: 2,000 rows took 55 s of the 60 s budget). 1,500 rows is ~14-26 s idle. See maxWorkUnits in csvimport.go for the second,
// work-based cap.
const (
	MaxCSVBytes     = 2 << 20
	MaxCSVRows      = 1500
	MaxExportRows   = 50000
	maxRowErrors    = 200
	stockPrefix     = "stock:"
	ignoredPrefix   = "x_"
	maxWarehouseCol = 100
)

// Row error codes (contract G2).
const (
	codeRequired        = "required"
	codeTooLong         = "too_long"
	codeInvalidValue    = "invalid_value"
	codeDuplicateSKU    = "duplicate_sku"
	codeConflictField   = "conflict_product_field"
	codeSlugTaken       = "slug_taken"
	codeSKUOtherProduct = "sku_in_other_product"
	codeSKUArchived     = "sku_archived"
	codeOptionMismatch  = "option_mismatch"
	codeUnknownWh       = "unknown_warehouse"
	codeUnknownColl     = "unknown_collection"
	codeLimit           = "limit"
	codeConflict        = "conflict"
)

// RowError is one finding; Row is the 1-based data row (the header is row 0, a file-level finding is row 0 with Column "").
type RowError struct {
	Row    int    `json:"row"`
	Column string `json:"column"`
	Code   string `json:"code"`
}

var (
	slugRx = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	skuRx  = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)
	// fixedColumns are the export order before the stock columns; collections and image_count follow them.
	fixedColumns = []string{"handle", "title", "description", "status", "option1_name", "option1_value", "option2_name", "option2_value",
		"option3_name", "option3_value", "sku", "price", "compare_at_price"}
	tailColumns = []string{"collections", "image_count"}
)

// CurrencyExponent is the decimal places of a currency's major unit: JPY, KRW and VND have none, every other store currency has two.
func CurrencyExponent(code string) int {
	switch code {
	case "JPY", "KRW", "VND":
		return 0
	}
	return 2
}

// parseMajor reads a decimal major-unit price ("12", "12.5", "12.50") into minor units; more fractional digits than the exponent, a
// sign, an exponent form or a thousands separator is refused (a spreadsheet must not silently round a price).
func parseMajor(s string, exp int) (int64, bool) {
	whole, frac, hasFrac := strings.Cut(s, ".")
	if whole == "" || len(whole) > 12 || (hasFrac && (frac == "" || len(frac) > exp)) {
		return 0, false
	}
	for _, part := range []string{whole, frac} {
		for _, c := range part {
			if c < '0' || c > '9' {
				return 0, false
			}
		}
	}
	digits := whole + frac + strings.Repeat("0", exp-len(frac))
	v, err := strconv.ParseInt(digits, 10, 64)
	if err != nil || v > command.MaxMoney {
		return 0, false
	}
	return v, true
}

// formatMajor is the inverse of parseMajor for a non-negative minor amount.
func formatMajor(minor int64, exp int) string {
	if exp == 0 {
		return strconv.FormatInt(minor, 10)
	}
	text := strconv.FormatInt(minor, 10)
	if len(text) <= exp {
		text = strings.Repeat("0", exp-len(text)+1) + text
	}
	return text[:len(text)-exp] + "." + text[len(text)-exp:]
}

// guardCell protects a text cell from spreadsheet formula injection: a cell starting with = + - @ tab or CR gets a leading
// apostrophe, which Excel and Sheets display as plain text; unguardCell removes exactly that one apostrophe on import.
func guardCell(s string) string {
	if needsGuard(s) {
		return "'" + s
	}
	return s
}

// needsGuard: the cell starts with a formula trigger, or with an apostrophe in front of one (so a literal "'=x" survives the round trip).
func needsGuard(s string) bool {
	if s == "" {
		return false
	}
	if s[0] == '\'' {
		return len(s) > 1 && strings.ContainsRune("=+-@\t\r", rune(s[1]))
	}
	return strings.ContainsRune("=+-@\t\r", rune(s[0]))
}

func unguardCell(s string) string {
	if len(s) >= 2 && s[0] == '\'' && needsGuard(s[1:]) {
		return s[1:]
	}
	return s
}

// Row is one data row of the file after parsing; a nil Price / Compare is an empty cell.
type Row struct {
	N           int
	Handle      string
	Title       string
	Description string
	Status      string
	OptName     [3]string
	OptValue    [3]string
	SKU         string
	Price       *int64
	Compare     *int64
	Stock       map[string]int64 // header warehouse name -> target on-hand; only non-empty cells
	Collections []string
}

// File is a parsed upload. Warehouses are the stock: header names in column order; Errors may be non-empty with Rows partly filled.
type File struct {
	Rows       []Row
	Warehouses []string
	Errors     []RowError
	// HasCompare: the file has a compare_at_price column. Without it a matched variant keeps its compare-at price; with it the cell is
	// authoritative (an empty cell clears it).
	HasCompare bool
}

func addErr(errs *[]RowError, row int, column, code string) {
	*errs = append(*errs, RowError{Row: row, Column: column, Code: code})
}

// ParseFile reads the uploaded CSV (UTF-8, optional BOM, comma separated, RFC 4180 quotes). exp is the store currency exponent. It
// never returns a Go error: every defect is a RowError, so the preview can list all of them.
func ParseFile(data []byte, exp int) File {
	var f File
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if !utf8.Valid(data) {
		addErr(&f.Errors, 0, "", codeInvalidValue)
		return f
	}
	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		code := codeInvalidValue
		if errors.Is(err, io.EOF) {
			code = codeRequired
		}
		addErr(&f.Errors, 0, "", code)
		return f
	}
	cols := make([]string, len(header))
	stockAt := map[int]string{}
	seen := map[string]bool{}
	known := map[string]bool{}
	for _, c := range append(append([]string{}, fixedColumns...), tailColumns...) {
		known[c] = true
	}
	for i, h := range header {
		name := strings.TrimSpace(h)
		switch {
		case strings.HasPrefix(strings.ToLower(name), stockPrefix):
			wh := strings.TrimSpace(name[len(stockPrefix):])
			if wh == "" || len(f.Warehouses) >= maxWarehouseCol || seen["stock:"+wh] {
				addErr(&f.Errors, 0, name, codeInvalidValue)
			}
			seen["stock:"+wh] = true
			stockAt[i] = wh
			f.Warehouses = append(f.Warehouses, wh)
			cols[i] = "stock"
			continue
		case strings.HasPrefix(strings.ToLower(name), ignoredPrefix):
			cols[i] = ""
			continue
		}
		name = strings.ToLower(name)
		if !known[name] || seen[name] {
			addErr(&f.Errors, 0, name, codeInvalidValue)
		}
		seen[name] = true
		cols[i] = name
	}
	for _, required := range []string{"handle", "title"} {
		if !seen[required] {
			addErr(&f.Errors, 0, required, codeRequired)
		}
	}
	f.HasCompare = seen["compare_at_price"]
	if len(f.Errors) > 0 {
		return f // a broken header makes every row meaningless
	}
	records, err := reader.ReadAll()
	if err != nil {
		addErr(&f.Errors, 0, "", codeInvalidValue)
		return f
	}
	if len(records) > MaxCSVRows {
		addErr(&f.Errors, 0, "", codeLimit)
		return f
	}
	for n, rec := range records {
		row, rowErrs := parseRow(n+1, rec, cols, stockAt, exp)
		f.Errors = append(f.Errors, rowErrs...)
		f.Rows = append(f.Rows, row)
	}
	return f
}

func parseRow(n int, rec, cols []string, stockAt map[int]string, exp int) (Row, []RowError) {
	row := Row{N: n, Stock: map[string]int64{}}
	var errs []RowError
	if len(rec) != len(cols) {
		addErr(&errs, n, "", codeInvalidValue)
		return row, errs
	}
	cell := func(i int) string { return unguardCell(strings.TrimSpace(rec[i])) }
	bad := func(col, code string) { addErr(&errs, n, col, code) }
	text := func(col, v string, max int) string {
		if utf8.RuneCountInString(v) > max {
			bad(col, codeTooLong)
		}
		if strings.ContainsAny(v, "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x0b\x0c\x0e\x0f") {
			bad(col, codeInvalidValue)
		}
		return v
	}
	for i, col := range cols {
		v := cell(i)
		switch col {
		case "":
		case "handle":
			row.Handle = v
			if v == "" {
				bad(col, codeRequired)
			} else if len(v) > 80 || !slugRx.MatchString(v) {
				bad(col, codeInvalidValue)
			}
		case "title":
			row.Title = text(col, v, 120)
		case "description":
			row.Description = text(col, v, 8000)
		case "status":
			row.Status = strings.ToLower(v)
			if row.Status != "" && row.Status != "draft" && row.Status != "active" && row.Status != "archived" {
				bad(col, codeInvalidValue)
			}
		case "option1_name", "option2_name", "option3_name":
			row.OptName[col[6]-'1'] = text(col, v, 30)
		case "option1_value", "option2_value", "option3_value":
			row.OptValue[col[6]-'1'] = text(col, v, 40)
		case "sku":
			row.SKU = v
			if v != "" && !skuRx.MatchString(v) {
				bad(col, codeInvalidValue)
			}
		case "price", "compare_at_price":
			if v == "" {
				break
			}
			minor, ok := parseMajor(v, exp)
			if !ok {
				bad(col, codeInvalidValue)
			} else if col == "price" {
				row.Price = &minor
			} else {
				row.Compare = &minor
			}
		case "collections":
			for _, slug := range strings.Split(v, "|") {
				if slug = strings.TrimSpace(slug); slug == "" {
					continue
				} else if len(slug) > 80 || !slugRx.MatchString(slug) {
					bad(col, codeInvalidValue)
				} else {
					row.Collections = append(row.Collections, slug)
				}
			}
		case "image_count": // informational, ignored on import
		case "stock":
			if v == "" {
				break
			}
			q, err := strconv.ParseInt(v, 10, 64)
			if err != nil || q < 0 || q > command.MaxQuantity {
				bad("stock:"+stockAt[i], codeInvalidValue)
			} else {
				row.Stock[stockAt[i]] = q
			}
		}
	}
	if row.SKU == "" && (row.Price != nil || row.Compare != nil || len(row.Stock) > 0 || row.OptValue != [3]string{}) {
		bad("sku", codeRequired) // variant data without a SKU code has nowhere to go
	}
	if row.Price != nil && row.Compare != nil && *row.Compare <= *row.Price {
		bad("compare_at_price", codeInvalidValue)
	} else if row.Price == nil && row.Compare != nil {
		bad("price", codeRequired)
	}
	return row, errs
}

// Group is every row of one handle, in file order; the first row's product-level cells are authoritative.
type Group struct {
	Handle      string
	First       int // data row of the first row, where product-level findings are reported
	Title       string
	Description string
	Status      string
	OptName     [3]string
	Collections []string
	Rows        []Row // only rows with a SKU code
}

// GroupRows folds rows by handle and reports the cross-row defects: a product-level cell that disagrees with the first row, a duplicate
// SKU code anywhere in the file, an option axis without a name (or a value without an axis) and a variant row missing an axis value.
func GroupRows(rows []Row) ([]Group, []RowError) {
	var groups []Group
	var errs []RowError
	at := map[string]int{}
	skus := map[string]bool{}
	for _, r := range rows {
		if r.Handle == "" {
			continue // already reported by ParseFile
		}
		i, ok := at[r.Handle]
		if !ok {
			i = len(groups)
			at[r.Handle] = i
			groups = append(groups, Group{Handle: r.Handle, First: r.N, Title: r.Title, Description: r.Description, Status: r.Status,
				OptName: r.OptName, Collections: r.Collections})
			if r.Title == "" {
				addErr(&errs, r.N, "title", codeRequired)
			}
		} else {
			g := &groups[i]
			differ := func(col string, a, b string) {
				if b != "" && b != a {
					addErr(&errs, r.N, col, codeConflictField)
				}
			}
			differ("title", g.Title, r.Title)
			differ("description", g.Description, r.Description)
			differ("status", g.Status, r.Status)
			for k := 0; k < 3; k++ {
				differ("option"+strconv.Itoa(k+1)+"_name", g.OptName[k], r.OptName[k])
			}
			if len(r.Collections) > 0 && strings.Join(r.Collections, "|") != strings.Join(g.Collections, "|") {
				addErr(&errs, r.N, "collections", codeConflictField)
			}
		}
		if r.SKU == "" {
			continue
		}
		if skus[r.SKU] {
			addErr(&errs, r.N, "sku", codeDuplicateSKU)
		}
		skus[r.SKU] = true
		groups[i].Rows = append(groups[i].Rows, r)
	}
	for _, g := range groups {
		gap := false
		for k := 0; k < 3; k++ {
			if g.OptName[k] == "" {
				gap = true
				continue
			}
			if gap {
				addErr(&errs, g.First, "option"+strconv.Itoa(k+1)+"_name", codeInvalidValue)
			}
		}
		for _, r := range g.Rows {
			for k := 0; k < 3; k++ {
				switch {
				case g.OptName[k] != "" && r.OptValue[k] == "":
					addErr(&errs, r.N, "option"+strconv.Itoa(k+1)+"_value", codeRequired)
				case g.OptName[k] == "" && r.OptValue[k] != "":
					addErr(&errs, r.N, "option"+strconv.Itoa(k+1)+"_value", codeInvalidValue)
				}
			}
		}
	}
	return groups, errs
}
