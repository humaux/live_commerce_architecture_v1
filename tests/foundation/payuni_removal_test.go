// Purpose: PAY-RM1 removal gates RM01/RM02 — after migrations/0161 the never-deployed W4-01B PAYUNi
//
//	notify receiver is gone (role, tables, four definers, review_cases relaxations, wake grant) while the
//	old PAYUNi hosted/query/capture path (0014–0018) is untouched; 0161 refuses to run over real data.
//
// Depends on: migrations.Apply (embedded SQL), per-test PG containers via mciStartPG, the shared
//
//	mustExec/countRows helpers; pg_catalog only (pg_constraint, pg_policy, to_regclass/regprocedure/regrole).
//
// Used by: scripts/dev/test-focused.sh '^TestRemovePayuniNotify' (local gate); CI full foundation suite.
// Invariants: never delete data silently — RM02 pins the RAISE-over-seeded-rows refusal (22023).
// Status: REAL_PG (no provider is called; seeded rows are synthetic).
package foundation_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/migrations"
)

// rmVersions lists migration file names (relative to migrations/) at or after the given cut points:
// numberedFrom over migrations/*.sql and postFrom over migrations/post_river/*.sql. postFrom="zzz"
// selects no post-River file. It mirrors the R2 upgrade gate's glob so a renumber shows up here.
func rmVersions(numberedFrom, postFrom string) []string {
	var out []string
	numbered, _ := filepath.Glob("../../migrations/[0-9][0-9][0-9][0-9]_*.sql")
	for _, p := range numbered {
		if b := filepath.Base(p); b >= numberedFrom {
			out = append(out, b)
		}
	}
	post, _ := filepath.Glob("../../migrations/post_river/[0-9][0-9][0-9][0-9]_*.sql")
	for _, p := range post {
		if b := filepath.Base(p); b >= postFrom {
			out = append(out, "post_river/"+b)
		}
	}
	sort.Strings(out)
	return out
}

// rmChecksums reads every listed migration from disk up front (a missing removal migration fails the
// test before a container is started) and returns the ledger checksums migrations.Apply expects.
func rmChecksums(t *testing.T, versions []string) map[string]string {
	t.Helper()
	out := make(map[string]string, len(versions))
	for _, v := range versions {
		body, err := os.ReadFile(filepath.Join("../../migrations", v))
		if err != nil {
			t.Fatalf("migration %s must exist: %v", v, err)
		}
		out[v] = fmt.Sprintf("%x", sha256.Sum256(body))
	}
	return out
}

// rmSkipLedger pre-marks versions as applied (same ledger trick as TestR2IntegrationUpgradeFromReleaseHead)
// so the following migrations.Apply skips them.
func rmSkipLedger(t *testing.T, pool *pgxpool.Pool, sums map[string]string) {
	t.Helper()
	mustExec(t, pool, `CREATE TABLE IF NOT EXISTS public.lc_schema_migrations (version text PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`)
	for v, sum := range sums {
		mustExec(t, pool, `INSERT INTO public.lc_schema_migrations(version,checksum) VALUES($1,$2)`, v, sum)
	}
}

// rmReg resolves one catalog name through to_regclass/to_regprocedure/to_regrole; "" means absent.
func rmReg(t *testing.T, pool *pgxpool.Pool, fn, name string) string {
	t.Helper()
	var out string
	if err := pool.QueryRow(context.Background(), `SELECT coalesce(`+fn+`($1)::text,'')`, name).Scan(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

// rmReasonCheckDef returns pg_get_constraintdef of payments.review_cases_reason_check.
func rmReasonCheckDef(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var def string
	if err := pool.QueryRow(context.Background(), `SELECT pg_get_constraintdef(c.oid) FROM pg_constraint c
		WHERE c.conrelid='payments.review_cases'::regclass AND c.conname='review_cases_reason_check'`).Scan(&def); err != nil {
		t.Fatal(err)
	}
	return def
}

func rmHashNotNull(t *testing.T, pool *pgxpool.Pool) bool {
	t.Helper()
	var notNull bool
	if err := pool.QueryRow(context.Background(), `SELECT a.attnotnull FROM pg_attribute a
		WHERE a.attrelid='payments.review_cases'::regclass AND a.attname='source_report_hash'`).Scan(&notNull); err != nil {
		t.Fatal(err)
	}
	return notNull
}

// rmAssertReceiverGone pins the RM01 object-level removal: every W4-01B catalog object is absent and the
// review_cases relaxations are reverted, while the stripe wake grant and the old PAYUNi path survive.
func rmAssertReceiverGone(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	for _, tbl := range []string{"payments.payuni_notify_receipts", "payments.payuni_notify_endpoints"} {
		if got := rmReg(t, pool, "to_regclass", tbl); got != "" {
			t.Errorf("table %s still exists after 0161", tbl)
		}
	}
	for _, fn := range []string{
		"payments.payuni_resolve_endpoint(bytea)",
		"payments.payuni_record_notify(bytea,bytea,text,text,bigint,text,text)",
		"payments.set_payuni_notify_endpoint(uuid,uuid,uuid,uuid,uuid,text,boolean,bytea)",
		"payments.guard_payuni_notify_receipt()",
	} {
		if got := rmReg(t, pool, "to_regprocedure", fn); got != "" {
			t.Errorf("function %s still exists after 0161", fn)
		}
	}
	if got := rmReg(t, pool, "to_regrole", "commerce_payuni_ingress"); got != "" {
		t.Errorf("role commerce_payuni_ingress still exists after 0161")
	}
	if n := countRows(t, pool, `SELECT count(*) FROM pg_policy WHERE polname IN ('payuni_notify_review','payuni_endpoint_integration','payuni_receipt_integration','payuni_endpoint_registry')`); n != 0 {
		t.Errorf("%d W4-01B policies survived 0161", n)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM pg_constraint WHERE conname='review_cases_report_hash_scope'`); n != 0 {
		t.Errorf("review_cases_report_hash_scope survived 0161")
	}
	if !rmHashNotNull(t, pool) {
		t.Errorf("review_cases.source_report_hash is still nullable after 0161")
	}
	var wake, stripeKind bool
	if err := pool.QueryRow(context.Background(), `SELECT has_column_privilege('commerce_integration_writer','river_payment.river_job','scheduled_at','UPDATE'),
		has_column_privilege('commerce_stripe_ingress','river_payment.river_job','kind','UPDATE')`).Scan(&wake, &stripeKind); err != nil {
		t.Fatal(err)
	}
	if wake {
		t.Errorf("commerce_integration_writer still holds the W4-01B UPDATE(scheduled_at) wake grant on river_payment.river_job")
	}
	if !stripeKind {
		t.Errorf("control: the stripe ingress UPDATE(kind) grant must survive the removal")
	}
	// Scope fence: the older PAYUNi hosted/query/capture path (0014–0018) is NOT part of this unit.
	if rmReg(t, pool, "to_regprocedure", "payments.apply_capture_payuni_v1(uuid,bytea)") == "" {
		t.Errorf("old PAYUNi capture definer was removed — out of PAY-RM1 scope")
	}
	if rmReg(t, pool, "to_regclass", "integration.merchant_accounts") == "" {
		t.Errorf("integration.merchant_accounts was removed — out of PAY-RM1 scope")
	}
	if rmReg(t, pool, "to_regrole", "commerce_payment_registrar") == "" {
		t.Errorf("commerce_payment_registrar was removed — out of PAY-RM1 scope")
	}
}

// TestRemovePayuniNotifyRM01Schema (RM01): database A stops before 0136 (ledger pre-mark), database B
// applies everything including 0161. B must equal the pre-0136 review_cases shape and contain none of
// the W4-01B objects. REAL_PG; two disposable containers.
func TestRemovePayuniNotifyRM01Schema(t *testing.T) {
	ctx := context.Background()
	sums := rmChecksums(t, rmVersions("0136", "0022"))
	if len(sums) == 0 {
		t.Fatal("no migrations at or after 0136 — glob broken")
	}
	pre := mciStartPG(t)
	rmSkipLedger(t, pre, sums)
	if err := migrations.Apply(ctx, pre); err != nil {
		t.Fatalf("pre-0136 apply: %v", err)
	}
	defBefore := rmReasonCheckDef(t, pre)
	if !rmHashNotNull(t, pre) {
		t.Fatal("pre-0136 baseline: source_report_hash must be NOT NULL")
	}
	if got := rmReg(t, pre, "to_regclass", "payments.payuni_notify_receipts"); got != "" {
		t.Fatalf("pre-0136 baseline already has the notify receiver (%s) — skip ledger is wrong", got)
	}

	fresh := mciStartPG(t)
	if err := migrations.Apply(ctx, fresh); err != nil {
		t.Fatalf("fresh apply with 0161: %v", err)
	}
	defAfter := rmReasonCheckDef(t, fresh)
	if defAfter != defBefore {
		t.Errorf("review_cases_reason_check did not return to the pre-0136 definition:\n pre-0136: %s\n after 0161: %s", defBefore, defAfter)
	}
	if strings.Contains(defAfter, "NOTIFY_MISMATCH") {
		t.Errorf("NOTIFY_MISMATCH survived in the reason check: %s", defAfter)
	}
	rmAssertReceiverGone(t, fresh)
}

// TestRemovePayuniNotifyRM02SeededDataBlocks (RM02): on a database at the 0160 state (0161 pre-marked),
// one seeded receipt row — and separately one NOTIFY_MISMATCH review case — must make 0161 abort with
// the named 22023 exception, leaving the receiver and the data fully intact. Forward-only, never silent.
func TestRemovePayuniNotifyRM02SeededDataBlocks(t *testing.T) {
	ctx := context.Background()
	versions := rmVersions("0161", "zzz")
	if len(versions) != 1 || !strings.HasPrefix(versions[0], "0161_") {
		t.Fatalf("expected exactly one 0161_* removal migration, got %v", versions)
	}
	sums := rmChecksums(t, versions)
	db := mciStartPG(t)
	rmSkipLedger(t, db, sums)
	if err := migrations.Apply(ctx, db); err != nil {
		t.Fatalf("0160-state apply: %v", err)
	}
	if rmReg(t, db, "to_regclass", "payments.payuni_notify_receipts") == "" {
		t.Fatal("0160-state baseline lacks the notify receiver — the skip ledger is wrong")
	}
	unmark := func() {
		mustExec(t, db, `DELETE FROM public.lc_schema_migrations WHERE version LIKE '0161_%'`)
	}
	// Seeding runs with FK triggers off on one pooled connection: the guard only counts rows, and the
	// synthetic receipt deliberately has no real endpoint/merchant chain behind it.
	conn, err := db.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	seed := func(sql string, args ...any) {
		if _, err := conn.Exec(ctx, `SET session_replication_role='replica'`); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, `RESET session_replication_role`); err != nil {
			t.Fatal(err)
		}
	}
	assertRefused := func(what string) {
		t.Helper()
		unmark()
		err := migrations.Apply(ctx, db)
		if err == nil {
			t.Fatalf("0161 applied over %s — data would have been deleted silently", what)
		}
		if !strings.Contains(err.Error(), "PAYUNi notify removal refused") || !strings.Contains(err.Error(), "22023") {
			t.Fatalf("0161 failed over %s with the wrong error: %v", what, err)
		}
		// Nothing changed: the receiver, its relaxation and the seeded row all survive the refused run.
		if rmReg(t, db, "to_regclass", "payments.payuni_notify_receipts") == "" {
			t.Errorf("refused 0161 dropped the receipts table over %s", what)
		}
		if !strings.Contains(rmReasonCheckDef(t, db), "NOTIFY_MISMATCH") {
			t.Errorf("refused 0161 restored the reason check over %s", what)
		}
		if n := countRows(t, db, `SELECT count(*) FROM public.lc_schema_migrations WHERE version LIKE '0161_%'`); n != 0 {
			t.Errorf("refused 0161 recorded itself in the ledger over %s", what)
		}
	}

	seed(`INSERT INTO payments.payuni_notify_receipts(id,endpoint_id,connection_id,environment,account_id,
		payload_sha256,merchant_trade_no,provider_reference,amount_twd,status,trade_status,disposition,reason)
		VALUES(gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),'SANDBOX','mock-account',
		decode(repeat('ab',32),'hex'),'ORDER_RM02','',25,'SUCCESS','1','UNKNOWN_TRADE','notify_unknown_trade')`)
	if n := countRows(t, db, `SELECT count(*) FROM payments.payuni_notify_receipts`); n != 1 {
		t.Fatalf("receipt seed visible as %d rows", n)
	}
	assertRefused("one receipt row")

	seed(`DELETE FROM payments.payuni_notify_receipts`)
	seed(`INSERT INTO payments.review_cases(tenant_id,store_id,attempt_id,reason,source_report_hash)
		VALUES(gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),'NOTIFY_MISMATCH',NULL)`)
	if n := countRows(t, db, `SELECT count(*) FROM payments.review_cases WHERE reason='NOTIFY_MISMATCH'`); n != 1 {
		t.Fatalf("review-case seed visible as %d rows", n)
	}
	assertRefused("one NOTIFY_MISMATCH review case")
}
