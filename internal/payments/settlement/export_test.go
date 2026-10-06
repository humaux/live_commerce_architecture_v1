// export_test.go: DB-free tests of the settlement CSV (contract §6.5, PF13 file half): BOM and header, no PII column, text-cell formula
// guard with numeric cells left numeric, totals block, mode 0600, absolute path only, never overwrite.
// Depends on: export.go. Callers: go test ./internal/payments/settlement.

package settlement

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sample() Statement {
	return Statement{StatementID: "5a5a5a5a-5a5a-4a5a-8a5a-5a5a5a5a5a01", PeriodStart: "2026-09-14", PeriodEnd: "2026-09-21", Currency: "TWD",
		CapturedMinor: 250000, RefundedMinor: 80000, DisputeMinor: 0, StripeFeeMinor: -4100, PlatformFeeMinor: 0, CarriedInMinor: -1000,
		NetPayableMinor: 164900, LineCount: 2,
		Lines: []Line{{OrderNumber: "LC-AAAA", Kind: "CHARGE", StoreMinor: 250000, FeeStoreMinor: -4100, TxnDate: "2026-09-15"},
			{OrderNumber: "=cmd|' /C calc'!A0", Kind: "REFUND", StoreMinor: -80000, FeeStoreMinor: 0, TxnDate: "2026-09-16"}}}
}

func TestWriteCSVLayoutGuardAndNumericCells(t *testing.T) {
	var b strings.Builder
	if err := WriteCSV(&b, sample()); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	want := "\xEF\xBB\xBFperiod_start,period_end,order_number,kind,amount,stripe_fee,txn_date\r\n" +
		"2026-09-14,2026-09-21,LC-AAAA,CHARGE,2500.00,-41.00,2026-09-15\r\n" +
		"2026-09-14,2026-09-21,'=cmd|' /C calc'!A0,REFUND,-800.00,0.00,2026-09-16\r\n" +
		"\r\ntotals_currency,TWD\r\ncaptured,2500.00\r\nrefunded,800.00\r\ndispute,0.00\r\nstripe_fee,-41.00\r\nplatform_fee,0.00\r\ncarried_in,-10.00\r\nnet_payable,1649.00\r\nline_count,2\r\n"
	if got != want {
		t.Fatalf("csv:\n%q\nwant\n%q", got, want)
	}
	for _, pii := range []string{"email", "phone", "recipient", "address", "buyer"} {
		if strings.Contains(strings.ToLower(got), pii) {
			t.Fatalf("csv mentions %q", pii)
		}
	}
}

func TestWriteFileModeAbsolutePathAndNoOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.csv")
	if err := WriteFile(path, sample()); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mode: %v %v", info, err)
	}
	first, _ := os.ReadFile(path)
	if err := WriteFile(path, Statement{StatementID: "x"}); err == nil {
		t.Fatal("an existing file was overwritten")
	}
	if again, _ := os.ReadFile(path); string(again) != string(first) {
		t.Fatal("the refused export changed the file")
	}
	link := filepath.Join(dir, "link.csv")
	if err := os.Symlink(path, link); err == nil {
		if err := WriteFile(link, sample()); err == nil {
			t.Fatal("a symlink path was accepted")
		}
	}
	for _, bad := range []string{"relative.csv", "./x.csv", dir + "/../x.csv", dir + "/sub/missing/x.csv", ""} {
		if err := WriteFile(bad, sample()); err == nil {
			t.Fatalf("path %q accepted", bad)
		}
	}
	if err := WriteFile(path, sample()); err == nil || strings.Contains(err.Error(), dir) {
		t.Fatalf("the error must not echo the path: %v", err)
	}
}
