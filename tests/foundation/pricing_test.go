package foundation_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/buyer"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
	"livecommerce/internal/pricing"
)

var (
	pricingGrantOnce sync.Once
	pricingGrantErr  error
)

func TestPricingBoundary(t *testing.T) {
	f := pricingFixture(t)
	ctx := context.Background()

	t.Run("checked HALF_UP calculation", func(t *testing.T) {
		base := pricing.Policy{MarketID: randomUUID(), Country: "US", Method: "home", Currency: "USD", ShippingMode: "country_flat", TaxMode: "exclusive", TaxBasis: "goods_and_shipping", Version: 1, ShippingMinor: 50, TaxRateBPS: 500, QuoteTTLSeconds: 300, Enabled: true}
		got, err := pricing.Calculate(base, []pricing.AmountLine{{UnitPriceMinor: 100, Quantity: 1}})
		if err != nil || got.SubtotalMinor != 100 || got.TaxMinor != 8 || got.ShippingTaxMinor != 3 || got.TotalMinor != 158 || got.Lines[0].TaxMinor != 5 || got.Lines[0].TotalMinor != 105 {
			t.Fatalf("exclusive calculation=%+v err=%v", got, err)
		}
		inclusive := base
		inclusive.TaxMode, inclusive.TaxBasis, inclusive.ShippingMinor = "inclusive", "goods_and_shipping", 105
		got, err = pricing.Calculate(inclusive, []pricing.AmountLine{{UnitPriceMinor: 105, Quantity: 1}})
		if err != nil || got.TaxMinor != 10 || got.ShippingTaxMinor != 5 || got.TotalMinor != 210 || got.Lines[0].TaxMinor != 5 || got.Lines[0].TotalMinor != 105 {
			t.Fatalf("inclusive calculation=%+v err=%v", got, err)
		}
		half := base
		half.TaxRateBPS, half.ShippingMinor, half.TaxBasis = 5000, 0, "goods"
		got, err = pricing.Calculate(half, []pricing.AmountLine{{UnitPriceMinor: 1, Quantity: 1}})
		if err != nil || got.TaxMinor != 1 || got.TotalMinor != 2 {
			t.Fatalf("HALF_UP boundary=%+v err=%v", got, err)
		}
		if _, err := pricing.Calculate(base, []pricing.AmountLine{{UnitPriceMinor: command.MaxMoney, Quantity: 2}}); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("multiplication overflow err=%v", err)
		}
		if _, err := pricing.Calculate(base, []pricing.AmountLine{{UnitPriceMinor: command.MaxMoney, Quantity: 1}, {UnitPriceMinor: 1, Quantity: 1}}); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("sum overflow err=%v", err)
		}
		if _, err := pricing.Calculate(base, nil); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("empty quote err=%v", err)
		}
		tooMany := make([]pricing.AmountLine, 51)
		for i := range tooMany {
			tooMany[i] = pricing.AmountLine{UnitPriceMinor: 1, Quantity: 1}
		}
		if _, err := pricing.Calculate(base, tooMany); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("51 lines err=%v", err)
		}
		invalidNone := base
		invalidNone.TaxMode, invalidNone.TaxRateBPS = "none", 1
		if _, err := pricing.Calculate(invalidNone, []pricing.AmountLine{{UnitPriceMinor: 1, Quantity: 1}}); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("none with tax err=%v", err)
		}
		encoded, err := json.Marshal(got)
		if err != nil || !strings.Contains(string(encoded), `"shipping_tax_minor"`) || strings.Contains(string(encoded), "ShippingTaxMinor") {
			t.Fatalf("calculation JSON=%s err=%v", encoded, err)
		}
	})

	marketKey := t04Key("pricing-market")
	market, err := pricingScoped(ctx, f, f.tokens["a"], f.storeA1, "pricing:write", func(tx pgx.Tx, scope platform.Scope) (pricing.Market, error) {
		return pricing.CreateMarket(ctx, tx, scope, marketKey, pricing.MarketInput{Code: "us_" + t04Tag(), Name: "United States", Currency: "USD"})
	})
	if err != nil {
		t.Fatalf("create market: %v", err)
	}

	t.Run("market replay actor conflict permission and CAS", func(t *testing.T) {
		replay, err := pricingScoped(ctx, f, f.tokens["a"], f.storeA1, "pricing:write", func(tx pgx.Tx, scope platform.Scope) (pricing.Market, error) {
			return pricing.CreateMarket(ctx, tx, scope, marketKey, pricing.MarketInput{Code: market.Code, Name: market.Name, Currency: market.Currency})
		})
		if err != nil || replay != market {
			t.Fatalf("market replay=%+v err=%v", replay, err)
		}
		_, err = pricingScoped(ctx, f, f.tokens["a"], f.storeA1, "pricing:write", func(tx pgx.Tx, scope platform.Scope) (pricing.Market, error) {
			return pricing.CreateMarket(ctx, tx, scope, marketKey, pricing.MarketInput{Code: market.Code, Name: "changed", Currency: market.Currency})
		})
		if !errors.Is(err, command.ErrConflict) {
			t.Fatalf("changed-body replay err=%v", err)
		}

		otherToken := seedPricingActor(t, f, f.tenantA, f.storeA1, true)
		_, err = pricingScoped(ctx, f, otherToken, f.storeA1, "pricing:write", func(tx pgx.Tx, scope platform.Scope) (pricing.Market, error) {
			return pricing.CreateMarket(ctx, tx, scope, marketKey, pricing.MarketInput{Code: market.Code, Name: market.Name, Currency: market.Currency})
		})
		if !errors.Is(err, command.ErrConflict) {
			t.Fatalf("other actor reused store-global key err=%v", err)
		}
		readOnlyToken := seedPricingActor(t, f, f.tenantA, f.storeA1, false)
		called := false
		err = platform.WithScope(ctx, f.runtime, readOnlyToken, f.storeA1, "pricing:write", func(pgx.Tx, platform.Scope) error {
			called = true
			return nil
		})
		if !errors.Is(err, platform.ErrForbidden) || called {
			t.Fatalf("pricing:write denial err=%v called=%v", err, called)
		}
		_, err = pricingScoped(ctx, f, f.tokens["a"], f.storeA1, "pricing:write", func(tx pgx.Tx, scope platform.Scope) (pricing.Market, error) {
			return pricing.CreateMarket(ctx, tx, scope, t04Key("currency"), pricing.MarketInput{Code: "tw_" + t04Tag(), Name: "Wrong currency", Currency: "TWD"})
		})
		if !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("currency mismatch err=%v", err)
		}

		type activeResult struct {
			market pricing.Market
			err    error
		}
		start := make(chan struct{})
		results := make(chan activeResult, 2)
		for i, active := range []bool{false, true} {
			go func(i int, active bool) {
				<-start
				updated, err := pricingScoped(ctx, f, f.tokens["a"], f.storeA1, "pricing:write", func(tx pgx.Tx, scope platform.Scope) (pricing.Market, error) {
					return pricing.SetMarketActive(ctx, tx, scope, fmt.Sprintf("market-cas:%d:%s", i, t04Tag()), market.ID, market.Version, active)
				})
				results <- activeResult{updated, err}
			}(i, active)
		}
		close(start)
		successes, conflicts := 0, 0
		for range 2 {
			result := <-results
			switch {
			case result.err == nil:
				successes++
				if result.market.Version != market.Version+1 {
					t.Fatalf("CAS winner version=%d", result.market.Version)
				}
			case errors.Is(result.err, command.ErrConflict):
				conflicts++
			default:
				t.Fatalf("CAS unexpected err=%v", result.err)
			}
		}
		if successes != 1 || conflicts != 1 {
			t.Fatalf("CAS successes=%d conflicts=%d", successes, conflicts)
		}

		stateMarket, err := createPricingMarket(ctx, f, "state")
		if err != nil {
			t.Fatal(err)
		}
		stateKey := t04Key("market-state")
		changed, err := pricingScoped(ctx, f, f.tokens["a"], f.storeA1, "pricing:write", func(tx pgx.Tx, scope platform.Scope) (pricing.Market, error) {
			return pricing.SetMarketActive(ctx, tx, scope, stateKey, stateMarket.ID, stateMarket.Version, false)
		})
		if err != nil || changed.Active {
			t.Fatalf("state change=%+v err=%v", changed, err)
		}
		replayed, err := pricingScoped(ctx, f, f.tokens["a"], f.storeA1, "pricing:write", func(tx pgx.Tx, scope platform.Scope) (pricing.Market, error) {
			return pricing.SetMarketActive(ctx, tx, scope, stateKey, stateMarket.ID, stateMarket.Version, false)
		})
		if err != nil || replayed != changed {
			t.Fatalf("state replay=%+v err=%v", replayed, err)
		}
		_, err = pricingScoped(ctx, f, f.tokens["a"], f.storeA1, "pricing:write", func(tx pgx.Tx, scope platform.Scope) (pricing.Market, error) {
			return pricing.SetMarketActive(ctx, tx, scope, stateKey, stateMarket.ID, changed.Version, true)
		})
		if !errors.Is(err, command.ErrConflict) {
			t.Fatalf("state changed-body replay err=%v", err)
		}
	})

	policyMarket, err := createPricingMarket(ctx, f, "policy")
	if err != nil {
		t.Fatalf("create policy market: %v", err)
	}
	zero := int64(0)
	policyInput := pricing.PolicyInput{MarketID: policyMarket.ID, Country: "US", Method: "home", Currency: "USD", ShippingMode: "country_flat", TaxMode: "none", TaxBasis: "goods", ExpectedVersion: 0, ShippingMinor: &zero, TaxRateBPS: &zero, QuoteTTLSeconds: 300, Enabled: true, ConfigurationRef: "synthetic operator configuration"}

	t.Run("policy serialization explicit zero immutable versions and disabled", func(t *testing.T) {
		omitted := policyInput
		omitted.ShippingMinor = nil
		_, err := pricingScoped(ctx, f, f.tokens["a"], f.storeA1, "pricing:write", func(tx pgx.Tx, scope platform.Scope) (pricing.Policy, error) {
			return pricing.SetPolicy(ctx, tx, scope, t04Key("omitted"), omitted)
		})
		if !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("omitted explicit zero err=%v", err)
		}

		start := make(chan struct{})
		errs := make(chan error, 2)
		for i := range 2 {
			go func(i int) {
				<-start
				_, err := pricingScoped(ctx, f, f.tokens["a"], f.storeA1, "pricing:write", func(tx pgx.Tx, scope platform.Scope) (pricing.Policy, error) {
					return pricing.SetPolicy(ctx, tx, scope, fmt.Sprintf("policy-create:%d:%s", i, t04Tag()), policyInput)
				})
				errs <- err
			}(i)
		}
		close(start)
		successes, conflicts := 0, 0
		for range 2 {
			err := <-errs
			if err == nil {
				successes++
			} else if errors.Is(err, command.ErrConflict) {
				conflicts++
			} else {
				t.Fatalf("policy create race err=%v", err)
			}
		}
		if successes != 1 || conflicts != 1 {
			t.Fatalf("policy create successes=%d conflicts=%d", successes, conflicts)
		}

		locked, err := pricingScoped(ctx, f, f.tokens["a"], f.storeA1, "pricing:read", func(tx pgx.Tx, scope platform.Scope) (pricing.Policy, error) {
			return pricing.LockCurrent(ctx, tx, scope.TenantID, scope.StoreID, policyMarket.ID, "US", "home")
		})
		if err != nil || locked.Version != 1 || locked.ShippingMinor != 0 || locked.TaxRateBPS != 0 {
			t.Fatalf("explicit-zero policy=%+v err=%v", locked, err)
		}
		replayInput := policyInput
		replayInput.Country = "MX"
		replayKey := t04Key("policy-replay")
		firstReplay, err := pricingScoped(ctx, f, f.tokens["a"], f.storeA1, "pricing:write", func(tx pgx.Tx, scope platform.Scope) (pricing.Policy, error) {
			return pricing.SetPolicy(ctx, tx, scope, replayKey, replayInput)
		})
		if err != nil {
			t.Fatalf("create replay policy: %v", err)
		}
		replayedPolicy, err := pricingScoped(ctx, f, f.tokens["a"], f.storeA1, "pricing:write", func(tx pgx.Tx, scope platform.Scope) (pricing.Policy, error) {
			return pricing.SetPolicy(ctx, tx, scope, replayKey, replayInput)
		})
		if err != nil || replayedPolicy != firstReplay {
			t.Fatalf("policy replay=%+v err=%v", replayedPolicy, err)
		}
		changedReplay := replayInput
		changedReplay.ConfigurationRef = "changed operator evidence"
		_, err = pricingScoped(ctx, f, f.tokens["a"], f.storeA1, "pricing:write", func(tx pgx.Tx, scope platform.Scope) (pricing.Policy, error) {
			return pricing.SetPolicy(ctx, tx, scope, replayKey, changedReplay)
		})
		if !errors.Is(err, command.ErrConflict) {
			t.Fatalf("policy changed-body replay err=%v", err)
		}
		otherActor := seedPricingActor(t, f, f.tenantA, f.storeA1, true)
		_, err = pricingScoped(ctx, f, otherActor, f.storeA1, "pricing:write", func(tx pgx.Tx, scope platform.Scope) (pricing.Policy, error) {
			return pricing.SetPolicy(ctx, tx, scope, replayKey, replayInput)
		})
		if !errors.Is(err, command.ErrConflict) {
			t.Fatalf("policy other-actor replay err=%v", err)
		}
		hundred := int64(100)
		updatedInput := policyInput
		updatedInput.ExpectedVersion, updatedInput.ShippingMinor, updatedInput.Enabled = 1, &hundred, true
		updated, err := pricingScoped(ctx, f, f.tokens["a"], f.storeA1, "pricing:write", func(tx pgx.Tx, scope platform.Scope) (pricing.Policy, error) {
			return pricing.SetPolicy(ctx, tx, scope, t04Key("policy-update"), updatedInput)
		})
		if err != nil || updated.Version != 2 || updated.ShippingMinor != 100 {
			t.Fatalf("updated policy=%+v err=%v", updated, err)
		}
		var v1Shipping, v2Shipping int64
		if err := f.owner.QueryRow(ctx, `SELECT max(shipping_minor) FILTER (WHERE version=1),max(shipping_minor) FILTER (WHERE version=2)
			FROM pricing.policy_versions WHERE market_id=$1`, policyMarket.ID).Scan(&v1Shipping, &v2Shipping); err != nil || v1Shipping != 0 || v2Shipping != 100 {
			t.Fatalf("immutable versions v1=%d v2=%d err=%v", v1Shipping, v2Shipping, err)
		}
		disabledInput := updatedInput
		disabledInput.ExpectedVersion, disabledInput.Enabled = 2, false
		disabled, err := pricingScoped(ctx, f, f.tokens["a"], f.storeA1, "pricing:write", func(tx pgx.Tx, scope platform.Scope) (pricing.Policy, error) {
			return pricing.SetPolicy(ctx, tx, scope, t04Key("policy-disable"), disabledInput)
		})
		if err != nil || disabled.Version != 3 || disabled.Enabled {
			t.Fatalf("disabled policy=%+v err=%v", disabled, err)
		}
		_, err = pricingScoped(ctx, f, f.tokens["a"], f.storeA1, "pricing:read", func(tx pgx.Tx, scope platform.Scope) (pricing.Policy, error) {
			return pricing.LockCurrent(ctx, tx, scope.TenantID, scope.StoreID, policyMarket.ID, "US", "home")
		})
		if !errors.Is(err, command.ErrNotFound) {
			t.Fatalf("disabled current policy err=%v", err)
		}
	})

	t.Run("audit failures roll back receipt market and policy", func(t *testing.T) {
		mustExec(t, f.owner, `CREATE FUNCTION pricing.test_fail_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action IN ('pricing.market.created','pricing.policy.set') THEN RAISE EXCEPTION 'synthetic pricing audit failure'; END IF; RETURN NEW; END $$`)
		mustExec(t, f.owner, `CREATE TRIGGER pricing_test_fail_audit BEFORE INSERT ON ops.audit_events FOR EACH ROW EXECUTE FUNCTION pricing.test_fail_audit()`)
		defer func() {
			mustExec(t, f.owner, `DROP TRIGGER pricing_test_fail_audit ON ops.audit_events`)
			mustExec(t, f.owner, `DROP FUNCTION pricing.test_fail_audit()`)
		}()

		failedCode, failedKey := "failed_"+t04Tag(), t04Key("market-audit")
		_, err := pricingScoped(ctx, f, f.tokens["a"], f.storeA1, "pricing:write", func(tx pgx.Tx, scope platform.Scope) (pricing.Market, error) {
			return pricing.CreateMarket(ctx, tx, scope, failedKey, pricing.MarketInput{Code: failedCode, Name: "Must roll back", Currency: "USD"})
		})
		if err == nil || countRows(t, f.owner, `SELECT count(*) FROM pricing.markets WHERE code=$1`, failedCode) != 0 || countRows(t, f.owner, `SELECT count(*) FROM ops.command_results WHERE idempotency_key=$1`, failedKey) != 0 {
			t.Fatalf("market audit rollback err=%v", err)
		}

		// Use the already-existing policy market and a new country key, so only the policy audit fails.
		policyKey := t04Key("policy-audit")
		failedPolicy := policyInput
		failedPolicy.Country = "CA"
		_, err = pricingScoped(ctx, f, f.tokens["a"], f.storeA1, "pricing:write", func(tx pgx.Tx, scope platform.Scope) (pricing.Policy, error) {
			return pricing.SetPolicy(ctx, tx, scope, policyKey, failedPolicy)
		})
		if err == nil || countRows(t, f.owner, `SELECT count(*) FROM pricing.policy_versions WHERE market_id=$1 AND country='CA'`, policyMarket.ID) != 0 || countRows(t, f.owner, `SELECT count(*) FROM pricing.policy_heads WHERE market_id=$1 AND country='CA'`, policyMarket.ID) != 0 || countRows(t, f.owner, `SELECT count(*) FROM ops.command_results WHERE idempotency_key=$1`, policyKey) != 0 {
			t.Fatalf("policy audit rollback err=%v", err)
		}
	})

	t.Run("buyer reads public policy fields but cannot read private fields or write", func(t *testing.T) {
		buyerPolicy := policyInput
		buyerPolicy.Country = "GB"
		if _, err := pricingScoped(ctx, f, f.tokens["a"], f.storeA1, "pricing:write", func(tx pgx.Tx, scope platform.Scope) (pricing.Policy, error) {
			return pricing.SetPolicy(ctx, tx, scope, t04Key("buyer-policy"), buyerPolicy)
		}); err != nil {
			t.Fatalf("create buyer-readable policy: %v", err)
		}
		authorities := openBuyerTestPools(t, f)
		service, err := buyer.New(authorities.issuer, time.Hour)
		if err != nil {
			t.Fatalf("new buyer service: %v", err)
		}
		capability := mustIssue(t, service, f.storeA1)
		err = buyer.WithScope(ctx, authorities.runtime, capability.Token, f.storeA1, func(buyerCtx context.Context, tx pgx.Tx, scope buyer.Scope) error {
			seen, err := pricing.LockMarket(buyerCtx, tx, scope.TenantID, scope.StoreID, policyMarket.ID)
			if err != nil || seen.ID != policyMarket.ID {
				return fmt.Errorf("buyer lock public market: market=%+v err=%w", seen, err)
			}
			current, err := pricing.LockCurrent(buyerCtx, tx, scope.TenantID, scope.StoreID, policyMarket.ID, "GB", "home")
			if err != nil || current.Country != "GB" || !current.Enabled {
				return fmt.Errorf("buyer lock public policy: policy=%+v err=%w", current, err)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("buyer public pricing read err=%v", err)
		}
		err = buyer.WithScope(ctx, authorities.runtime, capability.Token, f.storeA1, func(buyerCtx context.Context, tx pgx.Tx, scope buyer.Scope) error {
			var private string
			return tx.QueryRow(buyerCtx, `SELECT configuration_ref FROM pricing.policy_versions WHERE market_id=$1 LIMIT 1`, policyMarket.ID).Scan(&private)
		})
		requirePGCode(t, err, "42501", "buyer private policy field")
		err = buyer.WithScope(ctx, authorities.runtime, capability.Token, f.storeA1, func(buyerCtx context.Context, tx pgx.Tx, scope buyer.Scope) error {
			_, err := tx.Exec(buyerCtx, `INSERT INTO pricing.markets(tenant_id,store_id,code,name,currency,principal_id) VALUES($1,$2,'buyer_write','denied','USD',$3)`, scope.TenantID, scope.StoreID, randomUUID())
			return err
		})
		requirePGCode(t, err, "42501", "buyer pricing insert")
	})

	t.Run("inactive market remains lockable", func(t *testing.T) {
		inactive, err := createPricingMarket(ctx, f, "inactive")
		if err != nil {
			t.Fatal(err)
		}
		inactive, err = pricingScoped(ctx, f, f.tokens["a"], f.storeA1, "pricing:write", func(tx pgx.Tx, scope platform.Scope) (pricing.Market, error) {
			return pricing.SetMarketActive(ctx, tx, scope, t04Key("inactive"), inactive.ID, inactive.Version, false)
		})
		if err != nil || inactive.Active {
			t.Fatalf("deactivate market=%+v err=%v", inactive, err)
		}
		locked, err := pricingScoped(ctx, f, f.tokens["a"], f.storeA1, "pricing:read", func(tx pgx.Tx, scope platform.Scope) (pricing.Market, error) {
			return pricing.LockMarket(ctx, tx, scope.TenantID, scope.StoreID, inactive.ID)
		})
		if err != nil || locked.Active {
			t.Fatalf("lock inactive market=%+v err=%v", locked, err)
		}
	})
}

func pricingFixture(t *testing.T) *testFixture {
	t.Helper()
	f := t04Fixture(t)
	pricingGrantOnce.Do(func() {
		_, pricingGrantErr = f.owner.Exec(context.Background(), `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission)
			VALUES($1,$2,$3,'pricing:read'),($1,$2,$3,'pricing:write') ON CONFLICT DO NOTHING`, f.tenantA, f.storeA1, f.principalA)
	})
	if pricingGrantErr != nil {
		t.Fatalf("seed pricing permissions: %v", pricingGrantErr)
	}
	return f
}

func pricingScoped[T any](ctx context.Context, f *testFixture, token, store, permission string, fn func(pgx.Tx, platform.Scope) (T, error)) (T, error) {
	return t04Scoped(ctx, f, token, store, permission, fn)
}

func createPricingMarket(ctx context.Context, f *testFixture, prefix string) (pricing.Market, error) {
	return pricingScoped(ctx, f, f.tokens["a"], f.storeA1, "pricing:write", func(tx pgx.Tx, scope platform.Scope) (pricing.Market, error) {
		return pricing.CreateMarket(ctx, tx, scope, t04Key("market"), pricing.MarketInput{Code: prefix + "_" + t04Tag(), Name: prefix + " market", Currency: "USD"})
	})
}

func seedPricingActor(t *testing.T, f *testFixture, tenantID, storeID string, write bool) string {
	t.Helper()
	principal, token := randomUUID(), randomToken()
	mustExec(t, f.owner, `INSERT INTO identity.principals(id) VALUES($1)`, principal)
	mustExec(t, f.owner, `INSERT INTO identity.memberships(tenant_id,principal_id) VALUES($1,$2)`, tenantID, principal)
	mustExec(t, f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'store:read'),($1,$2,$3,'pricing:read')`, tenantID, storeID, principal)
	if write {
		mustExec(t, f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'pricing:write')`, tenantID, storeID, principal)
	}
	tx, err := f.owner.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if err := insertSession(context.Background(), tx, token, principal, "merchant", time.Now().Add(time.Hour), nil); err != nil {
		t.Fatalf("insert pricing actor session: %v", err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatalf("commit pricing actor session: %v", err)
	}
	return token
}
