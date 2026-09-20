package foundation_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/buyer"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
	"livecommerce/internal/storefront"
)

func checkoutQuoteFixture(t *testing.T) (cqHarness, storefront.Cart, storefront.Quote) {
	t.Helper()
	h := cqSetup(t)
	c := h.oneCart(t)
	q, err := h.quote(t04Key("checkout-quote"), c.Version)
	if err != nil {
		t.Fatal(err)
	}
	return h, c, q
}

func checkoutRevalidate(h cqHarness, cap buyer.Capability, id string, version int64) (storefront.Quote, error) {
	return cqBuyer(h.a.runtime, cap, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) (storefront.Quote, error) {
		return storefront.RevalidateQuote(ctx, tx, s, id, version)
	})
}

// Counts only this isolated fixture's business scope; validation may lock rows,
// but must not reserve inventory, mint receipts or enqueue a payment operation.
func checkoutQuoteFacts(t *testing.T, h cqHarness) [6]int {
	t.Helper()
	var out [6]int
	err := h.f.owner.QueryRow(context.Background(), `SELECT
	 (SELECT count(*) FROM buyer.command_results WHERE owner_id=$1),
	 (SELECT count(*) FROM storefront.events WHERE owner_id=$1),
	 (SELECT count(*) FROM storefront.quotes WHERE owner_id=$1),
	 (SELECT count(*) FROM inventory.reservations WHERE tenant_id=$2),
	 (SELECT count(*) FROM inventory.ledger WHERE tenant_id=$2),
	 (SELECT count(*) FROM integration.operations WHERE tenant_id=$2)`, h.cap.Scope.OwnerID, h.cap.Scope.TenantID).
		Scan(&out[0], &out[1], &out[2], &out[3], &out[4], &out[5])
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// Owner-only corruption of a synthetic snapshot exercises distrust of stored
// JSON. Never grant UPDATE to the ordinary buyer or weaken migration checks.
func checkoutPoisonQuote(t *testing.T, h cqHarness, q storefront.Quote) {
	t.Helper()
	b, err := json.Marshal(q)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, h.f.owner, `UPDATE storefront.quotes SET snapshot=$2,created_at=$3,expires_at=$4 WHERE id=$1`, q.ID, b, q.CreatedAt, q.ExpiresAt)
}

func TestCheckoutQuoteMatchingReadOnlyAndAuthority(t *testing.T) {
	h, c, q := checkoutQuoteFixture(t)
	before := checkoutQuoteFacts(t, h)
	out, err := checkoutRevalidate(h, h.cap, q.ID, c.Version)
	if err != nil || !sameQuote(out, q) {
		t.Fatalf("matching quote changed: %v", err)
	}
	other := mustIssue(t, h.service, h.cap.Scope.StoreID)
	foreignStore := mustIssue(t, h.service, h.f.storeA2)
	foreignTenant := mustIssue(t, h.service, h.f.storeB)
	for _, cap := range []buyer.Capability{other, foreignStore, foreignTenant} {
		if _, err = checkoutRevalidate(h, cap, q.ID, c.Version); !errors.Is(err, command.ErrNotFound) {
			t.Fatalf("foreign scope leaked quote: %v", err)
		}
	}
	for _, bad := range []struct {
		id string
		v  int64
	}{{"not-a-uuid", 1}, {q.ID, 0}, {q.ID, -1}} {
		if _, err = checkoutRevalidate(h, h.cap, bad.id, bad.v); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("invalid arguments: %v", err)
		}
	}
	if _, err = checkoutRevalidate(h, h.cap, randomUUID(), c.Version); !errors.Is(err, command.ErrNotFound) {
		t.Fatalf("missing quote: %v", err)
	}
	if _, err = checkoutRevalidate(h, h.cap, q.ID, c.Version+1); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("stale expected version: %v", err)
	}
	err = buyer.WithScope(context.Background(), h.a.runtime, h.cap.Token, h.cap.Scope.StoreID, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) error {
		s.OwnerID = other.Scope.OwnerID
		_, e := storefront.RevalidateQuote(ctx, tx, s, q.ID, c.Version)
		return e
	})
	if !errors.Is(err, command.ErrInvalid) {
		t.Fatalf("typed scope mismatch: %v", err)
	}
	if after := checkoutQuoteFacts(t, h); before != after {
		t.Fatalf("validation wrote facts: %v -> %v", before, after)
	}
}

func TestCheckoutQuoteRejectsCurrentStateDrift(t *testing.T) {
	for _, change := range []string{"cart", "quantity-without-version", "sku-price", "sku-version", "sku-inactive", "product-name", "product-version", "product-inactive", "market-version", "market-inactive", "policy-version", "policy-disabled"} {
		t.Run(change, func(t *testing.T) {
			h, c, q := checkoutQuoteFixture(t)
			switch change {
			case "cart":
				mustExec(t, h.f.owner, `UPDATE storefront.carts SET version=version+1 WHERE id=$1`, c.ID)
			case "quantity-without-version":
				mustExec(t, h.f.owner, `UPDATE storefront.cart_lines SET quantity=quantity+1 WHERE cart_id=$1`, c.ID)
			case "sku-price":
				mustExec(t, h.f.owner, `UPDATE catalog.skus SET price_minor=price_minor+1 WHERE id=$1`, q.Lines[0].SKUID)
			case "sku-version":
				mustExec(t, h.f.owner, `UPDATE catalog.skus SET version=version+1 WHERE id=$1`, q.Lines[0].SKUID)
			case "sku-inactive":
				mustExec(t, h.f.owner, `UPDATE catalog.skus SET status='archived' WHERE id=$1`, q.Lines[0].SKUID)
			case "product-name":
				mustExec(t, h.f.owner, `UPDATE catalog.products SET name=name || ' changed' WHERE id=$1`, q.Lines[0].ProductID)
			case "product-version":
				mustExec(t, h.f.owner, `UPDATE catalog.products SET version=version+1 WHERE id=$1`, q.Lines[0].ProductID)
			case "product-inactive":
				mustExec(t, h.f.owner, `UPDATE catalog.products SET status='archived' WHERE id=$1`, q.Lines[0].ProductID)
			case "market-version":
				mustExec(t, h.f.owner, `UPDATE pricing.markets SET version=version+1 WHERE id=$1`, h.market.ID)
			case "market-inactive":
				mustExec(t, h.f.owner, `UPDATE pricing.markets SET active=false WHERE id=$1`, h.market.ID)
			case "policy-version", "policy-disabled":
				in := h.policy
				in.ExpectedVersion = 1
				in.Enabled = change != "policy-disabled"
				if _, err := h.setPolicy(in); err != nil {
					t.Fatal(err)
				}
			}
			before := checkoutQuoteFacts(t, h)
			_, err := checkoutRevalidate(h, h.cap, q.ID, c.Version)
			if !errors.Is(err, command.ErrConflict) && !errors.Is(err, command.ErrNotFound) {
				t.Fatalf("drift accepted or wrong error: %v", err)
			}
			if out, e := h.get(h.cap, q.ID); e != nil || !sameQuote(out, q) {
				t.Fatalf("historical quote changed: %v", e)
			}
			if after := checkoutQuoteFacts(t, h); before != after {
				t.Fatalf("drift rejection wrote facts: %v -> %v", before, after)
			}
		})
	}
}

func TestCheckoutQuoteRejectsForgedSnapshotAndTime(t *testing.T) {
	for _, change := range []string{"price", "quantity", "line-tax", "line-discount", "discount", "subtotal", "aggregate-tax", "shipping", "shipping-tax", "total", "amount-lines", "empty-lines", "sku-identity", "policy-money", "policy-tax", "name", "description", "malformed-type", "expired", "future", "ttl"} {
		t.Run(change, func(t *testing.T) {
			h, c, q := checkoutQuoteFixture(t)
			switch change {
			case "price":
				q.Lines[0].UnitPriceMinor++
			case "quantity":
				q.Lines[0].Quantity++
			case "line-tax":
				q.Lines[0].Amount.TaxMinor++
			case "line-discount":
				q.Lines[0].Amount.DiscountMinor++
			case "discount":
				q.Amount.DiscountMinor++
			case "subtotal":
				q.Amount.SubtotalMinor++
			case "aggregate-tax":
				q.Amount.TaxMinor++
			case "shipping":
				q.Amount.ShippingMinor++
			case "shipping-tax":
				q.Amount.ShippingTaxMinor++
			case "total":
				q.Amount.TotalMinor--
			case "amount-lines":
				q.Amount.Lines[0].TotalMinor++
			case "empty-lines":
				q.Lines = nil
			case "sku-identity":
				q.Lines[0].SKUID = randomUUID()
			case "policy-money":
				q.Policy.ShippingMinor++
			case "policy-tax":
				q.Policy.TaxRateBPS++
			case "name":
				q.Lines[0].Name = "forged display identity"
			case "description":
				q.Lines[0].Description += " forged"
			case "expired":
				q.CreatedAt = q.CreatedAt.Add(-2 * time.Minute)
				q.ExpiresAt = q.ExpiresAt.Add(-2 * time.Minute)
			case "future":
				q.CreatedAt = q.CreatedAt.Add(2 * time.Minute)
				q.ExpiresAt = q.ExpiresAt.Add(2 * time.Minute)
			case "ttl":
				q.ExpiresAt = q.ExpiresAt.Add(time.Second)
			}
			checkoutPoisonQuote(t, h, q)
			if change == "malformed-type" {
				mustExec(t, h.f.owner, `UPDATE storefront.quotes SET snapshot=jsonb_set(snapshot,'{amount,total_minor}','"not-a-number"'::jsonb) WHERE id=$1`, q.ID)
				if _, err := h.get(h.cap, q.ID); !errors.Is(err, command.ErrConflict) {
					t.Fatalf("historical read exposed malformed JSON error: %v", err)
				}
			}
			before := checkoutQuoteFacts(t, h)
			if _, err := checkoutRevalidate(h, h.cap, q.ID, c.Version); !errors.Is(err, command.ErrConflict) {
				t.Fatalf("corrupted snapshot accepted: %v", err)
			}
			if after := checkoutQuoteFacts(t, h); before != after {
				t.Fatalf("corruption rejection wrote facts: %v -> %v", before, after)
			}
		})
	}
}

func TestCheckoutQuoteRetainsLocksUntilCallerEnds(t *testing.T) {
	for _, target := range []string{"cart", "policy", "product", "sku"} {
		t.Run(target, func(t *testing.T) {
			h, c, q := checkoutQuoteFixture(t)
			release, ready, done := make(chan struct{}), make(chan error, 1), make(chan error, 1)
			go func() {
				done <- buyer.WithScope(context.Background(), h.a.runtime, h.cap.Token, h.cap.Scope.StoreID, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) error {
					_, err := storefront.RevalidateQuote(ctx, tx, s, q.ID, c.Version)
					ready <- err
					if err != nil {
						return err
					}
					select {
					case <-release:
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				})
			}()
			defer func() {
				select {
				case <-release:
				default:
					close(release)
				}
			}()
			if err := waitError(t, ready); err != nil {
				t.Fatal(err)
			}
			writer, err := h.f.owner.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer writer.Rollback(context.Background())
			name := "checkout-quote-lock-" + t04Tag()
			if _, err = writer.Exec(context.Background(), `SELECT set_config('application_name',$1,true)`, name); err != nil {
				t.Fatal(err)
			}
			query, id := `UPDATE storefront.carts SET version=version+1 WHERE id=$1`, c.ID
			switch target {
			case "policy":
				query, id = `UPDATE pricing.policy_heads SET current_version=current_version WHERE market_id=$1`, h.market.ID
			case "product":
				query, id = `UPDATE catalog.products SET version=version+1 WHERE id=$1`, q.Lines[0].ProductID
			case "sku":
				query, id = `UPDATE catalog.skus SET version=version+1 WHERE id=$1`, q.Lines[0].SKUID
			}
			changed := make(chan error, 1)
			go func() { _, e := writer.Exec(context.Background(), query, id); changed <- e }()
			waitForDatabaseLock(t, h.f.owner, name)
			close(release)
			if err = waitError(t, done); err != nil {
				t.Fatal(err)
			}
			if err = waitError(t, changed); err != nil {
				t.Fatal(err)
			}
			if err = writer.Rollback(context.Background()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCheckoutQuoteRechecksClockAndStateAfterWaiting(t *testing.T) {
	for _, outcome := range []string{"expire", "price-change", "cancel"} {
		t.Run(outcome, func(t *testing.T) {
			h, c, q := checkoutQuoteFixture(t)
			if outcome == "expire" {
				var now time.Time
				if err := h.f.owner.QueryRow(context.Background(), `SELECT clock_timestamp()`).Scan(&now); err != nil {
					t.Fatal(err)
				}
				q.ExpiresAt = now.Add(2 * time.Second)
				q.CreatedAt = q.ExpiresAt.Add(-time.Duration(q.Policy.QuoteTTLSeconds) * time.Second)
				checkoutPoisonQuote(t, h, q)
			}
			holder, err := h.f.owner.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer holder.Rollback(context.Background())
			delta := int64(0)
			if outcome == "price-change" {
				delta = 1
			}
			if _, err = holder.Exec(context.Background(), `UPDATE catalog.skus SET price_minor=price_minor+$2 WHERE id=$1`, q.Lines[0].SKUID, delta); err != nil {
				t.Fatal(err)
			}
			name := "checkout-quote-wait-" + t04Tag()
			pool, err := platform.OpenBuyerPool(context.Background(), withApplicationName(t, h.a.runtimeURL, name))
			if err != nil {
				t.Fatal(err)
			}
			defer pool.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				done <- buyer.WithScope(ctx, pool, h.cap.Token, h.cap.Scope.StoreID, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) error {
					if outcome == "expire" {
						// Test-only longer row wait, still inside the production 5s
						// request bound, to prove expiry while actually waiting.
						if _, e := tx.Exec(ctx, `SET LOCAL lock_timeout='3s'`); e != nil {
							return e
						}
					}
					_, e := storefront.RevalidateQuote(ctx, tx, s, q.ID, c.Version)
					return e
				})
			}()
			waitForDatabaseLock(t, h.f.owner, name)
			if outcome == "expire" {
				var stillValid bool
				if err = h.f.owner.QueryRow(context.Background(), `SELECT clock_timestamp()<$1`, q.ExpiresAt).Scan(&stillValid); err != nil || !stillValid {
					t.Fatalf("expiry race did not begin before expiration: %v", err)
				}
				// Use the database clock, not host-clock assumptions or an
				// arbitrary delay that might end before the gate is exercised.
				mustExec(t, h.f.owner, `SELECT pg_sleep(GREATEST(0,extract(epoch FROM $1::timestamptz-clock_timestamp()))+0.02)`, q.ExpiresAt)
			}
			if outcome == "cancel" {
				cancel()
			}
			if err = holder.Commit(context.Background()); err != nil {
				t.Fatal(err)
			}
			err = waitError(t, done)
			if outcome == "cancel" {
				if err == nil {
					t.Fatal("cancelled validation succeeded")
				}
				reused := h
				reused.a.runtime = pool
				if _, err = checkoutRevalidate(reused, h.cap, q.ID, c.Version); err != nil {
					t.Fatalf("connection/scope did not recover: %v", err)
				}
				var clean bool
				if err = pool.QueryRow(context.Background(), `SELECT
				 coalesce(current_setting('app.tenant_id',true),'')='' AND
				 coalesce(current_setting('app.buyer_id',true),'')='' AND
				 coalesce(current_setting('app.buyer_session_id',true),'')=''`).Scan(&clean); err != nil || !clean {
					t.Fatalf("reused pool leaked transaction scope: %v", err)
				}
			} else if !errors.Is(err, command.ErrConflict) {
				t.Fatalf("post-wait validation accepted: %v", err)
			}
		})
	}
}
