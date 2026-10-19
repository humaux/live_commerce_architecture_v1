package foundation_test

// Purpose: PAYUNi hosted-handoff authority tests, now refusal gates for the dropped hosted writer.
// Depends on: p2bHostedRetired (payment_sql_contract_helpers_test.go).
// Used by: go test ./tests/foundation (CI full suite).
// Invariants: the dropped writer is refused and changes no row; Stripe equivalents are named in the map. PAY-RM2 phase 2b (0197, post_river/0026): see output/pay-rm2-p2b/test-contract-map.md.

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func hpDigest(t *testing.T, configURL, notifyURL string) []byte {
	t.Helper()
	raw, err := json.Marshal(struct {
		Version   string `json:"version"`
		ReturnURL string `json:"return_url"`
		NotifyURL string `json:"notify_url"`
	}{"payuni-hosted-v1", configURL, notifyURL})
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(raw)
	return hash[:]
}

func hpSQLState(err error, want string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == want
}

func TestBuyerPaymentHostedSQLAuthorityAndScope(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bHostedRetired(t)
}

func TestBuyerPaymentHostedForgedOuterFormRejected(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bHostedRetired(t)
}

func TestBuyerPaymentHostedStorageFailureRollsBack(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bHostedRetired(t)
}

func TestBuyerPaymentHostedDriftAndFinancialReviewDenyFirstTake(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bHostedRetired(t)
}

type checkoutResult struct {
	ID     string
	Amount int64
}

func hpSeedReviewFact(t *testing.T, h hpHarness, result checkoutResult, review bool) {
	t.Helper()
	report := []byte(`{"synthetic":"hosted-gate"}`)
	mustExec(t, h.f.owner, `INSERT INTO payments.provider_observations(tenant_id,store_id,attempt_id,source,execution_profile,environment,first_generation,report,report_hash)
	 SELECT tenant_id,store_id,id,'QUERY',execution_profile,environment,generation,$2::jsonb,sha256(convert_to($2::jsonb::text,'UTF8'))
	 FROM checkout.payment_attempts WHERE id=$1`, result.ID, report)
	if review {
		mustExec(t, h.f.owner, `INSERT INTO payments.review_cases(tenant_id,store_id,attempt_id,reason,source_report_hash)
		 SELECT tenant_id,store_id,id,'CONFLICTING_REPORT',sha256(convert_to($2::jsonb::text,'UTF8'))
		 FROM checkout.payment_attempts WHERE id=$1`, result.ID, report)
	} else {
		mustExec(t, h.f.owner, `INSERT INTO payments.facts(tenant_id,store_id,attempt_id,kind,amount_minor,currency,provider_reference,connection_id,execution_profile,environment,source_report_hash)
		 SELECT tenant_id,store_id,id,'AUTHORIZED',$3,currency,'hosted_fixture_reference',connection_id,execution_profile,environment,sha256(convert_to($2::jsonb::text,'UTF8'))
		 FROM checkout.payment_attempts WHERE id=$1`, result.ID, report, result.Amount)
	}
}

func TestBuyerPaymentHostedReplayAfterDisableAndRevokedCapability(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bHostedRetired(t)
}

func TestBuyerPaymentHostedQualificationExpiresDuringTakeWait(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bHostedRetired(t)
}

func TestBuyerPaymentHostedWriterCannotMutateFormOrClearHandoff(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bHostedRetired(t)
}

func hpDelayPageWrite(t *testing.T, h hpHarness, event string) string {
	t.Helper()
	name := "hp_delay_" + t04Tag()
	fn := pgx.Identifier{name}.Sanitize()
	trigger := pgx.Identifier{name + "_trigger"}.Sanitize()
	sequenceName := name + "_seq"
	sequence := pgx.Identifier{sequenceName}.Sanitize()
	mustExec(t, h.f.owner, fmt.Sprintf(`CREATE SEQUENCE public.%s`, sequence))
	mustExec(t, h.f.owner, fmt.Sprintf(`CREATE FUNCTION public.%s() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$ BEGIN PERFORM nextval('public.%s'::regclass); PERFORM pg_sleep(2.5); RETURN NEW; END $$`, fn, sequenceName))
	mustExec(t, h.f.owner, fmt.Sprintf(`CREATE TRIGGER %s BEFORE %s ON checkout.hosted_payment_pages FOR EACH ROW EXECUTE FUNCTION public.%s()`, trigger, event, fn))
	t.Cleanup(func() {
		mustExec(t, h.f.owner, fmt.Sprintf(`DROP TRIGGER IF EXISTS %s ON checkout.hosted_payment_pages`, trigger))
		mustExec(t, h.f.owner, fmt.Sprintf(`DROP FUNCTION IF EXISTS public.%s()`, fn))
		mustExec(t, h.f.owner, fmt.Sprintf(`DROP SEQUENCE IF EXISTS public.%s`, sequence))
	})
	return sequenceName
}

func hpAssertDelayedWriteEntered(t *testing.T, h hpHarness, sequenceName string) {
	t.Helper()
	var entered bool
	if err := h.f.owner.QueryRow(context.Background(), `SELECT is_called FROM public.`+pgx.Identifier{sequenceName}.Sanitize()).Scan(&entered); err != nil || !entered {
		t.Fatalf("HP05 delayed write trigger was never entered: %v", err)
	}
}

func hpAwaitOrderLock(t *testing.T, h hpHarness, blockerPID uint32) {
	t.Helper()
	ctx := context.Background()
	login := h.pool.Config().ConnConfig.User
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var waiting bool
		err := h.f.owner.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity
		 WHERE datname=current_database() AND usename=$1 AND state='active'
		 AND query LIKE '%checkout.take_hosted_page%' AND wait_event_type='Lock'
		 AND $2::int=ANY(pg_blocking_pids(pid)))`, login, int32(blockerPID)).Scan(&waiting)
		if err != nil {
			t.Fatal(err)
		}
		if waiting {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("HP04/HP05 take never reached the held order lock")
}

func TestBuyerPaymentHostedQualificationExpiresDuringFinalWrites(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bHostedRetired(t)
}

func TestBuyerPaymentHostedCapabilityRevokedDuringTakeWait(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bHostedRetired(t)
}

func TestBuyerPaymentHostedLocalDeadlineFence(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bHostedRetired(t)
}
