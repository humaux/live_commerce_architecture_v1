// Purpose: independently accept the owner-authorized A8 live_comment keyset upgrade:
// complete bounded HTTP pagination, tied timestamps/UUID ordering and session/store/tenant isolation.
// Depends on: bpEnv real HTTP router/REAL_PG fixtures, claims.bundles, live.sessions and
// inbox.live_comment_bundles; opaque next_cursor is consumed solely by the real route.
// Used by: focused TestLiveConsoleBuyerPanelReviewLiveCommentPagination acceptance runs.

package foundation_test

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestLiveConsoleBuyerPanelReviewLiveCommentPagination(t *testing.T) {
	e := bpNew(t)
	s1 := e.session("bp-review-pages-1")
	s2 := e.session("bp-review-pages-2")
	newer := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	older := newer.Add(-time.Hour)
	const limit = 4
	type expectedRow struct {
		id, session, platform string
		at                    time.Time
		pending               bool
	}
	var expected []expectedRow
	for i := 0; i < 11; i++ {
		session := s1
		if i >= 8 {
			session = s2
		}
		platform, rendered := "facebook", "messenger"
		if i%2 == 1 {
			platform, rendered = "instagram", "instagram"
		}
		at := older
		if i < 5 || i == 8 {
			at = newer
		}
		pending := i%3 == 0
		id := e.bundle(session, platform, bpActorKey(fmt.Sprintf("bp-review-pages-%d", i)), nil, nil, pending)
		e.exec(`UPDATE claims.bundles SET created_at=$4 WHERE tenant_id=$1 AND store_id=$2 AND id=$3`, e.tenant, e.store1, id, at)
		expected = append(expected, expectedRow{id, session, rendered, at, pending})
	}
	// UUID text comparison matches PostgreSQL UUID byte ordering for canonical UUIDs.
	// Both timestamp groups exceed one page, so a timestamp-only keyset loses rows.
	slices.SortFunc(expected, func(a, b expectedRow) int {
		if !a.at.Equal(b.at) {
			return b.at.Compare(a.at)
		}
		return strings.Compare(b.id, a.id)
	})

	// Real excluded rows are deliberately newer than the visible rows.
	label := "bp-review-manual"
	e.bundle(s1, "manual", bpActorKey("bp-review-manual"), nil, &label, false)
	purged := e.bundle(s1, "facebook", bpActorKey("bp-review-pages-purged"), nil, nil, false)
	e.exec(`UPDATE claims.bundles SET purged_at=clock_timestamp() WHERE tenant_id=$1 AND store_id=$2 AND id=$3`, e.tenant, e.store1, purged)
	store2Session := randomUUID()
	e.exec(`INSERT INTO live.sessions(id,tenant_id,store_id,principal_id,title) VALUES($1,$2,$3,$4,'bp-review-pages-store2')`,
		store2Session, e.tenant, e.store2, e.principal)
	e.exec(`INSERT INTO live.claim_windows(tenant_id,store_id,session_id,state,match_mode,generation,principal_id)
		VALUES($1,$2,$3,'CLOSED','EXACT',0,$4)`, e.tenant, e.store2, store2Session, e.principal)
	e.exec(`INSERT INTO claims.bundles(tenant_id,store_id,session_id,platform,actor_key)
		VALUES($1,$2,$3,'facebook',$4)`, e.tenant, e.store2, store2Session, bpActorKey("bp-review-pages-store2"))
	other := bpNew(t)
	otherSession := other.session("bp-review-pages-other-tenant")
	otherBundle := other.bundle(otherSession, "facebook", bpActorKey("bp-review-pages-other-tenant"), nil, nil, false)
	code, out, raw, _ := other.get(other.token, "/inbox/conversations?filter=live_comment&session_id="+otherSession)
	if code != 200 || len(bpArr(out, "items")) != 1 || bpStr(bpMap(bpArr(out, "items")[0]), "bundle_id") != otherBundle {
		t.Fatalf("other-tenant positive control: status=%d body=%s", code, raw)
	}

	for _, session := range []string{"", s1, s2} {
		name := "whole_store"
		if session != "" {
			name = "session_" + session
		}
		t.Run(name, func(t *testing.T) {
			var want []expectedRow
			for _, row := range expected {
				if session == "" || row.session == session {
					want = append(want, row)
				}
			}
			seenIDs := make(map[string]bool)
			seenCursors := make(map[string]bool)
			cursor := ""
			position := 0
			for page := 0; page < (len(want)+limit-1)/limit; page++ {
				query := url.Values{"filter": {"live_comment"}, "limit": {fmt.Sprint(limit)}}
				if session != "" {
					query.Set("session_id", session)
				}
				if cursor != "" {
					query.Set("cursor", cursor)
				}
				code, out, raw, _ := e.get(e.token, "/inbox/conversations?"+query.Encode())
				if code != 200 {
					t.Fatalf("page=%d status=%d body=%s", page+1, code, raw)
				}
				items := bpArr(out, "items")
				wantCount := min(limit, len(want)-position)
				if len(items) != wantCount || len(items) == 0 {
					t.Fatalf("page=%d items=%d want %d positive rows; body=%s", page+1, len(items), wantCount, raw)
				}
				for _, value := range items {
					row := bpMap(value)
					id := bpStr(row, "bundle_id")
					w := want[position]
					if seenIDs[id] || id != w.id {
						t.Fatalf("page=%d position=%d id=%s want=%s without duplicates (created_at DESC, UUID DESC); body=%s", page+1, position, id, w.id, raw)
					}
					seenIDs[id] = true
					if row["conversation_id"] != nil || bpStr(row, "session_id") != w.session || bpStr(row, "platform") != w.platform {
						t.Errorf("out-of-scope/incorrect row=%v want session=%s platform=%s conversation=null", row, w.session, w.platform)
					}
					at, err := time.Parse(time.RFC3339Nano, bpStr(row, "last_at"))
					if err != nil || !at.Equal(w.at) {
						t.Errorf("last_at=%v want=%s err=%v", row["last_at"], w.at.Format(time.RFC3339Nano), err)
					}
					if pending, _ := row["link_pending_manual"].(bool); pending != w.pending {
						t.Errorf("pending=%v want=%v row=%v", pending, w.pending, row)
					}
					position++
				}
				cursor = bpStr(out, "next_cursor")
				if position < len(want) {
					if cursor == "" || seenCursors[cursor] {
						t.Fatalf("page=%d has %d unseen rows but no advancing next_cursor; body=%s", page+1, len(want)-position, raw)
					}
					seenCursors[cursor] = true
				} else if cursor != "" {
					// A full final page may advertise an empty tail, as the existing
					// conversation cursor does; consume it through the real HTTP route.
					query.Set("cursor", cursor)
					code, tail, raw, _ := e.get(e.token, "/inbox/conversations?"+query.Encode())
					if code != 200 || len(bpArr(tail, "items")) != 0 || bpStr(tail, "next_cursor") != "" {
						t.Fatalf("terminal tail status=%d body=%s", code, raw)
					}
				}
			}
			if position != len(want) || len(seenIDs) != len(want) {
				t.Fatalf("collected=%d unique=%d want=%d", position, len(seenIDs), len(want))
			}
		})
	}

	for _, session := range []string{store2Session, otherSession} {
		code, out, raw, _ := e.get(e.token, "/inbox/conversations?filter=live_comment&session_id="+session)
		if code != 200 || len(bpArr(out, "items")) != 0 || bpStr(out, "next_cursor") != "" {
			t.Errorf("foreign session is visible under authenticated store: status=%d body=%s", code, raw)
		}
	}
}
