// Temporary diagnostic for the ordinary g06 dispatch. The PG and River paths
// are native. Only the named synthetic predecessor's consumer return is held.
package foundation_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func oaRiverContrast(t *testing.T, isolate bool) {
	foreignDB := pwIsolatedFixture(t)
	m := miSetupOn(t, foreignDB)
	asset := miAsset()
	binding := miBinding(t, m, asset, "facebook", foreignDB.tenantA, foreignDB.storeA1, foreignDB.principalA)
	miRoute(t, m, asset, foreignDB.tenantA, foreignDB.storeA1, binding)
	foreign := mcPost(t, m, asset, miMessage(asset, "m."+randomUUID(), "synthetic predecessor"))
	targetDB := foreignDB
	if isolate {
		targetDB = pwIsolatedFixture(t)
	}
	probe := newOARiverProbe(foreign, !isolate)
	oaAreaGoRecipientOn(t, true, lbuNew(t, tcvOpts{base: targetDB}), probe)
	if !isolate {
		t.Fatal("OA1 probe expected the unchanged 10s shared-queue oracle to fail")
	}
	var untouched bool
	err := foreignDB.owner.QueryRow(context.Background(), `SELECT j.state='available' AND j.attempt=0
		FROM river_meta.river_job j JOIN meta_inbox.events e ON e.job_id=j.id AND j.args->>'event_id'=e.id::text
		WHERE j.id=$1 AND e.id=$2`, foreign.job, foreign.id).Scan(&untouched)
	if err != nil || !untouched {
		t.Fatal("OA1 isolated candidate changed its predecessor's exact task")
	}
	t.Log("OA1 control predecessor stayed available at attempt=0")
}

type oaRiverProbe struct {
	started       time.Time
	foreign       mcEvent
	sharedBarrier bool
	entered       chan struct{}
	release       chan struct{}
	once          sync.Once
	mu            sync.Mutex
	phases        []string
}

func newOARiverProbe(foreign mcEvent, sharedBarrier bool) *oaRiverProbe {
	return &oaRiverProbe{started: time.Now(), foreign: foreign, sharedBarrier: sharedBarrier,
		entered: make(chan struct{}), release: make(chan struct{})}
}

func (p *oaRiverProbe) pool(t *testing.T, f *testFixture, role string) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(miRole(t, f, role))
	if err != nil {
		t.Fatal("OA1 probe pool config")
	}
	cfg.ConnConfig.Tracer = p
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal("OA1 probe pool")
	}
	t.Cleanup(pool.Close)
	return pool
}

type oaRiverTraceKey struct{}
type oaRiverTraceQuery struct {
	phase   string
	started time.Time
	foreign bool
}

func (p *oaRiverProbe) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	sql := strings.ToLower(data.SQL)
	query := oaRiverTraceQuery{started: time.Now()}
	switch {
	case strings.Contains(sql, "locked_jobs") && strings.Contains(sql, "skip locked"):
		query.phase = "fetch"
	case strings.Contains(sql, "load_social_event"):
		query.phase = "load"
		if len(data.Args) >= 2 {
			event, _ := data.Args[0].(string)
			job, _ := data.Args[1].(int64)
			query.foreign = event == p.foreign.id && job == p.foreign.job
		}
	case strings.Contains(sql, "finish_social_event"):
		query.phase = "finish"
	case strings.Contains(sql, "river_job") && strings.Contains(sql, "finalized_at") && strings.Contains(sql, "update"):
		query.phase = "river-write"
	case sql == "commit":
		query.phase = "commit"
	default:
		return ctx
	}
	p.add(fmt.Sprintf("%s start predecessor=%t", query.phase, query.foreign))
	return context.WithValue(ctx, oaRiverTraceKey{}, query)
}

func (p *oaRiverProbe) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	query, ok := ctx.Value(oaRiverTraceKey{}).(oaRiverTraceQuery)
	if !ok {
		return
	}
	outcome := "ok"
	if data.Err != nil {
		outcome = oaNotificationDiagnosticError(data.Err)
	}
	p.add(fmt.Sprintf("%s end predecessor=%t elapsed_ms=%d rows=%d %s",
		query.phase, query.foreign, time.Since(query.started).Milliseconds(), data.CommandTag.RowsAffected(), outcome))
	if query.foreign && p.sharedBarrier && data.Err == nil {
		p.once.Do(func() {
			p.add("predecessor load return held")
			close(p.entered)
			// Release comes from the unchanged 10s oracle. The longer bound
			// only protects diagnostic cleanup if that path fails earlier.
			timer := time.NewTimer(16 * time.Second)
			defer timer.Stop()
			select {
			case <-p.release:
			case <-timer.C:
				p.add("predecessor safety deadline expired")
			}
		})
	}
}

func (p *oaRiverProbe) add(event string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.phases) < 160 {
		p.phases = append(p.phases, fmt.Sprintf("%dms %s", time.Since(p.started).Milliseconds(), event))
	}
}

func (p *oaRiverProbe) report(t *testing.T) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, phase := range p.phases {
		t.Logf("OA1 phase %s", phase)
	}
}

func (p *oaRiverProbe) observe(t *testing.T, e *lbuEnv, target mcEvent, client, phase string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var snapshot string
	err := e.p.f.owner.QueryRow(ctx, `WITH relevant AS (
		SELECT CASE WHEN j.id=$1 AND ev.id=$2 THEN 'target' WHEN j.id=$3 AND ev.id::text=$4 THEN 'predecessor' ELSE 'other' END AS task,
		j.state::text, j.attempt,
		CASE WHEN j.kind='meta_inbox_v1' THEN 'meta_inbox_v1' ELSE 'other' END AS kind,
		coalesce($5=ANY(j.attempted_by),false) AS claimed_by_client,
		coalesce(ev.tenant_id=$6 AND ev.store_id=$7,false) AS same_fixture,
		coalesce(ev.terminal_reason='processed',false) AS processed,
		round(extract(epoch FROM (clock_timestamp()-j.attempted_at))*1000) AS claimed_ms_ago
		FROM river_meta.river_job j
		LEFT JOIN meta_inbox.events ev ON ev.job_id=j.id AND j.args->>'event_id'=ev.id::text
		WHERE j.queue='meta_inbox'
		AND ((j.id=$1 AND ev.id=$2) OR (j.id=$3 AND ev.id::text=$4) OR (j.state='running' AND $5=ANY(j.attempted_by)))
	)
	SELECT jsonb_build_object('at_ms',$8::bigint,'available_due',(
		SELECT count(*) FROM river_meta.river_job WHERE queue='meta_inbox' AND state='available' AND scheduled_at<=now()),
		'rows',coalesce((SELECT jsonb_agg(to_jsonb(r) ORDER BY r.task) FROM relevant r),'[]'::jsonb))::text`,
		target.job, target.id, p.foreign.job, p.foreign.id, client, e.tenant(), e.store(), time.Since(p.started).Milliseconds()).Scan(&snapshot)
	if err != nil {
		t.Logf("OA1 snapshot %s error=%s", phase, oaNotificationDiagnosticError(err))
		return
	}
	t.Logf("OA1 snapshot %s %s", phase, snapshot)
}
