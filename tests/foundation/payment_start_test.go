package foundation_test

// Purpose: payment start harness (psHarness, psSetup) and PAYUNi start tests, now refusal gates for checkout.start_payment.
// Depends on: buyer/checkout fixtures, pricing/fulfillment setup, p2bStartRetired.
// Used by: go test ./tests/foundation; psHarness used across payment, hosted, expiry and Stripe suites.
// Invariants: fixtures are synthetic; no provider request. PAY-RM2 phase 2b (0197, post_river/0026): see output/pay-rm2-p2b/test-contract-map.md.

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"livecommerce/internal/buyer"
	"livecommerce/internal/checkout"
	"livecommerce/internal/fulfillment"
	"livecommerce/internal/platform"
	"livecommerce/internal/pricing"
	"livecommerce/internal/storefront"
)

type psHarness struct {
	bcHarness
	starter                 *checkout.PaymentStarter
	sql                     *rm2StartSQL
	input                   checkout.PaymentInput
	hold                    checkout.Result
	account, binding, proof string
}

func psStarter(t *testing.T, pool *pgxpool.Pool, profile string) *checkout.PaymentStarter {
	return psStarterIn(t, pool, profile, "river_payment")
}

func psStarterIn(t *testing.T, pool *pgxpool.Pool, profile, schema string) *checkout.PaymentStarter {
	t.Helper()
	jobs, e := river.NewClient(riverpgxv5.New(pool), &river.Config{Schema: schema})
	if e != nil {
		t.Fatal(e)
	}
	s, e := checkout.NewPaymentStarter(context.Background(), pool, jobs, profile)
	if e != nil {
		t.Fatal(e)
	}
	return s
}

// A new synthetic tenant avoids changing the shared USD fixture's currency.
// All identities/keys/proofs are fictional; the task-owned PG container is the
// teardown boundary. No production role can mint this MOCK qualification.
func psSetup(t *testing.T) psHarness {
	return psSetupItems(t, 1)
}
func psSetupItems(t *testing.T, skuCount int) psHarness {
	return psSetupItemsOn(t, fixture(t), skuCount)
}

// Runtime queue audits require an isolated database, not the shared fixture's
// deliberately corrupted/relocated jobs. Domain setup remains identical.
func psSetupItemsOn(t *testing.T, base *testFixture, skuCount int, historical ...string) psHarness {
	t.Helper()
	expirySchema, paymentSchema := "river_expiry", "river_payment"
	if len(historical) == 1 && historical[0] == "river" {
		expirySchema, paymentSchema = "river", "river"
	} else if len(historical) != 0 {
		t.Fatal("unsupported historical River fixture")
	}
	ctx := context.Background()
	f := *base
	f.tenantA, f.storeA1, f.principalA = randomUUID(), randomUUID(), randomUUID()
	f.tokens = map[string]string{"a": randomToken()}
	mustExec(t, f.owner, `INSERT INTO control.tenants(id,name) VALUES($1,'payment mock tenant')`, f.tenantA)
	mustExec(t, f.owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'payment mock store','TWD')`, f.tenantA, f.storeA1)
	mustExec(t, f.owner, `INSERT INTO identity.principals(id) VALUES($1)`, f.principalA)
	mustExec(t, f.owner, `INSERT INTO identity.memberships(tenant_id,principal_id) VALUES($1,$2)`, f.tenantA, f.principalA)
	mustExec(t, f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission)
 SELECT $1,$2,$3,p FROM unnest(ARRAY['store:read','catalog:read','catalog:write','inventory:read','inventory:write','inventory:reserve','pricing:read','pricing:write','integration:manage','integration:read','integration:execute']) p`, f.tenantA, f.storeA1, f.principalA)
	tx, e := f.owner.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if e = insertSession(ctx, tx, f.tokens["a"], f.principalA, "merchant", t06GoFuture(), nil); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	quantities := make([]int64, skuCount)
	for i := range quantities {
		quantities[i] = 10
	}
	h := cqHarness{f: &f, a: openBuyerTestPools(t, &f), stock: t04CreateStock(t, &f, f.tokens["a"], f.storeA1, quantities...)}
	h.service, e = buyer.New(h.a.issuer, time.Hour)
	if e != nil {
		t.Fatal(e)
	}
	h.cap = mustIssue(t, h.service, f.storeA1)
	h.market, e = pricingScoped(ctx, &f, f.tokens["a"], f.storeA1, "pricing:write", func(tx pgx.Tx, s platform.Scope) (pricing.Market, error) {
		return pricing.CreateMarket(ctx, tx, s, t04Key("ps-market"), pricing.MarketInput{Code: "tw", Name: "Synthetic TW", Currency: "TWD"})
	})
	if e != nil {
		t.Fatal(e)
	}
	zero := int64(0)
	h.policy = pricing.PolicyInput{MarketID: h.market.ID, Country: "TW", Currency: "TWD", ShippingMode: "country_flat", ShippingMinor: &zero, TaxMode: "none", TaxBasis: "goods", TaxRateBPS: &zero, QuoteTTLSeconds: 300, Enabled: true, ConfigurationRef: "synthetic only"}
	delivery := fulfillment.ServiceInput{MarketID: h.market.ID, Country: "TW", Code: "home", PolicyVersion: 1, NameHans: "测试配送", NameHant: "測試配送", NameEN: "Mock delivery", DeliveryKind: "home", Mode: "MANUAL", Enabled: true, Visible: true}
	dsPolicy(t, h, delivery, 0, 0, true)
	if _, e = dsSet(h, t04Key("ps-delivery"), delivery); e != nil {
		t.Fatal(e)
	}
	allocation := fulfillment.AllocationInput{MarketID: h.market.ID, Country: "TW", Code: "home", ExpectedServiceVersion: 1, WarehouseIDs: []string{h.stock.warehouse.ID}}
	if _, e = daSet(h, t04Key("ps-allocation"), allocation); e != nil {
		t.Fatal(e)
	}
	poolURL := bcRole(t, &f, "commerce_checkout_runtime")
	pool, e := platform.OpenCheckoutPool(ctx, poolURL)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(pool.Close)
	worker := waOpen(t, &f, waPayment, platform.WorkerPayment)
	expiry := waOpen(t, &f, waExpiry, platform.WorkerExpiry)
	b := bcHarness{cqHarness: h, pool: pool, worker: worker, expiry: expiry, service: bcServiceIn(t, pool, expirySchema), delivery: delivery, allocation: allocation, poolURL: poolURL}
	items := make([]storefront.Item, skuCount)
	for i := range items {
		items[i] = storefront.Item{SKUID: h.stock.skus[skuCount-1-i].ID, Quantity: 2}
	}
	b.prepare(t, h.cap, items)
	hold, e := b.begin(t04Key("ps-hold"))
	if e != nil {
		t.Fatal(e)
	}
	p := psHarness{bcHarness: b, starter: psStarterIn(t, pool, "PROVIDER_MOCK", paymentSchema), hold: hold, account: randomUUID(), binding: randomUUID(), proof: randomUUID(), input: checkout.PaymentInput{OrderID: hold.OrderID, MethodCode: "payuni_credit", MethodVersion: 1}}
	tx, e = f.owner.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	statements := []string{
		`INSERT INTO integration.bindings(id,tenant_id,store_id,principal_id,provider,external_asset_id) VALUES($5,$1,$2,$3,'payuni','SANDBOX:mock-account')`,
		`INSERT INTO integration.merchant_accounts(id,tenant_id,store_id,principal_id,provider,environment,account_id,binding_id,credential_version) VALUES($4,$1,$2,$3,'payuni','SANDBOX','mock-account',$5,1)`,
		`INSERT INTO integration.account_credentials(tenant_id,store_id,connection_id,version,key_id,nonce,ciphertext,principal_id) VALUES($1,$2,$4,1,'mock_key',decode(repeat('00',12),'hex'),decode(repeat('00',17),'hex'),$3)`,
		`INSERT INTO payments.account_qualifications(id,tenant_id,store_id,connection_id,credential_version,environment,code,proof_class,evidence_ref,observed_at,expires_at) VALUES($6,$1,$2,$4,1,'SANDBOX','payuni_credit','PROVIDER_MOCK','disposable fixture; no provider request',clock_timestamp()-interval '1 second',clock_timestamp()+interval '1 hour')`,
		`INSERT INTO payments.method_versions(tenant_id,store_id,market_id,country,code,version,provider,environment,connection_id,binding_version,currency,name_hans,name_hant,name_en,enabled,visible,sort_order,min_amount_minor,max_amount_minor,principal_id,qualification_id) VALUES($1,$2,$7,'TW','payuni_credit',1,'payuni','SANDBOX',$4,1,'TWD','测试','測試','Mock',true,true,0,100,19999900,$3,$6)`,
		`INSERT INTO payments.method_heads(tenant_id,store_id,market_id,country,code,current_version) VALUES($1,$2,$7,'TW','payuni_credit',1)`,
	}
	// Bind all fixture identities with explicit types, including statements which
	// use only a subset. This is test data, not a dynamic production SQL builder.
	for _, sql := range statements {
		if _, e = tx.Exec(ctx, `WITH fixture_args AS (SELECT $1::uuid t,$2::uuid s,$3::uuid p,$4::uuid a,$5::uuid b,$6::uuid q,$7::uuid m) `+sql, f.tenantA, f.storeA1, f.principalA, p.account, p.binding, p.proof, h.market.ID); e != nil {
			t.Fatal(e)
		}
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	return p
}

func (p psHarness) start(key string) (checkout.PaymentResult, error) {
	if p.sql != nil {
		return p.sql.StartPayment(context.Background(), p.cap.Token, p.f.storeA1, key, p.input)
	}
	return p.starter.StartPayment(context.Background(), p.cap.Token, p.f.storeA1, key, p.input)
}
func (p psHarness) facts(t *testing.T) (out [7]int) {
	t.Helper()
	e := p.f.owner.QueryRow(context.Background(), `SELECT
 (SELECT count(*) FROM checkout.payment_attempts WHERE owner_id=$1),
 (SELECT count(*) FROM integration.operations WHERE buyer_owner_id=$1),
 (SELECT count(*) FROM checkout.command_results WHERE owner_id=$1 AND operation='checkout.payment.start'),
 (SELECT count(*) FROM checkout.events WHERE owner_id=$1 AND action='checkout.payment_started'),
 (SELECT count(*) FROM river_payment.river_job WHERE kind='payment_query_v1'),
 (SELECT count(*) FROM checkout.orders WHERE owner_id=$1 AND commercial_state='AWAITING_PAYMENT'),
 (SELECT count(*) FROM inventory.reservations WHERE buyer_owner_id=$1 AND state='PAYMENT_PENDING')`, p.cap.Scope.OwnerID).Scan(&out[0], &out[1], &out[2], &out[3], &out[4], &out[5], &out[6])
	if e != nil {
		t.Fatal(e)
	}
	return
}

func TestBuyerPaymentAtomicReplayAndPendingStock(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bStartRetired(t)
}

func TestBuyerPaymentConcurrentKeys(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bStartRetired(t)
}

func TestBuyerPaymentAdmissionDenials(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bStartRetired(t)
}

func TestBuyerPaymentFaultRollback(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bStartRetired(t)
}

func TestBuyerPaymentFinalWaitGate(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bStartRetired(t)
}

func TestBuyerPaymentAuthorityAndDispatcherFence(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bStartRetired(t)
}

func TestBuyerPaymentQualificationExpiresDuringFinalWrite(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bStartRetired(t)
}

func TestBuyerPaymentFreshSessionAndQualificationAuthority(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bStartRetired(t)
}

func TestBuyerPaymentExpiredHoldRace(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bStartRetired(t)
}
