// Purpose: REAL_PG acceptance of C2's private order-only pay-entry creation seam.
// Depends on: real admission, the execution fence, native Begin, and PayEntryKeys.
// Used by: focused OA1 CI before migration 0187; no complete-C2 receipt is forged.
// Invariants: native order provenance, locked validity, metadata-only exact replay,
// absolute lifetime, private domain grants, and rollback with all Begin effects.
package foundation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/orderattempt"
)

type oaPayEntryStoreWire struct {
	OrderID     string    `json:"order_id"`
	Source      string    `json:"source"`
	LinkVersion int64     `json:"link_version"`
	ExpiresAt   time.Time `json:"pay_entry_expires_at"`
}

func oaPayEntryStoreCall(tx pgx.Tx, attempt string, plain []byte, key string, digest, ciphertext []byte) ([]byte, error) {
	var raw []byte
	// Calls checkout.order_attempt_create_pay_entry, order-attempt-v1 §§3/4.
	err := tx.QueryRow(context.Background(), `SELECT checkout.order_attempt_create_pay_entry($1::uuid,$2::text,$3::text,$4::bytea,$5::bytea)`, attempt, string(plain), key, digest, ciphertext).Scan(&raw)
	return raw, err
}

func oaPayEntryStorePrepared(t *testing.T, e *lbuEnv, mode string, hours int) (orderattempt.AdmissionReceipt, pgx.Tx, []byte, orderattempt.PayEntryMaterial) {
	t.Helper()
	selection := oaBeginSelection(t, e, mode, "duplicate-origins")
	if mode == "bank_transfer" {
		mustExec(t, e.p.f.owner, `UPDATE checkout.bank_transfer_settings SET window_hours=$2,version=version+1 WHERE store_id=$1`, e.store(), hours)
	}
	a := oaAllocationAdmit(t, e, selection)
	tx, plain, quote, destination, _, lines := oaBeginPrepared(t, e, a)
	oaAbortCleanupTx(t, tx)
	got, _, err := oaBeginCall(tx, a.AttemptID, plain, quote, destination, lines)
	if err != nil || got.OrderID != a.AttemptID || got.JobID <= 0 {
		t.Fatalf("real native Begin prerequisite, not a fabricated order: %v", err)
	}
	material, err := payEntryTestKeys(t).Issue(orderattempt.PayEntryBinding{TenantID: e.tenant(), StoreID: e.store(), OrderID: a.AttemptID, Version: 1})
	if err != nil {
		t.Fatal("synthetic pay-entry material prerequisite")
	}
	return a, tx, plain, material
}

func TestOrderAttemptPayEntryStoreCreateReplayAndRollback(t *testing.T) {
	for _, tc := range []struct {
		name, mode string
		hours      int
	}{
		{"absolute-72h", "bank_transfer", 168}, {"earlier-order-deadline", "bank_transfer", 24},
		{"pickup", "pay_at_pickup", 0}, {"cod", "cash_on_delivery", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := lbuNew(t)
			a, tx, plain, material := oaPayEntryStorePrepared(t, e, tc.mode, tc.hours)
			oaStateExec(t, tx, `SELECT set_config('app.tenant_id','caller-marker',true),set_config('app.store_id','caller-marker',true),set_config('app.principal_id','caller-marker',true),set_config('app.buyer_id','caller-marker',true),set_config('app.buyer_session_id','caller-marker',true),set_config('app.order_attempt_id','caller-marker',true)`)
			raw, err := oaPayEntryStoreCall(tx, a.AttemptID, plain, material.KeyVersion, material.Digest[:], material.Ciphertext)
			if err != nil {
				t.Fatalf("pay-entry creation after native Begin: %v", err)
			}
			var got oaPayEntryStoreWire
			var fields map[string]json.RawMessage
			if err = json.Unmarshal(raw, &got); err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(raw, &fields); err != nil {
				t.Fatal(err)
			}
			if len(fields) != 4 || got.OrderID != a.AttemptID || got.Source != "merchant_manual" || got.LinkVersion != 1 || got.ExpiresAt.IsZero() {
				t.Fatal("receipt must contain exactly order_id/source/link_version/pay_entry_expires_at, never a bearer")
			}
			var exact bool
			err = tx.QueryRow(context.Background(), `SELECT current_setting('app.tenant_id')='caller-marker' AND current_setting('app.store_id')='caller-marker' AND current_setting('app.principal_id')='caller-marker' AND current_setting('app.buyer_id')='caller-marker' AND current_setting('app.buyer_session_id')='caller-marker' AND current_setting('app.order_attempt_id')='caller-marker'`).Scan(&exact)
			if err != nil || !exact {
				t.Fatalf("pay-entry helper leaked protected scope: %v", err)
			}
			oaStateExec(t, tx, `RESET ROLE`)
			var original string
			err = tx.QueryRow(context.Background(), `SELECT to_jsonb(p)::text,
			 p.tenant_id=o.tenant_id AND p.store_id=o.store_id AND p.owner_id=o.owner_id AND p.order_id=o.id
			 AND p.create_attempt_id=o.create_attempt_id AND o.creator_kind='order_attempt' AND o.creator_session_id IS NULL
			 AND p.order_id=$1::uuid AND p.create_attempt_id=$1::uuid AND p.link_version=1 AND p.revoked_at IS NULL
			 AND p.key_version=$2 AND p.token_hash=$3 AND p.credential_ciphertext=$4 AND o.source='merchant_manual'
			 AND p.expires_at=LEAST(o.expires_at,p.issued_at+interval '72 hours') AND p.expires_at=$5
			 AND p.issued_at>=o.created_at AND p.issued_at<=clock_timestamp() AND p.expires_at>clock_timestamp()
			 FROM checkout.order_pay_entries p JOIN checkout.orders o ON o.id=p.order_id WHERE p.order_id=$1::uuid`,
				a.AttemptID, material.KeyVersion, material.Digest[:], material.Ciphertext, got.ExpiresAt).Scan(&original, &exact)
			if err != nil || !exact {
				t.Fatalf("native provenance, material and lifetime: %v", err)
			}
			if tc.hours == 168 {
				err = tx.QueryRow(context.Background(), `SELECT expires_at=issued_at+interval '72 hours' FROM checkout.order_pay_entries WHERE order_id=$1`, a.AttemptID).Scan(&exact)
				if err != nil || !exact {
					t.Fatalf("72-hour absolute cap sentinel: %v", err)
				}
			} else {
				err = tx.QueryRow(context.Background(), `SELECT p.expires_at=o.expires_at FROM checkout.order_pay_entries p JOIN checkout.orders o ON o.id=p.order_id WHERE p.order_id=$1`, a.AttemptID).Scan(&exact)
				if err != nil || !exact {
					t.Fatalf("earlier native order deadline sentinel: %v", err)
				}
			}
			for _, fresh := range []bool{false, true} {
				candidate := material
				if fresh {
					candidate, err = payEntryTestKeys(t).Issue(orderattempt.PayEntryBinding{TenantID: e.tenant(), StoreID: e.store(), OrderID: a.AttemptID, Version: 1})
					if err != nil || candidate.Digest == material.Digest {
						t.Fatal("different-candidate replay prerequisite")
					}
				}
				oaStateExec(t, tx, `SET LOCAL ROLE commerce_order_attempt_owner`)
				again, callErr := oaPayEntryStoreCall(tx, a.AttemptID, plain, candidate.KeyVersion, candidate.Digest[:], candidate.Ciphertext)
				if callErr != nil || !bytes.Equal(raw, again) {
					t.Fatalf("same intent must replay original metadata: %v", callErr)
				}
				oaStateExec(t, tx, `RESET ROLE`)
				var current string
				err = tx.QueryRow(context.Background(), `SELECT to_jsonb(p)::text FROM checkout.order_pay_entries p WHERE order_id=$1`, a.AttemptID).Scan(&current)
				if err != nil || current != original {
					t.Fatalf("replay overwrote crypto, version or first-issuance lifetime: %v", err)
				}
			}
			err = tx.QueryRow(context.Background(), `SELECT
			 (SELECT count(*) FROM checkout.order_pay_entries WHERE order_id=$1)=1
			 AND NOT EXISTS(SELECT 1 FROM checkout.order_links WHERE order_id=$1)
			 AND NOT EXISTS(SELECT 1 FROM checkout.payment_attempts WHERE order_id=$1)
			 AND EXISTS(SELECT 1 FROM ops.order_attempts WHERE attempt_id=$1 AND state='PENDING' AND receipt IS NULL AND order_id IS NULL)`, a.AttemptID).Scan(&exact)
			if err != nil || !exact {
				t.Fatalf("no legacy bearer/provider attempt or fabricated full-C2 receipt: %v", err)
			}
			if err = tx.Rollback(context.Background()); err != nil {
				t.Fatal(err)
			}
			oaBeginZeroEffects(t, e, a.AttemptID)
			if n := e.count(`SELECT count(*) FROM checkout.order_pay_entries WHERE create_attempt_id=$1`, a.AttemptID); n != 0 {
				t.Fatal("entry survived outer rollback")
			}
		})
	}
}

func TestOrderAttemptPayEntryStoreContextAndInputDenials(t *testing.T) {
	for _, change := range []string{"changed-plan", "foreign-attempt", "empty-key", "bad-key", "nil-digest", "short-digest", "nil-ciphertext", "short-ciphertext"} {
		t.Run(change, func(t *testing.T) {
			e := lbuNew(t)
			a, tx, plain, material := oaPayEntryStorePrepared(t, e, "bank_transfer", 72)
			attempt, key, digest, ciphertext, code := a.AttemptID, material.KeyVersion, material.Digest[:], material.Ciphertext, "PT400"
			switch change {
			case "changed-plan":
				plain = append(plain, ' ')
			case "foreign-attempt":
				attempt = randomUUID()
				code = "PT409"
			case "empty-key":
				key = ""
			case "bad-key":
				key = "bad/key"
			case "nil-digest":
				digest = nil
			case "short-digest":
				digest = digest[:31]
			case "nil-ciphertext":
				ciphertext = nil
			case "short-ciphertext":
				ciphertext = ciphertext[:70]
			}
			raw, err := oaPayEntryStoreCall(tx, attempt, plain, key, digest, ciphertext)
			oaFenceCode(t, raw, err, code)
			_ = tx.Rollback(context.Background())
			oaBeginZeroEffects(t, e, a.AttemptID)
		})
	}
}

func TestOrderAttemptPayEntryStoreRefusesInvalidReplay(t *testing.T) {
	for _, change := range []string{"cancelled", "paid", "deadline", "revoked", "expired-entry", "released-hold", "erased-owner", "collected-pickup", "collected-cod"} {
		t.Run(change, func(t *testing.T) {
			e := lbuNew(t)
			mode := "bank_transfer"
			if change == "collected-pickup" {
				mode = "pay_at_pickup"
			}
			if change == "collected-cod" {
				mode = "cash_on_delivery"
			}
			a, tx, plain, material := oaPayEntryStorePrepared(t, e, mode, 72)
			if _, err := oaPayEntryStoreCall(tx, a.AttemptID, plain, material.KeyVersion, material.Digest[:], material.Ciphertext); err != nil {
				t.Fatalf("initial entry sentinel: %v", err)
			}
			oaStateExec(t, tx, `RESET ROLE`)
			// Counterexamples alter real rows within this rollback-only transaction;
			// they do not pretend to execute cancellation/collection public commands.
			switch change {
			case "cancelled":
				oaStateExec(t, tx, `UPDATE checkout.orders SET commercial_state='CANCELLED' WHERE id=$1`, a.AttemptID)
			case "paid":
				oaStateExec(t, tx, `UPDATE checkout.orders SET commercial_state='CONFIRMED' WHERE id=$1`, a.AttemptID)
			case "deadline":
				oaStateExec(t, tx, `UPDATE checkout.orders SET expires_at=created_at+interval '1 microsecond' WHERE id=$1`, a.AttemptID)
			case "revoked":
				oaStateExec(t, tx, `UPDATE checkout.order_pay_entries SET revoked_at=clock_timestamp() WHERE order_id=$1`, a.AttemptID)
			case "expired-entry":
				oaStateExec(t, tx, `UPDATE checkout.order_pay_entries SET issued_at=clock_timestamp()-interval '2 hours',expires_at=clock_timestamp()-interval '1 hour' WHERE order_id=$1`, a.AttemptID)
			case "released-hold":
				oaStateExec(t, tx, `UPDATE inventory.reservations SET state='RELEASED' WHERE id=$1`, a.AttemptID)
			case "erased-owner":
				oaStateExec(t, tx, `UPDATE buyer.owners SET active=false WHERE id=(SELECT owner_id FROM checkout.orders WHERE id=$1)`, a.AttemptID)
			case "collected-pickup", "collected-cod":
				oaStateExec(t, tx, `UPDATE checkout.orders SET collection_state='COLLECTED' WHERE id=$1`, a.AttemptID)
			}
			oaStateExec(t, tx, `SET LOCAL ROLE commerce_order_attempt_owner`)
			raw, err := oaPayEntryStoreCall(tx, a.AttemptID, plain, material.KeyVersion, material.Digest[:], material.Ciphertext)
			oaFenceCode(t, raw, err, "PT409")
			_ = tx.Rollback(context.Background())
			oaBeginZeroEffects(t, e, a.AttemptID)
		})
	}
}

func TestOrderAttemptPayEntryStorePrivateACLAndUnfenced(t *testing.T) {
	e := lbuNew(t)
	a := oaAllocationAdmit(t, e, oaBeginSelection(t, e, "bank_transfer", "duplicate-origins"))
	t.Run("fenced-but-no-native-order", func(t *testing.T) {
		tx, plain, _, _, _, _ := oaBeginPrepared(t, e, a)
		oaAbortCleanupTx(t, tx)
		raw, err := oaPayEntryStoreCall(tx, a.AttemptID, plain, "v1", bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 71))
		oaFenceCode(t, raw, err, "PT409")
	})
	for _, role := range []string{"commerce_runtime", "commerce_claims_worker", "commerce_buyer_runtime", "commerce_checkout_runtime", "commerce_buyer_writer", "commerce_privacy_writer", "commerce_order_attempt_owner"} {
		t.Run(role, func(t *testing.T) {
			tx, err := e.p.f.owner.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			oaAbortCleanupTx(t, tx)
			oaStateExec(t, tx, `SET LOCAL ROLE `+pgx.Identifier{role}.Sanitize())
			raw, err := oaPayEntryStoreCall(tx, a.AttemptID, []byte(`{}`), "v1", bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 71))
			code := "42501"
			if role == "commerce_order_attempt_owner" {
				code = "PT409"
			}
			oaFenceCode(t, raw, err, code)
		})
	}
	var exact bool
	err := e.p.f.owner.QueryRow(context.Background(), `SELECT p.prosecdef AND p.provolatile='v' AND p.prorettype='jsonb'::regtype
	 AND r.rolname='commerce_checkout_writer' AND NOT r.rolcanlogin AND NOT r.rolsuper AND NOT r.rolbypassrls
	 AND p.proconfig=ARRAY['search_path=pg_catalog'] AND obj_description(p.oid,'pg_proc') IS NOT NULL
	 AND (SELECT count(*) FROM aclexplode(p.proacl) x WHERE x.grantee<>p.proowner)=1
	 AND EXISTS(SELECT 1 FROM aclexplode(p.proacl) x JOIN pg_roles g ON g.oid=x.grantee WHERE g.rolname='commerce_order_attempt_owner' AND x.privilege_type='EXECUTE' AND NOT x.is_grantable)
	 AND EXISTS(SELECT 1 FROM pg_class c WHERE c.oid='checkout.order_pay_entries'::regclass AND c.relrowsecurity AND c.relforcerowsecurity)
	 AND has_table_privilege('commerce_checkout_writer','checkout.order_pay_entries','SELECT')
	 AND has_table_privilege('commerce_checkout_writer','checkout.order_pay_entries','INSERT')
	 FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid='checkout.order_attempt_create_pay_entry(uuid,text,text,bytea,bytea)'::regprocedure`).Scan(&exact)
	if err != nil || !exact {
		t.Fatalf("pay-entry exact helper ownership/ACL and forced RLS: %v", err)
	}
	for _, role := range []string{"commerce_runtime", "commerce_claims_worker", "commerce_order_attempt_owner"} {
		for _, privilege := range []string{"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER"} {
			var granted bool
			if err = e.p.f.owner.QueryRow(context.Background(), `SELECT has_table_privilege($1,'checkout.order_pay_entries',$2)`, role, privilege).Scan(&granted); err != nil || granted {
				t.Fatalf("direct pay-entry table grant %s/%s: %v", role, privilege, err)
			}
		}
		for _, privilege := range []string{"SELECT", "INSERT", "UPDATE", "REFERENCES"} {
			var granted bool
			if err = e.p.f.owner.QueryRow(context.Background(), `SELECT has_any_column_privilege($1,'checkout.order_pay_entries',$2)`, role, privilege).Scan(&granted); err != nil || granted {
				t.Fatalf("direct pay-entry column grant %s/%s: %v", role, privilege, err)
			}
		}
	}
}

func TestOrderAttemptPayEntryStoreNativeConstraints(t *testing.T) {
	e := lbuNew(t)
	a, tx, plain, material := oaPayEntryStorePrepared(t, e, "bank_transfer", 72)
	if _, err := oaPayEntryStoreCall(tx, a.AttemptID, plain, material.KeyVersion, material.Digest[:], material.Ciphertext); err != nil {
		t.Fatalf("native pay-entry row sentinel: %v", err)
	}
	oaStateExec(t, tx, `RESET ROLE`)
	for _, tc := range []struct{ name, assignment, code string }{
		{"orphan-order", "order_id='11111111-1111-4111-8111-111111111111'", "23503"},
		{"wrong-owner", "owner_id='11111111-1111-4111-8111-111111111111'", "23503"},
		{"wrong-store", "store_id='11111111-1111-4111-8111-111111111111'", "23503"},
		{"wrong-attempt", "create_attempt_id='11111111-1111-4111-8111-111111111111'", "23503"},
		{"null-attempt", "create_attempt_id=NULL", "23502"},
		{"zero-version", "link_version=0", "23514"},
		{"bad-key-version", "key_version='bad/key'", "23514"},
		{"short-hash", "token_hash=decode('01','hex')", "23514"},
		{"short-ciphertext", "credential_ciphertext=decode('01','hex')", "23514"},
		{"unbounded-lifetime", "expires_at=issued_at+interval '72 hours 1 second'", "23514"},
		{"backward-lifetime", "expires_at=issued_at", "23514"},
		{"backward-revocation", "revoked_at=issued_at-interval '1 second'", "23514"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			oaStateExec(t, tx, `SAVEPOINT invalid_entry`)
			_, err := tx.Exec(context.Background(), `UPDATE checkout.order_pay_entries SET `+tc.assignment+` WHERE order_id=$1`, a.AttemptID)
			var pgerr *pgconn.PgError
			ok := errors.As(err, &pgerr) && pgerr.Code == tc.code
			oaStateExec(t, tx, `ROLLBACK TO SAVEPOINT invalid_entry`)
			if !ok {
				t.Fatalf("native pay-entry constraint must reject %s with %s", tc.name, tc.code)
			}
		})
	}
	for _, tc := range []struct{ name, version, digest, revocation string }{
		{"duplicate-version", "1", "decode(repeat('ab',32),'hex')", "NULL"},
		{"second-unrevoked", "2", "decode(repeat('ac',32),'hex')", "NULL"},
		{"duplicate-digest", "2", "token_hash", "issued_at"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			oaStateExec(t, tx, `SAVEPOINT duplicate_entry`)
			_, err := tx.Exec(context.Background(), `INSERT INTO checkout.order_pay_entries(tenant_id,store_id,owner_id,order_id,create_attempt_id,link_version,key_version,token_hash,credential_ciphertext,issued_at,expires_at,revoked_at)
			 SELECT tenant_id,store_id,owner_id,order_id,create_attempt_id,`+tc.version+`,key_version,`+tc.digest+`,credential_ciphertext,issued_at,expires_at,`+tc.revocation+` FROM checkout.order_pay_entries WHERE order_id=$1`, a.AttemptID)
			var pgerr *pgconn.PgError
			ok := errors.As(err, &pgerr) && pgerr.Code == "23505"
			oaStateExec(t, tx, `ROLLBACK TO SAVEPOINT duplicate_entry`)
			if !ok {
				t.Fatalf("native pay-entry uniqueness must reject %s", tc.name)
			}
		})
	}
	_ = tx.Rollback(context.Background())
	oaBeginZeroEffects(t, e, a.AttemptID)
}
