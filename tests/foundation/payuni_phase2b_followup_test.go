// Purpose: gates for the PAY-RM2 phase-2b follow-ups (issue #93; review output/pay-rm2-p2b/review-1d7af85d0.md):
//
//	PFU01: 0199's stricter guard refuses (22023, nothing changed) while a PAYUNi reservation is still PAYMENT_PENDING
//	       (P2-2) and while a stored PAYUNi observation was never applied (P2-3); a failed-payment report 0018 applied
//	       as a no-op does not block it, and every row is still there.
//	PFU02: post_river/0027 re-runs that guard inside the post-River transaction (P2-1).
//	PFU03: on an upgrade from just before 0197 the numbered phase alone (post-River held back) leaves no grantee able
//	       to execute checkout.start_payment / integration.record_payment_query (P2-1 window), and revokes the leftover
//	       hosted-form writer authority (P2-5); the pre-state proves each privilege existed, so the gate is not vacuous.
//	PFU04 (review-105-336b06942 P1): the full real chain over a legacy PAYUNi history applies when that history is
//	       reconciled although 0018 cited its benign reports in nothing, and refuses (22023, revokes rolled back) when
//	       a paid report was never applied.
//
// Depends on: migrations.Apply; pwIsolatedFixture; psSetupItemsOn; rm2HistoricalAttempt; pqReport/pqJSON; rmVersions/
//
//	rmChecksums/rmSkipLedger (payuni_removal_test.go); p2bPreState (payuni_phase2b_test.go); pfeReplica
//	(payuni_form_erase_test.go); lriPre0032Fixture/lriApplyFull/lriLedger/lriCloseProducerPools
//	(legacy_runtime_upgrade_test.go); pwOldQuerySetupOn/pwKeys (payment_runtime_test.go); pqReport.
//
// Used by: CI full foundation suite; scripts/dev/test-focused.sh '^TestPayuniPhase2bFollowup'.
// Invariants: synthetic data only; the migrations under test never delete or rewrite a row.
package foundation_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/migrations"
)

const (
	pfuNumbered = "0199_payuni_phase2b_followups.sql"
	pfuPost     = "post_river/0027_payuni_phase2b_guard_recheck.sql"
	pfuStart    = "checkout.start_payment(bytea,uuid,text,bytea,uuid,text,bigint,text,uuid,bigint)"
	pfuRecord   = "integration.record_payment_query(uuid,bigint,bytea,text,jsonb,bigint)"
)

func pfuRequire(t *testing.T) {
	t.Helper()
	sums := rmChecksums(t, rmVersions("0199", "0027"))
	for _, v := range []string{pfuNumbered, pfuPost} {
		if _, ok := sums[v]; !ok {
			t.Fatalf("migrations/%s must exist (got %v)", v, sums)
		}
	}
}

// pfuApplyOnly releases exactly one ledger row and re-applies the chain.
func pfuApplyOnly(t *testing.T, pool *pgxpool.Pool, version string) error {
	t.Helper()
	mustExec(t, pool, `DELETE FROM public.lc_schema_migrations WHERE version=$1`, version)
	return migrations.Apply(context.Background(), pool)
}

func pfuRefused(t *testing.T, pool *pgxpool.Pool, version string, err error, fragments ...string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), "22023") {
		t.Fatalf("%s must refuse with 22023: %v", version, err)
	}
	for _, f := range fragments {
		if !strings.Contains(err.Error(), f) {
			t.Fatalf("%s refusal %q lacks %q", version, err, f)
		}
	}
	if n := countRows(t, pool, `SELECT count(*) FROM public.lc_schema_migrations WHERE version=$1`, version); n != 0 {
		t.Fatalf("refused %s recorded %d ledger row(s)", version, n)
	}
}

// TestPayuniPhase2bFollowupGuardStockAndUnappliedReport (PFU01).
func TestPayuniPhase2bFollowupGuardStockAndUnappliedReport(t *testing.T) {
	pfuRequire(t)
	f := pwIsolatedFixture(t)
	p := psSetupItemsOn(t, f, 1)
	result := rm2HistoricalAttempt(t, p)
	// Operator "reconciled" only the operation and the order: 0197's clauses pass, the stock is still held.
	pfeReplica(t, f.owner,
		`UPDATE integration.operations SET state='FAILED_FINAL' WHERE id='`+result.OperationID+`'`,
		`UPDATE checkout.orders SET commercial_state='CANCELLED' WHERE id='`+p.hold.OrderID+`'`)
	if n := countRows(t, f.owner, `SELECT count(*) FROM inventory.reservations WHERE id=$1 AND state='PAYMENT_PENDING'`, p.hold.OrderID); n != 1 {
		t.Fatalf("fixture: reservation PAYMENT_PENDING rows=%d", n)
	}
	// This isolated DB was built with 0199 already applied (its DDL is IF EXISTS / REVOKE, idempotent), so DDL
	// rollback cannot be observed here (the guard is the file's first statement in the one numbered transaction, as
	// in 0197 whose rollback P2B02 pins). What a refusal must leave unchanged here is the business history; the DDL rollback is pinned by PFU04 on the legacy chain.
	before := rm2Snapshot(t, p, result.AttemptID)
	pfuRefused(t, f.owner, pfuNumbered, pfuApplyOnly(t, f.owner, pfuNumbered),
		"0 non-terminal PAYUNi operation(s), 0 PAYUNi order(s) awaiting payment, 1 PAYUNi reservation(s) still PAYMENT_PENDING, 0 stored PAYUNi observation(s) never applied")
	if after := rm2Snapshot(t, p, result.AttemptID); after != before {
		t.Errorf("refused 0199 changed PAYUNi history:\nbefore=%s\nafter=%s", before, after)
	}

	// Stock released. The operation carries the TradeNo record_payment_query stored on it. A failed-payment report
	// (TradeStatus 2) is one 0018 applied without writing anything: it is not unapplied and 0199 proceeds.
	q := pqFixture{psHarness: p, result: result}
	pfeReplica(t, f.owner, `UPDATE inventory.reservations SET state='RELEASED' WHERE id='`+p.hold.OrderID+`'`,
		`UPDATE integration.operations SET provider_reference='`+pqReport(q)["TradeNo"].(string)+`' WHERE id='`+result.OperationID+`'`)
	pfuReport(t, q, func(r map[string]any) { r["TradeStatus"] = "2" })
	if err := pfuApplyOnly(t, f.owner, pfuNumbered); err != nil {
		t.Fatalf("0199 over a reconciled PAYUNi history with a failed-payment report: %v", err)
	}
	// A paid capture report that was never applied (no AUTHORIZED/CAPTURED fact). Replaces the former
	// "operator records a review case, 0199 proceeds" step (review-105-336b06942 P1: hand-written review cases are
	// not reconciliation; the runbook now escalates this count to the owner).
	pfuReport(t, q, func(r map[string]any) { r["CloseStatus"], r["CloseAmountTWD"] = "2", result.AmountMinor/100 })
	pfuRefused(t, f.owner, pfuNumbered, pfuApplyOnly(t, f.owner, pfuNumbered),
		"0 PAYUNi reservation(s) still PAYMENT_PENDING, 1 stored PAYUNi observation(s) never applied")
	if n := countRows(t, f.owner, `SELECT (SELECT count(*) FROM payments.provider_observations WHERE attempt_id=$1)+(SELECT count(*) FROM payments.facts WHERE attempt_id=$1)+(SELECT count(*) FROM payments.review_cases WHERE attempt_id=$1)+(SELECT count(*) FROM checkout.payment_attempts WHERE id=$1)`, result.AttemptID); n != 3 {
		t.Errorf("0199 changed PAYUNi history rows: %d", n)
	}
}

// TestPayuniPhase2bFollowupPostRiverRecheck (PFU02).
func TestPayuniPhase2bFollowupPostRiverRecheck(t *testing.T) {
	pfuRequire(t)
	f := pwIsolatedFixture(t)
	p := psSetupItemsOn(t, f, 1)
	rm2HistoricalAttempt(t, p) // operation non-terminal, order AWAITING_PAYMENT, stock PAYMENT_PENDING
	pfuRefused(t, f.owner, pfuPost, pfuApplyOnly(t, f.owner, pfuPost),
		"post-River re-check refused", "1 PAYUNi order(s) awaiting payment", "1 PAYUNi reservation(s) still PAYMENT_PENDING")
}

func pfuGrantees(t *testing.T, pool *pgxpool.Pool, fn string) string {
	t.Helper()
	return pfeRow(t, pool, `SELECT coalesce(string_agg(CASE WHEN a.grantee=0 THEN 'PUBLIC' ELSE a.grantee::regrole::text END,',' ORDER BY 1),'')
		FROM pg_proc p, aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a
		WHERE p.oid=to_regprocedure($1) AND a.grantee<>p.proowner AND a.privilege_type='EXECUTE'`, fn)
}

type pfuHosted struct {
	insert, update, read, cipher bool
	policies                     string
}

func pfuHostedAuthority(t *testing.T, pool *pgxpool.Pool) pfuHosted {
	t.Helper()
	var h pfuHosted
	if err := pool.QueryRow(context.Background(), `SELECT has_table_privilege('commerce_checkout_writer','checkout.hosted_payment_pages','INSERT'),
		has_column_privilege('commerce_checkout_writer','checkout.hosted_payment_pages','handed_out_at','UPDATE'),
		has_table_privilege('commerce_checkout_writer','checkout.hosted_payment_pages','SELECT'),
		has_column_privilege('commerce_checkout_writer','integration.account_credentials','ciphertext','SELECT'),
		(SELECT coalesce(string_agg(policyname,',' ORDER BY policyname),'') FROM pg_policies
		  WHERE (schemaname,tablename) IN (('checkout','hosted_payment_pages'),('integration','account_credentials'))
		  AND policyname IN ('hosted_read','hosted_insert','hosted_take','hosted_credential_read'))`).
		Scan(&h.insert, &h.update, &h.read, &h.cipher, &h.policies); err != nil {
		t.Fatal(err)
	}
	return h
}

// TestPayuniPhase2bFollowupClosesWindowAndRevokesHostedWriter (PFU03).
func TestPayuniPhase2bFollowupClosesWindowAndRevokesHostedWriter(t *testing.T) {
	pfuRequire(t)
	db, versions := p2bPreState(t)
	if g := pfuGrantees(t, db, pfuStart); !strings.Contains(g, "commerce_checkout_runtime") {
		t.Fatalf("pre-0197 baseline: start_payment grantees %q lack commerce_checkout_runtime — gate would be vacuous", g)
	}
	if g := pfuGrantees(t, db, pfuRecord); g == "" {
		t.Fatal("pre-0197 baseline: record_payment_query has no grantee — gate would be vacuous")
	}
	want := pfuHosted{insert: true, update: true, read: true, cipher: true, policies: "hosted_credential_read,hosted_insert,hosted_read,hosted_take"}
	if got := pfuHostedAuthority(t, db); got != want {
		t.Fatalf("pre-0197 baseline hosted authority %+v, want %+v", got, want)
	}
	// Release only the numbered files: post-River (0026 drop, 0027) stays recorded, i.e. the window state.
	var numbered []string
	for _, v := range versions {
		if !strings.HasPrefix(v, "post_river/") {
			numbered = append(numbered, v)
		}
	}
	mustExec(t, db, `DELETE FROM public.lc_schema_migrations WHERE version=ANY($1)`, numbered)
	if err := migrations.Apply(context.Background(), db); err != nil {
		t.Fatalf("numbered phase 2b + follow-ups: %v", err)
	}
	for _, fn := range []string{pfuStart, pfuRecord} {
		if rmReg(t, db, "to_regprocedure", fn) == "" {
			t.Fatalf("%s gone although post-River was held back — the window fixture is wrong", fn)
		}
		if g := pfuGrantees(t, db, fn); g != "" {
			t.Errorf("window: %s still executable by %q", fn, g)
		}
	}
	want = pfuHosted{read: true, policies: "hosted_read"}
	if got := pfuHostedAuthority(t, db); got != want {
		t.Errorf("hosted writer authority after 0199 %+v, want %+v", got, want)
	}
	fresh := fixture(t)
	if got := pfuHostedAuthority(t, fresh.owner); got != want {
		t.Errorf("fresh install hosted writer authority %+v, want %+v", got, want)
	}
}

// pfuInject (LC_PFU_INJECT=1) is the calibration fault for PFU04's rollback assertion: after the refused chain it
// commits 0199's revoke sections out of band, i.e. the state a runner that applied them outside the refused
// transaction would leave. The rollback assertion must then fail.
func pfuInject() bool { return os.Getenv("LC_PFU_INJECT") != "" }

// pfuLegacy is the P2B04 history: the legacy pre-0032 schema plus one PAYUNi attempt written by the historical
// producer. The operation carries the TradeNo that integration.record_payment_query stored on it (complete_operation).
func pfuLegacy(t *testing.T) (*testFixture, pqFixture) {
	t.Helper()
	f := lriPre0032Fixture(t)
	q := pwOldQuerySetupOn(t, f, pwKeys(t), "river")
	lriCloseProducerPools(q.psHarness)
	pfeReplica(t, f.owner, `UPDATE integration.operations SET provider_reference='`+pqReport(q)["TradeNo"].(string)+`' WHERE id='`+q.result.OperationID+`'`)
	return f, q
}

// pfuReport stores one QUERY observation the way the dropped record_payment_query did and returns its hash.
func pfuReport(t *testing.T, q pqFixture, edit func(map[string]any)) []byte {
	t.Helper()
	r := pqReport(q)
	edit(r)
	var hash []byte
	if err := q.f.owner.QueryRow(context.Background(), `INSERT INTO payments.provider_observations(tenant_id,store_id,attempt_id,source,execution_profile,environment,first_generation,report,report_hash)
		SELECT a.tenant_id,a.store_id,a.id,'QUERY',a.execution_profile,a.environment,a.generation,$2::jsonb,sha256(convert_to($2::jsonb::text,'UTF8'))
		FROM checkout.payment_attempts a WHERE a.id=$1 RETURNING report_hash`, q.result.AttemptID, pqJSON(r)).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	return hash
}

// pfuReconcile is the operator's cancellation: operation terminal, order CANCELLED, stock RELEASED.
func pfuReconcile(t *testing.T, q pqFixture) {
	t.Helper()
	pfeReplica(t, q.f.owner,
		`UPDATE integration.operations SET state='FAILED_FINAL' WHERE id='`+q.result.OperationID+`'`,
		`UPDATE checkout.orders SET commercial_state='CANCELLED' WHERE id='`+q.hold.OrderID+`'`,
		`UPDATE inventory.reservations SET state='RELEASED' WHERE id='`+q.hold.OrderID+`'`)
}

// TestPayuniPhase2bFollowupLegacyChainReconciledAndUnapplied (PFU04, review-105-336b06942 P1): the full real chain
// (lriApplyFull, no pre-record) over a legacy PAYUNi history.
//   - reconciled: five QUERY reports, each applied by the real 0018 body (payments.apply_capture on this schema);
//     0018 cites only the authorization in a fact; the pending/empty-TradeNo, DataSource B, duplicate-auth and
//     cancelled (TradeStatus 3) reports are cited by nothing. The operator cancelled the order. 0197-0199 and
//     post_river 0026/0027 must all apply.
//   - unapplied: the authorization report is stored but was never applied. 0199 must refuse (22023) and the refusal
//     must roll back the whole numbered batch, including 0199's own revokes: start_payment / record_payment_query
//     grantees and the hosted-form writer authority are exactly as before (calibrated by pfuInject).
func TestPayuniPhase2bFollowupLegacyChainReconciledAndUnapplied(t *testing.T) {
	pfuRequire(t)
	ctx := context.Background()
	t.Run("reconciled", func(t *testing.T) {
		f, q := pfuLegacy(t)
		hashes := [][]byte{
			pfuReport(t, q, func(r map[string]any) { r["TradeStatus"], r["TradeNo"] = "0", "" }),
			pfuReport(t, q, func(r map[string]any) { r["DataSource"] = "B" }),
			pfuReport(t, q, func(map[string]any) {}),
			pfuReport(t, q, func(r map[string]any) { r["CloseStatus"] = "1" }),
			pfuReport(t, q, func(r map[string]any) { r["TradeStatus"] = "3" }),
		}
		for i, h := range hashes {
			if _, err := f.owner.Exec(ctx, `SELECT payments.apply_capture($1::uuid,$2::bytea)`, q.result.AttemptID, h); err != nil {
				t.Fatalf("legacy 0018 apply of report %d: %v", i, err)
			}
		}
		if got := pfeRow(t, f.owner, `SELECT coalesce((SELECT string_agg(kind,',' ORDER BY kind) FROM payments.facts WHERE attempt_id=$1),'')||'/'||
			(SELECT count(*) FROM payments.review_cases WHERE attempt_id=$1)||'/'||
			(SELECT count(*) FROM payments.provider_observations x WHERE x.attempt_id=$1
			  AND NOT EXISTS(SELECT 1 FROM payments.facts y WHERE y.attempt_id=x.attempt_id AND y.source_report_hash=x.report_hash))`, q.result.AttemptID); got != "AUTHORIZED/0/4" {
			t.Fatalf("legacy 0018 history facts/reviews/uncited = %s, want AUTHORIZED/0/4 (four reports 0018 applied without citing them)", got)
		}
		pfuReconcile(t, q)
		if err := lriApplyFull(ctx, f.owner); err != nil {
			t.Fatalf("full chain over reconciled PAYUNi history must apply: %v", err)
		}
		for _, v := range []string{"0197_payuni_phase2b_drop.sql", pfuNumbered, "post_river/0026_payuni_phase2b_drop.sql", pfuPost} {
			if n := countRows(t, f.owner, `SELECT count(*) FROM public.lc_schema_migrations WHERE version=$1`, v); n != 1 {
				t.Errorf("reconciled chain: ledger has %d row(s) for %s", n, v)
			}
		}
		if n := countRows(t, f.owner, `SELECT count(*) FROM payments.provider_observations WHERE attempt_id=$1`, q.result.AttemptID); n != 5 {
			t.Errorf("reconciled chain changed PAYUNi observations: %d", n)
		}
	})
	t.Run("unapplied", func(t *testing.T) {
		f, q := pfuLegacy(t)
		pfuReport(t, q, func(r map[string]any) { r["TradeStatus"], r["TradeNo"] = "0", "" })
		pfuReport(t, q, func(map[string]any) {}) // paid authorization whose reconcile job never ran
		pfuReconcile(t, q)
		start, record, hosted := pfuGrantees(t, f.owner, pfuStart), pfuGrantees(t, f.owner, pfuRecord), pfuHostedAuthority(t, f.owner)
		if start == "" || record == "" || !hosted.insert || !hosted.update || !hosted.cipher {
			t.Fatalf("legacy baseline lacks the privileges 0199 revokes (start %q, record %q, hosted %+v); the rollback gate would be vacuous", start, record, hosted)
		}
		ledger := lriLedger(t, f, "")
		err := lriApplyFull(ctx, f.owner)
		if pfuInject() {
			body, rerr := os.ReadFile(filepath.Join("../../migrations", pfuNumbered))
			if rerr != nil {
				t.Fatal(rerr)
			}
			mustExec(t, f.owner, string(body)[strings.Index(string(body), "-- 2) "):])
		}
		pfuRefused(t, f.owner, pfuNumbered, err, "PAYUNi phase-2b follow-up refused",
			"0 PAYUNi reservation(s) still PAYMENT_PENDING, 1 stored PAYUNi observation(s) never applied")
		if got := lriLedger(t, f, ""); got != ledger {
			t.Error("refused chain changed the migration ledger")
		}
		if got := pfuGrantees(t, f.owner, pfuStart); got != start {
			t.Errorf("refused 0199 left start_payment grantees %q, want %q", got, start)
		}
		if got := pfuGrantees(t, f.owner, pfuRecord); got != record {
			t.Errorf("refused 0199 left record_payment_query grantees %q, want %q", got, record)
		}
		if got := pfuHostedAuthority(t, f.owner); got != hosted {
			t.Errorf("refused 0199 left hosted authority %+v, want %+v", got, hosted)
		}
	})
}
