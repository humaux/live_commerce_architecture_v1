//go:build browser

// Purpose: revoke and restore only the owned merchant inbox:reply grant for completed-order recovery clicks.
// Depends on: drawerPartialFault exact-key PG readback; product A16 and A16R remain real.
// Used by: runCreateOrderDrawerBrowser's eighth signed-ingress fixture; no production grants are touched.
// Invariants: I01/I02/I06/I11; bounded controls/readback emit only counts and restore the exact existing grant.
package foundation_test

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"
)

// newDrawerCapabilityControl reuses exact-key observations without installing another PG fault trigger.
func newDrawerCapabilityControl(t *testing.T, e *lbuEnv, bundle string, openSend func(string, string, string, []byte, []byte) ([]byte, error)) *drawerPartialFault {
	t.Helper()
	f := &drawerPartialFault{e: e, bundle: bundle, openSend: openSend, recoveryCount: 1, evidenceName: "capability-pg-readback.json"}
	t.Cleanup(func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.replyRevoked {
			_, err := e.p.f.owner.Exec(context.Background(), `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'inbox:reply') ON CONFLICT DO NOTHING`, e.tenant(), e.store(), e.p.f.principalA)
			if err != nil {
				t.Error("capability grant cleanup failed")
			}
			f.replyRevoked = false
		}
	})
	return f
}

// handleCapability provides runner-only controls; actual merchant submissions and recovery are browser clicks.
func (f *drawerPartialFault) handleCapability(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != "POST" || !strings.HasPrefix(r.URL.Path, "/__test/drawer-capability/") {
		return false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	w.Header().Set("Cache-Control", "no-store")
	var input struct {
		Key string `json:"key"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256))
	dec.DisallowUnknownFields()
	if dec.Decode(&input) != nil {
		http.Error(w, "invalid fixture control", 400)
		return true
	}
	e := f.e
	var err error
	switch strings.TrimPrefix(r.URL.Path, "/__test/drawer-capability/") {
	case "arm":
		if f.auditBase != nil || input.Key != "" {
			http.Error(w, "already armed", 409)
			return true
		}
		f.auditBase, err = f.audits(ctx)
	case "facts":
		if f.auditBase == nil || !regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`).MatchString(input.Key) || (f.key != "" && f.key != input.Key) {
			http.Error(w, "invalid observation", 400)
			return true
		}
		f.key = input.Key
		facts, inner := f.facts(ctx, input.Key)
		if inner != nil {
			http.Error(w, "observation failed", 500)
			return true
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(facts)
		return true
	case "revoke":
		if f.auditBase == nil || f.key == "" || f.replyRevoked {
			http.Error(w, "invalid grant state", 409)
			return true
		}
		result, inner := e.p.f.owner.Exec(ctx, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='inbox:reply'`, e.tenant(), e.store(), e.p.f.principalA)
		err = inner
		if err == nil && result.RowsAffected() != 1 {
			http.Error(w, "expected exactly one grant", 409)
			return true
		}
		if err == nil {
			f.replyRevoked = true
		}
	case "restore":
		if f.replyRevoked {
			_, err = e.p.f.owner.Exec(ctx, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'inbox:reply') ON CONFLICT DO NOTHING`, e.tenant(), e.store(), e.p.f.principalA)
			if err == nil {
				f.replyRevoked = false
			}
		}
	default:
		http.NotFound(w, r)
		return true
	}
	if err != nil {
		http.Error(w, "capability fixture failed", 500)
		return true
	}
	var grants int
	if err = e.p.f.owner.QueryRow(ctx, `SELECT count(*) FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='inbox:reply'`, e.tenant(), e.store(), e.p.f.principalA).Scan(&grants); err != nil {
		http.Error(w, "grant observation failed", 500)
		return true
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]int{"reply_grants": grants})
	return true
}

// checkCapabilityLedger requires the real submit, recovery and persisted-order controls in the new case.
func checkCapabilityLedger(t *testing.T, ledger []struct{ Case, Control, Status string }, evidence string) {
	t.Helper()
	for _, control := range []string{"capability lost completed reply", "capability A16R recovery", "capability recovered order persistence"} {
		matches := 0
		for _, row := range ledger {
			if row.Case == "DU3-capability-after-unknown-recovery" && row.Control == control && row.Status == "PASS" {
				matches++
			}
		}
		if matches != 1 {
			t.Fatalf("capability case requires exactly one %s click verdict, got %d; evidence=%s", control, matches, evidence)
		}
	}
}
