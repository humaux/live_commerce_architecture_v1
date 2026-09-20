package foundation_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
	"livecommerce/migrations"
)

func TestT04ActorAndAttestationCannotBeForged(t *testing.T) {
	f := t04Fixture(t)
	stock := t04CreateStock(t, f, f.tokens["a"], f.storeA1, 2)
	ctx := context.Background()
	var otherPrincipal string
	if err := f.owner.QueryRow(ctx, `SELECT principal_id::text FROM identity.sessions WHERE token_hash=$1`, tokenHash(f.tokens["a2"])).Scan(&otherPrincipal); err != nil {
		t.Fatal(err)
	}
	for name, sql := range map[string]string{
		"replay actor": `INSERT INTO ops.command_results(tenant_id,store_id,operation,idempotency_key,request_hash,response,principal_id) VALUES($1,$2,'review.actor',$3,decode(repeat('00',32),'hex'),'{}',$4)`,
		"price actor":  `INSERT INTO catalog.price_history(tenant_id,store_id,sku_id,version,price_minor,currency,principal_id) VALUES($1,$2,$3,1001,100,'USD',$4)`,
	} {
		t.Run(name, func(t *testing.T) {
			third := stock.skus[0].ID
			if name == "replay actor" {
				third = t04Key("forged")
			}
			err := platform.WithScope(ctx, f.runtime, f.tokens["a"], f.storeA1, "catalog:write", func(tx pgx.Tx, s platform.Scope) error {
				_, err := tx.Exec(ctx, sql, s.TenantID, s.StoreID, third, otherPrincipal)
				return err
			})
			if sqlState(err) != "42501" {
				t.Fatalf("forged actor accepted: %v", err)
			}
		})
	}
	err := platform.WithScope(ctx, f.runtime, f.tokens["a"], f.storeA1, "catalog:write", func(tx pgx.Tx, s platform.Scope) error {
		_, err := tx.Exec(ctx, `INSERT INTO catalog.skus(tenant_id,store_id,product_id,code,currency,price_minor,hs_confirmed,hs_confirmed_by,hs_confirmed_at)
		VALUES($1,$2,$3,$4,'USD',100,'123456',$5,clock_timestamp())`, s.TenantID, s.StoreID, stock.product.ID, "forged-"+t04Tag(), s.PrincipalID)
		return err
	})
	if sqlState(err) != "42501" {
		t.Fatalf("HS attestation injected: %v", err)
	}
}

func TestT04CommandWaitsForReplayBeyondRowLockTimeout(t *testing.T) {
	f := t04Fixture(t)
	ctx := context.Background()
	key := t04Key("slow-replay")
	started := make(chan struct{})
	done := make(chan error, 1)
	type response struct {
		ID string `json:"id"`
	}
	go func() {
		done <- platform.WithScope(ctx, f.runtime, f.tokens["a"], f.storeA1, "catalog:write", func(tx pgx.Tx, s platform.Scope) error {
			var out response
			return command.Run(ctx, tx, s, "review.slow", key, struct{ Value int }{1}, &out, func() error {
				close(started)
				if _, err := tx.Exec(ctx, `SELECT pg_sleep(1.2)`); err != nil {
					return err
				}
				out.ID = "stable-result"
				return command.Audit(ctx, tx, s, "review.slow")
			})
		})
	}()
	select {
	case <-started:
	case <-time.After(4 * time.Second):
		t.Fatal("first command did not start")
	}
	var second response
	err := platform.WithScope(ctx, f.runtime, f.tokens["a"], f.storeA1, "catalog:write", func(tx pgx.Tx, s platform.Scope) error {
		err := command.Run(ctx, tx, s, "review.slow", key, struct{ Value int }{1}, &second, func() error { return errors.New("duplicate callback executed") })
		if err != nil {
			return err
		}
		var restored string
		if err = tx.QueryRow(ctx, `SHOW lock_timeout`).Scan(&restored); err != nil {
			return err
		}
		if restored != "1s" {
			return fmt.Errorf("row timeout not restored: %s", restored)
		}
		return nil
	})
	if firstErr := <-done; firstErr != nil {
		t.Fatal(firstErr)
	}
	if err != nil || second.ID != "stable-result" {
		t.Fatalf("replay failed: %+v %v", second, err)
	}
	var count int
	if err = f.owner.QueryRow(ctx, `SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND operation='review.slow' AND idempotency_key=$3`, f.tenantA, f.storeA1, key).Scan(&count); err != nil || count != 1 {
		t.Fatalf("receipt count %d %v", count, err)
	}
}

func TestT04InvalidCommandResultDoesNotCallBusiness(t *testing.T) {
	f := t04Fixture(t)
	ctx := context.Background()
	var nilPointer *struct{ ID string }
	for _, invalid := range []any{nil, nilPointer, struct{ ID string }{}, "value"} {
		called := false
		err := platform.WithScope(ctx, f.runtime, f.tokens["a"], f.storeA1, "catalog:write", func(tx pgx.Tx, s platform.Scope) error {
			return command.Run(ctx, tx, s, "review.invalid", t04Key("result"), nil, invalid, func() error { called = true; return nil })
		})
		if !errors.Is(err, command.ErrInvalid) || called {
			t.Fatalf("invalid result had effect: err=%v called=%v", err, called)
		}
	}
}

func TestT04MigrationRejectsNewerDatabaseLedger(t *testing.T) {
	f := t04Fixture(t)
	ctx := context.Background()
	// This owner write is confined to the disposable local fixture, never a real DB.
	if _, err := f.owner.Exec(ctx, `INSERT INTO public.lc_schema_migrations(version,checksum) VALUES('9999_future.sql','fixture-only')`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := f.owner.Exec(context.Background(), `DELETE FROM public.lc_schema_migrations WHERE version='9999_future.sql' AND checksum='fixture-only'`); err != nil {
			t.Error(err)
		}
	})
	var before, after []byte
	if err := f.owner.QueryRow(ctx, `SELECT jsonb_agg(to_jsonb(m) ORDER BY version) FROM public.lc_schema_migrations m`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	err := migrations.Apply(ctx, f.owner)
	if err == nil || !strings.Contains(err.Error(), "unknown to this binary") {
		t.Fatalf("newer DB not rejected: %v", err)
	}
	if err = f.owner.QueryRow(ctx, `SELECT jsonb_agg(to_jsonb(m) ORDER BY version) FROM public.lc_schema_migrations m`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	var b, a any
	if json.Unmarshal(before, &b) != nil || json.Unmarshal(after, &a) != nil {
		t.Fatal("invalid ledger JSON")
	}
	if string(before) != string(after) {
		t.Fatal("rejected migration mutated ledger")
	}
}
