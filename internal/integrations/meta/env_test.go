package meta

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

const runtimeTestApps = `{"apps":[{"app_id":"123","object":"page","app_secret":"1234567890abcdef","verify_token":"abcdef1234567890"},{"app_id":"123","object":"instagram","app_secret":"abcdef1234567890","verify_token":"1234567890abcdef"}]}`

func runtimeTestKeys() string {
	return `{"keys":[{"id":"active","key_base64":"` + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)) + `"}]}`
}

func TestRuntimeEnvExactAppsAndRedaction(t *testing.T) {
	endpoints, err := LoadWebhookEndpoints(func(string) string { return runtimeTestApps })
	if err != nil || len(endpoints) != 2 || endpoints[0].Path != "/v1/meta/webhooks/123/page" ||
		endpoints[1].Path != "/v1/meta/webhooks/123/instagram" {
		t.Fatal("valid app configuration rejected")
	}
	for _, endpoint := range endpoints {
		for _, value := range []string{fmt.Sprint(endpoint), fmt.Sprintf("%+v", endpoint), fmt.Sprintf("%#v", endpoint)} {
			if strings.Contains(value, "1234567890abcdef") || !strings.Contains(value, "redacted") {
				t.Fatal("endpoint formatting exposed credential")
			}
		}
		encoded, err := json.Marshal(endpoint)
		if err != nil || strings.Contains(string(encoded), "1234567890abcdef") || !strings.Contains(string(encoded), "redacted") {
			t.Fatal("endpoint JSON exposed credential")
		}
	}
	bad := []string{
		``, `{}`, `{"apps":[]}`, `{"apps":"bad"}`, `{"apps":[{"app_id":123,"object":"page","app_secret":"1234567890abcdef","verify_token":"abcdef1234567890"}]}`,
		`{"apps":[{"app_id":"123","object":"page","app_secret":"1234567890abcdef","verify_token":"abcdef1234567890","extra":1}]}`,
		`{"apps":[{"app_id":"123","object":"page","app_secret":"1234567890abcdef","verify_token":"abcdef1234567890"}],"extra":1}`,
		`{"apps":[{"app_id":"123","object":"page","app_secret":"1234567890abcdef","verify_token":"abcdef1234567890"}],"apps":[]}`,
		`{"apps":[{"app_id":"123","object":"page","app_secret":"1234567890abcdef","verify_token":"abcdef1234567890"}]} true`,
		`{"apps":[{"app_id":"123","object":"page","app_secret":"1234567890abcdef","verify_token":"\ud800"}]}`,
		string([]byte{0xff}),
		strings.Repeat("x", 32769),
		strings.Replace(runtimeTestApps, `"object":"instagram"`, `"object":"page"`, 1),
		strings.Replace(runtimeTestApps, `"app_id":"123"`, `"app_id":"bad"`, 1),
	}
	for i, source := range bad {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			if _, err := LoadWebhookEndpoints(func(string) string { return source }); !errors.Is(err, ErrRuntimeConfig) {
				t.Fatal("invalid apps accepted")
			}
		})
	}
	if _, err := LoadWebhookEndpoints(nil); !errors.Is(err, ErrRuntimeConfig) {
		t.Fatal("nil environment accepted")
	}
}

func TestRuntimeEnvExactPayloadKeys(t *testing.T) {
	values := map[string]string{
		"COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID": "active",
		"COMMERCE_META_PAYLOAD_KEYS_JSON":     runtimeTestKeys(),
	}
	get := func(name string) string { return values[name] }
	if keys, err := LoadPayloadKeyring(get); err != nil || keys == nil {
		t.Fatal("valid payload key configuration rejected")
	}
	bad := []string{
		``, `{}`, `{"keys":[]}`, `{"keys":"bad"}`, `{"keys":[{"id":"active","key_base64":1}]}`,
		`{"keys":[{"id":"active","key_base64":"bad"}]}`,
		`{"keys":[{"id":"active","key_base64":"` + base64.StdEncoding.EncodeToString(make([]byte, 32)) + `"}]}`,
		`{"keys":[{"id":"active","key_base64":"` + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)) + `","other":1}]}`,
		`{"keys":[{"id":"active","key_base64":"` + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)) + `"}],"extra":1}`,
		strings.Repeat("x", 8193),
		strings.Replace(runtimeTestKeys(), `"id":"active"`, `"id":"bad/id"`, 1),
		strings.Replace(runtimeTestKeys(), `"keys"`, `"keys":"x","keys"`, 1),
		string([]byte{0xff}),
		strings.Replace(runtimeTestKeys(), `"keys"`, `"keys"`, 1) + ` true`,
	}
	for i, source := range bad {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			values["COMMERCE_META_PAYLOAD_KEYS_JSON"] = source
			if _, err := LoadPayloadKeyring(get); !errors.Is(err, ErrRuntimeConfig) {
				t.Fatal("invalid key configuration accepted")
			}
		})
	}
	values["COMMERCE_META_PAYLOAD_KEYS_JSON"] = runtimeTestKeys()
	values["COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID"] = "missing"
	if _, err := LoadPayloadKeyring(get); !errors.Is(err, ErrRuntimeConfig) {
		t.Fatal("missing active key accepted")
	}
	if _, err := LoadPayloadKeyring(nil); !errors.Is(err, ErrRuntimeConfig) {
		t.Fatal("nil environment accepted")
	}
}
