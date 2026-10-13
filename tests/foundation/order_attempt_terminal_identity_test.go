// Purpose: native M5/c/k acceptance of terminal replay identity and non-disclosure.
// Depends on: real C1, Executor/Abort, merchant sessions and existing native readback helpers.
// Used by: OA1 CI and the principal-ownership bypass mutation gate.
// Invariants: all-state keys remain principal-bound; relogin, later generations and
// retired scopes preserve the original terminal receipt without new domain effects.
package foundation_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/orderattempt"
)

func TestOrderAttemptTerminalIdentityReplay(t *testing.T) {
	for _, state := range []string{"COMMITTED", "ABORTED"} {
		t.Run(state, func(t *testing.T) {
			ctx := context.Background()
			e := lbuNew(t)
			svc, err := orderattempt.NewService(e.p.f.runtime, oaServiceKeys(t))
			if err != nil {
				t.Fatal(err)
			}
			req := oaServiceRequest(t, e)
			a, err := svc.Admit(ctx, e.token(), e.store(), req)
			if err != nil {
				t.Fatal("real C1 admission", err)
			}
			plan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
			executor := oaExecutorNew(t, e.p.f.runtime, oaServiceKeys(t))
			var terminal orderattempt.ExecutionReceipt
			if state == "COMMITTED" {
				terminal, err = executor.Execute(ctx, e.token(), e.store(), a.AttemptID, a.Generation)
				if err != nil {
					t.Fatal("actual C2 commit", err)
				}
				oaExecutorCommitted(t, e, a, plan, terminal, 5)
			} else {
				terminal, err = svc.Abort(ctx, e.token(), e.store(), a.AttemptID, a.Generation, "PLAN_UNAVAILABLE")
				if err != nil {
					t.Fatal("actual zero-effects abort", err)
				}
				oaExecutorAborted(t, e, a, terminal, "PLAN_UNAVAILABLE")
			}
			var original []byte
			if err = e.p.f.owner.QueryRow(ctx, `SELECT to_jsonb(a) FROM ops.order_attempts a WHERE attempt_id=$1`, a.AttemptID).Scan(&original); err != nil {
				t.Fatal(err)
			}
			// The foreign principal has current read authority in this exact store.
			// Supply the victim's exact body/CAS/key so only identity distinguishes it.
			_, foreignToken := lcPrincipal(t, e.p.f, e.tenant(), []string{e.store()}, "store:read", "orders:read")
			key := sha256.Sum256([]byte(req.TransportKey))
			var raw []byte
			err = e.p.f.runtime.QueryRow(ctx, `SELECT ops.prepare_order_attempt_admission($1::bytea,$2::uuid,$3::uuid,$4::bigint,$5::uuid,$6::bytea,$7::jsonb,'synthetic_v1'::text)`,
				tokenHash(foreignToken), e.store(), req.ExpectedScopeID, req.ExpectedGeneration, req.PreviousAttemptID, key[:], string(req.Selection)).Scan(&raw)
			var pgerr *pgconn.PgError
			if !errors.As(err, &pgerr) || pgerr.Code != "PT409" || pgerr.Message != "idempotency_conflict" || pgerr.Detail != "" || pgerr.Hint != "" || len(raw) != 0 {
				t.Fatal("foreign terminal key replay must return only the exact non-disclosing conflict")
			}
			for _, private := range []string{a.AttemptID, a.ScopeID, plan.BuyerOwnerID, "Synthetic Buyer", "synthetic@example.invalid", "0912345678", "Synthetic address"} {
				if strings.Contains(pgerr.Error()+pgerr.Where+pgerr.InternalQuery, private) {
					t.Fatal("foreign replay error disclosed original identity or contact")
				}
			}
			denied, err := svc.Admit(ctx, foreignToken, e.store(), req)
			var coded *orderattempt.AdmissionError
			if !errors.As(err, &coded) || coded.Code() != "idempotency_conflict" || coded.SQLState() != "PT409" || coded.OutcomeUnknown() || !reflect.DeepEqual(denied, orderattempt.AdmissionReceipt{}) {
				t.Fatal("foreign Service replay disclosed any receipt or changed conflict semantics")
			}
			raw = nil
			err = e.p.f.runtime.QueryRow(ctx, `SELECT ops.lock_order_attempt($1,$2,$3,$4)`, tokenHash(foreignToken), e.store(), a.AttemptID, a.Generation).Scan(&raw)
			oaFenceCode(t, raw, err, "PT404")
			if !errors.As(err, &pgerr) || pgerr.Message != "attempt_not_found" {
				t.Fatal("foreign terminal fence must use the fixed non-disclosing refusal")
			}
			foreignReceipt, err := executor.Execute(ctx, foreignToken, e.store(), a.AttemptID, a.Generation)
			if !errors.As(err, &coded) || coded.Code() != "not_found" || coded.SQLState() != "PT404" || !reflect.DeepEqual(foreignReceipt, orderattempt.ExecutionReceipt{}) {
				t.Fatal("foreign executor disclosed a terminal receipt")
			}
			// A new session for the original principal is sufficient; session identity
			// must not substitute for the durable principal/store authority.
			relogin := lcToken(t, e.p.f, plan.PrincipalID)
			mustExec(t, e.p.f.owner, `UPDATE identity.sessions SET revoked_at=clock_timestamp() WHERE token_hash=$1`, tokenHash(e.token()))
			checkReplay := func() {
				t.Helper()
				replay, err := svc.Admit(ctx, relogin, e.store(), req)
				if err != nil || !replay.Replay || replay.State != state || replay.AttemptID != a.AttemptID || replay.ScopeID != a.ScopeID || replay.Generation != a.Generation || !bytes.Equal(replay.Receipt, terminal.Receipt) {
					t.Fatal("same-principal replay lost the exact original terminal receipt")
				}
				again, err := executor.Execute(ctx, relogin, e.store(), a.AttemptID, a.Generation)
				if err != nil {
					t.Fatal("terminal executor replay", err)
				}
				oaExecutorSameReceipt(t, terminal, again)
			}
			checkReplay()
			// Explicit fresh CAS advances the active scope, never the old request.
			next := req
			next.ExpectedGeneration, next.PreviousAttemptID = a.Generation, &a.AttemptID
			next.TransportKey = "oa-terminal-next-" + randomUUID()
			newer, err := svc.Admit(ctx, relogin, e.store(), next)
			if err != nil || newer.AttemptID == a.AttemptID || newer.ScopeID != a.ScopeID || newer.Generation != a.Generation+1 || newer.Replay {
				t.Fatal("explicit next-generation admission failed", err)
			}
			if _, err = svc.Abort(ctx, relogin, e.store(), newer.AttemptID, newer.Generation, "PLAN_UNAVAILABLE"); err != nil {
				t.Fatal(err)
			}
			checkReplay()
			// Synthetic lifecycle setup takes the actual principal lock first; only
			// already-terminal scopes may retire. No terminal order is fabricated.
			lifecycle, err := e.p.f.owner.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer lifecycle.Rollback(ctx)
			oaStateExec(t, lifecycle, `SELECT 1 FROM identity.principals WHERE id=$1 FOR UPDATE`, plan.PrincipalID)
			oaStateExec(t, lifecycle, `UPDATE ops.order_attempt_scopes SET retired_at=clock_timestamp() WHERE scope_id=$1`, a.ScopeID)
			if err = lifecycle.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			var freshRaw []byte
			if err = e.p.f.runtime.QueryRow(ctx, `SELECT ops.read_order_attempt_scope($1,$2)`, tokenHash(relogin), e.store()).Scan(&freshRaw); err != nil {
				t.Fatal(err)
			}
			var fresh oaScopeReceipt
			if json.Unmarshal(freshRaw, &fresh) != nil || fresh.ScopeID == "" || fresh.ScopeID == a.ScopeID || fresh.Generation != 0 {
				t.Fatal("retirement did not produce a fresh zero-generation scope")
			}
			checkReplay()
			var retained []byte
			var jobs int
			if err = e.p.f.owner.QueryRow(ctx, `SELECT to_jsonb(a),(SELECT count(*) FROM river.river_job j WHERE j.kind='order_attempt_v1' AND j.args->>'attempt_id'=a.attempt_id::text) FROM ops.order_attempts a WHERE a.attempt_id=$1`, a.AttemptID).Scan(&retained, &jobs); err != nil || !bytes.Equal(original, retained) || jobs != 1 {
				t.Fatal("replay changed old durable identity/receipt or added an execution job", err)
			}
			if state == "COMMITTED" {
				oaExecutorCommitted(t, e, a, plan, terminal, 5)
			} else {
				oaExecutorAborted(t, e, a, terminal, "PLAN_UNAVAILABLE")
			}
		})
	}
}
