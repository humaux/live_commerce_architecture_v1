// k3_w4_01b_adversarial_test.go: K3 independent adversarial acceptance of the W4-01B PAYUNi
// NotifyURL receiver (the author is DeepSeek; these are NEW tests, no product code changed).
// Attack classes: forged/re-signed/tampered bodies, cross-store credential replay, rotation-grace
// boundary (expired previous version), concurrent replay dedup, disabled store/endpoint semantics,
// wake-target pinning (only the matching payment_query_v1 job, never an INSERT), execution-profile
// mismatch, LIVE refusal at every layer, and the ingress role's authority ceiling beyond the two
// 0136 definers. A notification is only a trigger to query: every test re-asserts zero money facts.
//
// Runs as/in: pwIsolatedFixture via pnSetup (REAL_PG, MOCK: signed bodies are synthetic).
// Status: NOT_RUN until bash scripts/dev/test-focused.sh '^TestK3W401B' is executed.
package foundation_test

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/payments/payuninotify"
)

// k3Sign builds a PAYUNi-shaped outer form with full control over the outer MerID/Status and the
// inner (encrypted) fields, signed with the given credentials — the adversary's signing oracle.
func k3Sign(t *testing.T, creds accounts.Credentials, outerMerID, outerStatus string, inner url.Values) string {
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
	return url.Values{"Status": {outerStatus}, "MerID": {outerMerID}, "Version": {"2.0"},
		"EncryptInfo": {encryptInfo}, "HashInfo": {strings.ToUpper(hex.EncodeToString(sum[:]))}}.Encode()
}

// k3Inner is a well-shaped inner notify form for the fixture attempt.
func k3Inner(p pnFixture, tradeNo string, amountTWD int64, status string) url.Values {
	return url.Values{"MerID": {"mock-account"}, "MerTradeNo": {p.result.MerchantTradeNo},
		"TradeNo": {tradeNo}, "TradeAmt": {strconv.FormatInt(amountTWD, 10)}, "PaymentType": {"1"},
		"Status": {status}, "TradeStatus": {"1"}}
}

// k3ReviewCount counts NOTIFY_MISMATCH review cases (notify must open them only on MISMATCH).
func k3ReviewCount(t *testing.T, p pnFixture) int {
	t.Helper()
	var n int
	if err := p.f.owner.QueryRow(context.Background(),
		`SELECT count(*) FROM payments.review_cases WHERE reason='NOTIFY_MISMATCH'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// k3JobFuture pushes the attempt's query job an hour into the future and reports whether it is
// still in the future (not woken) at check time.
func k3JobFuture(t *testing.T, p pnFixture) bool {
	t.Helper()
	var future bool
	if err := p.f.owner.QueryRow(context.Background(),
		`SELECT scheduled_at > clock_timestamp() FROM river_payment.river_job WHERE id=$1`,
		p.result.JobID).Scan(&future); err != nil {
		t.Fatal(err)
	}
	return future
}

func k3PushJob(t *testing.T, p pnFixture) {
	t.Helper()
	mustExec(t, p.f.owner, `UPDATE river_payment.river_job SET scheduled_at=clock_timestamp()+interval '1 hour' WHERE id=$1`, p.result.JobID)
}

// k3NoFacts asserts the notify left no money fact anywhere: no provider observation, no payment
// fact, and the order/attempt/reservation/stock chain unchanged.
func k3NoFacts(t *testing.T, p pnFixture) {
	t.Helper()
	var observations, facts int
	if err := p.f.owner.QueryRow(context.Background(),
		`SELECT (SELECT count(*) FROM payments.provider_observations),(SELECT count(*) FROM payments.facts)`).
		Scan(&observations, &facts); err != nil {
		t.Fatal(err)
	}
	if observations != 0 || facts != 0 {
		t.Fatalf("notify wrote money facts: observations=%d facts=%d", observations, facts)
	}
	p.pending(t)
}

// Forged, re-signed, rebound and malformed bodies are all refused with 400 and zero rows.
func TestK3W401BForgedBodiesLeaveNoRows(t *testing.T) {
	p := pnSetup(t)
	k3PushJob(t, p)
	attacker := accounts.Credentials{HashKey: strings.Repeat("A", 32), HashIV: strings.Repeat("B", 16)}
	amount := p.result.AmountMinor / 100

	// Re-signed: the genuine EncryptInfo but a HashInfo computed with the attacker's key.
	vals, err := url.ParseQuery(string(pnValidBody(t, p)))
	if err != nil {
		t.Fatal(err)
	}
	forged := sha256.Sum256([]byte(attacker.HashKey + vals.Get("EncryptInfo") + attacker.HashIV))
	vals.Set("HashInfo", strings.ToUpper(hex.EncodeToString(forged[:])))

	longTrade := k3Inner(p, "trade_"+t04Tag(), amount, "SUCCESS")
	longTrade.Set("MerTradeNo", strings.Repeat("T", 26)) // exceeds the 25-char trade pattern
	wrongInnerMer := k3Inner(p, "trade_"+t04Tag(), amount, "SUCCESS")
	wrongInnerMer.Set("MerID", "ATTACKER")

	cases := map[string]string{
		"attacker key signature":   k3Sign(t, attacker, "mock-account", "SUCCESS", k3Inner(p, "trade_"+t04Tag(), amount, "SUCCESS")),
		"re-signed genuine cipher": vals.Encode(),
		"outer MerID rebound":      k3Sign(t, pqOldSecret, "ATTACKER", "SUCCESS", k3Inner(p, "trade_"+t04Tag(), amount, "SUCCESS")),
		"inner MerID rebound":      k3Sign(t, pqOldSecret, "mock-account", "SUCCESS", wrongInnerMer),
		"status pair broken":       k3Sign(t, pqOldSecret, "mock-account", "SUCCESS", k3Inner(p, "trade_"+t04Tag(), amount, "UNKNOWN")),
		"trade no too long":        k3Sign(t, pqOldSecret, "mock-account", "SUCCESS", longTrade),
		"unsigned form":            "Status=SUCCESS&MerID=mock-account&Version=2.0",
		"duplicate outer field":    string(pnValidBody(t, p)) + "&Version=2.0",
		"garbage":                  "%%%not-a-form%%%",
	}
	// Version downgrade: keep the valid signature, swap the protocol version.
	v10, err := url.ParseQuery(string(pnValidBody(t, p)))
	if err != nil {
		t.Fatal(err)
	}
	v10.Set("Version", "1.0")
	cases["version downgrade"] = v10.Encode()

	for name, body := range cases {
		rec := pnPost(t, p.handler, p.token, body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: got %d want 400", name, rec.Code)
		}
	}
	if n := pnReceiptCount(t, p); n != 0 {
		t.Fatalf("forged deliveries left %d receipts", n)
	}
	if n := k3ReviewCount(t, p); n != 0 {
		t.Fatalf("forged deliveries opened %d review cases", n)
	}
	if !k3JobFuture(t, p) {
		t.Fatal("forged delivery woke the query job")
	}
	k3NoFacts(t, p)
}

// Concurrent replay of one valid delivery: exactly one receipt, one wake, all 200, no job insert.
func TestK3W401BConcurrentReplayExactlyOneReceipt(t *testing.T) {
	p := pnSetup(t)
	k3PushJob(t, p)
	body := string(pnValidBody(t, p))
	var jobsBefore int
	if err := p.f.owner.QueryRow(context.Background(), `SELECT count(*) FROM river_payment.river_job`).Scan(&jobsBefore); err != nil {
		t.Fatal(err)
	}
	const n = 8
	codes := make([]int, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i] = pnPost(t, p.handler, p.token, body).Code
		}(i)
	}
	wg.Wait()
	for i, c := range codes {
		if c != http.StatusOK {
			t.Fatalf("replay %d got %d want 200", i, c)
		}
	}
	disposition, attemptID, jobID, redelivery := pnReceipt(t, p, []byte(body))
	if disposition != "QUEUED" || attemptID != p.result.AttemptID || jobID != p.result.JobID || redelivery != n-1 {
		t.Fatalf("receipt %s attempt=%s job=%d redelivery=%d", disposition, attemptID, jobID, redelivery)
	}
	if got := pnReceiptCount(t, p); got != 1 {
		t.Fatalf("concurrent replay persisted %d receipts", got)
	}
	var jobsAfter int
	if err := p.f.owner.QueryRow(context.Background(), `SELECT count(*) FROM river_payment.river_job`).Scan(&jobsAfter); err != nil {
		t.Fatal(err)
	}
	if jobsAfter != jobsBefore {
		t.Fatalf("notify inserted river jobs: before=%d after=%d", jobsBefore, jobsAfter)
	}
	if k3JobFuture(t, p) {
		t.Fatal("valid notify did not wake the query job")
	}
	k3NoFacts(t, p)
}

// Rotation grace is exactly one version wide: after two rotations the two-behind signature dies.
func TestK3W401BRotationGraceBoundary(t *testing.T) {
	p := pnSetup(t)
	v3 := accounts.Credentials{HashKey: strings.Repeat("N", 32), HashIV: strings.Repeat("I", 16)}
	v4 := accounts.Credentials{HashKey: strings.Repeat("R", 32), HashIV: strings.Repeat("W", 16)}
	p.rotate(t, 2, v3) // current v3, previous v2 (pqOldSecret)
	p.rotate(t, 3, v4) // current v4, previous v3; v2 is now expired history

	// Two versions behind: refused, no rows.
	old := pnSignedBody(t, pqOldSecret, "mock-account", p.result.MerchantTradeNo, "trade_"+t04Tag(),
		p.result.AmountMinor/100, "SUCCESS", "1")
	if rec := pnPost(t, p.handler, p.token, string(old)); rec.Code != http.StatusBadRequest {
		t.Fatalf("expired previous credential admitted: %d", rec.Code)
	}
	if n := pnReceiptCount(t, p); n != 0 {
		t.Fatalf("expired credential left %d receipts", n)
	}
	// The grace boundary itself: previous (v3) and current (v4) both still admit.
	for name, creds := range map[string]accounts.Credentials{"previous v3": v3, "current v4": v4} {
		body := pnSignedBody(t, creds, "mock-account", p.result.MerchantTradeNo, "trade_"+t04Tag(),
			p.result.AmountMinor/100, "SUCCESS", "1")
		rec := pnPost(t, p.handler, p.token, string(body))
		if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
			t.Fatalf("%s signature refused: %d %q", name, rec.Code, rec.Body.String())
		}
	}
	if n := pnReceiptCount(t, p); n != 2 {
		t.Fatalf("grace boundary receipts = %d want 2", n)
	}
	k3NoFacts(t, p)
}

// A second store's connection re-registered with the SAME provider account and the SAME HashKey
// (merchant connected twice) still cannot pull tenant A's trade: the definer maps MerTradeNo only
// inside the endpoint's own connection scope.
func TestK3W401BCrossStoreSameCredentialReplayScoped(t *testing.T) {
	p := pnSetup(t)
	k3PushJob(t, p)
	ctx := context.Background()
	storeB, connB, bindB := randomUUID(), randomUUID(), randomUUID()
	mustExec(t, p.f.owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'k3 cross store','TWD')`, p.f.tenantA, storeB)
	mustExec(t, p.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'integration:manage')`, p.f.tenantA, storeB, p.f.principalA)
	// Seal the same HashKey/HashIV for connection B so the cross-store replay passes its signature check.
	keyID, nonce, ct, err := p.keys.SealPayuni(accounts.PayuniCredentialScope{TenantID: p.f.tenantA,
		StoreID: storeB, ConnectionID: connB, Environment: "SANDBOX", AccountID: "mock-account",
		CredentialVersion: 1}, pqOldSecret)
	if err != nil {
		t.Fatal(err)
	}
	// One transaction: account_current_credential_fk is DEFERRED (head and sealed version commit together).
	tx, err := p.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO integration.bindings(id,tenant_id,store_id,principal_id,provider,external_asset_id) VALUES($1,$2,$3,$4,'payuni','SANDBOX:mock-account')`, bindB, p.f.tenantA, storeB, p.f.principalA); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO integration.merchant_accounts(id,tenant_id,store_id,principal_id,provider,environment,account_id,binding_id,credential_version) VALUES($1,$2,$3,$4,'payuni','SANDBOX','mock-account',$5,1)`, connB, p.f.tenantA, storeB, p.f.principalA, bindB); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO integration.account_credentials(tenant_id,store_id,connection_id,version,key_id,nonce,ciphertext,principal_id) VALUES($1,$2,$3,1,$4,$5,$6,$7)`, p.f.tenantA, storeB, connB, keyID, nonce, ct, p.f.principalA); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	tokenB := randomToken()
	sumB := sha256.Sum256([]byte(tokenB))
	mustExec(t, p.f.owner, `SELECT payments.set_payuni_notify_endpoint($1,$2,$3,$4,$5,'PROVIDER_MOCK',true,$6)`, p.f.tenantA, storeB, p.f.principalA, connB, randomUUID(), sumB[:])

	// Tenant A's genuine signed delivery replayed at store B's endpoint: authentic signature, but
	// the trade belongs to connection A, so B records UNKNOWN_TRADE and nothing of A's moves.
	body := pnValidBody(t, p)
	rec := pnPost(t, p.handler, tokenB, string(body))
	if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Fatalf("cross-store replay: %d %q", rec.Code, rec.Body.String())
	}
	sum := sha256.Sum256(body)
	var disposition string
	var attemptID *string
	if err := p.f.owner.QueryRow(ctx, `SELECT disposition,attempt_id::text FROM payments.payuni_notify_receipts
		WHERE connection_id=$1 AND payload_sha256=$2`, connB, sum[:]).Scan(&disposition, &attemptID); err != nil {
		t.Fatal(err)
	}
	if disposition != "UNKNOWN_TRADE" || attemptID != nil {
		t.Fatalf("cross-store receipt %s attempt=%v", disposition, attemptID)
	}
	if !k3JobFuture(t, p) {
		t.Fatal("cross-store replay woke tenant A's query job")
	}
	var onA int
	if err := p.f.owner.QueryRow(ctx, `SELECT count(*) FROM payments.payuni_notify_receipts WHERE connection_id=$1`, p.account).Scan(&onA); err != nil || onA != 0 {
		t.Fatalf("cross-store replay wrote %d receipts on connection A (err=%v)", onA, err)
	}
	if n := k3ReviewCount(t, p); n != 0 {
		t.Fatalf("cross-store replay opened %d review cases", n)
	}
	k3NoFacts(t, p)
}

// OPS-01B: a disabled store's in-flight money must not be lost — the notify is still recorded and
// the query job still woken (the query path itself decides what to do for a disabled store).
func TestK3W401BDisabledStoreStillRecordsAndWakes(t *testing.T) {
	p := pnSetup(t)
	k3PushJob(t, p)
	mustExec(t, p.f.owner, `UPDATE control.stores SET active=false WHERE tenant_id=$1 AND id=$2`, p.f.tenantA, p.f.storeA1)
	body := pnValidBody(t, p)
	rec := pnPost(t, p.handler, p.token, string(body))
	if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Fatalf("disabled store notify: %d %q", rec.Code, rec.Body.String())
	}
	disposition, attemptID, jobID, _ := pnReceipt(t, p, body)
	if disposition != "QUEUED" || attemptID != p.result.AttemptID || jobID != p.result.JobID {
		t.Fatalf("disabled store receipt %s attempt=%s job=%d", disposition, attemptID, jobID)
	}
	if k3JobFuture(t, p) {
		t.Fatal("disabled store notify did not wake the query job")
	}
	k3NoFacts(t, p)
}

// A disabled endpoint is indistinguishable from an unknown one; token rotation kills the old token;
// LIVE is refused at both the definer and the constructor.
func TestK3W401BEndpointDisableRotateAndLiveRefusal(t *testing.T) {
	p := pnSetup(t)
	body := string(pnValidBody(t, p))

	// A fresh endpoint id each call: the upsert conflict target is (tenant,store,connection,profile),
	// so reusing the existing PK would 23505 instead of rotating in place.
	mustExec(t, p.f.owner, `SELECT payments.set_payuni_notify_endpoint($1,$2,$3,$4,$5,'PROVIDER_MOCK',false,$6)`,
		p.f.tenantA, p.f.storeA1, p.f.principalA, p.account, randomUUID(), p.tokenHash)
	disabled := pnPost(t, p.handler, p.token, body)
	unknown := pnPost(t, p.handler, randomToken(), body)
	if disabled.Code != http.StatusNotFound || unknown.Code != http.StatusNotFound ||
		disabled.Body.String() != unknown.Body.String() {
		t.Fatalf("disabled endpoint is distinguishable from unknown: %d %q vs %d %q",
			disabled.Code, disabled.Body.String(), unknown.Code, unknown.Body.String())
	}

	// Rotate the token: the old one dies, the new one admits the same signed body.
	newToken := randomToken()
	newHash := sha256.Sum256([]byte(newToken))
	mustExec(t, p.f.owner, `SELECT payments.set_payuni_notify_endpoint($1,$2,$3,$4,$5,'PROVIDER_MOCK',true,$6)`,
		p.f.tenantA, p.f.storeA1, p.f.principalA, p.account, randomUUID(), newHash[:])
	if rec := pnPost(t, p.handler, p.token, body); rec.Code != http.StatusNotFound {
		t.Fatalf("rotated-out token admitted: %d", rec.Code)
	}
	if rec := pnPost(t, p.handler, newToken, body); rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Fatalf("rotated token refused a valid delivery: %d %q", rec.Code, rec.Body.String())
	}

	// LIVE is refused at the registrar definer and at handler construction.
	if _, err := p.f.owner.Exec(context.Background(), `SELECT payments.set_payuni_notify_endpoint($1,$2,$3,$4,$5,'LIVE',true,$6)`,
		p.f.tenantA, p.f.storeA1, p.f.principalA, p.account, randomUUID(), newHash[:]); sqlState(err) != "22023" {
		t.Fatalf("LIVE endpoint profile accepted: %v", err)
	}
	for _, profile := range []string{"LIVE", "live", "PRODUCTION", ""} {
		if _, err := payuninotify.NewInbox(context.Background(), p.ingress, p.keys, profile); !errors.Is(err, payuninotify.ErrConfig) {
			t.Fatalf("profile %q admitted at construction: %v", profile, err)
		}
	}
	k3NoFacts(t, p)
}

// The wake touches only the matching payment_query_v1 job's scheduled_at: a second tenant's real
// query job (same kind, same queue table, different operation) stays in the future, the expiry
// family's lane is untouched, and the job count never grows. Orphan decoy rows cannot be inserted
// at all — integration.guard_payment_job_family refuses them with 22023 (verified as a control).
func TestK3W401BWakeOnlyMatchingJobNeverInserts(t *testing.T) {
	p := pnSetup(t)
	ctx := context.Background()
	// An orphan payment_query_v1 job is rejected by the family guard: no decoy can be smuggled in.
	if _, err := p.f.owner.Exec(ctx, `INSERT INTO river_payment.river_job(kind,args,queue,max_attempts)
		VALUES('payment_query_v1',jsonb_build_object('operation_id',$1::text,'version',1),'payment_mock_v1',25)`, randomUUID()); sqlState(err) != "22023" {
		t.Fatalf("orphan payment job insert: want 22023 got %v", err)
	}
	// A second tenant on the same database gets its own real payment_query_v1 job.
	q2 := pqSetupItemsOn(t, p.f, nil, false, 1)
	k3PushJob(t, p)
	mustExec(t, p.f.owner, `UPDATE river_payment.river_job SET scheduled_at=clock_timestamp()+interval '1 hour' WHERE id=$1`, q2.result.JobID)
	expiryImage := k3JobImage(t, p, `SELECT count(*) FROM river_expiry.river_job`)
	var before int
	if err := p.f.owner.QueryRow(ctx, `SELECT count(*) FROM river_payment.river_job`).Scan(&before); err != nil || before != 2 {
		t.Fatalf("jobs before=%d err=%v", before, err)
	}

	body := pnValidBody(t, p)
	if rec := pnPost(t, p.handler, p.token, string(body)); rec.Code != http.StatusOK {
		t.Fatalf("valid notify: %d", rec.Code)
	}
	if k3JobFuture(t, p) {
		t.Fatal("matching job was not woken")
	}
	var otherFuture, after int
	if err := p.f.owner.QueryRow(ctx, `SELECT count(*) FILTER (WHERE scheduled_at > clock_timestamp() AND id<>$1), count(*)
		FROM river_payment.river_job`, p.result.JobID).Scan(&otherFuture, &after); err != nil {
		t.Fatal(err)
	}
	if otherFuture != 1 {
		t.Fatal("wake touched the other tenant's query job")
	}
	if after != before {
		t.Fatalf("notify changed the job count: before=%d after=%d", before, after)
	}
	if got := k3JobImage(t, p, `SELECT count(*) FROM river_expiry.river_job`); got != expiryImage {
		t.Fatalf("notify touched the expiry family lane: before=%d after=%d", expiryImage, got)
	}
	k3NoFacts(t, p)
}

func k3JobImage(t *testing.T, p pnFixture, query string) int {
	t.Helper()
	var n int
	if err := p.f.owner.QueryRow(context.Background(), query).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// A SANDBOX-profile endpoint on the same connection receiving a notify for a PROVIDER_MOCK attempt
// records MISMATCH plus a review case and never wakes the job; a second distinct mismatch payload
// records a second receipt but dedups the review case on its primary key.
func TestK3W401BProfileMismatchReviewedNotWoken(t *testing.T) {
	p := pnSetup(t)
	k3PushJob(t, p)
	sandboxToken := randomToken()
	sandboxHash := sha256.Sum256([]byte(sandboxToken))
	mustExec(t, p.f.owner, `SELECT payments.set_payuni_notify_endpoint($1,$2,$3,$4,$5,'SANDBOX',true,$6)`,
		p.f.tenantA, p.f.storeA1, p.f.principalA, p.account, randomUUID(), sandboxHash[:])
	inbox, err := payuninotify.NewInbox(context.Background(), p.ingress, p.keys, "SANDBOX")
	if err != nil {
		t.Fatal(err)
	}
	handler, err := payuninotify.NewHandler(inbox)
	if err != nil {
		t.Fatal(err)
	}
	post := func(tradeNo string) *httptest.ResponseRecorder {
		body := pnSignedBody(t, pqOldSecret, "mock-account", p.result.MerchantTradeNo, tradeNo,
			p.result.AmountMinor/100, "SUCCESS", "1")
		return pnPost(t, handler, sandboxToken, string(body))
	}
	if rec := post("trade_" + t04Tag()); rec.Code != http.StatusOK {
		t.Fatalf("profile mismatch notify: %d", rec.Code)
	}
	if rec := post("trade_" + t04Tag()); rec.Code != http.StatusOK {
		t.Fatalf("second mismatch notify: %d", rec.Code)
	}
	var mismatches int
	if err := p.f.owner.QueryRow(context.Background(), `SELECT count(*) FROM payments.payuni_notify_receipts
		WHERE connection_id=$1 AND disposition='MISMATCH'`, p.account).Scan(&mismatches); err != nil || mismatches != 2 {
		t.Fatalf("mismatch receipts=%d err=%v", mismatches, err)
	}
	if n := k3ReviewCount(t, p); n != 1 {
		t.Fatalf("review cases=%d want 1 (PK-deduped)", n)
	}
	if !k3JobFuture(t, p) {
		t.Fatal("profile mismatch woke the query job")
	}
	k3NoFacts(t, p)
}

// The ingress login's authority ceiling is exactly the two definers: no direct read or write of any
// business, custody, river or catalog table, and no catalog secret read.
func TestK3W401BIngressAuthorityCeiling(t *testing.T) {
	p := pnSetup(t)
	ctx := context.Background()
	for name, q := range map[string]string{
		"receipts read":     `SELECT count(*) FROM payments.payuni_notify_receipts`,
		"endpoints read":    `SELECT count(*) FROM payments.payuni_notify_endpoints`,
		"endpoints write":   `UPDATE payments.payuni_notify_endpoints SET enabled=false`,
		"receipts write":    `UPDATE payments.payuni_notify_receipts SET disposition='QUEUED'`,
		"accounts read":     `SELECT count(*) FROM integration.merchant_accounts`,
		"orders read":       `SELECT count(*) FROM checkout.orders`,
		"tenants read":      `SELECT count(*) FROM control.tenants`,
		"review insert":     `INSERT INTO payments.review_cases(tenant_id,store_id,attempt_id,reason,source_report_hash) VALUES(gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),'NOTIFY_MISMATCH',NULL)`,
		"river delete":      `DELETE FROM river_payment.river_job WHERE false`,
		"river insert":      `INSERT INTO river_payment.river_job(kind,args,queue,max_attempts) VALUES('payment_query_v1','{}','payment_mock_v1',25)`,
		"catalog secrets":   `SELECT rolpassword FROM pg_authid LIMIT 1`,
		"registrar definer": `SELECT payments.set_payuni_notify_endpoint(gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),'PROVIDER_MOCK',true,decode(repeat('00',32),'hex'))`,
	} {
		if _, err := p.ingress.Exec(ctx, q); sqlState(err) != "42501" {
			t.Errorf("ingress %s: want 42501 got %v", name, err)
		}
	}
}

// Body-size and header gates hold exactly at their boundaries and leave no rows.
func TestK3W401BBodyAndHeaderBoundaries(t *testing.T) {
	p := pnSetup(t)
	path := "/v1/hooks/payuni/notify/" + p.token
	post := func(body string, hdr map[string]string, extraCT ...string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		if _, ok := hdr["Content-Type"]; !ok {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		for _, v := range extraCT {
			req.Header.Add("Content-Type", v)
		}
		rec := httptest.NewRecorder()
		p.handler.ServeHTTP(rec, req)
		return rec
	}
	// Exactly at the 8 KiB bound the body is admitted to verification (and fails the signature with
	// 400); one byte over is 413.
	if rec := post(strings.Repeat("x", 8192), nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("8192-byte body: got %d want 400", rec.Code)
	}
	if rec := post(strings.Repeat("x", 8193), nil); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("8193-byte body: got %d want 413", rec.Code)
	}
	// charset=UTF-8 is a valid form content type (reaches signature verification → 400 on garbage).
	if rec := post("garbage", map[string]string{"Content-Type": "application/x-www-form-urlencoded; charset=UTF-8"}); rec.Code != http.StatusBadRequest {
		t.Fatalf("charset=UTF-8 form: got %d want 400", rec.Code)
	}
	// A doubled Content-Type header is ambiguous: refused.
	if rec := post("garbage", map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, "application/x-www-form-urlencoded"); rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("doubled content type: got %d want 415", rec.Code)
	}
	// No Content-Type at all: refused.
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader("garbage"))
	req.Header.Del("Content-Type")
	rec := httptest.NewRecorder()
	p.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("missing content type: got %d want 415", rec.Code)
	}
	if n := pnReceiptCount(t, p); n != 0 {
		t.Fatalf("boundary probes left %d receipts", n)
	}
}
