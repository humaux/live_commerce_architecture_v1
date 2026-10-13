// Purpose: REAL_PG C1 facts compatibility for attempt-owned line/store materialization.
// Depends on: real Service.Admit, lbuNew merchant fixtures, persisted AEAD plan readback.
// Used by: OA1 CI; owner SQL only reads trusted binding metadata after runtime admission.
// Invariants: I01/I02/I08; nonzero frozen origins, immutable entered-store provenance,
// per-chain validation and enabled chains for every offline CVS payment mode; no C2.
package foundation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"livecommerce/internal/command"
	"livecommerce/internal/fulfillment"
	"livecommerce/internal/orderattempt"
)

const oaMaterialZeroUUID = "00000000-0000-0000-0000-000000000000"

func oaMaterialPlan(t *testing.T, e *lbuEnv, keys *orderattempt.PlanKeys, attempt string) (orderattempt.FrozenPlan, orderattempt.SealedPlan) {
	t.Helper()
	var binding orderattempt.PlanBinding
	var sealed orderattempt.SealedPlan
	var requestHash, factsDigest, planDigest []byte
	err := e.p.f.owner.QueryRow(context.Background(), `SELECT a.attempt_id::text,a.scope_id::text,a.intent_generation,
	 a.tenant_id::text,a.store_id::text,s.principal_id::text,a.buyer_owner_id::text,a.admitted_at,a.execute_by,
	 a.request_hash,a.facts_digest,a.plan_digest,a.plan_key_version,a.plan_ciphertext
	 FROM ops.order_attempts a JOIN ops.order_attempt_scopes s USING(scope_id)
	 WHERE a.attempt_id=$1 AND a.admission_finished`, attempt).
		Scan(&binding.AttemptID, &binding.ScopeID, &binding.Generation, &binding.TenantID, &binding.StoreID,
			&binding.PrincipalID, &binding.BuyerOwnerID, &binding.AdmittedAt, &binding.ExecuteBy,
			&requestHash, &factsDigest, &planDigest, &sealed.KeyVersion, &sealed.Ciphertext)
	if err != nil || len(requestHash) != 32 || len(factsDigest) != 32 || len(planDigest) != 32 {
		t.Fatalf("real sealed-plan sentinel: %v", err)
	}
	binding.Schema = 1
	copy(binding.RequestHash[:], requestHash)
	copy(binding.FactsDigest[:], factsDigest)
	copy(sealed.Digest[:], planDigest)
	plan, err := keys.Open(binding, sealed)
	if err != nil {
		t.Fatalf("real C1 plan decrypt: %v", err)
	}
	return plan, sealed
}

func oaMaterialRequest(t *testing.T, e *lbuEnv, selection map[string]any) orderattempt.AdmissionRequest {
	t.Helper()
	req := oaServiceRequest(t, e)
	var err error
	req.Selection, err = json.Marshal(selection)
	if err != nil {
		t.Fatal(err)
	}
	return req
}

func oaMaterialRefusal(t *testing.T, e *lbuEnv, svc *orderattempt.Service, req orderattempt.AdmissionRequest, state string) {
	t.Helper()
	_, err := svc.Admit(context.Background(), e.token(), e.store(), req)
	var coded *orderattempt.AdmissionError
	if !errors.As(err, &coded) || coded.SQLState() != state || coded.OutcomeUnknown() {
		t.Fatalf("definitive C1 refusal want %s: %v", state, err)
	}
	var generation int64
	var attempts int
	err = e.p.f.owner.QueryRow(context.Background(), `SELECT current_generation,
	 (SELECT count(*) FROM ops.order_attempts a WHERE a.scope_id=s.scope_id)
	 FROM ops.order_attempt_scopes s WHERE scope_id=$1`, req.ExpectedScopeID).Scan(&generation, &attempts)
	if err != nil || generation != req.ExpectedGeneration || attempts != 0 {
		t.Fatalf("refused C1 left authority: generation=%d attempts=%d err=%v", generation, attempts, err)
	}
}

func TestOrderAttemptMaterializationFactsManualOriginsAndReplay(t *testing.T) {
	for _, missing := range []bool{false, true} {
		name := "distinct explicit origins"
		if missing {
			name = "server origin beside explicit origin"
		}
		t.Run(name, func(t *testing.T) {
			e := lbuNew(t)
			selection := oaFactsSelection(e)
			items := selection["items"].([]map[string]any)
			if missing {
				delete(items[0], "origin_id")
			}
			req := oaMaterialRequest(t, e, selection)
			keys := oaServiceKeys(t)
			svc, err := orderattempt.NewService(e.p.f.runtime, keys)
			if err != nil {
				t.Fatal(err)
			}
			first, err := svc.Admit(context.Background(), e.token(), e.store(), req)
			if err != nil || first.State != "PENDING" || first.Replay {
				t.Fatalf("origin C1 positive sentinel: %+v %v", first, err)
			}
			plan, sealed := oaMaterialPlan(t, e, keys, first.AttemptID)
			var facts struct {
				Catalog struct {
					Lines []oaFactsLine `json:"lines"`
				} `json:"catalog"`
				Selection json.RawMessage `json:"selection"`
			}
			if err = json.Unmarshal(plan.Facts, &facts); err != nil || len(facts.Catalog.Lines) != 2 || len(plan.Calculation.Lines) != 2 {
				t.Fatalf("origin plan shape: %v", err)
			}
			for i, line := range facts.Catalog.Lines {
				if !command.ValidID(line.OriginID) || line.OriginID == oaMaterialZeroUUID || line.Ordinal != i+1 || line.SKUID != e.money ||
					line.Quantity != int64(i+2) || plan.Calculation.Lines[i].SubtotalMinor != int64(i+2)*30000 {
					t.Fatalf("nonzero distinct origin %d: %+v", i, line)
				}
				if supplied, ok := items[i]["origin_id"]; ok && line.OriginID != supplied {
					t.Fatalf("supplied origin replaced at ordinal %d", i+1)
				}
			}
			if facts.Catalog.Lines[0].OriginID == facts.Catalog.Lines[1].OriginID {
				t.Fatal("duplicate SKU origins merged")
			}
			var hashMatches bool
			if err = e.p.f.owner.QueryRow(context.Background(), `SELECT request_hash=sha256(convert_to(($2::jsonb)::text,'UTF8'))
			 FROM ops.order_attempts WHERE attempt_id=$1`, first.AttemptID, string(req.Selection)).Scan(&hashMatches); err != nil || !hashMatches {
				t.Fatalf("normalization changed canonical request hash: %v", err)
			}
			var original, frozen any
			if err = json.Unmarshal(req.Selection, &original); err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(facts.Selection, &frozen); err != nil {
				t.Fatal(err)
			}
			originalJSON, _ := json.Marshal(original)
			frozenJSON, _ := json.Marshal(frozen)
			if !bytes.Equal(originalJSON, frozenJSON) {
				t.Fatal("server origin rewrote raw selection")
			}
			replay, err := svc.Admit(context.Background(), e.token(), e.store(), req)
			if err != nil || !replay.Replay || replay.AttemptID != first.AttemptID {
				t.Fatalf("exact-key replay: %+v %v", replay, err)
			}
			_, replaySealed := oaMaterialPlan(t, e, keys, first.AttemptID)
			if replaySealed.Digest != sealed.Digest || !bytes.Equal(replaySealed.Ciphertext, sealed.Ciphertext) {
				t.Fatal("replay regenerated normalized origin or frozen plan")
			}
		})
	}
}

func TestOrderAttemptMaterializationFactsZeroAndAmbiguousOrigins(t *testing.T) {
	for _, name := range []string{"zero manual", "zero claim", "missing duplicate SKU origins"} {
		t.Run(name, func(t *testing.T) {
			e := lbuNew(t)
			selection := oaFactsSelection(e)
			items := selection["items"].([]map[string]any)
			if name == "missing duplicate SKU origins" {
				delete(items[0], "origin_id")
				delete(items[1], "origin_id")
			} else {
				items[0]["origin_id"] = oaMaterialZeroUUID
				if name == "zero claim" {
					_, offer, bundle, conversation := e.scenario()
					items[0]["bundle_id"], items[0]["offer_id"] = bundle, offer.ID
					selection["for"] = map[string]any{"bundle_ids": []string{bundle}, "conversation_id": conversation}
				}
			}
			req := oaMaterialRequest(t, e, selection)
			svc, err := orderattempt.NewService(e.p.f.runtime, oaServiceKeys(t))
			if err != nil {
				t.Fatal(err)
			}
			oaMaterialRefusal(t, e, svc, req, "PT400")
		})
	}
}

func oaMaterialCVS(t *testing.T, e *lbuEnv, kind, code, mode string) map[string]any {
	t.Helper()
	service, _, _ := e.service(kind, "MANUAL", 0)
	e.cvsSettings(tcvAllChains, true, "20000", 20)
	selection := oaFactsSelection(e)
	selection["delivery"] = map[string]any{"market_id": e.p.market.ID, "country": "TW", "service_code": service,
		"cvs": map[string]any{"store_code": code, "store_name": "Synthetic store", "store_address": "Synthetic store address"}}
	selection["payment_mode"] = mode
	return selection
}

func TestOrderAttemptMaterializationFactsCVSCodesAndSource(t *testing.T) {
	for _, mode := range []string{"bank_transfer", "pay_at_pickup"} {
		for _, chain := range []struct{ kind, valid, invalid string }{
			{"cvs_711", "123456", "12345"},
			{"cvs_familymart", "654321", "ABCDEF"},
			{"cvs_okmart", "1234", "12345"},
			{"cvs_hilife", "123", "12"},
			{"cvs_hilife", "12345678", "123456789"},
		} {
			t.Run(mode+"/"+chain.kind+"/"+chain.valid, func(t *testing.T) {
				e := lbuNew(t)
				selection := oaMaterialCVS(t, e, chain.kind, chain.invalid, mode)
				req := oaMaterialRequest(t, e, selection)
				keys := oaServiceKeys(t)
				svc, err := orderattempt.NewService(e.p.f.runtime, keys)
				if err != nil {
					t.Fatal(err)
				}
				oaMaterialRefusal(t, e, svc, req, "PT422")
				selection["delivery"].(map[string]any)["cvs"].(map[string]any)["store_code"] = chain.valid
				req.Selection, err = json.Marshal(selection)
				if err != nil {
					t.Fatal(err)
				}
				admitted, err := svc.Admit(context.Background(), e.token(), e.store(), req)
				if err != nil || admitted.State != "PENDING" {
					t.Fatalf("per-chain valid C1 sentinel: %+v %v", admitted, err)
				}
				plan, sealed := oaMaterialPlan(t, e, keys, admitted.AttemptID)
				var facts struct {
					Catalog struct {
						Pickup struct {
							fulfillment.Pickup
							AttemptID string `json:"create_attempt_id"`
							OwnerID   string `json:"buyer_owner_id"`
						} `json:"pickup"`
					} `json:"catalog"`
				}
				if err = json.Unmarshal(plan.Facts, &facts); err != nil {
					t.Fatal(err)
				}
				pickup := facts.Catalog.Pickup
				if pickup.Namespace != "attempt."+admitted.AttemptID || pickup.VerificationKind != "BUYER_ENTERED" || pickup.ID != "" ||
					pickup.Version != 1 || pickup.Kind != chain.kind || pickup.Code != chain.valid || pickup.Country != "TW" ||
					pickup.Name != "Synthetic store" || pickup.Address != "Synthetic store address" ||
					!pickup.AttestedAt.Equal(plan.AdmittedAt) || !pickup.ValidUntil.Equal(plan.AdmittedAt.Add(24*time.Hour)) ||
					pickup.AttemptID != admitted.AttemptID || pickup.OwnerID != plan.BuyerOwnerID {
					t.Fatalf("frozen attempt store source mismatch: %+v", pickup)
				}
				replay, err := svc.Admit(context.Background(), e.token(), e.store(), req)
				if err != nil || !replay.Replay || replay.AttemptID != admitted.AttemptID {
					t.Fatalf("CVS source replay: %+v %v", replay, err)
				}
				_, replaySealed := oaMaterialPlan(t, e, keys, admitted.AttemptID)
				if replaySealed.Digest != sealed.Digest || !bytes.Equal(replaySealed.Ciphertext, sealed.Ciphertext) {
					t.Fatal("CVS replay changed frozen source/lifetime")
				}
			})
		}
	}
}

func TestOrderAttemptMaterializationFactsCVSSourcePositive(t *testing.T) {
	e := lbuNew(t)
	selection := oaMaterialCVS(t, e, "cvs_711", "123456", "bank_transfer")
	req := oaMaterialRequest(t, e, selection)
	keys := oaServiceKeys(t)
	svc, err := orderattempt.NewService(e.p.f.runtime, keys)
	if err != nil {
		t.Fatal(err)
	}
	admitted, err := svc.Admit(context.Background(), e.token(), e.store(), req)
	if err != nil || admitted.State != "PENDING" {
		t.Fatalf("valid entered CVS C1 sentinel before metadata oracle: %+v %v", admitted, err)
	}
	plan, _ := oaMaterialPlan(t, e, keys, admitted.AttemptID)
	var facts struct {
		Catalog struct {
			Pickup fulfillment.Pickup `json:"pickup"`
		} `json:"catalog"`
	}
	if err = json.Unmarshal(plan.Facts, &facts); err != nil {
		t.Fatal(err)
	}
	pickup := facts.Catalog.Pickup
	if pickup.Namespace != "attempt."+admitted.AttemptID || pickup.VerificationKind != "BUYER_ENTERED" || pickup.Version != 1 ||
		pickup.Kind != "cvs_711" || pickup.Code != "123456" || pickup.ID != "" ||
		!pickup.AttestedAt.Equal(plan.AdmittedAt) || !pickup.ValidUntil.Equal(plan.AdmittedAt.Add(24*time.Hour)) {
		t.Fatalf("valid C1 lacks frozen entered-store source: %+v", pickup)
	}
}

func TestOrderAttemptMaterializationFactsBankCVSEnabledChain(t *testing.T) {
	e := lbuNew(t)
	selection := oaMaterialCVS(t, e, "cvs_711", "123456", "bank_transfer")
	e.cvsSettings(`["cvs_familymart"]`, true, "20000", 20)
	req := oaMaterialRequest(t, e, selection)
	keys := oaServiceKeys(t)
	svc, err := orderattempt.NewService(e.p.f.runtime, keys)
	if err != nil {
		t.Fatal(err)
	}
	oaMaterialRefusal(t, e, svc, req, "PT422")
	e.cvsSettings(`["cvs_711"]`, true, "20000", 20)
	admitted, err := svc.Admit(context.Background(), e.token(), e.store(), req)
	if err != nil || admitted.State != "PENDING" {
		t.Fatalf("reenabled bank-transfer CVS C1 sentinel: %+v %v", admitted, err)
	}
	_, sealed := oaMaterialPlan(t, e, keys, admitted.AttemptID)
	e.cvsSettings(`["cvs_familymart"]`, true, "20000", 20)
	replay, err := svc.Admit(context.Background(), e.token(), e.store(), req)
	if err != nil || !replay.Replay || replay.AttemptID != admitted.AttemptID {
		t.Fatalf("mutable chain gate blocked exact admitted replay: %+v %v", replay, err)
	}
	_, replaySealed := oaMaterialPlan(t, e, keys, admitted.AttemptID)
	if replaySealed.Digest != sealed.Digest || !bytes.Equal(replaySealed.Ciphertext, sealed.Ciphertext) {
		t.Fatal("bank chain edit replaced frozen plan")
	}
}
