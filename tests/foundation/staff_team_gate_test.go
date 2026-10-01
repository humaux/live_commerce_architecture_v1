package foundation_test

// INDEPENDENT gate for unit staff-team (R4, migration 0089, contracts/storefront-v2.md §D). Tier REAL_PG: the real definers, the
// real commerce_identity login pool, the real admin HTTP handler (internal/httpapi) and the real staff transport (identityhttp);
// only the mailbox is a capturing fake. Written from the contract text and docs/delivery/units/staff-team.md, NOT from
// internal/identity/staff.go or the author smoke (staff_team_smoke_test.go): expectations are the §D role bundles, "one-time token,
// 72 h, single use, bound to store + role", "at least one owner always remains", "revoke is immediate", "all actions audited".
// Run: bash scripts/dev/test-focused.sh '^TestStaffGate' (never PASS when zero tests ran; SKIP is not PASS).
// Fixture mechanics: principals other than the store creator are inserted through the OWNER pool (password credential + merchant
// session), exactly because the OIDC fixture can only mint creators; every other effect goes through the product surface.
// Gates: SG01 accept refusals/single use/expiry/binding · SG02 role bundles · SG03 owner floor (service, concurrency, direct DML) ·
// SG04 revoke and role change on the next request · SG05 owner-only management and tenant scope · SG06 token hygiene (SQL text,
// args, rows, logs, URL) · SG07 audit · SG08 mail (locale, link, resend).

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/identity"
	"livecommerce/internal/identityhttp"
	"livecommerce/internal/mail"
	"livecommerce/internal/platform"
)

// ---- fakes and helpers ---------------------------------------------------------------------------------------------

type sgMail struct {
	mu   sync.Mutex
	sent []mail.Message
	fail error // when set, Send returns it and records nothing
}

func (m *sgMail) Send(_ context.Context, msg mail.Message) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail != nil {
		return "", m.fail
	}
	m.sent = append(m.sent, msg)
	return "250 ok", nil
}

var sgTokenRe = regexp.MustCompile(`/invite/([A-Za-z0-9_-]{43})`)

func (m *sgMail) to(addr string) []mail.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []mail.Message
	for _, s := range m.sent {
		if s.To == addr {
			out = append(out, s)
		}
	}
	return out
}

func (m *sgMail) token(t *testing.T, addr string) string {
	t.Helper()
	all := m.to(addr)
	if len(all) == 0 {
		t.Fatalf("no invitation mail to %s", addr)
	}
	g := sgTokenRe.FindStringSubmatch(all[len(all)-1].Text)
	if g == nil {
		t.Fatalf("no invitation link in the last mail to %s", addr)
	}
	return g[1]
}

// sgTrace records every statement and every bound argument the identity pool sends to PostgreSQL.
type sgTrace struct {
	mu   sync.Mutex
	seen []string
}

func (tr *sgTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	var b strings.Builder
	b.WriteString(d.SQL)
	for _, a := range d.Args {
		b.WriteString("\x00")
		switch v := a.(type) {
		case []byte:
			b.Write(v)
		default:
			fmt.Fprint(&b, v)
		}
	}
	tr.mu.Lock()
	tr.seen = append(tr.seen, b.String())
	tr.mu.Unlock()
	return ctx
}
func (tr *sgTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}
func (tr *sgTrace) contains(needle string) int {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	n := 0
	for _, s := range tr.seen {
		if strings.Contains(s, needle) {
			n++
		}
	}
	return n
}
func (tr *sgTrace) count() int { tr.mu.Lock(); defer tr.mu.Unlock(); return len(tr.seen) }

type sgEnv struct {
	t      *testing.T
	f      *testFixture
	ctx    context.Context
	idPool *pgxpool.Pool // commerce_identity login (the one cmd/api gives the staff service), UNtraced
	trace  *sgTrace
	svc    *identity.Staff // runs on a traced copy of idPool
	mail   *sgMail
	origin string
	owner  identity.Session
	store  identity.Store
	// emails/bearers of the people created by join() / person(), by principal id
	emails  map[string]string
	bearers map[string]string
	inviter string // bearer of a current owner; join() invites with it
}

const sgOrigin = "https://admin.example.test"

func sgSetup(t *testing.T) *sgEnv {
	t.Helper()
	s, _, pool := identityFixture(t)
	f := fixture(t)
	ctx := context.Background()
	owner := identityLogin(t, s)
	store, err := s.CreateInitialStore(ctx, owner.Token, "sg-store-"+randomUUID(), firstStoreRequest())
	if err != nil {
		t.Fatal(err)
	}
	cfg := pool.Config()
	tr := &sgTrace{}
	cfg.ConnConfig.Tracer = tr
	traced, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(traced.Close)
	m := &sgMail{}
	svc, err := identity.NewStaff(traced, m, sgOrigin)
	if err != nil {
		t.Fatal(err)
	}
	return &sgEnv{t: t, f: f, ctx: ctx, idPool: pool, trace: tr, svc: svc, mail: m, origin: sgOrigin, owner: owner, store: store, emails: map[string]string{}, bearers: map[string]string{owner.PrincipalID: owner.Token}, inviter: owner.Token}
}

func sgEmail(prefix string) string {
	return prefix + strings.ReplaceAll(randomUUID(), "-", "")[:16] + "@example.test"
}

// person inserts a principal with a verified password email and a live merchant session (owner pool; see header).
func (e *sgEnv) person(email string) (principal, bearer string) {
	e.t.Helper()
	principal, bearer = randomUUID(), randomToken()
	hash := "$argon2id$v=19$m=19456,t=2,p=1$" + strings.Repeat("A", 22) + "$" + strings.Repeat("A", 43)
	steps := []struct {
		q    string
		args []any
	}{{`INSERT INTO identity.principals(id) VALUES($1::uuid)`, []any{principal}}}
	if email != "" {
		steps = append(steps, struct {
			q    string
			args []any
		}{`INSERT INTO identity.password_credentials(principal_id,email,password_hash,email_verified_at) VALUES($1::uuid,$2,$3,now())`, []any{principal, email, hash}})
	}
	steps = append(steps, struct {
		q    string
		args []any
	}{`INSERT INTO identity.sessions(id,token_hash,principal_id,audience,expires_at) VALUES(gen_random_uuid(),sha256($1::bytea),$2::uuid,'merchant',now()+interval '1 hour')`, []any{[]byte(bearer), principal}})
	for _, s := range steps {
		if _, err := e.f.owner.Exec(e.ctx, s.q, s.args...); err != nil {
			e.t.Fatal(err)
		}
	}
	e.emails[principal], e.bearers[principal] = email, bearer
	return principal, bearer
}

// join runs the whole product path: owner invites, the person accepts. Returns principal, bearer, token.
func (e *sgEnv) join(role string) (principal, bearer, token string) {
	e.t.Helper()
	email := sgEmail("sg" + strings.ReplaceAll(role, "_", ""))
	principal, bearer = e.person(email)
	if _, err := e.svc.Invite(e.ctx, e.inviter, e.store.StoreID, email, role, "en"); err != nil {
		e.t.Fatalf("invite %s: %v", role, err)
	}
	token = e.mail.token(e.t, email)
	j, err := e.svc.Accept(e.ctx, bearer, token)
	if err != nil || j.Role != role || j.StoreID != e.store.StoreID {
		e.t.Fatalf("accept %s: %+v %v", role, j, err)
	}
	return principal, bearer, token
}

func (e *sgEnv) can(bearer, perm string) error {
	return platform.WithScope(e.ctx, e.f.runtime, bearer, e.store.StoreID, perm, func(pgx.Tx, platform.Scope) error { return nil })
}

func (e *sgEnv) grants(principal string) []string {
	var g []string
	if err := e.f.owner.QueryRow(e.ctx, `SELECT COALESCE(array_agg(permission ORDER BY permission),'{}') FROM identity.store_grants WHERE store_id=$1::uuid AND principal_id=$2::uuid`, e.store.StoreID, principal).Scan(&g); err != nil {
		e.t.Fatal(err)
	}
	return g
}

func (e *sgEnv) owners() int {
	var n int
	if err := e.f.owner.QueryRow(e.ctx, `SELECT count(*) FROM identity.store_staff WHERE store_id=$1::uuid AND role='owner'`, e.store.StoreID).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

func (e *sgEnv) role(principal string) string {
	var r string
	err := e.f.owner.QueryRow(e.ctx, `SELECT role FROM identity.store_staff WHERE store_id=$1::uuid AND principal_id=$2::uuid`, e.store.StoreID, principal).Scan(&r)
	if errors.Is(err, pgx.ErrNoRows) {
		return ""
	}
	if err != nil {
		e.t.Fatal(err)
	}
	return r
}

func sgIs(t *testing.T, got, want error, what string) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Fatalf("%s: got %v, want %v", what, got, want)
	}
}

// sgUniverse is every permission the grant CHECK admits (the "all" of §D), read from the live constraint.
func (e *sgEnv) universe() []string {
	var def string
	if err := e.f.owner.QueryRow(e.ctx, `SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid='identity.store_grants'::regclass AND conname='store_grants_permission_check'`).Scan(&def); err != nil {
		e.t.Fatal(err)
	}
	var out []string
	for _, m := range regexp.MustCompile(`'([a-z_]+:[a-z_]+)'`).FindAllStringSubmatch(def, -1) {
		out = append(out, m[1])
	}
	if len(out) < 20 {
		e.t.Fatalf("permission universe looks wrong (%d): %s", len(out), def)
	}
	sort.Strings(out)
	return out
}

// sgBundle is §D transcribed: owner all; admin all but staff management and billing (billing:manage is the only billing
// permission; staff management is not a grantable permission); live_operator live:*, catalog:read, orders:read, inventory:read;
// fulfilment orders:read, fulfillment:write, orders:export, inventory:*; viewer every :read. store:read rides along in all
// (the platform requires it to resolve any scope).
func sgBundle(role string, universe []string) map[string]bool {
	out := map[string]bool{"store:read": true}
	for _, p := range universe {
		if p == "store:read" {
			continue
		}
		switch role {
		case "owner":
			out[p] = true
		case "admin":
			out[p] = p != "billing:manage"
		case "live_operator":
			out[p] = strings.HasPrefix(p, "live:") || p == "catalog:read" || p == "orders:read" || p == "inventory:read"
		case "fulfilment":
			out[p] = p == "orders:read" || p == "fulfillment:write" || p == "orders:export" || strings.HasPrefix(p, "inventory:")
		case "viewer":
			out[p] = strings.HasSuffix(p, ":read")
		}
	}
	return out
}

func pgCode(err error) string {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		return pg.Code + "/" + pg.Message
	}
	return ""
}

// ---- SG01 accept refusals, single use, expiry, binding ---------------------------------------------------------------

func TestStaffGateSG01AcceptRefusalsSingleUseExpiryBinding(t *testing.T) {
	e := sgSetup(t)
	email := sgEmail("sg01")
	_, bearer := e.person(email)
	inv, err := e.svc.Invite(e.ctx, e.owner.Token, e.store.StoreID, strings.ToUpper(email), "fulfilment", "zh-TW")
	if err != nil || inv.MailState != "SENT" {
		t.Fatalf("invite: %+v %v", inv, err)
	}
	if d := time.Until(inv.ExpiresAt); d < 71*time.Hour || d > 73*time.Hour {
		t.Fatalf("expiry must be 72 h from now, got %v", d)
	}
	token := e.mail.token(t, email) // the address was lower-cased: mail went to the normalized address
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`).MatchString(token) {
		t.Fatalf("token is not 256-bit base64url text: %q", token)
	}

	// Every refusal is the same error value and the same HTTP answer; none of them consumes the valid token.
	bff := randomToken()
	h, err := identityhttp.NewStaffHandler(e.svc, bff)
	if err != nil {
		t.Fatal(err)
	}
	accept := func(sessionBearer, tok string) (int, map[string]any) {
		body, _ := json.Marshal(map[string]string{"token": tok})
		r := httptest.NewRequest(http.MethodPost, "/v1/identity/staff/accept", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Commerce-BFF-Key", bff)
		r.Header.Set("Authorization", "Bearer "+sessionBearer)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		var out map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		delete(out, "request_id")
		return w.Code, out
	}
	type refusal struct {
		name   string
		bearer string
		tok    string
	}
	otherEmail := sgEmail("sg01other")
	_, otherBearer := e.person(otherEmail)
	_, oidcBearer := e.person("") // principal without any password credential (OIDC-only)
	expiredEmail, revokedEmail, usedEmail := sgEmail("sg01exp"), sgEmail("sg01rev"), sgEmail("sg01used")
	_, expiredBearer := e.person(expiredEmail)
	_, revokedBearer := e.person(revokedEmail)
	_, usedBearer := e.person(usedEmail)
	expiredInv, revokedInv := sgInvite(t, e, expiredEmail), sgInvite(t, e, revokedEmail)
	expiredTok, revokedTok := e.mail.token(t, expiredEmail), e.mail.token(t, revokedEmail)
	// "Expired": move the whole creation instant back 73 h; the 72 h CHECK (expires = created + 72 h) still holds.
	if _, err := e.f.owner.Exec(e.ctx, `UPDATE identity.staff_invitations SET created_at=created_at-interval '73 hours', expires_at=expires_at-interval '73 hours' WHERE id=$1::uuid`, expiredInv); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.RevokeInvite(e.ctx, e.owner.Token, e.store.StoreID, revokedInv); err != nil {
		t.Fatal(err)
	}
	sgInvite(t, e, usedEmail)
	usedTok := e.mail.token(t, usedEmail)
	if _, err := e.svc.Accept(e.ctx, usedBearer, usedTok); err != nil {
		t.Fatalf("first use of a fresh token: %v", err)
	}
	cases := []refusal{
		{"unknown token", bearer, randomToken()},
		{"malformed token", bearer, "short"},
		{"wrong email account", otherBearer, token},
		{"oidc-only account", oidcBearer, token},
		{"expired", expiredBearer, expiredTok},
		{"revoked", revokedBearer, revokedTok},
		{"already used", usedBearer, usedTok},
		{"used token, other account", otherBearer, usedTok},
	}
	var wantStatus int
	var wantBody map[string]any
	for i, c := range cases {
		_, gotErr := e.svc.Accept(e.ctx, c.bearer, c.tok)
		if !errors.Is(gotErr, identity.ErrInviteInvalid) {
			t.Errorf("%s: %v, want ErrInviteInvalid", c.name, gotErr)
		}
		status, body := accept(c.bearer, c.tok)
		if i == 0 {
			wantStatus, wantBody = status, body
			if status != http.StatusNotFound || body["code"] != "invite_invalid" {
				t.Errorf("generic refusal must be 404 invite_invalid, got %d %v", status, body)
			}
			continue
		}
		if status != wantStatus || fmt.Sprint(body) != fmt.Sprint(wantBody) {
			t.Errorf("%s answers differently from the first refusal (account/token oracle): %d %v vs %d %v", c.name, status, body, wantStatus, wantBody)
		}
	}
	if status, _ := accept(randomToken(), token); status != http.StatusUnauthorized {
		t.Errorf("a dead session must be 401, got %d", status)
	}
	var stray int
	if err := e.f.owner.QueryRow(e.ctx, `SELECT count(*) FROM identity.store_staff WHERE store_id=$1::uuid AND principal_id IN (SELECT principal_id FROM identity.sessions WHERE token_hash = ANY($2::bytea[]))`,
		e.store.StoreID, [][]byte{sgSHA(otherBearer), sgSHA(oidcBearer), sgSHA(expiredBearer), sgSHA(revokedBearer)}).Scan(&stray); err != nil || stray != 0 {
		t.Fatalf("a refused accept created membership (%d, %v)", stray, err)
	}

	// Refusals did not burn the token: the invited account still gets in, with the role bound at invite time.
	j, err := e.svc.Accept(e.ctx, bearer, token)
	if err != nil || j.StoreID != e.store.StoreID || j.Role != "fulfilment" {
		t.Fatalf("accept after refusals: %+v %v", j, err)
	}
	if _, err := e.svc.Accept(e.ctx, bearer, token); err == nil {
		t.Fatal("a token was accepted twice")
	}
}

func sgInvite(t *testing.T, e *sgEnv, email string) string {
	t.Helper()
	inv, err := e.svc.Invite(e.ctx, e.owner.Token, e.store.StoreID, email, "viewer", "en")
	if err != nil {
		t.Fatal(err)
	}
	return inv.ID
}

func sgSHA(s string) []byte { h := sha256.Sum256([]byte(s)); return h[:] }

func TestStaffGateSG01SingleUseUnderConcurrencyAndRace(t *testing.T) {
	e := sgSetup(t)
	email := sgEmail("sg01c")
	principal, bearer := e.person(email)
	sgInvite(t, e, email)
	token := e.mail.token(t, email)
	var wg sync.WaitGroup
	var ok, invalid, member, other atomic.Int32
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := e.svc.Accept(e.ctx, bearer, token)
			switch {
			case err == nil:
				ok.Add(1)
			case errors.Is(err, identity.ErrInviteInvalid):
				invalid.Add(1)
			case errors.Is(err, identity.ErrAlreadyMember):
				member.Add(1)
			default:
				other.Add(1)
				t.Logf("unexpected: %v", err)
			}
		}()
	}
	wg.Wait()
	if ok.Load() != 1 || other.Load() != 0 {
		t.Fatalf("6 concurrent accepts of one token: ok=%d invalid=%d member=%d other=%d, want exactly one success", ok.Load(), invalid.Load(), member.Load(), other.Load())
	}
	var rows int
	if err := e.f.owner.QueryRow(e.ctx, `SELECT count(*) FROM identity.store_staff WHERE store_id=$1::uuid AND principal_id=$2::uuid`, e.store.StoreID, principal).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("membership rows %d %v", rows, err)
	}

	// Accept racing the owner's revoke: the two outcomes are mutually exclusive and the result is self-consistent.
	for round := 0; round < 4; round++ {
		em := sgEmail("sg01r")
		pr, br := e.person(em)
		id := sgInvite(t, e, em)
		tok := e.mail.token(t, em)
		var accErr, revErr error
		wg.Add(2)
		go func() { defer wg.Done(); _, accErr = e.svc.Accept(e.ctx, br, tok) }()
		go func() { defer wg.Done(); revErr = e.svc.RevokeInvite(e.ctx, e.owner.Token, e.store.StoreID, id) }()
		wg.Wait()
		member := e.role(pr) != ""
		switch {
		case accErr == nil && member && errors.Is(revErr, identity.ErrNotFound):
		case revErr == nil && !member && errors.Is(accErr, identity.ErrInviteInvalid):
		default:
			t.Fatalf("round %d inconsistent: accept=%v revoke=%v member=%v", round, accErr, revErr, member)
		}
	}
}

func TestStaffGateSG01ExpiryWindowAndTTLConstraint(t *testing.T) {
	e := sgSetup(t)
	email := sgEmail("sg01t")
	sgInvite(t, e, email)
	var created, expires time.Time
	var bound string
	if err := e.f.owner.QueryRow(e.ctx, `SELECT created_at, expires_at, role FROM identity.staff_invitations WHERE email=$1`, email).Scan(&created, &expires, &bound); err != nil {
		t.Fatal(err)
	}
	if expires.Sub(created) != 72*time.Hour || bound != "viewer" {
		t.Fatalf("invitation row: ttl=%v role=%s", expires.Sub(created), bound)
	}
	// Direct DML cannot mint a longer-lived or hash-less invitation.
	_, err := e.f.owner.Exec(e.ctx, `INSERT INTO identity.staff_invitations(id,tenant_id,store_id,email,role,locale,token_hash,invited_by,created_at,expires_at)
		VALUES(gen_random_uuid(),$1::uuid,$2::uuid,$3,'viewer','en',sha256('x'::bytea),$4::uuid,now(),now()+interval '100 hours')`, e.store.TenantID, e.store.StoreID, sgEmail("sg01long"), e.owner.PrincipalID)
	if err == nil {
		t.Fatal("a 100 h invitation was accepted by the table")
	}
	// It lists as expired once the window passed and the owner sees that.
	if _, err := e.f.owner.Exec(e.ctx, `UPDATE identity.staff_invitations SET created_at=created_at-interval '73 hours', expires_at=expires_at-interval '73 hours' WHERE email=$1`, email); err != nil {
		t.Fatal(err)
	}
	team, err := e.svc.List(e.ctx, e.owner.Token, e.store.StoreID)
	if err != nil {
		t.Fatal(err)
	}
	seen := false
	for _, i := range team.Invitations {
		if i.Email == email {
			seen = true
			if !i.Expired {
				t.Fatalf("expired invitation is not flagged expired: %+v", i)
			}
		}
	}
	if !seen {
		t.Fatal("an unaccepted, unrevoked, expired invitation is not listed")
	}
}

func TestStaffGateSG01TokenBoundToStore(t *testing.T) {
	a := sgSetup(t)
	b := sgSetup(t) // a different tenant/store with its own owner
	email := sgEmail("sg01b")
	pa, bearer := a.person(email)
	sgInvite(t, a, email)
	token := a.mail.token(t, email)
	// Store B's service does not know store A's invitation by any other path: accepting through B's service is the same
	// definers on the same database, so the grant must land in store A only.
	j, err := b.svc.Accept(b.ctx, bearer, token)
	if err != nil || j.StoreID != a.store.StoreID {
		t.Fatalf("accept: %+v %v", j, err)
	}
	if got := b.grants(pa); len(got) != 0 {
		t.Fatalf("token for store A created grants in store B: %v", got)
	}
	if len(a.grants(pa)) == 0 {
		t.Fatal("token for store A created no grants in store A")
	}
	if a.role(pa) != "viewer" || b.role(pa) != "" {
		t.Fatalf("roles: A=%q B=%q", a.role(pa), b.role(pa))
	}
}

// ---- SG02 role bundles -----------------------------------------------------------------------------------------------

func TestStaffGateSG02RoleBundlesExactlyAsContract(t *testing.T) {
	e := sgSetup(t)
	universe := e.universe()
	for _, role := range []string{"owner", "admin", "live_operator", "fulfilment", "viewer"} {
		role := role
		t.Run(role, func(t *testing.T) {
			principal, bearer, _ := e.join(role)
			want := sgBundle(role, universe)
			var allowed []string
			for _, p := range universe {
				err := e.can(bearer, p)
				if want[p] {
					if err != nil {
						t.Errorf("%s must hold %s: %v", role, p, err)
					}
					allowed = append(allowed, p)
				} else if !errors.Is(err, platform.ErrForbidden) {
					t.Errorf("%s must NOT hold %s, got %v", role, p, err)
				}
			}
			// store_grants holds exactly the bundle: no extra rows the matrix above could not see.
			got := e.grants(principal)
			wantRows := map[string]bool{}
			for p, ok := range want {
				if ok {
					wantRows[p] = true
				}
			}
			if len(got) != len(wantRows) {
				t.Errorf("%s grant rows %v, want %d permissions", role, got, len(wantRows))
			}
			for _, g := range got {
				if !wantRows[g] {
					t.Errorf("%s has unexpected grant row %s", role, g)
				}
			}
			if role == "owner" && len(allowed) != len(universe) {
				t.Errorf("owner must hold all %d permissions, holds %d", len(universe), len(allowed))
			}
		})
	}
}

func TestStaffGateSG02StoreCreatorIsAnOwnerWithEveryPermission(t *testing.T) {
	e := sgSetup(t)
	for _, p := range e.universe() {
		if err := e.can(e.owner.Token, p); err != nil {
			t.Errorf("the store creator (owner) must hold %s: %v", p, err)
		}
	}
	team, err := e.svc.List(e.ctx, e.owner.Token, e.store.StoreID)
	if err != nil || team.MyRole == nil || *team.MyRole != "owner" || len(team.Members) != 1 || !team.Members[0].IsMe || team.Members[0].Role != "owner" {
		t.Fatalf("creator's team view: %+v %v", team, err)
	}
}

func TestStaffGateSG02HTTPSurfacePerRole(t *testing.T) {
	e := sgSetup(t)
	jobs, err := river.NewClient(riverpgxv5.New(nil), &river.Config{Schema: "river_payment"})
	if err != nil {
		t.Fatal(err)
	}
	api := httpapi.NewHandler(e.f.runtime, httpapi.Options{RefundJobs: jobs})
	base := "/v1/admin/stores/" + e.store.StoreID
	call := func(bearer, method, path, body string) (code int, out string) {
		defer func() {
			if recover() != nil { // authorization passed and a downstream stub (nil billing service) panicked: reached, not denied
				code, out = 599, "handler reached"
			}
		}()
		var r *http.Request
		if body == "" {
			r = httptest.NewRequest(method, path, nil)
		} else {
			r = httptest.NewRequest(method, path, strings.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
		}
		r.Header.Set("Authorization", "Bearer "+bearer)
		if method == http.MethodPost {
			r.Header.Set("Idempotency-Key", "sg02-refund-key-0001")
		}
		w := httptest.NewRecorder()
		api.ServeHTTP(w, r)
		return w.Code, w.Body.String()
	}
	refund := base + "/orders/" + randomUUID() + "/refunds"
	refundBody := `{"amount_minor":100,"reason":"duplicate","expected_refundable_minor":100}`
	type want struct{ refund, billing, fulfil, export bool }
	matrix := map[string]want{
		"owner":         {true, true, true, true},
		"admin":         {true, false, true, true},
		"live_operator": {false, false, false, false},
		"fulfilment":    {false, false, true, true},
		"viewer":        {false, false, false, false},
	}
	for role, w := range matrix {
		_, bearer, _ := e.join(role)
		denied := func(code int) bool { return code == http.StatusForbidden || code == http.StatusUnauthorized }
		// refund POST: authority is payments:refund; a role without it is exactly 403 before any order is looked at.
		code, body := call(bearer, http.MethodPost, refund, refundBody)
		if w.refund && denied(code) {
			t.Errorf("%s refund POST must pass authorization, got %d %s", role, code, body)
		}
		if !w.refund && code != http.StatusForbidden {
			t.Errorf("%s refund POST must be exactly 403, got %d %s", role, code, body)
		}
		// billing is owner-only (admin = all but staff management and billing).
		code, body = call(bearer, http.MethodGet, base+"/billing", "")
		if !w.billing && code != http.StatusForbidden {
			t.Errorf("%s GET billing must be 403, got %d %s", role, code, body)
		}
		if w.billing && denied(code) {
			t.Errorf("%s GET billing must be reachable, got %d %s", role, code, body)
		}
		// order-actions (orders:read is in every bundle) reports the order-side bundles truthfully.
		code, body = call(bearer, http.MethodGet, base+"/order-actions", "")
		if code != http.StatusOK {
			t.Errorf("%s order-actions: %d %s", role, code, body)
			continue
		}
		var raw map[string]bool
		_ = json.Unmarshal([]byte(body), &raw)
		if raw["refund"] != w.refund || raw["fulfillment_write"] != w.fulfil || raw["orders_export"] != w.export {
			t.Errorf("%s order-actions %v, want refund=%v fulfillment_write=%v orders_export=%v", role, raw, w.refund, w.fulfil, w.export)
		}
	}
}

// ---- SG03 owner floor ------------------------------------------------------------------------------------------------

func TestStaffGateSG03OwnerFloorThroughTheProduct(t *testing.T) {
	e := sgSetup(t)
	if e.owners() != 1 {
		t.Fatalf("the store creator must be the first owner, owners=%d", e.owners())
	}
	// The sole owner can be neither demoted, removed nor lowered by invitation acceptance; nothing changes on refusal.
	sgIs(t, e.svc.SetRole(e.ctx, e.owner.Token, e.store.StoreID, e.owner.PrincipalID, "admin"), identity.ErrLastOwner, "sole owner self-demote")
	sgIs(t, e.svc.Remove(e.ctx, e.owner.Token, e.store.StoreID, e.owner.PrincipalID), identity.ErrLastOwner, "sole owner self-remove")
	if e.role(e.owner.PrincipalID) != "owner" || len(e.grants(e.owner.PrincipalID)) == 0 {
		t.Fatal("a refused owner-floor change left a partial write")
	}
	if err := e.can(e.owner.Token, "billing:manage"); err != nil {
		t.Fatalf("sole owner lost authority after refused demotion: %v", err)
	}
	// With a second owner the first may step down; the second then becomes the sole owner and is protected in turn.
	second, secondBearer, _ := e.join("owner")
	if err := e.svc.SetRole(e.ctx, e.owner.Token, e.store.StoreID, e.owner.PrincipalID, "viewer"); err != nil {
		t.Fatalf("demote with another owner present: %v", err)
	}
	sgIs(t, e.svc.SetRole(e.ctx, secondBearer, e.store.StoreID, second, "viewer"), identity.ErrLastOwner, "new sole owner self-demote")
	sgIs(t, e.svc.Remove(e.ctx, secondBearer, e.store.StoreID, second), identity.ErrLastOwner, "new sole owner self-remove")
	// The demoted ex-owner has lost staff management immediately.
	_, err := e.svc.Invite(e.ctx, e.owner.Token, e.store.StoreID, sgEmail("sg03x"), "viewer", "en")
	sgIs(t, err, identity.ErrForbidden, "demoted owner invites")
	if e.owners() != 1 {
		t.Fatalf("owners=%d", e.owners())
	}
}

// restorePair makes both people owners again after a concurrent round: the surviving owner re-promotes a demoted member or
// re-invites a removed one (every person here has a password email, so the product path works).
func (e *sgEnv) restorePair(a, b string) {
	e.t.Helper()
	pair := [2]string{a, b}
	for i, self := range pair {
		if e.role(self) != "owner" {
			continue
		}
		other := pair[1-i]
		switch e.role(other) {
		case "owner":
			return
		case "":
			em := e.emails[other]
			if _, err := e.svc.Invite(e.ctx, e.bearers[self], e.store.StoreID, em, "owner", "en"); err != nil {
				e.t.Fatalf("re-invite: %v", err)
			}
			if _, err := e.svc.Accept(e.ctx, e.bearers[other], e.mail.token(e.t, em)); err != nil {
				e.t.Fatalf("re-accept: %v", err)
			}
		default:
			if err := e.svc.SetRole(e.ctx, e.bearers[self], e.store.StoreID, other, "owner"); err != nil {
				e.t.Fatalf("re-promote: %v", err)
			}
		}
		return
	}
	e.t.Fatal("no owner left to restore the pair")
}

func TestStaffGateSG03OwnerFloorUnderConcurrency(t *testing.T) {
	e := sgSetup(t)
	st := e.store.StoreID
	// Two password-backed owners X and Y; the creator steps down so the pair is the whole owner set.
	x, _, _ := e.join("owner")
	y, _, _ := e.join("owner")
	if err := e.svc.SetRole(e.ctx, e.owner.Token, st, e.owner.PrincipalID, "viewer"); err != nil {
		t.Fatal(err)
	}
	if e.owners() != 2 {
		t.Fatalf("owners=%d", e.owners())
	}
	bx, by := e.bearers[x], e.bearers[y]
	ops := []struct {
		name   string
		fx, fy func() error
	}{
		{"mutual demotion",
			func() error { return e.svc.SetRole(e.ctx, bx, st, y, "viewer") },
			func() error { return e.svc.SetRole(e.ctx, by, st, x, "viewer") }},
		{"both demote themselves",
			func() error { return e.svc.SetRole(e.ctx, bx, st, x, "admin") },
			func() error { return e.svc.SetRole(e.ctx, by, st, y, "admin") }},
		{"both remove themselves",
			func() error { return e.svc.Remove(e.ctx, bx, st, x) },
			func() error { return e.svc.Remove(e.ctx, by, st, y) }},
		{"mutual removal",
			func() error { return e.svc.Remove(e.ctx, bx, st, y) },
			func() error { return e.svc.Remove(e.ctx, by, st, x) }},
	}
	for _, o := range ops {
		for round := 0; round < 3; round++ {
			var ex, ey error
			var wg sync.WaitGroup
			start := make(chan struct{})
			wg.Add(2)
			go func() { defer wg.Done(); <-start; ex = o.fx() }()
			go func() { defer wg.Done(); <-start; ey = o.fy() }()
			close(start)
			wg.Wait()
			if n := e.owners(); n < 1 {
				t.Fatalf("%s round %d: the store has %d owners (x=%v y=%v)", o.name, round, n, ex, ey)
			}
			if ex == nil && ey == nil {
				t.Fatalf("%s round %d: both concurrent owner-reducing calls succeeded", o.name, round)
			}
			for _, err := range []error{ex, ey} {
				if err != nil && !errors.Is(err, identity.ErrLastOwner) && !errors.Is(err, identity.ErrForbidden) && !errors.Is(err, identity.ErrUnauthorized) && !errors.Is(err, identity.ErrNotFound) {
					t.Fatalf("%s round %d: unexpected refusal %v", o.name, round, err)
				}
			}
			e.restorePair(x, y)
		}
	}
}

// dml runs statements as one transaction on the OWNER pool, optionally as another role; the error is the first failure
// (statement or COMMIT: the owner floor is a deferred check).
func (e *sgEnv) dml(asRole string, stmts ...[2]any) error {
	tx, err := e.f.owner.Begin(e.ctx)
	if err != nil {
		e.t.Fatal(err)
	}
	if asRole != "" {
		if _, err := tx.Exec(e.ctx, `SET LOCAL ROLE `+pgx.Identifier{asRole}.Sanitize()); err != nil {
			_ = tx.Rollback(e.ctx)
			e.t.Fatalf("SET ROLE %s: %v", asRole, err)
		}
	}
	for _, s := range stmts {
		if _, err := tx.Exec(e.ctx, s[0].(string), s[1].([]any)...); err != nil {
			_ = tx.Rollback(e.ctx)
			return err
		}
	}
	return tx.Commit(e.ctx)
}

func TestStaffGateSG03OwnerFloorDirectDML(t *testing.T) {
	e := sgSetup(t)
	st, o := e.store.StoreID, e.owner.PrincipalID
	del := [2]any{`DELETE FROM identity.store_staff WHERE store_id=$1::uuid AND principal_id=$2::uuid`, []any{st, o}}
	demote := [2]any{`UPDATE identity.store_staff SET role='viewer' WHERE store_id=$1::uuid AND principal_id=$2::uuid`, []any{st, o}}
	for _, role := range []string{"", "commerce_staff_writer"} {
		for name, stmt := range map[string][2]any{"delete sole owner": del, "demote sole owner": demote} {
			err := e.dml(role, stmt)
			if err == nil || !strings.Contains(pgCode(err), "last_owner") {
				t.Errorf("direct DML (%s, role %q) must be refused with last_owner at commit: %v", name, role, err)
			}
			if e.owners() != 1 {
				t.Fatalf("%s role %q orphaned the store", name, role)
			}
		}
	}
	// Two owners: deleting one is fine, deleting both in one transaction is not.
	p2, _, _ := e.join("owner")
	del2 := [2]any{`DELETE FROM identity.store_staff WHERE store_id=$1::uuid AND principal_id=$2::uuid`, []any{st, p2}}
	if err := e.dml("", del, del2); err == nil || !strings.Contains(pgCode(err), "last_owner") {
		t.Errorf("deleting every owner in one transaction must fail: %v", err)
	}
	if e.owners() != 2 {
		t.Fatalf("owners=%d after a refused double delete", e.owners())
	}
	// Swapping owners inside one transaction (deferred check) is legal in either order.
	v, _, _ := e.join("viewer")
	promote := [2]any{`UPDATE identity.store_staff SET role='owner' WHERE store_id=$1::uuid AND principal_id=$2::uuid`, []any{st, v}}
	if err := e.dml("", del2, del, promote); err != nil {
		t.Fatalf("owner swap in one transaction: %v", err)
	}
	if e.owners() != 1 || e.role(v) != "owner" {
		t.Fatalf("after swap: owners=%d role(v)=%q", e.owners(), e.role(v))
	}
	e.inviter = e.bearers[v] // the creator is no longer an owner
	// Concurrent direct deletes of DIFFERENT owner rows: with owners {X, Y}, tx1 deletes Y (leaves X) and tx2 deletes X (leaves Y).
	// Each is legal alone; both would orphan the store. Disjoint rows, so neither statement blocks the other.
	for round := 0; round < 3; round++ {
		x, _, _ := e.join("owner")
		y, _, _ := e.join("owner")
		if _, err := e.f.owner.Exec(e.ctx, `UPDATE identity.store_staff SET role='viewer' WHERE store_id=$1::uuid AND role='owner' AND principal_id NOT IN ($2::uuid,$3::uuid)`, st, x, y); err != nil {
			t.Fatal(err)
		}
		if e.owners() != 2 {
			t.Fatalf("round %d: owners=%d, want exactly X and Y", round, e.owners())
		}
		tx1, err1 := e.f.owner.Begin(e.ctx)
		tx2, err2 := e.f.owner.Begin(e.ctx)
		if err1 != nil || err2 != nil {
			t.Fatal(err1, err2)
		}
		drop := `DELETE FROM identity.store_staff WHERE store_id=$1::uuid AND principal_id=$2::uuid`
		if _, err := tx1.Exec(e.ctx, drop, st, y); err != nil {
			t.Fatal(err)
		}
		if _, err := tx2.Exec(e.ctx, drop, st, x); err != nil {
			t.Fatal(err)
		}
		var c1, c2 error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); c1 = tx1.Commit(e.ctx) }()
		go func() { defer wg.Done(); c2 = tx2.Commit(e.ctx) }()
		wg.Wait()
		if n := e.owners(); n < 1 {
			t.Fatalf("round %d: concurrent direct deletes orphaned the store (owners=%d c1=%v c2=%v)", round, n, c1, c2)
		}
		if c1 == nil && c2 == nil {
			t.Fatalf("round %d: both conflicting direct deletes committed", round)
		}
		e.inviter = e.bearers[x]
		if e.role(x) != "owner" {
			e.inviter = e.bearers[y]
		}
	}
}

func TestStaffGateSG03AppLoginsCannotWriteStaffTablesDirectly(t *testing.T) {
	e := sgSetup(t)
	_, _, _ = e.join("viewer")
	st, me := e.store.StoreID, e.owner.PrincipalID
	stmts := map[string]string{
		"insert store_staff":        fmt.Sprintf(`INSERT INTO identity.store_staff(tenant_id,store_id,principal_id,role) VALUES('%s','%s','%s','owner')`, e.store.TenantID, st, randomUUID()),
		"update store_staff":        fmt.Sprintf(`UPDATE identity.store_staff SET role='owner' WHERE store_id='%s'`, st),
		"delete store_staff":        fmt.Sprintf(`DELETE FROM identity.store_staff WHERE store_id='%s' AND principal_id='%s'`, st, me),
		"insert store_grants":       fmt.Sprintf(`INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES('%s','%s','%s','billing:manage')`, e.store.TenantID, st, randomUUID()),
		"delete store_grants":       fmt.Sprintf(`DELETE FROM identity.store_grants WHERE store_id='%s'`, st),
		"update invitations":        `UPDATE identity.staff_invitations SET expires_at=expires_at+interval '30 days'`,
		"delete invitations":        `DELETE FROM identity.staff_invitations`,
		"read invitation hashes":    `SELECT token_hash FROM identity.staff_invitations`,
		"call staff_apply_role":     fmt.Sprintf(`SELECT identity.staff_apply_role('%s','%s','%s','owner')`, e.store.TenantID, st, me),
		"call staff_owner_ctx":      `SELECT * FROM identity.staff_owner_ctx(sha256('x'::bytea),gen_random_uuid())`,
		"call staff_role_bundle fn": `SELECT identity.staff_role_permissions('owner')`,
	}
	for name, q := range stmts {
		for poolName, pool := range map[string]*pgxpool.Pool{"identity": e.idPool, "runtime": e.f.runtime} {
			_, err := pool.Exec(e.ctx, q)
			var pg *pgconn.PgError
			if err == nil || !errors.As(err, &pg) || pg.Code != "42501" {
				t.Errorf("%s as the %s login must be permission denied (42501), got %v", name, poolName, err)
			}
		}
	}
}

// ---- SG04 revoke / role change take effect on the next request --------------------------------------------------------

func TestStaffGateSG04RoleChangeAndRevokeAreImmediate(t *testing.T) {
	e := sgSetup(t)
	p, bearer, _ := e.join("admin")
	for _, perm := range []string{"payments:refund", "orders:read", "catalog:write"} {
		if err := e.can(bearer, perm); err != nil {
			t.Fatalf("admin %s: %v", perm, err)
		}
	}
	// Role change: the SAME session, the very next request, has the new bundle.
	if err := e.svc.SetRole(e.ctx, e.owner.Token, e.store.StoreID, p, "viewer"); err != nil {
		t.Fatal(err)
	}
	if err := e.can(bearer, "orders:read"); err != nil {
		t.Fatalf("viewer lost read: %v", err)
	}
	for _, perm := range []string{"payments:refund", "catalog:write", "fulfillment:write"} {
		sgIs(t, e.can(bearer, perm), platform.ErrForbidden, "viewer "+perm)
	}
	if got := e.role(p); got != "viewer" {
		t.Fatalf("role=%s", got)
	}
	// Upgrade works the same way.
	if err := e.svc.SetRole(e.ctx, e.owner.Token, e.store.StoreID, p, "fulfilment"); err != nil {
		t.Fatal(err)
	}
	if err := e.can(bearer, "fulfillment:write"); err != nil {
		t.Fatalf("fulfilment after upgrade: %v", err)
	}
	sgIs(t, e.can(bearer, "audit:read"), platform.ErrForbidden, "fulfilment audit:read after role change")

	// Revoke: the next request is refused, whatever the permission, and the member can no longer manage or accept.
	if err := e.svc.Remove(e.ctx, e.owner.Token, e.store.StoreID, p); err != nil {
		t.Fatal(err)
	}
	for _, perm := range []string{"store:read", "orders:read", "fulfillment:write"} {
		if err := e.can(bearer, perm); err == nil {
			t.Fatalf("removed member still authorized for %s", perm)
		}
	}
	if len(e.grants(p)) != 0 || e.role(p) != "" {
		t.Fatalf("revoked member keeps grants %v / role %q", e.grants(p), e.role(p))
	}
	if _, err := e.svc.List(e.ctx, bearer, e.store.StoreID); err == nil {
		t.Fatal("removed member can still read the team")
	}
	var active bool
	if err := e.f.owner.QueryRow(e.ctx, `SELECT active FROM identity.memberships WHERE tenant_id=$1::uuid AND principal_id=$2::uuid`, e.store.TenantID, p).Scan(&active); err == nil && active {
		t.Fatal("membership of a member with no grants left is still active")
	}
	// Re-inviting works and the new bundle is the new role's, not a leftover of the old one.
	var em string
	if err := e.f.owner.QueryRow(e.ctx, `SELECT email FROM identity.password_credentials WHERE principal_id=$1::uuid`, p).Scan(&em); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Invite(e.ctx, e.owner.Token, e.store.StoreID, em, "live_operator", "en"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Accept(e.ctx, bearer, e.mail.token(t, em)); err != nil {
		t.Fatalf("re-accept after revoke: %v", err)
	}
	if err := e.can(bearer, "live:manage"); err != nil {
		t.Fatalf("live_operator after rejoin: %v", err)
	}
	sgIs(t, e.can(bearer, "fulfillment:write"), platform.ErrForbidden, "leftover fulfilment grant after rejoin as live_operator")
}

func TestStaffGateSG04NoRequestSucceedsAfterRevokeReturns(t *testing.T) {
	e := sgSetup(t)
	p, bearer, _ := e.join("viewer")
	var removed atomic.Bool
	var stop atomic.Bool
	var violations atomic.Int32
	var calls atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				after := removed.Load() // read BEFORE the request starts
				err := e.can(bearer, "orders:read")
				calls.Add(1)
				if after && err == nil {
					violations.Add(1)
				}
			}
		}()
	}
	time.Sleep(150 * time.Millisecond)
	if err := e.svc.Remove(e.ctx, e.owner.Token, e.store.StoreID, p); err != nil {
		t.Fatal(err)
	}
	removed.Store(true)
	time.Sleep(300 * time.Millisecond)
	stop.Store(true)
	wg.Wait()
	if calls.Load() < 20 {
		t.Fatalf("only %d requests ran; the test would be vacuous", calls.Load())
	}
	if violations.Load() != 0 {
		t.Fatalf("%d request(s) that started after Remove returned were still authorized", violations.Load())
	}
}

// ---- SG05 owner-only management and tenant scope ----------------------------------------------------------------------

func TestStaffGateSG05OnlyOwnerManagesTheTeam(t *testing.T) {
	e := sgSetup(t)
	victim, _, _ := e.join("viewer")
	inv := sgInvite(t, e, sgEmail("sg05pending"))
	for _, role := range []string{"admin", "live_operator", "fulfilment", "viewer"} {
		p, bearer, _ := e.join(role)
		team, err := e.svc.List(e.ctx, bearer, e.store.StoreID)
		if err != nil || team.MyRole == nil || *team.MyRole != role || len(team.Members) != 0 || len(team.Invitations) != 0 {
			t.Errorf("%s list must show only its own role: %+v %v", role, team, err)
		}
		_, err = e.svc.Invite(e.ctx, bearer, e.store.StoreID, sgEmail("sg05x"), "viewer", "en")
		sgIs(t, err, identity.ErrForbidden, role+" invite")
		_, err = e.svc.Invite(e.ctx, bearer, e.store.StoreID, sgEmail("sg05y"), "owner", "en")
		sgIs(t, err, identity.ErrForbidden, role+" invite owner")
		sgIs(t, e.svc.SetRole(e.ctx, bearer, e.store.StoreID, p, "owner"), identity.ErrForbidden, role+" self-promote")
		sgIs(t, e.svc.SetRole(e.ctx, bearer, e.store.StoreID, victim, "viewer"), identity.ErrForbidden, role+" set role")
		sgIs(t, e.svc.Remove(e.ctx, bearer, e.store.StoreID, victim), identity.ErrForbidden, role+" remove member")
		sgIs(t, e.svc.Remove(e.ctx, bearer, e.store.StoreID, e.owner.PrincipalID), identity.ErrForbidden, role+" remove owner")
		sgIs(t, e.svc.RevokeInvite(e.ctx, bearer, e.store.StoreID, inv), identity.ErrForbidden, role+" revoke invite")
		if e.role(p) != role {
			t.Errorf("%s role changed to %s", role, e.role(p))
		}
	}
	if e.role(victim) != "viewer" || e.role(e.owner.PrincipalID) != "owner" {
		t.Fatal("a non-owner changed the team")
	}
	// admin is "all but staff management and billing": explicit.
	_, adminBearer, _ := e.join("admin")
	sgIs(t, e.can(adminBearer, "billing:manage"), platform.ErrForbidden, "admin billing")
	if err := e.can(adminBearer, "ads:approve"); err != nil {
		t.Fatalf("admin ads:approve: %v", err)
	}
}

func TestStaffGateSG05TenantAndStoreScopeIsServerSide(t *testing.T) {
	a := sgSetup(t)
	b := sgSetup(t)
	pa, _, _ := a.join("viewer")
	invA := sgInvite(t, a, sgEmail("sg05a"))
	// B's owner names store A (or A's ids) explicitly: refused, nothing changes.
	_, err := b.svc.List(b.ctx, b.owner.Token, a.store.StoreID)
	if err == nil {
		t.Error("an owner of another tenant listed this team")
	}
	_, err = b.svc.Invite(b.ctx, b.owner.Token, a.store.StoreID, sgEmail("sg05evil"), "owner", "en")
	if err == nil {
		t.Error("an owner of another tenant invited into this store")
	}
	if err := b.svc.SetRole(b.ctx, b.owner.Token, a.store.StoreID, pa, "owner"); err == nil {
		t.Error("an owner of another tenant changed a role here")
	}
	if err := b.svc.Remove(b.ctx, b.owner.Token, a.store.StoreID, pa); err == nil {
		t.Error("an owner of another tenant removed a member here")
	}
	// Own store in the path, foreign ids in the body: not found, never cross-tenant.
	sgIs(t, b.svc.RevokeInvite(b.ctx, b.owner.Token, b.store.StoreID, invA), identity.ErrNotFound, "foreign invite id")
	sgIs(t, b.svc.SetRole(b.ctx, b.owner.Token, b.store.StoreID, pa, "owner"), identity.ErrNotFound, "foreign principal")
	sgIs(t, b.svc.Remove(b.ctx, b.owner.Token, b.store.StoreID, pa), identity.ErrNotFound, "foreign principal remove")
	if a.role(pa) != "viewer" || a.owners() != 1 {
		t.Fatal("cross-tenant call changed store A")
	}
	if team, err := a.svc.List(a.ctx, a.owner.Token, a.store.StoreID); err != nil || len(team.Invitations) != 1 {
		t.Fatalf("store A invitations changed: %+v %v", team, err)
	}
	// Inviting an address that already has an account elsewhere is indistinguishable from inviting a stranger (no account oracle).
	elsewhere := sgEmail("sg05known")
	b.person(elsewhere)
	stranger := sgEmail("sg05unknown")
	known, kerr := a.svc.Invite(a.ctx, a.owner.Token, a.store.StoreID, elsewhere, "viewer", "en")
	unknown, uerr := a.svc.Invite(a.ctx, a.owner.Token, a.store.StoreID, stranger, "viewer", "en")
	if kerr != nil || uerr != nil || known.MailState != unknown.MailState {
		t.Errorf("invite oracle: known=%+v %v, unknown=%+v %v", known, kerr, unknown, uerr)
	}
	// Unknown store, malformed ids and a dead session.
	_, err = a.svc.List(a.ctx, a.owner.Token, randomUUID())
	if err == nil {
		t.Error("unknown store listed")
	}
	_, err = a.svc.List(a.ctx, randomToken(), a.store.StoreID)
	sgIs(t, err, identity.ErrUnauthorized, "dead session")
	_, err = a.svc.Invite(a.ctx, a.owner.Token, "not-a-uuid", sgEmail("sg05z"), "viewer", "en")
	sgIs(t, err, identity.ErrInvalid, "bad store id")
}

// ---- SG06 token hygiene --------------------------------------------------------------------------------------------------

type sgSyncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *sgSyncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}
func (s *sgSyncBuf) String() string { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }

func TestStaffGateSG06TokenNeverInSQLRowsLogsOrURLs(t *testing.T) {
	logs := &sgSyncBuf{}
	prevSlog := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	log.SetOutput(logs)
	t.Cleanup(func() { slog.SetDefault(prevSlog); log.SetOutput(os.Stderr) })

	e := sgSetup(t)
	bff := randomToken()
	h, err := identityhttp.NewStaffHandler(e.svc, bff)
	if err != nil {
		t.Fatal(err)
	}
	var tokens []string
	var urls []string
	post := func(path, bearer string, body any, query string) (int, string) {
		raw, _ := json.Marshal(body)
		u := "/v1/identity/staff/" + path + query
		urls = append(urls, u)
		r := httptest.NewRequest(http.MethodPost, u, bytes.NewReader(raw))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Commerce-BFF-Key", bff)
		r.Header.Set("Authorization", "Bearer "+bearer)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		b, _ := io.ReadAll(w.Body)
		return w.Code, string(b)
	}
	email := sgEmail("sg06")
	_, bearer := e.person(email)
	code, body := post("invite", e.owner.Token, map[string]string{"store_id": e.store.StoreID, "email": email, "role": "viewer", "locale": "en"}, "")
	if code != http.StatusCreated {
		t.Fatalf("invite over HTTP: %d %s", code, body)
	}
	token := e.mail.token(t, email)
	tokens = append(tokens, token)
	// The token in the URL query of an API call is refused outright and does not accept anything.
	if code, body := post("accept", bearer, map[string]string{"token": token}, "?token="+token); code < 400 || strings.Contains(body, token) {
		t.Errorf("accept with a token in the query string must be refused without echoing it: %d %s", code, body)
	}
	if e.role(e.owner.PrincipalID) == "" {
		t.Fatal("setup")
	}
	if n := e.f.owner.QueryRow(e.ctx, `SELECT 1 FROM identity.store_staff s JOIN identity.sessions x ON x.principal_id=s.principal_id WHERE s.store_id=$1::uuid AND x.token_hash=sha256($2::bytea)`, e.store.StoreID, []byte(bearer)); n != nil {
		var one int
		if err := n.Scan(&one); err == nil {
			t.Fatal("the query-string accept created a membership")
		}
	}
	// Wrong-account, unknown and revoked attempts, then the real accept in the body.
	_, otherBearer := e.person(sgEmail("sg06o"))
	post("accept", otherBearer, map[string]string{"token": token}, "")
	other := randomToken()
	tokens = append(tokens, other)
	post("accept", bearer, map[string]string{"token": other}, "")
	if code, body := post("accept", bearer, map[string]string{"token": token}, ""); code != http.StatusOK || strings.Contains(body, token) {
		t.Fatalf("accept: %d %s", code, body)
	}
	// Second invitation: revoked, and resent.
	email2 := sgEmail("sg06b")
	e.person(email2)
	i2, _ := e.svc.Invite(e.ctx, e.owner.Token, e.store.StoreID, email2, "viewer", "zh-CN")
	tokens = append(tokens, e.mail.token(t, email2))
	_ = e.svc.RevokeInvite(e.ctx, e.owner.Token, e.store.StoreID, i2.ID)
	e.svc.Invite(e.ctx, e.owner.Token, e.store.StoreID, email2, "admin", "zh-TW")
	tokens = append(tokens, e.mail.token(t, email2))
	e.svc.List(e.ctx, e.owner.Token, e.store.StoreID)

	if e.trace.count() < 10 {
		t.Fatalf("only %d statements traced; the check would be vacuous", e.trace.count())
	}
	for _, tok := range tokens {
		if n := e.trace.contains(tok); n != 0 {
			t.Errorf("a plaintext token reached SQL text or bound arguments in %d statement(s)", n)
		}
		if strings.Contains(logs.String(), tok) {
			t.Error("a plaintext token is in the log output")
		}
		for _, u := range urls {
			if strings.Contains(strings.ReplaceAll(u, "?token="+token, ""), tok) {
				t.Error("a plaintext token is in an API URL")
			}
		}
		for _, q := range []string{
			`SELECT count(*) FROM identity.staff_invitations i WHERE i::text LIKE '%'||$1||'%'`,
			`SELECT count(*) FROM identity.store_staff i WHERE i::text LIKE '%'||$1||'%'`,
			`SELECT count(*) FROM identity.store_grants i WHERE i::text LIKE '%'||$1||'%'`,
			`SELECT count(*) FROM ops.audit_events i WHERE i::text LIKE '%'||$1||'%'`,
			`SELECT count(*) FROM identity.session_events i WHERE i::text LIKE '%'||$1||'%'`,
		} {
			var n int
			if err := e.f.owner.QueryRow(e.ctx, q, tok).Scan(&n); err != nil || n != 0 {
				t.Errorf("a plaintext token is stored: %s (%d, %v)", q, n, err)
			}
		}
	}
	// The only copy is the mail, and the link has no query string or fragment: path only.
	for _, m := range e.mail.sent {
		g := regexp.MustCompile(`https?://[^\s"<>]+/invite/[A-Za-z0-9_-]{43}[^\s"<>]*`).FindAllString(m.Text, -1)
		if len(g) != 1 {
			t.Errorf("mail to %s: want exactly one invitation link, got %v", m.To, g)
			continue
		}
		if !strings.HasPrefix(g[0], sgOrigin+"/") || strings.ContainsAny(g[0], "?#") || len(g[0]) != len(sgOrigin)+len("/en/invite/")+43 && len(g[0]) != len(sgOrigin)+len("/zh-TW/invite/")+43 && len(g[0]) != len(sgOrigin)+len("/zh-CN/invite/")+43 {
			t.Errorf("invitation link must be <public origin>/<locale>/invite/<token> with no query: %s", g[0])
		}
	}
}

// ---- SG07 audit --------------------------------------------------------------------------------------------------------

func TestStaffGateSG07EveryActionIsAudited(t *testing.T) {
	e := sgSetup(t)
	count := func(action string) int {
		var n int
		if err := e.f.owner.QueryRow(e.ctx, `SELECT count(*) FROM ops.audit_events WHERE store_id=$1::uuid AND action=$2`, e.store.StoreID, action).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	em := sgEmail("sg07")
	p, bearer := e.person(em)
	i1, err := e.svc.Invite(e.ctx, e.owner.Token, e.store.StoreID, em, "live_operator", "en")
	if err != nil {
		t.Fatal(err)
	}
	if count("staff.invited:live_operator") != 1 {
		t.Error("invite is not audited as staff.invited:<role>")
	}
	if err := e.svc.RevokeInvite(e.ctx, e.owner.Token, e.store.StoreID, i1.ID); err != nil {
		t.Fatal(err)
	}
	if count("staff.invite_revoked") != 1 {
		t.Error("revoke of an invitation is not audited")
	}
	if _, err := e.svc.Invite(e.ctx, e.owner.Token, e.store.StoreID, em, "live_operator", "en"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Accept(e.ctx, bearer, e.mail.token(t, em)); err != nil {
		t.Fatal(err)
	}
	if count("staff.accepted:live_operator") != 1 {
		t.Error("accept is not audited as staff.accepted:<role>")
	}
	if err := e.svc.SetRole(e.ctx, e.owner.Token, e.store.StoreID, p, "viewer"); err != nil {
		t.Fatal(err)
	}
	if count("staff.role_changed:viewer") != 1 {
		t.Error("role change is not audited as staff.role_changed:<new role>")
	}
	if err := e.svc.Remove(e.ctx, e.owner.Token, e.store.StoreID, p); err != nil {
		t.Fatal(err)
	}
	if count("staff.removed") != 1 {
		t.Error("removal is not audited")
	}
	// The actor is recorded: the owner for owner actions, the invitee for its own accept.
	var ownerActs, inviteeActs int
	_ = e.f.owner.QueryRow(e.ctx, `SELECT count(*) FROM ops.audit_events WHERE store_id=$1::uuid AND action LIKE 'staff.%' AND principal_id=$2::uuid`, e.store.StoreID, e.owner.PrincipalID).Scan(&ownerActs)
	_ = e.f.owner.QueryRow(e.ctx, `SELECT count(*) FROM ops.audit_events WHERE store_id=$1::uuid AND action LIKE 'staff.accepted:%' AND principal_id=$2::uuid`, e.store.StoreID, p).Scan(&inviteeActs)
	if ownerActs != 5 || inviteeActs != 1 {
		t.Errorf("actors: owner=%d (want 5: invited x2, invite revoked, role changed, removed) invitee-accepts=%d (want 1)", ownerActs, inviteeActs)
	}
	// A refused action leaves no audit row and no half-written state.
	before := count("staff.removed")
	sgIs(t, e.svc.Remove(e.ctx, e.owner.Token, e.store.StoreID, e.owner.PrincipalID), identity.ErrLastOwner, "last owner")
	if count("staff.removed") != before {
		t.Error("a refused removal was audited as done")
	}
	var total int
	_ = e.f.owner.QueryRow(e.ctx, `SELECT count(*) FROM ops.audit_events WHERE store_id=$1::uuid AND action LIKE 'staff.%'`, e.store.StoreID).Scan(&total)
	if total != 6 {
		t.Errorf("expected exactly 6 staff audit rows (2 invites, revoke, accept, role change, removal), got %d", total)
	}
}

// ---- SG08 mail ---------------------------------------------------------------------------------------------------------

func TestStaffGateSG08MailLocaleLinkAndResend(t *testing.T) {
	e := sgSetup(t)
	want := map[string]string{"zh-TW": "邀請", "zh-CN": "邀请", "en": "invite"}
	var first string
	for _, loc := range []string{"zh-TW", "zh-CN", "en"} {
		em := sgEmail("sg08" + strings.ToLower(strings.ReplaceAll(loc, "-", "")))
		inv, err := e.svc.Invite(e.ctx, e.owner.Token, e.store.StoreID, strings.ToUpper(em), "viewer", loc)
		if err != nil || inv.MailState != "SENT" {
			t.Fatalf("%s: %+v %v", loc, inv, err)
		}
		msgs := e.mail.to(em)
		if len(msgs) != 1 {
			t.Fatalf("%s: want exactly one mail to the normalized address, got %d", loc, len(msgs))
		}
		if !strings.Contains(strings.ToLower(msgs[0].Subject+msgs[0].Text), want[loc]) {
			t.Errorf("%s mail is not in the inviter's locale: %q", loc, msgs[0].Subject)
		}
		if !strings.Contains(msgs[0].Text, e.origin+"/"+loc+"/invite/") || !strings.Contains(msgs[0].HTML, e.origin+"/"+loc+"/invite/") {
			t.Errorf("%s mail link must be %s/%s/invite/<token> in both bodies", loc, e.origin, loc)
		}
		if !strings.Contains(msgs[0].Text, "72") {
			t.Errorf("%s mail must tell the 72 h validity", loc)
		}
		if first == "" {
			first = em
		}
	}
	// Resend = a new token; the old one dies (single live invitation per address).
	old := e.mail.token(t, first)
	_, bearer := e.person(first)
	if _, err := e.svc.Invite(e.ctx, e.owner.Token, e.store.StoreID, first, "admin", "en"); err != nil {
		t.Fatal(err)
	}
	fresh := e.mail.token(t, first)
	if fresh == old {
		t.Fatal("a resend reused the old token")
	}
	_, err := e.svc.Accept(e.ctx, bearer, old)
	sgIs(t, err, identity.ErrInviteInvalid, "superseded token")
	j, err := e.svc.Accept(e.ctx, bearer, fresh)
	if err != nil || j.Role != "admin" {
		t.Fatalf("resent invitation: %+v %v", j, err)
	}
	// Input validation: unknown role, bad e-mail, unsupported locale, already a member.
	_, err = e.svc.Invite(e.ctx, e.owner.Token, e.store.StoreID, sgEmail("sg08r"), "superuser", "en")
	sgIs(t, err, identity.ErrInvalid, "unknown role")
	_, err = e.svc.Invite(e.ctx, e.owner.Token, e.store.StoreID, "not-an-email", "viewer", "en")
	sgIs(t, err, identity.ErrInvalidEmail, "bad email")
	_, err = e.svc.Invite(e.ctx, e.owner.Token, e.store.StoreID, sgEmail("sg08l"), "viewer", "fr")
	sgIs(t, err, identity.ErrInvalid, "unsupported locale")
	_, err = e.svc.Invite(e.ctx, e.owner.Token, e.store.StoreID, first, "viewer", "en")
	sgIs(t, err, identity.ErrAlreadyMember, "invite an existing member")
}

func TestStaffGateSG08MailFailureIsRecordedNotHidden(t *testing.T) {
	e := sgSetup(t)
	for state, cause := range map[string]error{"FAILED": errors.New("smtp refused"), "UNKNOWN": mail.ErrUnknown} {
		e.mail.mu.Lock()
		e.mail.fail = cause
		e.mail.mu.Unlock()
		em := sgEmail("sg08f" + strings.ToLower(state))
		inv, err := e.svc.Invite(e.ctx, e.owner.Token, e.store.StoreID, em, "viewer", "en")
		if err != nil || inv.MailState != state {
			t.Fatalf("%s: %+v %v (the invitation exists, the mail outcome is reported, not an error)", state, inv, err)
		}
		team, err := e.svc.List(e.ctx, e.owner.Token, e.store.StoreID)
		if err != nil {
			t.Fatal(err)
		}
		shown := ""
		for _, i := range team.Invitations {
			if i.Email == em {
				shown = i.MailState
			}
		}
		if shown != state {
			t.Errorf("the owner sees mail_state %q for %s, want %q", shown, em, state)
		}
	}
	e.mail.mu.Lock()
	e.mail.fail = nil
	e.mail.mu.Unlock()
}

func TestStaffGateSG08ConcurrentInvitesOfOneAddressLeaveOneLiveToken(t *testing.T) {
	e := sgSetup(t)
	em := sgEmail("sg08c")
	_, bearer := e.person(em)
	var wg sync.WaitGroup
	var failed atomic.Int32
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := e.svc.Invite(e.ctx, e.owner.Token, e.store.StoreID, em, "viewer", "en"); err != nil {
				failed.Add(1)
				t.Logf("invite: %v", err)
			}
		}()
	}
	wg.Wait()
	if failed.Load() != 0 {
		t.Fatalf("%d concurrent invites of one address failed", failed.Load())
	}
	var live int
	if err := e.f.owner.QueryRow(e.ctx, `SELECT count(*) FROM identity.staff_invitations WHERE email=$1 AND accepted_at IS NULL AND revoked_at IS NULL`, em).Scan(&live); err != nil || live != 1 {
		t.Fatalf("live invitations for one address: %d %v", live, err)
	}
	// Of the five emailed tokens exactly one is acceptable.
	good := 0
	for _, m := range e.mail.to(em) {
		g := sgTokenRe.FindStringSubmatch(m.Text)
		if g == nil {
			t.Fatal("mail without link")
		}
		if j, err := e.svc.Accept(e.ctx, bearer, g[1]); err == nil && j.StoreID == e.store.StoreID {
			good++
		}
	}
	if len(e.mail.to(em)) != 5 || good != 1 {
		t.Fatalf("mails=%d acceptable tokens=%d, want 5 mails and exactly 1 acceptable token", len(e.mail.to(em)), good)
	}
}
