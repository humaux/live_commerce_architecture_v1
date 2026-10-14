// Purpose: REAL_PG regression for an authenticated historical notification recipient refusal.
// Depends on: signed native ingress, real C1 claim custody, payload AEAD and concrete Executor.
// Used by: parent-owned OA1 area-Go CI; no provider, product hook or trigger bypass.
// Invariants: malformed historical plaintext is distinguished from key/tag failure;
// exact fixture custody is restored, and notification-only VOID retains the complete order.

package foundation_test

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"
)

func TestOrderAttemptAreaGoAuthenticatedRecipientRefusalCommitsVoid(t *testing.T) {
	e := lbuNew(t)
	session, offer := e.session("A1", ltgLive, 5)
	bundle := e.h.accepted(t, session, "", "recipient-"+t04Tag(), "A1+2").BundleID
	e.h.closeWindow(t, session)
	m, conversation, binding, _ := oaNotificationSignedConversation(t, e)
	e.linkPeer(bundle, conversation)
	selection := oaClaimSelection(e, []string{bundle}, []string{offer.ID}, conversation)
	selection["send_payment_link"] = true
	a := oaAllocationAdmit(t, e, selection)
	plan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
	pool := miPool(t, e.p.f, "commerce_claims_worker")
	s := oaNotificationResumeScenario{oaCrashScenario: oaCrashScenario{
		e: e, a: a, pool: pool, bundle: bundle, offer: offer.ID,
		conversation: conversation, binding: binding,
		executor: oaNotificationExecutorWithPayloadKey(t, pool, m.key),
	}, plan: plan}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.p.f.owner.QueryRow(ctx, `SELECT coalesce(jsonb_agg(id ORDER BY id),'[]'::jsonb)
	 FROM river.river_job WHERE kind='external_operation_v1'`).Scan(&s.externalJobsBefore); err != nil {
		t.Fatal("capture exact external job identities", err)
	}

	var event, app, object, asset, eventKey, route, keyID, originalHash string
	var epoch int64
	var originalNonce, originalCiphertext []byte
	var native bool
	err := e.p.f.owner.QueryRow(ctx, `SELECT m.event_id::text,v.app_id,v.object,v.asset_id,v.event_key,
	 v.route_id::text,v.route_epoch,m.key_id,v.payload_hash,m.nonce,m.ciphertext,
	 v.completed AND v.terminal_reason='processed' AND v.terminal_evidence=v.payload_hash
	 AND EXISTS(SELECT 1 FROM meta_inbox.audit_events au WHERE au.event_id=v.id
	  AND au.action='processed' AND au.evidence_hash=v.payload_hash)
	 AND (SELECT count(*)=1 FROM social.messages x WHERE x.conversation_id=m.conversation_id)
	 FROM social.messages m JOIN meta_inbox.events v ON v.id=m.event_id
	 WHERE m.tenant_id=$1 AND m.store_id=$2 AND m.conversation_id=$3`,
		e.tenant(), e.store(), conversation).Scan(&event, &app, &object, &asset, &eventKey,
		&route, &epoch, &keyID, &originalHash, &originalNonce, &originalCiphertext, &native)
	if err != nil || !native || keyID != miKeyID {
		t.Fatalf("native signed processed message sentinel: native=%v err=%v", native, err)
	}
	block, err := aes.NewCipher(m.key)
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	aad := func(hash string) []byte {
		t.Helper()
		encoded, marshalErr := json.Marshal([]any{"livecommerce/meta-payload/v1", "event", event,
			app, object, "", eventKey, hash, e.tenant(), e.store(), route, epoch, keyID})
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		return encoded
	}
	original, err := aead.Open(nil, originalNonce, originalCiphertext, aad(originalHash))
	if err != nil {
		t.Fatal("independent native message authentication failed")
	}
	defer clear(original)
	var unit map[string]any
	if json.Unmarshal(original, &unit) != nil || unit["sender"] == nil || unit["recipient"] == nil {
		t.Fatal("signed baseline lacks its native sender/recipient")
	}
	delete(unit, "sender")
	plain, err := json.Marshal(unit)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(plain)
	sum := sha256.Sum256(plain)
	faultHash := hex.EncodeToString(sum[:])
	faultNonce := randomBytes(12)
	faultCiphertext := aead.Seal(nil, faultNonce, plain, aad(faultHash))
	opened, err := aead.Open(nil, faultNonce, faultCiphertext, aad(faultHash))
	if err != nil || !bytes.Equal(opened, plain) {
		t.Fatal("recipient fault must remain authenticated, never a tag failure")
	}
	clear(opened)

	// Current ingress correctly rejects this payload. Model only historical
	// authenticated source corruption after the actual native message commit.
	// Do not rewrite the processed audit or disable immutable/native guards.
	// Register exact restoration BEFORE either autocommit fixture mutation.
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		tx, beginErr := e.p.f.owner.Begin(cleanup)
		if beginErr != nil {
			t.Error("recipient fixture cleanup begin", beginErr)
			return
		}
		defer tx.Rollback(cleanup)
		events, restoreErr := tx.Exec(cleanup, `UPDATE meta_inbox.events SET payload_hash=$4
		 WHERE id=$1 AND tenant_id=$2 AND store_id=$3`, event, e.tenant(), e.store(), originalHash)
		if restoreErr != nil || events.RowsAffected() != 1 {
			t.Errorf("exact recipient event restore rows=%d err=%v", events.RowsAffected(), restoreErr)
			return
		}
		messages, restoreErr := tx.Exec(cleanup, `UPDATE social.messages SET nonce=$4,ciphertext=$5
		 WHERE event_id=$1 AND tenant_id=$2 AND store_id=$3 AND conversation_id=$6`,
			event, e.tenant(), e.store(), originalNonce, originalCiphertext, conversation)
		if restoreErr != nil || messages.RowsAffected() != 1 {
			t.Errorf("exact recipient message restore rows=%d err=%v", messages.RowsAffected(), restoreErr)
			return
		}
		var restored bool
		restoreErr = tx.QueryRow(cleanup, `SELECT v.payload_hash=$4 AND m.nonce=$5 AND m.ciphertext=$6
		 AND v.terminal_evidence=v.payload_hash
		 AND EXISTS(SELECT 1 FROM meta_inbox.audit_events au WHERE au.event_id=v.id AND au.action='processed' AND au.evidence_hash=v.payload_hash)
		 FROM meta_inbox.events v JOIN social.messages m ON m.event_id=v.id
		 WHERE v.id=$1 AND v.tenant_id=$2 AND v.store_id=$3 AND m.conversation_id=$7`,
			event, e.tenant(), e.store(), originalHash, originalNonce, originalCiphertext, conversation).Scan(&restored)
		if restoreErr != nil || !restored {
			t.Errorf("recipient source custody not exactly restored: restored=%v err=%v", restored, restoreErr)
			return
		}
		if restoreErr = tx.Commit(cleanup); restoreErr != nil {
			t.Error("recipient fixture cleanup commit", restoreErr)
		}
	})
	mustExec(t, e.p.f.owner, `UPDATE meta_inbox.events SET payload_hash=$4
	 WHERE id=$1 AND tenant_id=$2 AND store_id=$3`, event, e.tenant(), e.store(), faultHash)
	mustExec(t, e.p.f.owner, `UPDATE social.messages SET nonce=$4,ciphertext=$5
	 WHERE event_id=$1 AND tenant_id=$2 AND store_id=$3 AND conversation_id=$6`,
		event, e.tenant(), e.store(), faultNonce, faultCiphertext, conversation)
	var exactFault bool
	err = e.p.f.owner.QueryRow(ctx, `SELECT v.payload_hash=$4 AND m.nonce=$5 AND m.ciphertext=$6
	 AND a.state='PENDING' AND a.admission_finished AND a.order_id IS NULL
	 FROM meta_inbox.events v JOIN social.messages m ON m.event_id=v.id
	 JOIN ops.order_attempts a ON a.attempt_id=$7
	 WHERE v.id=$1 AND v.tenant_id=$2 AND v.store_id=$3 AND m.conversation_id=$8`,
		event, e.tenant(), e.store(), faultHash, faultNonce, faultCiphertext, a.AttemptID, conversation).Scan(&exactFault)
	if err != nil || !exactFault {
		t.Fatalf("authenticated historical recipient fault not installed: exact=%v err=%v", exactFault, err)
	}
	// This shared oracle retains every native claim, source, hold, pay-entry,
	// expiry, audit and immutable receipt assertion, plus the exact job-ID set.
	oaNotificationResumeExecuteVoid(t, s, "recipient_invalid")
}
