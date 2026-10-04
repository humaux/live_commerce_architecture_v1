package foundation_test

// Author smoke for migration 0089 / internal/identity/staff.go (R4 unit staff-team). Tier REAL_PG: the real definers, the
// real identity pool, a fake Mailer. It is NOT the independent gate (the tester writes that from the contract); it proves the
// unit's own claims once: owner floor in SQL, hashed single-use token, email binding with one generic refusal, 72 h expiry,
// role bundles, revoke effective on the next request, resend supersedes, per-store invitation ceiling.
// Principals other than the store creator are inserted through the owner pool (a password credential + merchant session),
// because the OIDC fixture only mints creators; each use says so.

import (
	"context"
	"errors"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/identity"
	"livecommerce/internal/mail"
	"livecommerce/internal/platform"
)

type stfMailer struct {
	mu   sync.Mutex
	sent []mail.Message
	err  error
}

func (m *stfMailer) Send(_ context.Context, msg mail.Message) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return "", m.err
	}
	m.sent = append(m.sent, msg)
	return "250 ok", nil
}

var stfTokenRe = regexp.MustCompile(`/invite/([A-Za-z0-9_-]{43})`)

// last returns the token of the most recent mail to `to`.
func (m *stfMailer) token(t *testing.T, to string) string {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := len(m.sent) - 1; i >= 0; i-- {
		if m.sent[i].To == to {
			if g := stfTokenRe.FindStringSubmatch(m.sent[i].Text); g != nil {
				return g[1]
			}
		}
	}
	t.Fatalf("no invitation mail to %s", to)
	return ""
}

type stfEnv struct {
	t       *testing.T
	f       *testFixture
	svc     *identity.Staff
	mailer  *stfMailer
	owner   identity.Session
	store   identity.Store
	ctx     context.Context
	staffer string
}

func stfSetup(t *testing.T) *stfEnv {
	t.Helper()
	s, _, pool := identityFixture(t)
	f := fixture(t)
	ctx := context.Background()
	owner := identityLogin(t, s)
	store, err := s.CreateInitialStore(ctx, owner.Token, "staff-store-"+randomUUID(), firstStoreRequest())
	if err != nil {
		t.Fatal(err)
	}
	m := &stfMailer{}
	svc, err := identity.NewStaff(pool, m, "https://admin.example.test")
	if err != nil {
		t.Fatal(err)
	}
	return &stfEnv{t: t, f: f, svc: svc, mailer: m, owner: owner, store: store, ctx: ctx}
}

// member inserts a principal with a verified password email and a live merchant session through the OWNER pool (the
// production path is password signup; this only avoids driving SMTP code flows in a smoke test). Returns principal, bearer.
func (e *stfEnv) member(email string) (string, string) {
	e.t.Helper()
	principal, bearer := randomUUID(), randomToken()
	hash := "$argon2id$v=19$m=19456,t=2,p=1$" + strings.Repeat("A", 22) + "$" + strings.Repeat("A", 43)
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO identity.principals(id) VALUES($1::uuid)`, []any{principal}},
		{`INSERT INTO identity.password_credentials(principal_id,email,password_hash,email_verified_at) VALUES($1::uuid,$2,$3,now())`, []any{principal, email, hash}},
		{`INSERT INTO identity.sessions(id,token_hash,principal_id,audience,expires_at) VALUES(gen_random_uuid(),sha256($1::bytea),$2::uuid,'merchant',now()+interval '1 hour')`, []any{[]byte(bearer), principal}},
	} {
		if _, err := e.f.owner.Exec(e.ctx, q.sql, q.args...); err != nil {
			e.t.Fatal(err)
		}
	}
	return principal, bearer
}

func (e *stfEnv) grants(principal string) []string {
	var g []string
	if err := e.f.owner.QueryRow(e.ctx, `SELECT COALESCE(array_agg(permission ORDER BY permission),'{}') FROM identity.store_grants WHERE store_id=$1::uuid AND principal_id=$2::uuid`, e.store.StoreID, principal).Scan(&g); err != nil {
		e.t.Fatal(err)
	}
	return g
}

func (e *stfEnv) bundle(role string) []string {
	var g []string
	if err := e.f.owner.QueryRow(e.ctx, `SELECT array(SELECT x FROM unnest(identity.staff_role_permissions($1)) x ORDER BY x)`, role).Scan(&g); err != nil {
		e.t.Fatal(err)
	}
	return g
}

func (e *stfEnv) access(bearer, perm string) error {
	return platform.WithScope(e.ctx, e.f.runtime, bearer, e.store.StoreID, perm, func(pgx.Tx, platform.Scope) error { return nil })
}

func stfNeed(t *testing.T, got, want error, what string) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Fatalf("%s: got %v, want %v", what, got, want)
	}
}

func TestStaffTeamSmokeInviteAcceptRevoke(t *testing.T) {
	e := stfSetup(t)
	var ownerID string
	if err := e.f.owner.QueryRow(e.ctx, `SELECT principal_id::text FROM identity.store_staff WHERE store_id=$1::uuid AND role='owner'`, e.store.StoreID).Scan(&ownerID); err != nil || ownerID != e.owner.PrincipalID {
		t.Fatalf("creator is not the first owner (trigger): %q %v", ownerID, err)
	}
	team, err := e.svc.List(e.ctx, e.owner.Token, e.store.StoreID)
	if err != nil || team.MyRole == nil || *team.MyRole != "owner" || len(team.Members) != 1 || len(team.Invitations) != 0 {
		t.Fatalf("owner list: %+v %v", team, err)
	}

	email := "invitee" + strings.ReplaceAll(randomUUID(), "-", "") + "@example.test"
	inv, err := e.svc.Invite(e.ctx, e.owner.Token, e.store.StoreID, strings.ToUpper(email), "viewer", "zh-TW")
	if err != nil || inv.MailState != "SENT" || time.Until(inv.ExpiresAt) < 71*time.Hour || time.Until(inv.ExpiresAt) > 73*time.Hour {
		t.Fatalf("invite: %+v %v", inv, err)
	}
	token := e.mailer.token(t, email) // the address was normalized to lower case before it was bound
	var plain int
	if err := e.f.owner.QueryRow(e.ctx, `SELECT count(*) FROM identity.staff_invitations i WHERE i::text LIKE '%'||$1||'%'`, token).Scan(&plain); err != nil || plain != 0 {
		t.Fatalf("plaintext token found in the invitation row (%d, %v)", plain, err)
	}
	if !strings.Contains(e.mailer.sent[0].Subject, "邀請") {
		t.Fatalf("mail is not in the inviter's locale: %q", e.mailer.sent[0].Subject)
	}

	// Binding: another signed-in account, an unknown token and a malformed token are refused with the SAME error.
	_, otherBearer := e.member("other" + strings.ReplaceAll(randomUUID(), "-", "") + "@example.test")
	_, err = e.svc.Accept(e.ctx, otherBearer, token)
	stfNeed(t, err, identity.ErrInviteInvalid, "wrong email")
	_, err = e.svc.Accept(e.ctx, otherBearer, randomToken())
	stfNeed(t, err, identity.ErrInviteInvalid, "unknown token")
	_, err = e.svc.Accept(e.ctx, otherBearer, "short")
	stfNeed(t, err, identity.ErrInviteInvalid, "malformed token")
	_, err = e.svc.Accept(e.ctx, randomToken(), token)
	stfNeed(t, err, identity.ErrUnauthorized, "no session")

	invitee, bearer := e.member(email)
	joined, err := e.svc.Accept(e.ctx, bearer, token)
	if err != nil || joined.StoreID != e.store.StoreID || joined.Role != "viewer" {
		t.Fatalf("accept: %+v %v", joined, err)
	}
	if got, want := e.grants(invitee), e.bundle("viewer"); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("viewer grants\n got %v\nwant %v", got, want)
	}
	if err := e.access(bearer, "orders:read"); err != nil {
		t.Fatalf("viewer cannot read orders: %v", err)
	}
	if err := e.access(bearer, "catalog:write"); !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("viewer can write the catalog: %v", err)
	}
	_, err = e.svc.Accept(e.ctx, bearer, token)
	stfNeed(t, err, identity.ErrInviteInvalid, "single use")

	// Non-owners cannot manage and see only their own role.
	mine, err := e.svc.List(e.ctx, bearer, e.store.StoreID)
	if err != nil || mine.MyRole == nil || *mine.MyRole != "viewer" || len(mine.Members) != 0 || len(mine.Invitations) != 0 {
		t.Fatalf("viewer list: %+v %v", mine, err)
	}
	_, err = e.svc.Invite(e.ctx, bearer, e.store.StoreID, "x@example.test", "viewer", "en")
	stfNeed(t, err, identity.ErrForbidden, "viewer invites")
	stfNeed(t, e.svc.SetRole(e.ctx, bearer, e.store.StoreID, invitee, "owner"), identity.ErrForbidden, "viewer self-promotes")
	stfNeed(t, e.svc.Remove(e.ctx, bearer, e.store.StoreID, ownerID), identity.ErrForbidden, "viewer removes owner")
	_, err = e.svc.Invite(e.ctx, e.owner.Token, e.store.StoreID, email, "admin", "en")
	stfNeed(t, err, identity.ErrAlreadyMember, "invite a member")

	// Role change replaces the bundle: admin has everything except billing.
	if err := e.svc.SetRole(e.ctx, e.owner.Token, e.store.StoreID, invitee, "admin"); err != nil {
		t.Fatal(err)
	}
	g := strings.Join(e.grants(invitee), ",")
	if g != strings.Join(e.bundle("admin"), ",") || strings.Contains(g, "billing:manage") || !strings.Contains(g, "payments:refund") {
		t.Fatalf("admin grants: %s", g)
	}
	if err := e.access(bearer, "billing:manage"); !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("admin reaches billing: %v", err)
	}

	// Revoke: the very next request is refused (membership inactive, grants gone); audit rows exist.
	if err := e.svc.Remove(e.ctx, e.owner.Token, e.store.StoreID, invitee); err != nil {
		t.Fatal(err)
	}
	if err := e.access(bearer, "store:read"); !errors.Is(err, platform.ErrScopeNotFound) {
		t.Fatalf("revoked member still authorizes: %v", err)
	}
	if n := len(e.grants(invitee)); n != 0 {
		t.Fatalf("grants left after revoke: %d", n)
	}
	var actions []string
	if err := e.f.owner.QueryRow(e.ctx, `SELECT array_agg(action ORDER BY created_at) FROM ops.audit_events WHERE store_id=$1::uuid AND action LIKE 'staff.%'`, e.store.StoreID).Scan(&actions); err != nil {
		t.Fatal(err)
	}
	if strings.Join(actions, ",") != "staff.invited:viewer,staff.accepted:viewer,staff.role_changed:admin,staff.removed" {
		t.Fatalf("audit trail: %v", actions)
	}
	// A revoked member can be invited again and gets a working bundle back.
	if _, err := e.svc.Invite(e.ctx, e.owner.Token, e.store.StoreID, email, "live_operator", "en"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Accept(e.ctx, bearer, e.mailer.token(t, email)); err != nil {
		t.Fatal(err)
	}
	if err := e.access(bearer, "live:manage"); err != nil {
		t.Fatalf("re-invited live operator: %v", err)
	}
	if err := e.access(bearer, "orders:export"); !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("live operator can export orders: %v", err)
	}
}

func TestStaffTeamSmokeExpiryResendRevokeInviteAndLimits(t *testing.T) {
	e := stfSetup(t)
	email := "late" + strings.ReplaceAll(randomUUID(), "-", "") + "@example.test"
	_, bearer := e.member(email)
	inv, err := e.svc.Invite(e.ctx, e.owner.Token, e.store.StoreID, email, "fulfilment", "en")
	if err != nil {
		t.Fatal(err)
	}
	first := e.mailer.token(t, email)
	// Resend: a NEW invitation revokes the open one for the same email; only one open row remains.
	if _, err := e.svc.Invite(e.ctx, e.owner.Token, e.store.StoreID, email, "fulfilment", "en"); err != nil {
		t.Fatal(err)
	}
	_, err = e.svc.Accept(e.ctx, bearer, first)
	stfNeed(t, err, identity.ErrInviteInvalid, "superseded token")
	second := e.mailer.token(t, email)
	team, _ := e.svc.List(e.ctx, e.owner.Token, e.store.StoreID)
	if len(team.Invitations) != 1 || team.Invitations[0].ID == inv.ID {
		t.Fatalf("open invitations after resend: %+v", team.Invitations)
	}
	// Age it past 72 h (test-only owner-pool UPDATE; the CHECK keeps expires_at = created_at + 72 h).
	if _, err := e.f.owner.Exec(e.ctx, `UPDATE identity.staff_invitations SET created_at=created_at-interval '4 days', expires_at=expires_at-interval '4 days' WHERE store_id=$1::uuid AND revoked_at IS NULL`, e.store.StoreID); err != nil {
		t.Fatal(err)
	}
	_, err = e.svc.Accept(e.ctx, bearer, second)
	stfNeed(t, err, identity.ErrInviteInvalid, "expired token")
	team, _ = e.svc.List(e.ctx, e.owner.Token, e.store.StoreID)
	if len(team.Invitations) != 1 || !team.Invitations[0].Expired {
		t.Fatalf("expired invitation not flagged: %+v", team.Invitations)
	}
	// Owner withdraws; unknown id is not_found.
	if err := e.svc.RevokeInvite(e.ctx, e.owner.Token, e.store.StoreID, team.Invitations[0].ID); err != nil {
		t.Fatal(err)
	}
	stfNeed(t, e.svc.RevokeInvite(e.ctx, e.owner.Token, e.store.StoreID, randomUUID()), identity.ErrNotFound, "unknown invitation")

	// Mail failure: the invitation exists and says FAILED; UNKNOWN is distinguished and never retried.
	e.mailer.err = mail.ErrFailed
	failed, err := e.svc.Invite(e.ctx, e.owner.Token, e.store.StoreID, "fail"+strings.ReplaceAll(randomUUID(), "-", "")+"@example.test", "viewer", "en")
	if err != nil || failed.MailState != "FAILED" {
		t.Fatalf("failed mail: %+v %v", failed, err)
	}
	e.mailer.err = mail.ErrUnknown
	unknown, err := e.svc.Invite(e.ctx, e.owner.Token, e.store.StoreID, "unk"+strings.ReplaceAll(randomUUID(), "-", "")+"@example.test", "viewer", "en")
	if err != nil || unknown.MailState != "UNKNOWN" {
		t.Fatalf("unknown mail: %+v %v", unknown, err)
	}
	e.mailer.err = nil
	var states []string
	if err := e.f.owner.QueryRow(e.ctx, `SELECT array_agg(mail_state ORDER BY mail_state) FROM identity.staff_invitations WHERE id IN ($1::uuid,$2::uuid)`, failed.ID, unknown.ID).Scan(&states); err != nil || strings.Join(states, ",") != "FAILED,UNKNOWN" {
		t.Fatalf("recorded mail states: %v %v", states, err)
	}

	// Ceiling: 20 live invitations per store (2 are live above), the 19th distinct address over the cap is refused.
	var refused error
	for i := 0; i < 25 && refused == nil; i++ {
		_, refused = e.svc.Invite(e.ctx, e.owner.Token, e.store.StoreID, "bulk"+strings.ReplaceAll(randomUUID(), "-", "")+"@example.test", "viewer", "en")
	}
	stfNeed(t, refused, identity.ErrTooManyInvitations, "invitation ceiling")
}

func TestStaffTeamSmokeOwnerFloor(t *testing.T) {
	e := stfSetup(t)
	stfNeed(t, e.svc.SetRole(e.ctx, e.owner.Token, e.store.StoreID, e.owner.PrincipalID, "admin"), identity.ErrLastOwner, "demote the last owner")
	stfNeed(t, e.svc.Remove(e.ctx, e.owner.Token, e.store.StoreID, e.owner.PrincipalID), identity.ErrLastOwner, "remove the last owner")

	// Direct SQL cannot orphan the store either: the deferred constraint trigger fails the COMMIT (owner pool, test-only).
	for name, stmt := range map[string]string{
		"delete": `DELETE FROM identity.store_staff WHERE store_id=$1::uuid`,
		"demote": `UPDATE identity.store_staff SET role='viewer' WHERE store_id=$1::uuid`,
	} {
		tx, err := e.f.owner.Begin(e.ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(e.ctx, stmt, e.store.StoreID); err != nil {
			t.Fatalf("%s: statement itself should pass (deferred): %v", name, err)
		}
		if err := tx.Commit(e.ctx); err == nil || !strings.Contains(err.Error(), "last_owner") {
			t.Fatalf("%s: commit must fail with last_owner, got %v", name, err)
		}
	}

	// Two owners demote each other at the same instant: exactly one change may win, one owner must remain.
	email := "second" + strings.ReplaceAll(randomUUID(), "-", "") + "@example.test"
	second, bearer := e.member(email)
	if _, err := e.svc.Invite(e.ctx, e.owner.Token, e.store.StoreID, email, "owner", "en"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Accept(e.ctx, bearer, e.mailer.token(t, email)); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		errs[0] = e.svc.SetRole(e.ctx, e.owner.Token, e.store.StoreID, second, "viewer")
	}()
	go func() {
		defer wg.Done()
		errs[1] = e.svc.SetRole(e.ctx, bearer, e.store.StoreID, e.owner.PrincipalID, "viewer")
	}()
	wg.Wait()
	var owners int
	if err := e.f.owner.QueryRow(e.ctx, `SELECT count(*) FROM identity.store_staff WHERE store_id=$1::uuid AND role='owner'`, e.store.StoreID).Scan(&owners); err != nil || owners != 1 {
		t.Fatalf("owners after the race: %d (%v) errs=%v", owners, err, errs)
	}
	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
		}
	}
	if ok != 1 {
		t.Fatalf("exactly one demotion may succeed: %v", errs)
	}
}

func TestStaffTeamSmokeBundlesAreSubsetOfOwnerAndConstraint(t *testing.T) {
	e := stfSetup(t)
	owner := e.bundle("owner")
	set := map[string]bool{}
	for _, p := range owner {
		set[p] = true
	}
	for _, role := range []string{"admin", "live_operator", "fulfilment", "viewer"} {
		b := e.bundle(role)
		if !sort.StringsAreSorted(b) || !set["store:read"] || !contains(b, "store:read") {
			t.Fatalf("%s lacks store:read", role)
		}
		for _, p := range b {
			if !set[p] {
				t.Errorf("%s has %s which owner lacks", role, p)
			}
		}
	}
	var def string
	if err := e.f.owner.QueryRow(e.ctx, `SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid='identity.store_grants'::regclass AND conname='store_grants_permission_check'`).Scan(&def); err != nil {
		t.Fatal(err)
	}
	for _, p := range owner {
		if !strings.Contains(def, "'"+p+"'") {
			t.Errorf("owner bundle names %s which the permission CHECK rejects", p)
		}
	}
	// Contract §D spot checks.
	if contains(e.bundle("admin"), "billing:manage") || !contains(owner, "billing:manage") {
		t.Error("billing:manage belongs to owner only")
	}
	if got := strings.Join(e.bundle("fulfilment"), ","); !strings.Contains(got, "fulfillment:write") || !strings.Contains(got, "orders:export") || strings.Contains(got, "live:manage") {
		t.Errorf("fulfilment bundle: %s", got)
	}
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
