// Purpose: gates for the PAY-RM2 phase-2b follow-ups (issue #93; review output/pay-rm2-p2b/review-1d7af85d0.md):
//
//	PFU01: 0199's stricter guard refuses (22023, nothing changed) while a PAYUNi reservation is still PAYMENT_PENDING
//	       (P2-2) and while a stored PAYUNi observation was never applied (P2-3); once an operator releases the stock
//	       and records a review case it proceeds and every row is still there.
//	PFU02: post_river/0027 re-runs that guard inside the post-River transaction (P2-1).
//	PFU03: on an upgrade from just before 0197 the numbered phase alone (post-River held back) leaves no grantee able
//	       to execute checkout.start_payment / integration.record_payment_query (P2-1 window), and revokes the leftover
//	       hosted-form writer authority (P2-5); the pre-state proves each privilege existed, so the gate is not vacuous.
//
// Depends on: migrations.Apply; pwIsolatedFixture; psSetupItemsOn; rm2HistoricalAttempt; pcFull/pqJSON; rmVersions/
//
//	rmChecksums/rmSkipLedger (payuni_removal_test.go); p2bPreState (payuni_phase2b_test.go); pfeReplica
//	(payuni_form_erase_test.go).
//
// Used by: CI full foundation suite; scripts/dev/test-focused.sh '^TestPayuniPhase2bFollowup'.
// Invariants: synthetic data only; the migrations under test never delete or rewrite a row.
package foundation_test

import (
	"context"
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
	// in 0197 whose rollback P2B02 pins). What a refusal must leave unchanged here is the business history.
	before := rm2Snapshot(t, p, result.AttemptID)
	pfuRefused(t, f.owner, pfuNumbered, pfuApplyOnly(t, f.owner, pfuNumbered),
		"0 non-terminal PAYUNi operation(s), 0 PAYUNi order(s) awaiting payment, 1 PAYUNi reservation(s) still PAYMENT_PENDING, 0 stored PAYUNi observation(s) never applied")
	if after := rm2Snapshot(t, p, result.AttemptID); after != before {
		t.Errorf("refused 0199 changed PAYUNi history:\nbefore=%s\nafter=%s", before, after)
	}

	// Stock released; a stored PAYUNi QUERY report that no fact or review case cites.
	pfeReplica(t, f.owner, `UPDATE inventory.reservations SET state='RELEASED' WHERE id='`+p.hold.OrderID+`'`)
	report := pcFull(pqFixture{psHarness: p, result: result})
	mustExec(t, f.owner, `INSERT INTO payments.provider_observations(tenant_id,store_id,attempt_id,source,execution_profile,environment,first_generation,report,report_hash)
		SELECT a.tenant_id,a.store_id,a.id,'QUERY',a.execution_profile,a.environment,a.generation,$2::jsonb,sha256(convert_to($2::jsonb::text,'UTF8'))
		FROM checkout.payment_attempts a WHERE a.id=$1`, result.AttemptID, pqJSON(report))
	pfuRefused(t, f.owner, pfuNumbered, pfuApplyOnly(t, f.owner, pfuNumbered),
		"0 PAYUNi reservation(s) still PAYMENT_PENDING, 1 stored PAYUNi observation(s) never applied")

	// Operator records a review case for it: the follow-up proceeds and history stays.
	mustExec(t, f.owner, `INSERT INTO payments.review_cases(tenant_id,store_id,attempt_id,reason,source_report_hash)
		SELECT tenant_id,store_id,attempt_id,'CONFLICTING_REPORT',report_hash FROM payments.provider_observations WHERE attempt_id=$1`, result.AttemptID)
	if err := pfuApplyOnly(t, f.owner, pfuNumbered); err != nil {
		t.Fatalf("0199 over reconciled PAYUNi history: %v", err)
	}
	if n := countRows(t, f.owner, `SELECT (SELECT count(*) FROM payments.provider_observations WHERE attempt_id=$1)+(SELECT count(*) FROM payments.review_cases WHERE attempt_id=$1)+(SELECT count(*) FROM checkout.payment_attempts WHERE id=$1)`, result.AttemptID); n != 3 {
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
