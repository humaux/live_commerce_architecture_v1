// DB-free unit tests for the pure half of the bulk tracking import (manual-fulfilment-v1 Amendment
// "M-7 revoked"): the header fold, the order-reference normalization, the carrier alias table, the
// strict CSV parser and the spreadsheet formula guard it shares with the product import. Database
// behaviour (precheck, RecordShipment, idempotency, result.csv) is in
// tests/foundation/tracking_import_test.go.

package merchanttools

import (
	"fmt"
	"strings"
	"testing"
)

func TestTrackingFoldHeader(t *testing.T) {
	cases := map[string]string{
		"   Order_Number  ": "order_number",
		"Ｏｒｄｅｒ＿Ｎｕｍｂｅｒ":      "order_number", // full-width ASCII maps to half-width
		"　訂單編號　":            "訂單編號",         // the ideographic space trims away
		"  Carrier  ":       "carrier",
		"追蹤網址":              "追蹤網址",
	}
	for in, want := range cases {
		if got := foldTrackingHeader(in); got != want {
			t.Errorf("foldTrackingHeader(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTrackingNormalizeOrderRef(t *testing.T) {
	const canonical = "12345678-1234-5678-9abc-def012345678"
	const hex = "12345678123456789ABCDEF012345678"
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"LC-" + hex, canonical, true},
		{"lc-" + strings.ToLower(hex), canonical, true},
		{canonical, canonical, true},
		{strings.ToUpper(canonical), canonical, true},
		{"  " + canonical + "  ", canonical, true}, // surrounding whitespace is trimmed
		{"not-an-order", "", false},
		{"LC-1234", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := normalizeTrackingOrderRef(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("normalizeTrackingOrderRef(%q) = %q,%v want %q,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestTrackingCarrierAliases(t *testing.T) {
	cases := map[string]string{
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
	for alias, want := range cases {
		if got := trackingCarrierAliases[alias]; got != want {
			t.Errorf("trackingCarrierAliases[%q] = %q, want %q", alias, got, want)
		}
	}
	// The CVS chains and sf_express are deliberately absent (T-OPEN-3): they are never bulk-shipped here.
	for _, absent := range []string{"sf_express", "seven_eleven_cvs", "cvs_711"} {
		if _, ok := trackingCarrierAliases[absent]; ok {
			t.Errorf("trackingCarrierAliases must not alias %q", absent)
		}
	}
}

func TestTrackingParseCSV(t *testing.T) {
	t.Run("ascii header and rows", func(t *testing.T) {
		data := "order_number,carrier,tracking_number\n" +
			"12345678-1234-5678-9abc-def012345678,black_cat,0012345678\n" +
			"87654321-4321-8765-cba9-210fedcba987,chunghwa_post,RA123456789TW\n"
		rows, code := parseTrackingCSV([]byte(data))
		if code != "" {
			t.Fatalf("code %q", code)
		}
		if len(rows) != 2 {
			t.Fatalf("rows %d", len(rows))
		}
		if rows[0].n != 1 || rows[0].orderRef != "12345678-1234-5678-9abc-def012345678" ||
			rows[0].carrier != "black_cat" || rows[0].trackingNumber != "0012345678" {
			t.Fatalf("row 1: %+v", rows[0])
		}
		if rows[1].n != 2 || rows[1].carrier != "chunghwa_post" || rows[1].trackingNumber != "RA123456789TW" {
			t.Fatalf("row 2: %+v", rows[1])
		}
	})
	t.Run("UTF-8 BOM, Chinese header and full-width header", func(t *testing.T) {
		data := "\xEF\xBB\xBF訂單編號,物流商,運單號碼\n" +
			"12345678-1234-5678-9abc-def012345678,新竹,ABC-1\n"
		rows, code := parseTrackingCSV([]byte(data))
		if code != "" || len(rows) != 1 {
			t.Fatalf("code %q rows %d", code, len(rows))
		}
		if rows[0].orderRef != "12345678-1234-5678-9abc-def012345678" || rows[0].carrier != "新竹" {
			t.Fatalf("row: %+v", rows[0])
		}
		full := "Ｏｒｄｅｒ＿Ｎｕｍｂｅｒ,ｃａｒｒｉｅｒ,ｔｒａｃｋｉｎｇ＿ｎｕｍｂｅｒ\n" +
			"12345678-1234-5678-9abc-def012345678,black_cat,0012345678\n"
		rows, code = parseTrackingCSV([]byte(full))
		if code != "" || len(rows) != 1 || rows[0].carrier != "black_cat" {
			t.Fatalf("full-width header: code %q rows %+v", code, rows)
		}
	})
	t.Run("unknown column ignored, blank rows skipped", func(t *testing.T) {
		data := "order_number,carrier,tracking_number,extra\n" +
			"\n" +
			"12345678-1234-5678-9abc-def012345678,black_cat,0012345678,ignored\n" +
			"  ,,,\n"
		rows, code := parseTrackingCSV([]byte(data))
		if code != "" || len(rows) != 1 {
			t.Fatalf("code %q rows %d", code, len(rows))
		}
	})
	t.Run("file-level refusals", func(t *testing.T) {
		cases := []struct {
			name, in, code string
		}{
			{"empty", "", "required"},
			{"not utf8", string([]byte{0xff, 0xfe, 0xfd}), "encoding_not_utf8"},
			{"missing column", "order_number,carrier\n12345678-1234-5678-9abc-def012345678,black_cat\n", "required"},
			{"duplicate known", "order_number,order_number,carrier,tracking_number\n", "invalid_request"},
		}
		for _, c := range cases {
			if _, code := parseTrackingCSV([]byte(c.in)); code != c.code {
				t.Errorf("%s: code %q want %q", c.name, code, c.code)
			}
		}
	})
	t.Run("too many rows", func(t *testing.T) {
		var b strings.Builder
		b.WriteString("order_number,carrier,tracking_number\n")
		for i := 0; i <= MaxTrackingImportRows; i++ {
			fmt.Fprintf(&b, "12345678-1234-5678-9abc-def012345678,black_cat,T%03d\n", i)
		}
		if _, code := parseTrackingCSV([]byte(b.String())); code != "too_many_rows" {
			t.Fatalf("code %q want too_many_rows", code)
		}
	})
	t.Run("field-count mismatch is a per-row code, not a file refusal", func(t *testing.T) {
		data := "order_number,carrier,tracking_number\n" +
			"12345678-1234-5678-9abc-def012345678,black_cat,0012345678\n" +
			"only-two-cells\n"
		rows, code := parseTrackingCSV([]byte(data))
		if code != "" || len(rows) != 2 || rows[1].code != "invalid_request" {
			t.Fatalf("code %q rows %+v", code, rows)
		}
	})
}

func TestTrackingGuardCellRoundTrip(t *testing.T) {
	for _, in := range []string{"=cmd", "+1", "-x", "@x", "'=x", "\tlead", "normal", ""} {
		guarded := guardCell(in)
		if got := unguardCell(guarded); got != in {
			t.Errorf("round trip of %q: guardCell=%q unguardCell=%q", in, guarded, got)
		}
	}
	if guardCell("normal") != "normal" || guardCell("") != "" {
		t.Error("guardCell must leave plain and empty cells alone")
	}
	if guardCell("=x") != "'=x" || unguardCell("'=x") != "=x" {
		t.Error("a formula trigger gets exactly one leading apostrophe")
	}
}

func TestTrackingMailETAHours(t *testing.T) {
	cases := map[int]int{0: 0, -1: 0, 1: 1, 29: 1, 30: 1, 31: 2, 60: 2, 500: 17}
	for in, want := range cases {
		if got := mailETAHours(in); got != want {
			t.Errorf("mailETAHours(%d) = %d want %d", in, got, want)
		}
	}
}
