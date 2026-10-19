// Purpose: PAY-RM2 phase-2b inverse gates. P2B01: after migrations/0197 + post_river/0026 the ten retained PAYUNi
//
//	SQL execution definers are gone on a fresh and on an upgraded database, and payments.apply_capture is the
//	pre-0197 dispatcher minus only its PAYUNi branch (same owner, ACL, SECURITY DEFINER, search_path).
//	P2B02: 0197 refuses (22023, nothing changed) while a PAYUNi query operation is non-terminal.
//	P2B03: on the full schema a Stripe capture still settles (fact, order, stock, work item, replay), while a
//	historical PAYUNi attempt with a valid stored report is refused by the dispatcher and stays readable, unchanged.
//
// Depends on: migrations.Apply; mciStartPG; rmVersions/rmChecksums/rmSkipLedger/rmReg/rm2HistoricalAttempt/
//
//	rm2Snapshot (payuni_removal_test.go); sucOrder (stripe_untracked_capture_test.go); psSetupItemsOn, pcFull,
//	pqJSON, moGrant/moRead, hpRole/hpStarter, platform.OpenHostedPool.
//
// Used by: CI full foundation suite; scripts/dev/test-focused.sh '^TestPayuniPhase2b'.
// Invariants: no data is deleted by the migration or by these gates; seeded rows are synthetic (no PII, no
// credentials, no provider call). The dispatcher expectation is derived from the pre-0197 installed body, never
// from a hand-copied string, so any Stripe-side drift fails here.

package foundation_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/checkout"
	"livecommerce/internal/command"
	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/platform"
	"livecommerce/migrations"
)

// p2bDropped is every SQL object PAY-RM2 phase 2b removes (DELIVERY.md "Phase 2b"), with its exact signature.
var p2bDropped = []string{
	"checkout.start_payment(bytea,uuid,text,bytea,uuid,text,bigint,text,uuid,bigint)",
	"integration.require_payment_query(uuid,bigint,bytea,text)",
	"integration.load_payment_query(uuid,bigint,bytea,text)",
	"integration.record_payment_query(uuid,bigint,bytea,text,jsonb,bigint)",
	"integration.finish_payment_query(uuid,bigint,bytea,text,text,text)",
	"payments.apply_capture_payuni_v1(uuid,bytea)",
	"checkout.hosted_guard(bytea,uuid,uuid,text,boolean)",
	"checkout.load_hosted_material(bytea,uuid,uuid,text,text)",
	"checkout.save_hosted_page(bytea,uuid,uuid,text,text,bytea,jsonb)",
	"checkout.take_hosted_page(bytea,uuid,uuid,text,bytea)",
}

// p2bKept are the historical read paths and tables that must survive the drop.
var p2bKept = []struct{ fn, name string }{
	{"to_regprocedure", "checkout.hosted_payment_view(bytea,uuid,uuid,text,bytea)"},
	{"to_regprocedure", "checkout.hosted_payment_view_v2(bytea,uuid,uuid,text,bytea,bytea)"},
	{"to_regprocedure", "checkout.hosted_payment_provider(bytea,uuid,uuid)"},
	{"to_regprocedure", "payments.apply_stripe_observation(uuid,bytea)"},
	{"to_regclass", "checkout.hosted_payment_pages"},
	{"to_regclass", "checkout.payment_attempts"},
	{"to_regclass", "payments.provider_observations"},
	{"to_regclass", "payments.facts"},
	{"to_regclass", "integration.operations"},
}

// p2bPayuniBranch is the only text 0197 removes from the 0062 dispatcher body.
const p2bPayuniBranch = " IF v_provider='payuni' THEN\n  PERFORM payments.apply_capture_payuni_v1(p_attempt,p_report_hash);\n ELSIF v_provider='stripe' THEN"

type p2bDispatcher struct {
	body, owner, acl, config string
	definer                  bool
}

func p2bReadDispatcher(t *testing.T, pool *pgxpool.Pool) p2bDispatcher {
	t.Helper()
	var d p2bDispatcher
	if err := pool.QueryRow(context.Background(), `SELECT prosrc,pg_get_userbyid(proowner),coalesce(proacl::text,''),
		coalesce(array_to_string(proconfig,','),''),prosecdef FROM pg_proc WHERE oid='payments.apply_capture(uuid,bytea)'::regprocedure`).
		Scan(&d.body, &d.owner, &d.acl, &d.config, &d.definer); err != nil {
		t.Fatal(err)
	}
	return d
}

// p2bAssertDropped pins the post-0197 shape against the pre-0197 dispatcher read from the same migration set.
func p2bAssertDropped(t *testing.T, pool *pgxpool.Pool, label string, pre p2bDispatcher) {
	t.Helper()
	for _, sig := range p2bDropped {
		if got := rmReg(t, pool, "to_regprocedure", sig); got != "" {
			t.Errorf("%s: %s still exists after PAY-RM2 phase 2b", label, sig)
		}
	}
	for _, k := range p2bKept {
		if rmReg(t, pool, k.fn, k.name) == "" {
			t.Errorf("%s: retained history object %s was removed", label, k.name)
		}
	}
	if strings.Count(pre.body, p2bPayuniBranch) != 1 {
		t.Fatalf("%s: pre-0197 dispatcher does not hold exactly one PAYUNi branch:\n%s", label, pre.body)
	}
	want := pre
	want.body = strings.Replace(pre.body, p2bPayuniBranch, " IF v_provider='stripe' THEN", 1)
	if got := p2bReadDispatcher(t, pool); got != want {
		t.Errorf("%s: apply_capture is not the pre-0197 dispatcher minus its PAYUNi branch:\n got=%+v\nwant=%+v", label, got, want)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
		WHERE n.nspname IN ('payments','checkout','integration') AND p.prosrc ILIKE '%apply_capture_payuni_v1%'`); n != 0 {
		t.Errorf("%s: %d installed function(s) still call apply_capture_payuni_v1", label, n)
	}
}

// p2bPreState returns a database migrated to just before 0197/post_river 0026, plus the ledger rows to release.
func p2bPreState(t *testing.T) (*pgxpool.Pool, []string) {
	t.Helper()
	versions := rmVersions("0197", "0026")
	sums := rmChecksums(t, versions)
	if _, ok := sums["0197_payuni_phase2b_drop.sql"]; !ok {
		t.Fatalf("migrations/0197_payuni_phase2b_drop.sql must exist (got %v)", versions)
	}
	if _, ok := sums["post_river/0026_payuni_phase2b_drop.sql"]; !ok {
		t.Fatalf("migrations/post_river/0026_payuni_phase2b_drop.sql must exist (got %v)", versions)
	}
	db := mciStartPG(t)
	rmSkipLedger(t, db, sums)
	if err := migrations.Apply(context.Background(), db); err != nil {
		t.Fatalf("pre-0197 apply: %v", err)
	}
	for _, sig := range p2bDropped {
		if rmReg(t, db, "to_regprocedure", sig) == "" {
			t.Fatalf("pre-0197 baseline lacks %s — the skip ledger is wrong", sig)
		}
	}
	return db, versions
}

// TestPayuniPhase2bSchemaFreshAndUpgrade (P2B01): upgrade and fresh install converge on the same dropped shape.
func TestPayuniPhase2bSchemaFreshAndUpgrade(t *testing.T) {
	ctx := context.Background()
	db, versions := p2bPreState(t)
	pre := p2bReadDispatcher(t, db)
	mustExec(t, db, `DELETE FROM public.lc_schema_migrations WHERE version=ANY($1)`, versions)
	for i := 0; i < 2; i++ { // the second Apply must be a no-op
		if err := migrations.Apply(ctx, db); err != nil {
			t.Fatalf("phase-2b upgrade apply %d: %v", i+1, err)
		}
	}
	p2bAssertDropped(t, db, "upgrade", pre)

	fresh := mciStartPG(t)
	if err := migrations.Apply(ctx, fresh); err != nil {
		t.Fatalf("fresh apply: %v", err)
	}
	p2bAssertDropped(t, fresh, "fresh", pre)
}

// TestPayuniPhase2bRefusesUnreconciledPayuni (P2B02): one non-terminal PAYUNi query operation blocks 0197 with the
// named 22023 refusal and leaves every definer and the row in place; once it is terminal the drop proceeds and the
// row is still there (nothing is deleted).
func TestPayuniPhase2bRefusesUnreconciledPayuni(t *testing.T) {
	ctx := context.Background()
	db, versions := p2bPreState(t)
	conn, err := db.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	// FK triggers off on one connection: the guard only reads the operation state; CHECK constraints stay on.
	seed := func(sql string, args ...any) {
		t.Helper()
		for _, q := range []string{`SET session_replication_role='replica'`, sql, `RESET session_replication_role`} {
			var a []any
			if q == sql {
				a = args
			}
			if _, err := conn.Exec(ctx, q, a...); err != nil {
				t.Fatal(err)
			}
		}
	}
	op := randomUUID()
	seed(`INSERT INTO integration.operations(id,tenant_id,store_id,principal_id,binding_id,binding_version,provider,external_asset_id,purpose,action,semantic_key,request_hash,request,job_id,state,generation,actor_kind,payment_attempt_id,buyer_owner_id,buyer_session_id)
		VALUES($1,gen_random_uuid(),gen_random_uuid(),NULL,gen_random_uuid(),1,'payuni','SANDBOX:p2b-synthetic','transactional','payuni.query','payment.query:'||$1,decode(repeat('ab',32),'hex'),jsonb_build_object('attempt_id',$1),1,'UNKNOWN',1,'BUYER_PAYMENT_QUERY',$1::uuid,gen_random_uuid(),gen_random_uuid())`, op)
	mustExec(t, db, `DELETE FROM public.lc_schema_migrations WHERE version=ANY($1)`, versions)
	err = migrations.Apply(ctx, db)
	if err == nil || !strings.Contains(err.Error(), "PAYUNi phase-2b drop refused") || !strings.Contains(err.Error(), "22023") {
		t.Fatalf("0197 over a non-terminal PAYUNi operation: %v", err)
	}
	for _, sig := range p2bDropped {
		if rmReg(t, db, "to_regprocedure", sig) == "" {
			t.Errorf("refused 0197 still dropped %s", sig)
		}
	}
	if n := countRows(t, db, `SELECT count(*) FROM public.lc_schema_migrations WHERE version=ANY($1)`, versions); n != 0 {
		t.Errorf("refused phase 2b recorded %d ledger row(s)", n)
	}
	seed(`UPDATE integration.operations SET state='FAILED_FINAL' WHERE id=$1`, op)
	if err = migrations.Apply(ctx, db); err != nil {
		t.Fatalf("0197 over a reconciled (terminal) PAYUNi operation: %v", err)
	}
	if n := countRows(t, db, `SELECT count(*) FROM integration.operations WHERE id=$1 AND state='FAILED_FINAL'`, op); n != 1 {
		t.Errorf("historical PAYUNi operation not retained after phase 2b: %d", n)
	}
	for _, sig := range p2bDropped {
		if got := rmReg(t, db, "to_regprocedure", sig); got != "" {
			t.Errorf("%s survived phase 2b", sig)
		}
	}
}

// TestPayuniPhase2bDispatchStripeCapturesPayuniRefused (P2B03) runs on the fully migrated fixture schema.
func TestPayuniPhase2bDispatchStripeCapturesPayuniRefused(t *testing.T) {
	ctx := context.Background()
	t.Run("stripe capture and replay settle", func(t *testing.T) {
		e, attempt, _ := sucOrder(t, true)
		hash := e.observe(t, attempt, e.sessionReport(t, attempt, "complete", "paid", "succeeded", false))
		for i := 0; i < 2; i++ {
			if err := e.apply(attempt, hash); err != nil {
				t.Fatalf("Stripe capture/replay %d: %v", i, err)
			}
		}
		if n := countRows(t, e.f.owner, `SELECT count(*) FROM payments.facts WHERE attempt_id=$1 AND kind='CAPTURED'`, attempt); n != 1 {
			t.Fatalf("CAPTURED facts=%d", n)
		}
		order := e.p.hold.OrderID
		if n := countRows(t, e.f.owner, `SELECT count(*) FROM checkout.orders o JOIN inventory.reservations r ON r.id=o.id WHERE o.id=$1 AND o.commercial_state='CONFIRMED' AND r.state='COMMITTED'`, order); n != 1 {
			t.Fatal("Stripe capture did not confirm the order and commit its stock")
		}
		if n := countRows(t, e.f.owner, `SELECT count(*) FROM inventory.ledger WHERE checkout_id=$1 AND kind='ALLOCATE'`, order); n != 1 {
			t.Fatalf("ALLOCATE ledger=%d", n)
		}
		if n := countRows(t, e.f.owner, `SELECT count(*) FROM fulfillment.payment_work_items WHERE order_id=$1 AND state='READY'`, order); n != 1 {
			t.Fatalf("ready work=%d", n)
		}
	})
	t.Run("historical PAYUNi attempt refused and readable", func(t *testing.T) {
		p := psSetupItemsOn(t, pwIsolatedFixture(t), 1)
		result := rm2HistoricalAttempt(t, p)
		// A stored, internally consistent QUERY report: before 0197 the dispatcher would capture it.
		report := pcFull(pqFixture{psHarness: p, result: result})
		report["TradeNo"] = ""
		mustExec(t, p.f.owner, `INSERT INTO payments.provider_observations(tenant_id,store_id,attempt_id,source,execution_profile,environment,first_generation,report,report_hash)
			SELECT a.tenant_id,a.store_id,a.id,'QUERY',a.execution_profile,a.environment,a.generation,$2::jsonb,sha256(convert_to($2::jsonb::text,'UTF8'))
			FROM checkout.payment_attempts a WHERE a.id=$1`, result.AttemptID, pqJSON(report))
		var hash []byte
		if err := p.f.owner.QueryRow(ctx, `SELECT report_hash FROM payments.provider_observations WHERE attempt_id=$1`, result.AttemptID).Scan(&hash); err != nil {
			t.Fatal(err)
		}
		before := rm2Snapshot(t, p, result.AttemptID)
		for i := 0; i < 2; i++ {
			_, err := p.worker.Exec(ctx, `SELECT payments.apply_capture($1::uuid,$2::bytea)`, result.AttemptID, hash)
			var pe *pgconn.PgError
			if !errors.As(err, &pe) || pe.Code != "PT409" || pe.Message != "payment operation unavailable" {
				t.Fatalf("PAYUNi dispatch %d not refused by the dispatcher: %v", i, err)
			}
		}
		if after := rm2Snapshot(t, p, result.AttemptID); after != before {
			t.Fatalf("refused PAYUNi dispatch changed history: before=%s after=%s", before, after)
		}
		moGrant(t, p.f, p.f.tenantA, p.f.storeA1, p.f.principalA)
		rows, err := moRead(ctx, p.f.runtime, p.f.tokens["a"], p.f.tenantA, p.f.storeA1, p.f.principalA, p.hold.OrderID, "all", 1)
		if err != nil || len(rows) != 1 || rows[0]["payment_state"] != "PENDING" || rows[0]["commercial_state"] != "AWAITING_PAYMENT" {
			t.Fatalf("merchant history hidden or changed: %+v %v", rows, err)
		}
		keys, err := accounts.NewKeyring("p2b_test", map[string][]byte{"p2b_test": randomBytes(32)}, randomBytes(32))
		if err != nil {
			t.Fatal(err)
		}
		pool, err := platform.OpenHostedPool(ctx, hpRole(t, p.f))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(pool.Close)
		service := hpStarter(t, pool, "PROVIDER_MOCK", keys, checkout.HostedConfig{}).(*checkout.HostedPaymentStarter)
		view, err := service.PaymentView(ctx, p.cap.Token, p.f.storeA1, p.hold.OrderID)
		if err != nil || view.PaymentState != "PENDING" || view.CommercialState != "AWAITING_PAYMENT" || len(view.Methods) != 0 {
			t.Fatalf("buyer history hidden or changed: %+v %v", view, err)
		}
		if out, err := service.TakeHosted(ctx, p.cap.Token, p.f.storeA1, p.hold.OrderID); !errors.Is(err, command.ErrConflict) || out.Form != nil || out.RedirectURL != "" {
			t.Fatalf("history resumed: %+v %v", out, err)
		}
		if after := rm2Snapshot(t, p, result.AttemptID); after != before {
			t.Fatal("historical read changed state")
		}
	})
}
