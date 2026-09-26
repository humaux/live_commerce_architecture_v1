package foundation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/platform"
)

const lmaUnavailable = "media authorization unavailable"

type lmaHarness struct {
	lp          lpHarness
	registrar   *pgxpool.Pool
	login       string
	session     string
	bindings    []string
	authorities []string
	media       string
	facebook    string
	instagram   string
}

func lmaLogin(t *testing.T, f *testFixture, member string) (string, *pgxpool.Pool) {
	t.Helper()
	name := "lma_" + strings.ReplaceAll(randomUUID(), "-", "")
	password := randomToken()
	statement := fmt.Sprintf("CREATE ROLE %s LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION PASSWORD '%s'", pgx.Identifier{name}.Sanitize(), password)
	if member != "" {
		statement += " IN ROLE " + pgx.Identifier{member}.Sanitize()
	}
	if _, err := f.owner.Exec(context.Background(), statement); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(f.databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.UserPassword(name, password)
	p, err := pgxpool.New(context.Background(), u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		p.Close()
		if _, err := f.owner.Exec(context.Background(), "DROP ROLE "+pgx.Identifier{name}.Sanitize()); err != nil {
			t.Errorf("drop synthetic login: %v", err)
		}
	})
	if err := p.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	return name, p
}

func lmaSetup(t *testing.T) *lmaHarness {
	t.Helper()
	h := &lmaHarness{lp: lpSetup(t)}
	h.login, h.registrar = lmaLogin(t, h.lp.f, "commerce_media_registrar")
	draft, err := lpCreate(h.lp, h.lp.token, h.lp.f.storeA1, t04Key("lma-draft"), lpInput("media authorization draft"))
	if err != nil {
		t.Fatal(err)
	}
	h.session = draft.ID
	h.media = h.binding(t, h.lp.f.tenantA, h.lp.f.storeA1, h.lp.actor, "livekit", "project_lma")
	h.facebook = h.binding(t, h.lp.f.tenantA, h.lp.f.storeA1, h.lp.actor, "facebook", "page_lma")
	h.instagram = h.binding(t, h.lp.f.tenantA, h.lp.f.storeA1, h.lp.actor, "instagram", "ig_lma")
	// Registered after lpSetup: this cleanup runs first and preserves FK order.
	t.Cleanup(func() {
		for _, id := range h.authorities {
			for _, q := range []string{
				"DELETE FROM live.media_authorization_revocations WHERE authorization_id=$1",
				"DELETE FROM live.media_authorization_destinations WHERE authorization_id=$1",
				"DELETE FROM live.prepared_media_authorizations WHERE id=$1",
			} {
				if _, err := h.lp.f.owner.Exec(context.Background(), q, id); err != nil {
					t.Errorf("media authority cleanup: %v", err)
				}
			}
		}
		for _, id := range h.bindings {
			if _, err := h.lp.f.owner.Exec(context.Background(), "DELETE FROM integration.bindings WHERE id=$1", id); err != nil {
				t.Errorf("media binding cleanup: %v", err)
			}
		}
	})
	return h
}

func (h *lmaHarness) binding(t *testing.T, tenant, store, principal, provider, asset string) string {
	t.Helper()
	id := randomUUID()
	_, err := h.lp.f.owner.Exec(context.Background(), `INSERT INTO integration.bindings
		(id,tenant_id,store_id,principal_id,provider,external_asset_id)
		VALUES($1,$2,$3,$4,$5,$6)`, id, tenant, store, principal, provider, asset)
	if err != nil {
		t.Fatal(err)
	}
	h.bindings = append(h.bindings, id)
	return id
}

func lmaDestination(id string, provider, asset string) map[string]any {
	return map[string]any{"binding_id": id, "binding_version": int64(1), "provider": provider, "external_asset_id": asset}
}

func (h *lmaHarness) spec() map[string]any {
	id := randomUUID()
	h.authorities = append(h.authorities, id)
	return map[string]any{
		"id": id, "tenant_id": h.lp.f.tenantA, "store_id": h.lp.f.storeA1,
		"session_id": h.session, "attempt_id": randomUUID(), "session_version": int64(1),
		"environment": "MOCK", "project_id": "project_lma",
		"endpoint_identity": "https://unit.livekit.cloud", "credential_version": int64(1),
		"media_binding_id": h.media, "media_binding_version": int64(1),
		"aspect_ratio": "16:9", "evidence_type": "MOCK_FIXTURE",
		"evidence_hash": strings.Repeat("a", 64), "evidence_issuer": "lma_fixture",
		"start_before":         time.Now().UTC().Add(time.Hour).Truncate(time.Second).Format(time.RFC3339),
		"max_duration_seconds": int64(900), "budget_currency": "USD", "budget_minor": int64(10000),
		"material_version": int64(1), "key_id": "lma_key_1",
		"destinations": []any{lmaDestination(h.facebook, "facebook", "page_lma")},
	}
}

func lmaEnvelope() ([]byte, []byte) {
	return bytes.Repeat([]byte{0x31}, 12), bytes.Repeat([]byte{0x42}, 32)
}

func lmaRegister(ctx context.Context, pool *pgxpool.Pool, spec any, nonce, ciphertext []byte) (string, error) {
	data, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}
	var id string
	err = pool.QueryRow(ctx, `SELECT live.register_prepared_media($1::jsonb,$2::bytea,$3::bytea)::text`, string(data), nonce, ciphertext).Scan(&id)
	return id, err
}

func lmaBad(t *testing.T, err error, canary string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "22023" || pgErr.Message != lmaUnavailable || pgErr.Detail != "" || pgErr.Hint != "" || strings.Contains(err.Error(), canary) {
		t.Fatalf("unredacted/nonstatic validation failure: code=%q message=%q detail=%q hint=%q err=%v", sqlState(err), func() string {
			if pgErr == nil {
				return ""
			}
			return pgErr.Message
		}(), func() string {
			if pgErr == nil {
				return ""
			}
			return pgErr.Detail
		}(), func() string {
			if pgErr == nil {
				return ""
			}
			return pgErr.Hint
		}(), err)
	}
}

func lmaCounts(t *testing.T, f *testFixture, id string) (header, children, revoked int) {
	t.Helper()
	err := f.owner.QueryRow(context.Background(), `SELECT
		(SELECT count(*) FROM live.prepared_media_authorizations WHERE id=$1),
		(SELECT count(*) FROM live.media_authorization_destinations WHERE authorization_id=$1),
		(SELECT count(*) FROM live.media_authorization_revocations WHERE authorization_id=$1)`, id).Scan(&header, &children, &revoked)
	if err != nil {
		t.Fatal(err)
	}
	return
}

func TestLiveMediaAuthorizationLMA01PersistenceReplayAndRollback(t *testing.T) {
	h := lmaSetup(t)
	ctx := context.Background()
	nonce, ciphertext := lmaEnvelope()
	spec := h.spec()
	id, err := lmaRegister(ctx, h.registrar, spec, nonce, ciphertext)
	if err != nil || id != spec["id"] {
		t.Fatalf("valid one-destination register: id=%q err=%v", id, err)
	}
	var tenant, store, session, attempt, project, endpoint, registrar, ratio, evidence string
	var sessionVersion, credentialVersion, mediaVersion, materialVersion, duration, budget int64
	var hash, gotNonce, gotCipher []byte
	err = h.lp.f.owner.QueryRow(ctx, `SELECT tenant_id::text,store_id::text,session_id::text,attempt_id::text,
		project_id,endpoint_identity,registered_by,aspect_ratio,evidence_type,session_version,credential_version,
		media_binding_version,material_version,max_duration_seconds,budget_minor,evidence_hash,nonce,ciphertext
		FROM live.prepared_media_authorizations WHERE id=$1`, id).Scan(&tenant, &store, &session, &attempt,
		&project, &endpoint, &registrar, &ratio, &evidence, &sessionVersion, &credentialVersion,
		&mediaVersion, &materialVersion, &duration, &budget, &hash, &gotNonce, &gotCipher)
	if err != nil || tenant != h.lp.f.tenantA || store != h.lp.f.storeA1 || session != h.session || attempt != spec["attempt_id"] || project != "project_lma" || endpoint != "https://unit.livekit.cloud" || registrar != h.login || ratio != "16:9" || evidence != "MOCK_FIXTURE" || sessionVersion != 1 || credentialVersion != 1 || mediaVersion != 1 || materialVersion != 1 || duration != 900 || budget != 10000 || !bytes.Equal(hash, bytes.Repeat([]byte{0xaa}, 32)) || !bytes.Equal(gotNonce, nonce) || !bytes.Equal(gotCipher, ciphertext) {
		t.Fatalf("typed header/provenance mismatch: err=%v tenant=%q store=%q session=%q attempt=%q registrar=%q", err, tenant, store, session, attempt, registrar)
	}
	if a, b, c := lmaCounts(t, h.lp.f, id); a != 1 || b != 1 || c != 0 {
		t.Fatalf("one destination counts=%d,%d,%d", a, b, c)
	}
	// JSONB whitespace/key order is not identity; typed value and envelope are.
	var replayID string
	data, _ := json.MarshalIndent(spec, "", "  ")
	err = h.registrar.QueryRow(ctx, `SELECT live.register_prepared_media($1::jsonb,$2,$3)::text`, string(data), nonce, ciphertext).Scan(&replayID)
	if err != nil || replayID != id {
		t.Fatalf("exact replay id=%q err=%v", replayID, err)
	}
	equivalent := map[string]any{}
	for k, v := range spec {
		equivalent[k] = v
	}
	for field, number := range map[string]string{
		"session_version": "1.0", "credential_version": "1.0",
		"media_binding_version": "1.0", "material_version": "1.0",
		"max_duration_seconds": "900.0", "budget_minor": "10000.0",
	} {
		equivalent[field] = json.Number(number)
	}
	destination := map[string]any{}
	for k, v := range spec["destinations"].([]any)[0].(map[string]any) {
		destination[k] = v
	}
	destination["binding_version"] = json.Number("1.0")
	equivalent["destinations"] = []any{destination}
	if equivalentID, err := lmaRegister(ctx, h.registrar, equivalent, nonce, ciphertext); err != nil || equivalentID != id {
		t.Fatalf("JSONB integral-decimal replay id=%q err=%v", equivalentID, err)
	}
	second := h.spec()
	second["destinations"] = []any{lmaDestination(h.facebook, "facebook", "page_lma"), lmaDestination(h.instagram, "instagram", "ig_lma")}
	secondID, err := lmaRegister(ctx, h.registrar, second, nonce, ciphertext)
	if err != nil || secondID != second["id"] {
		t.Fatalf("two destinations id=%q err=%v", secondID, err)
	}
	rows, err := h.lp.f.owner.Query(ctx, `SELECT ordinal,binding_id::text,provider,external_asset_id
		FROM live.media_authorization_destinations WHERE authorization_id=$1 ORDER BY ordinal`, secondID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for i, want := range []struct{ binding, provider, asset string }{{h.facebook, "facebook", "page_lma"}, {h.instagram, "instagram", "ig_lma"}} {
		if !rows.Next() {
			t.Fatal("missing ordered destination")
		}
		var ordinal int16
		var binding, provider, asset string
		if err := rows.Scan(&ordinal, &binding, &provider, &asset); err != nil || ordinal != int16(i+1) || binding != want.binding || provider != want.provider || asset != want.asset {
			t.Fatalf("destination %d: ordinal=%d binding=%q provider=%q asset=%q err=%v", i, ordinal, binding, provider, asset, err)
		}
	}
	if rows.Next() || rows.Err() != nil {
		t.Fatalf("extra destination or rows error: %v", rows.Err())
	}
	rollback := h.spec()
	tx, err := h.registrar.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	data, _ = json.Marshal(rollback)
	var rollbackID string
	if err = tx.QueryRow(ctx, `SELECT live.register_prepared_media($1::jsonb,$2,$3)::text`, string(data), nonce, ciphertext).Scan(&rollbackID); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if a, b, c := lmaCounts(t, h.lp.f, rollbackID); a != 0 || b != 0 || c != 0 {
		t.Fatal("rollback left authorization rows")
	}
	badChild := h.spec()
	badChild["destinations"] = []any{lmaDestination(h.facebook, "facebook", "page_lma"), lmaDestination(h.instagram, "instagram", "wrong_asset")}
	_, err = lmaRegister(ctx, h.registrar, badChild, nonce, ciphertext)
	lmaBad(t, err, "wrong_asset")
	if a, b, c := lmaCounts(t, h.lp.f, badChild["id"].(string)); a != 0 || b != 0 || c != 0 {
		t.Fatal("invalid second child left partial rows")
	}
}

func TestLiveMediaAuthorizationLMA02ActualRoleBoundary(t *testing.T) {
	h := lmaSetup(t)
	ctx := context.Background()
	nonce, ciphertext := lmaEnvelope()
	spec := h.spec()
	id, err := lmaRegister(ctx, h.registrar, spec, nonce, ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	for _, member := range []string{"", "commerce_runtime", "commerce_worker", "commerce_meta_ingress", "commerce_meta_worker", "commerce_checkout_runtime"} {
		t.Run("role-"+member, func(t *testing.T) {
			_, p := lmaLogin(t, h.lp.f, member)
			data, _ := json.Marshal(spec)
			for _, tc := range []struct {
				q    string
				args []any
			}{
				{`SELECT live.register_prepared_media($1::jsonb,$2,$3)`, []any{string(data), nonce, ciphertext}},
				{`SELECT live.revoke_prepared_media($1::uuid,$2::uuid,$3::uuid,'test')`, []any{h.lp.f.tenantA, h.lp.f.storeA1, id}},
				{`SELECT ciphertext FROM live.prepared_media_authorizations WHERE id=$1::uuid`, []any{id}},
				{`INSERT INTO live.media_authorization_revocations(authorization_id,tenant_id,store_id,reason_code,revoked_by) VALUES($1::uuid,$2::uuid,$3::uuid,'test','forged')`, []any{id, h.lp.f.tenantA, h.lp.f.storeA1}},
				{`UPDATE live.prepared_media_authorizations SET nonce=nonce WHERE id=$1::uuid`, []any{id}},
				{`DELETE FROM live.media_authorization_destinations WHERE authorization_id=$1::uuid`, []any{id}},
			} {
				_, err := p.Exec(ctx, tc.q, tc.args...)
				if sqlState(err) != "42501" {
					t.Fatalf("%s: expected ACL 42501, got %v", tc.q, err)
				}
			}
		})
	}
	for _, q := range []string{
		`SELECT ciphertext FROM live.prepared_media_authorizations WHERE id=$1`,
		`INSERT INTO live.prepared_media_authorizations(id) VALUES($1)`,
		`UPDATE live.prepared_media_authorizations SET nonce=nonce WHERE id=$1`,
		`DELETE FROM live.media_authorization_destinations WHERE authorization_id=$1`,
	} {
		if _, err := h.registrar.Exec(ctx, q, id); sqlState(err) != "42501" {
			t.Fatalf("registrar direct table privilege: %s: %v", q, err)
		}
	}
	var safe bool
	err = h.lp.f.owner.QueryRow(ctx, `WITH funcs AS (
		SELECT p.oid,p.proowner,p.proacl,p.prosecdef,p.proconfig,pg_get_userbyid(p.proowner) AS owner
		FROM pg_proc p WHERE p.oid IN (
		'live.register_prepared_media(jsonb,bytea,bytea)'::regprocedure,
		'live.revoke_prepared_media(uuid,uuid,uuid,text)'::regprocedure))
		SELECT count(*)=2 AND bool_and(prosecdef AND proconfig=ARRAY['search_path=pg_catalog']
		AND owner='commerce_media_writer' AND has_function_privilege('commerce_media_registrar',oid,'EXECUTE')
		AND NOT EXISTS(SELECT 1 FROM aclexplode(coalesce(proacl,acldefault('f',proowner))) acl WHERE acl.grantee=0 AND acl.privilege_type='EXECUTE')
		AND NOT has_function_privilege('commerce_runtime',oid,'EXECUTE')
		AND NOT has_function_privilege('commerce_worker',oid,'EXECUTE')) FROM funcs`).Scan(&safe)
	if err != nil || !safe {
		t.Fatalf("fixed definer ACL: safe=%t err=%v", safe, err)
	}
	err = h.lp.f.owner.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM pg_roles r WHERE r.rolname NOT IN
		('commerce_media_registrar','commerce_media_writer') AND r.rolname NOT LIKE 'pg_%' AND NOT r.rolsuper
		AND pg_has_role(r.oid,to_regrole('commerce_media_writer'),'MEMBER'))
		AND has_schema_privilege('commerce_media_registrar','live','USAGE')
		AND has_schema_privilege('commerce_media_writer','live','USAGE')
		AND has_schema_privilege('commerce_media_writer','control','USAGE')
		AND has_schema_privilege('commerce_media_writer','integration','USAGE')`).Scan(&safe)
	if err != nil || !safe {
		t.Fatalf("writer membership/schema usage: safe=%t err=%v", safe, err)
	}
	if a, b, c := lmaCounts(t, h.lp.f, id); a != 1 || b != 1 || c != 0 {
		t.Fatal("ACL attempts mutated facts")
	}
	for _, member := range []string{"commerce_runtime", "commerce_worker"} {
		t.Run("mixed-registrar-"+member, func(t *testing.T) {
			login, pool := lmaLogin(t, h.lp.f, member)
			if _, err := h.lp.f.owner.Exec(ctx, "GRANT commerce_media_registrar TO "+pgx.Identifier{login}.Sanitize()); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if _, err := h.lp.f.owner.Exec(context.Background(), "REVOKE commerce_media_registrar FROM "+pgx.Identifier{login}.Sanitize()); err != nil {
					t.Error(err)
				}
			})
			if member == "commerce_runtime" {
				if admitted, err := platform.OpenPool(ctx, pool.Config().ConnString()); err == nil {
					admitted.Close()
					t.Fatal("mixed registrar/runtime admitted")
				}
			} else if err := platform.ValidateWorkerPool(ctx, pool); err == nil {
				t.Fatal("mixed registrar/worker admitted")
			}
		})
	}
	t.Run("set-only-execute", func(t *testing.T) {
		login, pool := lmaLogin(t, h.lp.f, "commerce_runtime")
		role := "lma_reachable_" + strings.ReplaceAll(randomUUID(), "-", "")
		if _, err := h.lp.f.owner.Exec(ctx, "CREATE ROLE "+pgx.Identifier{role}.Sanitize()+" NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if _, err := h.lp.f.owner.Exec(context.Background(), "DROP ROLE "+pgx.Identifier{role}.Sanitize()); err != nil {
				t.Error(err)
			}
		})
		if _, err := h.lp.f.owner.Exec(ctx, "GRANT "+pgx.Identifier{role}.Sanitize()+" TO "+pgx.Identifier{login}.Sanitize()+" WITH INHERIT FALSE, SET TRUE"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if _, err := h.lp.f.owner.Exec(context.Background(), "REVOKE "+pgx.Identifier{role}.Sanitize()+" FROM "+pgx.Identifier{login}.Sanitize()); err != nil {
				t.Error(err)
			}
		})
		if _, err := h.lp.f.owner.Exec(ctx, "GRANT EXECUTE ON FUNCTION live.register_prepared_media(jsonb,bytea,bytea) TO "+pgx.Identifier{role}.Sanitize()); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if _, err := h.lp.f.owner.Exec(context.Background(), "REVOKE EXECUTE ON FUNCTION live.register_prepared_media(jsonb,bytea,bytea) FROM "+pgx.Identifier{role}.Sanitize()); err != nil {
				t.Error(err)
			}
		})
		if admitted, err := platform.OpenPool(ctx, pool.Config().ConnString()); err == nil {
			admitted.Close()
			t.Fatal("SET-reachable media EXECUTE admitted")
		}
	})
	t.Run("direct-execute", func(t *testing.T) {
		login, pool := lmaLogin(t, h.lp.f, "commerce_runtime")
		if _, err := h.lp.f.owner.Exec(ctx, "GRANT EXECUTE ON FUNCTION live.revoke_prepared_media(uuid,uuid,uuid,text) TO "+pgx.Identifier{login}.Sanitize()); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if _, err := h.lp.f.owner.Exec(context.Background(), "REVOKE EXECUTE ON FUNCTION live.revoke_prepared_media(uuid,uuid,uuid,text) FROM "+pgx.Identifier{login}.Sanitize()); err != nil {
				t.Error(err)
			}
		})
		if admitted, err := platform.OpenPool(ctx, pool.Config().ConnString()); err == nil {
			admitted.Close()
			t.Fatal("direct media EXECUTE admitted")
		}
	})
	t.Run("public-execute", func(t *testing.T) {
		_, pool := lmaLogin(t, h.lp.f, "commerce_worker")
		if _, err := h.lp.f.owner.Exec(ctx, "GRANT EXECUTE ON FUNCTION live.register_prepared_media(jsonb,bytea,bytea) TO PUBLIC"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if _, err := h.lp.f.owner.Exec(context.Background(), "REVOKE EXECUTE ON FUNCTION live.register_prepared_media(jsonb,bytea,bytea) FROM PUBLIC"); err != nil {
				t.Error(err)
			}
		})
		if err := platform.ValidateWorkerPool(ctx, pool); err == nil {
			t.Fatal("PUBLIC media EXECUTE admitted")
		}
	})
}

func TestLiveMediaAuthorizationLMA03ClosedValidationAndNoWrites(t *testing.T) {
	h := lmaSetup(t)
	ctx := context.Background()
	nonce, ciphertext := lmaEnvelope()
	foreignStore := h.binding(t, h.lp.f.tenantA, h.lp.f.storeA2, h.lp.actor, "facebook", "page_other_store")
	foreignTenant := h.binding(t, h.lp.f.tenantB, h.lp.f.storeB, h.lp.other, "facebook", "page_other_tenant")
	duplicateAsset := h.binding(t, h.lp.f.tenantA, h.lp.f.storeA1, h.lp.actor, "facebook", "page_lma")
	// Each negative owns a fresh authorization id, so a failure cannot hide a partial write.
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any, *[]byte, *[]byte)
	}{
		{"live", func(s map[string]any, _, _ *[]byte) { s["environment"] = "LIVE" }},
		{"fake-evidence", func(s map[string]any, _, _ *[]byte) { s["evidence_type"] = "REAL_LIVE" }},
		{"unknown", func(s map[string]any, _, _ *[]byte) { s["unexpected"] = "secret-canary-lma" }},
		{"missing", func(s map[string]any, _, _ *[]byte) { delete(s, "key_id") }},
		{"null", func(s map[string]any, _, _ *[]byte) { s["key_id"] = nil }},
		{"wrong-string-type", func(s map[string]any, _, _ *[]byte) { s["project_id"] = 7 }},
		{"wrong-number-type", func(s map[string]any, _, _ *[]byte) { s["budget_minor"] = "10000" }},
		{"fraction", func(s map[string]any, _, _ *[]byte) { s["budget_minor"] = 1.5 }},
		{"zero-id", func(s map[string]any, _, _ *[]byte) { s["attempt_id"] = "00000000-0000-0000-0000-000000000000" }},
		{"upper-id", func(s map[string]any, _, _ *[]byte) { s["attempt_id"] = strings.ToUpper(randomUUID()) }},
		{"foreign-tenant", func(s map[string]any, _, _ *[]byte) { s["tenant_id"] = h.lp.f.tenantB }},
		{"other-store", func(s map[string]any, _, _ *[]byte) { s["store_id"] = h.lp.f.storeA2 }},
		{"wrong-media-asset", func(s map[string]any, _, _ *[]byte) { s["project_id"] = "wrong_asset" }},
		{"stale-media", func(s map[string]any, _, _ *[]byte) { s["media_binding_version"] = 2 }},
		{"stale-destination", func(s map[string]any, _, _ *[]byte) {
			s["destinations"].([]any)[0].(map[string]any)["binding_version"] = 2
		}},
		{"wrong-destination", func(s map[string]any, _, _ *[]byte) {
			s["destinations"].([]any)[0].(map[string]any)["external_asset_id"] = "wrong_asset"
		}},
		{"cross-store-binding", func(s map[string]any, _, _ *[]byte) {
			s["destinations"] = []any{lmaDestination(foreignStore, "facebook", "page_other_store")}
		}},
		{"cross-tenant-binding", func(s map[string]any, _, _ *[]byte) {
			s["destinations"] = []any{lmaDestination(foreignTenant, "facebook", "page_other_tenant")}
		}},
		{"duplicate-binding", func(s map[string]any, _, _ *[]byte) {
			s["destinations"] = []any{lmaDestination(h.facebook, "facebook", "page_lma"), lmaDestination(h.facebook, "facebook", "page_lma")}
		}},
		{"duplicate-provider-asset", func(s map[string]any, _, _ *[]byte) {
			s["destinations"] = []any{lmaDestination(h.facebook, "facebook", "page_lma"), lmaDestination(duplicateAsset, "facebook", "page_lma")}
		}},
		{"media-as-destination", func(s map[string]any, _, _ *[]byte) {
			s["destinations"] = []any{lmaDestination(h.media, "livekit", "project_lma")}
		}},
		{"wrong-session-version", func(s map[string]any, _, _ *[]byte) { s["session_version"] = 2 }},
		{"bigint-overflow", func(s map[string]any, _, _ *[]byte) { s["session_version"] = json.Number("9223372036854775808") }},
		{"wrong-aspect", func(s map[string]any, _, _ *[]byte) { s["aspect_ratio"] = "9:16" }},
		{"past-deadline", func(s map[string]any, _, _ *[]byte) {
			s["start_before"] = time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)
		}},
		{"far-deadline", func(s map[string]any, _, _ *[]byte) {
			s["start_before"] = time.Now().UTC().Add(25 * time.Hour).Format(time.RFC3339)
		}},
		{"bad-endpoint", func(s map[string]any, _, _ *[]byte) { s["endpoint_identity"] = "https://unit.livekit.cloud/path" }},
		{"oversize-spec", func(s map[string]any, _, _ *[]byte) { s["evidence_issuer"] = strings.Repeat("x", 17000) }},
		{"zero-duration", func(s map[string]any, _, _ *[]byte) { s["max_duration_seconds"] = 0 }},
		{"too-long-duration", func(s map[string]any, _, _ *[]byte) { s["max_duration_seconds"] = 14401 }},
		{"zero-budget", func(s map[string]any, _, _ *[]byte) { s["budget_minor"] = 0 }},
		{"too-large-budget", func(s map[string]any, _, _ *[]byte) { s["budget_minor"] = int64(1000000001) }},
		{"bad-nonce", func(_ map[string]any, n, _ *[]byte) { *n = (*n)[:11] }},
		{"bad-ciphertext", func(_ map[string]any, _, c *[]byte) { *c = (*c)[:15] }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec := h.spec()
			n, c := append([]byte(nil), nonce...), append([]byte(nil), ciphertext...)
			tc.mutate(spec, &n, &c)
			_, err := lmaRegister(ctx, h.registrar, spec, n, c)
			lmaBad(t, err, "secret-canary-lma")
			if a, b, r := lmaCounts(t, h.lp.f, spec["id"].(string)); a != 0 || b != 0 || r != 0 {
				t.Fatal("invalid registration wrote rows")
			}
		})
	}
	for _, mode := range []string{"disabled-media", "disabled-destination"} {
		t.Run(mode, func(t *testing.T) {
			spec := h.spec()
			var q string
			var arg any
			switch mode {
			case "disabled-media":
				q, arg = `UPDATE integration.bindings SET enabled=false WHERE id=$1`, h.media
			case "disabled-destination":
				q, arg = `UPDATE integration.bindings SET enabled=false WHERE id=$1`, h.facebook
			}
			if _, err := h.lp.f.owner.Exec(ctx, q, arg); err != nil {
				t.Fatal(err)
			}
			_, err := lmaRegister(ctx, h.registrar, spec, nonce, ciphertext)
			lmaBad(t, err, "secret-canary-lma")
			if a, b, r := lmaCounts(t, h.lp.f, spec["id"].(string)); a != 0 || b != 0 || r != 0 {
				t.Fatal("disabled binding wrote rows")
			}
			if _, err := h.lp.f.owner.Exec(ctx, `UPDATE integration.bindings SET enabled=true WHERE id=$1`, arg); err != nil {
				t.Fatal(err)
			}
		})
	}
	// The existing planning schema makes a non-DRAFT program unreachable even
	// to the fixture owner; verify that prerequisite instead of skipping a case.
	if _, err := h.lp.f.owner.Exec(ctx, `UPDATE live.programs SET state='ACTIVE' WHERE session_id=$1`, h.session); sqlState(err) != "23514" {
		t.Fatalf("non-DRAFT program became representable: %v", err)
	}
}

func lmaObserveBlock(t *testing.T, owner *pgxpool.Pool, waiterPID, holderPID int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var blocked bool
		err := owner.QueryRow(context.Background(), `SELECT coalesce(wait_event_type='Lock',false) AND $2::int=ANY(pg_blocking_pids($1))
			FROM pg_stat_activity WHERE pid=$1`, waiterPID, holderPID).Scan(&blocked)
		if err != nil {
			t.Fatal(err)
		}
		if blocked {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("backend %d never observed blocked by %d", waiterPID, holderPID)
}

func TestLiveMediaAuthorizationLMA04ConcurrencyAndObservedWait(t *testing.T) {
	h := lmaSetup(t)
	ctx := context.Background()
	nonce, ciphertext := lmaEnvelope()
	spec := h.spec()
	var wg sync.WaitGroup
	ids := make([]string, 2)
	errs := make([]error, 2)
	for i := range ids {
		wg.Add(1)
		go func(i int) { defer wg.Done(); ids[i], errs[i] = lmaRegister(ctx, h.registrar, spec, nonce, ciphertext) }(i)
	}
	wg.Wait()
	if errs[0] != nil || errs[1] != nil || ids[0] != spec["id"] || ids[1] != ids[0] {
		t.Fatalf("concurrent replay ids=%v errors=%v", ids, errs)
	}
	if a, b, r := lmaCounts(t, h.lp.f, ids[0]); a != 1 || b != 1 || r != 0 {
		t.Fatal("concurrent registration duplicated facts")
	}
	changed := map[string]any{}
	for k, v := range spec {
		changed[k] = v
	}
	changed["budget_minor"] = int64(10001)
	_, err := lmaRegister(ctx, h.registrar, changed, nonce, ciphertext)
	lmaBad(t, err, "secret-canary-lma")
	changed = map[string]any{}
	for k, v := range spec {
		changed[k] = v
	}
	changedNonce := append([]byte(nil), nonce...)
	changedNonce[0]++
	_, err = lmaRegister(ctx, h.registrar, changed, changedNonce, ciphertext)
	lmaBad(t, err, "secret-canary-lma")
	for _, mode := range []string{"version", "deadline"} {
		t.Run(mode, func(t *testing.T) {
			s := h.spec()
			if mode == "deadline" {
				var startBefore time.Time
				if err := h.lp.f.owner.QueryRow(ctx, `SELECT clock_timestamp()+interval '3 seconds'`).Scan(&startBefore); err != nil {
					t.Fatal(err)
				}
				s["start_before"] = startBefore.UTC().Format(time.RFC3339Nano)
			}
			holder, err := h.lp.f.owner.Acquire(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer holder.Release()
			tx, err := holder.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			var holderPID int
			if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
				t.Fatal(err)
			}
			if _, err := tx.Exec(ctx, `UPDATE integration.bindings SET enabled=enabled WHERE id=$1`, h.media); err != nil {
				t.Fatal(err)
			}
			waiter, err := h.registrar.Acquire(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer waiter.Release()
			var waiterPID int
			if err := waiter.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&waiterPID); err != nil {
				t.Fatal(err)
			}
			result := make(chan error, 1)
			go func() {
				data, _ := json.Marshal(s)
				var id string
				result <- waiter.QueryRow(ctx, `SELECT live.register_prepared_media($1::jsonb,$2,$3)::text`, string(data), nonce, ciphertext).Scan(&id)
			}()
			lmaObserveBlock(t, h.lp.f.owner, waiterPID, holderPID)
			if mode == "version" {
				if _, err := tx.Exec(ctx, `UPDATE integration.bindings SET semantic_version=semantic_version+1 WHERE id=$1`, h.media); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := tx.Exec(ctx, `SELECT pg_sleep(GREATEST(0,extract(epoch FROM $1::timestamptz-clock_timestamp()))+0.03)`, s["start_before"]); err != nil {
					t.Fatal(err)
				}
			}
			if err := tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-result:
				lmaBad(t, err, "secret-canary-lma")
			case <-time.After(5 * time.Second):
				t.Fatal("blocked registration did not finish")
			}
			if a, b, r := lmaCounts(t, h.lp.f, s["id"].(string)); a != 0 || b != 0 || r != 0 {
				t.Fatal("post-wait invalid authority persisted")
			}
			if mode == "version" {
				if _, err := h.lp.f.owner.Exec(ctx, `UPDATE integration.bindings SET semantic_version=1 WHERE id=$1`, h.media); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestLiveMediaAuthorizationLMA05RevocationAndNoRevival(t *testing.T) {
	h := lmaSetup(t)
	ctx := context.Background()
	nonce, ciphertext := lmaEnvelope()
	spec := h.spec()
	id, err := lmaRegister(ctx, h.registrar, spec, nonce, ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.registrar.Exec(ctx, `SELECT live.revoke_prepared_media($1::uuid,$2::uuid,$3::uuid,$4::text)`, h.lp.f.tenantA, h.lp.f.storeA1, id, "operator_revoke"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.registrar.Exec(ctx, `SELECT live.revoke_prepared_media($1::uuid,$2::uuid,$3::uuid,$4::text)`, h.lp.f.tenantA, h.lp.f.storeA1, id, "operator_revoke"); err != nil {
		t.Fatal(err)
	}
	var by, reason string
	var at time.Time
	if err := h.lp.f.owner.QueryRow(ctx, `SELECT revoked_by,reason_code,revoked_at FROM live.media_authorization_revocations WHERE authorization_id=$1`, id).Scan(&by, &reason, &at); err != nil || by != h.login || reason != "operator_revoke" || at.IsZero() {
		t.Fatalf("revocation provenance by=%q reason=%q at=%v err=%v", by, reason, at, err)
	}
	for _, tc := range []struct{ name, tenant, store, id, reason string }{
		{"wrong-tenant", h.lp.f.tenantB, h.lp.f.storeA1, id, "operator_revoke"},
		{"wrong-store", h.lp.f.tenantA, h.lp.f.storeA2, id, "operator_revoke"},
		{"missing", h.lp.f.tenantA, h.lp.f.storeA1, randomUUID(), "operator_revoke"},
		{"changed-reason", h.lp.f.tenantA, h.lp.f.storeA1, id, "other_reason"},
		{"bad-reason", h.lp.f.tenantA, h.lp.f.storeA1, id, "secret-canary-lma\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := h.registrar.Exec(ctx, `SELECT live.revoke_prepared_media($1::uuid,$2::uuid,$3::uuid,$4::text)`, tc.tenant, tc.store, tc.id, tc.reason)
			lmaBad(t, err, "secret-canary-lma")
		})
	}
	_, err = lmaRegister(ctx, h.registrar, spec, nonce, ciphertext)
	lmaBad(t, err, "secret-canary-lma")
	if a, b, r := lmaCounts(t, h.lp.f, id); a != 1 || b != 1 || r != 1 {
		t.Fatal("revocation replay rewrote history")
	}
	late := h.spec()
	late["start_before"] = time.Now().UTC().Add(3 * time.Second).Truncate(time.Second).Format(time.RFC3339)
	lateID, err := lmaRegister(ctx, h.registrar, late, nonce, ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.lp.f.owner.Exec(ctx, `UPDATE integration.bindings SET enabled=false WHERE id=$1`, h.media); err != nil {
		t.Fatal(err)
	}
	if _, err := h.lp.f.owner.Exec(ctx, `SELECT pg_sleep(GREATEST(0,extract(epoch FROM $1::timestamptz-clock_timestamp()))+0.03)`, late["start_before"]); err != nil {
		t.Fatal(err)
	}
	if _, err := h.registrar.Exec(ctx, `SELECT live.revoke_prepared_media($1::uuid,$2::uuid,$3::uuid,$4::text)`, h.lp.f.tenantA, h.lp.f.storeA1, lateID, "expired_disabled"); err != nil {
		t.Fatalf("reducing authority after expiry/disable denied: %v", err)
	}
	if a, b, r := lmaCounts(t, h.lp.f, lateID); a != 1 || b != 1 || r != 1 {
		t.Fatal("late revocation facts changed")
	}
}
