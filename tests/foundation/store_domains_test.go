package foundation_test

// R5 unit store-domains, independent PG gate (REAL_PG; evidence MOCK: nothing here verifies DNS or a certificate).
// Written from docs/delivery/units/store-domains.md and migrations/0106_store_domains.sql, not from the implementation:
//
//	SDW01 handle format / reserved / numeric uniqueness       SDW02 platform row ACTIVE + idempotence
//	SDW03 merchant domain lifecycle (request -> ACTIVE)        SDW04 refusals + owner-only scope
//	SDW05 worker transitions + 301 primary + TLS ask           SDW06 schema/ACL inventory + EXECUTE matrix

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
	"livecommerce/internal/storefrontadmin"
	"livecommerce/internal/storefrontdomains"
)

// sdFix wraps the merchant-scoped t06GoFixture with the two service logins the unit adds work to: the verifier
// (worker transitions + renewal) and the buyer runtime (301 primary lookup). base is the platform base zone (LC_STORE_BASE_DOMAIN).
type sdFix struct {
	t     *testing.T
	f     *t06GoFixture
	b     *testFixture
	reg   *pgxpool.Pool
	op    *pgxpool.Pool
	buyer *pgxpool.Pool
	base  string
}

func sdSetup(t *testing.T) *sdFix {
	t.Helper()
	f := newT06GoFixture(t)
	b := f.base
	return &sdFix{
		t:     t,
		f:     f,
		b:     b,
		reg:   miPool(t, b, "commerce_storefront_verifier"),
		op:    miPool(t, b, "commerce_storefront_registrar"),
		buyer: miPool(t, b, "commerce_buyer_runtime"),
		base:  "example.com",
	}
}

// sdHost is a fresh hostname in a zone that is never under the platform base (example.net vs example.com), so a
// merchant request for it never trips the "reserved hostname" refusal.
func sdHost() string {
	return "shop-" + strings.ReplaceAll(randomUUID(), "-", "")[:16] + ".example.net"
}

func (s *sdFix) merchant(perm, token, store string, fn func(pgx.Tx, platform.Scope) error) error {
	return platform.WithScope(context.Background(), s.b.runtime, token, store, perm, fn)
}

// request issues Request with a fresh Idempotency-Key per call (a new merchant action = a new key, so a lifecycle
// re-request after suspend/detach rotates the token instead of replaying the first result).
func (s *sdFix) request(token, store, host string) (storefrontdomains.RequestResult, error) {
	return s.requestKey(token, store, host, "req-"+randomUUID())
}

// requestKey issues Request with an explicit Idempotency-Key (the idempotency gate replays a fixed key).
func (s *sdFix) requestKey(token, store, host, key string) (storefrontdomains.RequestResult, error) {
	var out storefrontdomains.RequestResult
	err := s.merchant("integration:manage", token, store, func(tx pgx.Tx, sc platform.Scope) error {
		var e error
		out, e = storefrontdomains.Request(context.Background(), tx, sc, token, key, host, s.base, nil)
		return e
	})
	return out, err
}

func (s *sdFix) read(token, store string) (storefrontdomains.ReadResult, error) {
	var out storefrontdomains.ReadResult
	err := s.merchant("integration:read", token, store, func(tx pgx.Tx, sc platform.Scope) error {
		var e error
		out, e = storefrontdomains.Read(context.Background(), tx, sc, token)
		return e
	})
	return out, err
}

func (s *sdFix) suspend(token, store, origin string) (storefrontdomains.MoveResult, error) {
	var out storefrontdomains.MoveResult
	err := s.merchant("integration:manage", token, store, func(tx pgx.Tx, sc platform.Scope) error {
		var e error
		out, e = storefrontdomains.Suspend(context.Background(), tx, sc, token, "sus-"+randomUUID(), origin)
		return e
	})
	return out, err
}

func (s *sdFix) detach(token, store, origin string) (storefrontdomains.MoveResult, error) {
	var out storefrontdomains.MoveResult
	err := s.merchant("integration:manage", token, store, func(tx pgx.Tx, sc platform.Scope) error {
		var e error
		out, e = storefrontdomains.Detach(context.Background(), tx, sc, token, "det-"+randomUUID(), origin)
		return e
	})
	return out, err
}

// handleOf reads the auto-assigned handle of a store (set by the 0106 BEFORE INSERT trigger).
func handleOf(t *testing.T, pool *pgxpool.Pool, store string) string {
	t.Helper()
	var h string
	if err := pool.QueryRow(context.Background(), `SELECT handle FROM control.stores WHERE id=$1`, store).Scan(&h); err != nil {
		t.Fatal(err)
	}
	return h
}

// platformOrigin calls the idempotent platform-subdomain writer and returns its {handle, origin}.
func platformOrigin(t *testing.T, pool *pgxpool.Pool, store, base string) (handle, origin string) {
	t.Helper()
	var raw []byte
	if err := pool.QueryRow(context.Background(), `SELECT control.ensure_store_platform_domain($1::uuid,$2)`, store, base).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var out struct {
		Handle string `json:"handle"`
		Origin string `json:"origin"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out.Handle, out.Origin
}

// domainState reads one domain row's state (and the pending token) by origin.
func domainState(t *testing.T, pool *pgxpool.Pool, origin string) (state string, token *string) {
	t.Helper()
	if err := pool.QueryRow(context.Background(), `SELECT state,verification_token FROM control.storefront_domains WHERE origin=$1`, origin).
		Scan(&state, &token); err != nil {
		t.Fatal(err)
	}
	return
}

func TestStoreDomainsSDW01HandleFormatReservedUniquenessSuffix(t *testing.T) {
	s := sdSetup(t)
	ctx := context.Background()
	o := s.b.owner

	// Format + reserved parity with internal/storehandles.Valid.
	for valid, handles := range map[bool][]string{
		true:  {"abc", "a-b", "a--b", "a0-b", strings.Repeat("a", 30)},
		false: {"", "ab", "a", "UPPER", "-lead", "trail-", "www", "admin", "stores", "xn--puny", strings.Repeat("a", 31), "a b", "a_b"},
	} {
		for _, h := range handles {
			var got bool
			if err := o.QueryRow(ctx, `SELECT control.store_handle_valid($1)`, h).Scan(&got); err != nil {
				t.Fatal(err)
			}
			if got != valid {
				t.Errorf("store_handle_valid(%q) = %v, want %v", h, got, valid)
			}
		}
	}
	for _, tc := range []struct {
		h        string
		reserved bool
	}{
		{"www", true}, {"status", true}, {"xn--bcher", true}, {"shop1", false}, {"shop-1", false},
	} {
		var got bool
		if err := o.QueryRow(ctx, `SELECT control.store_handle_reserved($1)`, tc.h).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != tc.reserved {
			t.Errorf("store_handle_reserved(%q) = %v, want %v", tc.h, got, tc.reserved)
		}
	}

	// The BEFORE INSERT trigger refuses an explicit invalid/reserved handle.
	for name, h := range map[string]string{"invalid": "UPPER", "reserved": "admin", "trailing hyphen": "abc-"} {
		if _, err := o.Exec(ctx, `INSERT INTO control.stores(tenant_id,id,name,currency,handle) VALUES($1,$2,'sdw01','USD',$3)`, s.f.tenant, randomUUID(), h); pgCode(err) != "PT400" {
			t.Errorf("explicit %s handle (%q): err=%v, want PT400", name, h, err)
		}
	}

	// Owner 2026-10-03: every display name gets a random eight-digit ID, never a slug.
	var slug string
	if err := o.QueryRow(ctx, `SELECT control.assign_store_handle('Acme Shop', $1::uuid)`, randomUUID()).Scan(&slug); err != nil || !regexp.MustCompile(`^[1-9][0-9]{7}$`).MatchString(slug) {
		t.Fatalf("assign_store_handle numeric ID = %q, %v", slug, err)
	}
	id := randomUUID()
	var fallback string
	if err := o.QueryRow(ctx, `SELECT control.assign_store_handle('店铺名', $1::uuid)`, id).Scan(&fallback); err != nil || !regexp.MustCompile(`^[1-9][0-9]{7}$`).MatchString(fallback) {
		t.Fatalf("assign_store_handle non-ASCII name numeric ID = %q, %v", fallback, err)
	}
	// Persisted numbers cannot be reassigned; the unique index remains the race backstop.
	taken := randomUUID()
	if err := o.QueryRow(ctx, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'Taken Name','USD') RETURNING handle`, s.f.tenant, taken).Scan(&slug); err != nil || !regexp.MustCompile(`^[1-9][0-9]{7}$`).MatchString(slug) {
		t.Fatalf("first assign = %q, %v", slug, err)
	}
	var other string
	if err := o.QueryRow(ctx, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'Taken Name','USD') RETURNING handle`, s.f.tenant, randomUUID()).Scan(&other); err != nil || other == slug || !regexp.MustCompile(`^[1-9][0-9]{7}$`).MatchString(other) {
		t.Fatalf("second numeric ID = %q, %v; first %q", other, err, slug)
	}
	// The unique index is the concurrency backstop: a raw duplicate handle is a 23505, not silently accepted.
	if err := o.QueryRow(ctx, `INSERT INTO control.stores(tenant_id,id,name,currency,handle) VALUES($1,$2,'Dup','USD',$3) RETURNING id`, s.f.tenant, randomUUID(), slug).Scan(&id); pgCode(err) != "23505" {
		t.Fatalf("duplicate handle: err=%v, want 23505", err)
	}
}

func TestStoreDomainsSDW02PlatformRowActive(t *testing.T) {
	s := sdSetup(t)
	ctx := context.Background()
	o := s.b.owner

	handle := handleOf(t, o, s.f.store)
	if !regexp.MustCompile(`^[1-9][0-9]{7}$`).MatchString(handle) {
		t.Fatalf("auto handle = %q, want a random eight-digit store number", handle)
	}
	h, origin := platformOrigin(t, o, s.f.store, s.base)
	if h != handle || origin != "https://"+handle+".example.com" {
		t.Fatalf("platform origin = handle %q origin %q", h, origin)
	}
	state, token := domainState(t, o, origin)
	if state != "ACTIVE" || token != nil {
		t.Fatalf("platform row = state %q token %v", state, token)
	}
	var evidence, validUntil string
	if err := o.QueryRow(ctx, `SELECT evidence_ref,to_char(valid_until,'YYYY') FROM control.storefront_domains WHERE origin=$1`, origin).Scan(&evidence, &validUntil); err != nil {
		t.Fatal(err)
	}
	if evidence != "platform-subdomain" || validUntil < "2026" {
		t.Fatalf("platform evidence/validity = %q %q", evidence, validUntil)
	}
	// Idempotent under the advisory lock: a second call returns the same origin, no second row.
	if _, again := platformOrigin(t, o, s.f.store, s.base); again != origin {
		t.Fatalf("second ensure = %q, want %q", again, origin)
	}
	if n := countRows(t, o, `SELECT count(*) FROM control.storefront_domains WHERE origin=$1`, origin); n != 1 {
		t.Fatalf("platform rows for %s = %d, want 1", origin, n)
	}
	// A store whose handle is cleared (the pre-0106 shape the backfill exists for) cannot get a platform origin.
	ghost := randomUUID()
	mustExec(t, o, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'ghost','USD')`, s.f.tenant, ghost)
	mustExec(t, o, `UPDATE control.stores SET handle=NULL WHERE id=$1`, ghost)
	var ghostHandle string
	if c := pgCode(o.QueryRow(ctx, `SELECT control.ensure_store_platform_domain($1::uuid,'example.com')`, ghost).Scan(&ghostHandle)); c != "PT409" {
		t.Fatalf("platform origin for a handle-less store: %q, want PT409", c)
	}
}

func TestStoreDomainsSDW03MerchantDomainLifecycle(t *testing.T) {
	s := sdSetup(t)
	ctx := context.Background()
	o := s.b.owner
	host := sdHost()
	origin := "https://" + host

	res, err := s.request(s.f.token, s.f.store, host)
	if err != nil || res.State != "REQUESTED" || res.Origin != origin || res.Version != 1 || res.DomainID == "" ||
		res.DNS.TXTName != "_lc-verify."+host || res.DNS.TXTValue == "" || res.DNS.CNAMETarget != "stores.example.com" || res.DNS.Apex {
		t.Fatalf("request: %+v %v", res, err)
	}
	if state, token := domainState(t, o, origin); state != "REQUESTED" || token == nil || *token != res.DNS.TXTValue {
		t.Fatalf("requested row = state %q token %v", state, token)
	}
	// The pending batch read hands the worker the hostname + token; nothing else.
	var dhost, dtok string
	var did string
	if err := s.reg.QueryRow(ctx, `SELECT domain_id,hostname,token FROM control.next_store_domain_dns_check() WHERE domain_id=$1`, res.DomainID).Scan(&did, &dhost, &dtok); err != nil || dhost != host || dtok != res.DNS.TXTValue {
		t.Fatalf("next_store_domain_dns_check = %q %q %q, %v", did, dhost, dtok, err)
	}

	// Worker transitions: DNS match -> OWNERSHIP_PENDING -> TLS_PENDING, then TLS complete -> ACTIVE (token cleared).
	adv := func(want string) string {
		t.Helper()
		var raw []byte
		if err := s.reg.QueryRow(ctx, `SELECT control.store_domain_dns_advance($1::uuid,true)`, res.DomainID).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var out storefrontdomains.AdvanceResult
		if err := json.Unmarshal(raw, &out); err != nil || out.State != want || out.Expired || out.DNSFailures != 0 {
			t.Fatalf("dns_advance = %s %v (want %s)", raw, err, want)
		}
		return out.State
	}
	if got := adv("OWNERSHIP_PENDING"); got != "OWNERSHIP_PENDING" {
		t.Fatalf("first advance = %q", got)
	}
	if got := adv("TLS_PENDING"); got != "TLS_PENDING" {
		t.Fatalf("second advance = %q", got)
	}
	if n := countRows(t, s.reg, `SELECT count(*) FROM control.next_store_domain_dns_check()`); n != 0 {
		t.Fatalf("pending DNS checks after advancing = %d, want 0", n)
	}
	var tlsOrigin string
	if err := s.reg.QueryRow(ctx, `SELECT domain_id,origin FROM control.next_store_domain_tls_probe() WHERE domain_id=$1`, res.DomainID).Scan(&did, &tlsOrigin); err != nil || tlsOrigin != origin {
		t.Fatalf("next_store_domain_tls_probe = %q %q, %v", did, tlsOrigin, err)
	}
	notAfter := time.Now().Add(90 * 24 * time.Hour).UTC()
	var complete struct {
		DomainID string `json:"domain_id"`
		State    string `json:"state"`
		Version  int64  `json:"version"`
	}
	var raw []byte
	if err := s.reg.QueryRow(ctx, `SELECT control.store_domain_tls_complete($1::uuid,$2)`, res.DomainID, notAfter).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &complete); err != nil || complete.DomainID != res.DomainID || complete.State != "ACTIVE" || complete.Version < 1 {
		t.Fatalf("tls_complete = %s %v", raw, err)
	}
	if state, token := domainState(t, o, origin); state != "ACTIVE" || token != nil {
		t.Fatalf("active row = state %q token %v", state, token)
	}
	rd, err := s.read(s.f.token, s.f.store)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range rd.Domains {
		if d.Origin == origin && (d.State != "ACTIVE" || d.Token != nil || !d.Serving) {
			t.Fatalf("merchant view of ACTIVE origin: %+v", d)
		}
	}
}

func TestStoreDomainsSDW04RefusalsAndOwnerOnly(t *testing.T) {
	s := sdSetup(t)
	o := s.b.owner

	// Under the platform base zone.
	if _, err := s.request(s.f.token, s.f.store, "shop.example.com"); !errors.Is(err, storefrontdomains.ErrReservedHostname) {
		t.Fatalf("under-base request: %v, want ErrReservedHostname", err)
	}
	// Invalid hostname (Go validator, before any connection).
	if _, err := s.request(s.f.token, s.f.store, "localhost"); !errors.Is(err, command.ErrInvalid) {
		t.Fatalf("localhost request: %v, want ErrInvalid", err)
	}
	// Owner-only: the integration:manage permission gates the request, not the bearer's existence.
	if _, err := s.request(s.f.otherToken, s.f.store, sdHost()); !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("integration:read grantee requested a domain: %v, want forbidden", err)
	}
	if _, err := s.request(s.f.missingPermission, s.f.store, sdHost()); !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("store:read-only grantee requested a domain: %v, want forbidden", err)
	}

	// Taken host: the same origin requested by a sibling store of the same tenant is domain_owned_elsewhere.
	host := sdHost()
	if _, err := s.request(s.f.token, s.f.store, host); err != nil {
		t.Fatal(err)
	}
	if _, err := s.request(s.f.token, s.f.otherStore, host); !errors.Is(err, storefrontdomains.ErrDomainOwnedElsewhere) {
		t.Fatalf("sibling store re-requested the host: %v, want ErrDomainOwnedElsewhere", err)
	}

	// Suspend/detach are owner actions on the store's own origin; another store cannot suspend it.
	origin := "https://" + host
	mv, err := s.suspend(s.f.token, s.f.store, origin)
	if err != nil || mv.State != "SUSPENDED" || !mv.Changed {
		t.Fatalf("suspend: %+v %v", mv, err)
	}
	if state, _ := domainState(t, o, origin); state != "SUSPENDED" {
		t.Fatalf("suspend did not persist: %q", state)
	}
	// P2-3: the owner can re-request their own SUSPENDED domain; it returns to REQUESTED with a fresh token.
	if again, err := s.request(s.f.token, s.f.store, host); err != nil || again.State != "REQUESTED" || again.Origin != origin {
		t.Fatalf("re-request of SUSPENDED: %+v %v, want REQUESTED", again, err)
	}
	// Detach is final for the owner path.
	mv, err = s.detach(s.f.token, s.f.store, origin)
	if err != nil || mv.State != "DETACHED" || !mv.Changed {
		t.Fatalf("detach: %+v %v", mv, err)
	}
	if state, _ := domainState(t, o, origin); state != "DETACHED" {
		t.Fatalf("detach did not persist: %q", state)
	}
	if again, err := s.detach(s.f.token, s.f.store, origin); err != nil || again.Changed || again.State != "DETACHED" {
		t.Fatalf("repeat detach: %+v %v, want a no-op", again, err)
	}
}

func TestStoreDomainsSDW05Worker301AndTLSAsk(t *testing.T) {
	s := sdSetup(t)
	ctx := context.Background()
	o := s.b.owner

	// The platform subdomain is ACTIVE and primary until a merchant ACTIVE origin takes over.
	_, platform := platformOrigin(t, o, s.f.store, s.base)
	host := sdHost()
	origin := "https://" + host
	res, err := s.request(s.f.token, s.f.store, host)
	if err != nil {
		t.Fatal(err)
	}
	// TLS ask: the merchant host is not eligible until TLS_PENDING/ACTIVE; the platform subdomain is.
	var ask bool
	if err := s.b.runtime.QueryRow(ctx, `SELECT control.resolve_storefront_ask($1)`, host).Scan(&ask); err != nil || ask {
		t.Fatalf("ask for a REQUESTED host = %v %v, want false", ask, err)
	}
	if err := s.b.runtime.QueryRow(ctx, `SELECT control.resolve_storefront_ask($1)`, strings.TrimPrefix(platform, "https://")).Scan(&ask); err != nil || !ask {
		t.Fatalf("ask for the platform subdomain = %v %v, want true", ask, err)
	}
	// Advance to ACTIVE via the registrar, then the merchant host answers the ask too.
	for i := 0; i < 2; i++ {
		var raw []byte
		if err := s.reg.QueryRow(ctx, `SELECT control.store_domain_dns_advance($1::uuid,true)`, res.DomainID).Scan(&raw); err != nil {
			t.Fatal(err)
		}
	}
	var tlsRaw []byte
	if err := s.reg.QueryRow(ctx, `SELECT control.store_domain_tls_complete($1::uuid,$2)`, res.DomainID, time.Now().Add(90*24*time.Hour).UTC()).Scan(&tlsRaw); err != nil {
		t.Fatal(err)
	}
	if err := s.b.runtime.QueryRow(ctx, `SELECT control.resolve_storefront_ask($1)`, host).Scan(&ask); err != nil || !ask {
		t.Fatalf("ask for an ACTIVE merchant host = %v %v, want true", ask, err)
	}

	// 301 primary: the merchant ACTIVE origin is primary; the platform subdomain 301s to it (and to itself = none).
	primary, err := storefrontdomains.PrimaryOrigin(ctx, s.buyer, platform)
	if err != nil || primary != origin {
		t.Fatalf("primary of the platform subdomain = %q %v, want %q", primary, err, origin)
	}
	self, err := storefrontdomains.PrimaryOrigin(ctx, s.buyer, origin)
	if err != nil || self != "" {
		t.Fatalf("primary of the primary = %q %v, want empty (no redirect)", self, err)
	}
	unknown, err := storefrontdomains.PrimaryOrigin(ctx, s.buyer, "https://"+sdHost())
	if err != nil || unknown != "" {
		t.Fatalf("primary of an unknown origin = %q %v, want empty", unknown, err)
	}
}

// sdFns is the full 0106 function surface: function -> owner, whether it is SECURITY DEFINER (the two pure-SQL
// handle helpers are not: they are IMMUTABLE and touch no table), and the non-owner EXECUTE grantees. The owner
// always keeps EXECUTE (materialised into the ACL by every REVOKE ALL ... FROM PUBLIC), so it is asserted by the
// test itself rather than listed here; nil means the function is owner-only.
var sdFns = []struct {
	fn, owner string
	secdef    bool
	exec      []string
}{
	{"control.store_handle_reserved(text)", "commerce_identity_writer", false, []string{"commerce_storefront_writer"}},
	{"control.store_handle_valid(text)", "commerce_identity_writer", false, []string{"commerce_storefront_writer"}},
	{"control.store_handle_taken(text)", "commerce_storefront_writer", true, []string{"commerce_identity_writer"}},
	{"control.assign_store_handle(text,uuid)", "commerce_identity_writer", true, nil},
	{"control.stores_handle_default()", "commerce_identity_writer", true, nil},
	{"control.stores_handle_changed()", "commerce_storefront_writer", true, nil},
	{"control.operator_set_store_handle(uuid,text,boolean)", "commerce_storefront_writer", true, []string{"commerce_storefront_registrar"}},
	{"control.ensure_store_platform_domain(uuid,text)", "commerce_storefront_writer", true, []string{"commerce_identity", "commerce_storefront_registrar"}},
	{"control.backfill_platform_domains(text)", "commerce_storefront_writer", true, []string{"commerce_storefront_registrar"}},
	{"control.request_merchant_domain(bytea,uuid,text,text,text)", "commerce_storefront_writer", true, []string{"commerce_runtime"}},
	{"control.read_store_domains(bytea,uuid)", "commerce_storefront_writer", true, []string{"commerce_runtime"}},
	{"control.suspend_merchant_domain(bytea,uuid,text)", "commerce_storefront_writer", true, []string{"commerce_runtime"}},
	{"control.detach_merchant_domain(bytea,uuid,text)", "commerce_storefront_writer", true, []string{"commerce_runtime"}},
	{"control.store_domain_dns_advance(uuid,boolean)", "commerce_storefront_writer", true, []string{"commerce_storefront_verifier"}},
	{"control.store_domain_tls_complete(uuid,timestamptz)", "commerce_storefront_writer", true, []string{"commerce_storefront_verifier"}},
	{"control.next_store_domain_dns_check()", "commerce_storefront_writer", true, []string{"commerce_storefront_verifier"}},
	{"control.next_store_domain_tls_probe()", "commerce_storefront_writer", true, []string{"commerce_storefront_verifier"}},
	{"control.store_domain_tls_renew(uuid,timestamptz)", "commerce_storefront_writer", true, []string{"commerce_storefront_verifier"}},
	{"control.next_store_domain_tls_renewal()", "commerce_storefront_writer", true, []string{"commerce_storefront_verifier"}},
	{"control.resolve_storefront_ask(text)", "commerce_storefront_writer", true, []string{"commerce_runtime"}},
	{"control.tls_pending_nonce(text,text)", "commerce_storefront_writer", true, []string{"commerce_runtime"}},
	{"control.admitted_storefront_hosts()", "commerce_storefront_writer", true, []string{"commerce_runtime"}},
	{"control.resolve_primary_origin(text)", "commerce_storefront_writer", true, []string{"commerce_buyer_runtime"}},
}

func TestStoreDomainsSDW06SchemaAndACLInventory(t *testing.T) {
	s := sdSetup(t)
	ctx := context.Background()
	o := s.b.owner

	for _, fn := range sdFns {
		var secdef bool
		var owner, cfg, vol, comment *string
		var acl []string
		err := o.QueryRow(ctx, `SELECT p.prosecdef,pg_get_userbyid(p.proowner),coalesce(p.proconfig::text,''),p.provolatile::text,
			obj_description(p.oid,'pg_proc'),coalesce((SELECT array_agg(a.grantee::regrole::text ORDER BY a.grantee::regrole::text) FROM aclexplode(p.proacl) a WHERE a.privilege_type='EXECUTE'),'{}')
			FROM pg_proc p WHERE p.oid=$1::regprocedure`, fn.fn).Scan(&secdef, &owner, &cfg, &vol, &comment, &acl)
		if err != nil {
			t.Fatalf("%s missing: %v", fn.fn, err)
		}
		if secdef != fn.secdef || *owner != fn.owner || !strings.Contains(*cfg, "search_path=pg_catalog") || comment == nil || *comment == "" {
			t.Fatalf("%s: secdef=%v want=%v owner=%s config=%s comment=%v", fn.fn, secdef, fn.secdef, *owner, *cfg, comment)
		}
		want := append(append([]string{}, fn.exec...), fn.owner)
		lcSameSet(t, fn.fn+" EXECUTE grantees", acl, want)
		var publicExec bool
		if err := o.QueryRow(ctx, `SELECT coalesce(bool_or(a.grantee=0),false) FROM pg_proc p, aclexplode(p.proacl) a WHERE p.oid=$1::regprocedure`, fn.fn).Scan(&publicExec); err != nil || publicExec {
			t.Fatalf("%s is executable by PUBLIC (err=%v)", fn.fn, err)
		}
		// The retired commerce_worker (and every worker authority) holds no EXECUTE on any 0106 definer.
		if err := o.QueryRow(ctx, `SELECT has_function_privilege('commerce_worker',$1::regprocedure,'EXECUTE')`, fn.fn).Scan(&publicExec); err != nil || publicExec {
			t.Fatalf("%s is executable by commerce_worker (err=%v)", fn.fn, err)
		}
	}
	// The BEFORE INSERT trigger exists and runs the handle default.
	if n := countRows(t, o, `SELECT count(*) FROM pg_trigger WHERE tgname='stores_handle_before_insert' AND tgrelid='control.stores'::regclass AND NOT tgisinternal`); n != 1 {
		t.Fatalf("stores_handle_before_insert = %d triggers, want 1", n)
	}
	// The AFTER UPDATE trigger detaches the old platform origin on a handle change (P1-4).
	if n := countRows(t, o, `SELECT count(*) FROM pg_trigger WHERE tgname='stores_handle_after_update' AND tgrelid='control.stores'::regclass AND NOT tgisinternal`); n != 1 {
		t.Fatalf("stores_handle_after_update = %d triggers, want 1", n)
	}
	// The handle column shape: nullable text, the format CHECK, the partial unique index.
	var nullable bool
	if err := o.QueryRow(ctx, `SELECT attnotnull FROM pg_attribute WHERE attrelid='control.stores'::regclass AND attname='handle'`).Scan(&nullable); err != nil || nullable {
		t.Fatalf("stores.handle must be nullable (trigger-filled, backfilled): %v", err)
	}
	if n := countRows(t, o, `SELECT count(*) FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid WHERE i.indrelid='control.stores'::regclass AND c.relname='stores_handle_unique' AND i.indisunique`); n != 1 {
		t.Fatalf("stores_handle_unique = %d, want 1 partial unique index", n)
	}
	// Column-level grant only: the writer can read the handle, the identity login names the schema without table rights.
	var writerHandle, writerStore bool
	if err := o.QueryRow(ctx, `SELECT has_column_privilege('commerce_storefront_writer','control.stores','handle','SELECT'),
		has_table_privilege('commerce_storefront_writer','control.stores','SELECT,INSERT,UPDATE,DELETE')`).Scan(&writerHandle, &writerStore); err != nil || !writerHandle || writerStore {
		t.Fatalf("writer stores.handle grant = %v, broad table access = %v (err=%v)", writerHandle, writerStore, err)
	}
	var identityUsage, identityTable bool
	if err := o.QueryRow(ctx, `SELECT has_schema_privilege('commerce_identity','control','USAGE'),
		has_table_privilege('commerce_identity','control.stores','SELECT,INSERT,UPDATE,DELETE') OR has_any_column_privilege('commerce_identity','control.stores','SELECT,INSERT,UPDATE')`).Scan(&identityUsage, &identityTable); err != nil || !identityUsage || identityTable {
		t.Fatalf("commerce_identity control usage = %v, any stores privilege = %v (err=%v)", identityUsage, identityTable, err)
	}
	// The registrar, verifier and runtime authorities have no direct write to the domain rows (they reach them through definers).
	for _, role := range []string{"commerce_storefront_registrar", "commerce_runtime", "commerce_storefront_verifier"} {
		var access bool
		if err := o.QueryRow(ctx, `SELECT has_table_privilege($1,'control.storefront_domains','INSERT,UPDATE,DELETE') OR has_any_column_privilege($1,'control.storefront_domains','INSERT,UPDATE')`, role).Scan(&access); err != nil || access {
			t.Fatalf("%s has direct write on storefront_domains (err=%v)", role, err)
		}
	}
	// N-P1-1: the verifier holds EXECUTE on exactly the six worker definers (four transitions + two renewal) and
	// nothing else anywhere — the WAS-style inventory. The sdFns loop above already proves the positive direction
	// (each of the six lists the verifier); this proves the negative: any other EXECUTE is drift.
	verifierFns := `ARRAY[
		'control.store_domain_dns_advance(uuid,boolean)'::regprocedure,
		'control.store_domain_tls_complete(uuid,timestamptz)'::regprocedure,
		'control.next_store_domain_dns_check()'::regprocedure,
		'control.next_store_domain_tls_probe()'::regprocedure,
		'control.store_domain_tls_renew(uuid,timestamptz)'::regprocedure,
		'control.next_store_domain_tls_renewal()'::regprocedure]`
	var verifierExtra []string
	// Explicit EXECUTE grants only (aclexplode), so PUBLIC-granted helpers (the river *_state_in_bitmask catalog
	// functions) never read as verifier privileges — has_function_privilege would include them via PUBLIC.
	vrows, err := o.Query(ctx, `SELECT p.oid::regprocedure::text
		FROM pg_proc p
		JOIN pg_namespace n ON n.oid=p.pronamespace
		JOIN pg_roles r ON r.rolname='commerce_storefront_verifier'
		CROSS JOIN LATERAL aclexplode(p.proacl) a
		WHERE a.grantee=r.oid AND a.privilege_type='EXECUTE'
		  AND NOT p.oid = ANY(`+verifierFns+`)
		ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	for vrows.Next() {
		var fn string
		if err := vrows.Scan(&fn); err != nil {
			vrows.Close()
			t.Fatal(err)
		}
		verifierExtra = append(verifierExtra, fn)
	}
	vrows.Close()
	if err := vrows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(verifierExtra) != 0 {
		t.Fatalf("commerce_storefront_verifier holds unexpected EXECUTE: %v", verifierExtra)
	}
	// The 0081 audit policy still admits its original six actions plus the three merchant self-service ones (same policy).
	var withCheck string
	if err := o.QueryRow(ctx, `SELECT pg_get_expr(polwithcheck,polrelid) FROM pg_policy WHERE polname='storefront_writer_audit_insert' AND polrelid='ops.audit_events'::regclass`).Scan(&withCheck); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"merchant.domain_requested", "merchant.domain_suspended", "merchant.domain_detached", "operator.domain_bound", "operator.handle_set"} {
		if !strings.Contains(withCheck, "'"+action+"'") {
			t.Fatalf("audit policy lost %s: %s", action, withCheck)
		}
	}
}

// publish flips the merchant's publication on a store through control.set_storefront_published, as the Settings card
// does — so SDW07 can assert the operator refuses a published store without --after-publish.
func (s *sdFix) publish(store string) {
	s.t.Helper()
	if err := s.merchant("integration:manage", s.f.token, store, func(tx pgx.Tx, sc platform.Scope) error {
		_, e := storefrontadmin.SetPublished(context.Background(), tx, sc, s.f.token, true, 0)
		return e
	}); err != nil {
		s.t.Fatal(err)
	}
}

// handleSetRaw runs control.operator_set_store_handle directly on the registrar pool with the base GUC set
// transaction-locally, independent of the Go HandleSet wrapper, so the SQL definer's own validation is asserted.
func (s *sdFix) handleSetRaw(store, handle string, afterPublish, setBase bool) error {
	s.t.Helper()
	ctx := context.Background()
	tx, err := s.op.Begin(ctx)
	if err != nil {
		s.t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if setBase {
		if _, err := tx.Exec(ctx, `SELECT set_config('lc.store_base_domain','example.com',true)`); err != nil {
			s.t.Fatal(err)
		}
	}
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT control.operator_set_store_handle($1::uuid,$2,$3)`, store, handle, afterPublish).Scan(&raw); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// opBeginner adapts the registrar pool to storefrontadmin.Beginner (pgx.Tx satisfies storefrontadmin.Tx), so the Go
// HandleSet wrapper can be exercised against a real pool exactly as the CLI does.
type opBeginner struct{ pool *pgxpool.Pool }

func (b opBeginner) Begin(ctx context.Context) (storefrontadmin.Tx, error) { return b.pool.Begin(ctx) }

func (s *sdFix) opBegin() storefrontadmin.Beginner { return opBeginner{s.op} }

// TestStoreDomainsSDW07OperatorHandleSet is the R5 Decision 1 gate: control.operator_set_store_handle validates like
// assignment, refuses a published store without --after-publish, and in one transaction changes the handle, detaches
// the old platform origin and creates the new ACTIVE origin, with one audit row.
func TestStoreDomainsSDW07OperatorHandleSet(t *testing.T) {
	s := sdSetup(t)
	ctx := context.Background()
	o := s.b.owner

	oldHandle := handleOf(t, o, s.f.store)
	_, oldOrigin := platformOrigin(t, o, s.f.store, s.base)
	otherHandle := handleOf(t, o, s.f.otherStore)
	if otherHandle == oldHandle {
		t.Fatalf("fixture handles collide: %q", oldHandle)
	}

	// The operator definer attributes its audit row to the store's owner principal (identity.initial_stores); the t06
	// fixture inserts stores directly (no onboarding), so seed the owner row + warehouse here (as spSetup does).
	wh := randomUUID()
	mustExec(t, o, `INSERT INTO inventory.warehouses(tenant_id,store_id,id,name) VALUES($1,$2,$3,'sdw07-wh')`, s.f.tenant, s.f.store, wh)
	mustExec(t, o, `INSERT INTO identity.initial_stores(principal_id,idempotency_key,request_hash,tenant_id,store_id,warehouse_id)
		VALUES($1,$2,decode(repeat('ab',32),'hex'),$3,$4,$5)`, s.f.principal, "sdw07-"+randomUUID()[:16], s.f.tenant, s.f.store, wh)
	t.Cleanup(func() {
		_, _ = o.Exec(context.Background(), `DELETE FROM identity.initial_stores WHERE tenant_id=$1`, s.f.tenant)
		_, _ = o.Exec(context.Background(), `DELETE FROM inventory.warehouses WHERE tenant_id=$1 AND name='sdw07-wh'`, s.f.tenant)
	})

	// The SQL definer's own validation, asserted directly (independent of the Go parity wrapper).
	if c := pgCode(s.handleSetRaw(s.f.store, "UPPER", false, true)); c != "PT400" {
		t.Errorf("invalid handle: %q, want PT400", c)
	}
	if c := pgCode(s.handleSetRaw(s.f.store, "admin", false, true)); c != "PT400" {
		t.Errorf("reserved handle: %q, want PT400", c)
	}
	if c := pgCode(s.handleSetRaw(s.f.store, "xn--puny", false, true)); c != "PT400" {
		t.Errorf("xn-- handle: %q, want PT400", c)
	}
	if c := pgCode(s.handleSetRaw(randomUUID(), "fresh-handle", false, true)); c != "PT404" {
		t.Errorf("unknown store: %q, want PT404", c)
	}
	if c := pgCode(s.handleSetRaw(s.f.store, "fresh-handle", false, false)); c != "PT409" {
		t.Errorf("base GUC unset: %q, want PT409", c)
	}
	if c := pgCode(s.handleSetRaw(s.f.store, otherHandle, false, true)); c != "PT409" {
		t.Errorf("taken handle: %q, want PT409", c)
	}

	// Valid change on a never-published store: the handle changes, the old platform origin is DETACHED and the new
	// one ACTIVE in the same transaction, and exactly one audit row is written.
	res, err := storefrontadmin.HandleSet(ctx, s.opBegin(), s.f.store, "renamed-handle", false, s.base)
	if err != nil || !res.Changed || res.StoreID != s.f.store || res.Handle != "renamed-handle" ||
		res.Origin != "https://renamed-handle.example.com" || res.AfterPublish {
		t.Fatalf("HandleSet valid = %+v %v", res, err)
	}
	if got := handleOf(t, o, s.f.store); got != "renamed-handle" {
		t.Fatalf("stores.handle = %q, want renamed-handle", got)
	}
	if state, _ := domainState(t, o, oldOrigin); state != "DETACHED" {
		t.Fatalf("old origin %s = %q, want DETACHED", oldOrigin, state)
	}
	if state, token := domainState(t, o, "https://renamed-handle.example.com"); state != "ACTIVE" || token != nil {
		t.Fatalf("new origin = %q %v, want ACTIVE with no token", state, token)
	}
	if n := countRows(t, o, `SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='operator.handle_set'`, s.f.store); n != 1 {
		t.Fatalf("audit operator.handle_set rows = %d, want 1", n)
	}

	// The Go wrapper mirrors the SQL validation: invalid/reserved refused before the DB, taken -> ErrConflict.
	for _, h := range []string{"UPPER", "admin", "xn--puny"} {
		if _, err := storefrontadmin.HandleSet(ctx, s.opBegin(), s.f.store, h, false, s.base); !errors.Is(err, command.ErrInvalid) {
			t.Errorf("HandleSet(%q) = %v, want ErrInvalid", h, err)
		}
	}
	if _, err := storefrontadmin.HandleSet(ctx, s.opBegin(), s.f.store, otherHandle, false, s.base); !errors.Is(err, command.ErrConflict) {
		t.Errorf("HandleSet(taken) = %v, want ErrConflict", err)
	}

	// A published store refuses the change without --after-publish, and allows it with the flag.
	s.publish(s.f.store)
	if _, err := storefrontadmin.HandleSet(ctx, s.opBegin(), s.f.store, "published-handle", false, s.base); !errors.Is(err, storefrontadmin.ErrStorePublished) {
		t.Errorf("HandleSet on a published store = %v, want ErrStorePublished", err)
	}
	res2, err := storefrontadmin.HandleSet(ctx, s.opBegin(), s.f.store, "published-handle", true, s.base)
	if err != nil || !res2.Changed || res2.Handle != "published-handle" ||
		res2.Origin != "https://published-handle.example.com" || !res2.AfterPublish {
		t.Fatalf("HandleSet --after-publish = %+v %v", res2, err)
	}
	if state, _ := domainState(t, o, "https://published-handle.example.com"); state != "ACTIVE" {
		t.Fatalf("published-handle origin = %q, want ACTIVE", state)
	}
}

// TestStoreDomainsIdempotency is the P0/I02 gate: a merchant domain write reached through the service runs in
// command.Run, so the same key+body replays the first result without rotating the TXT token/version or adding an
// audit row, and the same key with a different body is refused (receipt hash mismatch -> ErrConflict) with no write.
func TestStoreDomainsIdempotency(t *testing.T) {
	s := sdSetup(t)
	ctx := context.Background()
	o := s.b.owner
	host := sdHost()
	origin := "https://" + host
	key := "idem-" + randomUUID()

	first, err := s.requestKey(s.f.token, s.f.store, host, key)
	if err != nil || first.State != "REQUESTED" {
		t.Fatalf("first request: %+v %v", first, err)
	}
	state, token := domainState(t, o, origin)
	var version int64
	if err := o.QueryRow(ctx, `SELECT version FROM control.storefront_domains WHERE origin=$1`, origin).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if state != "REQUESTED" || token == nil || version != 1 {
		t.Fatalf("first row = state %q token %v version %d", state, token, version)
	}

	// Same key + same body replays the first result: the row (token/version) and the audit trail do not change.
	replay, err := s.requestKey(s.f.token, s.f.store, host, key)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if replay.DomainID != first.DomainID || replay.Version != first.Version || replay.DNS.TXTValue != first.DNS.TXTValue {
		t.Fatalf("replay = %+v, want the first result %+v", replay, first)
	}
	replayState, replayToken := domainState(t, o, origin)
	var replayVersion int64
	if err := o.QueryRow(ctx, `SELECT version FROM control.storefront_domains WHERE origin=$1`, origin).Scan(&replayVersion); err != nil {
		t.Fatal(err)
	}
	if replayState != "REQUESTED" || replayToken == nil || *replayToken != *token || replayVersion != version {
		t.Fatalf("replay row = state %q token %v version %d, want unchanged", replayState, replayToken, replayVersion)
	}
	if n := countRows(t, o, `SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='merchant.domain_requested'`, s.f.store); n != 1 {
		t.Fatalf("audit rows after replay = %d, want 1", n)
	}

	// Same key + a different body is refused, and writes nothing.
	if _, err := s.requestKey(s.f.token, s.f.store, sdHost(), key); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("same key different body: %v, want ErrConflict", err)
	}
	if n := countRows(t, o, `SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='merchant.domain_requested'`, s.f.store); n != 1 {
		t.Fatalf("audit rows after conflict = %d, want 1", n)
	}
}

// TestStoreDomainsMerchantViewKind is the DTO kind gate: the read DTO carries an authoritative kind (platform for the
// platform subdomain row, custom for a merchant row) from the row's evidence, so the UI can hide suspend/detach on the
// platform row without inferring it from the suffix.
func TestStoreDomainsMerchantViewKind(t *testing.T) {
	s := sdSetup(t)
	o := s.b.owner
	_, platform := platformOrigin(t, o, s.f.store, s.base)
	host := sdHost()
	origin := "https://" + host
	if _, err := s.request(s.f.token, s.f.store, host); err != nil {
		t.Fatal(err)
	}
	rd, err := s.read(s.f.token, s.f.store)
	if err != nil {
		t.Fatal(err)
	}
	byOrigin := make(map[string]storefrontdomains.DomainRow, len(rd.Domains))
	for _, d := range rd.Domains {
		byOrigin[d.Origin] = d
	}
	if d := byOrigin[platform]; d.Kind != "platform" {
		t.Fatalf("platform row %q kind = %q, want platform", platform, d.Kind)
	}
	if d := byOrigin[origin]; d.Kind != "custom" {
		t.Fatalf("merchant row %q kind = %q, want custom", origin, d.Kind)
	}
}

// TestStoreDomainsApexEdgeInstructions is the apex DNS-instruction gate: an apex host's request returns the edge
// A/AAAA set the verifier accepts (the same stores.<base> source), while a CNAME host keeps only the CNAME target.
func TestStoreDomainsApexEdgeInstructions(t *testing.T) {
	s := sdSetup(t)
	apexHost := "apex-" + strings.ReplaceAll(randomUUID(), "-", "")[:12] + ".net"
	edge := []string{"203.0.113.10", "2001:db8::10"}

	var out storefrontdomains.RequestResult
	if err := s.merchant("integration:manage", s.f.token, s.f.store, func(tx pgx.Tx, sc platform.Scope) error {
		var e error
		out, e = storefrontdomains.Request(context.Background(), tx, sc, s.f.token, "apex-"+randomUUID(), apexHost, s.base, edge)
		return e
	}); err != nil {
		t.Fatal(err)
	}
	if !out.DNS.Apex || len(out.DNS.EdgeAddresses) != 2 || out.DNS.EdgeAddresses[0] != edge[0] || out.DNS.EdgeAddresses[1] != edge[1] {
		t.Fatalf("apex instructions = %+v, want apex with edge %v", out.DNS, edge)
	}

	// A CNAME host keeps the CNAME instruction and no edge addresses.
	cname := sdHost()
	var cnameOut storefrontdomains.RequestResult
	if err := s.merchant("integration:manage", s.f.token, s.f.store, func(tx pgx.Tx, sc platform.Scope) error {
		var e error
		cnameOut, e = storefrontdomains.Request(context.Background(), tx, sc, s.f.token, "cname-"+randomUUID(), cname, s.base, edge)
		return e
	}); err != nil {
		t.Fatal(err)
	}
	if cnameOut.DNS.Apex || len(cnameOut.DNS.EdgeAddresses) != 0 || cnameOut.DNS.CNAMETarget != "stores."+s.base {
		t.Fatalf("cname instructions = %+v, want CNAME only", cnameOut.DNS)
	}
}

// TestStoreDomainsDeployPassesBaseDomainToMigrate (K3 final review P1): the migrate one-shot must receive LC_STORE_BASE_DOMAIN,
// otherwise 0106's platform-origin backfill silently no-ops in a deploy and existing stores (the pilot) get no https://<handle>.<base>;
// and preflight must FAIL (not skip) when the value is unset or invalid, in every environment (offline rule P19).
func TestStoreDomainsDeployPassesBaseDomainToMigrate(t *testing.T) {
	read := func(p string) string {
		b, err := os.ReadFile("../../" + p)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	compose := read("deploy/compose.yml")
	start := strings.Index(compose, "\n  migrate:\n")
	if start < 0 {
		t.Fatal("compose has no migrate service")
	}
	rest := compose[start+1:]
	if end := regexp.MustCompile(`\n  [a-z][a-z0-9-]*:\n`).FindStringIndex(rest[1:]); end != nil {
		rest = rest[:end[0]+1]
	}
	if !strings.Contains(rest, "LC_STORE_BASE_DOMAIN: ${LC_STORE_BASE_DOMAIN") {
		t.Fatal("compose migrate service does not pass LC_STORE_BASE_DOMAIN: 0106 backfill would silently skip existing stores")
	}
	// The api (onboarding handle + platform origin, merchant DNS instructions) and the claims-worker (verify sweep) read it too.
	for _, svc := range []string{"api", "claims-worker"} {
		start := strings.Index(compose, "\n  "+svc+":\n")
		if start < 0 {
			t.Fatalf("compose has no %s service", svc)
		}
		block := compose[start+1:]
		if end := regexp.MustCompile(`\n  [a-z][a-z0-9-]*:\n`).FindStringIndex(block[1:]); end != nil {
			block = block[:end[0]+1]
		}
		if !strings.Contains(block, "LC_STORE_BASE_DOMAIN: ${LC_STORE_BASE_DOMAIN") {
			t.Fatalf("compose %s service does not pass LC_STORE_BASE_DOMAIN", svc)
		}
	}
	if !strings.Contains(read("deploy/scripts/preflight.sh"), `"LC_STORE_BASE_DOMAIN is unset or not a DNS zone`) {
		t.Fatal("preflight does not fail an unset LC_STORE_BASE_DOMAIN")
	}
}
