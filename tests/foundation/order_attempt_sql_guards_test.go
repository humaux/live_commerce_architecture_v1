// Purpose: REAL_PG tests FIRST for OA1 notification transitions and retained custody digests.
// Depends on: actual C1/Executor, native signed recipient and complete READY artifact pair.
// Used by: SQL review CI; owner UPDATEs are rollback-only storage probes, never C3 execution.
// Invariants: authorization time and VOID stage agree; terminal notifications cannot regress;
// business audit digests retain exact attempt custody without retaining a hash of buyer PII.
package foundation_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func oaSQLGuardReject(t *testing.T, ctx context.Context, tx pgx.Tx, sql string, args ...any) {
	t.Helper()
	if _, err := tx.Exec(ctx, `SAVEPOINT oa_sql_guard_probe`); err != nil {
		t.Fatal(err)
	}
	_, err := tx.Exec(ctx, sql, args...)
	var p *pgconn.PgError
	if !errors.As(err, &p) || p.Code != "23514" {
		t.Fatalf("native notification guard must refuse with 23514: err=%v", err)
	}
	if _, err = tx.Exec(ctx, `ROLLBACK TO SAVEPOINT oa_sql_guard_probe`); err != nil {
		t.Fatal(err)
	}
}

func TestOrderAttemptSQLNotificationStageGuards(t *testing.T) {
	s, open, psid := oaOriginPinScenario(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := s.executor.Execute(ctx, "", "", s.a.AttemptID, s.a.Generation)
	if err != nil {
		t.Fatal("actual complete READY executor before storage probes", err)
	}
	durable := oaCrashCommitted(t, s, out)
	defer clear(durable)
	// Independent native HPKE open verifies the plaintext recipient and host.
	// The SQL fields alone are not treated as proof of ciphertext contents.
	oaOriginPinOpenCurrent(t, s, open, psid, s.e.origin)
	var notification string
	err = s.e.p.f.owner.QueryRow(ctx, `SELECT n.id::text FROM inbox.order_payment_notifications n
	 JOIN integration.operations x ON x.id=n.operation_id JOIN river.river_job j ON j.id=x.job_id
	 JOIN inbox.outbound_messages b ON b.id=n.outbound_id JOIN inbox.send_secrets z ON z.operation_id=x.id
	 WHERE n.create_attempt_id=$1 AND n.state='READY' AND n.void_stage='none'
	 AND n.dispatch_authorized_at IS NULL AND x.state='READY' AND j.kind='external_operation_v1'
	 AND b.operation_id=x.id AND octet_length(z.enc)=32 AND octet_length(z.sealed)>16`, s.a.AttemptID).Scan(&notification)
	if err != nil || notification == "" {
		t.Fatal("real operation-backed READY positive sentinel", err)
	}
	cases := []struct {
		name   string
		prefix []string
		bad    string
	}{
		{"ready_authorized", nil, `UPDATE inbox.order_payment_notifications SET dispatch_authorized_at=clock_timestamp() WHERE id=$1`},
		{"before_c3_authorized", nil, `UPDATE inbox.order_payment_notifications SET state='VOID',void_stage='before_c3',void_code='recipient_invalid',dispatch_authorized_at=clock_timestamp() WHERE id=$1`},
		{"after_c3_unauthorized", nil, `UPDATE inbox.order_payment_notifications SET state='VOID',void_stage='after_c3',void_code='recipient_invalid' WHERE id=$1`},
		{"ready_skips_dispatch", nil, `UPDATE inbox.order_payment_notifications SET state='ACCEPTED',dispatch_authorized_at=clock_timestamp() WHERE id=$1`},
		{"dispatch_returns_ready", []string{`UPDATE inbox.order_payment_notifications SET state='DISPATCHING',dispatch_authorized_at=clock_timestamp() WHERE id=$1`}, `UPDATE inbox.order_payment_notifications SET state='READY' WHERE id=$1`},
		{"unknown_claims_before_c3", []string{`UPDATE inbox.order_payment_notifications SET state='DISPATCHING',dispatch_authorized_at=clock_timestamp() WHERE id=$1`, `UPDATE inbox.order_payment_notifications SET state='DELIVERY_UNKNOWN' WHERE id=$1`}, `UPDATE inbox.order_payment_notifications SET state='VOID',void_stage='before_c3',void_code='recipient_invalid' WHERE id=$1`},
		{"accepted_claims_void", []string{`UPDATE inbox.order_payment_notifications SET state='DISPATCHING',dispatch_authorized_at=clock_timestamp() WHERE id=$1`, `UPDATE inbox.order_payment_notifications SET state='ACCEPTED' WHERE id=$1`}, `UPDATE inbox.order_payment_notifications SET state='VOID',void_stage='after_c3',void_code='recipient_invalid' WHERE id=$1`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tx, callErr := s.e.p.f.owner.Begin(ctx)
			if callErr != nil {
				t.Fatal(callErr)
			}
			defer func() {
				clean, stop := context.WithTimeout(context.Background(), 3*time.Second)
				defer stop()
				_ = tx.Rollback(clean)
			}()
			for _, prefix := range tc.prefix {
				if _, callErr = tx.Exec(ctx, prefix, notification); callErr != nil {
					t.Fatal("legal native storage transition before negative probe", callErr)
				}
			}
			oaSQLGuardReject(t, ctx, tx, tc.bad, notification)
		})
	}
	t.Run("ready_void_before_c3_positive", func(t *testing.T) {
		tx, callErr := s.e.p.f.owner.Begin(ctx)
		if callErr != nil {
			t.Fatal(callErr)
		}
		defer func() {
			clean, stop := context.WithTimeout(context.Background(), 3*time.Second)
			defer stop()
			_ = tx.Rollback(clean)
		}()
		if _, callErr = tx.Exec(ctx, `UPDATE inbox.order_payment_notifications SET state='VOID',void_stage='before_c3',void_code='recipient_invalid' WHERE id=$1`, notification); callErr != nil {
			t.Fatal("READY may become VOID before authorization", callErr)
		}
		var exact bool
		if callErr = tx.QueryRow(ctx, `SELECT state='VOID' AND void_stage='before_c3' AND dispatch_authorized_at IS NULL FROM inbox.order_payment_notifications WHERE id=$1`, notification).Scan(&exact); callErr != nil || !exact {
			t.Fatal("positive before-C3 VOID stage sentinel", callErr)
		}
	})
}

func TestOrderAttemptSQLNoRetainedSelectionDigest(t *testing.T) {
	var previousAttempt string
	var previousExpected [32]byte
	for i := 0; i < 2; i++ {
		t.Run(fmt.Sprintf("actual_attempt_%d", i+1), func(t *testing.T) {
			s, _, _ := oaOriginPinScenario(t)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			var canonical, custody []byte
			var tenant, store, attempt, owner, principal string
			var generation int64
			// Read authority from the actual C1 row and its authenticated scope,
			// independently of both retained digests and any product hash helper.
			err := s.e.p.f.owner.QueryRow(ctx, `SELECT a.request_hash,r.request_hash,
			 a.tenant_id::text,a.store_id::text,a.attempt_id::text,a.intent_generation,
			 a.buyer_owner_id::text,q.principal_id::text FROM ops.order_attempts a
			 JOIN ops.order_attempt_scopes q ON q.scope_id=a.scope_id AND q.tenant_id=a.tenant_id AND q.store_id=a.store_id
			 JOIN inbox.order_for_buyer r ON r.create_attempt_id=a.attempt_id
			 WHERE a.attempt_id=$1 AND a.scope_id=$2 AND a.intent_generation=$3 AND a.state='PENDING' AND a.admission_finished
			 AND q.principal_id=(a.admission_provenance->>'principal_id')::uuid
			 AND r.state='pending' AND r.order_id IS NULL AND r.request_id=a.attempt_id AND r.bundle_id=$4
			 AND r.tenant_id=a.tenant_id AND r.store_id=a.store_id AND r.buyer_id=a.buyer_owner_id
			 AND r.principal_id=q.principal_id`, s.a.AttemptID, s.a.ScopeID, s.a.Generation, s.bundle).
				Scan(&canonical, &custody, &tenant, &store, &attempt, &generation, &owner, &principal)
			if err != nil || len(canonical) != 32 || len(custody) != 32 {
				t.Fatal("real accepted claim admission custody before digest check", err)
			}
			if tenant != s.e.tenant() || store != s.e.store() || principal != s.e.p.f.principalA ||
				attempt != s.a.AttemptID || generation != s.a.Generation || owner == "" {
				t.Fatal("custody metadata must match authenticated C1 receipt and scope")
			}
			expected := sha256.Sum256([]byte(fmt.Sprintf("order-attempt-custody/v1|%s|%s|%s|%d|%s|%s",
				tenant, store, attempt, generation, owner, principal)))
			if previousAttempt != "" && (attempt == previousAttempt || expected == previousExpected) {
				t.Fatal("second actual admission must have a different identity and independently expected custody digest")
			}
			previousAttempt, previousExpected = attempt, expected
			if bytes.Equal(canonical, custody) {
				t.Fatal("claim reservation retains canonical request digest of buyer contact/address after ops retention")
			}
			if !bytes.Equal(custody, expected[:]) {
				t.Fatal("admission reservation must match independently calculated six-field custody digest")
			}
			out, err := s.executor.Execute(ctx, "", "", s.a.AttemptID, s.a.Generation)
			if err != nil {
				t.Fatal("actual C2 must consume the same non-PII admission custody", err)
			}
			durable := oaCrashCommitted(t, s, out)
			defer clear(durable)
			var commandDigest, placedDigest []byte
			err = s.e.p.f.owner.QueryRow(ctx, `SELECT c.request_hash,r.request_hash FROM checkout.command_results c
			 JOIN inbox.order_for_buyer r ON r.create_attempt_id=c.create_attempt_id
			 WHERE c.create_attempt_id=$1 AND c.operation='checkout.begin' AND c.idempotency_key=$1::uuid::text
			 AND c.creator_kind='order_attempt' AND r.state='placed' AND r.order_id=$1 AND r.bundle_id=$2
			 AND r.tenant_id=c.tenant_id AND r.store_id=c.store_id AND r.buyer_id=c.owner_id`, s.a.AttemptID, s.bundle).Scan(&commandDigest, &placedDigest)
			if err != nil || len(commandDigest) != 32 || !bytes.Equal(commandDigest, custody) || !bytes.Equal(placedDigest, custody) || bytes.Equal(commandDigest, canonical) {
				t.Fatal("native claim and Begin audit must retain one exact non-PII custody digest", err)
			}
			if !bytes.Equal(commandDigest, expected[:]) || !bytes.Equal(placedDigest, expected[:]) {
				t.Fatal("placed reservation and checkout command must match independently calculated six-field custody digest")
			}
		})
	}
}
