// Purpose: DB-free unit gates of the W6-05B ledger: the closed refusal-code set against the transport message table and against the codes migration 0159 can actually raise,
//   and the state-filter normalisation.
// Depends on: ledger.go, internal/httperror (message table), migrations/0159_operations_ledger.sql (read as text).
// Used by: go test ./internal/integrations/core. Real-PG behaviour is tests/foundation/operations_queue_test.go.
// Invariants: a refusal code without a message would reach the merchant as "internal"; a SQL refusal code missing from refusalCodes would degrade to a plain 409 "conflict".

package core

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"

	"livecommerce/internal/httperror"
)

func TestLedgerRefusalCodesSurviveHTTPError(t *testing.T) {
	for code := range refusalCodes {
		w := httptest.NewRecorder()
		httperror.Write(w, 409, code)
		var e httperror.Envelope
		if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil || e.Code != code {
			t.Errorf("code %q is rewritten to %q by httperror (add it to the message table)", code, e.Code)
		}
	}
}

// Every reason the SQL can return or raise is in refusalCodes, and every refusalCodes entry is produced somewhere in the SQL (no dead or missing code).
func TestLedgerRefusalCodesMatchMigration(t *testing.T) {
	raw, err := os.ReadFile("../../../migrations/0159_operations_ledger.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(raw)
	produced := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?:RETURN|THEN|ELSE|RAISE EXCEPTION) '([a-z_]+)'`).FindAllStringSubmatch(sql, -1) {
		produced[m[1]] = true
	}
	for _, m := range regexp.MustCompile(`RETURN CASE WHEN [^']*'([a-z_]+)' ELSE '([a-z_]+)'`).FindAllStringSubmatch(sql, -1) {
		produced[m[1]], produced[m[2]] = true, true
	}
	// Strings that are not 409 refusals: authentication/validation/lookup (PT401/403/404/422), event reasons and the lane names.
	for _, other := range []string{"unauthorized", "forbidden", "not_found", "invalid_request", "invalid_job", "retry_authorized", "requeue_authorized", "default", "ads"} {
		delete(produced, other)
	}
	for code := range produced {
		if !refusalCodes[code] {
			t.Errorf("migration 0159 can raise %q but it is not in refusalCodes", code)
		}
	}
	for code := range refusalCodes {
		if !produced[code] {
			t.Errorf("refusalCodes has %q which migration 0159 never produces", code)
		}
	}
}

func TestNormalizeLedgerState(t *testing.T) {
	for in, want := range map[string]string{"": "attention", "attention": "attention", "FAILED": "FAILED_FINAL", "FAILED_FINAL": "FAILED_FINAL", "UNKNOWN": "UNKNOWN", "READY": "READY"} {
		if got, ok := normalizeLedgerState(in); !ok || got != want {
			t.Errorf("%q -> %q %v, want %q", in, got, ok, want)
		}
	}
	for _, bad := range []string{"SUCCEEDED", "CANCELLED", "DISPATCHING", "all", "unknown", "FAILED_FINAL ", "x'; DROP"} {
		if _, ok := normalizeLedgerState(bad); ok {
			t.Errorf("%q accepted", bad)
		}
	}
}
