// Purpose: OC-02 recent re-authentication gates (contracts/merchant-password-auth-v1.md §16, Red tests): fail-closed
// identity.require_recent_auth, the authenticated_at backfill of migration 0201, the password/code and OIDC re-auth
// definers (session binding, principal match, no new session and no session.issued event) and their PUBLIC revoke.
// Depends on: the disposable lc_foundation_test database (LC_TEST_DATABASE_ALLOWED=1, LC_TEST_DATABASE_URL),
// livecommerce/migrations (Apply), internal/platform (OpenIdentityPool), internal/identity (HashPassword).
// Used by: go test ./tests/identity (CI shard:unit via scripts/dev/test-local.sh).
package reauthtests

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/identity"
	"livecommerce/internal/platform"
	"livecommerce/migrations"
)

type env struct {
	owner *pgxpool.Pool // migration owner (superuser in the disposable DB): fixtures and catalog reads only
	id    *pgxpool.Pool // a real commerce_identity login, as production uses (platform.OpenIdentityPool)
	dsn   string
}

var (
	envOnce sync.Once
	envVal  *env
	envSkip string
	envErr  error
)

func fixture(t *testing.T) *env {
	t.Helper()
	envOnce.Do(func() { envVal, envSkip, envErr = newEnv() })
	if envSkip != "" {
		t.Skip(envSkip)
	}
	if envErr != nil {
		t.Fatal(envErr)
	}
	return envVal
}

func newEnv() (*env, string, error) {
	if os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" || os.Getenv("LC_TEST_DATABASE_URL") == "" {
		return nil, "LC_TEST_DATABASE_ALLOWED=1 and LC_TEST_DATABASE_URL are required; OC-02 re-auth gates NOT_RUN", nil
	}
	cfg, err := pgxpool.ParseConfig(os.Getenv("LC_TEST_DATABASE_URL"))
	if err != nil {
		return nil, "", errors.New("LC_TEST_DATABASE_URL is invalid")
	}
	if cfg.ConnConfig.Host != "127.0.0.1" || cfg.ConnConfig.Database != "lc_foundation_test" {
		return nil, "", errors.New("refusing database outside 127.0.0.1/lc_foundation_test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	owner, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, "", err
	}
	if err := migrations.Apply(ctx, owner); err != nil {
		return nil, "", fmt.Errorf("apply migrations: %w", err)
	}
	var raw [12]byte
	_, _ = rand.Read(raw[:])
	role := "reauth_" + hex.EncodeToString(raw[:])
	secret := hex.EncodeToString(raw[:]) + hex.EncodeToString(raw[:])
	if _, err := owner.Exec(ctx, `CREATE ROLE `+pgx.Identifier{role}.Sanitize()+` LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE IN ROLE commerce_identity PASSWORD '`+secret+`'`); err != nil {
		return nil, "", err
	}
	u, _ := url.Parse(os.Getenv("LC_TEST_DATABASE_URL"))
	u.User = url.UserPassword(role, secret)
	id, err := platform.OpenIdentityPool(ctx, u.String())
	if err != nil {
		return nil, "", err
	}
	return &env{owner: owner, id: id, dsn: u.String()}, "", nil
}

func token() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func hash(s string) []byte { h := sha256.Sum256([]byte(s)); return h[:] }

func rnd32() []byte { b := make([]byte, 32); _, _ = rand.Read(b); return b }

func uuid() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

var seq atomic.Int64

func mustExec(t *testing.T, p *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := p.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func count(t *testing.T, p *pgxpool.Pool, sql string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := p.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return n
}

// principal inserts an active principal; with an OIDC identity when issuer != "".
func principal(t *testing.T, e *env, issuer, subject string) string {
	t.Helper()
	p := uuid()
	mustExec(t, e.owner, `INSERT INTO identity.principals(id) VALUES ($1::uuid)`, p)
	if issuer != "" {
		mustExec(t, e.owner, `INSERT INTO identity.external_identities(issuer,subject,principal_id) VALUES ($1,$2,$3::uuid)`, issuer, subject, p)
	}
	return p
}

// session inserts a live merchant session with its session.issued event and returns the bearer token.
func session(t *testing.T, e *env, p string) string {
	t.Helper()
	tok, id := token(), uuid()
	mustExec(t, e.owner, `INSERT INTO identity.sessions(id,token_hash,principal_id,audience,expires_at) VALUES ($1::uuid,$2,$3::uuid,'merchant',clock_timestamp()+interval '1 hour')`, id, hash(tok), p)
	mustExec(t, e.owner, `INSERT INTO identity.session_events(principal_id,session_id,action) VALUES ($1::uuid,$2::uuid,'session.issued')`, p, id)
	return tok
}

func setAuthAge(t *testing.T, e *env, tok, age string) {
	t.Helper()
	mustExec(t, e.owner, `UPDATE identity.sessions SET authenticated_at = clock_timestamp() - $2::interval WHERE token_hash=$1`, hash(tok), age)
}

func authAt(t *testing.T, e *env, tok string) time.Time {
	t.Helper()
	var at time.Time
	if err := e.owner.QueryRow(context.Background(), `SELECT authenticated_at FROM identity.sessions WHERE token_hash=$1`, hash(tok)).Scan(&at); err != nil {
		t.Fatalf("authenticated_at: %v", err)
	}
	return at
}

// sessionFacts is the evidence that a re-auth issued nothing: sessions and session.issued events of the principals.
func sessionFacts(t *testing.T, e *env, principals ...string) [2]int64 {
	t.Helper()
	return [2]int64{
		count(t, e.owner, `SELECT count(*) FROM identity.sessions WHERE principal_id::text = ANY($1)`, principals),
		count(t, e.owner, `SELECT count(*) FROM identity.session_events WHERE action='session.issued' AND principal_id::text = ANY($1)`, principals),
	}
}

func requireRecent(e *env, h []byte, window string) error {
	_, err := e.id.Exec(context.Background(), `SELECT identity.require_recent_auth($1,$2::interval)`, h, window)
	return err
}

func wantReauthRequired(t *testing.T, what string, err error) {
	t.Helper()
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != "PT403" || pg.Message != "reauth_required" {
		t.Errorf("%s: err = %v, want PT403 reauth_required", what, err)
	}
}

// RA01: require_recent_auth fails closed (NULL / unknown / revoked / expired / stale / -infinity / disabled credential)
// and passes only for a fresh live session; a stale session stays valid (403, not a logout).
func TestReauthRA01RequireRecentAuthFailClosed(t *testing.T) {
	e := fixture(t)
	p := principal(t, e, "", "")
	fresh, stale, revoked, expired, never := session(t, e, p), session(t, e, p), session(t, e, p), session(t, e, p), session(t, e, p)
	setAuthAge(t, e, fresh, "0 seconds")
	setAuthAge(t, e, stale, "2 hours")
	setAuthAge(t, e, revoked, "0 seconds")
	setAuthAge(t, e, expired, "0 seconds")
	mustExec(t, e.owner, `UPDATE identity.sessions SET revoked_at=clock_timestamp() WHERE token_hash=$1`, hash(revoked))
	mustExec(t, e.owner, `UPDATE identity.sessions SET expires_at=clock_timestamp()-interval '1 second' WHERE token_hash=$1`, hash(expired))
	mustExec(t, e.owner, `UPDATE identity.sessions SET authenticated_at='-infinity' WHERE token_hash=$1`, hash(never))

	if err := requireRecent(e, hash(fresh), "10 minutes"); err != nil {
		t.Fatalf("fresh session: %v", err)
	}
	wantReauthRequired(t, "NULL hash", requireRecent(e, nil, "10 minutes"))
	wantReauthRequired(t, "unknown hash", requireRecent(e, rnd32(), "10 minutes"))
	wantReauthRequired(t, "revoked session", requireRecent(e, hash(revoked), "10 minutes"))
	wantReauthRequired(t, "expired session", requireRecent(e, hash(expired), "10 minutes"))
	wantReauthRequired(t, "authenticated_at 2 hours ago", requireRecent(e, hash(stale), "10 minutes"))
	wantReauthRequired(t, "authenticated_at -infinity", requireRecent(e, hash(never), "60 minutes"))
	if n := count(t, e.owner, `SELECT count(*) FROM identity.sessions WHERE token_hash=$1 AND revoked_at IS NULL AND expires_at>clock_timestamp()`, hash(stale)); n != 1 {
		t.Errorf("stale session was invalidated by the check (rows=%d); reauth_required must not log out", n)
	}
	for _, w := range []string{"0 seconds", "59 seconds", "61 minutes"} {
		if err := requireRecent(e, hash(fresh), w); err == nil {
			t.Errorf("window %s accepted; allowed is 1-60 minutes", w)
		}
	}
	if _, err := e.id.Exec(context.Background(), `SELECT identity.require_recent_auth($1,NULL)`, hash(fresh)); err == nil {
		t.Error("NULL window passed")
	}

	// PD12: a disabled password credential revokes the guarded operations until a reset.
	q := principal(t, e, "", "")
	phc, err := identity.HashPassword("correct horse battery 1")
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, e.owner, `INSERT INTO identity.password_credentials(principal_id,email,password_hash,email_verified_at,disabled_at) VALUES ($1::uuid,$2,$3,clock_timestamp(),clock_timestamp())`,
		q, fmt.Sprintf("ra01.%d.%d@example.test", os.Getpid(), seq.Add(1)), phc)
	dis := session(t, e, q)
	setAuthAge(t, e, dis, "0 seconds")
	wantReauthRequired(t, "disabled credential", requireRecent(e, hash(dis), "10 minutes"))
}

// reauthDefiners are the definers 0201 adds, by identity arguments.
var reauthDefiners = []string{
	"identity.require_recent_auth(bytea,interval)",
	"identity.reauth_material(bytea)",
	"identity.start_reauth_challenge(uuid,bytea,bigint,bytea,bytea,text,bytea)",
	"identity.complete_reauth_challenge(bytea,bytea)",
	"identity.start_oidc_reauth_flow(bytea,bytea,text,text,text,bytea)",
	"identity.consume_login_flow_with_session(bytea,bytea,text)",
	"identity.complete_oidc_reauth(bytea,text,text,timestamptz)",
}

// RA02: PUBLIC cannot execute any re-auth definer; each is SECURITY DEFINER owned by commerce_identity_writer with
// search_path=pg_catalog and EXECUTE for commerce_identity. A role outside commerce_identity is refused (42501).
func TestReauthRA02PublicCannotExecute(t *testing.T) {
	e := fixture(t)
	ctx := context.Background()
	for _, fn := range reauthDefiners {
		var owner string
		var secdef, public, identityExec bool
		var config []string
		err := e.owner.QueryRow(ctx, `SELECT pg_get_userbyid(p.proowner), p.prosecdef, coalesce(p.proconfig,'{}'),
			EXISTS (SELECT 1 FROM aclexplode(coalesce(p.proacl, acldefault('f', p.proowner))) a WHERE a.grantee=0 AND a.privilege_type='EXECUTE'),
			has_function_privilege('commerce_identity', p.oid, 'EXECUTE')
			FROM pg_proc p WHERE p.oid=$1::regprocedure`, fn).Scan(&owner, &secdef, &config, &public, &identityExec)
		if err != nil {
			t.Errorf("%s: %v", fn, err)
			continue
		}
		if owner != "commerce_identity_writer" || !secdef || len(config) != 1 || config[0] != "search_path=pg_catalog" || public || !identityExec {
			t.Errorf("%s: owner=%s secdef=%v config=%v public=%v commerce_identity=%v", fn, owner, secdef, config, public, identityExec)
		}
	}
	// A real call as a role that is only PUBLIC.
	for _, call := range []string{
		`SELECT * FROM identity.complete_reauth_challenge('\x00'::bytea,'\x00'::bytea)`,
		`SELECT * FROM identity.complete_oidc_reauth('\x00'::bytea,'i','s',now())`,
	} {
		tx, err := e.owner.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		role := "reauth_public_" + hex.EncodeToString(rnd32()[:6])
		if _, err := tx.Exec(ctx, `CREATE ROLE `+role+` NOLOGIN; SET LOCAL ROLE `+role); err != nil {
			t.Fatal(err)
		}
		_, err = tx.Exec(ctx, call)
		var pg *pgconn.PgError
		if !errors.As(err, &pg) || pg.Code != "42501" {
			t.Errorf("PUBLIC-only role %s: err = %v, want 42501", call, err)
		}
		_ = tx.Rollback(ctx)
	}
}

// RA03: replaying migrations/0201 over sessions without the column backfills authenticated_at from the session's
// session.issued event, -infinity when there is none, then NOT NULL with DEFAULT clock_timestamp() (set after the backfill).
func TestReauthRA03Backfill(t *testing.T) {
	e := fixture(t)
	ctx := context.Background()
	body, err := os.ReadFile("../../migrations/0201_recent_auth.sql")
	if err != nil {
		t.Fatalf("migration 0201: %v", err)
	}
	p := principal(t, e, "", "")
	tx, err := e.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // the replay never commits: the shared DB keeps its migrated state
	if _, err := tx.Exec(ctx, `ALTER TABLE identity.sessions DROP COLUMN authenticated_at`); err != nil {
		t.Fatalf("drop column to reproduce the pre-0201 state: %v", err)
	}
	withEvent, without := uuid(), uuid()
	issued := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	for _, id := range []string{withEvent, without} {
		if _, err := tx.Exec(ctx, `INSERT INTO identity.sessions(id,token_hash,principal_id,audience,expires_at) VALUES ($1::uuid,$2,$3::uuid,'merchant',clock_timestamp()+interval '1 hour')`, id, rnd32(), p); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO identity.session_events(principal_id,session_id,action,created_at) VALUES ($1::uuid,$2::uuid,'session.issued',$3)`, p, withEvent, issued); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO identity.session_events(principal_id,session_id,action) VALUES ($1::uuid,$2::uuid,'session.revoked')`, p, without); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, string(body)); err != nil {
		t.Fatalf("replay 0201: %v", err)
	}
	var got time.Time
	var neg bool
	if err := tx.QueryRow(ctx, `SELECT authenticated_at FROM identity.sessions WHERE id=$1::uuid`, withEvent).Scan(&got); err != nil || !got.Equal(issued) {
		t.Errorf("session with session.issued: authenticated_at=%v (%v), want %v", got, err, issued)
	}
	if err := tx.QueryRow(ctx, `SELECT authenticated_at='-infinity' FROM identity.sessions WHERE id=$1::uuid`, without).Scan(&neg); err != nil || !neg {
		t.Errorf("session without session.issued: -infinity=%v (%v)", neg, err)
	}
	var nullable, def string
	if err := tx.QueryRow(ctx, `SELECT is_nullable, coalesce(column_default,'') FROM information_schema.columns WHERE table_schema='identity' AND table_name='sessions' AND column_name='authenticated_at'`).Scan(&nullable, &def); err != nil || nullable != "NO" || def != "clock_timestamp()" {
		t.Errorf("authenticated_at nullable=%s default=%q (%v), want NO / clock_timestamp()", nullable, def, err)
	}
}

// RA04: OIDC re-auth. A different account, an unknown identity, a stale or missing auth_time are refused with
// authenticated_at unchanged; the session's own account refreshes only that session. Nothing issues a session.
func TestReauthRA04OIDCPrincipalMatch(t *testing.T) {
	e := fixture(t)
	ctx := context.Background()
	iss := "https://idp.ra04.example.test"
	subA, subB := "a-"+uuid(), "b-"+uuid()
	a, b := principal(t, e, iss, subA), principal(t, e, iss, subB)
	tokA, tokB := session(t, e, a), session(t, e, b)
	setAuthAge(t, e, tokA, "2 hours")
	setAuthAge(t, e, tokB, "2 hours")
	before, beforeB := authAt(t, e, tokA), authAt(t, e, tokB)
	facts := sessionFacts(t, e, a, b)

	call := func(h []byte, issuer, subject string, authTime any) string {
		t.Helper()
		var outcome string
		var at *time.Time
		if err := e.id.QueryRow(ctx, `SELECT outcome, authenticated_at FROM identity.complete_oidc_reauth($1,$2,$3,$4::timestamptz)`, h, issuer, subject, authTime).Scan(&outcome, &at); err != nil {
			t.Fatalf("complete_oidc_reauth: %v", err)
		}
		return outcome
	}
	now := time.Now()
	for name, c := range map[string]struct {
		sub  string
		when any
	}{
		"other account (session A, login B)": {subB, now},
		"unknown identity":                   {"nobody-" + uuid(), now},
		"auth_time 11 minutes old":           {subA, now.Add(-11 * time.Minute)},
		"auth_time missing":                  {subA, nil},
	} {
		if got := call(hash(tokA), iss, c.sub, c.when); got != "REFUSED" {
			t.Errorf("%s: outcome %s, want REFUSED", name, got)
		}
		if at := authAt(t, e, tokA); !at.Equal(before) {
			t.Errorf("%s: authenticated_at moved %v -> %v", name, before, at)
		}
	}
	if got := call(rnd32(), iss, subA, now); got != "UNAUTHORIZED" {
		t.Errorf("unknown session: outcome %s, want UNAUTHORIZED", got)
	}
	if got := call(hash(tokA), iss, subA, now); got != "REAUTHENTICATED" {
		t.Fatalf("own account: outcome %s, want REAUTHENTICATED", got)
	}
	if at := authAt(t, e, tokA); !at.After(before) {
		t.Errorf("own account: authenticated_at %v not after %v", at, before)
	}
	if at := authAt(t, e, tokB); !at.Equal(beforeB) {
		t.Errorf("session B moved by A's re-auth: %v -> %v", beforeB, at)
	}
	if err := requireRecent(e, hash(tokA), "10 minutes"); err != nil {
		t.Errorf("after OIDC re-auth: %v", err)
	}
	wantReauthRequired(t, "session B after A's re-auth", requireRecent(e, hash(tokB), "10 minutes"))
	if got := sessionFacts(t, e, a, b); got != facts {
		t.Errorf("OIDC re-auth issued sessions/events: %v -> %v", facts, got)
	}
}

// RA05: password + code re-auth definers. The challenge is bound to the calling session hash: a code issued to
// session A cannot refresh session B; a wrong code counts an attempt; the right code refreshes only A, is single
// use, and no session or session.issued event is added on success or refusal.
func TestReauthRA05ChallengeBoundToSession(t *testing.T) {
	e := fixture(t)
	ctx := context.Background()
	p := principal(t, e, "", "")
	phc, err := identity.HashPassword("correct horse battery 1")
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, e.owner, `INSERT INTO identity.password_credentials(principal_id,email,password_hash,email_verified_at) VALUES ($1::uuid,$2,$3,clock_timestamp())`,
		p, fmt.Sprintf("ra05.%d.%d@example.test", os.Getpid(), seq.Add(1)), phc)
	tokA, tokB := session(t, e, p), session(t, e, p)
	setAuthAge(t, e, tokA, "2 hours")
	setAuthAge(t, e, tokB, "2 hours")
	beforeA, beforeB := authAt(t, e, tokA), authAt(t, e, tokB)
	facts := sessionFacts(t, e, p)

	start := func(tok string, code []byte) string {
		t.Helper()
		id := uuid()
		var exp time.Time
		if err := e.id.QueryRow(ctx, `SELECT identity.start_reauth_challenge($1::uuid,$2,1,$3,$4,'en',$5)`, id, hash(tok), rnd32(), code, rnd32()).Scan(&exp); err != nil {
			t.Fatalf("start_reauth_challenge: %v", err)
		}
		if d := time.Until(exp); d < 9*time.Minute || d > 11*time.Minute {
			t.Errorf("challenge expires in %v, want 10 minutes", d)
		}
		return id
	}
	complete := func(tok string, code []byte) string {
		t.Helper()
		var outcome string
		var at *time.Time
		if err := e.id.QueryRow(ctx, `SELECT outcome, authenticated_at FROM identity.complete_reauth_challenge($1,$2)`, hash(tok), code).Scan(&outcome, &at); err != nil {
			t.Fatalf("complete_reauth_challenge: %v", err)
		}
		return outcome
	}
	first := start(tokA, rnd32())
	code := rnd32()
	second := start(tokA, code)
	if n := count(t, e.owner, `SELECT count(*) FROM identity.email_challenges WHERE id=$1::uuid AND consumed_reason='superseded'`, first); n != 1 {
		t.Errorf("a new start did not supersede the session's open reauth challenge")
	}
	if n := count(t, e.owner, `SELECT count(*) FROM identity.email_challenges WHERE id=$1::uuid AND purpose='reauth' AND session_hash=$2 AND principal_id=$3::uuid`, second, hash(tokA), p); n != 1 {
		t.Errorf("reauth challenge is not bound to the calling session hash and principal")
	}
	if got := complete(tokB, code); got != "INVALID" {
		t.Errorf("session A's code on session B: %s, want INVALID", got)
	}
	if got := complete(tokA, rnd32()); got != "INVALID" {
		t.Errorf("wrong code: %s, want INVALID", got)
	}
	if n := count(t, e.owner, `SELECT attempts FROM identity.email_challenges WHERE id=$1::uuid`, second); n != 1 {
		t.Errorf("wrong code attempts = %d, want 1", n)
	}
	if at := authAt(t, e, tokA); !at.Equal(beforeA) {
		t.Errorf("refused re-auth moved authenticated_at %v -> %v", beforeA, at)
	}
	if got := complete(tokA, code); got != "VERIFIED" {
		t.Fatalf("right code: %s, want VERIFIED", got)
	}
	if got := complete(tokA, code); got != "INVALID" {
		t.Errorf("replayed code: %s, want INVALID", got)
	}
	if at := authAt(t, e, tokA); !at.After(beforeA) {
		t.Errorf("session A authenticated_at %v not after %v", at, beforeA)
	}
	if at := authAt(t, e, tokB); !at.Equal(beforeB) {
		t.Errorf("session B moved by A's re-auth: %v -> %v", beforeB, at)
	}
	if err := requireRecent(e, hash(tokA), "10 minutes"); err != nil {
		t.Errorf("after re-auth: %v", err)
	}
	wantReauthRequired(t, "session B", requireRecent(e, hash(tokB), "10 minutes"))
	if got := sessionFacts(t, e, p); got != facts {
		t.Errorf("password re-auth issued sessions/events: %v -> %v", facts, got)
	}
}
