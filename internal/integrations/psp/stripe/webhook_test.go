// webhook_test.go: SP05 — the §5.8 verifier against the Node-generated vectors in
// tests/payments/stripe-webhook-vectors.json, fresh Go-signed cases, the hmac.Equal
// source guard, and the independent Node cross-check. Non-goal: HTTP admission (SP13).
// Callers: go test ./internal/integrations/psp/stripe/...

package stripe

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const vectorsPath = "../../../../tests/payments/stripe-webhook-vectors.json"

type webhookVector struct {
	Name    string   `json:"name"`
	Secrets []string `json:"secrets"`
	Header  string   `json:"header"`
	Body    *string  `json:"body"`
	BodyB64 *string  `json:"body_b64"`
	Now     int64    `json:"now"`
	Expect  string   `json:"expect"`
	Event   *struct {
		ID                string `json:"id"`
		Type              string `json:"type"`
		ObjectType        string `json:"object_type"`
		SessionID         string `json:"session_id"`
		ClientReferenceID string `json:"client_reference_id"`
		MetadataAttempt   string `json:"metadata_attempt"`
		Livemode          bool   `json:"livemode"`
		AccountPresent    bool   `json:"account_present"`
		Probe             bool   `json:"probe"`
	} `json:"event"`
}

func sign(secret string, ts int64, body []byte) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(strconv.FormatInt(ts, 10) + "."))
	m.Write(body)
	return hex.EncodeToString(m.Sum(nil))
}

func TestStripeSP05Webhook(t *testing.T) {
	raw, err := os.ReadFile(vectorsPath)
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Vectors []webhookVector `json:"vectors"`
	}
	if err := json.Unmarshal(raw, &file); err != nil || len(file.Vectors) < 20 {
		t.Fatalf("vectors: %v n=%d", err, len(file.Vectors))
	}
	counts := map[string]int{}
	for _, v := range file.Vectors {
		t.Run(v.Name, func(t *testing.T) {
			verifier, err := NewWebhookVerifier(WebhookConfig{Secrets: v.Secrets, AccountID: fakeAccount, Environment: "SANDBOX"})
			if err != nil {
				t.Fatal(err)
			}
			var body []byte
			switch {
			case v.BodyB64 != nil:
				body, _ = base64.StdEncoding.DecodeString(*v.BodyB64)
			case v.Body != nil:
				body = []byte(*v.Body)
			}
			ev, err := verifier.Verify(body, v.Header, time.Unix(v.Now, 0))
			switch v.Expect {
			case "signature":
				if !errors.Is(err, ErrSignature) || ev != (Event{}) {
					t.Fatalf("want ErrSignature, got %v", err)
				}
			case "malformed":
				if err != nil || !ev.Malformed || ev.BodySHA256 != sha256.Sum256(body) || ev.SignedAt == 0 || ev.ID != "" {
					t.Fatalf("want malformed, got err=%v malformed=%v", err, ev.Malformed)
				}
			case "ok":
				e := v.Event
				if err != nil || ev.Malformed || e == nil || ev.ID != e.ID || ev.Type != e.Type || ev.ObjectType != e.ObjectType ||
					ev.SessionID != e.SessionID || ev.ClientReferenceID != e.ClientReferenceID ||
					ev.MetadataAttempt != e.MetadataAttempt || ev.Livemode != e.Livemode ||
					ev.AccountPresent != e.AccountPresent || ev.ProbeSession != e.Probe ||
					ev.APIVersion != APIVersion || ev.BodySHA256 != sha256.Sum256(body) || ev.Created <= 0 {
					t.Fatalf("projection mismatch: err=%v", err)
				}
			default:
				t.Fatalf("unknown expect %q", v.Expect)
			}
			counts[v.Expect]++
		})
	}
	if counts["ok"] == 0 || counts["malformed"] == 0 || counts["signature"] == 0 {
		t.Fatalf("vector kinds %v", counts)
	}

	t.Run("fresh Go-signed cases", func(t *testing.T) {
		v, _ := NewWebhookVerifier(WebhookConfig{Secrets: []string{fakeWhsecA, fakeWhsecB}, AccountID: fakeAccount, Environment: "SANDBOX"})
		body := []byte(`{"id":"evt_1","object":"event","type":"checkout.session.expired","livemode":false,"created":5,"data":{"object":{"id":"cs_test_9","object":"checkout.session","client_reference_id":null,"metadata":{}}}}`)
		now := time.Unix(1_800_000_000, 0)
		ts := now.Unix()
		hdr := "t=" + strconv.FormatInt(ts, 10) + ",v1=" + sign(fakeWhsecB, ts, body)
		ev, err := v.Verify(body, hdr, now)
		if err != nil || ev.SessionID != "cs_test_9" || ev.ClientReferenceID != "" || ev.SignedAt != ts {
			t.Fatalf("fresh: %v", err)
		}
		// Formatting the event never exposes its content.
		if s, _ := json.Marshal(ev); strings.Contains(string(s), "cs_test_9") {
			t.Fatal("event marshals content")
		}
		var nilVerifier *WebhookVerifier
		if _, err := nilVerifier.Verify(body, hdr, now); !errors.Is(err, ErrInvalid) {
			t.Fatal("nil verifier")
		}
		// Over-long reference values are bounded, never passed through.
		longRef := strings.Replace(string(body), `"client_reference_id":null`, `"client_reference_id":"`+strings.Repeat("x", 300)+`"`, 1)
		ev, err = v.Verify([]byte(longRef), "t="+strconv.FormatInt(ts, 10)+",v1="+sign(fakeWhsecA, ts, []byte(longRef)), now)
		if err != nil || ev.ClientReferenceID != invalidRef {
			t.Fatalf("long ref: %v %q", err, ev.ClientReferenceID)
		}
	})

	t.Run("hmac.Equal source guard", func(t *testing.T) {
		src, err := os.ReadFile("webhook.go")
		if err != nil {
			t.Fatal(err)
		}
		s := string(src)
		if !strings.Contains(s, "hmac.Equal(expected, v1s[i][:])") || strings.Contains(s, "bytes.Equal") ||
			strings.Contains(s, "subtle.") || strings.Contains(s, "json.Unmarshal") {
			t.Fatal("signature comparison must use hmac.Equal and strict JSON only")
		}
		if strings.Index(s, "hmac.Equal(") > strings.Index(s, "projectEvent(raw)") {
			t.Fatal("JSON parsing must follow signature verification")
		}
	})

	t.Run("node cross-check", func(t *testing.T) {
		node, err := exec.LookPath("node")
		if err != nil {
			t.Skip("NOT_RUN: node not on PATH")
		}
		script, _ := filepath.Abs("../../../../scripts/dev/stripe-webhook-check.mjs")
		vectors, _ := filepath.Abs(vectorsPath)
		out, err := exec.Command(node, script, vectors).CombinedOutput()
		if err != nil {
			t.Fatalf("node verifier failed: %v\n%s", err, out)
		}
		t.Logf("%s", strings.TrimSpace(string(out)))
	})
}
