// Purpose: REAL_PG security and historical-seal regressions for A16R's narrow reservation lookup.
// Depends on: claims.for_buyer_request(bytea,bytea,uuid), actual A16/router fixtures and scoped commerce_runtime transactions.
// Used by: focused recovery acceptance and foundation CI; no production hooks or fabricated HTTP responses.
// Invariants: I01/I02/I04/I06/I11; read-only lookup discloses only the matching server-owner attempt within merchant scope.
package foundation_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/claims"
	"livecommerce/internal/platform"
)

type lcrqPending struct {
	e                 *lbuEnv
	key               string
	body              map[string]any
	keyHash, bodyHash []byte
	buyer, request    string
}

// lcrqNewPending obtains every lookup input from an actual A16 committed reservation, before checkout Begin.
func lcrqNewPending(t *testing.T) lcrqPending {
	t.Helper()
	e := lbuNew(t)
	_, _, bundle, conversation := e.scenario()
	p := lcrqPending{e: e, key: t04Key("lcrq-pending"), body: e.body(2, []string{bundle}, conversation, false)}
	barrier, remove := lcrpFaultBarrier(t, e, "destination")
	lcrpInterrupt(t, e, barrier, func() (int, map[string]any) { return e.post(e.token(), p.key, p.body) })
	remove()
	p.keyHash, _ = lcrpKeys(e, p.key)
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT request_hash,buyer_id::text,request_id::text
 FROM inbox.order_for_buyer WHERE tenant_id=$1 AND store_id=$2 AND idempotency_key_hash=$3 AND state='pending'`,
		e.tenant(), e.store(), p.keyHash).Scan(&p.bodyHash, &p.buyer, &p.request); err != nil {
		t.Fatal("actual pending lookup fixture unavailable")
	}
	lcrpRequireFacts(t, e, p.key, lcrpFacts{Pending: 1, LiveGrants: 1})
	return p
}

func TestLiveConsoleRecoveryReservationLookupReadOnlyAndBinding(t *testing.T) {
	p := lcrqNewPending(t)
	e, ctx := p.e, context.Background()
	before := lcrqLedgerStamp(t, p)
	for _, tc := range []struct {
		name      string
		key, body []byte
		buyer     any
		found     bool
		state     string
		message   string
	}{
		{"own", p.keyHash, p.bodyHash, p.buyer, true, "", ""},
		{"absent", make([]byte, 32), p.bodyHash, p.buyer, false, "", ""},
		{"null-key", nil, p.bodyHash, p.buyer, false, "22023", ""},
		{"short-key", make([]byte, 31), p.bodyHash, p.buyer, false, "22023", ""},
		{"long-key", make([]byte, 33), p.bodyHash, p.buyer, false, "22023", ""},
		{"null-body", p.keyHash, nil, p.buyer, false, "22023", ""},
		{"short-body", p.keyHash, make([]byte, 31), p.buyer, false, "22023", ""},
		{"long-body", p.keyHash, make([]byte, 33), p.buyer, false, "22023", ""},
		{"null-buyer", p.keyHash, p.bodyHash, nil, false, "22023", ""},
		{"wrong-body", p.keyHash, make([]byte, 32), p.buyer, false, "PT409", "idempotency_conflict"},
		{"wrong-owner", p.keyHash, p.bodyHash, randomUUID(), false, "PT409", "idempotency_conflict"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var result *string
			err := platform.WithScope(ctx, e.p.f.runtime, e.token(), e.store(), "inventory:reserve", func(tx pgx.Tx, _ platform.Scope) error {
				// A real READ ONLY transaction also rejects any hidden write inside the definer.
				if _, err := tx.Exec(ctx, `SELECT set_config('transaction_read_only','on',true)`); err != nil {
					return err
				}
				return tx.QueryRow(ctx, `SELECT claims.for_buyer_request($1::bytea,$2::bytea,$3::uuid)::text`, tc.key, tc.body, tc.buyer).Scan(&result)
			})
			lcrqRequireError(t, err, tc.state, tc.message)
			if tc.state == "" && ((result != nil) != tc.found || (tc.found && *result != p.request)) {
				t.Fatal("lookup presence or matching request identity differs")
			}
		})
	}
	if lcrqLedgerStamp(t, p) != before {
		t.Fatal("read-only lookup changed reservation/grant/receipt ledger")
	}
	// A released row still binds its original body/owner but must not invite another grant-expiry update.
	if err := platform.WithScope(ctx, e.p.f.runtime, e.token(), e.store(), "inventory:reserve", func(tx pgx.Tx, _ platform.Scope) error {
		return claims.ReleaseForBuyer(ctx, tx, p.request)
	}); err != nil {
		t.Fatal("production reservation release failed")
	}
	released := lcrqLedgerStamp(t, p)
	for _, tc := range []struct {
		name  string
		body  []byte
		buyer string
		state string
	}{{"released", p.bodyHash, p.buyer, ""}, {"released-wrong-body", make([]byte, 32), p.buyer, "PT409"}, {"released-wrong-owner", p.bodyHash, randomUUID(), "PT409"}} {
		t.Run(tc.name, func(t *testing.T) {
			var result *string
			err := platform.WithScope(ctx, e.p.f.runtime, e.token(), e.store(), "inventory:reserve", func(tx pgx.Tx, _ platform.Scope) error {
				return tx.QueryRow(ctx, `SELECT claims.for_buyer_request($1::bytea,$2::bytea,$3::uuid)::text`, p.keyHash, tc.body, tc.buyer).Scan(&result)
			})
			message := ""
			if tc.state != "" {
				message = "idempotency_conflict"
			}
			lcrqRequireError(t, err, tc.state, message)
			if result != nil {
				t.Fatal("lookup returned a released request")
			}
		})
	}
	if lcrqLedgerStamp(t, p) != released {
		t.Fatal("released lookup changed its ledger")
	}
}

func TestLiveConsoleRecoveryReservationLookupChecksEveryMatchingRow(t *testing.T) {
	p := lcrqNewPending(t)
	e, ctx := p.e, context.Background()
	_, _, historicalBundle, _ := e.scenario()
	// Disclosed owner fixture: a legitimately accepted second bundle records a historical released row of this exact attempt.
	// The real pending row remains untouched; selecting just that row must not hide a mismatched historical tail.
	tag, err := e.p.f.owner.Exec(ctx, `INSERT INTO inbox.order_for_buyer
 (tenant_id,store_id,bundle_id,request_id,idempotency_key_hash,request_hash,buyer_id,state,conversation_id,principal_id)
 SELECT tenant_id,store_id,$4::uuid,request_id,idempotency_key_hash,request_hash,buyer_id,'released',conversation_id,principal_id
 FROM inbox.order_for_buyer WHERE tenant_id=$1 AND store_id=$2 AND idempotency_key_hash=$3 AND state='pending'`,
		e.tenant(), e.store(), p.keyHash, historicalBundle)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatal("historical matching-row fixture insert count differs")
	}
	lcrpRequireFacts(t, e, p.key, lcrpFacts{Pending: 1, Released: 1, LiveGrants: 1})
	probe := func(t *testing.T, token, wantState string) {
		t.Helper()
		before := lcrqLedgerStamp(t, p)
		var result *string
		err := platform.WithScope(ctx, e.p.f.runtime, token, e.store(), "inventory:reserve", func(tx pgx.Tx, _ platform.Scope) error {
			if _, err := tx.Exec(ctx, `SELECT set_config('transaction_read_only','on',true)`); err != nil {
				return err
			}
			return tx.QueryRow(ctx, `SELECT claims.for_buyer_request($1::bytea,$2::bytea,$3::uuid)::text`, p.keyHash, p.bodyHash, p.buyer).Scan(&result)
		})
		message := ""
		if wantState != "" {
			message = "idempotency_conflict"
		}
		lcrqRequireError(t, err, wantState, message)
		if wantState == "" && (result == nil || *result != p.request) {
			t.Fatal("mixed pending/released lookup did not return the original pending request")
		}
		if lcrqLedgerStamp(t, p) != before {
			t.Fatal("mixed-row read-only lookup changed its ledger")
		}
	}
	probe(t, e.token(), "")
	// A different authorized store principal may read the same server-owner binding; stored principal is not request identity.
	probe(t, e.member(), "")
	for _, tc := range []struct {
		name, assignment string
		value            any
	}{
		{"historical-body", "request_hash=$4::bytea", make([]byte, 32)},
		{"historical-owner", "buyer_id=$4::uuid", randomUUID()},
		{"historical-request", "request_id=$4::uuid", randomUUID()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Each mutation touches only the released tail. Restore all bound fields even when the assertion fails.
			t.Cleanup(func() {
				mustExec(t, e.p.f.owner, `UPDATE inbox.order_for_buyer SET request_hash=$4,buyer_id=$5::uuid,request_id=$6::uuid
 WHERE tenant_id=$1 AND store_id=$2 AND bundle_id=$3 AND state='released'`,
					e.tenant(), e.store(), historicalBundle, p.bodyHash, p.buyer, p.request)
			})
			tag, err := e.p.f.owner.Exec(ctx, `UPDATE inbox.order_for_buyer SET `+tc.assignment+`
 WHERE tenant_id=$1 AND store_id=$2 AND bundle_id=$3 AND state='released'`,
				e.tenant(), e.store(), historicalBundle, tc.value)
			if err != nil || tag.RowsAffected() != 1 {
				t.Fatal("historical tail mutation count differs")
			}
			probe(t, e.token(), "PT409")
		})
		probe(t, e.token(), "")
	}
	lcrpRequireFacts(t, e, p.key, lcrpFacts{Pending: 1, Released: 1, LiveGrants: 1})
}

func TestLiveConsoleRecoveryReservationLookupScopeAndAuthority(t *testing.T) {
	p := lcrqNewPending(t)
	e, ctx := p.e, context.Background()
	before := lcrqLedgerStamp(t, p)
	// psSetup clones the base tenant and retains only its own token. Build a real same-tenant foreign store;
	// use the shared base's separately authenticated tenant for the cross-tenant probe.
	foreignStore := randomUUID()
	mustExec(t, e.p.f.owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'recovery isolation fixture','TWD')`, e.tenant(), foreignStore)
	mustExec(t, e.p.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'store:read')`, e.tenant(), foreignStore, e.p.f.principalA)
	base := fixture(t)
	for _, tc := range []struct{ name, token, store string }{
		{"foreign-store", e.token(), foreignStore},
		{"foreign-tenant", base.tokens["b"], base.storeB},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var scope platform.Scope
			if err := platform.WithScope(ctx, e.p.f.runtime, tc.token, tc.store, "store:read", func(_ pgx.Tx, s platform.Scope) error { scope = s; return nil }); err != nil {
				t.Fatal("foreign fixture scope unavailable")
			}
			// Fixture-only grant: this authentic foreign principal passes the permission gate, so NULL proves isolation.
			tag, err := e.p.f.owner.Exec(ctx, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission)
 VALUES($1,$2,$3,'inventory:reserve') ON CONFLICT DO NOTHING`, scope.TenantID, scope.StoreID, scope.PrincipalID)
			if err != nil {
				t.Fatal("foreign fixture reserve grant failed")
			}
			if tag.RowsAffected() == 1 {
				t.Cleanup(func() {
					mustExec(t, e.p.f.owner, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='inventory:reserve'`, scope.TenantID, scope.StoreID, scope.PrincipalID)
				})
			}
			var result *string
			err = platform.WithScope(ctx, e.p.f.runtime, tc.token, tc.store, "inventory:reserve", func(tx pgx.Tx, _ platform.Scope) error {
				return tx.QueryRow(ctx, `SELECT claims.for_buyer_request($1::bytea,$2::bytea,$3::uuid)::text`, p.keyHash, p.bodyHash, p.buyer).Scan(&result)
			})
			lcrqRequireError(t, err, "", "")
			if result != nil {
				t.Fatal("foreign scope disclosed a reservation request")
			}
		})
	}
	t.Run("missing-reserve", func(t *testing.T) {
		token, _ := e.tcvEnv.member("orders:read")
		err := platform.WithScope(ctx, e.p.f.runtime, token, e.store(), "orders:read", func(tx pgx.Tx, _ platform.Scope) error {
			var result *string
			return tx.QueryRow(ctx, `SELECT claims.for_buyer_request($1::bytea,$2::bytea,$3::uuid)::text`, p.keyHash, p.bodyHash, p.buyer).Scan(&result)
		})
		lcrqRequireError(t, err, "PT403", "")
	})
	t.Run("buyer-guc", func(t *testing.T) {
		err := platform.WithScope(ctx, e.p.f.runtime, e.token(), e.store(), "inventory:reserve", func(tx pgx.Tx, _ platform.Scope) error {
			if _, err := tx.Exec(ctx, `SELECT set_config('app.buyer_id',$1,true)`, p.buyer); err != nil {
				return err
			}
			var result *string
			return tx.QueryRow(ctx, `SELECT claims.for_buyer_request($1::bytea,$2::bytea,$3::uuid)::text`, p.keyHash, p.bodyHash, p.buyer).Scan(&result)
		})
		lcrqRequireError(t, err, "22023", "")
	})
	for _, tc := range []struct {
		name   string
		iso    pgx.TxIsoLevel
		scoped bool
	}{
		{"no-scope", pgx.ReadCommitted, false}, {"repeatable-read", pgx.RepeatableRead, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var scope platform.Scope
			if tc.scoped {
				if err := platform.WithScope(ctx, e.p.f.runtime, e.token(), e.store(), "inventory:reserve", func(_ pgx.Tx, s platform.Scope) error { scope = s; return nil }); err != nil {
					t.Fatal("lookup scope fixture failed")
				}
			}
			tx, err := e.p.f.runtime.BeginTx(ctx, pgx.TxOptions{IsoLevel: tc.iso})
			if err != nil {
				t.Fatal("lookup guard transaction failed")
			}
			defer tx.Rollback(ctx)
			if tc.scoped {
				_, err = tx.Exec(ctx, `SELECT set_config('app.tenant_id',$1,true),set_config('app.store_id',$2,true),set_config('app.principal_id',$3,true)`, scope.TenantID, scope.StoreID, scope.PrincipalID)
			} else {
				_, err = tx.Exec(ctx, `SELECT set_config('app.tenant_id','',true),set_config('app.store_id','',true),set_config('app.principal_id','',true),set_config('app.buyer_id','',true)`)
			}
			if err != nil {
				t.Fatal("lookup guard GUC fixture failed")
			}
			var result *string
			err = tx.QueryRow(ctx, `SELECT claims.for_buyer_request($1::bytea,$2::bytea,$3::uuid)::text`, p.keyHash, p.bodyHash, p.buyer).Scan(&result)
			lcrqRequireError(t, err, "22023", "")
		})
	}
	if lcrqLedgerStamp(t, p) != before {
		t.Fatal("authority probes changed reservation/grant/receipt ledger")
	}
}

func TestLiveConsoleRecoveryReservationLookupNarrowACL(t *testing.T) {
	e, ctx := lbuNew(t), context.Background()
	var narrow bool
	err := e.p.f.owner.QueryRow(ctx, `SELECT p.prosecdef AND p.proowner='commerce_claims_writer'::regrole
 AND p.prorettype='uuid'::regtype AND p.proconfig=ARRAY['search_path=pg_catalog']
 AND has_function_privilege('commerce_runtime',p.oid,'EXECUTE')
 AND NOT EXISTS(SELECT 1 FROM aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a
 WHERE a.privilege_type='EXECUTE' AND a.grantee NOT IN ('commerce_runtime'::regrole,p.proowner))
 AND NOT EXISTS(SELECT 1 FROM aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a
 WHERE a.privilege_type='EXECUTE' AND a.grantee='commerce_runtime'::regrole AND a.is_grantable)
 AND NOT has_table_privilege('commerce_runtime','inbox.order_for_buyer','SELECT')
 FROM pg_proc p WHERE p.oid=to_regprocedure('claims.for_buyer_request(bytea,bytea,uuid)')`).Scan(&narrow)
	if err != nil || !narrow {
		t.Fatal("reservation lookup owner/definer/search_path/execute/table ACL pin failed")
	}
	err = platform.WithScope(ctx, e.p.f.runtime, e.token(), e.store(), "inventory:reserve", func(tx pgx.Tx, _ platform.Scope) error {
		var n int
		return tx.QueryRow(ctx, `SELECT count(*) FROM inbox.order_for_buyer`).Scan(&n)
	})
	lcrqRequireError(t, err, "42501", "")
}

func TestLiveConsoleRecoveryHistoricalNotCreatedRepairsPendingOnce(t *testing.T) {
	p := lcrqNewPending(t)
	e, ctx := p.e, context.Background()
	_, beginKey := lcrpKeys(e, p.key)
	ownerJSON, err := json.Marshal(struct {
		OwnerID string `json:"owner_id"`
	}{p.buyer})
	if err != nil {
		t.Fatal("historical seal encoding failed")
	}
	ownerHash := sha256.Sum256(ownerJSON)
	// Owner-only historical fixture: both append-only receipts predate repair, while the real reservation/grant remain pending.
	tx, err := e.p.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal("historical fixture transaction failed")
	}
	defer tx.Rollback(ctx)
	for _, query := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO ops.command_results(tenant_id,store_id,operation,idempotency_key,request_hash,response,principal_id)
 SELECT tenant_id,store_id,'merchanttools.order.checkout.closed',$4,$5,$6,principal_id FROM ops.command_results
 WHERE tenant_id=$1 AND store_id=$2 AND operation='merchanttools.order.attempt' AND idempotency_key=$3`, []any{e.tenant(), e.store(), p.key, beginKey, ownerHash[:], ownerJSON}},
		{`INSERT INTO ops.command_results(tenant_id,store_id,operation,idempotency_key,request_hash,response,principal_id)
 SELECT tenant_id,store_id,'merchanttools.order.attempt.recovery',idempotency_key,request_hash,'{"state":"not_created"}'::jsonb,principal_id
 FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND operation='merchanttools.order.attempt' AND idempotency_key=$3`, []any{e.tenant(), e.store(), p.key}},
	} {
		tag, err := tx.Exec(ctx, query.sql, query.args...)
		if err != nil || tag.RowsAffected() != 1 {
			t.Fatal("historical fixture receipt insert count differs")
		}
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal("historical fixture commit failed")
	}
	lcrpRequireFacts(t, e, p.key, lcrpFacts{Pending: 1, LiveGrants: 1, Seals: 1, Recoveries: 1})
	oldReceipts := lcrqReceiptStamp(t, p)
	status, out := lcrRecover(t, e, e.token(), p.key, p.body)
	if status != 200 || out["state"] != "not_created" {
		t.Fatalf("historical repair status=%d state=%s", status, lbuStr(out, "state"))
	}
	lcrpRequireFacts(t, e, p.key, lcrpFacts{Released: 1, ExpiredGrants: 1, Seals: 1, Recoveries: 1})
	if n := e.count(`SELECT count(*) FROM inbox.order_for_buyer x JOIN claims.merchant_origin_grants g
 ON (g.tenant_id,g.store_id,g.request_id,g.bundle_id)=(x.tenant_id,x.store_id,x.request_id,x.bundle_id)
 WHERE x.tenant_id=$1 AND x.store_id=$2 AND x.idempotency_key_hash=$3 AND x.xmin=g.xmin
 AND x.state='released' AND g.consumed_at IS NULL AND g.expires_at<=clock_timestamp()`, e.tenant(), e.store(), p.keyHash); n != 1 {
		t.Fatalf("historical atomic release/grant count=%d want=1", n)
	}
	if lcrqReceiptStamp(t, p) != oldReceipts {
		t.Fatal("historical repair replaced append-only receipts")
	}
	repaired := lcrqLedgerStamp(t, p)
	status, out = lcrRecover(t, e, e.token(), p.key, p.body)
	if status != 200 || out["state"] != "not_created" {
		t.Fatalf("historical repeat status=%d state=%s", status, lbuStr(out, "state"))
	}
	if lcrqLedgerStamp(t, p) != repaired {
		t.Fatal("historical recovery replay changed already-released ledger")
	}
	status, out = e.post(e.token(), p.key, p.body)
	if status != 409 || lbuCode(out) != "attempt_closed" {
		t.Fatalf("historical late original status=%d code=%s", status, lbuCode(out))
	}
}

func lcrqRequireError(t *testing.T, err error, state, message string) {
	t.Helper()
	if state == "" {
		if err != nil {
			t.Fatal("reservation lookup unexpectedly failed")
		}
		return
	}
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != state || (message != "" && pg.Message != message) {
		t.Fatalf("reservation lookup SQLSTATE=%s want=%s expected_message_match=%t", sqlState(err), state, pg != nil && pg.Message == message)
	}
}

// Ledger stamps compare row versions privately; only boolean/count/status failures enter evidence.
func lcrqLedgerStamp(t *testing.T, p lcrqPending) string {
	t.Helper()
	var stamp string
	err := p.e.p.f.owner.QueryRow(context.Background(), `SELECT
 coalesce((SELECT string_agg(x.xmin::text,',' ORDER BY x.request_id,x.bundle_id) FROM inbox.order_for_buyer x WHERE tenant_id=$1 AND store_id=$2),'')||'|'||
 coalesce((SELECT string_agg(g.xmin::text,',' ORDER BY g.request_id,g.bundle_id) FROM claims.merchant_origin_grants g WHERE tenant_id=$1 AND store_id=$2),'')||'|'||
 coalesce((SELECT string_agg(r.xmin::text,',' ORDER BY r.operation,r.idempotency_key) FROM ops.command_results r WHERE tenant_id=$1 AND store_id=$2),'')`, p.e.tenant(), p.e.store()).Scan(&stamp)
	if err != nil {
		t.Fatal("reservation lookup ledger stamp failed")
	}
	return stamp
}

func lcrqReceiptStamp(t *testing.T, p lcrqPending) string {
	t.Helper()
	var stamp string
	_, beginKey := lcrpKeys(p.e, p.key)
	err := p.e.p.f.owner.QueryRow(context.Background(), `SELECT coalesce(string_agg(xmin::text,',' ORDER BY operation),'') FROM ops.command_results
 WHERE tenant_id=$1 AND store_id=$2 AND ((operation='merchanttools.order.checkout.closed' AND idempotency_key=$3)
 OR (operation='merchanttools.order.attempt.recovery' AND idempotency_key=$4))`, p.e.tenant(), p.e.store(), beginKey, p.key).Scan(&stamp)
	if err != nil {
		t.Fatal("historical receipt stamp failed")
	}
	return stamp
}
