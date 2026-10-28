// Purpose: OC-03 re-authentication SQL guards (contracts/merchant-password-auth-v1.md §16, Red tests): every guarded
// business definer, called directly (no HTTP) with a session whose authenticated_at is 2 hours old, raises PT403
// reauth_required; a fresh session passes the guard; the owner role cannot be created (staff_invite, staff_set_role,
// accepting an old owner invitation: PT403 owner_role_disabled); EXECUTE on identity.require_recent_auth is held by the
// owners of the guarded definers and nobody else; plus the two OC-02 follow-ups of issue #215 (an OIDC re-auth never moves
// authenticated_at backwards; a future or infinite authenticated_at is not recent).
// Depends on: reauth_test.go (same package: disposable lc_foundation_test database, fixtures and helpers), migrations 0089
// (staff), 0137 (platform Stripe), 0079 (billing), 0074 (ads), 0078 (privacy), 0147 (reports), post_river/0016 (refund), 0203.
// Used by: go test ./tests/identity (CI shard:unit via scripts/dev/test-local.sh).
package reauthtests

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/identity"
)

// guardedSigs are the definers that must call identity.require_recent_auth (output/oc-03/DEFINERS.md).
var guardedSigs = []string{
	"identity.staff_invite(bytea,uuid,uuid,text,text,text,bytea)",
	"identity.staff_set_role(bytea,uuid,uuid,text)",
	"identity.staff_remove(bytea,uuid,uuid)",
	"payments.set_platform_stripe(bytea,uuid,text,boolean,text,text,bigint)",
	"payments.request_stripe_refund(bytea,uuid,uuid,text,bytea,bigint,text,bigint,uuid,bigint)",
	"payments.decide_bank_transfer(bytea,uuid,uuid,text,bytea,text,text)",
	"identity.export_unshipped_orders(bytea,uuid,integer)",
	"identity.export_finance_summary(bytea,uuid,date,date)",
	"identity.export_report(bytea,uuid,text,date,date,uuid,text)",
	"customers.record_export(bytea,uuid,uuid,text,uuid,jsonb)",
	"customers.export_import_profile(bytea,uuid,uuid)",
	"customers.erase_owner(bytea,uuid,uuid,text,uuid)",
	"billing.pin_customer(bytea,uuid,text,text,text)",
	"billing.record_checkout_session(bytea,uuid,text,text,timestamptz)",
	"ads.approve_draft(bytea,uuid,uuid,integer)",
	"ads.publish_draft(bytea,uuid,uuid,integer,uuid,bigint)",
}

// world is one store with its creator (an owner holding every permission), two live sessions of that owner (authenticated
// 2 hours ago / just now) and a second member for set_role and remove.
type world struct {
	e                    *env
	tenant, store, owner string
	stale, fresh         string
	member               string
}

func guardEmail(prefix string) string {
	return fmt.Sprintf("%s.%d.%d@example.test", prefix, os.Getpid(), seq.Add(1))
}

func newWorld(t *testing.T, e *env) *world {
	t.Helper()
	w := &world{e: e, owner: principal(t, e, "", "")}
	boot := session(t, e, w.owner)
	if err := e.id.QueryRow(context.Background(),
		`SELECT tenant_id::text, store_id::text FROM identity.create_initial_store($1,$2,$3,'OC03 tenant','OC03 store','OC03 warehouse','TWD')`,
		hash(boot), "oc03-"+uuid(), rnd32()).Scan(&w.tenant, &w.store); err != nil {
		t.Fatalf("fixture store: %v", err)
	}
	w.stale, w.fresh = session(t, e, w.owner), session(t, e, w.owner)
	setAuthAge(t, e, w.stale, "2 hours")
	setAuthAge(t, e, w.fresh, "0 seconds")
	w.member = principal(t, e, "", "")
	mustExec(t, e.owner, `INSERT INTO identity.memberships(tenant_id,principal_id) VALUES ($1::uuid,$2::uuid)`, w.tenant, w.member)
	mustExec(t, e.owner, `INSERT INTO identity.store_staff(tenant_id,store_id,principal_id,role) VALUES ($1::uuid,$2::uuid,$3::uuid,'viewer')`, w.tenant, w.store, w.member)
	return w
}

// call runs one definer the way a request does: one transaction carrying the scope GUCs the Go layer pins, rolled back
// afterwards so every case starts from the same state. The caller is the migration owner, so only the definer's own
// checks decide the outcome.
func (w *world) call(t *testing.T, sql string, args ...any) error {
	t.Helper()
	ctx := context.Background()
	tx, err := w.e.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id',$1,true),set_config('app.store_id',$2,true),set_config('app.principal_id',$3,true)`,
		w.tenant, w.store, w.owner); err != nil {
		t.Fatalf("scope GUCs: %v", err)
	}
	_, err = tx.Exec(ctx, sql, args...)
	return err
}

// invitation inserts a live invitation of the given role (the shape staff_invite writes) and returns its token hash.
func (w *world) invitation(t *testing.T, email, role string) []byte {
	t.Helper()
	th := rnd32()
	mustExec(t, w.e.owner, `WITH c AS (SELECT clock_timestamp() AS t)
		INSERT INTO identity.staff_invitations(id,tenant_id,store_id,email,role,locale,token_hash,invited_by,created_at,expires_at)
		SELECT $1::uuid,$2::uuid,$3::uuid,$4,$5,'en',$6,$7::uuid,c.t,c.t+interval '72 hours' FROM c`,
		uuid(), w.tenant, w.store, email, role, th, w.owner)
	return th
}

// invitee is a principal with a verified password email and a live session: the shape staff_accept needs.
func (w *world) invitee(t *testing.T, email string) string {
	t.Helper()
	phc, err := identity.HashPassword("correct horse battery 1")
	if err != nil {
		t.Fatal(err)
	}
	q := principal(t, w.e, "", "")
	mustExec(t, w.e.owner, `INSERT INTO identity.password_credentials(principal_id,email,password_hash,email_verified_at) VALUES ($1::uuid,$2,$3,clock_timestamp())`, q, email, phc)
	return session(t, w.e, q)
}

func wantPG(t *testing.T, what string, err error, code, msg string) {
	t.Helper()
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != code || pg.Message != msg {
		t.Errorf("%s: err = %v, want %s %s", what, err, code, msg)
	}
}

func isReauthRequired(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "PT403" && pg.Message == "reauth_required"
}

// gcase is one guarded definer call. args receives the bearer to authenticate with. mustSucceed marks the cases whose
// fixture is complete enough for a fresh session to succeed outright; the others only have to get past the guard.
type gcase struct {
	name        string
	sql         string
	args        func(w *world, tok string) []any
	mustSucceed bool
}

func guardCases() []gcase {
	day := time.Now().UTC()
	from := day.AddDate(0, 0, -7)
	return []gcase{
		{"staff_invite", `SELECT * FROM identity.staff_invite($1,$2::uuid,$3::uuid,$4,'viewer','en',$5)`,
			func(w *world, tok string) []any {
				return []any{hash(tok), w.store, uuid(), guardEmail("oc03.invite"), rnd32()}
			}, true},
		{"staff_set_role", `SELECT identity.staff_set_role($1,$2::uuid,$3::uuid,'admin')`,
			func(w *world, tok string) []any { return []any{hash(tok), w.store, w.member} }, true},
		{"staff_remove", `SELECT identity.staff_remove($1,$2::uuid,$3::uuid)`,
			func(w *world, tok string) []any { return []any{hash(tok), w.store, w.member} }, true},
		{"set_platform_stripe", `SELECT payments.set_platform_stripe($1,$2::uuid,'SANDBOX',false,NULL,NULL,0::bigint)`,
			func(w *world, tok string) []any { return []any{hash(tok), w.store} }, false},
		{"request_stripe_refund", `SELECT payments.request_stripe_refund($1,$2::uuid,$3::uuid,'oc03-refund-key',$4,100::bigint,'requested_by_customer',100::bigint,$5::uuid,1::bigint)`,
			func(w *world, tok string) []any { return []any{hash(tok), w.store, uuid(), rnd32(), uuid()} }, false},
		{"decide_bank_transfer", `SELECT payments.decide_bank_transfer($1,$2::uuid,$3::uuid,'oc03-decide-key',$4,'confirm',NULL)`,
			func(w *world, tok string) []any { return []any{hash(tok), w.store, uuid(), rnd32()} }, false},
		{"decide_bank_transfer_refund_offline", `SELECT payments.decide_bank_transfer($1,$2::uuid,$3::uuid,'oc03-offline-key',$4,'refund_offline',NULL)`,
			func(w *world, tok string) []any { return []any{hash(tok), w.store, uuid(), rnd32()} }, false},
		{"export_unshipped_orders", `SELECT identity.export_unshipped_orders($1,$2::uuid,10)`,
			func(w *world, tok string) []any { return []any{hash(tok), w.store} }, true},
		{"export_finance_summary", `SELECT identity.export_finance_summary($1,$2::uuid,$3::date,$4::date)`,
			func(w *world, tok string) []any { return []any{hash(tok), w.store, from, day} }, false},
		{"export_report", `SELECT identity.export_report($1,$2::uuid,'products',$3::date,$4::date,NULL::uuid,'SANDBOX')`,
			func(w *world, tok string) []any { return []any{hash(tok), w.store, from, day} }, false},
		{"record_export_merchant", `SELECT customers.record_export($1,$2::uuid,$3::uuid,'merchant',$4::uuid,'{}'::jsonb)`,
			func(w *world, tok string) []any { return []any{hash(tok), w.store, uuid(), uuid()} }, false},
		{"export_import_profile", `SELECT customers.export_import_profile($1,$2::uuid,$3::uuid)`,
			func(w *world, tok string) []any { return []any{hash(tok), w.store, uuid()} }, false},
		{"erase_owner_merchant", `SELECT customers.erase_owner($1,$2::uuid,$3::uuid,'merchant',$4::uuid)`,
			func(w *world, tok string) []any { return []any{hash(tok), w.store, uuid(), uuid()} }, false},
		{"pin_customer", `SELECT billing.pin_customer($1,$2::uuid,'SANDBOX','cus_oc03abc','acct_oc03abc')`,
			func(w *world, tok string) []any { return []any{hash(tok), w.store} }, false},
		{"record_checkout_session", `SELECT billing.record_checkout_session($1,$2::uuid,'SANDBOX','cs_oc03abc',clock_timestamp()+interval '10 minutes')`,
			func(w *world, tok string) []any { return []any{hash(tok), w.store} }, false},
		{"approve_draft", `SELECT ads.approve_draft($1,$2::uuid,$3::uuid,1)`,
			func(w *world, tok string) []any { return []any{hash(tok), w.store, uuid()} }, false},
		{"publish_draft", `SELECT ads.publish_draft($1,$2::uuid,$3::uuid,0,$4::uuid,1::bigint)`,
			func(w *world, tok string) []any { return []any{hash(tok), w.store, uuid(), uuid()} }, false},
	}
}

// OG01: a session authenticated 2 hours ago is refused by every guarded definer, called directly.
func TestGuardOG01StaleSessionRefusedByEveryDefiner(t *testing.T) {
	w := newWorld(t, fixture(t))
	for _, c := range guardCases() {
		t.Run(c.name, func(t *testing.T) {
			wantReauthRequired(t, c.name, w.call(t, c.sql, c.args(w, w.stale)...))
		})
	}
	// The refusal is a 403-class answer about freshness, not a session failure: the stale session is still live.
	if n := count(t, w.e.owner, `SELECT count(*) FROM identity.sessions WHERE token_hash=$1 AND revoked_at IS NULL AND expires_at>clock_timestamp()`, hash(w.stale)); n != 1 {
		t.Errorf("the stale session must stay valid after a refusal, live rows = %d", n)
	}
}

// OG02: a fresh session gets past the guard in every definer; the staff trio and the unshipped-orders export (whose
// fixtures are complete) succeed outright. The remaining families need a paid order, a draft or a customer; their
// success paths are covered by the existing foundation suites, which run on fresh sessions.
func TestGuardOG02FreshSessionPassesTheGuard(t *testing.T) {
	w := newWorld(t, fixture(t))
	for _, c := range guardCases() {
		t.Run(c.name, func(t *testing.T) {
			err := w.call(t, c.sql, c.args(w, w.fresh)...)
			if isReauthRequired(err) {
				t.Fatalf("%s: a fresh session was refused: %v", c.name, err)
			}
			if c.mustSucceed && err != nil {
				t.Errorf("%s: fresh session: %v, want success", c.name, err)
			}
		})
	}
}

// OG03: the owner role cannot be created (invite, set_role) and an owner invitation issued before 0203 cannot be accepted;
// ordinary invitations still work.
func TestGuardOG03OwnerRoleDisabled(t *testing.T) {
	w := newWorld(t, fixture(t))
	wantPG(t, "staff_invite(owner)", w.call(t, `SELECT * FROM identity.staff_invite($1,$2::uuid,$3::uuid,$4,'owner','en',$5)`,
		hash(w.fresh), w.store, uuid(), guardEmail("oc03.owner"), rnd32()), "PT403", "owner_role_disabled")
	wantPG(t, "staff_set_role(owner)", w.call(t, `SELECT identity.staff_set_role($1,$2::uuid,$3::uuid,'owner')`,
		hash(w.fresh), w.store, w.member), "PT403", "owner_role_disabled")

	ownerEmail, viewerEmail := guardEmail("oc03.acc.owner"), guardEmail("oc03.acc.viewer")
	ownerTok, viewerTok := w.invitee(t, ownerEmail), w.invitee(t, viewerEmail)
	ownerInvite, viewerInvite := w.invitation(t, ownerEmail, "owner"), w.invitation(t, viewerEmail, "viewer")
	wantPG(t, "staff_accept(pre-existing owner invitation)", w.call(t, `SELECT * FROM identity.staff_accept($1,$2)`, hash(ownerTok), ownerInvite),
		"PT403", "owner_role_disabled")
	if err := w.call(t, `SELECT * FROM identity.staff_accept($1,$2)`, hash(viewerTok), viewerInvite); err != nil {
		t.Errorf("staff_accept(viewer invitation): %v, want success", err)
	}
	if n := count(t, w.e.owner, `SELECT count(*) FROM identity.staff_invitations WHERE token_hash=$1 AND accepted_at IS NULL`, ownerInvite); n != 1 {
		t.Errorf("the refused owner invitation must stay unaccepted, rows = %d", n)
	}
}

// firstWriteOffset is the first INSERT/UPDATE/DELETE/FOR UPDATE/require_idempotency in a function definition, after
// dropping whole-line "--" comments so a comment cannot satisfy (or trip) the ordering assertion.
func firstWriteOffset(def string) (int, string) {
	var b strings.Builder
	for _, line := range strings.Split(def, "\n") {
		if i := strings.Index(line, "--"); i >= 0 {
			line = line[:i]
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	body := b.String()
	best, token := -1, ""
	for _, tok := range []string{"INSERT INTO", "UPDATE ", "DELETE FROM", "FOR UPDATE", "require_idempotency"} {
		if i := strings.Index(body, tok); i >= 0 && (best < 0 || i < best) {
			best, token = i, tok
		}
	}
	return best, token
}

// guardCallEnd returns the end offset of the require_recent_auth(...) statement starting at start.
func guardCallEnd(def string, start int) int {
	tail := def[start:]
	if end := strings.Index(tail, ");"); end >= 0 {
		return start + end + 2
	}
	return -1
}

// OG04: each guarded definer calls require_recent_auth with the fixed 10-minute window, before its first write or
// row lock (issue #228 item 2), and EXECUTE on it is held by commerce_identity_writer (its owner), commerce_identity
// (0201) and exactly the owners of the guarded definers; PUBLIC never.
func TestGuardOG04GuardPresentAndExecuteGrantsExact(t *testing.T) {
	e := fixture(t)
	ctx := context.Background()
	owners := map[string]bool{"commerce_identity_writer": true, "commerce_identity": true}
	for _, sig := range guardedSigs {
		var owner, def string
		if err := e.owner.QueryRow(ctx, `SELECT pg_get_userbyid(p.proowner), pg_get_functiondef(p.oid) FROM pg_proc p WHERE p.oid=$1::text::regprocedure`, sig).Scan(&owner, &def); err != nil {
			t.Errorf("%s: %v", sig, err)
			continue
		}
		guard := strings.Index(def, "identity.require_recent_auth(")
		if guard < 0 {
			t.Errorf("%s does not call identity.require_recent_auth", sig)
			owners[owner] = true
			continue
		}
		if end := guardCallEnd(def, guard); end < 0 || !strings.Contains(def[guard:end], "interval '10 minutes'") {
			t.Errorf("%s: require_recent_auth does not carry the fixed interval '10 minutes'", sig)
		}
		if write, tok := firstWriteOffset(def); write >= 0 && guard > write {
			t.Errorf("%s: require_recent_auth() at %d comes after the first %q at %d; the guard must precede every lock/write", sig, guard, tok, write)
		}
		owners[owner] = true
	}
	var accept string
	if err := e.owner.QueryRow(ctx, `SELECT pg_get_functiondef('identity.staff_accept(bytea,bytea)'::regprocedure)`).Scan(&accept); err != nil || !strings.Contains(accept, "owner_role_disabled") {
		t.Errorf("identity.staff_accept must refuse owner invitations (err=%v)", err)
	}
	rows, err := e.owner.Query(ctx, `SELECT CASE WHEN a.grantee=0 THEN 'PUBLIC' ELSE pg_get_userbyid(a.grantee) END
		FROM pg_proc p, LATERAL aclexplode(p.proacl) a WHERE p.oid='identity.require_recent_auth(bytea,interval)'::regprocedure AND a.privilege_type='EXECUTE'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]bool{}
	for rows.Next() {
		var g string
		if err := rows.Scan(&g); err != nil {
			t.Fatal(err)
		}
		got[g] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	keys := func(m map[string]bool) string {
		var s []string
		for k := range m {
			s = append(s, k)
		}
		sort.Strings(s)
		return strings.Join(s, ",")
	}
	if keys(got) != keys(owners) {
		t.Errorf("EXECUTE on identity.require_recent_auth: grantees {%s}, want {%s}", keys(got), keys(owners))
	}
}

// OG05 (issue #215 item 1): a password re-auth a minute ago, then an OIDC re-auth whose IdP auth_time is five minutes old:
// authenticated_at must not go back; a newer auth_time still moves it forward.
func TestGuardOG05OIDCReauthNeverMovesAuthenticatedAtBackwards(t *testing.T) {
	e := fixture(t)
	issuer, subject := "https://idp.oc03.example/"+uuid(), "sub-oc03"
	tok := session(t, e, principal(t, e, issuer, subject))
	reauth := func(authTime time.Time) {
		t.Helper()
		var outcome string
		if err := e.id.QueryRow(context.Background(), `SELECT outcome FROM identity.complete_oidc_reauth($1,$2,$3,$4)`, hash(tok), issuer, subject, authTime).Scan(&outcome); err != nil {
			t.Fatalf("complete_oidc_reauth: %v", err)
		}
		if outcome != "REAUTHENTICATED" {
			t.Fatalf("complete_oidc_reauth outcome = %q, want REAUTHENTICATED", outcome)
		}
	}
	setAuthAge(t, e, tok, "1 minute")
	before := authAt(t, e, tok)
	reauth(time.Now().Add(-5 * time.Minute))
	if after := authAt(t, e, tok); after.Before(before) {
		t.Errorf("OIDC re-auth moved authenticated_at back: %v -> %v", before, after)
	}
	setAuthAge(t, e, tok, "8 minutes")
	before = authAt(t, e, tok)
	reauth(time.Now().Add(-2 * time.Minute))
	if after := authAt(t, e, tok); !after.After(before) {
		t.Errorf("a newer auth_time must move authenticated_at forward: %v -> %v", before, after)
	}
}

// OG06 (issue #215 item 2): a future or infinite authenticated_at is not recent; a few seconds of clock skew still is.
func TestGuardOG06FutureOrInfiniteAuthenticatedAtIsNotRecent(t *testing.T) {
	e := fixture(t)
	p := principal(t, e, "", "")
	future, infinite, skew := session(t, e, p), session(t, e, p), session(t, e, p)
	setAuthAge(t, e, future, "-1 hour")
	mustExec(t, e.owner, `UPDATE identity.sessions SET authenticated_at='infinity' WHERE token_hash=$1`, hash(infinite))
	setAuthAge(t, e, skew, "-10 seconds")
	wantReauthRequired(t, "authenticated_at one hour ahead", requireRecent(e, hash(future), "10 minutes"))
	wantReauthRequired(t, "authenticated_at infinity", requireRecent(e, hash(infinite), "10 minutes"))
	if err := requireRecent(e, hash(skew), "10 minutes"); err != nil {
		t.Errorf("authenticated_at 10 s ahead (clock skew) must pass: %v", err)
	}
}

// OG07 (issue #228 item 6): a session whose authenticated_at is infinity cannot stay infinite after a successful
// OIDC re-auth: the GREATEST of the old value and the IdP auth_time is capped at the database clock.
func TestGuardOG07OIDCReauthCapsInfiniteAuthenticatedAt(t *testing.T) {
	e := fixture(t)
	issuer, subject := "https://idp.oc03-cap.example/"+uuid(), "sub-oc03-cap"
	tok := session(t, e, principal(t, e, issuer, subject))
	mustExec(t, e.owner, `UPDATE identity.sessions SET authenticated_at='infinity' WHERE token_hash=$1`, hash(tok))
	var outcome string
	if err := e.id.QueryRow(context.Background(), `SELECT outcome FROM identity.complete_oidc_reauth($1,$2,$3,clock_timestamp()-interval '1 minute')`,
		hash(tok), issuer, subject).Scan(&outcome); err != nil {
		t.Fatalf("complete_oidc_reauth: %v", err)
	}
	if outcome != "REAUTHENTICATED" {
		t.Fatalf("complete_oidc_reauth outcome = %q, want REAUTHENTICATED", outcome)
	}
	var infinite, capped bool
	if err := e.owner.QueryRow(context.Background(), `SELECT authenticated_at='infinity',
		authenticated_at <> 'infinity' AND authenticated_at <= clock_timestamp() FROM identity.sessions WHERE token_hash=$1`, hash(tok)).
		Scan(&infinite, &capped); err != nil {
		t.Fatalf("read authenticated_at: %v", err)
	}
	if infinite || !capped {
		t.Errorf("after OIDC re-auth authenticated_at infinity=%v capped=%v, want false/true", infinite, capped)
	}
}

// OG08 (issue #228 item 1): a session authenticated 11 minutes ago is refused by every guarded definer. A guard
// widened to 60 minutes (or any window above 11) turns this red.
func TestGuardOG08ElevenMinuteSessionRefusedByEveryDefiner(t *testing.T) {
	w := newWorld(t, fixture(t))
	setAuthAge(t, w.e, w.stale, "11 minutes")
	for _, c := range guardCases() {
		t.Run(c.name, func(t *testing.T) {
			wantReauthRequired(t, c.name, w.call(t, c.sql, c.args(w, w.stale)...))
		})
	}
}

// OG09 (issue #228 item 1): a session authenticated 9 minutes ago passes every guard. A guard narrowed below 10
// minutes turns this red.
func TestGuardOG09NineMinuteSessionPassesTheGuard(t *testing.T) {
	w := newWorld(t, fixture(t))
	setAuthAge(t, w.e, w.stale, "9 minutes")
	for _, c := range guardCases() {
		t.Run(c.name, func(t *testing.T) {
			err := w.call(t, c.sql, c.args(w, w.stale)...)
			if isReauthRequired(err) {
				t.Fatalf("%s: a 9-minute-old session was refused: %v", c.name, err)
			}
			if c.mustSucceed && err != nil {
				t.Errorf("%s: 9-minute-old session: %v, want success", c.name, err)
			}
		})
	}
}
