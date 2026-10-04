package foundation_test

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/platform"
	"livecommerce/migrations"
)

// A correct role graph is insufficient if a login can mutate the old River
// schema through a direct or inherited object grant. This is a real PG
// prefetch gate, not an assumption about ordinary application DML.
func TestMetaRuntimeIsolationWorkerObjectACL(t *testing.T) {
	f := mrFixture(t)
	ctx := context.Background()
	for _, tc := range []struct {
		name             string
		custom, settable bool
		public           bool
		grants, revokes  []string
	}{
		{"direct old table", false, false, false, []string{`GRANT USAGE ON SCHEMA river TO $ROLE`, `GRANT UPDATE ON river.river_job TO $ROLE`}, []string{`REVOKE ALL ON river.river_job FROM $ROLE`, `REVOKE ALL ON SCHEMA river FROM $ROLE`}},
		{"direct old column", false, false, false, []string{`GRANT USAGE ON SCHEMA river TO $ROLE`, `GRANT UPDATE(state) ON river.river_job TO $ROLE`}, []string{`REVOKE UPDATE(state) ON river.river_job FROM $ROLE`, `REVOKE ALL ON SCHEMA river FROM $ROLE`}},
		{"direct old sequence", false, false, false, []string{`GRANT USAGE ON SCHEMA river TO $ROLE`, `GRANT USAGE ON SEQUENCE river.river_job_id_seq TO $ROLE`}, []string{`REVOKE ALL ON SEQUENCE river.river_job_id_seq FROM $ROLE`, `REVOKE ALL ON SCHEMA river FROM $ROLE`}},
		{"inherited old table", true, false, false, []string{`GRANT USAGE ON SCHEMA river TO $ROLE`, `GRANT UPDATE ON river.river_job TO $ROLE`}, []string{`REVOKE ALL ON river.river_job FROM $ROLE`, `REVOKE ALL ON SCHEMA river FROM $ROLE`}},
		{"set reachable old table", true, true, false, []string{`GRANT USAGE ON SCHEMA river TO $ROLE`, `GRANT UPDATE ON river.river_job TO $ROLE`}, []string{`REVOKE ALL ON river.river_job FROM $ROLE`, `REVOKE ALL ON SCHEMA river FROM $ROLE`}},
		{"PUBLIC old table", false, false, true, []string{`GRANT USAGE ON SCHEMA river TO $ROLE`, `GRANT UPDATE ON river.river_job TO $ROLE`}, []string{`REVOKE ALL ON river.river_job FROM $ROLE`, `REVOKE ALL ON SCHEMA river FROM $ROLE`}},
		{"migration ledger", false, false, false, []string{`GRANT UPDATE(version) ON public.lc_schema_migrations TO $ROLE`}, []string{`REVOKE UPDATE(version) ON public.lc_schema_migrations FROM $ROLE`}},
		{"Meta River ledger table", false, false, false, []string{`GRANT UPDATE ON river_meta.river_migration TO $ROLE`}, []string{`REVOKE ALL ON river_meta.river_migration FROM $ROLE`}},
		{"Meta River ledger column", false, false, false, []string{`GRANT UPDATE(version) ON river_meta.river_migration TO $ROLE`}, []string{`REVOKE UPDATE(version) ON river_meta.river_migration FROM $ROLE`}},
		{"schema CREATE", false, false, false, []string{`GRANT CREATE ON SCHEMA social TO $ROLE`}, []string{`REVOKE CREATE ON SCHEMA social FROM $ROLE`}},
		{"social table", false, false, false, []string{`GRANT USAGE ON SCHEMA social TO $ROLE`, `GRANT UPDATE ON social.messages TO $ROLE`}, []string{`REVOKE ALL ON social.messages FROM $ROLE`, `REVOKE ALL ON SCHEMA social FROM $ROLE`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dsn := miRole(t, f, "commerce_meta_worker")
			if p, err := platform.OpenMetaWorkerPool(ctx, dsn); err != nil {
				t.Fatal("clean Meta worker denied", err)
			} else {
				p.Close()
			}
			u, err := url.Parse(dsn)
			if err != nil {
				t.Fatal(err)
			}
			login := pgx.Identifier{u.User.Username()}.Sanitize()
			grantee := login
			if tc.public {
				grantee = "PUBLIC"
			}
			if tc.custom {
				custom := pgx.Identifier{"mi_acl_" + t04Tag()}.Sanitize()
				mustExec(t, f.owner, `CREATE ROLE `+custom+` NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION`)
				membership := ` WITH INHERIT TRUE, SET FALSE`
				if tc.settable {
					membership = ` WITH INHERIT FALSE, SET TRUE`
				}
				mustExec(t, f.owner, `GRANT `+custom+` TO `+login+membership)
				t.Cleanup(func() {
					mustExec(t, f.owner, `REVOKE `+custom+` FROM `+login)
					mustExec(t, f.owner, `DROP ROLE `+custom)
				})
				grantee = custom
			}
			t.Cleanup(func() {
				for _, undo := range tc.revokes {
					mustExec(t, f.owner, strings.ReplaceAll(undo, "$ROLE", grantee))
				}
			})
			for _, grant := range tc.grants {
				mustExec(t, f.owner, strings.ReplaceAll(grant, "$ROLE", grantee))
			}
			if p, err := platform.OpenMetaWorkerPool(ctx, dsn); err == nil {
				p.Close()
				t.Fatal("Meta worker with effective cross-domain privilege admitted")
			}
			borrowed, err := pgxpool.New(ctx, dsn)
			if err != nil {
				t.Fatal(err)
			}
			defer borrowed.Close()
			if err := platform.ValidateMetaWorkerPool(ctx, borrowed); err == nil {
				t.Fatal("borrowed Meta worker pool with effective cross-domain privilege admitted")
			}
		})
	}
}

func TestMetaRuntimeIsolationWorkerRoleMatrix(t *testing.T) {
	f := mrFixture(t)
	ctx := context.Background()
	for _, role := range []string{"postgres", waPayment, waLive, waExpiry, waAds, waClaims, waLegacy, "commerce_runtime", "commerce_meta_ingress", "commerce_meta_consumer"} {
		t.Run("wrong role "+role, func(t *testing.T) {
			roleDSN := f.databaseURL
			if role != "postgres" {
				roleDSN = miRole(t, f, role)
			}
			if p, err := platform.OpenMetaWorkerPool(ctx, roleDSN); err == nil {
				p.Close()
				t.Fatal("nonexclusive Meta lifecycle role admitted", role)
			}
		})
	}
	dsn := miRole(t, f, "commerce_meta_worker")
	if p, err := platform.OpenMetaWorkerPool(ctx, dsn); err != nil {
		t.Fatal("clean Meta lifecycle role denied", err)
	} else {
		p.Close()
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	login := pgx.Identifier{u.User.Username()}.Sanitize()
	for _, other := range []string{waPayment, waLive, waExpiry, waAds, waClaims, waLegacy, "commerce_runtime", "commerce_meta_ingress", "commerce_meta_consumer", "pg_read_all_data", "pg_write_all_data"} {
		t.Run("mixed "+other, func(t *testing.T) {
			granted := pgx.Identifier{other}.Sanitize()
			mustExec(t, f.owner, `GRANT `+granted+` TO `+login+` WITH INHERIT TRUE, SET FALSE`)
			defer mustExec(t, f.owner, `REVOKE `+granted+` FROM `+login)
			if p, err := platform.OpenMetaWorkerPool(ctx, dsn); err == nil {
				p.Close()
				t.Fatal("mixed lifecycle authority admitted", other)
			}
			borrowed, err := pgxpool.New(ctx, dsn)
			if err != nil {
				t.Fatal(err)
			}
			defer borrowed.Close()
			if err := platform.ValidateMetaWorkerPool(ctx, borrowed); err == nil {
				t.Fatal("borrowed mixed lifecycle authority admitted", other)
			}
		})
	}
	ownerRole := pgx.Identifier{"mi_owner_" + t04Tag()}.Sanitize()
	owned := pgx.Identifier{"mi_owned_" + t04Tag()}.Sanitize()
	mustExec(t, f.owner, `CREATE ROLE `+ownerRole+` NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION`)
	mustExec(t, f.owner, `CREATE TABLE public.`+owned+`(id integer)`)
	mustExec(t, f.owner, `ALTER TABLE public.`+owned+` OWNER TO `+ownerRole)
	defer func() {
		mustExec(t, f.owner, `REVOKE `+ownerRole+` FROM `+login)
		mustExec(t, f.owner, `DROP TABLE public.`+owned)
		mustExec(t, f.owner, `DROP ROLE `+ownerRole)
	}()
	mustExec(t, f.owner, `GRANT `+ownerRole+` TO `+login+` WITH INHERIT TRUE, SET FALSE`)
	if p, err := platform.OpenMetaWorkerPool(ctx, dsn); err == nil {
		p.Close()
		t.Fatal("inherited object owner admitted as Meta lifecycle worker")
	}
	borrowed, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer borrowed.Close()
	if err := platform.ValidateMetaWorkerPool(ctx, borrowed); err == nil {
		t.Fatal("borrowed inherited owner admitted")
	}
	for _, role := range []string{waPayment, waLive, waExpiry, waAds, waClaims} {
		if _, err := miPool(t, f, role).Exec(ctx, `UPDATE river_meta.river_job SET state='available' WHERE id=-1`); miSQLState(err) != "42501" {
			t.Fatalf("ordinary worker %s crossed Meta schema state=%s", role, miSQLState(err))
		}
	}
	if _, err := borrowed.Exec(ctx, `UPDATE river.river_job SET state='available' WHERE id=-1`); miSQLState(err) != "42501" {
		t.Fatalf("Meta worker crossed ordinary River schema state=%s", miSQLState(err))
	}
}

func miIsoRows(t *testing.T, f *testFixture, table, predicate string) string {
	t.Helper()
	var rows string
	if err := f.owner.QueryRow(context.Background(), `SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text),'[]'::jsonb)::text FROM `+table+` x `+predicate).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	return rows
}

// This is the latest populated cutover, separate from both exact historical
// 0028->0029 and 0029->0030 upgrade tests.
func TestMetaRuntimeIsolationPopulated0030Cutover(t *testing.T) {
	f := mcPre0029Fixture(t)
	ctx := context.Background()
	mcApplyHistorical(t, f, "0029_meta_social_consumer.sql", "0030_meta_runtime.sql")
	page := mcOldSetup(t, f, "page")
	pageAsset := miAsset()
	pageBinding := miBinding(t, page, pageAsset, "facebook", f.tenantA, f.storeA1, f.principalA)
	miRoute(t, page, pageAsset, f.tenantA, f.storeA1, pageBinding)
	ig := mcOldSetup(t, f, "instagram")
	igAsset := miAsset()
	igBinding := miBinding(t, ig, igAsset, "instagram", f.tenantA, f.storeA1, f.principalA)
	miInstagramRoute(t, ig, igAsset, igBinding)
	pageRaw := miMessage(pageAsset, "m."+randomUUID(), "cutover-page")
	scheduled := mcOldPost(t, page, pageAsset, pageRaw)
	if replay := mcOldPost(t, page, pageAsset, pageRaw); replay.id != scheduled.id || replay.job != scheduled.job {
		t.Fatal("old-lane duplicate changed event or job identity")
	}
	igRaw := []byte(fmt.Sprintf(`{"object":"instagram","entry":[{"id":%q,"messaging":[{"sender":{"id":"4"},"recipient":{"id":%q},"message":{"mid":%q,"text":"cutover-ig"}}]}]}`, igAsset, igAsset, "m."+randomUUID()))
	retryable := mcOldPost(t, ig, igAsset, igRaw)
	terminal := mcOldPost(t, page, pageAsset, miMessage(pageAsset, "m."+randomUUID(), "cutover-terminal"))
	pruned := mcOldPost(t, page, pageAsset, miMessage(pageAsset, "m."+randomUUID(), "cutover-pruned"))
	quarantineAsset := miAsset()
	quarantined := mcOldPost(t, page, quarantineAsset, miMessage(quarantineAsset, "m."+randomUUID(), "cutover-quarantine"))
	if quarantined.job != 0 || miCount(t, f.owner, `SELECT count(*) FROM meta_inbox.events WHERE id=$1 AND disposition='QUARANTINED'`, quarantined.id) != 1 {
		t.Fatal("old-lane unknown asset not quarantined")
	}
	mustExec(t, f.owner, `UPDATE river.river_job SET state='scheduled',scheduled_at=clock_timestamp()+interval '1 minute' WHERE id=$1`, scheduled.job)
	mustExec(t, f.owner, `UPDATE river.river_job SET state='retryable',attempt=1,attempted_at=clock_timestamp(),scheduled_at=clock_timestamp()+interval '1 minute' WHERE id=$1`, retryable.job)
	for _, e := range []mcEvent{terminal, pruned} {
		mcRunning(t, page, e, 1)
		mcFinish(t, mcConsumer(t, page), e, 1, mcSubject())
		mustExec(t, f.owner, `UPDATE river.river_job SET state='completed',finalized_at=clock_timestamp() WHERE id=$1`, e.job)
	}
	mustExec(t, f.owner, `DELETE FROM river.river_job WHERE id=$1`, pruned.job)
	mustExec(t, f.owner, `INSERT INTO river.river_queue(name,paused_at) VALUES('meta_inbox',clock_timestamp())`)
	var queueCount int64
	if err := f.owner.QueryRow(ctx, `SELECT count(*) FROM river.river_queue WHERE name='meta_inbox' AND paused_at IS NOT NULL`).Scan(&queueCount); err != nil || queueCount != 1 {
		t.Fatal("old Meta queue was not paused", err)
	}
	// An ordinary runtime-produced River job proves cutover does not rewrite
	// unrelated rows in the old schema. Business producer lifecycle is MIso04.
	var unrelatedID int64
	if err := miPool(t, f, "commerce_runtime").QueryRow(ctx, `INSERT INTO river.river_job(kind,queue,args,max_attempts) VALUES('generic_cutover_probe','default','{}',25) RETURNING id`).Scan(&unrelatedID); err != nil {
		t.Fatal("old ordinary producer", err)
	}
	var oldHighWater int64
	if err := f.owner.QueryRow(ctx, `SELECT setval('river.river_job_id_seq',greatest((SELECT last_value FROM river.river_job_id_seq),$1::bigint)+100,true)`, pruned.job).Scan(&oldHighWater); err != nil {
		t.Fatal(err)
	}
	unrelatedBefore := miIsoRows(t, f, "river.river_job", `WHERE id=`+fmt.Sprint(unrelatedID))
	oldJobs := miIsoRows(t, f, "river.river_job", `WHERE kind='meta_inbox_v1' OR queue='meta_inbox'`)
	if oldJobs == "[]" || miCount(t, f.owner, `SELECT count(*) FROM river.river_job WHERE kind='meta_inbox_v1' OR queue='meta_inbox'`) != 3 {
		t.Fatal("populated old lane lacks scheduled, retryable, terminal jobs")
	}
	oldQueue := miIsoRows(t, f, "river.river_queue", `WHERE name='meta_inbox'`)
	tables := []string{"integration.bindings", "meta_inbox.routes", "meta_inbox.batches", "meta_inbox.batch_events", "meta_inbox.events", "meta_private.raw_bodies", "meta_private.event_bodies", "meta_private.quarantine_bodies", "social.conversations", "social.messages", "social.comment_events", "meta_inbox.audit_events"}
	before := make(map[string]string, len(tables))
	for _, table := range tables {
		before[table] = miIsoRows(t, f, table, "")
	}
	const historicalLedger = `WHERE (left(version,11) <> 'post_river/' AND version < '0031') OR (left(version,11) = 'post_river/' AND version < 'post_river/0004')`
	oldChecksums := miIsoRows(t, f, "public.lc_schema_migrations", historicalLedger)
	if err := migrations.Apply(ctx, f.owner); err != nil {
		t.Fatal("populated 0030 isolation cutover", err)
	}
	firstLedger := miIsoRows(t, f, "public.lc_schema_migrations", "")
	if err := migrations.Apply(ctx, f.owner); err != nil {
		t.Fatal("repeat populated isolation cutover", err)
	}
	if got := miIsoRows(t, f, "public.lc_schema_migrations", historicalLedger); got != oldChecksums {
		t.Fatal("historical migration ledger/checksum changed")
	}
	if got := miIsoRows(t, f, "public.lc_schema_migrations", ""); got != firstLedger {
		t.Fatal("repeat Apply mutated full migration ledger")
	}
	for _, table := range tables {
		if got := miIsoRows(t, f, table, ""); got != before[table] {
			t.Fatalf("cutover changed %s rows", table)
		}
	}
	if got := miIsoRows(t, f, "river_meta.river_job", ""); got != oldJobs {
		t.Fatal("cutover changed copied Meta job persisted fields")
	}
	if got := miIsoRows(t, f, "river_meta.river_queue", `WHERE name='meta_inbox'`); got != oldQueue {
		t.Fatal("cutover changed paused Meta queue row")
	}
	if got := miIsoRows(t, f, "river.river_job", `WHERE id=`+fmt.Sprint(unrelatedID)); got != unrelatedBefore ||
		miCount(t, f.owner, `SELECT count(*) FROM river.river_job WHERE kind='meta_inbox_v1' OR queue='meta_inbox'`) != 0 {
		t.Fatal("cutover changed unrelated old job or left a writable old Meta lane")
	}
	var nextID int64
	if err := f.owner.QueryRow(ctx, `SELECT nextval('river_meta.river_job_id_seq')`).Scan(&nextID); err != nil || nextID <= oldHighWater || nextID <= pruned.job {
		t.Fatalf("new job sequence reused source/pruned high-water: next=%d old=%d pruned=%d err=%v", nextID, oldHighWater, pruned.job, err)
	}
	if miCount(t, f.owner, `SELECT count(*) FROM public.lc_schema_migrations WHERE
		(left(version,11) <> 'post_river/' AND version < '0033') OR
		(left(version,11) = 'post_river/' AND version < 'post_river/0006')`) != 37 ||
		miCount(t, f.owner, `SELECT count(*) FROM public.lc_schema_migrations WHERE version='0033_live_planning.sql'`) != 1 {
		t.Fatal("0031/post0004 and 0032/post0005 ledger missing or Apply replayed a version")
	}
	mrReady(t, miPool(t, f, "commerce_meta_ingress"), true)
	mrReady(t, miPool(t, f, "commerce_meta_worker"), true)
}
