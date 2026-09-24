package foundation_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/buyer"
	"livecommerce/internal/command"
	"livecommerce/internal/fulfillment"
	"livecommerce/internal/platform"
	"livecommerce/internal/pricing"
	"livecommerce/internal/storefront"
)

// Reuse the real buyer/cart/pricing fixture; none of these tests call a carrier.
func dsSetup(t *testing.T) (cqHarness, fulfillment.ServiceInput) {
	t.Helper()
	h := cqSetup(t)
	mustExec(t, h.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission)
	 VALUES($1,$2,$3,'integration:manage'),($1,$2,$3,'integration:read') ON CONFLICT DO NOTHING`, h.f.tenantA, h.f.storeA1, h.f.principalA)
	in := fulfillment.ServiceInput{MarketID: h.market.ID, Country: "TW", Code: "home_" + t04Tag(), PolicyVersion: 1,
		NameHans: "标准配送", NameHant: "標準配送", NameEN: "Standard delivery", DeliveryKind: "home", Mode: "MANUAL", Enabled: true, Visible: true, SortOrder: 10}
	dsPolicy(t, h, in, 0, 50, true)
	return h, in
}

func dsPolicy(t *testing.T, h cqHarness, in fulfillment.ServiceInput, expected, fee int64, enabled bool) pricing.Policy {
	t.Helper()
	p := h.policy
	p.Method, p.ExpectedVersion, p.ShippingMinor, p.Enabled = "delivery:"+in.Code, expected, &fee, enabled
	out, err := h.setPolicy(p)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func dsSet(h cqHarness, key string, in fulfillment.ServiceInput) (fulfillment.Service, error) {
	return t04Scoped(context.Background(), h.f, h.f.tokens["a"], h.f.storeA1, "integration:manage", func(tx pgx.Tx, s platform.Scope) (fulfillment.Service, error) {
		return fulfillment.SetService(context.Background(), tx, s, h.f.tokens["a"], key, in)
	})
}

func dsGet(h cqHarness, in fulfillment.ServiceInput) (fulfillment.Service, error) {
	return t04Scoped(context.Background(), h.f, h.f.tokens["a"], h.f.storeA1, "integration:read", func(tx pgx.Tx, s platform.Scope) (fulfillment.Service, error) {
		return fulfillment.GetService(context.Background(), tx, s, h.f.tokens["a"], in.MarketID, in.Country, in.Code)
	})
}

func dsCount(t *testing.T, h cqHarness) int {
	t.Helper()
	return countRows(t, h.f.owner, `SELECT count(*) FROM fulfillment.service_versions WHERE market_id=$1`, h.market.ID)
}

func TestDeliveryServiceRevisionsReplayAndIndependentFees(t *testing.T) {
	h, in := dsSetup(t)
	if _, err := dsGet(h, in); !errors.Is(err, command.ErrNotFound) {
		t.Fatalf("missing service: %v", err)
	}
	key := t04Key("delivery-set")
	first, err := dsSet(h, key, in)
	if err != nil || first.Version != 1 || first.PolicyMethod != "delivery:"+in.Code || first.Currency != h.market.Currency {
		t.Fatalf("create: %+v %v", first, err)
	}
	read, err := dsGet(h, in)
	if err != nil || !reflect.DeepEqual(first, read) {
		t.Fatalf("readback: %+v %v", read, err)
	}
	changed := in
	changed.ExpectedVersion, changed.Visible = 1, false
	second, err := dsSet(h, t04Key("delivery-update"), changed)
	if err != nil || second.Version != 2 || second.Visible || !second.Enabled {
		t.Fatalf("independent visibility: %+v %v", second, err)
	}
	replay, err := dsSet(h, key, in)
	if err != nil || !reflect.DeepEqual(first, replay) {
		t.Fatalf("historical replay: %+v %v", replay, err)
	}
	if _, err = dsSet(h, key, changed); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("changed replay: %v", err)
	}
	if _, err = dsSet(h, t04Key("delivery-stale"), changed); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("CAS: %v", err)
	}

	other := in
	other.Code = "home_" + t04Tag()
	dsPolicy(t, h, other, 0, 130, true)
	if _, err = dsSet(h, t04Key("delivery-other"), other); err != nil {
		t.Fatal(err)
	}
	cart := h.oneCart(t)
	for _, tc := range []struct {
		code string
		fee  int64
	}{{in.Code, 50}, {other.Code, 130}} {
		q, err := cqBuyer(h.a.runtime, h.cap, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) (storefront.Quote, error) {
			return storefront.CreateQuote(ctx, tx, s, t04Key("delivery-quote"), storefront.QuoteInput{CartVersion: cart.Version, MarketID: h.market.ID, Country: "TW", Method: "delivery:" + tc.code})
		})
		if err != nil || q.Policy.ShippingMinor != tc.fee {
			t.Fatalf("independent fee %s: %+v %v", tc.code, q, err)
		}
	}
	if dsCount(t, h) != 3 {
		t.Fatal("unexpected history count")
	}
}

func TestDeliveryServiceStopAfterPolicyOrMarketChange(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		policyOff, marketOff bool
	}{
		{"policy-only", true, false}, {"market-only", false, true}, {"both", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, in := dsSetup(t)
			if _, err := dsSet(h, t04Key("delivery-first"), in); err != nil {
				t.Fatal(err)
			}
			dsPolicy(t, h, in, 1, 75, true)
			in.ExpectedVersion, in.PolicyVersion = 1, 2
			if _, err := dsSet(h, t04Key("delivery-reprice"), in); err != nil {
				t.Fatal(err)
			}
			if tc.policyOff {
				dsPolicy(t, h, in, 2, 90, false)
			}
			if tc.marketOff {
				_, err := pricingScoped(context.Background(), h.f, h.f.tokens["a"], h.f.storeA1, "pricing:write", func(tx pgx.Tx, s platform.Scope) (pricing.Market, error) {
					return pricing.SetMarketActive(context.Background(), tx, s, t04Key("delivery-market-off"), h.market.ID, h.market.Version, false)
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			stop := in
			stop.ExpectedVersion, stop.Enabled, stop.Visible = 2, false, false
			for _, mutate := range []func(*fulfillment.ServiceInput){
				func(v *fulfillment.ServiceInput) { v.PolicyVersion = 1 },
				func(v *fulfillment.ServiceInput) { v.NameEN = "Changed while stopping" },
				func(v *fulfillment.ServiceInput) { v.SortOrder++ },
				func(v *fulfillment.ServiceInput) { v.Enabled = true },
			} {
				bad := stop
				mutate(&bad)
				if _, err := dsSet(h, t04Key("delivery-stop-bad"), bad); !errors.Is(err, command.ErrConflict) {
					t.Fatalf("non-stop mutation should conflict: %v", err)
				}
			}
			got, err := dsSet(h, t04Key("delivery-stop"), stop)
			if err != nil || got.Enabled || got.Visible || got.Version != 3 || got.PolicyVersion != 2 {
				t.Fatalf("precise stop: %+v %v", got, err)
			}
			if dsCount(t, h) != 3 {
				t.Fatal("failed attempts wrote history")
			}
		})
	}
}

func TestDeliveryServiceValidationDraftAndAuthority(t *testing.T) {
	h, in := dsSetup(t)
	for name, mutate := range map[string]func(*fulfillment.ServiceInput){
		"api-enable":     func(v *fulfillment.ServiceInput) { v.Mode = "API" },
		"manual-binding": func(v *fulfillment.ServiceInput) { v.BindingID, v.BindingVersion = randomUUID(), 1 },
		"orphan-version": func(v *fulfillment.ServiceInput) { v.BindingVersion = 1 },
		"code":           func(v *fulfillment.ServiceInput) { v.Code = "other:unsafe" },
		"blank":          func(v *fulfillment.ServiceInput) { v.NameEN = "  " },
		"unicode-blank":  func(v *fulfillment.ServiceInput) { v.NameEN = "\u2003\u3000" },
		"invalid-utf8":   func(v *fulfillment.ServiceInput) { v.NameHans = string([]byte{0xff}) },
		"non-printable":  func(v *fulfillment.ServiceInput) { v.NameHant = "label\u200b" },
		"control":        func(v *fulfillment.ServiceInput) { v.NameHant = "bad\nlabel" },
		"length":         func(v *fulfillment.ServiceInput) { v.NameHans = strings.Repeat("字", 121) },
		"sort":           func(v *fulfillment.ServiceInput) { v.SortOrder = 1001 },
		"version":        func(v *fulfillment.ServiceInput) { v.ExpectedVersion = -1 },
		"policy":         func(v *fulfillment.ServiceInput) { v.PolicyVersion = 0 },
		"kind":           func(v *fulfillment.ServiceInput) { v.DeliveryKind = "unknown" },
		"cvs-country":    func(v *fulfillment.ServiceInput) { v.Country, v.DeliveryKind = "US", "cvs_711" },
	} {
		t.Run(name, func(t *testing.T) {
			bad := in
			mutate(&bad)
			if _, err := dsSet(h, t04Key("delivery-invalid"), bad); !errors.Is(err, command.ErrInvalid) {
				t.Fatalf("validation: %v", err)
			}
		})
	}
	if dsCount(t, h) != 0 {
		t.Fatal("invalid attempts wrote history")
	}
	api := in
	api.Mode, api.Enabled = "API", false
	before := countRows(t, h.f.owner, `SELECT count(*) FROM integration.operations WHERE tenant_id=$1`, h.f.tenantA)
	if got, err := dsSet(h, t04Key("delivery-api-draft"), api); err != nil || got.Enabled || got.Mode != "API" {
		t.Fatalf("draft: %+v %v", got, err)
	}
	if after := countRows(t, h.f.owner, `SELECT count(*) FROM integration.operations WHERE tenant_id=$1`, h.f.tenantA); after != before {
		t.Fatal("saving draft scheduled provider work")
	}
	foreignBinding := randomUUID()
	mustExec(t, h.f.owner, `INSERT INTO integration.bindings(id,tenant_id,store_id,principal_id,provider,external_asset_id) VALUES($1,$2,$3,$4,'test_shipping','synthetic')`, foreignBinding, h.f.tenantA, h.f.storeA2, h.f.principalA)
	api.ExpectedVersion, api.BindingID, api.BindingVersion = 1, foreignBinding, 1
	if _, err := dsSet(h, t04Key("delivery-foreign-binding"), api); !errors.Is(err, command.ErrNotFound) {
		t.Fatalf("foreign binding: %v", err)
	}
	localBinding := randomUUID()
	mustExec(t, h.f.owner, `INSERT INTO integration.bindings(id,tenant_id,store_id,principal_id,provider,external_asset_id,enabled) VALUES($1,$2,$3,$4,'test_shipping','synthetic',false)`, localBinding, h.f.tenantA, h.f.storeA1, h.f.principalA)
	api.BindingID = localBinding
	if got, err := dsSet(h, t04Key("delivery-bound-draft"), api); err != nil || got.Version != 2 || got.Enabled {
		t.Fatalf("disabled binding draft: %+v %v", got, err)
	}
	mustExec(t, h.f.owner, `UPDATE integration.bindings SET semantic_version=2 WHERE id=$1`, localBinding)
	api.ExpectedVersion = 2
	if _, err := dsSet(h, t04Key("delivery-stale-binding"), api); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("stale binding version: %v", err)
	}
	api.BindingVersion = 2
	if got, err := dsSet(h, t04Key("delivery-current-binding"), api); err != nil || got.Version != 3 || got.Enabled {
		t.Fatalf("current binding draft: %+v %v", got, err)
	}
	limited := seedPricingActor(t, h.f, h.f.tenantA, h.f.storeA1, false)
	_, err := t04Scoped(context.Background(), h.f, limited, h.f.storeA1, "store:read", func(tx pgx.Tx, s platform.Scope) (fulfillment.Service, error) {
		return fulfillment.SetService(context.Background(), tx, s, limited, t04Key("delivery-denied"), in)
	})
	if !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("permission bypass: %v", err)
	}
	_, err = t04Scoped(context.Background(), h.f, h.f.tokens["a"], h.f.storeA1, "integration:manage", func(tx pgx.Tx, s platform.Scope) (fulfillment.Service, error) {
		s.StoreID = h.f.storeA2
		return fulfillment.SetService(context.Background(), tx, s, h.f.tokens["a"], t04Key("delivery-forged"), in)
	})
	if !errors.Is(err, command.ErrInvalid) {
		t.Fatalf("forged typed scope must fail before RLS lookup: %v", err)
	}
	_, err = cqBuyer(h.a.runtime, h.cap, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) (int, error) {
		var n int
		err := tx.QueryRow(ctx, `SELECT count(*) FROM fulfillment.service_versions`).Scan(&n)
		return n, err
	})
	if err == nil {
		t.Fatal("ordinary buyer read merchant service binding data")
	}
}

func TestDeliveryServiceAuthorityBeforeReplay(t *testing.T) {
	h, in := dsSetup(t)
	ctx := context.Background()
	key := t04Key("delivery-actor")
	if _, err := dsSet(h, key, in); err != nil {
		t.Fatal(err)
	}
	other := seedPricingActor(t, h.f, h.f.tenantA, h.f.storeA1, false)
	var otherPrincipal string
	if err := h.f.owner.QueryRow(ctx, `SELECT principal_id::text FROM identity.sessions WHERE token_hash=$1`, tokenHash(other)).Scan(&otherPrincipal); err != nil {
		t.Fatal(err)
	}
	mustExec(t, h.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'integration:manage')`, h.f.tenantA, h.f.storeA1, otherPrincipal)
	_, err := t04Scoped(ctx, h.f, other, h.f.storeA1, "integration:manage", func(tx pgx.Tx, s platform.Scope) (fulfillment.Service, error) {
		return fulfillment.SetService(ctx, tx, s, other, key, in)
	})
	if !errors.Is(err, command.ErrConflict) {
		t.Fatalf("cross actor replay: %v", err)
	}
	_, err = t04Scoped(ctx, h.f, other, h.f.storeA1, "integration:manage", func(tx pgx.Tx, s platform.Scope) (fulfillment.Service, error) {
		return fulfillment.GetService(ctx, tx, s, other, in.MarketID, in.Country, in.Code)
	})
	if !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("Get without read grant: %v", err)
	}
	// Alter the actual transaction context independently from the typed scope.
	// Exact classification prevents an unrelated RLS not-found from passing this gate.
	for field, value := range map[string]string{"app.tenant_id": h.f.tenantB, "app.store_id": h.f.storeA2, "app.principal_id": otherPrincipal} {
		t.Run(field, func(t *testing.T) {
			_, err := t04Scoped(ctx, h.f, h.f.tokens["a"], h.f.storeA1, "integration:manage", func(tx pgx.Tx, s platform.Scope) (fulfillment.Service, error) {
				if _, err := tx.Exec(ctx, `SELECT set_config($1,$2,true)`, field, value); err != nil {
					return fulfillment.Service{}, err
				}
				return fulfillment.SetService(ctx, tx, s, h.f.tokens["a"], key, in)
			})
			if !errors.Is(err, command.ErrInvalid) {
				t.Fatalf("GUC mismatch replay: %v", err)
			}
		})
	}
	// Legitimate actors in another store/tenant still cannot read or mutate this service.
	for _, tc := range []struct{ token, tenant, store string }{
		{h.f.tokens["a2"], h.f.tenantA, h.f.storeA2}, {h.f.tokens["b"], h.f.tenantB, h.f.storeB},
	} {
		mustExec(t, h.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission)
		 SELECT $1,$2,principal_id,p FROM identity.sessions CROSS JOIN unnest(ARRAY['integration:manage','integration:read']) AS p WHERE token_hash=$3 ON CONFLICT DO NOTHING`, tc.tenant, tc.store, tokenHash(tc.token))
		_, err := t04Scoped(ctx, h.f, tc.token, tc.store, "integration:manage", func(tx pgx.Tx, s platform.Scope) (fulfillment.Service, error) {
			return fulfillment.SetService(ctx, tx, s, tc.token, key, in)
		})
		if !errors.Is(err, command.ErrNotFound) {
			t.Fatalf("cross scope write: %v", err)
		}
		_, err = t04Scoped(ctx, h.f, tc.token, tc.store, "integration:read", func(tx pgx.Tx, s platform.Scope) (fulfillment.Service, error) {
			return fulfillment.GetService(ctx, tx, s, tc.token, in.MarketID, in.Country, in.Code)
		})
		if !errors.Is(err, command.ErrNotFound) {
			t.Fatalf("cross scope read: %v", err)
		}
	}
	mustExec(t, h.f.owner, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='integration:manage'`, h.f.tenantA, h.f.storeA1, h.f.principalA)
	_, err = t04Scoped(ctx, h.f, h.f.tokens["a"], h.f.storeA1, "store:read", func(tx pgx.Tx, s platform.Scope) (fulfillment.Service, error) {
		return fulfillment.SetService(ctx, tx, s, h.f.tokens["a"], key, in)
	})
	if !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("revoked writer replay: %v", err)
	}
	if dsCount(t, h) != 1 {
		t.Fatal("denied authority attempts changed service history")
	}
}

func TestDeliveryServiceConcurrentCASAndReplay(t *testing.T) {
	h, in := dsSetup(t)
	key := t04Key("delivery-concurrent")
	var wg sync.WaitGroup
	errs := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := dsSet(h, key, in); errs <- err }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("replay: %v", err)
		}
	}
	if dsCount(t, h) != 1 {
		t.Fatal("duplicate history")
	}
	errs = make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			next := in
			next.ExpectedVersion, next.NameEN = 1, fmt.Sprintf("Delivery %d", i)
			_, err := dsSet(h, t04Key("delivery-cas"), next)
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	wins := 0
	for err := range errs {
		if err == nil {
			wins++
		} else if !errors.Is(err, command.ErrConflict) {
			t.Fatalf("CAS loser: %v", err)
		}
	}
	if wins != 1 || dsCount(t, h) != 2 {
		t.Fatalf("CAS winners=%d", wins)
	}
}

func TestDeliveryServiceSQLConstraintsAndRollback(t *testing.T) {
	h, in := dsSetup(t)
	first, err := dsSet(h, t04Key("delivery-sql"), in)
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"commerce_worker", "commerce_buyer_issuer", "commerce_identity"} {
		t.Run(role, func(t *testing.T) {
			tx, e := h.f.owner.Begin(context.Background())
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Rollback(context.Background())
			if _, e = tx.Exec(context.Background(), `SET LOCAL ROLE `+role); e != nil {
				t.Fatal(e)
			}
			_, e = tx.Exec(context.Background(), `SELECT * FROM fulfillment.service_versions`)
			var pgErr *pgconn.PgError
			if !errors.As(e, &pgErr) || pgErr.Code != "42501" {
				t.Fatalf("role gained merchant data: %v", e)
			}
		})
	}
	for _, statement := range []string{
		`UPDATE fulfillment.service_versions SET enabled=false WHERE market_id=$1`,
		`DELETE FROM fulfillment.service_versions WHERE market_id=$1`,
	} {
		_, err := t04Scoped(context.Background(), h.f, h.f.tokens["a"], h.f.storeA1, "integration:manage", func(tx pgx.Tx, s platform.Scope) (int, error) {
			_, e := tx.Exec(context.Background(), statement, h.market.ID)
			return 0, e
		})
		if err == nil {
			t.Fatal("history mutation allowed")
		}
	}
	validBinding := randomUUID()
	mustExec(t, h.f.owner, `INSERT INTO integration.bindings(id,tenant_id,store_id,principal_id,provider,external_asset_id) VALUES($1,$2,$3,$4,'test_shipping','synthetic')`, validBinding, h.f.tenantA, h.f.storeA1, h.f.principalA)
	for _, tc := range []struct{ name, change, sqlstate string }{
		{"api-enabled", `mode='API',enabled=true`, "23514"},
		{"blank-name", `name_hans='   '`, "23514"},
		{"control-name", `name_hant='label'||chr(10)`, "23514"},
		{"long-name", `name_en=repeat('字',121)`, "23514"},
		// The binding exists: only the null-pair CHECK, not an unrelated FK, may reject this.
		{"binding-null-version", fmt.Sprintf(`mode='API',enabled=false,binding_id='%s',binding_version=NULL`, validBinding), "23514"},
		{"foreign-policy", `policy_version=999`, "23503"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Owner-only corruption probes run in rolled-back transactions; runtime history stays immutable.
			tx, e := h.f.owner.Begin(context.Background())
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Rollback(context.Background())
			_, e = tx.Exec(context.Background(), `UPDATE fulfillment.service_versions SET `+tc.change+` WHERE market_id=$1`, h.market.ID)
			var pgErr *pgconn.PgError
			if !errors.As(e, &pgErr) || pgErr.Code != tc.sqlstate {
				t.Fatalf("expected exact constraint SQLSTATE %s: %v", tc.sqlstate, e)
			}
		})
	}
	for _, failure := range []struct{ table, when string }{{"ops.audit_events", "NEW.action='fulfillment.service.set'"}, {"ops.command_results", "NEW.operation='fulfillment.service.set'"}} {
		t.Run(failure.table, func(t *testing.T) {
			fn := "delivery_fail_" + t04Tag()
			mustExec(t, h.f.owner, `CREATE FUNCTION fulfillment.`+fn+`() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF `+failure.when+` THEN RAISE EXCEPTION 'synthetic rollback'; END IF; RETURN NEW; END $$`)
			mustExec(t, h.f.owner, `CREATE TRIGGER `+fn+` BEFORE INSERT ON `+failure.table+` FOR EACH ROW EXECUTE FUNCTION fulfillment.`+fn+`()`)
			defer mustExec(t, h.f.owner, `DROP FUNCTION fulfillment.`+fn+`() CASCADE`)
			next := in
			next.ExpectedVersion, next.NameEN = 1, "Fail atomically"
			key := t04Key("delivery-rollback")
			beforeAudit := countRows(t, h.f.owner, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND action='fulfillment.service.set'`, h.f.tenantA)
			if _, e := dsSet(h, key, next); e == nil {
				t.Fatal("injected failure ignored")
			}
			got, e := dsGet(h, in)
			if e != nil || !reflect.DeepEqual(first, got) || dsCount(t, h) != 1 {
				t.Fatalf("partial persisted facts: %+v %v", got, e)
			}
			if countRows(t, h.f.owner, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND action='fulfillment.service.set'`, h.f.tenantA) != beforeAudit ||
				countRows(t, h.f.owner, `SELECT count(*) FROM ops.command_results WHERE idempotency_key=$1`, key) != 0 {
				t.Fatal("failed command left audit or receipt")
			}
		})
	}
}
