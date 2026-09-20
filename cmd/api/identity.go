package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"livecommerce/internal/identity"
	"livecommerce/internal/identityhttp"
	"livecommerce/internal/oidclogin"
	"livecommerce/internal/platform"
)

type identityConfig struct {
	enabled                   bool
	dsn, bffKey, publicOrigin string
	provider                  oidclogin.Config
	policy                    identity.Policy
}

func flag(value string) (bool, error) {
	switch value {
	case "", "0":
		return false, nil
	case "1":
		return true, nil
	default:
		return false, errors.New("invalid boolean configuration")
	}
}

// The getenv seam proves disabled identity cannot accidentally read secrets,
// open its authority pool or perform provider discovery. No config is logged.
func loadIdentityConfig(getenv func(string) string) (identityConfig, error) {
	var c identityConfig
	var err error
	c.enabled, err = flag(getenv("COMMERCE_IDENTITY_ENABLED"))
	if err != nil || !c.enabled {
		return c, err
	}
	allowLoopback, err := flag(getenv("COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS"))
	if err != nil {
		return c, err
	}
	c.publicOrigin, err = publicOrigin(getenv("COMMERCE_PUBLIC_ORIGIN"), allowLoopback)
	if err != nil {
		return c, err
	}
	c.dsn = getenv("COMMERCE_IDENTITY_DATABASE_URL")
	c.bffKey = getenv("COMMERCE_BFF_KEY")
	c.provider = oidclogin.Config{
		Issuer: getenv("COMMERCE_OIDC_ISSUER"), ClientID: getenv("COMMERCE_OIDC_CLIENT_ID"),
		ClientSecret: getenv("COMMERCE_OIDC_CLIENT_SECRET"), RedirectURL: c.publicOrigin + "/api/auth/callback",
		AllowLoopbackForTests: allowLoopback,
	}
	c.policy.ProviderKey = getenv("COMMERCE_IDENTITY_PROVIDER_KEY")
	c.policy.SessionTTL, err = time.ParseDuration(getenv("COMMERCE_SESSION_TTL"))
	if err != nil || c.policy.SessionTTL < 5*time.Minute || c.policy.SessionTTL > 24*time.Hour {
		return c, errors.New("invalid session duration")
	}
	c.policy.OnboardingEnabled, err = flag(getenv("COMMERCE_ONBOARDING_ENABLED"))
	if err != nil {
		return c, err
	}
	if currencies := getenv("COMMERCE_ONBOARDING_CURRENCIES"); currencies != "" {
		for _, currency := range strings.Split(currencies, ",") {
			currency = strings.TrimSpace(currency)
			if len(currency) != 3 || strings.IndexFunc(currency, func(r rune) bool { return r < 'A' || r > 'Z' }) != -1 {
				return c, errors.New("invalid onboarding currency")
			}
			c.policy.Currencies = append(c.policy.Currencies, currency)
		}
	}
	if c.dsn == "" || !identityhttp.ValidSecret(c.bffKey) || c.provider.Issuer == "" || c.provider.ClientID == "" || len(c.policy.ProviderKey) == 0 || len(c.policy.ProviderKey) > 128 || (c.policy.OnboardingEnabled && len(c.policy.Currencies) == 0) {
		return c, errors.New("incomplete identity configuration")
	}
	if getenv("COMMERCE_FIXTURE_ENABLED") != "" && getenv("COMMERCE_FIXTURE_ENABLED") != "0" {
		return c, errors.New("identity and fixture modes are mutually exclusive")
	}
	return c, nil
}

func publicOrigin(raw string, allowLoopback bool) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Opaque != "" || u.User != nil || (u.Path != "" && u.Path != "/") || strings.ContainsAny(raw, "?#") || strings.IndexFunc(raw, func(r rune) bool { return r > 127 || r <= 32 }) != -1 {
		return "", errors.New("invalid public origin")
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && allowLoopback && loopbackHost(u.Hostname())) {
		return "", errors.New("public origin requires HTTPS")
	}
	host := strings.ToLower(u.Host)
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return "", errors.New("invalid public origin port")
		}
		if (u.Scheme == "https" && n == 443) || (u.Scheme == "http" && n == 80) {
			host = strings.TrimSuffix(host, ":"+port)
		}
	}
	return u.Scheme + "://" + host, nil
}

func loopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	addr, err := netip.ParseAddr(host)
	return err == nil && addr.Zone() == "" && addr.IsLoopback()
}

func privateIdentityAddress(addr string) bool {
	host, port, err := net.SplitHostPort(addr)
	if err != nil || host == "localhost" || !loopbackHost(host) {
		return false
	}
	n, err := strconv.Atoi(port)
	return err == nil && n > 0 && n <= 65535
}

func buildIdentityHandler(ctx context.Context, c identityConfig) (http.Handler, func(), error) {
	if !c.enabled {
		return nil, func() {}, nil
	}
	// This pool is never supplied to platform/httpapi. It can execute only the
	// bounded identity functions; the existing provider library validates tokens.
	pool, err := platform.OpenIdentityPool(ctx, c.dsn)
	if err != nil {
		return nil, nil, err
	}
	provider, err := oidclogin.New(ctx, c.provider)
	if err != nil {
		pool.Close()
		return nil, nil, err
	}
	s, err := identity.New(pool, provider, c.policy)
	if err != nil {
		pool.Close()
		return nil, nil, err
	}
	handler, err := identityhttp.NewHandler(s, c.bffKey)
	if err != nil {
		pool.Close()
		return nil, nil, err
	}
	return handler, pool.Close, nil
}
