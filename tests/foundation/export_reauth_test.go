// Purpose: FIX-EXPORT-REAUTH regression checks for every Go-rendered merchant CSV download.
// Depends on: real runtime-role HTTP fixtures, identity.complete_oidc_reauth and synthetic export fixtures.
// Used by: TestMerchantToolsCSVBoundaries, TestCarrierExport, TestTrackingImportSmoke and TestCustomerImport.
// Invariants: stale authentication never releases CSV bytes; re-auth preserves the session and file; no CSV bodies are logged.
package foundation_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/merchantorders"
	"livecommerce/internal/platform"
)

// checkExportReauth uses owner access only to age an existing synthetic session and bind a synthetic
// OIDC identity. Freshness is restored by the real re-auth definer, never by a fixture UPDATE.
func checkExportReauth(t *testing.T, f *testFixture, token, store string, request func(string) (int, []byte, http.Header), direct func() error) {
	t.Helper()
	ctx := context.Background()
	hash := sha256.Sum256([]byte(token))
	var principal string
	var original time.Time
	if err := f.owner.QueryRow(ctx, `SELECT principal_id::text,authenticated_at FROM identity.sessions WHERE token_hash=$1`, hash[:]).Scan(&principal, &original); err != nil {
		t.Fatal(err)
	}
	issuer, subject := "https://export-reauth.example.test/"+randomUUID(), "synthetic-export-owner"
	mustExec(t, f.owner, `INSERT INTO identity.external_identities(issuer,subject,principal_id) VALUES($1,$2,$3)`, issuer, subject, principal)
	t.Cleanup(func() {
		mustExec(t, f.owner, `UPDATE identity.sessions SET authenticated_at=$2 WHERE token_hash=$1`, hash[:], original)
		mustExec(t, f.owner, `DELETE FROM identity.external_identities WHERE issuer=$1 AND subject=$2`, issuer, subject)
	})
	age := func(interval string) {
		t.Helper()
		mustExec(t, f.owner, `UPDATE identity.sessions SET authenticated_at=clock_timestamp()-$2::interval WHERE token_hash=$1`, hash[:], interval)
	}
	age("9 minutes")
	status, baseline, headers := request(store)
	if status != http.StatusOK || headers.Get("Content-Type") != "text/csv; charset=utf-8" || !bytes.HasPrefix(baseline, []byte{0xef, 0xbb, 0xbf}) {
		t.Fatalf("nine-minute session: status=%d content-type=%q; want a BOM-prefixed CSV", status, headers.Get("Content-Type"))
	}
	for _, other := range []string{f.storeA2, f.storeB} {
		if status, _, _ := request(other); status != http.StatusForbidden && status != http.StatusNotFound {
			t.Errorf("cross-store export: status=%d, want 403 or 404", status)
		}
	}
	for _, interval := range []string{"11 minutes", "2 hours", "-2 minutes", "infinity", "-infinity"} {
		t.Run("refuse_"+interval, func(t *testing.T) {
			age(interval)
			status, body, headers := request(store)
			var refusal struct {
				Code string `json:"code"`
			}
			decodeErr := json.Unmarshal(body, &refusal)
			if status != http.StatusForbidden || decodeErr != nil || refusal.Code != "reauth_required" {
				t.Errorf("export freshness refusal: status=%d code=%q, want 403 reauth_required", status, refusal.Code)
			}
			if headers.Get("Content-Disposition") != "" || headers.Get("Content-Type") == "text/csv; charset=utf-8" || bytes.HasPrefix(body, []byte{0xef, 0xbb, 0xbf}) {
				t.Error("a refused export released CSV content or attachment headers")
			}
			if direct != nil {
				if err := direct(); !errors.Is(err, platform.ErrReauthRequired) {
					t.Errorf("direct Go export: error=%v, want ErrReauthRequired", err)
				}
			}
		})
	}
	// Same bearer, actual OIDC re-auth completion, then a byte-for-byte comparison of the allowed file.
	var outcome string
	if err := f.owner.QueryRow(ctx, `SELECT outcome FROM identity.complete_oidc_reauth($1,$2,$3,clock_timestamp())`, hash[:], issuer, subject).Scan(&outcome); err != nil || outcome != "REAUTHENTICATED" {
		t.Fatalf("complete_oidc_reauth: outcome=%q error=%v", outcome, err)
	}
	status, after, _ := request(store)
	if status != http.StatusOK || !bytes.Equal(after, baseline) {
		t.Errorf("same-session export after re-auth: status=%d content-unchanged=%t", status, bytes.Equal(after, baseline))
	}
	var live int
	if err := f.owner.QueryRow(ctx, `SELECT count(*) FROM identity.sessions WHERE token_hash=$1 AND revoked_at IS NULL AND expires_at>clock_timestamp()`, hash[:]).Scan(&live); err != nil || live != 1 {
		t.Fatalf("export refusal/re-auth must preserve the live session: count=%d error=%v", live, err)
	}
}

func checkCarrierExportReauth(t *testing.T, e *rfxEnv, order rfxOrder) {
	t.Helper()
	checkExportReauth(t, e.f, order.token(), order.store(), func(store string) (int, []byte, http.Header) {
		return e.call("POST", "/v1/admin/stores/"+store+"/orders/export?template=generic", order.token(), nil, plPickBody([]string{order.order}))
	}, func() error {
		before := e.mfxAudit(t, order, "orders.carrier_export")
		err := platform.WithScope(context.Background(), e.f.runtime, order.token(), order.store(), "orders:export", func(tx pgx.Tx, scope platform.Scope) error {
			_, err := merchantorders.CarrierExport(context.Background(), tx, scope, order.token(), "generic", merchantorders.PickListSelection{OrderIDs: []string{order.order}})
			return err
		})
		if after := e.mfxAudit(t, order, "orders.carrier_export"); after != before {
			t.Error("refused direct carrier export committed an audit row")
		}
		return err
	})
}
