//go:build browser

package foundation_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"livecommerce/internal/buyerhttp"
	"livecommerce/internal/checkout"
)

// The browser reaches production Next, private Go and task-owned PG. Its only
// PSP endpoint is intercepted in Chromium and never sent to the network.
func TestBrowserBuyerPaymentUI(t *testing.T) {
	if os.Getenv("LC_BROWSER_PAYMENT_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-payment")
	}
	h := hpSetup(t)
	bhPublish(t, h.bcHarness, "https://buyer.example", h.f.tenantA, h.f.storeA1)
	mustExec(t, h.f.owner, `UPDATE catalog.products SET name='Synthetic browser payment product',description='Synthetic acceptance fixture' WHERE id=$1`, h.stock.product.ID)
	bffKey := randomToken()
	handler, err := buyerhttp.New(context.Background(), h.a.issuer, h.a.runtime, h.service, bffKey, time.Hour, h.api.(*checkout.HostedPaymentStarter))
	if err != nil {
		t.Fatal("private payment HTTP fixture")
	}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := os.MkdirTemp(filepath.Join(root, "output/playwright"), "buyer-payment-")
	if err != nil {
		t.Fatal(err)
	}
	controlKey := randomToken()
	// Only scalar facts cross to Node. No database DSN, owner token, form or
	// merchant credential enters the browser process or its log.
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Gate-Key") != controlKey {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != "GET" || r.URL.Path != "/facts" {
			http.NotFound(w, r)
			return
		}
		var out struct {
			Orders   int `json:"orders"`
			Attempts int `json:"attempts"`
			Pages    int `json:"pages"`
			Issued   int `json:"issued"`
		}
		err := h.f.owner.QueryRow(r.Context(), `SELECT
		 (SELECT count(*) FROM checkout.orders WHERE store_id=$1),
		 (SELECT count(*) FROM checkout.payment_attempts WHERE store_id=$1),
		 (SELECT count(*) FROM checkout.hosted_payment_pages p JOIN checkout.payment_attempts a ON a.id=p.attempt_id WHERE a.store_id=$1),
		 (SELECT count(*) FROM checkout.hosted_payment_pages p JOIN checkout.payment_attempts a ON a.id=p.attempt_id WHERE a.store_id=$1 AND p.handed_out_at IS NOT NULL)`, h.f.storeA1).Scan(&out.Orders, &out.Attempts, &out.Pages, &out.Issued)
		if err != nil {
			http.Error(w, "fixture readback failed", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}))
	t.Cleanup(control.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "tests/storefront/buyer-payment-browser.mjs")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	defer func() {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}()
	cmd.Dir = root
	cmd.Env = browserEnvironment(map[string]string{
		"COMMERCE_BUYER_WEB_ENABLED": "1", "COMMERCE_BUYER_API_ORIGIN": server.URL, "COMMERCE_BUYER_DEMO_LABEL": "1",
		"COMMERCE_BUYER_BFF_KEY": bffKey, "COMMERCE_BUYER_COOKIE_KEY": randomToken(), "COMMERCE_BUYER_SESSION_TTL": "3600",
		"LC_PAYMENT_EVIDENCE": evidence, "LC_PAYMENT_PRODUCT": h.stock.product.ID, "LC_PAYMENT_CONTROL": control.URL, "LC_PAYMENT_CONTROL_KEY": controlKey,
	})
	log := browserLog(t, filepath.Join(evidence, "browser.log"))
	cmd.Stdout, cmd.Stderr = log, log
	if err = cmd.Run(); err != nil {
		t.Fatalf("buyer payment browser gate failed; evidence=%s", evidence)
	}
	data, err := os.ReadFile(filepath.Join(evidence, "result.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Cases int `json:"cases"`
		Posts []struct {
			OrderID string `json:"order_id"`
			Digest  string `json:"digest"`
		} `json:"posts"`
	}
	if json.Unmarshal(data, &result) != nil || result.Cases < 8 || len(result.Posts) != 2 || result.Posts[0].OrderID == result.Posts[1].OrderID {
		t.Fatal("incomplete browser payment evidence")
	}
	for _, post := range result.Posts {
		var form []byte
		var attempts, pages, issued, queryJobs, captures int
		err = h.f.owner.QueryRow(context.Background(), `SELECT
		 (SELECT count(*) FROM checkout.payment_attempts a WHERE a.order_id=$1 AND a.store_id=$2),
		 (SELECT count(*) FROM checkout.hosted_payment_pages p JOIN checkout.payment_attempts a ON a.id=p.attempt_id WHERE a.order_id=$1 AND a.store_id=$2),
		 (SELECT count(*) FROM checkout.hosted_payment_pages p JOIN checkout.payment_attempts a ON a.id=p.attempt_id WHERE a.order_id=$1 AND a.store_id=$2 AND p.handed_out_at IS NOT NULL),
		 (SELECT count(*) FROM river_payment.river_job j JOIN checkout.payment_attempts a ON a.job_id=j.id WHERE a.order_id=$1 AND a.store_id=$2 AND j.kind='payment_query_v1'),
		 (SELECT count(*) FROM payments.facts f JOIN checkout.payment_attempts a ON a.id=f.attempt_id WHERE a.order_id=$1 AND a.store_id=$2)`, post.OrderID, h.f.storeA1).Scan(&attempts, &pages, &issued, &queryJobs, &captures)
		if err != nil || attempts != 1 || pages != 1 || issued != 1 || queryJobs != 1 || captures != 0 {
			t.Fatal("browser payment did not persist exactly one pending attempt/page/handoff/query and zero financial reports")
		}
		err = h.f.owner.QueryRow(context.Background(), `SELECT p.form FROM checkout.hosted_payment_pages p JOIN checkout.payment_attempts a ON a.id=p.attempt_id WHERE a.order_id=$1 AND a.store_id=$2`, post.OrderID, h.f.storeA1).Scan(&form)
		if err != nil {
			t.Fatal("missing authoritative hosted page")
		}
		var safe struct {
			Action string `json:"action"`
			Fields struct {
				Version, MerID, EncryptInfo, HashInfo string
			} `json:"fields"`
		}
		if json.Unmarshal(form, &safe) != nil {
			t.Fatal("invalid persisted hosted form")
		}
		canonical := safe.Action + "\n" + safe.Fields.Version + "\n" + safe.Fields.MerID + "\n" + safe.Fields.EncryptInfo + "\n" + safe.Fields.HashInfo
		digest := sha256.Sum256([]byte(canonical))
		if post.Digest != hex.EncodeToString(digest[:]) {
			t.Fatal("browser native POST differs from persisted hosted page")
		}
	}
	var orders, attempts, pages, issued, facts, reviews int
	err = h.f.owner.QueryRow(context.Background(), `SELECT
	 (SELECT count(*) FROM checkout.orders WHERE store_id=$1),
	 (SELECT count(*) FROM checkout.payment_attempts WHERE store_id=$1),
	 (SELECT count(*) FROM checkout.hosted_payment_pages p JOIN checkout.payment_attempts a ON a.id=p.attempt_id WHERE a.store_id=$1),
	 (SELECT count(*) FROM checkout.hosted_payment_pages p JOIN checkout.payment_attempts a ON a.id=p.attempt_id WHERE a.store_id=$1 AND p.handed_out_at IS NOT NULL),
	 (SELECT count(*) FROM payments.facts WHERE store_id=$1),
	 (SELECT count(*) FROM payments.review_cases WHERE store_id=$1)`, h.f.storeA1).
		Scan(&orders, &attempts, &pages, &issued, &facts, &reviews)
	if err != nil || orders != 5 || attempts != 4 || pages != 4 || issued != 4 || facts != 0 || reviews != 0 {
		t.Fatal("browser payment global order/attempt/page/issue or financial-fact counts diverged")
	}
	t.Logf("PASS: actual Next-Go-PG payment UI, two native mock PSP posts, exact persisted form digests; cases=%d evidence=%s", result.Cases, evidence)
}
