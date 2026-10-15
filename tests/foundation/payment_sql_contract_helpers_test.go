// Responsibility: invoke retained phase-2a SQL contracts through their runtime roles.
// Depends on buyer scope and River's transactional insert; called only by A-bucket tests.
// No production PAYUNi adapter, credential decryption, encryption or provider IO exists here.
package foundation_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/buyer"
	"livecommerce/internal/checkout"
	"livecommerce/internal/command"
	"livecommerce/internal/integrations/accounts"
	integration "livecommerce/internal/integrations/core"
)

type rm2StartSQL struct {
	pool    *pgxpool.Pool
	profile string
}

func psSQLStarter(_ *testing.T, pool *pgxpool.Pool, profile string) *rm2StartSQL {
	return &rm2StartSQL{pool: pool, profile: profile}
}

func psSQLSetup(t *testing.T) psHarness {
	p := psSetup(t)
	p.sql = psSQLStarter(t, p.pool, "PROVIDER_MOCK")
	return p
}

func rm2SQLError(err error) error {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		switch pe.Code {
		case "PT401":
			return buyer.ErrUnauthorized
		case "PT404":
			return command.ErrNotFound
		case "PT409":
			return command.ErrConflict
		case "PT400", "22023":
			return command.ErrInvalid
		}
	}
	return err
}

// SQL requires the job and start to commit together. The receipt is read under
// the same SQL advisory lock as start_payment; a replay must not enqueue a job.
func rm2StartSQLTx(ctx context.Context, tx pgx.Tx, scope buyer.Scope, hash []byte, store, key string,
	in checkout.PaymentInput, profile string, digest []byte) (checkout.PaymentResult, bool, error) {
	var out checkout.PaymentResult
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`,
		"checkout.payment.start|"+scope.TenantID+"|"+store+"|"+scope.OwnerID+"|"+key); err != nil {
		return out, false, err
	}
	var savedHash, raw []byte
	err := tx.QueryRow(ctx, `SELECT request_hash,response FROM checkout.command_results
	 WHERE tenant_id=$1 AND store_id=$2 AND owner_id=$3 AND operation='checkout.payment.start' AND idempotency_key=$4`,
		scope.TenantID, store, scope.OwnerID, key).Scan(&savedHash, &raw)
	if err == nil {
		if !bytes.Equal(savedHash, digest) {
			return out, true, command.ErrConflict
		}
		// Re-resolve after the advisory-lock wait, using the database clock.
		var valid bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM buyer.resolve_scope($1,$2::uuid))`, hash, store).Scan(&valid); err != nil {
			return out, true, err
		}
		if !valid {
			return out, true, buyer.ErrUnauthorized
		}
		err = json.Unmarshal(raw, &out)
		return out, true, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return out, false, err
	}
	attempt := randomUUID()
	// Direct INSERT is intentional: no retired Go producer is invoked. The
	// database's deferred family trigger binds and routes this exact job at commit.
	var job int64
	err = tx.QueryRow(ctx, `INSERT INTO river_payment.river_job(kind,args,max_attempts,queue,scheduled_at)
	 VALUES('payment_query_v1',jsonb_build_object('operation_id',$1::text,'version',1),25,'default',clock_timestamp()+interval '5 seconds') RETURNING id`, attempt).Scan(&job)
	if err != nil {
		return out, false, err
	}
	err = tx.QueryRow(ctx, `SELECT checkout.start_payment($1,$2::uuid,$3,$4,$5::uuid,$6,$7::bigint,$8,$9::uuid,$10::bigint)`,
		hash, store, key, digest, in.OrderID, in.MethodCode, in.MethodVersion, profile, attempt, job).Scan(&raw)
	if err == nil {
		err = json.Unmarshal(raw, &out)
	}
	return out, false, err
}

func (s *rm2StartSQL) StartPayment(ctx context.Context, token, store, key string, in checkout.PaymentInput) (checkout.PaymentResult, error) {
	request, _ := json.Marshal(struct {
		Input   checkout.PaymentInput `json:"input"`
		Profile string                `json:"profile"`
	}{in, s.profile})
	digest := sha256.Sum256(request)
	var out checkout.PaymentResult
	err := buyer.WithScope(ctx, s.pool, token, store, func(ctx context.Context, tx pgx.Tx, scope buyer.Scope) error {
		var err error
		out, _, err = rm2StartSQLTx(ctx, tx, scope, tokenHash(token), store, key, in, s.profile, digest[:])
		return err
	})
	if err != nil {
		return checkout.PaymentResult{}, rm2SQLError(err)
	}
	return out, nil
}

type rm2HostedSQL struct {
	pool    *pgxpool.Pool
	profile string
	digest  []byte
}

func hpSQLStarter(t *testing.T, pool *pgxpool.Pool, profile string, _ *accounts.Keyring, config checkout.HostedConfig) *rm2HostedSQL {
	t.Helper()
	return &rm2HostedSQL{pool: pool, profile: profile, digest: hpDigest(t, config.ReturnURL, config.NotifyURL)}
}

func hpSQLSetup(t *testing.T) hpHarness {
	h := hpSetup(t)
	h.api = hpSQLStarter(t, h.pool, "PROVIDER_MOCK", nil, h.config)
	return h
}

func (s *rm2HostedSQL) BeginHosted(ctx context.Context, token, store, key string, in checkout.HostedInput) (checkout.PaymentResult, error) {
	request, _ := json.Marshal(struct {
		Input   checkout.HostedInput
		Profile string
		Config  []byte
	}{in, s.profile, s.digest})
	digest := sha256.Sum256(request)
	var out checkout.PaymentResult
	err := buyer.WithScope(ctx, s.pool, token, store, func(ctx context.Context, tx pgx.Tx, scope buyer.Scope) error {
		var replay bool
		var err error
		out, replay, err = rm2StartSQLTx(ctx, tx, scope, tokenHash(token), store, key, checkout.PaymentInput{OrderID: in.OrderID, MethodCode: in.MethodCode, MethodVersion: in.MethodVersion}, s.profile, digest[:])
		if err != nil || replay {
			return err
		}
		// Only SQL's outer-form/authority contract is under test. These bytes
		// deliberately contain no encrypted payment instruction or usable PSP form.
		var account, environment string
		if err = tx.QueryRow(ctx, `SELECT account_id,environment FROM checkout.load_hosted_material($1,$2::uuid,$3::uuid,$4,$5)`, tokenHash(token), store, in.OrderID, s.profile, in.Locale).Scan(&account, &environment); err != nil {
			return err
		}
		action := "https://sandbox-api.payuni.com.tw/api/upp"
		if environment == "LIVE" {
			action = "https://api.payuni.com.tw/api/upp"
		}
		form := checkout.HostedForm{Action: action, Fields: map[string]string{"MerID": account, "Version": "2.0", "EncryptInfo": "0000000000000000", "HashInfo": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}}
		raw, _ := json.Marshal(form)
		_, err = tx.Exec(ctx, `SELECT checkout.save_hosted_page($1,$2::uuid,$3::uuid,$4,$5,$6,$7::jsonb)`, tokenHash(token), store, in.OrderID, s.profile, in.Locale, s.digest, raw)
		return err
	})
	if err != nil {
		return checkout.PaymentResult{}, rm2SQLError(err)
	}
	return out, nil
}

func (s *rm2HostedSQL) TakeHosted(ctx context.Context, token, store, order string) (checkout.HostedHandoff, error) {
	var out checkout.HostedHandoff
	err := buyer.WithScope(ctx, s.pool, token, store, func(ctx context.Context, tx pgx.Tx, _ buyer.Scope) error {
		var raw []byte
		if err := tx.QueryRow(ctx, `SELECT checkout.take_hosted_page($1,$2::uuid,$3::uuid,$4,$5)`, tokenHash(token), store, order, s.profile, s.digest).Scan(&raw); err != nil {
			return err
		}
		return json.Unmarshal(raw, &out)
	})
	if err != nil {
		return checkout.HostedHandoff{}, rm2SQLError(err)
	}
	return out, nil
}

func (s *rm2HostedSQL) PaymentView(ctx context.Context, token, store, order string) (checkout.OrderPayment, error) {
	var out checkout.OrderPayment
	err := buyer.WithScope(ctx, s.pool, token, store, func(ctx context.Context, tx pgx.Tx, _ buyer.Scope) error {
		var raw []byte
		if err := tx.QueryRow(ctx, `SELECT checkout.hosted_payment_view($1,$2::uuid,$3::uuid,$4,$5)`, tokenHash(token), store, order, s.profile, s.digest).Scan(&raw); err != nil {
			return err
		}
		return json.Unmarshal(raw, &out)
	})
	if err != nil {
		return checkout.OrderPayment{}, rm2SQLError(err)
	}
	if out.HandoffExpiresAt != nil {
		utc := out.HandoffExpiresAt.UTC()
		out.HandoffExpiresAt = &utc
	}
	return out, nil
}

// Retained query/capture suites exercise installed SQL, rather than restoring a
// Go producer. Other suites continue to use the explicit round-2 history fixture.
func pqSQLSetup(t *testing.T, fresh bool, items int) pqFixture {
	return pqSQLSetupOn(t, fixture(t), fresh, items)
}

func pqSQLSetupOn(t *testing.T, base *testFixture, fresh bool, items int) pqFixture {
	t.Helper()
	p := psSetupItemsOn(t, base, items)
	if fresh {
		p.cap.Token, p.cap.Scope.SessionID = randomToken(), randomUUID()
		mustExec(t, p.f.owner, `INSERT INTO buyer.capability_sessions(tenant_id,store_id,owner_id,id,token_hash,expires_at) VALUES($1,$2,$3,$4,$5,clock_timestamp()+interval '1 hour')`, p.cap.Scope.TenantID, p.cap.Scope.StoreID, p.cap.Scope.OwnerID, p.cap.Scope.SessionID, tokenHash(p.cap.Token))
	}
	keys, err := accounts.NewKeyring("rm2_sql", map[string][]byte{"rm2_sql": randomBytes(32)}, randomBytes(32))
	if err != nil {
		t.Fatal(err)
	}
	service, err := accounts.New(keys, &integration.Service{})
	if err != nil {
		t.Fatal(err)
	}
	q := pqFixture{psHarness: p, schema: "river_payment", keys: keys, accountService: service}
	q.result, err = psSQLStarter(t, p.pool, "PROVIDER_MOCK").StartPayment(context.Background(), p.cap.Token, p.f.storeA1, t04Key("rm2-sql-start"), p.input)
	if err != nil {
		t.Fatal(err)
	}
	return q
}
