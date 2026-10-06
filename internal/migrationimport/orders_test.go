// Purpose: DB-free tests of the pure half of the historical-order import (orders_csv.go): column auto-mapping, aggregation of
//   one-line-per-item files into one unit per order, amount / date / quantity / city rules, inconsistent orders, the line cap and the
//   guarantee that an address column is never read.
// Depends on: testing only (synthetic ids, names and amounts).
// Used by: go test ./internal/migrationimport.

package migrationimport

import (
	"fmt"
	"strings"
	"testing"
)

const orderHeader = "訂單號碼,顧客編號,訂單日期,訂單狀態,訂單總金額,商品名稱,數量,城市,收件地址\n"

func unitsOf(t *testing.T, csv string, mapping map[string]string) parsedOrders {
	t.Helper()
	p, code := parseOrderCSV([]byte(csv), mapping)
	if code != "" {
		t.Fatalf("file code %q", code)
	}
	return p
}

func TestOrderAggregationAndFields(t *testing.T) {
	p := unitsOf(t, orderHeader+
		"O-1,C-1,2026-03-05 14:30:00,已完成,\"NT$1,280\",紅茶,2,台北市,SECRET-STREET-1\n"+
		"O-1,C-1,2026-03-05 14:30:00,已完成,\"NT$1,280\",綠茶,1,台北市,SECRET-STREET-1\n"+
		"O-2,C-2,2026/03/06,已出貨,300.00,貼紙,,新北市,\n"+
		"O-3,C-1,2026-03-07T10:00:00+09:00,取消,0,,,,\n", nil)
	if len(p.units) != 3 {
		t.Fatalf("units %d, want 3 (7 lines -> orders)", len(p.units))
	}
	u := p.units[0]
	if u.outcome != "" || u.orderID != "O-1" || u.customerID != "C-1" || u.status != "已完成" || u.totalMinor != 128000 ||
		u.items != "紅茶×2、綠茶×1" || u.city != "台北市" || u.n != 1 || u.orderedAt.UTC().Format("2006-01-02T15:04:05Z") != "2026-03-05T06:30:00Z" {
		t.Fatalf("unit 1: %+v", u)
	}
	if u := p.units[1]; u.outcome != "" || u.totalMinor != 30000 || u.items != "貼紙×1" || u.n != 3 {
		t.Fatalf("unit 2: %+v", u)
	}
	// An explicit zone is honoured; an empty item column gives an empty summary.
	if u := p.units[2]; u.outcome != "" || u.orderedAt.UTC().Format("2006-01-02T15:04:05Z") != "2026-03-07T01:00:00Z" || u.items != "" || u.totalMinor != 0 {
		t.Fatalf("unit 3: %+v", u)
	}
	// The address column is not part of the mapping and no field holds its text.
	if _, mapped := p.mapping["address"]; mapped {
		t.Fatal("an address column must never be mapped")
	}
	if fmt.Sprintf("%+v", p.units) == "" || strings.Contains(fmt.Sprintf("%+v", p.units), "SECRET-STREET") {
		t.Fatalf("address text leaked into a unit: %+v", p.units)
	}
}

func TestOrderRowFailures(t *testing.T) {
	long := strings.Repeat("a", 65)
	for name, tc := range map[string]struct{ line, code string }{
		"missing order id":    {",C-1,2026-03-05,完成,100,x,1,台北市,", "required"},
		"order id is a phone": {"0912345678,C-1,2026-03-05,完成,100,x,1,台北市,", "invalid_order_id"},
		"order id too long":   {long + ",C-1,2026-03-05,完成,100,x,1,台北市,", "invalid_order_id"},
		"customer is email":   {"O-9,a@example.test,2026-03-05,完成,100,x,1,台北市,", "invalid_external_id"},
		"missing customer":    {"O-9,,2026-03-05,完成,100,x,1,台北市,", "required"},
		"bad date":            {"O-9,C-1,yesterday,完成,100,x,1,台北市,", "invalid_date"},
		"year out of range":   {"O-9,C-1,1969-01-01,完成,100,x,1,台北市,", "invalid_date"},
		"fraction":            {"O-9,C-1,2026-03-05,完成,100.5,x,1,台北市,", "invalid_amount"},
		"negative":            {"O-9,C-1,2026-03-05,完成,-5,x,1,台北市,", "invalid_amount"},
		"exponent":            {"O-9,C-1,2026-03-05,完成,1e3,x,1,台北市,", "invalid_amount"},
		"missing total":       {"O-9,C-1,2026-03-05,完成,,x,1,台北市,", "required"},
		"missing status":      {"O-9,C-1,2026-03-05,,100,x,1,台北市,", "required"},
		"status too long":     {"O-9,C-1,2026-03-05," + strings.Repeat("狀", 41) + ",100,x,1,台北市,", "invalid_status"},
		"quantity zero":       {"O-9,C-1,2026-03-05,完成,100,x,0,台北市,", "invalid_quantity"},
		"quantity text":       {"O-9,C-1,2026-03-05,完成,100,x,two,台北市,", "invalid_quantity"},
		"address in city":     {"O-9,C-1,2026-03-05,完成,100,x,1,中山路12號,", "invalid_city"},
		"long city":           {"O-9,C-1,2026-03-05,完成,100," + "x,1," + strings.Repeat("市", 21) + ",", "invalid_city"},
		"field count":         {"O-9,C-1,2026-03-05", "invalid_request"},
	} {
		p := unitsOf(t, orderHeader+tc.line+"\n", nil)
		if len(p.units) != 1 || p.units[0].outcome != outcomeFailed || p.units[0].code != tc.code {
			t.Errorf("%s: %+v want %s", name, p.units, tc.code)
		}
	}
}

func TestOrderInconsistentLinesFailTheWholeOrder(t *testing.T) {
	p := unitsOf(t, orderHeader+
		"O-1,C-1,2026-03-05,完成,100,a,1,台北市,\n"+
		"O-1,C-1,2026-03-05,完成,200,b,1,台北市,\n"+ // total differs
		"O-2,C-1,2026-03-05,完成,100,a,1,台北市,\n"+
		"O-2,C-2,2026-03-05,完成,100,b,1,台北市,\n"+ // customer differs
		"O-3,C-1,2026-03-05,完成,100,a,1,台北市,\n"+
		"O-3,,,,,b,2,,\n", nil) // later lines may leave the order-level cells empty
	codes := []string{p.units[0].code, p.units[1].code, p.units[2].code}
	if codes[0] != "inconsistent_order" || codes[1] != "inconsistent_order" || codes[2] != "" || p.units[2].items != "a×1、b×2" {
		t.Fatalf("units %+v", p.units)
	}
}

func TestOrderFileCodesAndLineCap(t *testing.T) {
	if _, code := parseOrderCSV([]byte(""), nil); code != "required" {
		t.Fatalf("empty file: %q", code)
	}
	if _, code := parseOrderCSV([]byte("訂單號碼,顧客編號,訂單日期\n"), nil); code != "required" { // status and total unmapped
		t.Fatalf("missing required columns: %q", code)
	}
	if _, code := parseOrderCSV([]byte("\xff\xfe"), nil); code != "encoding_not_utf8" {
		t.Fatalf("not utf8: %q", code)
	}
	var b strings.Builder
	b.WriteString(orderHeader)
	for i := 0; i < MaxRows+1; i++ {
		fmt.Fprintf(&b, "O-%d,C-1,2026-03-05,完成,100,x,1,台北市,\n", i)
	}
	if _, code := parseOrderCSV([]byte(b.String()), nil); code != "too_many_rows" {
		t.Fatalf("%d lines: %q", MaxRows+1, code)
	}
	// An explicit mapping may unmap the optional city, and an unknown field is refused.
	if p := unitsOf(t, orderHeader+"O-1,C-1,2026-03-05,完成,100,x,1,中山路12號,\n", map[string]string{"city": ""}); p.units[0].outcome != "" || p.units[0].city != "" {
		t.Fatalf("unmapped city: %+v", p.units[0])
	}
	if _, code := parseOrderCSV([]byte(orderHeader), map[string]string{"address": "收件地址"}); code != "invalid_request" {
		t.Fatalf("unknown mapping field: %q", code)
	}
}

func TestItemsSummaryIsBounded(t *testing.T) {
	var b strings.Builder
	b.WriteString(orderHeader)
	for i := 0; i < 80; i++ {
		fmt.Fprintf(&b, "O-1,C-1,2026-03-05,完成,100,商品名稱很長很長很長%d,3,台北市,\n", i)
	}
	p := unitsOf(t, b.String(), nil)
	if p.units[0].outcome != "" || len([]rune(p.units[0].items)) != maxItemsRunes {
		t.Fatalf("items summary %d runes, outcome %q", len([]rune(p.units[0].items)), p.units[0].outcome)
	}
}
