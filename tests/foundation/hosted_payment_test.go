package foundation_test

// Purpose: hosted payment harness (hpHarness, hpSetup) and PAYUNi hosted tests, partly refusal gates after phase 2b.
// Depends on: ps fixtures, accounts keyring, checkout hosted starter, p2bHostedRetired.
// Used by: go test ./tests/foundation; hpHarness used by buyer view and phase-2b helpers.
// Invariants: synthetic merchant and local key material only; no provider form is posted. PAY-RM2 phase 2b (0197, post_river/0026): see output/pay-rm2-p2b/test-contract-map.md.

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"livecommerce/internal/checkout"
	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/platform"
)

// These fixtures use a disposable PG database, a synthetic merchant and local
// cryptographic material. No test posts a provider form or logs its contents.
type hpAPI interface {
	BeginHosted(context.Context, string, string, string, checkout.HostedInput) (checkout.PaymentResult, error)
	TakeHosted(context.Context, string, string, string) (checkout.HostedHandoff, error)
}

type hpHarness struct {
	psHarness
	pool      *pgxpool.Pool
	api       hpAPI
	key       []byte
	config    checkout.HostedConfig
	input     checkout.HostedInput
	secretKey string
	secretIV  string
}

func hpSetup(t *testing.T) hpHarness {
	t.Helper()
	p := psSetup(t)
	key := randomBytes(32)
	secretKey := strings.Repeat("K", 32)
	secretIV := strings.Repeat("V", 16)
	// psSetup deliberately seeds an unreadable historical credential for its
	// query-only tests. Replace that row in this disposable fixture with a real
	// envelope using exactly the production AAD shape.
	aad, err := json.Marshal(struct {
		FormatVersion     int    `json:"format_version"`
		TenantID          string `json:"tenant_id"`
		StoreID           string `json:"store_id"`
		ConnectionID      string `json:"connection_id"`
		Provider          string `json:"provider"`
		Environment       string `json:"environment"`
		AccountID         string `json:"account_id"`
		CredentialVersion int64  `json:"credential_version"`
	}{1, p.f.tenantA, p.f.storeA1, p.account, "payuni", "SANDBOX", "mock-account", 1})
	if err != nil {
		t.Fatal(err)
	}
	plain, err := json.Marshal(struct {
		HashKey string `json:"hash_key"`
		HashIV  string `json:"hash_iv"`
	}{secretKey, secretIV})
	if err != nil {
		t.Fatal(err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	nonce := randomBytes(gcm.NonceSize())
	sealed := gcm.Seal(nil, nonce, plain, aad)
	mustExec(t, p.f.owner, `UPDATE integration.account_credentials SET key_id='hosted_fixture',nonce=$1,ciphertext=$2 WHERE connection_id=$3 AND version=1`, nonce, sealed, p.account)
	keys, err := accounts.NewKeyring("hosted_fixture", map[string][]byte{"hosted_fixture": key}, randomBytes(32))
	if err != nil {
		t.Fatal(err)
	}
	pool, err := platform.OpenHostedPool(context.Background(), hpRole(t, p.f))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	config := checkout.HostedConfig{ReturnURL: "https://checkout.example.test/payment/return", NotifyURL: "https://checkout.example.test/payment/notify"}
	api := hpStarter(t, pool, "PROVIDER_MOCK", keys, config)
	return hpHarness{psHarness: p, pool: pool, api: api, key: key, config: config,
		input:     checkout.HostedInput{OrderID: p.hold.OrderID, MethodCode: "payuni_credit", MethodVersion: 1, Locale: "zh-TW"},
		secretKey: secretKey, secretIV: secretIV}
}

func hpRole(t *testing.T, f *testFixture) string {
	t.Helper()
	role := "hosted_test_" + hex.EncodeToString(randomBytes(6))
	password := hex.EncodeToString(randomBytes(24))
	quoted := pgx.Identifier{role}.Sanitize()
	mustExec(t, f.owner, fmt.Sprintf(`CREATE ROLE %s LOGIN INHERIT NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION PASSWORD '%s'`, quoted, password))
	mustExec(t, f.owner, `GRANT commerce_hosted_runtime TO `+quoted+` WITH INHERIT TRUE, SET FALSE`)
	t.Cleanup(func() { mustExec(t, f.owner, `DROP ROLE `+quoted) })
	return roleURL(t, f.databaseURL, role, password)
}

func hpStarter(t *testing.T, pool *pgxpool.Pool, profile string, keys *accounts.Keyring, config checkout.HostedConfig) hpAPI {
	t.Helper()
	jobs, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Schema: "river_payment"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := checkout.NewHostedPaymentStarter(context.Background(), pool, jobs, profile, keys, config)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func (h hpHarness) begin(key string) (checkout.PaymentResult, error) {
	return h.api.BeginHosted(context.Background(), h.cap.Token, h.f.storeA1, key, h.input)
}

func (h hpHarness) take() (checkout.HostedHandoff, error) {
	return h.api.TakeHosted(context.Background(), h.cap.Token, h.f.storeA1, h.hold.OrderID)
}

func (h hpHarness) counts(t *testing.T) (out [8]int64) {
	t.Helper()
	err := h.f.owner.QueryRow(context.Background(), `SELECT
	 (SELECT count(*) FROM checkout.payment_attempts WHERE owner_id=$1),
	 (SELECT count(*) FROM integration.operations WHERE buyer_owner_id=$1),
	 (SELECT count(*) FROM checkout.command_results WHERE owner_id=$1),
	 (SELECT count(*) FROM checkout.events WHERE owner_id=$1 AND action='checkout.payment_started'),
	 (SELECT count(*) FROM checkout.hosted_payment_pages p JOIN checkout.payment_attempts a ON a.id=p.attempt_id WHERE a.owner_id=$1),
	 (SELECT count(*) FROM checkout.orders WHERE owner_id=$1 AND commercial_state='AWAITING_PAYMENT'),
	 (SELECT count(*) FROM inventory.reservations WHERE buyer_owner_id=$1 AND state='PAYMENT_PENDING'),
	 (SELECT count(*) FROM river_payment.river_job WHERE kind='payment_query_v1')`, h.cap.Scope.OwnerID).
		Scan(&out[0], &out[1], &out[2], &out[3], &out[4], &out[5], &out[6], &out[7])
	if err != nil {
		t.Fatal(err)
	}
	return
}

func (h hpHarness) page(t *testing.T, attemptID string) (form []byte, preparedAt, expiresAt time.Time, handedOutAt *time.Time) {
	t.Helper()
	err := h.f.owner.QueryRow(context.Background(), `SELECT form,prepared_at,expires_at,handed_out_at FROM checkout.hosted_payment_pages WHERE attempt_id=$1`, attemptID).
		Scan(&form, &preparedAt, &expiresAt, &handedOutAt)
	if err != nil {
		t.Fatal(err)
	}
	return
}

func hpHandoffJSON(t *testing.T, out checkout.HostedHandoff) map[string]json.RawMessage {
	t.Helper()
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil {
		t.Fatal(err)
	}
	return fields
}

func hpDisposition(t *testing.T, out checkout.HostedHandoff) string {
	t.Helper()
	var value string
	if err := json.Unmarshal(hpHandoffJSON(t, out)["disposition"], &value); err != nil {
		t.Fatal("handoff disposition missing")
	}
	return value
}

func hpForm(t *testing.T, out checkout.HostedHandoff) (action string, fields map[string]string) {
	t.Helper()
	var form struct {
		Action string            `json:"action"`
		Fields map[string]string `json:"fields"`
	}
	if err := json.Unmarshal(hpHandoffJSON(t, out)["form"], &form); err != nil || form.Action == "" || len(form.Fields) != 4 {
		t.Fatal("issued handoff must contain one bounded scalar provider form")
	}
	return form.Action, form.Fields
}

func hpNoForm(t *testing.T, out checkout.HostedHandoff) {
	t.Helper()
	form := hpHandoffJSON(t, out)["form"]
	if len(form) != 0 && string(form) != "null" {
		t.Fatal("repeated handoff exposed a provider form")
	}
}

func TestBuyerPaymentHostedAtomicPreparationAndIndependentWire(t *testing.T) {
	rm2AssertRemovedHosted(t, "start", "zh-TW", false)
}

func TestBuyerPaymentHostedReplayConflictsAndSingleAttempt(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bHostedRetired(t)
}

func TestBuyerPaymentHostedConcurrentTakeOneForm(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bHostedRetired(t)
}

func TestBuyerPaymentHostedCryptoFailureRollsBack(t *testing.T) {
	rm2AssertRemovedHosted(t, "start", "zh-TW", true)
}

func TestBuyerPaymentHostedThreeFrozenLocales(t *testing.T) {
	for _, locale := range []string{"zh-CN", "zh-TW", "en"} {
		t.Run(locale, func(t *testing.T) { rm2AssertRemovedHosted(t, "start", locale, false) })
	}
}

func TestBuyerPaymentHostedAdmissionDriftBeforePrepare(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bHostedRetired(t)
}

func hpRotateFixtureHead(t *testing.T, h hpHarness) {
	t.Helper()
	mustExec(t, h.f.owner, `INSERT INTO integration.account_credentials(tenant_id,store_id,connection_id,version,key_id,nonce,ciphertext,principal_id)
	 SELECT tenant_id,store_id,connection_id,2,key_id,nonce,ciphertext,principal_id
	 FROM integration.account_credentials WHERE connection_id=$1 AND version=1`, h.account)
	mustExec(t, h.f.owner, `UPDATE integration.merchant_accounts SET credential_version=2 WHERE id=$1`, h.account)
}

func TestBuyerPaymentHostedSQLAtomicPreparationAndOneShot(t *testing.T) {
	// PAY-RM2 phase 2b (0197, post_river/0026) dropped the PAYUNi SQL this test drove; mapped (a)+(b) in
	// output/pay-rm2-p2b/test-contract-map.md: the dropped path is refused and writes nothing, and the
	// surviving invariant is asserted by the Stripe test named there.
	p2bHostedRetired(t)
}

// Independent verifier: it does not call PAYUNi Client.open or the hosted
// keyring. It checks both outer authentication and every frozen inner field.
