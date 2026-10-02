package foundation_test

// R5 unit store-domains, independent PG gate (REAL_PG; evidence MOCK: nothing here verifies DNS or a certificate).
// Written from docs/delivery/units/store-domains.md and migrations/0106_store_domains.sql, not from the implementation:
//
//	SDW01 handle format / reserved / uniqueness / suffix       SDW02 platform row ACTIVE + idempotence
//	SDW03 merchant domain lifecycle (request -> ACTIVE)        SDW04 refusals + owner-only scope
//	SDW05 worker transitions + 301 primary + TLS ask           SDW06 schema/ACL inventory + EXECUTE matrix

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
	"livecommerce/internal/storefrontdomains"
)

// sdFix wraps the merchant-scoped t06GoFixture with the two service logins the unit adds work to: the registrar
// (worker transitions) and the buyer runtime (301 primary lookup). base is the platform base zone (LC_STORE_BASE_DOMAIN).
type sdFix struct {
	t     *testing.T
	f     *t06GoFixture
	b     *testFixture
	reg   *pgxpool.Pool
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
		reg:   miPool(t, b, "commerce_storefront_registrar"),
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

func (s *sdFix) request(token, store, host string) (storefrontdomains.RequestResult, error) {
	var out storefrontdomains.RequestResult
	err := s.merchant("integration:manage", token, store, func(tx pgx.Tx, sc platform.Scope) error {
		var e error
		out, e = storefrontdomains.Request(context.Background(), tx, sc, token, host, s.base)
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
		out, e = storefrontdomains.Suspend(context.Background(), tx, sc, token, origin)
		return e
	})
	return out, err
}

func (s *sdFix) detach(token, store, origin string) (storefrontdomains.MoveResult, error) {
	var out storefrontdomains.MoveResult
	err := s.merchant("integration:manage", token, store, func(tx pgx.Tx, sc platform.Scope) error {
		var e error
		out, e = storefrontdomains.Detach(context.Background(), tx, sc, token, origin)
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

	// Slug + fallback + suffix (the single writer control.assign_store_handle).
	var slug string
	if err := o.QueryRow(ctx, `SELECT control.assign_store_handle('Acme Shop', $1::uuid)`, randomUUID()).Scan(&slug); err != nil || slug != "acme-shop" {
		t.Fatalf("assign_store_handle slug = %q, %v", slug, err)
	}
	id := randomUUID()
	var fallback string
	if err := o.QueryRow(ctx, `SELECT control.assign_store_handle('店铺名', $1::uuid)`, id).Scan(&fallback); err != nil || fallback != "store-"+strings.ReplaceAll(id, "-", "")[:8] {
		t.Fatalf("assign_store_handle fallback = %q, %v (want store-<id8>)", fallback, err)
	}
	// Uniqueness backstop + suffix: an occupied handle takes -2 for a different id.
	taken := randomUUID()
	if err := o.QueryRow(ctx, `SELECT control.assign_store_handle('Taken Name', $1::uuid)`, taken).Scan(&slug); err != nil || slug != "taken-name" {
		t.Fatalf("first assign = %q, %v", slug, err)
	}
	mustExec(t, o, `INSERT INTO control.stores(tenant_id,id,name,currency,handle) VALUES($1,$2,'Taken Name','USD','taken-name')`, s.f.tenant, taken)
	if err := o.QueryRow(ctx, `SELECT control.assign_store_handle('Taken Name', $1::uuid)`, randomUUID()).Scan(&slug); err != nil || slug != "taken-name-2" {
		t.Fatalf("suffix assign = %q, %v (want taken-name-2)", slug, err)
	}
	// The unique index is the concurrency backstop: a raw duplicate handle is a 23505, not silently accepted.
	if err := o.QueryRow(ctx, `INSERT INTO control.stores(tenant_id,id,name,currency,handle) VALUES($1,$2,'Dup','USD','taken-name') RETURNING id`, s.f.tenant, randomUUID()).Scan(&id); pgCode(err) != "23505" {
		t.Fatalf("duplicate handle: err=%v, want 23505", err)
	}
}

func TestStoreDomainsSDW02PlatformRowActive(t *testing.T) {
	s := sdSetup(t)
	ctx := context.Background()
	o := s.b.owner

	handle := handleOf(t, o, s.f.store)
	if handle == "" || !strings.HasPrefix(handle, "t06-go-store") {
		t.Fatalf("auto handle = %q, want the t06-go-store slug", handle)
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
	// A suspended row refuses a re-request of the same host by its owner (PT409 domain_suspended).
	if _, err := s.request(s.f.token, s.f.store, host); !errors.Is(err, storefrontdomains.ErrDomainSuspended) {
		t.Fatalf("re-request of SUSPENDED: %v, want ErrDomainSuspended", err)
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

// sdFns is the full 0106 function surface: function -> owner, whether it is SECURITY DEFINER (the three pure-SQL
// handle helpers are not: they are IMMUTABLE and touch no table), and the non-owner EXECUTE grantees. The owner
// always keeps EXECUTE (materialised into the ACL by every REVOKE ALL ... FROM PUBLIC), so it is asserted by the
// test itself rather than listed here; nil means the function is owner-only.
var sdFns = []struct {
	fn, owner string
	secdef    bool
	exec      []string
}{
	{"control.store_handle_reserved(text)", "commerce_identity_writer", false, nil},
	{"control.slug_store_handle(text)", "commerce_identity_writer", false, nil},
	{"control.store_handle_valid(text)", "commerce_identity_writer", false, nil},
	{"control.assign_store_handle(text,uuid)", "commerce_identity_writer", true, nil},
	{"control.stores_handle_default()", "commerce_identity_writer", true, nil},
	{"control.suggest_store_handle(text,text)", "commerce_identity_writer", true, []string{"commerce_identity"}},
	{"control.ensure_store_platform_domain(uuid,text)", "commerce_storefront_writer", true, []string{"commerce_identity", "commerce_runtime", "commerce_storefront_registrar"}},
	{"control.backfill_platform_domains(text)", "commerce_storefront_writer", true, []string{"commerce_storefront_registrar"}},
	{"control.request_merchant_domain(bytea,uuid,text,text,text)", "commerce_storefront_writer", true, []string{"commerce_runtime"}},
	{"control.read_store_domains(bytea,uuid)", "commerce_storefront_writer", true, []string{"commerce_runtime"}},
	{"control.suspend_merchant_domain(bytea,uuid,text)", "commerce_storefront_writer", true, []string{"commerce_runtime"}},
	{"control.detach_merchant_domain(bytea,uuid,text)", "commerce_storefront_writer", true, []string{"commerce_runtime"}},
	{"control.store_domain_dns_advance(uuid,boolean)", "commerce_storefront_writer", true, []string{"commerce_storefront_registrar"}},
	{"control.store_domain_tls_complete(uuid,timestamptz)", "commerce_storefront_writer", true, []string{"commerce_storefront_registrar"}},
	{"control.next_store_domain_dns_check()", "commerce_storefront_writer", true, []string{"commerce_storefront_registrar"}},
	{"control.next_store_domain_tls_probe()", "commerce_storefront_writer", true, []string{"commerce_storefront_registrar"}},
	{"control.resolve_storefront_ask(text)", "commerce_storefront_writer", true, []string{"commerce_runtime"}},
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
	// The registrar and runtime authorities have no direct write to the domain rows (they reach them through definers).
	for _, role := range []string{"commerce_storefront_registrar", "commerce_runtime"} {
		var access bool
		if err := o.QueryRow(ctx, `SELECT has_table_privilege($1,'control.storefront_domains','INSERT,UPDATE,DELETE') OR has_any_column_privilege($1,'control.storefront_domains','INSERT,UPDATE')`, role).Scan(&access); err != nil || access {
			t.Fatalf("%s has direct write on storefront_domains (err=%v)", role, err)
		}
	}
	// The 0081 audit policy still admits its original six actions plus the three merchant self-service ones (same policy).
	var withCheck string
	if err := o.QueryRow(ctx, `SELECT pg_get_expr(polwithcheck,polrelid) FROM pg_policy WHERE polname='storefront_writer_audit_insert' AND polrelid='ops.audit_events'::regclass`).Scan(&withCheck); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"merchant.domain_requested", "merchant.domain_suspended", "merchant.domain_detached", "operator.domain_bound"} {
		if !strings.Contains(withCheck, "'"+action+"'") {
			t.Fatalf("audit policy lost %s: %s", action, withCheck)
		}
	}
}
