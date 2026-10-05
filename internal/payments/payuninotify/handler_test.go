// handler_test.go: unit tests for request-shape refusals, token-hash routing, signature/rotation
// ordering and the no-ACK-before-commit rule, run against a fake store (MOCK tier). Non-goal: SQL,
// definer and role behavior; those are the foundation REAL_PG gates (NOT_RUN here).
// Callers: go test ./internal/payments/payuninotify/

package payuninotify

import (
	"bufio"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/integrations/psp/payuni"
)

const (
	tenantID = "11111111-1111-4111-8111-111111111111"
	storeID  = "22222222-2222-4222-8222-222222222222"
	connID   = "33333333-3333-4333-8333-333333333333"
	account  = "MerID_Test_1"
	hashKey  = "abcdefghijklmnopqrstuvwxyz123456" // 32 printable ASCII
	hashIV   = "1234567890abcdef"                 // 16 printable ASCII
)

// testToken is a 43-char base64url secret (32 random bytes) used only to select the connection.
var testToken = base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0xAB}, 32))

type fakeStore struct {
	mu          sync.Mutex
	m           material
	found       bool
	matErr      error
	recErr      error
	recorded    []payuni.NotificationAuth
	tokenHashes [][]byte
	payloads    [][]byte
	block       chan struct{}
}

func (f *fakeStore) material(context.Context, []byte) (material, bool, error) {
	return f.m, f.found, f.matErr
}

func (f *fakeStore) record(_ context.Context, tokenHash, payloadSHA []byte, auth payuni.NotificationAuth) (receipt, error) {
	if f.block != nil {
		<-f.block
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.recErr != nil {
		return receipt{}, f.recErr
	}
	f.recorded = append(f.recorded, auth)
	f.tokenHashes = append(f.tokenHashes, tokenHash)
	f.payloads = append(f.payloads, payloadSHA)
	return receipt{Disposition: "QUEUED", ReceiptID: "r1", AttemptID: "a1", TenantID: tenantID, StoreID: storeID, JobID: 1}, nil
}

func keyring(t *testing.T) *accounts.Keyring {
	t.Helper()
	k, err := accounts.NewKeyring("k1", map[string][]byte{"k1": bytes.Repeat([]byte{7}, 32)}, bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// notifyBody signs an inner form exactly as PAYUNi does (AES-256-GCM, HashInfo over
// HashKey+EncryptInfo+HashIV) so the handler's AuthenticateNotification path is exercised for real.
func notifyBody(t *testing.T, inner url.Values, status string) []byte {
	t.Helper()
	block, err := aes.NewCipher([]byte(hashKey))
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCMWithNonceSize(block, 16)
	if err != nil {
		t.Fatal(err)
	}
	sealed := aead.Seal(nil, []byte(hashIV), []byte(inner.Encode()), nil)
	ct, tag := sealed[:len(sealed)-aead.Overhead()], sealed[len(sealed)-aead.Overhead():]
	encryptInfo := hex.EncodeToString([]byte(base64.StdEncoding.EncodeToString(ct) + ":::" + base64.StdEncoding.EncodeToString(tag)))
	sum := sha256.Sum256([]byte(hashKey + encryptInfo + hashIV))
	hashInfo := strings.ToUpper(hex.EncodeToString(sum[:]))
	return []byte(url.Values{"Status": {status}, "MerID": {account}, "Version": {"2.0"},
		"EncryptInfo": {encryptInfo}, "HashInfo": {hashInfo}}.Encode())
}

func baseInner() url.Values {
	return url.Values{"MerID": {account}, "MerTradeNo": {"ORDER_1"}, "TradeNo": {"P123"},
		"TradeAmt": {"100"}, "PaymentType": {"1"}, "Status": {"SUCCESS"}, "TradeStatus": {"1"}}
}

func validBody(t *testing.T) []byte { return notifyBody(t, baseInner(), "SUCCESS") }

func fixture(t *testing.T) (*handler, *fakeStore) {
	t.Helper()
	keys := keyring(t)
	scope := accounts.PayuniCredentialScope{TenantID: tenantID, StoreID: storeID, ConnectionID: connID,
		Environment: "SANDBOX", AccountID: account, CredentialVersion: 1}
	keyID, nonce, ct, err := keys.SealPayuni(scope, accounts.Credentials{HashKey: hashKey, HashIV: hashIV})
	if err != nil {
		t.Fatal(err)
	}
	fs := &fakeStore{found: true, m: material{TenantID: tenantID, StoreID: storeID, ConnectionID: connID,
		Environment: "SANDBOX", AccountID: account, Profile: "SANDBOX", CurVersion: 1, CurKeyID: keyID,
		CurNonce: nonce, CurCiphertext: ct}}
	h, err := NewHandler(&Inbox{store: fs, keys: keys, profile: "SANDBOX", now: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	return h.(*handler), fs
}

func post(h http.Handler, path, body string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func expect(t *testing.T, rec *httptest.ResponseRecorder, status int, body string) {
	t.Helper()
	if rec.Code != status || rec.Body.String() != body || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("got %d %q cache=%q; want %d %q", rec.Code, rec.Body.String(), rec.Header().Get("Cache-Control"), status, body)
	}
}

func TestRequestShapeRefusals(t *testing.T) {
	h, fs := fixture(t)
	get := httptest.NewRecorder()
	h.ServeHTTP(get, httptest.NewRequest(http.MethodGet, routePrefix+testToken, nil))
	expect(t, get, 405, `{"error":"method_not_allowed"}`)
	if get.Header().Get("Allow") != "POST" {
		t.Fatal("405 without Allow: POST")
	}
	for _, p := range []string{"/v1/hooks/payuni/notify", "/v1/hooks/payuni/notify/",
		routePrefix + testToken + "/", routePrefix + strings.Repeat("A", 42) + "+",
		routePrefix + strings.Repeat("A", 42), routePrefix + strings.Repeat("A", 44),
		routePrefix + "%41" + testToken[1:]} {
		expect(t, post(h, p, string(validBody(t)), nil), 404, `{"error":"not_found"}`)
	}
	expect(t, post(h, routePrefix+testToken+"?", string(validBody(t)), nil), 400, `{"error":"invalid_request"}`)
	expect(t, post(h, routePrefix+testToken+"?x=1", string(validBody(t)), nil), 400, `{"error":"invalid_request"}`)
	for _, hdr := range []map[string]string{{"Content-Type": "application/json"},
		{"Content-Type": "application/x-www-form-urlencoded; charset=latin1"}, {"Content-Encoding": "gzip"}} {
		expect(t, post(h, routePrefix+testToken, string(validBody(t)), hdr), 415, `{"error":"unsupported_media_type"}`)
	}
	expect(t, post(h, routePrefix+testToken, strings.Repeat("x", maxBody+1), nil), 413, `{"error":"payload_too_large"}`)
	if len(fs.recorded) != 0 {
		t.Fatal("refused request reached admission")
	}
}

func TestBusyIsNonBlocking(t *testing.T) {
	h, fs := fixture(t)
	fs.block = make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < maxInFlight; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); post(h, routePrefix+testToken, string(validBody(t)), nil) }()
	}
	deadline := time.Now().Add(3 * time.Second)
	for len(h.sem) < maxInFlight && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	rec := post(h, routePrefix+testToken, string(validBody(t)), nil)
	expect(t, rec, 503, `{"error":"busy"}`)
	if rec.Header().Get("Retry-After") != "5" {
		t.Fatal("busy without Retry-After: 5")
	}
	close(fs.block)
	wg.Wait()
}

func TestMaterialAndSignatureOrdering(t *testing.T) {
	h, fs := fixture(t)
	body := string(validBody(t))
	// Valid delivery: token hash routes to the connection, the sealed credential opens, and the
	// signed form authenticates. The ACK is the officially unverified 200 with an empty body.
	rec := post(h, routePrefix+testToken, body, nil)
	expect(t, rec, 200, "")
	if len(fs.recorded) != 1 || fs.recorded[0].MerTradeNo != "ORDER_1" || fs.recorded[0].TradeNo != "P123" ||
		fs.recorded[0].AmountTWD != 100 || fs.recorded[0].Status != "SUCCESS" || fs.recorded[0].TradeStatus != "1" {
		t.Fatalf("recorded auth mismatch: %+v", fs.recorded)
	}
	wantTokenHash := sha256.Sum256([]byte(testToken))
	if !bytes.Equal(fs.tokenHashes[0], wantTokenHash[:]) {
		t.Fatal("store received a token hash other than sha256(token)")
	}
	// A tampered body fails the signature and must not reach admission.
	fs.recorded = nil
	tampered := strings.Replace(body, "SUCCESS", "UNKNOWN", 1)
	expect(t, post(h, routePrefix+testToken, tampered, nil), 400, `{"error":"invalid_signature"}`)
	if len(fs.recorded) != 0 {
		t.Fatal("unsigned or tampered delivery admitted")
	}
	// Unknown or wrong-profile endpoints are invisible (404).
	fs.found = false
	expect(t, post(h, routePrefix+testToken, body, nil), 404, `{"error":"not_found"}`)
	fs.found = true
	fs.m.Profile = "PROVIDER_MOCK"
	expect(t, post(h, routePrefix+testToken, body, nil), 404, `{"error":"not_found"}`)
	fs.m.Profile = "SANDBOX"
	// A credential envelope whose AAD version does not match cannot be opened (503, not 400).
	fs.m.CurVersion = 2
	expect(t, post(h, routePrefix+testToken, body, nil), 503, `{"error":"signing_unavailable"}`)
	fs.m.CurVersion = 1
	fs.matErr = errors.New("db down")
	expect(t, post(h, routePrefix+testToken, body, nil), 503, `{"error":"unavailable"}`)
	fs.matErr = nil
}

func TestRotationGraceOpensPreviousCredential(t *testing.T) {
	h, fs := fixture(t)
	body := string(validBody(t))
	keys := keyring(t)
	scopeV1 := accounts.PayuniCredentialScope{TenantID: tenantID, StoreID: storeID, ConnectionID: connID,
		Environment: "SANDBOX", AccountID: account, CredentialVersion: 1}
	prevKeyID, prevNonce, prevCT, err := keys.SealPayuni(scopeV1, accounts.Credentials{HashKey: hashKey, HashIV: hashIV})
	if err != nil {
		t.Fatal(err)
	}
	// Current envelope is stale (sealed under v1 but presented as v2): the handler must fall back to
	// the previous version and still accept the delivery during the rotation grace window.
	fs.m.CurVersion = 2
	fs.m.PrevVersion = int64p(1)
	fs.m.PrevKeyID = stringp(prevKeyID)
	fs.m.PrevNonce = prevNonce
	fs.m.PrevCiphertext = prevCT
	rec := post(h, routePrefix+testToken, body, nil)
	expect(t, rec, 200, "")
	if len(fs.recorded) != 1 {
		t.Fatalf("rotation grace delivery not recorded: %d", len(fs.recorded))
	}
	// No previous envelope and a stale current envelope is signing_unavailable (503).
	fs.m.PrevVersion, fs.m.PrevKeyID, fs.m.PrevNonce, fs.m.PrevCiphertext = nil, nil, nil, nil
	fs.recorded = nil
	expect(t, post(h, routePrefix+testToken, body, nil), 503, `{"error":"signing_unavailable"}`)
	if len(fs.recorded) != 0 {
		t.Fatal("stale credential admitted without rotation fallback")
	}
}

func TestRotationGraceVerifiesPreviousSignature(t *testing.T) {
	h, fs := fixture(t)
	keys := keyring(t)
	// The current credential is a rotated secret whose envelope opens cleanly, but the callback is
	// still signed with the previous HashKey (the merchant has not yet swapped the provider-side key).
	// The handler must fall back to the previous credential on the signature check itself.
	scopeV2 := accounts.PayuniCredentialScope{TenantID: tenantID, StoreID: storeID, ConnectionID: connID,
		Environment: "SANDBOX", AccountID: account, CredentialVersion: 2}
	curKeyID, curNonce, curCT, err := keys.SealPayuni(scopeV2, accounts.Credentials{HashKey: strings.Repeat("R", 32), HashIV: strings.Repeat("W", 16)})
	if err != nil {
		t.Fatal(err)
	}
	scopeV1 := accounts.PayuniCredentialScope{TenantID: tenantID, StoreID: storeID, ConnectionID: connID,
		Environment: "SANDBOX", AccountID: account, CredentialVersion: 1}
	prevKeyID, prevNonce, prevCT, err := keys.SealPayuni(scopeV1, accounts.Credentials{HashKey: hashKey, HashIV: hashIV})
	if err != nil {
		t.Fatal(err)
	}
	fs.m.CurVersion, fs.m.CurKeyID, fs.m.CurNonce, fs.m.CurCiphertext = 2, curKeyID, curNonce, curCT
	fs.m.PrevVersion, fs.m.PrevKeyID = int64p(1), stringp(prevKeyID)
	fs.m.PrevNonce, fs.m.PrevCiphertext = prevNonce, prevCT
	rec := post(h, routePrefix+testToken, string(validBody(t)), nil) // signed with the previous hashKey/hashIV
	expect(t, rec, 200, "")
	if len(fs.recorded) != 1 {
		t.Fatalf("previous-signature delivery not recorded: %d", len(fs.recorded))
	}
	// Without a previous envelope the stale signature is refused as 400, not silently accepted.
	fs.m.PrevVersion, fs.m.PrevKeyID, fs.m.PrevNonce, fs.m.PrevCiphertext = nil, nil, nil, nil
	fs.recorded = nil
	expect(t, post(h, routePrefix+testToken, string(validBody(t)), nil), 400, `{"error":"invalid_signature"}`)
	if len(fs.recorded) != 0 {
		t.Fatal("stale signature admitted without a previous credential")
	}
}

func TestTradeStatusMalformedIsRejected(t *testing.T) {
	h, fs := fixture(t)
	inner := baseInner()
	inner.Set("TradeStatus", "abc") // AuthenticateNotification leaves TradeStatus unvalidated
	expect(t, post(h, routePrefix+testToken, string(notifyBody(t, inner, "SUCCESS")), nil), 400, `{"error":"invalid_request"}`)
	if len(fs.recorded) != 0 {
		t.Fatal("malformed trade status reached admission")
	}
}

func TestNoACKWithoutCommit(t *testing.T) {
	h, fs := fixture(t)
	body := string(validBody(t))
	fs.recErr = errors.New("commit failed")
	expect(t, post(h, routePrefix+testToken, body, nil), 503, `{"error":"unavailable"}`)
	fs.recErr = nil
	expect(t, post(h, routePrefix+testToken, body, nil), 200, "")
}

func TestLogsCarryNoSecretsBodyOrSignature(t *testing.T) {
	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	defer slog.SetDefault(prev)
	h, fs := fixture(t)
	body := string(validBody(t))
	fs.recErr = errors.New("boom")
	post(h, routePrefix+testToken, body, nil)
	post(h, routePrefix+testToken, "garbage", nil)
	out := logs.String()
	for _, banned := range []string{testToken, "ORDER_1", "P123", hashKey, hashIV, account, "EncryptInfo"} {
		if strings.Contains(out, banned) {
			t.Fatalf("log leaked %q: %s", banned, out)
		}
	}
	if !strings.Contains(out, "payuni_notify_rejected") || !strings.Contains(out, "token_hash") {
		t.Fatal("log lacks the fixed code/token-hash fields")
	}
}

func TestNewHandlerRejectsIncompleteInbox(t *testing.T) {
	if _, err := NewHandler(nil); !errors.Is(err, ErrConfig) {
		t.Fatal("nil inbox accepted")
	}
	if _, err := NewHandler(&Inbox{profile: "LIVE"}); !errors.Is(err, ErrConfig) {
		t.Fatal("LIVE profile accepted")
	}
	if _, err := NewInbox(context.Background(), nil, keyring(t), "SANDBOX"); !errors.Is(err, ErrConfig) {
		t.Fatal("nil pool accepted")
	}
}

// A stalled body must not hold an admission slot or outlive the per-request read deadline.
func TestStalledBodiesDoNotExhaustAdmission(t *testing.T) {
	h, _ := fixture(t)
	h.budget = 400 * time.Millisecond
	srv := httptest.NewServer(h)
	defer srv.Close()
	stalled := func() net.Conn {
		c, err := net.Dial("tcp", srv.Listener.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(c, "POST %s%s HTTP/1.1\r\nHost: x\r\nContent-Type: application/x-www-form-urlencoded\r\nContent-Length: 4096\r\n\r\n", routePrefix, testToken)
		return c
	}
	var conns []net.Conn
	for i := 0; i < maxInFlight+8; i++ {
		conns = append(conns, stalled())
	}
	defer func() {
		for _, c := range conns {
			_ = c.Close()
		}
	}()
	time.Sleep(100 * time.Millisecond) // let the handlers reach the body read
	req, _ := http.NewRequest(http.MethodPost, srv.URL+routePrefix+testToken, strings.NewReader(string(validBody(t))))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("a fully sent signed delivery got %d while %d bodies were stalled", resp.StatusCode, len(conns))
	}
	_ = conns[0].SetReadDeadline(time.Now().Add(3 * time.Second))
	st, err := bufio.NewReader(conns[0]).ReadString('\n')
	if err != nil || !strings.Contains(st, "400") {
		t.Fatalf("stalled body not cut by the read deadline: %q %v", st, err)
	}
}

func int64p(v int64) *int64  { return &v }
func stringp(v string) *string { return &v }
