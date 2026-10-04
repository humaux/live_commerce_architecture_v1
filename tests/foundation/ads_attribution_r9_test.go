package foundation_test

// Independent owner R9 contract: refusal is a no-op, not a checkout error.
// All orders come from actual Begin. Owner writes below only prepare synthetic
// age/session counterexamples; no product guard or function is replaced.
import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/buyer"
)

func atR9Rows(t *testing.T, b bcHarness, order string) []string {
	t.Helper()
	return lcStrings(t, b.f.owner, `SELECT 'order:'||to_jsonb(o)::text FROM checkout.orders o WHERE id=$1
 UNION ALL SELECT 'attribution:'||to_jsonb(a)::text FROM orders.order_attribution a WHERE order_id=$1
 UNION ALL SELECT 'origins:'||to_jsonb(c)::text FROM claims.order_origins c WHERE order_id=$1
 UNION ALL SELECT 'reservation:'||to_jsonb(r)::text FROM inventory.reservations r WHERE checkout_id=$1
 UNION ALL SELECT 'ledger:'||to_jsonb(l)::text FROM inventory.ledger l WHERE checkout_id=$1`, order)
}

func atR9Committed(t *testing.T, b bcHarness, xid string) {
	t.Helper()
	var state string
	if err := b.f.owner.QueryRow(context.Background(), `SELECT pg_xact_status($1::xid8)`, xid).Scan(&state); err != nil || state != "committed" {
		t.Fatalf("attribution transaction did not commit: state=%s err=%v", state, err)
	}
}

func TestAdsAttributionR9BeginNeverFails(t *testing.T) {
	for _, name := range []string{"legitimate click", "garbage", "expired", "foreign", "missing", "unmarshalable timestamp"} {
		t.Run(name, func(t *testing.T) {
			b := bcSetup(t)
			e := newAdsEnv(t, adsOpts{fx: b.f})
			b.input.AdTouch = atLiveClick(t, e)
			d := b.input.AdTouch.DraftID
			switch name {
			case "garbage":
				b.input.AdTouch.DraftID = "garbage-draft"
			case "expired":
				b.input.AdTouch.ClickedAt = time.Now().Add(-8 * 24 * time.Hour)
			case "foreign":
				foreign := newAdsEnv(t, adsOpts{})
				b.input.AdTouch.DraftID = foreign.newDraft(adsDraftIn{})
			case "missing":
				b.input.AdTouch = nil
			case "unmarshalable timestamp":
				b.input.AdTouch.ClickedAt = time.Date(10000, time.January, 1, 0, 0, 0, 0, time.UTC)
				if _, err := json.Marshal(b.input.AdTouch); err == nil {
					t.Fatal("year10000 fixture unexpectedly marshals")
				}
			}
			key := t04Key("r9-begin")
			r, err := b.begin(key)
			if err != nil {
				t.Fatalf("R9 actual Begin must commit %s touch: %v", name, err)
			}
			if miCount(t, b.f.owner, `SELECT count(*) FROM checkout.orders o JOIN checkout.command_results r ON r.order_id=o.id
 WHERE o.id=$1 AND o.creator_session_id=$2 AND r.idempotency_key=$3`, r.OrderID, b.cap.Scope.SessionID, key) != 1 {
				t.Fatal("Begin order and idempotency receipt not committed")
			}
			want := int64(0)
			if name == "legitimate click" {
				want = 1
				if miCount(t, b.f.owner, `SELECT count(*) FROM orders.order_attribution WHERE order_id=$1 AND path='ad_click' AND draft_id=$2 AND fbc IS NULL AND fbp IS NULL`, r.OrderID, d) != 1 {
					t.Fatal("legitimate Begin did not freeze exact click")
				}
			}
			if n := miCount(t, b.f.owner, `SELECT count(*) FROM orders.order_attribution WHERE order_id=$1`, r.OrderID); n != want {
				t.Fatalf("touch %s attribution rows=%d want=%d", name, n, want)
			}
		})
	}
}

func TestAdsAttributionR9RefusalCommitsUnchanged(t *testing.T) {
	for _, name := range []string{"older than one minute", "same buyer other session", "other store", "second freeze", "garbage timestamp"} {
		t.Run(name, func(t *testing.T) {
			b := bcSetup(t)
			e := newAdsEnv(t, adsOpts{fx: b.f})
			d := e.newDraft(adsDraftIn{})
			if name == "second freeze" {
				b.input.AdTouch = atLiveClick(t, e)
				d = b.input.AdTouch.DraftID
			}
			r, err := b.begin(t04Key("r9-original"))
			if err != nil {
				t.Fatalf("actual Begin fixture: %v", err)
			}
			cap := b.cap
			touch, _ := json.Marshal(atTouch(e.newDraft(adsDraftIn{}), time.Second))
			switch name {
			case "older than one minute":
				mustExec(t, b.f.owner, `UPDATE checkout.orders SET created_at=clock_timestamp()-interval '2 minutes',expires_at=clock_timestamp()+interval '8 minutes' WHERE id=$1`, r.OrderID)
			case "same buyer other session":
				cap.Token, cap.Scope.SessionID = randomToken(), randomUUID()
				mustExec(t, b.f.owner, `INSERT INTO buyer.capability_sessions(tenant_id,store_id,owner_id,id,token_hash,expires_at)
 VALUES($1,$2,$3,$4,$5,clock_timestamp()+interval '1 hour')`, cap.Scope.TenantID, cap.Scope.StoreID, cap.Scope.OwnerID, cap.Scope.SessionID, tokenHash(cap.Token))
			case "other store":
				foreign := newAdsEnv(t, adsOpts{})
				cap = mustIssue(t, b.cqHarness.service, foreign.store)
				if cap.Scope.StoreID == b.cap.Scope.StoreID {
					t.Fatal("other-store probe did not establish different store")
				}
			case "garbage timestamp":
				touch, _ = json.Marshal(map[string]any{"draft_id": d, "clicked_at": "not-a-timestamp", "fbc": atFBC, "fbp": atFBP})
			}
			before := atR9Rows(t, b, r.OrderID)
			var xid string
			err = buyer.WithScope(context.Background(), b.pool, cap.Token, cap.Scope.StoreID, func(ctx context.Context, tx pgx.Tx, _ buyer.Scope) error {
				if _, err := tx.Exec(ctx, `SELECT orders.freeze_attribution($1,$2,$3,$4::jsonb,'192.0.2.11')`, tokenHash(cap.Token), cap.Scope.StoreID, r.OrderID, string(touch)); err != nil {
					return err
				}
				return tx.QueryRow(ctx, `SELECT pg_current_xact_id()::text`).Scan(&xid)
			})
			if err != nil {
				t.Fatalf("R9 %s must be no-op, not transaction failure: %v", name, err)
			}
			atR9Committed(t, b, xid)
			lcSameSet(t, name+" must change no order/attribution/origin/inventory facts", atR9Rows(t, b, r.OrderID), before)
		})
	}
}

func TestAdsAttributionR9MalformedServerParameters(t *testing.T) {
	b := bcSetup(t)
	// No order fixture needed: server-side malformed arguments must be PT400,
	// distinctly from the valid-but-ineligible order no-op cases above.
	for _, tc := range []struct {
		name  string
		hash  []byte
		store any
		order any
	}{
		{"short hash", []byte{1}, b.cap.Scope.StoreID, randomUUID()},
		{"NULL hash", nil, b.cap.Scope.StoreID, randomUUID()},
		{"NULL store", tokenHash(b.cap.Token), nil, randomUUID()},
		{"NULL order", tokenHash(b.cap.Token), b.cap.Scope.StoreID, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := b.pool.Exec(context.Background(), `SELECT orders.freeze_attribution($1,$2,$3,NULL,'')`, tc.hash, tc.store, tc.order)
			requirePGCode(t, err, "PT400", tc.name)
		})
	}
}
