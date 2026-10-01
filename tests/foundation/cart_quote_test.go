package foundation_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/buyer"
	"livecommerce/internal/catalog"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
	"livecommerce/internal/pricing"
	"livecommerce/internal/storefront"
)

type cqHarness struct {
	f       *testFixture
	a       buyerTestAuthorities
	service *buyer.Service
	stock   t04Stock
	cap     buyer.Capability
	market  pricing.Market
	policy  pricing.PolicyInput
}

func cqSetup(t *testing.T) cqHarness {
	t.Helper()
	f := pricingFixture(t)
	h := cqHarness{f: f, a: openBuyerTestPools(t, f), stock: t04CreateStock(t, f, f.tokens["a"], f.storeA1, 10, 20)}
	var err error
	h.service, err = buyer.New(h.a.issuer, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	h.cap = mustIssue(t, h.service, f.storeA1)
	h.market, err = createPricingMarket(context.Background(), f, "cart")
	if err != nil {
		t.Fatal(err)
	}
	shipping, tax := int64(50), int64(500)
	h.policy = pricing.PolicyInput{MarketID: h.market.ID, Country: "TW", Method: "cvs_711", Currency: "USD", ShippingMode: "country_flat", ShippingMinor: &shipping, TaxMode: "exclusive", TaxBasis: "goods_and_shipping", TaxRateBPS: &tax, QuoteTTLSeconds: 60, Enabled: true, ConfigurationRef: "synthetic flat rule; not a carrier"}
	if _, err = h.setPolicy(h.policy); err != nil {
		t.Fatal(err)
	}
	return h
}
func (h cqHarness) setPolicy(in pricing.PolicyInput) (pricing.Policy, error) {
	// Historical (pre-0088/0092) fixtures lack columns the current Go names (see lriShims).
	if err := lriAddShims(h.f); err != nil {
		return pricing.Policy{}, err
	}
	return pricingScoped(context.Background(), h.f, h.f.tokens["a"], h.f.storeA1, "pricing:write", func(tx pgx.Tx, s platform.Scope) (pricing.Policy, error) {
		return pricing.SetPolicy(context.Background(), tx, s, t04Key("cq-policy"), in)
	})
}
func cqBuyer[T any](pool *pgxpool.Pool, c buyer.Capability, fn func(context.Context, pgx.Tx, buyer.Scope) (T, error)) (out T, err error) {
	err = buyer.WithScope(context.Background(), pool, c.Token, c.Scope.StoreID, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) error { out, err = fn(ctx, tx, s); return err })
	return out, err
}
func (h cqHarness) cart(key string, in storefront.CartInput) (storefront.Cart, error) {
	return cqBuyer(h.a.runtime, h.cap, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) (storefront.Cart, error) {
		return storefront.SetCart(ctx, tx, s, key, in)
	})
}
func (h cqHarness) quote(key string, version int64) (storefront.Quote, error) {
	return cqBuyer(h.a.runtime, h.cap, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) (storefront.Quote, error) {
		return storefront.CreateQuote(ctx, tx, s, key, storefront.QuoteInput{CartVersion: version, MarketID: h.market.ID, Country: "TW", Method: "cvs_711"})
	})
}
func (h cqHarness) get(c buyer.Capability, id string) (storefront.Quote, error) {
	return cqBuyer(h.a.runtime, c, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) (storefront.Quote, error) {
		return storefront.GetQuote(ctx, tx, s, id)
	})
}
func (h cqHarness) oneCart(t *testing.T) storefront.Cart {
	t.Helper()
	c, e := h.cart(t04Key("cq-cart"), storefront.CartInput{Items: []storefront.Item{{SKUID: h.stock.skus[0].ID, Quantity: 2}}})
	if e != nil {
		t.Fatal(e)
	}
	return c
}

func TestCartReadReplayAndNoInventoryMutation(t *testing.T) {
	h := cqSetup(t)
	count := func(table string) int {
		return countRows(t, h.f.owner, `SELECT count(*) FROM `+table+` WHERE owner_id=$1`, h.cap.Scope.OwnerID)
	}
	before := countRows(t, h.f.owner, `SELECT count(*) FROM inventory.ledger WHERE sku_id=$1`, h.stock.skus[0].ID)
	for i := 0; i < 2; i++ {
		c, e := cqBuyer(h.a.runtime, h.cap, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) (storefront.Cart, error) {
			return storefront.GetCart(ctx, tx, s)
		})
		if e != nil || c.ID != "" || c.Version != 0 || c.Currency != "USD" || len(c.Items) != 0 {
			t.Fatalf("absent cart %+v %v", c, e)
		}
	}
	for _, table := range []string{"storefront.carts", "storefront.events", "buyer.command_results"} {
		if count(table) != 0 {
			t.Fatalf("GET wrote %s", table)
		}
	}
	key := t04Key("cq-cart")
	in := storefront.CartInput{Items: []storefront.Item{{SKUID: h.stock.skus[1].ID, Quantity: 1}, {SKUID: h.stock.skus[0].ID, Quantity: 2}}}
	first, e := h.cart(key, in)
	if e != nil || first.Version != 1 {
		t.Fatalf("create %+v %v", first, e)
	}
	in.Items[0], in.Items[1] = in.Items[1], in.Items[0]
	replay, e := h.cart(key, in)
	if e != nil || !reflect.DeepEqual(first, replay) {
		t.Fatalf("canonical replay %+v %v", replay, e)
	}
	in.Items[0].Quantity = 4
	if _, e = h.cart(key, in); !errors.Is(e, command.ErrConflict) {
		t.Fatalf("changed body %v", e)
	}
	second, e := h.cart(t04Key("cq-cart"), storefront.CartInput{ExpectedVersion: 1, Items: []storefront.Item{{SKUID: h.stock.skus[0].ID, Quantity: 5}}})
	if e != nil || second.Version != 2 || second.Items[0].Quantity != 5 {
		t.Fatalf("absolute %+v %v", second, e)
	}
	if _, e = h.cart(t04Key("cq-stale"), storefront.CartInput{ExpectedVersion: 1}); !errors.Is(e, command.ErrConflict) {
		t.Fatalf("stale %v", e)
	}
	in.Items[0].Quantity = 2
	replay, e = h.cart(key, in)
	if e != nil || !reflect.DeepEqual(first, replay) {
		t.Fatalf("historical replay %+v %v", replay, e)
	}
	if _, e = h.quote(t04Key("cq-quote"), 2); e != nil {
		t.Fatal(e)
	}
	clear, e := h.cart(t04Key("cq-clear"), storefront.CartInput{ExpectedVersion: 2})
	if e != nil || clear.Version != 3 || len(clear.Items) != 0 {
		t.Fatalf("clear %+v %v", clear, e)
	}
	if _, e = h.quote(t04Key("cq-empty"), 3); !errors.Is(e, command.ErrInvalid) {
		t.Fatalf("empty quote %v", e)
	}
	if after := countRows(t, h.f.owner, `SELECT count(*) FROM inventory.ledger WHERE sku_id=$1`, h.stock.skus[0].ID); after != before {
		t.Fatal("cart/quote wrote stock ledger")
	}
	var reserved, onhand int64
	if e = h.f.owner.QueryRow(context.Background(), `SELECT reserved,on_hand FROM inventory.balances WHERE warehouse_id=$1 AND sku_id=$2`, h.stock.warehouse.ID, h.stock.skus[0].ID).Scan(&reserved, &onhand); e != nil || reserved != 0 || onhand != 10 {
		t.Fatalf("stock %d %d %v", reserved, onhand, e)
	}
}

func TestQuoteSnapshotsPolicyChangesAndOwnerPrivacy(t *testing.T) {
	h := cqSetup(t)
	c := h.oneCart(t)
	key := t04Key("cq-quote")
	q, e := h.quote(key, c.Version)
	if e != nil {
		t.Fatal(e)
	}
	if q.Amount.SubtotalMinor != 2500 || q.Amount.ShippingMinor != 50 || q.Amount.TaxMinor != 128 || q.Amount.ShippingTaxMinor != 3 || q.Amount.TotalMinor != 2678 || q.MarketVersion != 1 || q.Policy.Version != 1 || len(q.Lines) != 1 {
		t.Fatalf("quote math %+v", q)
	}
	if q.Lines[0].Name != h.stock.product.Name || q.Lines[0].UnitPriceMinor != 1250 || q.ExpiresAt.Sub(q.CreatedAt) != time.Minute {
		t.Fatalf("snapshot %+v", q)
	}
	_, e = t04Scoped(context.Background(), h.f, h.f.tokens["a"], h.f.storeA1, "catalog:write", func(tx pgx.Tx, s platform.Scope) (catalog.SKU, error) {
		return catalog.SetSKUPrice(context.Background(), tx, s, t04Key("cq-price"), h.stock.skus[0].ID, catalog.PriceInput{ExpectedVersion: 1, PriceMinor: 1500})
	})
	if e != nil {
		t.Fatal(e)
	}
	p := h.policy
	p.ExpectedVersion = 1
	shipping := int64(80)
	p.ShippingMinor = &shipping
	if _, e = h.setPolicy(p); e != nil {
		t.Fatal(e)
	}
	old, e := h.get(h.cap, q.ID)
	if e != nil || !sameQuote(q, old) {
		t.Fatalf("historical snapshot changed %v", e)
	}
	replay, e := h.quote(key, c.Version)
	if e != nil || !sameQuote(q, replay) {
		t.Fatalf("quote replay changed %v", e)
	}
	fresh, e := h.quote(t04Key("cq-quote"), c.Version)
	if e != nil || fresh.Policy.Version != 2 || fresh.Lines[0].SKUVersion != 2 || fresh.Lines[0].UnitPriceMinor != 1500 {
		t.Fatalf("fresh snapshot %+v %v", fresh, e)
	}
	other := mustIssue(t, h.service, h.f.storeA1)
	if _, e = h.get(other, q.ID); !errors.Is(e, command.ErrNotFound) {
		t.Fatalf("cross owner %v", e)
	}
	foreign := mustIssue(t, h.service, h.f.storeB)
	if _, e = h.get(foreign, q.ID); !errors.Is(e, command.ErrNotFound) {
		t.Fatalf("cross tenant %v", e)
	}
	// Synthetic session refresh for the same owner: resource identity persists,
	// while command receipts remain distinct per capability session.
	refreshed := h.cap
	refreshed.Token = randomToken()
	refreshed.Scope.SessionID = randomUUID()
	mustExec(t, h.f.owner, `INSERT INTO buyer.capability_sessions(id,tenant_id,store_id,owner_id,token_hash,expires_at) VALUES($1,$2,$3,$4,$5,clock_timestamp()+interval '1 hour')`, refreshed.Scope.SessionID, refreshed.Scope.TenantID, refreshed.Scope.StoreID, refreshed.Scope.OwnerID, tokenHash(refreshed.Token))
	got, e := h.get(refreshed, q.ID)
	if e != nil || !sameQuote(q, got) {
		t.Fatalf("same owner refreshed session %v", e)
	}
	otherSession := h
	otherSession.cap = refreshed
	newReceipt, e := otherSession.quote(key, c.Version)
	if e != nil || newReceipt.ID == q.ID {
		t.Fatalf("session receipt must be independent %v", e)
	}
	// Seed a logically expired historical row/receipt using owner authority in
	// this disposable fixture; avoid a 60-second wall-clock sleep in every run.
	q.CreatedAt = q.CreatedAt.Add(-time.Hour)
	q.ExpiresAt = q.ExpiresAt.Add(-time.Hour)
	body, _ := json.Marshal(q)
	mustExec(t, h.f.owner, `UPDATE storefront.quotes SET created_at=$2,expires_at=$3,snapshot=$4 WHERE id=$1`, q.ID, q.CreatedAt, q.ExpiresAt, body)
	mustExec(t, h.f.owner, `UPDATE buyer.command_results SET response=$3 WHERE owner_id=$1 AND idempotency_key=$2 AND session_id=$4`, h.cap.Scope.OwnerID, key, body, h.cap.Scope.SessionID)
	expired, e := h.quote(key, c.Version)
	if e != nil || !sameQuote(q, expired) || expired.ExpiresAt.After(time.Now()) {
		t.Fatalf("expired replay extended %v", e)
	}
	p.ExpectedVersion = 2
	p.Enabled = false
	if _, e = h.setPolicy(p); e != nil {
		t.Fatal(e)
	}
	if _, e = h.quote(t04Key("cq-disabled"), c.Version); !errors.Is(e, command.ErrNotFound) {
		t.Fatalf("disabled policy %v", e)
	}
	if _, e = h.get(h.cap, q.ID); e != nil {
		t.Fatalf("disabled policy hid history %v", e)
	}
}
func sameQuote(a, b storefront.Quote) bool {
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	return string(left) == string(right)
}

func TestCartQuoteValidationSQLAndAtomicRollback(t *testing.T) {
	h := cqSetup(t)
	for name, in := range map[string]storefront.CartInput{
		"negative": {ExpectedVersion: -1}, "invalid-id": {Items: []storefront.Item{{SKUID: "bad", Quantity: 1}}},
		"zero":      {Items: []storefront.Item{{SKUID: h.stock.skus[0].ID, Quantity: 0}}},
		"duplicate": {Items: []storefront.Item{{SKUID: h.stock.skus[0].ID, Quantity: 1}, {SKUID: h.stock.skus[0].ID, Quantity: 2}}},
	} {
		if _, e := h.cart(t04Key("cq-invalid"), in); !errors.Is(e, command.ErrInvalid) {
			t.Fatalf("%s %v", name, e)
		}
	}
	if _, e := h.cart(t04Key("cq-missing"), storefront.CartInput{Items: []storefront.Item{{SKUID: randomUUID(), Quantity: 1}}}); !errors.Is(e, command.ErrNotFound) {
		t.Fatalf("missing SKU %v", e)
	}
	// An event failure after all business writes must roll back cart and receipt.
	fn := "cq_fail_" + t04Tag()
	ident := pgx.Identifier{"storefront", fn}.Sanitize()
	mustExec(t, h.f.owner, fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic event failure' USING ERRCODE='P0001'; END $$`, ident))
	mustExec(t, h.f.owner, fmt.Sprintf(`CREATE TRIGGER %s BEFORE INSERT ON storefront.events FOR EACH ROW WHEN (NEW.owner_id='%s'::uuid) EXECUTE FUNCTION %s()`, pgx.Identifier{fn}.Sanitize(), h.cap.Scope.OwnerID, ident))
	t.Cleanup(func() {
		mustExec(t, h.f.owner, fmt.Sprintf(`DROP TRIGGER IF EXISTS %s ON storefront.events`, pgx.Identifier{fn}.Sanitize()))
		mustExec(t, h.f.owner, `DROP FUNCTION `+ident+`()`)
	})
	key := t04Key("cq-rollback")
	_, e := h.cart(key, storefront.CartInput{Items: []storefront.Item{{SKUID: h.stock.skus[0].ID, Quantity: 1}}})
	requirePGCode(t, e, "P0001", "event rollback")
	for _, table := range []string{"storefront.carts", "storefront.cart_lines", "storefront.events", "buyer.command_results"} {
		if countRows(t, h.f.owner, `SELECT count(*) FROM `+table+` WHERE owner_id=$1`, h.cap.Scope.OwnerID) != 0 {
			t.Fatalf("partial write %s", table)
		}
	}
	mustExec(t, h.f.owner, fmt.Sprintf(`DROP TRIGGER %s ON storefront.events`, pgx.Identifier{fn}.Sanitize()))
	c, e := h.cart(key, storefront.CartInput{Items: []storefront.Item{{SKUID: h.stock.skus[0].ID, Quantity: 1}}})
	if e != nil {
		t.Fatal(e)
	}
	q, e := h.quote(t04Key("cq-quote"), c.Version)
	if e != nil {
		t.Fatal(e)
	}
	for name, sql := range map[string]string{
		"catalog write":     `UPDATE catalog.skus SET id=id WHERE id=$1`,
		"quote immutable":   `UPDATE storefront.quotes SET expires_at=expires_at WHERE id=$1`,
		"quote delete":      `DELETE FROM storefront.quotes WHERE id=$1`,
		"receipt immutable": `DELETE FROM buyer.command_results WHERE owner_id=$1`,
		"private policy":    `SELECT configuration_ref FROM pricing.policy_versions WHERE market_id=$1`,
	} {
		e = buyer.WithScope(context.Background(), h.a.runtime, h.cap.Token, h.f.storeA1, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) error {
			_, e := tx.Exec(ctx, sql, q.ID)
			if name == "catalog write" {
				_, e = tx.Exec(ctx, sql, h.stock.skus[0].ID)
			}
			return e
		})
		requirePGCode(t, e, "42501", name)
	}
	assertSQLDenied(t, h.f.runtime, `SELECT * FROM storefront.quotes`)
	assertSQLDenied(t, h.a.issuer, `SELECT * FROM storefront.carts`)
	// Quote event failure does not leave an orphan snapshot or replay receipt.
	mustExec(t, h.f.owner, fmt.Sprintf(`CREATE TRIGGER %s BEFORE INSERT ON storefront.events FOR EACH ROW WHEN (NEW.owner_id='%s'::uuid) EXECUTE FUNCTION %s()`, pgx.Identifier{fn}.Sanitize(), h.cap.Scope.OwnerID, ident))
	quoteKey := t04Key("cq-failed-quote")
	_, e = h.quote(quoteKey, c.Version)
	requirePGCode(t, e, "P0001", "quote event rollback")
	if countRows(t, h.f.owner, `SELECT count(*) FROM storefront.quotes WHERE owner_id=$1`, h.cap.Scope.OwnerID) != 1 || countRows(t, h.f.owner, `SELECT count(*) FROM buyer.command_results WHERE owner_id=$1 AND idempotency_key=$2`, h.cap.Scope.OwnerID, quoteKey) != 0 {
		t.Fatal("orphan quote/receipt")
	}
}

func TestCartConcurrentCASAndPriceLock(t *testing.T) {
	h := cqSetup(t)
	c := h.oneCart(t)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := int64(3); i < 5; i++ {
		wg.Add(1)
		go func(qty int64) {
			defer wg.Done()
			_, e := h.cart(t04Key("cq-race"), storefront.CartInput{ExpectedVersion: 1, Items: []storefront.Item{{SKUID: h.stock.skus[0].ID, Quantity: qty}}})
			results <- e
		}(i)
	}
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for e := range results {
		if e == nil {
			success++
		} else if errors.Is(e, command.ErrConflict) {
			conflict++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("CAS %d/%d", success, conflict)
	}
	// Hold catalog mutation, then prove quote waits and uses the post-lock price,
	// rather than taking a stale preliminary value into its immutable snapshot.
	owner, err := h.f.owner.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Rollback(context.Background())
	_, err = owner.Exec(context.Background(), `UPDATE catalog.skus SET price_minor=1700,version=version+1 WHERE tenant_id=$1 AND store_id=$2 AND id=$3`, h.cap.Scope.TenantID, h.f.storeA1, h.stock.skus[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	appName := "cq_lock_" + t04Tag()
	pool, err := platform.OpenBuyerPool(context.Background(), withApplicationName(t, h.a.runtimeURL, appName))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	locked := h
	locked.a.runtime = pool
	type result struct {
		q storefront.Quote
		e error
	}
	done := make(chan result, 1)
	go func() { q, e := locked.quote(t04Key("cq-lock"), c.Version+1); done <- result{q, e} }()
	waitForDatabaseLock(t, h.f.owner, appName)
	if err = owner.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-done:
		if got.e != nil || got.q.Lines[0].UnitPriceMinor != 1700 || got.q.Lines[0].SKUVersion != 2 {
			t.Fatalf("locked quote %+v %v", got.q, got.e)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("quote did not settle")
	}
}

func TestCartForeignKeysArchivedCatalogAndMarket(t *testing.T) {
	h := cqSetup(t)
	c := h.oneCart(t)
	// This FK fixture owns a fresh foreign store. Do not populate storeB: the
	// earlier catalog regression deliberately asserts its initial empty state.
	foreignTenant, foreignStore, _ := seedBuyerStores(t, h.f)
	foreignProduct, foreignSKU := randomUUID(), randomUUID()
	mustExec(t, h.f.owner, `INSERT INTO catalog.products(tenant_id,store_id,id,name) VALUES($1,$2,$3,'foreign FK fixture')`, foreignTenant, foreignStore, foreignProduct)
	mustExec(t, h.f.owner, `INSERT INTO catalog.skus(tenant_id,store_id,id,product_id,code,currency,price_minor) VALUES($1,$2,$3,$4,'foreign-fk','USD',1)`, foreignTenant, foreignStore, foreignSKU, foreignProduct)
	err := buyer.WithScope(context.Background(), h.a.runtime, h.cap.Token, h.f.storeA1, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) error {
		_, err := tx.Exec(ctx, `INSERT INTO storefront.cart_lines(tenant_id,store_id,owner_id,cart_id,sku_id,quantity) VALUES($1,$2,$3,$4,$5,1)`, s.TenantID, s.StoreID, s.OwnerID, c.ID, foreignSKU)
		return err
	})
	requirePGCode(t, err, "23503", "cross-store SKU FK")
	_, err = cqBuyer(h.a.runtime, h.cap, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) (storefront.Quote, error) {
		return storefront.CreateQuote(ctx, tx, s, t04Key("cq-no-policy"), storefront.QuoteInput{CartVersion: 1, MarketID: h.market.ID, Country: "TW", Method: "cvs_familymart"})
	})
	if !errors.Is(err, command.ErrNotFound) {
		t.Fatalf("missing policy must not infer zero %v", err)
	}
	_, err = pricingScoped(context.Background(), h.f, h.f.tokens["a"], h.f.storeA1, "pricing:write", func(tx pgx.Tx, s platform.Scope) (pricing.Market, error) {
		return pricing.SetMarketActive(context.Background(), tx, s, t04Key("cq-disable"), h.market.ID, 1, false)
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.quote(t04Key("cq-inactive"), 1); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("inactive market %v", err)
	}
	_, err = pricingScoped(context.Background(), h.f, h.f.tokens["a"], h.f.storeA1, "pricing:write", func(tx pgx.Tx, s platform.Scope) (pricing.Market, error) {
		return pricing.SetMarketActive(context.Background(), tx, s, t04Key("cq-enable"), h.market.ID, 2, true)
	})
	if err != nil {
		t.Fatal(err)
	}
	q, err := h.quote(t04Key("cq-before-archive"), 1)
	if err != nil || q.MarketVersion != 3 {
		t.Fatalf("market snapshot %v", err)
	}
	_, err = t04Scoped(context.Background(), h.f, h.f.tokens["a"], h.f.storeA1, "catalog:write", func(tx pgx.Tx, s platform.Scope) (catalog.SKU, error) {
		return catalog.ArchiveSKU(context.Background(), tx, s, t04Key("cq-archive"), h.stock.skus[0].ID, 1)
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.quote(t04Key("cq-after-archive"), 1); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("archived quote %v", err)
	}
	if _, err = h.cart(t04Key("cq-after-archive"), storefront.CartInput{ExpectedVersion: 1, Items: c.Items}); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("archived cart %v", err)
	}
	got, err := h.get(h.cap, q.ID)
	if err != nil || !sameQuote(q, got) {
		t.Fatalf("archive changed history %v", err)
	}
}

func TestQuoteCartMarketPolicyRaceSnapshots(t *testing.T) {
	for _, target := range []string{"cart", "market-version", "market-disabled", "policy-version", "policy-disabled"} {
		t.Run(target, func(t *testing.T) {
			h := cqSetup(t)
			c := h.oneCart(t)
			owner, err := h.f.owner.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer owner.Rollback(context.Background())
			expectedCart := c.Version
			switch target {
			case "cart":
				_, err = owner.Exec(context.Background(), `UPDATE storefront.carts SET version=version+1 WHERE owner_id=$1`, h.cap.Scope.OwnerID)
				if err == nil {
					_, err = owner.Exec(context.Background(), `UPDATE storefront.cart_lines SET quantity=3 WHERE owner_id=$1`, h.cap.Scope.OwnerID)
				}
				expectedCart++
			case "market-version", "market-disabled":
				_, err = owner.Exec(context.Background(), `UPDATE pricing.markets SET version=version+1,active=$2 WHERE id=$1`, h.market.ID, target != "market-disabled")
			case "policy-version", "policy-disabled":
				_, err = owner.Exec(context.Background(), `INSERT INTO pricing.policy_versions(tenant_id,store_id,market_id,country,method,version,currency,shipping_mode,shipping_minor,tax_mode,tax_basis,tax_rate_bps,quote_ttl_seconds,enabled,configuration_ref,principal_id)
				 SELECT tenant_id,store_id,market_id,country,method,2,currency,shipping_mode,80,tax_mode,tax_basis,tax_rate_bps,quote_ttl_seconds,$2,configuration_ref,principal_id FROM pricing.policy_versions WHERE market_id=$1 AND version=1`, h.market.ID, target != "policy-disabled")
				if err == nil {
					_, err = owner.Exec(context.Background(), `UPDATE pricing.policy_heads SET current_version=2 WHERE market_id=$1`, h.market.ID)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			appName := "cq_" + strings.ReplaceAll(target, "-", "_") + "_" + t04Tag()
			pool, err := platform.OpenBuyerPool(context.Background(), withApplicationName(t, h.a.runtimeURL, appName))
			if err != nil {
				t.Fatal(err)
			}
			defer pool.Close()
			locked := h
			locked.a.runtime = pool
			type result struct {
				q storefront.Quote
				e error
			}
			done := make(chan result, 1)
			go func() { q, e := locked.quote(t04Key("cq-race"), expectedCart); done <- result{q, e} }()
			waitForDatabaseLock(t, h.f.owner, appName)
			if err = owner.Commit(context.Background()); err != nil {
				t.Fatal(err)
			}
			select {
			case got := <-done:
				if target == "market-disabled" || target == "policy-disabled" {
					want := command.ErrConflict
					if target == "policy-disabled" {
						want = command.ErrNotFound
					}
					if !errors.Is(got.e, want) {
						t.Fatalf("disabled race %v", got.e)
					}
					if countRows(t, h.f.owner, `SELECT count(*) FROM storefront.quotes WHERE owner_id=$1`, h.cap.Scope.OwnerID) != 0 {
						t.Fatal("disabled dependency left quote")
					}
					return
				}
				if got.e != nil {
					t.Fatal(got.e)
				}
				switch target {
				case "cart":
					if got.q.CartVersion != 2 || got.q.Lines[0].Quantity != 3 {
						t.Fatalf("torn cart snapshot %+v", got.q)
					}
				case "market-version":
					if got.q.MarketVersion != 2 {
						t.Fatalf("stale market %+v", got.q)
					}
				case "policy-version":
					if got.q.Policy.Version != 2 || got.q.Amount.ShippingMinor != 80 {
						t.Fatalf("stale policy %+v", got.q)
					}
				}
			case <-time.After(6 * time.Second):
				t.Fatal("quote race did not settle")
			}
		})
	}
}

func TestBuyerConcurrentSameKeyReplay(t *testing.T) {
	h := cqSetup(t)
	key := t04Key("cq-duplicate")
	in := storefront.CartInput{Items: []storefront.Item{{SKUID: h.stock.skus[0].ID, Quantity: 2}}}
	type result struct {
		c storefront.Cart
		e error
	}
	done := make(chan result, 8)
	for i := 0; i < 8; i++ {
		go func() { c, e := h.cart(key, in); done <- result{c, e} }()
	}
	id := ""
	for i := 0; i < 8; i++ {
		select {
		case got := <-done:
			if got.e != nil {
				t.Fatal(got.e)
			}
			if id == "" {
				id = got.c.ID
			}
			if got.c.ID != id || got.c.Version != 1 {
				t.Fatalf("nonidentical replay %+v", got.c)
			}
		case <-time.After(6 * time.Second):
			t.Fatal("duplicate did not settle")
		}
	}
	for _, table := range []string{"storefront.carts", "storefront.cart_lines", "storefront.events", "buyer.command_results"} {
		if countRows(t, h.f.owner, `SELECT count(*) FROM `+table+` WHERE owner_id=$1`, h.cap.Scope.OwnerID) != 1 {
			t.Fatalf("duplicate side effect %s", table)
		}
	}
}

func TestBuyerCommandGuardsAndReceiptRollback(t *testing.T) {
	h := cqSetup(t)
	for _, kind := range []string{"nil", "non-pointer", "bad-key", "bad-operation", "wrong-session", "large-request", "callback-error", "large-result"} {
		t.Run(kind, func(t *testing.T) {
			called := false
			key := t04Key("cq-command")
			var value string
			err := buyer.WithScope(context.Background(), h.a.runtime, h.cap.Token, h.f.storeA1, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) error {
				var out any = &value
				var request any = "input"
				op := "test.guard"
				switch kind {
				case "nil":
					out = nil
				case "non-pointer":
					out = "x"
				case "bad-key":
					key = "x"
				case "bad-operation":
					op = "!"
				case "wrong-session":
					s.SessionID = randomUUID()
				case "large-request":
					request = strings.Repeat("x", 65537)
				}
				return buyer.RunCommand(ctx, tx, s, op, key, request, out, func() error {
					called = true
					_, e := tx.Exec(ctx, `INSERT INTO storefront.carts(tenant_id,store_id,owner_id,creator_session_id,currency) VALUES($1,$2,$3,$4,'USD')`, s.TenantID, s.StoreID, s.OwnerID, s.SessionID)
					if e != nil {
						return e
					}
					if kind == "callback-error" {
						return command.ErrConflict
					}
					if kind == "large-result" {
						value = strings.Repeat("x", 1048577)
					}
					return nil
				})
			})
			want := command.ErrInvalid
			if kind == "callback-error" {
				want = command.ErrConflict
			}
			if !errors.Is(err, want) {
				t.Fatalf("guard err %v", err)
			}
			if called != (kind == "callback-error" || kind == "large-result") {
				t.Fatalf("callback ran=%v", called)
			}
			if countRows(t, h.f.owner, `SELECT count(*) FROM storefront.carts WHERE owner_id=$1`, h.cap.Scope.OwnerID) != 0 {
				t.Fatal("failed receipt retained business write")
			}
		})
	}
}
