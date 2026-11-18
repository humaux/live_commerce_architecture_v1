// Purpose: falsify HTTP admission ordering and fixture identity drift without a database.
// Depends on: net/http/httptest, the portable corpus and scripted pgx transactions.
// Used by: focused local and CI Go tests. These transactions are stubs.
package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"livecommerce/bench/l01/neutral"
)

func signature(raw string) string {
	mac := hmac.New(sha256.New, []byte(neutral.DefaultHMACSecret))
	_, _ = mac.Write([]byte(raw))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func requestFor(url, raw string) *http.Request {
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(raw))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", signature(raw))
	req.Header.Set("X-Tenant-ID", neutral.DefaultSpoofedTenantHeader)
	return req
}

func fixtureBody(asset, mid string) string {
	return fmt.Sprintf(`{"object":"page","entry":[{"id":%q,"time":123,"messaging":[{"sender":{"id":"4"},"recipient":{"id":%q},"message":{"mid":%q,"text":"synthetic-w1-fixed-body"}}]}]}`, asset, asset, mid)
}

func TestFixtureSettingsGuard(t *testing.T) {
	t.Setenv("LC_TEST_DATABASE_ALLOWED", "1")
	t.Setenv("PGHOST", "127.0.0.1")
	t.Setenv("PGPORT", "15432")
	t.Setenv("PGHOSTADDR", "")
	t.Setenv("PGSERVICE", "")
	t.Setenv("PGOPTIONS", "")
	const want = "host=127.0.0.1 port=15432 dbname=lc_neutral_w1_test user=l01_w1_ingress sslmode=disable connect_timeout=2"
	if dsn, err := fixtureConnString(ingressRole); dsn != want || err != nil {
		t.Fatal("disposable setting did not select the fixed ingress connection")
	}
	for key, unsafe := range map[string]string{
		"LC_TEST_DATABASE_ALLOWED": "", "PGHOST": "localhost", "PGPORT": "0",
		"PGHOSTADDR": "127.0.0.2", "PGSERVICE": "override", "PGOPTIONS": "-c role=postgres",
	} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, unsafe)
			if _, err := fixtureConnString(ingressRole); err == nil {
				t.Fatal("unsafe fixture connection settings accepted")
			}
		})
	}
}

func TestRejectsBeforePersistence(t *testing.T) {
	body := fixtureBody(neutral.DefaultAuthoritativeAssetID, "m.w1.test")
	handler := newWebhook(func(context.Context, pgx.TxOptions) (pgx.Tx, error) {
		t.Fatal("rejected request reached BeginTx")
		return nil, nil
	})
	cases := []struct {
		name, raw string
		alter     func(*http.Request)
		status    int
	}{
		{"invalid MAC before parsing", "not JSON", func(r *http.Request) {
			r.Header.Set("X-Hub-Signature-256", signature(body))
		}, 403},
		{"exact wire bytes", body + " ", func(r *http.Request) {
			r.Header.Set("X-Hub-Signature-256", signature(body))
		}, 403},
		{"duplicate signature field", body, func(r *http.Request) {
			r.Header.Add("X-Hub-Signature-256", signature(body))
		}, 403},
		{"uppercase hex", body, func(r *http.Request) {
			r.Header.Set("X-Hub-Signature-256", "sha256="+strings.ToUpper(signature(body)[7:]))
		}, 403},
		{"duplicate JSON member", strings.Replace(body, `"object":`, `"obje\u0063t":"page","object":`, 1), nil, 400},
		{"trailing JSON", body + "{}", nil, 400},
		{"wrong case", strings.Replace(body, `"object"`, `"Object"`, 1), nil, 400},
		{"wrong object", strings.Replace(body, `"page"`, `"instagram"`, 1), nil, 400},
		{"wrong recipient", strings.Replace(body, `"recipient":{"id":"1000000001901101"}`, `"recipient":{"id":"1"}`, 1), nil, 400},
		{"multiple entries", strings.Replace(body, `"entry":[`, `"entry":[{},`, 1), nil, 400},
		{"multiple messages", strings.Replace(body, `"messaging":[`, `"messaging":[{},`, 1), nil, 400},
		{"invalid asset", fixtureBody("not-an-asset", "m.w1.test"), nil, 400},
		{"empty MID", fixtureBody(neutral.DefaultAuthoritativeAssetID, ""), nil, 400},
		{"NUL in MID", strings.Replace(body, "m.w1.test", `m.\u0000`, 1), nil, 400},
		{"invalid surrogate", strings.Replace(body, "m.w1.test", `\ud800`, 1), nil, 400},
		{"invalid UTF-8", strings.Replace(body, "m.w1.test", string([]byte{0xff}), 1), nil, 400},
		{"over cap", strings.Repeat(" ", maxBody+1), nil, 413},
		{"wrong route", body, func(r *http.Request) { r.URL.Path += "/" }, 404},
		{"wrong method", body, func(r *http.Request) { r.Method = http.MethodGet }, 405},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := requestFor(endpoint, tc.raw)
			if tc.alter != nil {
				tc.alter(req)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
		})
	}
}

type replyRow string

func (r replyRow) Scan(dest ...any) error {
	*dest[0].(*string) = string(r)
	return nil
}

type stubTx struct {
	pgx.Tx
	statuses []replyRow
	args     [][]any
	commit   func() error
	closed   bool
}

func (s *stubTx) QueryRow(_ context.Context, _ string, args ...any) pgx.Row {
	s.args = append(s.args, args)
	return s.statuses[len(s.args)-1]
}

func (s *stubTx) Commit(context.Context) error { return s.commit() }

func (s *stubTx) Rollback(context.Context) error {
	s.closed = true
	return nil
}

func TestACKRequiresCommit(t *testing.T) {
	raw := fixtureBody(neutral.DefaultAuthoritativeAssetID, "m.w1.commit")
	for _, tc := range []struct {
		name, result string
		commitError  error
		status       int
	}{
		{"routed", "routed", nil, 200},
		{"replay", "replay", nil, 200},
		{"conflict", "conflict", nil, 409},
		{"commit failure", "routed", errors.New("injected commit failure"), 503},
		{"unexpected result", "quarantined", nil, 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			committed := false
			tx := &stubTx{statuses: []replyRow{replyRow(tc.result)}, commit: func() error {
				// Observe the response writer at the instant Commit is called.
				if rec.Code != 0 || rec.Body.Len() != 0 {
					t.Fatal("response written before commit")
				}
				committed = true
				return tc.commitError
			}}
			rec.Code = 0
			newWebhook(func(_ context.Context, options pgx.TxOptions) (pgx.Tx, error) {
				if options.IsoLevel != pgx.ReadCommitted {
					t.Fatal("transaction isolation drifted")
				}
				return tx, nil
			}).ServeHTTP(rec, requestFor(endpoint, raw))
			if rec.Code != tc.status || (rec.Body.String() == "EVENT_RECEIVED") != (tc.status == 200) {
				t.Fatalf("got %d %q, want status %d", rec.Code, rec.Body.String(), tc.status)
			}
			if !tx.closed || committed != (tc.result != "quarantined") || len(tx.args) != 1 {
				t.Fatal("unexpected transaction lifecycle")
			}
			want := []any{neutral.DefaultAppID, "page", neutral.DefaultAuthoritativeAssetID, "m.w1.commit", sha([]byte(raw))}
			for i, v := range tx.args[0] {
				if v != want[i] {
					t.Errorf("unexpected admission argument at index %d", i)
				}
			}
		})
	}
}

func TestQuarantineSealsEachAttemptInOneTransaction(t *testing.T) {
	var attempts []*stubTx
	handler := newWebhook(func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
		tx := &stubTx{statuses: []replyRow{"needs_quarantine", "quarantined"}, commit: func() error { return nil }}
		attempts = append(attempts, tx)
		return tx, nil
	})
	raw := fixtureBody("1000000001901104", "m.w1.quarantine")
	for range 2 {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, requestFor(endpoint, raw))
		if rec.Code != 200 || rec.Body.String() != "EVENT_RECEIVED" {
			t.Fatalf("quarantine returned %d %q", rec.Code, rec.Body.String())
		}
	}
	if len(attempts) != 2 {
		t.Fatalf("transactions: %d", len(attempts))
	}
	for _, tx := range attempts {
		if len(tx.args) != 2 || len(tx.args[0]) != 5 || len(tx.args[1]) != 9 {
			t.Fatal("each transaction must include both admit calls and the correct argument counts")
		}
		for i, v := range tx.args[0] {
			if v != tx.args[1][i] {
				t.Fatal("event identity changed between admit calls")
			}
		}
		if tx.args[1][6] != "neutral_w1_fixture" || len(tx.args[1][7].([]byte)) != 12 ||
			len(tx.args[1][8].([]byte)) != len(raw)+16 {
			t.Fatal("quarantine envelope format differs")
		}
	}
	first, retry := attempts[0].args[1], attempts[1].args[1]
	if first[5] == retry[5] || bytes.Equal(first[7].([]byte), retry[7].([]byte)) {
		t.Fatal("quarantine retry reused UUID or nonce")
	}
}
