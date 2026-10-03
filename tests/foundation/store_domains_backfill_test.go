package foundation_test

// TestStoreDomainsBackfillsPre0106Stores is the N-P1-3 gate: a deploy that adds 0106 must give every store that
// predates it a handle and an ACTIVE platform-subdomain origin in the same migration run — not a separate operator
// one-shot. It builds the schema up to 0105, inserts a handle-less store (the pre-0106 shape), then applies 0106
// with the platform base zone set exactly as cmd/migrate does, and reads the rows back.

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"livecommerce/migrations"
)

func TestStoreDomainsBackfillsPre0106Stores(t *testing.T) {
	ctx := context.Background()
	owner := mciStartPG(t)
	const version = "0106_store_domains.sql"
	const base = "example.com"
	storeID := randomUUID()

	// Build everything up to 0105: pre-mark 0106 in the ledger so Apply skips it (same trick as the R2 gate).
	mustExec(t, owner, `CREATE TABLE public.lc_schema_migrations (version text PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`)
	body, err := os.ReadFile(filepath.Join("../../migrations", version))
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, owner, `INSERT INTO public.lc_schema_migrations(version,checksum) VALUES($1,$2)`, version, fmt.Sprintf("%x", sha256.Sum256(body)))
	if err := migrations.Apply(ctx, owner); err != nil {
		t.Fatalf("apply up to 0105: %v", err)
	}
	// The pre-0106 shape: a real tenant and store with no handle column and no storefront_domains row.
	tenant := randomUUID()
	mustExec(t, owner, `INSERT INTO control.tenants(id,name) VALUES($1,'backfill-tenant')`, tenant)
	mustExec(t, owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'backfill-store','USD')`, tenant, storeID)

	// Apply 0106 with the platform base zone (the LC_STORE_BASE_DOMAIN GUC), the way a deploy runs it.
	mustExec(t, owner, `DELETE FROM public.lc_schema_migrations WHERE version=$1`, version)
	if err := migrations.ApplyWithBaseDomain(ctx, owner, base); err != nil {
		t.Fatalf("apply 0106 with base domain: %v", err)
	}

	var handle string
	if err := owner.QueryRow(ctx, `SELECT handle FROM control.stores WHERE id=$1`, storeID).Scan(&handle); err != nil {
		t.Fatal(err)
	}
	if len(handle) != 8 || handle[0] < '1' || handle[0] > '9' || strings.Trim(handle, "0123456789") != "" {
		t.Fatalf("pre-0106 store must receive a random eight-digit number, got %q", handle)
	}
	var origin, state, evidence string
	var validUntil time.Time
	if err := owner.QueryRow(ctx, `SELECT origin,state,evidence_ref,valid_until FROM control.storefront_domains
		WHERE store_id=$1 AND evidence_ref='platform-subdomain'`, storeID).Scan(&origin, &state, &evidence, &validUntil); err != nil {
		t.Fatalf("pre-0106 store has no platform-subdomain row after 0106: %v", err)
	}
	if origin != "https://"+handle+"."+base || state != "ACTIVE" || evidence != "platform-subdomain" {
		t.Fatalf("platform row = origin %q state %q evidence %q, want https://%s.%s ACTIVE platform-subdomain", origin, state, evidence, handle, base)
	}
	if !validUntil.After(time.Now().Add(24 * time.Hour)) {
		t.Fatalf("platform valid_until = %v, want a long (3650d) validity", validUntil)
	}
}
