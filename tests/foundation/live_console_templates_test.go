package foundation_test

// Real-PG gate for the LC-B5 merchant message templates (contracts/live-console-v1.md §3.4/§3.5/§7.3, §11
// /message-templates, unit W2-05B). Exercises migration 0124 exactly once (the shared fixture already applies it
// twice), the frozen role/definer/EXECUTE-grant ACL, the two system-fixed templates, the live:manage vs inbox:reply
// permission split, publish append-only versioning with the template.published audit and the idempotent receipt
// replay, the §3.5 public-safe refusal, the fixed-id PT409 conflict, and cross-store RLS isolation. No send path
// (LC-B4), no LIVE Meta traffic, no real buyer PII: fixtures are synthetic UUIDs and fixed template bodies.

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/command"
	"livecommerce/internal/msgtemplates"
	"livecommerce/internal/platform"
)

func TestLiveConsoleTemplatesMigration0124ExactACL(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()

	var migrationCount int
	if err := f.owner.QueryRow(ctx, `SELECT count(*) FROM public.lc_schema_migrations WHERE version='0124_msg_templates.sql'`).Scan(&migrationCount); err != nil {
		t.Fatal(err)
	}
	if migrationCount != 1 {
		t.Fatalf("0124 migration row count=%d, want exactly 1", migrationCount)
	}

	var login, super, bypass, createRole, createDB, replication bool
	if err := f.owner.QueryRow(ctx, `SELECT rolcanlogin,rolsuper,rolbypassrls,rolcreaterole,rolcreatedb,rolreplication FROM pg_roles WHERE rolname='commerce_msgtemplates_writer'`).
		Scan(&login, &super, &bypass, &createRole, &createDB, &replication); err != nil {
		t.Fatal(err)
	}
	if login || super || bypass || createRole || createDB || replication {
		t.Fatalf("unsafe commerce_msgtemplates_writer attributes: login=%t super=%t bypass=%t createRole=%t createDB=%t replication=%t", login, super, bypass, createRole, createDB, replication)
	}

	for _, table := range []string{"msgtemplates.templates", "msgtemplates.fixed_templates"} {
		var rls, force bool
		if err := f.owner.QueryRow(ctx, `SELECT relrowsecurity, relforcerowsecurity FROM pg_class WHERE oid=$1::regclass`, table).Scan(&rls, &force); err != nil || !rls || !force {
			t.Fatalf("table %s RLS: enabled=%t forced=%t err=%v", table, rls, force, err)
		}
	}

	// The two system-fixed templates are seeded exactly once with the frozen shape.
	fixed := map[string]struct {
		kinds []string
		safe  bool
		body  string
	}{
		"order-pay-link/v1":  {[]string{"dm"}, false, "{{連結}}"},
		"offer-recommend/v1": {[]string{"recommend"}, true, "{{product.name}}"},
	}
	for id, want := range fixed {
		var kinds []string
		var safe bool
		var body string
		if err := f.owner.QueryRow(ctx, `SELECT kinds, public_safe, body FROM msgtemplates.fixed_templates WHERE template_id=$1 AND version=1`, id).Scan(&kinds, &safe, &body); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(kinds, want.kinds) || safe != want.safe || !strings.Contains(body, want.body) {
			t.Fatalf("fixed %s: kinds=%v safe=%t body=%q", id, kinds, safe, body)
		}
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM msgtemplates.fixed_templates`); n != 2 {
		t.Fatalf("fixed template rows=%d, want 2", n)
	}

	// The three definers are owned by the NOLOGIN writer, SECURITY DEFINER, pg_catalog search_path, correct
	// volatility, and EXECUTE is exactly {writer, runtime}.
	functions := []struct {
		signature, owner string
		stable           bool
		resultSubstr     []string
	}{
		{"msgtemplates.publish(text,text,text,text[],boolean)", "commerce_msgtemplates_writer", false, []string{"template_id", "version", "public_safe", "kinds"}},
		{"msgtemplates.list()", "commerce_msgtemplates_writer", true, []string{"template_id", "version", "name", "kinds", "public_safe", "created_at"}},
		{"msgtemplates.resolve(text,bigint)", "commerce_msgtemplates_writer", true, []string{"template_id", "version", "name", "body", "kinds", "public_safe", "fixed"}},
	}
	for _, fn := range functions {
		t.Run(fn.signature, func(t *testing.T) {
			var owner, result, volatility string
			var definer, noLogin, noBypass, fixedPath bool
			var principals []string
			err := f.owner.QueryRow(ctx, `SELECT pg_get_userbyid(p.proowner), pg_get_function_result(p.oid),
				p.prosecdef, NOT r.rolcanlogin, NOT r.rolbypassrls,
				p.proconfig=ARRAY['search_path=pg_catalog']::text[], p.provolatile,
				ARRAY(SELECT CASE WHEN a.grantee=0 THEN 'PUBLIC' ELSE pg_get_userbyid(a.grantee) END
				  FROM aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a
				  WHERE a.privilege_type='EXECUTE' ORDER BY 1)
				FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid=$1::regprocedure`, fn.signature).
				Scan(&owner, &result, &definer, &noLogin, &noBypass, &fixedPath, &volatility, &principals)
			if err != nil {
				t.Fatal(err)
			}
			wantVolatility := "v"
			if fn.stable {
				wantVolatility = "s"
			}
			if owner != fn.owner || !definer || !noLogin || !noBypass || !fixedPath || volatility != wantVolatility ||
				!slices.Equal(principals, []string{"commerce_msgtemplates_writer", "commerce_runtime"}) {
				t.Fatalf("0124 boundary: owner=%s definer=%v noLogin=%v noBypass=%v path=%v volatility=%s ACL=%v", owner, definer, noLogin, noBypass, fixedPath, volatility, principals)
			}
			for _, sub := range fn.resultSubstr {
				if !strings.Contains(result, sub) {
					t.Fatalf("result %q lacks %q", result, sub)
				}
			}
		})
	}
}

func TestLiveConsoleTemplatesPublishVersionAuditIdempotency(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	svc := msgtemplates.NewService()

	_, managerToken := lcPerson(t, f, f.tenantA, f.storeA1, "live:manage")

	publish := func(key string, in msgtemplates.PublishInput) (msgtemplates.PublishOutput, error) {
		var out msgtemplates.PublishOutput
		err := platform.WithScope(ctx, f.runtime, managerToken, f.storeA1, "live:manage", func(tx pgx.Tx, scope platform.Scope) error {
			var err error
			out, err = svc.Publish(ctx, tx, scope, key, in)
			return err
		})
		return out, err
	}

	base := msgtemplates.PublishInput{TemplateID: "welcome-msg", Name: "歡迎訊息", Kinds: []string{"dm"}, PublicSafe: false, Body: "嗨，歡迎光臨"}

	out, err := publish("tpl-publish-0001", base)
	if err != nil {
		t.Fatal(err)
	}
	if out.TemplateID != "welcome-msg" || out.Version != 1 || out.PublicSafe || !slices.Equal(out.Kinds, []string{"dm"}) {
		t.Fatalf("publish v1=%+v, want welcome-msg/1/false/[dm]", out)
	}

	// Idempotent replay: same key and body returns the saved v1 and does not append another version.
	replay, err := publish("tpl-publish-0001", base)
	if err != nil {
		t.Fatal(err)
	}
	if replay.Version != 1 {
		t.Fatalf("replay version=%d, want 1", replay.Version)
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM msgtemplates.templates WHERE tenant_id=$1 AND store_id=$2 AND template_id='welcome-msg'`, f.tenantA, f.storeA1); n != 1 {
		t.Fatalf("template rows after replay=%d, want 1", n)
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='template.published'`, f.tenantA, f.storeA1); n != 1 {
		t.Fatalf("audit rows after replay=%d, want 1", n)
	}

	// Same key + different body is an idempotency conflict, not a silent overwrite.
	_, err = publish("tpl-publish-0001", msgtemplates.PublishInput{TemplateID: "welcome-msg", Name: "歡迎訊息", Kinds: []string{"dm"}, PublicSafe: false, Body: "改了內容"})
	if !errors.Is(err, command.ErrConflict) {
		t.Fatalf("same key different body: err=%v, want command.ErrConflict", err)
	}

	// A republish (new key) appends version 2.
	out, err = publish("tpl-publish-0002", msgtemplates.PublishInput{TemplateID: "welcome-msg", Name: "歡迎訊息v2", Kinds: []string{"dm"}, PublicSafe: false, Body: "嗨，歡迎光臨（改）"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Version != 2 {
		t.Fatalf("publish v2 version=%d, want 2", out.Version)
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM msgtemplates.templates WHERE tenant_id=$1 AND store_id=$2 AND template_id='welcome-msg'`, f.tenantA, f.storeA1); n != 2 {
		t.Fatalf("template rows after v2=%d, want 2", n)
	}

	// The list (inbox:reply) shows the latest version only.
	_, replierToken := lcPerson(t, f, f.tenantA, f.storeA1, "inbox:reply")
	var list msgtemplates.ListOutput
	err = platform.WithScope(ctx, f.runtime, replierToken, f.storeA1, "inbox:reply", func(tx pgx.Tx, _ platform.Scope) error {
		var err error
		list, err = svc.List(ctx, tx)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	var got *msgtemplates.ListItem
	for i := range list.Items {
		if list.Items[i].TemplateID == "welcome-msg" {
			got = &list.Items[i]
		}
	}
	if got == nil || got.Version != 2 || got.Name != "歡迎訊息v2" || !slices.Equal(got.Kinds, []string{"dm"}) || got.PublicSafe {
		t.Fatalf("list welcome-msg=%+v, want version 2", got)
	}
}

func TestLiveConsoleTemplatesPublicSafeFixedIDAndPermission(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	svc := msgtemplates.NewService()

	_, managerToken := lcPerson(t, f, f.tenantA, f.storeA1, "live:manage")
	replier, _ := lcPerson(t, f, f.tenantA, f.storeA1, "inbox:reply")
	_, viewerToken := lcPerson(t, f, f.tenantA, f.storeA1)

	t.Run("public_safe template with a URL is refused §3.5", func(t *testing.T) {
		err := platform.WithScope(ctx, f.runtime, managerToken, f.storeA1, "live:manage", func(tx pgx.Tx, scope platform.Scope) error {
			_, err := svc.Publish(ctx, tx, scope, "tpl-safe-0001", msgtemplates.PublishInput{
				TemplateID: "bad-url", Name: "帶連結", Kinds: []string{"dm"}, PublicSafe: true, Body: "看這裡 https://example.com/a",
			})
			return err
		})
		var coded *msgtemplates.Error
		if !errors.As(err, &coded) || coded.Status != 422 || coded.Code != "public_reply_forbidden_content" {
			t.Fatalf("public unsafe: err=%v, want 422 public_reply_forbidden_content", err)
		}
		if n := countRows(t, f.owner, `SELECT count(*) FROM msgtemplates.templates WHERE tenant_id=$1 AND store_id=$2 AND template_id='bad-url'`, f.tenantA, f.storeA1); n != 0 {
			t.Fatalf("refused template still stored: rows=%d", n)
		}
	})

	t.Run("public_reply/recommend kind without public_safe is refused", func(t *testing.T) {
		err := platform.WithScope(ctx, f.runtime, managerToken, f.storeA1, "live:manage", func(tx pgx.Tx, scope platform.Scope) error {
			_, err := svc.Publish(ctx, tx, scope, "tpl-safe-0002", msgtemplates.PublishInput{
				TemplateID: "bad-kind", Name: "公開回覆", Kinds: []string{"public_reply"}, PublicSafe: false, Body: "歡迎光臨",
			})
			return err
		})
		if !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("public kind without public_safe: err=%v, want command.ErrInvalid", err)
		}
	})

	t.Run("publishing a fixed id is a 409 template_fixed", func(t *testing.T) {
		err := platform.WithScope(ctx, f.runtime, managerToken, f.storeA1, "live:manage", func(tx pgx.Tx, scope platform.Scope) error {
			_, err := svc.Publish(ctx, tx, scope, "tpl-fixed-0001", msgtemplates.PublishInput{
				TemplateID: "order-pay-link/v1", Name: "x", Kinds: []string{"dm"}, PublicSafe: false, Body: "x",
			})
			return err
		})
		requirePGCode(t, err, "PT409", "publish fixed id")
	})

	t.Run("inbox:reply principal cannot publish (definer gate)", func(t *testing.T) {
		tx := lcRoleTx(t, f, "commerce_runtime", f.tenantA, f.storeA1, replier)
		defer tx.Rollback(ctx)
		_, err := tx.Exec(ctx, `SELECT * FROM msgtemplates.publish('x','x','x',ARRAY['dm'],false)`)
		requirePGCode(t, err, "PT403", "replier publish definer")
	})

	t.Run("store:read only principal cannot open live:manage scope", func(t *testing.T) {
		err := platform.WithScope(ctx, f.runtime, viewerToken, f.storeA1, "live:manage", func(pgx.Tx, platform.Scope) error {
			t.Fatal("viewer live:manage callback executed")
			return nil
		})
		if !errors.Is(err, platform.ErrForbidden) {
			t.Fatalf("viewer live:manage scope: err=%v, want ErrForbidden", err)
		}
	})
}

func TestLiveConsoleTemplatesResolveAndCrossStore(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	svc := msgtemplates.NewService()

	_, managerToken := lcPerson(t, f, f.tenantA, f.storeA1, "live:manage")
	_, replierA2Token := lcPerson(t, f, f.tenantA, f.storeA2, "inbox:reply")
	viewer, _ := lcPerson(t, f, f.tenantA, f.storeA1)

	// Publish a merchant template in storeA1.
	err := platform.WithScope(ctx, f.runtime, managerToken, f.storeA1, "live:manage", func(tx pgx.Tx, scope platform.Scope) error {
		_, err := svc.Publish(ctx, tx, scope, "tpl-resolve-0001", msgtemplates.PublishInput{
			TemplateID: "welcome-msg", Name: "歡迎訊息", Kinds: []string{"dm"}, PublicSafe: false, Body: "嗨，歡迎光臨",
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	resolve := func(token, store, id string, version int64) (msgtemplates.Resolved, error) {
		var out msgtemplates.Resolved
		err := platform.WithScope(ctx, f.runtime, token, store, "live:manage", func(tx pgx.Tx, _ platform.Scope) error {
			var err error
			out, err = svc.Resolve(ctx, tx, id, version)
			return err
		})
		return out, err
	}

	t.Run("fixed order-pay-link resolves with the sealed link placeholder", func(t *testing.T) {
		r, err := resolve(managerToken, f.storeA1, "order-pay-link/v1", 1)
		if err != nil {
			t.Fatal(err)
		}
		if !r.Fixed || r.PublicSafe || !slices.Equal(r.Kinds, []string{"dm"}) || !strings.Contains(r.Body, "{{連結}}") {
			t.Fatalf("order-pay-link resolve=%+v", r)
		}
	})

	t.Run("fixed offer-recommend resolves public_safe", func(t *testing.T) {
		r, err := resolve(managerToken, f.storeA1, "offer-recommend/v1", 1)
		if err != nil {
			t.Fatal(err)
		}
		if !r.Fixed || !r.PublicSafe || !slices.Equal(r.Kinds, []string{"recommend"}) || !strings.Contains(r.Body, "{{product.name}}") {
			t.Fatalf("offer-recommend resolve=%+v", r)
		}
	})

	t.Run("merchant template resolves by version", func(t *testing.T) {
		r, err := resolve(managerToken, f.storeA1, "welcome-msg", 1)
		if err != nil {
			t.Fatal(err)
		}
		if r.Fixed || r.PublicSafe || r.Body != "嗨，歡迎光臨" || !slices.Equal(r.Kinds, []string{"dm"}) {
			t.Fatalf("merchant resolve=%+v", r)
		}
	})

	t.Run("unknown id and unknown version are not found", func(t *testing.T) {
		if _, err := resolve(managerToken, f.storeA1, "does-not-exist", 1); !errors.Is(err, command.ErrNotFound) {
			t.Fatalf("unknown id: err=%v, want ErrNotFound", err)
		}
		if _, err := resolve(managerToken, f.storeA1, "welcome-msg", 99); !errors.Is(err, command.ErrNotFound) {
			t.Fatalf("unknown version: err=%v, want ErrNotFound", err)
		}
	})

	t.Run("cross-store resolve is not found (RLS isolation)", func(t *testing.T) {
		var out msgtemplates.Resolved
		err := platform.WithScope(ctx, f.runtime, replierA2Token, f.storeA2, "inbox:reply", func(tx pgx.Tx, _ platform.Scope) error {
			var err error
			out, err = svc.Resolve(ctx, tx, "welcome-msg", 1)
			return err
		})
		if !errors.Is(err, command.ErrNotFound) {
			t.Fatalf("cross-store resolve: err=%v, want ErrNotFound", err)
		}
		_ = out
	})

	t.Run("cross-store list is empty", func(t *testing.T) {
		var list msgtemplates.ListOutput
		err := platform.WithScope(ctx, f.runtime, replierA2Token, f.storeA2, "inbox:reply", func(tx pgx.Tx, _ platform.Scope) error {
			var err error
			list, err = svc.List(ctx, tx)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(list.Items) != 0 {
			t.Fatalf("cross-store list=%+v, want empty", list.Items)
		}
	})

	t.Run("resolve definer gate denies store:read only principal", func(t *testing.T) {
		tx := lcRoleTx(t, f, "commerce_runtime", f.tenantA, f.storeA1, viewer)
		defer tx.Rollback(ctx)
		_, err := tx.Exec(ctx, `SELECT * FROM msgtemplates.resolve('welcome-msg', 1)`)
		requirePGCode(t, err, "PT403", "viewer resolve definer")
	})

	t.Run("list definer gate denies store:read only principal", func(t *testing.T) {
		tx := lcRoleTx(t, f, "commerce_runtime", f.tenantA, f.storeA1, viewer)
		defer tx.Rollback(ctx)
		_, err := tx.Exec(ctx, `SELECT * FROM msgtemplates.list()`)
		requirePGCode(t, err, "PT403", "viewer list definer")
	})
}
