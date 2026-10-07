//go:build browser

// Purpose: W6-U2 independent real-click ledger + ad-unbind/feed acceptance over isolated PG and production Next.
// Depends on: oqEnv/t06 fixtures, adsEnv's MOCK Graph, mabStartAdmin/brfPlaywright, migrations 0159/0160.
// Used by: scripts/dev/test-local.sh --browser-operations-ads.
// Invariants: runner control only prepares synthetic SQL fixtures; no UI assertion uses direct API mutation, no worker dispatches,
// UNKNOWN never retries, server CAS/capability/permissions remain authoritative. LIVE and provider SANDBOX are NOT_RUN.
package foundation_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"livecommerce/internal/httpapi"
)

func TestBrowserOperationsAds(t *testing.T) {
	if os.Getenv("LC_BROWSER_OPERATIONS_ADS_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-operations-ads")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	e := newOQEnv(t)
	a := newAdsEnv(t, adsOpts{fx: e.base, noWorker: true})
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	evidence := brfEvidence(t, root, "operations-ads")
	ops := map[string]string{}
	ready := e.mock(t)
	ops["cancel"] = ready.ID
	cas := e.mock(t)
	ops["cas"] = cas.ID
	retry := e.live(t)
	ops["retry"] = retry.ID
	e.set(t, retry.ID, "FAILED_FINAL", 2, "", "provider_failed")
	e.killJobs(t, retry.ID)
	failed := e.mock(t)
	ops["failed"] = failed.ID
	e.set(t, failed.ID, "FAILED_FINAL", 1, "", "provider_failed")
	e.killJobs(t, failed.ID)
	for _, name := range []string{"query", "limit", "reader"} {
		o := e.live(t)
		ops[name] = o.ID
		e.set(t, o.ID, "UNKNOWN", 3, "", "reconcile_budget_exhausted")
		e.killJobs(t, o.ID)
	}
	draftObject := randomUUID()
	ops["draft_object"] = draftObject
	protective := e.lane(t, "meta_ads", "meta.ads.pause", "marketing", `{"draft_id":"`+draftObject+`"}`)
	ops["protective"] = protective.ID
	orderObject := randomUUID()
	ops["order_object"] = orderObject
	order := e.op(t, "ecpay_logistics", "ecpay.cvs_create", "transactional", `{"order_id":"`+orderObject+`","private_canary":"SYNTHETIC-DO-NOT-EXPOSE"}`)
	ops["order"] = order.ID
	foreignBinding := e.register(t, e.otherStore, uniqueAction("w6-ui-foreign"))
	// plan binds to its fixture's store. Keep the foreign operation in the foreign scope;
	// changing the binding's store would make the UI isolation assertion vacuous.
	foreignFixture := *e.t06GoFixture
	foreignFixture.store = e.otherStore
	foreign := foreignFixture.plan(t, uniqueAction("w6-ui-foreign-op"), foreignBinding, `{"v":1}`)
	if e.count(t, `SELECT count(*) FROM integration.operations WHERE id=$1 AND store_id=$2`, foreign.OperationID, e.otherStore) != 1 {
		t.Fatal("foreign operation fixture was not planned in the other store")
	}
	ops["foreign"] = foreign.OperationID
	// Enough READY rows to require a second page at the UI's 20-row limit. Named rows stay newest.
	for i := 0; i < 30; i++ {
		o := e.mock(t)
		mustExec(t, e.base.owner, `UPDATE integration.operations SET created_at=clock_timestamp()-interval '1 day' WHERE id=$1`, o.ID)
	}
	mustExec(t, e.base.owner, `INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
  VALUES($1,$2,$3,3,'UNKNOWN','','synthetic_unknown')`, e.tenant, e.store, ops["query"])
	adDraft := a.newDraft(adsDraftIn{})
	activate := randomUUID()
	// Owner-only disclosed fixture: a READY activate in the frozen remote-object graph counts as possibly spending (AD6).
	mustExec(t, e.base.owner, `INSERT INTO integration.operations(id,tenant_id,store_id,principal_id,binding_id,binding_version,provider,
  external_asset_id,purpose,action,semantic_key,request_hash,request,job_id,state,generation)
  VALUES($1,$2,$3,$4,$5,1,'meta_ads',$6,'marketing','meta.ads.activate',$7,decode(repeat('01',32),'hex'),'{}',424242,'READY',0)`,
		activate, a.tenant, a.store, a.creator, a.adBinding, a.account, uniqueAction("w6-ui-activate"))
	mustExec(t, e.base.owner, `INSERT INTO ads.remote_objects(tenant_id,store_id,draft_id,publish_attempt,kind,seq,operation_id)
  VALUES($1,$2,$3,1,'activate',1,$4)`, a.tenant, a.store, adDraft, activate)
	feedOrigin := "https://w6-ui-" + adsDigits(8) + ".example.test"
	mustExec(t, e.base.owner, `INSERT INTO control.storefront_publications(tenant_id,store_id,published) VALUES($1,$2,true)
  ON CONFLICT(tenant_id,store_id) DO UPDATE SET published=true`, a.tenant, a.store)
	mustExec(t, e.base.owner, `INSERT INTO control.storefront_domains(tenant_id,store_id,origin,state,ownership_verified_at,tls_verified_at,valid_until,evidence_ref)
  VALUES($1,$2,$3,'ACTIVE',now()-interval '2 days',now()-interval '1 day',now()+interval '300 days','platform-subdomain')`, a.tenant, a.store, feedOrigin)
	t.Cleanup(func() {
		_, _ = e.base.owner.Exec(context.Background(), `DELETE FROM control.storefront_domains WHERE store_id=$1 AND origin=$2`, a.store, feedOrigin)
		_, _ = e.base.owner.Exec(context.Background(), `DELETE FROM control.storefront_publications WHERE store_id=$1`, a.store)
	})
	controlKey := randomToken()
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.Header.Get("X-Gate-Key") != controlKey {
			http.Error(w, "forbidden", 403)
			return
		}
		var err error
		switch r.URL.Path {
		case "/cas":
			_, err = e.base.owner.Exec(ctx, `UPDATE integration.operations SET generation=generation+1 WHERE id=$1`, ops["cas"])
		case "/limit":
			_, err = e.base.owner.Exec(ctx, `INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code,created_at)
    SELECT $1,$2,$3,3,'UNKNOWN','','query_requested',clock_timestamp()-make_interval(hours=>n) FROM generate_series(1,5) n`, e.tenant, e.store, ops["limit"])
		case "/ads/inflight":
			_, err = e.base.owner.Exec(ctx, `UPDATE integration.operations SET state='UNKNOWN',generation=1 WHERE id=$1`, activate)
		case "/ads/clear":
			_, err = e.base.owner.Exec(ctx, `UPDATE integration.operations SET state='BLOCKED_POLICY',generation=2 WHERE id=$1`, activate)
		case "/ads/unpublish", "/ads/publish":
			_, err = e.base.owner.Exec(ctx, `UPDATE control.storefront_publications SET published=$2 WHERE store_id=$1`, a.store, r.URL.Path == "/ads/publish")
		case "/readonly":
			_, err = e.base.owner.Exec(ctx, `DELETE FROM identity.store_grants WHERE (principal_id=$1 AND permission='integration:execute') OR (principal_id=$2 AND permission='ads:manage')`, e.principal, a.creator)
		case "/restore":
			_, err = e.base.owner.Exec(ctx, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES
    ($1,$2,$3,'integration:execute'),($4,$5,$6,'ads:manage') ON CONFLICT DO NOTHING`, e.tenant, e.store, e.principal, a.tenant, a.store, a.creator)
		default:
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, "fixture preparation failed", 500)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(control.Close)
	stack := mabStartAdmin(t, ctx, e.base, e.principal, evidence, httpapi.Options{SessionStoreList: true, OperationJobs: e.jobs})
	adsEvidence := filepath.Join(evidence, "ads-stack")
	if err = os.MkdirAll(adsEvidence, 0700); err != nil {
		t.Fatal(err)
	}
	adsStack := mabStartAdmin(t, ctx, e.base, a.creator, adsEvidence, httpapi.Options{SessionStoreList: true, Ads: a.svc})
	rawOps, _ := json.Marshal(ops)
	brfPlaywright(t, ctx, stack, []string{"operations-ads.spec.ts"}, map[string]string{
		"LC_BROWSER_STORE": e.store, "LC_BROWSER_OTHER_STORE": e.otherStore, "LC_BROWSER_OPS": string(rawOps),
		"LC_BROWSER_ADS_ORIGIN": adsStack.origin, "LC_BROWSER_ADS_STORE": a.store, "LC_BROWSER_ADS_ACCOUNT": a.account,
		"LC_BROWSER_ADS_OPERATION": activate, "LC_BROWSER_ADS_DRAFT": adDraft, "LC_BROWSER_FEED_URL": feedOrigin + "/feeds/meta.csv",
		"LC_BROWSER_CONTROL": control.URL, "LC_BROWSER_CONTROL_KEY": controlKey,
	})
	// Independent owner SQL readback supplements, never replaces, the actual UI click/reload assertions.
	if state, code, gen, _ := e.row(t, ops["cancel"]); state != "CANCELLED" || code != "cancelled_by_merchant" || gen != 1 {
		t.Fatalf("cancel not persisted: %s/%s/%d", state, code, gen)
	}
	if state, _, _, _ := e.row(t, ops["retry"]); state != "READY" || e.events(t, ops["retry"], "retry_authorized") != 1 {
		t.Fatal("registered retry not persisted exactly once")
	}
	if state, _, _, floor := e.row(t, ops["query"]); state != "UNKNOWN" || floor != 3 || e.events(t, ops["query"], "query_requested") != 1 {
		t.Fatal("query changed UNKNOWN or failed to persist")
	}
	if e.events(t, ops["limit"], "query_requested") != 5 || len(e.jobRows(t, ops["limit"])) != 1 {
		t.Fatal("capped query wrote an event or job")
	}
	if state, _, gen, _ := e.row(t, ops["cas"]); state != "READY" || gen != 1 || e.events(t, ops["cas"], "cancelled_by_merchant") != 0 {
		t.Fatal("stale CAS mutated operation")
	}
	if e.count(t, `SELECT count(*) FROM integration.bindings WHERE id=$1 AND NOT enabled`, a.adBinding) != 1 {
		t.Fatal("unbind not persisted")
	}
	if e.count(t, `SELECT count(*) FROM integration.meta_page_credentials WHERE binding_id=$1`, a.adBinding) != 0 {
		t.Fatal("unbind retained sealed credentials")
	}
	if e.count(t, `SELECT count(*) FROM ads.campaign_drafts WHERE id=$1`, adDraft) != 1 || e.count(t, `SELECT count(*) FROM ads.connections WHERE binding_id=$1`, a.adBinding) != 1 {
		t.Fatal("unbind destroyed history")
	}
	if len(e.jobRows(t, ops["query"])) != 2 || len(e.jobRows(t, ops["retry"])) != 2 {
		t.Fatal("query/retry did not enqueue exactly one persisted follow-up")
	}
	if e.audits(t, "integration.operation_query_requested") != 1 || e.audits(t, "integration.operation_retry_authorized") != 1 || e.audits(t, "integration.operation_cancelled") != 1 {
		t.Fatal("ledger action audit missing or duplicated")
	}
	if e.events(t, ops["protective"], "cancelled_by_merchant") != 0 || e.events(t, ops["failed"], "retry_authorized") != 0 {
		t.Fatal("a refused action mutated an operation")
	}
	if e.count(t, `SELECT count(*) FROM ads.remote_objects WHERE operation_id=$1`, activate) != 1 || e.count(t, `SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='ads.account_unbound'`, a.store) != 1 {
		t.Fatal("ad history/audit not retained exactly once")
	}
	brfShots(t, evidence, 12)
	t.Logf("BROWSER + REAL_PG + MOCK; evidence=%s; SANDBOX/LIVE NOT_RUN", evidence)
}
