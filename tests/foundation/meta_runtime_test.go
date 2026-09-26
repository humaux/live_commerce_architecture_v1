package foundation_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/integrations/meta"
	"livecommerce/internal/platform"
	"livecommerce/migrations"
)

// These tests use the existing labelled, loopback-only worker PG fixture.
// Every catalog mutation is confined to that test-owned cluster.
func mrFixture(t *testing.T) *testFixture {
	t.Helper()
	return pwIsolatedFixture(t)
}

func mrSetup(t *testing.T, f *testFixture) miTest {
	t.Helper()
	m := miTest{f: f, ingress: miPool(t, f, "commerce_meta_ingress"), registrar: miPool(t, f, "commerce_meta_registrar"), curator: miPool(t, f, "commerce_meta_curator"), key: randomBytes(32)}
	keys := mcKeys(t, m)
	var err error
	m.verifier, err = meta.NewVerifier(meta.Config{AppID: miApp, Object: "page", AppSecret: miSecret, VerifyToken: "meta-inbox-verify-token"})
	if err != nil {
		t.Fatal(err)
	}
	inbox, err := meta.NewInbox(context.Background(), m.ingress, keys)
	if err != nil {
		t.Fatal(err)
	}
	m.handler, err = meta.NewInboxHandler(m.verifier, inbox)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func mrReady(t *testing.T, pool *pgxpool.Pool, want bool) {
	t.Helper()
	var ready bool
	if err := pool.QueryRow(context.Background(), `SELECT meta_inbox.runtime_ready()`).Scan(&ready); err != nil || ready != want {
		t.Fatalf("runtime readiness=%v want=%v err=%v", ready, want, err)
	}
}

func TestMetaRuntimePoolRolesAndDatabaseIdentity(t *testing.T) {
	f := mrFixture(t)
	ctx := context.Background()
	ingressDSN := miRole(t, f, "commerce_meta_ingress")
	consumerDSN := miRole(t, f, "commerce_meta_consumer")
	workerDSN := miRole(t, f, "commerce_worker")
	ingress, err := platform.OpenMetaIngressPool(ctx, ingressDSN)
	if err != nil {
		t.Fatal("dedicated ingress rejected", err)
	}
	defer ingress.Close()
	consumer, err := platform.OpenMetaConsumerPool(ctx, consumerDSN)
	if err != nil {
		t.Fatal("dedicated consumer rejected", err)
	}
	defer consumer.Close()
	worker, err := platform.OpenWorkerPool(ctx, workerDSN)
	if err != nil {
		t.Fatal("ordinary worker rejected", err)
	}
	defer worker.Close()
	if err := platform.ValidateSameDatabase(ctx, f.runtime, ingress); err != nil {
		t.Fatal("same database API/ingress denied", err)
	}
	if err := platform.ValidateSameDatabase(ctx, worker, consumer); err != nil {
		t.Fatal("same database worker/consumer denied", err)
	}
	for name, dsn := range map[string]string{"owner": f.databaseURL, "runtime": f.runtime.Config().ConnString(), "consumer": consumerDSN, "worker": workerDSN} {
		t.Run("ingress rejects "+name, func(t *testing.T) {
			p, err := platform.OpenMetaIngressPool(ctx, dsn)
			if err == nil {
				p.Close()
				t.Fatal("wrong authority accepted as ingress")
			}
		})
	}
	for name, dsn := range map[string]string{"owner": f.databaseURL, "runtime": f.runtime.Config().ConnString(), "ingress": ingressDSN, "worker": workerDSN} {
		t.Run("consumer rejects "+name, func(t *testing.T) {
			p, err := platform.OpenMetaConsumerPool(ctx, dsn)
			if err == nil {
				p.Close()
				t.Fatal("wrong authority accepted as consumer")
			}
		})
	}
	if p, err := platform.OpenWorkerPool(ctx, consumerDSN); err == nil {
		p.Close()
		t.Fatal("consumer login accepted as ordinary River worker")
	}
	mixedDSN := miRole(t, f, "commerce_meta_ingress")
	mixedURL, err := url.Parse(mixedDSN)
	if err != nil {
		t.Fatal(err)
	}
	mixedRole := pgx.Identifier{mixedURL.User.Username()}.Sanitize()
	mustExec(t, f.owner, `GRANT commerce_meta_consumer TO `+mixedRole+` WITH INHERIT TRUE, SET FALSE`)
	t.Cleanup(func() { mustExec(t, f.owner, `REVOKE commerce_meta_consumer FROM `+mixedRole) })
	if p, err := platform.OpenMetaIngressPool(ctx, mixedDSN); err == nil {
		p.Close()
		t.Fatal("mixed ingress/consumer authority accepted as ingress")
	}
	if p, err := platform.OpenMetaConsumerPool(ctx, mixedDSN); err == nil {
		p.Close()
		t.Fatal("mixed ingress/consumer authority accepted as consumer")
	}
	var schemaUse, readinessExec, eventRead, loadExec bool
	if err := f.owner.QueryRow(ctx, `SELECT
	 has_schema_privilege('commerce_worker','meta_inbox','USAGE'),
	 has_function_privilege('commerce_worker','meta_inbox.runtime_ready()','EXECUTE'),
	 has_table_privilege('commerce_worker','meta_inbox.events','SELECT'),
	 has_function_privilege('commerce_worker','meta_inbox.load_social_event(uuid,bigint,integer)','EXECUTE')`).Scan(&schemaUse, &readinessExec, &eventRead, &loadExec); err != nil {
		t.Fatal(err)
	}
	if !schemaUse || !readinessExec || eventRead || loadExec {
		t.Fatalf("0030 widened authority: schema=%v readiness=%v event_read=%v load=%v", schemaUse, readinessExec, eventRead, loadExec)
	}
	mrReady(t, ingress, true)
	mrReady(t, worker, true)
	if _, err := worker.Exec(ctx, `SELECT count(*) FROM meta_inbox.events`); miSQLState(err) != "42501" {
		t.Fatalf("ordinary worker read inbox events SQLSTATE=%s", miSQLState(err))
	}
	// A new second cluster has the same database name and schema. Copy a
	// benign fixture ID too: neither name nor IDs prove physical DB identity.
	clone := mrFixture(t)
	mustExec(t, clone.owner, `INSERT INTO control.tenants(id,name) VALUES($1,'same-id-marker')`, f.tenantA)
	otherIngress := miPool(t, clone, "commerce_meta_ingress")
	otherConsumer := miPool(t, clone, "commerce_meta_consumer")
	for _, p := range []*pgxpool.Pool{otherIngress, otherConsumer} {
		var name string
		if err := p.QueryRow(ctx, `SELECT current_database()`).Scan(&name); err != nil || name != "lc_foundation_test" {
			t.Fatalf("clone fixture name=%s err=%v", name, err)
		}
	}
	if err := platform.ValidateSameDatabase(ctx, f.runtime, otherIngress); err == nil {
		t.Fatal("API/ingress accepted same-named distinct PG cluster")
	}
	if err := platform.ValidateSameDatabase(ctx, worker, otherConsumer); err == nil {
		t.Fatal("worker/consumer accepted same-named distinct PG cluster")
	}
	// A second database on the same server is likewise not a shared queue.
	mustExec(t, f.owner, `CREATE DATABASE lc_meta_runtime_other`)
	t.Cleanup(func() { mustExec(t, f.owner, `DROP DATABASE lc_meta_runtime_other`) })
	otherCfg := f.owner.Config().Copy()
	otherCfg.ConnConfig.Database = "lc_meta_runtime_other"
	otherDB, err := pgxpool.NewWithConfig(ctx, otherCfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(otherDB.Close)
	if err := platform.ValidateSameDatabase(ctx, f.owner, otherDB); err == nil {
		t.Fatal("different database within one cluster accepted")
	}
}

func TestMetaRuntimeSameDatabaseProbeReleasesBorrowedConnectionsAndLocks(t *testing.T) {
	f := mrFixture(t)
	ctx := context.Background()
	firstName := "mr_probe_first_" + t04Tag()
	secondName := "mr_probe_second_" + t04Tag()
	firstCfg, err := pgxpool.ParseConfig(mrNamedDSN(t, miRole(t, f, "commerce_meta_ingress"), firstName))
	if err != nil {
		t.Fatal(err)
	}
	secondCfg, err := pgxpool.ParseConfig(mrNamedDSN(t, miRole(t, f, "commerce_meta_consumer"), secondName))
	if err != nil {
		t.Fatal(err)
	}
	firstCfg.MaxConns, secondCfg.MaxConns = 1, 1
	first, err := pgxpool.NewWithConfig(ctx, firstCfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(first.Close)
	second, err := pgxpool.NewWithConfig(ctx, secondCfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(second.Close)
	checkReleased := func() {
		t.Helper()
		if first.Stat().AcquiredConns() != 0 || second.Stat().AcquiredConns() != 0 ||
			miCount(t, f.owner, `SELECT count(*) FROM pg_locks l JOIN pg_stat_activity a ON a.pid=l.pid WHERE l.locktype='advisory' AND a.application_name=ANY($1::text[])`, []string{firstName, secondName}) != 0 {
			t.Fatal("same-database probe retained a borrowed connection or advisory lock")
		}
	}
	if err := platform.ValidateSameDatabase(ctx, first, second); err != nil {
		t.Fatal(err)
	}
	checkReleased()
	occupied, err := second.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	result := make(chan error, 1)
	go func() { result <- platform.ValidateSameDatabase(cancelCtx, first, second) }()
	deadline := time.Now().Add(3 * time.Second)
	for miCount(t, f.owner, `SELECT count(*) FROM pg_locks l JOIN pg_stat_activity a ON a.pid=l.pid WHERE l.locktype='advisory' AND a.application_name=$1`, firstName) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond) // observability polling, not causal lock proof
	}
	if time.Now().After(deadline) {
		cancel()
		occupied.Release()
		t.Fatal("first probe transaction never held an advisory lock")
	}
	cancel()
	occupied.Release()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("cancelled split probe succeeded")
		}
	case <-time.After(4 * time.Second):
		t.Fatal("cancelled split probe did not release connections")
	}
	checkReleased()
	if err := platform.ValidateSameDatabase(ctx, first, second); err != nil {
		t.Fatal("pools unusable after cancelled probe", err)
	}
	checkReleased()
	// A shorter caller deadline must also unwind a probe that already holds A's
	// lock while B's single borrowed slot is occupied; do not cancel it early.
	func() {
		occupied, err := second.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer occupied.Release()
		deadlineCtx, stop := context.WithTimeout(ctx, time.Second)
		defer stop()
		result := make(chan error, 1)
		go func() { result <- platform.ValidateSameDatabase(deadlineCtx, first, second) }()
		observed := false
		for !observed {
			observed = miCount(t, f.owner, `SELECT count(*) FROM pg_locks l JOIN pg_stat_activity a ON a.pid=l.pid WHERE l.locktype='advisory' AND a.application_name=$1`, firstName) > 0
			if !observed && deadlineCtx.Err() != nil {
				t.Fatal("deadline elapsed before first probe acquired its advisory lock")
			}
			if !observed {
				time.Sleep(10 * time.Millisecond) // observe actual lock; timeout itself remains causal
			}
		}
		select {
		case err := <-result:
			if err == nil || deadlineCtx.Err() != context.DeadlineExceeded {
				t.Fatalf("naturally expired probe result=%v context=%v", err, deadlineCtx.Err())
			}
		case <-time.After(3 * time.Second):
			t.Fatal("naturally expired probe exceeded cleanup budget")
		}
	}()
	checkReleased()
	if err := platform.ValidateSameDatabase(ctx, first, second); err != nil {
		t.Fatal("pools unusable after natural deadline", err)
	}
	checkReleased()
}

func TestMetaRuntimeGuardMetadataAndActiveQueueFailClosed(t *testing.T) {
	f := mrFixture(t)
	ctx := context.Background()
	ingress := miPool(t, f, "commerce_meta_ingress")
	worker := miPool(t, f, "commerce_worker")
	consumer := miPool(t, f, "commerce_meta_consumer")
	keys, err := meta.NewPayloadKeyring(miKeyID, map[string][]byte{miKeyID: randomBytes(32)})
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := meta.NewVerifier(meta.Config{AppID: miApp, Object: "page", AppSecret: miSecret, VerifyToken: "meta-inbox-verify-token"})
	if err != nil {
		t.Fatal(err)
	}
	endpoints := []meta.WebhookEndpoint{{Path: "/v1/meta/webhooks/" + miApp + "/page", Verifier: verifier}}
	denyStartup := func(t *testing.T) {
		t.Helper()
		if h, err := meta.NewWebhookRouter(ctx, ingress, keys, endpoints); err == nil || h != nil {
			t.Fatal("tampered readiness admitted API router")
		}
		if client, err := meta.NewConsumerClient(ctx, worker, consumer, keys, 1); err == nil || client != nil {
			t.Fatal("tampered readiness admitted River fetch client")
		}
		if miCount(t, f.owner, `SELECT coalesce(max(attempt),0) FROM river.river_job WHERE kind='meta_inbox_v1'`) != 0 ||
			miCount(t, f.owner, `SELECT count(*) FROM social.messages`) != 0 ||
			miCount(t, f.owner, `SELECT count(*) FROM social.comment_events`) != 0 {
			t.Fatal("rejected constructor claimed job or wrote social fact")
		}
	}
	mrReady(t, ingress, true)
	if h, err := meta.NewWebhookRouter(ctx, ingress, keys, endpoints); err != nil || h == nil {
		t.Fatal("valid API router rejected", err)
	}
	if client, err := meta.NewConsumerClient(ctx, worker, consumer, keys, 1); err != nil || client == nil {
		t.Fatal("valid River client rejected", err)
	}
	for _, guard := range []struct{ table, name string }{
		{"river.river_job", "meta_job_family"},
		{"river.river_job", "meta_job_commit"},
		{"social.messages", "social_message_commit"},
		{"social.comment_events", "social_comment_commit"},
	} {
		var definition string
		if err := f.owner.QueryRow(ctx, `SELECT pg_get_triggerdef(oid) FROM pg_trigger WHERE tgrelid=$1::regclass AND tgname=$2`, guard.table, guard.name).Scan(&definition); err != nil {
			t.Fatal(err)
		}
		t.Run("disabled "+guard.name, func(t *testing.T) {
			mustExec(t, f.owner, `ALTER TABLE `+guard.table+` DISABLE TRIGGER `+pgx.Identifier{guard.name}.Sanitize())
			defer mustExec(t, f.owner, `ALTER TABLE `+guard.table+` ENABLE TRIGGER `+pgx.Identifier{guard.name}.Sanitize())
			mrReady(t, ingress, false)
			mrReady(t, worker, false)
			denyStartup(t)
		})
		t.Run("missing "+guard.name, func(t *testing.T) {
			mustExec(t, f.owner, `DROP TRIGGER `+pgx.Identifier{guard.name}.Sanitize()+` ON `+guard.table)
			defer mustExec(t, f.owner, definition)
			mrReady(t, ingress, false)
			denyStartup(t)
		})
		mrReady(t, ingress, true)
	}
	// A present name with a different function or a WHEN filter is not the
	// required mandatory deferred social guard.
	var socialDef string
	if err := f.owner.QueryRow(ctx, `SELECT pg_get_triggerdef(oid) FROM pg_trigger WHERE tgrelid='social.messages'::regclass AND tgname='social_message_commit'`).Scan(&socialDef); err != nil {
		t.Fatal(err)
	}
	mustExec(t, f.owner, `CREATE FUNCTION meta_inbox.mr_test_noop() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$ BEGIN RETURN NEW; END $$`)
	mustExec(t, f.owner, `ALTER FUNCTION meta_inbox.mr_test_noop() OWNER TO commerce_meta_writer`)
	t.Cleanup(func() { mustExec(t, f.owner, `DROP FUNCTION meta_inbox.mr_test_noop()`) })
	for name, replacement := range map[string]string{
		"replaced function": strings.Replace(socialDef, "meta_inbox.guard_social_insert()", "meta_inbox.mr_test_noop()", 1),
		"filtered trigger":  strings.Replace(socialDef, "FOR EACH ROW", "FOR EACH ROW WHEN (false)", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if replacement == socialDef {
				t.Fatal("catalog definition did not contain expected function or row clause")
			}
			mustExec(t, f.owner, `DROP TRIGGER social_message_commit ON social.messages`)
			defer func() {
				mustExec(t, f.owner, `DROP TRIGGER social_message_commit ON social.messages`)
				mustExec(t, f.owner, socialDef)
			}()
			mustExec(t, f.owner, replacement)
			mrReady(t, ingress, false)
			denyStartup(t)
		})
		mrReady(t, ingress, true)
	}
	t.Run("security invoker function", func(t *testing.T) {
		mustExec(t, f.owner, `ALTER FUNCTION meta_inbox.guard_social_insert() SECURITY INVOKER`)
		defer mustExec(t, f.owner, `ALTER FUNCTION meta_inbox.guard_social_insert() SECURITY DEFINER`)
		mrReady(t, ingress, false)
		denyStartup(t)
	})
	mrReady(t, ingress, true)
	m := mrSetup(t, f)
	asset := miAsset()
	binding := miBinding(t, m, asset, "facebook", f.tenantA, f.storeA1, f.principalA)
	miRoute(t, m, asset, f.tenantA, f.storeA1, binding)
	e := mcPost(t, m, asset, miMessage(asset, "m."+randomUUID(), "valid-linked-job"))
	mrReady(t, ingress, true)
	mutateJob := func(statement string) {
		mustExec(t, f.owner, `ALTER TABLE river.river_job DISABLE TRIGGER meta_job_family`)
		mustExec(t, f.owner, statement, e.job)
		mustExec(t, f.owner, `ALTER TABLE river.river_job ENABLE TRIGGER meta_job_family`)
	}
	for _, poisoned := range []struct{ name, change, restore string }{
		{"wrong queue", `UPDATE river.river_job SET queue='default' WHERE id=$1`, `UPDATE river.river_job SET queue='meta_inbox' WHERE id=$1`},
		{"wrong kind", `UPDATE river.river_job SET kind='mr_other_v1' WHERE id=$1`, `UPDATE river.river_job SET kind='meta_inbox_v1' WHERE id=$1`},
		{"extra args", `UPDATE river.river_job SET args=jsonb_set(args,'{extra}','1'::jsonb) WHERE id=$1`, `UPDATE river.river_job SET args=args-'extra' WHERE id=$1`},
		{"unique key", `UPDATE river.river_job SET unique_key=decode(repeat('ab',32),'hex') WHERE id=$1`, `UPDATE river.river_job SET unique_key=NULL WHERE id=$1`},
	} {
		t.Run("active job "+poisoned.name, func(t *testing.T) {
			mutateJob(poisoned.change)
			defer mutateJob(poisoned.restore)
			mrReady(t, ingress, false)
			mrReady(t, worker, false)
			denyStartup(t)
		})
		mrReady(t, ingress, true)
	}
	t.Run("active job without link", func(t *testing.T) {
		mustExec(t, f.owner, `ALTER TABLE river.river_job DISABLE TRIGGER meta_job_family`)
		mustExec(t, f.owner, `ALTER TABLE river.river_job DISABLE TRIGGER meta_job_commit`)
		var id int64
		err := f.owner.QueryRow(ctx, `INSERT INTO river.river_job(kind,queue,args,max_attempts) VALUES('meta_inbox_v1','meta_inbox',jsonb_build_object('event_id',$1::text,'version',1),2) RETURNING id`, randomUUID()).Scan(&id)
		mustExec(t, f.owner, `ALTER TABLE river.river_job ENABLE TRIGGER meta_job_commit`)
		mustExec(t, f.owner, `ALTER TABLE river.river_job ENABLE TRIGGER meta_job_family`)
		if err != nil {
			t.Fatal("seed test-owned unlinked active job", err)
		}
		defer mustExec(t, f.owner, `DELETE FROM river.river_job WHERE id=$1`, id)
		mrReady(t, ingress, false)
		mrReady(t, worker, false)
		denyStartup(t)
	})
	mrReady(t, ingress, true)
}

func TestMetaRuntimePopulated0029Upgrade(t *testing.T) {
	f := mcPre0029Fixture(t)
	ctx := context.Background()
	body, err := os.ReadFile("../../migrations/0029_meta_social_consumer.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, string(body)); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal("install exact 0029 SQL", err)
	}
	checksum := fmt.Sprintf("%x", sha256.Sum256(body))
	if _, err := tx.Exec(ctx, `INSERT INTO public.lc_schema_migrations(version,checksum) VALUES('0029_meta_social_consumer.sql',$1)`, checksum); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var oldLedger int
	var readyExists bool
	if err := f.owner.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.lc_schema_migrations),to_regprocedure('meta_inbox.runtime_ready()') IS NOT NULL`).Scan(&oldLedger, &readyExists); err != nil || oldLedger != 32 || readyExists {
		t.Fatalf("fixture is not 0029: ledger=%d ready=%v err=%v", oldLedger, readyExists, err)
	}
	m := mrSetup(t, f)
	asset := miAsset()
	binding := miBinding(t, m, asset, "facebook", f.tenantA, f.storeA1, f.principalA)
	route, _ := miRoute(t, m, asset, f.tenantA, f.storeA1, binding)
	e := mcPost(t, m, asset, miMessage(asset, "m."+randomUUID(), "legacy-0029"))
	mcRunning(t, m, e, 1)
	mcFinish(t, mcConsumer(t, m), e, 1, mcSubject())
	const snapshot = `SELECT jsonb_build_object(
	 'binding',(SELECT to_jsonb(b) FROM integration.bindings b WHERE b.id=$3),
	 'route',(SELECT to_jsonb(r) FROM meta_inbox.routes r WHERE r.id=$2),
	 'event',(SELECT to_jsonb(e) FROM meta_inbox.events e WHERE e.id=$1),
	 'batch_event',(SELECT to_jsonb(x) FROM meta_inbox.batch_events x WHERE x.event_id=$1),
	 'batch',(SELECT to_jsonb(b) FROM meta_inbox.batches b JOIN meta_inbox.batch_events x ON x.batch_id=b.id WHERE x.event_id=$1),
	 'raw',(SELECT to_jsonb(rb) FROM meta_private.raw_bodies rb JOIN meta_inbox.batch_events x ON x.batch_id=rb.batch_id WHERE x.event_id=$1),
	 'body',(SELECT to_jsonb(eb) FROM meta_private.event_bodies eb WHERE eb.event_id=$1),
	 'job',(SELECT to_jsonb(j) FROM river.river_job j WHERE j.id=$4),
	 'conversation',(SELECT to_jsonb(c) FROM social.conversations c JOIN social.messages x ON x.conversation_id=c.id WHERE x.event_id=$1),
	 'message',(SELECT to_jsonb(x) FROM social.messages x WHERE x.event_id=$1),
	 'audit',(SELECT to_jsonb(a) FROM meta_inbox.audit_events a WHERE a.event_id=$1 AND a.action='processed'))::text`
	var before, after string
	if err := f.owner.QueryRow(ctx, snapshot, e.id, route, binding, e.job).Scan(&before); err != nil {
		t.Fatal(err)
	}
	for _, row := range []string{"binding", "route", "event", "raw", "body", "job", "conversation", "message", "audit"} {
		if strings.Contains(before, `"`+row+`": null`) {
			t.Fatalf("0029 fixture lacks %s", row)
		}
	}
	if err := migrations.Apply(ctx, f.owner); err != nil {
		t.Fatal("populated 0029 to 0030 upgrade", err)
	}
	if err := migrations.Apply(ctx, f.owner); err != nil {
		t.Fatal("repeat 0030 migration", err)
	}
	if err := f.owner.QueryRow(ctx, snapshot, e.id, route, binding, e.job).Scan(&after); err != nil || after != before {
		t.Fatalf("0030 changed old receipts, ciphertext, job or social fact: err=%v equal=%v", err, after == before)
	}
	if miCount(t, f.owner, `SELECT count(*) FROM public.lc_schema_migrations`) != int64(oldLedger+1) {
		t.Fatal("0030 ledger missing or repeat Apply duplicated version")
	}
	mrReady(t, m.ingress, true)
	mrReady(t, miPool(t, f, "commerce_worker"), true)
}
