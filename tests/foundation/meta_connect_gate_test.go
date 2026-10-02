package foundation_test

// meta_connect_gate_test.go: INDEPENDENT test-author gates of unit meta-connect (R4 test wave; the author of these gates is not the
// implementer and wrote them from contracts/meta-claims-intake-v1.md "Merchant connect (R4)" + §7, not from internal/metaconnect).
// Tier: MOCK (REAL_PG + tests/metaconnect/fakegraph on loopback; the webhook -> claim -> reply chain is the MCI harness). Nothing here
// proves Meta, Facebook Login for Business or App Review (SANDBOX/LIVE are NOT_RUN). It reuses only test plumbing from
// meta_connect_test.go (mcnEnv: HTTP handler over the real service, fake Graph, dispatcher) and asserts the CONTRACT, never a
// private symbol of the service.
// Run: bash scripts/dev/test-focused.sh '^TestMetaConnectGate' ./tests/foundation
//
// Gates (ids are used by docs/delivery/GATES.md and output/meta-connect/tests/):
//   MCG01 TestMetaConnectGateState        state: single use, expiry, bound to store + principal, no open redirect surface
//   MCG02 TestMetaConnectGatePermissions  every missing permission / Page task / Instagram permission -> precise refusal, nothing enabled
//   MCG03 TestMetaConnectGateOwnership    one Page -> one store platform-wide (a store holds up to 10 Pages) incl. concurrent races, IG present / absent
//   MCG04 TestMetaConnectGateCustody      no plaintext token in any PG row / log / API response / audit; v2 opens only with the private ring
//   MCG05 TestMetaConnectGateIntake       claims-worker opens and replies (MOCK); Graph 190 -> reauth_required -> reconnect; disconnect stops intake
//   MCG06 TestMetaConnectGateRoles        only integration:manage roles (owner, admin) connect / disconnect
//   MCG07 TestMetaConnectGateAPICannotOpen the API process cannot open a v2 token (static + shape guard)

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/integrations/meta/pagetoken"
	"livecommerce/internal/integrations/metareply"
	"livecommerce/tests/metaconnect/fakegraph"
)

// ---------------------------------------------------------------------------------------------------------------------
// helpers (plumbing only)
// ---------------------------------------------------------------------------------------------------------------------

// mcgWithout is mcnFullPerms minus the named permissions.
func mcgWithout(drop ...string) []string {
	out := []string{}
	for _, p := range mcnFullPerms {
		keep := true
		for _, d := range drop {
			keep = keep && p != d
		}
		if keep {
			out = append(out, p)
		}
	}
	return out
}

// mcgTasksWithout is the full Page task list minus the named task.
func mcgTasksWithout(drop string) []string {
	out := []string{}
	for _, task := range mcnTasks {
		if task != drop {
			out = append(out, task)
		}
	}
	return out
}

// mcgDialogState reads the `state` parameter of a start response the way the browser would return it from Meta.
func mcgDialogState(t *testing.T, started mcnResp) string {
	t.Helper()
	d, err := url.Parse(started.str("dialog_url"))
	if err != nil || d.Query().Get("state") == "" {
		t.Fatalf("start response has no dialog state: %s", started.Raw)
	}
	return d.Query().Get("state")
}

// mcgCallback issues the OAuth return (code + state) on a store's callback as token.
func (m *mcnEnv) mcgCallback(store, token, code, state string) mcnResp {
	return m.callOn(store, token, "GET", "/callback?code="+url.QueryEscape(code)+"&state="+url.QueryEscape(state), false, nil)
}

// mcgPickView is the pick list of a state: page id -> (missing, ig_missing).
type mcgPickView struct{ missing, igMissing []string }

func (m *mcnEnv) mcgPicks(state string) map[string]mcgPickView {
	m.t.Helper()
	r := m.call("GET", "/states/"+state, false, nil)
	if r.Status != 200 {
		m.t.Fatalf("states/%s: %d %s", state, r.Status, r.Raw)
	}
	pages, _ := r.JSON["pages"].([]any)
	out := map[string]mcgPickView{}
	for _, p := range pages {
		row, _ := p.(map[string]any)
		id, _ := row["page_id"].(string)
		out[id] = mcgPickView{missing: toStrings(row["missing"]), igMissing: toStrings(row["ig_missing"])}
	}
	return out
}

// mcgNothingFor fails when any binding, route, credential, head or connection row exists for the assets (never partial-enable).
func (m *mcnEnv) mcgNothingFor(why string, assets ...string) {
	m.t.Helper()
	for _, q := range []string{
		`SELECT count(*) FROM integration.bindings WHERE external_asset_id=ANY($1)`,
		`SELECT count(*) FROM meta_inbox.routes WHERE asset_id=ANY($1)`,
		`SELECT count(*) FROM integration.meta_page_credentials WHERE asset_id=ANY($1)`,
		`SELECT count(*) FROM integration.meta_page_heads h JOIN integration.bindings b ON b.id=h.binding_id WHERE b.external_asset_id=ANY($1)`,
	} {
		if n := m.count(q, assets); n != 0 {
			m.t.Fatalf("%s: %d rows from %.70s", why, n, q)
		}
	}
}

// mcgFresh starts a connect for (store, token) with user and returns the state id (callback must be 200).
func (m *mcnEnv) mcgFresh(store, token string, user fakegraph.User) string {
	m.t.Helper()
	state, cb := m.connect(store, token, user)
	if cb.Status != 200 {
		m.t.Fatalf("callback: %d %s", cb.Status, cb.Raw)
	}
	return state
}

// ---------------------------------------------------------------------------------------------------------------------
// MCG01 state
// ---------------------------------------------------------------------------------------------------------------------

func TestMetaConnectGateState(t *testing.T) {
	m := newMcnEnv(t)
	f := m.f
	m.reset()
	page := mcnPage("MCG01 page", false)
	user := fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{page}}
	exchanges := func() int { return m.fake.Count("GET", "oauth/access_token") }

	t.Run("state is bound to store AND principal: a keyed start replays only for its own principal; the key is another principal's conflict", func(t *testing.T) {
		_, tokOther := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "integration:manage", "integration:read")
		key := t04Key("mcg-samekey")
		a := httptestStart(t, m, m.store, m.token, key)
		replay := httptestStart(t, m, m.store, m.token, key)
		if mcgDialogState(t, a) != mcgDialogState(t, replay) || a.str("state_id") != replay.str("state_id") {
			t.Fatalf("an Idempotency-Key replay returned a DIFFERENT state: %s vs %s", a.Raw, replay.Raw)
		}
		// The same key on the same store presented by another principal is a conflict, never a replay of the owner's receipt
		// (the receipt fingerprint binds the principal).
		if r := mcgRequest(m, m.store, tokOther, "POST", "/start", key); r.Status != 409 {
			t.Fatalf("another principal reused the key on the same store: %d %s", r.Status, r.Raw)
		}
		// Fresh keys for other (store, principal) pairs start their own, different states.
		_, tokA2 := lcPrincipal(t, f, f.tenantA, []string{f.storeA2}, "store:read", "integration:manage", "integration:read")
		b, c := mcgDialogState(t, httptestStart(t, m, f.storeA2, tokA2, t04Key("mcg-b"))), mcgDialogState(t, httptestStart(t, m, m.store, tokOther, t04Key("mcg-c")))
		if b == c || b == mcgDialogState(t, a) || c == mcgDialogState(t, a) {
			t.Fatal("two (store, principal) pairs produced the same OAuth state")
		}
	})

	t.Run("another merchant (other principal / other store / other tenant) presenting the state is refused and reaches no Meta", func(t *testing.T) {
		s := m.call("POST", "/start", true, nil)
		state := mcgDialogState(t, s)
		code := "SYNTH-CODE-" + t04Tag()
		m.fake.AddCode(code, user)
		_, sameStore := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "integration:manage", "integration:read")
		_, otherStore := lcPrincipal(t, f, f.tenantA, []string{f.storeA2}, "store:read", "integration:manage", "integration:read")
		_, otherTenant := lcPrincipal(t, f, f.tenantB, []string{f.storeB}, "store:read", "integration:manage", "integration:read")
		before := exchanges()
		for name, c := range map[string]struct{ store, tok string }{
			"other principal, same store":      {m.store, sameStore},
			"other store principal at its own": {f.storeA2, otherStore},
			"other tenant at its own store":    {f.storeB, otherTenant},
		} {
			r := m.mcgCallback(c.store, c.tok, code, state)
			if r.Status != 409 || r.code() != "state_mismatch" {
				t.Errorf("%s: want 409 state_mismatch, got %d %s", name, r.Status, r.Raw)
			}
			if strings.Contains(string(r.Raw), s.str("state_id")) || strings.Contains(string(r.Raw), f.storeA1) {
				t.Errorf("%s: refusal echoes a scope or state id: %s", name, r.Raw)
			}
		}
		// The owner principal addressing ANOTHER store with this state is refused too (no grant there, or a state of another store).
		if r := m.mcgCallback(f.storeA2, m.token, code, state); r.Status == 200 {
			t.Errorf("state of store A1 was accepted on store A2: %s", r.Raw)
		}
		if got := exchanges(); got != before {
			t.Fatalf("a refused state reached Meta: exchanges %d -> %d", before, got)
		}
		// Even if the state is later honoured for its owner, a foreign principal can neither read the pick list nor pick.
		ok := m.mcgCallback(m.store, m.token, code, state)
		if ok.Status != 200 {
			t.Fatalf("owner callback: %d %s", ok.Status, ok.Raw)
		}
		stateID := ok.str("state_id")
		if r := m.callOn(m.store, sameStore, "GET", "/states/"+stateID, false, nil); r.Status == 200 || strings.Contains(string(r.Raw), page.ID) {
			t.Fatalf("another principal read the pick list: %d %s", r.Status, r.Raw)
		}
		if r := m.callOn(m.store, sameStore, "POST", "/pick", true, map[string]any{"state_id": stateID, "page_id": page.ID, "include_instagram": false}); r.Status == 201 || r.Status == 200 {
			t.Fatalf("another principal picked with the owner's state: %d %s", r.Status, r.Raw)
		}
		m.mcgNothingFor("foreign pick attempts", page.ID)
	})

	t.Run("single use: a replayed callback (same code and state) is refused and never reaches Meta again", func(t *testing.T) {
		s := m.call("POST", "/start", true, nil)
		state := mcgDialogState(t, s)
		code := "SYNTH-CODE-" + t04Tag()
		m.fake.AddCode(code, user)
		if r := m.mcgCallback(m.store, m.token, code, state); r.Status != 200 {
			t.Fatalf("first callback: %d %s", r.Status, r.Raw)
		}
		before := exchanges()
		again := "SYNTH-CODE-" + t04Tag()
		m.fake.AddCode(again, user)
		for _, c := range []string{code, again} {
			r := m.mcgCallback(m.store, m.token, c, state)
			if r.Status != 409 || (r.code() != "state_mismatch" && r.code() != "state_used") {
				t.Fatalf("replayed state with code %s: want 409 state_mismatch|state_used, got %d %s", c[:14], r.Status, r.Raw)
			}
		}
		if exchanges() != before {
			t.Fatal("a replayed state reached Meta (code exchange)")
		}
	})

	t.Run("concurrent callbacks with one state: exactly one wins", func(t *testing.T) {
		s := m.call("POST", "/start", true, nil)
		state := mcgDialogState(t, s)
		const n = 6
		codes := make([]string, n)
		for i := range codes {
			codes[i] = "SYNTH-CODE-" + t04Tag()
			m.fake.AddCode(codes[i], user)
		}
		var wg sync.WaitGroup
		results := make([]mcnResp, n)
		gate := make(chan struct{})
		for i := range codes {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-gate
				results[i] = m.mcgCallback(m.store, m.token, codes[i], state)
			}()
		}
		close(gate)
		wg.Wait()
		wins := 0
		for _, r := range results {
			if r.Status == 200 {
				wins++
			} else if r.Status != 409 {
				t.Errorf("losing callback answered %d %s, want 409", r.Status, r.Raw)
			}
		}
		if wins != 1 {
			t.Fatalf("%d concurrent callbacks consumed one state, want exactly 1", wins)
		}
	})

	t.Run("expiry (10 minutes): an aged state is refused 410 before Meta", func(t *testing.T) {
		s := m.call("POST", "/start", true, nil)
		state := mcgDialogState(t, s)
		var secs float64
		if err := f.owner.QueryRow(context.Background(), `SELECT extract(epoch FROM expires_at-created_at) FROM integration.meta_connect_states WHERE id=$1`, s.str("state_id")).Scan(&secs); err != nil || secs < 595 || secs > 605 {
			t.Fatalf("state lifetime = %.3fs (%v), contract says 10 minutes", secs, err)
		}
		mustExec(t, f.owner, `UPDATE integration.meta_connect_states SET created_at=created_at-interval '2 hours',expires_at=clock_timestamp()-interval '1 second' WHERE id=$1`, s.str("state_id"))
		code := "SYNTH-CODE-" + t04Tag()
		m.fake.AddCode(code, user)
		before := exchanges()
		if r := m.mcgCallback(m.store, m.token, code, state); r.Status != 410 || r.code() != "state_expired" {
			t.Fatalf("expired state: %d %s", r.Status, r.Raw)
		}
		if exchanges() != before {
			t.Fatal("an expired state reached Meta")
		}
	})

	t.Run("expiry on the pick leg: an aged state refuses the pick with nothing enabled", func(t *testing.T) {
		pg := mcnPage("MCG01 pick-expiry", false)
		st := m.mcgFresh(m.store, m.token, fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{pg}})
		mustExec(t, f.owner, `UPDATE integration.meta_connect_states SET created_at=created_at-interval '2 hours',expires_at=clock_timestamp()-interval '1 second' WHERE id=$1`, st)
		subs := m.fake.Count("POST", "/subscribed_apps")
		if r := m.pick(st, pg.ID, false); r.Status != 410 {
			t.Fatalf("pick of an expired state: %d %s", r.Status, r.Raw)
		}
		if m.fake.Count("POST", "/subscribed_apps") != subs {
			t.Fatal("an expired pick called Meta")
		}
		m.mcgNothingFor("expired pick", pg.ID)
	})

	t.Run("no open-redirect surface: the callback accepts exactly {code,state}; extra parameters are refused and nothing redirects", func(t *testing.T) {
		s := m.call("POST", "/start", true, nil)
		state := mcgDialogState(t, s)
		code := "SYNTH-CODE-" + t04Tag()
		m.fake.AddCode(code, user)
		before := exchanges()
		for _, extra := range []string{"&redirect_uri=https://evil.example/", "&next=//evil.example", "&return_to=https%3A%2F%2Fevil.example", "&state=" + state} {
			r := m.callOn(m.store, m.token, "GET", "/callback?code="+code+"&state="+url.QueryEscape(state)+extra, false, nil)
			if r.Status == 200 || r.Status == 302 || r.Status == 303 {
				t.Errorf("callback with extra %q succeeded/redirected: %d %s", extra, r.Status, r.Raw)
			}
		}
		if exchanges() != before {
			t.Fatal("a malformed callback reached Meta")
		}
		// The Go endpoint answers JSON only: no Location, whatever the outcome.
		ok := m.mcgCallback(m.store, m.token, code, state)
		if ok.Status != 200 {
			t.Fatalf("clean callback: %d %s", ok.Status, ok.Raw)
		}
		if strings.Contains(string(ok.Raw), "http") || strings.Contains(string(ok.Raw), code) || strings.Contains(string(ok.Raw), state) {
			t.Fatalf("callback body echoes a URL, the code or the state: %s", ok.Raw)
		}
		// The dialog the merchant is sent to is the configured one only: https://www.facebook.com, the configured redirect URI.
		d, _ := url.Parse(s.str("dialog_url"))
		if d.Scheme != "https" || d.Host != "www.facebook.com" || d.Query().Get("redirect_uri") != mcnRedirect {
			t.Fatalf("dialog is not pinned to www.facebook.com + the configured redirect: %s", s.str("dialog_url"))
		}
	})
}

// httptestStart posts /start with a chosen Idempotency-Key (the generic helper always generates one).
func httptestStart(t *testing.T, m *mcnEnv, store, tok, key string) mcnResp {
	t.Helper()
	r := mcgRequest(m, store, tok, "POST", "/start", key)
	if r.Status != 201 {
		t.Fatalf("start: %d %s", r.Status, r.Raw)
	}
	return r
}

func mcgRequest(m *mcnEnv, store, tok, method, path, key string) mcnResp {
	req := httptest.NewRequest(method, "/v1/admin/stores/"+store+"/meta-connect"+path, nil)
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	rec := httptest.NewRecorder()
	m.handler.ServeHTTP(rec, req)
	out := mcnResp{Status: rec.Code, Raw: rec.Body.Bytes()}
	_ = json.Unmarshal(out.Raw, &out.JSON)
	return out
}

// ---------------------------------------------------------------------------------------------------------------------
// MCG02 permissions and tasks
// ---------------------------------------------------------------------------------------------------------------------

func TestMetaConnectGatePermissions(t *testing.T) {
	m := newMcnEnv(t)
	m.reset()

	type caseT struct {
		name      string
		user      func(page fakegraph.Page) fakegraph.User
		withIG    bool
		page      func() fakegraph.Page
		wantMiss  string // the one precise item the pick list must name (case-insensitive substring)
		igMissing bool   // named in ig_missing instead of missing
	}
	var cases []caseT
	for _, perm := range []string{"pages_show_list", "pages_manage_metadata", "pages_read_engagement", "pages_messaging"} {
		cases = append(cases, caseT{name: "missing permission " + perm, wantMiss: perm,
			page: func() fakegraph.Page { return mcnPage("MCG02 "+perm, false) },
			user: func(p fakegraph.Page) fakegraph.User {
				return fakegraph.User{Permissions: mcgWithout(perm), Pages: []fakegraph.Page{p}}
			}})
	}
	for _, task := range []string{"MESSAGING", "MODERATE"} {
		cases = append(cases, caseT{name: "missing Page task " + task, wantMiss: task,
			page: func() fakegraph.Page {
				p := mcnPage("MCG02 task "+task, false)
				p.Tasks = mcgTasksWithout(task)
				return p
			},
			user: func(p fakegraph.Page) fakegraph.User {
				return fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{p}}
			}})
	}
	for _, perm := range []string{"instagram_basic", "instagram_manage_comments", "instagram_manage_messages"} {
		cases = append(cases, caseT{name: "Instagram kept but missing " + perm, wantMiss: perm, withIG: true, igMissing: true,
			page: func() fakegraph.Page { return mcnPage("MCG02 ig "+perm, true) },
			user: func(p fakegraph.Page) fakegraph.User {
				return fakegraph.User{Permissions: mcgWithout(perm), Pages: []fakegraph.Page{p}}
			}})
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			page := c.page()
			state := m.mcgFresh(m.store, m.token, c.user(page))
			subs, reqs := m.fake.Count("POST", "/subscribed_apps"), len(m.fake.Requests())
			view, ok := m.mcgPicks(state)[page.ID]
			if !ok {
				t.Fatalf("page %s is not in the pick list", page.ID)
			}
			named := view.missing
			if c.igMissing {
				named = view.igMissing
			}
			if len(named) != 1 || !strings.Contains(strings.ToLower(named[0]), strings.ToLower(c.wantMiss)) {
				t.Fatalf("pick list must name exactly %q to re-grant, got missing=%v ig_missing=%v", c.wantMiss, view.missing, view.igMissing)
			}
			ids := []string{page.ID}
			if page.IGID != "" {
				ids = append(ids, page.IGID)
			}
			r := m.pick(state, page.ID, c.withIG)
			if r.Status != 422 || r.code() != "missing_permission" {
				t.Fatalf("pick: want 422 missing_permission, got %d %s", r.Status, r.Raw)
			}
			if strings.Contains(string(r.Raw), "SENTINEL") {
				t.Fatalf("refusal carries a token: %s", r.Raw)
			}
			m.mcgNothingFor(c.name, ids...)
			if m.fake.Count("POST", "/subscribed_apps") != subs {
				t.Fatal("a refused pick called subscribed_apps")
			}
			for _, q := range m.fake.Requests()[reqs:] {
				if strings.HasSuffix(q.Path, "/subscribed_apps") {
					t.Fatalf("refused pick made a Meta call: %+v", q)
				}
			}
			if s := m.status(); s.JSON["connected"] != false {
				t.Fatalf("store reports connected after a refused pick: %s", s.Raw)
			}
		})
	}

	t.Run("control: with every grant the same Page picks (the refusals above are about the grant, not the harness)", func(t *testing.T) {
		page := mcnPage("MCG02 control", true)
		state := m.mcgFresh(m.store, m.token, fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{page}})
		if r := m.pick(state, page.ID, true); r.Status != 201 {
			t.Fatalf("control pick: %d %s", r.Status, r.Raw)
		}
		m.reset()
	})
}

// ---------------------------------------------------------------------------------------------------------------------
// MCG03 ownership
// ---------------------------------------------------------------------------------------------------------------------

func TestMetaConnectGateOwnership(t *testing.T) {
	m := newMcnEnv(t)
	f := m.f
	m.reset()
	_, tokA2 := lcPrincipal(t, f, f.tenantA, []string{f.storeA2}, "store:read", "integration:manage", "integration:read")
	_, tokB := lcPrincipal(t, f, f.tenantB, []string{f.storeB}, "store:read", "integration:manage", "integration:read")
	type who struct{ store, tok string }
	stores := []who{{m.store, m.token}, {f.storeA2, tokA2}, {f.storeB, tokB}}

	t.Run("concurrent double-pick of one Page by three stores: exactly one wins, the others 409 page_taken without a leak", func(t *testing.T) {
		const rounds = 5
		for round := 0; round < rounds; round++ {
			page := mcnPage(fmt.Sprintf("MCG03 race %d", round), round%2 == 0)
			states := make([]string, len(stores))
			for i, s := range stores {
				states[i] = m.mcgFresh(s.store, s.tok, fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{page}})
			}
			results := make([]mcnResp, len(stores))
			var wg sync.WaitGroup
			gate := make(chan struct{})
			for i, s := range stores {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-gate
					results[i] = m.callOn(s.store, s.tok, "POST", "/pick", true, map[string]any{"state_id": states[i], "page_id": page.ID, "include_instagram": page.IGID != ""})
				}()
			}
			close(gate)
			wg.Wait()
			winners := -1
			for i, r := range results {
				switch r.Status {
				case 201:
					if winners != -1 {
						t.Fatalf("round %d: two stores both picked Page %s (%v)", round, page.ID, statuses(results))
					}
					winners = i
				case 409:
					if r.code() != "page_taken" && r.code() != "already_connected" {
						t.Errorf("round %d: loser %d code %q, want page_taken", round, i, r.code())
					}
					for _, id := range []string{f.storeA1, f.storeA2, f.storeB, f.tenantA, f.tenantB} {
						if strings.Contains(string(r.Raw), id) {
							t.Errorf("round %d: loser's refusal leaks a scope id: %s", round, r.Raw)
						}
					}
				default:
					t.Fatalf("round %d: store %d answered %d %s (want 201 or 409): %v", round, i, r.Status, r.Raw, statuses(results))
				}
			}
			if winners == -1 {
				t.Fatalf("round %d: nobody got the Page (%v)", round, statuses(results))
			}
			assets := []string{page.ID}
			if page.IGID != "" {
				assets = append(assets, page.IGID)
			}
			for _, q := range []string{
				`SELECT count(DISTINCT store_id) FROM integration.bindings WHERE external_asset_id=ANY($1) AND enabled`,
				`SELECT count(DISTINCT store_id) FROM meta_inbox.routes WHERE asset_id=ANY($1) AND enabled`,
				`SELECT count(DISTINCT store_id) FROM integration.meta_page_credentials WHERE asset_id=ANY($1)`,
			} {
				if n := m.count(q, assets); n != 1 {
					t.Fatalf("round %d: %d distinct stores hold the Page in %.80s, want 1", round, n, q)
				}
			}
			if n := m.count(`SELECT count(*) FROM integration.bindings WHERE external_asset_id=ANY($1) AND store_id<>$2`, assets, stores[winners].store); n != 0 {
				t.Fatalf("round %d: a losing store has %d binding rows for the Page", round, n)
			}
			if n := m.count(`SELECT count(*) FROM integration.meta_connections WHERE tenant_id IS NOT NULL AND page_id=$1`, page.ID); n != 1 {
				t.Fatalf("round %d: %d connection rows for the Page, want 1", round, n)
			}
			// free every store for the next round
			for _, s := range stores {
				if r := m.callOn(s.store, s.tok, "POST", "/disconnect", true, map[string]any{"page_id": page.ID}); r.Status != 200 && r.Status != 404 {
					t.Fatalf("round %d: disconnect: %d %s", round, r.Status, r.Raw)
				}
			}
		}
	})

	t.Run("one store connects two different Pages concurrently: both 201, two bindings, two rows", func(t *testing.T) {
		defer m.reset() // unconditional: a failure must not cascade into the next subtests
		p1, p2 := mcnPage("MCG03 twin 1", false), mcnPage("MCG03 twin 2", false)
		user := fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{p1, p2}}
		s1, s2 := m.mcgFresh(m.store, m.token, user), m.mcgFresh(m.store, m.token, user)
		var wg sync.WaitGroup
		gate := make(chan struct{})
		out := make([]mcnResp, 2)
		for i, pair := range [][2]string{{s1, p1.ID}, {s2, p2.ID}} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-gate
				out[i] = m.pick(pair[0], pair[1], false)
			}()
		}
		close(gate)
		wg.Wait()
		if out[0].Status != 201 || out[1].Status != 201 {
			t.Fatalf("two concurrent Page picks on one store: %v %s %s", statuses(out), out[0].Raw, out[1].Raw)
		}
		if n := m.count(`SELECT count(*) FROM integration.bindings WHERE store_id=$1 AND enabled AND external_asset_id=ANY($2)`, m.store, []string{p1.ID, p2.ID}); n != 2 {
			t.Fatalf("%d enabled Page bindings on one store, want 2", n)
		}
		if s := m.status(); s.JSON["count"] != float64(2) || s.JSON["cap"] != float64(10) {
			t.Fatalf("status after two Pages: %s", s.Raw)
		}
	})

	t.Run("same state picked twice concurrently: one 201, one refusal, one credential head", func(t *testing.T) {
		defer m.reset()
		page := mcnPage("MCG03 double", false)
		st := m.mcgFresh(m.store, m.token, fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{page}})
		var wg sync.WaitGroup
		gate := make(chan struct{})
		out := make([]mcnResp, 2)
		for i := range out {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-gate
				out[i] = m.pick(st, page.ID, false)
			}()
		}
		close(gate)
		wg.Wait()
		if !((out[0].Status == 201 && out[1].Status == 409) || (out[0].Status == 409 && out[1].Status == 201)) {
			t.Fatalf("double pick of one state: %v %s %s", statuses(out), out[0].Raw, out[1].Raw)
		}
		if n := m.count(`SELECT count(*) FROM integration.meta_page_heads h JOIN integration.bindings b ON b.id=h.binding_id WHERE b.external_asset_id=$1`, page.ID); n != 1 {
			t.Fatalf("%d credential heads for the Page, want 1", n)
		}
	})

	t.Run("Instagram present / absent: IG is bound only when present AND kept; asking for an absent IG is refused", func(t *testing.T) {
		withIG, noIG := mcnPage("MCG03 with IG", true), mcnPage("MCG03 without IG", false)
		user := fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{withIG, noIG}}
		// absent + asked for -> 422, nothing bound
		st := m.mcgFresh(m.store, m.token, user)
		if r := m.pick(st, noIG.ID, true); r.Status != 422 {
			t.Fatalf("include_instagram on a Page without Instagram: %d %s", r.Status, r.Raw)
		}
		m.mcgNothingFor("IG requested on a Page without IG", noIG.ID)
		// absent + not asked -> facebook only
		if r := m.pick(st, noIG.ID, false); r.Status != 201 || r.JSON["instagram"] != false {
			t.Fatalf("Facebook-only pick: %d %s", r.Status, r.Raw)
		}
		if s := m.status(); m.pageList()[0]["instagram"] != nil {
			t.Fatalf("status shows an Instagram account for a Page without one: %s", s.Raw)
		}
		m.reset()
		// present + not kept -> no instagram binding, route or credential
		st = m.mcgFresh(m.store, m.token, user)
		if r := m.pick(st, withIG.ID, false); r.Status != 201 || r.JSON["instagram"] != false {
			t.Fatalf("pick dropping Instagram: %d %s", r.Status, r.Raw)
		}
		if n := m.count(`SELECT count(*) FROM integration.bindings WHERE external_asset_id=$1`, withIG.IGID); n != 0 {
			t.Fatalf("Instagram dropped by the merchant but %d binding rows exist", n)
		}
		m.mcgNothingFor("IG dropped", withIG.IGID)
		m.reset()
		// present + kept -> both providers, both routed, both sealed at v1
		st = m.mcgFresh(m.store, m.token, user)
		if r := m.pick(st, withIG.ID, true); r.Status != 201 || r.JSON["instagram"] != true {
			t.Fatalf("pick keeping Instagram: %d %s", r.Status, r.Raw)
		}
		for _, c := range []struct{ provider, asset string }{{"facebook", withIG.ID}, {"instagram", withIG.IGID}} {
			if m.count(`SELECT count(*) FROM integration.bindings b JOIN meta_inbox.routes r ON r.binding_id=b.id AND r.enabled
				JOIN integration.meta_page_heads h ON h.binding_id=b.id AND h.current_version=1
				WHERE b.provider=$1 AND b.external_asset_id=$2 AND b.enabled AND b.store_id=$3`, c.provider, c.asset, m.store) != 1 {
				t.Fatalf("%s %s: want one enabled binding + enabled route + credential v1", c.provider, c.asset)
			}
		}
		if s := m.status(); m.pageList()[0]["last_event_at"] != nil {
			t.Fatalf("a fresh connection must have no last_event_at: %s", s.Raw)
		}
		m.reset()
	})
}

func statuses(rs []mcnResp) []int {
	out := make([]int, len(rs))
	for i, r := range rs {
		out[i] = r.Status
	}
	return out
}

// ---------------------------------------------------------------------------------------------------------------------
// MCG04 custody
// ---------------------------------------------------------------------------------------------------------------------

// mcgLogCapture redirects slog/log and os.Stderr into one buffer until the returned stop() (which returns everything written).
func mcgLogCapture(t *testing.T) (stop func() string) {
	t.Helper()
	var mu sync.Mutex
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&mcgLockedWriter{mu: &mu, w: &buf}, &slog.HandlerOptions{Level: slog.LevelDebug})))
	oldErr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = io.Copy(&mcgLockedWriter{mu: &mu, w: &buf}, r)
	}()
	var once sync.Once
	var out string
	stop = func() string {
		once.Do(func() {
			os.Stderr = oldErr
			_ = w.Close()
			<-done
			slog.SetDefault(prev)
			mu.Lock()
			out = buf.String()
			mu.Unlock()
		})
		return out
	}
	t.Cleanup(func() { stop() })
	return stop
}

type mcgLockedWriter struct {
	mu *sync.Mutex
	w  io.Writer
}

func (l *mcgLockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

func TestMetaConnectGateCustody(t *testing.T) {
	m := newMcnEnv(t)
	f := m.f
	ctx := context.Background()
	m.reset()
	stopLogs := mcgLogCapture(t)

	page := mcnPage("MCG04 custody", true)
	user := fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{page}}
	var bodies [][]byte
	record := func(rs ...mcnResp) {
		for _, r := range rs {
			bodies = append(bodies, r.Raw)
		}
	}
	start := m.call("POST", "/start", true, nil)
	record(start)
	dialogState := mcgDialogState(t, start)
	code := "SYNTH-CODE-" + t04Tag() + t04Tag()
	short, long := m.fake.AddCode(code, user)
	cb := m.mcgCallback(m.store, m.token, code, dialogState)
	record(cb)
	if cb.Status != 200 {
		t.Fatalf("callback: %d %s", cb.Status, cb.Raw)
	}
	stateID := cb.str("state_id")
	list := m.call("GET", "/states/"+stateID, false, nil)
	pick := m.pick(stateID, page.ID, true)
	status := m.status()
	record(list, pick, status)
	if pick.Status != 201 {
		t.Fatalf("pick: %d %s", pick.Status, pick.Raw)
	}
	// error responses too: a refused replay, a bad state, an unauthorised call
	record(m.mcgCallback(m.store, m.token, code, dialogState), m.mcgCallback(m.store, m.token, "NOT-A-CODE", strings.Repeat("A", 43)),
		m.callOn(m.store, "x", "GET", "/status", false, nil), m.pick(stateID, page.ID, true))

	needles := map[string]string{
		"page access token": page.Token, "short user token": short, "long user token": long, "app secret": miSecret,
		"OAuth code": code, "OAuth state": dialogState,
	}

	t.Run("no plaintext token, code, state or app secret in any PG row of any schema", func(t *testing.T) {
		rows, err := f.owner.Query(ctx, `SELECT table_schema||'.'||table_name FROM information_schema.tables
			WHERE table_type='BASE TABLE' AND table_schema NOT IN ('pg_catalog','information_schema') ORDER BY 1`)
		if err != nil {
			t.Fatal(err)
		}
		var tables []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				t.Fatal(err)
			}
			tables = append(tables, name)
		}
		rows.Close()
		if len(tables) < 50 {
			t.Fatalf("only %d tables enumerated: the scan would be vacuous", len(tables))
		}
		for _, table := range tables {
			for label, needle := range needles {
				if n := m.count(`SELECT count(*) FROM `+table+` t WHERE t::text LIKE '%'||$1||'%'`, needle); n != 0 {
					t.Errorf("%s appears in %d rows of %s", label, n, table)
				}
			}
		}
		// the sealed credential is opaque bytes: the plaintext (UTF-8) is not a substring of nonce or ciphertext either.
		for label, needle := range map[string]string{"page token": page.Token, "long user token": long} {
			if m.count(`SELECT count(*) FROM integration.meta_page_credentials WHERE position(convert_to($1,'UTF8') in ciphertext)>0 OR position(convert_to($1,'UTF8') in nonce)>0`, needle) != 0 {
				t.Errorf("%s is a substring of a stored credential column", label)
			}
		}
	})

	t.Run("the stored credential is meta-page-token-v2: 32-byte encapsulated key, sealed per binding, the user token is not persisted", func(t *testing.T) {
		if n := m.count(`SELECT count(*) FROM integration.meta_page_credentials c JOIN integration.bindings b ON b.id=c.binding_id WHERE b.external_asset_id=ANY($1) AND octet_length(c.nonce)=32 AND c.version=1`,
			[]string{page.ID, page.IGID}); n != 2 {
			t.Fatalf("%d v2 credential rows for the Page + Instagram bindings, want 2", n)
		}
		if n := m.count(`SELECT count(*) FROM information_schema.columns WHERE table_schema='integration' AND table_name IN ('meta_connect_states','meta_connections') AND (column_name ILIKE '%token%' OR column_name ILIKE '%secret%')`); n != 0 {
			t.Fatalf("%d token/secret-named columns on the connect tables", n)
		}
	})

	t.Run("no plaintext token in any API response (success and error bodies)", func(t *testing.T) {
		for i, raw := range bodies {
			for label, needle := range needles {
				if label == "OAuth code" || label == "OAuth state" {
					continue // a refused-callback body must not echo them either, but the dialog_url legitimately carries the state
				}
				if bytes.Contains(raw, []byte(needle)) {
					t.Errorf("response #%d carries the %s: %.120s", i, label, raw)
				}
			}
			if i != 0 && bytes.Contains(raw, []byte(code)) {
				t.Errorf("response #%d echoes the OAuth code", i)
			}
		}
		if bytes.Contains(status.Raw, []byte(dialogState)) || bytes.Contains(pick.Raw, []byte(dialogState)) {
			t.Error("card/pick responses carry the OAuth state")
		}
	})

	t.Run("no plaintext token, code or state in service logs (slog + stderr) during the whole flow", func(t *testing.T) {
		logs := stopLogs()
		for label, needle := range needles {
			if strings.Contains(logs, needle) {
				t.Errorf("%s appears in the captured logs", label)
			}
		}
	})

	t.Run("no token in any Meta-bound URL; tokens travel in Authorization or JSON body only", func(t *testing.T) {
		for _, r := range m.fake.Requests() {
			if r.HasQueryToken || (r.HasClientSecret && r.Path != "oauth/access_token") {
				t.Errorf("secret in a URL: %+v", r)
			}
		}
	})

	t.Run("a v2 credential opens only with the claims-worker's private ring, only under its exact scope", func(t *testing.T) {
		var binding, keyID string
		var nonce, ct []byte
		if err := f.owner.QueryRow(ctx, `SELECT c.binding_id::text,c.key_id,c.nonce,c.ciphertext FROM integration.meta_page_credentials c WHERE c.asset_id=$1 AND c.version=1`, page.ID).Scan(&binding, &keyID, &nonce, &ct); err != nil {
			t.Fatal(err)
		}
		scope := pagetoken.Scope{TenantID: f.tenantA, StoreID: m.store, BindingID: binding, Provider: "facebook", AssetID: page.ID, Version: 1}
		if plain, err := m.open.Open(scope, keyID, nonce, ct); err != nil || string(plain) != page.Token {
			t.Fatalf("private ring does not open the credential: %v", err)
		}
		for name, mutate := range map[string]func(s pagetoken.Scope) pagetoken.Scope{
			"other store":    func(s pagetoken.Scope) pagetoken.Scope { s.StoreID = f.storeA2; return s },
			"other tenant":   func(s pagetoken.Scope) pagetoken.Scope { s.TenantID = f.tenantB; return s },
			"other binding":  func(s pagetoken.Scope) pagetoken.Scope { s.BindingID = randomUUID(); return s },
			"other asset":    func(s pagetoken.Scope) pagetoken.Scope { s.AssetID = miAsset(); return s },
			"other provider": func(s pagetoken.Scope) pagetoken.Scope { s.Provider = "instagram"; return s },
			"other version":  func(s pagetoken.Scope) pagetoken.Scope { s.Version = 2; return s },
		} {
			if _, err := m.open.Open(mutate(scope), keyID, nonce, ct); err == nil {
				t.Errorf("credential opened under a wrong scope (%s): the HPKE info must bind it", name)
			}
		}
		flipped := append([]byte(nil), ct...)
		flipped[0] ^= 1
		if _, err := m.open.Open(scope, keyID, nonce, flipped); err == nil {
			t.Error("a tampered ciphertext opened")
		}
		// a different private ring (another deployment's) cannot open it
		_, other := mcnPageRing(t)
		if _, err := other.Open(scope, keyID, nonce, ct); err == nil {
			t.Error("a foreign private ring opened the credential")
		}
	})

	t.Run("audit rows exist for every step and contain no secret (already covered by the all-tables scan above)", func(t *testing.T) {
		for _, action := range []string{"meta.connect.started", "meta.connect.callback", "meta.connect.page_connected"} {
			if n := m.count(`SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action=$3`, f.tenantA, m.store, action); n < 1 {
				t.Errorf("no audit row for %s", action)
			}
		}
	})
	m.reset()
}

// ---------------------------------------------------------------------------------------------------------------------
// MCG05 intake: claims-worker opens, 190 -> reauth -> reconnect, disconnect stops intake
// ---------------------------------------------------------------------------------------------------------------------

func TestMetaConnectGateIntake(t *testing.T) {
	m := newMcnEnv(t)
	e, f := m.e, m.f
	m.reset()
	page := mcnPage("MCG05 intake", true)
	state := m.mcgFresh(m.store, m.token, fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{page}})
	if r := m.pick(state, page.ID, true); r.Status != 201 {
		t.Fatalf("pick: %d %s", r.Status, r.Raw)
	}
	var fbBinding, igBinding string
	for _, c := range []struct {
		dst      *string
		provider string
		asset    string
	}{{&fbBinding, "facebook", page.ID}, {&igBinding, "instagram", page.IGID}} {
		if err := f.owner.QueryRow(context.Background(), `SELECT id::text FROM integration.bindings WHERE store_id=$1 AND provider=$2 AND external_asset_id=$3 AND enabled`, m.store, c.provider, c.asset).Scan(c.dst); err != nil {
			t.Fatalf("%s binding: %v", c.provider, err)
		}
	}
	e.pageAsset, e.pageBinding, e.igAsset, e.igBinding = page.ID, fbBinding, page.IGID, igBinding
	e.pageToken = page.Token
	e.postID, e.mediaID = page.ID+"_"+mciDigits(10), "178"+mciDigits(13)
	e.srcFB = e.mustSource(t, "page", page.ID, e.postID, true)
	e.srcIG = e.mustSource(t, "instagram", page.IGID, e.mediaID, true)
	g := newMciGraph(t)

	t.Run("a worker WITHOUT the private ring (what the API process is) cannot send: denied before dispatch, no Graph call", func(t *testing.T) {
		noRing := e.newDispatcher(t, g, nil, func(d *mciDispatcher, pool *pgxpool.Pool) {
			routes, err := metareply.RoutesV2(pool, e.link, e.pageKeys, nil, metareply.Config{GraphBaseURL: g.srv.URL, GraphVersion: "v99.0", HTTPClient: g.srv.Client()})
			if err != nil {
				t.Fatal(err)
			}
			d.routes = routes
		})
		r := e.planReply(t, false, "", "A1")
		if r.op == "" {
			t.Fatal("no reply planned for the merchant-connected Page")
		}
		noRing.run(t, r.op)
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			if s, _, _, _ := e.opState(t, r.op); s != "PLANNED" && s != "PENDING" && s != "QUEUED" && s != "" {
				break
			}
			time.Sleep(50 * time.Millisecond) // polls persisted state
		}
		if s, _, _, _ := e.opState(t, r.op); s == "SUCCEEDED" {
			t.Fatal("a dispatcher without the private ring SENT a private reply from a v2 credential")
		}
		if g.posts(r.s.comment) != 0 {
			t.Fatal("a Graph request was made although the v2 token could not be opened")
		}
	})

	d := m.dispatcher(g)
	t.Run("claims-worker (private ring) sends the private reply on Facebook comments and Instagram live comments (MOCK)", func(t *testing.T) {
		for _, ig := range []bool{false, true} {
			r := e.planReply(t, ig, "", "A1")
			if r.op == "" || r.bundleID == "" {
				t.Fatalf("ig=%v: the comment on the connected Page did not become a claim with a planned reply", ig)
			}
			d.run(t, r.op)
			e.awaitOp(t, r.op, "SUCCEEDED", 30*time.Second, "completed")
			var posted *mciGraphReq
			for _, q := range g.all() {
				q := q
				if q.comment == r.s.comment {
					posted = &q
				}
			}
			if posted == nil || posted.bodyToken != page.Token || !strings.HasSuffix(posted.path, "/"+r.asset+"/messages") {
				t.Fatalf("ig=%v: reply not sent with the connected token on the right asset: %+v", ig, posted)
			}
		}
		if la, _ := m.pageList()[0]["last_event_at"].(string); la == "" {
			t.Fatalf("card must show last_event_at after routed comments: %s", m.status().Raw)
		}
	})

	t.Run("a non-190 Graph failure (429, 5xx) never flips the card; a Graph 190 does, and only that", func(t *testing.T) {
		for _, mode := range []string{"429", "5xx"} {
			r := e.planReply(t, false, "", "A1")
			g.setMode(mode)
			d.run(t, r.op)
			e.awaitOp(t, r.op, "UNKNOWN", 40*time.Second, "cancelled", "discarded", "completed")
			g.setMode("ok")
			if pages := m.pageList(); len(pages) == 0 || pages[0]["status"] != "active" {
				t.Fatalf("mode %s flipped the card", mode)
			}
		}
		r := e.planReply(t, false, "", "A1")
		g.setMode("400") // error.code 190
		d.run(t, r.op)
		e.awaitOp(t, r.op, "UNKNOWN", 40*time.Second, "cancelled", "discarded", "completed")
		g.setMode("ok")
		s := m.status()
		if pages := m.pageList(); len(pages) == 0 || pages[0]["status"] != "reauth_required" {
			t.Fatalf("Graph 190 must flip the card to reauth_required, got %s", s.Raw)
		}
		if s.JSON["connected"] != true {
			t.Fatalf("reauth_required keeps the connection record (so Reconnect is offered): %s", s.Raw)
		}
		// UNKNOWN is never blind-retried: exactly one POST for that comment
		if n := g.posts(r.s.comment); n != 1 {
			t.Fatalf("the 190 reply was sent %d times; an UNKNOWN outcome must not be retried", n)
		}
	})

	t.Run("reconnect (rotated Page token) -> active; the next reply uses the NEW token; one binding per asset still", func(t *testing.T) {
		rotated := page
		rotated.Token = "SENTINEL-EAAP-ROTATED-" + t04Tag() + t04Tag()
		st := m.mcgFresh(m.store, m.token, fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{rotated}})
		if r := m.pick(st, page.ID, true); r.Status != 201 {
			t.Fatalf("reconnect pick: %d %s", r.Status, r.Raw)
		}
		if pages := m.pageList(); len(pages) == 0 || pages[0]["status"] != "active" {
			t.Fatalf("status after reconnect: %s", m.status().Raw)
		}
		if m.count(`SELECT count(*) FROM integration.bindings WHERE store_id=$1 AND external_asset_id=ANY($2)`, m.store, []string{page.ID, page.IGID}) != 2 {
			t.Fatal("reconnect created a second binding row instead of reusing the first")
		}
		r := e.planReply(t, false, "", "A1")
		d.run(t, r.op)
		e.awaitOp(t, r.op, "SUCCEEDED", 30*time.Second, "completed")
		last := g.all()[len(g.all())-1]
		if last.comment != r.s.comment || last.bodyToken != rotated.Token {
			t.Fatal("the reply after a reconnect did not use the rotated Page token")
		}
	})

	t.Run("disconnect stops intake: a webhook comment afterwards makes no claim, no reply operation and no Graph call", func(t *testing.T) {
		bundlesBefore := e.bundleCount(t)
		if r := m.call("POST", "/disconnect", true, map[string]any{"page_id": page.ID}); r.Status != 200 {
			t.Fatalf("disconnect: %d %s", r.Status, r.Raw)
		}
		if s := m.status(); s.JSON["connected"] != false {
			t.Fatalf("status after disconnect: %s", s.Raw)
		}
		comment := mciDigits(15) + "_" + mciDigits(10)
		raw := mciFBBody(page.ID, e.postID, comment, mciDigits(15), "n", "A1", mciAt(2*time.Second), nil)
		if code, body := miPost(t, e.page, raw); code != 200 {
			t.Fatalf("webhook post (Meta always gets 200): %d %s", code, body)
		}
		// give the consumer + poller every chance to act, then assert nothing happened
		e.apply(t)
		if m.count(`SELECT count(*) FROM meta_inbox.events WHERE asset_id=$1 AND disposition='QUARANTINED' AND created_at>clock_timestamp()-interval '2 minutes'`, page.ID) < 1 {
			t.Error("a comment on a disconnected Page must be quarantined")
		}
		e.noIntake(t, "page", page.ID, comment, "a comment after disconnect must never become a claim intake")
		if e.opCount(t, comment) != 0 {
			t.Error("a reply operation was planned for a comment after disconnect")
		}
		if g.posts(comment) != 0 {
			t.Error("a private reply was sent for a comment after disconnect")
		}
		if e.bundleCount(t) != bundlesBefore {
			t.Error("a claim bundle was created after disconnect")
		}
		if m.count(`SELECT count(*) FROM integration.meta_page_credentials WHERE asset_id=ANY($1)`, []string{page.ID, page.IGID}) != 0 {
			t.Error("the sealed token survived the disconnect")
		}
	})
}

// ---------------------------------------------------------------------------------------------------------------------
// MCG06 roles
// ---------------------------------------------------------------------------------------------------------------------

func TestMetaConnectGateRoles(t *testing.T) {
	m := newMcnEnv(t)
	f := m.f
	ctx := context.Background()
	m.reset()
	page := mcnPage("MCG06 roles", false)

	for _, role := range []string{"owner", "admin", "live_operator", "fulfilment", "viewer"} {
		role := role
		t.Run("role "+role, func(t *testing.T) {
			var perms []string
			rows, err := f.owner.Query(ctx, `SELECT unnest(identity.staff_role_permissions($1))`, role)
			if err != nil {
				t.Fatal(err)
			}
			for rows.Next() {
				var p string
				if err := rows.Scan(&p); err != nil {
					t.Fatal(err)
				}
				perms = append(perms, p)
			}
			rows.Close()
			if len(perms) == 0 {
				t.Fatalf("role %s has an empty permission bundle", role)
			}
			holds := func(p string) bool {
				for _, x := range perms {
					if x == p {
						return true
					}
				}
				return false
			}
			wantManage := role == "owner" || role == "admin"
			if holds("integration:manage") != wantManage {
				t.Fatalf("contract: only owner/admin hold integration:manage; role %s holds it = %v", role, holds("integration:manage"))
			}
			principal, tok := lcPrincipal(t, f, f.tenantA, []string{m.store}, perms...)
			starts := m.callOn(m.store, tok, "POST", "/start", true, nil)
			if wantManage {
				if starts.Status != 201 {
					t.Fatalf("%s start: %d %s", role, starts.Status, starts.Raw)
				}
				state := mcgDialogState(t, starts)
				code := "SYNTH-CODE-" + t04Tag()
				m.fake.AddCode(code, fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{page}})
				cb := m.mcgCallback(m.store, tok, code, state)
				if cb.Status != 200 {
					t.Fatalf("%s callback: %d %s", role, cb.Status, cb.Raw)
				}
				// not connected yet in this subtest; disconnect on a clean store is 404, but must NOT be 403
				if r := m.callOn(m.store, tok, "POST", "/disconnect", true, map[string]any{"page_id": page.ID}); r.Status == 403 {
					t.Fatalf("%s may disconnect but got 403", role)
				}
				return
			}
			// not allowed: every mutating step is 403, and nothing is written
			for _, c := range []struct {
				method, path string
				body         any
			}{{"POST", "/start", nil}, {"POST", "/disconnect", map[string]any{"page_id": page.ID}},
				{"POST", "/pick", map[string]any{"state_id": randomUUID(), "page_id": page.ID, "include_instagram": false}},
				{"GET", "/states/" + randomUUID(), nil}} {
				r := m.callOn(m.store, tok, c.method, c.path, c.method == "POST", c.body)
				if r.Status != 403 {
					t.Errorf("%s %s %s: want 403, got %d %s", role, c.method, c.path, r.Status, r.Raw)
				}
			}
			// a manager's valid state presented by this role at the callback is refused
			ownerState := m.call("POST", "/start", true, nil)
			code := "SYNTH-CODE-" + t04Tag()
			m.fake.AddCode(code, fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{page}})
			before := m.fake.Count("GET", "oauth/access_token")
			if r := m.mcgCallback(m.store, tok, code, mcgDialogState(t, ownerState)); r.Status == 200 {
				t.Errorf("%s completed a callback with a manager's state", role)
			}
			if m.fake.Count("GET", "oauth/access_token") != before {
				t.Errorf("%s reached Meta through the callback", role)
			}
			if n := m.count(`SELECT count(*) FROM integration.meta_connect_states WHERE principal_id=$1`, principal); n != 0 {
				t.Errorf("%s left %d connect-state rows", role, n)
			}
			if n := m.count(`SELECT count(*) FROM ops.audit_events WHERE principal_id=$1 AND action LIKE 'meta.connect.%'`, principal); n != 0 {
				t.Errorf("%s left %d connect audit rows", role, n)
			}
			// read of the card: needs integration:read
			st := m.callOn(m.store, tok, "GET", "/status", false, nil)
			if holds("integration:read") && st.Status != 200 {
				t.Errorf("%s holds integration:read but status = %d", role, st.Status)
			}
			if !holds("integration:read") && st.Status != 403 {
				t.Errorf("%s lacks integration:read but status = %d", role, st.Status)
			}
		})
	}
	t.Run("no session: refused (401, or 422 when body validation runs first) and nothing is written", func(t *testing.T) {
		states := m.count(`SELECT count(*) FROM integration.meta_connect_states`)
		for _, c := range []struct{ method, path string }{{"POST", "/start"}, {"GET", "/status"}, {"POST", "/pick"}, {"POST", "/disconnect"}, {"GET", "/callback?code=a&state=" + strings.Repeat("A", 43)}} {
			r := mcgRequest(m, m.store, "", c.method, c.path, "")
			if r.Status != 401 && r.Status != 422 {
				t.Errorf("%s %s without a session: %d %s", c.method, c.path, r.Status, r.Raw)
			}
			if r.Status == 200 || r.Status == 201 {
				t.Errorf("%s %s without a session SUCCEEDED", c.method, c.path)
			}
		}
		if n := m.count(`SELECT count(*) FROM integration.meta_connect_states`); n != states {
			t.Errorf("an unauthenticated call wrote %d state rows", n-states)
		}
	})
}

// ---------------------------------------------------------------------------------------------------------------------
// MCG07 the API process cannot open a v2 token
// ---------------------------------------------------------------------------------------------------------------------

// TestMetaConnectGateAPICannotOpen: contract §7 amendment (integrator ruling 2026-10-01): the API may SEAL, never OPEN. The guard
// fails when cmd/api could open: it links an opener package, reads the private ring or the v1 AES ring, the compose api service or an
// env example mounts the private ring, the manifest lists api as a consumer of it, or the seal-side package gained an opener.
func TestMetaConnectGateAPICannotOpen(t *testing.T) {
	root := "../.."
	read := func(p string) string {
		raw, err := os.ReadFile(root + "/" + p)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	opener := regexp.MustCompile(`(?i)(HPKE_PRIVATE|PAGE_TOKEN_KEYS|page_hpke_private|commerce_meta_page_token_|pageopen|PrivateKey)`)

	t.Run("cmd/api dependency closure contains no opener", func(t *testing.T) {
		out, err := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}", root+"/cmd/api").CombinedOutput()
		if err != nil {
			t.Fatalf("go list: %v %s", err, out)
		}
		deps := "\n" + string(out)
		for _, bad := range []string{"integrations/meta/pagetoken/pageopen", "integrations/metareply", "integrations/meta_ads/tokenopen"} {
			if strings.Contains(deps, "\nlivecommerce/internal/"+bad+"\n") {
				t.Errorf("cmd/api links %s: it could open a stored token", bad)
			}
		}
		if !strings.Contains(deps, "\nlivecommerce/internal/integrations/meta/pagetoken\n") {
			t.Error("cmd/api does not link the seal package: the merchant connect could not seal")
		}
	})

	t.Run("cmd/api sources never name a private ring, the v1 AES keyring or an opener", func(t *testing.T) {
		entries, err := os.ReadDir(root + "/cmd/api")
		if err != nil {
			t.Fatal(err)
		}
		for _, en := range entries {
			if en.IsDir() || !strings.HasSuffix(en.Name(), ".go") || strings.HasSuffix(en.Name(), "_test.go") {
				continue
			}
			src := read("cmd/api/" + en.Name())
			// Comments may explain the rule; code may not use it.
			for i, line := range strings.Split(src, "\n") {
				code := line
				if j := strings.Index(code, "//"); j >= 0 {
					code = code[:j]
				}
				if opener.MatchString(code) && !strings.Contains(code, "ADS_TOKEN_HPKE_PUBLIC") {
					t.Errorf("cmd/api/%s:%d names an opening key/opener: %s", en.Name(), i+1, strings.TrimSpace(line))
				}
			}
		}
	})

	t.Run("compose: api mounts the public ring only; claims-worker alone mounts the private ring", func(t *testing.T) {
		compose := read("deploy/compose.yml")
		block := func(service string) string {
			start := strings.Index(compose, "\n  "+service+":\n")
			if start < 0 {
				t.Fatalf("compose has no %s service", service)
			}
			rest := compose[start+1:]
			end := regexp.MustCompile(`\n  [a-z][a-z0-9-]*:\n`).FindStringIndex(rest[1:])
			if end == nil {
				return rest
			}
			return rest[:end[0]+1]
		}
		for _, svc := range []string{"api", "admin", "storefront", "meta-worker", "payment-worker-sandbox", "payment-worker-live", "expiry-worker", "ads-worker"} {
			// Only the PAGE-token opening secrets count here: the v1 AES ring (commerce_meta_page_token_keys_json) and the
			// v2 HPKE private ring. The ads-worker's own ADS ring (commerce_meta_ads_hpke_private_keys_json) is a different
			// custody domain (meta-ads-v1) and must not trip this guard.
			if b := block(svc); regexp.MustCompile(`(?i)commerce_meta_page_token_|page_hpke_private`).MatchString(b) {
				t.Errorf("compose service %s mounts a Page-token opening secret", svc)
			}
		}
		if b := block("claims-worker"); !strings.Contains(b, "commerce_meta_page_hpke_private_keys_json") {
			t.Error("claims-worker does not mount the private ring: it could not send replies from v2 tokens")
		}
		if b := block("api"); !strings.Contains(b, "COMMERCE_META_PAGE_HPKE_PUBLIC_KEYS_JSON") {
			t.Error("api does not get the public ring: it could not seal")
		}
	})

	t.Run("env examples: the api env file carries no private-ring name", func(t *testing.T) {
		if opener.MatchString(regexp.MustCompile(`(?m)^\s*#.*$`).ReplaceAllString(read("deploy/env/api.env.example"), "")) {
			t.Error("deploy/env/api.env.example references an opening key")
		}
	})

	t.Run("secrets manifest: api is never a consumer of the private ring", func(t *testing.T) {
		seen := false
		for _, line := range strings.Split(read("deploy/secrets.manifest.tsv"), "\n") {
			cols := strings.Split(line, "\t")
			if strings.HasPrefix(line, "#") || len(cols) < 4 {
				continue
			}
			if cols[0] == "commerce_meta_page_hpke_private_keys_json" {
				seen = true
				for _, c := range strings.Split(cols[3], ",") {
					if strings.TrimSpace(c) == "api" {
						t.Errorf("manifest lists api as a consumer of %s", cols[0])
					}
				}
			}
		}
		if !seen {
			t.Error("manifest has no commerce_meta_page_hpke_private_keys_json row")
		}
	})

	t.Run("the seal-side package exposes no open capability", func(t *testing.T) {
		typ := reflect.TypeOf(&pagetoken.SealKeys{})
		for i := 0; i < typ.NumMethod(); i++ {
			name := strings.ToLower(typ.Method(i).Name)
			for _, bad := range []string{"open", "decrypt", "decap", "reveal", "private", "unseal"} {
				if strings.Contains(name, bad) {
					t.Errorf("SealKeys has method %s", typ.Method(i).Name)
				}
			}
		}
		src := read("internal/integrations/meta/pagetoken/pagetoken.go")
		for _, bad := range []string{"NewPrivateKey", "Decap", "hpke.Open", ".Open("} {
			if strings.Contains(src, bad) {
				t.Errorf("package pagetoken (linked into the API) contains %q", bad)
			}
		}
	})
}
