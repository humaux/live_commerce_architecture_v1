package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestDisabledWorkerReadsOnlyFlag(t *testing.T) {
	for _, flag := range []string{"", "0"} {
		read := []string{}
		env := func(name string) string {
			read = append(read, name)
			if name == "COMMERCE_EXPIRY_WORKER_ENABLED" {
				return flag
			}
			t.Fatal("disabled worker inspected another variable")
			return ""
		}
		if err := run(context.Background(), env); err != nil || len(read) != 1 ||
			read[0] != "COMMERCE_EXPIRY_WORKER_ENABLED" {
			t.Fatalf("disabled worker: reads=%v err=%v", read, err)
		}
	}
}

func TestWorkerConfigStrictAndRedacted(t *testing.T) {
	base := map[string]string{
		"COMMERCE_EXPIRY_WORKER_ENABLED":      "1",
		"COMMERCE_EXPIRY_WORKER_DATABASE_URL": "postgres://secret@localhost/test",
	}
	read := func(values map[string]string) func(string) string {
		return func(name string) string { return values[name] }
	}
	config, err := loadConfig(read(base))
	if err != nil || !config.enabled || config.concurrency != 4 || config.dsn != base["COMMERCE_EXPIRY_WORKER_DATABASE_URL"] {
		t.Fatal("valid default configuration rejected")
	}
	for _, tc := range []struct{ field, value string }{
		{"COMMERCE_EXPIRY_WORKER_ENABLED", "true"},
		{"COMMERCE_EXPIRY_WORKER_DATABASE_URL", ""},
		{"COMMERCE_EXPIRY_WORKER_CONCURRENCY", "0"},
		{"COMMERCE_EXPIRY_WORKER_CONCURRENCY", "17"},
		{"COMMERCE_EXPIRY_WORKER_CONCURRENCY", "04"},
		{"COMMERCE_EXPIRY_WORKER_CONCURRENCY", "+4"},
	} {
		values := map[string]string{}
		for k, v := range base {
			values[k] = v
		}
		values[tc.field] = tc.value
		_, err := loadConfig(read(values))
		if !errors.Is(err, errWorkerConfig) {
			t.Fatalf("%s accepted", tc.field)
		}
		if strings.Contains(err.Error(), tc.value) && tc.value != "" {
			t.Fatalf("%s leaked input in error", tc.field)
		}
	}
}

// storefront-v2 §E8: the buyer mail loop is opt-in and validated like the API's login-code mail; no value ever reaches an error.
func TestBuyerMailConfig(t *testing.T) {
	smtpSecret := "pw-" + "sentinel-for-test"
	base := map[string]string{
		"COMMERCE_EXPIRY_WORKER_ENABLED":      "1",
		"COMMERCE_EXPIRY_WORKER_DATABASE_URL": "postgres://secret@localhost/test",
		"COMMERCE_BUYER_MAIL_ENABLED":         "1",
		"COMMERCE_SMTP_HOST":                  "smtp.example.test",
		"COMMERCE_SMTP_USERNAME":              "mailer@example.test",
		"COMMERCE_SMTP_PASSWORD":              smtpSecret,
		"COMMERCE_MAIL_FROM":                  "mailer@example.test",
	}
	read := func(values map[string]string) func(string) string {
		return func(name string) string { return values[name] }
	}
	config, err := loadConfig(read(base))
	if err != nil || config.mail == nil || config.mail.dailyCap != 200 {
		t.Fatalf("valid mail configuration rejected: %v", err)
	}
	off := map[string]string{}
	for k, v := range base {
		off[k] = v
	}
	off["COMMERCE_BUYER_MAIL_ENABLED"] = "0"
	if config, err = loadConfig(read(off)); err != nil || config.mail != nil {
		t.Fatal("flag 0 must leave the loop off without reading SMTP variables")
	}
	for _, tc := range []struct{ field, value string }{
		{"COMMERCE_BUYER_MAIL_ENABLED", "yes"},
		{"COMMERCE_SMTP_HOST", ""},
		{"COMMERCE_SMTP_HOST", "localhost"}, // loopback needs the explicit test flag
		{"COMMERCE_SMTP_HOST", "-bad-.example.test"},
		{"COMMERCE_SMTP_PASSWORD", ""},
		{"COMMERCE_MAIL_FROM", "someone-else@example.test"}, // From must equal Username (mail.NewSMTP, M2)
		{"COMMERCE_MAIL_DAILY_CAP", "19"},
		{"COMMERCE_MAIL_DAILY_CAP", "100001"},
		{"COMMERCE_MAIL_DAILY_CAP", "many"},
	} {
		values := map[string]string{}
		for k, v := range base {
			values[k] = v
		}
		values[tc.field] = tc.value
		_, err := loadConfig(read(values))
		if !errors.Is(err, errWorkerConfig) {
			t.Fatalf("%s=%q accepted", tc.field, tc.value)
		}
		if strings.Contains(err.Error(), smtpSecret) || strings.Contains(err.Error(), "smtp.example.test") {
			t.Fatalf("%s leaked a value in the error", tc.field)
		}
	}
}
