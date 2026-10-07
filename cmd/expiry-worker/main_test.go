// Purpose: test worker config, mail opt-ins and secret input refusals without production services.
// Depends on: expiry-worker loadConfig, fixture environment values and temporary secret files.
// Used by: cmd/expiry-worker unit/race gates and integrator CI.
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
	if config.mail.adminOrigin != "" {
		t.Fatal("unset admin origin must leave the merchant-alert loop off")
	}
	off := map[string]string{}
	for k, v := range base {
		off[k] = v
	}
	off["COMMERCE_BUYER_MAIL_ENABLED"] = "0"
	if config, err = loadConfig(read(off)); err != nil || config.mail != nil {
		t.Fatal("flag 0 must leave the loop off without reading SMTP variables")
	}
	// A configured public origin must not opt the owner into merchant alerts.
	withAdmin := map[string]string{}
	for k, v := range base {
		withAdmin[k] = v
	}
	withAdmin["COMMERCE_ADMIN_ORIGIN"] = "https://admin.example.test"
	if config, err = loadConfig(read(withAdmin)); err != nil || config.mail.adminOrigin != "" {
		t.Fatal("buyer mail alone must not enable merchant alerts")
	}
	withAdmin["COMMERCE_MERCHANT_ALERT_MAIL"] = "1"
	if config, err = loadConfig(read(withAdmin)); err != nil || config.mail.adminOrigin != "https://admin.example.test" {
		t.Fatalf("valid admin origin rejected: %v", err)
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
		{"COMMERCE_ADMIN_ORIGIN", "http://admin.example.test"},       // §10: https only
		{"COMMERCE_ADMIN_ORIGIN", "https://admin.example.test\r\nx"}, // header injection
		{"COMMERCE_ADMIN_ORIGIN", "https://admin.example.test/a b"},  // whitespace
	} {
		values := map[string]string{}
		for k, v := range base {
			values[k] = v
		}
		values[tc.field] = tc.value
		if tc.field == "COMMERCE_ADMIN_ORIGIN" {
			values["COMMERCE_MERCHANT_ALERT_MAIL"] = "1"
		}
		_, err := loadConfig(read(values))
		if !errors.Is(err, errWorkerConfig) {
			t.Fatalf("%s=%q accepted", tc.field, tc.value)
		}
		if strings.Contains(err.Error(), smtpSecret) || strings.Contains(err.Error(), "smtp.example.test") {
			t.Fatalf("%s leaked a value in the error", tc.field)
		}
	}
}

// TestMerchantAlertMailOptIn checks both independent flags without a DB or SMTP send.
func TestMerchantAlertMailOptIn(t *testing.T) {
	for _, tc := range []struct {
		buyer, merchant, origin            string
		wantBuyer, wantMerchant, wantError bool
	}{
		{"1", "", "https://admin.example.test", true, false, false},
		{"1", "0", "http://ignored.invalid", true, false, false},
		{"1", "1", "https://admin.example.test", true, true, false},
		{"0", "1", "https://admin.example.test", false, true, false},
		{"0", "0", "https://admin.example.test", false, false, false},
		{"1", "yes", "https://admin.example.test", false, false, true},
		{"0", "01", "https://admin.example.test", false, false, true},
		{"0", "1", "", false, false, true},
	} {
		values := map[string]string{
			"COMMERCE_EXPIRY_WORKER_ENABLED": "1", "COMMERCE_EXPIRY_WORKER_DATABASE_URL": "postgres://fixture@localhost/test",
			"COMMERCE_BUYER_MAIL_ENABLED": tc.buyer, "COMMERCE_MERCHANT_ALERT_MAIL": tc.merchant,
			"COMMERCE_ADMIN_ORIGIN": tc.origin, "COMMERCE_SMTP_HOST": "smtp.example.test",
			"COMMERCE_SMTP_USERNAME": "mailer@example.test", "COMMERCE_SMTP_PASSWORD": "fixture-password-for-config-only",
			"COMMERCE_MAIL_FROM": "mailer@example.test",
		}
		cfg, err := loadConfig(func(name string) string {
			if name == "COMMERCE_ADMIN_ORIGIN" && tc.merchant != "1" {
				t.Fatal("disabled alerts must not read the admin origin")
			}
			return values[name]
		})
		if tc.wantError {
			if !errors.Is(err, errWorkerConfig) {
				t.Fatal("invalid alert configuration accepted")
			}
			continue
		}
		if err != nil || cfg.buyerMail != tc.wantBuyer {
			t.Fatal("wrong buyer mail opt-in")
		}
		if tc.wantBuyer || tc.wantMerchant {
			if cfg.mail == nil || (cfg.mail.adminOrigin != "") != tc.wantMerchant {
				t.Fatal("wrong merchant mail opt-in")
			}
		} else if cfg.mail != nil {
			t.Fatal("SMTP config loaded when both loops are off")
		}
	}
}
