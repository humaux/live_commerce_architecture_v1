//go:build browser

package foundation_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Actual form -> production Next -> Go -> isolated PG. Only the hostname/TLS
// edge and merchant/buyer data are synthetic; no business route is mocked.
func TestBrowserBuyerOrderUI(t *testing.T) {
	if os.Getenv("LC_BROWSER_ORDER_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-order")
	}
	h := bhSetup(t)
	mustExec(t, h.f.owner, `UPDATE catalog.products SET name='Synthetic browser order product',description='Synthetic acceptance fixture' WHERE id=$1`, h.stock.product.ID)
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
	}
	// This readback contains only IDs/counts; never snapshots or addresses.
	facts := func(ctx context.Context) ([]orderFact, error) {
		rows, err := h.f.owner.Query(ctx, `SELECT o.id::text,o.owner_id::text,o.commercial_state,o.total_minor,o.currency,o.country,
		 (SELECT count(*) FROM checkout.orders z WHERE z.owner_id=o.owner_id),
		 (SELECT count(*) FROM inventory.reservations z WHERE z.buyer_owner_id=o.owner_id),
		 (SELECT count(*) FROM river.river_job z JOIN checkout.orders q ON q.job_id=z.id WHERE q.owner_id=o.owner_id AND z.kind='checkout_expiry_v1'),
		 (SELECT count(*) FROM checkout.command_results z WHERE z.owner_id=o.owner_id),
		 (SELECT count(*) FROM inventory.ledger z WHERE z.buyer_owner_id=o.owner_id AND z.kind='RESERVE'),
		 (SELECT z.state FROM inventory.reservations z WHERE z.id=o.id)
		 FROM checkout.orders o WHERE o.store_id=$1 ORDER BY o.id`, h.f.storeA1)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []orderFact{}
		for rows.Next() {
			var f orderFact
			if err := rows.Scan(&f.ID, &f.Owner, &f.State, &f.Total, &f.Currency, &f.Country, &f.Orders, &f.Holds, &f.Jobs, &f.Receipts, &f.ReserveLines, &f.HoldState); err != nil {
				return nil, err
			}
			out = append(out, f)
		}
		return out, rows.Err()
	}
	before, err := facts(context.Background())
	if err != nil || len(before) != 0 {
		t.Fatal("order fixture is not empty")
	}
	// Global deltas catch orphan expiry jobs/holds even when no order row exists.
	countQueries := []string{
		`SELECT count(*) FROM checkout.orders`,
		`SELECT count(*) FROM inventory.reservations WHERE buyer_owner_id IS NOT NULL`,
		`SELECT count(*) FROM river.river_job WHERE kind='checkout_expiry_v1'`,
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
			e = h.worker.QueryRow(r.Context(), `SELECT disposition,retry_at FROM checkout.expire_held($1,1)`, id).Scan(&disposition, &retry)
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
		Cases  int      `json:"cases"`
		Orders []string `json:"orders"`
	}
	if json.Unmarshal(data, &result) != nil || result.Cases != 15 || len(result.Orders) != 6 {
		t.Fatal("incomplete browser order evidence")
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
	for _, f := range after {
		if !want[f.ID] || f.Orders != 1 || f.Holds != 1 || f.Jobs != 1 || f.Receipts != 1 || f.ReserveLines != 1 {
			t.Fatal("per-buyer order/hold/job/receipt/ledger gate failed")
		}
		if f.Currency != "USD" || f.Country != "TW" || f.Total <= 0 || (f.State != "DRAFT" && f.State != "CANCELLED") {
			t.Fatal("authoritative order scope/state/amount gate failed")
		}
		if (f.State == "DRAFT" && f.HoldState != "HELD") || (f.State == "CANCELLED" && f.HoldState != "EXPIRED") {
			t.Fatal("persisted order/hold state mismatch")
		}
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
	t.Logf("PASS: actual order UI; cases=%d; buyers=%d; exactly one order/hold/job/receipt/reserve per buyer; evidence=%s", result.Cases, len(after), evidence)
}
