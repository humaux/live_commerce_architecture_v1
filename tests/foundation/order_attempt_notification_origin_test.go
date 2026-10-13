// Purpose: REAL_PG tests-first proof that READY retains its prepared origin/version.
// Depends: signed native ingress, complete C1/C2, registrar domain definers and HPKE.
// Used by: parent-owned OA1 CI; no dispatcher, test trigger or substituted SQL helper.
// Invariants: changing authority after PAY_PREPARED cannot commit an old-origin
// artifact; rollback retains C1 and explicit retry seals the current eligible origin.
package foundation_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"livecommerce/internal/inbox"
	"livecommerce/internal/integrations/meta/pagetoken/pageopen"
	"livecommerce/internal/msgtemplates"
	"livecommerce/internal/orderattempt"
)

// Retain only the test-side HPKE opener omitted by the shared executor fixture;
// the production executor receives the same public-only key custody as usual.
func oaOriginPinScenario(t *testing.T) (oaCrashScenario, *pageopen.Keyring, string) {
	t.Helper()
	e := lbuNew(t)
	session, offer := e.session("A1", ltgLive, 5)
	bundle := e.h.accepted(t, session, "", "origin-pin-"+t04Tag(), "A1+2").BundleID
	e.h.closeWindow(t, session)
	m, conversation, binding, psid := oaNotificationSignedConversation(t, e)
	e.linkPeer(bundle, conversation)
	selection := oaClaimSelection(e, []string{bundle}, []string{offer.ID}, conversation)
	selection["send_payment_link"] = true
	a := oaAllocationAdmit(t, e, selection)
	config := miPool(t, e.p.f, "commerce_claims_worker").Config().Copy()
	application := "oa_origin_" + strings.ReplaceAll(randomUUID(), "-", "")
	if config.ConnConfig.RuntimeParams == nil {
		config.ConnConfig.RuntimeParams = map[string]string{}
	}
	config.ConnConfig.RuntimeParams["application_name"] = application
	config.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	keyJSON := `{"keys":[{"id":"` + miKeyID + `","key_base64":"` + base64.StdEncoding.EncodeToString(m.key) + `"}]}`
	keys, err := inbox.LoadKeyring(func(n string) string {
		switch n {
		case "COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID":
			return miKeyID
		case "COMMERCE_META_PAYLOAD_KEYS_JSON":
			return keyJSON
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	notifications, err := inbox.NewService(keys)
	if err != nil {
		t.Fatal(err)
	}
	seal, open := mcnPageRing(t)
	jobs, err := river.NewClient[pgx.Tx](riverpgxv5.New(pool), &river.Config{Schema: "river"})
	if err != nil {
		t.Fatal(err)
	}
	notifications.EnableSend(seal, jobs, msgtemplates.NewService())
	executor, err := orderattempt.NewExecutor(pool, oaServiceKeys(t), payEntryTestKeys(t), notifications)
	if err != nil {
		t.Fatal(err)
	}
	return oaCrashScenario{e: e, a: a, executor: executor, pool: pool, application: application,
		bundle: bundle, offer: offer.ID, conversation: conversation, binding: binding}, open, psid
}

func oaOriginPinAuthority(t *testing.T, s oaCrashScenario, origin string) int64 {
	t.Helper()
	var version int64
	var sole bool
	err := s.e.p.f.owner.QueryRow(context.Background(), `SELECT min(d.version),count(*)=1 AND bool_and(d.origin=$3)
	 FROM control.storefront_domains d JOIN control.storefront_publications p USING(tenant_id,store_id)
	 JOIN control.stores st ON st.tenant_id=d.tenant_id AND st.id=d.store_id JOIN control.tenants tn ON tn.id=d.tenant_id
	 WHERE d.tenant_id=$1 AND d.store_id=$2 AND st.active AND tn.active AND p.published AND d.state='ACTIVE'
	 AND d.ownership_verified_at<=clock_timestamp() AND d.tls_verified_at<=clock_timestamp() AND d.valid_until>clock_timestamp()`,
		s.e.tenant(), s.e.store(), origin).Scan(&version, &sole)
	if err != nil || !sole || version < 1 {
		t.Fatalf("sole current eligible native origin/version sentinel: %v", err)
	}
	return version
}

func oaOriginPinOpenCurrent(t *testing.T, s oaCrashScenario, open *pageopen.Keyring, psid, origin string) {
	t.Helper()
	var operation, body, locale, keyVersion string
	var enc, sealed, digest, credentialCT []byte
	err := s.e.p.f.owner.QueryRow(context.Background(), `SELECT n.operation_id::text,z.enc,z.sealed,
	 tpl.body,o.locale,p.key_version,p.token_hash,p.credential_ciphertext
	 FROM inbox.order_payment_notifications n JOIN inbox.send_secrets z ON z.operation_id=n.operation_id
	 JOIN checkout.orders o ON o.id=n.order_id JOIN checkout.order_pay_entries p ON p.order_id=o.id
	 JOIN inbox.outbound_messages b ON b.id=n.outbound_id
	 JOIN msgtemplates.fixed_templates tpl ON tpl.template_id=b.template_id AND tpl.version=b.template_version
	 WHERE n.create_attempt_id=$1`, s.a.AttemptID).Scan(&operation, &enc, &sealed, &body, &locale, &keyVersion, &digest, &credentialCT)
	if err != nil {
		t.Fatal("native dispatch and reusable entry readback", err)
	}
	material := orderattempt.PayEntryMaterial{KeyVersion: keyVersion, Ciphertext: credentialCT}
	if len(digest) != len(material.Digest) {
		t.Fatal("native pay-entry digest length")
	}
	copy(material.Digest[:], digest)
	credential, err := payEntryTestKeys(t).Open(orderattempt.PayEntryBinding{
		TenantID: s.e.tenant(), StoreID: s.e.store(), OrderID: s.a.AttemptID, Version: 1}, material)
	if err != nil {
		t.Fatal("native reusable entry did not open")
	}
	if _, err = open.OpenSend(s.e.tenant(), s.e.store(), randomUUID(), enc, sealed); err == nil {
		t.Fatal("dispatch lost its operation binding")
	}
	secret, err := open.OpenSend(s.e.tenant(), s.e.store(), operation, enc, sealed)
	if err != nil {
		t.Fatal("independent HPKE open failed")
	}
	defer clear(secret)
	var send struct {
		PSID string `json:"recipient_psid"`
		Text string `json:"text"`
	}
	link := origin + "/" + locale + "/pay-entry#o=" + s.a.AttemptID + "&t=" + credential
	if json.Unmarshal(secret, &send) != nil || send.PSID != psid || send.Text != strings.Replace(body, "{{連結}}", link, 1) {
		t.Fatal("recovery must encrypt the signed recipient and exact currently eligible origin link")
	}
}

func TestOrderAttemptNotificationOriginPin(t *testing.T) {
	for _, change := range []string{"switch_origin", "same_origin_version"} {
		t.Run(change, func(t *testing.T) {
			s, open, psid := oaOriginPinScenario(t)
			// The registrar requires the native owner mapping that the smaller
			// commerce fixture omits. Domain transitions themselves use definers.
			mustExec(t, s.e.p.f.owner, `INSERT INTO identity.initial_stores(principal_id,idempotency_key,request_hash,tenant_id,store_id,warehouse_id)
			 VALUES($1,$2,sha256($2::text::bytea),$3,$4,$5)`, s.e.p.f.principalA, "oa-origin-"+randomUUID(), s.e.tenant(), s.e.store(), s.e.p.stock.warehouse.ID)
			t.Cleanup(func() {
				mustExec(t, s.e.p.f.owner, `DELETE FROM identity.initial_stores WHERE principal_id=$1`, s.e.p.f.principalA)
			})
			registrar := miPool(t, s.e.p.f, "commerce_storefront_registrar")
			oldOrigin := s.e.origin
			oldVersion := oaOriginPinAuthority(t, s, oldOrigin)
			currentOrigin := oldOrigin
			if change == "switch_origin" {
				currentOrigin = "https://oa-origin-" + t04Tag() + ".example"
				t.Cleanup(func() {
					mustExec(t, s.e.p.f.owner, `DELETE FROM control.storefront_domains WHERE origin=$1`, currentOrigin)
				})
			}
			before := oaCrashCustody(t, s)
			defer clear(before)
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			holder, err := s.e.p.f.owner.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			holderPID := int32(holder.Conn().PgConn().PID())
			defer func() {
				clean, stop := context.WithTimeout(context.Background(), 3*time.Second)
				defer stop()
				_ = holder.Rollback(clean)
			}()
			// A real relation lock blocks the Executor's external-operation INSERT.
			// PAY_PREPARED already captured A, but HPKE sealing runs after InsertTx
			// returns; it must not seal that stale snapshot and commit against B.
			if _, err = holder.Exec(ctx, `LOCK TABLE river.river_job IN SHARE MODE`); err != nil {
				t.Fatal(err)
			}
			done := make(chan oaExecutorOutcome, 1)
			go func() {
				out, callErr := s.executor.Execute(ctx, "", "", s.a.AttemptID, s.a.Generation)
				done <- oaExecutorOutcome{out, callErr}
			}()
			finished := false
			defer func() {
				cancel()
				clean, stop := context.WithTimeout(context.Background(), 3*time.Second)
				defer stop()
				_ = holder.Rollback(clean)
				if !finished {
					select {
					case <-done:
					case <-clean.Done():
						t.Error("bounded origin-pin executor shutdown")
					}
				}
			}()
			ticker := time.NewTicker(5 * time.Millisecond)
			defer ticker.Stop()
			for {
				var blocked bool
				err = s.e.p.f.owner.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity a JOIN pg_locks l ON l.pid=a.pid
				 WHERE a.application_name=$1 AND a.wait_event_type='Lock' AND $2::int=ANY(pg_blocking_pids(a.pid))
				 AND l.relation='river.river_job'::regclass AND l.mode='RowExclusiveLock' AND NOT l.granted
				 AND lower(a.query) LIKE '%insert%river_job%')`, s.application, holderPID).Scan(&blocked)
				if err != nil {
					t.Fatal("native post-prepare job INSERT barrier", err)
				}
				if blocked {
					break
				}
				select {
				case result := <-done:
					finished = true
					t.Fatalf("executor missed the positive prepared-origin barrier: %v", result.err)
				case <-ctx.Done():
					t.Fatal("positive prepared-origin barrier absent")
				case <-ticker.C:
				}
			}
			transition, err := registrar.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				clean, stop := context.WithTimeout(context.Background(), 3*time.Second)
				defer stop()
				_ = transition.Rollback(clean)
			}()
			if change == "switch_origin" {
				if _, err = transition.Exec(ctx, `SELECT control.operator_suspend_domain($1)`, oldOrigin); err != nil {
					t.Fatal("native registrar suspends prepared origin", err)
				}
			}
			if _, err = transition.Exec(ctx, `SELECT control.operator_bind_domain($1,$2,'synthetic OA origin-pin proof',clock_timestamp()+interval '1 day')`, s.e.store(), currentOrigin); err != nil {
				t.Fatal("native registrar binds current origin/version", err)
			}
			if err = transition.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			currentVersion := oaOriginPinAuthority(t, s, currentOrigin)
			if change == "same_origin_version" && currentVersion <= oldVersion {
				t.Fatal("same-origin authority transition did not advance its native version")
			}
			if err = holder.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			var result oaExecutorOutcome
			select {
			case result = <-done:
				finished = true
			case <-ctx.Done():
				t.Fatal("origin-change refusal exceeded the existing bounded execution")
			}
			var failure *orderattempt.AdmissionError
			if !errors.As(result.err, &failure) || failure.SQLState() != "PT409" || failure.Code() != "order_attempt_retryable" || failure.OutcomeUnknown() {
				t.Fatalf("changed prepared origin/version must refuse READY with known rollback: state=%s err=%v", result.receipt.State, result.err)
			}
			if result.receipt.State != "" || len(result.receipt.Receipt) != 0 {
				t.Fatal("stale prepared origin produced a terminal receipt")
			}
			oaCrashPending(t, s, before)
			retryCtx, retryCancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer retryCancel()
			out, err := s.executor.Execute(retryCtx, "", "", s.a.AttemptID, s.a.Generation)
			if err != nil {
				t.Fatal("explicit recovery against current origin", err)
			}
			durable := oaCrashCommitted(t, s, out)
			defer clear(durable)
			oaOriginPinOpenCurrent(t, s, open, psid, currentOrigin)
			replay, err := s.executor.Execute(retryCtx, "", "", s.a.AttemptID, s.a.Generation)
			if err != nil || !bytes.Equal(out.Receipt, replay.Receipt) || !bytes.Equal(durable, oaCrashCommitted(t, s, replay)) {
				t.Fatal("terminal replay changed the current-origin durable READY artifacts")
			}
		})
	}
}
