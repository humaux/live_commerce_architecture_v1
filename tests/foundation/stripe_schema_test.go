// stripe_schema_test.go owns independent REAL_PG checks for the frozen Stripe schema.
// It never substitutes catalog text for a provider or browser acceptance result.
// Depends on: foundation's isolated PG fixture, migrations, and Stripe's UNIT amount rule.
// Used by: SP02, SP06 and SP19 focused gates.
package foundation_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/integrations/psp/stripe"
)

func stripeCatalogBool(t *testing.T, p *pgxpool.Pool, q string, args ...any) bool {
	t.Helper()
	var got bool
	if err := p.QueryRow(context.Background(), q, args...).Scan(&got); err != nil {
		t.Fatal(err)
	}
	return got
}

func stripeMustCatalog(t *testing.T, p *pgxpool.Pool, label, q string, args ...any) {
	t.Helper()
	if !stripeCatalogBool(t, p, q, args...) {
		t.Fatalf("SP06 missing schema requirement: %s", label)
	}
}

func stripeCheckDef(t *testing.T, p *pgxpool.Pool, table string, fragments ...string) {
	t.Helper()
	var defs string
	err := p.QueryRow(context.Background(), `SELECT coalesce(string_agg(pg_get_constraintdef(oid),' | '),'')
	 FROM pg_constraint WHERE conrelid=to_regclass($1) AND contype='c'`, table).Scan(&defs)
	if err != nil {
		t.Fatal(err)
	}
	defs = strings.ToLower(defs)
	for _, frag := range fragments {
		if !strings.Contains(defs, strings.ToLower(frag)) {
			t.Errorf("SP06 %s CHECK missing %q", table, frag)
		}
	}
}

func stripeSQLState(err error, code string) bool {
	var e *pgconn.PgError
	return errors.As(err, &e) && e.Code == code
}

func stripeExpectCheck(t *testing.T, p *pgxpool.Pool, table, columns, values string) {
	t.Helper()
	ctx := context.Background()
	tx, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	// LIKE copies NOT NULL and CHECK, but not foreign keys or row triggers. This
	// isolates a negative CHECK example from unrelated fixture dependencies.
	if _, err = tx.Exec(ctx, `CREATE TEMP TABLE stripe_check_case (LIKE `+table+` INCLUDING CONSTRAINTS)`); err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO stripe_check_case (`+columns+`) VALUES (`+values+`)`)
	if !stripeSQLState(err, "23514") {
		t.Fatalf("SP06 %s negative CHECK accepted or failed for wrong reason: %v", table, err)
	}
}

// TestStripeSP02Currency is the SQL half of SP02. The §0.1 ruling overrides
// stale JPY/one-account draft rows: five currencies, no conversion or rounding.
func TestStripeSP02Currency(t *testing.T) {
	f := fixture(t)
	for _, v := range []struct {
		name, currency string
		amount         int64
		ok             bool
	}{
		{"HKD minimum", "HKD", 400, true}, {"HKD below", "HKD", 399, false},
		{"USD minimum", "USD", 50, true}, {"USD below", "USD", 49, false},
		{"SGD minimum", "SGD", 50, true}, {"SGD below", "SGD", 49, false},
		{"MYR minimum", "MYR", 200, true}, {"MYR below", "MYR", 199, false},
		{"TWD minimum", "TWD", 100, true}, {"TWD step", "TWD", 101, false},
		{"TWD maximum", "TWD", 99999900, true}, {"TWD overflow", "TWD", 100000000, false},
		{"USD maximum", "USD", 99999999, true}, {"USD overflow", "USD", 100000000, false},
		{"JPY denied", "JPY", 500, false}, {"ISK denied", "ISK", 500, false},
		{"UGX denied", "UGX", 500, false}, {"HUF denied", "HUF", 500, false},
		{"BHD denied", "BHD", 500, false}, {"lowercase denied", "usd", 500, false},
		{"zero denied", "USD", 0, false}, {"negative denied", "USD", -1, false},
	} {
		t.Run(v.name, func(t *testing.T) {
			// I05: SQL admission and Go wire encoding must agree exactly on the charge.
			unit, goErr := stripe.UnitAmount(v.currency, v.amount)
			var sqlOK bool
			var sqlUnit *int64
			err := f.owner.QueryRow(context.Background(), `SELECT payments.stripe_amount_ok($1,$2),payments.stripe_unit_amount($1,$2)`, v.currency, v.amount).Scan(&sqlOK, &sqlUnit)
			if err != nil {
				t.Fatal(err)
			}
			if (goErr == nil) != v.ok || sqlOK != v.ok {
				t.Fatalf("Go/SQL admission: go=%v sql=%v want=%v", goErr, sqlOK, v.ok)
			}
			if v.ok {
				if sqlUnit == nil || *sqlUnit != unit || unit != v.amount {
					t.Fatalf("unit amount: Go=%d SQL=%v", unit, sqlUnit)
				}
			} else if sqlUnit != nil {
				t.Fatalf("rejected amount converted to %d", *sqlUnit)
			}
		})
	}
}

// TestStripeSP06Schema checks the migration's structural and privilege gates.
// The fixture applies migrations twice, catching repeat/checksum drift.
func TestStripeSP06Schema(t *testing.T) {
	f := fixture(t)
	p := f.owner
	for _, v := range []string{"0061_stripe_psp.sql", "post_river/0012_stripe_payment.sql"} {
		stripeMustCatalog(t, p, "migration "+v, `SELECT EXISTS(SELECT 1 FROM public.lc_schema_migrations WHERE version=$1 AND checksum<>'')`, v)
	}
	for _, table := range []string{"payments.stripe_sessions", "payments.stripe_webhook_receipts", "payments.stripe_signals", "payments.stripe_webhook_endpoints"} {
		stripeMustCatalog(t, p, table+" FORCE RLS", `SELECT coalesce((SELECT relrowsecurity AND relforcerowsecurity FROM pg_class WHERE oid=to_regclass($1)),false)`, table)
		for _, role := range []string{"PUBLIC", "commerce_runtime", "commerce_worker"} {
			stripeMustCatalog(t, p, table+" direct privilege refused to "+role, `SELECT NOT has_table_privilege($1,$2,'SELECT') AND NOT has_table_privilege($1,$2,'INSERT') AND NOT has_table_privilege($1,$2,'UPDATE')`, role, table)
		}
	}
	stripeCheckDef(t, p, "integration.merchant_accounts", "stripe", "acct_")
	stripeMustCatalog(t, p, "PAYUNi credential ciphertext remains non-null", `SELECT bool_and(attnotnull) FROM pg_attribute WHERE attrelid='integration.account_credentials'::regclass AND attname IN ('nonce','ciphertext')`)
	stripeMustCatalog(t, p, "no obsolete ENV_PLATFORM custody", `SELECT NOT EXISTS(SELECT 1 FROM pg_attribute WHERE attrelid='integration.account_credentials'::regclass AND attname IN ('custody','key_fingerprint') AND NOT attisdropped)`)
	stripeCheckDef(t, p, "payments.method_versions", "stripe_checkout", "SGD", "MYR", "TWD")
	stripeCheckDef(t, p, "payments.account_qualifications", "stripe_checkout", "REAL_LIVE")
	stripeCheckDef(t, p, "checkout.payment_attempts", "stripe_checkout", "stripe_amount_ok")
	stripeCheckDef(t, p, "integration.operations", "BUYER_PAYMENT_QUERY", "stripe.checkout_session", "MEDIA_ATTEMPT")
	stripeCheckDef(t, p, "payments.provider_observations", "LOCAL", "QUERY", "stripe")
	stripeCheckDef(t, p, "payments.facts", "CLOSED_UNPAID", "amount_minor", "currency", "provider_reference")
	stripeCheckDef(t, p, "payments.review_cases", "PROVIDER_AMOUNT_MISMATCH", "CLOSURE_CONTRADICTED")
	stripeCheckDef(t, p, "inventory.ledger", "CLOSED_UNPAID", "SYSTEM_PAYMENT", "RELEASE")
	stripeCheckDef(t, p, "checkout.events", "checkout.payment_closed")
	stripeCheckDef(t, p, "payments.stripe_sessions", "40 minutes", "7 minutes", "5 minutes", "session_id", "create_body_sha256")
	stripeCheckDef(t, p, "payments.stripe_webhook_receipts", "MALFORMED", "ACCEPTED", "signal_id")
	stripeCheckDef(t, p, "payments.stripe_signals", "STRIPE_WEBHOOK", "BUYER_REFRESH", "BUYER_CANCEL", "consumed_at")
	stripeCheckDef(t, p, "payments.stripe_webhook_endpoints", "key_version", "nonce", "ciphertext")
	for _, v := range []struct{ table, cols, vals string }{
		{"integration.merchant_accounts", "id,tenant_id,store_id,principal_id,provider,environment,account_id,binding_id,credential_version", "gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),'stripe','SANDBOX','bad',gen_random_uuid(),1"},
		{"integration.account_credentials", "tenant_id,store_id,connection_id,version,key_id,principal_id,nonce,ciphertext", "gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),1,'test',gen_random_uuid(),decode('aa','hex'),decode(repeat('aa',17),'hex')"},
		{"payments.account_qualifications", "id,tenant_id,store_id,connection_id,credential_version,environment,code,proof_class,evidence_ref,observed_at,expires_at", "gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),1,'LIVE','stripe_checkout','REAL_LIVE','test',now(),now()+interval '1 hour'"},
		{"payments.facts", "tenant_id,store_id,attempt_id,kind,amount_minor,currency,provider_reference,connection_id,execution_profile,environment,source_report_hash", "gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),'CLOSED_UNPAID',1,'USD','cs_test_fake',gen_random_uuid(),'PROVIDER_MOCK','SANDBOX',decode(repeat('aa',32),'hex')"},
		{"payments.review_cases", "tenant_id,store_id,attempt_id,reason,source_report_hash", "gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),'UNLISTED',decode(repeat('aa',32),'hex')"},
	} {
		t.Run("negative/"+v.table, func(t *testing.T) { stripeExpectCheck(t, p, v.table, v.cols, v.vals) })
	}
	// The revised §0.1 index must allow separate accounts in one environment,
	// while prohibiting one account being bound to two stores.
	stripeMustCatalog(t, p, "one Stripe account per store/environment", `SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE schemaname='integration' AND tablename='merchant_accounts' AND indexdef ILIKE '%UNIQUE%' AND indexdef ILIKE '%(tenant_id, store_id, environment)%' AND indexdef ILIKE '%provider%stripe%')`)
	stripeMustCatalog(t, p, "one store per Stripe account/environment", `SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE schemaname='integration' AND tablename='merchant_accounts' AND indexdef ILIKE '%UNIQUE%' AND indexdef ILIKE '%(environment, account_id)%' AND indexdef ILIKE '%provider%stripe%')`)
	stripeMustCatalog(t, p, "merchant Stripe account policy restrictive", `SELECT EXISTS(SELECT 1 FROM pg_policy WHERE polrelid='integration.merchant_accounts'::regclass AND polpermissive=false AND (pg_get_expr(polwithcheck,polrelid) ILIKE '%payuni%' OR pg_get_expr(polqual,polrelid) ILIKE '%payuni%'))`)
	stripeMustCatalog(t, p, "merchant credential insert restricted to PAYUNi", `SELECT EXISTS(SELECT 1 FROM pg_policy WHERE polrelid='integration.account_credentials'::regclass AND polpermissive=false AND pg_get_expr(polwithcheck,polrelid) ILIKE '%payuni%')`)
	for _, c := range []string{"create_first_sent_at", "create_last_sent_at", "create_send_count", "create_body_sha256", "create_suppressed_at", "session_id", "session_url", "payment_intent_id", "pinned_at", "url_purged_at", "expire_calls", "last_expire_at"} {
		stripeMustCatalog(t, p, "integration writer stripe_sessions UPDATE("+c+")", `SELECT has_column_privilege('commerce_integration_writer','payments.stripe_sessions',$1,'UPDATE') AND NOT has_column_privilege('commerce_runtime','payments.stripe_sessions',$1,'UPDATE')`, c)
	}
	for _, c := range []string{"first_handed_out_at", "cancel_requested_at", "refresh_count", "last_refresh_at", "signal_count"} {
		stripeMustCatalog(t, p, "checkout writer stripe_sessions UPDATE("+c+")", `SELECT has_column_privilege('commerce_checkout_writer','payments.stripe_sessions',$1,'UPDATE') AND NOT has_column_privilege('commerce_runtime','payments.stripe_sessions',$1,'UPDATE')`, c)
	}
	for _, sig := range []string{"integration.load_stripe_session(uuid,bigint,bytea,text)", "integration.mark_stripe_create_sent(uuid,bigint,bytea,text,bytea)", "integration.note_stripe_expire(uuid,bigint,bytea,text)", "integration.record_stripe_observation(uuid,bigint,bytea,text,jsonb,bigint,text)", "integration.finish_stripe_query(uuid,bigint,bytea,text,text)", "integration.consume_stripe_signal(uuid,uuid,bigint,bytea,text,text)", "payments.apply_capture(uuid,bytea)", "payments.apply_capture_payuni_v1(uuid,bytea)", "payments.apply_stripe_observation(uuid,bytea)"} {
		stripeMustCatalog(t, p, "definer/ACL "+sig, `SELECT EXISTS(SELECT 1 FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid=to_regprocedure($1) AND p.prosecdef AND p.proconfig @> ARRAY['search_path=pg_catalog']::text[] AND NOT r.rolcanlogin AND NOT has_function_privilege('PUBLIC',p.oid,'EXECUTE'))`, sig)
	}
	for _, name := range []string{"guard_payment_job_family", "payment_job_queue", "route_payment_queue_v1", "payment_queue_ready", "reject_legacy_family_job"} {
		stripeMustCatalog(t, p, "post-River "+name+" signal family", `SELECT EXISTS(SELECT 1 FROM pg_proc WHERE pronamespace='integration'::regnamespace AND proname=$1 AND pg_get_functiondef(oid) LIKE '%payment_signal_v1%')`, name)
	}
	stripeMustCatalog(t, p, "payment queue readiness", `SELECT integration.payment_queue_ready()`)
	// Body identity, not merely a renamed symbol: the 0018 PAYUNi function must
	// retain its original normalized PL/pgSQL body after dispatch is installed.
	var oldBody, newBody string
	if err := p.QueryRow(context.Background(), `SELECT prosrc FROM pg_proc WHERE oid='payments.apply_capture_payuni_v1(uuid,bytea)'::regprocedure`).Scan(&oldBody); err != nil {
		t.Fatal(err)
	}
	if err := p.QueryRow(context.Background(), `SELECT prosrc FROM pg_proc WHERE oid='payments.apply_capture(uuid,bytea)'::regprocedure`).Scan(&newBody); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("../../migrations/0018_payment_capture.sql")
	if err != nil {
		t.Fatal(err)
	}
	begin := strings.Index(string(source), "CREATE FUNCTION payments.apply_capture(")
	if begin < 0 {
		t.Fatal("0018 PAYUNi source body absent")
	}
	fragment := string(source)[begin:]
	start := strings.Index(fragment, "AS $$")
	if start < 0 {
		t.Fatal("0018 PAYUNi source body opener absent")
	}
	fragment = fragment[start+len("AS $$"):]
	end := strings.Index(fragment, "$$;")
	if end < 0 {
		t.Fatal("0018 PAYUNi source body terminator absent")
	}
	wantHash, gotHash := sha256.Sum256([]byte(fragment[:end])), sha256.Sum256([]byte(oldBody))
	if oldBody == "" || wantHash != gotHash || oldBody == newBody || !strings.Contains(newBody, "apply_capture_payuni_v1") {
		t.Fatal("PAYUNi body was not preserved behind Stripe dispatcher")
	}
}

func TestStripeSP19LiveRefusal(t *testing.T) {
	f := fixture(t)
	// SQL tier only: configuration and CLI refusal are separate UNIT gates.
	stripeExpectCheck(t, f.owner, "payments.account_qualifications",
		"id,tenant_id,store_id,connection_id,credential_version,environment,code,proof_class,evidence_ref,observed_at,expires_at",
		"gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),1,'LIVE','stripe_checkout','REAL_LIVE','test',now(),now()+interval '1 hour'")
}
