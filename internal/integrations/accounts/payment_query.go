package accounts

import (
	"context"
	"errors"
	"math"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/command"
	"livecommerce/internal/integrations/psp/payuni"
)

var ErrPaymentQueryMaterial = errors.New("payment_query_material_unavailable")

// PaymentQueryMaterial contains only a redacted client and immutable trade
// expectations. The historical credential never leaves the client boundary.
// When the authenticated SQL row is valid but local materialization fails,
// only Age is returned alongside ErrPaymentQueryMaterial for budget handling.
type PaymentQueryMaterial struct {
	Client   *payuni.Client
	Expected payuni.ExpectedTrade
	Age      time.Duration
}

func (k *Keyring) LoadPaymentQuery(ctx context.Context, tx pgx.Tx, operationID string,
	generation int64, token []byte, profile string, mockTransport ...http.RoundTripper) (PaymentQueryMaterial, error) {
	if ctx == nil || tx == nil || k == nil || !command.ValidID(operationID) || generation < 1 ||
		len(token) != 32 || !validQueryProfile(profile) || len(mockTransport) > 1 ||
		(len(mockTransport) == 1 && (profile != "PROVIDER_MOCK" || mockTransport[0] == nil)) {
		return PaymentQueryMaterial{}, ErrPaymentQueryMaterial
	}
	var tenantID, storeID, connectionID, provider, environment, accountID, keyID string
	var merchantTradeNo, currency, method, providerReference string
	var version, amount int64
	var nonce, ciphertext []byte
	var ageSeconds float64
	err := tx.QueryRow(ctx, `SELECT tenant_id,store_id,connection_id,provider,environment,
		account_id,credential_version,key_id,nonce,ciphertext,merchant_trade_no,
		currency,amount_minor,method_code,provider_reference,age_seconds
		FROM integration.load_payment_query($1::uuid,$2::bigint,$3::bytea,$4::text)`,
		operationID, generation, token, profile).Scan(&tenantID, &storeID, &connectionID, &provider,
		&environment, &accountID, &version, &keyID, &nonce, &ciphertext, &merchantTradeNo,
		&currency, &amount, &method, &providerReference, &ageSeconds)
	if err != nil || math.IsNaN(ageSeconds) || math.IsInf(ageSeconds, 0) || ageSeconds < 0 ||
		ageSeconds > float64(math.MaxInt64)/float64(time.Second) ||
		method != "payuni_credit" ||
		!validQueryEnvironment(profile, environment) {
		return PaymentQueryMaterial{}, ErrPaymentQueryMaterial
	}
	material := PaymentQueryMaterial{Age: time.Duration(ageSeconds * float64(time.Second))}
	amountTWD, err := payuni.AmountTWDFromMinor(currency, amount)
	if err != nil || amountTWD > 199999 {
		return material, ErrPaymentQueryMaterial
	}
	aad := credentialAAD{FormatVersion: 1, TenantID: tenantID, StoreID: storeID,
		ConnectionID: connectionID, Provider: provider, Environment: environment,
		AccountID: accountID, CredentialVersion: version}
	credentials, err := k.open(aad, keyID, nonce, ciphertext)
	if err != nil {
		return material, ErrPaymentQueryMaterial
	}
	client, err := payuni.NewQuery(payuni.Config{Environment: environment, MerchantID: accountID,
		HashKey: credentials.HashKey, HashIV: credentials.HashIV}, mockTransport...)
	if err != nil {
		return material, ErrPaymentQueryMaterial
	}
	material.Client = client
	material.Expected = payuni.ExpectedTrade{
		MerTradeNo: merchantTradeNo, TradeNo: providerReference, AmountTWD: amountTWD,
		Currency: currency, Method: method}
	return material, nil
}

func validQueryProfile(profile string) bool {
	return profile == "PROVIDER_MOCK" || profile == "SANDBOX" || profile == "LIVE"
}

func validQueryEnvironment(profile, environment string) bool {
	return (profile == "PROVIDER_MOCK" && environment == "SANDBOX") || profile == environment
}
