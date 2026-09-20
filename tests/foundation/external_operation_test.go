package foundation_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"livecommerce/internal/command"
	integration "livecommerce/internal/integrations/core"
	"livecommerce/internal/platform"
)

type t06GoFixture struct {
	base              *testFixture
	service           *integration.Service
	worker            *pgxpool.Pool
	tenant            string
	store             string
	otherStore        string
	principal         string
	otherPrincipal    string
	limitedPrincipal  string
	token             string
	otherToken        string
	missingPermission string
}

func newT06GoFixture(t *testing.T) *t06GoFixture {
	t.Helper()
	base := fixture(t)
	ctx := context.Background()
	client, err := river.NewClient(riverpgxv5.New(base.runtime), &river.Config{Schema: "river"})
	if err != nil {
		t.Fatal(err)
	}
	service, err := integration.New(client)
	if err != nil {
		t.Fatal(err)
	}
	f := &t06GoFixture{
		base:              base,
		service:           service,
		tenant:            randomUUID(),
		store:             randomUUID(),
		otherStore:        randomUUID(),
		principal:         randomUUID(),
		otherPrincipal:    randomUUID(),
		limitedPrincipal:  randomUUID(),
		token:             randomToken(),
		otherToken:        randomToken(),
		missingPermission: randomToken(),
	}
	tx, err := base.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `INSERT INTO control.tenants(id,name) VALUES($1,'t06-go-tenant')`, f.tenant); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES
		($1,$2,'t06-go-store','USD'),($1,$3,'t06-go-other-store','USD')`, f.tenant, f.store, f.otherStore); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO identity.principals(id) VALUES($1),($2),($3)`, f.principal, f.otherPrincipal, f.limitedPrincipal); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO identity.memberships(tenant_id,principal_id) VALUES($1,$2),($1,$3),($1,$4)`, f.tenant, f.principal, f.otherPrincipal, f.limitedPrincipal); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES
		($1,$2,$3,'store:read'),($1,$2,$3,'integration:manage'),($1,$2,$3,'integration:execute'),($1,$2,$3,'integration:read'),
		($1,$4,$3,'store:read'),($1,$4,$3,'integration:manage'),($1,$4,$3,'integration:execute'),($1,$4,$3,'integration:read'),
		($1,$2,$5,'store:read'),($1,$2,$5,'integration:execute'),($1,$2,$5,'integration:read'),
		($1,$2,$6,'store:read')`,
		f.tenant, f.store, f.principal, f.otherStore, f.otherPrincipal, f.limitedPrincipal); err != nil {
		t.Fatal(err)
	}
	if err = insertSession(ctx, tx, f.token, f.principal, "merchant", t06GoFuture(), nil); err != nil {
		t.Fatal(err)
	}
	if err = insertSession(ctx, tx, f.otherToken, f.otherPrincipal, "merchant", t06GoFuture(), nil); err != nil {
		t.Fatal(err)
	}
	if err = insertSession(ctx, tx, f.missingPermission, f.limitedPrincipal, "merchant", t06GoFuture(), nil); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	workerName := "t06_go_worker_" + strings.ReplaceAll(randomUUID()[:8], "-", "")
	password := hex.EncodeToString(randomBytes(32))
	if _, err = base.owner.Exec(ctx, fmt.Sprintf(`CREATE ROLE %s LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION IN ROLE commerce_worker PASSWORD '%s'`,
		pgx.Identifier{workerName}.Sanitize(), password)); err != nil {
		t.Fatal(err)
	}
	workerURL, err := url.Parse(base.databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	workerURL.User = url.UserPassword(workerName, password)
	f.worker, err = platform.OpenWorkerPool(ctx, workerURL.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		f.worker.Close()
		cleanup := context.Background()
		_, _ = base.owner.Exec(cleanup, `DELETE FROM river.river_job WHERE kind='external_operation_v1'
			AND args->>'operation_id' IN (SELECT id::text FROM integration.operations WHERE tenant_id=$1)`, f.tenant)
		_, _ = base.owner.Exec(cleanup, `DELETE FROM ops.command_results WHERE tenant_id=$1`, f.tenant)
		_, _ = base.owner.Exec(cleanup, `DELETE FROM ops.audit_events WHERE tenant_id=$1`, f.tenant)
		_, _ = base.owner.Exec(cleanup, `DELETE FROM integration.operation_events WHERE tenant_id=$1`, f.tenant)
		_, _ = base.owner.Exec(cleanup, `DELETE FROM integration.operations WHERE tenant_id=$1`, f.tenant)
		_, _ = base.owner.Exec(cleanup, `DELETE FROM integration.bindings WHERE tenant_id=$1`, f.tenant)
		_, _ = base.owner.Exec(cleanup, `DELETE FROM identity.store_grants WHERE tenant_id=$1`, f.tenant)
		_, _ = base.owner.Exec(cleanup, `DELETE FROM identity.sessions WHERE principal_id IN ($1,$2,$3)`, f.principal, f.otherPrincipal, f.limitedPrincipal)
		_, _ = base.owner.Exec(cleanup, `DELETE FROM identity.memberships WHERE tenant_id=$1`, f.tenant)
		_, _ = base.owner.Exec(cleanup, `DELETE FROM control.stores WHERE tenant_id=$1`, f.tenant)
		_, _ = base.owner.Exec(cleanup, `DELETE FROM control.tenants WHERE id=$1`, f.tenant)
		_, _ = base.owner.Exec(cleanup, `DELETE FROM identity.principals WHERE id IN ($1,$2,$3)`, f.principal, f.otherPrincipal, f.limitedPrincipal)
		_, _ = base.owner.Exec(cleanup, `DROP ROLE IF EXISTS `+pgx.Identifier{workerName}.Sanitize())
	})
	return f
}

func t06GoFuture() time.Time { return time.Now().UTC().Add(time.Hour) }

func (f *t06GoFixture) scoped(ctx context.Context, token, store string, fn func(pgx.Tx, platform.Scope) error) error {
	return platform.WithScope(ctx, f.base.runtime, token, store, "store:read", fn)
}

func (f *t06GoFixture) register(t *testing.T, store, key string) integration.Binding {
	t.Helper()
	var out integration.Binding
	err := f.scoped(context.Background(), f.token, store, func(tx pgx.Tx, scope platform.Scope) error {
		var err error
		out, err = f.service.RegisterBinding(context.Background(), tx, scope, f.token, key, "mock_provider", "asset:"+key)
		return err
	})
	if err != nil {
		t.Fatalf("register binding: %v", err)
	}
	return out
}

func (f *t06GoFixture) plan(t *testing.T, key string, binding integration.Binding, raw string) integration.PlanResult {
	t.Helper()
	var out integration.PlanResult
	err := f.scoped(context.Background(), f.token, f.store, func(tx pgx.Tx, scope platform.Scope) error {
		var err error
		out, err = f.service.Plan(context.Background(), tx, scope, f.token, key, integration.PlanInput{
			BindingID: binding.ID, ExpectedBindingVersion: binding.SemanticVersion,
			Purpose: "transactional", Action: "payment.authorize", Request: json.RawMessage(raw),
		})
		return err
	})
	if err != nil {
		t.Fatalf("plan operation: %v", err)
	}
	return out
}

func TestT06GoBindingPermissionsScopeAndCAS(t *testing.T) {
	f := newT06GoFixture(t)
	ctx := context.Background()
	binding := f.register(t, f.store, uniqueAction("t06.binding"))
	if binding.SemanticVersion != 1 || !binding.Enabled {
		t.Fatalf("binding=%+v, want enabled v1", binding)
	}

	var disabled integration.Binding
	err := f.scoped(ctx, f.token, f.store, func(tx pgx.Tx, scope platform.Scope) error {
		var err error
		disabled, err = f.service.SetBindingEnabled(ctx, tx, scope, f.token, uniqueAction("t06.disable"), binding.ID, 1, false)
		return err
	})
	if err != nil || disabled.Enabled || disabled.SemanticVersion != 2 {
		t.Fatalf("disable=%+v err=%v", disabled, err)
	}
	err = f.scoped(ctx, f.token, f.store, func(tx pgx.Tx, scope platform.Scope) error {
		_, err := f.service.SetBindingEnabled(ctx, tx, scope, f.token, uniqueAction("t06.stale"), binding.ID, 1, true)
		return err
	})
	if !errors.Is(err, command.ErrConflict) {
		t.Fatalf("stale CAS err=%v, want conflict", err)
	}
	err = f.scoped(ctx, f.missingPermission, f.store, func(tx pgx.Tx, scope platform.Scope) error {
		_, err := f.service.RegisterBinding(ctx, tx, scope, f.missingPermission, uniqueAction("t06.denied"), "mock_provider", "asset-denied")
		return err
	})
	if !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("missing integration:manage err=%v, want forbidden", err)
	}
	err = f.scoped(ctx, f.token, f.store, func(tx pgx.Tx, scope platform.Scope) error {
		scope.StoreID = f.otherStore
		_, err := f.service.RegisterBinding(ctx, tx, scope, f.token, uniqueAction("t06.scope"), "mock_provider", "asset-scope")
		return err
	})
	if !errors.Is(err, command.ErrInvalid) {
		t.Fatalf("mismatched transaction scope err=%v, want invalid", err)
	}
	err = f.scoped(ctx, f.token, f.store, func(tx pgx.Tx, scope platform.Scope) error {
		_, err := f.service.RegisterBinding(ctx, tx, scope, f.token, uniqueAction("t06.utf8"), "mock_provider", string([]byte{0xff}))
		return err
	})
	if !errors.Is(err, command.ErrInvalid) {
		t.Fatalf("invalid UTF-8 asset err=%v, want invalid", err)
	}
}

func TestT06GoPlanAtomicReplayCanonicalAndReadback(t *testing.T) {
	f := newT06GoFixture(t)
	ctx := context.Background()
	binding := f.register(t, f.store, uniqueAction("t06.plan.binding"))
	key := uniqueAction("t06.plan")
	planned := f.plan(t, key, binding, `{"amount":9007199254740993,"nested":{"b":2,"a":1}}`)

	var got integration.Operation
	err := f.scoped(ctx, f.otherToken, f.store, func(tx pgx.Tx, scope platform.Scope) error {
		var err error
		got, err = f.service.Get(ctx, tx, scope, f.otherToken, planned.OperationID)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	err = f.scoped(ctx, f.missingPermission, f.store, func(tx pgx.Tx, scope platform.Scope) error {
		_, err := f.service.Get(ctx, tx, scope, f.missingPermission, planned.OperationID)
		return err
	})
	if !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("missing integration:read err=%v, want forbidden", err)
	}
	err = f.scoped(ctx, f.token, f.otherStore, func(tx pgx.Tx, scope platform.Scope) error {
		_, err := f.service.Get(ctx, tx, scope, f.token, planned.OperationID)
		return err
	})
	if !errors.Is(err, command.ErrNotFound) {
		t.Fatalf("foreign store read err=%v, want not found", err)
	}
	var request map[string]any
	decoder := json.NewDecoder(strings.NewReader(string(got.Request)))
	decoder.UseNumber()
	if err := decoder.Decode(&request); err != nil {
		t.Fatal(err)
	}
	amount, ok := request["amount"].(json.Number)
	if !ok || amount.String() != "9007199254740993" {
		t.Fatalf("request=%s err=%v, numeric precision changed", got.Request, err)
	}
	var args map[string]any
	if err := f.base.owner.QueryRow(ctx, `SELECT args FROM river.river_job WHERE id=$1`, planned.JobID).Scan(&args); err != nil {
		t.Fatal(err)
	}
	if len(args) != 2 || args["operation_id"] != planned.OperationID || args["version"] != float64(1) {
		t.Fatalf("job args=%v, want only operation_id/version", args)
	}
	var operations, events, jobs, receipts, audits int
	if err := f.base.owner.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM integration.operations WHERE tenant_id=$1 AND semantic_key=$2),
		(SELECT count(*) FROM integration.operation_events WHERE tenant_id=$1 AND operation_id=$3),
		(SELECT count(*) FROM river.river_job WHERE id=$4),
		(SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND operation='integration.operation.plan' AND idempotency_key=$2),
		(SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND action='integration.operation.planned')`,
		f.tenant, key, planned.OperationID, planned.JobID).Scan(&operations, &events, &jobs, &receipts, &audits); err != nil {
		t.Fatal(err)
	}
	if operations != 1 || events != 1 || jobs != 1 || receipts != 1 || audits != 1 {
		t.Fatalf("facts op=%d event=%d job=%d receipt=%d audit=%d", operations, events, jobs, receipts, audits)
	}

	if _, err := f.base.owner.Exec(ctx, `DELETE FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2
		AND operation='integration.operation.plan' AND idempotency_key=$3`, f.tenant, f.store, key); err != nil {
		t.Fatal(err)
	}
	replayed := f.plan(t, key, binding, `{"nested":{"a":1,"b":2},"amount":9007199254740993}`)
	if replayed != planned {
		t.Fatalf("permanent replay=%+v want=%+v", replayed, planned)
	}
	if err := f.base.owner.QueryRow(ctx, `SELECT count(*) FROM river.river_job WHERE kind='external_operation_v1'
		AND args->>'operation_id'=$1`, planned.OperationID).Scan(&jobs); err != nil || jobs != 1 {
		t.Fatalf("replay jobs=%d err=%v", jobs, err)
	}

	err = f.scoped(ctx, f.token, f.store, func(tx pgx.Tx, scope platform.Scope) error {
		_, err := f.service.Plan(ctx, tx, scope, f.token, key, integration.PlanInput{
			BindingID: binding.ID, ExpectedBindingVersion: 1, Purpose: "transactional",
			Action: "payment.authorize", Request: json.RawMessage(`{"amount":9007199254740994}`),
		})
		return err
	})
	if !errors.Is(err, command.ErrConflict) {
		t.Fatalf("changed request err=%v, want conflict", err)
	}
}

func TestT06GoConcurrentReplayForeignBindingAndValidation(t *testing.T) {
	f := newT06GoFixture(t)
	ctx := context.Background()
	binding := f.register(t, f.store, uniqueAction("t06.concurrent.binding"))
	key := uniqueAction("t06.concurrent")
	results := make(chan integration.PlanResult, 8)
	errorsC := make(chan error, 8)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var out integration.PlanResult
			err := f.scoped(ctx, f.token, f.store, func(tx pgx.Tx, scope platform.Scope) error {
				var err error
				out, err = f.service.Plan(ctx, tx, scope, f.token, key, integration.PlanInput{
					BindingID: binding.ID, ExpectedBindingVersion: 1, Purpose: "service",
					Action: "message.send", Request: json.RawMessage(`{"message_id":"synthetic"}`),
				})
				return err
			})
			if err != nil {
				errorsC <- err
				return
			}
			results <- out
		}()
	}
	wg.Wait()
	close(results)
	close(errorsC)
	for err := range errorsC {
		t.Fatalf("concurrent replay: %v", err)
	}
	var first integration.PlanResult
	for out := range results {
		if first == (integration.PlanResult{}) {
			first = out
		}
		if out != first {
			t.Fatalf("replay result=%+v want=%+v", out, first)
		}
	}
	var count int
	if err := f.base.owner.QueryRow(ctx, `SELECT count(*) FROM integration.operations WHERE tenant_id=$1 AND semantic_key=$2`, f.tenant, key).Scan(&count); err != nil || count != 1 {
		t.Fatalf("operation count=%d err=%v", count, err)
	}
	err := f.scoped(ctx, f.otherToken, f.store, func(tx pgx.Tx, scope platform.Scope) error {
		_, err := f.service.Plan(ctx, tx, scope, f.otherToken, key, integration.PlanInput{
			BindingID: binding.ID, ExpectedBindingVersion: 1, Purpose: "service",
			Action: "message.send", Request: json.RawMessage(`{"message_id":"synthetic"}`),
		})
		return err
	})
	if !errors.Is(err, command.ErrConflict) {
		t.Fatalf("changed actor replay err=%v, want conflict", err)
	}
	err = f.scoped(ctx, f.missingPermission, f.store, func(tx pgx.Tx, scope platform.Scope) error {
		_, err := f.service.Plan(ctx, tx, scope, f.missingPermission, uniqueAction("t06.execute.denied"), integration.PlanInput{
			BindingID: binding.ID, ExpectedBindingVersion: 1, Purpose: "service", Action: "message.send", Request: json.RawMessage(`{}`),
		})
		return err
	})
	if !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("missing integration:execute err=%v, want forbidden", err)
	}

	foreign := f.register(t, f.otherStore, uniqueAction("t06.foreign.binding"))
	err = f.scoped(ctx, f.token, f.store, func(tx pgx.Tx, scope platform.Scope) error {
		_, err := f.service.Plan(ctx, tx, scope, f.token, uniqueAction("t06.foreign.plan"), integration.PlanInput{
			BindingID: foreign.ID, ExpectedBindingVersion: 1, Purpose: "service", Action: "message.send", Request: json.RawMessage(`{}`),
		})
		return err
	})
	if !errors.Is(err, command.ErrNotFound) {
		t.Fatalf("foreign binding err=%v, want not found", err)
	}
	for name, raw := range map[string]json.RawMessage{
		"array":    json.RawMessage(`[]`),
		"trailing": json.RawMessage(`{} {}`),
		"oversize": json.RawMessage(`{"v":"` + strings.Repeat("x", 65536) + `"}`),
	} {
		t.Run(name, func(t *testing.T) {
			err := f.scoped(ctx, f.token, f.store, func(tx pgx.Tx, scope platform.Scope) error {
				_, err := f.service.Plan(ctx, tx, scope, f.token, uniqueAction("t06.invalid"), integration.PlanInput{
					BindingID: binding.ID, ExpectedBindingVersion: 1, Purpose: "service", Action: "message.send", Request: raw,
				})
				return err
			})
			if !errors.Is(err, command.ErrInvalid) {
				t.Fatalf("err=%v, want invalid", err)
			}
		})
	}
}

func TestT06GoAtomicFailuresAndWorkerTokenFence(t *testing.T) {
	f := newT06GoFixture(t)
	ctx := context.Background()
	binding := f.register(t, f.store, uniqueAction("t06.atomic.binding"))
	t06GoInstallFailureTriggers(t, f)
	for _, stage := range []string{"job", "event", "audit"} {
		t.Run(stage, func(t *testing.T) {
			key := uniqueAction("t06.fail." + stage)
			var planned integration.PlanResult
			err := f.scoped(ctx, f.token, f.store, func(tx pgx.Tx, scope platform.Scope) error {
				if _, err := tx.Exec(ctx, `SELECT set_config('app.t06_fail_stage',$1,true)`, stage); err != nil {
					return err
				}
				var err error
				planned, err = f.service.Plan(ctx, tx, scope, f.token, key, integration.PlanInput{
					BindingID: binding.ID, ExpectedBindingVersion: 1, Purpose: "transactional",
					Action: "payment.authorize", Request: json.RawMessage(`{"synthetic":true}`),
				})
				return err
			})
			if err == nil {
				t.Fatalf("%s failure unexpectedly committed", stage)
			}
			var operations, jobs int
			if err := f.base.owner.QueryRow(ctx, `SELECT
				(SELECT count(*) FROM integration.operations WHERE tenant_id=$1 AND semantic_key=$2),
				(SELECT count(*) FROM river.river_job WHERE kind='external_operation_v1' AND args->>'operation_id'=$3)`,
				f.tenant, key, planned.OperationID).Scan(&operations, &jobs); err != nil {
				t.Fatal(err)
			}
			if operations != 0 || jobs != 0 {
				t.Fatalf("%s rollback left operation=%d jobs=%d", stage, operations, jobs)
			}
		})
	}

	planned := f.plan(t, uniqueAction("t06.worker"), binding, `{"synthetic":true}`)
	tx, err := f.worker.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	claim, err := f.service.Claim(ctx, tx, planned.OperationID, 30)
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if claim.Disposition != "claimed" || claim.Mode != "dispatch" || len(claim.LeaseToken) != 32 {
		_ = tx.Rollback(ctx)
		t.Fatalf("claim=%+v", claim)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	tx, err = f.worker.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	busy, err := f.service.Claim(ctx, tx, planned.OperationID, 30)
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if busy.Disposition != "busy" || len(busy.LeaseToken) != 0 {
		_ = tx.Rollback(ctx)
		t.Fatalf("busy claim leaked token: %+v", busy)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	tx, err = f.worker.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	wrong := make([]byte, 32)
	err = f.service.Complete(ctx, tx, planned.OperationID, claim.Generation, wrong, integration.Outcome{State: "SUCCEEDED", Code: "ok", ProviderReference: "synthetic-ref"})
	_ = tx.Rollback(ctx)
	if !errors.Is(err, command.ErrConflict) {
		t.Fatalf("wrong token err=%v, want conflict", err)
	}
	tx, err = f.worker.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.service.Complete(ctx, tx, planned.OperationID, claim.Generation, claim.LeaseToken,
		integration.Outcome{State: "SUCCEEDED", Code: "ok", ProviderReference: "synthetic-ref"}); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func t06GoInstallFailureTriggers(t *testing.T, f *t06GoFixture) {
	t.Helper()
	ctx := context.Background()
	name := "t06_go_fail_" + strings.ReplaceAll(randomUUID()[:8], "-", "")
	function := pgx.Identifier{"public", name}.Sanitize()
	if _, err := f.base.owner.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$
	BEGIN
		IF current_setting('app.t06_fail_stage',true)=TG_ARGV[0] THEN
			RAISE EXCEPTION 'synthetic t06 %% failure', TG_ARGV[0];
		END IF;
		RETURN NEW;
	END $$`, function)); err != nil {
		t.Fatal(err)
	}
	triggers := []struct{ table, stage string }{
		{"river.river_job", "job"},
		{"integration.operation_events", "event"},
		{"ops.audit_events", "audit"},
	}
	for index, trigger := range triggers {
		triggerName := pgx.Identifier{fmt.Sprintf("%s_%d", name, index)}.Sanitize()
		if _, err := f.base.owner.Exec(ctx, fmt.Sprintf(`CREATE TRIGGER %s BEFORE INSERT ON %s
			FOR EACH ROW EXECUTE FUNCTION %s('%s')`, triggerName, trigger.table, function, trigger.stage)); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for index, trigger := range triggers {
			triggerName := pgx.Identifier{fmt.Sprintf("%s_%d", name, index)}.Sanitize()
			_, _ = f.base.owner.Exec(context.Background(), fmt.Sprintf(`DROP TRIGGER IF EXISTS %s ON %s`, triggerName, trigger.table))
		}
		_, _ = f.base.owner.Exec(context.Background(), `DROP FUNCTION IF EXISTS `+function+`()`) // synthetic fixture only
	})
}
