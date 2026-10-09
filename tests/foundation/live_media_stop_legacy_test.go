package foundation_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"livecommerce/internal/platform"
	"livecommerce/migrations"
)

// This compiles the actual pre-LMR admission code, not a restated SQL predicate.
// Acceptance checkouts must retain this pinned Git object. A shallow checkout
// missing it fails explicitly; it must not silently fall back to current source.
func lmrLegacyAdmissionBinary(t *testing.T) string {
	t.Helper()
	const revision = "395b10dbf02ac5a3d071c7f80d45b20356d2c474"
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	list := exec.CommandContext(ctx, "git", "ls-tree", "-r", "--name-only", revision,
		"--", "go.mod", "go.sum", "internal/platform", "internal/httperror")
	list.Dir = filepath.Join("..", "..") // go test runs from tests/foundation.
	paths, err := list.Output()
	if err != nil {
		t.Fatal("pinned pre-LMR Git source unavailable", err)
	}
	dir := t.TempDir()
	for _, path := range strings.Fields(string(paths)) {
		if strings.HasSuffix(path, "_test.go") || (!strings.HasSuffix(path, ".go") && path != "go.mod" && path != "go.sum") {
			continue
		}
		read := exec.CommandContext(ctx, "git", "show", revision+":"+path)
		read.Dir = list.Dir
		body, err := read.Output()
		if err != nil {
			t.Fatal("read pinned admission source", err)
		}
		target := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	const main = `package main
import ("context"; "fmt"; "os"; "time"; "livecommerce/internal/platform")
func main() {
 ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second); defer cancel()
 pool, err := platform.OpenMediaExecutorPool(ctx, os.Getenv("LC_LMR_LEGACY_DSN"))
 if err != nil { fmt.Println("rejected"); return }
 pool.Close(); fmt.Println("accepted")
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(main), 0600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(dir, "legacy-media-admission")
	build := exec.CommandContext(ctx, "go", "build", "-mod=readonly", "-trimpath", "-o", binary, ".")
	build.Dir = dir
	build.Env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=go1.27.2")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build pinned admission: %v\n%s", err, output)
	}
	return binary
}

func TestLiveMediaStopLMR01FrozenLegacyAdmissionBinary(t *testing.T) {
	f := lriPre0032Fixture(t) // Requires explicit disposable-PG consent first.
	binary := lmrLegacyAdmissionBinary(t)
	ctx := context.Background()
	mcApplyHistorical(t, f, "0032_legacy_river_isolation.sql", "0033_live_planning.sql",
		"0034_live_media_authorization.sql", "0035_live_media_plan.sql")
	for _, schema := range []string{"river_payment", "river_expiry", "river_media"} {
		upstream, err := rivermigrate.New(riverpgxv5.New(f.owner), &rivermigrate.Config{
			Schema: schema, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := upstream.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
			t.Fatal(err)
		}
	}
	mcApplyHistorical(t, f, "post_river/0005_legacy_river_isolation.sql", "post_river/0006_live_media_queue.sql",
		"0036_live_media_execution.sql", "post_river/0007_live_media_execution.sql")
	login, loginPool := lmaLogin(t, f, "commerce_media_executor")
	// Match the accepted LME login: inherited authority, never SET ROLE.
	mustExec(t, f.owner, "REVOKE commerce_media_executor FROM "+pgx.Identifier{login}.Sanitize())
	mustExec(t, f.owner, "GRANT commerce_media_executor TO "+pgx.Identifier{login}.Sanitize()+" WITH INHERIT TRUE, SET FALSE")
	dsn := loginPool.Config().ConnString()
	probe := func(want string) {
		t.Helper()
		bounded, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		cmd := exec.CommandContext(bounded, binary)
		cmd.Env = append(os.Environ(), "LC_LMR_LEGACY_DSN="+dsn)
		output, err := cmd.CombinedOutput()
		if err != nil || strings.TrimSpace(string(output)) != want {
			t.Fatalf("legacy admission wanted %s; process err=%v; unexpected output=%t", want, err,
				strings.TrimSpace(string(output)) != want)
		}
	}
	probe("accepted") // Proves the executable/DSN work before the upgrade.
	if err := migrations.Apply(ctx, f.owner); err != nil {
		t.Fatal(err)
	}
	probe("rejected")
	pool, err := platform.OpenMediaExecutorPool(ctx, dsn)
	if err != nil {
		t.Fatal("current admission must accept the same upgraded database", err)
	}
	pool.Close()
}
