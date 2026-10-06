// Purpose: the operator CSV of one settlement statement (contract §6.5 settlement-export): UTF-8 BOM, formula-guarded text cells,
//   a totals block, written once to a new absolute path with mode 0600. No buyer PII column exists: an order number is the only
//   order identifier.
// Depends on: internal/csvguard (shared spreadsheet formula guard), the standard library.
// Used by: cmd/stripe-admin settlement-export (through stripeadmin.SettlementStatement), tests.
// Invariants: numeric cells come from int64 formatting only (a negative fee must stay numeric, so only TEXT cells are guarded);
//   the file never overwrites an existing path (O_EXCL, which also refuses a symlink at the path).
// Status: MOCK + REAL_PG.

package settlement

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"livecommerce/internal/csvguard"
)

// ErrExportPath is a refused output path (relative, existing, or not writable). The message never echoes the path.
var ErrExportPath = errors.New("settlement: export path refused")

// Header is the CSV column row (contract §6.5; the brief adds txn_date for the Taipei date of each line).
var Header = []string{"period_start", "period_end", "order_number", "kind", "amount", "stripe_fee", "txn_date"}

// major renders a signed minor amount with two decimals (TWD minor units are 1/100 NT$ in this ledger).
func major(minor int64) string {
	sign, v := "", minor
	if minor < 0 {
		sign, v = "-", -minor
	}
	return fmt.Sprintf("%s%d.%02d", sign, v/100, v%100)
}

func writeLine(b *bytes.Buffer, cells []string, text []bool) {
	for i, cell := range cells {
		if i > 0 {
			b.WriteByte(',')
		}
		if text[i] {
			cell = csvguard.Cell(cell)
		}
		if strings.ContainsAny(cell, ",\"\r\n") {
			b.WriteByte('"')
			b.WriteString(strings.ReplaceAll(cell, `"`, `""`))
			b.WriteByte('"')
		} else {
			b.WriteString(cell)
		}
	}
	b.WriteString("\r\n")
}

// WriteCSV renders the statement: BOM, header, one row per line, a blank row, then the totals block.
func WriteCSV(w io.Writer, st Statement) error {
	var b bytes.Buffer
	b.WriteString("\xEF\xBB\xBF")
	allText := []bool{true, true, true, true, true, true, true}
	writeLine(&b, Header, allText)
	for _, l := range st.Lines {
		writeLine(&b, []string{st.PeriodStart, st.PeriodEnd, l.OrderNumber, l.Kind, major(l.StoreMinor), major(l.FeeStoreMinor), l.TxnDate},
			[]bool{true, true, true, true, false, false, true})
	}
	b.WriteString("\r\n")
	for _, row := range [][2]string{
		{"totals_currency", st.Currency}, {"captured", major(st.CapturedMinor)}, {"refunded", major(st.RefundedMinor)},
		{"dispute", major(st.DisputeMinor)}, {"stripe_fee", major(st.StripeFeeMinor)}, {"platform_fee", major(st.PlatformFeeMinor)},
		{"carried_in", major(st.CarriedInMinor)}, {"net_payable", major(st.NetPayableMinor)}, {"line_count", strconv.Itoa(st.LineCount)},
	} {
		writeLine(&b, []string{row[0], row[1]}, []bool{true, row[0] == "totals_currency"})
	}
	_, err := w.Write(b.Bytes())
	return err
}

// WriteFile writes the CSV to path, which must be absolute and must not exist; the file is created 0600. A failed write removes it.
func WriteFile(path string, st Statement) (err error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return ErrExportPath
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return ErrExportPath
	}
	defer func() {
		if err != nil {
			_ = f.Close()
			_ = os.Remove(path)
		}
	}()
	if err = f.Chmod(0o600); err != nil { // the umask can only narrow, but never rely on it
		return ErrExportPath
	}
	if err = WriteCSV(f, st); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	return f.Close()
}
