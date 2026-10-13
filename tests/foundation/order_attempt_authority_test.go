// Purpose: real-login authority acceptance for order-attempt-v1 C1 scope discovery.
// Depends: migrated PG, identity sessions/grants, runtime login and ops entry definers.
// Used: OA1 CI before admission/execution callers are enabled.
// Invariants: server principal/store scope survives re-login; spoofed GUCs and
// invalid sessions never create an intent or grant internal domain-helper access.
package foundation_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

type oaScopeReceipt struct {
	ScopeID string `json:"scope_id"`
	Generation int64 `json:"intent_generation"`
	AttemptID *string `json:"attempt_id"`
}

func TestOrderAttemptAuthorityScopeSurvivesRelogin(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	principal, token := lcPrincipal(t, f, f.tenantA, []string{f.storeA1, f.storeA2}, "store:read", "orders:read")
	read := func(token, store string) oaScopeReceipt {
		t.Helper()
		var raw []byte
		if err := f.runtime.QueryRow(ctx, `SELECT ops.read_order_attempt_scope($1,$2)`, tokenHash(token), store).Scan(&raw); err != nil {
			t.Fatalf("real runtime scope read: %v", err)
		}
		var out oaScopeReceipt
		if err := json.Unmarshal(raw, &out); err != nil || out.ScopeID == "" || out.Generation != 0 || out.AttemptID != nil {
			t.Fatalf("fresh scope positive sentinel: %+v err=%v", out, err)
		}
		return out
	}
	first := read(token, f.storeA1)
	otherStore := read(token, f.storeA2)
	if first.ScopeID == otherStore.ScopeID {
		t.Fatal("same principal's different stores share a scope")
	}
	newToken := "oa-relogin-" + randomUUID()
	tx, err := f.owner.Begin(ctx)
	if err != nil { t.Fatal(err) }
	defer tx.Rollback(ctx)
	if err = insertSession(ctx, tx, newToken, principal, "merchant", time.Now().Add(time.Hour), nil); err != nil { t.Fatal(err) }
	if err = tx.Commit(ctx); err != nil { t.Fatal(err) }
	if got := read(newToken, f.storeA1); got.ScopeID != first.ScopeID {
		t.Fatalf("new session changed server scope: first=%s new=%s", first.ScopeID, got.ScopeID)
	}
	var scopes, attempts int
	err = f.owner.QueryRow(ctx, `SELECT count(*),(SELECT count(*) FROM ops.order_attempts WHERE scope_id=$1)
	 FROM ops.order_attempt_scopes WHERE scope_id=$1`, first.ScopeID).Scan(&scopes, &attempts)
	if err != nil || scopes != 1 || attempts != 0 { t.Fatalf("scope read admitted intent: scopes=%d attempts=%d err=%v", scopes, attempts, err) }
}

func TestOrderAttemptAuthorityRejectsSpoofedContext(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	for _, token := range []string{"revoked", "expired", "buyer"} {
		t.Run(token, func(t *testing.T) {
			tx, err := f.runtime.Begin(ctx)
			if err != nil { t.Fatal(err) }
			defer tx.Rollback(ctx)
			if _, err = tx.Exec(ctx, `SELECT set_config('app.tenant_id',$1,true),set_config('app.store_id',$2,true),set_config('app.principal_id',$3,true)`, f.tenantA, f.storeA1, f.principalA); err != nil { t.Fatal(err) }
			_, err = tx.Exec(ctx, `SELECT ops.read_order_attempt_scope($1,$2)`, tokenHash(f.tokens[token]), f.storeA1)
			var pgerr *pgconn.PgError
			if !errors.As(err, &pgerr) || pgerr.Code != "PT401" { t.Fatalf("spoofed context bypassed session: %v", err) }
		})
	}
}

func TestOrderAttemptAuthorityInternalAccessorIsNotRuntimeAPI(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	var exists bool
	if err := f.owner.QueryRow(ctx, `SELECT to_regprocedure('ops.order_attempt_admission_context(uuid,jsonb)') IS NOT NULL`).Scan(&exists); err != nil || !exists {
		t.Fatalf("internal accessor positive existence sentinel: exists=%v err=%v", exists, err)
	}
	_, err := f.runtime.Exec(ctx, `SELECT ops.order_attempt_admission_context($1,'{}'::jsonb)`, randomUUID())
	var pgerr *pgconn.PgError
	if !errors.As(err, &pgerr) || pgerr.Code != "42501" { t.Fatalf("runtime reached internal accessor: %v", err) }
}
