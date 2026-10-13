// Purpose: REAL_PG tests-first acceptance of OA1's private transaction-local notification plan.
// Depends on: real C1/fence/Begin/pay-entry helpers, inbox notification definers and native River tables.
// Used by: parent GitHub CI before notification product implementation; every partial C2 rolls back.
// Invariants: order-attempt-v1 §§1–4/6l; refused sends have VOID/NULL operation/no job,
// no fabricated binding, exact semantic identity, private authority and strict SQL NULL checks.
package foundation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/orderattempt"
)

type oaNotificationWire struct {
	NotificationID string `json:"notification_id"`
	State string `json:"state"`
	OperationID *string `json:"operation_id"`
	VoidCode *string `json:"void_code"`
}

func oaNotificationPrepare(tx pgx.Tx, attempt string, plain []byte) ([]byte, error) {
	var raw []byte
	// Calls inbox.order_attempt_prepare_notification, order-attempt-v1 §§2/3/4.
	err := tx.QueryRow(context.Background(), `SELECT inbox.order_attempt_prepare_notification($1::uuid,$2::text)`, attempt, string(plain)).Scan(&raw)
	return raw, err
}

func oaNotificationFinish(tx pgx.Tx, attempt string, plain []byte, material map[string]any) ([]byte, error) {
	encoded, err := json.Marshal(material)
	if err != nil { return nil, err }
	var raw []byte
	// Calls inbox.order_attempt_finish_notification, order-attempt-v1 §§3/4/6l.
	err = tx.QueryRow(context.Background(), `SELECT inbox.order_attempt_finish_notification($1::uuid,$2::text,$3::jsonb)`, attempt, string(plain), string(encoded)).Scan(&raw)
	return raw, err
}

// This is a native partial C2 fixture, never a fabricated COMMITTED attempt.
func oaNotificationNative(t *testing.T, e *lbuEnv, send bool) (orderattempt.AdmissionReceipt, pgx.Tx, []byte, string) {
	t.Helper()
	conversation := lcConversation(t, e.p.f, e.tenant(), e.store(), "page")
	mustExec(t, e.p.f.owner, `UPDATE inbox.conversation_state SET last_inbound_at=clock_timestamp() WHERE conversation_id=$1`, conversation)
	selection := oaBeginSelection(t, e, "bank_transfer", "duplicate-origins")
	selection["for"] = map[string]any{"bundle_ids":[]string{},"conversation_id":conversation}
	selection["send_payment_link"] = send
	a := oaAllocationAdmit(t, e, selection)
	tx, plain, quote, destination, _, lines := oaBeginPrepared(t, e, a)
	oaAbortCleanupTx(t, tx)
	got, _, err := oaBeginCall(tx, a.AttemptID, plain, quote, destination, lines)
	if err != nil || got.OrderID != a.AttemptID || got.JobID <= 0 { t.Fatalf("real Begin prerequisite: %v", err) }
	material, err := payEntryTestKeys(t).Issue(orderattempt.PayEntryBinding{TenantID:e.tenant(),StoreID:e.store(),OrderID:a.AttemptID,Version:1})
	if err != nil { t.Fatal("fixture pay-entry encryption prerequisite") }
	if _, err = oaPayEntryStoreCall(tx, a.AttemptID, plain, material.KeyVersion, material.Digest[:], material.Ciphertext); err != nil {
		t.Fatalf("real pay-entry prerequisite before notification: %v", err)
	}
	return a, tx, plain, conversation
}

func oaNotificationCounts(t *testing.T, tx pgx.Tx, attempt string, rows int) {
	t.Helper()
	var notifications, operations, jobs, expiry int
	err := tx.QueryRow(context.Background(), `SELECT
	 (SELECT count(*) FROM inbox.order_payment_notifications WHERE create_attempt_id=$1),
	 (SELECT count(*) FROM integration.operations WHERE request->>'order_id'=$1::text AND action='meta.dm_send'),
	 (SELECT count(*) FROM river.river_job WHERE kind='external_operation_v1' AND args->>'operation_id' IN
	   (SELECT operation_id::text FROM inbox.order_payment_notifications WHERE create_attempt_id=$1)),
	 (SELECT count(*) FROM river_expiry.river_job WHERE args->>'order_id'=$1::text)`, attempt).Scan(&notifications,&operations,&jobs,&expiry)
	if err != nil || notifications != rows || operations != 0 || jobs != 0 || expiry != 1 {
		t.Fatalf("local refusal effect counts: notification=%d operation=%d notification_job=%d expiry=%d err=%v", notifications,operations,jobs,expiry,err)
	}
	// Also detect an orphan notification job, which the NULL-operation join cannot see.
	var allJobs int
	if err = tx.QueryRow(context.Background(), `SELECT count(*) FROM river.river_job WHERE kind='external_operation_v1' AND xmin=pg_current_xact_id()::xid`).Scan(&allJobs); err != nil || allJobs != 0 {
		t.Fatalf("refusal inserted an orphan external job: %d err=%v", allJobs,err)
	}
}

func oaNotificationVoid(t *testing.T, raw []byte, code string) oaNotificationWire {
	t.Helper()
	var got oaNotificationWire
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw,&got); err != nil { t.Fatal(err) }
	if err := json.Unmarshal(raw,&fields); err != nil { t.Fatal(err) }
	if len(fields) != 4 || got.NotificationID == "" || got.State != "VOID" || got.OperationID != nil || got.VoidCode == nil || *got.VoidCode != code {
		t.Fatal("VOID receipt must contain exact metadata, a fixed refusal and explicit NULL operation")
	}
	if !bytes.Equal(fields["operation_id"],[]byte("null")) { t.Fatal("NULL operation must not be an omitted receipt field") }
	return got
}

func TestOrderAttemptNotificationNoBindingVoidNativeAndRollback(t *testing.T) {
	e := lbuNew(t)
	a, tx, plain, conversation := oaNotificationNative(t,e,true)
	var bindings int
	if err := tx.QueryRow(context.Background(), `SELECT count(*) FROM integration.bindings WHERE tenant_id=$1 AND store_id=$2 AND provider IN ('facebook','instagram')`,e.tenant(),e.store()).Scan(&bindings); err != nil || bindings != 0 {
		t.Fatalf("no-binding fixture sentinel: count=%d err=%v",bindings,err)
	}
	oaStateExec(t,tx,`SET LOCAL ROLE commerce_order_attempt_owner`)
	prepared, err := oaNotificationPrepare(tx,a.AttemptID,plain)
	if err != nil { t.Fatalf("notification prepare after actual native order/pay entry: %v",err) }
	var current struct { State string `json:"state"`; VoidCode string `json:"void_code"` }
	if err = json.Unmarshal(prepared,&current); err != nil || current.State != "VOID" || current.VoidCode != "capability" { t.Fatal("missing binding must be a definitive capability refusal") }
	raw, err := oaNotificationFinish(tx,a.AttemptID,plain,map[string]any{"state":"VOID","void_code":"capability"})
	if err != nil { t.Fatalf("refusal must preserve native order without inventing a binding: %v",err) }
	got := oaNotificationVoid(t,raw,"capability")
	replay, err := oaNotificationFinish(tx,a.AttemptID,plain,map[string]any{"state":"VOID","void_code":"capability"})
	if err != nil || !bytes.Equal(raw,replay) { t.Fatal("same initial notification must replay one immutable receipt") }
	oaStateExec(t,tx,`RESET ROLE`)
	var valid bool
	err = tx.QueryRow(context.Background(), `SELECT n.tenant_id=o.tenant_id AND n.store_id=o.store_id AND n.order_id=o.id
	 AND n.create_attempt_id=o.create_attempt_id AND n.create_attempt_id=$1 AND n.kind='initial' AND n.link_version=1
	 AND n.conversation_id=$2 AND n.binding_id IS NULL AND n.operation_id IS NULL AND n.outbound_id IS NULL
	 AND n.state='VOID' AND n.void_stage='before_c3' AND n.void_code='capability' AND n.dispatch_authorized_at IS NULL
	 AND n.worker_generation=0 AND octet_length(n.semantic_key)=32
	 AND n.semantic_key=sha256(convert_to('initial|'||$1::text,'UTF8'))
	 AND o.creator_kind='order_attempt' AND a.state='PENDING'
	 FROM inbox.order_payment_notifications n JOIN checkout.orders o ON o.id=n.order_id
	 JOIN ops.order_attempts a ON a.attempt_id=n.create_attempt_id WHERE n.id=$3`,a.AttemptID,conversation,got.NotificationID).Scan(&valid)
	if err != nil || !valid { t.Fatalf("VOID native provenance/minimality/semantic-key proof: %v",err) }
	oaNotificationCounts(t,tx,a.AttemptID,1)
	if err = tx.Rollback(context.Background()); err != nil { t.Fatal(err) }
	oaBeginZeroEffects(t,e,a.AttemptID)
	if err = e.p.f.owner.QueryRow(context.Background(), `SELECT count(*) FROM inbox.order_payment_notifications WHERE create_attempt_id=$1`,a.AttemptID).Scan(&bindings); err != nil || bindings != 0 { t.Fatal("outer rollback left notification evidence") }
}

func TestOrderAttemptNotificationNotRequestedHasNoEffects(t *testing.T) {
	e := lbuNew(t)
	a, tx, plain, _ := oaNotificationNative(t,e,false)
	oaStateExec(t,tx,`SET LOCAL ROLE commerce_order_attempt_owner`)
	raw, err := oaNotificationPrepare(tx,a.AttemptID,plain)
	if err != nil { t.Fatal(err) }
	var fields map[string]any
	if err = json.Unmarshal(raw,&fields); err != nil || len(fields) != 1 || fields["state"] != "NOT_REQUESTED" { t.Fatal("no-send prepare must expose only NOT_REQUESTED") }
	raw, err = oaNotificationFinish(tx,a.AttemptID,plain,map[string]any{"state":"NOT_REQUESTED"})
	if err != nil { t.Fatal(err) }
	if err = json.Unmarshal(raw,&fields); err != nil || len(fields) != 1 || fields["state"] != "NOT_REQUESTED" { t.Fatal("no-send finish must not create synthetic evidence") }
	oaStateExec(t,tx,`RESET ROLE`)
	oaNotificationCounts(t,tx,a.AttemptID,0)
}

func TestOrderAttemptNotificationCurrentRefusalWithoutBrowserSession(t *testing.T) {
	for _, tc := range []struct{name,code string}{ {"permission","forbidden"}, {"window","window_closed"} } {
		t.Run(tc.name,func(t *testing.T) {
			e := lbuNew(t)
			a, tx, plain, conversation := oaNotificationNative(t,e,true)
			// Fixture-only mutation AFTER actual C1/fence/Begin. Session removal proves private
			// continuation uses the admitted principal, while current messaging safety is rechecked.
			oaStateExec(t,tx,`DELETE FROM identity.sessions WHERE token_hash=$1`,tokenHash(e.token()))
			if tc.name == "permission" { oaStateExec(t,tx,`DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='inbox:reply'`,e.tenant(),e.store(),e.p.f.principalA) }
			if tc.name == "window" { oaStateExec(t,tx,`UPDATE inbox.conversation_state SET last_inbound_at=clock_timestamp()-interval '25 hours' WHERE conversation_id=$1`,conversation) }
			oaStateExec(t,tx,`SET LOCAL ROLE commerce_order_attempt_owner`)
			raw, err := oaNotificationFinish(tx,a.AttemptID,plain,map[string]any{"state":"VOID","void_code":tc.code})
			if err != nil { t.Fatalf("resume must give current notification refusal without browser-session reauthentication: %v",err) }
			oaNotificationVoid(t,raw,tc.code)
			oaStateExec(t,tx,`RESET ROLE`)
			oaNotificationCounts(t,tx,a.AttemptID,1)
		})
	}
}

func oaNotificationReject(t *testing.T, tx pgx.Tx, query string, args ...any) {
	t.Helper()
	oaStateExec(t,tx,`SAVEPOINT oa_notification_invalid`)
	_, err := tx.Exec(context.Background(),query,args...)
	var pgErr *pgconn.PgError
	if !errors.As(err,&pgErr) || pgErr.Code == "42883" || pgErr.Code == "42P01" { t.Fatalf("expected real enforcement refusal, not a missing product/prerequisite: %v",err) }
	oaStateExec(t,tx,`ROLLBACK TO SAVEPOINT oa_notification_invalid`)
}

func TestOrderAttemptNotificationStrictMaterialAndNullOperationChecks(t *testing.T) {
	e := lbuNew(t)
	a, tx, plain, _ := oaNotificationNative(t,e,true)
	oaStateExec(t,tx,`SET LOCAL ROLE commerce_order_attempt_owner`)
	for _, material := range []map[string]any{
		{"state":"VOID","void_code":"window_closed"},
		{"state":"VOID","void_code":"capability","operation_id":randomUUID()},
		{"state":"VOID","void_code":"capability","recipient":"synthetic-recipient"},
		{"state":"READY","operation_id":randomUUID()},
	} {
		encoded, err := json.Marshal(material)
		if err != nil { t.Fatal(err) }
		oaNotificationReject(t,tx,`SELECT inbox.order_attempt_finish_notification($1::uuid,$2::text,$3::jsonb)`,a.AttemptID,string(plain),string(encoded))
	}
	// A correct refusal after rejected input proves no savepoint leaked fake evidence.
	raw, err := oaNotificationFinish(tx,a.AttemptID,plain,map[string]any{"state":"VOID","void_code":"capability"})
	if err != nil { t.Fatal(err) }
	got := oaNotificationVoid(t,raw,"capability")
	oaStateExec(t,tx,`RESET ROLE`)
	for _, state := range []string{"READY","DISPATCHING","ACCEPTED","DELIVERY_UNKNOWN"} {
		oaNotificationReject(t,tx,`UPDATE inbox.order_payment_notifications SET state=$2 WHERE id=$1`,got.NotificationID,state)
	}
	for _, query := range []string{
		`UPDATE inbox.order_payment_notifications SET state=NULL WHERE id=$1`,
		`UPDATE inbox.order_payment_notifications SET void_stage=NULL WHERE id=$1`,
		`UPDATE inbox.order_payment_notifications SET void_stage='after_c3' WHERE id=$1`,
		`UPDATE inbox.order_payment_notifications SET void_stage='none' WHERE id=$1`,
		`UPDATE inbox.order_payment_notifications SET dispatch_authorized_at=clock_timestamp() WHERE id=$1`,
		`UPDATE inbox.order_payment_notifications SET link_version=0 WHERE id=$1`,
		`UPDATE inbox.order_payment_notifications SET semantic_key=decode('01','hex') WHERE id=$1`,
	} { oaNotificationReject(t,tx,query,got.NotificationID) }
	oaNotificationReject(t,tx,`INSERT INTO inbox.order_payment_notifications SELECT (jsonb_populate_record(NULL::inbox.order_payment_notifications,to_jsonb(n)||jsonb_build_object('id',gen_random_uuid()))).* FROM inbox.order_payment_notifications n WHERE n.id=$1`,got.NotificationID)
	oaNotificationCounts(t,tx,a.AttemptID,1)
}

func TestOrderAttemptNotificationPrivateAuthority(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	for _, signature := range []string{"inbox.order_attempt_prepare_notification(uuid,text)","inbox.order_attempt_finish_notification(uuid,text,jsonb)"} {
		var owner string
		var definer, safePath, publicExecute, coordinator bool
		err := f.owner.QueryRow(ctx,`SELECT r.rolname,p.prosecdef,p.proconfig=ARRAY['search_path=pg_catalog'],
		 EXISTS(SELECT 1 FROM aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a WHERE a.grantee=0 AND a.privilege_type='EXECUTE'),
		 has_function_privilege('commerce_order_attempt_owner',p.oid,'EXECUTE')
		 FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid=to_regprocedure($1)`,signature).Scan(&owner,&definer,&safePath,&publicExecute,&coordinator)
		if err != nil || owner != "commerce_integration_writer" || !definer || !safePath || publicExecute || !coordinator { t.Fatalf("notification definer exact ownership/authority %s: %v",signature,err) }
		for _, role := range []string{"commerce_runtime","commerce_claims_worker","commerce_checkout_runtime","commerce_privacy_writer"} {
			var allowed bool
			if err = f.owner.QueryRow(ctx,`SELECT has_function_privilege($1,to_regprocedure($2),'EXECUTE')`,role,signature).Scan(&allowed); err != nil || allowed { t.Fatalf("direct private helper authority %s/%s: %v",role,signature,err) }
		}
	}
	for _, role := range []string{"commerce_runtime","commerce_claims_worker","commerce_checkout_runtime","commerce_privacy_writer","commerce_order_attempt_owner"} {
		var writes bool
		err := f.owner.QueryRow(ctx,`SELECT has_table_privilege($1,'inbox.order_payment_notifications','INSERT,UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER')`,role).Scan(&writes)
		if err != nil || writes { t.Fatalf("notification table direct mutation %s: %v",role,err) }
	}
	for _, signature := range []string{"ops.order_attempt_execution_context(uuid,text)","social.read_thread(uuid,bigint,integer)","msgtemplates.resolve(text,bigint)"} {
		var allowed bool
		if err := f.owner.QueryRow(ctx,`SELECT has_function_privilege('commerce_integration_writer',to_regprocedure($1),'EXECUTE')`,signature).Scan(&allowed); err != nil || !allowed { t.Fatalf("exact notification reader prerequisite %s: %v",signature,err) }
	}
}

func TestOrderAttemptNotificationRequiresExactHeldPlan(t *testing.T) {
	e := lbuNew(t)
	a := oaAllocationAdmit(t,e,oaFactsSelection(e))
	tx, _ := oaFenceDomainTx(t,e,a,false)
	oaStateExec(t,tx,`SET LOCAL ROLE commerce_order_attempt_owner`)
	oaNotificationReject(t,tx,`SELECT inbox.order_attempt_prepare_notification($1::uuid,$2::text)`,a.AttemptID,`{}`)
	oaNotificationReject(t,tx,`SELECT inbox.order_attempt_finish_notification($1::uuid,$2::text,$3::jsonb)`,a.AttemptID,`{}`,`{"state":"NOT_REQUESTED"}`)
	oaStateExec(t,tx,`RESET ROLE`)
	var count int
	if err := tx.QueryRow(context.Background(),`SELECT count(*) FROM inbox.order_payment_notifications WHERE create_attempt_id=$1`,a.AttemptID).Scan(&count); err != nil || count != 0 { t.Fatal("unfenced call wrote notification effects") }
}
