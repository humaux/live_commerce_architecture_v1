package foundation_test

import (
	"encoding/json"
	"net/http/httptest"
	"slices"
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

// Leading Unicode whitespace (U+3000, NBSP, U+2003, zero-width) passes the buyer-name CHECK, but btrim only
// trims ASCII spaces: the old mask began with a non-printable rune that the Go row validator rejects, so one
// such historical order turned the whole store list into a 503. Rows must degrade one by one instead.
func TestMerchantOrdersV2LegacyUnicodeWhitespaceRecipient(t *testing.T) {
	q := pqSetup(t)
	moGrant(t, q.f, q.f.tenantA, q.f.storeA1, q.f.principalA)
	mustExec(t, q.f.owner, `WITH inserted AS (INSERT INTO checkout.orders
 SELECT (jsonb_populate_record(NULL::checkout.orders, to_jsonb(o)||jsonb_build_object(
  'id',gen_random_uuid(),'job_id',9200000000::bigint+n.i,
  'commercial_state','CANCELLED','fulfillment_state','CANCELLED',
  'snapshot',jsonb_set(o.snapshot,'{destination,recipient_name}',to_jsonb(n.name))
 ))).* FROM checkout.orders o CROSS JOIN (VALUES
   (1,E'\u3000\u738b\u5c0f\u660e'),
   (2,E'\u00a0Linda'),
   (3,E'\u2003\u200b\ufeffAnna'),
   (4,E' \u3000 Bob'),
   (5,E'\u3000\u00a0\u2003\u200b\ufeff')) AS n(i,name) WHERE o.id=$1
 RETURNING tenant_id,store_id,id,owner_id,creator_session_id,generation,created_at,expires_at)
 INSERT INTO inventory.reservations(tenant_id,store_id,id,state,expires_at,created_at,checkout_id,buyer_owner_id,buyer_session_id,generation)
 SELECT tenant_id,store_id,id,'RELEASED',expires_at,created_at,id,owner_id,creator_session_id,generation FROM inserted`, q.hold.OrderID)
	r := httptest.NewRequest("GET", "/v1/admin/stores/"+q.f.storeA1+"/orders?view=v2", nil)
	r.Header.Set("Authorization", "Bearer "+q.f.tokens["a"])
	w := httptest.NewRecorder()
	httpapi.NewHandler(q.f.runtime).ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("leading Unicode whitespace poisoned whole list: status=%d body=%s", w.Code, w.Body.String())
	}
	var out struct {
		Items []map[string]json.RawMessage `json:"items"`
		Total int                          `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 6 || out.Total != 6 {
		t.Fatalf("lost rows: %s", w.Body.String())
	}
	var got, keys []string
	for _, row := range out.Items {
		var id, recipient string
		if json.Unmarshal(row["order_id"], &id) != nil || json.Unmarshal(row["recipient_masked"], &recipient) != nil {
			t.Fatalf("bad row: %s", w.Body.String())
		}
		rowKeys := make([]string, 0, len(row))
		for key := range row {
			rowKeys = append(rowKeys, key)
		}
		slices.Sort(rowKeys)
		if keys == nil {
			keys = rowKeys
		} else if !slices.Equal(keys, rowKeys) {
			t.Fatalf("row field set differs (leak): %v vs %v", keys, rowKeys)
		}
		if id != q.hold.OrderID {
			got = append(got, recipient)
		}
	}
	want := []string{"A***", "B***", "L***", "—", "\u738b***"}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("masks=%q want=%q", got, want)
	}
	for _, secret := range []string{"\u5c0f\u660e", "inda", "nna", "Bob"} {
		if strings.Contains(w.Body.String(), secret) {
			t.Fatalf("list leaks recipient remainder %q: %s", secret, w.Body.String())
		}
	}
}
