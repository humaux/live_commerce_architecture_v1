//go:build browser

// Purpose: test-only MOCK W3-U2 business API with immutable command receipts and bounded fault controls.
// Depends on: Go httptest caller, shared console JSON/hash helpers and actual W3 API shapes.
// Used by: TestBrowserLiveSettingsUIRealChain; real signed authorization is checked before this backend.
// Invariants: I02/I06/I11/I14; private notes stay transient and only command hashes enter evidence.
package foundation_test

import (
	"encoding/json"
	"fmt"
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

// TestBrowserLiveSettingsMockCommentsContract checks the actual fixture handler
// against A2's seq/coverage envelope required by the merged comment-page parser.
func TestBrowserLiveSettingsMockCommentsContract(t *testing.T) {
	m := &blsMock{store: randomUUID(), scene: randomUUID(), bundle: randomUUID(), entry: randomUUID(), comment: "700_800"}
	m.reset()
	request := httptest.NewRequest(http.MethodGet, "/v1/admin/stores/"+m.store+"/live-sessions/"+m.scene+"/comments", nil)
	response := httptest.NewRecorder()
	if !m.serve(response, request, m.store) || response.Code != http.StatusOK {
		t.Fatal("fixture A2 route unavailable")
	}
	var page map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatal("fixture A2 JSON invalid")
	}
	if exhausted, ok := page["scan_exhausted"].(bool); !ok || !exhausted {
		t.Error("fixture's complete one-row page needs explicit A2 scan_exhausted=true")
	}
	rows, ok := page["items"].([]any)
	if !ok || len(rows) != 1 {
		t.Fatal("fixture one-row projection missing")
	}
	row, ok := rows[0].(map[string]any)
	if !ok || row["seq"] != float64(1) {
		t.Error("fixture's live comment needs A2 seq=1, matching the cursor")
	}
}

// PR31 regression sources are exercised through the actual MOCK handler, not browser response replacement.
func TestBrowserLiveSettingsMockPR31Sources(t *testing.T) {
	for _, scenario := range []string{"many-scenes", "reply-states", "missing-reminders"} {
		t.Run(scenario, func(t *testing.T) {
			m := &blsMock{store: randomUUID(), scene: randomUUID(), bundle: randomUUID(), entry: randomUUID(), comment: "700_800"}
			m.reset()
			m.fault = scenario
			get := func(resource string) (int, map[string]any) {
				t.Helper()
				response := httptest.NewRecorder()
				request := httptest.NewRequest(http.MethodGet, "/v1/admin/stores/"+m.store+"/"+resource, nil)
				if !m.serve(response, request, m.store) {
					t.Fatal("regression source route unavailable")
				}
				var body map[string]any
				if scenario == "missing-reminders" && strings.HasSuffix(resource, "/reminders") && response.Code == http.StatusNotFound && response.Header().Get("Content-Type") == "text/plain; charset=utf-8" {
					return response.Code, body
				}
				if json.Unmarshal(response.Body.Bytes(), &body) != nil {
					t.Fatal("regression source invalid JSON")
				}
				return response.Code, body
			}
			switch scenario {
			case "many-scenes":
				_, first := get("live-sessions?limit=20")
				rows, ok := first["items"].([]any)
				if !ok || len(rows) != 20 || first["next_cursor"] == "" {
					t.Fatal("21-scene source must expose a bounded first page and next cursor")
				}
				_, second := get("live-sessions?limit=20&cursor=" + first["next_cursor"].(string))
				rows, ok = second["items"].([]any)
				if !ok || len(rows) != 1 || second["next_cursor"] != "" {
					t.Fatal("21-scene source must expose one final scene")
				}
				last := rows[0].(map[string]any)
				id := last["session_id"].(string)
				status, detail := get("live-sessions/" + id)
				if status != 200 || detail["draft"].(map[string]any)["title"] != last["title"] {
					t.Fatal("later-scene detail disagrees with picker")
				}
				status, _ = get("live-sessions/" + id + "/reminders")
				if status != 200 {
					t.Fatal("later scene report not addressable")
				}
				status, block := get("live-sessions/" + id + "/claims/blocklist")
				if status != 200 || len(block["items"].([]any)) != 1 {
					t.Fatal("later scene blocklist not addressable")
				}
			case "reply-states":
				_, body := get("live-sessions/" + m.scene + "/comments")
				rows, ok := body["items"].([]any)
				if !ok || len(rows) != 5 {
					t.Fatal("five reply states absent")
				}
				states := []string{"SUCCEEDED", "READY", "FAILED_FINAL", "BLOCKED_POLICY", "UNKNOWN"}
				for i, value := range rows {
					row := value.(map[string]any)
					reply := row["marks"].(map[string]any)["private_reply"].(map[string]any)
					if row["seq"] != float64(i+1) || reply["state"] != states[i] || reply["kind"] != "out_of_stock" {
						t.Fatal("state or A2 sequence incorrect")
					}
				}
				if body["scan_exhausted"] != true || body["next"].(map[string]any)["seq"] != float64(5) {
					t.Fatal("five-row A2 coverage incomplete")
				}
			case "missing-reminders":
				status, _ := get("live-sessions/" + m.scene + "/reminders")
				if status != 404 {
					t.Fatal("unmounted reminder route must return404")
				}
				response := httptest.NewRecorder()
				request := httptest.NewRequest(http.MethodPost, "/v1/admin/stores/"+m.store+"/live-sessions/"+m.scene+"/reminders", nil)
				request.Header.Set("Idempotency-Key", "fixture-unmounted-route")
				m.serve(response, request, m.store)
				if response.Code != 404 || m.triggered || len(m.receipts) != 0 {
					t.Fatal("unmounted send route recorded an effect")
				}
				status, _ = get("live-settings/sold-out-reply")
				if status != 200 {
					t.Fatal("missing reminders broke settings")
				}
				status, _ = get("live-sessions/" + m.scene + "/claims/blocklist")
				if status != 200 {
					t.Fatal("missing reminders broke blocklist")
				}
			}
		})
	}
}

// The BFF distinguishes an unmounted Go route from a JSON404 scope-loss response.
// This wire assertion prevents an optional-service fixture from weakening that privacy boundary.
func TestBrowserLiveSettingsMockUnmountedReminderWire(t *testing.T) {
	m := &blsMock{store: randomUUID(), scene: randomUUID()}
	m.reset()
	m.fault = "missing-reminders"
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		t.Run(method, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(method, "/v1/admin/stores/"+m.store+"/live-sessions/"+m.scene+"/reminders", nil)
			request.Header.Set("Idempotency-Key", "fixture-unmounted-wire")
			if !m.serve(response, request, m.store) {
				t.Fatal("unmounted route fixture was not handled")
			}
			if response.Code != http.StatusNotFound || response.Header().Get("Content-Type") != "text/plain; charset=utf-8" || response.Body.String() != "404 page not found\n" {
				t.Fatal("unmounted route must reproduce Go http.NotFound status, content type and exact plain body")
			}
			if m.triggered || len(m.receipts) != 0 {
				t.Fatal("unmounted reminder route recorded a command effect")
			}
		})
	}
	// A missing scoped resource stays JSON not_found; it must never be mistaken for an unmounted service.
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/admin/stores/"+m.store+"/live-sessions/"+randomUUID()+"/reminders", nil)
	m.serve(response, request, m.store)
	if response.Code != http.StatusNotFound || response.Header().Get("Content-Type") != "application/json" || !strings.Contains(response.Body.String(), `"code":"not_found"`) {
		t.Fatal("scope/resource absence must retain JSON404 not_found")
	}
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

// Synthetic handles are stable fixture authority only, never real buyer identifiers.
func blsSceneID(index int) string { return fmt.Sprintf("81000000-0000-4000-8000-%012d", index) }

var blsReplyStates = []string{"SUCCEEDED", "READY", "FAILED_FINAL", "BLOCKED_POLICY", "UNKNOWN"}
var blsReplyRefs = []string{"700_801", "700_802", "700_803", "700_804", "700_805"}

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
	title := "MOCK settings scene"
	if m.fault == "many-scenes" && store == m.store {
		title = "MOCK settings scene 01"
		for i := 2; i <= 21; i++ {
			candidate := blsSceneID(i)
			if path == "live-sessions/"+candidate || strings.HasPrefix(path, "live-sessions/"+candidate+"/") {
				session = candidate
				title = fmt.Sprintf("MOCK settings scene %02d", i)
				break
			}
		}
	}
	base := "live-sessions/" + session
	if path == "live-sessions" && r.Method == "GET" {
		rows := []any{(&consoleMockScene{ID: session, SKU: m.bundle, Title: title}).draft()}
		cursor := ""
		if m.fault == "many-scenes" && store == m.store {
			rows = []any{}
			if r.URL.Query().Get("cursor") == "mock-scenes-page-two" {
				rows = append(rows, (&consoleMockScene{ID: blsSceneID(21), SKU: m.bundle, Title: "MOCK settings scene 21"}).draft())
			} else {
				for i := 1; i <= 20; i++ {
					id := blsSceneID(i)
					if i == 1 {
						id = m.scene
					}
					rows = append(rows, (&consoleMockScene{ID: id, SKU: m.bundle, Title: fmt.Sprintf("MOCK settings scene %02d", i)}).draft())
				}
				cursor = "mock-scenes-page-two"
			}
		}
		consoleJSON(w, 200, map[string]any{"items": rows, "next_cursor": cursor})
		return true
	}
	if path == base && r.Method == "GET" {
		draft := (&consoleMockScene{ID: session, SKU: m.bundle, Title: title}).draft()
		consoleJSON(w, 200, map[string]any{"draft": draft, "prepared": nil, "attempt": nil, "can_manage": true, "media_enabled": false})
		return true
	}
	if strings.HasPrefix(path, "live-sessions/") && !strings.HasPrefix(path, base+"/") {
		consoleJSON(w, 404, map[string]any{"code": "not_found"})
		return true
	}
	// Fault represents an unmounted optional read/send service, before receipt/effect handling.
	if path == base+"/reminders" && m.fault == "missing-reminders" {
		http.NotFound(w, r)
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
				if session == blsSceneID(21) {
					id = blsSceneID(22)
				}
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
			row := map[string]any{"ref": m.comment, "seq": 1, "parent_ref": nil, "created_at": "2030-01-01T00:00:00Z", "author_name": "PRIVATE_BUYER_SENTINEL", "text": "PRIVATE_COMMENT_SENTINEL", "is_page": false, "has_attachment": false,
				"marks": map[string]any{"intake": nil, "claim": map[string]any{"status": "ACCEPTED", "reason": nil, "offer_id": m.entry, "keyword": "A1", "quantity": 1, "bundle_id": m.bundle}, "private_reply": nil, "printed": nil, "public_replies": 0, "private_reply_available": false, "private_reply_unavailable_reason": "no_source"}}
			rows := []any{row}
			seq := 1
			if m.fault == "reply-states" {
				rows = []any{}
				for i, state := range blsReplyStates {
					copyRow := map[string]any{}
					for key, value := range row {
						copyRow[key] = value
					}
					copyMarks := map[string]any{}
					for key, value := range row["marks"].(map[string]any) {
						copyMarks[key] = value
					}
					copyMarks["private_reply"] = map[string]any{"kind": "out_of_stock", "state": state, "blocked_reason": nil}
					copyRow["marks"] = copyMarks
					copyRow["ref"] = blsReplyRefs[i]
					copyRow["seq"] = i + 1
					rows = append(rows, copyRow)
				}
				seq = len(rows)
			}
			consoleJSON(w, 200, map[string]any{"epoch": 1, "reset": false, "scan_exhausted": true, "items": rows, "next": map[string]any{"epoch": 1, "seq": seq}, "older_cursor": nil, "stream": map[string]any{"state": "live", "source_platform": "facebook", "video_embeddable": false}})
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
