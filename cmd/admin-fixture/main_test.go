package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestOwnerFileIsPrivateOneUseAndRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "owner.dsn")
	env := filepath.Join(dir, "session.env")
	if err := os.WriteFile(path, []byte("fixture-only-test-value"), 0600); err != nil {
		t.Fatal(err)
	}
	value, err := consumeOwnerFile(path, env)
	if err != nil || value != "fixture-only-test-value" {
		t.Fatal("private file did not deliver once")
	}
	if _, err = os.Lstat(path); !os.IsNotExist(err) {
		t.Fatal("credential file persisted")
	}
	if _, err = consumeOwnerFile(path, env); err == nil {
		t.Fatal("credential replay accepted")
	}
	target := filepath.Join(dir, "not-owned")
	if err = os.WriteFile(target, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	if _, err = consumeOwnerFile(path, env); err == nil {
		t.Fatal("symlink accepted")
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != "preserve" {
		t.Fatal("unowned target mutated")
	}
}

func TestFixtureConfigRejectsFallbackAndOtherDatabases(t *testing.T) {
	for _, dsn := range []string{
		"postgres://postgres@127.0.0.1:5432/lc_admin_fixture?sslmode=disable",
		"postgres://postgres@127.0.0.1,example.invalid:5432/lc_admin_fixture?sslmode=disable",
		"postgres://postgres@example.invalid:5432/lc_admin_fixture?sslmode=disable",
		"postgres://postgres@127.0.0.1:5432/customer?sslmode=disable",
	} {
		config, err := pgxpool.ParseConfig(dsn)
		if err != nil {
			t.Fatal("invalid test configuration")
		}
		want := strings.Contains(dsn, "127.0.0.1:5432/lc_admin_fixture?")
		if got := validFixtureConfig(config); got != want {
			t.Errorf("config guard got %v want %v", got, want)
		}
	}
}

// Uses only the already task-owned fixture cluster and creates its own empty
// database, so the running UI's seeded rows are never modified by this test.
func TestFixtureEmptyDatabaseRejectsPublicTableWithoutSideEffects(t *testing.T) {
	if os.Getenv("COMMERCE_FIXTURE_ALLOWED") != "1" {
		t.Skip("isolated fixture guard REAL_PG NOT_RUN")
	}
	config, err := pgxpool.ParseConfig(os.Getenv("LC_ADMIN_GUARD_DSN"))
	if err != nil || !validFixtureConfig(config) {
		t.Fatal("guard test refuses nonfixture owner")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	owner, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal("fixture owner unavailable")
	}
	defer owner.Close()
	name := "lc_guard_" + strings.ReplaceAll(uuid(), "-", "")
	identifier := pgx.Identifier{name}.Sanitize()
	if _, err = owner.Exec(ctx, "CREATE DATABASE "+identifier); err != nil {
		t.Fatal("create owned guard database failed")
	}
	defer func() {
		if _, e := owner.Exec(context.Background(), "DROP DATABASE "+identifier); e != nil {
			t.Error("owned guard database cleanup failed")
		}
	}()
	dsn := fmt.Sprintf("postgres://postgres:%s@127.0.0.1:%d/%s?sslmode=disable", config.ConnConfig.Password, config.ConnConfig.Port, name)
	fresh, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal("fresh guard pool failed")
	}
	defer fresh.Close()
	empty, err := emptyFixtureDatabase(ctx, fresh)
	if err != nil || !empty {
		t.Fatal("new fixture must be empty")
	}
	if _, err = fresh.Exec(ctx, `CREATE TABLE public.keep_me(marker text); INSERT INTO public.keep_me VALUES('preserve')`); err != nil {
		t.Fatal("negative fixture setup failed")
	}
	empty, err = emptyFixtureDatabase(ctx, fresh)
	if err != nil || empty {
		t.Fatal("public-only data must refuse migrations")
	}
	var marker string
	if err = fresh.QueryRow(ctx, `SELECT marker FROM public.keep_me`).Scan(&marker); err != nil || marker != "preserve" {
		t.Fatal("guard mutated existing public data")
	}
	var migrated bool
	if err = fresh.QueryRow(ctx, `SELECT to_regclass('public.lc_schema_migrations') IS NOT NULL`).Scan(&migrated); err != nil || migrated {
		t.Fatal("guard ran migration side effects")
	}
}
