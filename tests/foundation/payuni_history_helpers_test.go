// payuni_history_helpers_test.go seeds synthetic pre-removal account history only.
// Depends on the owner fixture and independent AES-GCM encoding; used by account/read tests.
// Never calls a retired Go writer or a provider; production permissions remain under test.
package foundation_test

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/payments"
	"livecommerce/internal/platform"
)

func maHistorical(t *testing.T, m *maFixture, store string, in accounts.CreateInput, versions int64) accounts.Connection {
	t.Helper()
	ctx := context.Background()
	id, binding := randomUUID(), randomUUID()
	tx, err := m.f.base.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO integration.bindings(id,tenant_id,store_id,principal_id,provider,external_asset_id) VALUES($1,$2,$3,$4,$5,$6)`, binding, m.f.tenant, store, m.f.principal, in.Provider, in.Environment+":"+in.AccountID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO integration.merchant_accounts(id,tenant_id,store_id,principal_id,provider,environment,account_id,binding_id,credential_version) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, m.f.tenant, store, m.f.principal, in.Provider, in.Environment, in.AccountID, binding, versions); err != nil {
		t.Fatal(err)
	}
	for version := int64(1); version <= versions; version++ {
		aad, err := json.Marshal(struct {
			FormatVersion     int    `json:"format_version"`
			TenantID          string `json:"tenant_id"`
			StoreID           string `json:"store_id"`
			ConnectionID      string `json:"connection_id"`
			Provider          string `json:"provider"`
			Environment       string `json:"environment"`
			AccountID         string `json:"account_id"`
			CredentialVersion int64  `json:"credential_version"`
		}{1, m.f.tenant, store, id, in.Provider, in.Environment, in.AccountID, version})
		if err != nil {
			t.Fatal(err)
		}
		plain, err := json.Marshal(struct {
			HashKey string `json:"hash_key"`
			HashIV  string `json:"hash_iv"`
		}{in.Credentials.HashKey, in.Credentials.HashIV})
		if err != nil {
			t.Fatal(err)
		}
		keyID := "fixture_v1"
		if version > 1 {
			keyID = "fixture_v2"
		}
		block, err := aes.NewCipher(m.keys[keyID])
		if err != nil {
			t.Fatal(err)
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			t.Fatal(err)
		}
		nonce := randomBytes(gcm.NonceSize())
		if _, err = tx.Exec(ctx, `INSERT INTO integration.account_credentials(tenant_id,store_id,connection_id,version,key_id,nonce,ciphertext,principal_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, m.f.tenant, store, id, version, keyID, nonce, gcm.Seal(nil, nonce, plain, aad), m.f.principal); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	out, err := m.get(m.f.token, store, id)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// Full row values catch in-place rewrites as well as inserts/deletes; counts alone cannot.
func maHistorySnapshot(t *testing.T, m *maFixture) string {
	t.Helper()
	var out string
	err := m.f.base.owner.QueryRow(context.Background(), `SELECT jsonb_build_array(
 (SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY id),'[]') FROM integration.bindings x WHERE tenant_id=$1),
 (SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY id),'[]') FROM integration.merchant_accounts x WHERE tenant_id=$1),
 (SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY connection_id,version),'[]') FROM integration.account_credentials x WHERE tenant_id=$1),
 (SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text),'[]') FROM ops.command_results x WHERE tenant_id=$1),
 (SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text),'[]') FROM ops.audit_events x WHERE tenant_id=$1),
 (SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY id),'[]') FROM integration.operations x WHERE tenant_id=$1)
 )::text`, m.f.tenant).Scan(&out)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// pmHistorical inserts immutable pre-removal method versions, never invokes SetMethod.
func pmHistorical(t *testing.T, p *pmFixture, in payments.MethodInput, version int64) payments.Method {
	t.Helper()
	ctx := context.Background()
	f := p.m.f
	tx, err := f.base.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO payments.method_versions(tenant_id,store_id,market_id,country,code,version,provider,environment,connection_id,binding_version,currency,name_hans,name_hant,name_en,enabled,visible,sort_order,min_amount_minor,max_amount_minor,principal_id)
 VALUES($1,$2,$3,$4,$5,$6,'payuni',$7,nullif($8,'')::uuid,nullif($9::bigint,0),'TWD',$10,$11,$12,$13,$14,$15,$16,$17,$18)`, f.tenant, f.store, in.MarketID, in.Country, in.Code, version, in.Environment, in.ConnectionID, in.BindingVersion, in.NameHans, in.NameHant, in.NameEN, in.Enabled, in.Visible, in.SortOrder, in.MinAmountMinor, in.MaxAmountMinor, f.principal); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO payments.method_heads(tenant_id,store_id,market_id,country,code,current_version) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(tenant_id,store_id,market_id,country,code) DO UPDATE SET current_version=EXCLUDED.current_version`, f.tenant, f.store, in.MarketID, in.Country, in.Code, version); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var out payments.Method
	err = f.scoped(ctx, f.token, f.store, func(tx pgx.Tx, s platform.Scope) error {
		var err error
		out, err = payments.GetMethod(ctx, tx, s, f.token, in.MarketID, in.Country, in.Code)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func pmHistorySnapshot(t *testing.T, p *pmFixture) string {
	t.Helper()
	var out string
	if err := p.m.f.base.owner.QueryRow(context.Background(), `SELECT jsonb_build_array(
 (SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY market_id,country,code,version),'[]') FROM payments.method_versions x WHERE tenant_id=$1),
 (SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY market_id,country,code),'[]') FROM payments.method_heads x WHERE tenant_id=$1)
 )::text`, p.m.f.tenant).Scan(&out); err != nil {
		t.Fatal(err)
	}
	return maHistorySnapshot(t, p.m) + out
}

// rm2ArchivedCapture supplies an already-recorded historical money fact for the
// unsupported-provider refund guard. It neither queries nor applies a payment.
// The order intentionally stays pending: refund admission must reject this old
// provider even when a CAPTURED fact exists. FK/hash/check constraints stay on.
func rm2ArchivedCapture(t *testing.T, q pqFixture) {
	rm2ArchivedFact(t, q, "CAPTURED")
}

// rm2ArchivedFact also preserves historical authorization-only order projections.
func rm2ArchivedFact(t *testing.T, q pqFixture, kind string) {
	t.Helper()
	ctx := context.Background()
	tx, err := q.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if kind != "AUTHORIZED" && kind != "CAPTURED" {
		t.Fatal("invalid archived fact kind")
	}
	reportMap := pcFull(q)
	if kind == "AUTHORIZED" {
		reportMap["CloseStatus"] = "9"
		delete(reportMap, "CloseAmountTWD")
	}
	report := pqJSON(reportMap)
	if _, err = tx.Exec(ctx, `INSERT INTO payments.provider_observations(tenant_id,store_id,attempt_id,source,execution_profile,environment,first_generation,report,report_hash)
 SELECT tenant_id,store_id,id,'QUERY',execution_profile,environment,2,$2::jsonb,sha256(convert_to($2::jsonb::text,'UTF8')) FROM checkout.payment_attempts WHERE id=$1`, q.result.AttemptID, report); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO payments.facts(tenant_id,store_id,attempt_id,kind,amount_minor,currency,provider_reference,connection_id,execution_profile,environment,source_report_hash)
 SELECT tenant_id,store_id,id,$3,amount_minor,currency,'archived_'||replace(id::text,'-',''),connection_id,execution_profile,environment,sha256(convert_to($2::jsonb::text,'UTF8')) FROM checkout.payment_attempts WHERE id=$1`, q.result.AttemptID, report, kind); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}
