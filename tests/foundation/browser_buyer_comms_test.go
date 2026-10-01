//go:build browser

package foundation_test

// TestBrowserBuyerComms (unit buyer-comms, independent R4 gate; contracts/storefront-v2.md §E4-E5): a buyer places a bank_transfer order with an e-mail
// in the real storefront shell, the REAL notify worker + the REAL SMTP adapter deliver the placed mail to the loopback SMTP FAKE
// (internal/mail/mailtest), and a fresh browser does the guest lookup with the order number from that mail. Chromium (or WebKit via
// LC_BROWSER_ENGINE), desktop + 390 px, zh-TW + en. Run through `bash scripts/dev/test-local.sh --browser-buyer-comms`.
// Evidence labels: BROWSER, MOCK mailbox (loopback SMTP fake, no real mailbox, no real mail), MOCK edge (synthetic TLS host + CONNECT proxy, X-Forwarded-For
// set by the test edge), no payment provider. The Node side is tests/storefront/buyer-comms-gate.mjs; it never sees a database credential.
// Owner-pool writes (disclosed fixtures): the product/store name for readable mails, the shell's option axis (sfiAxisBySKUCode), the merchant owner's
// address (bcmOwner); everything else is the product: checkout, notify triggers, worker, lookup. Independent PG readback at the end.

import (
	"context"
	"encoding/base64"
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

const bcOrigin = "https://comms.example"

func TestBrowserBuyerComms(t *testing.T) {
	if os.Getenv("LC_BROWSER_BUYER_COMMS_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use scripts/dev/test-local.sh --browser-buyer-comms")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	root, _ := filepath.Abs("../..")
	evidence := brfEvidence(t, root, "buyer-comms")

	g := bgNew(t, tcvOpts{origin: bcOrigin})
	g.cofEnsureSettings(0, true, false, 72)
	f := g.p.f
	const shop = "Morning Comms Shop"
	mustExec(t, f.owner, `UPDATE catalog.products SET name='Synthetic comms product',description='Synthetic acceptance fixture' WHERE id=$1`, g.p.stock.product.ID)
	sfiAxisBySKUCode(t, f.owner, g.p.stock.product.ID) // the storefront shell sells variants (one chip per SKU code)
	mustExec(t, f.owner, `UPDATE control.stores SET name=$2 WHERE id=$1`, g.store(), shop)
	worker := g.worker(100000)

	controlKey := randomToken()
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Gate-Key") != controlKey {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/tick":
			for i := 0; i < 30; i++ {
				n, err := worker.Once(r.Context())
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				if n == 0 {
					break
				}
			}
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/mails":
			out := []map[string]string{}
			for _, x := range g.srv.Messages() {
				out = append(out, map[string]string{"to": x.To, "subject": x.Subject, "text": x.Text, "html": x.HTML})
			}
			_ = json.NewEncoder(w).Encode(out)
		case r.Method == http.MethodGet && r.URL.Path == "/facts":
			var proofs, privacy, withEmail int
			_ = f.owner.QueryRow(r.Context(), `SELECT coalesce(sum(proof_count),0) FROM checkout.bank_transfers WHERE store_id=$1`, g.store()).Scan(&proofs)
			_ = f.owner.QueryRow(r.Context(), `SELECT count(*) FROM customers.privacy_actions WHERE tenant_id=$1`, g.tenant()).Scan(&privacy)
			_ = f.owner.QueryRow(r.Context(), `SELECT count(*) FROM checkout.orders WHERE store_id=$1 AND buyer_email IS NOT NULL`, g.store()).Scan(&withEmail)
			_ = json.NewEncoder(w).Encode(map[string]int{"proofs": proofs, "privacy_actions": privacy, "orders_with_email": withEmail})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(control.Close)

	cmd := exec.CommandContext(ctx, "node", "tests/storefront/buyer-comms-gate.mjs")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	defer func() {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}()
	cmd.Dir = root
	cmd.Env = browserEnvironment(map[string]string{
		"COMMERCE_BUYER_WEB_ENABLED": "1", "COMMERCE_BUYER_API_ORIGIN": g.bh.server.URL, "COMMERCE_BUYER_BFF_KEY": g.bh.key,
		"COMMERCE_BUYER_COOKIE_KEY": base64.RawURLEncoding.EncodeToString(randomBytes(32)), "COMMERCE_BUYER_SESSION_TTL": "3600",
		"LC_BC_EVIDENCE": evidence, "LC_BC_ORIGIN": bcOrigin, "LC_BC_PRODUCT": g.p.stock.product.ID, "LC_BC_SHOP": shop,
		"LC_BC_CONTROL": control.URL, "LC_BC_CONTROL_KEY": controlKey,
	})
	logFile := browserLog(t, filepath.Join(evidence, "buyer-comms-gate.log"))
	cmd.Stdout, cmd.Stderr = logFile, logFile
	if err := cmd.Run(); err != nil {
		t.Fatalf("buyer-comms browser gate failed: %v; evidence=%s", err, evidence)
	}
	out, err := os.ReadFile(filepath.Join(evidence, "buyer-comms-gate.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "cases=19 ") { // 4 runs x BC01-BC05 + BC06 + BC07
		t.Fatalf("the gate did not report all 19 cases; evidence=%s", evidence)
	}

	// ---- independent PG readback -----------------------------------------------------------------------------------------------------------
	n := func(q string, args ...any) int { return countRows(t, f.owner, q, args...) }
	if got := n(`SELECT count(*) FROM checkout.orders WHERE store_id=$1 AND payment_mode='bank_transfer' AND commercial_state='AWAITING_TRANSFER' AND buyer_email IS NOT NULL`, g.store()); got != 4 {
		t.Errorf("%d bank_transfer orders with an e-mail placed through the UI, want 4 (one per locale x viewport)", got)
	}
	if got := n(`SELECT count(*) FROM notify.outbox WHERE store_id=$1 AND kind='placed' AND state='SENT' AND recipient_hash IS NOT NULL`, g.store()); got != 4 {
		t.Errorf("%d placed rows SENT, want 4", got)
	}
	if got := n(`SELECT count(*) FROM notify.outbox WHERE store_id=$1 AND kind='placed'`, g.store()); got != 4 {
		t.Errorf("%d placed rows, want exactly one per order", got)
	}
	if got := n(`SELECT count(*) FROM buyer.capability_sessions WHERE view_order_id IS NOT NULL`); got < 4 {
		t.Errorf("%d view-only sessions, want at least 4 (one per guest lookup)", got)
	}
	if got := n(`SELECT coalesce(sum(proof_count),0) FROM checkout.bank_transfers WHERE store_id=$1`, g.store()); got != 1 {
		t.Errorf("proofs recorded %d, want exactly the one positive-control proof of the order's own session", got)
	}
	if got := n(`SELECT count(*) FROM customers.privacy_actions WHERE tenant_id=$1`, g.tenant()); got != 0 {
		t.Errorf("privacy actions %d, want 0", got)
	}
	if got := len(g.mailsTo(g.owner)); got > 1 {
		t.Errorf("the merchant got %d new-order mails inside one 5 minute window", got)
	}
	t.Logf("BROWSER (MOCK mailbox, MOCK edge) buyer-comms gate passed; evidence=%s", evidence)
}
