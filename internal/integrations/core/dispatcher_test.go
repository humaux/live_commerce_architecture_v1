package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/riverqueue/river"
)

func testDispatchRoute() DispatchRoute {
	return DispatchRoute{
		Provider: "synthetic", Action: "payment.authorize", Purpose: "transactional",
		Check: func(context.Context, DispatchRequest) error { return nil },
		Dispatch: func(context.Context, DispatchRequest) (Outcome, error) {
			return Outcome{State: "SUCCEEDED", Code: "ok"}, nil
		},
		Reconcile: func(context.Context, DispatchRequest) (Outcome, error) {
			return Outcome{State: "UNKNOWN", Code: "not_found"}, nil
		},
	}
}

func TestDispatcherOptionsAndRoutes(t *testing.T) {
	options := DefaultDispatcherOptions()
	if !validDispatcherOptions(options) {
		t.Fatal("default dispatcher options rejected")
	}
	invalid := []DispatcherOptions{
		{},
		{LeaseSeconds: 4, DBTimeout: time.Second, CallTimeout: time.Second, RetryDelay: time.Second, MaxGenerations: 2},
		{LeaseSeconds: 301, DBTimeout: time.Second, CallTimeout: time.Second, RetryDelay: time.Second, MaxGenerations: 2},
		{LeaseSeconds: 30, DBTimeout: 99 * time.Millisecond, CallTimeout: time.Second, RetryDelay: time.Second, MaxGenerations: 2},
		{LeaseSeconds: 30, DBTimeout: time.Second, CallTimeout: 61 * time.Second, RetryDelay: time.Second, MaxGenerations: 2},
		{LeaseSeconds: 30, DBTimeout: time.Second, CallTimeout: time.Second, RetryDelay: 99 * time.Millisecond, MaxGenerations: 2},
		{LeaseSeconds: 30, DBTimeout: time.Second, CallTimeout: time.Second, RetryDelay: time.Second, MaxGenerations: 1},
		{LeaseSeconds: 5, DBTimeout: 100 * time.Millisecond, CallTimeout: 3800 * time.Millisecond, RetryDelay: time.Second, MaxGenerations: 2},
	}
	for i, option := range invalid {
		if validDispatcherOptions(option) {
			t.Fatalf("invalid options %d accepted: %+v", i, option)
		}
	}

	route := testDispatchRoute()
	compiled, err := compileDispatchRoutes([]DispatchRoute{route})
	if err != nil {
		t.Fatal(err)
	}
	route.Provider = "changed"
	key := dispatchRouteKey{provider: "synthetic", action: "payment.authorize", purpose: "transactional"}
	if got := compiled[key].Provider; got != "synthetic" {
		t.Fatalf("route registry retained caller mutation: %q", got)
	}
	if _, err := compileDispatchRoutes([]DispatchRoute{testDispatchRoute(), testDispatchRoute()}); err == nil {
		t.Fatal("duplicate route accepted")
	}
	missing := testDispatchRoute()
	missing.Reconcile = nil
	if _, err := compileDispatchRoutes([]DispatchRoute{missing}); err == nil {
		t.Fatal("missing callback accepted")
	}
	many := make([]DispatchRoute, 65)
	for i := range many {
		many[i] = testDispatchRoute()
		many[i].Action = fmt.Sprintf("payment.action_%d", i)
	}
	if _, err := compileDispatchRoutes(many); err == nil {
		t.Fatal("unbounded route registry accepted")
	}
	if _, err := NewDispatcher(context.Background(), nil, []DispatchRoute{testDispatchRoute()}, options); err == nil {
		t.Fatal("nil worker pool accepted")
	}
}

func TestDispatcherRequestCopiesAndSanitizesCallbacks(t *testing.T) {
	operation := Operation{
		ID: "123e4567-e89b-12d3-a456-426614174000", TenantID: "tenant", StoreID: "store",
		PrincipalID: "actor", BindingID: "binding", BindingVersion: 7, Provider: "synthetic",
		ExternalAssetID: "asset", Purpose: "transactional", Action: "payment.authorize",
		Request: json.RawMessage(`{"amount":9007199254740993}`), ProviderReference: "remote-1",
	}
	checkRequest := dispatchRequest(operation)
	checkRequest.Request[0] = '['
	dispatchRequest := dispatchRequest(operation)
	if dispatchRequest.Request[0] != '{' || operation.Request[0] != '{' {
		t.Fatal("callback request mutation escaped its copy")
	}
	if dispatchRequest.IdempotencyKey != "lc:"+operation.ID || dispatchRequest.ProviderReference != "remote-1" {
		t.Fatalf("unstable callback snapshot: %+v", dispatchRequest)
	}

	secret := errors.New("provider secret must not escape")
	if err, panicked := invokeCheck(context.Background(), func(context.Context, DispatchRequest) error {
		panic(secret)
	}, dispatchRequest); err != nil || !panicked {
		t.Fatalf("check panic not sanitized: %v/%v", err, panicked)
	}
	if outcome, err, panicked := invokeOutcome(context.Background(), func(context.Context, DispatchRequest) (Outcome, error) {
		panic(secret)
	}, dispatchRequest); outcome != (Outcome{}) || err != nil || !panicked {
		t.Fatalf("outcome panic not sanitized: %+v/%v/%v", outcome, err, panicked)
	}
	for _, outcome := range []Outcome{
		{State: "BLOCKED_POLICY", Code: "adapter_must_not_choose_policy"},
		{State: "SUCCEEDED", Code: ""},
		{State: "UNKNOWN", Code: "ok", ProviderReference: "bad\nreference"},
	} {
		if validAdapterOutcome(outcome) {
			t.Fatalf("invalid adapter outcome accepted: %+v", outcome)
		}
	}
}

func TestDispatcherWorkerBoundsAndRejectsMalformedJobs(t *testing.T) {
	options := DefaultDispatcherOptions()
	dispatcher := &Dispatcher{options: options}
	if got := dispatcher.Timeout(nil); got != 30*time.Second {
		t.Fatalf("worker timeout=%s", got)
	}
	before := time.Now().UTC().Add(options.RetryDelay)
	retry := dispatcher.NextRetry(nil)
	if retry.Before(before) || retry.After(before.Add(time.Second)) {
		t.Fatalf("worker retry=%s, before=%s", retry, before)
	}
	if err := dispatcher.Work(context.Background(), nil); !errors.Is(err, errInvalidJob) {
		t.Fatalf("nil job error=%v", err)
	}
	job := &river.Job[externalOperationArgs]{Args: externalOperationArgs{
		OperationID: "123e4567-e89b-12d3-a456-426614174000", Version: 2,
	}}
	if err := dispatcher.Work(context.Background(), job); !errors.Is(err, errInvalidJob) {
		t.Fatalf("unknown job version error=%v", err)
	}
}
