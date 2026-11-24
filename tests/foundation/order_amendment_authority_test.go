package foundation_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/platform"
	"livecommerce/migrations"
)

// TestR03RoleUpgrade compares current sessions and new role assignments against
// the effective permissions from a real database through 0230.
func TestR03RoleUpgrade(t *testing.T) {
	const name = "0231_order_amendment.sql"
	ctx := context.Background()
	owner := mciStartPG(t)
	body, err := os.ReadFile("../../migrations/" + name)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, owner, `CREATE TABLE public.lc_schema_migrations (version text PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`)
	mustExec(t, owner, `INSERT INTO public.lc_schema_migrations(version,checksum) VALUES($1,$2)`, name, fmt.Sprintf("%x", sha256.Sum256(body)))
	if err := migrations.Apply(ctx, owner); err != nil {
		t.Fatal(err)
	}
	pass := randomToken()
	mustExec(t, owner, `CREATE ROLE r03_fixture_runtime LOGIN NOSUPERUSER NOBYPASSRLS IN ROLE commerce_runtime PASSWORD '`+pass+`'`)
	cfg := owner.Config()
	cfg.ConnConfig.User, cfg.ConnConfig.Password = "r03_fixture_runtime", pass
	runtime, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Close)
	env := sgSetupOn(t, &testFixture{owner: owner, runtime: runtime, databaseURL: owner.Config().ConnString()})
	roles := []string{"owner", "admin", "live_operator", "fulfilment", "viewer"}
	type member struct{ id, token string }
	makeMember := func(e *sgEnv, role string) member {
		if role == "owner" {
			return member{e.owner.PrincipalID, e.owner.Token}
		}
		id, token, _ := e.join(role)
		return member{id, token}
	}
	check := func(e *sgEnv, stage, role string, m member, want, universe []string) {
		t.Helper()
		var effective []string
		for _, p := range universe {
			if err := e.can(m.token, p); err == nil {
				effective = append(effective, p)
			} else if !errors.Is(err, platform.ErrForbidden) {
				t.Errorf("%s %s resolve %s: %v", stage, role, p, err)
			}
		}
		grants := e.grants(m.id)
		t.Logf("%s %s grants=%v effective=%v", stage, role, grants, effective)
		if !slices.Equal(grants, want) || !slices.Equal(effective, want) {
			t.Errorf("%s %s permission drift, want %v", stage, role, want)
		}
	}
	before, expected := map[string]member{}, map[string][]string{}
	for _, role := range roles {
		before[role] = makeMember(env, role)
		expected[role] = env.grants(before[role].id)
		check(env, "0230", role, before[role], expected[role], env.universe())
	}
	legacyViewer := makeMember(env, "viewer")
	mustExec(t, owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'inbox:read')`,
		env.store.TenantID, env.store.StoreID, legacyViewer.id)
	legacyExplicit := env.grants(legacyViewer.id)
	check(env, "0230-existing-explicit", "viewer", legacyViewer, legacyExplicit, env.universe())

	mustExec(t, owner, `DELETE FROM public.lc_schema_migrations WHERE version=$1`, name)
	if err := migrations.Apply(ctx, owner); err != nil {
		t.Fatal(err)
	}
	for _, role := range roles {
		check(env, "upgraded-existing-session", role, before[role], expected[role], env.universe())
	}
	check(env, "upgraded-existing-explicit", "viewer", legacyViewer, legacyExplicit, env.universe())
	upgraded := sgSetupOn(t, env.f)
	fresh := sgSetup(t)
	for _, role := range roles {
		check(upgraded, "upgraded-new-session", role, makeMember(upgraded, role), expected[role], upgraded.universe())
		check(fresh, "fresh-install", role, makeMember(fresh, role), expected[role], fresh.universe())
	}
	viewer := before["viewer"]
	if err := env.can(viewer.token, "orders:amend"); !errors.Is(err, platform.ErrForbidden) {
		t.Errorf("viewer before explicit grant: %v", err)
	}
	// A separately provisioned grant is visible to the same active session.
	mustExec(t, owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'orders:amend')`,
		env.store.TenantID, env.store.StoreID, viewer.id)
	if err := env.can(viewer.token, "orders:amend"); err != nil {
		t.Errorf("viewer after explicit grant: %v", err)
	}
	check(env, "other-session-after-explicit-grant", "owner", before["owner"], expected["owner"], env.universe())
}
