package merchanttools

import (
	"bytes"
	"crypto/sha256"
	"regexp"
	"strings"
	"testing"

	"livecommerce/internal/catalog"
	"livecommerce/internal/reporting"
	"livecommerce/internal/storefront"
)

func homeAddressOf(region, city, postal, line1 string) storefront.HomeAddress {
	return storefront.HomeAddress{Region: region, City: city, PostalCode: postal, Line1: line1}
}

// Pure-logic checks of the CSV format, the manual-order validator and the dashboard re-cut (contract G). Database behaviour is in
// tests/foundation/merchant_tools_smoke_test.go.

func TestMajorUnitsRoundTripAndRefusals(t *testing.T) {
	for _, c := range []struct {
		in    string
		exp   int
		minor int64
		ok    bool
	}{
		{"12", 2, 1200, true}, {"12.5", 2, 1250, true}, {"12.50", 2, 1250, true}, {"0.05", 2, 5, true}, {"12.505", 2, 0, false},
		{"12.", 2, 0, false}, {".5", 2, 0, false}, {"-1", 2, 0, false}, {"1e3", 2, 0, false}, {"1,200", 2, 0, false}, {"", 2, 0, false},
		{"500", 0, 500, true}, {"500.0", 0, 0, false}, {"1000000000000", 2, 0, false}, {"9999999999999", 2, 0, false},
	} {
		got, ok := parseMajor(c.in, c.exp)
		if ok != c.ok || (ok && got != c.minor) {
			t.Errorf("parseMajor(%q,%d)=%d,%v want %d,%v", c.in, c.exp, got, ok, c.minor, c.ok)
		}
	}
	for _, minor := range []int64{0, 5, 99, 100, 1250, 123456789} {
		for _, exp := range []int{0, 2} {
			if back, ok := parseMajor(formatMajor(minor, exp), exp); !ok || back != minor {
				t.Errorf("round trip minor=%d exp=%d: %q -> %d,%v", minor, exp, formatMajor(minor, exp), back, ok)
			}
		}
	}
	if CurrencyExponent("TWD") != 2 || CurrencyExponent("JPY") != 0 {
		t.Error("currency exponent")
	}
}

func TestFormulaGuardRoundTrip(t *testing.T) {
	for _, s := range []string{"=SUM(A1)", "+1", "-5", "@x", "\tx", "plain", "", "'quoted", "'=already"} {
		if got := unguardCell(guardCell(s)); got != s {
			t.Errorf("round trip %q -> %q", s, got)
		}
	}
	if guardCell("=1+1") != "'=1+1" || guardCell("abc") != "abc" {
		t.Error("guard shape")
	}
}

const sampleHeader = "handle,title,description,status,option1_name,option1_value,sku,price,compare_at_price,stock:Main,collections,image_count\r\n"

func TestParseFileAcceptsBOMQuotesAndGuards(t *testing.T) {
	body := "\xEF\xBB\xBF" + sampleHeader +
		"tee,\"Tee, \"\"Classic\"\"\",\"line1\nline2\",active,Size,S,TEE-S,12.50,15,7,summer|sale,3\r\n" +
		"tee,,,,,M,TEE-M,12.50,,0,,3\r\n" +
		"hat,'=Hat,,,,,HAT-1,5,,,,0\r\n"
	f := ParseFile([]byte(body), 2)
	if len(f.Errors) != 0 {
		t.Fatalf("errors: %+v", f.Errors)
	}
	if len(f.Rows) != 3 || f.Rows[0].Title != `Tee, "Classic"` || f.Rows[0].Description != "line1\nline2" || *f.Rows[0].Price != 1250 || *f.Rows[0].Compare != 1500 {
		t.Fatalf("row 1: %+v", f.Rows[0])
	}
	if f.Rows[0].Stock["Main"] != 7 || f.Rows[1].Stock["Main"] != 0 || len(f.Rows[2].Stock) != 0 {
		t.Errorf("stock cells: %+v %+v %+v", f.Rows[0].Stock, f.Rows[1].Stock, f.Rows[2].Stock)
	}
	if f.Rows[2].Title != "=Hat" || strings.Join(f.Rows[0].Collections, ",") != "summer,sale" || len(f.Warehouses) != 1 || f.Warehouses[0] != "Main" {
		t.Errorf("guard/collections/warehouses: %+v %v", f.Rows[2], f.Warehouses)
	}
}

func TestParseFileListsEveryRowError(t *testing.T) {
	body := sampleHeader +
		"Bad Handle,T,,,,,,,,,,\r\n" + // 1 invalid handle
		"ok1,,,weird,,,,,,,,\r\n" + // 2 missing title is a group finding; bad status here
		"ok2,T,,,,,SKU 1,5,,,,\r\n" + // 3 invalid sku
		"ok3,T,,,,,S3,1.234,,,,\r\n" + // 4 too many decimals
		"ok4,T,,,,,S4,5,4,,,\r\n" + // 5 compare <= price
		"ok5,T,,,,,,5,,,,\r\n" + // 6 price without sku
		"ok6,T,,,,,S6,5,,-1,,\r\n" + // 7 negative stock
		"ok7,T\r\n" // 8 wrong cell count
	f := ParseFile([]byte(body), 2)
	got := map[int][]string{}
	for _, e := range f.Errors {
		got[e.Row] = append(got[e.Row], e.Column+":"+e.Code)
	}
	want := map[int]string{1: "handle:invalid_value", 2: "status:invalid_value", 3: "sku:invalid_value", 4: "price:invalid_value",
		5: "compare_at_price:invalid_value", 6: "sku:required", 7: "stock:Main:invalid_value", 8: ":invalid_value"}
	for row, w := range want {
		if !contains(got[row], w) {
			t.Errorf("row %d: want %s in %v", row, w, got[row])
		}
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func TestParseFileHeaderAndLimits(t *testing.T) {
	for name, body := range map[string]string{
		"empty":            "",
		"no title column":  "handle,sku\r\na,b\r\n",
		"unknown column":   "handle,title,colour\r\na,b,c\r\n",
		"duplicate column": "handle,title,title\r\na,b,c\r\n",
		"not utf8":         "handle,title\r\n\xff,b\r\n",
	} {
		if f := ParseFile([]byte(body), 2); len(f.Errors) == 0 || f.Errors[0].Row != 0 {
			t.Errorf("%s: want a row-0 error, got %+v", name, f.Errors)
		}
	}
	if f := ParseFile([]byte("handle,title,x_note\r\na,b,whatever\r\n"), 2); len(f.Errors) != 0 {
		t.Errorf("x_ columns are ignored: %+v", f.Errors)
	}
	var big bytes.Buffer
	big.WriteString("handle,title\r\n")
	for i := 0; i <= MaxCSVRows; i++ {
		big.WriteString("a,b\r\n")
	}
	if f := ParseFile(big.Bytes(), 2); len(f.Errors) != 1 || f.Errors[0].Code != codeLimit {
		t.Errorf("row limit: %+v", f.Errors)
	}
}

func TestGroupRowsFindsCrossRowDefects(t *testing.T) {
	body := "handle,title,status,option1_name,option1_value,option2_name,option2_value,sku,price\r\n" +
		"a,A,active,Size,S,,,A-S,5\r\n" + // 1
		"a,Other,,,M,,,A-M,5\r\n" + // 2: title conflicts; option value M fits (blank name inherits? no: name blank => value without axis? first row names it)
		"a,,,,,,,A-S,5\r\n" + // 3: duplicate sku, missing option value
		"b,B,,,,Colour,,B-1,5\r\n" // 4: option2 without option1; missing value
	f := ParseFile([]byte(body), 2)
	if len(f.Errors) != 0 {
		t.Fatalf("parse: %+v", f.Errors)
	}
	groups, errs := GroupRows(f.Rows)
	if len(groups) != 2 || len(groups[0].Rows) != 3 {
		t.Fatalf("groups: %+v", groups)
	}
	got := map[string]bool{}
	for _, e := range errs {
		got[e.Column+":"+e.Code+":"+string(rune('0'+e.Row))] = true
	}
	for _, w := range []string{"title:conflict_product_field:2", "sku:duplicate_sku:3", "option1_value:required:3", "option2_name:invalid_value:4"} {
		if !got[w] {
			t.Errorf("missing %s in %v", w, errs)
		}
	}
}

func TestAxesMergeKeepsExistingValuesAndAppendsNew(t *testing.T) {
	g := Group{OptName: [3]string{"Size", "", ""}, Rows: []Row{{OptValue: [3]string{"M"}}, {OptValue: [3]string{"L"}}, {OptValue: [3]string{"M"}}}}
	want := axesOf(g)
	if len(want) != 1 || strings.Join(want[0].Values, ",") != "M,L" {
		t.Fatalf("axes: %+v", want)
	}
	existing := []catalog.OptionAxis{{Name: "Size", Values: []string{"S", "M"}}}
	merged := mergeAxes(existing, want)
	if strings.Join(merged[0].Values, ",") != "S,M,L" || sameAxes(merged, existing) {
		t.Errorf("merge: %+v", merged)
	}
	if got := mergeAxes(existing, []catalog.OptionAxis{{Name: "Colour", Values: []string{"red"}}}); got[0].Name != "Colour" {
		t.Errorf("different axis replaces: %+v", got)
	}
	if got := mergeAxes(existing, nil); !sameAxes(got, existing) {
		t.Errorf("no axis in file leaves the product alone: %+v", got)
	}
}

func validManual() ManualInput {
	return ManualInput{
		Items:       []ManualItem{{SKUID: "11111111-1111-4111-8111-111111111111", Quantity: 2}},
		Customer:    ManualCustomer{Name: "Wang Xiaoming", Phone: "0912-345-678", Email: "buyer@example.test"},
		Delivery:    ManualDelivery{OptionKey: "22222222-2222-4222-8222-222222222222|TW|home", HomeAddress: &homeAddress},
		PaymentMode: "bank_transfer", Locale: "zh-TW",
	}
}

func TestValidateManualRefusesWhatTheBuyerPathWouldNotTake(t *testing.T) {
	if _, err := ValidateManual(validManual()); err != nil {
		t.Fatalf("valid input refused: %v", err)
	}
	for name, mutate := range map[string]func(*ManualInput){
		"no items":         func(in *ManualInput) { in.Items = nil },
		"duplicate sku":    func(in *ManualInput) { in.Items = append(in.Items, in.Items[0]) },
		"zero quantity":    func(in *ManualInput) { in.Items[0].Quantity = 0 },
		"huge quantity":    func(in *ManualInput) { in.Items[0].Quantity = 1001 },
		"bad sku id":       func(in *ManualInput) { in.Items[0].SKUID = "x" },
		"card":             func(in *ManualInput) { in.PaymentMode = "card" },
		"bad locale":       func(in *ManualInput) { in.Locale = "fr" },
		"empty name":       func(in *ManualInput) { in.Customer.Name = "  " },
		"short phone":      func(in *ManualInput) { in.Customer.Phone = "123" },
		"letters in phone": func(in *ManualInput) { in.Customer.Phone = "09ab345678" },
		"bad email":        func(in *ManualInput) { in.Customer.Email = "no-at-sign" },
		"both destinations": func(in *ManualInput) {
			in.Delivery.CVS = &ManualCVS{StoreCode: "123456", StoreName: "Store", StoreAddress: "Taipei City"}
		},
		"no destination": func(in *ManualInput) { in.Delivery.HomeAddress = nil },
		"bad option key": func(in *ManualInput) { in.Delivery.OptionKey = "home" },
		"empty city":     func(in *ManualInput) { in.Delivery.HomeAddress = &homeAddressNoCity },
	} {
		in := validManual()
		mutate(&in)
		if _, err := ValidateManual(in); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// The wire type has no price, discount or shipping field at all: a merchant cannot override the quote.
	for _, forbidden := range []string{"price", "amount", "total", "discount", "shipping", "unit"} {
		if regexp.MustCompile(`json:"[a-z_]*` + forbidden).MatchString(manualJSONFields) {
			t.Errorf("ManualInput exposes a %s field", forbidden)
		}
	}
}

func TestCapabilityDerivationIsDeterministicScopedAndValid(t *testing.T) {
	m := &ManualOrders{secret: bytes.Repeat([]byte{7}, 43)}
	a, b := m.capability("order", "store-1", "key-1"), m.capability("order", "store-1", "key-1")
	if a != b || len(a) != 43 {
		t.Fatalf("not deterministic or wrong length: %q %q", a, b)
	}
	for _, other := range []string{m.capability("order", "store-2", "key-1"), m.capability("order", "store-1", "key-2"), m.capability("options", "store-1", "key-1")} {
		if other == a {
			t.Error("capability not bound to label, store and key")
		}
	}
	other := &ManualOrders{secret: bytes.Repeat([]byte{8}, 43)}
	if other.capability("order", "store-1", "key-1") == a {
		t.Error("capability not bound to the secret")
	}
	if !regexp.MustCompile(`^mt-[0-9a-f]{32}$`).MatchString(stepKey("some-merchant-key", "begin")) || stepKey("k", "cart") == stepKey("k", "quote") {
		t.Error("step keys")
	}
	_ = sha256.Sum256
}

func TestCutGMVSplitsWindowsAndNeverMixesCurrencies(t *testing.T) {
	rows := []reporting.FinanceRow{
		{Day: "2026-10-01", Currency: "TWD", Environment: "LIVE", CapturedMinor: 100, BankTransferConfirmedMinor: 20, PickupCollectedMinor: 3},
		{Day: "2026-09-30", Currency: "TWD", Environment: "LIVE", CapturedMinor: 1000, BankTransferConfirmedMinor: 200, PickupCollectedMinor: 30},
		{Day: "2026-10-01", Currency: "TWD", Environment: "SANDBOX", CapturedMinor: 7},
		{Day: "2026-09-29", Currency: "USD", Environment: "LIVE", CapturedMinor: 5},
	}
	got := cutGMV(rows, "2026-10-01")
	if len(got) != 3 {
		t.Fatalf("entries: %+v", got)
	}
	live := got[0]
	if live.Currency != "TWD" || live.Environment != "LIVE" || live.Today != (ModeTotals{CardMinor: 100, BankTransferMinor: 20, PayAtPickupMinor: 3}) || live.Last7Days != (ModeTotals{CardMinor: 1100, BankTransferMinor: 220, PayAtPickupMinor: 33}) {
		t.Errorf("TWD LIVE: %+v", live)
	}
	if got[1].Environment != "SANDBOX" || got[1].Today.CardMinor != 7 || got[2].Currency != "USD" || got[2].Today != (ModeTotals{}) || got[2].Last7Days.CardMinor != 5 {
		t.Errorf("others: %+v %+v", got[1], got[2])
	}
	if cutGMV(nil, "2026-10-01") == nil {
		t.Error("empty must be a non-nil slice")
	}
}

var (
	homeAddress       = homeAddressOf("Taipei", "Zhongzheng", "100", "1 Example Road")
	homeAddressNoCity = homeAddressOf("Taipei", "", "100", "1 Example Road")
	// manualJSONFields lists the wire names of ManualInput and its nested types, for the "no price field" check.
	manualJSONFields = `json:"items" json:"sku_id" json:"quantity" json:"customer" json:"name" json:"phone" json:"email" json:"delivery" json:"option_key" json:"home_address" json:"cvs" json:"store_code" json:"store_name" json:"store_address" json:"payment_mode" json:"locale"`
)
