// Purpose: authenticate and admit one synthetic W1 webhook, then return its committed outcome.
// Depends on: the neutral corpus constants, Go crypto and JSON, and the frozen l01_w1.admit function.
// Used by: the loopback Go W1 command and its conformance tests.
// Invariants: SQL resolves scope, quarantine is unscoped, and only a successful commit permits an ACK.
package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"livecommerce/bench/l01/neutral"
)

const (
	endpoint = "/__l01/w1/webhook"
	maxBody  = 1 << 20
	app      = neutral.DefaultAppID
	object   = neutral.DefaultObject
	secret   = neutral.DefaultHMACSecret
	keyID    = "neutral_w1_fixture"
)

var (
	signaturePattern = regexp.MustCompile(`^sha256=[0-9a-f]{64}$`)
	assetPattern     = regexp.MustCompile(`^[0-9]{1,40}$`)
)

type delivery struct {
	Object string `json:"object"`
	Entry  []struct {
		ID        string `json:"id"`
		Messaging []struct {
			Recipient struct {
				ID string `json:"id"`
			} `json:"recipient"`
			Message struct {
				MID string `json:"mid"`
			} `json:"message"`
		} `json:"messaging"`
	} `json:"entry"`
}

type beginTx func(context.Context, pgx.TxOptions) (pgx.Tx, error)

func newWebhook(begin beginTx) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != endpoint {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
		if err != nil {
			var oversized *http.MaxBytesError
			if errors.As(err, &oversized) {
				http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
			} else {
				http.Error(w, "body unreadable", http.StatusBadRequest)
			}
			return
		}
		if !authenticated(raw, r.Header.Values("X-Hub-Signature-256")) {
			http.Error(w, "signature rejected", http.StatusForbidden)
			return
		}
		asset, mid, ok := parseDelivery(raw)
		if !ok {
			http.Error(w, "invalid fixture delivery", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		status, err := admit(ctx, begin, raw, asset, mid)
		switch {
		case err != nil:
			http.Error(w, "fixture admission unavailable", http.StatusServiceUnavailable)
		case status == "conflict":
			http.Error(w, "digest conflict", http.StatusConflict)
		default:
			_, _ = io.WriteString(w, "EVENT_RECEIVED")
		}
	})
}

func authenticated(raw []byte, headers []string) bool {
	if len(headers) != 1 || !signaturePattern.MatchString(headers[0]) {
		return false
	}
	want, _ := hex.DecodeString(strings.TrimPrefix(headers[0], "sha256="))
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(raw)
	return hmac.Equal(mac.Sum(nil), want)
}

func parseDelivery(raw []byte) (asset, mid string, ok bool) {
	var d delivery
	// JSON v2 rejects duplicate names, invalid UTF-8, invalid escapes and trailing values.
	if err := json.Unmarshal(raw, &d); err != nil || d.Object != object ||
		len(d.Entry) != 1 || len(d.Entry[0].Messaging) != 1 {
		return "", "", false
	}
	entry := d.Entry[0]
	message := entry.Messaging[0]
	asset, mid = entry.ID, message.Message.MID
	return asset, mid, assetPattern.MatchString(asset) && asset == message.Recipient.ID &&
		len(mid) > 0 && len(mid) <= 1024 && !strings.ContainsRune(mid, 0)
}

func admit(ctx context.Context, begin beginTx, raw []byte, asset, mid string) (string, error) {
	tx, err := begin(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return "", err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()

	digest := sha(raw)
	var status string
	err = tx.QueryRow(ctx, "SELECT l01_w1.admit($1,$2,$3,$4,$5)", app, object, asset, mid, digest).Scan(&status)
	if err != nil {
		return "", err
	}

	switch status {
	case "needs_quarantine":
		id, nonce, ciphertext, err := seal(raw, asset, mid, digest)
		if err != nil {
			return "", err
		}
		err = tx.QueryRow(ctx, "SELECT l01_w1.admit($1,$2,$3,$4,$5,$6,$7,$8,$9)",
			app, object, asset, mid, digest, id, keyID, nonce, ciphertext).Scan(&status)
		if err != nil {
			return "", err
		}
		if status != "quarantined" {
			return "", errors.New("unexpected W1 quarantine result")
		}
	case "routed", "replay", "conflict":
	default:
		return "", errors.New("unexpected W1 admission result")
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return status, nil
}

func sha(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func seal(raw []byte, asset, mid, digest string) (pgtype.UUID, []byte, []byte, error) {
	id := pgtype.UUID{Valid: true}
	rand.Read(id.Bytes[:])
	id.Bytes[6] = id.Bytes[6]&0x0f | 0x40
	id.Bytes[8] = id.Bytes[8]&0x3f | 0x80

	// The checked-in PG reader uses this synthetic AES-256 key and the positional v1 AAD.
	block, err := aes.NewCipher(bytes.Repeat([]byte{0x31}, 32))
	if err != nil {
		return id, nil, nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return id, nil, nil, err
	}
	tuple, err := json.Marshal([]string{app, object, asset, mid})
	if err != nil {
		return id, nil, nil, err
	}
	aad, err := json.Marshal([]any{"livecommerce/meta-payload/v1", "quarantine", id.String(),
		app, object, "", sha(tuple), digest, "", "", "", 0, keyID})
	if err != nil {
		return id, nil, nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	rand.Read(nonce)
	return id, nonce, aead.Seal(nil, nonce, raw, aad), nil
}
