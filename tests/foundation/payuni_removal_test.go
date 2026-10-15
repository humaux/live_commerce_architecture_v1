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
// Historical post-River consumers stay held until their root dependencies exist; RM02 releases only 0161.
// PAY-RM2 adds runtime refusal and historical-read gates; no phase 2a schema change.
// Status: REAL_PG (CI only; no provider is called; seeded rows are synthetic).
package foundation_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"livecommerce/internal/checkout"
	"livecommerce/internal/command"
	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/payments"
	"livecommerce/internal/platform"

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
	// Every migration from 0161 on is pre-marked so the DB stays at the 0160 state; unmark() releases only 0161,
	// so later migrations (0162, ...) stay skipped and never run on the half-built schema.
	versions := rmVersions("0161", "0023")
	n0161 := 0
	for _, v := range versions {
		if strings.HasPrefix(v, "0161_") {
			n0161++
		}
	}
	if n0161 != 1 || !strings.HasPrefix(versions[0], "0161_") {
		t.Fatalf("expected exactly one 0161_* removal migration (first from 0161 on), got %v", versions)
	}
	sums := rmChecksums(t, versions)
	if _, ok := sums["post_river/0023_order_attempt_jobs.sql"]; !ok {
		t.Fatal("0160 baseline must hold the post-River order-attempt consumer with its unavailable root dependencies")
	}
	db := mciStartPG(t)
	rmSkipLedger(t, db, sums)
	if err := migrations.Apply(ctx, db); err != nil {
		t.Fatalf("0160-state apply: %v", err)
	}
	if rmReg(t, db, "to_regclass", "payments.payuni_notify_receipts") == "" {
		t.Fatal("0160-state baseline lacks the notify receiver — the skip ledger is wrong")
	}
	if n := countRows(t, db, `SELECT count(*) FROM pg_roles WHERE rolname='commerce_order_attempt_owner'`); n != 0 {
		t.Fatal("0160-state baseline unexpectedly applied the later order-attempt identity migration")
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

// rm2HistoricalAttempt models existing rows, never invokes a Go or SQL start API.
func rm2HistoricalAttempt(t *testing.T, p psHarness) checkout.PaymentResult {
	return rm2HistoryIn(t, p, "river_payment", "PROVIDER_MOCK", "payment_mock_v1")
}

// The owner inserts a synthetic archived aggregate with its original job in one transaction.
// Existing FK, queue-routing and provenance triggers remain enabled. No credentials are loaded,
// no start function is called and no network request is made. Used also on old-schema upgrades.
func rm2HistoryIn(t *testing.T, p psHarness, schema, profile, queue string) checkout.PaymentResult {
	t.Helper()
	if (schema != "river" && schema != "river_payment") || (profile != "PROVIDER_MOCK" && profile != "SANDBOX" && profile != "LIVE") {
		t.Fatal("invalid historical fixture target")
	}
	ctx := context.Background()
	jobs, err := river.NewClient(riverpgxv5.New(p.f.owner), &river.Config{Schema: schema})
	if err != nil {
		t.Fatal(err)
	}
	tx, err := p.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT set_config('app.tenant_id',$1,true),set_config('app.store_id',$2,true),set_config('app.principal_id','',true),set_config('app.buyer_id',$3,true),set_config('app.buyer_session_id',$4,true)`, p.f.tenantA, p.f.storeA1, p.cap.Scope.OwnerID, p.cap.Scope.SessionID); err != nil {
		t.Fatal(err)
	}
	attempt := randomUUID()
	job, err := jobs.InsertTx(ctx, tx, pqArgs{OperationID: attempt, Version: 1}, &river.InsertOpts{Queue: queue, ScheduledAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	var out checkout.PaymentResult
	err = tx.QueryRow(ctx, `INSERT INTO checkout.payment_attempts(tenant_id,store_id,owner_id,id,session_id,order_id,market_id,country,method_code,method_version,connection_id,credential_version,qualification_id,environment,execution_profile,binding_id,binding_version,currency,amount_minor,merchant_trade_no,state,generation,job_id)
 SELECT o.tenant_id,o.store_id,o.owner_id,$1,$2,o.id,o.market_id,o.country,m.code,m.version,a.id,a.credential_version,q.id,a.environment,$3,b.id,b.semantic_version,o.currency,o.total_minor,'P'||rtrim(translate(encode(uuid_send($1::uuid),'base64'),'+/','-_'),'='),'PAYMENT_PENDING',o.generation+1,$4
 FROM checkout.orders o JOIN payments.method_versions m ON m.tenant_id=o.tenant_id AND m.store_id=o.store_id AND m.market_id=o.market_id AND m.country=o.country AND m.code=$5 AND m.version=$6
 JOIN integration.merchant_accounts a ON a.id=m.connection_id JOIN integration.bindings b ON b.id=a.binding_id JOIN payments.account_qualifications q ON q.id=m.qualification_id
 WHERE o.id=$7 RETURNING order_id::text,id::text,id::text,job_id,generation,merchant_trade_no,currency,amount_minor,state`, attempt, p.cap.Scope.SessionID, profile, job.Job.ID, p.input.MethodCode, p.input.MethodVersion, p.hold.OrderID).Scan(&out.OrderID, &out.AttemptID, &out.OperationID, &out.JobID, &out.Generation, &out.MerchantTradeNo, &out.Currency, &out.AmountMinor, &out.State)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("synthetic archived payment"))
	if _, err = tx.Exec(ctx, `INSERT INTO integration.operations(id,tenant_id,store_id,principal_id,binding_id,binding_version,provider,external_asset_id,purpose,action,semantic_key,request_hash,request,job_id,state,generation,actor_kind,payment_attempt_id,buyer_owner_id,buyer_session_id)
 SELECT a.id,a.tenant_id,a.store_id,NULL,a.binding_id,a.binding_version,'payuni',b.external_asset_id,'transactional','payuni.query','payment.query:'||a.id,$2,jsonb_build_object('attempt_id',a.id::text),a.job_id,'UNKNOWN',1,'BUYER_PAYMENT_QUERY',a.id,a.owner_id,a.session_id FROM checkout.payment_attempts a JOIN integration.bindings b ON b.id=a.binding_id WHERE a.id=$1`, attempt, digest[:]); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code) SELECT tenant_id,store_id,id,1,'UNKNOWN','','buyer_payment_started' FROM checkout.payment_attempts WHERE id=$1`, attempt); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE checkout.orders SET commercial_state='AWAITING_PAYMENT',generation=$2,updated_at=clock_timestamp() WHERE id=$1`, p.hold.OrderID, out.Generation); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `SELECT set_config('app.tenant_id',$1,true),set_config('app.store_id',$2,true),set_config('app.buyer_id',$3,true),set_config('app.buyer_session_id',(SELECT creator_session_id::text FROM checkout.orders WHERE id=$4),true)`, p.f.tenantA, p.f.storeA1, p.cap.Scope.OwnerID, p.hold.OrderID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE inventory.reservations SET state='PAYMENT_PENDING',generation=$2 WHERE id=$1`, p.hold.OrderID, out.Generation); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `SELECT set_config('app.buyer_session_id',$1,true)`, p.cap.Scope.SessionID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO checkout.events(tenant_id,store_id,owner_id,order_id,session_id,generation,action,actor_kind) SELECT tenant_id,store_id,owner_id,order_id,session_id,generation,'checkout.payment_started','BUYER' FROM checkout.payment_attempts WHERE id=$1`, attempt); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO checkout.command_results(tenant_id,store_id,owner_id,operation,idempotency_key,creator_session_id,request_hash,order_id,response) SELECT tenant_id,store_id,owner_id,'checkout.payment.start',$2,session_id,$3,order_id,$4::jsonb FROM checkout.payment_attempts WHERE id=$1`, attempt, t04Key("rm2-history"), digest[:], raw); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return out
}

// rm2Snapshot covers the attempt, operation (including lease/generation/result), order, reservation,
// observations, facts, inventory ledger and operation events. River job cancellation is deliberately
// separate: acknowledging an obsolete queue item must not mutate any payment or inventory state.
func rm2Snapshot(t *testing.T, p psHarness, attempt string) string {
	t.Helper()
	var out string
	err := p.f.owner.QueryRow(context.Background(), `SELECT jsonb_build_array(
  (SELECT to_jsonb(a) FROM checkout.payment_attempts a WHERE a.id=$1),
  (SELECT to_jsonb(o) FROM integration.operations o WHERE o.id=$1),
  (SELECT to_jsonb(o) FROM checkout.orders o WHERE o.id=$2),
  (SELECT to_jsonb(r) FROM inventory.reservations r WHERE r.id=$2),
  (SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text),'[]') FROM payments.provider_observations x WHERE attempt_id=$1),
  (SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text),'[]') FROM payments.facts x WHERE attempt_id=$1),
  (SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text),'[]') FROM payments.review_cases x WHERE attempt_id=$1),
  (SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text),'[]') FROM integration.operation_events x WHERE operation_id=$1),
  (SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text),'[]') FROM inventory.ledger x WHERE reservation_id=$2),
  (SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text),'[]') FROM checkout.command_results x WHERE order_id=$2),
  (SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text),'[]') FROM checkout.events x WHERE order_id=$2),
  (SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text),'[]') FROM inventory.balances x WHERE (tenant_id,store_id)=(SELECT tenant_id,store_id FROM checkout.orders WHERE id=$2)),
  (SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text),'[]') FROM fulfillment.payment_work_items x WHERE attempt_id=$1),
  (SELECT to_jsonb(x) FROM checkout.hosted_payment_pages x WHERE attempt_id=$1)
 )::text`, attempt, p.hold.OrderID).Scan(&out)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestRemovePayuniGoRM2UnavailableAndNoStart(t *testing.T) {
	h := hpSetup(t) // existing fixture: eligible legacy method; production Go API under test
	before, beforeOther := bpCounts(t, h)
	err := platform.WithScope(context.Background(), h.f.runtime, h.f.tokens["a"], h.f.storeA1, "integration:read", func(tx pgx.Tx, scope platform.Scope) error {
		out, err := payments.InspectMethod(context.Background(), tx, scope, h.f.tokens["a"], payments.CheckInput{MarketID: h.market.ID, Country: "TW", Code: "payuni_credit", ExpectedVersion: 1, Environment: "SANDBOX", Currency: "TWD", AmountMinor: 2500})
		if err != nil {
			return err
		}
		if out.Available || !containsString(out.Reasons, "ADAPTER_UNAVAILABLE") {
			t.Fatalf("removed method available: %+v", out)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		view, err := h.api.(*checkout.HostedPaymentStarter).PaymentView(context.Background(), h.cap.Token, h.f.storeA1, h.hold.OrderID)
		if err != nil || len(view.Methods) != 0 || view.PaymentState != "NOT_STARTED" {
			t.Fatalf("removed buyer method offered: %+v %v", view, err)
		}
		if out, err := h.begin(t04Key("rm2-start")); !errors.Is(err, command.ErrInvalid) || out != (checkout.PaymentResult{}) {
			t.Fatalf("removed hosted start: %+v %v", out, err)
		}
		if out, err := h.start(t04Key("rm2-old-start")); !errors.Is(err, command.ErrInvalid) || out != (checkout.PaymentResult{}) {
			t.Fatalf("removed legacy start: %+v %v", out, err)
		}
		if out, err := h.take(); !errors.Is(err, command.ErrConflict) || out.Form != nil || out.RedirectURL != "" {
			t.Fatalf("removed form released: %+v %v", out, err)
		}
	}
	after, afterOther := bpCounts(t, h)
	if before != after || beforeOther != afterOther {
		t.Fatal("removed payment request changed durable state")
	}
}

func TestRemovePayuniGoRM2WorkerSkipsHistoryWithoutObservation(t *testing.T) {
	p := psSetupItemsOn(t, pwIsolatedFixture(t), 1)
	result := rm2HistoricalAttempt(t, p)
	keys, err := accounts.NewKeyring("rm2_test", map[string][]byte{"rm2_test": randomBytes(32)}, randomBytes(32))
	if err != nil {
		t.Fatal(err)
	}
	q := pqFixture{psHarness: p, keys: keys, result: result, schema: "river_payment"}
	// Seed one already persisted authenticated report and reconciliation job using unchanged SQL.
	// The new Go capture worker must refuse even this valid historical report.
	pcRecord(t, q, pcFull(q))
	before := rm2Snapshot(t, p, result.AttemptID)
	var calls atomic.Int64
	opts := payments.DefaultQueryWorkerOptions()
	opts.MockTransport = pqTransport(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, errors.New("removed provider must never be called")
	})
	client, err := payments.NewWorkerClient(context.Background(), p.worker, q.keys, "PROVIDER_MOCK", 1, opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pwStopClient(t, client) })
	mustExec(t, p.f.owner, `UPDATE river_payment.river_job SET scheduled_at=clock_timestamp() WHERE id=$1`, result.JobID)
	if err = client.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	pwAwaitJob(t, q, result.JobID, "cancelled")
	var captureJob int64
	if err = p.f.owner.QueryRow(context.Background(), `SELECT id FROM river_payment.river_job WHERE kind='payment_reconcile_v1' AND args->>'operation_id'=$1`, result.OperationID).Scan(&captureJob); err != nil {
		t.Fatal(err)
	}
	pwAwaitJob(t, q, captureJob, "cancelled")
	pwStopClient(t, client)
	if calls.Load() != 0 {
		t.Fatalf("removed provider called %d times", calls.Load())
	}
	if after := rm2Snapshot(t, p, result.AttemptID); after != before {
		t.Fatalf("history mutated after worker: before=%s after=%s", before, after)
	}
	// Merchant and buyer views retain the stored state, rather than hiding the old attempt.
	moGrant(t, p.f, p.f.tenantA, p.f.storeA1, p.f.principalA)
	rows, err := moRead(context.Background(), p.f.runtime, p.f.tokens["a"], p.f.tenantA, p.f.storeA1, p.f.principalA, p.hold.OrderID, "all", 1)
	if err != nil || len(rows) != 1 || rows[0]["payment_state"] != "PENDING" || rows[0]["commercial_state"] != "AWAITING_PAYMENT" {
		t.Fatalf("merchant history hidden or changed: %+v %v", rows, err)
	}
	// Buyer view must retain the recorded PENDING state; a handoff remains unavailable.
	pool, err := platform.OpenHostedPool(context.Background(), hpRole(t, p.f))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	service := hpStarter(t, pool, "PROVIDER_MOCK", q.keys, checkout.HostedConfig{}).(*checkout.HostedPaymentStarter)
	for i := 0; i < 2; i++ {
		view, err := service.PaymentView(context.Background(), p.cap.Token, p.f.storeA1, p.hold.OrderID)
		if err != nil || view.PaymentState != "PENDING" || view.CommercialState != "AWAITING_PAYMENT" || len(view.Methods) != 0 {
			t.Fatalf("history hidden or changed: %+v %v", view, err)
		}
		if out, err := service.TakeHosted(context.Background(), p.cap.Token, p.f.storeA1, p.hold.OrderID); !errors.Is(err, command.ErrConflict) || out.Form != nil || out.RedirectURL != "" {
			t.Fatalf("history resumed: %+v %v", out, err)
		}
	}
	if after := rm2Snapshot(t, p, result.AttemptID); after != before {
		t.Fatal("historical read changed state")
	}
}

func containsString(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
