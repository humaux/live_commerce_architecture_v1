package accounts

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/command"
	"livecommerce/internal/integrations/psp/payuni"
)

var ErrPaymentHostedMaterial = errors.New("hosted payment material unavailable")

// HostedConfig contains only server-owned callback endpoints, never buyer input.
type HostedConfig struct {
	ReturnURL string
	NotifyURL string
}

type BuiltHostedPayment struct {
	Form       payuni.HostedForm
	PreparedAt time.Time
	ExpiresAt  time.Time
}

// CanonicalDigest binds a prepared page to a versioned, normalized endpoint pair.
func (c HostedConfig) CanonicalDigest() (HostedConfig, [32]byte, error) {
	returnURL, ok := canonicalHostedURL(c.ReturnURL)
	if !ok {
		return HostedConfig{}, [32]byte{}, command.ErrInvalid
	}
	notifyURL, ok := canonicalHostedURL(c.NotifyURL)
	if !ok {
		return HostedConfig{}, [32]byte{}, command.ErrInvalid
	}
	canonical := HostedConfig{ReturnURL: returnURL, NotifyURL: notifyURL}
	body, _ := json.Marshal(struct {
		Version   string `json:"version"`
		ReturnURL string `json:"return_url"`
		NotifyURL string `json:"notify_url"`
	}{"payuni-hosted-v1", canonical.ReturnURL, canonical.NotifyURL})
	return canonical, sha256.Sum256(body), nil
}

func canonicalHostedURL(raw string) (string, bool) {
	if len(raw) == 0 || len(raw) > 2048 || strings.ContainsAny(raw, " \r\n\t\\%#") {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Opaque != "" || u.Host == "" || u.User != nil ||
		u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawFragment != "" || u.RawPath != "" ||
		u.Path == "" || !strings.HasPrefix(u.Path, "/") || path.Clean(u.Path) != u.Path {
		return "", false
	}
	host := strings.ToLower(u.Hostname())
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") ||
		net.ParseIP(host) != nil || !strings.Contains(host, ".") ||
		(strings.ToLower(u.Host) != host && strings.ToLower(u.Host) != host+":443") {
		return "", false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", false
		}
		for _, ch := range label {
			if ch != '-' && (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') {
				return "", false
			}
		}
	}
	last := host[strings.LastIndexByte(host, '.')+1:]
	if len(last) < 2 {
		return "", false
	}
	for _, ch := range last {
		if ch < 'a' || ch > 'z' {
			return "", false
		}
	}
	u.Host = host
	return u.String(), true
}

// BuildPaymentHosted decrypts historical material only inside the owned start
// transaction. Neither the credentials nor the signing client cross this API.
func (k *Keyring) BuildPaymentHosted(ctx context.Context, tx pgx.Tx, tokenHash []byte, storeID, orderID, profile, locale string,
	config HostedConfig) (BuiltHostedPayment, error) {
	canonical, _, err := config.CanonicalDigest()
	if ctx == nil || tx == nil || k == nil || err != nil || len(tokenHash) != 32 ||
		!command.ValidID(storeID) || !command.ValidID(orderID) ||
		!validHostedProfile(profile) || !validHostedLocale(locale) {
		return BuiltHostedPayment{}, ErrPaymentHostedMaterial
	}
	var tenantID, ownedStoreID, connectionID, provider, environment, accountID string
	var keyID, merchantTradeNo, currency, methodCode string
	var version, amountMinor int64
	var nonce, ciphertext []byte
	var preparedAt, expiresAt time.Time
	err = tx.QueryRow(ctx, `SELECT tenant_id,store_id,connection_id,provider,environment,
		account_id,credential_version,key_id,nonce,ciphertext,merchant_trade_no,
		currency,amount_minor,method_code,prepared_at,expires_at
		FROM checkout.load_hosted_material($1::bytea,$2::uuid,$3::uuid,$4::text,$5::text)`,
		tokenHash, storeID, orderID, profile, locale).Scan(&tenantID, &ownedStoreID, &connectionID, &provider,
		&environment, &accountID, &version, &keyID, &nonce, &ciphertext, &merchantTradeNo,
		&currency, &amountMinor, &methodCode, &preparedAt, &expiresAt)
	if err != nil {
		return BuiltHostedPayment{}, err
	}
	if ownedStoreID != storeID || provider != "payuni" || methodCode != "payuni_credit" ||
		!validHostedEnvironment(profile, environment) || preparedAt.IsZero() || expiresAt.IsZero() ||
		!expiresAt.After(preparedAt) || expiresAt.After(preparedAt.Add(time.Minute)) {
		return BuiltHostedPayment{}, ErrPaymentHostedMaterial
	}
	amountTWD, err := payuni.AmountTWDFromMinor(currency, amountMinor)
	if err != nil || amountTWD < 1 || amountTWD > 199999 {
		return BuiltHostedPayment{}, ErrPaymentHostedMaterial
	}
	aad := credentialAAD{FormatVersion: 1, TenantID: tenantID, StoreID: ownedStoreID,
		ConnectionID: connectionID, Provider: provider, Environment: environment,
		AccountID: accountID, CredentialVersion: version}
	credentials, err := k.open(aad, keyID, nonce, ciphertext)
	if err != nil {
		return BuiltHostedPayment{}, ErrPaymentHostedMaterial
	}
	client, err := payuni.New(payuni.Config{Environment: environment, MerchantID: accountID,
		HashKey: credentials.HashKey, HashIV: credentials.HashIV,
		ReturnURL: canonical.ReturnURL, NotifyURL: canonical.NotifyURL})
	if err != nil {
		return BuiltHostedPayment{}, ErrPaymentHostedMaterial
	}
	language := "zh-tw"
	if locale == "en" {
		language = "en"
	}
	form, err := client.BuildHosted(payuni.HostedRequest{MerTradeNo: merchantTradeNo,
		AmountTWD: amountTWD, Timestamp: preparedAt.Unix(), Description: "Store order",
		Method: methodCode, PageExpirySeconds: 300, Language: language})
	if err != nil {
		return BuiltHostedPayment{}, ErrPaymentHostedMaterial
	}
	return BuiltHostedPayment{Form: form, PreparedAt: preparedAt, ExpiresAt: expiresAt}, nil
}

func validHostedProfile(profile string) bool {
	return profile == "PROVIDER_MOCK" || profile == "SANDBOX" || profile == "LIVE"
}

func validHostedEnvironment(profile, environment string) bool {
	return profile == "PROVIDER_MOCK" && environment == "SANDBOX" || profile == environment
}

func validHostedLocale(locale string) bool {
	return locale == "zh-CN" || locale == "zh-TW" || locale == "en"
}
