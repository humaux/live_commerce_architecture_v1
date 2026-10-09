// Purpose: REAL_PG gates for W3-U2 sold-out reply settings through the full merchant router.
// Depends on: foundation fixture, platform.WithScope, msgtemplates.Publish, httpapi.NewHandler and migration 0151.
// Used by: scripts/dev/test-focused.sh 'TestLiveSettingsHTTP'.
// Invariants: I01 auth-derived scope, I02 same-key replay, I04 atomic setting/audit/receipt, I14 version CAS.
package foundation_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/claims"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/msgtemplates"
	"livecommerce/internal/platform"
)

func TestLiveSettingsHTTPStoreScopeCASReplay(t *testing.T) {
	f := fixture(t)
	store := randomUUID()
	mustExec(t, f.owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'Live settings gate','USD')`, f.tenantA, store)
	_, token := lcPrincipal(t, f, f.tenantA, []string{store}, "store:read", "live:read", "live:manage")
	_, readToken := lcPrincipal(t, f, f.tenantA, []string{store}, "store:read", "live:read")
	_, foreignToken := lcPrincipal(t, f, f.tenantB, []string{f.storeB}, "store:read", "live:read", "live:manage")
	labels, err := claims.NewLabelKey(randomBytes(32))
	if err != nil {
		t.Fatal(err)
	}
	handler := httpapi.NewHandler(f.runtime, httpapi.Options{Studio: true, ClaimLabels: &labels, MsgTemplates: msgtemplates.NewService()})
	tpl := "live-settings-" + t04Tag()
	publish := func(id, body string, kinds []string) {
		t.Helper()
		err := platform.WithScope(context.Background(), f.runtime, token, store, "live:manage", func(tx pgx.Tx, s platform.Scope) error {
			_, err := msgtemplates.NewService().Publish(context.Background(), tx, s, t04Key("ls-publish"), msgtemplates.PublishInput{TemplateID: id, Name: "Sold out gate", Body: body, Kinds: kinds})
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	publish(tpl, "Sorry, {{product.name}} is sold out", []string{"private_reply"})
	publish(tpl+"-bad-kind", "Sold out", []string{"dm"})
	publish(tpl+"-bad-var", "{{連結}}", []string{"private_reply"})
	t.Cleanup(func() {
		mustExec(t, f.owner, `DELETE FROM claims.sold_out_settings WHERE tenant_id=$1 AND store_id=$2`, f.tenantA, store)
		mustExec(t, f.owner, `DELETE FROM msgtemplates.templates WHERE tenant_id=$1 AND store_id=$2`, f.tenantA, store)
	})
	call := func(t *testing.T, method, pathStore, auth, key, body string, status int) map[string]any {
		t.Helper()
		var req *http.Request
		if body == "" {
			req = httptest.NewRequest(method, "/v1/admin/stores/"+pathStore+"/live-settings/sold-out-reply", nil)
		} else {
			req = httptest.NewRequest(method, "/v1/admin/stores/"+pathStore+"/live-settings/sold-out-reply", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
		}
		if auth != "" {
			req.Header.Set("Authorization", "Bearer "+auth)
		}
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != status {
			t.Fatalf("%s want %d got %d: %s", method, status, w.Code, w.Body.String())
		}
		if w.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal("private cache boundary missing")
		}
		var out map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	body := fmt.Sprintf(`{"enabled":false,"template_id":%q,"template_version":1,"expected_version":0}`, tpl)
	t.Run("unauthorized PUT 403", func(t *testing.T) { call(t, "PUT", store, readToken, "live-settings-denied", body, 403) })
	t.Run("cross-store PUT 404", func(t *testing.T) { call(t, "PUT", store, foreignToken, "live-settings-cross", body, 404) })
	t.Run("cross-store GET 404", func(t *testing.T) { call(t, "GET", store, foreignToken, "", "", 404) })
	t.Run("missing auth 401", func(t *testing.T) { call(t, "PUT", store, "", "live-settings-no-auth", body, 401) })
	t.Run("stale PUT 409", func(t *testing.T) {
		call(t, "PUT", store, token, "live-settings-old-cas", strings.Replace(body, `"expected_version":0`, `"expected_version":99`, 1), 409)
	})
	t.Run("read fixed default", func(t *testing.T) {
		out := call(t, "GET", store, readToken, "", "", 200)
		if len(out) != 4 || out["enabled"] != true || out["template_id"] != "sold-out-reply/v1" || out["template_version"] != float64(1) || out["version"] != float64(0) {
			t.Fatalf("default shape %v", out)
		}
	})
	t.Run("CAS and replay", func(t *testing.T) {
		first := call(t, "PUT", store, token, "live-settings-save", body, 200)
		if len(first) != 4 || first["enabled"] != false || first["version"] != float64(1) || first["template_id"] != tpl {
			t.Fatalf("saved shape %v", first)
		}
		replay := call(t, "PUT", store, token, "live-settings-save", body, 200)
		if fmt.Sprint(first) != fmt.Sprint(replay) {
			t.Fatal("receipt replay changed")
		}
		call(t, "PUT", store, token, "live-settings-stale", body, 409)
		call(t, "PUT", store, token, "live-settings-save", strings.Replace(body, `false`, `true`, 1), 409)
		next := strings.Replace(body, `"expected_version":0`, `"expected_version":1`, 1)
		call(t, "PUT", store, token, "live-settings-next", next, 200)
		replay = call(t, "PUT", store, token, "live-settings-save", body, 200)
		if replay["version"] != float64(1) {
			t.Fatal("old CAS replay did not return original receipt")
		}
		out := call(t, "GET", store, readToken, "", "", 200)
		if out["version"] != float64(2) {
			t.Fatal("current setting advanced by replay")
		}
		var n int
		if err := f.owner.QueryRow(context.Background(), `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='claims.sold_out_reply.set'`, f.tenantA, store).Scan(&n); err != nil || n != 2 {
			t.Fatalf("audit count %d err %v", n, err)
		}
	})
	t.Run("published and fixed template rules", func(t *testing.T) {
		for _, id := range []string{"sold-out-reply/v1", "order-pay-link/v1", tpl + "-bad-kind", tpl + "-bad-var"} {
			request := fmt.Sprintf(`{"enabled":false,"template_id":%q,"template_version":1,"expected_version":2}`, id)
			call(t, "PUT", store, token, t04Key("ls-reject"), request, 422)
		}
		request := fmt.Sprintf(`{"enabled":false,"template_id":%q,"template_version":99,"expected_version":2}`, tpl)
		call(t, "PUT", store, token, "live-settings-missing", request, 404)
		out := call(t, "GET", store, readToken, "", "", 200)
		if out["version"] != float64(2) {
			t.Fatal("rejected template changed setting")
		}
	})
}
