// Purpose: LC-B2-DEL privacy acceptance through the actual poller -> bridge -> A2 service on REAL_PG.
// Depends on: LCN01 synthetic Page-token/lease fixture and MOCK Graph missing-id response; no UI/provider traffic.
// Used by: focused foundation gates; polls a controlled fake until the contract's real60s deletion check.
package foundation_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/integrations/metareply"
	"livecommerce/internal/live"
	"livecommerce/internal/platform"
)

func TestLiveConsoleLCN01DeletionEviction(t *testing.T) {
	e := lcnSetup(t)
	ref, keep := lcnRef(), lcnRef()
	now := time.Now().UTC()
	e.graph.setComments(e.postID, []map[string]any{lcnComment(ref, now.Format(time.RFC3339), "42", "Synthetic deleted author", "synthetic deleted text", "", false), lcnComment(keep, now.Add(time.Second).Format(time.RFC3339), "42", "Synthetic existing author", "synthetic existing text", "", false)})
	c := e.console(t, "worker-deletion", metareply.ConsoleConfig{SweepInterval: 50 * time.Millisecond, PollInterval: 2 * time.Second})
	first := e.demandAndPoll(t, c, e.sourceID)
	if len(first.Items) != 2 {
		t.Fatalf("fixture ring size=%d", len(first.Items))
	}
	srv := httptest.NewServer(c.Handler())
	t.Cleanup(srv.Close)
	bridge, err := metareply.NewBridgeClient(srv.URL, e.bridgeToken)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := live.NewCommentStream(bridge, nil)
	if err != nil {
		t.Fatal(err)
	}
	read := func() live.ConsoleStreamPage {
		t.Helper()
		var page live.ConsoleStreamPage
		err := platform.WithScope(context.Background(), e.f.runtime, e.h.token, e.f.storeA1, "live:read", func(tx pgx.Tx, s platform.Scope) error {
			var err error
			page, err = stream.Comments(context.Background(), tx, s, e.h.token, e.session, live.ConsolePageQuery{Limit: 50})
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		return page
	}
	before := read()
	if len(before.Items) != 2 {
		t.Fatal("A2 fixture did not serve original comments")
	}
	e.graph.deleteRef(ref)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("owned poller did not stop")
		}
	}()
	deadline := time.Now().Add(68 * time.Second)
	removed := false
	for time.Now().Before(deadline) {
		page := read()
		if len(page.Items) == 1 && page.Items[0].Ref == keep {
			removed = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !removed {
		t.Fatal("deleted buyer comment remained in actual A2 after one60s check")
	}
	status, code, after := e.page(t, c.Handler(), e.sourceID, nil, nil, 50)
	if status != http.StatusOK || len(after.Items) != 1 || after.Items[0].Ref != keep || after.Epoch != first.Epoch || after.NextSeq < first.NextSeq {
		t.Fatalf("bridge deletion reset or sequence changed: status=%d code=%s epoch=%d seq=%d", status, code, after.Epoch, after.NextSeq)
	}
	checked := false
	for _, hit := range e.graph.hits() {
		if strings.Contains(hit.target, "ids=") {
			checked = true
			if !hit.bearer || strings.Contains(hit.target, "access_token") || strings.Contains(hit.target, e.pageToken) {
				t.Fatal("batch token transport violated")
			}
		}
	}
	if !checked {
		t.Fatal("no actual batched id check")
	}
}
