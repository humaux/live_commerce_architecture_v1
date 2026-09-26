package foundation_test

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"livecommerce/internal/integrations/meta"
)

// The historical gates install the original bytes and checksum, stopping at
// the named version. Latest migrations.Apply would instead move the old lane.
func mcApplyHistorical(t *testing.T, f *testFixture, versions ...string) {
	t.Helper()
	ctx := context.Background()
	tx, err := f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	for _, version := range versions {
		body, err := os.ReadFile(filepath.Join("../../migrations", version))
		if err != nil {
			t.Fatal(err)
		}
		checksum := fmt.Sprintf("%x", sha256.Sum256(body))
		var have string
		if err := tx.QueryRow(ctx, `SELECT coalesce((SELECT checksum FROM public.lc_schema_migrations WHERE version=$1),'')`, version).Scan(&have); err != nil {
			t.Fatal(err)
		}
		if have == checksum {
			continue
		}
		if have != "" {
			t.Fatalf("historical %s checksum changed", version)
		}
		if _, err := tx.Exec(ctx, string(body)); err != nil {
			t.Fatalf("historical %s: %v", version, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.lc_schema_migrations(version,checksum) VALUES($1,$2)`, version, checksum); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func mcOldSetup(t *testing.T, f *testFixture, object string) miTest {
	t.Helper()
	m := miTest{f: f, ingress: miPool(t, f, "commerce_meta_ingress"), registrar: miPool(t, f, "commerce_meta_registrar"), curator: miPool(t, f, "commerce_meta_curator"), key: randomBytes(32)}
	var err error
	m.verifier, err = meta.NewVerifier(meta.Config{AppID: miApp, Object: object, AppSecret: miSecret, VerifyToken: "meta-inbox-verify-token"})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

type mcOldJobArgs struct {
	EventID string `json:"event_id"`
	Version int    `json:"version"`
}

func (mcOldJobArgs) Kind() string { return "meta_inbox_v1" }

// Only old-database tests use this independent AES-GCM fixture envelope. The
// existing miDecrypt checks both AAD and the plaintext hash after sealing.
func mcOldSeal(t *testing.T, key []byte, class, id, app, object, bodyHash, eventKey, payloadHash, tenant, store, route string, epoch int64, plain []byte) ([]byte, []byte) {
	t.Helper()
	aad, err := json.Marshal([]any{"livecommerce/meta-payload/v1", class, id, app, object, bodyHash, eventKey, payloadHash, tenant, store, route, epoch, miKeyID})
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
	nonce := randomBytes(aead.NonceSize())
	ciphertext := aead.Seal(nil, nonce, plain, aad)
	if got := miDecrypt(t, key, class, id, app, object, bodyHash, eventKey, payloadHash, tenant, store, route, epoch, miKeyID, nonce, ciphertext); !bytes.Equal(got, plain) {
		t.Fatal("old-lane test envelope did not decrypt independently")
	}
	return nonce, ciphertext
}

// Replays original 0028 ingress SQL under the dedicated login and inserts the
// real River job transactionally into its original, fixed `river` schema.
// No owner-made receipt or production schema switch is involved.
func mcOldPost(t *testing.T, m miTest, asset string, raw []byte) mcEvent {
	t.Helper()
	ctx := context.Background()
	batch, err := m.verifier.Verify(raw, miSignature(raw))
	if err != nil || len(batch.Events) != 1 || batch.Events[0].AssetID != asset {
		t.Fatalf("invalid historical signed fixture: %v", err)
	}
	unit := batch.Events[0]
	e := mcEvent{key: unit.Key, hash: unit.PayloadHash, app: batch.AppID, object: batch.Object, asset: asset, plain: unit.Payload}
	jobs, err := river.NewClient(riverpgxv5.New(m.ingress), &river.Config{Schema: "river"})
	if err != nil {
		t.Fatal(err)
	}
	tx, err := m.ingress.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	var batchID string
	var replay bool
	if err := tx.QueryRow(ctx, `SELECT batch_id::text,replay FROM meta_inbox.begin_batch($1,$2,$3,$4)`, batch.AppID, batch.Object, batch.BodyHash, 1).Scan(&batchID, &replay); err != nil {
		t.Fatal(err)
	}
	if !replay {
		var needsBody bool
		var bodyClass string
		var tenant, store, route *string
		var epoch *int64
		if err := tx.QueryRow(ctx, `SELECT event_id::text,needs_body,body_class,tenant_id::text,store_id::text,route_id::text,route_epoch FROM meta_inbox.prepare_event($1::uuid,1,$2,$3,$4,$5,$6,$7)`, batchID, unit.Key, unit.PayloadHash, asset, unit.Kind, unit.QuarantineReason, unit.OccurredAt).Scan(&e.id, &needsBody, &bodyClass, &tenant, &store, &route, &epoch); err != nil {
			t.Fatal(err)
		}
		if needsBody {
			var scopeTenant, scopeStore, scopeRoute string
			var scopeEpoch int64
			if bodyClass == "event" {
				if tenant == nil || store == nil || route == nil || epoch == nil {
					t.Fatal("historical routed scope missing")
				}
				scopeTenant, scopeStore, scopeRoute, scopeEpoch = *tenant, *store, *route, *epoch
			} else if bodyClass != "quarantine" {
				t.Fatal("unexpected historical body class", bodyClass)
			}
			nonce, encrypted := mcOldSeal(t, m.key, bodyClass, e.id, batch.AppID, batch.Object, "", unit.Key, unit.PayloadHash, scopeTenant, scopeStore, scopeRoute, scopeEpoch, unit.Payload)
			var jobID any
			if bodyClass == "event" {
				job, err := jobs.InsertTx(ctx, tx, mcOldJobArgs{EventID: e.id, Version: 1}, &river.InsertOpts{Queue: "meta_inbox"})
				if err != nil || job == nil || job.Job == nil {
					t.Fatalf("historical River enqueue: %v", err)
				}
				e.job = job.Job.ID
				jobID = e.job
			}
			if _, err := tx.Exec(ctx, `SELECT meta_inbox.complete_event($1::uuid,$2::uuid,$3,$4,$5,$6::bigint)`, batchID, e.id, miKeyID, nonce, encrypted, jobID); err != nil {
				t.Fatal(err)
			}
		}
		rawNonce, rawCipher := mcOldSeal(t, m.key, "raw", batchID, batch.AppID, batch.Object, batch.BodyHash, "", "", "", "", "", 0, raw)
		if _, err := tx.Exec(ctx, `SELECT meta_inbox.complete_batch($1::uuid,$2,$3,$4)`, batchID, miKeyID, rawNonce, rawCipher); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.f.owner.QueryRow(ctx, `SELECT id::text,coalesce(route_id::text,''),coalesce(route_epoch,0),coalesce(job_id,0) FROM meta_inbox.events WHERE app_id=$1 AND object=$2 AND event_key=$3 AND payload_hash=$4 AND is_primary`, e.app, e.object, e.key, e.hash).Scan(&e.id, &e.route, &e.epoch, &e.job); err != nil {
		t.Fatal(err)
	}
	return e
}
