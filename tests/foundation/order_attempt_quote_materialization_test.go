// Purpose: REAL_PG acceptance for frozen OA1 attempt-cart and quote materialization.
// Depends on: real C1 Service.Admit, PlanKeys.Open, public execution fence and native storefront tables.
// Used by: parent-owned CI red/green; private helper tests always roll back, never claim full C2 success.
// Invariants: I01/I04/I08/I14; exact provenance, preserved origins and frozen amounts, no buyer cart mutation.
package foundation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/checkout"
	"livecommerce/internal/orderattempt"
	"livecommerce/internal/pricing"
	"livecommerce/internal/storefront"
)

type oaQuoteWire struct {
	CartID      string          `json:"cart_id"`
	QuoteID     string          `json:"quote_id"`
	CartVersion int64           `json:"cart_version"`
	Quote       json.RawMessage `json:"quote"`
}

func oaQuoteAdmit(t *testing.T, e *lbuEnv, live bool) orderattempt.AdmissionReceipt {
	t.Helper()
	req := oaServiceRequest(t, e)
	if live {
		_, offer, bundle, conversation := e.scenario()
		selection := oaFactsSelection(e)
		line := selection["items"].([]map[string]any)[0]
		line["bundle_id"], line["offer_id"] = bundle, offer.ID
		selection["for"] = map[string]any{"bundle_ids": []string{bundle}, "conversation_id": conversation}
		var err error
		req.Selection, err = json.Marshal(selection)
		if err != nil {
			t.Fatal(err)
		}
	}
	svc, err := orderattempt.NewService(e.p.f.runtime, oaServiceKeys(t))
	if err != nil {
		t.Fatal(err)
	}
	a, err := svc.Admit(context.Background(), e.token(), e.store(), req)
	if err != nil || a.State != "PENDING" || a.AttemptID == "" {
		t.Fatalf("real C1 positive sentinel: state=%s err=%v", a.State, err)
	}
	return a
}

func oaQuoteCall(tx pgx.Tx, attempt string, plain []byte) (oaQuoteWire, []byte, error) {
	var raw []byte
	// Calls storefront.order_attempt_materialize_quote, order-attempt-v1 §§2/6j.
	err := tx.QueryRow(context.Background(), `SELECT storefront.order_attempt_materialize_quote($1::uuid,$2::text)`, attempt, string(plain)).Scan(&raw)
	var out oaQuoteWire
	if err == nil {
		err = json.Unmarshal(raw, &out)
	}
	return out, raw, err
}

func TestOrderAttemptQuoteMaterializationFrozenReadbackAndRollback(t *testing.T) {
	for _, live := range []bool{false, true} {
		t.Run(map[bool]string{false: "catalog", true: "live-and-catalog-same-SKU"}[live], func(t *testing.T) {
			e := lbuNew(t)
			a := oaQuoteAdmit(t, e, live)
			ctx := context.Background()
			// Native fixture preparation gives this exact owner a real-session storefront
			// cart with a nontrivial version; an empty cart inventory is not a sentinel.
			session, browserCart := randomUUID(), randomUUID()
			mustExec(t, e.p.f.owner, `INSERT INTO buyer.capability_sessions(id,tenant_id,store_id,owner_id,token_hash,expires_at)
			 SELECT $2::uuid,tenant_id,store_id,buyer_owner_id,sha256(convert_to(($2::uuid)::text,'UTF8')),clock_timestamp()+interval '1 hour' FROM ops.order_attempts WHERE attempt_id=$1`, a.AttemptID, session)
			mustExec(t, e.p.f.owner, `INSERT INTO storefront.carts(tenant_id,store_id,owner_id,id,creator_session_id,currency,version)
			 SELECT tenant_id,store_id,buyer_owner_id,$2,$3,'TWD',7 FROM ops.order_attempts WHERE attempt_id=$1`, a.AttemptID, browserCart, session)
			mustExec(t, e.p.f.owner, `INSERT INTO storefront.cart_lines(tenant_id,store_id,owner_id,cart_id,sku_id,quantity)
			 SELECT tenant_id,store_id,buyer_owner_id,$2,$3,1 FROM ops.order_attempts WHERE attempt_id=$1`, a.AttemptID, browserCart, e.money)
			// Current catalog drift after real C1 must not reprice the attempt quote.
			mustExec(t, e.p.f.owner, `UPDATE catalog.skus SET price_minor=price_minor+123,version=version+1 WHERE tenant_id=$1 AND store_id=$2 AND id=$3`, e.tenant(), e.store(), e.money)
			tx, plain := oaFenceDomainTx(t, e, a, true)
			var plan orderattempt.FrozenPlan
			if err := json.Unmarshal(plain, &plan); err != nil {
				t.Fatal(err)
			}
			var facts struct {
				Catalog struct {
					Market pricing.Market `json:"market"`
					Policy pricing.Policy `json:"policy"`
					Lines  []struct {
						storefront.QuoteLine
						Ordinal  int64  `json:"ordinal"`
						OriginID string `json:"origin_id"`
						BundleID string `json:"bundle_id"`
						OfferID  string `json:"offer_id"`
					} `json:"lines"`
				} `json:"catalog"`
			}
			if err := json.Unmarshal(plan.Facts, &facts); err != nil || len(facts.Catalog.Lines) != 2 {
				t.Fatalf("frozen facts positive sentinel: %v", err)
			}
			var before []byte
			if err := tx.QueryRow(ctx, `SELECT coalesce(jsonb_agg(to_jsonb(c) ORDER BY id),'[]'::jsonb) FROM storefront.carts c WHERE tenant_id=$1 AND store_id=$2 AND cart_kind='storefront'`, e.tenant(), e.store()).Scan(&before); err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(before, []byte("[]")) {
				t.Fatal("storefront cart positive sentinel missing")
			}
			if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id','caller-marker',true),set_config('app.store_id','caller-marker',true),set_config('app.buyer_id','caller-marker',true),set_config('app.buyer_session_id','caller-marker',true)`); err != nil {
				t.Fatal(err)
			}
			if _, err := tx.Exec(ctx, `SET LOCAL ROLE commerce_order_attempt_owner`); err != nil {
				t.Fatal(err)
			}
			out, _, err := oaQuoteCall(tx, a.AttemptID, plain)
			if err != nil {
				t.Fatalf("materialization after real C1+Open+public fence: %v", err)
			}
			if out.CartID == "" || out.QuoteID == "" || out.CartVersion != 1 {
				t.Fatal("missing physical attempt IDs/version")
			}
			var quote storefront.Quote
			if err = json.Unmarshal(out.Quote, &quote); err != nil {
				t.Fatal(err)
			}
			if quote.ID != out.QuoteID || quote.CartID != out.CartID || quote.CartVersion != 1 || quote.CalculationVersion != "v1" || quote.Currency != facts.Catalog.Market.Currency || quote.MarketVersion != facts.Catalog.Market.Version || !reflect.DeepEqual(quote.Policy, facts.Catalog.Policy) || !reflect.DeepEqual(quote.Amount, plan.Calculation) || !quote.CreatedAt.Equal(plan.AdmittedAt) || !quote.ExpiresAt.Equal(plan.ExecuteBy) || len(quote.Lines) != 2 {
				t.Fatal("quote differs from authoritative frozen facts/calculation/times")
			}
			for i, line := range quote.Lines {
				frozen := facts.Catalog.Lines[i]
				price := frozen.UnitPriceMinor
				if live && i == 0 {
					price = ltgLive
				}
				if line.SKUID != frozen.SKUID || line.Name != frozen.Name || line.ProductVersion != frozen.ProductVersion || line.SKUVersion != frozen.SKUVersion || line.Quantity != frozen.Quantity || line.UnitPriceMinor != price || line.Amount != plan.Calculation.Lines[i] {
					t.Fatalf("frozen line/ordinal %d changed", i)
				}
				if live && i == 0 && (line.PriceRule != pricing.RuleLiveClaim || line.CatalogUnitPriceMinor != frozen.UnitPriceMinor || line.ClaimBundleID != frozen.BundleID || line.ClaimOfferID != frozen.OfferID) {
					t.Fatal("live exact-tuple price evidence lost")
				}
				if i == 1 && line.PriceRule != "" {
					t.Fatal("claim price leaked to same-SKU manual origin")
				}
			}
			var restored bool
			if err = tx.QueryRow(ctx, `SELECT current_setting('app.tenant_id')='caller-marker' AND current_setting('app.store_id')='caller-marker' AND current_setting('app.buyer_id')='caller-marker' AND current_setting('app.buyer_session_id')='caller-marker'`).Scan(&restored); err != nil || !restored {
				t.Fatal("helper leaked buyer context")
			}
			if _, err = tx.Exec(ctx, `RESET ROLE`); err != nil {
				t.Fatal(err)
			}
			var bound bool
			if err = tx.QueryRow(ctx, `SELECT c.cart_kind='order_attempt' AND c.creator_session_id IS NULL AND c.create_attempt_id=a.attempt_id AND c.owner_id=a.buyer_owner_id AND c.version=1
			 AND q.creator_kind='order_attempt' AND q.creator_session_id IS NULL AND q.create_attempt_id=a.attempt_id AND q.cart_version=1 AND q.snapshot=$4::jsonb
			 AND (SELECT count(*) FROM storefront.events WHERE create_attempt_id=a.attempt_id AND session_id IS NULL AND creator_kind='order_attempt')=2
			 AND (SELECT count(*) FROM storefront.cart_lines WHERE create_attempt_id=a.attempt_id AND cart_kind='order_attempt')=2
			 FROM ops.order_attempts a JOIN storefront.carts c ON c.create_attempt_id=a.attempt_id JOIN storefront.quotes q ON q.cart_id=c.id WHERE a.attempt_id=$1 AND c.id=$2 AND q.id=$3`, a.AttemptID, out.CartID, out.QuoteID, string(out.Quote)).Scan(&bound); err != nil || !bound {
				t.Fatalf("exact cart/quote/event FK provenance: %v", err)
			}
			var rawLines []byte
			if err = tx.QueryRow(ctx, `SELECT jsonb_agg(jsonb_build_object('line_id',l.line_id,'line_ordinal',l.line_ordinal,'origin_kind',l.origin_kind,'origin_id',l.origin_id) ORDER BY l.line_ordinal) FROM storefront.cart_lines l WHERE cart_id=$1`, out.CartID).Scan(&rawLines); err != nil {
				t.Fatal(err)
			}
			var persisted []map[string]any
			var snapshot map[string]json.RawMessage
			if err = json.Unmarshal(rawLines, &persisted); err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(out.Quote, &snapshot); err != nil {
				t.Fatal(err)
			}
			var lines []map[string]any
			if err = json.Unmarshal(snapshot["lines"], &lines); err != nil {
				t.Fatal(err)
			}
			for i, line := range lines {
				for _, key := range []string{"line_id", "line_ordinal", "origin_kind", "origin_id"} {
					if line[key] != persisted[i][key] || line[key] == nil {
						t.Fatalf("snapshot lost relational %s", key)
					}
				}
				want := "manual"
				if live && i == 0 {
					want = "claim"
				}
				if line["origin_kind"] != want || line["origin_id"] != facts.Catalog.Lines[i].OriginID || line["line_ordinal"] != float64(facts.Catalog.Lines[i].Ordinal) {
					t.Fatal("C1 origins/ordinals changed")
				}
			}
			// checkout.Get decodes this exact Snapshot type; it must retain attempt origins.
			encoded, err := json.Marshal(map[string]any{"quote": json.RawMessage(out.Quote)})
			if err != nil {
				t.Fatal(err)
			}
			var typed checkout.Snapshot
			if err = json.Unmarshal(encoded, &typed); err != nil {
				t.Fatal(err)
			}
			roundtrip, err := json.Marshal(typed.Quote)
			if err != nil {
				t.Fatal(err)
			}
			var round map[string]json.RawMessage
			if err = json.Unmarshal(roundtrip, &round); err != nil {
				t.Fatal(err)
			}
			var typedLines []map[string]any
			if err = json.Unmarshal(round["lines"], &typedLines); err != nil {
				t.Fatal(err)
			}
			for i, line := range typedLines {
				for _, key := range []string{"line_id", "line_ordinal", "origin_kind", "origin_id"} {
					if line[key] != lines[i][key] {
						t.Fatalf("typed checkout.Get snapshot dropped %s", key)
					}
				}
			}
			var after []byte
			if err = tx.QueryRow(ctx, `SELECT coalesce(jsonb_agg(to_jsonb(c) ORDER BY id),'[]'::jsonb) FROM storefront.carts c WHERE tenant_id=$1 AND store_id=$2 AND cart_kind='storefront'`, e.tenant(), e.store()).Scan(&after); err != nil || !bytes.Equal(before, after) {
				t.Fatal("buyer storefront cart/version changed")
			}
			if err = tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			var effects int
			if err = e.p.f.owner.QueryRow(ctx, `SELECT (SELECT count(*) FROM storefront.carts WHERE create_attempt_id=$1)+(SELECT count(*) FROM storefront.quotes WHERE create_attempt_id=$1)+(SELECT count(*) FROM storefront.cart_lines WHERE create_attempt_id=$1)+(SELECT count(*) FROM storefront.events WHERE create_attempt_id=$1)`, a.AttemptID).Scan(&effects); err != nil || effects != 0 {
				t.Fatalf("outer rollback retained effects=%d err=%v", effects, err)
			}
		})
	}
}

func TestOrderAttemptQuoteMaterializationFrozenShortTTLIsInformational(t *testing.T) {
	e := lbuNew(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	// Test-only native catalog preparation precedes real C1; attempt metadata and
	// ciphertext are never rewritten, triggers and time predicates remain enabled.
	mustExec(t, e.p.f.owner, `UPDATE pricing.policy_versions SET quote_ttl_seconds=60 WHERE tenant_id=$1 AND store_id=$2 AND market_id=$3 AND country='TW' AND method='delivery:home'`, e.tenant(), e.store(), e.p.market.ID)
	a := oaQuoteAdmit(t, e, false)
	seed, plain := oaFenceDomainTx(t, e, a, true)
	var plan orderattempt.FrozenPlan
	if err := json.Unmarshal(plain, &plan); err != nil {
		t.Fatal(err)
	}
	if err := seed.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var facts struct {
		Catalog struct {
			Policy pricing.Policy `json:"policy"`
		} `json:"catalog"`
	}
	if err := json.Unmarshal(plan.Facts, &facts); err != nil || facts.Catalog.Policy.QuoteTTLSeconds != 60 {
		t.Fatalf("short frozen TTL positive sentinel: %v", err)
	}
	wait := time.NewTimer(time.Until(plan.AdmittedAt.Add(61 * time.Second)))
	defer wait.Stop()
	select {
	case <-ctx.Done():
		t.Fatal("bounded short-TTL wait exceeded")
	case <-wait.C:
	}
	tx, exact := oaFenceDomainTx(t, e, a, true)
	var elapsed bool
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()>admitted_at+interval '60 seconds' AND clock_timestamp()<execute_by FROM ops.order_attempts WHERE attempt_id=$1`, a.AttemptID).Scan(&elapsed); err != nil || !elapsed {
		t.Fatalf("elapsed original TTL while attempt valid sentinel: %v", err)
	}
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE commerce_order_attempt_owner`); err != nil {
		t.Fatal(err)
	}
	out, _, err := oaQuoteCall(tx, a.AttemptID, exact)
	if err != nil {
		t.Fatalf("old policy TTL must not replace frozen execute_by: %v", err)
	}
	var quote storefront.Quote
	if err = json.Unmarshal(out.Quote, &quote); err != nil || !quote.CreatedAt.Equal(plan.AdmittedAt) || !quote.ExpiresAt.Equal(plan.ExecuteBy) || quote.Policy.QuoteTTLSeconds != 60 {
		t.Fatalf("original attempt lifetime lost: %v", err)
	}
}

func TestOrderAttemptQuoteMaterializationDenialsAndExactACL(t *testing.T) {
	e := lbuNew(t)
	a := oaQuoteAdmit(t, e, false)
	ctx := context.Background()
	for _, role := range []string{"commerce_runtime", "commerce_claims_worker", "commerce_checkout_writer", "commerce_privacy_writer"} {
		t.Run(role, func(t *testing.T) {
			tx, plain := oaFenceDomainTx(t, e, a, true)
			if _, err := tx.Exec(ctx, `SET LOCAL ROLE `+pgx.Identifier{role}.Sanitize()); err != nil {
				t.Fatal(err)
			}
			_, raw, err := oaQuoteCall(tx, a.AttemptID, plain)
			oaFenceCode(t, raw, err, "42501")
		})
	}
	seed, exact := oaFenceDomainTx(t, e, a, true)
	if err := seed.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"unfenced", "different-plan-bytes", "wrong-attempt", "altered-money"} {
		t.Run(mode, func(t *testing.T) {
			tx, plain := oaFenceDomainTx(t, e, a, mode != "unfenced")
			id, want := a.AttemptID, "PT400"
			switch mode {
			case "unfenced":
				plain = exact
				want = "PT409"
			case "different-plan-bytes":
				plain = append([]byte(" "), plain...)
			case "wrong-attempt":
				id = randomUUID()
				want = "PT409"
			case "altered-money":
				var p map[string]any
				if err := json.Unmarshal(plain, &p); err != nil {
					t.Fatal(err)
				}
				p["calculation"] = map[string]any{"total_minor": 1}
				var err error
				plain, err = json.Marshal(p)
				if err != nil {
					t.Fatal(err)
				}
			}
			if _, err := tx.Exec(ctx, `SET LOCAL ROLE commerce_order_attempt_owner`); err != nil {
				t.Fatal(err)
			}
			_, raw, err := oaQuoteCall(tx, id, plain)
			oaFenceCode(t, raw, err, want)
		})
	}
	var safe bool
	err := e.p.f.owner.QueryRow(ctx, `SELECT p.prosecdef AND p.provolatile='v' AND p.prorettype='jsonb'::regtype
	 AND r.rolname='commerce_buyer_writer' AND NOT(r.rolcanlogin OR r.rolsuper OR r.rolbypassrls)
	 AND p.proconfig=ARRAY['search_path=pg_catalog']::text[] AND length(obj_description(p.oid,'pg_proc'))>0
	 AND ARRAY(SELECT pg_get_userbyid(a.grantee)::text FROM aclexplode(p.proacl) a WHERE a.grantee<>p.proowner AND a.privilege_type='EXECUTE' ORDER BY 1)=ARRAY['commerce_order_attempt_owner']::text[]
	 AND NOT EXISTS(SELECT 1 FROM aclexplode(p.proacl) a WHERE a.grantee<>p.proowner AND a.is_grantable)
	 FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid=to_regprocedure('storefront.order_attempt_materialize_quote(uuid,text)')`).Scan(&safe)
	if err != nil || !safe {
		t.Fatalf("missing/unsafe exact materializer ACL: safe=%v err=%v", safe, err)
	}
}

func TestOrderAttemptQuoteMaterializationLegacyDTOUnchanged(t *testing.T) {
	line := storefront.QuoteLine{SKUID: randomUUID(), Quantity: 1, UnitPriceMinor: 10}
	raw, err := json.Marshal(line)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err = json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"line_id", "line_ordinal", "origin_kind", "origin_id"} {
		if _, ok := out[key]; ok {
			t.Fatalf("legacy storefront serialization added %s", key)
		}
	}
}

func TestOrderAttemptQuoteMaterializationTypedSnapshotOrigins(t *testing.T) {
	// This tests the actual checkout.Get decode type without a database/helper
	// dependency so an omitted optional field has its own causal red witness.
	line := map[string]any{"sku_id": randomUUID(), "quantity": float64(2), "unit_price_minor": float64(30000), "line_id": randomUUID(), "line_ordinal": float64(7), "origin_kind": "manual", "origin_id": randomUUID()}
	raw, err := json.Marshal(map[string]any{"quote": map[string]any{"lines": []map[string]any{line}}})
	if err != nil {
		t.Fatal(err)
	}
	var snapshot checkout.Snapshot
	if err = json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatal(err)
	}
	roundtrip, err := json.Marshal(snapshot.Quote)
	if err != nil {
		t.Fatal(err)
	}
	var quote struct {
		Lines []map[string]any `json:"lines"`
	}
	if err = json.Unmarshal(roundtrip, &quote); err != nil || len(quote.Lines) != 1 {
		t.Fatalf("typed quote line sentinel: %v", err)
	}
	for _, key := range []string{"line_id", "line_ordinal", "origin_kind", "origin_id"} {
		if quote.Lines[0][key] != line[key] {
			t.Fatalf("checkout.Get snapshot decode dropped %s", key)
		}
	}
}
