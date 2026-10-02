package foundation_test

// store_design_gate_test.go: independent REAL_PG gate suite for unit store-design (contracts/storefront-v2.md section B and
// B-acceptance; migration 0087). Written from the contract and the unit brief, not from internal/design. Evidence label
// REAL_PG over the real httpapi (admin, bearer) and buyerhttp (BFF key + origin) handlers; the admin browser and BFF
// transport are gate --browser-design, the storefront rendering of the document is unit storefront-shell (NOT_RUN here).
//
// Cases (each is a subtest of TestStoreDesignGate; `go test -run '^TestStoreDesignGate$/<case>'`):
//   SD01 isolation          cross tenant/store/permission/session on every admin route, RLS, media, preview, buyer origin
//   SD02 races              concurrent publish / save / rollback / media-cap / delete-vs-reference, versions stay contiguous
//   SD03 preview            token expiry, staleness, store binding, hashed at rest, header-only, no other grant
//   SD04 validation         422 {path, reason} for unknown keys, caps, enums and XSS payloads in EVERY string/markdown field
//   SD05 media              sniffing, size cap, 60-per-store cap, delete-while-referenced, rollback to a deleted image
//   SD06 history            append-only published_versions (owner and runtime), rollback copies, audit rows, draft untouched
//   SD07 buyer              bearer/cookie/query/method/origin refused, unpublished 404, default document, normalised shape
//   SD08 envelope           CAS fields required, content type, unknown envelope fields, document size cap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"livecommerce/internal/httpapi"
)

const (
	sdgPreviewHeader = "X-Commerce-Design-Preview"
	sdgSecondOrigin  = "https://buyer-b.example"
	sdgUnpubOrigin   = "https://unpublished-a2.example"
)

type sdgEnv struct {
	t     *testing.T
	f     *testFixture
	admin http.Handler
	buyer bhHarness
	baseA string
	baseB string
}

func sdgPNG(t *testing.T, w int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, 3))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func sdgGrant(t *testing.T, f *testFixture, token, store string, perms ...string) {
	t.Helper()
	for _, p := range perms {
		mustExec(t, f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission)
			SELECT m.tenant_id,$2,s.principal_id,$3 FROM identity.sessions s JOIN identity.memberships m USING(principal_id)
			JOIN control.stores st ON st.tenant_id=m.tenant_id AND st.id=$2 WHERE s.token_hash=$1 ON CONFLICT DO NOTHING`, tokenHash(token), store, p)
	}
	t.Cleanup(func() {
		for _, p := range perms {
			mustExec(t, f.owner, `DELETE FROM identity.store_grants g USING identity.sessions s WHERE s.token_hash=$1 AND g.principal_id=s.principal_id AND g.store_id=$2 AND g.permission=$3`, tokenHash(token), store, p)
		}
	})
}

func sdgReset(t *testing.T, f *testFixture) {
	t.Helper() // TRUNCATE does not fire the append-only row trigger; these tables belong to this suite and the smoke test only
	mustExec(t, f.owner, `TRUNCATE design.preview_tokens, design.published_versions, design.store_media, design.documents`)
}

func sdgSetup(t *testing.T) *sdgEnv {
	t.Helper()
	f := fixture(t)
	sdgGrant(t, f, f.tokens["a"], f.storeA1, "integration:read", "integration:manage")
	sdgGrant(t, f, f.tokens["b"], f.storeB, "integration:read", "integration:manage")
	sdgGrant(t, f, f.tokens["a2"], f.storeA1, "store:read", "integration:read") // read-only viewer of store A1
	sdgReset(t, f)
	t.Cleanup(func() { sdgReset(t, f) })
	buyer := bhSetup(t) // publishes https://buyer.example for store A1
	bhPublish(t, buyer.bcHarness, sdgSecondOrigin, f.tenantB, f.storeB)
	mustExec(t, f.owner, `INSERT INTO control.storefront_domains(tenant_id,store_id,origin,state,ownership_verified_at,tls_verified_at,valid_until,evidence_ref)
		VALUES($1,$2,$3,'ACTIVE',clock_timestamp()-interval '1 hour',clock_timestamp()-interval '1 hour',clock_timestamp()+interval '1 hour','SYNTHETIC unpublished gate only')`, f.tenantA, f.storeA2, sdgUnpubOrigin)
	t.Cleanup(func() { mustExec(t, f.owner, `DELETE FROM control.storefront_domains WHERE origin=$1`, sdgUnpubOrigin) })
	return &sdgEnv{t: t, f: f, admin: httpapi.NewHandler(f.runtime), buyer: buyer,
		baseA: "/v1/admin/stores/" + f.storeA1 + "/design", baseB: "/v1/admin/stores/" + f.storeB + "/design"}
}

func (e *sdgEnv) do(token, method, path, ctype string, body []byte, edit func(*http.Request)) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, bytes.NewReader(body))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if ctype != "" {
		r.Header.Set("Content-Type", ctype)
	}
	if edit != nil {
		edit(r)
	}
	w := httptest.NewRecorder()
	e.admin.ServeHTTP(w, r)
	return w
}

func (e *sdgEnv) json(token, method, path string, payload any) *httptest.ResponseRecorder {
	var data []byte
	if payload != nil {
		data, _ = json.Marshal(payload)
	}
	return e.do(token, method, path, "application/json", data, nil)
}

// must performs a call as the store-A merchant and decodes a 200 into out.
func (e *sdgEnv) must(method, path string, payload any, want int, out any) {
	e.t.Helper()
	w := e.json(e.f.tokens["a"], method, path, payload)
	if w.Code != want {
		e.t.Fatalf("%s %s: status=%d want=%d body=%s", method, path, w.Code, want, w.Body.String())
	}
	if out != nil {
		if err := json.Unmarshal(w.Body.Bytes(), out); err != nil {
			e.t.Fatalf("decode %s: %v", w.Body.String(), err)
		}
	}
}

type sdgDraft struct {
	Version          int64          `json:"version"`
	Document         map[string]any `json:"document"`
	PublishedVersion *int64         `json:"published_version"`
}

type sdgVersion struct {
	Version       int64  `json:"version"`
	Kind          string `json:"kind"`
	SourceVersion int64  `json:"source_version"`
}

type sdgVersions struct {
	Items       []sdgVersion `json:"items"`
	LiveVersion *int64       `json:"live_version"`
}

type sdgErr struct {
	Code    string         `json:"code"`
	Details map[string]any `json:"details"`
}

func (e *sdgEnv) draft() sdgDraft {
	e.t.Helper()
	var d sdgDraft
	e.must("GET", e.baseA+"/draft", nil, 200, &d)
	return d
}

func (e *sdgEnv) versions() sdgVersions {
	e.t.Helper()
	var v sdgVersions
	e.must("GET", e.baseA+"/versions", nil, 200, &v)
	return v
}

// save stores doc as the next draft (reading the current version first).
func (e *sdgEnv) save(doc map[string]any) int64 {
	e.t.Helper()
	cur := e.draft().Version
	var d sdgDraft
	e.must("PUT", e.baseA+"/draft", map[string]any{"expected_version": cur, "document": doc}, 200, &d)
	return d.Version
}

func (e *sdgEnv) publish(version int64) sdgVersion {
	e.t.Helper()
	var v sdgVersion
	e.must("POST", e.baseA+"/publish", map[string]any{"expected_draft_version": version}, 200, &v)
	return v
}

func (e *sdgEnv) upload(token, base string, data []byte) *httptest.ResponseRecorder {
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	part, _ := mw.CreateFormFile("file", "x.png")
	_, _ = part.Write(data)
	_ = mw.Close()
	return e.do(token, "POST", base+"/media", mw.FormDataContentType(), b.Bytes(), nil)
}

func (e *sdgEnv) image(base, token string, w int) string {
	e.t.Helper()
	res := e.upload(token, base, sdgPNG(e.t, w))
	if res.Code != 200 {
		e.t.Fatalf("upload w=%d: %d %s", w, res.Code, res.Body.String())
	}
	var m struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(res.Body.Bytes(), &m)
	return m.ID
}

func (e *sdgEnv) buyerDo(method, path string, body []byte, edit func(*http.Request)) (int, http.Header, []byte) {
	e.t.Helper()
	r, _ := http.NewRequest(method, e.buyer.server.URL+path, bytes.NewReader(body))
	r.Header.Set("X-Commerce-Buyer-BFF-Key", e.buyer.key)
	r.Header.Set("X-Commerce-Storefront-Origin", e.buyer.origin)
	if edit != nil {
		edit(r)
	}
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(r)
	if err != nil {
		e.t.Fatalf("buyer request failed: %v", err)
	}
	defer res.Body.Close()
	out, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	return res.StatusCode, res.Header, out
}

func (e *sdgEnv) buyerGet(path string, edit func(*http.Request)) (int, http.Header, []byte) {
	return e.buyerDo("GET", path, nil, edit)
}

func sdgOrigin(origin string) func(*http.Request) {
	return func(r *http.Request) { r.Header.Set("X-Commerce-Storefront-Origin", origin) }
}

func sdgPreview(token string) func(*http.Request) {
	return func(r *http.Request) { r.Header.Set(sdgPreviewHeader, token) }
}

type sdgBuyerDoc struct {
	Version  int64          `json:"version"`
	Document map[string]any `json:"document"`
}

func (e *sdgEnv) published() sdgBuyerDoc {
	e.t.Helper()
	status, _, body := e.buyerGet("/v1/buyer/design/published", nil)
	var d sdgBuyerDoc
	if err := json.Unmarshal(body, &d); status != 200 || err != nil {
		e.t.Fatalf("buyer published: %d %s", status, body)
	}
	return d
}

// ---- document helpers (the contract section B shape, every optional field populated) ----

func sdgDeep(v map[string]any) map[string]any {
	raw, _ := json.Marshal(v)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return out
}

func sdgFullDoc(logo, favicon, hero, side string) map[string]any {
	return map[string]any{
		"profile": map[string]any{
			"name": "Gate Shop", "tagline": "Tagline", "logo_image_id": logo, "favicon_image_id": favicon, "accent_color": "#247965", "announcement": "Free shipping",
			"contact": map[string]any{"email": "shop@example.com", "phone": "+886 2 1234 5678", "address": "1 Test Road", "line_url": "https://line.me/R/ti/p/@gate",
				"facebook_url": "https://www.facebook.com/gate", "instagram_url": "https://www.instagram.com/gate"},
		},
		"nav": map[string]any{
			"header": []any{
				map[string]any{"label": "Home", "kind": "home", "target": nil},
				map[string]any{"label": "All", "kind": "all_products", "target": nil},
				map[string]any{"label": "Summer", "kind": "collection", "target": "summer"},
				map[string]any{"label": "About", "kind": "page", "target": "about"},
				map[string]any{"label": "Blog", "kind": "url", "target": "https://example.com/blog"},
			},
			"footer": []any{
				map[string]any{"label": "About", "kind": "page", "target": "about"},
				map[string]any{"label": "Blog", "kind": "url", "target": "https://example.com/blog"},
				map[string]any{"label": "Summer", "kind": "collection", "target": "summer"},
			},
		},
		"home": map[string]any{"sections": []any{
			map[string]any{"type": "hero", "image_id": hero, "heading": "Hero", "subheading": "Sub", "cta_label": "Go", "cta_kind": "page", "cta_target": "about"},
			map[string]any{"type": "featured_collection", "collection_slug": "summer", "heading": "Featured", "limit": 8},
			map[string]any{"type": "product_grid", "heading": "Grid", "sort": "newest", "limit": 12},
			map[string]any{"type": "rich_text", "heading": "Story", "body": "Hello **bold** *it*\n\n- one\n- two\n\n[site](https://example.com/a)"},
			map[string]any{"type": "image_text", "image_id": side, "heading": "Img", "body": "Body text", "image_side": "left"},
		}},
		"pages": []any{
			map[string]any{"slug": "about", "title": "About", "body": "About **us**"},
			map[string]any{"slug": "shipping", "title": "Shipping", "body": "- fast"},
		},
	}
}

const sdgDelete = "\x00delete"

func sdgSet(root map[string]any, path []any, v any) map[string]any {
	out := sdgDeep(root)
	var cur any = out
	for i, p := range path {
		last := i == len(path)-1
		switch k := p.(type) {
		case string:
			m := cur.(map[string]any)
			if last {
				if v == sdgDelete {
					delete(m, k)
				} else {
					m[k] = v
				}
			} else {
				cur = m[k]
			}
		case int:
			s := cur.([]any)
			if last {
				s[k] = v
			} else {
				cur = s[k]
			}
		}
	}
	return out
}

func sdgPathString(path []any) string {
	var b strings.Builder
	for _, p := range path {
		switch k := p.(type) {
		case string:
			if b.Len() > 0 {
				b.WriteByte('.')
			}
			b.WriteString(k)
		case int:
			fmt.Fprintf(&b, "[%d]", k)
		}
	}
	return b.String()
}

// reject sends doc as the next draft version and requires 422 invalid_request with details.path; payload must not be echoed.
func (e *sdgEnv) reject(doc map[string]any, wantPath string, wantPrefix bool, payload string) {
	e.t.Helper()
	cur := e.draft().Version
	w := e.json(e.f.tokens["a"], "PUT", e.baseA+"/draft", map[string]any{"expected_version": cur, "document": doc})
	var env sdgErr
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	path, _ := env.Details["path"].(string)
	reason, _ := env.Details["reason"].(string)
	if w.Code != 422 || env.Code != "invalid_request" || path == "" || reason == "" {
		e.t.Errorf("%s payload %q: want 422 invalid_request with details{path,reason}, got %d %s", wantPath, payload, w.Code, w.Body.String())
		return
	}
	if (!wantPrefix && path != wantPath) || (wantPrefix && !strings.HasPrefix(path, wantPath)) {
		e.t.Errorf("%s payload %q: details.path=%q", wantPath, payload, path)
	}
	if len(payload) >= 6 && strings.Contains(w.Body.String(), payload) {
		e.t.Errorf("%s: the 422 body echoes the rejected value %q: %s", wantPath, payload, w.Body.String())
	}
	if after := e.draft().Version; after != cur {
		e.t.Errorf("%s payload %q: a refused document changed the draft version %d -> %d", wantPath, payload, cur, after)
	}
}

// accept saves doc and requires 200.
func (e *sdgEnv) accept(doc map[string]any, label string) {
	e.t.Helper()
	cur := e.draft().Version
	w := e.json(e.f.tokens["a"], "PUT", e.baseA+"/draft", map[string]any{"expected_version": cur, "document": doc})
	if w.Code != 200 {
		e.t.Errorf("%s: want 200, got %d %s", label, w.Code, w.Body.String())
	}
}

// ---- the suite ----

func TestStoreDesignGate(t *testing.T) {
	e := sdgSetup(t)
	cases := []struct {
		name string
		fn   func(*testing.T, *sdgEnv)
	}{
		{"SD01_isolation", sdgIsolation},
		{"SD02_races", sdgRaces},
		{"SD02b_delete_vs_reference_race", sdgDeleteVsReference},
		{"SD03_preview", sdgPreviewTokens},
		{"SD04_validation", sdgValidation},
		{"SD04b_plain_text_html", sdgValidation},
		{"SD05_media", sdgMedia},
		{"SD06_history", sdgHistory},
		{"SD07_buyer", sdgBuyer},
		{"SD08_envelope", sdgEnvelope},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sdgReset(t, e.f)
			sub := *e
			sub.t = t
			c.fn(t, &sub)
		})
	}
}

// TestStoreDesignPreviewTokenIssueNeverRefusesALegitimateRequest pins the flake behind "B preview token: 422" in the full
// suite: design.preview_tokens CHECKed expires_at <= created_at + 15 minutes while the two column defaults were separate
// clock_timestamp() calls, so about 2% of inserts (measured: 4372 of 200000 on the shared PG image) straddled a
// microsecond and answered 422 invalid_request with no cause in the body. With the bug present 600 issues all succeed with
// probability (1-0.02)^600 < 1e-5, so this fails essentially every run until the insert takes one clock reading.
func TestStoreDesignPreviewTokenIssueNeverRefusesALegitimateRequest(t *testing.T) {
	f := fixture(t)
	sdgGrant(t, f, f.tokens["a"], f.storeA1, "integration:read", "integration:manage")
	sdgReset(t, f)
	t.Cleanup(func() { sdgReset(t, f) })
	e := &sdgEnv{t: t, f: f, admin: httpapi.NewHandler(f.runtime), baseA: "/v1/admin/stores/" + f.storeA1 + "/design"}
	e.must("PUT", e.baseA+"/draft", map[string]any{"expected_version": 0, "document": map[string]any{"profile": map[string]any{"name": "Gate Shop", "accent_color": "#247965"}}}, 200, nil)
	refused := 0
	for i := 0; i < 600; i++ {
		if w := e.json(f.tokens["a"], "POST", e.baseA+"/preview-token", struct{}{}); w.Code != 200 {
			refused++
			t.Errorf("preview token #%d: %d %s", i, w.Code, w.Body.String())
			if refused >= 5 {
				t.Fatalf("stopping after %d refused issues out of %d", refused, i+1)
			}
		}
	}
}

func sdgIsolation(t *testing.T, e *sdgEnv) {
	f := e.f
	img := e.image(e.baseA, f.tokens["a"], 5)
	e.save(sdgFullDoc(img, img, img, img))
	d1 := e.draft().Version
	e.publish(d1)
	routes := []struct {
		method, path, ctype string
		body                string
	}{
		{"GET", "/draft", "", ""},
		{"PUT", "/draft", "application/json", fmt.Sprintf(`{"expected_version":%d,"document":{"profile":{"name":"hijack","accent_color":"#000000"}}}`, d1)},
		{"POST", "/publish", "application/json", fmt.Sprintf(`{"expected_draft_version":%d}`, d1)},
		{"POST", "/rollback", "application/json", `{"version":1}`},
		{"POST", "/preview-token", "application/json", `{}`},
		{"GET", "/versions", "", ""},
		{"GET", "/media", "", ""},
		{"GET", "/media/" + img, "", ""},
		{"POST", "/media/" + img + "/delete", "application/json", `{}`},
	}
	deny := func(label, token string, base string, edit func(*http.Request)) {
		for _, r := range routes {
			w := e.do(token, r.method, base+r.path, r.ctype, []byte(r.body), edit)
			if w.Code != 401 && w.Code != 403 && w.Code != 404 {
				t.Errorf("%s: %s %s -> %d (want 401/403/404) %s", label, r.method, r.path, w.Code, w.Body.String())
			}
		}
	}
	deny("tenant B token on store A1", f.tokens["b"], e.baseA, nil)
	deny("tenant A token on tenant B store", f.tokens["a"], e.baseB, nil)
	deny("tenant A token on same-tenant store A2 (no grant there)", f.tokens["a"], "/v1/admin/stores/"+f.storeA2+"/design", nil)
	deny("A2-only member on store A1 mutations are separate; no-integration member", f.tokens["revoked_grant"], e.baseA, nil)
	deny("spoofed tenant/store headers", f.tokens["b"], e.baseA, func(r *http.Request) {
		r.Header.Set("X-Tenant-ID", f.tenantA)
		r.Header.Set("X-Store-ID", f.storeA1)
		r.Header.Set("X-Forwarded-Host", "buyer.example")
	})
	deny("anonymous", "", e.baseA, nil)
	deny("expired session", f.tokens["expired"], e.baseA, nil)
	deny("revoked session", f.tokens["revoked"], e.baseA, nil)
	deny("buyer-audience session", f.tokens["buyer"], e.baseA, nil)
	// nothing above changed store A1
	if got := e.draft().Version; got != d1 {
		t.Fatalf("a refused request changed the draft: %d -> %d", d1, got)
	}
	if v := e.versions(); len(v.Items) != 1 {
		t.Fatalf("a refused request changed history: %+v", v)
	}

	// read-only viewer (integration:read on A1, via token a2): reads work, every write is 403
	for _, r := range routes {
		w := e.do(f.tokens["a2"], r.method, e.baseA+r.path, r.ctype, []byte(r.body), nil)
		if r.method == "GET" {
			if w.Code != 200 {
				t.Errorf("viewer GET %s -> %d", r.path, w.Code)
			}
		} else if w.Code != 403 {
			t.Errorf("viewer %s %s -> %d, want 403", r.method, r.path, w.Code)
		}
	}
	if w := e.upload(f.tokens["a2"], e.baseA, sdgPNG(t, 6)); w.Code != 403 {
		t.Errorf("viewer upload -> %d, want 403", w.Code)
	}

	// store B is an independent universe: own version counters, own media, A's image id is nothing to B
	if w := e.upload(f.tokens["b"], e.baseB, sdgPNG(t, 5)); w.Code != 200 {
		t.Fatalf("B upload: %d %s", w.Code, w.Body.String())
	}
	var bImg struct {
		ID string `json:"id"`
	}
	{
		w := e.upload(f.tokens["b"], e.baseB, sdgPNG(t, 5))
		_ = json.Unmarshal(w.Body.Bytes(), &bImg)
	}
	if bImg.ID == img {
		t.Fatalf("identical bytes in two stores share one media id %s: media is not store scoped", img)
	}
	if w := e.do(f.tokens["b"], "GET", e.baseB+"/media/"+img, "", nil, nil); w.Code != 404 {
		t.Errorf("store B read A's image id: %d", w.Code)
	}
	docB := map[string]any{"profile": map[string]any{"name": "Shop B", "accent_color": "#112233", "logo_image_id": img}}
	if w := e.json(f.tokens["b"], "PUT", e.baseB+"/draft", map[string]any{"expected_version": 0, "document": docB}); w.Code != 422 {
		t.Errorf("A's image referenced from store B draft: %d %s (want 422)", w.Code, w.Body.String())
	}
	docB["profile"].(map[string]any)["logo_image_id"] = bImg.ID
	if w := e.json(f.tokens["b"], "PUT", e.baseB+"/draft", map[string]any{"expected_version": 0, "document": docB}); w.Code != 200 {
		t.Fatalf("B draft: %d %s", w.Code, w.Body.String())
	}
	var vB sdgVersion
	if w := e.json(f.tokens["b"], "POST", e.baseB+"/publish", map[string]any{"expected_draft_version": 1}); w.Code != 200 {
		t.Fatalf("B publish: %d %s", w.Code, w.Body.String())
	} else {
		_ = json.Unmarshal(w.Body.Bytes(), &vB)
	}
	if vB.Version != 1 {
		t.Errorf("store B's first published version must be 1 (own counter), got %d", vB.Version)
	}
	pubA := e.published()
	_, _, raw := e.buyerGet("/v1/buyer/design/published", sdgOrigin(sdgSecondOrigin))
	var pubB sdgBuyerDoc
	_ = json.Unmarshal(raw, &pubB)
	if pubA.Document["profile"].(map[string]any)["name"] != "Gate Shop" || pubB.Document["profile"].(map[string]any)["name"] != "Shop B" {
		t.Errorf("buyer origins mixed documents: A=%v B=%v", pubA.Document["profile"], pubB.Document["profile"])
	}
	// the other store's image is not served on this origin
	if st, _, _ := e.buyerGet("/v1/buyer/media/s/"+img, sdgOrigin(sdgSecondOrigin)); st != 404 {
		t.Errorf("A's image on B's origin: %d", st)
	}
	if st, _, _ := e.buyerGet("/v1/buyer/media/s/"+bImg.ID, nil); st != 404 {
		t.Errorf("B's image on A's origin: %d", st)
	}
	if st, _, _ := e.buyerGet("/v1/buyer/media/s/"+img, nil); st != 200 {
		t.Errorf("A's image on A's origin: %d", st)
	}
	// preview tokens never cross stores
	var tokA, tokB struct {
		Token string `json:"token"`
	}
	e.must("POST", e.baseA+"/preview-token", struct{}{}, 200, &tokA)
	{
		w := e.json(f.tokens["b"], "POST", e.baseB+"/preview-token", struct{}{})
		if w.Code != 200 {
			t.Fatalf("B preview token: %d %s", w.Code, w.Body.String())
		}
		_ = json.Unmarshal(w.Body.Bytes(), &tokB)
	}
	if st, _, _ := e.buyerGet("/v1/buyer/design/preview", func(r *http.Request) { sdgOrigin(sdgSecondOrigin)(r); sdgPreview(tokA.Token)(r) }); st != 404 {
		t.Errorf("A's preview token on B's origin: %d", st)
	}
	if st, _, _ := e.buyerGet("/v1/buyer/design/preview", sdgPreview(tokB.Token)); st != 404 {
		t.Errorf("B's preview token on A's origin: %d", st)
	}
	if st, _, _ := e.buyerGet("/v1/buyer/design/preview", sdgPreview(tokA.Token)); st != 200 {
		t.Errorf("A's own preview token: %d", st)
	}

	// database layer: RLS repeats the store filter for the runtime role (positive control first)
	probe := func(tenant, store string) int {
		tx, err := f.runtime.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(context.Background())
		if _, err = tx.Exec(context.Background(), `SELECT set_config('app.tenant_id',$1,true),set_config('app.store_id',$2,true)`, tenant, store); err != nil {
			t.Fatal(err)
		}
		var n int
		if err = tx.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM design.documents WHERE store_id=$1)+(SELECT count(*) FROM design.published_versions WHERE store_id=$1)+(SELECT count(*) FROM design.store_media WHERE store_id=$1)`, f.storeA1).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if probe(f.tenantA, f.storeA1) == 0 {
		t.Fatal("RLS positive control: runtime role sees none of its own store's rows")
	}
	if n := probe(f.tenantB, f.storeB); n != 0 {
		t.Errorf("RLS: runtime role scoped to store B sees %d rows of store A1", n)
	}
	if n := probe(f.tenantA, f.storeA2); n != 0 {
		t.Errorf("RLS: runtime role scoped to store A2 sees %d rows of store A1", n)
	}
}

// sdgConcurrently runs n workers released together and returns their status codes.
func sdgConcurrently(n int, fn func(i int) int) []int {
	var wg sync.WaitGroup
	start := make(chan struct{})
	codes := make([]int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			codes[i] = fn(i)
		}(i)
	}
	close(start)
	wg.Wait()
	return codes
}

func sdgCount(codes []int, want int) (n int) {
	for _, c := range codes {
		if c == want {
			n++
		}
	}
	return n
}

func sdgNamed(name string) map[string]any {
	return map[string]any{"profile": map[string]any{"name": name, "accent_color": "#247965"}}
}

func sdgContiguous(t *testing.T, e *sdgEnv, label string) sdgVersions {
	t.Helper()
	v := e.versions()
	n := len(v.Items)
	for i, it := range v.Items { // newest first
		if it.Version != int64(n-i) {
			t.Errorf("%s: history is not contiguous 1..%d: %+v", label, n, v.Items)
			break
		}
	}
	if n > 0 && (v.LiveVersion == nil || *v.LiveVersion != int64(n)) {
		t.Errorf("%s: live version %v, want %d", label, v.LiveVersion, n)
	}
	return v
}

func sdgRaces(t *testing.T, e *sdgEnv) {
	tok := e.f.tokens["a"]
	// R1: N concurrent saves with the same expected_version: one winner.
	e.save(sdgNamed("seed"))
	cur := e.draft().Version
	codes := sdgConcurrently(10, func(i int) int {
		return e.json(tok, "PUT", e.baseA+"/draft", map[string]any{"expected_version": cur, "document": sdgNamed(fmt.Sprintf("racer-%d", i))}).Code
	})
	if sdgCount(codes, 200) != 1 || sdgCount(codes, 409) != 9 {
		t.Errorf("R1 concurrent saves: want 1x200 + 9x409, got %v", codes)
	}
	if e.draft().Version != cur+1 {
		t.Errorf("R1: draft version moved by %d, want exactly 1", e.draft().Version-cur)
	}

	// R2: N concurrent publishes of the same draft version: one winner, one history row, one audit row.
	cur = e.draft().Version
	auditBefore := countRows(t, e.f.owner, `SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='design.published'`, e.f.storeA1)
	codes = sdgConcurrently(10, func(int) int {
		return e.json(tok, "POST", e.baseA+"/publish", map[string]any{"expected_draft_version": cur}).Code
	})
	if sdgCount(codes, 200) != 1 || sdgCount(codes, 409) != 9 {
		t.Errorf("R2 concurrent publish: want 1x200 + 9x409, got %v", codes)
	}
	v := sdgContiguous(t, e, "R2")
	if len(v.Items) != 1 {
		t.Errorf("R2: %d history rows, want 1", len(v.Items))
	}
	if got := countRows(t, e.f.owner, `SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='design.published'`, e.f.storeA1) - auditBefore; got != 1 {
		t.Errorf("R2: %d audit rows for one publish", got)
	}

	// R3: publish vs save on the same version. A publish that wins must publish the document of the version it named,
	// never the newer save (no torn read), whatever the interleaving. 12 rounds.
	for round := 0; round < 12; round++ {
		cur = e.save(sdgNamed(fmt.Sprintf("round-%d-base", round)))
		var pub, put int
		sdgConcurrently(2, func(i int) int {
			if i == 0 {
				pub = e.json(tok, "POST", e.baseA+"/publish", map[string]any{"expected_draft_version": cur}).Code
				return pub
			}
			put = e.json(tok, "PUT", e.baseA+"/draft", map[string]any{"expected_version": cur, "document": sdgNamed(fmt.Sprintf("round-%d-newer", round))}).Code
			return put
		})
		if put != 200 && put != 409 || pub != 200 && pub != 409 {
			t.Fatalf("R3 round %d: publish=%d save=%d", round, pub, put)
		}
		live := e.published()
		wantName := fmt.Sprintf("round-%d-base", round)
		if pub == 200 && live.Document["profile"].(map[string]any)["name"] != wantName {
			t.Errorf("R3 round %d: publish of draft v%d went live with %v (torn read)", round, cur, live.Document["profile"])
		}
		if pub == 409 && put == 409 {
			t.Errorf("R3 round %d: both writers lost", round)
		}
		sdgContiguous(t, e, fmt.Sprintf("R3 round %d", round))
	}

	// R4: concurrent rollbacks to version 1 and concurrent publish: history stays gap-free and unique, live == newest row.
	before := len(e.versions().Items)
	codes = sdgConcurrently(8, func(i int) int {
		if i%4 == 3 {
			return e.json(tok, "POST", e.baseA+"/publish", map[string]any{"expected_draft_version": e.draft().Version}).Code
		}
		return e.json(tok, "POST", e.baseA+"/rollback", map[string]any{"version": 1}).Code
	})
	for _, c := range codes {
		if c != 200 && c != 409 {
			t.Errorf("R4: unexpected status %v", codes)
			break
		}
	}
	v = sdgContiguous(t, e, "R4")
	if len(v.Items) != before+sdgCount(codes, 200) {
		t.Errorf("R4: %d successes but history grew %d -> %d", sdgCount(codes, 200), before, len(v.Items))
	}
	// the buyer sees exactly the newest history row
	var newestDoc string
	if err := e.f.owner.QueryRow(context.Background(), `SELECT document::text FROM design.published_versions WHERE store_id=$1 ORDER BY version DESC LIMIT 1`, e.f.storeA1).Scan(&newestDoc); err != nil {
		t.Fatal(err)
	}
	var want map[string]any
	_ = json.Unmarshal([]byte(newestDoc), &want)
	if got := e.published(); got.Version != int64(len(v.Items)) || !reflect.DeepEqual(got.Document, want) {
		t.Errorf("R4: buyer sees v%d %v, newest history row is v%d %v", got.Version, got.Document["profile"], len(v.Items), want["profile"])
	}

	// R5: media cap under concurrency: 70 distinct uploads -> exactly 60 stored.
	mustExec(t, e.f.owner, `DELETE FROM design.store_media WHERE store_id=$1`, e.f.storeA1)
	codes = sdgConcurrently(70, func(i int) int { return e.upload(tok, e.baseA, sdgPNG(t, i+1)).Code })
	rows := countRows(t, e.f.owner, `SELECT count(*) FROM design.store_media WHERE store_id=$1`, e.f.storeA1)
	if rows != 60 || sdgCount(codes, 200) != 60 || sdgCount(codes, 409) != 10 {
		t.Errorf("R5 media cap: rows=%d, 200s=%d, 409s=%d, all=%v (want 60/60/10)", rows, sdgCount(codes, 200), sdgCount(codes, 409), codes)
	}

}

func sdgDeleteVsReference(t *testing.T, e *sdgEnv) {
	tok := e.f.tokens["a"]
	var cur int64
	// Delete vs a draft save that references the image. Never both succeed, never a dangling reference.
	mustExec(t, e.f.owner, `DELETE FROM design.store_media WHERE store_id=$1`, e.f.storeA1)
	for round := 0; round < 15; round++ {
		img := e.image(e.baseA, tok, 100+round)
		cur = e.draft().Version
		doc := sdgNamed("ref")
		doc["profile"].(map[string]any)["logo_image_id"] = img
		var save, del int
		sdgConcurrently(2, func(i int) int {
			if i == 0 {
				save = e.json(tok, "PUT", e.baseA+"/draft", map[string]any{"expected_version": cur, "document": doc}).Code
				return save
			}
			del = e.json(tok, "POST", e.baseA+"/media/"+img+"/delete", struct{}{}).Code
			return del
		})
		if save == 200 && del == 200 {
			t.Errorf("R6 round %d: the save referencing %s and the delete of %s both succeeded", round, img, img)
		}
		gone := countRows(t, e.f.owner, `SELECT count(*) FROM design.store_media WHERE store_id=$1 AND id=$2`, e.f.storeA1, img) == 0
		var ref int
		_ = e.f.owner.QueryRow(context.Background(), `SELECT count(*) FROM design.documents WHERE store_id=$1 AND document::text LIKE '%'||$2||'%'`, e.f.storeA1, img).Scan(&ref)
		if gone && ref > 0 {
			t.Errorf("R6 round %d: draft references deleted image %s (save=%d delete=%d)", round, img, save, del)
		}
		for _, c := range []int{save, del} {
			if c >= 500 {
				t.Errorf("R6 round %d: server error save=%d delete=%d", round, save, del)
			}
		}
		// clear the reference so the next round starts clean
		e.save(sdgNamed("clear"))
	}
}

func sdgPreviewTokens(t *testing.T, e *sdgEnv) {
	f := e.f
	tok := f.tokens["a"]
	// no saved draft yet: no token
	if w := e.json(tok, "POST", e.baseA+"/preview-token", struct{}{}); w.Code == 200 {
		t.Errorf("a preview token was issued for a store with no saved draft: %s", w.Body.String())
	}
	v1 := e.save(sdgNamed("preview one"))
	var tk struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	e.must("POST", e.baseA+"/preview-token", struct{}{}, 200, &tk)
	if len(tk.Token) < 40 {
		t.Errorf("token too short for 256-bit entropy: %d chars", len(tk.Token))
	}
	if ttl := time.Until(tk.ExpiresAt); !tk.ExpiresAt.IsZero() && (ttl > 15*time.Minute+10*time.Second || ttl < 14*time.Minute) {
		t.Errorf("token lifetime %v, want 15 minutes", ttl)
	}
	var tk2 struct {
		Token string `json:"token"`
	}
	e.must("POST", e.baseA+"/preview-token", struct{}{}, 200, &tk2)
	if tk2.Token == tk.Token {
		t.Error("two issued tokens are identical")
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM design.preview_tokens t WHERE t::text LIKE '%'||$1||'%'`, tk.Token); n != 0 {
		t.Errorf("the plaintext token is stored at rest (%d rows)", n)
	}
	status, header, body := e.buyerGet("/v1/buyer/design/preview", sdgPreview(tk.Token))
	if status != 200 || header.Get("Cache-Control") != "no-store" || !strings.Contains(string(body), `"version":`+fmt.Sprint(v1)) || !strings.Contains(string(body), "preview one") {
		t.Fatalf("preview read: %d %v %s", status, header, body)
	}
	// valid published read never shows the draft, with or without the preview header
	if _, _, b := e.buyerGet("/v1/buyer/design/published", sdgPreview(tk.Token)); strings.Contains(string(b), "preview one") {
		t.Errorf("a preview token made the published route serve the draft: %s", b)
	}
	// the token goes in a header only; query, bearer and cookie forms are not accepted
	for label, edit := range map[string]func(*http.Request){
		"query":  func(r *http.Request) { r.URL.RawQuery = "preview=" + tk.Token },
		"bearer": func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+tk.Token) },
		"cookie": func(r *http.Request) { r.Header.Set("Cookie", "preview="+tk.Token) },
	} {
		if st, _, b := e.buyerGet("/v1/buyer/design/preview", edit); st == 200 || strings.Contains(string(b), "preview one") {
			t.Errorf("token via %s was accepted: %d", label, st)
		}
	}
	if st, _, _ := e.buyerGet("/v1/buyer/design/preview", func(r *http.Request) {
		r.Header.Add(sdgPreviewHeader, tk.Token)
		r.Header.Add(sdgPreviewHeader, tk2.Token)
	}); st == 200 {
		t.Errorf("two token headers accepted: %d", st)
	}
	// malformed / unknown / empty tokens are the same 404
	for _, bad := range []string{"", "x", strings.Repeat("A", 43), strings.Repeat("A", 500), "<script>", tk.Token + "x", tk.Token[:len(tk.Token)-1], strings.ToLower(tk.Token), strings.ToUpper(tk.Token)} {
		if bad == tk.Token {
			continue
		}
		st, _, b := e.buyerGet("/v1/buyer/design/preview", sdgPreview(bad))
		if st != 404 || strings.Contains(string(b), "preview one") {
			t.Errorf("token %q: %d (want 404)", bad, st)
		}
	}
	// the miss is indistinguishable from an unpublished store (no oracle)
	_, _, miss := e.buyerGet("/v1/buyer/design/preview", sdgPreview("nope"))
	_, _, unpub := e.buyerGet("/v1/buyer/design/preview", func(r *http.Request) { sdgOrigin(sdgUnpubOrigin)(r); sdgPreview(tk.Token)(r) })
	var a, b map[string]any
	_ = json.Unmarshal(miss, &a)
	_ = json.Unmarshal(unpub, &b)
	delete(a, "request_id")
	delete(b, "request_id")
	if !reflect.DeepEqual(a, b) {
		t.Errorf("token miss and unpublished-origin answers differ: %s vs %s", miss, unpub)
	}

	// staleness: saving the draft after issue kills the older token; the new token binds to the new version
	v2 := e.save(sdgNamed("preview two"))
	if st, _, _ := e.buyerGet("/v1/buyer/design/preview", sdgPreview(tk.Token)); st != 404 {
		t.Errorf("token issued at draft v%d still works at v%d: %d", v1, v2, st)
	}
	if st, _, _ := e.buyerGet("/v1/buyer/design/preview", sdgPreview(tk2.Token)); st != 404 {
		t.Errorf("second token of v%d still works at v%d: %d", v1, v2, st)
	}
	var tk3 struct {
		Token string `json:"token"`
	}
	e.must("POST", e.baseA+"/preview-token", struct{}{}, 200, &tk3)
	if st, _, b := e.buyerGet("/v1/buyer/design/preview", sdgPreview(tk3.Token)); st != 200 || !strings.Contains(string(b), "preview two") {
		t.Errorf("fresh token: %d %s", st, b)
	}
	// publish does not change the draft version: a token of the saved draft stays usable (token grants a draft read only)
	e.publish(v2)
	if st, _, _ := e.buyerGet("/v1/buyer/design/preview", sdgPreview(tk3.Token)); st != 200 {
		t.Errorf("publishing must not invalidate the token of the unchanged draft: %d", st)
	}

	// expiry boundary: 1 minute left works, 1 second past 15 minutes does not (the CHECK pins the 15-minute ceiling)
	mustExec(t, f.owner, `UPDATE design.preview_tokens SET created_at=created_at-interval '14 minutes', expires_at=expires_at-interval '14 minutes'`)
	if st, _, _ := e.buyerGet("/v1/buyer/design/preview", sdgPreview(tk3.Token)); st != 200 {
		t.Errorf("token at minute 14 refused: %d", st)
	}
	mustExec(t, f.owner, `UPDATE design.preview_tokens SET created_at=created_at-interval '2 minutes', expires_at=expires_at-interval '2 minutes'`)
	if st, _, _ := e.buyerGet("/v1/buyer/design/preview", sdgPreview(tk3.Token)); st != 404 {
		t.Errorf("token at minute 16 still works: %d", st)
	}
	if _, err := f.owner.Exec(context.Background(), `UPDATE design.preview_tokens SET expires_at=created_at+interval '16 minutes'`); err == nil {
		t.Errorf("a token lifetime above 15 minutes can be stored")
	}

	// unpublishing the storefront kills preview too
	var tk4 struct {
		Token string `json:"token"`
	}
	e.must("POST", e.baseA+"/preview-token", struct{}{}, 200, &tk4)
	mustExec(t, f.owner, `UPDATE control.storefront_publications SET published=false WHERE store_id=$1`, f.storeA1)
	st, _, _ := e.buyerGet("/v1/buyer/design/preview", sdgPreview(tk4.Token))
	mustExec(t, f.owner, `UPDATE control.storefront_publications SET published=true WHERE store_id=$1`, f.storeA1)
	if st != 404 {
		t.Errorf("preview of an unpublished storefront: %d", st)
	}
	// issuing is a manage-permission action
	if w := e.json(f.tokens["a2"], "POST", e.baseA+"/preview-token", struct{}{}); w.Code != 403 {
		t.Errorf("read-only member issued a preview token: %d", w.Code)
	}
}

type sdgField struct {
	path []any
	max  int
}

func sdgValidation(t *testing.T, e *sdgEnv) {
	f := e.f
	img := e.image(e.baseA, f.tokens["a"], 5)
	other := e.image(e.baseB, f.tokens["b"], 7) // an image of ANOTHER store
	base := sdgFullDoc(img, img, img, img)
	e.accept(base, "the complete valid document")
	if t.Failed() {
		t.Fatal("the complete valid document is refused; nothing below is meaningful")
	}
	// round trip: the server returns what was stored, normalised and still valid
	rt := e.draft()
	if e.save(rt.Document) == 0 {
		t.Fatal("the returned draft cannot be saved back")
	}

	html := []string{`<script>alert(1)</script>`, `<img src=x onerror=alert(1)>`, `"><svg onload=alert(1)>`}
	urls := []string{`javascript:alert(1)`, `JaVaScRiPt:alert(1)`, `vbscript:msgbox(1)`, `data:text/html,<script>alert(1)</script>`, `http://insecure.example/`, `//evil.example/x`, `ftp://x.example/`, `https://ok.example/"><script>alert(1)</script>`}
	md := []string{`<script>alert(1)</script>`, `hello <b>x</b>`, `<img src=x onerror=alert(1)>`, `a < b`, `[x](javascript:alert(1))`, `[x](JAVASCRIPT:alert(1))`, `[x](http://a.example)`, `[x](//a.example)`, `[x](data:text/html;base64,PHNjcmlwdD5hbGVydCgxKTwvc2NyaXB0Pg==)`, `[x](vbscript:msgbox(1))`}

	text := [][]any{
		{"profile", "name"}, {"profile", "tagline"}, {"profile", "announcement"}, {"profile", "contact", "address"}, {"profile", "contact", "phone"}, {"profile", "contact", "email"},
		{"nav", "header", 0, "label"}, {"nav", "header", 4, "label"}, {"nav", "footer", 0, "label"}, {"nav", "footer", 2, "label"},
		{"home", "sections", 0, "heading"}, {"home", "sections", 0, "subheading"}, {"home", "sections", 0, "cta_label"},
		{"home", "sections", 1, "heading"}, {"home", "sections", 2, "heading"}, {"home", "sections", 3, "heading"}, {"home", "sections", 4, "heading"},
		{"pages", 0, "title"}, {"pages", 1, "title"},
	}
	slugs := [][]any{
		{"nav", "header", 2, "target"}, {"nav", "header", 3, "target"}, {"nav", "footer", 0, "target"}, {"nav", "footer", 2, "target"},
		{"home", "sections", 0, "cta_target"}, {"home", "sections", 1, "collection_slug"}, {"pages", 0, "slug"},
	}
	urlFields := [][]any{{"profile", "contact", "line_url"}, {"profile", "contact", "facebook_url"}, {"profile", "contact", "instagram_url"}, {"nav", "header", 4, "target"}, {"nav", "footer", 1, "target"}}
	mdFields := [][]any{{"home", "sections", 3, "body"}, {"home", "sections", 4, "body"}, {"pages", 0, "body"}, {"pages", 1, "body"}}
	idFields := [][]any{{"profile", "logo_image_id"}, {"profile", "favicon_image_id"}, {"home", "sections", 0, "image_id"}, {"home", "sections", 4, "image_id"}}

	if e.t.Name() == "TestStoreDesignGate/SD04b_plain_text_html" {
		for _, p := range text {
			for _, x := range html {
				e.reject(sdgSet(base, p, x), sdgPathString(p), false, x)
			}
		}
		return
	}
	for _, p := range slugs {
		for _, x := range append(append([]string{}, html...), `javascript:alert(1)`, `Bad Slug`, `../x`, `UPPER`) {
			e.reject(sdgSet(base, p, x), sdgPathString(p), false, x)
		}
	}
	for _, p := range urlFields {
		for _, x := range urls {
			e.reject(sdgSet(base, p, x), sdgPathString(p), false, x)
		}
	}
	for _, p := range mdFields {
		for _, x := range md {
			e.reject(sdgSet(base, p, x), sdgPathString(p), false, x)
		}
	}
	for _, p := range idFields {
		for _, x := range []any{`<script>alert(1)</script>`, `not-a-uuid`, `99999999-9999-4999-8999-999999999999`, other, 7, true} {
			e.reject(sdgSet(base, p, x), sdgPathString(p), false, fmt.Sprint(x))
		}
	}
	// required images cannot be null or absent
	for _, p := range [][]any{{"home", "sections", 0, "image_id"}, {"home", "sections", 4, "image_id"}} {
		e.reject(sdgSet(base, p, nil), sdgPathString(p), false, "")
		e.reject(sdgSet(base, p, sdgDelete), sdgPathString(p), false, "")
	}
	// enums and colours
	for _, c := range []struct {
		p []any
		v []any
	}{
		{[]any{"profile", "accent_color"}, []any{"red", "#12345", "#12345g", "247965", "#247965;}</style>", "url(javascript:alert(1))", "#2479655", nil, 5}},
		{[]any{"nav", "header", 0, "kind"}, []any{"javascript", "Home", "", "all_products\n", nil, 3}},
		{[]any{"nav", "footer", 0, "kind"}, []any{"home", "all_products", "url2"}}, // footer kinds are page|url|collection
		{[]any{"home", "sections", 2, "sort"}, []any{"random", "NEWEST", "price", nil, 1}},
		{[]any{"home", "sections", 4, "image_side"}, []any{"top", "Left", nil}},
		{[]any{"home", "sections", 0, "cta_kind"}, []any{"url", "home", "javascript"}},
		{[]any{"home", "sections", 0, "type"}, []any{"script", "Hero", "", nil, "html", "custom_css"}},
	} {
		for _, v := range c.v {
			e.reject(sdgSet(base, c.p, v), sdgPathString(c.p), false, fmt.Sprint(v))
		}
	}
	// numeric ranges 4..24 and 4..48
	for _, c := range []struct {
		p       []any
		lo, hi  int
		invalid []any
	}{
		{[]any{"home", "sections", 1, "limit"}, 4, 24, []any{3, 25, 0, -1, "8", 8.5, nil}},
		{[]any{"home", "sections", 2, "limit"}, 4, 48, []any{3, 49, 0, -1, "12", 12.5, nil}},
	} {
		for _, v := range c.invalid {
			e.reject(sdgSet(base, c.p, v), sdgPathString(c.p), false, fmt.Sprint(v))
		}
		e.accept(sdgSet(base, c.p, c.lo), "limit lower bound")
		e.accept(sdgSet(base, c.p, c.hi), "limit upper bound")
	}
	// cta_target is only for collection/page; page targets must be pages of this document; slugs unique
	e.reject(sdgSet(base, []any{"home", "sections", 0, "cta_kind"}, "all_products"), "home.sections[0].cta_target", false, "")
	e.reject(sdgSet(base, []any{"home", "sections", 0, "cta_target"}, "nope"), "home.sections[0].cta_target", false, "")
	e.reject(sdgSet(base, []any{"nav", "header", 3, "target"}, "nope"), "nav.header[3].target", false, "")
	e.reject(sdgSet(base, []any{"pages", 1, "slug"}, "about"), "pages[1]", true, "")
	// unknown keys at every object level
	for _, c := range []struct {
		p      []any
		prefix string
	}{
		{[]any{"x_unknown"}, ""}, {[]any{"profile", "x_unknown"}, "profile"}, {[]any{"profile", "contact", "x_unknown"}, "profile.contact"},
		{[]any{"nav", "x_unknown"}, "nav"}, {[]any{"nav", "header", 0, "x_unknown"}, "nav.header[0]"}, {[]any{"nav", "footer", 1, "x_unknown"}, "nav.footer[1]"},
		{[]any{"home", "x_unknown"}, "home"}, {[]any{"home", "sections", 0, "x_unknown"}, "home.sections[0]"}, {[]any{"home", "sections", 2, "x_unknown"}, "home.sections[2]"},
		{[]any{"pages", 0, "x_unknown"}, "pages[0]"}, {[]any{"profile", "css"}, "profile"}, {[]any{"home", "sections", 3, "html"}, "home.sections[3]"},
	} {
		e.reject(sdgSet(base, c.p, "x"), sdgPathString(c.p), false, "")
	}
	// a field of another section type is unknown for this type
	e.reject(sdgSet(base, []any{"home", "sections", 2, "body"}, "x"), "home.sections[2].body", false, "")
	// required leaves
	for _, p := range [][]any{{"profile", "name"}, {"profile", "accent_color"}, {"nav", "header", 0, "label"}, {"nav", "header", 0, "kind"}, {"home", "sections", 0, "type"}, {"pages", 0, "slug"}, {"pages", 0, "title"}, {"home", "sections", 1, "collection_slug"}, {"home", "sections", 1, "limit"}, {"home", "sections", 2, "sort"}, {"home", "sections", 4, "image_side"}} {
		e.reject(sdgSet(base, p, sdgDelete), sdgPathString(p), false, "")
	}
	// wrong JSON types
	for _, p := range [][]any{{"profile", "name"}, {"pages", 0, "body"}, {"home", "sections", 3, "body"}} {
		for _, v := range []any{5, true, []any{"a"}, map[string]any{"a": 1}} {
			e.reject(sdgSet(base, p, v), sdgPathString(p), false, "")
		}
	}
	e.reject(sdgSet(base, []any{"nav"}, "x"), "nav", false, "")
	e.reject(sdgSet(base, []any{"home", "sections"}, "x"), "home.sections", false, "")

	// caps: ASCII and CJK (characters, not bytes), exactly at the cap accepted, one over refused
	caps := []sdgField{
		{[]any{"profile", "name"}, 60}, {[]any{"profile", "tagline"}, 120}, {[]any{"profile", "announcement"}, 140}, {[]any{"profile", "contact", "address"}, 200}, {[]any{"profile", "contact", "phone"}, 30},
		{[]any{"nav", "header", 0, "label"}, 30}, {[]any{"nav", "footer", 0, "label"}, 30},
		{[]any{"home", "sections", 0, "heading"}, 80}, {[]any{"home", "sections", 0, "subheading"}, 160}, {[]any{"home", "sections", 0, "cta_label"}, 24},
		{[]any{"home", "sections", 3, "heading"}, 80}, {[]any{"home", "sections", 3, "body"}, 4000}, {[]any{"home", "sections", 4, "body"}, 2000},
		{[]any{"pages", 0, "title"}, 80}, {[]any{"pages", 0, "body"}, 20000},
	}
	for _, c := range caps {
		for _, unit := range []string{"a", "店"} {
			p := sdgPathString(c.path)
			if c.path[len(c.path)-1] == "phone" && unit == "店" {
				continue // a phone number is not CJK text
			}
			ch := unit
			if c.path[len(c.path)-1] == "phone" {
				ch = "1"
			}
			e.accept(sdgSet(base, c.path, strings.Repeat(ch, c.max)), fmt.Sprintf("%s exactly %d x %q", p, c.max, ch))
			e.reject(sdgSet(base, c.path, strings.Repeat(ch, c.max+1)), p, false, "")
		}
	}
	// list limits: header 8, footer 12, sections 20, pages 20
	hdr := func(i int) any {
		return map[string]any{"label": fmt.Sprint("l", i), "kind": "all_products", "target": nil}
	}
	ftr := func(i int) any {
		return map[string]any{"label": fmt.Sprint("l", i), "kind": "url", "target": "https://example.com/x"}
	}
	sec := func(i int) any { return map[string]any{"type": "rich_text", "heading": nil, "body": ""} }
	pg := func(i int) any { return map[string]any{"slug": fmt.Sprint("p", i), "title": "t", "body": ""} }
	// the nav/cta targets of `base` reference pages "about" / collections; lists replaced wholesale keep the document valid
	nobase := sdgSet(sdgSet(sdgSet(base, []any{"nav", "header"}, []any{}), []any{"nav", "footer"}, []any{}), []any{"home", "sections"}, []any{})
	nobase = sdgSet(nobase, []any{"pages"}, []any{})
	listDoc := func(path []any, n int, mk func(i int) any) map[string]any {
		items := make([]any, n)
		for i := range items {
			items[i] = mk(i)
		}
		return sdgSet(nobase, path, items)
	}
	for _, c := range []struct {
		path []any
		max  int
		mk   func(int) any
	}{{[]any{"nav", "header"}, 8, hdr}, {[]any{"nav", "footer"}, 12, ftr}, {[]any{"home", "sections"}, 20, sec}, {[]any{"pages"}, 20, pg}} {
		e.accept(listDoc(c.path, c.max, c.mk), sdgPathString(c.path)+" at its limit")
		e.reject(listDoc(c.path, c.max+1, c.mk), sdgPathString(c.path), true, "")
	}
	// benign punctuation is data, not markup: stored verbatim (escaping happens at render time)
	punct := `Tom & Jerry's "best" shop (台灣)`
	e.accept(sdgSet(base, []any{"profile", "name"}, punct), "punctuation in the name")
	if got := e.draft().Document["profile"].(map[string]any)["name"]; got != punct {
		t.Errorf("name stored as %q, want %q", got, punct)
	}
	good := "Para **bold** and *italic* and [a link](https://example.com/p?x=1&y=2).\n\n- one\n- two\n\n台灣 \"quoted\" 'single' & more"
	e.accept(sdgSet(base, []any{"pages", 0, "body"}, good), "valid restricted markdown")
	e.accept(sdgSet(base, []any{"pages", 0, "body"}, ""), "empty markdown body")
	// final state: all refused documents left the draft exactly as the last accepted one
	if d := e.draft(); d.Document["pages"].([]any)[0].(map[string]any)["body"] != "" {
		t.Errorf("draft after the sweep is not the last accepted document")
	}
}

func sdgMedia(t *testing.T, e *sdgEnv) {
	f := e.f
	tok := f.tokens["a"]
	// sniffing: the bytes decide, never the filename or declared type
	var jpg bytes.Buffer
	_ = jpeg.Encode(&jpg, image.NewRGBA(image.Rect(0, 0, 4, 4)), nil)
	if w := e.upload(tok, e.baseA, jpg.Bytes()); w.Code != 200 || !strings.Contains(w.Body.String(), "image/jpeg") {
		t.Errorf("jpeg: %d %s", w.Code, w.Body.String())
	}
	for label, data := range map[string][]byte{
		"svg":         []byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"/>`),
		"html":        []byte(`<html><script>alert(1)</script></html>`),
		"gif":         []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"),
		"png magic":   {0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A},
		"empty":       {},
		"php in text": []byte("<?php system($_GET['c']); ?>"),
	} {
		if w := e.upload(tok, e.baseA, data); w.Code != 422 && w.Code != 400 {
			t.Errorf("%s accepted as media: %d %s", label, w.Code, w.Body.String())
		}
	}
	// size cap 2 MiB
	noise := image.NewRGBA(image.Rect(0, 0, 800, 800))
	rand.New(rand.NewSource(1)).Read(noise.Pix)
	var big bytes.Buffer
	_ = png.Encode(&big, noise)
	if big.Len() <= 2<<20 {
		t.Fatalf("fixture image is only %d bytes", big.Len())
	}
	if w := e.upload(tok, e.baseA, big.Bytes()); w.Code != 413 && w.Code != 422 && w.Code != 400 {
		t.Errorf("oversize upload: %d", w.Code)
	}
	// non-multipart and multi-file bodies
	if w := e.do(tok, "POST", e.baseA+"/media", "application/json", []byte(`{}`), nil); w.Code != 415 && w.Code != 422 && w.Code != 400 {
		t.Errorf("json upload: %d", w.Code)
	}
	// 60 per store (the jpeg above is the first)
	for i := 0; i < 59; i++ {
		if w := e.upload(tok, e.baseA, sdgPNG(t, 10+i)); w.Code != 200 {
			t.Fatalf("image %d: %d %s", i+2, w.Code, w.Body.String())
		}
	}
	if w := e.upload(tok, e.baseA, sdgPNG(t, 200)); w.Code != 409 {
		t.Errorf("61st image: %d, want 409", w.Code)
	}
	if w := e.upload(tok, e.baseA, sdgPNG(t, 10)); w.Code != 200 { // identical bytes dedupe, even at the cap
		t.Errorf("re-upload of identical bytes at the cap: %d", w.Code)
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM design.store_media WHERE store_id=$1`, f.storeA1); n != 60 {
		t.Errorf("%d media rows, want 60", n)
	}
	// the cap is per store
	if w := e.upload(f.tokens["b"], e.baseB, sdgPNG(t, 200)); w.Code != 200 {
		t.Errorf("store B upload while A is full: %d", w.Code)
	}
	// deleting frees a slot
	rows, _ := f.owner.Query(context.Background(), `SELECT id::text FROM design.store_media WHERE store_id=$1 ORDER BY created_at LIMIT 3`, f.storeA1)
	var ids []string
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()
	if w := e.json(tok, "POST", e.baseA+"/media/"+ids[0]+"/delete", struct{}{}); w.Code != 200 {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	if w := e.upload(tok, e.baseA, sdgPNG(t, 200)); w.Code != 200 {
		t.Errorf("upload after delete: %d", w.Code)
	}
	if w := e.json(tok, "POST", e.baseA+"/media/"+ids[0]+"/delete", struct{}{}); w.Code != 404 {
		t.Errorf("second delete: %d, want 404", w.Code)
	}
	if w := e.do(tok, "GET", e.baseA+"/media/"+ids[0], "", nil, nil); w.Code != 404 {
		t.Errorf("deleted image still readable: %d", w.Code)
	}

	// delete-while-referenced, from every field that can reference an image
	mustExec(t, f.owner, `DELETE FROM design.store_media WHERE store_id=$1`, f.storeA1)
	for _, ref := range [][]any{{"profile", "logo_image_id"}, {"profile", "favicon_image_id"}, {"home", "sections", 0, "image_id"}, {"home", "sections", 4, "image_id"}} {
		spare := e.image(e.baseA, tok, 300)
		img := e.image(e.baseA, tok, 301)
		doc := sdgFullDoc(spare, spare, spare, spare)
		doc["profile"].(map[string]any)["favicon_image_id"] = nil
		doc = sdgSet(doc, ref, img)
		e.save(doc)
		if w := e.json(tok, "POST", e.baseA+"/media/"+img+"/delete", struct{}{}); w.Code != 409 {
			t.Errorf("image referenced by the saved draft at %s deleted: %d", sdgPathString(ref), w.Code)
		}
		d := e.draft().Version
		e.publish(d)
		e.save(sdgNamed("unreferenced draft")) // the draft no longer references it, the LIVE version still does
		if w := e.json(tok, "POST", e.baseA+"/media/"+img+"/delete", struct{}{}); w.Code != 409 {
			t.Errorf("image referenced only by the live version (%s) deleted: %d", sdgPathString(ref), w.Code)
		}
		// buyers can still load it
		if st, _, _ := e.buyerGet("/v1/buyer/media/s/"+img, nil); st != 200 {
			t.Errorf("live image not served: %d", st)
		}
		e.publish(e.draft().Version) // live version drops the reference
		if w := e.json(tok, "POST", e.baseA+"/media/"+img+"/delete", struct{}{}); w.Code != 200 {
			t.Errorf("unreferenced image not deletable: %d %s", w.Code, w.Body.String())
		}
		if st, _, _ := e.buyerGet("/v1/buyer/media/s/"+img, nil); st != 404 {
			t.Errorf("deleted image still served to buyers: %d", st)
		}
		// rolling back to the version that referenced the deleted image must not resurrect a dangling reference
		live := e.versions()
		oldest := live.Items[len(live.Items)-1].Version
		if w := e.json(tok, "POST", e.baseA+"/rollback", map[string]any{"version": oldest}); w.Code == 200 {
			t.Errorf("rollback to v%d (references deleted image %s) succeeded; the live store would render a broken image", oldest, img)
		}
		if got := e.published(); strings.Contains(fmt.Sprint(got.Document), img) {
			t.Errorf("live document references deleted image %s", img)
		}
		mustExec(t, f.owner, `TRUNCATE design.preview_tokens, design.published_versions, design.documents`)
		mustExec(t, f.owner, `DELETE FROM design.store_media WHERE store_id=$1`, f.storeA1)
	}

	// served bytes: type from the sniff, hardening headers, immutable, authenticated GET returns the same bytes
	img := e.image(e.baseA, tok, 9)
	e.save(map[string]any{"profile": map[string]any{"name": "m", "accent_color": "#247965", "logo_image_id": img}})
	e.publish(e.draft().Version)
	st, h, b := e.buyerGet("/v1/buyer/media/s/"+img, nil)
	if st != 200 || h.Get("Content-Type") != "image/png" || h.Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(h.Get("Content-Security-Policy"), "sandbox") || !strings.Contains(h.Get("Cache-Control"), "immutable") || !bytes.Equal(b, sdgPNG(t, 9)) {
		t.Errorf("buyer media: %d %v", st, h)
	}
	if w := e.do(tok, "GET", e.baseA+"/media/"+img, "", nil, nil); w.Code != 200 || !bytes.Equal(w.Body.Bytes(), sdgPNG(t, 9)) {
		t.Errorf("admin media bytes: %d", w.Code)
	}
	if w := e.do(tok, "GET", e.baseA+"/media/not-a-uuid", "", nil, nil); w.Code != 404 && w.Code != 422 && w.Code != 400 {
		t.Errorf("bad image id: %d", w.Code)
	}
	// admin list: ids only, no foreign rows
	var list struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	e.must("GET", e.baseA+"/media", nil, 200, &list)
	if len(list.Items) != 1 || list.Items[0].ID != img {
		t.Errorf("media list: %+v", list)
	}
}

func sdgHistory(t *testing.T, e *sdgEnv) {
	f := e.f
	marker := "audit-marker-" + randomUUID()
	auditN := func(action string) int {
		return countRows(t, f.owner, `SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action=$2`, f.storeA1, action)
	}
	pubBefore, rbBefore := auditN("design.published"), auditN("design.rolled_back")
	d1 := e.save(sdgNamed("one " + marker))
	v1 := e.publish(d1)
	d2 := e.save(sdgNamed("two"))
	v2 := e.publish(d2)
	if v1.Version != 1 || v2.Version != 2 || v1.Kind != "publish" || v1.SourceVersion != d1 {
		t.Fatalf("v1=%+v v2=%+v", v1, v2)
	}
	type row struct {
		Version, Source int64
		Kind, Doc, At   string
	}
	snapshot := func() []row {
		rs, err := f.owner.Query(context.Background(), `SELECT version,source_version,kind,document::text,published_at::text FROM design.published_versions WHERE store_id=$1 ORDER BY version`, f.storeA1)
		if err != nil {
			t.Fatal(err)
		}
		defer rs.Close()
		var out []row
		for rs.Next() {
			var r row
			_ = rs.Scan(&r.Version, &r.Source, &r.Kind, &r.Doc, &r.At)
			out = append(out, r)
		}
		return out
	}
	before := snapshot()
	draftBefore := e.draft()
	rb := e.json(f.tokens["a"], "POST", e.baseA+"/rollback", map[string]any{"version": 1})
	var v3 sdgVersion
	_ = json.Unmarshal(rb.Body.Bytes(), &v3)
	if rb.Code != 200 || v3.Version != 3 || v3.Kind != "rollback" || v3.SourceVersion != 1 {
		t.Fatalf("rollback: %d %s", rb.Code, rb.Body.String())
	}
	if draftAfter := e.draft(); draftAfter.Version != draftBefore.Version || !reflect.DeepEqual(draftAfter.Document, draftBefore.Document) {
		t.Errorf("rollback changed the draft: %+v -> %+v", draftBefore, draftAfter)
	}
	// more history after: earlier rows are byte-identical
	d3 := e.save(sdgNamed("three"))
	e.publish(d3)
	if w := e.json(f.tokens["a"], "POST", e.baseA+"/publish", map[string]any{"expected_draft_version": d3}); w.Code != 409 {
		t.Errorf("publishing the draft that is already live: %d, want 409", w.Code)
	}
	e.must("POST", e.baseA+"/rollback", map[string]any{"version": 2}, 200, nil)
	after := snapshot()
	if len(after) != 5 {
		t.Fatalf("history rows: %d, want 5", len(after))
	}
	for i, r := range before {
		if after[i] != r {
			t.Errorf("history row v%d changed: %+v -> %+v", r.Version, r, after[i])
		}
	}
	// the rollback row is a copy of the source row's document
	if after[2].Doc != after[0].Doc || after[4].Doc != after[1].Doc {
		t.Errorf("rollback rows are not copies of their sources")
	}
	// rollback rules
	live := e.versions()
	if w := e.json(f.tokens["a"], "POST", e.baseA+"/rollback", map[string]any{"version": *live.LiveVersion}); w.Code != 409 {
		t.Errorf("rollback to the live version: %d", w.Code)
	}
	for _, bad := range []any{99, 0, -1} {
		if w := e.json(f.tokens["a"], "POST", e.baseA+"/rollback", map[string]any{"version": bad}); w.Code != 404 && w.Code != 422 && w.Code != 400 {
			t.Errorf("rollback to %v: %d", bad, w.Code)
		}
	}
	if w := e.json(f.tokens["a"], "POST", e.baseA+"/rollback", struct{}{}); w.Code == 200 {
		t.Errorf("rollback without a version succeeded")
	}
	// publish rules: stale and already-live drafts are refused
	if w := e.json(f.tokens["a"], "POST", e.baseA+"/publish", map[string]any{"expected_draft_version": d3 - 1}); w.Code != 409 {
		t.Errorf("stale publish: %d", w.Code)
	}
	// database: nobody can rewrite or delete history
	for _, q := range []string{`UPDATE design.published_versions SET kind='publish'`, `UPDATE design.published_versions SET document='{}'::jsonb`, `DELETE FROM design.published_versions`} {
		if _, err := f.owner.Exec(context.Background(), q); err == nil {
			t.Errorf("owner: %q succeeded", q)
		}
	}
	tx, err := f.runtime.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(context.Background(), `SELECT set_config('app.tenant_id',$1,true),set_config('app.store_id',$2,true)`, f.tenantA, f.storeA1); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{`UPDATE design.published_versions SET kind='publish'`, `DELETE FROM design.published_versions`} {
		sp, _ := tx.Begin(context.Background())
		_, err := sp.Exec(context.Background(), q)
		_ = sp.Rollback(context.Background())
		if err == nil {
			t.Errorf("runtime role: %q succeeded", q)
		}
	}
	if !reflect.DeepEqual(snapshot(), after) {
		t.Errorf("history changed during the database attempts")
	}
	// audit: one row per publish/rollback, none for refusals, no document text, actor recorded
	if got, want := auditN("design.published")-pubBefore, 3; got != want {
		t.Errorf("%d publish audit rows, want %d (one per successful publish, none for refusals)", got, want)
	}
	if got, want := auditN("design.rolled_back")-rbBefore, 2; got != want {
		t.Errorf("%d rollback audit rows, want %d", got, want)
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM ops.audit_events a WHERE a.store_id=$1 AND row_to_json(a)::text LIKE '%'||$2||'%'`, f.storeA1, marker); n != 0 {
		t.Errorf("the document text leaked into %d audit rows", n)
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM ops.audit_events a WHERE a.store_id=$1 AND a.action='design.rolled_back' AND row_to_json(a)::text LIKE '%'||$2||'%'`, f.storeA1, f.principalA); n < 2 {
		t.Errorf("rollback audit rows naming the acting principal: %d, want at least 2", n)
	}
}

func sdgBuyer(t *testing.T, e *sdgEnv) {
	f := e.f
	// a published storefront with no design version: version 0 and the contract default, never 404
	status, _, body := e.buyerGet("/v1/buyer/design/published", nil)
	var def sdgBuyerDoc
	if err := json.Unmarshal(body, &def); status != 200 || err != nil || def.Version != 0 {
		t.Fatalf("default document: %d %s", status, body)
	}
	pr := def.Document["profile"].(map[string]any)
	secs := def.Document["home"].(map[string]any)["sections"].([]any)
	if pr["name"] != "fixture-store-a1" || pr["accent_color"] != "#247965" || len(secs) != 1 || secs[0].(map[string]any)["type"] != "product_grid" {
		t.Errorf("default document is not the contract default: %s", body)
	}
	// normalised shape: nullable keys present as null, accent lower-case, strings trimmed
	e.save(map[string]any{"profile": map[string]any{"name": "  Trim Me  ", "accent_color": "#AABBCC", "tagline": ""}})
	e.publish(e.draft().Version)
	pub := e.published()
	prof := pub.Document["profile"].(map[string]any)
	for _, k := range []string{"tagline", "logo_image_id", "favicon_image_id", "announcement"} {
		if v, ok := prof[k]; !ok || v != nil {
			t.Errorf("profile.%s must be present as null, got %v (present=%t)", k, v, ok)
		}
	}
	contact, _ := prof["contact"].(map[string]any)
	if prof["name"] != "Trim Me" || prof["accent_color"] != "#aabbcc" || contact == nil {
		t.Errorf("not normalised: %v", prof)
	}
	for _, k := range []string{"nav", "home", "pages"} {
		if _, ok := pub.Document[k]; !ok {
			t.Errorf("document.%s missing from the buyer shape", k)
		}
	}
	img := e.image(e.baseA, f.tokens["a"], 5)
	paths := []string{"/v1/buyer/design/published", "/v1/buyer/design/preview", "/v1/buyer/media/s/" + img}
	e.save(map[string]any{"profile": map[string]any{"name": "Trim Me", "accent_color": "#aabbcc", "logo_image_id": img}})
	e.publish(e.draft().Version)
	var tk struct {
		Token string `json:"token"`
	}
	e.must("POST", e.baseA+"/preview-token", struct{}{}, 200, &tk)
	withTok := func(edit func(*http.Request)) func(*http.Request) {
		return func(r *http.Request) {
			sdgPreview(tk.Token)(r)
			if edit != nil {
				edit(r)
			}
		}
	}
	for _, path := range paths {
		if st, _, _ := e.buyerGet(path, withTok(nil)); st != 200 && !(path == paths[0] && st == 422) {
			t.Errorf("%s control request: %d", path, st)
		}
		refused := map[string]func(*http.Request){
			"bearer":             func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+f.tokens["a"]) },
			"merchant bearer":    func(r *http.Request) { r.Header.Set("Authorization", "Bearer x") },
			"cookie":             func(r *http.Request) { r.Header.Set("Cookie", "__Host-commerce_session="+f.tokens["a"]) },
			"query":              func(r *http.Request) { r.URL.RawQuery = "preview=" + tk.Token },
			"empty query":        func(r *http.Request) { r.URL.RawQuery = "x=" },
			"wrong BFF key":      func(r *http.Request) { r.Header.Set("X-Commerce-Buyer-BFF-Key", e.buyer.key+"x") },
			"no BFF key":         func(r *http.Request) { r.Header.Del("X-Commerce-Buyer-BFF-Key") },
			"no origin":          func(r *http.Request) { r.Header.Del("X-Commerce-Storefront-Origin") },
			"origin with path":   func(r *http.Request) { r.Header.Set("X-Commerce-Storefront-Origin", e.buyer.origin+"/x") },
			"origin http":        func(r *http.Request) { r.Header.Set("X-Commerce-Storefront-Origin", "http://buyer.example") },
			"origin uppercase+x": func(r *http.Request) { r.Header.Set("X-Commerce-Storefront-Origin", "https://BUYER.example.evil.com") },
		}
		for label, edit := range refused {
			st, _, b := e.buyerGet(path, withTok(edit))
			if st < 400 || st >= 500 || strings.Contains(string(b), "Trim Me") {
				t.Errorf("%s with %s: %d (want a 4xx and no document)", path, label, st)
			}
		}
		for _, method := range []string{"POST", "PUT", "PATCH", "DELETE"} {
			if st, _, _ := e.buyerDo(method, path, []byte(`{}`), withTok(nil)); st != 405 && st != 404 && st != 400 && st != 403 {
				t.Errorf("%s %s: %d", method, path, st)
			}
		}
		if st, _, _ := e.buyerDo("GET", path, []byte(`{"x":1}`), withTok(func(r *http.Request) { r.Header.Set("Content-Type", "application/json") })); st == 200 {
			t.Errorf("GET %s with a body accepted", path)
		}
		// forwarded host / host header spoofing cannot select a store: the origin header is the only authority
		st, _, b := e.buyerGet(path, withTok(func(r *http.Request) {
			r.Header.Set("X-Commerce-Storefront-Origin", "https://not-published.example")
			r.Header.Set("X-Forwarded-Host", "buyer.example")
			r.Header.Set("Forwarded", "host=buyer.example")
			r.Host = "buyer.example"
		}))
		if st != 404 || strings.Contains(string(b), "Trim Me") {
			t.Errorf("%s with an unknown origin and spoofed forwarding headers: %d", path, st)
		}
		// a store with an active domain but no publication
		if st, _, _ := e.buyerGet(path, withTok(sdgOrigin(sdgUnpubOrigin))); st != 404 {
			t.Errorf("%s on an unpublished storefront: %d", path, st)
		}
	}
	// publication turned off after the fact: every design route stops serving
	mustExec(t, f.owner, `UPDATE control.storefront_publications SET published=false WHERE store_id=$1`, f.storeA1)
	for _, path := range paths {
		if st, _, b := e.buyerGet(path, withTok(nil)); st != 404 || strings.Contains(string(b), "Trim Me") {
			t.Errorf("%s after unpublishing the storefront: %d", path, st)
		}
	}
	mustExec(t, f.owner, `UPDATE control.storefront_publications SET published=true WHERE store_id=$1`, f.storeA1)
	if st, _, _ := e.buyerGet(paths[0], nil); st != 200 {
		t.Errorf("republished storefront: %d", st)
	}
	// media id syntax
	for _, bad := range []string{"not-a-uuid", "99999999-9999-4999-8999-999999999999", "..%2f..%2fetc", "%00"} {
		if st, _, _ := e.buyerGet("/v1/buyer/media/s/"+bad, nil); st != 404 && st != 422 && st != 400 {
			t.Errorf("media id %q: %d", bad, st)
		}
	}
	// every buyer answer is safe to render: JSON, nosniff, no store-internal ids
	_, h, b := e.buyerGet(paths[0], nil)
	if !strings.HasPrefix(h.Get("Content-Type"), "application/json") || h.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("published headers: %v", h)
	}
	for _, internal := range []string{f.tenantA, f.storeA1, f.principalA} {
		if strings.Contains(string(b), internal) {
			t.Errorf("buyer document leaks internal id %s", internal)
		}
	}
}

func sdgEnvelope(t *testing.T, e *sdgEnv) {
	tok := e.f.tokens["a"]
	doc := sdgNamed("env")
	raw := func(s string) *httptest.ResponseRecorder {
		return e.do(tok, "PUT", e.baseA+"/draft", "application/json", []byte(s), nil)
	}
	d, _ := json.Marshal(doc)
	for label, body := range map[string]string{
		"missing expected_version": `{"document":` + string(d) + `}`,
		"missing document":         `{"expected_version":0}`,
		"negative version":         `{"expected_version":-1,"document":` + string(d) + `}`,
		"string version":           `{"expected_version":"0","document":` + string(d) + `}`,
		"fractional version":       `{"expected_version":0.5,"document":` + string(d) + `}`,
		"unknown tenant_id":        `{"expected_version":0,"tenant_id":"` + e.f.tenantB + `","document":` + string(d) + `}`,
		"unknown store_id":         `{"expected_version":0,"store_id":"` + e.f.storeB + `","document":` + string(d) + `}`,
		"trailing value":           `{"expected_version":0,"document":` + string(d) + `} {}`,
		"document array":           `{"expected_version":0,"document":[]}`,
		"document null":            `{"expected_version":0,"document":null}`,
		"document string":          `{"expected_version":0,"document":"x"}`,
		"not json":                 `expected_version=0`,
		"empty":                    ``,
	} {
		if w := raw(body); w.Code != 400 && w.Code != 422 {
			t.Errorf("%s: %d %s", label, w.Code, w.Body.String())
		}
	}
	if n := countRows(t, e.f.owner, `SELECT count(*) FROM design.documents WHERE store_id IN ($1,$2)`, e.f.storeA1, e.f.storeB); n != 0 {
		t.Errorf("a refused envelope created %d drafts", n)
	}
	if w := e.do(tok, "PUT", e.baseA+"/draft", "text/plain", d, nil); w.Code != 415 {
		t.Errorf("text/plain: %d", w.Code)
	}
	if w := e.do(tok, "PUT", e.baseA+"/draft", "", d, nil); w.Code != 415 {
		t.Errorf("no content type: %d", w.Code)
	}
	// expected_version 0 creates, again 0 conflicts, the next write needs the new version
	var saved sdgDraft
	e.must("PUT", e.baseA+"/draft", map[string]any{"expected_version": 0, "document": doc}, 200, &saved)
	e.must("PUT", e.baseA+"/draft", map[string]any{"expected_version": 0, "document": doc}, 409, nil)
	e.must("PUT", e.baseA+"/draft", map[string]any{"expected_version": saved.Version + 5, "document": doc}, 409, nil)
	for _, route := range []struct{ path, body string }{
		{"/publish", `{}`}, {"/publish", `{"expected_draft_version":"1"}`}, {"/publish", `{"expected_draft_version":1,"x":1}`}, {"/publish", `{"expected_draft_version":0}`},
		{"/rollback", `{"version":"1"}`}, {"/rollback", `{"version":1,"x":1}`}, {"/preview-token", `{"x":1}`},
	} {
		if w := e.do(tok, "POST", e.baseA+route.path, "application/json", []byte(route.body), nil); w.Code == 200 || w.Code >= 500 {
			t.Errorf("POST %s %s: %d", route.path, route.body, w.Code)
		}
	}
	if v := e.versions(); len(v.Items) != 0 {
		t.Errorf("malformed publish/rollback produced history: %+v", v)
	}
	// size cap: the document is at most 256 KiB; a larger one is refused before it is stored, and a huge body cannot exhaust the server
	over := strings.Repeat(" ", 256<<10+1)
	bodyBig, _ := json.Marshal(map[string]any{"expected_version": saved.Version, "document": json.RawMessage(`{"profile":{"name":"x","accent_color":"#000000"},"pad":"` + over + `"}`)})
	if w := e.do(tok, "PUT", e.baseA+"/draft", "application/json", bodyBig, nil); w.Code != 400 && w.Code != 413 && w.Code != 422 {
		t.Errorf("oversized document: %d", w.Code)
	}
	huge := bytes.Repeat([]byte("a"), 5<<20)
	if w := e.do(tok, "PUT", e.baseA+"/draft", "application/json", huge, nil); w.Code != 400 && w.Code != 413 && w.Code != 422 {
		t.Errorf("5 MiB body: %d", w.Code)
	}
	if d := e.draft(); d.Version != saved.Version {
		t.Errorf("a refused write moved the draft: %d -> %d", saved.Version, d.Version)
	}
}
