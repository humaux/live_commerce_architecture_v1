// Purpose: customer import privacy, replay and private result-file HTTP acceptance.
// Depends on: migration-import-v1, real PG fixtures and the full httpapi router.
// Used by: TestCustomerImport in focused and foundation gates.
package foundation_test

// CI01-CI15 (contracts/migration-import-v1.md; unit w5-02b-customer-import; tier REAL_PG over httpapi.NewHandler, MOCK data: every
// name, phone and email is synthetic: +8869000000xx, example.test). Helper prefix `ci`. What it proves:
//   - CI01 preview rolls back, commit creates owner + profile + external id per row, the list shows imported:true with the imported
//     name / phone tail, search by name prefix and phone suffix works, detail works, audit customers.imported, no consent row;
//   - CI02 same file replays (same batch), same file with another count is 409, nothing is imported twice;
//   - CI03 same external id in a new file updates the profile and keeps the owner;
//   - CI04 a consent column is ignored (flagged in the preview), zero consent_events, consents stay not granted;
//   - CI05 invalid phone / over-long name / formula cells: row failures, unguard before storing, guard in results.csv;
//   - CI06 customers:privacy is required (customers:write and read-only are 403), no cross-tenant matching;
//   - CI07 erasure removes profile + external id, scrubs the id from retained results, replay_erasures re-deletes;
//   - CI08 5001 rows, 2 MiB + 1, Big5, nothing_to_apply, preview_stale (nothing written), 5000 rows inside the budget;
//   - CI09 no name / phone / email in results.csv, audit rows, receipts, batch rows or logs;
//   - CI10 an imported customer can be tagged and noted (W6-01B visibility rule extended);
//   - CI11 no login role holds a direct grant on the new tables or can insert owners;
//   - CI07b erasure leaves a salted tombstone: a re-import of the stale export refuses the erased id (`erased`), shows no id anywhere;
//   - CI12 the merchant privacy export contains the imported profile (phone, email, external id) and no receipt does;
//   - CI13 the owner-insert privilege of commerce_privacy_writer cannot create an owner in another store or tenant;
//   - CI14 two parallel commits of one file import it once (one replay), one owner per row;
//   - CI15 the commerce_auth read policy of import_profiles is store-scoped; a phone- or email-looking external id is refused.
// Disclosed owner-pool fixtures: store grants of the merchant principals (lcPrincipal) and the final purge of the import rows
// this test created in the shared stores A1 and B.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/command"
	"livecommerce/internal/httpapi"
)

type ciEnv struct {
	*ctEnv
	srv   *httptest.Server
	store string
	otok  string // tenant B, customers:privacy on store B
	logs  *bytes.Buffer
}

func ciSetup(t *testing.T) *ciEnv {
	t.Helper()
	c := &ciEnv{ctEnv: ctSetup(t), logs: &bytes.Buffer{}}
	_, c.otok = lcPrincipal(t, c.f, c.f.tenantB, []string{c.f.storeB}, "store:read", "customers:read", "customers:privacy")
	prevWriter, prevSlog := log.Writer(), slog.Default()
	log.SetOutput(c.logs)
	slog.SetDefault(slog.New(slog.NewTextHandler(c.logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	c.srv = httptest.NewServer(httpapi.NewHandler(c.f.runtime))
	c.store = c.f.storeA1
	t.Cleanup(func() {
		c.srv.Close()
		log.SetOutput(prevWriter)
		slog.SetDefault(prevSlog)
		for _, q := range []string{`DELETE FROM migrationimport.batches WHERE tenant_id=ANY($1)`, `DELETE FROM migrationimport.external_ids WHERE tenant_id=ANY($1)`,
			`DELETE FROM customers.import_profiles WHERE tenant_id=ANY($1)`} {
			mustExec(t, c.f.owner, q, []string{c.f.tenantA, c.f.tenantB})
		}
	})
	return c
}

// call sends one request to the importer routes of store (default A1) and returns status, body and headers.
func (c *ciEnv) call(method, path, token, body string, headers ...string) (int, []byte, http.Header) {
	c.t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, c.srv.URL+path, rd)
	if err != nil {
		c.t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if method == "POST" {
		req.Header.Set("Content-Type", "text/csv")
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	return res.StatusCode, raw, res.Header
}

func (c *ciEnv) imports(store string) string { return "/v1/admin/stores/" + store + "/imports" }

func (c *ciEnv) preview(token, body, query string) (int, map[string]any) {
	st, raw, _ := c.call("POST", c.imports(c.store)+"/customers/preview"+query, token, body)
	return st, sraJSON(raw)
}

func (c *ciEnv) commit(token, body string, expected int, extra string) (int, map[string]any) {
	st, raw, _ := c.call("POST", fmt.Sprintf("%s/customers/commit?expected_apply_rows=%d%s", c.imports(c.store), expected, extra), token, body)
	return st, sraJSON(raw)
}

func (c *ciEnv) n(query string, args ...any) int { return c.count(query, args...) }

// list returns the merchant customer list of A1 (limit 100) keyed by customer id.
func (c *ciEnv) list(query string) map[string]map[string]any {
	c.t.Helper()
	st, raw, _ := c.call("GET", "/v1/admin/stores/"+c.store+"/customers?limit=100"+query, c.adminTok, "")
	if st != 200 {
		c.t.Fatalf("list customers: %d %s", st, raw)
	}
	out := map[string]map[string]any{}
	for _, it := range sraJSON(raw)["items"].([]any) {
		m := it.(map[string]any)
		out[m["customer_id"].(string)] = m
	}
	return out
}

func (c *ciEnv) ownerOf(external string) string {
	c.t.Helper()
	var id string
	if err := c.f.owner.QueryRow(context.Background(), `SELECT internal_id::text FROM migrationimport.external_ids WHERE tenant_id=$1 AND store_id=$2 AND kind='customers' AND external_id=$3`,
		c.f.tenantA, c.store, external).Scan(&id); err != nil {
		c.t.Fatalf("owner of %s: %v", external, err)
	}
	return id
}

func ciFixture(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("testdata/shopline_customers_synthetic.csv")
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestCustomerImport(t *testing.T) {
	c := ciSetup(t)
	f := c.f
	file := ciFixture(t)
	const secretName, secretPhone, secretEmail = "測試顧客一", "0900-000-001", "cust1@example.test"
	var firstBatch string
	var owner1 string

	t.Run("CI01 preview rolls back, commit imports, list shows imported customers", func(t *testing.T) {
		st, pv := c.preview(c.adminTok, file, "")
		if st != 200 || pv["rows_total"] != float64(3) || pv["new_rows"] != float64(3) || pv["update_rows"] != float64(0) || pv["failed_rows"] != float64(0) || pv["apply_rows"] != float64(3) {
			t.Fatalf("preview: %d %v", st, pv)
		}
		m := pv["mapping"].(map[string]any)
		if m["external_id"] != "顧客編號" || m["name"] != "姓名" || m["phone"] != "手機" || m["email"] != "電子郵件" || m["consent"] != "同意行銷" {
			t.Fatalf("auto mapping: %v", m)
		}
		if c.n(`SELECT count(*) FROM customers.import_profiles WHERE tenant_id=$1`, f.tenantA) != 0 || c.n(`SELECT count(*) FROM migrationimport.external_ids WHERE tenant_id=$1`, f.tenantA) != 0 {
			t.Fatal("preview wrote rows")
		}
		ownersBefore := c.n(`SELECT count(*) FROM buyer.owners WHERE tenant_id=$1`, f.tenantA)
		st, cm := c.commit(c.adminTok, file, 3, "")
		firstBatch, _ = cm["batch_id"].(string)
		if st != 200 || !command.ValidID(firstBatch) || cm["created"] != float64(3) || cm["updated"] != float64(0) || cm["failed"] != float64(0) || cm["replayed"] != false {
			t.Fatalf("commit: %d %v", st, cm)
		}
		if c.n(`SELECT count(*) FROM buyer.owners WHERE tenant_id=$1`, f.tenantA)-ownersBefore != 3 ||
			c.n(`SELECT count(*) FROM customers.import_profiles WHERE tenant_id=$1 AND store_id=$2 AND source='shopline_csv'`, f.tenantA, c.store) != 3 ||
			c.n(`SELECT count(*) FROM migrationimport.external_ids WHERE tenant_id=$1 AND store_id=$2 AND kind='customers'`, f.tenantA, c.store) != 3 {
			t.Fatal("commit did not create 3 owners + profiles + external ids")
		}
		owner1 = c.ownerOf("SL-0001")
		if c.n(`SELECT count(*) FROM buyer.capability_sessions WHERE owner_id=$1`, owner1) != 0 || c.n(`SELECT count(*) FROM buyer.owners WHERE id=$1 AND active`, owner1) != 1 {
			t.Fatal("an imported owner must be an active owner without a capability session")
		}
		var phone, email, name string
		if err := f.owner.QueryRow(context.Background(), `SELECT phone_e164,email,display_name FROM customers.import_profiles WHERE owner_id=$1`, owner1).Scan(&phone, &email, &name); err != nil ||
			phone != "+886900000001" || email != secretEmail || name != secretName {
			t.Fatalf("profile: %q %q %q %v", phone, email, name, err)
		}
		if c.n(`SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='customers.imported'`, f.tenantA, c.store) != 1 {
			t.Fatal("commit wrote no single customers.imported audit row")
		}
		rows := c.list("")
		row, ok := rows[owner1]
		if !ok || row["imported"] != true || row["display_name"] != secretName || row["phone_last3"] != "001" || row["orders_count"] != float64(0) || row["active"] != true {
			t.Fatalf("list row: %v", row)
		}
		if consents := row["consents"].(map[string]any); consents["marketing_messages"] != false || consents["ads_personalization"] != false {
			t.Fatalf("imported consent must be unknown/not granted: %v", consents)
		}
		// Search: name prefix, local-format phone (the +886 profile is searched as 09...), phone tail.
		for _, q := range []string{"&q=" + url.QueryEscape("測試顧客"), "&q=0900000001", "&q=000001"} {
			if _, ok := c.list(q)[owner1]; !ok {
				t.Fatalf("search %q did not find the imported customer", q)
			}
		}
		if _, ok := c.list("&q=nobody-xyz")[owner1]; ok {
			t.Fatal("search matched the wrong customer")
		}
		st, raw, _ := c.call("GET", "/v1/admin/stores/"+c.store+"/customers/"+owner1, c.adminTok, "")
		d := sraJSON(raw)
		if st != 200 || d["imported"] != true || d["display_name"] != secretName || len(d["orders"].([]any)) != 0 {
			t.Fatalf("detail: %d %s", st, raw)
		}
	})

	t.Run("CI02 same file replays, a different count conflicts", func(t *testing.T) {
		ownersBefore := c.n(`SELECT count(*) FROM buyer.owners WHERE tenant_id=$1`, f.tenantA)
		st, cm := c.commit(c.adminTok, file, 3, "")
		if st != 200 || cm["replayed"] != true || cm["batch_id"] != firstBatch || cm["created"] != float64(3) {
			t.Fatalf("replay: %d %v", st, cm)
		}
		if st, body := c.commit(c.adminTok, file, 2, ""); st != 409 || body["code"] != "idempotency_conflict" {
			t.Fatalf("different count: %d %v", st, body)
		}
		if st, body := c.commit(c.adminTok, file, 3, "&mapping="+`{"phone":""}`); st != 409 || body["code"] != "idempotency_conflict" {
			t.Fatalf("different mapping: %d %v", st, body)
		}
		if c.n(`SELECT count(*) FROM buyer.owners WHERE tenant_id=$1`, f.tenantA) != ownersBefore || c.n(`SELECT count(*) FROM migrationimport.batches WHERE tenant_id=$1 AND store_id=$2`, f.tenantA, c.store) != 1 {
			t.Fatal("a replay or conflict imported something")
		}
	})

	t.Run("CI03 the same external id updates, it never creates a second owner", func(t *testing.T) {
		second := "顧客編號,姓名,手機\nSL-0001,測試顧客一改名,0900000011\nSL-0004,Test Customer Four,0900000004\n"
		st, pv := c.preview(c.adminTok, second, "")
		if st != 200 || pv["new_rows"] != float64(1) || pv["update_rows"] != float64(1) || pv["apply_rows"] != float64(2) {
			t.Fatalf("preview: %d %v", st, pv)
		}
		ownersBefore := c.n(`SELECT count(*) FROM buyer.owners WHERE tenant_id=$1`, f.tenantA)
		st, cm := c.commit(c.adminTok, second, 2, "")
		if st != 200 || cm["created"] != float64(1) || cm["updated"] != float64(1) {
			t.Fatalf("commit: %d %v", st, cm)
		}
		if c.n(`SELECT count(*) FROM buyer.owners WHERE tenant_id=$1`, f.tenantA)-ownersBefore != 1 || c.ownerOf("SL-0001") != owner1 {
			t.Fatal("the update created a second owner or moved the external id")
		}
		var name, phone string
		var email *string
		if err := f.owner.QueryRow(context.Background(), `SELECT display_name,phone_e164,email FROM customers.import_profiles WHERE owner_id=$1`, owner1).Scan(&name, &phone, &email); err != nil ||
			name != "測試顧客一改名" || phone != "+886900000011" || email == nil || *email != secretEmail {
			t.Fatalf("updated profile (an unmapped email column keeps the stored value): %q %q %v %v", name, phone, email, err)
		}
		// A mapped column with an empty cell clears the stored value.
		clear := "customer_id,name,phone\nSL-0004,Test Customer Four,\n"
		if st, cm := c.commit(c.adminTok, clear, 1, ""); st != 200 || cm["updated"] != float64(1) {
			t.Fatalf("clearing commit: %d %v", st, cm)
		}
		var cleared *string
		if err := f.owner.QueryRow(context.Background(), `SELECT phone_e164 FROM customers.import_profiles WHERE owner_id=$1`, c.ownerOf("SL-0004")).Scan(&cleared); err != nil || cleared != nil {
			t.Fatalf("mapped empty phone must clear: %v %v", cleared, err)
		}
	})

	t.Run("CI04 a consent column is ignored and flagged, no consent is written", func(t *testing.T) {
		st, pv := c.preview(c.adminTok, file, "")
		if st != 200 || pv["consent_ignored_rows"] != float64(3) {
			t.Fatalf("consent flag: %d %v", st, pv["consent_ignored_rows"])
		}
		flagged := 0
		for _, r := range pv["rows"].([]any) {
			if r.(map[string]any)["consent_ignored"] == true {
				flagged++
			}
		}
		if flagged != 3 {
			t.Fatalf("flagged rows %d", flagged)
		}
		for _, id := range []string{"SL-0001", "SL-0002", "SL-0003", "SL-0004"} {
			o := c.ownerOf(id)
			if c.n(`SELECT count(*) FROM customers.consent_events WHERE owner_id=$1`, o) != 0 || c.allows(o, "marketing_messages", "meta_dm") || c.allows(o, "ads_personalization", "meta_ads") {
				t.Fatalf("%s: imported consent is not unknown", id)
			}
		}
		if c.n(`SELECT count(*) FROM customers.consent_events WHERE tenant_id=$1 AND source<>'erasure'`, f.tenantA) != 0 {
			t.Fatal("the import wrote consent events")
		}
	})

	t.Run("CI05 bad cells fail their row; formula cells are unguarded on input and guarded in results.csv", func(t *testing.T) {
		bad := "customer_id,name,phone,email\n" +
			"BAD-1,Valid Name,0212345678,\n" + // landline: invalid_phone
			"BAD-2," + strings.Repeat("名", 81) + ",,\n" + // name_too_long
			"'=EVIL-3,'=SUM(1+1),0900000030,\n" + // guarded by an earlier export: stored without the apostrophe
			"BAD-4,Valid Name,,not-an-email\n"
		st, pv := c.preview(c.adminTok, bad, "")
		if st != 200 || pv["failed_rows"] != float64(3) || pv["new_rows"] != float64(1) {
			t.Fatalf("preview: %d %v", st, pv)
		}
		codes := []string{}
		for _, r := range pv["rows"].([]any) {
			codes = append(codes, fmt.Sprint(r.(map[string]any)["code"]))
		}
		if strings.Join(codes, ",") != "invalid_phone,name_too_long,<nil>,invalid_email" {
			t.Fatalf("row codes: %v", codes)
		}
		st, cm := c.commit(c.adminTok, bad, 1, "")
		if st != 200 || cm["created"] != float64(1) || cm["failed"] != float64(3) {
			t.Fatalf("commit: %d %v", st, cm)
		}
		var name string
		if err := f.owner.QueryRow(context.Background(), `SELECT p.display_name FROM customers.import_profiles p JOIN migrationimport.external_ids e ON e.internal_id=p.owner_id
		  WHERE e.tenant_id=$1 AND e.external_id='=EVIL-3'`, f.tenantA).Scan(&name); err != nil || name != "=SUM(1+1)" {
			t.Fatalf("unguarded stored name: %q %v", name, err)
		}
		st, raw, hdr := c.call("GET", c.imports(c.store)+"/"+cm["batch_id"].(string)+"/results.csv", c.adminTok, "")
		if st != 200 || !strings.HasPrefix(string(raw), "\xEF\xBB\xBF") || !strings.HasPrefix(hdr.Get("Content-Type"), "text/csv") || hdr.Get("Cache-Control") != "no-store, private" {
			t.Fatalf("results.csv: %d %q %v", st, raw, hdr)
		}
		body := string(raw)
		if !strings.Contains(body, "row,external_id,outcome,code") || !strings.Contains(body, "'=EVIL-3,created") || !strings.Contains(body, "1,,failed,invalid_phone") {
			t.Fatalf("results.csv content: %q", body)
		}
		st, raw, _ = c.call("GET", c.imports(c.store)+"/"+cm["batch_id"].(string)+"/results.csv?only=failed", c.adminTok, "")
		if st != 200 || strings.Contains(string(raw), "created") || strings.Count(string(raw), "failed") != 3 {
			t.Fatalf("only=failed: %d %q", st, raw)
		}
		// Another store's batch is invisible: tenant B asking for tenant A's batch id is not a 200.
		if st, _, _ := c.call("GET", c.imports(f.storeB)+"/"+firstBatch+"/results.csv", c.otok, ""); st != 404 {
			t.Fatalf("cross-store batch: %d", st)
		}
	})

	t.Run("CI06 customers:privacy is required; tenants never match", func(t *testing.T) {
		for name, token := range map[string]string{"customers:write only": c.authorATok, "read only": c.rtoken} {
			if st, _ := c.preview(token, file, ""); st != 403 {
				t.Fatalf("%s preview: %d", name, st)
			}
			if st, _ := c.commit(token, file, 3, ""); st != 403 {
				t.Fatalf("%s commit: %d", name, st)
			}
			if st, _, _ := c.call("GET", c.imports(c.store)+"/"+firstBatch+"/results.csv", token, ""); st != 403 {
				t.Fatalf("%s results: %d", name, st)
			}
		}
		if st, _, _ := c.call("POST", c.imports(c.store)+"/customers/preview", "", file); st != 401 {
			t.Fatalf("no token: %d", st)
		}
		// Tenant B holds customers:privacy on its own store only: store A1 is not theirs.
		if st, _, _ := c.call("POST", c.imports(c.store)+"/customers/preview", c.otok, file); st != 403 && st != 404 {
			t.Fatalf("other tenant on A1: %d", st)
		}
		// Importing the same external ids into tenant B's store creates its own owners: no cross-tenant matching.
		st, raw, _ := c.call("POST", fmt.Sprintf("%s/customers/commit?expected_apply_rows=3", c.imports(f.storeB)), c.otok, file)
		if st != 200 || sraJSON(raw)["created"] != float64(3) {
			t.Fatalf("tenant B commit: %d %s", st, raw)
		}
		if c.n(`SELECT count(DISTINCT internal_id) FROM migrationimport.external_ids WHERE external_id='SL-0001'`) != 2 {
			t.Fatal("tenant A and tenant B must hold different owners for the same external id")
		}
		if c.n(`SELECT count(*) FROM customers.import_profiles WHERE tenant_id=$1 AND store_id=$2`, f.tenantB, f.storeB) != 3 {
			t.Fatal("tenant B profiles")
		}
	})

	t.Run("CI07 erasure deletes the profile and the external id and scrubs retained results", func(t *testing.T) {
		victim := c.ownerOf("SL-0002")
		if !strings.Contains(c.resultsCSV(firstBatch), "SL-0002") {
			t.Fatal("precondition: the retained batch names SL-0002")
		}
		st, raw, _ := c.call("POST", "/v1/admin/stores/"+c.store+"/customers/"+victim+"/erasure", c.adminTok, `{"confirm":"ERASE"}`,
			"Content-Type", "application/json", "Idempotency-Key", t04Key("ci-erase"))
		if st != 200 {
			t.Fatalf("erasure: %d %s", st, raw)
		}
		if c.n(`SELECT count(*) FROM customers.import_profiles WHERE owner_id=$1`, victim)+c.n(`SELECT count(*) FROM migrationimport.external_ids WHERE internal_id=$1`, victim) != 0 {
			t.Fatal("profile or external id survived the erasure")
		}
		if _, ok := c.list("")[victim]; ok {
			t.Fatal("an erased imported customer is still listed")
		}
		body := c.resultsCSV(firstBatch)
		if strings.Contains(body, "SL-0002") || !strings.Contains(body, "SL-0001") || strings.Count(body, "created") != 3 {
			t.Fatalf("scrubbed results (the row stays, its external id goes): %q", body)
		}
		// restore replay (CD8): profile and external id that reappear are deleted again.
		mustExec(t, f.owner, `INSERT INTO customers.import_profiles(tenant_id,store_id,owner_id,display_name,source) VALUES($1,$2,$3,'restored','shopline_csv')`, f.tenantA, c.store, victim)
		mustExec(t, f.owner, `INSERT INTO migrationimport.external_ids(tenant_id,store_id,kind,external_id,internal_id) VALUES($1,$2,'customers','SL-0002',$3)`, f.tenantA, c.store, victim)
		mustExec(t, f.owner, `SELECT customers.replay_erasures(ARRAY[$1::uuid])`, victim)
		if c.n(`SELECT count(*) FROM customers.import_profiles WHERE owner_id=$1`, victim)+c.n(`SELECT count(*) FROM migrationimport.external_ids WHERE internal_id=$1`, victim) != 0 {
			t.Fatal("replay_erasures left imported data")
		}
	})

	t.Run("CI07b erasure leaves a salted tombstone: a stale export cannot resurrect the person", func(t *testing.T) {
		stale := strings.Replace(file, "Test Customer Three", "Test Customer Three Renamed", 1)
		st, pv := c.preview(c.adminTok, stale, "")
		if st != 200 || pv["erased_rows"] != float64(1) || pv["update_rows"] != float64(2) || pv["new_rows"] != float64(0) || pv["apply_rows"] != float64(2) || pv["failed_rows"] != float64(1) {
			t.Fatalf("preview after erasure: %d %v", st, pv)
		}
		for _, r := range pv["rows"].([]any) {
			m := r.(map[string]any)
			if m["code"] == "erased" && (m["row"] != float64(2) || m["outcome"] != "failed" || m["external_id"] != nil && m["external_id"] != "") {
				t.Fatalf("erased row must carry the row number and code only: %v", m)
			}
		}
		if strings.Contains(fmt.Sprint(pv), "SL-0002") {
			t.Fatalf("the erased id is echoed in the preview: %v", pv)
		}
		ownersBefore := c.n(`SELECT count(*) FROM buyer.owners WHERE tenant_id=$1`, f.tenantA)
		st, cm := c.commit(c.adminTok, stale, 2, "")
		if st != 200 || cm["updated"] != float64(2) || cm["created"] != float64(0) || cm["failed"] != float64(1) {
			t.Fatalf("commit after erasure: %d %v", st, cm)
		}
		if c.n(`SELECT count(*) FROM buyer.owners WHERE tenant_id=$1`, f.tenantA) != ownersBefore || c.n(`SELECT count(*) FROM migrationimport.external_ids WHERE tenant_id=$1 AND external_id='SL-0002'`, f.tenantA) != 0 {
			t.Fatal("an erased id created an owner or an external id again")
		}
		if body := c.resultsCSV(cm["batch_id"].(string)); strings.Contains(body, "SL-0002") || !strings.Contains(body, "erased") {
			t.Fatalf("results.csv of the batch: %q", body)
		}
		if c.n(`SELECT count(*) FROM migrationimport.batches b WHERE b.tenant_id=$1 AND b.results::text LIKE '%SL-0002%'`, f.tenantA) != 0 ||
			c.n(`SELECT count(*) FROM ops.command_results r WHERE r.tenant_id=$1 AND r.response::text LIKE '%SL-0002%'`, f.tenantA) != 0 {
			t.Fatal("the erased id survives in a batch or receipt")
		}
		if c.n(`SELECT count(*) FROM migrationimport.erased_external_ids WHERE tenant_id=$1 AND store_id=$2 AND kind='customers'`, f.tenantA, c.store) != 1 ||
			c.n(`SELECT count(*) FROM migrationimport.erased_external_ids WHERE id_digest=sha256('SL-0002'::bytea)`) != 0 ||
			c.n(`SELECT count(*) FROM migrationimport.store_salts WHERE tenant_id=$1 AND store_id=$2 AND octet_length(salt)=32`, f.tenantA, c.store) != 1 {
			t.Fatal("tombstone must be one salted digest per erased id, never an unsalted hash")
		}
		// An erased id on a row that fails Go validation (invalid phone) never reaches the tombstone check: it must not be echoed anywhere.
		failRow := "customer_id,name,phone\nSL-0002,Erased But Invalid,0212345678\nNEW-77,Valid New Row,0900000077\n"
		st, pv = c.preview(c.adminTok, failRow, "")
		if st != 200 || strings.Contains(fmt.Sprint(pv), "SL-0002") || pv["failed_rows"] != float64(1) {
			t.Fatalf("failed row of an erased id echoes it: %d %v", st, pv)
		}
		st, cm = c.commit(c.adminTok, failRow, 1, "")
		if st != 200 || cm["created"] != float64(1) || cm["failed"] != float64(1) {
			t.Fatalf("commit with a failed erased-id row: %d %v", st, cm)
		}
		if body := c.resultsCSV(cm["batch_id"].(string)); strings.Contains(body, "SL-0002") || !strings.Contains(body, "1,,failed,invalid_phone") {
			t.Fatalf("results.csv of the failed erased-id row: %q", body)
		}
		for label, q := range map[string]string{
			"batches":  `SELECT count(*) FROM migrationimport.batches b WHERE b.tenant_id=$1 AND (b.results::text LIKE '%SL-0002%' OR b.mapping::text LIKE '%SL-0002%')`,
			"receipts": `SELECT count(*) FROM ops.command_results r WHERE r.tenant_id=$1 AND r.response::text LIKE '%SL-0002%'`,
			"external": `SELECT count(*) FROM migrationimport.external_ids e WHERE e.tenant_id=$1 AND e.external_id='SL-0002'`,
		} {
			if got := c.n(q, f.tenantA); got != 0 {
				t.Fatalf("%s still hold the erased id after a failed row: %d", label, got)
			}
		}
		// The same id in another tenant's store is unaffected (the digest is per store).
		other := "customer_id,name\nSL-0002,Other Tenant Row\n"
		if st, raw, _ := c.call("POST", c.imports(f.storeB)+"/customers/commit?expected_apply_rows=1", c.otok, other); st != 200 || sraJSON(raw)["failed"] != float64(0) {
			t.Fatalf("tenant B same id: %d %s", st, raw)
		}
	})

	t.Run("CI08 limits and drift", func(t *testing.T) {
		var b strings.Builder
		b.WriteString("customer_id,name\n")
		for i := 0; i < 5001; i++ {
			fmt.Fprintf(&b, "BIG-%d,N\n", i)
		}
		if st, body := c.preview(c.adminTok, b.String(), ""); st != 422 || body["code"] != "too_many_rows" {
			t.Fatalf("5001 rows: %d %v", st, body)
		}
		if st, body := c.preview(c.adminTok, "customer_id,name\nA,"+strings.Repeat("x", 2<<20)+"\n", ""); st != 413 || body["code"] != "invalid_request" {
			t.Fatalf("2 MiB + 1: %d %v", st, body)
		}
		if st, body := c.preview(c.adminTok, "customer_id,name\nA1,\xa4\xa4\xa4\xe5\n", ""); st != 422 || body["code"] != "encoding_not_utf8" {
			t.Fatalf("big5: %d %v", st, body)
		}
		if st, body := c.preview(c.adminTok, "a,b\n1,2\n", ""); st != 422 || body["code"] != "required" {
			t.Fatalf("no mappable columns: %d %v", st, body)
		}
		if st, body := c.commit(c.adminTok, "customer_id,name,phone\nNA-1,Valid,0212345678\n", 0, ""); st != 422 || body["code"] != "nothing_to_apply" {
			t.Fatalf("nothing to apply: %d %v", st, body)
		}
		// Drift: the preview promised 1, the commit is sent 2 -> 409 preview_stale with the fresh preview, nothing written.
		drift := "customer_id,name\nDR-1,Drift One\n"
		before := c.n(`SELECT count(*) FROM migrationimport.external_ids WHERE tenant_id=$1`, f.tenantA)
		st, body := c.commit(c.adminTok, drift, 2, "")
		if st != 409 || body["new_rows"] != float64(1) || body["apply_rows"] != float64(1) {
			t.Fatalf("stale: %d %v", st, body)
		}
		if c.n(`SELECT count(*) FROM migrationimport.external_ids WHERE tenant_id=$1`, f.tenantA) != before {
			t.Fatal("a stale commit wrote rows")
		}
		// An explicit mapping works and is stored on the batch (headers only).
		mapped := "ref,who,tel\nMP-1,Mapped Name,0900000040\n"
		st, cm := c.commit(c.adminTok, mapped, 1, "&mapping="+`{"external_id":"ref","name":"who","phone":"tel"}`)
		if st != 200 || cm["created"] != float64(1) {
			t.Fatalf("mapped commit: %d %v", st, cm)
		}
		var mapping string
		if err := f.owner.QueryRow(context.Background(), `SELECT mapping::text FROM migrationimport.batches WHERE id=$1`, cm["batch_id"]).Scan(&mapping); err != nil || !strings.Contains(mapping, `"external_id": "ref"`) {
			t.Fatalf("stored mapping: %q %v", mapping, err)
		}
		// 5000 rows fit the 60 s budget (preview + commit, one transaction each).
		b.Reset()
		b.WriteString("customer_id,name,phone\n")
		for i := 0; i < 5000; i++ {
			fmt.Fprintf(&b, "MAX-%d,Max Customer %d,09%08d\n", i, i, i)
		}
		start := time.Now()
		if st, body := c.preview(c.adminTok, b.String(), ""); st != 200 || body["new_rows"] != float64(5000) {
			t.Fatalf("5000 preview: %d %v", st, body["new_rows"])
		}
		if st, cm := c.commit(c.adminTok, b.String(), 5000, ""); st != 200 || cm["created"] != float64(5000) {
			t.Fatalf("5000 commit: %d %v", st, cm)
		}
		t.Logf("5000-row preview + commit: %s", time.Since(start))
		if time.Since(start) > 50*time.Second {
			t.Fatalf("5000 rows took %s, over the budget margin", time.Since(start))
		}
	})

	t.Run("CI09 no name, phone or email in results, audit, receipts, batches or logs", func(t *testing.T) {
		for _, secret := range []string{"測試顧客", "Test Customer", "0900000", "900000001", "example.test", "Max Customer", "Valid Name", "Drift One", "Mapped Name"} {
			for label, q := range map[string]string{
				"audit":    `SELECT count(*) FROM ops.audit_events a WHERE a.tenant_id=$1 AND row_to_json(a)::text LIKE '%'||$2||'%'`,
				"receipts": `SELECT count(*) FROM ops.command_results r WHERE r.tenant_id=$1 AND (r.response::text LIKE '%'||$2||'%' OR r.operation LIKE '%'||$2||'%')`,
				"batches":  `SELECT count(*) FROM migrationimport.batches b WHERE b.tenant_id=$1 AND (b.results::text LIKE '%'||$2||'%' OR b.mapping::text LIKE '%'||$2||'%')`,
				"external": `SELECT count(*) FROM migrationimport.external_ids e WHERE e.tenant_id=$1 AND e.external_id LIKE '%'||$2||'%'`,
			} {
				if got := c.n(q, f.tenantA, secret); got != 0 {
					t.Errorf("%s holds %q (%d rows)", label, secret, got)
				}
			}
			if strings.Contains(c.logs.String(), secret) {
				t.Errorf("logs hold %q", secret)
			}
		}
		for _, id := range []string{firstBatch} {
			body := c.resultsCSV(id)
			for _, secret := range []string{secretName, secretPhone, secretEmail, "900000001", "example.test"} {
				if strings.Contains(body, secret) {
					t.Errorf("results.csv holds %q", secret)
				}
			}
		}
	})

	t.Run("CI10 an imported customer can be tagged and noted", func(t *testing.T) {
		tag := c.mustTag(c.adminTok, c.store, "匯入", "teal")
		owner := c.ownerOf("SL-0003")
		c.tagOwner(owner, tag.ID)
		if _, err := c.addNote(c.authorATok, c.store, owner, "synthetic imported note"); err != nil {
			t.Fatalf("note on an imported customer: %v", err)
		}
		d := c.detail(c.adminTok, c.store, owner)
		if len(d.Tags) != 1 || len(d.Notes) != 1 || !d.Imported {
			t.Fatalf("detail: tags=%v notes=%d imported=%v", d.Tags, len(d.Notes), d.Imported)
		}
		p, err := c.listByTag(c.adminTok, c.store, tag.ID, 10, "")
		if err != nil || len(p.Items) != 1 || p.Items[0].CustomerID != owner {
			t.Fatalf("tag filter: %v %v", p.Items, err)
		}
	})

	t.Run("CI11 no login role reaches the new tables or inserts owners", func(t *testing.T) {
		ctx := context.Background()
		for _, q := range []string{`SELECT 1 FROM migrationimport.batches`, `SELECT 1 FROM migrationimport.external_ids`, `SELECT 1 FROM customers.import_profiles`,
			`INSERT INTO buyer.owners(tenant_id,store_id) VALUES ('` + f.tenantA + `','` + c.store + `')`} {
			if _, err := f.runtime.Exec(ctx, q); err == nil || !strings.Contains(err.Error(), "42501") {
				t.Fatalf("runtime login must be denied %q: %v", q, err)
			}
		}
		if got := c.n(`SELECT count(*) FROM information_schema.table_privileges WHERE table_schema='migrationimport' AND grantee NOT IN ('commerce_privacy_writer') AND grantee<>(SELECT pg_get_userbyid(relowner) FROM pg_class WHERE oid='migrationimport.batches'::regclass)`); got != 0 {
			t.Fatalf("%d unexpected table grants in schema migrationimport", got)
		}
	})

	t.Run("CI12 the merchant privacy export contains the imported profile", func(t *testing.T) {
		owner := c.ownerOf("SL-0001")
		key := t04Key("ci-export")
		st, raw, _ := c.call("POST", "/v1/admin/stores/"+c.store+"/customers/"+owner+"/exports", c.adminTok, "", "Content-Type", "application/json", "Idempotency-Key", key)
		if st != 200 {
			t.Fatalf("export: %d %s", st, raw)
		}
		doc := sraJSON(raw)
		prof, _ := doc["import_profile"].(map[string]any)
		if prof == nil || prof["email"] != secretEmail || prof["phone"] != "+886900000001" || prof["display_name"] != secretName || prof["source"] != "shopline_csv" {
			t.Fatalf("export import_profile: %v", doc["import_profile"])
		}
		if ids, _ := prof["external_ids"].([]any); len(ids) != 1 || ids[0] != "SL-0001" {
			t.Fatalf("export external ids: %v", prof["external_ids"])
		}
		for label, q := range map[string]string{
			"privacy_actions": `SELECT count(*) FROM customers.privacy_actions a WHERE a.tenant_id=$1 AND row_to_json(a)::text LIKE '%'||$2||'%'`,
			"receipts":        `SELECT count(*) FROM ops.command_results r WHERE r.tenant_id=$1 AND r.response::text LIKE '%'||$2||'%'`,
			"audit":           `SELECT count(*) FROM ops.audit_events a WHERE a.tenant_id=$1 AND row_to_json(a)::text LIKE '%'||$2||'%'`,
		} {
			// The values actually exported (read from the export itself), not a remembered literal.
			secrets := []string{fmt.Sprint(prof["email"]), strings.TrimPrefix(fmt.Sprint(prof["phone"]), "+886"), fmt.Sprint(prof["display_name"]), fmt.Sprint(prof["external_ids"].([]any)[0])}
			for _, secret := range secrets {
				if len(secret) < 6 {
					t.Fatalf("scan token too short to be meaningful: %q", secret)
				}
				if got := c.n(q, f.tenantA, secret); got != 0 {
					t.Errorf("%s holds %q after the export", label, secret)
				}
			}
		}
		// A non-imported customer's export has no import_profile section.
		plain := c.bundleCustomer().Scope.OwnerID
		st, raw, _ = c.call("POST", "/v1/admin/stores/"+c.store+"/customers/"+plain+"/exports", c.adminTok, "", "Content-Type", "application/json", "Idempotency-Key", t04Key("ci-export2"))
		if st != 200 || sraJSON(raw)["import_profile"] != nil {
			t.Fatalf("plain export: %d %s", st, raw)
		}
	})

	t.Run("CI13 the owner-insert privilege cannot create an owner outside the GUC scope", func(t *testing.T) {
		ctx := context.Background()
		try := func(gucTenant, gucStore, tenant, store string) error {
			tx, err := f.owner.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			for _, q := range []string{`SET LOCAL ROLE commerce_privacy_writer`, `SELECT set_config('app.tenant_id','` + gucTenant + `',true),set_config('app.store_id','` + gucStore + `',true)`} {
				if _, err := tx.Exec(ctx, q); err != nil {
					t.Fatalf("%s: %v", q, err)
				}
			}
			_, err = tx.Exec(ctx, `INSERT INTO buyer.owners(tenant_id,store_id) VALUES($1,$2)`, tenant, store)
			return err
		}
		if err := try(f.tenantA, c.store, f.tenantA, c.store); err != nil {
			t.Fatalf("control: an owner in the GUC scope must insert: %v", err)
		}
		for name, args := range map[string][4]string{
			"other store, same tenant": {f.tenantA, c.store, f.tenantA, f.storeA2},
			"other tenant and store":   {f.tenantA, c.store, f.tenantB, f.storeB},
			"scope of another tenant":  {f.tenantB, f.storeB, f.tenantA, c.store},
		} {
			err := try(args[0], args[1], args[2], args[3])
			if err == nil || (!strings.Contains(err.Error(), "42501") && !strings.Contains(err.Error(), "row-level security")) {
				t.Fatalf("%s: owner insert must be refused, got %v", name, err)
			}
		}
		// A column outside (tenant_id, store_id) is not insertable either.
		tx, _ := f.owner.Begin(ctx)
		defer tx.Rollback(ctx)
		_, _ = tx.Exec(ctx, `SET LOCAL ROLE commerce_privacy_writer`)
		if _, err := tx.Exec(ctx, `INSERT INTO buyer.owners(id,tenant_id,store_id) VALUES(gen_random_uuid(),$1,$2)`, f.tenantA, c.store); err == nil {
			t.Fatal("inserting the owner id must be refused (column grant is tenant_id, store_id only)")
		}
	})

	t.Run("CI14 two parallel commits of one file import it once (deterministic barrier)", func(t *testing.T) {
		var b strings.Builder
		b.WriteString("customer_id,name\n")
		for i := 0; i < 40; i++ {
			fmt.Fprintf(&b, "PAR-%d,Parallel %d\n", i, i)
		}
		body := b.String()
		sum := sha256.Sum256([]byte(body))
		// The barrier: hold command.Run's own advisory lock key in a separate transaction, start both commits, wait until both are really
		// in flight (or 5 s: without the lock they do not wait at all), then release so they race for the lock together.
		lockKey := "command|" + f.tenantA + "|" + c.store + "|migrationimport.customers_commit|cimp-" + hex.EncodeToString(sum[:])[:32]
		ctx := context.Background()
		barrier, err := f.owner.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer barrier.Rollback(ctx)
		if _, err := barrier.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, lockKey); err != nil {
			t.Fatal(err)
		}
		ownersBefore := c.n(`SELECT count(*) FROM buyer.owners WHERE tenant_id=$1`, f.tenantA)
		type res struct {
			st int
			m  map[string]any
		}
		out := make(chan res, 2)
		for i := 0; i < 2; i++ {
			go func() {
				st, m := c.commit(c.adminTok, body, 40, "")
				out <- res{st, m}
			}()
		}
		waiting := 0
		for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
			waiting = c.n(`SELECT count(*) FROM pg_locks WHERE locktype='advisory' AND NOT granted`)
			if waiting >= 2 {
				break
			}
		}
		if err := barrier.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		a, b2 := <-out, <-out
		if waiting < 2 {
			t.Errorf("only %d of the two commits waited on the idempotency lock: the commits were not serialised by it", waiting)
		}
		if a.st != 200 || b2.st != 200 || a.m["batch_id"] != b2.m["batch_id"] || (a.m["replayed"] == b2.m["replayed"]) {
			t.Fatalf("parallel commits: %d %v / %d %v (want one execution and one replay of the same batch)", a.st, a.m, b2.st, b2.m)
		}
		if got := c.n(`SELECT count(*) FROM buyer.owners WHERE tenant_id=$1`, f.tenantA) - ownersBefore; got != 40 {
			t.Fatalf("owners created: %d, want 40", got)
		}
		if c.n(`SELECT count(*) FROM migrationimport.batches WHERE tenant_id=$1 AND store_id=$2 AND rows_total=40`, f.tenantA, c.store) != 1 {
			t.Fatal("two batches for one file")
		}
	})

	t.Run("CI15 the commerce_auth read policy is store-scoped; a phone or email as external id is refused", func(t *testing.T) {
		var qual string
		if err := f.owner.QueryRow(context.Background(), `SELECT qual FROM pg_policies WHERE schemaname='customers' AND tablename='import_profiles' AND policyname='auth_import_profile_read'`).Scan(&qual); err != nil ||
			!strings.Contains(qual, "app.tenant_id") || !strings.Contains(qual, "app.store_id") {
			t.Fatalf("auth_import_profile_read must be GUC-scoped: %q %v", qual, err)
		}
		bad := "customer_id,name\nmail@example.test,A Name\n0900-000-077,B Name\n+886900000078,C Name\nOK-77,D Name\n"
		st, pv := c.preview(c.adminTok, bad, "")
		codes := []string{}
		for _, r := range pv["rows"].([]any) {
			codes = append(codes, fmt.Sprint(r.(map[string]any)["code"]))
		}
		if st != 200 || strings.Join(codes, ",") != "invalid_external_id,invalid_external_id,invalid_external_id,<nil>" {
			t.Fatalf("external id shape: %d %v", st, codes)
		}
	})
}

func (c *ciEnv) resultsCSV(batch string) string {
	c.t.Helper()
	st, raw, _ := c.call("GET", c.imports(c.store)+"/"+batch+"/results.csv", c.adminTok, "")
	if st != 200 {
		c.t.Fatalf("results.csv: %d %s", st, raw)
	}
	return string(raw)
}
