// Purpose: UR02 store-level read/reset through committed inbound messages and the production trigger (#242).
// Depends on: lbSetup signed MOCK webhook/real consumer, real PG, platform.WithScope and inbox.Service.
// Used by: CI foundation catch-all shard; replaces internal/inbox/unread_test.go's simulated UR02.
// Invariants: no direct last_inbound_seq writes or disabled triggers; all original UR02 permissions remain asserted.
package foundation_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/inbox"
	"livecommerce/internal/platform"
)

func TestOC12UnreadStoreLevelAndReset(t *testing.T) {
	e := lbSetup(t)
	f := e.h.f
	ctx := context.Background()
	_, reader := lcPerson(t, f, f.tenantA, f.storeA1, "inbox:read")
	_, replier := lcPerson(t, f, f.tenantA, f.storeA1, "inbox:read", "inbox:reply")
	_, other := lcPerson(t, f, f.tenantA, f.storeA1, "inbox:read", "inbox:reply")
	list := func() inbox.ConversationList {
		t.Helper()
		var l inbox.ConversationList
		if err := platform.WithScope(ctx, f.runtime, other, f.storeA1, "inbox:read", func(tx pgx.Tx, _ platform.Scope) error {
			req := inbox.ListRequest{Filter: "unread", Limit: 50}
			for {
				page, err := e.svc.ListConversations(ctx, tx, req)
				if err != nil {
					return err
				}
				l.Items = append(l.Items, page.Items...)
				l.UnreadTotal = page.UnreadTotal
				if len(page.Items) < req.Limit {
					return nil
				}
				last := page.Items[len(page.Items)-1]
				req.LastAt, req.CursorID = &last.LastAt, &last.ConversationID
			}
		}); err != nil {
			t.Fatal(err)
		}
		return l
	}
	// The foundation store is shared with other serial scenarios; pin its existing unread total.
	baseline := list().UnreadTotal
	const peer = "900002420003"
	conv := e.postDM(t, peer, "SYNTHETIC-UR02-FIRST-INBOUND", time.Now())
	messageSeq := func(wantCount int) int64 {
		t.Helper()
		var count int
		var seq, stateSeq int64
		if err := f.owner.QueryRow(ctx, `SELECT count(*),max(server_seq) FROM social.messages WHERE conversation_id=$1`, conv).Scan(&count, &seq); err != nil {
			t.Fatal(err)
		}
		if err := f.owner.QueryRow(ctx, `SELECT last_inbound_seq FROM inbox.conversation_state WHERE conversation_id=$1`, conv).Scan(&stateSeq); err != nil {
			t.Fatal(err)
		}
		if count != wantCount || seq <= 0 || stateSeq != seq {
			t.Fatalf("committed inbound rows=%d seq=%d state_seq=%d, want rows=%d and matching positive sequence", count, seq, stateSeq, wantCount)
		}
		return seq
	}
	first := messageSeq(1)
	mark := func(token string) int64 {
		t.Helper()
		var got int64
		if err := platform.WithScope(ctx, f.runtime, token, f.storeA1, "inbox:read", func(tx pgx.Tx, _ platform.Scope) error {
			return tx.QueryRow(ctx, `SELECT inbox.mark_read($1,$2)`, conv, first).Scan(&got)
		}); err != nil {
			t.Fatal(err)
		}
		return got
	}
	assertUnread := func(want bool) {
		t.Helper()
		l := list()
		found := false
		for _, item := range l.Items {
			if !item.Unread || item.BundleOnly {
				t.Fatal("unread filter returned a non-unread item")
			}
			found = found || item.ConversationID == conv
		}
		wantTotal := baseline
		if want {
			wantTotal++
		}
		if found != want || l.UnreadTotal != wantTotal {
			t.Fatalf("UR02 unread=%t total=%d, want unread=%t total=%d", found, l.UnreadTotal, want, wantTotal)
		}
	}
	if got := mark(reader); got != 0 {
		t.Fatalf("inbox:read-only mark_read=%d, want 0 (unchanged)", got)
	}
	assertUnread(true)
	if got := mark(replier); got != first {
		t.Fatalf("inbox:reply mark_read=%d, want %d", got, first)
	}
	assertUnread(false) // Another staff member observes the store-level clear.
	if next := e.postDM(t, peer, "SYNTHETIC-UR02-NEW-INBOUND", time.Now()); next != conv {
		t.Fatal("new inbound created a different conversation")
	}
	if next := messageSeq(2); next <= first {
		t.Fatalf("new inbound sequence=%d, want > %d", next, first)
	}
	assertUnread(true) // A real committed social.messages INSERT reopens unread through the trigger.
}
