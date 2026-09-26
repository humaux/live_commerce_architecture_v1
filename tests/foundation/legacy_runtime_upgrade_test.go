package foundation_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"livecommerce/internal/platform"
	"github.com/riverqueue/river/rivermigrate"
)

// Reuse the exact pre-0029 fixture bytes and install only later accepted old
// versions. In particular, neither 0032 nor post/0005 exists at this boundary.
func lriPre0032Fixture(t *testing.T) *testFixture {
	t.Helper()
	f := mcPre0029Fixture(t)
	ctx := context.Background()
	mcApplyHistorical(t, f, "0029_meta_social_consumer.sql", "0030_meta_runtime.sql", "0031_meta_river_isolation.sql")
	upstream, err := rivermigrate.New(riverpgxv5.New(f.owner), &rivermigrate.Config{Schema: "river_meta", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := upstream.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		t.Fatal(err)
	}
	mcApplyHistorical(t, f, "post_river/0004_meta_river_isolation.sql")
	mustExec(t, f.owner, `GRANT USAGE ON SCHEMA river_meta TO commerce_meta_worker;
	 GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA river_meta TO commerce_meta_worker;
	 REVOKE ALL ON river_meta.river_migration FROM commerce_meta_worker;
	 GRANT USAGE,SELECT ON ALL SEQUENCES IN SCHEMA river_meta TO commerce_meta_worker`)
	if miCount(t, f.owner, `SELECT count(*) FROM public.lc_schema_migrations WHERE version IN ('0032_legacy_river_isolation.sql','post_river/0005_legacy_river_isolation.sql')`) != 0 {
		t.Fatal("historical fixture crossed 0032 boundary")
	}
	runtime, err := platform.OpenPool(ctx, bcRole(t, f, "commerce_runtime"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Close)
	f.runtime = runtime
	return f
}

// Historical post-River router gates must replay their original SQL bytes at
// the old cutover, not call latest Apply (which now performs the 0032 cutover).
func lriApplyHistoricalPost(f *testFixture, target string) error {
	ctx := context.Background()
	tx, err := f.owner.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	paths, err := filepath.Glob("../../migrations/[0-9][0-9][0-9][0-9]_*.sql")
	if err != nil {
		return err
	}
	posts, err := filepath.Glob("../../migrations/post_river/[0-9][0-9][0-9][0-9]_*.sql")
	if err != nil {
		return err
	}
	known := map[string]string{}
	for _, path := range append(paths, posts...) {
		version := filepath.Base(path)
		if strings.Contains(path, "post_river/") {
			version = "post_river/" + version
			if version >= "post_river/0005" {
				continue
			}
		} else if version >= "0032" {
			continue
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		known[version] = fmt.Sprintf("%x", sha256.Sum256(body))
	}
	rows, err := tx.Query(ctx, `SELECT version,checksum FROM public.lc_schema_migrations`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var version, checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			rows.Close()
			return err
		}
		if expected, ok := known[version]; !ok || checksum != expected {
			rows.Close()
			return fmt.Errorf("historical version unknown or changed: %s", version)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if _, ok := known[target]; !ok {
		return fmt.Errorf("historical post-River target unknown: %s", target)
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.lc_schema_migrations WHERE version=$1)`, target).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		body, err := os.ReadFile(filepath.Join("../../migrations", target))
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(body)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.lc_schema_migrations(version,checksum) VALUES($1,$2)`, target, known[target]); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
