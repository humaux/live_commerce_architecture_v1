// Purpose: DB-free tests of the Taiwan city allowlist and of its agreement with the CHECK of migrations/0156 (the importer and the table
//   backstop must list exactly the same 22 names).
// Depends on: testing, os, strings.
// Used by: go test ./internal/twcity.

package twcity

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{"台北市": "臺北市", "臺北市": "臺北市", " 台東縣 ": "臺東縣", "新北市": "新北市"} {
		if got, ok := Normalize(in); !ok || got != want {
			t.Errorf("%q -> %q %v, want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "台北", "王小明", "amy@mail.tw", "臺北市中正區", "Taipei City", "臺北市臺北市", "台北市 大安區"} {
		if got, ok := Normalize(in); ok {
			t.Errorf("%q must not be a city, got %q", in, got)
		}
	}
}

func TestListMatchesMigrationCheck(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/0156_order_history_import.sql")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`city text CHECK \(city IN \(([^)]*)\)\)`).FindStringSubmatch(string(raw))
	if m == nil {
		t.Fatal("city CHECK not found in 0156")
	}
	var sql []string
	for _, c := range strings.Split(m[1], ",") {
		sql = append(sql, strings.Trim(c, "'"))
	}
	if len(Cities) != 22 || strings.Join(sql, ",") != strings.Join(Cities, ",") {
		t.Fatalf("SQL list %v != Go list %v", sql, Cities)
	}
}
