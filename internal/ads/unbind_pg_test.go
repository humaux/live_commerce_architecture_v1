package ads

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/platform"
)

// unbind_pg_test.go is the W6-06B implementer's REAL_PG flow check (not the independent MA gates): ad-account unbind
// (Amendment W6-06B §A, migration 0160) and the catalog feed URL entry (§B). It runs on its OWN store (never the
// shared fixture store, whose binding state other tests of this package depend on) and skips unless
// LC_TEST_DATABASE_ALLOWED=1 (scripts/dev/test-focused.sh sets it). Evidence tier: REAL_PG, no Meta (fake ConnectFunc).

// unbindFx carries the second store's identities; the pools and migrations come from sharedAdsFx.
type unbindFx struct {
	f                                  *adsFx
	store                              string
	adBinding                          string // the enabled meta_ads binding of asset 9002 (store2)
	token2, token3, token4             string
	hash2, hash3, hash4                []byte
	principal2, principal3, principal4 string
	svc                                *Service
}

// setupUnbindFx provisions store2 + three principals on the shared fixture database:
// principal2 = full ads merchant (bind needs integration:manage too), principal3 = ads:read only,
// principal4 = ads:manage WITHOUT integration:manage (unbind must not need integration:manage).
func setupUnbindFx(t *testing.T) *unbindFx {
	t.Helper()
	f := sharedAdsFx(t)
	u := &unbindFx{f: f, store: newUUID(f)}
	f.must(`INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'ads-unbind-store','TWD')`, f.tenant, u.store)
	session := func(perms ...string) (string, []byte) {
		principal := newUUID(f)
		f.must(`INSERT INTO identity.principals(id) VALUES($1)`, principal)
		f.must(`INSERT INTO identity.memberships(tenant_id,principal_id) VALUES($1,$2)`, f.tenant, principal)
		for _, p := range perms {
			f.must(`INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,$4)`, f.tenant, u.store, principal, p)
		}
		token := randHex(32)
		sum := sha256.Sum256([]byte(token))
		f.must(`INSERT INTO identity.sessions(id,token_hash,principal_id,audience,expires_at) VALUES($1,$2,$3,'merchant',now()+interval '1 hour')`,
			newUUID(f), sum[:], principal)
		return token, sum[:]
	}
	u.token2, u.hash2 = session("store:read", "integration:manage", "ads:read", "ads:manage", "ads:approve")
	u.token3, u.hash3 = session("store:read", "ads:read")
	u.token4, u.hash4 = session("store:read", "ads:manage")
	// A distinct asset id (9002): the shared store1 fixture binds 9001 and other tests keep using it.
	connect := func(_ context.Context, code string, seal SealInfo) (ConnectResult, error) {
		if code != "fake-code-u6" || seal.StoreID != u.store || seal.TenantID != f.tenant {
			return ConnectResult{}, errors.New("unexpected")
		}
		return ConnectResult{ClientBusinessID: "555000222", Scopes: []string{"ads_management", "ads_read", "pages_show_list"},
			Picks: []Pick{{Kind: "ad_account", ID: "9002", Name: "Acct U6", Currency: "TWD", Timezone: "Asia/Taipei", AccountStatus: 1}},
			Token: SealedToken{KeyID: "k1", Enc: make([]byte, 32), Ciphertext: make([]byte, 48)}}, nil
	}
	svc, err := NewService(f.client, connect, DialogConfig{AppID: "4291253377792879", ConfigID: "123456789",
		RedirectURI: "https://admin.example.test/api/admin/ads/meta/callback", GraphVersion: "v26.0", StateKey: fillBytes(9)})
	if err != nil {
		t.Fatal(err)
	}
	u.svc = svc
	// connect -> callback -> bind through the EXISTING frozen flow (Amendment §A: re-binding later works through it).
	var start ConnectStart
	if err = u.scoped(u.token2, "ads:manage", func(tx pgx.Tx, s platform.Scope) (e error) {
		start, e = svc.Connect(f.ctx, tx, s, u.token2, "u6-connect-key-001")
		return e
	}); err != nil {
		t.Fatalf("connect: %v", err)
	}
	dialog, err := url.Parse(start.DialogURL)
	if err != nil {
		t.Fatal(err)
	}
	stateID, err := svc.Callback(f.ctx, f.runtime, u.token2, u.store, "fake-code-u6", dialog.Query().Get("state"))
	if err != nil {
		t.Fatalf("callback: %v", err)
	}
	var bound BindResult
	if err = u.scoped(u.token2, "ads:manage", func(tx pgx.Tx, s platform.Scope) (e error) {
		bound, e = svc.Bind(f.ctx, tx, s, u.token2, "u6-bind-key-0001", BindInput{StateID: stateID, AdAccountID: "9002"})
		return e
	}); err != nil || bound.AdBindingID == "" {
		t.Fatalf("bind: %v %+v", err, bound)
	}
	u.adBinding = bound.AdBindingID
	return u
}

// fillBytes is a distinct 32-byte dialog StateKey per service (the shared fixture uses 7s).
func fillBytes(fill byte) []byte {
	b := make([]byte, 32)
	for i := range b {
		b[i] = fill
	}
	return b
}

// scoped runs fn in a runtime-login transaction (platform.WithScope) for an arbitrary token/store of this fixture.
func (u *unbindFx) scoped(token, perm string, fn func(pgx.Tx, platform.Scope) error) error {
	return platform.WithScope(u.f.ctx, u.f.runtime, token, u.store, perm, fn)
}

// definer calls integration.meta_ads_unbind inside a scoped runtime transaction and returns its JSON result.
func (u *unbindFx) definer(token string, hash []byte, adAccount string) (string, error) {
	var raw string
	err := u.scoped(token, "ads:manage", func(tx pgx.Tx, _ platform.Scope) error {
		e := tx.QueryRow(u.f.ctx, `SELECT integration.meta_ads_unbind($1,$2,$3)::text`, hash, u.store, adAccount).Scan(&raw)
		return e
	})
	return raw, err
}

// credentialCount is the sealed-token material left for one binding (owner pool: RLS bypass).
func (u *unbindFx) credentialCount(binding string) (creds, heads string) {
	creds = u.f.str(`SELECT count(*)::text FROM integration.meta_page_credentials WHERE binding_id=$1`, binding)
	heads = u.f.str(`SELECT count(*)::text FROM integration.meta_page_heads WHERE binding_id=$1`, binding)
	return creds, heads
}

// insertOp directly inserts one operation row on a binding of store2 (owner pool; test fixture only, mimics the
// ledger states: DISPATCHING carries a live dispatch lease, READY carries none — 0008 CHECKs).
func (u *unbindFx) insertOp(t *testing.T, binding, action, state, key string) string {
	t.Helper()
	op := newUUID(u.f)
	leaseMode := ""
	var leaseUntil *time.Time
	var leaseHash []byte
	gen := 0
	if state == "DISPATCHING" {
		leaseMode, leaseUntil, leaseHash, gen = "dispatch", timePtr(time.Now().UTC().Add(time.Minute)), []byte(strings.Repeat("ab", 16)), 1
	}
	u.f.must(`INSERT INTO integration.operations(id,tenant_id,store_id,principal_id,binding_id,binding_version,provider,
		external_asset_id,purpose,action,semantic_key,request_hash,request,job_id,state,generation,lease_mode,lease_until,lease_token_hash)
		VALUES($1,$2,$3,$4,$5,1,'meta_ads','9002','marketing',$6,$7,decode(repeat('01',32),'hex'),'{}',424242,$8,$9,$10,$11,$12)`,
		op, u.f.tenant, u.store, u.principal2orFixture(), binding, action, key, state, gen, leaseMode, leaseUntil, leaseHash)
	return op
}

// principal2orFixture returns a membership principal of store2's tenant (operations FK to identity.memberships).
func (u *unbindFx) principal2orFixture() string {
	if u.principal2 == "" {
		u.principal2 = u.f.str(`SELECT principal_id::text FROM identity.sessions WHERE token_hash=$1`, u.hash2)
	}
	return u.principal2
}

func timePtr(t time.Time) *time.Time { return &t }

// pgCodeAndMessage extracts SQLSTATE + message of a PgError ("" when it is not one).
func pgCodeAndMessage(err error) (string, string) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code, pgErr.Message
	}
	return "", ""
}

func TestAdsUnbindRealPG(t *testing.T) {
	u := setupUnbindFx(t)
	f := u.f

	// §B feed URL entry: store2 publishes one ACTIVE platform-style domain.
	f.must(`INSERT INTO control.storefront_publications(tenant_id,store_id,published) VALUES($1,$2,true)`, f.tenant, u.store)
	f.must(`INSERT INTO control.storefront_domains(tenant_id,store_id,origin,state,ownership_verified_at,tls_verified_at,valid_until,evidence_ref)
		VALUES($1,$2,'https://shop-w606b.example.test','ACTIVE',now()-interval '2 days',now()-interval '1 day',now()+interval '300 days','platform-subdomain')`,
		f.tenant, u.store)

	t.Run("feed URL is store-scoped and needs ads:read", func(t *testing.T) {
		var raw string
		err := u.scoped(u.token2, "ads:read", func(tx pgx.Tx, _ platform.Scope) error {
			e := tx.QueryRow(u.f.ctx, `SELECT ads.catalog_feed_url($1,$2)::text`, u.hash2, u.store).Scan(&raw)
			return e
		})
		if err != nil {
			t.Fatalf("catalog_feed_url: %v", err)
		}
		var feed struct {
			FeedURL *string `json:"feed_url"`
			Domains []struct {
				Origin  string `json:"origin"`
				FeedURL string `json:"feed_url"`
			} `json:"domains"`
			Path string `json:"path"`
		}
		if err = json.Unmarshal([]byte(raw), &feed); err != nil {
			t.Fatalf("feed json: %v %s", err, raw)
		}
		if feed.Path != "/feeds/meta.csv" || len(feed.Domains) != 1 || feed.Domains[0].Origin != "https://shop-w606b.example.test" ||
			feed.Domains[0].FeedURL != "https://shop-w606b.example.test/feeds/meta.csv" ||
			feed.FeedURL == nil || *feed.FeedURL != "https://shop-w606b.example.test/feeds/meta.csv" {
			t.Fatalf("feed entry: %s", raw)
		}
		// ads:read is enough (the feed is public; no secret exists), a principal without it is refused.
		var raw3 string
		if err = u.scoped(u.token3, "ads:read", func(tx pgx.Tx, _ platform.Scope) error {
			e := tx.QueryRow(u.f.ctx, `SELECT ads.catalog_feed_url($1,$2)::text`, u.hash3, u.store).Scan(&raw3)
			return e
		}); err != nil || !strings.Contains(raw3, "shop-w606b") {
			t.Fatalf("ads:read principal: %v %s", err, raw3)
		}
		if err = u.scoped(u.token4, "ads:manage", func(tx pgx.Tx, _ platform.Scope) error {
			e := tx.QueryRow(u.f.ctx, `SELECT ads.catalog_feed_url($1,$2)::text`, u.hash4, u.store).Scan(&raw3)
			return e
		}); err == nil {
			t.Fatal("principal without ads:read got the feed entry")
		} else if code, msg := pgCodeAndMessage(err); code != "AD403" || msg != "forbidden" {
			t.Fatalf("want AD403 forbidden, got %s %s (%v)", code, msg, err)
		}
		// Cross-store: store1's entry never lists store2's origin.
		var raw1 string
		if err = f.scoped("ads:read", func(tx pgx.Tx, _ platform.Scope) error {
			e := tx.QueryRow(u.f.ctx, `SELECT ads.catalog_feed_url($1,$2)::text`, f.hash, f.store).Scan(&raw1)
			return e
		}); err != nil {
			t.Fatalf("store1 feed: %v", err)
		}
		if strings.Contains(raw1, "shop-w606b") {
			t.Fatalf("store2 origin leaked into store1: %s", raw1)
		}
	})

	t.Run("in-flight operations refuse the unbind and list themselves", func(t *testing.T) {
		op := u.insertOp(t, u.adBinding, "meta.ads.pause", "DISPATCHING", "u6-inflight-op-01")
		raw, err := u.definer(u.token2, u.hash2, "9002")
		if err != nil {
			t.Fatalf("unbind with an in-flight op: %v", err)
		}
		var refused struct {
			Refused    string `json:"refused"`
			Operations []struct {
				OperationID string `json:"operation_id"`
				Action      string `json:"action"`
				State       string `json:"state"`
			} `json:"operations"`
			OperationsTotal int `json:"operations_total"`
		}
		if err = json.Unmarshal([]byte(raw), &refused); err != nil {
			t.Fatalf("refusal json: %v %s", err, raw)
		}
		if refused.Refused != "operations_in_flight" || refused.OperationsTotal != 1 || len(refused.Operations) != 1 ||
			refused.Operations[0].OperationID != op || refused.Operations[0].State != "DISPATCHING" {
			t.Fatalf("refusal: %s", raw)
		}
		// Nothing was destroyed and the binding is still enabled.
		if creds, heads := u.credentialCount(u.adBinding); creds != "1" || heads != "1" {
			t.Fatalf("credentials destroyed despite refusal: %s/%s", creds, heads)
		}
		if enabled := f.str(`SELECT enabled::text FROM integration.bindings WHERE id=$1`, u.adBinding); enabled != "true" {
			t.Fatal("binding disabled despite refusal")
		}
		f.must(`DELETE FROM integration.operations WHERE id=$1`, op) // fixture cleanup: the op never dispatched
	})

	// A counting draft (AD6: an activate op exists that is not BLOCKED_POLICY) keeps the frozen 0074 trigger
	// bindings_ads_disable_guard armed: R2-ADS-PAUSE-1 "pause first, then disconnect".
	fbBinding := newUUID(f)
	f.must(`INSERT INTO integration.bindings(id,tenant_id,store_id,principal_id,provider,external_asset_id) VALUES($1,$2,$3,$4,'facebook','111')`,
		fbBinding, f.tenant, u.store, u.principal2orFixture())
	now := time.Now().UTC()
	draftIn := DraftInput{AdBindingID: u.adBinding, IdentityBindingID: fbBinding, Template: "BOOST_POST", SourceRef: "111_222",
		Currency: "TWD", LifetimeBudgetMinor: 300000, StartsAt: now.Add(11 * time.Minute), EndsAt: now.Add(49 * time.Hour),
		Countries: []string{"TW"}, AgeMin: 18, AgeMax: 65}
	var draftRaw json.RawMessage
	if err := u.scoped(u.token2, "ads:manage", func(tx pgx.Tx, s platform.Scope) (e error) {
		draftRaw, e = u.svc.CreateDraft(f.ctx, tx, s, u.token2, "u6-draft-key-0001", draftIn)
		return e
	}); err != nil {
		t.Fatalf("create draft: %v", err)
	}
	var created struct{ ID string }
	if err := json.Unmarshal(draftRaw, &created); err != nil || created.ID == "" {
		t.Fatalf("draft: %v %s", err, draftRaw)
	}
	activateOp := u.insertOp(t, u.adBinding, "meta.ads.activate", "READY", "u6-activate-op-001")
	f.must(`INSERT INTO ads.remote_objects(tenant_id,store_id,draft_id,publish_attempt,kind,seq,operation_id)
		VALUES($1,$2,$3,1,'activate',1,$4)`, f.tenant, u.store, created.ID, activateOp)

	t.Run("a counting draft refuses the unbind through the frozen trigger, atomically", func(t *testing.T) {
		var raw string
		err := u.scoped(u.token2, "ads:manage", func(tx pgx.Tx, _ platform.Scope) error {
			if e := tx.QueryRow(u.f.ctx, `SELECT integration.meta_ads_unbind($1,$2,$3)::text`, u.hash2, u.store, "9002").Scan(&raw); e != nil {
				return e
			}
			var step struct {
				Unbound bool `json:"unbound"`
			}
			if e := json.Unmarshal([]byte(raw), &step); e != nil || !step.Unbound {
				return errors.New("definer did not unbind: " + raw)
			}
			// The Go caller's CAS disable (the exact statement Service.Unbind runs) must hit bindings_ads_disable_guard.
			var bumped int64
			return tx.QueryRow(context.Background(), `UPDATE integration.bindings
				SET enabled=false,semantic_version=semantic_version+1,updated_at=clock_timestamp()
				WHERE tenant_id=$1 AND store_id=$2 AND id=$3 AND enabled RETURNING semantic_version`,
				f.tenant, u.store, u.adBinding).Scan(&bumped)
		})
		if code, msg := pgCodeAndMessage(err); code != "PT409" || msg != "binding_in_use" {
			t.Fatalf("want PT409 binding_in_use, got %s %s (%v)", code, msg, err)
		}
		// The refusal rolled the WHOLE transaction back: the sealed token survived, the binding is still enabled.
		if creds, heads := u.credentialCount(u.adBinding); creds != "1" || heads != "1" {
			t.Fatalf("credentials destroyed despite binding_in_use: %s/%s", creds, heads)
		}
		if enabled := f.str(`SELECT enabled::text FROM integration.bindings WHERE id=$1`, u.adBinding); enabled != "true" {
			t.Fatal("binding disabled despite binding_in_use")
		}
		// Fixture cleanup: park the activate op (BLOCKED_POLICY makes the draft non-counting again, AD6). The
		// generation bump is what the real claim path does and what operations_check3 requires off READY.
		f.must(`UPDATE integration.operations SET state='BLOCKED_POLICY', generation=generation+1 WHERE id=$1`, activateOp)
	})

	t.Run("unbind detaches, destroys the token and keeps history", func(t *testing.T) {
		var raw string
		err := u.scoped(u.token2, "ads:manage", func(tx pgx.Tx, _ platform.Scope) error {
			if e := tx.QueryRow(u.f.ctx, `SELECT integration.meta_ads_unbind($1,$2,$3)::text`, u.hash2, u.store, "9002").Scan(&raw); e != nil {
				return e
			}
			var bumped int64
			return tx.QueryRow(context.Background(), `UPDATE integration.bindings
				SET enabled=false,semantic_version=semantic_version+1,updated_at=clock_timestamp()
				WHERE tenant_id=$1 AND store_id=$2 AND id=$3 AND semantic_version=1 AND enabled RETURNING semantic_version`,
				f.tenant, u.store, u.adBinding).Scan(&bumped)
		})
		if err != nil {
			t.Fatalf("unbind: %v (%s)", err, raw)
		}
		var out struct {
			Unbound  bool `json:"unbound"`
			Bindings []struct {
				BindingID            string `json:"binding_id"`
				BindingVersion       int64  `json:"binding_version"`
				CredentialsDestroyed int    `json:"credentials_destroyed"`
			} `json:"bindings"`
		}
		if err = json.Unmarshal([]byte(raw), &out); err != nil || !out.Unbound || len(out.Bindings) != 1 ||
			out.Bindings[0].BindingID != u.adBinding || out.Bindings[0].BindingVersion != 1 || out.Bindings[0].CredentialsDestroyed != 1 {
			t.Fatalf("unbind result: %v %s", err, raw)
		}
		if enabled, version := f.str(`SELECT enabled::text FROM integration.bindings WHERE id=$1`, u.adBinding),
			f.str(`SELECT semantic_version::text FROM integration.bindings WHERE id=$1`, u.adBinding); enabled != "false" || version != "2" {
			t.Fatalf("binding after unbind: enabled=%s version=%s", enabled, version)
		}
		if creds, heads := u.credentialCount(u.adBinding); creds != "0" || heads != "0" {
			t.Fatalf("token material survived: %s/%s", creds, heads)
		}
		// History kept: connection row, draft, oauth state, operation ledger.
		if n := f.str(`SELECT count(*)::text FROM ads.connections WHERE store_id=$1 AND binding_id=$2`, u.store, u.adBinding); n != "1" {
			t.Fatalf("connection row deleted: %s", n)
		}
		if n := f.str(`SELECT count(*)::text FROM ads.campaign_drafts WHERE store_id=$1 AND id=$2`, u.store, created.ID); n != "1" {
			t.Fatalf("draft deleted: %s", n)
		}
		if n := f.str(`SELECT count(*)::text FROM ads.oauth_states WHERE store_id=$1`, u.store); n == "0" {
			t.Fatal("oauth state history deleted")
		}
		if n := f.str(`SELECT count(*)::text FROM integration.operations WHERE store_id=$1`, u.store); n == "0" {
			t.Fatal("operation ledger deleted")
		}
		// No new plans on the detached binding.
		err = u.scoped(u.token2, "ads:manage", func(tx pgx.Tx, s platform.Scope) error {
			_, e := u.svc.CreateDraft(f.ctx, tx, s, u.token2, "u6-draft-key-0002", draftIn)
			return e
		})
		if !isRefusal(err, "binding_disabled") {
			t.Fatalf("create on a detached binding: %v", err)
		}
	})

	t.Run("re-unbind is a 200 no-op", func(t *testing.T) {
		raw, err := u.definer(u.token2, u.hash2, "9002")
		if err != nil {
			t.Fatalf("no-op unbind: %v", err)
		}
		var out struct {
			Unbound        bool `json:"unbound"`
			AlreadyUnbound bool `json:"already_unbound"`
		}
		if err = json.Unmarshal([]byte(raw), &out); err != nil || out.Unbound || !out.AlreadyUnbound {
			t.Fatalf("no-op result: %v %s", err, raw)
		}
	})

	t.Run("cross-store unbind is a no-op that touches nothing", func(t *testing.T) {
		var raw string
		// store1's scope, store2's asset: the definer is store-scoped, so this finds nothing (never store2's binding).
		if err := f.scoped("ads:manage", func(tx pgx.Tx, _ platform.Scope) error {
			e := tx.QueryRow(u.f.ctx, `SELECT integration.meta_ads_unbind($1,$2,$3)::text`, f.hash, f.store, "9002").Scan(&raw)
			return e
		}); err != nil {
			t.Fatalf("cross-store call: %v", err)
		}
		var out struct {
			Unbound        bool `json:"unbound"`
			AlreadyUnbound bool `json:"already_unbound"`
		}
		if err := json.Unmarshal([]byte(raw), &out); err != nil || out.Unbound || !out.AlreadyUnbound {
			t.Fatalf("cross-store result: %v %s", err, raw)
		}
		if f.adBinding != "" { // the shared connect/bind test ran in this selection
			if enabled := f.str(`SELECT enabled::text FROM integration.bindings WHERE id=$1`, f.adBinding); enabled != "true" {
				t.Fatal("store1's own binding was disabled by a cross-store call")
			}
		}
	})

	t.Run("permissions", func(t *testing.T) {
		// The WithScope layer: a principal without ads:manage never reaches the definer.
		err := u.scoped(u.token3, "ads:manage", func(pgx.Tx, platform.Scope) error { return nil })
		if !errors.Is(err, platform.ErrForbidden) {
			t.Fatalf("scope without ads:manage: %v", err)
		}
		// The definer re-checks: opened with ads:read only, the ads:manage resolve_access refuses AD403.
		var raw string
		err = u.scoped(u.token3, "ads:read", func(tx pgx.Tx, _ platform.Scope) error {
			e := tx.QueryRow(u.f.ctx, `SELECT integration.meta_ads_unbind($1,$2,$3)::text`, u.hash3, u.store, "9002").Scan(&raw)
			return e
		})
		if code, msg := pgCodeAndMessage(err); code != "AD403" || msg != "forbidden" {
			t.Fatalf("definer without ads:manage: want AD403 forbidden, got %s %s (%v)", code, msg, err)
		}
		// ads:manage WITHOUT integration:manage is sufficient (the brief's permission rule; the binding is already
		// unbound here, so the proof is that the call authenticates and answers the no-op instead of refusing).
		if raw, err = u.definer(u.token4, u.hash4, "9002"); err != nil {
			t.Fatalf("ads:manage-only principal: %v %s", err, raw)
		}
		var noop struct {
			Unbound        bool `json:"unbound"`
			AlreadyUnbound bool `json:"already_unbound"`
		}
		if err = json.Unmarshal([]byte(raw), &noop); err != nil || noop.Unbound || !noop.AlreadyUnbound {
			t.Fatalf("ads:manage-only principal result: %v %s", err, raw)
		}
		// Input shape: a non-numeric ad account is refused AD422.
		if _, err = u.definer(u.token2, u.hash2, "act_9002"); err == nil {
			t.Fatal("non-numeric ad account accepted")
		} else if code, msg := pgCodeAndMessage(err); code != "AD422" || msg != "invalid_request" {
			t.Fatalf("want AD422 invalid_request, got %s %s", code, msg)
		}
	})
}

// TestAdsUnbindServiceRealPG is the Go half of Amendment W6-06B: Service.Unbind (409 operations_in_flight with the
// transport details, receipt + replay, idempotent no-op, the ads.account_unbound audit row, re-bind through the frozen
// connect flow) and Service.CatalogFeed. It provisions its OWN store via setupUnbindFx (a fresh 9002 binding), so it
// never depends on the state TestAdsUnbindRealPG left behind.
func TestAdsUnbindServiceRealPG(t *testing.T) {
	u := setupUnbindFx(t)
	f := u.f
	f.must(`INSERT INTO control.storefront_publications(tenant_id,store_id,published) VALUES($1,$2,true)`, f.tenant, u.store)
	f.must(`INSERT INTO control.storefront_domains(tenant_id,store_id,origin,state,ownership_verified_at,tls_verified_at,valid_until,evidence_ref)
		VALUES($1,$2,'https://shop-w606b-svc.example.test','ACTIVE',now()-interval '2 days',now()-interval '1 day',now()+interval '300 days','platform-subdomain')`,
		f.tenant, u.store)

	t.Run("in-flight operation refuses with a detailed 409 and writes nothing", func(t *testing.T) {
		op := u.insertOp(t, u.adBinding, "meta.ads.pause", "DISPATCHING", "u6-svc-inflight-01")
		var got UnbindResult
		err := u.scoped(u.token2, "ads:manage", func(tx pgx.Tx, s platform.Scope) (e error) {
			got, e = u.svc.Unbind(f.ctx, tx, s, u.token2, "u6-svc-unbind-key-01", UnbindInput{AdAccountID: "9002"})
			return e
		})
		var refused *Refusal
		if !errors.As(err, &refused) || refused.Code != "operations_in_flight" || refused.Status != 409 {
			t.Fatalf("want 409 operations_in_flight, got %v (%+v)", err, got)
		}
		d := refused.ErrorDetails()
		ops, _ := d["operations"].([]map[string]any)
		if d["operations_total"] != 1 || len(ops) != 1 || ops[0]["operation_id"] != op || ops[0]["state"] != "DISPATCHING" {
			t.Fatalf("details: %v", d)
		}
		if creds, heads := u.credentialCount(u.adBinding); creds != "1" || heads != "1" {
			t.Fatalf("credentials destroyed despite refusal: %s/%s", creds, heads)
		}
		if enabled := f.str(`SELECT enabled::text FROM integration.bindings WHERE id=$1`, u.adBinding); enabled != "true" {
			t.Fatal("binding disabled despite refusal")
		}
		// The refused transaction left NO receipt: the same key performs the real unbind below.
		if n := f.str(`SELECT count(*)::text FROM ops.command_results WHERE store_id=$1 AND operation='ads.meta.unbind'`, u.store); n != "0" {
			t.Fatalf("refused unbind left a receipt: %s", n)
		}
		f.must(`DELETE FROM integration.operations WHERE id=$1`, op) // fixture cleanup: the op never dispatched
	})

	t.Run("unbind writes the receipt and the audit row, replays, then no-ops", func(t *testing.T) {
		var first UnbindResult
		if err := u.scoped(u.token2, "ads:manage", func(tx pgx.Tx, s platform.Scope) (e error) {
			first, e = u.svc.Unbind(f.ctx, tx, s, u.token2, "u6-svc-unbind-key-01", UnbindInput{AdAccountID: "9002"})
			return e
		}); err != nil {
			t.Fatalf("unbind: %v", err)
		}
		if !first.Unbound || first.AlreadyUnbound || first.AdAccountID != "9002" ||
			len(first.BindingIDs) != 1 || first.BindingIDs[0] != u.adBinding {
			t.Fatalf("result: %+v", first)
		}
		if enabled, version := f.str(`SELECT enabled::text FROM integration.bindings WHERE id=$1`, u.adBinding),
			f.str(`SELECT semantic_version::text FROM integration.bindings WHERE id=$1`, u.adBinding); enabled != "false" || version != "2" {
			t.Fatalf("binding: enabled=%s version=%s", enabled, version)
		}
		if creds, heads := u.credentialCount(u.adBinding); creds != "0" || heads != "0" {
			t.Fatalf("token material survived: %s/%s", creds, heads)
		}
		if n := f.str(`SELECT count(*)::text FROM ops.audit_events WHERE store_id=$1 AND action='ads.account_unbound'`, u.store); n != "1" {
			t.Fatalf("audit rows: %s", n)
		}
		// Same key: the stored receipt, byte-equal result.
		var replay UnbindResult
		if err := u.scoped(u.token2, "ads:manage", func(tx pgx.Tx, s platform.Scope) (e error) {
			replay, e = u.svc.Unbind(f.ctx, tx, s, u.token2, "u6-svc-unbind-key-01", UnbindInput{AdAccountID: "9002"})
			return e
		}); err != nil || !reflect.DeepEqual(first, replay) {
			t.Fatalf("replay: %v %+v vs %+v", err, replay, first)
		}
		// New key: the idempotent no-op, without a second audit row.
		var second UnbindResult
		if err := u.scoped(u.token2, "ads:manage", func(tx pgx.Tx, s platform.Scope) (e error) {
			second, e = u.svc.Unbind(f.ctx, tx, s, u.token2, "u6-svc-unbind-key-02", UnbindInput{AdAccountID: "9002"})
			return e
		}); err != nil || second.Unbound || !second.AlreadyUnbound || len(second.BindingIDs) != 0 {
			t.Fatalf("no-op: %v %+v", err, second)
		}
		if n := f.str(`SELECT count(*)::text FROM ops.audit_events WHERE store_id=$1 AND action='ads.account_unbound'`, u.store); n != "1" {
			t.Fatalf("no-op audited again: %s", n)
		}
		// Local validation before any SQL: a non-numeric asset is refused invalid_request.
		err := u.scoped(u.token2, "ads:manage", func(tx pgx.Tx, s platform.Scope) (e error) {
			_, e = u.svc.Unbind(f.ctx, tx, s, u.token2, "u6-svc-unbind-key-03", UnbindInput{AdAccountID: "act_9002"})
			return e
		})
		if !isRefusal(err, "invalid_request") {
			t.Fatalf("non-numeric asset: %v", err)
		}
	})

	t.Run("re-bind works through the existing connect flow", func(t *testing.T) {
		var start ConnectStart
		if err := u.scoped(u.token2, "ads:manage", func(tx pgx.Tx, s platform.Scope) (e error) {
			start, e = u.svc.Connect(f.ctx, tx, s, u.token2, "u6-svc-connect-key-1")
			return e
		}); err != nil {
			t.Fatalf("connect: %v", err)
		}
		dialog, err := url.Parse(start.DialogURL)
		if err != nil {
			t.Fatal(err)
		}
		stateID, err := u.svc.Callback(f.ctx, f.runtime, u.token2, u.store, "fake-code-u6", dialog.Query().Get("state"))
		if err != nil {
			t.Fatalf("callback: %v", err)
		}
		var bound BindResult
		if err = u.scoped(u.token2, "ads:manage", func(tx pgx.Tx, s platform.Scope) (e error) {
			bound, e = u.svc.Bind(f.ctx, tx, s, u.token2, "u6-svc-bind-key-0001", BindInput{StateID: stateID, AdAccountID: "9002"})
			return e
		}); err != nil {
			t.Fatalf("bind: %v", err)
		}
		if bound.AdBindingID == "" || bound.AdBindingID == u.adBinding {
			t.Fatalf("rebind reused the disabled binding: %q", bound.AdBindingID)
		}
		if enabled := f.str(`SELECT enabled::text FROM integration.bindings WHERE id=$1`, bound.AdBindingID); enabled != "true" {
			t.Fatal("rebound binding is not enabled")
		}
		if creds, heads := u.credentialCount(bound.AdBindingID); creds != "1" || heads != "1" {
			t.Fatalf("rebound credentials: %s/%s", creds, heads)
		}
		// Both connection rows survive: the unbind detached, it never deleted history.
		if n := f.str(`SELECT count(*)::text FROM ads.connections WHERE store_id=$1`, u.store); n != "2" {
			t.Fatalf("connection history: %s", n)
		}
	})

	t.Run("catalog feed through the service", func(t *testing.T) {
		var raw json.RawMessage
		if err := u.scoped(u.token3, "ads:read", func(tx pgx.Tx, s platform.Scope) (e error) {
			raw, e = u.svc.CatalogFeed(f.ctx, tx, s, u.token3)
			return e
		}); err != nil {
			t.Fatalf("catalog feed: %v", err)
		}
		var feed struct {
			FeedURL string `json:"feed_url"`
			Path    string `json:"path"`
			Domains []struct {
				Origin  string `json:"origin"`
				FeedURL string `json:"feed_url"`
			} `json:"domains"`
		}
		if err := json.Unmarshal(raw, &feed); err != nil || feed.Path != "/feeds/meta.csv" ||
			feed.FeedURL != "https://shop-w606b-svc.example.test/feeds/meta.csv" || len(feed.Domains) != 1 ||
			feed.Domains[0].Origin != "https://shop-w606b-svc.example.test" {
			t.Fatalf("feed: %v %s", err, raw)
		}
		// Without ads:read the WithScope layer refuses before the definer ever runs.
		err := u.scoped(u.token4, "ads:read", func(tx pgx.Tx, s platform.Scope) error {
			_, e := u.svc.CatalogFeed(f.ctx, tx, s, u.token4)
			return e
		})
		if !errors.Is(err, platform.ErrForbidden) {
			t.Fatalf("feed without ads:read: %v", err)
		}
	})
}
