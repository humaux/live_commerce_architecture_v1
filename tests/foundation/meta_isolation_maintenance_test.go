package foundation_test

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

type miIsoMaintenanceIDs struct {
	scheduled int64
	retryable int64
	stale     int64
	terminal  int64
}

func miIsoJobState(t *testing.T, f *testFixture, table string, id int64) string {
	t.Helper()
	var state string
	if err := f.owner.QueryRow(context.Background(), `SELECT state::text FROM `+table+` WHERE id=$1`, id).Scan(&state); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "missing"
		}
		t.Fatal(err)
	}
	return state
}

func miIsoMaintained(t *testing.T, f *testFixture, table string, ids miIsoMaintenanceIDs) bool {
	t.Helper()
	return miIsoJobState(t, f, table, ids.scheduled) == "available" &&
		miIsoJobState(t, f, table, ids.retryable) == "available" &&
		miIsoJobState(t, f, table, ids.stale) != "running" &&
		miIsoJobState(t, f, table, ids.terminal) == "missing"
}

func miIsoWaitMaintenance(t *testing.T, f *testFixture, table string, ids miIsoMaintenanceIDs) {
	t.Helper()
	deadline := time.Now().Add(55 * time.Second) // upstream cleaner/rescuer default 30s plus startup jitter
	for time.Now().Before(deadline) {
		if miIsoMaintained(t, f, table, ids) {
			return
		}
		time.Sleep(100 * time.Millisecond) // polling for observed transitions, never a causal sleep
	}
	t.Fatalf("missing scheduler/rescuer/cleaner positive controls in %s: scheduled=%s retryable=%s stale=%s terminal=%s",
		table, miIsoJobState(t, f, table, ids.scheduled), miIsoJobState(t, f, table, ids.retryable),
		miIsoJobState(t, f, table, ids.stale), miIsoJobState(t, f, table, ids.terminal))
}

func TestMetaRuntimeIsolationTwoWayRealMaintenance(t *testing.T) {
	m := miSetup(t)
	f := m.f
	ctx := context.Background()
	asset := miAsset()
	binding := miBinding(t, m, asset, "facebook", f.tenantA, f.storeA1, f.principalA)
	miRoute(t, m, asset, f.tenantA, f.storeA1, binding)
	meta := miIsoMaintenanceIDs{}
	for i, target := range []*int64{&meta.scheduled, &meta.retryable, &meta.stale, &meta.terminal} {
		e := mcPost(t, m, asset, miMessage(asset, "m."+randomUUID(), fmt.Sprintf("maintenance-%d", i)))
		*target = e.job
	}
	for _, change := range []struct {
		id  int64
		sql string
	}{
		{meta.scheduled, `UPDATE river_meta.river_job SET state='scheduled',scheduled_at=clock_timestamp()-interval '1 second' WHERE id=$1`},
		{meta.retryable, `UPDATE river_meta.river_job SET state='retryable',attempt=1,attempted_at=clock_timestamp()-interval '2 hours',scheduled_at=clock_timestamp()-interval '1 second' WHERE id=$1`},
		{meta.stale, `UPDATE river_meta.river_job SET state='running',attempt=1,attempted_at=clock_timestamp()-interval '2 hours' WHERE id=$1`},
		{meta.terminal, `UPDATE river_meta.river_job SET state='completed',finalized_at=clock_timestamp()-interval '3 days' WHERE id=$1`},
	} {
		mustExec(t, f.owner, change.sql, change.id)
	}
	// Pause fetch, not maintenance. The four valid linked jobs remain eligible
	// for the three native River maintenance services in river_meta.
	mustExec(t, f.owner, `INSERT INTO river_meta.river_queue(name,paused_at) VALUES('meta_inbox',clock_timestamp()) ON CONFLICT(name) DO UPDATE SET paused_at=excluded.paused_at`)
	old := miIsoMaintenanceIDs{}
	for i, target := range []*int64{&old.scheduled, &old.retryable, &old.stale, &old.terminal} {
		state := []string{"scheduled", "retryable", "running", "completed"}[i]
		var err error
		err = f.owner.QueryRow(ctx, `INSERT INTO river.river_job(kind,queue,args,max_attempts,state,attempt,attempted_at,scheduled_at,finalized_at)
		 VALUES('mi_iso_old_control','default','{}',25,$1::river.river_job_state,
		 CASE WHEN $1 IN ('retryable','running') THEN 1 ELSE 0 END,
		 CASE WHEN $1 IN ('retryable','running') THEN clock_timestamp()-interval '2 hours' ELSE NULL END,
		 CASE WHEN $1 IN ('scheduled','retryable') THEN clock_timestamp()-interval '1 second' ELSE clock_timestamp() END,
		 CASE WHEN $1='completed' THEN clock_timestamp()-interval '3 days' ELSE NULL END) RETURNING id`, state).Scan(target)
		if err != nil {
			t.Fatal("seed unrelated old River control", err)
		}
	}
	oldBefore := miIsoRows(t, f, "river.river_job", `WHERE kind='mi_iso_old_control'`)
	if oldBefore == "[]" {
		t.Fatal("old maintenance controls absent")
	}
	workerBinary := mrBuild(t, "../../cmd/meta-worker", "meta-worker-maintenance")
	keyJSON := fmt.Sprintf(`{"keys":[{"id":%q,"key_base64":%q}]}`, miKeyID, base64.StdEncoding.EncodeToString(m.key))
	metaWorker := mrLaunch(t, workerBinary, "meta-maintenance", []string{
		"COMMERCE_META_WORKER_ENABLED=1",
		"COMMERCE_META_WORKER_DATABASE_URL=" + miRole(t, f, "commerce_meta_worker"),
		"COMMERCE_META_CONSUMER_DATABASE_URL=" + miRole(t, f, "commerce_meta_consumer"),
		"COMMERCE_META_WORKER_CONCURRENCY=1", "COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID=" + miKeyID,
		"COMMERCE_META_PAYLOAD_KEYS_JSON=" + keyJSON,
	})
	mrReadyLog(t, metaWorker, "meta_worker_ready")
	miIsoWaitMaintenance(t, f, "river_meta.river_job", meta)
	if got := miIsoRows(t, f, "river.river_job", `WHERE kind='mi_iso_old_control'`); got != oldBefore {
		t.Fatal("Meta worker maintained unrelated scheduled/retryable/stale/terminal old jobs")
	}
	mrStop(t, metaWorker, syscall.SIGTERM, true)
	// Re-arm still-linked jobs while Meta is stopped. A snapshot of only
	// already-promoted jobs would make the converse maintenance check vacuous.
	mustExec(t, f.owner, `UPDATE river_meta.river_job SET state='scheduled',scheduled_at=clock_timestamp()-interval '1 second' WHERE id=$1`, meta.scheduled)
	mustExec(t, f.owner, `UPDATE river_meta.river_job SET state='retryable',attempt=2,attempted_at=clock_timestamp()-interval '2 hours',scheduled_at=clock_timestamp()-interval '1 second' WHERE id=$1`, meta.retryable)
	mustExec(t, f.owner, `UPDATE river_meta.river_job SET state='running',attempt=2,attempted_at=clock_timestamp()-interval '2 hours' WHERE id=$1`, meta.stale)
	terminal := mcPost(t, m, asset, miMessage(asset, "m."+randomUUID(), "maintenance-converse-terminal"))
	mustExec(t, f.owner, `UPDATE river_meta.river_job SET state='completed',finalized_at=clock_timestamp()-interval '3 days' WHERE id=$1`, terminal.job)
	meta.terminal = terminal.job
	if miIsoJobState(t, f, "river_meta.river_job", meta.scheduled) != "scheduled" ||
		miIsoJobState(t, f, "river_meta.river_job", meta.retryable) != "retryable" ||
		miIsoJobState(t, f, "river_meta.river_job", meta.stale) != "running" ||
		miIsoJobState(t, f, "river_meta.river_job", meta.terminal) != "completed" {
		t.Fatal("converse Meta controls were not maintenance-eligible")
	}
	metaBefore := miIsoRows(t, f, "river_meta.river_job", "")
	oldWorkerBinary := mrBuild(t, "../../cmd/expiry-worker", "expiry-worker-maintenance")
	ordinary := mrLaunch(t, oldWorkerBinary, "old-maintenance", []string{
		"COMMERCE_EXPIRY_WORKER_ENABLED=1",
		"COMMERCE_EXPIRY_WORKER_DATABASE_URL=" + miRole(t, f, "commerce_worker"),
		"COMMERCE_EXPIRY_WORKER_CONCURRENCY=1",
	})
	mrReadyLog(t, ordinary, "expiry_worker_ready")
	miIsoWaitMaintenance(t, f, "river.river_job", old)
	if got := miIsoRows(t, f, "river_meta.river_job", ""); got != metaBefore {
		t.Fatal("old worker maintained Meta jobs after Meta worker stopped")
	}
	mrStop(t, ordinary, syscall.SIGTERM, true)
}
