// Purpose: REAL_PG gates for OA1 C1 subject discovery and exact private-owner binding.
// Depends on: native lbu fixtures, 0170 candidate records and 0172 admission accessor.
// Used by: OA1 CI; no local execution, disabled constraints or synthetic bearer identity.
// Invariants: I01/I02/I09/I11; explicit links only, all PII subjects indexed, no runtime helpers.
package foundation_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func oaSubjectTx(t *testing.T, e *lbuEnv) pgx.Tx {
	t.Helper()
	tx, err := e.p.f.owner.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	oaStateExec(t, tx, `SELECT set_config('app.tenant_id',$1,true),set_config('app.store_id',$2,true),set_config('app.principal_id',$3,true)`, e.tenant(), e.store(), e.p.f.principalA)
	return tx
}

func oaSubjectCall(tx pgx.Tx, selection map[string]any) (map[string]any, error) {
	raw, err := json.Marshal(selection)
	if err != nil {
		return nil, err
	}
	var data []byte
	err = tx.QueryRow(context.Background(), `SELECT customers.order_attempt_sources($1::jsonb)`, string(raw)).Scan(&data)
	var out map[string]any
	if err == nil {
		err = json.Unmarshal(data, &out)
	}
	return out, err
}

func oaSubjectReject(t *testing.T, tx pgx.Tx, query string, args []any, code, message string) {
	t.Helper()
	sp, err := tx.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_, err = sp.Exec(context.Background(), query, args...)
	rb := sp.Rollback(context.Background())
	var pe *pgconn.PgError
	if !errors.As(err, &pe) || pe.Code != code || (message != "" && pe.Message != message) || rb != nil {
		t.Fatalf("subject refusal want %s/%s: err=%v rollback=%v", code, message, err, rb)
	}
}

func oaSubjectJSON(t *testing.T, selection map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(selection)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestOrderAttemptSubjectsExplicitNativeLink(t *testing.T) {
	e := lbuNew(t)
	tx := oaSubjectTx(t, e)
	owner, conv := randomUUID(), randomUUID()
	oaStateExec(t, tx, `INSERT INTO buyer.owners(id,tenant_id,store_id) VALUES($1,$2,$3)`, owner, e.tenant(), e.store())
	oaStateExec(t, tx, `INSERT INTO social.conversations(id,tenant_id,store_id,app_id,object,asset_id,peer_key) VALUES($1,$2,$3,'123456789','page','123456789',repeat(replace($1::uuid::text,'-',''),2))`, conv, e.tenant(), e.store())
	oaStateExec(t, tx, `INSERT INTO inbox.conversation_state(tenant_id,store_id,conversation_id,customer_id) VALUES($1,$2,$3,$4)`, e.tenant(), e.store(), conv, owner)
	selection := oaFactsSelection(e)
	selection["for"] = map[string]any{"bundle_ids": []string{}, "conversation_id": conv, "customer_id": owner}
	oaStateExec(t, tx, `SET LOCAL ROLE commerce_order_attempt_owner`)
	out, err := oaSubjectCall(tx, selection)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"buyer_owner_id":"` + owner + `","subjects":[{"kind":"buyer_owner","id":"` + owner + `"},{"kind":"customer","id":"` + owner + `"}]}`
	var expected map[string]any
	if err := json.Unmarshal([]byte(want), &expected); err != nil {
		t.Fatal(err)
	}
	if oaSubjectJSON(t, out) != oaSubjectJSON(t, expected) {
		t.Fatalf("exact opaque source DTO: got=%v", out)
	}
	// Omitting the explicit customer never infers identity from the conversation or contact.
	selection["for"] = map[string]any{"bundle_ids": []string{}, "conversation_id": conv}
	out, err = oaSubjectCall(tx, selection)
	if err != nil || out["buyer_owner_id"] != nil || len(out["subjects"].([]any)) != 0 || len(out) != 2 {
		t.Fatalf("unlinked contact inferred identity: out=%v err=%v", out, err)
	}
	selection["for"] = map[string]any{"bundle_ids": []string{}, "conversation_id": randomUUID(), "customer_id": owner}
	oaSubjectReject(t, tx, `SELECT customers.order_attempt_sources($1::jsonb)`, []any{oaSubjectJSON(t, selection)}, "PT404", "not_found")
	selection["for"] = map[string]any{"bundle_ids": []string{}, "conversation_id": randomUUID()}
	oaSubjectReject(t, tx, `SELECT customers.order_attempt_sources($1::jsonb)`, []any{oaSubjectJSON(t, selection)}, "PT404", "not_found")
	selection["for"] = map[string]any{"bundle_ids": []string{}, "conversation_id": conv, "customer_id": randomUUID()}
	oaSubjectReject(t, tx, `SELECT customers.order_attempt_sources($1::jsonb)`, []any{oaSubjectJSON(t, selection)}, "PT404", "not_found")
	selection["for"] = map[string]any{"bundle_ids": []string{}, "conversation_id": nil, "customer_id": owner}
	oaStateExec(t, tx, `RESET ROLE`)
	oaStateExec(t, tx, `UPDATE buyer.owners SET active=false WHERE id=$1`, owner)
	oaStateExec(t, tx, `SET LOCAL ROLE commerce_order_attempt_owner`)
	oaSubjectReject(t, tx, `SELECT customers.order_attempt_sources($1::jsonb)`, []any{oaSubjectJSON(t, selection)}, "PT422", "ERASED")
}

func TestOrderAttemptSubjectsScopeAndSelection(t *testing.T) {
	e := lbuNew(t)
	tx := oaSubjectTx(t, e)
	owner := randomUUID()
	// lbu's cloned fixture replaces tenantA/storeA1, but keeps the original storeA2.
	// Read that store's actual tenant; mixing the cloned tenant with it fails its FK.
	oaStateExec(t, tx, `INSERT INTO buyer.owners(id,tenant_id,store_id)
	 SELECT $1,tenant_id,id FROM control.stores WHERE id=$2`, owner, e.p.f.storeA2)
	oaStateExec(t, tx, `SET LOCAL ROLE commerce_order_attempt_owner`)
	selection := oaFactsSelection(e)
	selection["for"] = map[string]any{"bundle_ids": []string{}, "conversation_id": nil, "customer_id": owner}
	oaSubjectReject(t, tx, `SELECT customers.order_attempt_sources($1::jsonb)`, []any{oaSubjectJSON(t, selection)}, "PT404", "not_found")
	for _, mutate := range []func(map[string]any){
		func(s map[string]any) { s["customer"].(map[string]any)["owner_id"] = owner },
		func(s map[string]any) { s["customer"].(map[string]any)["name"] = true },
		func(s map[string]any) { s["for"].(map[string]any)["customer_id"] = "not-a-uuid" },
		func(s map[string]any) { s["delivery"].(map[string]any)["subject_id"] = owner },
		func(s map[string]any) { s["items"].([]map[string]any)[0]["quantity"] = 1001 },
		func(s map[string]any) { s["send_payment_link"] = "true" },
	} {
		selection = oaFactsSelection(e)
		mutate(selection)
		oaSubjectReject(t, tx, `SELECT customers.order_attempt_sources($1::jsonb)`, []any{oaSubjectJSON(t, selection)}, "PT400", "invalid_selection")
	}
	selection = oaFactsSelection(e)
	oaStateExec(t, tx, `SELECT set_config('app.principal_id',$1,true)`, randomUUID())
	oaSubjectReject(t, tx, `SELECT customers.order_attempt_sources($1::jsonb)`, []any{oaSubjectJSON(t, selection)}, "PT403", "forbidden")
}

func TestOrderAttemptSubjectsDiscoveryDoesNotLockOwner(t *testing.T) {
	e := lbuNew(t)
	owner := randomUUID()
	mustExec(t, e.p.f.owner, `INSERT INTO buyer.owners(id,tenant_id,store_id) VALUES($1,$2,$3)`, owner, e.tenant(), e.store())
	t.Cleanup(func() { mustExec(t, e.p.f.owner, `DELETE FROM buyer.owners WHERE id=$1`, owner) })
	lock := oaSubjectTx(t, e)
	oaStateExec(t, lock, `SELECT id FROM buyer.owners WHERE id=$1 FOR UPDATE`, owner)
	reader := oaSubjectTx(t, e)
	oaStateExec(t, reader, `SET LOCAL lock_timeout='100ms'`)
	oaStateExec(t, reader, `SET LOCAL ROLE commerce_order_attempt_owner`)
	selection := oaFactsSelection(e)
	selection["for"].(map[string]any)["customer_id"] = owner
	out, err := oaSubjectCall(reader, selection)
	if err != nil || out["buyer_owner_id"] != owner {
		t.Fatalf("unlocked source discovery waited on owner lock: out=%v err=%v", out, err)
	}
}

func oaSubjectCandidate(t *testing.T, e *lbuEnv, selection map[string]any, owner string, private any) (pgx.Tx, string) {
	t.Helper()
	tx := oaSubjectTx(t, e)
	attempt, scope := randomUUID(), randomUUID()
	provenance, err := json.Marshal(map[string]any{"private_owner": private})
	if err != nil {
		t.Fatal(err)
	}
	oaStateExec(t, tx, `INSERT INTO ops.order_attempt_creators(attempt_id,tenant_id,store_id,buyer_owner_id) VALUES($1,$2,$3,$4)`, attempt, e.tenant(), e.store(), owner)
	oaStateExec(t, tx, `INSERT INTO ops.order_attempt_scopes(scope_id,tenant_id,store_id,principal_id,current_generation) VALUES($1,$2,$3,$4,1)`, scope, e.tenant(), e.store(), e.p.f.principalA)
	oaStateExec(t, tx, `INSERT INTO ops.order_attempts(attempt_id,tenant_id,store_id,scope_id,intent_generation,transport_key_digest,request_hash,request_schema_version,buyer_owner_id,admission_provenance,admitted_at,execute_by,state)
	 VALUES($1,$2,$3,$4,1,sha256($1::uuid::text::bytea),sha256(convert_to(($6::jsonb)::text,'UTF8')),1,$5,$7::jsonb,statement_timestamp(),statement_timestamp()+interval '15 minutes','PENDING')`, attempt, e.tenant(), e.store(), scope, owner, oaSubjectJSON(t, selection), string(provenance))
	oaStateExec(t, tx, `SET LOCAL ROLE commerce_order_attempt_owner`)
	return tx, attempt
}

func TestOrderAttemptSubjectsPrivateOwnerExactBinding(t *testing.T) {
	e := lbuNew(t)
	selection := oaFactsSelection(e)
	owner := randomUUID()
	tx, attempt := oaSubjectCandidate(t, e, selection, owner, true)
	oaStateExec(t, tx, `SELECT buyer.order_attempt_bind_owner($1::uuid,$2::jsonb)`, attempt, oaSubjectJSON(t, selection))
	oaStateExec(t, tx, `RESET ROLE`)
	var count int
	if err := tx.QueryRow(context.Background(), `SELECT count(*) FROM buyer.owners WHERE id=$1 AND tenant_id=$2 AND store_id=$3 AND active`, owner, e.tenant(), e.store()).Scan(&count); err != nil || count != 1 {
		t.Fatalf("private owner exact binding: count=%d err=%v", count, err)
	}
	oaStateExec(t, tx, `SET LOCAL ROLE commerce_order_attempt_owner`)
	oaSubjectReject(t, tx, `SELECT buyer.order_attempt_bind_owner($1::uuid,$2::jsonb)`, []any{attempt, oaSubjectJSON(t, selection)}, "PT409", "owner_conflict")
	oaStateExec(t, tx, `SELECT set_config('app.store_id',$1,true)`, e.p.f.storeA2)
	oaSubjectReject(t, tx, `SELECT buyer.order_attempt_bind_owner($1::uuid,$2::jsonb)`, []any{attempt, oaSubjectJSON(t, selection)}, "PT404", "attempt_not_found")
	oaStateExec(t, tx, `SELECT set_config('app.store_id',$1,true)`, e.store())
	selection["customer"].(map[string]any)["name"] = "Changed synthetic buyer"
	oaSubjectReject(t, tx, `SELECT buyer.order_attempt_bind_owner($1::uuid,$2::jsonb)`, []any{attempt, oaSubjectJSON(t, selection)}, "PT409", "idempotency_conflict")
}

func TestOrderAttemptSubjectsActualRuntimeCannotCall(t *testing.T) {
	e := lbuNew(t)
	tx, err := e.p.f.runtime.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	// A runtime can set arbitrary GUCs; those cannot grant access to internal helpers.
	oaStateExec(t, tx, `SELECT set_config('app.tenant_id',$1,true),set_config('app.store_id',$2,true),set_config('app.principal_id',$3,true)`, e.tenant(), e.store(), e.p.f.principalA)
	raw := oaSubjectJSON(t, oaFactsSelection(e))
	oaSubjectReject(t, tx, `SELECT customers.order_attempt_sources($1::jsonb)`, []any{raw}, "42501", "")
	oaSubjectReject(t, tx, `SELECT buyer.order_attempt_bind_owner($1::uuid,$2::jsonb)`, []any{randomUUID(), raw}, "42501", "")
	oaSubjectReject(t, tx, `SELECT inbox.order_attempt_customer_link($1::uuid,$2::uuid)`, []any{randomUUID(), randomUUID()}, "42501", "")
}

func TestOrderAttemptSubjectsExistingOwnerAndProvenance(t *testing.T) {
	for _, tc := range []struct {
		name    string
		private any
		present bool
		active  bool
		code    string
		message string
	}{
		{"linked", false, true, true, "", ""},
		{"missing", false, false, false, "PT404", "not_found"},
		{"erased", false, true, false, "PT422", "ERASED"},
		{"collision", true, true, true, "PT409", "owner_conflict"},
		{"missing_provenance", nil, false, false, "PT409", "invalid_admission_provenance"},
		{"string_provenance", "true", false, false, "PT409", "invalid_admission_provenance"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := lbuNew(t)
			selection := oaFactsSelection(e)
			owner := randomUUID()
			tx, attempt := oaSubjectCandidate(t, e, selection, owner, tc.private)
			if tc.present {
				oaStateExec(t, tx, `RESET ROLE`)
				oaStateExec(t, tx, `INSERT INTO buyer.owners(id,tenant_id,store_id,active) VALUES($1,$2,$3,$4)`, owner, e.tenant(), e.store(), tc.active)
				oaStateExec(t, tx, `SET LOCAL ROLE commerce_order_attempt_owner`)
			}
			if tc.code != "" {
				oaSubjectReject(t, tx, `SELECT buyer.order_attempt_bind_owner($1::uuid,$2::jsonb)`, []any{attempt, oaSubjectJSON(t, selection)}, tc.code, tc.message)
			} else {
				oaStateExec(t, tx, `SELECT buyer.order_attempt_bind_owner($1::uuid,$2::jsonb)`, attempt, oaSubjectJSON(t, selection))
			}
		})
	}
}

func TestOrderAttemptSubjectsDefinerACL(t *testing.T) {
	f := fixture(t)
	for _, tc := range []struct{ signature, owner, caller string }{
		{"customers.order_attempt_sources(jsonb)", "commerce_privacy_writer", "commerce_order_attempt_owner"},
		{"buyer.order_attempt_bind_owner(uuid,jsonb)", "commerce_buyer_writer", "commerce_order_attempt_owner"},
		{"inbox.order_attempt_customer_link(uuid,uuid)", "commerce_inbox_writer", "commerce_privacy_writer"},
	} {
		var safe bool
		err := f.owner.QueryRow(context.Background(), `SELECT p.prosecdef AND r.rolname=$2 AND NOT r.rolcanlogin
		 AND p.proconfig @> ARRAY['search_path=pg_catalog'] AND has_function_privilege($3,p.oid,'EXECUTE')
		 AND NOT EXISTS(SELECT 1 FROM aclexplode(p.proacl) a WHERE a.grantee=0 AND a.privilege_type='EXECUTE')
		 FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid=to_regprocedure($1)`, tc.signature, tc.owner, tc.caller).Scan(&safe)
		if err != nil || !safe {
			t.Fatalf("definer authority %s: safe=%v err=%v", tc.signature, safe, err)
		}
		for _, role := range []string{"commerce_runtime", "commerce_claims_worker", "commerce_buyer_runtime", "commerce_checkout_runtime"} {
			var allowed bool
			if err := f.owner.QueryRow(context.Background(), `SELECT has_function_privilege($1,to_regprocedure($2),'EXECUTE')`, role, tc.signature).Scan(&allowed); err != nil || allowed {
				t.Fatalf("runtime helper authority %s %s: allowed=%v err=%v", role, tc.signature, allowed, err)
			}
		}
	}
}
