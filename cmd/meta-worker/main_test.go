package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func workerTestEnv() map[string]string {
	return map[string]string{
		"COMMERCE_META_WORKER_ENABLED":        "1",
		"COMMERCE_META_WORKER_DATABASE_URL":   "postgres://synthetic.invalid/worker",
		"COMMERCE_META_CONSUMER_DATABASE_URL": "postgres://synthetic.invalid/consumer",
		"COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID": "active",
		"COMMERCE_META_PAYLOAD_KEYS_JSON":     `{"keys":[{"id":"active","key_base64":"` + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{5}, 32)) + `"}]}`,
	}
}

func TestMetaWorkerDisabledReadsOnlyFlag(t *testing.T) {
	for _, flagValue := range []string{"", "0"} {
		get := func(name string) string {
			if name != "COMMERCE_META_WORKER_ENABLED" {
				t.Fatal("disabled worker read authority setting")
			}
			return flagValue
		}
		c, err := loadConfig(get)
		if err != nil || c.enabled {
			t.Fatal("disabled worker changed")
		}
		if err := run(context.Background(), get); err != nil {
			t.Fatal("disabled worker opened dependency")
		}
	}
	for _, flagValue := range []string{"true", "2", " 1"} {
		if _, err := loadConfig(func(string) string { return flagValue }); !errors.Is(err, errWorkerConfig) {
			t.Fatal("noncanonical flag accepted")
		}
	}
}

func TestMetaWorkerConfigBoundsAndNoAppSecrets(t *testing.T) {
	values := workerTestEnv()
	get := func(name string) string {
		if name == "COMMERCE_META_APPS_JSON" {
			t.Fatal("worker read app secret configuration")
		}
		return values[name]
	}
	c, err := loadConfig(get)
	if err != nil || !c.enabled || c.concurrency != 4 || c.keys == nil {
		t.Fatal("valid worker configuration rejected")
	}
	for _, raw := range []string{"1", "16"} {
		values["COMMERCE_META_WORKER_CONCURRENCY"] = raw
		c, err := loadConfig(get)
		if err != nil || c.concurrency != 1 && raw == "1" || c.concurrency != 16 && raw == "16" {
			t.Fatal("concurrency boundary rejected")
		}
	}
	for _, raw := range []string{"0", "17", "01", "+1", "1 ", "1.0", "x"} {
		values["COMMERCE_META_WORKER_CONCURRENCY"] = raw
		if _, err := loadConfig(get); !errors.Is(err, errWorkerConfig) {
			t.Fatal("invalid concurrency accepted")
		}
	}
	values["COMMERCE_META_WORKER_CONCURRENCY"] = ""
	for _, name := range []string{"COMMERCE_META_WORKER_DATABASE_URL", "COMMERCE_META_CONSUMER_DATABASE_URL"} {
		original := values[name]
		for _, value := range []string{"", " ", strings.Repeat("x", 8193)} {
			values[name] = value
			if _, err := loadConfig(get); !errors.Is(err, errWorkerConfig) {
				t.Fatal("invalid database URL accepted")
			}
		}
		values[name] = original
	}
	if _, err := loadConfig(nil); !errors.Is(err, errWorkerConfig) {
		t.Fatal("nil environment accepted")
	}
	for _, rendered := range []string{fmt.Sprint(c), fmt.Sprintf("%+v", c), fmt.Sprintf("%#v", c)} {
		if strings.Contains(rendered, c.workerDSN) || strings.Contains(rendered, c.consumerDSN) || !strings.Contains(rendered, "redacted") {
			t.Fatal("worker config formatting leaked URL")
		}
	}
	encoded, err := json.Marshal(c)
	if err != nil || strings.Contains(string(encoded), c.workerDSN) || !strings.Contains(string(encoded), "redacted") {
		t.Fatal("worker config JSON leaked URL")
	}
}
