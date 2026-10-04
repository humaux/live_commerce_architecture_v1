package capiroute

// Independent AT2: a refusal at the new data boundary remains a policy denial.
import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/attribution"
	"livecommerce/internal/integrations/core"
)

type atRow func(...any) error

func (r atRow) Scan(dst ...any) error { return r(dst...) }

type atTx struct {
	pgx.Tx
	calls    []string
	claims   [][]any
	finalErr error
}

func (tx *atTx) QueryRow(_ context.Context, q string, args ...any) pgx.Row {
	tx.calls = append(tx.calls, q)
	tx.claims = append(tx.claims, args)
	switch len(tx.calls) {
	case 1:
		return atRow(func(dst ...any) error {
			tr := goodToken()
			v := []any{tr.tenant, tr.store, tr.binding, tr.provider, tr.asset, tr.version, tr.keyID, tr.nonce, tr.ciphertext, tr.scopes}
			for i := range dst {
				reflect.ValueOf(dst[i]).Elem().Set(reflect.ValueOf(v[i]))
			}
			return nil
		})
	case 2:
		return atRow(func(dst ...any) error {
			ur := goodUser()
			v := []any{ur.phone, ur.owner, ur.contents, ur.valueMinor, ur.currency, ur.sourceURL, ur.agent}
			for i := range dst {
				reflect.ValueOf(dst[i]).Elem().Set(reflect.ValueOf(v[i]))
			}
			return nil
		})
	default:
		return atRow(func(...any) error { return tx.finalErr })
	}
}
func TestAdsAttributionAT2FinalConsentBoundary(t *testing.T) {
	r, f := newFake(t, func(w http.ResponseWriter) { w.Write([]byte(`{"events_received":1}`)) })
	claim := core.SecretClaim{OperationID: testAttempt, Generation: 7, LeaseToken: []byte(strings.Repeat("x", 32))}
	tx := &atTx{finalErr: pgx.ErrNoRows}
	sec, err := r.loadSecret(context.Background(), tx, claim)
	if !errors.Is(err, core.ErrPolicyDenied) || len(sec.Reveal()) != 0 {
		t.Fatalf("last-boundary refusal: %v secret bytes=%d", err, len(sec.Reveal()))
	}
	if len(tx.calls) != 3 || !strings.Contains(tx.calls[2], "ads.capi_attribution_data") {
		t.Fatalf("new consent boundary never checked: %v", tx.calls)
	}
	for _, args := range tx.claims {
		if !reflect.DeepEqual(args, []any{claim.OperationID, claim.Generation, claim.LeaseToken}) {
			t.Fatalf("lease changed: %v", args)
		}
	}
	if len(f.calls) != 0 {
		t.Fatal("refused match context caused HTTP")
	}
}

func TestAdsAttributionAT2MatchFieldsWire(t *testing.T) {
	r, f := newFake(t, func(w http.ResponseWriter) { w.Write([]byte(`{"events_received":1,"fbtrace_id":"Synthetic"}`)) })
	ur := goodUser()
	fbc, fbp, ip, email := "fb.1.1790000000000.synthetic", "fb.1.1790000000000.456", "192.0.2.8", attribution.HashEmail("buyer@example.test")
	ur.fbc, ur.fbp, ur.clientIP, ur.emailHash = &fbc, &fbp, &ip, &email
	sec, err := r.assemble(goodToken(), ur)
	if err != nil {
		t.Fatal(err)
	}
	out, err := r.dispatch(context.Background(), dreq(goodRequest("")), sec)
	if err != nil || out.State != "SUCCEEDED" || len(f.calls) != 1 {
		t.Fatalf("dispatch %v %v calls=%d", out, err, len(f.calls))
	}
	ev := f.calls[0]["data"].([]any)[0].(map[string]any)
	ud := ev["user_data"].(map[string]any)
	if ev["event_id"] != attribution.EventID(testAttempt) || ud["fbc"] != fbc || ud["fbp"] != fbp || ud["client_ip_address"] != ip || ud["em"].([]any)[0] != attribution.HashEmail("buyer@example.test") {
		t.Fatalf("wire=%v", ev)
	}
	if !reflect.DeepEqual(keys(ud), []string{"client_ip_address", "client_user_agent", "em", "external_id", "fbc", "fbp"}) {
		t.Fatalf("wire keys %v", keys(ud))
	}
}
