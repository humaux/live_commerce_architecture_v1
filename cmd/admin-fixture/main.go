// admin-fixture seeds only a caller-created disposable PostgreSQL database.
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/catalog"
	"livecommerce/internal/inventory"
	"livecommerce/internal/platform"
	"livecommerce/migrations"
	"os"
	"time"
)

var codes = []string{"HA-001-BE", "AC-002-BK", "CL-003-SET", "ET-004-S", "ET-004-M", "ET-004-L", "CB-005-TC", "AC-006-GY", "CL-007-BR"}

func id() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = b[6]&15 | 64
	b[8] = b[8]&63 | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
func main() {
	ctx := context.Background()
	dsn := os.Getenv("FIXTURE_OWNER_DSN")
	if dsn == "" {
		panic("missing owner dsn")
	}
	owner, e := pgxpool.New(ctx, dsn)
	if e != nil {
		panic(e)
	}
	defer owner.Close()
	if e = migrations.Apply(ctx, owner); e != nil {
		panic(e)
	}
	tenant, store, principal, token, pass := id(), id(), id(), id()+id(), id()
	_, e = owner.Exec(ctx, "CREATE ROLE foundation_api LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE IN ROLE commerce_runtime PASSWORD '"+pass+"'")
	if e != nil {
		panic(e)
	}
	h := sha256.Sum256([]byte(token))
	_, e = owner.Exec(ctx, `INSERT INTO control.tenants(id,name)VALUES($1,'local-fixture'); INSERT INTO control.stores(tenant_id,id,name,currency)VALUES($1,$2,'助听器配件示例店','TWD'); INSERT INTO identity.principals(id)VALUES($3); INSERT INTO identity.memberships(tenant_id,principal_id)VALUES($1,$3); INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) SELECT $1,$2,$3,x FROM unnest(ARRAY['store:read','catalog:read','catalog:write','inventory:read','inventory:write'])x; INSERT INTO identity.sessions(id,token_hash,principal_id,audience,expires_at)VALUES($4,$5,$3,'merchant',$6)`, tenant, store, principal, id(), h[:], time.Now().Add(24*time.Hour))
	if e != nil {
		panic(e)
	}
	runtime := "postgres://foundation_api:" + pass + "@" + os.Getenv("FIXTURE_HOST") + "/" + os.Getenv("FIXTURE_DB") + "?sslmode=disable"
	pool, e := platform.OpenPool(ctx, runtime)
	if e != nil {
		panic(e)
	}
	defer pool.Close()
	var wh inventory.Warehouse
	e = platform.WithScope(ctx, pool, token, store, "inventory:write", func(tx pgx.Tx, s platform.Scope) error {
		var err error
		wh, err = inventory.CreateWarehouse(ctx, tx, s, "fixture-warehouse", "台北示例仓")
		return err
	})
	if e != nil {
		panic(e)
	}
	for i, code := range codes {
		var p catalog.Product
		e = platform.WithScope(ctx, pool, token, store, "catalog:write", func(tx pgx.Tx, s platform.Scope) error {
			var err error
			p, err = catalog.CreateProduct(ctx, tx, s, fmt.Sprintf("fixture-product-%02d", i), catalog.ProductInput{Name: "助听器配件通用示例", Description: "本地 UI 验收数据"})
			if err != nil {
				return err
			}
			_, err = catalog.CreateSKU(ctx, tx, s, fmt.Sprintf("fixture-sku-%02d", i), catalog.SKUInput{ProductID: p.ID, Code: code, PriceMinor: int64(9900 + i*100), WeightGrams: 10})
			return err
		})
		if e != nil {
			panic(e)
		}
		e = platform.WithScope(ctx, pool, token, store, "inventory:write", func(tx pgx.Tx, s platform.Scope) error {
			_, err := inventory.AdjustOnHand(ctx, tx, s, fmt.Sprintf("fixture-stock-%02d", i), inventory.Adjustment{WarehouseID: wh.ID, SKUID: mustSKU(ctx, pool, token, store, code), Delta: int64(10 + i), ExpectedVersion: 0, Reason: "local fixture"})
			return err
		})
		if e != nil {
			panic(e)
		}
	}
	fmt.Printf("STORE_ID=%s\nTOKEN=%s\nAPI_PASSWORD=%s\n", store, token, pass)
}
func mustSKU(ctx context.Context, p *pgxpool.Pool, t, store, code string) string {
	var id string
	e := platform.WithScope(ctx, p, t, store, "catalog:read", func(tx pgx.Tx, s platform.Scope) error {
		return tx.QueryRow(ctx, `SELECT id::text FROM catalog.skus WHERE code=$1`, code).Scan(&id)
	})
	if e != nil {
		panic(e)
	}
	return id
}
