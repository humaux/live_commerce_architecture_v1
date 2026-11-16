// Purpose: bridge the existing Meta compiled-process / isolated-PG fixtures to T19's Go+River integration pilot.
// Depends on: mrFixture/mrSetup/mrBuild/mrLaunch, Meta registrar SQL, meta_inbox and river_meta tables; loopback only.
// Used by: ROOT's focused CI hook; no product runtime or neutral cross-language implementation.
// Invariants: I01, I04, I11, I18, I21, I23; correctness precedes measurement; no worker/provider side effects.
package foundation_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	w1 "livecommerce/bench/l01/go-w1"
)

type l01Record struct {
	Block           w1.Block `json:"block"`
	Correctness     string   `json:"correctness"`
	UniqueEvents    int64    `json:"unique_events"`
	LinkedIntents   int64    `json:"linked_intents"`
	ExpectedUnique  int      `json:"expected_unique"`
	BodyBytes       []int    `json:"body_bytes"`
	CPUUserNS       int64    `json:"api_cpu_user_ns"`
	CPUSystemNS     int64    `json:"api_cpu_system_ns"`
	PeakRSSKiB      int64    `json:"api_peak_rss_kib"`
	Resources       string   `json:"resources"`
	DatabaseVersion string   `json:"database_version"`
}

// TestL01GoW1 owns exactly five independently seeded isolated Go+River pilot blocks.
// Frozen CI CLI (no secret arguments): bash scripts/dev/test-local.sh --l01-go-w1.
// RESULT.json is the complete artifact, including every attempt; it is not a language comparison.
func TestL01GoW1(t *testing.T) {
	report := map[string]any{
		"protocol": "Go+River SAME_HOST_PILOT integration reference v1",
		"status":   "NOT_RUN", "comparable_languages": false, "neutral_intent": false,
		"evidence_class": "NOT_RUN", "actual_model": "UNKNOWN", "actual_effort": "UNKNOWN",
		"api_processes": 1, "load_generator_processes": 1, "independent_block_fixtures": true,
		"hardware_model":  "UNKNOWN",
		"blocks_expected": 5, "attempts_per_block": w1.Attempts, "arrival_window_ns": int64(w1.Window),
		"max_inflight": w1.Concurrency, "request_deadline_ns": int64(w1.Deadline),
		"scheduler_lateness_drop_ns": int64(250 * time.Millisecond),
		"api_gomaxprocs":             2, "api_worker_processes": 0, "main_pool_max": 8, "ingress_pool_max": 8,
		"tls": "none / owned loopback", "go": runtime.Version(), "compiler": runtime.Compiler,
		"os": runtime.GOOS, "arch": runtime.GOARCH, "logical_cpus": runtime.NumCPU(),
		"build_flags":         "go build (default optimized; no race instrumentation in child API)",
		"success_denominator": "100 offered attempts/block; ACK quantiles include original and duplicate 200 responses only; 403 controls counted separately",
		"business_throughput": "60 distinct committed routed events/block, verified through independent SQL; never HTTP ACK count",
		"not_run":             []string{"neutral intent", "Rust", "Node", "W2", "W3", "providers", "independent-machine capacity", "full L01/T19/G12/G15/T20/T22", "worker RSS/GC/allocations/connection-wait/queue-age", "production"},
	}
	var records []l01Record
	defer func() {
		report["blocks"] = records
		if t.Failed() {
			report["status"] = "FAIL"
		}
		dir := filepath.Join("..", "..", "output", "ci-gates", "l01-go-w1")
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Error("artifact directory", err)
			return
		}
		raw, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			t.Error("artifact JSON", err)
			return
		}
		if err = os.WriteFile(filepath.Join(dir, "RESULT.json"), append(raw, '\n'), 0600); err != nil {
			t.Error("artifact write", err)
		}
	}()
	for key, args := range map[string][]string{"commit": {"rev-parse", "HEAD"}, "tree": {"rev-parse", "HEAD^{tree}"}} {
		out, err := exec.Command("git", args...).Output()
		if err != nil {
			t.Fatal("source pin", err)
		}
		report[key] = strings.TrimSpace(string(out))
	}
	sourceHashes := map[string]string{}
	for _, name := range []string{"bench/l01/go-w1/observer.go", "bench/l01/go-w1/observer_test.go", "tests/foundation/l01_go_w1_test.go", "go.mod", "go.sum"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", name))
		if err != nil {
			t.Fatal("source hash", err)
		}
		sum := sha256.Sum256(raw)
		sourceHashes[name] = hex.EncodeToString(sum[:])
	}
	report["source_sha256"] = sourceHashes
	for key, path := range map[string]string{"cpu_limit": "/sys/fs/cgroup/cpu.max", "memory_limit": "/sys/fs/cgroup/memory.max"} {
		raw, err := os.ReadFile(path)
		if err != nil {
			report[key] = "UNKNOWN"
		} else {
			report[key] = strings.TrimSpace(string(raw))
		}
	}
	binary := mrBuild(t, "../../cmd/api", "l01-meta-api")
	raw, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal("API binary hash", err)
	}
	sum := sha256.Sum256(raw)
	report["api_binary_sha256"] = hex.EncodeToString(sum[:])
	report["evidence_class"] = "REAL_PG / signed synthetic HTTP / SAME_HOST_PILOT"
	seeds := []int64{1901101, 1901102, 1901103, 1901104, 1901105}
	report["seeds"] = seeds
	for _, seed := range seeds {
		ok := t.Run(fmt.Sprintf("block_%d", seed), func(t *testing.T) {
			record := l01Record{ExpectedUnique: 60, Correctness: "NOT_RUN", Resources: "NOT_MEASURED", CPUUserNS: -1, CPUSystemNS: -1, PeakRSSKiB: -1}
			defer func() {
				if t.Failed() {
					record.Correctness = "FAIL"
				}
				records = append(records, record)
			}()
			l01Block(t, binary, seed, &record)
		})
		if !ok || records[len(records)-1].Correctness != "PASS" {
			t.Fatal("pilot aborted on first correctness/measurement failure; prior raw observations retained")
		}
	}
	report["status"] = "MEASURED_PILOT"
	report["correctness"] = "PASS"
	// No performance target is owner-approved; measured pilot does not imply capacity PASS.
}

func l01Block(t *testing.T, binary string, seed int64, record *l01Record) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	f := mrFixture(t)
	m := mrSetup(t, f)
	if err := f.owner.QueryRow(ctx, `SHOW server_version`).Scan(&record.DatabaseVersion); err != nil {
		t.Fatal(err)
	}
	asset := miAsset()
	binding := miBinding(t, m, asset, "facebook", f.tenantA, f.storeA1, f.principalA)
	route, epoch := miRoute(t, m, asset, f.tenantA, f.storeA1, binding)
	// Calls meta_inbox.activate_route (0028_meta_inbox.sql): a tenant/store mismatch must not create any route or receipt.
	routesBefore := miCount(t, f.owner, `SELECT count(*) FROM meta_inbox.routes`)
	_, err := m.registrar.Exec(ctx, `SELECT * FROM meta_inbox.activate_route($1,'page',$2,$3,$4,$5,1,$6,clock_timestamp()+interval '1 hour',0)`, miApp, asset, f.tenantB, f.storeA1, binding, strings.Repeat("a", 64))
	if miSQLState(err) != "PT409" || miCount(t, f.owner, `SELECT count(*) FROM meta_inbox.routes`) != routesBefore || miCount(t, f.owner, `SELECT count(*) FROM meta_inbox.events`) != 0 || miCount(t, f.owner, `SELECT count(*) FROM river_meta.river_job`) != 0 {
		t.Fatal("cross-tenant registrar refusal changed durable state")
	}
	keyJSON := fmt.Sprintf(`{"keys":[{"id":%q,"key_base64":%q}]}`, miKeyID, base64.StdEncoding.EncodeToString(m.key))
	appsJSON := fmt.Sprintf(`{"apps":[{"app_id":%q,"object":"page","app_secret":%q,"verify_token":"meta-inbox-verify-token"}]}`, miApp, miSecret)
	api := mrLaunch(t, binary, "l01-api", []string{
		"LISTEN_ADDR=" + mrEphemeralAddr, "DATABASE_URL=" + f.runtime.Config().ConnString(), "GOMAXPROCS=2",
		"COMMERCE_META_WEBHOOK_ENABLED=1", "COMMERCE_META_INGRESS_DATABASE_URL=" + miRole(t, f, "commerce_meta_ingress"),
		"COMMERCE_META_APPS_JSON=" + appsJSON, "COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID=" + miKeyID, "COMMERCE_META_PAYLOAD_KEYS_JSON=" + keyJSON,
	})
	origin := "http://" + mrBoundAddr(t, api, 8*time.Second)
	target := origin + "/v1/meta/webhooks/" + miApp + "/page"
	client := &http.Client{Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	// Correctness controls run before offering any measured load.
	preflight := miMessage(asset, "m."+randomUUID(), "synthetic-w1-preflight")
	for i := 0; i < 2; i++ {
		status, body := mrHTTP(t, client, http.MethodPost, target, preflight, miSignature(preflight))
		miStatus(t, status, body, 200)
	}
	if miCount(t, f.owner, `SELECT count(*) FROM meta_inbox.events`) != 1 || miCount(t, f.owner, `SELECT count(*) FROM river_meta.river_job`) != 1 {
		t.Fatal("preflight dedup violated")
	}
	bad := miMessage(asset, "m."+randomUUID(), "synthetic-invalid-signature")
	batchesBefore := miCount(t, f.owner, `SELECT count(*) FROM meta_inbox.batches`)
	status, body := mrHTTP(t, client, http.MethodPost, target, bad, "sha256="+strings.Repeat("0", 64))
	miStatus(t, status, body, 403)
	if miCount(t, f.owner, `SELECT count(*) FROM meta_inbox.batches`) != batchesBefore || miCount(t, f.owner, `SELECT count(*) FROM meta_inbox.events`) != 1 || miCount(t, f.owner, `SELECT count(*) FROM river_meta.river_job`) != 1 {
		t.Fatal("bad signature persisted")
	}
	unknown := miMessage(miAsset(), "m."+randomUUID(), "synthetic-unknown-asset")
	status, body = mrHTTP(t, client, http.MethodPost, target, unknown, miSignature(unknown))
	miStatus(t, status, body, 200)
	if miCount(t, f.owner, `SELECT count(*) FROM meta_inbox.events e JOIN meta_private.quarantine_bodies q ON q.event_id=e.id WHERE e.disposition='QUARANTINED' AND e.tenant_id IS NULL AND e.store_id IS NULL AND e.job_id IS NULL AND octet_length(q.ciphertext)>16`) != 1 || miCount(t, f.owner, `SELECT count(*) FROM river_meta.river_job`) != 1 {
		t.Fatal("unknown asset must retain encrypted unscoped quarantine without business job")
	}
	requests := make([]w1.Request, 0, w1.Attempts)
	for i := 0; i < 60; i++ {
		requests = append(requests, w1.Request{Body: miMessage(asset, fmt.Sprintf("m.w1.%d.%03d", seed, i), "synthetic-w1-fixed-body"), TenantHeader: f.tenantB})
	}
	for i := 0; i < 20; i++ {
		requests = append(requests, requests[i])
	}
	for i := 0; i < 20; i++ {
		requests = append(requests, w1.Request{Body: miMessage(asset, fmt.Sprintf("m.bad.%d.%03d", seed, i), "synthetic-w1-fixed-body"), BadSignature: true})
	}
	rng := rand.New(rand.NewSource(seed))
	rng.Shuffle(len(requests), func(i, j int) { requests[i], requests[j] = requests[j], requests[i] })
	for _, r := range requests {
		record.BodyBytes = append(record.BodyBytes, len(r.Body))
	}
	block, err := w1.Run(ctx, target, miSecret, requests, seed)
	record.Block = block
	if err != nil {
		t.Fatal(err)
	}
	// Independent SQL checks business cardinality, exact event->intent args, scope and transaction lineage.
	record.UniqueEvents = miCount(t, f.owner, `SELECT count(*) FROM meta_inbox.events WHERE disposition='ROUTED'`) - 1
	record.LinkedIntents = miCount(t, f.owner, `SELECT count(*) FROM meta_inbox.events e JOIN river_meta.river_job j ON j.id=e.job_id WHERE e.disposition='ROUTED' AND e.tenant_id=$1 AND e.store_id=$2 AND e.binding_id=$3 AND e.route_id=$4 AND e.route_epoch=$5 AND e.completed AND e.is_primary AND EXISTS (SELECT 1 FROM meta_inbox.batch_events be JOIN meta_inbox.batches b ON b.id=be.batch_id WHERE be.event_id=e.id AND b.finalized AND b.admission_xid=e.admission_xid) AND j.queue='meta_inbox' AND j.unique_key IS NULL AND j.kind='meta_inbox_v1' AND j.attempt=0 AND j.state='available' AND j.args=jsonb_build_object('event_id',e.id::text,'version',1)`, f.tenantA, f.storeA1, binding, route, epoch) - 1
	if record.UniqueEvents != 60 || record.LinkedIntents != 60 || miCount(t, f.owner, `SELECT count(*) FROM meta_inbox.events`) != 62 || miCount(t, f.owner, `SELECT count(*) FROM river_meta.river_job`) != 61 || miCount(t, f.owner, `SELECT count(*) FROM social.messages`) != 0 || miCount(t, f.owner, `SELECT count(*) FROM meta_inbox.events WHERE tenant_id=$1`, f.tenantB) != 0 {
		t.Fatal("Inbox/intent cardinality, linkage, dedup or server-side tenant scope violated")
	}
	if err := w1.Validate(block); err != nil {
		t.Fatal("measurement verdict", err)
	}
	if block.ACKs != 80 || block.Rejected != 20 {
		t.Fatal("offered-load mix changed")
	}
	record.Correctness = "PASS"
	mrLogNoSecrets(t, api, miSecret, keyJSON, appsJSON, f.tenantA, f.tenantB)
	mrStop(t, api, syscall.SIGTERM, true)
	if state := api.cmd.ProcessState; state != nil {
		record.CPUUserNS = int64(state.UserTime())
		record.CPUSystemNS = int64(state.SystemTime())
		record.Resources = "owned API lifetime CPU (startup + controls + load + shutdown); peak RSS only on Linux"
		if usage, ok := state.SysUsage().(*syscall.Rusage); ok && runtime.GOOS == "linux" {
			record.PeakRSSKiB = usage.Maxrss
		}
	}
}
