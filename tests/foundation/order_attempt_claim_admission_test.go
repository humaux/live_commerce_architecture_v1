// Purpose: REAL_PG C1 claim admission, frozen origins and legacy reservation isolation.
// Depends on: native migrated claims/ops tables, lbuNew and oaFactsCandidate fixtures.
// Used by: OA1 CI red/green gates; no provider or C2 execution is exercised here.
// Invariants: I01/I02/I08/I14; owner/hash binding, exact deadline, no origin collapse.
package foundation_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type oaClaimResult struct {
	RequestID *string `json:"request_id"`
	Reason    string  `json:"reason"`
	Lines     []struct {
		Ordinal        int    `json:"ordinal"`
		OriginID       string `json:"origin_id"`
		BundleID       string `json:"bundle_id"`
		OfferID        string `json:"offer_id"`
		SKUID          string `json:"sku_id"`
		Quantity       int64  `json:"quantity"`
		LivePriceMinor *int64 `json:"live_price_minor"`
	} `json:"lines"`
}

func oaClaimSelection(e *lbuEnv, bundles, offers []string, conversation string) map[string]any {
	selection := oaFactsSelection(e)
	items := make([]map[string]any, len(bundles))
	for i := range bundles {
		items[i] = map[string]any{"ordinal": i + 1, "origin_id": randomUUID(), "bundle_id": bundles[i], "offer_id": offers[i], "sku_id": e.money, "quantity": 2}
	}
	selection["items"] = items
	var thread any
	if conversation != "" {
		thread = conversation
	}
	selection["for"] = map[string]any{"bundle_ids": bundles, "conversation_id": thread}
	return selection
}

func oaClaimCall(tx pgx.Tx, attempt string, selection map[string]any) (oaClaimResult, error) {
	raw, err := json.Marshal(selection)
	if err != nil {
		return oaClaimResult{}, err
	}
	var output []byte
	err = tx.QueryRow(context.Background(), `SELECT claims.order_attempt_admission_origins($1::uuid,$2::jsonb)`, attempt, string(raw)).Scan(&output)
	var out oaClaimResult
	if err == nil {
		err = json.Unmarshal(output, &out)
	}
	return out, err
}

// A native unfinished C1 candidate with an old admitted_at proves the grant does
// not start a fresh 15-minute TTL when the helper is reached late.
func oaClaimLateCandidate(t *testing.T, e *lbuEnv, selection map[string]any, age string) (pgx.Tx, string) {
	t.Helper()
	tx, err := e.p.f.owner.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	attempt, scope, owner := randomUUID(), randomUUID(), randomUUID()
	raw, err := json.Marshal(selection)
	if err != nil {
		t.Fatal(err)
	}
	oaStateExec(t, tx, `SELECT set_config('app.tenant_id',$1,true),set_config('app.store_id',$2,true),
	 set_config('app.principal_id',$3,true),set_config('app.buyer_id','',true),set_config('app.buyer_session_id','',true)`, e.tenant(), e.store(), e.p.f.principalA)
	oaStateExec(t, tx, `INSERT INTO buyer.owners(id,tenant_id,store_id) VALUES($1,$2,$3)`, owner, e.tenant(), e.store())
	oaStateExec(t, tx, `INSERT INTO ops.order_attempt_creators(attempt_id,tenant_id,store_id,buyer_owner_id) VALUES($1,$2,$3,$4)`, attempt, e.tenant(), e.store(), owner)
	oaStateExec(t, tx, `INSERT INTO ops.order_attempt_scopes(scope_id,tenant_id,store_id,principal_id,current_generation) VALUES($1,$2,$3,$4,1)`, scope, e.tenant(), e.store(), e.p.f.principalA)
	oaStateExec(t, tx, `INSERT INTO ops.order_attempts(attempt_id,tenant_id,store_id,scope_id,intent_generation,
	 transport_key_digest,request_hash,request_schema_version,buyer_owner_id,admission_provenance,admitted_at,execute_by,state)
	 VALUES($1,$2,$3,$4,1,sha256($1::uuid::text::bytea),sha256(convert_to(($6::jsonb)::text,'UTF8')),1,$5,'{}',
	 statement_timestamp()-$7::interval,statement_timestamp()-$7::interval+interval '15 minutes','PENDING')`, attempt, e.tenant(), e.store(), scope, owner, string(raw), age)
	oaStateExec(t, tx, `SET LOCAL ROLE commerce_order_attempt_owner`)
	return tx, attempt
}

func oaClaimReject(t *testing.T, tx pgx.Tx, call func(pgx.Tx) error, code, message string) {
	t.Helper()
	probe, err := tx.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	err = call(probe)
	rollbackErr := probe.Rollback(context.Background())
	var pe *pgconn.PgError
	if !errors.As(err, &pe) || pe.Code != code || (message != "" && pe.Message != message) || rollbackErr != nil {
		t.Fatalf("claim refusal want %s/%s: err=%v rollback=%v", code, message, err, rollbackErr)
	}
}

func TestOrderAttemptClaimAdmissionOriginsDeadlineAndLegacyFence(t *testing.T) {
	e := lbuNew(t)
	_, first, b1, conversation := e.scenario()
	session, second := e.session("B2", ltgLive+700, 5)
	b2 := e.h.accepted(t, session, "", "second-"+t04Tag(), "B2+2").BundleID
	e.h.closeWindow(t, session)
	e.linkPeer(b2, conversation)
	// Existing granted claims keep their prices after pause (0158).
	mustExec(t, e.p.f.owner, `UPDATE live.offers SET active=false WHERE id=$1`, first.ID)
	selection := oaClaimSelection(e, []string{b1, b2}, []string{first.ID, second.ID}, conversation)
	tx, attempt := oaFactsCandidate(t, e, selection)
	out, err := oaClaimCall(tx, attempt, selection)
	if err != nil {
		t.Fatal(err)
	}
	if out.RequestID == nil || *out.RequestID != attempt || out.Reason != "" || len(out.Lines) != 2 {
		t.Fatalf("claim admission receipt: %+v", out)
	}
	for i, price := range []int64{ltgLive, ltgLive + 700} {
		line := out.Lines[i]
		item := selection["items"].([]map[string]any)[i]
		if line.Ordinal != i+1 || line.OriginID != item["origin_id"] || line.BundleID != item["bundle_id"] || line.OfferID != item["offer_id"] || line.SKUID != e.money || line.Quantity != 2 || line.LivePriceMinor == nil || *line.LivePriceMinor != price {
			t.Fatalf("per-origin price lost: %+v", line)
		}
	}
	oaStateExec(t, tx, `RESET ROLE`)
	var reservations, grants int
	if err := tx.QueryRow(context.Background(), `SELECT
	 (SELECT count(*) FROM inbox.order_for_buyer r JOIN ops.order_attempts a ON a.attempt_id=r.create_attempt_id
	 WHERE a.attempt_id=$1 AND r.request_id=a.attempt_id AND r.buyer_id=a.buyer_owner_id AND r.tenant_id=a.tenant_id AND r.store_id=a.store_id
	 AND r.expires_at=a.execute_by AND r.request_hash=a.request_hash AND r.state='pending'),
	 (SELECT count(*) FROM claims.merchant_origin_grants g JOIN ops.order_attempts a ON a.attempt_id=g.create_attempt_id
	 WHERE a.attempt_id=$1 AND g.request_id=a.attempt_id AND g.buyer_id=a.buyer_owner_id AND g.expires_at=a.execute_by)`, attempt).Scan(&reservations, &grants); err != nil || reservations != 2 || grants != 2 {
		t.Fatalf("exact owner/deadline/count binding: reservations=%d grants=%d err=%v", reservations, grants, err)
	}
	// Storefront must not borrow the still-unbound attempt grant, even for its owner.
	oaStateExec(t, tx, `SELECT set_config('app.principal_id','',true),
	 set_config('app.buyer_id',(SELECT buyer_owner_id::text FROM ops.order_attempts WHERE attempt_id=$1),true),
	 set_config('app.buyer_session_id',$2,true)`, attempt, randomUUID())
	oaStateExec(t, tx, `SET LOCAL ROLE commerce_buyer_runtime`)
	var legacyPrices int
	if err := tx.QueryRow(context.Background(), `SELECT count(*) FROM claims.live_prices(ARRAY[$1::uuid],ARRAY[$2::uuid],ARRAY[$3::uuid],ARRAY[2::bigint])`, b1, first.ID, e.money).Scan(&legacyPrices); err != nil || legacyPrices != 0 {
		t.Fatalf("legacy quote borrowed attempt grant: %d/%v", legacyPrices, err)
	}
	oaStateExec(t, tx, `RESET ROLE`)
	oaStateExec(t, tx, `SELECT set_config('app.principal_id',$1,true),set_config('app.buyer_id','',true),set_config('app.buyer_session_id','',true)`, e.p.f.principalA)
	// Age the legacy timestamps without moving the immutable attempt deadline.
	oaStateExec(t, tx, `UPDATE inbox.order_for_buyer SET updated_at=clock_timestamp()-interval '2 hours' WHERE request_id=$1`, attempt)
	oaStateExec(t, tx, `SET LOCAL ROLE commerce_runtime`)
	oaClaimReject(t, tx, func(probe pgx.Tx) error {
		_, err := probe.Exec(context.Background(), `SELECT * FROM claims.for_buyer_begin(sha256('legacy'::bytea),sha256('body'::bytea),$1,ARRAY[$2::uuid],$3)`, conversation, b1, randomUUID())
		return err
	}, "PT409", "bundle_already_ordered")
	for _, sql := range []string{`SELECT claims.for_buyer_release($1)`, `SELECT * FROM claims.for_buyer_finish($1,gen_random_uuid())`} {
		oaClaimReject(t, tx, func(probe pgx.Tx) error {
			_, err := probe.Exec(context.Background(), sql, attempt)
			return err
		}, "PT409", "attempt_owned_reservation")
	}
}

func TestOrderAttemptClaimAdmissionAuthorityAndHash(t *testing.T) {
	e := lbuNew(t)
	_, offer, bundle, conversation := e.scenario()
	selection := oaClaimSelection(e, []string{bundle}, []string{offer.ID}, conversation)
	tx, attempt := oaFactsCandidate(t, e, selection)
	changed := oaClaimSelection(e, []string{bundle}, []string{offer.ID}, conversation)
	oaClaimReject(t, tx, func(probe pgx.Tx) error { _, err := oaClaimCall(probe, attempt, changed); return err }, "PT409", "idempotency_conflict")
	oaStateExec(t, tx, `SELECT set_config('app.store_id',$1,true)`, randomUUID())
	oaClaimReject(t, tx, func(probe pgx.Tx) error { _, err := oaClaimCall(probe, attempt, selection); return err }, "PT404", "attempt_not_found")
	oaStateExec(t, tx, `SELECT set_config('app.store_id',$1,true)`, e.store())
	oaStateExec(t, tx, `SET LOCAL ROLE commerce_runtime`)
	oaClaimReject(t, tx, func(probe pgx.Tx) error { _, err := oaClaimCall(probe, attempt, selection); return err }, "42501", "")
	oaStateExec(t, tx, `RESET ROLE`)
	var ok bool
	err := tx.QueryRow(context.Background(), `SELECT p.prosecdef AND p.proconfig=ARRAY['search_path=pg_catalog']::text[]
	 AND p.proowner='commerce_claims_writer'::regrole
	 AND (SELECT array_agg(r.rolname::text ORDER BY r.rolname) FROM aclexplode(p.proacl) x JOIN pg_roles r ON r.oid=x.grantee
	 WHERE x.privilege_type='EXECUTE' AND x.grantee<>p.proowner)=ARRAY['commerce_order_attempt_owner']::text[]
	 AND NOT EXISTS(SELECT 1 FROM aclexplode(p.proacl) x WHERE x.grantee=0)
	 FROM pg_proc p WHERE p.oid='claims.order_attempt_admission_origins(uuid,jsonb)'::regprocedure`).Scan(&ok)
	if err != nil || !ok {
		t.Fatalf("internal claims helper ACL: %v/%v", ok, err)
	}
}

func TestOrderAttemptClaimAdmissionFailClosedAndExactOrigin(t *testing.T) {
	for _, scenario := range []string{"no_conversation", "bundle_buyer_unverified", "permission", "no_live_price", "superseded_origin", "bundle_buyer_mismatch", "foreign_origin", "quantity", "bundles", "duplicate_origin"} {
		t.Run(scenario, func(t *testing.T) {
			e := lbuNew(t)
			session, offer, bundle, conversation := e.scenario()
			if scenario == "no_conversation" {
				conversation = ""
			}
			if scenario == "bundle_buyer_unverified" {
				mustExec(t, e.p.f.owner, `DELETE FROM inbox.bundle_peers WHERE bundle_id=$1`, bundle)
			}
			if scenario == "bundle_buyer_mismatch" {
				conversation = lcConversation(t, e.p.f, e.tenant(), e.store(), "page")
			}
			if scenario == "permission" {
				mustExec(t, e.p.f.owner, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='live:manage'`, e.tenant(), e.store(), e.p.f.principalA)
			}
			if scenario == "no_live_price" {
				mustExec(t, e.p.f.owner, `UPDATE live.offers SET live_price_minor=NULL WHERE id=$1`, offer.ID)
			}
			if scenario == "superseded_origin" {
				mustExec(t, e.p.f.owner, `UPDATE live.offers SET active=false WHERE id=$1`, offer.ID)
				winner := e.h.livePriceOffer(t, session, "B2", e.money, 5, ltgLive+700)
				// Owner fixture inserts a second native claim for the same bundle/SKU.
				mustExec(t, e.p.f.owner, `INSERT INTO claims.lines(tenant_id,store_id,session_id,bundle_id,offer_id,sku_id,quantity,version)
				 SELECT tenant_id,store_id,session_id,bundle_id,$2,sku_id,2,1 FROM claims.lines WHERE bundle_id=$1 AND offer_id=$3`, bundle, winner.ID, offer.ID)
			}
			selection := oaClaimSelection(e, []string{bundle}, []string{offer.ID}, conversation)
			items := selection["items"].([]map[string]any)
			switch scenario {
			case "foreign_origin":
				items[0]["offer_id"] = randomUUID()
			case "quantity":
				items[0]["quantity"] = 3
			case "bundles":
				selection["for"].(map[string]any)["bundle_ids"] = []string{}
			case "duplicate_origin":
				selection["items"] = append(items, map[string]any{"ordinal": 2, "origin_id": randomUUID(), "bundle_id": bundle, "offer_id": offer.ID, "sku_id": e.money, "quantity": 2})
			}
			tx, attempt := oaFactsCandidate(t, e, selection)
			if scenario == "no_conversation" || scenario == "bundle_buyer_unverified" || scenario == "permission" || scenario == "no_live_price" || scenario == "superseded_origin" {
				out, err := oaClaimCall(tx, attempt, selection)
				if err != nil || out.Reason != scenario || len(out.Lines) != 1 || out.Lines[0].LivePriceMinor != nil {
					t.Fatalf("fallback %s: %+v err=%v", scenario, out, err)
				}
				oaStateExec(t, tx, `RESET ROLE`)
				var n int
				if err := tx.QueryRow(context.Background(), `SELECT count(*) FROM claims.merchant_origin_grants WHERE create_attempt_id=$1`, attempt).Scan(&n); err != nil || n != 0 {
					t.Fatalf("ineligible grant: %d/%v", n, err)
				}
				return
			}
			code := "PT400"
			if scenario == "bundle_buyer_mismatch" || scenario == "quantity" || scenario == "duplicate_origin" {
				code = "PT409"
			}
			if scenario == "foreign_origin" {
				code = "PT404"
			}
			oaClaimReject(t, tx, func(probe pgx.Tx) error { _, err := oaClaimCall(probe, attempt, selection); return err }, code, "")
			oaStateExec(t, tx, `RESET ROLE`)
			var n int
			if err := tx.QueryRow(context.Background(), `SELECT count(*) FROM inbox.order_for_buyer WHERE create_attempt_id=$1`, attempt).Scan(&n); err != nil || n != 0 {
				t.Fatalf("refusal wrote reservations: %d/%v", n, err)
			}
		})
	}
}

func TestOrderAttemptClaimAdmissionLateDeadline(t *testing.T) {
	for _, age := range []string{"14 minutes", "16 minutes"} {
		t.Run(age, func(t *testing.T) {
			e := lbuNew(t)
			_, offer, bundle, conversation := e.scenario()
			selection := oaClaimSelection(e, []string{bundle}, []string{offer.ID}, conversation)
			tx, attempt := oaClaimLateCandidate(t, e, selection, age)
			if age == "16 minutes" {
				oaClaimReject(t, tx, func(probe pgx.Tx) error { _, err := oaClaimCall(probe, attempt, selection); return err }, "PT409", "attempt_expired")
				return
			}
			if _, err := oaClaimCall(tx, attempt, selection); err != nil {
				t.Fatal(err)
			}
			oaStateExec(t, tx, `RESET ROLE`)
			var ok bool
			err := tx.QueryRow(context.Background(), `SELECT r.expires_at=a.execute_by AND g.expires_at=a.execute_by
			 AND g.expires_at<clock_timestamp()+interval '2 minutes'
			 FROM ops.order_attempts a JOIN inbox.order_for_buyer r ON r.create_attempt_id=a.attempt_id
			 JOIN claims.merchant_origin_grants g ON g.create_attempt_id=a.attempt_id WHERE a.attempt_id=$1`, attempt).Scan(&ok)
			if err != nil || !ok {
				t.Fatalf("late C1 extended deadline: %v/%v", ok, err)
			}
		})
	}
}

// Catalog-only lines share A16's 1000-unit bound even though an individual
// comment claim remains bounded by its smaller native claims.lines allowance.
func TestOrderAttemptClaimAdmissionCatalogQuantityBoundary(t *testing.T) {
	for _, quantity := range []int{1000, 1001} {
		t.Run(fmt.Sprint(quantity), func(t *testing.T) {
			e := lbuNew(t)
			selection := oaFactsSelection(e)
			selection["for"] = map[string]any{"bundle_ids": []string{}, "conversation_id": nil}
			selection["items"] = []map[string]any{{"sku_id": e.money, "quantity": quantity, "ordinal": 1, "origin_id": nil}}
			tx, attempt := oaFactsCandidate(t, e, selection)
			if quantity == 1001 {
				oaClaimReject(t, tx, func(probe pgx.Tx) error { _, err := oaClaimCall(probe, attempt, selection); return err }, "PT400", "invalid_selection")
				return
			}
			out, err := oaClaimCall(tx, attempt, selection)
			if err != nil || len(out.Lines) != 1 || out.Lines[0].Quantity != 1000 || out.Lines[0].LivePriceMinor != nil {
				t.Fatalf("catalog boundary must survive claim resolver without a live price: %+v err=%v", out, err)
			}
		})
	}
}
