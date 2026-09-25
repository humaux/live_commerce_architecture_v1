package foundation_test

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/payments"
)

// These are actual startup queries against an isolated PG, not a boolean mock.
// Owner corruption models deployment drift; the ordinary worker never repairs it.
func TestBuyerPaymentWorkerAdmissionFences(t *testing.T) {
	f := pwIsolatedFixture(t)
	keys := pwKeys(t)
	q := pqSetupItemsOn(t, f, keys, false, 1)
	var calls atomic.Int32
	opts := payments.DefaultQueryWorkerOptions()
	opts.MockTransport = pqTransport(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, errors.New("startup_must_not_query")
	})
	assertReady := func(t *testing.T, want bool) {
		t.Helper()
		client, err := payments.NewWorkerClient(context.Background(), q.worker, keys, "PROVIDER_MOCK", 1, opts)
		if want {
			if err != nil || client == nil {
				t.Fatal("valid router and linked job rejected")
			}
		} else if client != nil || err == nil || err.Error() != "payment_worker_queue_unready" {
			t.Fatal("invalid queue state admitted or leaked error detail")
		}
		if calls.Load() != 0 {
			t.Fatal("constructor contacted a provider")
		}
	}
	assertReady(t, true)
	var triggerDDL string
	if err := f.owner.QueryRow(context.Background(), `SELECT pg_get_triggerdef(oid)
		FROM pg_trigger WHERE tgrelid='river.river_job'::regclass AND tgname='payment_queue_route_v1'`).Scan(&triggerDDL); err != nil {
		t.Fatal(err)
	}
	t.Run("disabled router", func(t *testing.T) {
		mustExec(t, f.owner, `ALTER TABLE river.river_job DISABLE TRIGGER payment_queue_route_v1`)
		defer mustExec(t, f.owner, `ALTER TABLE river.river_job ENABLE TRIGGER payment_queue_route_v1`)
		assertReady(t, false)
	})
	t.Run("missing router", func(t *testing.T) {
		mustExec(t, f.owner, `DROP TRIGGER payment_queue_route_v1 ON river.river_job`)
		defer mustExec(t, f.owner, triggerDDL)
		assertReady(t, false)
	})
	t.Run("immediate timing", func(t *testing.T) {
		mustExec(t, f.owner, `DROP TRIGGER payment_queue_route_v1 ON river.river_job`)
		defer func() {
			mustExec(t, f.owner, `DROP TRIGGER payment_queue_route_v1 ON river.river_job`)
			mustExec(t, f.owner, triggerDDL)
		}()
		mustExec(t, f.owner, `CREATE CONSTRAINT TRIGGER payment_queue_route_v1 AFTER INSERT ON river.river_job
		 DEFERRABLE INITIALLY IMMEDIATE FOR EACH ROW EXECUTE FUNCTION integration.route_payment_queue_v1()`)
		assertReady(t, false)
	})
	for _, tc := range []struct{ name, change, restore string }{
		{"legacy queue drift", `UPDATE river.river_job SET queue='default' WHERE id=$1`, `UPDATE river.river_job SET queue='payment_mock_v1' WHERE id=$1`},
		{"wrong profile queue", `UPDATE river.river_job SET queue='payment_live_v1' WHERE id=$1`, `UPDATE river.river_job SET queue='payment_mock_v1' WHERE id=$1`},
		{"custom queue", `UPDATE river.river_job SET queue='unclaimed_custom' WHERE id=$1`, `UPDATE river.river_job SET queue='payment_mock_v1' WHERE id=$1`},
		{"foreign kind", `UPDATE river.river_job SET kind='not_a_payment' WHERE id=$1`, `UPDATE river.river_job SET kind='payment_query_v1' WHERE id=$1`},
		{"unlinked args", `UPDATE river.river_job SET args=jsonb_set(args,'{version}','2') WHERE id=$1`, `UPDATE river.river_job SET args=jsonb_set(args,'{version}','1') WHERE id=$1`},
		{"unique key drift", `UPDATE river.river_job SET unique_key=decode(repeat('ab',32),'hex') WHERE id=$1`, `UPDATE river.river_job SET unique_key=NULL WHERE id=$1`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mustExec(t, f.owner, tc.change, q.result.JobID)
			defer mustExec(t, f.owner, tc.restore, q.result.JobID)
			before := pwJobExceptQueue(t, f.owner, q.result.JobID)
			assertReady(t, false)
			if after := pwJobExceptQueue(t, f.owner, q.result.JobID); after != before {
				t.Fatal("startup audit mutated the invalid job")
			}
		})
	}
	assertReady(t, true)
	for _, role := range []string{"commerce_runtime", "commerce_checkout_runtime", "commerce_hosted_runtime", "commerce_buyer_runtime", "commerce_buyer_issuer", "commerce_identity"} {
		t.Run("reject "+role, func(t *testing.T) {
			pool, err := pgxpool.New(context.Background(), bcRole(t, f, role))
			if err != nil {
				t.Fatal("open isolated wrong-role pool")
			}
			defer pool.Close()
			client, err := payments.NewWorkerClient(context.Background(), pool, keys, "PROVIDER_MOCK", 1, opts)
			if client != nil || err == nil || err.Error() != "payment_worker_database" {
				t.Fatal("non-worker authority admitted")
			}
		})
	}
	client, err := payments.NewWorkerClient(context.Background(), f.owner, keys, "PROVIDER_MOCK", 1, opts)
	if client != nil || err == nil || err.Error() != "payment_worker_database" {
		t.Fatal("migration owner admitted")
	}
	if calls.Load() != 0 || q.reportCount(t) != 0 {
		t.Fatal("admission probes produced provider effects")
	}
	q.pending(t)
}
