// payuni_notify_test.go: REAL_PG foundation gate for the PAYUNi NotifyURL receiver (w4-01b-payuni-notify).
// The handler and its fake-store unit tests cover request shape and ordering; these tests run the real
// ingress role, endpoint custody, the two 0136 definers and the wake/dedup/mismatch money-path fence
// against an isolated per-test PG container. A notification is only a trigger to query: it never writes
// a money fact, and the only effect of a verified callback is one idempotent receipt plus a wake of the
// attempt's existing payment_query_v1 job (no job insert, no provider call, no LIVE).
//
// Runs as/in: pwIsolatedFixture (REAL_PG, MOCK: the signed body is synthetic, no provider is called).
// Status: NOT_RUN until bash scripts/dev/test-focused.sh '^TestPayuniNotify' is executed by the integrator.
package foundation_test

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/payments/payuninotify"
	"livecommerce/internal/platform"
)

// pnFixture extends the payment-query fixture with one registered notify endpoint, the ingress pool
// and a built handler. pqSetupWorker provisions the payuni merchant (SANDBOX), attempt (PROVIDER_MOCK,
// amount_minor=2500 → AmountTWD=25) and its single query job; the endpoint reuses that connection.
type pnFixture struct {
	pqFixture
	token      string
	tokenHash  []byte
	endpointID string
	ingress    *pgxpool.Pool
	handler    http.Handler
}

// pnSetup provisions the endpoint with pqOldSecret as the current (v2) credential, so a body signed
// with pqOldSecret authenticates against the current envelope. The token is stored only as sha256.
func pnSetup(t *testing.T) pnFixture {
	t.Helper()
	q := pqSetupWorker(t)
	token := randomToken()
	sum := sha256.Sum256([]byte(token))
	endpointID := randomUUID()
	mustExec(t, q.f.owner, `SELECT payments.set_payuni_notify_endpoint($1,$2,$3,$4,$5,'PROVIDER_MOCK',true,$6)`,
		q.f.tenantA, q.f.storeA1, q.f.principalA, q.account, endpointID, sum[:])
	login := saNewLogin(t, q.f, "commerce_payuni_ingress")
	pool, err := platform.OpenPayuniIngressPool(context.Background(), login.dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	inbox, err := payuninotify.NewInbox(context.Background(), pool, q.keys, "PROVIDER_MOCK")
	if err != nil {
		t.Fatal(err)
	}
	h, err := payuninotify.NewHandler(inbox)
	if err != nil {
		t.Fatal(err)
	}
	return pnFixture{pqFixture: q, token: token, tokenHash: sum[:], endpointID: endpointID, ingress: pool, handler: h}
}

// pnSignedBody builds an outer form exactly as PAYUNi signs a NotifyURL callback with the given
// HashKey/HashIV (AES-256-GCM, nonce=HashIV, EncryptInfo=hex(b64(ct):::b64(tag)),
// HashInfo=upper hex sha256(HashKey+EncryptInfo+HashIV)).
func pnSignedBody(t *testing.T, creds accounts.Credentials, accountID, merTradeNo, tradeNo string,
	amountTWD int64, status, tradeStatus string) []byte {
	t.Helper()
	inner := url.Values{"MerID": {accountID}, "MerTradeNo": {merTradeNo}, "TradeNo": {tradeNo},
		"TradeAmt": {strconv.FormatInt(amountTWD, 10)}, "PaymentType": {"1"},
		"Status": {status}, "TradeStatus": {tradeStatus}}
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
	hashInfo := strings.ToUpper(hex.EncodeToString(sum[:]))
	return []byte(url.Values{"Status": {status}, "MerID": {accountID}, "Version": {"2.0"},
		"EncryptInfo": {encryptInfo}, "HashInfo": {hashInfo}}.Encode())
}

func pnPost(t *testing.T, h http.Handler, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/hooks/payuni/notify/"+token, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func pnReceipt(t *testing.T, p pnFixture, payload []byte) (disposition, attemptID string, jobID int64, redelivery int) {
	t.Helper()
	sum := sha256.Sum256(payload)
	var att *string
	var job *int64
	if err := p.f.owner.QueryRow(context.Background(), `SELECT disposition,attempt_id::text,job_id,redelivery_count
		FROM payments.payuni_notify_receipts WHERE connection_id=$1 AND payload_sha256=$2`,
		p.account, sum[:]).Scan(&disposition, &att, &job, &redelivery); err != nil {
		t.Fatal(err)
	}
	if att != nil {
		attemptID = *att
	}
	if job != nil {
		jobID = *job
	}
	return
}

func pnReceiptCount(t *testing.T, p pnFixture) int {
	t.Helper()
	var n int
	if err := p.f.owner.QueryRow(context.Background(), `SELECT count(*) FROM payments.payuni_notify_receipts`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// pnValidBody signs a SUCCESS callback for the frozen attempt with pqOldSecret.
func pnValidBody(t *testing.T, p pnFixture) []byte {
	return pnSignedBody(t, pqOldSecret, "mock-account", p.result.MerchantTradeNo, "trade_"+t04Tag(),
		p.result.AmountMinor/100, "SUCCESS", "1")
}

func TestPayuniNotifyQueuedWakesQueryJobAndDedups(t *testing.T) {
	p := pnSetup(t)
	// Push the attempt's query job an hour into the future: only the notify wake can rewind it.
	mustExec(t, p.f.owner, `UPDATE river_payment.river_job SET scheduled_at=clock_timestamp()+interval '1 hour' WHERE id=$1`, p.result.JobID)
	body := pnValidBody(t, p)
	rec := pnPost(t, p.handler, p.token, string(body))
	if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Fatalf("verified notify: %d %q", rec.Code, rec.Body.String())
	}
	disposition, attemptID, jobID, redelivery := pnReceipt(t, p, body)
	if disposition != "QUEUED" || attemptID != p.result.AttemptID || jobID != p.result.JobID || redelivery != 0 {
		t.Fatalf("receipt %s attempt=%s job=%d redelivery=%d", disposition, attemptID, jobID, redelivery)
	}
	var rewound bool
	if err := p.f.owner.QueryRow(context.Background(), `SELECT scheduled_at < clock_timestamp() FROM river_payment.river_job WHERE id=$1`, p.result.JobID).Scan(&rewound); err != nil || !rewound {
		t.Fatalf("notify did not wake the query job: %v", err)
	}
	// No money fact: the notification only triggers a query; order/reservation/stock stay untouched.
	p.pending(t)
	// The same payload is idempotent: 200 again, still one receipt, redelivery_count bumped.
	rec = pnPost(t, p.handler, p.token, string(body))
	if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Fatalf("duplicate notify: %d %q", rec.Code, rec.Body.String())
	}
	_, _, _, redelivery = pnReceipt(t, p, body)
	if redelivery != 1 || pnReceiptCount(t, p) != 1 {
		t.Fatalf("duplicate notify redelivery=%d count=%d", redelivery, pnReceiptCount(t, p))
	}
	p.pending(t)
}

func TestPayuniNotifyMismatchOpensReviewCaseNoWake(t *testing.T) {
	p := pnSetup(t)
	mustExec(t, p.f.owner, `UPDATE river_payment.river_job SET scheduled_at=clock_timestamp()+interval '1 hour' WHERE id=$1`, p.result.JobID)
	body := pnSignedBody(t, pqOldSecret, "mock-account", p.result.MerchantTradeNo, "trade_"+t04Tag(),
		p.result.AmountMinor/100+1, "SUCCESS", "1")
	rec := pnPost(t, p.handler, p.token, string(body))
	if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Fatalf("mismatch notify: %d %q", rec.Code, rec.Body.String())
	}
	disposition, attemptID, jobID, _ := pnReceipt(t, p, body)
	if disposition != "MISMATCH" || attemptID != p.result.AttemptID || jobID != 0 {
		t.Fatalf("mismatch receipt %s attempt=%s job=%d", disposition, attemptID, jobID)
	}
	var review int
	if err := p.f.owner.QueryRow(context.Background(), `SELECT count(*) FROM payments.review_cases
		WHERE attempt_id=$1 AND reason='NOTIFY_MISMATCH' AND source_report_hash IS NULL`, p.result.AttemptID).Scan(&review); err != nil || review != 1 {
		t.Fatalf("mismatch review case n=%d err=%v", review, err)
	}
	var stillFuture bool
	if err := p.f.owner.QueryRow(context.Background(), `SELECT scheduled_at > clock_timestamp() FROM river_payment.river_job WHERE id=$1`, p.result.JobID).Scan(&stillFuture); err != nil || !stillFuture {
		t.Fatalf("mismatch woke the query job: %v", err)
	}
	p.pending(t)
}

func TestPayuniNotifyUnknownTradeReceiptOnly(t *testing.T) {
	p := pnSetup(t)
	body := pnSignedBody(t, pqOldSecret, "mock-account", "ORDER_"+t04Tag(), "trade_"+t04Tag(),
		p.result.AmountMinor/100, "SUCCESS", "1")
	rec := pnPost(t, p.handler, p.token, string(body))
	if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Fatalf("unknown-trade notify: %d %q", rec.Code, rec.Body.String())
	}
	disposition, attemptID, jobID, _ := pnReceipt(t, p, body)
	if disposition != "UNKNOWN_TRADE" || attemptID != "" || jobID != 0 {
		t.Fatalf("unknown-trade receipt %s attempt=%s job=%d", disposition, attemptID, jobID)
	}
	var review int
	if err := p.f.owner.QueryRow(context.Background(), `SELECT count(*) FROM payments.review_cases WHERE reason='NOTIFY_MISMATCH'`).Scan(&review); err != nil || review != 0 {
		t.Fatalf("unknown trade opened a review case n=%d err=%v", review, err)
	}
	p.pending(t)
}

func TestPayuniNotifyRejectionsLeaveNoRows(t *testing.T) {
	p := pnSetup(t)
	body := string(pnValidBody(t, p))
	// A tampered signature (HashInfo) is a crypto failure: 400 and no receipt row.
	vals, err := url.ParseQuery(body)
	if err != nil {
		t.Fatal(err)
	}
	vals.Set("HashInfo", strings.Repeat("0", 64))
	if rec := pnPost(t, p.handler, p.token, vals.Encode()); rec.Code != http.StatusBadRequest {
		t.Fatalf("tampered signature: %d", rec.Code)
	}
	// An unknown token is invisible (404), not an existence or authority leak.
	if rec := pnPost(t, p.handler, randomToken(), body); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown token: %d", rec.Code)
	}
	// A second endpoint on the same connection under SANDBOX is not admitted by a PROVIDER_MOCK process.
	sandboxToken := randomToken()
	sandboxHash := sha256.Sum256([]byte(sandboxToken))
	mustExec(t, p.f.owner, `SELECT payments.set_payuni_notify_endpoint($1,$2,$3,$4,$5,'SANDBOX',true,$6)`,
		p.f.tenantA, p.f.storeA1, p.f.principalA, p.account, randomUUID(), sandboxHash[:])
	if rec := pnPost(t, p.handler, sandboxToken, body); rec.Code != http.StatusNotFound {
		t.Fatalf("wrong-profile endpoint: %d", rec.Code)
	}
	// TradeStatus is authenticated but then rejected as malformed before any DB write.
	bad := pnSignedBody(t, pqOldSecret, "mock-account", p.result.MerchantTradeNo, "trade_"+t04Tag(),
		p.result.AmountMinor/100, "SUCCESS", "abc")
	if rec := pnPost(t, p.handler, p.token, string(bad)); rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed trade status: %d", rec.Code)
	}
	if n := pnReceiptCount(t, p); n != 0 {
		t.Fatalf("rejected deliveries left %d receipt rows", n)
	}
	p.pending(t)
}

// The definer independently rejects malformed input (defense in depth behind the handler's own 4xx).
func TestPayuniNotifyDefinerRejectsMalformedInput(t *testing.T) {
	p := pnSetup(t)
	ctx := context.Background()
	goodSHA := sha256.Sum256([]byte("x"))
	cases := []struct {
		name string
		sql  string
		args []any
	}{
		{"short token hash", `SELECT * FROM payments.payuni_record_notify(decode(repeat('00',31),'hex'),$1::bytea,'ORDER_1','',25,'SUCCESS','1')`, []any{goodSHA[:]}},
		{"short payload hash", `SELECT * FROM payments.payuni_record_notify($1::bytea,decode(repeat('00',31),'hex'),'ORDER_1','',25,'SUCCESS','1')`, []any{p.tokenHash}},
		{"bad merchant trade no", `SELECT * FROM payments.payuni_record_notify($1::bytea,$2::bytea,'bad trade no!','',25,'SUCCESS','1')`, []any{p.tokenHash, goodSHA[:]}},
		{"zero amount", `SELECT * FROM payments.payuni_record_notify($1::bytea,$2::bytea,'ORDER_1','',0,'SUCCESS','1')`, []any{p.tokenHash, goodSHA[:]}},
		{"bad status", `SELECT * FROM payments.payuni_record_notify($1::bytea,$2::bytea,'ORDER_1','',25,'NOPE','1')`, []any{p.tokenHash, goodSHA[:]}},
		{"bad trade status", `SELECT * FROM payments.payuni_record_notify($1::bytea,$2::bytea,'ORDER_1','',25,'SUCCESS','abc')`, []any{p.tokenHash, goodSHA[:]}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := p.ingress.Exec(ctx, c.sql, c.args...)
			if sqlState(err) != "22023" {
				t.Fatalf("%s: got %v want SQLSTATE 22023", c.name, err)
			}
		})
	}
	if n := pnReceiptCount(t, p); n != 0 {
		t.Fatalf("malformed definer input persisted %d receipts", n)
	}
}

func TestPayuniNotifyResolveEndpointMaterialAndGrace(t *testing.T) {
	p := pnSetup(t)
	ctx := context.Background()
	var accountID, profile, curKeyID string
	var curVersion int64
	var prevVersion *int64
	if err := p.ingress.QueryRow(ctx, `SELECT account_id,execution_profile,cur_version,cur_key_id,prev_version
		FROM payments.payuni_resolve_endpoint($1::bytea)`, p.tokenHash).Scan(&accountID, &profile, &curVersion, &curKeyID, &prevVersion); err != nil {
		t.Fatal(err)
	}
	if accountID != "mock-account" || profile != "PROVIDER_MOCK" || curVersion != 2 || curKeyID == "" || prevVersion == nil || *prevVersion != 1 {
		t.Fatalf("resolve material account=%q profile=%q cur=%d prev=%v keyID=%q", accountID, profile, curVersion, prevVersion, curKeyID)
	}
	// Rotate the connection to v3; the callback is still signed with the previous (v2=pqOldSecret)
	// HashKey, so the handler must fall back to the previous credential envelope to verify it.
	p.rotate(t, 2, accounts.Credentials{HashKey: strings.Repeat("N", 32), HashIV: strings.Repeat("I", 16)})
	body := pnValidBody(t, p) // signed with pqOldSecret (now the previous credential)
	rec := pnPost(t, p.handler, p.token, string(body))
	if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Fatalf("rotation-grace previous-signature notify: %d %q", rec.Code, rec.Body.String())
	}
	disposition, attemptID, jobID, _ := pnReceipt(t, p, body)
	if disposition != "QUEUED" || attemptID != p.result.AttemptID || jobID != p.result.JobID {
		t.Fatalf("grace receipt %s attempt=%s job=%d", disposition, attemptID, jobID)
	}
	p.pending(t)
}
