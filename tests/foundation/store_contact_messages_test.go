// Purpose: FEAT-CONTACT-STORE acceptance at the existing public buyer and authenticated merchant HTTP boundaries.
// Depends on: foundation PG/HTTP fixtures, buyerhttp, httpapi, notify.Worker and a test-owned MOCK mailer.
// Used by: the CI foundation catch-all shard; written before implementation, with no new production symbols.
// Invariants: synthetic PII only; published-origin authority, encrypted storage, durable replay and one outbox notification.
package foundation_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"

	"livecommerce/internal/httpapi"
	"livecommerce/internal/mail"
	"livecommerce/internal/notify"
)

const contactBuyerPath = "/v1/buyer/contact-messages"

// Test-owned wire types let the tests-only commit compile against the pre-feature tree.
type contactReceipt struct {
	MessageID string `json:"message_id"`
	Status    string `json:"status"`
}

type contactDetail struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	OrderNumber string `json:"order_number"`
	Message     string `json:"message"`
	Status      string `json:"status"`
	Version     int64  `json:"version"`
}

type contactList struct {
	Items      []contactDetail `json:"items"`
	NextCursor *string         `json:"next_cursor"`
}

func contactInput() map[string]any {
	return map[string]any{"name": "測試買家", "email": "buyer@example.test", "order_number": "LC-10482", "message": "Synthetic contact message <b>literal text</b>", "website": ""}
}

func contactEnv(t *testing.T) *tcvEnv {
	t.Helper()
	// Synthetic copies of the existing payload-keyring environment, never live credentials.
	t.Setenv("COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID", "contact-test")
	t.Setenv("COMMERCE_META_PAYLOAD_KEYS_JSON", fmt.Sprintf(`{"keys":[{"id":"contact-test","key_base64":%q}]}`, base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{19}, 32))))
	e := tcvNew(t)
	e.grantCreator("inbox:read", "inbox:reply")
	mustExec(t, e.p.f.owner, `UPDATE control.stores SET contact_email=$3 WHERE tenant_id=$1 AND id=$2`, e.tenant(), e.store(), contactRecipient(e))
	return e
}

func contactRecipient(e *tcvEnv) string { return "contact-" + e.tag + "@merchant.example.test" }

func contactSubmit(t *testing.T, e *tcvEnv, key, ip string, input any) bhResponse {
	t.Helper()
	return e.bh.request(t, http.MethodPost, contactBuyerPath, "", key, input, func(r *http.Request) {
		r.Header.Set("X-Commerce-Client-IP", ip)
	})
}

func contactReceived(t *testing.T, r bhResponse) contactReceipt {
	t.Helper()
	got := bhRead[contactReceipt](t, r, http.StatusCreated)
	var keys map[string]any
	if err := json.Unmarshal(r.body, &keys); err != nil || len(keys) != 2 || got.Status != "received" || len(got.MessageID) != 36 {
		t.Fatal("contact admission must return exactly an opaque message_id and received status, without PII")
	}
	if r.header.Get("Set-Cookie") != "" {
		t.Fatal("public contact admission must not create a buyer session")
	}
	return got
}

func contactAdminPath(store string) string { return "/v1/admin/stores/" + store + "/contact-messages" }

func contactAdmin(e *tcvEnv, token, method, suffix, key string, in any) *httptest.ResponseRecorder {
	var body []byte
	if in != nil {
		body, _ = json.Marshal(in)
	}
	var headers map[string]string
	if key != "" {
		headers = map[string]string{"Idempotency-Key": key}
	}
	contentType := ""
	if in != nil {
		contentType = "application/json"
	}
	return adminRequest(e.merchant, method, contactAdminPath(e.store())+suffix, token, body, contentType, headers)
}

func contactRead[T any](t *testing.T, r *httptest.ResponseRecorder) T {
	t.Helper()
	if r.Code != http.StatusOK {
		t.Fatalf("merchant contact HTTP status=%d, want=200", r.Code)
	}
	if !strings.Contains(r.Header().Get("Cache-Control"), "no-store") || r.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("merchant contact response must be private and non-cacheable")
	}
	var out T
	if err := json.Unmarshal(r.Body.Bytes(), &out); err != nil {
		t.Fatal("merchant contact response is not JSON")
	}
	return out
}

func TestStoreContactPublicPersistenceReplayAndMerchantHandling(t *testing.T) {
	e := contactEnv(t)
	in, key, ip := contactInput(), t04Key("contact-admit"), bgIP()
	first := contactReceived(t, contactSubmit(t, e, key, ip, in))
	if replay := contactReceived(t, contactSubmit(t, e, key, ip, in)); replay != first {
		t.Fatal("same contact command did not recover the original receipt")
	}
	// The same public route also admits an already registered buyer; the bearer never selects the store.
	issued := bhRead[struct {
		Token string `json:"token"`
	}](t, e.bh.request(t, "POST", "/v1/buyer/session", "", "", struct{}{}, nil), 200)
	contactReceived(t, e.bh.request(t, "POST", contactBuyerPath, issued.Token, key, in, func(r *http.Request) {
		r.Header.Set("X-Commerce-Client-IP", ip)
	}))
	changed := contactInput()
	changed["message"] = "A different synthetic question"
	bhError(t, contactSubmit(t, e, key, ip, changed), 409, "conflict")
	list := contactRead[contactList](t, contactAdmin(e, e.token(), "GET", "", "", nil))
	if len(list.Items) != 1 || list.Items[0].ID != first.MessageID || list.NextCursor != nil {
		t.Fatal("merchant contact list lost or duplicated the committed message")
	}
	path := "/" + first.MessageID
	detail := contactRead[contactDetail](t, contactAdmin(e, e.token(), "GET", path, "", nil))
	if detail.ID != first.MessageID || detail.Name != in["name"] || detail.Email != in["email"] || detail.OrderNumber != in["order_number"] || detail.Message != in["message"] || detail.Status != "open" || detail.Version != 0 {
		t.Fatal("merchant detail lost contact fields, changed literal message text or initial version")
	}
	markKey, mark := t04Key("contact-handled"), map[string]any{"expected_version": 0}
	handled := contactRead[contactDetail](t, contactAdmin(e, e.token(), "POST", path+"/handled", markKey, mark))
	if handled.Status != "handled" || handled.Version != 1 {
		t.Fatal("mark-handled did not apply a single versioned transition")
	}
	if replay := contactRead[contactDetail](t, contactAdmin(e, e.token(), "POST", path+"/handled", markKey, mark)); !reflect.DeepEqual(replay, handled) {
		t.Fatal("merchant idempotent retry did not return the original transition")
	}
	if r := contactAdmin(e, e.token(), "POST", path+"/handled", t04Key("contact-stale"), mark); r.Code != 409 {
		t.Fatalf("stale expected_version status=%d, want=409", r.Code)
	}
	// A newly constructed handler must read persistence, not an in-memory response cache.
	e.merchant = httpapi.NewHandler(e.p.f.runtime)
	if got := contactRead[contactDetail](t, contactAdmin(e, e.token(), "GET", path, "", nil)); !reflect.DeepEqual(got, handled) {
		t.Fatal("contact state did not survive handler replacement")
	}
}

func TestStoreContactValidationAndNoClientScope(t *testing.T) {
	e := contactEnv(t)
	cases := []struct {
		field string
		value any
	}{
		{"name", ""}, {"name", "  "}, {"name", strings.Repeat("買", 121)}, {"name", "buyer\r\nheader"},
		{"email", ""}, {"email", "not-an-email"}, {"email", "buyer@example.test\r\nBcc: injected@example.test"}, {"email", strings.Repeat("a", 250) + "@example.test"},
		{"order_number", strings.Repeat("x", 65)}, {"order_number", "LC-10482\x00"},
		{"message", ""}, {"message", " \n\t "}, {"message", strings.Repeat("問", 5001)}, {"message", "question\x00"},
		{"message", 1}, {"website", "https://spam.example.test"},
		{"tenant_id", e.p.f.tenantB}, {"store_id", e.p.f.storeB}, {"unexpected", true},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%02d-%s", i, tc.field), func(t *testing.T) {
			in := contactInput()
			in[tc.field] = tc.value
			r := contactSubmit(t, e, t04Key("contact-invalid"), bgIP(), in)
			bhError(t, r, 422, "invalid_request")
			for _, value := range []string{"buyer@example.test", "injected@example.test", "Synthetic contact message"} {
				if strings.Contains(string(r.body), value) {
					t.Fatal("validation error reflected buyer PII")
				}
			}
		})
	}
	for _, field := range []string{"name", "email", "message"} {
		in := contactInput()
		delete(in, field)
		bhError(t, contactSubmit(t, e, t04Key("contact-missing"), bgIP(), in), 422, "invalid_request")
	}
	bhError(t, contactSubmit(t, e, "", bgIP(), contactInput()), 422, "invalid_request")
	if list := contactRead[contactList](t, contactAdmin(e, e.token(), "GET", "", "", nil)); len(list.Items) != 0 {
		t.Fatal("validation or honeypot refusal committed a contact message")
	}
	// Order number and honeypot are optional; Unicode length is in characters, not bytes.
	in := contactInput()
	delete(in, "order_number")
	delete(in, "website")
	in["name"], in["message"] = strings.Repeat("買", 120), strings.Repeat("問", 5000)
	contactReceived(t, contactSubmit(t, e, t04Key("contact-bounds"), bgIP(), in))
}

func TestStoreContactPublishedOriginAndMerchantPermissions(t *testing.T) {
	e := contactEnv(t)
	first := contactReceived(t, contactSubmit(t, e, t04Key("contact-scope"), bgIP(), contactInput()))
	path := "/" + first.MessageID
	viewer, _ := e.member("inbox:read")
	limited, _ := e.member()
	contactRead[contactDetail](t, contactAdmin(e, viewer, "GET", path, "", nil))
	for _, tc := range []struct {
		token, method, suffix string
		input                 any
		status                int
	}{
		{"", "GET", "", nil, 401}, {limited, "GET", "", nil, 403},
		{limited, "GET", path, nil, 403}, {viewer, "POST", path + "/handled", map[string]any{"expected_version": 0}, 403},
	} {
		if r := contactAdmin(e, tc.token, tc.method, tc.suffix, t04Key("contact-denied"), tc.input); r.Code != tc.status {
			t.Fatalf("contact permission status=%d, want=%d", r.Code, tc.status)
		}
	}
	// A separate tenant and a sibling store both get the same absence as an unknown id.
	foreign := newT06GoFixture(t, e.p.f)
	sibling := randomUUID()
	mustExec(t, e.p.f.owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'Contact sibling','TWD')`, e.tenant(), sibling)
	mustExec(t, e.p.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) SELECT $1,$2,$3,p FROM unnest(ARRAY['store:read','inbox:read','inbox:reply']) p`, e.tenant(), sibling, e.p.f.principalA)
	for _, store := range []string{foreign.store, sibling} {
		r := adminRequest(e.merchant, "GET", contactAdminPath(store)+path, e.token(), nil, "", nil)
		if r.Code != 404 {
			t.Fatalf("foreign store read status=%d, want=404", r.Code)
		}
	}
	r := adminRequest(e.merchant, "GET", contactAdminPath(e.store())+path, foreign.token, nil, "", nil)
	if r.Code != 404 {
		t.Fatalf("foreign tenant read status=%d, want=404", r.Code)
	}
	for _, origin := range []string{"https://unpublished-contact.example.test", ""} {
		r := e.bh.request(t, "POST", contactBuyerPath, "", t04Key("contact-origin"), contactInput(), func(r *http.Request) {
			r.Header.Set("X-Commerce-Storefront-Origin", origin)
			r.Header.Set("X-Commerce-Client-IP", bgIP())
		})
		want := 404
		if origin == "" {
			want = 422
		}
		bhError(t, r, want, map[int]string{404: "not_found", 422: "invalid_request"}[want])
	}
	// Public contact cannot rely on a forged tenant header or an invalid private BFF key.
	for _, header := range []string{"X-Tenant-ID", "X-Commerce-Buyer-BFF-Key"} {
		r := e.bh.request(t, "POST", contactBuyerPath, "", t04Key("contact-header"), contactInput(), func(r *http.Request) { r.Header.Set(header, "untrusted") })
		if r.status != 401 && r.status != 403 {
			t.Fatalf("private boundary header %s admitted status=%d", header, r.status)
		}
	}
}

func TestStoreContactIPAndStoreRateLimitsAndReplay(t *testing.T) {
	e := contactEnv(t)
	ip := bgIP()
	key := t04Key("contact-throttle-replay")
	first := contactReceived(t, contactSubmit(t, e, key, ip, contactInput()))
	for range 4 {
		contactReceived(t, contactSubmit(t, e, t04Key("contact-ip"), ip, contactInput()))
	}
	r := contactSubmit(t, e, t04Key("contact-ip-limit"), ip, contactInput())
	bhError(t, r, 429, "rate_limited")
	if seconds, err := strconv.Atoi(r.header.Get("Retry-After")); err != nil || seconds < 1 || seconds > 600 {
		t.Fatal("contact throttle requires bounded Retry-After")
	}
	if got := contactReceived(t, contactSubmit(t, e, key, ip, contactInput())); got != first {
		t.Fatal("throttle hid an already committed receipt")
	}
	// A different IP is admitted until the independent store cap (100 per ten minutes) is reached.
	for range 95 {
		contactReceived(t, contactSubmit(t, e, t04Key("contact-store"), bgIP(), contactInput()))
	}
	bhError(t, contactSubmit(t, e, t04Key("contact-store-limit"), bgIP(), contactInput()), 429, "rate_limited")
}

func TestStoreContactConcurrentRetryAndEncryptedAtRest(t *testing.T) {
	e := contactEnv(t)
	key, ip := t04Key("contact-race"), bgIP()
	start, results := make(chan struct{}), make(chan bhResponse, 8)
	for range 8 {
		go func() {
			<-start
			results <- contactSubmit(t, e, key, ip, contactInput())
		}()
	}
	close(start)
	var first contactReceipt
	for range 8 {
		got := contactReceived(t, <-results)
		if first.MessageID == "" {
			first = got
		}
		if got != first {
			t.Fatal("concurrent retries returned different contact receipts")
		}
	}
	var raw string
	var count int
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT count(*),min(row_to_json(m)::text) FROM storefront.store_contact_messages m WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()).Scan(&count, &raw); err != nil {
		t.Fatal("contact storage inspection failed")
	}
	if count != 1 {
		t.Fatalf("concurrent contact rows=%d, want=1", count)
	}
	var scopedID, tenant, store string
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT id::text,tenant_id::text,store_id::text FROM storefront.store_contact_messages WHERE id=$1`, first.MessageID).Scan(&scopedID, &tenant, &store); err != nil || scopedID != first.MessageID || tenant != e.tenant() || store != e.store() {
		t.Fatal("contact persisted outside the server-resolved tenant/store")
	}
	for _, secret := range []string{"測試買家", "buyer@example.test", "Synthetic contact message", "LC-10482"} {
		if strings.Contains(raw, secret) {
			t.Fatal("contact PII persisted as plaintext")
		}
	}
	var columns map[string]any
	if err := json.Unmarshal([]byte(raw), &columns); err != nil {
		t.Fatal(err)
	}
	if columns["ciphertext"] == nil || columns["nonce"] == nil || columns["key_id"] == nil {
		t.Fatal("contact row has no authenticated encryption envelope")
	}
	// Ordinary runtime must not bypass the contact definer and read ciphertext or PII directly.
	if _, err := e.p.f.runtime.Exec(context.Background(), `SELECT * FROM storefront.store_contact_messages LIMIT 1`); err == nil {
		t.Fatal("runtime has direct contact-table read authority")
	}
}

type contactMailer struct {
	messages []mail.Message
	unknown  bool
}

func (m *contactMailer) Send(_ context.Context, msg mail.Message) (string, error) {
	m.messages = append(m.messages, msg)
	if m.unknown {
		return "", mail.ErrUnknown
	}
	return "250 MOCK accepted", nil
}

func (m *contactMailer) to(e *tcvEnv) []mail.Message {
	var out []mail.Message
	for _, msg := range m.messages {
		if msg.To == contactRecipient(e) {
			out = append(out, msg)
		}
	}
	return out
}

func contactDrain(t *testing.T, w *notify.Worker) {
	t.Helper()
	// The shared CI database may contain other tests' pending notifications. Drain with MOCK only;
	// assertions below select this fixture's unique recipient, without deleting anyone else's rows.
	for range 40 {
		n, err := w.Once(context.Background())
		if err != nil {
			t.Fatal("contact outbox worker failed")
		}
		if n == 0 {
			return
		}
	}
	t.Fatal("MOCK notification outbox did not drain within 40 batches")
}

func TestStoreContactNotificationOutboxAndNoSensitiveLogs(t *testing.T) {
	e := contactEnv(t)
	var logs bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })
	m := &contactMailer{}
	w, err := notify.NewWorker(e.p.expiry, m, 1000)
	if err != nil {
		t.Fatal(err)
	}
	key, ip := t04Key("contact-mail"), bgIP()
	in := contactInput()
	in["email"] = "buyer-" + e.tag + "@buyers.example.test"
	in["message"] = "Synthetic contact message " + e.tag + " <b>literal text</b>"
	first := contactReceived(t, contactSubmit(t, e, key, ip, in))
	if len(m.messages) != 0 {
		t.Fatal("submission performed inline SMTP instead of committing outbox intent")
	}
	contactDrain(t, w)
	if len(m.to(e)) != 1 {
		t.Fatal("outbox did not notify only the store profile contact_email")
	}
	for _, msg := range m.messages {
		if msg.To == in["email"] {
			t.Fatal("contact submission sent unauthorized outbound buyer mail")
		}
	}
	if strings.Contains(m.to(e)[0].HTML, "<b>literal text</b>") {
		t.Fatal("contact notification rendered buyer text as unescaped HTML")
	}
	if got := contactReceived(t, contactSubmit(t, e, key, ip, in)); got != first {
		t.Fatal("notification retry changed receipt")
	}
	contactDrain(t, w)
	if len(m.to(e)) != 1 {
		t.Fatal("submission replay sent a duplicate notification")
	}
	for _, secret := range []string{in["email"].(string), contactRecipient(e), "測試買家", "Synthetic contact message", "LC-10482"} {
		if strings.Contains(logs.String(), secret) {
			t.Fatal("contact flow logged PII or message body")
		}
	}
}

func TestStoreContactUnknownMailIsNotRetried(t *testing.T) {
	e := contactEnv(t)
	contactReceived(t, contactSubmit(t, e, t04Key("contact-mail-unknown"), bgIP(), contactInput()))
	m := &contactMailer{unknown: true}
	w, err := notify.NewWorker(e.p.expiry, m, 1000)
	if err != nil {
		t.Fatal(err)
	}
	contactDrain(t, w)
	contactDrain(t, w)
	if len(m.to(e)) != 1 {
		t.Fatalf("UNKNOWN contact notification attempts=%d, want=1", len(m.to(e)))
	}
}

func TestStoreContactConcurrentHandlingConflict(t *testing.T) {
	e := contactEnv(t)
	first := contactReceived(t, contactSubmit(t, e, t04Key("contact-handle-race"), bgIP(), contactInput()))
	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan int, 2)
	for range 2 {
		key := t04Key("contact-handle-race")
		wg.Go(func() {
			<-start
			results <- contactAdmin(e, e.token(), "POST", "/"+first.MessageID+"/handled", key, map[string]any{"expected_version": 0}).Code
		})
	}
	close(start)
	wg.Wait()
	counts := map[int]int{(<-results): 1}
	counts[<-results]++
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatalf("contact CAS statuses=%v, want one 200 and one 409", counts)
	}
}
