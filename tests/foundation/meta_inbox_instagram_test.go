package foundation_test

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/integrations/meta"
)

func miInstagram(t *testing.T, m miTest) miTest {
	t.Helper()
	keys, err := meta.NewPayloadKeyring(miKeyID, map[string][]byte{miKeyID: m.key})
	if err != nil {
		t.Fatal(err)
	}
	inbox, err := meta.NewInbox(context.Background(), m.ingress, keys)
	if err != nil {
		t.Fatal("Instagram ingress constructor", err)
	}
	m.verifier, err = meta.NewVerifier(meta.Config{AppID: miApp, Object: "instagram", AppSecret: miSecret, VerifyToken: "meta-inbox-verify-token"})
	if err != nil {
		t.Fatal(err)
	}
	m.handler, err = meta.NewInboxHandler(m.verifier, inbox)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func miInstagramRoute(t *testing.T, m miTest, asset, binding string) (string, int64) {
	t.Helper()
	var route string
	var epoch int64
	err := m.registrar.QueryRow(context.Background(), `SELECT * FROM meta_inbox.activate_route($1,'instagram',$2,$3,$4,$5,1,$6,$7,0)`, miApp, asset, m.f.tenantA, m.f.storeA1, binding, strings.Repeat("a", 64), time.Now().Add(time.Hour)).Scan(&route, &epoch)
	if err != nil {
		t.Fatal("trusted Instagram route", err)
	}
	return route, epoch
}

// MI01/02/03: a Facebook/Page binding cannot register an Instagram asset.
// The signed Instagram batch still admits all three scoped kinds atomically.
func TestMetaInboxInstagramThreeKindsProviderAndCipherScope(t *testing.T) {
	m := miInstagram(t, miSetup(t))
	ctx := context.Background()
	asset := miAsset()
	wrong := miBinding(t, m, asset, "facebook", m.f.tenantA, m.f.storeA1, m.f.principalA)
	_, err := m.registrar.Exec(ctx, `SELECT * FROM meta_inbox.activate_route($1,'instagram',$2,$3,$4,$5,1,$6,$7,0)`, miApp, asset, m.f.tenantA, m.f.storeA1, wrong, strings.Repeat("b", 64), time.Now().Add(time.Hour))
	if miSQLState(err) != "PT409" {
		t.Fatalf("Facebook binding activated Instagram: SQLSTATE=%s", miSQLState(err))
	}
	if miCount(t, m.f.owner, `SELECT count(*) FROM meta_inbox.asset_owners WHERE object='instagram' AND asset_id=$1`, asset) != 0 {
		t.Fatal("wrong-provider activation claimed asset ownership")
	}
	binding := miBinding(t, m, asset, "instagram", m.f.tenantA, m.f.storeA1, m.f.principalA)
	route, epoch := miInstagramRoute(t, m, asset, binding)
	raw := []byte(fmt.Sprintf(`{"object":"instagram","entry":[{"id":%q,"time":123,"changes":[{"field":"comments","value":{"id":%q,"text":"ig-comment"}},{"field":"live_comments","value":{"id":%q,"text":"ig-live"}}],"messaging":[{"sender":{"id":"4"},"recipient":{"id":%q},"message":{"mid":%q,"text":"ig-message"}}]}]}`, asset, miAsset(), miAsset(), asset, "m."+randomUUID()))
	batch, err := m.verifier.Verify(raw, miSignature(raw))
	if err != nil || len(batch.Events) != 3 {
		t.Fatal("Instagram three-kind fixture invalid")
	}
	for i, kind := range []string{"instagram_comment", "instagram_live_comment", "instagram_message"} {
		if batch.Events[i].Kind != kind {
			t.Fatalf("event %d kind=%s want=%s", i, batch.Events[i].Kind, kind)
		}
	}
	beforeJobs := miCount(t, m.f.owner, `SELECT count(*) FROM river.river_job WHERE kind='meta_inbox_v1'`)
	status, body := miPost(t, m, raw)
	miStatus(t, status, body, 200)
	var batchID, rawKey string
	var rawNonce, rawCipher []byte
	if err := m.f.owner.QueryRow(ctx, `SELECT b.id::text,r.key_id,r.nonce,r.ciphertext FROM meta_inbox.batches b JOIN meta_private.raw_bodies r ON r.batch_id=b.id WHERE b.app_id=$1 AND b.object='instagram' AND b.body_hash=$2`, miApp, batch.BodyHash).Scan(&batchID, &rawKey, &rawNonce, &rawCipher); err != nil {
		t.Fatal("Instagram raw ciphertext missing", err)
	}
	if got := miDecrypt(t, m.key, "raw", batchID, miApp, "instagram", batch.BodyHash, "", "", "", "", "", 0, rawKey, rawNonce, rawCipher); !bytes.Equal(got, raw) {
		t.Fatal("Instagram authenticated raw bytes changed")
	}
	if miCount(t, m.f.owner, `SELECT count(*) FROM meta_inbox.batch_events WHERE batch_id=$1`, batchID) != 3 || miCount(t, m.f.owner, `SELECT count(*) FROM river.river_job WHERE kind='meta_inbox_v1'`) != beforeJobs+3 {
		t.Fatal("Instagram batch dropped a unit or job")
	}
	for _, event := range batch.Events {
		var id, key, payloadHash, tenant, store, storedRoute, disposition, bodyKey string
		var storedEpoch, jobID int64
		var nonce, ciphertext []byte
		if err := m.f.owner.QueryRow(ctx, `SELECT e.id::text,e.event_key,e.payload_hash,e.tenant_id::text,e.store_id::text,e.route_id::text,e.route_epoch,e.job_id,e.disposition,b.key_id,b.nonce,b.ciphertext
			FROM meta_inbox.events e JOIN meta_private.event_bodies b ON b.event_id=e.id WHERE e.app_id=$1 AND e.object='instagram' AND e.event_key=$2`, miApp, event.Key).
			Scan(&id, &key, &payloadHash, &tenant, &store, &storedRoute, &storedEpoch, &jobID, &disposition, &bodyKey, &nonce, &ciphertext); err != nil {
			t.Fatal("Instagram scoped event missing", err)
		}
		if disposition != "ROUTED" || tenant != m.f.tenantA || store != m.f.storeA1 || storedRoute != route || storedEpoch != epoch || jobID < 1 {
			t.Fatal("Instagram event scope/job not frozen")
		}
		if got := miDecrypt(t, m.key, "event", id, miApp, "instagram", "", key, payloadHash, tenant, store, storedRoute, storedEpoch, bodyKey, nonce, ciphertext); !bytes.Equal(got, event.Payload) {
			t.Fatal("Instagram normalized payload mismatch")
		}
		block, err := aes.NewCipher(m.key)
		if err != nil {
			t.Fatal(err)
		}
		aead, err := cipher.NewGCM(block)
		if err != nil {
			t.Fatal(err)
		}
		pageAAD, err := json.Marshal([]any{"livecommerce/meta-payload/v1", "event", id, miApp, "page", "", key, payloadHash, tenant, store, storedRoute, storedEpoch, bodyKey})
		if err != nil {
			t.Fatal(err)
		}
		if opened, err := aead.Open(nil, nonce, ciphertext, pageAAD); err == nil || len(opened) > 0 {
			t.Fatal("Page object AAD opened Instagram ciphertext")
		}
	}
	status, body = miPost(t, m, raw)
	miStatus(t, status, body, 200)
	if miCount(t, m.f.owner, `SELECT count(*) FROM river.river_job WHERE kind='meta_inbox_v1'`) != beforeJobs+3 {
		t.Fatal("Instagram replay made more jobs")
	}
}

// MI01/05: a Page route and a matching self-registered Instagram binding are
// both insufficient to authorize the Instagram object for that numeric asset.
func TestMetaInboxInstagramCannotBorrowPageRoute(t *testing.T) {
	m := miInstagram(t, miSetup(t))
	asset := miAsset()
	pageBinding := miBinding(t, m, asset, "facebook", m.f.tenantA, m.f.storeA1, m.f.principalA)
	miRoute(t, m, asset, m.f.tenantA, m.f.storeA1, pageBinding)
	miBinding(t, m, asset, "instagram", m.f.tenantA, m.f.storeA1, m.f.principalA)
	raw := []byte(fmt.Sprintf(`{"object":"instagram","entry":[{"id":%q,"changes":[{"field":"comments","value":{"id":%q,"text":"untrusted"}}]}]}`, asset, miAsset()))
	batch, err := m.verifier.Verify(raw, miSignature(raw))
	if err != nil || len(batch.Events) != 1 || batch.Events[0].Kind != "instagram_comment" {
		t.Fatal("cross-object fixture invalid")
	}
	before := miCount(t, m.f.owner, `SELECT count(*) FROM river.river_job WHERE kind='meta_inbox_v1'`)
	status, body := miPost(t, m, raw)
	miStatus(t, status, body, 200)
	var id, reason, disposition, keyID string
	var nonce, ciphertext []byte
	if err := m.f.owner.QueryRow(context.Background(), `SELECT e.id::text,e.reason,e.disposition,q.key_id,q.nonce,q.ciphertext FROM meta_inbox.events e JOIN meta_private.quarantine_bodies q ON q.event_id=e.id WHERE e.app_id=$1 AND e.object='instagram' AND e.event_key=$2`, miApp, batch.Events[0].Key).
		Scan(&id, &reason, &disposition, &keyID, &nonce, &ciphertext); err != nil {
		t.Fatal("cross-object quarantine missing", err)
	}
	if reason != "untrusted_route" || disposition != "QUARANTINED" || miCount(t, m.f.owner, `SELECT count(*) FROM river.river_job WHERE kind='meta_inbox_v1'`) != before {
		t.Fatal("Page route or untrusted IG binding made an IG job")
	}
	if got := miDecrypt(t, m.key, "quarantine", id, miApp, "instagram", "", batch.Events[0].Key, batch.Events[0].PayloadHash, "", "", "", 0, keyID, nonce, ciphertext); !bytes.Equal(got, batch.Events[0].Payload) {
		t.Fatal("cross-object quarantine payload changed")
	}
}
