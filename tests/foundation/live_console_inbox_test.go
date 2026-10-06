package foundation_test

// Real-PG gate for the LC-B3 merchant inbox read side (contracts/live-console-v1.md §3.2/§3.6/§3.7,
// §11 A8-A11/A13/A14). Exercises migration 0119 exactly once (the shared fixture already applies it
// twice), the frozen definer/owner/EXECUTE-grant ACL, the inbox:read vs inbox:reply permission split
// (A8/A9/A10), A11 takeover CAS with lazy 6-hour expiry, A14 customer-link CAS + audit, and cross-store
// RLS isolation. No send path (LC-B4), no LIVE Meta traffic, no real buyer PII: fixtures are synthetic
// UUIDs and a synthetic buyer.owner.
//
// The read/write definers are called through platform.WithScope (the real service path) for positive
// flows and through SET LOCAL ROLE commerce_runtime/claims_worker with pinned GUCs for definer-level
// negatives and the claims-worker dm_window read (the merchant-orders definer-call idiom).

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/platform"
)

// lcConversation seeds a social.conversations row plus its inbox.conversation_state (defaults), the
// minimal pair every read/write definer below operates on. No social.messages / meta_inbox.events row is
// needed: the A8-A14 contracts under test touch only conversation + state + buyer.owners.
func lcConversation(t *testing.T, f *testFixture, tenant, store, object string) string {
	t.Helper()
	conv := randomUUID()
	// peer_key must be unique per conversation (UNIQUE(tenant_id,store_id,app_id,object,asset_id,peer_key)):
	// 64 lowercase hex chars derived from the fresh conversation id, so repeated seeds never collide.
	peerKey := strings.Repeat(strings.ReplaceAll(conv, "-", ""), 2)
	mustExec(t, f.owner, `INSERT INTO social.conversations(id,tenant_id,store_id,app_id,object,asset_id,peer_key)
		VALUES ($1,$2,$3,'123456789',$4,'1234567890',$5)`, conv, tenant, store, object, peerKey)
	mustExec(t, f.owner, `INSERT INTO inbox.conversation_state(tenant_id,store_id,conversation_id) VALUES ($1,$2,$3)`, tenant, store, conv)
	return conv
}

// lcPerson registers a merchant principal that is a member of the store with the given permissions
// (plus the store:read that identity.resolve_access requires first), and returns its id and bearer token.
// Callers never pass store:read themselves.
func lcPerson(t *testing.T, f *testFixture, tenant, store string, perms ...string) (principal, token string) {
	t.Helper()
	principal = randomUUID()
	token = randomToken()
	mustExec(t, f.owner, `INSERT INTO identity.principals(id) VALUES ($1)`, principal)
	mustExec(t, f.owner, `INSERT INTO identity.memberships(tenant_id,principal_id) VALUES ($1,$2)`, tenant, principal)
	for _, perm := range append([]string{"store:read"}, perms...) {
		mustExec(t, f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES ($1,$2,$3,$4)`, tenant, store, principal, perm)
	}
	mustExec(t, f.owner, `INSERT INTO identity.sessions(id,token_hash,principal_id,audience,expires_at) VALUES ($1,$2,$3,'merchant',$4)`,
		randomUUID(), tokenHash(token), principal, time.Now().Add(24*time.Hour))
	return principal, token
}

// lcRoleTx begins an owner transaction, switches to the given non-login role and pins the server-resolved
// tenant/store/principal GUCs, mirroring the definer-call idiom of the merchant-orders gate. role is a
// fixed migration role name, never caller input.
func lcRoleTx(t *testing.T, f *testFixture, role, tenant, store, principal string) pgx.Tx {
	t.Helper()
	tx, err := f.owner.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(context.Background(), `SET LOCAL ROLE `+role); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(context.Background(), `SELECT set_config('app.tenant_id',$1,true), set_config('app.store_id',$2,true), set_config('app.principal_id',$3,true)`, tenant, store, principal); err != nil {
		t.Fatal(err)
	}
	return tx
}

// lcOwnStores creates a fresh tenant with two stores so a test that reads the first page of
// social.list_conversations (50 rows, ordered last_at DESC then id DESC; a freshly seeded conversation has no
// timestamps, so its position is by random id) is not displaced by the hundreds of conversations other tests
// leave on the shared fixture store.
func lcOwnStores(t *testing.T, f *testFixture) (tenant, store1, store2 string) {
	t.Helper()
	tenant, store1, store2 = randomUUID(), randomUUID(), randomUUID()
	mustExec(t, f.owner, `INSERT INTO control.tenants(id,name) VALUES($1,'lc-inbox-own-tenant')`, tenant)
	mustExec(t, f.owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'lc-inbox-store-1','USD'),($1,$3,'lc-inbox-store-2','USD')`, tenant, store1, store2)
	return tenant, store1, store2
}

// lcListIDs returns the conversation ids visible through social.list_conversations under the current scope.
func lcListIDs(ctx context.Context, tx pgx.Tx) ([]string, error) {
	rows, err := tx.Query(ctx, `SELECT conversation_id::text FROM social.list_conversations('all', NULL, NULL, 50)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func TestLiveConsoleInboxMigration0122ExactACL(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()

	var migrationCount int
	if err := f.owner.QueryRow(ctx, `SELECT count(*) FROM public.lc_schema_migrations WHERE version='0119_live_console_inbox.sql'`).Scan(&migrationCount); err != nil {
		t.Fatal(err)
	}
	if migrationCount != 1 {
		t.Fatalf("0119 migration row count=%d, want exactly 1", migrationCount)
	}

	var login, super, bypass, createRole, createDB, replication bool
	if err := f.owner.QueryRow(ctx, `SELECT rolcanlogin,rolsuper,rolbypassrls,rolcreaterole,rolcreatedb,rolreplication FROM pg_roles WHERE rolname='commerce_inbox_writer'`).
		Scan(&login, &super, &bypass, &createRole, &createDB, &replication); err != nil {
		t.Fatal(err)
	}
	if login || super || bypass || createRole || createDB || replication {
		t.Fatalf("unsafe commerce_inbox_writer attributes: login=%t super=%t bypass=%t createRole=%t createDB=%t replication=%t", login, super, bypass, createRole, createDB, replication)
	}

	for _, table := range []string{"inbox.conversation_state", "inbox.thread_open_marks"} {
		var rls, force bool
		if err := f.owner.QueryRow(ctx, `SELECT relrowsecurity, relforcerowsecurity FROM pg_class WHERE oid=$1::regclass`, table).Scan(&rls, &force); err != nil || !rls || !force {
			t.Fatalf("table %s RLS: enabled=%t forced=%t err=%v", table, rls, force, err)
		}
	}

	var checkDef string
	if err := f.owner.QueryRow(ctx, `SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid='identity.store_grants'::regclass AND conname='store_grants_permission_check'`).Scan(&checkDef); err != nil {
		t.Fatal(err)
	}
	for _, perm := range []string{"inbox:read", "inbox:reply", "inventory:live_adjust"} {
		if !strings.Contains(checkDef, "'"+perm+"'") {
			t.Fatalf("store_grants_permission_check lacks %s: %s", perm, checkDef)
		}
	}

	var liveOperator []string
	if err := f.owner.QueryRow(ctx, `SELECT identity.staff_role_permissions('live_operator')`).Scan(&liveOperator); err != nil {
		t.Fatal(err)
	}
	if want := []string{"store:read", "live:read", "live:manage", "catalog:read", "orders:read", "inventory:read", "inbox:read", "inbox:reply", "inventory:live_adjust"}; !slices.Equal(liveOperator, want) {
		t.Fatalf("live_operator bundle=%v, want %v", liveOperator, want)
	}
	var viewer []string
	if err := f.owner.QueryRow(ctx, `SELECT identity.staff_role_permissions('viewer')`).Scan(&viewer); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(viewer, "inbox:read") {
		t.Fatalf("viewer bundle still carries inbox:read: %v", viewer)
	}
	if !slices.Contains(viewer, "store:read") {
		t.Fatalf("viewer bundle lost store:read: %v", viewer)
	}
	for _, perm := range viewer {
		if !strings.HasSuffix(perm, ":read") {
			t.Fatalf("viewer bundle has non-read permission %q", perm)
		}
	}
	var catalogue []string
	if err := f.owner.QueryRow(ctx, `SELECT identity.staff_permission_catalogue()`).Scan(&catalogue); err != nil {
		t.Fatal(err)
	}
	for _, perm := range []string{"inbox:read", "inbox:reply", "inventory:live_adjust"} {
		if !slices.Contains(catalogue, perm) {
			t.Fatalf("permission catalogue lacks %s", perm)
		}
	}

	var policyRoles []string
	if err := f.owner.QueryRow(ctx, `SELECT ARRAY(SELECT pg_get_userbyid(x) FROM unnest(polroles) x ORDER BY 1)
		FROM pg_policy WHERE polname='owners_inbox_lookup' AND polrelid='buyer.owners'::regclass`).Scan(&policyRoles); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(policyRoles, []string{"commerce_inbox_writer"}) {
		t.Fatalf("owners_inbox_lookup roles=%v, want [commerce_inbox_writer]", policyRoles)
	}

	functions := []struct {
		signature, owner, result string
		stable                   bool
		grantees                 []string
	}{
		{"inbox.principal_holds(text[])", "commerce_auth", "boolean", true,
			[]string{"commerce_auth", "commerce_inbox_writer", "commerce_meta_writer", "commerce_msgtemplates_writer"}},
		{"inbox.advance_conversation_state()", "commerce_meta_writer", "trigger", false,
			[]string{"commerce_meta_writer"}},
		{"inbox.dm_window(uuid,uuid,uuid)", "commerce_meta_writer",
			"TABLE(last_inbound_at timestamp with time zone, mode text, takeover_generation bigint, human_until timestamp with time zone)", true,
			[]string{"commerce_claims_worker", "commerce_meta_writer"}},
		{"social.read_thread(uuid,bigint,int)", "commerce_meta_writer",
			"TABLE(event_id uuid, key_id text, nonce bytea, ciphertext bytea, app_id text, object text, asset_id text, event_key text, payload_hash text, tenant_id uuid, store_id uuid, route_id uuid, route_epoch bigint, server_seq bigint, occurred_at timestamp with time zone, direction text)", true,
			[]string{"commerce_meta_writer", "commerce_runtime"}},
		{"social.list_conversations(text,timestamptz,uuid,int)", "commerce_meta_writer",
			"TABLE(conversation_id uuid, platform text, last_at timestamp with time zone, unread boolean, unreplied boolean, mode text, assignee uuid, window_open_until timestamp with time zone, linked_customer_id uuid)", true,
			[]string{"commerce_meta_writer", "commerce_runtime"}},
		{"social.conversation_meta(uuid)", "commerce_meta_writer",
			"TABLE(platform text, mode text, assignee uuid, takeover_generation bigint, human_until timestamp with time zone, window_open_until timestamp with time zone, last_inbound_at timestamp with time zone, last_inbound_seq bigint, read_seq bigint, linked_customer_id uuid, status text)", true,
			[]string{"commerce_meta_writer", "commerce_runtime"}},
		{"social.unread_conversation_count()", "commerce_meta_writer", "bigint", true,
			[]string{"commerce_meta_writer", "commerce_runtime"}},
		{"inbox.mark_read(uuid,bigint)", "commerce_inbox_writer", "bigint", false,
			[]string{"commerce_inbox_writer", "commerce_runtime"}},
		{"inbox.takeover(uuid,bigint)", "commerce_inbox_writer",
			"TABLE(mode text, assignee uuid, takeover_generation bigint)", false,
			[]string{"commerce_inbox_writer", "commerce_runtime"}},
		{"inbox.release(uuid,bigint)", "commerce_inbox_writer",
			"TABLE(mode text, assignee uuid, takeover_generation bigint)", false,
			[]string{"commerce_inbox_writer", "commerce_runtime"}},
		{"inbox.customer_link(uuid,uuid,bigint)", "commerce_inbox_writer",
			"TABLE(customer_id uuid, version bigint)", false,
			[]string{"commerce_inbox_writer", "commerce_runtime"}},
		{"inbox.thread_opened(uuid)", "commerce_inbox_writer", "void", false,
			[]string{"commerce_inbox_writer", "commerce_runtime"}},
		{"integration.finish_meta_resubscribe(uuid,text,text)", "commerce_integration_writer", "boolean", false,
			[]string{"commerce_claims_worker", "commerce_integration_writer"}},
		{"integration.claim_meta_resubscribe()", "commerce_integration_writer",
			"TABLE(o_job uuid, o_tenant uuid, o_store uuid, o_binding uuid, o_page text, o_version bigint, o_key_id text, o_nonce bytea, o_ciphertext bytea, o_attempt integer)", false,
			[]string{"commerce_claims_worker", "commerce_integration_writer"}},
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
			if owner != fn.owner || result != fn.result || !definer || !noLogin || !noBypass || !fixedPath || volatility != wantVolatility ||
				!slices.Equal(principals, fn.grantees) {
				t.Fatalf("0119 boundary: owner=%s result=%s definer=%v noLogin=%v noBypass=%v path=%v volatility=%s ACL=%v", owner, result, definer, noLogin, noBypass, fixedPath, volatility, principals)
			}
			var others int
			if err := f.owner.QueryRow(ctx, `SELECT count(*) FROM pg_roles WHERE rolname LIKE 'commerce\_%'
			 AND NOT (rolname = ANY($1::text[])) AND has_function_privilege(oid, $2, 'EXECUTE')`, fn.grantees, fn.signature).Scan(&others); err != nil || others != 0 {
				t.Fatalf("unexpected helper callers=%d err=%v", others, err)
			}
		})
	}
}

func TestLiveConsoleInboxLCN03PermissionSplit(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	tenantA, storeA1, _ := lcOwnStores(t, f)

	viewerPrincipal, viewerToken := lcPerson(t, f, tenantA, storeA1)
	readerPrincipal, readerToken := lcPerson(t, f, tenantA, storeA1, "inbox:read")
	replierPrincipal, replierToken := lcPerson(t, f, tenantA, storeA1, "inbox:read", "inbox:reply")

	convRead := lcConversation(t, f, tenantA, storeA1, "page")
	mustExec(t, f.owner, `UPDATE inbox.conversation_state SET last_inbound_seq=7, read_seq=3, last_inbound_at=clock_timestamp() WHERE conversation_id=$1`, convRead)
	convReply := lcConversation(t, f, tenantA, storeA1, "page")
	mustExec(t, f.owner, `UPDATE inbox.conversation_state SET last_inbound_seq=7, read_seq=3, last_inbound_at=clock_timestamp() WHERE conversation_id=$1`, convReply)

	t.Run("viewer cannot open inbox:read scope", func(t *testing.T) {
		err := platform.WithScope(ctx, f.runtime, viewerToken, storeA1, "inbox:read", func(pgx.Tx, platform.Scope) error {
			t.Fatal("unauthorized inbox:read callback executed")
			return nil
		})
		if !errors.Is(err, platform.ErrForbidden) {
			t.Fatalf("error=%v, want ErrForbidden", err)
		}
	})

	t.Run("viewer definer gate denies read and mark_read", func(t *testing.T) {
		tx := lcRoleTx(t, f, "commerce_runtime", tenantA, storeA1, viewerPrincipal)
		_, err := tx.Exec(ctx, `SELECT * FROM social.list_conversations('all', NULL, NULL, 50)`)
		requirePGCode(t, err, "PT403", "viewer list_conversations")
		_ = tx.Rollback(ctx)

		tx = lcRoleTx(t, f, "commerce_runtime", tenantA, storeA1, viewerPrincipal)
		_, err = tx.Exec(ctx, `SELECT inbox.mark_read($1, $2)`, convRead, int64(7))
		requirePGCode(t, err, "PT403", "viewer mark_read")
		_ = tx.Rollback(ctx)
	})

	t.Run("reader sees the conversation and mark_read stays put without inbox:reply", func(t *testing.T) {
		var ids []string
		var got int64
		err := platform.WithScope(ctx, f.runtime, readerToken, storeA1, "inbox:read", func(tx pgx.Tx, scope platform.Scope) error {
			if scope.PrincipalID != readerPrincipal {
				t.Fatalf("scope principal=%s, want %s", scope.PrincipalID, readerPrincipal)
			}
			var err error
			if ids, err = lcListIDs(ctx, tx); err != nil {
				return err
			}
			return tx.QueryRow(ctx, `SELECT inbox.mark_read($1, $2)`, convRead, int64(7)).Scan(&got)
		})
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(ids, convRead) {
			t.Fatalf("reader list missing convRead=%s: %v", convRead, ids)
		}
		if got != 3 {
			t.Fatalf("reader mark_read=%d, want 3 (unchanged)", got)
		}
		var readSeq int64
		if err := f.owner.QueryRow(ctx, `SELECT read_seq FROM inbox.conversation_state WHERE conversation_id=$1`, convRead).Scan(&readSeq); err != nil {
			t.Fatal(err)
		}
		if readSeq != 3 {
			t.Fatalf("reader row read_seq=%d, want 3", readSeq)
		}
	})

	t.Run("replier mark_read advances to 7", func(t *testing.T) {
		var got int64
		err := platform.WithScope(ctx, f.runtime, replierToken, storeA1, "inbox:read", func(tx pgx.Tx, scope platform.Scope) error {
			if scope.PrincipalID != replierPrincipal {
				t.Fatalf("scope principal=%s, want %s", scope.PrincipalID, replierPrincipal)
			}
			return tx.QueryRow(ctx, `SELECT inbox.mark_read($1, $2)`, convReply, int64(7)).Scan(&got)
		})
		if err != nil {
			t.Fatal(err)
		}
		if got != 7 {
			t.Fatalf("replier mark_read=%d, want 7", got)
		}
		var readSeq int64
		if err := f.owner.QueryRow(ctx, `SELECT read_seq FROM inbox.conversation_state WHERE conversation_id=$1`, convReply).Scan(&readSeq); err != nil {
			t.Fatal(err)
		}
		if readSeq != 7 {
			t.Fatalf("replier row read_seq=%d, want 7", readSeq)
		}
	})

	t.Run("A9 thread_opened requires inbox:read and coalesces its audit", func(t *testing.T) {
		conv := lcConversation(t, f, tenantA, storeA1, "page")
		tx := lcRoleTx(t, f, "commerce_runtime", tenantA, storeA1, viewerPrincipal)
		_, err := tx.Exec(ctx, `SELECT inbox.thread_opened($1)`, conv)
		requirePGCode(t, err, "PT403", "viewer thread_opened")
		if err := tx.Rollback(ctx); err != nil && err != pgx.ErrTxClosed {
			t.Fatal(err)
		}
		for i := 0; i < 2; i++ {
			err := platform.WithScope(ctx, f.runtime, readerToken, storeA1, "inbox:read", func(tx pgx.Tx, _ platform.Scope) error {
				_, err := tx.Exec(ctx, `SELECT inbox.thread_opened($1)`, conv)
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
		}
		if n := countRows(t, f.owner, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='inbox.thread_opened' AND principal_id=$3`, tenantA, storeA1, readerPrincipal); n != 1 {
			t.Fatalf("thread_opened audit rows=%d, want 1", n)
		}
		if n := countRows(t, f.owner, `SELECT count(*) FROM inbox.thread_open_marks WHERE conversation_id=$1 AND principal_id=$2`, conv, readerPrincipal); n != 1 {
			t.Fatalf("thread_open_marks rows=%d, want 1", n)
		}
	})
}

func TestLiveConsoleInboxLCN10TakeoverExpiryCustomerLink(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	tenantA, storeA1, storeA2 := lcOwnStores(t, f)

	replierPrincipal, replierToken := lcPerson(t, f, tenantA, storeA1, "inbox:read", "inbox:reply", "customers:read")
	readerPrincipal, readerToken := lcPerson(t, f, tenantA, storeA1, "inbox:read")

	t.Run("A11 takeover/release CAS and audit", func(t *testing.T) {
		conv := lcConversation(t, f, tenantA, storeA1, "page")
		var mode, assignee string
		var gen int64
		err := platform.WithScope(ctx, f.runtime, replierToken, storeA1, "inbox:reply", func(tx pgx.Tx, _ platform.Scope) error {
			return tx.QueryRow(ctx, `SELECT mode, coalesce(assignee::text,''), takeover_generation FROM inbox.takeover($1, $2)`, conv, int64(0)).Scan(&mode, &assignee, &gen)
		})
		if err != nil {
			t.Fatal(err)
		}
		if mode != "human" || assignee != replierPrincipal || gen != 1 {
			t.Fatalf("takeover=%q/%q/%d, want human/%s/1", mode, assignee, gen, replierPrincipal)
		}
		var rowMode, rowAssignee string
		var rowGen int64
		var humanNull bool
		if err := f.owner.QueryRow(ctx, `SELECT mode, coalesce(assignee_principal::text,''), takeover_generation, human_until IS NULL FROM inbox.conversation_state WHERE conversation_id=$1`, conv).Scan(&rowMode, &rowAssignee, &rowGen, &humanNull); err != nil {
			t.Fatal(err)
		}
		if rowMode != "human" || rowAssignee != replierPrincipal || rowGen != 1 || !humanNull {
			t.Fatalf("takeover row=%q/%q/%d/null=%t, want human/%s/1/null", rowMode, rowAssignee, rowGen, humanNull, replierPrincipal)
		}
		if n := countRows(t, f.owner, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='inbox.takeover' AND principal_id=$3`, tenantA, storeA1, replierPrincipal); n != 1 {
			t.Fatalf("takeover audit rows=%d, want 1", n)
		}

		err = platform.WithScope(ctx, f.runtime, replierToken, storeA1, "inbox:reply", func(tx pgx.Tx, _ platform.Scope) error {
			_, err := tx.Exec(ctx, `SELECT * FROM inbox.takeover($1, $2)`, conv, int64(0))
			return err
		})
		requirePGCode(t, err, "PT409", "stale takeover CAS")

		err = platform.WithScope(ctx, f.runtime, replierToken, storeA1, "inbox:reply", func(tx pgx.Tx, _ platform.Scope) error {
			return tx.QueryRow(ctx, `SELECT mode, coalesce(assignee::text,''), takeover_generation FROM inbox.release($1, $2)`, conv, int64(1)).Scan(&mode, &assignee, &gen)
		})
		if err != nil {
			t.Fatal(err)
		}
		if mode != "auto" || assignee != "" || gen != 2 {
			t.Fatalf("release=%q/%q/%d, want auto//2", mode, assignee, gen)
		}
		if n := countRows(t, f.owner, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='inbox.release' AND principal_id=$3`, tenantA, storeA1, replierPrincipal); n != 1 {
			t.Fatalf("release audit rows=%d, want 1", n)
		}
	})

	t.Run("lazy expiry reads auto + generation+1 with cleared assignee", func(t *testing.T) {
		conv := lcConversation(t, f, tenantA, storeA1, "page")
		mustExec(t, f.owner, `UPDATE inbox.conversation_state SET mode='human', assignee_principal=$2, human_until=clock_timestamp()-interval '1 hour', takeover_generation=2 WHERE conversation_id=$1`, conv, replierPrincipal)

		var mode, assignee, humanUntil string
		var gen int64
		err := platform.WithScope(ctx, f.runtime, readerToken, storeA1, "inbox:read", func(tx pgx.Tx, _ platform.Scope) error {
			return tx.QueryRow(ctx, `SELECT mode, coalesce(assignee::text,''), takeover_generation, coalesce(human_until::text,'') FROM social.conversation_meta($1)`, conv).Scan(&mode, &assignee, &gen, &humanUntil)
		})
		if err != nil {
			t.Fatal(err)
		}
		if mode != "auto" || assignee != "" || gen != 3 || humanUntil != "" {
			t.Fatalf("expired meta=%q/%q/%d/%q, want auto//3/", mode, assignee, gen, humanUntil)
		}

		var listMode, listAssignee string
		err = platform.WithScope(ctx, f.runtime, readerToken, storeA1, "inbox:read", func(tx pgx.Tx, _ platform.Scope) error {
			return tx.QueryRow(ctx, `SELECT mode, coalesce(assignee::text,'') FROM social.list_conversations('all', NULL, NULL, 50) WHERE conversation_id::text=$1`, conv).Scan(&listMode, &listAssignee)
		})
		if err != nil {
			t.Fatal(err)
		}
		if listMode != "auto" || listAssignee != "" {
			t.Fatalf("expired list mode/assignee=%q/%q, want auto/", listMode, listAssignee)
		}

		tx := lcRoleTx(t, f, "commerce_claims_worker", tenantA, storeA1, readerPrincipal)
		defer tx.Rollback(ctx)
		var dmMode, dmUntil string
		var dmGen int64
		if err := tx.QueryRow(ctx, `SELECT mode, takeover_generation, coalesce(human_until::text,'') FROM inbox.dm_window($1,$2,$3)`, tenantA, storeA1, conv).Scan(&dmMode, &dmGen, &dmUntil); err != nil {
			t.Fatal(err)
		}
		if dmMode != "auto" || dmGen != 3 || dmUntil != "" {
			t.Fatalf("expired dm_window=%q/%d/%q, want auto/3/", dmMode, dmGen, dmUntil)
		}
	})

	t.Run("A14 customer link CAS, audit and foreign 404", func(t *testing.T) {
		conv := lcConversation(t, f, tenantA, storeA1, "page")
		customerID := randomUUID()
		mustExec(t, f.owner, `INSERT INTO buyer.owners(id,tenant_id,store_id) VALUES ($1,$2,$3)`, customerID, tenantA, storeA1)
		foreignCustomer := randomUUID()
		mustExec(t, f.owner, `INSERT INTO buyer.owners(id,tenant_id,store_id) VALUES ($1,$2,$3)`, foreignCustomer, tenantA, storeA2)

		var linked string
		var version int64
		err := platform.WithScope(ctx, f.runtime, replierToken, storeA1, "inbox:reply", func(tx pgx.Tx, _ platform.Scope) error {
			return tx.QueryRow(ctx, `SELECT coalesce(customer_id::text,''), version FROM inbox.customer_link($1, $2, $3)`, conv, customerID, int64(0)).Scan(&linked, &version)
		})
		if err != nil {
			t.Fatal(err)
		}
		if linked != customerID || version != 1 {
			t.Fatalf("customer_link=%q/%d, want %s/1", linked, version, customerID)
		}
		var rowCustomer, rowPrincipal string
		var rowVersion int64
		if err := f.owner.QueryRow(ctx, `SELECT coalesce(customer_id::text,''), coalesce(customer_link_principal::text,''), version FROM inbox.conversation_state WHERE conversation_id=$1`, conv).Scan(&rowCustomer, &rowPrincipal, &rowVersion); err != nil {
			t.Fatal(err)
		}
		if rowCustomer != customerID || rowPrincipal != replierPrincipal || rowVersion != 1 {
			t.Fatalf("customer_link row=%q/%q/%d, want %s/%s/1", rowCustomer, rowPrincipal, rowVersion, customerID, replierPrincipal)
		}
		if n := countRows(t, f.owner, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='inbox.customer_linked' AND principal_id=$3`, tenantA, storeA1, replierPrincipal); n != 1 {
			t.Fatalf("customer_linked audit rows=%d, want 1", n)
		}

		err = platform.WithScope(ctx, f.runtime, replierToken, storeA1, "inbox:reply", func(tx pgx.Tx, _ platform.Scope) error {
			_, err := tx.Exec(ctx, `SELECT * FROM inbox.customer_link($1, $2, $3)`, conv, customerID, int64(0))
			return err
		})
		requirePGCode(t, err, "PT409", "stale customer link CAS")

		err = platform.WithScope(ctx, f.runtime, replierToken, storeA1, "inbox:reply", func(tx pgx.Tx, _ platform.Scope) error {
			return tx.QueryRow(ctx, `SELECT coalesce(customer_id::text,''), version FROM inbox.customer_link($1, NULL, $2)`, conv, int64(1)).Scan(&linked, &version)
		})
		if err != nil {
			t.Fatal(err)
		}
		if linked != "" || version != 2 {
			t.Fatalf("unlink=%q/%d, want /2", linked, version)
		}
		if n := countRows(t, f.owner, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='inbox.customer_unlinked' AND principal_id=$3`, tenantA, storeA1, replierPrincipal); n != 1 {
			t.Fatalf("customer_unlinked audit rows=%d, want 1", n)
		}

		err = platform.WithScope(ctx, f.runtime, replierToken, storeA1, "inbox:reply", func(tx pgx.Tx, _ platform.Scope) error {
			_, err := tx.Exec(ctx, `SELECT * FROM inbox.customer_link($1, $2, $3)`, conv, foreignCustomer, int64(2))
			return err
		})
		requirePGCode(t, err, "PT404", "foreign customer link")
	})
}

func TestLiveConsoleInboxCrossStoreIsolation(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	tenantA, storeA1, storeA2 := lcOwnStores(t, f)
	tenantB, storeB, _ := lcOwnStores(t, f)

	_, readerToken := lcPerson(t, f, tenantA, storeA1, "inbox:read")
	_, replierToken := lcPerson(t, f, tenantA, storeA1, "inbox:read", "inbox:reply")

	convA1 := lcConversation(t, f, tenantA, storeA1, "page")
	convA2 := lcConversation(t, f, tenantA, storeA2, "page")
	convB := lcConversation(t, f, tenantB, storeB, "page")

	var ids []string
	err := platform.WithScope(ctx, f.runtime, readerToken, storeA1, "inbox:read", func(tx pgx.Tx, _ platform.Scope) error {
		var err error
		ids, err = lcListIDs(ctx, tx)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(ids, convA1) {
		t.Fatalf("storeA1 reader missing convA1=%s: %v", convA1, ids)
	}
	if slices.Contains(ids, convA2) || slices.Contains(ids, convB) {
		t.Fatalf("cross-store leak in list: %v", ids)
	}

	var leaked bool
	err = platform.WithScope(ctx, f.runtime, readerToken, storeA1, "inbox:read", func(tx pgx.Tx, _ platform.Scope) error {
		rows, err := tx.Query(ctx, `SELECT * FROM social.conversation_meta($1)`, convA2)
		if err != nil {
			return err
		}
		defer rows.Close()
		leaked = rows.Next()
		return rows.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	if leaked {
		t.Fatal("conversation_meta leaked a cross-store conversation")
	}

	err = platform.WithScope(ctx, f.runtime, replierToken, storeA1, "inbox:reply", func(tx pgx.Tx, _ platform.Scope) error {
		_, err := tx.Exec(ctx, `SELECT * FROM inbox.takeover($1, $2)`, convA2, int64(0))
		return err
	})
	requirePGCode(t, err, "PT404", "cross-store takeover")
}
