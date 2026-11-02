//go:build browser

// Purpose: pin the Console MOCK inventory contract across copied sessions and warehouse boundaries.
// Depends on: the existing consoleMock HTTP handler, net/http/httptest; no server, PG, or provider.
// Used by: the --browser-live-console gate; these are MOCK contract checks, not UI or inventory SQL acceptance.
// Invariants: I02/I03; session identity must not fork a warehouse/SKU balance or duplicate a command effect.
package foundation_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestBrowserLiveConsoleMockBalance(t *testing.T) {
	newMock := func() (*consoleMock, *consoleMockScene) {
		s := &consoleMockScene{ID: "balance-original", Store: "balance-store", Title: "MOCK balance source", Phase: "draft", Version: 1, Offer: "balance-offer", SKU: "balance-sku", Warehouse: "balance-warehouse", OfferVersion: 1, Stock: 12, StockVersion: 1, Active: true}
		return &consoleMock{scenes: map[string]*consoleMockScene{s.ID: s}, responses: map[string]any{}}, s
	}
	adjustment := func(s *consoleMockScene, delta, version int64) map[string]any {
		return map[string]any{"sku_id": s.SKU, "warehouse_id": s.Warehouse, "delta": delta, "expected_version": version, "reason": "live_console_edit"}
	}
	adjust := func(t *testing.T, m *consoleMock, s *consoleMockScene, key string, body map[string]any, status int) map[string]any {
		t.Helper()
		return consoleBalanceRequest(t, m, s.Store, http.MethodPost, "/v1/admin/stores/"+s.Store+"/inventory/adjustments", key, body, status)
	}

	t.Run("copy_reads_one_shared_balance_and_version", func(t *testing.T) {
		m, s := newMock()
		out := consoleBalanceRequest(t, m, s.Store, http.MethodPost, "/v1/admin/stores/"+s.Store+"/live-sessions/"+s.ID+"/copy", "balance-copy", map[string]any{"title": "MOCK balance copy", "scheduled_at": nil, "expected_version": 1}, http.StatusOK)
		session, ok := out["session"].(map[string]any)
		if !ok {
			t.Fatalf("copy missing session: %v", out)
		}
		copyID, ok := session["session_id"].(string)
		if !ok || copyID == "" || copyID == s.ID {
			t.Fatalf("copy must return a new session: %v", session)
		}
		for _, id := range []string{s.ID, copyID} {
			consoleBalanceWant(t, m, s.Store, id, s.SKU, s.Warehouse, 12, 1)
		}
		adjust(t, m, s, "balance-shared-adjust", adjustment(s, 5, 1), http.StatusOK)
		// Check BOTH HTTP projections. Whichever scene a Go map visits first, the
		// old per-scene fixture leaves one at 12/v1 and cannot satisfy this oracle.
		for _, id := range []string{s.ID, copyID} {
			consoleBalanceWant(t, m, s.Store, id, s.SKU, s.Warehouse, 17, 2)
		}
		consoleBalanceWantEffects(t, m, 1)
	})

	t.Run("same_sku_different_warehouses_stay_isolated", func(t *testing.T) {
		m, s := newMock()
		other := *s // FIXTURE: an independent warehouse of the same SKU, not a session copy.
		other.ID, other.Offer, other.Warehouse = "balance-other-session", "balance-other-offer", "balance-other-warehouse"
		other.Stock, other.StockVersion = 40, 7
		m.scenes[other.ID] = &other
		// Each command is identified by its warehouse, never a map-selected scene.
		out := adjust(t, m, &other, "balance-other-adjust", adjustment(&other, 3, 7), http.StatusOK)
		if out["warehouse_id"] != other.Warehouse {
			t.Errorf("wrong warehouse receipt: %v", out)
		}
		consoleBalanceWant(t, m, s.Store, s.ID, s.SKU, s.Warehouse, 12, 1)
		consoleBalanceWant(t, m, s.Store, other.ID, other.SKU, other.Warehouse, 43, 8)
		adjust(t, m, s, "balance-first-adjust", adjustment(s, 5, 1), http.StatusOK)
		consoleBalanceWant(t, m, s.Store, s.ID, s.SKU, s.Warehouse, 17, 2)
		consoleBalanceWant(t, m, s.Store, other.ID, other.SKU, other.Warehouse, 43, 8)
	})

	t.Run("stale_balance_version_has_no_effect", func(t *testing.T) {
		m, s := newMock()
		adjust(t, m, s, "balance-cas-first", adjustment(s, 5, 1), http.StatusOK)
		out := adjust(t, m, s, "balance-cas-stale", adjustment(s, 3, 1), http.StatusConflict)
		if out["code"] != "version_conflict" {
			t.Errorf("stale CAS code: %v", out)
		}
		consoleBalanceWant(t, m, s.Store, s.ID, s.SKU, s.Warehouse, 17, 2)
		consoleBalanceWantEffects(t, m, 1)
	})

	t.Run("same_key_same_body_replays_without_second_effect", func(t *testing.T) {
		m, s := newMock()
		body := adjustment(s, 5, 1)
		first := adjust(t, m, s, "balance-replay", body, http.StatusOK)
		replay := adjust(t, m, s, "balance-replay", body, http.StatusOK)
		if !reflect.DeepEqual(first, replay) {
			t.Errorf("replay changed receipt: first=%v replay=%v", first, replay)
		}
		consoleBalanceWant(t, m, s.Store, s.ID, s.SKU, s.Warehouse, 17, 2)
		consoleBalanceWantEffects(t, m, 1)
	})
}

func consoleBalanceRequest(t *testing.T, m *consoleMock, store, method, path, key string, body map[string]any, wantStatus int) map[string]any {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(raw))
	r.Header.Set("Content-Type", "application/json")
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	w := httptest.NewRecorder()
	if !m.serve(w, r, store) {
		t.Fatalf("MOCK did not handle %s %s", method, path)
	}
	if w.Code != wantStatus {
		t.Fatalf("%s %s: status=%d want=%d body=%s", method, path, w.Code, wantStatus, w.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid MOCK response: %v", err)
	}
	return out
}

func consoleBalanceWant(t *testing.T, m *consoleMock, store, session, sku, warehouse string, sellable, version int64) {
	t.Helper()
	out := consoleBalanceRequest(t, m, store, http.MethodGet, "/v1/admin/stores/"+store+"/live-sessions/"+session+"/console", "", nil, http.StatusOK)
	offers, ok := out["offers"].([]any)
	if !ok || len(offers) != 1 {
		t.Fatalf("session %s missing one offer: %v", session, out)
	}
	offer, ok := offers[0].(map[string]any)
	if !ok || offer["sku_id"] != sku {
		t.Fatalf("session %s wrong SKU: %v", session, offers)
	}
	stock, ok := offer["stock"].(map[string]any)
	if !ok {
		t.Fatalf("session %s missing stock: %v", session, offer)
	}
	if stock["warehouse_id"] != warehouse || stock["sellable"] != float64(sellable) || stock["balance_version"] != float64(version) {
		t.Errorf("session %s balance=%v want warehouse=%s sellable=%d version=%d", session, stock, warehouse, sellable, version)
	}
}

func consoleBalanceWantEffects(t *testing.T, m *consoleMock, want int) {
	t.Helper()
	effects := 0
	for _, receipt := range m.receipts {
		if receipt.Action == "adjustments" && receipt.Effect {
			effects++
		}
	}
	if effects != want {
		t.Errorf("adjustment effects=%d want=%d", effects, want)
	}
}
