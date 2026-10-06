// Purpose: DB-free tests of the pure half of the customer import (customers_csv.go) and the batch helpers: header folding and
//   auto-mapping, explicit mapping, phone normalisation, cell limits, duplicate refusal, consent detection, guard round trip,
//   file-level codes, results.csv rendering rules and that no error text carries file content.
// Depends on: testing only (synthetic names and numbers: +8869000000xx, example.test).
// Used by: go test ./internal/migrationimport.

package migrationimport

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

const header = "顧客編號,姓名,手機,電子郵件,同意行銷,備註\n"

func rowsOf(t *testing.T, csv string, mapping map[string]string) parsedCustomers {
	t.Helper()
	p, code := parseCustomerCSV([]byte(csv), mapping)
	if code != "" {
		t.Fatalf("file code %q", code)
	}
	return p
}

func TestAutoMappingAndRows(t *testing.T) {
	p := rowsOf(t, header+"C1,測試一,0900-000-001,One@Example.Test,是,隨便\nC2,Test Two,+886 900 000 002,,,\n", nil)
	want := map[string]string{"external_id": "顧客編號", "name": "姓名", "phone": "手機", "email": "電子郵件", "consent": "同意行銷"}
	for k, v := range want {
		if p.mapping[k] != v {
			t.Fatalf("mapping[%s]=%q want %q (%v)", k, p.mapping[k], v, p.mapping)
		}
	}
	if len(p.rows) != 2 {
		t.Fatalf("rows %d", len(p.rows))
	}
	r := p.rows[0]
	if r.outcome != "" || r.externalID != "C1" || r.name != "測試一" || r.phone != "+886900000001" || r.email != "one@example.test" || !r.consentIgnored {
		t.Fatalf("row 1: %+v", r)
	}
	if r2 := p.rows[1]; r2.outcome != "" || r2.phone != "+886900000002" || r2.email != "" || r2.consentIgnored {
		t.Fatalf("row 2: %+v", r2)
	}
}

func TestExplicitMappingOverridesAndUnmaps(t *testing.T) {
	csv := "ref,who,tel,mail,tel2\nR1,Alice Test,0900000011,a@example.test,0900000012\n"
	p := rowsOf(t, csv, map[string]string{"external_id": "ref", "name": "WHO", "phone": "tel2", "email": ""})
	r := p.rows[0]
	if r.externalID != "R1" || r.name != "Alice Test" || r.phone != "+886900000012" || r.email != "" {
		t.Fatalf("mapped row: %+v", r)
	}
	for name, m := range map[string]map[string]string{
		"unknown header":   {"external_id": "nope", "name": "who"},
		"unknown field":    {"external_id": "ref", "name": "who", "address": "tel"},
		"column twice":     {"external_id": "ref", "name": "ref"},
		"required unmaped": {"external_id": "ref", "name": ""},
	} {
		if _, code := parseCustomerCSV([]byte(csv), m); code == "" {
			t.Fatalf("%s accepted", name)
		}
	}
	if _, code := parseCustomerCSV([]byte("a,b\n1,2\n"), nil); code != "required" {
		t.Fatalf("unmapped required columns: %q", code)
	}
}

func TestPhoneNormalisation(t *testing.T) {
	ok := map[string]string{
		"0900000001": "+886900000001", "0900-000-001": "+886900000001", "900000001": "+886900000001",
		"+886900000001": "+886900000001", "+886 900 000 001": "+886900000001", "886900000001": "+886900000001",
		"00886900000001": "+886900000001", "+8860900000001": "+886900000001", "(0900) 000.001": "+886900000001",
	}
	for in, want := range ok {
		if got, good := normalizeTWMobile(in); !good || got != want {
			t.Fatalf("%q -> %q %v want %q", in, got, good, want)
		}
	}
	for _, bad := range []string{"0212345678", "02-1234-5678", "090000000", "09000000011", "abc", "+15551234567", "0900000001x", "+", "0800000001"} {
		if got, good := normalizeTWMobile(bad); good {
			t.Fatalf("%q accepted as %q", bad, got)
		}
	}
}

func TestCellRulesAndCodes(t *testing.T) {
	long := strings.Repeat("長", 81)
	cases := map[string]string{
		"C1,Valid,0900000001,v@example.test": "",
		",Valid,,":                           "required",
		"C2,,,":                              "required",
		"C3," + long + ",,":                  "name_too_long",
		"C4,Valid,0212345678,":               "invalid_phone",
		"C5,Valid,,not-an-email":             "invalid_email",
		"C6,\"Two\nLines\",,":                "invalid_name",
		"C7,Zero​Width,,":                    "invalid_name",
		strings.Repeat("x", 65) + ",Valid,,": "invalid_external_id",
	}
	for line, want := range cases {
		p := rowsOf(t, "customer_id,name,phone,email\n"+line+"\n", nil)
		got := p.rows[0].code
		if got != want || (want != "" && p.rows[0].outcome != outcomeFailed) {
			t.Fatalf("%q -> code %q outcome %q, want %q", line, got, p.rows[0].outcome, want)
		}
	}
	// A refused over-long external id is echoed truncated, never in full.
	p := rowsOf(t, "customer_id,name\n"+strings.Repeat("x", 90)+",A\n", nil)
	if len([]rune(p.rows[0].externalID)) != 64 {
		t.Fatalf("echoed id length %d", len([]rune(p.rows[0].externalID)))
	}
}

func TestNameIsNFCAndFormulaGuardIsUnguarded(t *testing.T) {
	// "é" decomposed -> composed; a cell exported by us as '=cmd round-trips to =cmd (stored text), a plain apostrophe name is kept.
	p := rowsOf(t, "customer_id,name\nC1,Café\nC2,'=cmd\nC3,'Neil\nC4,\"'@x\"\n", nil)
	if p.rows[0].name != "Café" {
		t.Fatalf("not NFC: %q", p.rows[0].name)
	}
	if p.rows[1].name != "=cmd" || p.rows[2].name != "'Neil" || p.rows[3].name != "@x" {
		t.Fatalf("unguard: %q %q %q", p.rows[1].name, p.rows[2].name, p.rows[3].name)
	}
}

func TestDuplicateExternalIDRefusesEveryHolder(t *testing.T) {
	p := rowsOf(t, "customer_id,name,phone\nD1,One,\nD1,Two,\nD2,Three,\nD3,Four,\nD3,Five,bad\n", nil)
	got := []string{}
	for _, r := range p.rows {
		got = append(got, r.code)
	}
	want := []string{"duplicate_external_id", "duplicate_external_id", "", "duplicate_external_id", "invalid_phone"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("codes %v want %v", got, want)
	}
}

func TestFileLevelCodes(t *testing.T) {
	big5 := []byte("customer_id,name\nC1,\xa4\xa4\xa4\xe5\n") // invalid UTF-8
	if _, code := parseCustomerCSV(big5, nil); code != "encoding_not_utf8" {
		t.Fatalf("big5: %q", code)
	}
	if _, code := parseCustomerCSV(nil, nil); code != "required" {
		t.Fatalf("empty: %q", code)
	}
	var b strings.Builder
	b.WriteString("customer_id,name\n")
	for i := 0; i < MaxRows+1; i++ {
		fmt.Fprintf(&b, "C%d,N\n", i)
	}
	if _, code := parseCustomerCSV([]byte(b.String()), nil); code != "too_many_rows" {
		t.Fatalf("5001 rows: %q", code)
	}
	b.Reset()
	b.WriteString("customer_id,name\n")
	for i := 0; i < MaxRows; i++ {
		fmt.Fprintf(&b, "C%d,N\n\n", i) // blank lines are skipped and not counted
	}
	if p, code := parseCustomerCSV([]byte(b.String()), nil); code != "" || len(p.rows) != MaxRows {
		t.Fatalf("5000 rows: %q %d", code, len(p.rows))
	}
	if _, code := parseCustomerCSV([]byte("customer_id,name\nC1,\"open\n"), nil); code != "invalid_request" {
		t.Fatalf("malformed quote: %q", code)
	}
	// BOM and a short record.
	p, code := parseCustomerCSV([]byte("\xEF\xBB\xBFcustomer_id,name,phone\nC1,A\n"), nil)
	if code != "" || p.rows[0].code != "invalid_request" {
		t.Fatalf("short record: %q %+v", code, p.rows)
	}
}

func TestParseMapping(t *testing.T) {
	if m, err := ParseMapping(""); err != nil || m != nil {
		t.Fatalf("empty: %v %v", m, err)
	}
	if m, err := ParseMapping(`{"name":"姓名","phone":""}`); err != nil || m["name"] != "姓名" || m["phone"] != "" {
		t.Fatalf("mapping: %v %v", m, err)
	}
	for _, bad := range []string{"null", "[]", `{"name":1}`, "{", `{"name":"` + strings.Repeat("a", 101) + `"}`, strings.Repeat(" ", 2049)} {
		if _, err := ParseMapping(bad); err == nil {
			t.Fatalf("%q accepted", bad)
		}
	}
}

// The PG error mapping keeps the code only: a driver message that quotes a failing row must never survive into an error text.
func TestAbortAndMapPGNeverCarryContent(t *testing.T) {
	secret := "Failing row contains (Alice Secret, +886900000099, secret@example.test)"
	err := mapPG(&pgconn.PgError{Code: "23514", Message: secret, Detail: secret})
	if strings.Contains(err.Error(), "Secret") || strings.Contains(err.Error(), "886900000099") || !strings.Contains(err.Error(), "23514") {
		t.Fatalf("error text: %q", err.Error())
	}
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("not retryable: %v", err)
	}
	if e := mapPG(&pgconn.PgError{Code: "PT403"}); e == nil || strings.Contains(e.Error(), "PT") {
		t.Fatalf("PT403: %v", e)
	}
}

func TestPreviewCountsAndConsentIgnored(t *testing.T) {
	p := rowsOf(t, header+"C1,A,,,是,\nC2,B,,,,\nC3,,,,,\n", nil)
	p.rows[0].outcome, p.rows[1].outcome = outcomeCreated, outcomeUpdated
	pv := buildPreview("deadbeef", p)
	if pv.RowsTotal != 3 || pv.NewRows != 1 || pv.UpdateRows != 1 || pv.ApplyRows != 2 || pv.FailedRows != 1 || pv.ConsentIgnoredRows != 1 {
		t.Fatalf("preview: %+v", pv)
	}
	if !pv.Rows[0].ConsentIgnored || pv.Rows[1].ConsentIgnored || pv.Rows[2].Code != "required" {
		t.Fatalf("rows: %+v", pv.Rows)
	}
}
