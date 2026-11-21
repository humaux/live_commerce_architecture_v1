package foundation_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/platform"
)

func TestNewInitialStoreOwnerGetsAccountSettingsAndExecution(t *testing.T) {
	s, _, _ := identityFixture(t)
	f := fixture(t)
	ctx := context.Background()
	session := identityLogin(t, s)
	request := firstStoreRequest()
	const key = "initial-account-settings-v1"
	store, err := s.CreateInitialStore(ctx, session.Token, key, request)
	if err != nil {
		t.Fatal(err)
	}

	var grants []string
	if err := f.owner.QueryRow(ctx, `SELECT ARRAY(SELECT permission FROM identity.store_grants
		WHERE tenant_id=$1::uuid AND store_id=$2::uuid AND principal_id=$3::uuid
		AND permission LIKE 'integration:%' ORDER BY permission)`, store.TenantID, store.StoreID, session.PrincipalID).Scan(&grants); err != nil {
		t.Fatal(err)
	}
	// Ruling 24 (spec change of stripe-refund-v1 §12): 0065 also grants integration:execute, which the
	// claim-source definer requires with live:manage so the owner can bind their own posts.
	if !slices.Equal(grants, []string{"integration:execute", "integration:manage", "integration:read"}) {
		t.Fatalf("new owner integration grants = %v", grants)
	}
	for _, permission := range []string{"integration:read", "integration:manage", "integration:execute"} {
		if err := platform.WithScope(ctx, f.runtime, session.Token, store.StoreID, permission,
			func(_ pgx.Tx, scope platform.Scope) error {
				if scope.TenantID != store.TenantID || scope.PrincipalID != session.PrincipalID {
					return errors.New("wrong authenticated account scope")
				}
				return nil
			}); err != nil {
			t.Fatalf("new owner missing %s: %v", permission, err)
		}
	}

	var owner string
	var definer, publicExec, identityExec bool
	var config []string
	if err := f.owner.QueryRow(ctx, `SELECT pg_get_userbyid(proowner),prosecdef,proconfig,
		EXISTS(SELECT 1 FROM aclexplode(COALESCE(proacl,acldefault('f',proowner))) a
		 WHERE a.grantee=0 AND a.privilege_type='EXECUTE'),
		has_function_privilege('commerce_identity','identity.create_initial_store(bytea,text,bytea,text,text,text,text)'::regprocedure,'EXECUTE')
		FROM pg_proc WHERE oid='identity.create_initial_store(bytea,text,bytea,text,text,text,text)'::regprocedure`).
		Scan(&owner, &definer, &config, &publicExec, &identityExec); err != nil {
		t.Fatal(err)
	}
	if owner != "commerce_identity_writer" || !definer || publicExec || !identityExec ||
		!slices.Equal(config, []string{"search_path=pg_catalog"}) {
		t.Fatal("initial-store function authority changed")
	}

	if _, err := f.owner.Exec(ctx, `DELETE FROM identity.store_grants
		WHERE tenant_id=$1::uuid AND store_id=$2::uuid AND principal_id=$3::uuid
		AND permission IN ('integration:read','integration:manage','integration:execute')`, store.TenantID, store.StoreID, session.PrincipalID); err != nil {
		t.Fatal(err)
	}
	for _, permission := range []string{"integration:read", "integration:manage", "integration:execute"} {
		if err := platform.WithScope(ctx, f.runtime, session.Token, store.StoreID, permission,
			func(pgx.Tx, platform.Scope) error { return nil }); !errors.Is(err, platform.ErrForbidden) {
			t.Fatalf("revoked %s remained usable: %v", permission, err)
		}
	}
	replay, err := s.CreateInitialStore(ctx, session.Token, key, request)
	if err != nil || replay != store {
		t.Fatalf("idempotent replay after revocation: store=%+v err=%v", replay, err)
	}
	if err := f.owner.QueryRow(ctx, `SELECT ARRAY(SELECT permission FROM identity.store_grants
		WHERE tenant_id=$1::uuid AND store_id=$2::uuid AND principal_id=$3::uuid
		AND permission LIKE 'integration:%' ORDER BY permission)`, store.TenantID, store.StoreID, session.PrincipalID).Scan(&grants); err != nil {
		t.Fatal(err)
	}
	if len(grants) != 0 {
		t.Fatalf("replay restored revoked grants: %v", grants)
	}
}

func TestNewTWDStoreHasFourManualCVSServices(t *testing.T) {
	s, _, _ := identityFixture(t)
	f := fixture(t)
	ctx := context.Background()
	session := identityLogin(t, s)
	req := firstStoreRequest()
	req.Currency = "TWD"
	store, err := s.CreateInitialStore(ctx, session.Token, "onboard-default-cvs", req)
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	err = f.owner.QueryRow(ctx, `SELECT ARRAY(SELECT sv.delivery_kind FROM fulfillment.service_heads h
		JOIN fulfillment.service_versions sv ON (sv.tenant_id,sv.store_id,sv.market_id,sv.country,sv.code,sv.version)=
			(h.tenant_id,h.store_id,h.market_id,h.country,h.code,h.current_version)
		JOIN pricing.policy_versions p ON (p.tenant_id,p.store_id,p.market_id,p.country,p.method,p.version)=
			(sv.tenant_id,sv.store_id,sv.market_id,sv.country,sv.policy_method,sv.policy_version)
		JOIN fulfillment.allocation_heads ah ON (ah.tenant_id,ah.store_id,ah.market_id,ah.country,ah.code)=(h.tenant_id,h.store_id,h.market_id,h.country,h.code)
		WHERE h.tenant_id=$1 AND h.store_id=$2 AND sv.mode='MANUAL' AND sv.enabled AND sv.visible
			AND sv.binding_id IS NULL AND p.enabled AND p.shipping_minor=0 AND h.country='TW' ORDER BY sv.delivery_kind)`, store.TenantID, store.StoreID).Scan(&kinds)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(kinds, []string{"cvs_711", "cvs_familymart", "cvs_hilife", "cvs_okmart"}) {
		t.Fatalf("fresh TWD store selectable manual brands = %v", kinds)
	}
}

func TestManualCVSInitializerScope(t *testing.T) {
	s, _, _ := identityFixture(t)
	f := fixture(t)
	ctx := context.Background()
	session := identityLogin(t, s)
	req := firstStoreRequest()
	req.Currency = "USD"
	store, err := s.CreateInitialStore(ctx, session.Token, "onboard-usd-cvs", req)
	if err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM pricing.markets WHERE tenant_id=$1 AND store_id=$2`, store.TenantID, store.StoreID); n != 0 {
		t.Fatalf("USD markets created: %d", n)
	}
	for _, role := range []string{"commerce_identity", "commerce_runtime", "commerce_buyer_runtime", "commerce_checkout_runtime"} {
		var reachable bool
		if err := f.owner.QueryRow(ctx, `SELECT has_function_privilege($1,'fulfillment.seed_manual_cvs(uuid,uuid,uuid)','EXECUTE') OR
			has_function_privilege($1,'fulfillment.initial_store_manual_cvs()','EXECUTE') OR pg_has_role($1,'commerce_manual_cvs_initializer','MEMBER')`, role).Scan(&reachable); err != nil || reachable {
			t.Fatalf("initializer reachable by %s: %t %v", role, reachable, err)
		}
	}
}
