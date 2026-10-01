package foundation_test

// store_design_test.go: author smoke of unit store-design (migration 0087, contracts/storefront-v2.md section B) on a
// real PG: admin HTTP (httpapi, scope from the bearer) + buyer HTTP (buyerhttp, BFF key + verified origin). Evidence
// label REAL_PG; the storefront rendering and the admin UI are other units. The independent tester writes the gate suite.

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"livecommerce/internal/design"
	"livecommerce/internal/httpapi"
)

func sdPNG(t *testing.T, w int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, 3))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func sdGrant(t *testing.T, f *testFixture, token, store string) {
	t.Helper()
	mustExec(t, f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission)
		SELECT m.tenant_id,$2,s.principal_id,p FROM identity.sessions s JOIN identity.memberships m USING(principal_id)
		JOIN control.stores st ON st.tenant_id=m.tenant_id AND st.id=$2, unnest(ARRAY['integration:read','integration:manage']) p
		WHERE s.token_hash=$1 ON CONFLICT DO NOTHING`, tokenHash(token), store)
}

func TestStoreDesignRealPG(t *testing.T) {
	f := fixture(t)
	sdGrant(t, f, f.tokens["a"], f.storeA1)
	sdGrant(t, f, f.tokens["b"], f.storeB)
	t.Cleanup(func() { // TRUNCATE does not fire the append-only row trigger; these tables are used by this test only
		mustExec(t, f.owner, `TRUNCATE design.preview_tokens, design.published_versions, design.store_media, design.documents`)
	})
	mustExec(t, f.owner, `TRUNCATE design.preview_tokens, design.published_versions, design.store_media, design.documents`)
	admin := httpapi.NewHandler(f.runtime)
	buyer := bhSetup(t)
	base := "/v1/admin/stores/" + f.storeA1 + "/design"

	call := func(token, method, path, ctype string, body []byte) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, bytes.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		if ctype != "" {
			r.Header.Set("Content-Type", ctype)
		}
		w := httptest.NewRecorder()
		admin.ServeHTTP(w, r)
		return w
	}
	jsonCall := func(method, path string, payload any, want int, out any) *httptest.ResponseRecorder {
		t.Helper()
		var data []byte
		if payload != nil {
			data, _ = json.Marshal(payload)
		}
		w := call(f.tokens["a"], method, path, "application/json", data)
		if w.Code != want {
			t.Fatalf("%s %s: status=%d want=%d body=%s", method, path, w.Code, want, w.Body.String())
		}
		if out != nil {
			if err := json.Unmarshal(w.Body.Bytes(), out); err != nil {
				t.Fatal(err)
			}
		}
		return w
	}
	upload := func(token string, data []byte) *httptest.ResponseRecorder {
		var b bytes.Buffer
		mw := multipart.NewWriter(&b)
		part, _ := mw.CreateFormFile("file", "x.png")
		_, _ = part.Write(data)
		_ = mw.Close()
		return call(token, "POST", base+"/media", mw.FormDataContentType(), b.Bytes())
	}
	buyerGet := func(path string, edit func(*http.Request)) (int, http.Header, []byte) {
		t.Helper()
		r, _ := http.NewRequest("GET", buyer.server.URL+path, nil)
		r.Header.Set("X-Commerce-Buyer-BFF-Key", buyer.key)
		r.Header.Set("X-Commerce-Storefront-Origin", buyer.origin)
		if edit != nil {
			edit(r)
		}
		res, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal("buyer request failed")
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		return res.StatusCode, res.Header, body
	}
	type docResp struct {
		Version  int64           `json:"version"`
		Document json.RawMessage `json:"document"`
	}

	// 1. No draft yet: the derived default at version 0; buyers see the default at version 0 too.
	var draft design.Draft
	jsonCall("GET", base+"/draft", nil, 200, &draft)
	if draft.Version != 0 || draft.PublishedVersion != nil || !strings.Contains(string(draft.Document), `"product_grid"`) {
		t.Fatalf("default draft wrong: %+v", draft)
	}
	status, _, body := buyerGet("/v1/buyer/design/published", nil)
	var pub docResp
	if err := json.Unmarshal(body, &pub); status != 200 || err != nil || pub.Version != 0 || !strings.Contains(string(pub.Document), `"product_grid"`) {
		t.Fatalf("buyer default: %d %s", status, body)
	}

	// 2. Media: sniffed, deduplicated, scoped.
	var m1, again design.Media
	w := upload(f.tokens["a"], sdPNG(t, 8))
	if w.Code != 200 {
		t.Fatalf("upload: %d %s", w.Code, w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &m1)
	w = upload(f.tokens["a"], sdPNG(t, 8))
	_ = json.Unmarshal(w.Body.Bytes(), &again)
	if m1.ID == "" || again.ID != m1.ID || m1.ContentType != "image/png" {
		t.Fatalf("dedupe/sniff: %+v %+v", m1, again)
	}
	if w = upload(f.tokens["a"], []byte("<svg onload=alert(1)>")); w.Code != 422 {
		t.Fatalf("non-image must be 422, got %d", w.Code)
	}
	if w = call(f.tokens["b"], "GET", base+"/media/"+m1.ID, "", nil); w.Code != 403 && w.Code != 404 {
		t.Fatalf("foreign store must not read the image: %d", w.Code) // path store A + token B: scope refuses
	}
	if w = call(f.tokens["b"], "GET", "/v1/admin/stores/"+f.storeB+"/design/media/"+m1.ID, "", nil); w.Code != 404 {
		t.Fatalf("store B must not see store A's media: %d", w.Code)
	}

	// 3. Draft CAS and strict validation with a path.
	doc := map[string]any{
		"profile": map[string]any{"name": "測試店", "accent_color": "#247965"},
		"home": map[string]any{"sections": []any{
			map[string]any{"type": "hero", "image_id": m1.ID, "heading": "Hi", "cta_label": "逛逛", "cta_kind": "all_products"},
		}},
	}
	jsonCall("PUT", base+"/draft", map[string]any{"expected_version": 0, "document": doc}, 200, &draft)
	if draft.Version != 1 {
		t.Fatalf("first save: %+v", draft)
	}
	jsonCall("PUT", base+"/draft", map[string]any{"expected_version": 0, "document": doc}, 409, nil)
	bad := map[string]any{"profile": map[string]any{"name": "x", "accent_color": "#247965", "css": "body{}"}}
	var env struct {
		Code    string         `json:"code"`
		Details map[string]any `json:"details"`
	}
	jsonCall("PUT", base+"/draft", map[string]any{"expected_version": 1, "document": bad}, 422, &env)
	if env.Code != "invalid_request" || env.Details["path"] != "profile.css" {
		t.Fatalf("validation envelope: %+v", env)
	}
	foreign := map[string]any{"profile": map[string]any{"name": "x", "accent_color": "#247965", "logo_image_id": "99999999-9999-4999-8999-999999999999"}}
	jsonCall("PUT", base+"/draft", map[string]any{"expected_version": 1, "document": foreign}, 422, &env)
	if env.Details["path"] != "profile.logo_image_id" {
		t.Fatalf("unknown image path: %+v", env)
	}
	if w = call(f.tokens["a"], "PUT", base+"/draft", "application/json", []byte(`{"expected_version":1,"document":{},"tenant_id":"x"}`)); w.Code != 400 {
		t.Fatalf("unknown envelope field: %d", w.Code)
	}

	// 4. Preview token: bound to store + draft version, no-store, dies when the draft changes.
	var tok design.PreviewToken
	jsonCall("POST", base+"/preview-token", struct{}{}, 200, &tok)
	withToken := func(token string) func(*http.Request) {
		return func(r *http.Request) { r.Header.Set("X-Commerce-Design-Preview", token) }
	}
	status, header, body := buyerGet("/v1/buyer/design/preview", withToken(tok.Token))
	if status != 200 || header.Get("Cache-Control") != "no-store" || !strings.Contains(string(body), `"version":1`) {
		t.Fatalf("preview: %d %s %v", status, body, header)
	}
	if status, _, _ = buyerGet("/v1/buyer/design/preview", withToken(strings.Repeat("A", 43))); status != 404 {
		t.Fatalf("unknown token: %d", status)
	}
	if status, _, _ = buyerGet("/v1/buyer/design/preview", nil); status != 404 {
		t.Fatalf("missing token: %d", status)
	}
	mustExec(t, f.owner, `UPDATE design.preview_tokens SET created_at=created_at-interval '20 minutes', expires_at=expires_at-interval '20 minutes' WHERE draft_version=1`)
	if status, _, _ = buyerGet("/v1/buyer/design/preview", withToken(tok.Token)); status != 404 {
		t.Fatalf("expired token: %d", status)
	}
	jsonCall("POST", base+"/preview-token", struct{}{}, 200, &tok)
	jsonCall("PUT", base+"/draft", map[string]any{"expected_version": 1, "document": doc}, 200, &draft) // version 2
	if status, _, _ = buyerGet("/v1/buyer/design/preview", withToken(tok.Token)); status != 404 {
		t.Fatalf("stale-version token must read nothing: %d", status)
	}

	// 5. Publish / double publish / buyer read / media in use / media served to buyers.
	var v1, v2, v3 design.VersionInfo
	jsonCall("POST", base+"/publish", map[string]any{"expected_draft_version": 1}, 409, nil) // stale
	jsonCall("POST", base+"/publish", map[string]any{"expected_draft_version": 2}, 200, &v1)
	jsonCall("POST", base+"/publish", map[string]any{"expected_draft_version": 2}, 409, nil) // already live
	if v1.Version != 1 || v1.Kind != "publish" {
		t.Fatalf("v1: %+v", v1)
	}
	status, _, body = buyerGet("/v1/buyer/design/published", nil)
	_ = json.Unmarshal(body, &pub)
	if status != 200 || pub.Version != 1 || !strings.Contains(string(pub.Document), m1.ID) {
		t.Fatalf("buyer published: %d %s", status, body)
	}
	jsonCall("POST", base+"/media/"+m1.ID+"/delete", struct{}{}, 409, nil) // referenced by draft and live
	status, header, body = buyerGet("/v1/buyer/media/s/"+m1.ID, nil)
	if status != 200 || header.Get("Content-Type") != "image/png" || !strings.Contains(header.Get("Cache-Control"), "immutable") || !bytes.Equal(body, sdPNG(t, 8)) {
		t.Fatalf("buyer media: %d %v", status, header)
	}
	if status, _, _ = buyerGet("/v1/buyer/media/s/99999999-9999-4999-8999-999999999999", nil); status != 404 {
		t.Fatalf("unknown media: %d", status)
	}
	if status, _, _ = buyerGet("/v1/buyer/media/s/"+m1.ID, func(r *http.Request) { r.Header.Set("Authorization", "Bearer x") }); status != 403 {
		t.Fatalf("credential on a public route must be refused: %d", status)
	}

	// 6. Rollback = a new version copying an old one; history untouched; no-op rollback refused.
	doc["profile"].(map[string]any)["name"] = "第二版"
	jsonCall("PUT", base+"/draft", map[string]any{"expected_version": 2, "document": doc}, 200, &draft) // version 3
	jsonCall("POST", base+"/publish", map[string]any{"expected_draft_version": 3}, 200, &v2)
	jsonCall("POST", base+"/rollback", map[string]any{"version": 2}, 409, nil) // already live
	jsonCall("POST", base+"/rollback", map[string]any{"version": 99}, 404, nil)
	jsonCall("POST", base+"/rollback", map[string]any{"version": 1}, 200, &v3)
	if v2.Version != 2 || v3.Version != 3 || v3.Kind != "rollback" || v3.SourceVersion != 1 {
		t.Fatalf("rollback: %+v %+v", v2, v3)
	}
	status, _, body = buyerGet("/v1/buyer/design/published", nil)
	_ = json.Unmarshal(body, &pub)
	if pub.Version != 3 || !strings.Contains(string(pub.Document), "測試店") || strings.Contains(string(pub.Document), "第二版") {
		t.Fatalf("buyer after rollback: %s", body)
	}
	var versions design.VersionList
	jsonCall("GET", base+"/versions", nil, 200, &versions)
	if len(versions.Items) != 3 || versions.Items[0].Version != 3 || *versions.LiveVersion != 3 {
		t.Fatalf("versions: %+v", versions)
	}
	if _, err := f.owner.Exec(context.Background(), `UPDATE design.published_versions SET kind='publish' WHERE version=1`); err == nil {
		t.Fatal("published history must be append-only")
	}
	if _, err := f.owner.Exec(context.Background(), `DELETE FROM design.published_versions WHERE version=1`); err == nil {
		t.Fatal("published history must not be deletable")
	}
	var audits int
	if err := f.owner.QueryRow(context.Background(), `SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action IN ('design.published','design.rolled_back')`, f.storeA1).Scan(&audits); err != nil || audits != 3 {
		t.Fatalf("audit rows = %d (%v)", audits, err)
	}

	// 7. Unpublished origin and the 60-image cap.
	if status, _, _ = buyerGet("/v1/buyer/design/published", func(r *http.Request) { r.Header.Set("X-Commerce-Storefront-Origin", "https://nowhere.example") }); status != 404 {
		t.Fatalf("unpublished origin: %d", status)
	}
	mustExec(t, f.owner, `INSERT INTO design.store_media(tenant_id,store_id,content_type,bytes,sha256)
		SELECT $1,$2,'image/png',decode('00','hex'),sha256(g::text::bytea) FROM generate_series(1,58) g`, f.tenantA, f.storeA1)
	if w = upload(f.tokens["a"], sdPNG(t, 9)); w.Code != 200 { // 1 + 58 + this = 60
		t.Fatalf("60th image: %d %s", w.Code, w.Body.String())
	}
	if w = upload(f.tokens["a"], sdPNG(t, 10)); w.Code != 409 {
		t.Fatalf("61st image must be refused: %d", w.Code)
	}
}
