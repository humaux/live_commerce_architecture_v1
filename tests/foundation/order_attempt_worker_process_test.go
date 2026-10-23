// Purpose: REAL_PG packaged claims-worker acceptance of OA1's physical database boundary.
// Depends on: existing isolated process fixtures, narrow logins and the real River worker; mrEphemeralAddr/mrConsoleBoundAddr (CMD-2).
// Used by: OA1 CI before activating execution and recovery in the combined worker.
// Invariants: intake, retention and store verification share the worker's physical
// database; same-name clones refuse before worker routes or the bridge listener start. The bridge
// gets a loopback :0 and the test dials the port the worker logged, never one picked in advance.
package foundation_test

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestOrderAttemptProcessWorkerRejectsSplitParticipantPools(t *testing.T) {
	f, clone := mrFixture(t), mrFixture(t)
	var primaryName, cloneName string
	if err := f.owner.QueryRow(context.Background(), `SELECT current_database()`).Scan(&primaryName); err != nil {
		t.Fatal(err)
	}
	if err := clone.owner.QueryRow(context.Background(), `SELECT current_database()`).Scan(&cloneName); err != nil || primaryName != cloneName {
		t.Fatal("negative fixture must use the same database name on another cluster", err)
	}
	w := crNewWorkerEnv(t, f)
	names := []string{"COMMERCE_CLAIMS_INTAKE_DATABASE_URL", "COMMERCE_RETENTION_JOB_DATABASE_URL", "COMMERCE_STORE_VERIFY_DATABASE_URL"}
	roles := []string{"commerce_claims_intake", "commerce_retention_job", "commerce_storefront_verifier"}
	poolNames := append([]string{"COMMERCE_WORKER_DATABASE_URL"}, names...)
	var secrets []string
	for i, entry := range w.env {
		name, value, _ := strings.Cut(entry, "=")
		for _, poolName := range poolNames {
			if name == poolName {
				value = mrNamedDSN(t, value, name)
				w.env[i] = name + "=" + value
				secrets = append(secrets, value)
			}
		}
	}
	other := make([]string, len(roles))
	for i, role := range roles {
		dsn, _, password := crLoginDSN(t, clone, false, role)
		other[i] = mrNamedDSN(t, dsn, names[i])
		secrets = append(secrets, other[i], password)
	}
	linkKey := base64.StdEncoding.EncodeToString(w.linkRaw)
	pageKey := base64.StdEncoding.EncodeToString(w.pageKeyRaw)
	secrets = append(secrets, linkKey, pageKey, w.jobDSNSecret)
	binary := mrBuild(t, "../../cmd/claims-worker", "oa-worker-pool-boundary")
	environment := func(split int) []string {
		env := append([]string(nil), w.env...)
		if split >= 0 {
			env = w.with(names[split], other[split], false)
		}
		// This worker has no healthz endpoint. Its existing optional bridge is
		// the actual listening surface, configured identically in every case.
		// CMD-2: the worker binds a loopback :0 and reports the port it got, so no
		// test picks a port in advance (mrEphemeralAddr + mrConsoleBoundAddr).
		return append(env, "COMMERCE_CLAIMS_CONSOLE_ADDR="+mrEphemeralAddr,
			"COMMERCE_CLAIMS_BRIDGE_TOKEN="+linkKey, "COMMERCE_CLAIMS_CURSOR_KEY="+pageKey)
	}
	if !t.Run("same-physical-database", func(t *testing.T) {
		p := crLaunch(t, binary, environment(-1))
		// P2 (PR #168 review): one readiness budget from launch, shared by address discovery and the probe. The pre-PR test
		// spent this same eight seconds on the probe alone, so discovery must not open a second window.
		deadline := time.Now().Add(8 * time.Second)
		address := mrConsoleBoundAddr(t, p, time.Until(deadline))
		client := &http.Client{Timeout: time.Second}
		listening := false
		for time.Now().Before(deadline) {
			response, err := client.Get("http://" + address + "/internal/v1/comment-page")
			if err == nil {
				_ = response.Body.Close()
				if response.StatusCode != http.StatusMethodNotAllowed {
					t.Fatalf("bridge route status = %d, want 405", response.StatusCode)
				}
				listening = true
				break
			}
			time.Sleep(25 * time.Millisecond)
		}
		if !listening {
			t.Fatal("same-database worker did not open the configured bridge")
		}
		mrStop(t, p, syscall.SIGTERM, true)
		mrLogNoSecrets(t, p, secrets...)
		waitPoolsGone(t, f, "same-database worker left pools open", poolNames...)
	}) {
		t.Fatal("same-database positive must pass before split-database refusals count")
	}
	for i, name := range names {
		t.Run(name, func(t *testing.T) {
			p := mrLaunch(t, binary, "oa-worker-split-db", environment(i))
			mrFailsBeforeReady(t, p, "claims_worker_database_unavailable", secrets...)
			log := crReadLog(t, p)
			if strings.Contains(log, "claims_worker_routes") || strings.Contains(log, "claims_worker_ready") || strings.Contains(log, "claims_worker_console_ready") {
				t.Fatal("split-database worker crossed startup admission before refusing")
			}
			waitPoolsGone(t, f, "rejected worker left primary pools open", poolNames...)
			waitPoolsGone(t, clone, "rejected worker left clone pools open", poolNames...)
		})
	}
}
