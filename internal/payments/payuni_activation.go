// Purpose: PAYUNi merchant self-serve activation (w4-02b): start an NT$1 SANDBOX verification, record its authenticated query
//   evidence, run the LIVE read-only credential probe, expose the activation status, and sweep verifications for issuance.
// Depends on: internal/integrations/accounts (Keyring: opens the scoped credential envelope, derives the notify token),
//   internal/integrations/psp/payuni (hosted form, query, probe); SQL payments.start_payuni_verification /
//   load_payuni_activation_material / record_payuni_verification_query / record_payuni_live_probe /
//   issue_payuni_live_qualification / payuni_activation_status / sweep_payuni_verifications (migration 0137, SECURITY DEFINER);
//   PAYUNi hosts (sandbox-api / api.payuni.com.tw) for the query and the probe; env LC_PAYUNI_ENABLED (via SetPayuniEnabled).
// Used by: internal/httpapi/payment_activation.go (merchant routes), cmd/api (builds the service), cmd/payment-worker (sweeper),
//   tests/foundation/payuni_activation_test.go.
// Invariants: no network call inside a database transaction; proof class is never chosen by a caller (SQL derives it);
//   SANDBOX/MOCK issuance needs a CAPTURED query AND a signed notify receipt, LIVE needs a signature-verified probe; UNKNOWN
//   (timeout/transport) is never retried here; no card data and no credential is stored or returned.
// Status: MOCK/SANDBOX; the LIVE probe path exists, evidence NOT_RUN (no live key in any test).

package payments

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/command"
	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/integrations/psp/payuni"
	"livecommerce/internal/platform"
)

var (
	// ErrNotQualified: enabling needs a current, unexpired, unrevoked qualification (HTTP 409 not_qualified).
	ErrNotQualified = errors.New("payments: payment method not qualified")
	// ErrPlatformDisabled: LC_PAYUNI_ENABLED is off (HTTP 409 platform_disabled).
	ErrPlatformDisabled = errors.New("payments: payuni is not enabled on this platform")
	// ErrProfileNotAllowed: the deployment profile cannot run this activation step (HTTP 409 profile_not_allowed).
	ErrProfileNotAllowed = errors.New("payments: payuni activation step not allowed under this payment profile")
	// ErrProbeFailed: the LIVE probe did not produce an authenticated "no such trade" reply (HTTP 422 payuni_probe_failed).
	ErrProbeFailed = errors.New("payments: payuni probe failed")
)

var payuniEnabled atomic.Bool

// SetPayuniEnabled sets the platform switch (cmd/api reads LC_PAYUNI_ENABLED; default off). It gates SetMethod(Enabled) and the
// PLATFORM_DISABLED diagnostic only: verification and probing work while it is off so merchants can qualify before launch.
func SetPayuniEnabled(on bool) { payuniEnabled.Store(on) }

// PayuniEnabled reports the platform switch.
func PayuniEnabled() bool { return payuniEnabled.Load() }

// ActivationConfig is the process wiring of the activation service.
type ActivationConfig struct {
	// Profile is COMMERCE_PAYMENT_PROFILE: PROVIDER_MOCK or SANDBOX run verifications; LIVE only runs the probe.
	Profile string
	// NotifyBaseURL is the public https origin that reaches POST /v1/hooks/payuni/notify/{token} (LC_PAYUNI_NOTIFY_BASE_URL).
	NotifyBaseURL string
	// ReturnURL is the page the buyer-side browser returns to after the hosted form (LC_PAYUNI_VERIFY_RETURN_URL).
	ReturnURL string
	// QueryClient builds the query/probe client. nil means payuni.NewQuery (real PAYUNi host). Tests inject a client over a
	// MOCK transport; nothing in cmd/api sets it.
	QueryClient func(payuni.Config) (*payuni.Client, error)
}

// Activation runs the PAYUNi activation flow. Build it with NewActivation.
type Activation struct {
	keys *accounts.Keyring
	cfg  ActivationConfig
}

// NewActivation validates the wiring. It makes no database or provider call.
func NewActivation(keys *accounts.Keyring, cfg ActivationConfig) (*Activation, error) {
	if keys == nil || (cfg.Profile != "PROVIDER_MOCK" && cfg.Profile != "SANDBOX" && cfg.Profile != "LIVE") {
		return nil, command.ErrInvalid
	}
	for _, raw := range []string{cfg.NotifyBaseURL, cfg.ReturnURL} {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return nil, command.ErrInvalid
		}
	}
	if cfg.QueryClient == nil {
		cfg.QueryClient = func(c payuni.Config) (*payuni.Client, error) { return payuni.NewQuery(c) }
	}
	return &Activation{keys: keys, cfg: cfg}, nil
}

// VerificationStart is the merchant-visible result of Start: the NT$1 hosted form and the trade it pays.
type VerificationStart struct {
	VerificationID  string            `json:"verification_id"`
	MerchantTradeNo string            `json:"merchant_trade_no"`
	AmountTWD       int64             `json:"amount_twd"`
	ExpiresAt       time.Time         `json:"expires_at"`
	Replay          bool              `json:"replay"`
	FormAction      string            `json:"form_action"`
	FormFields      map[string]string `json:"form_fields"`
}

// ActivationStatus is the own-store read model of one connection's activation.
type ActivationStatus struct {
	VerificationID        string     `json:"verification_id,omitempty"`
	MerchantTradeNo       string     `json:"merchant_trade_no,omitempty"`
	VerificationExpiresAt *time.Time `json:"verification_expires_at,omitempty"`
	QueryCaptured         bool       `json:"query_captured"`
	NotifyReceived        bool       `json:"notify_received"`
	QualificationID       string     `json:"qualification_id,omitempty"`
	ProofClass            string     `json:"proof_class,omitempty"`
	QualifiedUntil        *time.Time `json:"qualified_until,omitempty"`
	CredentialVersion     int64      `json:"credential_version"`
	State                 string     `json:"state"` // CONFIGURED_UNVERIFIED | VERIFYING | VERIFIED_SANDBOX | VERIFIED_LIVE_PROBE | EXPIRED
}

// CheckResult is the outcome of one merchant-triggered query of a verification trade.
type CheckResult struct {
	Outcome string           `json:"outcome"` // CAPTURED | NOT_CAPTURED | UNAVAILABLE
	Status  ActivationStatus `json:"status"`
}

type material struct {
	scope       accounts.PayuniCredentialScope
	credentials accounts.Credentials
	tradeNo     string
	profile     string
	created     time.Time
}

// loadMaterial reads and opens the scoped credential envelope. Calls payments.load_payuni_activation_material (0137).
func (a *Activation) loadMaterial(ctx context.Context, tx pgx.Tx, connectionID, verificationID string) (material, error) {
	var m material
	var tenant, store, conn, env, account string
	var version, amount int64
	var keyID string
	var nonce, ciphertext []byte
	var verID, tradeNo, profile *string
	var created, expires *time.Time
	err := tx.QueryRow(ctx, `SELECT tenant_id::text,store_id::text,connection_id::text,environment,account_id,credential_version,
		key_id,nonce,ciphertext,verification_id::text,merchant_trade_no,execution_profile,amount_minor,created_at,expires_at
		FROM payments.load_payuni_activation_material($1::uuid,nullif($2::text,'')::uuid)`, connectionID, verificationID).
		Scan(&tenant, &store, &conn, &env, &account, &version, &keyID, &nonce, &ciphertext, &verID, &tradeNo, &profile, &amount, &created, &expires)
	if err != nil {
		return m, mapError(err)
	}
	m.scope = accounts.PayuniCredentialScope{TenantID: tenant, StoreID: store, ConnectionID: conn, Environment: env,
		AccountID: account, CredentialVersion: version}
	if m.credentials, err = a.keys.OpenPayuniActivation(m.scope, keyID, nonce, ciphertext); err != nil {
		return m, errors.New("payments: activation credential unavailable")
	}
	if tradeNo != nil && profile != nil && created != nil {
		m.tradeNo, m.profile, m.created = *tradeNo, *profile, *created
	}
	return m, nil
}

// Start begins (or replays by Idempotency-Key) one NT$1 SANDBOX verification for the merchant's own PAYUNi connection and
// returns the hosted form. Database only: it opens the sealed credential to sign the form, calls no provider.
// Calls payments.start_payuni_verification (0137); the notify endpoint token is derived from the keyring, hash stored only.
func (a *Activation) Start(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, connectionID string) (out VerificationStart, err error) {
	if a == nil || !command.ValidID(connectionID) {
		return out, command.ErrInvalid
	}
	if a.cfg.Profile == "LIVE" {
		return out, ErrProfileNotAllowed // the notify receiver never admits LIVE, so a deployment on LIVE cannot verify SANDBOX
	}
	if err = authorize(ctx, tx, scope, token, "integration:manage"); err != nil {
		return out, err
	}
	profile := a.cfg.Profile // PROVIDER_MOCK or SANDBOX
	notifyToken, tokenHash, err := a.keys.PayuniVerifyNotifyToken(connectionID, profile)
	if err != nil {
		return out, command.ErrInvalid
	}
	var verificationID, tradeNo string
	var expires time.Time
	if err = tx.QueryRow(ctx, `SELECT verification_id::text,merchant_trade_no,expires_at,replay
		FROM payments.start_payuni_verification($1::uuid,$2,$3,$4::bytea)`, connectionID, key, profile, tokenHash).
		Scan(&verificationID, &tradeNo, &expires, &out.Replay); err != nil {
		return out, mapError(err)
	}
	m, err := a.loadMaterial(ctx, tx, connectionID, verificationID)
	if err != nil {
		return out, err
	}
	notifyURL, err := url.JoinPath(a.cfg.NotifyBaseURL, "/v1/hooks/payuni/notify", notifyToken)
	if err != nil {
		return out, command.ErrInvalid
	}
	client, err := payuni.New(payuni.Config{Environment: "SANDBOX", MerchantID: m.scope.AccountID, HashKey: m.credentials.HashKey,
		HashIV: m.credentials.HashIV, ReturnURL: a.cfg.ReturnURL, NotifyURL: notifyURL})
	if err != nil {
		return out, command.ErrInvalid
	}
	form, err := client.BuildHosted(payuni.HostedRequest{MerTradeNo: tradeNo, AmountTWD: 1, Timestamp: time.Now().Unix(),
		Description: "Connection test", Method: "payuni_credit", PageExpirySeconds: 300, Language: "zh-tw"})
	if err != nil {
		return out, command.ErrInvalid
	}
	out.VerificationID, out.MerchantTradeNo, out.AmountTWD, out.ExpiresAt = verificationID, tradeNo, 1, expires
	out.FormAction, out.FormFields = form.Action, make(map[string]string, len(form.Fields))
	for k, v := range form.Fields {
		if len(v) == 1 {
			out.FormFields[k] = v[0]
		}
	}
	return out, nil
}

// Status reads the own-store activation read model of one connection (integration:read).
// Calls payments.payuni_activation_status (0137).
func (a *Activation) Status(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, connectionID string) (ActivationStatus, error) {
	if !command.ValidID(connectionID) {
		return ActivationStatus{}, command.ErrInvalid
	}
	if err := authorize(ctx, tx, scope, token, "integration:read"); err != nil {
		return ActivationStatus{}, err
	}
	return readStatus(ctx, tx, connectionID)
}

func readStatus(ctx context.Context, tx pgx.Tx, connectionID string) (out ActivationStatus, err error) {
	var verID, tradeNo, qualID, proof *string
	var vExpires, qUntil *time.Time
	var captured, notify *bool
	var verQual *string
	err = tx.QueryRow(ctx, `SELECT verification_id::text,merchant_trade_no,verification_expires_at,query_captured,notify_seen,
		verification_qualification_id::text,qualification_id::text,proof_class,qualification_expires_at,credential_version
		FROM payments.payuni_activation_status($1::uuid)`, connectionID).
		Scan(&verID, &tradeNo, &vExpires, &captured, &notify, &verQual, &qualID, &proof, &qUntil, &out.CredentialVersion)
	if err != nil {
		return out, mapError(err)
	}
	str := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	out.VerificationID, out.MerchantTradeNo, out.VerificationExpiresAt = str(verID), str(tradeNo), vExpires
	out.QueryCaptured, out.NotifyReceived = captured != nil && *captured, notify != nil && *notify
	out.QualificationID, out.ProofClass, out.QualifiedUntil = str(qualID), str(proof), qUntil
	switch {
	case out.QualificationID != "" && out.ProofClass == "REAL_LIVE":
		out.State = "VERIFIED_LIVE_PROBE"
	case out.QualificationID != "":
		out.State = "VERIFIED_SANDBOX"
	case out.VerificationID != "" && vExpires != nil && vExpires.After(time.Now()):
		out.State = "VERIFYING"
	case out.VerificationID != "":
		out.State = "EXPIRED"
	default:
		out.State = "CONFIGURED_UNVERIFIED"
	}
	return out, nil
}

// Check queries PAYUNi once for the verification trade and stores the authenticated report as evidence. Three phases so no
// provider call sits inside a database transaction: (1) open the scoped credential, (2) one query with a bounded timeout,
// (3) record the projected report. A transport/uncertain failure is UNAVAILABLE and nothing is recorded; the merchant may call again.
// Issuance is NOT done here: the payment worker issues once the signed notify receipt agrees (payments.issue_payuni_qualification).
func (a *Activation) Check(ctx context.Context, pool *pgxpool.Pool, token, storeID, connectionID, verificationID string) (CheckResult, error) {
	if a == nil || !command.ValidID(connectionID) || !command.ValidID(verificationID) {
		return CheckResult{}, command.ErrInvalid
	}
	var m material
	err := platform.WithScope(ctx, pool, token, storeID, "integration:manage", func(tx pgx.Tx, s platform.Scope) error {
		var e error
		m, e = a.loadMaterial(ctx, tx, connectionID, verificationID)
		return e
	})
	if err != nil {
		return CheckResult{}, err
	}
	client, err := a.cfg.QueryClient(payuni.Config{Environment: "SANDBOX", MerchantID: m.scope.AccountID,
		HashKey: m.credentials.HashKey, HashIV: m.credentials.HashIV})
	if err != nil {
		return CheckResult{}, command.ErrInvalid
	}
	callCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	// Calls PAYUNi /api/trade/query (payment-capture-v1 Decisions); read-only, one attempt, UNKNOWN is not retried here.
	obs, qerr := client.Query(callCtx, payuni.ExpectedTrade{MerTradeNo: m.tradeNo, AmountTWD: 1, Currency: "TWD", Method: "payuni_credit"}, time.Now().Unix())
	out := CheckResult{Outcome: "UNAVAILABLE"}
	err = platform.WithScope(ctx, pool, token, storeID, "integration:manage", func(tx pgx.Tx, s platform.Scope) error {
		if qerr == nil {
			encoded, e := json.Marshal(obs)
			if e != nil {
				return e
			}
			var captured bool
			// Calls payments.record_payuni_verification_query (0137): SQL re-derives CAPTURED; nothing is issued here.
			if e = tx.QueryRow(ctx, `SELECT payments.record_payuni_verification_query($1::uuid,$2::jsonb)`, verificationID, encoded).Scan(&captured); e != nil {
				return mapError(e)
			}
			out.Outcome = "NOT_CAPTURED"
			if captured {
				out.Outcome = "CAPTURED"
			}
		}
		st, e := readStatus(ctx, tx, connectionID)
		out.Status = st
		return e
	})
	return out, err
}

// LiveProbe proves a LIVE connection's MerID/HashKey/HashIV with one read-only query for a trade id that must not exist
// (probe.go) and, only when the reply authenticated, records the probe and issues REAL_LIVE. It starts no transaction and
// is refused unless the deployment profile is LIVE. UNKNOWN (timeout/transport) issues nothing and is not retried here.
// Calls PAYUNi /api/trade/query; payments.record_payuni_live_probe + issue_payuni_live_qualification (0137).
func (a *Activation) LiveProbe(ctx context.Context, pool *pgxpool.Pool, token, storeID, connectionID string) (ActivationStatus, error) {
	if a == nil || !command.ValidID(connectionID) {
		return ActivationStatus{}, command.ErrInvalid
	}
	if a.cfg.Profile != "LIVE" {
		return ActivationStatus{}, ErrProfileNotAllowed
	}
	var m material
	err := platform.WithScope(ctx, pool, token, storeID, "integration:manage", func(tx pgx.Tx, s platform.Scope) error {
		var e error
		m, e = a.loadMaterial(ctx, tx, connectionID, "")
		return e
	})
	if err != nil {
		return ActivationStatus{}, err
	}
	var raw [10]byte
	if _, err = rand.Read(raw[:]); err != nil {
		return ActivationStatus{}, ErrProbeFailed
	}
	probeTrade := "LP" + hex.EncodeToString(raw[:]) // 22 chars, never used to create a trade
	client, err := a.cfg.QueryClient(payuni.Config{Environment: m.scope.Environment, MerchantID: m.scope.AccountID,
		HashKey: m.credentials.HashKey, HashIV: m.credentials.HashIV})
	if err != nil {
		return ActivationStatus{}, ErrProbeFailed
	}
	callCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if perr := client.Probe(callCtx, probeTrade, time.Now().Unix()); perr != nil {
		return ActivationStatus{}, errors.Join(ErrProbeFailed, perr)
	}
	var out ActivationStatus
	err = platform.WithScope(ctx, pool, token, storeID, "integration:manage", func(tx pgx.Tx, s platform.Scope) error {
		var probeID string
		if e := tx.QueryRow(ctx, `SELECT payments.record_payuni_live_probe($1::uuid,$2,$3,true)::text`, connectionID,
			m.scope.CredentialVersion, probeTrade).Scan(&probeID); e != nil {
			return mapError(e)
		}
		if _, e := tx.Exec(ctx, `SELECT payments.issue_payuni_live_qualification($1::uuid)`, probeID); e != nil {
			return mapError(e)
		}
		var e error
		out, e = readStatus(ctx, tx, connectionID)
		return e
	})
	return out, err
}

// SweepVerifications issues qualifications for every verification whose query evidence is complete and whose signed notify
// receipt exists. Called by the payment worker (role commerce_payment_worker); it makes no provider call.
// Calls payments.sweep_payuni_verifications (0137).
func SweepVerifications(ctx context.Context, pool *pgxpool.Pool) (issued int, err error) {
	err = pool.QueryRow(ctx, `SELECT payments.sweep_payuni_verifications(50)`).Scan(&issued)
	return issued, err
}

// RunVerificationSweeper sweeps every interval until ctx ends. A failed sweep is logged with a fixed code and retried on the
// next tick (it is pure SQL over durable evidence, so retrying is safe).
func RunVerificationSweeper(ctx context.Context, pool *pgxpool.Pool, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sweepCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			if _, err := SweepVerifications(sweepCtx, pool); err != nil && ctx.Err() == nil {
				slog.Warn("payuni_verification_sweep_failed")
			}
			cancel()
		}
	}
}
