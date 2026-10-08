// Purpose: exercise A9's nullable UNKNOWN authority through the real merchant router and PostgreSQL, independently of display pagination.
// Depends on: lbSetup signed-webhook fixture, lcConversation, httpapi.NewHandler, inbox.read_outbound and social.read_thread (live-console-v1 §11 A9).
// Used by: test-focused.sh '^TestInboxUnknownAuthority$', foundation CI.
// Invariants: I01 (server-scoped reads), I11 (synthetic private bodies), UNKNOWN never authorizes another send; truncation fails closed.
// Status: MOCK (REAL_PG, loopback fake Graph; owner SQL below only seeds synthetic fixture rows).
package foundation_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"livecommerce/internal/httpapi"
)

// TestInboxUnknownAuthority proves the response fact survives inbound floors, outbound truncation and older-page reads.
func TestInboxUnknownAuthority(t *testing.T) {
	e := lbSetup(t)
	f := e.h.f
	handler := httpapi.NewHandler(f.runtime, httpapi.Options{Inbox: e.svc})
	base := time.Now().Add(-2 * time.Hour).UTC()
	newConversation := func(t *testing.T) string {
		conv := lcConversation(t, f, f.tenantA, f.storeA1, "page")
		mustExec(t, f.owner, `UPDATE inbox.conversation_state SET last_inbound_at=$2 WHERE conversation_id=$1`, conv, base)
		return conv
	}
	// Owner-only writes create display copies, including deliberately unreadable ciphertext. Authority must use operation state,
	// before opening that display copy. No producer, queue or provider action is invoked by these fixture inserts.
	seedOutbound := func(t *testing.T, conv, state string, at time.Time) {
		t.Helper()
		op := randomUUID()
		if state != "missing" {
			mustExec(t, f.owner, `INSERT INTO integration.operations(id,tenant_id,store_id,principal_id,binding_id,binding_version,
				provider,external_asset_id,purpose,action,semantic_key,request_hash,request,job_id,state,generation)
				VALUES($1,$2,$3,$4,$5,1,'mock','synthetic-authority','service','mock.authority',($1::uuid)::text,
				decode(repeat('01',32),'hex'),'{}',1,$6,1)`, op, f.tenantA, f.storeA1, e.h.actor, e.pageBinding, state)
		}
		mustExec(t, f.owner, `INSERT INTO inbox.outbound_messages(tenant_id,store_id,id,conversation_id,kind,operation_id,principal_id,
			key_id,nonce,ciphertext,body_hmac,created_at) VALUES($1,$2,$3,$4,'dm',$5,$6,'synthetic-authority',
			decode(repeat('01',12),'hex'),decode(repeat('01',17),'hex'),decode(repeat('01',32),'hex'),$7)`,
			f.tenantA, f.storeA1, randomUUID(), conv, op, e.h.actor, at)
	}
	get := func(t *testing.T, conv, query string) (int, map[string]json.RawMessage) {
		t.Helper()
		r := httptest.NewRequest(http.MethodGet, "/v1/admin/stores/"+f.storeA1+"/inbox/conversations/"+conv+"/messages"+query, nil)
		r.Header.Set("Authorization", "Bearer "+e.h.token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		var body map[string]json.RawMessage
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal("invalid A9 JSON", err)
		}
		if w.Code == http.StatusOK && w.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal("private A9 response must use no-store")
		}
		return w.Code, body
	}
	check := func(t *testing.T, conv, query, want string, maxOutbound int) {
		t.Helper()
		status, body := get(t, conv, query)
		if status != http.StatusOK {
			t.Fatalf("A9 status=%d, want 200", status)
		}
		// Dynamic JSON lookup keeps the red test compiling before the additive field exists.
		if got, exists := body["has_unknown_outbound"]; !exists || string(got) != want {
			t.Errorf("has_unknown_outbound=%s (exists=%v), want %s", got, exists, want)
		}
		var items []struct {
			Direction string `json:"direction"`
		}
		if err := json.Unmarshal(body["items"], &items); err != nil {
			t.Fatal(err)
		}
		outbound := 0
		for _, item := range items {
			if item.Direction == "out" {
				outbound++
			}
		}
		if outbound > maxOutbound {
			t.Fatalf("display outbound=%d exceeds preserved limit=%d", outbound, maxOutbound)
		}
	}
	for _, tc := range []struct {
		name        string
		count       int
		state, want string
	}{
		{"empty", 0, "SUCCEEDED", "false"},
		{"exhaustive_healthy", 49, "SUCCEEDED", "false"},
		{"known_unknown", 1, "UNKNOWN", "true"},
		{"missing_operation", 1, "missing", "null"},
		{"exactly_50_healthy", 50, "SUCCEEDED", "null"},
		{"unknown_in_full_scan", 50, "UNKNOWN", "true"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conv := newConversation(t)
			for i := 0; i < tc.count; i++ {
				seedOutbound(t, conv, tc.state, base.Add(time.Duration(i)*time.Second))
			}
			check(t, conv, "?limit=1", tc.want, 1)
			check(t, conv, "?before_seq=1&limit=1", tc.want, 0)
		})
	}
	t.Run("old_unknown_outside_50_newer_outbound", func(t *testing.T) {
		conv := newConversation(t)
		seedOutbound(t, conv, "UNKNOWN", base)
		for i := 1; i <= 51; i++ {
			seedOutbound(t, conv, "SUCCEEDED", base.Add(time.Duration(i)*time.Second))
		}
		check(t, conv, "?limit=50", "null", 50)
		check(t, conv, "?before_seq=1&limit=50", "null", 0)
	})
	t.Run("known_unknown_below_50_newer_inbound", func(t *testing.T) {
		psid := mciDigits(15)
		var conv string
		for i := 0; i < 51; i++ {
			conv = e.postDM(t, psid, fmt.Sprintf("synthetic inbound %d", i), base.Add(time.Hour+time.Duration(i)*time.Second))
		}
		seedOutbound(t, conv, "UNKNOWN", base)
		check(t, conv, "?limit=50", "true", 0)
		check(t, conv, "?before_seq=51&limit=50", "true", 0)
	})
	t.Run("scope_not_found", func(t *testing.T) {
		for _, conv := range []string{
			lcConversation(t, f, f.tenantA, f.storeA2, "page"),
			lcConversation(t, f, f.tenantB, f.storeB, "page"),
		} {
			status, body := get(t, conv, "")
			if status != http.StatusNotFound {
				t.Fatalf("cross-scope A9 status=%d, want 404", status)
			}
			if _, leaked := body["has_unknown_outbound"]; leaked {
				t.Fatal("cross-scope authority leaked")
			}
		}
	})
	// Cleanup is lbSetup/fixture-owned; no shared-machine processes are started by this test.
}
