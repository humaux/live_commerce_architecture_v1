// Purpose: REAL_PG tests FIRST for order-attempt-v1 A2 durable plan-key retirement.
// Depends on: real Service.Admit/Executor, registry/retirement definers, native logins,
// River InsertTx and fixture-owned PG trigger barriers; no provider or product hooks.
// Used by: parent-owned OA1 CI and A2-M1 mutation acceptance.
// Invariants: key row before C1 locks, permanent markers, exact replay, global
// reference refusal without disclosure; local key skew cannot authorize settlement.
package foundation_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"livecommerce/internal/orderattempt"
)

func oaKeyRing(t *testing.T, version string) *orderattempt.PlanKeys {
	t.Helper()
	k, err := orderattempt.NewPlanKeys(version, map[string][]byte{version: bytes.Repeat([]byte{7}, 32)})
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func oaKeyAdmit(t *testing.T, e *lbuEnv, version string) (*orderattempt.Service, orderattempt.AdmissionRequest, orderattempt.AdmissionReceipt) {
	t.Helper()
	svc, err := orderattempt.NewService(e.p.f.runtime, oaKeyRing(t, version))
	if err != nil {
		t.Fatal(err)
	}
	req := oaServiceRequest(t, e)
	a, err := svc.Admit(context.Background(), e.token(), e.store(), req)
	if err != nil || a.State != "PENDING" || a.AttemptID == "" {
		t.Fatal("real C1 control before retirement seam", err)
	}
	var exact bool
	err = e.p.f.owner.QueryRow(context.Background(), `SELECT admission_finished AND plan_key_version=$2
	 AND plan_ciphertext IS NOT NULL AND state='PENDING' AND order_id IS NULL
	 AND EXISTS(SELECT 1 FROM river.river_job j WHERE j.id=a.execution_job_id
	 AND j.kind='order_attempt_v1' AND j.args->>'attempt_id'=a.attempt_id::text)
	 FROM ops.order_attempts a WHERE attempt_id=$1`, a.AttemptID, version).Scan(&exact)
	if err != nil || !exact {
		t.Fatal("native C1 custody/job sentinel", err)
	}
	return svc, req, a
}

func oaKeyAdmin(t *testing.T, f *testFixture) *pgxpool.Pool {
	t.Helper()
	// Literal deployed login, provisioned only in the isolated foundation database.
	login := "lc_order_attempt_key_admin"
	var exists bool
	if err := f.owner.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=$1)`, login).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	// Never alter or drop a provisioned/shared login owned by another task.
	if exists {
		login = "a2_admin_test_" + hex.EncodeToString(randomBytes(8))
	}
	identifier := pgx.Identifier{login}.Sanitize()
	password := hex.EncodeToString(randomBytes(24))
	mustExec(t, f.owner, `CREATE ROLE `+identifier+` LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION PASSWORD '`+password+`'`)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if _, err := f.owner.Exec(ctx, `DROP ROLE `+identifier); err != nil {
			t.Error("bounded own admin-login cleanup", err)
		}
	})
	mustExec(t, f.owner, `GRANT commerce_order_attempt_key_admin TO `+identifier+` WITH INHERIT TRUE, SET FALSE`)
	p, err := pgxpool.New(context.Background(), roleURL(t, f.databaseURL, login, password))
	if err != nil {
		t.Fatal("open actual key-admin login", err)
	}
	t.Cleanup(p.Close)
	var actual string
	if err = p.QueryRow(context.Background(), `SELECT session_user`).Scan(&actual); err != nil || actual != login {
		t.Fatal("actual admin-login sentinel", err)
	}
	return p
}

func oaKeyRetire(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, version string) (string, error) {
	var result string
	// Calls ops.retire_order_attempt_plan_key, order-attempt-v1 amendment A2.
	err := q.QueryRow(ctx, `SELECT ops.retire_order_attempt_plan_key($1::text)`, version).Scan(&result)
	return result, err
}

func oaKeyError(t *testing.T, result string, err error, code, message string) {
	t.Helper()
	var p *pgconn.PgError
	if !errors.As(err, &p) || p.Code != code || p.Message != message || p.Detail != "" || p.Hint != "" || result != "" {
		t.Fatalf("expected fixed non-disclosing %s/%s refusal", code, message)
	}
}

func oaKeySnapshot(t *testing.T, e *lbuEnv, scope string) []byte {
	t.Helper()
	var raw []byte
	err := e.p.f.owner.QueryRow(context.Background(), `SELECT jsonb_build_object(
	 'scope',(SELECT to_jsonb(s) FROM ops.order_attempt_scopes s WHERE scope_id=$1),
	 'attempts',(SELECT coalesce(jsonb_agg(to_jsonb(a) ORDER BY attempt_id),'[]'::jsonb) FROM ops.order_attempts a WHERE scope_id=$1),
	 'jobs',(SELECT coalesce(jsonb_agg(to_jsonb(j) ORDER BY id),'[]'::jsonb) FROM river.river_job j
	 WHERE kind='order_attempt_v1' AND args->>'attempt_id' IN (SELECT attempt_id::text FROM ops.order_attempts WHERE scope_id=$1)))`, scope).Scan(&raw)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestOrderAttemptKeyRetirementReferencedAndUnmarkedLoss(t *testing.T) {
	e := lbuNew(t)
	version := "a2-" + randomUUID()
	svc, req, a := oaKeyAdmit(t, e, version)
	admin := oaKeyAdmin(t, e.p.f)
	before := oaKeySnapshot(t, e, a.ScopeID)
	result, err := oaKeyRetire(context.Background(), admin, version)
	oaKeyError(t, result, err, "PT409", "plan_key_referenced")
	if !bytes.Equal(before, oaKeySnapshot(t, e, a.ScopeID)) {
		t.Fatal("refused retirement changed admitted authority")
	}
	var active bool
	if err = e.p.f.owner.QueryRow(context.Background(), `SELECT retired_at IS NULL FROM ops.order_attempt_plan_key_versions WHERE key_version=$1`, version).Scan(&active); err != nil || !active {
		t.Fatal("refusal retired a referenced version", err)
	}
	// A missing process-local key is not loss authority, even while the registry
	// row is locked. It cannot release the reference or permit retirement.
	missing := oaKeyRing(t, "a2-other-"+randomUUID())
	executor := oaExecutorNew(t, miPool(t, e.p.f, "commerce_claims_worker"), missing)
	keyHolder, err := e.p.f.owner.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer keyHolder.Rollback(context.Background())
	oaStateExec(t, keyHolder, `SELECT key_version FROM ops.order_attempt_plan_key_versions WHERE key_version=$1 FOR UPDATE`, version)
	lost, err := executor.Execute(context.Background(), "", "", a.AttemptID, a.Generation)
	var retryable *orderattempt.AdmissionError
	if !errors.As(err, &retryable) || retryable.Code() != "order_attempt_retryable" || retryable.OutcomeUnknown() || lost.State != "" || len(lost.Receipt) != 0 {
		t.Fatal("local ring skew cannot authorize permanent loss", err)
	}
	oaExecutorNoEffects(t, e, a)
	if !bytes.Equal(before, oaKeySnapshot(t, e, a.ScopeID)) {
		t.Fatal("local ring skew changed admitted authority or its job")
	}
	if err = keyHolder.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	result, err = oaKeyRetire(context.Background(), admin, version)
	oaKeyError(t, result, err, "PT409", "plan_key_referenced")
	// Another process still has the admitted key. Completing the original C2
	// releases the PENDING reference; this is not a synthetic loss declaration.
	plan, _ := oaMaterialPlan(t, e, oaKeyRing(t, version), a.AttemptID)
	lost, err = oaExecutorNew(t, miPool(t, e.p.f, "commerce_claims_worker"), oaKeyRing(t, version)).Execute(context.Background(), "", "", a.AttemptID, a.Generation)
	if err != nil {
		t.Fatal("retained original key must complete the admitted attempt", err)
	}
	oaExecutorCommitted(t, e, a, plan, lost, 5)
	result, err = oaKeyRetire(context.Background(), admin, version)
	if err != nil || result != "retired" {
		t.Fatal("terminal C2 must allow retirement after the last reference", err)
	}
	var marker []byte
	if err = e.p.f.owner.QueryRow(context.Background(), `SELECT to_jsonb(v) FROM ops.order_attempt_plan_key_versions v WHERE key_version=$1 AND retired_at IS NOT NULL`, version).Scan(&marker); err != nil {
		t.Fatal(err)
	}
	result, err = oaKeyRetire(context.Background(), admin, version)
	if err != nil || result != "retired" {
		t.Fatal("retirement must be idempotent", err)
	}
	var again []byte
	if err = e.p.f.owner.QueryRow(context.Background(), `SELECT to_jsonb(v) FROM ops.order_attempt_plan_key_versions v WHERE key_version=$1`, version).Scan(&again); err != nil || !bytes.Equal(marker, again) {
		t.Fatal("retirement marker was replaced", err)
	}
	replay, err := svc.Admit(context.Background(), e.token(), e.store(), req)
	if err != nil || !replay.Replay || replay.AttemptID != a.AttemptID || !bytes.Equal(replay.Receipt, lost.Receipt) {
		t.Fatal("retired-key exact terminal admission replay", err)
	}
	terminal, err := executor.Execute(context.Background(), "", "", a.AttemptID, a.Generation)
	if err != nil {
		t.Fatal(err)
	}
	oaExecutorSameReceipt(t, lost, terminal)
	// Retired scope exact replay also precedes every fresh-version gate.
	mustExec(t, e.p.f.owner, `UPDATE ops.order_attempt_scopes SET retired_at=clock_timestamp() WHERE scope_id=$1`, a.ScopeID)
	replay, err = svc.Admit(context.Background(), e.token(), e.store(), req)
	if err != nil || !replay.Replay || !bytes.Equal(replay.Receipt, lost.Receipt) {
		t.Fatal("retired-scope/key replay lost terminal identity", err)
	}
	before = oaKeySnapshot(t, e, a.ScopeID)
	newReq := oaServiceRequest(t, e)
	freshBefore := oaKeySnapshot(t, e, newReq.ExpectedScopeID)
	var raw []byte
	err = e.p.f.runtime.QueryRow(context.Background(), `SELECT ops.prepare_order_attempt_admission($1,$2,$3,$4,$5,$6,$7::jsonb,$8::text)`,
		tokenHash(e.token()), e.store(), newReq.ExpectedScopeID, newReq.ExpectedGeneration, newReq.PreviousAttemptID, tokenHash(newReq.TransportKey), string(newReq.Selection), version).Scan(&raw)
	oaKeyError(t, string(raw), err, "PT409", "plan_key_retired")
	if !bytes.Equal(before, oaKeySnapshot(t, e, a.ScopeID)) {
		t.Fatal("stale process changed original terminal admission")
	}
	if !bytes.Equal(freshBefore, oaKeySnapshot(t, e, newReq.ExpectedScopeID)) {
		t.Fatal("retired key advanced fresh scope or inserted a job")
	}
	if count := miCount(t, e.p.f.owner, `SELECT count(*) FROM ops.order_attempts WHERE transport_key_digest=$1`, tokenHash(newReq.TransportKey)); count != 0 {
		t.Fatal("retired key created a fresh attempt")
	}
}

func TestOrderAttemptKeyRetirementGlobalReferences(t *testing.T) {
	e := lbuNew(t)
	version := "a2-cross-" + randomUUID()
	_, req, a := oaKeyAdmit(t, e, version)
	admin := oaKeyAdmin(t, e.p.f)
	// Deliberately set unrelated caller GUCs: visibility derives from session_user.
	conn, err := admin.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	if _, err = conn.Exec(context.Background(), `SELECT set_config('app.tenant_id',$1,false),set_config('app.store_id',$2,false)`, e.p.f.tenantB, e.p.f.storeB); err != nil {
		t.Fatal(err)
	}
	before := oaKeySnapshot(t, e, a.ScopeID)
	result, err := oaKeyRetire(context.Background(), conn, version)
	oaKeyError(t, result, err, "PT409", "plan_key_referenced")
	if !bytes.Equal(before, oaKeySnapshot(t, e, a.ScopeID)) {
		t.Fatal("cross-scope reference check changed attempt")
	}
	_, foreign := lcPrincipal(t, e.p.f, e.tenant(), []string{e.store()}, "store:read", "orders:read")
	var raw []byte
	err = e.p.f.runtime.QueryRow(context.Background(), `SELECT ops.prepare_order_attempt_admission($1,$2,$3,$4,$5,$6,$7::jsonb,$8::text)`,
		tokenHash(foreign), e.store(), req.ExpectedScopeID, req.ExpectedGeneration, req.PreviousAttemptID, tokenHash(req.TransportKey), string(req.Selection), version).Scan(&raw)
	oaKeyError(t, string(raw), err, "PT409", "idempotency_conflict")
	if !bytes.Equal(before, oaKeySnapshot(t, e, a.ScopeID)) {
		t.Fatal("foreign principal changed original admission")
	}
}

func oaKeyWaitBlocked(t *testing.T, f *testFixture, ctx context.Context, application string, holder uint32) uint32 {
	t.Helper()
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		var pid uint32
		err := f.owner.QueryRow(ctx, `SELECT coalesce(max(pid),0) FROM pg_stat_activity
		 WHERE application_name=$1 AND wait_event_type='Lock' AND $2::int=ANY(pg_blocking_pids(pid))`, application, int32(holder)).Scan(&pid)
		if err != nil {
			t.Fatal("native positive lock sentinel", err)
		}
		if pid != 0 {
			return pid
		}
		select {
		case <-ctx.Done():
			t.Fatal("no native blocking sentinel before deadline")
		case <-tick.C:
		}
	}
}

func oaKeyBarrier(t *testing.T, f *testFixture, version, barrier string) func() {
	t.Helper()
	name := "a2_key_" + hex.EncodeToString(randomBytes(8))
	fn := "ops." + pgx.Identifier{name}.Sanitize()
	trigger := pgx.Identifier{name}.Sanitize()
	ddl := fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
	 BEGIN PERFORM pg_advisory_xact_lock(hashtextextended('%s',0)); RETURN NEW; END $$`, fn, barrier)
	mustExec(t, f.owner, ddl)
	cleaned := false
	cleanup := func() {
		if cleaned {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		var exists bool
		if err := f.owner.QueryRow(ctx, `SELECT to_regclass('ops.order_attempt_plan_key_versions') IS NOT NULL`).Scan(&exists); err != nil {
			t.Error("own barrier relation cleanup sentinel", err)
			return
		}
		if exists {
			if _, err := f.owner.Exec(ctx, "DROP TRIGGER IF EXISTS "+trigger+" ON ops.order_attempt_plan_key_versions"); err != nil {
				t.Error("own barrier trigger cleanup", err)
				return
			}
		}
		if _, err := f.owner.Exec(ctx, "DROP FUNCTION IF EXISTS "+fn+"()"); err != nil {
			t.Error("own barrier function cleanup", err)
			return
		}
		cleaned = true
	}
	t.Cleanup(cleanup)
	mustExec(t, f.owner, "REVOKE ALL ON FUNCTION "+fn+"() FROM PUBLIC")
	mustExec(t, f.owner, fmt.Sprintf("CREATE TRIGGER %s AFTER INSERT ON ops.order_attempt_plan_key_versions FOR EACH ROW WHEN (NEW.key_version='%s') EXECUTE FUNCTION %s()", trigger, version, fn))
	return cleanup
}

func TestOrderAttemptKeyRetirementNativeInsertRace(t *testing.T) {
	for _, retireFirst := range []bool{false, true} {
		name := "admission-wins"
		if retireFirst {
			name = "retirement-wins"
		}
		t.Run(name, func(t *testing.T) {
			e := lbuNew(t)
			// Positive native C1 control, before any missing A2 seam can fail setup.
			oaKeyAdmit(t, e, "a2-control-"+randomUUID())
			admin := oaKeyAdmin(t, e.p.f)
			version, barrier := "a2-race-"+randomUUID(), "a2.barrier|"+randomUUID()
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			holder, err := e.p.f.owner.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer holder.Rollback(context.Background())
			oaStateExec(t, holder, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, barrier)
			oaKeyBarrier(t, e.p.f, version, barrier)
			runtimeApp, adminApp := "a2-runtime-"+randomUUID(), "a2-admin-"+randomUUID()
			cfg, err := pgxpool.ParseConfig(miRole(t, e.p.f, "commerce_runtime"))
			if err != nil {
				t.Fatal(err)
			}
			cfg.MaxConns = 1
			cfg.ConnConfig.RuntimeParams["application_name"] = runtimeApp
			runtime, err := pgxpool.NewWithConfig(ctx, cfg)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(runtime.Close)
			adminConn, err := admin.Acquire(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer adminConn.Release()
			if _, err = adminConn.Exec(ctx, `SELECT set_config('application_name',$1,false)`, adminApp); err != nil {
				t.Fatal(err)
			}
			var runtimePID uint32
			svc, err := orderattempt.NewService(runtime, oaKeyRing(t, version))
			if err != nil {
				t.Fatal(err)
			}
			req := oaServiceRequest(t, e)
			// The control settled no C2, so use another explicit principal/scope for the race.
			_, raceToken := lcPrincipal(t, e.p.f, e.tenant(), []string{e.store()}, "store:read", "orders:read", "catalog:read", "inventory:reserve")
			var scopeRaw []byte
			if err = runtime.QueryRow(ctx, `SELECT ops.read_order_attempt_scope($1,$2)`, tokenHash(raceToken), e.store()).Scan(&scopeRaw); err != nil {
				t.Fatal(err)
			}
			var scope oaScopeReceipt
			if err = json.Unmarshal(scopeRaw, &scope); err != nil {
				t.Fatal(err)
			}
			req.ExpectedScopeID, req.ExpectedGeneration, req.PreviousAttemptID = scope.ScopeID, scope.Generation, scope.AttemptID
			type admissionOutcome struct {
				a   orderattempt.AdmissionReceipt
				err error
			}
			type retirementOutcome struct {
				state string
				err   error
			}
			admitted := make(chan admissionOutcome, 1)
			retired := make(chan retirementOutcome, 1)
			var scopeHolder pgx.Tx
			admissionStarted, retirementStarted, admissionReceived, retirementReceived := false, false, false, false
			defer func() {
				cancel()
				cleanupCtx, stop := context.WithTimeout(context.Background(), 3*time.Second)
				defer stop()
				_ = holder.Rollback(cleanupCtx)
				if scopeHolder != nil {
					_ = scopeHolder.Rollback(cleanupCtx)
				}
				if admissionStarted && !admissionReceived {
					select {
					case <-admitted:
					case <-cleanupCtx.Done():
						t.Error("bounded admission goroutine cleanup")
					}
				}
				if retirementStarted && !retirementReceived {
					select {
					case <-retired:
					case <-cleanupCtx.Done():
						t.Error("bounded retirement goroutine cleanup")
					}
				}
			}()
			startAdmission := func() {
				admissionStarted = true
				go func() {
					a, callErr := svc.Admit(ctx, raceToken, e.store(), req)
					admitted <- admissionOutcome{a, callErr}
				}()
			}
			startRetirement := func() {
				retirementStarted = true
				go func() {
					state, callErr := oaKeyRetire(ctx, adminConn, version)
					retired <- retirementOutcome{state, callErr}
				}()
			}
			if retireFirst {
				// A held lower-order scope row must not become C1's first blocker.
				scopeHolder, err = e.p.f.owner.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				oaStateExec(t, scopeHolder, `SELECT scope_id FROM ops.order_attempt_scopes WHERE scope_id=$1 FOR UPDATE`, scope.ScopeID)
				startRetirement()
				oaKeyWaitBlocked(t, e.p.f, ctx, adminApp, holder.Conn().PgConn().PID())
				startAdmission()
				oaKeyWaitBlocked(t, e.p.f, ctx, runtimeApp, adminConn.Conn().PgConn().PID())
			} else {
				startAdmission()
				runtimePID = oaKeyWaitBlocked(t, e.p.f, ctx, runtimeApp, holder.Conn().PgConn().PID())
				startRetirement()
				oaKeyWaitBlocked(t, e.p.f, ctx, adminApp, runtimePID)
			}
			if err = holder.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			var ar admissionOutcome
			var rr retirementOutcome
			select {
			case ar = <-admitted:
				admissionReceived = true
			case <-ctx.Done():
				t.Fatal("bounded native admission race completion")
			}
			select {
			case rr = <-retired:
				retirementReceived = true
			case <-ctx.Done():
				t.Fatal("bounded native retirement race completion")
			}
			if retireFirst {
				if rr.err != nil || rr.state != "retired" {
					t.Fatal("admin must persist winning unknown-version marker", rr.err)
				}
				var coded interface{ Code() string }
				if !errors.As(ar.err, &coded) || coded.Code() != "plan_key_retired" || ar.a.AttemptID != "" {
					t.Fatal("stale process was admitted after retirement")
				}
				if count := miCount(t, e.p.f.owner, `SELECT count(*) FROM ops.order_attempts WHERE scope_id=$1`, scope.ScopeID); count != 0 {
					t.Fatal("retirement winner leaked admission effects")
				}
			} else {
				if ar.err != nil || ar.a.State != "PENDING" {
					t.Fatal("native admission winner must commit reference", ar.err)
				}
				oaKeyError(t, rr.state, rr.err, "PT409", "plan_key_referenced")
				var active bool
				if err = e.p.f.owner.QueryRow(ctx, `SELECT retired_at IS NULL FROM ops.order_attempt_plan_key_versions WHERE key_version=$1`, version).Scan(&active); err != nil || !active {
					t.Fatal("retirement crossed C1 committed reference", err)
				}
			}
		})
	}
}

func TestOrderAttemptKeyRetirementFinishVersionBound(t *testing.T) {
	e := lbuNew(t)
	oaKeyAdmit(t, e, "a2-control-"+randomUUID())
	_, token := lcPrincipal(t, e.p.f, e.tenant(), []string{e.store()}, "store:read", "orders:read", "catalog:read", "inventory:reserve")
	ctx := context.Background()
	var scopeRaw []byte
	if err := e.p.f.runtime.QueryRow(ctx, `SELECT ops.read_order_attempt_scope($1,$2)`, tokenHash(token), e.store()).Scan(&scopeRaw); err != nil {
		t.Fatal(err)
	}
	var scope oaScopeReceipt
	if err := json.Unmarshal(scopeRaw, &scope); err != nil {
		t.Fatal(err)
	}
	tx, err := e.p.f.runtime.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	selection, err := json.Marshal(oaFactsSelection(e))
	if err != nil {
		t.Fatal(err)
	}
	var raw []byte
	version := "a2-bound-" + randomUUID()
	err = tx.QueryRow(ctx, `SELECT ops.prepare_order_attempt_admission($1,$2,$3,$4,$5,$6,$7::jsonb,$8::text)`, tokenHash(token), e.store(), scope.ScopeID, scope.Generation, scope.AttemptID, tokenHash("a2-finish-"+randomUUID()), string(selection), version).Scan(&raw)
	if err != nil {
		t.Fatal("actual eight-argument prepare", err)
	}
	var a oaAdmission
	if err = json.Unmarshal(raw, &a); err != nil || a.AttemptID == "" {
		t.Fatal("native candidate sentinel", err)
	}
	client, err := river.NewClient(riverpgxv5.New(e.p.f.runtime), &river.Config{Schema: "river"})
	if err != nil {
		t.Fatal(err)
	}
	args := json.RawMessage(fmt.Sprintf(`{"attempt_id":%q,"intent_generation":%d}`, a.AttemptID, a.Generation))
	if _, err = client.InsertTx(ctx, tx, oaRawJob{"order_attempt_v1", args}, &river.InsertOpts{Queue: "claims", Priority: 1}); err != nil {
		t.Fatal(err)
	}
	raw = nil
	err = tx.QueryRow(ctx, `SELECT ops.finish_order_attempt_admission($1,$2,$3,$4,$5,decode(repeat('01',40),'hex'))`, tokenHash(token), a.AttemptID, a.Generation, tokenHash("synthetic plan"), "a2-wrong-"+randomUUID()).Scan(&raw)
	oaKeyError(t, string(raw), err, "PT409", "plan_key_version_mismatch")
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if count := miCount(t, e.p.f.owner, `SELECT count(*) FROM ops.order_attempts WHERE attempt_id=$1`, a.AttemptID); count != 0 {
		t.Fatal("mismatched version leaked candidate")
	}
}

func TestOrderAttemptKeyRetirementExactAuthority(t *testing.T) {
	e := lbuNew(t)
	oaKeyAdmit(t, e, "a2-control-"+randomUUID())
	admin := oaKeyAdmin(t, e.p.f)
	ctx := context.Background()
	var adminLogin string
	if err := admin.QueryRow(ctx, `SELECT session_user`).Scan(&adminLogin); err != nil {
		t.Fatal(err)
	}
	var exact bool
	err := e.p.f.owner.QueryRow(ctx, `SELECT
	 (SELECT NOT rolcanlogin AND NOT rolsuper AND NOT rolbypassrls FROM pg_roles WHERE rolname='commerce_order_attempt_key_admin')
	 AND (SELECT count(*)=1 AND bool_and(r.rolname='commerce_order_attempt_key_admin' AND m.inherit_option AND NOT m.set_option)
	 FROM pg_auth_members m JOIN pg_roles r ON r.oid=m.roleid WHERE m.member=$1::regrole)
	 AND (SELECT count(*)=1 AND bool_and(p.oid='ops.retire_order_attempt_plan_key(text)'::regprocedure)
	 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE p.prosecdef
	 AND n.nspname NOT LIKE 'pg_%' AND has_function_privilege('commerce_order_attempt_key_admin',p.oid,'EXECUTE'))
	 AND NOT EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
	 WHERE c.relkind IN ('r','p','v','m') AND n.nspname NOT LIKE 'pg_%' AND n.nspname<>'information_schema'
	 AND (has_table_privilege('commerce_order_attempt_key_admin',c.oid,'SELECT') OR has_table_privilege('commerce_order_attempt_key_admin',c.oid,'INSERT')
	 OR has_table_privilege('commerce_order_attempt_key_admin',c.oid,'UPDATE') OR has_table_privilege('commerce_order_attempt_key_admin',c.oid,'DELETE')))
	 AND (SELECT r.rolname='commerce_order_attempt_owner' AND p.prosecdef AND p.proconfig=ARRAY['search_path=pg_catalog']::text[]
	 AND NOT EXISTS(SELECT 1 FROM aclexplode(p.proacl) x WHERE x.grantee=0 AND x.privilege_type='EXECUTE')
	 FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid='ops.retire_order_attempt_plan_key(text)'::regprocedure)
	 AND to_regprocedure('ops.prepare_order_attempt_admission(bytea,uuid,uuid,bigint,uuid,bytea,jsonb)') IS NULL`, adminLogin).Scan(&exact)
	if err != nil || !exact {
		t.Fatal("A2 exact definer/login/table/old-ABI ACL pins", err)
	}
	for _, sql := range []string{`SELECT * FROM ops.order_attempt_plan_key_versions`, `UPDATE ops.order_attempt_plan_key_versions SET retired_at=NULL`,
		`SELECT * FROM ops.order_attempts`, `SELECT * FROM buyer.owners`, `SET ROLE commerce_order_attempt_owner`,
		`SELECT ops.read_order_attempt_scope(NULL,NULL)`} {
		_, err = admin.Exec(ctx, sql)
		var p *pgconn.PgError
		if !errors.As(err, &p) || p.Code != "42501" {
			t.Fatal("dedicated admin acquired another authority")
		}
	}
	for _, pool := range []*pgxpool.Pool{e.p.f.runtime, miPool(t, e.p.f, "commerce_claims_worker"), miPool(t, e.p.f, "commerce_privacy_writer")} {
		result, callErr := oaKeyRetire(ctx, pool, "a2-denied-"+randomUUID())
		var p *pgconn.PgError
		if !errors.As(callErr, &p) || p.Code != "42501" || result != "" {
			t.Fatal("ordinary actor gained retirement EXECUTE")
		}
	}
}
