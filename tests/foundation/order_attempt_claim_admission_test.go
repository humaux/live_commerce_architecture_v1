// Purpose: REAL_PG C1 claim admission, frozen origins and legacy reservation isolation.
// Depends on: native migrated claims/ops tables, lbuNew and oaFactsCandidate fixtures.
// Used by: OA1 CI red/green gates; no provider or C2 execution is exercised here.
// Invariants: I01/I02/I08/I14; owner/hash binding, exact deadline, no origin collapse.
package foundation_test

import (
	"context"
	"encoding/json"
	"errors"
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
	for _, scenario := range []string{"no_conversation", "bundle_buyer_unverified", "bundle_buyer_mismatch", "foreign_origin", "quantity", "bundles", "duplicate_origin"} {
		t.Run(scenario, func(t *testing.T) {
			e := lbuNew(t)
			_, offer, bundle, conversation := e.scenario()
			if scenario == "no_conversation" {
				conversation = ""
			}
			if scenario == "bundle_buyer_unverified" {
				mustExec(t, e.p.f.owner, `DELETE FROM inbox.bundle_peers WHERE bundle_id=$1`, bundle)
			}
			if scenario == "bundle_buyer_mismatch" {
				conversation = lcConversation(t, e.p.f, e.tenant(), e.store(), "page")
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
			if scenario == "no_conversation" || scenario == "bundle_buyer_unverified" {
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
