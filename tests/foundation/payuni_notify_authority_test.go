// payuni_notify_authority_test.go: REAL_PG authority gate for the PAYUNi notify ingress pool
// (migration 0136). The notify ingress may EXECUTE only the two 0136 definers and hold no table,
// sequence, schema or database authority; every other role (runtime, workers, stripe ingress,
// registrar) must be unable to open the pool or execute the definers. The matrix mutates role graphs
// and PUBLIC ACLs, so it runs on pwIsolatedFixture, never on the shared fixture.
package foundation_test

import (
	"context"
	"net/http"
	"testing"

	"livecommerce/internal/platform"
)

const (
	paIngressRole = "commerce_payuni_ingress"
	paIngressErr  = "payuni ingress database unavailable"
	paRegistrarFn = "payments.set_payuni_notify_endpoint(uuid,uuid,uuid,uuid,uuid,text,boolean,bytea)"
)

func TestPayuniNotifyAuthorityIngressMatrix(t *testing.T) {
	f := pwIsolatedFixture(t)
	cases := saMatrix(t, f, paIngressRole, "commerce_payment_registrar", paRegistrarFn)
	saRunMatrix(t, f, paIngressRole, paIngressErr, platform.OpenPayuniIngressPool, platform.ValidatePayuniIngressPool, cases)
	saWrongRoleOnly(t, f, "commerce_payment_registrar", paIngressErr, platform.OpenPayuniIngressPool)
}

// A correctly provisioned ingress login opens the pool, resolves its endpoint and records a real
// receipt; the effective authority stays notify-definer-only.
func TestPayuniNotifyAuthorityIngressPositive(t *testing.T) {
	p := pnSetup(t)
	ctx := context.Background()
	var accountID, profile string
	if err := p.ingress.QueryRow(ctx, `SELECT account_id,execution_profile FROM payments.payuni_resolve_endpoint($1::bytea)`, p.tokenHash).Scan(&accountID, &profile); err != nil || accountID != "mock-account" || profile != "PROVIDER_MOCK" {
		t.Fatalf("ingress resolve account=%q profile=%q err=%v", accountID, profile, err)
	}
	body := pnValidBody(t, p)
	if rec := pnPost(t, p.handler, p.token, string(body)); rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Fatalf("correct ingress login could not record a real delivery: %d %q", rec.Code, rec.Body.String())
	}
	for name, q := range map[string]string{
		"attempts read":      `SELECT merchant_trade_no FROM checkout.payment_attempts LIMIT 1`,
		"credentials read":   `SELECT ciphertext FROM integration.account_credentials LIMIT 1`,
		"river_job read":     `SELECT id FROM river_payment.river_job LIMIT 1`,
		"river_job update":   `UPDATE river_payment.river_job SET scheduled_at=clock_timestamp() WHERE false`,
		"endpoint write":     `INSERT INTO payments.payuni_notify_endpoints(endpoint_id,tenant_id,store_id,connection_id,environment,account_id,execution_profile,enabled,token_hash) VALUES(gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),'SANDBOX','x','PROVIDER_MOCK',true,decode(repeat('00',32),'hex'))`,
		"registrar function": `SELECT payments.set_payuni_notify_endpoint(gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),'PROVIDER_MOCK',true,decode(repeat('00',32),'hex'))`,
	} {
		if _, err := p.ingress.Exec(ctx, q); sqlState(err) != "42501" {
			t.Errorf("ingress %s: want 42501 got %v", name, err)
		}
	}
}

// Existing pool modes must keep refusing a login that also holds the PAYUNi notify ingress authority,
// so notify custody cannot leak into an ordinary runtime or worker.
func TestPayuniNotifyAuthorityLegacyPoolsRejectPayuniIngress(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	for _, o := range []struct {
		name, role string
		open       saOpener
	}{
		{"OpenPool", "commerce_runtime", platform.OpenPool},
		{"OpenBuyerPool", "commerce_buyer_runtime", platform.OpenBuyerPool},
		{"OpenCheckoutPool", "commerce_checkout_runtime", platform.OpenCheckoutPool},
		{"OpenHostedPool", "commerce_hosted_runtime", platform.OpenHostedPool},
		{"OpenWorkerPool(payment)", waPayment, waOpener(platform.WorkerPayment)},
	} {
		o := o
		t.Run(o.name+" plus payuni_ingress", func(t *testing.T) {
			clean := saNewLogin(t, f, o.role)
			p, err := o.open(ctx, clean.dsn)
			if err != nil {
				t.Fatalf("clean %s login denied by %s: %v", o.role, o.name, err)
			}
			p.Close()
			mixed := saNewLogin(t, f, o.role, paIngressRole)
			p, err = o.open(ctx, mixed.dsn)
			if err == nil {
				p.Close()
				t.Fatalf("%s admitted a login that also holds %s", o.name, paIngressRole)
			}
		})
	}
}

// The two notify definers belong to the webhook ingress only: runtime, the six worker authorities,
// the checkout/hosted runtimes and the stripe ingress must not be able to execute them, and the
// ingress role must not be able to reach the registrar's endpoint function.
func TestPayuniNotifyAuthorityNoExecuteElsewhere(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	funcs := []string{
		"payments.payuni_resolve_endpoint(bytea)",
		"payments.payuni_record_notify(bytea,bytea,text,text,bigint,text,text)",
	}
	for _, role := range []string{"commerce_runtime", "commerce_stripe_ingress", waPayment, waLive, waExpiry, waAds, waClaims, waLegacy, "commerce_checkout_runtime", "commerce_hosted_runtime"} {
		for _, fn := range funcs {
			var exec bool
			if err := f.owner.QueryRow(ctx, `SELECT coalesce(has_function_privilege($1,$2::regprocedure,'EXECUTE'),false)`, role, fn).Scan(&exec); err != nil || exec {
				t.Errorf("%s may EXECUTE %s (err=%v)", role, fn, err)
			}
		}
	}
	var ingressRegistrar bool
	if err := f.owner.QueryRow(ctx, `SELECT coalesce(has_function_privilege('commerce_payuni_ingress','payments.set_payuni_notify_endpoint(uuid,uuid,uuid,uuid,uuid,text,boolean,bytea)','EXECUTE'),false)`).Scan(&ingressRegistrar); err != nil || ingressRegistrar {
		t.Fatalf("payuni ingress may set endpoints (err=%v)", err)
	}
}

// ACL pin: every SECURITY DEFINER function added by migration 0136 is a fixed-shape definer owned by
// a NOLOGIN writer role, runs on a frozen search_path and exposes no PUBLIC EXECUTE. The receipts table
// grants the definer owner exactly the two columns its redelivery bump may write (the guard trigger
// forbids every other change), and both new tables are forced-RLS with no PUBLIC table privilege.
func TestPayuniNotifySchemaACLPin(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	for _, d := range []struct{ sig, owner string }{
		{"payments.guard_payuni_notify_receipt()", "commerce_integration_writer"},
		{"payments.payuni_resolve_endpoint(bytea)", "commerce_integration_writer"},
		{"payments.payuni_record_notify(bytea,bytea,text,text,bigint,text,text)", "commerce_integration_writer"},
		{"payments.set_payuni_notify_endpoint(uuid,uuid,uuid,uuid,uuid,text,boolean,bytea)", "commerce_payment_registry_writer"},
	} {
		var ok bool
		if err := f.owner.QueryRow(ctx, `SELECT EXISTS(
			SELECT 1 FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner
			WHERE p.oid=to_regprocedure($1) AND p.prosecdef
			 AND p.proconfig @> ARRAY['search_path=pg_catalog']::text[]
			 AND NOT r.rolcanlogin AND pg_get_userbyid(p.proowner)=$2
			 AND NOT EXISTS (SELECT 1 FROM aclexplode(COALESCE(p.proacl,acldefault('f',p.proowner))) acl
			  WHERE acl.grantee=0 AND acl.privilege_type='EXECUTE'))`,
			d.sig, d.owner).Scan(&ok); err != nil || !ok {
			t.Fatalf("0136 definer %s is not a fixed-shape %s definer (err=%v)", d.sig, d.owner, err)
		}
	}
	// The redelivery bump writes exactly these two columns; no other receipts column may be writable.
	var twoCols bool
	if err := f.owner.QueryRow(ctx, `SELECT has_column_privilege('commerce_integration_writer','payments.payuni_notify_receipts','redelivery_count','UPDATE')
		AND has_column_privilege('commerce_integration_writer','payments.payuni_notify_receipts','last_redelivered_at','UPDATE')
		AND (SELECT count(*) FROM pg_attribute a WHERE a.attrelid='payments.payuni_notify_receipts'::regclass
			AND a.attnum>0 AND NOT a.attisdropped
			AND has_column_privilege('commerce_integration_writer',a.attrelid,a.attnum,'UPDATE'))=2`).Scan(&twoCols); err != nil || !twoCols {
		t.Fatalf("receipts UPDATE column grant is not exactly (redelivery_count,last_redelivered_at) (err=%v)", err)
	}
	// The wake writes exactly one new river column: the integration writer may UPDATE only (queue — the pre-existing
	// 0005 queue router — and scheduled_at) on the payment queue, and never INSERT/DELETE there.
	var wakeOnly bool
	if err := f.owner.QueryRow(ctx, `SELECT (SELECT array_agg(a.attname::text ORDER BY a.attname) FROM pg_attribute a
			WHERE a.attrelid='river_payment.river_job'::regclass AND a.attnum>0 AND NOT a.attisdropped
			AND has_column_privilege('commerce_integration_writer',a.attrelid,a.attnum,'UPDATE'))=ARRAY['queue','scheduled_at']
		AND NOT has_table_privilege('commerce_integration_writer','river_payment.river_job','INSERT,DELETE')`).Scan(&wakeOnly); err != nil || !wakeOnly {
		t.Fatalf("integration writer river_payment.river_job UPDATE is not exactly (queue,scheduled_at) or can INSERT/DELETE (err=%v)", err)
	}
	for _, table := range []string{"payments.payuni_notify_endpoints", "payments.payuni_notify_receipts"} {
		var forced bool
		if err := f.owner.QueryRow(ctx, `SELECT c.relrowsecurity AND c.relforcerowsecurity
			AND NOT EXISTS (SELECT 1 FROM aclexplode(COALESCE(c.relacl,acldefault('r',c.relowner))) acl
			 WHERE acl.grantee=0 AND acl.privilege_type IN ('SELECT','INSERT','UPDATE','DELETE'))
			FROM pg_class c WHERE c.oid=to_regclass($1)`, table).Scan(&forced); err != nil || !forced {
			t.Fatalf("%s is not forced-RLS with no PUBLIC table privilege (err=%v)", table, err)
		}
	}
}
