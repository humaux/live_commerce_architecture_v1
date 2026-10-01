package foundation_test

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/integrations/meta"
)

const (
	miApp    = "123456789012345"
	miSecret = "meta-inbox-test-secret-only"
	miKeyID  = "test_key_v1"
)

type miTest struct {
	f                           *testFixture
	ingress, registrar, curator *pgxpool.Pool
	verifier                    *meta.Verifier
	handler                     http.Handler
	key                         []byte
}

func miRole(t *testing.T, f *testFixture, authority string) string {
	t.Helper()
	role := "mi_test_" + strings.ReplaceAll(randomUUID(), "-", "")
	password := hex.EncodeToString(randomBytes(24))
	identifier := pgx.Identifier{role}.Sanitize()
	mustExec(t, f.owner, `CREATE ROLE `+identifier+` LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION PASSWORD '`+password+`'`)
	mustExec(t, f.owner, `GRANT `+pgx.Identifier{authority}.Sanitize()+` TO `+identifier+` WITH INHERIT TRUE, SET FALSE`)
	t.Cleanup(func() {
		mustExec(t, f.owner, `REVOKE `+pgx.Identifier{authority}.Sanitize()+` FROM `+identifier)
		mustExec(t, f.owner, `DROP ROLE `+identifier)
	})
	return roleURL(t, f.databaseURL, role, password)
}

func miPool(t *testing.T, f *testFixture, role string) *pgxpool.Pool {
	t.Helper()
	p, err := pgxpool.New(context.Background(), miRole(t, f, role))
	if err != nil {
		t.Fatal("open isolated role pool", err)
	}
	t.Cleanup(p.Close)
	return p
}

func miSetup(t *testing.T) miTest {
	t.Helper()
	f := fixture(t)
	ctx := context.Background()
	ingress := miPool(t, f, "commerce_meta_ingress")
	registrar := miPool(t, f, "commerce_meta_registrar")
	curator := miPool(t, f, "commerce_meta_curator")
	key := randomBytes(32)
	keys, err := meta.NewPayloadKeyring(miKeyID, map[string][]byte{miKeyID: key})
	if err != nil {
		t.Fatal("test keyring", err)
	}
	inbox, err := meta.NewInbox(ctx, ingress, keys)
	if err != nil {
		t.Fatal("dedicated ingress rejected", err)
	}
	v, err := meta.NewVerifier(meta.Config{AppID: miApp, Object: "page", AppSecret: miSecret, VerifyToken: "meta-inbox-verify-token"})
	if err != nil {
		t.Fatal(err)
	}
	h, err := meta.NewInboxHandler(v, inbox)
	if err != nil {
		t.Fatal(err)
	}
	return miTest{f, ingress, registrar, curator, v, h, key}
}

func miAsset() string {
	return fmt.Sprintf("%d", binary.BigEndian.Uint64(randomBytes(8)))
}

func miBinding(t *testing.T, m miTest, asset, provider, tenant, store, principal string) string {
	t.Helper()
	id := randomUUID()
	if _, err := m.f.owner.Exec(context.Background(), `INSERT INTO integration.bindings(id,tenant_id,store_id,principal_id,provider,external_asset_id) VALUES($1,$2,$3,$4,$5,$6)`, id, tenant, store, principal, provider, asset); err != nil {
		t.Fatal("synthetic binding", err)
	}
	return id
}

func miRoute(t *testing.T, m miTest, asset, tenant, store, binding string) (string, int64) {
	t.Helper()
	proof := sha256.Sum256([]byte("synthetic proof " + asset))
	var id string
	var epoch int64
	err := m.registrar.QueryRow(context.Background(), `SELECT * FROM meta_inbox.activate_route($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, miApp, "page", asset, tenant, store, binding, int64(1), hex.EncodeToString(proof[:]), time.Now().Add(time.Hour), int64(0)).Scan(&id, &epoch)
	if err != nil {
		t.Fatal("synthetic registrar activation", err)
	}
	return id, epoch
}

func miMessage(asset, mid, text string) []byte {
	return []byte(fmt.Sprintf(`{"object":"page","entry":[{"id":%q,"time":123,"messaging":[{"sender":{"id":"4"},"recipient":{"id":%q},"message":{"mid":%q,"text":%q}}]}]}`, asset, asset, mid, text))
}

func miPost(t *testing.T, m miTest, raw []byte) (int, string) {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(miSecret))
	_, _ = mac.Write(raw)
	r := httptest.NewRequest(http.MethodPost, "/meta-webhook", bytes.NewReader(raw))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	w := httptest.NewRecorder()
	m.handler.ServeHTTP(w, r)
	return w.Code, w.Body.String()
}

func miStatus(t *testing.T, got int, body string, want int) {
	t.Helper()
	if got != want {
		t.Fatalf("HTTP status=%d want=%d", got, want)
	}
	if want == 200 && body != "EVENT_RECEIVED" {
		t.Fatal("wrong positive ACK")
	}
	if want != 200 && (strings.Contains(body, miSecret) || strings.Contains(body, "postgres") || strings.Contains(body, "SQLSTATE")) {
		t.Fatal("storage or secret leaked in error")
	}
}

func miSQLState(err error) string {
	var e *pgconn.PgError
	if !errors.As(err, &e) {
		return ""
	}
	return e.Code
}

func miDecrypt(t *testing.T, key []byte, class, id, app, object, bodyHash, eventKey, payloadHash, tenant, store, route string, epoch int64, keyID string, nonce, ciphertext []byte) []byte {
	t.Helper()
	aad, err := json.Marshal([]any{"livecommerce/meta-payload/v1", class, id, app, object, bodyHash, eventKey, payloadHash, tenant, store, route, epoch, keyID})
	if err != nil {
		t.Fatal(err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := aead.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		t.Fatal("independent stdlib decrypt rejected stored context")
	}
	var digest string
	if class == "raw" {
		digest = bodyHash
	} else {
		digest = payloadHash
	}
	sum := sha256.Sum256(plain)
	if hex.EncodeToString(sum[:]) != digest {
		t.Fatal("stored plaintext hash mismatch")
	}
	return plain
}

func miCount(t *testing.T, p *pgxpool.Pool, query string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := p.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal("count metadata", err)
	}
	return n
}

// MI01: registration is a trusted control-plane act; neither a merchant's
// binding nor changing current_user may manufacture ingress/route authority.
func TestMetaInboxRoleAndRouteAuthority(t *testing.T) {
	m := miSetup(t)
	ctx := context.Background()
	keys, err := meta.NewPayloadKeyring(miKeyID, map[string][]byte{miKeyID: m.key})
	if err != nil {
		t.Fatal(err)
	}
	for name, p := range map[string]*pgxpool.Pool{"migration owner": m.f.owner, "merchant runtime": m.f.runtime, "registrar": m.registrar, "curator": m.curator} {
		t.Run(name, func(t *testing.T) {
			if inbox, err := meta.NewInbox(ctx, p, keys); err == nil || inbox != nil {
				t.Fatal("unsafe login accepted as ingress")
			}
		})
	}
	for name, p := range map[string]*pgxpool.Pool{"ingress": m.ingress, "runtime": m.f.runtime, "curator": m.curator} {
		t.Run("route denied "+name, func(t *testing.T) {
			_, err := p.Exec(ctx, `SELECT * FROM meta_inbox.activate_route('123','page','1',$1,$2,$3,1,$4,clock_timestamp()+interval '1 hour',0)`, m.f.tenantA, m.f.storeA1, randomUUID(), strings.Repeat("a", 64))
			if miSQLState(err) != "42501" {
				t.Fatalf("route mutation SQLSTATE=%s, want 42501", miSQLState(err))
			}
		})
	}
	if _, err := m.f.runtime.Exec(ctx, `SET ROLE commerce_meta_ingress`); miSQLState(err) != "42501" {
		t.Fatalf("runtime SET ROLE ingress SQLSTATE=%s", miSQLState(err))
	}
	mixedDSN := miRole(t, m.f, "commerce_meta_ingress")
	mixedURL, err := url.Parse(mixedDSN)
	if err != nil {
		t.Fatal(err)
	}
	mixedRole := mixedURL.User.Username()
	if _, err = m.f.owner.Exec(ctx, `GRANT commerce_runtime TO `+pgx.Identifier{mixedRole}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { mustExec(t, m.f.owner, `REVOKE commerce_runtime FROM `+pgx.Identifier{mixedRole}.Sanitize()) })
	mixed, err := pgxpool.New(ctx, mixedDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mixed.Close)
	if inbox, err := meta.NewInbox(ctx, mixed, keys); err == nil || inbox != nil {
		t.Fatal("mixed ingress/runtime login admitted")
	}
	asset := miAsset()
	binding := miBinding(t, m, asset, "facebook", m.f.tenantA, m.f.storeA1, m.f.principalA)
	_, epoch := miRoute(t, m, asset, m.f.tenantA, m.f.storeA1, binding)
	if epoch != 1 {
		t.Fatalf("new route epoch=%d", epoch)
	}
	foreign := miBinding(t, m, asset, "facebook", m.f.tenantA, m.f.storeA2, m.f.principalA)
	proof := strings.Repeat("a", 64)
	_, err = m.registrar.Exec(ctx, `SELECT * FROM meta_inbox.activate_route($1,'page',$2,$3,$4,$5,1,$6,clock_timestamp()+interval '1 hour',$7)`, miApp, asset, m.f.tenantA, m.f.storeA2, foreign, proof, epoch)
	if miSQLState(err) != "PT409" {
		t.Fatalf("foreign self-asserted binding accepted: SQLSTATE=%s", miSQLState(err))
	}
	_, err = m.registrar.Exec(ctx, `SELECT * FROM meta_inbox.activate_route($1,'page',$2,$3,$4,$5,1,$6,clock_timestamp()+interval '1 hour',0)`, "888888", asset, m.f.tenantA, m.f.storeA2, foreign, proof)
	if miSQLState(err) != "PT409" {
		t.Fatalf("cross-app asset owner changed: SQLSTATE=%s", miSQLState(err))
	}
}

// MI03: a transaction may not publish an unfinalized batch even if the caller
// bypasses the Go service and omits every subsequent step.
func TestMetaInboxDeferredPartialBatchGuard(t *testing.T) {
	m := miSetup(t)
	ctx := context.Background()
	tx, err := m.ingress.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	bodyHash := strings.Repeat("a", 64)
	var id string
	var replay bool
	if err = tx.QueryRow(ctx, `SELECT * FROM meta_inbox.begin_batch($1,'page',$2,1)`, miApp, bodyHash).Scan(&id, &replay); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if replay || id == "" {
		_ = tx.Rollback(ctx)
		t.Fatal("new partial receipt not created")
	}
	if err = tx.Commit(ctx); err == nil {
		t.Fatal("unfinalized receipt committed")
	}
	if n := miCount(t, m.f.owner, `SELECT count(*) FROM meta_inbox.batches WHERE id=$1`, id); n != 0 {
		t.Fatal("failed transaction left partial receipt")
	}
}

// MI02/03/04/06: the signed HTTP path owns one atomic receipt, scope-frozen
// event and fixed River job. Exact same-body retries and rebatched MID are
// historical identities; changed payload is a new quarantine version only.
func TestMetaInboxSignedHTTPAdmissionReplayAndConflict(t *testing.T) {
	m := miSetup(t)
	asset := miAsset()
	mid := "m." + randomUUID()
	binding := miBinding(t, m, asset, "facebook", m.f.tenantA, m.f.storeA1, m.f.principalA)
	miRoute(t, m, asset, m.f.tenantA, m.f.storeA1, binding)
	raw := miMessage(asset, mid, "private-text-"+randomUUID())
	batch, err := m.verifier.Verify(raw, miSignature(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Events) != 1 {
		t.Fatal("synthetic message did not normalize to one event")
	}
	beforeJobs := miCount(t, m.f.owner, `SELECT count(*) FROM river_meta.river_job WHERE kind='meta_inbox_v1'`)
	status, body := miPost(t, m, raw)
	miStatus(t, status, body, 200)
	if n := miCount(t, m.f.owner, `SELECT count(*) FROM river_meta.river_job WHERE kind='meta_inbox_v1'`); n != beforeJobs+1 {
		t.Fatal("routed webhook did not create exactly one job")
	}
	var eventID, eventKey, payloadHash, tenantID, storeID, routeID, keyID, metadata string
	var routeEpoch, jobID int64
	var nonce, ciphertext []byte
	if err := m.f.owner.QueryRow(context.Background(), `SELECT e.id::text,e.event_key,e.payload_hash,e.tenant_id::text,e.store_id::text,e.route_id::text,e.route_epoch,e.job_id,b.key_id,b.nonce,b.ciphertext,row_to_json(e)::text
		FROM meta_inbox.events e JOIN meta_private.event_bodies b ON b.event_id=e.id WHERE e.app_id=$1 AND e.object='page' AND e.event_key=$2`, miApp, batch.Events[0].Key).
		Scan(&eventID, &eventKey, &payloadHash, &tenantID, &storeID, &routeID, &routeEpoch, &jobID, &keyID, &nonce, &ciphertext, &metadata); err != nil {
		t.Fatal("committed event body missing", err)
	}
	if eventKey != batch.Events[0].Key || payloadHash != batch.Events[0].PayloadHash || tenantID != m.f.tenantA || storeID != m.f.storeA1 || routeEpoch != 1 || jobID < 1 || keyID != miKeyID {
		t.Fatal("routed scope or envelope did not freeze")
	}
	plain := miDecrypt(t, m.key, "event", eventID, miApp, "page", "", eventKey, payloadHash, tenantID, storeID, routeID, routeEpoch, keyID, nonce, ciphertext)
	if !bytes.Equal(plain, batch.Events[0].Payload) {
		t.Fatal("normalized event did not round-trip exactly")
	}
	var batchID, rawKey string
	var rawNonce, rawCipher []byte
	if err := m.f.owner.QueryRow(context.Background(), `SELECT b.id::text,r.key_id,r.nonce,r.ciphertext FROM meta_inbox.batches b JOIN meta_private.raw_bodies r ON r.batch_id=b.id WHERE b.app_id=$1 AND b.object='page' AND b.body_hash=$2`, miApp, batch.BodyHash).
		Scan(&batchID, &rawKey, &rawNonce, &rawCipher); err != nil {
		t.Fatal("committed raw body missing", err)
	}
	if recovered := miDecrypt(t, m.key, "raw", batchID, miApp, "page", batch.BodyHash, "", "", "", "", "", 0, rawKey, rawNonce, rawCipher); !bytes.Equal(recovered, raw) {
		t.Fatal("authenticated raw bytes not exactly recoverable")
	}
	var kind, queue, args string
	if err := m.f.owner.QueryRow(context.Background(), `SELECT kind,queue,args::text FROM river_meta.river_job WHERE id=$1`, jobID).Scan(&kind, &queue, &args); err != nil {
		t.Fatal("linked job missing", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(args), &fields); err != nil || len(fields) != 2 || string(fields["event_id"]) != `"`+eventID+`"` || string(fields["version"]) != "1" || kind != "meta_inbox_v1" || queue != "meta_inbox" {
		t.Fatal("job args/kind/queue not exact internal identity")
	}
	if strings.Contains(metadata, "private-text-") || strings.Contains(args, "private-text-") || strings.Contains(args, asset) {
		t.Fatal("provider payload or external asset leaked into global metadata/job")
	}
	status, body = miPost(t, m, raw)
	miStatus(t, status, body, 200)
	if n := miCount(t, m.f.owner, `SELECT count(*) FROM river_meta.river_job WHERE kind='meta_inbox_v1'`); n != beforeJobs+1 {
		t.Fatal("same-body retry created a second job")
	}
	// New raw bytes with the same MID and normalized event are a separate receipt,
	// but may not create a new canonical event or job.
	rebatched := []byte(strings.Replace(string(raw), `"time":123`, `"time":124`, 1))
	if bytes.Equal(rebatched, raw) {
		t.Fatal("test did not re-batch")
	}
	status, body = miPost(t, m, rebatched)
	miStatus(t, status, body, 200)
	if n := miCount(t, m.f.owner, `SELECT count(*) FROM river_meta.river_job WHERE kind='meta_inbox_v1'`); n != beforeJobs+1 {
		t.Fatal("same MID re-batch created another job")
	}
	changed := []byte(strings.Replace(string(raw), "private-text-", "changed-text-", 1))
	conflictBatch, err := m.verifier.Verify(changed, miSignature(changed))
	if err != nil || len(conflictBatch.Events) != 1 {
		t.Fatal("changed payload fixture invalid")
	}
	status, body = miPost(t, m, changed)
	miStatus(t, status, body, 200)
	if n := miCount(t, m.f.owner, `SELECT count(*) FROM river_meta.river_job WHERE kind='meta_inbox_v1'`); n != beforeJobs+1 {
		t.Fatal("changed MID payload produced commerce job")
	}
	if n := miCount(t, m.f.owner, `SELECT count(*) FROM meta_inbox.events WHERE app_id=$1 AND object='page' AND event_key=$2`, miApp, batch.Events[0].Key); n != 2 {
		t.Fatalf("changed MID should have primary+conflict versions, got %d", n)
	}
	var conflictID, conflictKey, conflictHash, conflictKeyID, disposition, reason string
	var conflictNonce, conflictCipher []byte
	if err := m.f.owner.QueryRow(context.Background(), `SELECT e.id::text,e.event_key,e.payload_hash,e.disposition,e.reason,q.key_id,q.nonce,q.ciphertext
		FROM meta_inbox.events e JOIN meta_private.quarantine_bodies q ON q.event_id=e.id WHERE e.app_id=$1 AND e.object='page' AND e.event_key=$2 AND e.payload_hash=$3`, miApp, conflictBatch.Events[0].Key, conflictBatch.Events[0].PayloadHash).
		Scan(&conflictID, &conflictKey, &conflictHash, &disposition, &reason, &conflictKeyID, &conflictNonce, &conflictCipher); err != nil {
		t.Fatal("conflict quarantine body missing", err)
	}
	if disposition != "QUARANTINED" || reason != "payload_conflict" {
		t.Fatal("changed MID was not conflict quarantined")
	}
	conflictPlain := miDecrypt(t, m.key, "quarantine", conflictID, miApp, "page", "", conflictKey, conflictHash, "", "", "", 0, conflictKeyID, conflictNonce, conflictCipher)
	if !bytes.Equal(conflictPlain, conflictBatch.Events[0].Payload) {
		t.Fatal("quarantine did not preserve normalized changed payload")
	}
	if n := miCount(t, m.f.owner, `SELECT count(*) FROM meta_inbox.batches WHERE app_id=$1 AND object='page' AND body_hash IN ($2,$3,$4)`, miApp, batch.BodyHash, miBodyHash(rebatched), miBodyHash(changed)); n != 3 {
		t.Fatalf("all distinct raw receipts not persisted: %d", n)
	}
}

func miSignature(raw []byte) string {
	mac := hmac.New(sha256.New, []byte(miSecret))
	_, _ = mac.Write(raw)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func miBodyHash(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }

// MI06: force a failure at COMMIT, after all apparent inserts. The HTTP
// response cannot be 200 and the complete unit of work must disappear.
func TestMetaInboxDeferredCommitFailureNeverACKs(t *testing.T) {
	m := miSetup(t)
	ctx := context.Background()
	asset := miAsset()
	binding := miBinding(t, m, asset, "facebook", m.f.tenantA, m.f.storeA1, m.f.principalA)
	miRoute(t, m, asset, m.f.tenantA, m.f.storeA1, binding)
	raw := miMessage(asset, "m."+randomUUID(), "private-failed-commit")
	batch, err := m.verifier.Verify(raw, miSignature(raw))
	if err != nil || len(batch.Events) != 1 {
		t.Fatal("test message invalid")
	}
	beforeJobs := miCount(t, m.f.owner, `SELECT count(*) FROM river_meta.river_job WHERE kind='meta_inbox_v1'`)
	name := "mi_fail_" + strings.ReplaceAll(randomUUID(), "-", "")
	fn := pgx.Identifier{name}.Sanitize()
	if _, err := m.f.owner.Exec(ctx, `CREATE FUNCTION public.`+fn+`() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test deferred commit failure' USING ERRCODE='P0001'; END $$`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		mustExec(t, m.f.owner, `DROP TRIGGER IF EXISTS `+fn+` ON meta_inbox.batches`)
		mustExec(t, m.f.owner, `DROP FUNCTION IF EXISTS public.`+fn+`()`)
	})
	if _, err := m.f.owner.Exec(ctx, `CREATE CONSTRAINT TRIGGER `+fn+` AFTER INSERT ON meta_inbox.batches DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.`+fn+`()`); err != nil {
		t.Fatal(err)
	}
	status, body := miPost(t, m, raw)
	miStatus(t, status, body, http.StatusServiceUnavailable)
	if body != "COMMIT_UNAVAILABLE" {
		t.Fatal("commit error response was not fixed and sanitized")
	}
	if n := miCount(t, m.f.owner, `SELECT count(*) FROM meta_inbox.batches WHERE app_id=$1 AND object='page' AND body_hash=$2`, miApp, batch.BodyHash); n != 0 {
		t.Fatal("failed commit left receipt")
	}
	if n := miCount(t, m.f.owner, `SELECT count(*) FROM meta_inbox.events WHERE app_id=$1 AND object='page' AND event_key=$2`, miApp, batch.Events[0].Key); n != 0 {
		t.Fatal("failed commit left event")
	}
	if n := miCount(t, m.f.owner, `SELECT count(*) FROM river_meta.river_job WHERE kind='meta_inbox_v1'`); n != beforeJobs {
		t.Fatal("failed commit left job")
	}
	mustExec(t, m.f.owner, `DROP TRIGGER `+fn+` ON meta_inbox.batches`)
	mustExec(t, m.f.owner, `DROP FUNCTION public.`+fn+`()`)
	status, body = miPost(t, m, raw)
	miStatus(t, status, body, http.StatusOK)
}

// MI03/04/05: one signed envelope contains a routed unit, its duplicate, and
// an untrusted foreign asset. Registration later must not rehome the old key.
func TestMetaInboxMixedBatchAccountingAndRoutingFence(t *testing.T) {
	m := miSetup(t)
	ctx := context.Background()
	known, unknown := miAsset(), miAsset()
	knownMID, unknownMID := "m."+randomUUID(), "m."+randomUUID()
	knownText, unknownText := "routed-"+randomUUID(), "unknown-"+randomUUID()
	binding := miBinding(t, m, known, "facebook", m.f.tenantA, m.f.storeA1, m.f.principalA)
	route, epoch := miRoute(t, m, known, m.f.tenantA, m.f.storeA1, binding)
	unit := func(asset, mid, text string) string {
		return fmt.Sprintf(`{"sender":{"id":"4"},"recipient":{"id":%q},"message":{"mid":%q,"text":%q}}`, asset, mid, text)
	}
	raw := []byte(fmt.Sprintf(`{"object":"page","entry":[{"id":%q,"time":123,"messaging":[%s,%s]},{"id":%q,"time":123,"messaging":[%s]}]}`, known, unit(known, knownMID, knownText), unit(known, knownMID, knownText), unknown, unit(unknown, unknownMID, unknownText)))
	batch, err := m.verifier.Verify(raw, miSignature(raw))
	if err != nil || len(batch.Events) != 3 {
		t.Fatal("mixed batch fixture invalid")
	}
	if batch.Events[0].Key != batch.Events[1].Key || batch.Events[0].PayloadHash != batch.Events[1].PayloadHash {
		t.Fatal("test units not duplicate")
	}
	beforeJobs := miCount(t, m.f.owner, `SELECT count(*) FROM river_meta.river_job WHERE kind='meta_inbox_v1'`)
	status, body := miPost(t, m, raw)
	miStatus(t, status, body, 200)
	var count, distinct int64
	var ordinals string
	if err := m.f.owner.QueryRow(ctx, `SELECT count(*),count(DISTINCT be.event_id),array_agg(be.ordinal ORDER BY be.ordinal)::text FROM meta_inbox.batch_events be JOIN meta_inbox.batches b ON b.id=be.batch_id WHERE b.app_id=$1 AND b.object='page' AND b.body_hash=$2`, miApp, batch.BodyHash).Scan(&count, &distinct, &ordinals); err != nil {
		t.Fatal(err)
	}
	if count != 3 || distinct != 2 || ordinals != "{1,2,3}" {
		t.Fatalf("batch members lost or reordered: count=%d distinct=%d ordinals=%s", count, distinct, ordinals)
	}
	if n := miCount(t, m.f.owner, `SELECT count(*) FROM river_meta.river_job WHERE kind='meta_inbox_v1'`); n != beforeJobs+1 {
		t.Fatal("mixed batch emitted wrong job count")
	}
	var unknownID, unknownKey, unknownHash, reason, disposition, keyID string
	var nonce, ciphertext []byte
	if err := m.f.owner.QueryRow(ctx, `SELECT e.id::text,e.event_key,e.payload_hash,e.disposition,e.reason,q.key_id,q.nonce,q.ciphertext
		FROM meta_inbox.events e JOIN meta_private.quarantine_bodies q ON q.event_id=e.id WHERE e.app_id=$1 AND e.object='page' AND e.event_key=$2`, miApp, batch.Events[2].Key).
		Scan(&unknownID, &unknownKey, &unknownHash, &disposition, &reason, &keyID, &nonce, &ciphertext); err != nil {
		t.Fatal("unknown asset quarantine missing", err)
	}
	if disposition != "QUARANTINED" || reason != "untrusted_route" {
		t.Fatal("unknown asset got routed or wrong quarantine reason")
	}
	if recovered := miDecrypt(t, m.key, "quarantine", unknownID, miApp, "page", "", unknownKey, unknownHash, "", "", "", 0, keyID, nonce, ciphertext); !bytes.Equal(recovered, batch.Events[2].Payload) {
		t.Fatal("unknown asset payload not retained")
	}
	unknownBinding := miBinding(t, m, unknown, "facebook", m.f.tenantA, m.f.storeA1, m.f.principalA)
	miRoute(t, m, unknown, m.f.tenantA, m.f.storeA1, unknownBinding)
	// Rebatch after route activation: the original event remains untrusted.
	status, body = miPost(t, m, miMessage(unknown, unknownMID, unknownText))
	miStatus(t, status, body, 200)
	if n := miCount(t, m.f.owner, `SELECT count(*) FROM meta_inbox.events WHERE app_id=$1 AND object='page' AND event_key=$2`, miApp, unknownKey); n != 1 {
		t.Fatal("historical unknown event rehomed or duplicated")
	}
	if n := miCount(t, m.f.owner, `SELECT count(*) FROM river_meta.river_job WHERE kind='meta_inbox_v1'`); n != beforeJobs+1 {
		t.Fatal("late registration created a job for historical unknown")
	}
	var nextEpoch int64
	if err := m.registrar.QueryRow(ctx, `SELECT meta_inbox.disable_route($1,$2)`, route, epoch).Scan(&nextEpoch); err != nil || nextEpoch != epoch+1 {
		t.Fatal("route CAS disable failed", err)
	}
	status, body = miPost(t, m, miMessage(known, "m."+randomUUID(), "after-disable"))
	miStatus(t, status, body, 200)
	if n := miCount(t, m.f.owner, `SELECT count(*) FROM river_meta.river_job WHERE kind='meta_inbox_v1'`); n != beforeJobs+1 {
		t.Fatal("disabled route emitted a new job")
	}
	var frozenRoute string
	var frozenEpoch int64
	if err := m.f.owner.QueryRow(ctx, `SELECT route_id::text,route_epoch FROM meta_inbox.events WHERE app_id=$1 AND object='page' AND event_key=$2`, miApp, batch.Events[0].Key).Scan(&frozenRoute, &frozenEpoch); err != nil || frozenRoute != route || frozenEpoch != epoch {
		t.Fatal("old routed event scope/epoch mutated after revoke", err)
	}
}

// MI07: expiry never wins over unresolved work. Curator evidence and a terminal
// River state are separate requirements; neither ingress nor merchant can
// declare completion or inspect ciphertext.
func TestMetaInboxRetentionTerminalEvidenceAndPrivateACL(t *testing.T) {
	m := miSetup(t)
	ctx := context.Background()
	known, unknown := miAsset(), miAsset()
	binding := miBinding(t, m, known, "facebook", m.f.tenantA, m.f.storeA1, m.f.principalA)
	miRoute(t, m, known, m.f.tenantA, m.f.storeA1, binding)
	raw := []byte(fmt.Sprintf(`{"object":"page","entry":[{"id":%q,"messaging":[{"sender":{"id":"4"},"recipient":{"id":%q},"message":{"mid":%q,"text":"route"}}]},{"id":%q,"messaging":[{"sender":{"id":"4"},"recipient":{"id":%q},"message":{"mid":%q,"text":"quarantine"}}]}]}`, known, known, "m."+randomUUID(), unknown, unknown, "m."+randomUUID()))
	batch, err := m.verifier.Verify(raw, miSignature(raw))
	if err != nil || len(batch.Events) != 2 {
		t.Fatal("retention fixture invalid")
	}
	status, body := miPost(t, m, raw)
	miStatus(t, status, body, 200)
	var batchID, routedID, unknownID string
	var jobID int64
	if err := m.f.owner.QueryRow(ctx, `SELECT id::text FROM meta_inbox.batches WHERE app_id=$1 AND object='page' AND body_hash=$2`, miApp, batch.BodyHash).Scan(&batchID); err != nil {
		t.Fatal(err)
	}
	if err := m.f.owner.QueryRow(ctx, `SELECT id::text,job_id FROM meta_inbox.events WHERE app_id=$1 AND object='page' AND event_key=$2`, miApp, batch.Events[0].Key).Scan(&routedID, &jobID); err != nil {
		t.Fatal(err)
	}
	if err := m.f.owner.QueryRow(ctx, `SELECT id::text FROM meta_inbox.events WHERE app_id=$1 AND object='page' AND event_key=$2`, miApp, batch.Events[1].Key).Scan(&unknownID); err != nil {
		t.Fatal(err)
	}
	// This is an isolated synthetic DB; age is shifted only to exercise the
	// retention predicate, not to simulate an authorized review decision.
	mustExec(t, m.f.owner, `UPDATE meta_private.raw_bodies SET expires_at=clock_timestamp()-interval '1 hour' WHERE batch_id=$1`, batchID)
	mustExec(t, m.f.owner, `UPDATE meta_private.event_bodies SET expires_at=clock_timestamp()-interval '1 hour' WHERE event_id=$1`, routedID)
	mustExec(t, m.f.owner, `UPDATE meta_private.quarantine_bodies SET expires_at=clock_timestamp()-interval '1 hour' WHERE event_id=$1`, unknownID)
	purge := func(limit int) int {
		t.Helper()
		var n int
		if err := m.curator.QueryRow(ctx, `SELECT meta_inbox.purge_expired($1)`, limit).Scan(&n); err != nil {
			t.Fatal("curator purge", err)
		}
		return n
	}
	if n := purge(10); n != 0 {
		t.Fatal("age alone purged pending or unresolved body")
	}
	for name, p := range map[string]*pgxpool.Pool{"ingress": m.ingress, "registrar": m.registrar, "merchant": m.f.runtime} {
		t.Run("terminal denied "+name, func(t *testing.T) {
			_, err := p.Exec(ctx, `SELECT meta_inbox.record_terminal($1,'reviewed_rejected',$2)`, unknownID, strings.Repeat("a", 64))
			if miSQLState(err) != "42501" {
				t.Fatalf("unauthorized terminal SQLSTATE=%s", miSQLState(err))
			}
		})
	}
	if _, err := m.curator.Exec(ctx, `SELECT meta_inbox.record_terminal($1,'processed',$2)`, unknownID, strings.Repeat("b", 64)); miSQLState(err) != "22023" {
		t.Fatalf("curator asserted processed SQLSTATE=%s", miSQLState(err))
	}
	for name, p := range map[string]*pgxpool.Pool{"ingress": m.ingress, "registrar": m.registrar, "curator": m.curator, "merchant": m.f.runtime, "payment worker": miPool(t, m.f, waPayment), "claims worker": miPool(t, m.f, waClaims)} {
		t.Run("private read denied "+name, func(t *testing.T) {
			var id string
			err := p.QueryRow(ctx, `SELECT event_id::text FROM meta_private.event_bodies WHERE event_id=$1`, routedID).Scan(&id)
			if miSQLState(err) != "42501" {
				t.Fatalf("private ciphertext SELECT SQLSTATE=%s", miSQLState(err))
			}
		})
	}
	evidence := strings.Repeat("c", 64)
	if _, err := m.curator.Exec(ctx, `SELECT meta_inbox.record_terminal($1,'reviewed_rejected',$2)`, unknownID, evidence); err != nil {
		t.Fatal("quarantine review", err)
	}
	if n := purge(10); n != 1 {
		t.Fatalf("reviewed quarantine purge=%d want1", n)
	}
	if miCount(t, m.f.owner, `SELECT count(*) FROM meta_private.raw_bodies WHERE batch_id=$1`, batchID) != 1 || miCount(t, m.f.owner, `SELECT count(*) FROM meta_private.event_bodies WHERE event_id=$1`, routedID) != 1 {
		t.Fatal("raw or pending routed body purged early")
	}
	if _, err := m.curator.Exec(ctx, `SELECT meta_inbox.record_terminal($1,'retention_discarded',$2)`, routedID, strings.Repeat("d", 64)); err != nil {
		t.Fatal("routed terminal evidence", err)
	}
	if n := purge(10); n != 0 {
		t.Fatal("pending River job did not block terminalized body purge")
	}
	mustExec(t, m.f.owner, `UPDATE river_meta.river_job SET state='completed',finalized_at=clock_timestamp() WHERE id=$1`, jobID)
	if n := purge(1); n != 1 {
		t.Fatalf("bounded first purge=%d", n)
	}
	if n := purge(1); n != 1 {
		t.Fatalf("bounded second purge=%d", n)
	}
	if n := purge(1); n != 0 {
		t.Fatalf("retention not idempotent: %d", n)
	}
	if miCount(t, m.f.owner, `SELECT count(*) FROM meta_inbox.batches WHERE id=$1 AND finalized`, batchID) != 1 || miCount(t, m.f.owner, `SELECT count(*) FROM meta_inbox.events WHERE id IN ($1,$2)`, routedID, unknownID) != 2 {
		t.Fatal("purge deleted permanent dedupe metadata")
	}
	if miCount(t, m.f.owner, `SELECT count(*) FROM meta_private.raw_bodies WHERE batch_id=$1`, batchID) != 0 || miCount(t, m.f.owner, `SELECT count(*) FROM meta_private.event_bodies WHERE event_id=$1`, routedID) != 0 {
		t.Fatal("eligible ciphertext not purged")
	}
	beforeJobs := miCount(t, m.f.owner, `SELECT count(*) FROM river_meta.river_job WHERE kind='meta_inbox_v1'`)
	status, body = miPost(t, m, raw)
	miStatus(t, status, body, 200)
	if miCount(t, m.f.owner, `SELECT count(*) FROM river_meta.river_job WHERE kind='meta_inbox_v1'`) != beforeJobs || miCount(t, m.f.owner, `SELECT count(*) FROM meta_private.raw_bodies WHERE batch_id=$1`, batchID) != 0 {
		t.Fatal("historical replay resurrected job or ciphertext")
	}
	mustExec(t, m.f.owner, `DELETE FROM river_meta.river_job WHERE id=$1`, jobID)
	prunedJobs := miCount(t, m.f.owner, `SELECT count(*) FROM river_meta.river_job WHERE kind='meta_inbox_v1'`)
	status, body = miPost(t, m, raw)
	miStatus(t, status, body, 200)
	if miCount(t, m.f.owner, `SELECT count(*) FROM river_meta.river_job WHERE kind='meta_inbox_v1'`) != prunedJobs || miCount(t, m.f.owner, `SELECT count(*) FROM meta_private.raw_bodies WHERE batch_id=$1`, batchID) != 0 {
		t.Fatal("post-prune replay resurrected body or job")
	}
}

// MI04: concurrent identical deliveries serialize on the permanent body hash.
func TestMetaInboxConcurrentSameBodyHasOneIdentity(t *testing.T) {
	m := miSetup(t)
	asset := miAsset()
	binding := miBinding(t, m, asset, "facebook", m.f.tenantA, m.f.storeA1, m.f.principalA)
	miRoute(t, m, asset, m.f.tenantA, m.f.storeA1, binding)
	raw := miMessage(asset, "m."+randomUUID(), "same-body")
	batch, err := m.verifier.Verify(raw, miSignature(raw))
	if err != nil || len(batch.Events) != 1 {
		t.Fatal("concurrent fixture invalid")
	}
	before := miCount(t, m.f.owner, `SELECT count(*) FROM river_meta.river_job WHERE kind='meta_inbox_v1'`)
	type answer struct {
		status int
		body   string
	}
	start := make(chan struct{})
	done := make(chan answer, 2)
	for i := 0; i < 2; i++ {
		go func() { <-start; s, b := miPost(t, m, raw); done <- answer{s, b} }()
	}
	close(start)
	for i := 0; i < 2; i++ {
		select {
		case a := <-done:
			miStatus(t, a.status, a.body, 200)
		case <-time.After(12 * time.Second):
			t.Fatal("concurrent delivery stalled")
		}
	}
	if miCount(t, m.f.owner, `SELECT count(*) FROM meta_inbox.batches WHERE app_id=$1 AND object='page' AND body_hash=$2`, miApp, batch.BodyHash) != 1 || miCount(t, m.f.owner, `SELECT count(*) FROM meta_inbox.events WHERE app_id=$1 AND object='page' AND event_key=$2`, miApp, batch.Events[0].Key) != 1 || miCount(t, m.f.owner, `SELECT count(*) FROM river_meta.river_job WHERE kind='meta_inbox_v1'`) != before+1 {
		t.Fatal("concurrent replay created duplicate receipt, event, or job")
	}
}

// MI05: a delivery that waited on a binding lock must use the database clock
// after the wait, not the proof validity observed before it.
func TestMetaInboxProofExpiryAfterObservedBindingWait(t *testing.T) {
	m := miSetup(t)
	ctx := context.Background()
	asset := miAsset()
	binding := miBinding(t, m, asset, "facebook", m.f.tenantA, m.f.storeA1, m.f.principalA)
	route, _ := miRoute(t, m, asset, m.f.tenantA, m.f.storeA1, binding)
	var expires time.Time
	if err := m.f.owner.QueryRow(ctx, `UPDATE meta_inbox.routes SET proof_expires=clock_timestamp()+interval '700 milliseconds' WHERE id=$1 RETURNING proof_expires`, route).Scan(&expires); err != nil {
		t.Fatal(err)
	}
	lock, err := m.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback(context.Background())
	var locked string
	if err := lock.QueryRow(ctx, `SELECT id::text FROM integration.bindings WHERE id=$1 FOR UPDATE`, binding).Scan(&locked); err != nil || locked != binding {
		t.Fatal("test binding lock not held", err)
	}
	raw := miMessage(asset, "m."+randomUUID(), "after-proof-expiry")
	batch, err := m.verifier.Verify(raw, miSignature(raw))
	if err != nil || len(batch.Events) != 1 {
		t.Fatal("expiry fixture invalid")
	}
	before := miCount(t, m.f.owner, `SELECT count(*) FROM river_meta.river_job WHERE kind='meta_inbox_v1'`)
	type answer struct {
		status int
		body   string
	}
	done := make(chan answer, 1)
	go func() { s, b := miPost(t, m, raw); done <- answer{s, b} }()
	login := m.ingress.Config().ConnConfig.User
	observed := false
	until := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(until) {
		if miCount(t, m.f.owner, `SELECT count(*) FROM pg_stat_activity WHERE usename=$1 AND wait_event_type='Lock' AND query LIKE '%meta_inbox.prepare_event%'`, login) > 0 {
			observed = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !observed {
		t.Fatal("ingress was not observed waiting on binding lock")
	}
	var stillValid bool
	if err := m.f.owner.QueryRow(ctx, `SELECT proof_expires>clock_timestamp() FROM meta_inbox.routes WHERE id=$1`, route).Scan(&stillValid); err != nil || !stillValid {
		t.Fatal("proof expired before the observed lock wait", err)
	}
	for time.Now().Before(expires.Add(100 * time.Millisecond)) {
		time.Sleep(10 * time.Millisecond)
	}
	if err := lock.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case a := <-done:
		miStatus(t, a.status, a.body, 200)
	case <-time.After(12 * time.Second):
		t.Fatal("blocked delivery did not finish")
	}
	var disposition, reason string
	var jobID *int64
	if err := m.f.owner.QueryRow(ctx, `SELECT disposition,reason,job_id FROM meta_inbox.events WHERE app_id=$1 AND object='page' AND event_key=$2`, miApp, batch.Events[0].Key).Scan(&disposition, &reason, &jobID); err != nil {
		t.Fatal(err)
	}
	if disposition != "QUARANTINED" || reason != "untrusted_route" || jobID != nil || miCount(t, m.f.owner, `SELECT count(*) FROM river_meta.river_job WHERE kind='meta_inbox_v1'`) != before {
		t.Fatal("expired proof after lock wait produced business job")
	}
}

// MI07: an in-flight River lifecycle update must make ciphertext ineligible,
// even when the last committed job state is terminal. This is the regression
// counterexample for a plain SELECT of job state inside purgeable().
func TestMetaInboxPurgeSkipsLockedRiverJob(t *testing.T) {
	m := miSetup(t)
	ctx := context.Background()
	asset := miAsset()
	binding := miBinding(t, m, asset, "facebook", m.f.tenantA, m.f.storeA1, m.f.principalA)
	miRoute(t, m, asset, m.f.tenantA, m.f.storeA1, binding)
	raw := miMessage(asset, "m."+randomUUID(), "locked-retention")
	batch, err := m.verifier.Verify(raw, miSignature(raw))
	if err != nil || len(batch.Events) != 1 {
		t.Fatal("locked retention fixture invalid")
	}
	status, body := miPost(t, m, raw)
	miStatus(t, status, body, 200)
	var batchID, eventID string
	var jobID int64
	if err := m.f.owner.QueryRow(ctx, `SELECT id::text FROM meta_inbox.batches WHERE app_id=$1 AND object='page' AND body_hash=$2`, miApp, batch.BodyHash).Scan(&batchID); err != nil {
		t.Fatal(err)
	}
	if err := m.f.owner.QueryRow(ctx, `SELECT id::text,job_id FROM meta_inbox.events WHERE app_id=$1 AND object='page' AND event_key=$2`, miApp, batch.Events[0].Key).Scan(&eventID, &jobID); err != nil {
		t.Fatal(err)
	}
	mustExec(t, m.f.owner, `UPDATE meta_private.raw_bodies SET expires_at=clock_timestamp()-interval '1 hour' WHERE batch_id=$1`, batchID)
	mustExec(t, m.f.owner, `UPDATE meta_private.event_bodies SET expires_at=clock_timestamp()-interval '1 hour' WHERE event_id=$1`, eventID)
	if _, err := m.curator.Exec(ctx, `SELECT meta_inbox.record_terminal($1,'retention_discarded',$2)`, eventID, strings.Repeat("e", 64)); err != nil {
		t.Fatal(err)
	}
	mustExec(t, m.f.owner, `UPDATE river_meta.river_job SET state='completed',finalized_at=clock_timestamp() WHERE id=$1`, jobID)
	lock, err := m.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback(context.Background())
	var locked int64
	if err := lock.QueryRow(ctx, `SELECT id FROM river_meta.river_job WHERE id=$1 FOR UPDATE`, jobID).Scan(&locked); err != nil || locked != jobID {
		t.Fatal("job lifecycle row not locked", err)
	}
	bounded, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var purged int
	if err := m.curator.QueryRow(bounded, `SELECT meta_inbox.purge_expired(10)`).Scan(&purged); err != nil {
		t.Fatal("busy-job purge did not finish promptly", err)
	}
	if purged != 0 || miCount(t, m.f.owner, `SELECT count(*) FROM meta_private.raw_bodies WHERE batch_id=$1`, batchID) != 1 || miCount(t, m.f.owner, `SELECT count(*) FROM meta_private.event_bodies WHERE event_id=$1`, eventID) != 1 {
		t.Fatal("busy River job lost recoverable ciphertext")
	}
	if err := lock.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.curator.QueryRow(ctx, `SELECT meta_inbox.purge_expired(10)`).Scan(&purged); err != nil || purged != 2 {
		t.Fatalf("unlocked eligible bodies purge=%d err=%v", purged, err)
	}
}

// MI05: a merchant binding version change invalidates a previously trusted
// route until registrar re-authorization, without upgrading old quarantines.
func TestMetaInboxBindingVersionFenceAndReauthorization(t *testing.T) {
	m := miSetup(t)
	ctx := context.Background()
	asset := miAsset()
	binding := miBinding(t, m, asset, "facebook", m.f.tenantA, m.f.storeA1, m.f.principalA)
	route, epoch := miRoute(t, m, asset, m.f.tenantA, m.f.storeA1, binding)
	mustExec(t, m.f.owner, `UPDATE integration.bindings SET semantic_version=semantic_version+1 WHERE id=$1`, binding)
	stale := miMessage(asset, "m."+randomUUID(), "stale-binding")
	batch, err := m.verifier.Verify(stale, miSignature(stale))
	if err != nil || len(batch.Events) != 1 {
		t.Fatal("binding fixture invalid")
	}
	before := miCount(t, m.f.owner, `SELECT count(*) FROM river_meta.river_job WHERE kind='meta_inbox_v1'`)
	status, body := miPost(t, m, stale)
	miStatus(t, status, body, 200)
	if miCount(t, m.f.owner, `SELECT count(*) FROM river_meta.river_job WHERE kind='meta_inbox_v1'`) != before || miCount(t, m.f.owner, `SELECT count(*) FROM meta_inbox.events WHERE app_id=$1 AND object='page' AND event_key=$2 AND disposition='QUARANTINED' AND reason='untrusted_route'`, miApp, batch.Events[0].Key) != 1 {
		t.Fatal("stale binding version was trusted")
	}
	proof := strings.Repeat("f", 64)
	_, err = m.registrar.Exec(ctx, `SELECT * FROM meta_inbox.activate_route($1,'page',$2,$3,$4,$5,1,$6,clock_timestamp()+interval '1 hour',$7)`, miApp, asset, m.f.tenantA, m.f.storeA1, binding, proof, epoch)
	if miSQLState(err) != "PT409" {
		t.Fatalf("stale version reauthorization SQLSTATE=%s", miSQLState(err))
	}
	var nextRoute string
	var nextEpoch int64
	if err := m.registrar.QueryRow(ctx, `SELECT * FROM meta_inbox.activate_route($1,'page',$2,$3,$4,$5,2,$6,clock_timestamp()+interval '1 hour',$7)`, miApp, asset, m.f.tenantA, m.f.storeA1, binding, proof, epoch).Scan(&nextRoute, &nextEpoch); err != nil || nextRoute != route || nextEpoch != epoch+1 {
		t.Fatal("current binding reauthorization failed", err)
	}
	status, body = miPost(t, m, stale)
	miStatus(t, status, body, 200)
	if miCount(t, m.f.owner, `SELECT count(*) FROM river_meta.river_job WHERE kind='meta_inbox_v1'`) != before {
		t.Fatal("historical stale event rehomed after reauthorization")
	}
	status, body = miPost(t, m, miMessage(asset, "m."+randomUUID(), "fresh-binding"))
	miStatus(t, status, body, 200)
	if miCount(t, m.f.owner, `SELECT count(*) FROM river_meta.river_job WHERE kind='meta_inbox_v1'`) != before+1 {
		t.Fatal("fresh event not admitted after trusted reauthorization")
	}
}

// MI04: two distinct raw receipts racing for the same stable MID cannot each
// acquire a business job; the event-key lock is independent of body hash.
func TestMetaInboxConcurrentRebatchedMIDHasOneJob(t *testing.T) {
	m := miSetup(t)
	asset := miAsset()
	binding := miBinding(t, m, asset, "facebook", m.f.tenantA, m.f.storeA1, m.f.principalA)
	miRoute(t, m, asset, m.f.tenantA, m.f.storeA1, binding)
	rawA := miMessage(asset, "m."+randomUUID(), "same-event")
	rawB := []byte(strings.Replace(string(rawA), `"time":123`, `"time":124`, 1))
	first, err := m.verifier.Verify(rawA, miSignature(rawA))
	if err != nil || len(first.Events) != 1 {
		t.Fatal("first rebatch invalid")
	}
	second, err := m.verifier.Verify(rawB, miSignature(rawB))
	if err != nil || len(second.Events) != 1 || first.BodyHash == second.BodyHash || first.Events[0].Key != second.Events[0].Key || first.Events[0].PayloadHash != second.Events[0].PayloadHash {
		t.Fatal("rebatch did not preserve stable event identity")
	}
	before := miCount(t, m.f.owner, `SELECT count(*) FROM river_meta.river_job WHERE kind='meta_inbox_v1'`)
	type answer struct {
		status int
		body   string
	}
	start := make(chan struct{})
	done := make(chan answer, 2)
	for _, raw := range [][]byte{rawA, rawB} {
		raw := raw
		go func() { <-start; s, b := miPost(t, m, raw); done <- answer{s, b} }()
	}
	close(start)
	for i := 0; i < 2; i++ {
		select {
		case a := <-done:
			miStatus(t, a.status, a.body, 200)
		case <-time.After(12 * time.Second):
			t.Fatal("concurrent rebatch stalled")
		}
	}
	if miCount(t, m.f.owner, `SELECT count(*) FROM meta_inbox.batches WHERE app_id=$1 AND object='page' AND body_hash IN ($2,$3)`, miApp, first.BodyHash, second.BodyHash) != 2 || miCount(t, m.f.owner, `SELECT count(*) FROM meta_inbox.events WHERE app_id=$1 AND object='page' AND event_key=$2`, miApp, first.Events[0].Key) != 1 || miCount(t, m.f.owner, `SELECT count(*) FROM river_meta.river_job WHERE kind='meta_inbox_v1'`) != before+1 {
		t.Fatal("racing rebatches duplicated or dropped receipt/event/job")
	}
}
