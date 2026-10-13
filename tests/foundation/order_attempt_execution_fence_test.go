// Purpose: REAL_PG acceptance for the bounded OA1 execution-fence entry.
// Depends: C1 Service.Admit, real runtime/claims logins, native row locks and plan AEAD.
// Used: parent-owned CI red/green; this unit does not execute or settle an order.
// Invariants: attempt first, exact generation, current read authority, protected
// transaction witness, terminal replay without private plan or business effects.
package foundation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/orderattempt"
)

type oaFenceWire struct {
	State string `json:"state"`
	Receipt json.RawMessage `json:"receipt"`
	Binding *orderattempt.PlanBinding `json:"binding"`
	Envelope *orderattempt.SealedPlan `json:"envelope"`
}

func oaFenceAdmit(t *testing.T, e *lbuEnv) orderattempt.AdmissionReceipt {
	t.Helper()
	svc, err := orderattempt.NewService(e.p.f.runtime, oaServiceKeys(t))
	if err != nil { t.Fatal(err) }
	got, err := svc.Admit(context.Background(), e.token(), e.store(), oaServiceRequest(t, e))
	if err != nil || got.State != "PENDING" || got.AttemptID == "" || got.Generation != 1 {
		t.Fatalf("real C1 positive sentinel: %+v err=%v", got, err)
	}
	return got
}

func oaFenceCall(ctx context.Context, tx pgx.Tx, hash []byte, store, attempt string, generation int64) ([]byte, error) {
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT ops.lock_order_attempt($1,$2,$3,$4)`, hash, store, attempt, generation).Scan(&raw)
	return raw, err
}

func oaFenceCode(t *testing.T, raw []byte, err error, code string) {
	t.Helper()
	var pgerr *pgconn.PgError
	if !errors.As(err, &pgerr) || pgerr.Code != code || pgerr.Detail != "" || len(raw) != 0 {
		t.Fatalf("expected non-disclosing %s: bytes=%d err=%v", code, len(raw), err)
	}
}

func oaFenceOpen(t *testing.T, raw []byte, admitted orderattempt.AdmissionReceipt) oaFenceWire {
	t.Helper()
	var got oaFenceWire
	if err := json.Unmarshal(raw, &got); err != nil { t.Fatal(err) }
	if got.State != "PENDING" || got.Binding == nil || got.Envelope == nil || got.Binding.AttemptID != admitted.AttemptID || got.Binding.ScopeID != admitted.ScopeID || got.Binding.Generation != admitted.Generation {
		t.Fatal("pending fence omitted exact persisted binding/envelope")
	}
	if _, err := oaServiceKeys(t).Open(*got.Binding, *got.Envelope); err != nil {
		t.Fatalf("SQL wire must open the actual C1 ciphertext with the real Go codec: %v", err)
	}
	return got
}

func TestOrderAttemptExecutionFenceWireAndTransactionWitness(t *testing.T) {
	e := lbuNew(t)
	a := oaFenceAdmit(t, e)
	ctx := context.Background()
	var before []byte
	if err := e.p.f.owner.QueryRow(ctx, `SELECT to_jsonb(a)-'execution_xid' FROM ops.order_attempts a WHERE attempt_id=$1`, a.AttemptID).Scan(&before); err != nil { t.Fatal(err) }
	// Revoking placement permission cannot invalidate the durable admission.
	mustExec(t, e.p.f.owner, `DELETE FROM identity.store_grants WHERE store_id=$1 AND principal_id=$2 AND permission='catalog:read'`, e.store(), e.p.f.principalA)
	tx, err := e.p.f.runtime.Begin(ctx)
	if err != nil { t.Fatal(err) }
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT set_config('app.tenant_id','caller-marker',true),set_config('app.store_id','caller-marker',true),set_config('app.principal_id','caller-marker',true),set_config('app.order_attempt_execution_xid','forged',true)`); err != nil { t.Fatal(err) }
	raw, err := oaFenceCall(ctx, tx, tokenHash(e.token()), e.store(), a.AttemptID, a.Generation)
	if err != nil { t.Fatal(err) }
	opened := oaFenceOpen(t, raw, a)
	if opened.Binding.TenantID != e.tenant() || opened.Binding.StoreID != e.store() || opened.Binding.PrincipalID != e.p.f.principalA { t.Fatal("SQL binding differs from authenticated identity") }
	var xid, tenant, store, principal string
	if err = tx.QueryRow(ctx, `SELECT pg_current_xact_id()::text,current_setting('app.tenant_id'),current_setting('app.store_id'),current_setting('app.principal_id')`).Scan(&xid,&tenant,&store,&principal); err != nil { t.Fatal(err) }
	if tenant != "caller-marker" || store != tenant || principal != tenant { t.Fatal("execution leaked its validated context") }
	if err = tx.Commit(ctx); err != nil { t.Fatal(err) }
	var persisted string
	var after []byte
	var orders, carts, jobs int
	err = e.p.f.owner.QueryRow(ctx, `SELECT execution_xid::text,to_jsonb(a)-'execution_xid',
	 (SELECT count(*) FROM checkout.orders WHERE tenant_id=a.tenant_id AND store_id=a.store_id AND owner_id=a.buyer_owner_id),
	 (SELECT count(*) FROM storefront.carts WHERE tenant_id=a.tenant_id AND store_id=a.store_id AND owner_id=a.buyer_owner_id),
	 (SELECT count(*) FROM river.river_job WHERE kind='order_attempt_v1' AND args->>'attempt_id'=a.attempt_id::text)
	 FROM ops.order_attempts a WHERE attempt_id=$1`, a.AttemptID).Scan(&persisted,&after,&orders,&carts,&jobs)
	if err != nil || persisted != xid || !bytes.Equal(before,after) || orders != 0 || carts != 0 || jobs != 1 { t.Fatalf("fence changed business effects or lost protected witness: orders=%d carts=%d jobs=%d err=%v",orders,carts,jobs,err) }
	next, err := e.p.f.runtime.Begin(ctx)
	if err != nil { t.Fatal(err) }
	defer next.Rollback(ctx)
	var nextXID string
	if err = next.QueryRow(ctx, `SELECT pg_current_xact_id()::text`).Scan(&nextXID); err != nil || nextXID == xid { t.Fatalf("committed witness must not authorize the next transaction: %v",err) }
	_, err = next.Exec(ctx, `UPDATE ops.order_attempts SET execution_xid=pg_current_xact_id() WHERE attempt_id=$1`,a.AttemptID)
	oaFenceCode(t,nil,err,"42501")
}

func TestOrderAttemptExecutionFenceNoScopeOwnerOrPrincipalLocks(t *testing.T) {
	e := lbuNew(t)
	a := oaFenceAdmit(t,e)
	ctx, cancel := context.WithTimeout(context.Background(),10*time.Second)
	defer cancel()
	holder, err := e.p.f.owner.Begin(ctx)
	if err != nil { t.Fatal(err) }
	defer holder.Rollback(context.Background())
	var subjects int
	if err=holder.QueryRow(ctx,`SELECT count(*) FROM ops.order_attempt_subjects WHERE attempt_id=$1`,a.AttemptID).Scan(&subjects); err!=nil || subjects<1 { t.Fatalf("pending subject positive sentinel: count=%d err=%v",subjects,err) }
	if _,err=holder.Exec(ctx,`SELECT pg_advisory_xact_lock(hashtextextended('order-attempt-subject:'||tenant_id::text||':'||store_id::text||':'||subject_kind||':'||subject_id::text,0)) FROM ops.order_attempt_subjects WHERE attempt_id=$1 ORDER BY subject_kind,subject_id`,a.AttemptID); err!=nil { t.Fatal(err) }
	for _, sql := range []string{
		`SELECT scope_id FROM ops.order_attempt_scopes WHERE scope_id=(SELECT scope_id FROM ops.order_attempts WHERE attempt_id=$1) FOR UPDATE`,
		`SELECT id FROM buyer.owners WHERE id=(SELECT buyer_owner_id FROM ops.order_attempts WHERE attempt_id=$1) FOR UPDATE`,
		`SELECT id FROM identity.principals WHERE id=(SELECT (admission_provenance->>'principal_id')::uuid FROM ops.order_attempts WHERE attempt_id=$1) FOR UPDATE`,
	} {
		var id string
		if err = holder.QueryRow(ctx,sql,a.AttemptID).Scan(&id); err != nil || id == "" { t.Fatalf("held row positive sentinel: %v",err) }
	}
	tx, err := e.p.f.runtime.Begin(ctx)
	if err != nil { t.Fatal(err) }
	defer tx.Rollback(context.Background())
	raw, err := oaFenceCall(ctx,tx,tokenHash(e.token()),e.store(),a.AttemptID,a.Generation)
	if err != nil { t.Fatalf("attempt fence must finish while scope/owner/principal locks remain held: %v",err) }
	oaFenceOpen(t,raw,a)
	// The entry must nevertheless retain the actual attempt lock until rollback.
	_, err = holder.Exec(ctx,`SELECT attempt_id FROM ops.order_attempts WHERE attempt_id=$1 FOR UPDATE NOWAIT`,a.AttemptID)
	oaFenceCode(t,nil,err,"55P03")
}

func TestOrderAttemptExecutionFenceActualLoginsAndNoDisclosure(t *testing.T) {
	e := lbuNew(t)
	a := oaFenceAdmit(t,e)
	worker := miPool(t,e.p.f,"commerce_claims_worker")
	_, foreignToken := lcPrincipal(t,e.p.f,e.tenant(),[]string{e.store()},"store:read","orders:read")
	ctx := context.Background()
	for _, mode := range []string{"worker","worker-auth","runtime-no-auth","foreign-principal","foreign-store","stale-generation","missing"} {
		t.Run(mode,func(t *testing.T) {
			pool, hash, store, id, generation, want := e.p.f.runtime,tokenHash(e.token()),e.store(),a.AttemptID,a.Generation,"PT404"
			switch mode {
			case "worker": pool,hash,want=worker,nil,""
			case "worker-auth": pool,want=worker,"PT400"
			case "runtime-no-auth": hash,want=nil,"PT400"
			case "foreign-principal": hash=tokenHash(foreignToken)
			case "foreign-store": store=e.p.f.storeA2
			case "stale-generation": generation++; want="PT409"
			case "missing": id=randomUUID()
			}
			tx, err := pool.Begin(ctx)
			if err != nil { t.Fatal(err) }
			defer tx.Rollback(ctx)
			raw, err := oaFenceCall(ctx,tx,hash,store,id,generation)
			if want != "" { oaFenceCode(t,raw,err,want); return }
			if err != nil { t.Fatal(err) }
			oaFenceOpen(t,raw,a)
			var actual bool
			if err = tx.QueryRow(ctx,`SELECT session_user=current_user AND pg_has_role(session_user,'commerce_claims_worker','MEMBER')`).Scan(&actual); err != nil || !actual { t.Fatal("worker test did not use the actual worker login") }
		})
	}
	for _, role := range []string{"commerce_ads_worker","commerce_payment_worker","commerce_buyer_runtime","commerce_checkout_runtime"} {
		t.Run(role,func(t *testing.T) {
			pool := miPool(t,e.p.f,role)
			tx, err := pool.Begin(ctx)
			if err != nil { t.Fatal(err) }
			defer tx.Rollback(ctx)
			raw, err := oaFenceCall(ctx,tx,nil,e.store(),a.AttemptID,a.Generation)
			oaFenceCode(t,raw,err,"42501")
		})
	}
	var witnessAbsent bool
	if err := e.p.f.owner.QueryRow(ctx,`SELECT execution_xid IS NULL FROM ops.order_attempts WHERE attempt_id=$1`,a.AttemptID).Scan(&witnessAbsent); err != nil || !witnessAbsent { t.Fatal("rollback/denied callers persisted execution authority") }
}

func TestOrderAttemptExecutionFenceRetiredTerminalAndExpiry(t *testing.T) {
	for _, expired := range []bool{false,true} {
		t.Run(map[bool]string{false:"retained",true:"expired"}[expired],func(t *testing.T) {
			e := lbuNew(t)
			a := oaFenceAdmit(t,e)
			ctx := context.Background()
			// Storage-only native ABORTED fixture, not an executor or C2 success claim.
			mustExec(t,e.p.f.owner,`UPDATE ops.order_attempts SET state='ABORTED',plan_ciphertext=NULL,plan_key_version=NULL,settled_at=clock_timestamp(),receipt=jsonb_build_object('state','ABORTED','attempt_id',attempt_id,'intent_generation',intent_generation,'reason','EXPIRED') WHERE attempt_id=$1`,a.AttemptID)
			age := "0 days"
			if expired { age="90 days" }
			mustExec(t,e.p.f.owner,`UPDATE ops.order_attempt_scopes SET retired_at=clock_timestamp()-$2::interval WHERE scope_id=$1`,a.ScopeID,age)
			var before []byte
			if err := e.p.f.owner.QueryRow(ctx,`SELECT to_jsonb(a) FROM ops.order_attempts a WHERE attempt_id=$1`,a.AttemptID).Scan(&before); err != nil { t.Fatal(err) }
			tx, err := e.p.f.runtime.Begin(ctx)
			if err != nil { t.Fatal(err) }
			defer tx.Rollback(ctx)
			raw, err := oaFenceCall(ctx,tx,tokenHash(e.token()),e.store(),a.AttemptID,a.Generation)
			if expired {
				oaFenceCode(t,raw,err,"PT409")
				var pgerr *pgconn.PgError
				if !errors.As(err,&pgerr) || pgerr.Message != "receipt_expired" { t.Fatal("expired receipt must remain a denial, not absence") }
				return
			}
			if err != nil { t.Fatal(err) }
			var got oaFenceWire
			if err = json.Unmarshal(raw,&got); err != nil || got.State != "ABORTED" || got.Binding != nil || got.Envelope != nil { t.Fatal("terminal replay returned private plan or omitted terminal state") }
			var expected json.RawMessage
			var stored map[string]json.RawMessage
			if err = json.Unmarshal(before,&stored); err != nil { t.Fatal(err) }
			expected=stored["receipt"]
			if !bytes.Equal(got.Receipt,expected) { t.Fatal("terminal receipt was reconstructed or changed") }
			if err = tx.Commit(ctx); err != nil { t.Fatal(err) }
			var after []byte
			if err = e.p.f.owner.QueryRow(ctx,`SELECT to_jsonb(a) FROM ops.order_attempts a WHERE attempt_id=$1`,a.AttemptID).Scan(&after); err != nil || !bytes.Equal(before,after) { t.Fatal("terminal replay mutated the attempt or witness") }
		})
	}
}

func TestOrderAttemptExecutionFenceWaitRechecksAndWorkerSerializes(t *testing.T) {
	for _, mode := range []string{"worker-waits-inline","inline-waits-worker","read-revoked","session-revoked","placement-revoked","settled-during-wait"} {
		t.Run(mode,func(t *testing.T) {
			e := lbuNew(t)
			a := oaFenceAdmit(t,e)
			worker := miPool(t,e.p.f,"commerce_claims_worker")
			ctx,cancel := context.WithTimeout(context.Background(),15*time.Second)
			defer cancel()
			winnerPool,waiterPool,winnerHash,waiterHash := e.p.f.runtime,e.p.f.runtime,tokenHash(e.token()),tokenHash(e.token())
			if mode=="worker-waits-inline" { waiterPool,waiterHash=worker,nil }
			if mode=="inline-waits-worker" { winnerPool,winnerHash=worker,nil }
			if mode=="settled-during-wait" { winnerPool=e.p.f.owner }
			winner,err := winnerPool.Begin(ctx)
			if err != nil { t.Fatal(err) }
			defer winner.Rollback(context.Background())
			if mode=="settled-during-wait" {
				if _,err=winner.Exec(ctx,`SELECT attempt_id FROM ops.order_attempts WHERE attempt_id=$1 FOR UPDATE`,a.AttemptID); err != nil { t.Fatal(err) }
			} else {
				raw,err := oaFenceCall(ctx,winner,winnerHash,e.store(),a.AttemptID,a.Generation)
				if err != nil { t.Fatal(err) }; oaFenceOpen(t,raw,a)
			}
			waiter,err := waiterPool.Begin(ctx)
			if err != nil { t.Fatal(err) }
			defer waiter.Rollback(context.Background())
			type result struct { raw []byte; err error }
			done := make(chan result,1)
			received := false
			defer func(){ cancel(); _=winner.Rollback(context.Background()); if !received { <-done } }()
			go func(){ raw,err := oaFenceCall(ctx,waiter,waiterHash,e.store(),a.AttemptID,a.Generation); done<-result{raw,err} }()
			blocked := false
			deadline := time.Now().Add(5*time.Second)
			for time.Now().Before(deadline) {
				if err= e.p.f.owner.QueryRow(ctx,`SELECT $2::int=ANY(pg_blocking_pids($1))`,int32(waiter.Conn().PgConn().PID()),int32(winner.Conn().PgConn().PID())).Scan(&blocked); err != nil { t.Fatal(err) }
				if blocked { break }; time.Sleep(20*time.Millisecond)
			}
			if !blocked { t.Fatal("contender never reached real attempt fence (pg_blocking_pids)") }
			switch mode {
			case "read-revoked": mustExec(t,e.p.f.owner,`DELETE FROM identity.store_grants WHERE store_id=$1 AND principal_id=$2 AND permission='orders:read'`,e.store(),e.p.f.principalA)
			case "session-revoked": mustExec(t,e.p.f.owner,`UPDATE identity.sessions SET revoked_at=clock_timestamp() WHERE token_hash=$1`,tokenHash(e.token()))
			case "placement-revoked": mustExec(t,e.p.f.owner,`DELETE FROM identity.store_grants WHERE store_id=$1 AND principal_id=$2 AND permission='catalog:read'`,e.store(),e.p.f.principalA)
			case "settled-during-wait":
				if _,err=winner.Exec(ctx,`UPDATE ops.order_attempts SET state='ABORTED',plan_ciphertext=NULL,plan_key_version=NULL,settled_at=clock_timestamp(),receipt=jsonb_build_object('state','ABORTED','attempt_id',attempt_id,'intent_generation',intent_generation,'reason','EXPIRED') WHERE attempt_id=$1`,a.AttemptID); err != nil { t.Fatal(err) }
			}
			if err=winner.Commit(ctx); err != nil { t.Fatal(err) }
			got:=<-done; received=true
			if mode=="read-revoked" { oaFenceCode(t,got.raw,got.err,"PT403"); return }
			if mode=="session-revoked" { oaFenceCode(t,got.raw,got.err,"PT401"); return }
			if got.err != nil { t.Fatal(got.err) }
			if mode=="settled-during-wait" {
				var terminal oaFenceWire
				if err=json.Unmarshal(got.raw,&terminal); err != nil || terminal.State!="ABORTED" || terminal.Envelope!=nil || terminal.Binding!=nil { t.Fatal("post-wait state was not reread") }
			} else { oaFenceOpen(t,got.raw,a) }
		})
	}
}

func TestOrderAttemptExecutionFenceExactACL(t *testing.T) {
	f:=fixture(t)
	var safe bool
	err:=f.owner.QueryRow(context.Background(),`SELECT p.prosecdef AND p.provolatile='v' AND p.prorettype='jsonb'::regtype
	 AND r.rolname='commerce_order_attempt_owner' AND NOT (r.rolcanlogin OR r.rolsuper OR r.rolbypassrls)
	 AND p.proconfig=ARRAY['search_path=pg_catalog']::text[] AND length(obj_description(p.oid,'pg_proc'))>0
	 AND ARRAY(SELECT pg_get_userbyid(a.grantee)::text FROM aclexplode(p.proacl) a WHERE a.grantee<>p.proowner AND a.privilege_type='EXECUTE' ORDER BY 1)=ARRAY['commerce_claims_worker','commerce_runtime']::text[]
	 AND NOT EXISTS(SELECT 1 FROM aclexplode(p.proacl) a WHERE a.grantee<>p.proowner AND a.is_grantable)
	 FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid=to_regprocedure('ops.lock_order_attempt(bytea,uuid,uuid,bigint)')`).Scan(&safe)
	if err!=nil || !safe { t.Fatalf("missing or unsafe exact fence entry: safe=%v err=%v",safe,err) }
}

// The private caller is a NOLOGIN domain definer. The superuser harness switches
// session authorization only to acquire the fence as the real runtime login,
// then restores itself and SET ROLE to the domain owner to test its narrow ACL.
// Production runtime receives neither SET ROLE membership nor direct helper access.
func oaFenceDomainTx(t *testing.T,e *lbuEnv,a orderattempt.AdmissionReceipt,acquire bool) (pgx.Tx,[]byte) {
	t.Helper()
	ctx:=context.Background()
	var login string
	if err:=e.p.f.runtime.QueryRow(ctx,`SELECT session_user::text`).Scan(&login); err!=nil { t.Fatal(err) }
	tx,err:=e.p.f.owner.Begin(ctx)
	if err!=nil { t.Fatal(err) }
	t.Cleanup(func(){ _=tx.Rollback(context.Background()) })
	if !acquire { return tx,nil }
	if _,err=tx.Exec(ctx,`SET LOCAL SESSION AUTHORIZATION `+pgx.Identifier{login}.Sanitize()); err!=nil { t.Fatal(err) }
	raw,err:=oaFenceCall(ctx,tx,tokenHash(e.token()),e.store(),a.AttemptID,a.Generation)
	if err!=nil { t.Fatal(err) }
	got:=oaFenceOpen(t,raw,a)
	plan,err:=oaServiceKeys(t).Open(*got.Binding,*got.Envelope)
	if err!=nil { t.Fatal(err) }
	plain,err:=json.Marshal(plan)
	if err!=nil { t.Fatal(err) }
	if _,err=tx.Exec(ctx,`SET LOCAL SESSION AUTHORIZATION DEFAULT`); err!=nil { t.Fatal(err) }
	return tx,plain
}

func TestOrderAttemptExecutionFencePrivateContext(t *testing.T) {
	e:=lbuNew(t)
	a:=oaFenceAdmit(t,e)
	ctx:=context.Background()
	seed,plain:=oaFenceDomainTx(t,e,a,true)
	if err:=seed.Commit(ctx); err!=nil { t.Fatal(err) }
	for _,role:=range []string{"commerce_buyer_writer","commerce_checkout_writer","commerce_claims_writer","commerce_inbox_writer"} {
		t.Run(role,func(t *testing.T){
			tx,exact:=oaFenceDomainTx(t,e,a,true)
			if _,err:=tx.Exec(ctx,`SELECT set_config('app.tenant_id','caller-marker',true),set_config('app.store_id','caller-marker',true),set_config('app.principal_id','caller-marker',true)`); err!=nil { t.Fatal(err) }
			if _,err:=tx.Exec(ctx,`SET LOCAL ROLE `+pgx.Identifier{role}.Sanitize()); err!=nil { t.Fatal(err) }
			var raw []byte
			err:=tx.QueryRow(ctx,`SELECT ops.order_attempt_execution_context($1,$2)`,a.AttemptID,string(exact)).Scan(&raw)
			if err!=nil { t.Fatal(err) }
			var got struct { Binding orderattempt.PlanBinding `json:"binding"`; Plan orderattempt.FrozenPlan `json:"plan"` }
			if err=json.Unmarshal(raw,&got); err!=nil { t.Fatal(err) }
			got.Binding.AdmittedAt=got.Binding.AdmittedAt.UTC(); got.Binding.ExecuteBy=got.Binding.ExecuteBy.UTC()
			if got.Binding.AttemptID!=a.AttemptID || got.Binding.Generation!=a.Generation || got.Plan.Binding()!=got.Binding || len(got.Plan.Calculation.Lines)!=2 { t.Fatal("domain accessor did not return trusted binding and exact parsed plan") }
			var restored bool
			if err=tx.QueryRow(ctx,`SELECT current_setting('app.tenant_id')='caller-marker' AND current_setting('app.store_id')='caller-marker' AND current_setting('app.principal_id')='caller-marker'`).Scan(&restored); err!=nil || !restored { t.Fatal("private execution context leaked GUC authority") }
		})
	}
	for _,mode:=range []string{"old-xid","guc-only","altered-money","altered-owner","altered-facts","altered-hash","same-json-different-bytes","wrong-attempt"} {
		t.Run(mode,func(t *testing.T){
			acquire:=mode!="old-xid" && mode!="guc-only"
			tx,exact:=oaFenceDomainTx(t,e,a,acquire)
			if !acquire { exact=plain }
			want,id:="PT400",a.AttemptID
			switch mode {
			case "old-xid","guc-only": want="PT409"
			case "same-json-different-bytes": exact=append([]byte(" "),exact...)
			case "wrong-attempt": id=randomUUID(); want="PT409"
			default:
				var plan map[string]any
				if err:=json.Unmarshal(exact,&plan); err!=nil { t.Fatal(err) }
				switch mode {
				case "altered-money": plan["calculation"]=map[string]any{"subtotal_minor":1}
				case "altered-owner": plan["buyer_owner_id"]=randomUUID()
				case "altered-facts": plan["facts"]=map[string]any{"selection":map[string]any{}}
				case "altered-hash": plan["request_hash"]=make([]int,32)
				}
				var err error
				exact,err=json.Marshal(plan)
				if err!=nil { t.Fatal(err) }
			}
			if _,err:=tx.Exec(ctx,`SELECT set_config('app.tenant_id',$1,true),set_config('app.store_id',$2,true),set_config('app.principal_id',$3,true),set_config('app.order_attempt_execution_xid',pg_current_xact_id()::text,true)`,e.tenant(),e.store(),e.p.f.principalA); err!=nil { t.Fatal(err) }
			if _,err:=tx.Exec(ctx,`SET LOCAL ROLE commerce_checkout_writer`); err!=nil { t.Fatal(err) }
			var raw []byte
			err:=tx.QueryRow(ctx,`SELECT ops.order_attempt_execution_context($1,$2)`,id,string(exact)).Scan(&raw)
			oaFenceCode(t,raw,err,want)
		})
	}
	for _,role:=range []string{"commerce_runtime","commerce_claims_worker"} {
		t.Run(role+"-direct-context-denied",func(t *testing.T){
			pool:=miPool(t,e.p.f,role)
			var raw []byte
			err:=pool.QueryRow(ctx,`SELECT ops.order_attempt_execution_context($1,$2)`,a.AttemptID,string(plain)).Scan(&raw)
			oaFenceCode(t,raw,err,"42501")
		})
	}
	var safe bool
	err:=e.p.f.owner.QueryRow(ctx,`SELECT p.prosecdef AND p.provolatile='v' AND p.prorettype='jsonb'::regtype
	 AND r.rolname='commerce_order_attempt_owner' AND NOT (r.rolcanlogin OR r.rolsuper OR r.rolbypassrls)
	 AND p.proconfig=ARRAY['search_path=pg_catalog']::text[] AND length(obj_description(p.oid,'pg_proc'))>0
	 AND ARRAY(SELECT pg_get_userbyid(a.grantee)::text FROM aclexplode(p.proacl) a WHERE a.grantee<>p.proowner AND a.privilege_type='EXECUTE' ORDER BY 1)=ARRAY['commerce_buyer_writer','commerce_checkout_writer','commerce_claims_writer','commerce_inbox_writer']::text[]
	 AND NOT EXISTS(SELECT 1 FROM aclexplode(p.proacl) a WHERE a.grantee<>p.proowner AND a.is_grantable)
	 FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid=to_regprocedure('ops.order_attempt_execution_context(uuid,text)')`).Scan(&safe)
	if err!=nil || !safe { t.Fatalf("missing or unsafe private context ACL: safe=%v err=%v",safe,err) }
}

func TestOrderAttemptExecutionFenceMetadataContext(t *testing.T) {
	e:=lbuNew(t)
	a:=oaFenceAdmit(t,e)
	ctx:=context.Background()
	seed,_:=oaFenceDomainTx(t,e,a,true)
	if err:=seed.Commit(ctx); err!=nil { t.Fatal(err) }
	for _,role:=range []string{"commerce_buyer_writer","commerce_checkout_writer","commerce_claims_writer","commerce_inbox_writer"} {
		t.Run(role,func(t *testing.T){
			tx,_:=oaFenceDomainTx(t,e,a,true)
			if _,err:=tx.Exec(ctx,`SELECT set_config('app.tenant_id','caller-marker',true),set_config('app.store_id','caller-marker',true),set_config('app.principal_id','caller-marker',true)`); err!=nil { t.Fatal(err) }
			if _,err:=tx.Exec(ctx,`SET LOCAL ROLE `+pgx.Identifier{role}.Sanitize()); err!=nil { t.Fatal(err) }
			var raw []byte
			if err:=tx.QueryRow(ctx,`SELECT ops.order_attempt_fence_context($1,$2)`,a.AttemptID,a.Generation).Scan(&raw); err!=nil { t.Fatal(err) }
			var got orderattempt.PlanBinding
			var fields map[string]json.RawMessage
			if err:=json.Unmarshal(raw,&got); err!=nil { t.Fatal(err) }
			if err:=json.Unmarshal(raw,&fields); err!=nil { t.Fatal(err) }
			if len(fields)!=12 || got.Schema!=1 || got.AttemptID!=a.AttemptID || got.ScopeID!=a.ScopeID || got.Generation!=a.Generation || got.TenantID!=e.tenant() || got.StoreID!=e.store() || got.PrincipalID!=e.p.f.principalA || got.BuyerOwnerID=="" || got.RequestHash==([32]byte{}) || got.FactsDigest==([32]byte{}) || !got.AdmittedAt.Add(15*time.Minute).Equal(got.ExecuteBy) { t.Fatal("metadata context must expose exact stored binding without plaintext/ciphertext/receipt") }
			var restored bool
			if err:=tx.QueryRow(ctx,`SELECT current_setting('app.tenant_id')='caller-marker' AND current_setting('app.store_id')='caller-marker' AND current_setting('app.principal_id')='caller-marker'`).Scan(&restored); err!=nil || !restored { t.Fatal("private metadata context leaked GUC authority") }
		})
	}
	for _,mode:=range []string{"old-xid","guc-only","wrong-generation","terminal"} {
		t.Run(mode,func(t *testing.T){
			tx,_:=oaFenceDomainTx(t,e,a,mode=="wrong-generation" || mode=="terminal")
			generation:=a.Generation
			if mode=="wrong-generation" { generation++ }
			if mode=="terminal" {
				// Storage-only rollback fixture exercises the same-xid state recheck.
				if _,err:=tx.Exec(ctx,`UPDATE ops.order_attempts SET state='ABORTED',plan_ciphertext=NULL,plan_key_version=NULL,settled_at=clock_timestamp(),receipt=jsonb_build_object('state','ABORTED','attempt_id',attempt_id,'intent_generation',intent_generation,'reason','EXPIRED') WHERE attempt_id=$1`,a.AttemptID); err!=nil { t.Fatal(err) }
			}
			if _,err:=tx.Exec(ctx,`SELECT set_config('app.tenant_id',$1,true),set_config('app.store_id',$2,true),set_config('app.principal_id',$3,true),set_config('app.order_attempt_execution_xid',pg_current_xact_id()::text,true)`,e.tenant(),e.store(),e.p.f.principalA); err!=nil { t.Fatal(err) }
			if _,err:=tx.Exec(ctx,`SET LOCAL ROLE commerce_claims_writer`); err!=nil { t.Fatal(err) }
			var raw []byte
			err:=tx.QueryRow(ctx,`SELECT ops.order_attempt_fence_context($1,$2)`,a.AttemptID,generation).Scan(&raw)
			oaFenceCode(t,raw,err,"PT409")
		})
	}
	for _,role:=range []string{"commerce_runtime","commerce_claims_worker"} {
		t.Run(role+"-direct-metadata-denied",func(t *testing.T){
			pool:=miPool(t,e.p.f,role)
			var raw []byte
			err:=pool.QueryRow(ctx,`SELECT ops.order_attempt_fence_context($1,$2)`,a.AttemptID,a.Generation).Scan(&raw)
			oaFenceCode(t,raw,err,"42501")
		})
	}
	var safe bool
	err:=e.p.f.owner.QueryRow(ctx,`SELECT p.prosecdef AND p.provolatile='v' AND p.prorettype='jsonb'::regtype
	 AND r.rolname='commerce_order_attempt_owner' AND NOT (r.rolcanlogin OR r.rolsuper OR r.rolbypassrls)
	 AND p.proconfig=ARRAY['search_path=pg_catalog']::text[] AND length(obj_description(p.oid,'pg_proc'))>0
	 AND ARRAY(SELECT pg_get_userbyid(a.grantee)::text FROM aclexplode(p.proacl) a WHERE a.grantee<>p.proowner AND a.privilege_type='EXECUTE' ORDER BY 1)=ARRAY['commerce_buyer_writer','commerce_checkout_writer','commerce_claims_writer','commerce_inbox_writer']::text[]
	 AND NOT EXISTS(SELECT 1 FROM aclexplode(p.proacl) a WHERE a.grantee<>p.proowner AND a.is_grantable)
	 FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid=to_regprocedure('ops.order_attempt_fence_context(uuid,bigint)')`).Scan(&safe)
	if err!=nil || !safe { t.Fatalf("missing or unsafe plaintext-free metadata context ACL: safe=%v err=%v",safe,err) }
}
