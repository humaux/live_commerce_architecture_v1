//go:build browser

// Purpose: real-click admin tracking backfill through production Next, Go and isolated PostgreSQL.
// Depends on: brf signed-login/Next harness, rfx paid HOME orders and tracking-import HTTP commands.
// Used by: scripts/dev/test-local.sh --browser-tracking-backfill; no LIVE carrier or mail acceptance.
package foundation_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestBrowserTrackingBackfill(t *testing.T) {
	if os.Getenv("LC_BROWSER_TRACKING_BACKFILL") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-tracking-backfill")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 9*time.Minute)
	defer cancel()
	e := rfxNew(t)
	e.startWorker(t)
	base := e.storeFor(t)
	orders := map[string]rfxOrder{}
	ids := map[string]string{}
	for _, name := range []string{"tw-wide-a", "tw-wide-b", "tw-phone-a", "tw-phone-b", "cn-wide-a", "cn-wide-b", "cn-phone-a", "cn-phone-b", "en-wide-a", "en-wide-b", "en-phone-a", "en-phone-b", "stale-a", "stale-b", "lost"} {
		o := e.payMore(t, base)
		e.await(t, "capture settled", o.attempt, 45*time.Second, `SELECT NOT EXISTS(SELECT 1 FROM river_payment.river_job WHERE args->>'operation_id'=$1 AND state NOT IN ('completed','cancelled','discarded'))`)
		if got := e.mfxFulfilmentState(t, o); got != "MANUAL_UNASSIGNED" {
			t.Fatalf("synthetic HOME order not ready: %s", got)
		}
		orders[name], ids[name] = o, o.order
	}
	foreign := e.storeFor(t)
	ids["foreign"] = foreign.order
	_, viewerPrincipal := e.member(t, base, "orders:read")
	controlKey := randomToken()
	var observationsMu sync.Mutex
	type observation struct {
		Path  string `json:"path"`
		Hash  string `json:"hash"`
		Count string `json:"count"`
		Bytes int    `json:"bytes"`
	}
	observations := []observation{}
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// FIXTURE/FAULT only: advance one synthetic shipment, or revoke/grant the synthetic owner's write permission.
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodPost || r.Header.Get("X-Tracking-Test") != controlKey {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		var in struct{ Action string }
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&in) != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		switch in.Action {
		case "requests":
			observationsMu.Lock()
			snapshot := append([]observation{}, observations...)
			observationsMu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(snapshot)
			return
		case "ship-stale":
			e.mustShip(t, orders["stale-a"], 0, "black_cat", "STALE001")
		case "assert-stale-unshipped":
			if e.mfxFulfilmentState(t, orders["stale-b"]) != "MANUAL_UNASSIGNED" || e.mfxVersions(t, orders["stale-b"]) != 0 {
				t.Error("stale confirmation wrote the second order before fresh explicit confirmation")
				w.WriteHeader(http.StatusConflict)
				return
			}
		default:
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(control.Close)
	evidenceRoot := os.Getenv("LC_TRACKING_EVIDENCE_ROOT")
	if evidenceRoot == "" {
		t.Fatal("persistent main-output LC_TRACKING_EVIDENCE_ROOT required")
	}
	evidence := filepath.Join(evidenceRoot, time.Now().UTC().Format("20060102T150405.000000000"))
	if err := os.MkdirAll(evidence, 0o700); err != nil {
		t.Fatal(err)
	}
	stack := brfStartAdmin(t, ctx, e, base.s.p.f.principalA, evidence, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// OBSERVE real HTTP bytes before the unchanged product handler; never persist CSV or buyer values.
			if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/shipments/tracking-import/") {
				data, err := io.ReadAll(io.LimitReader(r.Body, 2*1024*1024+1))
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				_ = r.Body.Close()
				r.Body = io.NopCloser(bytes.NewReader(data))
				sum := sha256.Sum256(data)
				observationsMu.Lock()
				observations = append(observations, observation{Path: r.URL.Path, Hash: hex.EncodeToString(sum[:]), Count: r.URL.Query().Get("expected_apply_rows"), Bytes: len(data)})
				observationsMu.Unlock()
			}
			next.ServeHTTP(w, r)
		})
	})
	viewerEvidence := filepath.Join(evidence, "viewer")
	if err := os.MkdirAll(viewerEvidence, 0o700); err != nil {
		t.Fatal(err)
	}
	viewerStack := brfStartAdmin(t, ctx, e, viewerPrincipal, viewerEvidence)
	t.Cleanup(func() {
		observationsMu.Lock()
		wire, err := json.MarshalIndent(observations, "", "  ")
		observationsMu.Unlock()
		if err == nil {
			err = os.WriteFile(filepath.Join(evidence, "request-bindings.json"), wire, 0o600)
		}
		if err != nil {
			t.Errorf("preserve request binding evidence: %v", err)
		}
	})
	// The config must resolve Playwright from THIS worktree, while durable artifacts remain in main output.
	configDir, err := os.MkdirTemp(filepath.Join(stack.root, "output"), "tracking-playwright-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		err := filepath.WalkDir(configDir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			rel, _ := filepath.Rel(configDir, path)
			dest := filepath.Join(evidence, "runner", rel)
			if d.IsDir() {
				return os.MkdirAll(dest, 0o700)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(dest, data, 0o600)
		})
		if err != nil {
			t.Errorf("preserve runner evidence: %v", err)
			return
		}
		if _, err := os.Stat(filepath.Join(configDir, "KEEP")); os.IsNotExist(err) {
			_ = os.RemoveAll(configDir)
		}
	})
	runner := *stack
	runner.evidence = configDir
	// Audit rows have no order link; validate the store-level delta once, not once per order.
	auditsBefore := e.mfxAudit(t, base, "fulfillment.shipment_recorded")
	encoded, _ := json.Marshal(ids)
	brfPlaywright(t, ctx, &runner, []string{"tracking-backfill.spec.ts"}, map[string]string{
		"LC_BROWSER_EVIDENCE":    evidence,
		"LC_TRACKING_PROBE_LOST": os.Getenv("LC_TRACKING_PROBE_LOST"),
		"LC_TRACKING_STORE":      base.store(), "LC_TRACKING_IDS": string(encoded), "LC_TRACKING_VIEWER_ORIGIN": viewerStack.origin,
		"LC_TRACKING_CONTROL": control.URL, "LC_TRACKING_CONTROL_KEY": controlKey,
	})
	for name, o := range orders {
		if got := e.mfxFulfilmentState(t, o); got != "MERCHANT_SHIPPED" {
			t.Errorf("%s expected persisted MERCHANT_SHIPPED, got %s", name, got)
		}
		if got := e.mfxVersions(t, o); got != 1 {
			t.Errorf("%s expected one shipment version, got %d", name, got)
		}
		var mails int
		if err := e.f.owner.QueryRow(ctx, `SELECT count(*) FROM notify.outbox WHERE order_id=$1 AND kind='shipped'`, o.order).Scan(&mails); err != nil || mails != 1 {
			t.Errorf("%s expected one shipped outbox, count=%d err=%v", name, mails, err)
		}
	}
	if got := e.mfxAudit(t, base, "fulfillment.shipment_recorded") - auditsBefore; got != len(orders) {
		t.Errorf("expected %d shipment audit rows for the store, got %d", len(orders), got)
	}
	if !t.Failed() {
		t.Logf("PASS tracking backfill BROWSER MOCK + REAL_PG: 15 single-version shipments and store audit rows; evidence=%s", evidence)
	}
}
