package foundation_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/command"
	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/payments"
	"livecommerce/internal/platform"
	"livecommerce/internal/pricing"
)

type pmFixture struct {
	m  *maFixture
	in payments.MethodInput
}

func pmSetup(t *testing.T) *pmFixture {
	t.Helper()
	m := maSetup(t)
	// Only this isolated fixture's stores are changed, before creating markets.
	mustExec(t, m.f.base.owner, `UPDATE control.stores SET currency='TWD' WHERE tenant_id=$1`, m.f.tenant)
	var market pricing.Market
	e := m.f.scoped(context.Background(), m.f.token, m.f.store, func(tx pgx.Tx, s platform.Scope) error {
		var e error
		market, e = pricing.CreateMarket(context.Background(), tx, s, t04Key("pm-market"), pricing.MarketInput{Code: "tw", Name: "Fixture Taiwan", Currency: "TWD"})
		return e
	})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		mustExec(t, m.f.base.owner, `DELETE FROM payments.method_heads WHERE tenant_id=$1`, m.f.tenant)
		mustExec(t, m.f.base.owner, `DELETE FROM payments.method_versions WHERE tenant_id=$1`, m.f.tenant)
		mustExec(t, m.f.base.owner, `DELETE FROM pricing.markets WHERE tenant_id=$1`, m.f.tenant)
	})
	return &pmFixture{m: m, in: payments.MethodInput{MarketID: market.ID, Country: "TW", Code: "payuni_credit", Environment: "SANDBOX", NameHans: "信用卡", NameHant: "信用卡", NameEN: "Credit card", Visible: true, SortOrder: 10, MinAmountMinor: 100, MaxAmountMinor: 100000}}
}
func (p *pmFixture) set(token, store, key string, in payments.MethodInput) (out payments.Method, e error) {
	e = p.m.f.scoped(context.Background(), token, store, func(tx pgx.Tx, s platform.Scope) error {
		var e error
		out, e = payments.SetMethod(context.Background(), tx, s, token, key, in)
		return e
	})
	return
}
func (p *pmFixture) get(token, store string) (out payments.Method, e error) {
	e = p.m.f.scoped(context.Background(), token, store, func(tx pgx.Tx, s platform.Scope) error {
		var e error
		out, e = payments.GetMethod(context.Background(), tx, s, token, p.in.MarketID, p.in.Country, p.in.Code)
		return e
	})
	return
}
func (p *pmFixture) inspect(in payments.CheckInput) (out payments.Availability, e error) {
	e = p.m.f.scoped(context.Background(), p.m.f.token, p.m.f.store, func(tx pgx.Tx, s platform.Scope) error {
		var e error
		out, e = payments.InspectMethod(context.Background(), tx, s, p.m.f.token, in)
		return e
	})
	return
}
func (p *pmFixture) check(version int64) payments.CheckInput {
	return payments.CheckInput{MarketID: p.in.MarketID, Country: p.in.Country, Code: p.in.Code, ExpectedVersion: version, Environment: p.in.Environment, Currency: "TWD", AmountMinor: 1000}
}
func (p *pmFixture) link(t *testing.T) accounts.Connection {
	t.Helper()
	a := maHistorical(t, p.m, p.m.f.store, maInput(), 1)
	p.in.ConnectionID, p.in.BindingVersion = a.ID, a.BindingVersion
	return a
}
func (p *pmFixture) counts(t *testing.T) (out [9]int64) {
	t.Helper()
	m := p.m.facts(t)
	copy(out[:], m[:])
	if e := p.m.f.base.owner.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM payments.method_versions WHERE tenant_id=$1),(SELECT count(*) FROM payments.method_heads WHERE tenant_id=$1)`, p.m.f.tenant).Scan(&out[7], &out[8]); e != nil {
		t.Fatal(e)
	}
	return
}

func TestPaymentMethodsRevisionsReplayAndIsolation(t *testing.T) {
	p := pmSetup(t)
	f := p.m.f
	if _, e := p.get(f.token, f.store); !errors.Is(e, command.ErrNotFound) {
		t.Fatalf("missing method: %v", e)
	}
	p.link(t)
	first := pmHistorical(t, p, p.in, 1)
	if first.Version != 1 || first.Enabled || !first.Visible || first.Provider != "payuni" || first.Currency != "TWD" {
		t.Fatalf("historical metadata: %+v", first)
	}
	changed := p.in
	changed.ExpectedVersion = 1
	changed.Visible = false
	changed.NameHant = "線上刷卡"
	changed.SortOrder = 20
	second := pmHistorical(t, p, changed, 2)
	if second.Version != 2 || second.Visible || second.Enabled || second.NameHant != "線上刷卡" {
		t.Fatal("historical revision lost language/visibility")
	}
	before, snapshot := p.counts(t), pmHistorySnapshot(t, p)
	key := t04Key("pm-first")
	for _, in := range []payments.MethodInput{p.in, changed} {
		for _, k := range []string{key, t04Key("pm-stale")} {
			if out, e := p.set(f.token, f.store, k, in); !errors.Is(e, command.ErrInvalid) || out.Version != 0 {
				t.Fatalf("retired edit/replay admitted: %v", e)
			}
		}
	}
	for _, code := range []string{"payuni_installment", "payuni_atm", "payuni_cvs", "payuni_linepay"} {
		v := p.in
		v.Code = code
		if _, e := p.set(f.token, f.store, t04Key("pm-other"), v); !errors.Is(e, command.ErrInvalid) {
			t.Fatal(e)
		}
	}
	got, e := p.get(f.token, f.store)
	if e != nil || !reflect.DeepEqual(got, second) {
		t.Fatalf("current historical readback: %v", e)
	}
	if p.counts(t) != before || pmHistorySnapshot(t, p) != snapshot {
		t.Fatal("retired method call changed history, receipt, audit or jobs")
	}
	if _, e = p.get(f.token, f.otherStore); !errors.Is(e, command.ErrNotFound) {
		t.Fatalf("foreign store get: %v", e)
	}
	var n int
	e = f.scoped(context.Background(), f.token, f.otherStore, func(tx pgx.Tx, s platform.Scope) error {
		return tx.QueryRow(context.Background(), `SELECT count(*) FROM payments.method_versions WHERE market_id=$1`, p.in.MarketID).Scan(&n)
	})
	if e != nil || n != 0 {
		t.Fatalf("RLS leak: %d %v", n, e)
	}
	other := pmSetup(t)
	if _, e = other.set(other.m.f.token, other.m.f.store, t04Key("pm-foreign"), p.in); e == nil {
		t.Fatal("foreign tenant input accepted")
	}
}

func TestPaymentMethodsDiagnosticsAndCredentialRotation(t *testing.T) {
	p := pmSetup(t)
	f := p.m.f
	a := p.link(t)
	pmHistorical(t, p, p.in, 1)
	check := p.check(1)
	baseline, e := p.inspect(check)
	want := []string{"METHOD_DISABLED", "CREDENTIALS_UNVERIFIED", "ADAPTER_UNAVAILABLE"}
	if e != nil || baseline.Available || baseline.MethodVersion != 1 || baseline.CredentialVersion != 1 || !reflect.DeepEqual(baseline.Reasons, want) {
		t.Fatalf("diagnostics: %+v %v", baseline, e)
	}
	for _, tc := range []struct {
		name, reason string
		change       func(*payments.CheckInput)
	}{
		{"version", "METHOD_VERSION_CHANGED", func(v *payments.CheckInput) { v.ExpectedVersion = 2 }},
		{"environment", "ENVIRONMENT_MISMATCH", func(v *payments.CheckInput) { v.Environment = "LIVE" }},
		{"currency", "CURRENCY_MISMATCH", func(v *payments.CheckInput) { v.Currency = "USD" }},
		{"min", "AMOUNT_OUT_OF_RANGE", func(v *payments.CheckInput) { v.AmountMinor = 99 }},
		{"max", "AMOUNT_OUT_OF_RANGE", func(v *payments.CheckInput) { v.AmountMinor = 100001 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := check
			tc.change(&v)
			got, e := p.inspect(v)
			if e != nil || got.Available || !slices.Contains(got.Reasons, tc.reason) || len(got.Reasons) != len(want)+1 {
				t.Fatalf("independent diagnostic missing: %+v %v", got, e)
			}
		})
	}
	for _, amount := range []int64{100, 100000} {
		v := check
		v.AmountMinor = amount
		got, e := p.inspect(v)
		if e != nil || slices.Contains(got.Reasons, "AMOUNT_OUT_OF_RANGE") {
			t.Fatalf("inclusive bound: %+v %v", got, e)
		}
	}
	snapshot := pmHistorySnapshot(t, p)
	if _, e = p.m.rotate(f.token, t04Key("pm-rotate"), accounts.RotateInput{ConnectionID: a.ID, ExpectedVersion: 1, Credentials: accounts.Credentials{HashKey: "rotated_fixture", HashIV: "rotated_fixture_iv"}}); !errors.Is(e, command.ErrInvalid) || pmHistorySnapshot(t, p) != snapshot {
		t.Fatal(e)
	}
	got, e := p.inspect(check)
	if e != nil || got.MethodVersion != 1 || got.BindingVersion != 1 || got.CredentialVersion != 1 || !reflect.DeepEqual(got.Reasons, want) {
		t.Fatalf("rotation rebind: %+v %v", got, e)
	}
	e = f.scoped(context.Background(), f.token, f.store, func(tx pgx.Tx, s platform.Scope) error {
		_, e := f.service.SetBindingEnabled(context.Background(), tx, s, f.token, t04Key("pm-disable"), a.BindingID, 1, false)
		return e
	})
	if e != nil {
		t.Fatal(e)
	}
	got, e = p.inspect(check)
	if e != nil || got.BindingVersion != 2 || !slices.Contains(got.Reasons, "BINDING_VERSION_CHANGED") || !slices.Contains(got.Reasons, "BINDING_DISABLED") {
		t.Fatalf("binding gate: %+v %v", got, e)
	}
	mustExec(t, f.base.owner, `UPDATE pricing.markets SET active=false WHERE id=$1`, p.in.MarketID)
	hide := p.in
	hide.ExpectedVersion = 1
	hide.Visible = false
	if _, e = p.set(f.token, f.store, t04Key("pm-hide-stale"), hide); !errors.Is(e, command.ErrInvalid) {
		t.Fatal("retired hide admitted", e)
	}
	pmHistorical(t, p, hide, 2) // synthetic history: verify diagnostics for a previously hidden revision
	got, e = p.inspect(p.check(2))
	if e != nil || !slices.Contains(got.Reasons, "METHOD_HIDDEN") || !slices.Contains(got.Reasons, "MARKET_INACTIVE") {
		t.Fatalf("hidden inactive: %+v %v", got, e)
	}
	hide.ExpectedVersion = 2
	hide.ConnectionID = ""
	hide.BindingVersion = 0
	if _, e = p.set(f.token, f.store, t04Key("pm-unlink"), hide); !errors.Is(e, command.ErrInvalid) {
		t.Fatal("retired unlink admitted", e)
	}
	pmHistorical(t, p, hide, 3) // synthetic pre-removal unlinked revision
	got, e = p.inspect(p.check(3))
	if e != nil || got.BindingVersion != 0 || got.CredentialVersion != 0 || !slices.Contains(got.Reasons, "CONNECTION_MISSING") || slices.Contains(got.Reasons, "CREDENTIALS_UNVERIFIED") {
		t.Fatalf("unlinked: %+v %v", got, e)
	}
}

func TestPaymentMethodsAuthorityAndEnvironment(t *testing.T) {
	p := pmSetup(t)
	f := p.m.f
	p.link(t)
	key := t04Key("pm-auth")
	pmHistorical(t, p, p.in, 1)
	if _, e := p.get(f.otherToken, f.store); e != nil {
		t.Fatalf("read-only permission denied: %v", e)
	}
	if _, e := p.set(f.otherToken, f.store, t04Key("pm-readonly"), p.in); !errors.Is(e, command.ErrInvalid) {
		t.Fatalf("read-only wrote: %v", e)
	}
	if _, e := p.get(f.missingPermission, f.store); !errors.Is(e, platform.ErrForbidden) {
		t.Fatalf("no read permission: %v", e)
	}
	changed := p.in
	changed.ExpectedVersion = 1
	changed.Environment = "LIVE"
	if _, e := p.set(f.token, f.store, t04Key("pm-mixed-env"), changed); e == nil {
		t.Fatal("sandbox credential linked to live")
	}
	foreign := maHistorical(t, p.m, f.otherStore, maInput(), 1)
	var e error
	changed = p.in
	changed.Code, changed.ConnectionID = "payuni_atm", foreign.ID
	if _, e = p.set(f.token, f.store, t04Key("pm-cross-store-connection"), changed); !errors.Is(e, command.ErrInvalid) {
		t.Fatalf("same-market foreign connection not rejected: %v", e)
	}
	liveInput := maInput()
	liveInput.Environment = "LIVE"
	live := maHistorical(t, p.m, f.store, liveInput, 1)
	changed.Environment, changed.ConnectionID = "LIVE", live.ID
	if out, e := p.set(f.token, f.store, t04Key("pm-live-draft"), changed); !errors.Is(e, command.ErrInvalid) || out.Version != 0 {
		t.Fatalf("separate live draft: %+v %v", out, e)
	}
	missing := p.in
	missing.ConnectionID = randomUUID()
	if _, e := p.set(f.token, f.store, t04Key("pm-missing-account"), missing); e == nil {
		t.Fatal("nonexistent connection accepted")
	}
	stale := p.in
	stale.Code = "payuni_atm"
	stale.BindingVersion = 2
	if _, e := p.set(f.token, f.store, t04Key("pm-new-stale-binding"), stale); !errors.Is(e, command.ErrInvalid) {
		t.Fatalf("new stale binding accepted: %v", e)
	}
	var forged payments.Method
	e = f.scoped(context.Background(), f.token, f.store, func(tx pgx.Tx, s platform.Scope) error {
		s.StoreID = f.otherStore
		var e error
		forged, e = payments.GetMethod(context.Background(), tx, s, f.token, p.in.MarketID, "TW", p.in.Code)
		return e
	})
	if e == nil || forged.Version != 0 {
		t.Fatal("forged scope leaked method")
	}
	// A revoked caller must never recover an old permanent receipt from a retired writer.
	mustExec(t, f.base.owner, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='integration:manage'`, f.tenant, f.store, f.principal)
	if _, e = p.set(f.token, f.store, key, p.in); !errors.Is(e, command.ErrInvalid) {
		t.Fatalf("revoked manage replayed: %v", e)
	}
}

// Observe an actual database lock wait before revoking permission; a sleep or
// an immediately revoked token would only retest the preflight authorization.
func TestPaymentMethodsPermissionRevokedDuringWait(t *testing.T) {
	for _, kind := range []string{"set", "inspect"} {
		t.Run(kind, func(t *testing.T) {
			p := pmSetup(t)
			f := p.m.f
			pmHistorical(t, p, p.in, 1)
			before := p.counts(t)
			if kind == "set" {
				snapshot := pmHistorySnapshot(t, p)
				mustExec(t, f.base.owner, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='integration:manage'`, f.tenant, f.store, f.principal)
				in := p.in
				in.ExpectedVersion = 1
				in.Visible = false
				if _, err := p.set(f.token, f.store, t04Key("pm-retired-revoked"), in); !errors.Is(err, command.ErrInvalid) || p.counts(t) != before || pmHistorySnapshot(t, p) != snapshot {
					t.Fatal("retired writer admitted after permission loss", err)
				}
				return
			}
			block, e := f.base.owner.Begin(context.Background())
			if e != nil {
				t.Fatal(e)
			}
			defer block.Rollback(context.Background())
			if _, e = block.Exec(context.Background(), `SELECT id FROM pricing.markets WHERE id=$1 FOR UPDATE`, p.in.MarketID); e != nil {
				t.Fatal(e)
			}
			started := make(chan uint32, 1)
			done := make(chan error, 1)
			go func() {
				done <- f.scoped(context.Background(), f.token, f.store, func(tx pgx.Tx, s platform.Scope) error {
					started <- tx.Conn().PgConn().PID()
					_, e := payments.InspectMethod(context.Background(), tx, s, f.token, p.check(1))
					return e
				})
			}()
			pid := <-started
			// Always release and join on a failed assertion too, before fixture cleanup.
			joined := false
			defer func() {
				_ = block.Rollback(context.Background())
				if !joined {
					<-done
				}
			}()
			deadline := time.Now().Add(700 * time.Millisecond)
			blocked := false
			for time.Now().Before(deadline) {
				if e = f.base.owner.QueryRow(context.Background(), `SELECT cardinality(pg_blocking_pids($1))>0`, pid).Scan(&blocked); e != nil {
					t.Fatal(e)
				}
				if blocked {
					break
				}
				time.Sleep(5 * time.Millisecond)
			}
			if !blocked {
				t.Fatal("did not observe method waiting for market lock")
			}
			permission := "integration:read"
			mustExec(t, f.base.owner, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission=$4`, f.tenant, f.store, f.principal, permission)
			if e = block.Commit(context.Background()); e != nil {
				t.Fatal(e)
			}
			e = <-done
			joined = true
			if !errors.Is(e, platform.ErrForbidden) {
				t.Fatalf("revoked mid-wait result: %v", e)
			}
			if p.counts(t) != before {
				t.Fatal("revoked operation committed configuration/receipt/audit")
			}
		})
	}
}

func TestPaymentMethodsConcurrentReplayAndCAS(t *testing.T) {
	p := pmSetup(t)
	f := p.m.f
	pmHistorical(t, p, p.in, 1)
	key := t04Key("pm-race-create")
	before, snapshot := p.counts(t), pmHistorySnapshot(t, p)
	var wg sync.WaitGroup
	outs := make([]payments.Method, 4)
	errs := make([]error, 4)
	for _, update := range []bool{false, true} {
		for i := range errs {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				in := p.in
				if update {
					in.ExpectedVersion = 1
					in.SortOrder = i
				}
				outs[i], errs[i] = p.set(f.token, f.store, key, in)
			}(i)
		}
		wg.Wait()
		for i, e := range errs {
			if !errors.Is(e, command.ErrInvalid) || outs[i].Version != 0 {
				t.Fatalf("retired concurrent method write admitted: %v", e)
			}
		}
		if p.counts(t) != before || pmHistorySnapshot(t, p) != snapshot {
			t.Fatal("retired concurrent method writes changed history")
		}
	}
}

func TestPaymentMethodsSQLGuardAndScopedTargets(t *testing.T) {
	p := pmSetup(t)
	f := p.m.f
	p.link(t)
	pmHistorical(t, p, p.in, 1)
	enabled := p.in
	enabled.ExpectedVersion = 1
	enabled.Enabled = true
	if _, e := p.set(f.token, f.store, t04Key("pm-not-ready"), enabled); !errors.Is(e, command.ErrConflict) {
		t.Fatalf("Go admission guard: %v", e)
	}
	for _, q := range []string{`UPDATE payments.method_versions SET visible=false WHERE false`, `DELETE FROM payments.method_versions WHERE false`, `DELETE FROM payments.method_heads WHERE false`, `UPDATE payments.method_heads SET code=code WHERE false`} {
		e := f.scoped(context.Background(), f.token, f.store, func(tx pgx.Tx, s platform.Scope) error { _, e := tx.Exec(context.Background(), q); return e })
		var pg *pgconn.PgError
		if !errors.As(e, &pg) || pg.Code != "42501" {
			t.Fatalf("immutable ACL: %v", e)
		}
	}
	var n int
	var pg *pgconn.PgError
	if e := f.worker.QueryRow(context.Background(), `SELECT count(*) FROM payments.method_versions`).Scan(&n); !errors.As(e, &pg) || pg.Code != "42501" {
		t.Fatalf("worker acquired settings: %v", e)
	}
	for _, tc := range []struct{ name, expr, sqlstate, constraint string }{
		// 0016 replaces the disabled-only guard with a required scoped proof;
		// this ordinary merchant still cannot enable an unqualified method.
		{"enabled", "true,connection_id,environment,store_id,market_id", "23514", "method_admission_reference"},
		{"environment", "false,connection_id,'LIVE',store_id,market_id", "23503", "method_account_target_fk"},
		{"connection", "false,gen_random_uuid(),environment,store_id,market_id", "23503", "method_account_target_fk"},
		{"market", "false,connection_id,environment,store_id,gen_random_uuid()", "23503", "method_market_target_fk"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parts := strings.Split(tc.expr, ",")
			q := `INSERT INTO payments.method_versions(tenant_id,store_id,market_id,country,code,version,provider,environment,connection_id,binding_version,currency,name_hans,name_hant,name_en,enabled,visible,sort_order,min_amount_minor,max_amount_minor,principal_id)
  SELECT tenant_id,` + parts[3] + `,` + parts[4] + `,country,code,2,provider,` + parts[2] + `,` + parts[1] + `,binding_version,currency,name_hans,name_hant,name_en,` + parts[0] + `,visible,sort_order,min_amount_minor,max_amount_minor,principal_id FROM payments.method_versions WHERE market_id=$1 AND version=1`
			e := f.scoped(context.Background(), f.token, f.store, func(tx pgx.Tx, s platform.Scope) error {
				_, e := tx.Exec(context.Background(), q, p.in.MarketID)
				return e
			})
			var pg *pgconn.PgError
			if !errors.As(e, &pg) || pg.Code != tc.sqlstate || pg.ConstraintName != tc.constraint {
				t.Fatalf("wrong rejecting cause: %v", e)
			}
		})
	}
}

func TestPaymentMethodsAtomicFaultRollback(t *testing.T) {
	for _, update := range []bool{false, true} {
		for _, table := range []string{"payments.method_versions", "payments.method_heads", "ops.audit_events", "ops.command_results"} {
			t.Run(table+map[bool]string{false: "_create", true: "_update"}[update], func(t *testing.T) {
				p := pmSetup(t)
				f := p.m.f
				if update {
					pmHistorical(t, p, p.in, 1)
					p.in.ExpectedVersion = 1
					p.in.Visible = false
				}
				before := p.counts(t)
				name := "pm_fault_" + t04Tag()
				event := "INSERT"
				if update && table == "payments.method_heads" {
					event = "UPDATE"
				}
				mustExec(t, f.base.owner, `CREATE SEQUENCE public.`+name+`_hits`)
				mustExec(t, f.base.owner, `GRANT USAGE ON SEQUENCE public.`+name+`_hits TO commerce_runtime`)
				mustExec(t, f.base.owner, `CREATE FUNCTION public.`+name+`() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF current_setting('app.tenant_id',true)=`+quoteLiteral(f.tenant)+` THEN PERFORM nextval('public.`+name+`_hits'); RAISE EXCEPTION 'payment method synthetic fault'; END IF; RETURN NEW; END $$`)
				mustExec(t, f.base.owner, `CREATE TRIGGER `+name+` BEFORE `+event+` ON `+table+` FOR EACH ROW EXECUTE FUNCTION public.`+name+`()`)
				t.Cleanup(func() {
					mustExec(t, f.base.owner, `DROP TRIGGER `+name+` ON `+table)
					mustExec(t, f.base.owner, `DROP FUNCTION public.`+name+`()`)
					mustExec(t, f.base.owner, `DROP SEQUENCE public.`+name+`_hits`)
				})
				if _, e := p.set(f.token, f.store, t04Key("pm-fault"), p.in); !errors.Is(e, command.ErrInvalid) {
					t.Fatal("injected failure accepted")
				}
				var fired bool
				if e := f.base.owner.QueryRow(context.Background(), `SELECT is_called FROM public.`+name+`_hits`).Scan(&fired); e != nil || fired {
					t.Fatalf("retired method reached write target: %v", e)
				}
				if p.counts(t) != before {
					t.Fatal("partial method state survived")
				}
				got, e := p.get(f.token, f.store)
				if update && (e != nil || got.Version != 1 || !got.Visible) {
					t.Fatalf("old snapshot changed: %+v %v", got, e)
				}
				if !update && !errors.Is(e, command.ErrNotFound) {
					t.Fatalf("failed create visible: %v", e)
				}
			})
		}
	}
}
