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
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/command"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/loyalty"
	"livecommerce/internal/merchanttools"
	"livecommerce/internal/platform"
)

const lpMarker = "lp-marker"

func lpHash(token string) []byte { hash := sha256.Sum256([]byte(token)); return hash[:] }

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

type lpSessionAuth struct {
	token     string
	hash      []byte
	principal string
	issuer    string
	subject   string
	age       func(interval string)
	reauth    func()
}

// newSessionAuth provisions a dedicated synthetic session and binds a synthetic OIDC external identity.
// Freshness is aged with UPDATE authenticated_at and restored by identity.complete_oidc_reauth.
func (s *lpStore) newSessionAuth(permissions ...string) *lpSessionAuth {
	s.t.Helper()
	ctx := context.Background()
	var perms []string
	if !slices.Contains(permissions, "store:read") {
		perms = append(perms, "store:read")
	}
	perms = append(perms, permissions...)
	principal, token := lcPrincipal(s.t, s.f, s.tenant, []string{s.store}, perms...)
	hash := sha256.Sum256([]byte(token))
	var original time.Time
	if err := s.f.owner.QueryRow(ctx, `SELECT authenticated_at FROM identity.sessions WHERE token_hash=$1`, hash[:]).Scan(&original); err != nil {
		s.t.Fatalf("query authenticated_at: %v", err)
	}
	issuer := "https://loyalty-reauth.example.test/" + randomUUID()
	subject := "synthetic-loyalty-actor-" + randomUUID()
	mustExec(s.t, s.f.owner, `INSERT INTO identity.external_identities(issuer,subject,principal_id) VALUES($1,$2,$3)`, issuer, subject, principal)
	s.t.Cleanup(func() {
		mustExec(s.t, s.f.owner, `UPDATE identity.sessions SET authenticated_at=$2 WHERE token_hash=$1`, hash[:], original)
		mustExec(s.t, s.f.owner, `DELETE FROM identity.external_identities WHERE issuer=$1 AND subject=$2`, issuer, subject)
	})
	age := func(interval string) {
		s.t.Helper()
		mustExec(s.t, s.f.owner, `UPDATE identity.sessions SET authenticated_at=clock_timestamp()-$2::interval WHERE token_hash=$1`, hash[:], interval)
	}
	reauth := func() {
		s.t.Helper()
		var outcome string
		if err := s.f.owner.QueryRow(ctx, `SELECT outcome FROM identity.complete_oidc_reauth($1,$2,$3,clock_timestamp())`, hash[:], issuer, subject).Scan(&outcome); err != nil || outcome != "REAUTHENTICATED" {
			s.t.Fatalf("complete_oidc_reauth: outcome=%q error=%v", outcome, err)
		}
	}
	return &lpSessionAuth{
		token:     token,
		hash:      hash[:],
		principal: principal,
		issuer:    issuer,
		subject:   subject,
		age:       age,
		reauth:    reauth,
	}
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
	case errors.Is(err, platform.ErrReauthRequired):
		return 403, "reauth_required"
	case errors.Is(err, platform.ErrOwnerRoleDisabled):
		return 403, "owner_role_disabled"
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
	t.Run("full detail definer requires literal read permission", func(t *testing.T) {
		d := lpNewStore(t)
		member := d.mustMember("測試會員-SQL讀取", lpStr("lp-private-contact"), lpStr("lp-private-member-note"))
		d.mustAdjust(member.ID, 1, "lp-private-ledger-note")
		for _, permission := range []string{"customers:points", "customers:privacy", "customers:read"} {
			t.Run(permission, func(t *testing.T) {
				// Literal grants and the real runtime transaction/GUC scope are independent of the production bundle.
				token := d.principal(permission)
				hash := sha256.Sum256([]byte(token))
				var body []byte
				err := d.run(token, func(ctx context.Context, tx pgx.Tx, _ platform.Scope) error {
					return tx.QueryRow(ctx, `SELECT loyalty.read_member($1,$2::uuid,$3::uuid,NULL,$4)`, hash[:], d.store, member.ID, permission).Scan(&body)
				})
				if permission != "customers:read" {
					var pg *pgconn.PgError
					if !errors.As(err, &pg) || pg.Code != "PT403" {
						t.Errorf("full member/ledger definer with %s must refuse PT403; got err=%v body_bytes=%d", permission, err, len(body))
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				var detail loyalty.Detail
				if err := json.Unmarshal(body, &detail); err != nil || detail.Member.ID != member.ID || len(detail.Entries) != 1 {
					t.Fatalf("read-only principal must receive member and ledger: %+v %v", detail, err)
				}
			})
		}
	})
	t.Run("write projection is bound to one scoped command receipt", func(t *testing.T) {
		d := lpNewStore(t)
		points := d.principal("customers:points")
		in := loyalty.MemberInput{DisplayName: "測試會員-投影", ContactNote: lpStr("lp-projection-contact"), Notes: lpStr("lp-projection-notes")}
		key := d.key("projection-create")
		m, replay, err := d.create(points, key, in)
		if err != nil || replay || m.DisplayName != in.DisplayName {
			t.Fatalf("points-only create: %+v %v %v", m, replay, err)
		}
		// Independent canonical create request: field order matches the public typed command input, including explicit nulls.
		body := []byte(`{"display_name":"測試會員-投影","contact_note":"lp-projection-contact","notes":"lp-projection-notes"}`)
		digest := sha256.Sum256(body)
		project := func(token, store, op, k string, hash []byte) ([]byte, error) {
			var out []byte
			err := d.run(token, func(ctx context.Context, tx pgx.Tx, _ platform.Scope) error {
				return tx.QueryRow(ctx, `SELECT loyalty.project_write($1,$2::uuid,$3,$4,$5)`, lpHash(token), store, op, k, hash).Scan(&out)
			})
			return out, err
		}
		for _, token := range []string{points, d.principal("customers:points")} {
			out, err := project(token, d.store, "loyalty.member.create", key, digest[:])
			var got map[string]json.RawMessage
			if err != nil || json.Unmarshal(out, &got) != nil || len(got) != 1 || got["member"] == nil || bytes.Contains(out, []byte("entries")) || bytes.Contains(out, []byte("cursor")) || bytes.Contains(out, []byte("actor")) {
				t.Fatalf("member-only projection without creator-only rule: %s %v", out, err)
			}
		}
		other := lpNewStore(t)
		wrong := sha256.Sum256([]byte("wrong normalized request"))
		for _, tc := range []struct {
			name, store, op, key string
			hash                 []byte
		}{
			{"missing receipt", d.store, "loyalty.member.create", d.key("missing"), digest[:]},
			{"wrong operation", d.store, "loyalty.member.update", key, digest[:]},
			{"operation outside allowlist", d.store, "loyalty.member.export", key, digest[:]},
			{"wrong hash", d.store, "loyalty.member.create", key, wrong[:]},
			{"null hash", d.store, "loyalty.member.create", key, nil},
			{"foreign store", other.store, "loyalty.member.create", key, digest[:]},
		} {
			t.Run(tc.name, func(t *testing.T) {
				out, err := project(points, tc.store, tc.op, tc.key, tc.hash)
				if err == nil || len(out) != 0 {
					t.Fatalf("projection must refuse without data: %s %v", out, err)
				}
			})
		}
		if out, err := project(d.principal("customers:read"), d.store, "loyalty.member.create", key, digest[:]); err == nil || len(out) != 0 {
			t.Fatalf("read permission cannot substitute for the write projection permission: %s %v", out, err)
		}
		for table, want := range map[string]int{"loyalty.members": 1, "loyalty.point_entries": 0, "ops.command_results": 1} {
			if got := countRows(t, d.f.owner, "SELECT count(*) FROM "+table+" WHERE tenant_id=$1 AND store_id=$2", d.tenant, d.store); got != want {
				t.Fatalf("refused projections changed %s: got %d want %d", table, got, want)
			}
		}
		// There is no independent target argument: a sixth member id cannot redirect a valid receipt projection.
		err = d.run(points, func(ctx context.Context, tx pgx.Tx, _ platform.Scope) error {
			_, err := tx.Exec(ctx, `SELECT loyalty.project_write($1,$2::uuid,$3,$4,$5,$6::uuid)`, lpHash(points), d.store, "loyalty.member.create", key, digest[:], randomUUID())
			return err
		})
		var pg *pgconn.PgError
		if !errors.As(err, &pg) || pg.Code != "42883" {
			t.Fatalf("target substitution entrance exists: %v", err)
		}
		patch := loyalty.MemberPatch{ExpectedVersion: m.Version, Notes: lpStr("lp-updated-projection"), Fields: []string{"notes"}}
		updateKey := d.key("projection-update")
		updated, replay, err := d.update(points, updateKey, m.ID, patch)
		if err != nil || replay || updated.Notes == nil || *updated.Notes != *patch.Notes {
			t.Fatalf("points-only update: %+v %v %v", updated, replay, err)
		}
		adjustKey := d.key("projection-adjust")
		adjusted, replay, err := d.adjust(points, adjustKey, m.ID, 7, "lp-entry-note", nil)
		if err != nil || replay || adjusted.Balance != 7 {
			t.Fatalf("points-only adjust: %+v %v %v", adjusted, replay, err)
		}
		// A successful unrelated create receipt does not authorize arbitrary entry reads.
		err = d.run(points, func(ctx context.Context, tx pgx.Tx, _ platform.Scope) error {
			var out []byte
			return tx.QueryRow(ctx, `SELECT loyalty.read_entry($1,$2::uuid,$3::uuid)`, lpHash(points), d.store, adjusted.Entry.ID).Scan(&out)
		})
		if !errors.As(err, &pg) || pg.Code != "PT403" {
			t.Fatalf("bare ledger entry read must refuse points-only: %v", err)
		}
		readToken := d.principal("customers:read")
		err = d.run(readToken, func(ctx context.Context, tx pgx.Tx, _ platform.Scope) error {
			var out []byte
			return tx.QueryRow(ctx, `SELECT loyalty.read_entry($1,$2::uuid,$3::uuid)`, lpHash(readToken), d.store, adjusted.Entry.ID).Scan(&out)
		})
		if err != nil {
			t.Fatalf("literal read-only principal must read entry: %v", err)
		}
		if _, replayed, err := d.update(points, updateKey, m.ID, patch); err != nil || !replayed {
			t.Fatalf("points-only update replay: %v %v", replayed, err)
		}
		if _, _, err := d.erase(d.admin, d.key("projection-erase"), m.ID); err != nil {
			t.Fatal(err)
		}
		if got, replayed, err := d.create(points, key, in); err != nil || !replayed || got.DisplayName != "[erased]" || got.ContactNote != nil || got.Notes != nil {
			t.Fatalf("redacted create replay: %+v %v %v", got, replayed, err)
		}
		if got, replayed, err := d.update(points, updateKey, m.ID, patch); err != nil || !replayed || got.DisplayName != "[erased]" || got.Notes != nil {
			t.Fatalf("redacted update replay: %+v %v %v", got, replayed, err)
		}
		if got, replayed, err := d.adjust(points, adjustKey, m.ID, 7, "lp-entry-note", nil); err != nil || !replayed || got.Entry.Note == nil || *got.Entry.Note != "[erased]" || got.Balance != 7 {
			t.Fatalf("redacted adjust replay: %+v %v %v", got, replayed, err)
		}
		var receipts string
		if err := d.f.owner.QueryRow(context.Background(), `SELECT string_agg(response::text,' ') FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2`, d.tenant, d.store).Scan(&receipts); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(receipts, "lp-") || strings.Contains(receipts, in.DisplayName) {
			t.Fatalf("PII retained in command receipts: %s", receipts)
		}
	})

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
	t.Run("direct SQL definers require recent auth within ten minutes", func(t *testing.T) {
		d := lpNewStore(t)
		m := d.mustMember("測試會員-直接SQL", lpStr("lp-contact-direct"), lpStr("lp-notes-direct"))
		d.mustAdjust(m.ID, 10, "lp-ledger-direct")
		mErase := d.mustMember("測試會員-直接刪除", lpStr("lp-contact-erase"), lpStr("lp-notes-erase"))
		sa := d.newSessionAuth("customers:privacy")

		// 9 minutes old session is within the 10-minute window -> succeeds.
		sa.age("9 minutes")
		var baselineExport []byte
		err := d.run(sa.token, func(ctx context.Context, tx pgx.Tx, _ platform.Scope) error {
			return tx.QueryRow(ctx, `SELECT loyalty.export_member($1,$2::uuid,$3::uuid)`, sa.hash, d.store, m.ID).Scan(&baselineExport)
		})
		if err != nil || len(baselineExport) == 0 {
			t.Fatalf("nine-minute session direct export: %v", err)
		}

		// Fixed intervals that fail closed: 11 minutes (stale), 2 hours (stale), -2 minutes (future clock skew), infinity, -infinity.
		for _, interval := range []string{"11 minutes", "2 hours", "-2 minutes", "infinity", "-infinity"} {
			t.Run("refuse_"+interval, func(t *testing.T) {
				sa.age(interval)
				var out []byte
				err := d.run(sa.token, func(ctx context.Context, tx pgx.Tx, _ platform.Scope) error {
					return tx.QueryRow(ctx, `SELECT loyalty.export_member($1,$2::uuid,$3::uuid)`, sa.hash, d.store, m.ID).Scan(&out)
				})
				var pg *pgconn.PgError
				if !errors.As(err, &pg) || pg.Code != "PT403" || pg.Message != "reauth_required" {
					t.Errorf("stale (%s) direct export: got err=%v (pg=%+v), want PT403 reauth_required", interval, err, pg)
				}
				if len(out) > 0 {
					t.Errorf("refused direct export released bytes: len=%d", len(out))
				}
				var eraseOut []byte
				err = d.run(sa.token, func(ctx context.Context, tx pgx.Tx, _ platform.Scope) error {
					return tx.QueryRow(ctx, `SELECT loyalty.erase_member($1,$2::uuid,$3::uuid)`, sa.hash, d.store, mErase.ID).Scan(&eraseOut)
				})
				if !errors.As(err, &pg) || pg.Code != "PT403" || pg.Message != "reauth_required" {
					t.Errorf("stale (%s) direct erase: got err=%v (pg=%+v), want PT403 reauth_required", interval, err, pg)
				}
			})
		}

		// Unknown member still yields PT404
		sa.age("9 minutes")
		err = d.run(sa.token, func(ctx context.Context, tx pgx.Tx, _ platform.Scope) error {
			var out []byte
			return tx.QueryRow(ctx, `SELECT loyalty.export_member($1,$2::uuid,$3::uuid)`, sa.hash, d.store, randomUUID()).Scan(&out)
		})
		var pg *pgconn.PgError
		if !errors.As(err, &pg) || pg.Code != "PT404" {
			t.Errorf("unknown member direct export: got err=%v, want PT404", err)
		}

		// OIDC re-auth restores freshness for the same session and actor
		sa.age("2 hours")
		sa.reauth()
		var restoredExport []byte
		err = d.run(sa.token, func(ctx context.Context, tx pgx.Tx, _ platform.Scope) error {
			return tx.QueryRow(ctx, `SELECT loyalty.export_member($1,$2::uuid,$3::uuid)`, sa.hash, d.store, m.ID).Scan(&restoredExport)
		})
		if err != nil || !bytes.Equal(restoredExport, baselineExport) {
			t.Fatalf("re-authenticated direct export: %v, equal=%t", err, bytes.Equal(restoredExport, baselineExport))
		}
		var eraseResult []byte
		err = d.run(sa.token, func(ctx context.Context, tx pgx.Tx, _ platform.Scope) error {
			return tx.QueryRow(ctx, `SELECT loyalty.erase_member($1,$2::uuid,$3::uuid)`, sa.hash, d.store, mErase.ID).Scan(&eraseResult)
		})
		if err != nil {
			t.Fatalf("re-authenticated direct erase: %v", err)
		}
		if n := countRows(t, d.f.owner, `SELECT count(*) FROM loyalty.members WHERE id=$1 AND erased_at IS NOT NULL`, mErase.ID); n != 1 {
			t.Fatalf("mErase was not erased after reauth: %d", n)
		}
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
	t.Run("export and erasure recent auth across aging replay and new keys", func(t *testing.T) {
		s2 := lpNewStore(t)
		mExp := s2.mustMember("測試會員-匯出重放", lpStr("LINE:lp-exp-contact"), lpStr("lp-exp-notes"))
		s2.mustAdjust(mExp.ID, 30, "lp-exp-ledger")
		mErs := s2.mustMember("測試會員-刪除重放", lpStr("LINE:lp-ers-contact"), lpStr("lp-ers-notes"))
		s2.mustAdjust(mErs.ID, 20, "lp-ers-ledger")
		sa := s2.newSessionAuth("customers:privacy", "customers:points", "customers:read")

		// 1. Fresh session (9 minutes) produces allowed export with key1.
		sa.age("9 minutes")
		expKey1 := s2.key("reauth-exp")
		initialExport, replayed, err := s2.export(sa.token, expKey1, mExp.ID)
		if err != nil || replayed || len(initialExport) == 0 {
			t.Fatalf("fresh export: err=%v replayed=%t len=%d", err, replayed, len(initialExport))
		}
		if got := countRows(t, s2.f.owner, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='member.exported'`, s2.tenant, s2.store); got != 1 {
			t.Fatalf("expected 1 member.exported audit row, got %d", got)
		}

		// 2. Session ages to 11 minutes (stale).
		sa.age("11 minutes")

		// Same-key replay of export: since export builds fresh body via guarded SQL definer, stale session MUST refuse with reauth_required.
		_, _, err = s2.export(sa.token, expKey1, mExp.ID)
		lpWant(t, "same-key export after age", err, 403, "reauth_required")

		// New-key export also refuses.
		expKey2 := s2.key("reauth-exp")
		_, _, err = s2.export(sa.token, expKey2, mExp.ID)
		lpWant(t, "new-key export after age", err, 403, "reauth_required")
		if got := countRows(t, s2.f.owner, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='member.exported'`, s2.tenant, s2.store); got != 1 {
			t.Fatalf("refused export emitted audit row: %d", got)
		}
		if got := countRows(t, s2.f.owner, `SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND idempotency_key=$3`, s2.tenant, s2.store, expKey2); got != 0 {
			t.Fatalf("refused export recorded command receipt: %d", got)
		}

		// Erase on mErs with eraseKey1: must refuse with 403 reauth_required.
		eraseKey1 := s2.key("reauth-erase")
		_, _, err = s2.erase(sa.token, eraseKey1, mErs.ID)
		lpWant(t, "erase after age", err, 403, "reauth_required")

		// Assert mErs was not erased and no audit or receipt was written.
		if n := countRows(t, s2.f.owner, `SELECT count(*) FROM loyalty.members WHERE id=$1 AND erased_at IS NOT NULL`, mErs.ID); n != 0 {
			t.Fatalf("refused erase marked member erased: %d", n)
		}
		if got := countRows(t, s2.f.owner, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='member.erased'`, s2.tenant, s2.store); got != 0 {
			t.Fatalf("refused erase recorded audit: %d", got)
		}
		if got := countRows(t, s2.f.owner, `SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND idempotency_key=$3`, s2.tenant, s2.store, eraseKey1); got != 0 {
			t.Fatalf("refused erase recorded command receipt: %d", got)
		}

		// Assert session is still valid (not logged out or revoked).
		var sessionCount int
		if err := s2.f.owner.QueryRow(context.Background(), `SELECT count(*) FROM identity.sessions WHERE token_hash=$1 AND revoked_at IS NULL AND expires_at>clock_timestamp()`, sa.hash).Scan(&sessionCount); err != nil || sessionCount != 1 {
			t.Fatalf("stale session must remain valid: count=%d err=%v", sessionCount, err)
		}

		// 3. Restore freshness via real OIDC re-auth.
		sa.reauth()

		// Same-key replay of export now succeeds, replayed=true, content matches, audit count unchanged (audit once!).
		replayedExport, replayed, err := s2.export(sa.token, expKey1, mExp.ID)
		if err != nil || !replayed || !bytes.Equal(replayedExport, initialExport) {
			t.Fatalf("reauth same-key export replay: err=%v replayed=%t equal=%t", err, replayed, bytes.Equal(replayedExport, initialExport))
		}
		if got := countRows(t, s2.f.owner, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='member.exported'`, s2.tenant, s2.store); got != 1 {
			t.Fatalf("same-key export replay must not emit second audit row: %d", got)
		}

		// New-key export succeeds, replayed=false, audit count becomes 2.
		freshExport, replayed, err := s2.export(sa.token, expKey2, mExp.ID)
		if err != nil || replayed || len(freshExport) == 0 {
			t.Fatalf("reauth new-key export: err=%v replayed=%t", err, replayed)
		}
		if got := countRows(t, s2.f.owner, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='member.exported'`, s2.tenant, s2.store); got != 2 {
			t.Fatalf("new-key export must emit audit row: %d", got)
		}

		// Erase with eraseKey1 now succeeds.
		erasure, replayed, err := s2.erase(sa.token, eraseKey1, mErs.ID)
		if err != nil || replayed || erasure.MemberID != mErs.ID || erasure.EntriesRedacted != 1 {
			t.Fatalf("reauth erase: err=%v replayed=%t erasure=%+v", err, replayed, erasure)
		}
		if got := countRows(t, s2.f.owner, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='member.erased'`, s2.tenant, s2.store); got != 1 {
			t.Fatalf("erase must emit member.erased audit row: %d", got)
		}

		// 4. Age session again to 2 hours.
		sa.age("2 hours")

		// Replay of erase with eraseKey1 while stale must yield 403 reauth_required (§16 covers every erase).
		_, _, err = s2.erase(sa.token, eraseKey1, mErs.ID)
		lpWant(t, "same-key erase after age", err, 403, "reauth_required")

		// New key on already-erased member while stale also refuses with 403 reauth_required.
		eraseKey2 := s2.key("reauth-erase")
		_, _, err = s2.erase(sa.token, eraseKey2, mErs.ID)
		lpWant(t, "new-key erase on erased member while stale", err, 403, "reauth_required")

		// Confirm no additional audit/receipt effects from stale erase attempts.
		if got := countRows(t, s2.f.owner, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='member.erased'`, s2.tenant, s2.store); got != 1 {
			t.Fatalf("stale erase attempts emitted audit row: %d", got)
		}
		if got := countRows(t, s2.f.owner, `SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND idempotency_key=$3`, s2.tenant, s2.store, eraseKey2); got != 0 {
			t.Fatalf("stale erase attempt recorded receipt: %d", got)
		}

		// Real reauth restores freshness.
		sa.reauth()

		// Replay of erase with eraseKey1 returns exact stored receipt, replayed=true, audit count still 1.
		erasureReplay, replayed, err := s2.erase(sa.token, eraseKey1, mErs.ID)
		if err != nil || !replayed || erasureReplay != erasure {
			t.Fatalf("erasure replay with stored receipt after reauth: err=%v replayed=%t got=%+v want=%+v", err, replayed, erasureReplay, erasure)
		}
		if got := countRows(t, s2.f.owner, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='member.erased'`, s2.tenant, s2.store); got != 1 {
			t.Fatalf("erasure replay must not emit second audit row: %d", got)
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
	t.Run("privacy routes refuse stale auth with 403 reauth_required on the wire", func(t *testing.T) {
		sHTTP := lpNewStore(t)
		mHTTP := sHTTP.mustMember("測試會員-HTTP", lpStr("LINE:lp-http-contact"), lpStr("lp-http-notes"))
		sHTTP.mustAdjust(mHTTP.ID, 12, "lp-http-ledger")
		mHTTPErase := sHTTP.mustMember("測試會員-HTTP-刪除", lpStr("LINE:lp-http-erase"), lpStr("lp-http-erase-notes"))
		sa := sHTTP.newSessionAuth("customers:privacy", "customers:points", "customers:read")

		httpCall := func(tok, method, path, key string) (int, map[string]any, http.Header, []byte) {
			t.Helper()
			req, _ := http.NewRequest(method, srv.URL+"/v1/admin/stores/"+sHTTP.store+path, bytes.NewReader([]byte("{}")))
			req.Header.Set("Authorization", "Bearer "+tok)
			req.Header.Set("Content-Type", "application/json")
			if key != "" {
				req.Header.Set("Idempotency-Key", key)
			}
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			bodyBytes, _ := io.ReadAll(res.Body)
			var out map[string]any
			_ = json.Unmarshal(bodyBytes, &out)
			return res.StatusCode, out, res.Header, bodyBytes
		}

		// Fresh 9 minutes session succeeds.
		sa.age("9 minutes")
		expKey := sHTTP.key("http-exp")
		code, _, hdr, body := httpCall(sa.token, "POST", "/members/"+mHTTP.ID+"/export", expKey)
		if code != 200 || !strings.Contains(hdr.Get("Content-Disposition"), "attachment;") {
			t.Fatalf("fresh session export: code=%d hdr=%v len=%d", code, hdr, len(body))
		}

		// Stale sessions (11 minutes, 2 hours) refuse with exact 403 reauth_required on both export and erasure.
		for _, interval := range []string{"11 minutes", "2 hours"} {
			t.Run("refuse_"+interval, func(t *testing.T) {
				sa.age(interval)

				// Export refusal
				c, out, h, b := httpCall(sa.token, "POST", "/members/"+mHTTP.ID+"/export", sHTTP.key("http-exp-stale"))
				if c != http.StatusForbidden || out["code"] != "reauth_required" {
					t.Errorf("export refusal status=%d code=%v (want 403 reauth_required) len=%d", c, out["code"], len(b))
				}
				if disp := h.Get("Content-Disposition"); disp != "" {
					t.Errorf("refused export released attachment header: %q", disp)
				}
				if bytes.Contains(b, []byte("lp-http-notes")) || bytes.Contains(b, []byte("lp-http-contact")) {
					t.Errorf("refused export released member PII in body (len=%d)", len(b))
				}

				// Erasure refusal
				c, out, _, b = httpCall(sa.token, "POST", "/members/"+mHTTPErase.ID+"/erasure", sHTTP.key("http-erase-stale"))
				if c != http.StatusForbidden || out["code"] != "reauth_required" {
					t.Errorf("erasure refusal status=%d code=%v (want 403 reauth_required) len=%d", c, out["code"], len(b))
				}
				if n := countRows(t, sHTTP.f.owner, `SELECT count(*) FROM loyalty.members WHERE id=$1 AND erased_at IS NOT NULL`, mHTTPErase.ID); n != 0 {
					t.Fatalf("member was erased during refused call: %d", n)
				}
				if count := countRows(t, sHTTP.f.owner, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='member.erased'`, sHTTP.tenant, sHTTP.store); count != 0 {
					t.Fatalf("refused erasure emitted audit row: %d", count)
				}
			})
		}

		// Reauth restores freshness
		sa.reauth()
		eraseKey := sHTTP.key("http-erase-fresh")
		c, _, _, b := httpCall(sa.token, "POST", "/members/"+mHTTPErase.ID+"/erasure", eraseKey)
		if c != 200 {
			t.Fatalf("re-authenticated erasure: code=%d len=%d", c, len(b))
		}
		if n := countRows(t, sHTTP.f.owner, `SELECT count(*) FROM loyalty.members WHERE id=$1 AND erased_at IS NOT NULL`, mHTTPErase.ID); n != 1 {
			t.Fatalf("member was not erased after reauth: %d", n)
		}
	})
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
	runtimeExec := []string{"adjust_points", "create_member", "erase_member", "export_member", "precheck_award", "project_write", "read_entry", "read_member",
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

// Pagination exercises the public contract, including a changed filter and a member-bound ledger cursor.
func TestLoyaltyPaginationBindings(t *testing.T) {
	s := lpNewStore(t)
	for i := 0; i < 3; i++ {
		s.mustMember(fmt.Sprintf("測試會員-paged-%d", i), nil, nil)
	}
	first, err := s.search(s.admin, loyalty.SearchInput{Q: lpStr("paged"), Limit: 2})
	if err != nil || len(first.Members) != 2 || first.NextCursor == nil {
		t.Fatalf("first search page: %+v %v", first, err)
	}
	second, err := s.search(s.admin, loyalty.SearchInput{Q: lpStr("paged"), Limit: 2, Cursor: first.NextCursor})
	if err != nil || len(second.Members) != 1 || second.NextCursor != nil {
		t.Fatalf("second search page: %+v %v", second, err)
	}
	for _, m := range first.Members {
		if m.ID == second.Members[0].ID {
			t.Fatal("search repeated a member")
		}
	}
	_, err = s.search(s.admin, loyalty.SearchInput{Q: lpStr("changed"), Limit: 2, Cursor: first.NextCursor})
	lpWant(t, "changed filter", err, 422, "invalid_request")
	m := first.Members[0]
	for i := 0; i < 51; i++ {
		s.mustAdjust(m.ID, 1, "synthetic pagination")
	}
	d, err := s.get(s.admin, m.ID, "")
	if err != nil || len(d.Entries) != 50 || d.NextLedgerCursor == nil {
		t.Fatalf("first ledger page: %+v %v", d, err)
	}
	next, err := s.get(s.admin, m.ID, *d.NextLedgerCursor)
	if err != nil || len(next.Entries) != 1 || next.NextLedgerCursor != nil {
		t.Fatalf("second ledger page: %+v %v", next, err)
	}
	for _, e := range d.Entries {
		if e.ID == next.Entries[0].ID {
			t.Fatal("ledger repeated an entry")
		}
	}
	_, err = s.get(s.admin, second.Members[0].ID, *d.NextLedgerCursor)
	lpWant(t, "foreign member cursor", err, 422, "invalid_request")
}

// Disclosed fixture bulk-loads synthetic long notes solely to reach the contract's export byte bound.
func TestLoyaltyExportByteBound(t *testing.T) {
	s := lpNewStore(t)
	m := s.mustMember("測試會員-export-bound", nil, nil)
	mustExec(t, s.f.owner, `INSERT INTO loyalty.point_entries(tenant_id,store_id,member_id,delta,reason,note,actor_principal_id)
 SELECT $1,$2,$3,1,'manual_adjust',repeat('測',200),$4 FROM generate_series(1,1800)`, s.tenant, s.store, m.ID, s.adminID)
	key := s.key("export-bound")
	_, _, err := s.export(s.admin, key, m.ID)
	lpWant(t, "export larger than one MiB", err, 409, "export_too_large")
	if n := countRows(t, s.f.owner, `SELECT count(*) FROM ops.command_results WHERE store_id=$1 AND operation='loyalty.member.export' AND idempotency_key=$2`, s.store, key); n != 0 {
		t.Errorf("refused export left %d receipts", n)
	}
	if n := countRows(t, s.f.owner, `SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='member.exported'`, s.store); n != 0 {
		t.Errorf("refused export left %d audit rows", n)
	}
}
