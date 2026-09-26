package foundation_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/riverqueue/river/riverdriver/riverpgxv5"
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
	return f
}
