// Purpose: PR18 A2 item-seq red/green gates through real FB bridge and IG webhook-copy read seams.
// Depends on: lcnEnv/lcnIG fixtures, live.CommentStream, social.read_comment_events and scoped real PG.
// Used by: focused TestLiveConsoleLCN02ItemSeqWire and live-console foundation gates.
// Invariants: live-console-v1 §2.4/§2.6; FB/IG item numbers are authoritative, direct Graph history is null.
// Status: REAL_PG + MOCK Graph; no live Meta calls.
package foundation_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/integrations/metareply"
	"livecommerce/internal/live"
	"livecommerce/internal/platform"
)

// lcnSeqPage reads the actual A2 service under the authenticated runtime scope.
func lcnSeqPage(t *testing.T, e *lcnEnv, stream *live.CommentStream, session string, q live.ConsolePageQuery) live.ConsoleStreamPage {
	t.Helper()
	var out live.ConsoleStreamPage
	err := platform.WithScope(context.Background(), e.f.runtime, e.h.token, e.f.storeA1, "live:read", func(tx pgx.Tx, scope platform.Scope) error {
		var err error
		out, err = stream.Comments(context.Background(), tx, scope, e.h.token, session, q)
		return err
	})
	if err != nil {
		t.Fatalf("A2 comments: %v", err)
	}
	return out
}

// lcnAssertItemSeq tests JSON presence and value without requiring a field missing on pre-fix Go types.
func lcnAssertItemSeq(t *testing.T, page any, want map[string]*int64) {
	t.Helper()
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
	if len(wire.Items) != len(want) {
		t.Fatalf("wire items=%d want %d", len(wire.Items), len(want))
	}
	for _, it := range wire.Items {
		var ref string
		if err := json.Unmarshal(it["ref"], &ref); err != nil {
			t.Fatal(err)
		}
		seq, exists := it["seq"]
		expected, known := want[ref]
		if !known {
			t.Fatalf("unexpected ref %q", ref)
		}
		if !exists {
			t.Errorf("ref %s: seq omitted; must emit a number or explicit null", ref)
			continue
		}
		if expected == nil {
			if string(seq) != "null" {
				t.Errorf("history ref %s: seq=%s want explicit null", ref, seq)
			}
			continue
		}
		var got int64
		if string(seq) == "null" || json.Unmarshal(seq, &got) != nil || got != *expected {
			t.Errorf("ref %s: seq=%s want numeric %d", ref, seq, *expected)
		}
	}
}

func TestLiveConsoleLCN02ItemSeqWire(t *testing.T) {
	t.Run("facebook-buffer-and-direct-history", func(t *testing.T) {
		e := lcnSetup(t)
		now := time.Now().UTC()
		olderRef, newerRef := lcnRef(), lcnRef()
		e.graph.setComments(e.postID, []map[string]any{
			lcnComment(olderRef, now.Add(-time.Minute).Format(time.RFC3339), e.asset, "", "synthetic one", "", false),
			lcnComment(newerRef, now.Format(time.RFC3339), e.asset, "", "synthetic two", "", false),
		})
		c := e.console(t, "worker-seq-fb", metareply.ConsoleConfig{})
		bridgePage := e.demandAndPoll(t, c, e.sourceID)
		one, two := int64(1), int64(2)
		want := map[string]*int64{olderRef: &one, newerRef: &two}
		lcnAssertItemSeq(t, bridgePage, want)
		stream := e.lcnStream(t, c, nil)
		page := lcnSeqPage(t, e, stream, e.session, live.ConsolePageQuery{Limit: 100})
		lcnAssertItemSeq(t, page, want)
		if page.OlderCursor == nil {
			t.Fatal("fixture has no history cursor")
		}
		// Graph deliberately returns the same refs as the buffer: direct history still has no seq.
		history := lcnSeqPage(t, e, stream, e.session, live.ConsolePageQuery{Limit: 100, BeforeCursor: page.OlderCursor})
		lcnAssertItemSeq(t, history, map[string]*int64{olderRef: nil, newerRef: nil})
		if !e.graph.hasTarget("before=BEFORE_CURSOR_0") {
			t.Fatal("history did not use the real Graph backfill seam")
		}
	})
	t.Run("instagram-webhook-envelope", func(t *testing.T) {
		e := lcnSetup(t)
		ig := lcnSetupIG(t, e)
		firstRef, secondRef := mciDigits(17), mciDigits(17)
		at := time.Now().UTC().Add(-time.Second).Truncate(time.Second)
		ig.webhook(t, ig.media, firstRef, mciDigits(16), "", at)
		ig.webhook(t, ig.media, secondRef, mciDigits(16), "", at) // same created_at, different receipt seq
		var seqs []int64
		err := platform.WithScope(context.Background(), e.f.runtime, e.h.token, e.f.storeA1, "live:read", func(tx pgx.Tx, scope platform.Scope) error {
			// Read the authoritative values from the same definer used by the real A2 path.
			rows, err := tx.Query(context.Background(), `SELECT seq FROM social.read_comment_events($1::uuid,0,100)`, ig.session)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var seq int64
				if err := rows.Scan(&seq); err != nil {
					return err
				}
				seqs = append(seqs, seq)
			}
			return rows.Err()
		})
		if err != nil || len(seqs) != 2 || seqs[0] == seqs[1] {
			t.Fatalf("authoritative seqs=%v err=%v", seqs, err)
		}
		c := e.console(t, "worker-seq-ig", metareply.ConsoleConfig{})
		stream := e.lcnStream(t, c, mcKeys(t, ig.m))
		page := lcnSeqPage(t, e, stream, ig.session, live.ConsolePageQuery{Limit: 100})
		lcnAssertItemSeq(t, page, map[string]*int64{firstRef: &seqs[0], secondRef: &seqs[1]})
		if page.Next.Seq != seqs[1] {
			t.Fatalf("page next=%d want %d", page.Next.Seq, seqs[1])
		}
	})
}
