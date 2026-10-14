// Purpose: independently pin the two creator identities and origin-preserving materialization in OA-1.
// Depends on: real migrated PostgreSQL catalogs and fixture; order-attempt-v1 §§2,6j/k.
// Used by: foundation CI; catalog expressions are evaluated by PostgreSQL, never by a Go model.
// Invariants: I01/I03/I08/I09; no synthetic bearer session, no unscoped third creator.
// Status: REAL_PG schema gate; placement/runtime lifecycle needs separate C1/C2 tests.
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

func TestOrderAttemptProvenanceCreatorXOR(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	for _, tc := range []struct {
		table, kind, owner, session, constraint string
	}{
		{"storefront.carts", "cart_kind", "owner_id", "creator_session_id", "carts_creator_identity"},
		{"storefront.quotes", "creator_kind", "owner_id", "creator_session_id", "quotes_creator_identity"},
		{"storefront.events", "creator_kind", "owner_id", "session_id", "events_creator_identity"},
		{"storefront.destination_snapshots", "creator_kind", "owner_id", "creator_session_id", "destination_snapshots_creator_identity"},
		{"storefront.destination_events", "creator_kind", "owner_id", "session_id", "destination_events_creator_identity"},
		{"buyer.command_results", "creator_kind", "owner_id", "session_id", "command_results_creator_identity"},
		{"checkout.orders", "creator_kind", "owner_id", "creator_session_id", "orders_creator_identity"},
		{"checkout.command_results", "creator_kind", "owner_id", "creator_session_id", "command_results_creator_identity"},
		{"checkout.events", "creator_kind", "owner_id", "session_id", "events_creator_identity"},
		{"inventory.reservations", "creator_kind", "buyer_owner_id", "buyer_session_id", "reservations_creator_identity"},
		{"inventory.ledger", "creator_kind", "buyer_owner_id", "buyer_session_id", "ledger_creator_identity"},
	} {
		t.Run(tc.table, func(t *testing.T) {
			var nullable bool
			if err := f.owner.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_attribute
			 WHERE attrelid=to_regclass($1) AND attname=$2 AND NOT attnotnull AND NOT attisdropped)`, tc.table, tc.session).Scan(&nullable); err != nil || !nullable {
				t.Errorf("attempt creator cannot insert with NULL session on %s.%s: nullable=%v err=%v", tc.table, tc.session, nullable, err)
			}
			var expression string
			if err := f.owner.QueryRow(ctx, `SELECT pg_get_expr(conbin,conrelid) FROM pg_constraint
			 WHERE conrelid=to_regclass($1) AND conname=$2 AND contype='c' AND convalidated`, tc.table, tc.constraint).Scan(&expression); err != nil {
				t.Fatalf("missing validated creator XOR on %s: %v", tc.table, err)
			}
			const owner = "00000000-0000-4000-8000-000000000001"
			const identity = "00000000-0000-4000-8000-000000000002"
			for _, row := range []struct {
				name             string
				kind             any
				session, attempt any
				want             bool
			}{
				{"real_browser", "storefront", identity, nil, true},
				{"durable_attempt", "order_attempt", nil, identity, true},
				{"browser_without_session", "storefront", nil, nil, false},
				{"attempt_without_attempt", "order_attempt", nil, nil, false},
				{"browser_with_both", "storefront", identity, identity, false},
				{"attempt_with_both", "order_attempt", identity, identity, false},
				{"browser_using_attempt", "storefront", nil, identity, false},
				{"attempt_using_session", "order_attempt", identity, nil, false},
				{"unknown_kind", "worker", nil, identity, false},
				{"null_kind", nil, nil, identity, false},
			} {
				values := map[string]any{tc.kind: row.kind, tc.owner: owner, tc.session: row.session,
					"create_attempt_id": row.attempt, "checkout_id": identity, "id": identity, "generation": 1}
				data, err := json.Marshal(values)
				if err != nil {
					t.Fatal(err)
				}
				var admitted bool
				// CHECK accepts NULL: treat it as acceptance so a nullable loophole fails this gate.
				query := fmt.Sprintf(`SELECT (%s) IS NOT FALSE FROM jsonb_populate_record(NULL::%s,$1::jsonb)`, expression, tc.table)
				if err := f.owner.QueryRow(ctx, query, string(data)).Scan(&admitted); err != nil || admitted != row.want {
					t.Errorf("%s/%s: admitted=%v want=%v err=%v", tc.table, row.name, admitted, row.want, err)
				}
			}
			for _, target := range []struct {
				table         string
				local, remote []string
			}{
				{"buyer.capability_sessions", []string{"tenant_id", "store_id", tc.owner, tc.session}, []string{"tenant_id", "store_id", "owner_id", "id"}},
				{"ops.order_attempt_creators", []string{"tenant_id", "store_id", tc.owner, "create_attempt_id"}, []string{"tenant_id", "store_id", "buyer_owner_id", "attempt_id"}},
			} {
				oaProvenanceFK(t, f, tc.table, target.table, target.local, target.remote, false)
			}
		})
	}
}

func TestOrderAttemptProvenanceRemainingInventoryChecks(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	const identity = "00000000-0000-4000-8000-000000000002"
	// Complete synthetic CHECK inputs: testing just the new XOR would miss old
	// reservation_checkout_family or ledger_checkout_actor still requiring a session.
	base := map[string]any{
		"tenant_id": identity, "store_id": identity, "id": identity,
		"creator_kind": "order_attempt", "create_attempt_id": identity,
		"buyer_owner_id": identity, "buyer_session_id": nil, "checkout_id": identity,
		"generation": 1, "state": "HELD", "created_at": "2026-10-13T00:00:00Z",
		"expires_at": "2026-10-13T00:15:00Z", "warehouse_id": identity, "sku_id": identity,
		"actor_kind": "ORDER_ATTEMPT", "principal_id": identity, "reservation_id": identity,
		"kind": "RESERVE", "delta_on_hand": 0, "delta_reserved": 2,
		"delta_allocated": 0, "delta_unavailable": 0, "balance_version": 1,
		"operation": "checkout.order_attempt.reserve", "command_key": identity, "reason": "",
	}
	data, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"inventory.reservations", "inventory.ledger"} {
		rows, err := f.owner.Query(ctx, `SELECT conname,pg_get_expr(conbin,conrelid) FROM pg_constraint
		 WHERE conrelid=to_regclass($1) AND contype='c' AND convalidated ORDER BY conname`, table)
		if err != nil {
			t.Fatal(err)
		}
		type check struct{ name, expression string }
		var checks []check
		for rows.Next() {
			var c check
			if err := rows.Scan(&c.name, &c.expression); err != nil {
				t.Fatal(err)
			}
			checks = append(checks, c)
		}
		rows.Close()
		if err := rows.Err(); err != nil || len(checks) == 0 {
			t.Fatalf("no inventory CHECK sentinel on %s: checks=%d err=%v", table, len(checks), err)
		}
		for _, c := range checks {
			var accepted bool
			query := fmt.Sprintf(`SELECT (%s) IS NOT FALSE FROM jsonb_populate_record(NULL::%s,$1::jsonb)`, c.expression, table)
			if err := f.owner.QueryRow(ctx, query, string(data)).Scan(&accepted); err != nil || !accepted {
				t.Errorf("attempt-shaped inventory row blocked by %s.%s: accepted=%v err=%v", table, c.name, accepted, err)
			}
		}
	}
}

func TestOrderAttemptProvenanceScopedAggregate(t *testing.T) {
	f := fixture(t)
	// The retained business creator is separated from the 90-day replay record,
	// whose exact FK still binds every admitted attempt to the same owner and scope.
	oaProvenanceFK(t, f, "ops.order_attempts", "ops.order_attempt_creators",
		[]string{"tenant_id", "store_id", "buyer_owner_id", "attempt_id"},
		[]string{"tenant_id", "store_id", "buyer_owner_id", "attempt_id"}, false)
	// Every attempt-owned child also binds its exact attempt parent; an FK only to
	// ops.order_attempts would let two attempts of the same buyer exchange artifacts.
	for _, tc := range []struct {
		child, parent string
		local, remote []string
		deferred      bool
	}{
		{"storefront.quotes", "storefront.carts", []string{"tenant_id", "store_id", "owner_id", "cart_id", "create_attempt_id"}, []string{"tenant_id", "store_id", "owner_id", "id", "create_attempt_id"}, false},
		{"storefront.destination_snapshots", "storefront.carts", []string{"tenant_id", "store_id", "owner_id", "cart_id", "create_attempt_id"}, []string{"tenant_id", "store_id", "owner_id", "id", "create_attempt_id"}, false},
		{"checkout.orders", "storefront.quotes", []string{"tenant_id", "store_id", "owner_id", "cart_id", "quote_id", "create_attempt_id"}, []string{"tenant_id", "store_id", "owner_id", "cart_id", "id", "create_attempt_id"}, false},
		{"checkout.orders", "storefront.destination_snapshots", []string{"tenant_id", "store_id", "owner_id", "destination_id", "create_attempt_id"}, []string{"tenant_id", "store_id", "owner_id", "id", "create_attempt_id"}, false},
		{"inventory.reservations", "checkout.orders", []string{"tenant_id", "store_id", "checkout_id", "buyer_owner_id", "create_attempt_id"}, []string{"tenant_id", "store_id", "id", "owner_id", "create_attempt_id"}, true},
		{"checkout.orders", "inventory.reservations", []string{"tenant_id", "store_id", "id", "owner_id", "create_attempt_id"}, []string{"tenant_id", "store_id", "id", "buyer_owner_id", "create_attempt_id"}, true},
		{"inventory.ledger", "checkout.orders", []string{"tenant_id", "store_id", "checkout_id", "buyer_owner_id", "create_attempt_id"}, []string{"tenant_id", "store_id", "id", "owner_id", "create_attempt_id"}, false},
		{"checkout.command_results", "checkout.orders", []string{"tenant_id", "store_id", "owner_id", "order_id", "create_attempt_id"}, []string{"tenant_id", "store_id", "owner_id", "id", "create_attempt_id"}, false},
		{"checkout.events", "checkout.orders", []string{"tenant_id", "store_id", "owner_id", "order_id", "create_attempt_id"}, []string{"tenant_id", "store_id", "owner_id", "id", "create_attempt_id"}, false},
	} {
		t.Run(tc.child+"/"+tc.parent, func(t *testing.T) {
			oaProvenanceFK(t, f, tc.child, tc.parent, tc.local, tc.remote, tc.deferred)
		})
	}
}

func oaProvenanceFK(t *testing.T, f *testFixture, child, parent string, local, remote []string, deferred bool) {
	t.Helper()
	var present bool
	err := f.owner.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM pg_constraint c
	 WHERE c.conrelid=to_regclass($1) AND c.confrelid=to_regclass($2) AND c.contype='f'
	 AND c.convalidated AND c.confmatchtype='s' AND c.confupdtype='a' AND c.confdeltype='a'
	 AND c.condeferrable=$5 AND c.condeferred=$5
	 AND ARRAY(SELECT a.attname::text FROM unnest(c.conkey) WITH ORDINALITY k(num,pos)
	 JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attnum=k.num ORDER BY k.pos)=$3::text[]
	 AND ARRAY(SELECT a.attname::text FROM unnest(c.confkey) WITH ORDINALITY k(num,pos)
	 JOIN pg_attribute a ON a.attrelid=c.confrelid AND a.attnum=k.num ORDER BY k.pos)=$4::text[])`, child, parent, local, remote, deferred).Scan(&present)
	if err != nil || !present {
		t.Errorf("missing validated exact FK %s %v -> %s %v deferred=%v: present=%v err=%v", child, local, parent, remote, deferred, present, err)
	}
}

func TestOrderAttemptProvenanceIndependentCartsAndOrigins(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	for _, tc := range []struct {
		table     string
		columns   []string
		predicate string
	}{
		{"storefront.carts", []string{"tenant_id", "store_id", "owner_id"}, "cart_kind='storefront'"},
		{"storefront.carts", []string{"tenant_id", "store_id", "create_attempt_id"}, "cart_kind='order_attempt'"},
		{"storefront.cart_lines", []string{"tenant_id", "store_id", "owner_id", "cart_id", "sku_id"}, "cart_kind='storefront'"},
		{"storefront.cart_lines", []string{"cart_id", "sku_id", "origin_kind", "origin_id"}, "cart_kind='order_attempt'"},
		{"storefront.cart_lines", []string{"cart_id", "line_ordinal"}, ""},
		{"checkout.orders", []string{"tenant_id", "create_attempt_id"}, ""},
	} {
		t.Run(tc.table+"/"+tc.columns[len(tc.columns)-1], func(t *testing.T) {
			var present bool
			err := f.owner.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_index i
			 WHERE i.indrelid=to_regclass($1) AND i.indisunique AND i.indisvalid AND i.indisready
			 AND i.indislive AND i.indexprs IS NULL
			 AND ARRAY(SELECT a.attname::text FROM unnest(i.indkey) WITH ORDINALITY k(num,pos)
			 JOIN pg_attribute a ON a.attrelid=i.indrelid AND a.attnum=k.num
			 WHERE k.pos<=i.indnkeyatts ORDER BY k.pos)=$2::text[]
			 AND coalesce(regexp_replace(pg_get_expr(i.indpred,i.indrelid),
			 '\s|\(|\)|::[a-z_][a-z_0-9.]*','','g'),'')=$3)`, tc.table, tc.columns, tc.predicate).Scan(&present)
			if err != nil || !present {
				t.Errorf("missing independent cart/origin identity %s %v WHERE %s: %v %v", tc.table, tc.columns, tc.predicate, present, err)
			}
		})
	}
	// An old unfiltered owner or cart/SKU key silently blocks independent attempt
	// carts or merges duplicate SKU origins even when the new indexes also exist.
	for _, tc := range []struct {
		table   string
		columns []string
	}{
		{"storefront.carts", []string{"tenant_id", "store_id", "owner_id"}},
		{"storefront.cart_lines", []string{"tenant_id", "store_id", "owner_id", "cart_id", "sku_id"}},
	} {
		var blocked bool
		err := f.owner.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_index i WHERE i.indrelid=to_regclass($1)
		 AND i.indisunique AND i.indpred IS NULL AND i.indexprs IS NULL
		 AND ARRAY(SELECT a.attname::text FROM unnest(i.indkey) WITH ORDINALITY k(num,pos)
		 JOIN pg_attribute a ON a.attrelid=i.indrelid AND a.attnum=k.num
		 WHERE k.pos<=i.indnkeyatts ORDER BY k.pos)=$2::text[])`, tc.table, tc.columns).Scan(&blocked)
		if err != nil || blocked {
			t.Errorf("legacy unfiltered uniqueness blocks attempts: %s present=%v err=%v", tc.table, blocked, err)
		}
	}
	for _, column := range []string{"line_id", "line_ordinal", "origin_kind", "origin_id", "cart_kind"} {
		var present bool
		err := f.owner.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_attribute WHERE attrelid='storefront.cart_lines'::regclass
		 AND attname=$1 AND attnotnull AND NOT attisdropped)`, column).Scan(&present)
		if err != nil || !present {
			t.Errorf("stable cart line column missing/not nullable-safe: %s %v %v", column, present, err)
		}
	}
}

func TestOrderAttemptProvenanceCoordinatorNoDomainDML(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	var role bool
	if err := f.owner.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='commerce_order_attempt_owner')`).Scan(&role); err != nil || !role {
		t.Fatalf("coordinator role not established: %v %v", role, err)
	}
	for _, table := range []string{"storefront.carts", "storefront.cart_lines", "storefront.quotes", "storefront.destination_snapshots", "fulfillment.pickup_versions", "checkout.orders", "inventory.reservations", "inventory.ledger"} {
		var allowed bool
		err := f.owner.QueryRow(ctx, `SELECT has_table_privilege('commerce_order_attempt_owner',to_regclass($1),'INSERT,UPDATE,DELETE,TRUNCATE')
		 OR has_any_column_privilege('commerce_order_attempt_owner',to_regclass($1),'INSERT,UPDATE')`, table).Scan(&allowed)
		if err != nil {
			t.Fatal(err)
		}
		if allowed {
			t.Errorf("coordinator owns direct domain DML on %s", table)
		}
	}
}

func TestOrderAttemptProvenanceEnteredStore(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	oaProvenanceFK(t, f, "fulfillment.pickup_versions", "ops.order_attempt_creators",
		[]string{"tenant_id", "store_id", "buyer_owner_id", "create_attempt_id"},
		[]string{"tenant_id", "store_id", "buyer_owner_id", "attempt_id"}, false)
	var expression string
	if err := f.owner.QueryRow(ctx, `SELECT pg_get_expr(conbin,conrelid) FROM pg_constraint
	 WHERE conrelid='fulfillment.pickup_versions'::regclass AND conname='pickup_versions_attempt_creator'
	 AND contype='c' AND convalidated`).Scan(&expression); err != nil {
		t.Fatalf("missing entered-store attempt provenance: %v", err)
	}
	for _, tc := range []struct {
		kind           string
		owner, attempt any
		want           bool
	}{
		{"BUYER_ENTERED", nil, nil, true},
		{"MANUAL_ATTESTED", nil, nil, true},
		{"PROVIDER_DIRECTORY_VERIFIED", nil, nil, true},
		{"BUYER_ENTERED", "00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000002", true},
		{"BUYER_ENTERED", nil, "00000000-0000-4000-8000-000000000002", false},
		{"BUYER_ENTERED", "00000000-0000-4000-8000-000000000001", nil, false},
		{"MANUAL_ATTESTED", "00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000002", false},
		{"PROVIDER_DIRECTORY_VERIFIED", "00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000002", false},
	} {
		data, err := json.Marshal(map[string]any{"verification_kind": tc.kind, "buyer_owner_id": tc.owner, "create_attempt_id": tc.attempt})
		if err != nil {
			t.Fatal(err)
		}
		var accepted bool
		query := fmt.Sprintf(`SELECT (%s) IS NOT FALSE FROM jsonb_populate_record(NULL::fulfillment.pickup_versions,$1::jsonb)`, expression)
		if err := f.owner.QueryRow(ctx, query, string(data)).Scan(&accepted); err != nil || accepted != tc.want {
			t.Errorf("entered-store creator %s owner=%v attempt=%v: accepted=%v want=%v err=%v", tc.kind, tc.owner, tc.attempt, accepted, tc.want, err)
		}
	}
}

func TestOrderAttemptProvenanceNativeCartsAndOrigins(t *testing.T) {
	h := cqSetup(t)
	browser := h.oneCart(t)
	f := h.f
	ctx := context.Background()
	tx, err := f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	// This is a storage fixture in the isolated foundation DB, not fabricated C1
	// authorization. Runtime admission, entire C2 chain and actual-login rejection
	// remain separate required gates. No capability session is minted for an attempt.
	var sessionsBefore int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM buyer.capability_sessions WHERE tenant_id=$1 AND store_id=$2 AND owner_id=$3`, f.tenantA, f.storeA1, h.cap.Scope.OwnerID).Scan(&sessionsBefore); err != nil {
		t.Fatal(err)
	}
	attempts := []string{randomUUID(), randomUUID()}
	carts := []string{randomUUID(), randomUUID()}
	for i, attempt := range attempts {
		if _, err := tx.Exec(ctx, `INSERT INTO ops.order_attempt_creators(attempt_id,tenant_id,store_id,buyer_owner_id) VALUES($1,$2,$3,$4)`, attempt, f.tenantA, f.storeA1, h.cap.Scope.OwnerID); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO storefront.carts(tenant_id,store_id,owner_id,id,currency,version,cart_kind,create_attempt_id)
		 VALUES($1,$2,$3,$4,$5,1,'order_attempt',$6)`, f.tenantA, f.storeA1, h.cap.Scope.OwnerID, carts[i], browser.Currency, attempt); err != nil {
			t.Fatalf("native attempt cart %d refused: %v", i, err)
		}
	}
	origins := []string{randomUUID(), randomUUID()}
	for i, origin := range origins {
		if _, err := tx.Exec(ctx, `INSERT INTO storefront.cart_lines(tenant_id,store_id,owner_id,cart_id,sku_id,quantity,cart_kind,create_attempt_id,line_ordinal,origin_kind,origin_id)
		 VALUES($1,$2,$3,$4,$5,$6,'order_attempt',$7,$8,'claim',$9)`, f.tenantA, f.storeA1, h.cap.Scope.OwnerID, carts[0], h.stock.skus[0].ID, i+1, attempts[0], i+1, origin); err != nil {
			t.Fatalf("distinct origin for same SKU refused: %v", err)
		}
	}
	var count, quantity, sessionsAfter int
	if err := tx.QueryRow(ctx, `SELECT count(*),sum(quantity) FROM storefront.cart_lines WHERE cart_id=$1`, carts[0]).Scan(&count, &quantity); err != nil || count != 2 || quantity != 3 {
		t.Fatalf("origin preservation sentinel: lines=%d quantity=%d err=%v", count, quantity, err)
	}
	var version int64
	if err := tx.QueryRow(ctx, `SELECT version FROM storefront.carts WHERE tenant_id=$1 AND store_id=$2 AND owner_id=$3 AND id=$4 AND cart_kind='storefront'`, f.tenantA, f.storeA1, h.cap.Scope.OwnerID, browser.ID).Scan(&version); err != nil || version != browser.Version {
		t.Fatalf("browser cart changed: version=%d want=%d err=%v", version, browser.Version, err)
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM buyer.capability_sessions WHERE tenant_id=$1 AND store_id=$2 AND owner_id=$3`, f.tenantA, f.storeA1, h.cap.Scope.OwnerID).Scan(&sessionsAfter); err != nil || sessionsAfter != sessionsBefore {
		t.Fatalf("attempt storage minted a capability: before=%d after=%d err=%v", sessionsBefore, sessionsAfter, err)
	}
	// An unused valid creator keeps the FK oracle independent of the unique
	// attempt/cart index: an existing cart could otherwise reject first as 23505.
	unusedAttempt := randomUUID()
	if _, err := tx.Exec(ctx, `INSERT INTO ops.order_attempt_creators(attempt_id,tenant_id,store_id,buyer_owner_id) VALUES($1,$2,$3,$4)`, unusedAttempt, f.tenantA, f.storeA1, h.cap.Scope.OwnerID); err != nil {
		t.Fatal(err)
	}
	oaProvenanceRejected(t, tx, "23503", `INSERT INTO storefront.carts(tenant_id,store_id,owner_id,currency,version,cart_kind,create_attempt_id)
	 VALUES($1,$2,$3,$4,1,'order_attempt',$5)`, f.tenantA, f.storeA1, randomUUID(), browser.Currency, unusedAttempt)
	oaProvenanceRejected(t, tx, "23503", `INSERT INTO storefront.carts(tenant_id,store_id,owner_id,currency,version,cart_kind,create_attempt_id)
	 VALUES($1,$2,$3,$4,1,'order_attempt',$5)`, f.tenantA, f.storeA2, h.cap.Scope.OwnerID, browser.Currency, unusedAttempt)
	oaProvenanceRejected(t, tx, "23503", `INSERT INTO storefront.cart_lines(tenant_id,store_id,owner_id,cart_id,sku_id,quantity,cart_kind,create_attempt_id,line_ordinal,origin_kind,origin_id)
	 VALUES($1,$2,$3,$4,$5,1,'order_attempt',$6,3,'claim',$7)`, f.tenantA, f.storeA1, h.cap.Scope.OwnerID, carts[0], h.stock.skus[0].ID, attempts[1], randomUUID())
	oaProvenanceRejected(t, tx, "23505", `INSERT INTO storefront.cart_lines(tenant_id,store_id,owner_id,cart_id,sku_id,quantity,cart_kind,create_attempt_id,line_ordinal,origin_kind,origin_id)
	 VALUES($1,$2,$3,$4,$5,1,'order_attempt',$6,3,'claim',$7)`, f.tenantA, f.storeA1, h.cap.Scope.OwnerID, carts[0], h.stock.skus[0].ID, attempts[0], origins[0])
	oaProvenanceRejected(t, tx, "23505", `INSERT INTO storefront.cart_lines(tenant_id,store_id,owner_id,cart_id,sku_id,quantity)
	 VALUES($1,$2,$3,$4,$5,1)`, f.tenantA, f.storeA1, h.cap.Scope.OwnerID, browser.ID, h.stock.skus[0].ID)
}

func oaProvenanceRejected(t *testing.T, tx pgx.Tx, state, query string, args ...any) {
	t.Helper()
	ctx := context.Background()
	if _, err := tx.Exec(ctx, `SAVEPOINT oa_provenance_negative`); err != nil {
		t.Fatal(err)
	}
	_, err := tx.Exec(ctx, query, args...)
	var failure *pgconn.PgError
	if !errors.As(err, &failure) || failure.Code != state {
		t.Errorf("native provenance rejection: want SQLSTATE %s got %v", state, err)
	}
	if _, err := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT oa_provenance_negative`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `RELEASE SAVEPOINT oa_provenance_negative`); err != nil {
		t.Fatal(err)
	}
}
