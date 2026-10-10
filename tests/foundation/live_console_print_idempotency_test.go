// live_console_print_idempotency_test.go — LC-A3 fix gate: the A3 print route must honour its own
// Idempotency-Key (live-console-v1 §7.4 "idempotent per key", invariants I02). The route always REQUIRED a
// well-formed key (claims.go keyRequired), but internal/live/stream.go PrintComment called live.comment_print
// directly with no command.Run receipt, so a same-key retry incremented print_count again (observed 7→8 under
// REAL_PG). This gate drives the real HTTP route against real PG: same key → the stored receipt (identical
// body, no new increment); a new key → +1; the same key with a different canonical {session_id, comment_ref}
// → 409; cross-store/cross-session attempts never replay or move the store-A1 receipt; and neither the
// receipt nor any other base table ever stores the label text (the request survives only as a sha256 hash).
//
// Owns: the A3 per-key idempotency gate. Setup is the LCN05 harness (lcnSetup): a fresh principal with
// live:* on stores A1+A2, one OPEN-window session with an active Facebook Page source, the fake loopback
// Graph and the claims-worker console as the bridge. The owner (superuser) pool is used only for synthetic
// read-back (live.comment_prints, ops.command_results) and cleanup.
//
// Isolation: serial with the other live gates (store-wide one-OPEN-window rule); comment_prints rows for the
// second session are deleted before lcPurgeSessions (no ON DELETE CASCADE). Evidence label: REAL_PG, MOCK
// Graph (loopback).
package foundation_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/httpapi"
	"livecommerce/internal/integrations/metareply"
	"livecommerce/internal/live"
)

func TestLiveConsoleLCN05PrintIdempotencyKey(t *testing.T) {
	e := lcnSetup(t)
	now := time.Now().UTC()
	ref, ref2 := lcnRef(), lcnRef()
	text := "LCN05-KEY-SENTINEL-" + t04Tag()
	e.graph.setComments(e.postID, []map[string]any{
		lcnComment(ref, now.Format(time.RFC3339), e.asset, "", text, "", false),
	})
	c := e.console(t, "worker-printkey", metareply.ConsoleConfig{})
	bridgeSrv := httptest.NewServer(c.Handler())
	t.Cleanup(bridgeSrv.Close)
	bridge, err := metareply.NewBridgeClient(bridgeSrv.URL, e.bridgeToken)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := live.NewCommentStream(bridge, nil)
	if err != nil {
		t.Fatal(err)
	}
	page := e.demandAndPoll(t, c, e.sourceID)
	if len(page.Items) != 1 || page.Items[0].Ref != ref || page.Items[0].Text != text {
		t.Fatalf("polled %+v", page.Items)
	}
	api := httpapi.NewHandler(e.f.runtime, httpapi.Options{CommentStream: stream})

	t.Cleanup(func() { // receipts are not purged by lcPurgeSessions; siblings delete theirs by principal+operation
		mustExec(t, e.f.owner, `DELETE FROM ops.command_results WHERE principal_id=$1 AND operation='live.comment.print'`, e.h.actor)
	})
	a3 := func(store, session, r, key string) (int, string, live.CommentPrint) {
		t.Helper()
		w := adminRequest(api, http.MethodPost,
			"/v1/admin/stores/"+store+"/live-sessions/"+session+"/comments/"+r+"/print",
			e.h.token, []byte("{}"), "application/json", map[string]string{"Idempotency-Key": key})
		var out live.CommentPrint
		if w.Code == http.StatusOK {
			if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
				t.Fatalf("decode print response %s: %v", w.Body.String(), err)
			}
		}
		return w.Code, w.Body.String(), out
	}
	errCode := func(raw string) string {
		var envelope struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal([]byte(raw), &envelope)
		return envelope.Code
	}
	fact := func(store, session, r string) int64 {
		t.Helper()
		var count int64
		err := e.f.owner.QueryRow(context.Background(), `SELECT print_count FROM live.comment_prints
			WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3 AND comment_ref=$4`,
			e.f.tenantA, store, session, r).Scan(&count)
		if errors.Is(err, pgx.ErrNoRows) {
			return 0
		}
		if err != nil {
			t.Fatalf("read print fact: %v", err)
		}
		return count
	}

	// The A2 read serves the sentinel text from the worker's in-memory buffer, so the lcFind scan below is
	// meaningful: the text is live in the system while the print receipts are written.
	a2 := httptest.NewRequest(http.MethodGet,
		"/v1/admin/stores/"+e.f.storeA1+"/live-sessions/"+e.session+"/comments?limit=50", nil)
	a2.Header.Set("Authorization", "Bearer "+e.h.token)
	w := httptest.NewRecorder()
	api.ServeHTTP(w, a2)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), text) {
		t.Fatalf("A2 read status=%d body=%s", w.Code, w.Body.String())
	}

	key1, key2 := t04Key("lcn05a3-a"), t04Key("lcn05a3-b")

	// First print under key1: 200, count 1.
	st, raw1, p1 := a3(e.f.storeA1, e.session, ref, key1)
	if st != http.StatusOK {
		t.Fatalf("first print status=%d body=%s", st, raw1)
	}
	if p1.PrintCount != 1 || p1.LastPrintedAt == nil {
		t.Fatalf("first print %+v (body %s)", p1, raw1)
	}

	// Same key replay: the stored receipt — byte-identical body, print_count NOT incremented (§7.4).
	st, raw2, p2 := a3(e.f.storeA1, e.session, ref, key1)
	if st != http.StatusOK {
		t.Fatalf("replay status=%d body=%s", st, raw2)
	}
	if p2.PrintCount != 1 {
		t.Errorf("same-key replay returned print_count=%d, want the stored receipt's 1", p2.PrintCount)
	}
	if raw2 != raw1 {
		t.Errorf("same-key replay changed the response:\n first: %s\nreplay: %s", raw1, raw2)
	}
	if n := fact(e.f.storeA1, e.session, ref); n != 1 {
		t.Errorf("print fact after same-key replay = %d, want 1 (the replay must not print again)", n)
	}

	// A NEW key prints again: +1.
	st, raw3, p3 := a3(e.f.storeA1, e.session, ref, key2)
	if st != http.StatusOK {
		t.Fatalf("new-key print status=%d body=%s", st, raw3)
	}
	if p3.PrintCount != 2 {
		t.Errorf("new-key print returned print_count=%d, want 2", p3.PrintCount)
	}
	if n := fact(e.f.storeA1, e.session, ref); n != 2 {
		t.Errorf("print fact after new key = %d, want 2", n)
	}

	// Same key, different comment_ref: a different canonical request under a used key → 409, nothing printed.
	st, raw4, _ := a3(e.f.storeA1, e.session, ref2, key1)
	if st != http.StatusConflict || errCode(raw4) != "conflict" {
		t.Errorf("same key + other ref: status=%d body=%s, want 409 conflict", st, raw4)
	}
	if n := fact(e.f.storeA1, e.session, ref2); n != 0 {
		t.Errorf("conflicting comment_ref printed anyway: print_count=%d", n)
	}

	// Same key, different session (a fresh draft of the same store): 409, nothing stored for it.
	session2 := e.h.draft(t, e.f.storeA1)
	t.Cleanup(func() { // comment_prints has no ON DELETE CASCADE: clean before lcPurgeSessions (LIFO).
		mustExec(t, e.f.owner, `DELETE FROM live.comment_prints WHERE session_id=$1`, session2)
	})
	st, raw5, _ := a3(e.f.storeA1, session2, ref, key1)
	if st != http.StatusConflict || errCode(raw5) != "conflict" {
		t.Errorf("same key + other session: status=%d body=%s, want 409 conflict", st, raw5)
	}
	if n := fact(e.f.storeA1, session2, ref); n != 0 {
		t.Errorf("conflicting session printed anyway: print_count=%d", n)
	}

	// Cross-store: the store-A2 scope holds no receipt for key1 and the session is foreign there → 404,
	// never a replay of the store-A1 receipt, never a move of its fact.
	st, raw6, _ := a3(e.f.storeA2, e.session, ref, key1)
	if st != http.StatusNotFound || errCode(raw6) != "not_found" {
		t.Errorf("cross-store print: status=%d body=%s, want 404 not_found", st, raw6)
	}
	if n := fact(e.f.storeA1, e.session, ref); n != 2 {
		t.Errorf("cross-store attempt moved the store-A1 print fact to %d, want 2", n)
	}

	// Same key + same request from another principal with live:manage on the store: 409, never a replay
	// of the first principal's receipt (the request hash binds principal_id, like live.draft.*; K3 PR30).
	other, otherToken := lcPrincipal(t, e.f, e.f.tenantA, []string{e.f.storeA1}, "store:read", "live:read", "live:manage")
	t.Cleanup(func() {
		mustExec(t, e.f.owner, `DELETE FROM ops.command_results WHERE principal_id=$1 AND operation='live.comment.print'`, other)
	})
	wOther := adminRequest(api, http.MethodPost, "/v1/admin/stores/"+e.f.storeA1+"/live-sessions/"+e.session+"/comments/"+ref+"/print",
		otherToken, []byte("{}"), "application/json", map[string]string{"Idempotency-Key": key1})
	if wOther.Code != http.StatusConflict || errCode(wOther.Body.String()) != "conflict" {
		t.Errorf("same key + other principal: status=%d body=%s, want 409 conflict", wOther.Code, wOther.Body.String())
	}
	if n := fact(e.f.storeA1, e.session, ref); n != 2 {
		t.Errorf("other-principal attempt moved the print fact to %d, want 2", n)
	}

	// After every negative the store-A1 receipt still replays byte-identically.
	st, raw7, _ := a3(e.f.storeA1, e.session, ref, key1)
	if st != http.StatusOK || raw7 != raw1 {
		t.Errorf("post-negative replay: status=%d body=%s, want the original receipt %s", st, raw7, raw1)
	}

	// Receipts: exactly key1+key2 under this store and operation (the query is key-scoped because the
	// sibling LCN05 gate prints on the same shared store fixture); the stored response is only the fact
	// (print_count, last_printed_at) — session id, comment ref and label text survive, if at all, as the
	// sha256 request hash. The 409/404 negatives reused key1 with other requests and must have added
	// nothing.
	rows, err := e.f.owner.Query(context.Background(), `SELECT idempotency_key,response::text FROM ops.command_results
		WHERE tenant_id=$1 AND store_id=$2 AND operation='live.comment.print' AND idempotency_key IN ($3,$4)`,
		e.f.tenantA, e.f.storeA1, key1, key2)
	if err != nil {
		t.Fatalf("read receipts: %v", err)
	}
	defer rows.Close()
	seen := map[string]string{}
	for rows.Next() {
		var key, response string
		if err := rows.Scan(&key, &response); err != nil {
			t.Fatalf("scan receipt: %v", err)
		}
		seen[key] = response
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("receipt rows: %v", err)
	}
	if len(seen) != 2 || seen[key1] == "" || seen[key2] == "" {
		t.Errorf("receipts = %v, want exactly one row each for key1 and key2", seen)
	}
	for key, response := range seen {
		var fields map[string]any
		if err := json.Unmarshal([]byte(response), &fields); err != nil {
			t.Errorf("receipt %s is not JSON: %v (%s)", key, err, response)
			continue
		}
		if len(fields) != 2 || fields["print_count"] == nil || fields["last_printed_at"] == nil {
			t.Errorf("receipt %s stores %v, want only print_count+last_printed_at", key, fields)
		}
		if strings.Contains(response, ref) || strings.Contains(response, ref2) ||
			strings.Contains(response, e.session) || strings.Contains(response, session2) ||
			strings.Contains(response, text) {
			t.Errorf("receipt %s leaks request values: %s", key, response)
		}
	}

	// The label text exists only in memory and the HTTP responses: no base table (ops.command_results
	// included — lcFind scans every non-system schema) persists it.
	if hits := lcFind(t, e.f, text, nil); len(hits) != 0 {
		t.Errorf("label text persisted in %v", hits)
	}
}
