package foundation_test

// Purpose: OC-11 REAL_PG RED oracles for frozen storefront-v2 R1/R2 publish receipts.
// Depends on: the foundation PG fixture, httpapi, design, and ops.command_results.
// Used by: the foundation Go suite on GitHub CI; no local execution is authorized.
// Invariants: identical retries return the committed answer, never another version;
// malformed keys and changed payloads cannot write a version or a receipt.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"livecommerce/internal/design"
	"livecommerce/internal/httpapi"
)

func TestStoreDesignOC11PublishReceipt(t *testing.T) {
	f := fixture(t)
	sdGrant(t, f, f.tokens["a"], f.storeA1)
	h := httpapi.NewHandler(f.runtime)
	base := "/v1/admin/stores/" + f.storeA1 + "/design"
	call := func(method, path, body string, keys ...string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, base+path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+f.tokens["a"])
		r.Header.Set("Content-Type", "application/json")
		for _, key := range keys {
			r.Header.Add("Idempotency-Key", key)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	counts := func() (versions, receipts int) {
		t.Helper()
		err := f.owner.QueryRow(context.Background(), `SELECT
			(SELECT count(*) FROM design.published_versions WHERE store_id=$1),
			(SELECT count(*) FROM ops.command_results WHERE store_id=$1 AND operation='design.publish')`, f.storeA1).Scan(&versions, &receipts)
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	reset := func(t *testing.T) {
		t.Helper()
		sdgReset(t, f)
		w := call("PUT", "/draft", `{"expected_version":0,"document":{"profile":{"name":"OC11 synthetic store","accent_color":"#247965"}}}`)
		if w.Code != http.StatusOK {
			t.Fatalf("prepare draft: %d %s", w.Code, w.Body.String())
		}
	}
	t.Cleanup(func() { sdgReset(t, f) })
	const body = `{"expected_draft_version":1}`

	t.Run("same_key_replays_first_answer_after_draft_changes", func(t *testing.T) {
		reset(t)
		_, before := counts()
		first := call("POST", "/publish", body, "oc11-receipt-replay")
		if first.Code != http.StatusOK {
			t.Fatalf("first publish: %d %s", first.Code, first.Body.String())
		}
		var original design.VersionInfo
		if err := json.Unmarshal(first.Body.Bytes(), &original); err != nil || original.Version != 1 {
			t.Fatalf("invalid first answer: %s", first.Body.String())
		}
		// Fixture write through the real draft handler: recovery must retain the
		// original version even when the merchant has edited the draft meanwhile.
		saved := call("PUT", "/draft", `{"expected_version":1,"document":{"profile":{"name":"OC11 changed draft","accent_color":"#247965"}}}`)
		if saved.Code != http.StatusOK {
			t.Fatalf("change draft: %d %s", saved.Code, saved.Body.String())
		}
		replay := call("POST", "/publish", body, "oc11-receipt-replay")
		if replay.Code != http.StatusOK || replay.Body.String() != first.Body.String() {
			t.Errorf("R2 identical retry must replay first answer: first=%d %s replay=%d %s", first.Code, first.Body.String(), replay.Code, replay.Body.String())
		}
		if versions, receipts := counts(); versions != 1 || receipts != before+1 {
			t.Errorf("R1 atomic receipt/version: versions=%d receipts delta=%d; want 1/1", versions, receipts-before)
		}
	})

	t.Run("other_body_is_idempotency_conflict", func(t *testing.T) {
		reset(t)
		_, before := counts()
		if first := call("POST", "/publish", body, "oc11-receipt-conflict"); first.Code != http.StatusOK {
			t.Fatalf("first publish: %d %s", first.Code, first.Body.String())
		}
		w := call("POST", "/publish", `{"expected_draft_version":2}`, "oc11-receipt-conflict")
		var env struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil || w.Code != 409 || env.Code != "idempotency_conflict" {
			t.Errorf("R2 changed body must return 409 idempotency_conflict: %d %s", w.Code, w.Body.String())
		}
		if versions, receipts := counts(); versions != 1 || receipts != before+1 {
			t.Errorf("conflicting body wrote state: versions=%d receipts delta=%d", versions, receipts-before)
		}
	})

	for _, tc := range []struct {
		name string
		keys []string
	}{
		{"missing_key", nil},
		{"duplicate_key", []string{"oc11-duplicate-key", "oc11-duplicate-key"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reset(t)
			_, before := counts()
			w := call("POST", "/publish", body, tc.keys...)
			var env struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil || w.Code != 422 || env.Code != "invalid_request" {
				t.Errorf("R1 key must occur exactly once: %d %s", w.Code, w.Body.String())
			}
			if versions, receipts := counts(); versions != 0 || receipts != before {
				t.Errorf("invalid key wrote state: versions=%d receipts delta=%d", versions, receipts-before)
			}
		})
	}

	t.Run("concurrent_same_key_returns_one_committed_answer", func(t *testing.T) {
		reset(t)
		_, before := counts()
		const n = 4
		answers := make([]*httptest.ResponseRecorder, n)
		start := make(chan struct{})
		var ready, done sync.WaitGroup
		ready.Add(n)
		done.Add(n)
		for i := range answers {
			go func(i int) {
				defer done.Done()
				ready.Done()
				<-start
				answers[i] = call("POST", "/publish", body, "oc11-receipt-concurrent")
			}(i)
		}
		ready.Wait()
		close(start)
		done.Wait()
		for i, w := range answers {
			if w.Code != 200 || w.Body.String() != answers[0].Body.String() {
				t.Errorf("R2 concurrent retry %d: %d %s; first=%d %s", i, w.Code, w.Body.String(), answers[0].Code, answers[0].Body.String())
			}
		}
		if versions, receipts := counts(); versions != 1 || receipts != before+1 {
			t.Errorf("R2 concurrent retries: versions=%d receipts delta=%d; want 1/1", versions, receipts-before)
		}
	})
}
