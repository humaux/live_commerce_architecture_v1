//go:build browser

// Purpose: real-click MCH11 UI acceptance on signed MOCK identity, actual Go B1/B2 and disposable PG.
// Depends on: mhSetup/probe fixtures, mabStartAdmin, brfPlaywright, fakegraph OAuth; no real Meta traffic.
// Used by: scripts/dev/test-local.sh --browser-meta-health-ui; private control actions are fixture preparation only.
package foundation_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/claims"
	"livecommerce/internal/httpapi"
	metaoauth "livecommerce/internal/integrations/meta/oauth"
	"livecommerce/internal/live"
	"livecommerce/internal/metaconnect"
	"livecommerce/internal/platform"
	"livecommerce/tests/metaconnect/fakegraph"
)

// TestBrowserMetaHealthUI checks health advice and merchant actions through the actual browser/BFF/Go boundary.
func TestBrowserMetaHealthUI(t *testing.T) { metaHealthUIBrowser(t, false) }

// TestBrowserMetaHealthUIFocused covers the reported mobile restriction/connection advice failures only; it is not full MCH11 acceptance.
func TestBrowserMetaHealthUIFocused(t *testing.T) { metaHealthUIBrowser(t, true) }

func metaHealthUIBrowser(t *testing.T, focused bool) {
	if os.Getenv("LC_BROWSER_META_HEALTH_UI") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use --browser-meta-health-ui")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	m := mhSetup(t)
	p := m.connectPage(t, "Health check", true, mhScopes, true)
	// Reconnect creates inbox routes; remove only this fixture's bindings' routes before mhEnv deletes those bindings.
	t.Cleanup(func() {
		mustExec(t, m.f.owner, `DELETE FROM meta_inbox.audit_events WHERE route_id IN (SELECT id FROM meta_inbox.routes WHERE tenant_id=$1 AND store_id=$2 AND binding_id IN ($3,$4))`, m.tenant, m.store, p.fbBinding, p.igBinding)
		mustExec(t, m.f.owner, `DELETE FROM meta_inbox.routes WHERE tenant_id=$1 AND store_id=$2 AND binding_id IN ($3,$4)`, m.tenant, m.store, p.fbBinding, p.igBinding)
	})
	m.sweep(t)
	principal, fixtureToken := lcPrincipal(t, m.f, m.f.tenantA, []string{m.store, m.f.storeA2}, "store:read", "integration:read", "integration:manage", "orders:read", "catalog:read", "live:read", "live:manage", "integration:execute", "inventory:read")
	var draft live.Draft
	if err := platform.WithScope(ctx, m.f.runtime, fixtureToken, m.store, "live:manage", func(tx pgx.Tx, scope platform.Scope) (err error) {
		draft, err = live.CreateDraft(ctx, tx, scope, fixtureToken, t04Key("health-scene"), live.DraftInput{Title: "Synthetic health scene", AspectRatio: "9:16"})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	labels, err := claims.NewLabelKey(randomBytes(32))
	if err != nil {
		t.Fatal(err)
	}
	viewer, _ := lcPrincipal(t, m.f, m.f.tenantA, []string{m.store}, "store:read", "orders:read", "integration:read")
	fake := fakegraph.New()
	t.Cleanup(fake.Close)
	fake.ExpectApp(miApp, miSecret, mcnRedirect)
	graph, err := metaoauth.NewGraph(fake.URL(), mcnVersion, nil)
	if err != nil {
		t.Fatal(err)
	}
	jobs, err := newInsertOnlyClient(m.f)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := metaconnect.New(metaconnect.Config{Graph: graph, App: metaoauth.App{ID: miApp, RedirectURI: mcnRedirect, Secret: []byte(miSecret)}, ConfigID: mcnConfig, GraphVersion: mcnVersion, PageAppID: miApp, IGAppID: miApp, StateKey: metaconnect.StateKeyFor([]byte(miSecret)), Seal: m.seal}, jobs)
	if err != nil {
		t.Fatal(err)
	}
	oauthPage := mcnPage(p.pageName, true)
	oauthPage.ID = p.pageID
	if err = m.f.owner.QueryRow(ctx, `SELECT ig_id FROM integration.meta_connections WHERE tenant_id=$1 AND store_id=$2 AND page_id=$3`, m.tenant, m.store, p.pageID).Scan(&oauthPage.IGID); err != nil {
		t.Fatal(err)
	}
	key := randomToken()
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.Header.Get("X-Gate-Key") != key {
			http.Error(w, "forbidden", 403)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/oauth/code" {
			code := "SYNTH-HEALTH-" + t04Tag()
			fake.AddCode(code, fakegraph.User{Permissions: append(append([]string{}, mcnFullPerms...), "pages_manage_engagement"), Pages: []fakegraph.Page{oauthPage}})
			_ = json.NewEncoder(w).Encode(map[string]string{"code": code})
			return
		}
		if r.URL.Path == "/probe" {
			if err := m.probe.Work(r.Context(), nil); err != nil {
				http.Error(w, "fixture probe failed", 500)
				return
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		if r.URL.Path == "/facts" {
			var due string
			err := m.f.owner.QueryRow(r.Context(), `SELECT next_due_at::text FROM integration.meta_health_probes WHERE tenant_id=$1 AND store_id=$2 AND page_id=$3`, m.tenant, m.store, p.pageID).Scan(&due)
			if err != nil {
				http.Error(w, "fixture read failed", 500)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"next_due_at": due})
			return
		}
		// FIXTURE preparation only: deterministic snapshots under this test's exact tenant/store/Page.
		scenario := strings.TrimPrefix(r.URL.Path, "/scenario/")
		severity, state, reason, status := "none", "ok", "ok", "active"
		switch scenario {
		case "none":
		case "review":
			state, reason = "review_required", "standard_access"
		case "warning":
			severity, state, reason = "warning", "missing_permission", "perm_pages_manage_engagement"
		case "blocking":
			severity, state, reason, status = "blocking", "reauth_required", "token_revoked", "reauth_required"
		default:
			http.NotFound(w, r)
			return
		}
		_, err := m.f.owner.Exec(r.Context(), `UPDATE integration.meta_connections SET status='active' WHERE tenant_id=$1 AND store_id=$2 AND page_id=$3`, m.tenant, m.store, p.pageID)
		if err == nil {
			_, err = m.f.owner.Exec(r.Context(), `UPDATE integration.meta_health_probes SET next_due_at=clock_timestamp() WHERE tenant_id=$1 AND store_id=$2 AND page_id=$3`, m.tenant, m.store, p.pageID)
		}
		// Reconnect legitimately clears rows; recreate them through the real fake-Graph probe before preparing a profile.
		if err == nil {
			err = m.probe.Work(r.Context(), nil)
		}
		if err == nil && status != "active" {
			// 0125 meta_health_on_connection clears capability rows on every active UPDATE, even a no-op.
			// Keep the just-probed rows for active scenarios; only the blocking scenario changes connection status.
			_, err = m.f.owner.Exec(r.Context(), `UPDATE integration.meta_connections SET status=$4 WHERE tenant_id=$1 AND store_id=$2 AND page_id=$3`, m.tenant, m.store, p.pageID, status)
		}
		if err == nil {
			result, updateErr := m.f.owner.Exec(r.Context(), `UPDATE integration.binding_capabilities SET state=CASE WHEN $7='warning' AND capability<>'reply_public' THEN 'ok' ELSE $4 END,reason=CASE WHEN $7='warning' AND capability<>'reply_public' THEN 'ok' ELSE $5 END,checked_at=clock_timestamp()-interval '2 minutes' WHERE tenant_id=$1 AND store_id=$2 AND binding_id IN ($3,$6)`, m.tenant, m.store, p.fbBinding, state, reason, p.igBinding, scenario)
			err = updateErr
			if err == nil && result.RowsAffected() != 8 {
				err = fmt.Errorf("scenario requires 8 persisted named capability rows, got %d", result.RowsAffected())
			}
		}
		if err == nil {
			_, err = m.f.owner.Exec(r.Context(), `UPDATE integration.meta_health_probes SET severity=$4,last_checked_at=clock_timestamp()-interval '2 minutes',next_due_at=clock_timestamp()+interval '6 hours' WHERE tenant_id=$1 AND store_id=$2 AND page_id=$3`, m.tenant, m.store, p.pageID, severity)
		}
		if err != nil {
			http.Error(w, "fixture update failed", 500)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(control.Close)
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	evidence := brfEvidence(t, root, "meta-health-ui")
	durable := os.Getenv("LC_META_HEALTH_EVIDENCE_ROOT")
	if durable == "" {
		t.Fatal("missing durable evidence root")
	}
	t.Cleanup(func() {
		if err := os.MkdirAll(durable, 0700); err != nil {
			t.Error(err)
			return
		}
		if err := os.CopyFS(filepath.Join(durable, filepath.Base(evidence)), os.DirFS(evidence)); err != nil {
			t.Error(err)
		}
	})
	options := httpapi.Options{SessionStoreList: true, Studio: true, ClaimLabels: &labels, MetaConnect: svc, MetaHealth: &metaconnect.Health{Reader: metaconnect.TableReader{Fallback: metaconnect.SnapshotReader{Config: mhConfig()}}}}
	stack := mabStartAdmin(t, ctx, m.f, principal, evidence, options)
	viewerEvidence := filepath.Join(evidence, "viewer")
	if err := os.MkdirAll(viewerEvidence, 0700); err != nil {
		t.Fatal(err)
	}
	viewerStack := mabStartAdmin(t, ctx, m.f, viewer, viewerEvidence, options)
	focus := ""
	if focused {
		focus = "mobile-ci"
	}
	brfPlaywright(t, ctx, stack, []string{"meta-health-ui.spec.ts"}, map[string]string{"LC_HEALTH_FOCUS": focus, "LC_HEALTH_SCENE": draft.ID, "LC_HEALTH_STORE": m.store, "LC_HEALTH_OTHER_STORE": m.f.storeA2, "LC_HEALTH_PAGE": p.pageID, "LC_HEALTH_CONTROL": control.URL, "LC_HEALTH_KEY": key, "LC_HEALTH_VIEWER": viewerStack.origin})
	t.Logf("MCH11 browser evidence: %s", filepath.Join(durable, filepath.Base(evidence)))
}

// TestMetaHealthUIWireRequiresCapability protects the UI's named states from anonymous backend rows; no PG is used.
func TestMetaHealthUIWireRequiresCapability(t *testing.T) {
	var row metaconnect.HealthCapability
	if err := json.Unmarshal([]byte(`{"binding_id":"00000000-0000-4000-8000-000000000001","provider":"facebook","capability":"read_comment","state":"ok","reason":"ok","evidence":"MOCK"}`), &row); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err = json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	if value["capability"] != "read_comment" {
		t.Fatal("B1 drops the capability identifier; named UI states must not infer array positions")
	}
}
