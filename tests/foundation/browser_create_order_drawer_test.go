//go:build browser

// Purpose: independently verify LC-U3 over real Next/BFF/Go/PG without changing the manual-order gate's facts.
// Depends on: lbuEnv, real inbox A14, signed MOCK OIDC, real comment poller fixture and the Node drawer driver.
// Used by: TestBrowserManualOrderLink after its original ten cases and audit/link assertions.
// Invariants: I02/I03/I06/I09/I11; no bearer response, key, contact or address is emitted to evidence.
package foundation_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"livecommerce/internal/merchanttools"
	"livecommerce/internal/msgtemplates"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"livecommerce/internal/claims"
	"livecommerce/internal/claimsintake"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/identity"
	"livecommerce/internal/identityhttp"
	"livecommerce/internal/inbox"
	"livecommerce/internal/integrations/meta"
	"livecommerce/internal/metaconnect"
	"livecommerce/internal/oidclogin"
	"livecommerce/internal/platform"
)

type drawerFixture struct{ Linked, Unlinked, Bundle string }

func runCreateOrderDrawerBrowser(t *testing.T) {
	t.Helper()
	calibration := os.Getenv("LC_DRAWER_CALIBRATION")
	if calibration != "" && calibration != "new-key-on-retry" {
		t.Fatal("unsupported LC_DRAWER_CALIBRATION")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	e := lbuNew(t)
	e.service("cvs_711", "MANUAL", 0)
	e.cvsSettings(tcvAllChains, true, "20000", 20)
	var cvsKey string
	options, err := e.manual.Options(ctx, e.token(), e.store())
	if err != nil {
		t.Fatal(err)
	}
	for _, option := range options {
		if option.DeliveryKind == "cvs_711" {
			cvsKey = option.OptionKey
		}
	}
	if cvsKey == "" {
		t.Fatal("drawer requires a manual 7-ELEVEN option")
	}

	var skuCode string
	if err := e.p.f.owner.QueryRow(ctx, `SELECT code FROM catalog.skus WHERE id=$1`, e.money).Scan(&skuCode); err != nil {
		t.Fatal(err)
	}
	mustExec(t, e.p.f.owner, `UPDATE catalog.products SET name='LC-U3 synthetic catalog' WHERE id=(SELECT product_id FROM catalog.skus WHERE id=$1)`, e.money)
	// Synthetic capability evidence enables only the exact conversation binding, through the real B1 read model.
	pageID, bindingID := mciDigits(15), randomUUID()
	mustExec(t, e.p.f.owner, `INSERT INTO integration.bindings(tenant_id,store_id,id,principal_id,provider,external_asset_id) VALUES($1,$2,$3,$4,'facebook',$5)`, e.tenant(), e.store(), bindingID, e.p.f.principalA, pageID)
	mustExec(t, e.p.f.owner, `INSERT INTO integration.meta_connections(tenant_id,store_id,page_id,page_name,fb_binding,scopes,status,connected_by,route_expires_at) VALUES($1,$2,$3,'LC-U3 MOCK Page',$4,ARRAY['pages_messaging'],'active',$5,clock_timestamp()+interval '1 day')`, e.tenant(), e.store(), pageID, bindingID, e.p.f.principalA)
	mustExec(t, e.p.f.owner, `INSERT INTO integration.binding_capabilities(tenant_id,store_id,page_id,binding_id,provider,capability,state,reason,evidence,checked_at) VALUES($1,$2,$3,$4,'facebook','dm_session','ok','fixture_ok','MOCK',clock_timestamp())`, e.tenant(), e.store(), pageID, bindingID)
	pageFixture := e2eMetaSetup(t, e.p.f) // Owned signed-ingress fixture; its payload key also opens the actual queued DM sender.
	kr, err := inbox.LoadKeyring(func(n string) string {
		switch n {
		case "COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID":
			return miKeyID
		case "COMMERCE_META_PAYLOAD_KEYS_JSON":
			return `{"keys":[{"id":"` + miKeyID + `","key_base64":"` + base64.StdEncoding.EncodeToString(pageFixture.key) + `"}]}`
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	svc, err := inbox.NewService(kr)
	if err != nil {
		t.Fatal(err)
	}
	sealKeys, openSend := mcnPageRing(t)
	jobs, err := river.NewClient[pgx.Tx](riverpgxv5.New(e.p.f.runtime), &river.Config{Schema: "river"})
	if err != nil {
		t.Fatal(err)
	}
	svc.EnableSend(sealKeys, jobs, msgtemplates.NewService())
	e.fb, err = merchanttools.NewForBuyer(e.manual, svc)
	if err != nil {
		t.Fatal(err)
	}
	// No dispatcher starts: the real PG operation and sealed original link remain queued for inspection.
	e.handler = httpapi.NewHandler(e.p.f.runtime, httpapi.Options{SessionStoreList: true, Studio: true, ClaimLabels: &e.h.labels, CVS: e.cvs, ManualOrders: e.manual, ForBuyer: e.fb, Inbox: svc, MetaHealth: &metaconnect.Health{Reader: metaconnect.TableReader{Fallback: metaconnect.SnapshotReader{}}}})
	// Seed the customer's actual contact and most recent delivery via the ordinary real Place pipeline.
	st, seed := e.post(e.token(), t04Key("drawer-history"), e.body(1, nil, "", false))
	if st != 201 {
		t.Fatalf("drawer history fixture status=%d", st)
	}
	var customer string
	if err := e.p.f.owner.QueryRow(ctx, `SELECT owner_id::text FROM checkout.orders WHERE id=$1`, lbuStr(seed, "order_id")).Scan(&customer); err != nil {
		t.Fatal(err)
	}
	// Reuse the signed Meta ingress/consumer/intake harness in this same trade store.
	// Manual claims cannot be restricted: blocklist identity is Facebook/Instagram only (W3-05B).
	e.grantCreator("integration:execute")
	metaClaims := &mciEnv{t: t, h: e.h, page: pageFixture, pageAsset: pageID, pageBinding: bindingID, stopConsumer: mciNoop, consumerWorkers: 1}
	metaClaims.actor, err = meta.NewClaimsActorKey(randomBytes(32))
	if err != nil {
		t.Fatal(err)
	}
	metaClaims.link, err = claims.NewReplyLinkKey(randomBytes(32))
	if err != nil {
		t.Fatal(err)
	}
	miRoute(t, metaClaims.page, pageID, e.tenant(), e.store(), bindingID)
	var metaSessions []string
	// Only our six session sources, intake rows and restrictions are owned by this fixture.
	t.Cleanup(func() {
		for _, session := range metaSessions {
			for _, query := range []string{
				`DELETE FROM claims.blocked_actors WHERE tenant_id=$1 AND store_id=$2 AND source_bundle_id IN (SELECT id FROM claims.bundles WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3)`,
				`DELETE FROM claims.meta_intake WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3`,
				`DELETE FROM live.claim_sources WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3`,
				`DELETE FROM live.claim_window_intervals WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3`,
			} {
				if _, err := e.p.f.owner.Exec(context.Background(), query, e.tenant(), e.store(), session); err != nil {
					t.Errorf("drawer Meta fixture cleanup: %v", err)
				}
			}
		}
	})
	metaClaims.intakePool, err = platform.OpenClaimsIntakePool(ctx, miRole(t, e.p.f, "commerce_claims_intake"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(metaClaims.intakePool.Close)
	metaClaims.poller, err = claimsintake.New(ctx, metaClaims.intakePool, metaClaims.link, claimsintake.Config{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	metaClaims.startConsumer(t)
	t.Cleanup(func() { metaClaims.stopConsumer() })
	fixtures := []drawerFixture{}
	for i := 0; i < 6; i++ {
		session, _ := e.session("A1", ltgLive, 5)
		metaSessions = append(metaSessions, session)
		metaClaims.session, metaClaims.postID = session, pageID+"_"+mciDigits(10)
		metaClaims.mustSource(t, "page", pageID, metaClaims.postID, false)
		comment := metaClaims.postFBTo(t, metaClaims.postID, "", "", "A1+2", mciAt(3*time.Second), nil, false)
		// Keep the original consumer deadline; safe job facts diagnose a fixture failure without payloads.
		t.Cleanup(func() {
			if !t.Failed() {
				return
			}
			var state, reason, errorsJSON string
			if err := e.p.f.owner.QueryRow(context.Background(), `SELECT j.state,coalesce(ev.terminal_reason,''),coalesce(j.errors::text,'[]') FROM river_meta.river_job j JOIN meta_inbox.events ev ON ev.job_id=j.id WHERE ev.id=$1`, comment.ev.id).Scan(&state, &reason, &errorsJSON); err != nil {
				t.Log("drawer consumer diagnostic unavailable")
				return
			}
			if state == "completed" && reason == "processed" {
				return
			}
			codes := regexp.MustCompile(`SQLSTATE [0-9A-Z]{5}`).FindAllString(errorsJSON, -1)
			kinds := []string{}
			for _, kind := range []string{"permission denied", "too many clients", "decrypt", "key", "invalid", "stage_claim_intake", "retryable"} {
				if strings.Contains(errorsJSON, kind) {
					kinds = append(kinds, kind)
				}
			}
			t.Logf("drawer consumer fixture state=%s reason=%s SQLSTATE=%v categories=%v", state, reason, codes, kinds)
		})
		mcAwait(t, metaClaims.page, comment.ev)
		metaClaims.apply(t)
		claim := metaClaims.refresh(t, mciReply{s: comment, provider: "facebook", asset: pageID})
		if claim.intake.State != "APPLIED" || claim.ev.outcome != claims.OutcomeAccepted || claim.ev.platform != "facebook" || claim.ev.quantity != 2 || claim.bundleID == "" {
			t.Fatal("drawer Meta claim must be accepted")
		}
		bundle, conv := claim.bundleID, lcConversation(t, e.p.f, e.tenant(), e.store(), "page")
		e.linkPeer(bundle, conv)
		e.h.closeWindow(t, session)
		mustExec(t, e.p.f.owner, `UPDATE social.conversations SET asset_id=$2 WHERE id=$1`, conv, pageID)
		mustExec(t, e.p.f.owner, `UPDATE inbox.bundle_peers SET asset_id=$2 WHERE bundle_id=$1`, bundle, pageID)
		unlinked := lcConversation(t, e.p.f, e.tenant(), e.store(), "page")
		if len(fixtures) == 0 {
			// Fixture preparation through signed Meta ingress, not a product UI substitute.
			raw := []byte(fmt.Sprintf(`{"object":"page","entry":[{"id":%q,"time":123,"messaging":[{"sender":{"id":%q},"recipient":{"id":%q},"timestamp":%d,"message":{"mid":%q,"text":"SYNTHETIC request invoice"}}]}]}`, pageID, mciDigits(15), pageID, time.Now().UnixMilli(), "m."+randomUUID()))
			ev := mcPost(t, metaClaims.page, pageID, raw)
			mcAwait(t, metaClaims.page, ev)
			if err := e.p.f.owner.QueryRow(ctx, `SELECT conversation_id::text FROM social.messages WHERE event_id=$1`, ev.id).Scan(&unlinked); err != nil {
				t.Fatal(err)
			}
			// Preserve the original no-capability Unlinked A15 assertion until the queued case explicitly arms it.
			mustExec(t, e.p.f.owner, `UPDATE social.conversations SET asset_id='1234567890' WHERE id=$1`, unlinked)
		}
		mustExec(t, e.p.f.owner, `UPDATE inbox.conversation_state SET last_inbound_at=clock_timestamp()-interval '25 hours' WHERE conversation_id IN ($1,$2)`, conv, unlinked)
		// Calls real A14 CAS; no direct customer_id update and no owner guessing.
		b, _ := json.Marshal(map[string]any{"customer_id": customer, "expected_version": 0})
		reply := e.admin(e.token()).call("POST", "/inbox/conversations/"+conv+"/customer-link", t04Key("drawer-link"), "application/json", b)
		if reply.Code != 200 {
			t.Fatalf("drawer A14 fixture status=%d", reply.Code)
		}
		// Prepare a restriction through the real command; existing bundles/orders remain permitted by W3-05B.
		blockedBody, _ := json.Marshal(map[string]any{"bundle_id": bundle})
		blocked := e.admin(e.token()).call("POST", "/live-sessions/"+session+"/claims/blocklist", t04Key("drawer-restrict"), "application/json", blockedBody)
		if blocked.Code != 200 {
			t.Fatalf("drawer blocklist fixture status=%d", blocked.Code)
		}
		fixtures = append(fixtures, drawerFixture{conv, unlinked, bundle})
	}
	// The second harness has its own payload key; it must exclusively own the shared consumer queue.
	metaClaims.stopConsumer()
	// The existing signed-ingress/real-poller fixture proves the direct CommentStream trigger over real A2.
	comments := newConsoleCommentsFixture(t)
	cf := comments.e.h.f
	mustExec(t, cf.owner, `INSERT INTO identity.memberships(tenant_id,principal_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, cf.tenantA, e.p.f.principalA)
	for _, perm := range []string{"store:read", "live:read", "live:manage", "inbox:read", "orders:read", "inventory:reserve", "catalog:read"} {
		mustExec(t, cf.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, cf.tenantA, cf.storeA1, e.p.f.principalA, perm)
	}
	commentHandler := httpapi.NewHandler(cf.runtime, httpapi.Options{SessionStoreList: true, Studio: true, ClaimLabels: &comments.e.h.labels, CommentStream: comments.stream, Inbox: comments.e.svc, ForBuyer: e.fb, ManualOrders: e.manual,
		MetaHealth: &metaconnect.Health{Reader: metaconnect.TableReader{Fallback: metaconnect.SnapshotReader{}}}})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	_, port, _ := net.SplitHostPort(addr)
	_ = listener.Close()
	origin := browserFront(t, addr)
	idp := newBrowserIDP(t, origin+"/api/auth/callback")
	mustExec(t, e.p.f.owner, `INSERT INTO identity.external_identities(issuer,subject,principal_id) VALUES($1,'browser-subject',$2)`, idp.server.URL, e.p.f.principalA)
	// A real staff role with the exact problematic grant combination, in both owned fixture stores.
	// Its signed login stays behind the same OIDC/BFF/session seams as the owner cases.
	narrow := randomUUID()
	mustExec(t, e.p.f.owner, `INSERT INTO identity.principals(id) VALUES($1)`, narrow)
	for _, scope := range [][2]string{{e.tenant(), e.store()}, {cf.tenantA, cf.storeA1}} {
		mustExec(t, e.p.f.owner, `INSERT INTO identity.memberships(tenant_id,principal_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, scope[0], narrow)
		mustExec(t, e.p.f.owner, `INSERT INTO identity.store_staff(tenant_id,store_id,principal_id,role) VALUES($1,$2,$3,'fulfilment')`, scope[0], scope[1], narrow)
		for _, permission := range []string{"store:read", "inbox:read", "live:read", "orders:read", "inventory:reserve"} {
			mustExec(t, e.p.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,$4)`, scope[0], scope[1], narrow, permission)
		}
	}
	t.Cleanup(func() {
		mustExec(t, e.p.f.owner, `UPDATE identity.external_identities SET principal_id=$2 WHERE issuer=$1 AND subject='browser-subject'`, idp.server.URL, e.p.f.principalA)
		for _, query := range []string{`DELETE FROM identity.session_events WHERE principal_id=$1`, `DELETE FROM identity.sessions WHERE principal_id=$1`, `DELETE FROM ops.command_results WHERE principal_id=$1`, `DELETE FROM ops.audit_events WHERE principal_id=$1`, `DELETE FROM identity.store_staff WHERE principal_id=$1`, `DELETE FROM identity.store_grants WHERE principal_id=$1`, `DELETE FROM identity.memberships WHERE principal_id=$1`, `DELETE FROM identity.principals WHERE id=$1`} {
			mustExec(t, e.p.f.owner, query, narrow)
		}
	})
	_, _, authority := identityFixture(t)
	provider, err := oidclogin.New(ctx, oidclogin.Config{Issuer: idp.server.URL, ClientID: browserClientID, RedirectURL: idp.redirect, AllowLoopbackForTests: true})
	if err != nil {
		t.Fatal(err)
	}
	service, err := identity.New(authority, observedBrowserProvider{Provider: provider, t: t}, identity.Policy{ProviderKey: "browser-create-order-drawer-v1", SessionTTL: time.Hour})
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
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/__test/drawer-bank-transfer" {
			// Disclosed fault on this harness's synthetic settings only, after the real UI has loaded valid options.
			var input struct {
				Enabled bool `json:"enabled"`
			}
			dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128))
			dec.DisallowUnknownFields()
			if dec.Decode(&input) != nil {
				http.Error(w, "invalid fixture control", 400)
				return
			}
			result, err := e.p.f.owner.Exec(r.Context(), `UPDATE checkout.bank_transfer_settings SET enabled=$3,version=version+1,updated_at=clock_timestamp() WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store(), input.Enabled)
			if err != nil || result.RowsAffected() != 1 {
				http.Error(w, "fixture control failed", 500)
				return
			}
			w.WriteHeader(204)
			return
		}
		if r.Method == "POST" && r.URL.Path == "/__test/drawer-recovery-facts" {
			var input struct {
				Key string `json:"key"`
			}
			dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256))
			dec.DisallowUnknownFields()
			if dec.Decode(&input) != nil || !regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`).MatchString(input.Key) {
				http.Error(w, "invalid observation", 400)
				return
			}
			var seals int
			err := e.p.f.owner.QueryRow(r.Context(), `SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND operation='merchanttools.order.attempt.recovery' AND idempotency_key=$4 AND response='{"state":"not_created"}'::jsonb`, e.tenant(), e.store(), e.p.f.principalA, input.Key).Scan(&seals)
			if err != nil {
				http.Error(w, "observation failed", 500)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			_ = json.NewEncoder(w).Encode(map[string]int{"not_created_seals": seals})
			return
		}
		if r.Method == "POST" && r.URL.Path == "/__test/drawer-queued-window" {
			// Only this owned synthetic conversation is opened, after the existing closed-window UI assertions.
			var input struct {
				Open bool `json:"open"`
			}
			dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256))
			dec.DisallowUnknownFields()
			if dec.Decode(&input) != nil {
				http.Error(w, "invalid fixture control", 400)
				return
			}
			asset := "1234567890" // Restore the original unavailable capability before this fixture's later reuse.
			if input.Open {
				asset = pageID
			}
			_, err := e.p.f.owner.Exec(r.Context(), `UPDATE social.conversations SET asset_id=$2 WHERE id=$1`, fixtures[0].Unlinked, asset)
			if err == nil {
				_, err = e.p.f.owner.Exec(r.Context(), `UPDATE inbox.conversation_state SET last_inbound_at=clock_timestamp() WHERE conversation_id=$1`, fixtures[0].Unlinked)
			}
			if err != nil {
				http.Error(w, "fixture control failed", 500)
				return
			}
			w.WriteHeader(204)
			return
		}
		if r.Method == "POST" && r.URL.Path == "/__test/drawer-queued-facts" {
			var input struct {
				OrderID     string `json:"order_id"`
				OperationID string `json:"operation_id"`
				Link        string `json:"link"`
			}
			dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
			dec.DisallowUnknownFields()
			if dec.Decode(&input) != nil {
				http.Error(w, "invalid observation", 400)
				return
			}
			u, err := url.Parse(input.Link)
			if err != nil {
				http.Error(w, "invalid observation", 400)
				return
			}
			fragment, _ := url.ParseQuery(u.Fragment)
			digest := sha256.Sum256([]byte(fragment.Get("t")))
			var operations, links, regenerations int
			var enc, sealed []byte
			err = e.p.f.owner.QueryRow(r.Context(), `SELECT count(*) FROM integration.operations WHERE tenant_id=$1 AND store_id=$2 AND action='meta.dm_send' AND request->>'order_id'=$3`, e.tenant(), e.store(), input.OrderID).Scan(&operations)
			if err == nil {
				err = e.p.f.owner.QueryRow(r.Context(), `SELECT enc,sealed FROM inbox.send_secrets WHERE operation_id=$1`, input.OperationID).Scan(&enc, &sealed)
			}
			var plain []byte
			if err == nil {
				plain, err = openSend.OpenSend(e.tenant(), e.store(), input.OperationID, enc, sealed)
			}
			// Sealed JSON escapes ampersands in URLs; compare its decoded text, never its wire encoding.
			var message struct {
				Text string `json:"text"`
			}
			if err == nil {
				err = json.Unmarshal(plain, &message)
			}
			original := err == nil && strings.Contains(message.Text, input.Link)
			message.Text = ""
			clear(plain)
			if err == nil {
				err = e.p.f.owner.QueryRow(r.Context(), `SELECT count(*) FROM checkout.order_links WHERE order_id=$1 AND token_hash=$2 AND redeemed_at IS NULL AND expires_at>clock_timestamp()`, input.OrderID, digest[:]).Scan(&links)
			}
			if err == nil {
				err = e.p.f.owner.QueryRow(r.Context(), `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='order.manual_link_regenerated'`, e.tenant(), e.store()).Scan(&regenerations)
			}
			if err != nil {
				http.Error(w, "observation failed", 500)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"operations": operations, "secrets": 1, "original_link": original, "valid_links": links, "regenerations": regenerations})
			return
		}
		// Owned test-only switch after all owner contexts close; product handlers and grants stay real.
		if r.Method == "POST" && r.URL.Path == "/__test/drawer-principal" {
			var input struct {
				Limited bool `json:"limited"`
			}
			dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128))
			dec.DisallowUnknownFields()
			if dec.Decode(&input) != nil {
				http.Error(w, "invalid fixture control", 400)
				return
			}
			principal := e.p.f.principalA
			if input.Limited {
				principal = narrow
			}
			result, err := e.p.f.owner.Exec(r.Context(), `UPDATE identity.external_identities SET principal_id=$2 WHERE issuer=$1 AND subject='browser-subject'`, idp.server.URL, principal)
			if err != nil || result.RowsAffected() != 1 {
				http.Error(w, "fixture control failed", 500)
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method == "POST" && (r.URL.Path == "/__test/drawer-request-facts" || r.URL.Path == "/__test/manual-request-facts") {
			operation := "merchanttools.order.for_buyer"
			if r.URL.Path == "/__test/manual-request-facts" {
				operation = "merchanttools.order.manual"
			}
			var input struct {
				Keys []string `json:"keys"`
			}
			dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
			dec.DisallowUnknownFields()
			if dec.Decode(&input) != nil || len(input.Keys) < 1 || len(input.Keys) > 2 {
				http.Error(w, "invalid observation", 400)
				return
			}
			for _, key := range input.Keys {
				if !regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`).MatchString(key) {
					http.Error(w, "invalid observation", 400)
					return
				}
			}
			var orders, receipts int
			if err := e.p.f.owner.QueryRow(r.Context(), `SELECT count(DISTINCT o.id),count(*) FROM ops.command_results c
     JOIN checkout.orders o ON o.id=(c.response->>'order_id')::uuid AND o.tenant_id=c.tenant_id AND o.store_id=c.store_id
     WHERE c.tenant_id=$1 AND c.store_id=$2 AND c.principal_id=$3 AND c.operation=$5 AND c.idempotency_key=ANY($4::text[])`, e.tenant(), e.store(), e.p.f.principalA, input.Keys, operation).Scan(&orders, &receipts); err != nil {
				http.Error(w, "observation failed", 500)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			_ = json.NewEncoder(w).Encode(map[string]int{"orders": orders, "receipts": receipts})
			return
		}
		if r.Method == "GET" && r.URL.Path == "/__test/drawer-facts" {
			var orders int
			if err := e.p.f.owner.QueryRow(r.Context(), `SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2 AND source='merchant_manual'`, e.tenant(), e.store()).Scan(&orders); err != nil {
				http.Error(w, "fixture observation failed", 500)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			_ = json.NewEncoder(w).Encode(map[string]int{"orders": orders})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/admin/stores/"+cf.storeA1+"/") {
			commentHandler.ServeHTTP(w, r)
			return
		}
		e.handler.ServeHTTP(w, r)
	}))
	api := httptest.NewServer(mux)
	t.Cleanup(api.Close)
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	// Follow the existing manual-order harness: new runs stay in ignored output/playwright.
	evidence, err := os.MkdirTemp(filepath.Join(root, "output/playwright"), "create-order-drawer-")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(fixtures)
	cmd := exec.CommandContext(ctx, "node", "tests/admin/create-order-drawer-gate.mjs")
	cmd.Dir = root
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	defer func() {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}()
	cmd.Env = browserEnvironment(map[string]string{
		"COMMERCE_IDENTITY_ENABLED": "1", "COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS": "1", "COMMERCE_PUBLIC_ORIGIN": origin, "COMMERCE_API_ORIGIN": api.URL, "COMMERCE_OIDC_ISSUER": idp.server.URL, "COMMERCE_BFF_KEY": bffKey,
		"LC_DRAWER_EVIDENCE": evidence, "LC_DRAWER_PORT": port, "LC_DRAWER_STORE": e.store(), "LC_DRAWER_FIXTURES": string(raw), "LC_DRAWER_SKU": e.money, "LC_DRAWER_SKU_CODE": skuCode, "LC_DRAWER_HOME": e.homeKey, "LC_DRAWER_CVS": cvsKey,
		"LC_DRAWER_COMMENT_STORE": cf.storeA1, "LC_DRAWER_COMMENT_SCENE": comments.e.session, "LC_DRAWER_COMMENT_REF": fmt.Sprint(comments.ids["claim_ref"]), "LC_DRAWER_CALIBRATION": calibration,
	})
	log := browserLog(t, filepath.Join(evidence, "browser.log"))
	cmd.Stdout, cmd.Stderr = log, log
	runErr := cmd.Run()
	// Perform independent final PG readback even when the calibration intentionally failed in Node.
	var orders int
	if err := e.p.f.owner.QueryRow(ctx, `SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2 AND source='merchant_manual'`, e.tenant(), e.store()).Scan(&orders); err != nil {
		t.Fatal(err)
	}
	// One history + twelve drawer orders + one manual UNKNOWN/edit/replay order, each additionally bound to its request receipts.
	pg, _ := json.Marshal(map[string]any{"orders": orders, "expected": 14, "single_order": orders == 14})
	if err := os.WriteFile(filepath.Join(evidence, "pg-readback.json"), pg, 0600); err != nil {
		t.Fatal(err)
	}
	if runErr != nil {
		t.Fatalf("drawer browser gate failed: %v; PG orders=%d expected=14; evidence=%s", runErr, orders, evidence)
	}
	if orders != 14 {
		t.Fatalf("drawer single-order PG readback orders=%d expected=14; evidence=%s", orders, evidence)
	}
	for _, fx := range fixtures {
		if n := e.count(`SELECT count(*) FROM inbox.order_for_buyer WHERE store_id=$1 AND bundle_id=$2 AND state='placed'`, e.store(), fx.Bundle); n != 1 {
			t.Fatalf("drawer bundle reservation count=%d", n)
		}
	}
	data, err := os.ReadFile(filepath.Join(evidence, "result.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Cases  int      `json:"cases"`
		Failed []string `json:"failed"`
	}
	// Keep the original exact 26 cases, plus each of the three new narrow-role locale cases.
	if json.Unmarshal(data, &result) != nil || result.Cases != 29 || len(result.Failed) != 0 {
		t.Fatalf("drawer exact 29-case matrix result missing; evidence=%s", evidence)
	}
	ledgerBytes, err := os.ReadFile(filepath.Join(evidence, "click-ledger.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ledger []struct{ Case, Control, Status string }
	if err := json.Unmarshal(ledgerBytes, &ledger); err != nil {
		t.Fatal(err)
	}
	for _, locale := range []string{"en", "zh-TW", "zh-CN"} {
		matches := 0
		for _, row := range ledger {
			if row.Case == "DU3-no-catalog-"+locale+"-390" && row.Control == "catalog permission entry gates" && row.Status == "PASS" {
				matches++
			}
		}
		if matches != 1 {
			t.Fatalf("drawer narrow-role locale %s requires exactly one real click verdict, got %d; evidence=%s", locale, matches, evidence)
		}
	}
	t.Logf("PASS LC-U3 real browser/Go/PG matrix cases=%d orders=%d; evidence=%s", result.Cases, orders, evidence)
}
