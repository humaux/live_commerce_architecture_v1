//go:build browser

// Purpose: real buyer order browser acceptance with exact completed-case and PostgreSQL fact checks.
// Depends on: the production storefront, buyer HTTP fixture, Playwright order-gate.mjs and disposable PostgreSQL.
// Used by: --browser-order and the order step of --browser-webkit.

package foundation_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"
)

type browserOrderCartFact struct {
	ID             string          `json:"id"`
	Version        int64           `json:"version"`
	Writes         int             `json:"writes"`
	Receipts       int             `json:"receipts"`
	ReceiptVersion int64           `json:"receipt_version"`
	RequestHash    string          `json:"request_hash"`
	Items          json.RawMessage `json:"items"`
}

// Node evidence indentation and PostgreSQL jsonb formatting are not cart facts. Every
// scalar and the complete semantic items object still must match the independent read.
func browserOrderCartFactsEqual(a, b browserOrderCartFact) bool {
	var left, right any
	return a.ID == b.ID && a.Version == b.Version && a.Writes == b.Writes && a.Receipts == b.Receipts && a.ReceiptVersion == b.ReceiptVersion && a.RequestHash == b.RequestHash && json.Unmarshal(a.Items, &left) == nil && json.Unmarshal(b.Items, &right) == nil && reflect.DeepEqual(left, right)
}

func TestBuyerOrderRefreshFactComparison(t *testing.T) {
	pg := browserOrderCartFact{ID: "cart", Version: 1, Writes: 1, Receipts: 1, ReceiptVersion: 1, RequestHash: "exact-request-hash", Items: json.RawMessage(`[{"sku_id": "sku", "quantity": 1}]`)}
	node := pg
	node.Items = json.RawMessage(`[
  {"quantity": 1, "sku_id": "sku"}
]`)
	if reflect.DeepEqual(pg, node) || !browserOrderCartFactsEqual(pg, node) {
		t.Fatal("must accept semantic JSON equality while reproducing the former RawMessage byte-comparison failure")
	}
	for name, change := range map[string]func(*browserOrderCartFact){
		"id":              func(f *browserOrderCartFact) { f.ID = "other" },
		"version":         func(f *browserOrderCartFact) { f.Version++ },
		"writes":          func(f *browserOrderCartFact) { f.Writes++ },
		"receipts":        func(f *browserOrderCartFact) { f.Receipts++ },
		"receipt_version": func(f *browserOrderCartFact) { f.ReceiptVersion++ },
		"request_hash":    func(f *browserOrderCartFact) { f.RequestHash = "different" },
		"quantity":        func(f *browserOrderCartFact) { f.Items = json.RawMessage(`[{"sku_id":"sku","quantity":2}]`) },
		"sku":             func(f *browserOrderCartFact) { f.Items = json.RawMessage(`[{"sku_id":"other","quantity":1}]`) },
		"extra_line": func(f *browserOrderCartFact) {
			f.Items = json.RawMessage(`[{"sku_id":"sku","quantity":1},{"sku_id":"other","quantity":1}]`)
		},
		"extra_field": func(f *browserOrderCartFact) {
			f.Items = json.RawMessage(`[{"sku_id":"sku","quantity":1,"unexpected":true}]`)
		},
		"invalid_json": func(f *browserOrderCartFact) { f.Items = json.RawMessage(`[`) },
	} {
		t.Run(name, func(t *testing.T) {
			changed := node
			change(&changed)
			if browserOrderCartFactsEqual(pg, changed) {
				t.Fatal("different persisted fact incorrectly accepted")
			}
		})
	}
}

// Actual form -> production Next -> Go -> isolated PG. Only the hostname/TLS
// edge and merchant/buyer data are synthetic; no business route is mocked.
func TestBrowserBuyerOrderUI(t *testing.T) {
	if os.Getenv("LC_BROWSER_ORDER_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-order")
	}
	h := bhSetup(t)
	mustExec(t, h.f.owner, `UPDATE catalog.products SET name='Synthetic browser order product',description='Synthetic acceptance fixture' WHERE id=$1`, h.stock.product.ID)
	sfiAxisBySKUCode(t, h.f.owner, h.stock.product.ID) // the storefront shell sells variants (one chip per SKU code)
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := os.MkdirTemp(filepath.Join(root, "output/playwright"), "buyer-order-")
	if err != nil {
		t.Fatal(err)
	}
	type orderFact struct {
		ID           string `json:"id"`
		Owner        string `json:"owner"`
		State        string `json:"state"`
		Total        int64  `json:"total"`
		Currency     string `json:"currency"`
		Country      string `json:"country"`
		Orders       int    `json:"orders"`
		Holds        int    `json:"holds"`
		Jobs         int    `json:"jobs"`
		Receipts     int    `json:"receipts"`
		ReserveLines int    `json:"reserve_lines"`
		HoldState    string `json:"hold_state"`
		CartReceipts int    `json:"cart_receipts"`
		SnapshotHash string `json:"snapshot_hash"`
	}
	// This readback contains scalar facts and snapshot digests, never addresses.
	facts := func(ctx context.Context) ([]orderFact, error) {
		rows, err := h.f.owner.Query(ctx, `SELECT o.id::text,o.owner_id::text,o.commercial_state,o.total_minor,o.currency,o.country,
		 (SELECT count(*) FROM checkout.orders z WHERE z.owner_id=o.owner_id),
		 (SELECT count(*) FROM inventory.reservations z WHERE z.buyer_owner_id=o.owner_id),
		 (SELECT count(*) FROM river_expiry.river_job z JOIN checkout.orders q ON q.job_id=z.id WHERE q.owner_id=o.owner_id AND z.kind='checkout_expiry_v1'),
		 (SELECT count(*) FROM checkout.command_results z WHERE z.owner_id=o.owner_id),
		 (SELECT count(*) FROM inventory.ledger z WHERE z.buyer_owner_id=o.owner_id AND z.kind='RESERVE'),
		 (SELECT z.state FROM inventory.reservations z WHERE z.id=o.id),
		 (SELECT count(*) FROM buyer.command_results z WHERE z.owner_id=o.owner_id AND z.operation='cart.set'),md5(o.snapshot::text)
		 FROM checkout.orders o WHERE o.store_id=$1 ORDER BY o.id`, h.f.storeA1)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []orderFact{}
		for rows.Next() {
			var f orderFact
			if err := rows.Scan(&f.ID, &f.Owner, &f.State, &f.Total, &f.Currency, &f.Country, &f.Orders, &f.Holds, &f.Jobs, &f.Receipts, &f.ReserveLines, &f.HoldState, &f.CartReceipts, &f.SnapshotHash); err != nil {
				return nil, err
			}
			out = append(out, f)
		}
		return out, rows.Err()
	}
	type cartItem struct {
		SKUID    string `json:"sku_id"`
		Quantity int64  `json:"quantity"`
	}
	type cartFact = browserOrderCartFact
	// Scope the receipt to the initiating cart's owner/session and exact key; cart.updated
	// is inserted only by the real cart writer transaction, so replay must add none.
	cartFacts := func(ctx context.Context, cartID, key string) (cartFact, error) {
		var out cartFact
		err := h.f.owner.QueryRow(ctx, `SELECT c.id::text,c.version,
		 (SELECT count(*) FROM storefront.events e WHERE e.tenant_id=c.tenant_id AND e.store_id=c.store_id AND e.owner_id=c.owner_id AND e.cart_id=c.id AND e.action='cart.updated'),
		 (SELECT count(*) FROM buyer.command_results r WHERE r.tenant_id=c.tenant_id AND r.store_id=c.store_id AND r.owner_id=c.owner_id AND r.session_id=c.creator_session_id AND r.operation='cart.set' AND r.idempotency_key=$4 AND r.response->>'id'=c.id::text),
		 coalesce((SELECT (r.response->>'version')::bigint FROM buyer.command_results r WHERE r.tenant_id=c.tenant_id AND r.store_id=c.store_id AND r.owner_id=c.owner_id AND r.session_id=c.creator_session_id AND r.operation='cart.set' AND r.idempotency_key=$4 AND r.response->>'id'=c.id::text),0),
		 coalesce((SELECT encode(r.request_hash,'hex') FROM buyer.command_results r WHERE r.tenant_id=c.tenant_id AND r.store_id=c.store_id AND r.owner_id=c.owner_id AND r.session_id=c.creator_session_id AND r.operation='cart.set' AND r.idempotency_key=$4 AND r.response->>'id'=c.id::text),''),
		 coalesce((SELECT jsonb_agg(jsonb_build_object('sku_id',l.sku_id::text,'quantity',l.quantity) ORDER BY l.sku_id) FROM storefront.cart_lines l WHERE l.tenant_id=c.tenant_id AND l.store_id=c.store_id AND l.owner_id=c.owner_id AND l.cart_id=c.id),'[]'::jsonb)
		 FROM storefront.carts c WHERE c.tenant_id=$1 AND c.store_id=$2 AND c.id::text=$3`, h.f.tenantA, h.f.storeA1, cartID, key).
			Scan(&out.ID, &out.Version, &out.Writes, &out.Receipts, &out.ReceiptVersion, &out.RequestHash, &out.Items)
		return out, err
	}
	uuidPattern := regexp.MustCompile(`^[a-f0-9]{8}(-[a-f0-9]{4}){3}-[a-f0-9]{12}$`)
	keyPattern := regexp.MustCompile(`^[A-Za-z0-9_.:-]{8,128}$`)
	jsonEqual := func(a, b []byte) bool {
		var left, right any
		return json.Unmarshal(a, &left) == nil && json.Unmarshal(b, &right) == nil && reflect.DeepEqual(left, right)
	}
	before, err := facts(context.Background())
	if err != nil || len(before) != 0 {
		t.Fatal("order fixture is not empty")
	}
	// Global deltas catch orphan expiry jobs/holds even when no order row exists.
	countQueries := []string{
		`SELECT count(*) FROM checkout.orders`,
		`SELECT count(*) FROM inventory.reservations WHERE buyer_owner_id IS NOT NULL`,
		`SELECT count(*) FROM river_expiry.river_job WHERE kind='checkout_expiry_v1'`,
		`SELECT count(*) FROM checkout.command_results`,
		`SELECT count(*) FROM checkout.payment_attempts`,
		`SELECT count(*) FROM integration.operations`,
	}
	countsBefore := make([]int, len(countQueries))
	for i, q := range countQueries {
		countsBefore[i] = countRows(t, h.f.owner, q)
	}
	controlKey := randomToken()
	// Privileged control is ephemeral loopback, random-key protected, and absent
	// from Next/browser environments. Node never receives an owner database DSN.
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Gate-Key") != controlKey {
			http.Error(w, "forbidden", 403)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		fail := func() { http.Error(w, "fixture control failed", 500) }
		switch {
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/cart-facts/"):
			parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/cart-facts/"), "/")
			if len(parts) != 2 || !uuidPattern.MatchString(parts[0]) || !keyPattern.MatchString(parts[1]) {
				http.Error(w, "invalid fixture cart/key", 400)
				return
			}
			out, e := cartFacts(r.Context(), parts[0], parts[1])
			if e != nil {
				fail()
				return
			}
			_ = json.NewEncoder(w).Encode(out)
		case r.Method == "GET" && r.URL.Path == "/facts":
			out, e := facts(r.Context())
			if e != nil {
				fail()
				return
			}
			_ = json.NewEncoder(w).Encode(out)
		case r.Method == "POST" && (r.URL.Path == "/unpublish" || r.URL.Path == "/publish"):
			_, e := h.f.owner.Exec(r.Context(), `UPDATE control.storefront_publications SET published=$3 WHERE tenant_id=$1 AND store_id=$2`, h.f.tenantA, h.f.storeA1, r.URL.Path == "/publish")
			if e != nil {
				fail()
				return
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
		case r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/expire/"):
			id := strings.TrimPrefix(r.URL.Path, "/expire/")
			var count int
			e := h.f.owner.QueryRow(r.Context(), `SELECT count(*) FROM checkout.orders WHERE id::text=$1 AND store_id=$2`, id, h.f.storeA1).Scan(&count)
			if e != nil || count != 1 {
				http.Error(w, "unknown fixture order", 400)
				return
			}
			for _, table := range []string{"checkout.orders", "inventory.reservations"} {
				if _, e = h.f.owner.Exec(r.Context(), "UPDATE "+table+" SET expires_at=clock_timestamp()-interval '1 second',created_at=clock_timestamp()-interval '901 seconds' WHERE id=$1", id); e != nil {
					fail()
					return
				}
			}
			var disposition string
			var retry *time.Time
			e = h.expiry.QueryRow(r.Context(), `SELECT disposition,retry_at FROM checkout.expire_held($1,1)`, id).Scan(&disposition, &retry)
			if e != nil || disposition != "EXPIRED" {
				fail()
				return
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
		case r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/expire-quote/"):
			id := strings.TrimPrefix(r.URL.Path, "/expire-quote/")
			tag, e := h.f.owner.Exec(r.Context(), `UPDATE storefront.quotes SET created_at=created_at-interval '2 minutes',expires_at=expires_at-interval '2 minutes',snapshot=jsonb_set(jsonb_set(snapshot,'{created_at}',to_jsonb(created_at-interval '2 minutes')),'{expires_at}',to_jsonb(expires_at-interval '2 minutes')) WHERE id::text=$1 AND store_id=$2`, id, h.f.storeA1)
			if e != nil || tag.RowsAffected() != 1 {
				fail()
				return
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
		case r.Method == "POST" && r.URL.Path == "/service-drift":
			input := h.delivery
			input.ExpectedVersion = 1
			input.NameEN = "Synthetic changed delivery"
			if _, e := dsSet(h.cqHarness, t04Key("browser-service-drift"), input); e != nil {
				fail()
				return
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(control.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "tests/storefront/order-gate.mjs")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	defer func() {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}()
	cmd.Dir = root
	cmd.Env = browserEnvironment(map[string]string{
		"COMMERCE_BUYER_WEB_ENABLED": "1", "COMMERCE_BUYER_API_ORIGIN": h.server.URL, "COMMERCE_BUYER_DEMO_LABEL": "1",
		"COMMERCE_BUYER_BFF_KEY": h.key, "COMMERCE_BUYER_COOKIE_KEY": brToken(), "COMMERCE_BUYER_SESSION_TTL": "3600",
		"LC_ORDER_EVIDENCE": evidence, "LC_ORDER_PRODUCT": h.stock.product.ID, "LC_ORDER_CONTROL": control.URL, "LC_ORDER_CONTROL_KEY": controlKey,
	})
	log := browserLog(t, filepath.Join(evidence, "browser.log"))
	cmd.Stdout, cmd.Stderr = log, log
	if err = cmd.Run(); err != nil {
		t.Fatalf("buyer order browser gate failed; evidence=%s", evidence)
	}
	data, err := os.ReadFile(filepath.Join(evidence, "result.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Cases          int      `json:"cases"`
		Orders         []string `json:"orders"`
		RepeatedOrders []string `json:"repeated_orders"`
		Observations   []string `json:"observations"`
		CartRetries    []struct {
			CaseID      string   `json:"case_id"`
			Locale      string   `json:"locale"`
			Surface     string   `json:"surface"`
			Key         string   `json:"key"`
			RequestBody string   `json:"request_body"`
			CartID      string   `json:"cart_id"`
			SKUID       string   `json:"sku_id"`
			Quantity    int64    `json:"quantity"`
			Before      cartFact `json:"before"`
			After       cartFact `json:"after"`
			Reloaded    cartFact `json:"reloaded"`
		} `json:"cart_retries"`
		CartRefreshes []struct {
			CaseID string   `json:"case_id"`
			CartID string   `json:"cart_id"`
			Key    string   `json:"key"`
			SKUID  string   `json:"sku_id"`
			Before cartFact `json:"before"`
			After  cartFact `json:"after"`
			Phases []struct {
				Surface         string `json:"surface"`
				HeldStatus      int    `json:"held_status"`
				SessionStatus   int    `json:"session_status"`
				SessionFailures int    `json:"session_failures"`
				Settlement      struct {
					FetchCount int `json:"fetch_count"`
					LockCount  int `json:"lock_count"`
					Fetch      struct {
						Status   int    `json:"status"`
						State    string `json:"settlement"`
						Sequence int    `json:"sequence"`
					} `json:"fetch"`
					Lock struct {
						Name     string `json:"name"`
						State    string `json:"settlement"`
						Code     string `json:"code"`
						Status   int    `json:"status"`
						Sequence int    `json:"sequence"`
					} `json:"lock"`
					Barrier struct {
						Name     string `json:"name"`
						Acquired int    `json:"acquired_sequence"`
						Settled  int    `json:"settled_sequence"`
					} `json:"barrier"`
				} `json:"settlement"`
				HeldCart struct {
					ID      string     `json:"id"`
					Version int64      `json:"version"`
					Items   []cartItem `json:"items"`
				} `json:"held_cart"`
				Measurement struct {
					Empty    int  `json:"empty"`
					LineLoss int  `json:"line_loss"`
					SeenLine bool `json:"seen_line"`
					Focus    []struct {
						Type    string `json:"type"`
						Trusted bool   `json:"trusted"`
					} `json:"focus"`
					Samples []struct {
						Title *string `json:"title"`
						Lines int     `json:"lines"`
						Qty   *string `json:"qty"`
					} `json:"samples"`
				} `json:"measurement"`
			} `json:"phases"`
		} `json:"cart_refreshes"`
		ClickLedger []struct {
			CaseID   string `json:"case_id"`
			Page     string `json:"page"`
			Control  string `json:"control"`
			Action   string `json:"action"`
			Expected string `json:"expected"`
			Actual   string `json:"actual"`
			Status   string `json:"status"`
		} `json:"click_ledger"`
	}
	const refreshCase = "BC02 native focus failure preserves held real cart on retained drawer and fresh mount"
	if json.Unmarshal(data, &result) != nil {
		t.Fatal("unreadable browser order evidence")
	}
	// Keep the frozen existing order/Retry registry intact; BC02 adds exactly one required observation.
	oldObservations := []string{}
	refreshObservations := 0
	for _, observation := range result.Observations {
		if observation == refreshCase {
			refreshObservations++
		} else {
			oldObservations = append(oldObservations, observation)
		}
	}
	if result.Cases != len(result.Observations) || refreshObservations != 1 || !buyerOrderCasesComplete(result.Cases-1, oldObservations) || len(result.Orders) != 7 || len(result.RepeatedOrders) != 2 || result.RepeatedOrders[0] == result.RepeatedOrders[1] {
		t.Fatal("incomplete browser order evidence")
	}
	if len(result.CartRetries) != len(orderCartRetryCases) || len(result.CartRefreshes) != 1 || len(result.ClickLedger) != 30 {
		t.Fatal("expected six cart Retry cases, one two-phase native-focus race, and 21 old plus nine new real-click ledger rows")
	}
	seenCartCases, seenCartKeys := map[string]bool{}, map[string]bool{}
	cartIDs := map[string]int{}
	finalCartFacts := []cartFact{}
	for _, item := range result.CartRetries {
		wantCase := "BC01 " + item.Locale + " " + item.Surface + " visible cart Retry preserves key/body and one committed write after reload"
		if item.CaseID != wantCase || !orderCartRetryCases[item.CaseID] || seenCartCases[item.CaseID] || seenCartKeys[item.Key] || !keyPattern.MatchString(item.Key) || !uuidPattern.MatchString(item.CartID) || !uuidPattern.MatchString(item.SKUID) {
			t.Fatal("cart Retry evidence is missing, duplicated or not bound to a real cart/key")
		}
		seenCartCases[item.CaseID], seenCartKeys[item.Key] = true, true
		cartIDs[item.CartID]++
		quantity := int64(1)
		if item.Surface == "cart-page" {
			quantity = 2
		}
		var body struct {
			ExpectedVersion int64 `json:"expected_version"`
			Items           []struct {
				SKUID    string `json:"sku_id"`
				Quantity int64  `json:"quantity"`
			} `json:"items"`
		}
		if json.Unmarshal([]byte(item.RequestBody), &body) != nil || item.Quantity != quantity || body.ExpectedVersion != quantity-1 || len(body.Items) != 1 || body.Items[0].SKUID != item.SKUID || body.Items[0].Quantity != quantity {
			t.Fatal("cart Retry request body does not match the initiating visible control")
		}
		for _, f := range []cartFact{item.Before, item.After, item.Reloaded} {
			items, _ := json.Marshal(body.Items)
			if f.ID != item.CartID || f.Version != quantity || f.Writes != int(quantity) || f.Receipts != 1 || f.ReceiptVersion != quantity || len(f.RequestHash) != 64 || f.RequestHash != item.Before.RequestHash || !jsonEqual(f.Items, items) {
				t.Fatal("committed cart changed on visible Retry or reload")
			}
		}
		// Independently re-read PostgreSQL after Node exits; its self-reported facts alone are insufficient.
		current, e := cartFacts(context.Background(), item.CartID, item.Key)
		if e != nil || current.Version != 2 || current.Writes != 2 || current.Receipts != 1 || current.ReceiptVersion != quantity || current.RequestHash != item.Before.RequestHash {
			t.Fatal("final PostgreSQL cart/key receipt or write count mismatch")
		}
		body.Items[0].Quantity = 2
		items, _ := json.Marshal(body.Items)
		if !jsonEqual(current.Items, items) {
			t.Fatal("reloaded cart quantity does not match final PostgreSQL lines")
		}
		finalCartFacts = append(finalCartFacts, current)
	}
	if len(cartIDs) != 3 {
		t.Fatal("each locale must use a fresh cart for drawer and cart-page Retry")
	}
	for _, n := range cartIDs {
		if n != 2 {
			t.Fatal("each locale cart must have one drawer and one cart-page Retry")
		}
	}
	seenClicks := map[string]bool{}
	for _, row := range result.ClickLedger {
		key := row.CaseID + "/" + row.Control
		if (!seenCartCases[row.CaseID] && row.CaseID != refreshCase) || seenClicks[key] || row.Action != "click" || row.Status != "PASS" || row.Expected == "" || row.Actual != row.Expected || !strings.HasPrefix(row.Page, "https://buyer.example/") {
			t.Fatal("incomplete or duplicated cart click ledger")
		}
		seenClicks[key] = true
	}
	for caseID := range orderCartRetryCases {
		controls := []string{"View cart", "Increase quantity", "cart-problem > Retry"}
		if strings.Contains(caseID, " drawer ") {
			controls = []string{"add-to-cart", "header-cart", "cart-problem > Retry", "header-cart after reload"}
		}
		for _, control := range controls {
			if !seenClicks[caseID+"/"+control] {
				t.Fatal("cart Retry ledger omits an initiating, retry or reload verification click")
			}
		}
	}
	for _, control := range []string{"add-to-cart", "drawer held frame button", "drawer held cart text", "drawer failed frame button", "drawer failed cart text", "mount failed frame button", "mount failed cart text", "header-cart", "header-cart after reload"} {
		if !seenClicks[refreshCase+"/"+control] {
			t.Fatal("native-focus race omits a physical focus, cart creation or reload-verification click")
		}
	}
	r := result.CartRefreshes[0]
	if r.CaseID != refreshCase || !uuidPattern.MatchString(r.CartID) || !uuidPattern.MatchString(r.SKUID) || !keyPattern.MatchString(r.Key) || seenCartKeys[r.Key] || len(r.Phases) != 2 {
		t.Fatal("native-focus race is not uniquely bound to a real persisted cart/key")
	}
	current, e := cartFacts(context.Background(), r.CartID, r.Key)
	var currentItems []cartItem
	itemsError := json.Unmarshal(current.Items, &currentItems)
	if e == nil {
		data, marshalError := json.MarshalIndent(current, "", "  ")
		if marshalError != nil || os.WriteFile(filepath.Join(evidence, "cart-refresh-final-pg-facts.json"), data, 0o600) != nil {
			t.Fatal("cannot preserve independent native-focus race PG facts")
		}
	}
	if e != nil || !browserOrderCartFactsEqual(current, r.Before) || !browserOrderCartFactsEqual(current, r.After) || current.ID != r.CartID || current.Version != 1 || current.Writes != 1 || current.Receipts != 1 || current.ReceiptVersion != 1 || !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(current.RequestHash) || itemsError != nil || len(currentItems) != 1 || currentItems[0].SKUID != r.SKUID || currentItems[0].Quantity != 1 {
		t.Fatal("independent PostgreSQL re-read disagrees with the retained/reloaded cart or found an extra write")
	}
	for i, phase := range r.Phases {
		wantSurface, wantFocus := "drawer", 2
		if i == 1 {
			wantSurface, wantFocus = "mount", 1
		}
		m := phase.Measurement
		s := phase.Settlement
		if s.FetchCount != 1 || s.LockCount != 1 || s.Fetch.Status != 503 || s.Fetch.State != "resolved" || s.Lock.Name != "commerce-buyer-session-v1" || s.Lock.State != "rejected" || s.Lock.Code != "request_failed" || s.Lock.Status != 503 || s.Barrier.Name != s.Lock.Name || s.Fetch.Sequence < 1 || s.Fetch.Sequence >= s.Lock.Sequence || s.Lock.Sequence >= s.Barrier.Acquired || s.Barrier.Acquired >= s.Barrier.Settled {
			t.Fatal("failed actual session fetch/native lock rejection did not finish before the queued native release barrier and DOM sample")
		}
		if phase.Surface != wantSurface || phase.HeldStatus != 200 || phase.SessionStatus != 503 || phase.SessionFailures != 1 || phase.HeldCart.ID != current.ID || phase.HeldCart.Version != current.Version || !reflect.DeepEqual(phase.HeldCart.Items, currentItems) || m.Empty != 0 || m.LineLoss != 0 || !m.SeenLine || len(m.Focus) != wantFocus*2 || len(m.Samples) == 0 {
			t.Fatal("native-focus held-response/continuity evidence is incomplete or failed")
		}
		for j, event := range m.Focus {
			wantType := "blur"
			if j%2 == 1 {
				wantType = "focus"
			}
			if !event.Trusted || event.Type != wantType {
				t.Fatal("race was not triggered by the expected new native trusted focus events")
			}
		}
		for j, sample := range m.Samples {
			if i == 1 && j == 0 {
				if sample.Title == nil || *sample.Title != "Loading…" || sample.Lines != 0 {
					t.Fatal("fresh Provider mount never observed the held initial-read loading state")
				}
			} else if sample.Lines != 1 || sample.Qty == nil || *sample.Qty != "1" {
				t.Fatal("visible cart line disappeared or changed during held-read/failure/release")
			}
		}
		if i == 1 && len(m.Samples) < 2 {
			t.Fatal("fresh mount lacks loading-to-line publication proof after release")
		}
	}

	if data, e := json.MarshalIndent(finalCartFacts, "", "  "); e != nil || os.WriteFile(filepath.Join(evidence, "cart-final-pg-facts.json"), data, 0o600) != nil {
		t.Fatal("cannot preserve independent final cart facts")
	}
	after, err := facts(context.Background())
	if err != nil || len(after) != len(result.Orders) {
		t.Fatal("browser/database order count mismatch")
	}
	want := map[string]bool{}
	for _, id := range result.Orders {
		if want[id] {
			t.Fatal("duplicate order reported")
		}
		want[id] = true
	}
	owners := map[string]int{}
	repeatedOwner := ""
	for _, f := range after {
		owners[f.Owner]++
		ownerOrders := 1
		if f.ID == result.RepeatedOrders[0] || f.ID == result.RepeatedOrders[1] {
			ownerOrders = 2
			if repeatedOwner != "" && repeatedOwner != f.Owner {
				t.Fatal("second purchase belongs to a different buyer")
			}
			repeatedOwner = f.Owner
		}
		if !want[f.ID] || f.Orders != ownerOrders || f.Holds != ownerOrders || f.Jobs != ownerOrders || f.Receipts != ownerOrders || f.ReserveLines != ownerOrders {
			t.Fatal("per-buyer order/hold/job/receipt/ledger gate failed")
		}
		if f.Currency != "USD" || f.Country != "TW" || f.Total <= 0 || (f.State != "DRAFT" && f.State != "CANCELLED") {
			t.Fatal("authoritative order scope/state/amount gate failed")
		}
		if (f.State == "DRAFT" && f.HoldState != "HELD") || (f.State == "CANCELLED" && f.HoldState != "EXPIRED") {
			t.Fatal("persisted order/hold state mismatch")
		}
	}
	if !want[result.RepeatedOrders[0]] || !want[result.RepeatedOrders[1]] || len(owners) != 6 || owners[repeatedOwner] != 2 {
		t.Fatal("expected exactly six buyers, one with two distinct orders")
	}
	for i, q := range countQueries {
		want := len(result.Orders)
		if i >= 4 {
			want = 0
		}
		if countRows(t, h.f.owner, q)-countsBefore[i] != want {
			t.Fatal("global order/hold/job/receipt or forbidden payment/integration delta")
		}
	}
	t.Logf("PASS: actual order UI/history; cases=%d; buyers=%d; orders=%d; exact order/hold/job/receipt/reserve counts (one buyer twice, five once); evidence=%s", result.Cases, len(owners), len(after), evidence)
}
