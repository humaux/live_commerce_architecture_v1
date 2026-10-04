package foundation_test

// meta_connect_multipage_test.go: PG gates of unit meta-multi-page (migration 0108, docs/delivery/units/meta-multi-page.md):
// one store connects up to 10 Facebook Pages instead of exactly one. This file is the implementer's own gate: it proves the
// populated re-key (D1), the cap of 10 (D2), per-Page disconnect + one unsubscribe job per Page (D2), the concurrent
// same-Page connect race (D2), and that intake still routes a comment to the right session per Page (D3). Tier: MOCK /
// REAL_PG (the upgrade subtest provisions a throw-away labelled PostgreSQL 18 like claims-retention's populated upgrade;
// the rest reuse mcnEnv over the shared fixture). Nothing here proves Meta (SANDBOX/LIVE are NOT_RUN).
// Run: bash scripts/dev/test-focused.sh '^TestMetaConnectMultiPage|^TestMetaConnectGate'
//
// Gates (ids used by docs/delivery/GATES.md and output/meta-multi-page/tests/):
//   MPG01 TestMetaConnectMultiPageUpgrade          populated re-key: PK (tenant,store,page_id), seeded row survives, page_id stays UNIQUE, a second Page inserts
//   MPG02 TestMetaConnectGateCap                   the 10th Page connects, the 11th is refused 409 cap_exceeded, a refresh at the cap still works
//   MPG03 TestMetaConnectGatePerPageDisconnect     disconnect of one Page deletes only that row and enqueues exactly one unsubscribe job
//   MPG04 TestMetaConnectGateSamePageConcurrent    two concurrent connects of the same Page collapse to one row/binding/head
//   MPG05 TestMetaConnectGateMultiPageIntake       a comment on each connected Page routes to its bound session; an unbound post stages nothing

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"livecommerce/internal/claims"
	"livecommerce/migrations"
	"livecommerce/tests/metaconnect/fakegraph"
)

// ---------------------------------------------------------------------------------------------------------------------
// MPG01 populated upgrade (D1): the re-key backfills the existing single-Page row.
// ---------------------------------------------------------------------------------------------------------------------

func TestMetaConnectMultiPageUpgrade(t *testing.T) {
	ctx := context.Background()
	owner := mciStartPG(t) // skips (NOT_RUN) without LC_TEST_DATABASE_ALLOWED=1

	// Hold back 0108 and every later migration: the database is exactly a pre-multi-page deploy (single-Page PK).
	files, err := filepath.Glob("../../migrations/[0-9][0-9][0-9][0-9]_*.sql")
	if err != nil || len(files) < 60 {
		t.Fatalf("migration files: %d %v", len(files), err)
	}
	mustExec(t, owner, `CREATE TABLE public.lc_schema_migrations (version text PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`)
	sums := map[string]string{}
	var held []string
	for _, f := range files {
		version := filepath.Base(f)
		if version < "0108" {
			continue
		}
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		sums[version] = fmt.Sprintf("%x", sha256.Sum256(body))
		held = append(held, version)
		mustExec(t, owner, `INSERT INTO public.lc_schema_migrations(version,checksum) VALUES($1,$2)`, version, sums[version])
	}
	if !slices.Contains(held, "0108_meta_multi_page.sql") {
		t.Fatalf("0108 not found among the held-back migrations: %v", held)
	}
	if err := migrations.Apply(ctx, owner); err != nil {
		t.Fatalf("apply every migration before 0108: %v", err)
	}
	// Precondition: the single-Page PK has no page_id column.
	if n := countRows(t, owner, `SELECT count(*) FROM information_schema.key_column_usage kcu
		JOIN information_schema.table_constraints tc
		ON tc.constraint_name=kcu.constraint_name AND tc.constraint_schema=kcu.constraint_schema AND tc.table_name=kcu.table_name
		WHERE tc.table_schema='integration' AND tc.table_name='meta_connections' AND tc.constraint_type='PRIMARY KEY' AND kcu.column_name='page_id'`); n != 0 {
		t.Fatal("pre-0108 database already has page_id in the meta_connections primary key")
	}

	// ---- populate: the FK chain tenant -> store -> principal -> membership -> binding -> connection ----
	tenant, store, otherStore, principal := randomUUID(), randomUUID(), randomUUID(), randomUUID()
	mustExec(t, owner, `INSERT INTO control.tenants(id,name) VALUES($1,'mpg-a')`, tenant)
	mustExec(t, owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'MPG Shop','TWD'),($1,$3,'MPG Other','TWD')`, tenant, store, otherStore)
	mustExec(t, owner, `INSERT INTO identity.principals(id) VALUES($1)`, principal)
	mustExec(t, owner, `INSERT INTO identity.memberships(tenant_id,principal_id) VALUES($1,$2)`, tenant, principal)
	pageA, pageB, igAsset := miAsset(), miAsset(), miAsset()
	fbA, igBindingA, fbB := randomUUID(), randomUUID(), randomUUID()
	mustExec(t, owner, `INSERT INTO integration.bindings(id,tenant_id,store_id,principal_id,provider,external_asset_id)
		VALUES($1,$2,$3,$4,'facebook',$5),($6,$2,$3,$4,'instagram',$7),($8,$2,$3,$4,'facebook',$9)`,
		fbA, tenant, store, principal, pageA, igBindingA, igAsset, fbB, pageB)
	// The pre-0108 row: one (tenant, store) owns one Page, with its Instagram account.
	mustExec(t, owner, `INSERT INTO integration.meta_connections(tenant_id,store_id,page_id,page_name,fb_binding,ig_binding,ig_id,ig_username,scopes,status,connected_by,route_expires_at)
		VALUES($1,$2,$3,'MPG Page A',$4,$5,$6,'synthetic_ig',ARRAY['pages_show_list'],'active',$7,clock_timestamp()+interval '30 days')`,
		tenant, store, pageA, fbA, igBindingA, igAsset, principal)

	// ---- the upgrade: release 0108 ----
	for _, version := range held {
		mustExec(t, owner, `DELETE FROM public.lc_schema_migrations WHERE version=$1`, version)
	}
	if err := migrations.Apply(ctx, owner); err != nil {
		t.Fatalf("0108 on the populated database: %v", err)
	}
	ledger := crExplicitDigest(t, owner, "public.lc_schema_migrations", "applied_at")
	if err := migrations.Apply(ctx, owner); err != nil {
		t.Fatalf("second Apply: %v", err)
	}
	if crExplicitDigest(t, owner, "public.lc_schema_migrations", "applied_at") != ledger {
		t.Error("a second Apply changed the migration ledger")
	}
	if n := countRows(t, owner, `SELECT count(*) FROM public.lc_schema_migrations WHERE version='0108_meta_multi_page.sql' AND checksum=$1`, sums["0108_meta_multi_page.sql"]); n != 1 {
		t.Errorf("0108 ledger row with the file checksum: %d", n)
	}

	// 1. the re-key: the primary key is now (tenant_id, store_id, page_id).
	var pk string
	if err := owner.QueryRow(ctx, `SELECT string_agg(kcu.column_name,',' ORDER BY kcu.ordinal_position) FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		ON tc.constraint_name=kcu.constraint_name AND tc.constraint_schema=kcu.constraint_schema AND tc.table_name=kcu.table_name
		WHERE tc.table_schema='integration' AND tc.table_name='meta_connections' AND tc.constraint_type='PRIMARY KEY'`).Scan(&pk); err != nil || pk != "tenant_id,store_id,page_id" {
		t.Fatalf("meta_connections primary key after 0108 = %q (want tenant_id,store_id,page_id): %v", pk, err)
	}
	// 2. the existing (single) row survives the re-key intact.
	if n := countRows(t, owner, `SELECT count(*) FROM integration.meta_connections
		WHERE tenant_id=$1 AND store_id=$2 AND page_id=$3 AND page_name='MPG Page A' AND ig_id=$4`, tenant, store, pageA, igAsset); n != 1 {
		t.Fatal("the seeded single-Page row must survive the re-key")
	}
	// 3. page_id stays UNIQUE platform-wide: the same Page cannot feed a second store.
	fbOther := randomUUID()
	mustExec(t, owner, `INSERT INTO integration.bindings(id,tenant_id,store_id,principal_id,provider,external_asset_id) VALUES($1,$2,$3,$4,'facebook',$5)`, fbOther, tenant, otherStore, principal, pageA)
	if _, err := owner.Exec(ctx, `INSERT INTO integration.meta_connections(tenant_id,store_id,page_id,page_name,fb_binding,scopes,status,connected_by,route_expires_at)
		VALUES($1,$2,$3,'MPG Duplicate',$4,ARRAY['pages_show_list'],'active',$5,clock_timestamp()+interval '30 days')`, tenant, otherStore, pageA, fbOther, principal); err == nil {
		t.Fatal("page_id must stay UNIQUE platform-wide after the re-key")
	}
	// 4. a second, different Page of the SAME store now inserts (the whole point of the re-key).
	mustExec(t, owner, `INSERT INTO integration.meta_connections(tenant_id,store_id,page_id,page_name,fb_binding,scopes,status,connected_by,route_expires_at)
		VALUES($1,$2,$3,'MPG Page B',$4,ARRAY['pages_show_list'],'active',$5,clock_timestamp()+interval '30 days')`, tenant, store, pageB, fbB, principal)
	if n := countRows(t, owner, `SELECT count(*) FROM integration.meta_connections WHERE tenant_id=$1 AND store_id=$2`, tenant, store); n != 2 {
		t.Fatalf("a second Page of the same store must insert after the re-key: %d rows", n)
	}
}

// ---------------------------------------------------------------------------------------------------------------------
// MPG02 cap of 10 (D2)
// ---------------------------------------------------------------------------------------------------------------------

func TestMetaConnectGateCap(t *testing.T) {
	m := newMcnEnv(t)
	m.reset()
	defer m.reset()

	pages := make([]fakegraph.Page, 0, 10)
	for i := 0; i < 10; i++ {
		p := mcnPage(fmt.Sprintf("MPG cap %d", i), false)
		st := m.mcgFresh(m.store, m.token, fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{p}})
		if r := m.pick(st, p.ID, false); r.Status != 201 {
			t.Fatalf("pick %d: %d %s", i, r.Status, r.Raw)
		}
		pages = append(pages, p)
	}
	if s := m.status(); s.JSON["connected"] != true || s.JSON["count"] != float64(10) || s.JSON["cap"] != float64(10) {
		t.Fatalf("status at 10 Pages: %s", s.Raw)
	}
	// The 11th (a different Page) is refused before any Meta call, with the frozen code.
	extra := mcnPage("MPG cap 11", false)
	st := m.mcgFresh(m.store, m.token, fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{extra}})
	if r := m.pick(st, extra.ID, false); r.Status != 409 || r.code() != "cap_exceeded" {
		t.Fatalf("11th pick: %d %s", r.Status, r.Raw)
	}
	if n := m.count(`SELECT count(*) FROM integration.meta_connections WHERE store_id=$1`, m.store); n != 10 {
		t.Fatalf("%d connection rows after the 11th pick, want 10", n)
	}
	m.mcgNothingFor("the 11th pick", extra.ID)
	// Reconnecting an already-connected Page at the cap is a refresh, never refused.
	if r := m.pick(m.mcgFresh(m.store, m.token, fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{pages[0]}}), pages[0].ID, false); r.Status != 201 {
		t.Fatalf("refresh at the cap: %d %s", r.Status, r.Raw)
	}
	if n := m.count(`SELECT count(*) FROM integration.meta_connections WHERE store_id=$1`, m.store); n != 10 {
		t.Fatalf("%d connection rows after a refresh at the cap, want 10", n)
	}
}

// ---------------------------------------------------------------------------------------------------------------------
// MPG03 per-Page disconnect (D2)
// ---------------------------------------------------------------------------------------------------------------------

func TestMetaConnectGatePerPageDisconnect(t *testing.T) {
	m := newMcnEnv(t)
	m.reset()
	defer m.reset()

	pageA, pageB := mcnPage("MPG disc A", true), mcnPage("MPG disc B", false)
	for _, p := range []fakegraph.Page{pageA, pageB} {
		st := m.mcgFresh(m.store, m.token, fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{p}})
		if r := m.pick(st, p.ID, p.IGID != ""); r.Status != 201 {
			t.Fatalf("pick %s: %d %s", p.ID, r.Status, r.Raw)
		}
	}
	if r := m.call("POST", "/disconnect", true, map[string]any{"page_id": pageA.ID}); r.Status != 200 {
		t.Fatalf("disconnect A: %d %s", r.Status, r.Raw)
	}
	if n := m.count(`SELECT count(*) FROM integration.meta_connections WHERE store_id=$1 AND page_id=$2`, m.store, pageA.ID); n != 0 {
		t.Fatal("page A's connection row survived its disconnect")
	}
	if n := m.count(`SELECT count(*) FROM integration.meta_connections WHERE store_id=$1 AND page_id=$2`, m.store, pageB.ID); n != 1 {
		t.Fatal("page B must stay connected after A's disconnect")
	}
	if n := m.count(`SELECT count(*) FROM integration.bindings WHERE store_id=$1 AND external_asset_id=$2 AND enabled`, m.store, pageA.ID); n != 0 {
		t.Fatal("A's binding must be disabled after its disconnect")
	}
	if n := m.count(`SELECT count(*) FROM integration.bindings WHERE store_id=$1 AND external_asset_id=$2 AND enabled`, m.store, pageB.ID); n != 1 {
		t.Fatal("B's binding must stay enabled")
	}
	// Exactly one unsubscribe job for A (sealed token handed to the claims-worker); none for B.
	if j := m.job(pageA.ID); j.State != "PENDING" || !j.Sealed {
		t.Fatalf("unsubscribe job for A: %+v", j)
	}
	if n := m.count(`SELECT count(*) FROM integration.meta_unsubscribe_jobs WHERE store_id=$1 AND page_id=$2`, m.store, pageB.ID); n != 0 {
		t.Fatal("no unsubscribe job may exist for B")
	}
	if s := m.status(); s.JSON["connected"] != true || s.JSON["count"] != float64(1) {
		t.Fatalf("status after A's disconnect: %s", s.Raw)
	}
	if r := m.call("POST", "/disconnect", true, map[string]any{"page_id": pageB.ID}); r.Status != 200 {
		t.Fatalf("disconnect B: %d %s", r.Status, r.Raw)
	}
	if r := m.call("POST", "/disconnect", true, map[string]any{"page_id": pageA.ID}); r.Status != 404 || r.code() != "not_found" {
		t.Fatalf("second disconnect of A: %d %s", r.Status, r.Raw)
	}
}

// ---------------------------------------------------------------------------------------------------------------------
// MPG04 concurrent connect of the same Page (D2): the page never appears twice.
// ---------------------------------------------------------------------------------------------------------------------

func TestMetaConnectGateSamePageConcurrent(t *testing.T) {
	m := newMcnEnv(t)
	m.reset()
	defer m.reset()

	page := mcnPage("MPG same-page", false)
	user := fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{page}}
	s1, s2 := m.mcgFresh(m.store, m.token, user), m.mcgFresh(m.store, m.token, user)
	var wg sync.WaitGroup
	gate := make(chan struct{})
	out := make([]mcnResp, 2)
	for i, pair := range [][2]string{{s1, page.ID}, {s2, page.ID}} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-gate
			out[i] = m.pick(pair[0], pair[1], false)
		}()
	}
	close(gate)
	wg.Wait()
	for i, r := range out {
		switch r.Status {
		case 201: // refresh
		case 409:
			if r.code() != "page_taken" {
				t.Fatalf("pick %d: 409 %q, want page_taken: %s", i, r.code(), r.Raw)
			}
		default:
			t.Fatalf("pick %d: %d %s (want 201 or 409 page_taken)", i, r.Status, r.Raw)
		}
	}
	if n := m.count(`SELECT count(*) FROM integration.meta_connections WHERE page_id=$1`, page.ID); n != 1 {
		t.Fatalf("%d connection rows for the Page after concurrent picks, want exactly 1", n)
	}
	if n := m.count(`SELECT count(*) FROM integration.bindings WHERE store_id=$1 AND provider='facebook' AND external_asset_id=$2 AND enabled`, m.store, page.ID); n != 1 {
		t.Fatalf("%d enabled facebook bindings for the Page, want exactly 1", n)
	}
	if n := m.count(`SELECT count(*) FROM integration.meta_page_heads h JOIN integration.bindings b ON b.id=h.binding_id WHERE b.external_asset_id=$1`, page.ID); n != 1 {
		t.Fatalf("%d credential heads for the Page, want exactly 1", n)
	}
	// A later sequential reconnect still refreshes in place.
	if r := m.pick(m.mcgFresh(m.store, m.token, user), page.ID, false); r.Status != 201 {
		t.Fatalf("reconnect after the race: %d %s", r.Status, r.Raw)
	}
	if n := m.count(`SELECT count(*) FROM integration.meta_connections WHERE page_id=$1`, page.ID); n != 1 {
		t.Fatalf("%d connection rows after the reconnect, want 1", n)
	}
}

// ---------------------------------------------------------------------------------------------------------------------
// MPG05 intake routes a comment to the right session per Page (D3).
// ---------------------------------------------------------------------------------------------------------------------

func TestMetaConnectGateMultiPageIntake(t *testing.T) {
	m := newMcnEnv(t)
	e, f, h := m.e, m.f, m.e.h
	m.reset()
	defer m.reset()

	pageA, pageB := mcnPage("MPG intake A", false), mcnPage("MPG intake B", false)
	for _, p := range []fakegraph.Page{pageA, pageB} {
		st := m.mcgFresh(m.store, m.token, fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{p}})
		if r := m.pick(st, p.ID, false); r.Status != 201 {
			t.Fatalf("pick %s: %d %s", p.ID, r.Status, r.Raw)
		}
	}
	if s := m.status(); s.JSON["count"] != float64(2) {
		t.Fatalf("status after two Pages: %s", s.Raw)
	}
	postA, postB := pageA.ID+"_"+mciDigits(10), pageB.ID+"_"+mciDigits(10)
	// S1 (the harness session) binds pageA's post; S2 (a fresh session) binds pageB's post.
	if _, err := e.putSource(e.session, "page", pageA.ID, postA, false, "zh-TW", true, 0); err != nil {
		t.Fatalf("bind pageA on S1: %v", err)
	}
	// One OPEN window per store: close S1 before opening S2.
	h.closeWindow(t, e.session)
	s2 := h.draft(t, f.storeA1)
	h.open(t, s2, claims.MatchExact)
	if _, err := e.putSource(s2, "page", pageB.ID, postB, false, "zh-TW", true, 0); err != nil {
		t.Fatalf("bind pageB on S2: %v", err)
	}
	// S2's staged rows are not covered by e.cleanup (keyed on e.session); drop them so lcPurgeSessions can
	// delete S2's claim_windows without the claims.meta_intake.source_id FK firing.
	defer func() {
		mustExec(t, f.owner, `DELETE FROM claims.meta_intake WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3`, f.tenantA, f.storeA1, s2)
		mustExec(t, f.owner, `DELETE FROM live.claim_sources WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3`, f.tenantA, f.storeA1, s2)
	}()
	sessionOf := func(object, asset, ref string) string {
		var s string
		if err := f.owner.QueryRow(context.Background(), `SELECT session_id::text FROM claims.meta_intake WHERE object=$1 AND asset_id=$2 AND comment_ref=$3`, object, asset, ref).Scan(&s); err != nil {
			t.Fatalf("intake of %s %s: %v", object, ref, err)
		}
		return s
	}
	// A comment on pageA's bound post routes to S1; one on pageB's bound post routes to S2.
	commentA := mciDigits(15) + "_" + mciDigits(10)
	evA := mcPost(t, e.page, pageA.ID, mciFBBody(pageA.ID, postA, commentA, mciDigits(15), "n", "A1", mciAt(2*time.Second), nil))
	mcAwait(t, e.page, evA)
	if got := sessionOf("page", pageA.ID, commentA); got != e.session {
		t.Fatalf("pageA comment routed to session %s, want S1 %s", got, e.session)
	}
	commentB := mciDigits(15) + "_" + mciDigits(10)
	evB := mcPost(t, e.page, pageB.ID, mciFBBody(pageB.ID, postB, commentB, mciDigits(15), "n", "A1", mciAt(2*time.Second), nil))
	mcAwait(t, e.page, evB)
	if got := sessionOf("page", pageB.ID, commentB); got != s2 {
		t.Fatalf("pageB comment routed to session %s, want S2 %s", got, s2)
	}
	// A comment on an unbound post of a connected Page stages nothing.
	orphan := pageA.ID + "_" + mciDigits(10)
	commentO := mciDigits(15) + "_" + mciDigits(10)
	evO := mcPost(t, e.page, pageA.ID, mciFBBody(pageA.ID, orphan, commentO, mciDigits(15), "n", "A1", mciAt(2*time.Second), nil))
	mcAwait(t, e.page, evO)
	if n := m.count(`SELECT count(*) FROM claims.meta_intake WHERE object='page' AND asset_id=$1 AND comment_ref=$2`, pageA.ID, commentO); n != 0 {
		t.Fatal("a comment on an unbound post produced an intake row")
	}
}
