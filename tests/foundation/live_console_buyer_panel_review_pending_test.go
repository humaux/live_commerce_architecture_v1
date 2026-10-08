// Purpose: independently guard A8 session filtering before the pending-bundle bound;
// a session's actionable row must survive newer rows from another session.
// Depends on: bpEnv real HTTP router/REAL_PG fixtures, claims.bundles and
// inbox.link_pending_bundles; LC-B3b A8 session_id and Amendment 1 P2-2.
// Used by: focused TestLiveConsoleBuyerPanelReviewPendingSessionBeforeLimit runs.

package foundation_test

import (
	"fmt"
	"testing"
)

func TestLiveConsoleBuyerPanelReviewPendingSessionBeforeLimit(t *testing.T) {
	e := bpNew(t)
	wantedSession := e.session("bp-review-pending-wanted")
	otherSession := e.session("bp-review-pending-other")
	wanted := e.bundle(wantedSession, "facebook", bpActorKey("bp-review-pending-wanted"), nil, nil, true)
	e.exec(`UPDATE claims.bundles SET created_at='2026-01-01T00:00:00Z' WHERE tenant_id=$1 AND store_id=$2 AND id=$3`,
		e.tenant, e.store1, wanted)
	for i := 0; i < 51; i++ {
		other := e.bundle(otherSession, "facebook", bpActorKey(fmt.Sprintf("bp-review-pending-other-%d", i)), nil, nil, true)
		e.exec(`UPDATE claims.bundles SET created_at='2026-01-02T00:00:00Z' WHERE tenant_id=$1 AND store_id=$2 AND id=$3`,
			e.tenant, e.store1, other)
	}

	// Positive control: the newer distractors are real pending rows visible through A8.
	code, out, raw, _ := e.get(e.token, "/inbox/conversations?filter=unreplied&session_id="+otherSession)
	if code != 200 || len(bpArr(out, "items")) == 0 {
		t.Fatalf("other-session control: status=%d body=%s", code, raw)
	}
	for _, row := range bpArr(out, "items") {
		if bpStr(bpMap(row), "session_id") != otherSession {
			t.Fatalf("other-session control leaked a row: %s", raw)
		}
	}

	for _, filter := range []string{"all", "unreplied"} {
		t.Run(filter, func(t *testing.T) {
			code, out, raw, _ := e.get(e.token, "/inbox/conversations?filter="+filter+"&session_id="+wantedSession)
			if code != 200 {
				t.Fatalf("A8 status=%d body=%s", code, raw)
			}
			items := bpArr(out, "items")
			if len(items) != 1 {
				t.Fatalf("wanted session items=%d want 1 despite 51 newer other-session rows; body=%s", len(items), raw)
			}
			item := bpMap(items[0])
			if bpStr(item, "bundle_id") != wanted || bpStr(item, "session_id") != wantedSession || item["conversation_id"] != nil {
				t.Fatalf("wanted session row=%v want bundle=%s session=%s conversation=null", item, wanted, wantedSession)
			}
			if pending, _ := item["link_pending_manual"].(bool); !pending {
				t.Errorf("wanted row lost link_pending_manual: %v", item)
			}
			if unreplied, _ := item["unreplied"].(bool); !unreplied {
				t.Errorf("wanted row is not actionable under unreplied: %v", item)
			}
		})
	}
}
