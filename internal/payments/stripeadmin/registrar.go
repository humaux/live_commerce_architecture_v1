// registrar.go: the five registry operations (contracts/stripe-psp-v1.md §0.2 registrar SQL names,
// §13, integrator rulings 7, 8 and 10 of docs/delivery/units/stripe-b1-rulings.md).
// Ordering rule for every operation: validate input, then provider verification, then seal, then
// exactly one SQL call. Nothing is written when verification fails.
package stripeadmin

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/command"
	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/integrations/psp/stripe"
	"livecommerce/internal/platform"
)

// Fixed errors: they never wrap a driver, Stripe or SQL message (DSNs, keys and rows can hide in them).
var (
	ErrConfig   = errors.New("stripeadmin: config")
	ErrDatabase = errors.New("stripeadmin: database")
	ErrRejected = errors.New("stripeadmin: rejected")
	ErrProvider = errors.New("stripeadmin: provider")
)

const (
	sqlBudget       = 15 * time.Second
	qualifyBudget   = 60 * time.Second // three Stripe calls of up to 10 s each plus SQL
	environment     = "SANDBOX"        // B1 registers SANDBOX only; the SQL CHECK refuses LIVE independently
	qualifyValidFor = "30 days"        // §0.2: expiry in (now, observed_at + 30 days]
)

var (
	accountPattern = regexp.MustCompile(`^acct_[A-Za-z0-9]{1,59}$`)
	countryPattern = regexp.MustCompile(`^[A-Z]{2}$`)
)

// Scope is the operator-chosen owner scope; the SQL validates the owner membership of Principal.
type Scope struct{ TenantID, StoreID, PrincipalID string }

// EndpointInput.AccountID is optional: the account is derived from the registered connection
// (§0.2); a non-empty AccountID is only an operator cross-check and must equal it.
type EndpointInput struct {
	ConnectionID, EndpointID, AccountID, Profile string
	ExpectedVersion                              int64 // EndpointID "" iff ExpectedVersion==0
	Enabled                                      bool
	Secrets                                      accounts.StripeWebhookSecrets
}

type QualifyInput struct {
	ConnectionID, AccountID, SecretKey, Profile, Currency, ReturnURL string
	ExpectedVersion, AmountMinor                                     int64
}

type MethodInput struct {
	MarketID, Country, ConnectionID, QualificationID string
	ExpectedVersion                                  int64
	Enabled, Visible                                 bool
	Sort                                             int32
	MinMinor, MaxMinor                               int64
	NameHans, NameHant, NameEN                       string
}

type queryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Registrar owns its pool. apiKeys seals Stripe API keys, signingKeys seals webhook secrets; either
// may be nil when the command never needs it (the operation then fails with ErrConfig).
type Registrar struct {
	db          queryRower
	closePool   func()
	apiKeys     *accounts.Keyring
	signingKeys *accounts.Keyring
	transport   http.RoundTripper
}

func (Registrar) String() string     { return "stripeadmin.Registrar{redacted}" }
func (r Registrar) GoString() string { return r.String() }
func (Registrar) MarshalJSON() ([]byte, error) {
	return []byte(`"stripeadmin.Registrar{redacted}"`), nil
}

// Open connects with platform.OpenStripeRegistrarPool (masked errors). mockTransport is for tests
// and the PROVIDER_MOCK assembly only: none, or exactly one non-nil (stripe.NewWithMockTransport);
// the deployable CLI never passes one, so it always dials api.stripe.com through stripe.New.
func Open(ctx context.Context, dsn string, apiKeys, signingKeys *accounts.Keyring,
	mockTransport ...http.RoundTripper) (*Registrar, error) {
	if ctx == nil || len(mockTransport) > 1 || (len(mockTransport) == 1 && mockTransport[0] == nil) {
		return nil, ErrConfig
	}
	pool, err := platform.OpenStripeRegistrarPool(ctx, dsn)
	if err != nil {
		return nil, ErrDatabase
	}
	r := newRegistrar(pool, apiKeys, signingKeys, mockTransport...)
	r.closePool = pool.Close
	return r, nil
}

func newRegistrar(db queryRower, apiKeys, signingKeys *accounts.Keyring, mockTransport ...http.RoundTripper) *Registrar {
	r := &Registrar{db: db, apiKeys: apiKeys, signingKeys: signingKeys}
	if len(mockTransport) == 1 {
		r.transport = mockTransport[0]
	}
	return r
}

// Close releases the owned pool; safe on nil and repeatable.
func (r *Registrar) Close() {
	if r != nil && r.closePool != nil {
		r.closePool()
		r.closePool = nil
	}
}

func validScope(s Scope) bool {
	return command.ValidID(s.TenantID) && command.ValidID(s.StoreID) && command.ValidID(s.PrincipalID)
}

func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", ErrConfig
	}
	b[6], b[8] = b[6]&0x0f|0x40, b[8]&0x3f|0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// sqlError collapses a driver error into the fixed vocabulary. 22023 (invalid input), PT409
// (state/version conflict) and 42501 (scope/ACL refusal) are the definers' rejections; class 23
// (unique/check/FK, e.g. a second account for one store) is also an operator-input rejection.
func sqlError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		if pg.Code == "22023" || pg.Code == "PT409" || pg.Code == "42501" || (len(pg.Code) == 5 && pg.Code[:2] == "23") {
			return ErrRejected
		}
	}
	return ErrDatabase
}

func (r *Registrar) scan(ctx context.Context, dest any, sql string, args ...any) error {
	bounded, cancel := context.WithTimeout(ctx, sqlBudget)
	defer cancel()
	if err := r.db.QueryRow(bounded, sql, args...).Scan(dest); err != nil {
		return sqlError(err)
	}
	return nil
}

// providerClient builds the Stage-A client for the operator's key. LIVE keys are refused by the
// adapter's admission matrix (§5.2) and surfaced as ErrRejected.
func (r *Registrar) providerClient(secretKey, accountID string) (*stripe.Client, error) {
	cfg := stripe.Config{SecretKey: secretKey, AccountID: accountID, Environment: environment}
	var c *stripe.Client
	var err error
	if r.transport != nil {
		c, err = stripe.NewWithMockTransport(cfg, r.transport)
	} else {
		c, err = stripe.New(cfg)
	}
	switch {
	case errors.Is(err, stripe.ErrLiveRefused):
		return nil, ErrRejected
	case err != nil:
		return nil, ErrConfig
	}
	return c, nil
}

// verify implements ruling 8: GET /v1/account with the supplied key must return the operator's
// account id. The adapter admits only test keys in SANDBOX, so livemode=false holds by construction.
func (r *Registrar) verify(ctx context.Context, secretKey, accountID string) error {
	c, err := r.providerClient(secretKey, accountID)
	if err != nil {
		return err
	}
	if _, err := c.VerifyAccount(ctx); err != nil {
		if errors.Is(err, stripe.ErrAuthentication) {
			return ErrRejected // key belongs to another account or is invalid: nothing may be written
		}
		return ErrProvider
	}
	return nil
}

// Register verifies the account with its key, then atomically creates the binding, account and
// credential version 1 (integration.register_stripe_account; owner membership checked in SQL).
func (r *Registrar) Register(ctx context.Context, s Scope, accountID, secretKey string) (string, error) {
	if r == nil || r.db == nil || ctx == nil || r.apiKeys == nil || !validScope(s) || !accountPattern.MatchString(accountID) {
		return "", ErrConfig
	}
	connection, err := newUUID()
	if err != nil {
		return "", err
	}
	binding, err := newUUID()
	if err != nil {
		return "", err
	}
	bounded, cancel := context.WithTimeout(ctx, qualifyBudget)
	defer cancel()
	if err := r.verify(bounded, secretKey, accountID); err != nil {
		return "", err
	}
	keyID, nonce, ciphertext, err := r.apiKeys.SealStripeAPI(accounts.StripeAPIScope{TenantID: s.TenantID,
		StoreID: s.StoreID, ConnectionID: connection, Environment: environment, AccountID: accountID,
		CredentialVersion: 1}, accounts.StripeAPICredentials{SecretKey: secretKey})
	if err != nil {
		return "", ErrConfig
	}
	var out string
	if err := r.scan(bounded, &out, `SELECT integration.register_stripe_account($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::uuid,
		$6::text,$7::text,$8::text,$9::bytea,$10::bytea)::text`, s.TenantID, s.StoreID, s.PrincipalID,
		connection, binding, environment, accountID, keyID, nonce, ciphertext); err != nil {
		return "", err
	}
	if out != connection {
		return "", ErrDatabase
	}
	return connection, nil
}

// Rotate appends credential version expectedVersion+1 (integration.rotate_stripe_key locks the
// account and requires the exact previous version). The registrar cannot read the registered
// account id, so accountID is only proven to own the key; a mismatched id yields an envelope the
// worker later refuses (AAD binds the registered account), never a wrong-merchant charge.
func (r *Registrar) Rotate(ctx context.Context, s Scope, connectionID string, expectedVersion int64,
	accountID, secretKey string) (int64, error) {
	if r == nil || r.db == nil || ctx == nil || r.apiKeys == nil || !validScope(s) || !command.ValidID(connectionID) ||
		expectedVersion < 1 || !accountPattern.MatchString(accountID) {
		return 0, ErrConfig
	}
	bounded, cancel := context.WithTimeout(ctx, qualifyBudget)
	defer cancel()
	if err := r.verify(bounded, secretKey, accountID); err != nil {
		return 0, err
	}
	keyID, nonce, ciphertext, err := r.apiKeys.SealStripeAPI(accounts.StripeAPIScope{TenantID: s.TenantID,
		StoreID: s.StoreID, ConnectionID: connectionID, Environment: environment, AccountID: accountID,
		CredentialVersion: expectedVersion + 1}, accounts.StripeAPICredentials{SecretKey: secretKey})
	if err != nil {
		return 0, ErrConfig
	}
	var version int64
	if err := r.scan(bounded, &version, `SELECT integration.rotate_stripe_key($1::uuid,$2::uuid,$3::uuid,$4::uuid,
		$5::bigint,$6::text,$7::bytea,$8::bytea)`, s.TenantID, s.StoreID, s.PrincipalID, connectionID,
		expectedVersion, keyID, nonce, ciphertext); err != nil {
		return 0, err
	}
	if version != expectedVersion+1 {
		return 0, ErrDatabase // the sealed AAD bound expected+1; anything else is unreadable
	}
	return version, nil
}

// SetWebhookEndpoint seals the signing secrets under key_version expected+1 and calls
// payments.set_stripe_webhook_endpoint. Disabling re-seals too, so a caller must supply secrets.
// The AAD account comes from payments.stripe_endpoint_account (the registered connection, §0.2),
// the same value set_stripe_webhook_endpoint stores, so the envelope always opens at ingress; no
// process env account is trusted here (least privilege: the CLI webhook reads no STRIPE_ACCOUNT_ID).
func (r *Registrar) SetWebhookEndpoint(ctx context.Context, s Scope, in EndpointInput) (string, int64, error) {
	if r == nil || r.db == nil || ctx == nil || r.signingKeys == nil || !validScope(s) ||
		!command.ValidID(in.ConnectionID) || (in.AccountID != "" && !accountPattern.MatchString(in.AccountID)) ||
		(in.Profile != "PROVIDER_MOCK" && in.Profile != "SANDBOX") || in.ExpectedVersion < 0 ||
		(in.ExpectedVersion == 0) != (in.EndpointID == "") || (in.EndpointID != "" && !command.ValidID(in.EndpointID)) {
		return "", 0, ErrConfig
	}
	endpoint := in.EndpointID
	if endpoint == "" {
		var err error
		if endpoint, err = newUUID(); err != nil {
			return "", 0, err
		}
	}
	// payments.stripe_endpoint_account (registry_writer definer, EXECUTE registrar): the in-scope
	// connection's registered account; the connection's account is immutable (no rebind).
	var account string
	if err := r.scan(ctx, &account, `SELECT payments.stripe_endpoint_account($1::uuid,$2::uuid,$3::uuid,$4::uuid)`,
		s.TenantID, s.StoreID, s.PrincipalID, in.ConnectionID); err != nil {
		return "", 0, err
	}
	if !accountPattern.MatchString(account) {
		return "", 0, ErrDatabase
	}
	if in.AccountID != "" && in.AccountID != account {
		return "", 0, ErrRejected
	}
	next := in.ExpectedVersion + 1
	keyID, nonce, ciphertext, err := r.signingKeys.SealStripeWebhook(accounts.StripeWebhookScope{TenantID: s.TenantID,
		StoreID: s.StoreID, ConnectionID: in.ConnectionID, EndpointID: endpoint, Environment: environment,
		AccountID: account, Profile: in.Profile, KeyVersion: next}, in.Secrets)
	if err != nil {
		return "", 0, ErrConfig
	}
	var version int64
	if err := r.scan(ctx, &version, `SELECT payments.set_stripe_webhook_endpoint($1::uuid,$2::uuid,$3::uuid,$4::uuid,
		$5::uuid,$6::text,$7::bigint,$8::boolean,$9::text,$10::bytea,$11::bytea)`, s.TenantID, s.StoreID,
		s.PrincipalID, in.ConnectionID, endpoint, in.Profile, in.ExpectedVersion, in.Enabled, keyID,
		nonce, ciphertext); err != nil {
		return "", 0, err
	}
	if version != next {
		return "", 0, ErrDatabase
	}
	return endpoint, version, nil
}

// Qualify records method-qualification evidence for the credential version the operator expects.
// SANDBOX: verify the account, run stripe.ProbeCheckout (create at now+31m, expire, retrieve
// expired+unpaid+!livemode) and store evidence "stripe-probe:<session id>". PROVIDER_MOCK: no
// network, evidence "provider-mock:<qualification>". The SQL rechecks the head version after the
// probe, so an old-key probe cannot qualify a rotated key. observed_at/expires_at come from the DB
// transaction clock (now(), now()+30 days) so operator-host clock skew cannot violate the SQL's
// "not in the future" and "≤ observed+30 days" checks.
func (r *Registrar) Qualify(ctx context.Context, s Scope, in QualifyInput) (string, error) {
	if r == nil || r.db == nil || ctx == nil || !validScope(s) || !command.ValidID(in.ConnectionID) ||
		in.ExpectedVersion < 1 || (in.Profile != "PROVIDER_MOCK" && in.Profile != "SANDBOX") {
		return "", ErrConfig
	}
	qualification, err := newUUID()
	if err != nil {
		return "", err
	}
	bounded, cancel := context.WithTimeout(ctx, qualifyBudget)
	defer cancel()
	evidence := "provider-mock:" + qualification
	if in.Profile == "SANDBOX" {
		if !accountPattern.MatchString(in.AccountID) {
			return "", ErrConfig
		}
		client, err := r.providerClient(in.SecretKey, in.AccountID)
		if err != nil {
			return "", err
		}
		if _, err := client.VerifyAccount(bounded); err != nil {
			if errors.Is(err, stripe.ErrAuthentication) {
				return "", ErrRejected
			}
			return "", ErrProvider
		}
		session, _, err := client.ProbeCheckout(bounded, qualification, in.Currency, in.AmountMinor, in.ReturnURL)
		switch {
		case errors.Is(err, stripe.ErrInvalid) || errors.Is(err, stripe.ErrLiveRefused):
			return "", ErrRejected
		case err != nil:
			return "", ErrProvider
		}
		evidence = "stripe-probe:" + session
	}
	var out string
	if err := r.scan(bounded, &out, `SELECT payments.qualify_stripe_method($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::uuid,
		$6::bigint,$7::text,$8::text,now(),now()+interval '`+qualifyValidFor+`')::text`, s.TenantID, s.StoreID,
		s.PrincipalID, qualification, in.ConnectionID, in.ExpectedVersion, in.Profile, evidence); err != nil {
		return "", err
	}
	if out != qualification {
		return "", ErrDatabase
	}
	return qualification, nil
}

// SetMethod appends a stripe_checkout method revision (payments.set_stripe_method). Currency is
// derived from the locked market in SQL; no caller-supplied currency exists (§0.2).
func (r *Registrar) SetMethod(ctx context.Context, s Scope, in MethodInput) (int64, error) {
	if r == nil || r.db == nil || ctx == nil || !validScope(s) || !command.ValidID(in.MarketID) ||
		!command.ValidID(in.ConnectionID) || !command.ValidID(in.QualificationID) ||
		!countryPattern.MatchString(in.Country) || in.ExpectedVersion < 0 || in.Sort < 0 || in.Sort > 1000 {
		return 0, ErrConfig
	}
	// Amount bounds are operator input, not config: the definer raises 22023 (max<min) / PT409
	// (outside the currency range) for these, which sqlError maps to ErrRejected. Refusing early
	// must keep that class so the CLI reports the same outcome with or without the round trip.
	if in.MinMinor < 1 || in.MaxMinor < in.MinMinor {
		return 0, ErrRejected
	}
	var version int64
	if err := r.scan(ctx, &version, `SELECT payments.set_stripe_method($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::text,
		$6::uuid,$7::uuid,$8::bigint,$9::boolean,$10::boolean,$11::integer,$12::bigint,$13::bigint,
		$14::text,$15::text,$16::text)`, s.TenantID, s.StoreID, s.PrincipalID, in.MarketID, in.Country,
		in.ConnectionID, in.QualificationID, in.ExpectedVersion, in.Enabled, in.Visible, in.Sort,
		in.MinMinor, in.MaxMinor, in.NameHans, in.NameHant, in.NameEN); err != nil {
		return 0, err
	}
	return version, nil
}

var _ queryRower = (*pgxpool.Pool)(nil)
