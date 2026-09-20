package foundation_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/platform"
)

// These SQL probes bypass the HTTP repository's WHERE clauses deliberately:
// authorization must not accidentally substitute for testing the RLS policies.
func TestDirectSQLCannotEscapeResolvedStore(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	err := platform.WithScope(ctx, f.runtime, f.tokens["a"], f.storeA1, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
		var visible int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM control.stores`).Scan(&visible); err != nil {
			return err
		}
		if visible != 1 {
			t.Fatalf("unfiltered scoped stores = %d, want 1", visible)
		}
		for _, store := range []string{f.storeA2, f.storeB} {
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM control.stores WHERE id=$1`, store).Scan(&visible); err != nil {
				return err
			}
			if visible != 0 {
				t.Fatalf("foreign store leaked: count=%d", visible)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, tenant, store string }{
		{"same tenant other store", f.tenantA, f.storeA2},
		{"other tenant", f.tenantB, f.storeB},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := platform.WithScope(ctx, f.runtime, f.tokens["a"], f.storeA1, "audit:write", func(tx pgx.Tx, scope platform.Scope) error {
				_, err := tx.Exec(ctx, `INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES($1,$2,$3,'audit.rejected')`, tc.tenant, tc.store, scope.PrincipalID)
				return err
			})
			if sqlState(err) != "42501" {
				t.Fatalf("foreign-scope insert state=%q, want RLS denial 42501", sqlState(err))
			}
		})
	}
}

func TestAuditHTTPIsBoundedAndReadinessUsesDatabase(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	if err := platform.WithScope(ctx, f.runtime, f.tokens["a"], f.storeA1, "audit:write", func(tx pgx.Tx, scope platform.Scope) error {
		_, err := tx.Exec(ctx, `INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
			SELECT $1::uuid,$2::uuid,$3::uuid,'audit.page' FROM generate_series(1,60)`, scope.TenantID, scope.StoreID, scope.PrincipalID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	handler := platform.NewHandler(f.runtime)
	ready := httptest.NewRecorder()
	handler.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if ready.Code != http.StatusOK {
		t.Fatalf("real database readyz = %d, want 200", ready.Code)
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/stores/"+f.storeA1+"/audit-events?limit=999999", nil)
	req.Header.Set("Authorization", "Bearer "+f.tokens["a"])
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("audit status=%d, want 200", res.Code)
	}
	var rows []map[string]string
	if err := json.Unmarshal(res.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 50 {
		t.Fatalf("audit limit=%d, want 50", len(rows))
	}
	for _, row := range rows {
		if len(row) != 3 || row["id"] == "" || row["action"] == "" {
			t.Fatal("audit response did not match the three-field contract")
		}
		if _, err := time.Parse(time.RFC3339Nano, row["created_at"]); err != nil {
			t.Fatal("audit timestamp is not RFC3339")
		}
	}
	assertSafeHTTPResponse(t, res, f.tokens["a"])
}
