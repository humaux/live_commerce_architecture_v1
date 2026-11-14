//go:build browser

// Purpose: real buyer order browser acceptance with exact completed-case and PostgreSQL fact checks.
// Depends on: production storefront, buyer HTTP fixture, Playwright order-gate.mjs, disposable PG and pure buyer_order_evidence_test.go facts/sample checks.
// Used by: --browser-order and the order step of --browser-webkit.

package foundation_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Actual form -> production Next -> Go -> isolated PG. Only the hostname/TLS
// edge and merchant/buyer data are synthetic; no business route is mocked.
func TestBrowserBuyerOrderUI(t *testing.T) {
	if os.Getenv("LC_BROWSER_ORDER_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-order")
	}
	h := bhSetup(t)
	// /expire keeps the expiry pool live throughout the browser control phase.
	harnessReleaseSetupPools(t, []*pgxpool.Pool{h.f.owner, h.f.runtime, h.a.runtime, h.a.issuer, h.pool, h.expiry}, []*pgxpool.Pool{h.a.identity, h.worker})
	harnessWatchCapacity(t, h.f.owner)
	mustExec(t, h.f.owner, `UPDATE catalog.products SET name='Synthetic browser order product',description='Synthetic acceptance fixture' WHERE id=$1`, h.stock.product.ID)
	sfiAxisBySKUCode(t, h.f.owner, h.stock.product.ID) // the storefront shell sells variants (one chip per SKU code)
	// Separate tenant/store and pre-seeded shipped buyers: all old global baselines below
	// are taken AFTER fixture setup, so no paid/shipped row can replace the seven old orders.
	e := tcvNew(t, tcvOpts{stripe: true, origin: "https://aftersales.example"})
	stopAfterPayments := e.r.startWorker(t)
	e.grantCreator("orders:read", "fulfillment:write", "inventory:write")
	type afterWorld struct {
		Order string `json:"order"`
		Token string `json:"token"`
	}
	worlds := []afterWorld{}
	otherOwnedOrder := ""
	for i := 0; i < 9; i++ {
		b := e.newBuyer()
		order, _ := e.rtPaidOf(b)
		if i < 8 {
			e.rtShip(order)
		} else {
			// Same trusted historical-shipment fixture as BR06, before the tested UI phase.
			tx, err := e.p.f.owner.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			_, err = tx.Exec(context.Background(), `INSERT INTO fulfillment.manual_shipment_versions
			(tenant_id,store_id,owner_id,order_id,version,status,carrier_code,tracking_number,principal_id,recorded_at)
			VALUES($1,$2,$3,$4,1,'SHIPPED','chunghwa_post','SYNTHETIC',$5,clock_timestamp()-interval '337 hours')`, e.tenant(), e.store(), b.cap.Scope.OwnerID, order, e.p.f.principalA)
			if err == nil {
				_, err = tx.Exec(context.Background(), `INSERT INTO fulfillment.manual_shipment_heads(tenant_id,store_id,owner_id,order_id,current_version,updated_at) VALUES($1,$2,$3,$4,1,clock_timestamp()-interval '337 hours')`, e.tenant(), e.store(), b.cap.Scope.OwnerID, order)
			}
			if err == nil {
				_, err = tx.Exec(context.Background(), `UPDATE checkout.orders SET fulfillment_state='MERCHANT_SHIPPED' WHERE id=$1`, order)
			}
			if err != nil {
				_ = tx.Rollback(context.Background())
				t.Fatal(err)
			}
			if err = tx.Commit(context.Background()); err != nil {
				t.Fatal(err)
			}
		}
		worlds = append(worlds, afterWorld{Order: order, Token: b.cap.Token})
		if i == 0 {
			otherOwnedOrder, _ = e.rtPaidOf(b)
			e.rtShip(otherOwnedOrder)
		}
	}
	stopAfterPayments() // fixtures are paid; no background provider work may contaminate old zero-delta assertions
	viewToken := randomToken()
	var owner string
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT owner_id::text FROM checkout.orders WHERE id=$1`, worlds[0].Order).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	mustExec(t, e.p.f.owner, `SELECT buyer.issue_order_capability($1::uuid,$2::uuid,$3::uuid,sha256($4::bytea),3600::bigint)`, e.store(), owner, worlds[0].Order, []byte(viewToken))
	afterMoney := e.pgPaymentFingerprint()
	type afterFact struct {
		Requests int `json:"requests"`
		Receipts int `json:"receipts"`
		Ledger   int `json:"ledger"`
		Refunds  int `json:"refunds"`
	}
	afterFacts := func(order string) afterFact {
		return afterFact{
			Requests: e.count(`SELECT count(*) FROM returns.buyer_requests WHERE store_id=$1 AND order_id=$2`, e.store(), order),
			Receipts: e.count(`SELECT count(*) FROM returns.buyer_request_receipts WHERE store_id=$1 AND response->>'order_id'=$2`, e.store(), order),
			Ledger:   e.rtAll(), Refunds: e.count(`SELECT count(*) FROM payments.stripe_refunds WHERE store_id=$1`, e.store()),
		}
	}
	// Test-only origin dispatcher keeps real Next -> buyer HTTP -> PG on both worlds.
	oldURL, _ := url.Parse(h.server.URL)
	afterURL, _ := url.Parse(e.bh.server.URL)
	oldProxy, afterProxy := httputil.NewSingleHostReverseProxy(oldURL), httputil.NewSingleHostReverseProxy(afterURL)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Commerce-Storefront-Origin") == e.origin {
			if r.Header.Get("X-Commerce-Buyer-BFF-Key") != h.key {
				http.Error(w, "forbidden", 403)
				return
			}
			r.Header.Set("X-Commerce-Buyer-BFF-Key", e.bh.key)
			afterProxy.ServeHTTP(w, r)
			return
		}
		oldProxy.ServeHTTP(w, r)
	}))
	t.Cleanup(upstream.Close)
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
		case r.Method == "GET" && r.URL.Path == "/aftersales/worlds":
			_ = json.NewEncoder(w).Encode(map[string]any{"worlds": worlds, "view_token": viewToken, "other_owned_order": otherOwnedOrder})
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/aftersales/facts/"):
			order := strings.TrimPrefix(r.URL.Path, "/aftersales/facts/")
			known := false
			for _, world := range worlds {
				if world.Order == order {
					known = true
				}
			}
			if !known {
				http.Error(w, "unknown fixture order", 400)
				return
			}
			_ = json.NewEncoder(w).Encode(afterFacts(order))
		case r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/aftersales/decision/"):
			parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/aftersales/decision/"), "/")
			if len(parts) != 2 || !uuidPattern.MatchString(parts[0]) {
				http.Error(w, "invalid fixture decision", 400)
				return
			}
			var order, rma string
			var version int
			err := e.p.f.owner.QueryRow(r.Context(), `SELECT order_id::text,coalesce(return_id::text,''),version FROM returns.buyer_requests WHERE store_id=$1 AND id::text=$2`, e.store(), parts[0]).Scan(&order, &rma, &version)
			if err != nil {
				fail()
				return
			}
			known := false
			for _, world := range worlds {
				if world.Order == order {
					known = true
				}
			}
			if !known {
				fail()
				return
			}
			st := 0
			var out map[string]any
			key := t04Key("browser-after-decision")
			switch parts[1] {
			case "approve", "reject":
				in := map[string]any{"expected_version": version}
				if parts[1] == "reject" {
					in["reason"] = strings.Repeat("<script>synthetic rejection</script>", 20)
				}
				raw, _ := json.Marshal(in)
				st, out, _ = e.mcall(e.token(), "POST", e.rtPath("/return-requests/"+parts[0]+"/"+parts[1]), key, string(raw))
			case "receive":
				st, out = e.rtReceive(e.token(), rma, key, 1, 1)
			case "inspect":
				st, out = e.rtInspect(e.token(), rma, key, 2, 0, 1)
			case "close":
				st, out = e.rtClose(e.token(), rma, key, 3, "")
			case "cancel":
				st, out, _ = e.mcall(e.token(), "POST", e.rtPath("/returns/"+rma+"/cancel"), key, `{"expected_version":1}`)
			default:
				http.Error(w, "invalid fixture action", 400)
				return
			}
			if st != 200 {
				fail()
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "status": out["status"], "state": out["state"]})
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
		"COMMERCE_BUYER_WEB_ENABLED": "1", "COMMERCE_BUYER_API_ORIGIN": upstream.URL, "COMMERCE_BUYER_DEMO_LABEL": "1",
		"COMMERCE_BUYER_BFF_KEY": h.key, "COMMERCE_BUYER_COOKIE_KEY": brToken(), "COMMERCE_BUYER_SESSION_TTL": "3600",
		"LC_BO01_EARLY_HEAD": os.Getenv("LC_BO01_EARLY_HEAD"), "LC_BO01_REPEAT": os.Getenv("LC_BO01_REPEAT"),
		"LC_ORDER_EVIDENCE": evidence, "LC_ORDER_PRODUCT": h.stock.product.ID, "LC_ORDER_CONTROL": control.URL, "LC_ORDER_CONTROL_KEY": controlKey,
	})
	log := browserLog(t, filepath.Join(evidence, "browser.log"))
	cmd.Stdout, cmd.Stderr = log, log
	if err = cmd.Run(); err != nil {
		exitCode, exited, success := -1, false, false
		if cmd.ProcessState != nil {
			exitCode, exited, success = cmd.ProcessState.ExitCode(), cmd.ProcessState.Exited(), cmd.ProcessState.Success()
		}
		_, hasDeadline := ctx.Deadline()
		t.Fatalf("buyer order browser gate failed; error_type=%T exit_code=%d exited=%t success=%t ctx_error=%v ctx_cause=%v has_deadline=%t evidence=%s", err, exitCode, exited, success, ctx.Err(), context.Cause(ctx), hasDeadline, evidence)
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
					Samples []browserOrderCartSample `json:"samples"`
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
		Aftersales struct {
			Cases       int      `json:"cases"`
			Clicks      int      `json:"clicks"`
			Posts       int      `json:"posts"`
			Worlds      int      `json:"worlds"`
			ChildTests  []string `json:"child_tests"`
			ClickLedger []struct {
				CaseID  string          `json:"case_id"`
				Control string          `json:"control"`
				Actual  json.RawMessage `json:"actual"`
				Status  string          `json:"status"`
			} `json:"click_ledger"`
		} `json:"aftersales"`
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
	current, cartReadErr := cartFacts(context.Background(), r.CartID, r.Key)
	var currentItems []cartItem
	itemsError := json.Unmarshal(current.Items, &currentItems)
	if cartReadErr == nil {
		data, marshalError := json.MarshalIndent(current, "", "  ")
		if marshalError != nil || os.WriteFile(filepath.Join(evidence, "cart-refresh-final-pg-facts.json"), data, 0o600) != nil {
			t.Fatal("cannot preserve independent native-focus race PG facts")
		}
	}
	if cartReadErr != nil || !browserOrderCartFactsEqual(current, r.Before) || !browserOrderCartFactsEqual(current, r.After) || current.ID != r.CartID || current.Version != 1 || current.Writes != 1 || current.Receipts != 1 || current.ReceiptVersion != 1 || !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(current.RequestHash) || itemsError != nil || len(currentItems) != 1 || currentItems[0].SKUID != r.SKUID || currentItems[0].Quantity != 1 {
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
		if !browserOrderCartSamplesValid(phase.Surface, m.Samples) {
			t.Fatal("cart samples lack explicit visible loading before first publication or exact line continuity afterward")
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
	// Independent PG reads of the additive phase, with original order assertions above intact.
	if result.Aftersales.Worlds != 6 || result.Aftersales.Cases != 10 || result.Aftersales.Clicks < 60 || result.Aftersales.Posts != 35 || len(result.Aftersales.ChildTests) != 10 {
		t.Fatal("incomplete native aftersales evidence")
	}
	wantAfter := map[string]bool{"AS02 committed ACK loss": true, "AS03 stale CAS winner": true, "AS04 expired window": true, "AS05 owner order view-only denial": true}
	for _, locale := range []string{"zh-TW", "zh-CN", "en"} {
		for _, surface := range []string{"desktop", "mobile"} {
			wantAfter["AS01 "+locale+"/"+surface] = true
		}
	}
	seenAfter := map[string]bool{}
	for _, name := range result.Aftersales.ChildTests {
		if !wantAfter[name] || seenAfter[name] {
			t.Fatal("missing/duplicate aftersales child test")
		}
		seenAfter[name] = true
	}
	if result.Aftersales.Clicks != len(result.Aftersales.ClickLedger) {
		t.Fatal("aftersales click counter/ledger mismatch")
	}
	clickedCases := map[string]bool{}
	for _, row := range result.Aftersales.ClickLedger {
		if !wantAfter[row.CaseID] || row.Control == "" || len(row.Actual) == 0 || string(row.Actual) == "null" || row.Status != "PASS" {
			t.Fatal("invalid aftersales real-control evidence")
		}
		clickedCases[row.CaseID] = true
	}
	if len(clickedCases) != len(wantAfter) {
		t.Fatal("missing aftersales child control evidence")
	}
	finalAfterFacts := []map[string]any{}
	for i, world := range worlds {
		wantRequests, wantReceipts := 4, 8
		if i == 6 {
			wantRequests, wantReceipts = 1, 1
		}
		if i == 7 {
			wantRequests, wantReceipts = 1, 2
		}
		if i == 8 {
			wantRequests, wantReceipts = 0, 0
		}
		f := afterFacts(world.Order)
		finalAfterFacts = append(finalAfterFacts, map[string]any{"order_id": world.Order, "facts": f})
		if f.Requests != wantRequests || f.Receipts != wantReceipts || f.Refunds != 0 {
			t.Fatal("aftersales request/receipt/refund PG mismatch")
		}
		if i < 6 && (e.count(`SELECT count(*) FROM returns.buyer_requests WHERE store_id=$1 AND order_id=$2 AND status='withdrawn'`, e.store(), world.Order) != 1 || e.count(`SELECT count(*) FROM returns.buyer_requests WHERE store_id=$1 AND order_id=$2 AND status='approved'`, e.store(), world.Order) != 2 || e.count(`SELECT count(*) FROM returns.buyer_requests WHERE store_id=$1 AND order_id=$2 AND status='rejected'`, e.store(), world.Order) != 1) {
			t.Fatal("terminal status PG distribution mismatch")
		}
	}
	if e.pgPaymentFingerprint() != afterMoney {
		t.Fatal("aftersales changed payment/order money facts")
	}
	if raw, err := json.MarshalIndent(finalAfterFacts, "", "  "); err != nil || os.WriteFile(filepath.Join(evidence, "aftersales-final-pg-facts.json"), raw, 0o600) != nil {
		t.Fatal("cannot preserve independent aftersales PG facts")
	}
	t.Logf("PASS: actual order UI/history; cases=%d; buyers=%d; orders=%d; exact order/hold/job/receipt/reserve counts (one buyer twice, five once); evidence=%s", result.Cases, len(owners), len(after), evidence)
}
