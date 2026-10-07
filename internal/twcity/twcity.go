// Purpose: the closed list of Taiwan's 22 cities and counties used as the ONLY "city" the historical-order archive may hold (W5-03B,
//   migration-import-v1 section 7). Normalize maps an input cell to the canonical spelling (臺 not 台) or reports it is not a city.
// Depends on: strings only.
// Used by: internal/migrationimport/orders_csv.go (cell rule), internal/customers/historical.go (read / export re-validation); the same
//   list is the CHECK of customers.historical_orders.city in migrations/0156_order_history_import.sql (a test compares the two).
// Invariants: pure and total; never returns a part of the input (a street, name or email can never become a city). English names and
//   simplified-Chinese spellings are NOT accepted: the SHOPLINE Taiwan export writes Traditional Chinese, and an unlisted cell is dropped.

package twcity

import "strings"

// Cities is the canonical list: 6 special municipalities, 3 cities, 13 counties (Executive Yuan list), written with 臺.
var Cities = []string{
	"臺北市", "新北市", "桃園市", "臺中市", "臺南市", "高雄市", "基隆市", "新竹市", "嘉義市",
	"新竹縣", "苗栗縣", "彰化縣", "南投縣", "雲林縣", "嘉義縣", "屏東縣", "宜蘭縣", "花蓮縣", "臺東縣", "澎湖縣", "金門縣", "連江縣",
}

var allowed = func() map[string]bool {
	m := make(map[string]bool, len(Cities))
	for _, c := range Cities {
		m[c] = true
	}
	return m
}()

// Valid reports whether s is exactly one canonical city name.
func Valid(s string) bool { return allowed[s] }

// Normalize trims the cell, folds 台 to 臺 and returns the canonical name; ok is false for anything that is not one of the 22.
func Normalize(cell string) (string, bool) {
	c := strings.ReplaceAll(strings.TrimSpace(cell), "台", "臺")
	return c, allowed[c]
}
