// Purpose: exercise C1 through the real merchant entry before any order execution.
// Depends on: native lbu fixtures, runtime authentication, ops prepare/scope entries and pgx.
// Used by: OA1 CI admission tests; later C2 tests consume the same prepared authority.
// Invariants: current scope CAS, server-only private owner, canonical request facts,
// exact fifteen-minute deadline and rollback of every unfinished admission effect.
package foundation_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type oaAdmission struct {
	AttemptID    string          `json:"attempt_id"`
	ScopeID      string          `json:"scope_id"`
	Generation   int64           `json:"intent_generation"`
	BuyerOwnerID string          `json:"buyer_owner_id"`
	State        string          `json:"state"`
	Replay       bool            `json:"replay"`
	AdmittedAt   time.Time       `json:"admitted_at"`
	ExecuteBy    time.Time       `json:"execute_by"`
	RequestHash  string          `json:"request_hash"`
	FactsDigest  string          `json:"facts_digest"`
	Facts        json.RawMessage `json:"facts"`
}

func oaAdmissionScope(t *testing.T, e *lbuEnv) oaScopeReceipt {
	t.Helper()
	var raw []byte
	if err := e.p.f.runtime.QueryRow(context.Background(), `SELECT ops.read_order_attempt_scope($1,$2)`, tokenHash(e.token()), e.store()).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var out oaScopeReceipt
	if err := json.Unmarshal(raw, &out); err != nil || out.ScopeID == "" {
		t.Fatalf("scope positive sentinel: %+v err=%v", out, err)
	}
	return out
}

func oaPrepare(tx pgx.Tx, e *lbuEnv, scope oaScopeReceipt, key string, selection map[string]any) (oaAdmission, error) {
	raw, err := json.Marshal(selection)
	if err != nil {
		return oaAdmission{}, err
	}
	var result []byte
	// Calls ops.prepare_order_attempt_admission, order-attempt-v1 §2 C1; no execution yet.
	err = tx.QueryRow(context.Background(), `SELECT ops.prepare_order_attempt_admission($1,$2,$3,$4,$5,$6,$7::jsonb)`,
		tokenHash(e.token()), e.store(), scope.ScopeID, scope.Generation, scope.AttemptID, tokenHash(key), string(raw)).Scan(&result)
	var out oaAdmission
	if err == nil {
		err = json.Unmarshal(result, &out)
	}
	return out, err
}

func TestOrderAttemptAdmissionRealPrivateOwnerAndRollback(t *testing.T) {
	e := lbuNew(t)
	ctx := context.Background()
	scope := oaAdmissionScope(t, e)
	tx, err := e.p.f.runtime.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	prepared, err := oaPrepare(tx, e, scope, "oa-admit-"+randomUUID(), oaFactsSelection(e))
	if err != nil {
		t.Fatalf("real C1 prepare: %v", err)
	}
	if prepared.AttemptID == "" || prepared.BuyerOwnerID == "" || prepared.ScopeID != scope.ScopeID || prepared.Generation != scope.Generation+1 || prepared.Replay || prepared.State != "PENDING" || len(prepared.RequestHash) != 64 || len(prepared.FactsDigest) != 64 || len(prepared.Facts) == 0 || !prepared.AdmittedAt.Add(15*time.Minute).Equal(prepared.ExecuteBy) {
		t.Fatalf("server-bound admission metadata: %+v", prepared)
	}
	var raw []byte
	if err = tx.QueryRow(ctx, `SELECT ops.read_order_attempt_scope($1,$2)`, tokenHash(e.token()), e.store()).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var pending oaScopeReceipt
	if err = json.Unmarshal(raw, &pending); err != nil || pending.AttemptID == nil || *pending.AttemptID != prepared.AttemptID || pending.Generation != prepared.Generation {
		t.Fatalf("candidate must be visible inside the actual runtime transaction: %+v err=%v", pending, err)
	}
	// The native deferred constraint must refuse a caller that omits sealing/job linkage.
	err = tx.Commit(ctx)
	var pgerr *pgconn.PgError
	if !errors.As(err, &pgerr) || pgerr.Code != "23514" {
		t.Fatalf("unfinished real admission committed: %v", err)
	}
	var owners, attempts int
	err = e.p.f.owner.QueryRow(ctx, `SELECT (SELECT count(*) FROM buyer.owners WHERE id=$1),
	 (SELECT count(*) FROM ops.order_attempts WHERE attempt_id=$2)`, prepared.BuyerOwnerID, prepared.AttemptID).Scan(&owners, &attempts)
	if err != nil || owners != 0 || attempts != 0 {
		t.Fatalf("C1 failure leaked durable effects: owners=%d attempts=%d err=%v", owners, attempts, err)
	}
	if got := oaAdmissionScope(t, e); got.Generation != scope.Generation || got.AttemptID != nil {
		t.Fatalf("C1 rollback advanced the scope: %+v", got)
	}
}

func TestOrderAttemptAdmissionRequiresExactCASAndPlacementGrant(t *testing.T) {
	for _, mode := range []string{"scope", "generation", "terminal", "catalog"} {
		t.Run(mode, func(t *testing.T) {
			e := lbuNew(t)
			scope := oaAdmissionScope(t, e)
			want := "PT409"
			switch mode {
			case "scope":
				scope.ScopeID = randomUUID()
			case "generation":
				scope.Generation++
			case "terminal":
				id := randomUUID()
				scope.AttemptID = &id
			case "catalog":
				mustExec(t, e.p.f.owner, `DELETE FROM identity.store_grants WHERE store_id=$1 AND principal_id=$2 AND permission='catalog:read'`, e.store(), e.p.f.principalA)
				want = "PT403"
			}
			tx, err := e.p.f.runtime.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			_, err = oaPrepare(tx, e, scope, "oa-refuse-"+randomUUID(), oaFactsSelection(e))
			var pgerr *pgconn.PgError
			if !errors.As(err, &pgerr) || pgerr.Code != want {
				t.Fatalf("C1 %s refused incorrectly: %v", mode, err)
			}
		})
	}
}
