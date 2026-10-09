// Purpose: require explicit false scan_exhausted when the A2 producer has no raw-scan exhaustion proof.
// Depends on: ConsoleStreamPage and encoding/json; no database or external calls.
// Used by: go test ./internal/live; PR18 additive scan DTO acceptance.
// Invariants: live-console-v1 §2.6 (false is lack of EOF proof, not proof that more rows exist).
package live

import (
	"encoding/json"
	"testing"
)

func TestConsoleStreamPageScanExhaustedWire(t *testing.T) {
	raw, err := json.Marshal(ConsoleStreamPage{})
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	if got := string(wire["scan_exhausted"]); got != "false" {
		t.Fatalf("scan_exhausted=%q want explicit false", got)
	}
}
