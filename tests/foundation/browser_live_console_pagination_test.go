//go:build browser

// Purpose: Phase A regression for the Console MOCK list pagination contract.
// Depends on: the existing consoleMock.serve and standard-library HTTP recorders.
// Used by: TestBrowserLiveConsoleRealChain/pagination-contract; no server or PG setup.
// Invariants: I01 store isolation, I18 honest runtime evidence; list reads preserve all fixture state.
package foundation_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testConsoleMockPaginationContract(t *testing.T) {
	const store = "00000000-0000-4000-8000-000000000001"
	const foreignStore = "00000000-0000-4000-8000-000000000002"
	const foreignID = "00000000-0000-4000-8000-000000000099"
	mock := &consoleMock{
		scenes:    make(map[string]*consoleMockScene),
		responses: map[string]any{"retained-replay": map[string]any{"status": "unchanged"}},
		receipts:  []consoleMockReceipt{{Scene: foreignID, Action: "fixture", Status: 200, Effect: false}},
	}
	for i := 1; i <= 27; i++ {
		id := fmt.Sprintf("00000000-0000-4000-8000-%012d", 100+i)
		sceneStore := store
		if i == 27 {
			id, sceneStore = foreignID, foreignStore
		}
		mock.scenes[id] = &consoleMockScene{
			ID: id, Store: sceneStore, Title: fmt.Sprintf("MOCK pagination scene %02d", i),
			Phase: "draft", Offer: fmt.Sprintf("00000000-0000-4000-8000-%012d", 200+i),
			SKU:       fmt.Sprintf("00000000-0000-4000-8000-%012d", 300+i),
			Warehouse: fmt.Sprintf("00000000-0000-4000-8000-%012d", 400+i), Platform: "facebook",
			Version: 1, OfferVersion: 1, Stock: int64(i), StockVersion: 1,
			Generation: 1, Active: true, Recommended: map[string]any{"fixture": i}, Reads: []int64{int64(i)},
		}
	}
	snapshot := func() []byte {
		t.Helper()
		state, err := json.Marshal(struct {
			Scenes       map[string]*consoleMockScene
			Receipts     []consoleMockReceipt
			Responses    map[string]any
			BadAuthority int
		}{mock.scenes, mock.receipts, mock.responses, mock.badAuthority})
		if err != nil {
			t.Fatal(err)
		}
		return state
	}
	before := snapshot()
	defer func() {
		if after := snapshot(); !bytes.Equal(before, after) {
			t.Error("list GET mutated fixture scenes, receipts, replay responses, or authority counter")
		}
		sameStore, foreign := 0, 0
		for _, scene := range mock.scenes {
			if scene.Store == store {
				sameStore++
			}
			if scene.Store == foreignStore {
				foreign++
			}
		}
		if sameStore != 26 || foreign != 1 || len(mock.scenes) != 27 {
			t.Errorf("list GET must retain all rows: same-store=%d foreign=%d total=%d; want 26/1/27", sameStore, foreign, len(mock.scenes))
		}
	}()

	// Exercise the existing handler directly. This request does not start HTTP,
	// OIDC, PostgreSQL, Next, a browser, or a replacement pagination algorithm.
	request := httptest.NewRequest(http.MethodGet, "/v1/admin/stores/"+store+"/live-sessions?limit=20", nil)
	recorder := httptest.NewRecorder()
	if !mock.serve(recorder, request, store) {
		t.Fatal("existing Console MOCK handler did not handle collection GET")
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("collection GET status=%d; want 200", recorder.Code)
	}
	var page struct {
		Items []struct {
			SessionID string `json:"session_id"`
		} `json:"items"`
		NextCursor string `json:"next_cursor"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode collection page: %v", err)
	}
	// Keep both assertions nonfatal so the unchanged handler exposes both
	// failures (26 rows and an empty cursor), rather than masking either one.
	if len(page.Items) != 20 {
		t.Errorf("limit=20 returned %d items; want exactly 20", len(page.Items))
	}
	if page.NextCursor == "" {
		t.Error("26 same-store rows require a nonempty next_cursor after the first 20")
	}
	seen := make(map[string]bool)
	for _, item := range page.Items {
		scene := mock.scenes[item.SessionID]
		if item.SessionID == foreignID || scene == nil || scene.Store != store {
			t.Errorf("collection returned foreign or unknown session %q", item.SessionID)
		}
		if seen[item.SessionID] {
			t.Errorf("collection duplicated session %q", item.SessionID)
		}
		seen[item.SessionID] = true
	}
}
