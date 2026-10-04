// service_test.go: pure-logic unit tests (no database) for the merchant domain service methods and helpers. They fake
// the one-row definer answers (pgx.Tx/Querier) to prove input validation at the trust boundary, strict result decoding,
// the under-base refusal and SQLSTATE mapping. The definers themselves (lifecycle, compare-and-set, grants) are proven
// against real PG by the gate suite.

package storefrontdomains

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

const (
	testTenant    = "7d4a2f1c-9b3e-4c8a-9f12-6a5b7c8d9e0f"
	testStore     = "3f1b0c9e-5a77-4d1e-9d2a-0a7f4c2b9e11"
	testPrincipal = "9a2f6d1c-3b44-4c0a-8f55-1e6d7c8b9a22"
	testDomain    = "2c8e4f0a-6b7d-4e1f-8a3c-5d9e0f1a2b3c"
	idemKey       = "store-domain-test-key-0001"
)

// --- fakes shared with verify_test.go ---

type fakeRow struct {
	raw []byte
	str *string
	err error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	switch d := dest[0].(type) {
	case *[]byte:
		*d = r.raw
	case **string:
		*d = r.str
	default:
		return fmt.Errorf("unsupported scan dest %T", dest[0])
	}
	return nil
}

// fakeRows is a minimal pgx.Rows for the worker batch reads.
type fakeRows struct {
	rows    [][]any
	idx     int
	err     error
	scanErr error
}

func (r *fakeRows) Close()                                       {}
func (r *fakeRows) Err() error                                   { return r.err }
func (r *fakeRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }

func (r *fakeRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *fakeRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	vals := r.rows[r.idx-1]
	for i, d := range dest {
		if i >= len(vals) {
			return fmt.Errorf("scan: too many destinations")
		}
		switch p := d.(type) {
		case *string:
			*p, _ = vals[i].(string)
		case *int:
			*p, _ = vals[i].(int)
		case *time.Time:
			*p, _ = vals[i].(time.Time)
		case **time.Time:
			*p, _ = vals[i].(*time.Time)
		default:
			return fmt.Errorf("scan: unsupported dest %T", d)
		}
	}
	return nil
}

func (r *fakeRows) Values() ([]any, error) { return nil, nil }
func (r *fakeRows) RawValues() [][]byte    { return nil }
func (r *fakeRows) Conn() *pgx.Conn        { return nil }
func (r *fakeRows) TypeMap() *pgtype.Map   { return nil }

// fakeTx satisfies both pgx.Tx (service methods) and Querier (worker/primary calls). The embedded pgx.Tx is nil, so
// any method a test does not override panics — the tests only ever call QueryRow and Query.
type fakeTx struct {
	pgx.Tx
	row        fakeRow
	rowSeq     []fakeRow
	rowFn      func(args []any) fakeRow
	rows       *fakeRows
	rowsSeq    []*fakeRows
	args       []any
	callArgs   [][]any
	calls      int
	queryCalls int
}

func (f *fakeTx) QueryRow(_ context.Context, _ string, args ...any) pgx.Row {
	f.args = args
	f.callArgs = append(f.callArgs, args)
	if f.rowFn != nil {
		f.calls++
		return f.rowFn(args)
	}
	if f.rowSeq != nil {
		if f.calls >= len(f.rowSeq) {
			return fakeRow{err: errors.New("unexpected extra QueryRow")}
		}
		r := f.rowSeq[f.calls]
		f.calls++
		return r
	}
	f.calls++
	return f.row
}

func (f *fakeTx) Query(_ context.Context, _ string, args ...any) (pgx.Rows, error) {
	f.args = args
	if f.rowsSeq != nil {
		if f.queryCalls >= len(f.rowsSeq) {
			return nil, errors.New("unexpected extra Query")
		}
		r := f.rowsSeq[f.queryCalls]
		f.queryCalls++
		return r, nil
	}
	f.queryCalls++
	if f.rows == nil {
		return nil, errors.New("no rows configured")
	}
	return f.rows, nil
}

func sptr(s string) *string { return &s }

func pgErr(code, msg string) error { return &pgconn.PgError{Code: code, Message: msg} }

func validScope() platform.Scope {
	return platform.Scope{TenantID: testTenant, StoreID: testStore, PrincipalID: testPrincipal, Revision: 1}
}

func TestRequestInputsFailBeforeTheDatabase(t *testing.T) {
	scope := validScope()
	token := strings.Repeat("t", 43)
	for name, tc := range map[string]struct {
		tx    pgx.Tx
		scope platform.Scope
		token string
		host  string
		base  string
		want  error
	}{
		"nil tx":        {nil, scope, token, "shop.example.com", "example.com", command.ErrInvalid},
		"bad tenant":    {&fakeTx{}, platform.Scope{TenantID: "bad", StoreID: testStore, PrincipalID: testPrincipal, Revision: 1}, token, "shop.example.com", "example.com", command.ErrInvalid},
		"bad principal": {&fakeTx{}, platform.Scope{TenantID: testTenant, StoreID: testStore, PrincipalID: "bad", Revision: 1}, token, "shop.example.com", "example.com", command.ErrInvalid},
		"zero revision": {&fakeTx{}, platform.Scope{TenantID: testTenant, StoreID: testStore, PrincipalID: testPrincipal}, token, "shop.example.com", "example.com", command.ErrInvalid},
		"short token":   {&fakeTx{}, scope, "short", "shop.example.com", "example.com", command.ErrInvalid},
		"bad hostname":  {&fakeTx{}, scope, token, "Shop.Example.com:443", "example.com", command.ErrInvalid},
		"bad base":      {&fakeTx{}, scope, token, "shop.example.com", "localhost", command.ErrInvalid},
		"under base":    {&fakeTx{}, scope, token, "shop.example.com", "example.com", ErrReservedHostname},
		"exact base":    {&fakeTx{}, scope, token, "example.com", "example.com", ErrReservedHostname},
	} {
		_, err := Request(context.Background(), tc.tx, tc.scope, tc.token, idemKey, tc.host, tc.base, nil)
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
	}
	if _, err := Request(nil, &fakeTx{}, scope, token, idemKey, "shop.example.com", "example.com", nil); !errors.Is(err, command.ErrInvalid) {
		t.Errorf("nil context: %v", err)
	}
}

func TestRequestSuccess(t *testing.T) {
	scope := validScope()
	token := strings.Repeat("t", 43)
	const host = "shop.example.net"
	q := &fakeTx{rowFn: func(args []any) fakeRow {
		verification := args[4].(string)
		return fakeRow{raw: []byte(`{"domain_id":"` + testDomain + `","version":1,"state":"REQUESTED","origin":"https://` + host + `",` +
			`"dns":{"txt_name":"_lc-verify.` + host + `","txt_value":"` + verification + `","cname_target":"stores.example.com","apex":false}}`)}
	}}
	var out RequestResult
	if err := requestEntry(context.Background(), q, scope, token, host, "example.com", nil, &out); err != nil {
		t.Fatalf("requestEntry: %v", err)
	}
	if out.DomainID != testDomain || out.Version != 1 || out.State != "REQUESTED" || out.Origin != "https://"+host {
		t.Fatalf("Request result = %+v", out)
	}
	if out.DNS.TXTName != "_lc-verify."+host || out.DNS.CNAMETarget != "stores.example.com" || out.DNS.Apex || len(out.DNS.EdgeAddresses) != 0 {
		t.Fatalf("DNS instructions = %+v", out.DNS)
	}
	if out.DNS.TXTValue != q.args[4].(string) {
		t.Fatalf("token not echoed: %q vs %q", out.DNS.TXTValue, q.args[4])
	}
	if q.args[2] != host || q.args[3] != "example.com" {
		t.Fatalf("definer args = %v", q.args)
	}
}

func TestRequestEntryAttachesEdgeForApex(t *testing.T) {
	scope := validScope()
	token := strings.Repeat("t", 43)
	const host = "example.net"
	edge := []string{"203.0.113.10", "2001:db8::10"}
	q := &fakeTx{rowFn: func(args []any) fakeRow {
		verification := args[4].(string)
		return fakeRow{raw: []byte(`{"domain_id":"` + testDomain + `","version":1,"state":"REQUESTED","origin":"https://` + host + `",` +
			`"dns":{"txt_name":"_lc-verify.` + host + `","txt_value":"` + verification + `","cname_target":"stores.example.com","apex":true}}`)}
	}}
	var out RequestResult
	if err := requestEntry(context.Background(), q, scope, token, host, "example.com", edge, &out); err != nil {
		t.Fatalf("requestEntry: %v", err)
	}
	if !out.DNS.Apex || len(out.DNS.EdgeAddresses) != 2 || out.DNS.EdgeAddresses[0] != edge[0] || out.DNS.EdgeAddresses[1] != edge[1] {
		t.Fatalf("apex edge = %+v, want %v", out.DNS, edge)
	}
}

func TestRequestRejectsBadDefinerAnswer(t *testing.T) {
	scope := validScope()
	token := strings.Repeat("t", 43)
	const host = "shop.example.net"
	for name, raw := range map[string]string{
		"wrong txt name": `{"domain_id":"` + testDomain + `","version":1,"state":"REQUESTED","origin":"https://` + host + `","dns":{"txt_name":"_lc-verify.other.com","txt_value":"x","cname_target":"stores.example.com","apex":false}}`,
		"unknown field":  `{"domain_id":"` + testDomain + `","version":1,"state":"REQUESTED","origin":"https://` + host + `","extra":1}`,
		"wrong state":    `{"domain_id":"` + testDomain + `","version":1,"state":"ACTIVE","origin":"https://` + host + `","dns":{"txt_name":"_lc-verify.` + host + `","txt_value":"x","cname_target":"stores.example.com","apex":false}}`,
		"bad domain id":  `{"domain_id":"x","version":1,"state":"REQUESTED","origin":"https://` + host + `","dns":{"txt_name":"_lc-verify.` + host + `","txt_value":"x","cname_target":"stores.example.com","apex":false}}`,
	} {
		q := &fakeTx{row: fakeRow{raw: []byte(raw)}}
		var out RequestResult
		if err := requestEntry(context.Background(), q, scope, token, host, "example.com", nil, &out); !errors.Is(err, ErrUnavailable) {
			t.Errorf("%s: err = %v, want ErrUnavailable", name, err)
		}
	}
}

func TestReadDecodesAndValidates(t *testing.T) {
	scope := validScope()
	token := strings.Repeat("t", 43)
	goodToken := strings.Repeat("A", 43)
	ok := &fakeTx{row: fakeRow{raw: []byte(`{"domains":[` +
		`{"origin":"https://shop.example.com","state":"ACTIVE","version":4,"kind":"platform","token":null,"verify_deadline":null,"serving":true},` +
		`{"origin":"https://old.example.com","state":"REQUESTED","version":2,"kind":"custom","token":"` + goodToken + `","verify_deadline":"2027-01-01T00:00:00Z","serving":false}]}`)}}
	out, err := Read(context.Background(), ok, scope, token)
	if err != nil || len(out.Domains) != 2 || !out.Domains[0].Serving || out.Domains[0].Token != nil {
		t.Fatalf("Read = %+v, %v", out, err)
	}
	if out.Domains[0].Kind != "platform" || out.Domains[1].Kind != "custom" {
		t.Fatalf("kinds = %q %q", out.Domains[0].Kind, out.Domains[1].Kind)
	}
	if out.Domains[1].Token == nil || *out.Domains[1].Token != goodToken {
		t.Fatalf("pending token = %+v", out.Domains[1].Token)
	}
	if ok.args[1] != testStore {
		t.Fatalf("definer store arg = %v", ok.args[1])
	}
	// an empty list is valid (never-published stores have no merchant domains)
	empty := &fakeTx{row: fakeRow{raw: []byte(`{"domains":[]}`)}}
	if out, err := Read(context.Background(), empty, scope, token); err != nil || len(out.Domains) != 0 {
		t.Fatalf("empty Read = %+v, %v", out, err)
	}
	for name, raw := range map[string]string{
		"bad token":  `{"domains":[{"origin":"https://shop.example.com","state":"REQUESTED","version":2,"kind":"custom","token":"short","verify_deadline":null,"serving":false}]}`,
		"bad origin": `{"domains":[{"origin":"http://shop.example.com","state":"REQUESTED","version":2,"kind":"custom","token":null,"verify_deadline":null,"serving":false}]}`,
		"bad kind":   `{"domains":[{"origin":"https://shop.example.com","state":"REQUESTED","version":2,"kind":"other","token":null,"verify_deadline":null,"serving":false}]}`,
		"nil list":   `{"domains":null}`,
		"unknown":    `{"domains":[],"extra":1}`,
	} {
		q := &fakeTx{row: fakeRow{raw: []byte(raw)}}
		if _, err := Read(context.Background(), q, scope, token); !errors.Is(err, ErrUnavailable) {
			t.Errorf("%s: err = %v, want ErrUnavailable", name, err)
		}
	}
}

func TestSuspendDetach(t *testing.T) {
	scope := validScope()
	token := strings.Repeat("t", 43)
	origin := "https://shop.example.com"

	sus := &fakeTx{row: fakeRow{raw: []byte(`{"domain_id":"` + testDomain + `","version":3,"state":"SUSPENDED","changed":true}`)}}
	var out MoveResult
	if err := moveEntry(context.Background(), sus, scope, token, origin, `SELECT control.suspend_merchant_domain($1,$2::uuid,$3)`, "SUSPENDED", &out); err != nil || !out.Changed || out.Version != 3 || out.State != "SUSPENDED" {
		t.Fatalf("Suspend = %+v, %v", out, err)
	}
	if sus.args[2] != origin {
		t.Fatalf("suspend origin arg = %v", sus.args[2])
	}

	det := &fakeTx{row: fakeRow{raw: []byte(`{"domain_id":"` + testDomain + `","version":4,"state":"DETACHED","changed":true}`)}}
	if err := moveEntry(context.Background(), det, scope, token, origin, `SELECT control.detach_merchant_domain($1,$2::uuid,$3)`, "DETACHED", &out); err != nil || out.State != "DETACHED" {
		t.Fatalf("Detach = %+v, %v", out, err)
	}

	// a definer answering the wrong target state must not be reported as the requested one
	if err := moveEntry(context.Background(), det, scope, token, origin, `SELECT control.suspend_merchant_domain($1,$2::uuid,$3)`, "SUSPENDED", &out); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("suspend with DETACHED answer: %v", err)
	}

	// invalid origins are refused before any command.Run/definer work (public Suspend/Detach, no Exec needed).
	for _, bad := range []string{"", "http://shop.example.com", "https://localhost", "shop.example.com"} {
		if _, err := Suspend(context.Background(), &fakeTx{}, scope, token, idemKey, bad); !errors.Is(err, command.ErrInvalid) {
			t.Errorf("origin %q: %v", bad, err)
		}
		if _, err := Detach(context.Background(), &fakeTx{}, scope, token, idemKey, bad); !errors.Is(err, command.ErrInvalid) {
			t.Errorf("detach origin %q: %v", bad, err)
		}
	}
}

func TestPrimaryOrigin(t *testing.T) {
	q := &fakeTx{row: fakeRow{str: sptr("https://primary.example.com")}}
	if out, err := PrimaryOrigin(context.Background(), q, "https://shop.example.com"); err != nil || out != "https://primary.example.com" {
		t.Fatalf("PrimaryOrigin = %q, %v", out, err)
	}
	// nil means "already primary" -> no redirect
	none := &fakeTx{row: fakeRow{str: nil}}
	if out, err := PrimaryOrigin(context.Background(), none, "https://shop.example.com"); err != nil || out != "" {
		t.Fatalf("PrimaryOrigin nil = %q, %v", out, err)
	}
	for _, bad := range []string{"", "http://shop.example.com", "shop.example.com"} {
		if _, err := PrimaryOrigin(context.Background(), &fakeTx{}, bad); !errors.Is(err, command.ErrInvalid) {
			t.Errorf("origin %q: %v", bad, err)
		}
	}
	if _, err := PrimaryOrigin(context.Background(), nil, "https://shop.example.com"); !errors.Is(err, command.ErrInvalid) {
		t.Errorf("nil q: %v", err)
	}
	if _, err := PrimaryOrigin(nil, &fakeTx{}, "https://shop.example.com"); !errors.Is(err, command.ErrInvalid) {
		t.Errorf("nil ctx: %v", err)
	}
	// a definer answering a non-origin fails closed
	bad := &fakeTx{row: fakeRow{str: sptr("not-an-origin")}}
	if _, err := PrimaryOrigin(context.Background(), bad, "https://shop.example.com"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("bad primary: %v", err)
	}
}

func TestMapError(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want error
	}{
		{pgErr("PT400", "invalid"), command.ErrInvalid},
		{pgErr("22023", "numeric out of range"), command.ErrInvalid},
		{pgErr("23514", "check violation"), command.ErrInvalid},
		{pgErr("22P02", "invalid input"), command.ErrInvalid},
		{pgErr("PT401", "unauthorized"), platform.ErrUnauthorized},
		{pgErr("PT403", "forbidden"), platform.ErrForbidden},
		{pgErr("PT404", "not found"), platform.ErrScopeNotFound},
		{pgErr("PT409", "domain_active"), ErrDomainActive},
		{pgErr("PT409", "domain_suspended"), ErrDomainSuspended},
		{pgErr("PT409", "domain_detached"), ErrDomainDetached},
		{pgErr("PT409", "domain_owned_elsewhere"), ErrDomainOwnedElsewhere},
		{pgErr("PT409", "reserved hostname"), ErrReservedHostname},
		{pgErr("PT409", "base domain not configured"), ErrBaseDomainMissing},
		{pgErr("PT409", "platform_domain"), ErrPlatformDomain},
		{pgErr("PT409", "version_conflict"), command.ErrConflict},
		{pgErr("42501", "permission denied for function secret_name"), ErrUnavailable},
		{errors.New("dial tcp 10.0.0.1: connection refused password=hunter2"), ErrUnavailable},
		{context.DeadlineExceeded, context.DeadlineExceeded},
		{context.Canceled, context.Canceled},
	} {
		got := mapError(tc.err)
		if !errors.Is(got, tc.want) {
			t.Errorf("mapError(%v) = %v, want %v", tc.err, got, tc.want)
		}
		if got != nil && (strings.Contains(got.Error(), "hunter2") || strings.Contains(got.Error(), "secret_name")) {
			t.Errorf("driver text leaked: %v", got)
		}
	}
	// serialization/unique races pass through for the HTTP layer's retry/conflict mapping
	for _, code := range []string{"40001", "40P01", "55P03", "57014", "23505"} {
		raced := pgErr(code, "dup")
		if got := mapError(raced); !errors.Is(got, raced) {
			t.Errorf("%s not passed through: %v", code, got)
		}
	}
}

func TestValidHostname(t *testing.T) {
	for _, h := range []string{"shop.example.com", "www.example.com", "a.b.c.example.com", "xn--bcher-kva.example"} {
		if !validHostname(h) {
			t.Errorf("validHostname(%q) = false", h)
		}
	}
	for _, h := range []string{"", "Shop.Example.com", "shop.example.com:443", "shop.localhost", "商店.com", "-shop.example.com", "shop..example.com", strings.Repeat("a", 254)} {
		if validHostname(h) {
			t.Errorf("validHostname(%q) = true, want false", h)
		}
	}
}

func TestUnderBase(t *testing.T) {
	for _, pair := range [][2]string{
		{"example.com", "example.com"},
		{"shop.example.com", "example.com"},
		{"a.b.example.com", "example.com"},
	} {
		if !underBase(pair[0], pair[1]) {
			t.Errorf("underBase(%q, %q) = false", pair[0], pair[1])
		}
	}
	for _, pair := range [][2]string{
		{"example.org", "example.com"},
		{"notexample.com", "example.com"},
		{"example.com.evil.org", "example.com"},
	} {
		if underBase(pair[0], pair[1]) {
			t.Errorf("underBase(%q, %q) = true", pair[0], pair[1])
		}
	}
}

func TestValidToken(t *testing.T) {
	if !validToken(strings.Repeat("A", 43)) {
		t.Error("43-char base64url token rejected")
	}
	for _, bad := range []string{"", strings.Repeat("A", 42), strings.Repeat("A", 44), strings.Repeat("!", 43), strings.Repeat("A", 42) + "=="} {
		if validToken(bad) {
			t.Errorf("validToken(%q) = true", bad)
		}
	}
}

func TestRandomToken(t *testing.T) {
	a, b := randomToken(), randomToken()
	if a == b {
		t.Fatal("two tokens identical")
	}
	if !validToken(a) || !validToken(b) {
		t.Fatalf("randomToken not a valid token: %q %q", a, b)
	}
}
