// Purpose: REAL_PG tests-first acceptance of OA1's private transaction-local notification plan.
// Depends on: real C1/fence/Begin/pay-entry helpers, inbox notification definers and native River tables.
// Used by: parent GitHub CI before notification product implementation; every partial C2 rolls back.
// Invariants: order-attempt-v1 §§1–4/6l; refused sends have VOID/NULL operation/no job,
// no fabricated binding, exact semantic identity, private authority and strict SQL NULL checks.
package foundation_test

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"livecommerce/internal/inbox"
	"livecommerce/internal/integrations/core"
	"livecommerce/internal/integrations/meta"
	"livecommerce/internal/msgtemplates"
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
	// Begin/pay-entry use the checkout definer role; restore the fixture owner
	// before fixture mutation or switching to the private notification coordinator.
	oaStateExec(t,tx,`RESET ROLE`)
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
		`UPDATE inbox.order_payment_notifications SET semantic_key=decode(repeat('ab',32),'hex') WHERE id=$1`,
		`UPDATE inbox.order_payment_notifications SET owner_id=NULL WHERE id=$1`,
		`UPDATE inbox.order_payment_notifications SET kind='arbitrary' WHERE id=$1`,
	} { oaNotificationReject(t,tx,query,got.NotificationID) }
	oaNotificationReject(t,tx,`INSERT INTO inbox.order_payment_notifications SELECT (jsonb_populate_record(NULL::inbox.order_payment_notifications,to_jsonb(n)||jsonb_build_object('id',gen_random_uuid()))).* FROM inbox.order_payment_notifications n WHERE n.id=$1`,got.NotificationID)
	oaNotificationCounts(t,tx,a.AttemptID,1)
}

func TestOrderAttemptNotificationPrivateAuthority(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	oaProvenanceFK(t,f,"inbox.order_payment_notifications","checkout.orders",[]string{"tenant_id","store_id","owner_id","order_id","create_attempt_id"},[]string{"tenant_id","store_id","owner_id","id","create_attempt_id"},false)
	oaProvenanceFK(t,f,"inbox.order_payment_notifications","ops.order_attempt_creators",[]string{"tenant_id","store_id","owner_id","create_attempt_id"},[]string{"tenant_id","store_id","buyer_owner_id","attempt_id"},false)
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

func TestOrderAttemptNotificationStorefrontOriginCurrentEligibility(t *testing.T) {
	e := lbuNew(t)
	a,tx,plain,_ := oaNotificationNative(t,e,true)
	// A continuation has no browser session and resolves only the held native plan's store.
	oaStateExec(t,tx,`DELETE FROM identity.sessions WHERE token_hash=$1`,tokenHash(e.token()))
	foreignStore,foreignOrigin := randomUUID(),"https://foreign-"+strings.ReplaceAll(randomUUID(),"-","")+".example"
	oaStateExec(t,tx,`INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'synthetic foreign origin','TWD')`,e.p.f.tenantB,foreignStore)
	oaStateExec(t,tx,`INSERT INTO control.storefront_publications(tenant_id,store_id,published) VALUES($1,$2,true)`,e.p.f.tenantB,foreignStore)
	oaStateExec(t,tx,`INSERT INTO control.storefront_domains(tenant_id,store_id,origin,state,ownership_verified_at,tls_verified_at,valid_until,evidence_ref) VALUES($1,$2,$3,'ACTIVE',clock_timestamp()-interval '1 hour',clock_timestamp()-interval '1 hour',clock_timestamp()+interval '1 hour','synthetic foreign proof')`,e.p.f.tenantB,foreignStore,foreignOrigin)
	for _, tc := range []struct{name,state,query string}{
		{"configured","configured",`SELECT 1`},
		{"unpublished","storefront_unavailable",`UPDATE control.storefront_publications SET published=false WHERE tenant_id=$1 AND store_id=$2`},
		{"store_disabled","storefront_unavailable",`UPDATE control.stores SET active=false WHERE tenant_id=$1 AND id=$2`},
		{"tenant_disabled","storefront_unavailable",`UPDATE control.tenants SET active=false WHERE id=$1 AND $2::uuid IS NOT NULL`},
		{"expired","storefront_unavailable",`UPDATE control.storefront_domains SET valid_until=clock_timestamp()-interval '1 minute' WHERE tenant_id=$1 AND store_id=$2`},
		{"future_proof","storefront_unavailable",`UPDATE control.storefront_domains SET ownership_verified_at=clock_timestamp()+interval '1 hour',valid_until=clock_timestamp()+interval '2 hours' WHERE tenant_id=$1 AND store_id=$2`},
		{"ambiguous","domain_selection_required",`INSERT INTO control.storefront_domains(tenant_id,store_id,origin,state,ownership_verified_at,tls_verified_at,valid_until,evidence_ref) VALUES($1,$2,'https://oa-'||replace(gen_random_uuid()::text,'-','')||'.example','ACTIVE',clock_timestamp()-interval '1 hour',clock_timestamp()-interval '1 hour',clock_timestamp()+interval '1 hour','synthetic second domain')`},
	} {
		t.Run(tc.name,func(t *testing.T) {
			oaStateExec(t,tx,`SAVEPOINT oa_origin_case`)
			if tc.name!="configured" { oaStateExec(t,tx,tc.query,e.tenant(),e.store()) }
			oaStateExec(t,tx,`SET LOCAL ROLE commerce_integration_writer`)
			var raw []byte
			err := tx.QueryRow(context.Background(),`SELECT buyer.order_attempt_storefront_origin($1::uuid,$2::text)`,a.AttemptID,string(plain)).Scan(&raw)
			if err != nil { t.Fatalf("bound current origin helper: %v",err) }
			var result struct{ State string `json:"state"`; Origin *string `json:"origin"` }
			var fields map[string]json.RawMessage
			if err=json.Unmarshal(raw,&result); err != nil { t.Fatal(err) }
			if err=json.Unmarshal(raw,&fields); err != nil || len(fields)!=2 || result.State!=tc.state { t.Fatal("origin must have exact frozen state/origin shape") }
			if tc.state=="configured" { if result.Origin==nil || *result.Origin!=e.origin { t.Fatal("origin did not use existing same-store published proof") } } else if result.Origin!=nil || !bytes.Equal(fields["origin"],[]byte("null")) { t.Fatal("unavailable/ambiguous origin exposed a private URL") }
			oaStateExec(t,tx,`RESET ROLE`)
			oaStateExec(t,tx,`ROLLBACK TO SAVEPOINT oa_origin_case`)
		})
	}
	oaStateExec(t,tx,`SET LOCAL ROLE commerce_integration_writer`)
	oaNotificationReject(t,tx,`SELECT buyer.order_attempt_storefront_origin($1::uuid,$2::text)`,a.AttemptID,`{}`)
	oaStateExec(t,tx,`RESET ROLE`)
	var owner string
	var definer,safePath,publicExecute,integration,coordinator bool
	err := tx.QueryRow(context.Background(),`SELECT r.rolname,p.prosecdef,p.proconfig=ARRAY['search_path=pg_catalog'],
	 EXISTS(SELECT 1 FROM aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a WHERE a.grantee=0 AND a.privilege_type='EXECUTE'),
	 has_function_privilege('commerce_integration_writer',p.oid,'EXECUTE'),has_function_privilege('commerce_order_attempt_owner',p.oid,'EXECUTE')
	 FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid=to_regprocedure('buyer.order_attempt_storefront_origin(uuid,text)')`).Scan(&owner,&definer,&safePath,&publicExecute,&integration,&coordinator)
	if err != nil || owner!="commerce_buyer_writer" || !definer || !safePath || publicExecute || !integration || coordinator { t.Fatalf("origin exact sole-caller private authority: %v",err) }
	// 0064 already grants integration the public origin/state projection. This
	// private eligibility reader must not add direct access to ownership/TLS proof.
	var rawProofRead bool
	if err=tx.QueryRow(context.Background(),`SELECT has_column_privilege('commerce_integration_writer','control.storefront_domains','ownership_verified_at','SELECT') OR has_column_privilege('commerce_integration_writer','control.storefront_domains','tls_verified_at','SELECT') OR has_column_privilege('commerce_integration_writer','control.storefront_domains','valid_until','SELECT')`).Scan(&rawProofRead); err != nil || rawProofRead { t.Fatalf("origin preparation acquired raw proof-table authority: %v",err) }
	oaNotificationCounts(t,tx,a.AttemptID,0)
}

// Runtime interface assertion keeps tests FIRST compilable before the pure seam exists.
// Its absence must be an explicit red result, never a skipped crypto acceptance.
type oaNotificationPreparer interface {
	PrepareOrderAttemptNotification(json.RawMessage,string,string,string,int64) (json.RawMessage,error)
}

func oaNotificationSignedConversation(t *testing.T, e *lbuEnv) (miTest,string,string,string) {
	t.Helper()
	m := miSetup(t)
	// psSetupItemsOn owns a separate synthetic store in the same physical test DB.
	// The shared ingress/registrar pools have no caller-supplied store authority;
	// this fixture pointer only directs the existing helpers to that store's owner SQL.
	m.f = e.p.f
	asset, psid := miAsset(), miAsset()
	binding := miBinding(t,m,asset,"facebook",e.tenant(),e.store(),e.p.f.principalA)
	miRoute(t,m,asset,e.tenant(),e.store(),binding)
	mustExec(t,m.f.owner,`INSERT INTO integration.meta_connections(tenant_id,store_id,page_id,page_name,fb_binding,scopes,status,connected_by,route_expires_at)
	 VALUES($1,$2,$3,'synthetic OA page',$4,ARRAY['pages_messaging'],'active',$5,clock_timestamp()+interval '1 day')`,e.tenant(),e.store(),asset,binding,e.p.f.principalA)
	t.Cleanup(func(){ mustExec(t,m.f.owner,`DELETE FROM integration.meta_connections WHERE tenant_id=$1 AND store_id=$2`,e.tenant(),e.store()) })
	workerPool := miPool(t,m.f,"commerce_meta_worker")
	consumerPool := mcConsumer(t,m)
	client, err := meta.NewConsumerClient(context.Background(),workerPool,consumerPool,mcKeys(t,m),1)
	if err != nil { t.Fatalf("native same-store consumer constructor: %v",err) }
	if err = client.Start(context.Background()); err != nil { t.Fatal(err) }
	t.Cleanup(func(){
		stop, cancel := context.WithTimeout(context.Background(),8*time.Second)
		defer cancel()
		if err := client.StopAndCancel(stop); err != nil { t.Errorf("native consumer bounded stop: %v",err) }
	})
	raw := []byte(fmt.Sprintf(`{"object":"page","entry":[{"id":%q,"messaging":[{"sender":{"id":%q},"recipient":{"id":%q},"timestamp":%d,"message":{"mid":%q,"text":"synthetic OA inbound"}}]}]}`,asset,psid,asset,time.Now().UnixMilli(),"m."+randomUUID()))
	event := mcPost(t,m,asset,raw)
	mcAwait(t,m,event)
	var conversation string
	err = m.f.owner.QueryRow(context.Background(),`SELECT id::text FROM social.conversations WHERE tenant_id=$1 AND store_id=$2 AND app_id=$3 AND object='page' AND asset_id=$4 AND peer_key=$5`,e.tenant(),e.store(),miApp,asset,meta.SocialPeerKey(miApp,"page",asset,psid)).Scan(&conversation)
	if err != nil || conversation == "" { t.Fatalf("signed same-store native conversation sentinel: %v",err) }
	return m,conversation,binding,psid
}

func TestOrderAttemptNotificationReadyNativeActualCrypto(t *testing.T) {
	e := lbuNew(t)
	m,conversation,binding,psid := oaNotificationSignedConversation(t,e)
	selection := oaBeginSelection(t,e,"bank_transfer","duplicate-origins")
	selection["for"] = map[string]any{"bundle_ids":[]string{},"conversation_id":conversation}
	selection["send_payment_link"] = true
	a := oaAllocationAdmit(t,e,selection)
	tx, plain, quote, destination, _, lines := oaBeginPrepared(t,e,a)
	oaAbortCleanupTx(t,tx)
	if got, _, err := oaBeginCall(tx,a.AttemptID,plain,quote,destination,lines); err != nil || got.OrderID != a.AttemptID { t.Fatalf("READY native Begin: %v",err) }
	payKeys := payEntryTestKeys(t)
	payBinding := orderattempt.PayEntryBinding{TenantID:e.tenant(),StoreID:e.store(),OrderID:a.AttemptID,Version:1}
	payMaterial, err := payKeys.Issue(payBinding)
	if err != nil { t.Fatal("synthetic pay-entry issue") }
	if _, err = oaPayEntryStoreCall(tx,a.AttemptID,plain,payMaterial.KeyVersion,payMaterial.Digest[:],payMaterial.Ciphertext); err != nil { t.Fatalf("READY pay-entry prerequisite: %v",err) }
	credential, err := payKeys.Open(payBinding,payMaterial)
	if err != nil { t.Fatal("synthetic pay-entry open") }
	oaStateExec(t,tx,`RESET ROLE`)
	oaStateExec(t,tx,`SET LOCAL ROLE commerce_order_attempt_owner`)
	prepared, err := oaNotificationPrepare(tx,a.AttemptID,plain)
	if err != nil { t.Fatalf("actual READY eligibility/template/thread preparation: %v",err) }
	var contextWire struct {
		State string `json:"state"`
		BindingID string `json:"binding_id"`
		ConversationID string `json:"conversation_id"`
		TemplateBody string `json:"template_body"`
		TemplateID string `json:"template_id"`
		TemplateVersion int64 `json:"template_version"`
		Origin string `json:"origin"`
		Locale string `json:"locale"`
		Thread json.RawMessage `json:"thread_row"`
	}
	if err = json.Unmarshal(prepared,&contextWire); err != nil || contextWire.State != "PREPARED" || contextWire.BindingID != binding || contextWire.ConversationID != conversation || contextWire.TemplateID != "order-pay-link/v1" || contextWire.TemplateVersion != 1 || contextWire.Origin!=e.origin || contextWire.Locale!=selection["locale"] || len(contextWire.Thread) == 0 { t.Fatal("READY must come from current same-store native binding/template/encrypted inbound/published origin and frozen locale") }
	// tcvNew's existing bhPublish creates the real current publication/domain proof.
	// The URL is a dormant crypto fixture; URL-consumer activation is outside OA1.
	buyerLink := contextWire.Origin+"/"+contextWire.Locale+"/pay-entry#o="+a.AttemptID+"&t="+credential
	oaStateExec(t,tx,`RESET ROLE`)
	jobs, err := river.NewClient[pgx.Tx](riverpgxv5.New(e.p.f.runtime),&river.Config{Schema:"river"})
	if err != nil { t.Fatal(err) }
	// Obtain producer identity from the real runtime pool, not an invented role/session.
	var login string
	if err = e.p.f.runtime.QueryRow(context.Background(),`SELECT session_user::text`).Scan(&login); err != nil { t.Fatal(err) }
	operation,outbound := randomUUID(),randomUUID()
	oaStateExec(t,tx,`SET LOCAL SESSION AUTHORIZATION `+pgx.Identifier{login}.Sanitize())
	job, err := core.InsertOperationJob(context.Background(),jobs,tx,operation)
	if err != nil { t.Fatalf("actual runtime producer exact external-operation job: %v",err) }
	oaStateExec(t,tx,`SET LOCAL SESSION AUTHORIZATION DEFAULT`)
	// The test holds the worker-only private HPKE ring solely to open the sealed output.
	keyJSON := `{"keys":[{"id":"`+miKeyID+`","key_base64":"`+base64.StdEncoding.EncodeToString(m.key)+`"}]}`
	keys, err := inbox.LoadKeyring(func(n string) string { if n=="COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID" { return miKeyID }; if n=="COMMERCE_META_PAYLOAD_KEYS_JSON" { return keyJSON }; return "" })
	if err != nil { t.Fatal("synthetic payload ring") }
	svc, err := inbox.NewService(keys)
	if err != nil { t.Fatal(err) }
	seal, open := mcnPageRing(t)
	svc.EnableSend(seal,jobs,msgtemplates.NewService())
	preparer, exists := any(svc).(oaNotificationPreparer)
	if !exists { t.Fatal("frozen pure inbox.PrepareOrderAttemptNotification seam is absent") }
	material, err := preparer.PrepareOrderAttemptNotification(prepared,buyerLink,operation,outbound,job)
	if err != nil { t.Fatal("actual payload/recipient encryption failed") }
	var artifact map[string]any
	if err = json.Unmarshal(material,&artifact); err != nil { t.Fatal(err) }
	if len(artifact) != 12 || artifact["state"]!="READY" || artifact["operation_id"]!=operation || artifact["outbound_id"]!=outbound || artifact["job_id"]!=float64(job) { t.Fatal("READY artifact must have exact frozen shape and parent-minted IDs/job") }
	var fields map[string]json.RawMessage
	var thread map[string]json.RawMessage
	if err = json.Unmarshal(prepared,&fields); err != nil { t.Fatal(err) }
	if err = json.Unmarshal(fields["thread_row"],&thread); err != nil { t.Fatal(err) }
	threadKeys := []string{"event_id","key_id","nonce_hex","ciphertext_hex","app_id","object","asset_id","event_key","payload_hash","tenant_id","store_id","route_id","route_epoch","server_seq","occurred_at","direction"}
	if len(thread)!=len(threadKeys) { t.Fatal("encrypted thread DTO diverged from the frozen field set") }
	for _, name := range threadKeys { if _, present := thread[name]; !present { t.Fatalf("encrypted thread DTO omitted %s",name) } }
	// Tamper the actual authenticated inbound ciphertext. No fake recipient or
	// handcrafted event substitutes for the stored, signed social fact.
	thread["ciphertext_hex"] = json.RawMessage(`"01"`)
	fields["thread_row"], err = json.Marshal(thread)
	if err != nil { t.Fatal(err) }
	broken, err := json.Marshal(fields)
	if err != nil { t.Fatal(err) }
	if result, err := preparer.PrepareOrderAttemptNotification(broken,buyerLink,operation,outbound,job); err == nil || len(result)!=0 { t.Fatal("tampered inbound ciphertext produced a recipient/send artifact") }
	if result, err := preparer.PrepareOrderAttemptNotification(prepared,"",operation,outbound,job); err == nil || len(result)!=0 { t.Fatal("missing order-only URL produced a send artifact") }
	oaStateExec(t,tx,`SET LOCAL ROLE commerce_order_attempt_owner`)
	raw, err := oaNotificationFinish(tx,a.AttemptID,plain,artifact)
	if err != nil { t.Fatalf("native atomic READY/operation/job/outbound/secrets planning: %v",err) }
	var result oaNotificationWire
	if err = json.Unmarshal(raw,&result); err != nil || result.State!="READY" || result.NotificationID=="" || result.OperationID==nil || *result.OperationID!=operation || result.VoidCode!=nil { t.Fatal("READY metadata receipt must expose no bearer/recipient/body") }
	replay, err := oaNotificationFinish(tx,a.AttemptID,plain,artifact)
	if err != nil || !bytes.Equal(raw,replay) { t.Fatal("READY replay must not create another operation/job") }
	oaStateExec(t,tx,`RESET ROLE`)
	var exact bool
	err = tx.QueryRow(context.Background(),`SELECT n.owner_id=o.owner_id AND n.create_attempt_id=o.create_attempt_id AND n.order_id=o.id
	 AND n.tenant_id=o.tenant_id AND n.store_id=o.store_id AND n.link_version=1 AND n.kind='initial'
	 AND n.state='READY' AND n.void_stage='none' AND n.dispatch_authorized_at IS NULL AND n.binding_id=$2
	 AND n.operation_id=$3 AND n.outbound_id=$4 AND n.worker_generation=0
	 AND p.id=n.operation_id AND p.binding_id=n.binding_id AND p.action='meta.dm_send' AND p.state='READY' AND p.job_id=$5
	 AND p.request->>'order_id'=o.id::text AND p.request->>'conversation_id'=$6 AND p.request->>'outbound_id'=$4::text
	 AND j.kind='external_operation_v1' AND j.args=jsonb_build_object('operation_id',$3::text,'version',1)
	 AND j.xmin=pg_current_xact_id()::xid
	 FROM inbox.order_payment_notifications n JOIN checkout.orders o ON o.id=n.order_id
	 JOIN integration.operations p ON p.id=n.operation_id JOIN river.river_job j ON j.id=p.job_id WHERE n.id=$1`,result.NotificationID,binding,operation,outbound,job,conversation).Scan(&exact)
	if err != nil || !exact { t.Fatalf("READY exact native state/scope/provenance/job atomic pair: %v",err) }
	var displayKey string
	var nonce,displayCT,enc,sealed,digest []byte
	err = tx.QueryRow(context.Background(),`SELECT b.key_id,b.nonce,b.ciphertext,s.enc,s.sealed,b.body_hmac FROM inbox.outbound_messages b JOIN inbox.send_secrets s ON s.operation_id=b.operation_id WHERE b.id=$1 AND b.operation_id=$2`,outbound,operation).Scan(&displayKey,&nonce,&displayCT,&enc,&sealed,&digest)
	if err != nil { t.Fatal(err) }
	derive := hmac.New(sha256.New,m.key)
	derive.Write([]byte("livecommerce/meta-outbound-hmac/v1"+e.tenant()))
	mac := hmac.New(sha256.New,derive.Sum(nil))
	mac.Write([]byte("order-pay-link|"+a.AttemptID+"|"+contextWire.TemplateBody))
	if !hmac.Equal(digest,mac.Sum(nil)) || artifact["body_hmac_hex"]!=hex.EncodeToString(digest) { t.Fatal("body digest must independently authenticate the tenant, order and fixed template without the bearer") }
	var notifications,operations,nativeJobs,outbounds,secrets,expiry int
	err = tx.QueryRow(context.Background(),`SELECT
	 (SELECT count(*) FROM inbox.order_payment_notifications WHERE create_attempt_id=$1),
	 (SELECT count(*) FROM integration.operations WHERE request->>'order_id'=$1::text AND action='meta.dm_send'),
	 (SELECT count(*) FROM river.river_job WHERE kind='external_operation_v1' AND xmin=pg_current_xact_id()::xid),
	 (SELECT count(*) FROM inbox.outbound_messages WHERE operation_id=$2),
	 (SELECT count(*) FROM inbox.send_secrets WHERE operation_id=$2),
	 (SELECT count(*) FROM river_expiry.river_job WHERE args->>'order_id'=$1::text)`,a.AttemptID,operation).Scan(&notifications,&operations,&nativeJobs,&outbounds,&secrets,&expiry)
	if err != nil || notifications!=1 || operations!=1 || nativeJobs!=1 || outbounds!=1 || secrets!=1 || expiry!=1 { t.Fatalf("READY/replay exact effects n=%d op=%d job=%d outbound=%d secret=%d expiry=%d err=%v",notifications,operations,nativeJobs,outbounds,secrets,expiry,err) }
	// Pair enforcement and irreversible provenance remain true even for the fixture owner.
	oaNotificationReject(t,tx,`UPDATE inbox.order_payment_notifications SET operation_id=NULL WHERE id=$1`,result.NotificationID)
	oaNotificationReject(t,tx,`UPDATE inbox.order_payment_notifications SET state='VOID',void_stage='before_c3',void_code='capability',operation_id=NULL,outbound_id=NULL WHERE id=$1`,result.NotificationID)
	block, err := aes.NewCipher(m.key)
	if err != nil { t.Fatal(err) }
	aead, err := cipher.NewGCM(block)
	if err != nil { t.Fatal(err) }
	// Independent existing frozen AAD, not a production codec used as its own oracle.
	aad, _ := json.Marshal([]string{"livecommerce/meta-outbound/v1",e.tenant(),e.store(),outbound,"dm",displayKey})
	display, err := aead.Open(nil,nonce,displayCT,aad)
	if err != nil { t.Fatal("display ciphertext did not authenticate native tenant/store/outbound") }
	defer clear(display)
	if bytes.Contains(display,[]byte(credential)) || bytes.Contains(display,[]byte(buyerLink)) { t.Fatal("display payload retained an order credential") }
	if _, err = open.OpenSend(e.tenant(),e.store(),randomUUID(),enc,sealed); err == nil { t.Fatal("dispatch ciphertext was not operation-bound") }
	secret, err := open.OpenSend(e.tenant(),e.store(),operation,enc,sealed)
	if err != nil { t.Fatal("actual HPKE dispatch output did not open") }
	defer clear(secret)
	var send struct { PSID string `json:"recipient_psid"`; Text string `json:"text"` }
	if err = json.Unmarshal(secret,&send); err != nil || send.PSID!=psid || send.Text!=strings.Replace(contextWire.TemplateBody,"{{連結}}",buyerLink,1) { t.Fatal("dispatch copy must contain the real signed inbound recipient and one fixed-template order URL") }
	if err = tx.Rollback(context.Background()); err != nil { t.Fatal(err) }
	oaBeginZeroEffects(t,e,a.AttemptID)
	var remaining int
	err = e.p.f.owner.QueryRow(context.Background(),`SELECT
	 (SELECT count(*) FROM inbox.order_payment_notifications WHERE create_attempt_id=$1)+
	 (SELECT count(*) FROM integration.operations WHERE id=$2)+
	 (SELECT count(*) FROM river.river_job WHERE id=$3)+
	 (SELECT count(*) FROM inbox.outbound_messages WHERE id=$4)+
	 (SELECT count(*) FROM inbox.send_secrets WHERE operation_id=$2)`,a.AttemptID,operation,job,outbound).Scan(&remaining)
	if err != nil || remaining!=0 { t.Fatalf("outer native transaction rollback leaked READY artifacts: %d err=%v",remaining,err) }
}
