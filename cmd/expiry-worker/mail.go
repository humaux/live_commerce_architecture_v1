package main

// mail.go: optional buyer-notification loop of this process (contracts/storefront-v2.md §E8). Off unless COMMERCE_BUYER_MAIL_ENABLED=1.
// Why this process: it is the only River-running worker that touches nothing but PostgreSQL (no PSP, no Meta token), so the SMTP credential
// is the only new secret it holds, and the order lifecycle it already drives (expiry -> cancelled) is the first source of mail. The loop
// itself is a plain poll of notify.outbox (internal/notify), not a River job: queue admission on river_job is guarded by post_river triggers
// owned by other units.

import (
	"errors"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"livecommerce/internal/mail"
)

var smtpHostPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)

// mailConfig is the SMTP account and the daily cap shared with the login-code mail (COMMERCE_MAIL_DAILY_CAP).
type mailConfig struct {
	smtp     *mail.SMTP
	dailyCap int
}

// loadMailConfig mirrors cmd/api's password-mail variables (same names, same validation): COMMERCE_SMTP_HOST / _USERNAME / _PASSWORD
// (or _FILE), COMMERCE_MAIL_FROM, COMMERCE_MAIL_DAILY_CAP (20..100000, default 200). Errors never carry a value.
func loadMailConfig(getenv func(string) string) (*mailConfig, error) {
	loopback := getenv("COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS") == "1"
	password, err := secretEnv(getenv, "COMMERCE_SMTP_PASSWORD")
	if err != nil || len(password) > 256 || strings.IndexFunc(password, func(r rune) bool { return r < 32 || r == 127 }) != -1 {
		return nil, errWorkerConfig
	}
	host := strings.ToLower(getenv("COMMERCE_SMTP_HOST"))
	if isLoopback(host) {
		if !loopback {
			return nil, errWorkerConfig
		}
	} else if !smtpHostPattern.MatchString(host) || len(host) > 253 {
		return nil, errWorkerConfig
	}
	dailyCap := 200
	if v := getenv("COMMERCE_MAIL_DAILY_CAP"); v != "" {
		if dailyCap, err = strconv.Atoi(v); err != nil || dailyCap < 20 || dailyCap > 100000 {
			return nil, errWorkerConfig
		}
	}
	// Port stays 0 (= 465, implicit TLS) and RootCAs nil (system roots), exactly as cmd/api.
	smtp, err := mail.NewSMTP(mail.Config{Host: host, Username: getenv("COMMERCE_SMTP_USERNAME"), Password: password,
		From: getenv("COMMERCE_MAIL_FROM"), AllowLoopback: loopback})
	if err != nil {
		return nil, errWorkerConfig
	}
	return &mailConfig{smtp: smtp, dailyCap: dailyCap}, nil
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	addr, err := netip.ParseAddr(host)
	return err == nil && addr.Zone() == "" && addr.IsLoopback()
}

// secretEnv resolves NAME or NAME_FILE (one of them), as cmd/api does; lcentry normally expands the file into NAME before exec.
func secretEnv(getenv func(string) string, name string) (string, error) {
	value, path := getenv(name), getenv(name+"_FILE")
	switch {
	case value != "" && path != "":
		return "", errors.New("both set")
	case value != "":
		return strings.TrimRight(value, "\r\n"), nil
	case path != "":
		if !filepath.IsAbs(path) {
			return "", errors.New("secret file path must be absolute")
		}
		f, err := os.Open(path)
		if err != nil {
			return "", errors.New("secret file unreadable")
		}
		defer f.Close()
		b, err := io.ReadAll(io.LimitReader(f, 1025))
		if err != nil || len(b) == 0 || len(b) > 1024 || strings.TrimRight(string(b), "\r\n") == "" {
			return "", errors.New("secret file empty or too large")
		}
		return strings.TrimRight(string(b), "\r\n"), nil
	}
	return "", errors.New("not set")
}
