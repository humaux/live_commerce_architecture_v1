package main

import (
	"errors"
	"strings"
	"testing"
)

// The merchant Page connect is off unless COMMERCE_META_LOGIN_CONFIG_ID is set, and then every other value is required: one fixed
// error that never carries a value or a secret.
func TestNewMetaConnectConfig(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	if svc, err := newMetaConnect(nil, env(map[string]string{})); svc != nil || err != nil {
		t.Fatalf("unset must be off with no error: %v %v", svc, err)
	}
	if svc, err := newMetaConnect(nil, nil); svc != nil || !errors.Is(err, errMetaConnectConfig) {
		t.Fatalf("nil getenv: %v %v", svc, err)
	}
	const secret = "SENTINEL-APP-SECRET-0123456789abcdef"
	apps := `{"apps":[{"app_id":"4291253377792879","object":"page","app_secret":"` + secret + `","verify_token":"v"}]}`
	for name, m := range map[string]map[string]string{
		"no pool":           {"COMMERCE_META_LOGIN_CONFIG_ID": "2952863798433821", "COMMERCE_META_APPS_JSON": apps},
		"no apps":           {"COMMERCE_META_LOGIN_CONFIG_ID": "2952863798433821"},
		"apps without page": {"COMMERCE_META_LOGIN_CONFIG_ID": "2952863798433821", "COMMERCE_META_APPS_JSON": `{"apps":[{"app_id":"1","object":"instagram","app_secret":"x","verify_token":"v"}]}`},
		"bad config id":     {"COMMERCE_META_LOGIN_CONFIG_ID": "abc", "COMMERCE_META_APPS_JSON": apps},
	} {
		svc, err := newMetaConnect(nil, env(m))
		if svc != nil || !errors.Is(err, errMetaConnectConfig) || strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "2952863798433821") {
			t.Errorf("%s: %v %v", name, svc, err)
		}
	}
}
