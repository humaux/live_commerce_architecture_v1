// Purpose: verify the real console buffer page emits each item's arrival sequence on the bridge wire.
// Depends on: Console.ingest, Console.pageLocked and encoding/json; no DB or Graph credentials.
// Used by: go test ./internal/integrations/metareply; PR18 seq DTO acceptance.
// Invariants: live-console-v1 §2.4/§2.6 (item sequence is not the page cursor or created_at).
package metareply

import (
	"encoding/json"
	"testing"
	"time"
)

func TestConsoleCommentItemSeqWire(t *testing.T) {
	c := &Console{cfg: ConsoleConfig{}.withDefaults()}
	s := &consoleSource{pollEpoch: 7, byRef: map[string]consoleComment{}, state: "live"}
	now := time.Now().UTC()
	// Timestamp order differs from arrival order; the latter is the authoritative buffer seq.
	c.ingest(s, []BridgeComment{{Ref: "101", CreatedAt: now}, {Ref: "102", CreatedAt: now.Add(-time.Hour)}}, now)
	c.ingest(s, []BridgeComment{{Ref: "101", CreatedAt: now}}, now) // duplicates must not renumber
	page := c.pageLocked(s, BridgePageRequest{Limit: 100}, now)
	if page.NextSeq != 1 {
		t.Fatalf("fixture cursor=%d want 1", page.NextSeq)
	}
	raw, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	if len(wire.Items) != 2 {
		t.Fatalf("items=%d want 2", len(wire.Items))
	}
	for i, want := range []string{"2", "1"} {
		if got := string(wire.Items[i]["seq"]); got != want {
			t.Errorf("item %d seq=%q want numeric %s (page cursor=%d)", i, got, want, page.NextSeq)
		}
	}
	// Each serialization owns a stable number; a later ingestion cannot mutate an earlier page.
	c.ingest(s, []BridgeComment{{Ref: "103", CreatedAt: now}}, now)
	again, err := json.Marshal(page)
	if err != nil || string(again) != string(raw) {
		t.Fatal("later ingestion mutated an existing page")
	}
}
