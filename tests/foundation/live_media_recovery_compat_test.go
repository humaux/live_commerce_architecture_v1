package foundation_test

// Responsibility: legacy DB admission with optional future recovery catalog.
// Dependencies: production legacy migrations, mciStartPG/lmaLogin, platform validators.
// Called by MRR06; invariants: exact optional ABIs only, isolated grants restored.
// Future bodies are MODEL_ONLY inert catalog stubs, never migration/business proof.
import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/platform"
	"livecommerce/migrations"
)

func TestLiveMediaRecoveryMRR06Compatibility(t *testing.T) {
	f := mrr06LegacyFixture(t)
	ctx := context.Background()
	login, recovery := lmaLogin(t, f, "commerce_media_recovery")
	name := pgx.Identifier{login}.Sanitize()
	mustExec(t, f.owner, "REVOKE commerce_media_recovery FROM "+name)
	mustExec(t, f.owner, "GRANT commerce_media_recovery TO "+name+" WITH INHERIT TRUE, SET FALSE")
	validate := func() error { return platform.ValidateMediaRecoveryPool(ctx, recovery) }
	clean := func(t *testing.T) {
		t.Helper()
		if err := validate(); err != nil {
			t.Fatalf("clean legacy recovery rejected: %v", err)
		}
	}
	clean(t)
	var ready bool
	if err := recovery.QueryRow(ctx, "SELECT live.media_recovery_ready()").Scan(&ready); err != nil || !ready {
		t.Fatalf("legacy readiness: %v %v", ready, err)
	}

	// Ordered output columns are frozen in live-browser-input-recovery-v1.md.
	// The isolated legacy fixture has not executed the future business migration.
	abis := []struct{ signature, result string }{
		{"begin_media_recovery_episode_with_input(uuid,bigint,integer,boolean)", "TABLE(disposition text,episode_id uuid,operation_id uuid,job_id bigint,baseline_generation bigint,deadline_at timestamptz,candidate_count integer,coverage_known boolean,blocked_by_episode_id uuid,execution_profile text,input_required boolean,egress_required boolean)"},
		{"claim_media_recovery_observation_with_input(uuid,uuid,bigint,bytea)", "TABLE(disposition text,generation bigint,project_id text,credential_version bigint,endpoint_identity text,room_name text,publisher_identity text,egress_id text,execution_profile text,input_required boolean,egress_required boolean,input_observation_id uuid,egress_observation_id uuid)"},
		{"record_browser_input_recovery_observation(uuid,uuid,bigint,bytea,text,text,text,text)", "TABLE(disposition text,input_observation_id uuid)"},
		{"record_media_recovery_observation_with_input(uuid,uuid,bigint,bytea,text,text,text,text,bigint,bigint,bigint)", "TABLE(disposition text,observation_id uuid)"},
		{"finish_media_recovery_observation_with_input(uuid,uuid,bigint,bytea,text)", "text"},
		{"read_media_recovery_episode_with_input(uuid)", "TABLE(episode_id uuid,scope_status text,coverage_known boolean,candidate_count integer,blocked_by_episode_id uuid,operation_id uuid,disposition text,baseline_generation bigint,observation_id uuid,observation_source text,observation_generation bigint,witness_elapsed_ms bigint,timeout_at timestamptz,cleanup_required boolean,execution_profile text,input_required boolean,egress_required boolean,input_observation_id uuid,input_observation_source text,input_observation_result text,input_observation_generation bigint,job_id bigint)"},
		{"witness_media_recovery_episode_with_input(uuid,uuid,uuid,uuid,bigint)", "text"},
		{"media_browser_input_recovery_ready()", "boolean"},
	}
	for _, abi := range abis {
		var absent bool
		if err := f.owner.QueryRow(ctx, "SELECT to_regprocedure($1) IS NULL", "live."+abi.signature).Scan(&absent); err != nil || !absent {
			t.Fatalf("requires legacy database without %s: absent=%v err=%v", abi.signature, absent, err)
		}
	}
	// All mutations are scoped to the disposable fixture and undone even on failure.
	create := func(t *testing.T, signature, result string) {
		t.Helper()
		body := "BEGIN RETURN NULL; END"
		if strings.HasPrefix(result, "TABLE(") {
			body = "BEGIN RETURN; END"
		}
		volatility := "VOLATILE"
		if signature == "read_media_recovery_episode_with_input(uuid)" {
			volatility = "STABLE"
		}
		mustExec(t, f.owner, "CREATE FUNCTION live."+signature+" RETURNS "+result+" LANGUAGE plpgsql "+volatility+" SECURITY DEFINER SET search_path=pg_catalog AS $$ "+body+" $$")
		t.Cleanup(func() {
			if _, err := f.owner.Exec(ctx, "DROP FUNCTION live."+signature); err != nil {
				t.Errorf("drop owned catalog stub: %v", err)
			}
		})
		mustExec(t, f.owner, "ALTER FUNCTION live."+signature+" OWNER TO commerce_media_writer")
		mustExec(t, f.owner, "REVOKE ALL ON FUNCTION live."+signature+" FROM PUBLIC")
	}
	grantRejected := func(t *testing.T, signature, grantee string, check func() error) {
		t.Helper()
		target := pgx.Identifier{grantee}.Sanitize()
		mustExec(t, f.owner, "GRANT EXECUTE ON FUNCTION live."+signature+" TO "+target)
		defer mustExec(t, f.owner, "REVOKE EXECUTE ON FUNCTION live."+signature+" FROM "+target)
		if err := check(); err == nil {
			t.Fatal("forbidden EXECUTE admitted: " + signature)
		}
	}
	t.Run("absent_optional_does_not_mask_unknown_grant", func(t *testing.T) {
		create(t, "mrr06_unknown_callable()", "text")
		grantRejected(t, "mrr06_unknown_callable()", login, validate)
	})
	clean(t)
	for _, abi := range abis {
		create(t, abi.signature, abi.result)
		mustExec(t, f.owner, "GRANT EXECUTE ON FUNCTION live."+abi.signature+" TO commerce_media_recovery")
		// Assert fixture owner/ACL/search_path/SECDEF, not a validator promise about bodies.
		var exact bool
		if err := f.owner.QueryRow(ctx, `SELECT p.proowner='commerce_media_writer'::regrole AND p.prosecdef
    AND p.proconfig=ARRAY['search_path=pg_catalog']::text[]
    AND NOT EXISTS (SELECT 1 FROM aclexplode(p.proacl) a WHERE a.privilege_type='EXECUTE' AND a.grantee NOT IN ('commerce_media_writer'::regrole,'commerce_media_recovery'::regrole))
    AND has_function_privilege('commerce_media_recovery',p.oid,'EXECUTE')
    FROM pg_proc p WHERE p.oid=to_regprocedure($1)`, "live."+abi.signature).Scan(&exact); err != nil || !exact {
			t.Fatalf("future fixture catalog ACL mismatch %s: %v %v", abi.signature, exact, err)
		}
	}
	t.Run("exact_future_catalog_admitted", func(t *testing.T) {
		if err := validate(); err != nil {
			t.Fatalf("exact optional future ABI catalog unexpectedly refused: %v", err)
		}
		if err := recovery.QueryRow(ctx, "SELECT live.media_recovery_ready()").Scan(&ready); err != nil || !ready {
			t.Fatalf("legacy caller readiness changed: %v %v", ready, err)
		}
	})
	t.Run("wrong_overload", func(t *testing.T) {
		create(t, "read_media_recovery_episode_with_input(text)", "text")
		grantRejected(t, "read_media_recovery_episode_with_input(text)", login, validate)
	})
	t.Run("unknown_callable", func(t *testing.T) {
		create(t, "mrr06_unknown_callable()", "text")
		grantRejected(t, "mrr06_unknown_callable()", login, validate)
	})
	t.Run("registrar_function", func(t *testing.T) {
		grantRejected(t, "register_prepared_media(jsonb,bytea,bytea)", login, validate)
	})
	t.Run("forbidden_membership", func(t *testing.T) {
		for _, role := range []string{"commerce_media_worker", "commerce_media_registrar", "commerce_media_executor"} {
			t.Run(role, func(t *testing.T) {
				mustExec(t, f.owner, "GRANT "+role+" TO "+name+" WITH INHERIT TRUE, SET FALSE")
				defer mustExec(t, f.owner, "REVOKE "+role+" FROM "+name)
				if err := validate(); err == nil {
					t.Fatal("forbidden role admitted")
				}
			})
		}
	})
	for _, role := range []string{"commerce_media_worker", "commerce_runtime"} {
		t.Run(role+"_rejects_future_grants", func(t *testing.T) {
			other, pool := lmaLogin(t, f, role)
			quoted := pgx.Identifier{other}.Sanitize()
			mustExec(t, f.owner, "REVOKE "+role+" FROM "+quoted)
			mustExec(t, f.owner, "GRANT "+role+" TO "+quoted+" WITH INHERIT TRUE, SET FALSE")
			check := func() error {
				if role == "commerce_media_worker" {
					return platform.ValidateMediaWorkerPool(ctx, pool)
				}
				var opened *pgxpool.Pool
				opened, err := platform.OpenPool(ctx, pool.Config().ConnString())
				if opened != nil {
					opened.Close()
				}
				return err
			}
			if err := check(); err != nil {
				t.Fatalf("clean other pool rejected: %v", err)
			}
			for _, abi := range abis {
				grantRejected(t, abi.signature, other, check)
			}
			if err := check(); err != nil {
				t.Fatalf("other pool not restored: %v", err)
			}
		})
	}
	// Recheck positive at the end on the candidate, without turning expected base RED into setup failure.
	t.Run("restored_future_catalog_admitted", func(t *testing.T) {
		if err := validate(); err != nil {
			t.Fatalf("exact optional future ABI catalog unexpectedly refused after negatives: %v", err)
		}
	})
}

// mrr06LegacyFixture applies real historical SQL to a fresh disposable database.
// As in the R2 upgrade fixture, the full-file checksum temporarily defers only
// 0029; remove that placeholder before any legacy-state observation. Never apply
// 0029 and drop its objects: the optional catalog stubs below are MODEL_ONLY.
func mrr06LegacyFixture(t *testing.T) *testFixture {
	t.Helper()
	owner := mciStartPG(t)
	ctx := context.Background()
	const deferred = "post_river/0029_live_browser_input_recovery.sql"
	body, err := os.ReadFile("../../migrations/" + deferred)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, owner, `CREATE TABLE public.lc_schema_migrations (version text PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`)
	mustExec(t, owner, `INSERT INTO public.lc_schema_migrations(version,checksum) VALUES($1,$2)`, deferred, fmt.Sprintf("%x", sha256.Sum256(body)))
	if err := migrations.Apply(ctx, owner); err != nil {
		t.Fatalf("MRR06 real pre-0029 migrations: %v", err)
	}
	mustExec(t, owner, `DELETE FROM public.lc_schema_migrations WHERE version=$1`, deferred)
	if n := countRows(t, owner, `SELECT count(*) FROM public.lc_schema_migrations WHERE version=$1`, deferred); n != 0 {
		t.Fatalf("MRR06 legacy database has %d applied 0029 ledger entries", n)
	}
	return &testFixture{owner: owner, databaseURL: owner.Config().ConnString()}
}
