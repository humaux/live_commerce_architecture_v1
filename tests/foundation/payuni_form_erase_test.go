// Purpose: PAY-RM2 phase-2c gates for migrations/0198 (owner decision 2026-10-19 "PAYUNi 表单清除": erase, not
//
//	archive, the stored PAYUNi hosted FORM material, checkout.hosted_payment_pages.form).
//	PFE01: after 0198 no PAYUNi hosted page keeps form material (every one holds the {"erased":"0198"} marker) and
//	       every other column of the page is unchanged; a second Apply is a no-op.
//	PFE02: a Stripe-attempt hosted-page row and its attempt are byte-identical before and after 0198.
//	PFE03: the historical PAYUNi order stays readable through the merchant projection and the buyer payment view.
//	PFE04: 0198 refuses (22023, nothing erased, nothing recorded) while a PAYUNi order is unreconciled.
//
// Depends on: migrations.Apply; pwIsolatedFixture; psSetupItemsOn; rm2HistoricalAttempt (payuni_removal_test.go);
//
//	sucOrder (stripe_untracked_capture_test.go); moGrant/moRead; hpRole/hpStarter; platform.OpenHostedPool.
//
// Used by: CI full foundation suite; scripts/dev/test-focused.sh '^TestPayuniFormErase'.
// Invariants: synthetic data only (no PII, no credentials, no provider call). Real Stripe code never writes
// hosted_payment_pages (stripe_start_test pins 0 rows), so PFE02's Stripe page is a synthetic negative control that
// proves 0198 is scoped by provider, not by table. Historical PAYUNi credential versions are out of scope.
package foundation_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/checkout"
	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/platform"
	"livecommerce/migrations"
)

const pfeVersion = "0198_payuni_form_material_erase.sql"

// pfeForm is the shape 0025 save_hosted_page stored; values are synthetic.
const pfeForm = `{"action":"https://sandbox-api.payuni.com.tw/api/upp","fields":{"MerID":"mock-account","Version":"2.0","EncryptInfo":"0000000000000000","HashInfo":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}}`

func pfeRequire(t *testing.T) {
	t.Helper()
	if v := rmVersions("0198", "zzz"); len(v) == 0 || v[0] != pfeVersion {
		t.Fatalf("migrations/%s must exist (got %v)", pfeVersion, v)
	}
}

// pfeSeedPage writes one hosted page with form material for an existing attempt (owner pool, like 0025's writer).
func pfeSeedPage(t *testing.T, pool *pgxpool.Pool, attempt, form string) {
	t.Helper()
	mustExec(t, pool, `INSERT INTO checkout.hosted_payment_pages(attempt_id,locale,config_digest,prepared_at,expires_at,form)
	 SELECT a.id,'en',decode(repeat('cd',32),'hex'),a.created_at,a.created_at+interval '30 seconds',$2::jsonb
	 FROM checkout.payment_attempts a WHERE a.id=$1`, attempt, form)
}

// pfeReplica runs statements on one connection with user triggers off (FK/CHECK constraints stay on).
func pfeReplica(t *testing.T, pool *pgxpool.Pool, stmts ...string) {
	t.Helper()
	ctx := context.Background()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	for _, q := range append(append([]string{`SET session_replication_role='replica'`}, stmts...), `RESET session_replication_role`) {
		if _, err := conn.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
}

func pfeRow(t *testing.T, pool *pgxpool.Pool, query string, args ...any) string {
	t.Helper()
	var out string
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

// pfeReapply releases 0198 from the ledger and applies the chain again.
func pfeReapply(t *testing.T, pool *pgxpool.Pool) error {
	t.Helper()
	mustExec(t, pool, `DELETE FROM public.lc_schema_migrations WHERE version=$1`, pfeVersion)
	return migrations.Apply(context.Background(), pool)
}

// TestPayuniFormEraseErasesOnlyPayuni covers PFE01, PFE02 and PFE03 on one database.
func TestPayuniFormEraseErasesOnlyPayuni(t *testing.T) {
	pfeRequire(t)
	ctx := context.Background()
	f := pwIsolatedFixture(t)

	// Stripe negative control.
	_, stripeAttempt, _ := sucOrder(t, true, f)
	pfeSeedPage(t, f.owner, stripeAttempt, `{"action":"https://checkout.stripe.invalid/synthetic","fields":{"session":"cs_test_synthetic"}}`)

	// Historical PAYUNi order, reconciled (operation terminal, order cancelled) so the guard passes.
	p := psSetupItemsOn(t, f, 1)
	result := rm2HistoricalAttempt(t, p)
	pfeSeedPage(t, f.owner, result.AttemptID, pfeForm)
	pfeReplica(t, f.owner,
		`UPDATE integration.operations SET state='FAILED_FINAL' WHERE id='`+result.OperationID+`'`,
		`UPDATE checkout.orders SET commercial_state='CANCELLED' WHERE id='`+p.hold.OrderID+`'`)

	const pageKeep = `SELECT (to_jsonb(h)-'form')::text FROM checkout.hosted_payment_pages h WHERE attempt_id=$1`
	const stripeBytes = `SELECT (SELECT h::text FROM checkout.hosted_payment_pages h WHERE attempt_id=$1)||'|'||(SELECT a::text FROM checkout.payment_attempts a WHERE a.id=$1)`
	if n := countRows(t, f.owner, `SELECT count(*) FROM checkout.hosted_payment_pages WHERE attempt_id=$1 AND form=$2::jsonb`, result.AttemptID, pfeForm); n != 1 {
		t.Fatalf("PAYUNi form seed visible as %d rows", n)
	}
	payuniKeep := pfeRow(t, f.owner, pageKeep, result.AttemptID)
	stripeBefore := pfeRow(t, f.owner, stripeBytes, stripeAttempt)

	for i := 0; i < 2; i++ { // the second run must be a no-op
		if err := pfeReapply(t, f.owner); err != nil {
			t.Fatalf("0198 apply %d: %v", i+1, err)
		}
	}

	// PFE01
	if n := countRows(t, f.owner, `SELECT count(*) FROM checkout.hosted_payment_pages h JOIN checkout.payment_attempts a ON a.id=h.attempt_id
		JOIN integration.merchant_accounts c ON c.tenant_id=a.tenant_id AND c.store_id=a.store_id AND c.id=a.connection_id
		WHERE c.provider='payuni' AND (h.form IS NULL OR h.form<>'{"erased":"0198"}'::jsonb)`); n != 0 {
		t.Errorf("PFE01: %d PAYUNi hosted page(s) still hold form material", n)
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM checkout.hosted_payment_pages WHERE attempt_id=$1 AND form='{"erased":"0198"}'::jsonb`, result.AttemptID); n != 1 {
		t.Errorf("PFE01: seeded PAYUNi page not redacted (%d)", n)
	}
	if got := pfeRow(t, f.owner, pageKeep, result.AttemptID); got != payuniKeep {
		t.Errorf("PFE01: 0198 changed non-form columns:\n got=%s\nwant=%s", got, payuniKeep)
	}
	// PFE02
	if got := pfeRow(t, f.owner, stripeBytes, stripeAttempt); got != stripeBefore {
		t.Errorf("PFE02: Stripe hosted page/attempt changed:\n got=%s\nwant=%s", got, stripeBefore)
	}

	// PFE03: merchant and buyer reads of the historical PAYUNi order.
	moGrant(t, p.f, p.f.tenantA, p.f.storeA1, p.f.principalA)
	rows, err := moRead(ctx, p.f.runtime, p.f.tokens["a"], p.f.tenantA, p.f.storeA1, p.f.principalA, p.hold.OrderID, "all", 1)
	if err != nil || len(rows) != 1 || rows[0]["commercial_state"] != "CANCELLED" {
		t.Fatalf("PFE03: merchant history unreadable after erase: %+v %v", rows, err)
	}
	keys, err := accounts.NewKeyring("pfe_test", map[string][]byte{"pfe_test": randomBytes(32)}, randomBytes(32))
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
	if err != nil || view.CommercialState != "CANCELLED" || len(view.Methods) != 0 {
		t.Fatalf("PFE03: buyer history unreadable after erase: %+v %v", view, err)
	}
	if out, _ := service.TakeHosted(ctx, p.cap.Token, p.f.storeA1, p.hold.OrderID); out.Form != nil || out.RedirectURL != "" {
		t.Fatalf("PFE03: erased form handed out: %+v", out)
	}
}

// TestPayuniFormEraseRefusesUnreconciledPayuni (PFE04): an unreconciled PAYUNi order blocks 0198 with 22023; the form
// stays and the ledger row is not recorded.
func TestPayuniFormEraseRefusesUnreconciledPayuni(t *testing.T) {
	pfeRequire(t)
	f := pwIsolatedFixture(t)
	p := psSetupItemsOn(t, f, 1)
	result := rm2HistoricalAttempt(t, p) // operation non-terminal, order AWAITING_PAYMENT
	pfeSeedPage(t, f.owner, result.AttemptID, pfeForm)
	err := pfeReapply(t, f.owner)
	if err == nil || !strings.Contains(err.Error(), "PAYUNi form erase refused") || !strings.Contains(err.Error(), "22023") {
		t.Fatalf("0198 over an unreconciled PAYUNi order: %v", err)
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM checkout.hosted_payment_pages WHERE attempt_id=$1 AND form=$2::jsonb`, result.AttemptID, pfeForm); n != 1 {
		t.Errorf("refused 0198 changed the form (%d)", n)
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM public.lc_schema_migrations WHERE version=$1`, pfeVersion); n != 0 {
		t.Errorf("refused 0198 recorded %d ledger row(s)", n)
	}
}
