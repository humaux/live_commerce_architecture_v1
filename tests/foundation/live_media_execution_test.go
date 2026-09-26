package foundation_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"livecommerce/internal/integrations/livekit"
	"livecommerce/internal/live"
	"livecommerce/internal/platform"
)

// Every endpoint below is a task-owned loopback TLS server. The Cloud-shaped
// Host is retained for LKP validation, but an unrecognised dial target fails.
func lmeTransport(address string) *http.Transport {
	return &http.Transport{
		Proxy: nil, DisableKeepAlives: true,
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true}, // pinned local fixture only
		DialContext: func(ctx context.Context, _, target string) (net.Conn, error) {
			if target != "unit.livekit.cloud:443" {
				return nil, errors.New("unapproved fixture address")
			}
			return (&net.Dialer{}).DialContext(ctx, "tcp", address)
		},
	}
}

func lmeConfig() livekit.Config {
	return livekit.Config{Environment: "MOCK", Endpoint: "https://unit.livekit.cloud",
		APIKey: "lme_test_key", APISecret: strings.Repeat("s", 40),
		StreamHosts: []string{"ingest.example.com"}}
}

type lmeHarness struct {
	*lmpHarness
	plan      live.MediaStartResult
	worker    *pgxpool.Pool
	executor  *pgxpool.Pool
	keys      *livekit.MaterialKeyring
	project   live.MediaProject
	server    *httptest.Server
	starts    atomic.Int32
	lists     atomic.Int32
	queries   atomic.Int32
	stops     atomic.Int32
	streamURL string
}

func lmeSetup(t *testing.T, handler http.HandlerFunc) *lmeHarness {
	t.Helper()
	h := &lmeHarness{lmpHarness: &lmpHarness{lmaHarness: lmaSetup(t)}}
	h.streamURL = "rtmps://ingest.example.com/live/lme-secret-" + t04Tag()
	h.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/twirp/livekit.Egress/StartEgress":
			h.starts.Add(1)
		case "/twirp/livekit.Egress/ListEgress":
			h.lists.Add(1)
		case "/twirp/livekit.Egress/GetEgress":
			h.queries.Add(1)
		case "/twirp/livekit.Egress/StopEgress":
			h.stops.Add(1)
		default:
			t.Errorf("unexpected provider path: %s", r.URL.Path)
		}
		handler(w, r)
	}))
	t.Cleanup(h.server.Close)
	config := lmeConfig()
	transport := lmeTransport(h.server.Listener.Addr().String())
	client, err := livekit.New(config, transport)
	if err != nil {
		t.Fatal(err)
	}
	h.keys, err = livekit.NewMaterialKeyring("lma_key_1", map[string][]byte{"lma_key_1": bytes.Repeat([]byte{0x37}, 32)})
	if err != nil {
		t.Fatal(err)
	}
	h.specification = h.spec()
	input := livekit.StartInput{RoomName: "lc_" + strings.ReplaceAll(h.specification["attempt_id"].(string), "-", ""),
		AspectRatio: "16:9", StreamURLs: []string{h.streamURL}}
	scope := livekit.MaterialScope{TenantID: h.lp.f.tenantA, StoreID: h.lp.f.storeA1,
		SessionID: h.session, AttemptID: h.specification["attempt_id"].(string), ProjectID: "project_lma",
		CredentialVersion: 1, MaterialVersion: 1}
	sealed, err := h.keys.Seal(scope, client, input)
	if err != nil {
		t.Fatal(err)
	}
	if sealed.KeyID != "lma_key_1" {
		t.Fatal("fixture key mismatch")
	}
	if _, err = lmaRegister(context.Background(), h.registrar, h.specification, sealed.Nonce, sealed.Ciphertext); err != nil {
		t.Fatal(err)
	}
	h.input = live.MediaStartInput{SessionID: h.session, AuthorizationID: h.specification["id"].(string), ExpectedSessionVersion: 1}
	h.planner = lmpPlanner(t, h.lp.f.runtime, "river_media")
	// Cleanup precedes LMA/LSP cleanup; owner can remove only this fixture's rows.
	t.Cleanup(func() {
		if h.plan.AttemptID == "" {
			return
		}
		ctx := context.Background()
		tx, err := h.lp.f.owner.Begin(ctx)
		if err != nil {
			t.Error(err)
			return
		}
		defer tx.Rollback(ctx)
		queries := []string{
			`DELETE FROM live.media_observations WHERE attempt_id=$1`,
			`DELETE FROM live.media_execution_state WHERE attempt_id=$1`,
			`DELETE FROM river_media.river_job WHERE id=$1`,
			`DELETE FROM integration.operation_events WHERE operation_id=$1`,
			`DELETE FROM integration.operations WHERE id=$1`,
			`DELETE FROM live.media_attempts WHERE id=$1`,
		}
		for i, q := range queries {
			var arg any = h.plan.AttemptID
			switch i {
			case 2:
				arg = h.plan.JobID
			case 3, 4:
				arg = h.plan.OperationID
			}
			if _, err = tx.Exec(ctx, q, arg); err != nil {
				t.Errorf("execution cleanup %d: %v", i, err)
				return
			}
		}
		if _, err = tx.Exec(ctx, `DELETE FROM ops.command_results WHERE principal_id=$1 AND operation='live.media.start'`, h.lp.actor); err != nil {
			t.Error(err)
			return
		}
		if _, err = tx.Exec(ctx, `DELETE FROM ops.audit_events WHERE principal_id=$1 AND action='live.media.start.planned'`, h.lp.actor); err != nil {
			t.Error(err)
			return
		}
		if err = tx.Commit(ctx); err != nil {
			t.Error(err)
		}
	})
	h.plan, err = h.start(t04Key("lme-start"))
	if err != nil {
		t.Fatal(err)
	}
	if h.plan.State != "READY" || h.plan.AttemptID != scope.AttemptID || h.plan.RoomName != input.RoomName {
		t.Fatalf("bad frozen plan: %+v", h.plan)
	}
	h.project = live.MediaProject{ProjectID: "project_lma", CredentialVersion: 1, Config: config, Transport: transport}
	_, h.worker = lmaLogin(t, h.lp.f, "commerce_media_worker")
	_, h.executor = lmaLogin(t, h.lp.f, "commerce_media_executor")
	return h
}

func (h *lmeHarness) startWorker(t *testing.T) *river.Client[pgx.Tx] {
	t.Helper()
	client, err := live.NewMediaClient(context.Background(), h.worker, h.executor, h.keys, []live.MediaProject{h.project}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err = client.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		if err := client.StopAndCancel(ctx); err != nil {
			t.Error("media worker did not stop")
		}
	})
	return client
}

type lmeFacts struct {
	operation, result, resource, status, egress string
	generation, observations, events            int64
	reserved, cleanup, escalated                bool
}

func (h *lmeHarness) facts(t *testing.T) lmeFacts {
	t.Helper()
	var f lmeFacts
	err := h.lp.f.owner.QueryRow(context.Background(), `SELECT o.state,coalesce(o.result_code,''),o.generation,
		coalesce(e.resource_state,''),coalesce(e.transport_status,''),coalesce(e.egress_id,''),
		coalesce(e.wire_reserved_at IS NOT NULL,false),coalesce(e.cleanup_required,false),coalesce(e.escalated_at IS NOT NULL,false),
		(SELECT count(*) FROM live.media_observations WHERE attempt_id=$1),
		(SELECT count(*) FROM integration.operation_events WHERE operation_id=$2)
		FROM integration.operations o LEFT JOIN live.media_execution_state e ON e.operation_id=o.id WHERE o.id=$2`,
		h.plan.AttemptID, h.plan.OperationID).Scan(&f.operation, &f.result, &f.generation,
		&f.resource, &f.status, &f.egress, &f.reserved, &f.cleanup, &f.escalated, &f.observations, &f.events)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func (h *lmeHarness) await(t *testing.T, max time.Duration, pred func(lmeFacts) bool) lmeFacts {
	t.Helper()
	deadline := time.Now().Add(max)
	for time.Now().Before(deadline) {
		f := h.facts(t)
		if pred(f) {
			return f
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("media execution fact not reached: %+v", h.facts(t))
	return lmeFacts{}
}

type lmeLease struct {
	disposition, mode string
	generation        int64
	token             []byte
}

func (h *lmeHarness) claim(t *testing.T, seconds int) lmeLease {
	t.Helper()
	lease := lmeLease{token: randomBytes(32)}
	if err := h.executor.QueryRow(context.Background(), `SELECT disposition,generation,mode FROM live.claim_media_operation($1::uuid,$2::bigint,$3::integer,$4::bytea)`,
		h.plan.OperationID, h.plan.JobID, seconds, lease.token).Scan(&lease.disposition, &lease.generation, &lease.mode); err != nil {
		t.Fatal(err)
	}
	if lease.generation < 0 || !strings.Contains("|claimed|busy|terminal|escalated|", "|"+lease.disposition+"|") ||
		(lease.disposition == "claimed" && lease.mode != "dispatch" && lease.mode != "reconcile") ||
		(lease.disposition != "claimed" && lease.mode != "") {
		t.Fatalf("noncanonical claim: %+v", lease)
	}
	return lease
}

func (h *lmeHarness) load(t *testing.T, lease lmeLease) map[string]any {
	t.Helper()
	var raw []byte
	if err := h.executor.QueryRow(context.Background(), `SELECT live.load_media_material($1::uuid,$2::bigint,$3::bytea)`,
		h.plan.OperationID, lease.generation, lease.token).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	keys := []string{"tenant_id", "store_id", "session_id", "attempt_id", "project_id", "endpoint_identity", "credential_version", "material_version", "room_name", "aspect_ratio", "egress_id", "mode", "key_id", "nonce_hex", "ciphertext_hex"}
	if len(got) != len(keys) {
		t.Fatalf("material key count %d, want 15", len(got))
	}
	for _, key := range keys {
		if _, ok := got[key]; !ok {
			t.Fatalf("missing material key %s", key)
		}
	}
	for _, key := range []string{"credential_version", "material_version"} {
		if _, ok := got[key].(float64); !ok {
			t.Fatalf("%s not JSON number", key)
		}
	}
	for _, key := range keys {
		if key != "credential_version" && key != "material_version" {
			if _, ok := got[key].(string); !ok {
				t.Fatalf("%s not JSON string", key)
			}
		}
	}
	if got["attempt_id"] != h.plan.AttemptID || got["room_name"] != h.plan.RoomName || got["mode"] != lease.mode ||
		got["project_id"] != "project_lma" || got["endpoint_identity"] != "https://unit.livekit.cloud" {
		t.Fatalf("material scope mismatch: %v", got)
	}
	if lease.mode == "reconcile" && (got["key_id"] != "" || got["nonce_hex"] != "" || got["ciphertext_hex"] != "") {
		t.Fatal("reconcile exposed sealed material")
	}
	return got
}

func (h *lmeHarness) reserve(t *testing.T, lease lmeLease) error {
	t.Helper()
	_, err := h.executor.Exec(context.Background(), `SELECT live.reserve_media_start($1::uuid,$2::bigint,$3::bytea)`, h.plan.OperationID, lease.generation, lease.token)
	return err
}

func (h *lmeHarness) record(t *testing.T, lease lmeLease, source, id, status string, started, updated, ended int64) (string, error) {
	t.Helper()
	var out string
	err := h.executor.QueryRow(context.Background(), `SELECT live.record_media_observation($1::uuid,$2::bigint,$3::bytea,$4,$5,$6,$7,$8::bigint,$9::bigint,$10::bigint)`,
		h.plan.OperationID, lease.generation, lease.token, source, id, h.plan.RoomName, status, started, updated, ended).Scan(&out)
	return out, err
}

func lmeReply(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, body)
}

func (h *lmeHarness) observation(id, status string, started, updated, ended int64) string {
	return fmt.Sprintf(`{"egress_id":%q,"room_name":%q,"status":%q,"started_at":%q,"updated_at":%q,"ended_at":%q}`,
		id, h.plan.RoomName, status, fmt.Sprint(started), fmt.Sprint(updated), fmt.Sprint(ended))
}

func TestLiveMediaExecutionLME01RolesSignaturesAndSecretBoundary(t *testing.T) {
	h := lmeSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "unexpected I/O", 500) })
	ctx := context.Background()
	if err := platform.ValidateMediaWorkerPool(ctx, h.worker); err != nil {
		t.Fatalf("clean native role rejected: %v", err)
	}
	if err := platform.ValidateMediaExecutorPool(ctx, h.executor); err != nil {
		t.Fatalf("clean executor rejected: %v", err)
	}
	if _, err := live.NewMediaClient(ctx, h.worker, h.executor, h.keys, []live.MediaProject{h.project}, 1); err != nil {
		t.Fatalf("clean constructor rejected: %v", err)
	}
	for _, signature := range []string{
		"live.claim_media_operation(uuid,bigint,integer,bytea)", "live.load_media_material(uuid,bigint,bytea)",
		"live.reserve_media_start(uuid,bigint,bytea)", "live.record_media_observation(uuid,bigint,bytea,text,text,text,text,bigint,bigint,bigint)",
		"live.finish_media_uncertain(uuid,bigint,bytea,text)",
	} {
		var owner, result string
		var oid uint32
		if err := h.lp.f.owner.QueryRow(ctx, `SELECT p.oid,pg_get_userbyid(p.proowner),pg_get_function_result(p.oid)
		 FROM pg_proc p WHERE p.oid=to_regprocedure($1)`, signature).Scan(&oid, &owner, &result); err != nil {
			t.Fatalf("missing fixed function %s: %v", signature, err)
		}
		if owner != "commerce_media_writer" || result == "" {
			t.Fatalf("wrong owner/return of %s: %s %s", signature, owner, result)
		}
		for label, pool := range map[string]*pgxpool.Pool{"native": h.worker, "runtime": h.lp.f.runtime, "registrar": h.registrar} {
			var allowed bool
			if err := pool.QueryRow(ctx, `SELECT has_function_privilege(current_user,$1::oid,'EXECUTE')`, oid).Scan(&allowed); err != nil || allowed {
				t.Fatalf("%s reaches %s: %v %v", label, signature, allowed, err)
			}
		}
	}
	for _, table := range []string{"live.prepared_media_authorizations", "live.media_execution_state", "live.media_observations"} {
		for label, pool := range map[string]*pgxpool.Pool{"native": h.worker, "executor": h.executor} {
			var n int
			if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); err == nil {
				t.Fatalf("%s directly read %s", label, table)
			}
		}
	}
	if h.starts.Load()+h.lists.Load()+h.queries.Load()+h.stops.Load() != 0 {
		t.Fatal("role test made provider call")
	}
}

func TestLiveMediaExecutionLME02NativeRiverTLSAndNoOpenTransaction(t *testing.T) {
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	var h *lmeHarness
	h = lmeSetup(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/twirp/livekit.Egress/StartEgress" {
			http.Error(w, "unexpected", 500)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		var wire struct {
			Room    string `json:"room_name"`
			Outputs []struct {
				Stream struct {
					URLs []string `json:"urls"`
				} `json:"stream"`
			} `json:"outputs"`
		}
		if err := json.Unmarshal(body, &wire); err != nil {
			t.Error(err)
			return
		}
		if r.Method != http.MethodPost || r.Host != "unit.livekit.cloud" || wire.Room != h.plan.RoomName ||
			len(wire.Outputs) != 1 || len(wire.Outputs[0].Stream.URLs) != 1 || wire.Outputs[0].Stream.URLs[0] != h.streamURL {
			t.Error("Start wire host/method/body mismatch")
		}
		select {
		case entered <- struct{}{}:
		default:
		}
		select {
		case <-release:
		case <-r.Context().Done():
		}
		lmeReply(w, h.observation("EG_lme02", "EGRESS_ACTIVE", 100, 110, 0))
	})
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	h.startWorker(t)
	select {
	case <-entered:
	case <-time.After(15 * time.Second):
		t.Fatal("native River never made Start")
	}
	// A distinct owner session must acquire the operation row while the TLS
	// provider handler remains blocked; otherwise a DB transaction crossed I/O.
	tx, err := h.lp.f.owner.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	var id string
	if err = tx.QueryRow(context.Background(), `SELECT id::text FROM integration.operations WHERE id=$1 FOR UPDATE NOWAIT`, h.plan.OperationID).Scan(&id); err != nil || id != h.plan.OperationID {
		t.Fatalf("provider I/O held operation lock: %v", err)
	}
	if err = tx.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	releaseOnce.Do(func() { close(release) })
	f := h.await(t, 15*time.Second, func(f lmeFacts) bool { return f.observations == 1 })
	if f.operation != "UNKNOWN" || f.resource != "OBSERVED" || f.status != "EGRESS_ACTIVE" || f.egress != "EG_lme02" || !f.reserved || h.starts.Load() != 1 || h.stops.Load() != 0 {
		t.Fatalf("bad durable nonterminal Start: %+v starts=%d stops=%d", f, h.starts.Load(), h.stops.Load())
	}
}

func TestLiveMediaExecutionLME03ObservedWaitAndFencedReservation(t *testing.T) {
	h := lmeSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "no reservation allowed", 500) })
	lease := h.claim(t, 30)
	if lease.disposition != "claimed" || lease.mode != "dispatch" || lease.generation == 0 {
		t.Fatalf("initial lease: %+v", lease)
	}
	material := h.load(t, lease)
	if material["key_id"] != "lma_key_1" || material["egress_id"] != "" || material["nonce_hex"] == "" || material["ciphertext_hex"] == "" {
		t.Fatal("dispatch material not exact sealed envelope")
	}
	if h.facts(t).reserved {
		t.Fatal("load reserved a wire call")
	}
	ctx := context.Background()
	holder, err := h.lp.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(ctx)
	var holderPID int
	if err = holder.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
		t.Fatal(err)
	}
	if _, err = holder.Exec(ctx, `SELECT 1 FROM integration.bindings WHERE id=$1 FOR UPDATE`, h.media); err != nil {
		t.Fatal(err)
	}
	conn, err := h.executor.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	var waiterPID int
	if err = conn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&waiterPID); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, e := conn.Exec(ctx, `SELECT live.reserve_media_start($1::uuid,$2::bigint,$3::bytea)`, h.plan.OperationID, lease.generation, lease.token)
		done <- e
	}()
	lmaObserveBlock(t, h.lp.f.owner, waiterPID, holderPID, false)
	// Revocation is committed while reserve is genuinely waiting on the binding.
	if _, err = h.lp.f.owner.Exec(ctx, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='live:manage'`,
		h.lp.f.tenantA, h.lp.f.storeA1, h.lp.actor); err != nil {
		t.Fatal(err)
	}
	if err = holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
		if err == nil {
			t.Fatal("reservation ignored revoked grant after lock wait")
		}
	case <-time.After(6 * time.Second):
		t.Fatal("reserve waiter stuck")
	}
	if f := h.facts(t); f.reserved || f.observations != 0 || h.starts.Load() != 0 {
		t.Fatalf("denied reserve wrote facts or wire: %+v", f)
	}
	for _, bad := range []struct {
		name  string
		gen   int64
		token []byte
	}{
		{"wrong-token", lease.generation, randomBytes(32)}, {"stale-generation", lease.generation - 1, lease.token},
	} {
		if _, err := h.executor.Exec(ctx, `SELECT live.reserve_media_start($1::uuid,$2::bigint,$3::bytea)`, h.plan.OperationID, bad.gen, bad.token); err == nil {
			t.Fatalf("%s reserved", bad.name)
		}
	}
	if h.starts.Load()+h.lists.Load()+h.queries.Load()+h.stops.Load() != 0 {
		t.Fatal("provider called after denied reservation")
	}
}

// The child runs the same native River constructor in a distinct OS process.
// Parent owns and kills it; no provider or database address outside the local
// task fixture can be supplied by these narrowly validated environment vars.
func TestLiveMediaExecutionCrashChild(t *testing.T) {
	if os.Getenv("LC_LME_CHILD") != "1" {
		return
	}
	workerDSN, executorDSN, address := os.Getenv("LC_LME_WORKER_DSN"), os.Getenv("LC_LME_EXECUTOR_DSN"), os.Getenv("LC_LME_TLS_ADDR")
	if workerDSN == "" || executorDSN == "" || !strings.HasPrefix(address, "127.0.0.1:") {
		t.Fatal("invalid child fixture")
	}
	ctx := context.Background()
	worker, err := pgxpool.New(ctx, workerDSN)
	if err != nil {
		t.Fatal("child worker pool")
	}
	defer worker.Close()
	executor, err := pgxpool.New(ctx, executorDSN)
	if err != nil {
		t.Fatal("child executor pool")
	}
	defer executor.Close()
	keys, err := livekit.NewMaterialKeyring("lma_key_1", map[string][]byte{"lma_key_1": bytes.Repeat([]byte{0x37}, 32)})
	if err != nil {
		t.Fatal("child fixture key")
	}
	project := live.MediaProject{ProjectID: "project_lma", CredentialVersion: 1, Config: lmeConfig(), Transport: lmeTransport(address)}
	client, err := live.NewMediaClient(ctx, worker, executor, keys, []live.MediaProject{project}, 1)
	if err != nil {
		t.Fatal("child constructor")
	}
	if err := client.Start(ctx); err != nil {
		t.Fatal("child start")
	}
	select {}
}

func TestLiveMediaExecutionLME04LostReplyProcessDeathAndRoomRecovery(t *testing.T) {
	accepted := make(chan struct{}, 1)
	release := make(chan struct{})
	var once sync.Once
	var h *lmeHarness
	h = lmeSetup(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/twirp/livekit.Egress/StartEgress":
			_, _ = io.Copy(io.Discard, r.Body)
			select {
			case accepted <- struct{}{}:
			default:
			}
			select {
			case <-release:
			case <-r.Context().Done():
			}
			if conn, _, err := w.(http.Hijacker).Hijack(); err == nil {
				_ = conn.Close()
			}
		case "/twirp/livekit.Egress/ListEgress":
			body, _ := io.ReadAll(r.Body)
			if !bytes.Contains(body, []byte(h.plan.RoomName)) {
				t.Error("room discovery was not exact")
			}
			lmeReply(w, `{"items":[`+h.observation("EG_lme04", "EGRESS_ACTIVE", 100, 120, 0)+`]}`)
		case "/twirp/livekit.Egress/GetEgress":
			body, _ := io.ReadAll(r.Body)
			if !bytes.Contains(body, []byte("EG_lme04")) {
				t.Error("Query did not use pinned ID")
			}
			lmeReply(w, h.observation("EG_lme04", "EGRESS_COMPLETE", 100, 150, 140))
		default:
			http.Error(w, "no Stop", 500)
		}
	})
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(executable, "-test.run=^TestLiveMediaExecutionCrashChild$", "-test.timeout=45s")
	child.Env = append(os.Environ(), "LC_LME_CHILD=1", "LC_LME_WORKER_DSN="+h.worker.Config().ConnString(),
		"LC_LME_EXECUTOR_DSN="+h.executor.Config().ConnString(), "LC_LME_TLS_ADDR="+h.server.Listener.Addr().String())
	child.Stdout, child.Stderr = io.Discard, io.Discard
	if err = child.Start(); err != nil {
		t.Fatal("child start failed")
	}
	childDone := make(chan error, 1)
	go func() { childDone <- child.Wait() }()
	t.Cleanup(func() { _ = child.Process.Kill() })
	select {
	case <-accepted:
	case <-childDone:
		t.Fatal("child stopped before accepted Start")
	case <-time.After(15 * time.Second):
		t.Fatal("child did not reach Start")
	}
	if !h.facts(t).reserved {
		t.Fatal("Start reached provider without committed reservation")
	}
	// The provider accepted the resource but the process dies before its reply.
	if err = child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-childDone:
		if err == nil {
			t.Fatal("killed child exited cleanly")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("child not reaped")
	}
	once.Do(func() { close(release) })
	if _, err = h.lp.f.owner.Exec(context.Background(), `UPDATE integration.operations SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1`, h.plan.OperationID); err != nil {
		t.Fatal(err)
	}
	if _, err = h.lp.f.owner.Exec(context.Background(), `UPDATE river_media.river_job SET attempted_at=clock_timestamp()-interval '2 hours' WHERE id=$1 AND state='running'`, h.plan.JobID); err != nil {
		t.Fatal(err)
	}
	if _, err = h.registrar.Exec(context.Background(), `SELECT live.revoke_prepared_media($1::uuid,$2::uuid,$3::uuid,$4::text)`,
		h.lp.f.tenantA, h.lp.f.storeA1, h.input.AuthorizationID, "operator_revoke"); err != nil {
		t.Fatalf("revoke after accepted Start: %v", err)
	}
	h.startWorker(t)
	f := h.await(t, 45*time.Second, func(f lmeFacts) bool { return f.egress == "EG_lme04" && f.observations >= 1 })
	if h.starts.Load() != 1 || h.lists.Load() == 0 || f.operation != "UNKNOWN" || f.resource != "OBSERVED" {
		t.Fatalf("recovery repeated Start or lost room: %+v starts=%d lists=%d", f, h.starts.Load(), h.lists.Load())
	}
	// The same native job remains alive for its next exact-ID observation.
	h.await(t, 45*time.Second, func(f lmeFacts) bool { return f.resource == "TERMINAL" })
	if h.starts.Load() != 1 || h.queries.Load() == 0 || h.stops.Load() != 0 {
		t.Fatalf("recovery wire counts start/list/query/stop=%d/%d/%d/%d", h.starts.Load(), h.lists.Load(), h.queries.Load(), h.stops.Load())
	}
}
