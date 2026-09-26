package foundation_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/platform"
)

// Independent of the service author: exercise SQL privileges, not a Go mock.
// Queue identity survives ordinary worker lifecycle updates but cannot migrate
// into/out of this producer family, even with the existing broad worker grants.
func TestMetaInboxReservedQueueIntegrity(t *testing.T) {
	m := miSetup(t)
	ctx := context.Background()
	asset := miAsset()
	binding := miBinding(t, m, asset, "facebook", m.f.tenantA, m.f.storeA1, m.f.principalA)
	miRoute(t, m, asset, m.f.tenantA, m.f.storeA1, binding)
	status, body := miPost(t, m, miMessage(asset, "queue-integrity", "synthetic queue test"))
	miStatus(t, status, body, 200)
	var job int64
	if err := m.f.owner.QueryRow(ctx, `SELECT job_id FROM meta_inbox.events WHERE app_id=$1 AND asset_id=$2 AND disposition='ROUTED'`, miApp, asset).Scan(&job); err != nil {
		t.Fatal(err)
	}
	before := miCount(t, m.f.owner, `SELECT count(*) FROM river.river_job`)
	for _, tc := range []struct{ name, kind, queue, args string }{
		{"orphan exact shape", "meta_inbox_v1", "meta_inbox", fmt.Sprintf(`{"event_id":%q,"version":1}`, randomUUID())},
		{"foreign kind", "other_v1", "meta_inbox", `{}`},
		{"foreign queue", "meta_inbox_v1", "default", `{}`},
		{"both foreign", "other_v1", "default", `{}`},
		{"decimal version", "meta_inbox_v1", "meta_inbox", fmt.Sprintf(`{"event_id":%q,"version":1.0}`, randomUUID())},
		{"extra payload", "meta_inbox_v1", "meta_inbox", fmt.Sprintf(`{"event_id":%q,"version":1,"text":"must not persist"}`, randomUUID())},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := m.ingress.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			_, err = tx.Exec(ctx, `INSERT INTO river.river_job(kind,queue,args,max_attempts) VALUES($1,$2,$3::jsonb,25)`, tc.kind, tc.queue, tc.args)
			if err == nil {
				err = tx.Commit(ctx)
			}
			if miSQLState(err) != "22023" {
				t.Fatalf("invalid job state=%s", miSQLState(err))
			}
		})
	}
	if got := miCount(t, m.f.owner, `SELECT count(*) FROM river.river_job`); got != before {
		t.Fatal("invalid ingress jobs persisted")
	}
	worker := miPool(t, m.f, "commerce_worker")
	for _, change := range []string{
		`kind='generic_v1'`, `queue='default'`, `args=args || '{"payload":"forbidden"}'::jsonb`,
		`args=jsonb_set(args,'{version}','2')`, `id=id+90000000`, `unique_key=decode('abcd','hex')`,
	} {
		t.Run(change, func(t *testing.T) {
			_, err := worker.Exec(ctx, `UPDATE river.river_job SET `+change+` WHERE id=$1`, job)
			if miSQLState(err) != "22023" {
				t.Fatalf("identity rewrite state=%s", miSQLState(err))
			}
		})
	}
	// Direct SQL needs no public API or actual Meta credentials. A generic worker
	// cannot create a reserved job; a runtime UPDATE(kind) cannot convert one.
	_, err := worker.Exec(ctx, `INSERT INTO river.river_job(kind,queue,args,max_attempts) VALUES('meta_inbox_v1','meta_inbox',$1::jsonb,25)`, fmt.Sprintf(`{"event_id":%q,"version":1}`, randomUUID()))
	if miSQLState(err) != "42501" {
		t.Fatalf("worker impersonated producer state=%s", miSQLState(err))
	}
	var generic int64
	if err := m.f.runtime.QueryRow(ctx, `INSERT INTO river.river_job(kind,queue,args,max_attempts) VALUES('generic_probe_v1','default','{}',25) RETURNING id`).Scan(&generic); err != nil {
		t.Fatal(err)
	}
	_, err = m.f.runtime.Exec(ctx, `UPDATE river.river_job SET kind='meta_inbox_v1' WHERE id=$1`, generic)
	if miSQLState(err) != "22023" {
		t.Fatalf("generic runtime moved into reserved family state=%s", miSQLState(err))
	}
	_, err = worker.Exec(ctx, `UPDATE river.river_job SET queue='meta_inbox' WHERE id=$1`, generic)
	if miSQLState(err) != "22023" {
		t.Fatalf("generic worker moved into reserved queue state=%s", miSQLState(err))
	}
	if _, err := worker.Exec(ctx, `UPDATE river.river_job SET state='completed',finalized_at=clock_timestamp() WHERE id=$1`, job); err != nil {
		t.Fatal("ordinary worker completion denied", err)
	}
}

func TestMetaInboxPoolAuthorityIsExclusiveBothWays(t *testing.T) {
	m := miSetup(t)
	ctx := context.Background()
	role := pgx.Identifier{m.ingress.Config().ConnConfig.User}.Sanitize()
	for _, other := range []string{"commerce_runtime", "commerce_worker", "commerce_identity", "commerce_buyer_runtime", "commerce_buyer_issuer", "commerce_checkout_runtime", "commerce_hosted_runtime", "commerce_meta_registrar", "commerce_meta_curator", "commerce_meta_writer"} {
		t.Run(other, func(t *testing.T) {
			mustExec(t, m.f.owner, `GRANT `+pgx.Identifier{other}.Sanitize()+` TO `+role+` WITH INHERIT TRUE, SET FALSE`)
			defer mustExec(t, m.f.owner, `REVOKE `+pgx.Identifier{other}.Sanitize()+` FROM `+role)
			if err := platform.ValidateMetaIngressPool(ctx, m.ingress); err == nil {
				t.Fatal("mixed authority admitted")
			}
			if pool, err := platform.OpenPool(ctx, m.ingress.Config().ConnString()); err == nil {
				pool.Close()
				t.Fatal("mixed ingress admitted as merchant")
			}
			_, err := m.ingress.Exec(ctx, `SELECT * FROM meta_inbox.begin_batch('12','page',repeat('a',64),1)`)
			if miSQLState(err) != "42501" {
				t.Fatalf("mixed login SQL bypass state=%s", miSQLState(err))
			}
		})
	}
	mustExec(t, m.f.owner, `GRANT commerce_meta_ingress TO `+role+` WITH INHERIT TRUE, SET TRUE`)
	defer mustExec(t, m.f.owner, `GRANT commerce_meta_ingress TO `+role+` WITH INHERIT TRUE, SET FALSE`)
	if err := platform.ValidateMetaIngressPool(ctx, m.ingress); err == nil {
		t.Fatal("SET-capable ingress admitted")
	}
	_, err := m.ingress.Exec(ctx, `SELECT * FROM meta_inbox.begin_batch('12','page',repeat('a',64),1)`)
	if miSQLState(err) != "42501" {
		t.Fatalf("SET-capable SQL bypass state=%s", miSQLState(err))
	}
}
