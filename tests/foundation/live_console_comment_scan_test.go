// Purpose: PR18 raw IG scan exhaustion and cursor advancement gates through the real A2 service.
// Depends on: lcnEnv/lcnIG signed webhook fixtures, lcnSeqPage, social.read_comment_events and scoped PG.
// Used by: focused TestLiveConsoleLCN02ScanExhausted and live-console foundation gates.
// Invariants: live-console-v1 §2.6; count/advance raw rows before decryption/media filtering, never filtered items.
// Status: REAL_PG + MOCK Graph; owner writes only task-owned synthetic fixture rows.
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

type lcnRawScanRow struct {
	id  string
	seq int64
}

func lcnRawScan(t *testing.T, e *lcnEnv, session string) []lcnRawScanRow {
	t.Helper()
	var out []lcnRawScanRow
	err := platform.WithScope(context.Background(), e.f.runtime, e.h.token, e.f.storeA1, "live:read", func(tx pgx.Tx, scope platform.Scope) error {
		// The real definer owns raw positions, including rows that A2 cannot decrypt or accept for this media.
		rows, err := tx.Query(context.Background(), `SELECT event_id::text,seq FROM social.read_comment_events($1::uuid,0,100)`, session)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var row lcnRawScanRow
			if err := rows.Scan(&row.id, &row.seq); err != nil {
				return err
			}
			out = append(out, row)
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func lcnAssertScanExhausted(t *testing.T, page live.ConsoleStreamPage, want bool) {
	t.Helper()
	raw, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	expected := "false"
	if want {
		expected = "true"
	}
	if got := string(wire["scan_exhausted"]); got != expected {
		t.Errorf("scan_exhausted=%q want explicit %s (visible items=%d next.seq=%d)", got, expected, len(page.Items), page.Next.Seq)
	}
}

func TestLiveConsoleLCN02ScanExhausted(t *testing.T) {
	t.Run("full-raw-page-all-filtered", func(t *testing.T) {
		e := lcnSetup(t)
		ig := lcnSetupIG(t, e)
		at := time.Now().UTC().Add(-time.Second)
		ig.webhook(t, mciDigits(17), mciDigits(17), mciDigits(16), "", at) // valid ciphertext, foreign media
		corruptRef := mciDigits(17)
		ig.webhook(t, ig.media, corruptRef, mciDigits(16), "", at)
		// Fault injection only into this unit's synthetic event; no production or schema mutation.
		mustExec(t, e.f.owner, `UPDATE social.comment_events SET ciphertext=decode(repeat('00',32),'hex')
			WHERE tenant_id=$1 AND store_id=$2 AND comment_key=$3`, e.f.tenantA, e.f.storeA1,
			crCommentKey(miApp, "instagram", ig.asset, corruptRef))
		raw := lcnRawScan(t, e, ig.session)
		if len(raw) != 2 || raw[1].seq <= raw[0].seq {
			t.Fatalf("raw fixture=%v want two increasing positions", raw)
		}
		c := e.console(t, "worker-scan-filtered", metareply.ConsoleConfig{})
		stream := e.lcnStream(t, c, mcKeys(t, ig.m))
		page := lcnSeqPage(t, e, stream, ig.session, live.ConsolePageQuery{Limit: 2})
		if len(page.Items) != 0 {
			t.Fatalf("items=%d want both raw rows filtered", len(page.Items))
		}
		lcnAssertScanExhausted(t, page, false) // raw count == limit even though visible items are empty
		if page.Next.Seq != raw[1].seq {
			t.Errorf("all-filtered next.seq=%d want highest scanned position %d", page.Next.Seq, raw[1].seq)
		}
		empty := lcnSeqPage(t, e, stream, ig.session, live.ConsolePageQuery{Limit: 2, AfterSeq: &raw[1].seq})
		lcnAssertScanExhausted(t, empty, true)
		if len(empty.Items) != 0 || empty.Next.Seq != raw[1].seq {
			t.Errorf("empty query items=%d next=%d want 0/%d", len(empty.Items), empty.Next.Seq, raw[1].seq)
		}
	})
	t.Run("deleted-final-row-100-to-99", func(t *testing.T) {
		e := lcnSetup(t)
		ig := lcnSetupIG(t, e)
		at := time.Now().UTC().Add(-time.Second).Truncate(time.Second)
		consumer := mcConsumer(t, ig.m) // Reuse one real consumer pool; per-row pools exhaust the PG fixture's 60 slots.
		for i := 0; i < 100; i++ {
			ref := mciDigits(17)
			ev := mcPost(t, ig.m, ig.asset, mciIGBody(ig.asset, "live_comments", ig.media, ref, mciDigits(16),
				"ig_user", "synthetic scan comment", &at, nil))
			mcRunning(t, ig.m, ev, 1)
			if _, err := consumer.Exec(context.Background(), `SELECT meta_inbox.finish_social_event($1::uuid,$2::bigint,$3::integer,'comment',$4)`,
				ev.id, ev.job, 1, crCommentKey(miApp, "instagram", ig.asset, ref)); err != nil {
				t.Fatalf("project IG comment %d: %v", i, err)
			}
		}
		raw := lcnRawScan(t, e, ig.session)
		if len(raw) != 100 || raw[98].seq >= raw[99].seq {
			t.Fatalf("raw fixture count=%d want 100 and increasing final positions", len(raw))
		}
		c := e.console(t, "worker-scan-final", metareply.ConsoleConfig{})
		stream := e.lcnStream(t, c, mcKeys(t, ig.m))
		full := lcnSeqPage(t, e, stream, ig.session, live.ConsolePageQuery{Limit: 100})
		if len(full.Items) != 100 || full.Next.Seq != raw[99].seq {
			t.Fatalf("full items=%d next=%d want 100/%d", len(full.Items), full.Next.Seq, raw[99].seq)
		}
		lcnAssertScanExhausted(t, full, false)
		// Delete only the fixture's final comment copy, reproducing the disappeared historical last row.
		mustExec(t, e.f.owner, `DELETE FROM social.comment_events WHERE tenant_id=$1 AND store_id=$2 AND event_id=$3`,
			e.f.tenantA, e.f.storeA1, raw[99].id)
		trimmed := lcnSeqPage(t, e, stream, ig.session, live.ConsolePageQuery{Limit: 100})
		if len(trimmed.Items) != 99 || trimmed.Next.Seq != raw[98].seq {
			t.Errorf("trimmed items=%d next=%d want 99/%d", len(trimmed.Items), trimmed.Next.Seq, raw[98].seq)
		}
		lcnAssertScanExhausted(t, trimmed, true)
		empty := lcnSeqPage(t, e, stream, ig.session, live.ConsolePageQuery{Limit: 100, AfterSeq: &raw[98].seq})
		if len(empty.Items) != 0 || empty.Next.Seq != raw[98].seq {
			t.Errorf("after99 items=%d next=%d want 0/%d", len(empty.Items), empty.Next.Seq, raw[98].seq)
		}
		lcnAssertScanExhausted(t, empty, true)
	})
	t.Run("facebook-no-eof-proof", func(t *testing.T) {
		e := lcnSetup(t)
		e.graph.setComments(e.postID, []map[string]any{lcnComment(lcnRef(), time.Now().UTC().Format(time.RFC3339), e.asset, "", "synthetic", "", false)})
		c := e.console(t, "worker-scan-fb", metareply.ConsoleConfig{})
		e.demandAndPoll(t, c, e.sourceID)
		stream := e.lcnStream(t, c, nil)
		page := lcnSeqPage(t, e, stream, e.session, live.ConsolePageQuery{Limit: 100})
		lcnAssertScanExhausted(t, page, false)
	})
}
