//go:build browser

// Purpose: LC-U1 MOCK Console gate plus LC-U2a real Go/PG comment/inbox browser fixtures.
// Depends on: lcSetup/newBrowserIDP, real identity/PG scope, packaged Next BFF and Playwright;
// LC_BROWSER_LIVE_CONSOLE_ACCEPTANCE, LC_TEST_DATABASE_ALLOWED, LC_BROWSER_EVIDENCE_ROOT, optional LC_BROWSER_CONSOLE_GREP.
// Used by: scripts/dev/test-local.sh --browser-live-console; REAL_PG comments, MOCK Graph and legacy Console scenes.
// Invariants: I01/I02/I06/I11/I14/I18; browser writes cross actual session/Origin/CSRF checks.
package foundation_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/identity"
	"livecommerce/internal/identityhttp"
	"livecommerce/internal/metaconnect"
	"livecommerce/internal/msgtemplates"
	"livecommerce/internal/oidclogin"
	"livecommerce/internal/platform"
)

type consoleMockScene struct {
	ID, Store, Title, Phase, Offer, SKU, Warehouse string
	Platform                                       string
	Version, OfferVersion, Stock, StockVersion     int64
	Generation                                     int64
	Active                                         bool
	Missing                                        bool
	Copied                                         bool
	Recommended                                    any
	Reads                                          []int64
	Fault                                          string
	ListUnavailable                                bool
	FacebookSource                                 bool
}
type consoleMockReceipt struct {
	Scene    string         `json:"scene"`
	Action   string         `json:"action"`
	KeyHash  string         `json:"key_hash"`
	BodyHash string         `json:"body_hash"`
	Status   int            `json:"status"`
	Effect   bool           `json:"effect"`
	Input    map[string]any `json:"input"`
}
type consoleMock struct {
	mu           sync.Mutex
	scenes       map[string]*consoleMockScene
	receipts     []consoleMockReceipt
	responses    map[string]any
	badAuthority int
}

func consoleHash(value []byte) string { sum := sha256.Sum256(value); return hex.EncodeToString(sum[:]) }
func consoleJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func (s *consoleMockScene) draft() map[string]any {
	return map[string]any{"session_id": s.ID, "program_id": s.SKU, "title": s.Title, "scheduled_at": nil, "aspect_ratio": "9:16", "state": "DRAFT", "version": int64(1), "created_at": "2030-01-01T00:00:00Z", "updated_at": "2030-01-01T00:00:00Z"}
}
func (s *consoleMockScene) offerReceipt() map[string]any {
	return map[string]any{"offer_id": s.Offer, "session_id": s.ID, "keyword": "A1", "sku_id": s.SKU, "sku_code": "MOCK-TEA", "product_name": "MOCK Console Tea", "max_quantity_per_claim": 10, "active": s.Active, "version": s.OfferVersion, "activated_at": "2030-01-01T00:00:00Z", "updated_at": "2030-01-01T00:00:00Z", "sku_price_minor": 30000, "currency": "TWD", "live_price_minor": 20000}
}
func (s *consoleMockScene) window() map[string]any {
	state := "CLOSED"
	if s.Phase == "live" {
		state = "OPEN"
	}
	var opened any
	if state == "OPEN" {
		opened = "2030-01-01T00:00:00Z"
	}
	return map[string]any{"session_id": s.ID, "state": state, "match_mode": "KEYWORD_QTY_CONTAINS", "generation": s.Generation, "version": s.Version, "opened_at": opened, "closed_at": nil}
}
func (s *consoleMockScene) read() map[string]any {
	orders, paid, orderMinor, paidMinor := int64(3), int64(1), int64(60000), int64(20000)
	if s.Copied {
		orders, paid, orderMinor, paidMinor = 0, 0, 0, 0
	}
	return map[string]any{
		"session":      map[string]any{"id": s.ID, "title": s.Title, "lifecycle": s.Phase, "version": s.Version, "started_at": nil, "ended_at": nil},
		"window":       map[string]any{"state": s.window()["state"], "generation": s.Generation, "opened_at": s.window()["opened_at"], "match_mode": "KEYWORD_QTY_CONTAINS"},
		"stats":        map[string]any{"comments": map[string]any{"total": nil, "source": "unavailable"}, "keyword_comments": 0, "buyers": 0, "orders": map[string]any{"count": orders, "amount_minor": orderMinor}, "paid": map[string]any{"count": paid, "amount_minor": paidMinor}, "currency": "TWD", "as_of": "2030-01-01T00:00:00Z"},
		"offers":       []any{map[string]any{"offer_id": s.Offer, "keyword": "A1", "sku_id": s.SKU, "product_name": "MOCK Console Tea", "variant_label": "250 g", "active": s.Active, "version": s.OfferVersion, "live_price_minor": 20000, "sku_price_minor": 30000, "stock": map[string]any{"tracked": true, "sellable": s.Stock, "reserved": 0, "warehouse_id": s.Warehouse, "balance_version": s.StockVersion}, "claimed": map[string]any{"buyers": 0, "quantity": 0}, "ordered_qty": 0, "paid_qty": 0, "paid_amount_minor": 0, "sold_out": s.Stock <= 0, "low_stock": s.Stock > 0 && s.Stock <= 5}},
		"capabilities": map[string]any{}, "stream": map[string]any{"state": "unavailable", "poll_interval_ms": 5000, "last_ok_at": nil, "lag_ms": nil, "source_platform": s.Platform, "video_embeddable": false, "reason": "no_source"}, "recommended": s.Recommended,
	}
}

// serve handles only the declared MOCK contract; identity and scope are checked by the caller.
func (m *consoleMock) serve(w http.ResponseWriter, r *http.Request, store string) bool {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 5 {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if parts[4] == "live-sessions" && len(parts) == 5 && r.Method == http.MethodGet {
		items := []any{}
		for _, s := range m.scenes {
			if s.Store == store {
				if s.ListUnavailable {
					consoleJSON(w, 503, map[string]any{"code": "unavailable"})
					return true
				}
				items = append(items, s.draft())
			}
		}
		consoleJSON(w, 200, map[string]any{"items": items, "next_cursor": ""})
		return true
	}
	if parts[4] == "live-sessions" && len(parts) == 6 && parts[5] == "results" && r.Method == http.MethodGet {
		items := []any{}
		for _, id := range r.URL.Query()["session_id"] {
			if scene := m.scenes[id]; scene != nil && scene.Store == store {
				orders, paid := int64(3), int64(1)
				money := []any{map[string]any{"currency": "TWD", "order_minor": int64(60000), "paid_minor": int64(20000), "sandbox_paid_minor": int64(10000)}}
				if scene.Copied {
					orders, paid, money = 0, 0, []any{}
				}
				items = append(items, map[string]any{"session_id": id, "orders": orders, "paid_orders": paid, "multi_session_orders": 0, "money": money})
			}
		}
		consoleJSON(w, 200, map[string]any{"as_of": "2030-01-01T00:00:00Z", "items": items})
		return true
	}
	var s *consoleMockScene
	if len(parts) >= 6 && parts[4] == "live-sessions" {
		s = m.scenes[parts[5]]
	}
	if parts[4] == "inventory" && len(parts) == 6 && parts[5] == "adjustments" {
		// The body identifies the fixture balance; keep decoding in the common write path below.
	} else if s == nil {
		return false
	}
	if s != nil && s.Store != store {
		consoleJSON(w, 404, map[string]any{"code": "not_found"})
		return true
	}
	if s != nil && len(parts) == 7 && parts[6] == "console" && r.Method == http.MethodGet {
		if s.Missing {
			consoleJSON(w, 404, map[string]any{"code": "not_found"})
			return true
		}
		s.Reads = append(s.Reads, time.Now().UnixMilli())
		consoleJSON(w, 200, s.read())
		return true
	}
	if s != nil && len(parts) == 6 && r.Method == http.MethodGet {
		// Match the real studioDetail envelope, including separate planning/media observations.
		consoleJSON(w, 200, map[string]any{"draft": s.draft(), "prepared": nil, "attempt": nil, "can_manage": true, "media_enabled": false})
		return true
	}
	if s != nil && len(parts) == 7 && parts[6] == "claims" && r.Method == http.MethodGet {
		rejected := map[string]int64{}
		for _, reason := range []string{"NO_MATCH", "UNKNOWN_KEYWORD", "OFFER_INACTIVE", "INVALID_QUANTITY", "QUANTITY_REQUIRED", "QUANTITY_OVER_MAX", "BUNDLE_LIMIT"} {
			rejected[reason] = 0
		}
		consoleJSON(w, 200, map[string]any{"window": s.window(), "offers": []any{s.offerReceipt()}, "stats": map[string]any{"generation": s.Generation, "accepted": 0, "rejected": rejected}})
		return true
	}
	if s != nil && (s.Copied || s.FacebookSource) && len(parts) == 7 && parts[6] == "claim-source" && r.Method == http.MethodGet {
		var source any
		platforms := []any{}
		if s.FacebookSource {
			source = map[string]any{"id": s.Offer, "platform": "facebook", "object": "page", "asset_id": "123", "source_object_id": "123_456", "private_reply": false, "reply_locale": "en", "active": true, "version": 1, "verified": true, "intake_count": 0, "intake_capped": 0, "updated_at": "2030-01-01T00:00:00Z"}
			platforms = []any{"facebook"}
		}
		consoleJSON(w, 200, map[string]any{"source": source, "platforms": platforms})
		return true
	}
	if r.Method != http.MethodPost && r.Method != http.MethodPatch {
		return false
	}
	action := parts[len(parts)-1]
	if action != "lifecycle" && action != "recommend" && action != "adjustments" && action != "copy" && !(len(parts) == 9 && parts[6] == "claims" && parts[7] == "offers") {
		return false
	}
	var body map[string]any
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&body); err != nil {
		consoleJSON(w, 400, map[string]any{"code": "invalid_request"})
		return true
	}
	if action == "adjustments" {
		for _, candidate := range m.scenes {
			if candidate.Store == store && candidate.SKU == body["sku_id"] {
				s = candidate
				break
			}
		}
	}
	if s == nil {
		consoleJSON(w, 404, map[string]any{"code": "not_found"})
		return true
	}
	key := r.Header.Get("Idempotency-Key")
	raw, _ := json.Marshal(body)
	receipt := consoleMockReceipt{Scene: s.ID, Action: action, KeyHash: consoleHash([]byte(key)), BodyHash: consoleHash(raw), Status: 200, Input: body}
	if key == "" {
		consoleJSON(w, 400, map[string]any{"code": "idempotency_required"})
		return true
	}
	for _, prior := range m.receipts {
		if prior.KeyHash == receipt.KeyHash && prior.Effect {
			if prior.BodyHash != receipt.BodyHash || prior.Action != action {
				consoleJSON(w, 409, map[string]any{"code": "idempotency_conflict"})
				return true
			}
			m.receipts = append(m.receipts, receipt)
			consoleJSON(w, 200, m.responses[receipt.KeyHash])
			return true
		}
	}
	if s.Fault == "conflict" {
		s.Fault = ""
		s.OfferVersion++
		receipt.Status = 409
		m.receipts = append(m.receipts, receipt)
		consoleJSON(w, 409, map[string]any{"code": "version_conflict"})
		return true
	}
	var out any
	expected, _ := body["expected_version"].(float64)
	switch action {
	case "lifecycle":
		if int64(expected) != s.Version {
			consoleJSON(w, 409, map[string]any{"code": "version_conflict"})
			return true
		}
		verb, _ := body["action"].(string)
		if verb == "start" && (s.Phase == "draft" || s.Phase == "ended") {
			s.Phase = "live"
			s.Generation++
		} else if verb == "end" && s.Phase == "live" {
			s.Phase = "ended"
		} else {
			consoleJSON(w, 409, map[string]any{"code": "invalid_transition"})
			return true
		}
		s.Version++
		out = map[string]any{"lifecycle": s.Phase, "version": s.Version, "window": s.window()}
	case "recommend":
		if int64(expected) != s.OfferVersion {
			consoleJSON(w, 409, map[string]any{"code": "version_conflict"})
			return true
		}
		s.Recommended = map[string]any{"offer_id": s.Offer, "at": "2030-01-01T00:00:00Z"}
		out = map[string]any{"recommended_at": "2030-01-01T00:00:00Z"}
	case "adjustments":
		if int64(expected) != s.StockVersion {
			consoleJSON(w, 409, map[string]any{"code": "version_conflict"})
			return true
		}
		delta, _ := body["delta"].(float64)
		if body["reason"] != "live_console_edit" || delta == 0 || delta != float64(int64(delta)) || delta < -1000 || delta > 1000 {
			consoleJSON(w, 422, map[string]any{"code": "invalid_request"})
			return true
		}
		if s.Stock+int64(delta) < 0 || s.Fault == "below_reserved" {
			s.Fault = ""
			receipt.Status = 422
			m.receipts = append(m.receipts, receipt)
			consoleJSON(w, 422, map[string]any{"code": "below_reserved"})
			return true
		}
		s.Stock += int64(delta)
		s.StockVersion++
		out = map[string]any{"sku_id": s.SKU, "warehouse_id": s.Warehouse, "on_hand": s.Stock, "reserved": 0, "allocated": 0, "version": s.StockVersion}
	case "copy":
		if int64(expected) != 1 {
			consoleJSON(w, 409, map[string]any{"code": "version_conflict"})
			return true
		}
		copy := *s
		copy.ID = randomUUID()
		copy.Offer = randomUUID()
		copy.Phase = "draft"
		copy.Generation = 0
		copy.Version = 1
		copy.Title, _ = body["title"].(string)
		copy.Reads = nil
		copy.Fault = ""
		copy.Recommended = nil
		copy.Copied = true
		copy.FacebookSource = false // A5 copies products/window settings, never a provider source binding.
		m.scenes[copy.ID] = &copy
		window := copy.window()
		window["generation"] = int64(0)
		out = map[string]any{"session": copy.draft(), "window": window, "created": []any{copy.offerReceipt()}, "conflicts": []any{}, "source_version": int64(1)}
	default:
		// Existing M4 requires all three fields; the short §7.2 description cannot relax Go's decoder.
		if body["max_quantity_per_claim"] != float64(10) {
			consoleJSON(w, 422, map[string]any{"code": "invalid_request"})
			return true
		}
		if int64(expected) != s.OfferVersion {
			consoleJSON(w, 409, map[string]any{"code": "version_conflict"})
			return true
		}
		s.Active, _ = body["active"].(bool)
		s.OfferVersion++
		out = s.offerReceipt()
	}
	receipt.Effect = true
	m.responses[receipt.KeyHash] = out
	if s.Fault == "unknown" {
		s.Fault = ""
		receipt.Status = 503
		m.receipts = append(m.receipts, receipt)
		consoleJSON(w, 503, map[string]any{"code": "response_lost_after_commit"})
		return true
	}
	m.receipts = append(m.receipts, receipt)
	if action == "copy" && s.Fault == "delay_copy" {
		s.Fault = ""
		// Delay only this ACK, not concurrent store reads or the facts control endpoint.
		m.mu.Unlock()
		time.Sleep(2 * time.Second)
		m.mu.Lock()
	}
	consoleJSON(w, 200, out)
	return true
}

// TestBrowserLiveConsoleRealChain runs signed MOCK OIDC, Next and Go with LC-U1 MOCK scenes
// and LC-U2a real PG comments/inbox services. No LIVE Graph or production acceptance.
func TestBrowserLiveConsoleRealChain(t *testing.T) {
	t.Run("workspace", func(t *testing.T) { runBrowserLiveConsole(t, false) })
	t.Run("labels", func(t *testing.T) { runBrowserLiveConsole(t, true) })
}

// runBrowserLiveConsole keeps label claims isolated from the frozen one-claim workspace fixture.
func runBrowserLiveConsole(t *testing.T, labels bool) {
	if os.Getenv("LC_BROWSER_LIVE_CONSOLE_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-live-console")
	}
	calibration := os.Getenv("LC_CONSOLE_CALIBRATION")
	if calibration != "" && calibration != "retain-on-reset" {
		t.Fatal("unsupported LC_CONSOLE_CALIBRATION")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Second)
	defer cancel()
	comments := newConsoleCommentsFixture(t)
	if labels {
		t.Cleanup(func() {
			// Only this disposable fixture's A3 facts; run before its parent session cleanup.
			mustExec(t, comments.e.h.f.owner, `DELETE FROM live.comment_prints WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3`, comments.e.h.f.tenantA, comments.e.h.f.storeA1, comments.e.session)
		})
		refs := []string{comments.ids["claim_ref"].(string)}
		for i, ref := range comments.ids["reply_refs"].([]string)[:2] {
			at := time.Now().UTC().Add(time.Second)
			// Signed ingress + the real intake worker: no direct claim/print row fabrication.
			comments.e.postFB(t, ref, "", fmt.Sprintf("A1+%d", i+2), &at, nil)
			comments.e.apply(t)
			if claim := comments.e.claimOf(t, ref); claim.Outcome != "ACCEPTED" || claim.Bundle == "" {
				t.Fatal("W3-U3 synthetic claim not accepted")
			}
			refs = append(refs, ref)
		}
		comments.ids["print_refs"] = refs
	}
	h := comments.e.h
	mustExec(t, h.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'inventory:write')`, h.f.tenantA, h.f.storeA1, h.actor)
	narrow, narrowToken := lcPrincipal(t, h.f, h.f.tenantA, []string{h.f.storeA1}, "store:read", "live:read", "inventory:live_adjust")
	_, readToken := lcPrincipal(t, h.f, h.f.tenantA, []string{h.f.storeA1}, "store:read", "live:read", "live:manage")
	mustExec(t, h.f.owner, `INSERT INTO identity.store_staff(tenant_id,store_id,principal_id,role) VALUES($1,$2,$3,'admin'),($1,$2,$4,'live_operator')`, h.f.tenantA, h.f.storeA1, h.actor, narrow)
	t.Cleanup(func() {
		mustExec(t, h.f.owner, `DELETE FROM identity.store_staff WHERE principal_id IN ($1,$2)`, h.actor, narrow)
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	_, port, _ := net.SplitHostPort(addr)
	_ = listener.Close()
	origin := browserFront(t, addr)
	idp := newBrowserIDP(t, origin+"/api/auth/callback")
	mustExec(t, h.f.owner, `INSERT INTO identity.external_identities(issuer,subject,principal_id) VALUES($1,'browser-subject',$2)`, idp.server.URL, h.actor)
	t.Cleanup(func() {
		mustExec(t, h.f.owner, `DELETE FROM identity.external_identities WHERE issuer=$1`, idp.server.URL)
	})
	_, _, authority := identityFixture(t)
	provider, err := oidclogin.New(ctx, oidclogin.Config{Issuer: idp.server.URL, ClientID: browserClientID, RedirectURL: idp.redirect, AllowLoopbackForTests: true})
	if err != nil {
		t.Fatal(err)
	}
	service, err := identity.New(authority, observedBrowserProvider{Provider: provider, t: t}, identity.Policy{ProviderKey: "browser-live-console-mock-v1", SessionTTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	bffKey := randomToken()
	private, err := identityhttp.NewHandler(service, bffKey)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/v1/identity/", private)
	mux.Handle("/", httpapi.NewHandler(h.f.runtime, httpapi.Options{SessionStoreList: true, Studio: true, ClaimLabels: &h.labels,
		CommentStream: comments.stream, Inbox: comments.e.svc, MsgTemplates: msgtemplates.NewService(),
		MetaHealth: &metaconnect.Health{Reader: metaconnect.TableReader{Fallback: metaconnect.SnapshotReader{}}}}))
	mock := &consoleMock{scenes: map[string]*consoleMockScene{}, responses: map[string]any{}, receipts: []consoleMockReceipt{}}
	ids := []string{}
	lateScenes := []string{}
	for i := 0; i < 8; i++ {
		id := h.draft(t, h.f.storeA1)
		if i < 6 {
			ids = append(ids, id)
		} else {
			lateScenes = append(lateScenes, id)
		}
		platformName := "facebook"
		if i%2 == 1 {
			platformName = "instagram"
		}
		mock.scenes[id] = &consoleMockScene{ID: id, Store: h.f.storeA1, Title: fmt.Sprintf("LC-U1 MOCK lane %d", i), Platform: platformName, Phase: "draft", Version: 1, Offer: randomUUID(), SKU: randomUUID(), Warehouse: randomUUID(), OfferVersion: 1, Stock: 12, StockVersion: 1, Active: true}
	}
	other := h.draft(t, h.f.storeA2)
	mock.scenes[other] = &consoleMockScene{ID: other, Store: h.f.storeA2, Title: "LC-U1 MOCK other store", Platform: "facebook", Phase: "draft", Version: 1, Offer: randomUUID(), SKU: randomUUID(), Warehouse: randomUUID(), OfferVersion: 1, Stock: 9, StockVersion: 1, Active: true}
	controlKey := randomToken()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/__test/live-console/") {
			if r.Header.Get("X-Console-Control") != controlKey {
				consoleJSON(w, 403, map[string]any{"code": "forbidden"})
				return
			}
			if r.URL.Path == "/__test/live-console/fault" && r.Method == http.MethodPost {
				// Decode once so the pre-existing MOCK fault controls retain their exact behavior.
				var in struct {
					Scene string `json:"scene"`
					Mode  string `json:"mode"`
				}
				if json.NewDecoder(r.Body).Decode(&in) != nil {
					consoleJSON(w, 400, map[string]any{"code": "invalid_request"})
					return
				}
				if in.Scene == comments.e.session && in.Mode == "comments_reset" {
					if err := comments.resetComments(r.Context()); err != nil {
						consoleJSON(w, 500, map[string]any{"code": "fixture_reset_failed"})
						return
					}
					consoleJSON(w, 200, map[string]any{"armed": true})
					return
				}
				if in.Scene == comments.e.session && (in.Mode == "public_graph_unknown" || in.Mode == "public_graph_restore") {
					// TEST-ONLY network fault: the real operation worker receives a MOCK Graph 5xx.
					mode := "5xx"
					if in.Mode == "public_graph_restore" {
						mode = "ok"
					}
					comments.e.g.setMode(mode)
					consoleJSON(w, 200, map[string]any{"armed": true})
					return
				}
				if in.Scene == comments.e.session && (in.Mode == "grant_revoke" || in.Mode == "grant_restore") {
					// TEST SETUP ONLY: mutate this synthetic principal/store's real grant. The
					// BFF's authenticatedStores lookup must produce the scoped 404, not a mock response.
					query := `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='store:read'`
					if in.Mode == "grant_restore" {
						query = `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'store:read') ON CONFLICT DO NOTHING`
					}
					if _, err := h.f.owner.Exec(r.Context(), query, h.f.tenantA, h.f.storeA1, h.actor); err != nil {
						consoleJSON(w, 500, map[string]any{"code": "fixture_grant_failed"})
						return
					}
					consoleJSON(w, 200, map[string]any{"granted": in.Mode == "grant_restore"})
					return
				}
				raw, _ := json.Marshal(in)
				r.Body = io.NopCloser(strings.NewReader(string(raw)))
			}
			mock.mu.Lock()
			defer mock.mu.Unlock()
			if r.URL.Path == "/__test/live-console/fault" && r.Method == http.MethodPost {
				var in struct {
					Scene string `json:"scene"`
					Mode  string `json:"mode"`
				}
				if json.NewDecoder(r.Body).Decode(&in) != nil || mock.scenes[in.Scene] == nil || (in.Mode != "conflict" && in.Mode != "unknown" && in.Mode != "missing" && in.Mode != "restore" && in.Mode != "delay_copy" && in.Mode != "list_unavailable" && in.Mode != "list_restore" && in.Mode != "below_reserved" && in.Mode != "facebook_source") {
					consoleJSON(w, 400, map[string]any{"code": "invalid_request"})
					return
				}
				if in.Mode == "facebook_source" {
					mock.scenes[in.Scene].FacebookSource = true
				} else if in.Mode == "list_unavailable" || in.Mode == "list_restore" {
					mock.scenes[in.Scene].ListUnavailable = in.Mode == "list_unavailable"
				} else if in.Mode == "missing" || in.Mode == "restore" {
					mock.scenes[in.Scene].Missing = in.Mode == "missing"
				} else {
					mock.scenes[in.Scene].Fault = in.Mode
				}
				consoleJSON(w, 200, map[string]any{"armed": true})
				return
			}
			if r.URL.Path == "/__test/live-console/facts" && r.Method == http.MethodGet {
				facts, err := comments.facts(r.Context())
				if err != nil {
					consoleJSON(w, 500, map[string]any{"code": "fixture_facts_failed"})
					return
				}
				consoleJSON(w, 200, map[string]any{"class": "MOCK", "scenes": mock.scenes, "receipts": mock.receipts, "bad_authority": mock.badAuthority, "comments": facts})
				return
			}
			consoleJSON(w, 404, map[string]any{"code": "not_found"})
			return
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		comments.observe(r)
		if len(parts) >= 5 && parts[0] == "v1" && parts[1] == "admin" && parts[2] == "stores" && (parts[4] == "live-sessions" || parts[4] == "inventory") {
			if r.Header.Get("Cookie") != "" || r.Header.Get("X-Tenant-ID") != "" || r.Header.Get("X-Forwarded-Host") != "" {
				mock.mu.Lock()
				mock.badAuthority++
				mock.mu.Unlock()
			}
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			permission := "live:read"
			if r.Method != http.MethodGet {
				permission = "live:manage"
			}
			if len(parts) == 9 && parts[6] == "comments" && (parts[8] == "private-reply" || parts[8] == "public-reply") {
				permission = "inbox:reply"
			}
			if parts[4] == "inventory" {
				permission = "inventory:write"
			}
			err := platform.WithScope(r.Context(), h.f.runtime, token, parts[3], permission, func(pgx.Tx, platform.Scope) error { return nil })
			if parts[4] == "inventory" && errors.Is(err, platform.ErrForbidden) {
				// Mirror LC-B7's second, narrow grant. The MOCK write checks its fixed reason and bounds.
				err = platform.WithScope(r.Context(), h.f.runtime, token, parts[3], "inventory:live_adjust", func(pgx.Tx, platform.Scope) error { return nil })
			}
			if err != nil {
				status, code := http.StatusServiceUnavailable, "unavailable"
				switch {
				case errors.Is(err, platform.ErrForbidden):
					status, code = http.StatusForbidden, "forbidden"
				case errors.Is(err, platform.ErrUnauthorized):
					status, code = http.StatusUnauthorized, "unauthorized"
				case errors.Is(err, platform.ErrScopeNotFound):
					status, code = http.StatusNotFound, "not_found"
				}
				consoleJSON(w, status, map[string]any{"code": code})
				return
			}
			if mock.serve(w, r, parts[3]) {
				return
			}
		}
		mux.ServeHTTP(w, r)
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/live-sessions/"+comments.e.session+"/comments/") {
			if err := comments.dispatch(r.Context()); err != nil {
				t.Error("LC-U2a fixture dispatch failed")
			}
		}
	}))
	defer api.Close()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	evidenceRoot := browserEvidenceRoot(root)
	if err = os.MkdirAll(evidenceRoot, 0700); err != nil {
		t.Fatal(err)
	}
	evidence, err := os.MkdirTemp(evidenceRoot, "live-console-")
	if err != nil {
		t.Fatal(err)
	}
	nextLog := browserLog(t, filepath.Join(evidence, "next.log"))
	next := exec.CommandContext(ctx, "node", filepath.Join(root, "apps/admin/.next/standalone/apps/admin/server.js"))
	next.Dir = root
	next.Env = browserEnvironment(map[string]string{"HOSTNAME": "127.0.0.1", "PORT": port, "NODE_ENV": "production", "COMMERCE_IDENTITY_ENABLED": "1", "COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS": "1", "COMMERCE_PUBLIC_ORIGIN": origin, "COMMERCE_API_ORIGIN": api.URL, "COMMERCE_OIDC_ISSUER": idp.server.URL, "COMMERCE_BFF_KEY": bffKey, "LC_BROWSER_LIVE_CONSOLE_ACCEPTANCE": "1", "LC_CONSOLE_CALIBRATION": calibration})
	next.Stdout, next.Stderr = nextLog, nextLog
	if err = next.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- next.Wait() }()
	defer func() {
		_ = next.Process.Kill()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("owned Next did not stop")
		}
	}()
	ready := false
	for i := 0; i < 150; i++ {
		resp, e := http.Get("http://" + addr + "/en/")
		if e == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == 200 {
				ready = true
				break
			}
		}
		select {
		case <-ctx.Done():
			t.Fatal("Next readiness deadline")
		case <-time.After(100 * time.Millisecond):
		}
	}
	if !ready {
		t.Fatalf("Next not ready; evidence=%s", evidence)
	}
	sceneJSON, _ := json.Marshal(ids)
	commentsJSON, _ := json.Marshal(comments.ids)
	log := browserLog(t, filepath.Join(evidence, "playwright.log"))
	spec := "tests/admin/live-console.spec.ts"
	if labels {
		spec = "tests/admin/comment-label-print.spec.ts"
	}
	cmd := exec.CommandContext(ctx, "pnpm", "exec", "playwright", "test", spec, "--reporter=list", "--output="+filepath.Join(evidence, "results"))
	if grep := os.Getenv("LC_BROWSER_CONSOLE_GREP"); grep != "" {
		// Optional local focused-spec run; a filtered success is not acceptance of the full mode.
		cmd.Args = append(cmd.Args, "--grep", grep)
		t.Logf("FOCUSED_SPEC_ONLY: live-console grep=%q; full-mode acceptance remains NOT_RUN", grep)
	}
	cmd.Dir = root
	cmd.Env = browserEnvironment(map[string]string{"LC_CONSOLE_CALIBRATION": calibration, "FORCE_COLOR": "0", "NO_COLOR": "1", "LC_BROWSER_SUITE": "live-console", "LC_BROWSER_PUBLIC_ORIGIN": origin, "LC_BROWSER_API_ORIGIN": api.URL, "LC_BROWSER_EVIDENCE": evidence, "LC_BROWSER_CONSOLE_SCENES": string(sceneJSON), "LC_BROWSER_CONSOLE_LATE_SCENE": lateScenes[0], "LC_BROWSER_CONSOLE_LATE_DEST": lateScenes[1], "LC_BROWSER_CONSOLE_STORE": h.f.storeA1, "LC_BROWSER_CONSOLE_OTHER_STORE": h.f.storeA2, "LC_BROWSER_CONSOLE_OTHER_SCENE": other, "LC_BROWSER_CONSOLE_CONTROL": controlKey, "LC_BROWSER_CONSOLE_NARROW_TOKEN": narrowToken, "LC_BROWSER_CONSOLE_READ_TOKEN": readToken, "LC_BROWSER_CONSOLE_COMMENTS": string(commentsJSON)})
	var diagnostics consoleDiagnosticBuffer
	cmd.Stdout, cmd.Stderr = &diagnostics, &diagnostics
	runErr := cmd.Run()
	if _, err := io.WriteString(log, consolePlaywrightSummary(diagnostics.Bytes())); err != nil {
		t.Error("console diagnostic write failed")
	}
	commentFacts, factsErr := comments.facts(ctx)
	if factsErr != nil {
		t.Error("LC-U2a final fixture facts failed")
	}
	mock.mu.Lock()
	raw, _ := json.MarshalIndent(map[string]any{"class": "MOCK", "comments_class": "REAL_PG + MOCK Graph", "comments": commentFacts, "scenes": mock.scenes, "receipts": mock.receipts, "bad_authority": mock.badAuthority}, "", "  ")
	bad := mock.badAuthority
	mock.mu.Unlock()
	if err = os.WriteFile(filepath.Join(evidence, "mock-receipts.json"), raw, 0600); err != nil {
		t.Error(err)
	}
	if bad != 0 {
		t.Errorf("unexpected forwarded identity headers: %d", bad)
	}
	if runErr != nil {
		t.Fatalf("LC-U1/U2a browser failed: %v; evidence=%s", runErr, evidence)
	}
	t.Logf("LC-U1 MOCK Console; LC-U2a REAL_PG + MOCK Graph browser evidence=%s", evidence)
}
