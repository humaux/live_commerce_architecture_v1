//go:build browser

// Purpose: prevent LC-U2a Playwright assertion text entering persisted diagnostic logs.
// Depends on: standard-library bounded buffering; static test IDs and source coordinates only.
// Used by: TestBrowserLiveConsoleRealChain and its DB-free diagnostic gate.
// Invariants: I11; output never forwards arbitrary titles, comments or names.
package foundation_test

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"
)

// consoleDiagnosticBuffer keeps at most 1 MiB in process memory, never on disk.
type consoleDiagnosticBuffer struct{ buf bytes.Buffer }

func (b *consoleDiagnosticBuffer) Bytes() []byte { return b.buf.Bytes() }
func (b *consoleDiagnosticBuffer) Len() int      { return b.buf.Len() }

func (b *consoleDiagnosticBuffer) Write(p []byte) (int, error) {
	n := len(p)
	keep := (1 << 20) - b.Len()
	if keep > n {
		keep = n
	}
	if keep > 0 {
		_, _ = b.buf.Write(p[:keep])
	}
	return n, nil
}

// consolePlaywrightSummary keeps bounded numeric facts and known source coordinates.
func consolePlaywrightSummary(raw []byte) string {
	lines := []string{}
	for _, m := range regexp.MustCompile(inboxFailureCountPattern).FindAllSubmatch(raw, 20) {
		lines = append(lines, fmt.Sprintf("%s %s", m[1], m[2]))
	}
	for _, m := range regexp.MustCompile(`(?m)^[\t ]*[0-9]+\)[\t ]+(?:tests/admin/)?live-console\.spec\.ts:([0-9]{1,6}):([0-9]{1,6})[\t ]+›`).FindAllSubmatch(raw, 20) {
		if len(lines) >= 20 {
			break
		}
		lines = append(lines, fmt.Sprintf("failed live-console.spec.ts:%s:%s", m[1], m[2]))
	}
	if len(lines) == 0 {
		return "No structured Playwright summary available; raw output withheld.\n"
	}
	return strings.Join(lines, "\n") + "\n"
}

func TestBrowserLiveConsoleDiagnosticPrivacy(t *testing.T) {
	const secret = "SYNTHETIC_PRIVATE_COMMENT"
	raw := []byte("  1) tests/admin/live-console.spec.ts:44:3 › LC-U2a " + secret + "\n  Error: " + secret + "\n  1 failed " + secret + "\n  17 passed (1m)\n")
	got := consolePlaywrightSummary(raw)
	if strings.Contains(got, secret) || got != "1 failed\n17 passed\nfailed live-console.spec.ts:44:3\n" {
		t.Fatal("console diagnostics expose private text or omit fixed failure coordinates")
	}
	var b consoleDiagnosticBuffer
	large := bytes.Repeat([]byte("x"), 2<<20)
	if n, err := b.Write(large); n != len(large) || err != nil || b.Len() != 1<<20 {
		t.Fatal("diagnostic memory bound failed")
	}
	var copyBuffer consoleDiagnosticBuffer
	if _, err := io.Copy(&copyBuffer, strings.NewReader(string(large))); err != nil || copyBuffer.Len() != 1<<20 {
		t.Fatal("diagnostic io.Copy memory bound failed")
	}
	if got := consolePlaywrightSummary([]byte(strings.Repeat("  1) live-console.spec.ts:44:3 ›\n", 50))); strings.Count(got, "\n") != 20 {
		t.Fatal("diagnostic record bound failed")
	}
}
