package foundation_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"

	"livecommerce/internal/payments"
)

func legacyChangedColumns(before, after string) []string {
	var oldRows, newRows []map[string]any
	if json.Unmarshal([]byte(before), &oldRows) != nil || json.Unmarshal([]byte(after), &newRows) != nil {
		return []string{"invalid_snapshot"}
	}
	if len(newRows) == 0 {
		return []string{"row_deleted"}
	}
	if len(oldRows) != 1 || len(newRows) != 1 {
		return []string{"row_count"}
	}
	var changed []string
	for column, oldValue := range oldRows[0] {
		if !reflect.DeepEqual(oldValue, newRows[0][column]) {
			changed = append(changed, column)
		}
	}
	sort.Strings(changed)
	return changed
}

// The converse half uses the actual payment client, with native queue pause
// blocking fetch only. Its schema-wide maintenance must not touch the linked
// expiry, external or Meta rows even when all are eligible for maintenance.
func TestLegacyRuntimeIsolationPaymentDoesNotMaintainForeignFamilies(t *testing.T) {
	f := pwIsolatedFixture(t)
	ctx := context.Background()
	keys := pwKeys(t)
	queries := [4]pqFixture{}
	for i := range queries {
		queries[i] = pqSetupItemsOn(t, f, keys, false, 1)
		if i != 0 {
			ewCloseSeedPools(t, queries[i].psHarness)
		}
	}
	firstExpiry, externalID := pwDefaultDomainJobs(t, queries[0])
	expiry := miIsoMaintenanceIDs{scheduled: firstExpiry}
	for _, id := range []*int64{&expiry.retryable, &expiry.stale, &expiry.terminal} {
		p := ewSetup(t, f, 1)
		*id = p.hold.JobID
		ewCloseSeedPools(t, p)
	}
	meta := mrSetup(t, f)
	asset := miAsset()
	binding := miBinding(t, meta, asset, "facebook", f.tenantA, f.storeA1, f.principalA)
	miRoute(t, meta, asset, f.tenantA, f.storeA1, binding)
	metaEvent := mcPost(t, meta, asset, miMessage(asset, "m."+randomUUID(), "payment-leader-foreign"))
	for _, change := range []struct {
		id  int64
		sql string
	}{
		{queries[0].result.JobID, `UPDATE river_payment.river_job SET state='scheduled',scheduled_at=clock_timestamp()-interval '1 second' WHERE id=$1`},
		{queries[1].result.JobID, `UPDATE river_payment.river_job SET state='retryable',attempt=1,attempted_at=clock_timestamp()-interval '2 hours',scheduled_at=clock_timestamp()-interval '1 second' WHERE id=$1`},
		{queries[2].result.JobID, `UPDATE river_payment.river_job SET state='running',attempt=1,attempted_at=clock_timestamp()-interval '2 hours' WHERE id=$1`},
		{queries[3].result.JobID, `UPDATE river_payment.river_job SET state='completed',finalized_at=clock_timestamp()-interval '3 days' WHERE id=$1`},
		{expiry.scheduled, `UPDATE river_expiry.river_job SET state='scheduled',scheduled_at=clock_timestamp()-interval '1 second' WHERE id=$1`},
		{expiry.retryable, `UPDATE river_expiry.river_job SET state='retryable',attempt=1,attempted_at=clock_timestamp()-interval '2 hours',scheduled_at=clock_timestamp()-interval '1 second' WHERE id=$1`},
		{expiry.stale, `UPDATE river_expiry.river_job SET state='running',attempt=1,attempted_at=clock_timestamp()-interval '2 hours' WHERE id=$1`},
		{expiry.terminal, `UPDATE river_expiry.river_job SET state='completed',finalized_at=clock_timestamp()-interval '3 days' WHERE id=$1`},
		{externalID, `UPDATE river.river_job SET state='running',attempt=1,attempted_at=clock_timestamp()-interval '2 hours' WHERE id=$1`},
		{metaEvent.job, `UPDATE river_meta.river_job SET state='scheduled',scheduled_at=clock_timestamp()-interval '1 second' WHERE id=$1`},
	} {
		mustExec(t, f.owner, change.sql, change.id)
	}
	mustExec(t, f.owner, `INSERT INTO river_payment.river_queue(name,paused_at) VALUES('payment_mock_v1',clock_timestamp()) ON CONFLICT(name) DO UPDATE SET paused_at=excluded.paused_at`)
	foreignTables := []string{"river_expiry.river_job", "river.river_job", "river_meta.river_job"}
	before := make(map[string]string, len(foreignTables)+8)
	for _, table := range append(foreignTables, "checkout.orders", "checkout.payment_attempts", "inventory.reservations", "inventory.ledger", "integration.operations", "integration.operation_events", "meta_inbox.events", "social.messages") {
		before[table] = miIsoRows(t, f, table, "")
	}
	var calls atomic.Int32
	opts := payments.DefaultQueryWorkerOptions()
	opts.MockTransport = pqTransport(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, errors.New("paused_queue_fetched")
	})
	client, err := payments.NewWorkerClient(ctx, queries[0].worker, keys, "PROVIDER_MOCK", 1, opts)
	if err != nil || client == nil {
		t.Fatalf("payment native client preflight: %v", err)
	}
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	miIsoWaitMaintenance(t, f, "river_payment.river_job", miIsoMaintenanceIDs{
		scheduled: queries[0].result.JobID, retryable: queries[1].result.JobID,
		stale: queries[2].result.JobID, terminal: queries[3].result.JobID,
	})
	pwStopClient(t, client)
	if calls.Load() != 0 {
		t.Fatal("paused payment fetch invoked MOCK transport")
	}
	for table, old := range before {
		if got := miIsoRows(t, f, table, ""); got != old {
			t.Fatalf("payment leader maintained foreign %s: changed=%v", table, legacyChangedColumns(old, got))
		}
	}
}

// Expiry is the sole running CLI. All jobs are inserted through the ordinary
// checkout/payment/integration producers before owner-scoped lifecycle setup.
// The paused expiry queue prevents fetch, not River's schema-wide maintenance.
func TestLegacyRuntimeIsolationExpiryDoesNotMaintainForeignFamilies(t *testing.T) {
	f := ewFixture(t)
	ctx := context.Background()
	q := pqSetupItemsOn(t, f, nil, false, 1)
	pcRecord(t, q, pcFull(q)) // synthetic local report, no provider call
	var reconcileID int64
	if err := f.owner.QueryRow(ctx, `SELECT id FROM river_payment.river_job WHERE kind='payment_reconcile_v1' AND args->>'operation_id'=$1`, q.result.OperationID).Scan(&reconcileID); err != nil {
		t.Fatal("valid reconcile job missing", err)
	}
	staleExpiryID, externalID := pwDefaultDomainJobs(t, q)
	ewCloseSeedPools(t, q.psHarness)
	seed := ewSetup(t, f, 1)
	scheduledExpiryID := seed.hold.JobID
	ewCloseSeedPools(t, seed)
	seed = ewSetup(t, f, 1)
	retryableExpiryID := seed.hold.JobID
	ewCloseSeedPools(t, seed)
	seed = ewSetup(t, f, 1)
	terminalExpiryID := seed.hold.JobID
	ewCloseSeedPools(t, seed)
	for _, linked := range []struct {
		id        int64
		operation string
		kind      string
		table     string
	}{
		{q.result.JobID, q.result.OperationID, "payment_query_v1", "river_payment.river_job"},
		{reconcileID, q.result.OperationID, "payment_reconcile_v1", "river_payment.river_job"},
		{externalID, "", "external_operation_v1", "river.river_job"},
	} {
		var valid bool
		err := f.owner.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM `+linked.table+` j
		 JOIN integration.operations o ON o.id::text=j.args->>'operation_id'
		 WHERE j.id=$1 AND j.kind=$2 AND ($3='' OR o.id::text=$3)
		 AND (j.kind='payment_reconcile_v1' OR o.job_id=j.id))`, linked.id, linked.kind, linked.operation).Scan(&valid)
		if err != nil || !valid {
			t.Fatalf("foreign job lacks production operation link kind=%s err=%v", linked.kind, err)
		}
	}
	if miCount(t, f.owner, `SELECT count(*) FROM payments.provider_observations WHERE attempt_id=$1`, q.result.AttemptID) != 1 {
		t.Fatal("reconcile job lacks persisted provider observation")
	}
	for _, change := range []struct {
		id  int64
		sql string
	}{
		{scheduledExpiryID, `UPDATE river_expiry.river_job SET state='scheduled',scheduled_at=clock_timestamp()-interval '1 second' WHERE id=$1`},
		{retryableExpiryID, `UPDATE river_expiry.river_job SET state='retryable',attempt=1,attempted_at=clock_timestamp()-interval '2 hours',scheduled_at=clock_timestamp()-interval '1 second' WHERE id=$1`},
		{staleExpiryID, `UPDATE river_expiry.river_job SET state='running',attempt=1,attempted_at=clock_timestamp()-interval '2 hours' WHERE id=$1`},
		{terminalExpiryID, `UPDATE river_expiry.river_job SET state='completed',finalized_at=clock_timestamp()-interval '3 days' WHERE id=$1`},
		{q.result.JobID, `UPDATE river_payment.river_job SET state='running',attempt=1,attempted_at=clock_timestamp()-interval '2 hours' WHERE id=$1`},
		{externalID, `UPDATE river.river_job SET state='running',attempt=1,attempted_at=clock_timestamp()-interval '2 hours' WHERE id=$1`},
		{reconcileID, `UPDATE river_payment.river_job SET state='completed',finalized_at=clock_timestamp()-interval '3 days' WHERE id=$1`},
	} {
		mustExec(t, f.owner, change.sql, change.id)
	}
	mustExec(t, f.owner, `INSERT INTO river_expiry.river_queue(name,paused_at) VALUES('checkout_expiry_v1',clock_timestamp()) ON CONFLICT(name) DO UPDATE SET paused_at=excluded.paused_at`)
	foreign := []struct {
		name  string
		table string
		id    int64
		row   string
	}{
		{"payment_query", "river_payment.river_job", q.result.JobID, ""},
		{"payment_reconcile", "river_payment.river_job", reconcileID, ""},
		{"external_operation", "river.river_job", externalID, ""},
	}
	for i := range foreign {
		foreign[i].row = miIsoRows(t, f, foreign[i].table, "WHERE id="+fmt.Sprint(foreign[i].id))
		if foreign[i].row == "[]" {
			t.Fatal("foreign producer row absent before worker", foreign[i].name)
		}
	}
	tables := []string{"checkout.orders", "checkout.payment_attempts", "inventory.reservations", "inventory.ledger", "integration.operations", "integration.operation_events", "payments.provider_observations", "payments.facts"}
	business := make(map[string]string, len(tables))
	for _, table := range tables {
		business[table] = miIsoRows(t, f, table, "")
	}
	binary := mrBuild(t, "../../cmd/expiry-worker", "legacy-expiry-maintenance")
	worker := mrLaunch(t, binary, "legacy-expiry-maintenance", []string{
		"COMMERCE_EXPIRY_WORKER_ENABLED=1",
		"COMMERCE_EXPIRY_WORKER_DATABASE_URL=" + miRole(t, f, waExpiry),
		"COMMERCE_EXPIRY_WORKER_CONCURRENCY=1",
	})
	mrReadyLog(t, worker, "expiry_worker_ready")
	miIsoWaitMaintenance(t, f, "river_expiry.river_job", miIsoMaintenanceIDs{
		scheduled: scheduledExpiryID, retryable: retryableExpiryID,
		stale: staleExpiryID, terminal: terminalExpiryID,
	})
	mrStop(t, worker, syscall.SIGTERM, true)
	var differences []string
	for _, item := range foreign {
		after := miIsoRows(t, f, item.table, "WHERE id="+fmt.Sprint(item.id))
		if after != item.row {
			differences = append(differences, fmt.Sprintf("%s %s id=%d state=%s changed=%v", item.name, item.table, item.id,
				miIsoJobState(t, f, item.table, item.id), legacyChangedColumns(item.row, after)))
		}
	}
	var businessChanges []string
	for _, table := range tables {
		if miIsoRows(t, f, table, "") != business[table] {
			businessChanges = append(businessChanges, table)
		}
	}
	if len(differences) != 0 || len(businessChanges) != 0 {
		t.Fatalf("real expiry maintenance crossed linked foreign family: jobs=[%s] business_tables_changed=%v", strings.Join(differences, "; "), businessChanges)
	}
}
