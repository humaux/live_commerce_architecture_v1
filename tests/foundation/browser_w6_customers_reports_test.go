//go:build browser

// Purpose: W6-U1 genuine Go/PG plus signed HTTPS OIDC browser acceptance; no successful API response mocks.
// Depends on: rpWorld synthetic report corpus, scoped customers/loyalty commands and existing checkout seed leaf, identity/httpapi, production Next,
// Playwright and LC_W6UI_{REPORTS,CUSTOMERS}_ACCEPTANCE; requires LC_TEST_DATABASE_ALLOWED.
// Used by: GitHub --browser-reports and extended --browser-customers-billing gates; helpers use w6ui only.
// Invariants: I01/I02/I05/I06/I11/I14/I18; report seeds are disclosed MOCK data, not provider/capture acceptance.
package foundation_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/buyer"
	"livecommerce/internal/customers"
	"livecommerce/internal/fulfillment"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/identity"
	"livecommerce/internal/identityhttp"
	"livecommerce/internal/loyalty"
	"livecommerce/internal/merchantorders"
	"livecommerce/internal/oidclogin"
	"livecommerce/internal/pagination"
	"livecommerce/internal/platform"
	"livecommerce/internal/pricing"
	"livecommerce/internal/storefront"
)

type w6uiFixture struct {
	w                                                                                                 *rpWorld
	principal, token, customer, second, note, session                                                 string
	actors                                                                                            map[string]string
	mu                                                                                                sync.Mutex
	fault                                                                                             string
	requests                                                                                          []w6uiRequest
	member, zeroMember, erasedMember, emptyStore, foreignStore, foreignMember, seedActor, memberOrder string
	held                                                                                              chan struct{}
	heldStarted                                                                                       bool
	formsStore, formsAlt, formsPrincipal, formsToken                                                  string
	formsHeld                                                                                         chan struct{}
	formsStarted                                                                                      bool
}

type w6uiRequest struct {
	Method, Path, Key string
	Status            int
}

// TestW6UIFaultWrapper is DB-free harness calibration: a lost response happens after commit, once, with passthrough retry.
func TestW6UIFaultWrapper(t *testing.T) {
	x := &w6uiFixture{fault: "drop-note"}
	var commits atomic.Int64
	s := httptest.NewServer(w6uiWrap(x, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		commits.Add(1)
		w.Header().Set("X-Actual-Handler", "yes")
		w.WriteHeader(201)
		_, _ = w.Write([]byte("committed"))
	})))
	defer s.Close()
	client := &http.Client{Timeout: time.Second}
	for i := 0; i < 2; i++ {
		r, _ := http.NewRequest("POST", s.URL+"/v1/admin/stores/fixture/customers/fixture/notes", strings.NewReader("fixture"))
		r.Header.Set("Idempotency-Key", "w6ui-fixed-intent")
		response, e := client.Do(r)
		if i == 0 {
			if e == nil {
				_ = response.Body.Close()
				t.Fatal("W6UI-DROP-AFTER-COMMIT: first response was not dropped")
			}
		}
		if i == 1 {
			if e != nil {
				t.Fatal(e)
			}
			_ = response.Body.Close()
			if response.StatusCode != 201 || response.Header.Get("X-Actual-Handler") != "yes" {
				t.Fatal("W6UI-DROP-AFTER-COMMIT: retry not passed to actual handler")
			}
		}
	}
	if commits.Load() != 2 {
		t.Fatalf("W6UI-DROP-AFTER-COMMIT: actual handler commits=%d want2; fault must drop only after first handler invocation", commits.Load())
	}
}

// TestBrowserW6Reports drives every report tab/control through a real signed browser session.
func TestBrowserW6Reports(t *testing.T) { w6uiRun(t, "reports") }

// TestBrowserW6Customers drives tags/notes/filter/CAS/UNKNOWN controls, with independent SQL readback.
func TestBrowserW6Customers(t *testing.T) { w6uiRun(t, "customers") }

// TestW6UISeedFocused exercises only the real Go/PG seed used by both W6 browser gates, with no Next/browser process.
func TestW6UISeedFocused(t *testing.T) {
	if os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Skip("use scripts/dev/test-focused.sh with LC_FOCUSED_TAGS=browser")
	}
	x := w6uiSeed(t)
	err := x.scope(func(ctx context.Context, tx pgx.Tx, scope platform.Scope) error {
		_, err := customers.Get(ctx, tx, scope, x.token, x.customer)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestW6UIMembersOrderAPIFocused proves the member corpus through the production router without Next or a browser.
func TestW6UIMembersOrderAPIFocused(t *testing.T) {
	if os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use focused runner with browser tag")
	}
	x := w6uiSeed(t)
	reportStore := x.w.store
	h := httpapi.NewHandler(x.w.f.runtime, httpapi.Options{SessionStoreList: true, PaymentEnvironment: "SANDBOX"})
	read := func(store, resource, token string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest("GET", "/v1/admin/stores/"+store+"/"+resource, nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	reportPath := "reports/products?from=2026-09-01&to=2026-09-30"
	originalReport := read(reportStore, reportPath, x.token)
	if originalReport.Code != 200 {
		t.Fatalf("report control status=%d", originalReport.Code)
	}
	w6uiSeedMembers(t, x)
	reader := lcToken(t, x.w.f, x.actors["reader"])
	none := lcToken(t, x.w.f, x.actors["none"])
	snapshot := func() string {
		t.Helper()
		var state string
		err := x.w.f.owner.QueryRow(context.Background(), `SELECT jsonb_build_array(
   (SELECT count(*) FROM checkout.orders), (SELECT count(*) FROM checkout.payment_attempts),
   (SELECT count(*) FROM payments.facts), (SELECT count(*) FROM inventory.ledger),
   (SELECT count(*) FROM ops.command_results), (SELECT count(*) FROM ops.audit_events),
   (SELECT count(*) FROM loyalty.members), (SELECT count(*) FROM loyalty.point_entries),
   (SELECT coalesce(sum(balance),0) FROM loyalty.members))::text`).Scan(&state)
		if err != nil {
			t.Fatal(err)
		}
		return state
	}
	before := snapshot()
	defer func() {
		if after := snapshot(); after != before {
			t.Errorf("read-only business state changed: %s -> %s", before, after)
		}
	}()
	list := "orders?view=v2&limit=10&state=active"
	response := read(x.w.store, list, reader)
	var code struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(response.Body.Bytes(), &code)
	t.Logf("native ListV2 status=%d typed_code=%q", response.Code, code.Code)
	if response.Code != 200 {
		// Read-only diagnostic at the real definer seam; expose error class/SQLSTATE and structural counts only.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := platform.WithScope(ctx, x.w.f.runtime, reader, x.w.store, "orders:read", func(tx pgx.Tx, sc platform.Scope) error {
			_, err := merchantorders.ListV2(ctx, tx, sc, reader, merchantorders.ListV2Request{State: "active", Page: pagination.Request{Limit: 10}})
			t.Logf("scoped ListV2 unavailable=%t", errors.Is(err, merchantorders.ErrUnavailable))
			var raw []byte
			// Calls identity.read_merchant_orders_v2 (merchant orders v2); read-only diagnostics of the identical selected corpus.
			pgErr := tx.QueryRow(ctx, `SELECT identity.read_merchant_orders_v2($1,$2,11,NULL,NULL,'active','all','','','',NULL,NULL,NULL)`, tokenHash(reader), x.w.store).Scan(&raw)
			var pg *pgconn.PgError
			sqlstate := "none"
			if errors.As(pgErr, &pg) {
				sqlstate = pg.Code
			}
			t.Logf("raw projection SQLSTATE=%s success=%t", sqlstate, pgErr == nil)
			return pgErr
		})
		if err != nil {
			t.Logf("diagnostic transaction error class=%T", err)
		}
		var malformed, real int
		if err := x.w.f.owner.QueryRow(context.Background(), `SELECT count(*) FILTER(WHERE NOT(snapshot ? 'destination') OR NOT(snapshot#>'{quote}' ? 'totals')),count(*) FILTER(WHERE id=$2) FROM checkout.orders WHERE store_id=$1`, x.w.store, x.memberOrder).Scan(&malformed, &real); err != nil {
			t.Fatal(err)
		}
		t.Logf("selected corpus missing destination/totals=%d known native order=%d", malformed, real)
		t.Errorf("member-mode native ListV2 status=%d want200 code=%q", response.Code, code.Code)
	} else {
		var page merchantorders.PageV2
		if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		// 0110 matched predicate excludes DRAFT from active; checkout begin has not attempted payment.
		if page.Total != 0 || len(page.Items) != 0 {
			t.Errorf("active native DRAFT list: total=%d items=%d want0", page.Total, len(page.Items))
		}
		all := read(x.w.store, "orders?view=v2&limit=10&state=all", reader)
		if err := json.Unmarshal(all.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		if all.Code != 200 || page.Total != 1 || len(page.Items) != 1 || page.Items[0].OrderID != x.memberOrder {
			t.Errorf("all native order list: status=%d total=%d items=%d; expected exact checkout identity", all.Code, page.Total, len(page.Items))
		}
	}
	detail := read(x.w.store, "orders/"+x.memberOrder, reader)
	var body struct {
		ID    string `json:"order_id"`
		State string `json:"commercial_state"`
	}
	if err := json.Unmarshal(detail.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if detail.Code != 200 || body.ID != x.memberOrder || body.State != "DRAFT" {
		t.Errorf("native detail status=%d identity_match=%t", detail.Code, body.ID == x.memberOrder)
	}
	for _, item := range []struct {
		name, store, token string
		want               int
	}{
		{"no orders permission", x.w.store, none, 403}, {"foreign store", x.foreignStore, reader, 404},
	} {
		t.Run(item.name, func(t *testing.T) {
			if r := read(item.store, list, item.token); r.Code != item.want {
				t.Errorf("status=%d want%d", r.Code, item.want)
			}
		})
	}
	if after := read(reportStore, reportPath, x.token); after.Code != originalReport.Code || after.Body.String() != originalReport.Body.String() {
		t.Error("existing report-mode oracle changed")
	}
	if got := w6uiMemberCounts(t, x); got != [3]int{24, 53, 78} {
		t.Errorf("member fixture counts=%v", got)
	}
}

func w6uiRun(t *testing.T, mode string) {
	if os.Getenv("LC_W6UI_"+strings.ToUpper(mode)+"_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use the registered isolated W6 browser gate in GitHub CI")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	x := w6uiSeed(t)
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	evidence := brfEvidence(t, root, "w6-"+mode)
	stack, control := w6uiStart(t, ctx, x, evidence)
	env := map[string]string{
		"LC_W6UI_MODE": mode, "LC_W6UI_STORE": x.w.store, "LC_W6UI_CUSTOMER": x.customer,
		"LC_W6UI_SECOND": x.second, "LC_W6UI_SESSION": x.session, "LC_W6UI_PRINCIPAL": x.principal,
		"LC_W6UI_CONTROL_URL": control.URL, "LC_W6UI_CONTROL_KEY": control.key,
	}
	w6uiSpecs(t, ctx, stack, mode, env)
	// The browser ledger proves user clicks; SQL here independently proves persisted facts, never replaces the clicks.
	if mode == "reports" {
		for _, report := range []string{"products", "channels", "funnel", "manual_orders"} {
			if n := countRows(t, x.w.f.owner, `SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action=$2`, x.w.store, "reports.export."+report); n != 4 {
				t.Errorf("%s exports=%d, want exactly four locale/viewport downloads", report, n)
			}
		}
	} else {
		for _, action := range []string{"customers.tag_created", "customers.tag_renamed", "customers.tag_deleted", "customers.tagged", "customers.note_added", "customers.note_edited", "customers.note_deleted"} {
			if n := countRows(t, x.w.f.owner, `SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action=$2`, x.w.store, action); n < 1 {
				t.Errorf("missing persisted %s", action)
			}
		}
		if n := countRows(t, x.w.f.owner, `SELECT count(*) FROM customers.notes WHERE store_id=$1 AND body='w6ui-unknown-once'`, x.w.store); n != 1 {
			t.Errorf("UNKNOWN retry created %d notes, want one", n)
		}
		if n := countRows(t, x.w.f.owner, `SELECT count(*) FROM ops.command_results WHERE store_id=$1 AND operation LIKE 'customers.%' AND response::text LIKE '%w6ui-unknown-once%'`, x.w.store); n != 0 {
			t.Errorf("private body leaked to %d receipts", n)
		}
		if n := countRows(t, x.w.f.owner, `SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND details::text LIKE '%w6ui-%'`, x.w.store); n != 0 {
			t.Errorf("private fixture body leaked to %d audit details", n)
		}
	}
	w6uiAuthorityReadback(t, x)
	if mode == "customers" {
		t.Run("members-read", func(t *testing.T) {
			w6uiSeedMembers(t, x)
			env["LC_W6UI_STORE"] = x.w.store
			before := w6uiMemberCounts(t, x)
			if before != [3]int{24, 53, 78} {
				t.Fatalf("member fixture counts=%v want [24 53 78]", before)
			}
			if n := countRows(t, x.w.f.owner, `SELECT count(*) FROM loyalty.members WHERE store_id=$1 AND erased_at IS NULL`, x.w.store); n != 23 {
				t.Fatalf("active members=%d want23", n)
			}
			env["LC_W6UI_ORDER"] = x.memberOrder
			env["LC_W6UI_MEMBER"], env["LC_W6UI_ZERO_MEMBER"], env["LC_W6UI_ERASED_MEMBER"] = x.member, x.zeroMember, x.erasedMember
			env["LC_W6UI_EMPTY_STORE"], env["LC_W6UI_FOREIGN_STORE"], env["LC_W6UI_FOREIGN_MEMBER"], env["LC_W6UI_SEED_ACTOR"] = x.emptyStore, x.foreignStore, x.foreignMember, x.seedActor
			w6uiSpecs(t, ctx, stack, "members-read", env)
			after := w6uiMemberCounts(t, x)
			if before != after {
				t.Errorf("read journeys mutated member/ledger/command counts: %v -> %v", before, after)
			}
			data, _ := json.MarshalIndent(map[string]any{"before": before, "after": after, "classification": "REAL_PG; control/session writes excluded"}, "", "  ")
			if e := os.WriteFile(filepath.Join(evidence, "members-read-counts.json"), data, 0600); e != nil {
				t.Fatal(e)
			}
			data, e := os.ReadFile(filepath.Join(evidence, "members-read-click-ledger.json"))
			var rows []struct {
				Result string `json:"result"`
			}
			if e != nil || json.Unmarshal(data, &rows) != nil || len(rows) == 0 {
				t.Fatal("missing members read click ledger")
			}
			for _, row := range rows {
				if row.Result != "pass" {
					t.Errorf("members click result=%q", row.Result)
				}
			}
		})
		t.Run("members-forms", func(t *testing.T) {
			w6uiPrepareForms(t, x)
			env["LC_W6UI_FORMS_STORE"], env["LC_W6UI_FORMS_ALT"] = x.formsStore, x.formsAlt
			// Actual focused unit command: node --test --experimental-strip-types tests/admin/members-forms.test.ts.
			// Native production-built UI: tests/admin/members-forms.spec.ts; original read subtest already settled.
			w6uiSpecs(t, ctx, stack, "members-forms", env)
			data, err := os.ReadFile(filepath.Join(evidence, "members-forms-completion.json"))
			var completion struct {
				Cases []string `json:"cases"`
			}
			if err != nil || json.Unmarshal(data, &completion) != nil || len(completion.Cases) != 11 {
				t.Fatalf("M7-FORMS-REGISTERED-COMPLETION: missing eleven completed native cases: %v", err)
			}
			data, err = os.ReadFile(filepath.Join(evidence, "members-forms-click-ledger.json"))
			var rows []struct {
				Result string `json:"result"`
			}
			if err != nil || json.Unmarshal(data, &rows) != nil || len(rows) == 0 {
				t.Fatal("M7-FORMS-CLICK-LEDGER: missing native actions")
			}
			for _, row := range rows {
				if row.Result != "pass" {
					t.Error("M7-FORMS-CLICK-LEDGER: failed action")
				}
			}
			state, err := w6uiFormsState(x, "")
			if err != nil || state["members"] != int64(10) {
				t.Fatalf("M7-FORMS-SQL: member effects mismatch: %v %v", state, err)
			}
		})
	}
	x.mu.Lock()
	requests := append([]w6uiRequest(nil), x.requests...)
	x.mu.Unlock()
	// Keep exact test idempotency keys out of evidence: only method/path/status counts are needed here.
	counts := map[string]int{}
	for _, r := range requests {
		counts[fmt.Sprintf("%s %s %d", r.Method, r.Path, r.Status)]++
	}
	b, _ := json.MarshalIndent(counts, "", "  ")
	if err := os.WriteFile(filepath.Join(evidence, "go-http-counts.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	ledger, err := os.ReadFile(filepath.Join(evidence, "w6ui-click-ledger.json"))
	var rows []struct {
		Result string `json:"result"`
	}
	if err != nil || json.Unmarshal(ledger, &rows) != nil || len(rows) == 0 {
		t.Fatalf("missing nonempty click ledger: %v", err)
	}
	for _, row := range rows {
		if row.Result != "pass" {
			t.Errorf("click ledger contains %q", row.Result)
		}
	}
	t.Logf("W6 %s BROWSER (signed MOCK IdP, REAL_PG synthetic data); evidence=%s", mode, evidence)
}

func w6uiSeed(t *testing.T) *w6uiFixture {
	w := rpNewIn(t, "A", "SANDBOX")
	session, _, _ := rpSeedMain(w)
	x := &w6uiFixture{w: w, session: session, actors: map[string]string{}}
	x.principal, x.token = lcPrincipal(t, w.f, w.tenant, []string{w.store}, "store:read", "customers:read", "customers:write", "customers:privacy", "orders:read", "orders:export", "live:read")
	x.actors["owner"] = x.principal
	for role, perms := range map[string][]string{
		"reader": {"store:read", "customers:read", "orders:read"},
		"writer": {"store:read", "customers:read", "customers:write", "orders:read"},
		"none":   {"store:read"},
	} {
		id, _ := lcPrincipal(t, w.f, w.tenant, []string{w.store}, perms...)
		x.actors[role] = id
	}
	// Actual owners/import profiles make the synthetic report customer visible to the production merchant projection.
	x.customer, x.second = randomUUID(), randomUUID()
	for i, owner := range []string{x.customer, x.second} {
		mustExec(t, w.f.owner, `INSERT INTO buyer.owners(tenant_id,store_id,id) VALUES($1,$2,$3)`, w.tenant, w.store, owner)
		mustExec(t, w.f.owner, `INSERT INTO customers.import_profiles(tenant_id,store_id,owner_id,display_name,source) VALUES($1,$2,$3,$4,'shopline_csv')`, w.tenant, w.store, owner, fmt.Sprintf("W6 synthetic customer %d", i+1))
		// 0152 exposes import-only owners directly. Report-only rpWorld snapshots are deliberately incomplete for
		// merchant orders (no destination/phone/totals); attaching one poisons the actual customer Get/CAS projection.
	}
	mustExec(t, w.f.owner, `INSERT INTO customers.historical_orders(tenant_id,store_id,owner_id,external_order_id,ordered_at,status,total_minor,currency,items_summary) VALUES($1,$2,$3,'W6-ARCHIVE-ONLY',$4,'paid',999999,'TWD','W6-ARCHIVE-ONLY')`, w.tenant, w.store, x.customer, rpAt(10, 9))
	// The 21 catalogue entries exercise the real 20-checkbox cap; writes use the frozen Go/SQL path.
	for i := 1; i <= 21; i++ {
		var tag customers.TagRecord
		err := x.scope(func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
			var e error
			tag, e = customers.CreateTag(ctx, tx, s, x.token, t04Key("w6ui-seed-tag"), customers.TagInput{Name: fmt.Sprintf("Seed%02d", i), Color: "blue"})
			return e
		})
		if err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			if err = x.setTags(x.customer, []string{tag.ID}); err != nil {
				t.Fatal(err)
			}
		}
	}
	for i := 0; i < 52; i++ {
		var n customers.Note
		err := x.scope(func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
			var e error
			n, e = customers.AddNote(ctx, tx, s, x.token, t04Key("w6ui-seed-note"), x.customer, customers.NoteInput{Body: fmt.Sprintf("w6ui-seed-note-%02d", i)})
			return e
		})
		if err != nil {
			t.Fatal(err)
		}
		if i == 51 {
			x.note = n.ID
		}
	}
	// An independently denominated USD fact must never merge into the TWD oracle. Disclosed report fixture only.
	o := w.order(rpAt(10, 9), "CONFIRMED", "storefront", "card", rpLine{randomUUID(), randomUUID(), "USD", "USD-only", 1, 700})
	w.capture(o, 700, "SANDBOX", rpAt(10, 10))
	w.exec(`UPDATE checkout.orders SET currency='USD',snapshot=jsonb_set(snapshot,'{quote,currency}','"USD"') WHERE id=$1`, o.id)
	// 0061 attempt_method_amount_check: USD belongs to Stripe, while the shared reports helper seeds a synthetic TWD Stripe attempt.
	w.exec(`UPDATE checkout.payment_attempts SET method_code='stripe_checkout',currency='USD',amount_minor=700 WHERE id=$1`, o.attempt)
	w.exec(`UPDATE payments.facts SET currency='USD' WHERE attempt_id=$1`, o.attempt)
	return x
}

func (x *w6uiFixture) scope(fn func(context.Context, pgx.Tx, platform.Scope) error) error {
	ctx := context.Background()
	return platform.WithScope(ctx, x.w.f.runtime, x.token, x.w.store, "store:read", func(tx pgx.Tx, s platform.Scope) error { return fn(ctx, tx, s) })
}

func (x *w6uiFixture) setTags(owner string, ids []string) error {
	return x.scope(func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
		d, e := customers.Get(ctx, tx, s, x.token, owner)
		if e != nil {
			return e
		}
		// customers.set_owner_tags: actual scoped CAS fixture write, never a substitute for a browser Save click.
		_, e = customers.SetOwnerTags(ctx, tx, s, x.token, t04Key("w6ui-concurrent-tags"), owner, customers.SetTagsInput{TagIDs: ids, Revision: d.TagsRevision})
		return e
	})
}

type w6uiControlServer struct {
	*httptest.Server
	key string
}

func w6uiStart(t *testing.T, ctx context.Context, x *w6uiFixture, evidence string) (*brfStack, *w6uiControlServer) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	relay := newBrowserAdminRelay(t) // #116: the public origin stays owned; the child gets a private port with an EADDRINUSE relaunch
	address := relay.server.Listener.Addr().String()
	target, _ := url.Parse("http://" + address)
	// Always HTTPS, including Chromium: new leaf BFFs require Secure cookies. No auth-cookie injection.
	front := httptest.NewTLSServer(&httputil.ReverseProxy{Rewrite: func(r *httputil.ProxyRequest) {
		r.SetURL(target)
		r.Out.Host = r.In.Host
		r.Out.Header.Set("X-Forwarded-Proto", "https")
	}})
	t.Cleanup(front.Close)
	idp := newBrowserIDP(t, front.URL+"/api/auth/callback")
	mustExec(t, x.w.f.owner, `INSERT INTO identity.external_identities(issuer,subject,principal_id) VALUES($1,'browser-subject',$2)`, idp.server.URL, x.principal)
	role := "w6ui_" + strings.ReplaceAll(randomUUID(), "-", "")
	password := randomToken()
	mustExec(t, x.w.f.owner, `CREATE ROLE `+pgx.Identifier{role}.Sanitize()+` LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE IN ROLE commerce_identity PASSWORD '`+password+`'`)
	t.Cleanup(func() { mustExec(t, x.w.f.owner, `DROP ROLE `+pgx.Identifier{role}.Sanitize()) })
	u, err := url.Parse(x.w.f.databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.UserPassword(role, password)
	authority, err := platform.OpenIdentityPool(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(authority.Close)
	provider, err := oidclogin.New(ctx, oidclogin.Config{Issuer: idp.server.URL, ClientID: browserClientID, RedirectURL: idp.redirect, AllowLoopbackForTests: true})
	if err != nil {
		t.Fatal(err)
	}
	service, err := identity.New(authority, observedBrowserProvider{Provider: provider, t: t}, identity.Policy{ProviderKey: "w6ui", SessionTTL: time.Hour, OnboardingEnabled: true, Currencies: []string{"TWD", "USD"}})
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
	mux.Handle("/", httpapi.NewHandler(x.w.f.runtime, httpapi.Options{SessionStoreList: true, PaymentEnvironment: "SANDBOX"}))
	api := httptest.NewServer(w6uiWrap(x, mux))
	t.Cleanup(api.Close)
	control := &w6uiControlServer{key: randomToken()}
	control.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.Header.Get("X-W6UI-Gate-Key") != control.key {
			http.Error(w, "forbidden", 403)
			return
		}
		var in struct {
			Mode string `json:"mode"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 256)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if dec.Decode(&in) != nil || dec.Decode(&struct{}{}) != io.EOF {
			http.Error(w, "invalid", 422)
			return
		}
		var e error
		switch r.URL.Path {
		case "/actor":
			principal, ok := x.actors[in.Mode]
			if !ok {
				http.Error(w, "invalid", 422)
				return
			}
			_, e = x.w.f.owner.Exec(r.Context(), `UPDATE identity.external_identities SET principal_id=$2 WHERE issuer=$1 AND subject='browser-subject'`, idp.server.URL, principal)
		case "/fault":
			if in.Mode != "drop-note" && in.Mode != "read-503" && in.Mode != "truncate-report" && in.Mode != "tag-read-503" && in.Mode != "notes-read-503" && in.Mode != "members-search-503" && in.Mode != "members-ledger-503" && in.Mode != "members-hold" && in.Mode != "forms-hold" && in.Mode != "forms-drop" && in.Mode != "" {
				http.Error(w, "invalid", 422)
				return
			}
			x.mu.Lock()
			x.fault = in.Mode
			if in.Mode == "forms-hold" {
				x.formsHeld = make(chan struct{})
				x.formsStarted = false
			}
			if in.Mode == "members-hold" {
				if x.held != nil {
					close(x.held)
				}
				x.held = make(chan struct{})
				x.heldStarted = false
			}
			x.mu.Unlock()
		case "/forms-release":
			x.mu.Lock()
			if x.formsHeld != nil {
				close(x.formsHeld)
				x.formsHeld = nil
			}
			x.mu.Unlock()
		case "/forms-state":
			state, err := w6uiFormsState(x, in.Mode)
			if err != nil {
				http.Error(w, "forms SQL read failed", 500)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(state)
			return
		case "/forms-race":
			var balance int64
			e = x.w.f.owner.QueryRow(r.Context(), `SELECT balance FROM loyalty.members WHERE store_id=$1 AND id=$2`, x.formsStore, in.Mode).Scan(&balance)
			if e == nil && balance > 0 {
				e = platform.WithScope(r.Context(), x.w.f.runtime, x.formsToken, x.formsStore, "customers:points", func(tx pgx.Tx, sc platform.Scope) error {
					_, _, err := loyalty.Adjust(r.Context(), tx, sc, x.formsToken, t04Key("forms-race"), in.Mode, loyalty.AdjustInput{Delta: -balance, Note: "synthetic concurrent deduction"})
					return err
				})
			}
		case "/members-release":
			x.mu.Lock()
			if x.held != nil {
				close(x.held)
				x.held = nil
			}
			x.mu.Unlock()
		case "/members-held":
			x.mu.Lock()
			held := x.heldStarted
			x.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]bool{"held": held})
			return
		case "/members-revoke":
			_, e = x.w.f.owner.Exec(r.Context(), `UPDATE identity.sessions SET revoked_at=clock_timestamp() WHERE principal_id=$1`, x.actors["reader"])
		case "/deny-read":
			_, e = x.w.f.owner.Exec(r.Context(), `DELETE FROM identity.store_grants WHERE principal_id=$1 AND store_id=$2 AND permission='customers:read'`, x.actors["reader"], x.w.store)
		case "/restore-read":
			_, e = x.w.f.owner.Exec(r.Context(), `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'customers:read') ON CONFLICT DO NOTHING`, x.w.tenant, x.w.store, x.actors["reader"])
		case "/concurrent-tags":
			var id string
			e = x.w.f.owner.QueryRow(r.Context(), `SELECT id FROM customers.tags WHERE store_id=$1 AND name='Seed02'`, x.w.store).Scan(&id)
			if e == nil {
				e = x.setTags(x.customer, []string{id})
			}
		case "/concurrent-note":
			e = x.scope(func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
				var version int64
				// The owner pool supplies version only for fixture coordination; mutation runs via customers.EditNote under merchant scope.
				if e := x.w.f.owner.QueryRow(ctx, `SELECT version FROM customers.notes WHERE id=$1`, x.note).Scan(&version); e != nil {
					return e
				}
				_, e := customers.EditNote(ctx, tx, s, x.token, t04Key("w6ui-concurrent-note"), x.customer, x.note, customers.EditNoteInput{Body: "w6ui-concurrent-version", Version: version})
				return e
			})
		case "/deny-write":
			_, e = x.w.f.owner.Exec(r.Context(), `DELETE FROM identity.store_grants WHERE principal_id=$1 AND store_id=$2 AND permission='customers:write'`, x.principal, x.w.store)
		case "/restore-write":
			_, e = x.w.f.owner.Exec(r.Context(), `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'customers:write') ON CONFLICT DO NOTHING`, x.w.tenant, x.w.store, x.principal)
		case "/state":
			var unknown int
			e = x.w.f.owner.QueryRow(r.Context(), `SELECT count(*) FROM customers.notes WHERE store_id=$1 AND body IN ('w6ui-unknown-once','w6ui-revoked-unknown')`, x.w.store).Scan(&unknown)
			if e == nil {
				x.mu.Lock()
				writes := 0
				for _, q := range x.requests {
					if q.Method == "POST" && strings.HasSuffix(q.Path, "/notes") {
						writes++
					}
				}
				x.mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]int{"unknown_notes": unknown, "note_posts": writes})
				return
			}
		case "/revoke":
			_, e = x.w.f.owner.Exec(r.Context(), `UPDATE identity.sessions SET revoked_at=clock_timestamp() WHERE principal_id=$1`, x.principal)
		default:
			http.NotFound(w, r)
			return
		}
		if e != nil {
			http.Error(w, "fixture failed", 500)
			return
		}
		w.WriteHeader(204)
	}))
	t.Cleanup(control.Close)
	log := browserLog(t, filepath.Join(evidence, "next.log"))
	next := exec.CommandContext(ctx, "node", filepath.Join(root, "apps/admin/.next/standalone/apps/admin/server.js"))
	next.Dir = root
	next.Env = browserEnvironment(map[string]string{"HOSTNAME": "localhost", "NODE_OPTIONS": "--dns-result-order=ipv4first", "NODE_ENV": "production", "COMMERCE_IDENTITY_ENABLED": "1", "COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS": "1", "COMMERCE_PUBLIC_ORIGIN": front.URL, "COMMERCE_API_ORIGIN": api.URL, "COMMERCE_OIDC_ISSUER": idp.server.URL, "COMMERCE_BFF_KEY": bffKey, "COMMERCE_ONBOARDING_ENABLED": "1", "COMMERCE_ONBOARDING_CURRENCIES": "TWD,USD"})
	next.Stdout, next.Stderr = log, log
	next.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	next = relay.start(t, ctx, next)
	client := front.Client()
	client.Timeout = time.Second
	for attempt := 0; ; attempt++ {
		if response, e := client.Get(front.URL + "/api/stores"); e == nil {
			_ = response.Body.Close()
			if response.StatusCode == 401 {
				break
			}
		}
		if attempt > 100 {
			t.Fatalf("HTTPS admin readiness failed; evidence=%s", evidence)
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
	return &brfStack{root: root, evidence: evidence, origin: front.URL, api: api}, control
}

func w6uiAuthorityReadback(t *testing.T, x *w6uiFixture) {
	// Hidden/disabled UI controls do not weaken authority testing. These refused direct requests are backend evidence only.
	reader := lcToken(t, x.w.f, x.actors["reader"])
	for _, route := range []string{"reports/products.csv?from=2026-09-01&to=2026-09-30", "reports/funnel?from=2026-09-01&to=2026-09-30"} {
		r := httptest.NewRequest("GET", "/v1/admin/stores/"+x.w.store+"/"+route, nil)
		r.Header.Set("Authorization", "Bearer "+reader)
		w := httptest.NewRecorder()
		x.w.h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Errorf("reader authority %s status=%d want403", route, w.Code)
		}
	}
	r := httptest.NewRequest("POST", "/v1/admin/stores/"+x.w.store+"/customers/"+x.customer+"/notes", strings.NewReader(`{"body":"must-never-commit"}`))
	r.Header.Set("Authorization", "Bearer "+reader)
	r.Header.Set("Idempotency-Key", t04Key("w6ui-refused-note"))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	x.w.h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Errorf("read-only note write status=%d want403", w.Code)
	}
	if n := countRows(t, x.w.f.owner, `SELECT count(*) FROM customers.notes WHERE store_id=$1 AND body='must-never-commit'`, x.w.store); n != 0 {
		t.Errorf("refused backend note mutated %d rows", n)
	}
	for _, resource := range []string{"reports/products?from=2026-09-01&to=2026-09-30", "customers/" + x.customer} {
		r := httptest.NewRequest("GET", "/v1/admin/stores/"+x.w.f.storeB+"/"+resource, nil)
		r.Header.Set("Authorization", "Bearer "+reader)
		w := httptest.NewRecorder()
		x.w.h.ServeHTTP(w, r)
		if w.Code != 404 {
			t.Errorf("foreign scope %s=%d want404", resource, w.Code)
		}
	}
	owner := lcToken(t, x.w.f, x.principal)
	for _, days := range []struct {
		to     string
		status int
	}{{"2026-09-30", 200}, {"2026-10-01", 422}} {
		for _, report := range []string{"products", "channels", "funnel", "manual-orders"} {
			r := httptest.NewRequest("GET", "/v1/admin/stores/"+x.w.store+"/reports/"+report+"?from=2026-07-01&to="+days.to, nil)
			r.Header.Set("Authorization", "Bearer "+owner)
			w := httptest.NewRecorder()
			x.w.h.ServeHTTP(w, r)
			if w.Code != days.status {
				t.Errorf("inclusive range %s/%s=%d want%d", report, days.to, w.Code, days.status)
			}
		}
	}
}

func w6uiWrap(x *w6uiFixture, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		x.mu.Lock()
		formsStore := x.formsStore
		x.mu.Unlock()
		if formsStore != "" && r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/v1/admin/stores/"+formsStore+"/members") && !strings.HasSuffix(r.URL.Path, "/search") {
			w6uiFormsWrite(x, next, w, r)
			return
		}
		x.mu.Lock()
		fault := x.fault
		memberSearch := r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/members/search")
		memberDetail := r.Method == "GET" && strings.Contains(r.URL.Path, "/members/")
		memberFault := fault == "members-search-503" && memberSearch || fault == "members-ledger-503" && memberDetail && r.URL.Query().Get("ledger_cursor") != "" || fault == "members-hold" && (memberSearch || memberDetail)
		release := x.held
		if memberFault {
			x.fault = ""

		}
		match := fault == "drop-note" && r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/notes") || (fault == "read-503" || fault == "truncate-report") && r.Method == "GET" && strings.Contains(r.URL.Path, "/reports/") || fault == "tag-read-503" && r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/customers/tags") || fault == "notes-read-503" && r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/notes")
		if match {
			x.fault = ""
		}
		x.mu.Unlock()
		if memberFault && fault != "members-hold" {
			http.Error(w, "fixture transport unavailable", 503)
			return
		}
		if match && (fault == "read-503" || fault == "tag-read-503" || fault == "notes-read-503") {
			http.Error(w, "fixture transport unavailable", 503)
			return
		}
		recorded := httptest.NewRecorder()
		next.ServeHTTP(recorded, r)
		if memberFault && fault == "members-hold" && recorded.Code == http.StatusOK {
			x.mu.Lock()
			x.heldStarted = true
			x.mu.Unlock()
			timer := time.NewTimer(15 * time.Second)
			defer timer.Stop()
			select {
			case <-release:
			case <-r.Context().Done():
				return
			case <-timer.C:
				http.Error(w, "fixture hold expired", 503)
				return
			}
		}
		x.mu.Lock()
		x.requests = append(x.requests, w6uiRequest{r.Method, r.URL.Path, r.Header.Get("Idempotency-Key"), recorded.Code})
		x.mu.Unlock()
		if match && fault == "drop-note" && recorded.Code < 300 {
			// Commit already happened. Drop the actual response once; explicit UI retry must reuse its immutable command key.
			if hj, ok := w.(http.Hijacker); ok {
				if conn, _, e := hj.Hijack(); e == nil {
					_ = conn.Close()
					return
				}
			}
			panic(http.ErrAbortHandler)
		}
		for k, values := range recorded.Header() {
			w.Header()[k] = values
		}
		w.WriteHeader(recorded.Code)
		body := recorded.Body.Bytes()
		if match && fault == "truncate-report" && len(body) > 0 {
			body = body[:len(body)/2]
		}
		_, _ = w.Write(body)
	})
}

func w6uiSpecs(t *testing.T, ctx context.Context, s *brfStack, mode string, env map[string]string) {
	configName, logName := "playwright.w6.config.ts", "playwright.log"
	if mode == "members-read" {
		configName, logName = "playwright.w6.members-read.config.ts", "playwright-members-read.log"
	} else if mode == "members-forms" {
		configName, logName = "playwright.w6.members-forms.config.ts", "playwright-members-forms.log"
	}
	config := filepath.Join(s.evidence, configName)
	spec := "reports.spec.ts"
	if mode == "customers" {
		spec = "customer-tags.spec.ts"
	} else if mode == "members-read" {
		spec = "members-read.spec.ts"
	} else if mode == "members-forms" {
		spec = "members-forms.spec.ts"
	}
	body := fmt.Sprintf(`import {defineConfig} from "@playwright/test";
export default defineConfig({testDir:%q,testMatch:%q,workers:1,retries:0,timeout:120000,expect:{timeout:15000},reporter:[["list"]],outputDir:%q,use:{baseURL:%q,headless:true,ignoreHTTPSErrors:true,trace:"retain-on-failure",screenshot:"only-on-failure"}});`, filepath.Join(s.root, "tests/admin"), spec, filepath.Join(s.evidence, "results"), s.origin)
	if e := os.WriteFile(config, []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
	cmd := exec.CommandContext(ctx, "pnpm", "exec", "playwright", "test", "--config", config)
	cmd.Dir = s.root
	env["LC_BROWSER_PUBLIC_ORIGIN"], env["LC_BROWSER_EVIDENCE"] = s.origin, s.evidence
	// CI calibration truncates/drops the actual backend answer; it must fail the named UI contract assertion, not setup.
	if v := os.Getenv("LC_W6UI_CALIBRATION"); v != "" {
		env["LC_W6UI_CALIBRATION"] = v
	}
	cmd.Env = browserEnvironment(env)
	log := browserLog(t, filepath.Join(s.evidence, logName))
	cmd.Stdout, cmd.Stderr = log, log
	if e := cmd.Run(); e != nil {
		t.Fatalf("W6 browser gate failed: %v; evidence=%s", e, s.evidence)
	}
}

// w6uiSeedMembers uses scoped loyalty commands; owner SQL only supplies synthetic stores/grants/readback.
func w6uiSeedMembers(t *testing.T, x *w6uiFixture) {
	t.Helper()
	// Reports deliberately use incomplete order snapshots. Keep that corpus intact in its original store:
	// the member journey reads an entire order list before detail, so it needs native checkout orders only.
	// Reuse this world/database/router and the exact same actors; copy their existing permissions, never add any.
	reportStore := x.w.store
	x.w.store = randomUUID()
	mustExec(t, x.w.f.owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'members native store','TWD')`, x.w.tenant, x.w.store)
	actors := make([]string, 0, len(x.actors))
	for _, id := range x.actors {
		actors = append(actors, id)
	}
	mustExec(t, x.w.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission)
	 SELECT tenant_id,$2,principal_id,permission FROM identity.store_grants
	 WHERE tenant_id=$1 AND store_id=$3 AND principal_id=ANY($4::uuid[])`, x.w.tenant, x.w.store, reportStore, actors)
	x.emptyStore, x.foreignStore = randomUUID(), randomUUID()
	for _, item := range []struct{ tenant, store string }{{x.w.tenant, x.emptyStore}, {x.w.f.tenantB, x.foreignStore}} {
		mustExec(t, x.w.f.owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'members synthetic store','TWD')`, item.tenant, item.store)
	}
	for _, actor := range []string{x.actors["reader"], x.actors["none"]} {
		for _, permission := range []string{"store:read", "customers:read"} {
			if actor == x.actors["none"] && permission == "customers:read" {
				continue
			}
			mustExec(t, x.w.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, x.w.tenant, x.emptyStore, actor, permission)
		}
	}
	x.memberOrder = w6uiMemberOrder(t, x)
	var token string
	x.seedActor, token = lcPrincipal(t, x.w.f, x.w.tenant, []string{x.w.store}, lpAll...)
	run := func(fn func(context.Context, pgx.Tx, platform.Scope) error) {
		t.Helper()
		ctx := context.Background()
		if e := platform.WithScope(ctx, x.w.f.runtime, token, x.w.store, "store:read", func(tx pgx.Tx, sc platform.Scope) error { return fn(ctx, tx, sc) }); e != nil {
			t.Fatal(e)
		}
	}
	for i := 0; i < 24; i++ {
		var m loyalty.Member
		run(func(ctx context.Context, tx pgx.Tx, sc platform.Scope) (e error) {
			input := loyalty.MemberInput{DisplayName: fmt.Sprintf("LP2 synthetic %02d", i)}
			if i == 0 {
				contact := "LP2_SYNTHETIC_CONTACT"
				input.ContactNote = &contact
			}
			m, _, e = loyalty.CreateMember(ctx, tx, sc, token, t04Key("lp2-seed"), input)
			return e
		})
		if i == 0 {
			x.member = m.ID
		}
		if i == 1 {
			x.zeroMember = m.ID
		}
		n := 0
		if i == 0 {
			n = 52
		}
		if i == 23 {
			n = 1
			x.erasedMember = m.ID
		}
		for j := 0; j < n; j++ {
			run(func(ctx context.Context, tx pgx.Tx, sc platform.Scope) error {
				_, _, e := loyalty.Adjust(ctx, tx, sc, token, t04Key("lp2-seed-entry"), m.ID, loyalty.AdjustInput{Delta: 1, Note: "synthetic entry", OrderID: func() *string {
					if i == 0 && j == 51 {
						return &x.memberOrder
					}
					return nil
				}()})
				return e
			})
		}
		if i == 23 {
			run(func(ctx context.Context, tx pgx.Tx, sc platform.Scope) error {
				_, _, e := loyalty.Erase(ctx, tx, sc, token, t04Key("lp2-seed-erase"), m.ID)
				return e
			})
		}
	}
	_, foreignToken := lcPrincipal(t, x.w.f, x.w.f.tenantB, []string{x.foreignStore}, lpAll...)
	ctx := context.Background()
	if e := platform.WithScope(ctx, x.w.f.runtime, foreignToken, x.foreignStore, "store:read", func(tx pgx.Tx, sc platform.Scope) error {
		m, _, e := loyalty.CreateMember(ctx, tx, sc, foreignToken, t04Key("lp2-foreign"), loyalty.MemberInput{DisplayName: "foreign synthetic member"})
		x.foreignMember = m.ID
		return e
	}); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		x.mu.Lock()
		defer x.mu.Unlock()
		if x.held != nil {
			close(x.held)
			x.held = nil
		}
	})
}

func w6uiMemberCounts(t *testing.T, x *w6uiFixture) [3]int {
	t.Helper()
	return [3]int{
		countRows(t, x.w.f.owner, `SELECT count(*) FROM loyalty.members WHERE store_id=$1`, x.w.store),
		countRows(t, x.w.f.owner, `SELECT count(*) FROM loyalty.point_entries WHERE store_id=$1`, x.w.store),
		countRows(t, x.w.f.owner, `SELECT count(*) FROM ops.command_results WHERE store_id=$1 AND operation LIKE 'loyalty.%'`, x.w.store),
	}
}

// w6uiMemberOrder adapts moBeginInOtherStore's existing checkout leaf to this same store/pools.
// No second world, order INSERT, report snapshot or successful HTTP stub is used.
func w6uiMemberOrder(t *testing.T, x *w6uiFixture) string {
	t.Helper()
	ctx := context.Background()
	f := *x.w.f
	f.tenantA, f.storeA1 = x.w.tenant, x.w.store
	id, token := lcPrincipal(t, &f, f.tenantA, []string{f.storeA1}, "store:read", "catalog:read", "catalog:write", "inventory:read", "inventory:write", "inventory:reserve", "pricing:read", "pricing:write", "integration:manage", "integration:read", "orders:read")
	f.principalA, f.tokens = id, map[string]string{"a": token}
	h := cqHarness{f: &f, a: openBuyerTestPools(t, &f), stock: t04CreateStock(t, &f, f.tokens["a"], f.storeA1, 10)}
	var err error
	h.service, err = buyer.New(h.a.issuer, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	h.cap = mustIssue(t, h.service, f.storeA1)
	h.market, err = pricingScoped(ctx, &f, f.tokens["a"], f.storeA1, "pricing:write", func(tx pgx.Tx, scope platform.Scope) (pricing.Market, error) {
		return pricing.CreateMarket(ctx, tx, scope, t04Key("mo-other-market"), pricing.MarketInput{Code: "tw", Name: "Other store market", Currency: "TWD"})
	})
	if err != nil {
		t.Fatal(err)
	}
	zero := int64(0)
	h.policy = pricing.PolicyInput{MarketID: h.market.ID, Country: "TW", Currency: "TWD", ShippingMode: "country_flat", ShippingMinor: &zero, TaxMode: "none", TaxBasis: "goods", TaxRateBPS: &zero, QuoteTTLSeconds: 300, Enabled: true, ConfigurationRef: "synthetic merchant order fixture"}
	delivery := fulfillment.ServiceInput{MarketID: h.market.ID, Country: "TW", Code: "home", PolicyVersion: 1, NameHans: "测试配送", NameHant: "測試配送", NameEN: "Mock delivery", DeliveryKind: "home", Mode: "MANUAL", Enabled: true, Visible: true}
	dsPolicy(t, h, delivery, 0, 0, true)
	if _, err = dsSet(h, t04Key("mo-other-delivery"), delivery); err != nil {
		t.Fatal(err)
	}
	allocation := fulfillment.AllocationInput{MarketID: h.market.ID, Country: "TW", Code: "home", ExpectedServiceVersion: 1, WarehouseIDs: []string{h.stock.warehouse.ID}}
	if _, err = daSet(h, t04Key("mo-other-allocation"), allocation); err != nil {
		t.Fatal(err)
	}
	pool, err := platform.OpenCheckoutPool(ctx, bcRole(t, &f, "commerce_checkout_runtime"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	b := bcHarness{cqHarness: h, pool: pool, service: bcService(t, pool), delivery: delivery, allocation: allocation}
	b.prepare(t, h.cap, []storefront.Item{{SKUID: h.stock.skus[0].ID, Quantity: 1}})
	order, err := b.begin(t04Key("mo-other-begin"))
	if err != nil {
		t.Fatal(err)
	}
	return order.OrderID
}

// w6uiPrepareForms isolates every new grant/effect from the already-verified read corpus.
func w6uiPrepareForms(t *testing.T, x *w6uiFixture) {
	t.Helper()
	x.mu.Lock()
	x.formsStore, x.formsAlt = randomUUID(), randomUUID()
	x.mu.Unlock()
	for _, store := range []string{x.formsStore, x.formsAlt} {
		mustExec(t, x.w.f.owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'M7 synthetic forms','TWD')`, x.w.tenant, store)
	}
	x.formsPrincipal, x.formsToken = lcPrincipal(t, x.w.f, x.w.tenant, []string{x.formsStore}, "store:read", "customers:read", "customers:points")
	x.actors["forms-points"] = x.formsPrincipal
	reader, _ := lcPrincipal(t, x.w.f, x.w.tenant, []string{x.formsStore}, "store:read", "customers:read")
	x.actors["forms-reader"] = reader
	for _, permission := range []string{"store:read", "customers:read"} {
		mustExec(t, x.w.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,$4)`, x.w.tenant, x.formsAlt, x.formsPrincipal, permission)
	}
	t.Cleanup(func() {
		x.mu.Lock()
		defer x.mu.Unlock()
		if x.formsHeld != nil {
			close(x.formsHeld)
			x.formsHeld = nil
		}
	})
}

// SQL values, never response guesses, attest that writes committed before hold/drop.
func w6uiFormsState(x *w6uiFixture, member string) (map[string]int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	state := map[string]int64{}
	for _, item := range []struct{ name, sql string }{
		{"members", `SELECT count(*) FROM loyalty.members WHERE store_id=$1`},
		{"entries", `SELECT count(*) FROM loyalty.point_entries WHERE store_id=$1`},
		{"commands", `SELECT count(*) FROM ops.command_results WHERE store_id=$1 AND operation LIKE 'loyalty.%'`},
	} {
		var value int64
		if err := x.w.f.owner.QueryRow(ctx, item.sql, x.formsStore).Scan(&value); err != nil {
			return nil, err
		}
		state[item.name] = value
	}
	if member != "" {
		var balance, sum, entries int64
		err := x.w.f.owner.QueryRow(ctx, `SELECT balance,(SELECT coalesce(sum(delta),0) FROM loyalty.point_entries WHERE store_id=$1 AND member_id=$2),(SELECT count(*) FROM loyalty.point_entries WHERE store_id=$1 AND member_id=$2) FROM loyalty.members WHERE store_id=$1 AND id=$2`, x.formsStore, member).Scan(&balance, &sum, &entries)
		if err != nil {
			return nil, err
		}
		state["balance"], state["sum"], state["memberEntries"] = balance, sum, entries
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	state["held"] = 0
	if x.formsStarted {
		state["held"] = 1
	}
	for _, req := range x.requests {
		if req.Method == "POST" && strings.HasPrefix(req.Path, "/v1/admin/stores/"+x.formsStore+"/members") && !strings.HasSuffix(req.Path, "/search") {
			state["writeRequests"]++
		}
	}
	return state, nil
}

func w6uiFormsWrite(x *w6uiFixture, next http.Handler, w http.ResponseWriter, r *http.Request) {
	x.mu.Lock()
	fault, release := x.fault, x.formsHeld
	if fault == "forms-hold" || fault == "forms-drop" {
		x.fault = ""
	}
	x.mu.Unlock()
	recorded := httptest.NewRecorder()
	next.ServeHTTP(recorded, r) // Genuine production router + PG commit; no successful response stub.
	x.mu.Lock()
	x.requests = append(x.requests, w6uiRequest{r.Method, r.URL.Path, r.Header.Get("Idempotency-Key"), recorded.Code})
	x.mu.Unlock()
	if (fault == "forms-hold" || fault == "forms-drop") && recorded.Code >= 200 && recorded.Code < 300 {
		var receipt int
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := x.w.f.owner.QueryRow(ctx, `SELECT count(*) FROM ops.command_results WHERE store_id=$1 AND idempotency_key=$2`, x.formsStore, r.Header.Get("Idempotency-Key")).Scan(&receipt)
		cancel()
		if err != nil || receipt != 1 {
			http.Error(w, "M7 commit receipt SQL mismatch", 500)
			return
		}
		x.mu.Lock()
		x.formsStarted = true
		x.mu.Unlock()
		if fault == "forms-hold" {
			timer := time.NewTimer(10 * time.Second)
			defer timer.Stop()
			select {
			case <-release:
			case <-r.Context().Done():
				return
			case <-timer.C:
				http.Error(w, "M7 hold expired", 503)
				return
			}
		} else {
			if hj, ok := w.(http.Hijacker); ok {
				if conn, _, err := hj.Hijack(); err == nil {
					_ = conn.Close()
					return
				}
			}
			panic(http.ErrAbortHandler)
		}
	}
	for key, values := range recorded.Header() {
		w.Header()[key] = values
	}
	w.WriteHeader(recorded.Code)
	_, _ = w.Write(recorded.Body.Bytes())
}
