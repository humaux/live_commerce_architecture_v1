package meta

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthenticatedRawHandoff(t *testing.T) {
	v, err := NewVerifier(Config{AppID: "17", Object: "page", AppSecret: "synthetic-test-secret", VerifyToken: "synthetic-verify-token"})
	if err != nil {
		t.Fatal(err)
	}
	// Exact bytes include whitespace/escapes and more than one asset. This body
	// must never become one tenant's scoped payload; it is restricted raw evidence.
	raw := " {\n\"object\":\"page\",\"entry\":[{\"id\":\"100\",\"messaging\":[{\"sender\":{\"id\":\"200\"},\"recipient\":{\"id\":\"100\"},\"message\":{\"mid\":\"m-1\",\"text\":\"\\u4f60\"}}]},{\"id\":\"101\",\"changes\":[]}]} \n"
	mac := hmac.New(sha256.New, []byte("synthetic-test-secret"))
	_, _ = mac.Write([]byte(raw))
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	calls := 0
	fail := false
	h, err := newRawHandler(v, func(_ context.Context, batch Batch, owned []byte) error {
		calls++
		if string(owned) != raw || batch.BodyHash != digest(owned) || len(batch.Events) != 2 {
			t.Fatal("raw envelope changed or siblings lost before persistence")
		}
		if batch.Events[0].AssetID == batch.Events[1].AssetID {
			t.Fatal("mixed assets were combined")
		}
		payload := append([]byte(nil), batch.Events[0].Payload...)
		owned[0] = '!'
		if !bytes.Equal(payload, batch.Events[0].Payload) {
			t.Fatal("canonical payload aliases raw buffer")
		}
		if fail {
			return errors.New("sensitive-storage-details-must-not-leak")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	request := func(signature string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(raw))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Hub-Signature-256", signature)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := request(sig); w.Code != 200 || calls != 1 {
		t.Fatalf("valid handoff code=%d calls=%d", w.Code, calls)
	}
	if w := request("sha256=" + strings.Repeat("0", 64)); w.Code != 403 || calls != 1 {
		t.Fatalf("unverified raw reached callback code=%d calls=%d", w.Code, calls)
	}
	fail = true
	if w := request(sig); w.Code != 503 || calls != 2 || strings.Contains(w.Body.String(), "sensitive") {
		t.Fatalf("failed raw persistence not sanitized code=%d calls=%d", w.Code, calls)
	}
	if _, err := newRawHandler(v, nil); err == nil {
		t.Fatal("nil raw callback admitted")
	}
	if _, err := newRawHandler(&Verifier{}, func(context.Context, Batch, []byte) error { return nil }); err == nil {
		t.Fatal("zero verifier admitted")
	}
}
