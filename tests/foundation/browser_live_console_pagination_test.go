//go:build browser

// Purpose: Phase A regression for the Console MOCK list pagination contract.
// Depends on: the existing consoleMock.serve and standard-library HTTP recorders.
// Used by: TestBrowserLiveConsoleRealChain/pagination-contract; no server or PG setup.
// Invariants: I01 store isolation, I18 honest runtime evidence; list reads preserve all fixture state.
package foundation_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"livecommerce/internal/pagination"
	"livecommerce/internal/platform"
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
	if !mock.serve(recorder, request, store, platform.Scope{TenantID: "00000000-0000-4000-8000-000000000010", StoreID: store}) {
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

	// Phase B extends the original RED assertions above without replacing them.
	scope := platform.Scope{TenantID: "00000000-0000-4000-8000-000000000010", StoreID: store}
	base := "/v1/admin/stores/" + store + "/live-sessions"
	fetch := func(t *testing.T, target string, scopes ...platform.Scope) (int, []string, string) {
		t.Helper()
		recorder := httptest.NewRecorder()
		if !mock.serve(recorder, httptest.NewRequest(http.MethodGet, target, nil), store, scopes...) {
			t.Fatal("collection request unhandled")
		}
		var response struct {
			Items []struct {
				SessionID string `json:"session_id"`
			} `json:"items"`
			NextCursor string `json:"next_cursor"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		ids := []string{}
		for _, item := range response.Items {
			ids = append(ids, item.SessionID)
		}
		return recorder.Code, ids, response.NextCursor
	}
	t.Run("complete_26_with_timestamp_tie_at_page_boundary", func(t *testing.T) {
		if len(page.Items) != 20 || page.NextCursor == "" {
			t.Fatal("first page contract must hold before traversal")
		}
		all := []string{}
		for _, item := range page.Items {
			all = append(all, item.SessionID)
		}
		status, second, cursor := fetch(t, base+"?limit=20&cursor="+page.NextCursor, scope)
		if status != 200 || len(second) != 6 || cursor != "" {
			t.Fatalf("second page status/count/cursor=%d/%d/%q; want 200/6/empty", status, len(second), cursor)
		}
		all = append(all, second...)
		// All original zero-time fixtures share one timestamp. Literal ID order
		// independently pins the strict tie break, including rows20 and21.
		if len(all) != 26 {
			t.Fatalf("complete traversal has %d rows; want26", len(all))
		}
		for i, id := range all {
			want := fmt.Sprintf("00000000-0000-4000-8000-%012d", 126-i)
			if id != want {
				t.Errorf("descending row%d=%s; want%s", i+1, id, want)
			}
		}
		binding := pagination.Binding{TenantID: scope.TenantID, StoreID: store, Collection: "live-sessions"}
		terminal, err := pagination.Encode(binding, []string{"2030-01-01T00:00:00.000000Z", all[len(all)-1]})
		if err != nil {
			t.Fatal(err)
		}
		status, empty, cursor := fetch(t, base+"?limit=20&cursor="+terminal, scope)
		if status != 200 || len(empty) != 0 || cursor != "" {
			t.Errorf("terminal page=%d/%v/%q", status, empty, cursor)
		}
	})
	t.Run("canonical_queries_and_trusted_scope_only", func(t *testing.T) {
		for _, query := range []string{"?", "?limit", "?limit=", "?limit=020", "?limit=0", "?limit=-1", "?limit=101", "?limit=20&limit=20", "?limit=%32%30", "?limit=+20", "?limit=20=", "?limit=20&", "?cursor=", "?cursor=not_a_cursor", "?cursor=abc==", "?tenant_id=" + scope.TenantID, "?offset=20", "?limit=20&unknown=x"} {
			status, _, _ := fetch(t, base+query, scope)
			if status != 422 {
				t.Errorf("query %q status=%d; want422", query, status)
			}
		}
		for _, scopes := range [][]platform.Scope{nil, {scope, scope}, {{TenantID: scope.TenantID, StoreID: foreignStore}}, {{TenantID: "invalid", StoreID: store}}} {
			status, _, _ := fetch(t, base+"?limit=20", scopes...)
			if status != 422 {
				t.Errorf("invalid/missing trusted scope status=%d; want422", status)
			}
		}
		status, _, _ := fetch(t, "/v1/admin/stores/"+foreignStore+"/live-sessions?limit=20", scope)
		if status != 422 {
			t.Errorf("trusted scope did not match route store: status=%d; want422", status)
		}
		for _, query := range []string{"", "?limit=100"} {
			status, ids, cursor := fetch(t, base+query, scope)
			if status != 200 || len(ids) != 26 || cursor != "" {
				t.Errorf("default/max page status/count/cursor=%d/%d/%q", status, len(ids), cursor)
			}
		}
	})
	t.Run("cursor_binds_tenant_store_collection_and_canonical_json", func(t *testing.T) {
		for _, binding := range []pagination.Binding{
			{TenantID: "00000000-0000-4000-8000-000000000011", StoreID: store, Collection: "live-sessions"},
			{TenantID: scope.TenantID, StoreID: foreignStore, Collection: "live-sessions"},
			{TenantID: scope.TenantID, StoreID: store, Collection: "products"},
		} {
			keys := []string{"2030-01-01T00:00:00.000000Z", "00000000-0000-4000-8000-000000000107"}
			if binding.Collection == "products" {
				keys = keys[1:]
			}
			cursor, err := pagination.Encode(binding, keys)
			if err != nil {
				t.Fatal(err)
			}
			status, _, _ := fetch(t, base+"?limit=20&cursor="+cursor, scope)
			if status != 422 {
				t.Errorf("foreign binding %v status=%d; want422", binding, status)
			}
		}
		raw, err := base64.RawURLEncoding.DecodeString(page.NextCursor)
		if err != nil {
			t.Fatal(err)
		}
		noncanonical := base64.RawURLEncoding.EncodeToString(append([]byte(" "), raw...))
		status, _, _ := fetch(t, base+"?limit=20&cursor="+noncanonical, scope)
		if status != 422 {
			t.Errorf("noncanonical cursor status=%d; want422", status)
		}
	})
	t.Run("unavailable_collection_stays_503_without_state_effect", func(t *testing.T) {
		scene := mock.scenes["00000000-0000-4000-8000-000000000101"]
		scene.ListUnavailable = true
		defer func() { scene.ListUnavailable = false }()
		status, _, _ := fetch(t, base+"?limit=20", scope)
		if status != 503 {
			t.Errorf("unavailable collection status=%d; want503", status)
		}
	})
	t.Run("created_time_precedes_id_and_cursor_keeps_microseconds", func(t *testing.T) {
		oldRows := mock.scenes
		defer func() { mock.scenes = oldRows }()
		newest := "00000000-0000-4000-8000-000000000001"
		tied := "00000000-0000-4000-8000-000000000002"
		oldest := "00000000-0000-4000-8000-000000000999"
		at := time.Date(2020, 1, 1, 0, 0, 0, 123456000, time.UTC)
		mock.scenes = map[string]*consoleMockScene{
			newest: {ID: newest, Store: store, CreatedAt: at},
			tied:   {ID: tied, Store: store, CreatedAt: at},
			oldest: {ID: oldest, Store: store, CreatedAt: at.Add(-time.Microsecond)},
		}
		status, first, cursor := fetch(t, base+"?limit=1", scope)
		if status != 200 || len(first) != 1 || first[0] != tied || cursor == "" {
			t.Fatalf("timestamp-tie first page=%d/%v/%q", status, first, cursor)
		}
		binding := pagination.Binding{TenantID: scope.TenantID, StoreID: store, Collection: "live-sessions"}
		_, keys, err := pagination.Decode(pagination.Request{Limit: 1, Cursor: cursor}, binding, 2)
		if err != nil || len(keys) != 2 || keys[0] != "2020-01-01T00:00:00.123456Z" || keys[1] != tied {
			t.Fatalf("cursor must identify last returned microsecond row: keys=%v err=%v", keys, err)
		}
		status, rest, cursor := fetch(t, base+"?limit=20&cursor="+cursor, scope)
		if status != 200 || len(rest) != 2 || rest[0] != newest || rest[1] != oldest || cursor != "" {
			t.Errorf("time primary/id secondary page=%d/%v/%q", status, rest, cursor)
		}
	})
	t.Run("copy_sets_current_time_once_and_replay_keeps_it", func(t *testing.T) {
		source := &consoleMockScene{ID: "00000000-0000-4000-8000-000000000201", Store: store, Title: "MOCK dated source", Phase: "draft", Version: 1, OfferVersion: 1, CreatedAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)}
		isolated := &consoleMock{scenes: map[string]*consoleMockScene{source.ID: source}, responses: map[string]any{}}
		body := map[string]any{"title": "MOCK dated copy", "scheduled_at": nil, "expected_version": 1}
		path := base + "/" + source.ID + "/copy"
		started := time.Now().UTC().Truncate(time.Microsecond)
		first := consoleBalanceRequest(t, isolated, store, http.MethodPost, path, "pagination-copy-once", body, 200)
		copiedID := first["session"].(map[string]any)["session_id"].(string)
		copied := isolated.scenes[copiedID]
		if copied.CreatedAt.Before(started) || copied.CreatedAt.After(time.Now().UTC()) || !copied.CreatedAt.Equal(copied.UpdatedAt) || copied.CreatedAt.Nanosecond()%1000 != 0 {
			t.Errorf("copy time is not one current UTC microsecond observation: %v/%v", copied.CreatedAt, copied.UpdatedAt)
		}
		stamp := copied.CreatedAt
		replay := consoleBalanceRequest(t, isolated, store, http.MethodPost, path, "pagination-copy-once", body, 200)
		firstJSON, _ := json.Marshal(first)
		replayJSON, _ := json.Marshal(replay)
		if !bytes.Equal(firstJSON, replayJSON) || !copied.CreatedAt.Equal(stamp) {
			t.Error("same-key replay changed copied timestamps or response")
		}
		if len(isolated.scenes) != 2 || len(isolated.receipts) != 2 || !isolated.receipts[0].Effect || isolated.receipts[1].Effect {
			t.Error("copy replay duplicated business effect")
		}
		if source.CreatedAt.Year() != 2020 || source.UpdatedAt.Year() != 2020 {
			t.Error("copy changed source timestamps")
		}
	})
}
