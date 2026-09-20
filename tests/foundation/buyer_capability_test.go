package foundation_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/buyer"
	"livecommerce/internal/platform"
)

func TestBuyerCapabilityBoundary(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	authorities := openBuyerTestPools(t, f)
	runtime, issuer := authorities.runtime, authorities.issuer
	service, err := buyer.New(issuer, time.Hour)
	if err != nil {
		t.Fatalf("new buyer service: %v", err)
	}
	tenantID, storeID, otherStoreID := seedBuyerStores(t, f)
	assertBuyerWriterCanLockStore(t, f, tenantID, storeID)

	t.Run("issue resolve revoke and hash-only persistence", func(t *testing.T) {
		beforeMemberships := countRows(t, f.owner, `SELECT count(*) FROM identity.memberships`)
		first, err := service.IssueForTrustedStore(ctx, storeID)
		if err != nil {
			t.Fatalf("issue first capability: %v", err)
		}
		second, err := service.IssueForTrustedStore(ctx, storeID)
		if err != nil {
			t.Fatalf("issue second capability: %v", err)
		}
		if first.Token == second.Token || first.Scope.OwnerID == second.Scope.OwnerID || first.Scope.SessionID == second.Scope.SessionID {
			t.Fatal("repeated issuance did not create independent capability owners")
		}
		decoded, decodeErr := base64.RawURLEncoding.Strict().DecodeString(first.Token)
		if decodeErr != nil || len(first.Token) != 43 || len(decoded) != 32 {
			t.Fatal("issued token is not strict 32-byte base64url")
		}
		if first.Scope.TenantID != tenantID || first.Scope.StoreID != storeID || first.ExpiresAt.Before(time.Now().Add(59*time.Minute)) {
			t.Fatalf("unexpected issued capability metadata: scope=%+v expires_at=%s", first.Scope, first.ExpiresAt)
		}
		encoded, err := json.Marshal(first)
		if err != nil || strings.Contains(string(encoded), first.Token) || strings.Contains(string(encoded), "Token") {
			t.Fatalf("capability JSON exposed token field: marshal_error=%v", err)
		}
		if got := countRows(t, f.owner, `SELECT count(*) FROM identity.memberships`); got != beforeMemberships {
			t.Fatalf("buyer issuance changed merchant memberships: before=%d after=%d", beforeMemberships, got)
		}
		hash := sha256.Sum256([]byte(first.Token))
		var storedHash []byte
		if err := f.owner.QueryRow(ctx, `SELECT token_hash FROM buyer.capability_sessions WHERE id=$1`, first.Scope.SessionID).Scan(&storedHash); err != nil {
			t.Fatalf("read stored capability hash: %v", err)
		}
		if string(storedHash) != string(hash[:]) || string(storedHash) == first.Token {
			t.Fatal("capability persistence was not hash-only")
		}
		assertScope(t, runtime, first, storeID)
		assertScope(t, runtime, second, storeID)

		ownersBefore := countRows(t, f.owner, `SELECT count(*) FROM buyer.owners`)
		sessionsBefore := countRows(t, f.owner, `SELECT count(*) FROM buyer.capability_sessions`)
		mustExec(t, f.owner, `CREATE FUNCTION buyer.test_fail_issued_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action='capability.issued' THEN RAISE EXCEPTION 'synthetic issued event failure'; END IF; RETURN NEW; END $$`)
		mustExec(t, f.owner, `CREATE TRIGGER buyer_test_fail_issued_event BEFORE INSERT ON buyer.capability_events FOR EACH ROW EXECUTE FUNCTION buyer.test_fail_issued_event()`)
		func() {
			defer func() {
				mustExec(t, f.owner, `DROP TRIGGER buyer_test_fail_issued_event ON buyer.capability_events`)
				mustExec(t, f.owner, `DROP FUNCTION buyer.test_fail_issued_event()`)
			}()
			if _, err := service.IssueForTrustedStore(ctx, storeID); err == nil || errors.Is(err, buyer.ErrInvalid) || errors.Is(err, buyer.ErrUnauthorized) {
				t.Fatalf("issued-event failure was not sanitized: %v", err)
			}
		}()
		if countRows(t, f.owner, `SELECT count(*) FROM buyer.owners`) != ownersBefore || countRows(t, f.owner, `SELECT count(*) FROM buyer.capability_sessions`) != sessionsBefore {
			t.Fatal("issued-event failure did not roll back owner and session")
		}

		called := false
		if err := buyer.WithScope(ctx, runtime, first.Token, otherStoreID, func(context.Context, pgx.Tx, buyer.Scope) error {
			called = true
			return nil
		}); !errors.Is(err, buyer.ErrUnauthorized) || called {
			t.Fatalf("cross-store scope err=%v called=%v", err, called)
		}
		if err := service.Revoke(ctx, first.Token, otherStoreID); err != nil {
			t.Fatalf("wrong-store revoke must be a no-op: %v", err)
		}
		assertScope(t, runtime, first, storeID)
		if err := service.Revoke(ctx, first.Token, storeID); err != nil {
			t.Fatalf("revoke capability: %v", err)
		}
		if err := service.Revoke(ctx, first.Token, storeID); err != nil {
			t.Fatalf("repeat revoke capability: %v", err)
		}
		assertUnauthorized(t, runtime, first.Token, storeID)
		if got := countRows(t, f.owner, `SELECT count(*) FROM buyer.capability_events WHERE session_id=$1 AND action='capability.revoked'`, first.Scope.SessionID); got != 1 {
			t.Fatalf("revocation event count=%d, want 1", got)
		}
	})

	t.Run("validation states and authority matrix", func(t *testing.T) {
		if _, err := buyer.New(nil, time.Hour); !errors.Is(err, buyer.ErrInvalid) {
			t.Fatalf("nil issuer error=%v", err)
		}
		for _, ttl := range []time.Duration{time.Minute - time.Second, time.Minute + time.Nanosecond, 30*24*time.Hour + time.Second} {
			if _, err := buyer.New(issuer, ttl); !errors.Is(err, buyer.ErrInvalid) {
				t.Fatalf("ttl %s error=%v", ttl, err)
			}
		}
		if _, err := service.IssueForTrustedStore(ctx, "NOT-A-UUID"); !errors.Is(err, buyer.ErrInvalid) {
			t.Fatalf("invalid store error=%v", err)
		}
		if err := service.Revoke(ctx, "bad", storeID); !errors.Is(err, buyer.ErrInvalid) {
			t.Fatalf("invalid revoke error=%v", err)
		}
		assertUnauthorized(t, runtime, "bad", storeID)
		assertUnauthorized(t, runtime, f.tokens["a"], storeID)

		ownerInactive := mustIssue(t, service, storeID)
		mustExec(t, f.owner, `UPDATE buyer.owners SET active=false WHERE id=$1`, ownerInactive.Scope.OwnerID)
		assertUnauthorized(t, runtime, ownerInactive.Token, storeID)

		expired := mustIssue(t, service, storeID)
		mustExec(t, f.owner, `UPDATE buyer.capability_sessions SET created_at=clock_timestamp()-interval '2 seconds',expires_at=clock_timestamp()-interval '1 second' WHERE id=$1`, expired.Scope.SessionID)
		assertUnauthorized(t, runtime, expired.Token, storeID)
		if err := service.Revoke(ctx, expired.Token, storeID); err != nil {
			t.Fatalf("expired revoke must be a no-op: %v", err)
		}
		var revoked bool
		if err := f.owner.QueryRow(ctx, `SELECT revoked_at IS NOT NULL FROM buyer.capability_sessions WHERE id=$1`, expired.Scope.SessionID).Scan(&revoked); err != nil {
			t.Fatalf("read expired capability: %v", err)
		}
		if revoked || countRows(t, f.owner, `SELECT count(*) FROM buyer.capability_events WHERE session_id=$1`, expired.Scope.SessionID) != 1 {
			t.Fatal("expired revoke mutated the session or added a revoke event")
		}

		storeInactive := mustIssue(t, service, storeID)
		mustExec(t, f.owner, `UPDATE control.stores SET active=false WHERE id=$1`, storeID)
		assertUnauthorized(t, runtime, storeInactive.Token, storeID)
		if _, err := service.IssueForTrustedStore(ctx, storeID); !errors.Is(err, buyer.ErrUnauthorized) {
			t.Fatalf("inactive store issue error=%v", err)
		}
		mustExec(t, f.owner, `UPDATE control.stores SET active=true WHERE id=$1`, storeID)

		tenantInactive := mustIssue(t, service, storeID)
		mustExec(t, f.owner, `UPDATE control.tenants SET active=false WHERE id=$1`, tenantID)
		assertUnauthorized(t, runtime, tenantInactive.Token, storeID)
		mustExec(t, f.owner, `UPDATE control.tenants SET active=true WHERE id=$1`, tenantID)

		hash := sha256.Sum256([]byte("authority-matrix"))
		assertSQLDenied(t, runtime, `SELECT * FROM buyer.issue_capability($1::uuid,$2,60)`, storeID, hash[:])
		assertSQLDenied(t, runtime, `SELECT buyer.revoke_capability($1,$2::uuid)`, hash[:], storeID)
		assertSQLDenied(t, runtime, `SELECT * FROM buyer.capability_sessions`)
		assertSQLDenied(t, runtime, `INSERT INTO identity.memberships(tenant_id,principal_id) VALUES($1,$2)`, tenantID, randomUUID())
		assertSQLDenied(t, runtime, `SELECT * FROM identity.resolve_access($1,$2::uuid,'store:read')`, hash[:], storeID)
		assertSQLDenied(t, issuer, `SELECT * FROM buyer.resolve_scope($1,$2::uuid)`, hash[:], storeID)
		assertSQLDenied(t, issuer, `SELECT * FROM buyer.capability_sessions`)
		assertSQLDenied(t, issuer, `INSERT INTO buyer.owners(tenant_id,store_id) VALUES($1,$2)`, tenantID, storeID)
		assertSQLDenied(t, issuer, `INSERT INTO identity.memberships(tenant_id,principal_id) VALUES($1,$2)`, tenantID, randomUUID())
		assertSQLDenied(t, f.runtime, `SELECT * FROM buyer.resolve_scope($1,$2::uuid)`, hash[:], storeID)
		assertSQLDenied(t, f.runtime, `SELECT buyer.revoke_capability($1,$2::uuid)`, hash[:], storeID)
		assertSQLDenied(t, authorities.identity, `SELECT * FROM buyer.issue_capability($1::uuid,$2,60)`, storeID, hash[:])
		assertSQLDenied(t, authorities.identity, `SELECT * FROM buyer.resolve_scope($1,$2::uuid)`, hash[:], storeID)
		assertSQLDenied(t, authorities.identity, `SELECT buyer.revoke_capability($1,$2::uuid)`, hash[:], storeID)
		if pool, err := platform.OpenBuyerPool(ctx, f.databaseURL); err == nil {
			pool.Close()
			t.Fatal("privileged database owner opened as buyer runtime")
		}
		if pool, err := platform.OpenBuyerIssuerPool(ctx, f.databaseURL); err == nil {
			pool.Close()
			t.Fatal("privileged database owner opened as buyer issuer")
		}
		for _, hazard := range []struct {
			role string
			open func(context.Context, string) (*pgxpool.Pool, error)
			dsn  string
		}{{authorities.runtimeRole, platform.OpenBuyerPool, authorities.runtimeURL}, {authorities.issuerRole, platform.OpenBuyerIssuerPool, authorities.issuerURL}} {
			func() {
				mustExec(t, f.owner, `GRANT commerce_buyer_writer TO `+pgx.Identifier{hazard.role}.Sanitize())
				defer mustExec(t, f.owner, `REVOKE commerce_buyer_writer FROM `+pgx.Identifier{hazard.role}.Sanitize())
				assertPoolRejected(t, "buyer writer membership", hazard.open, hazard.dsn)
			}()
		}
		constructors := map[string]func(context.Context, string) (*pgxpool.Pool, error){
			"merchant runtime": platform.OpenPool, "identity": platform.OpenIdentityPool,
			"buyer runtime": platform.OpenBuyerPool, "buyer issuer": platform.OpenBuyerIssuerPool,
		}
		for name, constructor := range constructors {
			assertPoolRejected(t, name+" mixed", constructor, authorities.mixedURL)
		}
		assertPoolRejected(t, "merchant runtime from buyer runtime", platform.OpenPool, authorities.runtimeURL)
		assertPoolRejected(t, "identity from buyer runtime", platform.OpenIdentityPool, authorities.runtimeURL)
		assertPoolRejected(t, "buyer issuer from buyer runtime", platform.OpenBuyerIssuerPool, authorities.runtimeURL)
		assertPoolRejected(t, "merchant runtime from buyer issuer", platform.OpenPool, authorities.issuerURL)
		assertPoolRejected(t, "identity from buyer issuer", platform.OpenIdentityPool, authorities.issuerURL)
		assertPoolRejected(t, "buyer runtime from buyer issuer", platform.OpenBuyerPool, authorities.issuerURL)
		assertPoolRejected(t, "merchant runtime from identity", platform.OpenPool, authorities.identityURL)
		assertPoolRejected(t, "buyer runtime from identity", platform.OpenBuyerPool, authorities.identityURL)
		assertPoolRejected(t, "buyer issuer from identity", platform.OpenBuyerIssuerPool, authorities.identityURL)
		assertPoolRejected(t, "buyer runtime from merchant runtime", platform.OpenBuyerPool, f.runtime.Config().ConnConfig.ConnString())
		assertPoolRejected(t, "buyer issuer from merchant runtime", platform.OpenBuyerIssuerPool, f.runtime.Config().ConnConfig.ConnString())

		left, right := mustIssue(t, service, storeID), mustIssue(t, service, storeID)
		wrongStoreHash := sha256.Sum256([]byte("known-other-store"))
		_, err = f.owner.Exec(ctx, `INSERT INTO buyer.capability_sessions(tenant_id,store_id,owner_id,token_hash,expires_at) VALUES($1,$2,$3,$4,clock_timestamp()+interval '1 hour')`, tenantID, otherStoreID, left.Scope.OwnerID, wrongStoreHash[:])
		requirePGCode(t, err, "23503", "session composite FK")
		_, err = f.owner.Exec(ctx, `INSERT INTO buyer.capability_events(tenant_id,store_id,owner_id,session_id,action) VALUES($1,$2,$3,$4,'capability.revoked')`, tenantID, storeID, right.Scope.OwnerID, left.Scope.SessionID)
		requirePGCode(t, err, "23503", "event composite FK")
	})

	t.Run("callback rollback cancellation panic and GUC cleanup", func(t *testing.T) {
		capability := mustIssue(t, service, storeID)
		marker := errors.New("callback failed")
		withPinnedBuyerConnection(t, runtime, func() {
			err := buyer.WithScope(ctx, runtime, capability.Token, storeID, func(ctx context.Context, tx pgx.Tx, _ buyer.Scope) error {
				if _, err := tx.Exec(ctx, `CREATE TEMP TABLE buyer_callback_probe(value int); INSERT INTO buyer_callback_probe VALUES(1)`); err != nil {
					return err
				}
				return marker
			})
			if !errors.Is(err, marker) {
				t.Fatalf("callback error=%v", err)
			}
		})

		withPinnedBuyerConnection(t, runtime, func() {
			defer func() {
				if recovered := recover(); recovered != "buyer-panic" {
					t.Fatalf("panic=%v", recovered)
				}
			}()
			_ = buyer.WithScope(ctx, runtime, capability.Token, storeID, func(context.Context, pgx.Tx, buyer.Scope) error {
				panic("buyer-panic")
			})
		})

		withPinnedBuyerConnection(t, runtime, func() {
			cancelCtx, cancel := context.WithCancel(ctx)
			err := buyer.WithScope(cancelCtx, runtime, capability.Token, storeID, func(callbackCtx context.Context, _ pgx.Tx, _ buyer.Scope) error {
				cancel()
				<-callbackCtx.Done()
				return callbackCtx.Err()
			})
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("cancelled callback error=%v", err)
			}
		})
	})

	t.Run("revocation and state locks obey bounded ordering", func(t *testing.T) {
		capability := mustIssue(t, service, storeID)
		entered, release := make(chan struct{}), make(chan struct{})
		var releaseOnce sync.Once
		releaseTransaction := func() { releaseOnce.Do(func() { close(release) }) }
		t.Cleanup(releaseTransaction)
		resolveDone := make(chan error, 1)
		go func() {
			resolveDone <- buyer.WithScope(ctx, runtime, capability.Token, storeID, func(context.Context, pgx.Tx, buyer.Scope) error {
				close(entered)
				<-release
				return nil
			})
		}()
		select {
		case <-entered:
		case <-time.After(2 * time.Second):
			t.Fatal("authorized callback did not start")
		}
		revokeDone := make(chan error, 1)
		go func() { revokeDone <- service.Revoke(ctx, capability.Token, storeID) }()
		select {
		case err := <-revokeDone:
			t.Fatalf("revoke did not wait for authorized transaction: %v", err)
		case <-time.After(1200 * time.Millisecond):
		}
		releaseTransaction()
		if err := waitError(t, resolveDone); err != nil {
			t.Fatalf("authorized transaction finish: %v", err)
		}
		if err := waitError(t, revokeDone); err != nil {
			t.Fatalf("waited revoke: %v", err)
		}
		assertUnauthorized(t, runtime, capability.Token, storeID)

		expiring := mustIssue(t, service, storeID)
		mustExec(t, f.owner, `UPDATE buyer.capability_sessions SET created_at=clock_timestamp()-interval '1 second',expires_at=clock_timestamp()+interval '300 milliseconds' WHERE id=$1`, expiring.Scope.SessionID)
		holder, err := f.owner.Begin(ctx)
		if err != nil {
			t.Fatalf("begin expiry holder: %v", err)
		}
		t.Cleanup(func() { _ = holder.Rollback(context.Background()) })
		if _, err := holder.Exec(ctx, `SELECT id FROM buyer.capability_sessions WHERE id=$1 FOR UPDATE`, expiring.Scope.SessionID); err != nil {
			t.Fatalf("lock expiring session: %v", err)
		}
		called := false
		expiryDone := make(chan error, 1)
		applicationName := "buyer-expiry-wait-" + hex.EncodeToString(randomBytes(4))
		expiryRuntime, err := platform.OpenBuyerPool(ctx, withApplicationName(t, authorities.runtimeURL, applicationName))
		if err != nil {
			t.Fatalf("open expiry runtime: %v", err)
		}
		defer expiryRuntime.Close()
		go func() {
			expiryDone <- buyer.WithScope(ctx, expiryRuntime, expiring.Token, storeID, func(context.Context, pgx.Tx, buyer.Scope) error {
				called = true
				return nil
			})
		}()
		waitForDatabaseLock(t, f.owner, applicationName)
		time.Sleep(450 * time.Millisecond)
		_ = holder.Rollback(ctx)
		if err := waitError(t, expiryDone); !errors.Is(err, buyer.ErrUnauthorized) || called {
			t.Fatalf("expiry-after-lock err=%v called=%v", err, called)
		}

		mustExec(t, f.owner, `UPDATE control.stores SET active=true WHERE id=$1`, storeID)
		stateHolder, err := f.owner.Begin(ctx)
		if err != nil {
			t.Fatalf("begin state holder: %v", err)
		}
		t.Cleanup(func() { _ = stateHolder.Rollback(context.Background()) })
		if _, err := stateHolder.Exec(ctx, `UPDATE control.stores SET active=false WHERE id=$1`, storeID); err != nil {
			t.Fatalf("lock inactive store: %v", err)
		}
		issueDone := make(chan error, 1)
		go func() {
			_, err := service.IssueForTrustedStore(ctx, storeID)
			issueDone <- err
		}()
		select {
		case err := <-issueDone:
			t.Fatalf("issuance bypassed store state lock: %v", err)
		case <-time.After(250 * time.Millisecond):
		}
		if err := stateHolder.Commit(ctx); err != nil {
			t.Fatalf("commit inactive store: %v", err)
		}
		if err := waitError(t, issueDone); !errors.Is(err, buyer.ErrUnauthorized) {
			t.Fatalf("issue after inactive commit error=%v", err)
		}
		mustExec(t, f.owner, `UPDATE control.stores SET active=true WHERE id=$1`, storeID)

		for _, target := range []string{"tenant", "store", "owner"} {
			t.Run("resolve after locked "+target+" becomes inactive", func(t *testing.T) {
				assertInactiveLockRace(t, f, service, authorities, target)
			})
		}
	})
}

type buyerTestAuthorities struct {
	runtime, issuer, identity                    *pgxpool.Pool
	runtimeURL, issuerURL, identityURL, mixedURL string
	runtimeRole, issuerRole, identityRole        string
}

func openBuyerTestPools(t *testing.T, f *testFixture) buyerTestAuthorities {
	t.Helper()
	ctx := context.Background()
	suffix := hex.EncodeToString(randomBytes(6))
	runtimeRole, issuerRole := "foundation_buyer_runtime_"+suffix, "foundation_buyer_issuer_"+suffix
	identityRole, mixedRole := "foundation_buyer_identity_"+suffix, "foundation_buyer_mixed_"+suffix
	password := hex.EncodeToString(randomBytes(32))
	for role, memberships := range map[string]string{
		runtimeRole:  "commerce_buyer_runtime",
		issuerRole:   "commerce_buyer_issuer",
		identityRole: "commerce_identity",
		mixedRole:    "commerce_buyer_runtime,commerce_buyer_issuer",
	} {
		mustExec(t, f.owner, fmt.Sprintf(`CREATE ROLE %s LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE IN ROLE %s PASSWORD '%s'`, pgx.Identifier{role}.Sanitize(), memberships, password))
	}
	runtimeURL := roleURL(t, f.databaseURL, runtimeRole, password)
	issuerURL := roleURL(t, f.databaseURL, issuerRole, password)
	identityURL := roleURL(t, f.databaseURL, identityRole, password)
	mixedURL := roleURL(t, f.databaseURL, mixedRole, password)
	runtime, err := platform.OpenBuyerPool(ctx, runtimeURL)
	if err != nil {
		t.Fatalf("open buyer runtime: %v", err)
	}
	issuer, err := platform.OpenBuyerIssuerPool(ctx, issuerURL)
	if err != nil {
		runtime.Close()
		t.Fatalf("open buyer issuer: %v", err)
	}
	identity, err := platform.OpenIdentityPool(ctx, identityURL)
	if err != nil {
		runtime.Close()
		issuer.Close()
		t.Fatalf("open identity issuer: %v", err)
	}
	t.Cleanup(func() {
		runtime.Close()
		issuer.Close()
		identity.Close()
		mustExec(t, f.owner, fmt.Sprintf(`DROP ROLE %s,%s,%s,%s`, pgx.Identifier{runtimeRole}.Sanitize(), pgx.Identifier{issuerRole}.Sanitize(), pgx.Identifier{identityRole}.Sanitize(), pgx.Identifier{mixedRole}.Sanitize()))
	})
	return buyerTestAuthorities{runtime: runtime, issuer: issuer, identity: identity, runtimeURL: runtimeURL, issuerURL: issuerURL, identityURL: identityURL, mixedURL: mixedURL, runtimeRole: runtimeRole, issuerRole: issuerRole, identityRole: identityRole}
}

func seedBuyerStores(t *testing.T, f *testFixture) (string, string, string) {
	t.Helper()
	tenantID, storeID, otherStoreID := randomUUID(), randomUUID(), randomUUID()
	mustExec(t, f.owner, `INSERT INTO control.tenants(id,name) VALUES($1,'buyer-capability-tenant')`, tenantID)
	mustExec(t, f.owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'buyer-store','USD'),($1,$3,'other-buyer-store','TWD')`, tenantID, storeID, otherStoreID)
	return tenantID, storeID, otherStoreID
}

func assertBuyerWriterCanLockStore(t *testing.T, f *testFixture, tenantID, storeID string) {
	t.Helper()
	tx, err := f.owner.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin writer lock preflight: %v", err)
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(context.Background(), `SET LOCAL ROLE commerce_buyer_writer`); err != nil {
		t.Fatalf("assume buyer writer role: %v", err)
	}
	var resolvedTenant string
	if err := tx.QueryRow(context.Background(), `SELECT tenant_id::text FROM control.stores WHERE id=$1`, storeID).Scan(&resolvedTenant); err != nil {
		t.Fatalf("buyer writer locate store: %v", err)
	}
	var tenantActive, storeActive bool
	if err := tx.QueryRow(context.Background(), `SELECT active FROM control.tenants WHERE id=$1 FOR SHARE`, tenantID).Scan(&tenantActive); err != nil {
		t.Fatalf("buyer writer lock tenant: %v", err)
	}
	if err := tx.QueryRow(context.Background(), `SELECT active FROM control.stores WHERE tenant_id=$1 AND id=$2 FOR SHARE`, tenantID, storeID).Scan(&storeActive); err != nil {
		t.Fatalf("buyer writer lock store: %v", err)
	}
	if resolvedTenant != tenantID || !tenantActive || !storeActive {
		t.Fatal("buyer writer did not resolve the active trusted store")
	}
	_, err = tx.Exec(context.Background(), `UPDATE control.stores SET id=id WHERE tenant_id=$1 AND id=$2`, tenantID, storeID)
	requirePGCode(t, err, "42501", "buyer writer store mutation")
}

func assertInactiveLockRace(t *testing.T, f *testFixture, service *buyer.Service, authorities buyerTestAuthorities, target string) {
	t.Helper()
	tenantID, storeID, _ := seedBuyerStores(t, f)
	capability := mustIssue(t, service, storeID)
	var update, restore string
	var id string
	switch target {
	case "tenant":
		update, restore, id = `UPDATE control.tenants SET active=false WHERE id=$1`, `UPDATE control.tenants SET active=true WHERE id=$1`, tenantID
	case "store":
		update, restore, id = `UPDATE control.stores SET active=false WHERE id=$1`, `UPDATE control.stores SET active=true WHERE id=$1`, storeID
	case "owner":
		update, restore, id = `UPDATE buyer.owners SET active=false WHERE id=$1`, `UPDATE buyer.owners SET active=true WHERE id=$1`, capability.Scope.OwnerID
	default:
		t.Fatalf("unknown inactive target %q", target)
	}
	t.Cleanup(func() { mustExec(t, f.owner, restore, id) })
	holder, err := f.owner.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin inactive %s holder: %v", target, err)
	}
	t.Cleanup(func() { _ = holder.Rollback(context.Background()) })
	if _, err := holder.Exec(context.Background(), update, id); err != nil {
		t.Fatalf("lock inactive %s: %v", target, err)
	}
	applicationName := "buyer-inactive-" + target + "-" + hex.EncodeToString(randomBytes(3))
	runtime, err := platform.OpenBuyerPool(context.Background(), withApplicationName(t, authorities.runtimeURL, applicationName))
	if err != nil {
		t.Fatalf("open inactive %s runtime: %v", target, err)
	}
	defer runtime.Close()
	started, done := make(chan struct{}), make(chan error, 1)
	called := false
	go func() {
		close(started)
		done <- buyer.WithScope(context.Background(), runtime, capability.Token, storeID, func(context.Context, pgx.Tx, buyer.Scope) error {
			called = true
			return nil
		})
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatalf("inactive %s resolver did not start", target)
	}
	waitForDatabaseLock(t, f.owner, applicationName)
	if err := holder.Commit(context.Background()); err != nil {
		t.Fatalf("commit inactive %s: %v", target, err)
	}
	if err := waitError(t, done); !errors.Is(err, buyer.ErrUnauthorized) || called {
		t.Fatalf("inactive %s resolve err=%v called=%v", target, err, called)
	}
}

func withApplicationName(t *testing.T, databaseURL, applicationName string) string {
	t.Helper()
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatalf("parse application database url: %v", err)
	}
	query := parsed.Query()
	query.Set("application_name", applicationName)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func waitForDatabaseLock(t *testing.T, owner *pgxpool.Pool, applicationName string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var waiting bool
		if err := owner.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE application_name=$1 AND wait_event_type='Lock')`, applicationName).Scan(&waiting); err != nil {
			t.Fatalf("observe database lock: %v", err)
		}
		if waiting {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("database lock wait was not observed for %s", applicationName)
}

func waitError(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(6 * time.Second):
		t.Fatal("bounded buyer operation did not return")
		return nil
	}
}

func roleURL(t *testing.T, databaseURL, role, password string) string {
	t.Helper()
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatalf("parse test database url: %v", err)
	}
	parsed.User = url.UserPassword(role, password)
	return parsed.String()
}

func mustIssue(t *testing.T, service *buyer.Service, storeID string) buyer.Capability {
	t.Helper()
	capability, err := service.IssueForTrustedStore(context.Background(), storeID)
	if err != nil {
		t.Fatalf("issue capability: %v", err)
	}
	return capability
}

func assertScope(t *testing.T, pool *pgxpool.Pool, capability buyer.Capability, storeID string) {
	t.Helper()
	err := buyer.WithScope(context.Background(), pool, capability.Token, storeID, func(ctx context.Context, tx pgx.Tx, scope buyer.Scope) error {
		if scope != capability.Scope {
			t.Fatalf("scope=%+v want=%+v", scope, capability.Scope)
		}
		var tenant, store, owner, session, principal string
		if err := tx.QueryRow(ctx, `SELECT current_setting('app.tenant_id',true),current_setting('app.store_id',true),current_setting('app.buyer_id',true),current_setting('app.buyer_session_id',true),current_setting('app.principal_id',true)`).Scan(&tenant, &store, &owner, &session, &principal); err != nil {
			return err
		}
		if tenant != scope.TenantID || store != scope.StoreID || owner != scope.OwnerID || session != scope.SessionID || principal != "" {
			t.Fatalf("transaction scope leaked or mismatched: %q %q %q %q principal=%q", tenant, store, owner, session, principal)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("resolve capability: %v", err)
	}
}

func assertUnauthorized(t *testing.T, pool *pgxpool.Pool, token, storeID string) {
	t.Helper()
	called := false
	err := buyer.WithScope(context.Background(), pool, token, storeID, func(context.Context, pgx.Tx, buyer.Scope) error {
		called = true
		return nil
	})
	if !errors.Is(err, buyer.ErrUnauthorized) || called {
		t.Fatalf("authorization err=%v called=%v", err, called)
	}
}

func assertSQLDenied(t *testing.T, pool *pgxpool.Pool, query string, args ...any) {
	t.Helper()
	_, err := pool.Exec(context.Background(), query, args...)
	requirePGCode(t, err, "42501", "unauthorized SQL")
}

func requirePGCode(t *testing.T, err error, code, label string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != code {
		t.Fatalf("%s error=%v, want SQLSTATE %s", label, err, code)
	}
}

func assertPoolRejected(t *testing.T, label string, open func(context.Context, string) (*pgxpool.Pool, error), dsn string) {
	t.Helper()
	pool, err := open(context.Background(), dsn)
	if err == nil {
		pool.Close()
		t.Fatalf("%s pool accepted cross-authority login", label)
	}
}

func countRows(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return count
}

func mustExec(t *testing.T, pool *pgxpool.Pool, query string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), query, args...); err != nil {
		t.Fatalf("exec test setup: %v", err)
	}
}

func withPinnedBuyerConnection(t *testing.T, pool *pgxpool.Pool, run func()) {
	t.Helper()
	held := make([]*pgxpool.Conn, 0, 7)
	defer func() {
		for _, connection := range held {
			connection.Release()
		}
	}()
	for range 7 {
		connection, err := pool.Acquire(context.Background())
		if err != nil {
			t.Fatalf("pin buyer connection: %v", err)
		}
		held = append(held, connection)
	}
	run()
	connection, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatalf("reacquire scoped connection: %v", err)
	}
	defer connection.Release()
	var tenant, store, owner, session, principal string
	if err := connection.QueryRow(context.Background(), `SELECT coalesce(current_setting('app.tenant_id',true),''),coalesce(current_setting('app.store_id',true),''),coalesce(current_setting('app.buyer_id',true),''),coalesce(current_setting('app.buyer_session_id',true),''),coalesce(current_setting('app.principal_id',true),'')`).Scan(&tenant, &store, &owner, &session, &principal); err != nil {
		t.Fatalf("read cleared scope: %v", err)
	}
	if tenant != "" || store != "" || owner != "" || session != "" || principal != "" {
		t.Fatalf("scope survived connection reuse: %q %q %q %q %q", tenant, store, owner, session, principal)
	}
	var temporaryTable *string
	if err := connection.QueryRow(context.Background(), `SELECT to_regclass('pg_temp.buyer_callback_probe')::text`).Scan(&temporaryTable); err != nil {
		t.Fatalf("check callback rollback: %v", err)
	}
	if temporaryTable != nil {
		t.Fatal("callback transaction did not roll back temporary table")
	}
}
