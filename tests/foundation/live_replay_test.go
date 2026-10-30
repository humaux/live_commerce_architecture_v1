// Purpose: FEAT-LIVE-REPLAY REAL_PG/HTTP gates for merchant publication and buyer replay reads.
// Depends on: csSetup/bhSetup, production admin/buyer routers, catalog-v2 reads and migration 0213.
// Used by: GitHub Foundation non-browser Go suite.
// Invariants: server scope, live:manage, CAS and authenticated receipt replay; public ENDED sessions only;
// official stored video only; catalog remains the current price/stock/publication authority. Fixtures are synthetic.
package foundation_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"livecommerce/internal/claims"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/integrations/metareply"
	"livecommerce/internal/live"
	"livecommerce/internal/platform"
)

func TestLiveReplayOfficialReadPersistsVideo(t *testing.T) {
	x := newATSEnv(t, []string{"pages_read_engagement", "pages_show_list"})
	f := x.e.h.f
	const permalink = "https://www.facebook.com/synthetic.page/videos/77778888/"
	graph := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/v26.0/"+x.e.pageAsset+"/live_videos" || r.URL.Query().Has("access_token") {
			t.Error("replay attempted an unapproved Meta request")
			w.WriteHeader(400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]string{
			"id": "77778888", "post_id": x.e.postID, "title": "測試回放", "status": "VOD",
			"creation_time": "2026-10-30T12:00:00+0000", "permalink_url": permalink,
		}}})
	}))
	t.Cleanup(graph.Close)
	routes, err := metareply.LiveVideoRoutes(x.pool, x.e.pageKeys, nil, metareply.Config{GraphBaseURL: graph.URL, GraphVersion: "v26.0"})
	if err != nil {
		t.Fatal(err)
	}
	jobs, err := river.NewClient(riverpgxv5.New(f.runtime), &river.Config{Schema: "river"})
	if err != nil {
		t.Fatal(err)
	}
	var result live.PageLiveVideosRead
	err = x.e.h.do(x.e.h.token, f.storeA1, func(tx pgx.Tx, scope platform.Scope) error {
		var e error
		result, e = live.ReadPageLiveVideos(context.Background(), tx, scope, x.e.h.token, t04Key("replay-read"), x.e.pageBinding, jobs)
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	t06StartDispatcher(t, x.pool, x.queue, routes, mciDispatchOptions())
	x.run(result.OperationID)
	x.e.awaitOp(t, result.OperationID, "SUCCEEDED", 8*time.Second, "completed")
	var exists bool
	if err := f.owner.QueryRow(context.Background(), `SELECT to_regclass('live.replay_videos') IS NOT NULL`).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("FEAT-LIVE-REPLAY: successful official read must persist a durable live-video row")
	}
	t.Cleanup(func() {
		mustExec(t, f.owner, `DELETE FROM live.replay_videos WHERE tenant_id=$1 AND store_id=$2 AND binding_id=$3`, f.tenantA, f.storeA1, x.e.pageBinding)
	})
	var got string
	if err := f.owner.QueryRow(context.Background(), `SELECT permalink_url FROM live.replay_videos
		WHERE tenant_id=$1 AND store_id=$2 AND binding_id=$3 AND video_id='77778888'`, f.tenantA, f.storeA1, x.e.pageBinding).Scan(&got); err != nil || got != permalink {
		t.Fatalf("worker did not persist the actual official permalink: %q %v", got, err)
	}
	var forced, publicRead, buyerRead bool
	if err := f.owner.QueryRow(context.Background(), `SELECT relrowsecurity AND relforcerowsecurity,
		EXISTS(SELECT 1 FROM aclexplode(coalesce(relacl,acldefault('r',relowner))) a WHERE a.grantee=0 AND a.privilege_type='SELECT'),
		has_table_privilege('commerce_buyer_runtime','live.replay_videos','SELECT')
		FROM pg_class WHERE oid='live.replay_videos'::regclass`).Scan(&forced, &publicRead, &buyerRead); err != nil || !forced || publicRead || buyerRead {
		t.Fatalf("video table bypasses scoped public projection: forced=%t public=%t buyer=%t err=%v", forced, publicRead, buyerRead, err)
	}
}

func TestLiveReplayPublicationAndBuyerReads(t *testing.T) {
	b := bhSetup(t)
	e := csSetup(t)
	f := e.f
	admin := httpapi.NewHandler(f.runtime, httpapi.Options{Studio: true, ClaimLabels: &e.labels})
	session := e.session(t, e.store)
	path := "/v1/admin/stores/" + e.store + "/live-sessions/" + session + "/replay"
	call := func(t *testing.T, method, target, token, key, body string, status int, code string) []byte {
		t.Helper()
		r := httptest.NewRequest(method, target, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		w := httptest.NewRecorder()
		admin.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("FEAT-LIVE-REPLAY %s: status=%d want=%d body=%s", method, w.Code, status, w.Body.String())
		}
		if w.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal("merchant replay route must be private no-store")
		}
		var out map[string]any
		if json.Unmarshal(w.Body.Bytes(), &out) != nil || code != "" && out["code"] != code {
			t.Fatalf("response code wanted %q: %s", code, w.Body.String())
		}
		return append([]byte(nil), w.Body.Bytes()...)
	}
	// The RED boundary is an assertion on a missing HTTP behavior, never a missing Go symbol/compile error.
	initial := call(t, "GET", path, e.token, "", "", 200, "")
	var state struct {
		Public  bool  `json:"public"`
		Version int64 `json:"version"`
	}
	if json.Unmarshal(initial, &state) != nil || state.Public || state.Version != 1 {
		t.Fatalf("must default private at v1: %s", initial)
	}
	body := func(public bool, version int64) string {
		return fmt.Sprintf(`{"public":%t,"expected_version":%d}`, public, version)
	}
	t.Run("validation and staff permission", func(t *testing.T) {
		_, viewer := lcPrincipal(t, f, f.tenantA, []string{e.store}, "store:read", "live:read")
		call(t, "PUT", path, viewer, t04Key("replay-viewer"), body(true, 1), 403, "forbidden")
		call(t, "PUT", path, e.token, t04Key("replay-invalid"), body(true, 0), 422, "invalid_request")
		call(t, "PUT", path, e.token, t04Key("replay-draft"), body(true, 1), 409, "invalid_transition")
		call(t, "PUT", path, e.token, "", body(false, 1), 422, "invalid_request")
	})
	t.Run("merchant cross store and tenant", func(t *testing.T) {
		_, foreign := lcPrincipal(t, f, f.tenantB, []string{f.storeB}, "store:read", "live:read", "live:manage")
		call(t, "GET", path, foreign, "", "", 404, "not_found")
		call(t, "PUT", path, foreign, t04Key("replay-tenant"), body(true, 1), 404, "not_found")
		otherPath := strings.Replace(path, e.store, e.bare, 1)
		call(t, "PUT", otherPath, e.token, t04Key("replay-store"), body(true, 1), 404, "not_found")
		spoof := strings.TrimSuffix(body(true, 1), "}") + fmt.Sprintf(`,"tenant_id":%q,"store_id":%q}`, f.tenantB, f.storeB)
		call(t, "PUT", path, e.token, t04Key("replay-spoof"), spoof, 422, "invalid_request")
	})
	// Owner writes below prepare synthetic fixture state only; replay assertions use the actual HTTP handlers.
	sku := lcSKUs(t, f, f.tenantA, e.store, "USD", 1)[0]
	offer, err := e.createOffer(e.token, e.store, t04Key("replay-offer"), session,
		claims.OfferInput{Keyword: "REPLAY", SKUID: sku, MaxQuantityPerClaim: 2})
	if err != nil {
		t.Fatal(err)
	}
	var productID string
	if err := f.owner.QueryRow(e.ctx, `SELECT product_id::text FROM catalog.skus WHERE id=$1`, sku).Scan(&productID); err != nil {
		t.Fatal(err)
	}
	mustExec(t, f.owner, `UPDATE catalog.products SET status='active' WHERE id=$1`, productID)
	mustExec(t, f.owner, `UPDATE catalog.skus SET status='active' WHERE id=$1`, sku)
	post := e.asset + "_777"
	status, out, raw := e.call(t, "PUT", e.store, session, e.token, t04Key("replay-source"),
		csBody("https://www.facebook.com/synthetic.page/posts/777", false, "zh-TW", true, 0))
	csWant(t, "replay source fixture", status, out, raw, 200, "")
	mustExec(t, f.owner, `UPDATE live.sessions SET lifecycle='ended',lifecycle_at=clock_timestamp() WHERE id=$1`, session)
	t.Run("ended with no official video", func(t *testing.T) {
		call(t, "PUT", path, e.token, t04Key("replay-missing"), body(true, 1), 409, "no_official_video")
	})
	permalink := "https://www.facebook.com/synthetic.page/videos/777/"
	// Simulate a *future* existing live_videos worker completion. No historical Meta call or URL derivation.
	mustExec(t, f.owner, `INSERT INTO live.replay_videos(tenant_id,store_id,binding_id,video_id,post_id,permalink_url)
		VALUES($1,$2,$3,'777',$4,$5)`, f.tenantA, e.store, e.binding, post, permalink)
	t.Cleanup(func() {
		mustExec(t, f.owner, `DELETE FROM live.replay_videos WHERE tenant_id=$1 AND store_id=$2`, f.tenantA, e.store)
	})
	key := t04Key("replay-public")
	first := call(t, "PUT", path, e.token, key, body(true, 1), 200, "")
	if json.Unmarshal(first, &state) != nil || !state.Public || state.Version != 2 {
		t.Fatalf("public result: %s", first)
	}
	t.Run("idempotency and CAS", func(t *testing.T) {
		retry := call(t, "PUT", path, e.token, key, body(true, 1), 200, "")
		var a, b any
		_ = json.Unmarshal(first, &a)
		_ = json.Unmarshal(retry, &b)
		if !reflect.DeepEqual(a, b) {
			t.Fatal("same-key retry changed its receipt")
		}
		call(t, "PUT", path, e.token, key, body(false, 1), 409, "version_conflict")
		call(t, "PUT", path, e.token, t04Key("replay-stale"), body(false, 1), 409, "version_conflict")
	})
	b.origin = "https://replay.example"
	bhPublish(t, b.bcHarness, b.origin, f.tenantA, e.store)
	read := func(t *testing.T, target string, want int) map[string]any {
		t.Helper()
		return bhRead[map[string]any](t, b.request(t, "GET", target, "", "", nil, nil), want)
	}
	detailPath := "/v1/buyer/replays/" + session
	t.Run("public list detail and current catalog", func(t *testing.T) {
		list := read(t, "/v1/buyer/replays", 200)
		items, ok := list["items"].([]any)
		if !ok || len(items) != 1 || items[0].(map[string]any)["session_id"] != session {
			t.Fatalf("public list: %+v", list)
		}
		check := func() {
			t.Helper()
			detail := read(t, detailPath, 200)
			if detail["session_id"] != session || detail["permalink_url"] != permalink || detail["title"] == "" {
				t.Fatalf("detail: %+v", detail)
			}
			if _, err := time.Parse(time.RFC3339Nano, fmt.Sprint(detail["ended_at"])); err != nil {
				t.Fatal("ended_at must be the persisted session end time")
			}
			embed, err := url.Parse(fmt.Sprint(detail["embed_url"]))
			if err != nil || embed.Scheme != "https" || embed.Host != "www.facebook.com" || embed.Path != "/plugins/video.php" || embed.Query().Get("href") != permalink {
				t.Fatal("replay must use only Facebook's official embedded video player")
			}
			offers, ok := detail["offers"].([]any)
			if !ok || len(offers) != 1 {
				t.Fatalf("session offers: %+v", detail)
			}
			item := offers[0].(map[string]any)
			catalog := read(t, "/v1/buyer/catalog/v2/products/"+productID, 200)
			if item["offer_id"] != offer.ID || item["sku_id"] != sku || !reflect.DeepEqual(item["product"], catalog) {
				t.Fatalf("replay differs from current buyer catalog: %+v", item)
			}
		}
		check() // No inventory balance: sold-out is exactly the shop's current read.
		mustExec(t, f.owner, `UPDATE catalog.skus SET price_minor=3456 WHERE id=$1`, sku)
		check() // No historical price copy or client price is accepted.
		mustExec(t, f.owner, `UPDATE catalog.products SET status='draft' WHERE id=$1`, productID)
		detail := read(t, detailPath, 200)
		if offers, ok := detail["offers"].([]any); !ok || len(offers) != 0 {
			t.Fatal("unpublished catalog product leaked into replay")
		}
	})
	t.Run("buyer scope and credential refusal", func(t *testing.T) {
		b.origin = "https://buyer.example" // verified origin for a different store, same tenant
		read(t, detailPath, 404)
		b.origin = "https://replay-other.example"
		bhPublish(t, b.bcHarness, b.origin, f.tenantB, f.storeB)
		read(t, detailPath, 404)
		b.origin = "https://replay.example"
		read(t, detailPath+"?store_id="+f.storeA1, 422)
		read(t, detailPath+"?tenant_id="+f.tenantB, 422)
		r := b.request(t, "GET", detailPath, b.cap.Token, "", nil, nil)
		if r.status != 403 {
			t.Fatalf("public replay accepted a buyer bearer: %d", r.status)
		}
	})
	t.Run("unpublish and replay old receipt", func(t *testing.T) {
		call(t, "PUT", path, e.token, t04Key("replay-off"), body(false, 2), 200, "")
		call(t, "PUT", path, e.token, key, body(true, 1), 200, "")
		read(t, detailPath, 404)
		list := read(t, "/v1/buyer/replays", 200)
		if items, ok := list["items"].([]any); !ok || len(items) != 0 {
			t.Fatal("receipt replay republished a hidden session")
		}
	})
	t.Run("currently live session cannot remain visible", func(t *testing.T) {
		call(t, "PUT", path, e.token, t04Key("replay-republish"), body(true, 3), 200, "")
		mustExec(t, f.owner, `UPDATE live.sessions SET lifecycle='live',lifecycle_at=clock_timestamp() WHERE id=$1`, session)
		read(t, detailPath, 404)
		list := read(t, "/v1/buyer/replays", 200)
		if items, ok := list["items"].([]any); !ok || len(items) != 0 {
			t.Fatal("public flag exposed a session that is no longer ENDED")
		}
	})
}
