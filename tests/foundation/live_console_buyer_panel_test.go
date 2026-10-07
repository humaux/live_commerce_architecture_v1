package foundation_test

// Purpose: REAL_PG gates for unit LC-B3b (docs/delivery/units/lc-b3b-buyer-panel.md; contracts/live-console-v1.md
// §11 A8/A9/A13/A14 + amendment LC-B3b): the A13 buyer-panel read model (claims / claim_total_minor / orders /
// purchase_ordinal / auto_reply, migration 0165), the A8 session_id filter + live_comment bundle-only rows +
// link_version, the A9 link_version/binding_id, A14 CAS with the exposed link_version, I09 (claims.bundles.owner_id
// never links), cross-store/cross-tenant 404, the orders:read key omission, the 50/20 bounds and a sentinel leak scan,
// plus the exact ACL pins of the new/extended definers. Fixtures are synthetic UUIDs and names (no real buyer PII,
// no Meta traffic); checkout.orders rows are seeded through the replica-mode fixture idiom of
// merchant_tools_order_smoke_test.go (the panel read model never re-validates the cart/quote FK chain).
// Depends on: the shared fixture helpers (fixture, lcOwnStores, lcPerson), internal/inbox mounted through the real
// route handler (httpapi.Options{Inbox}), migrations 0008/0013/0060/0105/0119/0128/0129 and 0165 (the unit under test).
// Used by: bash scripts/dev/test-focused.sh 'TestLiveConsoleBuyerPanel' (LC-B3b DELIVERY evidence); no production code.

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/httpapi"
	"livecommerce/internal/inbox"
)

// bpEnv is one LC-B3b scenario: a fresh tenant with two stores, a full-permission merchant, a read-only merchant,
// a synthetic buyer owner (I09: it must never link a panel by itself), the page/IG bindings and the real HTTP
// handler with the inbox service mounted.
type bpEnv struct {
	t                      *testing.T
	f                      *testFixture
	tenant, store1, store2 string
	principal, token       string // store:read + inbox:read + inbox:reply + orders:read + customers:read
	roToken                string // inbox:read only
	handler                http.Handler
	owner1                 string // buyer.owners row
	fbBinding, igBinding   string
	jobSeq, opSeq          int64
}

func bpNew(t *testing.T) *bpEnv {
	t.Helper()
	f := fixture(t)
	tenant, store1, store2 := lcOwnStores(t, f)
	principal, token := lcPerson(t, f, tenant, store1, "inbox:read", "inbox:reply", "orders:read", "customers:read")
	_, roToken := lcPerson(t, f, tenant, store1, "inbox:read")

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	kr, err := inbox.LoadKeyring(func(n string) string {
		switch n {
		case "COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID":
			return miKeyID
		case "COMMERCE_META_PAYLOAD_KEYS_JSON":
			return `{"keys":[{"id":"` + miKeyID + `","key_base64":"` + base64.StdEncoding.EncodeToString(key) + `"}]}`
		}
		return ""
	})
	if err != nil {
		t.Fatal("inbox keyring", err)
	}
	svc, err := inbox.NewService(kr)
	if err != nil {
		t.Fatal(err)
	}
	e := &bpEnv{t: t, f: f, tenant: tenant, store1: store1, store2: store2, principal: principal, token: token, roToken: roToken,
		handler: httpapi.NewHandler(f.runtime, httpapi.Options{Inbox: svc})}
	e.owner1 = randomUUID()
	e.exec(`INSERT INTO buyer.owners(tenant_id,store_id,id) VALUES($1,$2,$3)`, tenant, store1, e.owner1)
	e.fbBinding = e.binding("facebook")
	e.igBinding = e.binding("instagram")
	t.Cleanup(e.cleanup)
	return e
}

func (e *bpEnv) exec(sql string, args ...any) {
	e.t.Helper()
	mustExec(e.t, e.f.owner, sql, args...)
}

func (e *bpEnv) execRow(sql string, args ...any) string {
	e.t.Helper()
	var out string
	if err := e.f.owner.QueryRow(context.Background(), sql, args...).Scan(&out); err != nil {
		e.t.Fatalf("seed row: %v (%s)", err, sql)
	}
	return out
}

// cleanup removes the replica-mode order fixtures (the shared fixture DB is tmpfs-bounded; the smoke-test precedent
// deletes what it cloned). The fresh tenant's small rows (sessions/bundles/conversations) die with the container.
func (e *bpEnv) cleanup() {
	ctx := context.Background()
	tx, err := e.f.owner.Begin(ctx)
	if err != nil {
		e.t.Logf("bp cleanup begin: %v", err)
		return
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SET LOCAL session_replication_role=replica`); err != nil {
		e.t.Logf("bp cleanup replica: %v", err)
		return
	}
	for _, q := range []string{
		`DELETE FROM claims.live_price_uses WHERE tenant_id=$1 AND store_id=$2`,
		`DELETE FROM inbox.order_for_buyer WHERE tenant_id=$1 AND store_id=$2`,
		`DELETE FROM integration.operations WHERE tenant_id=$1 AND store_id=$2`,
		`DELETE FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2 AND job_id>=986500000`,
	} {
		if _, err := tx.Exec(ctx, q, e.tenant, e.store1); err != nil {
			e.t.Logf("bp cleanup: %v", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		e.t.Logf("bp cleanup commit: %v", err)
	}
}

// conversation seeds social.conversations + inbox.conversation_state with an explicit asset id (the A9 binding_id
// match runs on provider+external_asset_id). peer_key is the lcConversation idiom: the conversation id hex, doubled.
// last_inbound_at is set because production conversations only exist once an inbound event arrived (and A9/A13 read
// conversation_meta's window_open_until = last_inbound_at + 24h); a never-messaged row is not a reachable state.
func (e *bpEnv) conversation(store, object, assetID string) string {
	e.t.Helper()
	conv := randomUUID()
	e.exec(`INSERT INTO social.conversations(id,tenant_id,store_id,app_id,object,asset_id,peer_key)
		VALUES($1,$2,$3,'123456789',$4,$5,$6)`, conv, e.tenant, store, object, assetID, e.peerKey(conv))
	e.exec(`INSERT INTO inbox.conversation_state(tenant_id,store_id,conversation_id,last_inbound_at,last_inbound_seq)
		VALUES($1,$2,$3,clock_timestamp(),1)`, e.tenant, store, conv)
	return conv
}

func (e *bpEnv) peerKey(conv string) string {
	return strings.Repeat(strings.ReplaceAll(conv, "-", ""), 2)
}

func (e *bpEnv) session(title string) string {
	e.t.Helper()
	s := randomUUID()
	e.exec(`INSERT INTO live.sessions(id,tenant_id,store_id,principal_id,title) VALUES($1,$2,$3,$4,$5)`,
		s, e.tenant, e.store1, e.principal, title)
	e.exec(`INSERT INTO live.claim_windows(tenant_id,store_id,session_id,state,match_mode,generation,principal_id)
		VALUES($1,$2,$3,'CLOSED','EXACT',0,$4)`, e.tenant, e.store1, s, e.principal)
	return s
}

func (e *bpEnv) sku(code string, price int64) string {
	e.t.Helper()
	product := randomUUID()
	e.exec(`INSERT INTO catalog.products(tenant_id,store_id,id,name) VALUES($1,$2,$3,'bp-product-'||$4)`,
		e.tenant, e.store1, product, code)
	sku := randomUUID()
	e.exec(`INSERT INTO catalog.skus(tenant_id,store_id,id,product_id,code,currency,price_minor) VALUES($1,$2,$3,$4,$5,'USD',$6)`,
		e.tenant, e.store1, sku, product, code, price)
	return sku
}

// offer seeds one live.offers row with its OWN sku: live_offer_active_sku admits at most one ACTIVE offer per
// (session, sku), so a multi-offer session needs one sku per offer. The leak scan uses offerOnSku with a sentinel sku.
func (e *bpEnv) offer(session, keyword string, livePrice int64) string {
	e.t.Helper()
	return e.offerOnSku(session, keyword, livePrice, e.sku("SKU-"+strings.ReplaceAll(randomUUID(), "-", "")[:8], 5000))
}

func (e *bpEnv) offerOnSku(session, keyword string, livePrice int64, skuID string) string {
	e.t.Helper()
	return e.execRow(`INSERT INTO live.offers(tenant_id,store_id,session_id,keyword,sku_id,max_quantity_per_claim,principal_id,live_price_minor)
		VALUES($1,$2,$3,$4,$5,5,$6,$7) RETURNING id::text`, e.tenant, e.store1, session, keyword, skuID, e.principal, livePrice)
}

// bundle seeds one claims.bundles row. platform 'facebook'/'instagram' rows must carry no label, 'manual' rows one
// (0060 CHECK). ownerID/bound_at are the buyer binding that I09 forbids the panel to use as an identity link.
func (e *bpEnv) bundle(session, platform, actorKey string, ownerID *string, label *string, pending bool) string {
	e.t.Helper()
	var bound any
	var owner any
	if ownerID != nil {
		owner = *ownerID
		bound = time.Now()
	}
	var lbl any
	if label != nil {
		lbl = *label
	}
	return e.execRow(`INSERT INTO claims.bundles(tenant_id,store_id,session_id,platform,actor_key,owner_id,bound_at,label,link_pending_manual)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id::text`,
		e.tenant, e.store1, session, platform, actorKey, owner, bound, lbl, pending)
}

// line seeds one claims.lines row; the sku is resolved from the offer (the lines FK names (session,offer,sku)).
func (e *bpEnv) line(bundle, session, offer string, qty int) {
	e.t.Helper()
	e.exec(`INSERT INTO claims.lines(tenant_id,store_id,session_id,bundle_id,offer_id,sku_id,quantity,version)
		VALUES($1,$2,$3,$4,$5,(SELECT sku_id FROM live.offers WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3 AND id=$5),$6,1)`,
		e.tenant, e.store1, session, bundle, offer, qty)
}

// linkPeer writes the ONLY identity link the panel may use (§3.7): the bundle_peers row a successful private reply left.
func (e *bpEnv) linkPeer(bundle, conv, object, assetID string) {
	e.t.Helper()
	e.exec(`INSERT INTO inbox.bundle_peers(tenant_id,store_id,bundle_id,peer_key,app_id,object,asset_id,operation_id)
		VALUES($1,$2,$3,$4,'123456789',$5,$6,$7)`, e.tenant, e.store1, bundle, e.peerKey(conv), object, assetID, randomUUID())
}

func (e *bpEnv) binding(provider string) string {
	e.t.Helper()
	id := randomUUID()
	e.exec(`INSERT INTO integration.bindings(tenant_id,store_id,id,principal_id,provider,external_asset_id)
		VALUES($1,$2,$3,$4,$5,'1234567890')`, e.tenant, e.store1, id, e.principal, provider)
	return id
}

// order seeds a checkout.orders row in replica mode (FK chain skipped; every CHECK still applies). The fixture idiom
// is merchant_tools_order_smoke_test.go's; job_id >= 986500000 marks the rows for cleanup.
func (e *bpEnv) order(commercial, fulfillment string, total int64, age time.Duration) string {
	e.t.Helper()
	e.jobSeq++
	ctx := context.Background()
	tx, err := e.f.owner.Begin(ctx)
	if err != nil {
		e.t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SET LOCAL session_replication_role=replica`); err != nil {
		e.t.Skipf("NOT_RUN: the owner role cannot disable FK triggers (%v)", err)
	}
	id := randomUUID()
	ageSec := fmt.Sprintf("%f seconds", age.Seconds())
	var created string
	err = tx.QueryRow(ctx, `INSERT INTO checkout.orders(tenant_id,store_id,owner_id,id,creator_session_id,cart_id,cart_version,
		quote_id,destination_id,market_id,country,service_code,service_version,allocation_version,currency,total_minor,
		commercial_state,fulfillment_state,generation,expires_at,job_id,snapshot,created_at,updated_at,payment_mode)
		VALUES($1,$2,$3,$4,$5,$6,1,$7,$8,$9,'TW','bp-svc',1,1,'USD',$10,$11,$12,1,
		 (clock_timestamp()-$13::interval)+interval '10 minutes',$14,'{}',clock_timestamp()-$13::interval,clock_timestamp()-$13::interval,'bank_transfer')
		RETURNING to_char(created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"')`,
		e.tenant, e.store1, e.owner1, id, randomUUID(), randomUUID(), randomUUID(), randomUUID(), randomUUID(),
		total, commercial, fulfillment, ageSec, 986500000+e.jobSeq).Scan(&created)
	if err != nil {
		e.t.Fatalf("seed order %s: %v", commercial, err)
	}
	if err := tx.Commit(ctx); err != nil {
		e.t.Fatal(err)
	}
	return id
}

func (e *bpEnv) use(bundle, offer, order string, qty int, unit int64) {
	e.t.Helper()
	e.exec(`INSERT INTO claims.live_price_uses(tenant_id,store_id,bundle_id,offer_id,order_id,quantity,unit_price_minor)
		VALUES($1,$2,$3,$4,$5,$6,$7)`, e.tenant, e.store1, bundle, offer, order, qty, unit)
}

// origin seeds a claims.order_origins row: the price-neutral claim-checkout ledger (no live_price_uses row).
func (e *bpEnv) origin(bundle, offer, order, session string) {
	e.t.Helper()
	e.exec(`INSERT INTO claims.order_origins(tenant_id,store_id,order_id,bundle_id,offer_id,line_version,session_id,occurred_at)
		VALUES($1,$2,$3,$4,$5,1,$6,clock_timestamp())`, e.tenant, e.store1, order, bundle, offer, session)
}

// forBuyer seeds an A16 order_for_buyer row. state is 'placed' for a live order and 'released' for one whose order
// was cancelled (the order_for_buyer_live unique index admits one pending/placed row per bundle, as production does).
func (e *bpEnv) forBuyer(bundle, order, state string) {
	e.t.Helper()
	e.exec(`INSERT INTO inbox.order_for_buyer(tenant_id,store_id,bundle_id,request_id,idempotency_key_hash,request_hash,
		buyer_id,order_id,state,principal_id)
		VALUES($1,$2,$3,$4,sha256($5::text::bytea),sha256($6::text::bytea),$7,$8,$9,$10)`,
		e.tenant, e.store1, bundle, randomUUID(), randomUUID(), randomUUID(), e.owner1, order, state, e.principal)
}

// autoReply seeds one meta.private_reply operation. kind 'auto' carries 0128's origin_kind marker; 'manual' carries
// only origin=human (plan_manual_private_reply), so the panel must ignore it however new it is.
func (e *bpEnv) autoReply(bundle, state, kind string, age time.Duration) string {
	e.t.Helper()
	e.opSeq++
	var req string
	if kind == "auto" {
		req = fmt.Sprintf(`{"v":1,"origin_kind":"auto","origin":"auto","bundle_id":%q}`, bundle)
	} else {
		req = fmt.Sprintf(`{"v":1,"origin":"human","bundle_id":%q}`, bundle)
	}
	op := randomUUID()
	ageSec := fmt.Sprintf("%f seconds", age.Seconds())
	// generation=1: the 0008 CHECK only allows generation 0 for READY rows.
	e.exec(`INSERT INTO integration.operations(tenant_id,store_id,id,principal_id,binding_id,binding_version,provider,
		external_asset_id,purpose,action,semantic_key,request_hash,request,job_id,state,generation,created_at)
		VALUES($1,$2,$3,$4,$5,1,'facebook','1234567890','service','meta.private_reply',$6,sha256($7::text::bytea),$7::jsonb,$8,$9,1,clock_timestamp()-$10::interval)`,
		e.tenant, e.store1, op, e.principal, e.fbBinding,
		fmt.Sprintf("bp-auto-%d-%s", e.opSeq, strings.ReplaceAll(randomUUID(), "-", "")),
		req, 986600000+e.opSeq, state, ageSec)
	return op
}

// get/post drive the REAL route handler (no direct service calls: the orders:read key omission lives in the handler).
func (e *bpEnv) get(token, path string) (int, map[string]any, string, http.Header) {
	e.t.Helper()
	r := httptest.NewRequest("GET", "/v1/admin/stores/"+e.store1+path, nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	e.handler.ServeHTTP(w, r)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out, w.Body.String(), w.Header()
}

func (e *bpEnv) get3(token, path string) (int, map[string]any, string) {
	e.t.Helper()
	c, o, r, _ := e.get(token, path)
	return c, o, r
}

func (e *bpEnv) post(token, path, key string, body map[string]any) (int, map[string]any, string) {
	e.t.Helper()
	raw, _ := json.Marshal(body)
	r := httptest.NewRequest("POST", "/v1/admin/stores/"+e.store1+path, bytes.NewReader(raw))
	r.Header.Set("Content-Type", "application/json")
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	e.handler.ServeHTTP(w, r)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out, w.Body.String()
}

func bpStr(m map[string]any, k string) string  { s, _ := m[k].(string); return s }
func bpNum(m map[string]any, k string) float64 { n, _ := m[k].(float64); return n }
func bpArr(m map[string]any, k string) []any   { a, _ := m[k].([]any); return a }
func bpMap(v any) map[string]any               { m, _ := v.(map[string]any); return m }

func bpActorKey(seed string) string {
	sum := make([]byte, 32)
	copy(sum, []byte(seed))
	return fmt.Sprintf("%x", sum)
}

// ---------------------------------------------------------------------------
// Gate 1: peer-linked claims and orders appear (A13 by conversation and by bundle).
// claim_total_minor = Σ quantity × the unit price the claim RECORDED (claims.live_price_uses.unit_price_minor, the
// newest recorded use; never recomputed from the current catalogue — the offers below carry a decoy live price).
// ---------------------------------------------------------------------------
func TestLiveConsoleBuyerPanelClaimsAndOrders(t *testing.T) {
	e := bpNew(t)
	s1 := e.session("bp-session-1")
	a1 := e.offer(s1, "A1", 9999) // decoy current-catalogue price: must NOT feed claim_total_minor
	a2 := e.offer(s1, "A2", 9999)
	b1 := e.bundle(s1, "facebook", bpActorKey("bp-actor-b1"), nil, nil, false)
	e.line(b1, s1, a1, 2)
	e.line(b1, s1, a2, 3)
	conv := e.conversation(e.store1, "page", "1234567890")
	e.linkPeer(b1, conv, "page", "1234567890")

	oOld := e.order("CONFIRMED", "MANUAL_UNASSIGNED", 900, 5*time.Hour)
	e.use(b1, a1, oOld, 1, 300)
	oNew := e.order("AWAITING_COLLECTION", "MANUAL_UNASSIGNED", 400, 1*time.Hour)
	e.use(b1, a1, oNew, 1, 400) // the newest recorded unit price of (b1,a1) wins: 2 × 400
	oFb1 := e.order("AWAITING_PAYMENT", "MANUAL_UNASSIGNED", 500, 2*time.Hour)
	e.forBuyer(b1, oFb1, "placed")
	oFb2 := e.order("CANCELLED", "CANCELLED", 700, 30*time.Minute)
	e.forBuyer(b1, oFb2, "released")

	code, out, raw, hdr := e.get(e.token, "/inbox/buyer-panel?conversation_id="+conv)
	if code != 200 {
		t.Fatalf("A13 status=%d body=%s", code, raw)
	}
	if got := hdr.Get("Cache-Control"); !strings.Contains(got, "no-store") {
		t.Errorf("A13 Cache-Control=%q, want no-store", got)
	}
	if bpStr(out, "platform") != "messenger" {
		t.Errorf("platform=%q body=%s", bpStr(out, "platform"), raw)
	}
	claims := bpArr(out, "claims")
	if len(claims) != 2 {
		t.Fatalf("claims=%v body=%s", claims, raw)
	}
	byKeyword := map[string]map[string]any{}
	for _, c := range claims {
		m := bpMap(c)
		byKeyword[bpStr(m, "keyword")] = m
		if len(m) != 4 { // the frozen A13 claim shape: exactly these four keys
			t.Errorf("claim row carries extra fields: %v", m)
		}
	}
	if m := byKeyword["A1"]; m == nil || bpStr(m, "session_id") != s1 || bpStr(m, "offer_id") != a1 || bpNum(m, "quantity") != 2 {
		t.Errorf("claim A1=%v want session=%s offer=%s qty=2", m, s1, a1)
	}
	if m := byKeyword["A2"]; m == nil || bpNum(m, "quantity") != 3 {
		t.Errorf("claim A2=%v want qty=3", m)
	}
	// 2×400 (recorded, newest use) + 3×0 (no recorded price) = 800; the decoy catalogue price would give 49995.
	if got := bpNum(out, "claim_total_minor"); got != 800 {
		t.Errorf("claim_total_minor=%v want 800 (recorded unit prices only)", got)
	}
	orders := bpArr(out, "orders")
	if len(orders) != 4 {
		t.Fatalf("orders=%v body=%s", orders, raw)
	}
	// newest first: CANCELLED for-buyer (30m), AWAITING_COLLECTION (1h), AWAITING_PAYMENT (2h), CONFIRMED (5h)
	wantOrder := []string{oFb2, oNew, oFb1, oOld}
	wantState := []string{"CANCELLED", "AWAITING_COLLECTION", "AWAITING_PAYMENT", "CONFIRMED"}
	for i, o := range orders {
		m := bpMap(o)
		if bpStr(m, "order_id") != wantOrder[i] || bpStr(m, "state") != wantState[i] {
			t.Fatalf("orders[%d]=%v want id=%s state=%s", i, m, wantOrder[i], wantState[i])
		}
		if want := "LC-" + strings.ToUpper(strings.ReplaceAll(wantOrder[i], "-", "")); bpStr(m, "number") != want {
			t.Errorf("orders[%d].number=%q want %q", i, bpStr(m, "number"), want)
		}
		if bpStr(m, "created_at") == "" {
			t.Errorf("orders[%d] has no created_at: %v", i, m)
		}
		if len(m) != 5 {
			t.Errorf("order row carries extra fields: %v", m)
		}
	}
	if got := bpNum(out, "purchase_ordinal"); got != 2 {
		t.Errorf("purchase_ordinal=%v want 2 (CONFIRMED + AWAITING_COLLECTION; not CANCELLED, not AWAITING_PAYMENT)", got)
	}
	if _, has := out["auto_reply"]; has {
		t.Errorf("auto_reply present without any automated reply: %s", raw)
	}
	if _, has := out["display_name"]; has {
		t.Errorf("display_name present without any conversation-held display copy: %s", raw)
	}
	if v, _ := out["link_pending_manual"].(bool); v {
		t.Errorf("link_pending_manual=%v want false", out["link_pending_manual"])
	}

	// The same panel by bundle: the claims/orders of that bundle alone, no conversation-scoped fields.
	code, out, raw = e.get3(e.token, "/inbox/buyer-panel?bundle_id="+b1)
	if code != 200 || len(bpArr(out, "claims")) != 2 || bpNum(out, "purchase_ordinal") != 2 {
		t.Fatalf("A13 by bundle: status=%d body=%s", code, raw)
	}
	if _, has := out["window_open_until"]; has {
		t.Errorf("A13 by bundle must not invent a conversation window: %s", raw)
	}
}

// A price-neutral claim checkout (order_origins only, no live_price_uses) is an order of the bundle and counts in purchase_ordinal.
func TestLiveConsoleBuyerPanelPriceNeutralOrigin(t *testing.T) {
	e := bpNew(t)
	s1 := e.session("bp-session-origin")
	a1 := e.offer(s1, "A1", 800)
	// Price-neutral means NULL; zero is rejected by the live_price_minor CHECK.
	e.exec(`UPDATE live.offers SET live_price_minor=NULL WHERE tenant_id=$1 AND store_id=$2 AND id=$3`, e.tenant, e.store1, a1)
	b1 := e.bundle(s1, "facebook", bpActorKey("bp-actor-origin"), nil, nil, false)
	e.line(b1, s1, a1, 1)
	conv := e.conversation(e.store1, "page", "1234567890")
	e.linkPeer(b1, conv, "page", "1234567890")
	oNeutral := e.order("CONFIRMED", "MANUAL_UNASSIGNED", 800, 3*time.Hour)
	e.origin(b1, a1, oNeutral, s1)
	oBoth := e.order("CONFIRMED", "MANUAL_UNASSIGNED", 900, 1*time.Hour)
	e.origin(b1, a1, oBoth, s1)
	e.use(b1, a1, oBoth, 1, 900) // in both ledgers: must be listed once

	code, out, raw, _ := e.get(e.token, "/inbox/buyer-panel?conversation_id="+conv)
	if code != 200 {
		t.Fatalf("A13 status=%d body=%s", code, raw)
	}
	orders := bpArr(out, "orders")
	if len(orders) != 2 || bpStr(bpMap(orders[0]), "order_id") != oBoth || bpStr(bpMap(orders[1]), "order_id") != oNeutral {
		t.Fatalf("orders=%v want [%s %s] (price-neutral origin included, deduped)", orders, oBoth, oNeutral)
	}
	if got := bpNum(out, "purchase_ordinal"); got != 2 {
		t.Errorf("purchase_ordinal=%v want 2", got)
	}
}

// ---------------------------------------------------------------------------
// Gates 2+3: an unlinked conversation shows none (P2-7c: 200 empty, not 404); a bundle whose owner_id matches but
// with no bundle_peers row shows NONE (I09 — owner_id never links).
// ---------------------------------------------------------------------------
func TestLiveConsoleBuyerPanelUnlinkedAndI09(t *testing.T) {
	e := bpNew(t)
	s1 := e.session("bp-session-i09")
	a1 := e.offer(s1, "A1", 700)
	b1 := e.bundle(s1, "facebook", bpActorKey("bp-actor-linked"), &e.owner1, nil, false) // owner1 on the LINKED bundle too: an owner_id hop (b1 -> owner1 -> b2) must not leak b2
	e.line(b1, s1, a1, 1)
	// b2 belongs to the SAME buyer owner — but no bundle_peers row links it to the conversation.
	b2 := e.bundle(s1, "facebook", bpActorKey("bp-actor-owned"), &e.owner1, nil, false)
	b2Offer := e.offer(s1, "B2", 800)
	e.line(b2, s1, b2Offer, 5)
	b2Order := e.order("CONFIRMED", "MANUAL_UNASSIGNED", 4000, time.Hour)
	e.use(b2, b2Offer, b2Order, 5, 800)

	conv := e.conversation(e.store1, "page", "1234567890")
	e.linkPeer(b1, conv, "page", "1234567890")

	code, out, raw, _ := e.get(e.token, "/inbox/buyer-panel?conversation_id="+conv)
	if code != 200 {
		t.Fatalf("A13 status=%d body=%s", code, raw)
	}
	for _, c := range bpArr(out, "claims") {
		if m := bpMap(c); bpStr(m, "offer_id") == b2Offer {
			t.Fatalf("I09 VIOLATION: the owner_id-matched bundle's claim appears: %s", raw)
		}
	}
	if got := bpNum(out, "claim_total_minor"); got != 0 {
		// b1's single line has no recorded price; b2's 5×800 must NOT count (no bundle_peers link).
		t.Errorf("claim_total_minor=%v want 0 (owner_id never links)", got)
	}
	for _, o := range bpArr(out, "orders") {
		if bpStr(bpMap(o), "order_id") == b2Order {
			t.Fatalf("I09 VIOLATION: the owner_id-matched bundle's order appears: %s", raw)
		}
	}
	if got := bpNum(out, "purchase_ordinal"); got != 0 {
		t.Errorf("purchase_ordinal=%v want 0 (b2's CONFIRMED order is not linked)", got)
	}
	if strings.Contains(raw, e.owner1) {
		t.Errorf("the buyer owner id leaked into the panel: %s", raw)
	}

	// A conversation with no bundle_peers row at all: 200 with empty claims/orders (Amendment 1 P2-7c), not a 404.
	plain := e.conversation(e.store1, "page", "1234567890")
	code, out, raw = e.get3(e.token, "/inbox/buyer-panel?conversation_id="+plain)
	if code != 200 {
		t.Fatalf("unlinked conversation: status=%d want 200 body=%s", code, raw)
	}
	if len(bpArr(out, "claims")) != 0 || len(bpArr(out, "orders")) != 0 || bpNum(out, "claim_total_minor") != 0 || bpNum(out, "purchase_ordinal") != 0 {
		t.Fatalf("unlinked conversation must show none: %s", raw)
	}

	// The explicit bundle path still answers for b2 (a merchant asking for a bundle is not an identity suggestion).
	code, out, raw = e.get3(e.token, "/inbox/buyer-panel?bundle_id="+b2)
	if code != 200 || len(bpArr(out, "claims")) != 1 || bpNum(out, "claim_total_minor") != 4000 || bpNum(out, "purchase_ordinal") != 1 {
		t.Fatalf("A13 by bundle for b2: status=%d body=%s", code, raw)
	}
}

// ---------------------------------------------------------------------------
// Gate 4: cross-store and cross-tenant are 404 (LCN03), for both query shapes.
// ---------------------------------------------------------------------------
func TestLiveConsoleBuyerPanelScope404(t *testing.T) {
	e := bpNew(t)
	s1 := e.session("bp-session-scope")
	b1 := e.bundle(s1, "facebook", bpActorKey("bp-actor-scope"), nil, nil, false)

	convStore2 := e.conversation(e.store2, "page", "1234567890")
	if code, _, raw := e.get3(e.token, "/inbox/buyer-panel?conversation_id="+convStore2); code != 404 {
		t.Errorf("cross-store conversation: status=%d want 404 body=%s", code, raw)
	}
	// store1's bundle queried through the store2 URL path (the token holds no store2 access at all).
	r := httptest.NewRequest("GET", "/v1/admin/stores/"+e.store2+"/inbox/buyer-panel?bundle_id="+b1, nil)
	r.Header.Set("Authorization", "Bearer "+e.token)
	w := httptest.NewRecorder()
	e.handler.ServeHTTP(w, r)
	if w.Code != 404 {
		t.Errorf("store2 path with a store1 principal: status=%d want 404 body=%s", w.Code, w.Body.String())
	}
	// Cross-tenant: a merchant of the shared fixture tenant reads its OWN conversation fine (control) ...
	_, otherToken := lcPerson(t, e.f, e.f.tenantA, e.f.storeA1, "inbox:read", "orders:read")
	convOther := lcConversation(t, e.f, e.f.tenantA, e.f.storeA1, "page")
	// Production realism for the control: a conversation exists because an inbound message arrived (see conversation()).
	mustExec(t, e.f.owner, `UPDATE inbox.conversation_state SET last_inbound_at=clock_timestamp(), last_inbound_seq=1
		WHERE tenant_id=$1 AND store_id=$2 AND conversation_id=$3`, e.f.tenantA, e.f.storeA1, convOther)
	r = httptest.NewRequest("GET", "/v1/admin/stores/"+e.f.storeA1+"/inbox/buyer-panel?conversation_id="+convOther, nil)
	r.Header.Set("Authorization", "Bearer "+otherToken)
	w = httptest.NewRecorder()
	e.handler.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("control: the other tenant's merchant reads its own conversation: status=%d body=%s", w.Code, w.Body.String())
	}
	// ... but store1's conversation is invisible to it, and vice versa.
	r = httptest.NewRequest("GET", "/v1/admin/stores/"+e.store1+"/inbox/buyer-panel?conversation_id="+convOther, nil)
	r.Header.Set("Authorization", "Bearer "+otherToken)
	w = httptest.NewRecorder()
	e.handler.ServeHTTP(w, r)
	if w.Code != 404 {
		t.Errorf("cross-tenant under store1: status=%d want 404 body=%s", w.Code, w.Body.String())
	}
	if code, _, raw := e.get3(e.token, "/inbox/buyer-panel?conversation_id="+convOther); code != 404 {
		t.Errorf("other tenant's conversation under store1: status=%d want 404 body=%s", code, raw)
	}
}

// ---------------------------------------------------------------------------
// Gate 5: without orders:read the "orders" key is OMITTED ENTIRELY (not an empty array); purchase_ordinal stays.
// ---------------------------------------------------------------------------
func TestLiveConsoleBuyerPanelOrdersPermission(t *testing.T) {
	e := bpNew(t)
	s1 := e.session("bp-session-perm")
	a1 := e.offer(s1, "A1", 100)
	b1 := e.bundle(s1, "facebook", bpActorKey("bp-actor-perm"), nil, nil, false)
	e.line(b1, s1, a1, 1)
	conv := e.conversation(e.store1, "page", "1234567890")
	e.linkPeer(b1, conv, "page", "1234567890")
	o1 := e.order("CONFIRMED", "MANUAL_UNASSIGNED", 100, time.Hour)
	e.use(b1, a1, o1, 1, 100)

	code, out, raw, _ := e.get(e.roToken, "/inbox/buyer-panel?conversation_id="+conv)
	if code != 200 {
		t.Fatalf("read-only A13 status=%d body=%s", code, raw)
	}
	if _, has := out["orders"]; has {
		t.Fatalf("orders key present without orders:read (must be omitted entirely): %s", raw)
	}
	if got := bpNum(out, "purchase_ordinal"); got != 1 {
		t.Errorf("purchase_ordinal=%v want 1 (the ordinal is not gated by orders:read)", got)
	}
	if len(bpArr(out, "claims")) != 1 || bpNum(out, "claim_total_minor") != 100 {
		t.Errorf("claims side changed for the read-only merchant: %s", raw)
	}

	code, out, raw = e.get3(e.token, "/inbox/buyer-panel?conversation_id="+conv)
	if code != 200 || len(bpArr(out, "orders")) != 1 {
		t.Fatalf("full A13: status=%d body=%s", code, raw)
	}
}

// ---------------------------------------------------------------------------
// Gate 6: bounds — 51 accepted claims render 50 (claim_total_minor still spans the full set); 22 orders render 20
// (purchase_ordinal still spans the full set).
// ---------------------------------------------------------------------------
func TestLiveConsoleBuyerPanelBounds(t *testing.T) {
	e := bpNew(t)
	s1 := e.session("bp-session-bounds")
	b1 := e.bundle(s1, "facebook", bpActorKey("bp-actor-bounds"), nil, nil, false)
	conv := e.conversation(e.store1, "page", "1234567890")
	e.linkPeer(b1, conv, "page", "1234567890")
	var hiddenOffer string
	for i := 1; i <= 51; i++ {
		off := e.offer(s1, fmt.Sprintf("K%02d", i), 1)
		e.line(b1, s1, off, 1)
		if i == 51 { // sorts last (keyword ASC within the session) → the row the 50-cap hides
			hiddenOffer = off
		}
	}
	hiddenOrder := e.order("CONFIRMED", "MANUAL_UNASSIGNED", 100000, 6*time.Hour)
	e.use(b1, hiddenOffer, hiddenOrder, 1, 100000) // the hidden claim carries the only recorded price
	// One placed for-buyer order per bundle (the order_for_buyer_live unique index), so 20 sibling bundles,
	// each peer-linked to the conversation; none of them carries claim lines.
	for i := 0; i < 20; i++ {
		fb := e.bundle(s1, "facebook", bpActorKey(fmt.Sprintf("bp-actor-bounds-fb%d", i)), nil, nil, false)
		e.linkPeer(fb, conv, "page", "1234567890")
		o := e.order("CONFIRMED", "MANUAL_UNASSIGNED", 10, time.Duration(i+1)*time.Hour)
		e.forBuyer(fb, o, "placed")
	}
	e.forBuyer(b1, e.order("CANCELLED", "CANCELLED", 10, 30*time.Hour), "released") // the 22nd order, hidden by the 20 cap

	code, out, raw, _ := e.get(e.token, "/inbox/buyer-panel?conversation_id="+conv)
	if code != 200 {
		t.Fatalf("A13 bounds status=%d body=%s", code, raw)
	}
	if n := len(bpArr(out, "claims")); n != 50 {
		t.Errorf("claims=%d want 50 (cap)", n)
	}
	for _, c := range bpArr(out, "claims") {
		if bpStr(bpMap(c), "offer_id") == hiddenOffer {
			t.Errorf("the last-sorted claim must be the one hidden by the cap: %v", c)
		}
	}
	if got := bpNum(out, "claim_total_minor"); got != 100000 {
		t.Errorf("claim_total_minor=%v want 100000 (spans the full set, not the 50 shown)", got)
	}
	if n := len(bpArr(out, "orders")); n != 20 {
		t.Errorf("orders=%d want 20 (cap)", n)
	}
	if got := bpNum(out, "purchase_ordinal"); got != 21 {
		t.Errorf("purchase_ordinal=%v want 21 (20 for-buyer CONFIRMED + the claim-checkout CONFIRMED; CANCELLED excluded)", got)
	}
}

// ---------------------------------------------------------------------------
// Gate 7: auto_reply is the NEWEST AUTOMATED private reply of the linked bundles (manual sends never count),
// rendered through §4.4's send_state; absent when there is none.
// ---------------------------------------------------------------------------
func TestLiveConsoleBuyerPanelAutoReply(t *testing.T) {
	e := bpNew(t)
	s1 := e.session("bp-session-auto")
	b1 := e.bundle(s1, "facebook", bpActorKey("bp-actor-auto"), nil, nil, false)
	conv := e.conversation(e.store1, "page", "1234567890")
	e.linkPeer(b1, conv, "page", "1234567890")
	e.autoReply(b1, "FAILED_FINAL", "auto", 2*time.Hour)
	e.autoReply(b1, "SUCCEEDED", "auto", time.Hour)
	e.autoReply(b1, "BLOCKED_POLICY", "manual", time.Minute) // newest overall, but human — must not win

	code, out, raw, _ := e.get(e.token, "/inbox/buyer-panel?conversation_id="+conv)
	if code != 200 {
		t.Fatalf("A13 auto status=%d body=%s", code, raw)
	}
	ar := bpMap(out["auto_reply"])
	if ar == nil || bpStr(ar, "send_state") != "sent" {
		t.Fatalf("auto_reply=%v want {send_state:sent} (newest AUTO operation; the older FAILED_FINAL maps to failed, the manual send is ignored) body=%s", out["auto_reply"], raw)
	}

	// A linked bundle with no automated reply at all: the key stays absent.
	b2 := e.bundle(s1, "facebook", bpActorKey("bp-actor-auto2"), nil, nil, false)
	conv2 := e.conversation(e.store1, "page", "1234567890")
	e.linkPeer(b2, conv2, "page", "1234567890")
	code, out, raw = e.get3(e.token, "/inbox/buyer-panel?conversation_id="+conv2)
	if code != 200 {
		t.Fatalf("A13 no-auto status=%d body=%s", code, raw)
	}
	if _, has := out["auto_reply"]; has {
		t.Errorf("auto_reply present with no automated reply for these bundles: %s", raw)
	}
}

// ---------------------------------------------------------------------------
// Gate 8: A8 — session_id filters conversations by their bundle_peers link; filter=live_comment returns the
// bundle-only rows of live comments in scope (with the real link_pending_manual flag); every item carries
// link_version.
// ---------------------------------------------------------------------------
func TestLiveConsoleBuyerPanelA8SessionAndLiveComment(t *testing.T) {
	e := bpNew(t)
	s1 := e.session("bp-session-a8-1")
	s2 := e.session("bp-session-a8-2")
	b1 := e.bundle(s1, "facebook", bpActorKey("bp-actor-a8-1"), nil, nil, false)
	b2 := e.bundle(s1, "instagram", bpActorKey("bp-actor-a8-2"), nil, nil, false)
	b3 := e.bundle(s1, "facebook", bpActorKey("bp-actor-a8-3"), nil, nil, true) // link pending, no conversation
	b5 := e.bundle(s2, "facebook", bpActorKey("bp-actor-a8-5"), nil, nil, false)

	c1 := e.conversation(e.store1, "page", "1234567890")
	e.linkPeer(b1, c1, "page", "1234567890")
	c2 := e.conversation(e.store1, "instagram", "1234567890")
	e.linkPeer(b2, c2, "instagram", "1234567890")
	c3 := e.conversation(e.store1, "page", "1234567890")
	e.linkPeer(b5, c3, "page", "1234567890")

	// Bump c1's link version through the real A14 write so link_version is observably non-zero.
	code, _, raw := e.post(e.token, "/inbox/conversations/"+c1+"/customer-link", "bp-a8-link-"+randomUUID(),
		map[string]any{"customer_id": e.owner1, "expected_version": 0})
	if code != 200 {
		t.Fatalf("A14 seed link: status=%d body=%s", code, raw)
	}

	listIDs := func(path string) map[string]map[string]any {
		code, out, raw, _ := e.get(e.token, path)
		if code != 200 {
			t.Fatalf("A8 %s: status=%d body=%s", path, code, raw)
		}
		items := map[string]map[string]any{}
		for _, it := range bpArr(out, "items") {
			m := bpMap(it)
			key := bpStr(m, "conversation_id")
			if key == "" {
				key = "bundle:" + bpStr(m, "bundle_id")
			}
			items[key] = m
		}
		return items
	}

	all := listIDs("/inbox/conversations?filter=all")
	for _, conv := range []string{c1, c2, c3} {
		if _, ok := all[conv]; !ok {
			t.Fatalf("filter=all missing %s: %v", conv, all)
		}
	}
	if v, ok := all[c1]["link_version"]; !ok || v == nil {
		t.Errorf("c1 item has no link_version: %v", all[c1])
	} else if int(bpNum(all[c1], "link_version")) != 1 {
		t.Errorf("c1 link_version=%v want 1 (the A14 write bumped it)", v)
	}
	if v, ok := all[c2]["link_version"]; !ok || v == nil {
		t.Errorf("c2 item has no link_version: %v", all[c2])
	}

	s1Only := listIDs("/inbox/conversations?filter=all&session_id=" + s1)
	if _, ok := s1Only[c3]; ok {
		t.Errorf("session_id=%s leaked the session-2 conversation: %v", s1, s1Only)
	}
	for _, conv := range []string{c1, c2} {
		if _, ok := s1Only[conv]; !ok {
			t.Errorf("session_id=%s lost its conversation %s: %v", s1, conv, s1Only)
		}
	}
	s2Only := listIDs("/inbox/conversations?filter=all&session_id=" + s2)
	if len(s2Only) != 1 || s2Only[c3] == nil {
		t.Errorf("session_id=%s: %v want exactly {c3}", s2, s2Only)
	}

	// filter=live_comment: bundle-only rows of live comments in scope (the whole store when no session is given):
	// b1/b2/b3 of s1 AND b5 of s2 — facebook renders as messenger, instagram stays instagram.
	lc := listIDs("/inbox/conversations?filter=live_comment")
	if len(lc) != 4 {
		t.Fatalf("live_comment items=%d %v want 4 (b1,b2,b3,b5)", len(lc), lc)
	}
	for key, want := range map[string]struct {
		platform string
		flag     bool
		session  string
	}{
		"bundle:" + b1: {"messenger", false, s1},
		"bundle:" + b2: {"instagram", false, s1},
		"bundle:" + b3: {"messenger", true, s1},
		"bundle:" + b5: {"messenger", false, s2},
	} {
		m, ok := lc[key]
		if !ok {
			t.Fatalf("live_comment missing %s: %v", key, lc)
		}
		if m["conversation_id"] != nil {
			t.Errorf("%s: conversation_id=%v want null", key, m["conversation_id"])
		}
		if bpStr(m, "platform") != want.platform {
			t.Errorf("%s: platform=%q want %q", key, bpStr(m, "platform"), want.platform)
		}
		if got, _ := m["link_pending_manual"].(bool); got != want.flag {
			t.Errorf("%s: link_pending_manual=%v want %v", key, m["link_pending_manual"], want.flag)
		}
		if bpStr(m, "session_id") != want.session {
			t.Errorf("%s: session_id=%v want %s", key, m["session_id"], want.session)
		}
		if v, ok := m["link_version"]; !ok || v != nil {
			t.Errorf("%s: link_version=%v want explicit null (a bundle row has no conversation version)", key, v)
		}
	}
	lcS2 := listIDs("/inbox/conversations?filter=live_comment&session_id=" + s2)
	if len(lcS2) != 1 || lcS2["bundle:"+b5] == nil {
		t.Errorf("live_comment&session_id=%s: %v want exactly {b5}", s2, lcS2)
	}
}

// ---------------------------------------------------------------------------
// Gate 9: A9 exposes link_version + binding_id; A14 CAS with the exposed link_version succeeds and a stale one is 409.
// ---------------------------------------------------------------------------
func TestLiveConsoleBuyerPanelA9A14(t *testing.T) {
	e := bpNew(t)
	conv := e.conversation(e.store1, "page", "1234567890")
	igConv := e.conversation(e.store1, "instagram", "1234567890")
	noBind := e.conversation(e.store1, "page", "555000111") // an asset with no binding row

	code, out, raw, _ := e.get(e.token, "/inbox/conversations/"+conv+"/messages")
	if code != 200 {
		t.Fatalf("A9 status=%d body=%s", code, raw)
	}
	v, ok := out["link_version"]
	if !ok || v == nil {
		t.Fatalf("A9 has no link_version: %s", raw)
	}
	if int(bpNum(out, "link_version")) != 0 {
		t.Errorf("A9 link_version=%v want 0", v)
	}
	convLV := int(bpNum(out, "link_version")) // captured before the other GETs reassign out
	if bpStr(out, "binding_id") != e.fbBinding {
		t.Errorf("A9 binding_id=%v want %s (the facebook binding of the conversation's asset)", out["binding_id"], e.fbBinding)
	}

	code, out, raw, _ = e.get(e.token, "/inbox/conversations/"+igConv+"/messages")
	if code != 200 || bpStr(out, "binding_id") != e.igBinding {
		t.Errorf("A9 instagram binding_id: status=%d body=%s", code, raw)
	}
	code, out, raw, _ = e.get(e.token, "/inbox/conversations/"+noBind+"/messages")
	if code != 200 || out["binding_id"] != nil {
		t.Errorf("A9 without a binding: status=%d binding_id=%v want null body=%s", code, out["binding_id"], raw)
	}

	// A14 CAS with the exposed link_version succeeds; the same expected_version afterwards is stale → 409.
	code, out, raw = e.post(e.token, "/inbox/conversations/"+conv+"/customer-link", "bp-a14-"+randomUUID(),
		map[string]any{"customer_id": e.owner1, "expected_version": convLV})
	if code != 200 {
		t.Fatalf("A14 with the exposed link_version: status=%d body=%s", code, raw)
	}
	if int(bpNum(out, "version")) != 1 {
		t.Errorf("A14 version=%v want 1", out["version"])
	}
	code, _, raw = e.post(e.token, "/inbox/conversations/"+conv+"/customer-link", "bp-a14-stale-"+randomUUID(),
		map[string]any{"customer_id": e.owner1, "expected_version": 0})
	if code != 409 || bpStr(raw2map(raw), "code") != "version_conflict" {
		t.Errorf("A14 stale: status=%d want 409 version_conflict body=%s", code, raw)
	}
	// A9 now exposes the bumped version.
	code, out, raw, _ = e.get(e.token, "/inbox/conversations/"+conv+"/messages")
	if code != 200 || int(bpNum(out, "link_version")) != 1 {
		t.Errorf("A9 after A14: status=%d link_version=%v want 1 body=%s", code, out["link_version"], raw)
	}
}

func raw2map(raw string) map[string]any {
	var m map[string]any
	_ = json.Unmarshal([]byte(raw), &m)
	return m
}

// ---------------------------------------------------------------------------
// Gate 10: leak scan — sentinel names (session title, product/SKU code, manual-bundle label, actor key) and the
// PSID-side identifiers (peer_key, buyer owner id) are absent from every response field the amendment does not
// specify. Non-vacuous: the specified fields (keyword, order id) must appear.
// ---------------------------------------------------------------------------
func TestLiveConsoleBuyerPanelLeakScan(t *testing.T) {
	e := bpNew(t)
	const (
		sentinelTitle = "BPSENTINELTITLE"
		sentinelSKU   = "BPSENTINELSKU"
		sentinelLabel = "BPSENTINELLABEL"
	)
	sentinelSku := e.sku(sentinelSKU, 900) // the sku() helper names the product after the code: both carry the sentinel
	s1 := e.session(sentinelTitle)
	a1 := e.offerOnSku(s1, "A1", 4242, sentinelSku)
	actor := bpActorKey(sentinelLabel + "-actor")
	label := sentinelLabel
	manual := e.bundle(s1, "manual", actor, &e.owner1, &label, false)
	e.line(manual, s1, a1, 2)
	fb := e.bundle(s1, "facebook", bpActorKey("bp-actor-leak-fb"), &e.owner1, nil, false)
	e.line(fb, s1, a1, 1)
	conv := e.conversation(e.store1, "page", "1234567890")
	e.linkPeer(fb, conv, "page", "1234567890")
	e.linkPeer(manual, conv, "page", "1234567890")
	o1 := e.order("CONFIRMED", "MANUAL_UNASSIGNED", 4242, time.Hour)
	e.use(fb, a1, o1, 1, 4242)
	e.autoReply(fb, "SUCCEEDED", "auto", time.Hour)

	sentinels := []string{sentinelTitle, sentinelSKU, sentinelLabel, actor, e.peerKey(conv), e.owner1}
	bodies := map[string]string{}
	_, _, bodies["a13_conv"], _ = e.get(e.token, "/inbox/buyer-panel?conversation_id="+conv)
	_, _, bodies["a13_bundle"], _ = e.get(e.token, "/inbox/buyer-panel?bundle_id="+fb)
	_, _, bodies["a8_all"], _ = e.get(e.token, "/inbox/conversations?filter=all")
	_, _, bodies["a8_live_comment"], _ = e.get(e.token, "/inbox/conversations?filter=live_comment")
	_, _, bodies["a9"], _ = e.get(e.token, "/inbox/conversations/"+conv+"/messages")
	for name, body := range bodies {
		for _, s := range sentinels {
			if s != "" && strings.Contains(body, s) {
				t.Errorf("%s leaks sentinel %q: %s", name, s, body)
			}
		}
	}
	// The keyword and the order id ARE specified (claims/orders rows), so they must appear — a leak scan that
	// passes on an empty body is vacuous.
	if !strings.Contains(bodies["a13_conv"], "A1") {
		t.Errorf("non-vacuity: the claims keyword is missing from A13: %s", bodies["a13_conv"])
	}
	if !strings.Contains(bodies["a13_conv"], o1) {
		t.Errorf("non-vacuity: the order id is missing from A13: %s", bodies["a13_conv"])
	}
}

// ---------------------------------------------------------------------------
// Gate 11: exact ACL of every new/extended definer of migration 0165 (owner, result type, SECURITY DEFINER,
// fixed search_path, volatility, EXECUTE grantees — and no other commerce_% role), plus the dropped old signature.
// ---------------------------------------------------------------------------
func TestLiveConsoleBuyerPanelExactACL(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	functions := []struct {
		signature string
		owner     string
		result    string
		stable    bool
		grantees  []string
	}{
		{"social.list_conversations(text,uuid,timestamptz,uuid,int)", "commerce_meta_writer",
			"TABLE(conversation_id uuid, platform text, last_at timestamp with time zone, unread boolean, unreplied boolean, mode text, assignee uuid, window_open_until timestamp with time zone, linked_customer_id uuid, link_version bigint)", true,
			[]string{"commerce_meta_writer", "commerce_runtime"}},
		{"social.conversation_meta(uuid)", "commerce_meta_writer",
			"TABLE(platform text, mode text, assignee uuid, takeover_generation bigint, human_until timestamp with time zone, window_open_until timestamp with time zone, last_inbound_at timestamp with time zone, last_inbound_seq bigint, read_seq bigint, linked_customer_id uuid, status text, link_version bigint)", true,
			[]string{"commerce_meta_writer", "commerce_runtime"}},
		{"claims.buyer_panel_claims(uuid,uuid,uuid[])", "commerce_claims_writer", "jsonb", true,
			[]string{"commerce_claims_writer", "commerce_integration_writer"}},
		{"claims.orders_of_bundles(uuid,uuid,uuid[])", "commerce_claims_writer", "TABLE(order_id uuid)", true,
			[]string{"commerce_claims_writer", "commerce_integration_writer"}},
		{"claims.session_peer_linked(uuid,uuid,uuid,text,text,text,text)", "commerce_claims_writer", "boolean", true,
			[]string{"commerce_claims_writer", "commerce_meta_writer"}},
		{"checkout.order_panel_facts(uuid,uuid,uuid[])", "commerce_checkout_writer",
			"TABLE(order_id uuid, commercial_state text, total_minor bigint, created_at timestamp with time zone)", true,
			[]string{"commerce_checkout_writer", "commerce_integration_writer"}},
		{"inbox.buyer_panel(uuid,uuid)", "commerce_integration_writer", "jsonb", true,
			[]string{"commerce_integration_writer", "commerce_runtime"}},
		{"inbox.link_pending_bundles(int,uuid)", "commerce_integration_writer",
			"TABLE(bundle_id uuid, session_id uuid, created_at timestamp with time zone)", true,
			[]string{"commerce_integration_writer", "commerce_runtime"}},
		{"inbox.live_comment_bundles(uuid,int)", "commerce_integration_writer",
			"TABLE(bundle_id uuid, session_id uuid, platform text, created_at timestamp with time zone, link_pending_manual boolean)", true,
			[]string{"commerce_integration_writer", "commerce_runtime"}},
		{"inbox.live_comment_bundles(uuid,int,timestamptz,uuid)", "commerce_integration_writer",
			"TABLE(bundle_id uuid, session_id uuid, platform text, created_at timestamp with time zone, link_pending_manual boolean)", true,
			[]string{"commerce_integration_writer", "commerce_runtime"}},
		{"inbox.conversation_binding(uuid)", "commerce_integration_writer", "uuid", true,
			[]string{"commerce_integration_writer", "commerce_runtime"}},
	}
	for _, fn := range functions {
		t.Run(fn.signature, func(t *testing.T) {
			var owner, result, volatility string
			var definer, noLogin, noBypass, fixedPath bool
			var principals []string
			err := f.owner.QueryRow(ctx, `SELECT pg_get_userbyid(p.proowner), pg_get_function_result(p.oid),
			 p.prosecdef, NOT r.rolcanlogin, NOT r.rolbypassrls,
			 p.proconfig=ARRAY['search_path=pg_catalog']::text[], p.provolatile,
			 ARRAY(SELECT CASE WHEN a.grantee=0 THEN 'PUBLIC' ELSE pg_get_userbyid(a.grantee) END
			   FROM aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a
			   WHERE a.privilege_type='EXECUTE' ORDER BY 1)
			 FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid=$1::regprocedure`, fn.signature).
				Scan(&owner, &result, &definer, &noLogin, &noBypass, &fixedPath, &volatility, &principals)
			if err != nil {
				t.Fatal(err)
			}
			wantVolatility := "v"
			if fn.stable {
				wantVolatility = "s"
			}
			if owner != fn.owner || result != fn.result || !definer || !noLogin || !noBypass || !fixedPath || volatility != wantVolatility ||
				!slices.Equal(principals, fn.grantees) {
				t.Fatalf("0165 boundary: owner=%s result=%s definer=%v noLogin=%v noBypass=%v path=%v volatility=%s ACL=%v", owner, result, definer, noLogin, noBypass, fixedPath, volatility, principals)
			}
			var others int
			if err := f.owner.QueryRow(ctx, `SELECT count(*) FROM pg_roles WHERE rolname LIKE 'commerce\_%'
			 AND NOT (rolname = ANY($1::text[])) AND has_function_privilege(oid, $2, 'EXECUTE')`, fn.grantees, fn.signature).Scan(&others); err != nil || others != 0 {
				t.Fatalf("unexpected helper callers=%d err=%v", others, err)
			}
		})
	}
	// The superseded 0119 list signature must be gone (no orphan overload left callable).
	var n int
	if err := f.owner.QueryRow(ctx, `SELECT count(*) FROM pg_proc p WHERE p.oid=to_regprocedure($1)`,
		"social.list_conversations(text,timestamptz,uuid,int)").Scan(&n); err != nil || n != 0 {
		t.Fatalf("old list_conversations signature still present (n=%d err=%v)", n, err)
	}
}
