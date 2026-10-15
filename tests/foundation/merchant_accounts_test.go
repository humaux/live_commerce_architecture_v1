package foundation_test

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/command"
	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/platform"
)

type maFixture struct {
	f         *t06GoFixture
	service   *accounts.Service
	keys      map[string][]byte
	replayKey []byte
}

func maSetup(t *testing.T) *maFixture {
	t.Helper()
	f := newT06GoFixture(t)
	fixtureKeys := map[string][]byte{"fixture_v1": randomBytes(32), "fixture_v2": randomBytes(32)}
	replayKey := randomBytes(32)
	keys, e := accounts.NewKeyring("fixture_v1", fixtureKeys, replayKey)
	if e != nil {
		t.Fatal(e)
	}
	s, e := accounts.New(keys, f.service)
	if e != nil {
		t.Fatal(e)
	}
	// This test owns the tenant. Remove the circular head/version pair in one
	// transaction before the existing base fixture removes binding/membership.
	t.Cleanup(func() {
		tx, e := f.base.owner.Begin(context.Background())
		if e != nil {
			t.Error(e)
			return
		}
		defer tx.Rollback(context.Background())
		if _, e = tx.Exec(context.Background(), `DELETE FROM integration.account_credentials WHERE tenant_id=$1`, f.tenant); e != nil {
			t.Error(e)
			return
		}
		if _, e = tx.Exec(context.Background(), `DELETE FROM integration.merchant_accounts WHERE tenant_id=$1`, f.tenant); e != nil {
			t.Error(e)
			return
		}
		if e = tx.Commit(context.Background()); e != nil {
			t.Error(e)
		}
	})
	return &maFixture{f: f, service: s, keys: fixtureKeys, replayKey: replayKey}
}
func maInput() accounts.CreateInput {
	return accounts.CreateInput{Provider: "payuni", Environment: "SANDBOX", AccountID: "fixture_" + t04Tag(), Credentials: accounts.Credentials{HashKey: strings.Repeat("K", 32), HashIV: strings.Repeat("V", 16)}}
}
func (m *maFixture) create(token, store, key string, in accounts.CreateInput) (out accounts.Connection, e error) {
	e = m.f.scoped(context.Background(), token, store, func(tx pgx.Tx, s platform.Scope) error {
		var e error
		out, e = m.service.Create(context.Background(), tx, s, token, key, in)
		return e
	})
	return
}
func (m *maFixture) rotate(token, key string, in accounts.RotateInput) (out accounts.Connection, e error) {
	e = m.f.scoped(context.Background(), token, m.f.store, func(tx pgx.Tx, s platform.Scope) error {
		var e error
		out, e = m.service.Rotate(context.Background(), tx, s, token, key, in)
		return e
	})
	return
}
func (m *maFixture) get(token, store, id string) (out accounts.Connection, e error) {
	e = m.f.scoped(context.Background(), token, store, func(tx pgx.Tx, s platform.Scope) error {
		var e error
		out, e = m.service.Get(context.Background(), tx, s, token, id)
		return e
	})
	return
}
func (m *maFixture) facts(t *testing.T) (out [7]int64) {
	t.Helper()
	e := m.f.base.owner.QueryRow(context.Background(), `SELECT
 (SELECT count(*) FROM integration.bindings WHERE tenant_id=$1),
 (SELECT count(*) FROM integration.merchant_accounts WHERE tenant_id=$1),
 (SELECT count(*) FROM integration.account_credentials WHERE tenant_id=$1),
 (SELECT count(*) FROM ops.command_results WHERE tenant_id=$1),
 (SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1),
 (SELECT count(*) FROM integration.operations WHERE tenant_id=$1),
 (SELECT count(*) FROM river.river_job)`, m.f.tenant).Scan(&out[0], &out[1], &out[2], &out[3], &out[4], &out[5], &out[6])
	if e != nil {
		t.Fatal(e)
	}
	return
}

func TestMerchantAccountsRoundTripPrivacyAndRotation(t *testing.T) {
	m := maSetup(t)
	in := maInput()
	a := maHistorical(t, m, m.f.store, in, 2)
	liveInput := in
	liveInput.Environment = "LIVE"
	live := maHistorical(t, m, m.f.store, liveInput, 1)
	if live.ID == a.ID || live.BindingID == a.BindingID || live.Environment != "LIVE" || live.State != "CONFIGURED_UNVERIFIED" {
		t.Fatal("archived environments not isolated")
	}
	maAssertPersistedSecret(t, m, live, 1, in.Credentials)
	for _, entry := range []struct {
		id   string
		want int
	}{{a.ID, 2}, {live.ID, 1}} {
		var n int
		if err := m.f.base.owner.QueryRow(context.Background(), `SELECT count(*) FROM integration.account_credentials WHERE connection_id=$1`, entry.id).Scan(&n); err != nil || n != entry.want {
			t.Fatal("immutable credential history lost", err)
		}
	}
	before, snapshot := m.facts(t), maHistorySnapshot(t, m)
	got, e := m.get(m.f.token, m.f.store, a.ID)
	if e != nil || got != a || a.CredentialVersion != 2 || a.BindingVersion != 1 || a.State != "CONFIGURED_UNVERIFIED" || a.KeyID != "fixture_v2" {
		t.Fatalf("historical metadata: %+v %v", got, e)
	}
	for _, version := range []int64{1, 2} {
		maAssertPersistedSecret(t, m, a, version, in.Credentials)
	}
	for _, environment := range []string{"SANDBOX", "LIVE"} {
		in.Environment = environment
		for range 2 {
			if out, e := m.create(m.f.token, m.f.store, t04Key("ma-create"), in); !errors.Is(e, command.ErrInvalid) || out != (accounts.Connection{}) {
				t.Fatalf("retired create admitted: %v", e)
			}
			if out, e := m.rotate(m.f.token, t04Key("ma-rotate"), accounts.RotateInput{ConnectionID: a.ID, ExpectedVersion: 2, Credentials: in.Credentials}); !errors.Is(e, command.ErrInvalid) || out != (accounts.Connection{}) {
				t.Fatalf("retired rotate admitted: %v", e)
			}
		}
	}
	if m.facts(t) != before || maHistorySnapshot(t, m) != snapshot {
		t.Fatal("retired account call changed history, receipts, audit or jobs")
	}
	meta, _ := json.Marshal(got)
	var response, audit string
	if e = m.f.base.owner.QueryRow(context.Background(), `SELECT coalesce(jsonb_agg(response)::text,'') FROM ops.command_results WHERE tenant_id=$1`, m.f.tenant).Scan(&response); e != nil {
		t.Fatal(e)
	}
	if e = m.f.base.owner.QueryRow(context.Background(), `SELECT coalesce(jsonb_agg(to_jsonb(a))::text,'') FROM ops.audit_events a WHERE tenant_id=$1`, m.f.tenant).Scan(&audit); e != nil {
		t.Fatal(e)
	}
	for _, v := range []string{string(meta), response, audit} {
		if strings.Contains(v, in.Credentials.HashKey) || strings.Contains(v, in.Credentials.HashIV) {
			t.Fatal("credential leaked into persistent projection")
		}
	}
	// Existing binding control is independent of the retired credential writer.
	e = m.f.scoped(context.Background(), m.f.token, m.f.store, func(tx pgx.Tx, s platform.Scope) error {
		_, e := m.f.service.SetBindingEnabled(context.Background(), tx, s, m.f.token, t04Key("ma-disable"), a.BindingID, 1, false)
		return e
	})
	if e != nil {
		t.Fatal(e)
	}
	disabled, e := m.get(m.f.token, m.f.store, a.ID)
	if e != nil || disabled.Enabled || disabled.BindingVersion != 2 || disabled.CredentialVersion != 2 {
		t.Fatal("historical control changed credential head", e)
	}
	stable := maHistorySnapshot(t, m)
	if _, e = m.rotate(m.f.token, t04Key("ma-disabled-rotate"), accounts.RotateInput{ConnectionID: a.ID, ExpectedVersion: 2, Credentials: in.Credentials}); !errors.Is(e, command.ErrInvalid) || maHistorySnapshot(t, m) != stable {
		t.Fatal("rotation changed disabled history", e)
	}
}

func maAssertPersistedSecret(t *testing.T, m *maFixture, a accounts.Connection, version int64, want accounts.Credentials) {
	t.Helper()
	var keyID string
	var nonce, encrypted []byte
	if e := m.f.base.owner.QueryRow(context.Background(), `SELECT key_id,nonce,ciphertext FROM integration.account_credentials WHERE tenant_id=$1 AND store_id=$2 AND connection_id=$3 AND version=$4`, m.f.tenant, m.f.store, a.ID, version).Scan(&keyID, &nonce, &encrypted); e != nil {
		t.Fatal(e)
	}
	if len(nonce) != 12 {
		t.Fatal("invalid nonce length")
	}
	if strings.Contains(string(encrypted), want.HashKey) || strings.Contains(string(encrypted), want.HashIV) {
		t.Fatal("credential stored in plaintext")
	}
	block, e := aes.NewCipher(m.keys[keyID])
	if e != nil {
		t.Fatal(e)
	}
	gcm, e := cipher.NewGCM(block)
	if e != nil {
		t.Fatal(e)
	}
	aad := struct {
		FormatVersion     int    `json:"format_version"`
		TenantID          string `json:"tenant_id"`
		StoreID           string `json:"store_id"`
		ConnectionID      string `json:"connection_id"`
		Provider          string `json:"provider"`
		Environment       string `json:"environment"`
		AccountID         string `json:"account_id"`
		CredentialVersion int64  `json:"credential_version"`
	}{1, m.f.tenant, m.f.store, a.ID, a.Provider, a.Environment, a.AccountID, version}
	raw, e := json.Marshal(aad)
	if e != nil {
		t.Fatal(e)
	}
	plain, e := gcm.Open(nil, nonce, encrypted, raw)
	if e != nil {
		t.Fatalf("persisted credential cannot decrypt: %v", e)
	}
	var fields struct {
		HashKey string `json:"hash_key"`
		HashIV  string `json:"hash_iv"`
	}
	if e = json.Unmarshal(plain, &fields); e != nil {
		t.Fatal("persisted secret malformed")
	}
	if fields.HashKey != want.HashKey || fields.HashIV != want.HashIV {
		t.Fatal("persisted credential contents differ")
	}
	originalAAD := aad
	for _, mutate := range []func(){func() { aad.CredentialVersion++ }, func() { aad.StoreID = m.f.otherStore }, func() { aad.Environment = "WRONG" }} {
		aad = originalAAD
		mutate()
		wrong, _ := json.Marshal(aad)
		if _, e = gcm.Open(nil, nonce, encrypted, wrong); e == nil {
			t.Fatal("persisted envelope accepted wrong AAD")
		}
	}
}

func TestMerchantAccountsSQLColumnAndMutationBoundaries(t *testing.T) {
	m := maSetup(t)
	a := maHistorical(t, m, m.f.store, maInput(), 1)
	var e error
	for _, q := range []string{
		`SELECT ciphertext FROM integration.account_credentials WHERE connection_id=$1`,
		`SELECT nonce FROM integration.account_credentials WHERE connection_id=$1`,
		`UPDATE integration.account_credentials SET key_id='changed' WHERE connection_id=$1`,
		`DELETE FROM integration.account_credentials WHERE connection_id=$1`,
		`UPDATE integration.merchant_accounts SET account_id='another' WHERE id=$1`,
		`DELETE FROM integration.merchant_accounts WHERE id=$1`,
	} {
		e = m.f.scoped(context.Background(), m.f.token, m.f.store, func(tx pgx.Tx, s platform.Scope) error { _, e := tx.Exec(context.Background(), q, a.ID); return e })
		var pg *pgconn.PgError
		if !errors.As(e, &pg) || pg.Code != "42501" {
			t.Fatalf("SQL boundary not permission denial: %v", e)
		}
	}
	var n int
	e = m.f.scoped(context.Background(), m.f.token, m.f.otherStore, func(tx pgx.Tx, s platform.Scope) error {
		return tx.QueryRow(context.Background(), `SELECT count(*) FROM integration.merchant_accounts WHERE id=$1`, a.ID).Scan(&n)
	})
	if e != nil || n != 0 {
		t.Fatalf("RLS metadata leaked: %v", e)
	}
	e = m.f.worker.QueryRow(context.Background(), `SELECT count(*) FROM integration.account_credentials`).Scan(&n)
	var pg *pgconn.PgError
	if !errors.As(e, &pg) || pg.Code != "42501" {
		t.Fatalf("worker has unrestricted credential read: %v", e)
	}
	// The deferred FK is a commit-time gate, not just a Go check.
	e = m.f.scoped(context.Background(), m.f.token, m.f.store, func(tx pgx.Tx, s platform.Scope) error {
		_, e := tx.Exec(context.Background(), `UPDATE integration.merchant_accounts SET credential_version=999 WHERE id=$1`, a.ID)
		return e
	})
	if !errors.As(e, &pg) || pg.Code != "23503" {
		t.Fatalf("dangling credential head committed: %v", e)
	}
	got, e := m.get(m.f.token, m.f.store, a.ID)
	if e != nil || got.CredentialVersion != 1 {
		t.Fatal("failed head change persisted")
	}
}

func TestMerchantAccountsAtomicFaultRollback(t *testing.T) {
	for _, table := range []string{"integration.bindings", "integration.merchant_accounts", "integration.account_credentials", "ops.audit_events", "ops.command_results"} {
		t.Run(table, func(t *testing.T) {
			m := maSetup(t)
			before := m.facts(t)
			name := "ma_fault_" + t04Tag()
			mustExec(t, m.f.base.owner, `CREATE SEQUENCE public.`+name+`_hits`)
			mustExec(t, m.f.base.owner, `GRANT USAGE ON SEQUENCE public.`+name+`_hits TO commerce_runtime`)
			condition := `current_setting('app.tenant_id',true)=` + quoteLiteral(m.f.tenant)
			mustExec(t, m.f.base.owner, `CREATE FUNCTION public.`+name+`() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF `+condition+` THEN PERFORM nextval('public.`+name+`_hits'); RAISE EXCEPTION 'merchant credential synthetic fault'; END IF; RETURN NEW; END $$`)
			mustExec(t, m.f.base.owner, `CREATE TRIGGER `+name+` BEFORE INSERT ON `+table+` FOR EACH ROW EXECUTE FUNCTION public.`+name+`()`)
			t.Cleanup(func() {
				mustExec(t, m.f.base.owner, `DROP TRIGGER `+name+` ON `+table)
				mustExec(t, m.f.base.owner, `DROP FUNCTION public.`+name+`()`)
				mustExec(t, m.f.base.owner, `DROP SEQUENCE public.`+name+`_hits`)
			})
			if _, e := m.create(m.f.token, m.f.store, t04Key("ma-fault"), maInput()); !errors.Is(e, command.ErrInvalid) {
				t.Fatal("fault accepted")
			}
			var fired bool
			if e := m.f.base.owner.QueryRow(context.Background(), `SELECT is_called FROM public.`+name+`_hits`).Scan(&fired); e != nil || fired {
				t.Fatalf("retired create reached SQL write target: %v", e)
			}
			if m.facts(t) != before {
				t.Fatal("fault left partial credential aggregate")
			}
		})
	}
}

func TestMerchantAccountsReplayConflictsAndAuthorization(t *testing.T) {
	m := maSetup(t)
	in := maInput()
	key := t04Key("ma-replay")
	a := maHistorical(t, m, m.f.store, in, 1)
	var e error
	before := m.facts(t)
	changed := in
	changed.Credentials.HashKey = strings.Repeat("X", 32)
	if _, e = m.create(m.f.token, m.f.store, key, changed); !errors.Is(e, command.ErrInvalid) {
		t.Fatalf("changed secret replay: %v", e)
	}
	if _, e = m.create(m.f.token, m.f.store, t04Key("ma-duplicate"), in); !errors.Is(e, command.ErrInvalid) {
		t.Fatalf("duplicate account: %v", e)
	}
	if m.facts(t) != before {
		t.Fatal("conflict left orphan binding/receipt")
	}
	if _, e = m.create(m.f.otherToken, m.f.store, key, in); e == nil {
		t.Fatal("read-only principal replay accepted")
	}
	if _, e = m.get(m.f.missingPermission, m.f.store, a.ID); e == nil {
		t.Fatal("missing read permission accepted")
	}
	if _, e = m.get(m.f.token, m.f.otherStore, a.ID); !errors.Is(e, command.ErrNotFound) {
		t.Fatalf("foreign store read: %v", e)
	}
	foreign := maSetup(t)
	if _, e = foreign.get(foreign.f.token, foreign.f.store, a.ID); !errors.Is(e, command.ErrNotFound) {
		t.Fatalf("foreign tenant read: %v", e)
	}
	e = m.f.scoped(context.Background(), m.f.token, m.f.store, func(tx pgx.Tx, s platform.Scope) error {
		s.StoreID = m.f.otherStore
		_, e := m.service.Get(context.Background(), tx, s, m.f.token, a.ID)
		return e
	})
	if e == nil {
		t.Fatal("forged scope accepted")
	}
	mustExec(t, m.f.base.owner, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND principal_id=$2 AND permission='integration:manage'`, m.f.tenant, m.f.principal)
	if _, e = m.create(m.f.token, m.f.store, key, in); e == nil {
		t.Fatal("revoked manage permission replayed")
	}
	if m.facts(t) != before {
		t.Fatal("denied authorization wrote state")
	}
}

func TestMerchantAccountsConcurrentReplayAndRotationCAS(t *testing.T) {
	m := maSetup(t)
	in := maInput()
	a := maHistorical(t, m, m.f.store, in, 1)
	before, snapshot := m.facts(t), maHistorySnapshot(t, m)
	key := t04Key("ma-concurrent")
	var results [4]accounts.Connection
	var errs [4]error
	var wg sync.WaitGroup
	for _, rotate := range []bool{false, true} {
		for i := range results {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				if rotate {
					results[i], errs[i] = m.rotate(m.f.token, key, accounts.RotateInput{ConnectionID: a.ID, ExpectedVersion: 1, Credentials: in.Credentials})
				} else {
					results[i], errs[i] = m.create(m.f.token, m.f.store, key, in)
				}
			}(i)
		}
		wg.Wait()
		for i := range results {
			if !errors.Is(errs[i], command.ErrInvalid) || results[i] != (accounts.Connection{}) {
				t.Fatalf("concurrent retired write admitted: %v", errs[i])
			}
		}
		if m.facts(t) != before || maHistorySnapshot(t, m) != snapshot {
			t.Fatal("concurrent retired writes changed credential aggregate")
		}
	}
}

func TestMerchantAccountsBindingTargetConstraints(t *testing.T) {
	m := maSetup(t)
	for _, kind := range []string{"foreign_store", "wrong_provider", "wrong_account", "wrong_environment"} {
		t.Run(kind, func(t *testing.T) {
			in := maInput()
			bindingStore, provider, asset := m.f.store, "payuni", "SANDBOX:"+in.AccountID
			switch kind {
			case "foreign_store":
				bindingStore = m.f.otherStore
			case "wrong_provider":
				provider = "mock_provider"
			case "wrong_account":
				asset = "SANDBOX:other_account"
			case "wrong_environment":
				asset = "LIVE:" + in.AccountID
			}
			var bindingID string
			e := m.f.scoped(context.Background(), m.f.token, bindingStore, func(tx pgx.Tx, s platform.Scope) error {
				b, e := m.f.service.RegisterBinding(context.Background(), tx, s, m.f.token, t04Key("ma-target"), provider, asset)
				bindingID = b.ID
				return e
			})
			if e != nil {
				t.Fatal(e)
			}
			before := m.facts(t)
			e = m.f.scoped(context.Background(), m.f.token, m.f.store, func(tx pgx.Tx, s platform.Scope) error {
				_, e := tx.Exec(context.Background(), `INSERT INTO integration.merchant_accounts(id,tenant_id,store_id,principal_id,provider,environment,account_id,binding_id,credential_version) VALUES($1,$2,$3,$4,'payuni','SANDBOX',$5,$6,1)`, randomUUID(), s.TenantID, s.StoreID, s.PrincipalID, in.AccountID, bindingID)
				return e
			})
			var pg *pgconn.PgError
			if !errors.As(e, &pg) || pg.Code != "23503" || !strings.Contains(pg.ConstraintName, "binding") {
				t.Fatalf("target FK not the rejecting cause: %v", e)
			}
			if m.facts(t) != before {
				t.Fatal("bad binding target left state")
			}
		})
	}
}

func TestMerchantAccountsRotationAtomicFaultRollback(t *testing.T) {
	for _, table := range []string{"integration.merchant_accounts", "integration.account_credentials", "ops.audit_events", "ops.command_results"} {
		t.Run(table, func(t *testing.T) {
			m := maSetup(t)
			in := maInput()
			a := maHistorical(t, m, m.f.store, in, 1)
			var e error
			before := m.facts(t)
			name := "ma_rotate_fault_" + t04Tag()
			event := "INSERT"
			if table == "integration.merchant_accounts" {
				event = "UPDATE"
			}
			mustExec(t, m.f.base.owner, `CREATE SEQUENCE public.`+name+`_hits`)
			mustExec(t, m.f.base.owner, `GRANT USAGE ON SEQUENCE public.`+name+`_hits TO commerce_runtime`)
			mustExec(t, m.f.base.owner, `CREATE FUNCTION public.`+name+`() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF current_setting('app.tenant_id',true)=`+quoteLiteral(m.f.tenant)+` THEN PERFORM nextval('public.`+name+`_hits'); RAISE EXCEPTION 'credential rotation synthetic fault'; END IF; RETURN NEW; END $$`)
			mustExec(t, m.f.base.owner, `CREATE TRIGGER `+name+` BEFORE `+event+` ON `+table+` FOR EACH ROW EXECUTE FUNCTION public.`+name+`()`)
			t.Cleanup(func() {
				mustExec(t, m.f.base.owner, `DROP TRIGGER `+name+` ON `+table)
				mustExec(t, m.f.base.owner, `DROP FUNCTION public.`+name+`()`)
				mustExec(t, m.f.base.owner, `DROP SEQUENCE public.`+name+`_hits`)
			})
			_, e = m.rotate(m.f.token, t04Key("ma-rotatefault"), accounts.RotateInput{ConnectionID: a.ID, ExpectedVersion: 1, Credentials: accounts.Credentials{HashKey: "new_key", HashIV: "new_iv"}})
			if !errors.Is(e, command.ErrInvalid) {
				t.Fatal("retired rotation did not refuse before SQL")
			}
			var fired bool
			if e = m.f.base.owner.QueryRow(context.Background(), `SELECT is_called FROM public.`+name+`_hits`).Scan(&fired); e != nil || fired {
				t.Fatalf("retired rotation reached SQL write target: %v", e)
			}
			if m.facts(t) != before {
				t.Fatal("failed rotation left partial aggregate")
			}
			got, e := m.get(m.f.token, m.f.store, a.ID)
			if e != nil || got.CredentialVersion != 1 {
				t.Fatal("failed rotation changed head")
			}
			maAssertPersistedSecret(t, m, a, 1, in.Credentials)
		})
	}
}
