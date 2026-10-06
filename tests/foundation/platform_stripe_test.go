package foundation_test

// W4-S1 platform Stripe (contracts/stripe-platform-account-v1.md §10, gates PF01-PF08 author smoke + the owner amendment AD-PF2
// allowlist). Tier: MOCK + REAL_PG (an isolated PG per gate), the SAME assembly functions the binaries call, the independent
// stripetest fake, a real payment worker and the real webhook handler. LIVE branches (platform approval, canary, live-revoke
// fan-out, LIVE currency) are NOT_RUN here: they need the owner's keys.
//
// Fixtures written through the migration-owner pool are named at their call site (grants, a bulk-enrollment fan-out bound check,
// the schema negative inserts). Every account, credential, qualification and method row of the platform store goes through
// stripeadmin; every merchant enable goes through internal/payments/platformstripe or the HTTP route.

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/command"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/payments/platformstripe"
	"livecommerce/internal/payments/stripeadmin"
	"livecommerce/internal/platform"
)

const pfTerms = "pf-2026-10"

// pfEnv is one isolated database: a platform store (own primary Stripe connection, PROVIDER_MOCK qualified), a platform webhook
// endpoint, a running payment worker and the merchant HTTP handler with the payments/card routes of a PROVIDER_MOCK deployment.
type pfEnv struct {
	*rfxEnv
	plat             sstStore
	op               stripeadmin.Scope
	endpoint, secret string
	version          int64 // payments.stripe_platform.version after the last operator call
	card             http.Handler
}

func pfNew(t *testing.T) *pfEnv {
	t.Helper()
	e := rfxNew(t)
	plat := e.stripeStore(t)
	endpoint, secret := e.endpoint(t, plat)
	pf := &pfEnv{rfxEnv: e, plat: plat, op: plat.scope, endpoint: endpoint, secret: secret,
		card: httpapi.NewHandler(e.f.runtime, httpapi.Options{PaymentProfile: "PROVIDER_MOCK"})}
	e.startWorker(t)
	return pf
}

func (pf *pfEnv) designate(t *testing.T) {
	t.Helper()
	v, err := pf.reg.PlatformDesignate(context.Background(), pf.op, stripeadmin.PlatformDesignateInput{ConnectionID: pf.plat.connection,
		DisplayName: "Platform Test", DescriptorDisplay: "LCPLATFORM", TermsVersion: pfTerms})
	if err != nil || v != 1 {
		t.Fatalf("designate: v=%d err=%v", v, err)
	}
	pf.version = v
}

func (pf *pfEnv) setOpen(t *testing.T, open bool) {
	t.Helper()
	v, err := pf.reg.PlatformOpen(context.Background(), pf.op, "SANDBOX", open, -1, pf.version)
	if err != nil {
		t.Fatalf("platform open=%v: %v", open, err)
	}
	pf.version = v
}

// merchant creates a separate-tenant store with billing:manage / refund / order grants (owner-pool fixture); allow allowlists it.
func (pf *pfEnv) merchant(t *testing.T, allow bool) psHarness {
	t.Helper()
	m := psSetupItemsOn(t, pf.f, 1)
	for _, perm := range []string{"billing:manage", "payments:refund", "orders:read"} {
		mustExec(t, pf.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`,
			m.f.tenantA, m.f.storeA1, m.f.principalA, perm)
	}
	if allow {
		pf.allow(t, m, true)
	}
	return m
}

func (pf *pfEnv) allow(t *testing.T, m psHarness, allowed bool) {
	t.Helper()
	if _, err := pf.reg.PlatformAllow(context.Background(), pf.op, m.f.tenantA, m.f.storeA1, "SANDBOX", allowed, "op@test", "tk-"+t04Tag()); err != nil {
		t.Fatalf("allow=%v: %v", allowed, err)
	}
}

func (pf *pfEnv) block(t *testing.T, m psHarness, blocked bool) {
	t.Helper()
	if _, err := pf.reg.PlatformBlock(context.Background(), pf.op, m.f.tenantA, m.f.storeA1, "SANDBOX", blocked, "op@test", "tk-"+t04Tag()); err != nil {
		t.Fatalf("block=%v: %v", blocked, err)
	}
}

func pfInput(enabled bool, expected int64, suffix string) platformstripe.Input {
	in := platformstripe.Input{Enabled: enabled, TermsVersion: pfTerms, ExpectedVersion: expected}
	if suffix != "" {
		in.DescriptorSuffix = &suffix
	}
	return in
}

func (pf *pfEnv) set(m psHarness, in platformstripe.Input) (platformstripe.Result, error) {
	ctx := context.Background()
	var out platformstripe.Result
	err := platform.WithScope(ctx, pf.f.runtime, m.f.tokens["a"], m.f.storeA1, "billing:manage", func(tx pgx.Tx, s platform.Scope) error {
		var e error
		out, e = platformstripe.Set(ctx, tx, s, m.f.tokens["a"], "PROVIDER_MOCK", in)
		return e
	})
	return out, err
}

func (pf *pfEnv) read(t *testing.T, m psHarness) platformstripe.Summary {
	t.Helper()
	ctx := context.Background()
	var out platformstripe.Summary
	err := platform.WithScope(ctx, pf.f.runtime, m.f.tokens["a"], m.f.storeA1, "integration:read", func(tx pgx.Tx, s platform.Scope) error {
		var e error
		out, e = platformstripe.Read(ctx, tx, s, m.f.tokens["a"], "PROVIDER_MOCK")
		return e
	})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return out
}

func pfCode(err error) string {
	var c *platformstripe.Error
	if errors.As(err, &c) {
		return c.Code
	}
	if err == nil {
		return ""
	}
	return "other:" + err.Error()
}

func (pf *pfEnv) mustSet(t *testing.T, m psHarness, in platformstripe.Input) platformstripe.Result {
	t.Helper()
	out, err := pf.set(m, in)
	if err != nil {
		t.Fatalf("set %+v: %v", in, err)
	}
	return out
}

// derived builds the sstStore of a merchant's DERIVED connection: the platform account and key, the store's own connection and head.
func (pf *pfEnv) derived(t *testing.T, m psHarness) sstStore {
	t.Helper()
	s := sstStore{p: m, scope: stripeadmin.Scope{TenantID: m.f.tenantA, StoreID: m.f.storeA1, PrincipalID: m.f.principalA},
		account: pf.plat.account, secret: pf.plat.secret}
	if err := pf.f.owner.QueryRow(context.Background(), `SELECT a.id::text,h.current_version,m.qualification_id::text FROM integration.merchant_accounts a
	 JOIN payments.method_heads h ON h.tenant_id=a.tenant_id AND h.store_id=a.store_id AND h.code='stripe_checkout'
	 JOIN payments.method_versions m ON m.tenant_id=h.tenant_id AND m.store_id=h.store_id AND m.market_id=h.market_id AND m.country=h.country
	  AND m.code=h.code AND m.version=h.current_version
	 WHERE a.tenant_id=$1 AND a.store_id=$2 AND a.platform_connection_id IS NOT NULL`, m.f.tenantA, m.f.storeA1).Scan(&s.connection, &s.method, &s.qualification); err != nil {
		t.Fatalf("derived rows of %s: %v", m.f.storeA1, err)
	}
	return s
}

func (pf *pfEnv) n(t *testing.T, q string, args ...any) int {
	return countRows(t, pf.f.owner, q, args...)
}

func pfSecretFree(t *testing.T, what string, v any) {
	t.Helper()
	raw := fmt.Sprint(v)
	for _, bad := range []string{"acct_", "sk_test", "rk_test", "ciphertext", "approval"} {
		if strings.Contains(raw, bad) {
			t.Fatalf("%s leaks %q: %s", what, bad, raw)
		}
	}
}

// ---------------------------------------------------------------------------------------------------------------------
// PF01 schema
// ---------------------------------------------------------------------------------------------------------------------

func TestPlatformStripePF01Schema(t *testing.T) {
	pf := pfNew(t)
	pf.designate(t)
	pf.setOpen(t, true)
	m := pf.merchant(t, true)
	other := pf.e2(t) // a second primary Stripe connection (another account)
	ctx := context.Background()
	owner := pf.f.owner

	derivedInsert := func(t *testing.T, platformConn, environment, account string) error {
		t.Helper()
		tx, err := owner.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		binding, conn := randomUUID(), randomUUID()
		// owner-pool fixture: raw derived rows that only the enable definer may create in production
		if _, err = tx.Exec(ctx, `INSERT INTO integration.bindings(id,tenant_id,store_id,principal_id,provider,external_asset_id) VALUES($1,$2,$3,$4,'stripe',$5)`,
			binding, m.f.tenantA, m.f.storeA1, m.f.principalA, environment+":"+account); err != nil {
			t.Fatal(err)
		}
		_, err = tx.Exec(ctx, `INSERT INTO integration.merchant_accounts(id,tenant_id,store_id,principal_id,provider,environment,account_id,binding_id,credential_version,platform_connection_id)
		 VALUES($1,$2,$3,$4,'stripe',$5,$6,$7,1,$8)`, conn, m.f.tenantA, m.f.storeA1, m.f.principalA, environment, account, binding, platformConn)
		return err
	}
	pgCode := func(err error) string {
		var pg interface{ SQLState() string }
		if errors.As(err, &pg) {
			return pg.SQLState()
		}
		return fmt.Sprint(err)
	}
	t.Run("derived account must mirror the designated platform connection", func(t *testing.T) {
		if err := derivedInsert(t, other.connection, "SANDBOX", other.account); pgCode(err) != "42501" {
			t.Fatalf("derived row on a non-designated connection: %v", err)
		}
		if err := derivedInsert(t, pf.plat.connection, "SANDBOX", "acct_Wrong"+t04Tag()); pgCode(err) != "42501" {
			t.Fatalf("derived row with another account: %v", err)
		}
		if err := derivedInsert(t, pf.plat.connection, "LIVE", pf.plat.account); pgCode(err) != "42501" {
			t.Fatalf("derived row with another environment: %v", err)
		}
		if err := derivedInsert(t, pf.plat.connection, "SANDBOX", pf.plat.account); err != nil {
			t.Fatalf("a correct derived row must insert: %v", err)
		}
	})
	t.Run("two primary rows on one account are still refused", func(t *testing.T) {
		tx, _ := owner.Begin(ctx)
		defer tx.Rollback(ctx)
		binding := randomUUID()
		if _, err := tx.Exec(ctx, `INSERT INTO integration.bindings(id,tenant_id,store_id,principal_id,provider,external_asset_id) VALUES($1,$2,$3,$4,'stripe',$5)`,
			binding, m.f.tenantA, m.f.storeA1, m.f.principalA, "SANDBOX:"+pf.plat.account); err != nil {
			t.Fatal(err)
		}
		_, err := tx.Exec(ctx, `INSERT INTO integration.merchant_accounts(id,tenant_id,store_id,principal_id,provider,environment,account_id,binding_id,credential_version)
		 VALUES($1,$2,$3,$4,'stripe','SANDBOX',$5,$6,1)`, randomUUID(), m.f.tenantA, m.f.storeA1, m.f.principalA, pf.plat.account, binding)
		if pgCode(err) != "23505" {
			t.Fatalf("second primary row on the platform account: %v", err)
		}
	})
	pf.mustSet(t, m, pfInput(true, 0, ""))
	d := pf.derived(t, m)
	t.Run("derived credential is a byte copy; pointer and bytes are guarded", func(t *testing.T) {
		var equal bool
		if err := owner.QueryRow(ctx, `SELECT d.sealed_version=p.version AND d.key_id=p.key_id AND d.nonce=p.nonce AND d.ciphertext=p.ciphertext
		 FROM integration.account_credentials d JOIN integration.account_credentials p ON p.connection_id=$2 AND p.version=1
		 WHERE d.connection_id=$1 AND d.version=1`, d.connection, pf.plat.connection).Scan(&equal); err != nil || !equal {
			t.Fatalf("derived credential is not the platform envelope: %v %v", equal, err)
		}
		_, err := owner.Exec(ctx, `INSERT INTO integration.account_credentials(tenant_id,store_id,connection_id,version,key_id,nonce,ciphertext,principal_id,sealed_version)
		 VALUES($1,$2,$3,2,'k',decode(repeat('00',12),'hex'),decode(repeat('00',17),'hex'),$4,1)`, m.f.tenantA, m.f.storeA1, d.connection, m.f.principalA)
		if pgCode(err) != "42501" {
			t.Fatalf("a derived credential that differs from the platform envelope: %v", err)
		}
		_, err = owner.Exec(ctx, `UPDATE integration.merchant_accounts SET platform_connection_id=NULL WHERE id=$1`, d.connection)
		if pgCode(err) != "42501" {
			t.Fatalf("the derived pointer is immutable: %v", err)
		}
	})
	t.Run("REAL_LIVE qualification needs an approval or a platform link", func(t *testing.T) {
		ins := func(link string) error {
			_, err := owner.Exec(ctx, `INSERT INTO payments.account_qualifications(id,tenant_id,store_id,connection_id,credential_version,environment,code,proof_class,evidence_ref,observed_at,expires_at,platform_qualification_id)
			 VALUES(gen_random_uuid(),$1,$2,$3,1,'LIVE','stripe_checkout','REAL_LIVE','x',clock_timestamp()-interval '1 second',clock_timestamp()+interval '1 day',NULLIF($4,'')::uuid)`,
				m.f.tenantA, m.f.storeA1, randomUUID(), link)
			return err
		}
		if c := pgCode(ins("")); c != "23514" {
			t.Fatalf("REAL_LIVE with neither link must fail the CHECK, got %s", c)
		}
		if c := pgCode(ins(randomUUID())); c != "23503" {
			t.Fatalf("with a platform link the CHECK passes and the FK decides, got %s", c)
		}
	})
	t.Run("runtime cannot set the derived pointer; the guard refuses an invisible account (P2-8)", func(t *testing.T) {
		var ok bool
		if err := owner.QueryRow(ctx, `SELECT has_column_privilege('commerce_runtime','integration.merchant_accounts','account_id','INSERT')
		 AND NOT has_column_privilege('commerce_runtime','integration.merchant_accounts','platform_connection_id','INSERT')`).Scan(&ok); err != nil || !ok {
			t.Fatalf("runtime INSERT column grant: %v %v", ok, err)
		}
	})
	t.Run("FORCE RLS and ACL matrix", func(t *testing.T) {
		for _, tbl := range []string{"payments.stripe_platform", "payments.platform_stripe_enrollments", "payments.platform_stripe_allowlist"} {
			var force bool
			var runtime, worker, ingress, checkout bool
			if err := owner.QueryRow(ctx, `SELECT c.relforcerowsecurity,
			 has_table_privilege('commerce_runtime',$1,'SELECT'),has_table_privilege('commerce_payment_worker',$1,'SELECT'),
			 has_table_privilege('commerce_stripe_ingress',$1,'SELECT'),has_table_privilege('commerce_integration_writer',$1,'SELECT')
			 FROM pg_class c WHERE c.oid=$1::regclass`, tbl).Scan(&force, &runtime, &worker, &ingress, &checkout); err != nil {
				t.Fatal(err)
			}
			if !force || runtime || worker || ingress || checkout {
				t.Fatalf("%s: force=%v runtime=%v worker=%v ingress=%v integration_writer=%v", tbl, force, runtime, worker, ingress, checkout)
			}
		}
		// the checkout writer reads display columns and the store's own suffix only, never an account id
		var ok bool
		if err := owner.QueryRow(ctx, `SELECT has_column_privilege('commerce_checkout_writer','payments.stripe_platform','display_name','SELECT')
		 AND NOT has_column_privilege('commerce_checkout_writer','payments.stripe_platform','account_id','SELECT')
		 AND NOT has_table_privilege('commerce_checkout_writer','payments.platform_stripe_allowlist','SELECT')`).Scan(&ok); err != nil || !ok {
			t.Fatalf("checkout writer column privileges: %v %v", ok, err)
		}
		for _, sig := range []string{"payments.platform_stripe_fanout(text,text)", "payments.platform_stripe_refresh_store(uuid,uuid,text,uuid,uuid,text)",
			"payments.platform_stripe_state(text)", "payments.platform_stripe_valid_qualification(text,text)", "payments.platform_stripe_summary(uuid,uuid,text)"} {
			var granted bool
			if err := owner.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_proc p CROSS JOIN LATERAL aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a
			 WHERE p.oid=$1::regprocedure AND a.grantee<>p.proowner)`, sig).Scan(&granted); err != nil || granted {
				t.Fatalf("internal helper %s must have no EXECUTE grant: %v %v", sig, granted, err)
			}
		}
	})
}

// derivedMore adds another buyer/order/hold to the store of m and returns its derived-connection sstStore.
func (pf *pfEnv) derivedMore(t *testing.T, m psHarness) sstStore {
	t.Helper()
	s := pf.derived(t, m)
	s.p = sstMoreHold(t, m)
	return s
}

// e2 seeds a second primary Stripe store (another account) in the same database.
func (pf *pfEnv) e2(t *testing.T) sstStore { t.Helper(); return pf.stripeStore(t) }

// ---------------------------------------------------------------------------------------------------------------------
// PF02 merchant enable (+ AD-PF2 allowlist)
// ---------------------------------------------------------------------------------------------------------------------

func TestPlatformStripePF02Enable(t *testing.T) {
	pf := pfNew(t)
	m := pf.merchant(t, false)
	if s := pf.read(t, m); s.PlatformState != "NONE" || s.StoreState != "NONE" || s.Allowed {
		t.Fatalf("read before designation: %+v", s)
	}
	// AD-PF2: a store the operator did not allowlist is refused first, whatever the platform state.
	if _, err := pf.set(m, pfInput(true, 0, "")); pfCode(err) != "platform_stripe_not_allowed" {
		t.Fatalf("not allowlisted: %v", err)
	}
	pf.designate(t)
	pf.allow(t, m, true)
	if s := pf.read(t, m); s.PlatformState != "CLOSED" || !s.Allowed || s.StoreState != "NONE" || s.TermsVersion == nil || *s.TermsVersion != pfTerms {
		t.Fatalf("read of a designated, closed platform: %+v", s)
	}
	if _, err := pf.set(m, pfInput(true, 0, "")); pfCode(err) != "platform_stripe_closed" {
		t.Fatalf("platform CLOSED must refuse enable: %v", err)
	}
	pf.setOpen(t, true)
	// no billing:manage -> forbidden (owner-pool grant fixture)
	mustExec(t, pf.f.owner, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND permission='billing:manage'`, m.f.tenantA, m.f.storeA1)
	if _, err := pf.set(m, pfInput(true, 0, "")); !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("without billing:manage: %v", err)
	}
	mustExec(t, pf.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'billing:manage')`, m.f.tenantA, m.f.storeA1, m.f.principalA)
	stale := pfInput(true, 0, "")
	stale.TermsVersion = "pf-old-1"
	for name, c := range map[string]struct {
		in   platformstripe.Input
		code string
	}{
		"stale terms":       {stale, "terms_version_stale"},
		"suffix too long":   {pfInput(true, 0, "ABCDEFGHIJK"), "descriptor_suffix_too_long"},
		"suffix characters": {pfInput(true, 0, "!!shop"), "invalid_descriptor_suffix"},
		"suffix no letter":  {pfInput(true, 0, "1234"), "invalid_descriptor_suffix"},
		"wrong version":     {pfInput(true, 5, ""), "version_changed"},
	} {
		if _, err := pf.set(m, c.in); pfCode(err) != c.code {
			t.Fatalf("%s: got %q want %q", name, pfCode(err), c.code)
		}
	}
	if pf.n(t, `SELECT count(*) FROM payments.platform_stripe_enrollments WHERE tenant_id=$1`, m.f.tenantA) != 0 {
		t.Fatal("a refused enable left an enrollment")
	}
	// a store that already holds its own primary Stripe connection cannot join the platform
	own := pf.merchant(t, true)
	pf.e2Seed(t, own)
	if _, err := pf.set(own, pfInput(true, 0, "")); pfCode(err) != "stripe_store_has_own_account" {
		t.Fatalf("own primary connection: %v", err)
	}

	// valid enable: binding, derived account, credential v1, qualification, method head, enrollment (one transaction)
	res := pf.mustSet(t, m, pfInput(true, 0, "SHOP A"))
	if res.State != "ENABLED" || res.Version != 1 || res.MaxMinor == nil || *res.MaxMinor != 99999900 || res.Currency == nil || *res.Currency != "TWD" ||
		res.DescriptorPreview == nil || *res.DescriptorPreview != "LCPLATFORM* SHOP A" {
		t.Fatalf("enable result: %+v", res)
	}
	pfSecretFree(t, "enable result", res)
	count := func() [6]int {
		var out [6]int
		_ = pf.f.owner.QueryRow(context.Background(), `SELECT
		 (SELECT count(*) FROM integration.bindings WHERE tenant_id=$1 AND store_id=$2 AND provider='stripe'),
		 (SELECT count(*) FROM integration.merchant_accounts WHERE tenant_id=$1 AND store_id=$2 AND platform_connection_id IS NOT NULL),
		 (SELECT count(*) FROM integration.account_credentials WHERE tenant_id=$1 AND store_id=$2 AND sealed_version IS NOT NULL),
		 (SELECT count(*) FROM payments.account_qualifications WHERE tenant_id=$1 AND store_id=$2 AND platform_qualification_id IS NOT NULL),
		 (SELECT count(*) FROM payments.method_heads WHERE tenant_id=$1 AND store_id=$2 AND code='stripe_checkout'),
		 (SELECT count(*) FROM payments.platform_stripe_enrollments WHERE tenant_id=$1 AND store_id=$2 AND descriptor_suffix='SHOP A')`,
			m.f.tenantA, m.f.storeA1).Scan(&out[0], &out[1], &out[2], &out[3], &out[4], &out[5])
		return out
	}
	if got := count(); got != [6]int{1, 1, 1, 1, 1, 1} {
		t.Fatalf("first enable must create exactly one of each row kind: %v", got)
	}
	var proof, ev string
	if err := pf.f.owner.QueryRow(context.Background(), `SELECT proof_class,evidence_ref FROM payments.account_qualifications WHERE tenant_id=$1 AND store_id=$2 AND platform_qualification_id IS NOT NULL`,
		m.f.tenantA, m.f.storeA1).Scan(&proof, &ev); err != nil || proof != "PROVIDER_MOCK" || ev != "platform:"+pf.plat.qualification {
		t.Fatalf("derived qualification mirrors the platform one: %s %s %v", proof, ev, err)
	}
	// replay of the stored state creates nothing and keeps the version
	if again := pf.mustSet(t, m, pfInput(true, 0, "SHOP A")); again.Version != 1 || again.State != "ENABLED" || count() != [6]int{1, 1, 1, 1, 1, 1} {
		t.Fatalf("replay: %+v %v", again, count())
	}
	sum := pf.read(t, m)
	pfSecretFree(t, "read", sum)
	if sum.StoreState != "ENABLED" || sum.PlatformState != "OPEN" || sum.AcceptedTermsVersion == nil || *sum.AcceptedTermsVersion != pfTerms || sum.Version != 1 {
		t.Fatalf("read after enable: %+v", sum)
	}
	// disable keeps every row; re-enable re-uses them
	if off := pf.mustSet(t, m, pfInput(false, 1, "")); off.State != "DISABLED" || off.Version != 2 || count() != [6]int{1, 1, 1, 1, 1, 1} {
		t.Fatalf("disable: %+v %v", off, count())
	}
	if on := pf.mustSet(t, m, pfInput(true, 2, "SHOP A")); on.State != "ENABLED" || on.Version != 3 || count() != [6]int{1, 1, 1, 1, 1, 1} {
		t.Fatalf("re-enable: %+v %v", on, count())
	}
	// disable is allowed while the platform is CLOSED and while the store is blocked; enable is not
	pf.setOpen(t, false)
	if _, err := pf.set(m, pfInput(false, 3, "")); err != nil {
		t.Fatalf("disable under CLOSED: %v", err)
	}
	if _, err := pf.set(m, pfInput(true, 4, "SHOP A")); pfCode(err) != "platform_stripe_closed" {
		t.Fatalf("re-enable under CLOSED: %v", err)
	}
	pf.setOpen(t, true)
	pf.block(t, m, true)
	if s := pf.read(t, m); s.StoreState != "BLOCKED" {
		t.Fatalf("blocked store state: %+v", s)
	}
	if _, err := pf.set(m, pfInput(true, 4, "SHOP A")); pfCode(err) != "platform_stripe_blocked" {
		t.Fatalf("enable while blocked: %v", err)
	}
	if _, err := pf.set(m, pfInput(false, 4, "")); err != nil {
		t.Fatalf("disable while blocked: %v", err)
	}
	// AD-PF2: withdrawing the allowlist refuses enable again, never disable
	pf.block(t, m, false)
	pf.allow(t, m, false)
	// P2-4 ruling: withdrawing the allowlist from an ENROLLED store also blocks it, so the blocked refusal answers first
	if _, err := pf.set(m, pfInput(true, 5, "SHOP A")); pfCode(err) != "platform_stripe_blocked" {
		t.Fatalf("enable after the allowlist was withdrawn: %v", err)
	}
	if _, err := pf.set(m, pfInput(false, 5, "")); err != nil {
		t.Fatalf("disable after the allowlist was withdrawn: %v", err)
	}
	// the operator audit rows exist (platform scope, target ids in details only)
	if pf.n(t, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action IN ('stripe.platform.allow','stripe.platform.disallow','stripe.platform.block','stripe.platform.unblock')`,
		pf.op.TenantID, pf.op.StoreID) < 4 {
		t.Fatal("operator allow/block audit rows missing in the platform scope")
	}
	_ = command.ErrConflict
}

// e2Seed gives the store of m its own primary Stripe connection (registered through the registrar).
func (pf *pfEnv) e2Seed(t *testing.T, m psHarness) sstStore { t.Helper(); return pf.seed(t, m) }

// ---------------------------------------------------------------------------------------------------------------------
// PF03 fan-out, derived refusals, the 2000 bound
// ---------------------------------------------------------------------------------------------------------------------

func TestPlatformStripePF03Fanout(t *testing.T) {
	pf := pfNew(t)
	pf.designate(t)
	pf.setOpen(t, true)
	a, b, c := pf.merchant(t, true), pf.merchant(t, true), pf.merchant(t, true)
	for _, m := range []psHarness{a, b, c} {
		pf.mustSet(t, m, pfInput(true, 0, ""))
	}
	pf.block(t, c, true) // c is skipped by every fan-out
	ctx := context.Background()
	head := func(m psHarness) (cred int64, sealed int64) {
		if err := pf.f.owner.QueryRow(ctx, `SELECT a.credential_version,c.sealed_version FROM integration.merchant_accounts a JOIN integration.account_credentials c
		 ON c.tenant_id=a.tenant_id AND c.store_id=a.store_id AND c.connection_id=a.id AND c.version=a.credential_version
		 WHERE a.tenant_id=$1 AND a.store_id=$2 AND a.platform_connection_id IS NOT NULL`, m.f.tenantA, m.f.storeA1).Scan(&cred, &sealed); err != nil {
			t.Fatal(err)
		}
		return
	}
	// rotate on the platform connection: new head for a and b in the same transaction, c untouched
	next := "sk_test_" + hex.EncodeToString(randomBytes(12))
	if err := pf.fake.AddAccount(pf.plat.account, next); err != nil {
		t.Fatal(err)
	}
	if v, err := pf.reg.Rotate(ctx, pf.plat.scope, pf.plat.connection, 1, pf.plat.account, next); err != nil || v != 2 {
		t.Fatalf("platform rotate: %d %v", v, err)
	}
	for name, m := range map[string]psHarness{"a": a, "b": b} {
		if cred, sealed := head(m); cred != 2 || sealed != 2 {
			t.Fatalf("store %s credential after rotate: version %d sealed %d", name, cred, sealed)
		}
	}
	// P2-5: a blocked store also gets the new credential head (its refunds keep needing the current key, §4.3) but nothing else
	if cred, sealed := head(c); cred != 2 || sealed != 2 {
		t.Fatalf("blocked store credential must follow the rotation: %d %d", cred, sealed)
	}
	// until the platform re-qualifies, the new derived head has no qualification: starts are blocked (existing rule)
	sa := pf.derived(t, a)
	if _, err := sa.begin(pf.svc, t04Key("pf03-pre"), sa.input("zh-TW")); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("start between rotate and re-qualify must be a 409: %v", err)
	}
	pf.plat.secret = next
	pf.plat.requalify(t, pf.sstEnv, 2)
	for _, m := range []psHarness{a, b} {
		if n := pf.n(t, `SELECT count(*) FROM payments.account_qualifications WHERE tenant_id=$1 AND store_id=$2 AND credential_version=2 AND platform_qualification_id=$3`, m.f.tenantA, m.f.storeA1, pf.plat.qualification); n != 1 {
			t.Fatalf("derived qualification for the new head: %d", n)
		}
		if n := pf.n(t, `SELECT count(*) FROM payments.method_heads h JOIN payments.method_versions v ON v.tenant_id=h.tenant_id AND v.store_id=h.store_id AND v.market_id=h.market_id
		 AND v.country=h.country AND v.code=h.code AND v.version=h.current_version WHERE h.tenant_id=$1 AND h.store_id=$2 AND v.enabled AND v.version=2`, m.f.tenantA, m.f.storeA1); n != 1 {
			t.Fatalf("method head CAS after qualify: %d", n)
		}
	}
	if n := pf.n(t, `SELECT count(*) FROM payments.account_qualifications WHERE tenant_id=$1 AND store_id=$2 AND credential_version=2`, c.f.tenantA, c.f.storeA1); n != 0 {
		t.Fatalf("blocked store got a qualification: %d", n)
	}
	// P2-6: a REAL_SANDBOX platform qualification is NOT mirrored into stores whose head runs PROVIDER_MOCK
	if _, err := pf.reg.Qualify(ctx, pf.plat.scope, stripeadmin.QualifyInput{ConnectionID: pf.plat.connection, AccountID: pf.plat.account,
		SecretKey: next, Profile: "SANDBOX", Currency: "TWD", ReturnURL: sstReturnURL, ExpectedVersion: 2, AmountMinor: 2500}); err != nil {
		t.Fatalf("platform SANDBOX qualify: %v", err)
	}
	if n := pf.n(t, `SELECT count(*) FROM payments.account_qualifications WHERE platform_qualification_id IS NOT NULL AND proof_class='REAL_SANDBOX'`); n != 0 {
		t.Fatalf("a head running PROVIDER_MOCK was re-armed with another proof class: %d", n)
	}
	if _, err := sa.begin(pf.svc, t04Key("pf03-post"), pf.derived(t, a).input("zh-TW")); err != nil {
		t.Fatalf("start after qualify: %v", err)
	}
	// a derived connection refuses rotate, qualify, webhook and method enabling (stripe_platform_derived)
	scope := stripeadmin.Scope{TenantID: a.f.tenantA, StoreID: a.f.storeA1, PrincipalID: a.f.principalA}
	derived := pf.derived(t, a)
	if _, err := pf.reg.Rotate(ctx, scope, derived.connection, 2, pf.plat.account, next); !errors.Is(err, stripeadmin.ErrRejected) {
		t.Fatalf("rotate on a derived connection: %v", err)
	}
	if _, err := pf.reg.Qualify(ctx, scope, stripeadmin.QualifyInput{ConnectionID: derived.connection, Profile: "PROVIDER_MOCK", ExpectedVersion: 2, Currency: "TWD", AmountMinor: 2500}); !errors.Is(err, stripeadmin.ErrRejected) {
		t.Fatalf("qualify on a derived connection: %v", err)
	}
	if _, _, err := pf.reg.SetWebhookEndpoint(ctx, scope, stripeadmin.EndpointInput{ConnectionID: derived.connection, Profile: "PROVIDER_MOCK", Enabled: true,
		Secrets: accounts.StripeWebhookSecrets{CurrentSecret: swhSecret()}}); !errors.Is(err, stripeadmin.ErrRejected) {
		t.Fatalf("webhook endpoint on a derived connection: %v", err)
	}
	if _, err := pf.reg.SetMethod(ctx, scope, derived.methodInput(derived.method, true, true, 2500, 99999900)); !errors.Is(err, stripeadmin.ErrRejected) {
		t.Fatalf("enabling a method on a derived connection: %v", err)
	}
	// the 2000 bound (owner-pool fixture: 2001 synthetic enrolled stores), refused as a whole and rolled back
	mustExec(t, pf.f.owner, fmt.Sprintf(`DO $$ DECLARE i int; t uuid; s uuid; p uuid; b uuid; c uuid; BEGIN
	 FOR i IN 1..2001 LOOP
	  t:=gen_random_uuid(); s:=gen_random_uuid(); p:=gen_random_uuid(); b:=gen_random_uuid(); c:=gen_random_uuid();
	  PERFORM set_config('app.tenant_id',t::text,true),set_config('app.store_id',s::text,true); -- the credential guard reads the account in scope
	  INSERT INTO control.tenants(id,name) VALUES(t,'bulk'); INSERT INTO control.stores(tenant_id,id,name,currency) VALUES(t,s,'bulk','TWD');
	  INSERT INTO identity.principals(id) VALUES(p); INSERT INTO identity.memberships(tenant_id,principal_id) VALUES(t,p);
	  INSERT INTO integration.bindings(id,tenant_id,store_id,principal_id,provider,external_asset_id) VALUES(b,t,s,p,'stripe','SANDBOX:'||'%[1]s'::text);
	  INSERT INTO integration.merchant_accounts(id,tenant_id,store_id,principal_id,provider,environment,account_id,binding_id,credential_version,platform_connection_id)
	   VALUES(c,t,s,p,'stripe','SANDBOX','%[1]s'::text,b,1,'%[2]s'::uuid);
	  INSERT INTO integration.account_credentials(tenant_id,store_id,connection_id,version,key_id,nonce,ciphertext,principal_id,sealed_version)
	   SELECT t,s,c,1,k.key_id,k.nonce,k.ciphertext,p,1 FROM integration.account_credentials k WHERE k.connection_id='%[2]s'::uuid AND k.version=1;
	  INSERT INTO payments.platform_stripe_enrollments(tenant_id,store_id,environment,connection_id,enrolled_by,terms_version,accepted_at)
	   VALUES(t,s,'SANDBOX',c,p,'pf-2026-10',clock_timestamp());
	 END LOOP; END $$`, pf.plat.account, pf.plat.connection))
	third := "sk_test_" + hex.EncodeToString(randomBytes(12))
	_ = pf.fake.AddAccount(pf.plat.account, third)
	if _, err := pf.reg.Rotate(ctx, pf.plat.scope, pf.plat.connection, 2, pf.plat.account, third); !errors.Is(err, stripeadmin.ErrRejected) {
		t.Fatalf("2001 enrollments: %v", err)
	}
	var v int64
	if err := pf.f.owner.QueryRow(ctx, `SELECT credential_version FROM integration.merchant_accounts WHERE id=$1`, pf.plat.connection).Scan(&v); err != nil || v != 2 {
		t.Fatalf("a refused fan-out must roll the rotate back: head %d %v", v, err)
	}
}

// ---------------------------------------------------------------------------------------------------------------------
// PF04 start + worker (aad_* envelope) + PF08 hosted collector
// ---------------------------------------------------------------------------------------------------------------------

func TestPlatformStripePF04Start(t *testing.T) {
	pf := pfNew(t)
	pf.designate(t)
	pf.setOpen(t, true)
	a, b := pf.merchant(t, true), pf.merchant(t, true)
	pf.mustSet(t, a, pfInput(true, 0, "SHOP A"))
	pf.mustSet(t, b, pfInput(true, 0, ""))
	sa, sb := pf.derived(t, a), pf.derived(t, b)
	ra, sessionA := pf.pinned(t, sa) // the worker opened the COPIED envelope with the aad_* scope and created the session
	rb, sessionB := pf.pinned(t, sb)
	rp, _ := pf.pinned(t, pf.plat) // a primary-store attempt is unchanged except for the lc_store keys
	params := func(attempt string) map[string]string {
		var raw []byte
		if err := pf.f.owner.QueryRow(context.Background(), `SELECT create_params FROM payments.stripe_sessions WHERE attempt_id=$1`, attempt).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		got := map[string]string{}
		for k, v := range jsonStringMap(t, raw) {
			got[k] = v
		}
		return got
	}
	for _, c := range []struct {
		attempt string
		s       sstStore
		suffix  string
	}{{ra.AttemptID, sa, "SHOP A"}, {rb.AttemptID, sb, ""}, {rp.AttemptID, pf.plat, ""}} {
		got := params(c.attempt)
		if got["metadata[lc_store]"] != c.s.p.f.storeA1 || got["payment_intent_data[metadata][lc_store]"] != c.s.p.f.storeA1 {
			t.Fatalf("store tag of %s: %v", c.attempt, got)
		}
		want := 24 // 22 frozen keys + the two lc_store keys
		if c.suffix != "" {
			want = 25
		}
		if got["payment_intent_data[statement_descriptor_suffix]"] != c.suffix || len(got) != want {
			t.Fatalf("descriptor suffix / key set of %s: suffix=%q keys=%d", c.attempt, got["payment_intent_data[statement_descriptor_suffix]"], len(got))
		}
	}
	// every attempt is in its own store's scope on its own connection; the derived ones carry the platform account
	for _, c := range []struct {
		attempt string
		s       sstStore
	}{{ra.AttemptID, sa}, {rb.AttemptID, sb}} {
		var tenant, store, conn string
		if err := pf.f.owner.QueryRow(context.Background(), `SELECT tenant_id::text,store_id::text,connection_id::text FROM checkout.payment_attempts WHERE id=$1`, c.attempt).Scan(&tenant, &store, &conn); err != nil ||
			tenant != c.s.p.f.tenantA || store != c.s.p.f.storeA1 || conn != c.s.connection {
			t.Fatalf("attempt scope: %s %s %s %v", tenant, store, conn, err)
		}
	}
	creates := map[string]string{}
	for _, r := range pf.fake.Requests() {
		if r.Method == http.MethodPost && r.Path == "/v1/checkout/sessions" {
			creates[strings.TrimPrefix(r.IdempotencyKey, "lc:stripe:cs-create:v1:")] = r.Account
		}
	}
	if creates[ra.AttemptID] != pf.plat.account || creates[rb.AttemptID] != pf.plat.account || sessionA == sessionB {
		t.Fatalf("derived creates must use the platform account and key: %v", creates)
	}
	if pf.fake.Counts().Denied != 0 {
		t.Fatal("the worker sent a wrong API key")
	}
	// PF08: hosted view collector for derived, absent (null) for primary; no id of any kind
	va, vp := pf.view(t, sa), pf.view(t, pf.plat)
	if va.Collector == nil || va.Collector.DisplayName != "Platform Test" || va.Collector.DescriptorPreview != "LCPLATFORM* SHOP A" || vp.Collector != nil {
		t.Fatalf("collector: derived=%+v primary=%+v", va.Collector, vp.Collector)
	}
	pfSecretFree(t, "hosted view", va)
}

func jsonStringMap(t *testing.T, raw []byte) map[string]string {
	t.Helper()
	out := map[string]string{}
	var any map[string]any
	if err := json.Unmarshal(raw, &any); err != nil {
		t.Fatal(err)
	}
	for k, v := range any {
		out[k] = fmt.Sprint(v)
	}
	return out
}

// ---------------------------------------------------------------------------------------------------------------------
// PF05 webhook mapping, PF06 refunds, PF07 kill switches
// ---------------------------------------------------------------------------------------------------------------------

func TestPlatformStripePF05Webhook(t *testing.T) {
	pf := pfNew(t)
	pf.designate(t)
	pf.setOpen(t, true)
	a, b := pf.merchant(t, true), pf.merchant(t, true)
	pf.mustSet(t, a, pfInput(true, 0, ""))
	pf.mustSet(t, b, pfInput(true, 0, ""))
	sa, sb := pf.derived(t, a), pf.derived(t, b)
	oa, ob := pf.pay(t, sa, pf.endpoint, pf.secret), pf.pay(t, sb, pf.endpoint, pf.secret) // ONE platform endpoint for both stores
	for _, o := range []rfxOrder{oa, ob} {
		var tenant, store string
		if err := pf.f.owner.QueryRow(context.Background(), `SELECT tenant_id::text,store_id::text FROM payments.stripe_webhook_receipts WHERE attempt_id=$1 AND disposition='ACCEPTED'`, o.attempt).Scan(&tenant, &store); err != nil ||
			tenant != o.s.p.f.tenantA || store != o.s.p.f.storeA1 {
			t.Fatalf("receipt scope of %s: %s %s %v", o.attempt, tenant, store, err)
		}
	}
	receipt := func(event string) (string, string) {
		var d, r string
		if err := pf.f.owner.QueryRow(context.Background(), `SELECT disposition,reason FROM payments.stripe_webhook_receipts WHERE endpoint_id=$1::uuid AND event_id=$2`, pf.endpoint, event).Scan(&d, &r); err != nil {
			t.Fatalf("receipt %s: %v", event, err)
		}
		return d, r
	}
	// a forged lc_store (B's store on A's session) never reaches a row
	a2 := pf.derivedMore(t, a)
	r2, s2 := pf.pinned(t, a2)
	forged := sflEvent(r2.AttemptID, s2)
	forged.ID, forged.Store = "evt_pf05_forged_"+t04Tag(), b.f.storeA1
	if status := pf.deliver(t, pf.endpoint, pf.secret, forged); status != 200 {
		t.Fatalf("forged lc_store answered %d", status)
	}
	if d, r := receipt(forged.ID); d != "QUARANTINED" || r != "reference_mismatch" {
		t.Fatalf("forged lc_store: %s %s", d, r)
	}
	// a session of no known attempt on the same endpoint
	unknown := sflEvent(randomUUID(), "cs_test_unknown_"+t04Tag()) // no known session and no known attempt reference
	unknown.ID = "evt_pf05_unknown_" + t04Tag()
	if status := pf.deliver(t, pf.endpoint, pf.secret, unknown); status != 200 {
		t.Fatalf("unknown session answered %d", status)
	}
	if d, r := receipt(unknown.ID); d != "QUARANTINED" || r != "unknown_session" {
		t.Fatalf("unknown session: %s %s", d, r)
	}
	// redelivery dedupes per (endpoint,event) across stores
	if status := pf.deliver(t, pf.endpoint, pf.secret, forged); status != 200 {
		t.Fatalf("redelivery answered %d", status)
	}
	if n := pf.n(t, `SELECT redelivery_count FROM payments.stripe_webhook_receipts WHERE event_id=$1`, forged.ID); n != 1 {
		t.Fatalf("redelivery_count %d", n)
	}
	// a correct lc_store is accepted
	good := sflEvent(r2.AttemptID, s2)
	good.ID, good.Store = "evt_pf05_good_"+t04Tag(), a.f.storeA1
	if status := pf.deliver(t, pf.endpoint, pf.secret, good); status != 200 {
		t.Fatalf("good lc_store answered %d", status)
	}
	if d, _ := receipt(good.ID); d != "ACCEPTED" {
		t.Fatalf("correct lc_store disposition %s", d)
	}
	// refund event on the platform endpoint maps to the refund's store
	pf.grant(t, oa, "orders:read", "payments:refund")
	refund := pf.mustRefund(t, oa, 800, "requested_by_customer")
	pf.awaitRefundFact(t, refund, oa.attempt, "SUCCEEDED") // the worker opened the derived envelope for the refund too
	id := srhEvent("refundcreated")
	if status, _ := pf.srhPost(t, pf.endpoint, pf.secret, pf.fake.RefundEventBody(id, "refund.created", pf.fake.RefundByRef(refund), false)); status != 200 {
		t.Fatalf("refund event answered %d", status)
	}
	var tenant, store string
	if err := pf.f.owner.QueryRow(context.Background(), `SELECT tenant_id::text,store_id::text FROM payments.stripe_webhook_receipts WHERE event_id=$1 AND refund_id=$2::uuid`, id, refund).Scan(&tenant, &store); err != nil ||
		tenant != a.f.tenantA || store != a.f.storeA1 {
		t.Fatalf("refund receipt scope: %s %s %v", tenant, store, err)
	}
}

func TestPlatformStripePF06Refund(t *testing.T) {
	pf := pfNew(t)
	pf.designate(t)
	pf.setOpen(t, true)
	a, b := pf.merchant(t, true), pf.merchant(t, true)
	pf.mustSet(t, a, pfInput(true, 0, ""))
	pf.mustSet(t, b, pfInput(true, 0, ""))
	oa, ob := pf.pay(t, pf.derived(t, a), pf.endpoint, pf.secret), pf.pay(t, pf.derived(t, b), pf.endpoint, pf.secret)
	pf.grant(t, oa, "orders:read", "payments:refund")
	pf.grant(t, ob, "orders:read", "payments:refund")
	// A's merchant targets B's order id inside A's own store: not found
	cross := oa
	cross.order = ob.order
	if status, _ := pf.request(cross, oa.token(), "pf06-"+t04Tag(), rfxBody(800, "requested_by_customer", oa.captured)); status != http.StatusNotFound {
		t.Fatalf("A refunding B's order: %d", status)
	}
	// over capacity (RD3 per attempt)
	if status, out := pf.request(oa, oa.token(), "pf06-"+t04Tag(), rfxBody(oa.captured+100, "requested_by_customer", oa.captured)); status != http.StatusUnprocessableEntity {
		t.Fatalf("over-capacity refund: %d %v", status, out)
	}
	first := pf.mustRefund(t, oa, 800, "requested_by_customer")
	pf.awaitRefundFact(t, first, oa.attempt, "SUCCEEDED")
	// refunds keep working after disable, block, platform close
	pf.mustSet(t, a, pfInput(false, 1, ""))
	pf.awaitRefundFact(t, pf.mustRefund(t, oa, 800, "requested_by_customer"), oa.attempt, "SUCCEEDED")
	pf.block(t, a, true)
	pf.awaitRefundFact(t, pf.mustRefund(t, oa, 800, "requested_by_customer"), oa.attempt, "SUCCEEDED")
	pf.setOpen(t, false)
	// the store's captured 2500 minus 3x800 leaves 100: one more 100 is accepted, 200 is not
	pf.mustRefund(t, oa, 100, "requested_by_customer")
	if status, _ := pf.request(oa, oa.token(), "pf06-"+t04Tag(), rfxBody(100, "requested_by_customer", 0)); status == http.StatusCreated {
		t.Fatal("refund beyond the store's own capture was accepted")
	}
}

func TestPlatformStripePF07Kill(t *testing.T) {
	pf := pfNew(t)
	pf.designate(t)
	pf.setOpen(t, true)
	a := pf.merchant(t, true)
	pf.mustSet(t, a, pfInput(true, 0, ""))
	sa := pf.derived(t, a)
	// in-flight attempt, then the operator blocks the store: new starts stop, the in-flight one still reaches CAPTURED
	res, session := pf.pinned(t, sa)
	pf.block(t, a, true)
	more := sa
	more.p = sstMoreHold(t, a)
	if _, err := more.begin(pf.svc, t04Key("pf07-blocked"), more.input("zh-TW")); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("new start after block: %v", err)
	}
	if !pf.fake.SetState(session, "complete", "paid") {
		t.Fatal("fake pay")
	}
	if status := pf.deliver(t, pf.endpoint, pf.secret, sflEvent(res.AttemptID, session)); status != 200 {
		t.Fatalf("webhook for the in-flight attempt: %d", status)
	}
	pf.awaitFact(t, res.AttemptID, "CAPTURED")
	// blocked: the merchant cannot re-enable; unblock alone does not re-enable; enable after unblock re-derives
	if _, err := pf.set(a, pfInput(true, 2, "")); pfCode(err) != "platform_stripe_blocked" {
		t.Fatalf("re-enable while blocked: %v", err)
	}
	pf.block(t, a, false)
	// the head now names the DISABLED version: starting on it is refused (unblock alone never re-enables)
	stillOff := pf.derived(t, a)
	stillOff.p = more.p
	if _, err := stillOff.begin(pf.svc, t04Key("pf07-unblocked-only"), stillOff.input("zh-TW")); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("unblock must not re-enable: %v", err)
	}
	before := pf.n(t, `SELECT count(*) FROM payments.account_qualifications WHERE tenant_id=$1 AND store_id=$2 AND platform_qualification_id IS NOT NULL`, a.f.tenantA, a.f.storeA1)
	cur := pf.read(t, a)
	pf.mustSet(t, a, pfInput(true, cur.Version, ""))
	if after := pf.n(t, `SELECT count(*) FROM payments.account_qualifications WHERE tenant_id=$1 AND store_id=$2 AND platform_qualification_id IS NOT NULL AND revoked_at IS NULL`, a.f.tenantA, a.f.storeA1); before != 1 || after != 1 {
		t.Fatalf("re-derivation: qualifications before=%d live-after=%d", before, after)
	}
	moreAgain := pf.derived(t, a)
	moreAgain.p = more.p
	if _, err := moreAgain.begin(pf.svc, t04Key("pf07-rederived"), moreAgain.input("zh-TW")); err != nil {
		t.Fatalf("start after unblock + enable: %v", err)
	}
	// merchant disable stops new starts; platform close stops only new enrollments
	pf.mustSet(t, a, pfInput(false, pf.read(t, a).Version, ""))
	third := moreAgain
	third.p = sstMoreHold(t, a)
	if _, err := third.begin(pf.svc, t04Key("pf07-disabled"), third.input("zh-TW")); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("start after merchant disable: %v", err)
	}
	b := pf.merchant(t, true)
	pf.setOpen(t, false)
	if _, err := pf.set(b, pfInput(true, 0, "")); pfCode(err) != "platform_stripe_closed" {
		t.Fatalf("enroll under CLOSED: %v", err)
	}
}

// ---------------------------------------------------------------------------------------------------------------------
// PF08 routes (HTTP_PG; the DB-free router build is internal/httpapi/payment_card_test.go)
// ---------------------------------------------------------------------------------------------------------------------

func TestPlatformStripePF08Routes(t *testing.T) {
	pf := pfNew(t)
	pf.designate(t)
	pf.setOpen(t, true)
	m := pf.merchant(t, false)
	path := "/v1/admin/stores/" + m.f.storeA1 + "/payments/card"
	call := func(method, token, body string) (int, string) {
		var rd io.Reader
		if body != "" {
			rd = strings.NewReader(body)
		}
		req := httptest.NewRequest(method, path, rd)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		pf.card.ServeHTTP(w, req)
		return w.Code, w.Body.String()
	}
	tok := m.f.tokens["a"]
	put := func(enabled bool, expected int, suffix string) (int, string) {
		return call(http.MethodPut, tok, fmt.Sprintf(`{"enabled":%v,"terms_version":%q,"descriptor_suffix":%s,"expected_version":%d}`, enabled, pfTerms, suffix, expected))
	}
	if status, _ := call(http.MethodGet, "", ""); status != http.StatusUnauthorized {
		t.Fatalf("GET without a token: %d", status)
	}
	if status, body := call(http.MethodGet, tok, ""); status != http.StatusOK || !strings.Contains(body, `"platform_state":"OPEN"`) || !strings.Contains(body, `"allowed":false`) {
		t.Fatalf("GET: %d %s", status, body)
	} else {
		pfSecretFree(t, "GET body", body)
	}
	if status, body := put(true, 0, "null"); status != http.StatusForbidden || !strings.Contains(body, "platform_stripe_not_allowed") {
		t.Fatalf("PUT not allowlisted: %d %s", status, body)
	}
	pf.allow(t, m, true)
	if status, body := put(true, 0, `"ABCDEFGHIJK"`); status != http.StatusUnprocessableEntity || !strings.Contains(body, "descriptor_suffix_too_long") {
		t.Fatalf("PUT long suffix: %d %s", status, body)
	}
	if status, body := put(true, 3, "null"); status != http.StatusConflict || !strings.Contains(body, "version_changed") {
		t.Fatalf("PUT wrong version: %d %s", status, body)
	}
	if status, body := call(http.MethodPut, tok, `{"enabled":true,"terms_version":"pf-2026-10","expected_version":0}`); status != http.StatusUnprocessableEntity {
		t.Fatalf("PUT missing key: %d %s", status, body)
	}
	if status, body := call(http.MethodPost, tok, `{}`); status != http.StatusMethodNotAllowed {
		t.Fatalf("POST: %d %s", status, body)
	}
	status, body := put(true, 0, `"SHOP A"`)
	if status != http.StatusOK || !strings.Contains(body, `"state":"ENABLED"`) || !strings.Contains(body, "LCPLATFORM* SHOP A") {
		t.Fatalf("PUT happy path: %d %s", status, body)
	}
	pfSecretFree(t, "PUT body", body)
	if status, body := put(false, 1, "null"); status != http.StatusOK || !strings.Contains(body, `"state":"DISABLED"`) {
		t.Fatalf("PUT disable: %d %s", status, body)
	}
}

// ---------------------------------------------------------------------------------------------------------------------
// P2-4: withdrawing the allowlist from a selling store also blocks it (stops new sales, refunds keep working)
// ---------------------------------------------------------------------------------------------------------------------

func TestPlatformStripePF11DisallowBlocks(t *testing.T) {
	pf := pfNew(t)
	pf.designate(t)
	pf.setOpen(t, true)
	a := pf.merchant(t, true)
	pf.mustSet(t, a, pfInput(true, 0, ""))
	oa := pf.pay(t, pf.derived(t, a), pf.endpoint, pf.secret)
	pf.grant(t, oa, "orders:read", "payments:refund")
	pf.allow(t, a, false)
	if s := pf.read(t, a); s.StoreState != "BLOCKED" || s.Allowed {
		t.Fatalf("disallow on a selling store: %+v", s)
	}
	more := pf.derivedMore(t, a)
	if _, err := more.begin(pf.svc, t04Key("pf11-new-start"), more.input("zh-TW")); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("new start after disallow: %v", err)
	}
	pf.awaitRefundFact(t, pf.mustRefund(t, oa, 800, "requested_by_customer"), oa.attempt, "SUCCEEDED") // refunds keep working
	pf.allow(t, a, true)
	if _, err := pf.set(a, pfInput(true, pf.read(t, a).Version, "")); pfCode(err) != "platform_stripe_blocked" {
		t.Fatalf("re-allow must not unblock: %v", err)
	}
	pf.block(t, a, false)
	pf.mustSet(t, a, pfInput(true, pf.read(t, a).Version, ""))
}

// ---------------------------------------------------------------------------------------------------------------------
// LIVE branch at SQL level (no Stripe call is needed: approvals, qualifications and the canary are rows). The owner pool
// plays the operator and the merchant; the definers under test are the production ones.
// ---------------------------------------------------------------------------------------------------------------------

type pfLive struct {
	f              *testFixture
	plat           psHarness
	conn, approval string
	version        int64
	t              *testing.T
}

var pfLiveChecklist = []string{"account_active", "canary_private", "descriptor", "dispute_notice", "managed_off", "merchant_terms", "payout_bank",
	"payout_ops", "platform_stripe_terms", "policy_pages", "radar_default", "rak_live", "tax_invoice", "three_ds", "webhook_live"}

const pfLiveReadiness = `{"AVSRule":true,"CVCRule":true,"ChargesEnabled":true,"CurrentlyDueCount":0,"DescriptorLength":12,"DetailsSubmitted":true,"PayoutsEnabled":true,"PrefixLength":8}`

func pfLiveNew(t *testing.T) *pfLive {
	t.Helper()
	f := pwIsolatedFixture(t)
	plat := psSetupItemsOn(t, f, 1)
	mustExec(t, f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'payments:refund') ON CONFLICT DO NOTHING`, plat.f.tenantA, plat.f.storeA1, plat.f.principalA)
	l := &pfLive{f: f, plat: plat, conn: randomUUID(), t: t}
	// owner-pool fixtures: the LIVE platform account row; the key bytes are opaque (no worker runs here)
	mustExec(t, f.owner, `SELECT integration.register_stripe_account($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::uuid,'LIVE','acct_LivePlat0001','k1',$6::bytea,$7::bytea)`,
		plat.f.tenantA, plat.f.storeA1, plat.f.principalA, l.conn, randomUUID(), randomBytes(12), randomBytes(48))
	l.approve(randomUUID(), 2000000)
	l.qualify()
	l.canary()
	mustExec(t, f.owner, `SELECT payments.designate_stripe_platform($1::uuid,$2::uuid,$3::uuid,$4::uuid,'Platform Live','LCLIVEPLAT','pf-2026-10',0)`,
		plat.f.tenantA, plat.f.storeA1, plat.f.principalA, l.conn)
	mustExec(t, f.owner, `SELECT payments.set_stripe_platform_open($1::uuid,$2::uuid,$3::uuid,'LIVE',true,NULL,1)`, plat.f.tenantA, plat.f.storeA1, plat.f.principalA)
	return l
}

func (l *pfLive) approve(id string, max int64) {
	l.t.Helper()
	mustExec(l.t, l.f.owner, `SELECT payments.approve_stripe_live($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::uuid,'TWD','approval-ref-'||$4::text,clock_timestamp()-interval '1 minute',5000,$6::bigint,$7::text[],$8::jsonb)`,
		l.plat.f.tenantA, l.plat.f.storeA1, l.plat.f.principalA, id, l.conn, max, pfLiveChecklist, pfLiveReadiness)
	l.approval = id
}

func (l *pfLive) qualify() {
	l.t.Helper()
	mustExec(l.t, l.f.owner, `SELECT payments.qualify_stripe_method($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::uuid,1,'LIVE','stripe-probe:cs_live_'||replace($4::text,'-',''),clock_timestamp()-interval '1 second',clock_timestamp()+interval '1 hour')`,
		l.plat.f.tenantA, l.plat.f.storeA1, l.plat.f.principalA, randomUUID(), l.conn)
}

// canary marks the active approval canary-verified directly (OWNER-POOL fixture; the real path needs a LIVE capture and refund).
func (l *pfLive) canary() {
	l.t.Helper()
	mustExec(l.t, l.f.owner, `UPDATE payments.stripe_live_approvals SET canary_attempt_id=gen_random_uuid(),canary_refund_id=gen_random_uuid(),canary_verified_at=clock_timestamp() WHERE id=$1`, l.approval)
}

func (l *pfLive) revokeSQL() string {
	return fmt.Sprintf(`SELECT payments.revoke_stripe_live('%s','%s','%s','%s','revoke-ref-%s')`, l.plat.f.tenantA, l.plat.f.storeA1, l.plat.f.principalA, l.approval, t04Tag())
}

func (l *pfLive) merchant(allow bool) psHarness {
	l.t.Helper()
	m := psSetupItemsOn(l.t, l.f, 1)
	mustExec(l.t, l.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'billing:manage') ON CONFLICT DO NOTHING`, m.f.tenantA, m.f.storeA1, m.f.principalA)
	if allow {
		mustExec(l.t, l.f.owner, `SELECT payments.allow_platform_stripe($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::uuid,'LIVE',true,'op@test','tk-live0001')`,
			l.plat.f.tenantA, l.plat.f.storeA1, l.plat.f.principalA, m.f.tenantA, m.f.storeA1)
	}
	return m
}

const pfLiveEnable = `SELECT payments.set_platform_stripe(sha256(convert_to($1,'UTF8')),$2::uuid,'LIVE',true,'pf-2026-10',NULL::text,$3::bigint)`

func (l *pfLive) liveDerived(m psHarness) int {
	return countRows(l.t, l.f.owner, `SELECT count(*) FROM payments.account_qualifications WHERE tenant_id=$1 AND store_id=$2 AND platform_qualification_id IS NOT NULL AND revoked_at IS NULL`, m.f.tenantA, m.f.storeA1)
}

func pfBlocked(t *testing.T, done <-chan error, what string) {
	t.Helper()
	select {
	case err := <-done:
		t.Fatalf("%s finished while the other transaction still held its locks (err=%v)", what, err)
	case <-time.After(1500 * time.Millisecond):
	}
}

// PF09 (P1-1): an enable and the platform live-revoke are serialised in both orders; a derived REAL_LIVE qualification can
// never survive the revoke.
func TestPlatformStripePF09LiveRevokeRace(t *testing.T) {
	ctx := context.Background()
	t.Run("enable first: revoke waits and then revokes the new derived qualification", func(t *testing.T) {
		l := pfLiveNew(t)
		m := l.merchant(true)
		tx, err := l.f.owner.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		var out []byte
		if err = tx.QueryRow(ctx, pfLiveEnable, m.f.tokens["a"], m.f.storeA1, int64(0)).Scan(&out); err != nil {
			t.Fatalf("LIVE enable: %v", err)
		}
		done := make(chan error, 1)
		go func() { _, e := l.f.owner.Exec(ctx, l.revokeSQL()); done <- e }()
		pfBlocked(t, done, "live-revoke")
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		if err = <-done; err != nil {
			t.Fatalf("revoke: %v", err)
		}
		if n := l.liveDerived(m); n != 0 {
			t.Fatalf("a derived REAL_LIVE qualification survived the platform revoke: %d", n)
		}
	})
	t.Run("revoke first: the enable waits, then finds the approval revoked", func(t *testing.T) {
		l := pfLiveNew(t)
		m := l.merchant(true)
		tx, err := l.f.owner.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if _, err = tx.Exec(ctx, l.revokeSQL()); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() {
			var o []byte
			done <- l.f.owner.QueryRow(ctx, pfLiveEnable, m.f.tokens["a"], m.f.storeA1, int64(0)).Scan(&o)
		}()
		pfBlocked(t, done, "enable")
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		if err = <-done; err == nil || !strings.Contains(err.Error(), "platform_stripe_unavailable") {
			t.Fatalf("enable after the revoke committed: %v", err)
		}
		if n := countRows(t, l.f.owner, `SELECT count(*) FROM payments.platform_stripe_enrollments WHERE tenant_id=$1`, m.f.tenantA); n != 0 || l.liveDerived(m) != 0 {
			t.Fatalf("a refused enable left rows: enrollments=%d", n)
		}
	})
}

// PF07 LIVE branch (P1-2): after a revoke and a fresh approval the platform is DESIGNATED until its new canary is verified;
// the new qualification is mirrored into NO store before then, and a merchant replay re-derives it afterwards.
func TestPlatformStripePF07LivePostRevoke(t *testing.T) {
	ctx := context.Background()
	l := pfLiveNew(t)
	m := l.merchant(true)
	var out []byte
	if err := l.f.owner.QueryRow(ctx, pfLiveEnable, m.f.tokens["a"], m.f.storeA1, int64(0)).Scan(&out); err != nil {
		t.Fatalf("LIVE enable: %v", err)
	}
	if l.liveDerived(m) != 1 {
		t.Fatal("enable must derive one REAL_LIVE qualification")
	}
	mustExec(t, l.f.owner, l.revokeSQL())
	if l.liveDerived(m) != 0 {
		t.Fatal("platform revoke left a derived qualification")
	}
	l.approve(randomUUID(), 1000000) // a NEW approval with a smaller cap, canary NOT yet verified
	l.qualify()                      // fan-out('qualify') runs with the platform DESIGNATED
	if n := l.liveDerived(m); n != 0 {
		t.Fatalf("a store was re-armed before the new canary was verified: %d derived qualifications", n)
	}
	l.canary() // platform OPEN again (enrollment_open is still true)
	if err := l.f.owner.QueryRow(ctx, pfLiveEnable, m.f.tokens["a"], m.f.storeA1, int64(0)).Scan(&out); err != nil {
		t.Fatalf("replay after the canary: %v", err)
	}
	var max int64
	var live bool
	if err := l.f.owner.QueryRow(ctx, `SELECT v.max_amount_minor,q.revoked_at IS NULL FROM payments.method_heads h JOIN payments.method_versions v
	 ON v.tenant_id=h.tenant_id AND v.store_id=h.store_id AND v.market_id=h.market_id AND v.country=h.country AND v.code=h.code AND v.version=h.current_version
	 JOIN payments.account_qualifications q ON q.tenant_id=v.tenant_id AND q.store_id=v.store_id AND q.id=v.qualification_id
	 WHERE h.tenant_id=$1 AND h.store_id=$2 AND h.code='stripe_checkout' AND v.enabled`, m.f.tenantA, m.f.storeA1).Scan(&max, &live); err != nil || !live || max != 1000000 {
		t.Fatalf("replay must re-derive under the NEW approval cap: max=%d live=%v err=%v", max, live, err)
	}
}
