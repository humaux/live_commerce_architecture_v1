package foundation_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/buyer"
	"livecommerce/internal/buyerhttp"
	"livecommerce/internal/checkout"
	"livecommerce/internal/httperror"
	"livecommerce/internal/storefront"
)

// These tests use real HTTP and isolated PostgreSQL roles/transactions. The
// publication proof, buyer/address and delivery configuration are synthetic;
// they are not production DNS, carrier or payment-provider acceptance.
type bhHarness struct {
	bcHarness
	server      *httptest.Server
	key, origin string
}

type bhResponse struct {
	status int
	header http.Header
	body   []byte
}

func bhPublish(t *testing.T, b bcHarness, origin, tenant, store string) {
	t.Helper()
	mustExec(t, b.f.owner, `INSERT INTO control.storefront_publications(tenant_id,store_id,published) VALUES($1,$2,true)`, tenant, store)
	// The serial foundation harness reuses its base stores. Remove only this
	// test's synthetic admission facts, rather than masking leakage with upsert.
	t.Cleanup(func() {
		mustExec(t, b.f.owner, `DELETE FROM control.storefront_publications WHERE tenant_id=$1 AND store_id=$2`, tenant, store)
	})
	mustExec(t, b.f.owner, `INSERT INTO control.storefront_domains(tenant_id,store_id,origin,state,ownership_verified_at,tls_verified_at,valid_until,evidence_ref)
		VALUES($1,$2,$3,'ACTIVE',clock_timestamp()-interval '1 hour',clock_timestamp()-interval '1 hour',clock_timestamp()+interval '1 hour','SYNTHETIC HTTP gate only')`, tenant, store, origin)
	t.Cleanup(func() { mustExec(t, b.f.owner, `DELETE FROM control.storefront_domains WHERE origin=$1`, origin) })
}

func bhSetup(t *testing.T) bhHarness {
	t.Helper()
	b := bcSetup(t)
	h := bhHarness{bcHarness: b, key: base64.RawURLEncoding.EncodeToString(randomBytes(32)), origin: "https://buyer.example"}
	bhPublish(t, b, h.origin, b.f.tenantA, b.f.storeA1)
	handler, err := buyerhttp.New(context.Background(), b.a.issuer, b.a.runtime, b.service, h.key, time.Hour)
	if err != nil {
		t.Fatal("buyer HTTP constructor failed")
	}
	h.server = httptest.NewServer(handler)
	t.Cleanup(h.server.Close)
	return h
}

func (h bhHarness) request(t *testing.T, method, path, token, key string, input any, edit func(*http.Request)) bhResponse {
	t.Helper()
	var data []byte
	var err error
	if input != nil {
		data, err = json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
	}
	r, err := http.NewRequest(method, h.server.URL+path, bytes.NewReader(data))
	if err != nil {
		t.Fatal("construct HTTP request")
	}
	r.Header.Set("X-Commerce-Buyer-BFF-Key", h.key)
	r.Header.Set("X-Commerce-Storefront-Origin", h.origin)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	if input != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	if edit != nil {
		edit(r)
	}
	client := &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(r)
	if err != nil {
		t.Fatal("HTTP request failed")
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		t.Fatal("HTTP response read failed")
	}
	if res.Header.Get("Cache-Control") != "no-store" || res.Header.Get("X-Content-Type-Options") != "nosniff" || len(res.Header.Get("X-Request-ID")) != 32 {
		t.Fatal("missing safe HTTP response headers")
	}
	return bhResponse{res.StatusCode, res.Header, body}
}

func bhRead[T any](t *testing.T, r bhResponse, status int) T {
	t.Helper()
	var out T
	if r.status != status {
		t.Fatalf("HTTP status=%d want=%d", r.status, status)
	}
	if err := json.Unmarshal(r.body, &out); err != nil {
		t.Fatal("invalid JSON response")
	}
	return out
}

func bhError(t *testing.T, r bhResponse, status int, code string) httperror.Envelope {
	t.Helper()
	e := bhRead[httperror.Envelope](t, r, status)
	if e.Code != code || e.RequestID != r.header.Get("X-Request-ID") || e.Details == nil {
		t.Fatal("unsafe or incorrect error envelope")
	}
	return e
}

func TestBuyerHTTPWorkflowReplayAndProjection(t *testing.T) {
	h := bhSetup(t)
	issued := bhRead[struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}](t, h.request(t, "POST", "/v1/buyer/session", "", "", struct{}{}, nil), 200)
	if len(issued.Token) != 43 || !issued.ExpiresAt.After(time.Now()) {
		t.Fatal("invalid private issuance")
	}
	token := issued.Token
	err := buyer.WithScope(context.Background(), h.a.runtime, token, h.f.storeA1, func(_ context.Context, _ pgx.Tx, s buyer.Scope) error {
		h.cap = buyer.Capability{Token: token, Scope: s}
		return nil
	})
	if err != nil {
		t.Fatal("issued capability did not resolve")
	}
	status := bhRead[struct {
		Authenticated bool `json:"authenticated"`
	}](t, h.request(t, "GET", "/v1/buyer/session", token, "", nil, nil), 200)
	if !status.Authenticated {
		t.Fatal("session was not authenticated")
	}
	empty := bhRead[storefront.Cart](t, h.request(t, "GET", "/v1/buyer/cart", token, "", nil, nil), 200)
	if empty.ID != "" || empty.Items == nil || len(empty.Items) != 0 {
		t.Fatal("new owner cart is not an empty array")
	}
	ci := storefront.CartInput{Items: []storefront.Item{{SKUID: h.stock.skus[0].ID, Quantity: 2}}}
	cartKey := t04Key("http-cart")
	c := bhRead[storefront.Cart](t, h.request(t, "PUT", "/v1/buyer/cart", token, cartKey, ci, nil), 200)
	if c.Version != 1 || len(c.Items) != 1 {
		t.Fatal("cart write projection mismatch")
	}
	replayed := bhRead[storefront.Cart](t, h.request(t, "PUT", "/v1/buyer/cart", token, cartKey, ci, nil), 200)
	if !reflect.DeepEqual(c, replayed) {
		t.Fatal("cart replay changed")
	}
	qi := storefront.QuoteInput{CartVersion: c.Version, MarketID: h.market.ID, Country: "TW", Method: "delivery:" + h.delivery.Code}
	q := bhRead[storefront.Quote](t, h.request(t, "POST", "/v1/buyer/quotes", token, t04Key("http-quote"), qi, nil), 200)
	if q.ID == "" || q.Amount.TotalMinor <= 0 || len(q.Lines) != 1 {
		t.Fatal("quote projection mismatch")
	}
	qRead := bhRead[storefront.Quote](t, h.request(t, "GET", "/v1/buyer/quotes/"+q.ID, token, "", nil, nil), 200)
	if qRead.ID != q.ID || qRead.Amount.TotalMinor != q.Amount.TotalMinor {
		t.Fatal("quote read changed")
	}
	d := bhRead[storefront.Destination](t, h.request(t, "PUT", "/v1/buyer/destination", token, t04Key("http-destination"), bdHome(c), nil), 200)
	dRead := bhRead[storefront.Destination](t, h.request(t, "GET", "/v1/buyer/destinations/"+d.ID, token, "", nil, nil), 200)
	if d.ID == "" || dRead.HomeAddress != d.HomeAddress || dRead.Version != d.Version {
		t.Fatal("destination read changed")
	}
	in := checkout.Input{QuoteID: q.ID, DestinationID: d.ID, CartVersion: c.Version, ServiceVersion: 1, AllocationVersion: 1}
	key := t04Key("http-checkout")
	before := h.facts(t)
	first := h.request(t, "POST", "/v1/buyer/checkout", token, key, in, nil)
	type receipt struct {
		OrderID   string    `json:"order_id"`
		ExpiresAt time.Time `json:"hold_expires_at"`
	}
	out := bhRead[receipt](t, first, 200)
	if out.OrderID == "" || !out.ExpiresAt.After(time.Now()) {
		t.Fatal("checkout receipt missing")
	}
	facts := h.facts(t)
	for i, after := range facts {
		if delta := after - before[i]; delta != 1 {
			t.Fatalf("checkout did not atomically create one fact[%d]: before=%d after=%d", i, before[i], after)
		}
	}
	second := h.request(t, "POST", "/v1/buyer/checkout", token, key, in, nil)
	if second.status != 200 || !bytes.Equal(first.body, second.body) || h.facts(t) != facts {
		t.Fatal("checkout replay added side effects or changed receipt")
	}
	in.CartVersion++
	bhError(t, h.request(t, "POST", "/v1/buyer/checkout", token, key, in, nil), 409, "conflict")
	if h.facts(t) != facts {
		t.Fatal("conflicting replay mutated checkout")
	}
	order := h.request(t, "GET", "/v1/buyer/orders/"+out.OrderID, token, "", nil, nil)
	type orderDisplay struct {
		OrderID          string `json:"order_id"`
		CommercialState  string `json:"commercial_state"`
		FulfillmentState string `json:"fulfillment_state"`
		Snapshot         struct {
			Quote struct {
				Currency string `json:"currency"`
			} `json:"quote"`
			Destination struct {
				RecipientName string `json:"recipient_name"`
			} `json:"destination"`
		} `json:"snapshot"`
	}
	display := bhRead[orderDisplay](t, order, 200)
	if display.OrderID != out.OrderID || display.CommercialState != "DRAFT" || display.Snapshot.Quote.Currency != c.Currency || display.Snapshot.Destination.RecipientName != "Synthetic Buyer" {
		t.Fatal("buyer order display lost required fields")
	}
	for _, forbidden := range []string{"tenant_id", "owner_id", "job_id", "reservation_id", "generation", "warehouse_ids", "allocation", "binding_id", "binding_version", "calculation_version", "policy", "cart_id", "market_id"} {
		if bytes.Contains(order.body, []byte(`"`+forbidden+`"`)) {
			t.Fatalf("internal order field exposed: %s", forbidden)
		}
	}
}

func TestBuyerHTTPOwnerStoreAndRevocation(t *testing.T) {
	h := bhSetup(t)
	order, err := h.begin(t04Key("http-owned-order"))
	if err != nil {
		t.Fatal(err)
	}
	other := mustIssue(t, h.cqHarness.service, h.f.storeA1)
	for _, path := range []string{"/v1/buyer/quotes/" + h.quote.ID, "/v1/buyer/destinations/" + h.destination.ID, "/v1/buyer/orders/" + order.OrderID} {
		bhError(t, h.request(t, "GET", path, other.Token, "", nil, nil), 404, "not_found")
	}
	bhError(t, h.request(t, "GET", "/v1/buyer/cart", h.f.tokens["a"], "", nil, nil), 401, "unauthorized")
	secondOrigin := "https://second.example"
	bhPublish(t, h.bcHarness, secondOrigin, h.f.tenantA, h.f.storeA2)
	bhError(t, h.request(t, "GET", "/v1/buyer/cart", h.cap.Token, "", nil, func(r *http.Request) { r.Header.Set("X-Commerce-Storefront-Origin", secondOrigin) }), 401, "unauthorized")
	// A live origin rebinding must not reuse the old buyer store scope.
	mustExec(t, h.f.owner, `UPDATE control.storefront_domains SET store_id=$2,version=version+1 WHERE origin=$1`, h.origin, h.f.storeA2)
	bhError(t, h.request(t, "GET", "/v1/buyer/cart", h.cap.Token, "", nil, nil), 401, "unauthorized")
	mustExec(t, h.f.owner, `UPDATE control.storefront_domains SET store_id=$2,version=version+1 WHERE origin=$1`, h.origin, h.f.storeA1)
	mustExec(t, h.f.owner, `UPDATE control.storefront_publications SET published=false,version=version+1 WHERE store_id=$1`, h.f.storeA1)
	for _, method := range []string{"GET", "DELETE"} {
		bhError(t, h.request(t, method, "/v1/buyer/session", h.cap.Token, "", nil, nil), 404, "not_found")
	}
	e := bhError(t, h.request(t, "POST", "/v1/buyer/session", "", "", struct{}{}, nil), 404, "not_found")
	if e.Retryable {
		t.Fatal("issuance error suggests retry")
	}
	mustExec(t, h.f.owner, `UPDATE control.storefront_publications SET published=true,version=version+1 WHERE store_id=$1`, h.f.storeA1)
	for i := 0; i < 2; i++ {
		r := h.request(t, "DELETE", "/v1/buyer/session", h.cap.Token, "", nil, nil)
		if r.status != 204 || len(r.body) != 0 {
			t.Fatal("logout is not empty/idempotent")
		}
	}
	bhError(t, h.request(t, "GET", "/v1/buyer/orders/"+order.OrderID, h.cap.Token, "", nil, nil), 401, "unauthorized")
	mustExec(t, h.f.owner, `UPDATE buyer.capability_sessions SET created_at=clock_timestamp()-interval '2 seconds',expires_at=clock_timestamp()-interval '1 second' WHERE id=$1`, other.Scope.SessionID)
	bhError(t, h.request(t, "GET", "/v1/buyer/cart", other.Token, "", nil, nil), 401, "unauthorized")
}

func TestBuyerHTTPAuthorityAndDatabaseFailure(t *testing.T) {
	h := bhSetup(t)
	ctx := context.Background()
	for _, pool := range []*pgxpool.Pool{h.f.owner, h.a.identity, h.a.issuer, h.pool} {
		if _, err := buyerhttp.New(ctx, h.a.issuer, pool, h.service, h.key, time.Hour); err == nil {
			t.Fatal("wrong borrowed buyer authority accepted")
		}
	}
	for _, pool := range []*pgxpool.Pool{h.f.owner, h.a.identity, h.a.runtime, h.pool} {
		if _, err := buyerhttp.New(ctx, pool, h.a.runtime, h.service, h.key, time.Hour); err == nil {
			t.Fatal("wrong borrowed issuer authority accepted")
		}
	}
	if _, err := buyerhttp.New(ctx, h.a.issuer, h.a.runtime, nil, h.key, time.Hour); err == nil {
		t.Fatal("missing checkout service accepted")
	}
	// Constructor failures must not close borrowed pools used by other handlers.
	if h.a.runtime.Ping(ctx) != nil || h.a.issuer.Ping(ctx) != nil {
		t.Fatal("constructor closed borrowed pool")
	}
	// A database failure on session issue uses an explicitly non-retryable safe
	// envelope. No raw driver text, schema or credential appears on the wire.
	h.a.issuer.Close()
	r := h.request(t, "POST", "/v1/buyer/session", "", "", struct{}{}, nil)
	e := bhError(t, r, 503, "unavailable")
	if e.Retryable || strings.Contains(string(r.body), "closed") || strings.Contains(string(r.body), h.key) {
		t.Fatal("issuance failure leaks or suggests retry")
	}
}

func TestBuyerHTTPAPIAssemblyRealPoolCleanup(t *testing.T) {
	h := bhSetup(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, "go", "test", "-race", "-tags", "buyerintegration", "-count=1", "-run", "^TestBuyerPoolAssemblyRealPG$", "-v", "./cmd/api")
	cmd.Dir = root
	// Supply only these fixture authorities; no production COMMERCE_* settings
	// are forwarded. The child invokes the actual unexported API assembly.
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "COMMERCE_") || strings.HasPrefix(name, "LC_BUYER_") {
			continue
		}
		cmd.Env = append(cmd.Env, entry)
	}
	cmd.Env = append(cmd.Env, "LC_BUYER_ASSEMBLY_GATE=1", "LC_BUYER_TEST_ISSUER_DSN="+h.a.issuerURL, "LC_BUYER_TEST_RUNTIME_DSN="+h.a.runtimeURL, "LC_BUYER_TEST_CHECKOUT_DSN="+h.poolURL)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		t.Fatalf("buyer API assembly gate failed: %s", output.String())
	}
	if !strings.Contains(output.String(), "--- PASS: TestBuyerPoolAssemblyRealPG") {
		t.Fatal("child assembly gate did not run")
	}
	t.Log("actual cmd/api buildBuyerHandler: valid roles and runtime/checkout/handler failure connection cleanup PASS")
}
