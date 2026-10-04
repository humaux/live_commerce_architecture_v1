package foundation_test

// meta_multi_page_independent_test.go: independent gates of unit meta-multi-page (migration 0108), written from
// docs/delivery/units/meta-multi-page.md and contracts/meta-claims-intake-v1.md ("Merchant connect (R4)" + §7) by the
// test author (not the implementer). They extend MPG01-MPG05 with the adversarial cases the brief names: the 11th
// connect refused under CONCURRENCY (the store advisory lock, not the sequential path), two stores racing the same
// page_id with the loser refused before AND after the winner's disconnect, a per-Page disconnect that leaves the peer
// Page's sealed token byte-identical and its intake routing live, and proof that no token or ciphertext reaches the
// status DTO, the stored pick list or the command receipts. Tier: MOCK / REAL_PG (fake Graph).
// Run: bash scripts/dev/test-focused.sh '^TestMetaConnectInd'
//
//   MPG06 TestMetaConnectIndCapConcurrent         3 concurrent picks of 3 new Pages at 9/10: exactly one 201, two 409 cap_exceeded
//   MPG07 TestMetaConnectIndCrossStoreRace        2 stores race one page_id; the loser is refused even after the winner disconnects
//   MPG08 TestMetaConnectIndDisconnectKeepsPeer   disconnect A: B's credentials byte-identical, B routes, A's post stages nothing
//   MPG09 TestMetaConnectIndNoTokenAnywhere       no token/ciphertext in the status DTO, pick list or command receipts

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"livecommerce/internal/claims"
	"livecommerce/tests/metaconnect/fakegraph"
)

// ---------------------------------------------------------------------------------------------------------------------
// MPG06 the cap of 10 holds under concurrency (D2): the finish-time check under the store advisory lock, not the
// prepare-time soft check, is what stops three racing picks at 9/10 from all committing.
// ---------------------------------------------------------------------------------------------------------------------

func TestMetaConnectIndCapConcurrent(t *testing.T) {
	m := newMcnEnv(t)
	m.reset()
	defer m.reset()

	for i := 0; i < 9; i++ {
		p := mcnPage(fmt.Sprintf("MPG06 base %d", i), false)
		st := m.mcgFresh(m.store, m.token, fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{p}})
		if r := m.pick(st, p.ID, false); r.Status != 201 {
			t.Fatalf("base pick %d: %d %s", i, r.Status, r.Raw)
		}
	}
	// Three distinct new Pages race for the last slot; every prepare passes (9 < 10), so only the locked finish decides.
	pages := []fakegraph.Page{mcnPage("MPG06 race 1", false), mcnPage("MPG06 race 2", false), mcnPage("MPG06 race 3", false)}
	states := make([]string, len(pages))
	for i, p := range pages {
		states[i] = m.mcgFresh(m.store, m.token, fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{p}})
	}
	out := make([]mcnResp, len(pages))
	var wg sync.WaitGroup
	gate := make(chan struct{})
	for i, p := range pages {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-gate
			out[i] = m.pick(states[i], p.ID, false)
		}()
	}
	close(gate)
	wg.Wait()
	wins := 0
	for i, r := range out {
		switch r.Status {
		case 201:
			wins++
		case 409:
			if r.code() != "cap_exceeded" {
				t.Fatalf("racer %d: 409 %q, want cap_exceeded: %s", i, r.code(), r.Raw)
			}
		default:
			t.Fatalf("racer %d: %d %s (want 201 or 409 cap_exceeded)", i, r.Status, r.Raw)
		}
	}
	if wins != 1 {
		t.Fatalf("%d racers won the last slot, want exactly 1", wins)
	}
	if n := m.count(`SELECT count(*) FROM integration.meta_connections WHERE store_id=$1`, m.store); n != 10 {
		t.Fatalf("%d connection rows after the race, want 10", n)
	}
	if s := m.status(); s.JSON["count"] != float64(10) || s.JSON["cap"] != float64(10) {
		t.Fatalf("status after the race: %s", s.Raw)
	}
}

// ---------------------------------------------------------------------------------------------------------------------
// MPG07 two stores race one page_id (D1/D2): exactly one wins, and a Page cannot move stores even after the winner
// disconnected it (asset_owners stays, 0095).
// ---------------------------------------------------------------------------------------------------------------------

func TestMetaConnectIndCrossStoreRace(t *testing.T) {
	m := newMcnEnv(t)
	f := m.f
	m.reset()
	defer m.reset()
	_, tokA2 := lcPrincipal(t, f, f.tenantA, []string{f.storeA2}, "store:read", "integration:manage", "integration:read")
	page := mcnPage("MPG07 race", false)
	defer func() { // the winner may be either store; m.reset() covers only m.store
		if r := m.callOn(f.storeA2, tokA2, "POST", "/disconnect", true, map[string]any{"page_id": page.ID}); r.Status != 200 && r.Status != 404 {
			t.Fatalf("cleanup disconnect on storeA2: %d %s", r.Status, r.Raw)
		}
	}()
	user := fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{page}}
	type who struct{ store, tok string }
	stores := []who{{m.store, m.token}, {f.storeA2, tokA2}}
	states := make([]string, 2)
	for i, s := range stores {
		states[i] = m.mcgFresh(s.store, s.tok, user)
	}
	out := make([]mcnResp, 2)
	var wg sync.WaitGroup
	gate := make(chan struct{})
	for i, s := range stores {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-gate
			out[i] = m.callOn(s.store, s.tok, "POST", "/pick", true, map[string]any{"state_id": states[i], "page_id": page.ID, "include_instagram": false})
		}()
	}
	close(gate)
	wg.Wait()
	winner := -1
	for i, r := range out {
		switch r.Status {
		case 201:
			if winner != -1 {
				t.Fatalf("both stores picked Page %s", page.ID)
			}
			winner = i
		case 409:
			if r.code() != "page_taken" {
				t.Fatalf("loser %d: 409 %q, want page_taken: %s", i, r.code(), r.Raw)
			}
			for _, id := range []string{f.storeA1, f.storeA2, f.tenantA} {
				if strings.Contains(string(r.Raw), id) {
					t.Fatalf("the loser's refusal leaks a scope id: %s", r.Raw)
				}
			}
		default:
			t.Fatalf("store %d: %d %s (want 201 or 409 page_taken)", i, r.Status, r.Raw)
		}
	}
	if winner == -1 {
		t.Fatal("nobody got the Page")
	}
	if n := m.count(`SELECT count(*) FROM integration.meta_connections WHERE page_id=$1`, page.ID); n != 1 {
		t.Fatalf("%d connection rows for the Page, want exactly 1", n)
	}
	if n := m.count(`SELECT count(*) FROM integration.bindings WHERE external_asset_id=$1 AND enabled`, page.ID); n != 1 {
		t.Fatalf("%d enabled bindings for the Page, want exactly 1", n)
	}
	if n := m.count(`SELECT count(*) FROM integration.meta_page_heads h JOIN integration.bindings b ON b.id=h.binding_id
		WHERE b.external_asset_id=$1`, page.ID); n != 1 {
		t.Fatalf("%d credential heads for the Page, want exactly 1", n)
	}

	// The winner disconnects; the asset_owners row stays, so the loser's fresh pick is still page_taken
	// (a Page can never silently move stores) and no credential of the loser appears.
	if r := m.callOn(stores[winner].store, stores[winner].tok, "POST", "/disconnect", true, map[string]any{"page_id": page.ID}); r.Status != 200 {
		t.Fatalf("winner disconnect: %d %s", r.Status, r.Raw)
	}
	loser := 1 - winner
	st := m.mcgFresh(stores[loser].store, stores[loser].tok, user)
	if r := m.callOn(stores[loser].store, stores[loser].tok, "POST", "/pick", true, map[string]any{"state_id": st, "page_id": page.ID, "include_instagram": false}); r.Status != 409 || r.code() != "page_taken" {
		t.Fatalf("loser's pick after the winner disconnected: %d %s (want 409 page_taken)", r.Status, r.Raw)
	}
	if n := m.count(`SELECT count(*) FROM integration.meta_page_credentials k JOIN integration.bindings b ON b.id=k.binding_id
		WHERE b.external_asset_id=$1`, page.ID); n != 0 {
		t.Fatalf("%d credential rows for the Page after the disconnect + refused pick, want 0", n)
	}
}

// ---------------------------------------------------------------------------------------------------------------------
// MPG08 per-Page disconnect (D2/D3): the peer Page's sealed token survives byte-identical and keeps routing; the
// disconnected Page's route is dead (a comment on its bound post stages nothing).
// ---------------------------------------------------------------------------------------------------------------------

func TestMetaConnectIndDisconnectKeepsPeer(t *testing.T) {
	m := newMcnEnv(t)
	e, f, h := m.e, m.f, m.e.h
	m.reset()
	defer m.reset()

	pageA, pageB := mcnPage("MPG08 disc A", true), mcnPage("MPG08 disc B", true)
	for _, p := range []fakegraph.Page{pageA, pageB} {
		st := m.mcgFresh(m.store, m.token, fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{p}})
		if r := m.pick(st, p.ID, true); r.Status != 201 {
			t.Fatalf("pick %s: %d %s", p.ID, r.Status, r.Raw)
		}
	}
	postA, postB := pageA.ID+"_"+mciDigits(10), pageB.ID+"_"+mciDigits(10)
	if _, err := e.putSource(e.session, "page", pageA.ID, postA, false, "zh-TW", true, 0); err != nil {
		t.Fatalf("bind pageA on S1: %v", err)
	}
	h.closeWindow(t, e.session)
	s2 := h.draft(t, f.storeA1)
	h.open(t, s2, claims.MatchExact)
	if _, err := e.putSource(s2, "page", pageB.ID, postB, false, "zh-TW", true, 0); err != nil {
		t.Fatalf("bind pageB on S2: %v", err)
	}
	defer func() {
		mustExec(t, f.owner, `DELETE FROM claims.meta_intake WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3`, f.tenantA, f.storeA1, s2)
		mustExec(t, f.owner, `DELETE FROM live.claim_sources WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3`, f.tenantA, f.storeA1, s2)
	}()

	// Snapshot every sealed credential of BOTH Pages (B's survival is the assertion, A's destruction too).
	type cred struct {
		asset     string
		version   int64
		keyID     string
		nonce, ct []byte
	}
	snap := func() []cred {
		rows, err := f.owner.Query(context.Background(), `SELECT b.external_asset_id,k.version,k.key_id,k.nonce,k.ciphertext
			FROM integration.meta_page_credentials k JOIN integration.bindings b ON b.id=k.binding_id
			WHERE k.store_id=$1 AND b.external_asset_id IN ($2,$3,$4,$5) ORDER BY b.external_asset_id,k.version`,
			m.store, pageA.ID, pageB.ID, pageA.IGID, pageB.IGID)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []cred
		for rows.Next() {
			var c cred
			if err := rows.Scan(&c.asset, &c.version, &c.keyID, &c.nonce, &c.ct); err != nil {
				t.Fatal(err)
			}
			out = append(out, c)
		}
		return out
	}
	before := snap()
	if len(before) != 4 { // A and B, facebook + instagram each
		t.Fatalf("%d credential rows before the disconnect, want 4 (A fb+ig, B fb+ig)", len(before))
	}
	if r := m.call("POST", "/disconnect", true, map[string]any{"page_id": pageA.ID}); r.Status != 200 {
		t.Fatalf("disconnect A: %d %s", r.Status, r.Raw)
	}
	after := snap()
	if len(after) != 2 {
		t.Fatalf("%d credential rows after A's disconnect, want exactly B's 2", len(after))
	}
	for _, c := range after {
		if c.asset != pageB.ID && c.asset != pageB.IGID {
			t.Fatalf("a surviving credential belongs to %s, want only Page B's", c.asset)
		}
	}
	for _, c := range after {
		var match *cred
		for i := range before {
			if before[i].asset == c.asset {
				match = &before[i]
			}
		}
		if match == nil || match.version != c.version || match.keyID != c.keyID ||
			string(match.nonce) != string(c.nonce) || string(match.ct) != string(c.ct) {
			t.Fatalf("Page B's credential of asset %s changed across A's disconnect", c.asset)
		}
	}
	if n := m.count(`SELECT count(*) FROM integration.meta_page_heads h JOIN integration.bindings b ON b.id=h.binding_id
		WHERE b.external_asset_id IN ($1,$2)`, pageA.ID, pageA.IGID); n != 0 {
		t.Fatalf("%d credential heads of Page A survived its disconnect", n)
	}
	if n := m.count(`SELECT count(*) FROM meta_inbox.routes WHERE store_id=$1 AND asset_id IN ($2,$3) AND enabled`,
		m.store, pageB.ID, pageB.IGID); n != 2 {
		t.Fatalf("%d enabled routes of Page B after A's disconnect, want 2", n)
	}

	// A comment on B's bound post still routes to B's session.
	commentB := mciDigits(15) + "_" + mciDigits(10)
	evB := mcPost(t, e.page, pageB.ID, mciFBBody(pageB.ID, postB, commentB, mciDigits(15), "n", "A1", mciAt(2*time.Second), nil))
	mcAwait(t, e.page, evB)
	var got string
	if err := f.owner.QueryRow(context.Background(), `SELECT session_id::text FROM claims.meta_intake
		WHERE object='page' AND asset_id=$1 AND comment_ref=$2`, pageB.ID, commentB).Scan(&got); err != nil || got != s2 {
		t.Fatalf("pageB comment after A's disconnect routed to %q (%v), want S2 %s", got, err, s2)
	}
	// A comment on the DISCONNECTED Page's bound post is quarantined (the route is disabled) and stages nothing.
	commentA := mciDigits(15) + "_" + mciDigits(10)
	raw := mciFBBody(pageA.ID, postA, commentA, mciDigits(15), "n", "A1", mciAt(2*time.Second), nil)
	if code, body := miPost(t, e.page, raw); code != 200 {
		t.Fatalf("webhook post (Meta always gets 200): %d %s", code, body)
	}
	e.apply(t)
	if n := m.count(`SELECT count(*) FROM meta_inbox.events WHERE asset_id=$1 AND disposition='QUARANTINED' AND created_at>clock_timestamp()-interval '2 minutes'`, pageA.ID); n < 1 {
		t.Fatal("a comment on the disconnected Page must be quarantined")
	}
	e.noIntake(t, "page", pageA.ID, commentA, "a comment on the disconnected Page must never become a claim intake")
}

// ---------------------------------------------------------------------------------------------------------------------
// MPG09 custody (D4 / contract §7): no Page token and no sealed bytes in anything the API returns or stores as text.
// ---------------------------------------------------------------------------------------------------------------------

func TestMetaConnectIndNoTokenAnywhere(t *testing.T) {
	m := newMcnEnv(t)
	m.reset()
	defer m.reset()

	page := mcnPage("MPG09 custody", true)
	token := page.Token // SENTINEL-EAAP-<hex24>, unique per Page
	st := m.mcgFresh(m.store, m.token, fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{page}})
	if r := m.pick(st, page.ID, true); r.Status != 201 {
		t.Fatalf("pick: %d %s", r.Status, r.Raw)
	}

	// 1. The status DTO carries neither the token nor any token-shaped key.
	s := m.status()
	if s.Status != 200 {
		t.Fatalf("status: %d %s", s.Status, s.Raw)
	}
	raw := string(s.Raw)
	for _, bad := range []string{token, "access_token", "ciphertext", "nonce", "key_id"} {
		if strings.Contains(raw, bad) {
			t.Fatalf("status leaks %q: %s", bad, raw)
		}
	}
	for _, p := range m.pageList() {
		for k := range p {
			switch k {
			case "id", "name", "status", "instagram", "permissions", "connected_at", "route_expires_at", "last_event_at":
			default:
				t.Fatalf("status page entry carries unexpected key %q: %s", k, raw)
			}
		}
	}
	// 2. The stored pick list (meta_connect_states.pages) and the command receipts never hold the token.
	if n := m.count(`SELECT count(*) FROM integration.meta_connect_states WHERE store_id=$1 AND pages::text LIKE '%'||$2||'%'`, m.store, token); n != 0 {
		t.Fatal("the Page token is in the stored pick list")
	}
	if n := m.count(`SELECT count(*) FROM ops.command_results WHERE store_id=$1 AND response::text LIKE '%'||$2||'%'`, m.store, token); n != 0 {
		t.Fatal("the Page token is in a command receipt")
	}
	// 3. The sealed bytes are not plaintext-recoverable and never appear in the status DTO (base64 or hex).
	var ct []byte
	if err := m.f.owner.QueryRow(context.Background(), `SELECT k.ciphertext FROM integration.meta_page_credentials k
		JOIN integration.bindings b ON b.id=k.binding_id
		WHERE k.store_id=$1 AND b.external_asset_id=$2 AND b.provider='facebook'`, m.store, page.ID).Scan(&ct); err != nil {
		t.Fatalf("sealed credential of the Page: %v", err)
	}
	if strings.Contains(string(ct), token) ||
		strings.Contains(raw, base64.StdEncoding.EncodeToString(ct)) || strings.Contains(raw, hex.EncodeToString(ct)) {
		t.Fatal("the sealed Page credential is plaintext-recoverable or leaks into the status DTO")
	}
	// 4. Disconnect produces exactly one audit row per action and the job holds sealed bytes only (contract R4 §4).
	if r := m.call("POST", "/disconnect", true, map[string]any{"page_id": page.ID}); r.Status != 200 {
		t.Fatalf("disconnect: %d %s", r.Status, r.Raw)
	}
	if n := m.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action !~ '^[a-z][a-z0-9_.:]{0,79}$'`, m.store); n != 0 {
		t.Fatalf("%d audit actions outside the fixed-shape constraint", n)
	}
	if j := m.job(page.ID); j.State != "PENDING" || !j.Sealed {
		t.Fatalf("unsubscribe job: %+v (want PENDING with sealed bytes only)", j)
	}
}
