//go:build browser

// Purpose: test-only MOCK W3-U2 business API with immutable command receipts and bounded fault controls.
// Depends on: Go httptest caller, shared console JSON/hash helpers and actual W3 API shapes.
// Used by: TestBrowserLiveSettingsUIRealChain; real signed authorization is checked before this backend.
// Invariants: I02/I06/I11/I14; private notes stay transient and only command hashes enter evidence.
package foundation_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

type blsReceipt struct {
	Action   string `json:"action"`
	KeyHash  string `json:"key_hash"`
	BodyHash string `json:"body_hash"`
	Status   int    `json:"status"`
	Effect   bool   `json:"effect"`
}

// TestBrowserLiveSettingsMockRequiresMerchantTemplate guards the test backend against accepting a fixed template PUT.
func TestBrowserLiveSettingsMockRequiresMerchantTemplate(t *testing.T) {
	m := &blsMock{store: randomUUID(), scene: randomUUID()}
	m.reset()
	request := httptest.NewRequest(http.MethodPut, "/v1/admin/stores/"+m.store+"/live-settings/sold-out-reply", strings.NewReader(`{"enabled":false,"template_id":"sold-out-reply/v1","template_version":1,"expected_version":3}`))
	request.Header.Set("Idempotency-Key", "fixture-fixed-template-negative")
	response := httptest.NewRecorder()
	if !m.serve(response, request, m.store) {
		t.Fatal("fixture route not handled")
	}
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("fixed template status=%d want422", response.Code)
	}
	if m.version != 3 || !m.enabled || m.templateID != "sold-out-reply/v1" {
		t.Fatal("rejected fixed template changed settings")
	}
	if len(m.receipts) != 1 || m.receipts[0].Effect {
		t.Fatal("fixed-template refusal recorded an effect")
	}
}

type blsMock struct {
	mu                                                                                        sync.Mutex
	store, otherStore, scene, otherScene, bundle, restrictedBundle, entry, nextEntry, comment string
	enabled                                                                                   bool
	version, templateVersion                                                                  int
	templateID                                                                                string
	publishedID                                                                               string
	templateBody                                                                              string
	note                                                                                      string
	removed, blocked, triggered                                                               bool
	receipts                                                                                  []blsReceipt
	results                                                                                   map[string]any
	fault                                                                                     string
	hold                                                                                      chan struct{}
	held                                                                                      bool
	badAuthority                                                                              int
	reads                                                                                     int
}

func (m *blsMock) reset() {
	m.enabled = true
	m.version = 3
	m.templateVersion = 1
	m.templateID = "sold-out-reply/v1"
	m.templateBody = ""
	m.publishedID = ""
	m.note = ""
	m.removed = false
	m.blocked = false
	m.triggered = false
	m.receipts = []blsReceipt{}
	m.results = map[string]any{}
	m.fault = ""
	m.held = false
}
func (m *blsMock) facts() any {
	return map[string]any{"class": "MOCK", "receipts": m.receipts, "enabled": m.enabled, "version": m.version,
		"template_version": m.templateVersion, "template_id": m.templateID, "blocked": m.blocked, "removed": m.removed,
		"bad_authority": m.badAuthority, "reads": m.reads, "held": m.held}
}
func blsExact(b map[string]any, required ...string) bool {
	if len(b) != len(required) {
		return false
	}
	for _, k := range required {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}
func (m *blsMock) settings() any {
	return map[string]any{"enabled": m.enabled, "template_id": m.templateID, "template_version": m.templateVersion, "version": m.version}
}
func (m *blsMock) report() any {
	queued := 0
	if m.triggered {
		queued = 2
	}
	return map[string]any{"sent": []any{}, "queued": queued, "failed": []any{}, "followup": []any{
		map[string]any{"bundle_id": m.bundle, "display_name": "PRIVATE_FOLLOWUP_SENTINEL", "reminder_state": "followup", "reason": "window_closed", "link_copy_allowed": true},
		map[string]any{"bundle_id": m.restrictedBundle, "display_name": "PRIVATE_RESTRICTED_SENTINEL", "reminder_state": "restricted", "reason": "restricted", "link_copy_allowed": false}}, "link": "https://fixture.invalid/zh-TW/checkout"}
}
func (m *blsMock) serve(w http.ResponseWriter, r *http.Request, store string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if store != m.store && store != m.otherStore {
		consoleJSON(w, 404, map[string]any{"code": "not_found"})
		return true
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/admin/stores/"+store+"/")
	session := m.scene
	if store == m.otherStore {
		session = m.otherScene
	}
	base := "live-sessions/" + session
	if path == "live-sessions" && r.Method == "GET" {
		s := consoleMockScene{ID: session, SKU: randomUUID(), Title: "MOCK settings scene"}
		consoleJSON(w, 200, map[string]any{"items": []any{s.draft()}, "next_cursor": ""})
		return true
	}
	if strings.HasPrefix(path, "live-sessions/") && !strings.HasPrefix(path, base+"/") {
		consoleJSON(w, 404, map[string]any{"code": "not_found"})
		return true
	}
	if r.Method == "GET" {
		m.reads++
		switch path {
		case "live-settings/sold-out-reply":
			if store != m.store {
				consoleJSON(w, 200, map[string]any{"enabled": true, "template_id": "sold-out-reply/v1", "template_version": 1, "version": 1})
				return true
			}
			consoleJSON(w, 200, m.settings())
			return true
		case "message-templates":
			consoleJSON(w, 200, map[string]any{"items": []any{map[string]any{"template_id": "sold-out-reply/v1", "version": 1, "name": "Fixed sold-out reply", "kinds": []string{"private_reply"}, "public_safe": false, "created_at": "2030-01-01T00:00:00Z"}}})
			return true
		case base + "/reminders":
			if store != m.store {
				consoleJSON(w, 200, map[string]any{"sent": []any{}, "queued": 0, "failed": []any{}, "followup": []any{}, "link": nil})
				return true
			}
			value := m.report()
			if m.hold != nil {
				ch := m.hold
				m.held = true
				m.mu.Unlock()
				select {
				case <-ch:
				case <-r.Context().Done():
				}
				m.mu.Lock()
			}
			consoleJSON(w, 200, value)
			return true
		case base + "/claims/blocklist":
			rows := []any{}
			cursor := ""
			if store == m.store {
				id := m.entry
				note := "PRIVATE_NOTE_SENTINEL"
				if r.URL.Query().Get("cursor") != "" {
					id = m.nextEntry
					note = "PRIVATE_SECOND_NOTE"
				} else {
					cursor = "mock-page-two"
				}
				if id != m.entry || !m.removed {
					rows = append(rows, map[string]any{"id": id, "platform": "facebook", "note": note, "source_bundle_id": m.bundle, "created_at": "2030-01-01T00:00:00Z"})
				}
				if m.blocked && r.URL.Query().Get("cursor") == "" {
					rows = append(rows, map[string]any{"id": m.restrictedBundle, "platform": "instagram", "note": m.note, "source_bundle_id": m.bundle, "created_at": "2030-01-02T00:00:00Z"})
				}
			}
			consoleJSON(w, 200, map[string]any{"items": rows, "next_cursor": cursor})
			return true
		case base + "/claims/blocklist/check":
			consoleJSON(w, 200, map[string]any{"restricted": m.blocked})
			return true
		case "inbox/buyer-panel":
			consoleJSON(w, 200, map[string]any{"display_name": "PRIVATE_BUYER_SENTINEL", "platform": "facebook", "purchase_ordinal": 1, "claims": []any{map[string]any{"session_id": m.scene, "offer_id": m.entry, "keyword": "A1", "quantity": 1}}, "claim_total_minor": 30000, "orders": []any{}, "link_pending_manual": false})
			return true
		case base + "/console":
			s := consoleMockScene{ID: session, Store: store, Title: "MOCK settings scene", Platform: "facebook", Phase: "draft", Version: 1, Offer: m.entry, SKU: m.bundle, Warehouse: m.entry, OfferVersion: 1, Stock: 1, StockVersion: 1}
			consoleJSON(w, 200, s.read())
			return true
		case base + "/comments":
			row := map[string]any{"ref": m.comment, "parent_ref": nil, "created_at": "2030-01-01T00:00:00Z", "author_name": "PRIVATE_BUYER_SENTINEL", "text": "PRIVATE_COMMENT_SENTINEL", "is_page": false, "has_attachment": false,
				"marks": map[string]any{"intake": nil, "claim": map[string]any{"status": "ACCEPTED", "reason": nil, "offer_id": m.entry, "keyword": "A1", "quantity": 1, "bundle_id": m.bundle}, "private_reply": nil, "printed": nil, "public_replies": 0, "private_reply_available": false, "private_reply_unavailable_reason": "no_source"}}
			consoleJSON(w, 200, map[string]any{"epoch": 1, "reset": false, "items": []any{row}, "next": map[string]any{"epoch": 1, "seq": 1}, "older_cursor": nil, "stream": map[string]any{"state": "live", "source_platform": "facebook", "video_embeddable": false}})
			return true
		}
		return false
	}
	action := ""
	switch {
	case path == "live-settings/sold-out-reply" && r.Method == "PUT":
		action = "sold-out"
	case path == "message-templates" && r.Method == "POST":
		action = "publish"
	case path == base+"/reminders" && r.Method == "POST":
		action = "remind"
	case path == base+"/claims/blocklist" && r.Method == "POST":
		action = "block"
	case strings.HasPrefix(path, base+"/claims/blocklist/entries/") && r.Method == "DELETE":
		action = "remove"
	default:
		return false
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 65536))
	if err != nil {
		consoleJSON(w, 400, map[string]any{"code": "invalid_request"})
		return true
	}
	key := r.Header.Get("Idempotency-Key")
	receipt := blsReceipt{Action: action, KeyHash: consoleHash([]byte(key)), BodyHash: consoleHash(raw), Status: 200}
	if key == "" {
		consoleJSON(w, 400, map[string]any{"code": "idempotency_required"})
		return true
	}
	for _, p := range m.receipts {
		if p.KeyHash == receipt.KeyHash && p.Effect {
			if p.BodyHash != receipt.BodyHash || p.Action != action {
				consoleJSON(w, 409, map[string]any{"code": "idempotency_conflict"})
				return true
			}
			m.receipts = append(m.receipts, receipt)
			consoleJSON(w, 200, m.results[p.KeyHash])
			return true
		}
	}
	b := map[string]any{}
	if len(raw) > 0 && json.Unmarshal(raw, &b) != nil {
		consoleJSON(w, 422, map[string]any{"code": "invalid_request"})
		return true
	}
	status := 200
	var out any
	switch action {
	case "remind":
		if len(raw) != 0 {
			status = 422
			break
		}
		m.triggered = true
		out = map[string]any{"queued": 2, "already_reminded": 0, "followup": 1, "restricted": 1, "refused": 0, "truncated": false, "results": []any{}}
	case "publish":
		id, _ := b["template_id"].(string)
		body, _ := b["body"].(string)
		if !blsExact(b, "template_id", "name", "kinds", "public_safe", "body") || !strings.HasPrefix(id, "merchant-sold-out-") || body == "" || b["public_safe"] != false {
			status = 422
			break
		}
		kinds, ok := b["kinds"].([]any)
		if !ok || len(kinds) != 1 || kinds[0] != "private_reply" {
			status = 422
			break
		}
		m.templateBody = body
		m.publishedID = id
		out = map[string]any{"template_id": id, "version": 7, "public_safe": false, "kinds": []string{"private_reply"}}
	case "sold-out":
		enabled, ok := b["enabled"].(bool)
		id, _ := b["template_id"].(string)
		v, okv := b["template_version"].(float64)
		// The production adapter resolves a merchant-published receipt and rejects every Fixed template.
		if !blsExact(b, "enabled", "template_id", "template_version", "expected_version") || !ok || !okv || !strings.HasPrefix(id, "merchant-sold-out-") || id != m.publishedID || v != 7 {
			status = 422
			break
		}
		if m.fault == "conflict" {
			m.fault = ""
			m.version++
		}
		if b["expected_version"] != float64(m.version) {
			status = 409
			break
		}
		m.enabled = enabled
		m.templateID = id
		m.templateVersion = int(v)
		m.version++
		out = m.settings()
	case "block":
		note, _ := b["note"].(string)
		bundle, _ := b["bundle_id"].(string)
		if bundle != m.bundle || len(b) > 2 || utf8.RuneCountInString(note) > 200 {
			status = 422
			break
		}
		m.blocked = true
		m.note = note
		out = map[string]any{"id": m.restrictedBundle, "platform": "facebook", "created_at": "2030-01-01T00:00:00Z", "created": true}
	case "remove":
		if len(raw) != 0 {
			status = 422
			break
		}
		if !strings.HasSuffix(path, "/"+m.entry) {
			status = 404
			break
		}
		m.removed = true
		out = map[string]any{"removed": true}
	}
	receipt.Status = status
	receipt.Effect = status == 200
	m.receipts = append(m.receipts, receipt)
	if status != 200 {
		code := "invalid_request"
		if status == 409 {
			code = "version_conflict"
		}
		consoleJSON(w, status, map[string]any{"code": code})
		return true
	}
	m.results[receipt.KeyHash] = out
	// FAULT INJECTION: apply the MOCK effect then drop only its acknowledgement; browser still uses real BFF.
	if m.fault == "unknown" {
		m.fault = ""
		if hijacker, ok := w.(http.Hijacker); ok {
			conn, _, e := hijacker.Hijack()
			if e == nil {
				_ = conn.Close()
				return true
			}
		}
	}
	consoleJSON(w, 200, out)
	return true
}
