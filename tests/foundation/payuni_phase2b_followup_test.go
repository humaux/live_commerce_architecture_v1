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
//	       a paid report was never applied; also a captured history (applies) and a failed report applied before a
//	       later capture (fail-safe refusal, runbook §4) (review-105-087c67855 P2).
//	PFU05 (issue #106): 0199's revoke quotes each grantee once: extra EXECUTE grants to NOLOGIN roles named lowercase,
//	       with uppercase and a space, and with an embedded double quote, plus PUBLIC, are all revoked and the numbered
//	       phase applies; 0199 re-applied with no extra grant, and with both functions gone, also applies.
//	PFU06 (issue #116): one seed per guard branch PFU04 left unseeded (post-capture CloseStatus 3, CardRemainAmountTWD
//	       below and above the amount, CAPTURE_EVIDENCE_INCOMPLETE): the report stored but not applied refuses the
//	       chain; once the real 0018 body applied it, the chain applies.
//	PFU07 (issue #116, review-123-aee4382f9 P2-1): the owner's read-only listing in docs/runbooks/deploy.md §4 is the
//	       guard predicate verbatim (0199 and post_river/0027), and on every PFU04/PFU06/PFU07 seed it lists exactly the
//	       reports the guard counts (including an authorization with CloseStatus 3 that a fact cites, applied before a
//	       later capture). Its projection and its lc_psql heredoc + READ ONLY / SET LOCAL / ROLLBACK wrapper are pinned
//	       too (review-124-cb721a802 P2-1/P2-2, review-125-c20176c95 P2-1).
//
// Depends on: migrations.Apply; pwIsolatedFixture; psSetupItemsOn; rm2HistoricalAttempt; pqReport/pqJSON; rmVersions/
//
//	rmChecksums/rmSkipLedger (payuni_removal_test.go); p2bPreState (payuni_phase2b_test.go); pfeReplica
//	(payuni_form_erase_test.go); lriPre0032Fixture/lriApplyFull/lriLedger/lriCloseProducerPools
//	(legacy_runtime_upgrade_test.go); pwOldQuerySetupOn/pwKeys (payment_runtime_test.go); pqReport;
//	the owner listing between the runbook:payuni-unapplied markers in docs/runbooks/deploy.md §4 (PFU04/06/07).
//
// Used by: CI full foundation suite; scripts/dev/test-focused.sh '^TestPayuniPhase2bFollowup'.
// Invariants: synthetic data only; the migrations under test never delete or rewrite a row.
package foundation_test

import (
	"context"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
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

// pfuApply stores one report per edit and applies each, in order, with the real legacy 0018 body.
func pfuApply(t *testing.T, q pqFixture, edits ...func(map[string]any)) {
	t.Helper()
	for _, edit := range edits {
		pfuApplyHash(t, q, pfuReport(t, q, edit))
	}
}

// pfuApplyHash applies one stored report with the real legacy 0018 body.
func pfuApplyHash(t *testing.T, q pqFixture, hash []byte) {
	t.Helper()
	if _, err := q.f.owner.Exec(context.Background(), `SELECT payments.apply_capture($1::uuid,$2::bytea)`, q.result.AttemptID, hash); err != nil {
		t.Fatalf("legacy 0018 apply of report %x: %v", hash, err)
	}
}

// pfuApplied is "fact kinds/review reasons/observations no fact cites" for the attempt, as 0018 left them.
func pfuApplied(t *testing.T, q pqFixture) string {
	t.Helper()
	return pfeRow(t, q.f.owner, `SELECT coalesce((SELECT string_agg(kind,',' ORDER BY kind) FROM payments.facts WHERE attempt_id=$1),'')||'/'||
		coalesce((SELECT string_agg(reason,',' ORDER BY reason) FROM payments.review_cases WHERE attempt_id=$1),'')||'/'||
		(SELECT count(*) FROM payments.provider_observations x WHERE x.attempt_id=$1
		  AND NOT EXISTS(SELECT 1 FROM payments.facts y WHERE y.attempt_id=x.attempt_id AND y.source_report_hash=x.report_hash))`, q.result.AttemptID)
}

// pfuChainApplies runs the full real chain and requires every phase-2b ledger row.
func pfuChainApplies(t *testing.T, f *testFixture, label string) {
	t.Helper()
	if err := lriApplyFull(context.Background(), f.owner); err != nil {
		t.Fatalf("full chain over %s PAYUNi history must apply: %v", label, err)
	}
	for _, v := range []string{"0197_payuni_phase2b_drop.sql", pfuNumbered, "post_river/0026_payuni_phase2b_drop.sql", pfuPost} {
		if n := countRows(t, f.owner, `SELECT count(*) FROM public.lc_schema_migrations WHERE version=$1`, v); n != 1 {
			t.Errorf("%s chain: ledger has %d row(s) for %s", label, n, v)
		}
	}
}

// TestPayuniPhase2bFollowupLegacyChainReconciledAndUnapplied (PFU04, review-105-336b06942 P1; review-105-087c67855
// P2-1/P2-2): the full real chain (lriApplyFull, no pre-record) over legacy PAYUNi histories whose reports are each
// applied by the real 0018 body (payments.apply_capture on this schema).
//   - reconciled: seven reports; 0018 cites only the authorization in a fact; the pending/empty-TradeNo, DataSource B,
//     duplicate-auth and TradeStatus 2/3/4 reports are cited by nothing. The operator cancelled the order. 0197-0199
//     and post_river 0026/0027 must all apply.
//   - captured: a captured history with repeated-capture, refund and post-capture TradeStatus 4 reports must apply.
//   - failed-before-capture: the documented fail-safe refusal (runbook §4).
//   - unapplied: the authorization report is stored but was never applied. 0199 must refuse (22023) and the refusal
//     must roll back the whole numbered batch, including 0199's own revokes: start_payment / record_payment_query
//     grantees and the hosted-form writer authority are exactly as before (calibrated by pfuInject).
func TestPayuniPhase2bFollowupLegacyChainReconciledAndUnapplied(t *testing.T) {
	pfuRequire(t)
	ctx := context.Background()
	t.Run("reconciled", func(t *testing.T) {
		f, q := pfuLegacy(t)
		// TradeStatus 2 (failed) and 4 added per review-105-087c67855 P2-2; the expectation moved from AUTHORIZED/0/4
		// and 5 observations to AUTHORIZED/0/6 and 7 (same assertion, two more no-op reports).
		pfuApply(t, q,
			func(r map[string]any) { r["TradeStatus"], r["TradeNo"] = "0", "" },
			func(r map[string]any) { r["DataSource"] = "B" },
			func(map[string]any) {},
			func(r map[string]any) { r["CloseStatus"] = "1" },
			func(r map[string]any) { r["TradeStatus"] = "2" },
			func(r map[string]any) { r["TradeStatus"] = "3" },
			func(r map[string]any) { r["TradeStatus"] = "4" })
		if got := pfuApplied(t, q); got != "AUTHORIZED//6" {
			t.Fatalf("legacy 0018 history facts/reviews/uncited = %s, want AUTHORIZED//6 (six reports 0018 applied without citing them)", got)
		}
		pfuReconcile(t, q)
		pfuListedWant(t, q, "")
		pfuChainApplies(t, f, "reconciled")
		if n := countRows(t, f.owner, `SELECT count(*) FROM payments.provider_observations WHERE attempt_id=$1`, q.result.AttemptID); n != 7 {
			t.Errorf("reconciled chain changed PAYUNi observations: %d", n)
		}
	})
	// captured (review-105-087c67855 P2-2): authorization, capture (0018 allocates stock and confirms the order), a
	// repeated capture, a refund report (0018 writes REFUND_HISTORY) and an expired report after the capture (TradeStatus
	// 4 on a captured attempt: 0018 writes CONFLICTING_REPORT). Every report is applied or cited, so the chain applies.
	t.Run("captured", func(t *testing.T) {
		f, q := pfuLegacy(t)
		amount := q.result.AmountMinor / 100
		capture := func(r map[string]any) { r["CloseStatus"], r["CloseAmountTWD"] = "2", amount }
		pfuApply(t, q,
			func(map[string]any) {},
			capture,
			func(r map[string]any) { capture(r); r["CardRefundType"] = "" },
			func(r map[string]any) { capture(r); r["CardRefundAmountTWD"] = amount },
			func(r map[string]any) { r["TradeStatus"] = "4" })
		if got := pfuApplied(t, q); got != "AUTHORIZED,CAPTURED/CONFLICTING_REPORT,REFUND_HISTORY/3" {
			t.Fatalf("legacy 0018 captured history facts/reviews/uncited-by-fact = %s", got)
		}
		if n := countRows(t, f.owner, `SELECT count(*) FROM checkout.orders o JOIN inventory.reservations r ON r.id=o.id
			WHERE o.id=$1 AND o.commercial_state='CONFIRMED' AND r.state='COMMITTED'`, q.hold.OrderID); n != 1 {
			t.Fatal("legacy 0018 capture did not confirm the order and commit the stock")
		}
		pfeReplica(t, f.owner, `UPDATE integration.operations SET state='SUCCEEDED' WHERE id='`+q.result.OperationID+`'`)
		pfuListedWant(t, q, "")
		pfuChainApplies(t, f, "captured")
	})
	// failed-before-capture (review-105-087c67855 P2-1, runbook §4): a failed report 0018 applied as a no-op BEFORE a
	// later capture. 0018 wrote no CONFLICTING_REPORT (no CAPTURED fact at that moment); the guard judges today's
	// captured state and refuses. Deliberate fail-safe divergence from 0018: the case goes to the owner.
	t.Run("failed-before-capture", func(t *testing.T) {
		f, q := pfuLegacy(t)
		pfuApply(t, q,
			func(map[string]any) {},
			func(r map[string]any) { r["TradeStatus"] = "2" },
			func(r map[string]any) { r["CloseStatus"], r["CloseAmountTWD"] = "2", q.result.AmountMinor/100 })
		if got := pfuApplied(t, q); got != "AUTHORIZED,CAPTURED//1" {
			t.Fatalf("legacy 0018 history facts/reviews/uncited = %s, want AUTHORIZED,CAPTURED//1", got)
		}
		pfeReplica(t, f.owner, `UPDATE integration.operations SET state='SUCCEEDED' WHERE id='`+q.result.OperationID+`'`)
		pfuListedWant(t, q, pfeRow(t, f.owner, `SELECT encode(report_hash,'hex') FROM payments.provider_observations WHERE attempt_id=$1 AND report->>'TradeStatus'='2'`, q.result.AttemptID))
		ledger := lriLedger(t, f, "")
		pfuRefused(t, f.owner, pfuNumbered, lriApplyFull(ctx, f.owner), "PAYUNi phase-2b follow-up refused",
			"0 PAYUNi reservation(s) still PAYMENT_PENDING, 1 stored PAYUNi observation(s) never applied")
		if got := lriLedger(t, f, ""); got != ledger {
			t.Error("refused chain changed the migration ledger")
		}
	})
	t.Run("unapplied", func(t *testing.T) {
		f, q := pfuLegacy(t)
		pfuReport(t, q, func(r map[string]any) { r["TradeStatus"], r["TradeNo"] = "0", "" })
		paid := pfuReport(t, q, func(map[string]any) {}) // paid authorization whose reconcile job never ran
		pfuReconcile(t, q)
		pfuListedWant(t, q, hex.EncodeToString(paid))
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

// TestPayuniPhase2bFollowupQuotedGrantees (PFU05, issue #106): regrole::text already quotes a name that needs it, so
// 0199 must not quote it again (the pre-44a2114ad quote_ident(...) form REVOKEd from a role named `"Legacy Pay"` that does
// not exist and rolled the numbered batch back). Same window fixture as PFU03.
func TestPayuniPhase2bFollowupQuotedGrantees(t *testing.T) {
	pfuRequire(t)
	ctx := context.Background()
	db, versions := p2bPreState(t)
	roles := []string{"pfu_legacy_pay", "Legacy Pay", `Legacy "Q" Pay`}
	for _, r := range roles {
		mustExec(t, db, `CREATE ROLE `+pgx.Identifier{r}.Sanitize()+` NOLOGIN`)
	}
	fns := []string{pfuStart, pfuRecord}
	for _, fn := range fns {
		for _, r := range roles {
			mustExec(t, db, `GRANT EXECUTE ON FUNCTION `+fn+` TO `+pgx.Identifier{r}.Sanitize())
		}
		mustExec(t, db, `GRANT EXECUTE ON FUNCTION `+fn+` TO PUBLIC`)
		if n := countRows(t, db, `SELECT count(DISTINCT a.grantee) FROM pg_proc p, aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a
			WHERE p.oid=to_regprocedure($1) AND a.privilege_type='EXECUTE'
			AND (a.grantee=0 OR a.grantee IN (SELECT oid FROM pg_roles WHERE rolname=ANY($2)))`, fn, roles); n != 4 {
			t.Fatalf("baseline: %s has %d of the 4 seeded grantees (3 roles + PUBLIC) — gate would be vacuous", fn, n)
		}
	}
	var numbered, post []string
	for _, v := range versions {
		if strings.HasPrefix(v, "post_river/") {
			post = append(post, v)
		} else {
			numbered = append(numbered, v)
		}
	}
	// Numbered phase only (post-River held back, both functions still exist).
	mustExec(t, db, `DELETE FROM public.lc_schema_migrations WHERE version=ANY($1)`, numbered)
	if err := migrations.Apply(ctx, db); err != nil {
		t.Fatalf("numbered phase with quoted-name grantees: %v", err)
	}
	for _, fn := range fns {
		if rmReg(t, db, "to_regprocedure", fn) == "" {
			t.Fatalf("%s gone although post-River was held back — the window fixture is wrong", fn)
		}
		if g := pfuGrantees(t, db, fn); g != "" {
			t.Errorf("after 0199 %s still executable by %q", fn, g)
		}
	}
	// No extra grant left: 0199 re-applies.
	if err := pfuApplyOnly(t, db, pfuNumbered); err != nil {
		t.Fatalf("0199 with no extra grant: %v", err)
	}
	// No function left (post_river/0026 dropped both): 0199 re-applies.
	mustExec(t, db, `DELETE FROM public.lc_schema_migrations WHERE version=ANY($1)`, post)
	if err := migrations.Apply(ctx, db); err != nil {
		t.Fatalf("post-River phase: %v", err)
	}
	for _, fn := range fns {
		if rmReg(t, db, "to_regprocedure", fn) != "" {
			t.Fatalf("%s survived post_river/0026", fn)
		}
	}
	if err := pfuApplyOnly(t, db, pfuNumbered); err != nil {
		t.Fatalf("0199 with both functions gone: %v", err)
	}
}

// TestPayuniPhase2bFollowupGuardBranchSeeds (PFU06, issue #116 / review-115-10380a952): one seed per 0199/0027 guard
// branch PFU04 left unseeded, each derived from what the real 0018 body writes for it (0018_payment_capture.sql
// payments.apply_capture): an authorization (and, where named, a capture) is applied first, so the seed report is
// counted by its own branch only. Stored but never applied, it must refuse the full chain (22023, ledger unchanged);
// once 0018 applied it (the review case below is written), the full chain must apply.
func TestPayuniPhase2bFollowupGuardBranchSeeds(t *testing.T) {
	pfuRequire(t)
	ctx := context.Background()
	for _, c := range []struct {
		name     string
		captured bool
		seed     func(amount int64) func(map[string]any)
		applied  string
	}{
		// v_captured AND CloseStatus 3 -> CONFLICTING_REPORT.
		{"post-capture-close-status-3", true,
			func(int64) func(map[string]any) { return func(r map[string]any) { r["CloseStatus"] = "3" } },
			"AUTHORIZED,CAPTURED/CONFLICTING_REPORT/1"},
		// v_refund via CardRemainAmountTWD < amount -> REFUND_HISTORY.
		{"card-remain-below-amount", false,
			func(a int64) func(map[string]any) { return func(r map[string]any) { r["CardRemainAmountTWD"] = a - 1 } },
			"AUTHORIZED/REFUND_HISTORY/1"},
		// v_conflict: CardRemainAmountTWD > amount -> CONFLICTING_REPORT.
		{"card-remain-above-amount", false,
			func(a int64) func(map[string]any) { return func(r map[string]any) { r["CardRemainAmountTWD"] = a + 1 } },
			"AUTHORIZED/CONFLICTING_REPORT/1"},
		// CloseStatus 2 without CloseAmountTWD is not v_capture -> CAPTURE_EVIDENCE_INCOMPLETE.
		{"capture-evidence-incomplete", false,
			func(int64) func(map[string]any) { return func(r map[string]any) { r["CloseStatus"] = "2" } },
			"AUTHORIZED/CAPTURE_EVIDENCE_INCOMPLETE/1"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f, q := pfuLegacy(t)
			amount := int64(q.result.AmountMinor / 100)
			if amount < 2 {
				t.Fatalf("fixture amount %d TWD too small for the CardRemainAmountTWD seeds", amount)
			}
			pfuApply(t, q, func(map[string]any) {})
			if c.captured {
				pfuApply(t, q, func(r map[string]any) { r["CloseStatus"], r["CloseAmountTWD"] = "2", amount })
				pfeReplica(t, f.owner, `UPDATE integration.operations SET state='SUCCEEDED' WHERE id='`+q.result.OperationID+`'`)
			} else {
				pfuReconcile(t, q)
			}
			hash := pfuReport(t, q, c.seed(amount))
			pfuListedWant(t, q, hex.EncodeToString(hash))
			ledger := lriLedger(t, f, "")
			pfuRefused(t, f.owner, pfuNumbered, lriApplyFull(ctx, f.owner), "PAYUNi phase-2b follow-up refused",
				"0 PAYUNi reservation(s) still PAYMENT_PENDING, 1 stored PAYUNi observation(s) never applied")
			if got := lriLedger(t, f, ""); got != ledger {
				t.Fatal("refused chain changed the migration ledger")
			}
			pfuApplyHash(t, q, hash)
			pfuListedWant(t, q, "")
			if got := pfuApplied(t, q); got != c.applied {
				t.Fatalf("legacy 0018 history facts/reviews/uncited = %s, want %s", got, c.applied)
			}
			pfuChainApplies(t, f, c.name)
		})
	}
}

const (
	pfuRunbook      = "../../docs/runbooks/deploy.md"
	pfuRunbookBegin = "-- runbook:payuni-unapplied-begin"
	pfuRunbookEnd   = "-- runbook:payuni-unapplied-end"
	pfuGuardBegin   = "SELECT count(*) INTO v_obs "
	pfuGuardEnd     = "\n PERFORM set_config('row_security','on'"
	pfuFrom         = "FROM payments.provider_observations x"
	// The pinned projection (review-124-cb721a802 P2-2): one row per guard-counted report, no DISTINCT/aggregate/SRF.
	pfuRunbookSelect = "SELECT x.attempt_id, encode(x.report_hash,'hex') AS report_hash, x.report->>'TradeNo' AS trade_no, " +
		"x.report->>'TradeStatus' AS trade_status, x.report->>'CloseStatus' AS close_status, x.report->>'DataSource' AS data_source, " +
		"s.has_auth, s.has_cap, s.has_cei, s.has_ref, s.has_con"
	// The read-only wrapper the runbook prescribes around the listing (review-124-cb721a802 P2-1), fed to lib.sh's
	// lc_psql as a quoted heredoc: the pilot host has no MIGRATION_DATABASE_URL (review-125-c20176c95 P2-1).
	pfuRunbookTxOpen  = "\nsudo bash -c 'source deploy/scripts/lib.sh && lc_load_env \"$LC_COMPOSE_ENV\" && lc_psql' deploy-payuni-unapplied <<'SQL'\nBEGIN READ ONLY;\nSET LOCAL row_security = off;\n" + pfuRunbookBegin
	pfuRunbookTxClose = pfuRunbookEnd + "\nROLLBACK;\nSQL\n"
)

// pfuBetween returns the text of path between begin and end (exclusive), or fails.
func pfuBetween(t *testing.T, path, begin, end string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	b := strings.Index(s, begin)
	if b < 0 {
		t.Fatalf("%s lacks %q", path, begin)
	}
	s = s[b+len(begin):]
	e := strings.Index(s, end)
	if e < 0 {
		t.Fatalf("%s lacks %q after %q", path, end, begin)
	}
	return s[:e]
}

// pfuRunbookQuery is the owner's read-only listing from docs/runbooks/deploy.md §4 (the lines between the markers).
func pfuRunbookQuery(t *testing.T) string {
	t.Helper()
	q := pfuBetween(t, pfuRunbook, pfuRunbookBegin, pfuRunbookEnd)
	return strings.TrimSuffix(strings.TrimSpace(q[strings.Index(q, "\n")+1:]), ";")
}

// pfuListedWant runs the runbook listing as the runbook says (READ ONLY transaction, SET LOCAL row_security off,
// rolled back) and requires exactly the wanted
// report hashes (hex, comma-joined in order; "" = none).
func pfuListedWant(t *testing.T, q pqFixture, want string) {
	t.Helper()
	ctx := context.Background()
	tx, err := q.f.owner.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	var got string
	if _, err = tx.Exec(ctx, `SET LOCAL row_security=off`); err == nil {
		err = tx.QueryRow(ctx, `SELECT coalesce(string_agg(report_hash,',' ORDER BY report_hash),'') FROM (`+pfuRunbookQuery(t)+`) l`).Scan(&got)
	}
	if err != nil {
		t.Fatalf("runbook listing: %v", err)
	}
	if got != want {
		t.Errorf("runbook listing = %q, want exactly the guard-counted reports %q", got, want)
	}
}

// TestPayuniPhase2bFollowupRunbookQuery (PFU07, review-123-aee4382f9 P2-1).
func TestPayuniPhase2bFollowupRunbookQuery(t *testing.T) {
	pfuRequire(t)
	q := pfuRunbookQuery(t)
	i := strings.Index(q, pfuFrom)
	if i < 0 {
		t.Fatalf("runbook listing lacks %q", pfuFrom)
	}
	if sel := strings.Join(strings.Fields(q[:i]), " "); sel != pfuRunbookSelect {
		t.Errorf("runbook listing projection changed (a DISTINCT, aggregate or set-returning call would change the row set):\ngot:  %s\nwant: %s", sel, pfuRunbookSelect)
	}
	if rb := pfuBetween(t, pfuRunbook, "", "\n### 4.1"); !strings.Contains(rb, pfuRunbookTxOpen) || !strings.Contains(rb, pfuRunbookTxClose) {
		t.Errorf("runbook listing must be sudo bash -c 'source lib.sh ... lc_psql' deploy-payuni-unapplied <<'SQL' BEGIN READ ONLY; SET LOCAL row_security = off; ... ROLLBACK; SQL")
	}
	// Runnable on the pilot host (issue #116): operator-facing runbook and ops SQL must not use the unset
	// $MIGRATION_DATABASE_URL (grants go via lc_psql; prose naming the variable without "$" is fine).
	files, _ := filepath.Glob("../../scripts/ops/*.sql")
	if len(files) < 2 {
		t.Fatalf("scripts/ops/*.sql matched %v, want the two grant scripts (a moved directory must not silently pass)", files)
	}
	for _, f := range append(files, pfuRunbook) {
		b, err := os.ReadFile(f)
		if err != nil || strings.Contains(string(b), "$MIGRATION_DATABASE_URL") || strings.Contains(string(b), "${MIGRATION_DATABASE_URL") {
			t.Errorf("%s must not use $MIGRATION_DATABASE_URL or ${MIGRATION_DATABASE_URL} (unset on the pilot host; err=%v); use lc_psql", f, err)
		}
	}
	// sudo convention (review-153-a2a269d73 P2-1): on a fenced code line every deploy/scripts/*.sh invocation (lib.sh is
	// sourced, not run) is preceded by "sudo" plus optional VAR=x words, and VAR=x never precedes sudo (sudo drops it).
	rb, err := os.ReadFile(pfuRunbook)
	if err != nil {
		t.Fatal(err)
	}
	invoke, envBeforeSudo := regexp.MustCompile(`deploy/scripts/[a-z-]+\.sh`), regexp.MustCompile(`\b[A-Z][A-Z0-9_]*=\S+\s+sudo\b`)
	inFence := false
	for n, line := range strings.Split(string(rb), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if !inFence {
			continue
		}
		code, _, _ := strings.Cut(line, " #")
		if envBeforeSudo.MatchString(code) {
			t.Errorf("deploy.md:%d: VAR=x placed before sudo (sudo resets it); write sudo VAR=x cmd: %s", n+1, line)
		}
		for _, m := range invoke.FindAllStringIndex(code, -1) {
			if strings.HasSuffix(code[m[0]:m[1]], "/lib.sh") {
				continue
			}
			if !regexp.MustCompile(`sudo( [A-Z][A-Z0-9_]*=\S+)* $`).MatchString(code[:m[0]]) {
				t.Errorf("deploy.md:%d: %s is not preceded by sudo: %s", n+1, code[m[0]:m[1]], line)
			}
		}
	}
	listed := strings.Join(strings.Fields(q[i:]+";"), " ")
	for _, v := range []string{pfuNumbered, pfuPost} {
		if g := strings.Join(strings.Fields(pfuBetween(t, filepath.Join("../../migrations", v), pfuGuardBegin, pfuGuardEnd)), " "); g != listed {
			t.Errorf("runbook listing FROM..WHERE drifted from the %s guard predicate:\nrunbook: %s\nguard:   %s", v, listed, g)
		}
	}
	// An authorization with CloseStatus 3 applied before any capture: 0018 cites it in the AUTHORIZED fact and writes
	// nothing else; after a later capture the guard counts it (has_cap AND CloseStatus 3, no CONFLICTING_REPORT).
	f, p := pfuLegacy(t)
	auth := pfuReport(t, p, func(r map[string]any) { r["CloseStatus"] = "3" })
	pfuApplyHash(t, p, auth)
	pfuApply(t, p, func(r map[string]any) { r["CloseStatus"], r["CloseAmountTWD"] = "2", p.result.AmountMinor/100 })
	if got := pfuApplied(t, p); got != "AUTHORIZED,CAPTURED//0" {
		t.Fatalf("legacy 0018 history facts/reviews/uncited = %s, want AUTHORIZED,CAPTURED//0", got)
	}
	pfeReplica(t, f.owner, `UPDATE integration.operations SET state='SUCCEEDED' WHERE id='`+p.result.OperationID+`'`)
	pfuListedWant(t, p, hex.EncodeToString(auth))
	pfuRefused(t, f.owner, pfuNumbered, lriApplyFull(context.Background(), f.owner), "PAYUNi phase-2b follow-up refused",
		"0 PAYUNi reservation(s) still PAYMENT_PENDING, 1 stored PAYUNi observation(s) never applied")
}
