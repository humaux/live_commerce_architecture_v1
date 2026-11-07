package foundation_test

// Purpose: LP-1 acceptance gates of contracts/loyalty-points-v1.md §5 — LPG01a, LPG01b, LPG02-LPG06, LPG06r, LPG07-LPG11 (LPG11's
//   transport half is DB-free in internal/httpapi/members_test.go). Every failed call asserts its exact refusal code.
// Runs as/in: the shared fixture PG (REAL_PG). Member gates use a fresh tenant+store per test (lpNewStore) and call internal/loyalty
//   inside platform.WithScope opened with store:read only, so the definer — not the route — decides every right. Manual-order gates
//   go through the REAL merchant handler and buyer pipeline (lpOrderEnv, the mtOrderEnv wiring) with the ManualOrders test seams.
// Disclosed owner-pool fixtures: store/tenant/principal rows and grants, the 1e12 balance preset of LPG01b/LPG03, the grant revoke of
//   LPG06r(b), the simulated restore of LPG06 (session_replication_role=replica) and the rolled-back probes of LPG08.
// Fixtures: synthetic 測試會員 names and lp-marker strings only; no real buyer data. No provider is contacted.
// Used by: GitHub CI (catch-all shard); bash scripts/dev/test-focused.sh '^TestLoyalty'.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/command"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/loyalty"
	"livecommerce/internal/merchanttools"
	"livecommerce/internal/platform"
)

const lpMarker = "lp-marker"

var lpAll = []string{"store:read", "customers:read", "customers:points", "customers:privacy"}

// lpStore is one fresh tenant + store with an admin principal holding every loyalty right.
type lpStore struct {
	t              *testing.T
	f              *testFixture
	tenant, store  string
	adminID, admin string
	keySeq         int
	keyMu          sync.Mutex
}

func lpNewStore(t *testing.T) *lpStore {
	t.Helper()
	f := fixture(t)
	s := &lpStore{t: t, f: f, tenant: randomUUID(), store: randomUUID()}
	mustExec(t, f.owner, `INSERT INTO control.tenants(id,name) VALUES($1,'loyalty test tenant')`, s.tenant)
	mustExec(t, f.owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'loyalty test store','TWD')`, s.tenant, s.store)
	s.adminID, s.admin = lcPrincipal(t, f, s.tenant, []string{s.store}, lpAll...)
	return s
}

func (s *lpStore) key(prefix string) string {
	s.keyMu.Lock()
	defer s.keyMu.Unlock()
	s.keySeq++
	return t04Key(fmt.Sprintf("%s-%d", prefix, s.keySeq))
}

func (s *lpStore) principal(perms ...string) string {
	s.t.Helper()
	_, token := lcPrincipal(s.t, s.f, s.tenant, []string{s.store}, append([]string{"store:read"}, perms...)...)
	return token
}

// rolePrincipal holds exactly the staff bundle of role (identity.staff_role_permissions) plus store:read.
func (s *lpStore) rolePrincipal(role string) string {
	s.t.Helper()
	var perms []string
	if err := s.f.owner.QueryRow(context.Background(), `SELECT identity.staff_role_permissions($1)`, role).Scan(&perms); err != nil {
		s.t.Fatal(err)
	}
	if !slices.Contains(perms, "store:read") {
		perms = append(perms, "store:read")
	}
	_, token := lcPrincipal(s.t, s.f, s.tenant, []string{s.store}, perms...)
	return token
}

// run opens the merchant transaction with store:read only: every loyalty right is then decided by the definer (and Go fence).
func (s *lpStore) run(token string, fn func(ctx context.Context, tx pgx.Tx, sc platform.Scope) error) error {
	ctx := context.Background()
	return platform.WithScope(ctx, s.f.runtime, token, s.store, "store:read", func(tx pgx.Tx, sc platform.Scope) error { return fn(ctx, tx, sc) })
}

func (s *lpStore) create(token, key string, in loyalty.MemberInput) (m loyalty.Member, replayed bool, err error) {
	err = s.run(token, func(ctx context.Context, tx pgx.Tx, sc platform.Scope) (e error) {
		m, replayed, e = loyalty.CreateMember(ctx, tx, sc, token, key, in)
		return e
	})
	return
}

func (s *lpStore) mustMember(name string, contact, notes *string) loyalty.Member {
	s.t.Helper()
	m, _, err := s.create(s.admin, s.key("lp-create"), loyalty.MemberInput{DisplayName: name, ContactNote: contact, Notes: notes})
	if err != nil {
		s.t.Fatalf("create member: %v", err)
	}
	return m
}

func (s *lpStore) update(token, key, member string, p loyalty.MemberPatch) (m loyalty.Member, replayed bool, err error) {
	err = s.run(token, func(ctx context.Context, tx pgx.Tx, sc platform.Scope) (e error) {
		m, replayed, e = loyalty.UpdateMember(ctx, tx, sc, token, key, member, p)
		return e
	})
	return
}

func (s *lpStore) adjust(token, key, member string, delta int64, note string, order *string) (r loyalty.AdjustResult, replayed bool, err error) {
	err = s.run(token, func(ctx context.Context, tx pgx.Tx, sc platform.Scope) (e error) {
		r, replayed, e = loyalty.Adjust(ctx, tx, sc, token, key, member, loyalty.AdjustInput{Delta: delta, Note: note, OrderID: order})
		return e
	})
	return
}

// adjustRetry is the client rule of contract §2: 503 retry_later is retried with the SAME key (no new effect is possible).
func (s *lpStore) adjustRetry(token, key, member string, delta int64, note string) (loyalty.AdjustResult, error) {
	var r loyalty.AdjustResult
	var err error
	for attempt := 0; attempt < 6; attempt++ {
		r, _, err = s.adjust(token, key, member, delta, note, nil)
		if status, code := lpClassify(err); status != 503 || code != "retry_later" {
			return r, err
		}
	}
	return r, err
}

func (s *lpStore) mustAdjust(member string, delta int64, note string) loyalty.AdjustResult {
	s.t.Helper()
	r, _, err := s.adjust(s.admin, s.key("lp-adjust"), member, delta, note, nil)
	if err != nil {
		s.t.Fatalf("adjust %d: %v", delta, err)
	}
	return r
}

func (s *lpStore) erase(token, key, member string) (r loyalty.Erasure, replayed bool, err error) {
	err = s.run(token, func(ctx context.Context, tx pgx.Tx, sc platform.Scope) (e error) {
		r, replayed, e = loyalty.Erase(ctx, tx, sc, token, key, member)
		return e
	})
	return
}

func (s *lpStore) export(token, key, member string) (body []byte, replayed bool, err error) {
	err = s.run(token, func(ctx context.Context, tx pgx.Tx, sc platform.Scope) (e error) {
		body, replayed, e = loyalty.Export(ctx, tx, sc, token, key, member)
		return e
	})
	return
}

func (s *lpStore) get(token, member, cursor string) (d loyalty.Detail, err error) {
	err = s.run(token, func(ctx context.Context, tx pgx.Tx, sc platform.Scope) (e error) {
		d, e = loyalty.Get(ctx, tx, sc, token, member, cursor)
		return e
	})
	return
}

func (s *lpStore) search(token string, in loyalty.SearchInput) (r loyalty.SearchResult, err error) {
	err = s.run(token, func(ctx context.Context, tx pgx.Tx, sc platform.Scope) (e error) {
		r, e = loyalty.Search(ctx, tx, sc, token, in)
		return e
	})
	return
}

func (s *lpStore) orderEntries(token, order string) (r loyalty.OrderEntries, err error) {
	err = s.run(token, func(ctx context.Context, tx pgx.Tx, sc platform.Scope) (e error) {
		r, e = loyalty.EntriesForOrder(ctx, tx, sc, token, order)
		return e
	})
	return
}

// balance reads the stored balance and SUM(delta) of a member (owner pool, read-only).
func (s *lpStore) balance(member string) (stored, sum int64) {
	s.t.Helper()
	if err := s.f.owner.QueryRow(context.Background(), `SELECT m.balance,coalesce((SELECT sum(e.delta) FROM loyalty.point_entries e
		WHERE e.tenant_id=m.tenant_id AND e.store_id=m.store_id AND e.member_id=m.id),0) FROM loyalty.members m WHERE m.id=$1`, member).Scan(&stored, &sum); err != nil {
		s.t.Fatalf("balance of %s: %v", member, err)
	}
	return
}

func (s *lpStore) entries(member string) int {
	return countRows(s.t, s.f.owner, `SELECT count(*) FROM loyalty.point_entries WHERE member_id=$1`, member)
}

// lpClassify is the refusal the merchant sees: loyalty.Classify first (the members.go order), then the shared platform sentinels.
func lpClassify(err error) (int, string) {
	if err == nil {
		return 0, ""
	}
	if status, code, ok := loyalty.Classify(err); ok {
		return status, code
	}
	switch {
	case errors.Is(err, platform.ErrUnauthorized):
		return 401, "unauthorized"
	case errors.Is(err, platform.ErrForbidden):
		return 403, "forbidden"
	case errors.Is(err, platform.ErrScopeNotFound), errors.Is(err, command.ErrNotFound):
		return 404, "not_found"
	case errors.Is(err, command.ErrInvalid):
		return 422, "invalid_request"
	}
	return 500, "unclassified: " + err.Error()
}

func lpWant(t *testing.T, label string, err error, status int, code string) {
	t.Helper()
	if s, c := lpClassify(err); s != status || c != code {
		t.Errorf("%s: got %d %q (err=%v), want %d %q", label, s, c, err, status, code)
	}
}

func lpStr(s string) *string { return &s }

// ---------------------------------------------------------------------------------------------------------------------------
// LPG01a / LPG01b: concurrent deductions serialize on the member row; both bounds refuse with their code, never a 23514.
// ---------------------------------------------------------------------------------------------------------------------------

// lpDeductLoad: balance 100, 50 concurrent -10 from two staff sessions -> exactly 10 succeed, 40 points_insufficient, balance 0.
func lpDeductLoad(t *testing.T, s *lpStore, staff2 string) {
	t.Helper()
	m := s.mustMember("測試會員-併發", nil, nil)
	s.mustAdjust(m.ID, 100, "開帳")
	var mu sync.Mutex
	codes := map[string]int{}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			token := s.admin
			if i%2 == 1 {
				token = staff2
			}
			_, err := s.adjustRetry(token, s.key("lp-load"), m.ID, -10, "扣點")
			status, code := lpClassify(err)
			mu.Lock()
			codes[fmt.Sprintf("%d %s", status, code)]++
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	if codes["0 "] != 10 || codes["409 points_insufficient"] != 40 || len(codes) != 2 {
		t.Errorf("50 concurrent -10 on 100: %v, want exactly 10 ok and 40 409 points_insufficient", codes)
	}
	if stored, sum := s.balance(m.ID); stored != 0 || sum != 0 {
		t.Errorf("after the load: balance=%d SUM(delta)=%d, want 0 and 0", stored, sum)
	}
}

func TestLoyaltyLPG01aConcurrentDeductions(t *testing.T) {
	s := lpNewStore(t)
	lpDeductLoad(t, s, s.principal("customers:points"))
}

func TestLoyaltyLPG01bBoundsNeverReachACheck(t *testing.T) {
	s := lpNewStore(t)
	t.Run("same load", func(t *testing.T) { lpDeductLoad(t, s, s.principal("customers:points")) })
	t.Run("above 1e12 is 422 limit_reached, nothing written", func(t *testing.T) {
		m := s.mustMember("測試會員-上限", nil, nil)
		// Disclosed fixture: a balance near the cap cannot be reached by <= 1e6 entries in a test (LPI1 is checked on every other member).
		mustExec(t, s.f.owner, `UPDATE loyalty.members SET balance=999999999995 WHERE id=$1`, m.ID)
		_, _, err := s.adjust(s.admin, s.key("lp-cap"), m.ID, 10, "超過上限", nil)
		lpWant(t, "adjust past 1e12", err, 422, "limit_reached")
		if stored, _ := s.balance(m.ID); stored != 999999999995 || s.entries(m.ID) != 0 {
			t.Errorf("refused adjust wrote: balance=%d entries=%d", stored, s.entries(m.ID))
		}
		if r, _, err := s.adjust(s.admin, s.key("lp-cap"), m.ID, 5, "剛好上限", nil); err != nil || r.Balance != 1000000000000 {
			t.Errorf("adjust to exactly 1e12: %+v %v", r, err)
		}
	})
	t.Run("below 0 is 409 points_insufficient, never clamped", func(t *testing.T) {
		m := s.mustMember("測試會員-不足", nil, nil)
		s.mustAdjust(m.ID, 5, "開帳")
		_, _, err := s.adjust(s.admin, s.key("lp-neg"), m.ID, -6, "扣太多", nil)
		lpWant(t, "deduct below 0", err, 409, "points_insufficient")
		if stored, sum := s.balance(m.ID); stored != 5 || sum != 5 {
			t.Errorf("balance=%d sum=%d, want 5/5 (no clamp)", stored, sum)
		}
	})
	t.Run("input bounds", func(t *testing.T) {
		m := s.mustMember("測試會員-輸入", nil, nil)
		for _, c := range []struct {
			delta int64
			note  string
		}{{0, "零"}, {1_000_001, "太大"}, {-1_000_001, "太小"}, {1, ""}, {1, strings.Repeat("長", 201)}, {1, "換\n行"}} {
			_, _, err := s.adjust(s.admin, s.key("lp-in"), m.ID, c.delta, c.note, nil)
			lpWant(t, fmt.Sprintf("adjust delta=%d note=%q", c.delta, c.note), err, 422, "invalid_request")
		}
		_, _, err := s.adjust(s.admin, s.key("lp-in"), m.ID, 1, "未知訂單", lpStr(randomUUID()))
		lpWant(t, "adjust referencing an unknown order", err, 404, "not_found")
	})
}

// ---------------------------------------------------------------------------------------------------------------------------
// Manual-order gates (LPG02, LPG03, LPG04 manual part, LPG06r, LPG07 order part) through the REAL merchant handler.
// ---------------------------------------------------------------------------------------------------------------------------

type lpOrderEnv struct {
	e       *tcvEnv
	a       mtAdmin
	manual  *merchanttools.ManualOrders
	s       *lpStore
	sku     string
	homeKey string
}

func lpNewOrderEnv(t *testing.T) *lpOrderEnv {
	t.Helper()
	e := tcvNew(t)
	e.grantCreator("orders:read", "integration:manage", "integration:read", "payments:refund", "fulfillment:write",
		"customers:read", "customers:points", "customers:privacy")
	e.topUp()
	e.cofEnsureSettings(0, true, true, 72)
	manual := mtManualOrders(t, e)
	h := httpapi.NewHandler(e.p.f.runtime, httpapi.Options{CVS: e.cvs, ManualOrders: manual})
	o := &lpOrderEnv{e: e, a: mtAdmin{t: t, h: h, store: e.store(), token: e.token()}, manual: manual, sku: e.p.stock.skus[0].ID,
		s: &lpStore{t: t, f: e.p.f, tenant: e.tenant(), store: e.store(), adminID: e.p.f.principalA, admin: e.token()}}
	for _, opt := range o.a.options() {
		if opt.DeliveryKind == "home" && slices.Contains(opt.PaymentModes, "bank_transfer") {
			o.homeKey = opt.OptionKey
		}
	}
	if o.homeKey == "" {
		t.Fatal("no home + bank transfer option")
	}
	t.Cleanup(func() { manual.FaultBeforeFinal, manual.FaultAfterAward = nil, nil })
	return o
}

func (o *lpOrderEnv) body(award map[string]any) map[string]any {
	b := mtBody(o.sku, 1, o.homeKey, "bank_transfer", mtHome)
	b["customer"] = map[string]any{"name": "測試會員-訂購人", "phone": "0900-000-000", "email": ""}
	if award != nil {
		b["member_award"] = award
	}
	return b
}

func lpAward(member string, points int64) map[string]any {
	return map[string]any{"member_id": member, "points": points}
}

func (o *lpOrderEnv) place(token, key string, award map[string]any) (int, map[string]any) {
	o.e.t.Helper()
	return o.a.with(token).place(key, o.body(award))
}

func lpAwardOf(t *testing.T, out map[string]any) map[string]any {
	t.Helper()
	award, ok := out["member_award"].(map[string]any)
	if !ok {
		t.Fatalf("answer has no member_award object: %v", out)
	}
	return award
}

// pipelineRows counts every row a manual-order pipeline writes before its final transaction, for this store.
func (o *lpOrderEnv) pipelineRows() string {
	o.e.t.Helper()
	var out []string
	for _, tb := range []string{"buyer.capability_sessions", "storefront.carts", "storefront.quotes", "inventory.reservations", "checkout.orders"} {
		out = append(out, fmt.Sprintf("%s=%d", tb, o.e.count(`SELECT count(*) FROM `+tb+` WHERE store_id=$1`, o.e.store())))
	}
	return strings.Join(out, " ")
}

func (o *lpOrderEnv) source(order string) string {
	o.e.t.Helper()
	var source string
	if err := o.e.p.f.owner.QueryRow(context.Background(), `SELECT source FROM checkout.orders WHERE id=$1`, order).Scan(&source); err != nil {
		o.e.t.Fatal(err)
	}
	return source
}

func (o *lpOrderEnv) receipt(key string) (bool, map[string]any) {
	o.e.t.Helper()
	var raw []byte
	err := o.e.p.f.owner.QueryRow(context.Background(), `SELECT response FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2
		AND operation='merchanttools.order.manual' AND idempotency_key=$3`, o.e.tenant(), o.e.store(), key).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		o.e.t.Fatal(err)
	}
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return true, out
}

func (o *lpOrderEnv) awardEntries(order string) int {
	return o.e.count(`SELECT count(*) FROM loyalty.point_entries WHERE store_id=$1 AND order_id=$2 AND reason='manual_order_award'`, o.e.store(), order)
}

func TestLoyaltyManualOrderGates(t *testing.T) {
	o := lpNewOrderEnv(t)
	s := o.s

	t.Run("LPG02 award, source mark and receipt commit together", func(t *testing.T) {
		m := s.mustMember("測試會員-手工單", nil, nil)
		key := t04Key("lpg02")
		code, out := o.place(s.admin, key, lpAward(m.ID, 30))
		if code != 201 {
			t.Fatalf("place with member_award: %d %v", code, out)
		}
		order, _ := out["order_id"].(string)
		award := lpAwardOf(t, out)
		if award["member_id"] != m.ID || award["points"] != float64(30) || award["state"] != "awarded" || award["entry_id"] == nil {
			t.Fatalf("member_award answer: %v", award)
		}
		if o.source(order) != "merchant_manual" || o.awardEntries(order) != 1 {
			t.Fatalf("source=%s award entries=%d", o.source(order), o.awardEntries(order))
		}
		if stored, sum := s.balance(m.ID); stored != 30 || sum != 30 {
			t.Fatalf("balance=%d sum=%d, want 30", stored, sum)
		}
		found, rec := o.receipt(key)
		if !found {
			t.Fatal("no receipt for the manual order")
		}
		if ra, _ := rec["member_award"].(map[string]any); ra["state"] != "awarded" || ra["entry_id"] != award["entry_id"] {
			t.Fatalf("receipt member_award: %v", rec["member_award"])
		}
		var details string
		if err := o.e.p.f.owner.QueryRow(context.Background(), `SELECT coalesce(details::text,'') FROM ops.audit_events WHERE store_id=$1
			AND action='order.manual_created' ORDER BY created_at DESC LIMIT 1`, o.e.store()).Scan(&details); err != nil || !strings.Contains(details, `"state": "awarded"`) {
			t.Fatalf("audit order.manual_created details lack the award state: %q (%v)", details, err)
		}
		entries, err := s.orderEntries(s.admin, order)
		if err != nil || len(entries.Entries) != 1 || entries.Entries[0].Delta != 30 || entries.Entries[0].MemberID != m.ID ||
			entries.Entries[0].Reason != "manual_order_award" || entries.Truncated {
			t.Fatalf("entries of the order: %+v %v", entries, err)
		}
		// LP19: 扣回本單積分 is a manual deduct referencing the order.
		if r, _, err := s.adjust(s.admin, t04Key("lpg02-back"), m.ID, -30, "扣回本單積分", &order); err != nil || r.Balance != 0 || r.Entry.OrderID == nil || *r.Entry.OrderID != order {
			t.Fatalf("referenced deduct: %+v %v", r, err)
		}
		if _, _, err := s.adjust(s.admin, t04Key("lpg02-back2"), m.ID, -30, "再扣一次", &order); err == nil {
			t.Fatal("a second deduct below 0 was accepted")
		} else {
			lpWant(t, "second referenced deduct", err, 409, "points_insufficient")
		}
	})

	t.Run("LPG02 a failed final transaction leaves no award; the same key then commits both", func(t *testing.T) {
		m := s.mustMember("測試會員-失敗重試", nil, nil)
		key := t04Key("lpg02-fault")
		o.manual.FaultAfterAward = func() error { return errors.New("injected failure after the award insert") }
		code, out := o.place(s.admin, key, lpAward(m.ID, 40))
		o.manual.FaultAfterAward = nil
		if code != 503 {
			t.Fatalf("faulted final transaction: %d %v, want 503", code, out)
		}
		var order string
		if err := o.e.p.f.owner.QueryRow(context.Background(), `SELECT id FROM checkout.orders WHERE store_id=$1 AND source='storefront' AND id<>$2`, o.e.store(), o.e.p.hold.OrderID).Scan(&order); err != nil {
			t.Fatalf("the pipeline should have left exactly one storefront order: %v", err)
		}
		if found, _ := o.receipt(key); found || o.awardEntries(order) != 0 || s.entries(m.ID) != 0 {
			t.Fatalf("after the rollback: receipt=%v award entries=%d", found, o.awardEntries(order))
		}
		code, out = o.place(s.admin, key, lpAward(m.ID, 40))
		if code != 201 || out["order_id"] != order || lpAwardOf(t, out)["state"] != "awarded" {
			t.Fatalf("retry with the same key: %d %v", code, out)
		}
		if o.source(order) != "merchant_manual" || o.awardEntries(order) != 1 {
			t.Fatalf("retry: source=%s award entries=%d", o.source(order), o.awardEntries(order))
		}
	})

	t.Run("LPG03 the pre-check refuses before any pipeline write", func(t *testing.T) {
		ok := s.mustMember("測試會員-預檢", nil, nil)
		erased := s.mustMember("測試會員-已清除", nil, nil)
		if _, _, err := s.erase(s.admin, t04Key("lpg03-erase"), erased.ID); err != nil {
			t.Fatal(err)
		}
		big := s.mustMember("測試會員-滿額", nil, nil)
		mustExec(t, s.f.owner, `UPDATE loyalty.members SET balance=999999999995 WHERE id=$1`, big.ID) // disclosed fixture
		other := lpNewStore(t)
		foreign := other.mustMember("測試會員-他店", nil, nil)
		weak, _ := o.e.member("inventory:reserve", "catalog:read")
		before := o.pipelineRows()
		for _, c := range []struct {
			name, token, member string
			points              int64
			status              int
			code                string
		}{
			{"unknown member", s.admin, randomUUID(), 10, 404, "not_found"},
			{"foreign-store member", s.admin, foreign.ID, 10, 404, "not_found"},
			{"erased member", s.admin, erased.ID, 10, 409, "member_erased"},
			{"no customers:points", weak, ok.ID, 10, 403, "forbidden"},
			{"balance + points > 1e12", s.admin, big.ID, 10, 422, "limit_reached"},
		} {
			code, out := o.place(c.token, t04Key("lpg03"), lpAward(c.member, c.points))
			if code != c.status || out["code"] != c.code {
				t.Errorf("%s: %d %v, want %d %s", c.name, code, out, c.status, c.code)
			}
		}
		if after := o.pipelineRows(); after != before {
			t.Errorf("a refused pre-check wrote pipeline rows:\n before %s\n after  %s", before, after)
		}
	})

	t.Run("LPG04 manual-order replay through the receipt", func(t *testing.T) {
		m := s.mustMember("測試會員-重放", nil, nil)
		key := t04Key("lpg04-order")
		code, first := o.place(s.admin, key, lpAward(m.ID, 7))
		if code != 201 {
			t.Fatalf("first: %d %v", code, first)
		}
		code, again := o.place(s.admin, key, lpAward(m.ID, 7))
		if code != 200 || lpAwardOf(t, again)["entry_id"] != lpAwardOf(t, first)["entry_id"] || lpAwardOf(t, again)["state"] != "awarded" {
			t.Fatalf("replay: %d %v", code, again)
		}
		for name, award := range map[string]map[string]any{"changed points": lpAward(m.ID, 8), "removed award": nil} {
			if code, out := o.place(s.admin, key, award); code != 409 || out["code"] != "idempotency_conflict" {
				t.Errorf("%s under the same key: %d %v, want 409 idempotency_conflict", name, code, out)
			}
		}
		if _, _, err := s.erase(s.admin, t04Key("lpg04-erase"), m.ID); err != nil {
			t.Fatal(err)
		}
		code, late := o.place(s.admin, key, lpAward(m.ID, 7))
		if code != 200 || lpAwardOf(t, late)["state"] != "awarded" || lpAwardOf(t, late)["entry_id"] != lpAwardOf(t, first)["entry_id"] {
			t.Fatalf("replay after erasure must return the stored receipt (pre-check skipped): %d %v", code, late)
		}
		if s.entries(m.ID) != 1 {
			t.Fatalf("entries=%d, want exactly one award", s.entries(m.ID))
		}
	})

	t.Run("LPG06r races between the pre-check and the final transaction", func(t *testing.T) {
		check := func(t *testing.T, key string, out map[string]any, state, reason string) string {
			t.Helper()
			order, _ := out["order_id"].(string)
			award := lpAwardOf(t, out)
			if award["state"] != state || (reason != "" && award["reason"] != reason) {
				t.Fatalf("member_award: %v, want state=%s reason=%s", award, state, reason)
			}
			if found, _ := o.receipt(key); !found || o.source(order) != "merchant_manual" {
				t.Fatalf("order must end merchant_manual with a receipt: source=%s receipt=%v", o.source(order), found)
			}
			return order
		}
		t.Run("a: erase commits first", func(t *testing.T) {
			m := s.mustMember("測試會員-競態甲", nil, nil)
			o.manual.FaultBeforeFinal = func() error {
				_, _, err := s.erase(s.admin, t04Key("lpg06r-a"), m.ID)
				return err
			}
			defer func() { o.manual.FaultBeforeFinal = nil }()
			key := t04Key("lpg06r-a-order")
			code, out := o.place(s.admin, key, lpAward(m.ID, 11))
			if code != 201 {
				t.Fatalf("place: %d %v", code, out)
			}
			order := check(t, key, out, "not_awarded", "member_erased")
			if o.awardEntries(order) != 0 || s.entries(m.ID) != 0 {
				t.Fatal("an award entry exists for a not_awarded order")
			}
		})
		t.Run("b: customers:points revoked", func(t *testing.T) {
			m := s.mustMember("測試會員-競態乙", nil, nil)
			o.manual.FaultBeforeFinal = func() error {
				_, err := s.f.owner.Exec(context.Background(), `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3
					AND permission='customers:points'`, o.e.tenant(), o.e.store(), o.e.p.f.principalA)
				return err
			}
			defer func() { o.manual.FaultBeforeFinal = nil; o.e.grantCreator("customers:points") }()
			key := t04Key("lpg06r-b-order")
			code, out := o.place(s.admin, key, lpAward(m.ID, 12))
			if code != 201 {
				t.Fatalf("place: %d %v", code, out)
			}
			order := check(t, key, out, "not_awarded", "forbidden")
			if o.awardEntries(order) != 0 {
				t.Fatal("an award entry exists after the right was revoked")
			}
		})
		t.Run("c: award commits first, erase second", func(t *testing.T) {
			m := s.mustMember("測試會員-競態丙", nil, nil)
			key := t04Key("lpg06r-c-order")
			code, out := o.place(s.admin, key, lpAward(m.ID, 13))
			if code != 201 {
				t.Fatalf("place: %d %v", code, out)
			}
			order := check(t, key, out, "awarded", "")
			var snapshot string
			if err := s.f.owner.QueryRow(context.Background(), `SELECT snapshot::text FROM checkout.orders WHERE id=$1`, order).Scan(&snapshot); err != nil {
				t.Fatal(err)
			}
			if _, _, err := s.erase(s.admin, t04Key("lpg06r-c"), m.ID); err != nil {
				t.Fatal(err)
			}
			var delta int64
			var orderRef string
			if err := s.f.owner.QueryRow(context.Background(), `SELECT delta,order_id::text FROM loyalty.point_entries WHERE member_id=$1`, m.ID).Scan(&delta, &orderRef); err != nil || delta != 13 || orderRef != order {
				t.Fatalf("award entry after erasure: delta=%d order=%s err=%v", delta, orderRef, err)
			}
			var after, name string
			if err := s.f.owner.QueryRow(context.Background(), `SELECT o.snapshot::text,m.display_name FROM checkout.orders o, loyalty.members m WHERE o.id=$1 AND m.id=$2`, order, m.ID).Scan(&after, &name); err != nil {
				t.Fatal(err)
			}
			if after != snapshot || name != "[erased]" {
				t.Fatalf("erasure must redact the member (%q) and leave the order snapshot unchanged (%v)", name, after == snapshot)
			}
		})
	})

	t.Run("LPG07 a manual order never creates or links a member", func(t *testing.T) {
		m := s.mustMember("測試會員-訂購人", lpStr("0900-000-000"), nil)
		members := o.e.count(`SELECT count(*) FROM loyalty.members WHERE store_id=$1`, o.e.store())
		if code, out := o.place(s.admin, t04Key("lpg07-order"), nil); code != 201 || out["member_award"] != nil {
			t.Fatalf("order without member_award: %d %v", code, out)
		}
		if n := o.e.count(`SELECT count(*) FROM loyalty.members WHERE store_id=$1`, o.e.store()); n != members || s.entries(m.ID) != 0 {
			t.Fatalf("an order whose 訂購人 equals a member's name/contact changed members %d -> %d or entries %d", members, n, s.entries(m.ID))
		}
	})
}

// ---------------------------------------------------------------------------------------------------------------------------
// LPG04 member commands: one effect per key, changed body = idempotency_conflict, keys are per operation, double click = one entry.
// ---------------------------------------------------------------------------------------------------------------------------

func TestLoyaltyLPG04MemberReplay(t *testing.T) {
	s := lpNewStore(t)
	in := loyalty.MemberInput{DisplayName: "測試會員-冪等", ContactNote: lpStr("LINE:lp-idem")}
	createKey := t04Key("lpg04-create")
	first, replayed, err := s.create(s.admin, createKey, in)
	if err != nil || replayed {
		t.Fatalf("create: %+v %v %v", first, replayed, err)
	}
	again, replayed, err := s.create(s.admin, createKey, in)
	if err != nil || !replayed || again.ID != first.ID {
		t.Fatalf("create replay: %+v replayed=%v %v", again, replayed, err)
	}
	if n := countRows(t, s.f.owner, `SELECT count(*) FROM loyalty.members WHERE store_id=$1`, s.store); n != 1 {
		t.Fatalf("members=%d after a replayed create", n)
	}
	_, _, err = s.create(s.admin, createKey, loyalty.MemberInput{DisplayName: "測試會員-別的"})
	lpWant(t, "create, same key, changed body", err, 409, "idempotency_conflict")

	updKey := t04Key("lpg04-update")
	patch := loyalty.MemberPatch{ExpectedVersion: first.Version, Notes: lpStr("備註一"), Fields: []string{"notes"}}
	u1, _, err := s.update(s.admin, updKey, first.ID, patch)
	if err != nil || u1.Version != first.Version+1 || u1.Notes == nil || *u1.Notes != "備註一" {
		t.Fatalf("update: %+v %v", u1, err)
	}
	u2, replayed, err := s.update(s.admin, updKey, first.ID, patch)
	if err != nil || !replayed || u2.Version != u1.Version {
		t.Fatalf("update replay: %+v %v %v", u2, replayed, err)
	}
	_, _, err = s.update(s.admin, updKey, first.ID, loyalty.MemberPatch{ExpectedVersion: first.Version, Notes: nil, Fields: []string{"notes"}})
	lpWant(t, "update, same key, explicit null instead of a value", err, 409, "idempotency_conflict")
	_, _, err = s.update(s.admin, t04Key("lpg04-stale"), first.ID, patch)
	lpWant(t, "update with a stale version", err, 409, "conflict")

	adjKey := t04Key("lpg04-adjust")
	a1, _, err := s.adjust(s.admin, adjKey, first.ID, 25, "加點", nil)
	if err != nil || a1.Balance != 25 {
		t.Fatalf("adjust: %+v %v", a1, err)
	}
	a2, replayed, err := s.adjust(s.admin, adjKey, first.ID, 25, "加點", nil)
	if err != nil || !replayed || a2.Entry.ID != a1.Entry.ID || a2.Balance != 25 || s.entries(first.ID) != 1 {
		t.Fatalf("adjust replay: %+v %v %v entries=%d", a2, replayed, err, s.entries(first.ID))
	}
	_, _, err = s.adjust(s.admin, adjKey, first.ID, 25, "加點", lpStr(randomUUID()))
	lpWant(t, "adjust, same key, order_id added", err, 409, "idempotency_conflict")
	_, _, err = s.adjust(s.admin, adjKey, first.ID, 26, "加點", nil)
	lpWant(t, "adjust, same key, other delta", err, 409, "idempotency_conflict")

	t.Run("one key string, two operations, two receipts", func(t *testing.T) {
		shared := t04Key("lpg04-shared")
		m, _, err := s.create(s.admin, shared, loyalty.MemberInput{DisplayName: "測試會員-共用鍵"})
		if err != nil {
			t.Fatal(err)
		}
		if _, replayed, err := s.adjust(s.admin, shared, m.ID, 3, "共用鍵", nil); err != nil || replayed {
			t.Fatalf("adjust under a key used by create: replayed=%v %v", replayed, err)
		}
		if n := countRows(t, s.f.owner, `SELECT count(*) FROM ops.command_results WHERE store_id=$1 AND idempotency_key=$2`, s.store, shared); n != 2 {
			t.Fatalf("receipts under the shared key: %d, want 2", n)
		}
	})

	t.Run("double click: two concurrent adjusts with one key", func(t *testing.T) {
		m := s.mustMember("測試會員-雙擊", nil, nil)
		key := t04Key("lpg04-double")
		var wg sync.WaitGroup
		results := make([]loyalty.AdjustResult, 2)
		errs := make([]error, 2)
		for i := range 2 {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				results[i], errs[i] = s.adjustRetry(s.admin, key, m.ID, 9, "雙擊")
			}(i)
		}
		wg.Wait()
		if errs[0] != nil || errs[1] != nil || results[0].Entry.ID == "" || results[0].Entry.ID != results[1].Entry.ID || s.entries(m.ID) != 1 {
			t.Fatalf("double click: %+v %v entries=%d", results, errs, s.entries(m.ID))
		}
	})

	t.Run("erase and export replay", func(t *testing.T) {
		m := s.mustMember("測試會員-匯出", nil, nil)
		s.mustAdjust(m.ID, 4, "匯出前")
		exKey := t04Key("lpg04-export")
		b1, _, err := s.export(s.admin, exKey, m.ID)
		if err != nil {
			t.Fatal(err)
		}
		b2, replayed, err := s.export(s.admin, exKey, m.ID)
		if err != nil || !replayed || !bytes.Equal(b1, b2) {
			t.Fatalf("export replay: replayed=%v equal=%v %v", replayed, bytes.Equal(b1, b2), err)
		}
		if n := countRows(t, s.f.owner, `SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='member.exported'`, s.store); n != 1 {
			t.Fatalf("member.exported audit rows: %d, want 1 (a replay writes none)", n)
		}
		erKey := t04Key("lpg04-erase")
		e1, _, err := s.erase(s.admin, erKey, m.ID)
		if err != nil {
			t.Fatal(err)
		}
		e2, replayed, err := s.erase(s.admin, erKey, m.ID)
		if err != nil || !replayed || e2 != e1 {
			t.Fatalf("erase replay: %+v vs %+v replayed=%v %v", e2, e1, replayed, err)
		}
	})
}

// ---------------------------------------------------------------------------------------------------------------------------
// LPG05 rights.
// ---------------------------------------------------------------------------------------------------------------------------

func TestLoyaltyLPG05Rights(t *testing.T) {
	s := lpNewStore(t)
	m := s.mustMember("測試會員-權限", nil, nil)
	for _, role := range []string{"owner", "admin"} {
		var holds bool
		if err := s.f.owner.QueryRow(context.Background(), `SELECT 'customers:points'=ANY(identity.staff_role_permissions($1))`, role).Scan(&holds); err != nil || !holds {
			t.Errorf("%s bundle lacks customers:points (%v)", role, err)
		}
	}
	tokens := map[string]string{
		"viewer": s.rolePrincipal("viewer"), "live_operator": s.rolePrincipal("live_operator"), "fulfilment": s.rolePrincipal("fulfilment"),
		"reserve+catalog": s.principal("inventory:reserve", "catalog:read"),
	}
	for name, token := range tokens {
		t.Run(name+" cannot write", func(t *testing.T) {
			_, _, err := s.create(token, s.key("lpg05"), loyalty.MemberInput{DisplayName: "測試會員-越權"})
			lpWant(t, "create", err, 403, "forbidden")
			_, _, err = s.update(token, s.key("lpg05"), m.ID, loyalty.MemberPatch{ExpectedVersion: m.Version, Notes: lpStr("x"), Fields: []string{"notes"}})
			lpWant(t, "update", err, 403, "forbidden")
			_, _, err = s.adjust(token, s.key("lpg05"), m.ID, 1, "越權", nil)
			lpWant(t, "adjust", err, 403, "forbidden")
			_, _, err = s.erase(token, s.key("lpg05"), m.ID)
			lpWant(t, "erase", err, 403, "forbidden")
			_, _, err = s.export(token, s.key("lpg05"), m.ID)
			lpWant(t, "export", err, 403, "forbidden")
		})
	}
	for _, name := range []string{"live_operator", "fulfilment", "reserve+catalog"} {
		token := tokens[name]
		t.Run(name+" without customers:read cannot read", func(t *testing.T) {
			_, err := s.search(token, loyalty.SearchInput{Limit: 10})
			lpWant(t, "search", err, 403, "forbidden")
			_, err = s.get(token, m.ID, "")
			lpWant(t, "detail", err, 403, "forbidden")
			_, err = s.orderEntries(token, randomUUID())
			lpWant(t, "entries by order", err, 403, "forbidden")
		})
	}
	if r, err := s.search(tokens["viewer"], loyalty.SearchInput{Limit: 10}); err != nil || len(r.Members) != 1 {
		t.Errorf("viewer (customers:read) search: %+v %v", r, err)
	}
	if n := countRows(t, s.f.owner, `SELECT count(*) FROM loyalty.point_entries WHERE store_id=$1`, s.store); n != 0 {
		t.Errorf("refused writes left %d entries", n)
	}
	t.Run("foreign store and foreign member are not_found", func(t *testing.T) {
		other := lpNewStore(t)
		err := s.run(other.admin, func(context.Context, pgx.Tx, platform.Scope) error { return nil })
		lpWant(t, "another tenant's principal on this store", err, 404, "not_found")
		fm := other.mustMember("測試會員-他店", nil, nil)
		_, err = s.get(s.admin, fm.ID, "")
		lpWant(t, "detail of another store's member", err, 404, "not_found")
		_, _, err = s.adjust(s.admin, s.key("lpg05"), fm.ID, 1, "他店", nil)
		lpWant(t, "adjust of another store's member", err, 404, "not_found")
	})
}

// ---------------------------------------------------------------------------------------------------------------------------
// LPG06 erasure, receipts and audit hold no member PII, restore replay.
// ---------------------------------------------------------------------------------------------------------------------------

func TestLoyaltyLPG06Erasure(t *testing.T) {
	s := lpNewStore(t)
	name, contact, notes := "測試會員-"+lpMarker+"-name", "LINE:"+lpMarker+"-contact", lpMarker+"-notes 測試"
	createKey := t04Key("lpg06-create")
	m, _, err := s.create(s.admin, createKey, loyalty.MemberInput{DisplayName: name, ContactNote: &contact, Notes: &notes})
	if err != nil {
		t.Fatal(err)
	}
	updKey := t04Key("lpg06-update")
	if _, _, err = s.update(s.admin, updKey, m.ID, loyalty.MemberPatch{ExpectedVersion: m.Version, Notes: lpStr(lpMarker + "-notes 第二版"), Fields: []string{"notes"}}); err != nil {
		t.Fatal(err)
	}
	adjKey := t04Key("lpg06-adjust")
	if _, _, err = s.adjust(s.admin, adjKey, m.ID, 50, lpMarker+"-ledger 第一筆", nil); err != nil {
		t.Fatal(err)
	}
	s.mustAdjust(m.ID, -20, lpMarker+"-ledger 第二筆")
	numbers := func() string {
		var out string
		if err := s.f.owner.QueryRow(context.Background(), `SELECT string_agg(id::text||'|'||delta||'|'||reason||'|'||coalesce(order_id::text,'')||'|'||actor_principal_id||'|'||created_at::text,',' ORDER BY id)
			FROM loyalty.point_entries WHERE member_id=$1`, m.ID).Scan(&out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	before := numbers()
	exportBefore, _, err := s.export(s.admin, t04Key("lpg06-export"), m.ID)
	if err != nil {
		t.Fatal(err)
	}
	eraseKey := t04Key("lpg06-erase")
	er, _, err := s.erase(s.admin, eraseKey, m.ID)
	if err != nil || er.MemberID != m.ID || er.EntriesRedacted != 2 || er.ErasedAt == "" {
		t.Fatalf("erase: %+v %v", er, err)
	}
	d, err := s.get(s.admin, m.ID, "")
	if err != nil || d.Member.DisplayName != "[erased]" || d.Member.ContactNote != nil || d.Member.Notes != nil || d.Member.ErasedAt == nil || d.Member.Balance != 30 {
		t.Fatalf("erased detail: %+v %v", d.Member, err)
	}
	for _, e := range d.Entries {
		if e.Note == nil || *e.Note != "[erased]" {
			t.Errorf("ledger note not redacted: %+v", e)
		}
	}
	if after := numbers(); after != before {
		t.Errorf("erasure changed ledger numbers:\n before %s\n after  %s", before, after)
	}
	t.Run("writes after erasure are member_erased", func(t *testing.T) {
		_, _, err := s.update(s.admin, s.key("lpg06"), m.ID, loyalty.MemberPatch{ExpectedVersion: d.Member.Version, Notes: lpStr("x"), Fields: []string{"notes"}})
		lpWant(t, "update", err, 409, "member_erased")
		_, _, err = s.adjust(s.admin, s.key("lpg06"), m.ID, 1, "清除後", nil)
		lpWant(t, "adjust", err, 409, "member_erased")
		_, _, err = s.erase(s.admin, s.key("lpg06"), m.ID)
		lpWant(t, "erase with a new key", err, 409, "member_erased")
	})
	t.Run("replays after erasure show [erased]", func(t *testing.T) {
		if again, replayed, err := s.erase(s.admin, eraseKey, m.ID); err != nil || !replayed || again != er {
			t.Errorf("erase replay: %+v %v %v", again, replayed, err)
		}
		if c, replayed, err := s.create(s.admin, createKey, loyalty.MemberInput{DisplayName: name, ContactNote: &contact, Notes: &notes}); err != nil || !replayed || c.DisplayName != "[erased]" || c.ContactNote != nil {
			t.Errorf("create replay: %+v %v %v", c, replayed, err)
		}
		if a, replayed, err := s.adjust(s.admin, adjKey, m.ID, 50, lpMarker+"-ledger 第一筆", nil); err != nil || !replayed || a.Entry.Note == nil || *a.Entry.Note != "[erased]" {
			t.Errorf("adjust replay: %+v %v %v", a, replayed, err)
		}
	})
	t.Run("search hides the erased member unless include_erased", func(t *testing.T) {
		if r, err := s.search(s.admin, loyalty.SearchInput{Limit: 100}); err != nil || len(r.Members) != 0 {
			t.Errorf("default search: %+v %v", r, err)
		}
		if r, err := s.search(s.admin, loyalty.SearchInput{IncludeErased: true, Limit: 100}); err != nil || len(r.Members) != 1 || !r.Members[0].Erased || r.Members[0].DisplayName != "[erased]" {
			t.Errorf("include_erased search: %+v %v", r, err)
		}
	})
	t.Run("no receipt and no audit row carries member PII", func(t *testing.T) {
		rows := lpTexts(t, s, `SELECT operation||' '||response::text FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2
			AND (operation LIKE 'loyalty.%' OR operation='merchanttools.order.manual')`)
		if len(rows) < 6 {
			t.Fatalf("only %d loyalty receipts scanned", len(rows))
		}
		audit := lpTexts(t, s, `SELECT action||' '||coalesce(details::text,'') FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2`)
		for _, want := range []string{"member.created", "member.updated", "member.exported", "member.erased"} {
			if !slices.ContainsFunc(audit, func(r string) bool { return strings.HasPrefix(r, want+" ") }) {
				t.Errorf("no %s audit row", want)
			}
		}
		for _, r := range append(rows, audit...) {
			if strings.Contains(r, lpMarker) || strings.Contains(r, "測試會員") {
				t.Errorf("member PII in a receipt or audit row: %s", r)
			}
		}
	})
	t.Run("export carries actor principal ids only", func(t *testing.T) {
		var doc struct {
			Member  map[string]any   `json:"member"`
			Entries []map[string]any `json:"entries"`
		}
		if err := json.Unmarshal(exportBefore, &doc); err != nil || doc.Member["display_name"] != name || len(doc.Entries) != 2 {
			t.Fatalf("export document: %s %v", exportBefore, err)
		}
		want := []string{"actor_principal_id", "created_at", "delta", "id", "note", "order_id", "reason"}
		for _, e := range doc.Entries {
			var keys []string
			for k := range e {
				keys = append(keys, k)
			}
			slices.Sort(keys)
			if !slices.Equal(keys, want) || e["actor_principal_id"] != s.adminID {
				t.Errorf("export entry keys %v (want %v) actor %v", keys, want, e["actor_principal_id"])
			}
		}
		if _, ok := doc.Member["created_by"]; ok {
			t.Error("export member carries created_by")
		}
	})
	t.Run("replay_member_erasures re-applies redaction after a restore", func(t *testing.T) {
		ctx := context.Background()
		tx, err := s.f.owner.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		// Disclosed fixture: a backup restore brings the pre-erasure row back (replica role: the restore loader bypasses triggers).
		for _, q := range []string{`SET LOCAL session_replication_role=replica`,
			`UPDATE loyalty.members SET display_name='` + name + `',contact_note='` + contact + `',notes='x',erased_at=NULL WHERE id='` + m.ID + `'`,
			`UPDATE loyalty.point_entries SET note='` + lpMarker + `-restored' WHERE member_id='` + m.ID + `'`,
			`SET LOCAL session_replication_role=origin`} {
			if _, err := tx.Exec(ctx, q); err != nil {
				t.Fatalf("%s: %v", q, err)
			}
		}
		var n int
		if err := tx.QueryRow(ctx, `SELECT loyalty.replay_member_erasures(ARRAY[$1::uuid])`, m.ID).Scan(&n); err != nil || n != 1 {
			t.Fatalf("replay: %d %v", n, err)
		}
		var dn string
		var cn, nn *string
		var erasedAt *string
		var leaked int
		if err := tx.QueryRow(ctx, `SELECT display_name,contact_note,notes,erased_at::text,(SELECT count(*) FROM loyalty.point_entries WHERE member_id=$1 AND note<>'[erased]')
			FROM loyalty.members WHERE id=$1`, m.ID).Scan(&dn, &cn, &nn, &erasedAt, &leaked); err != nil {
			t.Fatal(err)
		}
		if dn != "[erased]" || cn != nil || nn != nil || erasedAt == nil || leaked != 0 {
			t.Fatalf("after replay: name=%q contact=%v notes=%v erased_at=%v unredacted notes=%d", dn, cn, nn, erasedAt, leaked)
		}
	})
}

func lpTexts(t *testing.T, s *lpStore, q string) []string {
	t.Helper()
	rows, err := s.f.owner.Query(context.Background(), q, s.tenant, s.store)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			t.Fatal(err)
		}
		out = append(out, v)
	}
	return out
}

// ---------------------------------------------------------------------------------------------------------------------------
// LPG07 no auto-merge, no link to buyer.owners.
// ---------------------------------------------------------------------------------------------------------------------------

func TestLoyaltyLPG07NoMerge(t *testing.T) {
	s := lpNewStore(t)
	a := s.mustMember("測試會員-同名", lpStr("0911-111-111"), nil)
	b := s.mustMember("測試會員-同名", lpStr("0911-111-111"), nil)
	if a.ID == b.ID {
		t.Fatal("identical name and contact note merged into one member")
	}
	r, err := s.search(s.admin, loyalty.SearchInput{Q: lpStr("0911-111"), Limit: 10})
	if err != nil || len(r.Members) != 2 {
		t.Fatalf("search by contact note: %+v %v", r, err)
	}
	if n := countRows(t, s.f.owner, `SELECT count(*) FROM pg_constraint WHERE conrelid IN ('loyalty.members'::regclass,'loyalty.point_entries'::regclass)
		AND confrelid='buyer.owners'::regclass`); n != 0 {
		t.Errorf("%d foreign keys from loyalty.* to buyer.owners", n)
	}
	if n := countRows(t, s.f.owner, `SELECT count(*) FROM information_schema.columns WHERE table_schema='loyalty' AND column_name LIKE '%owner%'`); n != 0 {
		t.Errorf("%d owner columns in schema loyalty", n)
	}
}

// ---------------------------------------------------------------------------------------------------------------------------
// LPG08 ledger immutability and shape. Every probe runs in an owner-pool transaction that is always rolled back.
// ---------------------------------------------------------------------------------------------------------------------------

func TestLoyaltyLPG08LedgerImmutable(t *testing.T) {
	s := lpNewStore(t)
	live := s.mustMember("測試會員-帳本", nil, nil)
	liveEntry := s.mustAdjust(live.ID, 10, "帳本一").Entry.ID
	gone := s.mustMember("測試會員-帳本已清除", nil, nil)
	goneEntry := s.mustAdjust(gone.ID, 10, "帳本二").Entry.ID
	if _, _, err := s.erase(s.admin, s.key("lpg08"), gone.ID); err != nil {
		t.Fatal(err)
	}
	probe := func(role, q string, args ...any) string {
		t.Helper()
		ctx := context.Background()
		tx, err := s.f.owner.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id',$1,true),set_config('app.store_id',$2,true)`, s.tenant, s.store); err != nil {
			t.Fatal(err)
		}
		if role != "" {
			if _, err := tx.Exec(ctx, `SET LOCAL ROLE `+role); err != nil {
				t.Fatal(err)
			}
		}
		_, err = tx.Exec(ctx, q, args...)
		var pg *pgconn.PgError
		if errors.As(err, &pg) {
			return pg.Code
		}
		if err != nil {
			return err.Error()
		}
		return "ok"
	}
	for _, role := range []string{"commerce_runtime", "commerce_privacy_writer"} {
		for name, q := range map[string]string{
			"UPDATE delta":        `UPDATE loyalty.point_entries SET delta=99 WHERE id='` + liveEntry + `'`,
			"DELETE":              `DELETE FROM loyalty.point_entries WHERE id='` + liveEntry + `'`,
			"TRUNCATE":            `TRUNCATE loyalty.point_entries`,
			"UPDATE balance":      `UPDATE loyalty.members SET balance=999 WHERE id='` + live.ID + `'`,
			"UPDATE erased delta": `UPDATE loyalty.point_entries SET delta=99 WHERE id='` + goneEntry + `'`,
		} {
			if role == "commerce_privacy_writer" && name == "UPDATE balance" {
				continue // the definer owner is the balance writer (UPDATE(balance) granted); login roles are the ones refused
			}
			if got := probe(role, q); got != "42501" {
				t.Errorf("%s %s: %s, want 42501", role, name, got)
			}
		}
	}
	if got := probe("commerce_privacy_writer", `UPDATE loyalty.point_entries SET note='[erased]' WHERE id='`+liveEntry+`'`); got != "42501" {
		t.Errorf("definer owner redacting a non-erased member's note: %s, want 42501 (trigger)", got)
	}
	// As the table owner (bypasses grants, not the trigger): the trigger alone must refuse.
	for name, q := range map[string]string{
		"owner UPDATE delta of an erased member": `UPDATE loyalty.point_entries SET delta=99 WHERE id='` + goneEntry + `'`,
		"owner redact a non-erased member":       `UPDATE loyalty.point_entries SET note='[erased]' WHERE id='` + liveEntry + `'`,
		"owner DELETE":                           `DELETE FROM loyalty.point_entries WHERE id='` + liveEntry + `'`,
		"owner TRUNCATE":                         `TRUNCATE loyalty.point_entries`,
	} {
		if got := probe("", q); got != "42501" {
			t.Errorf("%s: %s, want 42501", name, got)
		}
	}
	if got := probe("", `INSERT INTO loyalty.point_entries(tenant_id,store_id,member_id,delta,reason,order_id,note,actor_principal_id)
		VALUES($1,$2,$3,-500,'manual_order_award',NULL,'[erased]',$4)`, s.tenant, s.store, live.ID, s.adminID); got != "23514" {
		t.Errorf("award row with a negative delta, no order and an [erased] note: %s, want 23514", got)
	}
	if got := probe("", `UPDATE loyalty.point_entries SET note='[erased]' WHERE id='`+goneEntry+`'`); got != "ok" {
		t.Errorf("the LP11 exception (erased member, note -> [erased]) is refused: %s", got)
	}
}

// ---------------------------------------------------------------------------------------------------------------------------
// LPG09 PII hygiene through the REAL merchant router (Go API log capture, refusal codes on the wire, no PII in URLs).
// The admin BFF half is LP-2's (the BFF rows do not exist in this unit; reported NOT_RUN in DELIVERY.md).
// ---------------------------------------------------------------------------------------------------------------------------

func TestLoyaltyLPG09HTTPRoutesLogNoPII(t *testing.T) {
	s := lpNewStore(t)
	var logs bytes.Buffer
	prevWriter, prevSlog := log.Writer(), slog.Default()
	log.SetOutput(&logs)
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer func() { log.SetOutput(prevWriter); slog.SetDefault(prevSlog) }()
	srv := httptest.NewServer(httpapi.NewHandler(s.f.runtime))
	defer srv.Close()
	base := srv.URL + "/v1/admin/stores/" + s.store
	call := func(method, path, key string, body any) (int, map[string]any) {
		t.Helper()
		var reader *bytes.Reader
		if body != nil {
			raw, _ := json.Marshal(body)
			reader = bytes.NewReader(raw)
		} else {
			reader = bytes.NewReader(nil)
		}
		req, _ := http.NewRequest(method, base+path, reader)
		req.Header.Set("Authorization", "Bearer "+s.admin)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		if strings.Contains(req.URL.String(), lpMarker) || strings.Contains(req.URL.String(), "測試") {
			t.Fatalf("PII in a URL: %s", req.URL)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var out map[string]any
		_ = json.NewDecoder(res.Body).Decode(&out)
		if cc := res.Header.Get("Cache-Control"); !strings.Contains(cc, "no-store") {
			t.Errorf("%s %s Cache-Control %q", method, path, cc)
		}
		return res.StatusCode, out
	}
	name := "測試會員-" + lpMarker
	code, created := call("POST", "/members", t04Key("lpg09"), map[string]any{"display_name": name, "contact_note": "LINE:" + lpMarker, "notes": lpMarker})
	if code != 201 {
		t.Fatalf("POST members: %d %v", code, created)
	}
	// Contract §2 returns {member}; the shared payment fixture also seeds a pre-existing storefront order above.
	createdMember, ok := created["member"].(map[string]any)
	if !ok {
		t.Fatalf("missing member envelope: %v", created)
	}
	member, _ := createdMember["id"].(string)
	if !command.ValidID(member) {
		t.Fatalf("invalid created member id: %q", member)
	}
	steps := []struct {
		method, path string
		key          bool
		body         any
		want         int
		code         string
	}{
		{"POST", "/members/search", false, map[string]any{"q": lpMarker, "include_erased": false, "cursor": nil, "limit": 10}, 200, ""},
		{"GET", "/members/" + member, false, nil, 200, ""},
		{"PATCH", "/members/" + member, true, map[string]any{"expected_version": 1, "notes": lpMarker + "-2"}, 200, ""},
		{"POST", "/members/" + member + "/adjustments", true, map[string]any{"delta": 5, "note": lpMarker + "-ledger", "order_id": nil}, 201, ""},
		{"POST", "/members/" + member + "/adjustments", true, map[string]any{"delta": -6, "note": lpMarker + "-too-much"}, 409, "points_insufficient"},
		{"POST", "/members/" + member + "/adjustments", true, map[string]any{"delta": 1, "note": "x", "balance": 1}, 400, "invalid_json"},
		{"GET", "/members/entries?order_id=" + randomUUID(), false, nil, 404, "not_found"},
		{"POST", "/members/" + member + "/export", true, map[string]any{}, 200, ""},
		{"POST", "/members/" + member + "/erasure", true, map[string]any{}, 200, ""},
		{"POST", "/members/" + member + "/adjustments", true, map[string]any{"delta": 1, "note": "清除後"}, 409, "member_erased"},
	}
	for _, st := range steps {
		key := ""
		if st.key {
			key = t04Key("lpg09")
		}
		code, out := call(st.method, st.path, key, st.body)
		if code != st.want || (st.code != "" && out["code"] != st.code) {
			t.Errorf("%s %s: %d %v, want %d %s", st.method, st.path, code, out, st.want, st.code)
		}
	}
	if strings.Contains(logs.String(), lpMarker) || strings.Contains(logs.String(), "測試會員") {
		t.Errorf("member PII in the Go API log:\n%s", logs.String())
	}
}

// ---------------------------------------------------------------------------------------------------------------------------
// LPG10 ACL pins of schema loyalty (the privacy-writer column/function pins are in customers_billing_schema_test.go, the worker pins in
// worker_authority_split_test.go WAS02).
// ---------------------------------------------------------------------------------------------------------------------------

func TestLoyaltyLPG10ACL(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	one := func(q string, args ...any) string {
		t.Helper()
		var v string
		if err := f.owner.QueryRow(ctx, q, args...).Scan(&v); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return v
	}
	if got := one(`SELECT coalesce(string_agg(pg_get_userbyid(a.grantee),',' ORDER BY pg_get_userbyid(a.grantee)),'') FROM pg_namespace n, aclexplode(n.nspacl) a
		WHERE n.nspname='loyalty' AND a.privilege_type='USAGE' AND a.grantee<>n.nspowner`); got != "commerce_privacy_writer,commerce_runtime" {
		t.Errorf("schema loyalty USAGE grantees: %s", got)
	}
	for _, tb := range []string{"loyalty.members", "loyalty.point_entries"} {
		if one(`SELECT (relrowsecurity AND relforcerowsecurity)::text FROM pg_class WHERE oid=$1::regclass`, tb) != "true" {
			t.Errorf("%s: RLS not ENABLE+FORCE", tb)
		}
		if got := one(`SELECT coalesce(string_agg(DISTINCT grantee,','),'') FROM information_schema.table_privileges WHERE table_schema||'.'||table_name=$1
			AND grantee NOT IN ('commerce_privacy_writer') AND grantee<>(SELECT pg_get_userbyid(relowner) FROM pg_class WHERE oid=$1::regclass)`, tb); got != "" {
			t.Errorf("%s has table grants to %s (definers only)", tb, got)
		}
		if got := one(`SELECT coalesce(string_agg(DISTINCT grantee,','),'') FROM information_schema.column_privileges WHERE table_schema||'.'||table_name=$1
			AND grantee NOT IN ('commerce_privacy_writer') AND grantee<>(SELECT pg_get_userbyid(relowner) FROM pg_class WHERE oid=$1::regclass)`, tb); got != "" {
			t.Errorf("%s has column grants to %s (definers only)", tb, got)
		}
	}
	runtimeExec := []string{"adjust_points", "create_member", "erase_member", "export_member", "precheck_award", "read_entry", "read_member",
		"read_members", "read_order_entries", "try_award_manual_order", "update_member"}
	rows, err := f.owner.Query(ctx, `SELECT p.proname,pg_get_userbyid(p.proowner),p.prosecdef,coalesce(p.proconfig::text,''),p.prorettype='trigger'::regtype,
		coalesce((SELECT string_agg(coalesce(nullif(pg_get_userbyid(a.grantee),''),'PUBLIC'),',') FROM aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a
		 WHERE a.privilege_type='EXECUTE' AND a.grantee<>p.proowner),'')
		FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='loyalty' ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for rows.Next() {
		var name, owner, config, exec string
		var definer, trigger bool
		if err := rows.Scan(&name, &owner, &definer, &config, &trigger, &exec); err != nil {
			t.Fatal(err)
		}
		seen++
		want := ""
		if slices.Contains(runtimeExec, name) {
			want = "commerce_runtime"
		}
		if owner != "commerce_privacy_writer" || !strings.Contains(config, "search_path=pg_catalog") || definer == trigger || exec != want {
			t.Errorf("loyalty.%s: owner=%s definer=%v trigger=%v config=%s EXECUTE=%q (want %q)", name, owner, definer, trigger, config, exec, want)
		}
	}
	rows.Close()
	if seen < len(runtimeExec)+2 {
		t.Errorf("only %d functions in schema loyalty", seen)
	}
	def := one(`SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid='identity.store_grants'::regclass AND conname='store_grants_permission_check'`)
	if strings.Count(def, "'customers:points'") != 1 {
		t.Errorf("permission CHECK holds customers:points %d times", strings.Count(def, "'customers:points'"))
	}
	if n := one(`SELECT count(*)::text FROM pg_trigger WHERE tgrelid='loyalty.point_entries'::regclass AND NOT tgisinternal
		AND tgname IN ('point_entries_immutable','point_entries_no_truncate') AND tgenabled='O'`); n != "2" {
		t.Errorf("ledger triggers enabled: %s, want 2", n)
	}
}
