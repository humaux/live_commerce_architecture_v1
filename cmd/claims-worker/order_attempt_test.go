// Purpose: pin opt-in OA1 startup custody through the real worker config entry.
// Depends on: loadConfig and existing synthetic worker configuration fixtures.
// Used by: tests-first process CI before worker registration/product wiring.
// Invariants: disabled OA1 reads no new custody, enabled OA1 fails closed, no
// buyer/actor/Stripe key access, and all config/error renderings stay redacted.
package main

import (
	"bytes"
	"crypto/ecdh"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

var orderAttemptCustodyNames = []string{
	"LC_ORDER_ATTEMPT_PLAN_ACTIVE_KEY", "LC_ORDER_ATTEMPT_PLAN_KEYS",
	"LC_ORDER_ATTEMPT_PAY_ENTRY_ACTIVE_KEY", "LC_ORDER_ATTEMPT_PAY_ENTRY_KEYS",
	"COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID", "COMMERCE_META_PAYLOAD_KEYS_JSON",
	"COMMERCE_META_PAGE_HPKE_ACTIVE_KEY_ID", "COMMERCE_META_PAGE_HPKE_PUBLIC_KEYS_JSON",
}

func orderAttemptWorkerEnv(t *testing.T) map[string]string {
	t.Helper()
	values := testEnv()
	private, err := ecdh.X25519().NewPrivateKey(bytes.Repeat([]byte{83},32))
	if err != nil { t.Fatal("synthetic public key fixture failed") }
	public := base64.StdEncoding.EncodeToString(private.PublicKey().Bytes())
	values["LC_ORDER_ATTEMPT_ENABLED"] = "1"
	values["LC_ORDER_ATTEMPT_PLAN_ACTIVE_KEY"] = "plan-v1"
	values["LC_ORDER_ATTEMPT_PLAN_KEYS"] = `{"plan-v1":"`+b64(71)+`","plan-old":"`+b64(72)+`"}`
	values["LC_ORDER_ATTEMPT_PAY_ENTRY_ACTIVE_KEY"] = "pay-v1"
	values["LC_ORDER_ATTEMPT_PAY_ENTRY_KEYS"] = `{"pay-v1":"`+b64(73)+`","pay-old":"`+b64(74)+`"}`
	values["COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID"] = "body-v1"
	values["COMMERCE_META_PAYLOAD_KEYS_JSON"] = `{"keys":[{"id":"body-v1","key_base64":"`+b64(75)+`"}]}`
	values["COMMERCE_META_PAGE_HPKE_ACTIVE_KEY_ID"] = "public-v1"
	values["COMMERCE_META_PAGE_HPKE_PUBLIC_KEYS_JSON"] = `{"keys":[{"id":"public-v1","public_key_base64":"`+public+`"}]}`
	return values
}

func orderAttemptForbiddenEnv(name string) bool {
	return strings.Contains(name,"BFF") || strings.Contains(name,"ACTOR") || strings.Contains(name,"STRIPE")
}

func TestOrderAttemptWorkerConfigDisabledDoesNotReadCustody(t *testing.T) {
	for _, flag := range []string{"", "0"} {
		t.Run("flag_"+flag,func(t *testing.T) {
			values:=orderAttemptWorkerEnv(t)
			values["LC_ORDER_ATTEMPT_ENABLED"]=flag
			seenFlag:=false
			config,err:=loadConfig(func(name string) string {
				if orderAttemptForbiddenEnv(name) { t.Fatalf("worker queried forbidden custody %s",name) }
				if name=="LC_ORDER_ATTEMPT_ENABLED" { seenFlag=true }
				for _, custody:=range orderAttemptCustodyNames {
					if name==custody { t.Fatalf("disabled OA1 queried custody %s",name) }
				}
				return values[name]
			})
			if err!=nil || !config.enabled || !seenFlag { t.Fatal("worker must recognize OA1 off without disabling existing worker or loading new custody") }
		})
	}
}

func TestOrderAttemptWorkerConfigEnabledLoadsAllDedicatedCustody(t *testing.T) {
	values:=orderAttemptWorkerEnv(t)
	seen:=map[string]bool{}
	config,err:=loadConfig(func(name string) string {
		if orderAttemptForbiddenEnv(name) { t.Fatalf("OA1 queried forbidden custody %s",name) }
		seen[name]=true
		return values[name]
	})
	if err!=nil || !config.enabled { t.Fatalf("complete OA1 custody rejected: %v",err) }
	if !seen["LC_ORDER_ATTEMPT_ENABLED"] { t.Fatal("OA1 activation flag ignored") }
	for _, name:=range orderAttemptCustodyNames {
		if !seen[name] { t.Fatalf("enabled OA1 did not load required custody %s",name) }
	}
	encoded,err:=json.Marshal(config)
	if err!=nil { t.Fatal("worker config JSON redaction failed") }
	for _, rendered:=range []string{fmt.Sprint(config),fmt.Sprintf("%+v",config),fmt.Sprintf("%#v",config),string(encoded)} {
		if !strings.Contains(rendered,"redacted") { t.Fatal("OA1 config lost redacted formatting") }
		for _, secret:=range []string{b64(71),b64(72),b64(73),b64(74),b64(75),values["LC_ORDER_ATTEMPT_PLAN_KEYS"],values["LC_ORDER_ATTEMPT_PAY_ENTRY_KEYS"],values["COMMERCE_META_PAYLOAD_KEYS_JSON"]} {
			if strings.Contains(rendered,secret) { t.Fatal("OA1 worker config rendering leaked key material") }
		}
	}
}

func TestOrderAttemptWorkerConfigEnabledFailsClosedOnCustodyErrors(t *testing.T) {
	assertRejected:=func(t *testing.T,values map[string]string) {
		t.Helper()
		_,err:=loadConfig(func(name string) string {
			if orderAttemptForbiddenEnv(name) { t.Fatalf("invalid OA1 config queried forbidden custody %s",name) }
			return values[name]
		})
		if !errors.Is(err,errWorkerConfig) { t.Fatal("enabled OA1 accepted incomplete/invalid custody or returned a nonfixed startup error") }
		for _, rendered:=range []string{err.Error(),fmt.Sprintf("%+v",err),fmt.Sprintf("%#v",err)} {
			for _, value:=range values {
				if len(value)>16 && strings.Contains(rendered,value) { t.Fatal("OA1 startup failure disclosed configuration value") }
			}
		}
	}
	for _, name:=range orderAttemptCustodyNames {
		t.Run("missing_"+name,func(t *testing.T) { values:=orderAttemptWorkerEnv(t); delete(values,name); assertRejected(t,values) })
	}
	for _, flag:=range []string{"true","2"," 1","1 ","false"} {
		t.Run("noncanonical_flag_"+flag,func(t *testing.T) { values:=orderAttemptWorkerEnv(t); values["LC_ORDER_ATTEMPT_ENABLED"]=flag; assertRejected(t,values) })
	}
	for _, tc:=range []struct{ name,key,value string }{
		{"plan malformed","LC_ORDER_ATTEMPT_PLAN_KEYS","synthetic-sensitive-invalid-json"},
		{"pay malformed","LC_ORDER_ATTEMPT_PAY_ENTRY_KEYS","synthetic-sensitive-invalid-json"},
		{"plan active missing","LC_ORDER_ATTEMPT_PLAN_ACTIVE_KEY","missing-active"},
		{"pay active missing","LC_ORDER_ATTEMPT_PAY_ENTRY_ACTIVE_KEY","missing-active"},
		{"plan duplicate","LC_ORDER_ATTEMPT_PLAN_KEYS",`{"plan-v1":"`+b64(71)+`","plan-v1":"`+b64(72)+`"}`},
		{"pay trailing","LC_ORDER_ATTEMPT_PAY_ENTRY_KEYS",`{"pay-v1":"`+b64(73)+`"} {}`},
		{"cross-purpose inactive alias","LC_ORDER_ATTEMPT_PAY_ENTRY_KEYS",`{"pay-v1":"`+b64(73)+`","alias":"`+b64(72)+`"}`},
		{"payload malformed","COMMERCE_META_PAYLOAD_KEYS_JSON","synthetic-sensitive-invalid-json"},
		{"payload active missing","COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID","missing-active"},
		{"public seal malformed","COMMERCE_META_PAGE_HPKE_PUBLIC_KEYS_JSON","synthetic-sensitive-invalid-json"},
		{"public seal active missing","COMMERCE_META_PAGE_HPKE_ACTIVE_KEY_ID","missing-active"},
	} {
		t.Run(tc.name,func(t *testing.T) { values:=orderAttemptWorkerEnv(t); values[tc.key]=tc.value; assertRejected(t,values) })
	}
}
