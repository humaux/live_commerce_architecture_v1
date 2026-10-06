// Purpose: API-process wiring of the PAYUNi self-serve activation (w4-02b): the LC_PAYUNI_ENABLED platform switch and the
//   verify/check/live-probe/status service handed to httpapi.Options.PayuniActivation.
// Depends on: internal/payments (Activation, SetPayuniEnabled), internal/integrations/accounts (keyring); env LC_PAYUNI_ENABLED,
//   LC_PAYUNI_NOTIFY_BASE_URL, LC_PAYUNI_VERIFY_RETURN_URL, COMMERCE_PAYMENT_PROFILE, COMMERCE_ACCOUNT_* (keyring).
// Used by: cmd/api/main.go, cmd/api/payuni_activation_test.go.
// Invariants: LC_PAYUNI_ENABLED defaults off and only gates enabling a method; the activation surface is mounted only when
//   LC_PAYUNI_NOTIFY_BASE_URL is set; a malformed value refuses startup; no value is logged.
// Status: MOCK/SANDBOX.

package main

import (
	"errors"

	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/payments"
)

var errPayuniActivationConfig = errors.New("payuni_activation_invalid_config")

// loadPayuniActivation applies the platform switch and builds the activation service. It always reads LC_PAYUNI_ENABLED (and sets
// payments.SetPayuniEnabled, default off); it reads nothing else, and returns a nil service, unless LC_PAYUNI_NOTIFY_BASE_URL is set.
func loadPayuniActivation(getenv func(string) string) (*payments.Activation, error) {
	if getenv == nil {
		return nil, errPayuniActivationConfig
	}
	enabled, err := flag(getenv("LC_PAYUNI_ENABLED"))
	if err != nil {
		return nil, errPayuniActivationConfig
	}
	payments.SetPayuniEnabled(enabled)
	base := getenv("LC_PAYUNI_NOTIFY_BASE_URL")
	if base == "" {
		return nil, nil
	}
	keys, err := accounts.LoadKeyring(getenv)
	if err != nil {
		return nil, errPayuniActivationConfig
	}
	svc, err := payments.NewActivation(keys, payments.ActivationConfig{Profile: getenv("COMMERCE_PAYMENT_PROFILE"),
		NotifyBaseURL: base, ReturnURL: getenv("LC_PAYUNI_VERIFY_RETURN_URL")})
	if err != nil {
		return nil, errPayuniActivationConfig
	}
	return svc, nil
}
