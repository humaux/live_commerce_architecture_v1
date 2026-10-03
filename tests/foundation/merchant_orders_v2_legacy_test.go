package foundation_test

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"livecommerce/internal/httpapi"
)

func TestMerchantOrdersV2LegacyDisplayFallback(t *testing.T) {
	q := pqSetup(t)
	moGrant(t, q.f, q.f.tenantA, q.f.storeA1, q.f.principalA)
	// Synthetic historical snapshots: retain all transaction fields and FK/checks.
	// The original, valid order must remain readable alongside each legacy row.
	mustExec(t, q.f.owner, `WITH inserted AS (INSERT INTO checkout.orders
 SELECT (jsonb_populate_record(NULL::checkout.orders, to_jsonb(o)||jsonb_build_object(
  'id',gen_random_uuid(),'job_id',9100000000::bigint+n,
  'commercial_state','CANCELLED','fulfillment_state','CANCELLED',
  'snapshot',jsonb_set(o.snapshot,'{destination}',jsonb_build_object(
    'kind',CASE WHEN n=1 THEN 'legacy_unknown' ELSE 'pickup' END,
    'recipient_name',CASE WHEN n=1 THEN NULL ELSE '   ' END,
    'pickup',jsonb_build_object('kind','legacy_unknown')))
 ))).* FROM checkout.orders o CROSS JOIN generate_series(1,2) n WHERE o.id=$1
 RETURNING tenant_id,store_id,id,owner_id,creator_session_id,generation,created_at,expires_at)
 INSERT INTO inventory.reservations(tenant_id,store_id,id,state,expires_at,created_at,checkout_id,buyer_owner_id,buyer_session_id,generation)
 SELECT tenant_id,store_id,id,'RELEASED',expires_at,created_at,id,owner_id,creator_session_id,generation FROM inserted`, q.hold.OrderID)
	r := httptest.NewRequest("GET", "/v1/admin/stores/"+q.f.storeA1+"/orders?view=v2", nil)
	r.Header.Set("Authorization", "Bearer "+q.f.tokens["a"])
	w := httptest.NewRecorder()
	httpapi.NewHandler(q.f.runtime).ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("legacy display field poisoned whole list: status=%d body=%s", w.Code, w.Body.String())
	}
	var out struct {
		Items []struct {
			ID        string `json:"order_id"`
			Recipient string `json:"recipient_masked"`
			Delivery  string `json:"delivery_kind"`
		} `json:"items"`
		Total  int            `json:"total"`
		Counts map[string]int `json:"counts"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 3 || out.Total != 3 || out.Counts["all"] != 3 {
		t.Fatalf("lost rows or counts: %s", w.Body.String())
	}
	valid, fallback := 0, 0
	for _, row := range out.Items {
		if row.ID == q.hold.OrderID {
			valid++
			if row.Delivery != "home" || !strings.HasSuffix(row.Recipient, "***") {
				t.Fatalf("valid row changed: %+v", row)
			}
		} else {
			fallback++
			if row.Recipient != "—" || row.Delivery != "unknown" {
				t.Fatalf("unsafe legacy display: %+v", row)
			}
		}
	}
	if valid != 1 || fallback != 2 {
		t.Fatalf("valid=%d fallback=%d", valid, fallback)
	}
}
