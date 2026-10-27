// Purpose: OC-12 real-PostgreSQL gates of the A8 `unread` filter (output/oc-12/UNREAD.md, contracts/live-console-v1.md §3.6):
// the unread definition (last_inbound_seq > read_seq, store-level, cleared only by inbox:reply, re-opened by a newer inbound
// seq) and cross-tenant / cross-store isolation of the filtered read through the real service path (platform.WithScope +
// inbox.Service.ListConversations), and the HTTP A8 route (filter=unread 200, unknown filter 400 invalid_filter).
// Depends on: the disposable lc_foundation_test database (LC_TEST_DATABASE_ALLOWED=1, LC_TEST_DATABASE_URL; otherwise every
// test is a SKIP = NOT_RUN, never a PASS), livecommerce/migrations (Apply), internal/platform (OpenPool, WithScope),
// internal/inbox (LoadKeyring, NewService, ListConversations), internal/httpapi (NewHandler, A8 route),
// social.list_conversations / inbox.mark_read definers.
// Used by: go test ./internal/inbox (CI shard:unit via scripts/dev/test-local.sh). Fixtures are synthetic UUIDs only; no buyer PII.
package inbox_test

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/httpapi"
	"livecommerce/internal/inbox"
	"livecommerce/internal/platform"
	"livecommerce/migrations"
)

type unreadEnv struct {
	owner   *pgxpool.Pool // migration owner (superuser in the disposable DB): fixtures only
	runtime *pgxpool.Pool // a real commerce_runtime login, as cmd/api uses (platform.OpenPool)
	svc     *inbox.Service
}

var (
	unreadOnce sync.Once
	unreadVal  *unreadEnv
	unreadSkip string
	unreadErr  error
)

func unreadFixture(t *testing.T) *unreadEnv {
	t.Helper()
	unreadOnce.Do(func() { unreadVal, unreadSkip, unreadErr = newUnreadEnv() })
	if unreadSkip != "" {
		t.Skip(unreadSkip)
	}
	if unreadErr != nil {
		t.Fatal(unreadErr)
	}
	return unreadVal
}

func newUnreadEnv() (*unreadEnv, string, error) {
	if os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" || os.Getenv("LC_TEST_DATABASE_URL") == "" {
		return nil, "LC_TEST_DATABASE_ALLOWED=1 and LC_TEST_DATABASE_URL are required; OC-12 unread gates NOT_RUN", nil
	}
	cfg, err := pgxpool.ParseConfig(os.Getenv("LC_TEST_DATABASE_URL"))
	if err != nil {
		return nil, "", errors.New("LC_TEST_DATABASE_URL is invalid")
	}
	if cfg.ConnConfig.Host != "127.0.0.1" || cfg.ConnConfig.Database != "lc_foundation_test" {
		return nil, "", errors.New("refusing database outside 127.0.0.1/lc_foundation_test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	owner, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, "", err
	}
	if err := migrations.Apply(ctx, owner); err != nil {
		return nil, "", fmt.Errorf("apply migrations: %w", err)
	}
	role, secret := "oc12_"+hex.EncodeToString(rnd(10)), hex.EncodeToString(rnd(32))
	if _, err := owner.Exec(ctx, `CREATE ROLE `+pgx.Identifier{role}.Sanitize()+` LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE IN ROLE commerce_runtime PASSWORD '`+secret+`'`); err != nil {
		return nil, "", err
	}
	u, _ := url.Parse(os.Getenv("LC_TEST_DATABASE_URL"))
	u.User = url.UserPassword(role, secret)
	runtime, err := platform.OpenPool(ctx, u.String())
	if err != nil {
		return nil, "", err
	}
	// A throwaway payload key: no message bodies are seeded, so it is never used to open anything.
	keysJSON := `{"keys":[{"id":"oc12-test","key_base64":"` + base64.StdEncoding.EncodeToString(rnd(32)) + `"}]}`
	keys, err := inbox.LoadKeyring(func(k string) string {
		switch k {
		case "COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID":
			return "oc12-test"
		case "COMMERCE_META_PAYLOAD_KEYS_JSON":
			return keysJSON
		}
		return ""
	})
	if err != nil {
		return nil, "", err
	}
	svc, err := inbox.NewService(keys)
	if err != nil {
		return nil, "", err
	}
	return &unreadEnv{owner: owner, runtime: runtime, svc: svc}, "", nil
}

func rnd(n int) []byte { b := make([]byte, n); _, _ = rand.Read(b); return b }

func newID() string {
	b := rnd(16)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func exec(t *testing.T, p *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := p.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("fixture %q: %v", strings.Fields(sql)[0], err)
	}
}

// tenantWithStores creates a fresh tenant with n stores, so list pages are never displaced by other tests' rows.
func tenantWithStores(t *testing.T, e *unreadEnv, n int) (string, []string) {
	t.Helper()
	tenant := newID()
	exec(t, e.owner, `INSERT INTO control.tenants(id,name) VALUES($1,'oc12-tenant')`, tenant)
	stores := make([]string, n)
	for i := range stores {
		stores[i] = newID()
		exec(t, e.owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'oc12-store','USD')`, tenant, stores[i])
	}
	return tenant, stores
}

// staff registers a store member with store:read plus perms and returns a bearer token (sessions store only its hash).
func staff(t *testing.T, e *unreadEnv, tenant, store string, perms ...string) string {
	t.Helper()
	principal, tok := newID(), base64.RawURLEncoding.EncodeToString(rnd(32))
	exec(t, e.owner, `INSERT INTO identity.principals(id) VALUES ($1)`, principal)
	exec(t, e.owner, `INSERT INTO identity.memberships(tenant_id,principal_id) VALUES ($1,$2)`, tenant, principal)
	for _, perm := range append([]string{"store:read"}, perms...) {
		exec(t, e.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES ($1,$2,$3,$4)`, tenant, store, principal, perm)
	}
	h := sha256.Sum256([]byte(tok))
	exec(t, e.owner, `INSERT INTO identity.sessions(id,token_hash,principal_id,audience,expires_at) VALUES ($1,$2,$3,'merchant',$4)`,
		newID(), h[:], principal, time.Now().Add(time.Hour))
	return tok
}

// conversation seeds a conversation plus its state row. inbound < 0 means no buyer message yet (last_inbound_seq NULL).
func conversation(t *testing.T, e *unreadEnv, tenant, store string, inbound, read int64) string {
	t.Helper()
	conv := newID()
	peer := strings.Repeat(strings.ReplaceAll(conv, "-", ""), 2)
	exec(t, e.owner, `INSERT INTO social.conversations(id,tenant_id,store_id,app_id,object,asset_id,peer_key) VALUES ($1,$2,$3,'123456789','page','1234567890',$4)`,
		conv, tenant, store, peer)
	exec(t, e.owner, `INSERT INTO inbox.conversation_state(tenant_id,store_id,conversation_id,read_seq) VALUES ($1,$2,$3,$4)`, tenant, store, conv, read)
	if inbound > 0 {
		setInbound(t, e, conv, inbound)
	}
	return conv
}

// setInbound stands in for inbox.advance_conversation_state (the social.messages trigger): it raises last_inbound_seq/at only.
func setInbound(t *testing.T, e *unreadEnv, conv string, seq int64) {
	t.Helper()
	exec(t, e.owner, `UPDATE inbox.conversation_state SET last_inbound_seq=$2, last_inbound_at=clock_timestamp() WHERE conversation_id=$1`, conv, seq)
}

// listUnread runs A8 filter=unread through the real service path. The only scope inputs are the bearer token and the
// path store id, exactly what httpapi passes; tenant comes from the session.
func listUnread(t *testing.T, e *unreadEnv, token, store string) (inbox.ConversationList, error) {
	t.Helper()
	var out inbox.ConversationList
	err := platform.WithScope(context.Background(), e.runtime, token, store, "inbox:read", func(tx pgx.Tx, _ platform.Scope) error {
		var err error
		out, err = e.svc.ListConversations(context.Background(), tx, inbox.ListRequest{Filter: "unread", Limit: 50})
		return err
	})
	return out, err
}

func ids(t *testing.T, l inbox.ConversationList) []string {
	t.Helper()
	var out []string
	for _, it := range l.Items {
		if it.BundleOnly || !it.Unread {
			t.Fatalf("filter=unread returned a non-unread item: %+v", it)
		}
		out = append(out, it.ConversationID)
	}
	slices.Sort(out)
	return out
}

func markRead(t *testing.T, e *unreadEnv, token, store, conv string, seq int64) int64 {
	t.Helper()
	var got int64
	err := platform.WithScope(context.Background(), e.runtime, token, store, "inbox:read", func(tx pgx.Tx, _ platform.Scope) error {
		return tx.QueryRow(context.Background(), `SELECT inbox.mark_read($1,$2)`, conv, seq).Scan(&got)
	})
	if err != nil {
		t.Fatalf("mark_read: %v", err)
	}
	return got
}

// UR01: unread = COALESCE(last_inbound_seq,0) > read_seq; no-inbound and fully-read conversations are excluded;
// unread_total counts exactly the filtered set of this store.
func TestOC12UnreadDefinition(t *testing.T) {
	e := unreadFixture(t)
	tenant, stores := tenantWithStores(t, e, 1)
	store := stores[0]
	reader := staff(t, e, tenant, store, "inbox:read")
	unread := conversation(t, e, tenant, store, 5, 2)
	conversation(t, e, tenant, store, 5, 5)  // read up to the newest inbound
	conversation(t, e, tenant, store, -1, 0) // no buyer message yet

	got, err := listUnread(t, e, reader, store)
	if err != nil {
		t.Fatalf("A8 filter=unread: %v", err)
	}
	if want := []string{unread}; !slices.Equal(ids(t, got), want) {
		t.Fatalf("filter=unread ids=%v, want %v", ids(t, got), want)
	}
	if got.UnreadTotal != 1 {
		t.Fatalf("unread_total=%d, want 1", got.UnreadTotal)
	}
}

// UR02: the marker is store-level and only inbox:reply clears it; a newer inbound seq makes the thread unread again.
func TestOC12UnreadStoreLevelAndReset(t *testing.T) {
	e := unreadFixture(t)
	tenant, stores := tenantWithStores(t, e, 1)
	store := stores[0]
	reader := staff(t, e, tenant, store, "inbox:read")
	replier := staff(t, e, tenant, store, "inbox:read", "inbox:reply")
	other := staff(t, e, tenant, store, "inbox:read", "inbox:reply")
	conv := conversation(t, e, tenant, store, 4, 0)

	if got := markRead(t, e, reader, store, conv, 4); got != 0 {
		t.Fatalf("inbox:read-only mark_read=%d, want 0 (unchanged)", got)
	}
	if l, err := listUnread(t, e, other, store); err != nil || !slices.Equal(ids(t, l), []string{conv}) {
		t.Fatalf("after read-only mark: unread=%v err=%v, want [%s]", l.Items, err, conv)
	}
	if got := markRead(t, e, replier, store, conv, 4); got != 4 {
		t.Fatalf("inbox:reply mark_read=%d, want 4", got)
	}
	// Store-level: another staff member no longer sees it as unread.
	if l, err := listUnread(t, e, other, store); err != nil || len(l.Items) != 0 || l.UnreadTotal != 0 {
		t.Fatalf("after replier mark: unread=%v total=%d err=%v, want none", l.Items, l.UnreadTotal, err)
	}
	setInbound(t, e, conv, 5) // a new buyer message
	if l, err := listUnread(t, e, other, store); err != nil || !slices.Equal(ids(t, l), []string{conv}) || l.UnreadTotal != 1 {
		t.Fatalf("after new inbound: unread=%v total=%d err=%v, want [%s] total 1", l.Items, l.UnreadTotal, err, conv)
	}
}

// UR03 (red first): the unread read never crosses tenant or store. Tenant A store 1 sees only its own unread thread;
// tenant B's staff sees none of A's, and naming A's store id in the request is refused before any read.
func TestOC12UnreadCrossTenantIsolation(t *testing.T) {
	e := unreadFixture(t)
	tenantA, storesA := tenantWithStores(t, e, 2)
	tenantB, storesB := tenantWithStores(t, e, 1)
	a1, a2, b1 := storesA[0], storesA[1], storesB[0]
	readerA1 := staff(t, e, tenantA, a1, "inbox:read")
	readerB1 := staff(t, e, tenantB, b1, "inbox:read")
	convA1 := conversation(t, e, tenantA, a1, 3, 0)
	convA2 := conversation(t, e, tenantA, a2, 3, 0)
	convB1 := conversation(t, e, tenantB, b1, 3, 0)

	la, err := listUnread(t, e, readerA1, a1)
	if err != nil {
		t.Fatalf("tenant A store 1 filter=unread: %v", err)
	}
	if got := ids(t, la); !slices.Equal(got, []string{convA1}) || la.UnreadTotal != 1 {
		t.Fatalf("tenant A store 1 unread=%v total=%d, want only %s (not store 2 %s, not tenant B %s)", got, la.UnreadTotal, convA1, convA2, convB1)
	}
	lb, err := listUnread(t, e, readerB1, b1)
	if err != nil {
		t.Fatalf("tenant B filter=unread: %v", err)
	}
	if got := ids(t, lb); !slices.Equal(got, []string{convB1}) || lb.UnreadTotal != 1 {
		t.Fatalf("tenant B unread=%v total=%d, want only %s", got, lb.UnreadTotal, convB1)
	}
	for _, foreign := range []string{a1, a2} {
		ran := false
		err := platform.WithScope(context.Background(), e.runtime, readerB1, foreign, "inbox:read", func(pgx.Tx, platform.Scope) error {
			ran = true
			return nil
		})
		if err == nil || ran {
			t.Fatalf("tenant B token with tenant A store %s: err=%v ran=%t, want refusal before the callback", foreign, err, ran)
		}
	}
}

// UR04: the HTTP A8 route (internal/httpapi, as cmd/api mounts it) accepts filter=unread with 200 and only this store's
// unread rows, rejects an unknown filter with 400 invalid_filter, and a foreign tenant's token naming this store gets no 200.
func TestOC12UnreadHTTP(t *testing.T) {
	e := unreadFixture(t)
	tenant, stores := tenantWithStores(t, e, 1)
	store := stores[0]
	reader := staff(t, e, tenant, store, "inbox:read")
	conv := conversation(t, e, tenant, store, 2, 0)
	conversation(t, e, tenant, store, 2, 2)
	tenantB, storesB := tenantWithStores(t, e, 1)
	foreign := staff(t, e, tenantB, storesB[0], "inbox:read")
	h := httpapi.NewHandler(e.runtime, httpapi.Options{Inbox: e.svc})
	get := func(token, query string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/stores/"+store+"/inbox/conversations?"+query, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		res := httptest.NewRecorder()
		h.ServeHTTP(res, req)
		return res
	}

	res := get(reader, "filter=unread")
	if res.Code != http.StatusOK {
		t.Fatalf("GET filter=unread status=%d body=%s, want 200", res.Code, res.Body.String())
	}
	var body struct {
		Items []struct {
			ConversationID string `json:"conversation_id"`
			Unread         bool   `json:"unread"`
		} `json:"items"`
		UnreadTotal int64 `json:"unread_total"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode A8 body: %v", err)
	}
	if len(body.Items) != 1 || body.Items[0].ConversationID != conv || !body.Items[0].Unread || body.UnreadTotal != 1 {
		t.Fatalf("GET filter=unread items=%+v total=%d, want only %s unread, total 1", body.Items, body.UnreadTotal, conv)
	}

	res = get(reader, "filter=bogus")
	if res.Code != http.StatusBadRequest || !strings.Contains(res.Body.String(), `"invalid_filter"`) {
		t.Fatalf("GET filter=bogus status=%d body=%s, want 400 invalid_filter", res.Code, res.Body.String())
	}

	if res = get(foreign, "filter=unread"); res.Code == http.StatusOK || strings.Contains(res.Body.String(), conv) {
		t.Fatalf("foreign tenant token on this store: status=%d body=%s, want refusal without data", res.Code, res.Body.String())
	}
}
