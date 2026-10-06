// payuni_activation_test.go: REAL_PG gate for the PAYUNi merchant self-serve activation (w4-02b, PA01-PA08 + the 0137 ACL pins).
// It runs the real merchant pool, the real payment-worker login (sweep/issue), the real notify ingress handler and the real
// activation service over a MOCK PAYUNi transport: NT$1 verification (query CAPTURED AND a signed notify receipt -> PROVIDER_MOCK
// qualification -> enabled method), the LIVE probe (signature-verified "no trade" -> REAL_LIVE), rotation, the platform switch
// and the forgery / cross-store / ACL matrix. No provider host is contacted, no live key exists.
//
// Runs as/in: pmSetup fixture (shared REAL_PG container), MOCK transport. Status: MOCK; SANDBOX and LIVE are NOT_RUN.
// Depends on: payment_methods_test.go (pmSetup), payuni_notify_test.go (pnSignedBody/pnPost), stripe_authority_test.go (saNewLogin),
// external_operation_authority_test.go (t06AuthorityLogin). Used by: bash scripts/dev/test-focused.sh '^TestPayuniActivation'.
package foundation_test

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/command"
	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/integrations/psp/payuni"
	"livecommerce/internal/payments"
	"livecommerce/internal/payments/payuninotify"
	"livecommerce/internal/platform"
)

// paMock is the MOCK PAYUNi query transport: the test sets the next reply (or error) before each call.
type paMock struct {
	mu    sync.Mutex
	body  string
	err   error
	block bool
	calls int
}

func (m *paMock) set(body string, err error, block bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.body, m.err, m.block = body, err, block
}

func (m *paMock) RoundTrip(r *http.Request) (*http.Response, error) {
	m.mu.Lock()
	body, err, block := m.body, m.err, m.block
	m.calls++
	m.mu.Unlock()
	if block {
		<-r.Context().Done()
		return nil, r.Context().Err()
	}
	if err != nil {
		return nil, err
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
}

func (m *paMock) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// paReply builds a query reply exactly as PAYUNi signs it with creds (AES-256-GCM, nonce=HashIV, HashInfo=sha256 upper hex).
func paReply(t *testing.T, creds accounts.Credentials, merID, outerStatus string, inner url.Values) string {
	t.Helper()
	block, err := aes.NewCipher([]byte(creds.HashKey))
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCMWithNonceSize(block, 16)
	if err != nil {
		t.Fatal(err)
	}
	sealed := aead.Seal(nil, []byte(creds.HashIV), []byte(inner.Encode()), nil)
	ct, tag := sealed[:len(sealed)-aead.Overhead()], sealed[len(sealed)-aead.Overhead():]
	encryptInfo := hex.EncodeToString([]byte(base64.StdEncoding.EncodeToString(ct) + ":::" + base64.StdEncoding.EncodeToString(tag)))
	sum := sha256.Sum256([]byte(creds.HashKey + encryptInfo + creds.HashIV))
	out, _ := json.Marshal(map[string]string{"Status": outerStatus, "MerID": merID, "Version": "2.0",
		"EncryptInfo": encryptInfo, "HashInfo": strings.ToUpper(hex.EncodeToString(sum[:]))})
	return string(out)
}

// paTrade is one authenticated query row; captured=true is a full CAPTURED credit trade of NT$1.
func paTradeReply(t *testing.T, creds accounts.Credentials, merID, tradeNo string, captured bool) string {
	t.Helper()
	row := map[string]string{"MerID": merID, "MerTradeNo": tradeNo, "TradeNo": "PT_" + t04Tag(), "TradeAmt": "1", "TradeStatus": "1",
		"PaymentType": "1", "Gateway": "2", "AuthType": "1", "DataSource": "A", "CloseStatus": "2", "CloseAmt": "1"}
	if !captured {
		row["CloseStatus"], row["CloseAmt"] = "", ""
	}
	inner := url.Values{"Status": {"SUCCESS"}}
	for k, v := range row {
		inner.Set("Result[0]["+k+"]", v)
	}
	return paReply(t, creds, merID, "SUCCESS", inner)
}

type paFixture struct {
	*pmFixture
	keys    *accounts.Keyring
	conn    accounts.Connection
	creds   accounts.Credentials
	account string
	act     *payments.Activation
	mock    *paMock
	ingress http.Handler
	worker  *pgxpool.Pool
}

// paSetup builds a TWD market, one real payuni connection (SANDBOX or LIVE) created through accounts.Service, the activation
// service over the MOCK transport, the real notify ingress handler and a real commerce_payment_worker login.
func paSetup(t *testing.T, environment, profile string) *paFixture {
	t.Helper()
	pm := pmSetup(t)
	f := pm.m.f
	keys, err := accounts.NewKeyring("fixture_v1", pm.m.keys, pm.m.replayKey)
	if err != nil {
		t.Fatal(err)
	}
	in := maInput()
	in.Environment = environment
	conn, err := pm.m.create(f.token, f.store, t04Key("pa-account"), in)
	if err != nil {
		t.Fatal(err)
	}
	pm.in.ConnectionID, pm.in.BindingVersion, pm.in.Environment = conn.ID, conn.BindingVersion, environment
	p := &paFixture{pmFixture: pm, keys: keys, conn: conn, creds: in.Credentials, account: in.AccountID, mock: &paMock{}}
	p.act, err = payments.NewActivation(keys, payments.ActivationConfig{Profile: profile,
		NotifyBaseURL: "https://hooks.example.com", ReturnURL: "https://admin.example.com/settings/payments",
		QueryClient: func(c payuni.Config) (*payuni.Client, error) {
			c.Environment = "SANDBOX" // the MOCK transport is only admitted for SANDBOX; a LIVE account's probe is simulated the same way
			return payuni.NewQuery(c, p.mock)
		}})
	if err != nil {
		t.Fatal(err)
	}
	login := saNewLogin(t, f.base, "commerce_payuni_ingress")
	pool, err := platform.OpenPayuniIngressPool(context.Background(), login.dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	inbox, err := payuninotify.NewInbox(context.Background(), pool, keys, "PROVIDER_MOCK")
	if err != nil {
		t.Fatal(err)
	}
	if p.ingress, err = payuninotify.NewHandler(inbox); err != nil {
		t.Fatal(err)
	}
	_, p.worker = t06AuthorityLogin(t, waPayment)
	payments.SetPayuniEnabled(true)
	t.Cleanup(func() {
		payments.SetPayuniEnabled(false)
		// Registered after pmSetup/maSetup, so it runs first: remove what 0137 and the qualification FKs hang on the connection.
		for _, q := range []string{`DELETE FROM payments.method_heads WHERE tenant_id=$1`, `DELETE FROM payments.method_versions WHERE tenant_id=$1`,
			`DELETE FROM payments.payuni_verifications WHERE tenant_id=$1`, `DELETE FROM payments.payuni_live_probes WHERE tenant_id=$1`,
			`DELETE FROM payments.payuni_notify_receipts WHERE endpoint_id IN (SELECT endpoint_id FROM payments.payuni_notify_endpoints WHERE tenant_id=$1)`,
			`DELETE FROM payments.payuni_notify_endpoints WHERE tenant_id=$1`, `DELETE FROM payments.account_qualifications WHERE tenant_id=$1`} {
			mustExec(t, f.base.owner, q, f.tenant)
		}
	})
	return p
}

func (p *paFixture) start(t *testing.T, key string) (payments.VerificationStart, error) {
	t.Helper()
	f := p.m.f
	var out payments.VerificationStart
	err := f.scoped(context.Background(), f.token, f.store, func(tx pgx.Tx, s platform.Scope) error {
		var e error
		out, e = p.act.Start(context.Background(), tx, s, f.token, key, p.conn.ID)
		return e
	})
	return out, err
}

func (p *paFixture) status(t *testing.T) payments.ActivationStatus {
	t.Helper()
	f := p.m.f
	var out payments.ActivationStatus
	if err := f.scoped(context.Background(), f.token, f.store, func(tx pgx.Tx, s platform.Scope) error {
		var e error
		out, e = p.act.Status(context.Background(), tx, s, f.token, p.conn.ID)
		return e
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

func (p *paFixture) check(verificationID string) (payments.CheckResult, error) {
	f := p.m.f
	return p.act.Check(context.Background(), f.base.runtime, f.token, f.store, p.conn.ID, verificationID)
}

// notify posts the signed NotifyURL callback PAYUNi would send for the verification trade through the real ingress handler.
func (p *paFixture) notify(t *testing.T, tradeNo, status string, amountTWD int64) int {
	t.Helper()
	token, _, err := p.keys.PayuniVerifyNotifyToken(p.conn.ID, "PROVIDER_MOCK")
	if err != nil {
		t.Fatal(err)
	}
	body := pnSignedBody(t, p.creds, p.account, tradeNo, "NT_"+t04Tag(), amountTWD, status, "1")
	return pnPost(t, p.ingress, token, string(body)).Code
}

func (p *paFixture) sweep(t *testing.T) int {
	t.Helper()
	n, err := payments.SweepVerifications(context.Background(), p.worker)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func (p *paFixture) qualifications(t *testing.T) int {
	t.Helper()
	return countRows(t, p.m.f.base.owner, `SELECT count(*) FROM payments.account_qualifications WHERE tenant_id=$1 AND connection_id=$2`, p.m.f.tenant, p.conn.ID)
}

func (p *paFixture) enable(t *testing.T, key string, expected int64) (payments.Method, error) {
	t.Helper()
	in := p.in
	in.ExpectedVersion, in.Enabled = expected, true
	return p.set(p.m.f.token, p.m.f.store, key, in)
}

// qualify runs the whole SANDBOX/MOCK verification (query CAPTURED + signed notify + worker sweep) and returns the verification.
func (p *paFixture) qualify(t *testing.T) payments.VerificationStart {
	t.Helper()
	v, err := p.start(t, t04Key("pa-verify"))
	if err != nil {
		t.Fatal(err)
	}
	p.mock.set(paTradeReply(t, p.creds, p.account, v.MerchantTradeNo, true), nil, false)
	if res, err := p.check(v.VerificationID); err != nil || res.Outcome != "CAPTURED" {
		t.Fatalf("check: %+v %v", res, err)
	}
	if code := p.notify(t, v.MerchantTradeNo, "SUCCESS", 1); code != 200 {
		t.Fatalf("notify status %d", code)
	}
	if n := p.sweep(t); n != 1 {
		t.Fatalf("sweep issued %d, want 1", n)
	}
	return v
}

// PA02 (+PA03 order, PA01 inside): the verification state machine and the enable gate.
func TestPayuniActivationPA02VerifiedMockQualifiesAndEnables(t *testing.T) {
	p := paSetup(t, "SANDBOX", "PROVIDER_MOCK")
	f := p.m.f
	if _, err := p.set(f.token, f.store, t04Key("pa-draft"), p.in); err != nil {
		t.Fatal(err)
	}
	// PA01: no qualification => 409 not_qualified, nothing written.
	versionsBefore := countRows(t, f.base.owner, `SELECT count(*) FROM payments.method_versions WHERE tenant_id=$1`, f.tenant)
	if _, err := p.enable(t, t04Key("pa-enable-none"), 1); !errors.Is(err, payments.ErrNotQualified) {
		t.Fatalf("PA01 enable without qualification: %v", err)
	}
	if n := countRows(t, f.base.owner, `SELECT count(*) FROM payments.method_versions WHERE tenant_id=$1`, f.tenant); n != versionsBefore {
		t.Fatalf("PA01 refused enable left %d new revisions", n-versionsBefore)
	}
	if st := p.status(t); st.State != "CONFIGURED_UNVERIFIED" {
		t.Fatalf("initial state %q", st.State)
	}

	vkey := t04Key("pa-verify")
	v, err := p.start(t, vkey)
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^V[A-Za-z0-9_-]{22}$`).MatchString(v.MerchantTradeNo) || v.AmountTWD != 1 || v.Replay ||
		!strings.HasPrefix(v.FormAction, "https://sandbox-api.payuni.com.tw/") {
		t.Fatalf("verification start: %+v", v)
	}
	for _, field := range []string{"MerID", "Version", "EncryptInfo", "HashInfo"} {
		if v.FormFields[field] == "" {
			t.Fatalf("hosted form lacks %s: %v", field, v.FormFields)
		}
	}
	for k, val := range v.FormFields {
		if strings.Contains(val, strings.Repeat("K", 32)) || strings.Contains(k, "HashKey") {
			t.Fatalf("hosted form leaks the key in %s", k)
		}
	}
	if st := p.status(t); st.State != "VERIFYING" || st.VerificationID != v.VerificationID {
		t.Fatalf("after start: %+v", st)
	}
	again, err := p.start(t, vkey) // same Idempotency-Key: the same verification, not a second one
	if err != nil || !again.Replay || again.VerificationID != v.VerificationID || again.MerchantTradeNo != v.MerchantTradeNo {
		t.Fatalf("idempotent replay: %+v %v", again, err)
	}
	if n := countRows(t, f.base.owner, `SELECT count(*) FROM payments.payuni_verifications WHERE tenant_id=$1`, f.tenant); n != 1 {
		t.Fatalf("verifications = %d, want 1", n)
	}
	if _, err = p.start(t, vkey); err != nil { // replay never rewrites evidence
		t.Fatal(err)
	}

	// PA03a: CAPTURED query alone is not enough.
	p.mock.set(paTradeReply(t, p.creds, p.account, v.MerchantTradeNo, true), nil, false)
	res, err := p.check(v.VerificationID)
	if err != nil || res.Outcome != "CAPTURED" || !res.Status.QueryCaptured || res.Status.NotifyReceived {
		t.Fatalf("check: %+v %v", res, err)
	}
	if n := p.sweep(t); n != 0 || p.qualifications(t) != 0 {
		t.Fatalf("PA03a query without notify issued %d qualifications", n)
	}
	if _, err = p.enable(t, t04Key("pa-enable-query-only"), 1); !errors.Is(err, payments.ErrNotQualified) {
		t.Fatalf("PA03a enable after query only: %v", err)
	}
	// A notify signed with another key (or unsigned) leaves no receipt: still nothing.
	if code := p.notify(t, v.MerchantTradeNo, "SUCCESS", 2); code != 200 { // wrong amount: receipt recorded, never qualifies
		t.Fatalf("notify(amount 2) status %d", code)
	}
	if n := p.sweep(t); n != 0 {
		t.Fatalf("a notify for NT$2 qualified the NT$1 verification")
	}

	if code := p.notify(t, v.MerchantTradeNo, "SUCCESS", 1); code != 200 {
		t.Fatalf("notify status %d", code)
	}
	if st := p.status(t); !st.NotifyReceived || !st.QueryCaptured || st.QualificationID != "" {
		t.Fatalf("both channels in, not yet issued: %+v", st)
	}
	if n := p.sweep(t); n != 1 {
		t.Fatalf("sweep issued %d, want 1", n)
	}
	if n := p.sweep(t); n != 0 || p.qualifications(t) != 1 {
		t.Fatalf("issuance is not idempotent: sweep=%d qualifications=%d", n, p.qualifications(t))
	}
	var proof, env, code, evidence string
	var expires time.Time
	if err = f.base.owner.QueryRow(context.Background(), `SELECT proof_class,environment,code,evidence_ref,expires_at FROM payments.account_qualifications
		WHERE tenant_id=$1 AND connection_id=$2`, f.tenant, p.conn.ID).Scan(&proof, &env, &code, &evidence, &expires); err != nil {
		t.Fatal(err)
	}
	if proof != "PROVIDER_MOCK" || env != "SANDBOX" || code != "payuni_credit" || evidence != "payuni-verify:"+v.MerchantTradeNo ||
		time.Until(expires) < 179*24*time.Hour || time.Until(expires) > 181*24*time.Hour {
		t.Fatalf("qualification row: %s %s %s %s %v", proof, env, code, evidence, expires)
	}
	if st := p.status(t); st.State != "VERIFIED_SANDBOX" || st.ProofClass != "PROVIDER_MOCK" || st.QualificationID == "" {
		t.Fatalf("verified state: %+v", st)
	}

	// Enable: the SetMethod definer finds the qualification itself and pins it.
	m, err := p.enable(t, t04Key("pa-enable"), 1)
	if err != nil || !m.Enabled || m.Version != 2 || m.QualificationID != p.status(t).QualificationID {
		t.Fatalf("enable: %+v %v", m, err)
	}
	if got, e := p.get(f.token, f.store); e != nil || !got.Enabled || got.QualificationID != m.QualificationID {
		t.Fatalf("readback: %+v %v", got, e)
	}
	av, err := p.inspect(p.check2(2))
	if err != nil || !av.Available || len(av.Reasons) != 0 {
		t.Fatalf("PA02 qualified enabled method must be available: %+v %v", av, err)
	}
	// The row the buyer path joins (0026 candidate / 0016 start_payment): enabled, visible, qualification of the CURRENT credential
	// version, proof class matching the PROVIDER_MOCK profile, unexpired, unrevoked.
	if n := countRows(t, f.base.owner, `SELECT count(*) FROM payments.method_heads h
		JOIN payments.method_versions m ON m.tenant_id=h.tenant_id AND m.store_id=h.store_id AND m.market_id=h.market_id AND m.country=h.country AND m.code=h.code AND m.version=h.current_version
		JOIN integration.merchant_accounts a ON a.tenant_id=m.tenant_id AND a.store_id=m.store_id AND a.id=m.connection_id AND a.provider='payuni' AND a.environment=m.environment
		JOIN payments.account_qualifications q ON q.tenant_id=m.tenant_id AND q.store_id=m.store_id AND q.id=m.qualification_id AND q.connection_id=a.id
		 AND q.credential_version=a.credential_version AND q.environment=a.environment AND q.code=m.code
		WHERE h.tenant_id=$1 AND m.enabled AND m.visible AND q.revoked_at IS NULL AND q.expires_at>clock_timestamp() AND q.proof_class='PROVIDER_MOCK'`, f.tenant); n != 1 {
		t.Fatalf("buyer-path predicate matched %d methods, want 1", n)
	}
}

// check2 is the inspect input for the current method version of the fixture.
func (p *paFixture) check2(version int64) payments.CheckInput { return p.pmFixture.check(version) }

// PA03b: a notify without CAPTURED query evidence never qualifies; a non-captured report is recorded but inert.
func TestPayuniActivationPA03NotifyWithoutCapturedQueryDoesNotQualify(t *testing.T) {
	p := paSetup(t, "SANDBOX", "PROVIDER_MOCK")
	v, err := p.start(t, t04Key("pa-verify"))
	if err != nil {
		t.Fatal(err)
	}
	if code := p.notify(t, v.MerchantTradeNo, "SUCCESS", 1); code != 200 {
		t.Fatalf("notify status %d", code)
	}
	if n := p.sweep(t); n != 0 {
		t.Fatal("notify alone qualified")
	}
	p.mock.set(paTradeReply(t, p.creds, p.account, v.MerchantTradeNo, false), nil, false) // authorized, no capture
	res, err := p.check(v.VerificationID)
	if err != nil || res.Outcome != "NOT_CAPTURED" || !res.Status.NotifyReceived {
		t.Fatalf("non-captured check: %+v %v", res, err)
	}
	if n := p.sweep(t); n != 0 || p.qualifications(t) != 0 {
		t.Fatal("notify + non-captured query qualified")
	}
	// An UNAVAILABLE query (transport failure) records nothing and is not retried by the service.
	calls := p.mock.callCount()
	p.mock.set("", errors.New("connection reset"), false)
	if res, err = p.check(v.VerificationID); err != nil || res.Outcome != "UNAVAILABLE" || p.mock.callCount() != calls+1 {
		t.Fatalf("UNAVAILABLE check: %+v %v calls=%d->%d", res, err, calls, p.mock.callCount())
	}
	// The SQL layer re-derives CAPTURED: a runtime caller cannot pass a captured flag, only a report, and a refund hint voids it.
	f := p.m.f
	report := `{"MerTradeNo":"` + v.MerchantTradeNo + `","AmountTWD":1,"TradeNo":"X","DataSource":"A","TradeStatus":"1","PaymentType":"1","AuthType":"1","CloseStatus":"2","CloseAmountTWD":1,"CardRefundStatus":"1"}`
	var captured bool
	if err = f.scoped(context.Background(), f.token, f.store, func(tx pgx.Tx, s platform.Scope) error {
		return tx.QueryRow(context.Background(), `SELECT payments.record_payuni_verification_query($1::uuid,$2::jsonb)`, v.VerificationID, report).Scan(&captured)
	}); err != nil || captured {
		t.Fatalf("report with refund hint counted as captured: %v %v", captured, err)
	}
}

// PA04: a verification attempt creates no order, no stock movement, no finance fact, no payment attempt and no job.
func TestPayuniActivationPA04CreatesNoOrderStockOrFinanceRow(t *testing.T) {
	p := paSetup(t, "SANDBOX", "PROVIDER_MOCK")
	f := p.m.f
	const q = `SELECT (SELECT count(*) FROM checkout.orders WHERE tenant_id=$1),(SELECT count(*) FROM checkout.payment_attempts WHERE tenant_id=$1),
	 (SELECT count(*) FROM payments.facts WHERE tenant_id=$1),(SELECT count(*) FROM payments.review_cases WHERE tenant_id=$1),
	 (SELECT count(*) FROM inventory.ledger WHERE tenant_id=$1),(SELECT count(*) FROM inventory.reservations WHERE tenant_id=$1),
	 (SELECT count(*) FROM river_payment.river_job),(SELECT count(*) FROM river.river_job)`
	var before, after [8]int
	scan := func(dst *[8]int) {
		if err := f.base.owner.QueryRow(context.Background(), q, f.tenant).Scan(&dst[0], &dst[1], &dst[2], &dst[3], &dst[4], &dst[5], &dst[6], &dst[7]); err != nil {
			t.Fatal(err)
		}
	}
	scan(&before)
	p.qualify(t)
	scan(&after)
	if before != after {
		t.Fatalf("verification wrote order/stock/finance/job rows: %v -> %v", before, after)
	}
}

// PA05 + PA06: rotation retires the qualification (Available=false CREDENTIAL_ROTATED, no re-enable) and the platform switch gates enable.
func TestPayuniActivationPA05PA06RotationAndPlatformSwitch(t *testing.T) {
	p := paSetup(t, "SANDBOX", "PROVIDER_MOCK")
	f := p.m.f
	if _, err := p.set(f.token, f.store, t04Key("pa-draft"), p.in); err != nil {
		t.Fatal(err)
	}
	pending, err := p.start(t, t04Key("pa-pending")) // pinned to credential v1, never completed
	if err != nil {
		t.Fatal(err)
	}
	p.qualify(t)

	// PA06: the switch off refuses the enable before any write, although a valid qualification exists.
	payments.SetPayuniEnabled(false)
	if _, err = p.enable(t, t04Key("pa-enable-off"), 1); !errors.Is(err, payments.ErrPlatformDisabled) {
		t.Fatalf("PA06 enable with LC_PAYUNI_ENABLED=0: %v", err)
	}
	payments.SetPayuniEnabled(true)
	if _, err = p.enable(t, t04Key("pa-enable"), 1); err != nil {
		t.Fatal(err)
	}
	payments.SetPayuniEnabled(false)
	if av, e := p.inspect(p.check2(2)); e != nil || av.Available || !slices.Contains(av.Reasons, "PLATFORM_DISABLED") || len(av.Reasons) != 1 {
		t.Fatalf("PA06 diagnostic with the switch off: %+v %v", av, e)
	}
	payments.SetPayuniEnabled(true)
	if av, e := p.inspect(p.check2(2)); e != nil || !av.Available {
		t.Fatalf("switch back on: %+v %v", av, e)
	}

	// PA05: rotate the credential.
	if _, err = p.m.rotate(f.token, t04Key("pa-rotate"), accounts.RotateInput{ConnectionID: p.conn.ID, ExpectedVersion: 1,
		Credentials: accounts.Credentials{HashKey: strings.Repeat("R", 32), HashIV: strings.Repeat("W", 16)}}); err != nil {
		t.Fatal(err)
	}
	av, err := p.inspect(p.check2(2))
	if err != nil || av.Available || av.CredentialVersion != 2 || !slices.Contains(av.Reasons, "CREDENTIAL_ROTATED") || slices.Contains(av.Reasons, "NOT_QUALIFIED") {
		t.Fatalf("PA05 rotated: %+v %v", av, err)
	}
	if _, err = p.enable(t, t04Key("pa-enable-rotated"), 2); !errors.Is(err, payments.ErrNotQualified) {
		t.Fatalf("PA05 re-enable with a rotated credential: %v", err)
	}
	if st := p.status(t); st.State != "VERIFYING" || st.QualificationID != "" || st.CredentialVersion != 2 {
		t.Fatalf("PA05 status after rotation still shows a qualification: %+v", st)
	}
	// A verification started on the old key can neither record evidence nor issue.
	p.mock.set(paTradeReply(t, p.creds, p.account, pending.MerchantTradeNo, true), nil, false)
	if _, err = p.check(pending.VerificationID); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("check on a rotated verification: %v", err)
	}
	// The SQL admission fence (0016 start_payment) compares the pinned credential version; mirror it for the enabled revision.
	if n := countRows(t, f.base.owner, `SELECT count(*) FROM payments.method_versions m JOIN payments.account_qualifications q ON q.id=m.qualification_id
		JOIN integration.merchant_accounts a ON a.id=m.connection_id WHERE m.tenant_id=$1 AND m.enabled AND q.credential_version=a.credential_version`, f.tenant); n != 0 {
		t.Fatalf("rotated qualification still matches the buyer-path join: %d", n)
	}
}

// PA07: the LIVE probe over the MOCK transport (real LIVE material path, simulated PAYUNi).
func TestPayuniActivationPA07LiveProbe(t *testing.T) {
	p := paSetup(t, "LIVE", "LIVE")
	f := p.m.f
	probe := func(ctx context.Context) (payments.ActivationStatus, error) {
		return p.act.LiveProbe(ctx, f.base.runtime, f.token, f.store, p.conn.ID)
	}
	counts := func() (int, int) {
		return countRows(t, f.base.owner, `SELECT count(*) FROM payments.payuni_live_probes WHERE tenant_id=$1`, f.tenant), p.qualifications(t)
	}
	wrong := accounts.Credentials{HashKey: strings.Repeat("Q", 32), HashIV: strings.Repeat("Z", 16)}
	noTrade := func(c accounts.Credentials) string {
		return paReply(t, c, p.account, "TRADE_NOT_EXIST", url.Values{"Status": {"TRADE_NOT_EXIST"}, "Message": {"no such trade"}})
	}
	for name, setup := range map[string]func() context.Context{
		"signature made with another key": func() context.Context { p.mock.set(noTrade(wrong), nil, false); return context.Background() },
		"transport failure":               func() context.Context { p.mock.set("", errors.New("reset"), false); return context.Background() },
		"unsigned reply": func() context.Context {
			p.mock.set(`{"Status":"TRADE_NOT_EXIST"}`, nil, false)
			return context.Background()
		},
		"trade row returned": func() context.Context {
			p.mock.set(paTradeReply(t, p.creds, p.account, "whatever", true), nil, false)
			return context.Background()
		},
	} {
		ctx := setup()
		if _, err := probe(ctx); !errors.Is(err, payments.ErrProbeFailed) {
			t.Fatalf("PA07 %s: %v", name, err)
		}
	}
	// Timeout: the provider call never returns. UNKNOWN: no row, no qualification, exactly one call (no retry).
	p.mock.set("", nil, true)
	calls := p.mock.callCount()
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	var err error
	if _, err = probe(ctx); !errors.Is(err, payments.ErrProbeFailed) || p.mock.callCount() != calls+1 {
		t.Fatalf("PA07 timeout: %v calls=%d->%d", err, calls, p.mock.callCount())
	}
	if pr, q := counts(); pr != 0 || q != 0 {
		t.Fatalf("PA07 a failed probe left probes=%d qualifications=%d", pr, q)
	}

	// Authenticated "no such trade": REAL_LIVE, evidence_ref names the probe receipt, status VERIFIED_LIVE_PROBE.
	p.mock.set(noTrade(p.creds), nil, false)
	st, err := probe(context.Background())
	if err != nil || st.State != "VERIFIED_LIVE_PROBE" || st.ProofClass != "REAL_LIVE" || st.QualificationID == "" {
		t.Fatalf("PA07 probe: %+v %v", st, err)
	}
	var evidence, probeID string
	if err = f.base.owner.QueryRow(context.Background(), `SELECT q.evidence_ref,p.id::text FROM payments.account_qualifications q
		JOIN payments.payuni_live_probes p ON p.qualification_id=q.id WHERE q.tenant_id=$1`, f.tenant).Scan(&evidence, &probeID); err != nil || evidence != "live-probe:"+probeID {
		t.Fatalf("evidence_ref %q vs probe %q: %v", evidence, probeID, err)
	}
	// LIVE method: enable needs REAL_LIVE, the switch on and a LIVE connection.
	if _, err = p.set(f.token, f.store, t04Key("pa-live-draft"), p.in); err != nil {
		t.Fatal(err)
	}
	m, err := p.enable(t, t04Key("pa-live-enable"), 1)
	if err != nil || !m.Enabled || m.Environment != "LIVE" || m.QualificationID != st.QualificationID {
		t.Fatalf("LIVE enable: %+v %v", m, err)
	}
	if av, e := p.inspect(payments.CheckInput{MarketID: p.in.MarketID, Country: "TW", Code: "payuni_credit", ExpectedVersion: 2, Environment: "LIVE", Currency: "TWD", AmountMinor: 1000}); e != nil || !av.Available {
		t.Fatalf("LIVE method not available: %+v %v", av, e)
	}

	// SQL pins: an unverified probe leaves no row; a stale probe cannot issue; profile fences in Go.
	err = f.scoped(context.Background(), f.token, f.store, func(tx pgx.Tx, s platform.Scope) error {
		_, e := tx.Exec(context.Background(), `SELECT payments.record_payuni_live_probe($1::uuid,1,'LPunverified0001',false)`, p.conn.ID)
		return e
	})
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != "22023" {
		t.Fatalf("unverified probe record: %v", err)
	}
	sandbox := paSetup(t, "SANDBOX", "SANDBOX")
	if _, err = sandbox.act.LiveProbe(context.Background(), sandbox.m.f.base.runtime, sandbox.m.f.token, sandbox.m.f.store, sandbox.conn.ID); !errors.Is(err, payments.ErrProfileNotAllowed) {
		t.Fatalf("LIVE probe under a SANDBOX deployment: %v", err)
	}
	if _, err = p.start(t, t04Key("pa-live-start")); !errors.Is(err, payments.ErrProfileNotAllowed) {
		t.Fatalf("SANDBOX verification under a LIVE deployment: %v", err)
	}
	// A SANDBOX connection can never be probed as LIVE, and a LIVE connection never verified as SANDBOX (PT409 -> conflict).
	live := paSetup(t, "LIVE", "LIVE")
	live.act, _ = payments.NewActivation(live.keys, payments.ActivationConfig{Profile: "SANDBOX", NotifyBaseURL: "https://hooks.example.com", ReturnURL: "https://admin.example.com/x"})
	if _, err = live.start(t, t04Key("pa-live-conn")); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("verification of a LIVE connection: %v", err)
	}
}

// PA08 + 0137 ACL: no caller-chosen proof class, no cross-store reach, no raw writes, exact EXECUTE sets.
func TestPayuniActivationPA08ForgeryCrossStoreAndRawWrites(t *testing.T) {
	p := paSetup(t, "SANDBOX", "PROVIDER_MOCK")
	f := p.m.f
	v := p.qualify(t)
	if _, err := p.set(f.token, f.store, t04Key("pa-draft"), p.in); err != nil {
		t.Fatal(err)
	}
	denied := func(name string, err error) {
		t.Helper()
		var pg *pgconn.PgError
		if !errors.As(err, &pg) || pg.Code != "42501" {
			t.Errorf("%s: want 42501, got %v", name, err)
		}
	}
	runtimeExec := func(q string, args ...any) error {
		return f.scoped(context.Background(), f.token, f.store, func(tx pgx.Tx, s platform.Scope) error {
			_, e := tx.Exec(context.Background(), q, args...)
			return e
		})
	}
	var qid string
	if err := f.base.owner.QueryRow(context.Background(), `SELECT id::text FROM payments.account_qualifications WHERE tenant_id=$1`, f.tenant).Scan(&qid); err != nil {
		t.Fatal(err)
	}
	// Merchants cannot write a qualification (any proof class), nor run the worker issuers.
	denied("raw qualification insert", runtimeExec(`INSERT INTO payments.account_qualifications(id,tenant_id,store_id,connection_id,credential_version,environment,code,proof_class,evidence_ref,observed_at,expires_at)
		VALUES(gen_random_uuid(),$1,$2,$3,1,'SANDBOX','payuni_credit','REAL_SANDBOX','forged',clock_timestamp()-interval '1 second',clock_timestamp()+interval '1 day')`, f.tenant, f.store, p.conn.ID))
	denied("issue_payuni_qualification as merchant", runtimeExec(`SELECT payments.issue_payuni_qualification($1::uuid)`, v.VerificationID))
	denied("sweep as merchant", runtimeExec(`SELECT payments.sweep_payuni_verifications(10)`))
	denied("read qualifications as merchant", runtimeExec(`SELECT count(*) FROM payments.account_qualifications`))
	denied("read verifications as merchant", runtimeExec(`SELECT count(*) FROM payments.payuni_verifications`))
	// A merchant cannot write an enabled method row by hand even with a real qualification id: RLS (restrictive) refuses it.
	denied("raw enabled method insert", runtimeExec(`INSERT INTO payments.method_versions(tenant_id,store_id,market_id,country,code,version,provider,environment,connection_id,binding_version,currency,name_hans,name_hant,name_en,enabled,visible,sort_order,min_amount_minor,max_amount_minor,principal_id,qualification_id)
		SELECT tenant_id,store_id,market_id,country,code,2,provider,environment,connection_id,binding_version,currency,name_hans,name_hant,name_en,true,visible,sort_order,min_amount_minor,max_amount_minor,principal_id,$2::uuid
		FROM payments.method_versions WHERE tenant_id=$1 AND version=1`, f.tenant, qid))
	// The payment worker cannot start verifications, read credentials or enable methods.
	for _, fn := range []string{`payments.start_payuni_verification($1::uuid,'abcdefgh','SANDBOX',decode(repeat('00',32),'hex'))`,
		`payments.load_payuni_activation_material($1::uuid,NULL)`, `payments.payuni_activation_status($1::uuid)`} {
		if _, err := p.worker.Exec(context.Background(), `SELECT * FROM `+fn, p.conn.ID); err == nil {
			t.Errorf("payment worker ran %s", fn)
		} else {
			denied("worker "+fn, err)
		}
	}
	// Cross-store: another store's authenticated scope cannot see or use this connection or verification (404).
	// f.token holds integration:manage on both stores of the tenant, so only the scope differs.
	var err error
	if err = f.scoped(context.Background(), f.token, f.otherStore, func(tx pgx.Tx, s platform.Scope) error {
		_, e := p.act.Start(context.Background(), tx, s, f.token, t04Key("pa-cross"), p.conn.ID)
		return e
	}); !errors.Is(err, command.ErrNotFound) {
		t.Fatalf("cross-store Start: %v", err)
	}
	if err = f.scoped(context.Background(), f.token, f.otherStore, func(tx pgx.Tx, s platform.Scope) error {
		_, e := p.act.Status(context.Background(), tx, s, f.token, p.conn.ID)
		return e
	}); !errors.Is(err, command.ErrNotFound) {
		t.Fatalf("cross-store Status: %v", err)
	}
	if _, err = p.act.Check(context.Background(), f.base.runtime, f.token, f.otherStore, p.conn.ID, v.VerificationID); !errors.Is(err, command.ErrNotFound) {
		t.Fatalf("cross-store Check: %v", err)
	}
	if _, err = p.act.LiveProbe(context.Background(), f.base.runtime, f.token, f.otherStore, p.conn.ID); !errors.Is(err, payments.ErrProfileNotAllowed) {
		t.Fatalf("probe under a MOCK deployment: %v", err)
	}
	// Same-store scope but another connection id / verification id never resolves.
	if _, err = p.act.Check(context.Background(), f.base.runtime, f.token, f.store, randomUUID(), v.VerificationID); !errors.Is(err, command.ErrNotFound) {
		t.Fatalf("unknown connection: %v", err)
	}
	if _, err = p.act.Check(context.Background(), f.base.runtime, f.token, f.store, p.conn.ID, randomUUID()); !errors.Is(err, command.ErrNotFound) {
		t.Fatalf("unknown verification: %v", err)
	}
	// No function takes a proof class: it cannot be forged by construction.
	for _, fn := range []string{"issue_payuni_qualification", "issue_payuni_live_qualification", "sweep_payuni_verifications",
		"record_payuni_verification_query", "record_payuni_live_probe", "start_payuni_verification", "enable_payuni_method"} {
		var args string
		if err = f.base.owner.QueryRow(context.Background(), `SELECT pg_get_function_arguments(p.oid) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
			WHERE n.nspname='payments' AND p.proname=$1`, fn).Scan(&args); err != nil || strings.Contains(strings.ToLower(args), "proof") {
			t.Fatalf("%s accepts a proof class: %q %v", fn, args, err)
		}
	}
}

// ACL pin of every 0137 function, table and policy (the schema ACL inventories pin the rest of payments.*).
func TestPayuniActivationACLPinned(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	const owner = "commerce_payment_registry_writer"
	funcs := []struct {
		sig      string
		grantees []string
	}{
		{"payments.start_payuni_verification(uuid,text,text,bytea)", []string{"commerce_runtime"}},
		{"payments.load_payuni_activation_material(uuid,uuid)", []string{"commerce_runtime"}},
		{"payments.record_payuni_verification_query(uuid,jsonb)", []string{"commerce_runtime"}},
		{"payments.issue_payuni_qualification(uuid)", []string{"commerce_payment_worker"}},
		{"payments.sweep_payuni_verifications(integer)", []string{"commerce_payment_worker"}},
		{"payments.record_payuni_live_probe(uuid,bigint,text,boolean)", []string{"commerce_runtime"}},
		{"payments.issue_payuni_live_qualification(uuid)", []string{"commerce_runtime"}},
		{"payments.payuni_activation_status(uuid)", []string{"commerce_runtime"}},
		{"payments.method_qualification_state(uuid)", []string{"commerce_runtime"}},
		{"payments.enable_payuni_method(uuid,text,text,bigint,text,uuid,bigint,text,text,text,boolean,integer,bigint,bigint)", []string{"commerce_runtime"}},
	}
	for _, fn := range funcs {
		var o string
		var secdef bool
		var cfg *string
		var holders []string
		if err := f.owner.QueryRow(ctx, `SELECT pg_get_userbyid(p.proowner),p.prosecdef,p.proconfig::text,
			coalesce((SELECT array_agg(CASE WHEN a.grantee=0 THEN 'PUBLIC' ELSE pg_get_userbyid(a.grantee) END ORDER BY 1) FROM aclexplode(p.proacl) a WHERE a.privilege_type='EXECUTE' AND a.grantee<>p.proowner),'{}')
			FROM pg_proc p WHERE p.oid=to_regprocedure($1)`, fn.sig).Scan(&o, &secdef, &cfg, &holders); err != nil {
			t.Fatalf("%s: %v", fn.sig, err)
		}
		if o != owner || !secdef || cfg == nil || !strings.Contains(*cfg, "search_path=pg_catalog") || !slices.Equal(holders, fn.grantees) {
			t.Errorf("%s: owner=%s secdef=%v config=%v holders=%v, want %s %v", fn.sig, o, secdef, cfg, holders, owner, fn.grantees)
		}
		// Every login-capable authority except the pinned grantee is refused at the catalog level.
		for _, role := range []string{"commerce_runtime", waPayment, waLive, waExpiry, waAds, waClaims, waLegacy, "commerce_payuni_ingress", "commerce_stripe_ingress", "commerce_checkout_runtime"} {
			var ok *bool
			if err := f.owner.QueryRow(ctx, `SELECT has_function_privilege($1,to_regprocedure($2),'EXECUTE')`, role, fn.sig).Scan(&ok); err != nil {
				t.Fatalf("%s %s: %v", role, fn.sig, err)
			}
			if want := slices.Contains(fn.grantees, role); ok == nil || *ok != want {
				t.Errorf("%s EXECUTE %s = %v, want %v", role, fn.sig, ok, want)
			}
		}
	}
	for _, rel := range []string{"payments.payuni_verifications", "payments.payuni_live_probes"} {
		for _, role := range []string{"commerce_runtime", waPayment, waLive, waExpiry, waAds, waClaims, waLegacy, "commerce_payuni_ingress", "commerce_checkout_runtime", "commerce_checkout_writer"} {
			var any bool
			if err := f.owner.QueryRow(ctx, `SELECT has_any_column_privilege($1,$2::regclass,'SELECT,INSERT,UPDATE') OR has_table_privilege($1,$2::regclass,'DELETE,TRUNCATE')`, role, rel).Scan(&any); err != nil || any {
				t.Errorf("%s holds a privilege on %s (err=%v)", role, rel, err)
			}
		}
		var rls, force bool
		if err := f.owner.QueryRow(ctx, `SELECT relrowsecurity,relforcerowsecurity FROM pg_class WHERE oid=$1::regclass`, rel).Scan(&rls, &force); err != nil || !rls || !force {
			t.Errorf("%s RLS=%v FORCE=%v (%v)", rel, rls, force, err)
		}
	}
	var n int
	if err := f.owner.QueryRow(ctx, `SELECT count(*) FROM pg_policy WHERE polname='payuni_method_enabled_definer_only' AND polpermissive=false`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("restrictive enabled-method fence missing: %d %v", n, err)
	}
}
